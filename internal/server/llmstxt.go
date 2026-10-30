package server

import (
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/resolve"
	"github.com/monolithiclab/gomddoc/internal/seo"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

// PageURLs is a content tree's page URL rule (D35), as its template renderer
// (template.Renderer) owns it, language prefix included.
type PageURLs interface {
	// ContentURL is the site URL file is published at.
	ContentURL(file string) string
	// MarkdownURL is the site URL of file's Markdown, "" when it has none.
	MarkdownURL(file string) string
}

// LLMsTxtSource is what an llms.txt (llmstxt.org) is generated from: one
// content tree's navigation, metadata, resolver (which files it publishes)
// and page URLs, the site's title, description and domain, and the MCP
// endpoint path to name ("" when none is mounted, as in a static build).
type LLMsTxtSource struct {
	Nav      *navigation.Generator // nil: no pages
	Index    *metadata.Index       // nil: no descriptions, nothing left out as noindex
	Resolver *resolve.PathResolver
	URLs     PageURLs
	Site     *config.SiteConfig
	MCPPath  string
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
// description as a blockquote, the root page and the top-level pages as a
// list, then one "## {section}" per top-level navigation directory listing its
// pages depth-first, directory pages first, then "## Other pages": every
// published page the navigation drops (a directory holding only its README),
// in path order. Each entry links the page's Markdown (PageURLs.MarkdownURL,
// falling back to its page URL when stripping did not map it) as "- [title](url): description", without the
// colon when the page has no description. Pages whose robots frontmatter says
// noindex are left out, as from the sitemap. URLs are absolute with a domain
// and root-relative without one. When src.MCPPath is set, a last line names
// the MCP endpoint.
func GenerateLLMsTxt(src LLMsTxtSource) []byte {
	w := &llmsTxtWriter{src: src, listed: make(map[string]bool)}
	b := &w.b
	b.WriteString("# " + oneLine(src.Site.Meta.Title) + "\n")
	if desc := oneLine(src.Site.Meta.Description); desc != "" {
		b.WriteString("\n> " + desc + "\n")
	}

	var tree *navigation.NavNode
	if src.Nav != nil {
		tree = src.Nav.Tree()
	}
	published := src.publishedFiles()

	// The root page leads, even when the navigation is empty (a single-page
	// site has no tree at all).
	var top []string
	if root := src.Site.DefaultIndex; (tree != nil && tree.File == root) || published[root] {
		top = w.add(top, root, src.Site.Meta.Title)
	}
	if tree != nil {
		for _, child := range tree.Children {
			if !child.IsDir {
				top = w.add(top, child.File, child.Label)
			}
		}
	}
	writeList(b, top)

	if tree != nil {
		for _, section := range tree.Children {
			if section.IsDir {
				w.section(section.Label, w.dirEntries(section, nil))
			}
		}
	}

	var rest []string
	for _, file := range slices.Sorted(maps.Keys(published)) {
		if !w.listed[file] {
			rest = w.add(rest, file, "")
		}
	}
	w.section("Other pages", rest)

	if src.MCPPath != "" {
		b.WriteString("\nMCP server (Streamable HTTP): " + siteURL(src.Site.Meta.Domain, src.MCPPath) + "\n")
	}
	return []byte(b.String())
}

// publishedFiles is every Markdown page the tree publishes, by fs-relative
// path: the files stripping mapped to a Markdown URL and the pages of the
// metadata index. Both were built with Pipeline.Exclude, so neither holds an
// excluded file.
func (src LLMsTxtSource) publishedFiles() map[string]bool {
	files := make(map[string]bool)
	if src.Resolver != nil {
		for file := range src.Resolver.AllMappings() {
			if _, ok := src.Resolver.MarkdownURLPath(file); ok {
				files[file] = true
			}
		}
	}
	if src.Index != nil {
		for _, page := range src.Index.AllPages() {
			files[strings.TrimPrefix(page.Path, "/")] = true
		}
	}
	return files
}

// llmsTxtWriter accumulates an llms.txt, remembering which files it has
// considered so the "Other pages" pass lists only what the navigation dropped.
type llmsTxtWriter struct {
	src    LLMsTxtSource
	b      strings.Builder
	listed map[string]bool
}

// section writes "## {label}" and entries, or nothing when entries is empty.
func (w *llmsTxtWriter) section(label string, entries []string) {
	if len(entries) > 0 {
		w.b.WriteString("\n## " + oneLine(label) + "\n")
		writeList(&w.b, entries)
	}
}

// dirEntries appends dir's page (its default index) and then its children,
// depth-first in navigation order.
func (w *llmsTxtWriter) dirEntries(dir *navigation.NavNode, out []string) []string {
	out = w.add(out, dir.File, dir.Label)
	for _, child := range dir.Children {
		if child.IsDir {
			out = w.dirEntries(child, out)
		} else {
			out = w.add(out, child.File, child.Label)
		}
	}
	return out
}

// add appends the list item for the page published from file, titled from the
// metadata index, else fallback, else the file's name; it appends nothing for
// no file, a file already considered, or a noindex page.
func (w *llmsTxtWriter) add(out []string, file, fallback string) []string {
	if file == "" || w.listed[file] {
		return out
	}
	w.listed[file] = true
	src := w.src

	title, desc := fallback, ""
	if src.Index != nil {
		if page := src.Index.ByPath("/" + file); page != nil {
			if isNoindex(page.Meta) {
				return out
			}
			if page.Title != "" {
				title = page.Title
			}
			desc = oneLine(page.Description)
		}
	}
	if title == "" {
		title = file
	}

	urlPath := src.URLs.MarkdownURL(file)
	if urlPath == "" {
		urlPath = src.URLs.ContentURL(file)
	}
	line := "- [" + linkTextEscaper.Replace(oneLine(title)) + "](" + siteURL(src.Site.Meta.Domain, urlPath) + ")"
	if desc != "" {
		line += ": " + desc
	}
	return append(out, line)
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
