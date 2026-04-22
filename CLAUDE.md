# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is `gomddoc`, a production-ready HTTP server that serves Markdown files as HTML.
It's a **restructured Go application** with clean architecture that:

- Serves Markdown files from a directory as rendered HTML
- Uses the gomarkdown library for Markdown to HTML conversion
- Includes graceful shutdown handling with signal trapping
- Defaults to serving README.md as the index file
- **Security**: Built-in path traversal protection via `os.OpenRoot()` (Go 1.24+)
- **Security Headers**: X-Content-Type-Options and X-Frame-Options middleware
- **HTTP Compliance**: Proper status codes (404 vs 500), Content-Type headers
- **Tested**: 57.5% test coverage with comprehensive test suite
- **Production Ready**: Proper error handling and caching headers
- **Interface-Driven**: Clean interfaces for extensibility and testing

## Development Commands

### Running the Application

```bash
make run                           # Run locally (go run ./cmd/gomddoc)
go run ./cmd/gomddoc -d /path      # Run with custom directory
go run ./cmd/gomddoc -p :9000      # Run on custom port
./build/gomddoc                    # Run built binary
```

### Testing and Quality

```bash
make test                   # Run tests with coverage report (57.5% coverage)
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

### **Current Structure (Phase 1 - Interface-Driven)**

```
gomddoc/
├── cmd/gomddoc/           # CLI entry point and main()
├── internal/
│   ├── config/           # Configuration management
│   ├── provider/         # Content providers (filesystem)
│   ├── processor/        # Document processors (markdown)
│   ├── template/         # Template rendering
│   └── server/           # HTTP server, handlers, middleware
├── assets/               # Embedded themes and templates
└── docs/                 # Architecture documentation
```

- **Interface-Driven**: Clean interfaces for Provider, Processor, Renderer, Server
- **HTTP server**: Standard library HTTP server with graceful shutdown
- **Markdown processing**: Uses gomarkdown with CommonExtensions + AutoHeadingIDs
- **File serving**: Uses `os.OpenRoot()` for secure file access within the specified directory
- **Security**: Built-in path traversal protection + security headers middleware
- **Configuration**: Command-line flags for directory (`-d`) and port (`-p`)
- **Testing**: Comprehensive test suite across all packages with 57.5% coverage
- **HTTP Compliance**: Proper status codes, Content-Type, and Cache-Control headers
- **Extensibility**: Ready for multiple providers, processors, and advanced features

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

### ✅ **PHASE 1 COMPLETED: Interface-Driven Architecture**

- **Project Structure**: Migrated from single file to proper Go project structure
- **Interface Design**: Clean interfaces for Provider, Processor, Renderer, Server
- **HTTP Compliance**: Proper 404/500 status codes, Content-Type headers
- **Security Headers**: X-Content-Type-Options and X-Frame-Options middleware
- **Comprehensive Testing**: 57.5% coverage with unit and integration tests (improved from 54.9%)
- **Path Security**: Built-in protection via `os.OpenRoot()` (Go 1.24+)
- **Caching**: 5-minute Cache-Control headers for browser optimization
- **Error Handling**: Proper logging and HTTP responses
- **Graceful Shutdown**: Signal handling for clean server shutdown
- **Backward Compatibility**: 100% identical behavior to single-file implementation

### **ARCHITECTURE BENEFITS**

- **Testability**: Each component can be unit tested in isolation
- **Maintainability**: Clear separation of concerns across packages
- **Extensibility**: Interface-based design ready for Phase 2+ features
- **Security**: All security features preserved and enhanced
- **Performance**: No performance regression, potential for optimization

### **DEVELOPMENT WORKFLOW**

1. Make code changes in appropriate `internal/` packages
2. Update tests in corresponding `*_test.go` files
3. Run `make format` to verify code quality
4. Run `make lint` to verify code quality
5. Run `make test` to ensure all tests pass (target: 57.5%+)
6. Run `make build` to create production binary
7. Commit changes with descriptive messages

### **NEXT PHASES READY**

The restructured architecture enables Phase 2+ features from `PLAN.md`:

- **Multiple Content Providers**: Database, GitHub, custom sources
- **Multiple Document Processors**: AsciiDoc, RST, Jupyter notebooks
- **Advanced Template System**: Multiple themes, i18n support
- **API Integration**: REST/GraphQL APIs for headless CMS functionality
- **Plugin Architecture**: Dynamic extension loading and custom processors

The application is production-ready with clean architecture, comprehensive testing, and full extensibility.

## Go best practices

- Use modern Golang 1.25+ patterns
- Prefer `any` to `interface{}`
- Always use `make test` to run tests