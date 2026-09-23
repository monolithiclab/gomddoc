package config

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
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
