# Search Index Redesign: Field-Tagged Postings

## Goal

Replace the current hybrid search index (single inverted index for body + linear `strings.Contains`
for title/description) with a unified inverted index where each posting carries a field tag. This
eliminates duplicate title/description storage, enables proper per-field TF-IDF scoring, and removes
linear substring scans from the query path.

## Current Problems

1. **Memory duplication**: `document` stores `title`, `description`, `titleLower`, and `descLower` —
   the lower variants exist solely for scoring.
2. **Scoring quality**: Title and description matches use `strings.Contains` (substring match) rather
   than tokenized TF-IDF. A title containing "getting started" matches the query "start" via
   substring, but this isn't how the body index works.
3. **Query performance**: `Search()` scans all candidates with `strings.Contains` per field per
   token — O(candidates * tokens * field_length) linear work that isn't indexed.
4. **Dead code**: `avgDL` is computed but never used.

## Design

### Data Structures

```go
type field uint8

const (
    fieldBody  field = iota
    fieldTitle
    fieldDesc
)

type posting struct {
    docIdx int
    freq   int
    field  field
}

type document struct {
    path        string
    title       string
    description string
    body        string // truncated to maxSnippetBody (8 KB)
}

type Index struct {
    docs          []document
    inverted      map[string][]posting
    docTermCounts []int // body token count per doc, for TF normalization
    docCount      int
}
```

Changes from current:
- `document` drops `titleLower`, `descLower`, `termFreqs`, and `totalTerms`.
- `posting` gains a `field` tag.
- `Index` drops `avgDL`, adds `docTermCounts`.

### Indexing (BuildIndex)

Each document produces postings from three fields during Phase 2 (concurrent read+tokenize):

```go
bodyFreqs, bodyTotal := tokenizeToFreqs(bodyStr)
titleFreqs, _ := tokenizeToFreqs(doc.title)
descFreqs, _ := tokenizeToFreqs(doc.description)
```

`tokenizeToFreqs` already applies `strings.ToLower` internally via `tokenize`, so no explicit
lowering is needed.

During Phase 3 (sequential merge), postings are added per field:

```go
for term, freq := range bodyFreqs {
    idx.inverted[term] = append(idx.inverted[term], posting{docIdx, freq, fieldBody})
}
for term, freq := range titleFreqs {
    idx.inverted[term] = append(idx.inverted[term], posting{docIdx, freq, fieldTitle})
}
for term, freq := range descFreqs {
    idx.inverted[term] = append(idx.inverted[term], posting{docIdx, freq, fieldDesc})
}
idx.docTermCounts = append(idx.docTermCounts, bodyTotal)
```

The intermediate `parseResult` struct carries all three freq maps and the body total so that
Phase 3 can build postings without re-tokenizing.

### Querying (Search)

#### Candidate Selection

For each query token, look up the posting list once. Build a per-doc posting map for fast scoring.
A document qualifies if the token appears in any field. AND semantics across tokens.

```go
type tokenInfo struct {
    idf   float64
    byDoc map[int][]posting
}

perToken := make([]tokenInfo, len(queryTokens))

for i, token := range queryTokens {
    posts, ok := idx.inverted[token]
    if !ok {
        return []SearchResult{} // AND: no matches
    }

    byDoc := make(map[int][]posting, len(posts))
    for _, p := range posts {
        byDoc[p.docIdx] = append(byDoc[p.docIdx], p)
    }

    df := len(byDoc)  // document frequency = number of distinct docs
    perToken[i] = tokenInfo{
        idf:   math.Log(float64(idx.docCount) / float64(df)),
        byDoc: byDoc,
    }

    // Intersect candidates
    if i == 0 {
        candidates = keys(byDoc)
    } else {
        candidates = intersect(candidates, byDoc)
    }
}
```

Note: `df` is computed as the number of distinct documents containing the token (across all
fields), not the raw posting list length. A token appearing in both title and body of the same
document counts as df=1 for that document.

#### Scoring

Per candidate, iterate each token's postings for that document:

```go
for _, docIdx := range candidates {
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
}
```

Title and description use raw IDF (not TF*IDF) because these fields are so short that term
frequency is meaningless — a token either appears or it doesn't. The boost factors (3.0 for title,
1.5 for description) match the current behavior.

IDF is precomputed once per token, resolving the REVIEW.md item about redundant IDF computation.

### Snippet Generation

No changes. `generateSnippet` still receives `doc.body` and query tokens. Body truncation to
`maxSnippetBody` at index time is preserved.

### Public API

No changes. `SearchResult` and `Search(query string, limit int) []SearchResult` are unchanged.
All callers (`server/search.go`, `mcp/tools.go`, `mcp/prompts.go`) are unaffected.

## REVIEW.md Items Resolved

- **Per-query `strings.ToLower` on static document fields** — eliminated entirely (no lowered fields stored).
- **Search index stores full document bodies** — already fixed (maxSnippetBody), design preserves this.
- **IDF recomputed for same token across documents** — resolved by precomputing IDF once per token.

## Testing

Existing test suite (`TestBuildIndex`, `TestSearch` with 8 subtests, `TestGenerateSnippet`) covers
all search behavior. The refactor is internal — same inputs, same ranking semantics, same API.
Tests should pass without modification. If any ranking order changes due to the shift from substring
matching to tokenized matching in title/description, update test expectations to reflect the
improved behavior.
