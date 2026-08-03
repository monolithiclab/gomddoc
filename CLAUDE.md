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
└── custom-renderers.md     # (legacy — should be folded into guide/)
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
- **Every content index takes `cfg.Site.Exclude`** — `resolve.Build`, `metadata.BuildIndex`,
  `navigation.NewGenerator`, `search.BuildIndex`. Access control cannot live in the request-path
  middleware alone: `strip_extensions` means the served URL (`/TODO`) does not match the pattern
  (`TODO.md`), so an index that ignores exclusions makes excluded content reachable
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
- **`yaml.Marshal`** for YAML output — never construct YAML with `fmt.Sprintf`/`fmt.Fprintf`
  (special characters like colons, brackets produce malformed output)
- **Every field of a marshalled XML struct needs an explicit `xml:` tag** — `encoding/xml` silently
  falls back to the Go field name, so an untagged `Link atomLink` emits `<Link>`, which is not the
  Atom element. A round-trip test through the same struct reads it back happily and cannot see it.
- **Consistent behavior across code paths** — error/fallback paths must behave identically to happy
  paths (e.g., if the fast path lowercases, the error path must too)
- **Counters over string-length comparisons** — detect "nothing written" with a counter, not by
  comparing buffer length against a magic string constant (breaks silently if format changes)
- **One mutable object, one lock** — never guard the same value with two independent mutexes. If two
  types need it, give one type ownership and let the other borrow through it.
- **go-git reads are writes** — `object.Tree` memoises lookups into unsynchronised maps and go-git's
  storers mutate on read. Tree/blob access needs an *exclusive* mutex; an `RWMutex` lets two
  "readers" hit a concurrent map write (an unrecoverable runtime throw). See `gitTreeState`.
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

### `io/fs` Spec Compliance

- **`ReadDir(n <= 0)`** returns all remaining entries with **`nil` error**, not `io.EOF`.
  Only `ReadDir(n > 0)` returns `io.EOF` when exhausted.
- **`fs.ValidPath`** — no leading `/`, no trailing `/`, no `..` segments, no empty segments
- **`path`** package for all `fs.FS` path operations (not `filepath`)

### HTTP Conventions

- **`w.Header().Add()` not `Set()`** for multi-value headers (`Vary`, etc.) — `Set` overwrites
  values from other middleware
- **Weak ETag comparison**: strip `W/` prefix before comparing opaque-tags (RFC 9110 §8.8.3.2)
- **`http.MaxBytesReader`** on any endpoint accepting request bodies — prevents memory exhaustion
- **Cap input lengths** (query params, form values) before processing — truncate, don't reject
- **`Vary` header required** when response depends on a request header (e.g., `Accept` for content
  negotiation, `Accept-Encoding` for compression)

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
