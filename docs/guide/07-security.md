---
title: "Security Features Guide"
description: "Overview of gomddoc's security features, including path traversal protection and hidden file blocking."
author: "nicolasm"
---

# Security

gomddoc is designed to be secure enough to be exposed to the internet. It serves plain HTTP, so run it behind a
reverse proxy (like Nginx or Cloudflare) for TLS and DDoS protection.

## 1. HTTP Basic Authentication

gomddoc supports HTTP Basic Authentication via htpasswd files with bcrypt-hashed passwords. Passwords are never passed
as CLI arguments or environment variables — they stay hashed on disk.

```bash
# Create an htpasswd file with bcrypt hashing (-B flag)
htpasswd -Bc .htpasswd admin
htpasswd -B .htpasswd viewer

# Start the server
gomddoc serve --basic-auth-file .htpasswd
# or: GOMDDOC_SERVER_BASIC_AUTH_FILE=.htpasswd gomddoc serve
```

Only bcrypt hashes (`$2y$`, `$2a$`, `$2b$`) are supported. Comments (`#`) and blank lines are allowed in the file. A
malformed line or a non-bcrypt hash stops startup with an error naming the line. `--basic-auth-file` exists on
`serve` only; `preview` and `build` have no authentication.

When enabled:

- Content pages, tag pages, `/api/*`, `/_mcp/`, `/sitemap.xml` and `/feed.xml` require valid credentials.
  Unauthenticated requests receive `401 Unauthorized` with a `WWW-Authenticate: Basic realm="gomddoc"` header
  prompting the browser to show a login dialog.
- **Unauthenticated routes:** `/health/live` and `/health/ready` (so container orchestrators can probe without
  credentials), `/robots.txt`, and `/_assets/` (theme and site static files).
- `/metrics` requires credentials when it is on the main port. On a separate `--admin-port` listener, `/metrics` and
  the health endpoints are unauthenticated; `/debug/pprof/*` requires credentials on either listener. See
  [Observability](08-observability.md#authentication). Never expose the admin port publicly: keep it on loopback or a
  private network.
- Passwords are verified using `bcrypt.CompareHashAndPassword`. An unknown username is checked against a dummy hash,
  so the response time does not reveal which usernames exist.

> [!WARNING]
> Basic Auth transmits credentials in base64 (not encrypted). Always use it behind a TLS-terminating reverse proxy
> (nginx, Cloudflare, etc.) or over HTTPS to prevent credential interception.

For production deployments requiring more advanced authentication (OAuth, SSO, JWT), use a reverse proxy that handles
auth and forwards authenticated requests to gomddoc.

## 2. Path Traversal Protection

gomddoc uses Go's `os.DirFS` (and the Git object tree for Git sources) to create a "jail" around the content
directory. A request for `../../etc/passwd` cannot reach outside it. The file system provider limits access to the
root directory given as the `DIR` argument (`GOMDDOC_SERVER_DIR`).

All request paths are normalized using forward slashes (`path.Clean`, not `filepath.Clean`) to comply with the
`io/fs` specification and prevent OS-specific separator issues on Windows.

## 3. Hidden File Blocking

gomddoc blocks access to "hidden" files and directories (those starting with a dot `.`) across all entry points:
HTTP content routes, MCP tools and resources, navigation, and `gomddoc build`.

- **Blocked:** `.env`, `.git/`, `.gomddoc/`, `.ssh/`, `.config/`, `.DS_Store`
- **Allowed:** `/.well-known/` (Standard for SSL verification, security.txt — IETF RFC 8615)

This prevents accidental exposure of configuration files, secrets, or git history.

One predicate answers the question everywhere: `provider.IsRestrictedPath`, which is
`IsHiddenPath` (the dotfile rule above, always enforced) OR `IsExcludedPath` (your `exclude:`
patterns, documented in [Configuration](02-configuration.md)). The HTTP side calls it from the
`ContentExclusion` middleware; the MCP side calls it from `read_page`, `read_section`,
`find_related`, the `docs://` resources and the prompts. Navigation and the build walk apply the same rules. Same
answer regardless of access method. `/_assets/` applies its own dotfile check and returns a plain 404.

A blocked path gets a **404**, not a 403, and the body is the theme's ordinary not-found page — a
403 would confirm that the file exists, and so would a differently-worded 404.

## 4. HTTP Method Filtering

Content routes only respond to **GET** and **HEAD** requests. All other methods (POST, PUT, DELETE, PATCH, etc.)
receive a **405 Method Not Allowed** response with an `Allow: GET, HEAD` header and an empty body.

This is enforced via the `MethodFilter` middleware in the request chain, before any content is processed. The other
routes are registered as `GET` patterns (which also match HEAD), so Go's router answers other methods with 405. The
MCP endpoint `/_mcp/` accepts POST, with request bodies capped at 1 MiB.

## 5. Git Provider Isolation

When serving from a Git repository:

- **In-memory by default.** The repository is cloned into memory and never written to disk, preventing residual
  data. This is the default behavior when `--git-storage-dir` is not set.
- **Disk-based storage.** When `--git-storage-dir` (`GOMDDOC_SERVER_GIT_STORAGE_DIR`) is set, the repository is
  cloned to the specified directory for large repositories that would exceed available memory. Each repository URL
  gets its own subdirectory, named after the first 16 hex characters of the URL's SHA-256 hash. **gomddoc does not
  delete this data**; the cache persists across restarts by design. It is the operator's responsibility to clean up
  the storage directory when it is no longer needed (e.g., `rm -rf` the directory, or use a scheduled job to prune
  stale caches).
- SSH keys (`--git-key-file`, `GOMDDOC_SERVER_GIT_SSH_KEY`) are read into memory by the Go process and are not
  accessible via the HTTP interface or file system operations.
- **File size limits**: Files larger than 50 MB are rejected to prevent memory exhaustion. The limit is fixed; no
  flag or environment variable changes it.
- **Git LFS**: LFS pointer files are detected and rejected with a 501 Not Implemented status.
- **Clone timeout**: The clone runs with a fixed 60-second timeout (`git.CloneContext()`) to prevent indefinite
  blocking against unresponsive hosts.

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

The server adds standard security headers to every response on the main listener (the admin listener sends none):

| Header                       | Value                                      | Purpose                        |
| ---------------------------- | ------------------------------------------ | ------------------------------ |
| `X-Content-Type-Options`     | `nosniff`                                  | Prevents MIME type sniffing    |
| `X-Frame-Options`            | `DENY`                                     | Prevents clickjacking          |
| `Referrer-Policy`            | `strict-origin-when-cross-origin`          | Controls referrer information  |
| `Permissions-Policy`         | `geolocation=(), microphone=(), camera=()` | Restricts browser features     |
| `Strict-Transport-Security`  | `max-age=31536000; includeSubDomains`      | HSTS (only for HTTPS requests) |

gomddoc listens on plain HTTP and does not terminate TLS, so the `Strict-Transport-Security` branch never fires on a
direct connection. Set HSTS on the TLS-terminating proxy. No `Content-Security-Policy` header is sent; set one at the
proxy if you need it (the default theme loads KaTeX and Mermaid from `cdn.jsdelivr.net` and fonts from Google Fonts,
and inlines its scripts and styles).

## 8. Timeouts

To protect against Slowloris attacks and resource exhaustion, the server has default timeouts:

| Setting              | Environment variable                      | Default | Max Allowed | Purpose                         |
| -------------------- | ----------------------------------------- | ------- | ----------- | ------------------------------- |
| **Read Header**      | `GOMDDOC_SERVER_HTTP_READ_HEADER_TIMEOUT` | 5s      | 1m          | Prevents slow header attacks    |
| **Write**            | `GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT`       | 30s     | 5m          | Prevents hung responses         |
| **Idle**             | `GOMDDOC_SERVER_HTTP_IDLE_TIMEOUT`        | 2m      | 10m         | Reclaims keep-alive connections |
| **Shutdown**         | `GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT`    | 1s      | none        | Graceful shutdown window        |
| **Max Header Size**  | `GOMDDOC_SERVER_HTTP_MAX_HEADER_MB`       | 1 MiB   | 10 MiB      | Limits request header memory    |

Values are Go durations (`10s`, `2m`) and whole MiB for the header size. They are validated at startup: a
non-positive or out-of-range read header, write, idle or header size value is reset to its default with a warning.
A negative shutdown timeout is a configuration error; one above 60s is accepted with a warning. The admin listener
uses the same values.

Other request limits: search queries (`/api/search?q=`) are truncated to 500 bytes, and MCP tool path arguments
longer than 1024 bytes are rejected.

## 9. Log Injection Prevention

User-controlled input (URL paths, remote addresses) is sanitized before logging via the `text.Safe` attribute
helper and its `text.SafeString` type, which implements `slog.LogValuer`. Control characters (newlines, carriage
returns, null bytes) are replaced with their escape sequences to prevent log forging attacks.

## 10. Template Security

Only `SiteConfig` (metadata, theme) is exposed to templates — never the full `Config` with operational settings
like ports, timeouts, or SSH keys. This prevents accidental leakage of server configuration through template
rendering.

TOC entries, navigation titles and breadcrumbs are passed to templates as data and escaped by `html/template` when
the theme renders them. Rendered markdown (`.Page.Content`) and inlined theme assets are trusted and inserted
unescaped: content and themes are author-controlled.
