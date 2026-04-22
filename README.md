# Gomddoc

A production-ready HTTP server for serving Markdown documentation with automatic rendering and content negotiation.

## Features

- **Universal Content Serving**: Serves Markdown, HTML, CSS, JS, images, and all other content types
- **Smart Rendering**: Automatic Markdown→HTML conversion with template wrapping
- **Content Negotiation**: HTTP Accept header support with proper 406 responses
- **Directory Listing**: Optional directory browsing with automatic README.md fallback
- **Zero Configuration**: Works out of the box - just point it at a directory
- **Secure by Design**: Built-in path traversal protection, hidden file blocking, secure defaults
- **Production Ready**: Proper HTTP status codes, security headers, graceful shutdown
- **Extensible Architecture**: Plugin-style renderer system for custom content types
- **Well Tested**: 78.9% test coverage with comprehensive integration tests

## Quick Start

### Installation

```bash
# Clone and build
git clone <repository-url>
cd gomddoc
make build

# Or run directly
make run
```

### Usage

```bash
# Serve current directory on default port 8080
./build/gomddoc

# Serve specific directory
./build/gomddoc -d /path/to/docs

# Use custom port
./build/gomddoc -p :9000

# Enable directory listing (disabled by default for security)
GOMDDOC_SERVER_DIR_INDEX=true ./build/gomddoc

# Combine options
./build/gomddoc -d ./docs -p :3000
```

### Examples

```bash
# Start server
$ ./build/gomddoc
2025/10/06 13:00:00 INFO Listening... Addr=:8080

# Access different content types
curl http://localhost:8080/                    # Serves README.md as HTML
curl http://localhost:8080/docs/guide.md       # Markdown → HTML
curl http://localhost:8080/assets/style.css    # CSS passthrough
curl http://localhost:8080/images/logo.png     # Image passthrough
curl http://localhost:8080/api/data.json       # JSON passthrough
```

## Supported Content Types

Gomddoc serves all content types with intelligent rendering:

| Content Type                   | Handling               | Template Wrapping |
| ------------------------------ | ---------------------- | ----------------- |
| `.md`, `.markdown`             | Markdown → HTML        | ✅ Yes            |
| `.html`, `.htm`                | HTML passthrough       | ✅ Yes            |
| `.css`                         | CSS passthrough        | ❌ No             |
| `.js`                          | JavaScript passthrough | ❌ No             |
| `.json`                        | JSON passthrough       | ❌ No             |
| `.png`, `.jpg`, `.gif`, `.svg` | Image passthrough      | ❌ No             |
| `.pdf`                         | PDF passthrough        | ❌ No             |
| All others                     | Binary passthrough     | ❌ No             |

**Directory Handling:**

- First tries to serve `README.md` from the directory
- If `README.md` not found and `GOMDDOC_SERVER_DIR_INDEX=false` (default): Returns 403 Forbidden
- If `README.md` not found and `GOMDDOC_SERVER_DIR_INDEX=true`: Generates markdown directory listing

## Configuration

### Command Line Flags

| Flag | Default | Description                         |
| ---- | ------- | ----------------------------------- |
| `-d` | `.`     | Directory to serve files from       |
| `-p` | `:8080` | Port to listen on (format: `:8080`) |

### Environment Variables

| Variable                       | Default     | Description                              |
| ------------------------------ | ----------- | ---------------------------------------- |
| `GOMDDOC_DIR`                  | `.`         | Directory to serve (same as `-d`)        |
| `GOMDDOC_PORT`                 | `:8080`     | Port to listen on (same as `-p`)         |
| `GOMDDOC_SERVER_DEFAULT_INDEX` | `README.md` | Default file to serve for directories    |
| `GOMDDOC_SERVER_DIR_INDEX`     | `false`     | Enable directory listing generation      |
| `GOMDDOC_DEV_MODE`             | `false`     | Disable template caching for development |
| `GOMDDOC_SHUTDOWN_TIMEOUT`     | `1s`        | Graceful shutdown timeout                |

**Site Configuration** (overrides `.gomddoc/config.yml`):

| Variable                   | Default        | Description                    |
| -------------------------- | -------------- | ------------------------------ |
| `GOMDDOC_META_TITLE`       | Directory name | Site title                     |
| `GOMDDOC_META_DESCRIPTION` | `""`           | Site description               |
| `GOMDDOC_META_DOMAIN`      | `""`           | Site domain (without protocol) |
| `GOMDDOC_THEME_NAME`       | `default`      | Theme name                     |

**Note**: All environment variables are prefixed with `GOMDDOC_`. Nested configuration fields use hierarchical prefixes:

- `ServerConfig` fields: `GOMDDOC_SERVER_*`
- `SiteConfig.Meta` fields: `GOMDDOC_META_*`
- `SiteConfig.Theme` fields: `GOMDDOC_THEME_*`

### Configuration File

Create `.gomddoc/config.yml` in your document directory:

```yaml
site:
  title: "My Documentation"
  description: "Project documentation site"
  baseurl: "https://docs.example.com"
  meta:
    author: "Your Name"
    keywords: "documentation, api, guide"

server:
  defaultindex: "README.md" # Optional
  dirindex: false # Optional (secure by default)
```

**Priority:** CLI flags > Environment variables > Config file > Defaults

## How It Works

### Rendering Pipeline

```
1. Request → Provider reads file + detects MIME type
2. Registry selects appropriate renderer based on MIME type
3. Renderer processes content (e.g., Markdown → HTML)
4. Handler wraps text/html in template OR serves raw for other types
5. Response → Client with proper Content-Type and headers
```

### Content Negotiation

Gomddoc supports HTTP Accept header negotiation:

```bash
# Request HTML (markdown files render to HTML)
curl -H "Accept: text/html" http://localhost:8080/docs.md
# Returns 200 with templated HTML

# Request JSON (markdown can't provide JSON)
curl -H "Accept: application/json" http://localhost:8080/docs.md
# Returns 406 Not Acceptable

# Wildcard accepts anything
curl -H "Accept: */*" http://localhost:8080/docs.md
# Returns 200 with HTML
```

### Built-in Renderers

**MarkdownRenderer** (`text/markdown` → `text/html`)

- Uses [gomarkdown](https://github.com/gomarkdown/markdown) library
- CommonExtensions: Tables, fenced code, strikethrough
- AutoHeadingIDs: Automatic anchor links
- NoEmptyLineBeforeBlock: Cleaner output

**PassthroughRenderer** (`*/*` → same type)

- Wildcard catch-all for unregistered MIME types
- Returns content unchanged
- Preserves original MIME type

## Security

- **Path Traversal Protection**: Uses `os.DirFS()` to jail file access within specified directory
- **Hidden File Blocking**: Middleware blocks all paths starting with `.` (except `.well-known/`)
- **Security Headers**: Automatically adds `X-Content-Type-Options: nosniff` and `X-Frame-Options: DENY`
- **Directory Listing Disabled by Default**: Prevents information disclosure (`GOMDDOC_SERVER_DIR_INDEX=false`)
- **Proper Error Codes**: 403 Forbidden for disabled features, 404 for missing files
- **Context Cancellation**: Protects against slow-loris attacks with request timeout handling

## Development

### Prerequisites

- Go 1.25+
- Make (optional, for convenience commands)

### Development Commands

```bash
make run          # Run locally with hot reload
make test         # Run tests with coverage report
make bench        # Run benchmarks
make lint         # Run all linters (format, vet, staticcheck, golangci-lint, gosec, gocritic)
make format       # Format code with gofmt
make build        # Build binary to build/gomddoc
make clean        # Remove build artifacts
make update-deps  # Update dependencies
```

### Project Structure

```
gomddoc/
├── cmd/gomddoc/              # CLI entry point and main()
├── internal/
│   ├── config/              # Configuration management
│   ├── provider/            # Content providers (filesystem, future: S3, DB)
│   ├── renderer/            # Content renderers (markdown, passthrough, custom)
│   ├── template/            # Template rendering and caching
│   └── server/              # HTTP server, handlers, middleware
├── assets/                   # Embedded themes and templates
├── docs/                     # Architecture documentation
├── Makefile                  # Development commands
├── CLAUDE.md                 # Development guidance
└── README.md                 # This file
```

### Architecture

See [docs/architecture.md](docs/architecture.md) for detailed architecture documentation.

**Key Design Principles:**

- **Interface-Driven**: Clean contracts for providers, renderers, and templates
- **MIME-Type Based**: Universal content handling via standard MIME types
- **Self-Declaring Renderers**: No hardcoded MIME→renderer mappings
- **Separation of Concerns**: Provider (I/O) → Renderer (transform) → Handler (HTTP)
- **Extensible**: Add custom renderers with ~10 lines of code

## Custom Renderers

Create custom renderers for specialized content types:

```go
package main

import (
    "context"
    "github.com/monolithiclab/gomddoc/internal/renderer"
)

// JSONRenderer converts JSON to formatted HTML
type JSONRenderer struct{}

func (j *JSONRenderer) SupportedMimeTypes() []string {
    return []string{"application/json"}
}

func (j *JSONRenderer) Render(ctx context.Context, content []byte) ([]byte, string, error) {
    // Check context
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Format JSON with syntax highlighting
    formatted := formatJSON(content)
    html := []byte("<pre class=\"json\">" + formatted + "</pre>")

    return html, "text/html; charset=utf-8", nil
}

// Register in main.go
registry.Register(&JSONRenderer{})
```

See [docs/custom-renderers.md](docs/custom-renderers.md) for more examples.

## API Reference

### HTTP Endpoints

All files are served through a unified content handler with automatic MIME detection:

| Path                 | Description                                        |
| -------------------- | -------------------------------------------------- |
| `/`                  | Serves default index file (README.md by default)   |
| `/{filename}`        | Serves file with automatic rendering               |
| `/{path}/{filename}` | Serves files from subdirectories                   |
| `/{directory}/`      | Serves README.md or directory listing (if enabled) |

### HTTP Headers

**Request Headers:**

- `Accept`: Content negotiation (e.g., `text/html`, `application/json`, `*/*`)

**Response Headers:**

- `Content-Type`: Detected MIME type with charset
- `Content-Length`: Byte length of response
- `Cache-Control: public, max-age=300` (5-minute cache for non-HTML)
- `X-Content-Type-Options: nosniff` (security)
- `X-Frame-Options: DENY` (security)

**Status Codes:**

- `200 OK`: Successfully served content
- `403 Forbidden`: Directory listing disabled or permission denied
- `404 Not Found`: File doesn't exist
- `406 Not Acceptable`: Client Accept header can't be satisfied
- `499`: Client closed request (context canceled)
- `504 Gateway Timeout`: Request timeout exceeded

## Performance

- **Lightweight**: ~8MB binary, <15MB RAM usage
- **Fast Startup**: Sub-second startup time
- **Template Caching**: Production mode caches parsed templates
- **Efficient**: Direct file serving with minimal allocations
- **Concurrent**: Thread-safe renderer registry
- **Scalable**: Handles hundreds of concurrent requests

## Testing

```bash
# Run all tests with coverage
make test

# Run specific package tests
go test ./internal/server -v

# Run integration tests only
go test ./internal/server -run Integration -v

# View coverage in browser
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Current Coverage:** 78.9% overall

- `internal/server`: 87.0%
- `internal/renderer`: 97.8%
- `internal/provider`: 89.5%
- `internal/config`: 84.5%
- `internal/template`: 83.8%

## Troubleshooting

### Directory listing shows 403 Forbidden

Directory listing is disabled by default for security. Enable it:

```bash
GOMDDOC_SERVER_DIR_INDEX=true ./build/gomddoc
```

### Image/CSS/JS files not loading

Ensure your HTML/Markdown uses relative paths:

```markdown
![Logo](./images/logo.png)

<link rel="stylesheet" href="./assets/style.css">
```

### 406 Not Acceptable errors

The server can't provide content in the format requested by the Accept header:

```bash
# This will fail if requesting JSON for a markdown file
curl -H "Accept: application/json" http://localhost:8080/docs.md

# Use */* to accept any format
curl -H "Accept: */*" http://localhost:8080/docs.md
```

### Common Issues

**Port already in use:**

```bash
./build/gomddoc -p :8081
```

**Permission denied:**

```bash
chmod 644 *.md
chmod 755 $(find . -type d)
```

**Template errors in dev mode:**

```bash
DEV_MODE=true ./build/gomddoc  # Disables template caching
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Format code: `make format`
4. Run linting: `make lint`
5. Run tests: `make test`
6. Submit a pull request

All contributions must maintain or improve test coverage (78.9%+).

## License

[Add your license here]

## Changelog

### v2.0.0 (Current - Renderer System)

- ✅ Universal content type support (Markdown, HTML, CSS, JS, images, etc.)
- ✅ Content renderer architecture with MIME-type based routing
- ✅ HTTP Accept header negotiation with proper 406 responses
- ✅ Directory listing generation (optional, secure by default)
- ✅ Custom renderer support for extensibility
- ✅ Comprehensive integration tests (78.9% coverage)
- ✅ MIME type normalization and wildcard matching
- ✅ Context cancellation support in renderers

### v1.0.0 (Previous - Basic Markdown Server)

- ✅ Markdown to HTML conversion
- ✅ Security headers middleware
- ✅ Path traversal protection
- ✅ Graceful shutdown
- ✅ Production-ready error handling
