# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is `gomddoc`, a production-ready HTTP server that serves Markdown files as HTML. It's a single-file Go application that:

- Serves Markdown files from a directory as rendered HTML
- Uses the gomarkdown library for Markdown to HTML conversion
- Includes graceful shutdown handling with signal trapping
- Defaults to serving README.md as the index file
- **Security**: Built-in path traversal protection via `os.OpenRoot()` (Go 1.24+)
- **Security Headers**: X-Content-Type-Options and X-Frame-Options middleware
- **HTTP Compliance**: Proper status codes (404 vs 500), Content-Type headers
- **Tested**: 55%+ test coverage with comprehensive test suite
- **Production Ready**: Proper error handling and caching headers

## Development Commands

### Running the Application

```bash
make run                    # Run locally (go run main.go)
go run main.go -d /path     # Run with custom directory
go run main.go -p :9000     # Run on custom port
```

### Testing and Quality

```bash
make test                   # Run tests with coverage report (55%+ coverage)
make bench                  # Run benchmarks
make lint                   # Run comprehensive linting (format, vet, staticcheck, golangci-lint, gosec, gocritic)
make format                 # Format source code with gofmt
make build                  # Build binary to build/gomddoc
```

### Dependencies

```bash
make update-deps            # Update all dependencies and tidy mod
make clean                  # Remove build artifacts and coverage files
```

## Architecture

- **Single binary**: Everything is contained in `main.go` (~150 lines)
- **HTTP server**: Standard library HTTP server with graceful shutdown
- **Markdown processing**: Uses gomarkdown with CommonExtensions + AutoHeadingIDs
- **File serving**: Uses `os.OpenRoot()` for secure file access within the specified directory
- **Security**: Built-in path traversal protection + security headers middleware
- **Configuration**: Command-line flags for directory (`-d`) and port (`-p`)
- **Testing**: Comprehensive test suite in `main_test.go` with 55%+ coverage
- **HTTP Compliance**: Proper status codes, Content-Type, and Cache-Control headers

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

## Current Implementation Status

### ✅ **COMPLETED FEATURES**
- **HTTP Compliance**: Proper 404/500 status codes, Content-Type headers
- **Security Headers**: X-Content-Type-Options and X-Frame-Options middleware
- **Comprehensive Testing**: 55%+ coverage with unit and integration tests
- **Path Security**: Built-in protection via `os.OpenRoot()` (Go 1.24+)
- **Caching**: 5-minute Cache-Control headers for browser optimization
- **Error Handling**: Proper logging and HTTP responses
- **Graceful Shutdown**: Signal handling for clean server shutdown

### **IMPORTANT NOTES**
- **Path Traversal**: Already secured by `os.OpenRoot()` - no manual validation needed
- **Single File**: Keep architecture simple, only split if >300 lines
- **Testing**: Focus on HTTP compliance, error handling, and security boundaries
- **Performance**: Current implementation is sufficient for typical use cases

### **DEVELOPMENT WORKFLOW**
1. Make code changes in `main.go`
2. Update tests in `main_test.go` if needed
3. Run `make lint` to verify code quality
4. Run `make test` to ensure all tests pass
5. Run `make build` to create production binary
6. Commit changes with descriptive messages

The application is now production-ready with proper security, testing, and HTTP compliance.

