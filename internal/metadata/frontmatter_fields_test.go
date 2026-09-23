package metadata

import (
	"io/fs"
	"regexp"
	"slices"
	"testing"

	"github.com/monolithiclab/gomddoc/docs"
)

var fieldRow = regexp.MustCompile("(?m)^\\| `([a-z_]+)` \\|")

// TestFrontmatterFields_MatchGuide: the declared list and the guide's
// "Standard Fields" table are the same set, in both directions.
func TestFrontmatterFields_MatchGuide(t *testing.T) {
	t.Parallel()
	page, err := fs.ReadFile(docs.Guide, "12-advanced/02-markdown-extensions.md")
	if err != nil {
		t.Fatal(err)
	}
	var documented []string
	for _, m := range fieldRow.FindAllSubmatch(page, -1) {
		documented = append(documented, string(m[1]))
	}
	var declared []string
	for _, f := range FrontmatterFields {
		declared = append(declared, f.Key)
	}
	slices.Sort(documented)
	slices.Sort(declared)
	if !slices.Equal(documented, declared) {
		t.Errorf("guide table %q != FrontmatterFields %q", documented, declared)
	}
}

// TestFrontmatterFields_CoverKnownKeys: every key BuildIndex lifts out of Meta
// is one the declared list tells agents about.
func TestFrontmatterFields_CoverKnownKeys(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"title", "description", "date", "tags"} {
		if !slices.ContainsFunc(FrontmatterFields, func(f FrontmatterField) bool { return f.Key == k }) {
			t.Errorf("knownKeys entry %q is not declared", k)
		}
	}
}

func TestFrontmatterFields_Described(t *testing.T) {
	t.Parallel()
	for _, f := range FrontmatterFields {
		if f.Type == "" || len(f.Description) < 15 {
			t.Errorf("%s: type=%q description=%q", f.Key, f.Type, f.Description)
		}
	}
}

func TestFrontmatterFieldList_IsCopy(t *testing.T) {
	t.Parallel()
	l := FrontmatterFieldList()
	l[0].Key = "mutated"
	if FrontmatterFields[0].Key == "mutated" {
		t.Error("FrontmatterFieldList aliases the package list")
	}
}
