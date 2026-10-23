// Package locale decides which language a request is for and what the UI
// strings say in it.
//
// DetectLanguages accepts a content-root directory as a language only when its
// name carries a script or region subtag. That is deliberately narrower than
// BCP 47, which would also claim doc/, api/, css/, bin/, id/, is/, no/ and
// it/ — and a false positive here does not merely mislabel a directory, it
// gives it a pipeline of its own and excludes it from the default one, so the
// site loses the content. See docs/decisions.md.
package locale

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Bundle holds locale strings for all loaded languages.
type Bundle struct {
	defaultLang string
	strings     map[string]map[string]string // lang → key → value
}

// pluralSuffixes are the CLDR plural categories a key may carry beyond its
// general form (which doubles as "other").
var pluralSuffixes = []string{"_zero", "_one", "_two", "_few", "_many"}

// T returns the translated string for the given language and key.
// Fallback chain: lang → defaultLang → key itself. A CLDR plural variant (a
// key ending in _zero, _one, _two, _few or _many) follows its general form:
// where lang translates the general form but not the variant, T answers the
// key itself, so the caller keeps lang's general form rather than the default
// language's variant, which would follow another language's plural rules.
func (b *Bundle) T(lang, key string) string {
	langStrings := b.strings[lang]
	if val, ok := langStrings[key]; ok {
		return val
	}
	for _, suffix := range pluralSuffixes {
		if general, ok := strings.CutSuffix(key, suffix); ok {
			if _, ok := langStrings[general]; ok {
				return key
			}
			break
		}
	}
	if val, ok := b.strings[b.defaultLang][key]; ok {
		return val
	}
	return key
}

// TFunc returns a translation function bound to the given language.
func (b *Bundle) TFunc(lang string) func(string) string {
	return func(key string) string {
		return b.T(lang, key)
	}
}

// LanguageName returns the display name for a language code.
// Falls back to the code itself if language_name is not set.
func (b *Bundle) LanguageName(lang string) string {
	if langStrings, ok := b.strings[lang]; ok {
		if name, ok := langStrings["language_name"]; ok {
			return name
		}
	}
	return lang
}

// Keys returns, sorted, every key starting with prefix in any loaded language.
func (b *Bundle) Keys(prefix string) []string {
	seen := make(map[string]bool)
	for _, langStrings := range b.strings {
		for key := range langStrings {
			if strings.HasPrefix(key, prefix) {
				seen[key] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

// LoadBundle loads locale YAML files from dir within fsys.
// Files must be named {lang}.yml (e.g., en-US.yml).
func LoadBundle(defaultLang string, fsys fs.FS, dir string) (*Bundle, error) {
	b := &Bundle{
		defaultLang: defaultLang,
		strings:     make(map[string]map[string]string),
	}
	if err := b.loadFrom(fsys, dir); err != nil {
		return nil, err
	}
	return b, nil
}

// MergeFrom loads locale files from another filesystem layer,
// overriding existing keys per language.
func (b *Bundle) MergeFrom(fsys fs.FS, dir string) error {
	return b.loadFrom(fsys, dir)
}

// loadFrom reads all .yml files in dir and merges them into the bundle.
func (b *Bundle) loadFrom(fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // directory doesn't exist — not an error
		}
		return fmt.Errorf("read locale dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".yml") {
			continue
		}

		lang := strings.TrimSuffix(name, ".yml")
		filePath := path.Join(dir, name)

		data, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return fmt.Errorf("read locale %s: %w", filePath, err)
		}

		var values map[string]string
		if err := yaml.Unmarshal(data, &values); err != nil {
			return fmt.Errorf("parse locale %s: %w", filePath, err)
		}

		if b.strings[lang] == nil {
			b.strings[lang] = make(map[string]string)
		}
		maps.Copy(b.strings[lang], values)
	}

	return nil
}
