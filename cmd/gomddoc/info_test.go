package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/capabilities"
	"github.com/monolithiclab/gomddoc/internal/config"
)

func runInfo(t *testing.T, dir string, asJSON bool) string {
	t.Helper()
	var buf bytes.Buffer
	cmd := &InfoCmd{Dir: dir, JSON: asJSON, out: &buf}
	if err := cmd.Run(testModel(t)); err != nil {
		t.Fatalf("info must not fail, got %v", err)
	}
	return buf.String()
}

func siteWithConfig(t *testing.T, yml string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".gomddoc"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gomddoc", "config.yml"), []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInfo_JSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		dir        func(t *testing.T) string
		wantLoaded bool
		wantFound  bool
		wantErr    string // substring of load_error; "" = none
		wantSource string
	}{
		{"site with config", func(t *testing.T) string { return siteWithConfig(t, "theme: {name: default}\n") }, true, true, "", "template-scan"},
		{"no config file", func(t *testing.T) string { return t.TempDir() }, true, false, "", "template-scan"},
		{"broken config", func(t *testing.T) string { return siteWithConfig(t, "nope: 1\n") }, false, true, "nope", "template-scan"},
		{"git URL", func(*testing.T) string { return "git+https://example.com/repo.git" }, false, false, "not inspected", "template-scan"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := tt.dir(t)
			out := runInfo(t, dir, true)
			var r capabilities.Report
			if err := json.Unmarshal([]byte(out), &r); err != nil {
				t.Fatalf("info --json is not a Report: %v\n%s", err, out)
			}
			if r.Config.Loaded != tt.wantLoaded || r.Config.FileFound != tt.wantFound || r.Config.Dir != dir {
				t.Errorf("config = %+v", r.Config)
			}
			if (tt.wantErr == "") != (r.Config.LoadError == "") || !strings.Contains(r.Config.LoadError, tt.wantErr) {
				t.Errorf("load_error = %q, want containing %q", r.Config.LoadError, tt.wantErr)
			}
			if r.Theme.Name != "default" || r.Theme.Source != tt.wantSource {
				t.Errorf("theme = %+v", r.Theme)
			}
		})
	}
}

// TestInfo_JSONIsReportJSON: info --json prints Report.JSON verbatim, the same
// bytes the gomddoc://capabilities resource serves.
func TestInfo_JSONIsReportJSON(t *testing.T) {
	t.Parallel()
	dir := siteWithConfig(t, "theme: {name: default}\n")
	got := runInfo(t, dir, true)
	cfg, err := config.NewFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := capabilities.Describe(capabilitiesInput(testModel(t), dir, cfg, nil, os.DirFS(dir))).JSON()
	if got != string(want)+"\n" {
		t.Errorf("info --json differs from Report.JSON()")
	}
}

func TestInfo_Human(t *testing.T) {
	t.Parallel()
	out := runInfo(t, siteWithConfig(t, "nope: 1\n"), false)
	for _, want := range []string{
		"Precedence: flag > env > file > default",
		"not loaded:",             // the reason is shown
		"nope",                    // ...and it is the loader's reason
		"GOMDDOC_SITE_THEME_NAME", // a setting's env var
		"site.meta.domain",        // a setting key
		"--domain",                // a joined flag
		"toc",                     // a theme feature
		"02-configuration.md",     // a guide page
		"gomddoc info --json",     // pointer to the machine-readable form
		"gomddoc schema",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output lacks %q", want)
		}
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Error("human output is JSON")
	}
}
