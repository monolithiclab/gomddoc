package main

import (
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/kong"

	"github.com/monolithiclab/gomddoc/docs"
	"github.com/monolithiclab/gomddoc/internal/config"
)

var envMention = regexp.MustCompile(`GOMDDOC_[A-Z0-9_]+`)

// testModel is the Kong model users run, built through the same options.
func testModel(t *testing.T) *kong.Application {
	t.Helper()
	return kong.Must(&CLI{}, parserOptions()...).Model
}

// knownEnvs splits knownEnvVars into exact names and map-setting prefixes.
func knownEnvs(t *testing.T) (exact, prefixes []string) {
	t.Helper()
	for _, k := range knownEnvVars(testModel(t)) {
		if strings.HasSuffix(k, "_") {
			prefixes = append(prefixes, k)
		} else {
			exact = append(exact, k)
		}
	}
	return exact, prefixes
}

// TestGuide_MentionsOnlyRealEnvVars: a guide that names a variable gomddoc does
// not read sends an agent to set something that does nothing.
func TestGuide_MentionsOnlyRealEnvVars(t *testing.T) {
	t.Parallel()
	exact, prefixes := knownEnvs(t)
	err := fs.WalkDir(docs.Guide, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".md" {
			return err
		}
		data, err := fs.ReadFile(docs.Guide, p)
		if err != nil {
			return err
		}
		for _, m := range envMention.FindAllString(string(data), -1) {
			if slices.Contains(exact, m) ||
				slices.ContainsFunc(prefixes, func(pre string) bool { return strings.HasPrefix(m, pre) }) {
				continue
			}
			t.Errorf("%s mentions %s, which gomddoc does not read", p, m)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestGuide_ConfigurationPageCoversEverySetting: every file key and env var in
// config.Schema is documented on the configuration page.
func TestGuide_ConfigurationPageCoversEverySetting(t *testing.T) {
	t.Parallel()
	data, err := fs.ReadFile(docs.Guide, "02-configuration.md")
	if err != nil {
		t.Fatal(err)
	}
	page := string(data)
	for _, s := range config.Schema() {
		if s.FileKey != "" && !strings.Contains(page, "`"+s.FileKey) {
			t.Errorf("02-configuration.md does not mention file key `%s`", s.FileKey)
		}
		if env, _ := strings.CutSuffix(s.Env, "<KEY>"); env != "" && !strings.Contains(page, env) {
			t.Errorf("02-configuration.md does not mention %s", env)
		}
	}
}
