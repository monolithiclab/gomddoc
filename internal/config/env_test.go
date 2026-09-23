package config

import (
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/diag"
	"github.com/monolithiclab/gomddoc/internal/testutil/logcapture"
)

// No t.Parallel: t.Setenv.
func TestApplyEnvOverrides_InvalidValues(t *testing.T) {
	t.Setenv("GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT", "abc")
	t.Setenv("GOMDDOC_SERVER_HTTP_MAX_HEADER_MB", "x")
	t.Setenv("GOMDDOC_SITE_SEARCH_INDEX", "maybe")
	c := New()
	got := c.ApplyEnvOverrides()
	var keys []string
	for _, f := range got {
		keys = append(keys, f.Key)
		if f.Code != "env.invalid-value" || f.Severity != diag.Warning || f.File != "" || f.Fix == "" {
			t.Errorf("finding = %+v", f)
		}
	}
	want := []string{"GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT", "GOMDDOC_SERVER_HTTP_MAX_HEADER_MB", "GOMDDOC_SITE_SEARCH_INDEX"}
	if !slices.Equal(keys, want) {
		t.Errorf("keys = %q, want %q", keys, want)
	}
	for _, f := range got {
		if f.Key == "GOMDDOC_SITE_SEARCH_INDEX" && !strings.Contains(f.Message, "bool") {
			t.Errorf("message %q does not name the expected type", f.Message)
		}
	}
	if c.Server.HTTP.WriteTimeout != DefaultWriteTimeout || !c.Site.Search.Index {
		t.Error("an unparseable value must leave the field as it was")
	}
}

// TestNewFromServeArgs_LogsEnvFindingOnce: the Config pass and the Site pass
// both read GOMDDOC_SITE_*; a bad value is still one log line.
func TestNewFromServeArgs_LogsEnvFindingOnce(t *testing.T) {
	t.Setenv("GOMDDOC_SITE_SEARCH_INDEX", "maybe")
	log := logcapture.Install(t, slog.LevelDebug)
	if _, err := NewFromServeArgs(ServeArgs{Dir: t.TempDir(), Port: DefaultPort}); err != nil {
		t.Fatal(err)
	}
	n := 0
	for l := range strings.SplitSeq(log.String(), "\n") {
		if strings.HasPrefix(l, "WARN ") && strings.Contains(l, "key=GOMDDOC_SITE_SEARCH_INDEX") && strings.Contains(l, "code=env.invalid-value") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("logged %d times, want once:\n%s", n, log)
	}
}
