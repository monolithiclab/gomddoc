# PLAN

Evolve gomddoc into the best git-native documentation viewer and static site generator. No databases, no CMS workflows — Git is the source of truth.

# Architecture Overview

## Core Principles

- **Git-native**: Stateless, read-only viewer — Git is the database
- **Interface-driven design**: All major components implement well-defined interfaces
- **Performance-first**: Built for speed with intelligent caching and disk-based storage
- **Security by design**: Path traversal protection, content sanitization, secure defaults
- **Developer experience**: Hot reload, comprehensive CLI, debugging tools
- **Zero-config useful, fully-config powerful**: Works out of the box, deeply configurable when needed

## Current Architecture

```
HTTP Request
    │
    ▼
Middleware (Security Headers → Method Filter → Hidden Path Block)
    │
    ▼
Handler (ServeContent)
    │
    ├── Provider (Filesystem | Git) → ReadFile + MIME detection
    ├── Registry (MIME → Renderer lookup with wildcards)
    ├── Renderer (Markdown → HTML | Passthrough)
    └── Template (Layout + Breadcrumbs + TOC)
    │
    ▼
HTTP Response
```

## What's Already Built

- Filesystem and Git providers (SSH auth, known_hosts, fail-closed)
- Markdown rendering (goldmark, GFM, frontmatter, TOC generation)
- MIME-type based renderer registry with wildcard matching
- Content negotiation (Accept header, q-values)
- Template system with overlay FS, caching, breadcrumbs
- Configuration (CLI flags, env vars, YAML config, reflection-based)
- Security (path traversal, hidden files, method filtering, headers, clone timeout, file size limits)
- Directory listing with secure defaults
- Comprehensive test suite (~78% coverage)

---

# Feature Specifications

## 1. Performance & Scaling

### 1.1 Disk-Based Git Storage

*Current Git provider is in-memory only, limiting repo size.*

- Implement `filesystem.Storage` backend for go-git to support large monorepos
- Configurable storage location with automatic cleanup
- Lazy blob loading — only fetch file content on request
- Benchmark memory usage vs in-memory for repos of varying size

### 1.2 HTTP Caching

- `ETag` headers derived from Git commit hashes (content-addressable)
- `If-Modified-Since` / `If-None-Match` support → 304 responses
- `Cache-Control` with configurable TTLs per content type
- Stale-while-revalidate for background git fetch

### 1.3 Response Compression

- Gzip/Brotli compression for text responses (HTML, CSS, JS, Markdown)
- Pre-compressed asset serving when available
- Minimum size threshold to avoid compressing tiny responses

### 1.4 Partial Clones

- `git clone --filter=blob:none` for sparse checkout of large repos
- Tree-only initial clone, fetch blobs on demand
- Depends on upstream go-git support — track and adopt when available

## 2. Search and Discovery

### 2.1 Full-Text Search

**Interface Definition:**

```go
type SearchEngine interface {
    Index(ctx context.Context, documents []Document) error
    Search(ctx context.Context, query string, opts SearchOpts) (*SearchResult, error)
    Suggest(ctx context.Context, prefix string) ([]string, error)
}
```

**Two-track strategy:**

- **Server-side**: Bleve integration for dynamic serving mode
- **Client-side**: Pagefind or Lunr.js index generation for static builds

**Features:**
- Incremental indexing on git fetch
- Frontmatter-aware filtering (`tag:golang status:published`)
- Fuzzy matching and typo tolerance
- Search result highlighting with context snippets

### 2.2 Auto-Generated Navigation

- Sidebar navigation tree from directory structure
- Configurable via `_nav.yaml` or `_sidebar.md` override files
- Collapsible sections with expand/collapse state persistence
- Active page highlighting and scroll-into-view
- Ordering via frontmatter `weight` field or filename prefix (`01-intro.md`)

### 2.3 Frontmatter Indexing

- Parse and index all frontmatter fields at startup / git fetch
- Tag and category listing pages (auto-generated taxonomy)
- Filter and sort content by any metadata field
- Expose metadata via internal template functions

## 3. Renderer Enhancements

### 3.1 Rich Markdown Extensions

- **Mermaid diagrams**: Server-side rendering via mermaid-go or client-side JS
- **Math**: KaTeX rendering for `$inline$` and `$$block$$` expressions
- **Syntax highlighting themes**: Multiple Chroma themes, configurable per-site
- **Admonitions**: `> [!NOTE]`, `> [!WARNING]`, `> [!TIP]` callout blocks (GitHub-style)
- **Content tabs**: Tabbed content blocks for multi-language examples
- **Task lists**: Interactive checkboxes (read-only display)

### 3.2 AsciiDoc Renderer

- `.adoc` file support via asciidoctor or native Go parser
- Cross-references and include directives
- Table of contents extraction matching Markdown renderer interface

### 3.3 OpenAPI Renderer

- Render `openapi.yaml` / `openapi.json` / `swagger.yaml` as interactive API docs
- Swagger UI or Redoc-style presentation
- Try-it-out panels for API exploration
- Schema visualization with expandable models

### 3.4 Jupyter Notebook Renderer

- Parse `.ipynb` and render cells (code + output + markdown)
- Syntax-highlighted code cells
- Inline image and plot rendering
- Math rendering in markdown cells

## 4. Static Site Generation

### 4.1 Build Command

```bash
gomddoc build [--output dist/] [--base-url https://example.com]
```

- Crawl content tree and render all pages to static HTML/CSS/JS
- Incremental builds — only re-render changed files (based on git diff)
- Parallel rendering with configurable worker count
- Build manifest with integrity checksums
- Exit code reflects build success/failure for CI integration

### 4.2 Asset Pipeline

- CSS/JS minification during build
- Asset fingerprinting for cache busting (`style.a1b2c3.css`)
- Critical CSS extraction and inlining for above-the-fold content
- Image optimization (WebP conversion, responsive srcset generation)
- Unused CSS elimination via PurgeCSS-style analysis

### 4.3 Output Targets

- Filesystem output (default) — compatible with any static host
- S3/GCS direct upload with proper Content-Type and Cache-Control
- Netlify/Vercel/GitHub Pages compatibility (redirects, headers files)
- Configurable base path for subdirectory hosting (`/docs/`)

### 4.4 SEO and Feeds

- XML sitemap generation with `lastmod` from git timestamps
- RSS/Atom feed generation from content with `published_at` frontmatter
- `robots.txt` generation with configurable rules
- Canonical URL enforcement
- Open Graph and Twitter Card meta tags from frontmatter
- JSON-LD structured data (Article, TechArticle, HowTo schemas)

## 5. Versioned Documentation

### 5.1 Multi-Version Serving

- Serve documentation from multiple Git branches or tags simultaneously
- URL structure: `/v2.1/getting-started` or `/latest/getting-started`
- Version switcher dropdown in the UI
- "Latest" alias that tracks a configurable branch/tag pattern
- Per-version search index

### 5.2 Version Configuration

```yaml
versions:
  - label: "v3.0 (latest)"
    ref: "main"
    alias: "latest"
  - label: "v2.x"
    ref: "v2"
  - label: "v1.x (legacy)"
    ref: "v1"
    banner: "This version is no longer maintained."
```

### 5.3 Cross-Version Features

- Diff view between versions of the same document
- "This page was updated in v3.0" banners with links
- Version-aware search (search within current version or all)

## 6. Multi-Repository Aggregation

### 6.1 Unified Documentation Site

- Serve content from multiple Git repositories as a single site
- Mount points: repo A at `/api/`, repo B at `/guides/`
- Independent refresh cycles per repository
- Unified navigation and search across all repos

### 6.2 Configuration

```yaml
sources:
  - url: "git@github.com:org/api-docs.git"
    mount: "/api"
    ref: "main"
  - url: "git@github.com:org/user-guides.git"
    mount: "/guides"
    ref: "main"
  - path: "./local-docs"
    mount: "/internal"
```

## 7. Theming and UI

### 7.1 Theme System

- Theme inheritance: Base → Custom (override specific partials without copying everything)
- Multiple built-in themes: documentation, blog, knowledge base
- Theme configuration via `theme.yaml` (colors, fonts, layout options)
- Dark/light mode toggle with system preference detection and persistence
- Print-friendly CSS media queries

### 7.2 UI Enhancements

- "Copy to clipboard" buttons on code blocks
- Anchor links on headings (hover-to-reveal)
- Keyboard navigation (previous/next page, search hotkey)
- Reading progress indicator
- "Edit this page on GitHub/GitLab" links (auto-generated from Git remote)
- "Last updated" timestamps from Git log
- Mobile-responsive layout with hamburger menu navigation

### 7.3 Code Block Features

- Line numbers (optional)
- Line highlighting (`go {hl_lines=[3,5-7]}`)
- Diff syntax highlighting (added/removed lines)
- Filename/title display above code blocks
- Collapsible long code blocks

## 8. Git-Workflow Integration

### 8.1 Branch Switching

- UI dropdown to switch between Git branches/tags
- Preview feature branches before merge
- Branch protection — only serve specified branches publicly

### 8.2 Webhook Receiver

- `POST /api/webhook` endpoint to trigger `git fetch` on push events
- HMAC signature verification (GitHub, GitLab, Bitbucket formats)
- Debounced fetch to avoid thundering herd on rapid pushes
- Cache invalidation after fetch

### 8.3 Git Metadata Integration

- `git log` integration for "last edited by" and "last edited at" per file
- Contributors list per page from git history
- Changelog generation from git commits affecting documentation

## 9. Observability

### 9.1 Prometheus Metrics

- Request latency histograms by path pattern and status code
- Cache hit/miss ratios (template cache, git object cache)
- Git fetch duration and frequency
- Active connections and request queue depth
- Content rendering duration by renderer type

### 9.2 Health Endpoints

- `GET /health/live` — process is alive
- `GET /health/ready` — git clone complete, ready to serve
- Structured health response with dependency status (git remote reachable, disk space)

### 9.3 Structured Logging

- JSON-formatted logs for production (already using `slog`)
- Request ID propagation through context
- Slow request logging with configurable threshold

## 10. Enterprise and Deployment

### 10.1 Authentication Middleware

- OIDC/OAuth2 middleware for "internal docs" use cases
- Bearer token validation for API access
- IP allowlist for restricted access
- Pluggable auth via middleware interface

### 10.2 Container-First Deployment

- Multi-stage Dockerfile with minimal final image (distroless/scratch)
- Helm chart for Kubernetes deployment
- Health probes, resource limits, HPA configuration
- Sidecar pattern for git-sync (alternative to built-in git provider)

## 11. CLI Enhancements

### 11.1 New Commands

```bash
gomddoc init                        # Scaffold new documentation project
gomddoc build [--watch]             # Static site generation
gomddoc serve [--port] [--host]     # Development server (existing)
gomddoc validate                    # Check for broken links, missing metadata
gomddoc search-index                # Pre-build search index
gomddoc export --format pdf         # Export to PDF (ICEBOX)
```

### 11.2 Link Validation

- Crawl all internal links and verify targets exist
- Optional external link checking with configurable timeout
- Report broken links with source location
- CI-friendly exit codes (non-zero on broken links)

### 11.3 Shell Completions

- Bash, Zsh, Fish, PowerShell completion scripts
- `gomddoc completion bash > /etc/bash_completion.d/gomddoc`

## 12. Plugin Architecture

### 13.1 Renderer Plugins

- WASM-based renderer plugins for sandboxed execution
- Plugin discovery from configured directories
- Hot-reload plugins in development mode
- Plugin manifest with version, author, supported MIME types

### 13.2 Middleware Plugins

- Custom middleware injection points (pre-handler, post-render)
- Authentication, rate limiting, analytics as plugins
- Configuration via site config YAML

---

# Implementation Priorities

## Phase 4: Performance & Scaling
1. Disk-based Git storage (large repo support)
2. HTTP caching with ETag/If-None-Match
3. Response compression (gzip/brotli)

## Phase 5: Search & Navigation
1. Full-text search (Bleve for server, Pagefind for static)
2. Auto-generated sidebar navigation
3. Frontmatter indexing and taxonomy pages

## Phase 6: Renderer Enhancement
1. Admonitions and content tabs
2. Mermaid diagram rendering
3. KaTeX math rendering
4. OpenAPI interactive docs

## Phase 7: Static Site Generation
1. `gomddoc build` command
2. Asset pipeline (minification, fingerprinting)
3. Sitemap, RSS/Atom, robots.txt
4. Output target compatibility (S3, Netlify, GitHub Pages)

## Phase 8: Versioned Documentation
1. Multi-branch/tag serving
2. Version switcher UI
3. Cross-version diffing

## Phase 9: Theming & UI Polish
1. Dark/light mode
2. Theme inheritance and multiple built-in themes
3. Code block enhancements (copy, line numbers, highlights)
4. "Edit on GitHub" links, git timestamps

## Phase 10: Git Workflow & Multi-Repo
1. Webhook receiver for cache invalidation
2. Branch switching UI
3. Multi-repository aggregation
4. Git metadata (contributors, last edited)

## Phase 11: Enterprise & Deployment
1. OIDC/OAuth authentication middleware
2. Container-optimized deployment (Dockerfile, Helm)
3. Prometheus metrics and health endpoints

## Phase 12: Advanced Features
1. Link validation CLI
2. Plugin architecture (WASM renderers)
3. Shell completions

---

# Icebox (DO NOT IMPLEMENT)

The following features are parked. They may be revisited in the future but are explicitly excluded from current and near-term development.

- **Content Includes** — `{{< include "path/to/file.md" >}}` directive with line ranges, recursive resolution, and cycle detection. Adds significant complexity to the rendering pipeline.
- **PDF Export** — Generate PDF from pages or entire doc tree via headless Chrome or Go PDF libraries. Heavy dependency, niche use case.
- **S3 Content Provider** — Read content from S3/GCS/MinIO buckets. Contradicts the git-native focus; use the static build output + S3 hosting instead.

---

# Deferred / Out of Scope

- Database providers (PostgreSQL, SQLite) — Git is the database
- Editorial workflows, draft management, content scheduling — use Git branches
- GraphQL API, REST CRUD API — this is a viewer, not a CMS
- User management, RBAC, multi-tenant — beyond scope of a documentation tool
- Comment systems, email integrations — use external services
- VS Code extension — low ROI vs improving the core tool
- i18n / multi-language — complex, low demand for technical docs
- A/B testing, visual regression testing — enterprise SaaS concerns
