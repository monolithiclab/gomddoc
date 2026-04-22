# Development Mode

For rapid iteration, gomddoc includes a development mode.

## Enabling Dev Mode

Use the `-dev` flag:

```bash
gomddoc -d ./docs -dev
```

Or the environment variable:

```bash
export GOMDDOC_DEV_MODE=true
gomddoc
```

## Features

### 1. Hot Reload
The server watches the `.gomddoc/` directory.
*   **Config Changes:** If you edit `config.yml`, the site configuration is reloaded immediately.
*   **Asset Changes:** If you edit templates in `.gomddoc/assets/`, the cache is cleared.

### 2. Caching Disabled
In production, parsed templates are cached in memory for performance. In dev mode, templates are re-parsed on every request. This ensures that any change you make to a `.tmpl` file is visible immediately upon refreshing your browser.

### 3. Debug Logging
You will see more verbose logs in the console indicating file watcher events:
`[DEV] Config file changed, reloading...`
