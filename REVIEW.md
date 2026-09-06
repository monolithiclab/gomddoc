# Codebase & Architecture Review: gomddoc

- **Review Date:** 2026-07-29 (15th pass — Fresh Review; see §10). Prior passes: 2026-06-11 (14th
  pass, §9, concluded 2026-06-16), 2026-04-10 (i18n simplification review, §6, concluded), earlier
  (13th pass, §1-5, concluded).
- **Status:** 15th pass — all HIGH findings fixed; ~30 MEDIUM/LOW findings remain open (§10, listed
  under each subsection). **The prioritized remediation plan lives in `docs/roadmap.md`'s
  Implementation Strategy section** — this file tracks findings, that file tracks what to do about
  them and in what order.
- **Reviewers:** Claude Architecture Analysis (parallel review agents + independent manual
  verification of every HIGH), each pass as noted.
- **Branch:** main
- **Go Version:** 1.26 (toolchain 1.26.5)
- **Coverage:** 88.1% product code — above the 87% target.
- **Trimmed 2026-09-06:** findings marked FIXED whose substance now lives in `docs/decisions.md`,
  `docs/architecture.md`, `docs/guide/`, `CLAUDE.md`, or a named regression test were compressed to
  a one-line pointer — full narratives (reproduction steps, benchmarks, rejected alternatives) are
  in git history before this date. **Section numbers (§9.1-9.10, §10.1-10.14) are preserved
  exactly** — dozens of test files and a few production comments cite them by section number
  (`grep -rn "REVIEW.md §\|REVIEW §"`) as regression-test provenance. Do not renumber; add new
  subsections rather than reusing a number.

---

## 1-5. 13th Pass — Historical Record (concluded)

All findings FIXED or Won't-Fix/False-Positive. Scores at the time: Architecture 9.0, Code Quality
9.0, Security 9.5, Testing 9.0, Performance 8.5. Two LOW performance items were carried forward
rather than fixed — both resurfaced and were tracked in §9.3/§9.8 (one, `buildNavItems`'s per-request
nav-tree copy, is now fixed; the other, double markdown parsing for enrichment + rendering, is still
open — see §9.8). Full original write-ups are in git history before 2026-09-06.

### Won't-Fix / False-Positive register (kept to prevent re-raising)

- `HTMLRenderer.Configure` being publicly mutable post-construction — internal-only, no race exists.
- HTTP write errors swallowed — standard Go pattern (client disconnect), nothing actionable.
- `OverlayFS`'s `ReadDir`/`Open` loop duplication — different return types, abstracting would obscure.
- `for i := range t.NumField()` — idiomatic Go 1.22+ range-over-int, not an issue.
- `NormalizeMimeType` called twice per request — normalizes two different values, not redundant.
- Server package coupling / handler naming — both appropriate for the project's size and stdlib conventions.
- `$BROWSER` env var in `preview` — local dev tool, standard Unix convention, not a server-side concern.
- `collectEnvVars`/`walkStruct` duplication in `internal/config` — evaluated, too many edge cases for the gain.
- `FindAvailablePort` TOCTOU race — development tool only.
- `OverlayProvider.RootFS` allocates a new `OverlayFS` per call — cannot be cached; underlying FS can change.
- `cases.Title(language.English)` allocated per call — too lightweight to pool.
- `contentURL` resolver lookup is O(1) map access — negligible cost, not an issue.
- `SitemapHandler`/`FeedHandler` retry-capable `sync.Mutex` cache (vs `sync.Once`) — intentional, so
  a failed generation retries on the next request.

---

## 6. i18n Simplification Review (2026-04-10) — concluded

All findings FIXED: duplicated i18n helpers (`buildLanguageInfos`, `withActiveLang`, `makeTFunc`)
consolidated into `template.BuildLanguageInfos()`, `template.WithActiveLang()`, and
`locale.Bundle.TFunc()`; the 13-param `buildFile` reduced to 9 via a `buildContext` struct; the dead
`ExtractLangFromPath` call and the per-file `LanguageInfo` clone removed. Full detail in git history.

---

## 9. 14th Pass — Fresh Review (2026-06-11), concluded 2026-06-16

- **Reviewers:** 6 parallel agents (performance, security, code quality, duplication & consistency,
  test coverage, documentation drift) + manual adversarial verification of every HIGH/contested finding.
- **Scope:** feature work landed since the 13th pass (tag components, see-also/related, i18n/l10n)
  that had never been reviewed.

> **Verification note:** one agent-reported HIGH was a fabricated false positive — see §9.6/§9.7.
> Trust the verdicts here over raw agent output.

### 9.1 HIGH — Correctness

#### ~~HIGH: Per-language content pages 404 in `serve` mode (i18n content tree non-functional)~~ FIXED

The `/{lang}` prefix is now stripped before the language handler, and each language pipeline builds
its own resolver. Fixing this exposed the §9.7 provider bug (below), also fixed. Regression:
`internal/server/language_content_test.go` (per the file's own §9.1 comment).

### 9.2 MEDIUM — Consistency / serve-build parity

All FIXED: see-also links now resolve through `contentURL` (no more raw `.md` paths); `serveHTML`/
`buildFile` page-context assembly unified via `tmpl.BuildPageContext`; error-page rendering unified
via `tmpl.BuildErrorContext`; `sitemap-index.xml` now built via `xml.MarshalIndent` instead of
hand-rolled `strings.Builder`.

### 9.3 MEDIUM — Performance (recently-landed code)

- ~~Tag pages re-parse their partial template on every request~~ FIXED — partials now cached
  (keyed by `theme/name`).
- ~~`TagsIndexHandler` allocates a full page slice just to count~~ FIXED — `metadata.Index.CountByTag`.
- **`buildNavItems` deep-copies the whole nav tree per content request** — this was later fixed as
  part of §10.4's MCP/navigation work; if you're looking for the current state, `Generator.Tree()`
  is now a cached immutable tree and per-request active/open marking is the only per-request work.

### 9.4 LOW — ALL FIXED

`pageTags` now normalizes consistently with the metadata index; `findRelatedDocs` gained a result
cap; `RelatedDocs` sort unified on `metadata.CompareTitles` with a stable secondary key;
`NewHTTPServer`'s per-language `LocaleBundle` nil-panic guarded; search query truncation now
UTF-8-safe; `headingLevel`'s indentation check is now tab-aware.

### 9.5 Test coverage gaps (verified) — ALL FIXED

Multi-language `build` mode, multi-tag `tag:` AND-intersection, and `generateRedirectFiles` are now
covered (`cmd/gomddoc/build_test.go`, `internal/search/index_test.go`). The multi-language build
test immediately caught the real §9.7 bug below.

### 9.6 Documentation drift (verified) — ALL FIXED

CLAUDE.md's package structure, `make run`'s argument, the tag/search/see-also feature docs,
`decisions.md`'s stale i18n note, and the theme count were all corrected in this pass. Superseded by
the more thorough §10.5 pass.

### 9.7 ~~False positive~~ — CORRECTION: the "exception" was a real bug (FIXED)

The verification had dismissed "multi-language pipelines silently dropped" as false, on the wrong
assumption that `fs.Sub(os.DirFS(dir), lang)` returns something implementing `fs.StatFS`. It does
not — `os.dirFS` has no `Sub` method, so `fs.Sub` returns a generic non-`fs.StatFS` wrapper, and
`NewFilesystemProviderFromFS` rejected it, silently dropping every per-language pipeline. **Fixed**:
`NewFilesystemProviderFromFS` now accepts any `fs.FS`. This is the bug cited as `§9.7` throughout the
codebase (`grep -rn "§9.7"` — `cmd/gomddoc/pipeline_test.go`, `internal/metadata/index.go`,
`internal/metadata/index_test.go`, CLAUDE.md's "A `slog.Warn(…); continue` branch" bullet).

### 9.8 Carried items from earlier passes (re-confirmed against `main`)

- ~~`extractTitle` opens every markdown file on first nav build~~ FIXED — nav generator now uses
  `SetTitleLookup` against the metadata index.
- ~~`AllMappings`/`AllPages` return internals without defensive copies~~ FIXED — both now deep-copy.
- ~~`SitemapHandler`/`FeedHandler` caching pattern duplicated~~ FIXED — extracted shared `lazyBytes`.
- **Double markdown parsing for enrichment + rendering** (STILL OPEN — deferred, architectural). Each
  markdown request is parsed twice by two differently-configured goldmark instances. Not a bug, a
  performance opportunity; would need either AST sharing (architectural change) or a bounded
  response-body cache keyed by `(path, Accept, lang)`. See `docs/roadmap.md`'s prioritized plan.

### 9.9 Security — clean

No HIGH/MEDIUM issues within the trusted-content model (traced every request-driven input path:
content, search, metadata, tag, MCP, assets, auth). Two optional LOW hardening items were
implemented anyway and are FIXED: an `fs.ValidPath` guard on the git provider's `ReadFile`/`Stat`,
and length caps on the `{tag}` route / MCP string inputs.

### 9.10 Recommended priority order — COMPLETE

All items addressed; see `docs/roadmap.md`'s Maintenance note (Implementation Strategy section) for
the summary and `git log` around 2026-06-16 for the commits.

---

## 10. 15th Pass — Fresh Review (2026-07-29)

- **Reviewers:** 6 parallel agents (performance, security incl. supply chain, Go idioms/code
  quality, duplication & serve-build parity, test coverage & quality, documentation drift) +
  independent manual reproduction.
- **Scope:** whole codebase, emphasis on previously-unreviewed territory — the release/distribution
  pipeline (GoReleaser, `scripts/install.sh`, Dockerfile, GitHub Actions) and the git provider's
  concurrency model.
- **Linters:** `make lint -j8`, `go vet`, `staticcheck`, `gosec`, `gocritic` — all clean, zero
  findings. `go test -race ./...` — clean. Every finding below came from manual reading.

> **Verification note:** every HIGH was reproduced against a running binary or confirmed by reading
> the actual dependency source. Trust the verdicts here over raw agent output.

### 10.1 HIGH — Security

- ~~**`exclude` patterns bypassable via clean URLs — excluded content served**~~ ✅ FIXED (`c80981e`,
  merged to `main` 2026-09-06). `resolve.Build` now takes `Exclude` like its three peer indexes.
  Regression: `resolve.TestResolver_ExcludedPathsHaveNoMapping`,
  `server.TestHandlerResolverHonoursExclusions`. Documented in `docs/decisions.md`, `docs/roadmap.md`,
  `docs/guide/02-configuration.md`.
- ~~**Shared `*object.Tree` mutated under a read lock — data race, unrecoverable crash**~~ ✅ FIXED.
  `gitTreeState` is now the sole owner of the cached tree behind one exclusive `sync.Mutex`. See
  `docs/decisions.md` § *Serialised Git Reads*. Regression: `TestGitProvider_ConcurrentReadsAreRaceFree`.
- ~~**`ensureCloned` reports success with a `nil` tree → nil-pointer panic**~~ ✅ FIXED alongside the
  tree race — `cloneLocked` now rolls back `g.repo`/`g.storage` on failure. Regression:
  `TestGitProvider_NilTreeIsAnErrorNotAPanic`.
- ~~**15 govulncheck-reachable vulnerabilities; no vuln gate in CI**~~ ✅ FIXED. Four modules bumped;
  `make vulncheck` + a dedicated CI job now gate this. See CLAUDE.md's Commands table.
- ~~**Admin port serves `/metrics` and `/debug/pprof/*` unauthenticated**~~ ✅ FIXED. Admin listener
  defaults to loopback; `/debug/pprof/*` sits behind the credential store via shared `mountPprof`.
  Documented in `docs/guide/08-observability.md`, `docs/guide/02-configuration.md`.
  - Follow-ups filed, still open, all LOW except the bcrypt one: admin `Shutdown` ignores
    `ShutdownTimeout`; admin mux skips `RequestID`/`SecurityHeaders`; `--admin-port 0.0.0.0:PORT`
    equal to `--port` isn't detected as on-main (string-equality, not canonicalized).
  - **MEDIUM (perf, still open): unauthenticated bcrypt CPU amplification.** `htpasswd.go`'s
    dummy-hash timing defence is correct, but an unauthenticated attacker controls how often it
    runs — measured ~10× CPU amplification per bad-credential request (cost-10, concurrency 50), no
    rate limit or lockout. Fix: bounded semaphore around `Validate`, or per-IP rate limiting on 401s.
- ~~**`install.sh` fails open on checksum verification, never verifies the cosign signature**~~ ✅
  FIXED. Missing-tool branch now `die`s; `verify_signature` runs `cosign verify-blob` pinned to this
  repo's release workflow at the installed tag; `GOMDDOC_REQUIRE_COSIGN=1` makes it mandatory.
- ~~**Release workflow actions pinned to mutable tags while holding `id-token: write`**~~ ✅ FIXED —
  all action refs in both workflows are full commit SHAs; goreleaser pinned to `~> v2.17`. Dependabot
  keeps the pins current.
- **LOW cluster, still open:** `.github/workflows/ci.yml` has no `permissions:` block (add
  `contents: read`); `FilesystemProvider` still lacks the `fs.ValidPath` guard the git provider has
  (§9.9); MCP resource/prompt handlers skip the `maxArgLen` cap their tool siblings apply, and
  `search_docs` has no query cap at all; no max-file-size cap in `FilesystemProvider.ReadFile` or
  `gitTreeFS.Open` (git provider itself enforces 50MB); no SBOM/SLSA provenance, and
  `before: hooks: - go mod tidy` in the release workflow can mutate `go.sum` mid-release.

### 10.2 HIGH — serve/build parity (static builds were materially different)

- ~~**`Page.Path` keeps `.md` in build; prev/next and sidebar active-state absent from static
  builds**~~ ✅ FIXED (one root cause, three symptoms). `(*resolve.PathResolver).PageURLPath` is now
  the single file-path-to-URL derivation; `buildFile` and three other near-copies (`resolvedPagePath`,
  `contentURL`, `navigation.buildTree`) all route through it. See `docs/decisions.md` § *One
  Derivation of a Page's URL*. Regression: `TestResolver_PageURLPath`,
  `TestBuildCmd_Run_PagePathIsTheURLPath`.
- **HIGH (open, design question): build derives the URL and the output path via two different,
  independently-diverging functions.** `PageURLPath` answers "what URL does serve publish this
  file at"; `buildFile`'s `htmlPath`/`prettyOutputPath` independently answers "where does build
  write it" — they take different inputs and diverge for `strip_extensions: []` and for
  `README.md`+`index.md` collisions (all still 404 in those configs). The §10.2 fix above corrected
  the common case; this is what it doesn't reach. Deriving the URL *from* `htmlPath` is the clean
  fix, but it surfaces an unresolved design question: with `strip_extensions: []`, build renders
  every `.md` to `.html` and copies no `.md`, so build's whole URL space differs from serve's, and
  the sitemap (which uses the serve scheme) would still disagree. Either build must force extension
  stripping, or it needs its own "publishing plan" that sitemap/feed also consult. **Not yet
  documented as a decision because it hasn't been decided** — see `docs/roadmap.md`'s prioritized
  plan.
- **MEDIUM (open): `/sitemap-index.xml` is generated by build but 404s in serve.** Build writes it;
  serve never registers a route for it.
- **MEDIUM (open): per-language tag pages gated on a domain in build only.** Serve gates per-language
  tag routes on `MetaIndex != nil && LocaleBundle != nil` (no domain condition); build's
  `emitTagPages` requires `cfg.Site.Meta.Domain != ""`. A no-domain multi-language site serves tags
  fine but ships a build missing them, with every tag chip pointing at a 404.
- **MEDIUM (open): the search UI ships in static builds with no backend.** `build` doesn't set
  `EnableSearch`, but `config.FeatureEnabled` treats an absent key as enabled, so every built page
  inlines `search.mjs`, whose only data source (`/api/search`) doesn't exist on a static host.
- **MEDIUM (open): the language prefix never reaches `Page.Path` in build.** `buildFile` applies
  `outputPrefix` to `htmlPath` but not to `pagePath`, so a translated page's canonical URL points at
  the *default*-language tree while `hreflang.html.tmpl` needs `Page.Path` to be language-relative.
  `Page.Path` is overloaded between two different meanings; one needs its own `PageContext` field.
- **MEDIUM (open): a directory index has two canonical URLs in serve.** `GET /guides` and
  `GET /guides/` both 200 with different `rel="canonical"` — serve derives `Page.Path` from the
  request, not a stable identity. Build always emits the no-slash form (agreeing with the sitemap);
  the fix is serve-side (canonicalise before building the page context, or 301 one form to the other).
- **MEDIUM (open): `redirect_from` on a default-index page targets a URL that's never written.**
  `BuildRedirectMap`/`ExtensionRedirect` hand-roll the clean-path lookup without the default-index
  fold that `PageURLPath` applies, so `docs/README.md`'s redirect targets `/docs/README`, which
  static builds never write (`/docs` is the real URL). Fix: route both through `PageURLPath`.
- **LOW (open): search results link to raw `.md` paths** — `internal/search/index.go` stores the raw
  path and `search.mjs` uses it directly as `href`; same class as the (fixed) §10.3 tag-list finding,
  but search has no resolver dependency, so the mapping belongs in `internal/server/search.go`.

### 10.3 HIGH/MEDIUM — correctness (affects both paths)

- ~~**Tag pages link to raw `.md` paths and drop the language prefix**~~ ✅ FIXED — `tags-list.html.tmpl`
  now uses `contentURL`, and the language prefix moved *into* `contentURL` itself
  (`template.WithLangPrefix`), fixing `see-also.html.tmpl` for free too.
- ~~**The bundled `default` theme forks `head-meta` and loses hreflang**~~ ✅ FIXED — `head.html.tmpl`
  now calls the shared `head-meta`/`head-katex` partials like all 7 external themes.
  - **MEDIUM (open): tag pages render with nil `Languages` and nil `Features`.** `RenderTagPage`
    builds its context without going through `config.MergeFeatures`, so a site with
    `theme.features.search: false` still ships search on tag pages, and `hreflang` never appears
    there. The natural fix (render through a real `TemplateContext`) has a trap: `tagPageData.Lang`
    is `""` for the default language while `TemplateContext.Lang()` never returns `""`.
  - **LOW (open): `tagURL` and `contentURL` disagree on when a URL gets a language prefix** if a
    site's `site.language` matches a detected content directory name — currently only a latent risk,
    since `locale.DetectLanguages` doesn't exclude the default language from directory detection.
  - **LOW (open): no structural test guards that a theme calls the shared head/script blocks** — the
    `head-meta` fork above was only caught by output assertions, after it had already shipped.
  - **LOW (open): `/api/tags/{tag}` returns real file paths, undocumented as such** in `docs/guide/`.
- ~~**Language directories indexed into the *default* pipeline**~~ ✅ FIXED — routed through the
  generic exclude mechanism (`Pipeline.Exclude`) rather than teaching four indexes about BCP 47. See
  `docs/decisions.md` § *One Effective Exclude List per Pipeline*.
- **HIGH (open, verified): two declared-and-documented parameters are silently ignored.**
  `handleGetTOC` (`internal/mcp/tools.go`) never reads `GetTOCInput.Path`, despite it being in the
  generated JSON Schema and documented in `docs/guide/04-mcp.md` — a client asking for one subtree
  gets the whole site with no error. `cmd/gomddoc/pipeline.go`'s `redirectFinderAdapter` ignores its
  path argument and always does a global DFS from the root, so `/guide/` with no index redirects to
  the site's first page, not the first page under `/guide/`.
- ~~**Six advertised environment variables are inert**~~ ✅ FIXED — `NewFromServeArgs` now calls
  `cfg.ApplyEnvOverrides()` at the right point in the precedence chain (before serve args, so Kong
  flags still win). Regression: `TestNewFromServeArgs_ArgsBeatEnv`.
  - **LOW (open): `--dir-index` answers to two different env var names** (`GOMDDOC_DIR_INDEX` on the
    Kong flag vs `GOMDDOC_SITE_DIR_INDEX` on the struct tag) — same class as `GOMDDOC_DOMAIN` vs
    `GOMDDOC_SITE_META_DOMAIN`. Pick one naming scheme.
- **MEDIUM (open): `findRelatedDocs` bypasses the documented single source of truth for tag
  normalization.** `internal/enricher/markdown.go` passes raw (untrimmed) tags to `ByTag`, so a tag
  written as `"  Deployment  "` gets a working chip/listing but an empty see-also section.
- **MEDIUM (open): index pages appear in their own "related docs" — serve only.** Serve's
  self-exclusion key is the request path (`/guide/`); the index stores `/guide/README`. Build is
  correct (uses the file path), so serve and build render different see-also blocks for index pages.
- **MEDIUM (open): synthetic tag pages ignore site feature config and have no i18n switcher** — same
  root cause as the nil-Features finding under §10.3's tag-page bullet above.
- **MEDIUM (open): git submodules classified as directories → `build` aborts.** `isDir :=
  !entry.Mode.IsFile()` is also true for `filemode.Submodule`; a gitlink causes `fs.WalkDir` to
  descend, both `tree.File`/`tree.Tree` fail, and the whole build aborts. Should be
  `entry.Mode == filemode.Dir`.
- **MEDIUM (open): `.well-known` silently dropped from git-backed sites.** The git provider's hidden-file
  filter is a blanket `strings.HasPrefix(name, ".")`, missing the RFC 8615 `.well-known` exception
  the filesystem provider honours. Direct `ReadFile` still works, so the divergence is silent.
- **MEDIUM (open): search results nondeterministically ordered on ties.** `candidates` seeds from map
  iteration (randomized) with no secondary sort key; same class as the (fixed) §9.4 `CompareTitles`
  finding, missed here. Related: `feed.go` sorts by mtime with no secondary key, causing spurious
  feed reordering when mtimes tie (universal after a fresh clone).
- **MEDIUM (open): snippet body truncated on a byte boundary → invalid UTF-8** in non-ASCII
  documents (`maxSnippetBody = 8192` bytes, no rune-boundary re-alignment).
- **MEDIUM (open): canonical URLs for non-root index pages lack the trailing slash the build
  serves** — `normalizePagePath` only adds the slash at the root; build writes `docs/index.html`
  (real URL `/docs/`) but the canonical says `/docs`.
- ~~**Config loading is silently permissive**~~ ✅ FIXED — `LoadFromFile` now decodes with
  `KnownFields(true)`, catching typo'd/misplaced keys at startup.
  - **LOW cluster, still open (same class, lower severity):** locale override typos silently ignored
    (`internal/locale/bundle.go`); `ValidateFeatureKeys` checks shape, not membership
    (`color_chip: true` singular passes and does nothing); yaml.v3 error messages leak Go type names;
    no `ConfigFilePath` helper (path spelled out at 7 call sites).
- **MEDIUM (open): divergent frontmatter parsers.** `extractFrontmatter` tolerates leading whitespace
  and an indented closing `---`; `StripFrontmatter` requires both at column 0 — so for those inputs,
  raw YAML flows into the search corpus, the markdown-passthrough response, and every MCP read path.

### 10.4 MEDIUM/LOW — Performance

All HIGH/MEDIUM items in this section are ✅ **FIXED** and documented in `CLAUDE.md`'s Go Conventions
(each has a dedicated bullet) and/or `docs/decisions.md`:

- `findRelatedDocs` O(co-tagged pages) with full-copy-per-tag → bounded top-N window + `iter.Seq`
  accessor (`Index.PagesByTag`). CLAUDE.md: "Bounded results keep a sorted top-N window."
- Search rebuilding a `map[int][]posting` per query token → sorted-merge over already-sorted
  postings. CLAUDE.md: "Never build a map at query time over data an index already ordered."
- Compression buffer pool poisoned after the first request → `Write` now decides before it copies.
  CLAUDE.md: "A pooled buffer's slice header belongs to the pool, not the request."
- Breadcrumbs generated twice per request → precomputed once in `BuildPageContext`. CLAUDE.md:
  "Anything more than one consumer reads is a `PageContext` field, never a template function."
- `findBestWindow` re-lowercasing ~50 substrings per search result → folds once per document.
  CLAUDE.md: "`strings.ToLower` is not positionally aligned with its input."
- MCP TOC rebuilding the nav tree per call → shared `Pipeline.NavGenerator`. CLAUDE.md: "A cached
  object is shared through the pipeline, never reconstructed at the consumer."
- Compression/metrics covering only 2 of 8 route groups → hoisted onto `base`. CLAUDE.md: "A
  cross-cutting middleware goes on the highest `RouteGroup`."
- Git reads doing redundant blob decodes / double path resolution under the (now serialised) tree
  lock → `resolveTreeNode`/`statTreeNode`/`blobBytes` via one `FindEntry` + object-hash lookup;
  `gitTreeFS` now implements `fs.StatFS`/`fs.ReadFileFS`. CLAUDE.md: "Resolve a git path once" and
  "A read-only `fs.FS` wrapper owes `io/fs` its optional interfaces."

**Still open (LOW, opportunistic):** inline-asset cache stores raw `[]byte` (copy per render);
`HasTemplate` does an `fs.Stat` per request; `emitTagPages` renders serially; tag pages rebuilt from
scratch per request (uncached); corpus read twice at startup (metadata + search index separately);
request-scoped `context.Background()` in the breadcrumb `isDir` closure (uncancellable git clone).
Git-read follow-ups, all LOW: directory-index reads bypass `maxFileSize`/LFS guards; `gitTreeFS.Open`
has no size cap; depth-2+ paths defeat go-git's subtree memo; three slightly-inconsistent
"is this a directory" checks remain.

### 10.5 Documentation drift

All 15 lettered findings (D1-D15: stale `--dev` flag docs, wrong navigation template function
references, admin-port health-endpoint claims, the dead custom-renderers doc, Docker/theme-count/
binary-size/RAM claims, sitemap-index-as-live-endpoint, README.html-that-doesn't-exist, and more)
are ✅ **FIXED**. Corrected files: `README.md`, `CLAUDE.md`, `docs/architecture.md`,
`docs/decisions.md`, `docs/guide/*.md` (multiple), `docs/seo-competitive-analysis.md` (dated rather
than rewritten). Two items were website/themes-repo-only and not committable from here (tracked
there). Full detail in git history if needed.

Two items surfaced *during* this pass and remain **open**:

- **MEDIUM: `/_assets/` promises `immutable` caching for a year on URLs that are not
  fingerprinted.** `assetURL` emits a plain `/_assets/<name>` path with no content hash. Nothing
  in-tree trips over it (bundled/external themes all inline CSS/JS instead), but a site's own
  `.gomddoc/static/` files referenced from Markdown hit exactly this. Options: fingerprint the URL,
  downgrade to `cacheDynamic`, or key on presence of a `?v=` query. Documented as a known mismatch in
  `docs/guide/12-advanced/01-http-behavior.md`; the code fix is still open.
- **LOW: five more guide blocks transcribe a shipped data structure with nothing pinning them**
  (only one — the translation-key list — got a doc-sync test, `TestBuiltinTranslationKeysAreDocumented`).
  The other five: the 12 template functions, the `<gmd-*>` element table, the MCP tools/resources/
  prompts list (duplicated in *two* guide pages), and the ~53 `GOMDDOC_*` env var names. Generalizing
  to one table-driven test over `{docPath, anchor, extractor}` rows is the fix, not five one-off tests.

Also fixed in this pass: godoc coverage (every package now has a doc comment, gated by
`staticcheck.conf` re-enabling ST1000); one shared `<meta name="description">` across all pages (now
per-page); `robots.txt`'s `Sitemap:` directive now names whatever build actually published, via
`writeSitemapIndex`'s return value rather than a forecasting predicate; JSON-LD `dateModified`/
`datePublished` now wired end-to-end (`seo.LastModified`/`StatModTime`); the heading-slug algorithm
is now documented and pinned (`TestHeadingSlugs`).

- **MEDIUM (open, surfaced while fixing the slug test): heading anchors drop every non-ASCII
  character.** `# Café Français` → `caf-franais`; a heading with no ASCII left falls back to a
  positional `heading-1`, `heading-2`, ... — unstable across edits, on an i18n-capable server where
  non-Latin content is a supported case. No author-side escape hatch either
  (`parser.WithHeadingAttribute()` isn't enabled, so `{#custom-id}` doesn't work). Two independent
  fixes: enable `parser.WithHeadingAttribute()` (cheap, gives authors the standard escape hatch), or
  a custom `parser.IDs` that percent-encodes instead of dropping (larger — changes existing anchors,
  a breaking change for deployed sites' inbound links).

### 10.6 Test coverage & quality

**The 87% target is already met** (88.1% product code) — the debt here is test *quality*, not
coverage volume.

✅ **FIXED:** `docs/skills/` committed and `.covignore`'d (closes the local/CI coverage-total
mismatch); unfalsifiable sitemap/feed/search assertions replaced with exhaustive structural
comparisons (surfaced a real Atom `<Link>` vs `<link>` bug, fixed); static-build tests now read
their output and deny sibling cross-contamination; silent-degradation `slog.Warn; continue` branches
now tested both ways (surfaced and fixed a dropped `skippedFiles` counter); `navigation.Generator`'s
lazy cache and three others (`lazyBytes`, `themevars`, `absOutputCache` — the last missed by the
original finding) now have cold-cache concurrency tests, per CLAUDE.md's "every lazy cache owes a
cold-cache concurrency test."

✅ **FIXED (this session, 2026-09-06):** MCP `ExcludePatterns` is now tested at both ends —
`cmd/gomddoc/mcp_test.go` drives `MCPCmd.Run` over its real stdio transport and asserts `read_page`
refuses an excluded path; `internal/mcp/server_test.go`'s fixture wires an exclude list into every
index and `ServerDeps` the way `gomddoc mcp` really does.

**Still open:**

- **LOW: `absOutputCache` (`cmd/gomddoc/build.go`) is a lazy cache for a value that's computed
  eagerly** — `Run` resolves it before any goroutine exists, so the `sync.Once` never actually
  arbitrates. Moving the resolved path onto `buildContext` deletes the cache and its error branch.
- **MEDIUM: `LanguagePipeline.Languages` and `.ByLang` are allowed to disagree** — a language whose
  pipeline failed to build stays in `Languages` (used by the switcher and sitemap-index) but not in
  `ByLang`. Currently unreachable in practice (both failure branches that would trigger it are
  unreachable by construction — documented, not faked, in the test suite).
- **LOW: build re-derives the language sub-FS the pipeline already built** — `fs.Sub(contentRoot,
  lang)` in `build.go` duplicates work `setupLanguagePipelines` already did.
- **LOW: three copies of the "fail one path" `fs.FS` test fake** across `internal/resolve`,
  `internal/metadata`, `cmd/gomddoc` — candidate for an `internal/testutil/failfs` extraction.
- **MEDIUM/LOW coverage gaps:** `ServeCmd.setup()`'s basic-auth branch untested across 14 call sites;
  SSH auth callback (`PublicKeysCallback`) never actually invoked by its own test; known-hosts
  callback never invoked either; `cloneLocked` at 16.7% (needs a network clone to reach further);
  quoted-date frontmatter branch (`case string:`) has no test.
- **Tests that assert nothing** (recommend deleting): `build_test.go` (`t.Logf("acceptable")` on both
  branches), `preview_test.go` (constructs a value, asserts the field it just set), `pipeline_test.go`
  (46 lines of setup, zero assertions), `mcp/server_test.go` (asserts a substring present in all
  three possible return paths).
- **Helper duplication** (CLAUDE.md "one canonical test helper per pattern"): `internal/config` has
  no `testhelpers_test.go` (8 inline repeats); `internal/template` re-inlines a default-layout MapFS
  37 times and has a latent bug where two tests hand-roll a registry missing
  `NewMarkdownPassthroughRenderer()`, silently exercising a different registry than every other test
  in the package.
- **Missing benchmarks on hot paths:** `metadata.BuildIndex`, `internal/provider` (none at all),
  `resolve` lookup, `template/breadcrumb`, `locale.Bundle.T`.

### 10.7 LOW — code quality (abridged)

✅ **FIXED, all documented in CLAUDE.md's Go/HTTP conventions or `docs/decisions.md`:** four
disagreeing 404 shapes unified into one `server.ErrorPage` per language scope; the three-way
"is this tag known" disagreement (`/tags/{tag}` 404 vs `/api/tags/{tag}` 200-empty vs MCP null)
unified via `metadata.Index.LookupTag`; XML endpoints (`/sitemap.xml`, `/feed.xml`, `/robots.txt`)
now revalidate via `serveWithETag` instead of hand-rolled writes; 12 dead symbols deleted; `fs`
contract violations fixed (`*fs.PathError` via `fsPathErr`, `gitTreeFS`'s not-found mapping now runs
both ways); two latent panics fixed (`findBestWindow`'s dual rune-alignment bound, an unchecked
`ast.Document` type assertion); unwrapped errors now carry context via double-`%w`; several
ignored-error and misleading-comment/shadowing findings fixed (the shadowing class is now a
`gocritic -enable="builtinShadow,importShadow"` lint gate, not five hand-edits); `isLanguageDir`
narrowed to require a script/region subtag (see `docs/decisions.md` § *A Language Directory Needs a
Script or a Region Subtag*); five Misc findings (MIME registration ownership, exclusion pattern
matching, Accept-header specificity tiebreak, `guardOutputDir`'s absent-vs-unknown conflation,
missing `*.test` gitignore entry) all fixed.

**Still open:**

- **NEW: unmatched `/api/*` routes answer `text/plain`, not JSON** — a JSON client parsing an
  unexpected-path error gets a decode failure instead of `{"error": ...}`. Fix: register
  `api.Handle("/", ...)` with a JSON 404.
- **NEW: `/api/*` responses are never cached or revalidated** — `writeJSON` streams through
  `json.Encoder` with no byte slice to hash. Marshalling to bytes first and handing to
  `serveWithETag` fixes both cache headers and revalidation at once. Documented as a known exception
  in CLAUDE.md's HTTP Conventions.
- **NEW: `TemplateCache` models a boolean as a two-implementation interface** — with `Clear()`
  deleted (dead code sweep above), the interface is just `Get`/`Set` over one real store and one
  no-op; the on/off bit is already tracked three other ways along the same path. Refactor candidate,
  not a defect.
- **NEW: `AllPages` has no `iter.Seq` sibling** — 5 read-only consumers still deep-copy the whole
  corpus (`sitemap.go`, `feed.go`, `redirect.go`, `search.buildMetaLookup`, `mcp/resources.go`); MCP's
  `handleListPages` also collect-all-then-truncates against CLAUDE.md's bounded-results rule.
- **NEW: `mcp/tools.go handleRelatedPages` reimplements `enricher.findRelatedDocs`** without the
  bound or sort, so the MCP answer and the rendered "related docs" block can disagree.
- **NEW: `navigation.Generator.Tree()` hands out its cached, mutable `*NavNode` root directly** —
  unenforced contract; every consumer currently only reads.
- **NEW, lowest priority:** `PageContext.Features` aliases the site-config map (documented, nothing
  writes it); `fstest.TestFS` conformance is never run and both `fs.FS` implementations would fail
  it (`OverlayFS.Open(dir)` disagrees with `OverlayFS.ReadDir`; git tree entries report size 0 via
  `Info()`); `inline_asset.go` discards the underlying read error, so a permission failure reads as
  a missing-asset typo; `internal/template` has no `testhelpers_test.go` despite ~60 duplicated
  constructions; the four content-index builders (`metadata`, `search`, `resolve`, `navigation`)
  disagree on what an unreadable subtree means, with `navigation.go` the worst (swallows the error
  with no log at all); a few call sites still hand-roll slog capture instead of using
  `internal/testutil/logcapture`; one raw `&fs.PathError{}` construction remains in `gitfs.go`
  instead of the package's `fsPathErr` helper.

### 10.8 `docs/architecture.md` drift

✅ **FIXED** — all 5 identified drift items (navigation template function, `assetURL` claims, the
route table, theme count, sitemap-index) corrected; see the file's current content.

### 10.9 Verified clean

- **All linters clean** repo-wide (gofmt, go vet, staticcheck, golangci-lint, gosec, gocritic).
  `go test -race ./...` clean. No `TODO`/`FIXME`/`XXX`/`HACK` in any non-test file.
- **Path containment** — traversal variants all blocked; `IsHiddenPath`/`SkipWalkEntry`/
  `IsRestrictedPath` fully covered.
- **`io/fs` spec compliance**, **goldmark concurrency**, **template caching concurrency**, **index
  build concurrency** — all correct, no races.
- **Auth** — bcrypt constant-time path correct (see the CPU-amplification finding under §10.1 for
  the one open gap); 401s verified across content, MCP, search, sitemap, tags.
- **Git provider** — no `exec.Command` (pure go-git); scheme allowlist; shallow clone with timeout;
  `createHostKeyCallback` fails closed.
- **Container** — distroless, non-root, no secrets in layers; release workflow permissions scoped
  exactly to need.
- **Metrics cardinality**, **sorting/idioms**, **sentinel errors** — all clean/idiomatic.
- **serve/build parity that *does* hold** (byte- or field-compared): `robots.txt`; sitemap/feed
  `<loc>`/ordering/cap; per-language sitemap/feed; `/tags/` HTML; the 404 page; rendered markdown
  body incl. anchors/admonitions/colour-chips; TOC markup; tag chips; see-also ordering/cap; inline
  theme CSS/JS; extension redirects; `redirect_from`; the language switcher.
- **CLAUDE.md testing conventions** — substantially compliant across the whole suite.
- **Extensive documentation verified accurate** — config tables, precedence rules, HTTP semantics
  (ETag/gzip/Accept negotiation), the route table, SEO/i18n behavior, exactly 6 MCP tools/4
  resources/3 prompts, the asset-overlay precedence, search UX. The release pipeline is internally
  consistent end-to-end.

### 10.10 Recommended priority order

Superseded. The prioritized remediation plan for every open finding in this file now lives in
`docs/roadmap.md`'s Implementation Strategy section, alongside the roadmap's own unimplemented
features — one list, ranked together, rather than two lists that can drift apart.

### 10.11 NEW — language-code handling (found while fixing §10.7's language-directory predicate)

All still **open**:

- **MEDIUM: `SiteConfig.Validate` never validates `language`.** `language: english` is accepted and
  flows into `<html lang>`, the locale bundle key, `LanguageInfo.Code`, and `x-default` hreflang. A
  `language.Parse` call in `Validate` is the whole fix. Worse case: a site whose content root
  contains a directory equal to the (default) `site.language` value gets that code twice in the
  switcher and two conflicting `hreflang` tags for it.
- **LOW: `?lang=` is an exact, case-sensitive lookup** — `?lang=fr-fr` silently falls back to the
  default even though BCP 47 codes are case-insensitive and the parameter is documented as such.
- **LOW: no test runs detection → pipelines → serve end-to-end** — existing fixtures hand-build
  `LangPipelines` with codes `DetectLanguages` could never actually produce.
- **LOW: a frozen plan doc (`docs/plans/2026-04-10-i18n-l10n.md`) still shows the deleted
  `IsBCP47Dir`** — defensible to leave (it's a dated record), noted so a future grep isn't surprised.

### 10.12 NEW — RFC 9110 §12.5.1 is evaluated globally, not per candidate

Both **open**, both LOW-probability (need a client sending a wildcard *and* a deprioritized specific
type, which browsers/LLM clients don't do):

- **MEDIUM: `registry.Get` early-returns at the first matching accepted range** instead of, per
  §12.5.1, finding the most-specific range *for each candidate output type* and maximising on that
  range's `q`. Fix: drop the early return, maximise on `(that range's q, outputScore, inputScore, order)`.
- **MEDIUM: `ParseAccept` drops `q=0` entries**, so `Accept: */*, text/html;q=0` (meaning "anything
  except HTML") can't be honoured — the exclusion information is discarded before the matcher sees it.

### 10.13 NEW — `Pipeline.Exclude` consumers that still read `cfg.Site.Exclude`

All LOW (only language directories are currently added to `Pipeline.Exclude`, and translated content
being reachable isn't a leak) — **open**:

- `MCPDeps.ExcludePatterns` (both `cmd/gomddoc/pipeline.go` and `mcp.go`) reads `cfg.Site.Exclude`
  directly instead of `lp.Default`'s effective list, so MCP's restricted-path gate doesn't know about
  excluded language directories.
- `internal/server/server.go`'s default content scope has the same gap — reachable if a language
  pipeline fails to build and falls through to the default scope.
- `internal/provider/gitfs.go`'s directory-listing filter duplicates `IsHiddenPath` without its
  `.well-known` exception.

Also open, both LOW: the three exclusion-pattern branches re-derive their structure per call instead
of compiling once at pipeline construction; `DetectMIME`'s process-global registry works today only
because every caller happens to import `negotiate` — not an enforced invariant.

### 10.14 NEW — every translated page advertises the default-language feed

**MEDIUM, open:** `<link rel="alternate">` for the feed is built via `canonicalURL`, which — unlike
`contentURL` — doesn't apply the renderer's language prefix. Per-language feeds exist and are
correct (build writes them, serve serves them); nothing links to them, so a reader subscribing from
any French page gets the English feed. Same class as the (fixed) §10.3 tag-link findings, one
artifact over. Check `hreflang` and the JSON-LD `WebSite` node for the same omission while fixing it.
