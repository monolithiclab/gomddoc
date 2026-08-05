---
title: "Theming & Assets Guide"
description: "Guide on how to customize themes and manage assets in gomddoc."
author: "nicolasm"
---

# Theming & Assets

gomddoc uses an **Overlay Filesystem** to handle assets. This means you can "overlay" your own custom files on top of the built-in defaults without replacing everything.

## Built-in Themes

gomddoc offers 8 themes. Set the theme in `.gomddoc/config.yml` or via `GOMDDOC_SITE_THEME_NAME`:

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

> **Theme bundling:** only the `default` theme is embedded in the gomddoc source tree (`cmd/gomddoc/assets/themes/`). The other 7 themes are maintained in the separate [`gomddoc-themes`](https://github.com/monolithiclab/gomddoc-themes) repository. To use one when building from source, copy its directory into your site's `.gomddoc/assets/themes/<name>/` overlay (see [Static Asset Overlay](#static-asset-overlay)), which takes precedence over the embedded assets.

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
│   ├── locales/                         ← Translation overrides (see Internationalization)
│   │   └── fr-FR.yml
│   └── assets/
│       └── themes/
│           └── default/
│               └── default.html.tmpl    ← Overrides built-in layout
└── README.md
```

The overlay filesystem checks your `.gomddoc/assets/` first, then falls back to the embedded defaults. Locale files
in `.gomddoc/locales/` override built-in and theme-provided translations.

### Static Asset Overlay

Static assets (CSS, JS, images, fonts) follow a three-layer priority system:

1. **Site-level** (`static/` in content root) — highest priority
2. **Theme-level** (`assets/themes/{name}/static/`) — theme-specific assets
3. **Shared** (`assets/shared/static/`) — common assets across themes

All static assets are served at `/_assets/` and cached with `Cache-Control: public, max-age=31536000, immutable`
(1-year, immutable).

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
    - `.Page.RelatedDocs`: Pages sharing frontmatter tags with this page, rendered by the `see-also` partial.
    - `.Page.PrevPage` / `.Page.NextPage`: Adjacent pages in navigation order (each has `.Path` and `.Title`).
    - `.Page.Breadcrumbs`: Trail from the site root to this page (each has `.Path` and `.Label`). Generated once per page, not on demand — there is no `breadcrumbs` template function.

### Template Functions

Custom functions available in templates:

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

- **`pageTags`**: Returns the current page's normalized frontmatter tags (lowercased, trimmed, deduplicated — the same
  normalization the metadata index applies). Used by the `tag-chips` partial.
  ```html
  {{ range pageTags .Page.Meta }}<a href="{{ tagURL $.Lang . }}">{{ . }}</a>{{ end }}
  ```

- **`tagURL`**: Returns the URL path for a tag's landing page, language-prefixed when the page is in a non-default
  language. Pass the current language (`$.Lang`) and the tag.
  ```html
  <a href="{{ tagURL $.Lang "go" }}">go</a>
  {{/* Output: /tags/go (default lang) or /fr-FR/tags/go (non-default) */}}
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

- **`themeVarsCSS`**: Returns CSS custom properties generated from the `theme.vars` config. Embed inside a `<style>`
  tag to make theme variables available to your CSS.
  ```html
  <style>{{ themeVarsCSS }}</style>
  {{/* Output: :root { --theme-primary-color: #2563eb; --theme-sidebar-width: 280px; } */}}
  ```

- **`jsonLD`**: Returns JSON-LD structured data (`<script type="application/ld+json">`) for the current page. Includes
  TechArticle, BreadcrumbList, and WebSite schemas. See [SEO — Structured Data](11-seo.md#structured-data-json-ld) for
  details.
  ```html
  {{ jsonLD . }}
  ```

- **`assetURL`**: Returns the URL path for a static asset, prefixed with `/_assets/`. Use this to reference theme
  assets in templates.
  ```html
  <link rel="icon" href="{{ assetURL "favicon.ico" }}">
  {{/* Output: /_assets/favicon.ico */}}
  ```

#### i18n Functions

These functions are available when multi-language support is active. See
[Internationalization](13-internationalization.md) for the full i18n guide.

- **`.T "key"`**: Returns the translated string for the current page's language. Falls back to the default language,
  then to the key itself.
  ```html
  <span>{{ .T "toc_title" }}</span>
  ```

- **`.Lang`**: Returns the BCP 47 language code for the current page. Respects per-page `lang` frontmatter overrides.
  ```html
  <html lang="{{ .Lang }}">
  ```

- **`.Languages`**: Returns all available languages as `LanguageInfo` objects (`.Code`, `.Name`, `.Active`,
  `.Default`). Used to build a language switcher.
  ```html
  {{- range .Languages }}
  <a href="/{{ .Code }}{{ $path }}">{{ .Name }}</a>
  {{- end }}
  ```

### Example Layout

```html
<!DOCTYPE html>
<html lang="{{ .Lang }}">
<head>
    <title>{{ if .Page.Meta.title }}{{ .Page.Meta.title }} | {{ end }}{{ .Site.Meta.Title }}</title>
    <meta name="description" content="{{ .Site.Meta.Description }}">
    <style>{{ themeVarsCSS }}</style>
    {{ template "hreflang" . }}
</head>
<body>
    <header>
        <a href="/">{{ .Site.Meta.Title }}</a>
        {{ template "lang-switcher" . }}
        {{- if .Feature "dark_mode" }}
        <button id="theme-toggle">{{ .T "aria_toggle_theme" }}</button>
        {{- end }}
    </header>

    <nav id="nav-sidebar" aria-label="{{ .T "aria_site_nav" }}">
        {{ navigation .Page.Path }}
    </nav>

    <nav aria-label="{{ .T "aria_breadcrumb" }}">
        {{ range .Page.Breadcrumbs }}
            <a href="{{ .Path }}">{{ .Label }}</a> /
        {{ end }}
    </nav>

    <article>
        {{ .Page.Content }}

        {{ $editLink := editURL .Page.Path }}
        {{ if $editLink }}
        <footer><a href="{{ $editLink }}">{{ .T "edit_page" }}</a></footer>
        {{ end }}
    </article>

    {{ template "toc" . }}

    {{- if .Feature "color_chips" }}
    <script type="module">{{ inlineJSAsset "gmd-color-chip.mjs" }}</script>
    {{- end }}
</body>
</html>
```

## Web Components

gomddoc uses custom HTML elements (web components) for interactive UI features rendered from markdown. The goldmark
renderer emits semantic `<gmd-*>` elements, and JavaScript modules loaded by the theme bring them to life. This
separation keeps the rendering pipeline concerned with structure while themes control presentation.

All web components are prefixed with `gmd-` to avoid collisions with other custom elements.

### Built-in Components

| Element | Source File | DOM | Feature Flag | Description |
|---|---|---|---|---|
| `<gmd-color-chip>` | `gmd-color-chip.mjs` | Shadow | `color_chips` | Inline color swatch from hex codes. Click to copy. |
| `<gmd-admonition>` | `gmd-admonition.mjs` | Light | `admonitions` | Styled callout block (note, tip, warning, etc.) |
| `<gmd-heading-anchor>` | `gmd-heading-anchor.mjs` | Shadow | `heading_anchors` | Anchor link (#) appended to headings, revealed on hover |

### How They Work

**Server side (goldmark):** During markdown rendering, goldmark extensions detect specific patterns and emit custom
elements instead of plain HTML:

- Backtick-wrapped hex codes (`\`#FF5733\``) become `<gmd-color-chip>#FF5733</gmd-color-chip>`
- `> [!NOTE]` blockquotes become `<gmd-admonition type="note" title="Note">...</gmd-admonition>`
- Headings with IDs get `<gmd-heading-anchor href="#id"></gmd-heading-anchor>` appended

**Client side (JavaScript):** The theme's template loads the component modules conditionally based on feature flags:

```html
{{- if .Feature "color_chips" }}
<script type="module">{{ inlineJSAsset "gmd-color-chip.mjs" }}</script>
{{- end }}
{{- if .Feature "admonitions" }}
<script type="module">{{ inlineJSAsset "gmd-admonition.mjs" }}</script>
{{- end }}
{{- if .Feature "heading_anchors" }}
<script type="module">{{ inlineJSAsset "gmd-heading-anchor.mjs" }}</script>
{{- end }}
```

If a feature flag is disabled, the custom element tags remain inert in the HTML — they render as empty inline
elements with no visual effect, since the JavaScript that defines them is never loaded.

### Shadow DOM vs Light DOM

Components use one of two DOM strategies:

- **Shadow DOM** (`<gmd-color-chip>`, `<gmd-heading-anchor>`): Styles are encapsulated inside the component. The
  component renders identically across all themes without theme-specific CSS. Themes can customize appearance
  through CSS custom properties (inherited into Shadow DOM) or `::part()` selectors.
- **Light DOM** (`<gmd-admonition>`): No Shadow DOM encapsulation. The component's children are styled by the
  theme's global CSS. This is necessary when the inner content (paragraphs, code blocks, lists) must be styled
  by theme rules.

### Overriding Components

Themes can replace any built-in web component by providing their own `.mjs` file with the same name. The overlay
filesystem resolves assets in priority order:

1. **Theme-level** — `assets/themes/{name}/shared/gmd-color-chip.mjs`
2. **Shared** — `assets/shared/gmd-color-chip.mjs` (built-in default)

To customize a component in your site, place a replacement file in `.gomddoc/assets/shared/`:

```text
.gomddoc/
└── assets/
    └── shared/
        └── gmd-color-chip.mjs    ← Your custom implementation
```

Your custom component must call `customElements.define("gmd-color-chip", ...)` with the same element name.

### CSS Custom Properties

Shadow DOM components inherit CSS custom properties from the document. All built-in components use these shared
variables so they adapt to any theme's color scheme automatically:

| Property | Used By | Fallback |
|---|---|---|
| `--color-text` | color chip label | `#1f2328` |
| `--color-text-muted` | heading anchor link | `#64748b` |
| `--color-primary` | heading anchor hover | `#2563eb` |
| `--color-bg-secondary` | color chip background | `rgba(255,255,255,0.6)` |
| `--color-border` | color chip and swatch border | `rgba(46,52,64,0.1)` |
| `--font-family-mono` | color chip label | system monospace stack |

Themes that define these properties (all built-in themes do) get consistent component styling with no additional
work.

### CSS Parts

Shadow DOM components expose `::part()` selectors for targeted styling:

- **`<gmd-color-chip>`**: `::part(chip)`, `::part(swatch)`, `::part(label)`
- **`<gmd-heading-anchor>`**: `::part(link)`

Example theme override:

```css
gmd-color-chip::part(swatch) {
  border-radius: 4px;  /* square swatches instead of circles */
}
```

## Theme Features Checklist

When creating a custom theme, ensure it supports these features for parity with built-in themes. All optional features must be wrapped in `{{ .Feature "name" }}` guards so they can be toggled per-site and per-page:

- **Light/dark mode toggle** — guarded by `{{ .Feature "dark_mode" }}`. Uses `{{ inlineJSAsset "theme-toggle.mjs" }}` to manage `data-theme` attribute, localStorage, and `prefers-color-scheme` fallback. Customize button content with `data-light`/`data-dark` text attributes or `[data-show-theme]` children for SVG icons.
- **Navigation sidebar** via `{{ navigation .Page.Path }}`
- **Table of contents** — guarded by `{{ .Feature "toc" }}`. Uses `{{ template "toc" . }}` partial with `toc-item` recursive template.
- **TOC scroll highlighting** — guarded by `{{ .Feature "toc" }}`. Uses `{{ inlineJSAsset "toc-highlight.mjs" }}` to track scroll position and set `.active` class on matching TOC link.
- **Breadcrumbs** via `{{ range .Page.Breadcrumbs }}`
- **Search button** — guarded by `{{ .Feature "search" }}`. Button with `id="search-toggle"` in the header.
- **Search modal** — guarded by `{{ .Feature "search" }}`. Uses `{{ inlineJSAsset "search.mjs" }}` with CSS custom properties for styling.
- **Admonition styling** for `.admonition-note`, `.admonition-tip`, `.admonition-important`, `.admonition-warning`, `.admonition-caution`
- **Color chip web component** — guarded by `{{ .Feature "color_chips" }}`. Uses `{{ inlineJSAsset "gmd-color-chip.mjs" }}`.
- **Tag chips** — guarded by `{{ .Feature "tag_chips" }}`. Uses `{{ template "tag-chips" . }}`, rendering the page's `pageTags` as links to their `tagURL` landing pages.
- **See-also (related pages)** — guarded by `{{ .Feature "see_also" }}`. Uses `{{ template "see-also" . }}`, listing `.Page.RelatedDocs` (pages sharing frontmatter tags).
- **Copy-to-clipboard** — guarded by `{{ .Feature "code_copy" }}`. Uses `{{ inlineJSAsset "code-copy.mjs" }}`, creates `.copy-btn` on code blocks. Customize text with `data-copy-label`/`data-copied-label` on `<html>`.
- **Heading anchors** (`.heading-anchor` class, revealed on hover)
- **Touch accessibility** with `@media (hover: none)` for copy buttons and heading anchors
- **KaTeX** — guarded by `{{ .Feature "katex" }}`. CSS link in `<head>` and auto-render scripts.
- **Mermaid** — guarded by `{{ .Feature "mermaid" }}`. Script for diagram rendering (theme-aware: dark/light).
- **Canonical URLs and Open Graph tags** via `{{ canonicalURL .Page.Path }}` when domain is configured
- **Language switcher** via `{{ template "lang-switcher" . }}` — shown when multiple languages are detected
- **hreflang tags** via `{{ template "hreflang" . }}` — SEO alternate language links in `<head>`
- **Translated UI strings** via `{{ .T "key" }}` — all user-visible text should use translation keys
- **HTML lang attribute** via `{{ .Lang }}` on the `<html>` tag
- **Theme variables** via `{{ themeVarsCSS }}` — CSS custom properties from config
- **Responsive design** with mobile breakpoints
