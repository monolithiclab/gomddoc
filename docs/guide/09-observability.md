# Observability

gomddoc exposes Prometheus metrics out of the box so you can monitor request
traffic, latency, and error rates.

## Metrics Endpoint

A `/metrics` endpoint is served alongside the content routes. It returns
metrics in the Prometheus exposition format and can be scraped by any
Prometheus-compatible collector.

```
GET /metrics
```

## Available Metrics

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

## Middleware Order

The metrics middleware is the innermost in the chain so it measures the actual
handler processing time without including overhead from security headers or
hidden-path checks:

```
SecurityHeaders -> BlockHiddenPaths -> Metrics -> Handler
```

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

> **Warning:** pprof endpoints bypass authentication and expose internal runtime
> details. Only enable on trusted networks for debugging purposes.

---

## Grafana Dashboard

Import the queries above into a Grafana dashboard to visualize:

- Request rate by status code
- Latency percentiles (p50, p95, p99)
- Error rate percentage
- Request volume by HTTP method
