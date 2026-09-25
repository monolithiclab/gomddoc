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

# Docker
docker run --rm -p 8080:8080 -v "$PWD:/site" ghcr.io/monolithiclab/gomddoc serve /site
```

The install script picks the archive matching your OS and architecture, verifies it against the
release `SHA256SUMS`, and installs to `/usr/local/bin` (through `sudo` when that directory is not
writable, and to `~/.local/bin` when `sudo` is not available either). Set `GOMDDOC_INSTALL_DIR` to
choose a different target, or `GOMDDOC_VERSION` to pin a release instead of taking the latest.

When [cosign](https://github.com/sigstore/cosign) is installed, the script verifies the keyless
signature over `SHA256SUMS` before trusting it. Install cosign first: `SHA256SUMS` is served from
the same origin as the archive, so the checksum alone does not prove where the binary came from.
`GOMDDOC_REQUIRE_COSIGN=1` turns the missing-cosign warning into an abort. The install also fails
closed if no SHA-256 tool is available, rather than skipping the checksum.

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

This creates `.gomddoc/config.yml` with the defaults written out: `default_index: README.md`, the site
title derived from the directory name, an empty description, the `default` theme and the `github`
highlighting style. `init` refuses to overwrite an existing config file. You can name a different
theme during init:

```bash
gomddoc init --theme midnight
```

Only `default` is bundled. Another theme, such as `midnight`, must be installed under
`.gomddoc/assets/themes/midnight/` (see [Theming](05-theming-and-assets.md)); until it is, pages
render with `default`.

Configuration is optional. gomddoc runs with no config file at all, using `README.md` as the index
page and the `default` theme.

### 3. Quick Preview

Preview your docs on the first free port:

```bash
gomddoc preview
```

This starts the server in development mode (the template cache is off, so template edits show on
reload) with automatic port assignment starting from 8080, and prints the URL. Add `--open` to
launch your default browser automatically:

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

Requests to the original `.md` path (e.g., `/guide.md`) are 301-redirected to the clean URL
(`/guide`). A directory URL such as `/guide/` serves that directory's `README.md`.

### 4. Static Site Generation

Build a complete static site for deployment to any hosting platform:

```bash
gomddoc build ./my-docs -o ./public
```

This renders all Markdown files through the full template pipeline and generates HTML in `./public/`
(default: `build/site`), ready for S3, Netlify, GitHub Pages, or Cloudflare Pages. Non-markdown files
(images, CSS, JS) are copied as-is. The output always includes `robots.txt` and `404.html`, and adds
`sitemap.xml` and `feed.xml` when a domain is configured (`meta.domain` or `--domain`).

### 5. AI-Native Documentation Access

Expose your documentation to AI models via the Model Context Protocol:

```bash
gomddoc mcp ./docs
```

AI clients like Claude Desktop, Cursor, and Claude Code can then search, read individual sections,
browse the navigation tree, and discover related pages. See [MCP Server](04-mcp.md) for setup
instructions.

## CLI Commands

gomddoc provides these commands:

| Command                            | Purpose                                                                     |
| ---------------------------------- | --------------------------------------------------------------------------- |
| `gomddoc serve [DIR]`              | Production HTTP server with admin port, pprof, Basic Auth and Git sources   |
| `gomddoc preview [DIR]`            | Quick preview with auto-port, dev mode, and optional browser auto-open      |
| `gomddoc build [DIR]`              | Static site generation — renders all markdown to HTML files                 |
| `gomddoc mcp [DIR]`                | MCP server for AI-native documentation access (stdio transport)             |
| `gomddoc init [DIR]`               | Scaffold a `.gomddoc/config.yml` with the defaults written out              |
| `gomddoc doctor [DIR]`             | Check configuration and content; report every problem with a fix            |
| `gomddoc info [DIR]`               | Show version, config file location, every setting, commands and theme       |
| `gomddoc schema`                   | Print the JSON Schema for `.gomddoc/config.yml`                             |
| `gomddoc help [TOPIC [SECTION]]`   | Read this guide in the terminal: list topics, read one, or `--search` it    |

Every command except `schema` and `help` accepts an optional `DIR` argument (default: `.`). For
`serve`, `preview`, `build`, `mcp` and `doctor` it is a content directory or Git URL; `info` does not
clone Git URLs. Run `gomddoc info` to see every setting with its environment variable and default,
and `gomddoc <command> --help` for a command's flags.

## Next Steps

- Configure the server: [Configuration](02-configuration.md)
- Connect to private repos: [Content Sources](03-content-sources.md)
- Expose docs to AI: [MCP Server](04-mcp.md)
- Customize the look: [Theming](05-theming-and-assets.md)
- Write richer content: [Markdown Extensions](12-advanced/02-markdown-extensions.md)
- Set up search: [Full-Text Search](10-search.md)
- Optimize for search engines: [SEO](11-seo.md)
- Add languages: [Internationalization](13-internationalization.md)
- Check a site for mistakes: [Doctor](14-doctor.md)
- Deploy to production: [Deployment](09-deployment.md)
