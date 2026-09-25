---
title: "Configuration Guide"
description: "Detailed guide on configuring gomddoc via CLI flags, environment variables, and config files."
author: "nicolasm"
---

# Configuration

gomddoc uses a layered configuration system. Settings come from four sources, applied in order of increasing priority:

1. **Defaults** — hardcoded sensible values
2. **Config file** — `.gomddoc/config.yml` in your content directory
3. **Environment variables** — `GOMDDOC_*` prefixed vars
4. **CLI flags/arguments** — always win

When the same setting is defined in multiple places, the highest-priority source wins. `site.*` settings can be set
in all four places; `server.*` settings (ports, timeouts, dev mode, pprof) are set by flag or environment variable only.

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
| `-p, --port` | `GOMDDOC_SERVER_PORT` | `:8080` | Listen address (`host:port` or `:port`; `:auto` picks the first free port from 8080) |
| `--admin-port` | `GOMDDOC_SERVER_ADMIN_PORT` | | Separate listen address for admin endpoints (health, metrics, pprof) |
| `-d, --domain` | `GOMDDOC_DOMAIN` | | Override `meta.domain` (canonical URLs, sitemap, feed, SEO tags) |
| `--git-key-file` | `GOMDDOC_SERVER_GIT_SSH_KEY` | | SSH key for private Git repos |
| `--git-storage-dir` | `GOMDDOC_SERVER_GIT_STORAGE_DIR` | | Disk-based Git clone directory (default: in-memory) |
| `--pprof` | `GOMDDOC_SERVER_PPROF` | `false` | Enable `/debug/pprof/` profiling endpoints |
| `--basic-auth-file` | `GOMDDOC_SERVER_BASIC_AUTH_FILE` | | Path to htpasswd file (bcrypt only) |

With `--admin-port` empty or equal to `--port`, the health, metrics and pprof endpoints are served on the main
listener and `serve` logs a warning. A host-less value such as `:9090` binds `127.0.0.1:9090`; give an explicit host
(`0.0.0.0:9090`) to listen on every interface.

### `preview`

Quick preview with dev mode enabled by default.

```bash
gomddoc preview [DIR] [flags]
```

| Flag/Arg | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `DIR` (arg) | `GOMDDOC_SERVER_DIR` | `.` | Content directory or Git URL |
| `-p, --port` | `GOMDDOC_SERVER_PORT` | `:auto` | Listen address (auto-assigns from 8080) |
| `-d, --domain` | `GOMDDOC_DOMAIN` | | Override `meta.domain` (canonical URLs, sitemap, feed, SEO tags) |
| `--git-key-file` | `GOMDDOC_SERVER_GIT_SSH_KEY` | | SSH key for private Git repos |
| `--git-storage-dir` | `GOMDDOC_SERVER_GIT_STORAGE_DIR` | | Disk-based Git clone directory (default: in-memory) |
| `--open` | `GOMDDOC_PREVIEW_OPEN` | `false` | Auto-open browser on startup |
| `--dir-index` | `GOMDDOC_DIR_INDEX` | `false` | Enable directory listings when no index file exists (sets `dir_index`) |

Preview always runs in dev mode (the template cache is off, so templates are re-parsed on every request) and defaults
to automatic port assignment. It has no `--admin-port` or `--pprof`: health and metrics are served on the main listener.

### `build`

Generate a static site from markdown content.

```bash
gomddoc build [DIR] [flags]
```

| Flag/Arg | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `DIR` (arg) | `GOMDDOC_SERVER_DIR` | `.` | Markdown source directory or Git URL |
| `-o, --output` | `GOMDDOC_BUILD_OUTPUT` | `build/site` | Output directory |
| `-d, --domain` | `GOMDDOC_DOMAIN` | | Override `meta.domain` (canonical URLs, sitemap, feed, SEO tags) |
| `--git-key-file` | `GOMDDOC_SERVER_GIT_SSH_KEY` | | SSH key for private Git repos |
| `--git-storage-dir` | `GOMDDOC_SERVER_GIT_STORAGE_DIR` | | Disk-based Git clone directory (default: in-memory) |

**Build behavior:**
- A `.gomddoc-build` sentinel file is written to the output directory. On subsequent builds, the sentinel proves the
  directory was created by gomddoc: the directory is deleted and rebuilt. Non-empty directories without the sentinel
  are refused.
- Markdown files are rendered to HTML through the full template pipeline
- The output layout follows `strip_extensions`. With it set (the default, `[".md"]`), `guide.md`
  becomes `guide/index.html` so the URL is `/guide`; with it empty, `guide.md` becomes `guide.html`.
  In both modes the default index (`README.md`) becomes `index.html` in its own directory — never
  `README.html` — unless that directory also has an `index.md`, which wins.
- In pretty mode, each source path is also written as a redirect stub, so `/guides/setup.md` still
  resolves on a static host: the stub lives at `guides/setup.md` and points at `/guides/setup`.
  Default-index files get no stub, because their URL is the directory itself.
- Pages with `redirect_from` frontmatter get a stub per source, at `<source>/index.html` (or at `<source>` itself
  when its last segment contains a dot)
- Non-markdown files (images, CSS, JS) are copied as-is
- Hidden files (starting with `.`) and `exclude` matches are skipped
- `robots.txt` is always generated
- `sitemap.xml` and `feed.xml` are generated when `meta.domain` is configured. With language directories, each
  language also gets `<lang>/sitemap.xml` and `<lang>/feed.xml`, and a `sitemap-index.xml` lists them
- Tag pages are written to `tags/index.html` and `tags/<tag>/index.html`
- `404.html` is generated for static host compatibility (Netlify, GitHub Pages, Cloudflare Pages), plus
  `<lang>/404.html` per language
- Theme assets are copied to `_assets/` in the output directory
- No search index is built: the theme's search modal queries `/api/search`, which only `serve` and `preview` provide

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

Creates `.gomddoc/config.yml` with `default_index`, `dir_index`, `meta.title` (derived from the directory name),
`meta.description`, `theme.name` and `highlighting.theme`. It fails if the file already exists. The theme name is not
checked: a theme other than `default` must be installed under `.gomddoc/assets/themes/<name>/`.

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
| `--git-storage-dir` | `GOMDDOC_SERVER_GIT_STORAGE_DIR` | | Disk-based Git clone directory (default: in-memory) |

### `info`

Describe gomddoc and, when run in a site, that site's configuration: config file location and whether it loaded (and
why not), the precedence rule, every setting with its config key, environment variable, flags, default and
description, the commands, the active theme's feature toggles and CSS variables, and the embedded guide's pages.

```bash
gomddoc info [DIR] [--json]
```

| Flag/Arg | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `DIR` (arg) | `GOMDDOC_SERVER_DIR` | `.` | Content directory to inspect. Git URLs are not cloned; defaults are shown |
| `--json` | | `false` | Print the capabilities report as JSON — the same document as the MCP resource `gomddoc://capabilities` |

`info` never fails on a broken site: a config that does not load is reported with the loader's reason and exit
status 0, so it is the command to run right after editing `.gomddoc/config.yml`.

### `schema`

Print the JSON Schema (draft 2020-12) for `.gomddoc/config.yml` — the same document as the MCP resource
`gomddoc://schema/config`. Point an editor's YAML language server at it for validation and completion.

```bash
gomddoc schema > .gomddoc/config.schema.json
```

### `help`

Read this guide in the terminal: list its topics, read one (or one section of it), or search it. Output to a terminal
goes through `$PAGER` (default `less -FRX`); piped output is plain markdown. Every command's `--help` ends with the
topic that covers it.

| Flag/Arg | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `TOPIC` (arg) | | | Guide topic; omit to list topics |
| `SECTION` (arg) | | | Heading anchor within the topic, e.g. `priority-order` |
| `-s, --search` | | | Search the guide |

```bash
gomddoc help                              # list topics
gomddoc help configuration                # read a topic
gomddoc help configuration priority-order # read one section
gomddoc help --search "exclude pattern"   # search
```

Topic names are the guide's file names without number or extension (`02-configuration.md` is `configuration`). The same
pages are served over MCP by `gomddoc_guide` and `gomddoc://guide/<path>`.

### `doctor`

Check the site's configuration and content and report every problem — unknown keys, invalid values, unknown env vars,
theme toggles, frontmatter types, redirect conflicts — each with its file, line and fix. Exits 1 on errors (and on
warnings with `--strict`).

```bash
gomddoc doctor [DIR] [--json] [--strict] [-v]
```

| Flag/Arg | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `DIR` (arg) | `GOMDDOC_SERVER_DIR` | `.` | Markdown directory or Git URL to check |
| `--json` | | `false` | Print the report as JSON (the same document as the MCP tool `gomddoc_doctor`) |
| `--strict` | | `false` | Exit non-zero on warnings as well as errors |
| `-v, --verbose` | | `false` | Include info findings (missing descriptions, unused theme vars) |
| `--git-key-file` | `GOMDDOC_SERVER_GIT_SSH_KEY` | | SSH key for private Git repos |
| `--git-storage-dir` | `GOMDDOC_SERVER_GIT_STORAGE_DIR` | | Disk-based Git clone directory (default: in-memory) |

See [Doctor](14-doctor.md) for the flags, the JSON report and every finding code.

---

## Site Configuration (`SITE`)

These settings control how your documentation is presented and served. They are typically defined in a
`.gomddoc/config.yml` file located at the root of your content directory. The file holds the `site.*` settings with
the `site.` prefix dropped: `site.theme.name` is written `theme: {name: …}`, with no `site:` wrapper.

The loader is strict:

- An unknown key is a startup error, not a silent no-op. This includes a `site:` wrapper and a feature toggle placed
  outside `theme.features`.
- Only the first YAML document is read. A `---` separator followed by more content is an error.
- An empty or comment-only file means "use the defaults".
- The file is read from a local content directory only. With a Git URL as `DIR`, a `.gomddoc/config.yml` inside the
  repository is not loaded; set site values through environment variables (`GOMDDOC_SITE_META_TITLE` and the others
  listed by `gomddoc info`) instead.

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
    primary: "#2563eb"
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
*   `false` (default): Redirects (302) to the first page in the navigation tree. If no pages exist in the directory,
    returns 403 Forbidden.
*   `true`: Auto-generates a Markdown list of files in the directory.
*   The `preview` command's `--dir-index` flag (`GOMDDOC_DIR_INDEX`) turns it on; no flag turns it off once the file or
    an environment variable has set it to `true`.
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
A list of glob patterns for files and directories that should not be served, indexed, or included in navigation. Works
like `.gitignore`: it extends the built-in dot-file blocking (`.git/`, `.env`, `.gomddoc/`; `.well-known/` is the one
dot-directory that is served) with user-defined rules.

*   A pattern without `/` is matched against each path segment (e.g., `*.bak` matches `docs/notes.bak`)
*   A pattern with `/` is matched against the full path (e.g., `docs/internal/*.md`)
*   A trailing `/` matches directory prefixes only (e.g., `drafts/` blocks `drafts/secret.md` but not `docs/drafts.md`).
    The directory part is itself a glob: `drafts/*/` blocks every entry directly under `drafts/`, files included
*   Patterns use Go's `path.Match` syntax (`*`, `?`, `[charclass]`)

```yaml
exclude:
  - "drafts/"
  - "*.bak"
  - "TODO.md"
  - "internal/"
```

*   **YAML:** `exclude`
*   **Env Var:** Not available (list type, YAML only)
*   **Default:** `[]` (empty — only dot files are blocked)

Excluded files return HTTP 404 and are omitted from navigation, search, tags, sitemap, feed, directory listings and
MCP tool access.
This holds for their clean URL too: with `strip_extensions` active, `TODO.md` blocks both `/TODO.md`
and `/TODO`, and neither appears in a static build's output.

### URL Extension Stripping

The `strip_extensions` option controls which file extensions are removed from URLs. Files matching these extensions that
have an HTML renderer are served at extensionless canonical URLs.

```yaml
strip_extensions:
  - .md
  - .html
```

*   **YAML:** `strip_extensions`
*   **Env Var:** Not available (list type, YAML only)
*   **Default:** `[".md"]` — markdown files are served without extensions by default
*   **Validation:** Each entry must start with a dot.

**Behavior:**

*   `docs/guide.md` becomes accessible at `/docs/guide`
*   Requests to `/docs/guide.md` are 301-redirected to `/docs/guide`
*   Non-rendered files (images, CSS, etc.) keep their extensions
*   Set to `[]` to disable extension stripping

**Collisions:**

*   If both `guide.md` and `guide.html` exist and both extensions are stripped, the first extension in the list wins;
    `gomddoc doctor` reports the collision as `content.path-collision`
*   If a file and directory share a name (e.g., `guide.md` and `guide/`), the file wins `/guide` and the directory is
    accessible via `/guide/`

**Build mode:**

When enabled, the build command generates pretty URLs: `guide.md` is output to `guide/index.html`. This works with any
static host without server-side rewrite rules.

### Metadata (`SITE.META`)

**Title**
The name of your documentation site. It appears in browser tabs, the site header, Open Graph tags and feeds.
*   **YAML:** `meta.title`
*   **Env Var:** `GOMDDOC_SITE_META_TITLE`
*   **Default:** The content directory's name, title-cased. For a Git URL the name is derived from the URL's last path
    segment, so set the title explicitly.

**Description**
A short summary of your site, used for the HTML `<meta name="description">` SEO tag.
*   **YAML:** `meta.description`
*   **Env Var:** `GOMDDOC_SITE_META_DESCRIPTION`
*   **Default:** Empty.

**Domain**
The primary domain name for your site (e.g., `docs.example.com`). Used for canonical URL generation, XML sitemap, Atom
feed, robots.txt `Sitemap:` directive, and Open Graph `og:url` tags. When set, every page gets a
`<link rel="canonical">` tag and the `/sitemap.xml` and `/feed.xml` endpoints become available. Must not include a
scheme or path: the hostname only.
*   **YAML:** `meta.domain`
*   **Env Var:** `GOMDDOC_SITE_META_DOMAIN`
*   **Flag:** `-d, --domain` on `serve`, `preview` and `build` (env `GOMDDOC_DOMAIN`), which overrides both the file and
    `GOMDDOC_SITE_META_DOMAIN`
*   **Default:** Empty.

**Robots**
Controls the default `<meta name="robots">` tag for all pages. Common values: `"index, follow"` (default browser
behavior), `"noindex"` (prevent indexing), `"noindex, nofollow"` (prevent indexing and link following). Can be
overridden per page via frontmatter. Pages whose frontmatter `robots` contains `noindex` are excluded from the sitemap
and feed.
*   **YAML:** `meta.robots`
*   **Env Var:** `GOMDDOC_SITE_META_ROBOTS`
*   **Default:** Empty (no tag emitted; crawlers default to `index, follow`).

### Edit URL

**Edit URL**
A base URL for "Edit this page" links that appear at the bottom of each page. When set, each rendered page will include
a link pointing to the source file on your repository hosting platform (e.g., GitHub, GitLab). The page's file path is
appended to this base URL.
*   **YAML:** `edit_url`
*   **Env Var:** `GOMDDOC_SITE_EDIT_URL`
*   **Default:** Empty (no edit link shown)
*   **Validation:** A scheme other than `http` or `https` is a startup error.
*   **Example:** `https://github.com/org/repo/edit/main`

When configured, a page at `/docs/guide.md` will produce an edit link pointing to
`https://github.com/org/repo/edit/main/docs/guide.md`.

### Theme (`SITE.THEME`)

**Theme Name**
Selects the visual theme to apply. The binary bundles one theme, `default`. Seven more — `academic`, `gitbook`,
`material`, `midnight`, `minimal`, `nord`, `ocean` — are distributed separately and must be copied into
`.gomddoc/assets/themes/<name>/` before they can be selected; naming one that is not installed falls back to `default`.
See [Theming & Assets](05-theming-and-assets.md#themes).
*   **YAML:** `theme.name`
*   **Env Var:** `GOMDDOC_SITE_THEME_NAME`
*   **Default:** `default`

**Feature Toggles**

Feature toggles control which optional theme capabilities are enabled. All features default to **enabled** (`true`)
unless explicitly disabled. Features can be toggled globally via config, per-environment via env vars, or per-page via
frontmatter.

**YAML:** `theme.features`

```yaml
theme:
  name: default
  features:
    dark_mode: true
    toc: true
    code_copy: true
    color_chips: true
    search: true        # Set false when search.index is false
    katex: false        # Disable KaTeX math rendering
    mermaid: false      # Disable Mermaid diagrams
    heading_anchors: true
```

**Env Vars:** `GOMDDOC_SITE_THEME_FEATURES_<NAME>=true|false` (the name is lowercased to form the key)

Keys must match `^[a-z][a-z0-9_]*$`; any other key is a startup error.

Each feature can be toggled individually via environment variables:

```bash
GOMDDOC_SITE_THEME_FEATURES_KATEX=false gomddoc serve     # Disable KaTeX
GOMDDOC_SITE_THEME_FEATURES_MERMAID=false gomddoc serve    # Disable Mermaid
GOMDDOC_SITE_THEME_FEATURES_COLOR_CHIPS=false gomddoc serve # Disable color chips
```

**Features read by the `default` theme:**

| Feature | Default | Controls |
|---------|---------|----------|
| `dark_mode` | `true` | Light/dark mode toggle button and script |
| `toc` | `true` | Table of contents sidebar |
| `code_copy` | `true` | Copy-to-clipboard button on code blocks |
| `color_chips` | `true` | Hex color code interactive swatches |
| `search` | `true` | Search button and modal. Independent of `search.index`: with the index off, the modal has no `/api/search` to query |
| `katex` | `true` | KaTeX math rendering (CSS and auto-render scripts) |
| `mermaid` | `true` | Mermaid diagram rendering |
| `heading_anchors` | `true` | Clickable anchor links on headings (goldmark extension) |
| `admonitions` | `true` | Admonition block rendering (goldmark extension) |
| `see_also` | `true` | "See also" list of related pages at the end of a page |
| `tag_chips` | `true` | Frontmatter tags rendered as chips linking to tag pages |

Other themes can read other keys. `gomddoc info` lists the keys the active theme reads.

Features not listed in config default to enabled. Setting a feature to `false` prevents the corresponding template
blocks and goldmark extensions from executing.

**Per-page overrides:** See [Frontmatter & Page-Level Overrides](#frontmatter--page-level-overrides) below.

### Theme Variables (`SITE.THEME.VARS`)

Theme variables let you customize a theme's visual appearance without creating a custom theme. They are exposed as CSS
custom properties (`--theme-{key}`) in a `:root` block, allowing themes to define configurable design tokens.

```yaml
theme:
  name: default
  vars:
    primary: "#2563eb"
    dark-primary: "#60a5fa"
```

This generates:

```css
:root {
  --theme-dark-primary: #60a5fa;
  --theme-primary: #2563eb;
}
```

Which keys have an effect depends on the theme. The `default` theme reads `bg`, `dark-bg`, `dark-primary`,
`dark-text`, `primary` and `text`; `gomddoc info` lists the keys the active theme reads.

*   **YAML:** `theme.vars`
*   **Env Var:** Not available (map type, YAML only)
*   **Default:** `{}` (empty)

**Validation:**
- Keys must contain only alphanumeric characters and hyphens
- Values must not contain braces (`{}`), semicolons (`;`), or angle brackets (`<>`)
- Invalid entries are logged as warnings and skipped (they don't fail startup)

Custom themes can reference these variables in their CSS. See [Theming & Assets](05-theming-and-assets.md) for the
`themeVarsCSS` template function.

### Search (`SITE.SEARCH`)

Controls full-text search index building. When enabled, `serve`, `preview` and `mcp` build an inverted index at startup;
`serve` and `preview` expose it at `/api/search` and `mcp` through `search_docs`. When disabled, `/api/search` is not
registered and `search_docs` reports that no index is available. The search UI in the theme is controlled separately
via `theme.features.search`.

*   **YAML:** `search.index`
*   **Env Var:** `GOMDDOC_SITE_SEARCH_INDEX`
*   **Default:** `true`

```yaml
search:
  index: false    # Disable search index building
```

### Syntax Highlighting (`SITE.HIGHLIGHTING`)

**Highlighting Theme**
Sets the Chroma syntax highlighting theme for code blocks. See [Chroma styles](https://xyproto.github.io/splash/docs/)
for available themes.
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
| `title` | — | Sets the page title in `<title>`, Open Graph `og:title`, search results, navigation and the tags API |
| `description` | `meta.description` | Page-level `<meta name="description">` and `og:description` (falls back to site description) |
| `og_type` | — | Controls `<meta property="og:type">` for this page (default: `article`) |
| `features` | `theme.features` | Per-page feature toggle overrides (see below) |
| `robots` | `meta.robots` | Controls `<meta name="robots">` for this page (e.g., `noindex`). Pages with `noindex` are excluded from the sitemap and feed |
| `lang` | `language` | Sets the `<html lang="...">` attribute for this page, overriding the site-level language |
| `tags` | — | Page tags (lowercased, trimmed): tag pages, `/api/tags`, related pages, `tag:` search filters and the MCP `find_related` tool |
| `date` | — | Publication date (`YYYY-MM-DD` or RFC 3339): feed entries, JSON-LD `datePublished`, sitemap fallback, tags API |
| `author` | — | Author name in the page's JSON-LD structured data |
| `layout` | — | Selects an alternate template layout file (e.g., `layout: wide` uses `wide.html.tmpl`); falls back to `default.html.tmpl` when the theme has no such file |
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
  katex: true     # Enable KaTeX for this page only
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
| `GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT` | Grace period for in-flight requests on SIGINT/SIGTERM. | `1s` | none (warns above `60s`) |
| `GOMDDOC_SERVER_HTTP_READ_HEADER_TIMEOUT` | Max time to read request headers. | `5s` | `1m` |
| `GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT` | Max time to write response. | `30s` | `5m` |
| `GOMDDOC_SERVER_HTTP_IDLE_TIMEOUT` | Keep-alive connection idle time. | `2m` | `10m` |
| `GOMDDOC_SERVER_HTTP_MAX_HEADER_MB` | Max request header size (MiB). | `1` | `10` |

Durations use Go syntax (`500ms`, `45s`, `2m`). A value at or below zero, or above the max, on the four rows with a max
is not an error: it logs a warning and falls back to the default. `SHUTDOWN_TIMEOUT` is checked instead by validation:
a negative value is a startup error, and a value above `60s` is kept with a warning.

A value that does not parse (`GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT=thirty`) is ignored with a warning, for every
`GOMDDOC_*` variable. `gomddoc doctor` reports these as `env.invalid-value`, and reports `GOMDDOC_*` variables gomddoc
does not read.

---

## Development Mode (`GOMDDOC_SERVER_DEV_MODE`)

`GOMDDOC_SERVER_DEV_MODE=true` disables the template cache, so templates are re-parsed on every request and theme
edits show on reload (default `false`). It also silences the warning about admin endpoints on the main port. Also
environment-only — no command exposes a `--dev` flag. `gomddoc preview` turns dev mode on unconditionally, so this
variable is only useful with `gomddoc serve`.

---

## Priority Order

When a setting is defined in multiple places, gomddoc follows this strict priority order (highest wins):

1. **CLI flags and arguments** — always take precedence (e.g., `-p :3000`, `./my-docs`)
2. **Environment variables** — `GOMDDOC_*` prefixed (re-applied after config file to ensure env > file)
3. **Config file** — values from `.gomddoc/config.yml`
4. **Defaults** — hardcoded fallback values

The full loading sequence in `config.NewFromServeArgs()` is:
Defaults → Env → CLI args → Dynamic defaults (e.g. title from dir name) → Config file → Env (re-apply) →
`--dir-index` → Normalize → Validate. The `--domain` flag is applied after that, and validated again.

Two boolean settings are OR'd rather than overridden: `dir_index` (preview's `--dir-index`) and dev mode. A flag can
turn them on, but cannot turn off a `true` that the file or an environment variable set.

The first Env pass exists only for settings no flag owns (`SERVER_HTTP_*`, `SERVER_DEV_MODE`). Flags
stay on top because the CLI parser has already folded `GOMDDOC_*` into every flag it defines — which is
also why that pass runs *before* the args rather than after.

### Machine-Readable Configuration

Everything in this page is also available from the binary itself, generated from the same source (the config
structs' tags), so it cannot fall out of date with the version you run:

- `gomddoc schema` — JSON Schema for `.gomddoc/config.yml`
- `gomddoc doctor --json` — every problem in the current configuration and content, with fixes
- `gomddoc info --json` — every setting with its config key, env var, flags and default, plus commands and theme
  features
- Over MCP (`gomddoc mcp`): `gomddoc://schema/config`, `gomddoc://capabilities` and the `gomddoc_guide` tool — see
  [MCP Server](04-mcp.md#self-documentation)

### Environment Variable Naming

Environment variables follow the struct nesting with underscores:

- `GOMDDOC_SERVER_PORT` — Maps to `Config.Server.Port`
- `GOMDDOC_SITE_DEFAULT_INDEX` — Maps to `Config.Site.DefaultIndex`
- `GOMDDOC_SITE_META_TITLE` — Maps to `Config.Site.Meta.Title`
- `GOMDDOC_SITE_THEME_FEATURES_KATEX` — Maps to `Config.Site.Theme.Features["katex"]`
