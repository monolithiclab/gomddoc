# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is `gomddoc`, a simple HTTP server that serves Markdown files as HTML. It's a single-file Go application that:

- Serves Markdown files from a directory as rendered HTML
- Uses the gomarkdown library for Markdown to HTML conversion
- Includes graceful shutdown handling with signal trapping
- Defaults to serving README.md as the index file

## Development Commands

### Running the Application

```bash
make run                    # Run locally (go run main.go)
go run main.go -d /path     # Run with custom directory
go run main.go -p :9000     # Run on custom port
```

### Testing and Quality

```bash
make test                   # Run tests with coverage report
make bench                  # Run benchmarks
make lint                   # Run comprehensive linting (format, vet, staticcheck, golangci-lint, gosec, gocritic)
make format                 # Format source code with gofmt
```

### Dependencies

```bash
make update-deps            # Update all dependencies and tidy mod
make clean                  # Remove build artifacts and coverage files
```

## Architecture

- **Single binary**: Everything is contained in `main.go`
- **HTTP server**: Standard library HTTP server with graceful shutdown
- **Markdown processing**: Uses gomarkdown with CommonExtensions + AutoHeadingIDs
- **File serving**: Uses `os.OpenRoot()` for secure file access within the specified directory
- **Configuration**: Command-line flags for directory (`-d`) and port (`-p`)

## Key Dependencies

- `github.com/gomarkdown/markdown`: Markdown parsing and HTML rendering
- `golang.org/x/sync/errgroup`: Graceful shutdown coordination

## Linting

The project uses extensive linting via `common-go.mk`. The `make lint` command runs:

- gofmt formatting check
- go vet
- staticcheck
- golangci-lint
- gosec (security)
- gocritic

Use `make lint -j8` to parallelize linting and `FORCE_UPDATE=1 make lint` to reinstall linters.

