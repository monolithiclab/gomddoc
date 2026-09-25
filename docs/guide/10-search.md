---
title: "Full-Text Search"
description: "Built-in full-text search with TF-IDF ranking, keyboard shortcuts, and zero external dependencies."
author: "nicolasm"
tags: ["search", "api"]
---

# Full-Text Search

gomddoc includes a built-in full-text search engine that indexes all markdown content at startup.
No external services, databases, or build steps are required: the search index lives in memory
and is ready as soon as the server starts.

## How It Works

When the server starts (via `serve`, `preview` or `mcp`), gomddoc walks the content directory and builds
an inverted index of all `.md` and `.markdown` files. The indexing process runs concurrently and
follows three phases:

1. **Discovery**: walks the filesystem to find all markdown files, skipping hidden files and
   directories (those starting with `.`) and paths matching `exclude`.
2. **Tokenization**: for each file, strips YAML frontmatter and markdown syntax (headings, links,
   emphasis, inline code, HTML tags), removes fenced code blocks entirely, then splits the plain text
   into lowercase terms on any character that is not a letter or digit. Terms shorter than 2 bytes are
   dropped. A page with no body text left is not indexed.
3. **Index construction**: builds an inverted index that maps each term to the documents where it
   appears, along with frequency counts used for relevance scoring.

Text inside fenced code blocks is not searchable. Inline code (`` `like this` ``) is.

The index is immutable after construction and is safe for concurrent reads. It is held entirely in
memory. Pages added or changed after startup are not searchable until the server restarts.

### Disabling the Index

Set `search.index: false` (or `GOMDDOC_SITE_SEARCH_INDEX=false`) to skip the index build on large sites:

```yaml
# .gomddoc/config.yml
search:
  index: false
```

`/api/search` is then not registered and returns `404`, the `search_docs` MCP tool answers that the
index is unavailable, and the `WebSite` JSON-LD omits its `SearchAction`. The theme's search button is
controlled separately by the `search` feature toggle (`theme.features.search: false`).

## Using Search

### Keyboard Shortcut

Press **Ctrl+K** (or **Cmd+K** on macOS) from any page to open the search modal. Pressing it again
closes the modal.

### Search Button

The default theme shows a search button in the header when the `search` feature is enabled (the
default). Click it to open the same search modal as the keyboard shortcut.

### Search Modal

The search modal provides an interactive search experience directly in the browser:

- **Results as you type**: queries are sent to `/api/search` with a 200 ms debounce, without pressing
  Enter. The modal requests at most 10 results.
- **Keyboard navigation**: use the Up and Down arrow keys to move between results, Enter to open
  the selected result (the first one if none is selected), and Escape to close the modal.
- **Snippet previews**: each result shows its path and a text excerpt (about 160 characters) with the
  matching terms highlighted using `<mark>` tags; the page description is shown when there is no snippet.
- **Ranked results**: results are ordered by relevance score, with title matches ranked higher than
  body content.

Result links point at the source file path (`/guide/setup.md`), which the server redirects to the clean URL.

The modal uses CSS custom properties from your theme (`--color-bg`, `--color-text`, `--color-primary`,
and others), so it follows your site's color scheme and dark mode.

## Search Ranking

Results are ranked using TF-IDF (Term Frequency-Inverse Document Frequency), a standard information
retrieval algorithm that balances how often a term appears in a document against how common it is
across all documents. This means rare, specific terms carry more weight than common words.

On top of TF-IDF, gomddoc weights matches by location. A body match scores the term's frequency in the
page (occurrences divided by the page's term count) times its IDF. A title or description match adds a
fixed amount, independent of how often the term appears there:

| Match Location | Score per matching term |
|----------------|-------------------------|
| Title | 3 × IDF |
| Description (from frontmatter) | 1.5 × IDF |
| Body content | term frequency × IDF |

A page whose title matches your query therefore ranks above a page that only mentions the term in its
body. The title comes from frontmatter `title`, then the first `#` heading, then the file name. Adding
`title` and `description` to your frontmatter improves ranking. Ties are broken deterministically, so the
same query always returns the same order.

Queries use **AND semantics** — all terms must appear in a document for it to match. This reduces
noise and returns more focused results. For example, searching for `git authentication` only returns
pages that contain both "git" and "authentication".

## Tag Filters

Queries can filter by frontmatter tags using the `tag:` prefix. Tag matching is case-insensitive and
uses the same AND semantics as free-text terms:

| Query | Matches |
|-------|---------|
| `tag:go` | All pages tagged `go` |
| `tag:go tag:tutorial` | Pages tagged with **both** `go` and `tutorial` |
| `tag:go channels` | Pages tagged `go` that also contain the term "channels" |

A **tag-only** query (no free-text terms) returns every matching page sorted alphabetically by title,
making `tag:` a quick way to list a topic. When free-text terms are present, the tag set first narrows
the candidate pages and the terms are then ranked by relevance within that set. An unknown tag (or any
tag in an AND chain that no page carries) yields no results.

The `tag:` syntax works identically in the browser search modal and the JSON API below.

## Search API

The search engine is also accessible via a JSON REST API, which is useful for integrations, scripts,
or building custom search interfaces:

```
GET /api/search?q=<query>&limit=<n>&lang=<code>
```

| Parameter | Required | Default | Max | Description |
|-----------|----------|---------|-----|-------------|
| `q` | Yes | | 500 bytes (longer queries are truncated) | Search query (multiple terms use AND semantics; supports `tag:` filters) |
| `limit` | No | 20 | 100 | Maximum results to return; larger values are capped |
| `lang` | No | default language | | BCP 47 code to search a specific language's index (e.g., `fr-FR`); read only on multi-language sites |

### Response

```json
[
  {
    "path": "/docs/guide.md",
    "title": "User Guide",
    "description": "Getting started with gomddoc",
    "snippet": "...text with <mark>matching</mark> terms...",
    "score": 0.231
  }
]
```

| Field | Description |
|-------|-------------|
| `path` | Source file path relative to the language's content root (`/docs/guide.md`), not the served URL |
| `title` | From frontmatter `title`, else the first `# heading`, else derived from the file name |
| `description` | From frontmatter `description` (omitted if empty) |
| `snippet` | Context excerpt (about 160 characters) with matched terms wrapped in `<mark>` tags |
| `score` | Relevance score (higher is better, not comparable across different queries); `0` for tag-only queries |

An empty or missing `q` parameter returns an empty array.

### Examples

```bash
# Search for documents about configuration
curl 'http://localhost:8080/api/search?q=configuration'

# Limit to top 5 results
curl 'http://localhost:8080/api/search?q=getting+started&limit=5'

# Filter by tag (URL-encode the colon as %3A or quote the query)
curl 'http://localhost:8080/api/search?q=tag:go+channels'

# Use content negotiation to get the markdown of a result (extensionless URL)
curl -H "Accept: text/markdown" 'http://localhost:8080/docs/guide'
```

## Multi-Language Search

When multiple languages are detected (via BCP 47 directories), gomddoc builds a separate search index for each
language. Each index contains only the content from its language directory, so search results are language-specific.

The `lang` query parameter selects which language's index to query; an unknown code falls back to the default
language. If `lang` is omitted, the first tag of the `Accept-Language` header is used when it names a detected
language, and the default language otherwise.

```bash
# Search French content
curl 'http://localhost:8080/api/search?q=configuration&lang=fr-FR'
```

The search modal sends no `lang` parameter, so it searches the index that matches the browser's `Accept-Language`
header rather than the language of the page it is opened on.

> [!WARNING]
> Known bug: results from a non-default language index come back without the language
> prefix. `/api/search?lang=fr-FR` reports `fr-FR/guide/setup.md` as `/guide/setup.md`, so the modal links French
> results to the default-language page (or to a 404 when no such page exists). Until it is fixed, prepend
> `/{lang}` yourself when consuming the API for a non-default language.

See [Internationalization](13-internationalization.md) for the full multi-language setup guide.

## Availability

Search is available in `serve` and `preview`, where the server builds and holds the index in memory,
and to MCP clients through `search_docs`. It does not work on a static site: `gomddoc build` writes no
search index, and a static host has no `/api/search` endpoint.

> [!WARNING]
> Known bug, tracked in `REVIEW.md`: `build` still emits the search button and modal on every page while
> the `search` feature is enabled (the default). On a static host every query gets a 404 and the modal
> shows no results. Set `theme.features.search: false` for static builds, or run
> [Pagefind](https://pagefind.app/) as a post-build step with its own search UI.

## Adding Search to Custom Themes

If you are building a custom theme, add search support with two additions:

1. Add a button with `id="search-toggle"` to your header. The search module binds to this element:
   ```html
   {{- if .Feature "search" }}
   <button id="search-toggle" aria-label="{{ .T "aria_search" }}" title="{{ .T "search_shortcut" }}">Search</button>
   {{- end }}
   ```

2. Include the search module in your template's scripts section. The shared `scripts-shared` block
   (`{{ template "scripts-shared" . }}`) already does this when the `search` feature is on:
   ```html
   {{- if .Feature "search" }}
   <script type="module">{{ inlineJSAsset "search.mjs" }}</script>
   {{- end }}
   ```

The search modal creates its own DOM elements and injects CSS that reads your theme's CSS
custom properties (`--color-bg`, `--color-text`, `--color-primary`, `--color-border`, etc.), with
fallback values when a property is not defined. No theme-specific CSS is needed.
