---
title: "API Reference"
description: "Complete reference for all HTTP endpoints, APIs, and system routes in gomddoc."
author: "nicolasm"
---

# API Reference

`gomddoc serve` and `gomddoc preview` expose several HTTP endpoints beyond content serving. This page documents every
route the server registers, and the tools, resources and prompts of the MCP server.

## Route Summary

| Route | Auth | Condition |
|-------|------|-----------|
| `GET /{path}` | Yes | Always |
| `GET /{lang}/{path}` | Yes | One per detected language directory |
| `GET /api/search` | Yes | Search index built (`search.index: true`, the default) |
| `GET /api/tags`, `GET /api/tags/{tag}` | Yes | Always in `serve` and `preview` |
| `GET /tags/`, `GET /tags/{tag}` | Yes | Always in `serve` and `preview` |
| `GET /{lang}/tags/`, `GET /{lang}/tags/{tag}` | Yes | One pair per detected language |
| `GET /sitemap.xml`, `GET /feed.xml` | Yes | `meta.domain` set |
| `GET /{lang}/sitemap.xml`, `GET /{lang}/feed.xml` | Yes | `meta.domain` set, one pair per detected language |
| `GET /robots.txt` | No | Always |
| `GET /_assets/*` | No | Always |
| `/_mcp/` | Yes | Always |
| `GET /health/live`, `GET /health/ready` | No | Always, on the main port and on the admin port |
| `/metrics` | See [Observability Endpoints](#observability-endpoints) | Main port, or the admin port when `--admin-port` is set |
| `GET /debug/pprof/*` | See [Observability Endpoints](#observability-endpoints) | `--pprof` set |

"Auth" means the route requires credentials when `--basic-auth-file` is configured.

## Content Endpoints

### `GET /{path}`

Serves content from the documentation directory. Markdown files are rendered as HTML by default. Only `GET` and
`HEAD` are allowed; any other method returns `405 Method Not Allowed` with an `Allow: GET, HEAD` header and no body.

**URL resolution:** Extensionless paths are resolved to real files via the `PathResolver`. A request
to `GET /guide/setup` resolves internally to `guide/setup.md` if that file exists and `.md` is in
the `strip_extensions` config. If the resolver has no mapping and the provider cannot find the
literal path, the server returns `404 Not Found`.

**Extension redirect:** Requests to paths with a strippable extension receive a `301 Moved
Permanently` redirect to the extensionless URL:

```
GET /guide/setup.md  →  301 Location: /guide/setup
```

This only applies to extensions listed in `strip_extensions` (default: `[".md"]`) and only when
the resolver has a mapping for the file. Non-stripped extensions (`.css`, `.png`, etc.) are
unaffected.

> [!WARNING]
> Known bug, tracked in `REVIEW.md`: a default index file redirects to its extensionless name instead of its
> directory. `GET /guide/README.md` redirects to `/guide/README`, which `serve` answers but `build` never writes.

**`redirect_from`:** A path listed in a page's `redirect_from` frontmatter answers `301` with the page's URL. This
lookup runs before the file is read.

**Directories without an index:** When a directory has no `default_index` file and `dir_index` is off, the server
answers `302 Found` with the first page of the site's navigation tree. That page is the first of the whole site, not
of the requested directory (a known bug, tracked in `REVIEW.md`).

**Hidden and excluded paths:** Dotfiles, dot-directories, and paths matching `exclude` return the same themed
`404` page as a path that does not exist.

**Content negotiation** via the `Accept` header controls the output format:

| Accept Header | Behavior |
|---------------|----------|
| `*/*` (default) | Markdown rendered as HTML |
| `text/html` | Markdown rendered as HTML |
| `text/markdown` | Markdown source, with the page's frontmatter replaced by a generated block (see below) |
| Other | `406 Not Acceptable`, plain-text body listing the available types |

The `text/markdown` response replaces the author's frontmatter with a YAML block holding `metadata` (the parsed
frontmatter), `related_docs`, `prev_page` and `next_page`, each omitted when empty. Every negotiated response carries
`Vary: Accept`.

Non-markdown files (images, CSS, JS, PDFs) are served as-is with their detected MIME type.

**Caching:** Successful responses include `Cache-Control: public, max-age=300` and a weak `ETag` (FNV-64a content
hash). Conditional requests with `If-None-Match` return `304 Not Modified` when content is unchanged.

### Per-Language Content

When multiple languages are detected, non-default language content is served under a language prefix:

| URL | Content |
|---|---|
| `GET /guide/setup` | Default language (e.g., English) |
| `GET /fr-FR/guide/setup` | French version |
| `GET /es-ES/guide/setup` | Spanish version |

Each language has its own content handler with independent navigation, search index, and metadata. The same
middleware (compression, method filter, content exclusion, extension redirect) applies to every language; the
extension redirect keeps the language prefix in its `Location`.

---

## Search API

### `GET /api/search`

Full-text search across all markdown content. Registered only when the search index was built (`search.index`
defaults to `true`); otherwise the path returns `404`.

| Parameter | Required | Default | Max | Description |
|-----------|----------|---------|-----|-------------|
| `q` | Yes | | 500 bytes (longer queries are truncated) | Search query (AND semantics for multiple terms; supports `tag:` filters) |
| `limit` | No | 20 | 100 | Maximum results to return; larger values are capped, invalid values use the default |
| `lang` | No | default language | | BCP 47 language code to search a specific language's index |

An empty or missing `q` returns `[]`.

The query supports `tag:<name>` filters (case-insensitive, AND-combined): `tag:go tag:tutorial` matches pages carrying
both tags, and `tag:go channels` narrows tagged pages by the free-text term. A tag-only query returns matches sorted
alphabetically by title. See [Full-Text Search](../10-search.md#tag-filters) for details.

The `lang` parameter is read only when the site has more than one language. It selects that language's index; an
unknown code falls back to the default language. If `lang` is omitted, the first tag of the `Accept-Language` header
is used when it names a detected language, and the default language otherwise. Multi-language responses carry
`Vary: Accept-Language`.

**Response:** JSON array of search results, sorted by relevance score.

```json
[
  {
    "path": "/guide/configuration.md",
    "title": "Configuration",
    "description": "How to configure gomddoc",
    "snippet": "...text with <mark>highlighted</mark> matches...",
    "score": 0.231
  }
]
```

`path` is the source file path, not the served URL. `description` is omitted when empty.

> [!WARNING]
> Known bug: for a non-default language, `path` is relative to the language directory and
> lacks the `/{lang}` prefix, so `fr-FR/guide/configuration.md` is reported as `/guide/configuration.md`, the path of
> the default-language page.

---

## Tags API

The tags API reads the default language's metadata index. Translated pages are listed on the per-language HTML tag
pages, not here.

### `GET /api/tags`

Returns all unique tags found in frontmatter across all documents, lowercased, sorted alphabetically.

**Response:**

```json
["api", "configuration", "deployment", "go"]
```

### `GET /api/tags/{tag}`

Returns all pages tagged with the specified tag. Tag matching is case-insensitive and ignores surrounding whitespace.

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

`path` is the source file path, not the served URL. `date` is `0001-01-01T00:00:00Z` when the page has no frontmatter
`date`. `meta` holds every other frontmatter field (`author`, `robots`, custom fields). `title`, `description`, `tags`
and `meta` are omitted when empty.

Pages come back sorted by title. Returns `404` with `{"error": "unknown tag"}` when no page carries
the tag, the same answer the HTML `GET /tags/{tag}` gives.

The JSON endpoints stream their output and carry no `ETag` or `Cache-Control`.

### `GET /tags/` and `GET /tags/{tag}` (HTML)

gomddoc also serves HTML tag pages rendered with the active theme:

| Endpoint | Description |
|---|---|
| `GET /tags/` | Index page listing every tag with its page count |
| `GET /tags/{tag}` | Landing page listing all pages carrying `{tag}`, sorted by title |

Tag values are percent-encoded in the path (e.g. `/tags/machine%20learning`). An unknown tag returns the themed 404
page. When multiple languages are detected, each language gets prefixed routes (`GET /{lang}/tags/` and
`GET /{lang}/tags/{tag}`) built from that language's pages. `build` mode also writes these pages as static HTML.

---

## SEO Endpoints

### `GET /robots.txt`

Returns a robots.txt file. Always available, without authentication. Disallows `/_assets/`, `/api/` and `/debug/`,
and ends with a `Sitemap:` directive when `meta.domain` is configured:

```
User-agent: *
Allow: /
Disallow: /_assets/
Disallow: /api/
Disallow: /debug/

Sitemap: https://docs.example.com/sitemap.xml
```

### `GET /sitemap.xml`

Returns an XML sitemap listing all indexed pages, the `/tags/` index, and one entry per tag page. Only available when
`meta.domain` is configured. Pages with `noindex` in their `robots` frontmatter are excluded. URLs use clean
extensionless paths when `strip_extensions` is active.

### `sitemap-index.xml`: not an endpoint

`gomddoc build` writes a `sitemap-index.xml` referencing all per-language sitemaps when multiple
languages are detected and `meta.domain` is configured. `serve` registers no route for it, so on
a live server this path returns `404`. The per-language sitemaps below are served in both modes.

### `GET /feed.xml`

Returns an Atom 1.0 feed with the 20 most recently modified pages. Only available when `meta.domain` is
configured. Pages with `noindex` in their `robots` frontmatter are excluded.

`/sitemap.xml`, `/feed.xml` and `/robots.txt` are generated on first request, cached for the life of the process,
and served with `Cache-Control: public, max-age=300` and an `ETag`.

### Per-Language SEO Endpoints

When multiple languages are detected and `meta.domain` is configured, each non-default language gets its own SEO
endpoints:

| Endpoint | Description |
|---|---|
| `GET /{lang}/sitemap.xml` | Sitemap for the specified language |
| `GET /{lang}/feed.xml` | Atom feed for the specified language |

For example, `GET /fr-FR/sitemap.xml` returns the sitemap for French content only.

---

## Health Endpoints

Health endpoints skip authentication, compression, metrics and the content filters, and always return JSON. On the
main port they still carry the security headers and `X-Request-ID`.

### `GET /health/live`

Liveness probe. Returns `200 OK` unconditionally.

```json
{"status": "ok"}
```

### `GET /health/ready`

Readiness probe. Checks that the content provider can stat its root.

**Healthy (200):**
```json
{"status": "ok"}
```

**Unhealthy (503):**
```json
{"status": "unavailable", "error": "provider not ready"}
```

The underlying error is logged at `WARN`, not returned.

---

## Observability Endpoints

### `/metrics`

Prometheus metrics in exposition format.

Without `--admin-port` (or with it equal to `--port`), `/metrics` is on the main port and requires credentials when
`--basic-auth-file` is configured. It is not compressed or counted by the request metrics. With a separate
`--admin-port`, it moves to the admin listener, where it has no authentication. Never expose the admin port publicly.

gomddoc's own metrics:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_requests_total` | Counter | `method`, `status` | Total HTTP requests |
| `http_request_duration_seconds` | Histogram | `method` | Request latency |

The Go runtime and process collectors of the Prometheus client are exported too.

### `GET /debug/pprof/*`

Go profiling endpoints. Only available when `--pprof` is set: `/debug/pprof/` (index, and every named profile such as
`heap`, `goroutine`, `allocs` under it), `cmdline`, `profile`, `symbol` and `trace`. They require credentials when
`--basic-auth-file` is configured, on either port. They are served on the admin port when `--admin-port` is set.

When `--admin-port` is configured, metrics and pprof move to the admin port and are removed from the
main port. Health endpoints stay on both, so probes pointed at the service port keep working. The admin listener
applies no security headers, request IDs or compression. See [Observability](../08-observability.md#admin-port).

---

## Static Assets

### `GET /_assets/*`

Serves theme static assets (CSS, JS, fonts, images). Available without authentication. Assets are served from the
overlay filesystem (site-level > theme-level > shared). Responses include
`Cache-Control: public, max-age=31536000, immutable` and an `ETag`. Hidden files (any dot-prefixed path segment) and
directories return `404`. `build` copies the same files into `_assets/` in the output directory.

---

## MCP Server (Model Context Protocol)

gomddoc includes a built-in MCP server for AI clients. Two transports are available:

- **stdio**: `gomddoc mcp [dir]` for local clients (Claude Desktop, Cursor, Claude Code)
- **Streamable HTTP**: `/_mcp/` on any running `gomddoc serve` or `gomddoc preview` instance, for remote clients

The HTTP endpoint is protected by the same authentication middleware as other endpoints, and request bodies are
limited to 1 MB. See the [MCP Server guide](../04-mcp.md) for setup instructions.

Every tool that reads a path applies the same hidden-file and `exclude` rules as HTTP, and reports a restricted path
as not found.

### MCP Tools

All tools are read-only and idempotent.

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `search_docs` | Full-text search with TF-IDF ranking | `query` (required), `limit` (default 20, max 100) |
| `read_page` | Read a page (frontmatter stripped), with a YAML header of its title, description and tags prepended | `path` (required, source file path such as `guide/configuration.md`) |
| `read_section` | Read a section by heading anchor ID | `path`, `heading_id` (both required) |
| `list_pages` | List pages, optionally filtered by tag | `tag` (optional), `limit` (default 50, max 200) |
| `get_table_of_contents` | Site-wide navigation tree | `path` (optional subtree root; ignored, see below) |
| `find_related` | Find related pages by shared tags | `path` (required) |

> [!WARNING]
> Known bug, tracked in `REVIEW.md`: `get_table_of_contents` ignores `path` and always returns the whole tree.

`gomddoc mcp` (stdio only) adds three tools that describe gomddoc itself rather than the site:

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `gomddoc_capabilities` | The capabilities report: every setting with its config key, env var, flags and default; commands; frontmatter fields; the active theme's features; guide pages | none |
| `gomddoc_guide` | gomddoc's user guide | none (list pages), `query` (search; `limit` default 10, max 50), `path` (read a page), `path` + `section` (read one section) |
| `gomddoc_doctor` | Check the served site's configuration and content, reloaded on every call | `verbose` (include `info` findings) |

### MCP Resources

| URI | Type | MIME Type | Description |
|-----|------|-----------|-------------|
| `docs://site/index` | Static | `application/json` | All pages with metadata |
| `docs://site/tags` | Static | `application/json` | All unique tags |
| `docs://site/page/{+path}` | Template | `text/markdown` | Page content (frontmatter stripped) |
| `docs://site/tag/{tag}` | Template | `application/json` | Pages with a specific tag; an unknown tag is a resource-not-found error |

`gomddoc mcp` (stdio only) adds:

| URI | Type | MIME Type | Description |
|-----|------|-----------|-------------|
| `gomddoc://capabilities` | Static | `application/json` | The capabilities report (the same document as `gomddoc info --json`) |
| `gomddoc://schema/config` | Static | `application/schema+json` | JSON Schema for `.gomddoc/config.yml` (the same document as `gomddoc schema`) |
| `gomddoc://guide/{+path}` | Template | `text/markdown` | A page of gomddoc's user guide, e.g. `gomddoc://guide/02-configuration.md` |

### MCP Prompts

| Prompt | Arguments | Description |
|--------|-----------|-------------|
| `explain_concept` | `concept` (required) | Explain a concept using docs as source |
| `troubleshoot` | `issue` (required), `error_message` (optional) | Step-by-step troubleshooting |
| `summarize_page` | `path` (required) | Concise page summary |
| `learn_gomddoc` | none | Onboarding to gomddoc itself (stdio only) |

---

## Middleware Chain

Content requests pass through this chain (outermost to innermost):

1. **SecurityHeaders**: sets `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
   `Referrer-Policy: strict-origin-when-cross-origin` and `Permissions-Policy`. `Strict-Transport-Security` is set
   only on TLS connections; gomddoc listens on plain HTTP, so set HSTS at the TLS-terminating proxy.
2. **RequestID**: sets `X-Request-ID` on the response, reusing a well-formed incoming `X-Request-ID` and generating
   one otherwise
3. **Compression**: gzip for responses of 1 KB or more (skips already-compressed types like images); sets
   `Vary: Accept-Encoding`
4. **Metrics**: records Prometheus metrics
5. **BasicAuth**: HTTP Basic Authentication (only when `--basic-auth-file` is configured)
6. **stripPathPrefix**: per-language routes only; removes the `/{lang}` segment
7. **MethodFilter**: allows GET and HEAD only, returns 405 for others
8. **ContentExclusion**: blocks hidden files (dotfiles) and user-configured exclude patterns (returns 404)
9. **ExtensionRedirect**: 301 redirects from `.md` (or other stripped extensions) to extensionless URLs

Steps 1 and 2 wrap every route on the main port. Steps 3 and 4 wrap every route except `/health/*` and `/metrics`.
Step 5 wraps every route except `/health/*`, `/robots.txt` and `/_assets/`; pprof applies its own copy of the same
check. Steps 6 to 9 apply to content routes only.

When `--admin-port` is configured, metrics and pprof move to the admin port and are removed from the
main port. See [Observability](../08-observability.md#admin-port) for details.
