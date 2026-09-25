# `gomddoc doctor` (Self-Documentation Sub-Spec 2 of 3)

**Status**: Implemented 2026-09-23 (commits `59fe6de` through `2497403`, with follow-up fixes `8f4d882`, `d2a3446`,
`88da0d0`, `d307429`). Design approved 2026-09-23. Link and anchor checking is still not implemented (open roadmap
item). Sub-spec 3 (`gomddoc help <topic>`) shipped in `37250b5` with no spec document.
**Roadmap entry**: "Self-Documentation via MCP" → `gomddoc doctor`
**Builds on**: `docs/specs/2026-09-23-self-documentation-design.md` (capabilities report, `config.Schema()`,
`template.ThemeFeatures`, `metadata.FrontmatterFields`, the stdio-only `gomddoc://` namespace)
**Deferred**: link and anchor checking (follow-up), `gomddoc help <topic>` (sub-spec 3)

## Goal

One command — and one MCP tool — that analyses a site and reports **every** problem with its configuration and the
cheap-to-detect problems in its content, each with a location and a concrete fix. Agents first: an agent that has just
edited `.gomddoc/config.yml` or a page calls `gomddoc_doctor` (or `gomddoc doctor --json`) and gets a machine-readable
list of what is wrong and how to fix it.

Today problems surface three ways, none of them usable for this: `Validate` stops at the first error, `Normalize`
silently replaces bad values and logs a warning, and several content problems are logged (resolver collisions, tag-route
collisions) or dropped outright (`redirect_from` given as a string, two pages claiming the same redirect source,
unparseable frontmatter at `Debug`).

## Principles

1. **The code that detects a problem reports it.** A check that has a runtime counterpart is not re-implemented in
   `doctor`: the producer (`config.Normalize`, `config.Validate`, the env walker, `resolve.Build`,
   `server.BuildRedirectMap`, the tag-collision check) returns findings; `serve`/`preview`/`build` log them exactly as
   they log today, and `doctor` collects them. What `doctor` reports is by construction what `serve` would hit.
2. **One finding type** (`diag.Finding`) for every producer and every output — human, JSON, MCP.
3. **Report, don't stop.** `doctor` collects everything; only an unreachable target prevents content checks, and even
   that is a finding in a well-formed report.
4. **Fresh every time.** Each run (CLI or MCP call) reloads config and content; nothing is trusted from process start.

## Components

### `internal/diag` (new)

```go
type Severity string

const (
    Error   Severity = "error"
    Warning Severity = "warning"
    Info    Severity = "info"
)

type Finding struct {
    Severity Severity `json:"severity"`
    Code     string   `json:"code"`           // stable identifier, e.g. "config.unknown-key"
    File     string   `json:"file,omitempty"` // content-root relative: ".gomddoc/config.yml", "guide/setup.md"
    Line     int      `json:"line,omitempty"` // 1-based; 0 when not applicable
    Key      string   `json:"key,omitempty"`  // "meta.domain", "GOMDDOC_SITE_THEME", "tags"
    Message  string   `json:"message"`
    Fix      string   `json:"fix,omitempty"`  // what to change, concretely
}

// Log writes findings through slog at the matching level (Error→Error, Warning→Warn, Info→Debug), with code, file,
// line and key as attributes. serve/preview/build call it where they used to call slog directly.
func Log(findings []Finding)
```

The package comment states the contract: a producer returns findings *and* keeps its runtime behaviour; logging is the
caller's job through `Log`, so the same finding reaches both `doctor` and the server log.

### Producers that gain findings

| Producer | Today | After |
|----------|-------|-------|
| `config.(*Config).Normalize` | `slog.Warn` per replaced value | returns `[]diag.Finding` (`config.value-replaced`); callers `diag.Log` them |
| `config.(*Config).Validate` / `SiteConfig.Validate` | first error | new `ValidateAll() []diag.Finding` (`config.invalid-value`); `Validate()` stays the first-error wrapper used by `NewFromServeArgs` |
| env walker (`walkStruct`) | `slog.Warn` on unparseable values | reports `env.invalid-value` findings through the load result |
| `config.SiteConfig.LoadFromFile` | first decode error | also returns the decoded `*yaml.Node` so findings can carry line numbers |
| `resolve.Build` | `slog.Warn` on extension collision / file shadows directory | returns findings (`content.path-collision`) via `BuildOptions` or a result field |
| `server.BuildRedirectMap` | silently ignores non-list `redirect_from`, last writer wins on duplicate sources | returns findings (`content.frontmatter-type`, `content.redirect-conflict`) |
| `warnTagsContentCollision` (cmd) | `slog.Warn` | `tagsContentCollision` returns findings (`content.tags-collision`) |

The exact signature change per producer is a plan decision; the rule is that the producer's runtime result is
unchanged and its findings are returned, never only logged. Each existing `slog.Warn` above is replaced by a
`diag.Log` of the returned findings at the call site, so server logs keep the same level and gain `code`.

A config-loading entry point for doctor, `config.Inspect(dir) (Inspection)`, runs the same steps as
`NewFromServeArgs` but collects instead of failing: the decoded node, parse errors, unknown keys, type errors,
`ValidateAll`, `Normalize`'s replacements and env findings, plus the resulting `*Config` (defaults where the file could
not be used, and a note saying so).

### `internal/doctor` (new)

```go
type Input struct {
    Target     string        // DIR or Git URL, as given
    Inspection config.Inspection
    Content    fs.FS         // nil when the target is unreachable
    Assets     fs.FS
    Pipelines  []PipelineView // default + each language: name, root FS, Pipeline.Exclude, producer findings
    Environ    []string      // os.Environ(), injected for tests
    KnownEnvs  []string      // settings + Kong flag envs + install.sh vars, built by cmd
}

type Report struct {
    Target   string         `json:"target"`
    Summary  Summary        `json:"summary"` // true counts, including hidden info
    Assumed  string         `json:"assumed,omitempty"` // e.g. "config.yml did not parse; content checked with defaults"
    Findings []diag.Finding `json:"findings"` // sorted: severity, file, line, code
}

type Summary struct {
    Errors     int  `json:"errors"`
    Warnings   int  `json:"warnings"`
    Info       int  `json:"info"`
    InfoHidden bool `json:"info_hidden"`
}

func Run(ctx context.Context, in Input, opts Options) Report // Options{Verbose bool}
```

Checks owned by `doctor` (no runtime counterpart): unknown config keys, unknown env vars, theme checks, exclude
patterns matching nothing, frontmatter types and missing title/description. `Run` merges them with the producers'
findings, sorts, counts, and drops `info` findings from `Findings` unless `Verbose` (counts stay true).

### CLI

```text
gomddoc doctor [DIR] [--json] [--strict] [-v|--verbose]
```

- `DIR` (default `.`, env `GOMDDOC_SERVER_DIR`): directory or Git URL (cloned like `serve`; `--git-key-file` and
  `--git-storage-dir` as on the other content commands).
- Human output: one line per finding (`error  .gomddoc/config.yml:4  config.unknown-key  meta.titel: …`) with the fix
  indented below, then the summary line (`2 errors, 1 warning (3 info hidden, use -v)`).
- `--json`: `doctor.Report`, indented.
- Exit status: `1` if any error; `1` if any warning with `--strict`; `0` otherwise. `info` never fails a run.
- `-v`/`--verbose`: include `info` findings (both human and JSON). No other command has a verbosity flag today; this
  one is the first and sets the convention.

### MCP

`SelfDocs` gains `Doctor func(ctx context.Context, verbose bool) doctor.Report`, injected by `cmd` (it owns config
loading and the Kong model). `gomddoc_doctor {verbose?: bool}` returns the report as JSON, re-running every check on
every call. For a Git source the provider's snapshot is re-checked; the report's `target` says so. Registered only
with `SelfDocs`, i.e. stdio only. `learn_gomddoc` tells agents to call it after editing config or content.

## Finding catalogue

**Config file** (`.gomddoc/config.yml`; `Line` from the YAML node)

| Code | Severity | Condition | Fix |
|------|----------|-----------|-----|
| `config.parse-error` | error | invalid YAML, or a second `---` document | loader's message and line |
| `config.unknown-key` | error | key not in the schema (all of them, not the first); includes a `site:` wrapper | "did you mean `title`?" (edit distance ≤ 2 among sibling keys), else the valid keys at that level |
| `config.wrong-type` | error | e.g. `exclude: drafts/`, a non-bool feature | the expected type, from the schema |
| `config.invalid-value` | error | everything `ValidateAll` rejects | the validator's reason |
| `config.value-replaced` | warning | `Normalize` replaced a value (timeouts, empty theme name) | "using X; set a value within …" |

**Environment**

| Code | Severity | Condition |
|------|----------|-----------|
| `env.invalid-value` | warning | a `GOMDDOC_*` duration/int/bool that does not parse (ignored at runtime) |
| `env.unknown` | warning | a `GOMDDOC_*` variable gomddoc does not read (settings + Kong flag envs + install.sh vars are known) |

**Theme** (template scan: names only)

| Code | Severity | Condition |
|------|----------|-----------|
| `theme.not-installed` | warning | `theme.name` is not installed; default renders |
| `theme.unknown-feature` | warning | a `features` key (config or any page frontmatter) the active theme never reads |
| `theme.unknown-var` | info | a `theme.vars` key the theme's CSS never reads |

**Content** (per language pipeline, each walking with its own `Pipeline.Exclude`)

| Code | Severity | Condition |
|------|----------|-----------|
| `content.frontmatter-invalid` | error | frontmatter YAML does not parse |
| `content.frontmatter-type` | error | a declared field with the wrong type: `tags: foo`, unparseable `date`, string `redirect_from`, non-bool `features` values, non-string `title` |
| `content.redirect-conflict` | error | two pages claim the same `redirect_from` source, or a source is itself a real page (redirects are checked first, so that page is unreachable) |
| `content.path-collision` | warning | resolver extension collision, or a file shadowing a directory |
| `content.tags-collision` | warning | content at `tags.md` or `tags/`, shadowed by the tag routes |
| `content.missing-title` | info | no `title` frontmatter and no heading to derive one from |
| `content.missing-description` | info | no `description` (falls back to the site's) |

**Exclusions**

| Code | Severity | Condition |
|------|----------|-----------|
| `exclude.matches-nothing` | warning | a `site.exclude` pattern that matches no file or directory |

**Target**

| Code | Severity | Condition |
|------|----------|-----------|
| `target.unreachable` | error | directory missing, clone failed; content checks skipped |
| `target.read-error` | error | a file could not be read mid-walk (the error text is the message) |

## Error handling

- Unreachable target: one `target.unreachable` finding, exit 1, well-formed JSON.
- Config that does not parse: its findings, then content checks with default settings; `Report.Assumed` says so.
- Unexpected read failures during the walk: `target.read-error` findings; the run continues.
- `doctor` never panics on user content; a check that fails internally reports `target.read-error` rather than aborting.

## Testing

- **Producers**: each gains a test for both halves — findings returned, and the call site's log output unchanged
  (`logcapture.Has(msg, attrs...)`), per CLAUDE.md's `slog.Warn; continue` rule.
- **`doctor.Run`**: a table over small `fstest.MapFS` sites, one row per finding code, asserting the exact finding
  (code, severity, file, line, key); a clean site yields zero findings (falsifies a check that fires on everything).
- **Config lines**: every config-file finding's `Line` is asserted.
- **Verbose**: `info` findings hidden without it, present with it, `Summary` identical either way.
- **CLI**: exit status for clean / warnings only / warnings + `--strict` / errors / unreachable target; `--json`
  unmarshals into `doctor.Report`.
- **MCP**: `gomddoc_doctor` over stdio reflects an edit made on disk after the server started; absent from
  `siteMCPServer`.
- **Drift**: every finding code emitted anywhere appears in the guide's catalogue table, and every code in the table is
  emitted somewhere (a registry of codes in `diag` or `doctor` makes this checkable).

## Documentation

- New guide page `14-doctor.md`: usage, exit codes, verbose, the catalogue with fixes.
- `02-configuration.md`: `doctor` section beside `info` and `schema`.
- `04-mcp.md`: `gomddoc_doctor` in the self-documentation table.
- `docs/architecture.md`: `diag` findings from producers to `doctor` and `diag.Log`.
- `docs/decisions.md`: producers report (vs. re-implemented checks vs. log scraping); `-v` as the first verbosity flag.
- `docs/roadmap.md`: tick `doctor`; add link/anchor checking as the follow-up.

## Out of scope

- Broken links and anchors (next follow-up).
- Duplicate heading anchors (REVIEW.md §11.5), language-directory detection issues.
- Auto-fixing (`doctor --fix`).
- `gomddoc help <topic>` (sub-spec 3).

## Divergences from implementation

- `target.read-error` is a warning, not an error (`88da0d0`): an unreadable file means a check was incomplete, and
  serve logs it at Warn as it did before.
- `diag` also exports `Catalogue` (every code with its severity and summary), `New` (severity looked up from the
  catalogue; an unknown code panics), `Sort`, `Dedupe` and `WithFilePrefix`. `Dedupe` merges only identical findings,
  or an unlocated finding with a located one on the same code, file and key (`8f4d882`).
- `doctor.Input` has no `Content` field and `PipelineView` carries no findings. `Input` takes `Unreachable error`,
  `Producer []diag.Finding` (all pipelines' findings, language files prefixed with `fr-FR/` and so on) and `Note`.
  `Report` gained a `note` field (for example, that a Git source's snapshot was checked).
- `config.SiteConfig.LoadFromFile` is unchanged. `config.Inspect` reads and decodes `config.yml` into a `yaml.Node`
  itself and exposes it as `Inspection.Node`.
- `resolve.Build` keeps its `*PathResolver` return; findings come from `(*PathResolver).Findings()`.
  `BuildRedirectMap` returns `(URLRedirectMap, []diag.Finding)`.
- Pipelines collect producer findings in `LanguagePipeline.Findings`, and `setupLanguagePipelines` logs them through
  `diag.Log` unless `PipelineOptions.ReportOnly`, which doctor sets.
- A local target that is missing or not a directory is `target.unreachable`. A provider that fails on a config value
  (such as an empty `default_index`) skips only the content checks and says so in the note, so the config findings
  are still reported (`d2a3446`).
- The CLI returns an `exitCodeError` for a non-zero status; `main` exits with it without logging.
- The "did you mean" matcher is `text.Closest` in `internal/text` (`8fb0529`), shared with `gomddoc help`.
