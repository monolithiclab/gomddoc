---
title: "Configuration Guide"
description: "Detailed guide on configuring gomddoc via CLI flags, environment variables, and config files."
author: "nicolasm"
---

# Configuration

gomddoc uses a structured configuration system divided into two main sections:
1.  **Server (`SERVER`)**: Runtime settings (port, host, timeouts, content source). Configured via CLI flags or Environment Variables.
2.  **Site (`SITE`)**: Content presentation (metadata, theme, behavior). Configured via `.gomddoc/config.yml` or Environment Variables.

---

## 1. Runtime Configuration (`SERVER`)

These settings control the `gomddoc` process.

### Basic Options

**Content Directory**
Sets the source of your documentation. This can be a local path (e.g., `./docs`) or a Git URL.
*   **CLI Flag:** `-d`
*   **Env Var:** `GOMDDOC_SERVER_DIR`
*   **Default:** `.` (Current directory)

**Server Port**
Sets the network address and port the server listens on.
*   **CLI Flag:** `-p`
*   **Env Var:** `GOMDDOC_SERVER_PORT`
*   **Default:** `:8080`

**Development Mode**
Enables development features: activates hot-reloading for `.gomddoc/config.yml` and theme templates, and disables all response caching.
*   **CLI Flag:** `-dev`
*   **Env Var:** `GOMDDOC_SERVER_DEV_MODE`
*   **Default:** `false`

**Git SSH Key**
The absolute path to a private SSH key file used for authenticating with private Git repositories.
*   **CLI Flag:** `--git-key-file`
*   **Env Var:** `GOMDDOC_SERVER_GIT_SSH_KEY`
*   **Default:** Empty (Anonymous access only)

### Network Tuning (`SERVER.HTTP`)

Advanced settings to tune the HTTP server timeouts and limits.

| Env Variable | Description | Default | Max |
| :--- | :--- | :--- | :--- |
| `GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT` | Graceful shutdown duration. | `1s` | `60s` |
| `GOMDDOC_SERVER_HTTP_READ_HEADER_TIMEOUT` | Max time to read request headers. | `5s` | `60s` |
| `GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT` | Max time to write response. | `30s` | `5m` |
| `GOMDDOC_SERVER_HTTP_IDLE_TIMEOUT` | Keep-alive connection idle time. | `120s` | `10m` |
| `GOMDDOC_SERVER_HTTP_MAX_HEADER_MB` | Max request header size (MB). | `1` | `10` |

---

## 2. Site Configuration (`SITE`)

These settings control how your documentation is presented and served. They are typically defined in a `.gomddoc/config.yml` file located at the root of your content directory.

**Example `.gomddoc/config.yml`:**

```yaml
default_index: "README.md"
dir_index: false

meta:
  title: "My Project Docs"
  description: "API Reference"
  domain: "docs.example.com"

theme:
  name: "default"
```

### Content Behavior

**Default Index**
The filename to look for when a user requests a directory (e.g., `/api/`).
*   **YAML:** `default_index`
*   **Env Var:** `GOMDDOC_SITE_DEFAULT_INDEX`
*   **Default:** `README.md`

**Directory Listings**
Controls whether to auto-generate a Markdown list of files when a directory is requested but no `README.md` exists.
*   **YAML:** `dir_index`
*   **Env Var:** `GOMDDOC_SITE_DIR_INDEX`
*   **Default:** `false` (Returns 403 Forbidden for security)

### Metadata (`SITE.META`)

**Title**
The name of your documentation site. It appears in browser tabs and the site header.
*   **YAML:** `meta.title`
*   **Env Var:** `GOMDDOC_SITE_META_TITLE`
*   **Default:** The capitalized name of your content directory.

**Description**
A short summary of your site, used for the HTML `<meta name="description">` SEO tag.
*   **YAML:** `meta.description`
*   **Env Var:** `GOMDDOC_SITE_META_DESCRIPTION`
*   **Default:** Empty.

**Domain**
The primary domain name for your site (e.g., `docs.example.com`). Used for internal validation and canonical URL generation.
*   **YAML:** `meta.domain`
*   **Env Var:** `GOMDDOC_SITE_META_DOMAIN`
*   **Default:** Empty.

### Theme (`SITE.THEME`)

**Theme Name**
Selects the visual theme to apply. gomddoc looks for a folder with this name in the internal or local `assets/themes/` directory.
*   **YAML:** `theme.name`
*   **Env Var:** `GOMDDOC_SITE_THEME_NAME`
*   **Default:** `default`

---

## HTTP Caching Behavior

gomddoc includes built-in HTTP caching support to reduce bandwidth and improve performance for repeat visitors.

### Cache-Control

All successful responses include a `Cache-Control: public, max-age=300` header, allowing browsers and intermediate caches to store responses for 5 minutes before revalidating.

### ETag / 304 Not Modified

Every response includes an `ETag` header, a content-based fingerprint computed using the FNV-64a hash algorithm. The ETag is a weak validator (prefixed with `W/`) since the same content may be served with different transfer encodings (e.g., gzip).

When a browser makes a subsequent request, it sends the cached ETag in the `If-None-Match` header. If the content has not changed, gomddoc responds with `304 Not Modified` and an empty body, saving bandwidth and processing time.

This applies to all content types:
- **Markdown pages**: The ETag is computed from the fully rendered HTML (after template wrapping), so any change to content, metadata, or templates produces a new ETag.
- **Static assets** (CSS, JS, images): The ETag is computed from the raw file content.

No configuration is required. ETag caching is always enabled.

---

## Priority Order

When a setting is defined in multiple places, gomddoc follows this strict priority order:

1.  **CLI Flags:** Always take precedence.
2.  **Environment Variables:** Override configuration files.
3.  **Config File:** Values defined in `.gomddoc/config.yml`.
4.  **Defaults:** Hardcoded fallback values.
