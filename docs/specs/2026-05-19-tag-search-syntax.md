# `tag:` Search Syntax (Tag Components Sub-Spec 3 of 3)

**Status**: design approved 2026-05-19; revised 2026-05-29 (review fixes); revised 2026-06-09
(tag membership resolved against the metadata index rather than a search-local `byTag` map — see
Architecture and Edge Cases)
**Sub-spec**: 3 of 3 in the broader Tag Components roadmap entry
**Sub-specs in scope**: `tag:` prefix parser in the search engine plus a discoverable UI tip
**Sub-specs already shipped**: chips + per-tag pages + tag index (sub-spec 1); see-also section (sub-spec 2)

## Goal

Let users filter search results by tag using a `tag:` prefix in the search input. `tag:deployment
kubernetes` returns pages tagged `deployment` whose body/title/description also contains
"kubernetes". Multiple tag terms AND together (`tag:foo tag:bar` requires both). Works in the
search modal, the `/api/search` JSON endpoint, and the MCP `search_docs` tool — all route through
the same `Index.Search` method (which also backs the MCP search prompt), so the parser lives in
one place.

## Design Decisions

### Architecture: filter, not field

Parse `tag:` terms out of the query string before tokenization. Resolve tag membership against
the **metadata index** (`metadata.Index.ByTag`) — the single source of truth already backing the
`/tags/{tag}` listing — rather than maintaining a second tag→doc map inside the search index. The
search index keeps only a lightweight `path → docIdx` map (`pathToDoc`) plus a reference to the
metadata index; mixed queries map the tagged pages to doc indices via `pathToDoc` and run the
existing inverted-index TF-IDF search restricted to that set.

> Earlier revisions built a search-local `Index.byTag map[string][]int`. That duplicated the
> metadata index's tag mapping (and its bugs — e.g. duplicate tags were not deduplicated in either
> place) and forced the bodyless-page divergence below. Resolving against metadata removes the
> duplicate map, the sorted-slice intersection helper, and the divergence in one move.

Rejected alternative: index tags as a new search field (`fieldTag`). Adds re-indexing semantics
and conflates body matching with tag matching (a page with tag `go` and body mentioning `go`
should match `go` via body and `tag:go` via tag — separate fields makes that mental model
muddier, not cleaner).

### Tag value parsing

Single-word values only in v1. `tag:foo` is a tag filter; `tag:foo bar` is `tag:foo` plus
free-text `bar`. Multi-word tags can't be filtered directly without quoting; users with such
tags either slugify them or wait for `tag:"foo bar"` syntax in a future sub-feature.

Tag values are lowercased to match `metadata.Index` storage. Empty `tag:` tokens (the literal
string `tag:`) are skipped. Because `parseQuery` extracts tag values from the raw query *before*
tokenization, tag values bypass the tokenizer's 2-char minimum and letter/digit-only filtering —
a single-char tag (`tag:c`) or a tag with dots/hyphens filters correctly.

### Result ordering

- **Mixed query** (`tag:foo bar`): existing TF-IDF ordering over the tag-filtered candidate set.
- **Tag-only query** (`tag:foo`, `tag:foo tag:bar`): alphabetical by title, case-insensitive,
  via `metadata.CompareTitles`, with `Path` as a stable secondary key for ties (`slices.SortFunc`
  is not stable). Mirrors both the ordering **and the result set** of the `/tags/{tag}` listing
  page from sub-spec 1 (it resolves against the same metadata index). Pages in the search corpus
  contribute their indexed title/description (carrying the heading/filename title fallbacks);
  bodyless tagged pages keep their frontmatter values.
- **Empty query**: existing behavior — return empty result list.
- **Unknown tag**: empty result list (consistent with current AND semantics for free-text).

### Snippets

- **Mixed query**: existing free-text snippet generation, unchanged.
- **Tag-only query**: `Snippet` left empty. The modal does **not** currently render anything when
  `Snippet` is empty (`search.mjs:263` emits the snippet div only when `r.snippet` is truthy), so
  this sub-spec adds a `Description` fallback to the result rendering (see Components → Client).
  MCP and the JSON API already expose `Description` as a separate field, so no server-side snippet
  duplication is needed.

### UI discovery

Server-side parsing alone leaves the feature undiscoverable. Add a one-line caption below the
search input: `Tip: tag:name filters by tag` (locale key `search_tag_tip`). No autocomplete,
no `<datalist>`, no new JS components — just a static caption rendered by `search.mjs` from a
new `data-search-tag-tip` attribute on `<html>` (mirroring the existing `data-search-*` attrs).

## Architecture

No new packages, no new architectural layers. Reuses:

- `metadata.Index.AllPages()` — already read at search-index build time for title/description.
- `metadata.Index.ByTag()` and `metadata.CompareTitles()` — reused at query time to resolve tag
  membership and order tag-only results (no second tag map or comparator in the search package).
- The existing search modal infrastructure (`search.mjs`, the `<html>` data attributes for i18n,
  the `data-search-*` convention).
- The existing locale bundle pattern.

## Components

### Server (gomddoc CLI)

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/search/parse.go` | Create | `parseQuery(query string) (tags []string, freeText string)` — splits the raw query on whitespace; a token is a tag filter only when the **raw** token satisfies `strings.HasPrefix(tok, "tag:")` with a non-empty remainder (value lowercased, no further tokenization); the rest joins back with spaces |
| `internal/search/index.go` (`Index` struct) | Modify | Add `metaIndex *metadata.Index` and `pathToDoc map[string]int` (corpus page path → doc index). No search-local tag map |
| `internal/search/index.go` (`BuildIndex`) | Modify | After the doc list is assembled, retain the `metaIndex` pointer and build the `pathToDoc` map. Tag membership is resolved at query time, not precomputed |
| `internal/search/index.go` (`Search` + `taggedPages`) | Modify | Call `parseQuery`; if tag filters present, resolve the AND-intersection of `metaIndex.ByTag(tag)` page sets (`taggedPages`); if free-text empty, sort via `metadata.CompareTitles` and return (tag-only, includes bodyless pages); otherwise map the tagged pages to doc indices via `pathToDoc` and restrict the existing TF-IDF flow to that set up front |
| `internal/search/index_test.go` | Modify | New tests: `TestParseQuery` (table-driven), `TestSearch_TagFilter`, `TestSearch_TagOnlyAlphabetical`, `TestSearch_UnknownTagReturnsEmpty`, `TestSearch_TagPlusFreeText`, `TestSearch_TagCaseInsensitive`, `TestSearch_EmptyTagTokenIgnored`, `TestSearch_TagOnlyIncludesBodylessPage`, `TestSearch_MixedQueryExcludesBodylessPage` |
| `internal/metadata/index.go` (`pageFromFrontmatter`) | Modify | Deduplicate normalized tags per page so `byTag`/`ByTag` hold each page at most once per tag (fixes duplicate results in search, the `/tags` listing, tag counts, and related-docs) |

### Client (default theme + locale + shared module)

| File | Action | Responsibility |
|------|--------|----------------|
| `cmd/gomddoc/assets/locales/en-US.yml` | Modify | Append `search_tag_tip: "Tip: tag:name filters by tag"` |
| `cmd/gomddoc/assets/themes/default/layouts/default.html.tmpl` | Modify | Add `data-search-tag-tip="{{ .T "search_tag_tip" }}"` to the `<html>` tag, next to the existing `data-search-*` attributes |
| `cmd/gomddoc/assets/shared/search.mjs` | Modify | (1) Read `data-search-tag-tip`, render a `<div class="search-tip">` between the input wrapper and the results list when the attribute is non-empty; add one CSS rule for `.search-tip` (muted color, small font, padding). (2) Add a `Description` fallback in `renderResults` (line 263): when `r.snippet` is empty but `r.description` is set, render `escapeHTML(r.description)` in a `.search-result-snippet` div so tag-only results still show preview text. Shared module — all themes inherit the fix |

### MCP tool description

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/mcp/tools.go` (`search_docs` tool description) | Modify | Append a sentence to the description: `Supports tag: prefix syntax (e.g. "tag:deployment kubernetes") to filter by frontmatter tag.` |

### Roadmap

| File | Action |
|------|--------|
| `docs/roadmap.md` | Mark `Search by tag (\`tag:\` prefix)` done |

### Coordinated change (separate repo)

The 7 themes in `gomddoc-themes` each add the same `data-search-tag-tip` attribute to their
`<html>` element. Single mechanical edit per theme, same shape as the existing
`data-search-placeholder` and `tag-chips` follow-ups.

## Data Flow

### Mixed query: `GET /api/search?q=tag:deployment+kubernetes`

```
SearchHandler
  → Index.Search("tag:deployment kubernetes", limit)
       → parseQuery → tagFilters=["deployment"], freeText="kubernetes"
       → taggedPages(["deployment"]) → []PageInfo via metaIndex.ByTag
       → tagSet = {pathToDoc[p.Path] for tagged pages in corpus}
       → tokenize("kubernetes") → ["kubernetes"]
       → existing token loop, first token's candidates restricted to tagSet up front
       → TF-IDF scoring on body/title/description postings
       → sort by score desc, take limit, build SearchResult slice with snippets
  → JSON response
```

### Tag-only query: `tag:deployment`

```
Index.Search("tag:deployment", limit)
  → parseQuery → tagFilters=["deployment"], freeText=""
  → taggedPages(["deployment"]) → []PageInfo via metaIndex.ByTag
  → freeText empty → tagOnlyResults(pages, limit)
       → enrich corpus pages with indexed title/description (fallbacks)
       → slices.SortFunc on metadata.CompareTitles, Path tiebreak
       → take limit
       → SearchResult{Path, Title, Description} for each (Snippet="")
  → JSON response (modal renders Description via the new empty-snippet fallback)
```

### Unknown tag: `tag:doesnotexist`

```
parseQuery → tagFilters=["doesnotexist"], freeText=""
taggedPages(["doesnotexist"]) → metaIndex.ByTag returns nil
return []SearchResult{}
```

## Edge Cases

| Case | Behavior |
|------|----------|
| `tag:` (empty value after colon) | Token skipped during parsing; no tag filter added |
| `tag:foo tag:bar` | AND — page must have both tags |
| `tag:Foo` | Lowercased to `foo`, matches tag stored as `foo` in the index |
| `:tag` | Doesn't start with `tag:`; passes through as free-text token |
| `tag::foo` | Tag value is `:foo` after the prefix strip; lowercased; idx.byTag lookup misses → empty results. Same as unknown tag |
| Literal body match for the string `tag:value` | Treated as a tag filter, not a body search. Power-user opt-out: quote it — `"tag:value"` keeps its leading `"`, so `HasPrefix(tok, "tag:")` is false and it falls through to free text, where the tokenizer then strips the quotes/colon into body tokens `tag` + `value` (an AND body search). v1 trade-off |
| Tagged page with empty/whitespace-only body | **Tag-only query**: appears, because membership is resolved against the metadata index (which retains all pages); title/description come from its frontmatter. Consistent with the `/tags/{tag}` listing page. **Mixed query** (`tag:foo bar`): excluded, because a bodyless page has no indexed body and so cannot match the free-text term |
| Search query contains only whitespace | Existing behavior preserved — empty result list |
| `metaIndex == nil` during BuildIndex | `idx.byTag` stays empty; `tag:` queries return zero results gracefully |
| Tag value with hyphens, underscores, digits, dots, or other non-whitespace punctuation | `parseQuery` splits only on whitespace, so any non-whitespace value is passed through verbatim to `idx.byTag`. Matches whatever the metadata validator allows (currently anything except `/` and `\` and whitespace-only entries) |
| Multi-word tag like `"machine learning"` | Can't be filtered in v1 — `tag:machine learning` filters by tag `machine` plus free-text `learning`. Documented in tip + spec |

## Testing

| Layer | Coverage |
|-------|----------|
| `parseQuery` | Table-driven test: no `tag:` terms, single `tag:foo`, multiple `tag:foo tag:bar`, mixed `tag:foo bar baz`, empty `tag:`, `:foo` not a tag, uppercase value lowercased, whitespace-only input |
| `Index.Search` tag filter | Index two pages with tag `shared`, one with tag `other`; `tag:shared` returns the two `shared` pages; `tag:other` returns the one; `tag:nonexistent` returns empty |
| Tag-only alphabetical | Index pages with titles `Zebra`, `Alpha`, `Mango` all tagged `topic`; `tag:topic` returns them in `Alpha`, `Mango`, `Zebra` order |
| Tag + free-text | Index a `shared`-tagged page mentioning `kubernetes` and another `shared`-tagged page that doesn't; `tag:shared kubernetes` returns only the first |
| Case insensitivity | `tag:Shared` matches pages tagged `shared` |
| Empty tag token | `tag: kubernetes` (with space) treats `kubernetes` as the only free-text term, no tag filter |
| Bodyless page (tag-only) | Index a `shared`-tagged page with frontmatter only (no prose body); confirm `tag:shared` **does** return it (consistent with the `/tags/` page) |
| Bodyless page (mixed) | The same bodyless page is excluded from `tag:shared kubernetes` — no body to match the free-text term |
| Duplicate tags on a page | A page with the same tag listed twice appears once in `tag:` results (deduplicated at the metadata layer) |
| Tag-only stable ordering | Two `topic`-tagged pages with identical titles sort deterministically by `Path` |
| MCP `search_docs` description | Snapshot test of the tool description (or just inspect the registered description string) confirms it mentions `tag:` syntax |
| Search modal client behavior | Manual smoke test via `gomddoc preview testsite` after build: open search modal, confirm the tip appears below the input, and that a `tag:`-only query shows the page `Description` as preview text (empty-snippet fallback) |

Coverage target: maintain 87%+ overall.

## Rollout

1. **This repo PR** — parser, `byTag` index, `Search` extension, locale string, theme layout edit,
   `search.mjs` update, MCP tool description, tests, roadmap update.
2. **`gomddoc-themes` coordinated PR** — add `data-search-tag-tip` to each of the 7 themes' `<html>`
   element (bundle with the still-uncommitted `tag-chips` and `see-also` wiring).
3. **`gomddoc-website` PR** — update `docs/features.md` (extend the "Metadata & Tags" section to
   describe the syntax) and the comparison pages if any of them call out search capabilities
   (bundle with the still-uncommitted feature flag docs).

## Out of Scope (Deferred)

- `tag:"foo bar"` quoted multi-word values.
- `<datalist>` of known tags for native browser hint.
- Full client-side autocomplete dropdown of matching tags as the user types.
- A separate `&tags=foo,bar` JSON API parameter — the unified `q=` string covers it.
- "Show me documents WITHOUT this tag" syntax (`-tag:foo`).
- Field prefixes for other fields (`title:`, `desc:`).
