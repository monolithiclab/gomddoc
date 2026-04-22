---
title: "HTTP Behavior"
description: "Built-in HTTP features including caching, compression, and content negotiation."
author: "nicolasm"
---

# HTTP Behavior

gomddoc includes built-in HTTP features that require no configuration: caching, compression, and content negotiation.

## Caching

### Cache-Control

All successful responses include a `Cache-Control: public, max-age=300` header, allowing browsers and intermediate caches to store responses for 5 minutes before revalidating.

### ETag / 304 Not Modified

Every response includes an `ETag` header, a content-based fingerprint computed using the FNV-64a hash algorithm. The ETag is a weak validator (prefixed with `W/`) since the same content may be served with different transfer encodings (e.g., gzip).

When a browser makes a subsequent request, it sends the cached ETag in the `If-None-Match` header. If the content has not changed, gomddoc responds with `304 Not Modified` and an empty body, saving bandwidth and processing time.

This applies to all content types:
- **Markdown pages**: The ETag is computed from the fully rendered HTML (after template wrapping), so any change to content, metadata, or templates produces a new ETag.
- **Static assets** (CSS, JS, images): The ETag is computed from the raw file content.

No configuration is required. ETag caching is always enabled.

---

## Compression

gomddoc automatically compresses HTTP responses using gzip when all of the following conditions are met:

1. The client sends an `Accept-Encoding` header that includes `gzip`.
2. The response body is **1 KB or larger**. Smaller responses are sent uncompressed because the compression overhead would outweigh the savings.
3. The response content type is **not already compressed**. Binary formats such as images (`image/*`), video (`video/*`), audio (`audio/*`), and archive types (`application/zip`, `application/gzip`, etc.) are never re-compressed.

When compression is active, the middleware:

- Sets `Content-Encoding: gzip` on the response.
- Removes the `Content-Length` header (the compressed size is not known in advance).
- Always sets `Vary: Accept-Encoding` so that caches distinguish between compressed and uncompressed variants.

Compression requires no configuration and is always enabled. It uses a `sync.Pool` of gzip writers internally to minimize memory allocations under load.

---

## Content Negotiation (Accept Header)

gomddoc uses the HTTP `Accept` header to determine the output format for each request. This
enables the same URL to serve different representations depending on what the client wants — a
pattern known as server-driven content negotiation (RFC 7231 §5.3).

### How It Works

When a request arrives for a markdown file, gomddoc inspects the `Accept` header and selects the
best matching renderer. The decision follows standard HTTP quality-value semantics (`q=` weights),
with the most specific media type preferred over wildcards.

| Accept Header | Response Format | Content-Type |
|---------------|----------------|--------------|
| `*/*` (default, browsers) | Fully rendered HTML with theme, navigation, and TOC | `text/html` |
| `text/html` | Fully rendered HTML | `text/html` |
| `text/markdown` | Raw markdown with YAML frontmatter stripped | `text/markdown` |
| Other (e.g., `application/json`) | `406 Not Acceptable` | — |

Non-markdown files (CSS, JavaScript, images, PDFs) are always served as-is with their detected
MIME type, regardless of the `Accept` header. Content negotiation only applies to markdown content.

### Requesting Raw Markdown

The `text/markdown` output format returns the page content as clean markdown with YAML frontmatter
removed. This is useful for:

- **AI models and LLMs** — token-efficient access to documentation content without HTML markup
- **API consumers** — scripts and tools that process markdown directly
- **Content pipelines** — downstream systems that need the source markdown for further processing

```bash
# Get rendered HTML (default)
curl http://localhost:8080/docs/guide.md

# Get raw markdown (frontmatter stripped)
curl -H "Accept: text/markdown" http://localhost:8080/docs/guide.md

# Explicitly request HTML
curl -H "Accept: text/html" http://localhost:8080/docs/guide.md
```

### Error Handling

When the server cannot produce any of the media types listed in the `Accept` header, it returns
`406 Not Acceptable` with a plain-text body listing the available output formats. This tells the
client exactly which formats are supported so it can retry with a valid type:

```bash
# Returns 406 — no renderer produces JSON for markdown input
curl -H "Accept: application/json" http://localhost:8080/docs/guide.md
```

### Integration with the MCP Server

The MCP server's `read_page` tool uses the same content pipeline but always returns markdown
(equivalent to `Accept: text/markdown`). If you are building integrations for AI models, the
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

### Handler Resolution

When the handler receives a request for a path that the provider cannot find (e.g., `/guide/setup`
— no such literal file exists), it consults the `PathResolver`:

1. Strip the leading `/` to get the fs-relative path (`guide/setup`).
2. Call `resolver.Resolve("guide/setup")` — returns `guide/setup.md` if mapped.
3. Re-read the file from the provider using the real path.

This means the resolver runs *after* the provider's initial lookup fails, not before. Files that
exist at their literal path (images, CSS, non-stripped extensions) are served directly without
resolver involvement.

### 404 Behavior

When a request path has no extension and the resolver has no mapping for it, the handler returns
`404 Not Found`. The resolver does not guess or probe — if the path was not registered at startup,
it does not exist.

### Middleware Ordering

Content requests pass through the middleware chain in this order:

1. **ContentExclusion** — blocks hidden files (dotfiles) and user-configured exclude patterns
2. **ExtensionRedirect** — redirects `.md` (or other stripped extensions) to extensionless URLs
3. **Metrics** — records Prometheus request metrics
4. **Handler** — resolves path, reads content, renders, and responds

The redirect middleware runs before the handler so that extension-bearing requests never reach
the rendering pipeline — they are redirected immediately.

### Build Mode

In `gomddoc build`, the resolver drives pretty URL output. Instead of generating `guide.html`,
the build command generates `guide/index.html` so that static hosts serve the page at `/guide/`.
Extension redirect HTML files (e.g., `guide.md` containing a meta-refresh redirect to `/guide`)
are also generated so old extension-based URLs work on static hosts that do not support server-side
redirects.
