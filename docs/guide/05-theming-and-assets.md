---
title: "Theming & Assets Guide"
description: "Guide on how to customize themes and manage assets in gomddoc."
author: "nicolasm"
---

# Theming & Assets

gomddoc uses an **Overlay Filesystem** to handle assets. This means you can "overlay" your own custom files on top of
the built-in defaults without replacing everything.

## Themes

Eight themes exist, but **only `default` is bundled with the binary** — it is the only one you can
select without first installing it. The other seven live in the separate
[`gomddoc-themes`](https://github.com/monolithiclab/gomddoc-themes) repository; copy the one you want
into your site's `.gomddoc/assets/themes/<name>/` overlay (see [Static Asset
Overlay](#static-asset-overlay)), which takes precedence over the embedded assets. Setting
`theme.name` to a theme you have not installed falls back to `default` and logs a
`Theme template not found, falling back to default` warning.

Set the theme in `.gomddoc/config.yml` or via `GOMDDOC_SITE_THEME_NAME`:

| Theme | Bundled | Style | Description |
|-------|---------|-------|-------------|
| `default` | ✅ | General purpose | Three-column layout (nav + content + TOC), Inter font |
| `academic` | download | Scholarly | Serif typography (Merriweather), justified text, warm parchment palette |
| `gitbook` | download | Documentation | Book-style reading (1.8 line height), tinted nav sidebar |
| `material` | download | Design system | Material Design 3, rounded corners, tonal elevation |
| `midnight` | download | Dark-first | Neon purple/cyan gradients, glowing code blocks |
| `minimal` | download | Brutalist | System fonts only, zero border-radius, heavy typographic hierarchy |
| `nord` | download | Color palette | Nord 16-color palette, frosted glass aesthetic |
| `ocean` | download | Colorful | Teal/navy gradients, SVG wave under the header |

All themes include: light/dark mode, TOC sidebar, navigation sidebar, breadcrumbs, admonitions, color chips,
copy-to-clipboard code blocks, heading anchors, KaTeX math, Mermaid diagrams, and touch device accessibility.

Every downloadable theme except `minimal` reads a `google_fonts` feature key that gates its Google Fonts stylesheet;
set `theme.features.google_fonts: false` to stop loading fonts from `fonts.googleapis.com`. The `default` theme does
not read this key and always loads Inter and JetBrains Mono from Google Fonts.

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
│   ├── partials/                        ← Site partials; override every theme's partials
│   │   └── footer.html.tmpl
│   ├── static/                          ← Site static files, served at /_assets/
│   │   └── logo.png
│   └── assets/
│       ├── shared/                      ← Overrides shared scripts (gmd-*.mjs, search.mjs, ...)
│       └── themes/
│           └── default/
│               └── layouts/
│                   └── default.html.tmpl  ← Overrides built-in layout
└── README.md
```

The `.gomddoc/` directory is layered over the files embedded in the binary: a file at `.gomddoc/assets/themes/...`
replaces the embedded `assets/themes/...` file of the same path. Resolution is whole-file; a replacement file is used
as-is, not merged. Locale files in `.gomddoc/locales/` are the exception: they override the built-in translations key by
key. The overlay works for Git sources too, since `.gomddoc/` is read from the content root.

### Static Asset Overlay

Static assets (CSS, JS, images, fonts) follow a three-layer priority system:

1. **Site-level** (`.gomddoc/static/`) — highest priority
2. **Theme-level** (`assets/themes/{name}/static/`, embedded or from `.gomddoc/assets/themes/{name}/static/`) —
   theme-specific assets. The default theme ships `favicon.ico` here.
3. **Shared** (`assets/shared/static/`, from `.gomddoc/assets/shared/static/`) — common assets across themes. The
   binary embeds no files in this layer.

All static assets are served at `/_assets/` without authentication, with an `ETag` and
`Cache-Control: public, max-age=31536000, immutable` (1-year, immutable). Directory paths and dotfiles return a
plain 404. `gomddoc build` copies the same layers into `_assets/` in the output directory.

Files under `.gomddoc/` are never served as content: the dot-directory is blocked on every content route (see
[Security](07-security.md#3-hidden-file-blocking)).

## Creating a Custom Theme

1.  **Define Theme:** In `.gomddoc/config.yml`:
    ```yaml
    theme:
      name: "custom"
    ```
2.  **Create Template:** Create `.gomddoc/assets/themes/custom/layouts/default.html.tmpl` with a
    skeleton that calls partials. Add partials in `.gomddoc/assets/themes/custom/partials/`
    (e.g., `head.html.tmpl`, `header.html.tmpl`, `nav.html.tmpl`, `toc.html.tmpl`, `scripts.html.tmpl`).

### Layouts and Partials

A theme has two directories:

- **`layouts/`** — full-page templates. `default.html.tmpl` renders content pages, tag pages and the tag index;
  `error.html.tmpl` renders error pages (404, 500, 501, ...). A page selects another layout with `layout: name` in
  its frontmatter, which loads `layouts/name.html.tmpl` from the active theme; when that file does not exist,
  `default.html.tmpl` is used. A layout the active theme lacks is taken from the default theme.
- **`partials/`** — every `*.html.tmpl` file is parsed alongside the layout, so any `{{ define "name" }}` block is
  callable with `{{ template "name" . }}`.

Partials are layered, highest priority last:

1. The default theme's partials (only for a non-default theme), so a theme overrides only what it changes.
2. The active theme's partials.
3. Site partials in `.gomddoc/partials/*.html.tmpl`, which override both.

The default theme defines these templates: `head`, `head-meta`, `head-katex`, `header`, `nav`, `nav-item`, `toc`,
`toc-item`, `tag-chips`, `see-also`, `footer`, `scripts`, `scripts-shared`, `hreflang`, `lang-switcher` and
`jsonld`.

Two partials render page bodies rather than page fragments, and receive their own data instead of the
`TemplateContext`: `tags-list` (the body of `/tags/{tag}`; fields `.Tag`, `.Pages`, `.Lang`, `.T`) and `tags-index`
(the body of `/tags/`; fields `.Tags`, each with `.Tag` and `.Count`, plus `.Lang` and `.T`). `.T` is a function
here: call it with `{{ call .T "key" }}`. gomddoc loads each from the active theme's `partials/` directory, falling
back to the default theme's, and wraps the result in `default.html.tmpl`.

### Template Variables

The template engine (Go `html/template`) receives a `TemplateContext` with:

- **`.Site`**: Global site configuration (`*config.SiteConfig`). Server settings (ports, timeouts, Git keys) are not
  reachable from templates.
    - `.Site.Meta.Title`: Site title.
    - `.Site.Meta.Description`: Site description.
    - `.Site.Meta.Domain`: Site domain.
    - `.Site.Meta.Robots`: Site-wide `robots` meta value.
    - `.Site.Language`: Default content language (BCP 47).
    - `.Site.Theme.Name`: Current theme name.
    - `.Site.Theme.Vars`: The `theme.vars` map (`map[string]string`).
    - `.Site.Theme.Features`: Site-level feature toggle map (`map[string]bool`). Use `.Feature` instead, which also
      applies page overrides.
    - `.Site.DefaultIndex`: Default index file name.
    - `.Site.DirIndex`: Whether directory listing is enabled.
    - `.Site.EditURL`: Base URL for "Edit this page" links.
    - `.Site.Highlighting.Theme`: Chroma syntax highlighting theme name.
    - `.Site.Search.Index`: Whether the search index is built.
    - `.Site.Exclude`, `.Site.StripExtensions`: The `exclude` and `strip_extensions` lists.

- **`.Page`**: Current page data (`PageContext`).
    - `.Page.Content`: The rendered HTML content (type `template.HTML`, safe for embedding).
    - `.Page.Path`: Current URL path, without the language prefix (the request path in `serve`, the page's URL in
      `build`).
    - `.Page.Meta`: Map of YAML Front Matter metadata (e.g., `{{ index .Page.Meta "title" }}`). `title` is always
      set: when the frontmatter has none, it is derived from the path. On error pages it holds `error_code`,
      `error_title`, `error_message` and `robots: noindex`.
    - `.Page.Features`: Pre-merged feature toggles (site defaults + page overrides).
    - `.Page.TOC`: Table of Contents tree (use the `toc` template function to render).
    - `.Page.RelatedDocs`: Pages sharing frontmatter tags with this page, rendered by the `see-also` partial.
    - `.Page.PrevPage` / `.Page.NextPage`: Adjacent pages in navigation order (each has `.Path` and `.Title`).
    - `.Page.Breadcrumbs`: Trail from the site root to this page (each has `.Path` and `.Label`). Generated once per
      page, not on demand — there is no `breadcrumbs` template function.
    - `.Page.Navigation`: Sidebar tree, or nil when navigation is disabled. `.Page.Navigation.Items` is a `[]NavItem`,
      each with `.Title`, `.Path`, `.IsDir`, `.Children`, plus `.Active` (this node is the current page) and `.Open`
      (this node or a descendant is). Like breadcrumbs, this is page data — there is no `navigation` template function;
      the theme owns the markup and walks the tree itself (see below).
    - `.Page.ModTime`: Source file modification time (`time.Time`); zero when unknown or when `meta.domain` is not
      set.

### Template Functions

Custom functions available in templates:

- **`toc`**: Returns a filtered list of `TOCNode` items for template rendering. Accepts optional min/max heading levels.
  Used with the recursive `toc-item` partial defined in `toc.html.tmpl`.
  ```html
  {{ $items := toc .Page.TOC }}           {{/* Default: h1-h2 */}}
  {{ $items := toc .Page.TOC 2 3 }}       {{/* Only h2-h3 */}}
  {{ range $items }}{{ template "toc-item" . }}{{ end }}
  ```

- **`editURL`**: Appends the given path to the configured `edit_url` base (trailing `/` on the base is trimmed, a
  leading `/` on the path is added). Returns empty string if `edit_url` is not configured.
  ```html
  {{ $editLink := editURL .Page.Path }}
  {{ if $editLink }}
      <a href="{{ $editLink }}">Edit this page</a>
  {{ end }}
  ```

- **`canonicalURL`**: Returns the full canonical URL (`https://{domain}{path}`) for a URL path, folding a trailing
  default index file into its directory. Returns empty string if `meta.domain` is not configured.
  ```html
  {{ $canonical := canonicalURL .Page.Path }}
  {{ if $canonical }}
      <link rel="canonical" href="{{ $canonical }}">
  {{ end }}
  ```

- **`.Feature`**: Method on `TemplateContext` that checks whether a named feature is enabled. Returns `true` by default
  for unknown features. Uses the pre-merged `Page.Features` map (site defaults + page frontmatter overrides).
  ```html
  {{- if .Feature "dark_mode" }}
  <button id="theme-toggle">Toggle Theme</button>
  {{- end }}
  {{- if .Feature "color_chips" }}
  <script type="module">{{ inlineJSAsset "gmd-color-chip.mjs" }}</script>
  {{- end }}
  ```

- **`contentURL`**: Returns the absolute URL path (without scheme or domain) for a content file. When extension
  stripping is active, returns the clean extensionless path. Default index files (e.g., `README.md`) are resolved to
  their directory path. On a non-default language page, the `/{lang}` prefix is added; do not add it in the template.
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

- **`inlineJSAsset`**: Loads an asset from the theme directory (`assets/themes/{name}/`), falling back to
  `assets/shared/`, and returns it as `template.JS` for embedding inside `<script>` tags. The name is a relative path;
  a missing asset fails the render. Assets are cached after the first read, except in dev mode.
  ```html
  <script type="module">{{ inlineJSAsset "gmd-color-chip.mjs" }}</script>
  <script type="module">{{ inlineJSAsset "search.mjs" }}</script>
  ```

- **`inlineCSSAsset`**: Loads an asset and returns it as `template.CSS` for safe embedding inside `<style>` tags.
  ```html
  <style>{{ inlineCSSAsset "custom.css" }}</style>
  ```

- **`inlineHTMLAsset`**: Loads an asset and returns it as `template.HTML` for safe embedding in HTML context (e.g.
  inline SVGs).
  ```html
  {{ inlineHTMLAsset "logo.svg" }}
  ```

- **`themeVarsCSS`**: Returns CSS custom properties generated from the `theme.vars` config, one `--theme-{key}` per
  entry in key order. Embed inside a `<style>` tag to make theme variables available to your CSS. Keys may contain
  only letters, digits and `-`; values containing `{`, `}`, `<`, `>` or `;` are skipped with a warning. Returns an
  empty string when no valid variable is set.
  ```html
  <style>{{ themeVarsCSS }}</style>
  {{/* Output: :root { --theme-primary-color: #2563eb; --theme-sidebar-width: 280px; } */}}
  ```

- **`jsonLD`**: Takes the `PageContext` (`.Page`) and returns the page's JSON-LD structured data as `template.JS`,
  without the `<script>` wrapper. Includes TechArticle and BreadcrumbList schemas, plus WebSite on index pages.
  Returns an empty string when `meta.domain` is not set. The default theme's `jsonld` partial wraps it; call that
  partial with `{{ template "jsonld" . }}`. See [SEO — Structured Data](11-seo.md#structured-data-json-ld) for
  details.
  ```html
  {{- $json := jsonLD .Page }}{{ if $json }}<script type="application/ld+json">{{ $json }}</script>{{ end }}
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

- **`.T "key"`**: Returns the translated string for the language of the content tree serving the page. Falls back
  to the default language, then to the key itself. A frontmatter `lang` changes `.Lang`, not the strings `.T`
  returns.
  ```html
  <span>{{ .T "toc_title" }}</span>
  ```

- **`.Lang`**: Returns the BCP 47 language code for the current page. Respects per-page `lang` frontmatter overrides.
  ```html
  <html lang="{{ .Lang }}">
  ```

- **`.Languages`**: Returns all available languages as `LanguageInfo` objects (`.Code`, `.Name`, `.Active`,
  `.Default`), default language first. Empty when the site has a single language, and on tag pages. Used to build a
  language switcher. The default language is served without a prefix.
  ```html
  {{- $path := .Page.Path }}
  {{- range .Languages }}
  <a href="{{ if .Default }}{{ $path }}{{ else }}/{{ .Code }}{{ $path }}{{ end }}">{{ .Name }}</a>
  {{- end }}
  ```

### Rendering the Navigation Sidebar

The tree arrives as data, so the theme owns the markup. Define a template that recurses on
`.Children` and invoke it once per top-level item — this is what the default theme's
`partials/nav.html.tmpl` does:

```html
{{ define "nav-item" -}}
<li>
  {{- if .IsDir -}}
  <details{{ if .Open }} open{{ end }}>
    <summary>{{ .Title }}</summary>
    {{- if .Children }}
    <ul>{{ range .Children }}{{ template "nav-item" . }}{{ end }}</ul>
    {{- end }}
  </details>
  {{- else -}}
  <a href="{{ .Path }}"{{ if .Active }} class="active"{{ end }}>{{ .Title }}</a>
  {{- end }}
</li>
{{- end }}
```

```html
{{- if and .Page.Navigation .Page.Navigation.Items }}
<nav aria-label="{{ .T "aria_site_nav" }}">
  <ul>{{ range .Page.Navigation.Items }}{{ template "nav-item" . }}{{ end }}</ul>
</nav>
{{- end }}
```

Guard on `.Page.Navigation` before `.Items`: the field is nil when navigation is disabled, and on
error pages. `.Open` is already set on every ancestor of the current page, so no theme-side path
comparison is needed to expand the right branches.

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

    {{- if and .Page.Navigation .Page.Navigation.Items }}
    <nav id="nav-sidebar" aria-label="{{ .T "aria_site_nav" }}">
        <ul>{{ range .Page.Navigation.Items }}{{ template "nav-item" . }}{{ end }}</ul>
    </nav>
    {{- end }}

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

**Client side (JavaScript):** The theme's template loads the component scripts conditionally based on feature flags.
The default theme's `scripts-shared` partial does this:

```html
{{- if .Feature "admonitions" }}
<script>{{ inlineJSAsset "gmd-admonition.mjs" }}</script>
{{- end }}
{{- if .Feature "heading_anchors" }}
<script>{{ inlineJSAsset "gmd-heading-anchor.mjs" }}</script>
{{- end }}
{{- if .Feature "color_chips" }}
<script type="module">{{ inlineJSAsset "gmd-color-chip.mjs" }}</script>
{{- end }}
```

The same feature flags gate the renderer. When a flag is off, the renderer emits no custom element: a hex code stays
a plain code span, an `> [!NOTE]` block stays a blockquote, and headings get no anchor.

### Shadow DOM vs Light DOM

Components use one of two DOM strategies:

- **Shadow DOM** (`<gmd-color-chip>`, `<gmd-heading-anchor>`): Styles are encapsulated inside the component. The
  component renders identically across all themes without theme-specific CSS. Themes can customize appearance
  through CSS custom properties (inherited into Shadow DOM) or `::part()` selectors.
- **Light DOM** (`<gmd-admonition>`): No Shadow DOM encapsulation. The component's children are styled by the
  theme's global CSS. This is necessary when the inner content (paragraphs, code blocks, lists) must be styled
  by theme rules.

### Overriding Components

Themes can replace any built-in web component by providing their own `.mjs` file with the same name. `inlineJSAsset`
resolves assets in priority order:

1. **Theme-level** — `assets/themes/{name}/gmd-color-chip.mjs`
2. **Shared** — `assets/shared/gmd-color-chip.mjs` (built-in default)

To customize a component in your site, place a replacement file in `.gomddoc/assets/shared/` (all themes) or
`.gomddoc/assets/themes/{name}/` (one theme):

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

Themes that define these properties (every shipped theme does) get consistent component styling with no additional
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

When creating a custom theme, ensure it supports these features for parity with the shipped themes. All optional
features must be wrapped in `{{ .Feature "name" }}` guards so they can be toggled per-site and per-page.
`gomddoc info` lists the feature keys and `--theme-*` variables the active theme reads (`theme.features`, `theme.vars`):

- **Light/dark mode toggle** — guarded by `{{ .Feature "dark_mode" }}`. Uses `{{ inlineJSAsset "theme-toggle.mjs" }}` to
  manage `data-theme` attribute, localStorage, and `prefers-color-scheme` fallback. Customize button content with
  `data-light`/`data-dark` text attributes or `[data-show-theme]` children for SVG icons.
- **Navigation sidebar** — range over `.Page.Navigation.Items` with a recursive `nav-item` template (see [Rendering the
  Navigation Sidebar](#rendering-the-navigation-sidebar)).
- **Table of contents** — guarded by `{{ .Feature "toc" }}`. Uses `{{ template "toc" . }}` partial with `toc-item`
  recursive template.
- **TOC scroll highlighting** — guarded by `{{ .Feature "toc" }}`. Uses `{{ inlineJSAsset "toc-highlight.mjs" }}` to
  track scroll position and set `.active` class on matching TOC link.
- **Breadcrumbs** via `{{ range .Page.Breadcrumbs }}`
- **Search button** — guarded by `{{ .Feature "search" }}`. Button with `id="search-toggle"` in the header.
- **Search modal** — guarded by `{{ .Feature "search" }}`. Uses `{{ inlineJSAsset "search.mjs" }}` with CSS custom
  properties for styling. Opens with Ctrl+K / Cmd+K; UI strings come from `data-search-*` attributes on `<html>`.
- **Admonition styling** for `.admonition-note`, `.admonition-tip`, `.admonition-important`, `.admonition-warning`,
  `.admonition-caution`
- **Color chip web component** — guarded by `{{ .Feature "color_chips" }}`. Uses
  `{{ inlineJSAsset "gmd-color-chip.mjs" }}`.
- **Tag chips** — guarded by `{{ .Feature "tag_chips" }}`. Uses `{{ template "tag-chips" . }}`, rendering the page's
  `pageTags` as links to their `tagURL` landing pages.
- **See-also (related pages)** — guarded by `{{ .Feature "see_also" }}`. Uses `{{ template "see-also" . }}`, listing
  `.Page.RelatedDocs` (pages sharing frontmatter tags).
- **Copy-to-clipboard** — guarded by `{{ .Feature "code_copy" }}`. Uses `{{ inlineJSAsset "code-copy.mjs" }}`, creates
  `.copy-btn` on code blocks. Customize text with `data-copy-label`/`data-copied-label`/`data-copy-aria` on `<html>`.
- **Heading anchors** — guarded by `{{ .Feature "heading_anchors" }}`. Uses
  `{{ inlineJSAsset "gmd-heading-anchor.mjs" }}`; the anchor is revealed on heading hover.
- **Admonitions** — guarded by `{{ .Feature "admonitions" }}`. Uses `{{ inlineJSAsset "gmd-admonition.mjs" }}`.
- **Touch accessibility** with `@media (hover: none)` for copy buttons (heading anchors handle it inside their shadow
  DOM)
- **KaTeX** — guarded by `{{ .Feature "katex" }}`. CSS link in `<head>` and auto-render scripts.
- **Mermaid** — guarded by `{{ .Feature "mermaid" }}`. Script for diagram rendering (theme-aware: dark/light).
- **Canonical URLs and Open Graph tags** via `{{ canonicalURL .Page.Path }}` when domain is configured, or by calling
  the default theme's `{{ template "head-meta" . }}`
- **JSON-LD** via `{{ template "jsonld" . }}`
- **Edit link** via `{{ editURL .Page.Path }}` (the default theme's `footer` partial)
- **Error layout** — `layouts/error.html.tmpl`, reading `.Page.Meta.error_code`, `error_title` and `error_message`
- **Language switcher** via `{{ template "lang-switcher" . }}` — shown when multiple languages are detected
- **hreflang tags** via `{{ template "hreflang" . }}` — SEO alternate language links in `<head>`
- **Translated UI strings** via `{{ .T "key" }}` — all user-visible text should use translation keys
- **HTML lang attribute** via `{{ .Lang }}` on the `<html>` tag
- **Theme variables** via `{{ themeVarsCSS }}` — CSS custom properties from config
- **Responsive design** with mobile breakpoints
