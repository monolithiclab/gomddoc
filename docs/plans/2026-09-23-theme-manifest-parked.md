# Theme Manifest (`theme.yml`) — Parked

**Status**: parked 2026-09-23, not scheduled. Do not implement until the open questions below are settled with the
developer.
**Context**: `docs/specs/2026-09-23-self-documentation-design.md` — the self-documentation sub-spec uses a
best-effort template scan (`template.ThemeFeatures`) for theme facts in the meantime.

## Problem

A theme defines its own feature keys (`{{ .Feature "toc" }}`) and CSS variables (`var(--theme-primary)`), but
nothing declares them. Consequences:

- `ValidateFeatureKeys` checks key shape only, so `features: {serach: false}` is accepted silently.
- Agents (and `gomddoc doctor`) can learn feature *names* by scanning templates but not what they do or what they
  default to.
- `theme.vars` is a free-form map; a typo'd var name is silently ignored by the CSS.

## Candidate design (as drafted during brainstorming)

- A `theme.yml` next to a theme's `layouts/`:

  ```yaml
  features:
    toc: {description: "Right-hand table of contents with scroll highlighting", default: true}
  vars:
    primary: {description: "Accent colour (light mode)", default: "#2563eb", example: "#0f766e"}
  ```

- A new `internal/theme` package: `theme.Load(assetsFS, name)` reads the manifest through the overlay FS, so a
  site-installed theme is covered; falls back to the template scan (names only) when absent.
- A drift test per theme: manifest features == `.Feature` calls in templates; manifest vars == `var(--theme-*)`
  in CSS.
- The default theme ships a manifest; each of the 7 themes in gomddoc-themes gains one (separate commit there).
- Consumers: `capabilities.Report.theme` (`source: "manifest"`), the JSON Schema (`theme.features` gains an enum
  per active theme?), `gomddoc doctor` (unknown feature/var warnings).

## Open questions

1. Is a separate manifest the right home, or should declarations live in the templates themselves (e.g. a
   `{{ define "gomddoc:features" }}` block) so there is no second file to keep in sync?
2. Should the JSON Schema be theme-dependent (enum of the active theme's features), given that `gomddoc schema` is
   otherwise a static document?
3. Do features ever need to be non-boolean (e.g. `toc_depth`)? That changes the manifest shape and
   `theme.features`' type.
4. How does a manifest interact with overriding a single partial of the default theme — does the override inherit
   the default manifest, extend it, or replace it?
5. Versioning: does a manifest declare the gomddoc version range it targets?

## Not in this document

No task list: it gets one when the design is settled and a spec is written.
