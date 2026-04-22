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

This starts the server in dev mode (no caching) and auto-assigns a port starting from 8080. Add `--open` to launch your default browser automatically.

### 4. Live Server (Local Folder)

For production use, `serve` gives full control:

```bash
gomddoc serve
```

Open your browser to `http://localhost:8080`. You'll see your markdown rendered with auto-generated navigation.

Serve a specific directory on a different port:

```bash
gomddoc serve ./my-docs -p :3000
```

### 5. Serving from Git

Serve directly from a public repository without cloning it manually (lazy clone on first request):

```bash
gomddoc serve "git+https://github.com/monolithiclab/gomddoc.git"
```

### 6. Static Site Generation

Build a static site for deployment:

```bash
gomddoc build ./my-docs -o ./public
```

This generates HTML files in `./public/` ready for S3, Netlify, or GitHub Pages.

## CLI Commands

gomddoc provides four commands:

| Command | Purpose |
|---------|---------|
| `gomddoc serve [DIR]` | Production HTTP server with full control over port, caching, Git sources |
| `gomddoc preview [DIR]` | Quick preview with auto-port, no caching, and optional browser auto-open |
| `gomddoc build [DIR]` | Static site generation — renders all markdown to HTML files |
| `gomddoc init [DIR]` | Scaffold a `.gomddoc/config.yml` with sensible defaults |

All commands accept an optional `DIR` argument (default: `.`) specifying the content directory or Git URL. See [Configuration](02-configuration.md) for the full reference of flags, environment variables, and config file options.

## Next Steps

*   Configure the server: [Configuration](02-configuration.md)
*   Connect to private repos: [Content Sources](03-content-sources.md)
*   Customize the look: [Theming](04-theming-and-assets.md)
*   Set up search: [Full-Text Search](11-search.md)
*   Optimize for search engines: [SEO](12-seo.md)
