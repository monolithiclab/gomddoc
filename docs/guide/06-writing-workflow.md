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

Preview enables dev mode automatically and is the recommended command during active writing —
it prioritizes convenience over production settings. `serve` always runs in production mode.

## Development Mode

Development mode (enabled by `preview`) disables caching and enables verbose logging, so every
browser refresh shows the latest version of your content.

### What Changes in Dev Mode

**Template reloading.** In production, parsed templates are cached in memory for performance.
In dev mode, templates are re-parsed on every request. Any change to a `.tmpl` file — layout,
partial, or theme override in `.gomddoc/assets/` — is visible immediately upon refresh.

**Config logging.** The full resolved configuration is logged at startup, which helps verify
that your `.gomddoc/config.yml` changes are being picked up:

```
INFO Development mode enabled config={...}
```

**Debug logging.** You will see verbose `slog.Debug` output including file lookups in overlay
filesystem layers, hidden path blocking events, environment variable overrides, and template
cache operations.

**No effective caching.** HTTP caching headers are still sent but have no practical effect since
content is always re-rendered from source. Every page refresh shows the latest content, template
changes take effect immediately, and configuration changes to `.gomddoc/config.yml` are picked up
on the next request.

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

The assigned port is logged at startup:

```
INFO Auto-assigned port port=:8081
INFO Server started url=http://localhost:8081
```

## Recommended Workflow

1. Run `gomddoc preview --open` in your docs directory
2. Edit markdown files, templates, or config in your editor
3. Refresh the browser to see changes instantly
4. Use `Ctrl+K` to test the search modal with your content
5. When satisfied, build the static site with `gomddoc build`

## YAML Frontmatter

Every markdown file can include optional YAML frontmatter at the top, enclosed by `---` delimiters.
Frontmatter fields affect how the page is rendered, indexed, and displayed in search results. See
[Markdown Extensions](12-advanced/02-markdown-extensions.md) for the full frontmatter field reference and
[Configuration](02-configuration.md) for how frontmatter interacts with site-level settings.

## Tags and Discovery

Adding a `tags` list to a page's frontmatter drives three discovery features automatically — no extra
configuration beyond the relevant theme feature flags:

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
- **See-also (related pages)** — the `see_also` theme feature appends a "See also" section listing other
  pages that share one or more tags with the current page, ranked by overlap.

Tags are normalized (lowercased, trimmed, deduplicated) for indexing, and they are also queryable via
the `tag:` syntax in [full-text search](10-search.md#tag-filters) and the JSON `/api/tags` endpoints.
Both `tag_chips` and `see_also` are enabled by default in the built-in themes and can be toggled per
site or per page via [feature flags](02-configuration.md).
