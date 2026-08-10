package seo

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGenerateJSONLD(t *testing.T) {
	t.Parallel()

	baseCfg := JSONLDConfig{
		Domain:       "docs.example.com",
		SiteName:     "My Docs",
		DefaultIndex: "README.md",
		HasSearch:    true,
	}

	tests := []struct {
		name       string
		cfg        JSONLDConfig
		page       JSONLDPage
		wantEmpty  bool
		wantTypes  []string // expected @type values in the JSON array
		checkExtra func(t *testing.T, schemas []map[string]any)
	}{
		{
			name:      "empty domain returns empty",
			cfg:       JSONLDConfig{},
			page:      JSONLDPage{Path: "/test.md"},
			wantEmpty: true,
		},
		{
			name: "basic page with title",
			cfg:  baseCfg,
			page: JSONLDPage{
				Path:  "/guide/setup.md",
				Title: "Setup Guide",
			},
			wantTypes: []string{"TechArticle"},
			checkExtra: func(t *testing.T, schemas []map[string]any) {
				t.Helper()
				article := schemas[0]
				if article["headline"] != "Setup Guide" {
					t.Errorf("headline = %v, want %q", article["headline"], "Setup Guide")
				}
				wantURL := "https://docs.example.com/guide/setup.md"
				if article["url"] != wantURL {
					t.Errorf("url = %v, want %q", article["url"], wantURL)
				}
				if _, ok := article["author"]; ok {
					t.Error("author should be omitted when empty")
				}
				if _, ok := article["datePublished"]; ok {
					t.Error("datePublished should be omitted when zero")
				}
				if _, ok := article["dateModified"]; ok {
					t.Error("dateModified should be omitted when both Modified and Date are zero")
				}
			},
		},
		{
			name: "page with all fields",
			cfg:  baseCfg,
			page: JSONLDPage{
				Path:        "/guide/setup.md",
				Title:       "Setup Guide",
				Description: "How to set up",
				Author:      "Alice",
				Date:        time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
			},
			wantTypes: []string{"TechArticle"},
			checkExtra: func(t *testing.T, schemas []map[string]any) {
				t.Helper()
				article := schemas[0]
				if article["description"] != "How to set up" {
					t.Errorf("description = %v", article["description"])
				}
				author, ok := article["author"].(map[string]any)
				if !ok {
					t.Fatal("author should be a map")
				}
				if author["name"] != "Alice" {
					t.Errorf("author.name = %v", author["name"])
				}
				if article["datePublished"] != "2025-06-15T00:00:00Z" {
					t.Errorf("datePublished = %v", article["datePublished"])
				}
			},
		},
		{
			name: "breadcrumbs with >1 item",
			cfg:  baseCfg,
			page: JSONLDPage{
				Path:  "/guide/setup.md",
				Title: "Setup",
				Breadcrumbs: []BreadcrumbItem{
					{Name: "Home", URL: "https://docs.example.com/"},
					{Name: "Guide", URL: "https://docs.example.com/guide/"},
					{Name: "Setup", URL: "https://docs.example.com/guide/setup.md"},
				},
			},
			wantTypes: []string{"TechArticle", "BreadcrumbList"},
			checkExtra: func(t *testing.T, schemas []map[string]any) {
				t.Helper()
				bc := schemas[1]
				items, ok := bc["itemListElement"].([]any)
				if !ok {
					t.Fatal("itemListElement should be an array")
				}
				if len(items) != 3 {
					t.Errorf("itemListElement length = %d, want 3", len(items))
				}
				first := items[0].(map[string]any)
				if first["position"] != float64(1) {
					t.Errorf("first position = %v, want 1", first["position"])
				}
				if first["name"] != "Home" {
					t.Errorf("first name = %v", first["name"])
				}
			},
		},
		{
			name: "single breadcrumb omits BreadcrumbList",
			cfg:  baseCfg,
			page: JSONLDPage{
				Path:  "/",
				Title: "Home",
				Breadcrumbs: []BreadcrumbItem{
					{Name: "Home", URL: "https://docs.example.com/"},
				},
			},
			wantTypes: []string{"TechArticle"},
		},
		{
			name: "index page includes WebSite with SearchAction",
			cfg:  baseCfg,
			page: JSONLDPage{
				Path:    "/README.md",
				Title:   "Home",
				IsIndex: true,
			},
			wantTypes: []string{"TechArticle", "WebSite"},
			checkExtra: func(t *testing.T, schemas []map[string]any) {
				t.Helper()
				ws := schemas[1]
				if ws["name"] != "My Docs" {
					t.Errorf("website name = %v", ws["name"])
				}
				action, ok := ws["potentialAction"].(map[string]any)
				if !ok {
					t.Fatal("potentialAction should be a map")
				}
				if action["@type"] != "SearchAction" {
					t.Errorf("action type = %v", action["@type"])
				}
			},
		},
		{
			name: "index page without search omits SearchAction",
			cfg: JSONLDConfig{
				Domain:       "docs.example.com",
				SiteName:     "My Docs",
				DefaultIndex: "README.md",
				HasSearch:    false,
			},
			page: JSONLDPage{
				Path:    "/",
				IsIndex: true,
			},
			wantTypes: []string{"TechArticle", "WebSite"},
			checkExtra: func(t *testing.T, schemas []map[string]any) {
				t.Helper()
				ws := schemas[1]
				if _, ok := ws["potentialAction"]; ok {
					t.Error("potentialAction should be omitted when HasSearch is false")
				}
			},
		},
		{
			// The two dates are independent signals: publication comes from
			// frontmatter, modification from the file. Asserting only that both
			// keys are present passes an implementation that emits the same
			// value twice, so the row uses distinct times and checks each.
			name: "modified is independent of published",
			cfg:  baseCfg,
			page: JSONLDPage{
				Path:     "/guide/setup.md",
				Date:     time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
				Modified: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			},
			wantTypes: []string{"TechArticle"},
			checkExtra: func(t *testing.T, schemas []map[string]any) {
				t.Helper()
				article := schemas[0]
				if got, want := article["datePublished"], "2025-06-15T00:00:00Z"; got != want {
					t.Errorf("datePublished = %v, want %q", got, want)
				}
				if got, want := article["dateModified"], "2026-01-02T03:04:05Z"; got != want {
					t.Errorf("dateModified = %v, want %q", got, want)
				}
			},
		},
		{
			// Git-backed sites and any provider whose Stat fails land here. The
			// fallback matches feed.go's <updated>, so the two documents cannot
			// claim different freshness for one page.
			name: "modified falls back to published",
			cfg:  baseCfg,
			page: JSONLDPage{
				Path: "/guide/setup.md",
				Date: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
			},
			wantTypes: []string{"TechArticle"},
			checkExtra: func(t *testing.T, schemas []map[string]any) {
				t.Helper()
				if got, want := schemas[0]["dateModified"], "2025-06-15T00:00:00Z"; got != want {
					t.Errorf("dateModified = %v, want %q", got, want)
				}
			},
		},
		{
			// A page with no frontmatter date still has an mtime, and that is
			// the ranking signal sitemap.xml's <lastmod> already publishes.
			// datePublished stays absent rather than being invented from it.
			name: "modified without published",
			cfg:  baseCfg,
			page: JSONLDPage{
				Path:     "/guide/setup.md",
				Modified: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			},
			wantTypes: []string{"TechArticle"},
			checkExtra: func(t *testing.T, schemas []map[string]any) {
				t.Helper()
				if _, ok := schemas[0]["datePublished"]; ok {
					t.Error("datePublished should not be invented from the file mtime")
				}
				if got, want := schemas[0]["dateModified"], "2026-01-02T03:04:05Z"; got != want {
					t.Errorf("dateModified = %v, want %q", got, want)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GenerateJSONLD(tt.cfg, tt.page)

			if tt.wantEmpty {
				if result != "" {
					t.Errorf("expected empty result, got %q", result)
				}
				return
			}

			if result == "" {
				t.Fatal("expected non-empty result")
			}

			var schemas []map[string]any
			if err := json.Unmarshal([]byte(result), &schemas); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, result)
			}

			if len(schemas) != len(tt.wantTypes) {
				t.Fatalf("got %d schemas, want %d", len(schemas), len(tt.wantTypes))
			}

			for i, wantType := range tt.wantTypes {
				if schemas[i]["@type"] != wantType {
					t.Errorf("schema[%d].@type = %v, want %q", i, schemas[i]["@type"], wantType)
				}
			}

			if tt.checkExtra != nil {
				tt.checkExtra(t, schemas)
			}
		})
	}
}
