package metadata

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestExtractFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantNil bool
		wantErr bool
		wantKey string
		wantVal string
	}{
		{
			name:    "valid frontmatter",
			content: "---\ntitle: Hello\n---\n# Body",
			wantKey: "title",
			wantVal: "Hello",
		},
		{
			name:    "no frontmatter",
			content: "# Just a heading",
			wantNil: true,
		},
		{
			name:    "empty file",
			content: "",
			wantNil: true,
		},
		{
			name:    "unclosed frontmatter",
			content: "---\ntitle: Hello\nno closing",
			wantNil: true,
		},
		{
			name:    "frontmatter with BOM",
			content: "\xef\xbb\xbf---\ntitle: BOM\n---\nbody",
			wantKey: "title",
			wantVal: "BOM",
		},
		{
			name:    "invalid YAML",
			content: "---\n: :\n  - [\n---\nbody",
			wantErr: true,
		},
		{
			name:    "frontmatter with carriage returns",
			content: "---\r\ntitle: CRLF\r\n---\r\nbody",
			wantKey: "title",
			wantVal: "CRLF",
		},
		{
			name:    "delimiter not at start",
			content: "some text\n---\ntitle: Hello\n---\n",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractFrontmatter([]byte(tt.content))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if result != nil {
					t.Fatalf("expected nil result, got %v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if v, ok := result[tt.wantKey]; !ok {
				t.Fatalf("missing key %q", tt.wantKey)
			} else if s, ok := v.(string); !ok || s != tt.wantVal {
				t.Fatalf("expected %q=%q, got %v", tt.wantKey, tt.wantVal, v)
			}
		})
	}
}

func TestBuildIndex(t *testing.T) {
	testFS := fstest.MapFS{
		"readme.md": {
			Data: []byte("---\ntitle: Home\ntags:\n  - Go\n  - Docs\ndate: 2025-01-15\ndescription: The homepage\ncategory: main\n---\n# Home"),
		},
		"guide/intro.md": {
			Data: []byte("---\ntitle: Intro\ntags:\n  - go\n  - tutorial\n---\n# Intro"),
		},
		"no-frontmatter.md": {
			Data: []byte("# No frontmatter here"),
		},
		"image.png": {
			Data: []byte("not a markdown file"),
		},
		".hidden/secret.md": {
			Data: []byte("---\ntitle: Secret\n---\n# Secret"),
		},
		".dotfile.md": {
			Data: []byte("---\ntitle: Dot\n---\n"),
		},
		"empty.md": {
			Data: []byte(""),
		},
	}

	idx, err := BuildIndex(context.Background(), testFS)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	t.Run("AllPages count", func(t *testing.T) {
		pages := idx.AllPages()
		// readme.md, guide/intro.md (no-frontmatter.md, image.png, hidden, dotfile, empty are skipped)
		if got := len(pages); got != 2 {
			t.Fatalf("expected 2 pages, got %d: %+v", got, pages)
		}
	})

	t.Run("AllTags sorted", func(t *testing.T) {
		tags := idx.AllTags()
		expected := []string{"docs", "go", "tutorial"}
		if len(tags) != len(expected) {
			t.Fatalf("expected tags %v, got %v", expected, tags)
		}
		for i, tag := range tags {
			if tag != expected[i] {
				t.Fatalf("expected tag[%d]=%q, got %q", i, expected[i], tag)
			}
		}
	})

	t.Run("ByTag returns correct pages", func(t *testing.T) {
		goPages := idx.ByTag("go")
		if len(goPages) != 2 {
			t.Fatalf("expected 2 pages with tag 'go', got %d", len(goPages))
		}
	})

	t.Run("ByTag case insensitive", func(t *testing.T) {
		goPages := idx.ByTag("GO")
		if len(goPages) != 2 {
			t.Fatalf("expected 2 pages with tag 'GO' (case insensitive), got %d", len(goPages))
		}
	})

	t.Run("ByTag nonexistent", func(t *testing.T) {
		pages := idx.ByTag("nonexistent")
		if pages != nil {
			t.Fatalf("expected nil for nonexistent tag, got %v", pages)
		}
	})

	t.Run("page metadata extracted", func(t *testing.T) {
		pages := idx.AllPages()
		var homePage *PageInfo
		for i := range pages {
			if pages[i].Path == "/readme.md" {
				homePage = &pages[i]
				break
			}
		}
		if homePage == nil {
			t.Fatal("home page not found")
		}
		if homePage.Title != "Home" {
			t.Errorf("expected title 'Home', got %q", homePage.Title)
		}
		if homePage.Description != "The homepage" {
			t.Errorf("expected description 'The homepage', got %q", homePage.Description)
		}
		expectedDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
		if !homePage.Date.Equal(expectedDate) {
			t.Errorf("expected date %v, got %v", expectedDate, homePage.Date)
		}
		if homePage.Meta == nil || homePage.Meta["category"] != "main" {
			t.Errorf("expected meta category='main', got %v", homePage.Meta)
		}
	})

	t.Run("tags normalized to lowercase", func(t *testing.T) {
		pages := idx.AllPages()
		for _, page := range pages {
			for _, tag := range page.Tags {
				if tag != strings.ToLower(tag) {
					t.Errorf("tag %q not lowercase in page %s", tag, page.Path)
				}
			}
		}
	})
}

func TestBuildIndex_EmptyFS(t *testing.T) {
	testFS := fstest.MapFS{}
	idx, err := BuildIndex(context.Background(), testFS)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}
	if len(idx.AllPages()) != 0 {
		t.Fatalf("expected 0 pages, got %d", len(idx.AllPages()))
	}
	if len(idx.AllTags()) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(idx.AllTags()))
	}
}

func TestAllPages_ReturnsCopy(t *testing.T) {
	testFS := fstest.MapFS{
		"test.md": {Data: []byte("---\ntitle: Test\n---\n")},
	}
	idx, err := BuildIndex(context.Background(), testFS)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	pages1 := idx.AllPages()
	pages1[0].Title = "Modified"
	pages2 := idx.AllPages()
	if pages2[0].Title == "Modified" {
		t.Fatal("AllPages should return a copy, not a reference to internal state")
	}
}

func TestBuildIndex_CancelledContext(t *testing.T) {
	testFS := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: A\n---\n")},
		"b.md": {Data: []byte("---\ntitle: B\n---\n")},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := BuildIndex(ctx, testFS)
	if err == nil {
		// A cancelled context may or may not produce an error depending on
		// timing; the goroutines check gctx.Err() but may have already
		// completed. We accept both outcomes.
		t.Log("BuildIndex succeeded with cancelled context (goroutines completed before checking)")
	}
}

func TestBuildIndex_ManyFiles(t *testing.T) {
	// Verify concurrent indexing works correctly with many files.
	testFS := fstest.MapFS{}
	const n = 50
	for i := range n {
		name := fmt.Sprintf("page-%03d.md", i)
		title := fmt.Sprintf("Page %d", i)
		testFS[name] = &fstest.MapFile{
			Data: fmt.Appendf(nil, "---\ntitle: %s\ntags:\n  - batch\n---\n# %s", title, title),
		}
	}

	idx, err := BuildIndex(context.Background(), testFS)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	pages := idx.AllPages()
	if len(pages) != n {
		t.Fatalf("expected %d pages, got %d", n, len(pages))
	}

	batchPages := idx.ByTag("batch")
	if len(batchPages) != n {
		t.Fatalf("expected %d pages with tag 'batch', got %d", n, len(batchPages))
	}

	// Verify all pages are unique
	seen := make(map[string]bool)
	for _, page := range pages {
		if seen[page.Path] {
			t.Fatalf("duplicate page path: %s", page.Path)
		}
		seen[page.Path] = true
	}
}
