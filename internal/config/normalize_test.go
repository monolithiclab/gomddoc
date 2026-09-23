package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/monolithiclab/gomddoc/internal/diag"
	"github.com/monolithiclab/gomddoc/internal/testutil/logcapture"
)

func TestNormalize_Findings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		mutate   func(c *Config)
		key      string
		file     string
		contains []string // configured value and the value used instead
		check    func(c *Config) bool
	}{
		{"empty theme", func(c *Config) { c.Site.Theme.Name = "" }, "theme.name", ConfigFile,
			[]string{DefaultThemeName}, func(c *Config) bool { return c.Site.Theme.Name == DefaultThemeName }},
		{"read header zero", func(c *Config) { c.Server.HTTP.ReadHeaderTimeout = 0 }, "server.http.read_header_timeout", "",
			[]string{"0s", "5s"}, func(c *Config) bool { return c.Server.HTTP.ReadHeaderTimeout == DefaultReadHeaderTimeout }},
		{"read header too long", func(c *Config) { c.Server.HTTP.ReadHeaderTimeout = 2 * time.Minute }, "server.http.read_header_timeout", "",
			[]string{"2m0s", "1m0s", "5s"}, func(c *Config) bool { return c.Server.HTTP.ReadHeaderTimeout == DefaultReadHeaderTimeout }},
		{"write negative", func(c *Config) { c.Server.HTTP.WriteTimeout = -time.Second }, "server.http.write_timeout", "",
			[]string{"-1s", "30s"}, func(c *Config) bool { return c.Server.HTTP.WriteTimeout == DefaultWriteTimeout }},
		{"write too long", func(c *Config) { c.Server.HTTP.WriteTimeout = 10 * time.Minute }, "server.http.write_timeout", "",
			[]string{"10m0s", "5m0s", "30s"}, func(c *Config) bool { return c.Server.HTTP.WriteTimeout == DefaultWriteTimeout }},
		{"idle zero", func(c *Config) { c.Server.HTTP.IdleTimeout = 0 }, "server.http.idle_timeout", "",
			[]string{"0s", "2m0s"}, func(c *Config) bool { return c.Server.HTTP.IdleTimeout == DefaultIdleTimeout }},
		{"idle too long", func(c *Config) { c.Server.HTTP.IdleTimeout = time.Hour }, "server.http.idle_timeout", "",
			[]string{"1h0m0s", "10m0s"}, func(c *Config) bool { return c.Server.HTTP.IdleTimeout == DefaultIdleTimeout }},
		{"header zero", func(c *Config) { c.Server.HTTP.MaxHeaderMB = 0 }, "server.http.max_header_mb", "",
			[]string{"0", "1"}, func(c *Config) bool { return c.Server.HTTP.MaxHeaderMB == DefaultMaxHeaderMB }},
		{"header too big", func(c *Config) { c.Server.HTTP.MaxHeaderMB = 50 }, "server.http.max_header_mb", "",
			[]string{"50", "10"}, func(c *Config) bool { return c.Server.HTTP.MaxHeaderMB == DefaultMaxHeaderMB }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := New()
			tt.mutate(c)
			got := c.Normalize()
			if len(got) != 1 {
				t.Fatalf("findings = %+v, want exactly one", got)
			}
			f := got[0]
			if f.Code != "config.value-replaced" || f.Severity != diag.Warning || f.Key != tt.key || f.File != tt.file || f.Fix == "" {
				t.Errorf("finding = %+v", f)
			}
			for _, w := range tt.contains {
				if !strings.Contains(f.Message, w) {
					t.Errorf("message %q lacks %q", f.Message, w)
				}
			}
			if !tt.check(c) {
				t.Error("value was not replaced")
			}
		})
	}
	if got := New().Normalize(); len(got) != 0 {
		t.Errorf("defaults produce findings: %+v", got)
	}
}

// TestNewFromServeArgs_LogsNormalizeFindings: moving the warning out of
// Normalize must not lose it from serve's log, nor change its level.
func TestNewFromServeArgs_LogsNormalizeFindings(t *testing.T) {
	t.Setenv("GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT", "10m")
	log := logcapture.Install(t, slog.LevelDebug)
	if _, err := NewFromServeArgs(ServeArgs{Dir: t.TempDir(), Port: DefaultPort}); err != nil {
		t.Fatal(err)
	}
	if !log.HasContaining(slog.LevelWarn, "WriteTimeout", slog.String("code", "config.value-replaced"),
		slog.String("key", "server.http.write_timeout")) {
		t.Errorf("replacement not logged at Warn with its code:\n%s", log)
	}
}
