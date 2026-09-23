package config

import (
	"slices"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/diag"
)

func TestValidateAll_ReportsEveryInvalidValue(t *testing.T) {
	t.Parallel()
	c := New()
	c.Server.Dir = t.TempDir()
	c.Site.DefaultIndex = ""
	c.Site.EditURL = "ftp://x/"
	c.Site.Meta.Domain = "https://docs.example.com"
	c.Site.Theme.Features = map[string]bool{"toc": true, "Toc": false, "bad-key": true}
	c.Site.StripExtensions = []string{".md", "html"}
	c.Server.AdminPort = "nope"

	got := c.ValidateAll()
	var keys []string
	for _, f := range got {
		keys = append(keys, f.Key)
		if f.Code != "config.invalid-value" || f.Severity != diag.Error || f.Message == "" {
			t.Errorf("finding = %+v", f)
		}
		wantFile := ConfigFile
		if f.Key == "server.admin_port" {
			wantFile = ""
		}
		if f.File != wantFile {
			t.Errorf("%s: File = %q, want %q", f.Key, f.File, wantFile)
		}
	}
	want := []string{"server.admin_port", "default_index", "edit_url", "meta.domain",
		"theme.features.Toc", "theme.features.bad-key", "strip_extensions"}
	if !slices.Equal(keys, want) {
		t.Errorf("keys = %q, want %q", keys, want)
	}

	// Validate still stops at the first problem, with the text it always had.
	if err := c.Validate(); err == nil || err.Error() != `invalid admin port format: address nope: missing port in address` {
		t.Errorf("Validate() = %v", err)
	}
	if err := c.Site.Validate(); err == nil || err.Error() != "default_index must not be empty" {
		t.Errorf("Site.Validate() = %v", err)
	}
}

func TestValidateAll_ValidConfig(t *testing.T) {
	t.Parallel()
	c := New()
	c.Server.Dir = "/definitely/not/a/dir" // the directory is doctor's target check, not a config value
	if got := c.ValidateAll(); len(got) != 0 {
		t.Errorf("findings = %+v", got)
	}
}
