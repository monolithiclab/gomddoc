package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// llmsTxtLink matches an llms.txt entry, "- [title](url)" with an optional
// ": description", capturing the URL.
var llmsTxtLink = regexp.MustCompile(`(?m)^- \[(?:[^\]\\]|\\.)*\]\(([^)]+)\)`)

// llmsTxtMCPLine is the line serve adds for its /_mcp/ endpoint; a static build
// has none, which is the one difference between the two files (D55).
const llmsTxtMCPLine = "\nMCP server (Streamable HTTP): /_mcp/\n"

// writeLLMsTxtSite writes a site with a home page, a directory page, a page
// without a description, a noindex page, a redirect_from source and a French
// tree, plus domain in the config when it is not empty.
func writeLLMsTxtSite(t *testing.T, domain string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := "meta:\n  title: Docs\n  description: All the docs.\n"
	if domain != "" {
		cfg += "  domain: " + domain + "\n"
	}
	writeTestFile(t, dir, ".gomddoc/config.yml", cfg)
	writeTestFile(t, dir, "README.md", "---\ntitle: Home\ndescription: Start here.\n---\n# Home\n")
	writeTestFile(t, dir, "guides/README.md", "---\ntitle: Guides\n---\n# Guides\n")
	writeTestFile(t, dir, "guides/setup.md", "---\ntitle: Setup\ndescription: Install it.\n---\n# Setup\n")
	writeTestFile(t, dir, "guides/teardown.md", "# Teardown\n")
	writeTestFile(t, dir, "draft.md", "---\ntitle: Draft\nrobots: noindex\n---\n# Draft\n")
	writeTestFile(t, dir, "moved.md", "---\ntitle: Moved\nredirect_from:\n  - /old-page\n---\n# Moved\n")
	writeTestFile(t, dir, "fr-FR/README.md", "---\ntitle: Accueil\n---\n# Accueil\n")
	writeTestFile(t, dir, "fr-FR/guides/setup.md", "---\ntitle: Installation\n---\n# Installation\n")
	return dir
}

// TestLLMsTxt_ServeAndBuild pins D55 end to end: serve answers /llms.txt and
// /fr-FR/llms.txt as text/plain, each listing every navigation page of its tree
// once (noindex pages and redirect sources left out) by a .md link the same
// server answers 200; serve adds the MCP line, and build writes the same files
// without it.
func TestLLMsTxt_ServeAndBuild(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, domain, origin string
	}{
		{name: "relative without a domain"},
		{name: "absolute with a domain", domain: "docs.example.com", origin: "https://docs.example.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srcDir := writeLLMsTxtSite(t, tc.domain)
			served := serveHandler(t, srcDir)

			outDir := filepath.Join(t.TempDir(), "out")
			if err := (&BuildCmd{Dir: srcDir, Output: outDir}).Run(); err != nil {
				t.Fatalf("build: %v", err)
			}

			for _, file := range []struct {
				path string
				want []string
			}{
				{"/llms.txt", []string{"/README.md", "/guides/README.md", "/guides/setup.md", "/guides/teardown.md", "/moved.md"}},
				{"/fr-FR/llms.txt", []string{"/fr-FR/README.md", "/fr-FR/guides/setup.md"}},
			} {
				w := httptest.NewRecorder()
				served.ServeHTTP(w, httptest.NewRequest(http.MethodGet, file.path, nil))
				if w.Code != http.StatusOK {
					t.Fatalf("serve GET %s = %d, want 200", file.path, w.Code)
				}
				if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
					t.Errorf("serve GET %s Content-Type = %q", file.path, ct)
				}
				body := w.Body.String()

				var links []string
				for _, m := range llmsTxtLink.FindAllStringSubmatch(body, -1) {
					link, ok := strings.CutPrefix(m[1], tc.origin)
					if !ok || !strings.HasPrefix(link, "/") {
						t.Errorf("%s: link %q is not %s-relative", file.path, m[1], tc.origin)
						continue
					}
					links = append(links, link)

					page := httptest.NewRecorder()
					served.ServeHTTP(page, httptest.NewRequest(http.MethodGet, link, nil))
					if page.Code != http.StatusOK || !strings.HasPrefix(page.Header().Get("Content-Type"), "text/markdown") {
						t.Errorf("%s: GET %s = %d %q, want 200 text/markdown", file.path, link, page.Code, page.Header().Get("Content-Type"))
					}
				}
				slices.Sort(links)
				if !slices.Equal(links, file.want) {
					t.Errorf("%s links = %v, want each of %v once\n%s", file.path, links, file.want, body)
				}
				for _, absent := range []string{"draft", "Draft", "old-page"} {
					if strings.Contains(body, absent) {
						t.Errorf("%s lists %q:\n%s", file.path, absent, body)
					}
				}
				if !strings.Contains(body, "- [Setup]("+tc.origin+"/guides/setup.md): Install it.\n") && file.path == "/llms.txt" {
					t.Errorf("%s lacks the described setup entry:\n%s", file.path, body)
				}

				mcpLine := strings.Replace(llmsTxtMCPLine, "/_mcp/", tc.origin+"/_mcp/", 1)
				if !strings.HasSuffix(body, mcpLine) {
					t.Errorf("serve %s does not end with the MCP line %q:\n%s", file.path, mcpLine, body)
				}
				built := readTestFile(t, outDir, strings.TrimPrefix(file.path, "/"))
				if want := strings.TrimSuffix(body, mcpLine); built != want {
					t.Errorf("build %s is not serve's without the MCP line\nbuild:\n%s\nserve:\n%s", file.path, built, body)
				}
			}
		})
	}
}
