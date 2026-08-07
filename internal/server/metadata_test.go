package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/metadata"
)

func buildTestIndex(t *testing.T, files fstest.MapFS) *metadata.Index {
	t.Helper()
	idx, err := metadata.BuildIndex(context.Background(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	return idx
}

func TestTagsHandler(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		files      fstest.MapFS
		wantStatus int
		wantTags   []string
		wantCTType string
	}{
		{
			name:       "empty index returns empty array",
			files:      fstest.MapFS{},
			wantStatus: http.StatusOK,
			wantTags:   []string{},
			wantCTType: mimeJSON,
		},
		{
			name: "populated index returns sorted tags",
			files: fstest.MapFS{
				"doc1.md": &fstest.MapFile{Data: []byte("---\ntitle: Doc1\ntags:\n  - go\n  - testing\n---\n# Doc1")},
				"doc2.md": &fstest.MapFile{Data: []byte("---\ntitle: Doc2\ntags:\n  - api\n  - go\n---\n# Doc2")},
			},
			wantStatus: http.StatusOK,
			wantTags:   []string{"api", "go", "testing"},
			wantCTType: mimeJSON,
		},
		{
			name: "single file with tags",
			files: fstest.MapFS{
				"doc1.md": &fstest.MapFile{Data: []byte("---\ntitle: Doc1\ntags:\n  - zebra\n  - alpha\n---\n# Doc1")},
			},
			wantStatus: http.StatusOK,
			wantTags:   []string{"alpha", "zebra"},
			wantCTType: mimeJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			index := buildTestIndex(t, tt.files)
			handler := NewMetadataHandler(index)

			req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
			w := httptest.NewRecorder()

			handler.TagsHandler(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			ct := w.Header().Get("Content-Type")
			if ct != tt.wantCTType {
				t.Errorf("Content-Type = %q, want %q", ct, tt.wantCTType)
			}

			var got []string
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("failed to decode JSON: %v", err)
			}

			if len(got) != len(tt.wantTags) {
				t.Fatalf("got %d tags, want %d", len(got), len(tt.wantTags))
			}
			for i, tag := range tt.wantTags {
				if got[i] != tag {
					t.Errorf("tag[%d] = %q, want %q", i, got[i], tag)
				}
			}
		})
	}
}

func TestTagPagesHandler(t *testing.T) {
	t.Parallel()
	files := fstest.MapFS{
		"doc1.md": &fstest.MapFile{Data: []byte("---\ntitle: Doc1\ntags:\n  - go\n  - testing\n---\n# Doc1")},
		"doc2.md": &fstest.MapFile{Data: []byte("---\ntitle: Doc2\ntags:\n  - go\n  - api\n---\n# Doc2")},
	}
	index := buildTestIndex(t, files)
	handler := NewMetadataHandler(index)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tags/{tag}", handler.TagPagesHandler)

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if ct := w.Header().Get("Content-Type"); ct != mimeJSON {
			t.Errorf("%s: Content-Type = %q, want %q", path, ct, mimeJSON)
		}
		return w
	}

	tests := []struct {
		name       string
		path       string
		wantTitles []string // in response order; LookupTag sorts by title
	}{
		{"existing tag returns matching pages", "/api/tags/go", []string{"Doc1", "Doc2"}},
		{"tag with single match", "/api/tags/api", []string{"Doc2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := get(tt.path)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}

			var pages []metadata.PageInfo
			if err := json.Unmarshal(w.Body.Bytes(), &pages); err != nil {
				t.Fatalf("failed to decode JSON: %v", err)
			}
			got := make([]string, len(pages))
			for i, p := range pages {
				got[i] = p.Title
			}
			if !slices.Equal(got, tt.wantTitles) {
				t.Errorf("titles = %v, want %v", got, tt.wantTitles)
			}
		})
	}

	// 404, not 200 with []: an empty result can only mean "no such tag", and
	// /tags/nonexistent answers the same question with a 404. The body is
	// compared raw — a decode into map[string]string is blind to the wire shape.
	t.Run("unknown tag is 404", func(t *testing.T) {
		t.Parallel()
		for _, path := range []string{
			"/api/tags/nonexistent",
			"/api/tags/" + strings.Repeat("x", metadata.MaxTagLength+1),
		} {
			w := get(path)
			if w.Code != http.StatusNotFound {
				t.Errorf("%s: status = %d, want 404", path, w.Code)
			}
			if got, want := strings.TrimSpace(w.Body.String()), `{"error":"unknown tag"}`; got != want {
				t.Errorf("%s: body = %q, want %q", path, got, want)
			}
		}
	})
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		status     int
		value      any
		wantStatus int
		wantCT     string
	}{
		{
			name:       "200 with string slice",
			status:     http.StatusOK,
			value:      []string{"a", "b"},
			wantStatus: http.StatusOK,
			wantCT:     mimeJSON,
		},
		{
			name:       "404 with error map",
			status:     http.StatusNotFound,
			value:      map[string]string{"error": "not found"},
			wantStatus: http.StatusNotFound,
			wantCT:     mimeJSON,
		},
		{
			name:       "201 with empty object",
			status:     http.StatusCreated,
			value:      map[string]any{},
			wantStatus: http.StatusCreated,
			wantCT:     mimeJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()

			writeJSON(w, tt.status, tt.value)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			ct := w.Header().Get("Content-Type")
			if ct != tt.wantCT {
				t.Errorf("Content-Type = %q, want %q", ct, tt.wantCT)
			}

			// Verify body is valid JSON
			var decoded any
			if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
				t.Errorf("response body is not valid JSON: %v", err)
			}
		})
	}
}
