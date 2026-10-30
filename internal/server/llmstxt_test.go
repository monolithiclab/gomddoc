package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/resolve"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

// llmsTxtTestFS covers every shape an llms.txt entry takes: the home page and
// a directory page (linked by their README.md, D54), a nested page flattened
// into its top-level section, a page without a description (no colon), one
// without frontmatter (titled from its heading), a title with brackets, a
// multi-line description, and a noindex page left out.
var llmsTxtTestFS = fstest.MapFS{
	"README.md":                 {Data: []byte("---\ntitle: Home\ndescription: Start here.\n---\n# Home\n")},
	"intro.md":                  {Data: []byte("---\ntitle: Intro\ndescription: |\n  What it is.\n  Two lines.\n---\n# Intro\n")},
	"hidden.md":                 {Data: []byte("---\ntitle: Hidden\nrobots: \"noindex, nofollow\"\n---\n# Hidden\n")},
	"api/ref.md":                {Data: []byte("---\ntitle: Reference\ndescription: The API.\n---\n# Reference\n")},
	"guides/README.md":          {Data: []byte("---\ntitle: Guides overview\n---\n# Guides\n")},
	"guides/setup.md":           {Data: []byte("---\ntitle: Setup [beta]\ndescription: Install it.\n---\n# Setup\n")},
	"guides/teardown.md":        {Data: []byte("# Teardown\n")},
	"guides/advanced/tuning.md": {Data: []byte("---\ntitle: Tuning\n---\n# Tuning\n")},
	// README-only directories: the navigation drops them, so the published one
	// is listed under "Other pages"; the noindex and the excluded one are not.
	"changelog/README.md": {Data: []byte("---\ntitle: Changelog\n---\n# Changelog\n")},
	"drafts/README.md":    {Data: []byte("---\ntitle: Drafts\nrobots: noindex\n---\n# Drafts\n")},
	"private/README.md":   {Data: []byte("---\ntitle: Private\n---\n# Private\n")},
}

// llmsTxtExclude is the fixture's Pipeline.Exclude.
var llmsTxtExclude = []string{"private/"}

// llmsTxtSource builds the source GenerateLLMsTxt reads, the way the pipeline
// does: one resolver for the navigation and the links, one metadata index.
func llmsTxtSource(t *testing.T, domain, prefix, mcpPath string) LLMsTxtSource {
	t.Helper()
	resolver := resolve.Build(llmsTxtTestFS, resolve.BuildOptions{
		StripExtensions: []string{".md"},
		Exclude:         llmsTxtExclude,
		HasRenderer:     func(m string) bool { return m == "text/markdown" },
	})
	idx, err := metadata.BuildIndex(context.Background(), llmsTxtTestFS, llmsTxtExclude)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	nav := navigation.NewGenerator(llmsTxtTestFS, "README.md", llmsTxtExclude, resolver)
	nav.SetTitleLookup(idx.TitleForPath)

	site := config.NewSiteConfig(".")
	site.Meta.Title = "Docs"
	site.Meta.Description = "All the docs."
	site.Meta.Domain = domain
	return LLMsTxtSource{
		Nav:        nav,
		Index:      idx,
		Resolver:   resolver,
		Site:       &site,
		PathPrefix: prefix,
		MCPPath:    mcpPath,
	}
}

func TestGenerateLLMsTxt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		domain  string
		prefix  string
		mcpPath string
		want    string
	}{
		{
			name: "relative without a domain, no MCP line",
			want: `# Docs

> All the docs.

- [Home](/README.md): Start here.
- [Intro](/intro.md): What it is. Two lines.

## Api

- [Reference](/api/ref.md): The API.

## Guides

- [Guides overview](/guides/README.md)
- [Tuning](/guides/advanced/tuning.md)
- [Setup \[beta\]](/guides/setup.md): Install it.
- [Teardown](/guides/teardown.md)

## Other pages

- [Changelog](/changelog/README.md)
`,
		},
		{
			name:    "absolute with a domain, MCP line last",
			domain:  "docs.example.com",
			mcpPath: "/_mcp/",
			want: `# Docs

> All the docs.

- [Home](https://docs.example.com/README.md): Start here.
- [Intro](https://docs.example.com/intro.md): What it is. Two lines.

## Api

- [Reference](https://docs.example.com/api/ref.md): The API.

## Guides

- [Guides overview](https://docs.example.com/guides/README.md)
- [Tuning](https://docs.example.com/guides/advanced/tuning.md)
- [Setup \[beta\]](https://docs.example.com/guides/setup.md): Install it.
- [Teardown](https://docs.example.com/guides/teardown.md)

## Other pages

- [Changelog](https://docs.example.com/changelog/README.md)

MCP server (Streamable HTTP): https://docs.example.com/_mcp/
`,
		},
		{
			name:    "language prefix, relative MCP line without a domain",
			prefix:  "/fr-FR",
			mcpPath: "/_mcp/",
			want: `# Docs

> All the docs.

- [Home](/fr-FR/README.md): Start here.
- [Intro](/fr-FR/intro.md): What it is. Two lines.

## Api

- [Reference](/fr-FR/api/ref.md): The API.

## Guides

- [Guides overview](/fr-FR/guides/README.md)
- [Tuning](/fr-FR/guides/advanced/tuning.md)
- [Setup \[beta\]](/fr-FR/guides/setup.md): Install it.
- [Teardown](/fr-FR/guides/teardown.md)

## Other pages

- [Changelog](/fr-FR/changelog/README.md)

MCP server (Streamable HTTP): /_mcp/
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := string(GenerateLLMsTxt(llmsTxtSource(t, tt.domain, tt.prefix, tt.mcpPath)))
			if got != tt.want {
				t.Errorf("GenerateLLMsTxt() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

// TestGenerateLLMsTxt_Minimal pins the edges: no site description means no
// blockquote, a home page without a title takes the site title, and without
// stripping a page's link is its page URL (the file path, served as is).
func TestGenerateLLMsTxt_Minimal(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"README.md": {Data: []byte("# Welcome\n")},
		"page.md":   {Data: []byte("# Page\n")},
	}
	site := config.NewSiteConfig(".")
	site.Meta.Title = "Docs"
	site.Meta.Description = ""
	got := string(GenerateLLMsTxt(LLMsTxtSource{
		Nav:  navigation.NewGenerator(fsys, "README.md", nil, nil),
		Site: &site,
	}))
	want := "# Docs\n\n- [Docs](/)\n- [Page](/page.md)\n"
	if got != want {
		t.Errorf("GenerateLLMsTxt() = %q, want %q", got, want)
	}
}

// TestGenerateLLMsTxt_SinglePage pins the root link on a site whose only page
// is its root README: the navigation tree is nil there.
func TestGenerateLLMsTxt_SinglePage(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{"README.md": {Data: []byte("---\ntitle: Home\n---\n# Home\n")}}
	resolver := resolve.Build(fsys, resolve.BuildOptions{
		StripExtensions: []string{".md"},
		HasRenderer:     func(m string) bool { return m == "text/markdown" },
	})
	nav := navigation.NewGenerator(fsys, "README.md", nil, resolver)
	if nav.Tree() != nil {
		t.Fatal("fixture no longer has a nil navigation tree")
	}
	site := config.NewSiteConfig(".")
	site.Meta.Title = "Docs"
	site.Meta.Description = "One page."
	got := string(GenerateLLMsTxt(LLMsTxtSource{Nav: nav, Index: buildTestIndex(t, fsys), Resolver: resolver, Site: &site}))
	want := "# Docs\n\n> One page.\n\n- [Home](/README.md)\n"
	if got != want {
		t.Errorf("GenerateLLMsTxt() = %q, want %q", got, want)
	}
}

func TestLLMsTxtHandler(t *testing.T) {
	t.Parallel()
	assertRevalidates(t, revalidationCase{
		Handler:   NewLLMsTxtHandler(llmsTxtSource(t, "", "", "")),
		Path:      "/llms.txt",
		WantType:  mimePlain,
		WantCache: cacheDynamic,
	})
}

// TestServer_LLMsTxtRoutes pins the registration: /llms.txt for the default
// tree and /{lang}/llms.txt per language, each with that tree's pages, and the
// MCP line exactly when the server mounts /_mcp/.
func TestServer_LLMsTxtRoutes(t *testing.T) {
	t.Parallel()

	for _, mcpOn := range []bool{false, true} {
		var mcp http.Handler
		if mcpOn {
			mcp = http.NotFoundHandler()
		}
		h := newMarkdownURLServerWithMCP(t, "", "<html>{{.Page.Content}}</html>", mcp)
		for _, tc := range []struct{ path, wantLink string }{
			{"/llms.txt", "[Guide](/guide.md)"},
			{"/fr/llms.txt", "[Guide](/fr/guide.md)"},
		} {
			w := getWithAccept(h, tc.path, "")
			if w.Code != http.StatusOK {
				t.Fatalf("mcp=%v GET %s = %d, want 200", mcpOn, tc.path, w.Code)
			}
			if ct := w.Header().Get("Content-Type"); ct != mimePlain {
				t.Errorf("mcp=%v GET %s Content-Type = %q, want %q", mcpOn, tc.path, ct, mimePlain)
			}
			body := w.Body.String()
			if !containsLine(body, "- "+tc.wantLink) {
				t.Errorf("mcp=%v GET %s lacks %s:\n%s", mcpOn, tc.path, tc.wantLink, body)
			}
			if got := containsLine(body, "MCP server (Streamable HTTP): /_mcp/"); got != mcpOn {
				t.Errorf("mcp=%v GET %s has the MCP line = %v:\n%s", mcpOn, tc.path, got, body)
			}
		}
	}
}

// containsLine reports whether body holds line as a whole line.
func containsLine(body, line string) bool {
	return strings.Contains("\n"+body, "\n"+line+"\n")
}
