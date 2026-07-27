package main

import (
	"runtime/debug"
	"strings"
	"testing"
)

func buildInfo(modVersion string) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: modVersion}}, true
	}
}

func noBuildInfo() (*debug.BuildInfo, bool) { return nil, false }

func TestResolveVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		injected      string
		readBuildInfo func() (*debug.BuildInfo, bool)
		want          string
	}{
		{
			name:          "ldflags version wins over build info",
			injected:      "0.1.1",
			readBuildInfo: buildInfo("v9.9.9"),
			want:          "0.1.1",
		},
		{
			name:          "ldflags version is normalized",
			injected:      "v0.1.1-2-gabc1234",
			readBuildInfo: buildInfo("v9.9.9"),
			want:          "0.1.1-2-gabc1234",
		},
		{
			name:          "go install falls back to module version",
			injected:      devVersion,
			readBuildInfo: buildInfo("v0.1.1"),
			want:          "0.1.1",
		},
		{
			name:          "go install falls back to pseudo-version",
			injected:      devVersion,
			readBuildInfo: buildInfo("v0.0.0-20260725120000-abcdef123456"),
			want:          "0.0.0-20260725120000-abcdef123456",
		},
		{
			name:          "working-tree build stays dev",
			injected:      devVersion,
			readBuildInfo: buildInfo("(devel)"),
			want:          devVersion,
		},
		{
			name:          "empty module version stays dev",
			injected:      devVersion,
			readBuildInfo: buildInfo(""),
			want:          devVersion,
		},
		{
			name:          "unreadable build info stays dev",
			injected:      devVersion,
			readBuildInfo: noBuildInfo,
			want:          devVersion,
		},
		{
			name:          "empty ldflags value falls back to build info",
			injected:      "",
			readBuildInfo: buildInfo("v0.1.1"),
			want:          "0.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveVersion(tt.injected, tt.readBuildInfo); got != tt.want {
				t.Errorf("resolveVersion(%q) = %q, want %q", tt.injected, got, tt.want)
			}
		})
	}
}

// The package-level version must always hold something printable: it is
// reported by `gomddoc --version`, `gomddoc info`, the generator meta tag, and
// the MCP server handshake.
func TestVersionIsResolved(t *testing.T) {
	t.Parallel()

	if version == "" {
		t.Fatal("version is empty")
	}
	// Which value init lands on depends on how the test binary was built (a
	// test binary usually reports "(devel)", but a VCS-stamped one does not),
	// so assert only the invariant every build path must satisfy.
	if strings.HasPrefix(version, "v") {
		t.Errorf("version = %q, want no leading %q", version, "v")
	}
}
