package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/search"
)

func buildTestSearchIndex(t *testing.T) *search.Index {
	t.Helper()

	contentRoot := fstest.MapFS{
		"README.md":    &fstest.MapFile{Data: []byte("# Welcome\n\nThis is the home page for the documentation project.")},
		"guide.md":     &fstest.MapFile{Data: []byte("# Getting Started Guide\n\nLearn how to get started quickly.")},
		"reference.md": &fstest.MapFile{Data: []byte("# API Reference\n\nThe API provides endpoints for resources.")},
	}

	idx, err := search.BuildIndex(context.Background(), contentRoot, nil, nil)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}
	return idx
}

func TestSearchEndpoint(t *testing.T) {
	t.Parallel()

	idx := buildTestSearchIndex(t)
	handler := NewSearchHandler(idx)

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantEmpty  bool
		minResults int
	}{
		{
			name:       "empty query returns empty array",
			query:      "/api/search",
			wantStatus: http.StatusOK,
			wantEmpty:  true,
		},
		{
			name:       "empty q param returns empty array",
			query:      "/api/search?q=",
			wantStatus: http.StatusOK,
			wantEmpty:  true,
		},
		{
			name:       "valid query returns results",
			query:      "/api/search?q=guide",
			wantStatus: http.StatusOK,
			minResults: 1,
		},
		{
			name:       "no matching terms returns empty",
			query:      "/api/search?q=zzzznonexistent",
			wantStatus: http.StatusOK,
			wantEmpty:  true,
		},
		{
			name:       "limit parameter respected",
			query:      "/api/search?q=the&limit=1",
			wantStatus: http.StatusOK,
			minResults: 1,
		},
		{
			name:       "invalid limit uses default",
			query:      "/api/search?q=guide&limit=abc",
			wantStatus: http.StatusOK,
			minResults: 1,
		},
		{
			name:       "negative limit uses default",
			query:      "/api/search?q=guide&limit=-5",
			wantStatus: http.StatusOK,
			minResults: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.query, nil)
			rec := httptest.NewRecorder()

			handler.SearchEndpoint(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			contentType := rec.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", contentType)
			}

			var results []search.SearchResult
			if err := json.NewDecoder(rec.Body).Decode(&results); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.wantEmpty && len(results) != 0 {
				t.Errorf("expected empty results, got %d", len(results))
			}
			if tt.minResults > 0 && len(results) < tt.minResults {
				t.Errorf("expected at least %d results, got %d", tt.minResults, len(results))
			}
		})
	}
}

func TestSearchEndpoint_LimitCapped(t *testing.T) {
	t.Parallel()

	idx := buildTestSearchIndex(t)
	handler := NewSearchHandler(idx)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=the&limit=999", nil)
	rec := httptest.NewRecorder()

	handler.SearchEndpoint(rec, req)

	var results []search.SearchResult
	if err := json.NewDecoder(rec.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should not exceed maxSearchLimit (100) even though 999 was requested
	if len(results) > maxSearchLimit {
		t.Errorf("results (%d) exceeded max limit (%d)", len(results), maxSearchLimit)
	}
}

func TestSearchEndpoint_ResultStructure(t *testing.T) {
	t.Parallel()

	idx := buildTestSearchIndex(t)
	handler := NewSearchHandler(idx)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=guide", nil)
	rec := httptest.NewRecorder()

	handler.SearchEndpoint(rec, req)

	var results []search.SearchResult
	if err := json.NewDecoder(rec.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	r := results[0]
	if r.Path == "" {
		t.Error("result missing path")
	}
	if r.Title == "" {
		t.Error("result missing title")
	}
	if r.Snippet == "" {
		t.Error("result missing snippet")
	}
	if r.Score <= 0 {
		t.Error("result should have positive score")
	}
}
