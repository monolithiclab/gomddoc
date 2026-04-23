package main

import (
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

// BenchmarkNavBuilder measures the per-request cost of building the
// []enricher.NavItem tree with active/open marking from the cached
// navigation tree. This is the hot path replacing the previous
// cloneTree + markActive + convertNavNodes sequence.
func BenchmarkNavBuilder(b *testing.B) {
	cases := []struct {
		name        string
		dirs, pages int
		current     string
	}{
		{"small_50pages", 5, 10, "/section-02/page-05.md"},
		{"medium_200pages", 20, 10, "/section-10/page-05.md"},
		{"large_1000pages", 50, 20, "/section-25/page-10.md"},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			fs := fstest.MapFS{}
			for d := range tc.dirs {
				for p := range tc.pages {
					path := fmt.Sprintf("section-%02d/page-%02d.md", d, p)
					fs[path] = &fstest.MapFile{Data: fmt.Appendf(nil, "# Page %d-%d\n\nBody.", d, p)}
				}
			}
			navGen := navigation.NewGenerator(fs, "README.md", nil, nil)
			builder := navBuilderAdapter(navGen)
			// Warm cache.
			_ = builder(tc.current)

			b.ReportAllocs()
			for b.Loop() {
				_ = builder(tc.current)
			}
		})
	}
}
