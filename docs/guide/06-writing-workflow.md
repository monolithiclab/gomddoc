---
title: "Writing Workflow"
description: "How to author and preview documentation with gomddoc's development mode."
author: "nicolasm"
---

# Writing Workflow

gomddoc is designed for a fast authoring loop: write markdown, refresh your browser, and see the
result immediately. This page covers the tools and modes that make content creation efficient.

## Quick Preview

The fastest way to start writing is the `preview` command. It enables development mode
automatically, auto-assigns a port, and can open your browser:

```bash
gomddoc preview --open
```

`preview` takes the same directory argument as `serve` (a local directory or a Git URL) and these flags:
`-p/--port` (default `:auto`), `-d/--domain`, `--open` (`GOMDDOC_PREVIEW_OPEN`), `--dir-index`, `--git-key-file`
and `--git-storage-dir`. It has no `--admin-port`, `--pprof` or `--basic-auth-file`; health and metrics stay on the
main port.

Preview is the recommended command during active writing. `serve` runs in production mode unless
`GOMDDOC_SERVER_DEV_MODE=true` is set.

## Development Mode

Development mode (always on for `preview`, opt-in for `serve` with `GOMDDOC_SERVER_DEV_MODE=true`) disables the
template and inline-asset caches, so a browser refresh shows your latest template edits.

### What Changes in Dev Mode

**Template reloading.** In production, parsed templates and the assets inlined with `inlineJSAsset`,
`inlineCSSAsset` and `inlineHTMLAsset` are cached in memory. In dev mode, they are re-read on every request. Any
change to a `.tmpl` file (layout, partial, or theme override in `.gomddoc/`) or to an inlined script is visible
on refresh.

**Startup logging.** Dev mode logs the directory, port and theme at startup, and the admin-port warning that
`serve` prints is suppressed:

```
INFO Development mode enabled dir=docs port=:8080 theme=default
```

**Content edits.** Markdown is rendered from source on every request in both modes, so an edit to an existing
page's body shows on refresh.

**What needs a restart.** The configuration, navigation tree, metadata and tag index, search index and clean-URL
map are built once at startup, in dev mode too. Restart `preview` after you change `.gomddoc/config.yml`, add,
rename or delete a file, or change a page's title or tags, so navigation, search, tag pages and `strip_extensions`
URLs pick it up. For Git sources the repository is cloned at startup, so new commits also need a restart.

HTTP caching headers (`ETag`, `Cache-Control: public, max-age=300` on pages) are sent in both modes. Use a hard
refresh if the browser serves a cached page.

## Auto-Port Assignment

Both `preview` (by default) and `serve` (with `-p :auto`) support automatic port assignment. The
server scans for the first available port starting from 8080, which is useful when running multiple
instances or when port 8080 is already in use:

```bash
# Preview always auto-assigns
gomddoc preview

# Serve with explicit auto-assign
gomddoc serve -p :auto
```

The assigned port is logged at startup, and `preview` also prints the URL:

```
INFO Auto-assigned port port=:8081
Preview: http://localhost:8081
INFO Server started url=http://localhost:8081 dir=. dev=true
```

## Recommended Workflow

1. Run `gomddoc preview --open` in your docs directory
2. Edit markdown files and templates in your editor
3. Refresh the browser to see changes; restart after config changes or new files
4. Use `Ctrl+K` (`Cmd+K` on macOS) to test the search modal with your content
5. Run `gomddoc doctor` to check configuration and content
6. When satisfied, build the static site with `gomddoc build`

## YAML Frontmatter

Every markdown file can include optional YAML frontmatter at the top, enclosed by `---` delimiters.
Frontmatter fields affect how the page is rendered, indexed, and displayed in search results. See
[Markdown Extensions](12-advanced/02-markdown-extensions.md) for the full frontmatter field reference and
[Configuration](02-configuration.md) for how frontmatter interacts with site-level settings.

## Tags and Discovery

Adding a `tags` list to a page's frontmatter drives three discovery features. They need no configuration beyond
the relevant theme feature flags:

```yaml
---
title: Deploying with Docker
tags: [deployment, docker]
---
```

- **Tag chips** — the `tag_chips` theme feature renders the page's tags as clickable chips near the top
  of the page, each linking to that tag's landing page.
- **Tag pages** — gomddoc serves an HTML index at `/tags/` listing every tag, and a landing page at
  `/tags/{tag}` listing all pages carrying that tag. These are emitted as static HTML in `build` mode
  too. (See the [API Reference](12-advanced/03-api-reference.md) for the tag endpoints.)
- **See-also (related pages)** — the `see_also` theme feature appends a "See also" section listing up to 10 other
  pages that share one or more tags with the current page, sorted by title.

Tags are normalized (lowercased, trimmed, deduplicated; tags containing `/` or `\` are dropped) for indexing, and
they are also queryable via the `tag:` syntax in [full-text search](10-search.md#tag-filters) and the JSON
`/api/tags` endpoints. Both `tag_chips` and `see_also` are enabled by default and can be toggled per site or per
page via [feature flags](02-configuration.md).
