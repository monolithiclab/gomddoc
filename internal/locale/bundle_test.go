package locale

import (
	"testing"
	"testing/fstest"
)

func TestBundle_T(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		lang     string
		key      string
		expected string
	}{
		{
			name:     "exact match",
			lang:     "fr-FR",
			key:      "toc_title",
			expected: "Sur cette page",
		},
		{
			name:     "fallback to default language",
			lang:     "fr-FR",
			key:      "missing_in_french",
			expected: "default value",
		},
		{
			name:     "fallback to key when not in any language",
			lang:     "fr-FR",
			key:      "totally_unknown",
			expected: "totally_unknown",
		},
		{
			name:     "unknown language falls back to default",
			lang:     "ja-JP",
			key:      "toc_title",
			expected: "On this page",
		},
		{
			name:     "default language exact match",
			lang:     "en-US",
			key:      "toc_title",
			expected: "On this page",
		},
	}

	b := &Bundle{
		defaultLang: "en-US",
		strings: map[string]map[string]string{
			"en-US": {
				"toc_title":         "On this page",
				"missing_in_french": "default value",
			},
			"fr-FR": {
				"toc_title": "Sur cette page",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := b.T(tt.lang, tt.key)
			if got != tt.expected {
				t.Errorf("T(%q, %q) = %q, want %q", tt.lang, tt.key, got, tt.expected)
			}
		})
	}
}

func TestLoadBundle(t *testing.T) {
	t.Parallel()

	localeFS := fstest.MapFS{
		"locales/en-US.yml": &fstest.MapFile{
			Data: []byte("language_name: English\ntoc_title: \"On this page\"\n"),
		},
		"locales/fr-FR.yml": &fstest.MapFile{
			Data: []byte("language_name: Français\ntoc_title: \"Sur cette page\"\n"),
		},
	}

	b, err := LoadBundle("en-US", localeFS, "locales")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	if got := b.T("en-US", "toc_title"); got != "On this page" {
		t.Errorf("got %q, want %q", got, "On this page")
	}
	if got := b.T("fr-FR", "toc_title"); got != "Sur cette page" {
		t.Errorf("got %q, want %q", got, "Sur cette page")
	}
	if got := b.T("fr-FR", "language_name"); got != "Français" {
		t.Errorf("got %q, want %q", got, "Français")
	}
}

func TestLoadBundle_MergeLayers(t *testing.T) {
	t.Parallel()

	base := fstest.MapFS{
		"locales/en-US.yml": &fstest.MapFile{
			Data: []byte("toc_title: \"On this page\"\nedit_page: \"Edit this page\"\n"),
		},
	}
	override := fstest.MapFS{
		"locales/en-US.yml": &fstest.MapFile{
			Data: []byte("edit_page: \"Edit this page on GitHub\"\n"),
		},
	}

	b, err := LoadBundle("en-US", base, "locales")
	if err != nil {
		t.Fatalf("LoadBundle base: %v", err)
	}
	if err := b.MergeFrom(override, "locales"); err != nil {
		t.Fatalf("MergeFrom: %v", err)
	}

	if got := b.T("en-US", "edit_page"); got != "Edit this page on GitHub" {
		t.Errorf("got %q, want %q", got, "Edit this page on GitHub")
	}
	if got := b.T("en-US", "toc_title"); got != "On this page" {
		t.Errorf("got %q, want %q", got, "On this page")
	}
}

func TestBundle_Languages(t *testing.T) {
	t.Parallel()

	b := &Bundle{
		defaultLang: "en-US",
		strings: map[string]map[string]string{
			"en-US": {"language_name": "English"},
			"fr-FR": {"language_name": "Français"},
		},
	}

	langs := b.Languages()
	if len(langs) != 2 {
		t.Fatalf("got %d languages, want 2", len(langs))
	}
}

func TestBundle_LanguageName(t *testing.T) {
	t.Parallel()

	b := &Bundle{
		defaultLang: "en-US",
		strings: map[string]map[string]string{
			"en-US": {"language_name": "English"},
			"fr-FR": {},
		},
	}

	if got := b.LanguageName("en-US"); got != "English" {
		t.Errorf("got %q, want %q", got, "English")
	}
	if got := b.LanguageName("fr-FR"); got != "fr-FR" {
		t.Errorf("got %q, want %q", got, "fr-FR")
	}
}

func TestLoadBundle_TagKeys(t *testing.T) {
	t.Parallel()

	localeFS := fstest.MapFS{
		"locales/en-US.yml": &fstest.MapFile{
			Data: []byte("tags_title: Tags\ntags_index_title: \"All tags\"\ntags_tagged_as: \"Pages tagged %s\"\ntags_empty: \"No pages tagged %s\"\n"),
		},
	}

	b, err := LoadBundle("en-US", localeFS, "locales")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	// Verify the four new tag-related keys are present and have non-empty values.
	keys := []struct {
		key      string
		expected string
	}{
		{"tags_title", "Tags"},
		{"tags_index_title", "All tags"},
		{"tags_tagged_as", "Pages tagged %s"},
		{"tags_empty", "No pages tagged %s"},
	}

	for _, tt := range keys {
		got := b.T("en-US", tt.key)
		if got != tt.expected {
			t.Errorf("T(%q, %q) = %q, want %q", "en-US", tt.key, got, tt.expected)
		}
	}
}

func TestLoadBundle_SeeAlsoKey(t *testing.T) {
	t.Parallel()

	localeFS := fstest.MapFS{
		"locales/en-US.yml": &fstest.MapFile{
			Data: []byte("language_name: English\nsee_also: \"See also\"\n"),
		},
	}

	b, err := LoadBundle("en-US", localeFS, "locales")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	got := b.T("en-US", "see_also")
	if got == "" || got == "see_also" {
		t.Errorf("bundle.T(\"see_also\") returned %q; expected a localized string", got)
	}
}
