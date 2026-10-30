package assets

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestIconFindings(t *testing.T) {
	t.Parallel()

	files := func(dir string, names ...string) fstest.MapFS {
		fsys := fstest.MapFS{"README.md": {}}
		for _, n := range names {
			fsys[dir+"/"+n] = &fstest.MapFile{}
		}
		return fsys
	}
	merge := func(a, b fstest.MapFS) fstest.MapFS {
		for k, v := range b {
			a[k] = v
		}
		return a
	}

	tests := []struct {
		name     string
		fsys     fstest.MapFS
		wantFile string // "" means no finding
	}{
		{name: "built-in set only", fsys: fstest.MapFS{"README.md": {}}},
		{name: "whole set in .gomddoc/static", fsys: files(".gomddoc/static", IconFiles...)},
		{
			name: "whole set in the theme's static",
			fsys: files(".gomddoc/assets/themes/acme/static", IconFiles...),
		},
		{
			name: "set split across site layers",
			fsys: merge(files(".gomddoc/static", IconFiles[:3]...),
				files(".gomddoc/assets/shared/static", IconFiles[3:]...)),
		},
		{
			name:     "only favicon.ico replaced",
			fsys:     files(".gomddoc/static", "favicon.ico"),
			wantFile: ".gomddoc/static/favicon.ico",
		},
		{
			name:     "another theme's icons do not count",
			fsys:     merge(files(".gomddoc/static", "favicon.ico"), files(".gomddoc/assets/themes/other/static", IconFiles[1:]...)),
			wantFile: ".gomddoc/static/favicon.ico",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IconFindings(tt.fsys, "acme")
			if tt.wantFile == "" {
				if len(got) != 0 {
					t.Fatalf("findings = %+v, want none", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("findings = %+v, want one", got)
			}
			f := got[0]
			if f.Code != "theme.icons-mixed" || f.File != tt.wantFile {
				t.Errorf("code/file = %s/%s, want theme.icons-mixed/%s", f.Code, f.File, tt.wantFile)
			}
			if !strings.Contains(f.Message, "favicon.svg") || !strings.Contains(f.Fix, "favicon.svg") {
				t.Errorf("message/fix should name the missing favicon.svg: %q / %q", f.Message, f.Fix)
			}
		})
	}
}
