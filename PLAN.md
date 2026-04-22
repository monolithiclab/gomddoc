# PLAN

Transform gomddoc from a single-file Markdown server into a comprehensive static site generator and content management platform.

# Architecture Overview

## Core Principles

- **Interface-driven design**: All major components implement well-defined interfaces
- **Plugin architecture**: Extensible through a robust plugin system
- **Performance-first**: Built for speed with intelligent caching and optimization
- **Security by design**: Path traversal protection, content sanitization, secure defaults
- **Developer experience**: Hot reload, comprehensive CLI, debugging tools

## System Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Content       │    │   Processing    │    │   Output        │
│   Providers     │───▶│   Pipeline      │───▶│   Generators    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
│                      │                      │
├─ Filesystem          ├─ Markdown            ├─ Static HTML
├─ Database            ├─ AsciiDoc            ├─ API JSON
├─ GitHub              ├─ ReStructuredText    ├─ Progressive PWA
└─ Custom              └─ Custom              └─ Embedded Binary
```

# Feature Specifications

## 1. Content Infrastructure

### 1.1 Document Providers

**Interface Definition:**

```go
type DocumentProvider interface {
    List(ctx context.Context, filters ListFilters) ([]Document, error)
    Get(ctx context.Context, path string) (*Document, error)
    Watch(ctx context.Context) (<-chan DocumentEvent, error)
    Metadata(ctx context.Context, path string) (*Metadata, error)
}
```

**Filesystem Provider** (Primary)

- Recursive directory scanning with configurable depth limits
- Real-time file watching via `fsnotify` with debouncing
- Symbolic link support with cycle detection
- `.gomddocignore` support (gitignore syntax)
- Concurrent processing with worker pools
- Content indexing and modification tracking

**Database Provider** (PostgreSQL/SQLite)

- Schema: `documents(id, path, content, metadata_json, created_at, updated_at, published_at)`
- Full-text search via PostgreSQL `tsvector` or SQLite FTS5
- ACID transactions and connection pooling
- Migration system for schema updates

**GitHub Provider**

- GitHub API v4 (GraphQL) integration
- Webhook support for real-time updates
- Private repository support with secure authentication
- Rate limiting with exponential backoff
- Local caching with Git LFS support

**Multi-Provider Aggregation**

- Priority-based content resolution
- Content deduplication via SHA-256 hashing
- Unified search across all providers
- Provider health monitoring and failover

### 1.2 Document Processing

**Interface Definition:**

```go
type DocumentProcessor interface {
    CanProcess(filename string) bool
    Process(ctx context.Context, content []byte, metadata *Metadata) (*ProcessedDocument, error)
    Extensions() []string
    ContentType() string
}
```

**Markdown Processor** (Primary)

- Extended CommonMark with GitHub Flavored Markdown
- Custom extensions: tables, task lists, footnotes, strikethrough
- Math rendering via KaTeX integration
- Mermaid diagram support
- Syntax highlighting via Chroma (200+ languages)
- Custom shortcodes: `{{< youtube "id" >}}`, `{{< tweet "id" >}}`
- Auto-generated table of contents
- Reading time estimation

**AsciiDoc Processor**

- Full AsciiDoc specification compliance
- Cross-references and conditional content
- Bibliography and citation management
- PlantUML and Graphviz diagram integration

**ReStructuredText Processor**

- Python docutils integration
- Sphinx-compatible directives
- API documentation extraction
- Cross-document linking

**Jupyter Notebook Processor**

- `.ipynb` parsing and rendering
- Code execution results display
- Math and plot rendering
- Export to static HTML

**Plugin Architecture**

- Dynamic processor loading via Go plugins
- Processor priority chains and fallbacks
- Sandboxed execution environment
- Hot-reloading in development mode

### 1.3 Asset Management

**Theme System Architecture:**

```
themes/
├── {theme-name}/
│   ├── templates/
│   │   ├── layout.html.tmpl
│   │   ├── post.html.tmpl
│   │   ├── list.html.tmpl
│   │   └── 404.html.tmpl
│   ├── static/
│   │   ├── css/
│   │   ├── js/
│   │   └── images/
│   └── theme.yaml
```

**Asset Override System**

- Priority order: `.gomddoc/assets/` → embedded `assets/`
- Hot-swappable themes in development mode
- Asset fingerprinting for cache busting
- Conditional loading based on page type

**Static File Serving**

- MIME type detection with appropriate headers
- Gzip/Brotli compression for text assets
- ETags and Last-Modified headers
- Range request support
- Security headers: CSP, CORS configuration

## 2. Content Management

### 2.1 Document Metadata

**Frontmatter Support**

- YAML frontmatter (primary): `---` delimited
- TOML frontmatter: `+++` delimited
- JSON frontmatter: `{}` delimited
- Validation schemas for consistency
- Custom field definitions per document type

**Core Metadata Schema**

```yaml
# Essential fields
title: string
description: string
author: string | []string
created_at: timestamp
updated_at: timestamp
published_at: timestamp
status: draft | published | archived | private
slug: string
canonical_url: string

# Taxonomic fields
tags: []string
categories: []string
series: string
series_order: int
difficulty: beginner | intermediate | advanced
language: string
translations: []string

# SEO fields
meta_description: string
keywords: []string
robots: string
og_title: string
og_description: string
og_image: string
twitter_card: summary | summary_large_image
schema_type: string
```

**Metadata Processing**

- Automatic extraction from content
- Validation with custom rules
- Default value assignment
- Inheritance from parent directories
- Conflict resolution strategies

### 2.2 URL Management

**Interface Definition:**

```go
type URLRouter interface {
    AddRoute(pattern string, handler RouteHandler) error
    Match(path string) (*RouteMatch, error)
    Generate(document *Document) (string, error)
    ListRoutes() []Route
}
```

**URL Rewriting Engine**

- Automatic `.md` extension removal
- Configurable patterns via regex
- Path normalization and case handling
- Unicode URL support
- Conflict detection and resolution

**Pretty URL Patterns**

- Hierarchical: `/category/subcategory/post`
- Date-based: `/2024/01/15/post-title`
- Author-based: `/authors/john-doe/post-title`
- Custom patterns: `/blog/{year}/{month}/{slug}`

**URL Aliases and Redirects**

- Multiple aliases via metadata: `aliases: ["/old-path"]`
- Automatic redirect generation (301/302)
- Redirect chain prevention
- Bulk migration tools

### 2.3 Content Lifecycle

**Interface Definition:**

```go
type ContentWorkflow interface {
    CreateWorkflow(config WorkflowConfig) (*Workflow, error)
    AssignTask(workflowID, userID string, task Task) error
    CompleteTask(taskID string, result TaskResult) error
    GetWorkflowStatus(workflowID string) (*WorkflowStatus, error)
}
```

**Draft and Preview System**

- Multi-stage lifecycle: Draft → Review → Published → Archived
- Preview URLs with authentication: `/preview/{token}/{path}`
- Branch-based drafts for collaboration
- Visual diff comparison
- Mobile/desktop preview modes

**Content Scheduling**

- Time-based publication with `published_at` metadata
- Recurring content schedules
- Timezone-aware scheduling
- Preview scheduled content

**Editorial Workflow**

- Assignment system with roles and permissions
- Comment system with threaded discussions
- Approval workflows with multi-stage review
- Notification system for state changes

## 3. Site Generation

### 3.1 Build System

**Interface Definition:**

```go
type SiteGenerator interface {
    Build(ctx context.Context, config BuildConfig) (*BuildResult, error)
    Watch(ctx context.Context, config BuildConfig) (<-chan BuildEvent, error)
    Serve(ctx context.Context, config ServeConfig) error
    Clean(config BuildConfig) error
}
```

**Static Site Generation**

- Complete site pre-rendering to HTML/CSS/JS
- Incremental builds: process only changed files
- Asset optimization: minification, compression, bundling
- Multi-target output: filesystem, S3, CDN
- Build artifacts with integrity checksums

**Dynamic Serving Mode**

- Real-time document processing on HTTP requests
- Smart caching with TTL and invalidation
- Hot reload of templates and configurations
- Development-friendly error pages
- Performance monitoring

**Hybrid Mode**

- Static generation for stable content
- Dynamic rendering for user-specific content
- Edge-side includes (ESI) support
- Progressive enhancement strategies

### 3.2 Template System

**Interface Definition:**

```go
type TemplateRenderer interface {
    Render(ctx context.Context, template string, data interface{}) ([]byte, error)
    Dependencies(template string) ([]string, error)
    Validate(template string) error
    Hot
}
```

**Component Architecture**

- Reusable components with props: `{{< card title="Title" >}}`
- Slot-based content projection
- Component composition and nesting
- Auto-completion in development mode

**Theme Inheritance**

- Multi-level inheritance: Base → Framework → Site → Custom
- Partial template overrides
- Conflict resolution strategies
- Theme development tools

**Advanced Template Functions**

```go
// Built-in functions
{{ markdown .Content }}
{{ highlight .Code "go" }}
{{ asset "style.css" }}
{{ image .Image 800 600 }}
{{ date .PublishedAt "2006-01-02" }}
{{ truncate .Description 150 }}
{{ taxonomy "tags" }}
{{ related . 5 }}
```

**Multi-Language Support (i18n)**

- Translation key management: `{{ i18n "welcome.message" }}`
- Pluralization rules per language
- RTL language support
- Date/number formatting per locale

## 4. Search and Discovery

### 4.1 Full-Text Search

**Interface Definition:**

```go
type SearchEngine interface {
    Index(ctx context.Context, documents []Document) error
    Search(ctx context.Context, query SearchQuery) (*SearchResult, error)
    Suggest(ctx context.Context, partial string) ([]string, error)
    UpdateIndex(ctx context.Context, document Document) error
}
```

**Bleve Search Integration**

- Go-native full-text search engine
- Multi-language analyzers
- Faceted search with filters
- Boolean search operators
- Search result ranking
- Auto-complete functionality

**Search Features**

- Advanced query syntax: `tag:golang AND status:published`
- Search suggestions and auto-complete
- Faceted search by metadata fields
- Search analytics and optimization

### 4.2 Content Relationships

**Related Content System**

- Bi-directional link tracking
- Tag-based clustering
- Content similarity algorithms
- Series management with navigation
- Automated recommendations

**Content Graph**

- Dependency tracking for cross-references
- Backlink generation
- Link validation and repair
- Graph visualization tools

## 5. Performance and SEO

### 5.1 Performance Optimization

**Interface Definition:**

```go
type PerformanceOptimizer interface {
    OptimizeAssets(ctx context.Context, assets []Asset) ([]OptimizedAsset, error)
    GenerateCriticalCSS(ctx context.Context, url string) (string, error)
    AnalyzePerformance(ctx context.Context, url string) (*PerformanceReport, error)
}
```

**Core Web Vitals**

- Largest Contentful Paint (LCP) optimization
- First Input Delay (FID) reduction
- Cumulative Layout Shift (CLS) prevention
- Performance budgets with enforcement
- Real User Monitoring (RUM) integration

**Asset Optimization**

- Critical CSS extraction and inlining
- Resource bundling and code splitting
- Image optimization with WebP/AVIF
- Font optimization with preloading
- Service worker for caching

### 5.2 SEO Features

**Structured Data**

- Automatic JSON-LD generation
- Schema.org markup for different content types
- Rich snippets optimization
- Social media meta tags

**Site Features**

- XML sitemap generation with priorities
- RSS/Atom feeds with customization
- Robots.txt management
- Canonical URL enforcement
- International SEO with hreflang

## 6. Developer Experience

### 6.1 Development Tools

**Hot Reload System**

- File system watching with debouncing
- Incremental compilation
- Browser auto-refresh via WebSocket
- CSS hot-swapping
- Error overlay in development

**CLI Tool Suite**

```bash
gomddoc new [type] [name]           # Create content
gomddoc build [--watch] [--dev]     # Build site
gomddoc serve [--port] [--host]     # Development server
gomddoc deploy [target]             # Deploy to platforms
gomddoc import [source] [format]    # Import content
gomddoc optimize                    # Performance analysis
gomddoc validate                    # Content validation
```

**Development Server**

- Multi-port setup: content + admin interface
- HTTPS with self-signed certificates
- Proxy support for API backends
- Development middleware and debugging
- Mock data generation

### 6.2 Testing and Quality

**Content Validation**

- Link checking and validation
- Image optimization analysis
- Accessibility testing
- SEO analysis with recommendations
- Performance testing integration

**Build Quality**

- Automated testing in CI/CD
- Performance regression detection
- Cross-browser testing integration
- Visual regression testing
- A/B testing framework

## 7. API and Integration

### 7.1 Content API

**Interface Definition:**

```go
type ContentAPI interface {
    GetDocument(ctx context.Context, id string) (*Document, error)
    ListDocuments(ctx context.Context, filters DocumentFilters) (*DocumentList, error)
    CreateDocument(ctx context.Context, doc *Document) (*Document, error)
    UpdateDocument(ctx context.Context, id string, doc *Document) (*Document, error)
    DeleteDocument(ctx context.Context, id string) error
    SearchDocuments(ctx context.Context, query SearchQuery) (*SearchResult, error)
}
```

**REST API Endpoints**

```
GET    /api/v1/documents              # List with pagination
GET    /api/v1/documents/{id}         # Single document
POST   /api/v1/documents              # Create document
PUT    /api/v1/documents/{id}         # Update document
DELETE /api/v1/documents/{id}         # Delete document
GET    /api/v1/search                 # Full-text search
GET    /api/v1/taxonomy/{type}        # Metadata taxonomy
GET    /api/v1/analytics              # Performance metrics
```

**GraphQL API**

- Complete schema with Query/Mutation types
- Real-time subscriptions for content updates
- Efficient nested field selection
- Advanced filtering and pagination

### 7.2 Integration Platform

**Webhook System**

- Event-driven notifications: `document.created`, `document.updated`
- HMAC signature verification
- Retry mechanisms with exponential backoff
- Webhook testing and debugging tools

**Third-Party Integrations**

- Analytics: Google Analytics 4, Adobe Analytics
- Search: Algolia, Elasticsearch cloud
- CDN: Cloudflare, AWS CloudFront
- Storage: AWS S3, Google Cloud Storage
- Email: SendGrid, Mailchimp
- Comments: Disqus, GitHub Discussions

**Authentication Providers**

- OAuth 2.0: GitHub, Google, Microsoft
- SAML SSO for enterprise
- API key management with scoping
- JWT token validation
- Role-based access control (RBAC)

## 8. Monitoring and Analytics

### 8.1 Observability

**Interface Definition:**

```go
type MetricsCollector interface {
    TrackPageView(ctx context.Context, event PageViewEvent) error
    GetPopularContent(timeRange TimeRange, limit int) ([]ContentMetric, error)
    GetContentPerformance(documentID string) (*PerformanceMetric, error)
    GenerateReport(config ReportConfig) (*AnalyticsReport, error)
}
```

**Prometheus Metrics**

- Standard HTTP metrics: duration, status codes, request size
- Application metrics: processing time, cache ratios
- Content metrics: page views, unique visitors
- System metrics: memory, goroutines, GC duration

**Health Monitoring**

- Liveness probe: `/health/live`
- Readiness probe: `/health/ready`
- Startup probe: `/health/startup`
- Deep health checks for dependencies
- Graceful degradation indicators

**Distributed Tracing**

- OpenTelemetry integration
- Span creation for processing pipeline
- Trace correlation across services
- Performance bottleneck identification

### 8.2 Content Analytics

**Performance Monitoring**

- Core Web Vitals tracking
- Resource loading performance
- Build performance analytics
- Cache efficiency monitoring
- Real User Monitoring (RUM)

**Privacy-Compliant Analytics**

- GDPR/CCPA compliant data collection
- Anonymized IP storage
- Cookie-free analytics options
- User consent management
- Data export and purging capabilities

## 9. Configuration Management

### 9.1 Site Configuration

**Primary Configuration** (`.gomddoc/config.yaml`)

```yaml
site:
  title: "Site Title"
  description: "Site Description"
  base_url: "https://example.com"
  theme: "default"
  language: "en"

build:
  mode: "static" | "dynamic" | "hybrid"
  output_dir: "./dist"
  minify_html: true
  minify_css: true
  concatenate_js: true
  optimize_images: true

cache:
  templates_ttl: "5m"
  static_assets_ttl: "1h"
  content_ttl: "10m"

providers:
  - type: "filesystem"
    path: "./content"
  - type: "github"
    repo: "user/repo"
    token: "${GITHUB_TOKEN}"

processors:
  markdown:
    extensions: ["tables", "footnotes", "strikethrough"]
    syntax_highlighting: true
    math_rendering: true
```

### 9.2 Environment Management

**Environment Variables**

- Development, staging, production configurations
- Secret management with encryption
- Feature flags for gradual rollouts
- Configuration validation and schema enforcement

## Implementation Priorities

### Phase 1: Core Foundation

1. Document provider interface and filesystem implementation
2. Basic Markdown processing with frontmatter
3. Simple template system with theme support
4. Static site generation with incremental builds
5. Development server with hot reload

### Phase 2: Content Management

1. Advanced metadata processing and validation
2. URL routing and rewriting system
3. Multi-format document processors (AsciiDoc, RST)
4. Asset management and optimization
5. Content lifecycle and workflow system

### Phase 3: Search and Discovery

1. Full-text search with Bleve integration
2. Content relationship tracking
3. Advanced URL patterns and redirects
4. SEO optimization features
5. Performance monitoring basics

### Phase 4: API and Integration

1. REST and GraphQL APIs
2. Authentication and authorization
3. Webhook system
4. Third-party service integrations
5. Plugin architecture

### Phase 5: Advanced Features

1. Multi-language support (i18n)
2. Advanced analytics and monitoring
3. Progressive Web App features
4. Enterprise integrations
5. Advanced developer tooling

# Completed Features

- [x] Edit on GitHub links (WU-13): Configurable `edit_url` in site config, `editURL` template function, "Edit this page" footer links

# Deferred Features

- Content versioning/revision history
- VS Code extension
- Git hooks integration
- Advanced image optimization
- Dark/light mode theming
