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

## Current State (Phase 8 In Progress)

The foundation is production-ready with comprehensive test coverage across internal packages:

- **CLI**: Kong-based subcommand architecture (`gomddoc serve`, `gomddoc build`), version injection via ldflags,
  exhaustive `--help` with env var discovery.
- **Configuration**: Reflection-based env var walking, CLI flags, YAML config files with proper validation
  and correct prefix nesting (`GOMDDOC_SITE_*`). Priority: flags > env > file > defaults.
- **Providers**: Filesystem (os.DirFS with path traversal protection) and Git (go-git, memory or disk-based
  storage, SSH auth with known_hosts, fail-closed).
- **Rendering**: Markdown (goldmark, GFM, syntax highlighting, admonitions, color chips, TOC, heading anchors,
  YAML frontmatter), passthrough for all other MIME types via `*/*` wildcard. Post-processing pipeline:
  goldmark → heading anchors → admonitions → color chips.
- **Theming**: 8 bundled themes (default, academic, gitbook, material, midnight, minimal, nord, ocean) with
  light/dark mode, TOC scroll highlighting, touch device accessibility, copy-to-clipboard code blocks,
  KaTeX math rendering, and Mermaid diagram support. Color chips rendered via a `<color-chip>` web component
  with Shadow DOM encapsulation.
- **HTTP**: Two-dimensional content negotiation (input type + Accept header), gzip compression,
  security headers, ETag, request ID tracking, graceful shutdown.
- **Monitoring**: Prometheus metrics (`/metrics`), health probes (`/health/live`, `/health/ready`).
- **Security**: Hidden file blocking, method filtering (GET/HEAD), clone timeout, file size limits.
- **Navigation**: Auto-generated sidebar from directory structure with collapsible directories, active state
  tracking, and title extraction from markdown headings.
- **Metadata**: Frontmatter indexing across all pages with JSON API (`/api/tags`, `/api/tags/{tag}`).
- **Static site generation**: `gomddoc build` command for deploying to S3, Netlify, GitHub Pages.
- **Edit links**: Configurable `edit_url` in site config with "Edit this page" footer links.
- **Template functions**: `navigation`, `toc`, `breadcrumbs`, `editURL`, `inlineAsset` available in themes.
- **Documentation**: User guides (`docs/guide/`), architecture reference (`docs/architecture.md`).

See `docs/architecture.md` for detailed architecture and `docs/guide/` for user documentation.

## Phase 4: Performance and Scaling (Partial)

- [x] **Disk-based Git storage**: `DiskStorageFactory` for the Git provider via `--git-storage-dir` flag.
      Prevents OOM on large repos by cloning to disk instead of memory.
- [ ] **Partial clones**: `git clone --filter=blob:none` when upstream library support matures.
- [x] **Auto-port assignment**: `--port :auto` (or `-p :auto`) scans for an available port starting from
      8080. Works in any mode, not just dev. Port discovery via sequential `net.Listen` scan.
- [ ] **Profiling and benchmarks**: Add comprehensive benchmarks and `pprof` integration for
      data-driven performance optimization.
  - [ ] **Benchmark suite**: Table-driven benchmarks for hot-path functions: markdown rendering
        (`MarkdownRenderer.Render`), enrichment (`MarkdownEnricher.Enrich`), content negotiation
        (`DefaultRegistry.Get`), template rendering (`HTMLRenderer.Render`), and compression
        (`CompressResponseWriter`). Include small, medium, and large document sizes.
  - [ ] **`pprof` endpoint**: Expose `/debug/pprof/` behind a `--pprof` flag (disabled by default,
        never in production). CPU, heap, goroutine, and mutex profiles available at runtime.
  - [ ] **CI benchmark tracking**: Run benchmarks in CI with `go test -bench -benchmem`. Use
        `benchstat` to detect regressions against the baseline. Fail CI on >10% degradation.
  - [ ] **Allocation reduction**: Profile and reduce allocations in the request hot path
        (provider → enricher → renderer → template → compress → serve). Target zero-alloc for
        ETag checks and content negotiation.

## Phase 5: Search and Discovery (Partial)

- [ ] **Full-text search**: Decision pending — Option C (stdlib inverted index) for serve, Option B (Pagefind)
      for build. See `docs/decisions.md` for analysis.
- [x] **Auto-navigation**: Sidebar navigation generated from directory structure with collapsible `<details>`
      elements, title extraction, and active path highlighting.
- [x] **Metadata indexing**: Lightweight frontmatter parser indexes tags/categories across all pages.
      JSON API: `GET /api/tags` and `GET /api/tags/{tag}`.
- [x] **Rethink `dir_index`**: When `dir_index=false` (default), directories without an index file now
      redirect (302) to the first page in the navigation tree instead of returning 403. The synthetic
      listing (`dir_index=true`) is preserved as a legacy option. Empty directories still return 403.

## Phase 6: Renderer Enhancement (Partial)

_Expand the rendering pipeline with proper content negotiation, enrichment, and format support._

### 6a: Two-Dimensional Content Negotiation (Done)

Reworked the renderer registry so renderers declare both **input** and **output** MIME types. The handler
uses the file's MIME type and the client's `Accept` header to select the best renderer before rendering.

- [x] **`internal/negotiate` package**: Extracted `MediaType`, `ParseAccept`, `Matches` from
      `internal/server/accept.go` into a standalone package to avoid import cycles between renderer and server.
- [x] **Renderer interface change**: Replaced `SupportedMimeTypes() []string` with two methods:
      `InputMimeTypes() []string` and `OutputMimeTypes() []string`. Each renderer declares its
      input→output transformation.
- [x] **Registry two-dimensional lookup**: `Get(inputMimeType, acceptedTypes)` returns `(renderer,
      selectedOutputType, error)`. Selection algorithm: filter by input match, then rank by output
      specificity (exact=3 > type/*=2 > */*=1). On tie, latest registered wins (allows overrides).
      Wildcard outputs resolve to the input type before matching.
- [x] **MarkdownRenderer**: input `["text/markdown"]`, output `["text/html"]`. Unchanged behavior.
- [x] **MarkdownPassthroughRenderer**: input `["text/markdown"]`, output `["text/markdown"]`. Returns
      raw markdown with frontmatter stripped and metadata/TOC extracted. Enables LLM-friendly API access
      via `Accept: text/markdown`.
- [x] **PassthroughRenderer**: input `["*/*"]`, output `["*/*"]`. Catch-all fallback, lowest priority.
- [x] **Handler update**: Content negotiation now happens _before_ rendering. New flow:
      `ReadFile → ParseAccept → Get(input, accepted) → Render → Serve`. No wasted CPU on 406 responses.
- [x] **406 Not Acceptable**: When no renderer matches, returns 406 with a list of available output types.
- [x] **Build command**: Registry-based dispatch replaces hardcoded `text/markdown` check. Uses
      `registry.Get(mimeType, htmlAccept)` to determine renderable files.

### 6b: Content Enricher Pipeline (Done)

Introduced an `Enricher` step that runs before rendering. The enricher extracts structured data from
content (metadata, TOC, navigation, related documents) independent of the output format. Renderers
receive enrichment data and focus solely on content transformation.

- [x] **`internal/enricher/` package**: New package with `EnrichmentData`, `TOCNode`, `NavTree`, `NavItem`,
      `RelatedDoc` types, `Enricher` and `EnricherRegistry` interfaces.
- [x] **MarkdownEnricher**: Lightweight goldmark (GFM + meta, no highlighting) extracts YAML frontmatter,
      builds TOC from headings, and finds related documents via shared tags (using metadata index).
      Accepts `NavBuilder` function for navigation (avoids import cycles).
- [x] **NoOpEnricher**: Returns empty `EnrichmentData{}`. Used as registry fallback for non-markdown types.
- [x] **Enricher registry**: MIME-type-keyed `DefaultEnricherRegistry` with `sync.RWMutex`. `Get()` never
      returns nil — falls back to NoOpEnricher. Normalizes MIME types (strips charset params).
- [x] **Render signature change**: `Render(ctx, content, enrichment)` — renderers receive `*EnrichmentData`.
      Simplified `RenderResult` to just `Content` + `MimeType` (metadata/TOC removed, provided by enricher).
- [x] **Handler pipeline**: `ReadFile → Enrich(content, path) → ParseAccept → Get(input, accepted)
      → Render(content, enrichment) → serveHTML(content, enrichment)`. Enrichment happens before rendering.
- [x] **TOCNode moved**: From `renderer` to `enricher` package. Template package imports `enricher` directly.
- [x] **Renderer cleanup**: MarkdownRenderer removed metadata/TOC extraction (still parses markdown for
      HTML + post-processing). MarkdownPassthroughRenderer simplified to just frontmatter stripping.
- [x] **Navigation via enricher**: Navigation tree generation moved from template layer into the enricher
      pipeline. `MarkdownEnricher` accepts a `NavBuilder` function (wraps `navigation.Generator` to avoid
      import cycles), stores the tree in `EnrichmentData.Navigation`, and passes it to templates via
      `PageContext.Navigation`. Template `{{ navigation .Page.Navigation }}` renders from the enriched data.
      Handler uses a `RedirectFinder` closure for dir-no-index redirects, breaking the server→navigation
      import dependency.

### 6c: Additional Renderers (Partial)

_Expand format support for technical documentation._

- [x] **KaTeX math rendering**: Client-side via CDN with `$...$` (inline) and `$$...$$` (display) delimiters.
      Auto-render extension scans page content on load.
- [x] **Mermaid diagrams**: Client-side via CDN with fenced `mermaid` code blocks. Theme-aware (dark/light).

## Phase 7: Static Site Generation (Partial)

- [x] **`gomddoc build` command**: Walk content tree, render markdown through template pipeline, output
      static HTML. Generates `index.html` alongside `README.html` for clean URLs. Copies non-markdown files as-is.
- [ ] **Sitemap generation**: Generate `sitemap.xml` during build with `<url>` entries for all rendered
      pages. Include `<lastmod>` from Git commit dates (filesystem provider falls back to file mtime).
      Respects `base_url` from site config for absolute URLs. Excludes hidden files and non-HTML outputs.
      See also Phase 9a for dynamic `/sitemap.xml` in serve mode.
- [ ] **Asset optimization**: Minify HTML/CSS/JS during build.
- [ ] **Static host compatibility**: Output structure compatible with S3, Netlify, Cloudflare Pages.

## Phase 7b: `gomddoc preview` — Quick Local Preview (Done)

_A zero-friction subcommand for previewing documentation locally, distinct from the production-grade `serve`._

- [x] **`gomddoc preview [dir]`**: Serve a directory (defaults to `.`) with sensible defaults optimized for
      local authoring. Differences from `serve`:
  - **Auto port**: Find an available port starting from 8080 (`--port :auto`).
  - **Auto open**: Launch the default browser on startup (`--no-open` to disable).
  - **Dev mode on**: Implies `--dev` (no caching) without requiring the flag.
  - **Minimal output**: Print only the URL and "Press Ctrl+C to stop".
- [x] **Port discovery**: Sequential scan from 8080 via `net.Listen`. Port is resolved before server
      creation so the startup log shows a working clickable URL.
- [x] **Browser open**: Uses `open` (macOS), `xdg-open` (Linux), `start` (Windows). Respects `$BROWSER`.

## Phase 8: Theming Engine

_Transform themes from monolithic templates into composable, configurable, distributable packages._

### 8a: Theme Folder Structure

Rework the current flat structure (`<theme>/default.html.tmpl`) into a well-defined package layout
that supports partials, multiple page types, and static assets.

- [x] **Canonical structure**: Restructured each theme directory to composable layout:
  ```
  <theme>/
  ├── README.md              # Enriched frontmatter metadata + description
  ├── layouts/
  │   └── default.html.tmpl   # Skeleton calling partials (~40-50 lines)
  ├── partials/
  │   ├── head.html.tmpl     # <head>: meta, fonts, CSS
  │   ├── header.html.tmpl   # Site header bar
  │   ├── nav.html.tmpl      # Navigation sidebar (self-contained)
  │   ├── toc.html.tmpl      # TOC sidebar (self-contained)
  │   └── scripts.html.tmpl  # All <script> blocks
  └── screenshots/
      ├── desktop-light.png
      └── desktop-dark.png
  ```
- [x] **Migration**: Moved `default.html.tmpl` into `layouts/`, screenshots into `screenshots/`.
- [x] **Embedded themes update**: Reworked default theme (`cmd/gomddoc/assets/themes/default/`) and
      all 7 material themes to the new structure. Template renderer updated to parse layouts + partials.
- [ ] **Page type templates**: Support `page.html.tmpl` and other layout variants (future).
- [ ] **Static asset directory**: Theme-specific `static/` directory for non-inline assets (future).

### 8b: Template Partials

Break the monolithic `default.html.tmpl` into composable partials that themes can selectively override.

- [x] **Partial system**: Split all theme layouts into `head.html.tmpl`, `header.html.tmpl`, `nav.html.tmpl`,
      `toc.html.tmpl`, `scripts.html.tmpl`. Main layout assembles partials via `{{ template "head" . }}` etc.
      Each partial is self-contained and calls its own template functions.
- [ ] **Partial override resolution**: Theme provides base partials; site `.gomddoc/partials/` overrides
      specific ones without copying the whole theme. Resolution order: site partials > theme partials > default.
- [x] **UI polish**: Copy-to-clipboard for code blocks.
- [x] **Dark mode**: Native light/dark toggle with `prefers-color-scheme` fallback.
- [x] **8 bundled themes**: default, academic, gitbook, material, midnight, minimal, nord, ocean.
      Each with light/dark screenshots, responsive design, and full feature parity.
- [x] **TOC scroll highlighting**: Active heading tracking in table of contents sidebar.
- [x] **Touch device accessibility**: `@media (hover: none)` shows copy buttons and heading anchors by default.
- [x] **Color chip web component**: `<color-chip>` custom element with Shadow DOM, click-to-copy, accessible.
- [x] **`inlineAsset` template function**: Load shared assets (JS/CSS) from theme or shared directory.

### 8c: Theme Variables

Expose theme colors and typography as named variables defined in the theme `README.md` frontmatter and
overridable from the site config.

- [ ] **Variable extraction**: Parse theme `README.md` frontmatter `colors` map (light/dark background,
      text, primary, etc.) into CSS custom properties injected at render time.
- [ ] **Config overrides**: Allow `theme_vars` in `.gomddoc/config.yml` to override any theme variable
      without forking the theme. Example: `theme_vars: { primary: "#e63946" }`.
- [ ] **Extended palette**: Themes may define additional variables (accent, border, code-bg, etc.) beyond
      the required `background`, `text`, `primary`. All are overridable.

### 8d: Page Types

Support multiple layout variants selectable from content frontmatter.

- [ ] **Page type templates**: Themes provide a generic `default.html.tmpl` (default) plus optional
      type-specific layouts: `page.html.tmpl`, `api.html.tmpl`, `changelog.html.tmpl`, etc.
- [ ] **Frontmatter `layout` field**: Content files select their layout via `layout: api` in frontmatter.
      Falls back to `default.html.tmpl` if the specified type doesn't exist in the theme.
- [ ] **Layout inheritance**: Type-specific layouts can extend the base layout, overriding only the
      content block while inheriting header, footer, and scripts.

### 8e: Static Asset Serving

Serve theme-specific and shared static files (JS, CSS, images, fonts) via a dedicated `/_assets/` route,
using the overlay filesystem to allow themes to override shared resources.

- [ ] **Shared static directory**: Introduce `assets/shared/static/` in the embedded FS for cross-theme
      resources (web components, shared JS libraries, common icons). Served at `/_assets/shared/`.
- [ ] **Theme static directory**: Each theme's `static/` folder is served at `/_assets/theme/`. Contains
      theme-specific CSS, JS, images, and fonts that are too large or inappropriate to inline.
- [ ] **Overlay resolution for statics**: Build the static file FS as an overlay stack:
      site `.gomddoc/static/` > theme `static/` > `shared/static/`. This lets themes override shared
      resources (e.g. a custom web component variant) and sites override theme resources without forking.
      Reuses the existing `OverlayFS` implementation.
- [ ] **`/_assets/` HTTP handler**: Register a handler that serves files from the static overlay FS
      with proper MIME types, ETags, and cache headers. Block directory listings and dotfiles.
- [ ] **Template `assetURL` function**: Provide `{{ assetURL "color-chip.js" }}` in templates that
      resolves to `/_assets/theme/color-chip.js` (or `/_assets/shared/...` for shared assets). Replaces
      the current `{{ asset }}` inline approach with external `<script src>` / `<link href>` references.
- [ ] **Migrate inline JS**: Move the current `{{ asset "color-chip.js" }}` inline pattern to
      `<script type="module" src="{{ assetURL "color-chip.js" }}"></script>` once static serving is live.
      Keep `{{ asset }}` available as a fallback for small snippets where inlining is preferred.
- [ ] **Build mode**: `gomddoc build` copies the resolved static overlay into the output `_assets/`
      directory. Shared and theme statics are merged with the same override precedence.

## Phase 9: SEO and Discoverability

_Technical SEO features to compete with MkDocs Material, Docusaurus, and Hugo for search rankings.
gomddoc's server-rendered HTML with no client-side framework is a natural Core Web Vitals advantage —
these items close the gap on crawl management, structured data, and social sharing.
See `docs/seo-competitive-analysis.md` for full competitive analysis._

### 9a: Pre-Launch SEO (P0)

_Must ship before public launch. These are table-stakes features that every documentation tool provides._

- [ ] **Canonical URLs**: Add `<link rel="canonical" href="...">` to every page's `<head>`.
      Constructed from `domain` config + page path. Prevents duplicate content penalties when
      content is accessible via multiple URLs (trailing slash, `README.md` vs `index.html`).
      Expose `canonicalURL` in `TemplateContext` or as a template function. Low complexity.
- [ ] **XML sitemap**: _Already planned in Phase 7._ Additionally, serve `/sitemap.xml` dynamically
      in `serve` mode (not just `build`). Include `<lastmod>` from Git commit dates (see Git-based
      timestamps below).
- [ ] **`robots.txt`**: Serve `/robots.txt` with `Sitemap:` directive and sensible defaults
      (allow all, block `/_assets/`, `/api/`, `/.well-known/`). In `build` mode, write file to
      output directory. In `serve` mode, serve from a dedicated handler. Low complexity.
- [ ] **Open Graph meta tags**: Add `og:title`, `og:description`, `og:url`, `og:type` ("article"),
      `og:site_name` to every page. Add corresponding `twitter:card` (summary) tags. All data
      already available from frontmatter and site config. Optional `og:image` (see P1 social
      images). Add to theme `head.html.tmpl` partials. Low complexity.

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

**Immediate focus (Phase 8):** Theme folder restructuring (partials, page types, static assets) and theme variables.
**Pre-launch (Phase 9a):** Canonical URLs, robots.txt, Open Graph — table-stakes SEO before public release.
**Next up (Phase 4 & 5):** Profiling/benchmarks for data-driven optimization, full-text search for content discovery.
**Then (Phase 9b):** Post-launch SEO (JSON-LD, Git timestamps, social images).
**High-value (Phase 10a):** MCP interface — low complexity (thin adapter over existing layers), high differentiation.

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
