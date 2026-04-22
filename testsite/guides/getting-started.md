---
title: Getting Started
description: Install and run your first Acme service
tags:
  - guides
  - installation
---

# Getting Started

This guide walks you through installing the Acme Platform and running your first microservice. By the end, you will have a working HTTP endpoint responding on port 9090.

## Prerequisites

Before you begin, ensure you have the following installed:

- **Go 1.22** or later ([download](https://go.dev/dl/))
- **Docker** (optional, for container deployment)
- **PostgreSQL 14+** (or use the bundled SQLite adapter for development)

> [!TIP]
> Use `go version` to verify your Go installation. If you see `go1.22` or higher, you are ready to proceed.

## Installation

### From Source

Clone the repository and build the binary:

```bash
git clone https://github.com/acme/platform.git
cd platform
make build
```

The compiled binary will be placed in `./build/acme`.

### Using Go Install

Alternatively, install directly with `go install`:

```bash
go install github.com/acme/platform/cmd/acme@latest
```

### Docker

Pull the official image:

```bash
docker pull ghcr.io/acme/platform:latest
docker run -p 9090:9090 ghcr.io/acme/platform:latest
```

> [!WARNING]
> The Docker image runs as a non-root user by default. If you need to bind to ports below 1024, you must configure `NET_BIND_SERVICE` capabilities.

## Your First Service

Create a new file called `main.go`:

```go
package main

import (
    "fmt"
    "log"
    "net/http"

    "github.com/acme/platform/sdk"
)

func main() {
    svc := sdk.NewService("hello-world")

    svc.Handle("/greet", func(ctx sdk.Context) error {
        name := ctx.Query("name", "World")
        return ctx.JSON(http.StatusOK, map[string]string{
            "message": fmt.Sprintf("Hello, %s!", name),
        })
    })

    log.Fatal(svc.ListenAndServe(":9090"))
}
```

Run the service:

```bash
go run main.go
```

Test it with curl:

```bash
curl http://localhost:9090/greet?name=Acme
# {"message": "Hello, Acme!"}
```

> [!IMPORTANT]
> Always call `svc.ListenAndServe` as the last statement in `main()`. The method blocks until the server shuts down or encounters a fatal error.

## Project Structure

A typical Acme project follows this layout:

```
my-service/
├── cmd/
│   └── server/
│       └── main.go          # Entry point
├── internal/
│   ├── handler/             # HTTP handlers
│   ├── model/               # Data models
│   └── store/               # Database access
├── config.yml               # Service configuration
├── go.mod
└── Makefile
```

## Configuration

Acme uses a layered configuration system. See the [Configuration Guide](configuration.md) for details.

The minimal configuration file looks like this:

```yaml
service:
  name: hello-world
  port: 9090

database:
  driver: sqlite
  dsn: ":memory:"

logging:
  level: info
  format: json
```

## Next Steps

1. Read the [Configuration Guide](configuration.md) to learn about all available options
2. Explore the [API Reference](../reference/api.md) for endpoint documentation
3. Check the [Examples](../examples/) for more complex use cases
