# See Also Section (Tag Components Sub-Spec 2 of 3)

**Status**: design approved 2026-04-24
**Sub-spec**: 2 of 3 in the broader Tag Components roadmap entry
**Sub-specs in scope**: "See also" section at the bottom of every page that has at least one
related page (linked by shared frontmatter tags)
**Sub-specs deferred**: `tag:` search syntax (sub-spec 3)

## Goal

Surface frontmatter-tag-based relationships between pages by rendering a **See also** section at
the bottom of every page that has related pages. "Related" means: shares at least one frontmatter
tag with the current page, and is not the current page itself.

This sub-spec is template-only. The enricher already computes `EnrichmentData.RelatedDocs` on
every markdown render via `MetaIndex.ByTag`. Sub-spec 2 wires that computed data into the
template chain and renders it.

## Design Decisions

### Section heading

`<h2>` element (sibling of the page H1 in the document outline). Heading text comes from the
locale string `see_also` (default English: `"See also"`).

### Sort

Alphabetical by title, case-insensitive. Sorting happens in the handler before assigning to the
template context (NOT in the enricher) so the enricher stays generic for non-template consumers
(JSON API, MCP).

### Limit

None. The section renders all related pages. Most sites have small per-tag overlaps; pagination
or top-N truncation can be added later if a real site needs it.

### Visibility

Gated by a `see_also` feature flag, default on, per-page overridable via frontmatter
`features: { see_also: false }`. Mirrors the `tag_chips` flag pattern from sub-spec 1.

### Empty state

When `RelatedDocs` is empty (the current page has no tags, or all its tags are unique to the
page), the partial early-returns and renders nothing. No "no related pages yet" placeholder.

### Per-language

Implicit. Each language pipeline already has its own `MetaIndex`. The enricher running through
that index produces language-scoped results without any per-language code in this sub-spec.

### Position in layout

After the markdown content body, before the prev/next page navigation. Final visual order on a
page: `H1 → tag chips → content → see also → prev/next → footer`.

## Architecture

No new architectural layers. Reuses:

- `enricher.findRelatedDocs` (already runs on every enrich call).
- The partial inheritance pattern from sub-spec 1 (default theme partials inherited by all 8
  themes).
- The feature-flag pattern from sub-spec 1 (`config.FeatureEnabled` already supports per-page
  overrides).

## Components

### Code changes (this repo)

| File | Change |
|------|--------|
| `internal/template/renderer.go` (`PageContext` struct) | Add `RelatedDocs []enricher.RelatedDoc` field |
| `internal/server/handler.go` (`serveHTML`) | Sort `enrichment.RelatedDocs` alphabetically by title (case-insensitive) and assign to `PageContext.RelatedDocs` |
| `cmd/gomddoc/build.go` (markdown render path) | Same wiring as the handler — sort and assign before invoking the renderer |
| `cmd/gomddoc/assets/locales/en-US.yml` | Add `see_also: "See also"` |

### New theme assets (default theme, inherited by all 8)

| File | Purpose |
|------|---------|
| `cmd/gomddoc/assets/themes/default/partials/see-also.html.tmpl` | Renders the `<section class="see-also"><h2>See also</h2><ul>…</ul></section>` markup, gated by `see_also` feature flag |
| `cmd/gomddoc/assets/themes/default/layouts/default.html.tmpl` (modified) | Insert `{{ template "see-also" . }}` after content body, before the prev/next nav block |

### Coordinated change (separate repo)

The 7 themes in `gomddoc-themes` each add the same `{{ template "see-also" . }}` invocation
to their main layout in the same position. Single shared partial, no per-theme styling needed.

## Data Flow

```
HTTP request → Handler.ServeContent
  → enricher.Enrich(ctx, content, path)
       → findRelatedDocs(path, meta) populates EnrichmentData.RelatedDocs
  → serveHTML(w, r, content, enrichment)
       → slices.SortFunc(enrichment.RelatedDocs, byTitleCaseInsensitive)
       → assign to PageContext.RelatedDocs
       → renderer.Render(...)
             → layout calls {{ template "see-also" . }}
             → partial gates on .Feature "see_also" and non-empty list
             → emit <section><h2 id="see-also-heading">See also</h2><ul>…</ul></section>
```

Build mode follows the same path through `cmd/gomddoc/build.go:buildFile`. Same enricher, same
renderer, same partial.

## Edge Cases

| Case | Behavior |
|------|----------|
| Page with no `tags:` frontmatter | `findRelatedDocs` returns nil → partial renders nothing |
| Page has tags but none are shared with other pages | Same as above — empty list, no section |
| `see_also: false` per page or site | Partial's `{{ if .Feature "see_also" }}` guard skips rendering |
| Current page would appear in its own results | Enricher already filters via the `seen` map keyed on `currentPath` |
| Pages with `robots: noindex` appear in related list | **Show them** — `noindex` is a search-engine signal; users navigating between related pages should still see them. Same rationale as tag listings in sub-spec 1 |
| Excluded paths (`exclude:` glob match) | Already absent from the metadata index, so they're never returned by `ByTag` |
| Concurrent requests | Stateless given an immutable `MetaIndex` |
| Title fallback | Enricher pulls title from frontmatter; if absent, the metadata index's `DeriveTitle` fallback applies, so `RelatedDoc.Title` is always non-empty |

## Section Decorations

- `<h2>` heading element (sibling of H1 in outline).
- `id="see-also-heading"` on the heading, with `aria-labelledby` on the parent `<section>` for
  accessibility. Not used as a deep-link target in v1.
- Inline CSS uses `--color-border` and `--color-primary` theme variables (same vars already used
  by the tag-chips partial), so the section themes correctly in light and dark modes.

## Testing

| Layer | Coverage |
|-------|----------|
| `internal/template/renderer_test.go` | New `TestRender_RelatedPages` covering: section renders for non-empty list, empty list omits section, feature flag off omits section, alphabetical order verified |
| `internal/server/handler_test.go` (or new) | Integration test: GET a markdown page with tags shared by another page → response includes `<section class="related-pages">` with the expected link |
| `cmd/gomddoc/build_test.go` | Assert build output for a tagged page includes the see-also section |
| Existing enricher tests | Already cover `RelatedDocs` population — no changes needed |

Coverage target: maintain 86%+ overall.

## Rollout

1. **This repo PR** — `PageContext` field, handler wiring, partial, layout integration, locale
   string, tests, roadmap update.
2. **`gomddoc-themes` coordinated PR** — add `{{ template "see-also" . }}` invocation to
   each of the 7 other themes' main layouts at the same position.
3. **`gomddoc-website` PR** — document the `see_also` feature flag and the partial in the theming
   guide.

## Out of Scope (Deferred)

- Tag-overlap relevance ranking (option B in clarifying questions).
- Relations beyond shared tags (same directory, same author, etc.).
- Per-page limit / pagination on the section.
- Deep-link anchor for the section heading itself.
- `tag:` search syntax — sub-spec 3.
