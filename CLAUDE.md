# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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
├── cmd/gomddoc/           # CLI entry point (Kong), serve subcommand
├── internal/
│   ├── config/            # Configuration (NewFromServeArgs, validation, YAML, env overrides)
│   ├── provider/          # Content providers (filesystem, git)
│   ├── renderer/          # Content renderers (markdown, passthrough)
│   ├── template/          # Template rendering, caching, breadcrumbs
│   ├── server/            # HTTP server, handlers, middleware, content negotiation
│   └── assets/            # Overlay filesystem for theme overrides
├── assets/                # Embedded themes and templates
└── docs/                  # Architecture, decisions, roadmap, user guide
```

Core patterns:
- **MIME-type based routing** with renderer registry (exact > type/* > */*)
- **Content negotiation** via HTTP Accept header with q-values
- **Options struct pattern** for constructors (e.g., `NewMarkdownRenderer(MarkdownOptions{...})`)
- **Config/SiteConfig separation**: server config not exposed to templates (security)
- **Config precedence**: CLI flags > env vars > `.gomddoc/config.yml` > defaults
- **Post-processing pipeline**: goldmark → heading anchors → admonitions → color chips

## Key Dependencies

- `github.com/alecthomas/kong`: CLI subcommand parsing with struct tags
- `github.com/yuin/goldmark`: Markdown parsing and HTML rendering
- `github.com/go-git/go-git/v5`: Git provider (pure Go, no CGO)
- `github.com/prometheus/client_golang`: Prometheus metrics
- `golang.org/x/sync/errgroup`: Graceful shutdown coordination

## Development Workflow

1. Make code changes in appropriate `internal/` packages
2. Update tests in corresponding `*_test.go` files
3. Run `make codefix` to upgrade to latest Go best practices
4. Run `make format` then `make lint` to verify code quality
5. Run `make test` to ensure all tests pass (target: 78%+)
6. Run `make build` to create production binary

## Go Best Practices

- Use modern Go 1.25+ patterns
- Prefer `any` to `interface{}`
- Always use `make test` to run tests
- Table-driven tests with `[]struct{...}` test tables
- Sentinel errors with `errors.Is()` for classification
- All errors wrapped with `fmt.Errorf("context: %w", err)`
