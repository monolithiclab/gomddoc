---
name: "Default"
category: "General Purpose"
tags: ["clean", "modern", "responsive", "dark-mode"]
author: "gomddoc"
license: "MIT"
fonts:
  heading: "Inter"
  body: "Inter"
  mono: "JetBrains Mono"
colors:
  light:
    background: "#ffffff"
    text: "#1e293b"
    primary: "#2563eb"
  dark:
    background: "#13111C"
    text: "#e2e8f0"
    primary: "#818CF8"
features:
  - "light-dark-mode"
  - "toc-scroll-highlighting"
  - "collapsible-navigation"
  - "breadcrumbs"
  - "admonitions"
  - "color-chips"
  - "code-copy"
  - "heading-anchors"
  - "katex"
  - "mermaid"
  - "responsive"
  - "touch-accessible"
screenshots:
  desktop-light: "screenshots/desktop-light.png"
  desktop-dark: "screenshots/desktop-dark.png"
---

# Default

The default gomddoc theme. A clean, modern documentation layout with a left navigation sidebar,
right-hand table of contents, and full dark mode support. Uses Inter for body text and JetBrains
Mono for code.

Light mode uses a white background with blue accents. Dark mode switches to a deep
purple-tinted background with indigo highlights. Without a stored choice, the theme follows the
system `prefers-color-scheme` setting.

Features a sticky header with backdrop blur and a gradient bottom border, breadcrumb navigation,
collapsible nav sections, scroll-tracked TOC highlighting, a search modal (Ctrl+K / Cmd+K),
copy-to-clipboard buttons on code blocks, tag chips, a "See also" section, an "Edit this page"
link when `edit_url` is set, and a language switcher on multi-language sites. Admonitions use
color-coded left borders with tinted backgrounds. Below 1200px the TOC sidebar moves behind a
toggle button; below 992px the nav sidebar collapses into a hamburger menu.

Every optional part is gated by a feature toggle: `admonitions`, `code_copy`, `color_chips`,
`dark_mode`, `heading_anchors`, `katex`, `mermaid`, `search`, `see_also`, `tag_chips` and `toc`.

## CSS Variables

The following CSS variables can be overridden via `theme.vars` in `.gomddoc/config.yml`:

| Variable | Default (Light) | Default (Dark) | Description |
|----------|----------------|----------------|-------------|
| `bg` | `#ffffff` | — | Page background |
| `text` | `#1e293b` | — | Primary text color |
| `primary` | `#2563eb` | — | Accent/link color |
| `dark-bg` | — | `#13111C` | Dark mode background |
| `dark-text` | — | `#e2e8f0` | Dark mode text |
| `dark-primary` | — | `#818CF8` | Dark mode accent |

Example configuration:

```yaml
theme:
  name: default
  vars:
    bg: "#f5f5f5"
    primary: "#0066cc"
    dark-bg: "#1a1a2e"
    dark-primary: "#4da6ff"
```

![Light mode](screenshots/desktop-light.png)
![Dark mode](screenshots/desktop-dark.png)
