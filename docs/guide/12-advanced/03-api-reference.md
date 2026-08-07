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

**URL resolution:** Extensionless paths are resolved to real files via the `PathResolver`. A request
to `GET /guide/setup` resolves internally to `guide/setup.md` if that file exists and `.md` is in
the `strip_extensions` config. If the resolver has no mapping and the provider cannot find the
literal path, the server returns `404 Not Found`.

**Extension redirect:** Requests to paths with a strippable extension receive a `301 Moved
Permanently` redirect to the extensionless canonical URL:

```
GET /guide/setup.md  →  301 Location: /guide/setup
```

This only applies to extensions listed in `strip_extensions` (default: `[".md"]`) and only when
the resolver has a mapping for the file. Non-stripped extensions (`.css`, `.png`, etc.) are
unaffected.

**Content negotiation** via the `Accept` header controls the output format:

| Accept Header | Behavior |
|---------------|----------|
| `*/*` (default) | Markdown rendered as HTML |
| `text/html` | Markdown rendered as HTML |
| `text/markdown` | Raw markdown (frontmatter stripped) |
| Other | `406 Not Acceptable` |

Non-markdown files (images, CSS, JS, PDFs) are served as-is with their detected MIME type.

**Caching:** All responses include `Cache-Control: public, max-age=300` and a weak `ETag` (FNV-64a content hash).
Conditional requests with `If-None-Match` return `304 Not Modified` when content is unchanged.

### Per-Language Content

When multiple languages are detected, non-default language content is served under a language prefix:

| URL | Content |
|---|---|
| `GET /guide/setup` | Default language (e.g., English) |
| `GET /fr-FR/guide/setup` | French version |
| `GET /es-ES/guide/setup` | Spanish version |

Each language has its own content handler with independent navigation, search index, and metadata. All middleware
(compression, caching, extension redirect, content exclusion) applies identically to all languages.

---

## Search API

### `GET /api/search`

Full-text search across all markdown content.

| Parameter | Required | Default | Max | Description |
|-----------|----------|---------|-----|-------------|
| `q` | Yes | | 500 chars | Search query (AND semantics for multiple terms; supports `tag:` filters) |
| `limit` | No | 20 | 100 | Maximum results to return |
| `lang` | No | default language | | BCP 47 language code to search a specific language's index |

The query supports `tag:<name>` filters (case-insensitive, AND-combined): `tag:go tag:tutorial` matches pages carrying
both tags, and `tag:go channels` narrows tagged pages by the free-text term. A tag-only query returns matches sorted
alphabetically by title. See [Full-Text Search](../10-search.md#tag-filters) for details.

When multiple languages are configured, the `lang` parameter selects which language's search index to query. If
omitted, the language is resolved from the `Accept-Language` request header, falling back to the site's default
language.

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

Pages come back sorted by title. Returns `404` with `{"error": "unknown tag"}` when no page carries
the tag, the same answer the HTML `GET /tags/{tag}` gives.

### `GET /tags/` and `GET /tags/{tag}` (HTML)

In addition to the JSON API above, gomddoc serves human-readable HTML tag pages rendered with the active theme:

| Endpoint | Description |
|---|---|
| `GET /tags/` | Index page listing every tag with its page count |
| `GET /tags/{tag}` | Landing page listing all pages carrying `{tag}`, sorted by title |

Tag values are percent-encoded in the path (e.g. `/tags/machine%20learning`). When multiple languages are detected,
each language gets prefixed routes (`GET /{lang}/tags/` and `GET /{lang}/tags/{tag}`). These pages are also emitted as
static HTML by `build` mode, so they work on static hosts. The JSON `/api/tags` routes remain available for
integrations.

---

## SEO Endpoints

### `GET /robots.txt`

Returns a robots.txt file. Always available regardless of domain configuration. Includes a `Sitemap:` directive when
`meta.domain` is configured.

### `GET /sitemap.xml`

Returns an XML sitemap listing all indexed pages. Only available when `meta.domain` is configured. Pages with
`robots: noindex` frontmatter are excluded. URLs use clean extensionless paths when `strip_extensions` is active.

### `sitemap-index.xml` — not an endpoint

`gomddoc build` writes a `sitemap-index.xml` referencing all per-language sitemaps when multiple
languages are detected and `meta.domain` is configured. **`serve` registers no route for it**, so on
a live server this path 404s. The per-language sitemaps below are served in both modes.

### `GET /feed.xml`

Returns an Atom 1.0 XML feed with the 20 most recently modified pages. Only available when `meta.domain` is
configured. Pages with `robots: noindex` are excluded. The feed is cached after first generation.

### Per-Language SEO Endpoints

When multiple languages are detected, each non-default language gets its own SEO endpoints:

| Endpoint | Description |
|---|---|
| `GET /{lang}/sitemap.xml` | Sitemap for the specified language |
| `GET /{lang}/feed.xml` | Atom feed for the specified language |

For example, `GET /fr-FR/sitemap.xml` returns the sitemap for French content only.

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

Serves theme static assets (CSS, JS, fonts, images). Available in both `serve` and `build` modes. Assets are served
from the overlay filesystem (site-level > theme-level > shared). Responses include
`Cache-Control: public, max-age=31536000, immutable` for aggressive caching. Hidden files (dotfiles) and directory
listings are blocked (return 404).

---

## MCP Server (Model Context Protocol)

gomddoc includes a built-in MCP server for AI-native documentation access. Two transports are
available:

- **stdio**: `gomddoc mcp [dir]` — for local clients (Claude Desktop, Cursor, Claude Code)
- **Streamable HTTP**: `/_mcp/` endpoint on any running `gomddoc serve` or `gomddoc preview`
  instance — for remote clients

The HTTP endpoint is protected by the same authentication middleware as other endpoints. See the
[MCP Server guide](../04-mcp.md) for setup instructions.

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
6. **ContentExclusion** — blocks hidden files (dotfiles) and user-configured exclude patterns (returns 404)
7. **ExtensionRedirect** — 301 redirects from `.md` (or other stripped extensions) to extensionless canonical URLs
8. **Metrics** — records Prometheus metrics

Health and robots endpoints bypass all middleware (including authentication). API, metrics, and SEO endpoints are
registered within the auth group. The MCP endpoint (`/_mcp/`) and pprof endpoints (`/debug/pprof/*`) also require
credentials when `--basic-auth-file` is configured. Static assets at `/_assets/` bypass auth for unauthenticated
access.

When `--admin-port` is configured, metrics and pprof move to the admin port and are removed from the
main port. Health endpoints stay on both, so probes pointed at the service port keep working. See
[Observability](../08-observability.md#admin-port) for details.
