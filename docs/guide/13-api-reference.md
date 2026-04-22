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

## Middleware Chain

All content requests pass through this middleware chain (outermost to innermost):

1. **SecurityHeaders** — sets `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`, `HSTS`
2. **RequestID** — assigns a unique request ID (trusts upstream `X-Request-ID` if present)
3. **BasicAuth** — HTTP Basic Authentication (only when `--basic-auth-file` is configured)
4. **Compression** — gzip for responses >= 1KB (skips pre-compressed types like images)
5. **MethodFilter** — allows GET and HEAD only, returns 405 for others
6. **BlockHiddenPaths** — blocks dotfiles except `/.well-known/`
7. **Metrics** — records Prometheus metrics

Health, metrics, API, and SEO endpoints are registered directly on the mux and bypass the content middleware chain (including authentication).
