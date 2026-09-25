# Theme Manifest (`theme.yml`) — Parked

**Status**: Parked 2026-09-23, not scheduled; still parked as of 2026-09-25 (no `theme.yml`, no `internal/theme`
package, roadmap item "Theme manifest — parked" open). Do not implement until the open questions below are settled
with the developer. `gomddoc doctor` (`2497403` and earlier) has since shipped theme checks on top of the template
scan; see Divergences.
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

## Divergences from implementation

Changes since this was parked that bear on the Problem and the open questions:

- There is no `ValidateFeatureKeys` any more. The key-shape check (`^[a-z][a-z0-9_]*$`) is part of
  `Config.ValidateAll` (`21f487c`), and it still checks shape only.
- `gomddoc doctor` now catches typos by template scan: `theme.unknown-feature` (warning) for a `theme.features` key or a
  page `features:` key the active theme never reads, and `theme.unknown-var` (info, shown with `-v`) for a
  `theme.vars` key the theme's CSS never reads. `features: {serach: false}` is still accepted at runtime, but doctor
  reports it. What the manifest would still add is descriptions and defaults.
- `template.ThemeFeatures` scans the templates the renderer resolves: the theme's own, the default theme's partials it
  inherits, site partials under `.gomddoc/partials/`, and the default layout it falls back to (`f1173ea`). This is the
  scan's answer to question 4 (a partial override contributes its feature calls; inherited partials keep theirs). A
  manifest would need a matching rule.
- An uninstalled theme is reported as `source: "fallback-default"` with the default theme's names, and doctor reports
  `theme.not-installed`.
- The JSON Schema is still static (question 2): `theme.features` accepts any key matching the pattern with a boolean
  value, and its description points to the capabilities report for the active theme's keys. `theme.features` is still
  `map[string]bool` (question 3).
