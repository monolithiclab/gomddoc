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
  mobile-light: "screenshots/mobile-light.png"
  mobile-dark: "screenshots/mobile-dark.png"
---

# Default

The default gomddoc theme. A clean, modern documentation layout with a left navigation sidebar,
right-hand table of contents, and full dark mode support. Uses Inter for body text and JetBrains
Mono for code.

Light mode uses a crisp white background with blue accents. Dark mode switches to a deep
purple-tinted background with indigo highlights and a subtle gradient border on code blocks.

Features sticky header with backdrop blur, breadcrumb navigation, collapsible nav sections,
scroll-tracked TOC highlighting, and copy-to-clipboard buttons on code blocks. Admonitions use
color-coded left borders with tinted backgrounds. Responsive layout hides the TOC on tablets and
collapses the nav sidebar into a hamburger menu on mobile.

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
