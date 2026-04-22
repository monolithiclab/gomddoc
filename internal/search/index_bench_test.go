package search

import (
	"context"
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/metadata"
)

func BenchmarkBuildIndex(b *testing.B) {
	contentRoot := fstest.MapFS{}
	for i := range 50 {
		contentRoot[fmt.Sprintf("doc%d.md", i)] = &fstest.MapFile{
			Data: fmt.Appendf(nil, "# Document %d\n\nThis is document number %d with some searchable content about various topics including Go programming, web development, and documentation tools.", i, i),
		}
	}

	metaIndex, err := metadata.BuildIndex(context.Background(), contentRoot, nil)
	if err != nil {
		b.Fatalf("metadata.BuildIndex failed: %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		_, err := BuildIndex(context.Background(), contentRoot, metaIndex, nil)
		if err != nil {
			b.Fatalf("BuildIndex failed: %v", err)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	contentRoot := fstest.MapFS{}
	for i := range 50 {
		contentRoot[fmt.Sprintf("doc%d.md", i)] = &fstest.MapFile{
			Data: fmt.Appendf(nil, "# Document %d\n\nThis is document number %d with searchable content about Go programming, web servers, and markdown rendering.", i, i),
		}
	}

	metaIndex, err := metadata.BuildIndex(context.Background(), contentRoot, nil)
	if err != nil {
		b.Fatalf("metadata.BuildIndex failed: %v", err)
	}

	idx, err := BuildIndex(context.Background(), contentRoot, metaIndex, nil)
	if err != nil {
		b.Fatalf("BuildIndex failed: %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		idx.Search("programming", 10)
	}
}
