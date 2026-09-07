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

- Provide generic gomddoc skill.
- Build-time assertion that no static-build output path matches an `exclude` pattern, failing the
  build if one does. Would have caught the §10.1 clean-URL bypass's `build`-mode stub leak
  (REVIEW.md §10.1) without anyone needing to think to look for it — a second, independent check
  on top of the fix itself.

## Current State

The foundation is production-ready with 90.8% test coverage (target: 87%+). For a full description
of current capabilities, see `docs/architecture.md`.

**Completed phases:** 1-3 (core), 4 (partial), 5 (partial), 6 (renderer enhancement),
7 (partial — sitemap generation complete), 7b (preview), 8 (theming engine), 9a (pre-launch SEO),
9b-9d (most post-launch SEO), 10a-10b (MCP server + HTTP), tag components (chips, listing/index
pages, `tag:` search, related/see-also section), i18n/l10n (UI strings + multi-language content).
A GitHub Actions CI pipeline (lint + test on an ubuntu+macOS matrix, plus a `govulncheck` job) is in
place — see Distribution and Packaging. Four review passes (13th, i18n simplification, 14th, 15th —
see REVIEW.md) have fixed every HIGH finding and the large majority of MEDIUM/LOW ones; the
remainder is tracked in REVIEW.md and prioritized in this doc's Implementation Strategy section.

**Phase 5 note:** Server-side full-text search and client-side search UI are complete for `serve`
and `preview` modes. Build-mode search (Pagefind) deferred.

**Phase 4 note:** Benchmarks, pprof, and allocation reduction are complete. CI benchmark tracking
is unblocked (a CI pipeline now exists) but not yet done — see Phase 4, still low priority. Partial
clones are blocked by go-git library limitations.

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
- [x] **Static host compatibility, mostly**: directory-based clean-URL output (`guide/index.html`)
      and static `404.html` generation already target S3/Netlify/GitHub Pages/Cloudflare Pages
      compatibility (see CLI and Developer Experience's URL Extension Stripping, and Phase 9c's
      Static error pages). What's left is narrower than the original scope suggested — see Asset
      optimization above, and the still-open serve/build parity gaps tracked in this doc's
      Implementation Strategy.

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
See `docs/seo-competitive-analysis.md` for the full competitive analysis — it is the pre-Phase-9
research that produced this list, so its verdicts on gomddoc are historical; this section is the
authoritative record of what shipped._

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
- [x] **Consistent "last modified" fallback**: `seo.LastModified`/`seo.StatModTime` are now the one
      place `datePublished`/`dateModified` get derived — file `Stat()` (Git provider returns commit
      time) falling back to frontmatter `date`, `.UTC()`-normalized. Used by JSON-LD `dateModified`
      and both feed/sitemap `<lastmod>`, so the four call sites that used to hand-roll this
      independently can no longer disagree.
- [ ] **True per-file Git commit dates**: the Git provider's `ModTime` is still one constant (the
      HEAD commit's `Author.When`) applied to every file in the tree, not a per-file `git log` walk —
      so `datePublished`/`dateModified` above are a consistent *fallback*, not real "first commit" /
      "last commit" dates per page. Needs a per-path commit-log walk, cached at startup or lazily to
      avoid per-request Git operations (real cost on a git-backed site — see CLAUDE.md's "go-git
      reads are writes"). Medium complexity.
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
- [x] **Heading anchor slug stability — documented and pinned**: `TestHeadingSlugs`
      (`internal/renderer/markdown_test.go`) pins goldmark's `WithAutoHeadingID` output across both
      the renderer's and the enricher's parser instances; `docs/guide/12-advanced/02-markdown-extensions.md`
      documents the five-rule algorithm and a worked table.
- [ ] **Heading anchor escape hatch + non-ASCII handling** (REVIEW.md §10.5): goldmark's slugifier
      drops every non-ASCII character rather than transliterating it — `# Café Français` becomes
      `caf-franais`, and an all-non-Latin heading (Japanese, Korean, Greek, Cyrillic) falls back to
      a positional `heading-1`, `heading-2`, ... that reorders itself the moment a section is added.
      There's no author-side override either: `parser.WithHeadingAttribute()` isn't enabled, so the
      conventional `{#custom-id}` syntax doesn't work. Two independent fixes, either or both: enable
      `parser.WithHeadingAttribute()` (cheap, standard escape hatch for any mangled slug); a custom
      `parser.IDs` that percent-encodes non-ASCII runes instead of dropping them (larger — changes
      existing anchors, a breaking change for any deployed site's inbound links). Low-medium
      complexity for the first, medium for the second.

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
      section). All 8 themes (`default` plus the 7 in `gomddoc-themes`) must include the tag chips. Low-medium complexity.
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

- [x] **Data plumbing**: `navigation.Generator.PrevNext` (O(1) lookup against a flattened page index
      built once at cache warm-up) feeds `PageContext.PrevPage`/`NextPage` in both `serve` and
      `build`. Currently consumed only by the `<link rel="prev/next">` **head** tags
      (`head-shared.html.tmpl`) — the data every UI piece below needs already exists and is wired.
- [ ] **Visible page-bottom UI**: no theme renders `.PrevPage`/`.NextPage` anywhere but the head
      tags. Render a two-column footer with page titles in all 8 themes (`default` plus the 7 in
      `gomddoc-themes`). Low complexity now that the data plumbing is done — this is template
      integration only.
- [ ] **Frontmatter `prev`/`next` overrides**: allow `prev`/`next` frontmatter fields pointing to
      relative paths (e.g., `next: 02-configuration.md`) to override the navigation-tree-derived
      order; a value of `false` suppresses the link for that direction. Not started — the current
      `PrevNext` always derives from navigation-tree order. Low-medium complexity.

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

### 10c: WebMCP — Browser-Native Tool Exposure (Idea, blocked on browser support)

_Both of gomddoc's existing MCP transports (10a, 10b) require the visiting agent to be an MCP
client with `gomddoc mcp` (or `/_mcp/`) configured. WebMCP is a different reach: it lets a page
register tools an agent already *in the browser tab* can call, with no MCP client setup at all._

WebMCP (`navigator.modelContext.registerTool`, W3C Web Machine Learning Community Group, jointly
authored by Google and Microsoft) lets a page declare typed, JSON-Schema tools that an in-page
agent discovers and calls directly — no separate transport, no server-to-agent connection, just a
JS API a rendered page calls on itself. Chrome ships it behind an origin trial (Chrome 149+,
`chrome://flags/#enable-webmcp-testing`); OpenAI turned on "Site tools" in ChatGPT's Atlas browser
in August 2026. It is aimed mainly at transactional sites (booking, checkout) where an agent would
otherwise have to guess which button does what — a documentation site has no such workflow to
expose. What it *does* give gomddoc: a way to hand a browser-resident agent (Claude in Chrome,
Copilot, ChatGPT's browsing mode) the same structured access `internal/mcp` already gives a
configured MCP client, for a reader who has a gomddoc page open and nothing configured.

- [ ] **Register `search_docs` / `find_related` / `get_table_of_contents` as WebMCP tools**: in
      `serve`/`preview` mode, have the rendered page call `navigator.modelContext.registerTool`
      (feature-detected — most browsers don't implement it yet) for the same operations
      `internal/mcp/tools.go` already exposes, backed by the same `internal/search`/
      `internal/template/navigation` indexes behind `/api/search`. An in-tab agent gets structured
      search and navigation instead of scraping the DOM, without the reader or the agent's harness
      configuring an MCP client. Gate behind a `.Feature` like every other optional UI component
      (see `docs/roadmap.md`'s Theme Configuration section), off by default until the API is
      unflagged in at least one shipping browser.
      **Not implementable end-to-end yet** — `build` output is static HTML with no backend for the
      tool's `execute` callback to call, so like Content Annotations this is `serve`/`preview` only
      unless a client-side search index (Phase 5's deferred Pagefind path) ships first. Revisit once
      Chrome/Edge ship the API unflagged; low-medium complexity once the platform stabilizes.

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

**Publishing status (verified against the published v0.1.1).** v0.1.0's release job 403'd on a
read-only `HOMEBREW_TAP_TOKEN`, and the repo and ghcr package were both private. All four are
resolved; every install path was exercised end to end:

| Path            | Status                                                                    |
| --------------- | ------------------------------------------------------------------------- |
| `curl \| sh`    | ✅ Resolves latest, verifies the SHA256 against `SHA256SUMS`, installs.    |
| `brew install`  | ✅ Formula published to `monolithiclab/homebrew-tap`.                      |
| `go install`    | ✅ Installs from `proxy.golang.org`.                                       |
| `docker pull`   | ✅ Anonymous pull works; multi-arch index (`linux/amd64`, `linux/arm64`).  |
| cosign          | ✅ `verify-blob` on `SHA256SUMS` returns `Verified OK`.                    |

_Note:_ docker tags are **unprefixed** (`ghcr.io/monolithiclab/gomddoc:0.1.1`, not `:v0.1.1`) —
GoReleaser's `{{ .Version }}` strips the `v`.

- [x] **GitHub Releases with GoReleaser**: `.goreleaser.yaml` produces cross-platform binaries
      (linux/darwin × amd64/arm64) as tar.gz archives with `SHA256SUMS`, a grouped changelog from
      conventional commits, and a GitHub Release on tagged builds. Checksums are cosign-signed (keyless
      via OIDC). Every action in both workflows is pinned to a full commit SHA with a `# vX.Y.Z`
      comment, and goreleaser to `~> v2.17` — the release job holds `id-token: write`, so an upstream
      tag repoint would otherwise be able to mint valid Sigstore signatures over arbitrary artifacts.
      Dependabot's `github-actions` ecosystem keeps the pins current. _Note:_ windows builds were
      omitted (gomddoc is primarily a server) — add a `windows` goos entry if a Windows binary is
      wanted.
- [x] **`go install` support**: module path is public, assets are embedded in-module via `//go:embed`,
      and `go install github.com/monolithiclab/gomddoc/cmd/gomddoc@latest` works. `-ldflags -X main.version`
      injection is wired for GoReleaser and `make build`, but `go install` applies no ldflags — v0.1.1
      installed that way reported `dev`. `cmd/gomddoc/version.go` now falls back to the module version
      the toolchain stamps into build info (`debug.ReadBuildInfo`), and normalizes every source to the
      unprefixed form so all build paths agree.
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
- [x] **Install script**: `scripts/install.sh` — POSIX `sh`, no dependencies beyond curl/wget and
      tar. Detects OS/arch (linux/darwin × amd64/arm64), resolves the latest tag via the GitHub API
      (or honours `GOMDDOC_VERSION`), verifies the archive against the release `SHA256SUMS` before
      installing, and targets `/usr/local/bin` with a `~/.local/bin` fallback (`GOMDDOC_INSTALL_DIR`
      overrides). Fails closed when no SHA-256 tool is present, and verifies the cosign signature
      over `SHA256SUMS` when cosign is on `PATH` (`GOMDDOC_REQUIRE_COSIGN=1` makes it mandatory).
      Warns when the install dir is off `PATH`. Served via
      `curl -fsSL .../scripts/install.sh | sh`.
- [x] **Release preflight for the Homebrew tap token**: `.github/workflows/release.yml` validates
      that `HOMEBREW_TAP_TOKEN` has push access to `monolithiclab/homebrew-tap` **before** building,
      distinguishing unset / 401 / 404 / read-only and printing the exact remediation. Prerelease
      tags warn instead of failing, matching `skip_upload: auto`. Added after the v0.1.0 release
      failed at the final step (403) having already published a formula-less GitHub Release.

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

## Content Annotations → Agent Handoff

_Select text in a rendered page, leave a note, and have an agent turn the accumulated notes into a
branch of Markdown edits. Review, amendment and "we should also document X" all start from the page
where the reader noticed the problem, instead of from a blank issue form._

The pull is real: today a reader who spots a wrong sentence has to translate "third paragraph under
*Caching*" into a file path, find the line, and write a diff — and the reader who spots it is usually
not the person who can fix it. An annotation captures the observation at full fidelity at the moment
it happens, and an agent is very good at turning "this contradicts the section above" into a patch.
But the naive version of this feature breaks four things gomddoc has deliberately built, and the
design below exists to survive them.

### What it breaks, and what that forces

1. **It is an editorial workflow, which is in Deferred (Not Planned).** "Editorial workflows (drafts,
   reviews, scheduling) — use Git branches" rules out exactly this, and the Vision says "no CMS. The
   'database' is Git". The line that keeps the feature honest: **an annotation is transport, not
   state.** It is never a source of truth, never rendered as site content, and its whole lifecycle is
   *captured → drained into a diff → deleted*. A comment thread that accumulates replies and outlives
   the commit is a CMS and is out of scope. If the feature starts growing statuses, assignees or
   resolution states, it has become the thing this repo declined to build.
2. **Character offsets into rendered HTML do not address Markdown.** goldmark rewrites text on the way
   through: `**bold**` contributes the four characters `bold`, an admonition contributes a title the
   source never wrote, and `# Café Français` yields the id `caf-franais` (both pinned by
   `TestHeadingSlugs`, `renderer/markdown_test.go:241`). So a `(line, char)` pair taken from the DOM addresses a document that exists
   only in the browser. Worse, it is invalidated by the *next* commit even when the sentence it points
   at is untouched. The anchor has to survive both translations.
3. **It is the first untrusted input in the product.** CLAUDE.md's trusted content model — "no
   untrusted user input reaches rendered output", which is why there is no sanitizer and no CSP, and
   why reviewers are told to treat XSS findings as false positives — holds only because every string
   in a page came from the author's git repo. An annotation body does not. That convention must either
   be narrowed in writing (annotations are the documented exception) or preserved by never rendering
   an annotation body into a page. **Prefer the second**: the reader's own note can be echoed
   client-side without a server round-trip, and every other consumer is a CLI or an agent.
4. **`AuthStore` is a shared password, not an identity.** Basic auth gates the content port with one
   credential (`server.go:105`), so the server cannot attribute an annotation to anyone. An unsigned
   note from nobody is not reviewable. Until there is real identity (Phase 11 OIDC), the author is
   whatever the annotator types, and the design has to be honest that this is an internal-trust
   feature, not a public "suggest an edit" widget.

Two further limits worth stating before anyone plans around them: `gomddoc build` output is static
files, so annotation capture is a `serve`/`preview` feature only — and static hosting is the largest
deployment mode. And for a one-word typo the existing edit link already wins. Annotations earn their
keep on the case the edit link is bad at: noticing something *without* knowing the fix, and
accumulating a dozen small observations across a reading session into one coherent handoff.

### The design that survives them

- [ ] **Quote-based anchors, not offsets**: adopt the W3C Web Annotation Data Model's
      `TextQuoteSelector` — `exact` plus ~32 characters of `prefix` and `suffix` — rather than
      inventing a scheme. Resolution re-finds the quote in the **Markdown source**, so it tolerates
      edits anywhere else in the file. Store alongside it: the real content path (not the URL —
      `strip_extensions` means they differ), the commit SHA the reader was looking at, and the
      language of the pipeline that served the page.
- [ ] **`data-src-line` on block elements**: a goldmark extension in the shape of the existing
      `ext_anchors.go` / `ext_admonition.go`, emitting each block node's source line span from
      `node.Lines()`. That narrows quote resolution from the whole file to one block, which is what
      makes it robust for a quote that appears more than once, and it costs one attribute per block.
      A `TextPositionSelector` may ride along as a hint, never as the primary anchor.
- [ ] **Three resolution outcomes, all reported**: `exact` (quote found once in the block), `fuzzy`
      (found after normalizing whitespace, or found elsewhere in the file), `orphaned` (the text is
      gone). Orphaned annotations are the interesting ones — the sentence was already rewritten — and
      they must be surfaced rather than dropped. Reuse the existing search tokenizer for fuzzy
      matching rather than adding a dependency.
- [ ] **Append-only capture, off by default**: `annotations.enabled: false`. When on, `POST
      /api/annotations` appends one JSON object per line to `.gomddoc/annotations/<lang>.ndjson`,
      behind `http.MaxBytesReader` with capped field lengths. Append-only means no read-modify-write
      and no lock beyond the file; NDJSON means a corrupt line costs one annotation. The directory is
      gitignored by default — the notes are scaffolding, the commit is the artifact.
- [ ] **The harness pulls; gomddoc never pushes**: no LLM client in the binary. Shipping one would put
      an API key in a docs server's config, add egress from a process whose whole job is reading
      files, couple the release to a vendor's API, and — the real problem — require granting the
      server write access to the repository. Instead the agent comes to gomddoc, which it already can:
      add read-only MCP tools (`list_annotations`, `resolve_annotation`) beside the existing six, so
      any harness with the MCP server configured can enumerate annotations with their resolved source
      spans. The agent runs where the developer already runs it, with the credentials they already
      have, and produces a branch and a PR by the ordinary means.
- [ ] **`gomddoc annotations` subcommand**: `list`, `export` (a prompt-ready bundle — annotation,
      resolved span, surrounding source context), and `prune` (drop annotations whose quote no longer
      resolves *and* whose target file changed since the recorded SHA). `export` is the escape hatch
      for harnesses that do not speak MCP; `prune` is what keeps the store from becoming the archive
      this design says it must not be.
- [ ] **UI: capture only**: a selection popover that posts the annotation and confirms it, with the
      reader's own pending notes held client-side. No sidebar of other people's comments, no threads,
      no rendering of a stored body into the page — that is what keeps item 3 above from becoming a
      sanitizer, a CSP and a review-convention rewrite. Behind a `.Feature` gate like every other
      optional UI component, so themes opt in.

**Sequencing.** The anchor is the whole feature; everything else is plumbing around it. Build
`data-src-line` and the quote resolver *first*, with a `gomddoc annotations resolve` command and a
test corpus of hand-written anchors against a file that then gets edited underneath them. If
resolution is not reliable there, the UI is not worth building — an annotation that lands on the
wrong paragraph is worse than no annotation, because an agent will confidently patch the wrong
sentence. Realistically this follows Phase 11 identity work; without it the feature only makes sense
for a team that already trusts everyone who can reach the port.

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

## Visual Polish

_Small, self-contained presentation fixes that don't warrant their own phase._

- [ ] **Color chip: support other CSS color formats**: `hexColorPattern` in
      `internal/renderer/ext_colorchip.go` only matches 3/6-digit hex (`^#(?:[0-9a-fA-F]{6}|[0-9a-fA-F]{3})$`),
      so a code span like `` `rgba(255, 87, 51, 0.5)` `` renders as plain code instead of a
      `<gmd-color-chip>`. Extend the matcher (and the chip's own color-preview rendering) to also
      recognize `rgb()`, `rgba()`, `hsl()`, and `hsla()`. Low-medium complexity.
- [ ] **Enhance the default favicon**: `cmd/gomddoc/assets/themes/default/static/favicon.ico` is a
      generic placeholder — not distinctive or recognizable as gomddoc's mark. Design a proper mark
      and generate the full set per `docs/skills/favicons/SKILL.md` (favicon.ico, favicon.svg,
      apple-touch-icon, Android/maskable PNGs, site.webmanifest) rather than shipping only the legacy
      `.ico`. Low-medium complexity.
- [ ] **Replace the colorful sun/moon emoji dark-mode toggle with simple line icons**: the toggle
      button in `header.html.tmpl` hardcodes 🌙 as its default content, and `theme-toggle.mjs`
      documents that a theme *can* supply its own `<svg data-show-theme="light/dark">` icons but
      falls back to the emoji when it doesn't — every theme including `default` currently relies on
      that fallback. Ship simple monochrome sun/moon (or similar) line-icon SVGs in the default theme
      and carry them into the other 7 themes, so no theme ships the emoji fallback. Low complexity.

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

Development proceeds in phases building on stable foundations. Each phase delivers complete, tested
functionality. **This section is the one prioritized backlog** — it merges REVIEW.md's still-open
findings (15th pass, §10) with this roadmap's own unimplemented features into a single ranked list,
so the two documents can't quietly disagree about what's next. REVIEW.md tracks *what's wrong*; this
section tracks *what to do about it and in what order*.

### Tier 0 — Ship blocking

1. **Ship `v0.1.2` (security + canonical URLs)** — every released version (`v0.1.0`, `v0.1.1`) has
   a HIGH-severity `exclude` bypass: with `strip_extensions` active (the default), a file excluded
   as `SECRET.md` was still served in full at `/SECRET` while search, navigation and metadata
   correctly omitted it, so the bypass produced no visible signal (`build` additionally emitted
   redirect stubs disclosing every excluded file's name). Fixed by `c80981e`, now on `main` — see
   REVIEW.md §10.1 for the full writeup and regression tests. `v0.1.1` also ships static builds
   whose `rel="canonical"`/`og:url`/JSON-LD `@id` keep the `.md` extension, contradicting the same
   build's own `sitemap.xml` (found live on monolithiclab.fr, pinned to `v0.1.1`); fixed by
   `5dfc253`, landed two days after the `v0.1.1` tag — see REVIEW.md §10.2. What's left for both is
   delivery, not code: (a) tag `v0.1.2` and call out the exclude bypass in the release notes so
   operators pinned to `v0.1.1` can judge their own exposure and rotate anything sensitive that was
   exposed, (b) update `monolithiclab/homebrew-tap` so `brew install` stops shipping the vulnerable
   build, (c) bump the `GOMDDOC_VERSION` pin in downstream sites built from this repo
   (`monolithiclab/website`).

### Tier 1 — Correctness (serve/build parity and tag/i18n bugs, REVIEW.md §10.2/§10.3/§10.14)

Start with the design question — several of the bugs below are symptoms of it, and fixing them ahead
of a decision risks a second divergence to unwind later:

1. **Decide build's URL-space policy under `strip_extensions: []`** (REVIEW.md §10.2, HIGH — design
   question). `PageURLPath` (what serve publishes) and `htmlPath`/`prettyOutputPath` (what build
   writes) are two independently-diverging functions once extension stripping is disabled or a
   `README.md`+`index.md` collision occurs. Either build must force extension stripping, or it needs
   its own "publishing plan" that the sitemap/feed also consult. Record the decision in
   `docs/decisions.md` once made — this is exactly the kind of choice that entry is for.
2. **Serve/build parity fixes unblocked by that decision**: `/sitemap-index.xml` 404s in serve but
   build writes it; per-language tag pages gated on a domain in build only, not serve; the search UI
   ships in static builds with no `/api/search` backend; the language prefix never reaches
   `Page.Path` in build (hreflang mismatch); a directory index has two canonical URLs in serve;
   `redirect_from` on a default-index page targets a URL build never writes; search results link to
   raw `.md` paths.
3. **Tag/i18n correctness cluster** (independent of the design question above, can start now):
   synthetic tag pages render with nil `Features`/`Languages` (silently re-enables disabled features,
   no hreflang on `/tags/*`); every translated page's feed `<link rel="alternate">` points at the
   default-language feed (§10.14); `SiteConfig.Validate` never validates `language`; `?lang=` is
   case-sensitive.
4. **HIGH: two declared-and-documented parameters are silently ignored** (REVIEW.md §10.3, verified).
   `get_table_of_contents`'s `path` input is in the MCP tool's JSON Schema and documented as scoping
   the result to a subtree, but `handleGetTOC` never reads it — a client asking for one subtree gets
   the whole site back with no error. `redirectFinderAdapter` ignores its path argument and always
   does a global DFS from the root, so `/guide/` with no index redirects to the site's first page,
   not the first page under `/guide/`. Independent, low-effort fixes.
5. **Misc correctness, MEDIUM, independent fixes**: `findRelatedDocs` bypasses the tag-normalization
   single source of truth (untrimmed tags silently drop see-also); index pages appear in their own
   related-docs list in serve; git submodules misclassified as directories abort the whole build;
   `.well-known` silently dropped from git-backed sites; search results nondeterministic on score
   ties; snippet body truncates on a byte boundary (invalid UTF-8); non-root index canonical URLs
   miss a trailing slash; two frontmatter parsers disagree on what counts as frontmatter (raw YAML
   can leak into the search corpus).
6. **Bug fix**: theme-fallback layout loads partials from the default theme instead of the active
   overloaded theme (see Bugs).
7. **Heading anchor escape hatch + non-ASCII handling** (REVIEW.md §10.5, MEDIUM): see Phase 9c —
   cheap win (`parser.WithHeadingAttribute()`), closes a real gap on an i18n-capable server.

### Tier 2 — Security hardening (REVIEW.md §10.1, all findings past the v0.1.2 fix)

1. **MEDIUM: unauthenticated bcrypt CPU amplification** — no rate limit or lockout on failed Basic
   Auth attempts; ~10× CPU cost per bad-credential request. Bounded semaphore around `Validate`, or
   per-IP rate limiting on 401s.
2. **LOW cluster, low effort each**: `ci.yml` missing a `permissions:` block; `FilesystemProvider`
   missing the `fs.ValidPath` guard the git provider has; MCP resource/prompt handlers missing the
   `maxArgLen` cap their tool siblings apply; no query cap on `search_docs`; no max-file-size cap in
   `FilesystemProvider.ReadFile`/`gitTreeFS.Open`; no SBOM/SLSA provenance; release workflow's
   `go mod tidy` hook can mutate `go.sum` mid-release.

### Tier 3 — Roadmap features already in flight

1. **Self-Documentation via MCP** — bundle `docs/guide/` via `embed.FS` so `gomddoc mcp` (no content
   dir) serves gomddoc's own guide, plus a `learn_gomddoc` onboarding prompt. Contained, high-leverage
   differentiator that dogfoods the MCP interface.
2. **Next/previous page-bottom UI** — the data plumbing is done (see Page Navigation); this is
   template integration only, now low complexity.
3. **Developer-experience polish** — autoreload in `preview` (SSE), build-mode asset minification.
4. **True per-file Git commit dates** (see Phase 9b) — the consistent-fallback half already shipped;
   this is the real per-path `git log` walk, medium complexity.

### Tier 4 — Code quality & performance cleanup (opportunistic, REVIEW.md §10.4/§10.6/§10.7/§10.13)

Not release-blocking; pick up alongside adjacent work rather than as standalone tasks. Grouped by
theme rather than listed exhaustively — see REVIEW.md for the full set:

- **API consistency**: unmatched `/api/*` routes answer `text/plain` not JSON; `/api/*` responses are
  never cached/ETagged (`writeJSON` streams with no byte slice to hash).
- **Dedup/refactor candidates**: `TemplateCache` models a boolean as a two-implementation interface;
  `AllPages` has no `iter.Seq` sibling (5 consumers still deep-copy); MCP's `handleRelatedPages`
  reimplements `enricher.findRelatedDocs` without its bound/sort; three copies of the "fail one path"
  `fs.FS` test fake; `internal/template` has no `testhelpers_test.go` despite ~60 duplicated
  constructions; `Pipeline.Exclude` has three remaining stragglers still reading `cfg.Site.Exclude`
  directly (MCP deps ×2, default server content scope).
- **MEDIUM, currently unreachable in practice**: `LanguagePipeline.Languages`/`.ByLang` allowed to
  disagree when a language pipeline fails to build.
- **Performance, all measured LOW-impact**: inline-asset cache stores raw `[]byte`; `HasTemplate`
  does an `fs.Stat` per request; `emitTagPages` renders serially; tag pages rebuilt uncached per
  request; corpus read twice at startup (metadata + search index separately); request-scoped
  `context.Background()` in the breadcrumb `isDir` closure.
- **RFC 9110 §12.5.1 Accept negotiation edge cases** (MEDIUM, need an unusual client): `registry.Get`
  evaluates specificity globally rather than per candidate; `ParseAccept` drops `q=0` exclusions.
- **Test quality**: several tests that assert nothing (recommend deleting); coverage gaps in SSH
  auth, basic-auth branches, `cloneLocked`; missing benchmarks on `metadata.BuildIndex`,
  `internal/provider`, `resolve`, `template/breadcrumb`, `locale.Bundle.T`.

**Maintenance:** The 14th review pass (REVIEW.md §9, landed 2026-06-11) is **concluded** as of
2026-06-16, and the 15th pass (§10, landed 2026-07-29) has all HIGH findings fixed with the remainder
folded into the tiers above. One perf opportunity remains deferred from the 14th pass (not a
defect): double markdown parsing for enrichment + rendering (§9.8) — architectural, would need AST
sharing or a bounded response-body cache.

**Deferred:** CI benchmark tracking (now unblocked but low priority), build-mode search (Phase 5,
Pagefind), partial clones (blocked by go-git).

## Deferred (Not Planned)

These features were considered but deprioritized to keep gomddoc focused:

- Database providers (PostgreSQL/SQLite) — Git is the database
- REST/GraphQL APIs — gomddoc is a viewer, not a headless CMS
- Editorial workflows (drafts, reviews, scheduling) — use Git branches. Content Annotations is the
  one deliberate brush against this line: it is scoped to capture-and-drain precisely so it stays
  transport rather than becoming the comment store this entry rules out.
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
