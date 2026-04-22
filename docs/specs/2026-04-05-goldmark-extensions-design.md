# Post-Processors to Goldmark Extensions

**Date:** 2026-04-05
**Status:** Approved

## Summary

Transform the three regex-based HTML post-processors (heading anchors, admonitions, color chips)
into proper goldmark AST extensions. Each becomes a `goldmark.Extender` with AST transformers
and custom node renderers, operating on structured AST nodes instead of regex-replacing rendered
HTML bytes.

## Motivation

The current post-processors operate on rendered HTML via regex patterns. This works but is
architecturally fragile — regex on HTML is inherently brittle, the transformations can't compose
with other goldmark extensions, and they're not testable at the AST level. Moving to goldmark
extensions integrates these features into the standard goldmark pipeline.

## Design

### Extension Structure

Each feature becomes a standalone goldmark extension implementing `goldmark.Extender`:

| Extension | File | Custom AST Node | Mechanism |
|-----------|------|-----------------|-----------|
| Heading Anchors | `ext_anchors.go` | None | Custom `NodeRenderer` for `ast.KindHeading` |
| Admonitions | `ext_admonition.go` | `AdmonitionNode` (block) | AST transformer + node renderer |
| Color Chips | `ext_colorchip.go` | `ColorChipNode` (inline) | AST transformer + node renderer |

All three extensions are always registered on the goldmark instance. Feature toggling happens
inside the extensions themselves.

### Feature Flag Integration

Feature flags cannot gate extension registration because goldmark extensions are registered once
at construction, but features can be toggled per-page via frontmatter.

**Mechanism:** `MarkdownRenderer.Render()` computes merged features (site-level + page-level)
and stores them on the goldmark parser context via a shared context key:

```go
// ext_features.go
var featuresContextKey = parser.NewContextKey()
```

```go
// In MarkdownRenderer.Render():
merged := config.MergeFeatures(m.features, pageFeatures)
pCtx.Set(featuresContextKey, merged)
```

The AST transformers read this key. If a feature is disabled, the transformer skips
transformation entirely — original AST nodes remain and render with goldmark's default
renderers. This avoids needing fallback rendering logic in custom node renderers.

### Extension Implementations

#### Heading Anchors (`ext_anchors.go`)

No custom AST node. Registers a custom `NodeRenderer` for `ast.KindHeading`:

1. Checks feature flag from parser context; if disabled, delegates to default heading rendering
2. Renders the opening `<hN id="...">` tag
3. Renders child nodes (heading text content)
4. Appends `<a href="#id" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>`
5. Renders closing `</hN>` tag
6. Headings without an ID attribute render normally (no anchor appended)

Note: This one uses a renderer override rather than an AST transformer because there's no
structural change to the AST — we're only adding decoration to the HTML output of an existing
node type.

#### Admonitions (`ext_admonition.go`)

Custom `AdmonitionNode` block node with fields:
- `AdmonitionType string` — lowercase type name (note, tip, important, warning, caution)
- `Title string` — display title (Note, Tip, Important, Warning, Caution)

**AST Transformer:**
1. Walks the document for `ast.KindBlockquote` nodes
2. Checks if first child paragraph's text starts with `[!TYPE]`
3. If feature disabled (via parser context), skips
4. Replaces the blockquote with an `AdmonitionNode`, stripping the `[!TYPE]` marker
5. Moves remaining blockquote children into the admonition node

**Node Renderer:**
Emits:
```html
<div class="admonition admonition-{type}">
<p class="admonition-title">{Title}</p>
{children}
</div>
```

Supported types: NOTE, TIP, IMPORTANT, WARNING, CAUTION (case-insensitive matching on input).

#### Color Chips (`ext_colorchip.go`)

Custom `ColorChipNode` inline node with field:
- `HexColor string` — the hex color value including `#` (e.g., `#FF5733`)

**AST Transformer:**
1. Walks the document for `ast.KindCodeSpan` nodes
2. Reads the code span's text content from source bytes
3. If content matches `#` + exactly 3 or 6 hex digits, and feature is enabled, replaces with `ColorChipNode`
4. Code spans inside fenced code blocks (`<pre><code>`) are not `ast.CodeSpan` nodes, so they're naturally excluded

**Node Renderer:**
Emits:
```html
<color-chip>#HEX</color-chip>
```

### File Changes

**New files:**
- `internal/renderer/ext_features.go` — shared `featuresContextKey`
- `internal/renderer/ext_anchors.go` — heading anchor extension
- `internal/renderer/ext_admonition.go` — admonition extension + `AdmonitionNode`
- `internal/renderer/ext_colorchip.go` — color chip extension + `ColorChipNode`

**Modified files:**
- `internal/renderer/markdown.go` — register extensions in `goldmark.New()`, remove post-processing block, set features on parser context

**Deleted files:**
- `internal/renderer/anchors.go`
- `internal/renderer/admonition.go`
- `internal/renderer/colorchip.go`

**Test files (rewritten):**
- `internal/renderer/anchors_test.go` — AST-level unit tests + same integration cases
- `internal/renderer/admonition_test.go` — same
- `internal/renderer/colorchip_test.go` — same
- `internal/renderer/markdown_test.go` — unchanged (validates end-to-end)

**No changes to:**
- `internal/config/features.go`
- `internal/enricher/`
- `cmd/gomddoc/pipeline.go`
- Theme CSS

### Constraints

- **HTML output must be byte-identical** to current regex-based post-processors. No behavioral change.
- All existing tests must pass with equivalent assertions (test structure may change from testing raw HTML functions to testing via the renderer).
- Feature flag behavior (site-level defaults, per-page overrides, default-to-enabled) must be preserved exactly.
