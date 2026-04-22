# Agents Context

This file provides guidance to AI agents when working with code in this repository.

## Project Overview

`gomddoc` is a production-ready HTTP server that serves Markdown files as HTML. It's a **Go application** with a
stateless, git-native architecture. Strategic direction: the best git-backed documentation viewer — no databases,
no CMS, no editorial workflows.

Key documentation:

- `docs/architecture.md` — detailed architecture reference
- `docs/decisions.md` — architectural decisions log with alternatives considered
- `docs/roadmap.md` — phased roadmap (Phase 4+), deferred ideas
- `docs/guide/` — user-facing guide (10 chapters)

## Development Commands

```bash
make run                    # Run locally (go run ./cmd/gomddoc serve)
make build                  # Build binary to build/gomddoc
make install                # Install to $GOBIN (or $GOPATH/bin)
make test                   # Run tests with coverage (~78% coverage)
make bench                  # Run benchmarks
make lint                   # Comprehensive linting (gofmt, vet, staticcheck, golangci-lint, gosec, gocritic)
make format                 # Format source code
make ci                     # Run codefix, format, lint and tests
make update-deps            # Update all dependencies and tidy mod
make clean                  # Remove build artifacts and coverage files
```

Use `make lint -j8` to parallelize linting and `FORCE_UPDATE=1 make lint` to reinstall linters.

## Architecture

```
gomddoc/
├── cmd/gomddoc/           # CLI entry point (Kong), serve + build subcommands
├── internal/
│   ├── config/            # Configuration (NewFromServeArgs, validation, YAML, env overrides)
│   ├── enricher/          # Content enrichment (metadata, TOC, related docs extraction before rendering)
│   ├── metadata/          # Frontmatter indexing, tag API
│   ├── provider/          # Content providers (filesystem, git with memory/disk storage)
│   ├── negotiate/         # Content negotiation (MediaType, ParseAccept, Matches)
│   ├── renderer/          # Content renderers (markdown, markdown-passthrough, passthrough)
│   ├── template/          # Template rendering, caching, breadcrumbs, navigation
│   ├── server/            # HTTP server, handlers, middleware
│   └── assets/            # Overlay filesystem for theme overrides
├── assets/                # Embedded themes and templates
├── docs/                  # Architecture, decisions, roadmap, user guide
└── material/
    ├── themes/            # Public free themes for gomddoc
    └── public-website/    # Public website with marketing material, documentation, etc.
```

### Core patterns

- **Two-dimensional content negotiation**: renderer registry matches input MIME type + Accept header output type
- **Content negotiation** via HTTP Accept header with q-values (negotiate before rendering)
- **Options struct pattern** for constructors (e.g., `NewMarkdownRenderer(MarkdownOptions{...})`)
- **Config/SiteConfig separation**: server config not exposed to templates (security)
- **Config precedence**: CLI flags > env vars > `.gomddoc/config.yml` > defaults
- **Enricher pipeline**: enricher extracts metadata/TOC before rendering; renderers receive `*enricher.EnrichmentData`
- **Post-processing pipeline**: goldmark → heading anchors → admonitions → color chips

## Key Dependencies

- `github.com/alecthomas/kong`: CLI subcommand parsing with struct tags
- `github.com/yuin/goldmark`: Markdown parsing and HTML rendering
- `github.com/go-git/go-git/v5`: Git provider (pure Go, no CGO)
- `github.com/prometheus/client_golang`: Prometheus metrics
- `golang.org/x/sync/errgroup`: Graceful shutdown coordination

## Development Workflow

1. Make code changes in appropriate `internal/` packages
2. Maintain tests in corresponding `*_test.go` files to cover at least 80% of the code base.
   New feature must be tested as extensively as possible.
3. Run `make codefix` to upgrade to latest Go best practices
4. Run `make format lint -j8` to verify and enforce code quality
5. Run `make lint -j8` to enforce code standards
6. Run `make test` to ensure all tests pass (target: 78%+)
7. Run `make build` to create production binary

### General Conventions

- Run manual tests using Chrome devtool MCP server, use `material/testsite/` as a gomddoc directory target
- Update the documentation in `docs/` upon success.
- No backward compatibility concerns: gomddoc is unpublished yet. No legacy mode or migration shims needed.
- Keep dependencies minimal across all repos.
- Prevent duplicated code whenever possible

### Go Best Practices

- Use modern Go 1.25+ patterns
- Prefer `any` to `interface{}`
- Always use `make test` to run tests
- Table-driven tests with `[]struct{...}` test tables
- Sentinel errors with `errors.Is()` for classification
- All errors wrapped with `fmt.Errorf("context: %w", err)`
- Join path with path.Join with each folder its own parameter (eg. bad: `path.Join("assets/themes/", cfg.Theme)`; good: `path.Join("assets", "themes", cfg.Theme)`)

### Subsequent tasks

#### After implementing a new CLI feature

1. Update the marketing site content in `material/public-website` if the feature is user-facing.
2. Check if existing themes need updates in `material/themes` (new template variables, new elements to style).

#### After modifying theme capabilities

1. Check if the CLI's template system needs changes to support new theme features.
2. Update the marketing site's custom theme if it uses patterns affected by the change.

#### After changing CLI configuration options

1. Update `material/public-website/docs/configuration.md` to reflect new options.
2. Update theme READMEs in `material/themes/` if themes expose new config.
