package locale

import (
	"io/fs"

	"golang.org/x/text/language"
)

// isLanguageDir reports whether a content-root directory name is a BCP 47 tag
// naming a translation tree: a valid language subtag carrying a script and/or a
// region, spelled canonically. "en-US", "zh-Hans", "es-419" and "sr-Latn-RS"
// qualify; "en", "doc", "zz-ZZ", "en-us" and "is-a-test" do not.
//
// It is deliberately narrower than BCP 47. See docs/decisions.md, "A Language
// Directory Needs a Script or a Region Subtag".
func isLanguageDir(name string) bool {
	tag, err := language.Parse(name)
	if err != nil {
		return false
	}

	// Raw reports the subtags that were actually written; Script and Region
	// would infer the ones that were not. This is what rejects "is-a-test":
	// Parse reads it as Icelandic plus an extension singleton, and an extension
	// is not a region.
	base, script, region := tag.Raw()
	if script == (language.Script{}) && region == (language.Region{}) {
		return false
	}

	// Compose keeps only base/script/region, so a variant, extension or
	// privateuse subtag on a tag that does have a region ("en-US-x-foo",
	// "ca-ES-valencia") cannot survive the round-trip. The comparison against
	// name is also what rejects a non-canonical spelling such as "en-us".
	canon, err := language.Compose(base, script, region)
	return err == nil && canon.String() == name
}

// DetectLanguages scans the root of a filesystem for language directories.
func DetectLanguages(fsys fs.FS) []string {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil
	}

	var langs []string
	for _, entry := range entries {
		if entry.IsDir() && isLanguageDir(entry.Name()) {
			langs = append(langs, entry.Name())
		}
	}
	return langs
}
