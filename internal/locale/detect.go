package locale

import "io/fs"

// IsBCP47Dir checks if a directory name is a valid BCP 47 language tag (ll-CC format).
func IsBCP47Dir(name string) bool {
	if len(name) != 5 || name[2] != '-' {
		return false
	}
	for _, c := range name[:2] {
		if c < 'a' || c > 'z' {
			return false
		}
	}
	for _, c := range name[3:] {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// DetectLanguages scans the root of a filesystem for BCP 47 directories.
func DetectLanguages(fsys fs.FS) []string {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil
	}

	var langs []string
	for _, entry := range entries {
		if entry.IsDir() && IsBCP47Dir(entry.Name()) {
			langs = append(langs, entry.Name())
		}
	}
	return langs
}
