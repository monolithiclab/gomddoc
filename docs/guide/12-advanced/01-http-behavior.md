---
title: "HTTP Behavior"
description: "Built-in HTTP features including caching, compression, and content negotiation."
author: "nicolasm"
---

# HTTP Behavior

gomddoc includes built-in HTTP features that require no configuration: caching, compression, and content negotiation.

## Caching

Cache-Control and ETag travel together: both are written by the one helper (`serveWithETag`) that
every handler holding a complete response body goes through. A route either has both or neither,
and the scope below is the same for the two sections.

### Cache-Control

| Routes | `Cache-Control` |
|--------|-----------------|
| Rendered pages, raw files, `/tags/`, `/sitemap.xml`, `/feed.xml`, `/robots.txt` | `public, max-age=300` |
| `/_assets/` (static files) | `public, max-age=31536000, immutable` |
| `/api/*`, `/health/*`, `/metrics`, error responses | *(none)* |

Five minutes is short enough that a redeploy propagates quickly and long enough to absorb a reload.

> [!WARNING]
> `/_assets/` URLs are **not** fingerprinted — `assetURL "css/site.css"` yields
> `/_assets/css/site.css`, the same URL before and after you edit the file. Combined with
> `max-age=31536000, immutable`, a browser that has fetched one will not ask about it again for a
> year. Add your own cache-busting query string (`/_assets/css/site.css?v=3`) when you change a
> published static file. Tracked in `REVIEW.md`.

### ETag / 304 Not Modified

Responses written through `serveWithETag` — the same set as the first two rows above — carry an
`ETag`: a content fingerprint computed with the FNV-64a hash. It is a weak validator (prefixed with
`W/`) since the same content may be served with different transfer encodings (e.g., gzip).

When a browser makes a subsequent request, it sends the cached ETag in the `If-None-Match` header. If the content has
not changed, gomddoc responds with `304 Not Modified` and an empty body, saving bandwidth and processing time.

What the fingerprint is taken over depends on the route:
- **Markdown pages**: the fully rendered HTML (after template wrapping), so any change to content, metadata, or
  templates produces a new ETag. A `text/markdown` response is hashed as sent.
- **Content files** (images, PDFs, CSS under the content root) and **`/_assets/`**: the raw file content.
- **`/tags/` pages**: the rendered HTML.
- **`/sitemap.xml`, `/feed.xml`, `/robots.txt`**: the generated document, which is built once and
  cached for the life of the process.

The `/api/*` JSON endpoints are the deliberate exception: they stream through a `json.Encoder`
rather than buffering a body, so there is nothing to hash and no conditional-request handling. A
client polling `/api/tags` re-downloads the payload every time.

No configuration is required, and there is no way to turn ETags off.

---

## Compression

gomddoc automatically compresses HTTP responses using gzip when all of the following conditions are met:

1. The client sends an `Accept-Encoding` header that includes `gzip`.
2. The response body is **1 KB or larger**. Smaller responses are sent uncompressed because the compression overhead
   would outweigh the savings.
3. The response content type is **not already compressed**. Binary formats such as images (`image/*`), video
   (`video/*`), audio (`audio/*`), and archive types (`application/zip`, `application/gzip`, etc.) are never
   re-compressed.

When compression is active, the middleware:

- Sets `Content-Encoding: gzip` on the response.
- Removes the `Content-Length` header (the compressed size is not known in advance).
- Always sets `Vary: Accept-Encoding` so that caches distinguish between compressed and uncompressed variants.

Compression requires no configuration and cannot be turned off. It uses a `sync.Pool` of gzip
writers internally to minimize memory allocations under load.

Three route groups sit outside it, each deliberately:

- **`/health/live` and `/health/ready`** — probe bodies are far below the 1 KB threshold, so the
  middleware would never fire anyway.
- **`/metrics`** — `promhttp` negotiates its own encoding.
- **The admin listener** (`--admin-port`) — health, metrics and pprof there are served without any middleware.

Everything else — pages, `/tags/`, `/sitemap.xml`, `/feed.xml`, `/robots.txt`, `/_assets/`,
`/api/*`, `/_mcp/` — is compressed, because the middleware is attached to the root route group
rather than to individual handlers. A route added later gets it by default rather than by
remembering to opt in.

---

## Content Negotiation (Accept Header)

gomddoc uses the HTTP `Accept` header to determine the output format for each request. This
enables the same URL to serve different representations depending on what the client wants — a
pattern known as server-driven content negotiation (RFC 9110 §12.5.1).

### How It Works

When a request arrives for a markdown file, gomddoc inspects the `Accept` header and selects the
best matching renderer. The decision follows standard HTTP quality-value semantics (`q=` weights),
with the most specific media type preferred over wildcards.

| Accept Header | Response Format | Content-Type |
|---------------|----------------|--------------|
| `*/*` (default, browsers) | Fully rendered HTML with theme, navigation, and TOC | `text/html` |
| `text/html` | Fully rendered HTML | `text/html` |
| `text/markdown` | Markdown source with a generated frontmatter block (see below) | `text/markdown; charset=utf-8` |
| Other (e.g., `application/json`) | `406 Not Acceptable` | — |

Non-markdown files (CSS, JavaScript, images, PDFs) are served as-is with their detected MIME type. They go
through the same renderer registry, whose passthrough renderer returns the input type, so an `Accept` header that
excludes that type gets a `406` too. Every negotiated response carries `Vary: Accept`.

### Requesting Raw Markdown

The `text/markdown` output format returns the page's markdown body. The author's frontmatter is removed and
replaced by a generated YAML block with these keys, each omitted when empty:

- `metadata`: the page's parsed frontmatter
- `related_docs`: pages sharing tags with this one (`path` and `title`)
- `prev_page`, `next_page`: the neighbouring pages in navigation order

```markdown
---
metadata:
    description: Step-by-step guides
    tags:
        - guides
    title: Guides
related_docs:
    - path: /guides/configuration.md
      title: Configuration
---

# Guides
```

When there is nothing to report, the body is returned without a frontmatter block. This format is useful for:

- **AI models and LLMs** — token-efficient access to documentation content without HTML markup
- **API consumers** — scripts and tools that process markdown directly
- **Content pipelines** — downstream systems that need the source markdown for further processing

```bash
# Get rendered HTML (default)
curl http://localhost:8080/docs/guide

# Get the markdown source with generated frontmatter
curl -H "Accept: text/markdown" http://localhost:8080/docs/guide

# Explicitly request HTML
curl -H "Accept: text/html" http://localhost:8080/docs/guide
```

Request the extensionless URL: `/docs/guide.md` answers `301` to `/docs/guide` before negotiation runs (see
[Extension Redirect Middleware](#extension-redirect-middleware)), and `curl` does not follow redirects without `-L`.

### Error Handling

When the server cannot produce any of the media types listed in the `Accept` header, it returns
`406 Not Acceptable` with a plain-text body listing the available output formats, so the client can
retry with a valid type:

```bash
# Returns 406 — no renderer produces JSON for markdown input
curl -H "Accept: application/json" http://localhost:8080/docs/guide
# Not Acceptable: available types: text/markdown, text/html
```

### Integration with the MCP Server

The MCP server's `read_page` tool reads the same files but always returns markdown: the body with
frontmatter stripped, preceded by a YAML header of the page's title, description and tags. It takes
a source file path (`docs/guide.md`), not a URL. If you are building integrations for AI models, the
MCP server is the preferred access method — see [MCP Server](../04-mcp.md) for details.

No configuration is required. Content negotiation is always enabled.

---

## URL Resolution

gomddoc serves extensionless (clean) URLs by default. A request to `/guide/setup` resolves to
the underlying file `guide/setup.md` transparently. This section explains how the resolution
pipeline works.

### PathResolver

At startup, gomddoc walks the content filesystem and builds an immutable `PathResolver` — a pair
of maps that translate between extensionless paths and real file paths. The resolver is configured
by the `strip_extensions` site config option (default: `[".md"]`).

For each file in the content tree, the resolver checks:

1. Whether the file extension is in `strip_extensions`.
2. Whether a renderer is registered for the file's MIME type (only renderable content gets clean URLs).
3. Whether the resulting extensionless path collides with an existing mapping or directory.

The resolver produces O(1) lookups in both directions:

- **`Resolve(cleanPath) -> realPath`** — used by the handler when the provider returns "not found"
  for an extensionless path.
- **`CleanPath(realPath) -> cleanPath`** — used by the sitemap, feed, canonical URL, and redirect
  map generators to emit extensionless URLs.

### Extension Redirect Middleware

The `ExtensionRedirect` middleware intercepts requests that include a strippable extension and
issues a `301 Moved Permanently` redirect to the canonical extensionless URL:

```
GET /guide/setup.md  →  301 → /guide/setup
GET /guide/setup     →  (passes through to handler)
```

The redirect only fires when the resolver has a mapping for the requested file. Requests with
extensions not in `strip_extensions` (e.g., `.css`, `.png`) pass through unmodified.

> [!WARNING]
> Known bug, tracked in `REVIEW.md`: the redirect target is the resolver's clean path, which keeps a default index
> file's name. `/guide/README.md` redirects to `/guide/README`, not `/guide/`; `serve` answers that URL, but
> `build` never writes it.

### Handler Resolution

Before reading any file, the handler checks the `redirect_from` map built from page frontmatter; a
match answers `301` with the page's URL.

When the handler receives a request for a path that the provider cannot find (e.g., `/guide/setup`
— no such literal file exists), it consults the `PathResolver`:

1. Strip the leading `/` to get the fs-relative path (`guide/setup`).
2. Call `resolver.Resolve("guide/setup")` — returns `guide/setup.md` if mapped.
3. Re-read the file from the provider using the real path.

The resolver runs *after* the provider's initial lookup fails, not before. Files that
exist at their literal path (images, CSS, non-stripped extensions) are served directly without
resolver involvement.

A directory URL serves its `default_index` file (`README.md` by default). A directory without one
renders a listing when `dir_index` is on, and otherwise answers `302 Found` with the first page of the
site's navigation tree (the first page of the whole site, not of that directory: a known bug, tracked in
`REVIEW.md`).

### 404 Behavior

When a request path has no extension and the resolver has no mapping for it, the handler returns
`404 Not Found`. The resolver does not guess or probe — if the path was not registered at startup,
it does not exist.

### Middleware Ordering

Content requests pass through the middleware chain in this order (outermost first):

1. **SecurityHeaders** and **RequestID** — wrap the whole main-port mux, including `/health/*` and `/metrics`
2. **Compression** — gzip, plus `Vary: Accept-Encoding`
3. **Metrics** — records Prometheus request metrics
4. **BasicAuth** — only when credentials are configured
5. **stripPathPrefix** — per-language routes only; removes the `/{lang}` segment
6. **MethodFilter** — 405 for anything but GET and HEAD
7. **ContentExclusion** — blocks hidden files (dotfiles) and user-configured exclude patterns
8. **ExtensionRedirect** — redirects `.md` (or other stripped extensions) to extensionless URLs
9. **Handler** — resolves path, reads content, renders, and responds

Steps 2 and 3 sit above the filters on purpose: that is what makes the 405s, the 404s from
`ContentExclusion` and the 301s from `ExtensionRedirect` show up in `http_requests_total` at all.
The redirect middleware runs before the handler so that extension-bearing requests never reach
the rendering pipeline — they are redirected immediately.

### Build Mode

In `gomddoc build`, the resolver drives pretty URL output. Instead of generating `guide.html`,
the build command generates `guide/index.html` so that static hosts serve the page at `/guide/`.
Extension redirect HTML files (e.g., `guide.md` containing a meta-refresh redirect to `/guide`)
are also generated so old extension-based URLs work on static hosts that do not support server-side
redirects. `redirect_from` sources get the same kind of meta-refresh page.
