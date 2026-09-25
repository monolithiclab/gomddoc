# `gomddoc doctor` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status**: Implemented 2026-09-23. All 14 tasks shipped: `59fe6de` (1), `b6194ad` (2), `0381fae` (3), `21f487c` (4),
`c31f34b` (5), `7128c58` (6), `80f59f4` (7), `7b564c3` (8), `c4d0faa` (9), `e217faf` (10), `53b87b3` (11), `9ddb625`
(12), `f5a9426` (13), `2497403` (14). Follow-up fixes: `8f4d882`, `d2a3446`, `88da0d0`, `d307429`.

**Goal:** `gomddoc doctor [DIR] [--json] [--strict] [-v]` and a stdio `gomddoc_doctor` MCP tool that report every
configuration problem and the cheap content problems, each as a `diag.Finding` with location and fix.

**Architecture:** A new `internal/diag` owns the finding type, the code catalogue (the one place a code's severity is
decided) and `Log`. Producers that detect problems at runtime — `config` normalize/validate/env, `resolve.Build`,
`server.BuildRedirectMap`, the tag-route collision check — return findings; serve/preview/build/mcp log them through
`diag.Log`. `config.Inspect` loads a site the way `NewFromServeArgs` does but collects instead of failing.
`internal/doctor` adds the checks with no runtime counterpart and assembles the `Report`. `cmd` wires CLI and MCP.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3` (`yaml.Node` for line numbers, `*yaml.TypeError` for type errors),
existing `provider`, `resolve`, `metadata`, `template.ThemeFeatures`, `config.Schema()`. No new dependencies.

**Spec:** `docs/specs/2026-09-23-doctor-design.md`

## Global Constraints

- No new module dependencies.
- A producer's runtime result is unchanged by this work; it returns findings instead of calling `slog` itself, and
  its caller calls `diag.Log`. Log **levels** stay as they were; messages may gain values that were attributes.
- Every finding is built through `diag.New(code, …)`; severity comes from the catalogue, never from the call site.
- `path`, not `filepath`, for `fs.FS`; content walks use `provider.SkipWalkEntry` with the pipeline's `Exclude`.
- `info` findings are hidden unless verbose; `Summary` counts are always true.
- Exit status: 1 on any error, or any warning with `--strict`; 0 otherwise. `info` never fails.
- `gomddoc_doctor` is registered only with `SelfDocs` (stdio). Each call reloads config and content.
- `make ci` per task; commit per task (developer authorized executor commits), message ending with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

- A `config.yml` with several problems at once (unknown key + wrong type + invalid domain): every one reported, each
  with its own line (Task 5, `TestInspect` "several problems" row).
- A page with no frontmatter at all: not a frontmatter error, only the two `info` findings (Task 11 row).
- A language directory (`fr-FR/`) with its own `redirect_from` conflict: reported with the `fr-FR/` file prefix, and
  the default pipeline does not also report it (Task 7/11 rows).
- The same problem reported by a producer and by a doctor check (a string `redirect_from`): exactly one finding
  (`diag.Dedupe`, Task 11 row).
- `gomddoc_doctor` called after the config file is edited on disk: the second call sees the edit (Task 13).

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/diag/diag.go` | Create | `Severity`, `Finding`, `Catalogue`, `New`, `Log`, `Sort`, `Dedupe`, `WithFilePrefix` |
| `internal/config/config.go` | Modify | Normalize/env/validate return findings; `ValidateAll` |
| `internal/config/inspect.go` | Create | `Inspection`, `Inspect(dir)` |
| `internal/resolve/resolver.go` | Modify | `Findings()` instead of `slog.Warn` |
| `internal/server/redirect.go` | Modify | `BuildRedirectMap` returns findings |
| `internal/metadata/index.go` | Modify | export `FrontmatterBlock` |
| `cmd/gomddoc/pipeline.go` | Modify | `Pipeline.ContentRoot`, `Pipeline.Findings`, `LanguagePipeline.Findings`, `tagsContentCollision` |
| `cmd/gomddoc/{serve,build,mcp}.go` | Modify | `diag.Log` the pipeline findings |
| `internal/doctor/doctor.go` | Create | `Input`, `Report`, `Summary`, `Options`, `Run` |
| `internal/doctor/configchecks.go` | Create | unknown keys, env, theme, exclude |
| `internal/doctor/contentchecks.go` | Create | frontmatter types, missing title/description |
| `cmd/gomddoc/doctor.go` | Create | `DoctorCmd`, `runDoctor` (shared with MCP) |
| `internal/mcp/selfdocs.go` | Modify | `gomddoc_doctor` |
| `docs/guide/14-doctor.md` | Create | usage + catalogue |
| docs (02, 04, architecture, decisions, roadmap) | Modify | |

---

### Task 1: `internal/diag`

**Interfaces — Produces:**

```go
type Severity string // Error, Warning, Info
type Finding struct { Severity; Code, File string; Line int; Key, Message, Fix string } // JSON tags per spec
type CodeInfo struct { Code string; Severity Severity; Summary string }
var Catalogue []CodeInfo           // every code in the spec's catalogue, in spec order
func New(code, file string, line int, key, message, fix string) Finding // panics on an unknown code
func Log(findings []Finding)       // Error→slog.Error, Warning→slog.Warn, Info→slog.Debug; attrs code/file/line/key
func Sort(findings []Finding)      // severity (error, warning, info), file, line, code, key
func Dedupe(findings []Finding) []Finding // same code+file+key(+line) once, first wins
func WithFilePrefix(findings []Finding, prefix string) []Finding // "fr-FR" + "a.md" → "fr-FR/a.md"; skips File == ""
```

- [x] **Step 1: Tests** (`diag_test.go`): `New` takes severity from the catalogue for three codes of different
  severities and panics on `"nope"`; `Catalogue` codes are unique and each severity is valid; `Sort` orders a shuffled
  table exhaustively; `Dedupe` keeps the first of two equal findings and keeps findings that differ only in key;
  `WithFilePrefix` prefixes and leaves `File == ""` alone; `Log` emits one record per finding at the mapped level with
  `code`/`file`/`line`/`key` attrs (`logcapture.Has`).
- [x] **Step 2:** run, FAIL (package missing).
- [x] **Step 3: Implement.** Package comment: producers return findings and keep their runtime behaviour; logging is
  the caller's job through `Log`, so serve's log and `doctor` see the same finding. Catalogue codes (spec order):
  `config.parse-error` E, `config.unknown-key` E, `config.wrong-type` E, `config.invalid-value` E,
  `config.value-replaced` W, `env.invalid-value` W, `env.unknown` W, `theme.not-installed` W,
  `theme.unknown-feature` W, `theme.unknown-var` I, `content.frontmatter-invalid` E, `content.frontmatter-type` E,
  `content.redirect-conflict` E, `content.path-collision` W, `content.tags-collision` W, `content.missing-title` I,
  `content.missing-description` I, `exclude.matches-nothing` W, `target.unreachable` E, `target.read-error` E.
- [x] **Step 4:** PASS; `make ci`; commit `feat(diag): one finding type and code catalogue for doctor and the logs`.

### Task 2: `config.Normalize` returns findings

**Interfaces:** `func (c *Config) Normalize() []diag.Finding`, `func (sc *SiteConfig) Normalize() []diag.Finding`,
`normalizeHTTP() []diag.Finding`. `normalizeAdminAddr` keeps its `slog.Info` (not a problem, a documented behaviour).

- [x] **Step 1: Tests** (table): empty theme name → `config.value-replaced`, key `theme.name`, message names the
  default; each of the eight HTTP branches (≤0 and >max for four fields) → one finding keyed
  `server.http.<field>` whose message contains the configured and the default value; valid config → no findings.
  Plus `NewFromServeArgs` with `GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT=10m` still logs one Warn record containing
  "WriteTimeout" with `code=config.value-replaced` (logcapture; no `t.Parallel`, uses `t.Setenv`).
- [x] **Step 2:** FAIL (signature).
- [x] **Step 3:** Replace each `slog.Warn` with a `diag.New("config.value-replaced", ConfigFile, 0, key, msg, fix)`
  where the message folds in the old attributes ("WriteTimeout 10m0s exceeds maximum 5m0s; using default 30s") and
  fix names the range. `ConfigFile` const = `.gomddoc/config.yml` (exported, reused by Inspect/doctor) — but for
  `server.*` keys `File` is `""` (they are not file settings). Callers: `NewFromServeArgs` → `diag.Log(cfg.Normalize())`;
  `build.go:116` → `diag.Log(cfg.Site.Normalize())`.
- [x] **Step 4:** PASS; `make ci`; commit `refactor(config): Normalize reports replaced values as findings`.

### Task 3: env overrides report unparseable values

**Interfaces:** `func (c *Config) ApplyEnvOverrides() []diag.Finding`, `func (sc *SiteConfig) ApplyEnvOverrides()
[]diag.Finding`; `walkStruct(v, t, prefix string, out *[]diag.Finding)`.

- [x] **Step 1: Tests** (sequential, `t.Setenv`): `GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT=abc`, `…_MAX_HEADER_MB=x`,
  `GOMDDOC_SITE_SEARCH_INDEX=maybe` → three `env.invalid-value` findings keyed by the variable name, File `""`,
  message naming the expected type; the field keeps its previous value. `NewFromServeArgs` with a bad
  `GOMDDOC_SITE_SEARCH_INDEX` logs it **once** (the Config pass and the Site pass both see it — dedupe).
- [x] **Step 2:** FAIL.
- [x] **Step 3:** Thread `out` through `walkStruct`; `NewFromServeArgs` collects both passes and logs
  `diag.Dedupe(all)`.
- [x] **Step 4:** PASS; `make ci`; commit `refactor(config): env overrides report unparseable values as findings`.

### Task 4: `Config.ValidateAll`

**Interfaces:** internal `type fieldError struct{ key string; err error }`; `func (sc *SiteConfig) validationErrors()
[]fieldError`; `func (c *Config) serverValidationErrors() []fieldError`; `func (c *Config) ValidateAll()
[]diag.Finding` (site + server, **not** the directory check — that is `target.unreachable`). `SiteConfig.Validate()`
and `Config.Validate()` return the first error with **unchanged** text; the shutdown-timeout `slog.Warn` stays in
`validateServer`.

- [x] **Step 1: Tests:** a SiteConfig with a scheme in `meta.domain`, an `edit_url` with `ftp:`, `strip_extensions:
  [md]`, feature key `Toc`, empty `default_index` → `ValidateAll` returns five `config.invalid-value` findings with
  keys `meta.domain`, `edit_url`, `strip_extensions`, `theme.features`, `default_index` and File `.gomddoc/config.yml`;
  `Validate()` returns the first one's error text exactly as before (pin the current string). Existing validate
  tests stay green.
- [x] **Step 2:** FAIL. **Step 3:** implement. **Step 4:** PASS; `make ci`; commit
  `feat(config): ValidateAll reports every invalid value`.

### Task 5: `config.Inspect`

**Interfaces:**

```go
type Inspection struct {
    Config   *Config      // never nil; defaults where the file could not be used
    Node     *yaml.Node   // document node of config.yml; nil when absent or unparseable
    Findings []diag.Finding
    Assumed  string       // non-empty when the file was unusable and defaults stand in
}
func Inspect(dir string) Inspection
```

Steps inside: `New()`; env (Config pass); `Server.Dir = dir`; `ComputeDynamicDefaults`; read the file (absent → no
node, no finding); `yaml.Unmarshal` into a `yaml.Node` — syntax error → `config.parse-error` with the line parsed from
yaml.v3's `yaml: line N:` prefix, `Assumed` set; more than one document → `config.parse-error` at the second
document's line; otherwise `node.Decode(&cfg.Site)` **without** KnownFields — a `*yaml.TypeError` yields one
`config.wrong-type` per entry (`line N: cannot unmarshal …`), keyed by the YAML path found at that line; then site env
pass, `Normalize`, `ValidateAll`; `diag.Dedupe`. Unknown keys are doctor's check (Task 9), done on `Node`.

- [x] **Step 1: Tests** (table over temp dirs): no file → no findings, `Node == nil`; valid testsite config → no
  findings, `Node != nil`; syntax error on line 3 → one `config.parse-error` Line 3, `Assumed` non-empty, `Config`
  has defaults; `---` second document at line 4 → parse-error Line 4; "several problems" (`exclude: drafts/` line 2,
  `meta: {domain: https://x}` line 3) → `config.wrong-type` Line 2 key `exclude` **and** `config.invalid-value` key
  `meta.domain`; env bad value → `env.invalid-value` present once.
- [x] **Step 2:** FAIL. **Step 3:** implement (`internal/config/inspect.go`; helper `keyAtLine(node, line) string`
  walking mapping nodes). **Step 4:** PASS; `make ci`; commit `feat(config): Inspect loads a site collecting every
  problem`.

### Task 6: `resolve.Build` findings

**Interfaces:** `func (r *PathResolver) Findings() []diag.Finding` (a copy). Build records
`content.path-collision` for both extension-collision branches and file-shadows-directory (File = the file, Key =
clean path), and `target.read-error` for the unreadable-path branch; no `slog` left in `Build`.

- [x] **Step 1:** Update `resolver_test.go`'s log assertions to finding assertions (same cases, exact findings), and
  add one `setupPipeline` test in `cmd` asserting the collision is still logged at Warn with `code=content.path-collision`
  (so the log half is pinned where it now happens).
- [x] **Step 2:** FAIL. **Step 3:** implement; `setupPipeline` appends `resolver.Findings()` to `p.Findings` (Task 7
  adds the field — introduce `Findings []diag.Finding` and `ContentRoot fs.FS` on `Pipeline` here) and every command
  logs them (Task 7 finishes the logging sites; here log in `setupPipeline` temporarily if Task 7's aggregate is not
  in yet, then move it). **Step 4:** PASS; `make ci`; commit `refactor(resolve): report path collisions as findings`.

### Task 7: redirect and tag-route findings; one logging point per command

**Interfaces:** `func BuildRedirectMap(index, resolver, basePath) (URLRedirectMap, []diag.Finding)`;
`func tagsContentCollision(contentRoot fs.FS) []diag.Finding`; `LanguagePipeline.Findings []diag.Finding` (default
pipeline's findings plus each language's, prefixed with `WithFilePrefix(…, lang)`); `setupPipeline` no longer logs
producer findings — `setupServer`, `BuildCmd.Run` and `MCPCmd.Run` call `diag.Log(…Findings)` once.

BuildRedirectMap findings: `redirect_from` present but not a list → `content.frontmatter-type` (key
`redirect_from`); a non-string or empty item → same; two pages claiming one source → `content.redirect-conflict` on
the second page (by `AllPages` order), message naming the first; a source that is itself a page
(`resolver.Resolve(trimmed)` found, or `index.ByPath(src)` non-nil) → `content.redirect-conflict`, message "…makes
that page unreachable".

- [x] **Step 1: Tests:** `redirect_test.go` rows for each finding plus "the map is unchanged" (same entries as before
  for the same fixture); `tagsContentCollision` rows for `tags.md`, `tags/`, none; a pipeline test for a language
  directory asserting `fr-FR/` prefixed Files in `LanguagePipeline.Findings` and no duplicate from the default
  pipeline (it excludes `fr-FR/`).
- [x] **Step 2:** FAIL. **Step 3:** implement. **Step 4:** PASS; `make ci`; commit
  `feat(server): report redirect_from problems instead of dropping them`.

### Task 8: `metadata.FrontmatterBlock`

**Interfaces:** `func FrontmatterBlock(content []byte) (yamlBytes []byte, ok bool)` — the bytes between the
delimiters, exactly what `extractFrontmatter` parses (it now calls this). YAML line 1 is file line 2.

- [x] Tests: no frontmatter, valid, unterminated, CRLF, leading spaces before `---`; `extractFrontmatter` behaviour
  unchanged (existing tests). Implement, PASS, `make ci`, commit `refactor(metadata): expose the frontmatter block`.

### Task 9: `internal/doctor` core, config-key and env checks

**Interfaces:**

```go
type PipelineView struct { Lang string; Root fs.FS; Exclude []string } // Lang "" for default
type Input struct {
    Target      string
    Unreachable error          // non-nil: only target.unreachable is reported
    Inspection  config.Inspection
    Pipelines   []PipelineView
    Producer    []diag.Finding // LanguagePipeline.Findings
    Assets      fs.FS
    Environ     []string
    KnownEnvs   []string       // exact names; map settings contribute prefixes ending in "_"
    Note        string         // e.g. "Git source: checked the provider's snapshot"
}
type Options struct{ Verbose bool }
type Summary struct { Errors, Warnings, Info int; InfoHidden bool } // json per spec
type Report struct { Target string; Summary Summary; Assumed, Note string; Findings []diag.Finding }
func Run(ctx context.Context, in Input, opts Options) Report
```

Checks in this task: `config.unknown-key` — walk `Inspection.Node` mapping keys against the tree of
`config.Schema()` FileKeys (map-typed leaves `theme.vars`/`theme.features` accept any child); Line = key node's line;
fix "did you mean `X`?" when one sibling is within Levenshtein 2, else "valid keys here: a, b, c". `env.unknown` —
every `GOMDDOC_*` in `Environ` not in `KnownEnvs` and not under a known prefix. `Run`: merge Inspection findings,
producer findings and check findings; `Dedupe`; `Sort`; count; drop `Info` unless verbose.

- [x] Tests: unknown key at root/nested/`site:` wrapper with lines and suggestions (`titel`→`title`,
  `hightlighting`→`highlighting`, `zzz` → valid-keys fix); map children never unknown; env rows (known, known map
  prefix, unknown, non-GOMDDOC ignored); Run: verbose on/off gives identical Summary and differing Findings; a
  clean input yields zero findings; Unreachable → exactly one `target.unreachable`. Implement, PASS, `make ci`,
  commit `feat(doctor): report unknown config keys and env vars`.

### Task 10: theme and exclude checks

- `theme.not-installed` when `ThemeFeatures(...).Source == fallback-default` (key `theme.name`, line from node).
- `theme.unknown-feature` for config `theme.features` keys not in the scan (only when Source is template-scan or
  fallback-default), line from node; page `features:` keys are checked in Task 11 with the same helper
  `unknownFeatures(keys, scan)`.
- `theme.unknown-var` for `theme.vars` keys not in the scan's Vars.
- `exclude.matches-nothing`: for each `Site.Exclude` pattern, walk the content root (hidden paths skipped) and test
  every file and directory path with `provider.IsExcludedPath(p, []string{pattern})` (and `p + "/"` for dirs, matching
  how the provider tests directories); no match → finding keyed by the pattern, line from the node's sequence item.
- [x] Tests per code with exact findings, and negative rows (installed theme, known feature, matching pattern).
  Implement, PASS, `make ci`, commit `feat(doctor): theme and exclude checks`.

### Task 11: content checks

Per `PipelineView`: walk `Root` with `SkipWalkEntry(…, Exclude)`, `.md` files only; read (error →
`target.read-error`); `FrontmatterBlock` → `yaml.Unmarshal` into a Node (error → `content.frontmatter-invalid`, line
= yaml line + 1); for each `metadata.FrontmatterFields` key present, check by `Type`: `string` → scalar `!!str`;
`[]string` → sequence of `!!str` scalars; `date` → `metadata.ParseFrontmatterDate` non-zero on the decoded value;
`map[string]bool` → mapping of `!!bool` values → else `content.frontmatter-type` at the value's line with the expected
type as fix. Page `features` keys → `unknownFeatures`. Missing `title` (and no `^#\s+\S` line in the body) →
`content.missing-title`; missing `description` → `content.missing-description`. Files are prefixed with the
pipeline's `Lang`.

- [x] Tests: one row per code; "no frontmatter at all" → only the two info findings; `tags: foo` → frontmatter-type;
  string `redirect_from` reported by both the producer and this check → one finding after `Run`; an excluded file
  with bad frontmatter → nothing; `fr-FR/page.md` → File `fr-FR/page.md`. Implement, PASS, `make ci`, commit
  `feat(doctor): frontmatter and page metadata checks`.

### Task 12: `gomddoc doctor`

**Interfaces:** `DoctorCmd{Dir; JSON, Strict, Verbose bool; GitSSHKey, GitStorageDir string; out io.Writer}`;
`func runDoctor(ctx, app *kong.Application, dir string, gitCfg provider.GitProviderConfig, verbose bool)
doctor.Report` (shared with MCP): `config.Inspect`; provider (`target.unreachable` on error); `RootFS`;
`setupLanguagePipelines` with metadata only; views from `lp`; `KnownEnvs` from `config.Schema()`, Kong flag envs,
`scripts`-independent install vars (embed the three names as a const list with a test that reads
`scripts/install.sh` and requires equality); `Environ` = `os.Environ()`. `Run(app)` prints and returns
`exitError{code}` so Kong exits with the right status without logging "Fatal error" for a clean non-zero.

Human format: `error    .gomddoc/config.yml:4  config.unknown-key  meta.titel: unknown key` / `         fix: did you mean
`title`?`, then `2 errors, 1 warning (3 info hidden, use -v)`; "No problems found." when empty.

- [x] Tests: exit status table (clean 0; warning 0; warning+strict 1; error 1; missing dir 1); `--json` unmarshals
  into `doctor.Report`; `-v` shows info lines, summary identical; human output contains file:line and fix. Wire into
  `CLI`, check `main` exits with the code (exitError handled in `main`). Implement, PASS, manual run on `testsite`
  and on a broken temp site, `make ci`, commit `feat(cli): add gomddoc doctor`.

### Task 13: `gomddoc_doctor`

**Interfaces:** `SelfDocs.Doctor func(ctx context.Context, verbose bool) doctor.Report`; tool `gomddoc_doctor
{verbose?: bool}` → JSON; nil `Doctor` → tool not registered. `MCPCmd.Run` sets it to a closure over `runDoctor`.
`learn_gomddoc` gains "call gomddoc_doctor after editing config or content".

- [x] Tests: internal/mcp with a fake `Doctor` counting calls (two calls → two invocations; verbose passed through);
  stdio test in cmd: start `gomddoc mcp` on a temp site, call `gomddoc_doctor` (no errors), write a bad key into
  `config.yml`, call again → `config.unknown-key` present; `siteMCPServer` lists no `gomddoc_doctor`. Implement, PASS,
  `make ci`, commit `feat(mcp): add gomddoc_doctor`.

### Task 14: docs and catalogue drift

- `docs/guide/14-doctor.md` (title/description frontmatter): usage, flags, exit status, verbose, JSON shape, the
  catalogue table (code, severity, meaning, fix), MCP tool.
- Drift test (cmd): every `diag.Catalogue` code appears in `14-doctor.md`'s table with the same severity, and every
  code in the table is in the catalogue; every code is emitted by some non-test code (`grep`-free: a test that
  collects codes from all tests' findings is too loose — instead assert each catalogue code string appears in a
  non-`_test.go` Go file under `internal/` or `cmd/`, reading files via `os.DirFS("../..")`).
- `02-configuration.md` `doctor` section; `04-mcp.md` self-documentation table row; README of the guide lists page 14;
  architecture (diag flow), decisions (producers report; `-v` as first verbosity flag), roadmap (tick doctor, add
  link/anchor checking).
- [x] `make ci`, commit `docs: document gomddoc doctor`.

## Divergences from implementation

- Task 1: `target.read-error` is catalogued as a warning, not an error (`88da0d0`). `Dedupe` merges only identical
  findings, or an unlocated finding with a located one on the same code, file and key (`8f4d882`).
- Task 7: `setupLanguagePipelines` logs producer findings through `diag.Log` unless `PipelineOptions.ReportOnly`
  (set by doctor), rather than each command calling `diag.Log` itself.
- Task 9: the Levenshtein "did you mean" helper moved to `internal/text` as `text.Closest` (`8fb0529`).
- Task 12: the error type is `exitCodeError`, and `runDoctor` also takes the environment
  (`runDoctor(ctx, app, dir, gitCfg, environ, verbose)`); `runDoctorWith` serves the MCP tool from an already open
  provider. A local target that is missing or not a directory is `target.unreachable`, and a provider that fails on a
  config value skips only the content checks (`d2a3446`).
