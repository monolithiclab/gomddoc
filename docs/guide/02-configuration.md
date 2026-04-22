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
| `--basic-auth-file` | `GOMDDOC_SERVER_BASIC_AUTH_FILE` | | Path to htpasswd file (bcrypt only) |

### `preview`

Quick preview with dev mode enabled by default.

```bash
gomddoc preview [DIR] [flags]
```

| Flag/Arg | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `DIR` (arg) | `GOMDDOC_SERVER_DIR` | `.` | Content directory |
| `-p, --port` | `GOMDDOC_SERVER_PORT` | `:auto` | Listen address (auto-assigns from 8080) |
| `-d, --domain` | `GOMDDOC_DOMAIN` | | Override site domain for SEO (canonical, sitemap, etc.) |
| `--open` | `GOMDDOC_PREVIEW_OPEN` | `false` | Auto-open browser on startup |

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
| `-f, --force` | `GOMDDOC_BUILD_FORCE` | `false` | Overwrite output directory if it exists |

**Build behavior:**
- The build fails if the output directory already exists (prevents stale content). Use `--force` to remove it and rebuild cleanly.
- Markdown files are rendered to HTML through the full template pipeline
- `README.md` files generate both `README.html` and `index.html` for clean URLs (unless `index.md` exists in the same directory)
- Non-markdown files (images, CSS, JS) are copied as-is
- Hidden files (starting with `.`) are skipped
- `robots.txt` is always generated
- `sitemap.xml` is generated when `meta.domain` is configured
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
color_chips: true

meta:
  title: "My Project Docs"
  description: "API Reference"
  domain: "docs.example.com"

theme:
  name: "default"

highlighting:
  theme: "github"
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
Sets the `lang` attribute on the `<html>` tag (e.g., `<html lang="en">`). Lighthouse flags missing or incorrect language attributes. Can be overridden per page via frontmatter `lang` field.
*   **YAML:** `language`
*   **Env Var:** `GOMDDOC_SITE_LANGUAGE`
*   **Default:** `"en"`

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

### Rendering Options

**Color Chips**
Controls whether hex color codes in backticks (e.g., `` `#ff5733` ``) are rendered as interactive color swatches. Can be overridden per-page via frontmatter.
*   **YAML:** `color_chips`
*   **Env Var:** `GOMDDOC_SITE_COLOR_CHIPS`
*   **Default:** `true`

### Theme (`SITE.THEME`)

**Theme Name**
Selects the visual theme to apply. gomddoc ships with 8 built-in themes: `default`, `academic`, `gitbook`, `material`, `midnight`, `minimal`, `nord`, `ocean`.
*   **YAML:** `theme.name`
*   **Env Var:** `GOMDDOC_SITE_THEME_NAME`
*   **Default:** `default`

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
| `color_chips` | `color_chips` | Enables or disables color chip rendering on this page, overriding the global setting |
| `robots` | `meta.robots` | Controls `<meta name="robots">` for this page (e.g., `noindex`). Pages with `noindex` are excluded from the sitemap |
| `lang` | `language` | Sets the `<html lang="...">` attribute for this page, overriding the site-level language |
| `tags` | — | Page tags for the metadata index, queryable via `/api/tags` and the MCP `find_related` tool |
| `date` | — | Publication date included in tags API responses |

### How Overrides Work

The `color_chips` field is the clearest example of the override pattern. If your site config sets
`color_chips: false` to disable color chips globally, a page with `color_chips: true` in its
frontmatter will still render color chips. The reverse also works — disable chips on a single
technical page while keeping them on everywhere else.

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
