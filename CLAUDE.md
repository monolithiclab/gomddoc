# Agents Context

This file provides guidance to AI agents when working with code in this repository.

## Project Overview

`gomddoc` is a production-ready HTTP server that serves Markdown files as HTML. Go application
with a stateless, git-native architecture. No databases, no CMS, no editorial workflows.

**Key docs** (read these for deep context, don't duplicate their content here):

- `docs/architecture.md` — architecture reference (components, data flow, middleware, themes)
- `docs/decisions.md` — architectural decisions log with alternatives considered
- `docs/roadmap.md` — phased roadmap, deferred ideas
- `docs/guide/` — user-facing guide (12 chapters)

## Commands

```bash
make ci                     # Full pipeline: codefix + format + lint + tests (USE THIS)
make test                   # Tests with coverage (target: 87%+)
make lint -j8               # Parallelize linting
make build                  # Production binary → build/gomddoc
make bench                  # Benchmarks
make run                    # Run locally (go run ./cmd/gomddoc serve)
FORCE_UPDATE=1 make lint    # Reinstall linters
```

Always use Makefile targets. `make ci` is the single command to validate changes.

## Project Structure

```
cmd/gomddoc/           # CLI (Kong): serve, build, preview, init subcommands
internal/
├── config/            # Config loading (CLI > env > YAML > defaults), validation
├── enricher/          # Pre-rendering extraction (metadata, TOC, navigation, related docs)
├── metadata/          # Frontmatter indexing, tag API
├── negotiate/         # HTTP content negotiation (Accept header, MIME types)
├── provider/          # Content sources (filesystem, git, overlay)
├── renderer/          # Content renderers (markdown→HTML, passthrough)
├── search/            # Full-text search (inverted index, TF-IDF ranking)
├── server/            # HTTP server, handlers, middleware, RouteGroup
├── template/          # HTML rendering, caching, breadcrumbs, navigation
├── assets/            # Overlay filesystem for theme overrides
└── text/              # Text utilities (sanitize, title case)
material/
├── themes/            # Public free themes (8 bundled)
└── public-website/    # Marketing site
```

## Development Workflow

1. Make minimal, focused code changes
2. Write/update tests (`*_test.go`) — target 87%+ coverage
3. Run `make ci` — must pass before drafting commit
4. Update relevant documentation in `docs/`
5. Draft and print the Git commit message (let user commit manually)

### Conventions

- **No backward compatibility concerns**: gomddoc is unpublished. No legacy shims.
- **Minimal dependencies** across all repos
- **Prevent duplicated code** — extract shared helpers
- **Manual testing**: Use Chrome DevTools MCP, target `material/testsite/`
- **Options struct pattern** for constructors (not functional options)
- **`path` not `filepath`** for `fs.FS` operations (forward slashes per `io/fs` spec)
- **`filepath`** only for OS filesystem operations (writing files to disk)

### Go Conventions

- Modern Go 1.25+ patterns (`slices.Clone`, `strings.SplitSeq`, `b.Loop()`)
- `any` over `interface{}`
- Table-driven tests: `[]struct{...}` with `t.Parallel()`
- Sentinel errors with `errors.Is()` for classification
- Errors wrapped: `fmt.Errorf("context: %w", err)`
- Path joining: `path.Join("assets", "themes", cfg.Theme)` (each segment separate)

### After implementing changes

#### New CLI feature
1. Update `material/public-website` if user-facing
2. Check if themes in `material/themes` need updates

#### Theme capability change
1. Check if template system needs changes
2. Update marketing site's custom theme if affected

#### Configuration change
1. Update `material/public-website/docs/configuration.md`
2. Update theme READMEs in `material/themes/` if relevant
