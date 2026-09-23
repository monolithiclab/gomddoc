package main

import (
	"cmp"
	"io/fs"
	"path"
	"slices"

	"github.com/alecthomas/kong"

	"github.com/monolithiclab/gomddoc/docs"
	"github.com/monolithiclab/gomddoc/internal/assets"
	"github.com/monolithiclab/gomddoc/internal/capabilities"
	"github.com/monolithiclab/gomddoc/internal/config"
)

// commandsFromKong converts Kong's model into the capabilities form, so
// internal/capabilities does not depend on the CLI library. Commands are
// sorted by name; hidden commands and flags are left out. Kong's --help and
// --version live on the application node, not on subcommands, so they never
// appear here.
func commandsFromKong(app *kong.Application) []capabilities.Command {
	var cmds []capabilities.Command
	for _, n := range app.Children {
		if n.Hidden {
			continue
		}
		c := capabilities.Command{Name: n.Name, Help: n.Help}
		for _, p := range n.Positional {
			c.Args = append(c.Args, capabilities.Arg{
				Name: p.Name, Help: p.Help, Default: p.Default, Env: firstOr(p.Tag.Envs), Required: p.Required,
			})
		}
		for _, f := range n.Flags {
			if f.Hidden {
				continue
			}
			short := ""
			if f.Short != 0 {
				short = "-" + string(f.Short)
			}
			c.Flags = append(c.Flags, capabilities.Flag{
				Name: "--" + f.Name, Short: short, Help: f.Help, Default: f.Default,
				Env: firstOr(f.Envs), Setting: f.Tag.Get("setting"),
			})
		}
		cmds = append(cmds, c)
	}
	slices.SortFunc(cmds, func(a, b capabilities.Command) int { return cmp.Compare(a.Name, b.Name) })
	return cmds
}

func firstOr(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

// capabilitiesInput builds the report input for dir. cfg and cfgErr are
// whatever the caller's config load produced. contentRoot is nil when the
// caller has no content to look at (info on a Git URL); the theme is then
// scanned from the embedded assets alone.
func capabilitiesInput(app *kong.Application, dir string, cfg *config.Config, cfgErr error, contentRoot fs.FS) capabilities.Input {
	in := capabilities.Input{
		Version: version, Dir: dir, Config: cfg, ConfigErr: cfgErr,
		Commands: commandsFromKong(app), Guide: docs.Guide, Assets: embeddedAssets,
	}
	if contentRoot != nil {
		_, err := fs.Stat(contentRoot, path.Join(config.ConfigDirName, config.ConfigFileName))
		in.FileFound = err == nil
		in.Assets = assets.BuildFS(contentRoot, embeddedAssets)
	}
	return in
}
