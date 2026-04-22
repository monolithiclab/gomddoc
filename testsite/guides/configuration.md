---
title: Configuration
description: All configuration options for the Acme Platform
tags:
  - guides
  - configuration
---

# Configuration

Acme Platform uses a layered configuration system. Values are resolved in the following order of precedence:

1. **Command-line flags** -- highest priority
2. **Environment variables** -- prefixed with `ACME_`
3. **Configuration file** -- `config.yml` in the working directory
4. **Built-in defaults** -- sensible values for development

## Configuration File

The default configuration file is `config.yml`. Below is a complete reference with all available options and their defaults.

### Service Settings

```yaml
service:
  name: my-service        # Service name (used in logs and metrics)
  port: 9090              # HTTP listen port
  host: 0.0.0.0           # Bind address
  read_timeout: 30s       # Maximum duration for reading request
  write_timeout: 60s      # Maximum duration for writing response
  shutdown_timeout: 15s   # Graceful shutdown deadline
```

### Database Settings

```yaml
database:
  driver: postgres                           # postgres | sqlite | mysql
  dsn: "postgres://user:pass@localhost/acme" # Connection string
  max_open_conns: 25                         # Maximum open connections
  max_idle_conns: 5                          # Maximum idle connections
  conn_max_lifetime: 5m                      # Connection maximum lifetime
```

> [!CAUTION]
> Never commit database credentials to version control. Use environment variables (`ACME_DATABASE_DSN`) or a secrets manager in production environments.

### Logging Settings

```yaml
logging:
  level: info    # debug | info | warn | error
  format: json   # json | text
  output: stdout # stdout | stderr | /path/to/file.log
```

### Cache Settings

```yaml
cache:
  driver: memory   # memory | redis
  ttl: 5m          # Default time-to-live
  max_size: 1000   # Maximum number of entries (memory driver only)
  redis:
    addr: localhost:6379
    password: ""
    db: 0
```

## Environment Variables

Every configuration key can be overridden with an environment variable. The mapping follows these rules:

- Prefix all variables with `ACME_`
- Replace dots and nesting with underscores
- Use uppercase

**Examples:**

| Config Key | Environment Variable |
|---|---|
| `service.name` | `ACME_SERVICE_NAME` |
| `service.port` | `ACME_SERVICE_PORT` |
| `database.driver` | `ACME_DATABASE_DRIVER` |
| `database.dsn` | `ACME_DATABASE_DSN` |
| `logging.level` | `ACME_LOGGING_LEVEL` |
| `cache.redis.addr` | `ACME_CACHE_REDIS_ADDR` |

```bash
export ACME_SERVICE_PORT=8080
export ACME_DATABASE_DSN="postgres://prod-user:secret@db.internal/acme"
export ACME_LOGGING_LEVEL=warn
```

## Command-Line Flags

Flags override all other configuration sources:

```bash
acme serve --port 8080 --log-level debug --db-driver sqlite
```

Run `acme serve --help` for a complete list of available flags.

## Validation

Acme validates the configuration at startup and reports all errors at once, rather than failing on the first invalid value.

> [!NOTE]
> Validation runs after all configuration layers are merged. This means a valid config file can be partially overridden by invalid environment variables, and the error will reference the final merged value.

Common validation errors:

- `service.port`: must be between 1 and 65535
- `database.driver`: must be one of `postgres`, `sqlite`, or `mysql`
- `logging.level`: must be one of `debug`, `info`, `warn`, or `error`
- `cache.ttl`: must be a valid Go duration string (e.g., `5m`, `1h30m`)

## Profiles

You can maintain multiple configuration files for different environments:

```
config.yml          # Base / development
config.staging.yml  # Staging overrides
config.prod.yml     # Production overrides
```

Load a specific profile with:

```bash
acme serve --config config.prod.yml
```

> [!TIP]
> Use `acme config dump` to print the fully resolved configuration after all layers are merged. This is useful for debugging precedence issues.
