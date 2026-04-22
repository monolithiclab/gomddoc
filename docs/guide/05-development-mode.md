---
title: "Development Mode Guide"
description: "Instructions for using gomddoc's development mode with hot reload."
author: "nicolasm"
---

# Development Mode

For rapid iteration, gomddoc includes a development mode.

## Enabling Dev Mode

Use the `-dev` flag:

```bash
gomddoc serve ./docs --dev
```

Or the environment variable:

```bash
export GOMDDOC_SERVER_DEV_MODE=true
gomddoc
```

## Features

### 1. Template Reloading

In production, parsed templates are cached in memory via `CachedTemplateStore` (backed by `sync.Map`) for
performance. In dev mode, a `PassthroughTemplateStore` is used instead — templates are re-parsed on every request.
This ensures that any change you make to a `.tmpl` file is visible immediately upon refreshing your browser.

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
