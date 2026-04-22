---
title: "Gomddoc Architecture"
description: "High-level architectural overview of the gomddoc HTTP server."
author: "nicolasm"
---

# Gomddoc Architecture

## Overview

Gomddoc is a production-ready HTTP server built with a clean, interface-driven architecture for serving documentation
with automatic content rendering. The system supports both local filesystem and remote Git repositories as content
sources. It is designed for extensibility, testability, and HTTP compliance.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                         HTTP Request                         │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
                  ┌─────────────────────┐
                  │     Middleware       │
                  │  1. Security Headers │
                  │  2. Method Filter    │
                  │  3. Hidden Path Block│
                  └──────────┬──────────┘
                             │
                             ▼
                  ┌─────────────────┐
                  │     Handler     │
                  │  ServeContent() │
                  └────────┬────────┘
                           │
            ┌──────────────┼──────────────┐
            │              │              │
            ▼              ▼              ▼
     ┌───────────┐  ┌───────────┐  ┌────────────┐
     │ Provider  │  │ Registry  │  │  Template  │
     │  ReadFile │  │    Get    │  │   Render   │
     └─────┬─────┘  └─────┬─────┘  └──────┬─────┘
           │              │                │
           │              ▼                │
           │       ┌────────────┐          │
           │       │  Renderer  │          │
           │       │   Render   │          │
           │       └─────┬──────┘          │
           │             │                 │
           └─────────────┼─────────────────┘
                         │
                         ▼
                ┌─────────────────┐
                │  HTTP Response  │
                └─────────────────┘
```

## Core Components

### 1. Provider Layer

**Responsibility:** File I/O, MIME type detection, directory handling

```go
type Provider interface {
    ReadFile(path string) (content []byte, mimeType string, error)
    Stat(path string) (fs.FileInfo, error)
    DefaultIndex() string
    Close() error
}
```

**Implementations:**

- **FilesystemProvider** — Local filesystem via `os.DirFS()` with path traversal protection
- **GitProvider** — Remote Git repositories via go-git with in-memory clone

Both providers share a common `normalizePath()` function for converting request paths to fs-compatible paths.

**Key Features:**
- stdlib `fs.FS` compatible paths (forward slashes, no leading `/`)
- Secure by default (DirIndex=false)
- Hidden file filtering in directory listings
- Directory requests try default index, then optionally generate listings

### 2. Renderer Layer

**Responsibility:** Content transformation with MIME-type routing

```go
type ContentRenderer interface {
    SupportedMimeTypes() []string
    Render(ctx context.Context, content []byte) (*RenderResult, error)
}

type RenderResult struct {
    Content  []byte
    MimeType string
    Metadata map[string]any
    TOC      *TOCNode
}
```

**Built-in Renderers:**

| Renderer              | Input             | Output                       | Template Wrapped |
| --------------------- | ----------------- | ---------------------------- | ---------------- |
| **MarkdownRenderer**  | `text/markdown`   | `text/html; charset=utf-8`   | Yes              |
| **PassthroughRenderer** | `*/*` (wildcard)  | `""` (passthrough signal)   | No               |

### 3. Registry

**Responsibility:** MIME type → renderer mapping with wildcard support

```go
type RendererRegistry interface {
    Register(renderer ContentRenderer)
    Get(mimeType string) (ContentRenderer, error)  // Returns ErrNoRenderer if not found
}
```

**Lookup order:** exact match → `type/*` → `*/*`

Thread-safe with `sync.RWMutex`. Collision detection with `slog.Warn`.

### 4. Handler Layer

**Responsibility:** HTTP orchestration and content negotiation

**Flow:**
1. Read file + get MIME type (Provider)
2. Get renderer from registry (normalized MIME type)
3. Render content (produces output MIME type + metadata + TOC)
4. Content negotiation on OUTPUT MIME type vs Accept header
5. Serve (wrapped in template if HTML, raw otherwise)

### 5. Template Layer

**Responsibility:** HTML template rendering with caching and breadcrumbs

```go
type Renderer interface {
    Render(ctx context.Context, name string, data any) ([]byte, error)
}
```

**Features:**
- Embedded themes via `embed.FS` with overlay filesystem for customization
- Production mode: `CachedTemplateStore` (sync.Map cache)
- Dev mode: `PassthroughTemplateStore` (always re-parse)
- Breadcrumb generation from URL paths
- TOC generation from heading structure
- Buffer pool with 64KB cap to prevent memory bloat
- File handle validation at startup

### 6. Middleware Chain

**Order (outermost to innermost):**

1. **SecurityHeaders** — Sets X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Permissions-Policy, HSTS
2. **MethodFilter** — Returns 405 Method Not Allowed for non-GET/HEAD requests with `Allow` header
3. **BlockHiddenPaths** — Returns 404 for dot-prefixed path segments (except `.well-known`)

## Data Flow

### Markdown File Request

```
1. GET /docs/guide.md
   ↓
2. Middleware (security headers → method filter → hidden path check)
   ↓
3. Handler.ServeContent()
   ↓
4. Provider.ReadFile("/docs/guide.md")
   → content: "# Guide\n\nText..."
   → mimeType: "text/markdown; charset=utf-8"
   ↓
5. Registry.Get("text/markdown")  // normalized
   → MarkdownRenderer
   ↓
6. MarkdownRenderer.Render(ctx, content)
   → RenderResult{Content: "<h1>Guide</h1>...", MimeType: "text/html; charset=utf-8", Metadata: {...}, TOC: {...}}
   ↓
7. Content negotiation: ParseAccept("text/html, */*") vs "text/html"
   → Accepted ✅
   ↓
8. Handler.serveHTML(w, r, result)
   → Template.Render("layout.html.tmpl", {Site, Page{Content, Meta, TOC, Path}})
   → Response: Templated HTML with breadcrumbs, title, TOC
```

### Non-HTML File Request

```
1. GET /images/logo.png
   ↓
2. Middleware chain
   ↓
3. Provider.ReadFile("/images/logo.png")
   → content: [binary data], mimeType: "image/png"
   ↓
4. Registry.Get("image/png") → PassthroughRenderer (via */* wildcard)
   ↓
5. PassthroughRenderer.Render() → same content, mimeType: "" (passthrough)
   ↓
6. Handler.serveRaw(w, content, "image/png")
   → Response: PNG with Content-Type, Content-Length, Cache-Control
```

## Error Handling

### Error Classification

All provider and renderer errors are mapped to HTTP status codes via `classifyError()`:

| Error                          | HTTP Status | Code |
| ------------------------------ | ----------- | ---- |
| `provider.ErrDirListingDisabled` | Forbidden | 403  |
| `os.ErrNotExist`, `provider.ErrNotFound` | Not Found | 404 |
| `fs.ErrPermission`            | Forbidden   | 403  |
| `context.Canceled`            | Client Closed | 499 |
| `context.DeadlineExceeded`    | Gateway Timeout | 504 |
| `provider.ErrInvalidGitURL`   | Bad Request | 400  |
| `provider.ErrGitAuthFailed`   | Unauthorized | 401 |
| `provider.ErrGitConnectFailed` | Bad Gateway | 502 |
| `provider.ErrGitRefNotFound`  | Not Found   | 404  |
| `provider.ErrGitLFSNotSupported` | Not Implemented | 501 |
| `provider.ErrFileTooLarge`    | Request Entity Too Large | 413 |
| `renderer.ErrNoRenderer`      | Unsupported Media Type | 415 |
| _(default)_                   | Internal Server Error | 500 |

All errors use `errors.Is()` for classification, supporting wrapped errors via `fmt.Errorf("%w", err)`.

### Custom Error Type

```go
type PathError struct {
    Op   string  // "read", "stat", "clone", etc.
    Path string
    Err  error   // Underlying sentinel error
}
```

## MIME Type Handling

### Normalization

`NormalizeMimeType()` strips charset/parameters for routing, but the full MIME type is preserved for HTTP headers.

- `"text/html; charset=utf-8"` → `"text/html"` (for routing)
- Full type preserved in `Content-Type` header

### Registry Wildcard Matching

1. Exact match: `"text/markdown"`
2. Type wildcard: `"text/*"`
3. Catch-all: `"*/*"`

## Security

- **Path traversal**: `os.DirFS()` jails file access
- **Hidden files**: Middleware blocks dot-prefixed paths (except `.well-known`)
- **Method filtering**: Only GET and HEAD allowed (405 for others)
- **Security headers**: nosniff, DENY framing, referrer policy, permissions policy, HSTS
- **Git SSH**: Host key verification via known_hosts (fail closed, no TOFU)
- **Clone timeout**: Enforced via `context.WithTimeout` (default 60s)
- **File size limits**: Git provider enforces 50MB max (configurable)
- **Log injection**: `text.SafeString` sanitizes user input in log messages

## Configuration

### Loading Priority (highest wins)

1. CLI flags (`-d`, `-p`, `-dev`, `--git-key-file`)
2. Environment variables (`GOMDDOC_SERVER_*`, `GOMDDOC_SITE_*`)
3. Config file (`.gomddoc/config.yml`)
4. Defaults

### Config Structure

```go
type Config struct {
    Server ServerConfig `env:"SERVER"`
    Site   SiteConfig   `env:"SITE"`
}

type ServerConfig struct {
    Port      string     `env:"PORT"`        // ":8080"
    DevMode   bool       `env:"DEV_MODE"`    // false
    Dir       string     `env:"DIR"`         // "."
    GitSSHKey string     `env:"GIT_SSH_KEY"` // ""
    HTTP      HTTPConfig `env:"HTTP"`        // Timeout tuning
}

type SiteConfig struct {
    DefaultIndex string      `env:"DEFAULT_INDEX" yaml:"default_index"` // "README.md"
    DirIndex     bool        `env:"DIR_INDEX"     yaml:"dir_index"`     // false
    Meta         MetaConfig  `env:"META"          yaml:"meta"`
    Theme        ThemeConfig `env:"THEME"         yaml:"theme"`
}
```

Environment variables are applied via reflection-based walking of the struct tree with `env` tags.

## Testing

### Coverage (as of 2026-03-09)

| Package                    | Coverage |
| -------------------------- | -------- |
| internal/common            | 100.0%   |
| internal/renderer          | 96.8%    |
| internal/text              | 95.2%    |
| internal/template          | 93.3%    |
| internal/server            | 92.8%    |
| internal/template/breadcrumb | 92.0% |
| internal/config            | 82.4%    |
| internal/assets            | 80.8%    |
| internal/provider          | 67.5%    |
| **Overall**                | **78.1%** |

### Test Strategy

- **Unit tests**: Package-level isolation with interfaces for mocking
- **Table-driven tests**: All components use `[]struct{...}` test tables
- **Parallel tests**: `t.Parallel()` where possible
- **Integration tests**: End-to-end request flow with real Provider + Registry + Handler
- **Context cancellation**: Tested in renderers and template layer

## Extensibility

### Adding Custom Renderers

See [Custom Renderers Guide](custom-renderers.md) for complete examples. In brief:

```go
type MyRenderer struct{}
func (r *MyRenderer) SupportedMimeTypes() []string { return []string{"text/x-custom"} }
func (r *MyRenderer) Render(ctx context.Context, content []byte) (*renderer.RenderResult, error) {
    // transform content...
    return &renderer.RenderResult{Content: output, MimeType: "text/html; charset=utf-8"}, nil
}

// Register in main.go:
registry.Register(&MyRenderer{})
```

### Adding Custom Providers

Implement the `Provider` interface. The `NewProvider()` factory auto-detects Git URLs vs filesystem paths.

## Design Decisions

| Decision | Rationale |
| -------- | --------- |
| MIME-type routing | Universal, stdlib-backed, natural fit with HTTP Accept negotiation |
| Separate Provider/Renderer | I/O vs transformation separation; mock either independently |
| PassthroughRenderer `*/*` | No maintenance when new file types added; guarantees all types handled |
| No context in Provider | stdlib `fs.FS` compatibility; timeout at HTTP level |
| `RenderResult` struct | Extensible return (content + metadata + TOC) without interface churn |
| `path` not `filepath` for fs.FS | `io/fs` spec requires forward slashes; `filepath` breaks on Windows |
| Clone timeout via context | Standard Go pattern; `git.CloneContext()` respects cancellation |
| SSH fail-closed | Security: no TOFU fallback; require known_hosts for host key verification |

## Glossary

- **Provider**: Reads files and detects MIME types (filesystem or Git)
- **Renderer**: Transforms content from one MIME type to another
- **Registry**: MIME type → renderer mapping with wildcard support
- **Handler**: HTTP request orchestrator
- **MIME Normalization**: Stripping charset parameters for routing decisions
- **Content Negotiation**: Matching server output to client Accept header
- **Passthrough**: Serving content unchanged with original MIME type
- **Template Wrapping**: Adding HTML layout around rendered content
- **TOC**: Table of Contents extracted from heading structure
- **Overlay FS**: Layered filesystem where user assets override embedded defaults
