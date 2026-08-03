---
title: "Quick Start"
description: "A quick start guide for new users."
author: "nicolasm"
---

# Quick Start

### 1. Installation

```bash
# Install script (Linux / macOS, amd64 / arm64)
curl -fsSL https://raw.githubusercontent.com/monolithiclab/gomddoc/main/scripts/install.sh | sh

# Homebrew (macOS / Linux)
brew install monolithiclab/tap/gomddoc

# Go toolchain
go install github.com/monolithiclab/gomddoc/cmd/gomddoc@latest
```

The install script picks the archive matching your OS and architecture, verifies it against the
release `SHA256SUMS`, and installs to `/usr/local/bin` (falling back to `~/.local/bin` when that
is not writable). Set `GOMDDOC_INSTALL_DIR` to choose a different target, or `GOMDDOC_VERSION`
to pin a release instead of taking the latest.

When [cosign](https://github.com/sigstore/cosign) is installed, the script verifies the keyless
signature over `SHA256SUMS` before trusting it — worth doing, because that file is served from the
same origin as the archive. `GOMDDOC_REQUIRE_COSIGN=1` turns the missing-cosign warning into an
abort. The install also fails closed if no SHA-256 tool is available, rather than skipping the
checksum.

Building from source:

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

This creates `.gomddoc/config.yml` with sensible defaults — site title derived from the directory
name, the default theme selected, and color chips enabled. You can pick a different theme during init:

```bash
gomddoc init --theme midnight
```

Configuration is optional. gomddoc works out of the box with no config file at all, using `README.md`
as the index page and the `default` theme.

### 3. Quick Preview

Preview your docs with zero friction — auto-finds a free port and opens your browser:

```bash
gomddoc preview
```

This starts the server in development mode (no caching, verbose logging) with automatic port
assignment starting from 8080. Add `--open` to launch your default browser automatically:

```bash
gomddoc preview --open
```

### URL Structure

Files in your content directory map to clean, extensionless URLs:

| File | URL |
|------|-----|
| `README.md` | `/` |
| `guide.md` | `/guide` |
| `guide/setup.md` | `/guide/setup` |
| `api/reference.md` | `/api/reference` |

Requests to the original `.md` path (e.g., `/guide.md`) are automatically 301-redirected to the
clean URL (`/guide`).

### 4. Static Site Generation

Build a complete static site for deployment to any hosting platform:

```bash
gomddoc build ./my-docs -o ./public
```

This renders all Markdown files through the full template pipeline and generates HTML in `./public/`,
ready for S3, Netlify, GitHub Pages, or Cloudflare Pages. Non-markdown files (images, CSS, JS) are
copied as-is. The output includes `robots.txt`, `feed.xml`, and `sitemap.xml` when a domain is
configured.

### 5. AI-Native Documentation Access

Expose your documentation to AI models via the Model Context Protocol:

```bash
gomddoc mcp ./docs
```

AI clients like Claude Desktop, Cursor, and Claude Code can then search, read individual sections,
browse the navigation tree, and discover related pages — all without leaving their interface. See
[MCP Server](04-mcp.md) for setup instructions.

## CLI Commands

gomddoc provides these commands:

| Command                 | Purpose                                                                  |
| ----------------------- | ------------------------------------------------------------------------ |
| `gomddoc serve [DIR]`   | Production HTTP server with full control over port, caching, Git sources |
| `gomddoc preview [DIR]` | Quick preview with auto-port, no caching, and optional browser auto-open |
| `gomddoc build [DIR]`   | Static site generation — renders all markdown to HTML files              |
| `gomddoc mcp [DIR]`     | MCP server for AI-native documentation access (stdio transport)          |
| `gomddoc init [DIR]`    | Scaffold a `.gomddoc/config.yml` with sensible defaults                  |
| `gomddoc info`          | Show version, config file location, and all environment variables        |

All commands accept an optional `DIR` argument (default: `.`) specifying the content directory or
Git URL. Run `gomddoc info` to see all available environment variables and their defaults.

## Next Steps

- Configure the server: [Configuration](02-configuration.md)
- Connect to private repos: [Content Sources](03-content-sources.md)
- Expose docs to AI: [MCP Server](04-mcp.md)
- Customize the look: [Theming](05-theming-and-assets.md)
- Write richer content: [Markdown Extensions](12-advanced/02-markdown-extensions.md)
- Set up search: [Full-Text Search](10-search.md)
- Optimize for search engines: [SEO](11-seo.md)
- Add languages: [Internationalization](13-internationalization.md)
- Deploy to production: [Deployment](09-deployment.md)
