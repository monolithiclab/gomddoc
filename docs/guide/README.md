---
title: "Gomddoc User Guide"
description: "Comprehensive user guide for gomddoc, covering features, configuration, and usage."
author: "nicolasm"
---

# Gomddoc User Guide

## What is gomddoc?

**gomddoc** is a documentation server and static site generator that turns a folder of Markdown files
into a fully styled, searchable documentation site. Point it at a local directory or a remote Git
repository, and it renders your content as HTML on the fly — complete with navigation sidebar, table
of contents, full-text search, and eight built-in themes. When you are ready to deploy, `gomddoc build`
produces static HTML files you can host anywhere.

gomddoc requires no databases, no CMS, and no build pipelines. Your Markdown files are the source of
truth, and Git is your version control system. The server is stateless by design: restart it to pick
up content changes, or serve directly from a Git URL to always show the latest version.

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
  includes sitemap.xml, robots.txt, canonical URLs, and Open Graph tags out of the box.
- **AI-Native Access:** The built-in MCP server lets AI models (Claude, GPT, Copilot) search, read,
  and navigate your documentation programmatically — no scraping or copy-pasting.

## Table of Contents

1. [Quick Start](01-quickstart.md) — Install, configure, and preview your first docs
2. [Configuration](02-configuration.md) — CLI flags, environment variables, and config file reference
3. [Content Sources (Git & Local)](03-content-sources.md) — Filesystem, Git URLs, SSH auth, disk storage
4. [MCP Server](04-mcp.md) — AI-native documentation access via the Model Context Protocol
5. [Theming & Assets](05-theming-and-assets.md) — Eight built-in themes, custom themes, template functions
6. [Writing Workflow](06-writing-workflow.md) — Preview, dev mode, auto-port, authoring tips
7. [Security](07-security.md) — Authentication, path traversal, hidden files, security headers
8. [Observability](08-observability.md) — Health checks, Prometheus metrics, pprof profiling
9. [Deployment](09-deployment.md) — Live server, Docker, Kubernetes, static hosting
10. [Full-Text Search](10-search.md) — Built-in search engine with TF-IDF ranking and keyboard shortcuts
11. [SEO](11-seo.md) — Canonical URLs, sitemap, robots.txt, Open Graph tags
### Advanced Topics

- [HTTP Behavior](12-advanced/01-http-behavior.md) — Caching, compression, content negotiation
- [Markdown Extensions](12-advanced/02-markdown-extensions.md) — GFM, frontmatter, admonitions, KaTeX, Mermaid, color chips
- [API Reference](12-advanced/03-api-reference.md) — All HTTP endpoints, MCP tools, and system routes

## Core Features

- **Universal Serving:** Renders Markdown to HTML on-the-fly with automatic navigation, table of
  contents, and breadcrumbs. Serves images, PDFs, JSON, and scripts natively alongside your docs.
- **Git Native:** Connects directly to public or private Git repositories (GitHub, GitLab, Bitbucket)
  without manual cloning. Lazy shallow clones minimize bandwidth and memory. SSH authentication with
  strict host key verification.
- **Full-Text Search:** Built-in inverted index with TF-IDF ranking, title and description boosts,
  and snippet generation. Accessible via Ctrl+K keyboard shortcut, a header search button, or the
  REST API. Works across all themes with zero configuration.
- **MCP Server:** Built-in Model Context Protocol server lets AI models (Claude, GPT, Copilot) search,
  read, and navigate your documentation programmatically. Six tools, four resources, and three prompt
  templates — all read-only and safe for auto-approval.
- **Eight Themes:** Ships with default, academic, gitbook, material, midnight, minimal, nord, and
  ocean themes. All support light/dark mode, responsive design, and every rendering feature. Create
  custom themes with Go templates and CSS custom properties.
- **SEO Ready:** Automatic sitemap.xml, robots.txt, canonical URLs, and Open Graph meta tags. All
  features work in both serve and build modes. Configure a domain and everything activates.
- **Markdown Extensions:** GitHub Flavored Markdown, YAML frontmatter, five admonition types, KaTeX
  math rendering, Mermaid diagrams, interactive color chips, heading anchors, and scroll-tracked
  table of contents.
- **Secure by Default:** Path traversal protection, hidden file blocking, HTTP method filtering
  (GET/HEAD only), SSH host key verification (no TOFU), comprehensive security headers, and optional
  HTTP Basic Authentication via htpasswd.
- **Static Site Generation:** Build complete static HTML sites for deployment to S3, Netlify, GitHub
  Pages, or Cloudflare Pages. Output includes all SEO files and theme assets.
- **Zero-Config:** Works out of the box with sensible defaults. Use `README.md` as index, the default
  theme, and no configuration file. Add a `.gomddoc/config.yml` when you need customization.
- **Production Ready:** Graceful shutdown, configurable timeouts, buffer pooling, structured logging
  with log injection prevention, Prometheus metrics, health endpoints, and pprof profiling.
