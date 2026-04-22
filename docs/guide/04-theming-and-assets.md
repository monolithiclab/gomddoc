---
title: "Theming & Assets Guide"
description: "Guide on how to customize themes and manage assets in gomddoc."
author: "nicolasm"
---

# Theming & Assets

gomddoc uses an **Overlay Filesystem** to handle assets. This means you can "overlay" your own custom files on top of the built-in defaults without replacing everything.

## Directory Structure

To customize your site, create a `.gomddoc` folder in your content root:

```text
/my-docs
├── .gomddoc/
│   ├── config.yml
│   └── assets/
│       └── themes/
│           └── default/
│               ├── layout.html.tmpl  <-- Overrides built-in layout
│               └── style.css         <-- Adds/Overrides CSS
└── README.md
```

## Creating a Custom Theme

1.  **Define Theme:** In `.gomddoc/config.yml`:
    ```yaml
    theme:
      name: "custom"
    ```
2.  **Create Template:** Create `.gomddoc/assets/themes/custom/layout.html.tmpl`.

### Template Variables

The template engine (Go `html/template`) receives a `TemplateContext` with:

- **`.Site`**: Global site configuration (`*config.SiteConfig`).
    - `.Site.Meta.Title`: Site title.
    - `.Site.Meta.Description`: Site description.
    - `.Site.Meta.Domain`: Site domain.
    - `.Site.Theme.Name`: Current theme name.
    - `.Site.DefaultIndex`: Default index file name.
    - `.Site.DirIndex`: Whether directory listing is enabled.

- **`.Page`**: Current page data (`PageContext`).
    - `.Page.Content`: The rendered HTML content (type `template.HTML`, safe for embedding).
    - `.Page.Path`: Current request URL path.
    - `.Page.Meta`: Map of YAML Front Matter metadata (e.g., `{{ index .Page.Meta "title" }}`).
    - `.Page.TOC`: Table of Contents tree (use the `toc` template function to render).

### Template Functions

Two custom functions are available in templates:

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

### Example Layout

```html
<!DOCTYPE html>
<html>
<head>
    <title>{{ .Site.Meta.Title }}</title>
    <meta name="description" content="{{ .Site.Meta.Description }}">
</head>
<body>
    <nav>
        {{ range breadcrumbs .Page.Path }}
            <a href="{{ .Path }}">{{ .Label }}</a> /
        {{ end }}
    </nav>

    <aside>
        {{ toc .Page.TOC 2 3 }}
    </aside>

    <main>
        {{ .Page.Content }}
    </main>

    <footer>
        Served by gomddoc
    </footer>
</body>
</html>
```

## Asset Serving

Any file placed in `.gomddoc/assets/` can be accessed via `/_assets/` (or relative paths depending on theme implementation, though direct asset serving logic maps usually to the root or specific asset handlers).

*Currently, `gomddoc` serves content directly. For theme assets (CSS/JS), they should be referenced relative to the theme structure or served as static files if exposed.*

*(Note: The current implementation primarily embeds the layout. Static asset serving for themes might require specific handler mapping which is standard in the default theme).*
