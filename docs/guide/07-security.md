---
title: "Security Features Guide"
description: "Overview of gomddoc's security features, including path traversal protection and hidden file blocking."
author: "nicolasm"
---

# Security

gomddoc is designed to be secure enough to be exposed to the internet, although running it behind a reverse proxy
(like Nginx or Cloudflare) is recommended for TLS and DDoS protection.

## 1. HTTP Basic Authentication

gomddoc supports HTTP Basic Authentication via htpasswd files with bcrypt-hashed passwords. Passwords are never passed as CLI arguments or environment variables — they stay hashed on disk.

```bash
# Create an htpasswd file with bcrypt hashing (-B flag)
htpasswd -Bc .htpasswd admin
htpasswd -B .htpasswd viewer

# Start the server
gomddoc serve --basic-auth-file .htpasswd
```

Only bcrypt hashes (`$2y$`, `$2a$`, `$2b$`) are supported. Comments (`#`) and blank lines are allowed in the file.

When enabled:

- All content requests require valid credentials. Unauthenticated requests receive `401 Unauthorized` with a `WWW-Authenticate` header prompting the browser to show a login dialog.
- **Health endpoints (`/health/live`, `/health/ready`) bypass authentication** — they are registered outside the middleware chain so container orchestrators can probe without credentials.
- Passwords are verified using `bcrypt.CompareHashAndPassword`, which is constant-time by design.

> [!WARNING]
> Basic Auth transmits credentials in base64 (not encrypted). Always use it behind a TLS-terminating reverse proxy (nginx, Cloudflare, etc.) or over HTTPS to prevent credential interception.

For production deployments requiring more advanced authentication (OAuth, SSO, JWT), use a reverse proxy that handles auth and forwards authenticated requests to gomddoc.

## 2. Path Traversal Protection

We use Go's `os.DirFS` (and the Git object tree for Git sources) to create a "jail" around the content directory.
It is impossible for a user to request `../../etc/passwd`. The file system provider strictly limits access to the
root directory specified by the `DIR` argument.

All request paths are normalized using forward slashes (`path.Clean`, not `filepath.Clean`) to comply with the
`io/fs` specification and prevent OS-specific separator issues on Windows.

## 3. Hidden File Blocking

gomddoc automatically blocks access to "hidden" files and directories (those starting with a dot `.`) across all entry points — both HTTP content routes and MCP tools/resources.

- **Blocked:** `.env`, `.git/`, `.gomddoc/`, `.ssh/`, `.config/`, `.DS_Store`
- **Allowed:** `/.well-known/` (Standard for SSL verification, security.txt — IETF RFC 8615)

This prevents accidental exposure of configuration files, secrets, or git history.

One predicate answers the question everywhere: `provider.IsRestrictedPath`, which is
`IsHiddenPath` (the dotfile rule above, always enforced) OR `IsExcludedPath` (your `exclude:`
patterns, documented in [Configuration](02-configuration.md)). The HTTP side calls it from the
`ContentExclusion` middleware; the MCP side calls it from `read_page`, `read_section`,
`find_related`, the `docs://` resources and the prompts. Same answer regardless of access method.

A blocked path gets a **404**, not a 403, and the body is the theme's ordinary not-found page — a
403 would confirm that the file exists, and so would a differently-worded 404.

## 4. HTTP Method Filtering

The server only responds to **GET** and **HEAD** requests. All other methods (POST, PUT, DELETE, PATCH, etc.)
receive a **405 Method Not Allowed** response with a proper `Allow: GET, HEAD` header.

This is enforced via the `MethodFilter` middleware in the request chain, before any content is processed.

## 5. Git Provider Isolation

When serving from a Git repository:

- **In-memory by default.** The repository is cloned into memory and never written to disk, preventing residual
  data. This is the default behavior when `--git-storage-dir` is not set.
- **Disk-based storage.** When `--git-storage-dir` is set, the repository is cloned to the specified directory for
  large repositories that would exceed available memory. Each repository URL gets a unique subdirectory (SHA-256
  hash of the URL). **gomddoc does not delete this data** — the cache persists across restarts by design. It is
  the operator's responsibility to clean up the storage directory when it is no longer needed (e.g., `rm -rf`
  the directory, or use a scheduled job to prune stale caches).
- SSH keys used for authentication are handled in memory by the Go process and are not accessible via the HTTP
  interface or file system operations.
- **File size limits**: Files larger than 50MB (configurable via `WithMaxFileSize`) are rejected to prevent memory
  exhaustion.
- **Git LFS**: LFS pointer files are detected and rejected with a 501 Not Implemented status.
- **Clone timeout**: A 60-second timeout (configurable via `WithCloneTimeout`) is enforced using
  `git.CloneContext()` to prevent indefinite blocking against unresponsive hosts.

## 6. SSH Host Key Verification

When connecting to Git repositories over SSH:

- Host key verification checks the `SSH_KNOWN_HOSTS` environment variable first, then falls back to
  `~/.ssh/known_hosts` (resolved via `os.UserHomeDir()`).
- **There is no TOFU (Trust On First Use) fallback.** If `known_hosts` is missing or the host is not listed,
  the connection fails with an error.
- This "fail closed" design prevents man-in-the-middle attacks where an attacker impersonates a Git server.

To add a host to known_hosts before using gomddoc:
```bash
ssh-keyscan github.com >> ~/.ssh/known_hosts
```

In CI/CD or containers without a home directory, point the env var to the system known_hosts:
```bash
export SSH_KNOWN_HOSTS=/etc/ssh/known_hosts
```

## 7. HTTP Security Headers

The server adds standard security headers to every response:

| Header                       | Value                                      | Purpose                        |
| ---------------------------- | ------------------------------------------ | ------------------------------ |
| `X-Content-Type-Options`     | `nosniff`                                  | Prevents MIME type sniffing    |
| `X-Frame-Options`            | `DENY`                                     | Prevents clickjacking          |
| `Referrer-Policy`            | `strict-origin-when-cross-origin`          | Controls referrer information  |
| `Permissions-Policy`         | `geolocation=(), microphone=(), camera=()` | Restricts browser features     |
| `Strict-Transport-Security`  | `max-age=31536000; includeSubDomains`      | HSTS (only for HTTPS requests) |

## 8. Timeouts

To protect against Slowloris attacks and resource exhaustion, the server has default timeouts:

| Setting              | Default | Max Allowed | Purpose                       |
| -------------------- | ------- | ----------- | ----------------------------- |
| **Read Header**      | 5s      | 60s         | Prevents slow header attacks  |
| **Write**            | 30s     | 5m          | Prevents hung responses       |
| **Idle**             | 120s    | 10m         | Reclaims keep-alive connections |
| **Shutdown**         | 1s      | 60s         | Graceful shutdown window      |
| **Max Header Size**  | 1 MB    | 10 MB       | Limits request header memory  |

All timeouts are validated at startup. Invalid values are reset to defaults with a warning.

## 9. Log Injection Prevention

User-controlled input (URL paths, environment variables) is sanitized before logging via the `text.SafeString`
type, which implements `slog.LogValuer`. Control characters (newlines, carriage returns, null bytes) are replaced
with their escape sequences to prevent log forging attacks.

## 10. Template Security

Only `SiteConfig` (metadata, theme) is exposed to templates — never the full `Config` with operational settings
like ports, timeouts, or SSH keys. This prevents accidental leakage of server configuration through template
rendering.

XSS protection is built into TOC generation: all heading text and IDs are escaped via `template.HTMLEscapeString`.
