---
title: "Observability"
description: "Health endpoints, Prometheus metrics, and pprof profiling for monitoring gomddoc."
author: "nicolasm"
---

# Observability

gomddoc provides health endpoints, Prometheus metrics, and optional pprof profiling for monitoring and debugging.

## Admin Port

By default, health, metrics, and pprof endpoints are served on the same port as your content. For production
deployments, you can isolate these admin endpoints onto a dedicated port using `--admin-port`:

```bash
gomddoc serve --admin-port :9090 ./docs
```

Or via environment variable:

```bash
GOMDDOC_SERVER_ADMIN_PORT=:9090 gomddoc serve ./docs
```

When configured, health checks (`/health/live`, `/health/ready`), metrics (`/metrics`), and pprof (`/debug/pprof/*`)
are served exclusively on the admin port. They are removed from the main port entirely. This allows you to:

- Expose the admin port only to your internal network or monitoring systems
- Keep the main port clean for user traffic
- Apply different firewall rules or network policies per port

The admin port bypasses authentication — it does not require Basic Auth credentials even when `--basic-auth-file` is
configured on the main port.

---

## Health Endpoints

gomddoc exposes two health check endpoints for liveness and readiness monitoring. These endpoints bypass all
middleware (security headers, authentication, hidden path blocking) to ensure they are always accessible and
lightweight.

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
{"status":"unavailable","error":"stat error message"}
```

---

## Metrics Endpoint

A `/metrics` endpoint is served alongside the content routes. It returns
metrics in the Prometheus exposition format and can be scraped by any
Prometheus-compatible collector.

```
GET /metrics
```

### Application Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `http_requests_total` | Counter | `method`, `status` | Total HTTP requests, labeled by method (GET, HEAD) and status code (200, 304, 404, etc.) |
| `http_request_duration_seconds` | Histogram | `method` | Request latency in seconds. Uses default Prometheus buckets (5ms to 10s). Labeled by HTTP method |

### Go Process Metrics

The `/metrics` endpoint also exposes standard Go runtime metrics from the Prometheus client library:

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

The metrics middleware is the innermost in the content middleware chain, so it measures actual handler processing
time without overhead from security headers, compression, or path checks:

```
SecurityHeaders → RequestID → [Auth] → Compression → MethodFilter → ContentExclusion → ExtensionRedirect → Metrics → Handler
```

---

## pprof Profiling

gomddoc can expose Go's built-in profiling endpoints for performance analysis.
This is disabled by default and should never be enabled in production.

### Enabling pprof

```bash
gomddoc serve --pprof ./docs
```

Or via environment variable:

```bash
GOMDDOC_SERVER_PPROF=true gomddoc serve ./docs
```

### Available Profiles

When enabled, the following endpoints are available at `/debug/pprof/`:

| Endpoint | Description |
|---|---|
| `/debug/pprof/` | Index page listing all profiles |
| `/debug/pprof/profile` | CPU profile (30s default, use `?seconds=N`) |
| `/debug/pprof/heap` | Heap memory allocations |
| `/debug/pprof/goroutine` | All current goroutines |
| `/debug/pprof/allocs` | Past memory allocations |
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
> `--basic-auth-file` is configured. Without auth, they are publicly
> accessible — only enable on trusted networks for debugging purposes.

