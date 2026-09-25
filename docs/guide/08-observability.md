---
title: "Observability"
description: "Health endpoints, Prometheus metrics, and pprof profiling for monitoring gomddoc."
author: "nicolasm"
---

# Observability

gomddoc provides health endpoints, Prometheus metrics, request IDs, structured logs, and optional pprof profiling for
monitoring and debugging.

## Admin Port

By default, health, metrics, and pprof endpoints are served on the same port as your content, and `serve` logs
`admin endpoints on main port — use --admin-port for production` at startup (`preview` does not). For production
deployments, you can isolate these admin endpoints onto a dedicated port using `--admin-port` (`serve` only):

```bash
gomddoc serve --admin-port :9090 ./docs
```

Or via environment variable:

```bash
GOMDDOC_SERVER_ADMIN_PORT=:9090 gomddoc serve ./docs
```

When configured, metrics (`/metrics`) and pprof (`/debug/pprof/*`) move to the admin port and are
removed from the main port. Requesting them there falls through to the content handler, which answers with the
theme's 404 page. Setting `--admin-port` to the same value as `--port` keeps everything on the main listener.

**Health checks are the exception.** `/health/live` and `/health/ready` stay on *both* ports, because
load balancer and Kubernetes probes target the service port, not an internal admin port; moving them
would break every probe the moment you split the ports. Splitting the ports gives you:

- Expose the admin port only to your internal network or monitoring systems
- Keep the main port clean for user traffic
- Apply different firewall rules or network policies per port

> [!WARNING]
> Never expose the admin port publicly. It serves `/metrics` and the health endpoints without authentication, even
> when `--basic-auth-file` is set, and carries pprof when `--pprof` is on. Keep it on loopback or a private network,
> and block it at the firewall or with a network policy.

### Bind address

A bare port (`:9090`) binds **loopback only** — `127.0.0.1:9090`. The admin port carries heap dumps and the process
command line when `--pprof` is on, so "every interface" is the wrong default for someone who wrote just a port number.
To reach it from another host, give an explicit address:

```bash
gomddoc serve --admin-port 0.0.0.0:9090 ./docs   # all interfaces
gomddoc serve --admin-port 10.0.0.5:9090 ./docs  # one interface
```

The startup log prints the address actually bound:

```
INFO Admin port bound to loopback; set an explicit host to widen addr=127.0.0.1:9090
INFO Admin server started url=http://127.0.0.1:9090
```

### Authentication

On the admin listener, health and metrics are unauthenticated: scrapers do not carry credentials, and the loopback
default is what limits who can reach them. On the main listener, `/metrics` sits behind `--basic-auth-file` when it
is configured, like the content routes; the health endpoints are unauthenticated on both listeners.

`/debug/pprof/*` is the exception: when `--basic-auth-file` is configured, it requires those credentials on whichever
listener carries it — both ports mount pprof through the same gate. Without a credential file, `--pprof` logs a warning
and serves the endpoints open: `/debug/pprof/cmdline` returns the full command line and `/debug/pprof/heap` dumps
in-memory content.

Note that the loopback default applies to `--admin-port` only. `gomddoc serve --pprof` with no `--admin-port` keeps
pprof on the main listener, which binds every interface — so always pair `--pprof` with `--basic-auth-file`, an
`--admin-port`, or both.

---

## Health Endpoints

gomddoc exposes two health check endpoints for liveness and readiness monitoring. They skip authentication,
compression, request metrics and hidden path blocking. On the main listener they still carry the security headers
and an `X-Request-ID`. Responses are `application/json`.

### Liveness: `GET /health/live`

Returns `200 OK` unconditionally. This confirms the process is running and able to handle HTTP requests.

```json
{"status":"ok"}
```

### Readiness: `GET /health/ready`

Checks whether the content provider (filesystem or Git) is accessible by calling `Stat(".")` on the root directory.
Returns `200 OK` if the provider is healthy, or `503 Service Unavailable` if not.

**Healthy (200):**
```json
{"status":"ok"}
```

**Unhealthy (503):**
```json
{"status":"unavailable","error":"provider not ready"}
```

The underlying error is logged as `readiness check failed` at warning level, not returned to the caller.

---

## Metrics Endpoint

A `/metrics` endpoint is served on the admin listener, or alongside the content routes when there is none. It
returns metrics in the Prometheus exposition format and can be scraped by any Prometheus-compatible collector.

```
GET /metrics
```

### Application Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `http_requests_total` | Counter | `method`, `status` | Total HTTP requests, labeled by method and status code (200, 301, 304, 401, 404, 405, etc.) |
| `http_request_duration_seconds` | Histogram | `method` | Request latency in seconds. Uses default Prometheus buckets (5ms to 10s). Labeled by HTTP method |

Every route on the main listener is counted (content, tag pages, `/api/*`, `/_mcp/`, `/_assets/`, `/robots.txt`,
`/sitemap.xml`, `/feed.xml`), except `/health/*`, `/metrics` itself and `/debug/pprof/*`. Requests to the admin
listener are not counted. Metrics are process-wide and not labeled by path or language.

### Go Process Metrics

The `/metrics` endpoint also exposes the Prometheus client library's default Go runtime and process collectors,
including:

| Metric | Type | Description |
|---|---|---|
| `go_goroutines` | Gauge | Number of goroutines currently running |
| `go_threads` | Gauge | Number of OS threads created |
| `go_memstats_alloc_bytes` | Gauge | Bytes of allocated heap objects |
| `go_memstats_sys_bytes` | Gauge | Total bytes of memory obtained from the OS |
| `go_memstats_heap_inuse_bytes` | Gauge | Bytes in in-use heap spans |
| `go_memstats_heap_objects` | Gauge | Number of allocated heap objects |
| `go_gc_duration_seconds` | Summary | GC pause duration distribution |
| `process_cpu_seconds_total` | Counter | Total user and system CPU time spent |
| `process_resident_memory_bytes` | Gauge | Resident memory size (RSS) |
| `process_open_fds` | Gauge | Number of open file descriptors |

These are useful for monitoring memory usage, GC pressure, and goroutine leaks.

### Example Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: "gomddoc"
    static_configs:
      - targets: ["localhost:8080"]
```

When using `--admin-port`, point the scrape target at the admin port instead.

### Example PromQL Queries

Request rate over the last 5 minutes:

```promql
rate(http_requests_total[5m])
```

95th percentile request latency:

```promql
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
```

Error rate (non-2xx responses):

```promql
sum(rate(http_requests_total{status!~"2.."}[5m]))
  /
sum(rate(http_requests_total[5m]))
```

Memory usage:

```promql
go_memstats_alloc_bytes
```

### Middleware Order

The request chain on the main listener, outermost first:

```
SecurityHeaders → RequestID → router → Compression → Metrics → [BasicAuth] → MethodFilter → ContentExclusion → ExtensionRedirect → Handler
```

`MethodFilter`, `ContentExclusion` and `ExtensionRedirect` apply to content routes only. Because Metrics sits outside
authentication and the content checks, 401, 405, 404 (excluded paths) and 301 (extension redirects) responses are
counted, and the recorded duration includes authentication and path checks.

---

## Request IDs

Every response on the main listener carries an `X-Request-ID` header. An incoming `X-Request-ID` is echoed back when it
is 1-128 characters of letters, digits, `-` and `_`; otherwise gomddoc generates one (`{unix-microseconds}-{counter}`).
Set the header at your proxy to correlate proxy and gomddoc logs.

## Logs

gomddoc logs through Go's default `log/slog` logger: text lines on stderr, at Info level and above, in the form
`2026/09/25 17:16:09 INFO Server started url=http://localhost:8080 dir=docs dev=false`. There is no flag for JSON
output or for Debug level. Startup lines report the loaded configuration, auto-assigned ports, the listen URLs, the
admin-port warning, and each built language pipeline; `gomddoc build` ends with a `Build complete` line that counts
markdown, copied and skipped files, bytes written and elapsed time. Requests are not logged individually.

---

## pprof Profiling

gomddoc can expose Go's built-in profiling endpoints for performance analysis.
This is disabled by default and should never be enabled in production. `--pprof` exists on `serve` only.

### Enabling pprof

```bash
gomddoc serve --pprof ./docs
```

Or via environment variable:

```bash
GOMDDOC_SERVER_PPROF=true gomddoc serve ./docs
```

### Available Profiles

When enabled, the following endpoints are available at `/debug/pprof/`, on the admin listener when `--admin-port` is
set and on the main listener otherwise. Every named runtime profile is served through the index handler:

| Endpoint | Description |
|---|---|
| `/debug/pprof/` | Index page listing all profiles |
| `/debug/pprof/profile` | CPU profile (30s default, use `?seconds=N`) |
| `/debug/pprof/heap` | Heap memory allocations |
| `/debug/pprof/goroutine` | All current goroutines |
| `/debug/pprof/allocs` | Past memory allocations |
| `/debug/pprof/block`, `/debug/pprof/mutex`, `/debug/pprof/threadcreate` | Other runtime profiles |
| `/debug/pprof/cmdline` | The process command line |
| `/debug/pprof/symbol` | Symbol lookup for program counters |
| `/debug/pprof/trace` | Execution trace (use `?seconds=N`) |

### Example Usage

Capture a 10-second CPU profile:

```bash
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=10
```

Analyze heap allocations:

```bash
go tool pprof http://localhost:8080/debug/pprof/heap
```

View all goroutines in the browser:

```bash
curl http://localhost:8080/debug/pprof/goroutine?debug=1
```

> [!NOTE]
> pprof endpoints are protected by HTTP Basic Authentication when
> `--basic-auth-file` is configured. Without auth, they are open to anyone
> who can reach the listener, and gomddoc logs a warning at startup. Only
> enable them on trusted networks for debugging purposes.

