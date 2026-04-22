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

gomddoc uses the HTTP `Accept` header to determine the output format for each request. This enables
the same URL to serve different representations depending on what the client wants.

**Default behavior** (browser requests, `Accept: */*`): Markdown files are rendered as HTML.

**Raw markdown** (`Accept: text/markdown`): Returns the raw markdown content with YAML frontmatter
stripped. Useful for LLMs, API consumers, and scripts that prefer markdown over rendered HTML.

```bash
# Get rendered HTML (default)
curl http://localhost:8080/docs/guide.md

# Get raw markdown
curl -H "Accept: text/markdown" http://localhost:8080/docs/guide.md

# 406 Not Acceptable — no renderer produces JSON for markdown input
curl -H "Accept: application/json" http://localhost:8080/docs/guide.md
```

**Non-markdown files** (CSS, images, etc.): Always served as-is with their detected MIME type,
regardless of the Accept header.

**406 Not Acceptable**: When the server cannot produce any of the requested output types, it returns
406 with a list of available output types in the response body.

No configuration is required. Content negotiation is always enabled.
