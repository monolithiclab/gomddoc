---
title: "Observability"
description: "Health endpoints, Prometheus metrics, and pprof profiling for monitoring gomddoc."
author: "nicolasm"
---

# Observability

gomddoc provides health endpoints, Prometheus metrics, and optional pprof profiling for monitoring and debugging.

## Health Endpoints

gomddoc exposes two health check endpoints compatible with Kubernetes liveness and readiness probes. These endpoints bypass all middleware (security headers, hidden path blocking) to ensure they are always accessible and lightweight.

### Liveness: `GET /health/live`

Returns `200 OK` unconditionally. This confirms the process is running and able to handle HTTP requests.

```json
{"status":"ok"}
```

Use this as a Kubernetes liveness probe. If this endpoint stops responding, the container should be restarted.

### Readiness: `GET /health/ready`

Checks whether the content provider (filesystem) is accessible by calling `Stat(".")` on the root directory. Returns `200 OK` if the provider is healthy, or `503 Service Unavailable` if not.

**Healthy (200):**
```json
{"status":"ok"}
```

**Unhealthy (503):**
```json
{"status":"unavailable","error":"stat error message"}
```

Use this as a Kubernetes readiness probe. If this endpoint returns 503, the pod should be removed from the service load balancer until it recovers.

### Kubernetes Configuration

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
    - name: gomddoc
      image: gomddoc:latest
      livenessProbe:
        httpGet:
          path: /health/live
          port: 8080
        initialDelaySeconds: 3
        periodSeconds: 10
      readinessProbe:
        httpGet:
          path: /health/ready
          port: 8080
        initialDelaySeconds: 5
        periodSeconds: 10
```

### Docker Compose Healthcheck

```yaml
services:
  gomddoc:
    image: gomddoc:latest
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health/ready"]
      interval: 30s
      timeout: 5s
      retries: 3
```

---

## Metrics Endpoint

A `/metrics` endpoint is served alongside the content routes. It returns
metrics in the Prometheus exposition format and can be scraped by any
Prometheus-compatible collector.

```
GET /metrics
```

### Available Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `http_requests_total` | Counter | `method`, `status` | Total number of HTTP requests |
| `http_request_duration_seconds` | Histogram | `method` | Request duration in seconds (default buckets) |

### Example Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: "gomddoc"
    static_configs:
      - targets: ["localhost:3000"]
```

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

### Middleware Order

The metrics middleware is the innermost in the chain so it measures the actual
handler processing time without including overhead from security headers or
hidden-path checks:

```
SecurityHeaders -> RequestID -> Compression -> MethodFilter -> BlockHiddenPaths -> Metrics -> Handler
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

---

## Grafana Dashboard

Import the queries above into a Grafana dashboard to visualize:

- Request rate by status code
- Latency percentiles (p50, p95, p99)
- Error rate percentage
- Request volume by HTTP method
