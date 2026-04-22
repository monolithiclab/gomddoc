package search

import (
	"strings"
	"testing"
)

func TestTruncateAtWord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		text   string
		maxLen int
		want   string
	}{
		{
			name:   "short text unchanged",
			text:   "hello world",
			maxLen: 50,
			want:   "hello world",
		},
		{
			name:   "truncates at word boundary",
			text:   "the quick brown fox jumps",
			maxLen: 15,
			want:   "the quick...",
		},
		{
			name:   "empty text",
			text:   "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "html escapes output",
			text:   "<script>alert('xss')</script>",
			maxLen: 100,
			want:   "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:   "CJK text truncates at rune boundary",
			text:   "日本語のテスト文字列です",
			maxLen: 9, // 3 runes * 3 bytes each
			want:   "日本語...",
		},
		{
			name:   "emoji truncates cleanly",
			text:   "hello 🎉🎊🎈 world",
			maxLen: 12, // "hello " (6) + one emoji (4) = 10, next emoji starts at 10
			want:   "hello...",
		},
		{
			name:   "single long word uses rune-aligned fallback",
			text:   "supercalifragilisticexpialidocious",
			maxLen: 10,
			want:   "supercalif...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := truncateAtWord(tt.text, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateAtWord(%q, %d) = %q, want %q", tt.text, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFindBestWindow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		tokens     []string
		windowSize int
		wantPos    int
	}{
		{
			name:       "content shorter than window",
			content:    "short",
			tokens:     []string{"short"},
			windowSize: 100,
			wantPos:    0,
		},
		{
			name:       "finds token cluster",
			content:    strings.Repeat("padding ", 20) + "target word target" + strings.Repeat(" padding", 20),
			tokens:     []string{"target"},
			windowSize: 50,
		},
		{
			name:       "CJK content finds match",
			content:    strings.Repeat("あ", 100) + "検索テスト" + strings.Repeat("い", 100),
			tokens:     []string{"検索"},
			windowSize: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lower := strings.ToLower(tt.content)
			pos := findBestWindow(lower, tt.tokens, tt.windowSize)

			if tt.name == "content shorter than window" {
				if pos != tt.wantPos {
					t.Errorf("findBestWindow() = %d, want %d", pos, tt.wantPos)
				}
				return
			}

			// Verify the window contains the token
			end := min(pos+tt.windowSize, len(lower))
			window := lower[pos:end]
			found := false
			for _, token := range tt.tokens {
				if strings.Contains(window, token) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("findBestWindow() returned pos=%d, window does not contain any token", pos)
			}
		})
	}
}

func TestHighlightTerms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		text   string
		tokens []string
		want   string
	}{
		{
			name:   "single match",
			text:   "hello world",
			tokens: []string{"world"},
			want:   "hello <mark>world</mark>",
		},
		{
			name:   "no match",
			text:   "hello world",
			tokens: []string{"missing"},
			want:   "hello world",
		},
		{
			name:   "multiple matches",
			text:   "the fox and the fox",
			tokens: []string{"fox"},
			want:   "the <mark>fox</mark> and the <mark>fox</mark>",
		},
		{
			name:   "overlapping tokens merged",
			text:   "foobar baz",
			tokens: []string{"foobar", "foo"},
			want:   "<mark>foobar</mark> baz",
		},
		{
			name:   "html escaped around marks",
			text:   "<b>bold</b> match",
			tokens: []string{"match"},
			want:   "&lt;b&gt;bold&lt;/b&gt; <mark>match</mark>",
		},
		{
			name:   "case insensitive match",
			text:   "Hello World",
			tokens: []string{"hello"},
			want:   "<mark>Hello</mark> World",
		},
		{
			name:   "word boundary prevents partial match",
			text:   "foobar foo",
			tokens: []string{"foo"},
			want:   "foobar <mark>foo</mark>",
		},
		{
			name:   "CJK adjacent to CJK not matched (no word boundary)",
			text:   "これは検索テストです",
			tokens: []string{"検索"},
			want:   "これは検索テストです",
		},
		{
			name:   "CJK with space boundaries matched",
			text:   "hello 検索 world",
			tokens: []string{"検索"},
			want:   "hello <mark>検索</mark> world",
		},
		{
			name:   "emoji in text preserved",
			text:   "hello 🎉 world",
			tokens: []string{"world"},
			want:   "hello 🎉 <mark>world</mark>",
		},
		{
			name:   "digit boundary respected",
			text:   "http2 test",
			tokens: []string{"http2"},
			want:   "<mark>http2</mark> test",
		},
		{
			name:   "digit not split from letters",
			text:   "http2 test",
			tokens: []string{"http"},
			want:   "http2 test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := highlightTerms(tt.text, tt.tokens)
			if got != tt.want {
				t.Errorf("highlightTerms(%q, %v) = %q, want %q", tt.text, tt.tokens, got, tt.want)
			}
		})
	}
}

func TestIsWordBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		pos  int
		want bool
	}{
		{"start of string", "hello", 0, true},
		{"end of string", "hello", 5, true},
		{"space boundary", "hello world", 5, true},
		{"mid word", "hello", 2, false},
		{"digit-letter boundary", "abc 123", 3, true},
		{"letter-digit not boundary", "abc123", 3, false},
		{"CJK boundary with space", "日本 語", 6, true},
		{"CJK adjacent to ASCII", "日本abc", 6, false},
		{"punctuation boundary", "hello,world", 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isWordBoundary(tt.text, tt.pos)
			if got != tt.want {
				t.Errorf("isWordBoundary(%q, %d) = %v, want %v", tt.text, tt.pos, got, tt.want)
			}
		})
	}
}

func TestSortSpans(t *testing.T) {
	t.Parallel()

	spans := []span{{10, 15}, {0, 5}, {5, 10}}
	sortSpans(spans)

	for i := 1; i < len(spans); i++ {
		if spans[i].start < spans[i-1].start {
			t.Errorf("spans not sorted: %v", spans)
			break
		}
	}
}

func TestMergeSpans(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		spans []span
		want  []span
	}{
		{"nil input", nil, nil},
		{"no overlap", []span{{0, 5}, {10, 15}}, []span{{0, 5}, {10, 15}}},
		{"adjacent", []span{{0, 5}, {5, 10}}, []span{{0, 10}}},
		{"overlap", []span{{0, 7}, {5, 10}}, []span{{0, 10}}},
		{"contained", []span{{0, 10}, {3, 7}}, []span{{0, 10}}},
		{"multiple merges", []span{{0, 5}, {3, 8}, {7, 12}}, []span{{0, 12}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mergeSpans(tt.spans)
			if len(got) != len(tt.want) {
				t.Fatalf("mergeSpans(%v) = %v, want %v", tt.spans, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("mergeSpans(%v)[%d] = %v, want %v", tt.spans, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGenerateSnippet_Unicode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		queryTokens []string
		maxLen      int
		wantSubstr  string
	}{
		{
			name:        "CJK content no word boundary (no highlight)",
			content:     "これは日本語の検索テストです。全文検索エンジンのテスト。",
			queryTokens: []string{"検索"},
			maxLen:      200,
			wantSubstr:  "検索",
		},
		{
			name:        "CJK with space boundaries highlighted",
			content:     "this is a 検索 test for search",
			queryTokens: []string{"検索"},
			maxLen:      200,
			wantSubstr:  "<mark>検索</mark>",
		},
		{
			name:        "emoji in content",
			content:     "Welcome to the docs 🎉 This is a test of search functionality",
			queryTokens: []string{"search"},
			maxLen:      200,
			wantSubstr:  "<mark>search</mark>",
		},
		{
			name:        "accented characters",
			content:     "Le café est prêt pour le déjeuner",
			queryTokens: []string{"café"},
			maxLen:      200,
			wantSubstr:  "<mark>café</mark>",
		},
		{
			name:        "default maxLen when zero",
			content:     "hello world",
			queryTokens: []string{"hello"},
			maxLen:      0,
			wantSubstr:  "<mark>hello</mark>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := generateSnippet(tt.content, tt.queryTokens, tt.maxLen)
			if !strings.Contains(got, tt.wantSubstr) {
				t.Errorf("generateSnippet() = %q, want substring %q", got, tt.wantSubstr)
			}
		})
	}
}
