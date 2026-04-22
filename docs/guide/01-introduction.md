---
title: "Introduction & Quick Start"
description: "Introduction to gomddoc and a quick start guide for new users."
author: "nicolasm"
---

# Introduction & Quick Start

## What is gomddoc?

gomddoc is a documentation server. Unlike static site generators (Hugo, Jekyll) that require a build step, or simple file servers (Python `http.server`, `caddy`) that just serve raw files, gomddoc sits in the middle. It reads Markdown files and serves them as styled HTML, on demand.

It is designed for:
*   **Internal Documentation:** Serve your team's docs directly from your Git repo.
*   **API References:** Quick, navigable access to Markdown specs.
*   **Project Wikis:** A robust viewer for any folder of notes.

## Quick Start

### 1. Installation

(Assuming you have the binary built as `gomddoc`)

```bash
# Build from source
make build
# The binary is now at ./build/gomddoc
```

### 2. Basic Usage (Local Folder)

Serve the current directory:

```bash
./build/gomddoc
```

Open your browser to `http://localhost:8080`.

Serve a specific directory on a different port:

```bash
./build/gomddoc -d ./my-docs -p :3000
```

### 3. Serving from Git

Serve directly from a public repository without cloning it manually:

```bash
./build/gomddoc -d "git+https://github.com/monolithiclab/gomddoc.git"
```

## Next Steps

*   Configure the server: [Configuration](02-configuration.md)
*   Connect to private repos: [Content Sources](03-content-sources.md)
*   Customize the look: [Theming](04-theming-and-assets.md)
