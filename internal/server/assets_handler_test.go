package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestAssetsHandler_ServeHTTP(t *testing.T) {
	t.Parallel()

	staticFS := fstest.MapFS{
		"style.css":            {Data: []byte("body { color: red; }")},
		"js/app.js":            {Data: []byte("console.log('hello')")},
		"img/logo.png":         {Data: []byte("PNG-DATA")},
		".hidden":              {Data: []byte("secret")},
		"sub/.dotfile":         {Data: []byte("secret")},
		"color-chip.js":        {Data: []byte("export default {}")},
		"deep/nested/file.css": {Data: []byte("div { margin: 0; }")},
	}

	handler := NewAssetsHandler(staticFS)

	tests := []struct {
		name          string
		path          string
		wantStatus    int
		wantBody      string
		wantType      string
		wantCacheCtrl string
		wantETag      bool
	}{
		{
			name:          "serves CSS file",
			path:          "/style.css",
			wantStatus:    http.StatusOK,
			wantBody:      "body { color: red; }",
			wantType:      "text/css; charset=utf-8",
			wantCacheCtrl: "public, max-age=31536000, immutable",
			wantETag:      true,
		},
		{
			name:          "serves JS file in subdirectory",
			path:          "/js/app.js",
			wantStatus:    http.StatusOK,
			wantBody:      "console.log('hello')",
			wantType:      "text/javascript; charset=utf-8",
			wantCacheCtrl: "public, max-age=31536000, immutable",
			wantETag:      true,
		},
		{
			name:          "serves deeply nested file",
			path:          "/deep/nested/file.css",
			wantStatus:    http.StatusOK,
			wantBody:      "div { margin: 0; }",
			wantType:      "text/css; charset=utf-8",
			wantCacheCtrl: "public, max-age=31536000, immutable",
			wantETag:      true,
		},
		{
			name:       "404 for non-existent file",
			path:       "/nonexistent.css",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "404 for root directory listing",
			path:       "/",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "404 for dot path",
			path:       "/.",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "blocks dotfiles at root",
			path:       "/.hidden",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "blocks dotfiles in subdirectory",
			path:       "/sub/.dotfile",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "blocks dotfile directory segments",
			path:       "/.secret/file.css",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Handler receives the path after StripPrefix removes /_assets/
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				if got := rec.Body.String(); got != tt.wantBody {
					t.Errorf("body = %q, want %q", got, tt.wantBody)
				}
				if got := rec.Header().Get("Content-Type"); got != tt.wantType {
					t.Errorf("Content-Type = %q, want %q", got, tt.wantType)
				}
				if got := rec.Header().Get("Cache-Control"); got != tt.wantCacheCtrl {
					t.Errorf("Cache-Control = %q, want %q", got, tt.wantCacheCtrl)
				}
				if tt.wantETag {
					etag := rec.Header().Get("ETag")
					if etag == "" {
						t.Error("expected ETag header to be set")
					}
				}
			}
		})
	}
}

func TestAssetsHandler_ETagConditional(t *testing.T) {
	t.Parallel()

	staticFS := fstest.MapFS{
		"style.css": {Data: []byte("body { color: red; }")},
	}

	assertRevalidates(t, revalidationCase{
		Handler:   NewAssetsHandler(staticFS),
		Path:      "/style.css",
		WantType:  "text/css; charset=utf-8",
		WantCache: cacheImmutable,
	})
}

func TestAssetsHandler_ETagMismatch(t *testing.T) {
	t.Parallel()

	staticFS := fstest.MapFS{
		"style.css": {Data: []byte("body { color: red; }")},
	}
	handler := NewAssetsHandler(staticFS)

	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	req.Header.Set("If-None-Match", `W/"stale-etag"`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("mismatched ETag status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAssetsHandler_DirectoryBlocked(t *testing.T) {
	t.Parallel()

	staticFS := fstest.MapFS{
		"js/app.js": {Data: []byte("code")},
	}
	handler := NewAssetsHandler(staticFS)

	req := httptest.NewRequest(http.MethodGet, "/js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// fstest.MapFS creates implicit directories, so "js" resolves as a dir
	if rec.Code != http.StatusNotFound {
		t.Errorf("directory path status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAssetsHandler_NilStaticFS(t *testing.T) {
	t.Parallel()

	// Verify NewAssetsHandler does not panic with a valid FS
	handler := NewAssetsHandler(fstest.MapFS{})
	req := httptest.NewRequest(http.MethodGet, "/anything.css", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("empty FS status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
