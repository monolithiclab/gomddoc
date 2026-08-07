---
title: "Custom Renderers"
description: "Adding a content renderer to the gomddoc source tree: the ContentRenderer contract, two-dimensional negotiation, registration, and testing."
author: "nicolasm"
---

# Custom Renderers

A **renderer** turns the bytes of one MIME type into the bytes of another. Markdown to HTML is the
one you already use; the same interface covers CSV to an HTML table, AsciiDoc to HTML, or JSON to a
pretty-printed page.

> **This is a source-tree change, not a plugin.** `internal/renderer` is an internal package, so no
> code outside this module can import it, and renderers are registered in `setupPipeline`
> (`cmd/gomddoc/pipeline.go`) — a `package main` you do not control. Adding a renderer means forking
> gomddoc or contributing upstream. There is no runtime plugin mechanism, and adding one is not on
> the [roadmap](../../roadmap.md).

## The ContentRenderer contract

```go
type ContentRenderer interface {
    InputMimeTypes() []string
    OutputMimeTypes() []string
    Render(ctx context.Context, content []byte, enrichment *enricher.EnrichmentData) (*RenderResult, error)
}

type RenderResult struct {
    Content  []byte // the transformed bytes
    MimeType string // output type; empty means "keep the input type" (passthrough)
}
```

Three things follow from the shape of this interface:

- **Renderers declare both directions.** The registry maps input type → renderer without any
  hardcoded configuration, and it needs the output types to answer an `Accept` header. This is why
  the same `.md` file can be served as HTML to a browser and as `text/markdown` to an agent.
- **Renderers are stateless and used concurrently.** One instance serves every request. Do not
  accumulate per-request state on the struct; if you need scratch space, allocate it in `Render` or
  take it from a `sync.Pool`.
- **Renderers do not extract metadata.** Frontmatter, TOC, navigation, related docs and prev/next
  are produced earlier by the *enricher* and handed to you in `enrichment`. Use what you need and
  ignore the rest — a renderer that re-parses frontmatter is doing work the pipeline already did.

`enrichment` may be nil for content types no enricher handles. Guard before dereferencing.

## A worked example: CSV to an HTML table

```go
package renderer

import (
    "bytes"
    "context"
    "encoding/csv"
    "errors"
    "fmt"
    "html"
    "io"

    "github.com/monolithiclab/gomddoc/internal/enricher"
)

// CSVRenderer renders comma-separated values as an HTML table.
type CSVRenderer struct{}

func NewCSVRenderer() *CSVRenderer { return &CSVRenderer{} }

func (c *CSVRenderer) InputMimeTypes() []string  { return []string{"text/csv"} }
func (c *CSVRenderer) OutputMimeTypes() []string { return []string{"text/html"} }

func (c *CSVRenderer) Render(ctx context.Context, content []byte, _ *enricher.EnrichmentData) (*RenderResult, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }

    r := csv.NewReader(bytes.NewReader(content))
    r.FieldsPerRecord = -1 // ragged rows are a data problem, not a 500

    var buf bytes.Buffer
    buf.WriteString("<table>")
    for row := 0; ; row++ {
        rec, err := r.Read()
        if errors.Is(err, io.EOF) {
            break
        }
        if err != nil {
            return nil, fmt.Errorf("parse csv at row %d: %w", row+1, err)
        }
        cell := "td"
        if row == 0 {
            cell = "th"
        }
        buf.WriteString("<tr>")
        for _, field := range rec {
            fmt.Fprintf(&buf, "<%s>%s</%s>", cell, html.EscapeString(field), cell)
        }
        buf.WriteString("</tr>")
    }
    buf.WriteString("</table>")

    return &RenderResult{
        Content:  buf.Bytes(),
        MimeType: "text/html; charset=utf-8",
    }, nil
}
```

Register it in `setupPipeline` (`cmd/gomddoc/pipeline.go`):

```go
registry.Register(renderer.NewCSVRenderer())
```

`text/csv` already maps from `.csv` via the standard library's MIME table. For an extension Go does
not know, add it to the `init()` in `internal/negotiate/mime.go`, which is where `.md` and
`.markdown` are registered:

```go
_ = mime.AddExtensionType(".adoc", "text/asciidoc; charset=utf-8")
```

Without that, the provider reports `application/octet-stream` and your renderer is never consulted.

Put it there rather than beside your renderer. An `init()` only runs if its package is linked, and
the packages that call `negotiate.DetectMIME` — `internal/resolve` among them — do not import
`internal/renderer`. `.md` used to be registered in `markdown.go`, so clean-URL resolution worked
only because something else pulled the renderer package into the binary.

## How the registry picks a renderer

`DefaultRegistry.Get(inputMimeType, accepted)` matches on two axes:

1. **Input.** Score each registered renderer's `InputMimeTypes()` against the detected type — exact
   match `3`, `type/*` `2`, `*/*` `1`. Renderers scoring zero are dropped.
2. **Output.** Walk the `Accept` header's media types in q-value order and score each surviving
   candidate's `OutputMimeTypes()` by the same rule. The first accepted type that matches anything
   wins; lower-q types are never consulted.

The combined score is `inputScore*10 + outputScore`, so **input specificity always dominates**: a
renderer declaring `text/csv` beats the catch-all `PassthroughRenderer` (`*/*` → `*/*`) no matter
what the `Accept` header says. That is why the passthrough renderer never needs defending against —
it only wins when nothing more specific matched.

**Ties break toward the most recently registered renderer**, and that does matter between two
renderers with the same input type. `setupPipeline` registers `MarkdownPassthroughRenderer`
(`text/markdown` → `text/markdown`) *before* `MarkdownRenderer` (`text/markdown` → `text/html`), so
under `Accept: */*` — a bare `curl`, or a browser preflight — both score `31` and the later
registration wins, serving HTML. Swap those two lines and every unnegotiated request starts
returning raw markdown. If you add a second renderer for a type something already handles, its
position in `setupPipeline` is the whole tie-break.

If a renderer matches the input but none can produce an acceptable output type, `Get` returns
`ErrNoMatchingRenderer` and the handler responds **406 Not Acceptable**, listing
`AvailableOutputTypes(inputMimeType)`. If nothing matches the input at all, it returns `ErrNoRenderer`.

An output type of `*/*` resolves to the *input* type before matching, which is how the passthrough
renderer advertises "whatever came in, unchanged" without enumerating every MIME type.

## Conventions

- **Check `ctx.Err()` before doing work.** Every shipped renderer opens with it. Long renders should
  check again between expensive stages.
- **Return a full MIME type including charset** — `text/html; charset=utf-8`, not `text/html`. The
  registry normalizes for matching, but the string you return goes onto the wire as `Content-Type`.
- **Return an empty `MimeType` only for genuine passthrough**, where the provider's detected type is
  the right answer (binaries, images).
- **Wrap errors with context**: `fmt.Errorf("parse csv at row %d: %w", row, err)`. A bare `err` from
  a renderer reaches the log with no indication of which file failed.
- **Escape anything that ends up in HTML.** This is the one place the project's trusted-content model
  does not carry: markdown and theme templates are author-controlled, but a renderer that builds
  markup by concatenation will emit whatever byte sequence its input contained.

## Testing

Renderers are pure functions of `(content, enrichment)`, so table-driven tests need no server:

```go
func TestCSVRenderer(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name, input, want string
        wantErr           bool
    }{
        {name: "header and row", input: "a,b\n1,2\n", want: "<th>a</th><th>b</th>"},
        {name: "escapes markup", input: "x\n<script>\n", want: "&lt;script&gt;"},
        {name: "unterminated quote", input: "a,\"b\n", wantErr: true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            got, err := NewCSVRenderer().Render(t.Context(), []byte(tt.input), nil)
            if (err != nil) != tt.wantErr {
                t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
            }
            if err == nil && !strings.Contains(string(got.Content), tt.want) {
                t.Errorf("Content = %q, want it to contain %q", got.Content, tt.want)
            }
        })
    }
}
```

Two cases worth adding beyond the happy path:

- **Cancellation** — pass an already-cancelled `context.WithCancel` and assert the error. Do not use
  a nanosecond timeout plus `time.Sleep`; it is slower and flaky.
- **Negotiation** — register your renderer in a `DefaultRegistry` and assert `Get` returns it for
  your input type, and returns `ErrNoMatchingRenderer` for an `Accept` your renderer cannot satisfy.
  This is the half that unit-testing `Render` alone will not catch.

## Troubleshooting

| Symptom | Cause |
| --- | --- |
| Renderer never called | The provider detected a different input type. Check `mime.TypeByExtension` for your extension and register it in `init()` if the standard library does not know it. |
| 406 Not Acceptable | The request's `Accept` header does not intersect your `OutputMimeTypes()`. The response body lists what *is* available for that input type. |
| Content served raw | `PassthroughRenderer` won, which only happens when your `InputMimeTypes()` scored zero — see the row above. |
| Another renderer wins for the same input type | It was registered later. Move your `Register` call below it in `setupPipeline`. |
| `context deadline exceeded` | The render exceeded the request timeout. Renderers shelling out to an external tool need their own bounded `exec.CommandContext` — and note gomddoc otherwise ships zero `exec.Command` calls, so this is a meaningful change to the security posture. |
