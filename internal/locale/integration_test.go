package locale_test

import (
	"net/http/httptest"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/locale"
	"github.com/monolithiclab/gomddoc/internal/server"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
)

// TestSingleLanguageSiteUnaffected verifies that a site with no BCP 47
// directories behaves as a plain English site: no detected languages,
// T() returns English defaults, and Lang() uses the config default.
func TestSingleLanguageSiteUnaffected(t *testing.T) {
	t.Parallel()

	// Content filesystem with no BCP 47 directories.
	contentFS := fstest.MapFS{
		"guide.md":      &fstest.MapFile{Data: []byte("# Guide")},
		"docs/intro.md": &fstest.MapFile{Data: []byte("# Intro")},
	}

	// No languages should be detected.
	langs := locale.DetectLanguages(contentFS)
	if len(langs) != 0 {
		t.Errorf("DetectLanguages: got %v, want empty", langs)
	}

	// Load a bundle with English defaults.
	localeFS := fstest.MapFS{
		"locales/en-US.yml": &fstest.MapFile{
			Data: []byte("toc_title: \"On this page\"\nedit_page: \"Edit\"\n"),
		},
	}
	bundle, err := locale.LoadBundle("en-US", localeFS, "locales")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	// T() returns English strings even when called with default lang.
	if got := bundle.T("en-US", "toc_title"); got != "On this page" {
		t.Errorf("T(en-US, toc_title) = %q, want %q", got, "On this page")
	}

	// TemplateContext reports en-US from site config when no i18n is wired.
	tc := &tmpl.TemplateContext{
		Site: &config.SiteConfig{Language: "en-US"},
	}
	if got := tc.Lang(); got != "en-US" {
		t.Errorf("Lang() = %q, want %q", got, "en-US")
	}
}

// TestMultiLanguageDetection verifies that BCP 47 subdirectories are
// discovered and that the bundle serves different translations per language.
func TestMultiLanguageDetection(t *testing.T) {
	t.Parallel()

	contentFS := fstest.MapFS{
		"guide.md":        &fstest.MapFile{Data: []byte("# Guide")},
		"fr-FR/guide.md":  &fstest.MapFile{Data: []byte("# Guide FR")},
		"es-ES/guide.md":  &fstest.MapFile{Data: []byte("# Guía")},
		"docs/readme.md":  &fstest.MapFile{Data: []byte("# Docs")},
		"_assets/main.js": &fstest.MapFile{Data: []byte("// js")},
	}

	langs := locale.DetectLanguages(contentFS)
	if len(langs) != 2 {
		t.Fatalf("DetectLanguages: got %d languages %v, want 2", len(langs), langs)
	}

	found := make(map[string]bool)
	for _, l := range langs {
		found[l] = true
	}
	if !found["fr-FR"] || !found["es-ES"] {
		t.Errorf("expected fr-FR and es-ES, got %v", langs)
	}

	// Load a bundle with translations for each detected language.
	localeFS := fstest.MapFS{
		"locales/en-US.yml": &fstest.MapFile{
			Data: []byte("language_name: English\ntoc_title: \"On this page\"\n"),
		},
		"locales/fr-FR.yml": &fstest.MapFile{
			Data: []byte("language_name: Français\ntoc_title: \"Sur cette page\"\n"),
		},
		"locales/es-ES.yml": &fstest.MapFile{
			Data: []byte("language_name: Español\ntoc_title: \"En esta página\"\n"),
		},
	}

	bundle, err := locale.LoadBundle("en-US", localeFS, "locales")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	tests := []struct {
		lang, key, want string
	}{
		{"en-US", "toc_title", "On this page"},
		{"fr-FR", "toc_title", "Sur cette page"},
		{"es-ES", "toc_title", "En esta página"},
		{"fr-FR", "language_name", "Français"},
		{"es-ES", "language_name", "Español"},
	}
	for _, tt := range tests {
		if got := bundle.T(tt.lang, tt.key); got != tt.want {
			t.Errorf("T(%q, %q) = %q, want %q", tt.lang, tt.key, got, tt.want)
		}
	}
}

// TestAPILanguageResolution tests server.ResolveAPILanguage with query params,
// Accept-Language header, and fallback to default.
func TestAPILanguageResolution(t *testing.T) {
	t.Parallel()

	known := map[string]bool{
		"en-US": true,
		"fr-FR": true,
		"es-ES": true,
	}
	defaultLang := "en-US"

	tests := []struct {
		name       string
		query      string
		acceptLang string
		want       string
	}{
		{
			name:  "query param selects known language",
			query: "lang=fr-FR",
			want:  "fr-FR",
		},
		{
			name:  "query param with unknown language falls back",
			query: "lang=ja-JP",
			want:  "en-US",
		},
		{
			name:       "Accept-Language header used when no query param",
			acceptLang: "es-ES",
			want:       "es-ES",
		},
		{
			name:       "Accept-Language with quality values",
			acceptLang: "fr-FR;q=0.9, en-US;q=0.8",
			want:       "fr-FR",
		},
		{
			name:       "unknown Accept-Language falls back to default",
			acceptLang: "ja-JP",
			want:       "en-US",
		},
		{
			name:       "query param takes precedence over Accept-Language",
			query:      "lang=es-ES",
			acceptLang: "fr-FR",
			want:       "es-ES",
		},
		{
			name: "no language specified uses default",
			want: "en-US",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			url := "/api/search?q=test"
			if tt.query != "" {
				url += "&" + tt.query
			}
			r := httptest.NewRequest("GET", url, nil)
			if tt.acceptLang != "" {
				r.Header.Set("Accept-Language", tt.acceptLang)
			}

			got := server.ResolveAPILanguage(r, known, defaultLang)
			if got != tt.want {
				t.Errorf("ResolveAPILanguage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBundleLoadingAndMerging tests the full override chain:
// built-in base → theme overrides → site overrides.
func TestBundleLoadingAndMerging(t *testing.T) {
	t.Parallel()

	// Layer 1: built-in base strings.
	baseFS := fstest.MapFS{
		"locales/en-US.yml": &fstest.MapFile{
			Data: []byte("toc_title: \"On this page\"\nedit_page: \"Edit\"\nsearch_placeholder: \"Search...\"\n"),
		},
		"locales/fr-FR.yml": &fstest.MapFile{
			Data: []byte("toc_title: \"Sur cette page\"\nedit_page: \"Modifier\"\nsearch_placeholder: \"Rechercher...\"\n"),
		},
	}

	bundle, err := locale.LoadBundle("en-US", baseFS, "locales")
	if err != nil {
		t.Fatalf("LoadBundle base: %v", err)
	}

	// Verify base values.
	if got := bundle.T("en-US", "edit_page"); got != "Edit" {
		t.Errorf("base en-US edit_page = %q, want %q", got, "Edit")
	}
	if got := bundle.T("fr-FR", "edit_page"); got != "Modifier" {
		t.Errorf("base fr-FR edit_page = %q, want %q", got, "Modifier")
	}

	// Layer 2: theme overrides (only overrides edit_page).
	themeFS := fstest.MapFS{
		"locales/en-US.yml": &fstest.MapFile{
			Data: []byte("edit_page: \"Edit on GitHub\"\n"),
		},
	}
	if err := bundle.MergeFrom(themeFS, "locales"); err != nil {
		t.Fatalf("MergeFrom theme: %v", err)
	}

	// edit_page overridden for en-US, toc_title unchanged.
	if got := bundle.T("en-US", "edit_page"); got != "Edit on GitHub" {
		t.Errorf("after theme merge, en-US edit_page = %q, want %q", got, "Edit on GitHub")
	}
	if got := bundle.T("en-US", "toc_title"); got != "On this page" {
		t.Errorf("after theme merge, en-US toc_title = %q, want %q", got, "On this page")
	}
	// French unchanged (theme didn't override it).
	if got := bundle.T("fr-FR", "edit_page"); got != "Modifier" {
		t.Errorf("after theme merge, fr-FR edit_page = %q, want %q", got, "Modifier")
	}

	// Layer 3: site-level overrides (overrides search_placeholder for both).
	siteFS := fstest.MapFS{
		"locales/en-US.yml": &fstest.MapFile{
			Data: []byte("search_placeholder: \"Find docs...\"\n"),
		},
		"locales/fr-FR.yml": &fstest.MapFile{
			Data: []byte("search_placeholder: \"Trouver...\"\n"),
		},
	}
	if err := bundle.MergeFrom(siteFS, "locales"); err != nil {
		t.Fatalf("MergeFrom site: %v", err)
	}

	// search_placeholder overridden, others intact.
	if got := bundle.T("en-US", "search_placeholder"); got != "Find docs..." {
		t.Errorf("after site merge, en-US search_placeholder = %q, want %q", got, "Find docs...")
	}
	if got := bundle.T("fr-FR", "search_placeholder"); got != "Trouver..." {
		t.Errorf("after site merge, fr-FR search_placeholder = %q, want %q", got, "Trouver...")
	}
	if got := bundle.T("en-US", "edit_page"); got != "Edit on GitHub" {
		t.Errorf("after site merge, en-US edit_page = %q, want %q", got, "Edit on GitHub")
	}
	if got := bundle.T("en-US", "toc_title"); got != "On this page" {
		t.Errorf("after site merge, en-US toc_title = %q, want %q", got, "On this page")
	}

	// MergeFrom with missing directory is not an error.
	emptyFS := fstest.MapFS{}
	if err := bundle.MergeFrom(emptyFS, "nonexistent"); err != nil {
		t.Errorf("MergeFrom nonexistent dir: unexpected error %v", err)
	}
}

// TestTemplateContextI18n tests the TemplateContext i18n wiring:
// T(), Lang(), Lang() with frontmatter override, and Languages().
func TestTemplateContextI18n(t *testing.T) {
	t.Parallel()

	// Build a bundle for the translation function.
	localeFS := fstest.MapFS{
		"locales/en-US.yml": &fstest.MapFile{
			Data: []byte("toc_title: \"On this page\"\nedit_page: \"Edit\"\n"),
		},
		"locales/fr-FR.yml": &fstest.MapFile{
			Data: []byte("toc_title: \"Sur cette page\"\nedit_page: \"Modifier\"\n"),
		},
	}
	bundle, err := locale.LoadBundle("en-US", localeFS, "locales")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	languages := []tmpl.LanguageInfo{
		{Code: "en-US", Name: "English", Active: false},
		{Code: "fr-FR", Name: "Français", Active: true},
	}

	t.Run("T returns translated strings", func(t *testing.T) {
		t.Parallel()

		tc := &tmpl.TemplateContext{
			Site: &config.SiteConfig{Language: "en-US"},
			Page: tmpl.PageContext{Meta: map[string]any{}},
		}
		tFunc := func(key string) string { return bundle.T("fr-FR", key) }
		tc.WithI18n("fr-FR", tFunc, languages)

		if got := tc.T("toc_title"); got != "Sur cette page" {
			t.Errorf("T(toc_title) = %q, want %q", got, "Sur cette page")
		}
		if got := tc.T("edit_page"); got != "Modifier" {
			t.Errorf("T(edit_page) = %q, want %q", got, "Modifier")
		}
	})

	t.Run("T returns key for unknown strings", func(t *testing.T) {
		t.Parallel()

		tc := &tmpl.TemplateContext{
			Site: &config.SiteConfig{Language: "en-US"},
			Page: tmpl.PageContext{Meta: map[string]any{}},
		}
		tFunc := func(key string) string { return bundle.T("fr-FR", key) }
		tc.WithI18n("fr-FR", tFunc, languages)

		if got := tc.T("nonexistent_key"); got != "nonexistent_key" {
			t.Errorf("T(nonexistent_key) = %q, want %q", got, "nonexistent_key")
		}
	})

	t.Run("T without i18n wiring returns key", func(t *testing.T) {
		t.Parallel()

		tc := &tmpl.TemplateContext{
			Site: &config.SiteConfig{Language: "en-US"},
			Page: tmpl.PageContext{Meta: map[string]any{}},
		}

		if got := tc.T("toc_title"); got != "toc_title" {
			t.Errorf("T(toc_title) without i18n = %q, want %q", got, "toc_title")
		}
	})

	t.Run("Lang returns wired language", func(t *testing.T) {
		t.Parallel()

		tc := &tmpl.TemplateContext{
			Site: &config.SiteConfig{Language: "en-US"},
			Page: tmpl.PageContext{Meta: map[string]any{}},
		}
		tc.WithI18n("fr-FR", func(key string) string { return key }, nil)

		if got := tc.Lang(); got != "fr-FR" {
			t.Errorf("Lang() = %q, want %q", got, "fr-FR")
		}
	})

	t.Run("Lang with frontmatter override", func(t *testing.T) {
		t.Parallel()

		tc := &tmpl.TemplateContext{
			Site: &config.SiteConfig{Language: "en-US"},
			Page: tmpl.PageContext{
				Meta: map[string]any{"lang": "de-DE"},
			},
		}
		tc.WithI18n("fr-FR", func(key string) string { return key }, nil)

		// Frontmatter lang overrides the wired language.
		if got := tc.Lang(); got != "de-DE" {
			t.Errorf("Lang() with frontmatter = %q, want %q", got, "de-DE")
		}
	})

	t.Run("Lang falls back to site config", func(t *testing.T) {
		t.Parallel()

		tc := &tmpl.TemplateContext{
			Site: &config.SiteConfig{Language: "en-US"},
			Page: tmpl.PageContext{Meta: map[string]any{}},
		}
		// No WithI18n call.

		if got := tc.Lang(); got != "en-US" {
			t.Errorf("Lang() fallback = %q, want %q", got, "en-US")
		}
	})

	t.Run("Languages returns the language list", func(t *testing.T) {
		t.Parallel()

		tc := &tmpl.TemplateContext{
			Site: &config.SiteConfig{Language: "en-US"},
			Page: tmpl.PageContext{Meta: map[string]any{}},
		}
		tc.WithI18n("fr-FR", func(key string) string { return key }, languages)

		got := tc.Languages()
		if len(got) != 2 {
			t.Fatalf("Languages() returned %d items, want 2", len(got))
		}

		idx := slices.IndexFunc(got, func(l tmpl.LanguageInfo) bool { return l.Code == "fr-FR" })
		if idx < 0 {
			t.Fatal("Languages() missing fr-FR")
		}
		if !got[idx].Active {
			t.Error("fr-FR should be active")
		}
		if got[idx].Name != "Français" {
			t.Errorf("fr-FR Name = %q, want %q", got[idx].Name, "Français")
		}
	})

	t.Run("Languages returns nil when not wired", func(t *testing.T) {
		t.Parallel()

		tc := &tmpl.TemplateContext{
			Site: &config.SiteConfig{Language: "en-US"},
			Page: tmpl.PageContext{Meta: map[string]any{}},
		}

		if got := tc.Languages(); got != nil {
			t.Errorf("Languages() without i18n = %v, want nil", got)
		}
	})
}
