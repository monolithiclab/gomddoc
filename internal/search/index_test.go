package search

import (
	"context"
	"fmt"
	"maps"
	"math"
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
			// A rune-counting threshold drops "文" and turns this red.
			name:  "keeps a single CJK ideograph",
			input: "a 文 ab",
			want:  []string{"文", "ab"},
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
func buildTaggedIndex(tb testing.TB, files fstest.MapFS) *Index {
	tb.Helper()
	metaIndex, err := metadata.BuildIndex(context.Background(), files, nil)
	if err != nil {
		tb.Fatalf("metadata.BuildIndex failed: %v", err)
	}
	idx, err := BuildIndex(context.Background(), files, metaIndex, nil)
	if err != nil {
		tb.Fatalf("BuildIndex failed: %v", err)
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

// mergeCorpus builds a deterministic n-document corpus whose vocabulary is
// spread unevenly: "common" is in every body, the eight base words each land in
// roughly a third of the bodies, and every document repeats one of them in its
// title and two more in its description. That mix is what exercises the posting
// merge — long lists next to short ones, and documents carrying body, title and
// description postings for the same term.
func mergeCorpus(tb testing.TB, n int) *Index {
	tb.Helper()

	words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta"}
	files := make(fstest.MapFS, n)
	for i := range n {
		var b strings.Builder
		fmt.Fprintf(&b, "---\ntitle: %s Document %d\ndescription: About %s and %s\ntags:\n  - t%d\n  - every\n---\n",
			words[i%len(words)], i, words[(i+1)%len(words)], words[(i+3)%len(words)], i%5)
		b.WriteString("# Heading\n\ncommon prose ")
		for j, w := range words {
			if (i+j)%3 == 0 {
				fmt.Fprintf(&b, "%s %s ", w, w)
			}
		}
		fmt.Fprintf(&b, "\n\nBody number %d with searchable text.\n", i)
		files[fmt.Sprintf("doc%04d.md", i)] = &fstest.MapFile{Data: []byte(b.String())}
	}
	return buildTaggedIndex(tb, files)
}

// referenceSearch reproduces the pre-merge candidate selection and scoring: a
// map[int][]posting built per query token, candidates collected into a set, and
// every candidate scored by looking its postings back up. It is the oracle the
// linear merge in Search is checked against.
//
// It shares Search's tag preamble and result materialization deliberately —
// this diff did not change those, and a hand-copy would drift. What it does not
// share is the part under test: candidate selection and score accumulation.
func referenceSearch(idx *Index, query string, limit int) []SearchResult {
	tagFilters, freeText := parseQuery(query)

	var tagged []metadata.PageInfo
	if len(tagFilters) > 0 {
		if tagged = idx.taggedPages(tagFilters); len(tagged) == 0 {
			return []SearchResult{}
		}
	}

	queryTokens := tokenize(freeText)
	if len(queryTokens) == 0 {
		if len(tagFilters) > 0 {
			return idx.tagOnlyResults(tagged, limit)
		}
		return []SearchResult{}
	}

	var tagSet map[int]struct{}
	if len(tagFilters) > 0 {
		if tagSet = idx.tagDocSet(tagged); len(tagSet) == 0 {
			return []SearchResult{}
		}
	}

	idfs := make([]float64, len(queryTokens))
	byDoc := make([]map[int][]posting, len(queryTokens))
	candidates := map[int]struct{}{}
	for i, token := range queryTokens {
		posts, ok := idx.inverted[token]
		if !ok {
			return []SearchResult{}
		}
		byDoc[i] = map[int][]posting{}
		for _, p := range posts {
			byDoc[i][p.docIdx] = append(byDoc[i][p.docIdx], p)
		}
		idfs[i] = math.Log(float64(idx.docCount) / float64(len(byDoc[i])))

		if i == 0 {
			for docIdx := range byDoc[i] {
				if tagSet != nil {
					if _, ok := tagSet[docIdx]; !ok {
						continue
					}
				}
				candidates[docIdx] = struct{}{}
			}
		} else {
			for docIdx := range candidates {
				if _, ok := byDoc[i][docIdx]; !ok {
					delete(candidates, docIdx)
				}
			}
		}
		if len(candidates) == 0 {
			return []SearchResult{}
		}
	}

	scored := make([]scoredDoc, 0, len(candidates))
	for _, docIdx := range slices.Sorted(maps.Keys(candidates)) {
		s := scoredDoc{docIdx: docIdx}
		for i := range queryTokens {
			for _, p := range byDoc[i][docIdx] {
				switch p.field {
				case fieldBody:
					s.score += float64(p.freq) / float64(idx.docTermCounts[docIdx]) * idfs[i]
				case fieldTitle:
					s.score += 3.0 * idfs[i]
				case fieldDesc:
					s.score += 1.5 * idfs[i]
				}
			}
		}
		scored = append(scored, s)
	}

	// Rank by sorting everything and truncating — the shape rankTopN's window
	// replaces. Result materialization is shared: this diff did not change it.
	slices.SortFunc(scored, compareScored)
	return idx.buildResults(scored[:min(limit, len(scored))], queryTokens)
}

func TestSearch_MergeMatchesReference(t *testing.T) {
	t.Parallel()
	idx := mergeCorpus(t, 60)

	// Every shape the merge has to handle: a token matching the whole corpus,
	// tokens matching a subset, multi-token AND down to an empty intersection,
	// unknown tokens, and both mixed and tag-only queries.
	queries := []string{
		"common",
		"alpha",
		"common alpha",
		"alpha beta",
		"alpha beta gamma",
		"common alpha beta gamma delta epsilon zeta eta theta",
		"document",
		"nonexistent",
		"common nonexistent",
		"tag:t1 alpha",
		"tag:every common",
		"tag:t1 tag:t2 common",
		"tag:nope common",
		"tag:t1",
		"",
	}

	// limit=3 exercises the top-N window with evictions, limit=1000 the
	// fill-only path where every candidate is kept.
	for _, limit := range []int{3, 1000} {
		for _, q := range queries {
			t.Run(fmt.Sprintf("%s/limit=%d", q, limit), func(t *testing.T) {
				t.Parallel()
				got, want := idx.Search(q, limit), referenceSearch(idx, q, limit)
				if !slices.Equal(got, want) {
					t.Errorf("Search(%q, %d):\n got %+v\nwant %+v", q, limit, got, want)
				}
			})
		}
	}
}

// The merge walks each posting list once against an ascending candidate list;
// a token whose postings all sort before (or all after) the surviving
// candidates must neither match nor run off the end of either slice.
func TestSearch_MergeDisjointPostings(t *testing.T) {
	t.Parallel()
	idx := buildTaggedIndex(t, fstest.MapFS{
		"a.md": &fstest.MapFile{Data: []byte("---\ntitle: A\n---\nonly-in-first shared-token\n")},
		"b.md": &fstest.MapFile{Data: []byte("---\ntitle: B\n---\nshared-token\n")},
		"c.md": &fstest.MapFile{Data: []byte("---\ntitle: C\n---\nshared-token only-in-last\n")},
	})

	tests := []struct {
		query string
		want  []string
	}{
		{"shared-token only-in-first", []string{"/a.md"}},
		{"only-in-first shared-token", []string{"/a.md"}},
		{"shared-token only-in-last", []string{"/c.md"}},
		{"only-in-last shared-token", []string{"/c.md"}},
		{"only-in-first only-in-last", nil},
		{"shared-token", []string{"/a.md", "/b.md", "/c.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			t.Parallel()
			// Not sorted: the docIdx tiebreaker makes the order deterministic,
			// and every doc here scores identically on "shared-token".
			if got := resultPaths(idx.Search(tt.query, 10)); !slices.Equal(got, tt.want) {
				t.Errorf("Search(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

// BuildIndex's ordering guarantee, asserted where it is established rather than
// inferred from query results: Search's linear merges are wrong without it.
func TestBuildIndex_PostingsSorted(t *testing.T) {
	t.Parallel()
	idx := mergeCorpus(t, 30)

	if len(idx.inverted) == 0 {
		t.Fatal("empty index")
	}
	for term, posts := range idx.inverted {
		seen := make(map[int]bool, len(posts))
		for j, p := range posts {
			if j > 0 && p.docIdx < posts[j-1].docIdx {
				t.Fatalf("%q posting %d: docIdx %d after %d — not ascending", term, j, p.docIdx, posts[j-1].docIdx)
			}
			// A document's postings must also be consecutive: distinctDocs and
			// seekDoc both stop at the first docIdx change.
			if j > 0 && p.docIdx != posts[j-1].docIdx && seen[p.docIdx] {
				t.Fatalf("%q posting %d: docIdx %d recurs after a gap", term, j, p.docIdx)
			}
			seen[p.docIdx] = true
		}
	}
}

// Equally-scored hits must order by docIdx, not by whatever the unstable sort
// leaves behind: serve and build would otherwise disagree on the same query.
func TestSearch_EqualScoresOrderByDocIndex(t *testing.T) {
	t.Parallel()
	// Identical bodies and titles, so all three score identically. Doc indices
	// follow the walk order, which is alphabetical.
	body := "---\ntitle: Same Title\n---\n# Same Title\n\nidentical body text here.\n"
	idx := buildTaggedIndex(t, fstest.MapFS{
		"c.md": &fstest.MapFile{Data: []byte(body)},
		"a.md": &fstest.MapFile{Data: []byte(body)},
		"b.md": &fstest.MapFile{Data: []byte(body)},
	})

	results := idx.Search("identical", 10)
	if got := resultPaths(results); !slices.Equal(got, []string{"/a.md", "/b.md", "/c.md"}) {
		t.Errorf("tied results = %v, want ascending doc order", got)
	}
	for _, r := range results[1:] {
		if r.Score != results[0].Score {
			t.Fatalf("fixture is not tied: %v", results)
		}
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

func TestSearch_MultiTagAndIntersection(t *testing.T) {
	t.Parallel()
	idx := buildTaggedIndex(t, fstest.MapFS{
		"ab.md":  &fstest.MapFile{Data: []byte("---\ntitle: AB\ntags:\n  - alpha\n  - beta\n---\n# AB\n\nAlpha beta body.")},
		"a.md":   &fstest.MapFile{Data: []byte("---\ntitle: A\ntags:\n  - alpha\n---\n# A\n\nAlpha only body.")},
		"b.md":   &fstest.MapFile{Data: []byte("---\ntitle: B\ntags:\n  - beta\n---\n# B\n\nBeta only body.")},
		"abc.md": &fstest.MapFile{Data: []byte("---\ntitle: ABC\ntags:\n  - alpha\n  - beta\n  - gamma\n---\n# ABC\n\nAlpha beta gamma body.")},
	})

	// Two tags: only pages carrying BOTH alpha and beta survive the narrowing.
	got := resultPaths(idx.Search("tag:alpha tag:beta", 10))
	slices.Sort(got)
	if want := []string{"/ab.md", "/abc.md"}; !slices.Equal(got, want) {
		t.Errorf("tag:alpha tag:beta = %v, want %v", got, want)
	}

	// Three tags: the chain narrows further to the single page with all three.
	got = resultPaths(idx.Search("tag:alpha tag:beta tag:gamma", 10))
	if want := []string{"/abc.md"}; !slices.Equal(got, want) {
		t.Errorf("tag:alpha tag:beta tag:gamma = %v, want %v", got, want)
	}

	// A second tag that no page carries collapses the intersection to empty,
	// even though the first tag matches pages.
	if got := idx.Search("tag:alpha tag:nonexistent", 10); len(got) != 0 {
		t.Errorf("tag:alpha tag:nonexistent = %v, want empty", resultPaths(got))
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
