# URL Extension Stripping

**Date**: 2026-04-04
**Status**: Draft

## Summary

Add support for serving content files at extensionless URLs. A file `docs/guide.md`
on disk becomes accessible at `/docs/guide` instead of `/docs/guide.md`. Requests to
the original extension-based URL get 301-redirected to the canonical extensionless URL.

## Configuration

New field in `SiteConfig`:

```go
StripExtensions []string `yaml:"strip_extensions"`
```

- **Default**: `[".md"]` (enabled by default)
- **Empty list** `[]` disables the feature
- **Order matters**: first extension in the list wins collisions

Config example:

```yaml
strip_extensions:
  - .md
  - .html
```

## Architecture: Resolution Table

A new package `internal/resolve` provides a `PathResolver` built at startup by walking
the provider's `RootFS`.

### Data structure

```go
type PathResolver struct {
    // extensionless path -> real file path
    // e.g., "docs/guide" -> "docs/guide.md"
    toReal map[string]string

    // real file path -> extensionless path (reverse)
    // e.g., "docs/guide.md" -> "docs/guide"
    toClean map[string]string
}
```

### Build algorithm

For each file discovered during the `RootFS` walk:

1. Check if its extension is in `strip_extensions`
2. Check if a renderer exists for that file's MIME type (only strip rendered content)
3. Compute the extensionless path by stripping the final extension
4. Collision checks (in priority order):
   - If extensionless path already claimed by a higher-priority extension: skip, log warning
   - If extensionless path matches an existing directory: file wins, log warning
5. Insert into both maps

### Rebuild

Git provider already has a refresh mechanism. The resolver rebuilds when content
changes. Filesystem provider builds once at startup (consistent with current
behavior).

### API

```go
// Resolve maps an extensionless path to the real file path.
func (r *PathResolver) Resolve(path string) (realPath string, found bool)

// CleanPath maps a real file path to its extensionless canonical path.
func (r *PathResolver) CleanPath(realPath string) (cleanPath string, found bool)
```

## Serve Mode Integration

### Redirect middleware

New middleware, placed after `ContentExclusion` and before the content handler.

When a request arrives with a strippable extension (e.g., `/docs/guide.md`), check
if the resolver maps that file to a clean path. If so, 301 redirect to `/docs/guide`.

### Handler path resolution

In `ServeContent`, when `provider.ReadFile()` fails for a path without an extension,
consult the resolver: `resolver.Resolve("docs/guide")` returns `"docs/guide.md"`.
Read the resolved real path from the provider.

`r.URL.Path` stays as-is (the clean URL) for template context, breadcrumbs, and
title derivation. Only the provider call uses the resolved real path.

### Navigation links

The navigation generator uses `CleanPath` to emit extensionless URLs (e.g.,
`/docs/guide` instead of `/docs/guide.md`).

### Sitemap, feed, MCP

Same treatment: use `CleanPath` when generating URLs for entries that have clean
paths.

## Build Mode Integration

When `strip_extensions` is active, the build command uses pretty URL output.

### Output rules

1. Files whose extension is in `strip_extensions` AND have a renderer:
   pretty URL output (`guide.md` -> `guide/index.html`)
2. Files named `index.md`: stay as `index.html` in the same directory
   (no double nesting `index/index.html`)
3. Default index files (e.g., `README.md`): already become `index.html`
   (unchanged)
4. Files not in `strip_extensions` or without a renderer: current behavior
   (copy as-is or render to `.html`)
5. Collision (file `guide.md` + directory `guide/`): the file's pretty
   URL output (`guide/index.html`) takes the slot. The directory's own
   default index (e.g., `guide/README.md`) falls back to
   `guide/README.html` instead of becoming `guide/index.html`.
   Warning logged.

### Redirect pages

Generate HTML meta-refresh redirect pages from `docs/guide.md` to `docs/guide`
(no trailing slash) so old extension-based links still work on static hosts.

Existing `redirect_from` frontmatter redirect pages also use clean paths.

### SEO files

URLs in generated sitemap, feed, and robots files use clean paths.

## Edge Cases

1. **`index.md`** -- never stripped to `index/index.html`. Stays `index.html`
   in its directory. `/docs/` serves it (unchanged).

2. **Default index (`README.md`)** -- already becomes `index.html` when no
   `index.md` exists. Not affected by `strip_extensions`.

3. **File + directory collision** (`guide.md` + `guide/`) -- file wins `/guide`.
   Directory reachable via `/guide/`. Warning logged at startup/build.

4. **Multi-extension collision** (`guide.md` + `guide.html`, both stripped) --
   first extension in `strip_extensions` wins. Losing file keeps its full
   extension URL. Warning logged.

5. **Multiple dots in filename** (`my.config.md`) -- only the last extension
   stripped: `/my.config`.

6. **Non-rendered files** (`image.jpg`, `data.csv`) -- extensions never stripped,
   regardless of `strip_extensions` list. Only files with a matching renderer
   are eligible.

7. **Empty `strip_extensions: []`** -- feature fully disabled, current behavior
   preserved.

8. **Trailing slashes** -- `/docs/guide/` is a directory request, `/docs/guide`
   is a file request (the stripped URL). No ambiguity because directory URLs
   use trailing slashes.

### Invariants

- Every file has exactly one canonical URL
- The resolver is the single source of truth for path mapping
- Navigation, sitemap, feed, MCP all derive URLs from the resolver
- Collisions are warned at startup/build time, never silently swallowed

## Testing Strategy

### Unit tests (`internal/resolve`)

- Basic mapping: extension stripped, reverse lookup works
- Collision detection: file/dir, multi-extension priority
- Index file handling: `index.md` and default index not double-nested
- Non-rendered files skipped
- Empty config disables feature
- Multiple dots in filenames

### Redirect middleware tests

- Requests with strippable extensions get 301
- Non-strippable extensions pass through
- Excluded paths not redirected

### Handler integration tests

- Extensionless paths resolve and serve correctly
- `r.URL.Path` preserved for template context

### Build command tests

- Pretty URL output structure (`guide.md` -> `guide/index.html`)
- `index.md` stays `index.html`
- Redirect pages generated (extension -> extensionless, no trailing slash)
- Collision cases produce warnings and correct output

### Navigation tests

- Nav links emit clean paths when resolver is active
- `markActive` matches against clean paths

### Integration tests

- Start server, request extensionless URL, verify content served
- Request with extension, verify 301 to clean URL
- Sitemap/feed URLs use clean paths
