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
