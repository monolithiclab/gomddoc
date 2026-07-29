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

## Feature Toggle System

**Chosen**: Generic `map[string]bool` with default-to-true semantics and three-layer override (config → env → frontmatter)

**Alternatives considered**:
- **Individual boolean fields** (`ColorChips bool`, `HasSearch bool`): Initial implementation. Each new feature required a struct field, config tag, env tag, template check, and renderer wiring. Doesn't scale.
- **Bitfield/enum**: Compile-time checked but can't add features from config/frontmatter without code changes.
- **String set (`map[string]struct{}`)**: Can't distinguish "disabled" from "not configured". No natural default-to-true.

**Why `map[string]bool`**: Arbitrary features without code changes. `nil` map = all enabled (safe default). Per-feature env vars via `GOMDDOC_SITE_FEATURES_<NAME>=bool`. Per-page frontmatter override via `features:` map. Pre-merged at `PageContext` construction time — templates call `.Feature "name"` without re-merging. Feature keys validated with `^[a-z][a-z0-9_]*$` regex.

**Key decisions**:
- **Default-to-true**: Unknown features return `true`. This means adding a new feature guard to a template doesn't break existing sites — it only takes effect when explicitly disabled. Requires no config migration.
- **Pre-merged on PageContext**: Site defaults + page overrides are merged once at `TemplateContext` construction (in handler/build), not on every `.Feature` call. Uses `maps.Clone` to avoid mutating the site config.
- **Env var pattern**: `GOMDDOC_SITE_FEATURES_KATEX=false` uses reflection-based `walkStruct` extended with `reflect.Map` handling for `map[string]bool` types.
- **Renderer gating**: Goldmark extensions (heading_anchors, admonitions, color_chips) check the merged feature map via parser context (AST transformers) or document attribute (node renderers). The renderer sets merged features on both channels before parsing/rendering.

## API Design Patterns

- **Options struct** preferred over functional options (simpler, sufficient, zero-value gives sensible defaults)
- Example: `NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": true}})` — empty struct gives defaults
- **Constructor pattern**: `config.NewFromServeArgs(ServeArgs{...})` replaced `Load()` + `ParseFlags()`

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

**Chosen**: Shadow DOM `<color-chip>` custom element, shared via `inlineJSAsset`

**Alternatives considered**:
- **Inline `<span>` with styles**: Simple but leaks CSS between themes; each theme must define chip styles independently
- **Server-side SVG**: Would add rendering complexity; no interactivity (click-to-copy)
- **CSS-only with `background-color`**: No click-to-copy; requires parsing hex in CSS (not possible without JS)

**Why web component**: Shadow DOM encapsulation means the chip renders identically across all 8 themes without any theme-specific CSS. The `::part(swatch)` and `::part(label)` CSS parts allow themes to customize appearance if needed. Click-to-copy with "Copied!" feedback provides utility. The component is loaded once via `{{ inlineJSAsset "color-chip.mjs" }}` — shared across all themes from `assets/shared/`.

**Pipeline integration**: The `ColorChipExtension` goldmark extension detects hex color codes in `ast.CodeSpan` nodes during AST transformation and replaces them with `ColorChipNode` custom nodes, rendered as `<color-chip>#HEX</color-chip>`. Only backtick-wrapped hex codes are transformed (fenced code blocks and plain text are unaffected). Controlled by the `color_chips` feature toggle (default: enabled) with per-page frontmatter override via `features: { color_chips: false }`.

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

## Inline Asset Template Functions

**Chosen**: Three typed template functions (`inlineJSAsset`, `inlineCSSAsset`, `inlineHTMLAsset`) that load assets from theme directory with shared directory fallback

**Alternatives considered**:
- **Embed directly in each theme**: Duplicates code across 8 themes; updating means touching all themes
- **External `<script src>` URL**: Requires static asset serving infrastructure (Phase 8e); not yet available
- **Global template function with hardcoded paths**: Inflexible; can't be overridden per-theme
- **Single `inlineAsset` returning one type**: Go's `html/template` applies context-aware escaping — `template.JS` works in `<script>` but is escaped in `<style>` or bare HTML; `template.HTML` works bare but is quoted/escaped inside `<script>`. No single type works in all contexts.

**Why three typed functions**: Search order (theme dir → shared dir) lets themes override shared assets without forking. Each function returns the correct `html/template` safe type for its context: `template.JS` for `<script>`, `template.CSS` for `<style>`, `template.HTML` for bare HTML (e.g. inline SVGs). Asset file reading is shared via an unexported `readAsset` method. When Phase 8e (static asset serving) ships, themes can migrate to `<script src="{{ assetURL ... }}">` while keeping inline asset functions as a fallback for small snippets.

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

**Why CLI flag**: Simplest approach. Disabled by default, logs `slog.Warn` when enabled. Routes registered via the auth `RouteGroup` — protected by BasicAuth when configured. Config flows through `ServerConfig.Pprof` and is overridable via `GOMDDOC_SERVER_PPROF` env var.

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

**Search UI:** Shared `search.mjs` module loaded via `{{ inlineJSAsset "search.mjs" }}` across all 8 themes.
CSS injected dynamically using theme custom properties (`--color-*`) for automatic cross-theme and dark
mode compatibility — no per-theme CSS needed. Follows the `color-chip.mjs` precedent for shared assets.

**Build mode:** Option B (Pagefind) deferred as optional post-build step.

## Search Index: Field-Tagged Postings

**Chosen**: Unified inverted index with field-tagged postings (`fieldBody`, `fieldTitle`, `fieldDesc`)

**Previous approach**: Hybrid index — inverted index for body content, linear `strings.Contains` on pre-lowered `titleLower`/`descLower` fields for title/description matching. `avgDL` computed but unused. IDF recomputed per candidate per token.

**Alternatives considered**:
- **Three separate inverted indexes** (one per field): Clean separation but triples lookup cost per query token (3 map lookups instead of 1). Complicates AND intersection across fields — a document matching "deploy" in title and "guide" in body requires cross-index merging. More memory overhead from three map structures.
- **Composite weighted frequencies** (pre-blend field weights into a single score per term per doc): Single lookup but loses per-field scoring flexibility. Changing boost factors requires full reindex. Can't distinguish which field matched for result presentation.

**Why field-tagged postings**: Single map lookup per query token. Postings carry a `field` tag, so scoring partitions by field naturally with a `switch`. Title/description go through the same `tokenizeToFreqs` pipeline as body — no separate lowering or substring matching. `document` struct drops `titleLower`, `descLower`, `termFreqs`, `totalTerms`. `Index` drops `avgDL`, adds `docTermCounts` for per-document body TF normalization. IDF precomputed once per query token in a `tokenInfo` struct. Boost factors (3x title, 1.5x description) adjustable without reindexing.

## Systematic `t.Parallel()` Adoption

**Chosen**: Add `t.Parallel()` to every test function and subtest unless incompatible

**Incompatible patterns** (correctly excluded):
- `t.Setenv` — mutates process environment, panics if test is parallel
- `testing.AllocsPerRun` — panics in parallel tests (Go runtime restriction)

**Scope**: config, server, renderer, enricher, negotiate, text, template/breadcrumb, cmd/gomddoc —
all packages that had gaps identified in the 8th review pass.

**Why**: Parallel tests catch shared-state bugs at test time rather than production, and reduce total
test suite wall-clock time. The Go testing framework enforces that `t.Setenv` and `t.Parallel` are
mutually exclusive at runtime, so the exclusion is safe by construction.

## MCP Server (Phase 10)

**Chosen**: Official Go MCP SDK (`github.com/modelcontextprotocol/go-sdk` v1.4.x) with thin adapter architecture

**Alternatives considered**:
- **mcp-go** (`github.com/mark3labs/mcp-go`): Larger community (8.5k stars) but v0.46.0 (unstable semver), functional options pattern, manual schema builders
- **Custom JSON-RPC implementation**: No deps but high effort to track MCP spec changes
- **HTTP-only API**: Simpler but MCP is purpose-built for AI tool use with auto-discovery

**Why official SDK**: Semver stable (v1.4.x), auto-generates JSON Schema from Go struct tags (matches gomddoc conventions), maintained by Anthropic + Google, will track spec authoritatively. Struct-based options align with gomddoc's Options struct pattern.

**Key design decisions**:
- **Thin adapter**: MCP package calls existing `Provider.ReadFile()`, `MetaIndex.AllPages()`, `SearchIndex.Search()`, and `navigation.Generator.Generate()` — no new parsing or indexing
- **`docs://` URI scheme**: Semantic resource identification separate from HTTP URLs
- **Section extraction without goldmark**: Line-based heading parser keeps MCP package lightweight with no dependency on the rendering pipeline
- **All tools `readOnlyHint: true`**: Trust signal for MCP clients to enable auto-approval
- **Dual transport**: stdio for local clients (Claude Desktop, Cursor), Streamable HTTP for remote access

**Discarded: MCP for static sites (Phase 10c)**:
Originally planned `gomddoc mcp --built-dir` to serve MCP from `gomddoc build` output, plus a build-time `_mcp/manifest.json` manifest. Dropped because `gomddoc mcp` already works with any content directory — running it against the source markdown provides richer metadata (frontmatter, tags) than post-build HTML. The manifest adds build complexity for a use case already covered by the existing command.

## URL Extension Stripping

**Chosen**: Extensionless canonical URLs with a resolution table built at startup

**Context**: Clean URLs (`/guide/setup` instead of `/guide/setup.md`) are standard practice for web applications. They improve UX (shorter, more memorable URLs), follow REST semantics (resources identified without implementation detail leaking), and benefit SEO (search engines treat extensionless URLs as the canonical form). For a documentation server, clean URLs also mean that content can migrate between formats (e.g., `.md` to `.rst`) without breaking external links.

**Alternatives considered**:
- **Handler-level rewriting** (middleware probes provider for each request): The middleware would strip the extension and try `provider.ReadFile()` with the new path. Simple but performs a double lookup on every request — first the original path fails, then the rewritten path is tried. No collision detection. No way to use clean paths in sitemaps/feeds without duplicating the logic.
- **Provider-level resolution** (provider maps paths internally): The provider would accept both `guide.md` and `guide` and resolve internally. Rejected because it muddies the provider's responsibility — providers are content sources, not URL routers. Also prevents sharing the resolution table with other components (sitemap, feed, canonical URL).
- **Resolution table** (chosen): `resolve.Build()` walks `fs.FS` at startup and creates an immutable `PathResolver` with two maps (`toReal` and `toClean`). O(1) lookup in both directions. Collision detection runs at build time with logged warnings. The resolver is shared by the handler, sitemap, feed, canonical URL, redirect map, and build command — single source of truth for path mapping.

**Trade-offs**: Memory for the two maps is negligible for documentation sites (one map entry per file with a strippable extension). The resolver must be rebuilt when content changes (e.g., after a git pull), but this already happens via provider reinitialization.

**Collision strategy**:
- **File vs. directory**: File wins. If `guide.md` and `guide/` both exist, `/guide` resolves to the file. A warning is logged at startup.
- **Multi-extension conflicts**: The first extension in the `strip_extensions` config list wins. If both `guide.md` and `guide.html` exist and both extensions are strippable, the one whose extension appears first in the config claims `/guide`. The other is skipped with a warning.

## Post-Processors to Goldmark Extensions

**Chosen**: Replace regex-based HTML post-processors with proper goldmark AST extensions

**Previous approach**: Three functions (`addHeadingAnchors`, `transformAdmonitions`, `transformColorChips`) ran sequentially on rendered HTML bytes using regex replacement. Feature flags were checked at the `Render()` call site.

**Alternatives considered**:
- **Renderer-only extensions** (override `NodeRenderer` for existing node types): Less code but mixes detection logic into rendering, can't represent admonitions/color chips in the AST
- **Hybrid** (AST transform for admonitions, renderer-only for anchors/chips): Right-sized complexity but inconsistent patterns

**Why full AST extensions**: Each feature represents different semantic content. Modeling them as AST nodes (or renderer overrides for headings) keeps the architecture clean. Extensions compose naturally with other goldmark extensions. AST-level transforms are testable independently of HTML output.

**Key design decisions**:
- **Two-channel feature flags**: Parser context key for AST transformers (which have `parser.Context`), document attribute for node renderers (which only have `ast.Node`). Both set by `MarkdownRenderer.Render()` before parsing/rendering.
- **Always-registered extensions**: All three extensions are registered on the goldmark instance. Disabled extensions skip transformation at runtime, leaving original AST nodes to render with goldmark's defaults.
- **No custom node for heading anchors**: Headings don't change structurally — only the HTML output gains an anchor link. A renderer override for `ast.KindHeading` is sufficient.
- **Custom nodes for admonitions/color chips**: `AdmonitionNode` (block) and `ColorChipNode` (inline) replace `ast.Blockquote` and `ast.CodeSpan` respectively. This makes the semantic change visible in the AST.

## HTML-in-Go to Web Components

**Chosen**: Replace hardcoded HTML in goldmark renderers with `<gmd-*>` web components

**Previous approach**: Goldmark node renderers directly emitted HTML markup (`<div class="admonition">`, `<a class="heading-anchor">`), making it impossible for themes to customize the rendered output.

**Alternatives considered**:
- **Template partials for goldmark output**: Pre-render Go `html/template` partials and inject the HTML strings into goldmark renderers. Would couple the renderer package to the template package and require passing a template executor through the goldmark pipeline.
- **CSS-only customization**: Keep HTML structure but expose CSS custom properties. Limits customization to styling — themes can't change structure, add icons, or alter behavior.

**Why web components**: Goldmark stays concerned with structure (emitting semantic custom elements), while themes control presentation via overridable JS files. Web components are a web standard, require no build tools, and can be overridden per-theme by providing a replacement `.mjs` file.

**Key design decisions**:
- **Light DOM for admonitions**: Theme CSS must apply to inner content (paragraphs, code blocks, lists). Shadow DOM would require extensive `::part()` exposure.
- **Shadow DOM for heading anchors and color chips**: Self-contained elements with no inner content from markdown. Encapsulation prevents style leakage.
- **`gmd-` prefix**: Namespaces all custom elements to avoid collisions. Applied retroactively to `<color-chip>` → `<gmd-color-chip>`.

## `cases.Title` Per-Call Allocation

**Chosen**: Create a fresh `cases.Title(language.English)` at every call site

**Alternative considered**:
- **Package-level `var caser = cases.Title(language.English)`**: Avoids per-call allocation but `cases.Caser` holds mutable internal state (`x/text/transform` writes to internal buffers on every `.String()` call). Confirmed not goroutine-safe by `-race` detector.

**Why per-call**: The allocation is negligible. A `sync.Pool` of casers could be used if this becomes a hot path, but benchmarks show no need.

## Deferred / Discarded Ideas

| Idea | Status | Reason |
|------|--------|--------|
| Database providers (PostgreSQL/SQLite) | Discarded | Git is the database; keeps app stateless |
| REST/GraphQL APIs | Discarded | gomddoc is a viewer, not a headless CMS |
| Editorial workflows (drafts, reviews) | Discarded | Use Git branches instead |
| VS Code extension | Deferred | Low priority |
| Plugin architecture / dynamic loading | Deferred | Interface-based extensibility is sufficient |
| Content versioning | Discarded | Use Git history directly |
| BOM (byte order mark) stripping | Discarded | Unnecessary edge case; not worth the complexity |
| TOML/JSON frontmatter | Deferred | YAML is standard; others can be added later |
| Fuzz testing for MarkdownRenderer | Deferred | Recommended in spec but not yet implemented |
| Panic recovery in Handler | Deferred | Nice-to-have, not critical |
| `git+https://` with auth tokens | Deferred | Only anonymous HTTPS for now |
| CORS headers | Not planned | Not needed for doc viewer |
| Rate limiting | Not planned | Not needed for doc viewer |
| WebSocket (live reload) | Deferred | Phase 8+ |

## Tag URL Format & Pages

**Chosen**: percent-encoded tag values in URLs (`/tags/machine%20learning`), centralized validator skipping invalid tags at index-build time

**Alternatives considered**:
- **Slugified URLs (kebab-case)**: shorter and prettier, but introduces a slug→tag reverse map and invents collisions where none exist in the source data. Rejected.
- **Single global `/tags/` (no per-lang)**: simpler routing but mixes languages and breaks the per-lang navigation model. Rejected — per-lang routing matches the existing sitemap/feed pattern.

**Why percent-encoding**: tag values already exist in frontmatter as plain strings; encoding rather than slugifying preserves them exactly. Pathological tag values containing `/` or whitespace-only entries are skipped at `metadata.Index` build time with a logged warning, so the encoding stays simple and the bad-data surface is closed at the source.

**Related-pages and `tag:` search**: originally deferred to follow-up sub-specs to keep the first PR focused on the user-visible discovery features (chips + landing pages). Both have since shipped — see-also related pages via the enricher, and `tag:` filter syntax in full-text search.

**Renderer interface extension**: `RenderTagPage` and `RenderTagsIndex` were added to the `template.Renderer` interface (not just the `*HTMLRenderer` concrete type) so the server can register handlers via the abstract dependency without type-asserting. There's only one renderer implementation today; the interface is a layering signal more than a polymorphism enabler.

## Version String Resolution

**Chosen**: `-ldflags -X main.version` as the primary source, with `debug.ReadBuildInfo().Main.Version` as a fallback, normalized to the unprefixed form.

**Why the fallback**: ldflags are applied by GoReleaser and `make build` — but *not* by `go install github.com/monolithiclab/gomddoc/cmd/gomddoc@latest`, which is a documented install path. v0.1.1 installed that way reported `dev`. The Go toolchain already stamps the resolved module version into the binary's build info (`mod github.com/monolithiclab/gomddoc v0.1.1`), so the correct value is present with no build-system changes.

**Alternatives considered**:
- **Drop ldflags, use build info only**: build info reports `(devel)` for anything built from a working tree, so `make build` would lose its `git describe` value. Rejected.
- **Resolve lazily at each call site**: `version` has four consumers (`--version`, `info`, the generator meta tag, the MCP handshake). A single `init()` keeps them consistent.

**Constraint worth remembering**: `var version = "dev"` must stay initialized to a *constant string expression*. `-X` is documented to only take effect on variables declared uninitialized or initialized to a constant — `var version = resolve(...)` would silently defeat the linker. That is why resolution happens in `init()` rather than in the initializer.

**Normalization**: GoReleaser injects `{{ .Version }}` (unprefixed, `0.1.1`), the Makefile injects `git describe` (prefixed, `v0.1.1-2-gabc1234`), and build info carries module versions (`v0.1.1`). All three are trimmed to the unprefixed form so `--version` output matches the archive names and docker tags.

## Serialised Git Reads

**Chosen**: one exclusive `sync.Mutex` (`gitTreeState`) guarding every post-clone read of the git object graph — tree lookups, blob contents, and directory listings, for both `GitProvider` and the `gitTreeFS` handles it hands out.

**Why it is not a tuning choice**: go-git is not safe for concurrent reads. `object.Tree` memoises lookups by writing unsynchronised maps (`FindEntry` writes `t.t`, `entry` writes `t.m`), so `Tree.File`/`Tree.Tree` are *writes* to the receiver. The storers mutate on read too, and `filesystem.ObjectStorage` shares one packfile handle it seeks on. Two concurrent readers produce a concurrent map write, which Go turns into an unrecoverable `throw()` — not a panic any middleware could catch. The previous code served reads under an `RWMutex` read lock and, worse, guarded one shared tree with *two* different locks (`GitProvider.mu` and `gitFSState.mu`).

**Alternatives considered**:
- **Stop sharing the tree; re-derive `commit.Tree()` per request**: tested under `-race` against a packed disk-backed repo. It still races, and *earlier* — in `filesystem.ObjectStorage.EncodedObject`, before a tree is even decoded. It is also strictly more expensive: a fresh root-tree decode per request, and it throws away `FindEntry`'s subtree cache so nested paths re-decode every intermediate. Rejected.
- **Pre-warm the tree caches at clone time so lookups become read-only**: `FindEntry` still writes `t.t` for two-segment paths regardless of what is cached. Rejected as unsound.
- **Finer-grained locking (e.g. release before reading blob contents)**: the blob is arguably detached from the tree by the time `Tree.File` returns, but that depends on go-git internals that vary by storer and version. Not worth betting server stability on for a memcpy. Revisit only with a benchmark.

**Consequence to remember**: git-backed sites now serve reads one at a time, including blob decompression. That is a real throughput reduction versus the (incorrect) previous behaviour. The scaling lever is independent repo handles, *not* a finer lock.
