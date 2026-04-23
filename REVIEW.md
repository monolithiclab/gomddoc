# Codebase & Architecture Review: gomddoc

- **Review Date:** 2026-04-08 (13th pass -- Fresh Review)
- **Reviewers:** Claude Architecture Analysis (6 parallel review agents)
- **Branch:** main
- **Go Version:** 1.26.1
- **Coverage:** 88.6% overall
- **Source LoC:** ~8,025 (production) + ~19,232 (tests)

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
