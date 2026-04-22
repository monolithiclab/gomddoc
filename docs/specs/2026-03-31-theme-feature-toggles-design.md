# Design: Generic Theme Feature Toggles

**Date:** 2026-03-31
**Status:** Approved

## Problem

Adding a new feature toggle requires a new `SiteConfig` field, env var, YAML key, validation code,
and template wiring. Features like KaTeX, Mermaid, and search UI are always loaded with no way to
disable them. Only `ColorChips` has a toggle, implemented as a one-off `SiteConfig` field with
per-page frontmatter override. This doesn't scale.

## Design

### Config Surface

Feature toggles live under `features` at the site config root in `.gomddoc/config.yml`:

```yaml
theme:
  name: material
  vars:
    primary: "#2563eb"

features:
  katex: false
  mermaid: false
  color_chips: true
  dark_mode: true
  admonitions: true
  code_copy: true
  toc: true
  heading_anchors: true
```

**Defaults:** All features default to `true`. Users only configure what they want to disable. This preserves backward compatibility — existing sites that don't set `theme.features` get identical behavior to today.

**Environment override:** `GOMDDOC_SITE_FEATURES_KATEX=false` (follows the existing `GOMDDOC_` prefix + struct path convention: `Site.Features.Katex`).

**Per-page override:** Any feature can be toggled per page via frontmatter, nested under a `features` key:

```yaml
---
title: Simple Page
features:
  katex: false
  mermaid: false
---
```

### Known Feature Keys

The following keys have defined semantics. Themes may support additional keys.

| Key               | Controls                                  | Default |
| ----------------- | ----------------------------------------- | ------- |
| `katex`           | KaTeX math rendering (CDN scripts)        | `true`  |
| `mermaid`         | Mermaid diagram rendering (CDN scripts)   | `true`  |
| `color_chips`     | Inline color chip web component           | `true`  |
| `dark_mode`       | Theme toggle button and dark mode support | `true`  |
| `admonitions`     | Admonition block styling                  | `true`  |
| `code_copy`       | Copy button on code blocks                | `true`  |
| `toc`             | Table of contents sidebar                 | `true`  |
| `heading_anchors` | Anchor links on headings                  | `true`  |

`search` has no special handling. If disabled in the site config, the search index is not built and the search UI is not loaded. If a user disables `search` site-wide but enables it per page via frontmatter, the search UI will load but won't work (no index) — consistency is the user's responsibility.

### Config Struct Changes

```go
type SiteConfig struct {
    // ... existing fields ...
    Features map[string]bool `yaml:"features"`
}
```

`SiteConfig.ColorChips` is removed. Its value migrates to `SiteConfig.Features["color_chips"]`.

`SiteConfig.HasSearch` is removed. Search availability is set directly in `SiteConfig.Features["search"]` by the search indexer at runtime.

`ThemeConfig` is unchanged — it keeps `Name` and `Vars` only.

### Template Access

Templates access features via the `feature` template function — a closure that accepts only the feature key name:

```
{{ if feature "katex" }}
    <script defer src="https://cdn.jsdelivr.net/npm/katex@0.16.21/dist/katex.min.js"></script>
    ...
{{ end }}
```

The closure captures a **merged features map** built per page by the pipeline, resolving in this order (first match wins):

1. **Page frontmatter** — `Page.Meta["features"]["katex"]` (per-page override)
2. **Site config** — `SiteConfig.Features["katex"]` (site default)
3. **Built-in default** — `true` (if not configured anywhere)

See [Template Function](#template-function) for implementation details.

### Go Implementation

A `featureEnabled` function resolves feature state:

```go
func featureEnabled(name string, features map[string]bool) bool {
    if v, ok := features[name]; ok {
        return v
    }
    // Default: enabled
    return true
}
```

This function is used in two places:

- **Renderer:** called with the merged features map to gate post-processors
- **Templates:** via the `feature` closure (see [Template Function](#template-function))

### Renderer Changes

`MarkdownRenderer` currently takes `ColorChips bool` in `MarkdownOptions` and calls `colorChipsEnabled()` for per-page override logic. This changes to:

- `MarkdownOptions` receives `Features map[string]bool` (from `SiteConfig.Features`)
- `colorChipsEnabled()` is replaced by `featureEnabled()`
- The renderer checks `featureEnabled("color_chips", opts.Features)` before calling `transformColorChips()`
- The renderer checks `featureEnabled("heading_anchors", opts.Features)` before calling `addHeadingAnchors()`
- The renderer checks `featureEnabled("admonitions", opts.Features)` before calling `transformAdmonitions()`

### Theme Changes

All 8 themes' `scripts.html.tmpl` wrap conditional features:

```html
{{ define "scripts" }} {{ if feature "dark_mode" }}
<script>
  {{ inlineJSAsset "theme-toggle.mjs" }}
</script>
{{ end }} {{ if feature "code_copy" }}
<script>
  {{ inlineJSAsset "code-copy.mjs" }}
</script>
{{ end }} {{ if feature "toc" }}
<script>
  {{ inlineJSAsset "toc-highlight.mjs" }}
</script>
{{ end }} {{ if feature "katex" }}
<link
  rel="stylesheet"
  href="https://cdn.jsdelivr.net/npm/katex@0.16.21/dist/katex.min.css"
/>
<script
  defer
  src="https://cdn.jsdelivr.net/npm/katex@0.16.21/dist/katex.min.js"
></script>
<script
  defer
  src="https://cdn.jsdelivr.net/npm/katex@0.16.21/dist/contrib/auto-render.min.js"
  onload="renderMathInElement(document.body, ...);"
></script>
{{ end }} {{ if feature "mermaid" }}
<script type="module">
  import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs';
  mermaid.initialize({...});
</script>
{{ end }} {{ if feature "color_chips" }}
<script type="module">
  {{ inlineJSAsset "color-chip.mjs" }}
</script>
{{ end }} {{ if feature "search" }}
<script type="module">
  {{ inlineJSAsset "search.mjs" }}
</script>
{{ end }} {{ end }}
```

Themes also conditionally render structural elements:

- `toc.html.tmpl`: wrap TOC sidebar in `{{ if feature "toc" }}`
- `header.html.tmpl`: wrap theme toggle button in `{{ if feature "dark_mode" }}`
- `head.html.tmpl`: wrap KaTeX CSS `<link>` in `{{ if feature "katex" }}`

### Template Function

The pipeline builds a merged features map per page (page frontmatter overrides site defaults). A `feature` closure captures this merged map and takes only the key name:

```go
// In the pipeline, per page:
merged := mergeFeatures(h.siteConfig.Features, pageMeta)

// In funcMap (or per-render execution data):
"feature": func(name string) bool {
    return featureEnabled(name, merged)
},
```

```go
func mergeFeatures(site map[string]bool, pageMeta map[string]any) map[string]bool {
    merged := maps.Clone(site)
    if pageMeta == nil {
        return merged
    }
    features, ok := pageMeta["features"]
    if !ok {
        return merged
    }
    fm, ok := features.(map[string]any)
    if !ok {
        return merged
    }
    for k, v := range fm {
        if b, ok := v.(bool); ok {
            merged[k] = b
        }
    }
    return merged
}
```

Templates call it as:

```
{{ if feature "katex" }}
    <script defer src="https://cdn.jsdelivr.net/npm/katex@0.16.21/dist/katex.min.js"></script>
    ...
{{ end }}
```

The `feature` closure is rebuilt per page render with the merged map. Cached templates are unaffected — the closure is injected via the template execution data, not the template definition.

### Config Validation

- Feature keys are validated: `^[a-z][a-z0-9_]*$` (lowercase, underscores, starts with letter)
- Values must be boolean (YAML `true`/`false`)
- Unknown feature keys are allowed (themes may define custom features)

### Config Documentation

Update `docs/guide/05-theming-and-assets.md` and `material/public-website/docs/configuration.md` to document `features`.

### Migration Path

1. Remove `SiteConfig.ColorChips` field
2. Remove `SiteConfig.HasSearch` field — replace all usages with `featureEnabled("search", ...)`
3. `NewSiteConfig()` initializes `SiteConfig.Features` map (no defaults needed — `featureEnabled()` defaults to `true`)
4. Search indexer checks `featureEnabled("search", ...)` to decide whether to build the index
5. Remove `colorChipsEnabled()` from renderer
6. `MarkdownOptions.ColorChips bool` replaced by `MarkdownOptions.Features map[string]bool`
7. Update pipeline.go to pass features through
8. Update all tests referencing `ColorChips` or `HasSearch`

## Out of Scope

- Converting post-processors (admonitions, anchors) to goldmark AST extensions (separate issue #4 in REVIEW.md)
- Feature dependencies (e.g., "toc requires heading_anchors") — themes handle this implicitly
- Custom feature key registration — themes can use any key; unknown keys are passed through
- Chained map pattern (`ChainedMap[V]` with ordered `Get` across multiple maps) — considered for lazy cascading lookup instead of `mergeFeatures()`, but overkill for ~8 boolean keys. Revisit if cascading config expands to theme vars or per-directory overrides
