package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/assets"
	"github.com/monolithiclab/gomddoc/internal/server"
)

// The helper is the one shared module fetched by URL: modules import
// /_assets/gmd-i18n.mjs, so both `serve` and `build` must publish it.

func TestI18nHelper_ServedAsJavaScript(t *testing.T) {
	t.Parallel()

	staticFS := assets.BuildStaticFS(embeddedAssets, "default")
	if staticFS == nil {
		t.Fatal("BuildStaticFS returned nil")
	}

	rec := httptest.NewRecorder()
	server.NewAssetsHandler(staticFS).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/gmd-i18n.mjs", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", got)
	}
	if !strings.Contains(rec.Body.String(), "export function i18n") {
		t.Error("body does not export i18n")
	}
}

func TestI18nHelper_CopiedByBuild(t *testing.T) {
	t.Parallel()

	staticFS := assets.BuildStaticFS(embeddedAssets, "default")
	if staticFS == nil {
		t.Fatal("BuildStaticFS returned nil")
	}

	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}
	if err := b.copyStaticAssets(staticFS, &buildStats{}); err != nil {
		t.Fatalf("copyStaticAssets error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "_assets", "gmd-i18n.mjs")); err != nil {
		t.Errorf("build output lacks _assets/gmd-i18n.mjs: %v", err)
	}
}
