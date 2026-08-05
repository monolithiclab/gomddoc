package search

import (
	"context"
	"fmt"
	"strings"
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

// BenchmarkGenerateSnippet exercises the part of snippet generation that
// mergeCorpus cannot: its bodies are shorter than the 160-byte window, so
// findBestWindow returns 0 without ever entering its ~50-iteration sampling
// loop. Real bodies run to maxSnippetBody, where lowering per window instead of
// per document was 13.9% of all search-query allocations.
func BenchmarkGenerateSnippet(b *testing.B) {
	// Mixed case throughout: an all-lowercase body would let strings.ToLower
	// return its argument and hide the allocation this benchmark is about.
	body := strings.Repeat("Lorem Ipsum dolor sit Amet, consectetur adipiscing ELIT. ", maxSnippetBody/57)
	tokens := []string{"consectetur", "adipiscing"}

	b.ReportAllocs()
	for b.Loop() {
		generateSnippet(body, tokens, defaultSnippetLen)
	}
}

// BenchmarkSearch measures how a query scales with the corpus, which is what
// the posting-list merge is about. The queries differ in the shape they hand the
// merge and the ranking:
//
//   - "common" is in every document, so its posting list is the whole corpus —
//     the case a per-token map made quadratic. It also has df == docCount, hence
//     idf == 0 and no score spread, so it does not exercise ranking.
//   - "alpha" matches a third of the corpus with varied scores: the honest
//     measure of the top-N window.
//   - "common alpha" and "tag:t1 common" add a second token and the tag
//     pre-filter.
func BenchmarkSearch(b *testing.B) {
	for _, size := range []int{50, 500, 2000} {
		b.Run(fmt.Sprintf("docs=%d", size), func(b *testing.B) {
			idx := mergeCorpus(b, size)
			for _, query := range []string{"common", "alpha", "common alpha", "tag:t1 common"} {
				b.Run(query, func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						idx.Search(query, 10)
					}
				})
			}
		})
	}
}
