package search

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	stdpath "path"
	"runtime"
	"slices"

	"golang.org/x/sync/errgroup"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// SearchResult represents a single search hit.
type SearchResult struct {
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Snippet     string  `json:"snippet"`
	Score       float64 `json:"score"`
}

// maxSnippetBody is the maximum body size stored per document for snippet generation.
// Snippets are ~160 chars; findBestWindow samples 50 positions. 8 KB is ample context
// while capping memory for large documents.
const maxSnippetBody = 8192

// document stores display data and snippet body for a single indexed file.
type document struct {
	path        string
	title       string
	description string
	body        string // markdown body (truncated to maxSnippetBody) for snippet generation
}

// field identifies which document field a posting belongs to.
type field uint8

const (
	fieldBody field = iota
	fieldTitle
	fieldDesc
)

// posting is an entry in the inverted index.
type posting struct {
	docIdx int
	freq   int
	field  field
}

// Index is an immutable full-text search index. Thread-safe after construction.
type Index struct {
	docs          []document
	inverted      map[string][]posting
	docTermCounts []int // body token count per doc, for TF normalization
	docCount      int
}

// BuildIndex walks the filesystem, reads all markdown files, tokenizes their content,
// and builds an inverted index. An optional metadata.Index provides titles and descriptions
// from frontmatter. Hidden files and directories are skipped.
//
// The build follows three phases: (1) collect paths, (2) read+tokenize concurrently,
// (3) merge into inverted index.
func BuildIndex(ctx context.Context, rootFS fs.FS, metaIndex *metadata.Index, excludePatterns []string) (*Index, error) {
	// Phase 1: Collect markdown file paths.
	var paths []string
	err := fs.WalkDir(rootFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk error at %s: %w", path, err)
		}

		if skip, skipErr := provider.SkipWalkEntry(path, d.Name(), d.IsDir(), excludePatterns); skip {
			return skipErr
		}
		if d.IsDir() {
			return nil
		}

		if ext := stdpath.Ext(d.Name()); ext != ".md" && ext != ".markdown" {
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
		doc        document
		bodyFreqs  map[string]int
		bodyTotal  int
		titleFreqs map[string]int
		descFreqs  map[string]int
		ok         bool
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

			body := text.StripFrontmatter(content)
			bodyStr := stripMarkdown(string(body))

			bodyFreqs, bodyTotal := tokenizeToFreqs(bodyStr)
			if bodyTotal == 0 {
				return nil
			}

			// Truncate body for snippet storage to cap memory usage.
			snippetBody := bodyStr
			if len(snippetBody) > maxSnippetBody {
				snippetBody = snippetBody[:maxSnippetBody]
			}

			doc := document{
				path: "/" + p,
				body: snippetBody,
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
				doc.title = text.DeriveTitle(p)
			}

			titleFreqs, _ := tokenizeToFreqs(doc.title)
			descFreqs, _ := tokenizeToFreqs(doc.description)

			results[i] = parseResult{
				doc:        doc,
				bodyFreqs:  bodyFreqs,
				bodyTotal:  bodyTotal,
				titleFreqs: titleFreqs,
				descFreqs:  descFreqs,
				ok:         true,
			}
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

	for _, r := range results {
		if !r.ok {
			continue
		}
		docIdx := len(idx.docs)
		idx.docs = append(idx.docs, r.doc)
		idx.docTermCounts = append(idx.docTermCounts, r.bodyTotal)

		for term, freq := range r.bodyFreqs {
			idx.inverted[term] = append(idx.inverted[term], posting{docIdx: docIdx, freq: freq, field: fieldBody})
		}
		for term, freq := range r.titleFreqs {
			idx.inverted[term] = append(idx.inverted[term], posting{docIdx: docIdx, freq: freq, field: fieldTitle})
		}
		for term, freq := range r.descFreqs {
			idx.inverted[term] = append(idx.inverted[term], posting{docIdx: docIdx, freq: freq, field: fieldDesc})
		}
	}

	idx.docCount = len(idx.docs)

	return idx, nil
}

// Search finds documents matching all query terms (AND semantics), ranked by TF-IDF
// with field-specific boosts. Returns at most limit results.
func (idx *Index) Search(query string, limit int) []SearchResult {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return []SearchResult{}
	}

	// Per-token info: precomputed IDF and per-doc postings.
	type tokenInfo struct {
		idf   float64
		byDoc map[int][]posting
	}

	perToken := make([]tokenInfo, len(queryTokens))

	// Find documents containing ALL query tokens (AND semantics).
	var candidates []int
	for i, token := range queryTokens {
		posts, ok := idx.inverted[token]
		if !ok {
			return []SearchResult{} // AND: if any token has no matches, no results
		}

		byDoc := make(map[int][]posting, len(posts))
		for _, p := range posts {
			byDoc[p.docIdx] = append(byDoc[p.docIdx], p)
		}

		// df = number of distinct documents containing this token (across all fields).
		df := len(byDoc)
		perToken[i] = tokenInfo{
			idf:   math.Log(float64(idx.docCount) / float64(df)),
			byDoc: byDoc,
		}

		if i == 0 {
			candidates = make([]int, 0, len(byDoc))
			for d := range byDoc {
				candidates = append(candidates, d)
			}
		} else {
			// Intersect
			filtered := candidates[:0]
			for _, d := range candidates {
				if _, ok := byDoc[d]; ok {
					filtered = append(filtered, d)
				}
			}
			candidates = filtered
		}

		if len(candidates) == 0 {
			return []SearchResult{}
		}
	}

	// Score candidates using field-specific boosts.
	type scored struct {
		docIdx int
		score  float64
	}
	scoredResults := make([]scored, len(candidates))

	for i, docIdx := range candidates {
		score := 0.0
		for _, ti := range perToken {
			for _, p := range ti.byDoc[docIdx] {
				switch p.field {
				case fieldBody:
					// Safe: bodyTotal == 0 docs are excluded during indexing.
					tf := float64(p.freq) / float64(idx.docTermCounts[docIdx])
					score += tf * ti.idf
				case fieldTitle:
					score += 3.0 * ti.idf
				case fieldDesc:
					score += 1.5 * ti.idf
				}
			}
		}
		scoredResults[i] = scored{docIdx: docIdx, score: score}
	}

	// Sort by score descending.
	slices.SortFunc(scoredResults, func(a, b scored) int {
		return cmp.Compare(b.score, a.score)
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
	for line := range bytes.SplitSeq(content, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 || trimmed[0] != '#' {
			continue
		}
		// Strip leading '#' run and the space after it.
		i := 0
		for i < len(trimmed) && trimmed[i] == '#' {
			i++
		}
		return string(bytes.TrimSpace(trimmed[i:]))
	}
	return ""
}
