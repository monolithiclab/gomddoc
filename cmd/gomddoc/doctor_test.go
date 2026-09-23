package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/diag"
	"github.com/monolithiclab/gomddoc/internal/doctor"
)

const cleanPage = "---\ntitle: Home\ndescription: Welcome\n---\n# Home\n"

func runDoctorCmd(t *testing.T, cmd DoctorCmd) (string, int) {
	t.Helper()
	var buf bytes.Buffer
	cmd.out = &buf
	cmd.environ = func() []string { return nil } // the developer's own GOMDDOC_* must not leak in
	err := cmd.Run(testModel(t))
	code := 0
	var exit exitCodeError
	switch {
	case errors.As(err, &exit):
		code = int(exit)
	case err != nil:
		t.Fatalf("doctor returned %v", err)
	}
	return buf.String(), code
}

func doctorSite(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		writeTestFile(t, dir, name, content)
	}
	return dir
}

func TestDoctor_ExitStatus(t *testing.T) {
	t.Parallel()
	clean := map[string]string{"README.md": cleanPage}
	warning := map[string]string{"README.md": cleanPage, ".gomddoc/config.yml": "exclude: [archive/]\n"}
	errored := map[string]string{"README.md": cleanPage, ".gomddoc/config.yml": "meta:\n  titel: x\n"}
	infoOnly := map[string]string{"README.md": "# Home\n"}
	tests := []struct {
		name   string
		files  map[string]string
		dir    string
		strict bool
		want   int
	}{
		{"clean", clean, "", false, 0},
		{"info only", infoOnly, "", true, 0},
		{"warning", warning, "", false, 0},
		{"warning strict", warning, "", true, 1},
		{"error", errored, "", false, 1},
		{"missing directory", nil, "/definitely/not/here", false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := tt.dir
			if dir == "" {
				dir = doctorSite(t, tt.files)
			}
			out, code := runDoctorCmd(t, DoctorCmd{Dir: dir, Strict: tt.strict})
			if code != tt.want {
				t.Errorf("exit %d, want %d; output:\n%s", code, tt.want, out)
			}
		})
	}
}

func TestDoctor_JSON(t *testing.T) {
	t.Parallel()
	dir := doctorSite(t, map[string]string{"README.md": "# Home\n", ".gomddoc/config.yml": "meta:\n  titel: x\n"})
	out, code := runDoctorCmd(t, DoctorCmd{Dir: dir, JSON: true})
	var r doctor.Report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("--json is not a Report: %v\n%s", err, out)
	}
	if code != 1 || r.Target != dir || r.Summary.Errors != 1 || r.Summary.Info != 1 || !r.Summary.InfoHidden || len(r.Findings) != 1 ||
		r.Findings[0].Code != "config.unknown-key" || r.Findings[0].Line != 2 {
		t.Errorf("code %d report %+v", code, r)
	}
}

func TestDoctor_HumanAndVerbose(t *testing.T) {
	t.Parallel()
	dir := doctorSite(t, map[string]string{"README.md": "# Home\n", ".gomddoc/config.yml": "meta:\n  titel: x\n"})
	quiet, _ := runDoctorCmd(t, DoctorCmd{Dir: dir})
	verbose, _ := runDoctorCmd(t, DoctorCmd{Dir: dir, Verbose: true})

	for _, want := range []string{".gomddoc/config.yml:2", "config.unknown-key", "fix: did you mean `title`?",
		"1 error, 0 warnings (1 info hidden, use -v)"} {
		if !strings.Contains(quiet, want) {
			t.Errorf("quiet output lacks %q:\n%s", want, quiet)
		}
	}
	if strings.Contains(quiet, "content.missing-description") || !strings.Contains(verbose, "content.missing-description") {
		t.Errorf("info must show only with -v:\nquiet:\n%s\nverbose:\n%s", quiet, verbose)
	}
	if !strings.Contains(verbose, "1 error, 0 warnings, 1 info") {
		t.Errorf("verbose summary:\n%s", verbose)
	}
	clean, _ := runDoctorCmd(t, DoctorCmd{Dir: doctorSite(t, map[string]string{"README.md": cleanPage})})
	if !strings.Contains(clean, "No problems found.") {
		t.Errorf("clean output:\n%s", clean)
	}
}

func TestDoctor_UnknownEnv(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := DoctorCmd{Dir: doctorSite(t, map[string]string{"README.md": cleanPage}), JSON: true, out: &buf,
		environ: func() []string {
			return []string{"GOMDDOC_SITE_THEME=nord", "GOMDDOC_SITE_THEME_NAME=default", "GOMDDOC_INSTALL_DIR=/x"}
		}}
	_ = cmd.Run(testModel(t))
	var r doctor.Report
	if err := json.Unmarshal(buf.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, f := range r.Findings {
		keys = append(keys, f.Code+" "+f.Key)
	}
	if !slices.Equal(keys, []string{"env.unknown GOMDDOC_SITE_THEME"}) {
		t.Errorf("findings = %q", keys)
	}
}

// TestInstallScriptEnvs: the variables doctor treats as the install script's
// are exactly the ones scripts/install.sh reads.
func TestInstallScriptEnvs(t *testing.T) {
	t.Parallel()
	script, err := os.ReadFile("../../scripts/install.sh")
	if err != nil {
		t.Fatal(err)
	}
	var inScript []string
	for _, m := range regexp.MustCompile(`GOMDDOC_[A-Z0-9_]+`).FindAllString(string(script), -1) {
		if !slices.Contains(inScript, m) {
			inScript = append(inScript, m)
		}
	}
	slices.Sort(inScript)
	got := slices.Sorted(slices.Values(installScriptEnvs))
	if !slices.Equal(got, inScript) {
		t.Errorf("installScriptEnvs = %q, install.sh reads %q", got, inScript)
	}
}

// TestDoctor_LanguageDirectory: a translated page's problem is reported once,
// under its language directory.
func TestDoctor_LanguageDirectory(t *testing.T) {
	t.Parallel()
	dir := doctorSite(t, map[string]string{
		"README.md":       cleanPage,
		"fr-FR/README.md": "---\ntitle: Accueil\ndescription: Bienvenue\ntags: guide\n---\n# Accueil\n",
	})
	out, code := runDoctorCmd(t, DoctorCmd{Dir: dir, JSON: true})
	var r doctor.Report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range r.Findings {
		got = append(got, fmt.Sprintf("%s %s:%d", f.Code, f.File, f.Line))
	}
	if code != 1 || !slices.Equal(got, []string{"content.frontmatter-type fr-FR/README.md:4"}) {
		t.Errorf("exit %d, findings %q", code, got)
	}
}

func TestDoctorLocation(t *testing.T) {
	t.Parallel()
	for f, want := range map[diag.Finding]string{
		{File: "a.md", Line: 3}:     "a.md:3",
		{File: "a.md"}:              "a.md",
		{Key: "GOMDDOC_SITE_THEME"}: "GOMDDOC_SITE_THEME",
		{}:                          "-",
	} {
		if got := location(f); got != want {
			t.Errorf("location(%+v) = %q, want %q", f, got, want)
		}
	}
	if exitCodeError(1).Error() != "exit status 1" {
		t.Error("exitCodeError text")
	}
}

func doctorJSON(t *testing.T, dir string) (doctor.Report, int) {
	t.Helper()
	out, code := runDoctorCmd(t, DoctorCmd{Dir: dir, JSON: true})
	var r doctor.Report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("not a Report: %v\n%s", err, out)
	}
	return r, code
}

func codes(r doctor.Report) []string {
	var out []string
	for _, f := range r.Findings {
		out = append(out, f.Code+" "+f.Key)
	}
	return out
}

// TestDoctor_Target: a missing directory or a file is unreachable; a provider
// that fails on a config value is not — the config findings must show.
func TestDoctor_Target(t *testing.T) {
	t.Parallel()
	gone, code := doctorJSON(t, t.TempDir()+"/absent")
	if got := codes(gone); code != 1 || !slices.Equal(got, []string{"target.unreachable "}) {
		t.Errorf("missing dir: exit %d findings %q", code, got)
	}

	file := doctorSite(t, map[string]string{"README.md": cleanPage})
	notDir, _ := doctorJSON(t, file+"/README.md")
	if got := codes(notDir); !slices.Equal(got, []string{"target.unreachable "}) {
		t.Errorf("file as target: %q", got)
	}

	badIndex := doctorSite(t, map[string]string{"README.md": cleanPage,
		".gomddoc/config.yml": "default_index: \"\"\nbogus: 1\nedit_url: ftp://x/\n"})
	r, code := doctorJSON(t, badIndex)
	want := []string{"config.invalid-value default_index", "config.unknown-key bogus", "config.invalid-value edit_url"} // by line
	if got := codes(r); code != 1 || !slices.Equal(got, want) {
		t.Errorf("empty default_index: exit %d findings %q, want %q", code, got, want)
	}
	if !strings.Contains(r.Note, "content not checked") {
		t.Errorf("note = %q, want it to say the content was not checked", r.Note)
	}
}
