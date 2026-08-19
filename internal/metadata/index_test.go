package metadata

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/monolithiclab/gomddoc/internal/testutil/logcapture"
	"github.com/monolithiclab/gomddoc/internal/testutil/markdown"
)

func TestExtractFrontmatter(t *testing.T) {
	t.Parallel()

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
			content: markdown.ValidFrontmatterWithBody,
			wantKey: "title",
			wantVal: "Hello World",
		},
		{
			name:    "no frontmatter",
			content: markdown.SimpleHeading,
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
			name:    "invalid YAML",
			content: markdown.MalformedFrontmatter,
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
			t.Parallel()
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
	t.Parallel()

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

	idx, err := BuildIndex(context.Background(), testFS, nil)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	t.Run("AllPages count", func(t *testing.T) {
		t.Parallel()
		pages := idx.AllPages()
		// readme.md, guide/intro.md (no-frontmatter.md, image.png, hidden, dotfile, empty are skipped)
		if got := len(pages); got != 2 {
			t.Fatalf("expected 2 pages, got %d: %+v", got, pages)
		}
	})

	t.Run("AllTags sorted", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
		goPages := idx.ByTag("go")
		if len(goPages) != 2 {
			t.Fatalf("expected 2 pages with tag 'go', got %d", len(goPages))
		}
	})

	t.Run("ByTag case insensitive", func(t *testing.T) {
		t.Parallel()
		goPages := idx.ByTag("GO")
		if len(goPages) != 2 {
			t.Fatalf("expected 2 pages with tag 'GO' (case insensitive), got %d", len(goPages))
		}
	})

	t.Run("ByTag nonexistent", func(t *testing.T) {
		t.Parallel()
		pages := idx.ByTag("nonexistent")
		if pages != nil {
			t.Fatalf("expected nil for nonexistent tag, got %v", pages)
		}
	})

	t.Run("page metadata extracted", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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

func TestByPath(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"readme.md": {
			Data: []byte("---\ntitle: Home\ntags:\n  - go\n---\n# Home"),
		},
		"guide/intro.md": {
			Data: []byte("---\ntitle: Intro\n---\n# Intro"),
		},
	}

	idx, err := BuildIndex(context.Background(), testFS, nil)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	t.Run("existing path", func(t *testing.T) {
		t.Parallel()
		page := idx.ByPath("/readme.md")
		if page == nil {
			t.Fatal("expected non-nil page for /readme.md")
		}
		if page.Title != "Home" {
			t.Errorf("Title = %q, want %q", page.Title, "Home")
		}
	})

	t.Run("nested path", func(t *testing.T) {
		t.Parallel()
		page := idx.ByPath("/guide/intro.md")
		if page == nil {
			t.Fatal("expected non-nil page for /guide/intro.md")
		}
		if page.Title != "Intro" {
			t.Errorf("Title = %q, want %q", page.Title, "Intro")
		}
	})

	t.Run("nonexistent path", func(t *testing.T) {
		t.Parallel()
		page := idx.ByPath("/nonexistent.md")
		if page != nil {
			t.Fatalf("expected nil for nonexistent path, got %+v", page)
		}
	})

	t.Run("returns copy not reference", func(t *testing.T) {
		t.Parallel()
		page1 := idx.ByPath("/readme.md")
		page1.Title = "Modified"
		page2 := idx.ByPath("/readme.md")
		if page2.Title == "Modified" {
			t.Fatal("ByPath should return a copy, not a reference to internal state")
		}
	})
}

func TestIndex_TitleForPath(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"readme.md":      &fstest.MapFile{Data: []byte("---\ntitle: Home\n---\n# Home")},
		"guide/intro.md": &fstest.MapFile{Data: []byte("---\ntitle: Intro\n---\n# Intro")},
		"guide/bare.md":  &fstest.MapFile{Data: []byte("---\ntags: [x]\n---\n# Heading Only")},
		"no-frontmatter": &fstest.MapFile{Data: []byte("# Nothing")},
	}

	idx, err := BuildIndex(context.Background(), testFS, nil)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	// Keys are fs-relative (no leading slash) — that is the whole point of the
	// helper: navigation.Generator hands it "guide/intro.md", not "/guide/intro.md".
	tests := []struct {
		name     string
		filePath string
		want     string
	}{
		{"root page", "readme.md", "Home"},
		{"nested page", "guide/intro.md", "Intro"},
		{"indexed but no frontmatter title", "guide/bare.md", ""},
		{"not indexed", "no-frontmatter", ""},
		{"leading slash is not the key", "/readme.md", ""},
		{"nonexistent", "missing.md", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := idx.TitleForPath(tt.filePath); got != tt.want {
				t.Errorf("TitleForPath(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestBuildIndex_EmptyFS(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{}
	idx, err := BuildIndex(context.Background(), testFS, nil)
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

// TestIndexAccessors_DeepCopy asserts that every accessor handing a PageInfo out
// of the index deep-copies it: the struct, its Tags slice and its Meta map. The
// index is immutable after construction, so a caller must not be able to reach
// through a returned value and change what the next reader sees. Checking only
// Title passes a bare struct copy that still shares Tags and Meta — which is
// exactly how ByPath and ByTag drifted from AllPages (REVIEW §9.8, §10.7).
func TestIndexAccessors_DeepCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		get  func(*Index) *PageInfo
	}{
		{"AllPages", func(idx *Index) *PageInfo { pages := idx.AllPages(); return &pages[0] }},
		{"ByTag", func(idx *Index) *PageInfo { pages := idx.ByTag("go"); return &pages[0] }},
		{"ByPath", func(idx *Index) *PageInfo { return idx.ByPath("/test.md") }},
		// LookupTag delegates to ByTag today, but it is the accessor all three
		// tag transports call, so it is pinned here in its own right — a future
		// shallow-copy shortcut inside it must fail this test, not hide behind
		// ByTag's row. PagesByTag is deliberately absent: it aliases by contract.
		{"LookupTag", func(idx *Index) *PageInfo { pages, _ := idx.LookupTag("go"); return &pages[0] }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			idx, err := BuildIndex(context.Background(), fstest.MapFS{
				"test.md": {Data: []byte("---\ntitle: Test\ntags: [go, api]\ncategory: docs\n---\n")},
			}, nil)
			if err != nil {
				t.Fatalf("BuildIndex failed: %v", err)
			}

			first := tt.get(idx)
			if first == nil || len(first.Tags) == 0 || first.Meta == nil {
				t.Fatalf("fixture must yield a page with tags and meta, got %+v", first)
			}
			first.Title = "MUTATED"
			first.Tags[0] = "MUTATED"
			first.Meta["category"] = "MUTATED"

			second := tt.get(idx)
			if second.Title == "MUTATED" {
				t.Error("Title mutation leaked into the index")
			}
			if second.Tags[0] == "MUTATED" {
				t.Error("Tags slice is shared with the index, not cloned")
			}
			if second.Meta["category"] == "MUTATED" {
				t.Error("Meta map is shared with the index, not cloned")
			}
		})
	}
}

func TestBuildIndex_CancelledContext(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: A\n---\n")},
		"b.md": {Data: []byte("---\ntitle: B\n---\n")},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Deterministic, not best-effort: the context is cancelled before BuildIndex is
	// called, so every parse goroutine sees a non-nil gctx.Err() on entry and returns
	// it. The previous form (`err != nil && !errors.Is(...)`) also passed on err ==
	// nil, i.e. when cancellation was ignored entirely.
	_, err := BuildIndex(ctx, testFS, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("BuildIndex with a cancelled context: err = %v, want context.Canceled", err)
	}
}

func TestBuildIndex_ManyFiles(t *testing.T) {
	t.Parallel()

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

	idx, err := BuildIndex(context.Background(), testFS, nil)
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

func TestBuildIndex_TagValidation(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"valid.md":    {Data: []byte("---\ntags:\n  - go\n  - machine learning\n  - core.runtime\n  - \"2026\"\n---\n# Valid")},
		"with-bad.md": {Data: []byte("---\ntags:\n  - good\n  - bad/slash\n  - \"\"\n  - \"  \"\n---\n# Bad mix")},
	}

	idx, err := BuildIndex(context.Background(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	tags := idx.AllTags()
	for _, want := range []string{"go", "machine learning", "core.runtime", "2026", "good"} {
		if !slices.Contains(tags, want) {
			t.Errorf("AllTags missing %q (got %v)", want, tags)
		}
	}
	for _, banned := range []string{"bad/slash", "", "  "} {
		if slices.Contains(tags, banned) {
			t.Errorf("AllTags should not contain %q (got %v)", banned, tags)
		}
	}
}

func TestBuildIndex_DuplicateTagsDeduped(t *testing.T) {
	t.Parallel()

	// "go" appears three times (once via differing case/whitespace that
	// normalizes to the same value); the page must be indexed under it once.
	files := fstest.MapFS{
		"dup.md": {Data: []byte("---\ntitle: Dup\ntags:\n  - go\n  - Go\n  - \" go \"\n  - api\n---\n# Dup")},
	}

	idx, err := BuildIndex(context.Background(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	page := idx.ByPath("/dup.md")
	if page == nil {
		t.Fatal("page /dup.md not indexed")
	}
	if want := []string{"go", "api"}; !slices.Equal(page.Tags, want) {
		t.Errorf("page.Tags = %v, want %v", page.Tags, want)
	}
	if pages := idx.ByTag("go"); len(pages) != 1 {
		t.Errorf("ByTag(go) returned %d pages, want 1 (no duplicate)", len(pages))
	}
}

func TestCountByTag(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"a.md": {Data: []byte("---\ntags: [go, api]\n---\n# A")},
		"b.md": {Data: []byte("---\ntags: [go]\n---\n# B")},
	}
	idx, err := BuildIndex(context.Background(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	tests := []struct {
		tag  string
		want int
	}{
		{"go", 2},
		{"api", 1},
		{"GO", 2},      // case-insensitive, matches ByTag
		{"missing", 0}, // unknown tag
	}
	for _, tt := range tests {
		if got := idx.CountByTag(tt.tag); got != tt.want {
			t.Errorf("CountByTag(%q) = %d, want %d", tt.tag, got, tt.want)
		}
		// CountByTag must agree with len(ByTag).
		if got, want := idx.CountByTag(tt.tag), len(idx.ByTag(tt.tag)); got != want {
			t.Errorf("CountByTag(%q)=%d disagrees with len(ByTag)=%d", tt.tag, got, want)
		}
	}
}

// erroringFS fails Open for one named path and delegates everything else.
//
// It embeds the fs.FS interface, not the concrete fstest.MapFS: promoting
// ReadFile would satisfy fs.ReadFileFS, and BuildIndex's fs.ReadFile would then
// bypass this Open and read the file successfully — the test would pass against
// a build that never took the branch it exists to cover.
type erroringFS struct {
	fs.FS
	failPath string
}

func (e erroringFS) Open(name string) (fs.File, error) {
	if name == e.failPath {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return e.FS.Open(name)
}

// TestBuildIndex_SkipsUnparseableFilesWithoutFailing pins the two skips in phase
// 2: a file the walk listed but cannot be read, and one whose frontmatter is not
// YAML. Both return nil, so the build succeeds with the page simply absent.
//
// That is the intended contract — one corrupt page must not blank a site — but
// it is also the mechanism that hid the fs.Sub/fs.StatFS bug in REVIEW.md §9.7,
// so the assertions run both ways and the read failure's warning is part of
// them. Absence alone would pass an index that skipped everything, silently,
// which is exactly what §9.7 did.
func TestBuildIndex_SkipsUnparseableFilesWithoutFailing(t *testing.T) {
	// No t.Parallel: logcapture installs the process-wide default logger.

	log := logcapture.Install(t, slog.LevelWarn)

	files := fstest.MapFS{
		"good.md":       {Data: []byte("---\ntitle: Good\ntags:\n  - keep\n---\n# Good")},
		"unreadable.md": {Data: []byte("---\ntitle: Unreadable\n---\n# Unreadable")},
		"malformed.md":  {Data: []byte("---\ntitle: [unterminated\n---\n# Malformed")},
	}
	rootFS := erroringFS{FS: files, failPath: "unreadable.md"}

	idx, err := BuildIndex(context.Background(), rootFS, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v (a skipped file must not fail the build)", err)
	}

	got := make([]string, 0, len(files))
	for _, p := range idx.AllPages() {
		got = append(got, p.Path)
	}
	slices.Sort(got)
	// Exhaustive, not "contains": a Contains check on the good page would pass
	// an index that also kept a half-parsed unreadable.md.
	if want := []string{"/good.md"}; !slices.Equal(got, want) {
		t.Errorf("indexed pages = %v, want %v", got, want)
	}
	if !log.Has("skipping unreadable file during index build", slog.String("path", "unreadable.md")) {
		t.Errorf("an unreadable file was dropped without a warning naming it; log:\n%s", log)
	}
	if tags := idx.AllTags(); !slices.Equal(tags, []string{"keep"}) {
		t.Errorf("AllTags = %v, want [keep] — the surviving page's tags must still index", tags)
	}
}
