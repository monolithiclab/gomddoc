package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
			wantCTType: "application/json",
		},
		{
			name: "populated index returns sorted tags",
			files: fstest.MapFS{
				"doc1.md": &fstest.MapFile{Data: []byte("---\ntitle: Doc1\ntags:\n  - go\n  - testing\n---\n# Doc1")},
				"doc2.md": &fstest.MapFile{Data: []byte("---\ntitle: Doc2\ntags:\n  - api\n  - go\n---\n# Doc2")},
			},
			wantStatus: http.StatusOK,
			wantTags:   []string{"api", "go", "testing"},
			wantCTType: "application/json",
		},
		{
			name: "single file with tags",
			files: fstest.MapFS{
				"doc1.md": &fstest.MapFile{Data: []byte("---\ntitle: Doc1\ntags:\n  - zebra\n  - alpha\n---\n# Doc1")},
			},
			wantStatus: http.StatusOK,
			wantTags:   []string{"alpha", "zebra"},
			wantCTType: "application/json",
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

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "existing tag returns matching pages",
			path:       "/api/tags/go",
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "tag with single match",
			path:       "/api/tags/api",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "missing tag returns empty array",
			path:       "/api/tags/nonexistent",
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tags/{tag}", handler.TagPagesHandler)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			ct := w.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}

			var pages []metadata.PageInfo
			if err := json.Unmarshal(w.Body.Bytes(), &pages); err != nil {
				t.Fatalf("failed to decode JSON: %v", err)
			}

			if len(pages) != tt.wantCount {
				t.Errorf("got %d pages, want %d", len(pages), tt.wantCount)
			}
		})
	}
}

func TestTagPagesHandler_EmptyTag(t *testing.T) {
	t.Parallel()
	index := buildTestIndex(t, fstest.MapFS{})
	handler := NewMetadataHandler(index)

	// Call TagPagesHandler directly without mux routing so PathValue("tag") returns ""
	req := httptest.NewRequest(http.MethodGet, "/api/tags/", nil)
	w := httptest.NewRecorder()

	handler.TagPagesHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if body["error"] != "tag parameter required" {
		t.Errorf("error = %q, want %q", body["error"], "tag parameter required")
	}
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
			wantCT:     "application/json",
		},
		{
			name:       "404 with error map",
			status:     http.StatusNotFound,
			value:      map[string]string{"error": "not found"},
			wantStatus: http.StatusNotFound,
			wantCT:     "application/json",
		},
		{
			name:       "201 with empty object",
			status:     http.StatusCreated,
			value:      map[string]any{},
			wantStatus: http.StatusCreated,
			wantCT:     "application/json",
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
