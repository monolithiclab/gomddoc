---
title: "Full-Text Search"
description: "Built-in full-text search with TF-IDF ranking, keyboard shortcuts, and zero external dependencies."
author: "nicolasm"
---

# Full-Text Search

gomddoc includes a built-in full-text search engine that indexes all markdown content at startup. No external services, databases, or build steps are required.

## How It Works

When the server starts (via `serve` or `preview`), gomddoc builds an inverted index of all `.md` and `.markdown` files in your content directory. The index:

1. **Walks** the filesystem to discover markdown files (skipping hidden files/directories)
2. **Tokenizes** each document — strips frontmatter, removes markdown syntax, splits into lowercase terms
3. **Builds** an inverted index mapping terms to documents with frequency counts

The index is held in memory and is read-only after construction.

## Using Search

### Keyboard Shortcut

Press `Ctrl+K` (or `Cmd+K` on macOS) to open the search modal from any page.

### Search Button

Every built-in theme includes a search button in the header. Click it to open the search modal.

### Search Modal

The search modal provides:

- **Instant results** — debounced at 200ms as you type
- **Keyboard navigation** — Arrow keys to move between results, Enter to navigate, Escape to close
- **Snippet previews** — contextual text excerpts with highlighted matching terms
- **Ranked results** — title matches rank higher than body content

## Search Ranking

Results are ranked using TF-IDF (Term Frequency-Inverse Document Frequency) with boost factors:

| Match Location | Boost |
|----------------|-------|
| Title | 3x |
| Description | 1.5x |
| Body content | 1x |

Queries use AND semantics — all terms must appear in a document for it to match.

## Search API

The search engine is also accessible via a REST API:

```
GET /api/search?q=<query>&limit=<n>
```

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `q` | Yes | | Search query (multiple terms use AND semantics) |
| `limit` | No | 20 | Max results to return (capped at 100) |

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
| `path` | Document path |
| `title` | From frontmatter `title` or first heading |
| `description` | From frontmatter `description` (omitted if empty) |
| `snippet` | Context excerpt (~160 chars) with `<mark>` highlighted terms |
| `score` | Relevance score (higher is better) |

An empty query returns an empty array.

### Example

```bash
# Search for documents about configuration
curl 'http://localhost:8080/api/search?q=configuration'

# Limit to top 5 results
curl 'http://localhost:8080/api/search?q=getting+started&limit=5'
```

## Availability

Search is available in `serve` and `preview` modes. It is not available in `build` mode (static sites do not have a server-side search endpoint).

## Theme Support

All 8 built-in themes include the search button and modal. Custom themes can add search support by:

1. Adding a button with `id="search-toggle"` to the header
2. Including the search module in scripts: `<script type="module">{{ inlineAsset "search.mjs" }}</script>`

The search modal injects its own CSS using theme CSS custom properties (`--color-bg`, `--color-text`, `--color-primary`, etc.), so it adapts to any theme's color scheme and dark mode automatically.
