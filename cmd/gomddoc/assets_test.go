package main

import (
	"io/fs"
	"path"
	"regexp"
	"testing"
)

var (
	// customElements.define("gmd-color-chip", GmdColorChip);
	reElementDefine = regexp.MustCompile(`customElements\.define\(\s*["']([a-z0-9-]+)["']`)
	// gmd-color-chip:not(:defined) { … } — a custom element name always carries a
	// hyphen, and the left edge is anchored so `.some-class:not(:defined)` and
	// `#some-id:not(:defined)` are not read as element names.
	reNotDefinedSelector = regexp.MustCompile(`(?m)(?:^|[^.#\w-])([a-z][a-z0-9]*(?:-[a-z0-9]+)+):not\(\s*:defined\s*\)`)
)

// TestBundledThemes_NotDefinedSelectorsNameARealElement guards the failure mode
// that shipped in all seven external themes: a `:not(:defined)` anti-FOUC rule
// written against `color-chip`, when the component registers itself as
// `gmd-color-chip`. The selector matches nothing, the rule is dead, and no
// build, lint or render test notices — the page just flashes unstyled.
//
// It reads the embedded FS, not the overlay: a site's own
// `.gomddoc/assets/.../head.html.tmpl` legitimately shadows the bundled one, and
// that content is author-controlled with no build-time moment to check it.
func TestBundledThemes_NotDefinedSelectorsNameARealElement(t *testing.T) {
	t.Parallel()

	// A theme may ship its own .mjs beside the shared ones (inlineJSAsset
	// resolves theme-first, then shared), so both halves come from one walk over
	// everything embedded rather than from a fixed pair of directories.
	defined := make(map[string]bool)
	type selector struct {
		file, element string
	}
	var selectors []selector

	err := fs.WalkDir(embeddedAssets, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// Skips the theme screenshots, which outweigh every text file combined.
		switch path.Ext(p) {
		case ".mjs", ".tmpl", ".css":
		default:
			return nil
		}
		src, err := fs.ReadFile(embeddedAssets, p)
		if err != nil {
			return err
		}
		for _, m := range reElementDefine.FindAllSubmatch(src, -1) {
			defined[string(m[1])] = true
		}
		for _, m := range reNotDefinedSelector.FindAllSubmatch(src, -1) {
			selectors = append(selectors, selector{file: p, element: string(m[1])})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir() error = %v", err)
	}

	// Both counters keep the loop below from passing on an empty corpus — if
	// either regex stops matching what it used to, that is the bug, not a pass.
	if len(defined) == 0 {
		t.Fatal("no customElements.define() found in the embedded assets")
	}
	if len(selectors) == 0 {
		t.Fatal("no :not(:defined) custom-element selector found in the bundled themes")
	}

	for _, s := range selectors {
		if !defined[s.element] {
			t.Errorf("%s: selector %q:not(:defined) names an element no component defines (defined: %v)", s.file, s.element, defined)
		}
	}
}
