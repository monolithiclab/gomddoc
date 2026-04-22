# GitHub Actions CI

**Date**: 2026-04-10
**Status**: Approved

## Goal

Run `make lint test` on every push to validate code correctness. This is gomddoc's
first CI pipeline — keep it minimal and optimize later.

## Design

Single workflow file at `.github/workflows/ci.yml`.

### Trigger

`push` on all branches. No pull request trigger (pushes to PR branches already
trigger the workflow).

### Job

One job named `ci`, running on `ubuntu-latest`.

### Steps

1. **Checkout** — `actions/checkout@v4`
2. **Setup Go** — `actions/setup-go@v5` with `go-version-file: go.mod` (reads Go
   version from the module file, currently 1.26). Caching is enabled by default,
   keyed on `go.sum`. The Go module cache and build cache are automatically
   restored and saved.
3. **Lint and test** — `make lint test`. Runs `go vet`, `gofmt` check,
   `staticcheck`, `golangci-lint`, `gosec`, `gocritic`, then `go test -race -cover`.

### What is NOT in scope

- **`make codefix` and `make format`** — these modify code in place. CI should
  detect problems, not fix them.
- **Linter caching** — linters are installed via `go install` on each run (~30-60s
  overhead). Acceptable for now; can be optimized with pinned versions and
  `$GOPATH/bin` caching later.
- **Coverage thresholds** — tests report coverage but CI does not enforce a minimum.
  Can be added later.
- **Concurrency controls** — no `concurrency` group to cancel superseded runs.
  Can be added if queue buildup becomes a problem.
- **Artifacts** — no coverage report upload. Can be added later.
- **Multi-platform** — `ubuntu-latest` only. gomddoc is pure Go with no CGO or
  platform-specific dependencies.

## Workflow YAML

```yaml
name: CI

on:
  push:

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make lint test
```
