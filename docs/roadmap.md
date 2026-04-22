# PLAN2: Gomddoc Evolution Roadmap

## Overview

Gomddoc currently exists as a production-ready HTTP server with clean, interface-driven architecture. It serves Markdown
files as HTML through a MIME-type based rendering system with proper content negotiation, security headers, and
extensible design patterns. The codebase achieves 78.9% test coverage and demonstrates excellent adherence to SOLID
principles with zero critical issues.

The vision transforms this foundation into a comprehensive static site generator and content management platform while
preserving the architectural strengths that make the current implementation successful. This evolution follows a
deliberate path from stability improvements through incremental feature additions, ensuring each phase builds on solid
ground.

## Current State: Phase 2 Complete

The existing architecture implements five core layers working in concert. The Provider layer handles file I/O and MIME
type detection using Go's standard library filesystem interfaces with path traversal protection via `os.DirFS()`. The
Renderer layer transforms content through a MIME-type routing system, currently supporting markdown-to-HTML conversion
and passthrough for other file types. A Registry maps MIME types to renderers using wildcard matching, enabling
extensibility without hardcoded mappings. The Handler orchestrates HTTP requests through content negotiation based on
Accept headers. Finally, the Template layer wraps rendered HTML in themes with optional caching for production
deployments.

This foundation excels at separation of concerns, testability, and security. Path traversal attacks are impossible by
design. Hidden files remain inaccessible except for RFC-compliant `.well-known` paths. Security headers protect against
common attacks. Error handling properly distinguishes between client errors and server failures without leaking
implementation details. The system handles request cancellation gracefully throughout the rendering pipeline.

However, several quality improvements deserve attention before expanding functionality. Configuration validation remains
incomplete, allowing invalid settings to cause runtime failures rather than startup errors. Resource cleanup on error
paths could leak file handles in edge cases. The template cache grows unbounded, which works for typical deployments but
lacks documentation about this limitation. Some validation logic appears in multiple places without clear justification.
Thread safety guarantees exist but remain undocumented, creating uncertainty for future maintainers.

## Foundation: Stability and Quality

Before introducing new features, the codebase requires hardening in three areas. First, comprehensive validation must
prevent invalid configurations from reaching runtime. Port formats need verification, directories must exist and be
readable, timeout values should fall within reasonable bounds. Domain validation should reject protocols, paths, and
ports while trimming whitespace. Path cleaning logic deserves extraction to a single, well-tested helper function that
handles edge cases explicitly rather than relying on implicit behavior.

Second, resource management needs defensive improvements. The provider must close properly even when subsequent
initialization steps fail. The HTTP server needs WriteTimeout and IdleTimeout to prevent resource exhaustion from slow
clients. MaxHeaderBytes should limit header size to prevent memory-based denial of service. Request path length
validation should occur at the HTTP layer before reaching breadcrumb generation. Thread safety guarantees for all shared
components deserve explicit documentation explaining what operations are safe concurrently and which require external
synchronization.

Third, testing and documentation gaps need filling. Integration tests should cover error paths in main.go that currently
lack coverage. Concurrency tests must verify that shared components behave correctly under parallel access. Stress tests
with large files and deep directory structures should confirm performance characteristics remain acceptable at scale.
Benchmarks for critical paths enable detection of performance regressions. Documentation should include example
configuration files, migration guides from earlier versions, troubleshooting common issues, and performance tuning
recommendations.

## Phase 3: Provider Expansion

The current filesystem provider serves as a template for additional content sources. The Provider interface requires no
changes to support databases, object storage, or API backends. Each new provider implements the same contract: read
files, detect MIME types, provide metadata, support directory operations.

A PostgreSQL provider enables content stored in relational databases with full-text search via tsvector columns. Schema
design separates document content from metadata in JSONB fields, enabling flexible querying without schema migrations
for every metadata field. Connection pooling and prepared statements ensure efficient operation under load. SQLite
offers similar capabilities for single-user or embedded scenarios without external database dependencies.

Cloud storage providers unlock horizontal scaling. An S3 provider reads content from AWS buckets with proper credential
management and region configuration. Similar providers for Google Cloud Storage and Azure Blob Storage follow the same
pattern. Local caching strategies reduce latency and API costs for frequently accessed content. Conditional requests
using ETags prevent unnecessary data transfer when content hasn't changed.

GitHub integration treats repositories as content sources. The GraphQL API provides efficient queries for file content
and metadata. Webhook support enables real-time updates when content changes. Private repository access requires secure
token management. Local caching with Git integration allows offline operation and faster access to frequently requested
files.

Multiple providers can coexist through aggregation. A priority system determines which provider answers requests when
multiple sources contain the same path. Content deduplication via hashing prevents duplicate processing. Unified search
operates across all providers transparently. Health monitoring detects provider failures and routes around them
gracefully.

## Phase 4: Renderer Enhancement

The renderer system currently handles Markdown and passthrough content. Additional renderers expand supported formats
without architectural changes. Each renderer declares its supported MIME types and implements the same transformation
interface.

AsciiDoc support targets technical documentation with advanced features like conditional inclusion, cross-references,
and bibliography management. Integration with the asciidoctor library provides full specification compliance. PlantUML
and Graphviz diagrams render inline. Output remains HTML but with richer semantic structure than basic Markdown
provides.

ReStructuredText serves Python documentation communities through docutils integration. Sphinx-compatible directives work
without modification. API documentation extraction from source code docstrings happens during rendering. Cross-document
linking maintains referential integrity across large documentation sets.

Jupyter notebook rendering transforms `.ipynb` files into static HTML with code execution results, plots, and
interactive elements preserved visually. Math rendering via KaTeX or MathJax ensures equations display properly. Code
syntax highlighting matches the notebook's original appearance.

Enhanced Markdown rendering adds features beyond basic CommonMark. Math blocks render inline and display equations.
Mermaid diagrams generate flowcharts, sequence diagrams, and graphs. Custom shortcodes embed external content like
YouTube videos or tweets. Automatically generated tables of contents help navigate long documents. Reading time
estimation appears in metadata. Footnotes and citations provide academic paper support.

A plugin architecture allows custom renderers without forking the codebase. Go's plugin system enables dynamic loading
of compiled shared libraries. Sandboxing protects against malicious renderers. Priority chains let multiple renderers
handle the same MIME type with fallback behavior. Hot reloading in development mode enables rapid iteration on custom
renderers.

## Phase 5: Template System Evolution

The current template system wraps HTML content in a basic layout. Evolution requires component-based architecture, theme
inheritance, and internationalization support while maintaining the simplicity that makes the current system
understandable.

Components act as reusable template fragments with named slots for content projection. A card component accepts title,
body, and footer sections. Composition allows nesting components to build complex layouts from simple parts.
Autodiscovery in development mode makes components available without explicit registration. Props provide type-safe data
passing between parent templates and child components.

Theme inheritance creates layering from generic to specific. A base theme provides the fundamental structure. Framework
themes build on the base with opinionated styling. Site-specific themes override individual templates or components
while inheriting the rest. Partial overrides let sites customize header navigation without replacing the entire layout.
Conflict resolution favors more specific themes when multiple layers define the same template.

Internationalization support begins with message catalogs organized by language code. Template functions extract
translated strings by key. Pluralization rules differ by language and receive proper handling. Right-to-left language
support flips layouts appropriately. Date and number formatting respects locale conventions. Language-specific content
variants select automatically based on Accept-Language headers or URL prefixes.

Advanced template functions expose common operations without requiring custom renderers. Markdown rendering of content
fields happens inline. Syntax highlighting for code snippets uses the same library as the Markdown renderer. Asset
fingerprinting generates cache-busting filenames. Image resizing creates responsive variants. Date formatting supports
custom patterns. Text truncation with word boundaries prevents awkward cuts. Taxonomy queries generate tag clouds and
category listings. Related content functions suggest similar documents based on metadata.

## Phase 6: Content Management

Currently, the system serves files without understanding their relationships or lifecycle. Content management introduces
metadata, URL control, and workflow support.

Frontmatter parsing extracts structured metadata from document preambles. YAML remains the primary format with support
for TOML and JSON alternatives. Schema validation ensures required fields exist and types match expectations. Default
values fill missing optional fields. Inheritance propagates metadata from parent directories to children. Custom field
definitions per document type enable flexibility without schema rigidity.

Standard metadata fields cover common needs. Title, description, and author information populate HTML meta tags.
Created, updated, and published timestamps track document lifecycle. Status flags distinguish drafts from published
content. Slug fields override default URL generation. Canonical URLs prevent duplicate content penalties. Tags and
categories organize content taxonomically. Series information groups related documents with explicit ordering.
Difficulty levels and language codes enable filtered browsing. Translation relationships link equivalent content across
languages.

URL management transforms file paths into readable web addresses. Pattern-based rewriting removes file extensions and
normalizes structure. Hierarchical patterns mirror directory organization. Date-based patterns suit blog-style content.
Author-based patterns enable multi-user sites. Custom patterns using metadata fields provide maximum flexibility.
Aliases create multiple URLs for the same content supporting legacy links. Redirect generation creates proper 301
responses preventing broken bookmarks. Conflict detection prevents multiple documents from claiming the same URL.

Content lifecycle support enables collaborative workflows. Draft status hides content from production while making it
available in development. Preview URLs with authentication tokens share pre-publication content securely. Branch-based
drafts enable parallel work on different versions. Visual diff comparison shows changes between versions. Scheduled
publication using future timestamps queues content for automatic release. Review workflows assign documents to editors
with approval requirements. Comment threads enable discussion of changes. Notification systems alert interested parties
to state transitions.

## Phase 7: Static Site Generation

The current dynamic serving model generates HTML per request. Static generation pre-renders all content for deployment
to CDNs or static hosting platforms.

The build system processes an entire content tree into an output directory. Complete site rendering generates HTML, CSS,
and JavaScript files with proper relative linking. Incremental builds detect changes and process only affected files.
Asset optimization minifies HTML, CSS, and JavaScript automatically. Image optimization generates WebP and AVIF variants
with responsive sizing. Output targets support local filesystems, S3 buckets, and CDN integration. Build artifacts
include integrity checksums for verification.

A hybrid model combines static and dynamic approaches. Stable content renders statically during build. User-specific or
frequently changing content renders dynamically per request. Edge Side Includes allow mixing static and dynamic
fragments in a single page. Progressive enhancement ensures functionality degrades gracefully when dynamic features
fail.

Watch mode enables continuous rebuilds during development. File system monitoring detects changes with debouncing to
prevent rebuild storms. Partial invalidation of caches ensures changes propagate immediately. Browser auto-refresh via
WebSocket eliminates manual page reloads. CSS hot-swapping updates styles without full page refresh. Error overlays
display build failures prominently during development.

Build performance optimization becomes critical for large sites. Parallel processing uses all available CPU cores.
Worker pools manage concurrency without overwhelming the system. Dependency tracking rebuilds only affected pages when
shared resources change. Caching strategies preserve expensive operations across builds. Performance monitoring tracks
build duration and identifies bottlenecks.

## Phase 8: Search and Discovery

Content without discovery remains invisible. Search infrastructure enables users to find relevant documents while
content relationships surface related material automatically.

Full-text search integration uses Bleve, a pure Go search library requiring no external dependencies. Document indexing
extracts text content and metadata during builds or on content updates. Multi-language analyzers understand stemming and
stop words for various languages. Boolean query operators combine search terms with AND, OR, and NOT logic. Faceted
search filters results by metadata fields like tags or dates. Search result ranking uses TF-IDF and other relevance
signals. Autocomplete suggestions help users formulate effective queries. Search analytics track popular queries and
failed searches informing content strategy.

Content relationships emerge from multiple signals. Bi-directional link tracking identifies documents that reference
each other. Tag-based clustering finds documents sharing keywords. Content similarity algorithms compare text and
metadata to identify related material. Series management creates explicit ordered relationships. Automated
recommendations suggest next documents based on current context. Backlink generation shows what content references a
particular document. Link validation detects broken references during builds. Dependency graphs visualize content
relationships.

Performance optimization ensures search remains fast at scale. Index updates happen incrementally rather than rebuilding
everything. Query caching returns frequent searches from memory. Result pagination prevents overwhelming responses.
Index size monitoring ensures storage remains manageable. Query performance metrics identify slow searches needing
optimization.

## Phase 9: Developer Experience

The current server starts with a single command but lacks tools for content creation, validation, and deployment.
Developer experience improvements reduce friction and catch errors early.

A comprehensive CLI provides subcommands for common operations. Creating new content generates files with appropriate
frontmatter templates. Building sites runs the static generation process with configurable output. Serving content
starts the development server with hot reload. Deploying sites pushes content to configured targets with atomic updates.
Importing content from other systems migrates existing material. Optimizing sites runs performance analysis with
actionable recommendations. Validating content checks metadata, links, and images before deployment.

The development server runs multiple services simultaneously. A primary port serves content with hot reload. An admin
interface on a separate port provides content management UI. HTTPS with self-signed certificates tests secure contexts.
Proxy support forwards API requests to backend services during development. Development middleware adds debugging
headers and detailed error pages. Mock data generation creates realistic test content.

Content validation catches errors before deployment. Link checking verifies all internal and external references
resolve. Image optimization analysis identifies opportunities to reduce file sizes. Accessibility testing finds common
WCAG violations. SEO analysis scores pages and suggests improvements. Performance testing validates Core Web Vitals
metrics. CI/CD integration runs validations automatically on every commit.

Hot reload monitors the filesystem for changes and triggers appropriate rebuilds. Content changes regenerate affected
pages. Template changes rebuild all pages using those templates. Configuration changes restart the server with new
settings. CSS changes hot-swap without page refresh. JavaScript changes reload modules preserving application state when
possible. Browser WebSocket receives reload notifications.

## Phase 10: API and Integration

The current system operates standalone. API support enables headless CMS usage, third-party integrations, and advanced
workflows.

A RESTful API exposes content operations over HTTP. Listing documents supports pagination, filtering, and sorting.
Retrieving single documents returns content and metadata. Creating documents accepts content and frontmatter. Updating
documents modifies existing content or metadata. Deleting documents removes content permanently with confirmation.
Full-text search uses the same backend as the web interface. Taxonomy endpoints list available tags, categories, and
other metadata values. Analytics endpoints expose performance and usage metrics.

GraphQL provides an alternative with efficient nested queries. Schema defines available queries and mutations. Real-time
subscriptions push updates via WebSocket when content changes. Field selection minimizes data transfer by requesting
only needed attributes. Advanced filtering and pagination handle large datasets efficiently. Type safety ensures queries
remain valid across API versions.

Webhook systems enable event-driven integrations. Document creation, update, and deletion events trigger notifications.
HMAC signatures verify webhook authenticity preventing spoofing. Retry mechanisms with exponential backoff handle
transient failures. Webhook testing tools simulate events during development. Event logging tracks webhook delivery
success and failures.

Third-party integrations connect to common services. Analytics platforms like Google Analytics track visitor behavior.
Search services like Algolia provide advanced search with typo tolerance and faceting. CDN integration with Cloudflare
or CloudFront accelerates global delivery. Cloud storage integration with S3 or GCS handles large media files. Email
services like SendGrid or Mailchimp enable newsletters and notifications. Comment systems like Disqus or GitHub
Discussions add interactivity.

Authentication enables access control and personalization. OAuth 2.0 integration supports GitHub, Google, and Microsoft
logins. SAML provides enterprise SSO for organizational deployments. API key management creates scoped tokens for
programmatic access. JWT validation enables stateless authentication. Role-based access control defines permissions per
user or group.

## Phase 11: Observability and Operations

Production deployments require visibility into system behavior and performance characteristics.

Prometheus metrics expose operational data for monitoring systems. Standard HTTP metrics track request counts,
durations, and status codes. Application metrics measure processing time, cache hit rates, and render operations.
Content metrics count page views and unique visitors. System metrics monitor memory usage, goroutine counts, and garbage
collection duration. Custom metrics instrument business-specific operations.

Health checks enable automated monitoring and orchestration. Liveness probes verify the server process runs correctly.
Readiness probes check all dependencies are available before accepting traffic. Startup probes handle long
initialization periods gracefully. Deep health checks validate database connections, file system access, and external
API availability. Graceful degradation indicators show partial system failures.

Distributed tracing follows requests through complex processing pipelines. OpenTelemetry integration provides
vendor-neutral instrumentation. Span creation marks each processing stage with timing and metadata. Trace correlation
links related operations across service boundaries. Performance bottleneck identification highlights slow operations.
Trace sampling balances observability with overhead.

Content analytics track user behavior and content performance. Core Web Vitals measurement monitors Largest Contentful
Paint, First Input Delay, and Cumulative Layout Shift. Resource loading performance identifies slow assets. Build
performance analytics track generation duration and throughput. Cache efficiency monitoring measures hit rates and
eviction patterns. Real User Monitoring captures actual user experience data. Privacy-compliant analytics respect GDPR
and CCPA by anonymizing data. Cookie-free analytics options avoid consent requirements. User consent management
integrates with common frameworks. Data export and purging capabilities satisfy user rights requests.

## Phase 12: Performance and SEO

Performance and discoverability determine user satisfaction and search rankings.

Core Web Vitals optimization targets Google's quality metrics. Largest Contentful Paint optimization ensures primary
content renders quickly through critical CSS extraction and resource prioritization. First Input Delay reduction
minimizes JavaScript execution blocking user interaction. Cumulative Layout Shift prevention reserves space for images
and dynamic content. Performance budgets enforce maximum acceptable resource sizes. Real User Monitoring tracks actual
user experience across geographies and devices.

Asset optimization reduces bandwidth and improves load times. Critical CSS extraction inlines above-the-fold styles in
the HTML head. Resource bundling combines multiple files reducing HTTP requests. Code splitting loads JavaScript
incrementally as needed. Image optimization generates modern formats like WebP and AVIF with responsive variants. Font
optimization includes only used glyphs and preloads critical fonts. Service workers cache assets enabling offline access
and instant repeat visits.

SEO features improve search engine visibility and ranking. Structured data in JSON-LD format provides rich snippet
information. Schema.org markup describes article, organization, and breadcrumb semantics. Social media meta tags
populate Open Graph and Twitter Card previews. XML sitemap generation includes all public pages with priorities and
update frequencies. RSS and Atom feeds enable subscription to new content. Robots.txt management controls crawler
access. Canonical URL enforcement prevents duplicate content penalties. International SEO with hreflang tags relates
translated content. Mobile-friendliness testing ensures responsive design works correctly.

## Phase 13: Advanced Features

Several features enhance capabilities without blocking earlier phases.

Multi-provider aggregation serves content from multiple sources simultaneously. Priority-based resolution determines
which provider wins when paths conflict. Content deduplication via SHA-256 hashing prevents redundant processing.
Unified search indexes all providers transparently. Provider health monitoring detects failures and routes around
problems. Load balancing distributes requests across provider instances.

Advanced caching strategies improve performance. Multi-tier caching uses memory, disk, and CDN layers appropriately.
Cache invalidation strategies handle content updates efficiently. Conditional requests with ETags reduce bandwidth.
Stale-while-revalidate patterns serve cached content while updating in background. Cache warming preloads frequently
accessed content.

Content versioning tracks document history. Revision history records every change with author and timestamp. Diff views
compare any two versions. Rollback capabilities restore previous versions. Branch-based editing enables parallel work
streams. Merge conflict resolution handles simultaneous edits.

Visual editing provides WYSIWYG content authoring. Live preview shows rendered output while editing. Inline editing
changes content directly in preview. Drag-and-drop media upload simplifies image insertion. Markdown toolbar aids
formatting for non-technical authors. Distraction-free writing mode maximizes screen space for content.

## Implementation Strategy

Development proceeds in phases building on stable foundations. Each phase delivers complete, tested functionality rather
than partial implementations. Quality gates prevent advancement to the next phase until current work meets standards.

Phase 3 focuses on stability improvements identified in the code review. All high-priority issues receive fixes with
tests proving correctness. Medium-priority improvements reduce technical debt. Documentation fills gaps in
understanding. Benchmarks establish performance baselines preventing regressions. This phase succeeds when test coverage
exceeds 85% and all known issues have resolutions or documented decisions to defer.

Phase 4 expands providers beginning with the simplest addition. SQLite support demonstrates database integration without
external dependencies. S3 support proves cloud storage patterns. Multiple provider aggregation shows coordination
between sources. Each provider includes comprehensive tests and documentation. Success means content can be served
transparently from any provider.

Phase 5 adds renderers following the same incremental approach. Enhanced Markdown rendering adds practical features
users request. AsciiDoc support validates integration with external libraries. Custom renderer plugin architecture
proves extensibility. Success means any format can be rendered through standard interfaces.

Phase 6 builds template system enhancements incrementally. Component support enables reusable fragments. Theme
inheritance proves multi-layer composition. Internationalization demonstrates locale-aware rendering. Success means
themes can be customized without forking base templates.

Phase 7 implements content management features progressively. Frontmatter parsing and validation come first. URL
management builds on metadata foundation. Content lifecycle features enable collaborative workflows. Success means
content can be managed through its entire lifecycle from draft to archive.

Phase 8 delivers static site generation capabilities. Basic build system pre-renders content. Incremental builds
optimize performance. Hybrid mode proves selective static and dynamic rendering. Success means sites deploy to static
hosting platforms or CDNs without dynamic runtime requirements.

Phase 9 adds search and discovery features. Bleve integration provides full-text search. Content relationship tracking
surfaces related material. Performance optimization ensures search scales. Success means users find relevant content
quickly.

Phase 10 enhances developer experience through tooling. CLI commands automate common tasks. Development server
improvements reduce iteration time. Content validation catches errors early. Success means developers work efficiently
with quick feedback loops.

Phase 11 exposes APIs for headless usage. REST endpoints provide standard CRUD operations. GraphQL enables efficient
nested queries. Webhook systems integrate with external services. Success means gomddoc functions as a headless CMS
backend.

Phase 12 instruments observability. Prometheus metrics expose operational data. Health checks enable automated
monitoring. Distributed tracing follows request paths. Success means operations teams understand system behavior and can
respond to issues proactively.

Phase 13 optimizes performance and SEO. Core Web Vitals monitoring tracks quality metrics. Asset optimization reduces
load times. SEO features improve discoverability. Success means sites perform well and rank appropriately in search
results.

This roadmap transforms gomddoc from a focused Markdown server into a comprehensive content platform while maintaining
the architectural clarity and code quality that characterize the current implementation. Each phase delivers value
independently while enabling subsequent capabilities. The result combines the simplicity users appreciate with the power
they need.
