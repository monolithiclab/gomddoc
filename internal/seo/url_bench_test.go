package seo

import "testing"

// BenchmarkPageURL measures the per-call cost of PageURL with the
// single-entry domain cache warm — the typical production path.
func BenchmarkPageURL(b *testing.B) {
	_ = PageURL("docs.example.com", "/", "README.md") // warm cache

	b.ReportAllocs()
	for b.Loop() {
		_ = PageURL("docs.example.com", "/guide/setup", "README.md")
	}
}
