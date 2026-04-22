package search

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/monolithiclab/gomddoc/internal/metadata"
)

// SearchResult represents a single search hit.
type SearchResult struct {
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Snippet     string  `json:"snippet"`
	Score       float64 `json:"score"`
}

// document stores indexed content for a single file.
type document struct {
	path        string
	title       string
	description string
	body        string         // markdown body (frontmatter stripped) for snippet generation
	termFreqs   map[string]int // term -> count in body
	totalTerms  int
}

// posting is an entry in the inverted index.
type posting struct {
	docIdx int
	freq   int
}

// Index is an immutable full-text search index. Thread-safe after construction.
type Index struct {
	docs     []document
	inverted map[string][]posting
	docCount int
	avgDL    float64 // average document length for TF-IDF normalization
}

// BuildIndex walks the filesystem, reads all markdown files, tokenizes their content,
// and builds an inverted index. An optional metadata.Index provides titles and descriptions
// from frontmatter. Hidden files and directories are skipped.
//
// The build follows three phases: (1) collect paths, (2) read+tokenize concurrently,
// (3) merge into inverted index.
func BuildIndex(ctx context.Context, rootFS fs.FS, metaIndex *metadata.Index) (*Index, error) {
	// Phase 1: Collect markdown file paths.
	var paths []string
	err := fs.WalkDir(rootFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk error at %s: %w", path, err)
		}

		name := d.Name()
		if strings.HasPrefix(name, ".") && path != "." {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		if ext := filepath.Ext(name); ext != ".md" && ext != ".markdown" {
			return nil
		}

		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("building search index: %w", err)
	}

	// Build a lookup from metadata index for title/description.
	metaLookup := buildMetaLookup(metaIndex)

	// Phase 2: Read and tokenize concurrently.
	type parseResult struct {
		doc document
		ok  bool
	}

	results := make([]parseResult, len(paths))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	for i, p := range paths {
		g.Go(func() error {
			if gctx.Err() != nil {
				return gctx.Err()
			}

			content, readErr := fs.ReadFile(rootFS, p)
			if readErr != nil {
				slog.Debug("skipping unreadable file during search index build", "path", p, "error", readErr)
				return nil
			}

			body := stripFrontmatter(content)
			bodyStr := stripMarkdown(string(body))

			freqs, total := tokenizeToFreqs(bodyStr)
			if total == 0 {
				return nil
			}

			doc := document{
				path:       "/" + p,
				body:       bodyStr,
				termFreqs:  freqs,
				totalTerms: total,
			}

			// Get title and description from metadata index if available.
			if meta, ok := metaLookup["/"+p]; ok {
				doc.title = meta.Title
				doc.description = meta.Description
			}

			// Fallback title: extract from first heading or derive from filename.
			if doc.title == "" {
				doc.title = extractFirstHeading(content)
			}
			if doc.title == "" {
				doc.title = deriveTitle(p)
			}

			results[i] = parseResult{doc: doc, ok: true}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("building search index: %w", err)
	}

	// Phase 3: Merge into inverted index.
	idx := &Index{
		inverted: make(map[string][]posting),
	}

	totalTerms := 0
	for _, r := range results {
		if !r.ok {
			continue
		}
		docIdx := len(idx.docs)
		idx.docs = append(idx.docs, r.doc)

		for term, freq := range r.doc.termFreqs {
			idx.inverted[term] = append(idx.inverted[term], posting{docIdx: docIdx, freq: freq})
		}
		totalTerms += r.doc.totalTerms
	}

	idx.docCount = len(idx.docs)
	if idx.docCount > 0 {
		idx.avgDL = float64(totalTerms) / float64(idx.docCount)
	}

	return idx, nil
}

// Search finds documents matching all query terms (AND semantics), ranked by TF-IDF
// with boosted scores for title and description matches. Returns at most limit results.
func (idx *Index) Search(query string, limit int) []SearchResult {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return []SearchResult{}
	}

	// Find documents containing ALL query tokens.
	var candidates []int
	for i, token := range queryTokens {
		postings, ok := idx.inverted[token]
		if !ok {
			return []SearchResult{} // AND: if any token has no matches, no results
		}

		docSet := make(map[int]bool, len(postings))
		for _, p := range postings {
			docSet[p.docIdx] = true
		}

		if i == 0 {
			candidates = make([]int, 0, len(docSet))
			for d := range docSet {
				candidates = append(candidates, d)
			}
		} else {
			// Intersect
			filtered := candidates[:0]
			for _, d := range candidates {
				if docSet[d] {
					filtered = append(filtered, d)
				}
			}
			candidates = filtered
		}

		if len(candidates) == 0 {
			return []SearchResult{}
		}
	}

	// Score candidates.
	type scored struct {
		docIdx int
		score  float64
	}
	scoredResults := make([]scored, len(candidates))

	for i, docIdx := range candidates {
		doc := idx.docs[docIdx]
		score := 0.0

		for _, token := range queryTokens {
			tf := float64(doc.termFreqs[token]) / float64(doc.totalTerms)
			df := len(idx.inverted[token])
			idf := math.Log(float64(idx.docCount) / float64(df))
			score += tf * idf
		}

		// Title boost: 3x for each query token found in title
		titleLower := strings.ToLower(doc.title)
		for _, token := range queryTokens {
			if strings.Contains(titleLower, token) {
				df := len(idx.inverted[token])
				idf := math.Log(float64(idx.docCount) / float64(df))
				score += 3.0 * idf
			}
		}

		// Description boost: 1.5x
		descLower := strings.ToLower(doc.description)
		for _, token := range queryTokens {
			if strings.Contains(descLower, token) {
				df := len(idx.inverted[token])
				idf := math.Log(float64(idx.docCount) / float64(df))
				score += 1.5 * idf
			}
		}

		scoredResults[i] = scored{docIdx: docIdx, score: score}
	}

	// Sort by score descending.
	slices.SortFunc(scoredResults, func(a, b scored) int {
		if a.score > b.score {
			return -1
		}
		if a.score < b.score {
			return 1
		}
		return 0
	})

	// Take top limit.
	if limit > len(scoredResults) {
		limit = len(scoredResults)
	}
	scoredResults = scoredResults[:limit]

	// Build results with snippets.
	results := make([]SearchResult, len(scoredResults))
	for i, s := range scoredResults {
		doc := idx.docs[s.docIdx]
		results[i] = SearchResult{
			Path:        doc.path,
			Title:       doc.title,
			Description: doc.description,
			Snippet:     generateSnippet(doc.body, queryTokens, defaultSnippetLen),
			Score:       math.Round(s.score*1000) / 1000,
		}
	}

	return results
}

// buildMetaLookup creates a path → PageInfo map from a metadata index.
func buildMetaLookup(metaIndex *metadata.Index) map[string]metadata.PageInfo {
	if metaIndex == nil {
		return nil
	}
	pages := metaIndex.AllPages()
	lookup := make(map[string]metadata.PageInfo, len(pages))
	for _, p := range pages {
		lookup[p.Path] = p
	}
	return lookup
}

// extractFirstHeading extracts the text of the first ATX heading from raw markdown content.
func extractFirstHeading(content []byte) string {
	for _, line := range strings.SplitN(string(content), "\n", 50) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			return strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		}
	}
	return ""
}

// deriveTitle generates a title from a file path by cleaning up the filename.
func deriveTitle(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	if len(name) > 0 {
		return strings.ToUpper(name[:1]) + name[1:]
	}
	return name
}
