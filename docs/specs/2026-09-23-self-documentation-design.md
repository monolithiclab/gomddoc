# Self-Documentation: Capabilities Model and Embedded Guide (Sub-Spec 1 of 3)

**Status**: Implemented 2026-09-23 (commits `6b912cf` through `c449d06`, with follow-up fixes `f1173ea`, `23aad3b`,
`4f4a79c`, `442424c`, `fddfc40`). Design approved 2026-09-23. All three sub-specs have shipped: sub-spec 2
(`gomddoc doctor`, `docs/specs/2026-09-23-doctor-design.md`, implemented 2026-09-23) and sub-spec 3 (`gomddoc help
<topic>`, commit `37250b5` "feat(cli): add gomddoc help", 2026-09-23). Sub-spec 3 has no spec or plan document. The
theme manifest remains parked.
**Roadmap entry**: "Self-Documentation via MCP" (`docs/roadmap.md`)
**Sub-specs in scope**: machine-readable capabilities model (config schema, env vars, commands, frontmatter, theme
features) and the embedded guide, exposed over stdio MCP and the CLI (`info`, `info --json`, `schema`)
**Sub-specs deferred**: `gomddoc doctor` (sub-spec 2), topic help `gomddoc help <topic>` (sub-spec 3)
**Parked**: theme manifest (`theme.yml`) — see `docs/plans/2026-09-23-theme-manifest-parked.md`

## Goal

gomddoc is self-sufficient for an AI harness: an agent can discover what gomddoc does, how configuration works
(flag vs. env var vs. config file, schema, defaults, bounds), what the active theme supports, and read gomddoc's own
guide — with no external documentation and no network access. Agents are the primary audience, humans second.

Success looks like: an agent given only `gomddoc mcp` (or only the `gomddoc` binary and a shell) can write a valid
`.gomddoc/config.yml` for a stated goal on the first try, and answer "which env var sets X / what does flag Y
override" from gomddoc itself.

## Principles

1. **One source per fact.** Every config fact is declared once, on the config struct field. The JSON Schema, the
   env-var table, `info`, the capabilities report and the MCP resources are projections of it. None hand-rolls its
   own copy.
2. **The report is the answer; transports only render it.** `capabilities.Describe` builds one `Report`. The CLI
   prints it (human or JSON); MCP serves it. `info --json` and `gomddoc://capabilities` are byte-identical.
3. **Self-docs are for the operator's agent, not the site's readers.** The gomddoc namespace is registered on stdio
   MCP only. The HTTP MCP endpoint mounted by `serve` does not advertise its generator's manual.
4. **Best effort is labelled.** Where a fact is inferred rather than declared (theme features), the report says so
   (`"source": "template-scan"`) instead of presenting it as authoritative.

## Components

### 1. `docs` package — embedded guide

`docs/guide.go` declares `package docs` with:

```go
//go:embed guide
var guideFS embed.FS

// Guide is gomddoc's user guide, rooted at docs/guide/.
var Guide fs.FS // = fs.Sub(guideFS, "guide"), panics at init only on a malformed embed
```

The file must live in `docs/` because `go:embed` cannot reference parent directories. The package holds this one
variable and its package comment (ST1000). It stays in the coverage total (unlike `docs/skills/`) — it ships in the
binary.

### 2. `internal/config` — `Schema()` from struct tags

New struct tags on config fields, read by reflection alongside the existing `env:` and `yaml:` tags:

| Tag | Required | Meaning |
|-----|----------|---------|
| `doc:"…"` | every leaf field | One-to-three sentence description, agent-oriented (states effect and interactions) |
| `enum:"a,b,c"` | no | Closed set of allowed values |
| `min:"…"` / `max:"…"` | no | Bounds (durations as Go duration strings, ints as ints); must agree with the existing `Max*` constants |
| `example:"…"` | no | Illustrative value when the default is empty |

The existing `collectEnvVars` walker is generalised into one walker producing a flat `[]Setting`:

```go
type Setting struct {
    Key         string   `json:"key"`          // dotted path from Config root: "site.theme.name", "server.port"
    FileKey     string   `json:"file_key"`     // key inside config.yml ("theme.name"); "" when not file-settable
    Env         string   `json:"env"`          // "GOMDDOC_SITE_THEME_NAME"; "" when no env var
    Flags       []string `json:"flags"`        // CLI flags that set it, filled by the caller (see Wiring)
    Type        string   `json:"type"`         // string | bool | int | duration | []string | map[string]string | map[string]bool
    Default     any      `json:"default"`
    Description string   `json:"description"`
    Enum        []string `json:"enum,omitempty"`
    Min         string   `json:"min,omitempty"`
    Max         string   `json:"max,omitempty"`
    Example     string   `json:"example,omitempty"`
}
```

- `Key` is the dotted path from `Config`. `FileKey` drops the `site.` prefix, because `config.yml` holds
  `SiteConfig` only; it is empty for `server.*` settings. The rule is stated once, in the schema's top-level
  `description` and in the report's `config` block.
- `EnvVars()` becomes a projection of `Schema()` (same names, types and defaults as today; the refactor is guarded by
  a test pinning the current output).
- `JSONSchema() []byte` emits JSON Schema draft 2020-12 for `config.yml`: `type`, `description`, `default`, `enum`,
  `minimum`/`maximum` (durations as `pattern` + description, since JSON Schema has no duration type),
  `additionalProperties: false` on every object. `theme.features` is `{"type": "object", "propertyNames":
  {"pattern": "^[a-z][a-z0-9_]*$"}, "additionalProperties": {"type": "boolean"}}` — no enum while theme facts are
  best effort. `theme.vars` is a free-form string map.
- `Precedence` is exported as data (`[]string{"flag", "env", "file", "default"}`) so the report and the guide cite
  the same order.

`Schema()` does not return an error: it walks static types. Malformed tags are caught by tests (below), not at
runtime.

### 3. `internal/metadata` — declared frontmatter fields

The frontmatter keys with special behaviour are currently scattered (`title`, `description`, `date`, `tags` in
`knownKeys`; `features`, `lang`, `redirect_from`, `robots`, `author` read elsewhere). Add one declared list:

```go
type FrontmatterField struct {
    Key, Type, Description string
}

var FrontmatterFields = []FrontmatterField{ /* title, description, date, tags, author, lang, robots, features, redirect_from */ }
```

`knownKeys` is derived from the subset `metadata` indexes itself. A drift test checks the list against the field
table in `docs/guide/12-advanced/02-markdown-extensions.md`.

### 4. `internal/template` — `ThemeFeatures` (best effort)

```go
type ThemeInfo struct {
    Name     string   `json:"active"`
    Source   string   `json:"source"`   // "template-scan" | "unavailable"
    Features []string `json:"features"` // sorted, from {{ .Feature "x" }} in *.tmpl
    Vars     []string `json:"vars"`     // sorted, from var(--theme-x) in *.css, "--theme-" stripped
}

func ThemeFeatures(assets fs.FS, name string) ThemeInfo
```

Scans the active theme through the same overlay FS the renderer uses, so a site-installed theme is covered. Names
only; descriptions live in `05-theming-and-assets.md`, which the report points to. A theme with no templates found
yields `Source: "unavailable"` and empty lists — not an error. The theme manifest, when designed, replaces this
function's body (see the parked plan).

### 5. `internal/capabilities` — the report

```go
type Input struct {
    Version    string
    Config     *config.Config // nil when no site config could be loaded
    ConfigErr  error          // why it could not, reported as a string
    Commands   []Command      // from Kong's model, converted in cmd/
    Assets     fs.FS          // overlay FS for the theme scan
    Guide      fs.FS
}

func Describe(in Input) Report
```

`Report` (abridged JSON):

```json
{
  "version": "0.1.2",
  "config": {
    "file": ".gomddoc/config.yml",
    "file_keys": "config.yml holds the site.* settings with the site. prefix dropped",
    "precedence": ["flag", "env", "file", "default"],
    "loaded": true,
    "load_error": "",
    "schema_uri": "gomddoc://schema/config"
  },
  "settings": [ /* []config.Setting, flags filled in */ ],
  "commands": [ {"name": "serve", "help": "…", "args": [...], "flags": [{"name": "--port", "env": "…", "default": "…", "help": "…"}]} ],
  "frontmatter": [ /* metadata.FrontmatterFields */ ],
  "theme": { "active": "default", "source": "template-scan", "features": [...], "vars": [...],
             "docs": "gomddoc://guide/05-theming-and-assets.md" },
  "guide": [ {"path": "02-configuration.md", "title": "…", "description": "…"} ]
}
```

`Command` is gomddoc's own struct; `cmd/` converts `*kong.Application` into it so `internal/capabilities` does not
import Kong. Each `Setting.Flags` is filled by joining a Kong flag's `env` tag against `Setting.Env` — the env var
name is the only key both sides already carry, so no new tag is needed. Guide entries come from each page's
frontmatter (`title`, `description`).

`Describe` does not fail. Missing pieces are reported in-band (`config.loaded: false` + `load_error`,
`theme.source: "unavailable"`).

### 6. `internal/mcp` — gomddoc namespace (stdio only)

```go
type SelfDocs struct {
    Report capabilities.Report
    Guide  fs.FS
}

// ServerDeps gains:
SelfDocs *SelfDocs // nil: namespace not registered (the serve HTTP mount)
```

Registered only when `SelfDocs != nil`:

| Kind | Name / URI | Returns |
|------|-----------|---------|
| Resource | `gomddoc://capabilities` | `Report` as JSON |
| Resource | `gomddoc://schema/config` | `config.JSONSchema()` (`application/schema+json`) |
| Resource template | `gomddoc://guide/{+path}` | Guide page as markdown, frontmatter stripped |
| Tool | `gomddoc_capabilities {}` | `Report` as JSON |
| Tool | `gomddoc_guide {query?, path?, section?, limit?}` | See below |
| Prompt | `learn_gomddoc` | Onboarding message (see below) |

`gomddoc_guide` modes, chosen by which arguments are set:

- `query` only → ranked hits `[{path, title, snippet}]` from a search index over the guide (same engine and ranking
  as `search_docs`).
- `path` → the page, markdown, frontmatter stripped, title/description prepended (same shape as `read_page`).
- `path` + `section` → one section via the existing `ExtractSection`.
- nothing → the guide's page list (`path`, `title`, `description`), the same list as `Report.guide`.
- `query` together with `path` → tool error (ambiguous).

An unknown `path` or `section` is a tool error whose text lists the valid paths (or the page's heading IDs), so an
agent corrects itself in one call. Arguments are capped at `maxArgLen`.

The guide's metadata and search indexes are built with the existing `metadata.BuildIndex` and `search.BuildIndex`
over `SelfDocs.Guide`, lazily, behind one `sync.Once` owned by the MCP server. A build failure is returned as the
tool error on every call (it can only mean a broken embed; a test pins that the real embed builds).

`learn_gomddoc` returns one user message containing: the guide README, a compact capabilities summary (commands,
precedence rule, setting count and where to read the schema, active theme features), and instructions — read
`gomddoc://schema/config` before writing `config.yml`, use `gomddoc_guide` for depth, prefer env vars for secrets
and deployment-specific values, files for site identity.

Naming: everything self-describing uses the `gomddoc` prefix or `gomddoc://` scheme; user content stays under
`docs://site/…`. A site page called `capabilities` cannot collide.

### 7. CLI

- **`gomddoc info [DIR] [--json]`** — loads the site config from `DIR` (default `.`) through the normal loader. On
  failure it continues with defaults and reports why. Human output: version, config precedence, config file
  location and load status, settings grouped by section (key, env var, flags, default, description), commands,
  active theme features and vars, guide page list. `--json` prints `Report` verbatim.
- **`gomddoc schema`** — prints `config.JSONSchema()` to stdout.
- **`gomddoc mcp`** — builds the `Report` once at startup (after config load) and passes
  `SelfDocs{Report, docs.Guide}`. `serve`'s HTTP MCP mount passes nil.

Kong's model reaches commands via `ctx.Run(ctx.Model)` in `main.go`; `InfoCmd.Run` and `MCPCmd.Run` take
`*kong.Application` as a bound parameter. The conversion `kong.Application → []capabilities.Command` lives in
`cmd/gomddoc` (one function, shared by both).

## Data flow

```text
config struct tags ──► config.Schema() ──┬──► JSONSchema() ──► `gomddoc schema`, gomddoc://schema/config
                                         └──► EnvVars()
Kong model ──► []Command ──┐
metadata.FrontmatterFields ┤
template.ThemeFeatures ────┼──► capabilities.Describe ──► Report ──┬──► `gomddoc info [--json]`
docs.Guide (frontmatter) ──┘                                        └──► gomddoc://capabilities, gomddoc_capabilities
docs.Guide ──► lazy metadata + search index ──► gomddoc_guide, gomddoc://guide/{+path}, learn_gomddoc
```

## Error handling

| Situation | Behaviour |
|-----------|-----------|
| Leaf config field without `doc:`, malformed `enum`/`min`/`max`, default outside bounds | Test failure (`config` package) |
| `info` / `mcp` cannot load site config | `info`: continue with defaults, `config.loaded: false` + reason. `mcp`: unchanged (already fails fast — it needs content) |
| Theme scan finds no templates | `theme.source: "unavailable"`, empty lists |
| Guide index build fails | `gomddoc_guide` search returns the error; page/section reads and resources still work |
| Unknown guide path / section | Tool error listing valid paths / heading IDs |
| `query` + `path` both set | Tool error |

## Testing

- **config**: table test that every leaf field reachable from `Config` has a non-empty `doc:`; `enum`/`min`/`max`
  parse and the default satisfies them; `min`/`max` agree with the `Max*` constants. `EnvVars()` output pinned
  against the pre-refactor list (exhaustive `slices.Equal`). `JSONSchema()` is valid JSON, and an in-test
  structural walk (no validator dependency) accepts `testsite/.gomddoc/config.yml` and rejects an unknown key at
  root and nested levels.
- **Drift**: every `Setting.FileKey` and `Setting.Env` appears in `docs/guide/02-configuration.md`; every
  `FrontmatterField.Key` appears in the markdown-extensions field table; every guide page has `title` and
  `description` frontmatter.
- **template**: `ThemeFeatures` on the embedded default theme returns exactly the 11 features and 6 vars
  (exhaustive). An overlay adding a feature in a site-level template is picked up. Missing theme →
  `"unavailable"`.
- **capabilities**: `Describe` with a nil config reports `loaded: false` and the error text; flags join onto
  settings by env var (a setting with a flag and one without, both asserted).
- **mcp**: namespace present iff `SelfDocs != nil` (list tools/resources/prompts both ways). Each `gomddoc_guide`
  mode against an `fstest.MapFS` guide, results unmarshalled and compared exhaustively; unknown path error names
  valid paths; `query`+`path` error. Cold-cache `fanout.Run(50, …)` on the guide index asserting pointer identity.
  One test against the real `docs.Guide` confirming the index builds and a known query hits
  `02-configuration.md`.
- **cmd**: `info --json` unmarshals into `capabilities.Report` and equals the `gomddoc://capabilities` resource
  byte-for-byte for the same inputs; `schema` output equals the resource; `info` in a directory with a broken
  config exits 0 and reports the error.

## Documentation

- `docs/guide/04-mcp.md`: "Self-documentation" section (namespace, tools, prompt, stdio-only rationale).
- `docs/guide/02-configuration.md`: pointer to `gomddoc schema` and `gomddoc info --json`; `info` section updated.
- `docs/architecture.md`: capabilities layer and the one-source-of-facts flow.
- `docs/decisions.md`: struct tags as the fact source (vs. hand table / `go generate`); stdio-only namespace;
  template scan as the interim for theme facts.
- `docs/roadmap.md`: tick the self-documentation items; link the parked theme manifest plan.
- gomddoc-website `docs/configuration.md`: pointer to `gomddoc schema` / `info --json`.

## Out of scope

- `gomddoc doctor` (sub-spec 2) — will consume `Schema()`, `JSONSchema()` and `ThemeFeatures`.
- `gomddoc help <topic>` (sub-spec 3) — will render `docs.Guide` pages in the terminal.
- Theme manifest — parked.
- A hosted schema URL and a `yaml-language-server` hint in `gomddoc init`.
- Self-docs on the HTTP MCP endpoint.

## Divergences from implementation

- Struct tags: only `doc`, `max` and `default_doc` shipped. `enum`, `min` and `example` were dropped, and `Setting`
  has no `Enum`, `Min` or `Example` fields. It gained `DefaultNote` (`default_note`) for computed defaults, such as the
  title derived from the content directory.
- `config.EnvVars()`, `EnvVar` and `collectEnvVars` were deleted, not kept as a projection of `Schema()` (`c891f4a`).
  Map-valued settings spell their env var with a key placeholder (`GOMDDOC_SITE_THEME_FEATURES_<KEY>`).
- Flags are joined onto settings by a Kong `setting:"<key>"` tag first, then by env var name. `--domain`
  (`GOMDDOC_DOMAIN`) and preview's `--dir-index` (`GOMDDOC_DIR_INDEX`) set settings whose own env vars differ.
  `Setting.Flags` also lists positional args that set a setting (uppercased, such as `DIR`).
- `metadata.knownKeys` is not derived from `FrontmatterFields`; a test asserts it is a subset. The declared list also
  carries `og_type` and `layout`.
- `ThemeFeatures` scans the templates the renderer resolves (theme, inherited default partials, site partials,
  default-layout fallback), not the theme directory alone (`f1173ea`). A theme that is not installed is reported as
  the default theme with `source: "fallback-default"`; `unavailable` applies only when not even the default theme
  has layouts.
- `capabilities.Input` also takes `Dir` and `FileStatus`. `Report.config` carries `dir` and `file_status`
  (`found`, `not_found`, `not_inspected`, `unknown`; `442424c`), and `Report` has a `guide_error` field. Guide
  entries are `guide.Topic` values with a `topic` name alongside `path`, `title` and `description`.
- The guide is served through `internal/guide` (`2b05595`), not an MCP-owned index: `guide.New` builds the topic list,
  the search index is built lazily behind `sync.OnceValues`, and a page is reachable by path or by topic name
  (`configuration`). Section extraction moved to `internal/text` (`e6f0ce5`). `gomddoc help` uses the same package.
- `SelfDocs` gained a `Doctor` function; when set, the stdio namespace also registers the `gomddoc_doctor` tool
  (sub-spec 2).
- `gomddoc_guide` errors are `IsError: true` tool results.
- `info`'s `DIR` argument reads `GOMDDOC_SERVER_DIR`. A Git URL is not cloned; `info` reports defaults.
