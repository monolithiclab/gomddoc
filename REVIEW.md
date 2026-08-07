# Codebase & Architecture Review: gomddoc

- **Review Date:** 2026-07-29 (15th pass — Fresh Review; see §10). Prior: 2026-06-11 (14th pass).
- **Status:** 15th pass **OPEN**. Six parallel agents + independent manual reproduction of every
  HIGH. This pass found the first **HIGH-severity security defect** since the review series began
  (§10.1 exclude bypass) and a cluster of **serve/build output divergences** (§10.2) that make the
  static build materially different from the live server.
- **Reviewers:** Claude Architecture Analysis (6 parallel review agents + manual verification)
- **Branch:** main
- **Go Version:** 1.26 (toolchain 1.26.5)
- **Coverage:** 88.1% product code — **above the 87% target**. `make test` now reports this figure
  directly, and reports the same one in CI: the 82.7% that used to be quoted alongside it was an
  artifact of the untracked `docs/skills/favicons/scripts` package (0%, 148 stmts), which
  `.covignore` now filters out next to `internal/testutil/*`.
- **Source LoC:** ~12,785 (production) + ~27,311 (tests)
- **Latest findings:** §10 (15th pass). Sections 1–8 are the 13th-pass record and §9 the 14th-pass
  record, both retained for history.

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

#### ~~HIGH: Per-language content pages 404 in `serve` mode (i18n content tree non-functional)~~ FIXED

> **FIXED** — the `/{lang}` prefix is now stripped before the language handler and each language
> pipeline builds its own resolver; serve+build integration tests added (§9.5). Fixing this also
> exposed the §9.7 provider bug (`fs.Sub(os.DirFS)` not `fs.StatFS`), now also fixed. Original
> finding retained below for the record.

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

- ~~**See-also links emit raw `.md` paths**~~ FIXED — `see-also.html.tmpl` now wraps `$doc.Path` in
  the `contentURL` func, so related-page links are extension-normalized like every other link type on
  `strip_extensions` sites (no extra 301 hop). Affected both serve and build.
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
- ~~**`sitemap-index.xml` built with manual `strings.Builder`**~~ FIXED — replaced with a
  `sitemapIndex` struct marshalled via `xml.MarshalIndent` (`server.GenerateSitemapIndex`), matching
  the sitemap/feed code path and respecting `seo.PageURL` scheme handling.

### 9.3 MEDIUM — Performance (recently-landed code)

- ~~**Tag pages re-parse their partial template on every request**~~ FIXED — parsed partials are now
  cached (keyed by `theme/name`) in the renderer, so `RenderTagPage`/`RenderTagsIndex` use the same
  cache path as the main content path instead of re-parsing on every `/tags/` hit.
- ~~**`TagsIndexHandler` allocates a full `[]PageInfo` per tag just to count**~~ FIXED — added
  `metadata.Index.CountByTag(tag)`, which returns the bucket length without allocating/copying a page
  slice.
- **`buildNavItems` deep-copies the whole nav tree per content request** (STILL OPEN — deferred) —
  `cmd/gomddoc/pipeline.go` allocates a fresh `NavItem` for every node in the site nav on each
  request just to flip `Active`/`Open` on the current path. O(total pages) allocations per page view.
  Correctness-preserving refactor deferred to a future pass alongside the §9.8 double-parse work.

### 9.4 LOW — ALL FIXED

- ~~**`pageTags` returns frontmatter tags verbatim**~~ FIXED — `pageTags` now applies the same
  lowercase/trim/slash-reject/dedup normalization as the metadata index, so chips no longer drift in
  case/whitespace or link to dropped slash-tags.
- ~~**`findRelatedDocs` has no result cap**~~ FIXED — the related-docs list is now capped to a top-N,
  so a page with a very common tag no longer renders every co-tagged page into see-also.
- ~~**`RelatedDocs` sort duplicated + reimplements `CompareTitles`**~~ FIXED — unified on
  `metadata.CompareTitles`, and the comparator gained a `cmp.Compare(a.Path, b.Path)` secondary key so
  equal-title ordering is now stable across the tag/HTML/search paths.
- ~~**`NewHTTPServer` dereferences `LocaleBundle` unconditionally in the per-language loop**~~ FIXED —
  the per-language loop now guards `LocaleBundle != nil` like the default path, closing the latent
  nil-panic.
- ~~**Search query truncated on a byte boundary**~~ FIXED — query truncation now respects UTF-8 rune
  boundaries.
- ~~**`headingLevel` indentation check ignores tabs**~~ FIXED — the indentation check is now
  tab-aware, matching the comment.

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

### 9.8 Carried items from earlier passes (re-confirmed against `main`)

- ~~**`extractTitle` opens every markdown file on first nav build**~~ FIXED — the nav generator now
  labels leaf pages from the metadata index via a `SetTitleLookup` hook, falling back to a file scan
  only when a page has no indexed title.
- ~~**`AllMappings` returns the internal map without a defensive copy**~~ FIXED — `resolve.Resolver`
  now returns a defensive copy.
- ~~**`AllPages` shallow clone shares inner references**~~ FIXED — `metadata.Index.AllPages` now
  deep-copies the inner `Meta`/`Tags` references.
- ~~**SitemapHandler/FeedHandler caching pattern duplicated**~~ FIXED — extracted a shared `lazyBytes`
  helper for the sitemap/feed static-endpoint caching.
- **Double markdown parsing for enrichment + rendering** (STILL OPEN — DEFERRED) — architectural; a
  bounded response-body cache keyed by `(path, Accept, lang)` would also remove per-hit
  re-render/recompress for this read-heavy, immutable-content server. A perf opportunity, not a
  defect — deferred to a future pass.

### 9.9 Security — clean

The security agent traced every request-driven input path (content, search, metadata, tag, MCP,
assets, auth) and found **no HIGH/MEDIUM issues** within the trusted-content model. Path traversal
is blocked by the `io/fs` containment model plus `IsHiddenPath` (rejects any `..` segment); body
endpoints use `MaxBytesReader`; search query/limit are capped/clamped; excluded/hidden paths return
404 (no existence leakage); auth uses constant-time bcrypt with a dummy-hash timing defense; the
crafted-`{lang}` route cannot escape the content root (`fs.Sub` enforces `fs.ValidPath`). Two
optional LOW hardening items were implemented anyway: an `fs.ValidPath` guard in the git provider's
`ReadFile`/`Stat` for non-HTTP robustness, and length caps on the `{tag}` route / MCP string inputs
for consistency with the search handler. ✅ Both FIXED.

### 9.10 Recommended priority order — COMPLETE

All items below were addressed in the 14th pass (concluded 2026-06-16):

1. ✅ **§9.1** (i18n per-language content 404) — fixed via `StripPrefix` + per-language resolver; serve
   and build integration tests added (§9.5). Exposed and fixed the §9.7 provider bug.
2. ✅ **See-also `.md` links + `executePartial` re-parse** (§9.2, §9.3) — links normalized via
   `contentURL`; tag-page partials now cached.
3. ✅ **Documentation drift** (§9.6) — CLAUDE.md structure/subcommands, guide coverage for
   tag/search/see-also features, decisions.md, and the theme-bundling note all corrected.
4. ✅ **Remaining MEDIUM consistency** — shared `BuildPageContext`/`BuildErrorContext` builders
   (§9.2), sitemap-index marshalling (§9.2), tag counting without allocation (§9.3).
5. ✅ **LOW and carried items** (§9.4, §9.8) — including `extractTitle` reuse of indexed titles,
   defensive copies, and lazy sitemap/feed caching.

**Deferred (not defects) — two perf opportunities for a future pass:**

- `buildNavItems` deep-copies the nav tree per request (§9.3) — O(total pages) allocations per view.
- Double markdown parsing for enrichment + rendering (§9.8) — a bounded response-body cache would also
  remove per-hit re-render/recompress.

---

## 10. 15th Pass — Fresh Review (2026-07-29)

- **Reviewers:** 6 parallel agents (performance, security incl. supply chain, Go idioms/code quality,
  duplication & serve-build parity, test coverage & quality, documentation drift) + independent
  manual reproduction.
- **Scope:** whole codebase, with emphasis on territory never reviewed before — the release and
  distribution pipeline (GoReleaser, `scripts/install.sh`, Dockerfile, GitHub Actions), and the
  git provider's concurrency model.
- **Linters:** `make lint -j8`, `go vet`, `staticcheck`, `gosec`, `gocritic` — **all clean, zero
  findings**. `go test -race ./...` — clean. Every finding below came from manual reading.

> **Verification note:** every HIGH below was reproduced against a running binary or confirmed by
> reading the actual dependency source — not inferred. Findings are marked ✅ *reproduced* where a
> command-line repro exists. Trust the verdicts here over raw agent output.

### 10.1 HIGH — Security

#### ~~HIGH: `exclude` patterns are bypassable via clean URLs — excluded content is served~~ ✅ FIXED

> **Fixed:** `resolve.Build` now takes `resolve.BuildOptions{StripExtensions, Exclude, HasRenderer}`
> and skips hidden/excluded entries via `provider.SkipWalkEntry`, matching its three peer indexes.
> Verified end-to-end: `/TODO` now 404s and the static build emits no `TODO.md`/`drafts/` stubs.
> The two directory/file walks were merged into one in the same change (closes the §10.7 LOW item
> "resolver does two full filesystem walks at startup") — `WalkDir` visits lexically and `cleanPath`
> is always a sibling of `p`, so a directory is always seen before the file that shadows it.
> Regression tests: `resolve.TestResolver_ExcludedPathsHaveNoMapping` (unit, incl. `AllMappings`)
> and `server.TestHandlerResolverHonoursExclusions` (middleware + handler, canary body).

Original finding:

`internal/resolve/resolver.go:30` — `resolve.Build` is the **only** content index not given
`cfg.Site.Exclude`. Its three peers all take it (`cmd/gomddoc/pipeline.go:212,222,238`). So the
resolver maps a clean URL to *every* file, excluded or not.

`ContentExclusion` (`internal/server/middleware.go:94`) only inspects the **incoming** `r.URL.Path`,
and `Handler.ServeContent` (`internal/server/handler.go:98-99`) feeds the resolver's output straight
back into the provider with no re-check. `FilesystemProvider.ReadFile` does not check exclusions for
regular files either. Exclude patterns match the *filename* (`TODO.md`); the served URL is
*extensionless* (`/TODO`); the middleware never fires.

`strip_extensions` defaults to `[".md"]` (`internal/config/config.go:204`) — **this is the default
configuration.** Reproduced with `exclude: ["TODO.md", "drafts/"]`:

| Request | Result |
| ------------------ | -------------------------------------- |
| `GET /TODO.md` | 404 ✅ correctly excluded |
| `GET /TODO` | **200 — full file contents served** ❌ |
| `GET /drafts/plan.md` | 404 ✅ |
| `GET /drafts/plan` | 404 ✅ |

- **Scope:** filename/extension-shaped patterns (`TODO.md`, `*.draft.md`) are bypassable.
  Directory-prefix patterns (`drafts/`) are **not** — the prefix still matches the clean path.
  Hidden (`.`-prefixed) paths are **not** — stripping `.md` from `.secret.md` leaves `.secret`.
- **Why it is worse than a plain leak:** search, metadata and navigation all correctly omit the file
  (`/api/search` for the canary returns `[]`), so the page is invisible in every listing while being
  directly fetchable. The operator gets no signal.
- **Also leaks into `build`** ✅ reproduced — `generateExtensionRedirects` (`cmd/gomddoc/build.go:616`)
  iterates the unfiltered `resolver.AllMappings()` and emits `TODO.md` and `drafts/plan.md` as
  meta-refresh stubs pointing at the live URLs. No content leaks, but the existence and names of
  every excluded file are published — including the `drafts/` case that serve mode blocks.
- **Fix:** thread `cfg.Site.Exclude` into `resolve.Build` and skip excluded files, matching the other
  three indexes. Defence in depth: re-check `IsRestrictedPath(realPath)` in `handler.go` after
  `resolver.Resolve`, and in `FilesystemProvider.ReadFile`.

#### ~~HIGH: shared `*object.Tree` mutated under a read lock — data race, unrecoverable crash~~ ✅ FIXED

> **Fixed:** `gitFSState` was renamed `gitTreeState` and made the *sole* owner of the cached tree,
> behind an exclusive `sync.Mutex`. `GitProvider.tree` is gone — it was a second alias to the same
> mutable object guarded by a second lock (`g.mu`), which is why the two files could not be reasoned
> about together. `ReadFile`, `Stat` and `gitTreeFS.Open` now all take the one lock.
>
> The "stop sharing one tree, re-derive `commit.Tree()` per operation" alternative was tested and
> **rejected**: under `-race` against a packed disk-backed repo it races *earlier*, inside
> `filesystem.ObjectStorage.EncodedObject`, because the storers are not concurrency-safe either. It
> also costs a fresh decode of every tree along the path. Recorded in `docs/decisions.md`
> ("Serialised Git Reads") along with the throughput consequence.
>
> The nil-tree panic below was fixed in the same change. Regression test:
> `TestGitProvider_ConcurrentReadsAreRaceFree` — 40 goroutines over nested paths hitting
> `ReadFile` + `Stat` + `RootFS().Open` on one tree. Confirmed falsifiable: reverting the lock to
> `RWMutex`/`RLock` reproduces `WARNING: DATA RACE ... object.(*Tree).FindEntry()` at
> `tree.go:131/132` (v5.17.2; `:151/152` after the v5.19.1 bump — the memoisation is still
> unsynchronised there). This closes the §10.6 gap "concurrent `ReadFile` is never tested".
>
> Also folded in: `gitDirFile` no longer retains the shared root tree (it materialises its entries
> under the lock), which closes a latent hole where `ReadDir` touched the tree after the lock was
> released and kept the git object graph alive past `Close()`. Its entry-building loop, which had
> drifted into a duplicate of `listDirectoryLocked`, is now the shared `treeDirEntries` helper.

Original finding:

`internal/provider/git.go:372-373,386,393` and `internal/provider/gitfs.go:40,55,61`.

```go
g.mu.RLock()
defer g.mu.RUnlock()
...
file, err := g.tree.File(cleanPath)   // git.go:386
_, treeErr := g.tree.Tree(cleanPath)  // git.go:393
```

Both `File` and `Tree` funnel into `FindEntry`, which **mutates the receiver with no
synchronization** — verified in the pinned `go-git/v5@v5.17.2` source,
`plumbing/object/tree.go:129-133` (`t.t = make(map[string]*Tree)`), `:159` (`t.t[pathCurrent] = tree`),
and `:184-186` → `buildMap()` writing `t.m`.

`RLock` permits concurrent readers, so N simultaneous requests to a git-backed site all write the
same maps. A concurrent map write is a runtime `throw()` — **not recoverable**, and there is no
recovery middleware anywhere in the repo. Agent reproduced with `-race` (64 goroutines, one shared
tree): `WARNING: DATA RACE ... object.(*Tree).FindEntry()`.

CI is green because `TestGitProvider_EnsureCloned_ConcurrentAccess` (`git_test.go:1109`) exercises
only `ensureCloned` and pre-populates `repo`/`tree`, so all 50 goroutines return at the first
`RLock` — a concurrent `ReadFile` is never tested (see §10.7).

- **Fix:** serialize tree access with a full `Mutex`, or stop sharing one tree (re-derive
  `commit.Tree()` per operation).

#### ~~HIGH: `ensureCloned` reports success with a `nil` tree → nil-pointer panic~~ ✅ FIXED

> **Fixed** alongside the tree race. `cloneLocked` now un-publishes `g.repo` (and `g.storage`) if
> `resolveCommitLocked`/`cacheTreeLocked` fails, so the "initialised" predicate can never be true
> without a tree, and the next call retries — matching how a failed clone already behaved. An
> explicit `initialised bool` was considered and rejected: it adds a second field that must be kept
> in agreement with `repo`/`storage`/`tree`, with nothing enforcing it.
>
> The `Close()`-interleaving window is separate and remains reachable, so `ReadFile`/`Stat` also
> re-check `tree != nil` under the tree lock and return `ErrProviderClosed` instead of panicking.
> Regression test: `TestGitProvider_NilTreeIsAnErrorNotAPanic`.

Original finding:

`internal/provider/git.go:195-198,251,254`. `g.repo` is published at `:251` **before** commit/tree
resolution can fail at `:254`, but the liveness predicate is `g.repo != nil`, not `g.tree != nil`.
After any `resolveCommitLocked`/`cacheTreeLocked` failure (bad `#ref`, or `#main:missing-subdir`),
the provider is permanently in a state where `ensureCloned` returns `nil` and `g.tree` is `nil`.
`FindEntry`'s first statement writes `t.t`, so this panics rather than erroring. The same window
exists between `ensureCloned` returning and `g.mu.RLock()` being taken, where `Close()` (`git.go:174`,
`g.tree = nil`) can interleave.

- **Fix:** assign `g.repo` only after the tree is cached; re-validate `g.closed`/`g.tree != nil`
  under the `RLock` in `ReadFile`/`Stat`.

#### ~~MEDIUM: 15 govulncheck-reachable vulnerabilities; no vuln gate in CI~~ ✅ FIXED

> All four modules bumped to their fixed versions (which pulled `sha1cd`, `x/net`, `x/sync` and
> `x/sys` along with them). `govulncheck ./...` now reports **0 affected**; the 8 that remain are in
> required-but-uncalled modules.
>
> The gate is a `vulncheck` Makefile target plus a dedicated CI job, rather than a step bolted onto
> `test`: the scan needs network and a separate tool install, so folding it into the matrix would
> have paid for both twice and coupled a supply-chain signal to a flaky-network test run.
> `make vulncheck` runs the same command locally; `FORCE_UPDATE=1` refreshes the tool (needed after a
> Go toolchain bump — a govulncheck built against an older Go fails to load `std` and reports
> confusing type errors rather than a clean version complaint).
>
> Not added to `make ci`: it would put a network round-trip in the inner edit-test loop.
>
> Original finding:

`govulncheck ./...` → *"Your code is affected by 15 vulnerabilities from 4 modules."*

| Module | Current | Fixed in | Count |
| ---------------------------- | -------- | -------- | ----- |
| `golang.org/x/crypto` (ssh) | v0.49.0 | v0.52.0 | 7 |
| `github.com/go-git/go-git/v5`| v5.17.2 | v5.19.1 | 5 |
| `github.com/go-git/go-billy/v5` | v5.8.0 | v5.9.0 | 2 (chroot escape) |
| `golang.org/x/text` | v0.35.0 | v0.39.0 | 1 |

`x/text` is the only **request-path-reachable** trace (`internal/text/text.go:22 TitleCase →
cases.Caser.String → norm.Form.Properties`). The ssh/go-git ones need a configured git provider —
still real exposure for git-backed deployments. All four are plain `go get` bumps. Dependabot only
landed in `1ac1169`, so this is a one-time catch-up. `.github/workflows/ci.yml:34` runs only
`make lint test` — add a `govulncheck` step or this recurs silently.

#### ~~MEDIUM: admin port serves `/metrics` and `/debug/pprof/*` unauthenticated~~ ✅ FIXED

`internal/server/admin.go:28` — no auth middleware in the constructor, and `--admin-port :18101`
binds all interfaces, not loopback. Running with `--basic-auth-file` does **not** protect it:
`curl http://localhost:18101/debug/pprof/cmdline` returns the full command line including the
credential file path. `/debug/pprof/heap` dumps in-memory content — including excluded documents and
parsed credential material. `/debug/pprof/profile` is a free 30s-CPU-burn DoS primitive.

Real exposure rather than a nicety because the admin port is *documented* as the way to scrape
metrics, so operators will expose it to a metrics network. Minimum fix: default the admin listener
to `127.0.0.1`; gate `--pprof` behind the credential store. Secondary: the admin server sets only
`ReadHeaderTimeout` — no `WriteTimeout`/`IdleTimeout`.

- **Fixed:** all three. `Config.Normalize` rewrites a host-less `--admin-port` to `127.0.0.1:PORT`
  (an explicit host, including `0.0.0.0`, is honoured as the opt-in it is; `AdminPort == Port` is
  left alone, since `server.go` compares the two strings to decide whether admin lives on the main
  listener). `AdminServerConfig` gained `AuthStore`, so `/debug/pprof/*` sits behind the same
  credential store the main port already puts it behind — `/metrics` and `/health/*` stay open,
  because scrapers carry no credentials and loopback is what protects them. Without a credential
  file, `--pprof` now warns about what it is exposing. `AdminServerConfig.HTTP` carries the main
  server's timeouts, so `WriteTimeout`/`IdleTimeout`/`MaxHeaderBytes` are no longer unset.
- **Both listeners now share one mount.** The two ports kept separate copies of the five-route pprof
  block and this fix made them diverge (only one gained the gate). `server.mountPprof` owns the
  routes, the auth gate, and the warning; `ServerConfig.AdminOnMain()` replaces the three hand-copied
  `AdminPort == "" || AdminPort == Port` comparisons that had to agree.
- **Not a problem, verified:** `WriteTimeout` does not truncate `/debug/pprof/profile`. `net/http/pprof`
  extends its own deadline to `WriteTimeout + seconds` (`configureWriteDeadline`, called from
  `Profile`, `Trace`, and the delta path). Measured: `WriteTimeout=30s` + `?seconds=30` returns a
  complete 200 at 30.01s. A `clearWriteDeadline` middleware was written for this and deleted — it ran
  *before* the handler, so the stdlib overwrote it microseconds later, and on the three endpoints
  where the cleared deadline did survive it removed a bound rather than adding one.

##### Follow-ups surfaced while fixing this (not addressed here)

- **LOW — admin shutdown ignores `ShutdownTimeout`.** `HTTPServer.Shutdown` applies
  `cfg.Server.HTTP.ShutdownTimeout`; `AdminServer.Shutdown` does not, and `AdminServer.Start` calls it
  with a bare `context.Background()`. Admin shutdown can block the errgroup indefinitely after the
  main server has already given up.
- **LOW — the admin mux skips `RequestID` and `SecurityHeaders`,** which `server.go` wraps around the
  main mux. A shared listener constructor would settle this along with the timeout copying.
- **LOW — `--port :8080 --admin-port 0.0.0.0:8080` is not string-equal,** so `AdminOnMain` is false and
  a second listener is built on a port already in use. The `bind: address already in use` surfaces only
  after the provider and every index have been built. Comparing canonicalized addresses would fix it.
- **MEDIUM (perf) — bcrypt runs on every authenticated request.** `NewBasicAuthMiddleware` sits on the
  main port's root group, and `CredentialStore.Validate` is 73ms at htpasswd's default cost 10 —
  roughly 13 req/s per core with `--basic-auth-file` set. A verified-credential cache in
  `CredentialStore` (SHA-256 of user:pass for already-proven pairs, `subtle.ConstantTimeCompare`,
  immutable after parse so no TTL) makes the second and later requests ~200ns. Not pprof-relevant
  (one request per profile), but it caps the whole site.

#### ~~MEDIUM: `install.sh` fails open on checksum verification, never verifies the cosign signature~~ ✅ FIXED

`scripts/install.sh:91` — when neither `sha256sum` nor `shasum` is present it `warn`s and
`return 0`s. A `curl | sh` installer should `die`. An attacker who can influence the environment
(minimal container, stripped `PATH`) downgrades to no verification and gets an unchecked binary
installed with `install -m 0755`, sometimes via `sudo` (`:179`).

Compounding: `SHA256SUMS` is fetched from the **same origin** as the archive (`:163-166`), so anyone
who can serve a malicious tarball can serve a matching sums file. The `.sig`/`.pem` that
`.goreleaser.yaml` produces are never downloaded or verified, despite the file header at `:11`
claiming *"Checksums are also cosign-signed"* — which reads as a stronger guarantee than delivered.
Minor: no `--proto '=https' --tlsv1.2` on the `curl` calls (`:38`, `:49`), so `-L` would follow a
redirect to plain HTTP.

Positives: `set -eu`, `mktemp -d` with `trap ... EXIT INT TERM`, strict OS/arch allowlists,
shellcheck gating in CI.

- **Fixed:** the missing-tool branch now `die`s. `verify_signature` downloads `SHA256SUMS.sig`/`.pem`
  and runs `cosign verify-blob` with `--certificate-identity` pinned to this repo's release workflow
  *at the tag being installed*, so a signature lifted from another project or another tag does not
  verify. cosign is opportunistic — absent, the script warns that `SHA256SUMS` shares an origin with
  the archive and continues; `GOMDDOC_REQUIRE_COSIGN=1` makes it mandatory. Present but unverifiable
  is always fatal, since that is the interesting failure. `curl` calls carry
  `--proto '=https' --tlsv1.2` and `wget` carries `--https-only`.
- **Deliberately not fail-closed by default:** requiring cosign unconditionally would break
  `curl | sh` on every machine without it, which is most of them. The header comment and README now
  state plainly that a cosign-less install verifies integrity but not provenance.

#### ~~MEDIUM: release workflow actions pinned to mutable tags while holding `id-token: write`~~ ✅ FIXED

`.github/workflows/release.yml:9` scopes `contents/packages/id-token` exactly right. But every action
is a mutable ref (`actions/checkout@v6`, `setup-go@v6`, `docker/*@v4`, `sigstore/cosign-installer@v3`,
`goreleaser-action@v7` with `version: latest`). Any upstream tag repoint executes attacker code in a
job holding an OIDC token capable of producing **valid Sigstore signatures over arbitrary
artifacts**, plus write access to releases, GHCR, and `HOMEBREW_TAP_TOKEN`. Downstream signature
verification would then succeed on a backdoored binary. Pin to full commit SHAs; pin goreleaser to an
exact version.

- **Fixed:** all 8 action refs across `release.yml` **and** `ci.yml` are full commit SHAs with a
  `# vX.Y.Z` trailing comment. `ci.yml` was not in the finding but shares the mechanism, and leaving
  half the repo on mutable tags makes it unreadable which half is deliberate. `goreleaser-action`'s
  `version: latest` became `"~> v2.17"`, so a goreleaser major cannot land unreviewed in the job that
  holds the OIDC token.
- **Pins will not rot:** `.github/dependabot.yml` already runs the `github-actions` ecosystem weekly,
  and Dependabot rewrites both the SHA and the version comment.

#### MEDIUM: unauthenticated bcrypt CPU amplification

`internal/server/htpasswd.go:53` — the dummy-hash timing defence is correct and worth keeping, but an
unauthenticated attacker controls how often it runs. Measured (cost-10, 100 requests at concurrency
50): bad credentials **0.916s wall, 99% CPU** (~80ms server CPU/request) vs 0.186s with no
`Authorization` header. ~10× amplification, no rate limit, no concurrency cap, no lockout. Consider a
bounded semaphore around `Validate` or per-IP rate limiting on 401s.

#### LOW — security

- `.github/workflows/ci.yml` has **no `permissions:` block** — jobs inherit the default token scope
  on `pull_request`. Add `permissions: contents: read`.
- `FilesystemProvider` lacks the `fs.ValidPath` guard §9.9 added to the git provider
  (`internal/provider/filesystem.go:68-77`). `normalizePath` leaves `../x` intact, `fs.Stat` returns
  `fs.ErrInvalid`, which matches no sentinel in `classifyError` → **500** where the git provider
  returns 404. Not a traversal hole (`io/fs` containment holds) but a half-applied hardening and a
  "consistent behavior across code paths" violation.
- MCP resource/prompt handlers skip the `maxArgLen` cap their tool siblings apply
  (`internal/mcp/resources.go:76,101`, `prompts.go:115`); `search_docs` (`tools.go:109-121`) has no
  query cap at all, unlike the HTTP handler's 500-rune truncation.
- No max-file-size cap in `FilesystemProvider.ReadFile` (git enforces 50MB). Content is
  author-controlled, so not an attack vector — but a stray large file is an unbounded per-request
  allocation. `gitTreeFS` (`gitfs.go:76-88`) **bypasses** the git provider's own 50MB cap and copies
  every blob 2–3× (`Contents()` string → `[]byte` → `io.ReadAll`).
- Supply-chain nicety: `.goreleaser.yaml` signs checksums and Docker manifests but generates no SBOM
  and no SLSA provenance. `before: hooks: - go mod tidy` (`:7`) can mutate `go.sum` during a release
  build; `go mod download` + verify is the reproducible choice.

### 10.2 HIGH — serve/build parity (static builds are materially different)

All four reproduced by building and serving `testsite/` with `meta.domain` set and diffing output.

#### ~~HIGH: `Page.Path` keeps the `.md` extension in build~~ ✅ FIXED

`internal/server/handler.go:167` passes the *request* path; `cmd/gomddoc/build.go:432` passes the
*file* path (`"/" + filePath`). `Page.Path` feeds `canonicalURL`, `og:url`, JSON-LD `@id`,
`breadcrumbs`, `editURL` and hreflang. Measured on `guides/getting-started`:

| | serve | build |
| ------------------- | --------------------------------------- | ------------------------------------------- |
| `rel="canonical"` | `https://example.com/guides/getting-started` | `…/guides/getting-started.md` ❌ |
| `og:url` | `…/getting-started` | `…/getting-started.md` ❌ |
| JSON-LD `@id` | `…/getting-started` | `…/getting-started.md` ❌ |

The **same build** emits `<loc>https://example.com/guides/getting-started</loc>` in its own
`sitemap.xml` (`internal/server/sitemap.go:121` uses `resolvedPagePath`) — so a static site advertises
one URL in the sitemap and a different, self-contradicting canonical on the page. §9.2 extracted
`BuildPageContext` to prevent divergence; the divergence moved into the *value* of `Path`.

- **Fixed:** `(*resolve.PathResolver).PageURLPath(realPath, defaultIndex)` is now the single
  file-path-to-URL derivation, and `buildFile` calls it. The same commit collapsed the three
  pre-existing near-copies onto it — `server.resolvedPagePath` (sitemap), `HTMLRenderer.contentURL`
  (see-also/tag templates) and `navigation.buildTree` — and moved `IsDefaultIndex` from
  `internal/server` to `internal/resolve` so `internal/template` can reach it (server imports
  template, so the dependency cannot run the other way). This one change also fixed the next two
  findings. Rationale and rejected alternatives: `docs/decisions.md` §"One Derivation of a Page's
  URL". Regression guards: `TestResolver_PageURLPath` (derivation matrix) and
  `TestBuildCmd_Run_PagePathIsTheURLPath` (wiring, asserted against real build output).

#### ~~HIGH: prev/next links absent from every static build~~ ✅ FIXED

`cmd/gomddoc/build.go:125` enables navigation explicitly *"needed for prev/next page links"*, but
`Generator.PrevNext` (`internal/template/navigation/navigation.go:98`) looks up
`cachedIndex[NormalizeRequestPath(currentPath)]`, and the index is keyed on **resolver clean paths**.
Build passes `/guides/getting-started.md` → miss → `nil, nil`. Verified: serve renders
`<link rel="prev">` and `<link rel="next">`; the built page renders **zero** prev/next markup. The
feature is silently dead in every static build with `strip_extensions` (the default).

- **Fixed** by the `PageURLPath` change above; `TestBuildCmd_Run_PagePathIsTheURLPath` asserts
  `rel="prev"`/`rel="next"` on a middle sibling in real build output.

#### ~~MEDIUM: sidebar active/open-ancestor state absent from static builds~~ ✅ FIXED

Same root cause — `buildNavItems` (`cmd/gomddoc/pipeline.go:276-300`) compares
`node.CompareClean(currentPath)` against the normalized `.md` path. Verified: serve renders
`<details open>` ×1; build renders 0. Every page in a static build shows a fully-collapsed,
unhighlighted sidebar.

- **Fixed** by the `PageURLPath` change above, asserted in the same build test.

#### MEDIUM: `/sitemap-index.xml` is generated by build but 404s in serve ✅ reproduced

`cmd/gomddoc/build.go:259-266` writes it; `internal/server/server.go:150-182` never registers it.
`GET /sitemap-index.xml` → **404**. Documented as a live endpoint in
`docs/guide/12-advanced/03-api-reference.md:161-164`.

#### MEDIUM: per-language tag pages gated on a domain in build only

`cmd/gomddoc/build.go:236` gates the per-language `emitTagPages` on `cfg.Site.Meta.Domain != ""`;
`internal/server/server.go:177` gates the serve routes on `MetaIndex != nil && LocaleBundle != nil`
with no domain condition. A multi-language site with no domain serves `/fr/tags/` fine but ships a
build with those pages missing and every in-page tag chip pointing at a 404. Sitemap/feed legitimately
need a domain; tag pages do not. (The *default*-language `emitTagPages` at `:188` has no domain check,
so the divergence exists inside build mode too.)

#### MEDIUM: the search UI ships in static builds with no backend

Build runs with `EnableSearch` unset (`build.go:123-127`), but `config.FeatureEnabled` returns `true`
for absent keys (`internal/config/features.go:15-23`), so every built page renders `#search-toggle`
and inlines `search.mjs`, whose only data source is `fetch('/api/search?q=' …)`
(`cmd/gomddoc/assets/shared/search.mjs:248`). Static hosts have no `/api/search` → Ctrl+K opens a
permanently-empty box. Either force `search: false` into merged features during build, or emit a
static index the script can fetch.

#### HIGH: build derives the URL and the output path with two different functions — NEW ✅ reproduced

`PageURLPath` answers "what URL does the **serve** scheme publish this file at". `buildFile`'s
`htmlPath`/`prettyOutputPath` (`cmd/gomddoc/build.go:458-471`, `:612`) independently answers "where
does build **write** it". They take different inputs — `prettyOutputPath` sees `dirsWithIndexMD` and
the `StripExtensions`-empty branch, the resolver sees neither — and diverge in ordinary configs.
Reproduced with `meta.domain: example.com`:

| config | written | `rel="canonical"` | lands on |
| ------------------------------ | -------------------------- | ---------------------- | ----------------------- |
| `strip_extensions: []` | `guides/alpha.html` | `…/guides/alpha.md` ❌ | 404 |
| `strip_extensions: []` | `guides/index.html` | `…/guides/index.md` ❌ | 404 |
| default, `guides/index.md` | `guides/index.html` | `…/guides/index` ❌ | 404 |
| default, `README.md`+`index.md` | `guides/README/index.html` | `…/guides` ❌ | *index.md's* page |

Only the last column differs from a correct build; the §10.2 fix above corrected the common case
(`guides/alpha.md` → `/guides/alpha`) and these are what it does not reach. The clean fix is to
derive the URL *from* `htmlPath` (`x/index.html` → `/x`, `x.html` → `/x.html`), which is provably
consistent because it reads the path actually written — but it exposes a design question the
`Page.Path` commit deliberately did not answer: with `strip_extensions: []`, build renders every
`.md` to `.html` and copies no `.md`, so **build's whole URL space differs from serve's** and the
sitemap (which uses the serve scheme) would still disagree. Either build must force extension
stripping, or it needs a publishing plan that the sitemap/feed also consult.

#### MEDIUM: the language prefix never reaches `Page.Path` in build — NEW

`buildFile` applies `outputPrefix` to `htmlPath` (`cmd/gomddoc/build.go:467-470`) but not to
`pagePath`, so `fr-FR/guides/alpha.md` is written to `/fr-FR/guides/alpha/index.html` while its
canonical says `…/guides/alpha` — a URL that exists only in the default-language tree. Not a
one-liner: `partials/hreflang.html.tmpl:10` renders `href="/{{.Code}}{{$path}}"` and therefore
*requires* `Page.Path` to be language-relative. `Page.Path` is overloaded — "identity within a
language tree" for hreflang, "absolute site URL" for canonical/og/JSON-LD — and one of the two needs
its own field on `PageContext`.

#### MEDIUM: a directory index has two canonical URLs in serve — NEW (found while fixing §10.2)

Serve derives `Page.Path` from the *request*, so `GET /guides` and `GET /guides/` both return 200 and
emit `rel="canonical"` of `…/guides` and `…/guides/` respectively. `Page.Path` in serve is therefore
the request, not a stable page identity — textbook duplicate content. Build now always emits the
no-slash form, agreeing with `sitemap.xml` and with every internal link (`contentURL`,
`navigation.buildTree`). Fix is serve-side: canonicalise before building the page context, or 301 one
form to the other. Pre-existing; unchanged by the `PageURLPath` commit.

#### MEDIUM: `redirect_from` on a default-index page targets a URL that is never written — NEW

`internal/server/redirect.go:39-45` (`BuildRedirectMap`) hand-rolls the clean-path lookup without the
default-index fold, so `docs/README.md` with `redirect_from` produces a redirect to `/docs/README`
while the page's canonical and the sitemap both say `/docs`. In static builds `/docs/README` is never
written (`prettyOutputPath` emits `docs/index.html`, and `generateExtensionRedirects` skips
default-index files), so the redirect stub lands on a 404. `internal/server/redirect.go:99`
(`ExtensionRedirect`) has the same gap. Fix: route both through `resolve.PathResolver.PageURLPath`,
which needs `defaultIndex` threaded into `BuildRedirectMap` (callers: `cmd/gomddoc/pipeline.go:222`,
`cmd/gomddoc/build.go:584`) and into the middleware constructor.

#### LOW: search results link to raw `.md` paths — NEW

`internal/search/index.go:152` stores `"/"+realPath` and `cmd/gomddoc/assets/shared/search.mjs:267`
uses it directly as `href`. Same class as the tag-list finding in §10.3, but the search package has
no resolver dependency, so the mapping belongs in `internal/server/search.go` or at index build.

### 10.3 HIGH/MEDIUM — correctness (affects both paths)

#### ~~HIGH: tag pages link to raw `.md` paths and drop the language prefix~~ ✅ FIXED

`see-also.html.tmpl` does it right (`{{ contentURL $doc.Path }}`, fixed in §9.2).
`cmd/gomddoc/assets/themes/default/partials/tags-list.html.tmpl:9` does not:

```gotemplate
<a class="tag-result-title" href="{{ $page.Path }}">{{ $page.Title }}</a>
```

Verified: `/tags/getting-started` → `href="/README.md"`. On serve every one of these is a 301 hop; in
a static build it lands on a meta-refresh stub. On `/fr-FR/tags/guide` the link is **`/page.md`** —
no `/fr-FR` prefix, because the language index stores paths relative to the language sub-FS, so the
link resolves to the **English** page. `contentURL` is already in the partial func map
(`internal/template/renderer.go:542`) — a one-line fix plus a lang prefix.

- **Fixed:** the partial now renders `{{ contentURL $page.Path }}`, and the language prefix moved
  *into* `contentURL` — `HTMLRenderer` gained a `langPrefix` set by `template.WithLangPrefix(lang)`
  when the per-language pipeline is built (`pipeline.go`). Prefixing in the template was the first
  attempt; it was rejected because it makes every partial responsible for remembering the rule, and
  `see-also.html.tmpl:8` had already forgotten it (its links pointed at the default-language tree —
  fixed for free by this move). Serve had a second half of the same bug: `server.go` handed the
  per-language tag handlers `opts.TemplateRenderer`, the *default* pipeline's, so a page existing
  only under `fr/` linked as `/fr/b.md` where build emitted `/fr/b`. `LangPipelineConfig` gained a
  `TemplateRenderer` field, matching the `Resolver` field beside it. Covered by
  `TestBuildCmd_Run_DefaultThemeLinksAndHreflang` (build) and `TestServer_TagRoutes_PerLanguage`
  (serve), both with a French-only page so the default resolver cannot accidentally satisfy them.

#### ~~HIGH: the bundled `default` theme forks `head-meta` and loses hreflang~~ ✅ FIXED

`cmd/gomddoc/assets/themes/default/partials/head-shared.html.tmpl:1-7` exists explicitly *"to avoid
duplicating the same ~20-line block in every head.html.tmpl"*, and **all 7** themes in
`gomddoc-themes` call `{{ template "head-meta" . }}`. The bundled default theme does **not** —
`partials/head.html.tmpl` re-implements canonical/feed/prev/next/og:*/twitter:* verbatim and is the
only file in the repo that mentions neither `head-meta` nor `hreflang`. The one line it fails to copy
is `{{ template "hreflang" . }}` (`head-shared.html.tmpl:30`), so the default theme — the one every
new site starts on — emits **zero `<link rel="alternate" hreflang>` tags**. Deleting the duplicated
block and calling `head-meta` fixes the SEO bug and removes ~20 duplicated lines.

- **Fixed:** `head.html.tmpl` now calls `{{ template "head-meta" . }}`, matching all 7 external
  themes; the duplicated canonical/feed/prev/next/og/twitter block and the redundant
  viewport/description tags are gone (−24 lines). The same file's hand-inlined KaTeX stylesheet —
  the last fork of a shared block in the bundled theme — now calls `{{ template "head-katex" . }}`,
  so the pinned KaTeX version lives in one place again.
  `TestBuildCmd_Run_DefaultThemeLinksAndHreflang` asserts the three hreflang links.

#### MEDIUM: tag pages render with nil `Languages` and nil `Features` — NEW (found while fixing §10.3)

`internal/template/renderer.go:338` (`RenderTagPage`) builds its context with `nil` languages and no
feature map. Two consequences, both invisible until you look at a rendered tag page:

- `head-meta`'s `hreflang` block is gated on `gt (len .Languages) 1`, so it emits **nothing** on
  `/tags/*` — the fix above reaches content pages only.
- `config.FeatureEnabled` returns `true` for a nil map (`internal/config/features.go:16-18`), so a
  site with `theme.features.search: false` still ships `search.mjs` on every tag page.

The natural fix — render tag pages against a real `TemplateContext` — has a trap:
`tagPageData.Lang` is `""` for the default language, while `TemplateContext.Lang()` never returns
`""` (it falls back to `Site.Language`, default `en-US`). Anything that switches to the latter and
still derives a URL prefix from it produces `/en-US/...`. `TestServer_TagRoutes_PerLanguage` and
`TestBuildCmd_Run_DefaultThemeLinksAndHreflang` both assert unprefixed default-language links, so
they will catch it.

#### LOW: two disagreeing answers to "when does a URL get a language prefix" — NEW (found while fixing §10.3)

`tagURL` (`internal/template/renderer.go:655-660`) omits the prefix when `lang == "" || lang ==
defaultLang`; `contentURL`'s `langPrefix` is set for every pipeline in `lp.ByLang`. They agree today
only because `locale.DetectLanguages` never yields the default language as a *separate* pipeline in
practice — but it does not exclude a directory matching `site.language` either
(`internal/locale/detect.go:27-40`). With `site.language: en-US` **and** an `en-US/` content dir, the
tag page is served at `/en-US/tags/{tag}` while `tagURL` links it as `/tags/{tag}`. Either make
`DetectLanguages` skip the default language, or make both funcs consult one predicate.

#### LOW: no structural guard that a theme calls the shared head/script blocks — NEW (found while fixing §10.3)

The `head-meta` fork was caught by output assertions, i.e. only after it had shipped an observable
loss. A ~15-line test over the **embedded** theme FS (`cmd/gomddoc/assets/themes/*/partials/`)
asserting each `head.html.tmpl` contains `{{ template "head-meta" . }}` and `{{ template "head-katex"
. }}`, and each `scripts.html.tmpl` contains `scripts-shared`, would fail at the moment of forking.
Note it must **not** run against `assetsFS` or live in `ValidateDefaultTheme` — that FS is the
overlay, where a site's own `.gomddoc/assets/.../head.html.tmpl` legitimately shadows the bundled
one, and failing startup on a valid override would be worse than the bug.

#### LOW: `/api/tags/{tag}` returns real file paths, undocumented as such — NEW (found while fixing §10.3)

`internal/server/server.go:126-127` serves `metadata.PageInfo.Path` verbatim. That is deliberate —
MCP and the metadata API are path-oriented — but nothing says so, and the tag *page* now emits URLs
for the same data. One sentence in `docs/guide/` stating that API paths are file paths, not URLs, and
that consumers wanting a link must resolve them.

#### ~~HIGH: language directories are indexed into the *default* pipeline~~ ✅ reproduced — **FIXED**

`metadata.BuildIndex` and `navigation.buildTree` filter only via `SkipWalkEntry`/`IsRestrictedPath`;
neither skips BCP 47 dirs. With one `fr-FR/page.md`, identically on serve and build:

- default `sitemap.xml` contains `https://example.com/fr-FR/page` — duplicated in `fr-FR/sitemap.xml`
  (duplicate-content signal to crawlers); default `feed.xml` and `/tags/guide` likewise include it.
- default sidebar shows a directory labelled `Fr-Fr`.
- build reports `markdown_files=4` for 3 source files: `fr-FR/page.md` is rendered **twice**, both
  writing `fr-FR/page/index.html`. Correctness currently depends on walk ordering.

Two behaviours presently *depend* on this leak and will break when it is fixed: per-language
`redirect_from` (only the default pipeline gets `URLRedirects` — `server.go:250` vs the per-language
handler at `:208-219`; `generateRedirectFiles` likewise default-only, `build.go:178`) and
per-language extension redirects in build. Fix the leak and wire those per-language in the same change.

**Fixed.** Routed through the *existing* generic exclude mechanism rather than teaching four indexes
about BCP 47: `setupLanguagePipelines` now detects languages **before** building the default pipeline
and passes them down as `PipelineOptions.ExtraExclude` in `provider.IsExcludedPath`'s directory-prefix
form (`"fr-FR/"`). `setupPipeline` merges that with `cfg.Site.Exclude` into one `Pipeline.Exclude`
consumed by all four indexes *and* by build's static walk (`buildContext.exclude`) — the walk
previously read `cfg.Site.Exclude` directly, which is what produced the double render. Build now
reports `markdown_files=3` for 3 source files.

Both free-riding behaviours were wired per language in the same commit. `BuildRedirectMap` gained a
`basePath` parameter, matching the convention `ExtensionRedirect` already used: it prefixes the
**targets** only, because `stripPathPrefix` removes `/{lang}` before the handler runs, so the
**sources** must stay content-root-relative. `PipelineOptions.Lang` is the single input driving both
that prefix and `template.WithLangPrefix` (previously a post-hoc `Configure` call in
`setupLanguagePipelines`). `LangPipelineConfig.URLRedirects` carries the map to the per-language
handler; build calls `generateRedirectFiles` and `generateExtensionRedirects` per language.

MCP was deliberately left on `cfg.Site.Exclude` — an agent querying the site should see translations.
See `docs/decisions.md` § *One Effective Exclude List per Pipeline*.

#### HIGH: two declared-and-documented parameters are silently ignored ✅ verified

- `internal/mcp/tools.go:227` — `handleGetTOC` never references `input`. `GetTOCInput.Path` is in the
  generated JSON Schema (`tools.go:90-92`, `jsonschema:"subtree root path"`) and documented as
  working in `docs/guide/04-mcp.md:187`. A client passing `path: "guide/"` gets the whole site back
  with no error — the worst tool-contract failure mode, because the model believes it was scoped.
- `cmd/gomddoc/pipeline.go` — `redirectFinderAdapter` ignores its path argument and calls
  `navigation.FindFirstPage(navGen.Tree())`, a global DFS from the root. The contract
  (`internal/server/handler.go:21-23`) is *"returns the first page path under **a directory**"*, so
  `/guide/` with no index redirects to the **site's** first page, not the first page under `/guide/`.

#### ~~HIGH: six advertised environment variables are inert~~ ✅ reproduced — **FIXED**

`internal/config/config.go:144` calls `cfg.Site.ApplyEnvOverrides()` — the **site-only** walker.
`(*Config).ApplyEnvOverrides` (`:232`) exists but its only callers are tests. Kong exposes no
equivalent flags, so `GOMDDOC_SERVER_DEV_MODE` and all five `GOMDDOC_SERVER_HTTP_*` vars silently do
nothing — while `cmd/gomddoc/info.go:31-42` advertises every one of them via `config.EnvVars()`, and
`docs/guide/02-configuration.md:492-503` documents them as *"only configurable via environment
variables"*. Fix: call `cfg.ApplyEnvOverrides()` (which walks nested structs including `Site`).

**Fixed.** `NewFromServeArgs` now calls `cfg.ApplyEnvOverrides()` — but **before** the serve args,
not after, which is where this review's suggested fix would have put it. Kong already folds `env:`
tags into every flag it owns, so the args arrive carrying the resolved flag > env > default answer;
a whole-Config env walk placed *after* them re-reads the environment and lets
`GOMDDOC_SERVER_PORT` beat an explicit `--port`, inverting the documented precedence.
`TestNewFromServeArgs_ArgsBeatEnv` guards that ordering (it fails if the call is moved down).

`DevMode` is then OR'd rather than assigned, because it is the one field with no flag on any
command: `args.DevMode` is false for `serve` even when the env var asked for dev mode. The first
draft OR'd `Pprof` and `DirIndex` too; both were wrong. `GOMDDOC_SERVER_PPROF` *is* Kong-owned, so
OR-ing it made `GOMDDOC_SERVER_PPROF=true gomddoc serve --no-pprof` enable pprof — the same
precedence inversion the surrounding comment exists to prevent. `Site.DirIndex` was a provable
no-op, since step 5 re-reads `GOMDDOC_SITE_DIR_INDEX` after the args and owns the field either way.
`TestNewFromServeArgs_ArgsBeatEnv` now covers the `--no-pprof` case.

Doc updates: `02-configuration.md` gained a `GOMDDOC_SERVER_DEV_MODE` row (it was advertised by
`info` and referenced in the README, but the guide never documented it) and its loading-sequence
line now shows the leading env pass with the reason.

##### Follow-up surfaced while fixing this (not addressed here)

- **`--dir-index` answers to two different env vars.** The Kong tag on `preview.DirIndex` is
  `GOMDDOC_DIR_INDEX` (`cmd/gomddoc/preview.go:21`), while the struct tag on `SiteConfig.DirIndex`
  makes it `GOMDDOC_SITE_DIR_INDEX`. Both now work, and `info` lists only the second. Same class as
  `GOMDDOC_DOMAIN` vs `GOMDDOC_SITE_META_DOMAIN`. Pick one naming scheme per setting.

#### MEDIUM: `findRelatedDocs` bypasses the documented single source of truth for tag normalization

`internal/metadata/index.go:266-270` states `NormalizeTags` is *"the single source of truth … so
chips, links, related docs, and the index always agree."* `internal/enricher/markdown.go:139-157`
instead type-asserts `[]any` (missing the `[]string` shape) and passes raw strings to `ByTag`, which
only lowercases without trimming. Reproduced with `tags: ["  Deployment  "]`: the tag chip and
`/tags/deployment` work, but the see-also section is **absent** (control run with the untrimmed tag
renders 7 matches).

#### MEDIUM: index pages appear in their own "related docs" — serve only

`internal/enricher/markdown.go:147-148` — `currentPath` is `r.URL.Path` verbatim, so for `/guide/` the
self-exclusion key is `/guide/` while the index stores `/guide/README`. Build passes `"/"+filePath`
and is correct, so serve and build render different see-also blocks for every index page.

#### MEDIUM: synthetic tag pages ignore site feature config and have no i18n switcher

`internal/template/renderer.go:332-340,371-381` build a `PageContext` with **no `Features`** and `nil`
languages. Content pages go through `config.MergeFeatures` (`internal/template/context.go:46`).
Because `FeatureEnabled` treats `nil` as "all on", a site that disables `search`/`toc`/`katex` in
`theme.features` gets them **silently re-enabled on tag pages only**. `nil` languages also means no
language switcher and no hreflang on `/tags/*`.

#### MEDIUM: git submodules classified as directories → `build` aborts

`internal/provider/git.go:474`, `gitfs.go:131` — `isDir := !entry.Mode.IsFile()`. go-git's
`filemode.IsFile()` is true for `Regular`/`Deprecated`/`Executable`/`Symlink`; the only non-file modes
are `Dir` **and `Submodule`**. A gitlink yields `isDir == true`, `fs.WalkDir` descends, both
`tree.File` and `tree.Tree` fail, and `cmd/gomddoc/build.go:353-355` turns the `fs.ErrNotExist` into
`return fmt.Errorf("walk %s: %w", …)`, **aborting the whole build**. Should be
`entry.Mode == filemode.Dir`, skipping submodules.

#### MEDIUM: `.well-known` silently dropped from git-backed sites

`internal/provider/gitfs.go:128` and `git.go:470` apply a blanket `strings.HasPrefix(entry.Name, ".")`
filter, while `IsHiddenPath` (`exclusion.go:20-22`) carves out `.well-known` as an explicit RFC 8615
exception that the filesystem provider honours. `gomddoc build` therefore omits `.well-known`
entirely from a git-backed site. Direct `ReadFile` still works, which makes the divergence silent.
The filter is duplicated verbatim in two places that must stay in sync.

#### MEDIUM: search results nondeterministically ordered on ties

`internal/search/index.go:296-298,351-353` — `candidates` is seeded from map iteration (randomized),
and `slices.SortFunc` is pdqsort (not stable) with no secondary key. Ties are common: title matches
score `3.0 * idf` ignoring `freq`, and a term present in every document has `idf = 0`, tying **all**
candidates. Exactly the defect §9.4 fixed for `CompareTitles`. Fix both halves: seed from the ordered
`posts` slice and add a path tiebreak. Same class: `internal/server/feed.go:118` sorts by mtime with
no secondary key — pages sharing an mtime (universal after a fresh `git clone`) reorder between
requests, producing spurious feed churn.

#### MEDIUM: snippet body truncated on a byte boundary → invalid UTF-8

`internal/search/index.go:146-149` — `maxSnippetBody = 8192` is **bytes**; the cut lands mid-rune for
any non-ASCII document, and neither `generateSnippet` nor `truncateAtWord` re-aligns, so the partial
sequence reaches the JSON response as U+FFFD. Same class as §9.4's query-truncation fix, missed for
the body.

#### MEDIUM: canonical URLs for non-root index pages lack the trailing slash the build serves

`internal/seo/url.go:37-42` — `normalizePagePath` reduces `/docs/README.md` to `/docs`; only the root
becomes `/`. But build writes `docs/index.html`, i.e. the real URL is `/docs/`. Every non-root index
page gets a canonical and a sitemap `<loc>` that redirect-hop. The slash should be added whenever
`defaultIndex` was stripped, not only at the root.

#### ~~MEDIUM: config loading is silently permissive~~ — **FIXED**

`LoadFromFile` now decodes with `yaml.Decoder` + `KnownFields(true)`, so a `site:` wrapper, a
top-level `features:`/`color_chips:`, or any misspelled key fails at startup with the offending
line instead of applying nothing. Three things the fix had to get right beyond flipping the flag:

- **`io.EOF` is not an error here.** yaml.v3 returns it from `Decode` for an empty *and* a
  comment-only file; without the branch every `# …`-only config.yml became a hard failure.
- **`Decode` reads one document.** A stray `---` parks the rest of the file in a second document
  and drops it — the identical invisible failure through another door, so a second `Decode` must
  see `io.EOF` or the load fails with the separator's line number.
- **`testsite/.gomddoc/config.yml` was itself a victim**: it carried a top-level `color_chips: true`
  that never reached `theme.features`. The project's own fixture would not have loaded under the
  new rules. `docs/` and the website config were clean.

Also `cmd/gomddoc/init_test.go`'s round-trip test decoded with a bare `yaml.Unmarshal`, which is
exactly the leniency production dropped. `initConfig` hand-mirrors `SiteConfig`'s yaml tags with no
compile-time link, so the test would have stayed green while `gomddoc init && gomddoc serve` started
failing on a freshly scaffolded site. It now loads through `LoadFromFile`.

Left open (same class, lower severity, deliberately *not* rolled into that commit):

- `internal/locale/bundle.go:114` — site `.gomddoc/locales/*.yml` overrides decode into
  `map[string]string` and `maps.Copy` over the base, so a typo'd key adds a dead entry and the
  built-in string keeps winning. Themes contribute their own keys, so this wants a `slog.Warn`
  against the known set, not an error.
- `internal/config/features.go` — `ValidateFeatureKeys` checks the *shape* (`^[a-z][a-z0-9_]*$`),
  not membership. `color_chip: true` (singular) passes and does nothing. Feature names are open by
  design (`{{ .Feature "x" }}` works for names no Go code knows), so again: warn, don't reject.
- The yaml.v3 error leaks the Go type name — `field x not found in type config.ThemeConfig` gives
  no path back to the `theme:` key the user typed. Tolerable at the top level, worse when nested.
- `filepath.Join(root, ConfigDirName, ConfigFileName)` is spelled out at seven sites with no
  `ConfigFilePath` helper. Cosmetic until the filename gains a `.yaml` variant.

#### MEDIUM: divergent frontmatter parsers

`internal/text/frontmatter.go:11-17` vs `internal/metadata/index.go:326-363`. `extractFrontmatter`
tolerates leading whitespace and an indented closing `---`; `StripFrontmatter` requires both at
column 0. For those inputs the index extracts frontmatter but the stripper does not, so **raw YAML
flows into the search corpus, the markdown passthrough response, and every MCP read path**.

### 10.4 MEDIUM/LOW — Performance

#### ~~HIGH: `findRelatedDocs` is O(co-tagged pages) per request with a full `PageInfo` copy per tag~~ ✅ reproduced — **FIXED**

`internal/enricher/markdown.go:133-184` + `internal/metadata/index.go:243-253`. `ByTag` materializes a
full `[]PageInfo` (~104 B each) per tag; `findRelatedDocs` then builds a `seen` map over all of them,
sorts the whole candidate list, and discards everything past index 9. Runs in `Enrich` on **every**
markdown request and for **every file** in a static build (making builds O(n²) in co-tagged pages).

| corpus | ns/op | B/op | allocs/op |
| ------ | ------- | ------- | --------- |
| baseline (no metaIndex) | 13,236 | 16,680 | 181 |
| 100 pages | 39,257 | 56,209 | 420 |
| 500 pages | 149,805 | 231,157 | 1,228 |
| 2000 pages | 550,190 | 885,934 | 4,245 |

At 2000 pages related-docs alone is ~40× the enrichment baseline and dominates request latency.
**Fix:** add an allocation-free accessor mirroring the existing `CountByTag`, and keep a top-N heap
instead of collect-all-then-sort-then-truncate.

**Fixed.** Three changes, each removing one term of the growth:

- `Index.PagesByTag` — an `iter.Seq[*PageInfo]` mirroring `CountByTag`, so a caller that reads two
  fields no longer forces a `[]PageInfo` copy of the tag's whole page set.
- `findRelatedDocs` keeps a sorted window of `maxRelatedDocs` instead of collecting every candidate.
  A candidate ordered after the window's worst is dropped on sight, so the sort is bounded to ten
  elements and only runs when a candidate actually improves the result. No heap: at N=10 a
  `slices.SortFunc` on admission is smaller and cheaper than `container/heap`.
- The `seen` set over the whole corpus is gone. A page carrying two of the current page's tags is
  visited twice, but a page evicted from the window is by definition worse than the current worst
  and gets rejected — so the *window itself* is the only place a duplicate can land, and dedup is a
  scan of ten entries.

`text.CompareTitles` was the last per-candidate allocation: `strings.Compare(ToLower(a), ToLower(b))`
copies both strings whenever a title has an uppercase rune, which titles do. It now compares lowered
runes in place. UTF-8 is order-preserving, so that is byte-identical to the old result; a differential
test pins it against the old definition over every pair from a 38-string corpus. This is the shared
title comparator, so the metadata index and search sorts get it too. (Previously 0% covered.)

`BenchmarkMarkdownEnricher_RelatedDocs` now pins the scaling — allocations are flat across corpus
sizes, which is the property that was broken:

| corpus | ns/op | B/op | allocs/op |
| ------ | ------- | ------- | --------- |
| baseline (no metaIndex) | 11,420 | 16,064 | 167 |
| 100 pages | 16,913 | 16,384 | 168 |
| 500 pages | 29,502 | 16,384 | 168 |
| 2000 pages | 71,281 (was 550,190) | 16,384 (was 885,934) | 168 (was 4,245) |

The residual ns/op growth is the unavoidable single pass over the tag's page list.

#### ~~HIGH: search builds a `map[int][]posting` over the entire posting list, per query token~~ ✅ reproduced — **FIXED**

`internal/search/index.go:278-322`. Rebuilt from scratch on every query, for every token, over the
**whole** posting list — including documents the AND-intersection immediately discards. pprof:
`Search` accounts for **77.6%** of query allocations (2000 docs: 493µs, 591 KB, 4,824 allocs).

The key fact: phase 3 of `BuildIndex` (`:194-211`) appends postings in ascending `docIdx` order, so
posting lists are **already sorted** — intersection is a linear merge needing zero maps. Compute `df`
by counting distinct `docIdx` transitions during the scan.

**Fixed.** No map is built at query time, and no pass touches more than it has to:

- `distinctDocs(posts)` walks a posting list's distinct documents — a `docIdx` change is a new
  document. It gives both `df` and, for the seed token, the candidate set.
- Candidates are seeded from the first token with the `tag:` restriction applied up front, so later
  tokens intersect against the tag-filtered set rather than the corpus, and the seed slice is sized
  `min(df, len(tagSet))`.
- Intersection and scoring are both linear merges over the same `seekDoc` cursor primitive: a
  posting for a document the intersection dropped is skipped, not looked up. Scores live in a
  `[]float64` parallel to the candidates rather than a slice of pairs.
- Ranking is a sorted top-N window (`rankTopN`), not sort-all-then-truncate — `limit` is 20 by
  default and capped at 100, while a common term makes every document a candidate. This was the
  largest remaining cost once the maps were gone.
- `compareScored` breaks score ties by `docIdx`. Candidates used to come out of Go map iteration, so
  equally-scored hits were ordered arbitrarily and could differ between serve and build.

The ordering invariant `Search` depends on is documented on the `Index.inverted` field, where a
reader of the data structure lands, and `TestBuildIndex_PostingsSorted` asserts it at the producer.
`TestSearch_MergeMatchesReference` pins candidate selection and scoring against `referenceSearch`, a
test-only map-based oracle, over 15 query shapes × 2 limits on a 60-document corpus — same paths,
same order, same scores, same snippets. The oracle deliberately shares the tag preamble and result
materialization (unchanged by this work) but ranks by sorting everything, so a broken window fails it.

`BenchmarkSearch` (2000 docs, limit 10):

| query | ns/op | B/op | allocs/op |
| ---------------------------- | ------------------- | -------------------- | -------------- |
| `common` (every doc; idf 0) | 24,160 (was 139,036) | 36,464 (was 264,786) | 57 (was 2,066) |
| `alpha` (⅓ of docs, varied scores) | 19,192 (was 137,064) | 25,280 (was 156,776) | 87 (was 1,509) |
| `common alpha` | 32,157 (was 278,856) | 35,888 (was 379,368) | 128 (was 3,559) |
| `tag:t1 common` | 60,029 (was 180,312) | 68,760 (was 297,233) | 62 (was 2,071) |

Allocation counts are flat across corpus sizes (identical at 50, 500 and 2000 docs); the residual
byte growth is the candidate and score slices, both O(matching docs). Note `common` has
`df == docCount`, so `idf == 0` and every score ties — it measures the posting-list scan, not the
ranking. `alpha` is the one to watch for ranking changes.

Measured follow-ups left on the table, in descending value:

- **`df` is recomputed per query although it is static.** A full scan of the token's posting list per
  token per query, ~4µs per 2000-posting list. Storing it beside the slice (`map[string]termEntry`)
  computes it once in phase 3. Also unlocks intersecting tokens in ascending-`df` order, which is
  result-neutral and shortens every merge.
- **`taggedPages` copies `[]metadata.PageInfo` for the mixed path.** ~41.6 KB of the 68.8 KB in
  `tag:t1 common` at 2000 docs, to read `p.Path` and discard the rest. `PagesByTag` fixes it, but
  `tagOnlyResults` *writes* to the pages it is given, so the tag-only path must keep the copies —
  it needs a separate `tagDocSet(tags)` entry point, not a swap.
- **Three passes over the seed token's posting list** (df, candidates, scoring) where one would do,
  once `df` is precomputed.
- **Linear cursor advance in both merges.** Galloping or a binary search pays off past ~10⁴ docs,
  when a one-candidate set is intersected against a whole-corpus token.
- Measured and rejected, so nobody re-litigates it: shrinking `posting` from 24 to 12 bytes
  (`int32` fields) does not move query latency — 3.990µs vs 3.993µs on a 2000-element scan. These
  loops are branch-bound, not bandwidth-bound. Consider it for index memory, never for latency.

#### ~~MEDIUM: the compression buffer pool is poisoned after the first request~~ ✅ FIXED

Fixed at the root: `Write` now **decides before it copies**. It buffers only while
`len(buf)+len(b) < minCompressionSize` and otherwise commits, flushes whatever prefix was buffered,
and hands the write straight to the encoder. The buffer therefore can never reach
`minCompressionSize`, so `bufPool` allocates exactly that (was 4096) and the slice is never
reallocated — which in turn kills the poisoning: `returnBuf` no longer writes `cw.buf` back over the
pool's slice header at all, it just hands the pointer back. A slice that somehow *did* grow is now
dropped for free, so `maxPoolBufferSize` and its branch are gone rather than merely revived.

Four collapses fell out of it: `decide()` + `flushBuffer`'s header-writing head became one
`commit()`; the `compress bool` field went away (`gzw != nil` already carries the decision);
`Close`'s two `!cw.decided` branches became one that calls `flushBuffer` instead of open-coding it;
and `Write` now has a single return contract (the encoder's `n`) instead of two.

Measured (`BenchmarkCompression`, median of 5, before → after):

| Case | ns/op | B/op | allocs/op |
|------|-------|------|-----------|
| BelowThreshold_TextHTML (~500B) | 375 → 237 | 689 → 112 | 4 → 3 |
| AboveThreshold_TextHTML_Gzip (~5KB) | 26061 → 25161 | 6001 → 291 | 5 → 4 |
| AboveThreshold_PrefixThenBody_Gzip | 26584 → 25627 | 5852 → 291 | 6 → 4 |
| AboveThreshold_ImagePNG_Skipped (~5KB) | 1384 → 234 | 6647 → 112 | 4 → 3 |
| HEAD_LargeTextHTML | 93 → 96 | 32 → 32 | 2 → 2 |

The benchmark itself had to be fixed first: it wrote into an `httptest` recorder and converted its
payload from `string` per request, both of which cost more than the middleware and hid the change
entirely (the same run against the *old* code reported 19KB/op either way). It now writes to a
discarding `ResponseWriter` and reuses `[]byte` payloads, and covers the prefix-then-body case.

Tests: `TestCompressionWriter_BufferHandback` is one table over the six ways a response ends, each
asserting `cap(cw.buf)` — the response body is byte-identical whether the buffer survives or not, so
capacity is the only observable. Both defects show up there: niling drops it to zero, appending a
body reallocates it away from the pooled array. It replaces three tests, one of which
(`ReturnBuf_OversizedDiscarded`) sent a request *without* `Accept-Encoding`, so the middleware
returned before allocating a buffer and it asserted nothing at all.

Measured follow-ups left on the table (from the efficiency pass, numbers pre-date this fix):

- `&compressionWriter{}` heap-allocates 80 B per gzip-eligible request, including 304s with no body.
  Pooling the struct with the buffer inline (`[minCompressionSize]byte`) removes that allocation
  *and* the second pool round-trip, since the buffer is now a fixed size with no growth path. Only
  safe because no handler here retains `w` past `ServeHTTP` — verify that before doing it.
- `gzw.Reset` clears flate's ~640 KB of hash tables per request at `DefaultCompression`; levels 1–3
  use the much smaller `deflateFast` tables. Unmeasured: sweep levels 1/4/5/6 for ns/op *and* output
  size before touching it.
- `shouldSkipContentType` lowercases before stripping `;` parameters, so `charset=UTF-8` allocates a
  copy of the whole string. Nothing in-tree spells a content type with uppercase, so this is latent.

Original finding:

`internal/server/compression.go` — `flushBuffer` sets `cw.buf = nil` on both branches, and `Close`
does the same, so by the time the deferred `returnBuf` runs `cw.buf` is **always** `nil`:

```go
if cap(cw.buf) <= maxPoolBufferSize {  // cap(nil) == 0, always true
    *cw.bufPtr = cw.buf[:0]            // stores a nil slice back into the pool
    bufPool.Put(cw.bufPtr)
}
```

The pool is refilled with zero-capacity slices forever — every request re-grows from scratch, and
`maxPoolBufferSize` is dead code. `New` allocates `make([]byte, 0, 4096)`, so the intent is clear.
**Fix:** reset the length instead of niling (or nil *after* `returnBuf`). Separately, `Write` copies
the entire body via `append` even though `serveWithETag` delivers it in a single call — skip
buffering when the first `Write` already exceeds `minCompressionSize`.

#### MEDIUM: breadcrumbs generated twice per request ✅ FIXED

`cmd/gomddoc/assets/themes/default/layouts/default.html.tmpl:20` called `breadcrumbs .Page.Path`, and
`partials/head.html.tmpl:1018` → `{{ template "jsonld" . }}` → `jsonLD .Page` →
`internal/template/renderer.go generateJSONLD` called `breadcrumbGen.Generate(page.Path)` again.
That closure is a real provider `Stat` syscall (`cmd/gomddoc/pipeline.go:166-169`), plus
`text.TitleCase` per segment. Measured: 2,158 ns / 2,184 B / 23 allocs per `Generate` *excluding* the
syscall. (This also subsumes the `cases.Title` `sync.Pool` idea already marked Won't-Fix in §5 — that
decision stands.)

> **Correction to the original finding:** it claimed `Generate` calls `g.isDir()` **per path
> segment**. It does not — `internal/template/breadcrumb/generator.go:64` calls it exactly once per
> `Generate`, for the trailing segment only. Intermediate segments are known to be directories. The
> per-request *doubling* was real; the per-segment syscall count was not.

**Fix:** memoising inside the template function is structurally impossible — `funcMap` is bound at
parse time and parsed templates are cached and shared across concurrent `Render` calls, so there is
no per-render seam. Breadcrumbs moved instead to precomputed page data, where every other derived
page value already lives (`TOC`, `Navigation`, `PrevPage`, `RelatedDocs`):

- The `breadcrumbs` template function was **deleted** from `funcMap` (12 functions remain).
- `Renderer` gained `Breadcrumbs(path) []breadcrumb.Breadcrumb`; `PageContext` gained a
  `Breadcrumbs` field.
- `BuildPageContext` is the single generation site. Callers pass `PageContextInput.Renderer`, not a
  precomputed trail, so the trail and `Path` cannot disagree.
- `generateJSONLD` maps `page.Breadcrumbs` to `[]seo.BreadcrumbItem` instead of regenerating.
- All 8 theme layouts (default + the 7 in `gomddoc-themes`) now use `{{ range .Page.Breadcrumbs }}`.

`BenchmarkPageWithBreadcrumbs` (context build + render of a layout with both the breadcrumb bar and
JSON-LD), median of 5 at `-benchtime 2000x` on an Apple M1 Max:

| | ns/op | B/op | allocs/op |
|---|---|---|---|
| before | 21548 | 15023 | 258 |
| after | 19789 | 13140 | 236 |
| | **−8.2%** | **−12.5%** | **−8.5%** |

Plus one provider `Stat` per page that the benchmark's in-memory `isDir` closure does not model.

**Behaviour change:** error pages no longer emit a JSON-LD `BreadcrumbList`. `BuildErrorContext`
leaves the field nil; `error.html.tmpl` has no breadcrumb bar, so nothing visible changes, and error
pages are `robots: noindex` anyway. Documented at the function and in `architecture.md`.

Also fixed in passing: `breadcrumb.Generator` sized its `strings.Builder` with
`Grow(len(filepath))`, one byte short of what the loop writes (`len(filepath)+1` — one `/` per
segment, and `filepath` already holds `len(segments)-1` of them).

**Left for later** (recorded here so they are not lost):

- `RenderTagPage`/`RenderTagsIndex` (`internal/template/renderer.go`) hand-assemble a `PageContext`
  literal instead of going through `BuildPageContext`, so they now assign `Breadcrumbs` by hand and
  still skip `config.MergeFeatures` — a site that disables `toc` or `search` in `theme.features`
  still gets them on `/tags/` and `/tags/{tag}` (this is the §10.3 "nil `Features` on tag pages"
  finding, same root cause). Their trail is also fully static, so the generator's `Stat` for a
  synthetic path is a guaranteed miss on every tag page — worst case under `GitProvider`, which
  takes the exclusive tree mutex for both the failed file and failed directory lookup.
- `Breadcrumbs` re-derives, with a second provider round trip, the directory-vs-file bit the
  provider already knew when `ServeContent` read the page. Surfacing the resolved kind from the read
  would remove the round trip on the hot path; it also uses `context.Background()`
  (`cmd/gomddoc/pipeline.go:166-169`), so it is not cancellable with the request — see the
  `request-scoped context.Background()` entry below.

#### MEDIUM: `findBestWindow` lowercases ~50 substrings per search result ✅ FIXED

`internal/search/snippet.go:112` — `strings.ToLower(content[pos:end])` inside a ~50-iteration sampling
loop, allocating a fresh copy each time. **13.9%** of all search-query allocations. Fix: lower the
document body once per doc, or store a pre-lowered snippet body in the index at build time.

**Fix.** Lowered once per document and sliced per window — not stored in the index, which would
double the 8 KB `maxSnippetBody` per document for a fold that only the queried results need.
`strings.ToLower` returns its argument unchanged when there is nothing to fold, so an all-lowercase
body now costs zero allocations instead of ~50.

`highlightTerms` had the same shape one level down: it called `findTokenSpans` per query token, and
each call re-lowered the same ~160-byte snippet. It now folds once and passes the copy down —
`findTokenSpans` takes `lower` as a parameter, matching `findTokenSpansRunewise`'s existing signature.

> **Sharp edge found while fixing.** Go's `strings.ToLower` uses *simple* case mapping: `İ` (U+0130,
> 2 bytes) maps to `i` (1 byte), so the lowered copy can be **shorter** than the original, not longer.
> `lower[pos:end]` then slices a region that is not `content[pos:end]` — and panics outright once
> enough bytes have been lost. `findBestWindow` keeps the per-window lowering for those bodies,
> guarded by a `len(lower) == len(content)` check, the same guard `findTokenSpans` already used.
> The first draft evaluated `lower[pos:end]` before the guard; the new
> `TestFindBestWindow/case_folding_changes_byte_length` case caught it as a panic.

Measured (`BenchmarkGenerateSnippet`, new — `mergeCorpus`'s bodies are shorter than the 160-byte
window, so `findBestWindow` returned 0 without ever entering the sampling loop and the existing
benchmarks could not see this at all):

| | before | after | |
|---|---|---|---|
| ns/op | 22450 | 19970 | −11.0% |
| B/op | 10016 | 10048 | +0.3% |
| allocs/op | 67 | 17 | **−74.6%** |

Bytes are flat because one 8 KB fold replaces ~50 × 176 B ones; the win is allocation count and the
GC pressure behind it. `BenchmarkSearch/docs=2000/common_alpha` picks up the `highlightTerms` half:
128 → 118 allocs/op, 32230 → 30300 ns/op.

#### MEDIUM: MCP TOC rebuilds the nav tree per call and re-opens every markdown file ✅ FIXED

`internal/mcp/tools.go:233-234` constructs a fresh `navigation.NewGenerator` per call, making its
`sync.Once` cache useless, and never installs `SetTitleLookup` — so `buildTree` falls through to
`extractTitle`, which **opens and line-scans every `.md` file in the site** on every
`get_table_of_contents` call. §9.8 eliminated exactly this for the HTTP path; the MCP path was left
behind even though `s.deps.MetaIndex` is right there. (The pipeline's cached generator at
`cmd/gomddoc/pipeline.go:222` is never passed into `ServerDeps`.)

**Fix.** The pipeline's generator is published as `Pipeline.NavGenerator` and handed to
`mcp.ServerDeps.NavGenerator`; `handleGetTOC` calls `Tree()` on it. `ServerDeps.DefaultIndex` and
`.Resolver` existed only to feed the removed constructor and are gone. `gomddoc mcp` built its
pipeline with `EnableNavigation: false`, so the fix required flipping it — the generator is lazy
(bare `sync.Once`, `NewGenerator` only assigns fields), so this costs one struct allocation at
startup and the first `get_table_of_contents` call does strictly less work than before.

The title-lookup closure, previously written out at each site, is now
`metadata.(*Index).TitleForPath` passed as a method value. It also drops `ByPath`'s defensive
`PageInfo` copy (struct + `Tags` slice header) per navigation leaf.

Behaviour note, deliberate: under `serve`, MCP receives the **default** pipeline's generator, whose
exclude list hides every detected language directory. Translated pages are therefore absent from
`get_table_of_contents` — consistent with `list_pages` and `search_docs`, which already read that
pipeline's indexes, and better than the old TOC, which listed them with raw `.md` URLs the default
resolver could not clean. They remain readable by path through `read_page`:
`ServerDeps.ExcludePatterns` stays `cfg.Site.Exclude`, the author's access-control intent.

`TestTools_GetTOC_SharedGenerator` pins both halves — leaf labels must come from the metadata index
(frontmatter title, not the H1) and a `countingFS` must see no new `Open` calls on the second call.
Restoring the per-call constructor turns both red. `setupPipeline`'s option table now asserts
`NavGenerator` tracks `EnableNavigation`, which is the half no handler test can see.

**Left open** (pre-existing, unrelated to the perf fix): `GetTOCInput.Path` is advertised in the tool
schema as "subtree root path" and documented in `docs/guide/04-mcp.md` and
`docs/guide/12-advanced/03-api-reference.md`, but `handleGetTOC` never reads it — an agent asking for
one subtree gets the whole site. Now cheap to honour (a pointer walk into the cached tree) or delete.

#### MEDIUM: compression and metrics cover only 2 of 8 route groups ✅ FIXED

`internal/server/server.go:228,259` — the only two `Compression` occurrences, both content subgroups.
Measured:

```
GET /guides/getting-started  Accept-Encoding: gzip → Content-Encoding: gzip
GET /tags/                   Accept-Encoding: gzip → NOT COMPRESSED (51 KB HTML)
GET /sitemap.xml             Accept-Encoding: gzip → NOT COMPRESSED
```

`/api/search`, `/tags/`, `/tags/{tag}`, `/sitemap.xml`, `/feed.xml`, `/_assets/` (CSS/JS) and
`/robots.txt` are never gzipped — and per CLAUDE.md's own rule never get `Vary: Accept-Encoding`.
These are the *most* compressible payloads on the site. `http_requests_total` likewise counts content
requests only, under-reporting real traffic. `RouteGroup.Subgroup` composes cleanly; hang these off a
subgroup carrying at least Compression + Metrics.

**Fix.** `Compression` and `Metrics` moved off the two content subgroups onto a new
`base := NewGroup(mux, "", Compression, Metrics)`, and every user-facing route now descends from it:
`/robots.txt` and `/_assets/` directly, everything else through `auth := base.Subgroup("", authMW...)`.
The two content subgroups keep only their own concerns (`stripPathPrefix`, MethodFilter,
ContentExclusion, ExtensionRedirect). Attaching a cross-cutting concern to a leaf is what let six
sibling groups opt out silently; on the parent, a route added later gets it without anyone
remembering to.

Two ordering consequences, both improvements:

- BasicAuth is now *inside* Compression and Metrics, so 401s are counted.
- Metrics now wraps MethodFilter/ContentExclusion/ExtensionRedirect, so their 405/403/301 responses
  are counted too, not just handler hits.

Three groups stay deliberately off `base`, each with the reason in a comment at the registration
site: `/health/*` (probe traffic would swamp the counters; bodies are far below the 1 KB threshold),
`/metrics` (`promhttp` negotiates its own encoding, and a scrape that increments the counter it is
reporting feeds its own numbers back), and pprof (already-compressed binary, not user-facing).

Tests: `internal/server/route_coverage_test.go` tables the eight routes and asserts, per route, a
200, a `+1` delta on `httpRequestsTotal{GET,200}`, `Vary: Accept-Encoding`, `Content-Encoding: gzip`,
and a gzip stream that decodes to ≥ `minCompressionSize`. Its fixture is 60 pages with a shared and a
unique tag each, sized so `/tags/`, `/api/tags` and the rendered page all clear the threshold.
`TestRobotsTxt_VaryWithoutCompression` covers the small-body half of the contract (Vary present,
no `Content-Encoding`), and `TestMetricsEndpoint_NotSelfCounted` pins the `/metrics` exemption.
Mutation-verified: moving either middleware back down to `content` turns all seven non-content cases
plus the robots test red. No `t.Parallel` — `httpRequestsTotal` is a package-level Prometheus counter.

This also partly resolves the `docs/01-http-behavior.md` MEDIUM drift entry ("overstates Cache-Control,
ETag and compression as universal") — compression *is* now universal across user-facing routes.
Cache-Control and ETag remain content-handler-only.

#### ~~MEDIUM: git reads do redundant work under the (now serialised) tree lock~~ ✅ FIXED

Surfaced while fixing the §10.1 tree race. All of these were on the critical path of a single
exclusive lock, so they cost throughput on git-backed sites rather than just CPU:

- **`Stat` decompresses an entire blob to read `file.Size`** (`internal/provider/git.go:509`).
  `tree.File()` calls `GetBlob`, which loads and decodes the whole object; only `Mode` and `Size` are
  used. On packfile-backed disk storage that is a full delta/zlib decode, discarded immediately —
  and `cmd/gomddoc/pipeline.go:167` wires `prov.Stat` into the breadcrumb generator, so it runs on
  every page request.
- **Every path is resolved twice on a directory hit** — `git.go:393`+`400`, `git.go:509`+`522`,
  `gitfs.go:63`+`69` each ran `FindEntry` for the same path once as a file and once as a tree. One
  `FindEntry` then switching on `entry.Mode` does it in one pass. Worse on the filesystem storer,
  where the failing `File()` attempt fully decodes the tree object before discarding it on the type
  check.
- **`file.Contents()` round-trips through a `bytes.Buffer` and a `string`** (`git.go:420`,
  `gitfs.go:84`) — roughly 3N allocated and 4N copied for an N-byte page, given the trailing
  `[]byte(content)`. `file.Reader()` + `io.ReadFull` into a `make([]byte, file.Size)` is one
  exact-size allocation.

**Fix.** `gitTreeState` now carries the tree's own object storer (`objects
storer.EncodedObjectStorer`) beside the tree — published by `cloneLocked` and cleared by `Close`, both
under the one lock that already guards the tree. `object.Tree` keeps its storer unexported, and that
field is what lets a caller ask for an object by a hash the path walk already produced. Three helpers
in `gitfs.go` now own the object access, and both surfaces (the `GitProvider` methods and the
`gitTreeFS` handed out by `RootFS`) go through them:

- `resolveTreeNode` walks a path once, switches on `entry.Mode`, and fetches through the hash
  (`object.GetTree(objects, entry.Hash)` for a subtree, `tree.TreeEntryFile(entry)` for a blob) —
  no probe, no second walk. It restores `file.Name` to the full path, which `TreeEntryFile` sets to
  the base name alone.
- `statTreeNode` answers a stat from the entry plus `objects.EncodedObjectSize(entry.Hash)`.
  `tree.Size` was not enough: `Tree.FindEntry` only consults its subtree cache from three segments up
  (`for i := len(pathParts) - 1; i > 1`), so a second lookup of `docs/guide.md` re-decodes the `docs`
  tree. Verified against go-git v5.19.1.
- `blobBytes` reads a blob into one exact-size allocation.

The same defect had a second home the original entry missed: `gitTreeFS` implemented only `fs.FS`, so
`fs.Stat` and `fs.ReadFile` fell back to `Open` — which decodes the whole blob. Sitemap and feed
generation stat every page in the site (`internal/server/sitemap.go:130`, `feed.go:111`) for a
`ModTime` that is the commit timestamp, the same constant for every file in the tree; the metadata and
search index builds read every file in the repository through `fs.ReadFile`. `gitTreeFS` now
implements `fs.StatFS` and `fs.ReadFileFS`.

Measured on an ~8 KB page, in-memory storage (`BenchmarkGitProvider_*`, `BenchmarkGitTreeFS_*`, Apple
M1 Max, `-count 3`). Memory storage understates the win — the deployed storer for `--git-storage-dir`
is `filesystem.ObjectStorage`, where materialising an object is a packfile seek plus a delta and zlib
decode rather than a map lookup:

| Benchmark | Before | After |
| --------------------- | ---------------------------- | -------------------------- |
| `GitProvider.Stat` | 675 ns, 480 B, 11 allocs | 585 ns, 352 B, 9 allocs |
| `GitProvider.ReadFile` | 11.0 µs, 49.3 KB, 19 allocs | 2.40 µs, 8.7 KB, 13 allocs |
| `GitProvider` dir read | 1017 ns, 641 B, 21 allocs | 872 ns, 609 B, 19 allocs |
| `fs.Stat` (RootFS) | 2469 ns, 8823 B, 16 allocs | 564 ns, 352 B, 9 allocs |
| `fs.ReadFile` (RootFS) | 4190 ns, 17.0 KB, 17 allocs | 2363 ns, 8.7 KB, 13 allocs |

Tests (`internal/provider/git_reads_test.go`) assert the object access itself, not the timing: a
`countingStorer` separates `EncodedObject` (materialise) from `EncodedObjectSize` (header), and each
assertion was mutation-verified — restoring the `tree.File` probe, `tree.Size`, `file.Contents()`, or
removing either `fs.StatFS`/`fs.ReadFileFS` turns the relevant counter or the `cap(body) == len(body)`
check red. The package's provider-construction helpers moved to `internal/provider/testhelpers_test.go`,
where `newTestTreeState` pairs a tree with its storer so a hand-built `gitTreeState` cannot reach the
`tree != nil, objects == nil` state the provider never produces.

Whether the blob read can move *outside* the lock is a separate question — the blob looks detached
from the tree once the file is resolved, but that depends on storer- and version-specific go-git
internals (see `docs/decisions.md`, "Serialised Git Reads"). Do not change it without a benchmark
justifying the risk. **Left open**, all raised during this fix's review and none of them regressions
from it:

- **The directory-index read bypasses `readFileLocked`'s guards** (`git.go:441`) — the closure
  `handleDirectoryLocked` passes to `handleDirectory` reads the default index with
  `dirTree.File(g.defaultIndex)` + `blobBytes`, so neither `maxFileSize` nor `isLFSPointer` applies.
  An LFS-tracked `README.md` is rejected at `/README.md` and served as raw pointer text at `/`.
- **`gitTreeFS.Open` applies no size cap at all** and `blobBytes` commits `make([]byte, f.Size)` from
  a header-declared size, so the index builds will happily materialise a repo-sized blob.
- **Depth-2 paths defeat go-git's subtree memo** — `FindEntry`'s cache is only consulted from three
  segments up, so `/docs/guide.md`, the commonest URL shape on a docs site, re-decodes the `docs`
  tree on every request. A `map[string]*object.Tree` beside `tree`/`objects` in `gitTreeState`
  (immutable for the provider's lifetime, cleared by `Close`) would fix it.
- **Three spellings of "this entry is a directory"** — `entry.Mode == filemode.Dir` in
  `resolveTreeNode`/`statTreeNode`, `!entry.Mode.IsFile()` in `treeDirEntries`. They agree except on
  submodule gitlinks, where behaviour is unchanged from before this fix.
- **`git_test.go` still hand-rolls 22 `&GitProvider{…}` literals** that `gitProviderOver` (now in
  `testhelpers_test.go`) covers; adding the `objects` field meant editing 11 of them.

#### LOW — performance

- **Inline-asset cache stores raw `[]byte`** (`internal/template/inline_asset.go:36-62`) — `readAsset`
  caches bytes, then each call does `template.JS(data)`/`CSS`/`HTML`, a full copy per render
  (~22.8 KB/page across 7 inlined JS assets). Cache the converted value.
- ~~**Resolver does two full filesystem walks at startup**~~ **FIXED** — collapsed into one walk as
  part of the §10.1 exclude fix.
- **`HasTemplate` does an `fs.Stat` per request** (`internal/template/renderer.go:735-739`, via
  `ResolveLayout` at `handler.go:175`). The template set is fixed after startup; memoize, gated on
  `cacheAssets`.
- **`emitTagPages` renders serially** (`cmd/gomddoc/build.go:733-772`) while the markdown walk is
  parallel — a serial tail on a tag-heavy build. Wrap in an errgroup at `runtime.NumCPU()`.
- **Tag pages rebuilt from scratch per request** (`internal/server/tags_html.go`) — `ByTag` copy +
  sort + full render, uncapped and uncached, on an index that is immutable after startup.
- **Corpus read twice at startup** — `metadata.BuildIndex` and `search.BuildIndex` each independently
  `fs.ReadFile` every `.md`. Merging into one read pass would halve startup I/O.
- **`request-scoped context.Background()`** (`cmd/gomddoc/pipeline.go:166-169`) — the breadcrumb
  `isDir` closure runs on the request path but is uncancellable, because
  `breadcrumb.IsDirFunc` drops the context. For a git provider this can block on a clone no client
  can cancel. Relatedly, `internal/provider/git.go:202-247` runs the clone under the write lock driven
  by a *request* context, so one client navigating away aborts a provider-global operation for
  everyone queued behind `g.mu.Lock()`.

### 10.5 Documentation drift

§9.6's fixes **held** — CLAUDE.md's package tree still matches all 16 `internal/` packages, `make run`
still shows its argument, MCP counts are still 6/4/3. One §9.6 fix was *incomplete rather than
regressed*: the theme-count correction was applied to `05-theming-and-assets.md` only.

**HIGH — actively wrong; a new user following these fails immediately:**

| # | Doc | Reality |
| --- | ---------------------------------------- | ------------------------------------------------- |
| D1 | `README.md:118,377` documents `serve --dev` | ~~✅ verified: `unknown flag --dev`~~ — **FIXED**. The flag table now matches `serve --help`, and both `--dev` call sites point at `preview` / `GOMDDOC_SERVER_DEV_MODE` |
| D2 | `README.md:118` uses `-d ./testsite` as the directory | ~~✅ verified: `-d` is `--domain`~~ — **FIXED**. README uses the positional arg; the k8s manifest now passes `args: ["serve", "/content"]` |
| D3 | `02-configuration.md:492-503` documents 5 `GOMDDOC_SERVER_HTTP_*` vars + `DEV_MODE` | ~~✅ verified inert~~ — **FIXED**, they now take effect; see §10.3 |
| ~~D4~~ | ~~Six files document a `navigation` template function~~ | ✅ FIXED. The FuncMap had **12** entries and no `navigation`; a theme copying the documented example failed to parse. `architecture.md`'s table was rewritten from `renderer.go:funcMap` — it had also omitted `canonicalURL`, `jsonLD`, `tagURL` and `pageTags`, listed `.Feature` (a `PageContext` method, not a FuncMap entry) and claimed `assetURL` validates existence. The guide gained a **Rendering the Navigation Sidebar** section with the recursive `nav-item` markup, since deleting the function reference without a replacement leaves theme authors with nothing. `architecture.md` §7 also documented `NavNode` as the template-facing type: it is the cached generator tree (`Label`, no active state), while templates range over `enricher.NavItem` (`Title`/`Active`/`Open`) — both are now shown, one per side of the cache boundary. `decisions.md` records *why* it is a field and not a function (parse-time FuncMap binding on shared cached templates leaves nowhere to memoize — the same reason breadcrumbs moved in §10.4) |
| ~~D5~~ | ~~`08-observability.md:26-27` — `--admin-port` removes health *"from the main port entirely"*~~ | ✅ FIXED. `/health/live` and `/health/ready` return 200 on **both** ports; only `/metrics` (`server.go:139-143`) and pprof (`:208-213`) are gated on `AdminOnMain()`. Both guides now say so *and* say why — probes target the service port, so moving health would break every one of them the moment the ports are split, which is what makes this the one place the three "admin endpoints" deliberately diverge. `03-api-reference.md:304` carried the same wrong claim. `TestHTTPServer_AdminPortSeparation` covered `/metrics` and pprof but not health, so the doc had nothing to drift against; it has two rows for it now |
| ~~D6~~ | ~~`docs/custom-renderers.md` teaches `Renderer` with a dead interface~~ | ✅ FIXED. The 870-line legacy file is deleted and replaced by `docs/guide/12-advanced/04-custom-renderers.md`, written against the real `InputMimeTypes`/`OutputMimeTypes`/`Render(ctx, content, *enricher.EnrichmentData)` contract. Two things the old doc could not have said because they are structural: `internal/renderer` is **unimportable** from outside the module and registration lives in `cmd/gomddoc/pipeline.go` (`package main`), so *"Register in main.go"* meant forking gomddoc, not writing a plugin — the new page leads with that. And the registry's tie-break is load-bearing: the score is `inputScore*10 + outputScore`, so input specificity dominates and the `*/*` passthrough can never shadow anything, but **within** one input type the later registration wins — `MarkdownRenderer` is registered after `MarkdownPassthroughRenderer`, which is the only reason `Accept: */*` yields HTML instead of raw markdown. The nine non-compiling examples collapse to one (CSV → HTML table) plus the `mime.AddExtensionType` step without which a new extension is never routed. `README.md`'s inline example had the same dead interface and is rewritten; `architecture.md:842` now links the guide and records the ordering rule |
| D7 | `09-deployment.md:105-118` — *"multi-stage Dockerfile"* | ~~verified~~ — **FIXED**. The section now leads with `docker pull ghcr.io/...`, states that the Dockerfile is not self-contained, and gives a `GOOS=linux` cross-compile before `docker build`. The bare-`gomddoc` run example gained the `serve` subcommand |
| D8 | `02-configuration.md:88` — README.md generates *"both `README.html` and `index.html`"* | `prettyOutputPath` (`build.go:600-604`) emits only `<dir>/index.html`; `:616-622` explicitly skips the redirect stub |
| ~~D9~~ | ~~`03-api-reference.md:161-164` lists `GET /sitemap-index.xml` as a live endpoint~~ | ✅ FIXED. The citation was right (`docs/guide/12-advanced/03-api-reference.md`; a first pass looked only at `docs/guide/` non-recursively and wrongly called it stale — commit `065f15a`'s message repeats that error). The `### GET /sitemap-index.xml` heading is gone: it now says *not an endpoint* and names build as the only writer. Two more files carried the same claim and were fixed in the same pass: `11-seo.md:107` presented `/sitemap-index.xml` as a URL immediately after a paragraph contrasting serve and build for `sitemap.xml`, and `13-internationalization.md:300` listed it under *Per-Language Features* beside genuinely-served URLs. Both now say build-only and name the consequence (that URL 404s on a live server). `TestServer_PerLanguageSitemaps_ButNoIndex` pins it; it asserts both per-language sitemaps 200 first, because the routes are gated on `meta.domain` and the 404 is otherwise a dead assertion. The build-side generation was already covered (`build_test.go:1247`) — only serve's *absence* was not |
| ~~D8~~ | ~~`02-configuration.md:88` says `README.md` generates both `README.html` and `index.html`~~ | ✅ FIXED. Only `index.html` is written, in both branches of `buildFile:474-481`, and four `build_test.go` assertions already denied `README.html` — the tests were right and the prose was wrong. The bullet now states the real rule and, since it is the fact the wrong claim was reaching for, distinguishes the two layouts `strip_extensions` selects. The audit found the same error one level down: `generateExtensionRedirects`' own doc comment claimed `guide.html -> guide/`, when the stub is written at the *source* path (`guides/setup.md`, holding HTML that points at `/guides/setup`) and default-index files are skipped. Comment corrected, and `TestBuildCmd_Run_ExtensionRedirectStubs` now reads a stub back and denies both wrong shapes — nothing had, which is why the comment drifted |
| ~~D10~~ | ~~*"8 built-in themes"* across the repo and 6 website files~~ | ✅ FIXED. `cmd/gomddoc/assets/themes/` contains only `default`; the other seven are a directory drop from `gomddoc-themes`. The guide's theme table gained a **Bundled** column, and `05-theming-and-assets.md`'s correct-but-buried footnote was promoted above the table. Where the count was incidental (*"works across all eight built-in themes"*) the phrasing now says *every theme*, which stays true if the count changes |
| D11 | `gomddoc-website/docs/distribution.md:75-97` — wrong archive filenames; advertises Windows binaries | `.goreleaser.yaml:14,23-24` builds linux/darwin × amd64/arm64 only; `install.sh:62` hard-`die`s on any other OS |
| ~~D12~~ | ~~Top-level `features:` documented in `02-markdown-extensions.md:205-210` + website~~ | ✅ FIXED, at both altitudes. The docs now nest under `theme.features` (the top-level `features:` blocks elsewhere in the guide are *frontmatter*, where it is correct — only the `config.yml` example was wrong), and `LoadFromFile` no longer accepts the misplaced key silently: see the `KnownFields` entry in §10.3. The website's inert `GOMDDOC_SITE_COLOR_CHIPS` is replaced by the real `GOMDDOC_SITE_THEME_FEATURES_COLOR_CHIPS`, which is what `applyEnvOverridesWithPrefix`'s `map[string]bool` branch actually reads |
| ~~D13~~ | ~~`docs/seo-competitive-analysis.md:18-32` claims canonical URLs, OG, robots and sitemap are **absent**; `:605` recommends skipping hreflang~~ | ✅ FIXED by dating the document rather than rewriting it — the competitor survey and the per-recommendation rationale are still correct and still worth reading; only the verdicts on gomddoc went stale. A banner marks it pre-Phase-9 research, a new **Implementation Status** table audits all 18 against the tree (14 shipped, 2 partial, 2 not), the two inverted claims carry inline corrections, the Hreflang "skip" section is struck (i18n shipped, and the tags followed it), and `roadmap.md` Phase 9 is now named as the authoritative record. Two gaps the audit surfaced are recorded below: JSON-LD carries no dates, and the heading-slug algorithm is unpinned by any test |
| D14 | `gomddoc-website/docs/distribution.md:36-38` shows plain `https://…git` as a Git source | Only `git://`, `git+ssh://`, `git+https://` are accepted (`config.go:379-383`) |
| D15 | `gomddoc-themes/themes/CLAUDE.md:94,150,167` reference `color-chip.mjs` | The file is `gmd-color-chip.mjs`. Copy-paste produces 404s |

**MEDIUM — incomplete or stale (abridged):** README's *"~8MB binary"* (actually 32 MB unstripped,
22 MB with GoReleaser's `-s -w`); `01-http-behavior.md` overstates Cache-Control, ETag and
compression as universal (all three are scoped — see §10.4); `13-internationalization.md` documents
3 locale layers (there are 2), 20 translation keys (there are 26), and preview on `:8080` (it is
`:auto`); `07-security.md:54` names a `BlockHiddenPaths` middleware and `provider.IsHiddenPath` — the
real names are `ContentExclusion` and `provider.IsRestrictedPath`; the website has **no CLI reference
page** (6 subcommands, 20+ flags) and omits `language`, `exclude`, `strip_extensions`, `search.index`,
`theme.vars`, `theme.features`, `meta.robots`, `server.admin_port`; ~~the website says 11 template
functions~~ (corrected to 12 in §10.4's breadcrumb commit) but still omits the `.gomddoc/partials/`
override layer; `gomddoc-themes` has no root README;
stale "Go 1.25" references (go.mod says 1.26); `CLAUDE.md:29-36` omits `docs/plans/`, and four plan
documents are misfiled under `docs/specs/`.

**Godoc:** 21 of 22 packages have **no package doc comment** — only `internal/resolve/resolver.go:1`
has one. There is no `.golangci.yml`, so no `revive`/`stylecheck` rule enforces it.

#### MEDIUM: every page shares one `<meta name="description">` — NEW (found while auditing D13)

`partials/head-shared.html.tmpl:29` emits `<meta name="description" content="{{ .Site.Meta.Description }}">`
unconditionally. Two lines above it, `og:description` and `twitter:description` both prefer
`.Page.Meta.description` and fall back to the site's — so a page that sets `description:` in
frontmatter gets it into the Open Graph tags and into `sitemap`/JSON-LD, but *not* into the one tag
search engines actually read for the result snippet. Every page on the site therefore advertises the
same description, which is the duplicate-content signal the canonical work was meant to avoid. The
fix is the three-line `if/else if` already sitting next to it. `06-auto-generated meta description`
in the SEO analysis is a separate, larger item; this is the plain frontmatter path being dropped.

#### MEDIUM: `robots.txt` points crawlers past the sitemap index — NEW (found while auditing D9)

`GenerateRobotsTxt` (`robots.go:31`) always emits `Sitemap: <domain>/sitemap.xml`, and build calls it
with no knowledge of how many languages there are (`build.go:561`). So a multi-language build writes
`sitemap-index.xml` — whose entire purpose is to be the single entry point — and then tells crawlers
to read the default-language sitemap instead, which by design lists no translated page
(§13-i18n: "the default language indexes **only** the content outside the language directories").
Translated pages are discoverable only if the crawler guesses the index URL. `generateSEOFiles`
already runs after `detectedLangs` is known; the directive should follow the same
`len(detectedLangs) > 1` condition the index file does. Serve has no index to point at, so it should
keep the current line — which makes this one of the few places serve and build *should* differ.

#### LOW: JSON-LD has no `datePublished`/`dateModified` — NEW (found while auditing D13)

`seo.JSONLDPage` carries `Title`/`Description`/`Author`/`Breadcrumbs` but no dates
(`renderer.go:592-606`), so the `TechArticle` node omits both. The data exists — `sitemap.xml` and
`feed.xml` already read `ModTime()` through the provider, and the git provider returns commit time —
so this is plumbing a value that is one `Stat` away, not a new capability. `dateModified` is the
ranking signal the sitemap `<lastmod>` is already claiming.

#### LOW: the heading-slug algorithm is documented nowhere and pinned by no test — NEW

Anchor stability is an inbound-link contract: `#installation` silently becoming `#installing`
breaks every external link to it, and nothing in the tree would go red. Both parsers enable
goldmark's `parser.WithAutoHeadingID` (`renderer/markdown.go:83`, `enricher/markdown.go:65`), so the
algorithm is goldmark's and is stable in practice — but it is *goldmark's choice*, not ours, and a
dependency bump could change it. One table-driven test over the punctuation/unicode/duplicate cases
plus a paragraph in the guide converts an implicit dependency into a stated one.

### 10.6 Test coverage & quality

**The 87% target is already met** (88.1% product code) — see the header. No coverage-chasing work is
needed; the debt is in test *quality*.

#### ~~The untracked `docs/skills/` package~~ ✅ FIXED

~~`docs/skills/favicons/scripts/favicon-check.go` is a standalone `package main` (484 lines) — Claude
Code agent tooling, not product code — that **is** in the module: `go list ./...` includes
`github.com/monolithiclab/gomddoc/docs/skills/favicons/scripts`, and its 148 uncovered statements are
the sole reason aggregate coverage reads below target. Because CI checks out without the directory,
local `make test` and CI compute permanently different totals.~~

**Decision: committed, and `docs/skills/` added to `.covignore`.** Of the three dispositions, this is
the only one that makes local and CI agree — gitignoring it would have left the local run still
walking the directory, so the two totals would keep diverging, just silently and in the other
direction. The `.agents/` precedent from `1ac1169` does not transfer: that directory was ignored
because it held business-confidential product-marketing context, and a favicon procedure has none.
Both runs now report **88.1%**.

The tool stays a real package in the module, so `go vet`, `staticcheck`, `golangci-lint`, `gosec` and
`gocritic` cover it — it is Go code in the repo and should compile and lint like the rest. What is
scoped away is the two filters that answer "which packages ship": `.covignore` drops it from the
coverage total (same rationale as `internal/testutil/`), and `make vulncheck` now scans
`./cmd/... ./internal/...` instead of `./...`. That second one was not on the list. `image/png` enters
this module's vulnerability graph *solely* through `favicon-check.go`'s `png.DecodeConfig`, and
`make vulncheck` has a dedicated CI job — so an `image/png` CVE would have blocked the product over a
tool that is never distributed (`.goreleaser.yaml` builds only `./cmd/gomddoc`; `.dockerignore`
excludes `docs/`). Both filters are now prefix contracts on `docs/skills/`, recorded in `CLAUDE.md`.

Two defects came out of committing it:

- **`SKILL.md` never referenced its own script.** The "Verification checklist" hand-rolled a `curl`
  loop plus `magick identify`, `file` and `python3 -m json.tool` — a strict subset of what
  `favicon-check.go` already asserts, which is why nothing invoked the tool. The checklist now runs
  it, and lists only the two rules it genuinely cannot check over HTTP (the maskable safe zone, and
  DevTools manifest warnings).
- **`attr()` matched attribute names inside longer ones.** Its regex anchored on `\b`, and a word
  boundary also sits between the hyphen and the `s` of `data-sizes`, so `attr(tag, "sizes")` read a
  `data-sizes` value as `sizes`. Found by the new test, not by inspection. Rather than tune the
  anchor, `attr` was replaced by `attrs`, which parses *whole* names into a map: the value lands
  under `data-sizes`, so a lookup of `sizes` misses by construction. That also retired the module's
  only in-function `regexp.MustCompile` — `attr` recompiled per call, once per candidate tag.

`favicon_check_test.go` covers the byte-level parsers — the ICO directory (including the width-byte-0
means-256 encoding), the PNG colour-type and `tRNS` alpha detection, the chunk walk's overflow guard,
the head-tag matchers and the manifest checks. `checkPNGSize` is left untested: it now takes a single
`edge` (every favicon is square), so it is a comparison against what `png.DecodeConfig` returns with
no width/height pair to transpose. The `.claude/settings.local.json` alongside the skill stays
untracked — `.gitignore` already ignores `.claude/`, and it holds machine-local absolute paths.

**Simplify pass — skipped findings**, each a real observation whose fix costs more than it returns:

- The eight asset paths are written twice (fetch list, then per-asset checks). Deduplicating them
  means restructuring `main()` around a table, well outside this diff.
- `checkPNGNoAlpha` indexes the IHDR colour-type byte rather than reusing `png.DecodeConfig`. The
  `color.Model` → colour-type mapping is not one-to-one, and the palette+`tRNS` path would still
  need its own chunk walk.
- `TestCheckPNGNoAlpha`'s three passing colour-type rows (0, 2, 3) look redundant with each other,
  but they are the complete enumeration of PNG's non-alpha types — dropping any leaves a colour type
  no test names.
- Hand-built PNG/ICO fixtures could use `binary.Append`/`png.Encode`. Byte literals are what make the
  offsets the checks index visible in the fixture.
- The eight asset fetches run sequentially. They share one keep-alive client against one host; at a
  50 ms RTT that is ~0.4 s for a command a human runs by hand.

#### ~~HIGH: unfalsifiable assertions~~ ✅ FIXED

- ~~`internal/server/sitemap_test.go:57,61` assert `Contains(body, "https://docs.example.com/")` and
  `".../docs/"`, labelled *"README.md stripped"*. Both are **prefixes** of
  `https://docs.example.com/docs/guide.md`, which line `:53` already proved is present. **Neither
  assertion can ever fail.** Same defect at `:215` and `feed_test.go:46`.~~
- ~~`internal/server/feed_test.go:103` — `TestGenerateFeed_LimitsEntries` asserts
  `if count > feedMaxEntries`. **One-sided:** a feed emitting *zero* entries passes. Should be `!=`.~~
- ~~`feed_test.go` never asserts entry **ordering** despite staggering ModTimes; reversing the sort is
  invisible. Neither `TestGenerateSitemap` nor `TestGenerateFeed` `xml.Unmarshal`s its output, unlike
  `TestGenerateSitemapIndex` (`:238`), which does it correctly.~~
- ~~`internal/metadata/index_test.go:340` — `TestBuildIndex_CancelledContext` asserts
  `if err != nil && !errors.Is(err, context.Canceled)`, so it **passes when `err == nil`**.~~

> **Fixed.** `TestGenerateSitemap` and `TestGenerateFeed` now `xml.Unmarshal` into the production
> `urlSet`/`atomFeed` and compare exhaustively — `maps.Equal` over loc→lastmod for the sitemap,
> `slices.Equal` over ordered entry IDs for the feed. Every remaining raw-string check is delimited
> (`<loc>…</loc>`, `<id>…</id>`). The finding named three sites; `grep` found five — `:140` and `:205`
> carried the same dead root-URL assertion and were fixed too. Each fix was verified by mutation
> (reverse the feed sort, `candidates = nil`, append to `tagURL`, drop the search cap).
>
> Three things the exact comparisons surfaced that the substring sweeps had hidden:
> 1. `GenerateSitemap` also emits tag pages — `/tags/` and `/tags/<tag>`, no `<lastmod>`. Never
>    mentioned by any assertion before.
> 2. A folded index URL is `/docs`, **no trailing slash**, while the tag index is `/tags/` **with**
>    one. The old comment claimed `/docs/` and was in fact matching the `/docs/guide.md` entry.
>    Build mode writes `docs/index.html`, so its real URL is `/docs/` — see §10.2.
> 3. `atomEntry.Link` had **no `xml` tag**, so `encoding/xml` fell back to the field name and every
>    entry shipped `<Link rel="alternate">`. RFC 4287 requires `<link>`; feed readers would not find
>    the alternate link. Unmarshalling through the production struct round-trips this happily, which
>    is why the raw-string `<Link` guard stays alongside the structural assertions. Fixed in
>    `feed.go` with `xml:"link"`.
>
> Also folded in from the same class: `search_test.go`'s `TestSearchEndpoint_LimitCapped` asserted
> `len(results) > maxSearchLimit` against a **three-document** fixture, so deleting
> `min(parsed, maxSearchLimit)` left it green. It now indexes `maxSearchLimit+10` matching pages and
> asserts `!= maxSearchLimit`.
>
> Still open, same class, lower value: `tags_html_test.go:158` orders bare tag names
> (`strings.Index(body, "go")`) rather than the delimited `go(1)` forms four lines above.

#### ~~HIGH: static-build tests are existence-only, over a parallel write path~~ ✅ FIXED

~~`cmd/gomddoc/build.go:376,510` write output from an errgroup; the tests never read what was written.
`TestBuildCmd_EmitsTagPages` (`build_test.go:960`) makes three `os.Stat` calls and **zero byte reads**;
`TestBuildCmd_Run_WithSubdirectories` (`:650`) five. If the errgroup raced and wrote one page's
content into another's `index.html` — the canonical bug for parallel per-file rendering — green.
`TestBuildCmd_Run_MultiLanguage` (`:990`) `os.Stat`s `fr-FR/sitemap.xml` but never inspects it, so a
per-language sitemap full of English URLs is invisible. `feed.xml` content is never asserted anywhere.
**This is precisely the failure class that shipped as a real bug in §9.5/§9.7.**~~

> **Fixed.** All four tests now read their outputs, and each asserts the *sibling's* content is
> absent so cross-contamination fails rather than passes:
> - `TestBuildCmd_Run_WithSubdirectories` — each `api/*/index.html` must carry its own `<h1>` and not
>   the other's; `assets/logo.txt` compared byte for byte.
> - `TestBuildCmd_EmitsTagPages` — `tags/go/` lists A and B, `tags/machine%20learning/` lists A and
>   **not** B, `tags/index.html` carries both links with counts `(2)`/`(1)`.
> - `TestBuildCmd_Run_MultiLanguage` — `fr-FR/sitemap.xml` and `fr-FR/feed.xml` are parsed and their
>   full URL sets compared, so a per-language document full of default-language URLs now fails.
> - `TestBuildCmd_Run_WithDomain` — replaced `Contains(sitemap, "build.example.com")` (true of any
>   non-empty sitemap) with exhaustive loc and feed-entry-ID comparisons. This is where `feed.xml`
>   content gets asserted at all for the first time.
>
> Verified by mutation: `pagesPerTag[tag]` → `AllPages()`, `"/"+lang` → `""` for the per-language
> sitemap/feed, and `fp` → `filePaths[0]` in the render errgroup each turn the suite red.
>
> The multi-language test's root-sitemap assertion originally pinned the **language-directory leak**
> as it stood: `sitemap.xml` listed `/fr-FR` and `/fr-FR/guide` alongside the default-language pages.
> That leak is now fixed (see the §10.3 finding below), and `wantRootLocs` asserts the corrected
> behaviour — the root sitemap holds default-language URLs only.

#### HIGH: silent-degradation branches untested

`cmd/gomddoc/pipeline.go:128-130,134-136,140-143` — all three `slog.Warn(...) + continue` paths are
uncovered. These are the exact branches that hid the §9.7 `fs.Sub`/`fs.StatFS` bug. The bug was fixed;
**the silent-failure mechanism that hid it was not tested.** Same class:
`internal/metadata/index.go:92-101` (concurrent-parse silent skips) and `cmd/gomddoc/build.go:218-223`,
where per-language failures `slog.Warn; continue` while default-language failures `return err` — a
build can exit 0 having produced no French site. (`build.go:221-223` also aggregates 3 of 4 counters,
silently dropping `skippedFiles`.)

#### HIGH: `navigation.Generator`'s lazy cache has zero concurrent coverage

`internal/template/navigation/navigation.go:60-63` — `cacheOnce sync.Once` guarding `cachedTree`/
`cachedPages`/`cachedIndex`, populated lazily from concurrent HTTP handlers. `navigation_test.go`
contains no goroutines, so `-race` never observes it. Highest-risk untested concurrency primitive in
the repo. Same gap, lower blast radius: `internal/server/lazybytes.go:17`,
`internal/template/themevars.go:15`, `internal/seo/url.go:18`. The pattern to copy is
`internal/template/renderer_test.go:1553` — 50 goroutines on a cold cache asserting exactly one `Set`.

**MEDIUM:** MCP `ExcludePatterns` is never set in any MCP test (grep count: 0), so dropping it from
all five call sites passes the suite — and `MCPCmd.Run` is at 0% with no `cmd/gomddoc/mcp_test.go`, so
the field has no coverage at either end of its wire. `ServeCmd.setup()`'s basic-auth branch
(`serve.go:46-52`) is never exercised with `BasicAuthFile` set across 14 call sites — dropping
`AuthStore:` from the options literal yields a silently unauthenticated server with a green suite.
SSH auth: the `PublicKeysCallback` closure never runs (`TestSetupSSHAuth_WithValidKey` generates a
real key and never reads it; it and the nonexistent-key test assert the *identical* thing), and
`TestCreateHostKeyCallback_WithKnownHosts` never invokes the callback — so whether the fail-closed
host-key control accepts a matching key and rejects a mismatched one is untested. `cloneLocked` is at
16.7%; `TestGitProvider_EnsureCloned_ConcurrentAccess` pre-populates `repo`/`tree`, so the clone-once
race is never exercised. (The *tree* race this hid is now covered by
`TestGitProvider_ConcurrentReadsAreRaceFree`; `cloneLocked` itself still needs a network clone to
reach, so its rollback path stays uncovered.)
`internal/metadata/index.go:157-160` — the `case string:` date branch has **no test** (tests only use
unquoted `date:`, which YAML decodes as `time.Time`); quoted `date: "2025-01-15"` is an ordinary
frontmatter shape.

**Tests that assert nothing** (recommend deleting rather than leaving as false confidence):
`build_test.go:685` (`if err != nil { t.Logf("acceptable") }` — both branches pass),
`preview_test.go:183` (constructs `&PreviewCmd{Open: true}`, asserts `cmd.Open`),
`pipeline_test.go:80` (46 lines of setup, zero properties asserted),
`mcp/server_test.go:437` (`Contains(text, "related")` — all three return paths contain it).

**Helper duplication (CLAUDE.md violation):** the `.gomddoc/config.yml`-in-`t.TempDir()` pair is
inlined 8× in `internal/config` (no `testhelpers_test.go`); the default-layout MapFS is re-inlined 37×
in `internal/template`; `internal/server` *has* a canonical `setupTestRenderer` yet re-inlines its
body 4× plus two near-duplicate named helpers. **Latent bug:** `server_test.go:55-57` and `:146-148`
hand-roll the registry and **omit `NewMarkdownPassthroughRenderer()`**, so those two tests exercise a
different registry than every other server test. `mime.AddExtensionType(".md", …)` appears in 4
`init()`s with **inconsistent values** (`navigation_test.go:14` registers `"text/markdown"`, the other
three `"text/markdown; charset=utf-8"`).

**Missing benchmarks on hot paths:** `metadata.BuildIndex` (runs at every startup; `search` has one,
`metadata` does not), `provider` (no benchmarks at all), `resolve` lookup, `template/breadcrumb`,
`locale.Bundle.T`. `make bench` works; 22 benchmarks across 10 packages; the §9.3 fixes are guarded
(`BenchmarkTree` 2.16 ns, `BenchmarkPrevNext` 58 ns, 0 allocs, flat across 50/200/1000 pages).

### 10.7 LOW — code quality (abridged)

- **Four different 404 shapes**: themed HTML (`handler.go:218-225`), plain-text *"File not found"*
  (`middleware.go:99-105`), `http.NotFound` (`tags_html.go`, `assets_handler.go`), and Go's default
  mux 404. A client cannot distinguish "excluded" from "missing"; only one honours the theme.
- **`/api/tags/{tag}` and `/tags/{tag}` disagree on unknown tags** — 200 with `[]`
  (`metadata.go:43-47`) vs 404 (`tags_html.go:37-41`). Same index, same question, two answers.
- **Cache headers only on some endpoints** — `/sitemap.xml`, `/feed.xml`, `/robots.txt` hand-roll
  `Set`+`WriteHeader`+`Write` with no ETag and no Cache-Control, even though all three are
  `lazyBytes`-cached immutable byte slices, i.e. ideal ETag candidates.
- **Dead code:** `navigation/flatten.go:5 FlattenPages` (duplicates `appendLeaves`),
  `template/renderer.go:743 ClearCache`, `assets/overlay.go:14 NewOverlayFS` (zero callers in all
  three repos), `locale/detect.go:43 ExtractLangFromPath` (last call site removed by §6),
  `locale/bundle.go:38 DefaultLang`, `negotiate/accept.go:50 (MediaType).String()`,
  `mcp/server.go:23 ServerDeps.SiteName` (populated by both call sites, never read),
  `server/server.go:47 HTTPServer.handler` (assigned at `:284`, never read — pins the whole handler
  graph for the server's lifetime).
- **Aliasing:** `metadata.ByPath`/`ByTag` return values aliasing the index's `Tags` slice and `Meta`
  map — `clonePage` exists and is used by `AllPages` (§9.8) but not here, and
  `search/index.go:381` documents the opposite. `template/context.go:32-38` — `meta` aliases
  `in.Enrichment.Metadata`, so `meta["title"] = …` mutates the caller's `EnrichmentData`; safe only
  because enrichment is per-request, and becomes a race the moment the §9.8 response cache lands.
- **`fs` contract violations:** `provider/overlay_fs.go:53,84,125,185` return bare `fs.ErrNotExist`
  where `io/fs` requires `*fs.PathError`, so `errors.As` consumers lose the path.
  `gitfs.go:78` leaks a raw go-git error out of `Open`.
- **Latent panics:** `search/snippet.go:100-103` guards `pos > 0` instead of `pos < len(content)` —
  brute-forced to `index out of range` with `content="あ", windowSize=1`; unreachable today but
  `generateSnippet` takes `maxLen` as a parameter. `renderer/markdown.go:128` — unchecked
  `doc.(*ast.Document)` assertion in the hot request path.
- **Unwrapped errors** (CLAUDE.md requires `%w`): `template/renderer.go:306-309,410-412,430-436`.
  `:434-436` discards the *primary theme's* parse error entirely and surfaces only the fallback's, so
  a broken theme partial reports as a missing default partial.
- **Ignored errors:** `build.go:324` uses `err == io.EOF` not `errors.Is`;
  `provider/git.go:118-121,318-321` collapse four distinct `parseGitURL` messages into a bare sentinel
  (and this is the failure that produces §10.1's nil-tree state, so the cause matters);
  `renderer/markdown_passthrough.go:49-56` drops a `yaml.Marshal` error with no log;
  `resolve/resolver.go:48-51` silently tolerates an unreadable subtree (symptom: 404s on clean URLs,
  no log line) where both peer index builders wrap and return.
- **Misleading comments/naming:** `search/snippet.go:255` says *"using insertion sort"* over a
  `slices.SortFunc` body (CLAUDE.md bans manual insertion sorts — the comment claims one exists);
  `snippet.go:14` calls byte offsets "character range" in the one file where that distinction is the
  entire difficulty; `search/tokenizer.go:60,68` says "shorter than 2 characters" over a byte check;
  `template/renderer.go:647 filterTOCNodes(nodes, min, max)` shadows the `min`/`max` builtins CLAUDE.md
  lists as target idioms; `mcp/section.go:27,104` shadows the imported `internal/text` package (the
  enricher solved the same collision with `txt "…/internal/text"`).
- **`locale.IsBCP47Dir` accepts only `ll-CC`** (`detect.go:8-23`) — `fr`, `en`, `zh-Hans`, `es-419`
  are all valid BCP 47 and all rejected. The name promises more than the implementation delivers.
- **Misc:** `.md`/`.markdown` MIME types are registered in `internal/renderer` (`markdown.go:28-29`),
  a package `resolve` does not import — so the entire clean-URL feature depends on `internal/renderer`
  happening to be linked in; move them next to the `.mjs` registration in `negotiate/mime.go`.
  `exclusion.go:81-86`'s trailing-`/` branch uses `HasPrefix`, so `drafts/*/` matches nothing and
  **fails open**, contradicting its own doc comment. `negotiate/accept.go:83-85` sorts by `q` only,
  ignoring RFC 9110 §12.5.1 specificity, so `Accept: */*, text/markdown` resolves at `*/*`.
  `build.go:286-289 guardOutputDir` returns `nil` on *any* stat error, so a permission error reads as
  "nothing to guard". No `*.test` entry in `.gitignore` (a 19 MB `template.test` artifact appeared in
  the working tree during this review).

### 10.8 `docs/architecture.md` drift

| Line | Claim | Reality |
| ---- | -------------------------------------------- | ------------------------------------------- |
| ~~230, 264~~ | ~~`navigation` template func~~ | ✅ FIXED with D4 |
| ~~238~~ | ~~`assetURL` *"(validates existence)"*~~ | ✅ FIXED — the whole FuncMap table was rewritten from `renderer.go:funcMap` |
| ~~386~~ | ~~ContentExclusion *"uses `provider.IsHiddenPath()`"*~~ | ✅ FIXED — names `IsRestrictedPath()`, and notes why the middleware alone cannot enforce exclusion |
| ~~391-398~~ | ~~Route table~~ | ✅ FIXED. Also removed a phantom `auth → mcp` subgroup — `/_mcp/` is a plain handler on `auth` — and recorded that the per-language sitemap/feed/tag routes hang off `auth`, not the language subgroup |
| ~~404~~ | ~~*"8 bundled themes"*~~ | ✅ FIXED with D10 — split into a one-row bundled table and a seven-row downloadable one |
| ~~505~~ | ~~sitemap-index~~ | ✅ FIXED — the SEO list now states it is build-only, so a multi-language site behind `serve` has per-language sitemaps and nothing indexing them |

### 10.9 Verified clean

- **All linters clean** — `gofmt`, `go vet`, `staticcheck`, `golangci-lint`, `gosec`, `gocritic`, zero
  findings repo-wide. `go test -race ./...` clean (602 test functions, one conditional skip).
  No `TODO`/`FIXME`/`XXX`/`HACK` in any non-test file.
- **Path containment** — `..`, `%2e%2e`, `..%2f`, double-encoded and backslash variants all 404 or
  redirect within the root. `IsHiddenPath` 100% covered, `SkipWalkEntry` 100%, `IsRestrictedPath`
  100%. `fs.ValidPath` guards on the git provider; `os.DirFS`+`fs.Sub` used correctly; `path` not
  `filepath` throughout `fs.FS` code (`filepath` appears only for genuine OS operations).
- **`io/fs` spec compliance** — `gitDirFile.ReadDir` is fully correct: `n <= 0` returns all remaining
  with `nil` error (never `io.EOF`), `n > 0` returns `io.EOF` only when exhausted, both branches
  `slices.Clone`.
- **goldmark concurrency** — sharing one `goldmark.Markdown` across requests is safe in both the
  renderer and the enricher: fresh `parser.Context` and `text.Reader` per call, no per-request write
  to a shared field.
- **Template caching** — `sync.Map` + `singleflight` with a correct double-check inside the flight;
  pooled buffers reset before `Put`, `slices.Clone` before return. The §5 `Configure` Won't-Fix holds.
- **Index build concurrency** — both `BuildIndex` implementations pre-size results and have each
  goroutine write a distinct index. No shared map writes, no slice growth, no race. Only `Search`'s
  candidate seeding leaks map order (§10.3).
- **Auth** — bcrypt cost and the dummy-hash constant-time path are correct; 401s verified on `/`,
  `/_mcp/`, `/api/search`, `/sitemap.xml`, `/tags/`. Unauthenticated by design and appropriately so:
  `/health/*`, `/robots.txt`, `/_assets/`.
- **Git provider** — no `exec.Command` anywhere (pure go-git); scheme allowlist; shallow single-branch
  clone with a 60s timeout; `createHostKeyCallback` **fails closed** when known_hosts is unavailable.
- **Container** — distroless `static-debian12:nonroot`, binary only, no secrets in layers, non-root by
  default. Absent `HEALTHCHECK` is correct for distroless. Release workflow permissions are scoped
  exactly to need, and the tap-token preflight is a genuinely good failure-mode design.
- **Metrics cardinality** — labels are `{method, status}` only, and `MethodFilterMiddleware` is
  chained *outside* `Metrics`, so unbounded method strings cannot reach the label set.
- **Sorting/idioms** — every sort uses `slices.SortFunc`/`SortStableFunc` with `cmp.Compare`/
  `time.Compare`; no manual insertion sorts. Modern Go is genuinely current throughout
  (`slices`/`maps`/`cmp`, `SplitSeq`/`FieldsSeq`, `atomic.Pointer[T]`, range-over-int, `b.Loop()`,
  `slog`). Zero `interface{}`. `yaml.Marshal` for all YAML output. Pointer receivers consistent on
  every type. No package-level mutable state outside sentinels, pools and immutable regexps.
- **Sentinel errors** — all classified with `errors.Is`, `Unwrap` wired so `errors.As` works
  end-to-end. `classifyCloneError` prefers typed matching, falling back to strings only for the two
  cases go-git does not export.
- **serve/build parity that *does* hold** (byte- or field-compared on `testsite/` and an i18n
  fixture): `robots.txt`; `sitemap.xml` `<loc>` sets, ordering and `<lastmod>`; `feed.xml` entry ids,
  ordering and 20-entry cap; per-language sitemap/feed when a domain is set; `/tags/` and `/tags/{tag}`
  HTML; the 404 page (shared `BuildErrorContext`); rendered markdown body, heading anchors,
  admonitions, colour chips, `<gmd-*>` components; TOC markup; tag chips on content pages; see-also
  ordering and cap; inline theme CSS/JS and `_assets/` copy; extension redirects; `redirect_from`
  including the language case; the language switcher.
- **CLAUDE.md testing conventions: substantially compliant** — zero `t.Setenv`/`t.Parallel`
  collisions across 10 sites; the single `AllocsPerRun` site is correctly non-parallel; every
  Prometheus before/after-delta test correctly omits `t.Parallel()`; all six cancellation tests use
  already-cancelled `context.WithCancel`, not sleep/timeout patterns.
- **Extensive documentation verified accurate** — the `02-configuration.md` CLI tables match the Kong
  structs exactly (including `-d, --domain`); config precedence, `theme.features` shape and key
  pattern, theme-var sanitization, `dir_index` behaviour, git URL schemes; weak-ETag semantics, gzip
  threshold/skip-list/`Vary`, Accept negotiation incl. 406, resolver-after-provider-miss ordering;
  the full route table; SEO scheme defaulting, robots body, sitemap `noindex` exclusion, feed cap and
  RFC3339, JSON-LD shape; i18n fallback chain, URL prefixing, `LanguageInfo` ordering, per-language
  index/sitemap/feed/nav/404; the GFM/admonition/colour-chip/KaTeX extension set; exactly 6 MCP tools,
  4 resources and 3 prompts with documented names and limits; the three-layer asset overlay and
  partial precedence; search UX keybindings and TF-IDF boosts. The release pipeline itself
  (`.goreleaser.yaml` ↔ README ↔ `install.sh` archive naming and checksum flow) is internally
  consistent.

### 10.10 Recommended priority order

1. ~~**§10.1 exclude bypass**~~ — **DONE.** `cfg.Site.Exclude` threaded into `resolve.Build`; the
   build-mode path disclosure went with it.
2. ~~**§10.1 git tree data race + nil-tree panic**~~ — **DONE.** `gitTreeState` is now the sole owner
   of the cached tree behind one exclusive mutex; `cloneLocked` rolls back on failure.
   `TestGitProvider_ConcurrentReadsAreRaceFree` closes the §10.6 coverage gap.
3. ~~**§10.1 `govulncheck`**~~ — **DONE.** Four modules bumped to fixed versions; `make vulncheck`
   plus a CI job gates against recurrence.
4. ~~**§10.2 `Page.Path` resolution in build**~~ — **DONE.** `resolve.PathResolver.PageURLPath` is
   now the only file-path-to-URL derivation; the four hand-rolled copies were collapsed onto it.
   Fixes canonical/og/JSON-LD/breadcrumbs **and** prev/next **and** sidebar state. Surfaced three
   new findings in §10.2 (serve's two canonical forms, `redirect_from` on index pages, search hrefs).
5. ~~**§10.3 tag-list `.md` links + default-theme `head-meta`/hreflang**~~ — **DONE.** The default
   theme no longer forks `head-meta` or `head-katex`; the language prefix moved into `contentURL`
   (`template.WithLangPrefix`), which also fixed `see-also`'s links, and per-language pipelines now
   carry their own `TemplateRenderer` on serve. Surfaced four new findings in §10.3 (nil
   `Languages`/`Features` on tag pages, the `tagURL`/`contentURL` prefix disagreement, the missing
   theme-contract test, and the undocumented raw paths in `/api/tags/{tag}`).
6. **§10.1 admin port to loopback; `install.sh` fail-closed; pin actions to SHAs.**
7. ~~**§10.3 inert env vars**~~ — **DONE.** `NewFromServeArgs` runs the whole-Config env walk, placed
   *before* the serve args so Kong-resolved flags still win, with the three opt-in booleans OR'd
   instead of assigned. Surfaced the `GOMDDOC_DIR_INDEX` vs `GOMDDOC_SITE_DIR_INDEX` dual-naming
   drift (see §10.3).
8. ~~**§10.5 D1/D2/D7**~~ — **DONE.** README's `serve` flag table was rebuilt from `serve --help`
   (it listed a non-existent `-d, --dir` and `--dev`, and omitted `--admin-port`, `--pprof`,
   `--basic-auth-file`, `--git-storage-dir`); `GOMDDOC_SERVER_DIR_INDEX` in the usage block was
   corrected to `GOMDDOC_SITE_DIR_INDEX`; the Docker section no longer claims a multi-stage build.
9. ~~**§10.6 unfalsifiable assertions**~~ — **DONE.** Sitemap/feed tests unmarshal and compare
   exhaustively, the remaining raw-string checks are delimited, the search-limit cap is tested
   against a fixture larger than the cap, and the four static-build tests read their outputs with
   sibling-content denials. Surfaced a real Atom bug (`<Link>` instead of `<link>`), the
   `/docs` vs `/docs/` index-URL split (§10.2), and tag pages nothing had ever asserted.
10. ~~**§10.3 language-directory leak**~~ — **DONE.** The default pipeline now excludes every
    detected BCP 47 directory via one `Pipeline.Exclude` list shared by all four indexes and build's
    walk; `URLRedirects` and `generateExtensionRedirects` are wired per language in the same commit,
    with redirect *targets* language-prefixed and *sources* left content-root-relative. Build stopped
    rendering translated pages twice. A language whose pipeline failed to build is now skipped
    outright instead of rendered with the default resolver (which emitted raw `.md` links).
11. **§10.4 performance** — ~~`findRelatedDocs` top-N~~ (DONE), ~~search merge-intersection~~ (DONE),
    ~~compression pool nil~~ (DONE), ~~double breadcrumb generation~~ (DONE),
    ~~compression/metrics route coverage~~ (DONE), ~~`findBestWindow` re-lowercasing~~ (DONE),
    ~~MCP TOC nav rebuild~~ (DONE), ~~git-read blob decompression~~ (DONE) — §10.4 complete.
12. ~~**Decide `docs/skills/`**~~ (§10.6) — **DONE.** Committed and added to `.covignore`, so
    `make test` reports 88.1% both locally and in CI. Committing it surfaced two defects in the
    script it had been hiding: `SKILL.md` duplicated the tool's checks instead of invoking it, and
    `attr()`'s `\b` anchor matched attribute names inside longer ones (`data-sizes` read as `sizes`).
    `make vulncheck` is now scoped to `./cmd/... ./internal/...` for the same "what ships" reason —
    `image/png` reached the product's vuln graph only through this script.
13. **§10.5 remaining doc drift, §10.7 LOW cleanups** — opportunistic.
