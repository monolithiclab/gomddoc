package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestSchema_Keys(t *testing.T) {
	t.Parallel()
	var keys, envs []string
	for _, s := range Schema() {
		keys = append(keys, s.Key)
		envs = append(envs, s.Env)
	}
	wantKeys := []string{
		"server.port", "server.admin_port", "server.dev_mode", "server.dir", "server.pprof",
		"server.http.shutdown_timeout", "server.http.read_header_timeout", "server.http.write_timeout",
		"server.http.idle_timeout", "server.http.max_header_mb",
		"site.default_index", "site.dir_index", "site.edit_url", "site.language",
		"site.meta.title", "site.meta.description", "site.meta.domain", "site.meta.robots",
		"site.theme.name", "site.theme.vars", "site.theme.features",
		"site.highlighting.theme", "site.search.index", "site.exclude", "site.strip_extensions",
	}
	wantEnvs := []string{
		"GOMDDOC_SERVER_PORT", "GOMDDOC_SERVER_ADMIN_PORT", "GOMDDOC_SERVER_DEV_MODE", "GOMDDOC_SERVER_DIR",
		"GOMDDOC_SERVER_PPROF", "GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT", "GOMDDOC_SERVER_HTTP_READ_HEADER_TIMEOUT",
		"GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT", "GOMDDOC_SERVER_HTTP_IDLE_TIMEOUT", "GOMDDOC_SERVER_HTTP_MAX_HEADER_MB",
		"GOMDDOC_SITE_DEFAULT_INDEX", "GOMDDOC_SITE_DIR_INDEX", "GOMDDOC_SITE_EDIT_URL", "GOMDDOC_SITE_LANGUAGE",
		"GOMDDOC_SITE_META_TITLE", "GOMDDOC_SITE_META_DESCRIPTION", "GOMDDOC_SITE_META_DOMAIN",
		"GOMDDOC_SITE_META_ROBOTS", "GOMDDOC_SITE_THEME_NAME", "", "GOMDDOC_SITE_THEME_FEATURES_<KEY>",
		"GOMDDOC_SITE_HIGHLIGHTING_THEME", "GOMDDOC_SITE_SEARCH_INDEX", "", "",
	}
	if !slices.Equal(keys, wantKeys) {
		t.Errorf("keys:\n got %q\nwant %q", keys, wantKeys)
	}
	if !slices.Equal(envs, wantEnvs) {
		t.Errorf("envs:\n got %q\nwant %q", envs, wantEnvs)
	}
}

func TestSchema_FileKeys(t *testing.T) {
	t.Parallel()
	for _, s := range Schema() {
		want, _ := strings.CutPrefix(s.Key, "site.")
		if !strings.HasPrefix(s.Key, "site.") {
			want = ""
		}
		if s.FileKey != want {
			t.Errorf("%s: FileKey = %q, want %q", s.Key, s.FileKey, want)
		}
	}
}

// TestSchema_EveryLeafDocumented is the gate that makes the doc tag mandatory:
// a new config field without one fails here, not in an agent's hands.
func TestSchema_EveryLeafDocumented(t *testing.T) {
	t.Parallel()
	for _, s := range Schema() {
		if len(s.Description) < 20 {
			t.Errorf("%s: doc tag %q is missing or too short to be useful", s.Key, s.Description)
		}
		if s.Type == "" {
			t.Errorf("%s: empty type", s.Key)
		}
	}
}

func TestSchema_Defaults(t *testing.T) {
	t.Parallel()
	byKey := map[string]Setting{}
	for _, s := range Schema() {
		byKey[s.Key] = s
	}
	tests := []struct {
		key  string
		want any
	}{
		{"server.port", DefaultPort},
		{"server.http.write_timeout", DefaultWriteTimeout.String()},
		{"server.http.max_header_mb", DefaultMaxHeaderMB},
		{"site.theme.name", DefaultThemeName},
		{"site.search.index", true},
		{"site.theme.features", nil},
	}
	for _, tt := range tests {
		if got := byKey[tt.key].Default; got != tt.want {
			t.Errorf("%s default = %#v, want %#v", tt.key, got, tt.want)
		}
	}
	// Computed default: documented by note, never a machine-specific value.
	title := byKey["site.meta.title"]
	if title.Default != nil || title.DefaultNote == "" {
		t.Errorf("site.meta.title: Default=%#v DefaultNote=%q, want nil and a note", title.Default, title.DefaultNote)
	}
	if got, _ := byKey["site.strip_extensions"].Default.([]string); !slices.Equal(got, []string{".md"}) {
		t.Errorf("site.strip_extensions default = %#v", byKey["site.strip_extensions"].Default)
	}
}

// TestSchema_MaxMatchesConstants: the max tag documents a bound normalizeHTTP
// enforces through the Max* constants. They must agree.
func TestSchema_MaxMatchesConstants(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"server.http.read_header_timeout": MaxReadHeaderTimeout.String(),
		"server.http.write_timeout":       MaxWriteTimeout.String(),
		"server.http.idle_timeout":        MaxIdleTimeout.String(),
		"server.http.max_header_mb":       strconv.Itoa(MaxMaxHeaderMB),
	}
	for _, s := range Schema() {
		w, bounded := want[s.Key]
		if !bounded {
			if s.Max != "" {
				t.Errorf("%s: unexpected max %q", s.Key, s.Max)
			}
			continue
		}
		got := s.Max
		if s.Type == "duration" {
			d, err := time.ParseDuration(s.Max)
			if err != nil {
				t.Fatalf("%s: max %q: %v", s.Key, s.Max, err)
			}
			got = d.String()
		}
		if got != w {
			t.Errorf("%s: max = %q, want %q", s.Key, got, w)
		}
	}
}

func TestSchema_ReturnsFreshSlice(t *testing.T) {
	t.Parallel()
	a := Schema()
	a[0].Flags = append(a[0].Flags, "--mutated")
	if b := Schema(); len(b[0].Flags) != 0 {
		t.Error("Schema() shares state between calls")
	}
}

// validateAgainst is a structural validator for the subset of JSON Schema that
// JSONSchema emits (type, properties, additionalProperties, items,
// propertyNames.pattern). It exists so the test needs no validator dependency.
func validateAgainst(schema map[string]any, doc any, at string) []string {
	var errs []string
	switch schema["type"] {
	case "object":
		obj, ok := doc.(map[string]any)
		if !ok {
			return []string{at + ": want object"}
		}
		props, _ := schema["properties"].(map[string]any)
		for _, k := range slices.Sorted(maps.Keys(obj)) {
			child := at + "." + k
			if pat, ok := schema["propertyNames"].(map[string]any); ok {
				if !regexp.MustCompile(pat["pattern"].(string)).MatchString(k) {
					errs = append(errs, child+": key does not match propertyNames")
					continue
				}
			}
			if sub, ok := props[k].(map[string]any); ok {
				errs = append(errs, validateAgainst(sub, obj[k], child)...)
				continue
			}
			switch ap := schema["additionalProperties"].(type) {
			case bool:
				if !ap {
					errs = append(errs, child+": unknown key")
				}
			case map[string]any:
				errs = append(errs, validateAgainst(ap, obj[k], child)...)
			}
		}
	case "array":
		arr, ok := doc.([]any)
		if !ok {
			return []string{at + ": want array"}
		}
		for i, v := range arr {
			errs = append(errs, validateAgainst(schema["items"].(map[string]any), v, fmt.Sprintf("%s[%d]", at, i))...)
		}
	case "string":
		if _, ok := doc.(string); !ok {
			errs = append(errs, at+": want string")
		}
	case "boolean":
		if _, ok := doc.(bool); !ok {
			errs = append(errs, at+": want boolean")
		}
	}
	return errs
}

func loadSchema(t *testing.T) map[string]any {
	t.Helper()
	var s map[string]any
	if err := json.Unmarshal(JSONSchema(), &s); err != nil {
		t.Fatalf("JSONSchema is not valid JSON: %v", err)
	}
	return s
}

func TestJSONSchema_Header(t *testing.T) {
	t.Parallel()
	s := loadSchema(t)
	if s["$schema"] != "https://json-schema.org/draft/2020-12/schema" || s["$id"] != "gomddoc://schema/config" {
		t.Errorf("header: $schema=%v $id=%v", s["$schema"], s["$id"])
	}
	if s["additionalProperties"] != false {
		t.Error("root must reject unknown keys")
	}
	if desc, _ := s["description"].(string); !strings.Contains(desc, FileKeysNote) {
		t.Error("root description must carry FileKeysNote")
	}
}

// TestJSONSchema_CoversEveryFileKey: every file-settable setting is reachable
// in the schema, and nothing else is.
func TestJSONSchema_CoversEveryFileKey(t *testing.T) {
	t.Parallel()
	s := loadSchema(t)
	var got []string
	var walk func(node map[string]any, prefix string)
	walk = func(node map[string]any, prefix string) {
		props, _ := node["properties"].(map[string]any)
		for k, v := range props {
			child := v.(map[string]any)
			if _, nested := child["properties"]; nested {
				walk(child, prefix+k+".")
				continue
			}
			got = append(got, prefix+k)
		}
	}
	walk(s, "")
	var want []string
	for _, st := range Schema() {
		if st.FileKey != "" {
			want = append(want, st.FileKey)
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("schema leaves:\n got %q\nwant %q", got, want)
	}
}

func TestJSONSchema_Defaults(t *testing.T) {
	t.Parallel()
	s := loadSchema(t)
	props := s["properties"].(map[string]any)
	if got := props["default_index"].(map[string]any)["default"]; got != DefaultIndex {
		t.Errorf("default_index default = %v", got)
	}
	title := props["meta"].(map[string]any)["properties"].(map[string]any)["title"].(map[string]any)
	if _, has := title["default"]; has || !strings.Contains(title["description"].(string), "Default: the content directory") {
		t.Errorf("meta.title must describe its computed default, got %v", title)
	}
}

func TestJSONSchema_Validates(t *testing.T) {
	t.Parallel()
	s := loadSchema(t)
	testsite, err := os.ReadFile("../../testsite/.gomddoc/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		yaml    string
		wantErr string // "" = valid
	}{
		{"testsite", string(testsite), ""},
		{"every key", "default_index: index.md\ndir_index: true\nedit_url: https://x/\nlanguage: fr-FR\n" +
			"meta: {title: T, description: D, domain: d.example, robots: noindex}\n" +
			"theme: {name: nord, vars: {bg: '#fff'}, features: {toc: false}}\n" +
			"highlighting: {theme: monokai}\nsearch: {index: false}\nexclude: [drafts/]\nstrip_extensions: [.md]\n", ""},
		{"unknown root key", "nope: 1\n", ".nope: unknown key"},
		{"unknown nested key", "meta: {titel: x}\n", ".meta.titel: unknown key"},
		{"site wrapper", "site: {meta: {domain: x}}\n", ".site: unknown key"},
		{"server key in file", "port: ':9000'\n", ".port: unknown key"},
		{"bad feature key", "theme: {features: {Toc: false}}\n", ".theme.features.Toc: key does not match propertyNames"},
		{"feature not bool", "theme: {features: {toc: 'no'}}\n", ".theme.features.toc: want boolean"},
		{"exclude not list", "exclude: drafts/\n", ".exclude: want array"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var doc map[string]any
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatal(err)
			}
			errs := validateAgainst(s, doc, "")
			switch {
			case tt.wantErr == "" && len(errs) > 0:
				t.Errorf("want valid, got %q", errs)
			case tt.wantErr != "" && !slices.Equal(errs, []string{tt.wantErr}):
				t.Errorf("errs = %q, want [%q]", errs, tt.wantErr)
			}
		})
	}
}

func TestJSONSchema_ReturnsCopy(t *testing.T) {
	t.Parallel()
	a := JSONSchema()
	a[0] = 'X'
	if JSONSchema()[0] == 'X' {
		t.Error("JSONSchema returns the cached slice")
	}
}

// TestJSONSchema_NoNullDefaults: a "default": null on a typed property violates
// that property's own type, and editors flag it.
func TestJSONSchema_NoNullDefaults(t *testing.T) {
	t.Parallel()
	var walk func(node map[string]any, at string)
	walk = func(node map[string]any, at string) {
		if d, has := node["default"]; has && d == nil {
			t.Errorf("%s: default is null", at)
		}
		props, _ := node["properties"].(map[string]any)
		for k, v := range props {
			walk(v.(map[string]any), at+"."+k)
		}
	}
	walk(loadSchema(t), "")
}
