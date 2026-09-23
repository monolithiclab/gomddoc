package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/diag"
)

// ids renders findings as "code file:line key" for exhaustive comparison.
func ids(fs []diag.Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, fmt.Sprintf("%s %s:%d %s", f.Code, f.File, f.Line, f.Key))
	}
	return out
}

func inspect(t *testing.T, yml string) config.Inspection {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, config.ConfigDirName), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFile), []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	return config.Inspect(dir)
}

func TestUnknownKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		yml    string
		want   []string
		fixHas []string // one substring per finding, same order
	}{
		{"clean", "meta: {title: x}\ntheme: {name: default, vars: {anything: x}, features: {whatever: true}}\n", nil, nil},
		{"typo at root", "hightlighting: {theme: x}\n",
			[]string{"config.unknown-key .gomddoc/config.yml:1 hightlighting"}, []string{"did you mean `highlighting`?"}},
		{"typo nested", "meta:\n  titel: x\n",
			[]string{"config.unknown-key .gomddoc/config.yml:2 meta.titel"}, []string{"did you mean `title`?"}},
		{"no close match", "meta:\n  zzz: x\n",
			[]string{"config.unknown-key .gomddoc/config.yml:2 meta.zzz"}, []string{"valid keys here: description, domain, robots, title"}},
		{"site wrapper", "site:\n  meta: {title: x}\n",
			[]string{"config.unknown-key .gomddoc/config.yml:1 site"}, []string{"remove the `site:` wrapper"}},
		{"server setting in the file", "port: ':9000'\n",
			[]string{"config.unknown-key .gomddoc/config.yml:1 port"}, []string{"GOMDDOC_SERVER_PORT"}},
		{"several", "meta:\n  titel: x\nnope: 1\n",
			[]string{"config.unknown-key .gomddoc/config.yml:2 meta.titel", "config.unknown-key .gomddoc/config.yml:3 nope"}, []string{"title", "valid keys here"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := unknownKeys(inspect(t, tt.yml).Node)
			if !slices.Equal(ids(got), tt.want) {
				t.Fatalf("findings = %q, want %q", ids(got), tt.want)
			}
			for i, f := range got {
				if !strings.Contains(f.Fix, tt.fixHas[i]) {
					t.Errorf("%s: fix %q lacks %q", f.Key, f.Fix, tt.fixHas[i])
				}
			}
		})
	}
	if got := unknownKeys(nil); got != nil {
		t.Errorf("nil node: %+v", got)
	}
}

func TestUnknownEnv(t *testing.T) {
	t.Parallel()
	known := []string{"GOMDDOC_SITE_THEME_NAME", "GOMDDOC_SERVER_PORT", "GOMDDOC_SITE_THEME_FEATURES_"}
	environ := []string{
		"GOMDDOC_SITE_THEME_NAME=nord",      // known
		"GOMDDOC_SITE_THEME_FEATURES_TOC=0", // known prefix
		"GOMDDOC_SITE_THEME=nord",           // unknown, close to a known one
		"GOMDDOC_WHATEVER=1",                // unknown, nothing close
		"HOME=/root",                        // not ours
		"GOMDDOC_NOVALUE",                   // malformed entry, still a name
	}
	got := unknownEnv(environ, known)
	want := []string{"env.unknown :0 GOMDDOC_NOVALUE", "env.unknown :0 GOMDDOC_SITE_THEME", "env.unknown :0 GOMDDOC_WHATEVER"}
	diag.Sort(got)
	if !slices.Equal(ids(got), want) {
		t.Errorf("findings = %q, want %q", ids(got), want)
	}
	for _, f := range got {
		if f.Key == "GOMDDOC_SITE_THEME" && !strings.Contains(f.Fix, "GOMDDOC_SITE_THEME_NAME") {
			t.Errorf("fix %q should suggest GOMDDOC_SITE_THEME_NAME", f.Fix)
		}
	}
}

func TestRun(t *testing.T) {
	t.Parallel()
	in := Input{
		Target:     "site",
		Inspection: inspect(t, "meta:\n  titel: x\n"),
		Producer: []diag.Finding{
			diag.New("content.missing-description", "a.md", 0, "", "no description", "add one"),
			diag.New("content.tags-collision", "tags.md", 0, "", "shadowed", "rename"),
		},
	}
	quiet := Run(context.Background(), in, Options{})
	verbose := Run(context.Background(), in, Options{Verbose: true})

	wantSummary := Summary{Errors: 1, Warnings: 1, Info: 1}
	if quiet.Summary != (Summary{Errors: 1, Warnings: 1, Info: 1, InfoHidden: true}) || verbose.Summary != wantSummary {
		t.Errorf("summaries: quiet %+v verbose %+v", quiet.Summary, verbose.Summary)
	}
	if got := ids(quiet.Findings); !slices.Equal(got, []string{
		"config.unknown-key .gomddoc/config.yml:2 meta.titel", "content.tags-collision tags.md:0 "}) {
		t.Errorf("quiet findings = %q", got)
	}
	if len(verbose.Findings) != 3 || verbose.Findings[2].Code != "content.missing-description" {
		t.Errorf("verbose findings = %q", ids(verbose.Findings))
	}
	if quiet.Target != "site" {
		t.Errorf("target = %q", quiet.Target)
	}
}

func TestRun_CleanAndUnreachable(t *testing.T) {
	t.Parallel()
	clean := Run(context.Background(), Input{Target: "x", Inspection: config.Inspect(t.TempDir())}, Options{Verbose: true})
	if len(clean.Findings) != 0 || clean.Findings == nil || clean.Summary != (Summary{}) {
		t.Errorf("clean site: %+v", clean)
	}
	gone := Run(context.Background(), Input{Target: "x", Unreachable: errors.New("stat x: no such file"),
		Inspection: inspect(t, "nope: 1\n")}, Options{})
	if got := ids(gone.Findings); !slices.Equal(got, []string{"target.unreachable :0 "}) || !strings.Contains(gone.Findings[0].Message, "no such file") {
		t.Errorf("unreachable: %q %+v", got, gone.Findings)
	}
}
