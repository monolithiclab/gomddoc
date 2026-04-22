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
- **No context in Provider**: Maintains stdlib `fs.FS` compatibility; timeout enforced at HTTP level
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

**Why lightweight parser**: Custom `extractFrontmatter()` finds `---` delimiters and calls `yaml.Unmarshal` — no goldmark needed. Index built at startup by walking `RootFS()`. Tags normalized to lowercase. API: `GET /api/tags` (all tags) and `GET /api/tags/{tag}` (pages by tag). Routes registered before the catch-all handler in `server.go`.

## Static Site Generation (`gomddoc build`)

**Chosen**: Walk-and-render approach reusing the serve pipeline

**Alternatives considered**:
- **HTTP crawling** (start server, crawl with HTTP client): Handles all edge cases but slow, complex, requires port allocation
- **Separate rendering pipeline**: Duplicates serve logic, diverges over time
- **Template-only output** (skip template wrapping, output raw HTML): Simpler but useless without styling

**Why walk-and-render**: Reuses the exact same provider → renderer → template pipeline as `serve.go`. Walks `contentRoot` with `fs.WalkDir`, renders `.md` files through the full pipeline, copies non-markdown files as-is. Generates `index.html` alongside `README.html` for clean URLs. Config reused via `NewFromServeArgs` with dummy port. Trade-off: `deriveTitle()` duplicated from `server/handler.go` (unexported) — acceptable for 10 lines vs adding a shared package.

## Full-Text Search (Decision Pending)

**Options analyzed** (not yet implemented):
- **Option A — Bleve** (server-side, Go-native): Rich queries, fuzzy matching. Heavy deps (~20+ transitive), increases binary size.
- **Option B — Pagefind** (client-side, build-time): Zero server cost, tiny JS, excellent relevance. Requires `gomddoc build` first. External binary dep.
- **Option C — Stdlib inverted index**: Zero deps, simple. No fuzzy matching, basic relevance.
- **Option D — Lunr.js** (client-side): Established but aging, larger index files than Pagefind.

**Recommendation**: Option C for `gomddoc serve` (keeps zero-external-deps philosophy), Option B for `gomddoc build` (as optional post-build step).

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
