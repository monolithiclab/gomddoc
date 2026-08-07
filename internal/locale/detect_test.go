package locale

import (
	"testing"
	"testing/fstest"
)

func TestIsLanguageDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected bool
	}{
		{"en-US", true},
		{"fr-FR", true},
		{"pt-BR", true},
		{"zh-CN", true},
		// Beyond ll-CC: script subtag, UN M.49 region, and both at once.
		{"zh-Hans", true},
		{"es-419", true},
		{"sr-Latn-RS", true},
		{"zh-Hant-TW", true},
		// A bare primary subtag is refused however valid, because these are
		// ordinary directory names that language.Parse happens to accept.
		{"en", false},
		{"doc", false},
		{"api", false},
		{"css", false},
		{"bin", false},
		{"is", false},
		// Well-formed but not a language: the old check passed these and
		// invented a pipeline for them.
		{"zz-ZZ", false},
		{"xx-XX", false},
		// Three-letter primary subtags are valid and now pass: "abc" is
		// Ambala Ayta. ll-CC rejected it on length alone.
		{"abc-DE", true},
		// Non-canonical spellings, which would otherwise split one language
		// across two directories.
		{"EN-US", false},
		{"fr-fr", false},
		{"zh-hans", false},
		// language.Parse accepts a variant, extension or privateuse subtag and
		// returns the tag verbatim, so a bare tag.String() == name round-trip
		// lets all four of these through; only recomposing from
		// base/script/region drops them.
		{"en-US-x-foo", false},
		{"en-US-u-co-phonebk", false},
		{"ca-ES-valencia", false},
		{"de-CH-1901", false},
		// The same shape without a region, and the one that matters in
		// practice: Parse reads "is-a-test" as Icelandic plus an extension
		// singleton, and an extension is not a region, so the bare-subtag rule
		// above is what keeps it out.
		{"is-a-test", false},
		// Hyphenated content directories.
		{"getting-started", false},
		{"my-project", false},
		{"en-user-guide", false},
		{"english", false},
		{"en_US", false},
		{"ab-DEF", false},
		{".hidden", false},
		{"docs", false},
		{"_assets", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isLanguageDir(tt.name); got != tt.expected {
				t.Errorf("isLanguageDir(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestDetectLanguages(t *testing.T) {
	t.Parallel()

	contentFS := fstest.MapFS{
		"guide.md":       &fstest.MapFile{Data: []byte("hello")},
		"fr-FR/guide.md": &fstest.MapFile{Data: []byte("bonjour")},
		"es-ES/guide.md": &fstest.MapFile{Data: []byte("hola")},
		"docs/readme.md": &fstest.MapFile{Data: []byte("docs")},
	}

	langs := DetectLanguages(contentFS)
	if len(langs) != 2 {
		t.Fatalf("got %d languages, want 2: %v", len(langs), langs)
	}

	found := make(map[string]bool)
	for _, l := range langs {
		found[l] = true
	}
	if !found["fr-FR"] || !found["es-ES"] {
		t.Errorf("expected fr-FR and es-ES, got %v", langs)
	}
}
