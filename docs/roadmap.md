---
title: "Gomddoc Roadmap"
description: "Strategic roadmap for gomddoc evolution."
author: "nicolasm"
---

# Gomddoc Roadmap

## Vision

Transform gomddoc from a documentation server into the best git-native documentation viewer and static site
generator. No databases, no editorial workflows, no CMS. The "database" is Git.

- **Read-heavy focus**: Search, speed, and rendering quality over write features
- **Stateless**: No persistent storage beyond Git repos
- **Interface-driven**: All major components implement well-defined interfaces
- **Performance-first**: Built for speed with intelligent caching
- **Security by design**: Path traversal protection, content sanitization, secure defaults

## Current State (Phase 7 In Progress)

The foundation is production-ready with comprehensive test coverage across internal packages:

- **CLI**: Kong-based subcommand architecture (`gomddoc serve`, `gomddoc build`), version injection via ldflags,
  exhaustive `--help` with env var discovery.
- **Configuration**: Reflection-based env var walking, CLI flags, YAML config files with proper validation
  and correct prefix nesting (`GOMDDOC_SITE_*`). Priority: flags > env > file > defaults.
- **Providers**: Filesystem (os.DirFS with path traversal protection) and Git (go-git, memory or disk-based
  storage, SSH auth with known_hosts, fail-closed).
- **Rendering**: Markdown (goldmark, GFM, syntax highlighting, admonitions, color chips, TOC, heading anchors,
  YAML frontmatter), passthrough for all other MIME types via `*/*` wildcard.
- **HTTP**: Content negotiation, gzip compression, security headers, ETag, request ID tracking, graceful shutdown.
- **Monitoring**: Prometheus metrics (`/metrics`), health probes (`/health/live`, `/health/ready`).
- **Security**: Hidden file blocking, method filtering (GET/HEAD), clone timeout, file size limits.
- **Navigation**: Auto-generated sidebar from directory structure with collapsible directories, active state
  tracking, and title extraction from markdown headings.
- **Metadata**: Frontmatter indexing across all pages with JSON API (`/api/tags`, `/api/tags/{tag}`).
- **Static site generation**: `gomddoc build` command for deploying to S3, Netlify, GitHub Pages.
- **Edit links**: Configurable `edit_url` in site config with "Edit this page" footer links.
- **Documentation**: User guides (`docs/guide/`), architecture reference (`docs/architecture.md`).

See `docs/architecture.md` for detailed architecture and `docs/guide/` for user documentation.

## Phase 4: Performance and Scaling (Partial)

- [x] **Disk-based Git storage**: `DiskStorageFactory` for the Git provider via `--git-storage-dir` flag.
      Prevents OOM on large repos by cloning to disk instead of memory.
- [ ] **Partial clones**: `git clone --filter=blob:none` when upstream library support matures.
- [ ] in dev mode, support automatic port assignment so that agents can work concurrently on multiple instances. if called with `--dev -p ":auto", then it should look for an available port starting with the default one.

## Phase 5: Search and Discovery (Partial)

- [ ] **Full-text search**: Decision pending — Option C (stdlib inverted index) for serve, Option B (Pagefind)
      for build. See `docs/decisions.md` for analysis.
- [x] **Auto-navigation**: Sidebar navigation generated from directory structure with collapsible `<details>`
      elements, title extraction, and active path highlighting.
- [x] **Metadata indexing**: Lightweight frontmatter parser indexes tags/categories across all pages.
      JSON API: `GET /api/tags` and `GET /api/tags/{tag}`.

## Phase 6: Renderer Enhancement

_Expand support for technical documentation formats._

- [ ] **AsciiDoc support**: Renderer for `.adoc` files (popular in technical writing).
- [ ] **OpenAPI renderer**: Render `swagger.yaml` / `openapi.json` as interactive API docs.
- [ ] **Rich Markdown**: Native Mermaid diagrams, MathJax/KaTeX (currently client-side CDN only).

## Phase 7: Static Site Generation (Partial)

- [x] **`gomddoc build` command**: Walk content tree, render markdown through template pipeline, output
      static HTML. Generates `index.html` alongside `README.html` for clean URLs. Copies non-markdown files as-is.
- [ ] **Asset optimization**: Minify HTML/CSS/JS during build.
- [ ] **Static host compatibility**: Output structure compatible with S3, Netlify, Cloudflare Pages.

## Phase 8: Theming Engine

_Transform themes from monolithic templates into composable, configurable, distributable packages._

### 8a: Theme Folder Structure

Rework the current flat structure (`<theme>/layout.html.tmpl`) into a well-defined package layout
that supports partials, multiple page types, and static assets.

- [ ] **Canonical structure**: Restructure each theme directory from the current flat layout to:
  ```
  <theme>/
  ├── README.md              # Metadata (frontmatter) and description
  ├── layouts/
  │   ├── layout.html.tmpl   # Base layout (required)
  │   ├── page.html.tmpl     # Page type overrides (optional)
  │   └── ...
  ├── partials/
  │   ├── header.html.tmpl
  │   ├── footer.html.tmpl
  │   ├── nav.html.tmpl
  │   ├── toc.html.tmpl
  │   ├── head.html.tmpl
  │   └── scripts.html.tmpl
  ├── static/                # Theme-specific static assets (CSS, JS, images, fonts)
  └── screenshots/
      ├── light.png
      └── dark.png
  ```
- [ ] **Migration**: Move existing `layout.html.tmpl` into `layouts/`, screenshots into `screenshots/`.
      Maintain backward compatibility: if `layout.html.tmpl` exists at root, use it (legacy mode).
- [ ] **Embedded themes update**: Rework all bundled themes (`cmd/gomddoc/assets/themes/*`) to the
      new structure. Update `go:embed` directives and the overlay filesystem accordingly.

### 8b: Template Partials

Break the monolithic `layout.html.tmpl` into composable partials that themes can selectively override.

- [ ] **Partial system**: Split layout into `header.html.tmpl`, `footer.html.tmpl`, `nav.html.tmpl`,
      `toc.html.tmpl`, `head.html.tmpl`, `scripts.html.tmpl`. Main layout assembles partials via
      `{{ template "header" . }}`.
- [ ] **Partial override resolution**: Theme provides base partials; site `.gomddoc/partials/` overrides
      specific ones without copying the whole theme. Resolution order: site partials > theme partials > default.
- [x] **UI polish**: Copy-to-clipboard for code blocks.
- [x] **Dark mode**: Native light/dark toggle.

### 8c: Theme Variables

Expose theme colors and typography as named variables defined in the theme `README.md` frontmatter and
overridable from the site config.

- [ ] **Variable extraction**: Parse theme `README.md` frontmatter `colors` map (light/dark background,
      text, primary, etc.) into CSS custom properties injected at render time.
- [ ] **Config overrides**: Allow `theme_vars` in `.gomddoc/config.yml` to override any theme variable
      without forking the theme. Example: `theme_vars: { primary: "#e63946" }`.
- [ ] **Extended palette**: Themes may define additional variables (accent, border, code-bg, etc.) beyond
      the required `background`, `text`, `primary`. All are overridable.

### 8d: Page Types

Support multiple layout variants selectable from content frontmatter.

- [ ] **Page type templates**: Themes provide a generic `layout.html.tmpl` (default) plus optional
      type-specific layouts: `page.html.tmpl`, `api.html.tmpl`, `changelog.html.tmpl`, etc.
- [ ] **Frontmatter `layout` field**: Content files select their layout via `layout: api` in frontmatter.
      Falls back to `layout.html.tmpl` if the specified type doesn't exist in the theme.
- [ ] **Layout inheritance**: Type-specific layouts can extend the base layout, overriding only the
      content block while inheriting header, footer, and scripts.

### 8e: Static Asset Serving

Serve theme-specific and shared static files (JS, CSS, images, fonts) via a dedicated `/_assets/` route,
using the overlay filesystem to allow themes to override shared resources.

- [ ] **Shared static directory**: Introduce `assets/shared/static/` in the embedded FS for cross-theme
      resources (web components, shared JS libraries, common icons). Served at `/_assets/shared/`.
- [ ] **Theme static directory**: Each theme's `static/` folder is served at `/_assets/theme/`. Contains
      theme-specific CSS, JS, images, and fonts that are too large or inappropriate to inline.
- [ ] **Overlay resolution for statics**: Build the static file FS as an overlay stack:
      site `.gomddoc/static/` > theme `static/` > `shared/static/`. This lets themes override shared
      resources (e.g. a custom web component variant) and sites override theme resources without forking.
      Reuses the existing `OverlayFS` implementation.
- [ ] **`/_assets/` HTTP handler**: Register a handler that serves files from the static overlay FS
      with proper MIME types, ETags, and cache headers. Block directory listings and dotfiles.
- [ ] **Template `assetURL` function**: Provide `{{ assetURL "color-chip.js" }}` in templates that
      resolves to `/_assets/theme/color-chip.js` (or `/_assets/shared/...` for shared assets). Replaces
      the current `{{ asset }}` inline approach with external `<script src>` / `<link href>` references.
- [ ] **Migrate inline JS**: Move the current `{{ asset "color-chip.js" }}` inline pattern to
      `<script type="module" src="{{ assetURL "color-chip.js" }}"></script>` once static serving is live.
      Keep `{{ asset }}` available as a fallback for small snippets where inlining is preferred.
- [ ] **Build mode**: `gomddoc build` copies the resolved static overlay into the output `_assets/`
      directory. Shared and theme statics are merged with the same override precedence.

## Phase 9: Enterprise Features

- [ ] **S3 provider**: Serve content directly from S3 buckets (for non-git use cases).
- [ ] **Authentication**: OIDC/OAuth middleware for private documentation.
- [ ] **Branch switching**: UI dropdown to switch between Git branches/tags.

## Phase 10: Git Workflow Integration

- [ ] **Webhooks**: Endpoint to trigger `git fetch` on push events (cache invalidation).
- [ ] **PR preview**: Serve content from PR branches for review.

## Phase 11: Theme Marketplace

_Enable community theme sharing via a GitHub-based registry._

### 11a: Theme Registry

- [ ] **Default marketplace repository**: A GitHub repository (e.g. `gomddoc/themes`) acts as the theme
      registry. Contains an `index.json` manifest listing available themes with name, description, author,
      version, repository URL, and preview image URLs.
- [ ] **Theme package format**: Each theme is a Git repository containing `layout.html.tmpl`, optional
      partials, a `README.md` with frontmatter metadata (name, category, fonts, colors), and screenshot
      previews (`light.png`, `dark.png`).
- [ ] **Custom registries**: Users can configure alternative marketplace URLs in `.gomddoc/config.yml`
      via `theme_registry: https://github.com/org/custom-themes` for private or corporate theme registries.

### 11b: `gomddoc theme` Subcommand

- [ ] **`gomddoc theme list`**: Fetch and display available themes from the marketplace. Shows name,
      category, author, and short description in a table. Supports `--json` output.
- [ ] **`gomddoc theme search <query>`**: Filter themes by name, category, or keyword. Example:
      `gomddoc theme search dark` finds midnight, nord, etc.
- [ ] **`gomddoc theme info <name>`**: Show detailed theme information: full description, color palette,
      fonts, screenshot URLs, download URL, and installation instructions.
- [ ] **`gomddoc theme install <name>`**: Download the theme into `.gomddoc/themes/<name>/`. Clones the
      theme repository (sparse checkout of the theme directory if from the default registry). Updates
      `.gomddoc/config.yml` to set `theme: <name>`.
- [ ] **`gomddoc theme update [name]`**: Pull latest version of installed theme(s). Without a name,
      updates all installed themes.

## Future CLI Commands

| Command                 | Purpose                                              | Status  |
| ----------------------- | ---------------------------------------------------- | ------- |
| `gomddoc serve`         | HTTP server for live documentation                   | Done    |
| `gomddoc build`         | Static site generation                               | Done    |
| `gomddoc init`          | Scaffold a `.gomddoc/` directory with default config | Planned |
| `gomddoc validate`      | Validate config and check for broken links           | Planned |
| `gomddoc theme list`    | List available themes from the marketplace           | Planned |
| `gomddoc theme search`  | Search themes by name, category, or keyword          | Planned |
| `gomddoc theme info`    | Show detailed theme information                      | Planned |
| `gomddoc theme install` | Download and install a theme                         | Planned |
| `gomddoc theme update`  | Update installed theme(s) to latest version          | Planned |

## Implementation Strategy

Development proceeds in phases building on stable foundations. Each phase delivers complete, tested functionality.

**Immediate focus (Phase 5 & 6):** Full-text search for content discovery, and expanding renderer support.

## Deferred (Not Planned)

These features were considered but deprioritized to keep gomddoc focused:

- Database providers (PostgreSQL/SQLite) — Git is the database
- REST/GraphQL APIs — gomddoc is a viewer, not a headless CMS
- Editorial workflows (drafts, reviews, scheduling) — use Git branches
- Content versioning/revision history — use Git history
- i18n/multi-language — out of scope for now
- VS Code extension
- Plugin architecture / dynamic renderer loading
