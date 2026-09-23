package docs

import (
	"context"
	"io/fs"
	"path"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/metadata"
)

// TestGuide_EveryPageHasTitleAndDescription pins what the capabilities report
// and gomddoc_guide list: a page without them shows up as a bare path. The
// frontmatter is read through metadata.BuildIndex because that is how the
// report reads it.
func TestGuide_EveryPageHasTitleAndDescription(t *testing.T) {
	t.Parallel()
	var files int
	err := fs.WalkDir(Guide, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && path.Ext(p) == ".md" {
			files++
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	// 13 top-level pages plus 12-advanced/: catches an embed that drops the subdirectory.
	if files < 15 {
		t.Errorf("embedded %d pages, want at least 15", files)
	}

	idx, err := metadata.BuildIndex(context.Background(), Guide, nil)
	if err != nil {
		t.Fatal(err)
	}
	pages := idx.AllPages()
	if len(pages) != files {
		t.Errorf("indexed %d pages, embedded %d", len(pages), files)
	}
	for _, p := range pages {
		if p.Title == "" || p.Description == "" {
			t.Errorf("%s: title=%q description=%q, want both non-empty", p.Path, p.Title, p.Description)
		}
	}
}

func TestGuide_RootedAtGuideDir(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"README.md", "02-configuration.md", "12-advanced/02-markdown-extensions.md"} {
		if _, err := fs.Stat(Guide, p); err != nil {
			t.Errorf("fs.Stat(Guide, %q): %v", p, err)
		}
	}
}
