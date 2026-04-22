# Custom Renderers Guide

This guide shows how to create custom renderers for gomddoc to support additional content types and transformations.

## Table of Contents

- [Basic Renderer](#basic-renderer)
- [JSON Pretty-Print Renderer](#json-pretty-print-renderer)
- [Syntax Highlighting Renderer](#syntax-highlighting-renderer)
- [AsciiDoc Renderer](#asciidoc-renderer)
- [SVG to PNG Renderer](#svg-to-png-renderer)
- [Testing Custom Renderers](#testing-custom-renderers)

## ContentRenderer Interface

All custom renderers must implement this interface:

```go
type ContentRenderer interface {
    // SupportedMimeTypes returns MIME types this renderer can process
    // Use normalized MIME types (no charset parameters)
    SupportedMimeTypes() []string

    // Render processes content
    // Returns: (output content, output MIME type, error)
    // Empty output MIME type means "use input MIME type" (passthrough)
    Render(ctx context.Context, content []byte) ([]byte, string, error)
}
```

## Basic Renderer

Minimal renderer that uppercases text files:

```go
package main

import (
    "bytes"
    "context"
    "github.com/monolithiclab/gomddoc/internal/renderer"
)

type UppercaseRenderer struct{}

func NewUppercaseRenderer() *UppercaseRenderer {
    return &UppercaseRenderer{}
}

func (u *UppercaseRenderer) SupportedMimeTypes() []string {
    return []string{"text/plain"}
}

func (u *UppercaseRenderer) Render(ctx context.Context, content []byte) ([]byte, string, error) {
    // Always check context first
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Transform content
    upper := bytes.ToUpper(content)

    // Return: content, output MIME type, error
    return upper, "text/plain; charset=utf-8", nil
}

// Register in main.go:
// registry.Register(NewUppercaseRenderer())
```

## JSON Pretty-Print Renderer

Formats JSON files with syntax highlighting:

```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "html"
)

type JSONRenderer struct{}

func NewJSONRenderer() *JSONRenderer {
    return &JSONRenderer{}
}

func (j *JSONRenderer) SupportedMimeTypes() []string {
    return []string{"application/json"}
}

func (j *JSONRenderer) Render(ctx context.Context, content []byte) ([]byte, string, error) {
    // Check context
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Validate JSON
    var data interface{}
    if err := json.Unmarshal(content, &data); err != nil {
        return nil, "", fmt.Errorf("invalid JSON: %w", err)
    }

    // Check context before expensive operation
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Pretty-print with indentation
    formatted, err := json.MarshalIndent(data, "", "  ")
    if err != nil {
        return nil, "", fmt.Errorf("format JSON: %w", err)
    }

    // Escape for HTML and add styling
    escaped := html.EscapeString(string(formatted))
    output := fmt.Sprintf(`<div class="json-viewer">
<style>
.json-viewer {
    background: #f5f5f5;
    border: 1px solid #ddd;
    border-radius: 4px;
    padding: 16px;
    overflow-x: auto;
}
.json-viewer pre {
    margin: 0;
    font-family: 'Courier New', monospace;
    font-size: 14px;
}
</style>
<pre>%s</pre>
</div>`, escaped)

    return []byte(output), "text/html; charset=utf-8", nil
}

// Register:
// registry.Register(NewJSONRenderer())
```

**Usage:**
```bash
# Create test JSON file
echo '{"name":"John","age":30,"city":"NYC"}' > data.json

# Start server with JSON renderer registered
./build/gomddoc

# Access via browser
curl http://localhost:8080/data.json
# Returns formatted, styled HTML
```

## Syntax Highlighting Renderer

Uses Chroma library for code syntax highlighting:

```go
package main

import (
    "bytes"
    "context"
    "fmt"
    "path/filepath"

    "github.com/alecthomas/chroma/v2"
    "github.com/alecthomas/chroma/v2/formatters/html"
    "github.com/alecthomas/chroma/v2/lexers"
    "github.com/alecthomas/chroma/v2/styles"
)

type CodeRenderer struct{}

func NewCodeRenderer() *CodeRenderer {
    return &CodeRenderer{}
}

func (c *CodeRenderer) SupportedMimeTypes() []string {
    return []string{
        "text/x-python",
        "text/x-go",
        "text/x-java",
        "text/x-c",
        "text/x-rust",
        "application/javascript",
    }
}

func (c *CodeRenderer) Render(ctx context.Context, content []byte) ([]byte, string, error) {
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Auto-detect language
    lexer := lexers.Analyse(string(content))
    if lexer == nil {
        lexer = lexers.Fallback
    }

    // Use GitHub style
    style := styles.Get("github")
    if style == nil {
        style = styles.Fallback
    }

    // HTML formatter with line numbers
    formatter := html.New(
        html.WithLineNumbers(true),
        html.WithClasses(true),
        html.TabWidth(4),
    )

    // Check context before expensive operation
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Tokenize
    iterator, err := lexer.Tokenise(nil, string(content))
    if err != nil {
        return nil, "", fmt.Errorf("tokenize: %w", err)
    }

    // Format to HTML
    var buf bytes.Buffer
    buf.WriteString("<style>")
    err = formatter.WriteCSS(&buf, style)
    if err != nil {
        return nil, "", fmt.Errorf("write CSS: %w", err)
    }
    buf.WriteString("</style>")

    err = formatter.Format(&buf, style, iterator)
    if err != nil {
        return nil, "", fmt.Errorf("format: %w", err)
    }

    return buf.Bytes(), "text/html; charset=utf-8", nil
}

// Register:
// registry.Register(NewCodeRenderer())
```

**Installation:**
```bash
go get github.com/alecthomas/chroma/v2
```

## AsciiDoc Renderer

Converts AsciiDoc to HTML:

```go
package main

import (
    "bytes"
    "context"
    "fmt"
    "os/exec"
)

type AsciiDocRenderer struct{}

func NewAsciiDocRenderer() *AsciiDocRenderer {
    return &AsciiDocRenderer{}
}

func (a *AsciiDocRenderer) SupportedMimeTypes() []string {
    return []string{"text/asciidoc"}
}

func (a *AsciiDocRenderer) Render(ctx context.Context, content []byte) ([]byte, string, error) {
    // Check context
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Call asciidoctor CLI
    cmd := exec.CommandContext(ctx, "asciidoctor", "-o", "-", "-")
    cmd.Stdin = bytes.NewReader(content)

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    // Execute with context (respects cancellation)
    if err := cmd.Run(); err != nil {
        if ctx.Err() != nil {
            return nil, "", ctx.Err()
        }
        return nil, "", fmt.Errorf("asciidoctor failed: %w (%s)", err, stderr.String())
    }

    return stdout.Bytes(), "text/html; charset=utf-8", nil
}

// Register:
// registry.Register(NewAsciiDocRenderer())
```

**Prerequisites:**
```bash
# Install asciidoctor
gem install asciidoctor

# Or use Docker
docker pull asciidoctor/docker-asciidoctor
```

## SVG to PNG Renderer

Converts SVG images to PNG for better compatibility:

```go
package main

import (
    "bytes"
    "context"
    "fmt"
    "image/png"

    "github.com/tdewolff/canvas"
    "github.com/tdewolff/canvas/renderers/rasterizer"
)

type SVGToPNGRenderer struct {
    width  int
    height int
}

func NewSVGToPNGRenderer(width, height int) *SVGToPNGRenderer {
    return &SVGToPNGRenderer{
        width:  width,
        height: height,
    }
}

func (s *SVGToPNGRenderer) SupportedMimeTypes() []string {
    return []string{"image/svg+xml"}
}

func (s *SVGToPNGRenderer) Render(ctx context.Context, content []byte) ([]byte, string, error) {
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Parse SVG
    c, err := canvas.ParseSVG(bytes.NewReader(content))
    if err != nil {
        return nil, "", fmt.Errorf("parse SVG: %w", err)
    }

    // Check context before expensive operation
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Render to raster
    img := rasterizer.Draw(c, canvas.DPMM(3.0), canvas.DefaultColorSpace)

    // Encode to PNG
    var buf bytes.Buffer
    if err := png.Encode(&buf, img); err != nil {
        return nil, "", fmt.Errorf("encode PNG: %w", err)
    }

    return buf.Bytes(), "image/png", nil
}

// Register:
// registry.Register(NewSVGToPNGRenderer(1920, 1080))
```

**Installation:**
```bash
go get github.com/tdewolff/canvas
```

## CSV to HTML Table Renderer

Converts CSV files to HTML tables:

```go
package main

import (
    "bytes"
    "context"
    "encoding/csv"
    "fmt"
    "html"
)

type CSVRenderer struct{}

func NewCSVRenderer() *CSVRenderer {
    return &CSVRenderer{}
}

func (c *CSVRenderer) SupportedMimeTypes() []string {
    return []string{"text/csv"}
}

func (c *CSVRenderer) Render(ctx context.Context, content []byte) ([]byte, string, error) {
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Parse CSV
    reader := csv.NewReader(bytes.NewReader(content))
    records, err := reader.ReadAll()
    if err != nil {
        return nil, "", fmt.Errorf("parse CSV: %w", err)
    }

    if len(records) == 0 {
        return []byte("<p>Empty CSV file</p>"), "text/html; charset=utf-8", nil
    }

    // Check context
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Build HTML table
    var buf bytes.Buffer
    buf.WriteString(`<style>
table { border-collapse: collapse; width: 100%; margin: 20px 0; }
th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
th { background-color: #f5f5f5; font-weight: bold; }
tr:nth-child(even) { background-color: #f9f9f9; }
</style>
<table>
<thead><tr>`)

    // Header row
    for _, cell := range records[0] {
        fmt.Fprintf(&buf, "<th>%s</th>", html.EscapeString(cell))
    }
    buf.WriteString("</tr></thead><tbody>")

    // Data rows
    for _, row := range records[1:] {
        buf.WriteString("<tr>")
        for _, cell := range row {
            fmt.Fprintf(&buf, "<td>%s</td>", html.EscapeString(cell))
        }
        buf.WriteString("</tr>")
    }

    buf.WriteString("</tbody></table>")

    return buf.Bytes(), "text/html; charset=utf-8", nil
}

// Register:
// registry.Register(NewCSVRenderer())
```

## Testing Custom Renderers

### Unit Test Template

```go
package main

import (
    "context"
    "testing"
    "time"
)

func TestJSONRenderer_SupportedMimeTypes(t *testing.T) {
    renderer := NewJSONRenderer()
    types := renderer.SupportedMimeTypes()

    if len(types) != 1 {
        t.Errorf("expected 1 MIME type, got %d", len(types))
    }

    if types[0] != "application/json" {
        t.Errorf("expected application/json, got %s", types[0])
    }
}

func TestJSONRenderer_Render(t *testing.T) {
    renderer := NewJSONRenderer()

    tests := []struct {
        name        string
        input       []byte
        expectError bool
        checkOutput func([]byte, string, error) error
    }{
        {
            name:        "valid JSON",
            input:       []byte(`{"name":"John","age":30}`),
            expectError: false,
            checkOutput: func(output []byte, mimeType string, err error) error {
                if err != nil {
                    return err
                }
                if mimeType != "text/html; charset=utf-8" {
                    t.Errorf("expected text/html, got %s", mimeType)
                }
                if !bytes.Contains(output, []byte("John")) {
                    t.Error("output should contain 'John'")
                }
                return nil
            },
        },
        {
            name:        "invalid JSON",
            input:       []byte(`{invalid`),
            expectError: true,
        },
        {
            name:        "empty input",
            input:       []byte(`null`),
            expectError: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := context.Background()
            output, mimeType, err := renderer.Render(ctx, tt.input)

            if tt.expectError && err == nil {
                t.Error("expected error, got nil")
            }

            if !tt.expectError && err != nil {
                t.Errorf("unexpected error: %v", err)
            }

            if tt.checkOutput != nil {
                if err := tt.checkOutput(output, mimeType, err); err != nil {
                    t.Errorf("output check failed: %v", err)
                }
            }
        })
    }
}

func TestJSONRenderer_ContextCancellation(t *testing.T) {
    renderer := NewJSONRenderer()

    // Create cancelled context
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    _, _, err := renderer.Render(ctx, []byte(`{"test":true}`))
    if err != context.Canceled {
        t.Errorf("expected context.Canceled, got %v", err)
    }
}

func TestJSONRenderer_ContextTimeout(t *testing.T) {
    renderer := NewJSONRenderer()

    // Very short timeout
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
    defer cancel()

    time.Sleep(10 * time.Millisecond)

    _, _, err := renderer.Render(ctx, []byte(`{"test":true}`))
    if err == nil {
        t.Error("expected timeout error")
    }
}
```

### Integration Test

```go
func TestJSONRenderer_Integration(t *testing.T) {
    // Create temporary directory with test JSON
    testDir := t.TempDir()
    jsonFile := filepath.Join(testDir, "data.json")
    err := os.WriteFile(jsonFile, []byte(`{"users":[{"name":"Alice"},{"name":"Bob"}]}`), 0644)
    if err != nil {
        t.Fatalf("failed to create test file: %v", err)
    }

    // Setup gomddoc components
    siteConfig := config.NewSiteConfig(testDir)
    prov, err := provider.NewFilesystemProvider(testDir, "README.md", false)
    if err != nil {
        t.Fatalf("failed to create provider: %v", err)
    }
    defer prov.Close()

    // Register custom renderer
    registry := renderer.NewDefaultRegistry()
    registry.Register(renderer.NewMarkdownRenderer())
    registry.Register(NewJSONRenderer())  // Custom renderer
    registry.Register(renderer.NewPassthroughRenderer())

    templateRenderer := setupTestRenderer()
    handler := server.NewHandler(prov, registry, templateRenderer, siteConfig)

    // Test request
    req := httptest.NewRequest("GET", "/data.json", nil)
    req.Header.Set("Accept", "text/html")
    w := httptest.NewRecorder()

    handler.ServeContent(w, req)

    // Verify response
    if w.Code != 200 {
        t.Errorf("expected status 200, got %d", w.Code)
    }

    body := w.Body.String()
    if !strings.Contains(body, "Alice") || !strings.Contains(body, "Bob") {
        t.Error("response should contain formatted JSON data")
    }
}
```

## Registration in main.go

```go
package main

import (
    "github.com/monolithiclab/gomddoc/internal/renderer"
    // ... other imports
)

func main() {
    // ... config setup ...

    // Create registry
    registry := renderer.NewDefaultRegistry()

    // Register built-in renderers
    registry.Register(renderer.NewMarkdownRenderer())
    registry.Register(renderer.NewPassthroughRenderer())

    // Register custom renderers
    registry.Register(NewJSONRenderer())
    registry.Register(NewCodeRenderer())
    registry.Register(NewCSVRenderer())

    // ... rest of setup ...
}
```

## Best Practices

### 1. Always Check Context

```go
func (r *MyRenderer) Render(ctx context.Context, content []byte) ([]byte, string, error) {
    // Check at start
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Check before expensive operations
    parsed := parseContent(content)

    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    rendered := renderParsed(parsed)
    return rendered, "text/html; charset=utf-8", nil
}
```

### 2. Return Full MIME Types

```go
// ✅ GOOD: Include charset
return html, "text/html; charset=utf-8", nil

// ❌ BAD: Missing charset (browser might misinterpret)
return html, "text/html", nil
```

### 3. Proper Error Wrapping

```go
// ✅ GOOD: Wrap with %w for error classification
if err != nil {
    return nil, "", fmt.Errorf("parse JSON: %w", err)
}

// ❌ BAD: Loses error context with %v
if err != nil {
    return nil, "", fmt.Errorf("parse JSON: %v", err)
}
```

### 4. Thread Safety

```go
// ✅ GOOD: Stateless renderer
type MyRenderer struct {
    // No mutable state
}

// ❌ BAD: Shared mutable state (not thread-safe)
type MyRenderer struct {
    buffer *bytes.Buffer  // Shared across goroutines!
}
```

### 5. Escape HTML

```go
import "html"

// ✅ GOOD: Escape user content
escaped := html.EscapeString(userInput)

// ❌ BAD: XSS vulnerability
output := "<div>" + userInput + "</div>"
```

## Performance Tips

1. **Avoid Pooling Parsers**: Create fresh instances per render for thread safety
2. **Context Checks**: Add checks before expensive operations
3. **Minimal Allocations**: Reuse buffers where possible (within single render)
4. **Lazy Loading**: Only import heavy dependencies if MIME type registered

## Troubleshooting

### Renderer Not Being Called

Check MIME type registration:
```bash
# Provider must return correct MIME type
# Add to init() if needed:
func init() {
    mime.AddExtensionType(".adoc", "text/asciidoc")
}
```

### 406 Not Acceptable Errors

Ensure output MIME type is acceptable:
```go
// If client accepts text/html, renderer must output text/html
return html, "text/html; charset=utf-8", nil
```

### Context Deadline Exceeded

Large files or slow operations:
```go
// Add periodic context checks
for i, chunk := range largeData {
    if i%100 == 0 {
        if err := ctx.Err(); err != nil {
            return nil, "", err
        }
    }
    process(chunk)
}
```

## Complete Example: Mermaid Diagram Renderer

```go
package main

import (
    "bytes"
    "context"
    "fmt"
    "os/exec"
)

type MermaidRenderer struct{}

func NewMermaidRenderer() *MermaidRenderer {
    return &MermaidRenderer{}
}

func (m *MermaidRenderer) SupportedMimeTypes() []string {
    return []string{"text/vnd.mermaid"}
}

func (m *MermaidRenderer) Render(ctx context.Context, content []byte) ([]byte, string, error) {
    if err := ctx.Err(); err != nil {
        return nil, "", err
    }

    // Wrap in HTML with mermaid.js
    html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <script type="module">
        import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
        mermaid.initialize({ startOnLoad: true });
    </script>
</head>
<body>
    <div class="mermaid">
%s
    </div>
</body>
</html>`, content)

    return []byte(html), "text/html; charset=utf-8", nil
}
```

Usage:
```bash
# Create diagram file
cat > diagram.mmd << 'EOF'
graph TD
    A[Start] --> B[Process]
    B --> C{Decision}
    C -->|Yes| D[End]
    C -->|No| B
EOF

# Start server
./build/gomddoc

# View: http://localhost:8080/diagram.mmd
```

## Resources

- [gomarkdown library](https://github.com/gomarkdown/markdown)
- [Chroma syntax highlighter](https://github.com/alecthomas/chroma)
- [Canvas SVG library](https://github.com/tdewolff/canvas)
- [MIME types reference](https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types)
- [HTTP Accept header](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept)
