---
title: "Health Endpoints"
description: "Kubernetes-compatible health check endpoints for liveness and readiness probes."
author: "nicolasm"
---

# Health Endpoints

gomddoc exposes two health check endpoints that are compatible with Kubernetes liveness and readiness probes. These endpoints bypass all middleware (security headers, hidden path blocking) to ensure they are always accessible and lightweight.

## Endpoints

### Liveness: `GET /health/live`

Returns `200 OK` unconditionally. This confirms the process is running and able to handle HTTP requests.

**Response:**

```json
{"status":"ok"}
```

Use this as a Kubernetes liveness probe. If this endpoint stops responding, the container should be restarted.

### Readiness: `GET /health/ready`

Checks whether the content provider (filesystem) is accessible by calling `Stat(".")` on the root directory. Returns `200 OK` if the provider is healthy, or `503 Service Unavailable` if not.

**Healthy response (200):**

```json
{"status":"ok"}
```

**Unhealthy response (503):**

```json
{"status":"unavailable","error":"stat error message"}
```

Use this as a Kubernetes readiness probe. If this endpoint returns 503, the pod should be removed from the service load balancer until it recovers.

## Kubernetes Configuration

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

## Docker Compose Healthcheck

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

## Design Notes

- Health endpoints are registered directly on the HTTP mux, outside the middleware chain. They do not receive security headers or hidden-path blocking.
- All responses use `Content-Type: application/json`.
- The readiness check logs a warning via `slog` when the provider is unavailable, which aids in diagnosing issues without exposing internal details to external callers beyond the error message.
