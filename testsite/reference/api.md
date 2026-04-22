---
title: API Reference
description: REST API endpoints and data models
tags:
  - reference
  - api
---

# API Reference

The Acme Platform exposes a RESTful API over HTTP. All endpoints accept and return JSON. Authentication is required for all endpoints except health checks.

## Base URL

```
https://api.acme.example.com/v1
```

## Authentication

Include a Bearer token in the `Authorization` header:

```bash
curl -H "Authorization: Bearer eyJhbGciOi..." \
     https://api.acme.example.com/v1/services
```

> [!WARNING]
> API tokens expire after 24 hours. Use the `/auth/refresh` endpoint to obtain a new token before expiry. Requests with expired tokens receive a `401 Unauthorized` response.

## Endpoints

### List Services

```
GET /v1/services
```

Returns a paginated list of all registered services.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `per_page` | integer | 20 | Items per page (max 100) |
| `status` | string | all | Filter by status: `active`, `inactive`, `all` |
| `sort` | string | name | Sort field: `name`, `created_at`, `updated_at` |

**Response:**

```json
{
  "data": [
    {
      "id": "svc_abc123",
      "name": "payment-service",
      "status": "active",
      "endpoint": "https://payment.internal:9090",
      "created_at": "2025-01-15T10:30:00Z",
      "updated_at": "2025-03-01T14:22:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 42
  }
}
```

### Get Service

```
GET /v1/services/:id
```

Returns details for a single service.

**Response:** `200 OK`

```json
{
  "id": "svc_abc123",
  "name": "payment-service",
  "status": "active",
  "endpoint": "https://payment.internal:9090",
  "config": {
    "replicas": 3,
    "memory_limit": "512Mi",
    "cpu_limit": "500m"
  },
  "health": {
    "status": "healthy",
    "last_check": "2025-03-18T12:00:00Z",
    "uptime": "72h15m"
  },
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-03-01T14:22:00Z"
}
```

### Create Service

```
POST /v1/services
```

Registers a new service.

**Request Body:**

```json
{
  "name": "notification-service",
  "endpoint": "https://notify.internal:9090",
  "config": {
    "replicas": 2,
    "memory_limit": "256Mi"
  }
}
```

**Response:** `201 Created`

> [!NOTE]
> Service names must be unique within a namespace. Names must match the pattern `^[a-z][a-z0-9-]{2,62}$` (lowercase, alphanumeric with hyphens, 3-63 characters).

### Delete Service

```
DELETE /v1/services/:id
```

Removes a service registration. This does **not** stop the running service -- it only removes it from the registry.

**Response:** `204 No Content`

> [!CAUTION]
> Deleting a service removes all associated metrics and health check history. This action cannot be undone.

## Error Responses

All errors follow a consistent format:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Service name is required",
    "details": [
      {
        "field": "name",
        "reason": "must not be empty"
      }
    ]
  }
}
```

### Status Codes

| Code | Meaning |
|------|---------|
| `200` | Success |
| `201` | Created |
| `204` | No Content (successful deletion) |
| `400` | Bad Request (validation error) |
| `401` | Unauthorized (missing or invalid token) |
| `403` | Forbidden (insufficient permissions) |
| `404` | Not Found |
| `409` | Conflict (duplicate resource) |
| `429` | Too Many Requests (rate limited) |
| `500` | Internal Server Error |

## Rate Limiting

The API enforces rate limits per API token:

- **Standard tier:** 100 requests per minute
- **Premium tier:** 1000 requests per minute

Rate limit headers are included in every response:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 87
X-RateLimit-Reset: 1710763200
```

> [!IMPORTANT]
> When you receive a `429` response, back off and retry after the time indicated in the `X-RateLimit-Reset` header (Unix timestamp). Continuing to send requests will extend the cooldown period.
