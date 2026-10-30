package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/resolve"
	"github.com/monolithiclab/gomddoc/internal/search"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

// markdownURLFiles is the default-language tree the .md URL tests serve. guide
// carries frontmatter and a tag shared with README, so its enriched body holds
// metadata and a related doc; prevNextForGuide adds prev/next, keyed on the
// clean path, so a .md request enriched under its own path would lose them.
var markdownURLFiles = fstest.MapFS{
	"README.md":      {Data: []byte("---\ntitle: Home\ntags: [docs]\n---\n# Home\n")},
	"guide.md":       {Data: []byte("---\ntitle: Guide\ntags: [docs]\n---\n# Guide\n\nguide body\n\n" + guideLinks)},
	"docs/README.md": {Data: []byte("---\ntitle: Docs\n---\n# Docs\n\ndocs index body\n")},
	"sub/index.md":   {Data: []byte("---\ntitle: Sub\n---\n# Sub\n\nsub index body\n")},
	"notes.markdown": {Data: []byte("---\ntitle: Notes\n---\n# Notes\n\nnotes body\n")},
	"about.html":     {Data: []byte("<p>about</p>")},
}

var markdownURLFrFiles = fstest.MapFS{
	"guide.md": {Data: []byte("---\ntitle: Guide FR\n---\n# Guide\n\nfrench-only-marker-54\n\n[self](guide.md#top)\n")},
}

// guideLinks are guide's in-body links: pages by their Markdown file, a
// directory page through its default index, and a link to no page.
const guideLinks = "[home](README.md) [docs](docs/README.md#intro) [notes](./notes.markdown) [nope](nope.md)\n"

// prevNextForGuide answers only for the clean path of guide, as the navigation
// generator does for a leaf page.
func prevNextForGuide(p string) (prev, next *enricher.PageLink) {
	if p != "/guide" {
		return nil, nil
	}
	return &enricher.PageLink{Path: "/", Title: "Home"}, &enricher.PageLink{Path: "/notes", Title: "Notes"}
}

func markdownURLEnrichers(t *testing.T, files fstest.MapFS) enricher.EnricherRegistry {
	t.Helper()
	idx, err := metadata.BuildIndex(context.Background(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	reg := enricher.NewDefaultEnricherRegistry()
	reg.Register(enricher.NewMarkdownEnricher(enricher.MarkdownEnricherOptions{
		MetaIndex:       idx,
		PrevNextBuilder: prevNextForGuide,
	}))
	return reg
}

// newMarkdownURLServer serves markdownURLFiles (and a fr tree) through the full
// route table, with .md, .markdown and .html stripped. layout is the default
// template, so a test can read PageContext fields back.
func newMarkdownURLServer(t *testing.T, domain, layout string) http.Handler {
	t.Helper()
	return newMarkdownURLServerWithMCP(t, domain, layout, nil)
}

// newMarkdownURLServerWithMCP is newMarkdownURLServer with mcp mounted at
// /_mcp/ (nil mounts nothing), and each tree's navigation for llms.txt.
func newMarkdownURLServerWithMCP(t *testing.T, domain, layout string, mcp http.Handler) http.Handler {
	t.Helper()

	strip := []string{".md", ".markdown", ".html"}
	isRenderable := func(m string) bool { return m == "text/markdown" || m == "text/html" }

	cfg := &config.Config{
		Server: config.ServerConfig{Port: ":8080", Dir: "."},
		Site:   config.NewSiteConfig("."),
	}
	cfg.Site.StripExtensions = strip
	cfg.Site.Meta.Domain = domain

	renderer := func(resolver *resolve.PathResolver, opts ...tmpl.RendererOption) *tmpl.HTMLRenderer {
		opts = append(opts, tmpl.WithResolver(resolver))
		return tmpl.NewHTMLRenderer(&cfg.Site, fstest.MapFS{
			"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(layout)},
			"assets/themes/default/layouts/error.html.tmpl":   {Data: []byte(errorLayout)},
		}, opts...)
	}

	resolver := resolve.Build(markdownURLFiles, resolve.BuildOptions{StripExtensions: strip, HasRenderer: isRenderable})
	frResolver := resolve.Build(markdownURLFrFiles, resolve.BuildOptions{StripExtensions: strip, HasRenderer: isRenderable})

	srv := NewHTTPServer(HTTPServerConfig{
		Config:           cfg,
		Provider:         newMemoryProvider(markdownURLFiles, "README.md", false),
		Registry:         setupTestRegistry(),
		EnricherRegistry: markdownURLEnrichers(t, markdownURLFiles),
		TemplateRenderer: renderer(resolver),
		Resolver:         resolver,
		NavGenerator:     navigation.NewGenerator(markdownURLFiles, "README.md", nil, resolver),
		SearchIndex:      markdownURLSearch(t, markdownURLFiles),
		MCPHandler:       mcp,
		AllLanguages:     []string{"fr"},
		DefaultLang:      "en-US",
		LangPipelines: map[string]LangPipelineConfig{
			"fr": {
				TemplateRenderer: renderer(frResolver, tmpl.WithLangPrefix("fr")),
				Provider:         newMemoryProvider(markdownURLFrFiles, "README.md", false),
				Resolver:         frResolver,
				NavGenerator:     navigation.NewGenerator(markdownURLFrFiles, "README.md", nil, frResolver),
				SearchIndex:      markdownURLSearch(t, markdownURLFrFiles),
				EnricherRegistry: markdownURLEnrichers(t, markdownURLFrFiles),
			},
		},
	})
	return srv.server.Handler
}

func markdownURLSearch(t *testing.T, files fstest.MapFS) *search.Index {
	t.Helper()
	idx, err := search.BuildIndex(context.Background(), files, nil, nil)
	if err != nil {
		t.Fatalf("search.BuildIndex: %v", err)
	}
	return idx
}

func getWithAccept(h http.Handler, target, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// TestMarkdownURL_ServesNegotiatedBody pins D54: a page's .md URL answers 200
// with the body the clean URL negotiates for Accept: text/markdown, whatever
// the request's own Accept says, and names the clean URL as canonical.
func TestMarkdownURL_ServesNegotiatedBody(t *testing.T) {
	t.Parallel()

	h := newMarkdownURLServer(t, "", "<html>{{.Page.Content}}</html>")

	tests := []struct {
		name      string
		mdURL     string
		accept    string
		cleanURL  string
		canonical string
		contains  string
	}{
		{"page", "/guide.md", "", "/guide", "/guide", "guide body"},
		{"Accept text/html is ignored", "/guide.md", "text/html", "/guide", "/guide", "guide body"},
		{"a browser's Accept is ignored", "/guide.md", "text/html,application/xhtml+xml,*/*;q=0.8", "/guide", "/guide", "guide body"},
		{"README.md serves the directory page", "/docs/README.md", "text/html", "/docs", "/docs", "docs index body"},
		{"index.md", "/sub/index.md", "", "/sub/index", "/sub/index", "sub index body"},
		{".markdown", "/notes.markdown", "", "/notes", "/notes", "notes body"},
		{"language prefix", "/fr/guide.md", "text/html", "/fr/guide", "/fr/guide", "french-only-marker-54"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := getWithAccept(h, tt.mdURL, tt.accept)
			if w.Code != http.StatusOK {
				t.Fatalf("GET %s = %d (Location %q), want 200", tt.mdURL, w.Code, w.Header().Get("Location"))
			}
			if ct := w.Header().Get("Content-Type"); ct != "text/markdown; charset=utf-8" {
				t.Errorf("Content-Type = %q, want text/markdown; charset=utf-8", ct)
			}
			if got, want := w.Header().Get("Link"), "<"+tt.canonical+`>; rel="canonical"`; got != want {
				t.Errorf("Link = %q, want %q", got, want)
			}

			negotiated := getWithAccept(h, tt.cleanURL, "text/markdown")
			if negotiated.Code != http.StatusOK {
				t.Fatalf("GET %s (text/markdown) = %d, want 200", tt.cleanURL, negotiated.Code)
			}
			if w.Body.String() != negotiated.Body.String() {
				t.Errorf("GET %s body differs from the negotiated %s body\n.md:\n%s\nnegotiated:\n%s",
					tt.mdURL, tt.cleanURL, w.Body.String(), negotiated.Body.String())
			}
			if !strings.Contains(w.Body.String(), tt.contains) {
				t.Errorf("body missing %q:\n%s", tt.contains, w.Body.String())
			}
		})
	}

	// The enrichment is the clean URL's: frontmatter merged with related docs
	// and prev/next, which key on the clean path.
	body := getWithAccept(h, "/guide.md", "").Body.String()
	for _, want := range []string{"title: Guide", "related_docs:", "prev_page:", "next_page:"} {
		if !strings.Contains(body, want) {
			t.Errorf("/guide.md body missing %q:\n%s", want, body)
		}
	}
}

// The clean URL is unchanged: HTML by default, Markdown by negotiation, and a
// Vary: Accept on both.
func TestMarkdownURL_CleanURLStillNegotiates(t *testing.T) {
	t.Parallel()

	h := newMarkdownURLServer(t, "", "<html>{{.Page.Content}}</html>")

	w := getWithAccept(h, "/guide", "")
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("GET /guide Content-Type = %q, want text/html", ct)
	}
	if v := w.Header().Get("Vary"); !strings.Contains(v, "Accept") {
		t.Errorf("GET /guide Vary = %q, want Accept", v)
	}
	w = getWithAccept(h, "/guide", "text/markdown")
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/markdown") {
		t.Errorf("GET /guide (text/markdown) Content-Type = %q, want text/markdown", ct)
	}
}

func TestMarkdownURL_CanonicalWithDomain(t *testing.T) {
	t.Parallel()

	h := newMarkdownURLServer(t, "docs.example.com", "<html>{{.Page.Content}}</html>")

	for target, want := range map[string]string{
		"/guide.md":       `<https://docs.example.com/guide>; rel="canonical"`,
		"/docs/README.md": `<https://docs.example.com/docs>; rel="canonical"`,
		"/fr/guide.md":    `<https://docs.example.com/fr/guide>; rel="canonical"`,
	} {
		w := getWithAccept(h, target, "")
		if got := w.Header().Get("Link"); got != want {
			t.Errorf("GET %s Link = %q, want %q", target, got, want)
		}
	}
}

func TestMarkdownURL_MissingAndOtherExtensions(t *testing.T) {
	t.Parallel()

	h := newMarkdownURLServer(t, "", "<html>{{.Page.Content}}</html>")

	// A .md URL for a missing page is a 404, not a redirect.
	for _, target := range []string{"/nope.md", "/fr/nope.md", "/nope.markdown"} {
		if w := getWithAccept(h, target, ""); w.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d (Location %q), want 404", target, w.Code, w.Header().Get("Location"))
		}
	}

	// Any other stripped extension still answers 301 to the clean URL.
	w := getWithAccept(h, "/about.html", "")
	if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "/about" {
		t.Errorf("GET /about.html = %d, Location %q; want 301 to /about", w.Code, w.Header().Get("Location"))
	}
}

// TestMarkdownURL_PageContextAlternate pins the field the shared head partial
// links the Markdown from: the page's .md URL with the language prefix, the
// default index's own file for a directory page, and nothing on a page
// without a source file.
func TestMarkdownURL_PageContextAlternate(t *testing.T) {
	t.Parallel()

	h := newMarkdownURLServer(t, "", "[{{.Page.MarkdownURL}}]")

	for target, want := range map[string]string{
		"/guide":     "[/guide.md]",
		"/docs":      "[/docs/README.md]",
		"/":          "[/README.md]",
		"/sub/index": "[/sub/index.md]",
		"/notes":     "[/notes.markdown]",
		"/fr/guide":  "[/fr/guide.md]",
	} {
		w := getWithAccept(h, target, "text/html")
		if got := w.Body.String(); got != want {
			t.Errorf("GET %s MarkdownURL = %s, want %s", target, got, want)
		}
	}
}

// TestMarkdownURL_HTMLLinksCleanURLs pins D54's split: the HTML page links
// each page its body names at the page's clean URL (with the language prefix),
// while the Markdown keeps the author's links.
func TestMarkdownURL_HTMLLinksCleanURLs(t *testing.T) {
	t.Parallel()

	h := newMarkdownURLServer(t, "", "<html>{{.Page.Content}}</html>")

	html := getWithAccept(h, "/guide", "text/html").Body.String()
	for _, want := range []string{`href="/"`, `href="/docs#intro"`, `href="/notes"`, `href="nope.md"`} {
		if !strings.Contains(html, want) {
			t.Errorf("GET /guide missing %s:\n%s", want, html)
		}
	}
	if fr := getWithAccept(h, "/fr/guide", "text/html").Body.String(); !strings.Contains(fr, `href="/fr/guide#top"`) {
		t.Errorf("GET /fr/guide missing href=\"/fr/guide#top\":\n%s", fr)
	}

	md := getWithAccept(h, "/guide.md", "").Body.String()
	if !strings.Contains(md, guideLinks) {
		t.Errorf("GET /guide.md does not keep the author's links:\n%s", md)
	}
}

// mdPageHref matches an href naming a page's Markdown URL, fragment or query
// allowed.
var mdPageHref = regexp.MustCompile(`href="([^"#?]*\.(?:md|markdown))(?:[#?][^"]*)?"`)

// TestMarkdownURL_HTMLHasNoMarkdownHref: a rendered HTML page links a .md page
// URL only as its own rel="alternate".
func TestMarkdownURL_HTMLHasNoMarkdownHref(t *testing.T) {
	t.Parallel()

	layout := `<html><head>{{with .Page.MarkdownURL}}<link rel="alternate" type="text/markdown" href="{{.}}">{{end}}</head>` +
		`<body>{{.Page.Content}}</body></html>`
	h := newMarkdownURLServer(t, "", layout)
	alternate := regexp.MustCompile(`<link rel="alternate" type="text/markdown" href="[^"]*">`)

	for _, target := range []string{"/", "/guide", "/docs", "/notes", "/fr/guide"} {
		body := getWithAccept(h, target, "text/html").Body.String()
		if !alternate.MatchString(body) {
			t.Errorf("GET %s has no rel=alternate link:\n%s", target, body)
		}
		rest := alternate.ReplaceAllString(body, "")
		for _, m := range mdPageHref.FindAllStringSubmatch(rest, -1) {
			if m[1] != "nope.md" { // the one link to no page stays as written
				t.Errorf("GET %s links Markdown URL %q outside rel=alternate", target, m[0])
			}
		}
	}
}

// TestMarkdownURL_SearchLinksCleanURLs: the search API, whose paths the
// theme's search box uses as hrefs, answers each page's clean URL, with the
// language prefix for a language's index.
func TestMarkdownURL_SearchLinksCleanURLs(t *testing.T) {
	t.Parallel()

	h := newMarkdownURLServer(t, "", "<html>{{.Page.Content}}</html>")

	for target, want := range map[string]string{
		"/api/search?q=guide+body":                    "/guide",
		"/api/search?q=docs+index+body":               "/docs",
		"/api/search?q=french-only-marker-54&lang=fr": "/fr/guide",
	} {
		w := getWithAccept(h, target, "")
		var results []search.SearchResult
		if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
			t.Fatalf("GET %s: %v\n%s", target, err, w.Body.String())
		}
		if len(results) == 0 || results[0].Path != want {
			t.Errorf("GET %s top result = %+v, want path %s", target, results, want)
		}
	}
}
