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
    - `.Site.ColorChips`: Whether color chips are enabled.
    - `.Site.Highlighting.Theme`: Chroma syntax highlighting theme name.

- **`.Page`**: Current page data (`PageContext`).
    - `.Page.Content`: The rendered HTML content (type `template.HTML`, safe for embedding).
    - `.Page.Path`: Current request URL path.
    - `.Page.Meta`: Map of YAML Front Matter metadata (e.g., `{{ index .Page.Meta "title" }}`).
    - `.Page.TOC`: Table of Contents tree (use the `toc` template function to render).

### Template Functions

Seven custom functions are available in templates:

- **`breadcrumbs`**: Generates breadcrumb navigation from a path.
  ```html
  {{ range breadcrumbs .Page.Path }}
      <a href="{{ .Path }}">{{ .Label }}</a> /
  {{ end }}
  ```

- **`toc`**: Renders a Table of Contents as nested `<ul>` HTML. Accepts optional min/max heading levels.
  ```html
  {{ toc .Page.TOC }}           {{/* Default: h1-h2 */}}
  {{ toc .Page.TOC 2 3 }}       {{/* Only h2-h3 */}}
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

- **`inlineJSAsset`**: Loads an asset from the theme directory (falling back to shared assets) and returns it as `template.JS` for safe embedding inside `<script>` tags.
  ```html
  <script type="module">{{ inlineJSAsset "color-chip.mjs" }}</script>
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
        <button id="theme-toggle">Toggle Theme</button>
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

    <aside id="toc-sidebar">
        {{ toc .Page.TOC 2 3 }}
    </aside>

    <!-- Color Chip Web Component -->
    <script type="module">{{ inlineJSAsset "color-chip.mjs" }}</script>
</body>
</html>
```

## Theme Features Checklist

When creating a custom theme, ensure it supports these features for parity with built-in themes:

- **Light/dark mode toggle** with `data-theme` attribute and `prefers-color-scheme` CSS fallback
- **Navigation sidebar** via `{{ navigation .Page.Path }}`
- **Table of contents** via `{{ toc .Page.TOC }}` with scroll highlighting
- **Breadcrumbs** via `{{ breadcrumbs .Page.Path }}`
- **Search button** with `id="search-toggle"` in the header
- **Search modal** via `{{ inlineJSAsset "search.mjs" }}` — uses CSS custom properties for styling
- **Admonition styling** for `.admonition-note`, `.admonition-tip`, `.admonition-important`, `.admonition-warning`, `.admonition-caution`
- **Color chip web component** via `{{ inlineJSAsset "color-chip.mjs" }}`
- **Copy-to-clipboard** on code blocks
- **Heading anchors** (`.heading-anchor` class, revealed on hover)
- **Touch accessibility** with `@media (hover: none)` for copy buttons and heading anchors
- **KaTeX** CSS and auto-render scripts for math rendering
- **Mermaid** script for diagram rendering (theme-aware: dark/light)
- **Canonical URLs and Open Graph tags** via `{{ canonicalURL .Page.Path }}` when domain is configured
- **Responsive design** with mobile breakpoints
