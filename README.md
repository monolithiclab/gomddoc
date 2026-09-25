# Gomddoc

A production-ready HTTP server and static site generator for Markdown documentation, with content negotiation,
full-text search, SEO output and a built-in MCP server.

## Features

- **Universal Content Serving**: Renders Markdown to HTML inside a theme; serves HTML, CSS, JS, images and every
  other file type with its detected MIME type
- **Content Negotiation**: `Accept` header support; `Accept: text/markdown` returns the raw Markdown source, an
  unsatisfiable `Accept` gets `406`
- **Clean URLs**: `.md` is stripped from URLs by default (`/guide` serves `guide.md`; `/guide.md` redirects with
  `301`)
- **Git-Native Content**: Serve a local directory or a `git+https://` / `git+ssh://` repository URL (in-memory or
  on-disk clone)
- **Static Site Generation**: `gomddoc build` writes a static site with clean-URL directories, `404.html`, sitemap,
  feed and redirect stubs
- **Full-Text Search**: TF-IDF inverted index built at startup, `tag:` filter syntax, `GET /api/search`, and a
  Ctrl+K search dialog in every theme
- **SEO**: Canonical URLs, `sitemap.xml`, `robots.txt`, Atom `feed.xml`, Open Graph and Twitter tags, JSON-LD,
  per-page `robots`, `redirect_from` and `rel="prev/next"`, enabled by setting a domain
- **Tags**: Frontmatter tags rendered as chips, `/tags/` and `/tags/{tag}` pages, a see-also section and
  `/api/tags`
- **Internationalization**: BCP 47 language directories (`fr-FR/`) served under `/{lang}/`, translated UI strings,
  a language switcher and `hreflang` tags
- **MCP Server**: `gomddoc mcp` (stdio) and `/_mcp/` (Streamable HTTP) expose read-only search, page, section and
  navigation tools to AI agents
- **Themes**: One bundled theme (`default`) plus seven more in
  [gomddoc-themes](https://github.com/monolithiclab/gomddoc-themes); feature toggles and CSS variables in config
- **Self-Documenting**: `gomddoc help`, `gomddoc info`, `gomddoc schema` and `gomddoc doctor` read the embedded
  guide, list every setting, print the config JSON Schema and check a site for problems
- **Secure Defaults**: Path jail, hidden-file and `exclude` blocking (404), GET/HEAD-only content routes,
  security headers, optional htpasswd Basic Auth
- **Operations**: Graceful shutdown, configurable timeouts, gzip, ETags, Prometheus metrics, health endpoints,
  optional pprof, and a separate admin listener

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
`SHA256SUMS`, and installs to `/usr/local/bin` (using `sudo` when that directory is not writable, and
falling back to `~/.local/bin` when `sudo` is unavailable). If
[cosign](https://github.com/sigstore/cosign) is on your `PATH` it also verifies the signature over
`SHA256SUMS` first — install cosign before running the script to get a provenance-checked install,
since `SHA256SUMS` is otherwise served from the same origin as the archive it vouches for. Set
`GOMDDOC_REQUIRE_COSIGN=1` to abort rather than continue when the signature cannot be checked.

Override the target with `GOMDDOC_INSTALL_DIR`, or pin a version with `GOMDDOC_VERSION`:

```bash
curl -fsSL https://raw.githubusercontent.com/monolithiclab/gomddoc/main/scripts/install.sh \
  | GOMDDOC_VERSION=0.1.3 GOMDDOC_INSTALL_DIR="$HOME/bin" sh
```

Prebuilt binaries and checksums for Linux and macOS (amd64/arm64) are attached to each
[GitHub Release](https://github.com/monolithiclab/gomddoc/releases). Checksums are signed with
[cosign](https://github.com/sigstore/cosign) — verify them with:

```bash
cosign verify-blob SHA256SUMS \
  --bundle SHA256SUMS.sigstore.json \
  --certificate-identity-regexp '^https://github.com/monolithiclab/gomddoc/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Releases up to v0.1.2 predate the bundle and ship `SHA256SUMS.sig` + `SHA256SUMS.pem` instead;
pass `--certificate SHA256SUMS.pem --signature SHA256SUMS.sig` in place of `--bundle`.

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
# Serve the current directory on :8080
gomddoc serve

# Serve a specific directory (positional argument)
gomddoc serve /path/to/docs

# Serve a Git repository (cloned in memory at startup)
gomddoc serve git+https://github.com/org/repo.git

# Use a custom port
gomddoc serve -p :9000

# Local preview: dev mode, first free port from 8080, optional browser open
gomddoc preview ./docs --open

# Build a static site into build/site
gomddoc build ./docs -o build/site -d docs.example.com

# Scaffold .gomddoc/config.yml
gomddoc init ./docs

# Check the site's configuration and content (exit non-zero on warnings with --strict)
gomddoc doctor ./docs --strict

# Read the embedded guide, list every setting, print the config JSON Schema
gomddoc help configuration
gomddoc info ./docs
gomddoc schema

# Show version
gomddoc --version
```

| Command   | Purpose                                                                              |
| --------- | ------------------------------------------------------------------------------------ |
| `serve`   | Production HTTP server                                                               |
| `preview` | Local preview in dev mode (render cache off), auto-assigned port, `--open` for browser |
| `build`   | Static site generation (`-o`, default `build/site`)                                  |
| `init`    | Create `.gomddoc/config.yml` (`-t` picks the theme)                                  |
| `mcp`     | MCP server over stdio for AI agents; also serves gomddoc's own guide                 |
| `doctor`  | Report configuration and content problems with fixes (`--json`, `--strict`, `-v`)    |
| `help`    | Read or search (`-s`) the embedded guide                                             |
| `info`    | List settings, env vars, flags, theme features and guide topics (`--json`)          |
| `schema`  | Print the JSON Schema for `.gomddoc/config.yml`                                      |

### Examples

```bash
# Start server
$ gomddoc serve
2026/09/25 13:00:00 INFO Server started url=http://localhost:8080 dir=. dev=false

# Access different content types
curl http://localhost:8080/                          # README.md rendered as HTML
curl http://localhost:8080/docs/guide                # docs/guide.md rendered as HTML
curl -i http://localhost:8080/docs/guide.md          # 301 to /docs/guide
curl -H "Accept: text/markdown" http://localhost:8080/docs/guide   # raw Markdown source
curl http://localhost:8080/assets/style.css          # CSS passthrough
curl http://localhost:8080/images/logo.png           # Image passthrough
curl "http://localhost:8080/api/search?q=install"    # Full-text search (JSON)
```

## Supported Content Types

| Content Type                   | Handling                                     | Template Wrapping |
| ------------------------------ | -------------------------------------------- | ----------------- |
| `.md`, `.markdown`             | Markdown → HTML (or raw with `text/markdown`) | Yes               |
| `.html`, `.htm`                | HTML passthrough                             | Yes               |
| `.css`                         | CSS passthrough                              | No                |
| `.js`, `.mjs`                  | JavaScript passthrough                       | No                |
| `.json`                        | JSON passthrough                             | No                |
| `.png`, `.jpg`, `.gif`, `.svg` | Image passthrough                            | No                |
| `.pdf`                         | PDF passthrough                              | No                |
| All others                     | Binary passthrough                           | No                |

**Directory Handling:**

- Serves the directory's `default_index` file (`README.md` by default)
- Without an index file and with `dir_index: false` (default): redirects (302) to the first page of the navigation
  tree, or returns 403 Forbidden when there is none
- Without an index file and with `dir_index: true`: renders a generated directory listing

## Configuration

**Priority:** CLI flags > environment variables > `.gomddoc/config.yml` > defaults

`gomddoc info` prints every setting with its env var, flag and default; `gomddoc schema` prints the JSON Schema for
the config file. The full reference is the [Configuration guide](docs/guide/02-configuration.md)
(`gomddoc help configuration`).

### Command Line Flags

```bash
gomddoc serve [<dir>] [flags]
```

| Flag                | Default | Env Var                          | Description                                          |
| ------------------- | ------- | -------------------------------- | ---------------------------------------------------- |
| `DIR` (positional)  | `.`     | `GOMDDOC_SERVER_DIR`             | Markdown directory or Git URL                        |
| `-p`, `--port`      | `:8080` | `GOMDDOC_SERVER_PORT`            | HTTP listen address (`:auto` picks the first free port from 8080) |
| `--admin-port`      |         | `GOMDDOC_SERVER_ADMIN_PORT`      | Separate address for health, metrics, and pprof      |
| `-d`, `--domain`    |         | `GOMDDOC_DOMAIN`                 | Override `meta.domain` for canonical URLs and SEO    |
| `--git-key-file`    |         | `GOMDDOC_SERVER_GIT_SSH_KEY`     | Path to SSH private key file                         |
| `--git-storage-dir` |         | `GOMDDOC_SERVER_GIT_STORAGE_DIR` | Disk-based Git clone directory (default: memory)     |
| `--pprof`           | `false` | `GOMDDOC_SERVER_PPROF`           | Enable pprof endpoints at `/debug/pprof/`            |
| `--basic-auth-file` |         | `GOMDDOC_SERVER_BASIC_AUTH_FILE` | Path to htpasswd file (bcrypt hashes only)           |
| `--version`         |         |                                  | Show version and exit                                |

Other commands' own flags:

| Command   | Flag                | Default      | Env Var                | Description                              |
| --------- | ------------------- | ------------ | ---------------------- | ---------------------------------------- |
| `build`   | `-o`, `--output`    | `build/site` | `GOMDDOC_BUILD_OUTPUT` | Output directory                         |
| `preview` | `-p`, `--port`      | `:auto`      | `GOMDDOC_SERVER_PORT`  | HTTP listen address                      |
| `preview` | `--open`            | `false`      | `GOMDDOC_PREVIEW_OPEN` | Open the browser on startup              |
| `preview` | `--dir-index`       | `false`      | `GOMDDOC_DIR_INDEX`    | Enable generated directory listings      |
| `init`    | `-t`, `--theme`     | `default`    |                        | Theme written to the new config          |
| `doctor`  | `--json`            | `false`      |                        | Print the report as JSON                 |
| `doctor`  | `--strict`          | `false`      |                        | Exit non-zero on warnings too            |
| `doctor`  | `-v`, `--verbose`   | `false`      |                        | Include info-level findings              |
| `info`    | `--json`            | `false`      |                        | Print the capabilities report as JSON    |
| `help`    | `-s`, `--search`    |              |                        | Search the guide                         |

`build`, `preview`, `mcp` and `doctor` also take the positional directory, `--git-key-file` and
`--git-storage-dir`; `build` and `preview` take `-d, --domain`.

There is no `--dev` flag. Dev mode comes from `gomddoc preview`, which enables it unconditionally,
or from `GOMDDOC_SERVER_DEV_MODE=true`.

### Environment Variables

`gomddoc info` lists every environment variable with its type and default. `gomddoc <command> --help` shows only
the vars that have a matching flag.

**Server settings** (flag or environment variable only; not read from `config.yml`):

| Variable                                  | Type     | Default | Description                                |
| ----------------------------------------- | -------- | ------- | ------------------------------------------ |
| `GOMDDOC_SERVER_DIR`                      | string   | `.`     | Markdown directory or Git URL              |
| `GOMDDOC_SERVER_PORT`                     | string   | `:8080` | HTTP listen address (host:port)            |
| `GOMDDOC_SERVER_DEV_MODE`                 | bool     | `false` | Disable the render cache (always on for `preview`) |
| `GOMDDOC_SERVER_ADMIN_PORT`               | string   |         | Separate address for admin endpoints       |
| `GOMDDOC_SERVER_PPROF`                    | bool     | `false` | Enable pprof endpoints                     |
| `GOMDDOC_SERVER_BASIC_AUTH_FILE`          | string   |         | htpasswd file (bcrypt hashes only)         |
| `GOMDDOC_SERVER_GIT_SSH_KEY`              | string   |         | Path to SSH private key file               |
| `GOMDDOC_SERVER_GIT_STORAGE_DIR`          | string   |         | Disk-based Git clone directory             |
| `GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT`    | duration | `1s`    | Graceful shutdown timeout                  |
| `GOMDDOC_SERVER_HTTP_READ_HEADER_TIMEOUT` | duration | `5s`    | HTTP read header timeout                   |
| `GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT`       | duration | `30s`   | HTTP write timeout                         |
| `GOMDDOC_SERVER_HTTP_IDLE_TIMEOUT`        | duration | `2m`    | HTTP idle timeout                          |
| `GOMDDOC_SERVER_HTTP_MAX_HEADER_MB`       | int      | `1`     | Maximum header size in MiB                 |

**Site settings** (also configurable in `.gomddoc/config.yml`):

| Variable                             | Type   | Default                  | Description                                       |
| ------------------------------------ | ------ | ------------------------ | ------------------------------------------------- |
| `GOMDDOC_SITE_DEFAULT_INDEX`         | string | `README.md`              | File served for a directory URL                   |
| `GOMDDOC_SITE_DIR_INDEX`             | bool   | `false`                  | Enable generated directory listings               |
| `GOMDDOC_SITE_EDIT_URL`              | string |                          | Base URL for "Edit this page" links               |
| `GOMDDOC_SITE_LANGUAGE`              | string | `en-US`                  | BCP 47 language of the default content tree       |
| `GOMDDOC_SITE_META_TITLE`            | string | Directory name, title-cased | Site title                                     |
| `GOMDDOC_SITE_META_DESCRIPTION`      | string |                          | Site description                                  |
| `GOMDDOC_SITE_META_DOMAIN`           | string |                          | Site domain (bare host, no scheme or path)        |
| `GOMDDOC_SITE_META_ROBOTS`           | string |                          | Site-wide `<meta name="robots">` value            |
| `GOMDDOC_SITE_THEME_NAME`            | string | `default`                | Theme name                                        |
| `GOMDDOC_SITE_THEME_FEATURES_<KEY>`  | bool   | `true`                   | Theme feature toggle, e.g. `..._COLOR_CHIPS=false` |
| `GOMDDOC_SITE_HIGHLIGHTING_THEME`    | string | `github`                 | Chroma syntax highlighting style                  |
| `GOMDDOC_SITE_SEARCH_INDEX`          | bool   | `true`                   | Build the full-text search index at startup       |

`theme.vars`, `exclude` and `strip_extensions` are config-file only.

### Configuration File

Create `.gomddoc/config.yml` in your content directory (`gomddoc init` writes one). Keys sit at the top level, with
no `site:` wrapper; an unknown key is a startup error.

```yaml
default_index: README.md
dir_index: false
edit_url: https://github.com/org/repo/edit/main/docs
language: en-US
meta:
  title: My Documentation
  description: Project documentation site
  domain: docs.example.com
theme:
  name: default
  vars:
    primary: "#0969da"
  features:
    color_chips: true
    mermaid: false
highlighting:
  theme: github
search:
  index: true
exclude:
  - drafts/
  - TODO.md
strip_extensions: [.md]
```

## How It Works

### Rendering Pipeline

```
1. Request → Provider reads file + detects MIME type
2. Registry selects a renderer from the MIME type and the Accept header
3. Enricher extracts frontmatter, TOC, navigation and related pages
4. Renderer processes content (e.g., Markdown → HTML)
5. Handler wraps text/html in the theme template OR serves raw for other types
6. Response → Client with Content-Type, ETag and Cache-Control
```

### Content Negotiation

```bash
# Request HTML (markdown files render to HTML)
curl -H "Accept: text/html" http://localhost:8080/docs
# Returns 200 with templated HTML

# Request the Markdown source
curl -H "Accept: text/markdown" http://localhost:8080/docs
# Returns 200 with text/markdown

# Request JSON (markdown can't provide JSON)
curl -H "Accept: application/json" http://localhost:8080/docs
# Returns 406 Not Acceptable

# Wildcard accepts anything
curl -H "Accept: */*" http://localhost:8080/docs
# Returns 200 with HTML
```

### Built-in Renderers

**MarkdownRenderer** (`text/markdown` → `text/html`)

- Uses [goldmark](https://github.com/yuin/goldmark) with GitHub Flavored Markdown (tables, strikethrough, linkify,
  task lists) and raw HTML enabled
- YAML frontmatter is stripped from the output
- Syntax highlighting via Chroma (`highlighting.theme`, default `github`)
- Automatic heading IDs with hover anchors
- GitHub-style admonitions (`[!NOTE]`, `[!TIP]`, `[!IMPORTANT]`, `[!WARNING]`, `[!CAUTION]`)
- Color chips: backtick-wrapped hex codes (e.g. `` `#E91E63` ``) render as an inline swatch + code label
  - Controlled by the theme feature `color_chips` (default: on): `theme.features.color_chips` in config or
    `GOMDDOC_SITE_THEME_FEATURES_COLOR_CHIPS`
  - Override per page with frontmatter `features: { color_chips: false }`

KaTeX math and Mermaid diagrams render client-side in the theme (features `katex` and `mermaid`).

**MarkdownPassthroughRenderer** (`text/markdown` → `text/markdown`)

- Serves the Markdown source when the client asks for `text/markdown`

**PassthroughRenderer** (`*/*` → same type)

- Wildcard catch-all for every other MIME type
- Returns content unchanged with its original MIME type

## Security

- **Path Traversal Protection**: File access is jailed to the content root (`os.DirFS` for directories,
  `fs.ValidPath` checks for Git trees)
- **Hidden and Excluded Paths**: Any path segment starting with `.` (except `.well-known`) and every `exclude`
  pattern return 404, the same page as a missing file; excluded files are also absent from navigation, search,
  metadata, MCP and build output
- **Method Filtering**: Content routes accept GET and HEAD only (405 otherwise)
- **Security Headers**: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
  `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy`, and `Strict-Transport-Security` on TLS
  requests
- **Basic Auth**: `--basic-auth-file` gates every route except health, `robots.txt` and `/_assets/` (bcrypt
  htpasswd entries only)
- **Directory Listing Disabled by Default**: `dir_index: false`
- **Timeouts**: Read-header, write and idle timeouts, and a request header size cap
- **Git over SSH**: Strict host key verification against `known_hosts`

## Development

### Prerequisites

- Go 1.26+ (see `go.mod`)
- Make (optional, for convenience commands)

### Development Commands

```bash
make ci           # codefix + format + lint + test — run this before committing
make run          # Serve ./testsite locally (go run; no hot reload)
make test         # Run tests with race detection and a coverage report
make bench        # Run benchmarks (bench-save / bench-compare for benchstat baselines)
make lint         # Run all linters (format, vet, staticcheck, golangci-lint, gosec, gocritic, govulncheck)
make format       # Format code with gofmt
make codefix      # Apply go fix modernizations
make build        # Build binary to build/gomddoc
make install      # Install gomddoc to $GOBIN (or $GOPATH/bin)
make clean        # Remove build artifacts
make update-deps  # Update dependencies
```

`make lint -j8` runs the linters in parallel; each one is also a target of its own (`make lint-vulncheck`).
`make lint` needs network access for `govulncheck`.

There is no file watcher. `gomddoc preview` runs in dev mode, which re-reads content and templates
from disk per request, so editing the body of an existing page and refreshing the browser is enough.
Adding, renaming or deleting a file needs a restart: the path resolver, navigation tree, metadata
index and search index are all built once at startup.

### Project Structure

```
gomddoc/
├── cmd/gomddoc/              # CLI entry point, subcommands, and embedded assets/
│   └── assets/              # Embedded default theme, shared static files, locales
├── internal/                 # 20 packages — see CLAUDE.md for the full list
│   ├── config/              # Configuration management
│   ├── provider/            # Content providers (filesystem, git, overlay)
│   ├── renderer/            # Content renderers (markdown, markdown passthrough, passthrough)
│   ├── template/            # Template rendering and caching, breadcrumbs, navigation
│   ├── search/              # Full-text search index
│   ├── mcp/                 # Model Context Protocol server
│   └── server/              # HTTP server, handlers, middleware
├── docs/                     # Architecture, decisions, roadmap, specs, plans
│   └── guide/               # User guide, embedded in the binary (gomddoc help)
├── scripts/install.sh        # Release install script
├── testsite/                 # Sample site for local testing
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

The full reference is the [API Reference guide](docs/guide/12-advanced/03-api-reference.md)
(`gomddoc help api-reference`).

### HTTP Endpoints

| Path                                   | Description                                                                 |
| -------------------------------------- | --------------------------------------------------------------------------- |
| `/{path}`                              | Content: Markdown rendered as HTML, other files passed through              |
| `/{directory}/`                        | The directory's `default_index` file, a redirect, or a listing (see above)  |
| `/{lang}/{path}`                       | Content of a detected language directory (e.g. `/fr-FR/guide`)              |
| `GET /api/search?q=&limit=&lang=`      | Full-text search (JSON); supports `tag:name` terms                          |
| `GET /api/tags`, `GET /api/tags/{tag}` | Tags with counts; pages with a tag (JSON)                                   |
| `GET /tags/`, `GET /tags/{tag}`        | Tag index and tag listing pages (HTML)                                      |
| `GET /sitemap.xml`, `GET /feed.xml`    | Sitemap and Atom feed; registered only when `meta.domain` is set            |
| `GET /robots.txt`                      | Robots file, with a `Sitemap:` line when a domain is set                    |
| `GET /_assets/{path}`                  | Theme and shared static assets                                              |
| `/_mcp/`                               | MCP Streamable HTTP endpoint (`serve`, `preview`)                           |
| `GET /health/live`, `GET /health/ready` | Liveness and readiness probes (never behind Basic Auth)                    |
| `/metrics`                             | Prometheus metrics (on `--admin-port` when set)                             |
| `GET /debug/pprof/`                    | pprof, only with `--pprof` (on `--admin-port` when set)                     |

### HTTP Headers

**Request Headers:**

- `Accept`: Content negotiation (e.g., `text/html`, `text/markdown`, `*/*`)
- `Accept-Encoding`: gzip compression
- `If-None-Match`: Conditional requests against the `ETag`

**Response Headers:**

- `Content-Type`: Detected MIME type with charset
- `Content-Length`: Byte length of response
- `ETag`: FNV-64a content hash
- `Cache-Control: public, max-age=300` on rendered pages, files, sitemap and feed;
  `public, max-age=31536000, immutable` on `/_assets/`
- `Vary: Accept` and `Vary: Accept-Encoding`
- `X-Request-ID`: Request identifier, also logged
- Security headers (see [Security](#security))

**Status Codes:**

- `200 OK`: Successfully served content
- `301 Moved Permanently`: Extension redirect (`/guide.md` → `/guide`) or frontmatter `redirect_from`
- `302 Found`: Directory without an index file, redirected to the first page
- `304 Not Modified`: `If-None-Match` matched the `ETag`
- `401 Unauthorized`: Basic Auth required or failed
- `403 Forbidden`: Directory listing disabled or permission denied
- `404 Not Found`: File doesn't exist, is hidden, or is excluded
- `405 Method Not Allowed`: Content request other than GET or HEAD
- `406 Not Acceptable`: Client Accept header can't be satisfied
- `499`: Client closed request (context canceled)
- `504 Gateway Timeout`: Request timeout exceeded

## Performance

- **Single static binary**: about 37 MB from `make build`, about 26 MB for release builds (GoReleaser strips
  symbols with `-s -w`). No runtime, no shared libraries, no sidecar services
- **Startup indexing**: Path resolver, navigation tree, metadata index and search index are built once at startup
- **Template Caching**: Parsed templates and rendered partials are cached outside dev mode
- **Conditional Requests**: ETags on every complete response; `304` without a body on a match
- **Compression**: gzip with pooled writers and buffers
- **Concurrent**: Thread-safe renderer registry and caches

## Testing

```bash
# Run all tests with race detection and coverage
make test

# Run specific package tests
go test ./internal/server -v

# Run integration tests only
go test ./internal/server -run Integration -v

# View coverage in browser (after make test)
go tool cover -html=cover.out
```

The coverage target is 87%+. `make test` prints the per-function and total coverage, excluding `internal/testutil/`
and `docs/skills/` (see `.covignore`).

## Troubleshooting

### Directory URL redirects or shows 403 Forbidden

A directory without its `default_index` file redirects to the first page, or answers 403 when there is none.
Enable generated listings:

```bash
GOMDDOC_SITE_DIR_INDEX=true gomddoc serve
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
# This fails: a markdown file cannot be served as JSON
curl -H "Accept: application/json" http://localhost:8080/docs

# Use */* to accept any format
curl -H "Accept: */*" http://localhost:8080/docs
```

### Configuration problems

```bash
gomddoc doctor ./docs    # Every configuration and content problem, each with a fix
```

### Common Issues

**Port already in use:**

```bash
gomddoc serve -p :8081
gomddoc serve -p :auto   # first free port from 8080
```

**Permission denied:**

```bash
chmod 644 *.md
chmod 755 $(find . -type d)
```

**Template or content edits not showing:**

```bash
gomddoc preview        # Dev mode: render cache off, templates re-parsed on every request
GOMDDOC_SERVER_DEV_MODE=true gomddoc serve
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Run `make ci` (codefix, format, lint, tests)
4. Submit a pull request

All contributions must keep test coverage at 87% or above.

## License

gomddoc is **dual-licensed**:

- **Noncommercial use** (personal projects, hobby use, education, research, and other noncommercial
  purposes) is free under the [PolyForm Noncommercial License 1.0.0](LICENSE).
- **Commercial use** — any use that is not a noncommercial purpose, including use in or for a
  for-profit business, product, or service — requires a separate commercial license. See
  [LICENSE-COMMERCIAL.md](LICENSE-COMMERCIAL.md).

## Changelog

Release notes for each version are on [GitHub Releases](https://github.com/monolithiclab/gomddoc/releases),
generated by GoReleaser from conventional commit messages.
