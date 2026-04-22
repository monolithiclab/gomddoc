---
title: "Theming & Assets Guide"
description: "Guide on how to customize themes and manage assets in gomddoc."
author: "nicolasm"
---

# Theming & Assets

gomddoc uses an **Overlay Filesystem** to handle assets. This means you can "overlay" your own custom files on top of the built-in defaults without replacing everything.

## Built-in Themes

gomddoc ships with 8 themes. Set the theme in `.gomddoc/config.yml` or via `GOMDDOC_SITE_THEME_NAME`:

| Theme | Style | Description |
|-------|-------|-------------|
| `default` | General purpose | Three-column layout (nav + content + TOC), Inter font |
| `academic` | Scholarly | Serif typography (Merriweather), justified text, warm parchment palette |
| `gitbook` | Documentation | Book-style reading (1.8 line height), tinted nav sidebar |
| `material` | Design system | Material Design 3, rounded corners, tonal elevation |
| `midnight` | Dark-first | Neon purple/cyan gradients, glowing code blocks |
| `minimal` | Brutalist | System fonts only, zero border-radius, heavy typographic hierarchy |
| `nord` | Color palette | Nord 16-color palette, frosted glass aesthetic |
| `ocean` | Colorful | Teal/navy gradients, sine-wave header clip-path |

All themes include: light/dark mode, TOC sidebar, navigation sidebar, breadcrumbs, admonitions, color chips, copy-to-clipboard code blocks, heading anchors, KaTeX math, Mermaid diagrams, and touch device accessibility.

### Switching Themes

```yaml
# .gomddoc/config.yml
theme:
  name: "nord"
```

Or via environment variable:

```bash
GOMDDOC_SITE_THEME_NAME=midnight gomddoc serve
```

## Directory Structure

To customize your site, create a `.gomddoc` folder in your content root:

```text
/my-docs
├── .gomddoc/
│   ├── config.yml
│   └── assets/
│       └── themes/
│           └── default/
│               └── default.html.tmpl  <-- Overrides built-in layout
└── README.md
```

The overlay filesystem checks your `.gomddoc/assets/` first, then falls back to the embedded defaults.

## Creating a Custom Theme

1.  **Define Theme:** In `.gomddoc/config.yml`:
    ```yaml
    theme:
      name: "custom"
    ```
2.  **Create Template:** Create `.gomddoc/assets/themes/custom/layouts/default.html.tmpl` with a
    skeleton that calls partials. Add partials in `.gomddoc/assets/themes/custom/partials/`
    (e.g., `head.html.tmpl`, `header.html.tmpl`, `nav.html.tmpl`, `toc.html.tmpl`, `scripts.html.tmpl`).

### Template Variables

The template engine (Go `html/template`) receives a `TemplateContext` with:

- **`.Site`**: Global site configuration (`*config.SiteConfig`).
    - `.Site.Meta.Title`: Site title.
    - `.Site.Meta.Description`: Site description.
    - `.Site.Meta.Domain`: Site domain.
    - `.Site.Theme.Name`: Current theme name.
    - `.Site.DefaultIndex`: Default index file name.
    - `.Site.DirIndex`: Whether directory listing is enabled.
    - `.Site.EditURL`: Base URL for "Edit this page" links.
    - `.Site.Features`: Feature toggle map (`map[string]bool`).
    - `.Site.Highlighting.Theme`: Chroma syntax highlighting theme name.

- **`.Page`**: Current page data (`PageContext`).
    - `.Page.Content`: The rendered HTML content (type `template.HTML`, safe for embedding).
    - `.Page.Path`: Current request URL path.
    - `.Page.Meta`: Map of YAML Front Matter metadata (e.g., `{{ index .Page.Meta "title" }}`).
    - `.Page.Features`: Pre-merged feature toggles (site defaults + page overrides).
    - `.Page.TOC`: Table of Contents tree (use the `toc` template function to render).

### Template Functions

Custom functions available in templates:

- **`breadcrumbs`**: Generates breadcrumb navigation from a path.
  ```html
  {{ range breadcrumbs .Page.Path }}
      <a href="{{ .Path }}">{{ .Label }}</a> /
  {{ end }}
  ```

- **`toc`**: Returns a filtered list of `TOCNode` items for template rendering. Accepts optional min/max heading levels. Used with the recursive `toc-item` partial defined in `toc.html.tmpl`.
  ```html
  {{ $items := toc .Page.TOC }}           {{/* Default: h1-h2 */}}
  {{ $items := toc .Page.TOC 2 3 }}       {{/* Only h2-h3 */}}
  {{ range $items }}{{ template "toc-item" . }}{{ end }}
  ```

- **`navigation`**: Generates the sidebar navigation tree with active state highlighting.
  ```html
  <nav>{{ navigation .Page.Path }}</nav>
  ```

- **`editURL`**: Combines the configured `edit_url` base with the current page path. Returns empty string if `edit_url` is not configured.
  ```html
  {{ $editLink := editURL .Page.Path }}
  {{ if $editLink }}
      <a href="{{ $editLink }}">Edit this page</a>
  {{ end }}
  ```

- **`canonicalURL`**: Returns the full canonical URL for a page (requires `meta.domain`). Returns empty string if domain is not configured.
  ```html
  {{ $canonical := canonicalURL .Page.Path }}
  {{ if $canonical }}
      <link rel="canonical" href="{{ $canonical }}">
  {{ end }}
  ```

- **`.Feature`**: Method on `TemplateContext` that checks whether a named feature is enabled. Returns `true` by default for unknown features. Uses the pre-merged `Page.Features` map (site defaults + page frontmatter overrides).
  ```html
  {{- if .Feature "dark_mode" }}
  <button id="theme-toggle">Toggle Theme</button>
  {{- end }}
  {{- if .Feature "color_chips" }}
  <script type="module">{{ inlineJSAsset "gmd-color-chip.mjs" }}</script>
  {{- end }}
  ```

- **`contentURL`**: Returns the absolute URL path (without scheme or domain) for a content file. When extension stripping is active, returns the clean extensionless path. Default index files (e.g., `README.md`) are resolved to their directory path.
  ```html
  <a href="{{ contentURL "docs/guide.md" }}">Guide</a>
  {{/* Output: /docs/guide (with extension stripping) */}}
  {{/* Output: /docs/guide.md (without extension stripping) */}}
  ```

- **`inlineJSAsset`**: Loads an asset from the theme directory (falling back to shared assets) and returns it as `template.JS` for safe embedding inside `<script>` tags.
  ```html
  <script type="module">{{ inlineJSAsset "gmd-color-chip.mjs" }}</script>
  <script type="module">{{ inlineJSAsset "search.mjs" }}</script>
  ```

- **`inlineCSSAsset`**: Loads an asset and returns it as `template.CSS` for safe embedding inside `<style>` tags.
  ```html
  <style>{{ inlineCSSAsset "custom.css" }}</style>
  ```

- **`inlineHTMLAsset`**: Loads an asset and returns it as `template.HTML` for safe embedding in HTML context (e.g. inline SVGs).
  ```html
  {{ inlineHTMLAsset "logo.svg" }}
  ```

### Example Layout

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <title>{{ if .Page.Meta.title }}{{ .Page.Meta.title }} | {{ end }}{{ .Site.Meta.Title }}</title>
    <meta name="description" content="{{ .Site.Meta.Description }}">
</head>
<body>
    <header>
        <a href="/">{{ .Site.Meta.Title }}</a>
        {{- if .Feature "dark_mode" }}
        <button id="theme-toggle">Toggle Theme</button>
        {{- end }}
    </header>

    <nav id="nav-sidebar">
        {{ navigation .Page.Path }}
    </nav>

    <nav aria-label="Breadcrumb">
        {{ range breadcrumbs .Page.Path }}
            <a href="{{ .Path }}">{{ .Label }}</a> /
        {{ end }}
    </nav>

    <article>
        {{ .Page.Content }}

        {{ $editLink := editURL .Page.Path }}
        {{ if $editLink }}
        <footer><a href="{{ $editLink }}">Edit this page</a></footer>
        {{ end }}
    </article>

    {{ template "toc" . }}

    {{- if .Feature "color_chips" }}
    <!-- Color Chip Web Component -->
    <script type="module">{{ inlineJSAsset "gmd-color-chip.mjs" }}</script>
    {{- end }}
</body>
</html>
```

## Theme Features Checklist

When creating a custom theme, ensure it supports these features for parity with built-in themes. All optional features must be wrapped in `{{ .Feature "name" }}` guards so they can be toggled per-site and per-page:

- **Light/dark mode toggle** — guarded by `{{ .Feature "dark_mode" }}`. Uses `{{ inlineJSAsset "theme-toggle.mjs" }}` to manage `data-theme` attribute, localStorage, and `prefers-color-scheme` fallback. Customize button content with `data-light`/`data-dark` text attributes or `[data-show-theme]` children for SVG icons.
- **Navigation sidebar** via `{{ navigation .Page.Path }}`
- **Table of contents** — guarded by `{{ .Feature "toc" }}`. Uses `{{ template "toc" . }}` partial with `toc-item` recursive template.
- **TOC scroll highlighting** — guarded by `{{ .Feature "toc" }}`. Uses `{{ inlineJSAsset "toc-highlight.mjs" }}` to track scroll position and set `.active` class on matching TOC link.
- **Breadcrumbs** via `{{ breadcrumbs .Page.Path }}`
- **Search button** — guarded by `{{ .Feature "search" }}`. Button with `id="search-toggle"` in the header.
- **Search modal** — guarded by `{{ .Feature "search" }}`. Uses `{{ inlineJSAsset "search.mjs" }}` with CSS custom properties for styling.
- **Admonition styling** for `.admonition-note`, `.admonition-tip`, `.admonition-important`, `.admonition-warning`, `.admonition-caution`
- **Color chip web component** — guarded by `{{ .Feature "color_chips" }}`. Uses `{{ inlineJSAsset "gmd-color-chip.mjs" }}`.
- **Copy-to-clipboard** — guarded by `{{ .Feature "code_copy" }}`. Uses `{{ inlineJSAsset "code-copy.mjs" }}`, creates `.copy-btn` on code blocks. Customize text with `data-copy-label`/`data-copied-label` on `<html>`.
- **Heading anchors** (`.heading-anchor` class, revealed on hover)
- **Touch accessibility** with `@media (hover: none)` for copy buttons and heading anchors
- **KaTeX** — guarded by `{{ .Feature "katex" }}`. CSS link in `<head>` and auto-render scripts.
- **Mermaid** — guarded by `{{ .Feature "mermaid" }}`. Script for diagram rendering (theme-aware: dark/light).
- **Canonical URLs and Open Graph tags** via `{{ canonicalURL .Page.Path }}` when domain is configured
- **Responsive design** with mobile breakpoints
