package renderer

import (
	"context"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

// makeMarkdownSmall returns a ~50 byte markdown document.
func makeMarkdownSmall() []byte {
	return []byte("# Hello\n\nThis is a small markdown document.\n")
}

// makeMarkdownMedium returns a ~2KB markdown document with varied elements.
func makeMarkdownMedium() []byte {
	return []byte(`# Project Overview

This document describes the **project architecture** and its _key components_.
The system uses a ~~monolithic~~ microservices approach for better scalability.

## Getting Started

To install the project, run the following command:

` + "```bash" + `
go install github.com/example/project@latest
export GOMDDOC_PORT=8080
export GOMDDOC_THEME=default
gomddoc serve --dir ./docs
` + "```" + `

## Features

- Fast markdown rendering with syntax highlighting
- Table of contents generation from headings
- GFM table support with alignment
- Admonition blocks for notes and warnings
- Color chip rendering for hex codes like ` + "`#E91E63`" + `

### Configuration

The project supports multiple configuration formats:

1. YAML configuration files in ` + "`.gomddoc/config.yml`" + `
2. Environment variables prefixed with ` + "`GOMDDOC_`" + `
3. Command-line flags that override everything

| Option       | Default   | Description                          |
|--------------|-----------|--------------------------------------|
| port         | 8080      | HTTP server port                     |
| theme        | default   | Documentation theme name             |
| highlight    | github    | Syntax highlight theme for code      |
| color-chips  | false     | Enable inline color chip rendering   |
| log-level    | info      | Logging verbosity (debug/info/warn)  |

## Usage

Here is a simple example of how to use the [API](https://example.com/api):

` + "```go" + `
package main

import (
    "fmt"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello, World!")
    })
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        fmt.Fprintln(w, "ok")
    })
    log.Println("Starting server on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
` + "```" + `

## Contributing

Please read our [contributing guide](CONTRIBUTING.md) before submitting pull requests.
All code must pass ` + "`make lint`" + ` and ` + "`make test`" + ` before review.

> **Note**: All contributions must include tests and documentation updates.
> We use table-driven tests extensively throughout the codebase.

> [!WARNING]
> Breaking changes require a migration guide and at least two reviewers.

### Code Review Checklist

- [ ] Tests added for new functionality
- [ ] Documentation updated
- [ ] No linting errors
- [ ] Benchmarks show no regression

For more information, contact us at [support@example.com](mailto:support@example.com).
`)
}

// makeMarkdownLarge returns a ~20KB markdown document with extensive GFM features.
func makeMarkdownLarge() []byte {
	var b strings.Builder
	b.Grow(22000)

	b.WriteString(`# Comprehensive Architecture Guide

This is a large document that exercises all major markdown features including
**bold text**, _italic text_, ~~strikethrough~~, and ` + "`inline code`" + `.
It covers the full system architecture from high-level design through deployment.

## Table of Contents

- [Introduction](#introduction)
- [Architecture](#architecture)
- [Core Services](#core-services)
- [API Reference](#api-reference)
- [Database Schema](#database-schema)
- [Deployment](#deployment)
- [Monitoring](#monitoring)
- [Security](#security)
- [Troubleshooting](#troubleshooting)
- [Appendix](#appendix)

## Introduction

The system is designed around a **microservices architecture** that prioritizes
scalability, maintainability, and developer experience. Each service communicates
via gRPC for internal calls and exposes REST endpoints for external consumers.
The architecture follows domain-driven design principles with bounded contexts
that map cleanly to individual services.

> **Important**: This architecture requires Go 1.25+ and uses modern concurrency
> patterns including structured concurrency and error groups.

> [!NOTE]
> This is an admonition block that should be transformed by the post-processor.
> It contains important information about system requirements.

> [!WARNING]
> Be careful when modifying the core pipeline. Changes affect all downstream
> services and require coordinated deployment across the cluster.

> [!TIP]
> Use the development docker-compose setup for local testing before deploying
> to the staging environment. This catches most integration issues early.

### Design Principles

The architecture is guided by several core principles that inform every decision:

1. **Simplicity over cleverness** — Code should be readable by any team member
2. **Explicit over implicit** — No magic, no hidden behavior
3. **Composition over inheritance** — Use interfaces and embedding
4. **Fail fast, fail loudly** — Return errors early with context
5. **Measure before optimizing** — Profile first, then fix bottlenecks

`)

	// Generate code blocks in multiple languages
	languages := []struct {
		name, ext, sample string
	}{
		{"Go", "go", `package main

import (
    "context"
    "fmt"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "time"

    "golang.org/x/sync/errgroup"
)

type Server struct {
    httpServer *http.Server
    logger     *slog.Logger
}

func NewServer(addr string, logger *slog.Logger) *Server {
    mux := http.NewServeMux()
    srv := &Server{
        httpServer: &http.Server{Addr: addr, Handler: mux},
        logger:     logger,
    }
    mux.HandleFunc("GET /health", srv.handleHealth)
    mux.HandleFunc("GET /api/v1/docs", srv.handleListDocs)
    mux.HandleFunc("GET /api/v1/docs/{id}", srv.handleGetDoc)
    return srv
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    fmt.Fprintln(w, "ok")
}

func (s *Server) handleListDocs(w http.ResponseWriter, r *http.Request) {
    s.logger.Info("listing documents", "remote", r.RemoteAddr)
    w.Header().Set("Content-Type", "application/json")
    fmt.Fprintln(w, "[]")
}

func (s *Server) handleGetDoc(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    s.logger.Info("getting document", "id", id)
    http.NotFound(w, r)
}

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()

    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    slog.SetDefault(logger)

    srv := NewServer(":8080", logger)

    g, gCtx := errgroup.WithContext(ctx)
    g.Go(func() error {
        logger.Info("starting server", "addr", ":8080")
        return srv.httpServer.ListenAndServe()
    })
    g.Go(func() error {
        <-gCtx.Done()
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        return srv.httpServer.Shutdown(shutdownCtx)
    })

    if err := g.Wait(); err != nil && err != http.ErrServerClosed {
        logger.Error("server error", "err", err)
        os.Exit(1)
    }
}`},
		{"Python", "python", `from fastapi import FastAPI, HTTPException, Depends
from pydantic import BaseModel, Field
from typing import Optional, List
from datetime import datetime
import asyncio
import logging
import uuid

logger = logging.getLogger(__name__)
app = FastAPI(title="Document API", version="1.0.0")

class Document(BaseModel):
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    title: str = Field(..., min_length=1, max_length=255)
    content: str
    author: str
    tags: List[str] = []
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: Optional[datetime] = None

class DocumentUpdate(BaseModel):
    title: Optional[str] = None
    content: Optional[str] = None
    tags: Optional[List[str]] = None

# In-memory store for demonstration
documents: dict[str, Document] = {}

@app.get("/api/v1/docs", response_model=List[Document])
async def list_documents(skip: int = 0, limit: int = 20):
    docs = list(documents.values())
    return docs[skip:skip + limit]

@app.get("/api/v1/docs/{doc_id}", response_model=Document)
async def get_document(doc_id: str):
    if doc_id not in documents:
        raise HTTPException(status_code=404, detail="Document not found")
    return documents[doc_id]

@app.post("/api/v1/docs", response_model=Document, status_code=201)
async def create_document(doc: Document):
    documents[doc.id] = doc
    logger.info(f"Created document {doc.id}: {doc.title}")
    return doc

@app.put("/api/v1/docs/{doc_id}", response_model=Document)
async def update_document(doc_id: str, update: DocumentUpdate):
    if doc_id not in documents:
        raise HTTPException(status_code=404, detail="Document not found")
    doc = documents[doc_id]
    update_data = update.dict(exclude_unset=True)
    update_data["updated_at"] = datetime.utcnow()
    updated = doc.copy(update=update_data)
    documents[doc_id] = updated
    return updated

@app.delete("/api/v1/docs/{doc_id}", status_code=204)
async def delete_document(doc_id: str):
    if doc_id not in documents:
        raise HTTPException(status_code=404, detail="Document not found")
    del documents[doc_id]`},
		{"SQL", "sql", `-- Schema for the document management system
CREATE SCHEMA IF NOT EXISTS docs;

CREATE TABLE IF NOT EXISTS docs.users (
    id          BIGSERIAL PRIMARY KEY,
    username    VARCHAR(64) UNIQUE NOT NULL,
    email       VARCHAR(255) UNIQUE NOT NULL,
    full_name   VARCHAR(255),
    role        VARCHAR(32) NOT NULL DEFAULT 'viewer',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS docs.documents (
    id          BIGSERIAL PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    slug        VARCHAR(255) UNIQUE NOT NULL,
    content     TEXT NOT NULL,
    author_id   BIGINT REFERENCES docs.users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    version     INTEGER NOT NULL DEFAULT 1,
    metadata    JSONB DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS docs.tags (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(64) UNIQUE NOT NULL,
    color       VARCHAR(7) DEFAULT '#6B7280'
);

CREATE TABLE IF NOT EXISTS docs.document_tags (
    document_id BIGINT REFERENCES docs.documents(id) ON DELETE CASCADE,
    tag_id      INTEGER REFERENCES docs.tags(id) ON DELETE CASCADE,
    PRIMARY KEY (document_id, tag_id)
);

-- Indexes for common query patterns
CREATE INDEX idx_documents_author ON docs.documents(author_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_created ON docs.documents(created_at DESC);
CREATE INDEX idx_documents_slug ON docs.documents(slug) WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_metadata ON docs.documents USING GIN(metadata);

-- Full-text search
ALTER TABLE docs.documents ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(content, '')), 'B')
    ) STORED;

CREATE INDEX idx_documents_search ON docs.documents USING GIN(search_vector);

-- Audit trail
CREATE TABLE IF NOT EXISTS docs.audit_log (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT REFERENCES docs.users(id),
    action      VARCHAR(32) NOT NULL,
    entity_type VARCHAR(32) NOT NULL,
    entity_id   BIGINT NOT NULL,
    changes     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_entity ON docs.audit_log(entity_type, entity_id);`},
		{"YAML", "yaml", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: gomddoc
  namespace: documentation
  labels:
    app: gomddoc
    version: v1.0.0
    team: platform
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: gomddoc
  template:
    metadata:
      labels:
        app: gomddoc
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
    spec:
      serviceAccountName: gomddoc
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
      containers:
        - name: gomddoc
          image: ghcr.io/example/gomddoc:latest
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
            - name: metrics
              containerPort: 9090
              protocol: TCP
          env:
            - name: GOMDDOC_THEME
              value: "default"
            - name: GOMDDOC_PORT
              value: "8080"
            - name: GOMDDOC_LOG_LEVEL
              value: "info"
          resources:
            requests:
              memory: "64Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: http
            initialDelaySeconds: 3
            periodSeconds: 5
          volumeMounts:
            - name: docs
              mountPath: /data/docs
              readOnly: true
      volumes:
        - name: docs
          persistentVolumeClaim:
            claimName: gomddoc-docs`},
		{"JavaScript", "javascript", `import express from 'express';
import cors from 'cors';
import helmet from 'helmet';
import rateLimit from 'express-rate-limit';

const app = express();
const port = process.env.PORT || 3000;

// Middleware
app.use(helmet());
app.use(cors({ origin: process.env.ALLOWED_ORIGINS?.split(',') }));
app.use(express.json({ limit: '10mb' }));

// Rate limiting
const limiter = rateLimit({
    windowMs: 15 * 60 * 1000, // 15 minutes
    max: 100,
    standardHeaders: true,
    legacyHeaders: false,
});
app.use('/api/', limiter);

// Health check
app.get('/health', (req, res) => {
    res.json({ status: 'ok', timestamp: new Date().toISOString() });
});

// Document routes
app.get('/api/v1/docs', async (req, res) => {
    const { page = 1, limit = 20, tag } = req.query;
    try {
        const docs = await fetchDocuments({ page, limit, tag });
        res.json({ data: docs, page, limit });
    } catch (err) {
        console.error('Failed to fetch documents:', err);
        res.status(500).json({ error: 'Internal server error' });
    }
});

app.listen(port, () => {
    console.log("Server listening on port " + port);
});`},
	}

	for _, lang := range languages {
		b.WriteString("### " + lang.name + " Service Implementation\n\n")
		b.WriteString("The following " + lang.name + " code demonstrates the recommended pattern for this service.\n")
		b.WriteString("Pay attention to error handling and graceful shutdown behavior.\n\n")
		b.WriteString("```" + lang.ext + "\n")
		b.WriteString(lang.sample)
		b.WriteString("\n```\n\n")
	}

	// GFM tables
	b.WriteString(`## API Reference

### Endpoints

| Method | Path                  | Description                    | Auth | Rate Limit |
|--------|-----------------------|--------------------------------|------|------------|
| GET    | /api/v1/docs          | List all documents             | Yes  | 100/min    |
| GET    | /api/v1/docs/:id      | Get a single document by ID    | Yes  | 200/min    |
| POST   | /api/v1/docs          | Create a new document          | Yes  | 50/min     |
| PUT    | /api/v1/docs/:id      | Update an existing document    | Yes  | 50/min     |
| PATCH  | /api/v1/docs/:id      | Partial update a document      | Yes  | 50/min     |
| DELETE | /api/v1/docs/:id      | Soft-delete a document         | Yes  | 20/min     |
| GET    | /api/v1/search        | Full-text search documents     | Yes  | 60/min     |
| GET    | /api/v1/tags          | List all tags                  | No   | 200/min    |
| GET    | /api/v1/tags/:id/docs | List documents by tag          | No   | 100/min    |
| GET    | /api/v1/health        | Health check endpoint          | No   | unlimited  |
| GET    | /api/v1/metrics       | Prometheus metrics             | No   | unlimited  |

### Response Codes

| Code | Meaning                | When                                    |
|------|------------------------|-----------------------------------------|
| 200  | OK                     | Successful read or update               |
| 201  | Created                | Successful resource creation            |
| 204  | No Content             | Successful deletion                     |
| 301  | Moved Permanently      | Resource URL has changed                |
| 304  | Not Modified           | ETag matches, use cached version        |
| 400  | Bad Request            | Invalid input or malformed JSON         |
| 401  | Unauthorized           | Missing or expired authentication token |
| 403  | Forbidden              | Insufficient permissions for action     |
| 404  | Not Found              | Resource does not exist                 |
| 409  | Conflict               | Version conflict during update          |
| 422  | Unprocessable Entity   | Valid JSON but semantic errors          |
| 429  | Too Many Requests      | Rate limit exceeded                     |
| 500  | Internal Server Error  | Unexpected server failure               |
| 503  | Service Unavailable    | Server temporarily overloaded           |

### Authentication

All authenticated endpoints require a Bearer token in the Authorization header:

`)

	b.WriteString("```http\nGET /api/v1/docs HTTP/1.1\nHost: docs.example.com\nAuthorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...\nAccept: application/json\n```\n\n")

	// Deployment section with nested lists
	b.WriteString(`## Deployment

### Prerequisites

- **Runtime Requirements**
    - Go 1.25 or later
    - Docker 24.0+ with BuildKit enabled
    - Kubernetes 1.28+ (for cluster deployment)
        - Helm 3.12+ for chart management
        - kubectl configured with cluster access
        - Ingress controller (nginx or traefik)
    - PostgreSQL 16+ for metadata storage
- **Build Requirements**
    - GNU Make 4.0+
    - Git 2.40+
    - golangci-lint v1.55+
    - gosec v2.18+
- **Optional Tools**
    - Air for hot-reloading during development
    - k9s for Kubernetes cluster management
    - pgcli for database interaction

### Step-by-Step Guide

1. Clone the repository and set up the workspace
2. Configure environment variables:
    - ` + "`GOMDDOC_PORT`" + `: Server port (default: 8080)
    - ` + "`GOMDDOC_THEME`" + `: Documentation theme
    - ` + "`GOMDDOC_LOG_LEVEL`" + `: Logging level (debug, info, warn, error)
    - ` + "`GOMDDOC_METRICS_PORT`" + `: Prometheus metrics port (default: 9090)
    - ` + "`GOMDDOC_GIT_URL`" + `: Git repository URL for content
3. Build the binary
    1. Run ` + "`make build`" + ` to compile the binary
    2. Verify the output exists in ` + "`build/gomddoc`" + `
    3. Run smoke tests with ` + "`make test`" + `
    4. Run benchmarks with ` + "`make bench`" + `
4. Deploy to staging
    1. Build and push the Docker image
    2. Apply Kubernetes manifests to staging namespace
    3. Verify health endpoints return 200
    4. Run integration tests against staging
5. Deploy to production
    1. Create a release tag
    2. Push the Docker image with release tag
    3. Apply Kubernetes manifests to production namespace
    4. Monitor error rates for 30 minutes
    5. Verify all health endpoints

> **Best Practice**: Always run the full test suite before deploying.
> Use ` + "`make ci`" + ` to run the complete CI pipeline locally.
>
> > **Nested Quote**: For critical deployments, also run load tests
> > using the benchmark suite to verify performance under pressure.
> > Target p99 latency should remain under 50ms for document rendering.

`)

	// Monitoring section
	b.WriteString(`## Monitoring

### Key Metrics

The application exposes Prometheus metrics at ` + "`/metrics`" + `:

| Metric Name                          | Type      | Description                        |
|--------------------------------------|-----------|------------------------------------|
| gomddoc_http_requests_total          | Counter   | Total HTTP requests by status code |
| gomddoc_http_request_duration_seconds| Histogram | Request latency distribution       |
| gomddoc_render_duration_seconds      | Histogram | Markdown render time               |
| gomddoc_cache_hits_total             | Counter   | Template cache hit count           |
| gomddoc_cache_misses_total           | Counter   | Template cache miss count          |
| gomddoc_git_fetch_duration_seconds   | Histogram | Git fetch operation time           |
| gomddoc_active_connections           | Gauge     | Current active HTTP connections    |

### Alerting Rules

Configure the following alerts in your monitoring system:

- **High error rate**: > 1% of requests returning 5xx over 5 minutes
- **High latency**: p99 latency > 100ms over 5 minutes
- **Cache degradation**: Cache hit rate < 80% over 15 minutes
- **Git fetch failures**: > 3 consecutive fetch failures

`)

	// Security section
	b.WriteString(`## Security

### Authentication Flow

The system uses **JWT tokens** with RS256 signing for authentication.
Tokens are issued by the identity provider and validated on each request.

### Headers

All responses include the following security headers:

| Header                    | Value                          | Purpose                    |
|---------------------------|--------------------------------|----------------------------|
| X-Content-Type-Options    | nosniff                        | Prevent MIME sniffing      |
| X-Frame-Options           | DENY                           | Prevent clickjacking       |
| X-XSS-Protection          | 1; mode=block                  | XSS filter                 |
| Content-Security-Policy   | default-src 'self'             | CSP enforcement            |
| Strict-Transport-Security | max-age=31536000               | HSTS enforcement           |
| Referrer-Policy           | strict-origin-when-cross-origin| Referrer control           |

`)

	// Task list (GFM extension)
	b.WriteString(`## Release Checklist

- [x] Update version in go.mod
- [x] Run full test suite with ` + "`make test`" + `
- [x] Run benchmarks with ` + "`make bench`" + `
- [x] Update CHANGELOG.md with release notes
- [x] Run security scan with ` + "`gosec`" + `
- [ ] Create release tag following semver
- [ ] Build release binaries for all platforms
- [ ] Publish Docker image to registry
- [ ] Update documentation site
- [ ] Update Helm chart version
- [ ] Send release announcement to team
- [ ] Monitor error rates post-deployment

`)

	// Troubleshooting with mixed formatting
	b.WriteString(`## Troubleshooting

### Common Issues

**Problem**: Server fails to start with "port already in use" error.

*Solution*: Check for other processes using the port:

` + "```bash" + `
lsof -i :8080
kill -9 <PID>
# Or use a different port
GOMDDOC_PORT=9080 gomddoc serve
` + "```" + `

---

**Problem**: Markdown rendering produces unexpected output.

*Solution*: Verify your markdown follows [GFM specification](https://github.github.com/gfm/).
Common issues include:

1. Missing blank lines before lists
2. Incorrect indentation in nested blocks
3. Unescaped special characters: ` + "`*`" + `, ` + "`_`" + `, ` + "`` ` ``" + `, ` + "`#`" + `
4. Mixed tab and space indentation
5. Missing language identifier on fenced code blocks

---

**Problem**: Syntax highlighting not working for a specific language.

*Solution*: Check that the language identifier in your fenced code block matches
a [supported Chroma lexer](https://github.com/alecthomas/chroma#supported-languages).

| Language   | Identifier  | Aliases          |
|------------|-------------|------------------|
| Go         | go          | golang           |
| Python     | python      | py, python3      |
| JavaScript | javascript  | js, node         |
| TypeScript | typescript  | ts               |
| Rust       | rust        | rs               |
| SQL        | sql         | mysql, postgres  |
| Shell      | bash        | sh, zsh          |
| YAML       | yaml        | yml              |
| JSON       | json        | jsonc            |
| HTML       | html        | htm              |

---

**Problem**: Git provider fails to clone the repository.

*Solution*: Verify the repository URL and credentials:

` + "```bash" + `
# Test SSH access
ssh -T git@github.com

# Test HTTPS access
git ls-remote https://github.com/example/docs.git

# Check environment variable
echo $GOMDDOC_GIT_URL
` + "```" + `

---

**Problem**: High memory usage under load.

*Solution*: Check the following potential causes:

1. **Template cache size** — Large sites may need cache limits
2. **Concurrent requests** — Limit with reverse proxy
3. **Large documents** — Consider splitting into smaller pages
4. **Memory leaks** — Profile with ` + "`go tool pprof`" + `

` + "```bash" + `
# Profile memory usage
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof -http=:6060 heap.prof
` + "```" + `

## Appendix

### Glossary

| Term          | Definition                                              |
|---------------|---------------------------------------------------------|
| GFM           | GitHub Flavored Markdown, an extension of CommonMark    |
| Enrichment    | Pre-render extraction of metadata, TOC, and related docs|
| Provider      | Content source abstraction (filesystem or git)          |
| Renderer      | Component that transforms input to output MIME type     |
| Admonition    | Callout block for notes, warnings, tips                 |
| Color Chip    | Inline visual swatch for hex color codes                |
| ETag          | HTTP header for content-hash-based cache validation     |
| Frontmatter   | YAML metadata block at the top of markdown files        |

### Version History

| Version | Date       | Changes                                    |
|---------|------------|--------------------------------------------|
| 1.0.0   | 2026-03-01 | Initial release with core features         |
| 1.1.0   | 2026-03-15 | Added git provider and syntax highlighting |
| 1.2.0   | 2026-03-24 | Performance benchmarks and optimization    |

---

*This document is auto-generated. Last updated: 2026-03-24*
`)

	return []byte(b.String())
}

func BenchmarkMarkdownRender(b *testing.B) {
	tests := []struct {
		name    string
		content func() []byte
	}{
		{"Small", makeMarkdownSmall},
		{"Medium", makeMarkdownMedium},
		{"Large", makeMarkdownLarge},
	}

	r := NewMarkdownRenderer(MarkdownOptions{
		HighlightTheme: "github",
		ColorChips:     true,
	})
	ctx := context.Background()
	enrichment := &enricher.EnrichmentData{}

	for _, tt := range tests {
		content := tt.content()
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(content)))
			for b.Loop() {
				_, err := r.Render(ctx, content, enrichment)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
