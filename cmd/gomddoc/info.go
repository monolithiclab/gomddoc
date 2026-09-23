package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/alecthomas/kong"

	"github.com/monolithiclab/gomddoc/internal/capabilities"
	"github.com/monolithiclab/gomddoc/internal/config"
)

// InfoCmd describes gomddoc and, when run in a site, that site's configuration.
type InfoCmd struct {
	Dir  string `arg:"" optional:"" default:"." env:"GOMDDOC_SERVER_DIR" help:"Content directory to inspect. Git URLs are not cloned; defaults are shown."`
	JSON bool   `name:"json" help:"Print the capabilities report as JSON (the same document as the MCP resource gomddoc://capabilities)."`

	out io.Writer // nil: os.Stdout
}

// Run prints the report. It never fails on a bad site: a config that does not
// load is the report's content, not the command's error.
func (i *InfoCmd) Run(app *kong.Application) error {
	var (
		cfg    *config.Config
		cfgErr error
		root   fs.FS
	)
	if config.IsGitURL(i.Dir) {
		cfgErr = errors.New("git sources are not inspected by info; showing defaults")
	} else {
		cfg, cfgErr = config.NewFromDir(i.Dir)
		root = os.DirFS(i.Dir)
	}
	report := capabilities.Describe(capabilitiesInput(app, i.Dir, cfg, cfgErr, root))

	w := i.out
	if w == nil {
		w = os.Stdout
	}
	if i.JSON {
		_, err := fmt.Fprintf(w, "%s\n", report.JSON())
		return err
	}
	return writeInfo(w, report)
}

// writeInfo renders the report for a human reader. Everything here is also in
// the JSON form; this is a view of it, not a second source.
func writeInfo(w io.Writer, r capabilities.Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "gomddoc %s\n\n", r.Version)
	writeConfigSection(&b, r.Config)
	writeSettingsSection(&b, r.Settings)

	b.WriteString("\nCommands:\n")
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, c := range r.Commands {
		fmt.Fprintf(tw, "  %s\t%s\n", c.Name, c.Help)
	}
	tw.Flush() // #nosec G104 -- strings.Builder cannot fail

	fmt.Fprintf(&b, "\nTheme %s (%s):\n", r.Theme.Name, r.Theme.Source)
	fmt.Fprintf(&b, "  features: %s\n", listOrNone(r.Theme.Features))
	fmt.Fprintf(&b, "  vars:     %s\n", listOrNone(r.Theme.Vars))
	fmt.Fprintf(&b, "  docs:     %s\n", r.Theme.Docs)

	b.WriteString("\nGuide (gomddoc mcp serves it as gomddoc://guide/<path>):\n")
	tw = tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, p := range r.Guide {
		fmt.Fprintf(tw, "  %s\t%s\n", p.Path, p.Title)
	}
	tw.Flush() // #nosec G104 -- strings.Builder cannot fail
	if r.GuideError != "" {
		fmt.Fprintf(&b, "  (guide index failed: %s)\n", r.GuideError)
	}

	b.WriteString("\nMachine-readable: gomddoc info --json · config schema: gomddoc schema\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func writeConfigSection(b *strings.Builder, c capabilities.ConfigInfo) {
	found := strings.ReplaceAll(c.FileStatus, "_", " ")
	status := "loaded"
	if !c.Loaded {
		status = "not loaded: " + c.LoadError
	}
	fmt.Fprintf(b, "Config: %s/%s (%s), %s\n", c.Dir, c.File, found, status)
	fmt.Fprintf(b, "Precedence: %s\n", strings.Join(c.Precedence, " > "))
	fmt.Fprintf(b, "%s\n", c.FileKeys)
}

// writeSettingsSection prints one row per setting, grouped by the key's first
// two segments (site.meta, server.http, …), with the description underneath.
func writeSettingsSection(b *strings.Builder, settings []config.Setting) {
	b.WriteString("\nSettings:\n")
	group := ""
	for _, s := range settings {
		parts := strings.SplitN(s.Key, ".", 3)
		if g := strings.Join(parts[:min(2, len(parts)-1)], "."); g != group {
			group = g
			fmt.Fprintf(b, "\n  [%s]\n", group)
		}
		def := fmt.Sprintf("%v", s.Default)
		switch {
		case s.DefaultNote != "":
			def = s.DefaultNote
		case s.Default == nil || def == "":
			def = "(empty)"
		}
		fmt.Fprintf(b, "  %s\n", s.Key)
		fmt.Fprintf(b, "      env: %s  flags: %s  default: %s\n", orNone(s.Env), listOrNone(s.Flags), def)
		fmt.Fprintf(b, "      %s\n", s.Description)
	}
}

func orNone(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func listOrNone(l []string) string {
	if len(l) == 0 {
		return "-"
	}
	return strings.Join(l, ", ")
}
