# Path Traversal Fixes

**Date:** 2026-04-04
**Status:** Approved
**Addresses:** REVIEW.md HIGH: `redirect_from` path traversal, MEDIUM: `writeOutputFile` no guard,
LOW: `readAsset` path traversal

## Problem

`writeOutputFile` joins `b.Output` with a caller-supplied `relPath` using `filepath.Join` but never
verifies the result stays under `b.Output`. A `relPath` containing `..` segments escapes the output
directory. This affects all callers, most critically `generateRedirectFiles` where `relPath` comes
from user-authored `redirect_from` frontmatter.

Separately, `readAsset` joins a caller-supplied `name` with a directory prefix but does not validate
the path before calling `fs.ReadFile`.

## Solution

### `writeOutputFile` containment check

After computing `outPath := filepath.Join(b.Output, relPath)`, resolve both `b.Output` and `outPath`
to absolute paths and verify `outPath` is under `b.Output` using `strings.HasPrefix` on the cleaned
absolute paths. Return an error if the path escapes.

### `readAsset` validation

Before calling `fs.ReadFile`, validate the joined path with `fs.ValidPath`. Return an error if
invalid.

## Files Changed

- `cmd/gomddoc/build.go` -- add containment check in `writeOutputFile`
- `cmd/gomddoc/build_test.go` -- add test for path traversal rejection
- `internal/template/inline_asset.go` -- add `fs.ValidPath` check in `readAsset`
- `internal/template/inline_asset_test.go` -- add test for traversal rejection
