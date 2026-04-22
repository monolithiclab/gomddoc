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
4. [Theming & Assets](04-theming-and-assets.md)
5. [Development Mode](05-development-mode.md)
6. [Security](06-security.md)
7. [HTTP Behavior](07-http-behavior.md)
8. [Markdown Extensions](08-markdown-extensions.md)
9. [Observability](09-observability.md)
10. [Deployment](10-deployment.md)
11. [Full-Text Search](11-search.md)
12. [SEO](12-seo.md)
13. [API Reference](13-api-reference.md)

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
- **Static Site Generation:** Build static HTML sites for deployment to any host (S3, Netlify, GitHub Pages).
- **Production Ready:** Graceful shutdown, configurable timeouts, buffer pooling, structured logging with log
  injection prevention, Prometheus metrics, health endpoints.
