package server

import (
	"net/http"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/resolve"
	"github.com/monolithiclab/gomddoc/internal/seo"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

// LLMsTxtSource is what an llms.txt (llmstxt.org) is generated from: one
// content tree's navigation, metadata and resolver, the site's title,
// description and domain, the tree's URL prefix ("" or "/{lang}"), and the
// MCP endpoint path to name ("" when none is mounted, as in a static build).
type LLMsTxtSource struct {
	Nav        *navigation.Generator // nil: no pages
	Index      *metadata.Index       // nil: no descriptions, nothing left out as noindex
	Resolver   *resolve.PathResolver
	Site       *config.SiteConfig
	PathPrefix string
	MCPPath    string
}

// LLMsTxtHandler serves a content tree's llms.txt, generated on first request
// and cached: its sources are immutable after setup.
type LLMsTxtHandler struct {
	body *lazyBytes
}

// NewLLMsTxtHandler creates an LLMsTxtHandler for src.
func NewLLMsTxtHandler(src LLMsTxtSource) *LLMsTxtHandler {
	return &LLMsTxtHandler{
		body: newLazyBytes("llms.txt", func() ([]byte, error) {
			return GenerateLLMsTxt(src), nil
		}),
	}
}

// ServeHTTP writes the llms.txt response.
func (h *LLMsTxtHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.body.serve(w, r, mimePlain)
}

// GenerateLLMsTxt produces the llms.txt (D55): "# {site title}", the site
// description as a blockquote, the home page and top-level pages as a list,
// then one "## {section}" per top-level navigation directory listing its pages
// depth-first, directory pages first. Each entry links the page's Markdown
// (resolve.PathResolver.MarkdownURLPath, falling back to its page URL when
// stripping did not map it) as "- [title](url): description", without the
// colon when the page has no description. Pages whose robots frontmatter says
// noindex are left out, as from the sitemap. URLs are absolute with a domain
// and root-relative without one. When src.MCPPath is set, a last line names
// the MCP endpoint.
func GenerateLLMsTxt(src LLMsTxtSource) []byte {
	var b strings.Builder
	b.WriteString("# " + oneLine(src.Site.Meta.Title) + "\n")
	if desc := oneLine(src.Site.Meta.Description); desc != "" {
		b.WriteString("\n> " + desc + "\n")
	}

	var tree *navigation.NavNode
	if src.Nav != nil {
		tree = src.Nav.Tree()
	}
	if tree != nil {
		var top []string
		if entry := src.entry(tree.File, src.Site.Meta.Title); entry != "" {
			top = append(top, entry)
		}
		for _, child := range tree.Children {
			if !child.IsDir {
				if entry := src.entry(child.File, child.Label); entry != "" {
					top = append(top, entry)
				}
			}
		}
		writeList(&b, top)

		for _, section := range tree.Children {
			if !section.IsDir {
				continue
			}
			if entries := src.sectionEntries(section, nil); len(entries) > 0 {
				b.WriteString("\n## " + oneLine(section.Label) + "\n")
				writeList(&b, entries)
			}
		}
	}

	if src.MCPPath != "" {
		b.WriteString("\nMCP server (Streamable HTTP): " + siteURL(src.Site.Meta.Domain, src.MCPPath) + "\n")
	}
	return []byte(b.String())
}

// sectionEntries appends dir's page (its default index) and then its children,
// depth-first in navigation order.
func (src LLMsTxtSource) sectionEntries(dir *navigation.NavNode, out []string) []string {
	if entry := src.entry(dir.File, dir.Label); entry != "" {
		out = append(out, entry)
	}
	for _, child := range dir.Children {
		if child.IsDir {
			out = src.sectionEntries(child, out)
		} else if entry := src.entry(child.File, child.Label); entry != "" {
			out = append(out, entry)
		}
	}
	return out
}

// entry renders the list item for the page published from file, titled from
// the metadata index or fallback; "" for no file or a noindex page.
func (src LLMsTxtSource) entry(file, fallback string) string {
	if file == "" {
		return ""
	}
	title, desc := fallback, ""
	if src.Index != nil {
		if page := src.Index.ByPath("/" + file); page != nil {
			if isNoindex(page.Meta) {
				return ""
			}
			if page.Title != "" {
				title = page.Title
			}
			desc = oneLine(page.Description)
		}
	}

	urlPath, ok := src.Resolver.MarkdownURLPath(file)
	if !ok {
		urlPath = src.Resolver.PageURLPath(file, src.Site.DefaultIndex)
	}
	line := "- [" + linkTextEscaper.Replace(oneLine(title)) + "](" + siteURL(src.Site.Meta.Domain, src.PathPrefix+urlPath) + ")"
	if desc != "" {
		line += ": " + desc
	}
	return line
}

// writeList writes entries as a Markdown list preceded by a blank line.
func writeList(b *strings.Builder, entries []string) {
	if len(entries) == 0 {
		return
	}
	b.WriteString("\n")
	for _, e := range entries {
		b.WriteString(e + "\n")
	}
}

// linkTextEscaper keeps a title from closing its link text early.
var linkTextEscaper = strings.NewReplacer(`\`, `\\`, "[", `\[`, "]", `\]`)

// oneLine collapses s's whitespace, newlines included, to single spaces: every
// llms.txt entry is one line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// isNoindex reports whether a page's robots frontmatter asks search engines
// not to index it, which drops the page from the sitemap, the feed and
// llms.txt.
func isNoindex(meta map[string]any) bool {
	robots, ok := meta["robots"].(string)
	return ok && strings.Contains(robots, "noindex")
}

// siteURL is the URL of sitePath, a site-absolute path: absolute with the
// configured domain, root-relative without one. The canonical Link header and
// llms.txt both name URLs this way.
func siteURL(domain, sitePath string) string {
	if u := seo.PageURL(domain, sitePath, ""); u != "" {
		return u
	}
	return sitePath
}
