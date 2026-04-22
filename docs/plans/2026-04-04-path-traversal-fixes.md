# Path Traversal Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent path traversal in build output writes and template asset reads.

**Architecture:** Add a containment check in `writeOutputFile` that resolves both paths to absolute and verifies the output stays under the output directory. Add `fs.ValidPath` validation in `readAsset`.

**Tech Stack:** Go stdlib (`path/filepath`, `io/fs`)

---

### Task 1: Add path containment check to `writeOutputFile`

**Files:**
- Modify: `cmd/gomddoc/build.go:531-545`
- Modify: `cmd/gomddoc/build_test.go`

- [ ] **Step 1: Add test for path traversal rejection**

Add to `cmd/gomddoc/build_test.go` after `TestWriteOutputFile_ReadOnlyDir`:

```go
func TestWriteOutputFile_PathTraversal(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}

	tests := []struct {
		name    string
		relPath string
	}{
		{"dot-dot prefix", "../escape.html"},
		{"nested dot-dot", "sub/../../escape.html"},
		{"absolute path", "/etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := b.writeOutputFile(tt.relPath, []byte("pwned"))
			if err == nil {
				t.Error("writeOutputFile should reject path traversal")
			}
			if !strings.Contains(err.Error(), "outside output directory") {
				t.Errorf("error = %q, want substring %q", err, "outside output directory")
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/gomddoc/ -run TestWriteOutputFile_PathTraversal -v`
Expected: FAIL (no containment check yet)

- [ ] **Step 3: Add containment check to `writeOutputFile`**

Replace the `writeOutputFile` method in `cmd/gomddoc/build.go` (lines 531-545):

```go
// writeOutputFile writes content to a file in the output directory, creating parent directories as needed.
func (b *BuildCmd) writeOutputFile(relPath string, content []byte) error {
	outPath := filepath.Join(b.Output, relPath)

	// Resolve to absolute paths and verify containment.
	absOutput, err := filepath.Abs(b.Output)
	if err != nil {
		return fmt.Errorf("resolve output directory: %w", err)
	}
	absOut, err := filepath.Abs(outPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if !strings.HasPrefix(absOut, absOutput+string(filepath.Separator)) && absOut != absOutput {
		return fmt.Errorf("path %q escapes outside output directory", relPath)
	}

	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(outPath, content, 0644); err != nil { // #nosec G306
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/gomddoc/ -run TestWriteOutputFile -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/gomddoc/build.go cmd/gomddoc/build_test.go
git commit -m "Add path containment check to writeOutputFile

Resolves both output dir and target path to absolute paths and verifies
the target stays under the output directory. Prevents redirect_from
path traversal and hardens all other callers."
```

---

### Task 2: Add `fs.ValidPath` check to `readAsset`

**Files:**
- Modify: `internal/template/inline_asset.go:12-20`
- Modify: `internal/template/inline_asset_test.go`

- [ ] **Step 1: Add test for path traversal rejection**

Add to `internal/template/inline_asset_test.go`:

```go
func TestReadAsset_PathTraversal(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte("ok")},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	tests := []struct {
		name string
		path string
	}{
		{"dot-dot", "../../../etc/passwd"},
		{"absolute", "/etc/passwd"},
		{"dot prefix", "./test.js"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := r.readAsset(tt.path)
			if err == nil {
				t.Errorf("readAsset(%q) should reject invalid path", tt.path)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/template/ -run TestReadAsset_PathTraversal -v`
Expected: FAIL for at least one case (fs.ReadFile may already reject some, but not all)

- [ ] **Step 3: Add `fs.ValidPath` check to `readAsset`**

Replace `readAsset` in `internal/template/inline_asset.go` (lines 10-21):

```go
// readAsset searches for a named asset in the theme directory first,
// then falls back to the shared assets directory (overlay semantics).
func (h *HTMLRenderer) readAsset(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, fmt.Errorf("invalid asset path %q", name)
	}
	themeDir := path.Join("assets", "themes", h.siteConfig.Theme.Name)
	for _, dir := range []string{themeDir, "assets/shared"} {
		data, err := fs.ReadFile(h.assetsFS, path.Join(dir, name))
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("asset %q not found in theme or shared", name)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/template/ -v`
Expected: all PASS

- [ ] **Step 5: Run full CI**

Run: `make ci`
Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add internal/template/inline_asset.go internal/template/inline_asset_test.go
git commit -m "Add fs.ValidPath check to readAsset

Validates asset name before constructing path, preventing traversal
outside the expected asset directories."
```
