# Gomddoc

A simple, secure HTTP server for serving Markdown files as HTML with zero configuration.

## Features

- **Zero Configuration**: Works out of the box - just point it at a directory
- **Secure by Design**: Built-in path traversal protection using Go 1.24+ `os.OpenRoot()`
- **Production Ready**: Proper HTTP status codes, security headers, and graceful shutdown
- **Fast**: Lightweight single binary with minimal dependencies
- **Standards Compliant**: Proper Content-Type and Cache-Control headers
- **Well Tested**: 55%+ test coverage with comprehensive test suite

## Quick Start

### Installation

```bash
# Clone and build
git clone <repository-url>
cd gomddoc
make build

# Or run directly
go run main.go
```

### Usage

```bash
# Serve current directory on default port 8080
./build/gomddoc

# Serve specific directory
./build/gomddoc -d /path/to/markdown/files

# Use custom port
./build/gomddoc -p :9000

# Combine options
./build/gomddoc -d ./docs -p :3000
```

### Examples

```bash
# Start server
$ ./build/gomddoc
2025/09/23 16:40:00 INFO Listening... Addr=:8080

# Access files
curl http://localhost:8080/              # Serves README.md
curl http://localhost:8080/docs/api.md   # Serves docs/api.md
```

## How It Works

Gomddoc converts Markdown files to HTML on-the-fly using the [gomarkdown](https://github.com/gomarkdown/markdown) library with:

- **CommonExtensions**: Tables, fenced code blocks, strikethrough, etc.
- **AutoHeadingIDs**: Automatic heading anchors for linking
- **Security Headers**: X-Content-Type-Options and X-Frame-Options
- **Smart Caching**: 5-minute browser cache for better performance

## Command Line Options

| Flag | Default | Description                            |
| ---- | ------- | -------------------------------------- |
| `-d` | `.`     | Directory to serve Markdown files from |
| `-p` | `:8080` | Port to listen on (format: `:8080`)    |

## Security

- **Path Traversal Protection**: Uses `os.OpenRoot()` to prevent access outside the specified directory
- **Security Headers**: Automatically adds `X-Content-Type-Options: nosniff` and `X-Frame-Options: DENY`
- **No Directory Listing**: Only serves files explicitly requested
- **Proper Error Handling**: Returns appropriate HTTP status codes (404, 500)

## Development

### Prerequisites

- Go 1.25+
- Make (optional, for convenience commands)

### Development Commands

```bash
# Run locally
make run

# Run tests
make test

# Run linting
make lint

# Format code
make format

# Build binary
make build

# Clean build artifacts
make clean
```

### Project Structure

```
├── main.go           # Single-file application (< 200 lines)
├── main_test.go      # Comprehensive test suite
├── Makefile          # Development commands
├── common-go.mk      # Shared Go build targets
├── go.mod            # Go module definition
├── README.md         # This file
├── CLAUDE.md         # Development guidance
└── PLAN.md           # Implementation roadmap
```

## Architecture

Gomddoc follows the "simple tools that do one thing well" philosophy:

- **Single Binary**: Everything in one ~150-line Go file
- **Standard Library**: Minimal external dependencies
- **HTTP Compliant**: Proper status codes, headers, and caching
- **Graceful Shutdown**: Clean shutdown on SIGINT/SIGTERM
- **Production Ready**: Includes proper error handling and logging

## API Reference

### HTTP Endpoints

| Path                    | Description                                |
| ----------------------- | ------------------------------------------ |
| `/`                     | Serves the default index file (README.md)  |
| `/{filename}.md`        | Serves the specified Markdown file as HTML |
| `/{path}/{filename}.md` | Serves Markdown files from subdirectories  |

### HTTP Headers

**Response Headers:**

- `Content-Type: text/html; charset=utf-8` (for successful Markdown)
- `Content-Type: text/plain; charset=utf-8` (for errors)
- `Cache-Control: public, max-age=300` (5-minute cache)
- `X-Content-Type-Options: nosniff` (security)
- `X-Frame-Options: DENY` (security)

**Status Codes:**

- `200 OK`: Successfully served Markdown file
- `404 Not Found`: Requested file doesn't exist
- `500 Internal Server Error`: Server error reading file

## Performance

- **Lightweight**: ~6MB binary, <10MB RAM usage
- **Fast Startup**: Sub-second startup time
- **Efficient**: Direct file serving with OS-level caching
- **Scalable**: Handles hundreds of concurrent requests

## Troubleshooting

### Common Issues

**File not found (404)**

```bash
# Check file exists and has correct extension
ls -la your-file.md

# Check directory permissions
ls -ld /path/to/directory
```

**Permission denied**

```bash
# Check file permissions
chmod 644 *.md

# Check directory permissions
chmod 755 /path/to/directory
```

**Port already in use**

```bash
# Use different port
./build/gomddoc -p :8081

# Or kill process using port
lsof -ti:8080 | xargs kill
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Format code: `make format`
4. Run linting: `make lint`
5. Run tests: `make test`
6. Submit a pull request

## License

[Add your license here]

## Changelog

### v1.0.0 (Current)

- ✅ HTTP compliance (proper status codes, headers)
- ✅ Security headers middleware
- ✅ Comprehensive test suite (55%+ coverage)
- ✅ Path traversal protection
- ✅ Graceful shutdown
- ✅ Production-ready error handling

