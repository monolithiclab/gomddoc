# Gomddoc Architecture

## Overview

Gomddoc is a production-ready HTTP server built with a clean, interface-driven architecture for serving documentation with automatic content rendering. The system is designed for extensibility, testability, and HTTP compliance.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                         HTTP Request                         │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
                  ┌─────────────────┐
                  │   Middleware    │
                  │   - Security    │
                  │   - Hidden Path │
                  └────────┬────────┘
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

**Responsibility:** File I/O and MIME type detection

```go
type Provider interface {
    ReadFile(path string) (content []byte, mimeType string, error)
    Stat(path string) (fs.FileInfo, error)
    DefaultIndex() string
    Close() error
}
```

**Implementation:** `FilesystemProvider`
- Uses `os.DirFS()` for path traversal protection
- Detects MIME types via `mime.TypeByExtension()`
- Handles directory requests with README.md fallback
- Generates directory listings when enabled

**Key Features:**
- stdlib `fs.FS` compatible (no context parameter)
- Type assertion to `fs.StatFS` for `Stat()` method
- Secure by default (DirIndex=false)
- Hidden file filtering in directory listings

### 2. Renderer Layer

**Responsibility:** Content transformation with MIME-type routing

```go
type ContentRenderer interface {
    SupportedMimeTypes() []string
    Render(ctx context.Context, content []byte) ([]byte, string, error)
}
```

**Built-in Renderers:**

**MarkdownRenderer**
- Input: `text/markdown`
- Output: `text/html; charset=utf-8`
- Uses gomarkdown with CommonExtensions + AutoHeadingIDs
- Context-aware (checks cancellation before/after parsing)
- Thread-safe (stateless, creates fresh parser per render)

**PassthroughRenderer**
- Input: `*/*` (wildcard)
- Output: `""` (empty = passthrough signal)
- Returns content unchanged
- Catch-all for unregistered MIME types

### 3. Registry

**Responsibility:** MIME type → renderer mapping with wildcard support

```go
type RendererRegistry interface {
    Register(renderer ContentRenderer)
    Get(mimeType string) (ContentRenderer, error)
}
```

**Features:**
- Self-declaring renderers (no hardcoded mappings)
- Wildcard matching: exact → `type/*` → `*/*`
- Collision detection with `slog.Warn`
- Thread-safe with `sync.RWMutex`
- MIME normalization before lookup

### 4. Handler Layer

**Responsibility:** HTTP orchestration and content negotiation

**Flow:**
1. Read file + get MIME type (Provider)
2. Parse Accept header
3. Get renderer from registry (normalized MIME type)
4. Render content
5. Content negotiation on OUTPUT MIME type
6. Serve (wrapped in template if HTML, raw otherwise)

**Key Methods:**
- `ServeContent(w, r)`: Main request handler
- `serveHTML(w, r, content)`: Template wrapping for HTML
- `serveRaw(w, content, mimeType)`: Direct serving for other types
- `handleError(w, r, err, path)`: Error classification and responses

### 5. Template Layer

**Responsibility:** HTML template rendering with caching

```go
type Renderer interface {
    Render(ctx context.Context, name string, data any) ([]byte, error)
    ClearCache()
    ValidateDefaultTheme() error
}
```

**Features:**
- Embedded themes via `embed.FS`
- Production mode: `CachedTemplateStore` (LRU cache)
- Dev mode: `NoCacheStore` (always fresh)
- Breadcrumb generation from URL paths
- Functional options pattern for configuration

## Data Flow

### Markdown File Request

```
1. GET /docs/guide.md
   ↓
2. Middleware (security headers, hidden path blocking)
   ↓
3. Handler.ServeContent()
   ↓
4. Provider.ReadFile("/docs/guide.md")
   → content: "# Guide\n\nText..."
   → mimeType: "text/markdown; charset=utf-8"
   ↓
5. ParseAccept("text/html, */*")
   → acceptedTypes: [{text/html, q=1.0}, {*/*, q=1.0}]
   ↓
6. Registry.Get("text/markdown")  // normalized
   → MarkdownRenderer
   ↓
7. MarkdownRenderer.Render(ctx, content)
   → output: "<h1 id=\"guide\">Guide</h1>\n<p>Text...</p>"
   → outputMimeType: "text/html; charset=utf-8"
   ↓
8. Content negotiation on "text/html"
   → Accepted ✅ (matches Accept: text/html)
   ↓
9. Handler.serveHTML(w, r, output)
   → Template.Render("layout.html.tmpl", {Site, Page})
   → Response: Templated HTML with breadcrumbs, title, etc.
```

### Image File Request

```
1. GET /images/logo.png
   ↓
2. Middleware (security headers)
   ↓
3. Handler.ServeContent()
   ↓
4. Provider.ReadFile("/images/logo.png")
   → content: [binary PNG data]
   → mimeType: "image/png"
   ↓
5. ParseAccept("*/*")
   → acceptedTypes: [{*/*, q=1.0}]
   ↓
6. Registry.Get("image/png")  // normalized
   → PassthroughRenderer (via */* wildcard)
   ↓
7. PassthroughRenderer.Render(ctx, content)
   → output: [same binary data]
   → outputMimeType: ""  (empty = passthrough)
   ↓
8. finalMimeType = mimeType  // preserve original
   ↓
9. Content negotiation on "image/png"
   → Accepted ✅ (matches Accept: */*)
   ↓
10. Handler.serveRaw(w, output, "image/png")
    → Response: PNG with proper Content-Type, Cache-Control
```

### Directory Request (DirIndex=true)

```
1. GET /docs/
   ↓
2. Middleware
   ↓
3. Handler.ServeContent()
   ↓
4. Provider.ReadFile("/docs")
   ↓
5. Provider.handleDirectory("/docs", "/docs")
   ↓
6. Try: fs.ReadFile("docs/README.md")
   → Not found
   ↓
7. Check: dirIndex == true ✅
   ↓
8. fs.ReadDir("docs/")
   → entries: [guide.md, api.md, images/]
   ↓
9. GenerateMarkdownListing("/docs", entries)
   → content: "# Index of /docs\n\n- [images/](images/)\n- [api.md](api.md)\n- [guide.md](guide.md)"
   → mimeType: "text/markdown; charset=utf-8"
   ↓
10. [Continue as markdown file...]
```

## MIME Type Handling

### Normalization

```go
func NormalizeMimeType(mimeType string) string {
    mediaType, _, err := mime.ParseMediaType(mimeType)
    if err != nil {
        return mimeType  // malformed - return as-is
    }
    return mediaType
}
```

**Examples:**
- `"text/html; charset=utf-8"` → `"text/html"`
- `"application/json"` → `"application/json"`
- `"invalid"` → `"invalid"` (passthrough)

**Usage:**
- Before registry lookup
- Before `text/html` comparison
- After rendering for final MIME type decision

**Preservation:**
- Full MIME type (with charset) used in HTTP `Content-Type` header
- Normalization only for routing decisions

### Registry Wildcard Matching

**Lookup Order:**
1. Exact match: `"text/markdown"`
2. Type wildcard: `"text/*"`
3. Catch-all: `"*/*"`

**Example:**
```
Request MIME: "application/pdf"

1. Check registry["application/pdf"]  → not found
2. Check registry["application/*"]     → not found
3. Check registry["*/*"]               → PassthroughRenderer ✅
```

## HTTP Content Negotiation

### Accept Header Parsing

```go
type MediaType struct {
    Type    string   // "text"
    Subtype string   // "html"
    Q       float64  // Quality factor (0.0-1.0)
    Full    string   // "text/html"
}
```

**Features:**
- Parses q-values: `Accept: text/html;q=0.9, application/json;q=1.0`
- Stable sort preserves client preference for equal q-values
- Defaults to `*/*` if header missing
- Validates with `mime.ParseMediaType()`

### Negotiation Flow

**IMPORTANT:** Negotiation happens on OUTPUT MIME type, not input.

```go
// ❌ WRONG: Negotiate on input type
content, mimeType, _ := provider.ReadFile(path)
if !acceptable(mimeType) {
    return 406
}
output, _, _ := renderer.Render(ctx, content)

// ✅ CORRECT: Negotiate on output type
content, mimeType, _ := provider.ReadFile(path)
renderer, _ := registry.Get(mimeType)
output, outputMimeType, _ := renderer.Render(ctx, content)
if !acceptable(outputMimeType) {  // Check AFTER rendering
    return 406
}
```

**Why?** Markdown files produce HTML. Client accepts HTML, not markdown.

## Error Handling

### Sentinel Errors

```go
// internal/provider/errors.go
var (
    ErrDirListingDisabled = errors.New("directory listing disabled")
    ErrNotFound           = errors.New("not found")
)
```

### Error Classification

```go
func classifyError(err error) int {
    switch {
    case errors.Is(err, provider.ErrDirListingDisabled):
        return http.StatusForbidden  // 403
    case errors.Is(err, os.ErrNotExist):
        return http.StatusNotFound  // 404
    case errors.Is(err, fs.ErrPermission):
        return http.StatusForbidden  // 403
    case errors.Is(err, context.Canceled):
        return 499  // Client closed request
    case errors.Is(err, context.DeadlineExceeded):
        return http.StatusGatewayTimeout  // 504
    default:
        return http.StatusInternalServerError  // 500
    }
}
```

### Error Wrapping

All errors wrapped with `fmt.Errorf("%w", err)` for proper classification:

```go
// ✅ CORRECT
if err != nil {
    return fmt.Errorf("read file %s: %w", path, err)
}

// ❌ WRONG (breaks error classification)
if err != nil {
    return fmt.Errorf("read file %s: %v", path, err)
}
```

## Security

### Path Traversal Protection

```go
fsys := os.DirFS(dir)  // Jails file access to dir
content, err := fs.ReadFile(fsys, cleanPath)
```

**Protection:**
- `os.DirFS()` prevents access outside root directory
- `..` and absolute paths automatically blocked
- Works across all OS (Windows, Linux, macOS)

### Hidden File Blocking

```go
func BlockHiddenPaths(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        parts := strings.Split(r.URL.Path, "/")
        for _, part := range parts {
            if strings.HasPrefix(part, ".") && part != "." && !strings.HasPrefix(r.URL.Path, "/.well-known/") {
                http.Error(w, "403 Forbidden", http.StatusForbidden)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}
```

**Blocks:**
- `.git/`, `.env`, `.config/`, `.gomddoc/`
- Any file starting with `.` at any path depth

**Allows:**
- `/.well-known/` (IETF RFC 8615 compliance)

### Security Headers

```go
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-Frame-Options", "DENY")
```

### Secure Defaults

- `DirIndex: false` - No directory listing by default
- `ShutdownTimeout: 1s` - Prevents hung connections
- `ReadHeaderTimeout: 5s` - Prevents slow-loris attacks

## Configuration

### Loading Priority

```
1. Defaults (config.New())
2. Config file (.gomddoc/config.yml)
3. Environment variables
4. CLI flags (highest priority)
```

### Config Structure

```go
type Config struct {
    Dir             string
    Port            string
    ShutdownTimeout time.Duration
    DevMode         bool
    Server          *ServerConfig
    Site            *SiteConfig
}

type ServerConfig struct {
    DefaultIndex string  // "README.md"
    DirIndex     bool    // false (secure by default)
}

type SiteConfig struct {
    Meta struct {
        Title       string
        Description string
        Author      string
        Keywords    string
    }
    BaseURL string
}
```

## Testing Strategy

### Unit Tests

- Package-level isolation
- Mock dependencies via interfaces
- Table-driven tests for edge cases
- Context cancellation testing

### Integration Tests

- End-to-end request flow
- Temporary directories for file fixtures
- Real Provider + Registry + Handler
- All content types covered

### Coverage Targets

- Overall: 78.9%
- Server: 87.0%
- Renderer: 97.8%
- Provider: 89.5%

## Performance Considerations

### Template Caching

**Production Mode:**
```go
cache := &CachedTemplateStore{}  // LRU cache
renderer.Configure(WithCache(cache))
```

**Dev Mode:**
```go
// No cache - always fresh templates
renderer.Configure(WithCache(&NoCacheStore{}))
```

### Stateless Renderers

All renderers are stateless for thread safety:
- No shared mutable state
- Can be called concurrently
- Fresh parser instance per render (library constraint)

### Efficient MIME Detection

```go
// Fast path: extension-based
mimeType := mime.TypeByExtension(filepath.Ext(path))
if mimeType == "" {
    mimeType = "application/octet-stream"  // Safe default
}
```

## Extensibility

### Adding Custom Renderers

```go
// 1. Implement ContentRenderer interface
type AsciiDocRenderer struct{}

func (a *AsciiDocRenderer) SupportedMimeTypes() []string {
    return []string{"text/asciidoc"}
}

func (a *AsciiDocRenderer) Render(ctx context.Context, content []byte) ([]byte, string, error) {
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    html := convertAsciiDocToHTML(content)
    return html, "text/html; charset=utf-8", nil
}

// 2. Register in main.go
registry.Register(&AsciiDocRenderer{})
```

### Adding Custom Providers

```go
// Implement Provider interface
type S3Provider struct {
    client *s3.Client
    bucket string
}

func (s *S3Provider) ReadFile(path string) ([]byte, string, error) {
    obj, err := s.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: &s.bucket,
        Key:    &path,
    })
    // ...
}
```

## Future Enhancements

### Phase 3: Advanced Features

- [ ] Multiple provider support (S3, database, GitHub)
- [ ] Plugin architecture with dynamic loading
- [ ] REST/GraphQL API for headless CMS
- [ ] i18n support in templates
- [ ] Full-text search
- [ ] Syntax highlighting for code blocks
- [ ] Live reload in dev mode

### Architectural Readiness

The current architecture supports all Phase 3 features without breaking changes:
- Provider abstraction ready for multiple backends
- Renderer registry supports dynamic registration
- Handler independent of provider/renderer implementation
- Template system ready for theme switching

## Design Decisions

### Why MIME-Type Based Routing?

- **Universal**: Works for all content types
- **Stdlib**: Go's `mime` package is robust and tested
- **HTTP Compliant**: Natural fit with Accept header negotiation
- **Self-Documenting**: Renderer declares what it handles

### Why Separate Provider and Renderer?

- **Separation of Concerns**: I/O vs transformation
- **Testability**: Mock filesystem without touching renderer
- **Extensibility**: Replace either independently
- **Stdlib Compatibility**: Provider uses `fs.FS` interface

### Why PassthroughRenderer Uses `*/*`?

- **Simplicity**: Single wildcard instead of explicit type list
- **Maintainability**: No updates needed when new file types added
- **Fail-Safe**: Guarantees all content types are handled

### Why No Context in Provider.ReadFile()?

- **stdlib Compatibility**: `fs.FS` interface has no context
- **Simplicity**: File reads are fast, timeout at HTTP level
- **Flexibility**: Works with any `fs.FS` implementation

## Glossary

- **Provider**: Component responsible for reading files and detecting MIME types
- **Renderer**: Component that transforms content from one MIME type to another
- **Registry**: MIME type → renderer mapping with wildcard support
- **Handler**: HTTP request orchestrator
- **MIME Normalization**: Stripping charset parameters for routing decisions
- **Content Negotiation**: Matching server output to client Accept header
- **Passthrough**: Serving content unchanged with original MIME type
- **Template Wrapping**: Adding HTML layout around rendered content
