# PLAN2: Gomddoc Evolution Roadmap

## Overview

Gomddoc currently exists as a production-ready HTTP server with clean, interface-driven architecture. It serves Markdown
files as HTML through a MIME-type based rendering system with proper content negotiation, security headers, and
extensible design patterns. The codebase achieves ~79% test coverage and demonstrates excellent adherence to SOLID
principles.

The vision transforms this foundation into a comprehensive documentation viewer and static site generator while preserving the architectural strengths that make the current implementation successful. This evolution prioritizes the "Git-Native" identity of the tool, focusing on reading and rendering content directly from repositories rather than building a heavy database-backed CMS.

## Current State: Phase 3 Complete (Stability & Foundation)

The existing architecture implements five core layers working in concert (Provider, Renderer, Registry, Handler, Template). Recent improvements have solidified the foundation:

- **Configuration**: A robust reflection-based configuration system handles environment variables and CLI flags with proper validation.
- **Authentication**: A secure, explicit SSH key mechanism supports private Git repositories without relying on implicit agent state.
- **Documentation**: Comprehensive user guides and architectural specs are in place.
- **Security**: Path traversal protection and hidden file blocking are strictly enforced.

The system is stable, secure, and ready for feature expansion.

## Strategic Pivot: "The Best Git-Backed Viewer"

Rather than evolving into a traditional database-backed CMS, `gomddoc` will double down on its unique strength: being a stateless, git-native documentation viewer. This means:

1.  **No Databases:** We de-prioritize SQL/NoSQL backends. The "database" is Git.
2.  **Read-Heavy Focus:** We focus on search, speed, and rendering quality over "write" features like draft management.
3.  **Scalability via Disk:** We solve memory constraints for large repos using disk-based git storage, not external databases.

## Refined Roadmap

### Phase 4: Performance & Scaling (Git-Centric)

*Current Git provider is in-memory only, limiting repo size.*

- [ ] **Disk-Based Git Storage**: Implement `filesystem.Storage` for the Git provider to support large monorepos without OOM errors.
- [ ] **HTTP Caching**: Implement `If-Modified-Since` / `ETag` support based on Git commit hashes to reduce bandwidth.
- [ ] **Partial Clones**: Investigate `git clone --filter=blob:none` when upstream library support matures.

### Phase 5: Search and Discovery

*Discovery is critical for documentation.*

- [ ] **Full-Text Search**: Integrate `bleve` or a client-side solution (like Pagefind/Lunr) to index content.
- [ ] **Navigation**: Auto-generate sidebar navigation from directory structure.
- [ ] **Metadata Indexing**: Parse frontmatter to allow filtering by tags or categories.

### Phase 6: Renderer Enhancement

*Expand support for technical documentation formats.*

- [ ] **AsciiDoc Support**: Add a renderer for `.adoc` files (popular in technical writing).
- [ ] **OpenAPI Renderer**: Render `swagger.yaml` / `openapi.json` files as interactive API docs.
- [ ] **Rich Markdown**: Add native support for Mermaid diagrams, MathJax/KaTeX, and syntax highlighting.

### Phase 7: Static Site Generation (SSG)

*Bridge the gap between dynamic serving and static hosting.*

- [ ] **Build Command**: Add `gomddoc build` to crawl the content tree and output static HTML/CSS/JS.
- [ ] **Asset Optimization**: Minify assets during the build process.
- [ ] **S3/Netlify Compat**: Ensure output structure is compatible with common static hosts.

### Phase 8: User Experience & Theming

*Make the default experience polished and professional.*

- [ ] **Theme Inheritance**: Allow custom themes to override specific partials (e.g., just the footer) without copying the whole layout.
- [ ] **UI Polish**: Add "Copy to Clipboard" buttons for code blocks.
- [ ] **Dark Mode**: Native toggle for light/dark themes.

### Phase 9: Enterprise Integrations

- [ ] **S3 Provider**: Support serving content directly from S3 buckets (for non-git use cases).
- [ ] **Authentication**: Add OIDC/OAuth middleware to put documentation behind a login (e.g., "Internal Docs Only").
- [ ] **Observability**: Add Prometheus metrics for request latency and cache hit rates.

### Phase 10: Content Management (Git-Workflow)

*Postponed: Instead of building a UI for editing, improve the Git workflow.*

- [ ] **Branch Switching**: UI dropdown to switch between Git branches/tags view.
- [ ] **Edit Links**: "Edit this page on GitHub/GitLab" buttons.
- [ ] **Webhooks**: Endpoint to trigger a `git fetch` (cache invalidation) on push events.

## Implementation Strategy

Development proceeds in phases building on stable foundations. Each phase delivers complete, tested functionality.

**Immediate Focus (Phase 4 & 5):** The priority is making the tool viable for large repositories (Disk Storage) and usable for end-users (Search).

**Deferred:** Database providers (PostgreSQL/SQLite) are removed from the immediate roadmap to keep the application stateless and simple. Write-based CMS features are deprioritized in favor of a robust Git-based read-only workflow.