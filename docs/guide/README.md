---
title: "Gomddoc User Guide"
description: "Comprehensive user guide for gomddoc, covering features, configuration, and usage."
author: "nicolasm"
---

# Gomddoc User Guide

## What is gomddoc?

**gomddoc** is a documentation server and static site generator that turns a folder of Markdown files
into a styled, searchable documentation site. Point it at a local directory or a remote Git
repository, and it renders your content as HTML on request, with a navigation sidebar, table of
contents, full-text search, and a theme system. When you are ready to deploy, `gomddoc build`
produces static HTML files you can host anywhere.

gomddoc requires no databases, no CMS, and no build pipelines. Your Markdown files are the source of
truth, and Git is your version control system. The server keeps no state of its own: navigation,
search and metadata indexes are built at startup, so restart it to pick up added or renamed pages. A
Git URL is cloned once at startup and served at that commit until the next restart.

It is designed for:

- **Team Documentation:** Serve your internal docs directly from a Git repository with automatic
  navigation, search, and metadata indexing. Tag pages with YAML frontmatter for cross-cutting
  discovery.
- **API References:** Render Markdown specs with syntax-highlighted code blocks, math formulas
  (KaTeX), and diagrams (Mermaid). Content negotiation lets API clients request raw markdown
  instead of HTML.
- **Project Wikis:** A lightweight, fast viewer for any folder of notes — no configuration
  required. Run `gomddoc preview` and start reading.
- **Static Sites:** Build and deploy to S3, Netlify, GitHub Pages, or any static host. The output
  includes robots.txt, and with `meta.domain` set, sitemap.xml, feed.xml and canonical URLs.
- **AI-Native Access:** The built-in MCP server lets AI clients (Claude, Cursor, Copilot) search, read,
  and navigate your documentation without scraping HTML.

## Table of Contents

1. [Quick Start](01-quickstart.md) — Install, configure, and preview your first docs
2. [Configuration](02-configuration.md) — CLI flags, environment variables, and config file reference
3. [Content Sources (Git & Local)](03-content-sources.md) — Filesystem, Git URLs, SSH auth, disk storage
4. [MCP Server](04-mcp.md) — AI-native documentation access via the Model Context Protocol
5. [Theming & Assets](05-theming-and-assets.md) — The bundled theme, seven downloadable ones, custom themes, template functions
6. [Writing Workflow](06-writing-workflow.md) — Preview, dev mode, auto-port, authoring tips
7. [Security](07-security.md) — Authentication, path traversal, hidden files, security headers
8. [Observability](08-observability.md) — Health checks, Prometheus metrics, pprof profiling
9. [Deployment](09-deployment.md) — Live server, Docker, Kubernetes, static hosting
10. [Full-Text Search](10-search.md) — Built-in search engine with TF-IDF ranking and keyboard shortcuts
11. [SEO](11-seo.md) — Canonical URLs, sitemap, robots.txt, Open Graph tags, Atom feeds
12. [Advanced Topics](12-advanced/README.md) — HTTP behavior, Markdown extensions, API reference, custom renderers
    1. [HTTP Behavior](12-advanced/01-http-behavior.md) — Caching, compression, content negotiation, URL resolution
    2. [Markdown Extensions](12-advanced/02-markdown-extensions.md) — GFM, frontmatter, admonitions, KaTeX, Mermaid, color chips
    3. [API Reference](12-advanced/03-api-reference.md) — All HTTP endpoints, MCP tools, and system routes
    4. [Custom Renderers](12-advanced/04-custom-renderers.md) — The `ContentRenderer` contract, negotiation, registration, testing
13. [Internationalization](13-internationalization.md) — Multi-language sites, translations, hreflang, language switcher
14. [Doctor](14-doctor.md) — Check configuration and content: every problem with its location and fix

The same guide ships in the binary: `gomddoc help` lists its topics, `gomddoc help <topic>` reads one, and
`gomddoc help --search <query>` searches it.

## Commands

| Command | Purpose |
|---------|---------|
| `gomddoc serve [DIR]` | Serve a directory or Git URL as HTML over HTTP (default port `:8080`) |
| `gomddoc preview [DIR]` | Local preview: auto-assigned port, dev mode on, optional `--open` to launch a browser |
| `gomddoc build [DIR]` | Write a static site to `--output` (default `build/site`) |
| `gomddoc init [DIR]` | Create `.gomddoc/` with a default `config.yml` |
| `gomddoc mcp [DIR]` | Run the MCP server on stdio for local AI clients |
| `gomddoc doctor [DIR]` | Check configuration and content; report every problem with a fix (`--json`, `--strict`) |
| `gomddoc info [DIR]` | List every setting with its config key, env var, flag and default, plus theme features and guide pages (`--json`) |
| `gomddoc schema` | Print the JSON Schema for `.gomddoc/config.yml` |
| `gomddoc help [TOPIC [SECTION]]` | Read this guide from the terminal |

`gomddoc <command> --help` lists a command's flags.

## Core Features

- **Universal Serving:** Renders Markdown to HTML on request with navigation, table of contents, and breadcrumbs.
  Serves images, PDFs, JSON, and scripts alongside your docs with their detected MIME type.
- **Git Native:** Serves public or private Git repositories (GitHub, GitLab, Bitbucket) without manual cloning.
  Shallow, single-branch clones keep bandwidth and memory low. SSH authentication with strict host key
  verification.
- **Full-Text Search:** Built-in inverted index with TF-IDF ranking, title and description boosts, `tag:` filters,
  and snippet generation. Opened with Ctrl+K / Cmd+K or the header search button, and queryable through
  `/api/search`. Available in `serve` and `preview`; `build` output has no search backend.
- **MCP Server:** Built-in Model Context Protocol server that lets AI clients search, read, and navigate your
  documentation. Six tools, four resources, and three prompts, all read-only. `gomddoc mcp` (stdio) also exposes
  gomddoc's own guide, capabilities report, and `gomddoc_doctor` check.
- **Themes:** The `default` theme is bundled. Seven more (academic, gitbook, material, midnight, minimal, nord,
  ocean) are available from [gomddoc-themes](https://github.com/monolithiclab/gomddoc-themes) and install under
  `.gomddoc/assets/themes/<name>/`. Themes support light/dark mode and responsive layouts, and expose per-feature
  toggles. Custom themes use Go templates and CSS custom properties.
- **Internationalization:** Multi-language documentation with BCP 47 directory-based content (`fr-FR/`, `es-ES/`),
  translated UI strings, a language switcher, per-language search, sitemaps and feeds, and hreflang tags. Add a
  language by creating a directory.
- **SEO:** sitemap.xml, robots.txt, Atom feed, canonical URLs, hreflang, Open Graph and Twitter Card tags, and
  JSON-LD structured data, in both `serve` and `build`. Setting `meta.domain` enables the ones that need absolute
  URLs.
- **Markdown Extensions:** GitHub Flavored Markdown, YAML frontmatter, five admonition types, Chroma syntax
  highlighting, KaTeX math, Mermaid diagrams, color chips, heading anchors, and a scroll-tracked table of contents.
- **Secure by Default:** Path traversal protection, hidden file blocking, HTTP method filtering (GET/HEAD only), SSH
  host key verification (no TOFU), security headers, and optional HTTP Basic Authentication via htpasswd (bcrypt).
- **Static Site Generation:** `gomddoc build` writes a static site for S3, Netlify, GitHub Pages, or Cloudflare
  Pages, with pretty URLs, SEO files, tag pages, a `404.html`, and theme assets.
- **Self-Describing:** `gomddoc info`, `gomddoc schema`, `gomddoc help` and `gomddoc doctor` describe every setting,
  the config schema, this guide, and what is wrong with a site. The same data is available to agents over MCP.
- **Zero-Config:** Works with defaults: `README.md` as the directory index, the default theme, and no configuration
  file. Run `gomddoc init` to create `.gomddoc/config.yml` when you need to customize.
- **Production Ready:** Graceful shutdown, configurable timeouts, buffer pooling, structured logging with log
  injection prevention, Prometheus metrics, health endpoints, a separate admin port, and pprof profiling.
