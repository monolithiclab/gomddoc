# Tag Components — Chips, Listing & Index Pages (Sub-Spec 1 of 3)

**Status**: design approved 2026-04-23
**Sub-spec**: 1 of 3 in the broader Tag Components roadmap entry
**Sub-specs in scope**: chips, per-tag listing pages, tag index page
**Sub-specs deferred**: related-pages section (sub-spec 2), `tag:` search syntax (sub-spec 3)

## Goal

Surface frontmatter `tags:` as first-class navigation. Three user-visible features:

1. **Tag chips** below the H1 of every page that has tags.
2. **Per-tag listing page** at `/tags/{tag}` showing search-result-style cards for every page tagged `{tag}`.
3. **Tag index page** at `/tags/` listing all tags alphabetically with counts.

All three are per-language: non-default languages get `/{lang}/tags/...`.

## Design Decisions

### URL format

Tags are percent-encoded literally in URLs — `machine learning` → `/tags/machine%20learning`. No
slugification, no kebab-case transform.

Tags containing `/` are skipped at metadata-index build time with a logged warning. Pathological
tag values (slashes, control chars) would otherwise create build-mode collisions and
filesystem-path ambiguity. Validation runs once in `metadata.Index.BuildIndex` so all downstream
consumers (handlers, enricher's `RelatedDocs`, search) see a consistent set.

### Multi-language

Per-language tag pages, mirroring the existing per-language sitemap/feed pattern:

- Default language: `/tags/{tag}` and `/tags/`.
- Each non-default language `lang`: `/{lang}/tags/{tag}` and `/{lang}/tags/`.

Each language's `metadata.Index` drives its own listings. A French tag page only shows French pages.

### Chip placement

- **Position**: inline horizontal row of chips between the H1 and the body content, in the main
  content area.
- **Visibility**: gated by a new `tag_chips` feature flag, **default on**. Site-level via
  `features:` config; per-page override via frontmatter `features: { tag_chips: false }`. Mirrors
  the existing `color_chips` / `katex` / `mermaid` toggle pattern.

### Per-tag listing page (`/tags/{tag}`)

Search-result-style cards: each entry shows the page title (linked) and frontmatter description
when present. Sorted alphabetically by title.

### Tag index page (`/tags/`)

Alphabetical list of every tag with its document count: `kubernetes (3)`, `tutorial (5)`, etc.
Each entry links to the per-tag page.

### Pagination, sort variants, tag cloud

Out of scope for v1. Most sites have <100 pages per tag. Will revisit when bitten.

## Architecture

No new architectural layers. Reuses:

- `metadata.Index.AllTags()` and `ByTag(tag)` (already present).
- The per-language pipeline (`LangPipelines`) registered in `internal/server/server.go` (already
  used for sitemap/feed).
- The template renderer's partial inheritance system (default theme partials are inherited by all
  themes unless overridden).
- The standard build-time walk-and-emit pattern in `cmd/gomddoc/build.go`.

## Components

### New code (this repo)

| File | Purpose |
|------|---------|
| `internal/metadata/index.go` (modified) | Tag charset validation in `BuildIndex` — log warning + skip tags containing `/`, empty, or whitespace-only |
| `internal/server/tags_html.go` (new) | `TagPageHandler` (GET `/tags/{tag}`), `TagsIndexHandler` (GET `/tags/`) |
| `internal/server/server.go` (modified) | Register HTML routes for default lang + each non-default lang pipeline |
| `internal/server/sitemap.go` (modified) | Include per-language `/tags/{escaped-tag}` and `/tags/` URLs in generated sitemap |
| `internal/template/renderer.go` (modified) | New methods `RenderTagPage(ctx, lang, tag, pages)` and `RenderTagsIndex(ctx, lang, tagsWithCounts)` — execute the appropriate partial to build body HTML, then invoke the standard layout with that as `Page.Content` |
| `internal/template/renderer.go` `funcMap` (modified) | New template helpers registered on the renderer: `tagURL(lang, tag)` using `url.PathEscape`, `pageTags(meta)` to normalize the YAML `[]any` value of `Meta["tags"]` to `[]string` |
| `cmd/gomddoc/build.go` (modified) | Iterate `AllTags()` per language, emit `tags/{escaped-tag}/index.html` + `tags/index.html` |
| `cmd/gomddoc/locales/en-US.yml` (modified) | New keys: `tags.title`, `tags.indexTitle`, `tags.taggedAs`, `tags.count`, `tags.empty` |

### New theme assets (default theme, inherited by all 8)

| File | Purpose |
|------|---------|
| `partials/tag-chips.html.tmpl` | Chip rendering. Reads `.Page.Meta.tags` via `pageTags`, emits `<ul class="tag-chips">…</ul>` with chip links via `tagURL` |
| `partials/tags-list.html.tmpl` | Body of `/tags/{tag}` — search-result-style cards |
| `partials/tags-index.html.tmpl` | Body of `/tags/` — `<a>tag</a> (n)` per row |
| `layouts/default.html.tmpl` (modified) | Call `{{ template "tag-chips" . }}` below H1, gated by `{{ if .Page.Feature "tag_chips" }}` |

### Coordinated change (separate repo)

The 7 themes in `gomddoc-themes` each add the same `{{ template "tag-chips" . }}` invocation to
their main layout below the H1. The shared partial uses CSS custom properties from the existing
theme variable system (`--color-primary`, `--color-bg-subtle`), so no per-theme styling is needed.

## Data Flow

### Request flow (serve mode)

```
GET /fr/tags/machine%20learning
  → langRouter strips /fr/ prefix, picks French pipeline
  → TagPageHandler.ServeHTTP
  → r.PathValue("tag") = "machine learning" (URL-decoded by ServeMux)
  → MetaIndex.ByTag("machine learning") → []PageInfo
  → templateRenderer.RenderTagPage(ctx, "fr", tag, pages)
       → execute "tags-list" partial against {Tag, Pages, Lang} → body HTML
       → set Page.Content = body, Page.Path = "/fr/tags/machine learning"
       → execute default.html.tmpl layout → final HTML
  → serveWithETag (FNV-64a content hash, Cache-Control: max-age=300)
```

### Build flow

```
For each language pipeline lp:
  For each tag in lp.MetaIndex.AllTags():
    Render → write tags/{url.PathEscape(tag)}/index.html
             (lang-prefixed if non-default)
  Render index → write tags/index.html
                 (lang-prefixed if non-default)
```

### Chip rendering

Tags come from `Page.Meta["tags"]` (already populated by enricher from frontmatter). Active
language comes from `Page.Lang` (already set by handler). Chip URLs are built as
`tagURL(.Lang, tag)` which produces `/{lang prefix}/tags/{url.PathEscape(tag)}`.

## Edge Cases

| Case | Behavior |
|------|----------|
| Page with no `tags:` frontmatter | Chip partial early-returns; renders nothing |
| `tag_chips: false` per page or site | Layout's `{{ if }}` guard skips the partial |
| `GET /tags/nonexistent` | **404** — non-existent resource; correct HTTP semantics |
| `GET /tags/` on a site with zero tags | Renders the page with locale string `tags.empty` |
| Frontmatter `tags: [""]` or whitespace entries | Skipped during `metadata.Index` build |
| Tag with `/` in value | Skipped + warned at index build time |
| Page with `robots: noindex` appears in a tag listing | Shown — `noindex` is a search-engine signal; users navigating via chips should still find their pages |
| Excluded paths (`exclude:` glob match) | Already absent from `metadata.Index`; no extra filtering |
| Concurrent requests | `RenderTagPage`/`RenderTagsIndex` are stateless given an immutable `MetaIndex` |
| Cache headers | Standard `serveWithETag` + `Cache-Control: public, max-age=300` |
| Sitemap inclusion | Tag pages added per-language to `GenerateSitemap` |
| Atom feed | Skipped — tag pages have no meaningful update timestamp |
| User content at `/tags` (e.g. `tags.md` at content root) | Tag routes register before the catch-all content handler, so they win. A non-fatal startup warning surfaces if `tags.md` or a `tags/` directory exists in any language's content root, advising the user to rename. The warning is per-language (each language pipeline checks its own root) |

## Tag Page Decorations

- **Breadcrumbs**: `Home › Tags › {tag}` for `/tags/{tag}`; `Home › Tags` for `/tags/`. The
  handler passes the request URL path (e.g. `/fr/tags/machine learning`) to the existing
  breadcrumb generator, and the renderer supplies a label override map so the segment `tags`
  resolves to the localized label rather than the title-cased "Tags" default.
- **`<title>`**: `{tag} · Tags · {SiteName}` for per-tag; `Tags · {SiteName}` for index.
- **Meta description**: omitted (auto-generated descriptions are SEO noise).
- **JSON-LD**: omitted — tag pages are aggregations, not `TechArticle`s.

## Testing

| Layer | Coverage |
|-------|----------|
| `metadata.Index` tag validator | Table test: valid tags accepted; `/`-containing, empty, and whitespace-only tags skipped with logged warning |
| `internal/server/tags_html.go` handlers | 200 for existing tag (response contains expected page link); 404 for nonexistent tag; 200 for empty `/tags/` rendering empty-state string; per-language routing isolates results to that language's pipeline |
| `internal/template/renderer.go` `RenderTagPage`/`RenderTagsIndex` | Renders against `fstest.MapFS` theme; output contains the listed pages; `tag_chips` feature flag gates the chip partial; chip URLs use the active language prefix |
| `cmd/gomddoc/build.go` build mode | Generates `tags/{escaped-tag}/index.html` for each unique tag plus `tags/index.html`, per language. Path-traversal guard rejects `tag` values that resolve outside output dir (defense in depth) |
| Sitemap | Generated sitemap includes all tag URLs for the language |
| `tagURL` / `pageTags` helpers | Unit tests for normalization and per-language URL construction |

Coverage target: maintain 86%+ overall; new files at 90%+.

## Rollout

1. **This repo PR** — validator, handlers, render methods, build-mode emission, sitemap inclusion,
   default theme partials + layout call, locale strings, tests.
2. **`gomddoc-themes` coordinated PR** — add `{{ template "tag-chips" . }}` invocation to each of
   the 7 other themes' main layouts at the same position.
3. **`gomddoc-website` PR** — update theming guide (new partial overridable), frontmatter
   reference (`tag_chips` feature flag), and a small "Tags" section in the user guide.

## Out of Scope (Deferred)

- Pagination on per-tag listings.
- Custom per-tag pages (user-authored `tags/foo.md` overriding the auto-generated listing).
- "Trending" or "popular" sort.
- Related-pages section at the bottom of content pages — sub-spec 2.
- `tag:` search syntax in the search modal — sub-spec 3.
