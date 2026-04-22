---
title: "Configuration Guide"
description: "Detailed guide on configuring gomddoc via CLI flags, environment variables, and config files."
author: "nicolasm"
---

# Configuration

gomddoc uses a two-layer configuration system to separate **how the server runs** from **how the content looks**.

1.  **Runtime Configuration:** Controls the HTTP server process, networking, and core behavior. Configured via CLI flags or environment variables.
2.  **Site Configuration:** Controls the website metadata and theme. Configured via a `.gomddoc/config.yml` file or environment variables.

---

## 1. Runtime Configuration

These settings affect the `gomddoc` process itself. They are typically set by the person deploying or running the application.

### Basic Options

**Content Directory (`-d`)**
Sets the source of your documentation. This can be a local path (e.g., `./docs`) or a Git URL (e.g., `git+https://...`).
*   **CLI Flag:** `-d`
*   **Env Var:** `GOMDDOC_DIR`
*   **Default:** `.` (Current directory)

**Server Port (`-p`)**
Sets the network address and port the server listens on.
*   **CLI Flag:** `-p`
*   **Env Var:** `GOMDDOC_PORT`
*   **Default:** `:8080`

**Development Mode (`-dev`)**
Enables development features: activates hot-reloading for `.gomddoc/config.yml` and theme templates, and disables all response caching.
*   **CLI Flag:** `-dev`
*   **Env Var:** `GOMDDOC_DEV_MODE`
*   **Default:** `false`

**Git SSH Key (`--git-key-file`)**
The absolute path to a private SSH key file used for authenticating with private Git repositories.
*   **CLI Flag:** `--git-key-file`
*   **Env Var:** `GOMDDOC_GIT_SSH_KEY_FILE`
*   **Default:** Empty (Anonymous access only)

### Process & Security

**Shutdown Timeout**
The maximum duration to wait for active requests to finish before the server forcefully exits during a shutdown signal.
*   **Env Var:** `GOMDDOC_SHUTDOWN_TIMEOUT`
*   **Default:** `1s`

**Default Index**
The filename gomddoc looks for when a directory is requested (e.g., `/api/`).
*   **Env Var:** `GOMDDOC_SERVER_DEFAULT_INDEX`
*   **Default:** `README.md`

**Directory Listings**
If enabled, gomddoc will generate a Markdown list of files when a directory is requested and no index file is found.
*   **Env Var:** `GOMDDOC_SERVER_DIR_INDEX`
*   **Default:** `false` (Returns 403 Forbidden for security)

### Network Tuning

**Read Header Timeout**
The maximum time allowed to read request headers. This is a critical setting for mitigating Slowloris attacks.
*   **Env Var:** `GOMDDOC_READ_HEADER_TIMEOUT`
*   **Default:** `5s` (Max: `60s`)

**Write Timeout**
The maximum duration before timing out writes of the response.
*   **Env Var:** `GOMDDOC_WRITE_TIMEOUT`
*   **Default:** `30s` (Max: `5m`)

**Idle Timeout**
The maximum amount of time to wait for the next request when keep-alives are enabled.
*   **Env Var:** `GOMDDOC_IDLE_TIMEOUT`
*   **Default:** `120s` (Max: `10m`)

**Max Header Size**
The maximum allowed size of request headers in Megabytes.
*   **Env Var:** `GOMDDOC_MAX_HEADER_MB`
*   **Default:** `1` (1MB) (Max: `10`)

---

## 2. Site Configuration

These settings control the presentation of your documentation. You define these in a `.gomddoc/config.yml` file located at the root of your content directory.

### Section: Meta

**Title**
The name of your documentation site. It appears in browser tabs and the site header.
*   **YAML:** `meta.title`
*   **Env Var:** `GOMDDOC_META_TITLE`
*   **Default:** The capitalized name of your content directory.

**Description**
A short summary of your site, used for the HTML `<meta name="description">` SEO tag.
*   **YAML:** `meta.description`
*   **Env Var:** `GOMDDOC_META_DESCRIPTION`
*   **Default:** Empty.

**Domain**
The primary domain name for your site (e.g., `docs.example.com`). Used for internal validation and canonical URL generation.
*   **YAML:** `meta.domain`
*   **Env Var:** `GOMDDOC_META_DOMAIN`
*   **Default:** Empty.

### Section: Theme

**Theme Name**
Selects the visual theme to apply. gomddoc looks for a folder with this name in the internal or local `assets/themes/` directory.
*   **YAML:** `theme.name`
*   **Env Var:** `GOMDDOC_THEME_NAME`
*   **Default:** `default`

---

## Priority Order

When a setting is defined in multiple places, gomddoc follows this strict priority order:

1.  **CLI Flags:** Always take precedence.
2.  **Environment Variables:** Override configuration files.
3.  **Config File:** Values defined in `.gomddoc/config.yml`.
4.  **Defaults:** Hardcoded fallback values.