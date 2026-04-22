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

The template engine (Go `html/template`) receives a context object with:

*   **`.Site`**: Global site configuration.
    *   `.Site.Meta.Title`: Site title.
    *   `.Site.Meta.Description`: Site description.
    *   `.Site.Meta.Domain`: Site domain.
    *   `.Site.Theme.Name`: Current theme name.

*   **`.Page`**: Current page data.
    *   `.Page.Content`: The rendered HTML content (safe HTML).
    *   `.Page.Breadcrumbs`: Map of paths to labels for navigation.
    *   `.Page.Meta`: Map of Front Matter metadata (e.g., `{{ .Page.Meta.title }}`, `{{ .Page.Meta.tags }}`).

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
        {{ range $path, $label := .Page.Breadcrumbs }}
            <a href="{{ $path }}">{{ $label }}</a> /
        {{ end }}
    </nav>

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
