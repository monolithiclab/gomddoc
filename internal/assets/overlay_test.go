package assets

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
)

func TestBuildStaticFS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		overlayFS fstest.MapFS
		theme     string
		wantNil   bool
		wantFile  string
		wantData  string
	}{
		{
			name: "returns nil when no static dirs exist",
			overlayFS: fstest.MapFS{
				"assets/themes/default/layouts/default.html.tmpl": {Data: []byte("tmpl")},
			},
			theme:   "default",
			wantNil: true,
		},
		{
			name: "serves site-level static files",
			overlayFS: fstest.MapFS{
				"static/custom.css": {Data: []byte("site-level")},
			},
			theme:    "default",
			wantFile: "custom.css",
			wantData: "site-level",
		},
		{
			name: "serves theme-level static files",
			overlayFS: fstest.MapFS{
				"assets/themes/mytheme/static/theme.css": {Data: []byte("theme-level")},
			},
			theme:    "mytheme",
			wantFile: "theme.css",
			wantData: "theme-level",
		},
		{
			name: "serves shared static files",
			overlayFS: fstest.MapFS{
				"assets/shared/static/shared.js": {Data: []byte("shared-level")},
			},
			theme:    "default",
			wantFile: "shared.js",
			wantData: "shared-level",
		},
		{
			name: "site-level overrides theme-level",
			overlayFS: fstest.MapFS{
				"static/style.css":                       {Data: []byte("site-wins")},
				"assets/themes/default/static/style.css": {Data: []byte("theme-loses")},
			},
			theme:    "default",
			wantFile: "style.css",
			wantData: "site-wins",
		},
		{
			name: "theme-level overrides shared",
			overlayFS: fstest.MapFS{
				"assets/themes/default/static/common.js": {Data: []byte("theme-wins")},
				"assets/shared/static/common.js":         {Data: []byte("shared-loses")},
			},
			theme:    "default",
			wantFile: "common.js",
			wantData: "theme-wins",
		},
		{
			name: "site-level overrides shared",
			overlayFS: fstest.MapFS{
				"static/common.js":               {Data: []byte("site-wins")},
				"assets/shared/static/common.js": {Data: []byte("shared-loses")},
			},
			theme:    "default",
			wantFile: "common.js",
			wantData: "site-wins",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := BuildStaticFS(tt.overlayFS, tt.theme)

			if tt.wantNil {
				if result != nil {
					t.Error("expected nil, got non-nil FS")
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil FS, got nil")
			}

			data, err := fs.ReadFile(result, tt.wantFile)
			if err != nil {
				t.Fatalf("ReadFile(%q) error: %v", tt.wantFile, err)
			}
			if string(data) != tt.wantData {
				t.Errorf("ReadFile(%q) = %q, want %q", tt.wantFile, data, tt.wantData)
			}
		})
	}
}

func TestBuildFS(t *testing.T) {
	t.Parallel()

	embedded := fstest.MapFS{
		"themes/default/layout.html": &fstest.MapFile{Data: []byte("embedded")},
	}

	t.Run("returns embedded when no config dir", func(t *testing.T) {
		t.Parallel()
		contentRoot := fstest.MapFS{
			"README.md": &fstest.MapFile{Data: []byte("hello")},
		}
		result := BuildFS(contentRoot, embedded)
		data, err := fs.ReadFile(result, "themes/default/layout.html")
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "embedded" {
			t.Errorf("got %q, want %q", data, "embedded")
		}
	})

	t.Run("overlays config dir on top of embedded", func(t *testing.T) {
		t.Parallel()
		contentRoot := fstest.MapFS{
			config.ConfigDirName + "/themes/default/layout.html": &fstest.MapFile{Data: []byte("local")},
		}
		result := BuildFS(contentRoot, embedded)

		// Local file should win
		data, err := fs.ReadFile(result, "themes/default/layout.html")
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "local" {
			t.Errorf("got %q, want %q", data, "local")
		}
	})

	t.Run("falls back to embedded for missing files", func(t *testing.T) {
		t.Parallel()
		contentRoot := fstest.MapFS{
			config.ConfigDirName + "/themes/custom/layout.html": &fstest.MapFile{Data: []byte("custom")},
		}
		result := BuildFS(contentRoot, embedded)

		// Default theme should come from embedded
		data, err := fs.ReadFile(result, "themes/default/layout.html")
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "embedded" {
			t.Errorf("got %q, want %q", data, "embedded")
		}
	})
}
