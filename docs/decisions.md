# Architectural Decisions Log

Key decisions made during gomddoc development, including alternatives considered and reasons for rejection.
Extracted from completed spec files before deletion.

## CLI Framework

**Chosen**: Kong (`github.com/alecthomas/kong`)

**Alternatives considered**:
- **Cobra** (`github.com/spf13/cobra`): Industry standard (Docker, K8s, Hugo), but imperative style, heavier deps (~5 transitive), doesn't natively show env vars in help, verbose boilerplate
- **urfave/cli v3**: Popular, good subcommand support, but v3 less battle-tested, not struct-based, more boilerplate than Kong
- **stdlib `flag`**: Zero deps but requires significant custom code for subcommand routing, help gen, env var display

**Why Kong**: Declarative struct tags mirror existing `env:`/`yaml:` tag pattern. Native env var display in `--help`. Zero transitive deps. Compile-time checked subcommands. Auto-generated exhaustive help.

## Git Library

**Chosen**: go-git (`github.com/go-git/go-git/v5`) — pure Go

**Alternatives considered**:
- **git2go (libgit2)**: More complete but requires CGO
- **Shell exec (`git` binary)**: Simple but requires git installed on host
- **GitHub API**: Only works with GitHub, not GitLab/Bitbucket

**Why go-git**: Pure Go, no CGO, embedded-friendly, works with any Git server.

## Content Rendering Architecture

**Chosen**: MIME-type based registry with `ContentRenderer` interface returning `*RenderResult`

**Key decisions**:
- **MIME normalization**: `mime.ParseMediaType()` strips charset for routing, full type preserved for HTTP headers
- **Registry.Get() returns `(ContentRenderer, error)`** not `(ContentRenderer, bool)` — idiomatic Go error handling
- **Registry collisions**: Warn with `slog.Warn`, then override (enables testing/plugins)
- **Wildcard matching order**: exact > `type/*` > `*/*`
- **PassthroughRenderer**: Registered as `*/*` wildcard — simplest approach, zero maintenance when new file types appear
- **Context in Provider**: `context.Context` added to `ReadFile` and `Stat` for cancellation/deadline propagation from HTTP handlers. Supersedes the earlier decision to omit context for `fs.FS` compatibility.
- **Provider type**: `fs.StatFS` interface value, not `*os.FS` pointer
- **MIME registration**: In `MarkdownRenderer` `init()` — co-located with the renderer that owns it
- **goldmark thread safety**: Parser/renderer are reused (goldmark is thread-safe), no pooling needed

## Metadata / RenderResult

**Chosen**: `RenderResult` struct instead of multiple return values

**Evolution**:
1. Initially `Render()` returned `([]byte, string, error)` — content, MIME type, error
2. Considered `([]byte, interface{}, string, error)` — adding metadata as 2nd return
3. Considered separate `MetadataExtractor` interface — too complex
4. **Final**: `(*RenderResult, error)` with struct containing Content, MimeType, Metadata, TOC

**Why**: Extensible without interface churn. Prepares for future fields (images found, links for validation). Clean API.

## Configuration Architecture

**Chosen**: Custom implementation with reflection-based env var walking

**Config/SiteConfig separation** (security decision):
- **`Config`** (server/operational): Port, dir, dev mode — NOT exposed to templates
- **`SiteConfig`** (site/presentation): Title, theme, domain — exposed to templates safely
- Templates cannot access server internals (port, file paths, shutdown timeouts)

**Precedence models**:
- Server config: CLI flags > env vars > defaults (Kong handles natively)
- Site config: env vars > YAML file > defaults (allows environment-specific customization)

**Alternatives rejected**:
- **Viper/koanf**: Would add external deps; custom implementation is simpler for our specific needs and provides full control

**Template cache (Strategy Pattern)**:
- `CachedTemplateStore` (production): Thread-safe `sync.Map`
- `PassthroughTemplateStore` (dev): Always re-parse, no caching
- **Rejected alternatives**: Conditional in Render() (leaks config), two Renderer implementations (duplicates logic), config flag checking (couples renderer to config)

## Git Provider Security

**Key security decisions**:
- **SSH**: Fail-closed via known_hosts — no TOFU (Trust On First Use) fallback. SSH agent not supported (explicit key file only)
- **Error classification**: Uses `errors.Is()` with go-git typed sentinel errors (`transport.ErrAuthenticationRequired`, `transport.ErrRepositoryNotFound`, `plumbing.ErrReferenceNotFound`) instead of string matching. String fallbacks retained only for errors without typed sentinels.
- **Clone timeout**: Enforced via `context.WithTimeout` (default 60s)
- **File size limits**: 50MB max per file (configurable)
- **LFS**: Pointer files detected and rejected with 501 Not Implemented
- **Lazy initialization**: Clone happens on first `ReadFile()`, not at startup
- **In-memory storage**: No persistent disk cache, cleared on restart (restart = update content)

## API Design Patterns

- **Options struct** preferred over functional options (simpler, sufficient, zero-value gives sensible defaults)
- Example: `NewMarkdownRenderer(MarkdownOptions{ColorChips: true})` — empty struct gives defaults
- **Constructor pattern**: `config.NewFromServeArgs(dir, port, devMode, gitSSHKey)` replaced `Load()` + `ParseFlags()`

## Disk-Based Git Storage

**Chosen**: `DiskStorageFactory` using `go-git/v5/storage/filesystem` with LRU object cache

**Alternatives considered**:
- **Always in-memory**: Simple but OOMs on large repos (50MB+ object databases)
- **Persistent disk cache with invalidation**: More complex, requires tracking remote HEAD changes
- **SQLite-backed storage**: Would add CGO dependency

**Why filesystem.Storage**: Uses go-git's native `filesystem.Storage` with `cache.NewObjectLRUDefault()`. Zero new deps (go-billy already in go.mod). URL-hashed subdirectories under `--git-storage-dir` isolate per-repo caches. Factory pattern (`StorageFactory`) keeps memory storage as default for small repos.

## Auto-Navigation Sidebar

**Chosen**: FS-walking navigation generator with template function rendering

**Alternatives considered**:
- **Config-driven navigation** (manual YAML sidebar definition): Precise control but high maintenance burden, doesn't scale with directory changes
- **Handler-level generation** (generate in handler, pass via PageContext): Works but couples handler to navigation package
- **Client-side JS navigation** (fetch navigation JSON, render in browser): Extra HTTP request, flicker on load

**Why FS-walking + template function**: Navigation generator walks `Provider.RootFS()` to build a `NavNode` tree, marks active path, and renders via `{{ navigation .Page.Path }}` template function. Same pattern as breadcrumbs — no handler changes needed. Uses `<details>/<summary>` for collapsible directories. Title extracted from first `# heading` in each `.md` file (skipping frontmatter). Directories without renderable children are pruned.

## Metadata Indexing

**Chosen**: Lightweight YAML frontmatter parser with in-memory index + JSON API

**Alternatives considered**:
- **goldmark-meta reuse** (full markdown parse per file): Correct but 10-100x slower — parses entire markdown just to extract frontmatter
- **Database-backed index** (SQLite/bbolt): Persistent but adds complexity and deps for a read-only use case
- **No aggregation** (per-page only): Simplest but blocks tag-based discovery features

**Why lightweight parser**: Custom `extractFrontmatter()` finds `---` delimiters and calls `yaml.Unmarshal` — no goldmark needed. Index built at startup with three-phase concurrent approach: collect file paths → parse frontmatter in parallel with `errgroup` → merge results sequentially (deterministic output). Tags normalized to lowercase. API: `GET /api/tags` (all tags) and `GET /api/tags/{tag}` (pages by tag). Routes registered before the catch-all handler in `server.go`.

## Static Site Generation (`gomddoc build`)

**Chosen**: Walk-and-render approach reusing the serve pipeline

**Alternatives considered**:
- **HTTP crawling** (start server, crawl with HTTP client): Handles all edge cases but slow, complex, requires port allocation
- **Separate rendering pipeline**: Duplicates serve logic, diverges over time
- **Template-only output** (skip template wrapping, output raw HTML): Simpler but useless without styling

**Why walk-and-render**: Reuses the exact same provider → renderer → template pipeline as `serve.go`. Walks `contentRoot` with `fs.WalkDir`, renders `.md` files through the full pipeline, copies non-markdown files as-is. Generates `index.html` alongside `README.html` for clean URLs. Config reused via `NewFromServeArgs` with dummy port. Title derivation uses the shared `text.DeriveTitle` function (extracted to `internal/text/`).

## Color Chip Web Component

**Chosen**: Shadow DOM `<color-chip>` custom element, shared via `inlineAsset`

**Alternatives considered**:
- **Inline `<span>` with styles**: Simple but leaks CSS between themes; each theme must define chip styles independently
- **Server-side SVG**: Would add rendering complexity; no interactivity (click-to-copy)
- **CSS-only with `background-color`**: No click-to-copy; requires parsing hex in CSS (not possible without JS)

**Why web component**: Shadow DOM encapsulation means the chip renders identically across all 8 themes without any theme-specific CSS. The `::part(swatch)` and `::part(label)` CSS parts allow themes to customize appearance if needed. Click-to-copy with "Copied!" feedback provides utility. The component is loaded once via `{{ inlineAsset "color-chip.mjs" }}` — shared across all themes from `assets/shared/`.

**Post-processing integration**: The `transformColorChips()` function converts `<code>#HEX</code>` to `<color-chip>#HEX</color-chip>` during rendering. Only backtick-wrapped hex codes are transformed (fenced code blocks and plain text are unaffected). Controlled by `color_chips` config (default: true) with per-page frontmatter override.

## TOC Scroll Highlighting

**Chosen**: `getBoundingClientRect()` with scroll event listener (passive)

**Alternatives considered**:
- **IntersectionObserver**: Modern API but requires careful threshold tuning; doesn't naturally give "last heading above viewport" semantics
- **Scroll position + offset calculation**: Manual math with `offsetTop` — fragile with sticky headers and dynamic content

**Why getBoundingClientRect**: Simple, well-supported, directly answers "which heading last scrolled past the top?" with a single `<= 100` threshold. Passive scroll listener avoids jank. Includes fallback for TOC entries without a matching heading (walks backward to nearest ancestor heading that is in the TOC). Auto-scrolls the TOC sidebar to keep the active item centered.

## Touch Device Accessibility

**Chosen**: `@media (hover: none)` CSS media query

**Key decisions**:
- Copy buttons always visible on touch (no hover state to reveal them)
- Heading anchors always visible at reduced opacity on touch
- Applied consistently across all 8 themes

**Why**: Touch devices (phones, tablets) cannot hover. Without this, copy buttons and heading anchors are invisible and unreachable. The `hover: none` media query is well-supported (95%+ browser coverage) and cleanly separates touch from pointer interaction models.

## Theme System

**Chosen**: 8 bundled themes with full feature parity, single-file `default.html.tmpl` architecture

**Key decisions**:
- **Single-file themes**: Each theme is one `default.html.tmpl` with inline CSS/JS. Simpler than multi-file setups; entire theme is self-contained and easy to copy/customize.
- **Feature parity**: All themes must support: light/dark mode, TOC, navigation, breadcrumbs, admonitions, color chips, code copy, heading anchors, KaTeX, Mermaid, touch accessibility. Prevents "works in default theme but not in X" bugs.
- **Client-side KaTeX/Mermaid**: Loaded from jsDelivr CDN. Zero server-side deps. Theme-aware (Mermaid initializes with dark/light theme based on `data-theme` attribute).
- **`prefers-color-scheme` CSS fallback**: All themes include `@media (prefers-color-scheme: dark) { :root:not([data-theme]) { ... } }` so dark mode works even without JavaScript/localStorage.

## `inlineAsset` Template Function

**Chosen**: Template function that loads assets from theme directory with shared directory fallback

**Alternatives considered**:
- **Embed directly in each theme**: Duplicates code across 8 themes; updating means touching all themes
- **External `<script src>` URL**: Requires static asset serving infrastructure (Phase 8e); not yet available
- **Global template function with hardcoded paths**: Inflexible; can't be overridden per-theme

**Why `inlineAsset`**: Search order (theme dir → shared dir) lets themes override shared assets without forking. Returns `template.JS` for safe inline embedding. Currently used for `color-chip.mjs`. When Phase 8e (static asset serving) ships, themes can migrate to `<script src="{{ assetURL ... }}">` while keeping `inlineAsset` as a fallback for small snippets.

## Cache Busting Strategy

**Chosen**: Content-hash ETags (FNV-64a) on rendered output

**Alternatives considered**:
- **Git commit-based invalidation**: Use commit hash as cache key. Simpler mental model ("new commit = new content") but busts cache for all resources on every commit, even unmodified files. Especially problematic with PR/tag preview (Phase 10) where many commits touch few files.

**Why content hashes**: FNV-64a ETag on the rendered HTML means only genuinely changed pages are invalidated. No dependency on Git metadata at serve time. Works identically for filesystem and git providers.

## Benchmark Strategy

**Chosen**: Per-package `*_bench_test.go` files with table-driven small/medium/large document sizes

**Key decisions**:
- **`b.Loop()` over `b.N`**: Go 1.25+ `b.Loop()` eliminates common benchmark pitfalls (compiler optimization, timer management)
- **Three document sizes**: Small (~50B), medium (~2KB), large (~20KB) — covers cache-friendly and cache-busting scenarios
- **Separate bench files**: `*_bench_test.go` keeps benchmarks isolated from unit tests; `make bench` uses `-run=^$` to skip unit tests
- **`benchstat` comparison**: `make bench-save` captures baseline, `make bench-compare` detects regressions. Count=6 for statistical significance
- **No CI integration yet**: Deferred until GitHub Actions is set up

## pprof Integration

**Chosen**: `--pprof` CLI flag enabling `net/http/pprof` handlers on the existing mux

**Alternatives considered**:
- **Always-on pprof on separate port**: Avoids accidental production exposure but adds port management complexity
- **Build tag (`-tags pprof`)**: Zero overhead when disabled but complicates the build process

**Why CLI flag**: Simplest approach. Disabled by default, logs `slog.Warn` when enabled. Routes registered directly on the mux (bypass content middleware, like `/health/` and `/metrics`). Config flows through `ServerConfig.Pprof` and is overridable via `GOMDDOC_SERVER_PPROF` env var.

## Pre-Launch SEO (Phase 9a)

**Chosen**: Template functions + dedicated handlers + shared `internal/seo` package

**Key decisions**:
- **Shared URL construction**: `seo.PageURL(domain, path, defaultIndex)` in `internal/seo/url.go` used by template functions, sitemap handler, and build command. Uses `net/url` for proper URL building. Default index filename from config (not hardcoded).
- **Single head partial**: All SEO tags in `default/partials/head.html.tmpl` — inherited by all 8 themes via partial override system. No per-theme changes needed.
- **`og:type` overridable**: Defaults to `article` but frontmatter `og_type` field overrides it (e.g., `og_type: website` for landing pages).
- **Conditional tags**: All SEO tags degrade gracefully — canonical/OG URL tags only render when `meta.domain` is configured. Description falls back from page to site level.
- **Sitemap handler**: Only registered when both `MetaIndex` and `Meta.Domain` are available. Uses `metadata.Index.AllPages()` for page discovery.
- **robots.txt handler**: Always registered (useful even without domain). `Sitemap:` directive only included when domain is set.
- **Build integration**: `robots.txt` always generated. `sitemap.xml` only generated when `Meta.Domain` is configured. Uses same `GenerateSitemap`/`GenerateRobotsTxt` functions as serve handlers.

## Allocation Reduction (Phase 4)

**Goal**: Minimize heap allocations on the request hot path, targeting zero-alloc for ETag checks and content negotiation.

**Key decisions**:
- **Inline FNV-64a**: Replaced `hash/fnv` package with 3-line inline computation in `generateETag()`. Eliminates `hash.Hash` interface allocation. Combined with `strconv.AppendUint` into a `[20]byte` stack buffer (replacing `fmt.Sprintf`), reduces from 2+ to 1 allocation (the returned string).
- **Hand-rolled Accept parser**: Replaced `mime.ParseMediaType()` in `ParseAccept()` with manual semicolon/slash scanning. `mime.ParseMediaType` allocates a `map[string]string` for parameters on every call — unnecessary for Accept headers where only `q=` matters. New parser uses `strings.IndexByte` for type/subtype splitting (zero-alloc) and `strings.SplitSeq` iterator for parameters.
- **`strings.IndexByte` over `strings.Split`**: Throughout `Matches()`, `inputMatchScore()`, `outputMatchScore()`. `strings.Split` allocates a `[]string` slice; `IndexByte` returns an index for substring slicing (zero-alloc).
- **Stack-allocated candidate array**: `Registry.Get()` uses `var candidateBuf [8]candidate` instead of `var candidates []candidate`. For typical registries (≤8 renderers), avoids heap allocation entirely.
- **`NormalizeMimeType` fast-path**: Skip `mime.ParseMediaType` when input has no semicolon (common case for pre-normalized MIME types from `Provider.ReadFile`).
- **Compression buffer pool**: `sync.Pool` for `compressionWriter.buf` byte slices. Acquired on request start, returned in `Close()`. Avoids per-request 4KB buffer allocation.

**Results** (allocs/op):
| Path | Before | After |
|------|--------|-------|
| `generateETag` | 2+ | 1 |
| `checkETag` | 0 | 0 |
| `ParseAccept` (single) | 5+ | 2 |
| `ParseAccept` (browser) | 12+ | 4 |
| `Matches` | 1 | 0 |
| `Registry.Get` | 5-12 | 0 |

## Full-Text Search (Phase 5)

**Chosen: Option C — Stdlib inverted index** for `gomddoc serve` mode.

**Options analyzed:**
- **Option A — Bleve** (server-side, Go-native): Rich queries, fuzzy matching. Heavy deps (~20+ transitive), increases binary size.
- **Option B — Pagefind** (client-side, build-time): Zero server cost, tiny JS, excellent relevance. Requires `gomddoc build` first. External binary dep.
- **Option C — Stdlib inverted index**: Zero deps, simple. No fuzzy matching, basic relevance.
- **Option D — Lunr.js** (client-side): Established but aging, larger index files than Pagefind.

**Why Option C:** Fits the zero-external-deps philosophy. The index is built at startup using the same
3-phase concurrent pattern as `metadata.BuildIndex` (walk → tokenize in parallel → merge). Documents
are tokenized by stripping markdown syntax and splitting on non-alphanumeric boundaries. Ranking uses
TF-IDF with title (3x) and description (1.5x) boosts. Query semantics are AND (all terms must match).
Snippets are generated with `<mark>` highlighting around query terms.

**API:** `GET /api/search?q=<query>&limit=<n>` returns JSON array of `{path, title, description, snippet, score}`.

**Search UI:** Shared `search.mjs` module loaded via `{{ inlineAsset "search.mjs" }}` across all 8 themes.
CSS injected dynamically using theme custom properties (`--color-*`) for automatic cross-theme and dark
mode compatibility — no per-theme CSS needed. Follows the `color-chip.mjs` precedent for shared assets.

**Build mode:** Option B (Pagefind) deferred as optional post-build step.

## Deferred / Discarded Ideas

| Idea | Status | Reason |
|------|--------|--------|
| Database providers (PostgreSQL/SQLite) | Discarded | Git is the database; keeps app stateless |
| REST/GraphQL APIs | Discarded | gomddoc is a viewer, not a headless CMS |
| Editorial workflows (drafts, reviews) | Discarded | Use Git branches instead |
| i18n / multi-language | Deferred | Out of scope for current focus |
| VS Code extension | Deferred | Low priority |
| Plugin architecture / dynamic loading | Deferred | Interface-based extensibility is sufficient |
| Content versioning | Discarded | Use Git history directly |
| TOML/JSON frontmatter | Deferred | YAML is standard; others can be added later |
| Fuzz testing for MarkdownRenderer | Deferred | Recommended in spec but not yet implemented |
| Panic recovery in Handler | Deferred | Nice-to-have, not critical |
| `git+https://` with auth tokens | Deferred | Only anonymous HTTPS for now |
| CORS headers | Not planned | Not needed for doc viewer |
| Rate limiting | Not planned | Not needed for doc viewer |
| WebSocket (live reload) | Deferred | Phase 8+ |
