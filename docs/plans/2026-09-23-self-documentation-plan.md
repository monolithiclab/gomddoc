# Self-Documentation (Sub-Spec 1 of 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status**: Implemented 2026-09-23. All 14 tasks shipped: `6b912cf` (1), `c891f4a` (2), `480d7a4` (3), `1dcc358` (4),
`06b4ee9` (5), `3224034` (6), `5e1c3f1` (7), `e41fa6a` (8), `c80885e` (9), `85f6a2e` (10), `ea4fcde` and `1c10b21`
(11), `e4e9ee7` (12), `228ece9` (13), `c449d06` (14). Later refactors changed parts of Tasks 6 and 11 (see
Divergences).

**Goal:** gomddoc describes itself — config schema, env vars, flags, commands, frontmatter, theme features and its
own embedded guide — over stdio MCP and the CLI (`info`, `info --json`, `schema`), from one source per fact.

**Architecture:** Config facts move onto struct tags and are read by one reflection walker (`config.Schema()`), from
which the JSON Schema is projected. `internal/capabilities` assembles a single `Report` from the schema, Kong's
command model, a declared frontmatter list, a best-effort theme template scan and the embedded guide. The CLI prints
the report; `internal/mcp` serves it plus the guide under a `gomddoc://` namespace registered only for stdio.

**Tech Stack:** Go 1.26, `embed`, `reflect`, `encoding/json`, Kong v1.15 (`kong.Application` model, `Tag.Get`),
`github.com/modelcontextprotocol/go-sdk/mcp`, existing `metadata`/`search` indexes. No new dependencies.

**Spec:** `docs/specs/2026-09-23-self-documentation-design.md`

## Global Constraints

- No new module dependencies (no JSON Schema validator library — tests validate structurally).
- `path`, not `filepath`, for every `fs.FS` operation; `filepath` only for OS paths.
- Every new package has a package comment stating its job and the contract a caller gets wrong (ST1000 is on).
- Never name a local after a builtin or imported package (`gocritic builtinShadow,importShadow` gates it).
- Table-driven tests with `t.Parallel()`; exhaustive comparisons (`slices.Equal`, `maps.Equal`) over `Contains`.
- MCP self-docs register **only** when `ServerDeps.SelfDocs != nil`; `serve`'s HTTP mount passes nil.
- User-content URIs stay `docs://site/…`; everything self-describing is `gomddoc://…` or `gomddoc_…`.
- `info --json` and `gomddoc://capabilities` are both `Report.JSON()` — no second serialization path.
- Validate every task with `make ci`. Do not `git commit`: at the end of each task, print a draft commit message
  (concise, conventional prefix, ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`) and stop
  for the developer to commit.

## Deviations from the spec (decided while planning)

- **`enum` and `min` tags are dropped** (YAGNI): no file-settable field has a closed value set, and the only bounds
  in the codebase are the `Max*` constants. Only `doc`, `max` and `default_doc` are added.
- **`config.EnvVars()` / `EnvVar` / `collectEnvVars` are deleted**, not kept as a projection: their only caller is
  `info`, which is rewritten to print the report. A projection with no production caller is dead code.
- **Map-valued env vars are spelled with their key placeholder**: `GOMDDOC_SITE_THEME_FEATURES_<KEY>`, because
  that is the variable an agent actually sets.
- **Flags that set a config value under a different env var name** (`--domain` → `GOMDDOC_DOMAIN` sets
  `site.meta.domain`; preview's `--dir-index` → `GOMDDOC_DIR_INDEX` sets `site.dir_index`) carry a Kong
  `setting:"<key>"` tag. Joining on env var name alone misses them. Kong exposes arbitrary struct tags through
  `Tag.Get`.
- **`metadata.knownKeys` is not derived** from `FrontmatterFields`; a test asserts it is a subset instead.
- **`gomddoc_guide` failures return `IsError: true`** tool results (the existing six tools return plain text).
  The spec calls them tool errors; `IsError` is how MCP tells an agent to correct its call.

## Review Focus

- `gomddoc info` run outside any site, in a directory with a broken `config.yml`, and with a Git URL: must exit 0
  and say why the config was not loaded (Task 9 tests all three).
- `gomddoc_guide` with a path that is valid `fs` syntax but not a guide page (`../go.mod`, `/02-configuration.md`,
  `12-advanced`): must be an error listing valid pages, never a read (Task 11 tests these rows).
- A theme overridden at site level (`.gomddoc/assets/themes/default/partials/x.html.tmpl` adding a feature): the
  scan must see the override (Task 6 tests it).
- Guide text that names an env var that does not exist (the guide today says `GOMDDOC_SITE_FEATURES_KATEX`, which
  is not a variable): the drift test must fail on it (Task 5 fixes the existing one and pins the rule).
- A cold `gomddoc mcp` hit by concurrent `gomddoc_guide` searches: exactly one guide index build (Task 11 fanout
  test with pointer identity).

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `docs/guide.go` | Create | `package docs`: `Guide fs.FS` embedding `docs/guide/` |
| `docs/guide_test.go` | Create | Every guide page has `title` and `description` frontmatter |
| `internal/config/schema.go` | Create | `Setting`, `Precedence`, `FileKeysNote`, `Schema()`, `JSONSchema()` |
| `internal/config/schema_test.go` | Create | Doc-tag coverage, pinned keys/envs, bounds, structural schema validation |
| `internal/config/config.go` | Modify | `doc`/`max`/`default_doc` tags on every leaf; delete `EnvVar`, `EnvVars`, `collectEnvVars` |
| `internal/config/config_test.go` | Modify | Delete `TestEnvVars` (superseded by `schema_test.go`) |
| `internal/metadata/frontmatter_fields.go` | Create | `FrontmatterField`, `FrontmatterFields` |
| `internal/metadata/frontmatter_fields_test.go` | Create | Drift vs. guide table; `knownKeys` ⊆ declared list |
| `internal/template/themefeatures.go` | Create | `ThemeInfo`, `ThemeFeatures(assets, name)` |
| `internal/template/themefeatures_test.go` | Create | Default theme exhaustive, overlay pickup, missing theme |
| `internal/capabilities/capabilities.go` | Create | `Input`, `Report`, `Command`, `Flag`, `Arg`, `GuidePage`, `Describe`, `Report.JSON` |
| `internal/capabilities/capabilities_test.go` | Create | Flag joining, nil config, guide list, JSON round trip |
| `cmd/gomddoc/main.go` | Modify | `parserOptions()`, `ctx.Run(ctx.Model)`, `Schema` command |
| `cmd/gomddoc/capabilities.go` | Create | `commandsFromKong`, `capabilitiesInput` |
| `cmd/gomddoc/capabilities_test.go` | Create | Every `setting:` tag resolves; `--domain`/`--port` joins |
| `cmd/gomddoc/guide_drift_test.go` | Create | Guide ↔ settings/flags env-var and file-key drift, both directions |
| `cmd/gomddoc/info.go` | Rewrite | `InfoCmd{Dir, JSON}` printing the report |
| `cmd/gomddoc/info_test.go` | Create | Human/JSON output, broken config, Git URL, missing dir |
| `cmd/gomddoc/schema.go` | Create | `SchemaCmd` |
| `cmd/gomddoc/schema_test.go` | Create | Output equals `config.JSONSchema()` |
| `cmd/gomddoc/serve_test.go` | Modify | Delete the `writeEnvVarsHelp` test |
| `cmd/gomddoc/build.go`, `serve.go`, `preview.go` | Modify | `setting:` tags on `Domain` / `DirIndex` |
| `cmd/gomddoc/mcp.go` | Modify | Build report, pass `SelfDocs` |
| `cmd/gomddoc/mcp_test.go` | Modify | Stdio server lists the gomddoc namespace |
| `internal/mcp/server.go` | Modify | `SelfDocs`, `ServerDeps.SelfDocs`, lazy guide index, registration |
| `internal/mcp/selfdocs.go` | Create | Resources, `gomddoc_capabilities`, `gomddoc_guide`, `learn_gomddoc` |
| `internal/mcp/selfdocs_test.go` | Create | Namespace on/off, each guide mode, errors, fanout |
| `internal/mcp/section.go` | Modify | `HeadingIDs(content)` for the unknown-section error |
| `internal/mcp/tools.go` | Modify | Extract `pageHeader` from `buildPageHeader` for reuse |
| `docs/guide/02-configuration.md`, `04-mcp.md`, `12-advanced/02-markdown-extensions.md` | Modify | Drift fixes, new sections |
| `docs/architecture.md`, `docs/decisions.md`, `docs/roadmap.md`, `REVIEW.md` | Modify | Docs workflow |
| `../gomddoc-website/docs/configuration.md` | Modify | Pointer to `schema` / `info --json` |

---

### Task 1: Embed the guide (`docs` package)

**Files:**
- Create: `docs/guide.go`
- Test: `docs/guide_test.go`

**Interfaces:**
- Produces: `docs.Guide fs.FS` — rooted at `docs/guide/`, so paths are `README.md`, `02-configuration.md`,
  `12-advanced/02-markdown-extensions.md`.

- [x] **Step 1: Write the failing test**

```go
package docs

import (
	"io/fs"
	"path"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/monolithiclab/gomddoc/internal/text"
)

// TestGuide_EveryPageHasTitleAndDescription pins what the capabilities report
// and gomddoc_guide list: a page without them shows up as a bare path.
func TestGuide_EveryPageHasTitleAndDescription(t *testing.T) {
	t.Parallel()
	var pages int
	err := fs.WalkDir(Guide, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".md" {
			return err
		}
		pages++
		data, err := fs.ReadFile(Guide, p)
		if err != nil {
			return err
		}
		var fm struct{ Title, Description string }
		if err := yaml.Unmarshal(text.Frontmatter(data), &fm); err != nil {
			t.Errorf("%s: frontmatter: %v", p, err)
		}
		if fm.Title == "" || fm.Description == "" {
			t.Errorf("%s: title=%q description=%q, want both non-empty", p, fm.Title, fm.Description)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if pages < 15 { // 13 top-level pages + 12-advanced/ — catches an embed pattern that drops the subdirectory
		t.Errorf("embedded %d pages, want at least 15", pages)
	}
}

func TestGuide_RootedAtGuideDir(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"README.md", "02-configuration.md", "12-advanced/02-markdown-extensions.md"} {
		if _, err := fs.Stat(Guide, p); err != nil {
			t.Errorf("fs.Stat(Guide, %q): %v", p, err)
		}
	}
}
```

If `internal/text` has no `Frontmatter(data) []byte` extractor (it has `StripFrontmatter`), add one next to
`StripFrontmatter` in `internal/text/frontmatter.go` with a table test in `frontmatter_test.go` covering: no
frontmatter → nil; `---\na: 1\n---\nbody` → `a: 1\n`; unterminated → nil. Grep first
(`grep -rn "func Frontmatter\|func ExtractFrontmatter" internal/`) — reuse an existing one if present.

- [x] **Step 2: Run to verify it fails**

Run: `go test ./docs/ -run TestGuide -v`
Expected: FAIL — `undefined: Guide`

- [x] **Step 3: Implement**

```go
// Package docs embeds gomddoc's user guide so the binary can serve its own
// documentation (gomddoc info, the gomddoc:// MCP namespace) with no network
// access and no checkout.
//
// It lives in docs/ only because go:embed cannot reach a parent directory.
// Guide is rooted at docs/guide/ — paths are "02-configuration.md", not
// "guide/02-configuration.md" — and it is the only thing in this package: the
// Go code under docs/skills/ is tooling and is excluded from coverage and
// vulncheck by path prefix, which this package deliberately is not.
package docs

import (
	"embed"
	"io/fs"
)

//go:embed guide
var guideFS embed.FS

// Guide is gomddoc's user guide, rooted at docs/guide/.
var Guide = mustSub(guideFS, "guide")

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic("docs: " + err.Error()) // only reachable with a malformed embed directive
	}
	return sub
}
```

- [x] **Step 4: Run to verify it passes**

Run: `go test ./docs/ -v`
Expected: PASS. If a guide page lacks `title`/`description`, add them to that page (it is a real gap).

- [x] **Step 5: `make ci`, then draft the commit message** (`feat(docs): embed the user guide in the binary`).

---

### Task 2: `config.Schema()` from struct tags

**Files:**
- Create: `internal/config/schema.go`, `internal/config/schema_test.go`
- Modify: `internal/config/config.go` (tags; delete `EnvVar`, `EnvVars`, `collectEnvVars`),
  `internal/config/config_test.go` (delete `TestEnvVars`)
- Modify: `cmd/gomddoc/info.go` — temporarily replace `writeEnvVarsHelp`'s body to range over `config.Schema()`
  (print `s.Env`, `s.Type`, default) so the tree compiles; Task 9 rewrites the file. Update
  `cmd/gomddoc/serve_test.go`'s `writeEnvVarsHelp` test only as far as needed to compile (Task 9 deletes it).

**Interfaces:**
- Produces:

```go
type Setting struct {
	Key         string   `json:"key"`                    // "site.theme.name", "server.http.write_timeout"
	FileKey     string   `json:"file_key,omitempty"`     // "theme.name"; "" for server.*
	Env         string   `json:"env,omitempty"`          // "GOMDDOC_SITE_THEME_NAME"; "" when none
	Flags       []string `json:"flags,omitempty"`        // filled by capabilities.Describe
	Type        string   `json:"type"`                   // string|bool|int|duration|[]string|map[string]string|map[string]bool
	Default     any      `json:"default"`                // durations as strings ("30s")
	DefaultNote string   `json:"default_note,omitempty"` // for computed defaults; Default is nil then
	Description string   `json:"description"`
	Max         string   `json:"max,omitempty"`
}
var Precedence = []string{"flag", "env", "file", "default"}
const FileKeysNote = "…"
func Schema() []Setting // fresh slice every call; callers may mutate it
```

- [x] **Step 1: Write the failing tests** (`schema_test.go`)

```go
package config

import (
	"slices"
	"strconv"
	"testing"
	"time"
)

func TestSchema_Keys(t *testing.T) {
	t.Parallel()
	var keys, envs []string
	for _, s := range Schema() {
		keys = append(keys, s.Key)
		envs = append(envs, s.Env)
	}
	wantKeys := []string{
		"server.port", "server.admin_port", "server.dev_mode", "server.dir", "server.pprof",
		"server.http.shutdown_timeout", "server.http.read_header_timeout", "server.http.write_timeout",
		"server.http.idle_timeout", "server.http.max_header_mb",
		"site.default_index", "site.dir_index", "site.edit_url", "site.language",
		"site.meta.title", "site.meta.description", "site.meta.domain", "site.meta.robots",
		"site.theme.name", "site.theme.vars", "site.theme.features",
		"site.highlighting.theme", "site.search.index", "site.exclude", "site.strip_extensions",
	}
	wantEnvs := []string{
		"GOMDDOC_SERVER_PORT", "GOMDDOC_SERVER_ADMIN_PORT", "GOMDDOC_SERVER_DEV_MODE", "GOMDDOC_SERVER_DIR",
		"GOMDDOC_SERVER_PPROF", "GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT", "GOMDDOC_SERVER_HTTP_READ_HEADER_TIMEOUT",
		"GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT", "GOMDDOC_SERVER_HTTP_IDLE_TIMEOUT", "GOMDDOC_SERVER_HTTP_MAX_HEADER_MB",
		"GOMDDOC_SITE_DEFAULT_INDEX", "GOMDDOC_SITE_DIR_INDEX", "GOMDDOC_SITE_EDIT_URL", "GOMDDOC_SITE_LANGUAGE",
		"GOMDDOC_SITE_META_TITLE", "GOMDDOC_SITE_META_DESCRIPTION", "GOMDDOC_SITE_META_DOMAIN",
		"GOMDDOC_SITE_META_ROBOTS", "GOMDDOC_SITE_THEME_NAME", "", "GOMDDOC_SITE_THEME_FEATURES_<KEY>",
		"GOMDDOC_SITE_HIGHLIGHTING_THEME", "GOMDDOC_SITE_SEARCH_INDEX", "", "",
	}
	if !slices.Equal(keys, wantKeys) {
		t.Errorf("keys:\n got %q\nwant %q", keys, wantKeys)
	}
	if !slices.Equal(envs, wantEnvs) {
		t.Errorf("envs:\n got %q\nwant %q", envs, wantEnvs)
	}
}

func TestSchema_FileKeys(t *testing.T) {
	t.Parallel()
	for _, s := range Schema() {
		want := ""
		if s.Key[:5] == "site." {
			want = s.Key[5:]
		}
		if s.FileKey != want {
			t.Errorf("%s: FileKey = %q, want %q", s.Key, s.FileKey, want)
		}
	}
}

// TestSchema_EveryLeafDocumented is the gate that makes the doc tag mandatory:
// a new config field without one fails here, not in an agent's hands.
func TestSchema_EveryLeafDocumented(t *testing.T) {
	t.Parallel()
	for _, s := range Schema() {
		if len(s.Description) < 20 {
			t.Errorf("%s: doc tag %q is missing or too short to be useful", s.Key, s.Description)
		}
		if s.Type == "" {
			t.Errorf("%s: empty type", s.Key)
		}
	}
}

func TestSchema_Defaults(t *testing.T) {
	t.Parallel()
	byKey := map[string]Setting{}
	for _, s := range Schema() {
		byKey[s.Key] = s
	}
	tests := []struct {
		key  string
		want any
	}{
		{"server.port", DefaultPort},
		{"server.http.write_timeout", DefaultWriteTimeout.String()},
		{"server.http.max_header_mb", DefaultMaxHeaderMB},
		{"site.theme.name", DefaultThemeName},
		{"site.search.index", true},
	}
	for _, tt := range tests {
		if got := byKey[tt.key].Default; got != tt.want {
			t.Errorf("%s default = %#v, want %#v", tt.key, got, tt.want)
		}
	}
	// Computed default: documented by note, never a machine-specific value.
	title := byKey["site.meta.title"]
	if title.Default != nil || title.DefaultNote == "" {
		t.Errorf("site.meta.title: Default=%#v DefaultNote=%q, want nil and a note", title.Default, title.DefaultNote)
	}
	if got := byKey["site.strip_extensions"].Default; !slices.Equal(got.([]string), []string{".md"}) {
		t.Errorf("site.strip_extensions default = %#v", got)
	}
}

// TestSchema_MaxMatchesConstants: the max tag is documentation of a bound that
// normalizeHTTP enforces through the Max* constants. They must agree.
func TestSchema_MaxMatchesConstants(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"server.http.read_header_timeout": MaxReadHeaderTimeout.String(),
		"server.http.write_timeout":       MaxWriteTimeout.String(),
		"server.http.idle_timeout":        MaxIdleTimeout.String(),
		"server.http.max_header_mb":       strconv.Itoa(MaxMaxHeaderMB),
	}
	for _, s := range Schema() {
		w, bounded := want[s.Key]
		if !bounded {
			if s.Max != "" {
				t.Errorf("%s: unexpected max %q", s.Key, s.Max)
			}
			continue
		}
		got := s.Max
		if s.Type == "duration" {
			d, err := time.ParseDuration(s.Max)
			if err != nil {
				t.Fatalf("%s: max %q: %v", s.Key, s.Max, err)
			}
			got = d.String()
		}
		if got != w {
			t.Errorf("%s: max = %q, want %q", s.Key, got, w)
		}
	}
}

func TestSchema_ReturnsFreshSlice(t *testing.T) {
	t.Parallel()
	a := Schema()
	a[0].Flags = append(a[0].Flags, "--mutated")
	if b := Schema(); len(b[0].Flags) != 0 {
		t.Error("Schema() shares state between calls")
	}
}
```

- [x] **Step 2: Run to verify failure**

Run: `go test ./internal/config/ -run TestSchema -v`
Expected: FAIL — `undefined: Schema`

- [x] **Step 3: Implement `schema.go`**

```go
package config

import (
	"reflect"
	"strings"
	"time"
)

// Setting describes one configuration value. Schema derives every Setting from
// the config structs' tags (env, yaml, doc, max, default_doc), which makes those
// tags the single source for everything that describes configuration: the JSON
// Schema, `gomddoc info` and the MCP capabilities report. Document a field on
// the field.
type Setting struct { /* as in Interfaces */ }

// Precedence is the order in which configuration sources override each other,
// highest first. server.* settings have no file source.
var Precedence = []string{"flag", "env", "file", "default"}

// FileKeysNote states how Setting.Key maps onto config.yml. It is quoted by the
// JSON Schema and the capabilities report so both say it the same way.
const FileKeysNote = "config.yml holds the site.* settings with the `site.` prefix dropped " +
	"(site.theme.name is `theme: {name: …}`); server.* settings are set by flag or environment variable only."

// Schema returns every leaf configuration setting in struct order. It builds a
// fresh slice on each call, so callers may fill Flags without copying.
func Schema() []Setting {
	var out []Setting
	walkSchema(reflect.ValueOf(New()).Elem(), "", "GOMDDOC", &out)
	return out
}

func walkSchema(v reflect.Value, keyPrefix, envPrefix string, out *[]Setting) {
	t := v.Type()
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		key := settingName(sf)
		if keyPrefix != "" {
			key = keyPrefix + "." + key
		}
		env := ""
		if tag := sf.Tag.Get("env"); tag != "" {
			env = envPrefix + "_" + tag
		}
		if sf.Type.Kind() == reflect.Struct {
			childPrefix := env
			if childPrefix == "" {
				childPrefix = envPrefix
			}
			walkSchema(v.Field(i), key, childPrefix, out)
			continue
		}
		*out = append(*out, newSetting(sf, v.Field(i), key, env))
	}
}

// settingName is the yaml key when the field has one, else its env segment
// lowercased: server.* fields are not file-settable and carry no yaml tag.
func settingName(sf reflect.StructField) string {
	if name, _, _ := strings.Cut(sf.Tag.Get("yaml"), ","); name != "" {
		return name
	}
	return strings.ToLower(sf.Tag.Get("env"))
}

func newSetting(sf reflect.StructField, fv reflect.Value, key, env string) Setting {
	s := Setting{
		Key:         key,
		Env:         env,
		Type:        settingType(sf.Type),
		Default:     settingDefault(fv),
		DefaultNote: sf.Tag.Get("default_doc"),
		Description: sf.Tag.Get("doc"),
		Max:         sf.Tag.Get("max"),
	}
	if rest, ok := strings.CutPrefix(key, "site."); ok {
		s.FileKey = rest
	}
	// Map-valued env overrides are read per key (walkStruct scans for the
	// prefix), so the variable an agent sets is the prefix plus the key.
	if env != "" && sf.Type.Kind() == reflect.Map {
		s.Env = env + "_<KEY>"
	}
	if s.DefaultNote != "" {
		s.Default = nil
	}
	return s
}

func settingType(t reflect.Type) string {
	if t == reflect.TypeFor[time.Duration]() {
		return "duration"
	}
	return t.String()
}

func settingDefault(fv reflect.Value) any {
	if d, ok := fv.Interface().(time.Duration); ok {
		return d.String()
	}
	if fv.Kind() == reflect.Map && fv.IsNil() {
		return nil
	}
	return fv.Interface()
}
```

- [x] **Step 4: Add tags to every leaf field in `config.go`**

Use these exact descriptions (agent-oriented: effect, then interactions). Add `max` where shown and
`default_doc` on `Meta.Title`.

| Field | Tags to add |
|-------|-------------|
| `ServerConfig.Port` | `doc:"HTTP listen address (host:port or :port). ':auto' picks the first free port from 8080."` |
| `ServerConfig.AdminPort` | `doc:"Listen address for /healthz, /metrics and pprof. Empty or equal to port serves them on the main listener; a bare port binds 127.0.0.1."` |
| `ServerConfig.DevMode` | `doc:"Disables template caching and asset immutability so edits show on reload. Always on for preview."` |
| `ServerConfig.Dir` | `doc:"Content root: a local directory or a git://, git+ssh:// or git+https:// URL."` |
| `ServerConfig.Pprof` | `doc:"Exposes /debug/pprof/ on the admin listener."` |
| `HTTPConfig.ShutdownTimeout` | `doc:"Grace period for in-flight requests on SIGINT/SIGTERM."` |
| `HTTPConfig.ReadHeaderTimeout` | `doc:"Maximum time to read request headers; out-of-range values fall back to the default." max:"1m"` |
| `HTTPConfig.WriteTimeout` | `doc:"Maximum time to write a response; out-of-range values fall back to the default." max:"5m"` |
| `HTTPConfig.IdleTimeout` | `doc:"Keep-alive idle timeout; out-of-range values fall back to the default." max:"10m"` |
| `HTTPConfig.MaxHeaderMB` | `doc:"Maximum request header size in MiB; out-of-range values fall back to the default." max:"10"` |
| `SiteConfig.DefaultIndex` | `doc:"File served for a directory URL (README.md serves /docs/ from docs/README.md). Must not be empty."` |
| `SiteConfig.DirIndex` | `doc:"Renders an auto-generated listing for directories that have no default_index file."` |
| `SiteConfig.EditURL` | `doc:"Base URL for 'Edit this page' links; the page's path is appended. http or https only."` |
| `SiteConfig.Language` | `doc:"BCP 47 language of the default content tree (<html lang>, UI strings). Frontmatter lang overrides it per page."` |
| `MetaConfig.Title` | `doc:"Site title used in <title>, Open Graph tags and feeds." default_doc:"the content directory's name, title-cased"` |
| `MetaConfig.Description` | `doc:"Site description; the fallback <meta name=description> for pages without one."` |
| `MetaConfig.Domain` | `doc:"Bare host (docs.example.com, no scheme or path). Enables canonical URLs, sitemap.xml, feed.xml and the robots.txt Sitemap line."` |
| `MetaConfig.Robots` | `doc:"Site-wide <meta name=robots> value, e.g. noindex. Frontmatter robots overrides it per page."` |
| `ThemeConfig.Name` | `doc:"Theme directory under assets/themes/. Only 'default' is bundled; others must be installed in .gomddoc/assets/themes/<name>/ or rendering falls back to default."` |
| `ThemeConfig.Vars` | `doc:"Free-form key/value map emitted as --theme-<key> CSS custom properties. Which keys a theme reads is theme-specific; see the capabilities report's theme.vars."` |
| `ThemeConfig.Features` | `doc:"Feature toggles keyed by name (all default to true). Keys must match ^[a-z][a-z0-9_]*$; which keys a theme honours is listed in the capabilities report's theme.features. Frontmatter features override per page."` |
| `HighlightConfig.Theme` | `doc:"Chroma style name for fenced code blocks, e.g. github, monokai, dracula."` |
| `SearchConfig.Index` | `doc:"Builds the full-text search index at startup; disabling it removes /api/search and the search UI's backend."` |
| `SiteConfig.Exclude` | `doc:"Glob patterns (content-root relative) hidden from every route, index, navigation and build output. A trailing / matches a directory."` |
| `SiteConfig.StripExtensions` | `doc:"Extensions removed from served URLs (/guide instead of /guide.md); the extension-ful URL 301-redirects. Each must start with a dot."` |

Before copying any description, check it against the code it describes (e.g. `normalizeHTTP` for the "fall back to
the default" claim, `IsGitURL` for the scheme list, `generateThemeVarsCSS` for `--theme-<key>`) and correct the
wording if the code disagrees — the doc tag is now the documentation.

Then delete `EnvVar`, `EnvVars` and `collectEnvVars` from `config.go` and `TestEnvVars` from `config_test.go`.

- [x] **Step 5: Run to verify it passes**

Run: `go test ./internal/config/ ./cmd/gomddoc/ -v -run 'TestSchema|Env'`
Expected: PASS

- [x] **Step 6: `make ci`, then draft the commit message** (`feat(config): derive a settings schema from struct
  tags`).

---

### Task 3: `config.JSONSchema()`

**Files:**
- Modify: `internal/config/schema.go`
- Test: `internal/config/schema_test.go`

**Interfaces:**
- Consumes: `Schema()`, `FileKeysNote`, `featureKeyPattern` (in `features.go`).
- Produces: `func JSONSchema() []byte` — indented JSON, deterministic, a fresh copy per call.

- [x] **Step 1: Write the failing tests**

```go
// validateAgainst is a structural validator for the subset of JSON Schema that
// JSONSchema emits (type, properties, additionalProperties, items,
// propertyNames.pattern). It exists so the test needs no validator dependency.
func validateAgainst(schema map[string]any, doc any, at string) []string {
	var errs []string
	switch schema["type"] {
	case "object":
		obj, ok := doc.(map[string]any)
		if !ok {
			return []string{at + ": want object"}
		}
		props, _ := schema["properties"].(map[string]any)
		for k, v := range obj {
			child := at + "." + k
			if pat, ok := schema["propertyNames"].(map[string]any); ok {
				if !regexp.MustCompile(pat["pattern"].(string)).MatchString(k) {
					errs = append(errs, child+": key does not match propertyNames")
				}
			}
			if sub, ok := props[k].(map[string]any); ok {
				errs = append(errs, validateAgainst(sub, v, child)...)
				continue
			}
			switch ap := schema["additionalProperties"].(type) {
			case bool:
				if !ap {
					errs = append(errs, child+": unknown key")
				}
			case map[string]any:
				errs = append(errs, validateAgainst(ap, v, child)...)
			}
		}
	case "array":
		arr, ok := doc.([]any)
		if !ok {
			return []string{at + ": want array"}
		}
		for i, v := range arr {
			errs = append(errs, validateAgainst(schema["items"].(map[string]any), v, fmt.Sprintf("%s[%d]", at, i))...)
		}
	case "string":
		if _, ok := doc.(string); !ok {
			errs = append(errs, at+": want string")
		}
	case "boolean":
		if _, ok := doc.(bool); !ok {
			errs = append(errs, at+": want boolean")
		}
	}
	return errs
}

func loadSchema(t *testing.T) map[string]any {
	t.Helper()
	var s map[string]any
	if err := json.Unmarshal(JSONSchema(), &s); err != nil {
		t.Fatalf("JSONSchema is not valid JSON: %v", err)
	}
	return s
}

func TestJSONSchema_Header(t *testing.T) {
	t.Parallel()
	s := loadSchema(t)
	if s["$schema"] != "https://json-schema.org/draft/2020-12/schema" || s["$id"] != "gomddoc://schema/config" {
		t.Errorf("header: $schema=%v $id=%v", s["$schema"], s["$id"])
	}
	if s["additionalProperties"] != false {
		t.Error("root must reject unknown keys")
	}
	if !strings.Contains(s["description"].(string), FileKeysNote) {
		t.Error("root description must carry FileKeysNote")
	}
}

// TestJSONSchema_CoversEveryFileKey: every file-settable setting is reachable
// in the schema, and nothing else is.
func TestJSONSchema_CoversEveryFileKey(t *testing.T) {
	t.Parallel()
	s := loadSchema(t)
	var got []string
	var walk func(node map[string]any, prefix string)
	walk = func(node map[string]any, prefix string) {
		props, _ := node["properties"].(map[string]any)
		for k, v := range props {
			child := v.(map[string]any)
			if _, nested := child["properties"]; nested {
				walk(child, prefix+k+".")
				continue
			}
			got = append(got, prefix+k)
		}
	}
	walk(s, "")
	var want []string
	for _, st := range Schema() {
		if st.FileKey != "" {
			want = append(want, st.FileKey)
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("schema leaves:\n got %q\nwant %q", got, want)
	}
}

func TestJSONSchema_Validates(t *testing.T) {
	t.Parallel()
	s := loadSchema(t)
	testsite, err := os.ReadFile("../../testsite/.gomddoc/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		yaml    string
		wantErr string // "" = valid
	}{
		{"testsite", string(testsite), ""},
		{"every key", "default_index: index.md\ndir_index: true\nedit_url: https://x/\nlanguage: fr-FR\n" +
			"meta: {title: T, description: D, domain: d.example, robots: noindex}\n" +
			"theme: {name: nord, vars: {bg: '#fff'}, features: {toc: false}}\n" +
			"highlighting: {theme: monokai}\nsearch: {index: false}\nexclude: [drafts/]\nstrip_extensions: [.md]\n", ""},
		{"unknown root key", "nope: 1\n", ".nope: unknown key"},
		{"unknown nested key", "meta: {titel: x}\n", ".meta.titel: unknown key"},
		{"site wrapper", "site: {meta: {domain: x}}\n", ".site: unknown key"},
		{"server key in file", "port: ':9000'\n", ".port: unknown key"},
		{"bad feature key", "theme: {features: {Toc: false}}\n", ".theme.features.Toc: key does not match propertyNames"},
		{"feature not bool", "theme: {features: {toc: 'no'}}\n", ".theme.features.toc: want boolean"},
		{"exclude not list", "exclude: drafts/\n", ".exclude: want array"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var doc map[string]any
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatal(err)
			}
			errs := validateAgainst(s, doc, "")
			switch {
			case tt.wantErr == "" && len(errs) > 0:
				t.Errorf("want valid, got %q", errs)
			case tt.wantErr != "" && !slices.Equal(errs, []string{tt.wantErr}):
				t.Errorf("errs = %q, want [%q]", errs, tt.wantErr)
			}
		})
	}
}

func TestJSONSchema_ReturnsCopy(t *testing.T) {
	t.Parallel()
	a := JSONSchema()
	a[0] = 'X'
	if JSONSchema()[0] == 'X' {
		t.Error("JSONSchema returns the cached slice")
	}
}
```

The "every key" row is also what makes `TestJSONSchema_Validates` falsifiable against a schema that dropped a
property: a missing leaf turns that row into an "unknown key" error.

- [x] **Step 2: Run to verify failure**

Run: `go test ./internal/config/ -run TestJSONSchema -v`
Expected: FAIL — `undefined: JSONSchema`

- [x] **Step 3: Implement**

```go
// JSONSchema returns a JSON Schema (draft 2020-12) for .gomddoc/config.yml,
// projected from Schema. The document is static, so it is built once; each
// call returns a copy because the caller owns what it is handed.
func JSONSchema() []byte { return slices.Clone(jsonSchema()) }

var jsonSchema = sync.OnceValue(func() []byte {
	root := schemaObject()
	root["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	root["$id"] = "gomddoc://schema/config"
	root["title"] = "gomddoc site configuration (.gomddoc/config.yml)"
	root["description"] = "Unknown keys are rejected at load time. " + FileKeysNote
	for _, s := range Schema() {
		if s.FileKey == "" {
			continue
		}
		parts := strings.Split(s.FileKey, ".")
		parent := root
		for _, p := range parts[:len(parts)-1] {
			props := parent["properties"].(map[string]any)
			child, ok := props[p].(map[string]any)
			if !ok {
				child = schemaObject()
				props[p] = child
			}
			parent = child
		}
		parent["properties"].(map[string]any)[parts[len(parts)-1]] = settingSchema(s)
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		panic("config: marshal JSON Schema: " + err.Error()) // maps of strings, bools and slices cannot fail
	}
	return data
})

func schemaObject() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}}
}

func settingSchema(s Setting) map[string]any {
	m := map[string]any{"description": s.Description}
	switch s.Type {
	case "string":
		m["type"] = "string"
	case "bool":
		m["type"] = "boolean"
	case "[]string":
		m["type"] = "array"
		m["items"] = map[string]any{"type": "string"}
	case "map[string]string":
		m["type"] = "object"
		m["additionalProperties"] = map[string]any{"type": "string"}
	case "map[string]bool":
		m["type"] = "object"
		m["propertyNames"] = map[string]any{"pattern": featureKeyPattern.String()}
		m["additionalProperties"] = map[string]any{"type": "boolean"}
	default:
		// A new file-settable type must be mapped here; TestJSONSchema_* panics first.
		panic(fmt.Sprintf("config: no JSON Schema mapping for %s (%s)", s.Key, s.Type))
	}
	if s.DefaultNote != "" {
		m["description"] = s.Description + " Default: " + s.DefaultNote + "."
	} else if s.Default != nil {
		m["default"] = s.Default
	}
	return m
}
```

- [x] **Step 4: Run to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS

- [x] **Step 5: `make ci`, then draft the commit message** (`feat(config): project a JSON Schema for
  config.yml`).

---

### Task 4: Declared frontmatter fields

**Files:**
- Create: `internal/metadata/frontmatter_fields.go`, `internal/metadata/frontmatter_fields_test.go`
- Modify: `docs/guide/12-advanced/02-markdown-extensions.md` (add the `author` row)

**Interfaces:**
- Produces: `type FrontmatterField struct{ Key, Type, Description string }` with JSON tags `key`, `type`,
  `description`; `var FrontmatterFields []FrontmatterField`; `func FrontmatterFieldList() []FrontmatterField`
  returning a clone (the report must not alias the package var).

- [x] **Step 1: Write the failing tests**

```go
package metadata

import (
	"io/fs"
	"regexp"
	"slices"
	"testing"

	"github.com/monolithiclab/gomddoc/docs"
)

var fieldRow = regexp.MustCompile("(?m)^\\| `([a-z_]+)` \\|")

// TestFrontmatterFields_MatchGuide: the declared list and the guide's
// "Standard Fields" table are the same set, in both directions.
func TestFrontmatterFields_MatchGuide(t *testing.T) {
	t.Parallel()
	page, err := fs.ReadFile(docs.Guide, "12-advanced/02-markdown-extensions.md")
	if err != nil {
		t.Fatal(err)
	}
	var documented []string
	for _, m := range fieldRow.FindAllSubmatch(page, -1) {
		documented = append(documented, string(m[1]))
	}
	var declared []string
	for _, f := range FrontmatterFields {
		declared = append(declared, f.Key)
	}
	slices.Sort(documented)
	slices.Sort(declared)
	if !slices.Equal(documented, declared) {
		t.Errorf("guide table %q != FrontmatterFields %q", documented, declared)
	}
}

func TestFrontmatterFields_CoverKnownKeys(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"title", "description", "date", "tags"} { // BuildIndex's knownKeys
		if !slices.ContainsFunc(FrontmatterFields, func(f FrontmatterField) bool { return f.Key == k }) {
			t.Errorf("knownKeys entry %q is not declared", k)
		}
	}
}

func TestFrontmatterFieldList_IsCopy(t *testing.T) {
	t.Parallel()
	l := FrontmatterFieldList()
	l[0].Key = "mutated"
	if FrontmatterFields[0].Key == "mutated" {
		t.Error("FrontmatterFieldList aliases the package list")
	}
}
```

Check first that `fieldRow` matches **only** the Standard Fields table (grep the page for other backticked first
columns). If another table matches, restrict parsing to the text between `### Standard Fields` and the next `###`.

- [x] **Step 2: Run to verify failure**

Run: `go test ./internal/metadata/ -run Frontmatter -v`
Expected: FAIL — `undefined: FrontmatterFields`

- [x] **Step 3: Implement**

```go
package metadata

// FrontmatterField is a frontmatter key gomddoc gives special behaviour.
// Anything not listed lands in PageInfo.Meta untouched.
type FrontmatterField struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// FrontmatterFields lists every special frontmatter key, wherever it is read
// (metadata, template, seo, resolve). It is the list agents see; a drift test
// holds it equal to the guide's Standard Fields table.
var FrontmatterFields = []FrontmatterField{
	{"title", "string", "Page title: <title>, Open Graph, search results, navigation label."},
	{"description", "string", "Page description: <meta name=description>, Open Graph, search results."},
	{"tags", "[]string", "Tags, lowercased and trimmed: tag pages, /api/tags, related pages, tag: search."},
	{"date", "date", "Publication date (YYYY-MM-DD or RFC 3339): feed, JSON-LD datePublished, sitemap fallback."},
	{"author", "string", "Author name for JSON-LD and the byline."},
	{"og_type", "string", "Open Graph type (default article)."},
	{"robots", "string", "Per-page <meta name=robots>; noindex also drops the page from sitemap and feed."},
	{"lang", "string", "BCP 47 language for this page's <html lang>, overriding site.language."},
	{"layout", "string", "Alternate layout template: layout: wide renders with wide.html.tmpl."},
	{"redirect_from", "[]string", "Old URL paths that 301-redirect to this page."},
	{"features", "map[string]bool", "Per-page theme feature overrides, merged over site.theme.features."},
}

// FrontmatterFieldList returns a copy of FrontmatterFields.
func FrontmatterFieldList() []FrontmatterField { return slices.Clone(FrontmatterFields) }
```

Verify each description against its consumer before committing (`renderer.go:632` for `author`, `renderer.go:716`
for `layout`, `seo/jsonld.go`, `metadata.ParseFrontmatterDate`).

- [x] **Step 4: Add the `author` row** to the Standard Fields table in `02-markdown-extensions.md`, with the same
  description as the declared entry.

- [x] **Step 5: Run to verify it passes**

Run: `go test ./internal/metadata/ -v`
Expected: PASS

- [x] **Step 6: `make ci`, then draft the commit message** (`feat(metadata): declare the special frontmatter
  fields`).

---

### Task 5: Guide drift test for settings and flags (+ guide fixes)

**Files:**
- Modify: `cmd/gomddoc/build.go`, `serve.go`, `preview.go` (`setting:` tags)
- Create: `cmd/gomddoc/guide_drift_test.go`
- Modify: `cmd/gomddoc/main.go` (`parserOptions`, `ctx.Run(ctx.Model)`)
- Modify: `docs/guide/*.md` as the test demands

**Interfaces:**
- Consumes: `config.Schema()`, `docs.Guide`, `parserOptions()` (introduced here in `main.go`, see Step 3).
- Produces: `func parserOptions() []kong.Option`; Kong struct tag `setting:"<Setting.Key>"` on flags that set a
  config value under a different env var.

- [x] **Step 1: Write the failing test** (`guide_drift_test.go`)

```go
package main

import (
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/kong"

	"github.com/monolithiclab/gomddoc/docs"
	"github.com/monolithiclab/gomddoc/internal/config"
)

var envMention = regexp.MustCompile(`GOMDDOC_[A-Z0-9_]+`)

func testModel(t *testing.T) *kong.Application {
	t.Helper()
	return kong.Must(&CLI{}, parserOptions()...).Model
}

// knownEnvs is every variable gomddoc reads: config settings plus Kong flag
// envs. Map settings contribute their prefix ("GOMDDOC_SITE_THEME_FEATURES_").
func knownEnvs(t *testing.T) (exact []string, prefixes []string) {
	for _, s := range config.Schema() {
		if p, ok := strings.CutSuffix(s.Env, "<KEY>"); ok {
			prefixes = append(prefixes, p)
		} else if s.Env != "" {
			exact = append(exact, s.Env)
		}
	}
	for _, n := range testModel(t).Children {
		for _, f := range n.Flags {
			exact = append(exact, f.Envs...)
		}
		for _, p := range n.Positional {
			exact = append(exact, p.Tag.Envs...)
		}
	}
	return exact, prefixes
}

// TestGuide_MentionsOnlyRealEnvVars: a guide that names a variable gomddoc does
// not read sends an agent to set something that does nothing.
func TestGuide_MentionsOnlyRealEnvVars(t *testing.T) {
	t.Parallel()
	exact, prefixes := knownEnvs(t)
	err := fs.WalkDir(docs.Guide, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".md" {
			return err
		}
		data, err := fs.ReadFile(docs.Guide, p)
		if err != nil {
			return err
		}
		for _, m := range envMention.FindAllString(string(data), -1) {
			if slices.Contains(exact, m) ||
				slices.ContainsFunc(prefixes, func(pre string) bool { return strings.HasPrefix(m, pre) }) {
				continue
			}
			t.Errorf("%s mentions %s, which gomddoc does not read", p, m)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestGuide_ConfigurationPageCoversEverySetting: every file key and env var in
// config.Schema is documented on the configuration page.
func TestGuide_ConfigurationPageCoversEverySetting(t *testing.T) {
	t.Parallel()
	page, err := fs.ReadFile(docs.Guide, "02-configuration.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range config.Schema() {
		if s.FileKey != "" && !strings.Contains(string(page), "`"+s.FileKey) {
			t.Errorf("02-configuration.md does not mention file key `%s`", s.FileKey)
		}
		if env, _ := strings.CutSuffix(s.Env, "<KEY>"); env != "" && !strings.Contains(string(page), env) {
			t.Errorf("02-configuration.md does not mention %s", env)
		}
	}
}
```

- [x] **Step 2: Introduce `parserOptions()` in `main.go`** (needed by `testModel`) and switch `main` to it:

```go
// parserOptions configures the Kong parser. Tests build the model through the
// same options, so the commands the capabilities report lists are the ones a
// user runs.
func parserOptions() []kong.Option {
	return []kong.Option{
		kong.Name("gomddoc"),
		kong.Description("A production-ready HTTP server that serves Markdown files as HTML."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
	}
}

func main() {
	cli := CLI{}
	ctx := kong.Parse(&cli, parserOptions()...)
	if err := ctx.Run(ctx.Model); err != nil {
		slog.Error("Fatal error", slog.Any("error", err))
		os.Exit(1)
	}
}
```

`ctx.Run(ctx.Model)` binds `*kong.Application` so a command's `Run(app *kong.Application)` can receive it; commands
whose `Run()` takes nothing are unaffected.

- [x] **Step 3: Run to verify failure**

Run: `go test ./cmd/gomddoc/ -run TestGuide_ -v`
Expected: FAIL — at least `02-configuration.md mentions GOMDDOC_SITE_FEATURES_KATEX, which gomddoc does not read`.

- [x] **Step 4: Fix the guide** until both tests pass. Known fix: `GOMDDOC_SITE_FEATURES_KATEX` →
  `GOMDDOC_SITE_THEME_FEATURES_KATEX`. For each other failure, add the missing key/variable to the relevant
  section of `02-configuration.md` (Site Configuration tables, Network Tuning) using the setting's doc tag
  wording. Do not weaken the tests.

- [x] **Step 5: Add `setting:` tags** so Task 8 can join flags whose env var differs from the setting's:
  - `ServeCmd.Domain`, `PreviewCmd.Domain`, `BuildCmd.Domain`: `setting:"site.meta.domain"`
  - `PreviewCmd.DirIndex`: `setting:"site.dir_index"`
  - Grep all command structs for any other flag that feeds a `config.Config` field (follow `ServeArgs` and
    `ServerSetupOptions` into `config`) and tag it the same way.

- [x] **Step 6: Run to verify it passes**

Run: `go test ./cmd/gomddoc/ -run TestGuide_ -v`
Expected: PASS

- [x] **Step 7: `make ci`, then draft the commit message** (`test(docs): hold the guide to the env vars and keys
  gomddoc reads`, body naming the `GOMDDOC_SITE_FEATURES_KATEX` fix).

---

### Task 6: `template.ThemeFeatures` (best effort)

**Files:**
- Create: `internal/template/themefeatures.go`, `internal/template/themefeatures_test.go`

**Interfaces:**
- Produces:

```go
const (
	ThemeSourceTemplateScan = "template-scan"
	ThemeSourceUnavailable  = "unavailable"
)
type ThemeInfo struct {
	Name     string   `json:"active"`
	Source   string   `json:"source"`
	Features []string `json:"features"` // sorted, never nil
	Vars     []string `json:"vars"`     // sorted, never nil; "--theme-" stripped
}
func ThemeFeatures(assets fs.FS, name string) ThemeInfo
```

`assets` is the asset FS as the renderer sees it (embedded `assets/` possibly overlaid by `.gomddoc/`), so theme
files live under `assets/themes/<name>/`.

- [x] **Step 1: Write the failing tests**

```go
package template

import (
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/provider"
)

func TestThemeFeatures(t *testing.T) {
	t.Parallel()
	embedded := defaultThemeAssets(t) // see Step 1b
	overlay := provider.NewOverlayFS(fstest.MapFS{
		"assets/themes/default/partials/banner.html.tmpl": {Data: []byte(`{{ if .Feature "banner" }}<div style="color: var(--theme-banner-fg)"></div>{{ end }}`)},
	}, embedded)

	wantDefault := ThemeInfo{
		Name:   "default",
		Source: ThemeSourceTemplateScan,
		Features: []string{"admonitions", "code_copy", "color_chips", "dark_mode", "heading_anchors", "katex",
			"mermaid", "search", "see_also", "tag_chips", "toc"},
		Vars: []string{"bg", "dark-bg", "dark-primary", "dark-text", "primary", "text"},
	}
	withBanner := wantDefault
	withBanner.Features = slices.Insert(slices.Clone(wantDefault.Features), 1, "banner")
	withBanner.Vars = slices.Insert(slices.Clone(wantDefault.Vars), 0, "banner-fg")

	tests := []struct {
		name   string
		assets fs.FS
		theme  string
		want   ThemeInfo
	}{
		{"embedded default", embedded, "default", wantDefault},
		{"site overlay adds a feature and a var", overlay, "default", withBanner},
		{"missing theme", embedded, "nord", ThemeInfo{Name: "nord", Source: ThemeSourceUnavailable, Features: []string{}, Vars: []string{}}},
		{"theme dir without templates", fstest.MapFS{"assets/themes/empty/static/x.css": {Data: []byte("a{}")}}, "empty",
			ThemeInfo{Name: "empty", Source: ThemeSourceUnavailable, Features: []string{}, Vars: []string{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ThemeFeatures(tt.assets, tt.theme)
			if got.Name != tt.want.Name || got.Source != tt.want.Source ||
				!slices.Equal(got.Features, tt.want.Features) || !slices.Equal(got.Vars, tt.want.Vars) {
				t.Errorf("got %+v\nwant %+v", got, tt.want)
			}
			if got.Features == nil || got.Vars == nil {
				t.Error("lists must be non-nil so they marshal as [] not null")
			}
		})
	}
}
```

- [x] **Step 1b: Provide the embedded default theme to the test.** The embed lives in `cmd/gomddoc`
  (`embeddedAssets`), which `internal/template` cannot import. Check how existing `internal/template` tests get
  real theme files (`grep -rn "os.DirFS\|cmd/gomddoc/assets" internal/template/*_test.go`) and reuse that helper;
  if none exists, write `defaultThemeAssets(t)` returning
  `fstest`-free `os.DirFS("../../cmd/gomddoc")` (whose `assets/themes/default/…` layout matches the embed) and put
  it in `internal/template/testhelpers_test.go`.

- [x] **Step 2: Run to verify failure**

Run: `go test ./internal/template/ -run TestThemeFeatures -v`
Expected: FAIL — `undefined: ThemeFeatures`

- [x] **Step 3: Implement**

```go
package template

import (
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
)

// Theme facts reported by ThemeFeatures come from where they are sourced.
const (
	ThemeSourceTemplateScan = "template-scan"
	ThemeSourceUnavailable  = "unavailable"
)

// ThemeInfo lists the feature keys and CSS variables a theme reads.
type ThemeInfo struct { /* as in Interfaces */ }

var (
	featureCall = regexp.MustCompile(`\.Feature\s+"([a-z][a-z0-9_]*)"`)
	themeVarUse = regexp.MustCompile(`var\(--theme-([a-z0-9-]+)`)
)

// ThemeFeatures scans a theme's templates for {{ .Feature "x" }} and its
// templates and CSS for var(--theme-x). It is best effort — names only, no
// descriptions or defaults — until themes declare their features (see
// docs/plans/2026-09-23-theme-manifest-parked.md). It reads through the same
// overlay the renderer uses, so a site-level override is seen. A theme that is
// not installed, or has no templates, is reported unavailable rather than as an
// error: the caller is describing, not rendering.
func ThemeFeatures(assets fs.FS, name string) ThemeInfo {
	info := ThemeInfo{Name: name, Source: ThemeSourceUnavailable, Features: []string{}, Vars: []string{}}
	features, vars := map[string]struct{}{}, map[string]struct{}{}
	templates := 0
	root := path.Join("assets", "themes", name)
	err := fs.WalkDir(assets, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := path.Ext(p)
		if ext != ".tmpl" && ext != ".css" {
			return nil
		}
		data, err := fs.ReadFile(assets, p)
		if err != nil {
			return err
		}
		if ext == ".tmpl" {
			templates++
			for _, m := range featureCall.FindAllSubmatch(data, -1) {
				features[string(m[1])] = struct{}{}
			}
		}
		for _, m := range themeVarUse.FindAllSubmatch(data, -1) {
			vars[string(m[1])] = struct{}{}
		}
		return nil
	})
	if err != nil || templates == 0 {
		return info
	}
	info.Source = ThemeSourceTemplateScan
	info.Features = slices.Sorted(maps.Keys(features))
	info.Vars = slices.Sorted(maps.Keys(vars))
	return info
}
```

- [x] **Step 4: Run to verify it passes**

Run: `go test ./internal/template/ -run TestThemeFeatures -v`
Expected: PASS. If the default-theme lists differ from the test's, the test is the claim to check — grep the
templates, and fix whichever side is wrong.

- [x] **Step 5: `make ci`, then draft the commit message** (`feat(template): list a theme's feature keys and CSS
  vars by template scan`).

---

### Task 7: `internal/capabilities`

**Files:**
- Create: `internal/capabilities/capabilities.go`, `internal/capabilities/capabilities_test.go`

**Interfaces:**
- Consumes: `config.Schema`, `config.Precedence`, `config.FileKeysNote`, `config.DefaultThemeName`,
  `config.ConfigDirName`, `config.ConfigFileName`, `metadata.FrontmatterFieldList`, `metadata.BuildIndex`,
  `template.ThemeFeatures`.
- Produces:

```go
type Input struct {
	Version   string
	Dir       string
	Config    *config.Config // nil when it could not be loaded
	ConfigErr error
	FileFound bool
	Commands  []Command
	Assets    fs.FS // nil: theme reported unavailable
	Guide     fs.FS
}
type Command struct {
	Name  string `json:"name"`
	Help  string `json:"help"`
	Args  []Arg  `json:"args,omitempty"`
	Flags []Flag `json:"flags,omitempty"`
}
type Arg struct {
	Name     string `json:"name"`
	Help     string `json:"help"`
	Default  string `json:"default,omitempty"`
	Env      string `json:"env,omitempty"`
	Required bool   `json:"required"`
}
type Flag struct {
	Name    string `json:"name"`            // "--port"
	Short   string `json:"short,omitempty"` // "-p"
	Help    string `json:"help"`
	Default string `json:"default,omitempty"`
	Env     string `json:"env,omitempty"`
	Setting string `json:"setting,omitempty"` // Kong `setting:` tag; "" = join by Env
}
type ConfigInfo struct {
	Dir        string   `json:"dir"`
	File       string   `json:"file"`
	FileFound  bool     `json:"file_found"`
	FileKeys   string   `json:"file_keys"`
	Precedence []string `json:"precedence"`
	Loaded     bool     `json:"loaded"`
	LoadError  string   `json:"load_error,omitempty"`
	SchemaURI  string   `json:"schema_uri"`
}
type ThemeReport struct {
	template.ThemeInfo
	Docs string `json:"docs"`
}
type GuidePage struct {
	Path        string `json:"path"` // "02-configuration.md", no leading slash
	Title       string `json:"title"`
	Description string `json:"description"`
}
type Report struct {
	Version     string                      `json:"version"`
	Config      ConfigInfo                  `json:"config"`
	Settings    []config.Setting            `json:"settings"`
	Commands    []Command                   `json:"commands"`
	Frontmatter []metadata.FrontmatterField `json:"frontmatter"`
	Theme       ThemeReport                 `json:"theme"`
	Guide       []GuidePage                 `json:"guide"`
	GuideError  string                      `json:"guide_error,omitempty"`
}
const (
	CapabilitiesURI = "gomddoc://capabilities"
	SchemaURI       = "gomddoc://schema/config"
	GuideURIPrefix  = "gomddoc://guide/"
)
func Describe(in Input) Report
func (r Report) JSON() []byte
```

- [x] **Step 1: Write the failing tests**

```go
package capabilities

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
)

var testGuide = fstest.MapFS{
	"README.md":          {Data: []byte("---\ntitle: Guide\ndescription: Start here\n---\n# Guide\n")},
	"02-configuration.md": {Data: []byte("---\ntitle: Configuration\ndescription: Settings\n---\n# Config\n")},
	"12-advanced/01-http.md": {Data: []byte("---\ntitle: HTTP\ndescription: Caching\n---\n# HTTP\n")},
}

var testCommands = []Command{
	{Name: "serve", Flags: []Flag{
		{Name: "--port", Env: "GOMDDOC_SERVER_PORT"},
		{Name: "--domain", Env: "GOMDDOC_DOMAIN", Setting: "site.meta.domain"},
		{Name: "--basic-auth-file", Env: "GOMDDOC_SERVER_BASIC_AUTH_FILE"}, // command-only
	}, Args: []Arg{{Name: "dir", Env: "GOMDDOC_SERVER_DIR"}}},
	{Name: "preview", Flags: []Flag{{Name: "--port", Env: "GOMDDOC_SERVER_PORT"}}},
}

func settingByKey(r Report, key string) config.Setting {
	i := slices.IndexFunc(r.Settings, func(s config.Setting) bool { return s.Key == key })
	if i < 0 {
		return config.Setting{}
	}
	return r.Settings[i]
}

func TestDescribe_JoinsFlagsOntoSettings(t *testing.T) {
	t.Parallel()
	r := Describe(Input{Config: config.New(), Commands: testCommands, Guide: testGuide})
	tests := []struct {
		key  string
		want []string
	}{
		{"server.port", []string{"--port"}},         // joined by env, deduplicated across commands
		{"site.meta.domain", []string{"--domain"}},  // joined by setting tag; env differs
		{"server.dir", []string{"DIR"}},             // positional
		{"site.theme.name", nil},                    // no flag
	}
	for _, tt := range tests {
		if got := settingByKey(r, tt.key).Flags; !slices.Equal(got, tt.want) {
			t.Errorf("%s flags = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestDescribe_ConfigBlock(t *testing.T) {
	t.Parallel()
	loaded := Describe(Input{Dir: "site", Config: config.New(), FileFound: true, Guide: testGuide})
	failed := Describe(Input{Dir: "site", ConfigErr: errors.New("config.yml: field nope not found"), Guide: testGuide})

	if !loaded.Config.Loaded || loaded.Config.LoadError != "" || !loaded.Config.FileFound {
		t.Errorf("loaded: %+v", loaded.Config)
	}
	if failed.Config.Loaded || failed.Config.LoadError != "config.yml: field nope not found" {
		t.Errorf("failed: %+v", failed.Config)
	}
	if failed.Theme.Name != config.DefaultThemeName {
		t.Errorf("theme falls back to %q, got %q", config.DefaultThemeName, failed.Theme.Name)
	}
	if !slices.Equal(loaded.Config.Precedence, config.Precedence) || loaded.Config.SchemaURI != SchemaURI ||
		loaded.Config.File != ".gomddoc/config.yml" || loaded.Config.FileKeys != config.FileKeysNote {
		t.Errorf("config block: %+v", loaded.Config)
	}
}

func TestDescribe_Guide(t *testing.T) {
	t.Parallel()
	r := Describe(Input{Guide: testGuide})
	want := []GuidePage{
		{"02-configuration.md", "Configuration", "Settings"},
		{"12-advanced/01-http.md", "HTTP", "Caching"},
		{"README.md", "Guide", "Start here"},
	}
	if !slices.Equal(r.Guide, want) {
		t.Errorf("guide = %+v\nwant %+v", r.Guide, want)
	}
}

func TestDescribe_NilAssetsThemeUnavailable(t *testing.T) {
	t.Parallel()
	r := Describe(Input{Config: config.New(), Guide: testGuide})
	if r.Theme.Source != "unavailable" || r.Theme.Docs != GuideURIPrefix+"05-theming-and-assets.md" {
		t.Errorf("theme: %+v", r.Theme)
	}
}

func TestReport_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	r := Describe(Input{Version: "1.2.3", Config: config.New(), Commands: testCommands, Guide: testGuide})
	var back Report
	if err := json.Unmarshal(r.JSON(), &back); err != nil {
		t.Fatal(err)
	}
	if back.Version != "1.2.3" || len(back.Settings) != len(r.Settings) || len(back.Commands) != 2 ||
		len(back.Frontmatter) == 0 || len(back.Guide) != 3 {
		t.Errorf("round trip lost data: %+v", back)
	}
	// Stable output: same inputs, same bytes (MCP and info --json are compared byte-for-byte).
	if string(r.JSON()) != string(Describe(Input{Version: "1.2.3", Config: config.New(), Commands: testCommands, Guide: testGuide}).JSON()) {
		t.Error("JSON() is not deterministic")
	}
}
```

- [x] **Step 2: Run to verify failure**

Run: `go test ./internal/capabilities/ -v`
Expected: FAIL — package does not exist

- [x] **Step 3: Implement**

```go
// Package capabilities answers "what can gomddoc do, and how is it
// configured" as one Report: settings (from config struct tags), commands
// (from Kong's model, converted by the caller), special frontmatter keys, the
// active theme's features, and the embedded guide's pages.
//
// Every transport renders the same Report — `gomddoc info` as text, `info
// --json` and the gomddoc://capabilities MCP resource through Report.JSON — so
// none of them may assemble its own. Describe never fails: a config that did
// not load or a theme that is not installed is reported in-band, because the
// situations where an agent asks for this report are exactly the ones where
// something is wrong.
package capabilities

// … types from Interfaces …

// Describe builds the report.
func Describe(in Input) Report {
	r := Report{
		Version:     in.Version,
		Settings:    config.Schema(),
		Commands:    in.Commands,
		Frontmatter: metadata.FrontmatterFieldList(),
		Config: ConfigInfo{
			Dir:        in.Dir,
			File:       config.ConfigDirName + "/" + config.ConfigFileName,
			FileFound:  in.FileFound,
			FileKeys:   config.FileKeysNote,
			Precedence: slices.Clone(config.Precedence),
			Loaded:     in.Config != nil,
			SchemaURI:  SchemaURI,
		},
	}
	if in.ConfigErr != nil {
		r.Config.LoadError = in.ConfigErr.Error()
	}
	joinFlags(r.Settings, in.Commands)

	themeName := config.DefaultThemeName
	if in.Config != nil {
		themeName = in.Config.Site.Theme.Name
	}
	info := template.ThemeInfo{Name: themeName, Source: template.ThemeSourceUnavailable, Features: []string{}, Vars: []string{}}
	if in.Assets != nil {
		info = template.ThemeFeatures(in.Assets, themeName)
	}
	r.Theme = ThemeReport{ThemeInfo: info, Docs: GuideURIPrefix + "05-theming-and-assets.md"}

	pages, err := guidePages(in.Guide)
	r.Guide = pages
	if err != nil {
		r.GuideError = err.Error()
	}
	return r
}

// joinFlags records, on each setting, the flags and positional args that set
// it: by the flag's `setting` tag when present, else by env var name — the one
// key both a Kong flag and a config field already carry.
func joinFlags(settings []config.Setting, cmds []Command) {
	byEnv, byKey := map[string]int{}, map[string]int{}
	for i, s := range settings {
		byKey[s.Key] = i
		if s.Env != "" {
			byEnv[s.Env] = i
		}
	}
	add := func(i int, name string) {
		if !slices.Contains(settings[i].Flags, name) {
			settings[i].Flags = append(settings[i].Flags, name)
		}
	}
	for _, c := range cmds {
		for _, f := range c.Flags {
			if i, ok := byKey[f.Setting]; ok {
				add(i, f.Name)
			} else if i, ok := byEnv[f.Env]; ok && f.Setting == "" {
				add(i, f.Name)
			}
		}
		for _, a := range c.Args {
			if i, ok := byEnv[a.Env]; ok {
				add(i, strings.ToUpper(a.Name))
			}
		}
	}
	for i := range settings {
		slices.Sort(settings[i].Flags)
	}
}

func guidePages(guide fs.FS) ([]GuidePage, error) {
	if guide == nil {
		return []GuidePage{}, nil
	}
	idx, err := metadata.BuildIndex(context.Background(), guide, nil)
	if err != nil {
		return []GuidePage{}, err
	}
	pages := []GuidePage{}
	for p := range idx.AllPagesSeq() { // or AllPages(); pick the accessor that exists
		pages = append(pages, GuidePage{strings.TrimPrefix(p.Path, "/"), p.Title, p.Description})
	}
	slices.SortFunc(pages, func(a, b GuidePage) int { return cmp.Compare(a.Path, b.Path) })
	return pages, nil
}

// JSON is the one serialization of a Report; every transport uses it.
func (r Report) JSON() []byte {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		panic("capabilities: marshal report: " + err.Error()) // plain data; cannot fail
	}
	return data
}
```

`metadata.Index` exposes `AllPages()` (deep copies) — use it unless an iterator accessor exists; this runs once per
process. Settings with nil `Default` marshal as `null`, which is intended.

- [x] **Step 4: Run to verify it passes**

Run: `go test ./internal/capabilities/ -v`
Expected: PASS

- [x] **Step 5: `make ci`, then draft the commit message** (`feat(capabilities): assemble one self-description
  report`).

---

### Task 8: Kong model conversion and the capabilities input

**Files:**
- Modify: `cmd/gomddoc/capabilities.go`
- Create: `cmd/gomddoc/capabilities_test.go`

**Interfaces:**
- Consumes: `capabilities.Command/Flag/Arg/Input`, `docs.Guide`, `embeddedAssets`, `assets.BuildFS`.
- Produces:

```go
func commandsFromKong(app *kong.Application) []capabilities.Command
// capabilitiesInput builds the report input for dir. cfg/cfgErr are whatever the
// caller's config load produced; contentRoot may be nil (then no overlay).
func capabilitiesInput(app *kong.Application, dir string, cfg *config.Config, cfgErr error, contentRoot fs.FS) capabilities.Input
```

- [x] **Step 1: Write the failing tests**

```go
package main

import (
	"slices"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/capabilities"
	"github.com/monolithiclab/gomddoc/internal/config"
)

// TestSettingTags_Resolve: a `setting:` tag naming a key that does not exist
// would silently drop the join.
func TestSettingTags_Resolve(t *testing.T) {
	t.Parallel()
	keys := map[string]bool{}
	for _, s := range config.Schema() {
		keys[s.Key] = true
	}
	for _, c := range commandsFromKong(testModel(t)) {
		for _, f := range c.Flags {
			if f.Setting != "" && !keys[f.Setting] {
				t.Errorf("%s %s: setting:%q is not a config key", c.Name, f.Name, f.Setting)
			}
		}
	}
}

func TestCommandsFromKong(t *testing.T) {
	t.Parallel()
	cmds := commandsFromKong(testModel(t))
	var names []string
	for _, c := range cmds {
		names = append(names, c.Name)
	}
	want := []string{"build", "info", "init", "mcp", "preview", "schema", "serve"}
	if !slices.Equal(names, want) {
		t.Errorf("commands = %q, want %q", names, want)
	}
	serve := cmds[slices.IndexFunc(cmds, func(c capabilities.Command) bool { return c.Name == "serve" })]
	i := slices.IndexFunc(serve.Flags, func(f capabilities.Flag) bool { return f.Name == "--port" })
	if i < 0 {
		t.Fatal("serve has no --port")
	}
	if got := serve.Flags[i]; got.Short != "-p" || got.Env != "GOMDDOC_SERVER_PORT" || got.Default != ":8080" || got.Help == "" {
		t.Errorf("--port = %+v", got)
	}
	j := slices.IndexFunc(serve.Flags, func(f capabilities.Flag) bool { return f.Name == "--domain" })
	if serve.Flags[j].Setting != "site.meta.domain" {
		t.Errorf("--domain setting = %q", serve.Flags[j].Setting)
	}
	if len(serve.Args) != 1 || serve.Args[0].Name != "dir" || serve.Args[0].Env != "GOMDDOC_SERVER_DIR" {
		t.Errorf("serve args = %+v", serve.Args)
	}
}

func TestCapabilitiesInput_FileFound(t *testing.T) {
	t.Parallel()
	withFile := fstest.MapFS{".gomddoc/config.yml": {Data: []byte("theme: {name: default}\n")}}
	if in := capabilitiesInput(testModel(t), "x", config.New(), nil, withFile); !in.FileFound {
		t.Error("FileFound = false with a config file present")
	}
	if in := capabilitiesInput(testModel(t), "x", config.New(), nil, fstest.MapFS{}); in.FileFound {
		t.Error("FileFound = true with no config file")
	}
}
```

The `"schema"` entry in the command list is added by Task 9; until then remove it from `want` and add it back there.

- [x] **Step 2: Run to verify failure**

Run: `go test ./cmd/gomddoc/ -run 'TestSettingTags|TestCommandsFromKong|TestCapabilitiesInput' -v`
Expected: FAIL — `undefined: commandsFromKong`

- [x] **Step 3: Implement**

```go
// commandsFromKong converts Kong's model into the capabilities form, so
// internal/capabilities does not depend on the CLI library. Commands are
// sorted by name; hidden commands, hidden flags and Kong's own --help are
// left out.
func commandsFromKong(app *kong.Application) []capabilities.Command {
	var cmds []capabilities.Command
	for _, n := range app.Children {
		if n.Hidden {
			continue
		}
		c := capabilities.Command{Name: n.Name, Help: n.Help}
		for _, p := range n.Positional {
			c.Args = append(c.Args, capabilities.Arg{
				Name: p.Name, Help: p.Help, Default: p.Default, Env: firstOr(p.Tag.Envs), Required: p.Required,
			})
		}
		for _, f := range n.Flags {
			if f.Hidden || f.Name == "help" {
				continue
			}
			short := ""
			if f.Short != 0 {
				short = "-" + string(f.Short)
			}
			c.Flags = append(c.Flags, capabilities.Flag{
				Name: "--" + f.Name, Short: short, Help: f.Help, Default: f.Default,
				Env: firstOr(f.Envs), Setting: f.Tag.Get("setting"),
			})
		}
		cmds = append(cmds, c)
	}
	slices.SortFunc(cmds, func(a, b capabilities.Command) int { return cmp.Compare(a.Name, b.Name) })
	return cmds
}

func firstOr(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

func capabilitiesInput(app *kong.Application, dir string, cfg *config.Config, cfgErr error, contentRoot fs.FS) capabilities.Input {
	in := capabilities.Input{
		Version: version, Dir: dir, Config: cfg, ConfigErr: cfgErr,
		Commands: commandsFromKong(app), Guide: docs.Guide, Assets: embeddedAssets,
	}
	if contentRoot != nil {
		_, err := fs.Stat(contentRoot, config.ConfigDirName+"/"+config.ConfigFileName)
		in.FileFound = err == nil
		in.Assets = assets.BuildFS(contentRoot, embeddedAssets)
	}
	return in
}
```

Check whether `n.Flags` includes Kong's inherited app-level `--help`/`--version` flags; the `f.Name == "help"`
filter must exclude exactly those (add `"version"` if it appears).

- [x] **Step 4: Run to verify it passes**

Run: `go test ./cmd/gomddoc/ -v -run 'TestSettingTags|TestCommandsFromKong|TestCapabilitiesInput'`
Expected: PASS

- [x] **Step 5: `make ci`, then draft the commit message** (`feat(cli): convert the Kong model for the
  capabilities report`).

---

### Task 9: `gomddoc info [DIR] [--json]` and `gomddoc schema`

**Files:**
- Rewrite: `cmd/gomddoc/info.go`
- Create: `cmd/gomddoc/info_test.go`, `cmd/gomddoc/schema.go`, `cmd/gomddoc/schema_test.go`
- Modify: `cmd/gomddoc/main.go` (`Schema SchemaCmd` field, `Info` help text),
  `cmd/gomddoc/serve_test.go` (delete the `writeEnvVarsHelp` test), `cmd/gomddoc/capabilities_test.go` (add
  `"schema"` back to `want`)

**Interfaces:**
- Consumes: `capabilitiesInput`, `capabilities.Describe`, `Report.JSON`, `config.JSONSchema`, `config.NewFromDir`,
  `config.IsGitURL`.
- Produces: `InfoCmd{Dir string; JSON bool; out io.Writer}`, `SchemaCmd{out io.Writer}`.

- [x] **Step 1: Write the failing tests** (`info_test.go`)

```go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/capabilities"
	"github.com/monolithiclab/gomddoc/internal/config"
)

func runInfo(t *testing.T, dir string, asJSON bool) string {
	t.Helper()
	var buf bytes.Buffer
	cmd := &InfoCmd{Dir: dir, JSON: asJSON, out: &buf}
	if err := cmd.Run(testModel(t)); err != nil {
		t.Fatalf("info must not fail, got %v", err)
	}
	return buf.String()
}

func siteWithConfig(t *testing.T, yml string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".gomddoc"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gomddoc", "config.yml"), []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInfo_JSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		dir        func(t *testing.T) string
		wantLoaded bool
		wantFound  bool
		wantErr    string // substring of load_error
		wantTheme  string
	}{
		{"site with config", func(t *testing.T) string { return siteWithConfig(t, "theme: {name: default}\n") }, true, true, "", "default"},
		{"no config file", func(t *testing.T) string { return t.TempDir() }, true, false, "", "default"},
		{"broken config", func(t *testing.T) string { return siteWithConfig(t, "nope: 1\n") }, false, true, "nope", "default"},
		{"git URL", func(*testing.T) string { return "git+https://example.com/repo.git" }, false, false, "not inspected", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := tt.dir(t)
			out := runInfo(t, dir, true)
			var r capabilities.Report
			if err := json.Unmarshal([]byte(out), &r); err != nil {
				t.Fatalf("info --json is not a Report: %v\n%s", err, out)
			}
			if r.Config.Loaded != tt.wantLoaded || r.Config.FileFound != tt.wantFound ||
				!strings.Contains(r.Config.LoadError, tt.wantErr) || r.Theme.Name != tt.wantTheme {
				t.Errorf("config=%+v theme=%q", r.Config, r.Theme.Name)
			}
			if tt.wantErr == "" && r.Config.LoadError != "" {
				t.Errorf("unexpected load_error %q", r.Config.LoadError)
			}
		})
	}
}

// TestInfo_JSONIsReportJSON pins principle 2: info --json prints Report.JSON
// verbatim, the same bytes the gomddoc://capabilities resource serves.
func TestInfo_JSONIsReportJSON(t *testing.T) {
	t.Parallel()
	dir := siteWithConfig(t, "theme: {name: default}\n")
	got := runInfo(t, dir, true)
	cfg, err := config.NewFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := capabilities.Describe(capabilitiesInput(testModel(t), dir, cfg, nil, os.DirFS(dir))).JSON()
	if strings.TrimSuffix(got, "\n") != string(want) {
		t.Errorf("info --json differs from Report.JSON()")
	}
}

func TestInfo_Human(t *testing.T) {
	t.Parallel()
	out := runInfo(t, siteWithConfig(t, "nope: 1\n"), false)
	for _, want := range []string{
		"Precedence: flag > env > file > default",
		"not loaded:",                    // the reason is shown
		"GOMDDOC_SITE_THEME_NAME",        // a setting's env var
		"site.meta.domain",               // a setting key
		"--domain",                       // a joined flag
		"toc",                            // a theme feature
		"02-configuration.md",            // a guide page
		"gomddoc info --json",            // pointer to the machine-readable form
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output lacks %q", want)
		}
	}
}
```

`schema_test.go`:

```go
func TestSchemaCmd(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := (&SchemaCmd{out: &buf}).Run(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSuffix(buf.String(), "\n") != string(config.JSONSchema()) {
		t.Error("schema output differs from config.JSONSchema()")
	}
}
```

- [x] **Step 2: Run to verify failure**

Run: `go test ./cmd/gomddoc/ -run 'TestInfo|TestSchemaCmd' -v`
Expected: FAIL — unknown fields `JSON`, `out`; undefined `SchemaCmd`

- [x] **Step 3: Implement `info.go`**

```go
// InfoCmd describes gomddoc and, when run in a site, that site's configuration.
type InfoCmd struct {
	Dir  string `arg:"" optional:"" default:"." env:"GOMDDOC_SERVER_DIR" help:"Content directory to inspect. Git URLs are not cloned; defaults are shown."`
	JSON bool   `name:"json" help:"Print the capabilities report as JSON (the same document as the MCP resource gomddoc://capabilities)."`

	out io.Writer // nil: os.Stdout
}

// Run prints the report. It never fails on a bad site: a config that does not
// load is the report's content, not the command's error.
func (i *InfoCmd) Run(app *kong.Application) error {
	var (
		cfg    *config.Config
		cfgErr error
		root   fs.FS
	)
	if config.IsGitURL(i.Dir) {
		cfgErr = errors.New("git sources are not inspected by info; showing defaults")
	} else {
		cfg, cfgErr = config.NewFromDir(i.Dir)
		root = os.DirFS(i.Dir)
	}
	report := capabilities.Describe(capabilitiesInput(app, i.Dir, cfg, cfgErr, root))

	w := i.out
	if w == nil {
		w = os.Stdout
	}
	if i.JSON {
		_, err := fmt.Fprintf(w, "%s\n", report.JSON())
		return err
	}
	return writeInfo(w, report)
}
```

`writeInfo(w io.Writer, r capabilities.Report) error` writes, in order, using `text/tabwriter` like the old code:

1. `gomddoc <version>`
2. `Config: <dir>/.gomddoc/config.yml (found|not found), loaded | not loaded: <reason>`
3. `Precedence: flag > env > file > default` (join `r.Config.Precedence` with `" > "`) and `r.Config.FileKeys`
4. `Settings:` grouped by the first two key segments, one row per setting: `key`, `env`, `flags`, `default`
   (or `default_note`), then the description on an indented second line
5. `Commands:` one line each, `name  help`
6. `Theme <name> (<source>): features …; vars …`
7. `Guide:` one line per page, `path  title`
8. Final line: `Machine-readable: gomddoc info --json · config schema: gomddoc schema`

Keep it one function per section if `writeInfo` passes ~60 lines. Delete `writeEnvVarsHelp` and its test.

- [x] **Step 4: Implement `schema.go`**

```go
// SchemaCmd prints the JSON Schema for .gomddoc/config.yml.
type SchemaCmd struct {
	out io.Writer // nil: os.Stdout
}

// Run writes config.JSONSchema.
func (s *SchemaCmd) Run() error {
	w := s.out
	if w == nil {
		w = os.Stdout
	}
	_, err := fmt.Fprintf(w, "%s\n", config.JSONSchema())
	return err
}
```

In `main.go`'s `CLI`: `Schema SchemaCmd `cmd:"" help:"Print the JSON Schema for .gomddoc/config.yml."``, and set
`Info`'s help to `"Describe gomddoc: settings, env vars, flags, theme features and guide pages (--json for
agents)."`.

- [x] **Step 5: Run to verify it passes**

Run: `go test ./cmd/gomddoc/ -v -run 'TestInfo|TestSchemaCmd|TestCommandsFromKong'`
Expected: PASS

- [x] **Step 6: Manual check** — `go run ./cmd/gomddoc info testsite`, `go run ./cmd/gomddoc info --json testsite |
  head -40`, `go run ./cmd/gomddoc schema | head`. Read the human output as an agent would; fix anything unclear.

- [x] **Step 7: `make ci`, then draft the commit message** (`feat(cli): info describes gomddoc from the
  capabilities report; add schema`).

---

### Task 10: MCP self-docs namespace — resources and `gomddoc_capabilities`

**Files:**
- Modify: `internal/mcp/server.go`, `internal/mcp/tools.go` (extract `pageHeader`)
- Create: `internal/mcp/selfdocs.go`, `internal/mcp/selfdocs_test.go`

**Interfaces:**
- Consumes: `capabilities.Report`, `capabilities.CapabilitiesURI/SchemaURI/GuideURIPrefix`, `config.JSONSchema`.
- Produces:

```go
type SelfDocs struct {
	Report capabilities.Report
	Guide  fs.FS
}
// ServerDeps gains: SelfDocs *SelfDocs
func pageHeader(title, description string, tags []string) string // "" when all empty
func (s *MCPServer) guidePage(p string) ([]byte, error)           // allowlisted read, used by Tasks 10–12
```

- [x] **Step 1: Write the failing tests** (`selfdocs_test.go`)

```go
package mcp

var selfDocsGuide = fstest.MapFS{
	"README.md":           {Data: []byte("---\ntitle: Guide\ndescription: Start here\n---\n# Guide\n\nRead the pages.\n")},
	"02-configuration.md": {Data: []byte("---\ntitle: Configuration\ndescription: Settings\n---\n# Configuration\n\n## Environment variables\n\nSet GOMDDOC_SITE_THEME_NAME.\n\n## Priority order\n\nFlags win.\n")},
	"05-theming-and-assets.md": {Data: []byte("---\ntitle: Theming\ndescription: Themes\n---\n# Theming\n\nThemes and vars.\n")},
}

func setupSelfDocs(t *testing.T, withSelfDocs bool) *testFixture {
	t.Helper()
	var sd *SelfDocs
	if withSelfDocs {
		sd = &SelfDocs{
			Report: capabilities.Describe(capabilities.Input{Version: "test", Config: config.New(), Guide: selfDocsGuide}),
			Guide:  selfDocsGuide,
		}
	}
	return connect(t, NewServer(ServerDeps{
		Provider: &testProvider{fsys: fstest.MapFS{}, defaultIndex: "README.md"},
		SelfDocs: sd,
	}))
}
```

Extract `connect(t, *MCPServer) *testFixture` from `setupTest` (the in-memory transport half) into
`server_test.go` and make `setupTest` call it — one canonical helper.

```go
func TestSelfDocs_RegisteredOnlyWhenSet(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{true, false} {
		f := setupSelfDocs(t, on)
		defer f.close(t)
		ctx := context.Background()

		tools, err := f.session.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		var toolNames []string
		for _, tl := range tools.Tools {
			toolNames = append(toolNames, tl.Name)
		}
		res, _ := f.session.ListResources(ctx, nil)
		var uris []string
		for _, r := range res.Resources {
			uris = append(uris, r.URI)
		}
		tmpl, _ := f.session.ListResourceTemplates(ctx, nil)
		var templates []string
		for _, r := range tmpl.ResourceTemplates {
			templates = append(templates, r.URITemplate)
		}
		prompts, _ := f.session.ListPrompts(ctx, nil)
		var promptNames []string
		for _, p := range prompts.Prompts {
			promptNames = append(promptNames, p.Name)
		}

		checks := []struct {
			list []string
			name string
		}{
			{toolNames, "gomddoc_capabilities"},
			{toolNames, "gomddoc_guide"}, // Task 11
			{uris, "gomddoc://capabilities"},
			{uris, "gomddoc://schema/config"},
			{templates, "gomddoc://guide/{+path}"},
			{promptNames, "learn_gomddoc"}, // Task 12
		}
		for _, c := range checks {
			if got := slices.Contains(c.list, c.name); got != on {
				t.Errorf("SelfDocs set=%v: %s registered=%v", on, c.name, got)
			}
		}
		// The site's own surface is unaffected either way.
		if !slices.Contains(toolNames, "search_docs") {
			t.Errorf("SelfDocs set=%v: search_docs missing", on)
		}
	}
}
```

Leave the `gomddoc_guide` and `learn_gomddoc` rows commented out until Tasks 11 and 12 uncomment them.

```go
func TestSelfDocs_CapabilitiesAndSchema(t *testing.T) {
	t.Parallel()
	f := setupSelfDocs(t, true)
	defer f.close(t)
	ctx := context.Background()
	want := string(f.server.deps.SelfDocs.Report.JSON())

	res, err := f.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "gomddoc://capabilities"})
	if err != nil || res.Contents[0].Text != want || res.Contents[0].MIMEType != "application/json" {
		t.Errorf("capabilities resource: err=%v mime=%q equal=%v", err, res.Contents[0].MIMEType, res.Contents[0].Text == want)
	}
	tool, err := f.session.CallTool(ctx, &mcp.CallToolParams{Name: "gomddoc_capabilities", Arguments: map[string]any{}})
	if err != nil || tool.Content[0].(*mcp.TextContent).Text != want {
		t.Errorf("gomddoc_capabilities differs from the resource (err=%v)", err)
	}
	schema, err := f.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "gomddoc://schema/config"})
	if err != nil || schema.Contents[0].Text != string(config.JSONSchema()) || schema.Contents[0].MIMEType != "application/schema+json" {
		t.Errorf("schema resource: err=%v", err)
	}
}

func TestSelfDocs_GuideResource(t *testing.T) {
	t.Parallel()
	f := setupSelfDocs(t, true)
	defer f.close(t)
	ctx := context.Background()

	res, err := f.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "gomddoc://guide/02-configuration.md"})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Contents[0].Text; !strings.HasPrefix(got, "# Configuration\n") || strings.Contains(got, "title: Configuration") {
		t.Errorf("guide page must be markdown with frontmatter stripped, got %q", got)
	}
	for _, bad := range []string{"gomddoc://guide/", "gomddoc://guide/nope.md", "gomddoc://guide/../go.mod",
		"gomddoc://guide//02-configuration.md"} {
		if _, err := f.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: bad}); err == nil {
			t.Errorf("%s: want not-found error", bad)
		}
	}
}
```

- [x] **Step 2: Run to verify failure**

Run: `go test ./internal/mcp/ -run TestSelfDocs -v`
Expected: FAIL — unknown field `SelfDocs`

- [x] **Step 3: Implement**

In `server.go`: add `SelfDocs` type and field (doc comment: "nil for the HTTP mount in serve: a public site's MCP
endpoint does not advertise its generator's manual"), and in `NewServer` after `registerPrompts()`:

```go
	if deps.SelfDocs != nil {
		s.registerSelfDocs()
	}
```

Update the package comment's first sentence to mention the optional gomddoc namespace.

In `tools.go`, extract the YAML-header body of `buildPageHeader` into `pageHeader(title, description string, tags
[]string) string` and make `buildPageHeader` call it.

`selfdocs.go`:

```go
func (s *MCPServer) registerSelfDocs() {
	s.server.AddResource(&mcp.Resource{
		URI: capabilities.CapabilitiesURI, Name: "gomddoc Capabilities", Title: "gomddoc Capabilities",
		Description: "What this gomddoc binary can do and how it is configured: every setting (config key, env var, flags, default), commands, frontmatter fields, the active theme's features, guide pages.",
		MIMEType: "application/json",
	}, s.handleCapabilitiesResource)
	s.server.AddResource(&mcp.Resource{
		URI: capabilities.SchemaURI, Name: "gomddoc Config Schema", Title: "gomddoc Config Schema",
		Description: "JSON Schema for .gomddoc/config.yml. Read before writing a config file.",
		MIMEType: "application/schema+json",
	}, s.handleSchemaResource)
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: capabilities.GuideURIPrefix + "{+path}", Name: "gomddoc Guide Page", Title: "gomddoc Guide Page",
		Description: "A page of gomddoc's own user guide, as markdown. List pages with gomddoc_guide.",
		MIMEType: "text/markdown",
	}, s.handleGuideResource)
	mcp.AddTool(s.server, &mcp.Tool{
		Name: "gomddoc_capabilities", Title: "gomddoc Capabilities",
		Description: "Describe gomddoc itself (not the site's content): settings with config keys, env vars, flags and defaults; config precedence; commands; frontmatter fields; the active theme's feature toggles; guide pages.",
		Annotations: readOnlyAnnotations,
	}, s.handleCapabilitiesTool)
}

// guidePage reads a guide page by path. The report's page list is the
// allowlist: a path that is not a listed page is not found, whatever fs.FS
// would make of it.
func (s *MCPServer) guidePage(p string) ([]byte, error) {
	if len(p) > maxArgLen || !slices.ContainsFunc(s.deps.SelfDocs.Report.Guide, func(g capabilities.GuidePage) bool { return g.Path == p }) {
		return nil, fs.ErrNotExist
	}
	return fs.ReadFile(s.deps.SelfDocs.Guide, p)
}
```

Handlers: `handleCapabilitiesResource` → `jsonResourceResult(uri, string(Report.JSON()))`; `handleSchemaResource` →
same shape with MIME `application/schema+json`; `handleGuideResource` → trim `GuideURIPrefix`, `guidePage`, on
error `mcp.ResourceNotFoundError(uri)`, else `text.StripFrontmatter`; `handleCapabilitiesTool` →
`textResult(string(Report.JSON()))`.

- [x] **Step 4: Run to verify it passes**

Run: `go test ./internal/mcp/ -v`
Expected: PASS (all existing tests too — `setupTest` now goes through `connect`)

- [x] **Step 5: `make ci`, then draft the commit message** (`feat(mcp): serve the capabilities report and config
  schema under gomddoc://`).

---

### Task 11: `gomddoc_guide` tool with a lazy guide index

**Files:**
- Modify: `internal/mcp/server.go` (`guideSearch` field), `internal/mcp/section.go` (`HeadingIDs`),
  `internal/mcp/selfdocs.go`, `internal/mcp/selfdocs_test.go`, `internal/mcp/section_test.go`

**Interfaces:**
- Consumes: `guidePage`, `pageHeader`, `ExtractSection`, `metadata.BuildIndex`, `search.BuildIndex`.
- Produces: `MCPServer.guideSearch func() (*search.Index, error)` (a `sync.OnceValues`);
  `func HeadingIDs(content []byte) []string`; `GuideInput{Query, Path, Section string; Limit int}`.

- [x] **Step 1: Write the failing tests**

`section_test.go`:

```go
func TestHeadingIDs(t *testing.T) {
	t.Parallel()
	got := HeadingIDs([]byte("---\ntitle: x\n---\n# Top\n\n## Environment variables\n\n```\n# not a heading? see note\n```\n### Priority order\n"))
	want := []string{"top", "environment-variables", "priority-order"}
	if !slices.Equal(got, want) {
		t.Errorf("HeadingIDs = %q, want %q", got, want)
	}
}
```

Note: check whether `ExtractSection` treats `#` inside fenced code as a heading. `HeadingIDs` must agree with
`ExtractSection` exactly — every ID it lists must be extractable. If `ExtractSection` does not skip fences, drop the
fence line from this fixture rather than making the two disagree (fixing fences in both is a separate change; note
it in REVIEW.md if it is broken).

`selfdocs_test.go`:

```go
func callGuide(t *testing.T, f *testFixture, args map[string]any) (string, bool) {
	t.Helper()
	res, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{Name: "gomddoc_guide", Arguments: args})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return res.Content[0].(*mcp.TextContent).Text, res.IsError
}

func TestGuideTool(t *testing.T) {
	t.Parallel()
	f := setupSelfDocs(t, true)
	defer f.close(t)

	t.Run("list", func(t *testing.T) {
		out, isErr := callGuide(t, f, map[string]any{})
		var got []capabilities.GuidePage
		if isErr || json.Unmarshal([]byte(out), &got) != nil {
			t.Fatalf("list: isErr=%v out=%s", isErr, out)
		}
		if !slices.Equal(got, f.server.deps.SelfDocs.Report.Guide) {
			t.Errorf("list = %+v", got)
		}
	})
	t.Run("search", func(t *testing.T) {
		// "priority" occurs only in 02-configuration.md. Avoid env var names as queries here:
		// the tokenizer may split on underscores, and "theme" also matches the theming page.
		out, isErr := callGuide(t, f, map[string]any{"query": "priority"})
		var hits []struct{ Path, Title, Snippet string }
		if isErr || json.Unmarshal([]byte(out), &hits) != nil {
			t.Fatalf("search: isErr=%v out=%s", isErr, out)
		}
		if len(hits) != 1 || hits[0].Path != "02-configuration.md" || hits[0].Title != "Configuration" {
			t.Errorf("hits = %+v", hits)
		}
	})
	t.Run("search no hits", func(t *testing.T) {
		out, isErr := callGuide(t, f, map[string]any{"query": "zzzqqq"})
		if isErr || out != "[]" {
			t.Errorf("no hits: isErr=%v out=%q, want []", isErr, out)
		}
	})
	t.Run("page", func(t *testing.T) {
		out, isErr := callGuide(t, f, map[string]any{"path": "02-configuration.md"})
		want := "---\ntitle: Configuration\ndescription: Settings\n---\n\n# Configuration\n"
		if isErr || !strings.HasPrefix(out, want) || strings.Count(out, "title: Configuration") != 1 {
			t.Errorf("page: isErr=%v out=%q", isErr, out)
		}
	})
	t.Run("section", func(t *testing.T) {
		out, isErr := callGuide(t, f, map[string]any{"path": "02-configuration.md", "section": "priority-order"})
		if isErr || out != "## Priority order\n\nFlags win." {
			t.Errorf("section: isErr=%v out=%q", isErr, out)
		}
	})

	errs := []struct {
		name string
		args map[string]any
		want []string // every substring must appear in the error text
	}{
		{"unknown page", map[string]any{"path": "nope.md"}, []string{"nope.md", "02-configuration.md", "README.md"}},
		{"traversal", map[string]any{"path": "../go.mod"}, []string{"02-configuration.md"}},
		{"leading slash", map[string]any{"path": "/02-configuration.md"}, []string{"02-configuration.md"}},
		{"directory", map[string]any{"path": "12-advanced"}, []string{"README.md"}},
		{"unknown section", map[string]any{"path": "02-configuration.md", "section": "nope"},
			[]string{"nope", "environment-variables", "priority-order"}},
		{"section without path", map[string]any{"section": "priority-order"}, []string{"path"}},
		{"query and path", map[string]any{"query": "x", "path": "README.md"}, []string{"query", "path"}},
		{"oversized", map[string]any{"path": strings.Repeat("a", maxArgLen+1)}, []string{"README.md"}},
	}
	for _, tt := range errs {
		t.Run(tt.name, func(t *testing.T) {
			out, isErr := callGuide(t, f, tt.args)
			if !isErr {
				t.Fatalf("want IsError, got %q", out)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("error %q lacks %q", out, w)
				}
			}
		})
	}
}

// TestGuideSearch_ColdConcurrent: the lazy guide index is used cold and
// concurrently by the first burst of calls; exactly one build must win.
func TestGuideSearch_ColdConcurrent(t *testing.T) {
	t.Parallel()
	s := NewServer(ServerDeps{
		Provider: &testProvider{fsys: fstest.MapFS{}, defaultIndex: "README.md"},
		SelfDocs: &SelfDocs{Report: capabilities.Describe(capabilities.Input{Guide: selfDocsGuide}), Guide: selfDocsGuide},
	})
	got := make([]*search.Index, 50)
	fanout.Run(50, func(i int) {
		idx, err := s.guideSearch()
		if err != nil {
			t.Errorf("guideSearch: %v", err)
		}
		got[i] = idx
	})
	for i, idx := range got {
		if idx == nil || idx != got[0] {
			t.Fatalf("call %d returned a different index (%p vs %p)", i, idx, got[0])
		}
	}
}

// TestGuideTool_RealEmbed: the shipped guide indexes and answers a real query.
func TestGuideTool_RealEmbed(t *testing.T) {
	t.Parallel()
	s := NewServer(ServerDeps{
		Provider: &testProvider{fsys: fstest.MapFS{}, defaultIndex: "README.md"},
		SelfDocs: &SelfDocs{Report: capabilities.Describe(capabilities.Input{Guide: docs.Guide}), Guide: docs.Guide},
	})
	idx, err := s.guideSearch()
	if err != nil {
		t.Fatal(err)
	}
	hits := idx.Search("environment variable naming precedence", 10)
	if !slices.ContainsFunc(hits, func(h search.SearchResult) bool { return strings.TrimPrefix(h.Path, "/") == "02-configuration.md" }) {
		t.Errorf("real guide search missed 02-configuration.md: %+v", hits)
	}
}
```

Add the `gomddoc_guide` row back into `TestSelfDocs_RegisteredOnlyWhenSet`.

- [x] **Step 2: Run to verify failure**

Run: `go test ./internal/mcp/ -run 'TestGuide|TestHeadingIDs' -v`
Expected: FAIL — `undefined: HeadingIDs`, unknown tool `gomddoc_guide`

- [x] **Step 3: Implement**

`section.go`:

```go
// HeadingIDs lists the anchor IDs ExtractSection accepts for content, in
// document order. It uses ExtractSection's own heading parser, so every ID it
// returns is extractable.
func HeadingIDs(content []byte) []string {
	var ids []string
	for line := range bytes.SplitSeq(text.StripFrontmatter(content), []byte("\n")) {
		if _, heading, ok := headingLevel(string(line)); ok {
			ids = append(ids, slugifyHeading(heading))
		}
	}
	return ids
}
```

`server.go` — field and construction:

```go
	// guideSearch builds the guide's search index on first use and shares it:
	// sync.OnceValues, so a cold burst of calls builds it once. nil unless
	// SelfDocs is set.
	guideSearch func() (*search.Index, error)
```

```go
	if deps.SelfDocs != nil {
		guide := deps.SelfDocs.Guide
		s.guideSearch = sync.OnceValues(func() (*search.Index, error) { return buildGuideSearch(guide) })
		s.registerSelfDocs()
	}
```

`selfdocs.go`:

```go
// GuideInput is the input for gomddoc_guide. The fields choose the mode.
type GuideInput struct {
	Query   string `json:"query,omitempty" jsonschema:"search gomddoc's guide; returns ranked pages"`
	Path    string `json:"path,omitempty" jsonschema:"guide page to read, e.g. 02-configuration.md (list pages by calling with no arguments)"`
	Section string `json:"section,omitempty" jsonschema:"heading anchor within path, e.g. environment-variables"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum search results (default 10, max 50)"`
}

func buildGuideSearch(guide fs.FS) (*search.Index, error) {
	ctx := context.Background()
	meta, err := metadata.BuildIndex(ctx, guide, nil)
	if err != nil {
		return nil, fmt.Errorf("guide metadata index: %w", err)
	}
	idx, err := search.BuildIndex(ctx, guide, meta, nil)
	if err != nil {
		return nil, fmt.Errorf("guide search index: %w", err)
	}
	return idx, nil
}

func errorResult(msg string) *mcp.CallToolResult {
	r := textResult(msg)
	r.IsError = true
	return r
}

func (s *MCPServer) handleGuideTool(_ context.Context, _ *mcp.CallToolRequest, in GuideInput) (*mcp.CallToolResult, any, error) {
	switch {
	case in.Query != "" && in.Path != "":
		return errorResult("Set either query (search) or path (read), not both."), nil, nil
	case in.Section != "" && in.Path == "":
		return errorResult("section needs a path: the page that contains it."), nil, nil
	case in.Query != "":
		return s.guideSearchResult(in.Query, in.Limit), nil, nil
	case in.Path != "":
		return s.guideReadResult(in.Path, in.Section), nil, nil
	default:
		return jsonTextResult(s.deps.SelfDocs.Report.Guide), nil, nil
	}
}
```

- `guideSearchResult`: cap `query` at `maxArgLen` (error if longer), clamp limit (default 10, max 50), call
  `s.guideSearch()` (return `errorResult(err.Error())` on failure), map results to
  `[]struct{Path, Title, Snippet string}` with JSON tags `path/title/snippet` and the leading `/` trimmed from
  `Path`, and return it through `jsonTextResult` (an empty slice marshals as `[]`, not `null`).
- `guideReadResult`: `guidePage(path)`; on error `errorResult` whose text is `Unknown guide page %q. Pages: a, b,
  c` (paths from `Report.Guide`, comma-joined). With a section: `ExtractSection`; on `ErrSectionNotFound`,
  `errorResult` listing `HeadingIDs(content)`. Without a section: look the page up in `Report.Guide` for
  title/description, return `pageHeader(title, description, nil) + string(text.StripFrontmatter(content))`.
- `jsonTextResult(v any) *mcp.CallToolResult`: `json.Marshal`, then `textResult`.
- Register the tool in `registerSelfDocs`:

```go
	mcp.AddTool(s.server, &mcp.Tool{
		Name:  "gomddoc_guide",
		Title: "gomddoc Guide",
		Description: "gomddoc's own user guide (how to configure, theme, deploy and use gomddoc — not the site's content). " +
			"No arguments: list pages. query: search. path: read a page. path + section: read one section.",
		Annotations: readOnlyAnnotations,
	}, s.handleGuideTool)
```

- [x] **Step 4: Run to verify it passes**

Run: `go test ./internal/mcp/ -race -v`
Expected: PASS

- [x] **Step 5: `make ci`, then draft the commit message** (`feat(mcp): add gomddoc_guide over the embedded guide`).

---

### Task 12: `learn_gomddoc` prompt

**Files:**
- Modify: `internal/mcp/selfdocs.go`, `internal/mcp/selfdocs_test.go`

**Interfaces:**
- Consumes: `SelfDocs.Report`, `guidePage("README.md")`.

- [x] **Step 1: Write the failing test**

```go
func TestLearnGomddocPrompt(t *testing.T) {
	t.Parallel()
	f := setupSelfDocs(t, true)
	defer f.close(t)
	res, err := f.session.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "learn_gomddoc"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Messages) != 1 {
		t.Fatalf("want one message, got %d", len(res.Messages))
	}
	msg := res.Messages[0].Content.(*mcp.TextContent).Text
	r := f.server.deps.SelfDocs.Report
	for _, want := range []string{
		"Read the pages.",                    // guide README body
		"flag > env > file > default",        // precedence
		r.Config.FileKeys,                     // file-key rule
		"gomddoc://schema/config",            // where the schema is
		"gomddoc_guide",                      // where depth is
		fmt.Sprintf("%d settings", len(r.Settings)),
		"serve",                              // a command
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Contains(msg, "title: Guide") {
		t.Error("README frontmatter must be stripped")
	}
}
```

`setupSelfDocs` builds its report with no commands; pass
`Commands: []capabilities.Command{{Name: "serve", Help: "Serve"}}` in its `capabilities.Input` so the command
assertion is falsifiable. Uncomment the `learn_gomddoc` row in `TestSelfDocs_RegisteredOnlyWhenSet`.

- [x] **Step 2: Run to verify failure**

Run: `go test ./internal/mcp/ -run TestLearnGomddocPrompt -v`
Expected: FAIL — prompt not found

- [x] **Step 3: Implement**

```go
	s.server.AddPrompt(&mcp.Prompt{
		Name:        "learn_gomddoc",
		Title:       "Learn gomddoc",
		Description: "Onboard to gomddoc itself: what it does, how configuration works, where to find details.",
	}, s.handleLearnGomddoc)
```

`handleLearnGomddoc` builds one user message with `strings.Builder`:

1. `"You are working with gomddoc <version>. Its guide's overview follows.\n\n"` + README body (frontmatter
   stripped; if `guidePage("README.md")` fails, omit the section — the rest still stands).
2. `"## How configuration works\n"`: `"Precedence: " + strings.Join(Precedence, " > ")`, the `FileKeys` note, and
   `"%d settings are documented in gomddoc://capabilities (key, env var, flags, default)."`.
3. `"## Commands\n"`: one `- name: help` line per command.
4. `"## Active theme\n"`: name, source, features comma-joined.
5. `"## How to proceed\n"`: read `gomddoc://schema/config` before writing `.gomddoc/config.yml`; use
   `gomddoc_guide` (query, or path + section) for anything not covered here; put secrets and deployment-specific
   values (ports, domain, auth file) in env vars or flags, site identity (title, theme, exclude) in config.yml;
   after writing a config, run `gomddoc info` to see whether it loads.

- [x] **Step 4: Run to verify it passes**

Run: `go test ./internal/mcp/ -v`
Expected: PASS

- [x] **Step 5: `make ci`, then draft the commit message** (`feat(mcp): add the learn_gomddoc onboarding prompt`).

---

### Task 13: Wire self-docs into `gomddoc mcp`

**Files:**
- Modify: `cmd/gomddoc/mcp.go`, `cmd/gomddoc/mcp_test.go`, `cmd/gomddoc/pipeline.go` (comment only)

**Interfaces:**
- Consumes: `capabilitiesInput`, `capabilities.Describe`, `mcp.SelfDocs`, `docs.Guide`.

- [x] **Step 1: Write the failing test** — read `TestMCPCmd_Run_ServesConfiguredContentOverStdio` first and follow
  its harness exactly. Add assertions (or a sibling test using the same harness) that over stdio:
  - `ListTools` includes `gomddoc_capabilities` and `gomddoc_guide`;
  - reading `gomddoc://capabilities` returns a `capabilities.Report` whose `Config.Dir` is the test's directory
    and `Theme.Source` is `"template-scan"` (the embedded default theme was scanned through the overlay).

  And in the `serve` path, assert the opposite: find the existing test that exercises the `/mcp` HTTP handler
  (`grep -rn "MCPHandler\|/mcp" cmd/gomddoc/*_test.go internal/server/*_test.go`) and add a `tools/list`
  assertion that `gomddoc_capabilities` is **absent**. If no such test exists, add one that builds
  `setupServer` for a temp site, POSTs an MCP `initialize` + `tools/list` to `/mcp`, and checks the tool list.

- [x] **Step 2: Run to verify failure**

Run: `go test ./cmd/gomddoc/ -run TestMCPCmd -v`
Expected: FAIL — `gomddoc_capabilities` not listed

- [x] **Step 3: Implement** — `MCPCmd.Run(app *kong.Application) error`; after the pipeline is set up:

```go
	contentRoot, err := prov.RootFS(context.Background())
	if err != nil {
		return fmt.Errorf("content root: %w", err)
	}
	report := capabilities.Describe(capabilitiesInput(app, m.Dir, cfg, nil, contentRoot))

	mcpServer := mcp.NewServer(mcp.ServerDeps{
		// … existing fields …
		SelfDocs: &mcp.SelfDocs{Report: report, Guide: docs.Guide},
	})
```

In `pipeline.go` at the `mcp.NewServer` call (around line 450), add a one-line comment: `SelfDocs stays nil: the
gomddoc:// namespace is for the operator's agent over stdio, not the site's readers.` Update `MCPCmd`'s help text:
`"Start MCP server for AI model integration (stdio). Also serves gomddoc's own guide and capabilities under
gomddoc://."`

If a test calls `(&MCPCmd{…}).Run()` with no arguments, pass `testModel(t)`.

- [x] **Step 4: Run to verify it passes**

Run: `go test ./cmd/gomddoc/ -race -v -run 'TestMCPCmd|MCP'`
Expected: PASS

- [x] **Step 5: Manual check** with an MCP client (Claude Code: `claude mcp add gomddoc-test -- go run
  ./cmd/gomddoc mcp testsite`, or the MCP Inspector): call `gomddoc_guide` with no args, a query, a path + section;
  read `gomddoc://capabilities`; get `learn_gomddoc`. Confirm an agent could write a valid `config.yml` from them.

- [x] **Step 6: `make ci`, then draft the commit message** (`feat(mcp): serve gomddoc's self-docs from gomddoc
  mcp`).

---

### Task 14: Documentation

**Files:**
- Modify: `docs/guide/04-mcp.md`, `docs/guide/02-configuration.md`, `docs/architecture.md`, `docs/decisions.md`,
  `docs/roadmap.md`, `REVIEW.md`, `../gomddoc-website/docs/configuration.md`

- [x] **Step 1: `04-mcp.md`** — add a `## Self-Documentation` section: the namespace table (3 resources,
  2 tools, 1 prompt, as in the spec's §6 table), the `gomddoc_guide` modes, why it is stdio-only, and one
  example agent flow (`learn_gomddoc` → `gomddoc://schema/config` → write config → `gomddoc info`). Update the
  page's tool/resource count wherever it states one.

- [x] **Step 2: `02-configuration.md`** — rewrite the `### info` section for `info [DIR] [--json]`; add
  `### schema`; add a short "Machine-readable configuration" paragraph near Priority Order pointing to
  `gomddoc schema`, `gomddoc info --json` and the MCP resources. Keep `TestGuide_*` green.

- [x] **Step 3: `architecture.md`** — add a "Self-description" subsection: the data-flow diagram from the spec, the
  one-source rule (struct tags), and where each consumer sits.

- [x] **Step 4: `decisions.md`** — three entries in the file's existing format: (1) config facts on struct tags,
  alternatives hand-written table / `go generate`; (2) gomddoc namespace on stdio only, alternative both
  transports; (3) theme facts by template scan until the manifest design settles, link the parked plan. Also note
  the deviations listed at the top of this plan (dropped `enum`/`min`, `EnvVars` deleted, `setting:` Kong tag).

- [x] **Step 5: `roadmap.md`** — tick the three "Self-Documentation via MCP" items (noting `mcp` always serves
  the guide alongside content rather than only when no directory is given), add two open items for sub-spec 2
  (`gomddoc doctor`) and sub-spec 3 (`gomddoc help <topic>`), add a "Theme manifest — parked" item linking
  `docs/plans/2026-09-23-theme-manifest-parked.md`, and update the Tier 3 priority list.

- [x] **Step 6: `REVIEW.md`** — record, as an open LOW issue: env var names for the same setting differ between
  flags and config (`GOMDDOC_DOMAIN` vs `GOMDDOC_SITE_META_DOMAIN`; preview's `GOMDDOC_DIR_INDEX` vs
  `GOMDDOC_SITE_DIR_INDEX`). The `setting:` tag makes the report correct; unifying the names is a separate,
  user-visible change.

- [x] **Step 7: gomddoc-website `docs/configuration.md`** — add a short section pointing to `gomddoc schema` (for
  editor validation) and `gomddoc info --json`. This is a separate repository: draft its commit message separately.

- [x] **Step 8: `make ci`, then draft the commit messages** (`docs: document gomddoc's self-description surface`;
  website: `docs(configuration): point to gomddoc schema and info --json`).

## Divergences from implementation

The plan's own "Deviations from the spec" section describes the shipped design. Later changes:

- Task 6: `ThemeFeatures` scans the templates the renderer resolves (inherited default partials, site partials,
  default-layout fallback) and reports an uninstalled theme as `fallback-default` (`f1173ea`).
- Task 7: the report gained `config.dir`, `config.file_status` (`442424c`) and `guide_error`; guide entries carry a
  `topic` name (`2b05595`).
- Task 11: the MCP-owned guide index, page allowlist and search moved to `internal/guide` (`2b05595`), shared with
  `gomddoc help` (`37250b5`). Section extraction moved from `internal/mcp` to `internal/text` (`e6f0ce5`).
- Task 13: `mcp.SelfDocs` later gained a `Doctor` function for the `gomddoc_doctor` tool (`f5a9426`).
