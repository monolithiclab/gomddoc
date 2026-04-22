---
title: "Gomddoc User Guide"
description: "Comprehensive user guide for gomddoc, covering features, configuration, and usage."
author: "nicolasm"
---

# Gomddoc User Guide

**gomddoc** is a high-performance, production-ready HTTP server designed to serve Markdown documentation as rendered HTML. It bridges the gap between static files and dynamic serving, supporting content from local directories or remote Git repositories with zero build steps.

## Table of Contents

1. [Introduction & Quick Start](01-introduction.md)
2. [Configuration](02-configuration.md)
3. [Content Sources (Git & Local)](03-content-sources.md)
4. [Theming & Assets](04-theming-and-assets.md)
5. [Development Mode](05-development-mode.md)
6. [Security](06-security.md)

## Core Features

*   **Universal Serving:** Renders Markdown to HTML on-the-fly; serves images, PDFs, JSON, and scripts natively.
*   **Git Native:** Connects directly to public or private Git repositories (GitHub, GitLab, etc.) without manual cloning.
*   **Hot Reload:** Built-in development mode that detects changes instantly.
*   **Secure by Default:** Path traversal protection, hidden file blocking, and security headers.
*   **Zero-Config:** Works out of the box with sensible defaults (like using `README.md` as index).
*   **Extensible:** Supports custom templates and configurable metadata via YAML.
