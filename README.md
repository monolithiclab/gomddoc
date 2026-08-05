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
- **Well Tested**: 83.5% test coverage with comprehensive integration tests

## Quick Start

### Installation

```bash
# Install script (Linux / macOS, amd64 / arm64)
curl -fsSL https://raw.githubusercontent.com/monolithiclab/gomddoc/main/scripts/install.sh | sh

# Homebrew (macOS / Linux)
brew install monolithiclab/tap/gomddoc

# Go toolchain
go install github.com/monolithiclab/gomddoc/cmd/gomddoc@latest

# Docker (multi-arch image on GitHub Container Registry)
docker run --rm -p 8080:8080 -v "$PWD:/site" ghcr.io/monolithiclab/gomddoc serve /site
```

The install script downloads the matching release archive, verifies it against the release
`SHA256SUMS`, and installs to `/usr/local/bin` (falling back to `~/.local/bin`). If
[cosign](https://github.com/sigstore/cosign) is on your `PATH` it also verifies the signature over
`SHA256SUMS` first — install cosign before running the script to get a provenance-checked install,
since `SHA256SUMS` is otherwise served from the same origin as the archive it vouches for. Set
`GOMDDOC_REQUIRE_COSIGN=1` to abort rather than continue when the signature cannot be checked.

Override the target with `GOMDDOC_INSTALL_DIR`, or pin a version with `GOMDDOC_VERSION`:

```bash
curl -fsSL https://raw.githubusercontent.com/monolithiclab/gomddoc/main/scripts/install.sh \
  | GOMDDOC_VERSION=0.1.1 GOMDDOC_INSTALL_DIR="$HOME/bin" sh
```

Prebuilt binaries and checksums for Linux and macOS (amd64/arm64) are attached to each
[GitHub Release](https://github.com/monolithiclab/gomddoc/releases). Checksums are signed with
[cosign](https://github.com/sigstore/cosign) — verify them with:

```bash
cosign verify-blob SHA256SUMS \
  --certificate SHA256SUMS.pem --signature SHA256SUMS.sig \
  --certificate-identity-regexp '^https://github.com/monolithiclab/gomddoc/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Build from source:

```bash
# Clone and build
git clone https://github.com/monolithiclab/gomddoc
cd gomddoc
make build

# Or run directly
make run
```

### Usage

```bash
# Serve current directory on default port 8080
./build/gomddoc serve

# Serve specific directory (positional argument)
./build/gomddoc serve /path/to/docs

# Use custom port
./build/gomddoc serve -p :9000

# Enable directory listing (disabled by default for security)
GOMDDOC_SITE_DIR_INDEX=true ./build/gomddoc serve

# Combine options
./build/gomddoc serve ./docs -p :3000

# Show version
./build/gomddoc --version
```

### Examples

```bash
# Start server
$ ./build/gomddoc serve
2025/10/06 13:00:00 INFO Server started url=http://localhost:8080 dir=. dev=false

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
- If `README.md` not found and `GOMDDOC_SITE_DIR_INDEX=false` (default): Returns 403 Forbidden
- If `README.md` not found and `GOMDDOC_SITE_DIR_INDEX=true`: Generates markdown directory listing

## Configuration

### Command Line Flags

```bash
gomddoc serve [flags]
```

| Flag                | Default | Env Var                           | Description                                       |
| ------------------- | ------- | --------------------------------- | ------------------------------------------------- |
| `DIR` (positional)  | `.`     | `GOMDDOC_SERVER_DIR`              | Markdown directory or Git URL                     |
| `-p`, `--port`      | `:8080` | `GOMDDOC_SERVER_PORT`             | HTTP listen address (`:auto` to auto-assign)      |
| `--admin-port`      |         | `GOMDDOC_SERVER_ADMIN_PORT`       | Separate address for health, metrics, and pprof   |
| `-d`, `--domain`    |         | `GOMDDOC_DOMAIN`                  | Override site domain for canonical URLs and SEO   |
| `--git-key-file`    |         | `GOMDDOC_SERVER_GIT_SSH_KEY`      | Path to SSH private key file                      |
| `--git-storage-dir` |         | `GOMDDOC_SERVER_GIT_STORAGE_DIR`  | Disk-based Git clone directory (default: memory)  |
| `--pprof`           | `false` | `GOMDDOC_SERVER_PPROF`            | Enable pprof endpoints at `/debug/pprof/`         |
| `--basic-auth-file` |         | `GOMDDOC_SERVER_BASIC_AUTH_FILE`  | Path to htpasswd file (bcrypt hashes only)        |
| `--version`         |         |                                   | Show version and exit                             |

There is no `--dev` flag. Dev mode comes from `gomddoc preview`, which enables it unconditionally,
or from `GOMDDOC_SERVER_DEV_MODE=true`.

### Environment Variables

All environment variables can be listed with `gomddoc info`, which prints each one with its type and
default. `gomddoc serve --help` shows only the vars that have a matching flag.

**Server settings:**

| Variable                                  | Type     | Default     | Description                            |
| ----------------------------------------- | -------- | ----------- | -------------------------------------- |
| `GOMDDOC_SERVER_DIR`                      | string   | `.`         | Markdown directory or Git URL          |
| `GOMDDOC_SERVER_PORT`                     | string   | `:8080`     | HTTP listen address (host:port)        |
| `GOMDDOC_SERVER_DEV_MODE`                 | bool     | `false`     | Enable development mode                |
| `GOMDDOC_SERVER_ADMIN_PORT`               | string   |             | Separate address for admin endpoints   |
| `GOMDDOC_SERVER_PPROF`                    | bool     | `false`     | Enable pprof endpoints                 |
| `GOMDDOC_SERVER_BASIC_AUTH_FILE`          | string   |             | htpasswd file (bcrypt hashes only)     |
| `GOMDDOC_SERVER_GIT_SSH_KEY`              | string   |             | Path to SSH private key file           |
| `GOMDDOC_SERVER_GIT_STORAGE_DIR`          | string   |             | Disk-based Git clone directory         |
| `GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT`    | duration | `1s`        | Graceful shutdown timeout              |
| `GOMDDOC_SERVER_HTTP_READ_HEADER_TIMEOUT` | duration | `5s`        | HTTP read header timeout               |
| `GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT`       | duration | `30s`       | HTTP write timeout                     |
| `GOMDDOC_SERVER_HTTP_IDLE_TIMEOUT`        | duration | `2m`        | HTTP idle timeout                      |
| `GOMDDOC_SERVER_HTTP_MAX_HEADER_MB`       | int      | `1`         | Maximum header size in MB              |

**Site settings** (also configurable via `.gomddoc/config.yml`):

| Variable                        | Type   | Default      | Description                    |
| ------------------------------- | ------ | ------------ | ------------------------------ |
| `GOMDDOC_SITE_DEFAULT_INDEX`    | string | `README.md`  | Default file for directories   |
| `GOMDDOC_SITE_DIR_INDEX`        | bool   | `false`      | Enable directory listing       |
| `GOMDDOC_SITE_EDIT_URL`         | string |              | Edit URL template              |
| `GOMDDOC_SITE_COLOR_CHIPS`      | bool   | `true`       | Enable hex color chip rendering |
| `GOMDDOC_SITE_META_TITLE`       | string | Dir name     | Site title                     |
| `GOMDDOC_SITE_META_DESCRIPTION` | string |              | Site description               |
| `GOMDDOC_SITE_META_DOMAIN`      | string |              | Site domain (without protocol) |
| `GOMDDOC_SITE_THEME_NAME`       | string | `default`    | Theme name                     |
| `GOMDDOC_SITE_HIGHLIGHTING_THEME` | string | `github`   | Syntax highlighting theme      |

**Note**: All environment variables use hierarchical prefixes: `GOMDDOC_SERVER_*` for server settings, `GOMDDOC_SITE_*` for site settings. Environment variables take precedence over config file values.

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
  color_chips: true # Optional (enabled by default)
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

- Uses [goldmark](https://github.com/yuin/goldmark) library
- CommonExtensions: Tables, fenced code, strikethrough
- AutoHeadingIDs: Automatic anchor links with hover anchors
- GitHub-style admonitions (`[!NOTE]`, `[!WARNING]`, etc.)
- Color chips: backtick-wrapped hex codes (e.g. `` `#E91E63` ``) render as an inline swatch + code label
  - Enabled globally via `GOMDDOC_SITE_COLOR_CHIPS` (default: `true`) or `color_chips` in config
  - Override per page with YAML frontmatter: `color_chips: false` to disable, `color_chips: true` to re-enable

**PassthroughRenderer** (`*/*` → same type)

- Wildcard catch-all for unregistered MIME types
- Returns content unchanged
- Preserves original MIME type

## Security

- **Path Traversal Protection**: Uses `os.DirFS()` to jail file access within specified directory
- **Hidden File Blocking**: Middleware blocks all paths starting with `.` (except `.well-known/`)
- **Security Headers**: Automatically adds `X-Content-Type-Options: nosniff` and `X-Frame-Options: DENY`
- **Directory Listing Disabled by Default**: Prevents information disclosure (`GOMDDOC_SITE_DIR_INDEX=false`)
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

A renderer turns the bytes of one MIME type into the bytes of another. Adding one is a change to
the gomddoc source tree, not a plugin — `internal/renderer` is an internal package and registration
happens in `cmd/gomddoc/pipeline.go`:

```go
// JSONRenderer converts JSON to a formatted HTML block.
type JSONRenderer struct{}

func (j *JSONRenderer) InputMimeTypes() []string  { return []string{"application/json"} }
func (j *JSONRenderer) OutputMimeTypes() []string { return []string{"text/html"} }

func (j *JSONRenderer) Render(ctx context.Context, content []byte, _ *enricher.EnrichmentData) (*renderer.RenderResult, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }

    var buf bytes.Buffer
    if err := json.Indent(&buf, content, "", "  "); err != nil {
        return nil, fmt.Errorf("indent json: %w", err)
    }

    return &renderer.RenderResult{
        Content:  []byte("<pre class=\"json\">" + html.EscapeString(buf.String()) + "</pre>"),
        MimeType: "text/html; charset=utf-8",
    }, nil
}

// In setupPipeline (cmd/gomddoc/pipeline.go):
registry.Register(&JSONRenderer{})
```

See the [Custom Renderers guide](docs/guide/12-advanced/04-custom-renderers.md) for the negotiation
rules, MIME registration, and testing.

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
GOMDDOC_SITE_DIR_INDEX=true ./build/gomddoc serve
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
./build/gomddoc serve -p :8081
```

**Permission denied:**

```bash
chmod 644 *.md
chmod 755 $(find . -type d)
```

**Template errors in dev mode:**

```bash
./build/gomddoc preview        # Dev mode: templates re-parsed on every request
GOMDDOC_SERVER_DEV_MODE=true ./build/gomddoc serve
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

gomddoc is **dual-licensed**:

- **Noncommercial use** (personal projects, hobby use, education, research, and other noncommercial
  purposes) is free under the [PolyForm Noncommercial License 1.0.0](LICENSE).
- **Commercial use** — any use that is not a noncommercial purpose, including use in or for a
  for-profit business, product, or service — requires a separate commercial license. See
  [LICENSE-COMMERCIAL.md](LICENSE-COMMERCIAL.md).

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
