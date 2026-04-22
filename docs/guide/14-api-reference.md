---
title: "API Reference"
description: "Complete reference for all HTTP endpoints, APIs, and system routes in gomddoc."
author: "nicolasm"
---

# API Reference

gomddoc exposes several HTTP endpoints beyond content serving. This page documents all system routes.

## Content Endpoints

### `GET /{path}`

Serves content from the documentation directory. Markdown files are rendered as HTML by default.

**Content negotiation** via the `Accept` header controls the output format:

| Accept Header | Behavior |
|---------------|----------|
| `*/*` (default) | Markdown rendered as HTML |
| `text/html` | Markdown rendered as HTML |
| `text/markdown` | Raw markdown (frontmatter stripped) |
| Other | `406 Not Acceptable` |

Non-markdown files (images, CSS, JS, PDFs) are served as-is with their detected MIME type.

**Caching:** All responses include `Cache-Control: public, max-age=300` and a weak `ETag` (FNV-64a content hash). Conditional requests with `If-None-Match` return `304 Not Modified` when content is unchanged.

---

## Search API

### `GET /api/search`

Full-text search across all markdown content.

| Parameter | Required | Default | Max | Description |
|-----------|----------|---------|-----|-------------|
| `q` | Yes | | | Search query (AND semantics for multiple terms) |
| `limit` | No | 20 | 100 | Maximum results to return |

**Response:** JSON array of search results, sorted by relevance score.

```json
[
  {
    "path": "/guide/configuration.md",
    "title": "Configuration",
    "description": "How to configure gomddoc",
    "snippet": "...text with <mark>highlighted</mark> matches...",
    "score": 15.2
  }
]
```

---

## Tags API

### `GET /api/tags`

Returns all unique tags found in frontmatter across all documents, sorted alphabetically.

**Response:**

```json
["api", "configuration", "deployment", "go"]
```

### `GET /api/tags/{tag}`

Returns all pages tagged with the specified tag. Tag matching is case-insensitive.

**Response:**

```json
[
  {
    "path": "/guide/configuration.md",
    "title": "Configuration",
    "description": "How to configure gomddoc",
    "tags": ["configuration", "guide"],
    "date": "2026-03-25T00:00:00Z",
    "meta": {
      "author": "Jane Doe"
    }
  }
]
```

Returns an empty array if no pages have the specified tag.

---

## SEO Endpoints

### `GET /robots.txt`

Returns a robots.txt file. Always available regardless of domain configuration. Includes a `Sitemap:` directive when `meta.domain` is configured.

### `GET /sitemap.xml`

Returns an XML sitemap listing all indexed pages. Only available when `meta.domain` is configured.

---

## Health Endpoints

Health endpoints bypass all middleware (security headers, authentication, hidden path blocking) and always return JSON.

### `GET /health/live`

Liveness probe — returns `200 OK` unconditionally.

```json
{"status": "ok"}
```

### `GET /health/ready`

Readiness probe — checks if the content provider is accessible.

**Healthy (200):**
```json
{"status": "ok"}
```

**Unhealthy (503):**
```json
{"status": "unavailable", "error": "stat error message"}
```

---

## Observability Endpoints

### `GET /metrics`

Prometheus metrics in exposition format. Always available.

Exposed metrics:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_requests_total` | Counter | `method`, `status` | Total HTTP requests |
| `http_request_duration_seconds` | Histogram | `method` | Request latency |

### `GET /debug/pprof/*`

Go profiling endpoints. Only available when `--pprof` flag is set. Includes CPU profile, heap, goroutine, allocs, and trace endpoints.

---

## Static Assets

### `GET /_assets/*`

Serves theme static assets (CSS, JS, fonts). Only available in `build` mode — in `serve` mode, assets are inlined in templates.

---

## MCP Server (Model Context Protocol)

gomddoc includes a built-in MCP server for AI-native documentation access. Start it with
`gomddoc mcp [dir]` (stdio transport). See the [MCP Server guide](04-mcp.md) for setup instructions.

### MCP Tools

All tools are read-only and idempotent.

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `search_docs` | Full-text search with TF-IDF ranking | `query` (required), `limit` (default: 20) |
| `read_page` | Read page with metadata prepended | `path` (required) |
| `read_section` | Read a section by heading anchor ID | `path`, `heading_id` (both required) |
| `list_pages` | List pages, optionally filtered by tag | `tag` (optional), `limit` (default: 50) |
| `get_table_of_contents` | Site-wide navigation tree | `path` (optional subtree root) |
| `find_related` | Find related pages by shared tags | `path` (required) |

### MCP Resources

| URI | Type | MIME Type | Description |
|-----|------|-----------|-------------|
| `docs://site/index` | Static | `application/json` | All pages with metadata |
| `docs://site/tags` | Static | `application/json` | All unique tags |
| `docs://site/page/{+path}` | Template | `text/markdown` | Page content (frontmatter stripped) |
| `docs://site/tag/{tag}` | Template | `application/json` | Pages with a specific tag |

### MCP Prompts

| Prompt | Arguments | Description |
|--------|-----------|-------------|
| `explain_concept` | `concept` (required) | Explain a concept using docs as source |
| `troubleshoot` | `issue` (required), `error_message` (optional) | Step-by-step troubleshooting |
| `summarize_page` | `path` (required) | Concise page summary |

---

## Middleware Chain

All content requests pass through this middleware chain (outermost to innermost):

1. **SecurityHeaders** — sets `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`, `HSTS`
2. **RequestID** — assigns a unique request ID (trusts upstream `X-Request-ID` if present)
3. **BasicAuth** — HTTP Basic Authentication (only when `--basic-auth-file` is configured)
4. **Compression** — gzip for responses >= 1KB (skips pre-compressed types like images)
5. **MethodFilter** — allows GET and HEAD only, returns 405 for others
6. **BlockHiddenPaths** — blocks dotfiles except `/.well-known/`
7. **Metrics** — records Prometheus metrics

Health, metrics, API, and SEO endpoints are registered directly on the mux and bypass the content middleware chain (including authentication). Pprof endpoints (`/debug/pprof/*`) are behind the auth RouteGroup and require credentials when `--basic-auth-file` is configured.
