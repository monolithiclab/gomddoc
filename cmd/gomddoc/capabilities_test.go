package main

import (
	"slices"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/capabilities"
	"github.com/monolithiclab/gomddoc/internal/config"
)

// TestSettingTags_Resolve: a `setting:` tag naming a key that does not exist
// would silently drop the flag from the report.
func TestSettingTags_Resolve(t *testing.T) {
	t.Parallel()
	keys := map[string]bool{}
	for _, s := range config.Schema() {
		keys[s.Key] = true
	}
	var tagged int
	for _, c := range commandsFromKong(testModel(t)) {
		for _, f := range c.Flags {
			if f.Setting == "" {
				continue
			}
			tagged++
			if !keys[f.Setting] {
				t.Errorf("%s %s: setting:%q is not a config key", c.Name, f.Name, f.Setting)
			}
		}
	}
	if tagged < 4 { // --domain on build/preview/serve, --dir-index on preview
		t.Errorf("found %d setting-tagged flags, want at least 4", tagged)
	}
}

func findCommand(t *testing.T, cmds []capabilities.Command, name string) capabilities.Command {
	t.Helper()
	i := slices.IndexFunc(cmds, func(c capabilities.Command) bool { return c.Name == name })
	if i < 0 {
		t.Fatalf("no %s command", name)
	}
	return cmds[i]
}

func findFlag(t *testing.T, c capabilities.Command, name string) capabilities.Flag {
	t.Helper()
	i := slices.IndexFunc(c.Flags, func(f capabilities.Flag) bool { return f.Name == name })
	if i < 0 {
		t.Fatalf("%s has no %s", c.Name, name)
	}
	return c.Flags[i]
}

func TestCommandsFromKong(t *testing.T) {
	t.Parallel()
	cmds := commandsFromKong(testModel(t))
	var names []string
	for _, c := range cmds {
		names = append(names, c.Name)
		if c.Help == "" {
			t.Errorf("%s has no help", c.Name)
		}
		for _, f := range c.Flags {
			if f.Name == "--help" || f.Name == "--version" {
				t.Errorf("%s lists Kong's own %s", c.Name, f.Name)
			}
		}
	}
	want := []string{"build", "info", "init", "mcp", "preview", "serve"}
	if !slices.Equal(names, want) {
		t.Errorf("commands = %q, want %q", names, want)
	}

	serve := findCommand(t, cmds, "serve")
	port := findFlag(t, serve, "--port")
	if port.Short != "-p" || port.Env != "GOMDDOC_SERVER_PORT" || port.Default != ":8080" || port.Help == "" || port.Setting != "" {
		t.Errorf("--port = %+v", port)
	}
	if got := findFlag(t, serve, "--domain").Setting; got != "site.meta.domain" {
		t.Errorf("--domain setting = %q", got)
	}
	if len(serve.Args) != 1 || serve.Args[0].Name != "dir" || serve.Args[0].Env != "GOMDDOC_SERVER_DIR" || serve.Args[0].Default != "." {
		t.Errorf("serve args = %+v", serve.Args)
	}
}

func TestCapabilitiesInput(t *testing.T) {
	t.Parallel()
	withFile := fstest.MapFS{".gomddoc/config.yml": {Data: []byte("theme: {name: default}\n")}}
	in := capabilitiesInput(testModel(t), "x", config.New(), nil, withFile)
	if !in.FileFound || in.Dir != "x" || in.Version != version || in.Guide == nil || in.Assets == nil || len(in.Commands) == 0 {
		t.Errorf("with file: %+v", in)
	}
	if in := capabilitiesInput(testModel(t), "x", config.New(), nil, fstest.MapFS{}); in.FileFound {
		t.Error("FileFound = true with no config file")
	}
	// No content root (a git URL info will not clone): embedded assets only.
	if in := capabilitiesInput(testModel(t), "git+https://x/y.git", nil, nil, nil); in.FileFound || in.Assets == nil {
		t.Errorf("nil root: %+v", in)
	}
}
