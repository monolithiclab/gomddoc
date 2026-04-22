---
title: "Gomddoc User Guide"
description: "Comprehensive user guide for gomddoc, covering features, configuration, and usage."
author: "nicolasm"
---

# Gomddoc User Guide

**gomddoc** is a high-performance, production-ready HTTP server designed to serve Markdown documentation as rendered
HTML. It bridges the gap between static files and dynamic serving, supporting content from local directories or
remote Git repositories with zero build steps.

## Table of Contents

1. [Introduction & Quick Start](01-introduction.md)
2. [Configuration](02-configuration.md)
3. [Content Sources (Git & Local)](03-content-sources.md)
4. [MCP Server](04-mcp.md)
5. [Theming & Assets](05-theming-and-assets.md)
6. [Development Mode](06-development-mode.md)
7. [Security](07-security.md)
8. [HTTP Behavior](08-http-behavior.md)
9. [Markdown Extensions](09-markdown-extensions.md)
10. [Observability](10-observability.md)
11. [Deployment](11-deployment.md)
12. [Full-Text Search](12-search.md)
13. [SEO](13-seo.md)
14. [API Reference](14-api-reference.md)

## Core Features

- **Universal Serving:** Renders Markdown to HTML on-the-fly; serves images, PDFs, JSON, and scripts natively.
- **Git Native:** Connects directly to public or private Git repositories (GitHub, GitLab, etc.) without manual
  cloning. Shallow clones with timeout enforcement.
- **Full-Text Search:** Built-in inverted index with TF-IDF ranking, accessible via keyboard shortcut or search button.
- **SEO Ready:** Automatic sitemap.xml, robots.txt, canonical URLs, and Open Graph tags.
- **Secure by Default:** Path traversal protection, hidden file blocking, HTTP method filtering (GET/HEAD only),
  SSH host key verification (no TOFU), and comprehensive security headers.
- **Zero-Config:** Works out of the box with sensible defaults (like using `README.md` as index).
- **Extensible:** Custom renderers, configurable themes, YAML front matter support, and auto-generated TOC.
- **MCP Server:** Built-in Model Context Protocol server for AI-native documentation access (Claude, Cursor, Copilot).
- **Static Site Generation:** Build static HTML sites for deployment to any host (S3, Netlify, GitHub Pages).
- **Production Ready:** Graceful shutdown, configurable timeouts, buffer pooling, structured logging with log
  injection prevention, Prometheus metrics, health endpoints.
