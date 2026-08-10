# Search Index Field-Tagged Postings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hybrid search index with a unified field-tagged inverted index that eliminates title/description duplication, enables proper per-field TF-IDF, and removes linear substring scans.

**Architecture:** Single inverted index where each posting carries a field tag (body/title/description). Documents become pure display records. Scoring queries the unified index once per token and partitions by field for weighted scoring.

**Tech Stack:** Go standard library, existing `tokenizeToFreqs` function

---

## File Structure

- Modify: `internal/search/index.go` — data structures, `BuildIndex`, `Search`
- Test: `internal/search/index_test.go` — existing tests (may need ranking expectation updates)

No new files. No callers change (`server/search.go`, `mcp/tools.go`, `mcp/prompts.go` use the unchanged `Search` method and `SearchResult` type).

---

### Task 1: Update data structures

**Files:**
- Modify: `internal/search/index.go:30-59`

- [ ] **Step 1: Run existing tests to establish baseline**

Run: `go test ./internal/search/... -v -count=1`
Expected: All tests PASS

- [ ] **Step 2: Update `posting` struct to include field tag**

Replace the `posting` type and add the `field` enum in `internal/search/index.go`. Replace lines 41-52:

```go
// field identifies which document field a posting belongs to.
type field uint8

const (
	fieldBody  field = iota
	fieldTitle
	fieldDesc
)

// posting is an entry in the inverted index.
type posting struct {
	docIdx int
	freq   int
	field  field
}
```

- [ ] **Step 3: Update `document` struct**

Replace the `document` struct (lines 36-45) with the simplified version that drops `titleLower`, `descLower`, `termFreqs`, and `totalTerms`:

```go
// document stores display data and snippet body for a single indexed file.
type document struct {
	path        string
	title       string
	description string
	body        string // markdown body (truncated to maxSnippetBody) for snippet generation
}
```

- [ ] **Step 4: Update `Index` struct**

Replace the `Index` struct (lines 53-59) to drop `avgDL` and add `docTermCounts`:

```go
// Index is an immutable full-text search index. Thread-safe after construction.
type Index struct {
	docs          []document
	inverted      map[string][]posting
	docTermCounts []int // body token count per doc, for TF normalization
	docCount      int
}
```

- [ ] **Step 5: Verify compilation fails with expected errors**

Run: `go build ./internal/search/...`
Expected: Compilation errors in `BuildIndex` and `Search` referencing removed fields (`termFreqs`, `totalTerms`, `titleLower`, `descLower`, `avgDL`). This confirms the struct changes are correct and the next tasks will fix these references.

---

### Task 2: Update BuildIndex to produce field-tagged postings

**Files:**
- Modify: `internal/search/index.go:60-191`

- [ ] **Step 1: Update parseResult struct and Phase 2 worker**

Replace the `parseResult` struct and the Phase 2 worker goroutine body (lines 96-161). The new `parseResult` carries three freq maps and the body total instead of a full `document`:

```go
	// Phase 2: Read and tokenize concurrently.
	type parseResult struct {
		doc       document
		bodyFreqs map[string]int
		bodyTotal int
		titleFreqs map[string]int
		descFreqs  map[string]int
		ok        bool
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
```

- [ ] **Step 2: Update Phase 3 merge to emit field-tagged postings**

Replace the Phase 3 merge block (lines 166-191) with:

```go
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
```

- [ ] **Step 3: Remove the `strings` import if no longer used**

Check: `strings` is still used by `extractFirstHeading` (line 322, 323, 324). Keep the import.

- [ ] **Step 4: Verify BuildIndex compiles**

Run: `go build ./internal/search/...`
Expected: Compilation errors only in `Search` method (not `BuildIndex`). `BuildIndex` should compile cleanly now.

---

### Task 3: Rewrite Search with field-aware scoring

**Files:**
- Modify: `internal/search/index.go:193-305`

- [ ] **Step 1: Replace the Search method**

Replace the entire `Search` method (lines 195-305) with the new implementation:

```go
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
```

- [ ] **Step 2: Verify full compilation**

Run: `go build ./internal/search/...`
Expected: Clean compilation, no errors.

- [ ] **Step 3: Run all search tests**

Run: `go test ./internal/search/... -v -count=1`
Expected: All tests PASS. If any ranking-order test fails (e.g., `TestSearch/title_boost` or `TestSearch/description_boost`), the change is expected — the new tokenized matching produces slightly different scores than the old substring matching. Update the expected values in the test to match the improved behavior.

- [ ] **Step 4: Run full CI**

Run: `make ci`
Expected: All tests pass, coverage stays at 87%+.

- [ ] **Step 5: Commit**

```bash
git add internal/search/index.go internal/search/index_test.go
git commit -m "Refactor search index to use field-tagged postings

Replace hybrid search approach (inverted index for body + linear
strings.Contains for title/description) with a unified inverted index
where each posting carries a field tag (body/title/description).

- Proper per-field TF-IDF scoring replaces substring matching
- Eliminates titleLower/descLower/termFreqs/totalTerms/avgDL fields
- Precomputes IDF once per query token instead of per candidate
- Document frequency counts distinct docs, not raw posting list length
- No public API changes; all callers unaffected

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Update REVIEW.md

**Files:**
- Modify: `REVIEW.md`

- [ ] **Step 1: Mark the IDF recomputation item as FIXED**

In section 5 (Performance Issues), find the `LOW: IDF recomputed for same token across documents`
entry and mark it as fixed:

```markdown
#### ~~LOW: IDF recomputed for same token across documents in search scoring~~ FIXED

IDF is now precomputed once per query token in the `perToken` array, not recomputed per candidate.
```

- [ ] **Step 2: Verify no other REVIEW.md items need updates**

The following items were already marked FIXED in earlier commits and are unaffected:
- "Per-query `strings.ToLower` on static document fields" — already FIXED
- "Search index stores full document bodies" — already FIXED

The `avgDL` dead code item (section 4, "search.Index.avgDL computed but never used") should also
be marked FIXED since the field is removed:

```markdown
#### ~~LOW: `search.Index.avgDL` computed but never used~~ FIXED

Removed in the field-tagged postings refactor. TF normalization uses `docTermCounts` per-document.
```

- [ ] **Step 3: Update the recommendations section**

Mark the IDF recomputation recommendation as FIXED if it appears in section 9.

- [ ] **Step 4: Commit**

```bash
git add REVIEW.md
git commit -m "Update REVIEW.md: mark IDF and avgDL items as fixed

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```
