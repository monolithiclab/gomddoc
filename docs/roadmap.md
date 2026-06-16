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

## Idea

- MCP enable website for forms, etc. eg.
  agents discover monolithic and are able to interact with it.
- Provide generic gomddoc skill.

## Current State

The foundation is production-ready with 87.3% test coverage. For a full description
of current capabilities, see `docs/architecture.md`.

**Completed phases:** 1-3 (core), 4 (partial), 5 (partial), 6 (renderer enhancement),
7 (partial — sitemap generation complete), 7b (preview), 8 (theming engine), 9a (pre-launch SEO),
9b-9d (most post-launch SEO), 10a-10b (MCP server + HTTP), tag components (chips, listing/index
pages, `tag:` search, related/see-also section), i18n/l10n (UI strings + multi-language content).
A minimal GitHub Actions CI pipeline (`make lint test` on push) is in place. Six review batches
resolved 45/46 identified issues (security, correctness, deduplication, hardening).

**Phase 5 note:** Server-side full-text search and client-side search UI are complete for `serve`
and `preview` modes. Build-mode search (Pagefind) deferred.

**Phase 4 note:** Benchmarks, pprof, and allocation reduction are complete. CI benchmark
tracking is deferred until a CI pipeline is established. Partial clones are blocked by
go-git library limitations.

## Phase 4: Performance and Scaling

- [ ] **Partial clones**: `git clone --filter=blob:none` when upstream library support matures.
- [ ] **CI benchmark tracking**: Run benchmarks in CI with `go test -bench -benchmem`. Use
      `benchstat` to detect regressions against the baseline. Fail CI on >10% degradation.
      Now unblocked — a minimal CI pipeline exists (see Distribution & Packaging). Makefile targets
      (`bench-save`, `bench-compare`) support local regression detection.
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

- [x] **Guard build output directory**: `gomddoc build` uses a `.gomddoc-build` sentinel file to
      track directories it created. Non-empty directories without the sentinel are refused,
      preventing accidental deletion of unrelated files. Low complexity.
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
- [x] **Exclude patterns**: Configurable `exclude` list in `.gomddoc/config.yml` using glob patterns
      (`path.Match` syntax). Patterns block matching files from HTTP serving, navigation, search,
      metadata indexing, MCP access, and directory listings. `IsExcludedPath` + `IsRestrictedPath`
      consolidate hidden-path and exclude-pattern checks. `ContentExclusion` middleware replaces
      `BlockHiddenPaths`. All blocked paths return 404 (no existence leakage).
- [x] **URL extension stripping**: `strip_extensions` config option (default: `[".md"]`) removes
      specified extensions from URLs. Requests to `/docs/guide.md` redirect (301) to `/docs/guide`.
      In build mode, generates directory-based URLs (`guide/index.html`) for static host compatibility.
      Implemented in `internal/resolve` with collision detection (first configured extension wins).
      Low-medium complexity.

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

- [x] **Clickable tag chips in page rendering**: Render frontmatter `tags` as clickable chips
      (styled inline elements) on each page. Each chip links to a tag listing page. Position
      configurable via theme template (typically below the page title or in a sidebar metadata
      section). All 8 built-in themes must include the tag chips. Low-medium complexity.
- [x] **Tag listing page (`/tags/{tag}`)**: Server-rendered HTML page listing all pages tagged
      with a given tag. Reuses the existing `MetaIndex.ByTag()` lookup. Each result shows title,
      description, and path as a clickable link. Shares the site's theme and navigation chrome.
      In `build` mode, generate a static HTML page per tag under `tags/`. Medium complexity.
- [x] **Tag index page (`/tags/`)**: Overview page listing all tags with document counts. Each
      tag links to its listing page. Serves as a discovery entry point. Generated statically in
      `build` mode. Low complexity.
- [x] **Search by tag (`tag:` prefix)**: Extend the search engine to support `tag:XXX` syntax.
      When a query term starts with `tag:`, match it against the metadata index tags instead of
      the full-text index. Can be combined with free-text terms (e.g., `tag:deployment kubernetes`
      searches for "kubernetes" in pages tagged "deployment"). A static "Tip: tag:name filters by
      tag" caption below the search input makes the syntax discoverable (autocomplete/suggestions
      deferred). Medium complexity.
- [x] **Related pages via tags**: Display a "Related pages" section at the bottom of each page,
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

## Self-Documentation via MCP

_gomddoc bundles its own guide and serves it to AI agents via CLI MCP, enabling agents to learn
how to build documentation sites with gomddoc without external docs._

- [ ] **Bundled guide content**: Embed the `docs/guide/` documentation into the binary via `embed.FS`.
      This is gomddoc's own user guide — configuration, theming, features, MCP usage, etc. The
      embedded content is self-contained and versioned with the binary.
- [ ] **`gomddoc mcp` serves bundled guide**: When `gomddoc mcp` runs without a content directory
      argument, it serves the bundled guide instead of requiring user content. This lets any
      MCP-compatible agent (Claude, Cursor, Windsurf, etc.) query gomddoc's own documentation to
      learn how to set up a site, configure themes, write frontmatter, use exclude patterns, etc.
      With a content directory, the guide content is available alongside user content (e.g. via a
      `guide://` resource prefix or a `gomddoc_help` tool).
- [ ] **Agent onboarding prompt**: Add an MCP prompt (`learn_gomddoc`) that walks an agent through
      gomddoc's capabilities — what config options exist, how themes work, what frontmatter fields
      are available — by pulling from the bundled guide. This is the "teach me how to use you"
      entry point.

**Why:** gomddoc's MCP server already lets agents query _user_ documentation. Bundling its own
guide closes the loop — agents can learn how to _use_ gomddoc itself via the same protocol.
Dogfooding the MCP interface with gomddoc's own docs.

## Distribution and Packaging

_Make gomddoc easy to install across platforms and deployment targets. A GoReleaser-based release
pipeline now exists (`.goreleaser.yaml` + `.github/workflows/release.yml`); it activates on a pushed
`vX.Y.Z` tag and requires the `HOMEBREW_TAP_TOKEN` repo secret plus ghcr package permissions._

- [x] **GitHub Releases with GoReleaser**: `.goreleaser.yaml` produces cross-platform binaries
      (linux/darwin × amd64/arm64) as tar.gz archives with `SHA256SUMS`, a grouped changelog from
      conventional commits, and a GitHub Release on tagged builds. Checksums are cosign-signed (keyless
      via OIDC). _Note:_ windows builds were omitted (gomddoc is primarily a server) — add a `windows`
      goos entry if a Windows binary is wanted.
- [x] **`go install` support**: module path is public, assets are embedded in-module via `//go:embed`,
      and `-ldflags -X main.version` injection is wired. `go install github.com/monolithiclab/gomddoc/cmd/gomddoc@latest`
      should work — verify against the first published tag.
- [x] **Official Docker image**: GoReleaser `dockers:` + `docker_manifests:` publish a multi-arch
      (`linux/amd64`, `linux/arm64`) image to `ghcr.io/monolithiclab/gomddoc` with `latest` + semver
      tags, built from the `gcr.io/distroless/static-debian12:nonroot` runtime and cosign-signed.
      _Deferred:_ a `HEALTHCHECK` instruction (distroless has no shell; would need a binary-based probe)
      and major-version (`v1`) tag.
- [x] **Homebrew tap**: GoReleaser `brews:` publishes a formula to `monolithiclab/homebrew-tap`
      (`brew install monolithiclab/tap/gomddoc`) with a `gomddoc --version` smoke test. Requires the
      `HOMEBREW_TAP_TOKEN` secret.
- [x] **CI pipeline (GitHub Actions)**: `.github/workflows/ci.yml` runs `make lint test` (vet, gofmt,
      staticcheck, golangci-lint, gosec, gocritic, `go test -race -cover`) on an ubuntu+macOS matrix
      for pushes to `main` and all PRs, with Go module caching, concurrency cancellation of superseded
      runs, and coverage-artifact upload. _Remaining enhancement (deferred):_ coverage threshold
      enforcement (87%+).
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

## Internationalization and Localization

_Multi-language documentation sites with translated UI chrome._

### UI String Localization

- [x] **Locale file format**: Three-layer YAML locale loading (built-in → theme → site overrides)
      with BCP 47 filenames (e.g., `en-US.yml`, `fr-FR.yml`). Built-in `en-US.yml` ships with 20
      UI string keys. Sites override via `.gomddoc/locales/`.
- [x] **`language` config integration**: Default language changed to BCP 47 `en-US`. Templates
      access strings via `{{ .T "key" }}` with fallback chain: requested lang → default → raw key.
      `locale.Bundle` manages loading and lookup.
- [x] **Per-page language override**: Frontmatter `lang` overrides site-level language for
      `<html lang>` and locale string selection via `TemplateContext.Lang()`.

### Multi-Language Content

- [x] **Content tree per language**: BCP 47 directories at content root (e.g., `fr-FR/`).
      Default language served at `/`, others at `/{lang}/`. Each language gets its own Provider
      (SubdirProvider), metadata index, search index, and navigation tree. Auto-detected from
      directory names matching BCP 47 pattern.
- [x] **Language switcher**: `lang-switcher.html.tmpl` partial in default theme. Shows active
      language as text, others as links to `/{lang}/` prefixed paths.
- [x] **`hreflang` tags**: `hreflang.html.tmpl` partial renders `<link rel="alternate">` tags
      with `x-default` for the default language. Included via `head-shared.html.tmpl`.
- [x] **Per-language sitemap**: Per-language routes at `/{lang}/sitemap.xml` and `/{lang}/feed.xml`.
      Build mode generates `sitemap-index.xml` referencing per-language sitemaps.

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

- [x] **Per-language content pages 404 in `serve` mode** (HIGH — REVIEW.md §9.1). FIXED: the
      `/{lang}` prefix is now stripped before the language handler and each language pipeline builds
      its own resolver; serve+build integration tests added (§9.5). A follow-on provider bug
      (`fs.Sub(os.DirFS)` not being `fs.StatFS`, §9.7) that silently dropped per-language pipelines
      was also fixed.
- [x] **See-also links emit raw `.md` paths** (MEDIUM — REVIEW.md §9.2). FIXED: `see-also.html.tmpl`
      now resolves `$doc.Path` via the `contentURL` template func, matching every other link type on
      `strip_extensions` sites.
- [x] **Tag pages re-parse their partial template on every request** (MEDIUM perf — REVIEW.md §9.3).
      FIXED: parsed partials are now cached (keyed by `theme/name`), so `/tags/` requests use the same
      cache path as the main content path.
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

**Immediate focus (next steps, in priority order):**

1. **Distribution & Packaging** — the main blocker to public adoption. gomddoc is build-from-source
   only today. Sequence: GoReleaser → GitHub Releases (cross-platform binaries + checksums) → then
   `go install` verification, `curl | sh` install script, Homebrew tap, and the multi-arch Docker
   image, all of which build on releases. CI already runs `make lint test`.
2. **Self-Documentation via MCP** — bundle `docs/guide/` via `embed.FS` so `gomddoc mcp` (no content
   dir) serves gomddoc's own guide, plus a `learn_gomddoc` onboarding prompt. Contained, high-leverage
   differentiator that dogfoods the MCP interface.
3. **Developer-experience polish** — autoreload in `preview` (SSE), next/previous page links at page
   bottom (the `<link rel="next/prev">` head tags already exist), and build-mode asset minification.
4. **Bug fix** — theme-fallback layout loads partials from the default theme instead of the active
   overloaded theme (see Bugs).

**Maintenance:** The 14th review pass (REVIEW.md §9, landed 2026-06-11) is **concluded** as of
2026-06-16. The entire §9.10 priority list was addressed: the HIGH i18n serve 404 (§9.1) plus a
follow-on provider bug it exposed (§9.7), the MEDIUM tag/see-also consistency and perf items
(§9.2/§9.3), the shared page/error context builders (§9.2), the test gaps in multi-language build and
multi-tag search (§9.5), and the documentation drift (§9.6). Two perf opportunities are deferred to a
future pass (neither a defect): the per-request nav-tree copy (§9.3 `buildNavItems`) and the double
markdown parse (§9.8). With i18n now functional end-to-end, the distribution push is unblocked.

**Deferred:** CI benchmark tracking (now unblocked but low priority), build-mode search (Phase 5,
Pagefind), partial clones (blocked by go-git).

## Deferred (Not Planned)

These features were considered but deprioritized to keep gomddoc focused:

- Database providers (PostgreSQL/SQLite) — Git is the database
- REST/GraphQL APIs — gomddoc is a viewer, not a headless CMS
- Editorial workflows (drafts, reviews, scheduling) — use Git branches
- Content versioning/revision history — use Git history
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
