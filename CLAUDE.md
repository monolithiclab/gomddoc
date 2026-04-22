# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is `gomddoc`, a production-ready HTTP server that serves Markdown files as HTML.
It's a **Go application** with clean architecture that:

- Serves Markdown files from a directory as rendered HTML
- Uses the gomarkdown library for Markdown to HTML conversion
- Includes graceful shutdown handling with signal trapping
- Defaults to serving README.md as the index file
- **Security**: Built-in path traversal protection via `os.OpenRoot()` (Go 1.24+)
- **Security Headers**: X-Content-Type-Options and X-Frame-Options middleware
- **HTTP Compliance**: Proper status codes (404 vs 500), Content-Type headers
- **Tested**: high test coverage with comprehensive test suite
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
make test                   # Run tests with coverage report (78.9% coverage)
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

### **Current Structure**

```
gomddoc/
├── cmd/gomddoc/           # CLI entry point and main()
├── internal/
│   ├── config/           # Configuration management (ServerConfig, SiteConfig)
│   ├── provider/         # Content providers (filesystem with MIME detection)
│   ├── renderer/         # Content renderers (markdown, passthrough, custom)
│   ├── template/         # Template rendering and caching
│   └── server/           # HTTP server, handlers, middleware, content negotiation
├── assets/               # Embedded themes and templates
└── docs/                 # Architecture and custom renderer documentation
```

- **MIME-Type Based Routing**: Universal content handling via standard MIME types
- **Renderer Registry**: Self-declaring renderers with wildcard matching
- **Content Negotiation**: HTTP Accept header support with proper 406 responses
- **Directory Listing**: Optional generation with secure defaults (DirIndex=false)
- **HTTP server**: Standard library HTTP server with graceful shutdown
- **File serving**: Uses `os.DirFS()` for secure file access within specified directory
- **Security**: Path traversal protection, hidden file blocking, secure defaults
- **Configuration**: CLI flags, environment variables, YAML config file support
- **Testing**: Comprehensive test suite with integration tests (78.9% coverage)
- **HTTP Compliance**: Proper status codes, Content-Type, Cache-Control, Content-Length headers
- **Extensibility**: Add custom renderers with ~10 lines of code

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

### ✅ **PHASE 2 COMPLETED: Content Rendering System**

- **Universal Content Serving**: Markdown, HTML, CSS, JS, images, and all file types
- **MIME-Type Based Routing**: Renderer registry with wildcard matching (exact → type/_ → _/\*)
- **Content Negotiation**: HTTP Accept header parsing with q-values and stable sort
- **Directory Listing**: Optional generation with README.md fallback (DirIndex configurable)
- **Custom Renderer Support**: Extensible architecture for adding content transformations
- **Built-in Renderers**: MarkdownRenderer (md→html) and PassthroughRenderer (*/*→same)
- **MIME Normalization**: Charset stripping for routing, preservation for HTTP headers
- **Error Classification**: Sentinel errors with proper HTTP status codes (403/404/406/499/504)
- **Context Cancellation**: All renderers support request timeout and cancellation
- **Comprehensive Testing**: 78.9% coverage with integration tests (improved from 57.5%)
- **Thread Safety**: Stateless renderers with concurrent registry access

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
5. Run `make test` to ensure all tests pass (target: 78.9%+)
6. Run `make build` to create production binary
7. Commit changes with descriptive messages

### **NEXT PHASES READY**

The renderer architecture enables Phase 3+ features:

- **Multiple Content Providers**: S3, database, GitHub API, custom sources
- **Additional Renderers**: AsciiDoc, RST, Jupyter notebooks, syntax highlighting
- **Advanced Template System**: Multiple themes, i18n support, live reload
- **API Integration**: REST/GraphQL APIs for headless CMS functionality
- **Plugin Architecture**: Dynamic renderer loading at runtime
- **Full-Text Search**: Index content for fast searching
- **Live Preview**: WebSocket-based hot reload for development

The application is production-ready with:

- Clean, extensible architecture
- Comprehensive testing
- Complete documentation (README, architecture, custom renderers)
- Real-world deployment readiness

## Go best practices

- Use modern Golang 1.25+ patterns
- Prefer `any` to `interface{}`
- Always use `make test` to run tests
