# Agents Context

This file provides guidance to AI agents when working with code in this repository.

## Project Overview

`gomddoc` is a production-ready HTTP server that serves Markdown files as HTML. Go application
with a stateless, git-native architecture. No databases, no CMS, no editorial workflows.

**Key docs** (read these for deep context, don't duplicate their content here — see Documentation section below for purpose and workflow):

## Commands

```bash
make ci                     # Full pipeline: codefix + format + lint + tests (USE THIS)
make test                   # Tests with coverage (target: 87%+)
make lint -j8               # Parallelize linting
make build                  # Production binary → build/gomddoc
make bench                  # Benchmarks
make run                    # Run locally (go run ./cmd/gomddoc serve testsite)
make vulncheck              # govulncheck dependency scan (needs network; not in `make ci`)
FORCE_UPDATE=1 make lint    # Reinstall linters (same for vulncheck after a Go toolchain bump)
```

Always use Makefile targets. `make ci` is the single command to validate changes.

## Documentation

```
REVIEW.md                   # Codebase review tracker (issues, recommendations, scores)
docs/
├── architecture.md         # Component relationships, data flow, middleware, themes
├── decisions.md            # Technical choices log (with alternatives considered)
├── roadmap.md              # Phased roadmap, deferred ideas
├── guide/                  # Feature documentation (agent + human audience)
├── specs/                  # Feature specifications (written before implementation)
└── skills/                 # Agent skills (SKILL.md + scripts/) — excluded from coverage
```

**Purpose of each:**

- **REVIEW.md** — created by review tools (Claude, Gemini) and manual input. Tracks open issues,
  fixed issues, scores, and prioritized recommendations. Source of truth for what needs fixing.
- **architecture.md** — how the building blocks relate: pipeline stages, provider/renderer/template
  layering, middleware chain, theme resolution. Update when adding or restructuring components.
- **decisions.md** — log of technical choices with alternatives considered and rationale. Prevents
  repeating past mistakes. Add an entry when making a non-obvious architectural decision.
- **guide/** — feature documentation aimed primarily at agents (via MCP) so they can use gomddoc
  correctly, but useful for humans too. Update when adding user-facing features.
- **specs/** — complete feature specifications written _before_ implementation for non-trivial
  features. Written by agents, validated by the developer. Implementation follows the spec.
- **skills/** — Claude Code skills: one directory per skill, each a `SKILL.md` plus any `scripts/`
  it drives. Repo-agnostic engineering procedure, not product docs — keep business context out of
  them (that is what the gitignored `.agents/` is for). A skill's Go scripts are real packages in
  this module, so they are vetted and linted like everything else, but they are tools, not product
  code, and both "what ships" filters exclude them by path prefix: `.covignore` drops `docs/skills/`
  from the coverage total (same rationale as `internal/testutil/`) and `make vulncheck` scans
  `./cmd/... ./internal/...` rather than `./...`. Both are prefix contracts — a script placed
  anywhere but `docs/skills/<name>/scripts/` re-enters the coverage total and puts its imports back
  in the product's vulnerability graph.

**Feature workflow:**

1. Features originate from `REVIEW.md` issues or user requests (roadmap additions)
2. Write a spec in `docs/specs/`, asking the developer clarifying questions
3. Developer validates the spec
4. Implement the feature
5. Update `docs/roadmap.md`, `docs/architecture.md`, `docs/decisions.md`, and `docs/guide/` as needed

## Project Structure

```
cmd/gomddoc/           # CLI (Kong): build, info, init, mcp, preview, serve subcommands
internal/
├── assets/            # Overlay filesystem for theme overrides
├── config/            # Config loading (CLI > env > YAML > defaults), validation
├── enricher/          # Pre-rendering extraction (metadata, TOC, navigation, related docs)
├── locale/            # i18n: BCP 47 language detection, locale bundles, translation lookup
├── mcp/               # Model Context Protocol server (tools, resources, prompts)
├── metadata/          # Frontmatter indexing, tag API
├── negotiate/         # HTTP content negotiation (Accept header, MIME types)
├── provider/          # Content sources (filesystem, git, overlay)
├── renderer/          # Content renderers (markdown→HTML, passthrough)
├── resolve/           # Clean-URL ↔ real-path resolution (strip_extensions, redirects)
├── search/            # Full-text search (inverted index, TF-IDF ranking)
├── seo/               # Canonical URL building for sitemap/feed/SEO tags
├── server/            # HTTP server, handlers, middleware, RouteGroup
├── template/          # HTML rendering, caching, breadcrumbs, navigation
├── testutil/          # Shared test helpers
└── text/              # Text utilities (sanitize, title case)
testsite/              # Lorem ipsum test site for quick testing
```

**Related repositories:**

- [gomddoc-themes](git@github.com:monolithiclab/gomddoc-themes.git) — 7 additional themes (academic, gitbook, material, midnight, minimal, nord, ocean)
- [gomddoc-website](git@github.com:monolithiclab/gomddoc-website.git) — Marketing/public website

## Development Workflow

1. Make minimal, focused code changes
2. Write/update tests (`*_test.go`) — target 87%+ coverage
3. Run `make ci` — must pass before drafting commit
4. Update relevant documentation in `docs/`
5. Draft and print the Git commit message (let user commit manually)
6. Never squash commits — each commit must be atomic and self-contained

### Conventions

- **No backward compatibility concerns**: gomddoc is unpublished. No legacy shims.
- **Trusted content model**: Theme templates and markdown content are author-controlled. No
  untrusted user input reaches rendered output. XSS/injection hardening (CSP, HTML sanitizer,
  CSS sanitization) is not needed — treat these as false positives in reviews.
- **Minimal dependencies** across all repos
- **Prevent duplicated code** — extract shared helpers
- **Every content index takes `Pipeline.Exclude`, never `cfg.Site.Exclude` directly** —
  `resolve.Build`, `metadata.BuildIndex`, `navigation.NewGenerator`, `search.BuildIndex`, *and*
  build's static walk (`buildContext.exclude`). Access control cannot live in the request-path
  middleware alone: `strip_extensions` means the served URL (`/TODO`) does not match the pattern
  (`TODO.md`), so an index that ignores exclusions makes excluded content reachable. `Pipeline.Exclude`
  is `cfg.Site.Exclude` plus that pipeline's own additions (the default pipeline excludes every
  detected BCP 47 directory — each language has its own pipeline). A consumer that reads
  `cfg.Site.Exclude` instead walks content the pipeline's own resolver and indexes know nothing about:
  that is how build rendered every translated page twice.
- **Per-language redirect maps: sources unprefixed, targets prefixed** — `stripPathPrefix` removes
  `/{lang}` before the language handler runs, so `URLRedirectMap` **keys** stay content-root-relative
  while **values** must be absolute site paths. `BuildRedirectMap`'s and `ExtensionRedirect`'s
  `basePath` parameter applies to targets only.
- **Manual testing**: Use Chrome DevTools MCP, target `testsite/`
- **Options struct pattern** or **functional options**, depending on the case
- **`path` not `filepath`** for `fs.FS` operations (forward slashes per `io/fs` spec)
- **`filepath`** only for OS filesystem operations (writing files to disk)

### Go Conventions

- Modern Go 1.25+ patterns (`slices.Clone`, `strings.SplitSeq`, `b.Loop()`)
- `any` over `interface{}`
- Table-driven tests: `[]struct{...}` with `t.Parallel()`
- Sentinel errors with `errors.Is()` for classification
- Errors wrapped: `fmt.Errorf("context: %w", err)`
- Path joining: `path.Join("assets", "themes", cfg.Theme)` (each segment separate)
- **`for i := range N`** over `for i := 0; i < N; i++` (Go 1.22+ range-over-int)
- **`slices.SortFunc` + `cmp.Compare`/`time.Compare`** — no manual insertion sorts or if/else chains
- **Bounded results keep a sorted top-N window** — never collect-all, sort, then truncate on a path
  that runs per request or per file. Drop a candidate ordered after the window's worst on sight; sort
  only the window, only on admission. It also shrinks the dedup set: with a window, a duplicate can
  only land *in* the window (an evicted entry is worse than the current worst and gets rejected), so
  `slices.ContainsFunc` over N replaces a set over the whole corpus. See `findRelatedDocs`.
- **Return an `iter.Seq` accessor next to any slice-returning one** — `Index.ByTag` copies a
  `PageInfo` per page; `Index.PagesByTag` yields pointers and allocates nothing. Callers that read a
  field or two, or discard most of what they see, take the iterator. The pair only works if the
  slice-returning half copies *deeply* — a bare struct copy still shares every slice and map field,
  so `ByPath`/`ByTag` handed out an aliased `Tags` and `Meta` from an index that is immutable after
  construction, and the iterator had nothing left to be faster than. Route every such accessor
  through one `clonePage`-style helper and pin them in a single table test, listing even the ones
  that merely delegate — otherwise a later shortcut inside the delegator hides behind its
  delegatee's row. The genuinely-aliasing accessor says so in its doc comment and stays out of the
  table. Corollary for the *consumer* side: a function that writes into a caller-supplied map clones
  it, and the comment names the write — not a hypothetical future cache.
- **Never build a map at query time over data an index already ordered** — a build phase that appends
  one document at a time leaves its lists sorted, so lookups become linear merges. `search.Search`
  rebuilt a `map[int][]posting` per query token over the whole posting list; the postings were already
  ascending by `docIdx`. Where a query relies on such an ordering, say so in a comment at the loop that
  produces it (`BuildIndex` phase 3), not only at the loop that consumes it. Corollary: results derived
  from map iteration are unordered — give the final sort a deterministic tiebreaker, or serve and build
  disagree on equally-ranked hits.
- **A cached object is shared through the pipeline, never reconstructed at the consumer** — publish it
  as a `Pipeline` field and pass it down. `handleGetTOC` built its own `navigation.Generator` per MCP
  call: a fresh `sync.Once` re-walked the content *and*, missing the title lookup the pipeline installs,
  sent `buildTree` back to `extractTitle` — opening and line-scanning every `.md` file, per request. The
  tell is a constructor call inside a request handler. Corollary: when the consumer needs the cached
  object, its command must enable the pipeline stage that builds it (`gomddoc mcp` had
  `EnableNavigation: false`), and `setupPipeline`'s option table test must assert the field is non-nil —
  the two halves live in different files with no compile-time link between them.
- **`yaml.Marshal`** for YAML output — never construct YAML with `fmt.Sprintf`/`fmt.Fprintf`
  (special characters like colons, brackets produce malformed output)
- **Every field of a marshalled XML struct needs an explicit `xml:` tag** — `encoding/xml` silently
  falls back to the Go field name, so an untagged `Link atomLink` emits `<Link>`, which is not the
  Atom element. A round-trip test through the same struct reads it back happily and cannot see it.
- **Consistent behavior across code paths** — error/fallback paths must behave identically to happy
  paths (e.g., if the fast path lowercases, the error path must too)
- **Counters over string-length comparisons** — detect "nothing written" with a counter, not by
  comparing buffer length against a magic string constant (breaks silently if format changes)
- **`strings.ToLower` is not positionally aligned with its input, and can return something
  *shorter*** — Go applies simple case mapping, so `İ` (U+0130, 2 bytes) becomes `i` (1 byte). Any
  code that indexes the original with an offset found in the lowered copy — or the reverse, as
  `findBestWindow` does with `lower[pos:end]` — needs a `len(lower) == len(s)` guard, and the guard
  must come *before* the slice, not after: past enough lost bytes the slice is out of range and
  panics. Fold once and reuse it (`strings.ToLower` returns its argument when there is nothing to
  fold, so the common case is free); do not fold the same string once per loop iteration or once per
  token. `findTokenSpansRunewise` is the aligned-offset path when you actually need the positions.
- **A symbol whose only callers are `_test.go` files is dead, and its tests are what disguise it** —
  `(MediaType).Matches` had a doc comment, a table test and a benchmark, and zero production callers;
  `renderer/registry.go` hand-rolled the same RFC 9110 media-range match twice. Grep excluding
  `_test.go` before believing a symbol is live. Deletion cascades, so follow the chain: removing
  `HTMLRenderer.ClearCache` made `TemplateCache.Clear()` test-only, which made both implementations'
  `Clear` test-only, which took the interface method. Corollary: an interface left with one real
  implementation and one no-op is storing a boolean — say so where it is constructed, or collapse it.
- **One mutable object, one lock** — never guard the same value with two independent mutexes. If two
  types need it, give one type ownership and let the other borrow through it.
- **A pooled buffer's slice header belongs to the pool, not the request** — `compressionWriter` wrote
  its working slice back over the pooled header on return (`*bufPtr = cw.buf[:0]`), and every exit
  path had niled that slice first, so the pool was refilled with zero-capacity slices and its
  `cap(buf) <= maxPoolBufferSize` guard was dead code (`cap(nil)` passes any bound). Hand back the
  pointer you got and leave `*bufPtr` alone; then size the pooled buffer so the working slice cannot
  outgrow it (here: commit the compress/passthrough decision *before* appending, so the buffer never
  reaches `minCompressionSize`). A slice that grows anyway is then dropped for free instead of parked.
  Corollary: don't buffer what you can forward — when a write already answers the question the buffer
  exists to answer, commit and write through. Test it by asserting `cap()`: `len()` and the response
  body are identical whether the buffer survives or not.
- **go-git reads are writes** — `object.Tree` memoises lookups into unsynchronised maps and go-git's
  storers mutate on read. Tree/blob access needs an *exclusive* mutex; an `RWMutex` lets two
  "readers" hit a concurrent map write (an unrecoverable runtime throw). See `gitTreeState`.
- **Resolve a git path once, then work from the hash it produced** — `tree.File(p)` is `FindEntry` +
  a full object decode, so probing with it and falling back to `tree.Tree(p)` on failure throws away
  a packfile delta + zlib decode on every directory hit; `tree.Size(p)` and `tree.Tree(p)` re-walk
  the path, and `Tree.FindEntry` only consults its subtree cache from three segments up
  (`for i := len(pathParts) - 1; i > 1`), so the second walk of `docs/guide.md` re-decodes `docs`.
  `FindEntry` once, switch on `entry.Mode`, then `object.GetTree(objects, entry.Hash)` /
  `tree.TreeEntryFile(entry)` / `objects.EncodedObjectSize(entry.Hash)`. `object.Tree` keeps its
  storer unexported, which is why `gitTreeState` carries `objects` beside `tree` — one lock, set and
  cleared together. See `resolveTreeNode`, `statTreeNode`.
- **A read-only `fs.FS` wrapper owes `io/fs` its optional interfaces** — `fs.Stat` and `fs.ReadFile`
  fall back to `Open`, and for `gitTreeFS` that decoded the whole blob: sitemap and feed generation
  stat every page for a `ModTime` that is one constant, and the metadata and search index builds read
  every file in the repo through a fallback that copies the payload a second time. Implementing
  `fs.StatFS` cut `fs.Stat` from 2469 ns/8823 B to 564 ns/352 B. Assert them
  (`var _ fs.StatFS = (*T)(nil)`) — nothing else catches the regression, because the fallback is
  always correct, only slow. The same trap in reverse: a type embedding the `fs.FS` *interface* does
  not satisfy `fs.ReadDirFS`, so `fs.ReadDir` routes through `Open` (see `countingFS`).
- **Never derive a page URL from a file path by hand** — call
  `(*resolve.PathResolver).PageURLPath(realPath, defaultIndex)`. It is the only implementation of
  the clean-path lookup plus default-index fold. Four hand-rolled copies silently disagreed, which
  is how static builds shipped `.md` canonicals and dead prev/next links. (Build's
  `prettyOutputPath` is a *separate* mapping — file to output path, not file to URL. They diverge
  in some configs; see `REVIEW.md` §10.2.)
- **Templates link content through `contentURL`, never a raw path** — it applies `PageURLPath` *and*
  the renderer's language prefix, so a partial must not add `/{lang}` itself. The corollary: hand a
  per-language pipeline its **own** `TemplateRenderer` (`LangPipelineConfig.TemplateRenderer`), never
  the default one — the default resolver cannot map a page that exists only in that language, so the
  link silently degrades to a raw `.md` path pointing at the wrong tree.
- **Anything more than one consumer reads is a `PageContext` field, never a template function** —
  `funcMap` is bound at parse time and parsed templates are cached and shared across concurrent
  `Render` calls, so there is no per-render seam where a memo could live: a function with two callers
  in a layout does its work twice per request, every request. Breadcrumbs cost a provider `Stat` and
  were computed once by the breadcrumb bar and again by the JSON-LD partial. Derive it in
  `BuildPageContext` instead, next to `TOC`/`Navigation`/`PrevPage`/`RelatedDocs`. Corollary: pass
  `PageContextInput` the *producer* (`Renderer`), not the produced value — a caller can hand a
  precomputed trail that disagrees with `Path`, but it cannot hand a renderer that does.

### `io/fs` Spec Compliance

- **`ReadDir(n <= 0)`** returns all remaining entries with **`nil` error**, not `io.EOF`.
  Only `ReadDir(n > 0)` returns `io.EOF` when exhausted.
- **`fs.ValidPath`** — no leading `/`, no trailing `/`, no `..` segments, no empty segments
- **`path`** package for all `fs.FS` path operations (not `filepath`)
- **`*fs.PathError` or nothing** — every method taking a name owes one. A bare `fs.ErrNotExist`
  satisfies `errors.Is` but `errors.As` finds no path, so `fs.WalkDir` has nothing to report, and a
  test asserting only `errors.Is` cannot see the difference — assert `Op` and `Path` too. Give the
  package *one* construction (`provider.fsPathErr` + `op*` constants): two implementations spelling
  `ReadFile`'s `Op` differently is a disagreement a per-implementation helper preserves. The mapping
  onto the sentinels runs **both ways** — flattening every backend failure to `fs.ErrNotExist` makes
  a corrupt repository render as an empty site instead of failing, so map only the not-found errors
  and let the rest through (`provider.treeErr`).
- **`fstest.TestFS` is the conformance check** — a new `fs.FS` implementation runs it in its test.
  It catches what hand-written tests do not: an `Open(dir)` handle whose `ReadDir` disagrees with the
  filesystem's own `ReadDir`, a `DirEntry.Info()` that disagrees with `Stat`.

### HTTP Conventions

- **`w.Header().Add()` not `Set()`** for multi-value headers (`Vary`, etc.) — `Set` overwrites
  values from other middleware
- **Weak ETag comparison**: strip `W/` prefix before comparing opaque-tags (RFC 9110 §8.8.3.2)
- **A handler holding a complete body serves it through `serveWithETag`, never `Set`+`WriteHeader`+
  `Write`** — `/sitemap.xml`, `/feed.xml` and `/robots.txt` are generated once and cached for the
  process's lifetime, and all three hand-rolled the write: no ETag, no `Cache-Control`, so every
  crawler hit re-downloaded a document that could not have changed. The rule now lives on
  `lazyBytes.serve` rather than in however many handlers happen to wrap a `lazyBytes`, which is also
  the one place the plain-text 500 (an HTML error body on an XML endpoint is worse than a bare one)
  is written and commented. Test it with `assertRevalidates`: 200 with a body, Content-Type,
  Cache-Control and an ETag, then a conditional GET answering 304 with the same Content-Type and no
  body. Each half alone passes a broken handler — checking only the 304 status passes one that still
  ships the payload, checking only the headers passes one that ignores `If-None-Match` — and the
  three call sites that predated the helper each asserted a different subset.
  Known exception: `writeJSON` (`/api/*`) streams through a `json.Encoder` with no byte slice in
  hand, so it neither caches nor revalidates. That is recorded in REVIEW.md, not silently accepted.
- **`http.MaxBytesReader`** on any endpoint accepting request bodies — prevents memory exhaustion
- **Cap input lengths** (query params, form values) before processing — truncate, don't reject
- **`Vary` header required** when response depends on a request header (e.g., `Accept` for content
  negotiation, `Accept-Encoding` for compression)
- **A cross-cutting middleware goes on the highest `RouteGroup` that wants it, never on a leaf** —
  `Subgroup` inherits the parent's chain, so a concern attached to one leaf is a concern every
  sibling opts out of *silently*, and every route added later opts out by default. `Compression` and
  `Metrics` sat on the two content subgroups: `/tags/`, `/sitemap.xml`, `/feed.xml`, `/_assets/`,
  `/api/*` and `/robots.txt` — the most compressible payloads on the site — were served plain, with
  no `Vary`, and uncounted. They now hang off `base`. Exemptions are the exception and each carries
  its reason at the registration site (`/metrics` must not count its own scrape; health probes would
  swamp the counters). Corollary: hoisting `Metrics` above MethodFilter/ContentExclusion/
  ExtensionRedirect is what makes 405/403/301 show up in `http_requests_total` at all.
- **Every route in a language scope writes errors through the scope's one `ErrorPage`** — a status
  code chosen to hide something is undone by a body that reveals it. `ContentExclusion` returns 404
  precisely so an excluded path is indistinguishable from an absent one; because the statuses match
  by construction, the *body* is the only thing a client can still compare, and it used to differ
  (plain-text `File not found` against the handler's themed page, with the tag routes on
  `net/http`'s default as a third shape). The `Handler` *owns* its scope's writer — `NewHandler`
  builds it from the renderer, lang and `TFunc` it was given, so no caller can hand a scope a writer
  that disagrees with it — and siblings borrow it via `Handler.ErrorPage()`. `NewHTTPServer`
  therefore builds each scope's handler before the routes that borrow from it; registration order is
  irrelevant under Go 1.22+ ServeMux specificity matching. Anything rendering the same page outside
  the request path calls `ErrorPage.Render` rather than re-deriving the context — build's static
  `404.html` did the latter and drifted. Test it by requesting the *same* URL against two sites, one
  where the file exists and is excluded and one where it never existed, then comparing bodies
  byte-for-byte; asserting each is "themed" passes even when they differ. Exemptions are legitimate
  but must say why at the call site: `/_assets/` (a sub-resource fetch), the 406 (the client's
  `Accept` excluded HTML), MethodFilter's bodyless 405, and the XML endpoints.
- **When N transports ask the index the same question, the index answers it — they only choose how
  to render the answer** — "is this a known tag" had *three* answers (`/tags/{tag}` 404,
  `/api/tags/{tag}` 200 `[]`, MCP `docs://site/tag/{tag}` a successful read of JSON `null`), because
  each handler re-derived it from a raw `ByTag`. `Index.LookupTag` now owns the length bound, the
  normalization, the emptiness verdict *and* the sort; the handlers differ only in how a `false`
  looks on the wire. Put the sort in the shared lookup too — it is what makes the JSON array, the
  HTML page and the static build agree on order, and it deletes build's duplicate `slices.SortFunc`.
  Corollary: a lookup helper must normalize its argument exactly as the build phase keyed the map —
  `ByTag` lowercased where `BuildIndex` used `normalizeTag` (lowercase *plus* trim), so
  `/api/tags/%20go` missed an indexed `go` and nothing failed. And keep the length guard *ahead* of
  the normalize, so a megabyte of path segment is rejected before it is copied.

### Testing Conventions

- **No `t.Parallel()` when tests share global mutable state** (e.g., Prometheus counters, package-
  level vars) — use before/after delta patterns with sequential execution instead
- **`t.Parallel()` is incompatible with `t.Setenv`** (panics) and `testing.AllocsPerRun`\*\* (panics)
- **Context cancellation tests**: use already-cancelled `context.WithCancel`, not nanosecond
  timeouts + `time.Sleep` (deterministic, no flakiness, no unnecessary delays)
- **One canonical test helper per pattern** — don't duplicate helpers across test files; place the
  shared helper in a `testhelpers_test.go` file
- **Assertions must be falsifiable** — before adding one, ask what production change would turn it
  red. `strings.Contains(body, "https://x.com/")` is dead when every URL in the fixture starts with
  that prefix; `len(r) > max` is dead when the fixture is smaller than `max`. Parse generated
  documents (`xml.Unmarshal` into the production struct) and compare exhaustively with
  `slices.Equal`/`maps.Equal`, or delimit the substring (`<loc>…</loc>`). Keep a few raw-string
  checks for wire format — a round-trip through the production struct is blind to element names.
- **Tests over parallel write paths must read what was written, and deny the sibling's content** —
  `os.Stat` cannot see one page's HTML landing in another page's `index.html`

### After implementing changes

#### New CLI feature

1. Update [gomddoc-website](git@github.com:monolithiclab/gomddoc-website.git) if user-facing
2. Check if themes in [gomddoc-themes](git@github.com:monolithiclab/gomddoc-themes.git) need updates

#### Theme capability change

1. Check if template system needs changes
2. Update marketing site's custom theme if affected

#### Configuration change

1. Update `docs/configuration.md` in [gomddoc-website](git@github.com:monolithiclab/gomddoc-website.git)
2. Update theme READMEs in [gomddoc-themes](git@github.com:monolithiclab/gomddoc-themes.git) if relevant
