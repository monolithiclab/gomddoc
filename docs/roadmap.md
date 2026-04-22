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

The foundation is production-ready with 87.3% test coverage. For a full description
of current capabilities, see `docs/architecture.md`.

**Completed phases:** 1-3 (core), 4 (partial), 5 (partial), 6 (renderer enhancement),
7 (partial — sitemap generation complete), 7b (preview), 8 (theming engine), 9a (pre-launch SEO),
10a-10b (MCP server + HTTP). Six review batches resolved 45/46 identified issues (security,
correctness, deduplication, hardening).

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
- [x] **Search UI**: Client-side search modal shared across all 8 themes via `inlineJSAsset "search.mjs"`.
      Ctrl+K / Cmd+K shortcut, debounced API fetch, arrow key navigation, highlighted snippets.
      CSS uses theme custom properties for automatic cross-theme and dark mode compatibility.
- [ ] **Full-text search (build)**: Option B (Pagefind) as optional post-build step.

## Phase 7: Static Site Generation

- [x] **Sitemap generation**: Generate `sitemap.xml` during build with `<url>` entries for all rendered
      pages. Include `<lastmod>` from file mtime (Git provider returns commit timestamps).
      Respects `base_url` from site config for absolute URLs. Excludes hidden files and non-HTML outputs.
      Implemented in `cmd/gomddoc/build.go` via `server.GenerateSitemap()`.
- [ ] **Asset optimization**: Minify HTML/CSS/JS during build.
- [ ] **Static host compatibility**: Output structure compatible with S3, Netlify, Cloudflare Pages.

## CLI and Developer Experience

_Usability improvements to the serve, build, and preview subcommands._

- [x] **Guard build output directory**: `gomddoc build` fails if the output directory already
      exists, preventing stale content accumulation. Use `--force` / `-f` to remove the existing
      output and rebuild cleanly. Low complexity.
- [x] **Remove `--dev` from `serve`**: `serve` is always production mode. `preview` is the
      designated dev command (no caching, template re-parsing, verbose logging).
- [ ] **Autoreload in `preview`**: Automatically reload the browser when content files change.
      Inject a small script into rendered pages that connects via Server-Sent Events (SSE) to
      a `/_preview/events` endpoint. The server watches the content directory with `fsnotify`
      and pushes reload events on file changes. SSE is simpler than WebSocket (no upgrade
      handshake, native `EventSource` API, works through proxies). Only active in `preview`
      mode — `serve` never injects the script. Medium complexity.
- [x] **`--domain` flag for serve/build/preview**: `-d, --domain` / `GOMDDOC_DOMAIN` on all
      three subcommands overrides `meta.domain` from config. CLI flag takes precedence.
- [x] **URL extension stripping**: `strip_extensions` config option (default: `[".md"]`) removes
      specified extensions from URLs. Requests to `/docs/guide.md` redirect (301) to `/docs/guide`.
      In build mode, generates directory-based URLs (`guide/index.html`) for static host compatibility.
      Implemented in `internal/resolve` with collision detection (first configured extension wins).
      Low-medium complexity.
- [ ] Add an ignore list (matching gitignore rules) to exclude files/folder/globs/...

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

- [x] **JSON-LD structured data**: `<script type="application/ld+json">` with Schema.org markup.
      Per-page `TechArticle` (headline, description, author, datePublished from frontmatter).
      `BreadcrumbList` matching visible breadcrumbs. `WebSite` with `SearchAction` on index page.
      Generated by `internal/seo.GenerateJSONLD()`, exposed via `jsonLD` template function and
      overridable `jsonld` partial. All 8 themes updated.
- [ ] **Git-based timestamps**: Expose `datePublished` (first commit date) and `dateModified`
      (last commit date) per page from Git provider. Filesystem provider falls back to mtime.
      Used in JSON-LD, sitemap `<lastmod>`, and optionally displayed in UI ("Last updated on...").
      Cache at startup or lazily to avoid per-request Git operations. Medium complexity.
- [x] **Sitemap `<lastmod>` timestamps**: `<lastmod>` on each sitemap entry from `fs.Stat()`
      file mtime. Git provider returns commit timestamps. Omitted gracefully if stat fails.

### 9c: SEO Polish (P2)

_Enhances competitiveness and closes remaining gaps._

- [x] **Per-page `robots` meta tag**: Frontmatter `robots: noindex` or `robots: noindex, nofollow`
      controls per-page indexing. Site-wide default via `meta.robots` config. Pages with `noindex`
      are automatically excluded from the sitemap. Low complexity.
- [x] **HTML `lang` attribute**: `<html lang="...">` configurable via `language` field in
      `SiteConfig` (default `"en"`). Per-page override via frontmatter `lang` field.
      All 8 themes + marketing site updated. Low complexity.
- [x] **Static error pages**: Styled standalone error pages (404, 403, 500) with inline CSS and
      dark mode support. `build` outputs `404.html` for static host compatibility (Netlify, GitHub
      Pages, Cloudflare Pages). `serve` uses the same pages for error responses. Low complexity.
- [ ] **Heading anchor slug stability**: Document and test the slug algorithm for long-term URL
      stability. Ensure GitHub-compatible slugs. Support frontmatter `id` override per heading.
      Low complexity.

### 9d: SEO Long-Term (P3)

- [x] **Redirect support**: Frontmatter `redirect_from: [/old-url]` enables reverse redirects.
      In `serve` mode, 301 responses via redirect map built at startup. In `build` mode,
      generates HTML files with `<meta http-equiv="refresh">` for universal static host
      compatibility. Redirect map is zero-cost per request.
- [x] **Atom feed**: Generate `/feed.xml` (Atom format) listing the 20 most recently modified
      pages. Served dynamically in `serve` mode and generated during `build`. Requires
      `meta.domain`. Excludes `robots: noindex` pages. Autodiscovery `<link>` in all themes.
- [ ] **Preconnect/preload resource hints**: Add `<link rel="preconnect">` for external domains
      (Google Fonts, KaTeX/Mermaid CDNs) and `<link rel="preload">` for critical resources in
      theme `<head>`. Improves LCP. Low complexity.
- [ ] **Image dimension attributes**: Post-process rendered HTML to add `width`/`height` to
      `<img>` tags that lack them. Prevents CLS (Core Web Vitals). Reads dimensions from content
      provider. Medium complexity.
- [x] **`<link rel="next/prev">`**: Sequential page links derived from navigation order. Minor
      crawl efficiency signal. Low complexity.

## Tag Components

_Make tags a first-class navigation and discovery mechanism. Tags already exist in frontmatter and
the metadata index — these items surface them in the UI and search engine._

- [ ] **Clickable tag chips in page rendering**: Render frontmatter `tags` as clickable chips
      (styled inline elements) on each page. Each chip links to a tag listing page. Position
      configurable via theme template (typically below the page title or in a sidebar metadata
      section). All 8 built-in themes must include the tag chips. Low-medium complexity.
- [ ] **Tag listing page (`/tags/{tag}`)**: Server-rendered HTML page listing all pages tagged
      with a given tag. Reuses the existing `MetaIndex.ByTag()` lookup. Each result shows title,
      description, and path as a clickable link. Shares the site's theme and navigation chrome.
      In `build` mode, generate a static HTML page per tag under `tags/`. Medium complexity.
- [ ] **Tag index page (`/tags/`)**: Overview page listing all tags with document counts. Each
      tag links to its listing page. Serves as a discovery entry point. Generated statically in
      `build` mode. Low complexity.
- [ ] **Search by tag (`tag:` prefix)**: Extend the search engine to support `tag:XXX` syntax.
      When a query term starts with `tag:`, match it against the metadata index tags instead of
      the full-text index. Can be combined with free-text terms (e.g., `tag:deployment kubernetes`
      searches for "kubernetes" in pages tagged "deployment"). Update the search modal UI to
      display tag suggestions. Medium complexity.
- [ ] **Related pages via tags**: Display a "Related pages" section at the bottom of each page,
      populated from shared frontmatter tags. The enricher already computes `RelatedDocs` via
      `ByTag()` — expose via template and render in all themes. Medium complexity (enricher done,
      needs template integration). _Moved from Phase 9c._

## Page Navigation (Next/Previous)

- [ ] **Next/previous links at page bottom**: Display "Previous" and "Next" links at the bottom
      of each page for sequential reading. By default, derive order from the navigation tree
      (depth-first traversal matches sidebar order). Allow frontmatter overrides via `prev` and
      `next` fields pointing to relative paths (e.g., `next: 02-configuration.md`). A frontmatter
      value of `false` suppresses the link for that direction. Render in all 8 built-in themes as
      a two-column footer with page titles. Works in both `serve` and `build` modes. Medium
      complexity (navigation flattening + template integration).

## Phase 10: MCP Interface — AI-Native Documentation Access

_Expose documentation content via the [Model Context Protocol](https://modelcontextprotocol.io) so AI
models (Claude, GPT, Copilot, etc.) can directly read, search, and navigate documentation. This turns
gomddoc into an AI-queryable knowledge base — a strategic differentiator no other documentation tool
offers natively._

**Why MCP over HTTP APIs:** MCP is purpose-built for AI tool use. Models discover capabilities via
the protocol, not by reading API docs. A single `gomddoc mcp` command makes every document instantly
available to any MCP-compatible client (Claude Desktop, Cursor, Windsurf, custom agents) with zero
configuration on the model side.

**Architecture fit:** The MCP server is a thin adapter over existing gomddoc internals. No new parsing
logic — it reuses Provider (file I/O), Metadata Index (tag queries), Search Index (full-text search),
and Navigation (TOC tree) directly. Package: `internal/mcp/`.

```
MCP Client (Claude, etc.)
    ↕ JSON-RPC (stdio or Streamable HTTP)
internal/mcp/server.go
    ↓ adapts to existing interfaces
Provider → Metadata Index → Search Index → Navigation
```

### 10a: Core MCP Server (Complete)

_Full MCP server with tools, resources, prompts, and section-level access._

- [x] **`internal/mcp/` package**: MCP server using the official Go MCP SDK
      (`github.com/modelcontextprotocol/go-sdk` v1.4.x). Thin adapter over Provider,
      MetaIndex, SearchIndex, and Navigation. Supports stdio and Streamable HTTP transports.
- [x] **`gomddoc mcp` subcommand**: stdio transport for local use. Speaks MCP JSON-RPC over
      stdin/stdout. Works with Claude Desktop, Cursor, Claude Code, and any MCP-compatible
      client. Kong subcommand with same positional `dir` argument as `serve`.
- [x] **6 tools**: `search_docs` (full-text search with TF-IDF ranking), `read_page` (page
      content with metadata), `list_pages` (browse with tag filter), `get_table_of_contents`
      (navigation tree), `read_section` (sub-page access by heading ID), `find_related`
      (related pages via shared tags). All annotated `readOnlyHint: true`.
- [x] **4 resources**: `docs://site/index` (all pages JSON), `docs://site/tags` (all tags),
      `docs://site/page/{+path}` (page content), `docs://site/tag/{tag}` (pages by tag).
      Custom `docs://` URI scheme for semantic identification.
- [x] **3 prompts**: `explain_concept`, `troubleshoot`, `summarize_page`. Pre-built
      interaction patterns that search docs and embed relevant context.
- [x] **Section extraction**: Line-based heading parser (`ExtractSection`, `slugifyHeading`)
      extracts content under a specific heading ID without goldmark dependency. Key
      differentiator — sub-page access for token-efficient retrieval.

### 10b: HTTP Integration (Complete)

_Streamable HTTP transport for remote MCP access._

- [x] **Streamable HTTP at `/_mcp/`**: Mount `MCPServer.HTTPHandler()` on the existing HTTP
      server behind the auth RouteGroup. `MCPHandler http.Handler` added to `HTTPServerConfig`.
      Protected by the same authentication middleware as other endpoints. MCP server is created
      in `setupServer` and shared between stdio (`gomddoc mcp`) and HTTP (`gomddoc serve/preview`)
      transports.

## Distribution and Packaging

_Make gomddoc easy to install across platforms and deployment targets. Currently gomddoc is built
from source via `make build` — these items add standard distribution channels._

- [ ] **`go install` support**: Ensure `go install github.com/monolithiclab/gomddoc/cmd/gomddoc@latest`
      works cleanly. Requires the module path to be publicly resolvable and the `embed` directive
      to work with `go install` (assets must be in the module, not generated). Verify version
      injection via `-ldflags` still works. May need a thin `main.go` wrapper if the `assets/`
      embed causes issues with `go install`. Low complexity.
- [ ] **Official Docker image**: Publish a multi-arch (`linux/amd64`, `linux/arm64`) Docker image
      to GitHub Container Registry (`ghcr.io/monolithiclab/gomddoc`). Multi-stage build with
      `gcr.io/distroless/static-debian12` as the runtime image (~5MB). Tags: `latest`, semver
      (`v1.2.3`), major (`v1`). Include a `HEALTHCHECK` instruction pointing at `/health/live`.
      Document `docker run` examples for serve, build, and mcp subcommands. Medium complexity.
- [ ] **Homebrew tap**: Create a `homebrew-tap` repository with a formula that downloads the
      pre-built binary from GitHub Releases. Formula should include a `test` block that runs
      `gomddoc --version`. Consider whether to start with a tap (`brew tap monolithiclab/tap && brew
install gomddoc`) or aim for Homebrew core inclusion later. Low complexity once releases
      exist.
- [ ] **GitHub Releases with GoReleaser**: Set up GoReleaser to produce cross-platform binaries
      (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64) on tagged releases.
      Generate checksums and a changelog from conventional commits. GoReleaser can also drive the
      Homebrew formula and Docker image builds. Medium complexity.
- [ ] **CI pipeline (GitHub Actions)**: Automated build, test, and lint on push/PR. Run `make ci`
      with coverage threshold enforcement (87%+). Gate merges on passing CI. Prerequisite for
      automated releases and benchmark tracking. Medium complexity.
- [ ] **Install script**: One-liner `curl | sh` install script that detects OS/arch, downloads the
      correct binary from GitHub Releases, and places it in `/usr/local/bin` (or `$HOME/.local/bin`).
      Common pattern for CLI tools. Low complexity.

## Phase 11: Enterprise Features

- [ ] **TLS with automatic certificates**: Built-in HTTPS via `golang.org/x/crypto/acme/autocert`.
      `--tls-auto` flag enables Let's Encrypt automatic certificate provisioning and renewal with
      no manual cert management. Certificates cached to disk (`--tls-cache-dir`, default
      `~/.cache/gomddoc/certs`). Requires port 443 and a publicly reachable domain. Serves HTTP
      on port 80 only for ACME challenges and HTTPS redirects. For self-managed certificates,
      `--tls-cert` and `--tls-key` flags accept PEM file paths. Both modes call
      `http.Server.ListenAndServeTLS` — no reverse proxy required for simple deployments. Medium
      complexity.
- [ ] **S3 provider**: Serve content directly from S3 buckets (for non-git use cases).
- [ ] **Authentication**: OIDC/OAuth middleware for private documentation.
- [ ] **Branch switching**: UI dropdown to switch between Git branches/tags.

## Phase 12: Git Workflow Integration

- [ ] **Webhooks**: Endpoint to trigger `git fetch` on push events (cache invalidation).
- [ ] **PR preview**: Serve content from PR branches for review.

## Theme Configuration

_Allow themes to ship sane defaults for features and variables, reducing site-level boilerplate._

- [ ] **Theme-level config file**: Each theme can include a `config.yml` in its directory
      (e.g., `assets/themes/midnight/config.yml`) defining default feature toggles and theme
      variables. This lets themes declare their intent — a dark-first theme can default
      `dark_mode: true`, a minimal theme can disable `color_chips`, and every theme can ship
      its full color palette as variable defaults without requiring the site to copy them.

      **Config loading order** (highest wins):

      ```
      CLI flags / arguments
        ↓
      Environment variables (GOMDDOC_SITE_*)
        ↓
      Page frontmatter (per-page overrides)
        ↓
      Site config (.gomddoc/config.yml)
        ↓
      Theme config (assets/themes/<name>/config.yml)
        ↓
      Hardcoded defaults
      ```

      **Theme config schema** (subset of site config — only presentation-related fields):

      ```yaml
      # assets/themes/midnight/config.yml
      features:
        dark_mode: true
        color_chips: false

      theme:
        vars:
          bg: "#0d1117"
          text: "#e6edf3"
          primary: "#7c3aed"
          dark-bg: "#0d1117"
          dark-text: "#e6edf3"
      ```

      Theme config provides defaults that site config overrides. A site using the `midnight`
      theme gets its color palette for free, but can override any variable in
      `.gomddoc/config.yml` under `theme.vars`. Similarly, theme feature defaults are
      overridden by site-level `features:` and then by per-page frontmatter.

      **Scope restrictions**: Theme config can only set `features` and `theme.vars` — not
      `meta`, `edit_url`, `default_index`, or other operational settings. This prevents themes
      from overriding site identity or behavior.

      **Implementation**: Load theme config after defaults, before site config, in the config
      loading pipeline. Use the same YAML parsing as site config. Bundled themes include their
      config in the embedded FS; installed themes load from `.gomddoc/assets/themes/<name>/`.
      Medium complexity.

## Phase 13: Theme Marketplace

_Enable community theme sharing via a GitHub-based registry._

### 13a: Theme Registry

- [ ] **Default marketplace repository**: A GitHub repository (e.g. `gomddoc/themes`) acts as the theme
      registry. Contains an `index.json` manifest listing available themes with name, description, author,
      version, repository URL, and preview image URLs.
- [ ] **Theme package format**: Each theme is a Git repository containing `default.html.tmpl`, optional
      partials, an optional `config.yml` with feature and variable defaults (see Theme Configuration),
      a `README.md` with frontmatter metadata (name, category, fonts, colors), and screenshot
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

## Bugs

- [ ] When falling back to the default theme for layout (eg. error.html.tmpl), the template loads the
      partials from default rather than those from the overloaded theme.

## Future CLI Commands

| Command                 | Purpose                                              | Status  |
| ----------------------- | ---------------------------------------------------- | ------- |
| `gomddoc serve`         | Production HTTP server for documentation             | Done    |
| `gomddoc build`         | Static site generation                               | Done    |
| `gomddoc preview`       | Quick local preview with auto-open browser           | Done    |
| `gomddoc init`          | Scaffold a `.gomddoc/` directory with default config | Done    |
| `gomddoc mcp`           | MCP server for AI-native documentation access        | Done    |
| `gomddoc info`          | Show version, config file location, environment vars | Done    |
| `gomddoc validate`      | Validate config and check for broken links           | Planned |
| `gomddoc theme list`    | List available themes from the marketplace           | Planned |
| `gomddoc theme search`  | Search themes by name, category, or keyword          | Planned |
| `gomddoc theme info`    | Show detailed theme information                      | Planned |
| `gomddoc theme install` | Download and install a theme                         | Planned |
| `gomddoc theme update`  | Update installed theme(s) to latest version          | Planned |

## Implementation Strategy

Development proceeds in phases building on stable foundations. Each phase delivers complete, tested functionality.

**Immediate focus:** Phase 9c SEO polish, tag components, distribution/CI pipeline.
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
- Auto-generated meta description (authors should set frontmatter `description`)
- Social preview image generation (high complexity, requires image deps, niche benefit)
- SEO content analysis / keyword scoring (Yoast-style — content strategy, not technical SEO)
- AMP (deprecated as ranking signal by Google)
- News/video sitemaps (documentation sites don't need these)
- WebP/AVIF image conversion (belongs in CI/CD pipeline, not documentation server)
