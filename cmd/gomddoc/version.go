package main

import (
	"runtime/debug"
	"strings"
)

// devVersion is the value of version in any build that carries no version
// information: plain `go build`, `go run`, and tests.
const devVersion = "dev"

// version is injected at build time via -ldflags "-X main.version=..." by
// GoReleaser and the Makefile. `go install` applies no ldflags, so it is
// resolved from the module version the toolchain stamps into the binary's
// build info instead. Kept as a plain constant-initialized string variable:
// -X only takes effect on variables declared uninitialized or initialized to
// a constant string expression.
var version = devVersion

func init() {
	version = resolveVersion(version, debug.ReadBuildInfo)
}

// resolveVersion normalizes the linker-injected version, falling back to the
// main module's version from build info when no ldflags were applied.
//
// readBuildInfo is a parameter rather than a direct call to debug.ReadBuildInfo
// so tests can exercise the fallback: a test binary's own build info always
// reports "(devel)".
func resolveVersion(injected string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if injected != devVersion && injected != "" {
		return normalizeVersion(injected)
	}

	bi, ok := readBuildInfo()
	// "(devel)" is what the toolchain reports for a module built from a working
	// tree rather than installed at a version — no more informative than "dev".
	if !ok || bi.Main.Version == "" || bi.Main.Version == "(devel)" {
		return devVersion
	}
	return normalizeVersion(bi.Main.Version)
}

// normalizeVersion strips the Go module "v" prefix so every build path reports
// the same string: GoReleaser injects an unprefixed "0.1.1", the Makefile
// injects `git describe` output ("v0.1.1-2-gabc1234"), and build info carries
// module versions ("v0.1.1").
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}
