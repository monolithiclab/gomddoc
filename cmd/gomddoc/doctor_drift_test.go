package main

import (
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/docs"
	"github.com/monolithiclab/gomddoc/internal/diag"
)

var catalogueRow = regexp.MustCompile("(?m)^\\| `([a-z]+\\.[a-z-]+)` \\| (error|warning|info) \\|")

// TestDoctorGuide_MatchesCatalogue: the guide's finding table and
// diag.Catalogue list the same codes with the same severities.
func TestDoctorGuide_MatchesCatalogue(t *testing.T) {
	t.Parallel()
	page, err := fs.ReadFile(docs.Guide, "14-doctor.md")
	if err != nil {
		t.Fatal(err)
	}
	documented := map[string]string{}
	for _, m := range catalogueRow.FindAllStringSubmatch(string(page), -1) {
		documented[m[1]] = m[2]
	}
	for _, c := range diag.Catalogue {
		if got, ok := documented[c.Code]; !ok || got != string(c.Severity) {
			t.Errorf("%s: guide says %q, catalogue says %q", c.Code, got, c.Severity)
		}
		delete(documented, c.Code)
	}
	for code := range documented {
		t.Errorf("guide documents %s, which is not in diag.Catalogue", code)
	}
}

// TestDoctorCatalogue_EveryCodeEmitted: each code appears in production code
// outside internal/diag — a catalogued code nothing reports is a promise the
// guide makes and doctor cannot keep.
func TestDoctorCatalogue_EveryCodeEmitted(t *testing.T) {
	t.Parallel()
	repo := os.DirFS("../..")
	var src strings.Builder
	for _, root := range []string{"cmd", "internal"} {
		err := fs.WalkDir(repo, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || path.Ext(p) != ".go" || strings.HasSuffix(p, "_test.go") || strings.HasPrefix(p, "internal/diag/") {
				return err
			}
			data, err := fs.ReadFile(repo, p)
			src.Write(data)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range diag.Catalogue {
		if !strings.Contains(src.String(), `"`+c.Code+`"`) {
			t.Errorf("%s is catalogued but no production code reports it", c.Code)
		}
	}
}
