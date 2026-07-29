---
title: "Configuration Guide"
description: "Detailed guide on configuring gomddoc via CLI flags, environment variables, and config files."
author: "nicolasm"
---

# Configuration

gomddoc uses a layered configuration system. Settings come from three sources, applied in order of increasing priority:

1. **Defaults** — hardcoded sensible values
2. **Config file** — `.gomddoc/config.yml` in your content directory
3. **Environment variables** — `GOMDDOC_*` prefixed vars
4. **CLI flags/arguments** — always win

When the same setting is defined in multiple places, the highest-priority source wins.

---

## CLI Reference

### `serve`

Production HTTP server.

```bash
gomddoc serve [DIR] [flags]
```

| Flag/Arg | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `DIR` (arg) | `GOMDDOC_SERVER_DIR` | `.` | Content directory or Git URL |
| `-p, --port` | `GOMDDOC_SERVER_PORT` | `:8080` | Listen address (`:auto` for auto-assign) |
| `-d, --domain` | `GOMDDOC_DOMAIN` | | Override site domain for SEO (canonical, sitemap, etc.) |
| `--git-key-file` | `GOMDDOC_SERVER_GIT_SSH_KEY` | | SSH key for private Git repos |
| `--git-storage-dir` | `GOMDDOC_SERVER_GIT_STORAGE_DIR` | | Disk-based Git clone directory |
| `--pprof` | `GOMDDOC_SERVER_PPROF` | `false` | Enable profiling endpoints |
| `--admin-port` | `GOMDDOC_SERVER_ADMIN_PORT` | | Separate listen address for admin endpoints (health, metrics, pprof) |
| `--basic-auth-file` | `GOMDDOC_SERVER_BASIC_AUTH_FILE` | | Path to htpasswd file (bcrypt only) |

### `preview`

Quick preview with dev mode enabled by default.

```bash
gomddoc preview [DIR] [flags]
```

| Flag/Arg | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `DIR` (arg) | `GOMDDOC_SERVER_DIR` | `.` | Content directory or Git URL |
| `-p, --port` | `GOMDDOC_SERVER_PORT` | `:auto` | Listen address (auto-assigns from 8080) |
| `-d, --domain` | `GOMDDOC_DOMAIN` | | Override site domain for SEO (canonical, sitemap, etc.) |
| `--git-key-file` | `GOMDDOC_SERVER_GIT_SSH_KEY` | | SSH key for private Git repos |
| `--git-storage-dir` | `GOMDDOC_SERVER_GIT_STORAGE_DIR` | | Disk-based Git clone directory |
| `--open` | `GOMDDOC_PREVIEW_OPEN` | `false` | Auto-open browser on startup |
| `--dir-index` | `GOMDDOC_DIR_INDEX` | `false` | Enable directory listings when no index file exists |

Preview enables dev mode (no caching, template re-parsing on every request) and defaults to automatic port assignment.

### `build`

Generate a static site from markdown content.

```bash
gomddoc build [DIR] [flags]
```

| Flag/Arg | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `DIR` (arg) | `GOMDDOC_SERVER_DIR` | `.` | Markdown source directory or Git URL |
| `-o, --output` | `GOMDDOC_BUILD_OUTPUT` | `build/site` | Output directory |
| `-d, --domain` | `GOMDDOC_DOMAIN` | | Override site domain for SEO (canonical, sitemap, etc.) |
| `--git-key-file` | `GOMDDOC_SERVER_GIT_SSH_KEY` | | SSH key for private Git repos |
| `--git-storage-dir` | `GOMDDOC_SERVER_GIT_STORAGE_DIR` | | Disk-based Git clone directory |

**Build behavior:**
- A `.gomddoc-build` sentinel file is written to the output directory. On subsequent builds, the sentinel proves the directory was created by gomddoc and is safe to overwrite. Non-empty directories without the sentinel are refused.
- Markdown files are rendered to HTML through the full template pipeline
- `README.md` files generate both `README.html` and `index.html` for clean URLs (unless `index.md` exists in the same directory)
- Non-markdown files (images, CSS, JS) are copied as-is
- Hidden files (starting with `.`) are skipped
- `robots.txt` is always generated
- `sitemap.xml` is generated when `meta.domain` is configured
- `404.html` is generated for static host compatibility (Netlify, GitHub Pages, Cloudflare Pages)
- Theme assets are copied to `_assets/` in the output directory

Build statistics are reported on completion:
```
INFO Build complete markdown_files=42 copied_files=15 skipped_files=3 total_bytes=524288 elapsed=1.2s
```

### `init`

Scaffold a `.gomddoc/config.yml` with sensible defaults.

```bash
gomddoc init [DIR] [flags]
```

| Flag/Arg | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `DIR` (arg) | | `.` | Directory to initialize |
| `-t, --theme` | | `default` | Theme to use in generated config |

Creates `.gomddoc/config.yml` with the site title derived from the directory name.

### `mcp`

Start an MCP server for AI model integration (stdio transport). See [MCP Server](04-mcp.md) for
full documentation.

```bash
gomddoc mcp [DIR] [flags]
```

| Flag/Arg | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `DIR` (arg) | `GOMDDOC_SERVER_DIR` | `.` | Markdown directory or Git URL |
| `--git-key-file` | `GOMDDOC_SERVER_GIT_SSH_KEY` | | SSH key for private Git repos |
| `--git-storage-dir` | `GOMDDOC_SERVER_GIT_STORAGE_DIR` | | Disk-based Git clone directory |

### `info`

Show version, config file location, and all environment variables with their types and defaults.

```bash
gomddoc info
```

Run `gomddoc info` for a complete list of all `GOMDDOC_*` environment variables.

---

## Site Configuration (`SITE`)

These settings control how your documentation is presented and served. They are typically defined in a `.gomddoc/config.yml` file located at the root of your content directory.

**Example `.gomddoc/config.yml`:**

```yaml
default_index: "README.md"
dir_index: false
edit_url: "https://github.com/org/repo/edit/main"
language: "en-US"
exclude:
  - "drafts/"
  - "*.bak"

meta:
  title: "My Project Docs"
  description: "API Reference"
  domain: "docs.example.com"

theme:
  name: "default"
  vars:
    primary-color: "#2563eb"
  features:
    color_chips: true
    katex: false
    mermaid: false

highlighting:
  theme: "github"

search:
  index: true
```

### Content Behavior

**Default Index**
The filename to look for when a user requests a directory (e.g., `/api/`).
*   **YAML:** `default_index`
*   **Env Var:** `GOMDDOC_SITE_DEFAULT_INDEX`
*   **Default:** `README.md`
*   **Validation:** Must not be empty — startup fails with an error if set to `""`.

**Directory Listings**
Controls what happens when a directory is requested but no index file (e.g., `README.md`) exists:
*   `false` (default): Redirects (302) to the first page in the navigation tree. If no pages exist in the directory, returns 403 Forbidden.
*   `true`: Auto-generates a Markdown list of files in the directory.
*   **YAML:** `dir_index`
*   **Env Var:** `GOMDDOC_SITE_DIR_INDEX`
*   **Default:** `false`

**Language**
Sets the default language for the site using a BCP 47 code (e.g., `en-US`, `fr-FR`). This controls the `lang`
attribute on the `<html>` tag, determines which content is served at the root URL in multi-language sites, and sets the
fallback language for translations. Can be overridden per page via frontmatter `lang` field.

When BCP 47 directories (e.g., `fr-FR/`, `es-ES/`) exist in the content root, gomddoc automatically detects them and
creates per-language pipelines. See [Internationalization](13-internationalization.md) for details.
*   **YAML:** `language`
*   **Env Var:** `GOMDDOC_SITE_LANGUAGE`
*   **Default:** `"en-US"`

**Exclude Patterns**
A list of glob patterns for files and directories that should not be served, indexed, or included in navigation. Works like `.gitignore` — extends the built-in dot-file blocking (`.git/`, `.env`, `.gomddoc/`) with user-defined rules.

*   A pattern without `/` is matched against each path segment (e.g., `*.bak` matches `docs/notes.bak`)
*   A pattern with `/` is matched against the full path (e.g., `docs/internal/*.md`)
*   A trailing `/` matches directory prefixes only (e.g., `drafts/` blocks `drafts/secret.md` but not `docs/drafts.md`)
*   Patterns use Go's `path.Match` syntax (`*`, `?`, `[charclass]`)

```yaml
exclude:
  - "drafts/"
  - "*.bak"
  - "TODO.md"
  - "internal/"
```

*   **YAML:** `exclude`
*   **Default:** `[]` (empty — only dot files are blocked)

Excluded files return HTTP 404, are omitted from navigation, search, directory listings, and MCP tool access.
This holds for their clean URL too: with `strip_extensions` active, `TODO.md` blocks both `/TODO.md`
and `/TODO`, and neither appears in a static build's output.

### URL Extension Stripping

The `strip_extensions` option controls which file extensions are removed from URLs. Files matching these extensions that have an HTML renderer are served at extensionless canonical URLs.

```yaml
strip_extensions:
  - .md
  - .html
```

*   **YAML:** `strip_extensions`
*   **Default:** `[".md"]` — markdown files are served without extensions by default

**Behavior:**

*   `docs/guide.md` becomes accessible at `/docs/guide`
*   Requests to `/docs/guide.md` are 301-redirected to `/docs/guide`
*   Non-rendered files (images, CSS, etc.) keep their extensions
*   Set to `[]` to disable extension stripping

**Collisions:**

*   If both `guide.md` and `guide.html` exist and both extensions are stripped, the first extension in the list wins
*   If a file and directory share a name (e.g., `guide.md` and `guide/`), the file wins `/guide` and the directory is accessible via `/guide/`

**Build mode:**

When enabled, the build command generates pretty URLs: `guide.md` is output to `guide/index.html`. This works with any static host without server-side rewrite rules.

### Metadata (`SITE.META`)

**Title**
The name of your documentation site. It appears in browser tabs and the site header.
*   **YAML:** `meta.title`
*   **Env Var:** `GOMDDOC_SITE_META_TITLE`
*   **Default:** The capitalized name of your content directory.

**Description**
A short summary of your site, used for the HTML `<meta name="description">` SEO tag.
*   **YAML:** `meta.description`
*   **Env Var:** `GOMDDOC_SITE_META_DESCRIPTION`
*   **Default:** Empty.

**Domain**
The primary domain name for your site (e.g., `docs.example.com`). Used for canonical URL generation, XML sitemap, robots.txt `Sitemap:` directive, and Open Graph `og:url` tags. When set, every page gets a `<link rel="canonical">` tag and a `/sitemap.xml` endpoint becomes available. Must not include a scheme or path — just the hostname.
*   **YAML:** `meta.domain`
*   **Env Var:** `GOMDDOC_SITE_META_DOMAIN`
*   **Default:** Empty.

**Robots**
Controls the default `<meta name="robots">` tag for all pages. Common values: `"index, follow"` (default browser behavior), `"noindex"` (prevent indexing), `"noindex, nofollow"` (prevent indexing and link following). Can be overridden per page via frontmatter. Pages with `noindex` are automatically excluded from the sitemap.
*   **YAML:** `meta.robots`
*   **Env Var:** `GOMDDOC_SITE_META_ROBOTS`
*   **Default:** Empty (no tag emitted; crawlers default to `index, follow`).

### Edit URL

**Edit URL**
A base URL for "Edit this page" links that appear at the bottom of each page. When set, each rendered page will include a link pointing to the source file on your repository hosting platform (e.g., GitHub, GitLab). The page's file path is appended to this base URL.
*   **YAML:** `edit_url`
*   **Env Var:** `GOMDDOC_SITE_EDIT_URL`
*   **Default:** Empty (no edit link shown)
*   **Example:** `https://github.com/org/repo/edit/main`

When configured, a page at `/docs/guide.md` will produce an edit link pointing to `https://github.com/org/repo/edit/main/docs/guide.md`.

### Theme (`SITE.THEME`)

**Theme Name**
Selects the visual theme to apply. gomddoc ships with 8 built-in themes: `default`, `academic`, `gitbook`, `material`, `midnight`, `minimal`, `nord`, `ocean`.
*   **YAML:** `theme.name`
*   **Env Var:** `GOMDDOC_SITE_THEME_NAME`
*   **Default:** `default`

**Feature Toggles**

Feature toggles control which optional theme capabilities are enabled. All features default to **enabled** (`true`) unless explicitly disabled. Features can be toggled globally via config, per-environment via env vars, or per-page via frontmatter.

**YAML:** `theme.features`

```yaml
theme:
  name: default
  features:
    dark_mode: true
    toc: true
    code_copy: true
    color_chips: true
    search: true       # Auto-enabled when search index is available
    katex: false        # Disable KaTeX math rendering
    mermaid: false      # Disable Mermaid diagrams
    heading_anchors: true
```

**Env Vars:** `GOMDDOC_SITE_THEME_FEATURES_<NAME>=true|false`

Each feature can be toggled individually via environment variables:

```bash
GOMDDOC_SITE_THEME_FEATURES_KATEX=false gomddoc serve     # Disable KaTeX
GOMDDOC_SITE_THEME_FEATURES_MERMAID=false gomddoc serve    # Disable Mermaid
GOMDDOC_SITE_THEME_FEATURES_COLOR_CHIPS=false gomddoc serve # Disable color chips
```

**Available features:**

| Feature | Default | Controls |
|---------|---------|----------|
| `dark_mode` | `true` | Light/dark mode toggle button and script |
| `toc` | `true` | Table of contents sidebar |
| `code_copy` | `true` | Copy-to-clipboard button on code blocks |
| `color_chips` | `true` | Hex color code interactive swatches |
| `search` | `true` | Search button and modal (auto-enabled when search index is available) |
| `katex` | `true` | KaTeX math rendering (CSS and auto-render scripts) |
| `mermaid` | `true` | Mermaid diagram rendering |
| `heading_anchors` | `true` | Clickable anchor links on headings (goldmark extension) |
| `admonitions` | `true` | Admonition block rendering (goldmark extension) |

Features not listed in config default to enabled. Setting a feature to `false` prevents the corresponding template blocks and goldmark extensions from executing.

**Per-page overrides:** See [Frontmatter & Page-Level Overrides](#frontmatter--page-level-overrides) below.

### Theme Variables (`SITE.THEME.VARS`)

Theme variables let you customize a theme's visual appearance without creating a custom theme. They are exposed as CSS
custom properties (`--theme-{key}`) in a `:root` block, allowing themes to define configurable design tokens.

```yaml
theme:
  name: default
  vars:
    primary-color: "#2563eb"
    font-family: "Georgia, serif"
    sidebar-width: "280px"
```

This generates:

```css
:root {
  --theme-font-family: Georgia, serif;
  --theme-primary-color: #2563eb;
  --theme-sidebar-width: 280px;
}
```

*   **YAML:** `theme.vars`
*   **Env Var:** Not available (map type, YAML only)
*   **Default:** `{}` (empty)

**Validation:**
- Keys must contain only alphanumeric characters and hyphens
- Values must not contain braces (`{}`), semicolons (`;`), or angle brackets (`<>`)
- Invalid entries are logged and skipped (they don't fail startup)

Custom themes can reference these variables in their CSS. See [Theming & Assets](05-theming-and-assets.md) for the
`themeVarsCSS` template function.

### Search (`SITE.SEARCH`)

Controls full-text search index building. When enabled, gomddoc builds an inverted index at startup and exposes a `/api/search` endpoint. The search UI in the theme is controlled separately via `theme.features.search`.

*   **YAML:** `search.index`
*   **Env Var:** `GOMDDOC_SITE_SEARCH_INDEX`
*   **Default:** `true`

```yaml
search:
  index: false    # Disable search index building
```

### Syntax Highlighting (`SITE.HIGHLIGHTING`)

**Highlighting Theme**
Sets the Chroma syntax highlighting theme for code blocks. See [Chroma styles](https://xyproto.github.io/splash/docs/) for available themes.
*   **YAML:** `highlighting.theme`
*   **Env Var:** `GOMDDOC_SITE_HIGHLIGHTING_THEME`
*   **Default:** `github`

---

## Frontmatter & Page-Level Overrides

Individual markdown pages can override certain site-level settings through YAML frontmatter. When
a frontmatter field is present on a page, it takes precedence over the corresponding config file
or environment variable setting — but only for that page.

### Override Fields

| Frontmatter Field | Overrides | Effect |
|-------------------|-----------|--------|
| `title` | — | Sets the page title in `<title>`, Open Graph `og:title`, search results, and the tags API |
| `description` | `meta.description` | Page-level `<meta name="description">` and `og:description` (falls back to site description) |
| `og_type` | — | Controls `<meta property="og:type">` for this page (default: `article`) |
| `features` | `theme.features` | Per-page feature toggle overrides (see below) |
| `robots` | `meta.robots` | Controls `<meta name="robots">` for this page (e.g., `noindex`). Pages with `noindex` are excluded from the sitemap |
| `lang` | `language` | Sets the `<html lang="...">` attribute for this page, overriding the site-level language |
| `tags` | — | Page tags for the metadata index, queryable via `/api/tags` and the MCP `find_related` tool |
| `date` | — | Publication date included in tags API responses |
| `layout` | — | Selects an alternate template layout file (e.g., `layout: wide` uses `wide.html.tmpl`) |
| `redirect_from` | — | List of URL paths that should redirect (301) to this page. See [URL Redirects](#url-redirects) below |

### How Overrides Work

**Feature toggles** use a merge strategy: site-level features provide defaults, and per-page
frontmatter overrides specific features without affecting others. For example, if your site
enables `color_chips` globally, a page can disable it with:

```yaml
---
features:
  color_chips: false
---
```

The reverse also works — enable a feature on a single page while keeping it disabled site-wide:

```yaml
# .gomddoc/config.yml
theme:
  features:
    katex: false    # KaTeX disabled globally
```

```yaml
---
# page frontmatter
features:
  katex: true     # Enable KaTeX just for this page
---
```

Features are merged at page construction time: site defaults are cloned, then page overrides
are applied on top. Templates access the merged result via `{{ .Feature "name" }}`.

The `description` field falls back through two levels: page frontmatter first, then
`meta.description` from the config file. If neither is set, the description meta tag is omitted.
This means you can set a site-wide default description and override it on pages where a more
specific summary makes sense.

### Custom Frontmatter Fields

Any frontmatter field not in the table above is stored in the page's metadata map. These custom
fields are accessible in templates via `{{ index .Page.Meta "field_name" }}` and appear in tags
API responses. They do not affect rendering behavior but are useful for adding structured metadata
to pages.

See [Markdown Extensions](12-advanced/02-markdown-extensions.md) for the full frontmatter syntax and
field reference.

### URL Redirects

The `redirect_from` frontmatter field creates 301 redirects from old URLs to the current page. This is useful when
reorganizing documentation — old bookmarks and external links continue to work.

```yaml
---
title: "Setup Guide"
redirect_from:
  - /old/installation
  - /getting-started
---
```

With this frontmatter, requests to `/old/installation` and `/getting-started` are 301-redirected to the current page's
URL. In `build` mode, redirect HTML files containing `<meta http-equiv="refresh">` are generated at the old paths for
static hosts that don't support server-side redirects.

Redirects are checked early in request processing (before content is read) and use the path resolver for clean target
URLs, preventing double redirects.

---

## Network Tuning (`SERVER.HTTP`)

Advanced settings to tune the HTTP server timeouts and limits. These are only configurable via environment variables.

| Env Variable | Description | Default | Max |
| :--- | :--- | :--- | :--- |
| `GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT` | Graceful shutdown duration. | `1s` | `60s` |
| `GOMDDOC_SERVER_HTTP_READ_HEADER_TIMEOUT` | Max time to read request headers. | `5s` | `60s` |
| `GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT` | Max time to write response. | `30s` | `5m` |
| `GOMDDOC_SERVER_HTTP_IDLE_TIMEOUT` | Keep-alive connection idle time. | `120s` | `10m` |
| `GOMDDOC_SERVER_HTTP_MAX_HEADER_MB` | Max request header size (MB). | `1` | `10` |

---

## Priority Order

When a setting is defined in multiple places, gomddoc follows this strict priority order (highest wins):

1. **CLI flags and arguments** — always take precedence (e.g., `-p :3000`, `./my-docs`)
2. **Environment variables** — `GOMDDOC_*` prefixed (re-applied after config file to ensure env > file)
3. **Config file** — values from `.gomddoc/config.yml`
4. **Defaults** — hardcoded fallback values

The full loading sequence in `config.NewFromServeArgs()` is:
Defaults → CLI args → Dynamic defaults (e.g. title from dir name) → Config file → Env (re-apply) → Validate.

### Environment Variable Naming

Environment variables follow the struct nesting with underscores:

- `GOMDDOC_SERVER_PORT` — Maps to `Config.Server.Port`
- `GOMDDOC_SITE_DEFAULT_INDEX` — Maps to `Config.Site.DefaultIndex`
- `GOMDDOC_SITE_META_TITLE` — Maps to `Config.Site.Meta.Title`
- `GOMDDOC_SITE_FEATURES_KATEX` — Maps to `Config.Site.Features["katex"]`
