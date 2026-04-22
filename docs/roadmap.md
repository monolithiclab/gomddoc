---
title: "Gomddoc Roadmap"
description: "Strategic roadmap for gomddoc evolution."
author: "nicolasm"
---

# Gomddoc Roadmap

## Vision

Transform gomddoc from a documentation server into the best git-native documentation viewer and static site
generator. No databases, no editorial workflows, no CMS. The "database" is Git.

- **Read-heavy focus**: Search, speed, and rendering quality over write features
- **Stateless**: No persistent storage beyond Git repos
- **Interface-driven**: All major components implement well-defined interfaces
- **Performance-first**: Built for speed with intelligent caching
- **Security by design**: Path traversal protection, content sanitization, secure defaults

## Current State (Phase 3 Complete)

The foundation is production-ready with ~78% test coverage:

- **CLI**: Kong-based subcommand architecture (`gomddoc serve`), version injection via ldflags, exhaustive
  `--help` with env var discovery.
- **Configuration**: Reflection-based env var walking, CLI flags, YAML config files with proper validation
  and correct prefix nesting (`GOMDDOC_SITE_*`). Priority: flags > env > file > defaults.
- **Providers**: Filesystem (os.DirFS with path traversal protection) and Git (go-git, in-memory clone,
  SSH auth with known_hosts, fail-closed).
- **Rendering**: Markdown (goldmark, GFM, syntax highlighting, admonitions, color chips, TOC, heading anchors,
  YAML frontmatter), passthrough for all other MIME types via `*/*` wildcard.
- **HTTP**: Content negotiation, gzip compression, security headers, ETag, request ID tracking, graceful shutdown.
- **Monitoring**: Prometheus metrics (`/metrics`), health probes (`/health/live`, `/health/ready`).
- **Security**: Hidden file blocking, method filtering (GET/HEAD), clone timeout, file size limits.
- **Edit links**: Configurable `edit_url` in site config with "Edit this page" footer links.
- **Documentation**: User guides (`docs/guide/`), architecture reference (`docs/architecture.md`).

See `docs/architecture.md` for detailed architecture and `docs/guide/` for user documentation.

## Phase 4: Performance and Scaling

*Current Git provider is in-memory only, limiting repo size.*

- [ ] **Disk-based Git storage**: Implement `filesystem.Storage` for the Git provider to support large monorepos
  without OOM errors.
- [ ] **HTTP caching**: `If-Modified-Since` / `ETag` based on Git commit hashes to reduce bandwidth.
- [ ] **Partial clones**: `git clone --filter=blob:none` when upstream library support matures.

## Phase 5: Search and Discovery

*Discovery is critical for documentation.*

- [ ] **Full-text search**: Bleve or client-side solution (Pagefind/Lunr) for content indexing.
- [ ] **Auto-navigation**: Generate sidebar navigation from directory structure.
- [ ] **Metadata indexing**: Parse frontmatter for filtering by tags/categories.

## Phase 6: Renderer Enhancement

*Expand support for technical documentation formats.*

- [ ] **AsciiDoc support**: Renderer for `.adoc` files (popular in technical writing).
- [ ] **OpenAPI renderer**: Render `swagger.yaml` / `openapi.json` as interactive API docs.
- [ ] **Rich Markdown**: Native Mermaid diagrams, MathJax/KaTeX (currently client-side CDN only).

## Phase 7: Static Site Generation

*Bridge the gap between dynamic serving and static hosting.*

- [ ] **`gomddoc build` command**: Crawl content tree, output static HTML/CSS/JS.
- [ ] **Asset optimization**: Minify HTML/CSS/JS during build.
- [ ] **Static host compatibility**: Output structure compatible with S3, Netlify, Cloudflare Pages.

## Phase 8: User Experience and Theming

*Make the default experience polished and professional.*

- [ ] **Theme inheritance**: Override specific partials without copying whole layout.
- [ ] **UI polish**: Copy-to-clipboard for code blocks.
- [ ] **Dark mode**: Native light/dark toggle.

## Phase 9: Enterprise Features

- [ ] **S3 provider**: Serve content directly from S3 buckets (for non-git use cases).
- [ ] **Authentication**: OIDC/OAuth middleware for private documentation.
- [ ] **Branch switching**: UI dropdown to switch between Git branches/tags.

## Phase 10: Git Workflow Integration

- [ ] **Webhooks**: Endpoint to trigger `git fetch` on push events (cache invalidation).
- [ ] **PR preview**: Serve content from PR branches for review.

## Future CLI Commands

| Command | Purpose |
|---------|---------|
| `gomddoc init` | Scaffold a `.gomddoc/` directory with default config |
| `gomddoc build` | Static site generation |
| `gomddoc validate` | Validate config and check for broken links |

## Implementation Strategy

Development proceeds in phases building on stable foundations. Each phase delivers complete, tested functionality.

**Immediate focus (Phase 4 & 5):** Making the tool viable for large repositories (disk storage) and usable for
end-users (search).

## Deferred (Not Planned)

These features were considered but deprioritized to keep gomddoc focused:

- Database providers (PostgreSQL/SQLite) — Git is the database
- REST/GraphQL APIs — gomddoc is a viewer, not a headless CMS
- Editorial workflows (drafts, reviews, scheduling) — use Git branches
- Content versioning/revision history — use Git history
- i18n/multi-language — out of scope for now
- VS Code extension
- Plugin architecture / dynamic renderer loading
