---
title: "Configuration Guide"
description: "Detailed guide on configuring gomddoc via CLI flags, environment variables, and config files."
author: "nicolasm"
---

# Configuration

gomddoc uses a tiered configuration system. This allows you to set defaults, override them with environment variables for deployment, and use CLI flags for immediate control.

**Priority (Highest to Lowest):**
1.  **CLI Flags**
2.  **Environment Variables**
3.  **Config File** (`.gomddoc/config.yml`)
4.  **Defaults**

## 1. CLI Flags

These control **how** the server runs (operational settings).

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-d` | Path to content (Local dir or Git URL) | `.` |
| `-p` | HTTP Port to listen on | `:8080` |
| `-dev` | Enable Development Mode (Hot Reload) | `false` |
| `--git-key-file` | Path to SSH private key for Git auth | `""` |

Example:
```bash
gomddoc -d ./docs -p :9090 -dev
```

## 2. Environment Variables

Useful for containerized environments (Docker, Kubernetes).

### Application Config
Controls server behavior.

| Variable | Maps to Flag | Description |
| :--- | :--- | :--- |
| `GOMDDOC_PORT` | `-p` | Server port (e.g., `:8080`) |
| `GOMDDOC_DIR` | `-d` | Content directory or Git URL |
| `GOMDDOC_DEV_MODE` | `-dev` | `true` or `false` |
| `GOMDDOC_GIT_SSH_KEY_FILE` | `--git-key-file` | Path to private SSH key |
| `GOMDDOC_SHUTDOWN_TIMEOUT` | N/A | Graceful shutdown duration (e.g., `5s`) |

### Site Config
Overrides settings usually found in `config.yml`.

| Variable | Maps to Config Field |
| :--- | :--- |
| `GOMDDOC_META_TITLE` | `meta.title` |
| `GOMDDOC_META_DOMAIN` | `meta.domain` |
| `GOMDDOC_THEME_NAME` | `theme.name` |

## 3. Configuration File (`.gomddoc/config.yml`)

This file controls **what** is displayed (site presentation). It must be placed inside a `.gomddoc` folder at the root of your content directory.

**File:** `/path/to/docs/.gomddoc/config.yml`

```yaml
meta:
  title: "My Project Docs"       # Overrides default directory-based title
  description: "Official API documentation"
  domain: "docs.example.com"     # Used for canonical URLs

theme:
  name: "default"                # Currently the only built-in theme
```

## 4. Defaults

If nothing is configured:
*   **Port:** `:8080`
*   **Directory:** Current working directory (`.`)
*   **Title:** The capitalized name of the root directory (e.g., serving `./my-project` yields title "My-project").
*   **Theme:** `default`
