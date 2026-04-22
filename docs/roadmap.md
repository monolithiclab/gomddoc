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
- **HTTP**: Content negotiation (output-type only — input→output negotiation planned for Phase 6a),
  gzip compression, security headers, ETag, request ID tracking, graceful shutdown.
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

### 6a: Two-Dimensional Content Negotiation

Rework the renderer registry so renderers declare both **input** and **output** MIME types. The handler
uses the file's MIME type and the client's `Accept` header to select the best renderer.

- [ ] **Renderer interface change**: Replace `SupportedMimeTypes() []string` with two methods:
      `InputMimeTypes() []string` (what the renderer can read) and `OutputMimeTypes() []string`
      (what it can produce). Each renderer is a single input→output transformation.
- [ ] **Registry two-dimensional lookup**: `Get(inputMimeType string, acceptedTypes []MediaType)` replaces
      `Get(mimeType string)`. Selection algorithm: 1. Filter renderers whose `InputMimeTypes()` match the file's MIME type. 2. For each accepted type (sorted by q-value from `ParseAccept`), rank matching renderers by output
      specificity: exact match (e.g. `text/html`) > type wildcard (`text/*`) > catch-all (`*/*`). 3. Pick the highest-ranked renderer. On tie, latest registered wins (allows overrides).
- [ ] **MarkdownRenderer**: input `["text/markdown"]`, output `["text/html"]`. Transforms markdown to HTML
      via goldmark with post-processing pipeline (heading anchors, admonitions, color chips).
- [ ] **MarkdownPassthroughRenderer**: input `["text/markdown"]`, output `["text/markdown"]`. Returns
      markdown content as-is (or with enrichment, see 6b). Serves LLMs and API consumers that prefer
      raw markdown over rendered HTML.
- [ ] **PassthroughRenderer**: input `["*/*"]`, output `["*/*"]`. Catch-all fallback, lowest priority.
      Returns content unchanged with the provider's detected MIME type.
- [ ] **Handler update**: Move content negotiation _before_ rendering. Today the handler renders first,
      then checks `Accept` (wasting CPU on 406 responses). New flow: select renderer based on input type +
      Accept header, then render. The `RenderResult` drops `Metadata` and `TOC` fields (moved to enricher).
- [ ] **406 Not Acceptable**: When no renderer matches the input type + Accept combination, return 406
      with a list of available output types in the response body.

### 6b: Content Enricher Pipeline

Introduce an `Enricher` step that runs before rendering. The enricher extracts structured data from
content (metadata, TOC, navigation, related documents) independent of the output format. Renderers
receive enrichment data and decide what to use.

- [ ] **Enricher interface**:

  ```go
  type EnrichmentData struct {
      Metadata    map[string]any
      TOC         *TOCNode
      Navigation  *NavTree       // format-agnostic tree; renderers decide presentation
      RelatedDocs []RelatedDoc
  }

  type NavTree struct {
      Items []NavItem
  }

  type NavItem struct {
      Title    string
      Path     string
      Active   bool      // on the current page's path
      Children []NavItem
  }

  type Enricher interface {
      SupportedMimeTypes() []string
      Enrich(ctx context.Context, content []byte, path string) (*EnrichmentData, error)
  }
  ```

- [ ] **MarkdownEnricher**: Extracts YAML frontmatter, builds TOC from headings, resolves navigation
      context from the file path, and finds related documents via shared tags (using the metadata index).
- [ ] **Render signature change**: `Render(ctx, content, enrichment)` — renderers receive enrichment data.
      The HTML renderer uses TOC/metadata for template context. The passthrough renderer may inject
      related doc links or return content unchanged. Renderers are not required to use enrichment.
- [ ] **Handler pipeline**: `Provider.ReadFile() → Enricher.Enrich() → Renderer.Render(content, enrichment)
→ Handler (template wrap if HTML, raw otherwise)`. The enricher needs access to the metadata index
      and navigation builder (dependency injection via constructor, not stateless like renderers).
- [ ] **Enricher registry**: MIME-type-keyed registry similar to the renderer registry. Falls back to a
      no-op enricher (returns empty `EnrichmentData`) for types without a dedicated enricher.

### 6c: Additional Renderers

_Expand format support for technical documentation._

- [ ] **AsciiDoc support**: Renderer for `.adoc` files (popular in technical writing).
- [ ] **OpenAPI renderer**: Render `swagger.yaml` / `openapi.json` as interactive API docs.
- [x] **KaTeX math rendering**: Client-side via CDN with `$...$` (inline) and `$$...$$` (display) delimiters.
      Auto-render extension scans page content on load.
- [x] **Mermaid diagrams**: Client-side via CDN with fenced `mermaid` code blocks. Theme-aware (dark/light).
- [ ] **Server-side Mermaid/KaTeX**: Native rendering without client-side JS dependency.

## Phase 7: Static Site Generation (Partial)

- [x] **`gomddoc build` command**: Walk content tree, render markdown through template pipeline, output
      static HTML. Generates `index.html` alongside `README.html` for clean URLs. Copies non-markdown files as-is.
- [ ] **Sitemap generation**: Generate `sitemap.xml` during build with `<url>` entries for all rendered
      pages. Include `<lastmod>` from Git commit dates (filesystem provider falls back to file mtime).
      Respects `base_url` from site config for absolute URLs. Excludes hidden files and non-HTML outputs.
- [ ] **Asset optimization**: Minify HTML/CSS/JS during build.
- [ ] **Static host compatibility**: Output structure compatible with S3, Netlify, Cloudflare Pages.

## Phase 7b: `gomddoc preview` — Quick Local Preview

_A zero-friction subcommand for previewing documentation locally, distinct from the production-grade `serve`._

- [ ] **`gomddoc preview [dir]`**: Serve a directory (defaults to `.`) with sensible defaults optimized for
      local authoring. Differences from `serve`:
  - **Auto port**: Find an available port starting from 8080. No "address already in use" errors.
  - **Auto open**: Launch the default browser and navigate to the local URL on startup (`--no-open` to disable).
  - **Dev mode on**: Implies `--dev` (no caching, hot reload) without requiring the flag.
  - **Directory index**: Enable `dir_index: true` by default so folders without a README show a file listing.
  - **Minimal output**: Print only the URL and "Press Ctrl+C to stop" — no verbose startup logs.
- [ ] **Port discovery**: Use `net.Listen(":0")` or scan from 8080 upward to find a free port.
      Store the chosen port in a predictable location (e.g. `.gomddoc/.preview-port`) so tooling can find it.
- [ ] **Browser open**: Use `open` (macOS), `xdg-open` (Linux), or `start` (Windows) to launch the browser.
      Respect `$BROWSER` env var if set.

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

## Phase 9: Enterprise Features

- [ ] **S3 provider**: Serve content directly from S3 buckets (for non-git use cases).
- [ ] **Authentication**: OIDC/OAuth middleware for private documentation.
- [ ] **Branch switching**: UI dropdown to switch between Git branches/tags.

## Phase 10: Git Workflow Integration

- [ ] **Webhooks**: Endpoint to trigger `git fetch` on push events (cache invalidation).
- [ ] **PR preview**: Serve content from PR branches for review.

## Phase 11: Theme Marketplace

_Enable community theme sharing via a GitHub-based registry._

### 11a: Theme Registry

- [ ] **Default marketplace repository**: A GitHub repository (e.g. `gomddoc/themes`) acts as the theme
      registry. Contains an `index.json` manifest listing available themes with name, description, author,
      version, repository URL, and preview image URLs.
- [ ] **Theme package format**: Each theme is a Git repository containing `default.html.tmpl`, optional
      partials, a `README.md` with frontmatter metadata (name, category, fonts, colors), and screenshot
      previews (`light.png`, `dark.png`).
- [ ] **Custom registries**: Users can configure alternative marketplace URLs in `.gomddoc/config.yml`
      via `theme_registry: https://github.com/org/custom-themes` for private or corporate theme registries.

### 11b: `gomddoc theme` Subcommand

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
| `gomddoc preview`       | Quick local preview with auto-open browser           | Planned |
| `gomddoc init`          | Scaffold a `.gomddoc/` directory with default config | Done    |
| `gomddoc validate`      | Validate config and check for broken links           | Planned |
| `gomddoc theme list`    | List available themes from the marketplace           | Planned |
| `gomddoc theme search`  | Search themes by name, category, or keyword          | Planned |
| `gomddoc theme info`    | Show detailed theme information                      | Planned |
| `gomddoc theme install` | Download and install a theme                         | Planned |
| `gomddoc theme update`  | Update installed theme(s) to latest version          | Planned |

## Implementation Strategy

Development proceeds in phases building on stable foundations. Each phase delivers complete, tested functionality.

**Immediate focus (Phase 6a/6b):** Two-dimensional content negotiation and enricher pipeline — foundational
changes that unlock LLM-friendly API access, cleaner separation of concerns, and richer cross-document features.
**Next up (Phase 8):** Theme folder restructuring (partials, page types, static assets) and theme variables.
**Then (Phase 5 & 6c):** Full-text search for content discovery, and expanding renderer support (AsciiDoc, OpenAPI).

## Deferred (Not Planned)

These features were considered but deprioritized to keep gomddoc focused:

- Database providers (PostgreSQL/SQLite) — Git is the database
- REST/GraphQL APIs — gomddoc is a viewer, not a headless CMS
- Editorial workflows (drafts, reviews, scheduling) — use Git branches
- Content versioning/revision history — use Git history
- i18n/multi-language — out of scope for now
- VS Code extension
- Plugin architecture / dynamic renderer loading
