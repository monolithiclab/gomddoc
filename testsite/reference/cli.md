---
title: CLI Reference
description: Command-line interface reference
tags:
  - reference
  - cli
---

# CLI Reference

The `acme` command-line tool provides commands for managing services, inspecting configuration, and running the platform locally.

## Installation

```bash
go install github.com/acme/platform/cmd/acme@latest
```

Verify the installation:

```bash
acme version
# acme v3.2.0 (go1.24.1, linux/amd64)
```

## Global Flags

These flags are available on all commands:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | `config.yml` | Path to configuration file |
| `--log-level` | `-l` | `info` | Log level: debug, info, warn, error |
| `--no-color` | | `false` | Disable colored output |
| `--quiet` | `-q` | `false` | Suppress non-error output |

## Commands

### `acme serve`

Start the HTTP server.

```bash
acme serve [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `9090` | HTTP listen port |
| `--host` | `0.0.0.0` | Bind address |
| `--dev` | `false` | Enable development mode |
| `--db-driver` | `postgres` | Database driver |

**Examples:**

```bash
# Start with defaults
acme serve

# Development mode with SQLite
acme serve --dev --db-driver sqlite

# Custom port and log level
acme serve --port 8080 -l debug
```

> [!TIP]
> In development mode (`--dev`), the server enables hot-reloading, verbose logging, and uses an in-memory SQLite database by default.

### `acme migrate`

Run database migrations.

```bash
acme migrate [up|down|status] [flags]
```

**Subcommands:**

- `up` -- apply all pending migrations
- `down` -- roll back the last migration
- `status` -- show migration status

```bash
# Apply all pending migrations
acme migrate up

# Roll back the last migration
acme migrate down

# Show migration status
acme migrate status
# +----+---------------------------+--------+---------------------+
# | ID | Name                      | Status | Applied At          |
# +----+---------------------------+--------+---------------------+
# |  1 | create_services_table     | done   | 2025-01-15 10:30:00 |
# |  2 | add_health_checks        | done   | 2025-02-01 09:15:00 |
# |  3 | add_service_metrics       | pending|                     |
# +----+---------------------------+--------+---------------------+
```

> [!WARNING]
> Running `acme migrate down` in production is destructive. Always create a database backup before rolling back migrations.

### `acme config`

Inspect and validate configuration.

```bash
acme config [dump|validate] [flags]
```

**Subcommands:**

- `dump` -- print the fully resolved configuration
- `validate` -- check configuration for errors

```bash
# Dump resolved config as YAML
acme config dump

# Validate without starting the server
acme config validate
# Configuration is valid.
```

### `acme health`

Check service health.

```bash
acme health [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | `http://localhost:9090` | Service URL to check |
| `--timeout` | `5s` | Request timeout |

```bash
acme health --url https://api.acme.example.com
# Status: healthy
# Uptime: 72h15m
# Database: connected
# Cache: connected
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error |
| `2` | Configuration error |
| `3` | Database connection error |
| `4` | Migration error |

## Shell Completion

Generate shell completion scripts:

```bash
# Bash
acme completion bash > /etc/bash_completion.d/acme

# Zsh
acme completion zsh > "${fpath[1]}/_acme"

# Fish
acme completion fish > ~/.config/fish/completions/acme.fish
```
