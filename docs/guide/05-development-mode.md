---
title: "Development Mode Guide"
description: "Instructions for using gomddoc's development mode with hot reload."
author: "nicolasm"
---

# Development Mode

For rapid iteration, gomddoc includes a development mode that disables caching and enables verbose logging.

## Enabling Dev Mode

### Using the `serve` command

Use the `--dev` flag:

```bash
gomddoc serve ./docs --dev
```

Or the environment variable:

```bash
export GOMDDOC_SERVER_DEV_MODE=true
gomddoc serve
```

### Using the `preview` command

The `preview` command enables dev mode automatically — no flag needed:

```bash
gomddoc preview ./docs
```

Preview also auto-assigns a port (starting from 8080) and can open your browser:

```bash
gomddoc preview ./docs --open
```

## Features

### 1. Template Reloading

In production, parsed templates are cached in memory via `CachedTemplateStore` (backed by `sync.Map`) for
performance. In dev mode, a `PassthroughTemplateStore` is used instead — templates are re-parsed on every request.
This ensures that any change you make to a `.tmpl` file is visible immediately upon refreshing your browser.

This applies to:
- Layout templates (`layouts/default.html.tmpl`)
- Partial templates (`partials/*.html.tmpl`)
- Theme overrides in `.gomddoc/assets/`

### 2. Config Logging

When dev mode is enabled, the full configuration is logged at startup for debugging:

```
INFO Development mode enabled config={...}
```

### 3. Debug Logging

You will see more verbose `slog.Debug` output in the console, including:
- File lookups in overlay filesystem layers
- Hidden path blocking events
- Environment variable overrides applied
- Template cache operations

### 4. Caching Disabled

In dev mode, HTTP caching headers (`Cache-Control`, `ETag`) are still sent but have no practical effect since
content is always re-rendered from source. This means:
- Every page refresh shows the latest content
- Template changes take effect immediately
- Configuration changes to `.gomddoc/config.yml` are picked up on next request

## Auto-Port Assignment

Both `preview` (by default) and `serve` (with `-p :auto`) support automatic port assignment. The server scans
for the first available port starting from 8080, which is useful when:

- Running multiple gomddoc instances simultaneously
- Port 8080 is already in use by another service
- Running in CI environments where ports may be unpredictable

```bash
# Auto-assign port
gomddoc serve -p :auto

# Preview always auto-assigns
gomddoc preview
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
