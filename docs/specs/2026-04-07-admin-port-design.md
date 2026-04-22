# Admin Port for Metrics & Pprof

**Date:** 2026-04-07
**Status:** Approved
**Addresses:** REVIEW.md §2.2 — "Metrics and pprof endpoints share auth — no separate control"

---

## Problem

`/metrics` (Prometheus) and `/debug/pprof/*` are registered on the main HTTP port behind the same
auth as content routes. When auth is not configured, both are publicly accessible. When auth is
configured, they share the same htpasswd credentials. There is no way to isolate admin/observability
endpoints from content serving.

## Solution

Add a `--admin-port` flag that starts a second HTTP listener dedicated to admin endpoints (metrics,
pprof, health). When set, these endpoints are removed from the main port (except health, which
stays on both). When not set, current behavior is preserved with a startup warning.

## Configuration

- **Flag:** `--admin-port` (e.g., `:9090`)
- **Env:** `GOMDDOC_SERVER_ADMIN_PORT`
- **Config field:** `ServerConfig.AdminPort string`
- **Default:** empty (admin endpoints on main port)

## Routing

### When `--admin-port` is set

| Endpoint             | Main port | Admin port |
| -------------------- | --------- | ---------- |
| `/health/live`       | Yes       | Yes        |
| `/health/ready`      | Yes       | Yes        |
| `/metrics`           | No        | Yes        |
| `/debug/pprof/*`     | No        | Yes (if `--pprof`) |
| Content (`/`)        | Yes       | No         |
| API (`/api/*`)       | Yes       | No         |
| MCP (`/_mcp/`)       | Yes       | No         |
| Assets (`/_assets/`) | Yes       | No         |

### When `--admin-port` equals `--port` (explicit main port)

If the user sets `--admin-port` to the same value as `--port`, they explicitly want admin endpoints
on the main mux. In this case:
- Admin endpoints are registered on the main mux (behind main auth)
- No separate listener is started
- No startup warning is logged

### When `--admin-port` is not set (default)

Current behavior preserved: metrics and pprof on main port behind main auth.

Startup warning for `gomddoc serve` (not preview):
```
WARN admin endpoints on main port — use --admin-port for production
```

## Admin Server

### Design

- New `AdminServer` struct — thin wrapper around `http.Server`
- No middleware (no auth, no security headers, no request IDs)
- Handlers registered via a `RouteGroup` (not directly on the mux) so that an auth middleware
  can be added in the future without restructuring
- Minimal mux: health, metrics, optionally pprof
- Same `Start(ctx)`/`Shutdown(ctx)` interface pattern as `HTTPServer`

### Lifecycle

The admin server starts and stops alongside the main server in `runUntilCancelled`. Two additional
goroutines in the `errgroup`:
1. `adminServer.Start(ctx)` — blocks until shutdown or error
2. Wait for context cancellation, then `adminServer.Shutdown(ctx)`

## Code Changes

### `internal/config/config.go`

Add `AdminPort string` field to `ServerConfig` with `env:"ADMIN_PORT"`.

### `cmd/gomddoc/serve.go`

Add `--admin-port` flag:
```go
AdminPort string `name:"admin-port" default:"" env:"GOMDDOC_SERVER_ADMIN_PORT" help:"Listen address for admin endpoints (metrics, pprof, health)."`
```

Pass through `ServerSetupOptions`.

### `internal/server/server.go`

New `AdminServer` struct:
```go
type AdminServerConfig struct {
    Addr     string
    Provider provider.Provider // for health checks
    Pprof    bool
}
```

`NewAdminServer(cfg AdminServerConfig) *AdminServer` — creates mux with health, metrics, and
optionally pprof. No middleware.

In `NewHTTPServer`: when `cfg.Config.Server.AdminPort != ""` and differs from `Port`, skip
registering `/metrics` and `/debug/pprof/*` on the main mux. When `AdminPort == Port`, register
them on the main mux as today. Health endpoints always registered on main mux.

### `cmd/gomddoc/pipeline.go`

- `ServerSetupOptions`: add `AdminPort string`
- `setupResult`: add `adminServer *server.AdminServer` (nil when admin port not set)
- `setupServer`: create admin server when `AdminPort` is set
- `runUntilCancelled`: accept optional admin server, add start+shutdown goroutines to errgroup

### Startup warning

In `setupServer`, when `AdminPort` is empty and not in dev/preview mode:
```go
slog.Warn("admin endpoints on main port — use --admin-port for production")
```

Warning is suppressed when `AdminPort == Port` (explicit opt-in to main port).

## Testing

- **`server_test.go`**: verify `/metrics` and `/debug/pprof/` are not registered on main mux when
  admin port is configured
- **`admin_server_test.go`**: verify admin server serves `/metrics`, `/health/live`, `/health/ready`,
  and `/debug/pprof/` (when enabled)
- **Same-port opt-in**: verify that `--admin-port` equal to `--port` registers admin endpoints on
  main mux, does not start a second listener, and does not log a warning
- **Startup warning**: verify warning is logged when admin port is not set

## Not Included

- No admin auth (`--admin-auth-file`) — network-level isolation only
- No robots.txt or static assets on admin port
- No middleware on admin port
- No changes to `build` or `preview` commands (admin port is serve-only)
