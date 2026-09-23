package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// findingIDs renders findings as "code key line" for exhaustive comparison.
func findingIDs(ins Inspection) []string {
	var ids []string
	for _, f := range ins.Findings {
		ids = append(ids, fmt.Sprintf("%s %s %d", f.Code, f.Key, f.Line))
	}
	return ids
}

func siteDir(t *testing.T, yml *string) string {
	t.Helper()
	dir := t.TempDir()
	if yml != nil {
		if err := os.MkdirAll(filepath.Join(dir, ConfigDirName), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(*yml), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

//go:fix inline

func TestInspect(t *testing.T) {
	t.Parallel()
	testsite, err := os.ReadFile("../../testsite/.gomddoc/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		yml         *string
		want        []string
		wantNode    bool
		wantAssumed bool
	}{
		{"no file", nil, nil, false, false},
		{"testsite", new(string(testsite)), nil, true, false},
		{"empty file", new("# just a comment\n"), nil, false, false},
		{"syntax error", new("meta:\n  title: x\nexclude: [a,\n"), []string{"config.parse-error  3"}, false, true},
		{"second document", new("meta:\n  title: x\n---\ntheme: {name: y}\n"), []string{"config.parse-error  3"}, false, true}, // the separator line
		{"several problems", new("default_index: x\nexclude: drafts/\nmeta: {domain: https://x}\ntheme:\n  features:\n    toc: nope\n"),
			[]string{"config.wrong-type exclude 2", "config.wrong-type theme.features.toc 6", "config.invalid-value meta.domain 3"}, true, false},
		{"theme name emptied", new("theme: {name: ''}\n"), []string{"config.value-replaced theme.name 1"}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ins := Inspect(siteDir(t, tt.yml))
			if got := findingIDs(ins); !slices.Equal(got, tt.want) {
				t.Errorf("findings:\n got %q\nwant %q\n%+v", got, tt.want, ins.Findings)
			}
			if (ins.Node != nil) != tt.wantNode || (ins.Assumed != "") != tt.wantAssumed || ins.Config == nil {
				t.Errorf("node=%v assumed=%q config=%v", ins.Node != nil, ins.Assumed, ins.Config != nil)
			}
			for _, f := range ins.Findings {
				if f.File != ConfigFile {
					t.Errorf("%s: File = %q", f.Code, f.File)
				}
			}
		})
	}
}

// TestInspect_KeepsWhatParsed: a type error on one key leaves the rest applied.
func TestInspect_KeepsWhatParsed(t *testing.T) {
	t.Parallel()
	ins := Inspect(siteDir(t, new("exclude: drafts/\nmeta: {title: Handbook}\n")))
	if ins.Config.Site.Meta.Title != "Handbook" {
		t.Errorf("title = %q", ins.Config.Site.Meta.Title)
	}
}

// No t.Parallel: t.Setenv.
func TestInspect_EnvFindingOnce(t *testing.T) {
	t.Setenv("GOMDDOC_SITE_SEARCH_INDEX", "maybe")
	ins := Inspect(siteDir(t, nil))
	if got := findingIDs(ins); !slices.Equal(got, []string{"env.invalid-value GOMDDOC_SITE_SEARCH_INDEX 0"}) {
		t.Errorf("findings = %q", got)
	}
}
