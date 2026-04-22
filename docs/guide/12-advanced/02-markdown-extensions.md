---
title: "Markdown Extensions"
description: "Markdown extensions supported by gomddoc: GFM, frontmatter, admonitions, KaTeX math, Mermaid diagrams, and color chips."
author: "nicolasm"
tags: ["markdown", "extensions"]
---

# Markdown Extensions

gomddoc supports several Markdown extensions beyond standard CommonMark, powered by goldmark and client-side rendering libraries.

## GitHub Flavored Markdown (GFM)

All GFM extensions are enabled by default:

- **Tables**: Pipe-delimited table syntax
- **Strikethrough**: `~~deleted text~~`
- **Task lists**: `- [x] completed` / `- [ ] pending`
- **Autolinks**: URLs are automatically linked

## YAML Front Matter

Documents can include YAML metadata at the top:

```markdown
---
title: My Document
description: A brief summary of this page
author: Jane Doe
tags: [go, markdown]
date: 2026-03-25
og_type: article
features:
  color_chips: true
---

# Content starts here
```

### Standard Fields

gomddoc processes these frontmatter fields with special behavior:

| Field | Type | Description |
|-------|------|-------------|
| `title` | string | Page title — used in `<title>`, Open Graph tags, search results, and the tags API |
| `description` | string | Page description — used in `<meta name="description">`, Open Graph, and search results |
| `tags` | array | Page tags — normalized to lowercase, queryable via `/api/tags` endpoint |
| `date` | string | Publication date (`YYYY-MM-DD`) — included in tags API responses |
| `og_type` | string | Open Graph type (defaults to `article`) — controls `<meta property="og:type">` |
| `features` | map | Per-page feature toggle overrides (e.g., `features: { color_chips: false }`) |

### Custom Fields

Any other frontmatter fields are stored in the page's metadata map and accessible in templates via `{{ index .Page.Meta "field_name" }}`. They also appear in the tags API response under the `meta` object.

```yaml
---
title: API Reference
author: Jane Doe
category: reference
custom_field: custom_value
---
```

Access in templates:

```html
{{ if index .Page.Meta "author" }}
  <span>By {{ index .Page.Meta "author" }}</span>
{{ end }}
```

## Admonitions (Callout Blocks)

Admonitions are styled callout blocks that highlight important information. They use the same syntax as [GitHub's alerts](https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax#alerts).

### Syntax

Write a blockquote that starts with a type marker on the first line:

```markdown
> [!NOTE]
> This is a note with useful information.

> [!TIP]
> This is a helpful tip for the reader.

> [!IMPORTANT]
> This is important information that should not be missed.

> [!WARNING]
> This is a warning about potential issues.

> [!CAUTION]
> This is a caution about dangerous operations.
```

### Supported Types

| Type        | Purpose                                      | Color  |
|-------------|----------------------------------------------|--------|
| `NOTE`      | Useful information the reader should know     | Blue   |
| `TIP`       | Helpful advice for better results             | Green  |
| `IMPORTANT` | Key information that should not be overlooked | Purple |
| `WARNING`   | Potential issues or things to be aware of     | Orange |
| `CAUTION`   | Dangerous operations or critical warnings     | Red    |

### Multi-Line Content

Admonitions can contain multiple lines and paragraphs:

```markdown
> [!WARNING]
> This is the first paragraph of the warning.
>
> This is a second paragraph with more details.
```

The type markers are case-insensitive: `[!NOTE]`, `[!note]`, and `[!Note]` all produce the same result. Regular blockquotes (those without a type marker) are not affected.

## Math Rendering (KaTeX)

gomddoc supports mathematical notation using KaTeX, rendered client-side in the browser. Both inline and display (block) math are supported.

### Inline Math

Use single dollar signs to wrap inline math expressions:

```markdown
The quadratic formula is $x = \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}$ which solves any quadratic equation.
```

### Display Math

Use double dollar signs for display (block) math equations:

```markdown
$$
\int_{-\infty}^{\infty} e^{-x^2} dx = \sqrt{\pi}
$$
```

### Supported Features

KaTeX supports a large subset of LaTeX math commands including:

- Greek letters: `$\alpha$`, `$\beta$`, `$\gamma$`
- Fractions: `$\frac{a}{b}$`
- Subscripts and superscripts: `$x_i^2$`
- Matrices and arrays
- Summations and integrals: `$\sum_{i=1}^{n} i$`
- Common operators and symbols

For a complete list of supported functions, see the [KaTeX documentation](https://katex.org/docs/supported.html).

### How It Works

Math rendering uses the KaTeX auto-render extension loaded from the jsDelivr CDN. When a page loads, the script scans the page content for `$...$` (inline) and `$$...$$` (display) delimiters and renders them as formatted math. No server-side processing or additional dependencies are required.

### Escaping Dollar Signs

If you need to display a literal dollar sign without triggering math rendering, use a backslash: `\$`. For example, `\$100` renders as a plain dollar amount.

## Mermaid Diagrams

gomddoc supports Mermaid diagrams using fenced code blocks with the `mermaid` language identifier:

````markdown
```mermaid
graph TD
    A[Start] --> B{Decision}
    B -->|Yes| C[Action]
    B -->|No| D[End]
```
````

Mermaid diagrams are rendered client-side using the Mermaid JavaScript library loaded from the jsDelivr CDN. Supported diagram types include flowcharts, sequence diagrams, Gantt charts, class diagrams, and more. See the [Mermaid documentation](https://mermaid.js.org/) for details.

## Color Chips

Hex color codes wrapped in backticks are automatically rendered as interactive color swatches using a `<color-chip>` web component.

### Syntax

Simply wrap a hex color code in backticks:

```markdown
The primary color is `#2563eb` and the accent is `#ec4899`.
```

Both 3-digit (`#fff`) and 6-digit (`#ffffff`) hex codes are supported. The color chip displays a small swatch next to the hex code. Clicking the chip copies the hex value to your clipboard.

### Controlling Color Chips

Color chips are enabled by default. You can disable them globally in `.gomddoc/config.yml`:

```yaml
features:
  color_chips: false
```

Or per-page via frontmatter:

```markdown
---
features:
  color_chips: false
---
# My Page Without Color Chips
```

Per-page frontmatter overrides the global setting. Hex codes inside fenced code blocks are never transformed.

## Heading Anchors

All headings with auto-generated IDs get clickable anchor links. The anchor (`#`) appears when you hover over a heading (or is always visible on touch devices). Clicking the anchor updates the URL hash for easy linking to specific sections.

## Table of Contents

gomddoc automatically generates a table of contents from headings (h1-h3) in Markdown documents. The TOC appears as a sidebar on desktop and a toggleable panel on mobile. Heading IDs are auto-generated for anchor linking.

### TOC Scroll Highlighting

As you scroll through a page, the TOC sidebar automatically highlights the currently visible section. The active heading is tracked and the TOC auto-scrolls to keep the active item centered. This works across all built-in themes.
