package navigation

import (
	"fmt"
	"testing"
	"testing/fstest"
)

// buildBenchFS builds a synthetic content tree with `dirs` directories,
// each containing `pagesPerDir` markdown pages. Approximates a real-world
// site of dirs*pagesPerDir total pages.
func buildBenchFS(dirs, pagesPerDir int) fstest.MapFS {
	fs := fstest.MapFS{}
	for d := range dirs {
		for p := range pagesPerDir {
			path := fmt.Sprintf("section-%02d/page-%02d.md", d, p)
			fs[path] = &fstest.MapFile{Data: fmt.Appendf(nil, "# Page %d-%d\n\nContent.", d, p)}
		}
	}
	return fs
}

// BenchmarkPrevNext measures the per-request cost of looking up
// previous/next pages on a cached Generator.
func BenchmarkPrevNext(b *testing.B) {
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
			gen := NewGenerator(buildBenchFS(tc.dirs, tc.pages), "README.md", nil, nil)
			// Warm cache.
			gen.Tree()

			b.ReportAllocs()
			for b.Loop() {
				prev, next := gen.PrevNext(tc.current)
				_, _ = prev, next
			}
		})
	}
}

// BenchmarkTree measures the per-request cost of accessing the cached
// navigation tree. Should be a single map-free pointer load.
func BenchmarkTree(b *testing.B) {
	gen := NewGenerator(buildBenchFS(20, 10), "README.md", nil, nil)
	gen.Tree() // warm

	b.ReportAllocs()
	for b.Loop() {
		_ = gen.Tree()
	}
}
