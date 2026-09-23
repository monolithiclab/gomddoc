package capabilities

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/template"
)

var testGuide = fstest.MapFS{
	"README.md":              {Data: []byte("---\ntitle: Guide\ndescription: Start here\n---\n# Guide\n")},
	"02-configuration.md":    {Data: []byte("---\ntitle: Configuration\ndescription: Settings\n---\n# Config\n")},
	"12-advanced/01-http.md": {Data: []byte("---\ntitle: HTTP\ndescription: Caching\n---\n# HTTP\n")},
}

var testCommands = []Command{
	{Name: "serve", Flags: []Flag{
		{Name: "--port", Env: "GOMDDOC_SERVER_PORT"},
		{Name: "--domain", Env: "GOMDDOC_DOMAIN", Setting: "site.meta.domain"},
		{Name: "--basic-auth-file", Env: "GOMDDOC_SERVER_BASIC_AUTH_FILE"}, // command-only
	}, Args: []Arg{{Name: "dir", Env: "GOMDDOC_SERVER_DIR"}}},
	{Name: "preview", Flags: []Flag{{Name: "--port", Env: "GOMDDOC_SERVER_PORT"}}},
}

func settingByKey(r Report, key string) config.Setting {
	i := slices.IndexFunc(r.Settings, func(s config.Setting) bool { return s.Key == key })
	if i < 0 {
		return config.Setting{}
	}
	return r.Settings[i]
}

func TestDescribe_JoinsFlagsOntoSettings(t *testing.T) {
	t.Parallel()
	r := Describe(Input{Config: config.New(), Commands: testCommands, Guide: testGuide})
	tests := []struct {
		key  string
		want []string
	}{
		{"server.port", []string{"--port"}},        // joined by env, deduplicated across commands
		{"site.meta.domain", []string{"--domain"}}, // joined by setting tag; env differs
		{"server.dir", []string{"DIR"}},            // positional
		{"site.theme.name", nil},                   // no flag
	}
	for _, tt := range tests {
		if got := settingByKey(r, tt.key).Flags; !slices.Equal(got, tt.want) {
			t.Errorf("%s flags = %q, want %q", tt.key, got, tt.want)
		}
	}
}

// TestDescribe_SettingTagWinsOverEnv: a flag tagged for one setting never also
// joins another through its env var.
func TestDescribe_SettingTagWinsOverEnv(t *testing.T) {
	t.Parallel()
	cmds := []Command{{Name: "x", Flags: []Flag{{Name: "--odd", Env: "GOMDDOC_SERVER_PORT", Setting: "site.meta.domain"}}}}
	r := Describe(Input{Commands: cmds, Guide: testGuide})
	if got := settingByKey(r, "server.port").Flags; got != nil {
		t.Errorf("server.port flags = %q, want none", got)
	}
	if got := settingByKey(r, "site.meta.domain").Flags; !slices.Equal(got, []string{"--odd"}) {
		t.Errorf("site.meta.domain flags = %q", got)
	}
}

func TestDescribe_ConfigBlock(t *testing.T) {
	t.Parallel()
	loaded := Describe(Input{Dir: "site", Config: config.New(), FileFound: true, Guide: testGuide})
	failed := Describe(Input{Dir: "site", ConfigErr: errors.New("config.yml: field nope not found"), Guide: testGuide})

	if !loaded.Config.Loaded || loaded.Config.LoadError != "" || !loaded.Config.FileFound || loaded.Config.Dir != "site" {
		t.Errorf("loaded: %+v", loaded.Config)
	}
	if failed.Config.Loaded || failed.Config.LoadError != "config.yml: field nope not found" {
		t.Errorf("failed: %+v", failed.Config)
	}
	if failed.Theme.Name != config.DefaultThemeName {
		t.Errorf("theme falls back to %q, got %q", config.DefaultThemeName, failed.Theme.Name)
	}
	if !slices.Equal(loaded.Config.Precedence, config.Precedence) || loaded.Config.SchemaURI != SchemaURI ||
		loaded.Config.File != ".gomddoc/config.yml" || loaded.Config.FileKeys != config.FileKeysNote {
		t.Errorf("config block: %+v", loaded.Config)
	}
}

func TestDescribe_ActiveThemeFromConfig(t *testing.T) {
	t.Parallel()
	cfg := config.New()
	cfg.Site.Theme.Name = "custom"
	assets := fstest.MapFS{"assets/themes/custom/layouts/default.html.tmpl": {Data: []byte(`{{ if .Feature "toc" }}{{ end }}`)}}
	r := Describe(Input{Config: cfg, Assets: assets, Guide: testGuide})
	if r.Theme.Name != "custom" || r.Theme.Source != template.ThemeSourceTemplateScan || !slices.Equal(r.Theme.Features, []string{"toc"}) {
		t.Errorf("theme: %+v", r.Theme)
	}
}

func TestDescribe_Guide(t *testing.T) {
	t.Parallel()
	r := Describe(Input{Guide: testGuide})
	want := []GuidePage{
		{"02-configuration.md", "Configuration", "Settings"},
		{"12-advanced/01-http.md", "HTTP", "Caching"},
		{"README.md", "Guide", "Start here"},
	}
	if !slices.Equal(r.Guide, want) || r.GuideError != "" {
		t.Errorf("guide = %+v (err %q)\nwant %+v", r.Guide, r.GuideError, want)
	}
}

func TestDescribe_NilAssetsThemeUnavailable(t *testing.T) {
	t.Parallel()
	r := Describe(Input{Config: config.New(), Guide: testGuide})
	if r.Theme.Source != template.ThemeSourceUnavailable || r.Theme.Docs != GuideURIPrefix+"05-theming-and-assets.md" ||
		r.Theme.Features == nil {
		t.Errorf("theme: %+v", r.Theme)
	}
}

func TestReport_JSON(t *testing.T) {
	t.Parallel()
	in := Input{Version: "1.2.3", Config: config.New(), Commands: testCommands, Guide: testGuide}
	r := Describe(in)
	var back Report
	if err := json.Unmarshal(r.JSON(), &back); err != nil {
		t.Fatal(err)
	}
	if back.Version != "1.2.3" || len(back.Settings) != len(r.Settings) || len(back.Commands) != 2 ||
		len(back.Frontmatter) == 0 || !slices.Equal(back.Guide, r.Guide) || back.Theme.Name != r.Theme.Name {
		t.Errorf("round trip lost data: %+v", back)
	}
	// The embedded ThemeInfo flattens: "active", not a nested object.
	if !bytes.Contains(r.JSON(), []byte(`"active": "default"`)) {
		t.Error(`theme must serialize as {"active": …}`)
	}
	// Same inputs, same bytes: MCP and info --json are compared byte-for-byte.
	if !bytes.Equal(r.JSON(), Describe(in).JSON()) {
		t.Error("JSON() is not deterministic")
	}
}
