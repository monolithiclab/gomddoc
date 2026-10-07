# CLAUDE.md

gomddoc is a single-binary Go server and static site generator for Markdown documentation (content negotiation,
search, SEO output, an MCP server), released through GoReleaser to GitHub Releases, the Homebrew tap and ghcr.io.
See [README.md](README.md) for what it does and how to run it, `docs/` (table below) for the reasoning, and
`make help` for every command.

## Hard rules

- **Every content index takes `Pipeline.Exclude`, never `cfg.Site.Exclude`.** That covers `resolve.Build`,
  `metadata.BuildIndex`, `navigation.NewGenerator`, `search.BuildIndex` and build's static walk
  (`buildContext.exclude`). With `strip_extensions` the served URL (`/TODO`) doesn't match the pattern (`TODO.md`),
  so an index that ignores the pipeline's list makes excluded content reachable. `Pipeline.Exclude` is
  `cfg.Site.Exclude` plus each pipeline's own additions (the default pipeline excludes every language directory, D36).
- **Never derive a page URL from a file path by hand.** Call `(*resolve.PathResolver).PageURLPath` (D35); templates
  link through `contentURL`, which also applies the language prefix, so a partial never adds `/{lang}` itself. Each
  language pipeline gets its own `TemplateRenderer` (`LangPipelineConfig.TemplateRenderer`). Build's
  `prettyOutputPath` is a separate file-to-output mapping (review §10.2).
- **Per-language redirect maps: sources unprefixed, targets prefixed.** `stripPathPrefix` removes `/{lang}` before
  the language handler runs, so `URLRedirectMap` keys stay content-root-relative and values are absolute site paths.
- **Released software: pinned sites upgrade across versions.** Since v0.1.2 `.gomddoc/config.yml` is decoded with
  `KnownFields(true)`, so a misplaced key (a top-level `color_chips:`) is a load error, not a silent no-op. A config,
  flag or env var change is user-visible: say so in the guide and the release notes. The no-compat-shims rule covers
  gomddoc's internals, not artifacts users already installed (`scripts/install.sh` keeps verifying v0.1.0–v0.1.2's
  `.sig`/`.pem` pair, D43).
- **The release token never leaves CI.** `HOMEBREW_TAP_TOKEN` is a repository secret only; never tag or run a real
  release from a session (`goreleaser release --snapshot --clean --skip=publish,sign,docker` proves a change).
- **Tests never touch the network**, apart from `govulncheck` inside `make lint`.

## Constraints that look like bugs

- **Trusted content model.** Theme templates and Markdown are author-controlled; no untrusted input reaches rendered
  output. CSP, an HTML sanitizer or CSS sanitizing are not missing features: treat such review findings as false
  positives.
- **The `meta.Meta` goldmark extension stays in the renderer** (`internal/renderer/markdown.go`). The enricher
  extracts frontmatter; `meta.Meta` is what strips it from the rendered page. Without it, frontmatter renders as
  visible HTML.
- **Navigation sorts alphabetically, directories and files interleaved**, not directories first. Deliberate
  (`internal/template/navigation`).
- **Cache validation is content hashes only:** a weak FNV-64a ETag of the rendered bytes, never a Git commit (D18).
- **`HTTPServer.Shutdown` applies `GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT` itself**; a timeout at the call site is redundant.
- **Language directories need a script or region subtag** (`fr-FR`, `zh-Hant`), never a bare `fr`: full BCP 47
  would claim `doc/`, `api/`, `bin/`, `it/`, and a claimed directory becomes its own pipeline and vanishes from the
  default site (D37). `x/text/language`'s `Script()`/`Region()` infer subtags never written; recompose with
  `language.Compose`.
- **go-git reads are writes.** `object.Tree` memoises into unsynchronised maps, so tree and blob access holds an
  exclusive mutex (`gitTreeState`), never an `RWMutex` (D34). Resolve a path once with `FindEntry` and work from the
  hash (`resolveTreeNode`, `statTreeNode`).
- **`ContentExclusion` answers 404, and every route in a language scope writes errors through that scope's one
  `ErrorPage`** (`Handler.ErrorPage()`), so an excluded page is byte-identical to a missing one. Exemptions say why
  at the call site (`/_assets/`, the 406, MethodFilter's 405, the XML endpoints).
- **`/metrics` on `--admin-port` has no Basic Auth, by design** (pprof keeps it). The guide warns never to expose
  the admin port.

## Traps `make ci` won't explain

- **Package comments say the non-obvious thing.** ST1000 fails a package without one; the bar is the contract a
  caller gets wrong (why `negotiate` owns the `.md` MIME registration), not "Package x does x".
- **`#nosec Gxxx -- reason`, always with the reason.** The shared response writers carry `#nosec G705` because
  gosec reads `w.Write` as XSS taint (`server/etag.go`, `server/compression.go`); the trusted content model is why.
- **A process-global registration lives in the package that owns its accessor.** `mime.AddExtensionType` for `.md`
  is in `internal/negotiate/mime.go`; a second one in a test file wins for that whole test binary.
- **`funcMap` is bound at parse time and templates are shared**, so anything more than one consumer reads is a
  `PageContext` field derived in `BuildPageContext`, never a template function.
- **A cached object travels through the `Pipeline`**, never rebuilt by a consumer (`Pipeline.NavGenerator`); a
  command that consumes it must enable the stage, and `setupPipeline`'s table test asserts the field is set.
- **A cross-cutting middleware goes on the highest `RouteGroup`** (`base`); on a leaf, every sibling silently opts
  out.
- **A handler with a complete body serves it through `serveWithETag`** (`lazyBytes.serve`) and is tested with
  `assertRevalidates`; `/api/*` (`writeJSON`) is the recorded exception.
- **N transports asking the same question share one index answer** (`Index.LookupTag` owns bound, normalisation,
  verdict and sort; the handlers only render it).
- **One implementation per rule:** media-range precedence is `MediaType.Specificity()`; "last modified" is
  `seo.LastModified`; the sitemap path is what `writeSitemapIndex` returns, never a predicate forecasting it;
  `fs.PathError`s come from `provider.fsPathErr`.
- **`docs/skills/<name>/scripts/` is a path-prefix contract.** Those Go scripts are linted, but `.covignore` and
  `VULNCHECK_PACKAGES` exclude them by prefix; a script anywhere else re-enters the coverage total and the shipped
  vulnerability graph. `internal/testutil/` is excluded from coverage the same way.
- **`docs/` is a Go package** (`guide.go` embeds `docs/guide/`): every guide page needs `title` and `description`
  frontmatter (`guide_test.go`), and `cmd/gomddoc`'s drift tests compare the guide with the code.
- `make lint` and `make ci` need the network (govulncheck). `COVERAGE_MIN = 87` is enforced by `make test`.

## Where things go

- A command: `cmd/gomddoc/<name>.go`, wired in `main.go`'s Kong struct, tested through parse and dispatch; its
  reference in `docs/guide/02-configuration.md`.
- A config setting: a field with a `doc` tag (and `max`/`default_doc`); `config.Schema()`, `info`, the MCP report and
  `doctor` derive from it (D41). A flag that sets a differently-named setting carries `setting:"<key>"`.
- A detectable problem: the producer returns a `diag.Finding` with a code from the `diag` catalogue; serve logs it
  and `doctor` reports it (D42). Never re-implement a check in `internal/doctor`.
- A user-facing feature: a page or section in `docs/guide/` (it ships in the binary). Then check the sibling repos:
  `gomddoc-website` (`docs/configuration.md` for a config change) and the seven `gomddoc-themes` (template or
  feature changes).
- Shared test helpers: `testhelpers_test.go` per package, `internal/testutil/<name>` once a second package needs one.
  Lazy caches get a cold-cache test through `testutil/fanout.Run`; log assertions go through
  `testutil/logcapture`.
- Manual UI checks: Chrome DevTools MCP against `make run` (`testsite/`).

## Documentation

| Document | Owns |
| --- | --- |
| `README.md` | What gomddoc is, status, install, the docs index |
| `docs/guide/` | The user manual, embedded in the binary (`gomddoc help`, MCP); update with every user-facing change |
| `docs/architecture.md` | Package map, commands, pipelines, middleware chain, data flow, themes, repository layout |
| `docs/decisions.md` | Append-only, D-numbered decisions; D0 indexes the decision tables kept elsewhere |
| `docs/superpowers/specs/`, `plans/` | Feature designs (`-design`) and plans, each plan ending in `## Execution notes` |
| `docs/reviews/` | Review rounds; their open findings live in `ROADMAP.md` |
| `docs/research/` | External research (the SEO competitive analysis) |
| `docs/skills/` | Claude Code skills with their scripts; procedure, no business context (that is the ignored `.agents/`) |
| `ROADMAP.md` | Ideas, the ranked backlog (Implementation Strategy) and open review findings |

Go lessons that apply beyond this repo live in the `go-cli-development` skill's `lessons.md`, not here.

## Workflow

1. Non-trivial feature: a spec in `docs/superpowers/specs/`, validated by Nicolas, then a plan in `plans/`
2. Minimal change, with tests, plus the owning doc (guide, architecture, decisions) in the same commit
3. `make ci` green
4. Commit `type(scope): summary` with the session's trailers; never squash, push only when asked
