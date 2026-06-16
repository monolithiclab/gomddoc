# Codebase & Architecture Review: gomddoc

- **Review Date:** 2026-06-11 (14th pass — Fresh Review; see §9). Prior: 2026-04-08 (13th pass).
- **Reviewers:** Claude Architecture Analysis (6 parallel review agents + manual verification)
- **Branch:** main
- **Go Version:** 1.26.1
- **Coverage:** 81.3% aggregate (`go test ./...`); most `internal/*` packages 87–96%, `cmd/gomddoc` 68.4%
- **Source LoC:** ~8,025 (production) + ~19,232 (tests)
- **Latest findings:** §9 (14th pass). Sections 1–8 are the 13th-pass record, retained for history.

---

## 1. Executive Summary

Fresh review with all previously-fixed items discarded. Six parallel agents covered architecture,
code quality, performance, security, test coverage, and documentation. The codebase has a strong
architectural foundation: mature pipeline design, clean interfaces, thorough test coverage, and
well-structured web components. Remaining issues are a mix of performance opportunities,
architectural nits, and test coverage gaps.

### Quality Metrics

| Category     | Score      | Notes                                                        |
| ------------ | ---------- | ------------------------------------------------------------ |
| Architecture | 9.0/10     | Strong layering; minor coupling and exposure issues          |
| Code Quality | 9.0/10     | Clean interfaces; consistent patterns; proper error handling |
| Security     | 9.5/10     | Trusted content model; path traversal guards; auth hardening |
| Testing      | 9.0/10     | 88.6% coverage; minor gaps in cmd/ remain                    |
| Performance  | 8.5/10     | Good caching/pooling; per-request allocations remain         |
| **Overall**  | **9.0/10** | **Solid core; actionable MEDIUM and LOW items remain**       |

---

## 2. Architecture Issues

### LOW: `AllMappings` exposes internal map without defensive copy

`internal/resolve/resolver.go:138` returns the internal `toClean` map directly. The comment warns
callers not to modify it, but this is a fragile contract. Only caller is `build.go:generateExtensionRedirects`.

### LOW: `AllPages` returns shallow clone with shared inner references

`internal/metadata/index.go:186` -- `slices.Clone` copies the slice but not inner `Meta map[string]any`
or `Tags []string` fields. No callers currently mutate, so this is defensive-only.

---

## 3. Performance Issues

### ~~MEDIUM: Navigation `cloneTree` allocates per request~~ FIXED

`internal/template/navigation/navigation.go` — replaced per-request `cloneTree` + `markActive`
with an immutable cached tree. `Generator.Tree()` is a pointer load (~2 ns, 0 allocs).
Active/Open marking moved to `cmd/gomddoc/pipeline.go:buildNavItems`, which walks the cached
tree and emits `[]enricher.NavItem` in a single pass. For a 1000-page site the per-request
allocation count drops from ~2200 to ~50 (~40× reduction).

### ~~MEDIUM: Navigation prev/next is O(n) per request~~ FIXED

`cmd/gomddoc/pipeline.go:prevNextBuilderAdapter` now calls `Generator.PrevNext`, which is an
O(1) map lookup against a flattened page index built once during cache warm-up. Benchmarks:
0 allocs/op, ~55 ns/op regardless of site size (50/200/1000 pages all measure the same).

### ~~LOW: `seo.PageURL` calls `url.Parse` on every invocation~~ FIXED

`internal/seo/url.go` — added a single-entry `atomic.Pointer[parsedDomain]` cache. The first
call parses, subsequent calls return a value-copy of the cached `url.URL`. Benchmarks: 275 ns/4
allocs → 113 ns/2 allocs. Degrades gracefully to per-call parse if the domain string changes.

### ~~LOW: Sequential static asset copying in build~~ FIXED

`cmd/gomddoc/build.go:copyStaticAssets` — first walks to collect paths, then runs copies through
an `errgroup` sized to `runtime.NumCPU()`, mirroring the markdown build pattern.

### ~~LOW: `OverlayFS.Open` logs at DEBUG level on every file open~~ FIXED

`internal/provider/overlay_fs.go` — removed the success-path debug log and the per-iteration
non-ErrNotExist debug log on Open. Kept the original control flow.

### ~~LOW: `build.go` reads entire files into memory for static asset copying~~ FIXED

`cmd/gomddoc/build.go:streamOutputFile` — new streaming helper opens the source FS file and
`io.Copy`s straight to disk, avoiding the prior `fs.ReadFile`-into-`[]byte` round-trip.
`writeOutputFile` and `streamOutputFile` share path validation via `resolveOutputPath`.

### LOW: Navigation `extractTitle` opens every markdown file on first request

`internal/template/navigation/navigation.go:180` -- for every markdown file, opens and scans for
a heading. The metadata index already has titles and could be reused.

### ~~LOW: `extractFirstHeading` converts `[]byte` to `string`~~ FIXED

`internal/search/index.go:extractFirstHeading` — switched to `bytes.SplitSeq` and `bytes.TrimSpace`,
allocating only the final returned title string instead of a full content copy.

### LOW: Double markdown parsing for enrichment + rendering

Each markdown request is parsed twice: once by the enricher for metadata/TOC extraction and once
by the renderer for HTML output. The two goldmark instances have different extensions, so AST
sharing would require architectural changes. Not a bug, but a performance opportunity.

---

## 4. Testing Issues

### ~~MEDIUM: `loadAuthStore()` untested (security-relevant)~~ FIXED

`cmd/gomddoc/serve.go:93-105` -- added tests covering valid entries, multiple entries with
comments, invalid format, unsupported hash, and nonexistent file.

### ~~MEDIUM: `admin.go` Start/Shutdown at 0% coverage~~ FIXED

`internal/server/admin.go:68,83` -- added tests for Start with context cancellation (graceful
shutdown) and standalone Shutdown.

### ~~LOW: `metadata.ByPath()` untested~~ FIXED

`internal/metadata/index.go:201` -- added tests covering existing path, nested path, nonexistent
path, and copy-vs-reference semantics.

### LOW: `cmd/gomddoc` overall coverage improved

Package-wide coverage improved. Remaining uncovered areas: parts of `generateRedirectFiles`,
preview command's `$BROWSER` handling.

---

## 5. False Positives & Won't-Fix

Items below were raised during reviews but are not genuine issues. Documented here to prevent
re-raising in future reviews.

### Won't fix: LOW: `HTMLRenderer.Configure` is public and allows post-construction mutation

`internal/template/renderer.go:143-147` -- the public `Configure` method invites misuse after
construction. Currently called only from `pipeline.go` before the server starts, so no race
exists. Consider making it package-private or documenting it as startup-only.

**Won't fix**: internal only.

### Won't-Fix: HTTP write errors swallowed

Standard Go pattern -- `http.ResponseWriter.Write` errors indicate client disconnection. Nothing
actionable server-side. All handlers follow the same pattern.

### Won't-Fix: OverlayFS loop duplication

`internal/provider/overlay_fs.go` -- `ReadDir` and `Open` have similar loop patterns but different
return types (`fs.DirEntry` vs `fs.File`). Abstracting would obscure rather than clarify.

### Won't-Fix: `for i := range t.NumField()` is "non-idiomatic"

This is perfectly idiomatic Go 1.22+ range-over-int. No change needed.

### False Positive: `NormalizeMimeType` called twice per request

The two calls normalize different values (provider MIME type vs Accept header value). Not
redundant.

### False Positive: Handler naming inconsistencies

Go HTTP handlers in `server/` follow standard library naming conventions. No issue.

### False Positive: Server package coupling

For a project of this size, the `server` package appropriately groups HTTP concerns. Splitting
would add unnecessary indirection.

### False Positive: `$BROWSER` env var in preview command

The preview command runs locally on the developer's machine. `$BROWSER` is standard Unix
convention (`xdg-open` fallback). Not a server-side security concern.

### Won't-Fix: `collectEnvVars` and `walkStruct` duplication

`internal/config/config.go` -- both functions walk struct fields using reflection with similar
traversal logic. Already evaluated: too many edge cases for too little gain.

### Won't-Fix: `FindAvailablePort` has TOCTOU race

`internal/server/port.go:19-26` -- port found available, listener closed, caller binds later.
Development tool only; will address if it causes real issues.

### Won't-Fix: `OverlayProvider.RootFS` creates new `OverlayFS` on every call

`internal/provider/overlay.go:71` -- cannot be cached because the primary provider's `RootFS()`
may return different FS instances over time (e.g., git provider updates).

### Won't-Fix: `cases.Title(language.English)` allocated per call

`internal/text/title.go` and `internal/text/text.go` -- creates a new `cases.Caser` on every
invocation. Very lightweight object, not worth `sync.Pool` complexity.

### Won't-Fix: `contentURL` resolver lookup is O(1) map access

Map lookup for extensionless URL resolution. Negligible cost.

### Won't-Fix: Retry-capable cache (mutex vs sync.Once)

`SitemapHandler` and `FeedHandler` use `sync.Mutex`-guarded lazy init intentionally -- if
generation fails, subsequent requests retry. This is the desired behavior.

---

## 6. i18n Simplification Review (2026-04-10)

Full-codebase review after i18n/l10n implementation. Three parallel review agents (code reuse,
code quality, efficiency) identified issues. Items marked FIXED were addressed in this pass.

### FIXED: Duplicated i18n helpers between build.go and server.go

`buildLanguageInfos`, `withActiveLang`, and `makeTFunc` were duplicated as private functions
in `server.go` and inlined in `build.go`. Moved to shared locations:
- `template.BuildLanguageInfos()` and `template.WithActiveLang()` in `internal/template/renderer.go`
- `locale.Bundle.TFunc(lang)` method in `internal/locale/bundle.go`
- Removed 43 lines of duplicate code from `server.go`; replaced inline copies in `build.go`

### FIXED: Parameter sprawl in build functions (13-param `buildFile`)

Introduced `buildContext` struct in `build.go` holding shared dependencies (registry,
enricherRegistry, templateRenderer, siteConfig, bundle, languageInfos, lang, tFunc). Reduced
`buildFile` from 13 to 9 parameters. Eliminated `walkAndBuild` and `walkAndBuildLang` wrapper
functions that existed solely to pass through parameter lists.

### FIXED: Dead `ExtractLangFromPath` call in build

`build.go:buildFile` called `locale.ExtractLangFromPath` per file, but the sub-FS was already
scoped to the language directory, so it always returned empty. Language is now set once on
`buildContext` per walk, not re-derived per file.

### FIXED: Per-file `LanguageInfo` clone in build

`buildFile` cloned and mutated the `languageInfos` slice per file to set the Active flag. Since
the language is constant within a `walkAndBuildToDir` call, the Active-flagged copy is now
pre-computed once on `buildContext`.

### LOW: SitemapHandler/FeedHandler caching pattern duplicated

`internal/server/sitemap.go` and `internal/server/feed.go` share identical `sync.Mutex + cached
[]byte + getOrGenerate()` pattern. Could extract a `lazyGenerator` helper.

### ~~LOW: `writeOutputFile` calls `filepath.Abs` per file~~ FIXED

`cmd/gomddoc/build.go:resolveAbsOutput` — `BuildCmd.absOutput` is now memoized via `sync.Once`,
populated once on entry to `Run` (and lazily on first call when tests construct `BuildCmd`
directly without going through `Run`). Validation and `os.MkdirAll` are shared between
`writeOutputFile` and the new `streamOutputFile` via `resolveOutputPath`.

### ~~LOW: `inlineJSAsset`/`inlineCSSAsset` reads files on every template execution~~ FIXED

`internal/template/inline_asset.go:readAsset` — added a per-renderer `sync.Map` keyed by asset
name. Caching is gated on `cacheAssets` (set by `WithCache`) so dev/preview mode still reloads
assets from disk on every render. `ClearCache` invalidates the asset cache alongside the
template cache.

### LOW: Navigation `extractTitle` scans files when metadata index has titles

`internal/template/navigation/navigation.go:extractTitle` opens and scans every `.md` file for
`# ` headings during nav tree build. The metadata index already has titles. Accept an optional
title lookup function backed by `metadata.Index`, falling back to file scan.

---

## 7. Recommendations (Priority Order)

1. ~~**Pre-compute flattened page list for prev/next**~~ FIXED (see §3).
2. ~~**Compute navigation IsActive/IsOpen without cloning**~~ FIXED (see §3).
3. **Address remaining LOW items** as opportunity permits.

---

## 8. Conclusion

The `gomddoc` codebase is well-structured with mature pipeline design, clean interfaces, and
strong test coverage (86.8%). The architecture scores reflect a clean, idiomatic Go codebase with
proper layering and error handling. Both MEDIUM navigation perf items are now fixed (immutable
cached tree, O(1) prev/next). The i18n implementation was cleaned up to eliminate duplicated
helpers, parameter sprawl, and dead code paths. Remaining LOW items are genuine but low-impact
improvements that can be addressed opportunistically.

---

## 9. 14th Pass — Fresh Review (2026-06-11)

- **Reviewers:** 6 parallel review agents (performance, security, code quality, duplication &
  consistency, test coverage, documentation drift) + manual adversarial verification of every
  HIGH/contested finding against `main`.
- **Coverage:** 81.3% aggregate (`go test ./...`), pulled down by `cmd/gomddoc` (68.4%) and the
  0%-counted `internal/testutil/*` helper packages. Most `internal/*` packages remain 87–96%.
- **Scope:** focused on feature work landed since the 13th pass (tag components, see-also/related,
  i18n/l10n) that had never been reviewed.

> **Verification note:** Findings below were confirmed by reading the actual code (both sides of
> each claim). One agent-reported HIGH was a fabricated false positive — see §9.6. Trust the
> verdicts here over raw agent output.

### 9.1 HIGH — Correctness

#### HIGH: Per-language content pages 404 in `serve` mode (i18n content tree non-functional)

`internal/server/server.go:211-218` registers the per-language content handler under
`auth.Subgroup("/"+lang, ...)` **without `http.StripPrefix`**. `RouteGroup.Subgroup`
(`internal/server/group.go:29-36`) only concatenates the prefix into the mux pattern; Go's
`ServeMux` does not strip the matched prefix. So `Handler.ServeContent` (`handler.go:95`) receives
the full path and calls `h.provider.ReadFile(ctx, "/fr-FR/page.md")` — but `h.provider` is the
language pipeline's provider, already rooted at the `fr-FR/` subdir (`pipeline.go:127-133` via
`fs.Sub`). The lookup becomes `fr-FR/fr-FR/page.md` → `ErrNotFound`. The resolver fallback
(`handler.go:99`) uses the **default-language** resolver (`opts.Resolver`), which has no
language-prefixed entries, so it also misses. Result: every non-default-language content page
returns an error. `h.lang` is used only for the i18n template context (`handler.go:197`), never for
path resolution.

- **Why tag routes pass and hid this:** the per-language tag tests
  (`tags_routes_test.go:TestServer_TagRoutes_PerLanguage`) exercise `/fr/tags/...`, which is served
  by the tag handlers off the **metadata index**, not the provider — so they never touch the broken
  content path. There is **no per-language content-serving test** (`language_test.go` covers only
  `ResolveAPILanguage`/`parseAcceptLanguage`).
- **serve/build divergence:** `build` mode walks the language sub-FS directly
  (`walkAndBuildLang`) and writes `fr-FR/…` outputs, so the static build produces correct localized
  pages while the live server 404s them.
- **Fix:** strip the `/{lang}` prefix before the language handler (e.g. wrap with
  `http.StripPrefix("/"+lang, …)`), and build a per-language resolver in each language pipeline
  instead of sharing the default one. Add an integration test that serves a file from a `ll-CC/`
  directory in both serve and build modes.
- **Confidence:** high (verified end-to-end against the code and the routing model).

### 9.2 MEDIUM — Consistency / serve-build parity

- **See-also links emit raw `.md` paths** — `cmd/gomddoc/assets/themes/default/partials/see-also.html.tmpl:8`
  renders `<a href="{{ $doc.Path }}">` where `$doc.Path` is the metadata index's raw path
  (`/foo.md`, set in `internal/enricher/markdown.go:161-164`). Every other link type is
  extension-normalized (nav via `resolver.CleanPath`, prev/next via `canonicalURL`). On a
  `strip_extensions` site the see-also section links to `/foo.md` (an extra 301 hop in serve;
  fragile in build) while the rest of the page links to `/foo`. Fix in the template via the existing
  `contentURL` func, or normalize `RelatedDoc.Path` in the enricher. Affects both serve and build.
- ~~**`serveHTML` and `buildFile` duplicate the entire `TemplateContext`/`PageContext` assembly**~~
  FIXED — extracted `tmpl.BuildPageContext(...)` (`internal/template/context.go`). `serveHTML`
  (`handler.go`) and `buildFile` (`build.go`) now both feed `PageContextInput`; default-title
  fallback, `MergeFeatures`, all `PageContext` fields, and `WithI18n` live in one place. New page
  fields can no longer diverge. Unit-tested in `context_test.go`. (The RelatedDocs sort noted here
  had already moved to the enricher in §9.4.)
- ~~**Error-page rendering duplicated**~~ FIXED — extracted `tmpl.BuildErrorContext(...)`
  (`internal/template/context.go`), used by `handler.go` and `build.go`. The parity gap is closed:
  build now passes `bc.languageInfos` (was `nil`), so the static 404 carries the language switcher
  like the live server.
- **`sitemap-index.xml` built with manual `strings.Builder`** — `cmd/gomddoc/build.go:260-273`
  string-concatenates `<sitemapindex>` with a hard-coded `https://`, while
  `internal/server/sitemap.go` and `feed.go` use `xml.MarshalIndent`. Violates the CLAUDE.md rule
  against constructing markup with `fmt`/string-building, and bypasses `seo.PageURL` scheme
  handling. Define a `sitemapIndex` struct and marshal it.

### 9.3 MEDIUM — Performance (recently-landed code)

- **Tag pages re-parse their partial template on every request** — `executePartial`
  (`internal/template/renderer.go:394-410`) calls `template.New(...).Funcs(...).ParseFS(...)`
  unconditionally, including in production. `RenderTagPage`/`RenderTagsIndex` route through it on
  every `/tags/` and `/tags/{tag}` hit, bypassing the `TemplateCache`+`singleflight` that the main
  content path uses. Also returns a non-pooled `bytes.Buffer`. A clear regression specific to the
  tag feature — cache the parsed partial (key by `theme/name`). _(Independently found by two
  agents.)_
- **`TagsIndexHandler` allocates a full `[]PageInfo` per tag just to count** —
  `internal/server/tags_html.go:65-68` calls `len(h.index.ByTag(tag))`, and `ByTag`
  (`internal/metadata/index.go:228-237`) allocates+copies a slice of every matching page only for
  its length. Add `CountByTag(tag) int { return len(idx.byTag[strings.ToLower(tag)]) }`.
- **`buildNavItems` deep-copies the whole nav tree per content request** —
  `cmd/gomddoc/pipeline.go:265-289` allocates a fresh `NavItem` for every node in the site nav on
  each request just to flip `Active`/`Open` on the current path. O(total pages) allocations per
  page view — the largest per-request allocation after rendering itself. Only the active spine
  changes between requests. _(Medium confidence — correctness-preserving refactor; extend the
  existing navigation benchmark to cover the adapter.)_

### 9.4 LOW

- **`pageTags` returns frontmatter tags verbatim** — `internal/template/renderer.go:691-715` skips
  the lowercase/trim/slash-reject/dedup that `metadata.pageFromFrontmatter` applies
  (`index.go:160-184`). Chips therefore display original case/whitespace and can show slash-tags the
  index dropped. Links mostly still resolve because `ByTag` lowercases its lookup; the genuine
  breakage is slash-tags (chip links to a contentless tag page → 404) and cosmetic display drift.
  Expose `metadata.NormalizeTags(...)` and reuse it in `pageTags` (and `findRelatedDocs`).
- **`findRelatedDocs` has no result cap** — `internal/enricher/markdown.go:130-168`; a page with a
  very common tag renders every co-tagged page into the see-also section. Cap to top-N.
- **`RelatedDocs` sort duplicated + reimplements `CompareTitles`** — identical inline
  `slices.SortFunc(...ToLower...)` in `handler.go:179-181` and `build.go:444-446`; the canonical
  comparator is `metadata.CompareTitles` (`index.go:241`). Note `CompareTitles` itself lacks a path
  tiebreaker, so tag-page ordering is unstable for equal titles (the search path adds
  `cmp.Compare(a.Path,b.Path)` — the tag/HTML paths don't). Unify and add a secondary key.
- **`NewHTTPServer` dereferences `LocaleBundle` unconditionally in the per-language loop** —
  `server.go:195` (`opts.LocaleBundle.TFunc(lang)`) while the default path guards `!= nil`
  (`server.go:222`). Latent nil-panic if a caller sets `LangPipelines` without a bundle.
- **Search query truncated on a byte boundary** — `internal/server/search.go:52-54`
  (`q = q[:maxQueryLength]`) can split a UTF-8 rune. Truncate on a rune boundary.
- **`headingLevel` indentation check ignores tabs** — `internal/mcp/section.go:60-63` trims only
  spaces; comment and code disagree on tab handling. Author-controlled content, low impact.

### 9.5 Test coverage gaps (verified)

- ~~**HIGH risk: multi-language `build` mode is entirely untested**~~ FIXED —
  `TestBuildCmd_Run_MultiLanguage` (`cmd/gomddoc/build_test.go`) creates a `fr-FR/` BCP-47 directory
  and asserts per-language content, 404, sitemap, feed, tag pages, and root `sitemap-index.xml`.
  **The test immediately caught a real bug** that §9.7 had dismissed as a low-priority "exception":
  `fs.Sub(os.DirFS(dir), lang)` is **not** `fs.StatFS` (os.dirFS has no `Sub` method), so every
  per-language pipeline failed to build and all per-language sitemap/feed/tag output was silently
  skipped. Fixed by relaxing `NewFilesystemProviderFromFS` to accept any `fs.FS` (see §9.7).
- ~~**HIGH risk: multi-tag `tag:` AND-intersection untested**~~ FIXED —
  `TestSearch_MultiTagAndIntersection` (`internal/search/index_test.go`) exercises `Search()` with
  two and three `tag:` filters (narrowing) plus a second unknown tag (collapse-to-empty), covering
  the `taggedPages` intersection loop.
- ~~**MEDIUM: `generateRedirectFiles` body untested**~~ FIXED —
  `TestBuildCmd_GeneratesRedirectFiles` writes `redirect_from` frontmatter and asserts each source
  becomes a `source/index.html` meta-refresh redirect to the canonical target.
- LOW: tag HTML handler 500 branches (`tags_html.go`); `emitTagPages` lang-prefix branch (now
  covered by the multi-language-build test above).

### 9.6 Documentation drift (verified) — ALL FIXED

- ~~**HIGH: `CLAUDE.md` "Project Structure" omits 5 real packages**~~ FIXED — added `locale`, `mcp`,
  `resolve`, `seo`, `testutil` to the structure tree (now alphabetized) and corrected the
  `cmd/gomddoc/` line to list all six subcommands (build, info, init, mcp, preview, serve).
- ~~**MEDIUM: `make run` documented without its argument**~~ FIXED — `CLAUDE.md` now shows
  `go run ./cmd/gomddoc serve testsite`, matching the Makefile.
- ~~**MEDIUM: shipped features undocumented in the guide**~~ FIXED — added a "Tag Filters" section to
  `10-search.md` and the `tag:` note to the `/api/search` reference; documented the HTML `/tags/` and
  `/tags/{tag}` pages in `03-api-reference.md`; documented the `tag_chips`/`see_also` partials,
  `pageTags`/`tagURL` functions, and `.Page.RelatedDocs`/`PrevPage`/`NextPage` in
  `05-theming-and-assets.md`; and added a "Tags and Discovery" section to `06-writing-workflow.md`.
- ~~**MEDIUM: `decisions.md` contradicts shipped reality**~~ FIXED — removed the stale "i18n —
  Deferred" row and updated the related-pages/`tag:` note to record that both have since shipped.
- ~~**LOW: "ships with 8 themes"**~~ FIXED — `05-theming-and-assets.md` now states only `default` is
  embedded in the source tree and documents the `gomddoc-themes` overlay mechanism for the other 7.
- _Verified accurate (no drift):_ `02-configuration.md` (all CLI flags/env/config fields incl.
  `--domain`, `exclude`, `strip_extensions`, `robots`, `language`, `redirect_from`), `04-mcp.md`
  (exact 6 tools / 4 resources / 3 prompts), `architecture.md` (resolve, MCP, i18n), Makefile
  targets in `CLAUDE.md`.

### 9.7 ~~False positive~~ — CORRECTION: the "exception" was a real bug (FIXED)

- **"Multi-language pipelines are silently dropped because `fs.Sub` isn't `fs.StatFS`"** — the
  verification dismissed this as FALSE on the assumption that `fs.Sub(os.DirFS(dir), lang)` returns
  the underlying `os.dirFS` via a `SubFS` optimization. **That assumption was wrong.** `os.dirFS`
  does **not** implement `fs.SubFS` (it has no `Sub` method), so `fs.Sub` returns a generic wrapper
  that is **not** `fs.StatFS`. `NewFilesystemProviderFromFS` then rejected it with "filesystem does
  not support Stat", so every per-language pipeline silently failed (`WARN Failed to create provider
  for language`) and per-language sitemap/feed/tag output was skipped. The serve-mode tag tests
  passed only because they feed `fstest.MapFS` differently, masking the real-FS path.
- **Fix:** `NewFilesystemProviderFromFS` now accepts any `fs.FS` (field `root fs.FS`, no StatFS
  assertion). The provider already routed every stat through the package-level `fs.Stat(f.root, …)`,
  which works on non-StatFS filesystems, so the assertion was both unnecessary and the sole blocker.
  Regression-tested by `TestBuildCmd_Run_MultiLanguage` (§9.5). _(The §9.1 prefix-strip breakage was
  a separate, also-real issue — both are now fixed.)_

### 9.8 Still-open items carried from earlier passes (re-confirmed against `main`)

- **`extractTitle` opens every markdown file on first nav build** — `navigation.go:195` still scans
  files for `# ` headings instead of reusing metadata-index titles. Highest-value carried LOW.
- **`AllMappings` returns the internal map without a defensive copy** — `resolve/resolver.go:138`.
- **`AllPages` shallow clone shares inner references** — `metadata/index.go`.
- **SitemapHandler/FeedHandler caching pattern duplicated** — `server/sitemap.go` + `feed.go`
  (`robots.go` is a third, eager variant — three caching idioms for three static endpoints).
- **Double markdown parsing for enrichment + rendering** — architectural; a bounded response-body
  cache keyed by `(path, Accept, lang)` would also remove per-hit re-render/recompress for this
  read-heavy, immutable-content server (perf opportunity, not a defect).

### 9.9 Security — clean

The security agent traced every request-driven input path (content, search, metadata, tag, MCP,
assets, auth) and found **no HIGH/MEDIUM issues** within the trusted-content model. Path traversal
is blocked by the `io/fs` containment model plus `IsHiddenPath` (rejects any `..` segment); body
endpoints use `MaxBytesReader`; search query/limit are capped/clamped; excluded/hidden paths return
404 (no existence leakage); auth uses constant-time bcrypt with a dummy-hash timing defense; the
crafted-`{lang}` route cannot escape the content root (`fs.Sub` enforces `fs.ValidPath`). Two
optional LOW hardening items: add an `fs.ValidPath` guard in the git provider's `normalizePath`
callers (`provider/git.go:367,486`) for non-HTTP robustness, and cap the `{tag}` route / MCP string
inputs for consistency with the search handler.

### 9.10 Recommended priority order

1. **Fix §9.1** (i18n per-language content 404) — a shipped feature is non-functional in serve mode;
   add the missing serve+build integration tests (§9.5) alongside the fix.
2. **See-also `.md` links + `executePartial` re-parse** (§9.2, §9.3) — small, high-value correctness
   and perf fixes in the new tag/see-also code.
3. **Documentation drift** (§9.6) — cheap, and §9.1's existence shows the docs oversold i18n.
4. **Remaining MEDIUM consistency** (shared context/error builders, sitemap-index marshalling).
5. **LOW items and carried items** (§9.4, §9.8) as opportunity permits.
