---
title: "Doctor"
description: "Check configuration and content with gomddoc doctor or gomddoc_doctor: every problem, with location and fix."
tags: ["doctor", "configuration", "mcp", "ai"]
---

# Doctor

`gomddoc doctor` checks a site and reports **every** problem with its configuration, and the cheap-to-detect problems
in its content, each with a file, a line and a concrete fix. It is built for agents first: after editing
`.gomddoc/config.yml` or a page, an agent runs `gomddoc doctor --json` (or calls the `gomddoc_doctor` MCP tool) and
gets a machine-readable list of what is wrong.

Doctor reports what `serve` would hit: the checks that have a runtime counterpart — config loading and validation,
clean-URL collisions, `redirect_from` handling — are the same code `serve` runs, which reports its findings instead of
only logging them. `serve` logs the same findings at startup, with the same `code`.

## Usage

```bash
gomddoc doctor [DIR] [--json] [--strict] [-v|--verbose]
```

| Flag/Arg | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `DIR` (arg) | `GOMDDOC_SERVER_DIR` | `.` | Site directory or Git URL (cloned like `serve`) |
| `--json` | | `false` | Print the report as JSON — the same document as `gomddoc_doctor` returns |
| `--strict` | | `false` | Exit non-zero on warnings too (for CI) |
| `-v`, `--verbose` | | `false` | Include `info` findings |
| `--git-key-file` | `GOMDDOC_SERVER_GIT_SSH_KEY` | | SSH key for private Git repos |
| `--git-storage-dir` | `GOMDDOC_SERVER_GIT_STORAGE_DIR` | | Disk-based Git clone directory |

**Exit status:** `1` when there is any `error`, or any `warning` with `--strict`; `0` otherwise. `info` findings
never fail a run.

**Verbose:** `info` findings (missing descriptions, unused theme variables) are hidden unless `-v` is given. The
summary always counts them: `2 errors, 1 warning (3 info hidden, use -v)`. The JSON report follows the same rule, and
its `summary.info` is always the true count.

Human output, one finding per line with its fix beneath:

```text
error    .gomddoc/config.yml:3  config.unknown-key  unknown key `hightlighting` at the top level
         fix: did you mean `highlighting`?
warning  .gomddoc/config.yml:7  theme.unknown-feature  theme "default" never reads the feature toggle "serach"
         fix: did you mean `search`?
1 error, 1 warning (2 info hidden, use -v)
```

JSON output:

```json
{
  "target": "./docs",
  "summary": {"errors": 1, "warnings": 1, "info": 2, "info_hidden": true},
  "findings": [
    {"severity": "error", "code": "config.unknown-key", "file": ".gomddoc/config.yml", "line": 3,
     "key": "hightlighting", "message": "unknown key `hightlighting` at the top level",
     "fix": "did you mean `highlighting`?"}
  ]
}
```

`assumed` appears when `config.yml` could not be used and defaults stood in; `note` when a Git source was checked.

## MCP: `gomddoc_doctor`

`gomddoc mcp` (stdio) exposes the same check as the `gomddoc_doctor` tool, with an optional `verbose` argument. Every
call reloads the configuration and rebuilds the indexes, so an agent can edit and check again without restarting the
server. For a Git source the server's clone is checked. The tool is not on the HTTP endpoint `serve` mounts — see
[MCP Server](04-mcp.md#self-documentation).

## Finding codes

| Code | Severity | Meaning | Fix |
|------|----------|---------|-----|
| `config.parse-error` | error | `config.yml` is not valid YAML, or holds a second document after `---` | Fix the syntax at the reported line; merge the documents |
| `config.unknown-key` | error | A `config.yml` key gomddoc does not define — every one is reported, not only the first | "Did you mean" a close key; otherwise the valid keys at that level. A `site:` wrapper or a server setting in the file get their own fix |
| `config.wrong-type` | error | A value of the wrong type, e.g. `exclude: drafts/` instead of a list | Use the type `gomddoc schema` gives |
| `config.invalid-value` | error | A value gomddoc rejects: a domain with a scheme, a non-http(s) `edit_url`, a `strip_extensions` entry without a dot, a bad feature key | The validator's reason |
| `config.value-replaced` | warning | A value replaced by its default because it is out of range (HTTP timeouts, header size, empty theme name) | Set a value within the stated range |
| `env.invalid-value` | warning | A `GOMDDOC_*` duration, int or bool that does not parse — it is ignored | Set a valid value or unset it |
| `env.unknown` | warning | A `GOMDDOC_*` variable gomddoc does not read | The nearest real variable |
| `theme.not-installed` | warning | `theme.name` is not installed; the default theme renders | Install the theme under `.gomddoc/assets/themes/<name>/` |
| `theme.unknown-feature` | warning | A feature toggle (config or page frontmatter) the active theme never reads | "Did you mean" a toggle the theme reads |
| `theme.unknown-var` | info | A `theme.vars` key the theme's CSS never reads | The variables the theme reads |
| `content.frontmatter-invalid` | error | A page's frontmatter is not valid YAML — the page silently loses its title and tags | Fix the YAML between the `---` lines |
| `content.frontmatter-type` | error | A special frontmatter field of the wrong type: `tags: foo`, an unparseable `date`, a string `redirect_from`, non-boolean `features` | The expected shape, e.g. `tags: [a, b]` |
| `content.redirect-conflict` | error | Two pages claim one `redirect_from` source, or a source is itself a page (which makes that page unreachable) | Keep the source in one page; remove sources that name real pages |
| `content.path-collision` | warning | Two files claim one clean URL (`guide.md` and `guide.html`), or a file shadows a directory | Rename one of them |
| `content.tags-collision` | warning | Content at `tags.md` or `tags/`, shadowed by the auto-generated tag pages | Rename it |
| `content.missing-title` | info | A page with neither a `title` nor a `#` heading | Add `title:` to the frontmatter |
| `content.missing-description` | info | A page without a `description` | Add `description:` to the frontmatter |
| `exclude.matches-nothing` | warning | An `exclude` pattern that matches no file or directory — usually a typo that leaves content published | Check the pattern's form |
| `target.unreachable` | error | The directory or repository could not be opened; nothing else is checked | Check the path, URL or credentials |
| `target.read-error` | error | A file could not be read while checking, or the site could not be built | Check permissions; fix the other findings first |

Content checks walk every language directory's pages with that language's exclusions; a translated page's findings are
reported under its directory (`fr-FR/guide.md`). Broken links and anchors are not checked yet.
