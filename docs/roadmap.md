---
title: "Gomddoc Roadmap"
description: "Strategic roadmap for gomddoc evolution."
author: "nicolasm"
---

# Gomddoc Roadmap

## Vision

Transform gomddoc from a documentation server into the best git-native documentation viewer and static site
generator. No databases, no editorial workflows, no CMS. The "database" is Git.

- **Read-heavy focus**: Search, speed, and rendering quality over write features
- **Stateless**: No persistent storage beyond Git repos
- **Interface-driven**: All major components implement well-defined interfaces
- **Performance-first**: Built for speed with intelligent caching
- **Security by design**: Path traversal protection, content sanitization, secure defaults

## Current State

The foundation is production-ready with 87.9% test coverage. For a full description
of current capabilities, see `docs/architecture.md`.

**Completed phases:** 1-3 (core), 4 (partial), 5 (partial), 6 (renderer enhancement),
7 (partial), 7b (preview), 8 (theming engine), 9a (pre-launch SEO).

**Phase 5 note:** Server-side full-text search and client-side search UI are complete for `serve`
and `preview` modes. Build-mode search (Pagefind) deferred.

**Phase 4 note:** Benchmarks, pprof, and allocation reduction are complete. CI benchmark
tracking is deferred until a CI pipeline is established. Partial clones are blocked by
go-git library limitations.

## Phase 4: Performance and Scaling

- [ ] **Partial clones**: `git clone --filter=blob:none` when upstream library support matures.
- [ ] **CI benchmark tracking**: Run benchmarks in CI with `go test -bench -benchmem`. Use
      `benchstat` to detect regressions against the baseline. Fail CI on >10% degradation.
      Deferred until CI pipeline is established. Makefile targets (`bench-save`, `bench-compare`)
      support local regression detection.
- [x] **Allocation reduction**: Profiled and reduced allocations in the request hot path.
      ETag generation: inline FNV-64a + `strconv.AppendUint` (1 alloc, down from 2+).
      ETag checking: zero allocs. Content negotiation: hand-rolled Accept parser replacing
      `mime.ParseMediaType` (2-4 allocs, down from 5-12). `Matches()`: zero allocs.
      Registry lookup: zero allocs (down from 5-12) via `strings.IndexByte` and stack-allocated
      candidate arrays. Compression: `sync.Pool` for buffers. `NormalizeMimeType`: fast-path
      for parameter-free MIME types.

## Phase 5: Search and Discovery

- [x] **Full-text search (serve)**: Stdlib inverted index (`internal/search/`) built at startup. TF-IDF ranking
      with title/description boosts, AND query semantics, snippet generation with `<mark>` highlighting.
      API: `GET /api/search?q=<query>&limit=<n>`. Enabled in `serve` and `preview` modes.
- [x] **Search UI**: Client-side search modal shared across all 8 themes via `inlineAsset "search.mjs"`.
      Ctrl+K / Cmd+K shortcut, debounced API fetch, arrow key navigation, highlighted snippets.
      CSS uses theme custom properties for automatic cross-theme and dark mode compatibility.
- [ ] **Full-text search (build)**: Option B (Pagefind) as optional post-build step.

## Phase 7: Static Site Generation

- [ ] **Sitemap generation**: Generate `sitemap.xml` during build with `<url>` entries for all rendered
      pages. Include `<lastmod>` from Git commit dates (filesystem provider falls back to file mtime).
      Respects `base_url` from site config for absolute URLs. Excludes hidden files and non-HTML outputs.
      See also Phase 9a for dynamic `/sitemap.xml` in serve mode.
- [ ] **Asset optimization**: Minify HTML/CSS/JS during build.
- [ ] **Static host compatibility**: Output structure compatible with S3, Netlify, Cloudflare Pages.

## Phase 9: SEO and Discoverability

_Technical SEO features to compete with MkDocs Material, Docusaurus, and Hugo for search rankings.
gomddoc's server-rendered HTML with no client-side framework is a natural Core Web Vitals advantage —
these items close the gap on crawl management, structured data, and social sharing.
See `docs/seo-competitive-analysis.md` for full competitive analysis._

### 9a: Pre-Launch SEO (P0)

_Must ship before public launch. These are table-stakes features that every documentation tool provides._

- [x] **Canonical URLs**: `<link rel="canonical">` on every page via `canonicalURL` template function.
      Uses `meta.domain` config + page path with `net/url` for proper URL construction. Default index
      files stripped from URLs.
- [x] **XML sitemap**: `/sitemap.xml` served dynamically in `serve` mode and generated during `build`.
      Uses metadata index for page discovery. `<lastmod>` deferred to Git-based timestamps (Phase 9b).
- [x] **`robots.txt`**: `/robots.txt` served in `serve` mode and generated during `build`.
      Blocks `/_assets/`, `/api/`, `/debug/`. Includes `Sitemap:` directive when domain is configured.
- [x] **Open Graph meta tags**: `og:title`, `og:description`, `og:url`, `og:type`, `og:site_name` on
      every page. `og:type` defaults to `article` but overridable via frontmatter `og_type` field.
      Matching `twitter:card` summary tags. All in default theme's `head.html.tmpl` partial (inherited
      by all 8 themes).

### 9b: Post-Launch SEO (P1)

_High-impact features that differentiate gomddoc from competitors._

- [ ] **JSON-LD structured data**: Inject `<script type="application/ld+json">` with Schema.org
      markup. Per-page `TechArticle` schema (headline, datePublished, dateModified, author,
      description). `BreadcrumbList` schema matching breadcrumb navigation. `WebSite` schema on
      index page (with `SearchAction` when search is implemented). Most documentation tools lack
      this — competitive advantage for rich snippets. Medium complexity.
- [ ] **Auto-generated meta description**: When frontmatter `description` is missing, extract
      first ~160 characters of rendered text (strip Markdown/HTML, truncate at word boundary).
      Ensures every page has a reasonable description without requiring frontmatter. Low complexity.
- [ ] **Git-based timestamps**: Expose `datePublished` (first commit date) and `dateModified`
      (last commit date) per page from Git provider. Filesystem provider falls back to mtime.
      Used in JSON-LD, sitemap `<lastmod>`, and optionally displayed in UI ("Last updated on...").
      Cache at startup or lazily to avoid per-request Git operations. Medium complexity.
- [ ] **Social preview image generation**: Auto-generate OG images (1200×630 PNG) per page from
      title, description, and site branding. Used as `og:image`. Build-time only (too expensive
      for serve mode). MkDocs Material's most popular feature. High complexity — requires image
      generation in Go (e.g., `fogleman/gg`).

### 9c: SEO Polish (P2)

_Enhances competitiveness and closes remaining gaps._

- [ ] **Per-page `robots` meta tag**: Frontmatter `robots: noindex` or `robots: noindex, nofollow`
      controls per-page indexing. Site-wide default via config. Needed for draft pages or pages
      that shouldn't appear in search results. Low complexity.
- [ ] **HTML `lang` attribute**: Add `lang` attribute to `<html>` tag (e.g., `<html lang="en">`).
      Configurable via `language` field in `SiteConfig`, default `"en"`. Lighthouse flags its
      absence. Low complexity.
- [ ] **Related pages via tags**: Display "Related pages" section at page bottom, populated from
      shared frontmatter tags. Enricher already computes `RelatedDocs` via `ByTag()` — expose
      via template function or enrich into `PageContext`. Strong internal linking signal. Medium
      complexity (enricher done, needs template integration).
- [ ] **404 page with navigation**: Custom 404 page including site navigation and suggested pages.
      In `build` mode, output `404.html` (convention for Netlify, GitHub Pages, Cloudflare Pages).
      Low complexity.
- [ ] **Heading anchor slug stability**: Document and test the slug algorithm for long-term URL
      stability. Ensure GitHub-compatible slugs. Support frontmatter `id` override per heading.
      Low complexity.

### 9d: SEO Long-Term (P3)

- [ ] **Redirect support**: Frontmatter `redirect_from: [/old-url]` and/or `_redirects` file.
      In `serve` mode, 301 responses. In `build` mode, generate redirect HTML or `_redirects`
      file for static hosts. Medium complexity.
- [ ] **RSS/Atom feed**: Generate `/feed.xml` listing recently modified pages. Minor SEO impact
      but aids content discoverability. Medium complexity.
- [ ] **Preconnect/preload resource hints**: Add `<link rel="preconnect">` for external domains
      (Google Fonts, KaTeX/Mermaid CDNs) and `<link rel="preload">` for critical resources in
      theme `<head>`. Improves LCP. Low complexity.
- [ ] **Image dimension attributes**: Post-process rendered HTML to add `width`/`height` to
      `<img>` tags that lack them. Prevents CLS (Core Web Vitals). Reads dimensions from content
      provider. Medium complexity.
- [ ] **`<link rel="next/prev">`**: Sequential page links derived from navigation order. Minor
      crawl efficiency signal. Low complexity.

## Phase 10: MCP Interface — AI-Native Documentation Access

_Expose documentation content via the [Model Context Protocol](https://modelcontextprotocol.io) so AI
models (Claude, GPT, Copilot, etc.) can directly read, search, and navigate documentation. This turns
gomddoc into an AI-queryable knowledge base — a strategic differentiator no other documentation tool
offers natively._

**Why MCP over HTTP APIs:** MCP is purpose-built for AI tool use. Models discover capabilities via
the protocol, not by reading API docs. A single `gomddoc mcp` command or SSE endpoint makes every
document instantly available to any MCP-compatible client (Claude Desktop, Cursor, Windsurf, custom
agents) with zero configuration on the model side.

**Architecture fit:** The MCP server is a thin adapter over existing gomddoc internals. No new parsing
logic — it reuses Provider (file I/O), Enricher (metadata/TOC extraction), and Metadata Index (tag
queries) directly. New package: `internal/mcp/`.

```
MCP Client (Claude, etc.)
    ↕ JSON-RPC (stdio or SSE)
internal/mcp/server.go
    ↓ adapts to existing interfaces
Provider → Enricher → Metadata Index → Navigation
```

### 10a: Core MCP Server

_Minimum viable MCP server with file access and metadata._

- [ ] **`internal/mcp/` package**: MCP server implementation using a Go MCP SDK (e.g.,
      `github.com/mark3labs/mcp-go`). Implements the MCP server protocol with resource and
      tool capabilities. Stateless — instantiated with Provider, EnricherRegistry, and
      metadata Index references.
- [ ] **`gomddoc mcp` subcommand**: stdio transport for local use. Speaks MCP JSON-RPC over
      stdin/stdout. Works with Claude Desktop (`claude_desktop_config.json`), Cursor, and any
      MCP-compatible client. Kong subcommand with same positional `dir` argument as `serve`.
- [ ] **Resource: `docs://list`**: List all documentation files in the content tree. Returns
      paths, titles (from frontmatter or filename), and MIME types. Backed by `Provider.RootFS`
      + `fs.WalkDir`. Supports optional `path` parameter to list a subdirectory.
- [ ] **Resource: `docs://{path}`**: Read a specific file's content. Returns raw markdown with
      frontmatter stripped (clean content for model consumption). Backed by `Provider.ReadFile` +
      frontmatter stripping (reuses `MarkdownPassthroughRenderer` logic). Non-markdown files
      returned as-is.
- [ ] **Tool: `get_page_metadata`**: Extract structured metadata for a file — title, description,
      tags, TOC structure, related documents. Backed by `EnricherRegistry.Get(mime).Enrich()`.
      Returns JSON with `metadata`, `toc`, and `related_docs` fields.
- [ ] **Tool: `list_tags`**: List all tags across the documentation. Backed by
      `metadata.Index.AllTags()`. Returns sorted string array.
- [ ] **Tool: `search_by_tag`**: Find all pages with a given tag. Backed by
      `metadata.Index.ByTag()`. Returns array of `{path, title, description}`.
- [ ] **Tool: `list_pages`**: List all indexed pages with metadata. Backed by
      `metadata.Index.AllPages()`. Supports optional tag filter. Returns array of `PageInfo`.

### 10b: Advanced MCP Capabilities

_Richer AI interactions — search, navigation context, and batch access._

- [ ] **SSE transport**: HTTP-based MCP endpoint at `/_mcp/` on the existing server. Enables
      remote MCP access without stdio. Shares the server's Provider and metadata instances.
      Protected by the same middleware (auth, rate limiting) as other endpoints.
- [ ] **Tool: `search`**: Full-text search across all documentation. Backed by Phase 5 search
      index (when available). Returns ranked results with snippets. Falls back to tag search
      if full-text search is not configured.
- [ ] **Tool: `get_navigation`**: Get the navigation tree for a given path — what sections exist,
      what's adjacent. Backed by `navigation.Generator.Generate()`. Helps models understand
      documentation structure and find relevant sections.
- [ ] **Tool: `get_section`**: Read a specific section of a document by heading ID. Extracts
      content between two headings using the TOC structure. Useful for targeted retrieval
      without loading entire documents.
- [ ] **Tool: `find_related`**: Find documents related to a given page via shared tags, same
      directory, or similar metadata. Backed by `EnricherRegistry` + `metadata.Index`.
      Returns ranked list with relevance reason.
- [ ] **Batch resource reads**: Support reading multiple files in a single request. Reduces
      round trips for models that need context from several documents simultaneously.

### 10c: MCP for Static Sites

_MCP capabilities for `gomddoc build` output, enabling AI access to pre-built documentation._

- [ ] **`gomddoc mcp --built-dir`**: Serve MCP from a `gomddoc build` output directory.
      Uses filesystem provider over the built output. Metadata index built from rendered HTML
      or a pre-generated `metadata.json` manifest.
- [ ] **Build-time manifest**: `gomddoc build` generates `_mcp/manifest.json` containing all
      page metadata, tags, TOC structures, and relationships. Enables lightweight MCP serving
      without re-parsing content.

## Phase 11: Enterprise Features

- [ ] **S3 provider**: Serve content directly from S3 buckets (for non-git use cases).
- [ ] **Authentication**: OIDC/OAuth middleware for private documentation.
- [ ] **Branch switching**: UI dropdown to switch between Git branches/tags.

## Phase 12: Git Workflow Integration

- [ ] **Webhooks**: Endpoint to trigger `git fetch` on push events (cache invalidation).
- [ ] **PR preview**: Serve content from PR branches for review.

## Phase 13: Theme Marketplace

_Enable community theme sharing via a GitHub-based registry._

### 13a: Theme Registry

- [ ] **Default marketplace repository**: A GitHub repository (e.g. `gomddoc/themes`) acts as the theme
      registry. Contains an `index.json` manifest listing available themes with name, description, author,
      version, repository URL, and preview image URLs.
- [ ] **Theme package format**: Each theme is a Git repository containing `default.html.tmpl`, optional
      partials, a `README.md` with frontmatter metadata (name, category, fonts, colors), and screenshot
      previews (`light.png`, `dark.png`).
- [ ] **Custom registries**: Users can configure alternative marketplace URLs in `.gomddoc/config.yml`
      via `theme_registry: https://github.com/org/custom-themes` for private or corporate theme registries.

### 13b: `gomddoc theme` Subcommand

- [ ] **`gomddoc theme list`**: Fetch and display available themes from the marketplace. Shows name,
      category, author, and short description in a table. Supports `--json` output.
- [ ] **`gomddoc theme search <query>`**: Filter themes by name, category, or keyword. Example:
      `gomddoc theme search dark` finds midnight, nord, etc.
- [ ] **`gomddoc theme info <name>`**: Show detailed theme information: full description, color palette,
      fonts, screenshot URLs, download URL, and installation instructions.
- [ ] **`gomddoc theme install <name>`**: Download the theme into `.gomddoc/themes/<name>/`. Clones the
      theme repository (sparse checkout of the theme directory if from the default registry). Updates
      `.gomddoc/config.yml` to set `theme: <name>`.
- [ ] **`gomddoc theme update [name]`**: Pull latest version of installed theme(s). Without a name,
      updates all installed themes.

## Future CLI Commands

| Command                 | Purpose                                              | Status  |
| ----------------------- | ---------------------------------------------------- | ------- |
| `gomddoc serve`         | Production HTTP server for documentation             | Done    |
| `gomddoc build`         | Static site generation                               | Done    |
| `gomddoc preview`       | Quick local preview with auto-open browser           | Done    |
| `gomddoc init`          | Scaffold a `.gomddoc/` directory with default config | Done    |
| `gomddoc mcp`            | MCP server for AI-native documentation access        | Planned |
| `gomddoc validate`      | Validate config and check for broken links           | Planned |
| `gomddoc theme list`    | List available themes from the marketplace           | Planned |
| `gomddoc theme search`  | Search themes by name, category, or keyword          | Planned |
| `gomddoc theme info`    | Show detailed theme information                      | Planned |
| `gomddoc theme install` | Download and install a theme                         | Planned |
| `gomddoc theme update`  | Update installed theme(s) to latest version          | Planned |

## Implementation Strategy

Development proceeds in phases building on stable foundations. Each phase delivers complete, tested functionality.

**Immediate focus (Phase 9b):** Post-launch SEO (JSON-LD, Git timestamps, social images).
**High-value (Phase 10a):** MCP interface — low complexity (thin adapter over existing layers), high differentiation.
**Deferred:** CI benchmark tracking (Phase 4, needs CI pipeline), build-mode search (Phase 5, Pagefind).

## Deferred (Not Planned)

These features were considered but deprioritized to keep gomddoc focused:

- Database providers (PostgreSQL/SQLite) — Git is the database
- REST/GraphQL APIs — gomddoc is a viewer, not a headless CMS
- Editorial workflows (drafts, reviews, scheduling) — use Git branches
- Content versioning/revision history — use Git history
- i18n/multi-language — out of scope for now
- VS Code extension
- Plugin architecture / dynamic renderer loading
- AsciiDoc renderer (`.adoc` support) — niche demand, adds CGO or subprocess dependency
- OpenAPI renderer (interactive API docs) — better served by dedicated tools (Swagger UI, Redoc)
- Server-side Mermaid/KaTeX rendering — client-side CDN approach works well, avoids heavy dependencies
- SEO content analysis / keyword scoring (Yoast-style — content strategy, not technical SEO)
- AMP (deprecated as ranking signal by Google)
- News/video sitemaps (documentation sites don't need these)
- WebP/AVIF image conversion (belongs in CI/CD pipeline, not documentation server)
