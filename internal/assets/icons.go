package assets

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/diag"
)

// ManifestIcon is an icon the generated web manifest lists.
type ManifestIcon struct {
	Name, Sizes, Purpose string
}

// ManifestIcons are the Android icons of the shared set, in manifest order.
var ManifestIcons = []ManifestIcon{
	{"android-chrome-192x192.png", "192x192", "any"},
	{"android-chrome-512x512.png", "512x512", "any"},
	{"maskable-icon-192x192.png", "192x192", "maskable"},
	{"maskable-icon-512x512.png", "512x512", "maskable"},
}

// IconFiles is the icon set the shared static layer ships: what the
// head-icons partial links, then the manifest's icons.
var IconFiles = append([]string{"favicon.ico", "favicon.svg", "apple-touch-icon.png"}, func() []string {
	names := make([]string, len(ManifestIcons))
	for i, icon := range ManifestIcons {
		names[i] = icon.Name
	}
	return names
}()...)

// IconFindings warns when the site supplies part of the icon set and leaves
// the rest to gomddoc's built-in one: a browser that picks a file the site
// did not replace (Chromium prefers favicon.svg over favicon.ico) shows the
// gomddoc icon. The site's files are those under .gomddoc/ in any static
// layer BuildStaticFS stacks for themeName.
func IconFindings(contentRoot fs.FS, themeName string) []diag.Finding {
	var site, builtin []string
	first := ""
	for _, name := range IconFiles {
		found := ""
		for _, dir := range staticLayerDirs(themeName) {
			if p := path.Join(config.ConfigDirName, dir, name); fileExists(contentRoot, p) {
				found = p
				break
			}
		}
		if found == "" {
			builtin = append(builtin, name)
			continue
		}
		if first == "" {
			first = found
		}
		site = append(site, name)
	}
	if len(site) == 0 || len(builtin) == 0 {
		return nil
	}
	return []diag.Finding{diag.New("theme.icons-mixed", first, 0, "",
		fmt.Sprintf("the site replaces %s but not %s, so browsers that pick those show gomddoc's icon",
			strings.Join(site, ", "), strings.Join(builtin, ", ")),
		fmt.Sprintf("add %s next to %s", strings.Join(builtin, ", "), path.Dir(first)))}
}

func fileExists(fsys fs.FS, name string) bool {
	_, err := fs.Stat(fsys, name)
	return err == nil
}
