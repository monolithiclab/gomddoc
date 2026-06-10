package search

import (
	"context"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/metadata"
)

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

	// Verify behavioral expectations through Search rather than internal fields
	for _, term := range []string{"welcome", "started", "api"} {
		results := idx.Search(term, 10)
		if len(results) == 0 {
			t.Errorf("Search(%q) returned no results, want at least one", term)
		}
	}

	// Verify all 3 markdown files were indexed (not png or hidden)
	all := idx.Search("the", 10) // common word appearing in all docs
	if len(all) != 3 {
		t.Errorf("Search(\"the\") returned %d results, want 3", len(all))
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

	// "visible" only appears in visible.md; hidden files should not be indexed
	results := idx.Search("visible", 10)
	if len(results) != 1 {
		t.Errorf("Search(\"visible\") returned %d results, want 1 (only visible.md)", len(results))
	}

	// Hidden content should not appear
	hidden := idx.Search("hidden", 10)
	if len(hidden) != 0 {
		t.Errorf("Search(\"hidden\") returned %d results, want 0", len(hidden))
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

func TestParseQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantTags []string
		wantText string
	}{
		{"no tag terms", "foo bar", nil, "foo bar"},
		{"single tag", "tag:foo", []string{"foo"}, ""},
		{"multiple tags", "tag:foo tag:bar", []string{"foo", "bar"}, ""},
		{"mixed", "tag:foo bar baz", []string{"foo"}, "bar baz"},
		{"empty tag token dropped", "tag: kubernetes", nil, "kubernetes"},
		{"colon-prefixed not a tag", ":foo", nil, ":foo"},
		{"uppercase lowercased", "tag:Foo", []string{"foo"}, ""},
		{"whitespace only", "   ", nil, ""},
		{"double colon", "tag::foo", []string{":foo"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotTags, gotText := parseQuery(tt.input)
			if !slices.Equal(gotTags, tt.wantTags) {
				t.Errorf("tags = %v, want %v", gotTags, tt.wantTags)
			}
			if gotText != tt.wantText {
				t.Errorf("freeText = %q, want %q", gotText, tt.wantText)
			}
		})
	}
}

// buildTaggedIndex builds a search index (with its backing metadata index)
// from the given files. Shared setup for the tag: search tests.
func buildTaggedIndex(t *testing.T, files fstest.MapFS) *Index {
	t.Helper()
	metaIndex, err := metadata.BuildIndex(context.Background(), files, nil)
	if err != nil {
		t.Fatalf("metadata.BuildIndex failed: %v", err)
	}
	idx, err := BuildIndex(context.Background(), files, metaIndex, nil)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}
	return idx
}

// taggedSite is the standard fixture: two pages tagged "shared", one tagged
// "other"; only deploy.md mentions kubernetes.
func taggedSite() fstest.MapFS {
	return fstest.MapFS{
		"deploy.md": &fstest.MapFile{Data: []byte("---\ntitle: Deploy\ntags:\n  - shared\n  - deployment\n---\n# Deploy\n\nDeploying with kubernetes clusters.")},
		"config.md": &fstest.MapFile{Data: []byte("---\ntitle: Config\ntags:\n  - shared\n---\n# Config\n\nConfiguration settings and options.")},
		"other.md":  &fstest.MapFile{Data: []byte("---\ntitle: Other\ntags:\n  - other\n---\n# Other\n\nSome entirely unrelated content here.")},
	}
}

func resultPaths(results []SearchResult) []string {
	paths := make([]string, len(results))
	for i, r := range results {
		paths[i] = r.Path
	}
	return paths
}

func TestSearch_TagFilter(t *testing.T) {
	t.Parallel()
	idx := buildTaggedIndex(t, taggedSite())

	shared := resultPaths(idx.Search("tag:shared", 10))
	slices.Sort(shared)
	if want := []string{"/config.md", "/deploy.md"}; !slices.Equal(shared, want) {
		t.Errorf("tag:shared = %v, want %v", shared, want)
	}

	other := resultPaths(idx.Search("tag:other", 10))
	if want := []string{"/other.md"}; !slices.Equal(other, want) {
		t.Errorf("tag:other = %v, want %v", other, want)
	}
}

func TestSearch_UnknownTagReturnsEmpty(t *testing.T) {
	t.Parallel()
	idx := buildTaggedIndex(t, taggedSite())

	if results := idx.Search("tag:doesnotexist", 10); len(results) != 0 {
		t.Errorf("expected 0 results for unknown tag, got %d", len(results))
	}
}

func TestSearch_TagOnlyAlphabetical(t *testing.T) {
	t.Parallel()
	idx := buildTaggedIndex(t, fstest.MapFS{
		"z.md": &fstest.MapFile{Data: []byte("---\ntitle: Zebra\ntags:\n  - topic\n---\n# Zebra\n\nZebra body content.")},
		"a.md": &fstest.MapFile{Data: []byte("---\ntitle: Alpha\ntags:\n  - topic\n---\n# Alpha\n\nAlpha body content.")},
		"m.md": &fstest.MapFile{Data: []byte("---\ntitle: Mango\ntags:\n  - topic\n---\n# Mango\n\nMango body content.")},
	})

	titles := make([]string, 0, 3)
	for _, r := range idx.Search("tag:topic", 10) {
		titles = append(titles, r.Title)
	}
	if want := []string{"Alpha", "Mango", "Zebra"}; !slices.Equal(titles, want) {
		t.Errorf("tag-only order = %v, want %v", titles, want)
	}
}

func TestSearch_TagOnlyStableOrder(t *testing.T) {
	t.Parallel()
	idx := buildTaggedIndex(t, fstest.MapFS{
		"b.md": &fstest.MapFile{Data: []byte("---\ntitle: Same\ntags:\n  - dup\n---\n# Same\n\nFirst body.")},
		"a.md": &fstest.MapFile{Data: []byte("---\ntitle: Same\ntags:\n  - dup\n---\n# Same\n\nSecond body.")},
	})

	if got := resultPaths(idx.Search("tag:dup", 10)); !slices.Equal(got, []string{"/a.md", "/b.md"}) {
		t.Errorf("equal-title order = %v, want path-sorted [/a.md /b.md]", got)
	}
}

func TestSearch_TagPlusFreeText(t *testing.T) {
	t.Parallel()
	idx := buildTaggedIndex(t, taggedSite())

	results := idx.Search("tag:shared kubernetes", 10)
	if got := resultPaths(results); !slices.Equal(got, []string{"/deploy.md"}) {
		t.Errorf("tag:shared kubernetes = %v, want [/deploy.md]", got)
	}
}

func TestSearch_TagCaseInsensitive(t *testing.T) {
	t.Parallel()
	idx := buildTaggedIndex(t, taggedSite())

	if got := len(idx.Search("tag:Shared", 10)); got != 2 {
		t.Errorf("tag:Shared returned %d results, want 2", got)
	}
}

func TestSearch_EmptyTagTokenIgnored(t *testing.T) {
	t.Parallel()
	idx := buildTaggedIndex(t, taggedSite())

	// "tag:" is dropped; "kubernetes" is the only (free-text) term.
	results := idx.Search("tag: kubernetes", 10)
	if got := resultPaths(results); !slices.Equal(got, []string{"/deploy.md"}) {
		t.Errorf("'tag: kubernetes' = %v, want [/deploy.md]", got)
	}
}

func TestSearch_TagOnlyIncludesBodylessPage(t *testing.T) {
	t.Parallel()
	// stub.md is tagged "shared" but has no body, so it is absent from the
	// search corpus. Tag-only queries resolve against the metadata index, so it
	// still appears — consistent with the /tags/{tag} listing page. Its title
	// comes from frontmatter (no indexed heading fallback available).
	files := taggedSite()
	files["stub.md"] = &fstest.MapFile{Data: []byte("---\ntitle: Stub\ntags:\n  - shared\n---\n")}
	idx := buildTaggedIndex(t, files)

	paths := resultPaths(idx.Search("tag:shared", 10))
	slices.Sort(paths)
	if want := []string{"/config.md", "/deploy.md", "/stub.md"}; !slices.Equal(paths, want) {
		t.Errorf("tag:shared = %v, want %v (bodyless stub included)", paths, want)
	}
}

func TestSearch_MixedQueryExcludesBodylessPage(t *testing.T) {
	t.Parallel()
	// A bodyless page has no indexed body, so it can never match the free-text
	// portion of a mixed query even though it carries the tag.
	files := taggedSite()
	files["stub.md"] = &fstest.MapFile{Data: []byte("---\ntitle: Stub\ntags:\n  - shared\n---\n")}
	idx := buildTaggedIndex(t, files)

	if got := resultPaths(idx.Search("tag:shared kubernetes", 10)); !slices.Equal(got, []string{"/deploy.md"}) {
		t.Errorf("tag:shared kubernetes = %v, want [/deploy.md]", got)
	}
}

func TestSearch_TagOnlySnippetEmptyDescriptionSet(t *testing.T) {
	t.Parallel()
	idx := buildTaggedIndex(t, fstest.MapFS{
		"p.md": &fstest.MapFile{Data: []byte("---\ntitle: Page\ndescription: A short description\ntags:\n  - topic\n---\n# Page\n\nBody text here.")},
	})

	results := idx.Search("tag:topic", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Snippet != "" {
		t.Errorf("tag-only snippet = %q, want empty", results[0].Snippet)
	}
	if results[0].Description != "A short description" {
		t.Errorf("description = %q, want %q", results[0].Description, "A short description")
	}
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
