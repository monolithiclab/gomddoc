---
title: "Introduction & Quick Start"
description: "Introduction to gomddoc and a quick start guide for new users."
author: "nicolasm"
---

# Introduction & Quick Start

## What is gomddoc?

gomddoc is a documentation server and static site generator. It reads Markdown files and serves them as styled HTML — either on demand via `gomddoc serve` or as pre-built static files via `gomddoc build`.

It is designed for:
*   **Internal Documentation:** Serve your team's docs directly from your Git repo with auto-generated navigation.
*   **API References:** Quick, navigable access to Markdown specs with tag-based metadata indexing.
*   **Project Wikis:** A robust viewer for any folder of notes.
*   **Static Sites:** Build and deploy to S3, Netlify, or GitHub Pages.

## Quick Start

### 1. Installation

```bash
# Build from source
make build
# The binary is now at ./build/gomddoc

# Or install to $GOBIN
make install
```

### 2. Initialize Configuration (Optional)

Scaffold a `.gomddoc/` directory with a default `config.yml`:

```bash
gomddoc init
```

This creates `.gomddoc/config.yml` with sensible defaults (title derived from directory name, default theme, color chips enabled). Use `--theme` to pick a different theme:

```bash
gomddoc init --theme midnight
```

### 3. Quick Preview

Preview your docs with zero friction — auto-finds a free port and opens your browser:

```bash
gomddoc preview
```

This starts the server in dev mode (no caching), auto-assigns a port starting from 8080, and opens your default browser. Use `--no-open` to skip the browser launch.

### 4. Live Server (Local Folder)

For production use, `serve` gives full control:

```bash
gomddoc serve
```

Open your browser to `http://localhost:8080`. You'll see your markdown rendered with auto-generated navigation.

Serve a specific directory on a different port:

```bash
gomddoc serve -d ./my-docs -p :3000
```

### 5. Serving from Git

Serve directly from a public repository without cloning it manually (lazy clone on first request):

```bash
gomddoc serve -d "git+https://github.com/monolithiclab/gomddoc.git"
```

### 6. Static Site Generation

Build a static site for deployment:

```bash
gomddoc build -d ./my-docs -o ./public
```

This generates HTML files in `./public/` ready for S3, Netlify, or GitHub Pages.

## Next Steps

*   Configure the server: [Configuration](02-configuration.md)
*   Connect to private repos: [Content Sources](03-content-sources.md)
*   Customize the look: [Theming](04-theming-and-assets.md)
