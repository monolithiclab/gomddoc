package locale

import (
	"testing"
	"testing/fstest"
)

func TestIsBCP47Dir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected bool
	}{
		{"en-US", true},
		{"fr-FR", true},
		{"pt-BR", true},
		{"zh-CN", true},
		{"en", false},
		{"english", false},
		{"EN-US", false},
		{"en_US", false},
		{"fr-fr", false},
		{"abc-DE", false},
		{"ab-DEF", false},
		{".hidden", false},
		{"docs", false},
		{"_assets", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsBCP47Dir(tt.name); got != tt.expected {
				t.Errorf("IsBCP47Dir(%q) = %v, want %v", tt.name, got, tt.expected)
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

func TestExtractLangFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		wantLang string
		wantRest string
	}{
		{"/fr-FR/guide", "fr-FR", "/guide"},
		{"/fr-FR/", "fr-FR", "/"},
		{"/fr-FR", "fr-FR", "/"},
		{"/guide", "", "/guide"},
		{"/docs/readme", "", "/docs/readme"},
		{"/", "", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			lang, rest := ExtractLangFromPath(tt.path)
			if lang != tt.wantLang || rest != tt.wantRest {
				t.Errorf("ExtractLangFromPath(%q) = (%q, %q), want (%q, %q)",
					tt.path, lang, rest, tt.wantLang, tt.wantRest)
			}
		})
	}
}
