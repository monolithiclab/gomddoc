package search

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/metadata"
)

func TestStripFrontmatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no frontmatter",
			input: "# Hello\n\nSome content",
			want:  "# Hello\n\nSome content",
		},
		{
			name:  "with frontmatter",
			input: "---\ntitle: Test\n---\n# Hello\n\nContent",
			want:  "# Hello\n\nContent",
		},
		{
			name:  "with BOM and frontmatter",
			input: "\xef\xbb\xbf---\ntitle: Test\n---\nBody",
			want:  "Body",
		},
		{
			name:  "unclosed frontmatter",
			input: "---\ntitle: Test\nNo closing",
			want:  "---\ntitle: Test\nNo closing",
		},
		{
			name:  "empty body after frontmatter",
			input: "---\ntitle: Test\n---\n",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := string(stripFrontmatter([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("stripFrontmatter() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripMarkdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "headings",
			input: "# Title\n## Subtitle",
			want:  "Title\nSubtitle",
		},
		{
			name:  "links",
			input: "See [the docs](https://example.com) for details",
			want:  "See the docs for details",
		},
		{
			name:  "images",
			input: "![alt text](image.png)",
			want:  "alt text",
		},
		{
			name:  "emphasis",
			input: "This is **bold** and *italic*",
			want:  "This is bold and italic",
		},
		{
			name:  "inline code",
			input: "Use the `fmt.Println` function",
			want:  "Use the fmt.Println function",
		},
		{
			name:  "code fence",
			input: "Before\n```go\nfmt.Println(\"hi\")\n```\nAfter",
			want:  "Before\n \nAfter",
		},
		{
			name:  "blockquotes",
			input: "> This is quoted\n> text",
			want:  "This is quoted\ntext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple words",
			input: "Hello World",
			want:  []string{"hello", "world"},
		},
		{
			name:  "filters short tokens",
			input: "I am a test",
			want:  []string{"am", "test"},
		},
		{
			name:  "handles punctuation",
			input: "hello-world foo_bar",
			want:  []string{"hello", "world", "foo", "bar"},
		},
		{
			name:  "preserves numbers",
			input: "Go 1.25 release",
			want:  []string{"go", "25", "release"},
		},
		{
			name:  "empty input",
			input: "",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tokenize(tt.input)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("tokenize() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("tokenize()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTokenizeToFreqs(t *testing.T) {
	t.Parallel()

	freqs, total := tokenizeToFreqs("hello world hello")
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if freqs["hello"] != 2 {
		t.Errorf("freq[hello] = %d, want 2", freqs["hello"])
	}
	if freqs["world"] != 1 {
		t.Errorf("freq[world] = %d, want 1", freqs["world"])
	}
}

func TestGenerateSnippet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		queryTokens []string
		maxLen      int
		wantMark    bool   // expect <mark> tags
		wantSubstr  string // expected substring in result
	}{
		{
			name:        "highlights match",
			content:     "The quick brown fox jumps over the lazy dog",
			queryTokens: []string{"fox"},
			maxLen:      100,
			wantMark:    true,
			wantSubstr:  "<mark>fox</mark>",
		},
		{
			name:        "empty query",
			content:     "Some content here",
			queryTokens: []string{},
			maxLen:      100,
			wantMark:    false,
			wantSubstr:  "Some content here",
		},
		{
			name:        "empty content",
			content:     "",
			queryTokens: []string{"test"},
			maxLen:      100,
			wantMark:    false,
			wantSubstr:  "",
		},
		{
			name:        "truncation adds ellipsis",
			content:     strings.Repeat("word ", 100),
			queryTokens: []string{"nonexistent"},
			maxLen:      50,
			wantMark:    false,
			wantSubstr:  "...",
		},
		{
			name:        "html escapes non-match text",
			content:     "Test <script>alert('xss')</script> content match",
			queryTokens: []string{"match"},
			maxLen:      200,
			wantMark:    true,
			wantSubstr:  "&lt;script&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := generateSnippet(tt.content, tt.queryTokens, tt.maxLen)
			if tt.wantMark && !strings.Contains(got, "<mark>") {
				t.Errorf("expected <mark> tags in snippet: %q", got)
			}
			if tt.wantSubstr != "" && !strings.Contains(got, tt.wantSubstr) {
				t.Errorf("expected %q in snippet, got: %q", tt.wantSubstr, got)
			}
		})
	}
}

func TestBuildIndex(t *testing.T) {
	t.Parallel()

	contentRoot := fstest.MapFS{
		"README.md":    &fstest.MapFile{Data: []byte("---\ntitle: Home\ndescription: Welcome page\n---\n# Welcome\n\nThis is the home page with important content.")},
		"guide.md":     &fstest.MapFile{Data: []byte("# Getting Started\n\nLearn how to get started with the project.")},
		"reference.md": &fstest.MapFile{Data: []byte("# API Reference\n\nThe API provides endpoints for managing resources.")},
		"image.png":    &fstest.MapFile{Data: []byte("not markdown")},
		".hidden/x.md": &fstest.MapFile{Data: []byte("# Hidden")},
		".secret.md":   &fstest.MapFile{Data: []byte("# Secret")},
	}

	metaIndex, err := metadata.BuildIndex(context.Background(), contentRoot, nil)
	if err != nil {
		t.Fatalf("metadata.BuildIndex failed: %v", err)
	}

	idx, err := BuildIndex(context.Background(), contentRoot, metaIndex, nil)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	if idx.docCount != 3 {
		t.Errorf("docCount = %d, want 3", idx.docCount)
	}

	// Verify inverted index contains expected terms
	if _, ok := idx.inverted["welcome"]; !ok {
		t.Error("expected 'welcome' in inverted index")
	}
	if _, ok := idx.inverted["started"]; !ok {
		t.Error("expected 'started' in inverted index")
	}
	if _, ok := idx.inverted["api"]; !ok {
		t.Error("expected 'api' in inverted index")
	}
}

func TestBuildIndexSkipsHidden(t *testing.T) {
	t.Parallel()

	contentRoot := fstest.MapFS{
		"visible.md":   &fstest.MapFile{Data: []byte("# Visible\n\nThis page is visible.")},
		".hidden.md":   &fstest.MapFile{Data: []byte("# Hidden\n\nThis page is hidden.")},
		".dir/page.md": &fstest.MapFile{Data: []byte("# In Hidden Dir\n\nAlso hidden.")},
	}

	idx, err := BuildIndex(context.Background(), contentRoot, nil, nil)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	if idx.docCount != 1 {
		t.Errorf("docCount = %d, want 1 (only visible.md)", idx.docCount)
	}
}

func TestSearch(t *testing.T) {
	t.Parallel()

	contentRoot := fstest.MapFS{
		"README.md":    &fstest.MapFile{Data: []byte("---\ntitle: Home\ndescription: The home page\n---\n# Welcome Home\n\nThis is the home page for the project.")},
		"guide.md":     &fstest.MapFile{Data: []byte("# Getting Started Guide\n\nLearn how to get started with the project quickly.")},
		"reference.md": &fstest.MapFile{Data: []byte("# API Reference\n\nThe API provides endpoints for managing resources and data.")},
		"advanced.md":  &fstest.MapFile{Data: []byte("# Advanced Topics\n\nAdvanced configuration and performance tuning for the project.")},
	}

	metaIndex, err := metadata.BuildIndex(context.Background(), contentRoot, nil)
	if err != nil {
		t.Fatalf("metadata.BuildIndex failed: %v", err)
	}

	idx, err := BuildIndex(context.Background(), contentRoot, metaIndex, nil)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	t.Run("single term", func(t *testing.T) {
		t.Parallel()
		results := idx.Search("project", 10)
		if len(results) == 0 {
			t.Fatal("expected results for 'project'")
		}
		// All docs mentioning "project" should be returned
		for _, r := range results {
			if r.Path == "" || r.Title == "" {
				t.Errorf("result missing path or title: %+v", r)
			}
			if r.Score <= 0 {
				t.Errorf("result should have positive score: %+v", r)
			}
		}
	})

	t.Run("title boost", func(t *testing.T) {
		t.Parallel()
		results := idx.Search("api", 10)
		if len(results) == 0 {
			t.Fatal("expected results for 'api'")
		}
		// The API Reference page should rank first due to title boost
		if results[0].Path != "/reference.md" {
			t.Errorf("expected /reference.md first (title boost), got %s", results[0].Path)
		}
	})

	t.Run("multi-term AND", func(t *testing.T) {
		t.Parallel()
		results := idx.Search("getting started", 10)
		if len(results) == 0 {
			t.Fatal("expected results for 'getting started'")
		}
		// Only guide.md contains both "getting" and "started"
		if results[0].Path != "/guide.md" {
			t.Errorf("expected /guide.md first, got %s", results[0].Path)
		}
	})

	t.Run("limit", func(t *testing.T) {
		t.Parallel()
		results := idx.Search("project", 1)
		if len(results) != 1 {
			t.Errorf("expected 1 result with limit=1, got %d", len(results))
		}
	})

	t.Run("no results", func(t *testing.T) {
		t.Parallel()
		results := idx.Search("nonexistentterm", 10)
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("empty query", func(t *testing.T) {
		t.Parallel()
		results := idx.Search("", 10)
		if len(results) != 0 {
			t.Errorf("expected 0 results for empty query, got %d", len(results))
		}
	})

	t.Run("results have snippets", func(t *testing.T) {
		t.Parallel()
		results := idx.Search("home", 10)
		if len(results) == 0 {
			t.Fatal("expected results for 'home'")
		}
		if results[0].Snippet == "" {
			t.Error("expected non-empty snippet")
		}
	})

	t.Run("description boost", func(t *testing.T) {
		t.Parallel()
		results := idx.Search("home", 10)
		if len(results) == 0 {
			t.Fatal("expected results for 'home'")
		}
		// README.md has "home" in title AND description, should rank highest
		if results[0].Path != "/README.md" {
			t.Errorf("expected /README.md first (title+description boost), got %s", results[0].Path)
		}
	})
}

func TestExtractFirstHeading(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"h1", "# Hello World", "Hello World"},
		{"h2", "## Section Title", "Section Title"},
		{"no heading", "Just text", ""},
		{"heading after text", "text\n# Title", "Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractFirstHeading([]byte(tt.input))
			if got != tt.want {
				t.Errorf("extractFirstHeading() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want string
	}{
		{"README.md", "README"},
		{"getting-started.md", "Getting started"},
		{"api_reference.md", "Api reference"},
		{"docs/guide.md", "Guide"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := deriveTitle(tt.path)
			if got != tt.want {
				t.Errorf("deriveTitle(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
