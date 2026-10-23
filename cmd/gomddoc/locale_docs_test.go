package main

import (
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// i18nGuidePath is the guide page whose "Built-in Translation Keys" block
// claims to reproduce the shipped en-US.yml.
const i18nGuidePath = "../../docs/guide/13-internationalization.md"

// builtinLocalePath is the embedded baseline the guide is documenting.
const builtinLocalePath = "assets/locales/en-US.yml"

// TestBuiltinTranslationKeysAreDocumented pins the guide's key list against the
// file it transcribes. The list had drifted to 20 of 26 keys: nothing failed
// when tags_title, see_also and four siblings were added, because a YAML block
// inside a Markdown fence is not compiled, not parsed and not read by anything.
//
// Set equality both ways is the point. Asserting only "every shipped key is
// documented" passes a doc that also lists keys which no longer exist, which is
// the failure mode a translator notices last — they translate a key the
// templates never ask for and the string simply never appears.
func TestBuiltinTranslationKeysAreDocumented(t *testing.T) {
	t.Parallel()

	raw, err := embeddedAssets.ReadFile(builtinLocalePath)
	if err != nil {
		t.Fatalf("read %s: %v", builtinLocalePath, err)
	}
	var shipped map[string]string
	if err := yaml.Unmarshal(raw, &shipped); err != nil {
		t.Fatalf("parse %s: %v", builtinLocalePath, err)
	}

	documented := documentedLocaleKeys(t)

	shippedKeys := slices.Sorted(maps.Keys(shipped))
	if !slices.Equal(documented, shippedKeys) {
		t.Errorf("%s documents\n  %v\nbut %s ships\n  %v",
			i18nGuidePath, documented, builtinLocalePath, shippedKeys)
	}
}

// documentedLocaleKeys returns the sorted top-level keys of the first ```yaml
// fence following the "### Built-in Translation Keys" heading. It owns the read
// as well as the parse, so every message naming i18nGuidePath lives in one place.
//
// Only that fence is pinned. The page's other YAML block, the fr-FR example, is
// a fragment on purpose — locales merge per key — and the page now says so, which
// is the difference between an exemption and this helper's Cut taking the first
// match by accident.
func documentedLocaleKeys(t *testing.T) []string {
	t.Helper()

	guide, err := os.ReadFile(i18nGuidePath)
	if err != nil {
		t.Fatalf("read %s: %v", i18nGuidePath, err)
	}

	const heading = "### Built-in Translation Keys"
	_, after, found := strings.Cut(string(guide), heading)
	if !found {
		t.Fatalf("%s has no %q heading", i18nGuidePath, heading)
	}
	_, after, found = strings.Cut(after, "```yaml\n")
	if !found {
		t.Fatalf("%s has no yaml fence under %q", i18nGuidePath, heading)
	}
	block, _, found := strings.Cut(after, "```")
	if !found {
		t.Fatalf("%s has an unterminated yaml fence under %q", i18nGuidePath, heading)
	}

	var keys map[string]string
	if err := yaml.Unmarshal([]byte(block), &keys); err != nil {
		t.Fatalf("parse the yaml fence under %q: %v", heading, err)
	}
	return slices.Sorted(maps.Keys(keys))
}
