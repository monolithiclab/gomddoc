---
title: "Full-Text Search"
description: "Built-in full-text search with TF-IDF ranking, keyboard shortcuts, and zero external dependencies."
author: "nicolasm"
tags: ["search", "api"]
---

# Full-Text Search

gomddoc includes a built-in full-text search engine that indexes all markdown content at startup.
No external services, databases, or build steps are required — the search index lives in memory
and is ready as soon as the server starts.

## How It Works

When the server starts (via `serve` or `preview`), gomddoc walks the content directory and builds
an inverted index of all `.md` and `.markdown` files. The indexing process runs concurrently for
performance and follows three phases:

1. **Discovery** — walks the filesystem to find all markdown files, skipping hidden files and
   directories (those starting with `.`).
2. **Tokenization** — for each file, strips YAML frontmatter and markdown syntax (headings, links,
   emphasis, code fences), then splits the plain text into lowercase terms on non-alphanumeric
   boundaries.
3. **Index construction** — builds an inverted index that maps each term to the documents where it
   appears, along with frequency counts used for relevance scoring.

The index is immutable after construction and is safe for concurrent reads. It is held entirely in
memory, which keeps query latency low — typically under a millisecond for most documentation sites.

## Using Search

### Keyboard Shortcut

Press **Ctrl+K** (or **Cmd+K** on macOS) from any page to open the search modal. This is the fastest
way to find content and works across all eight built-in themes.

### Search Button

Every built-in theme includes a search button in the header (typically a magnifying glass icon).
Click it to open the same search modal as the keyboard shortcut.

### Search Modal

The search modal provides an interactive search experience directly in the browser:

- **Instant results** — queries are sent to the server with a 200ms debounce as you type, so results
  appear almost immediately without requiring you to press Enter.
- **Keyboard navigation** — use the Up and Down arrow keys to move between results, Enter to navigate
  to the selected result, and Escape to close the modal.
- **Snippet previews** — each result includes a contextual text excerpt (~160 characters) with the
  matching terms highlighted using `<mark>` tags, helping you judge relevance before clicking.
- **Ranked results** — results are ordered by relevance score, with title matches ranked higher than
  body content.

The modal uses CSS custom properties from your theme (`--color-bg`, `--color-text`, `--color-primary`,
and others) so it automatically matches your site's color scheme and adapts to dark mode without any
additional configuration.

## Search Ranking

Results are ranked using TF-IDF (Term Frequency-Inverse Document Frequency), a standard information
retrieval algorithm that balances how often a term appears in a document against how common it is
across all documents. This means rare, specific terms carry more weight than common words.

On top of TF-IDF, gomddoc applies boost factors to weight matches by location:

| Match Location | Boost Factor |
|----------------|-------------|
| Title (from frontmatter) | 3x |
| Description (from frontmatter) | 1.5x |
| Body content | 1x |

This ensures that a page whose title directly matches your query ranks above a page that merely
mentions the term somewhere in its body. Adding `title` and `description` fields to your YAML
frontmatter improves search quality significantly.

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
| `q` | Yes | | 500 chars | Search query (multiple terms use AND semantics; supports `tag:` filters) |
| `limit` | No | 20 | 100 | Maximum results to return |
| `lang` | No | default language | | BCP 47 code to search a specific language's index (e.g., `fr-FR`) |

### Response

```json
[
  {
    "path": "/docs/guide.md",
    "title": "User Guide",
    "description": "Getting started with gomddoc",
    "snippet": "...text with <mark>matching</mark> terms...",
    "score": 12.45
  }
]
```

| Field | Description |
|-------|-------------|
| `path` | Document path relative to the content root |
| `title` | From frontmatter `title`, or derived from the first `# heading` in the file |
| `description` | From frontmatter `description` (omitted if empty) |
| `snippet` | Context excerpt (~160 chars) with matched terms wrapped in `<mark>` tags |
| `score` | Relevance score (higher is better, not comparable across different queries) |

An empty or missing `q` parameter returns an empty array.

### Examples

```bash
# Search for documents about configuration
curl 'http://localhost:8080/api/search?q=configuration'

# Limit to top 5 results
curl 'http://localhost:8080/api/search?q=getting+started&limit=5'

# Filter by tag (URL-encode the colon as %3A or quote the query)
curl 'http://localhost:8080/api/search?q=tag:go+channels'

# Use content negotiation to get raw markdown for a result
curl -H "Accept: text/markdown" 'http://localhost:8080/docs/guide.md'
```

## Multi-Language Search

When multiple languages are detected (via BCP 47 directories), gomddoc builds a separate search index for each
language. Each index contains only the content from its language directory, so search results are language-specific.

The `lang` query parameter selects which language's index to query. If omitted, the language is resolved from the
`Accept-Language` request header, falling back to the site's default language.

```bash
# Search French content
curl 'http://localhost:8080/api/search?q=configuration&lang=fr-FR'
```

The search modal in the browser uses the current page's language automatically — no user action is needed.

See [Internationalization](13-internationalization.md) for the full multi-language setup guide.

## Availability

Search is available in `serve` and `preview` modes, where the server can build and hold the index in
memory. It is not available in `build` mode — static sites do not have a server-side search endpoint.
For static site search, consider running [Pagefind](https://pagefind.app/) as a post-build step.

## Adding Search to Custom Themes

All eight built-in themes include the search button and modal. If you are building a custom theme,
add search support with two additions:

1. Add a button with `id="search-toggle"` to your header — the search module binds to this element:
   ```html
   <button id="search-toggle" aria-label="Search">Search</button>
   ```

2. Include the search module in your template's scripts section:
   ```html
   <script type="module">{{ inlineJSAsset "search.mjs" }}</script>
   ```

The search modal creates its own DOM elements and injects CSS dynamically using your theme's CSS
custom properties (`--color-bg`, `--color-text`, `--color-primary`, `--color-border`, etc.). This
means the modal adapts to any theme's color scheme and dark mode automatically — no theme-specific
CSS is needed.
