// Package docs embeds gomddoc's user guide so the binary can serve its own
// documentation (gomddoc info, the gomddoc:// MCP namespace) with no network
// access and no checkout.
//
// It lives in docs/ only because go:embed cannot reach a parent directory.
// Guide is rooted at docs/guide/ — paths are "02-configuration.md", not
// "guide/02-configuration.md" — and it is the only thing in this package: the
// Go code under docs/skills/ is tooling and is excluded from coverage and
// vulncheck by path prefix, which this package deliberately is not.
package docs

import (
	"embed"
	"io/fs"
)

//go:embed guide
var guideFS embed.FS

// Guide is gomddoc's user guide, rooted at docs/guide/.
var Guide = mustSub(guideFS, "guide")

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic("docs: " + err.Error()) // only reachable with a malformed embed directive
	}
	return sub
}
