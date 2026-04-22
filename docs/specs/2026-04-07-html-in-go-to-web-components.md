# Move HTML-in-Go to Web Components and Templates

**Date:** 2026-04-07
**Status:** Approved
**Addresses:** REVIEW.md HIGH "Hardcoded HTML in Go (Theme Inflexibility)"

## Summary

Replace hardcoded HTML in goldmark renderers with web component custom elements, move the
JSON-LD `<script>` wrapper into its template partial, and rename `<color-chip>` to
`<gmd-color-chip>` for namespace consistency.

## Motivation

Four goldmark renderers and one template function hardcode HTML in Go, making it impossible for
themes to customize their rendering. By emitting semantic custom elements, goldmark stays
concerned with structure while themes control presentation via overridable web component JS files.

## Changes

### 1. Admonitions: `<gmd-admonition>` (light DOM)

**Goldmark output** changes from:

```html
<div class="admonition admonition-note"><p class="admonition-title">Note</p>
<p>Content</p></div>
```

To:

```html
<gmd-admonition type="note" title="Note">
<p>Content</p>
</gmd-admonition>
```

**Web component** (`shared/gmd-admonition.mjs`):

- Light DOM so theme CSS applies to inner content naturally
- On `connectedCallback`: reads `type` and `title` attributes, prepends a title element
  (`<p class="admonition-title">`) and adds `class="admonition admonition-{type}"` to the host
- Themes override by providing their own `gmd-admonition.mjs` in their assets

**File changes:**

- `internal/renderer/ext_admonition.go` — change `renderAdmonition` to emit
  `<gmd-admonition type="..." title="...">` / `</gmd-admonition>`
- `cmd/gomddoc/assets/shared/gmd-admonition.mjs` — new web component
- `cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl` — add conditional
  `inlineJSAsset` for `gmd-admonition.mjs` gated on `admonitions` feature

**Loading:** conditional on `admonitions` feature flag in `head-shared.html.tmpl`, matching
the existing pattern for `color-chip.mjs`.

### 2. Heading Anchors: `<gmd-heading-anchor>` (Shadow DOM)

**Goldmark output** changes from:

```html
<h2 id="foo">Text <a href="#foo" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a></h2>
```

To:

```html
<h2 id="foo">Text <gmd-heading-anchor href="#foo"></gmd-heading-anchor></h2>
```

**Web component** (`shared/gmd-heading-anchor.mjs`):

- Shadow DOM (self-contained, like `color-chip`)
- Renders an `<a>` with `#` text, `aria-hidden="true"`, `tabindex="-1"`
- Exposes `::part(link)` for theme styling
- CSS custom properties for color/opacity inherited from document

**File changes:**

- `internal/renderer/ext_anchors.go` — change `renderHeading` to emit
  `<gmd-heading-anchor href="#id">` instead of the inline `<a>` tag
- `cmd/gomddoc/assets/shared/gmd-heading-anchor.mjs` — new web component
- `cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl` — add conditional
  `inlineJSAsset` for `gmd-heading-anchor.mjs` gated on `heading_anchors` feature

**Loading:** conditional on `heading_anchors` feature flag.

### 3. JSON-LD: Move `<script>` Wrapper into Partial

**Current state:** `generateJSONLD` in `internal/template/renderer.go:352` wraps the JSON-LD
payload in `<script type="application/ld+json">...</script>` and returns `template.HTML`.

**Change:** `generateJSONLD` returns just the raw JSON string. The existing
`jsonld.html.tmpl` partial wraps it:

```html
{{ define "jsonld" }}{{ $json := jsonLD .Page }}{{ if $json }}
<script type="application/ld+json">{{ $json }}</script>{{ end }}{{ end }}
```

**File changes:**

- `internal/template/renderer.go` — `generateJSONLD` returns `template.HTML` containing only
  the JSON payload (no `<script>` wrapper). Empty string if no JSON-LD data.
- `cmd/gomddoc/assets/themes/default/partials/jsonld.html.tmpl` — add `<script>` wrapper
  and empty-check

### 4. Rename `<color-chip>` to `<gmd-color-chip>`

Align existing web component with the `gmd-` prefix convention.

**File changes:**

- `cmd/gomddoc/assets/shared/color-chip.mjs` — rename to `gmd-color-chip.mjs`, update
  `customElements.define("gmd-color-chip", ...)` and class name
- `internal/renderer/ext_colorchip.go` — emit `<gmd-color-chip>` instead of `<color-chip>`
- `cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl` — update
  `inlineJSAsset "color-chip.mjs"` to `"gmd-color-chip.mjs"`

## Out of Scope

- **Search `<mark>` tags** (`internal/search/snippet.go`) — postponed. The search package
  operates outside the template system and returns HTML via API; different solution needed.

## Testing

- Existing goldmark extension tests updated to assert new custom element output
- Existing renderer integration tests updated for new HTML structure
- Theme CSS tests in `testsite/` verified manually via Chrome DevTools MCP
- JSON-LD template tests updated for the new partial-wrapped output

## Theme Impact

- Default theme CSS for admonitions (`.admonition`, `.admonition-title`) continues to work
  because the light DOM web component applies the same classes
- Heading anchor CSS (`.heading-anchor`) moves into the web component's Shadow DOM;
  existing theme CSS targeting `.heading-anchor` should be removed from theme stylesheets
  since the component owns its own styles
- Third-party themes in `gomddoc-themes` will need updated CSS and the new shared JS assets
