package main

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/monolithiclab/gomddoc/internal/config"
)

// InfoCmd holds flags for the info subcommand.
type InfoCmd struct{}

// Run executes the info command.
func (i *InfoCmd) Run() error {
	w := os.Stdout

	fmt.Fprintln(w, "gomddoc")
	fmt.Fprintf(w, "version: %s\n", version)

	fmt.Fprintln(w, "Config file:")
	fmt.Fprintln(w, "  .gomddoc/config.yml (relative to content directory)")
	fmt.Fprintln(w, "See gomddoc init for automatic config file generation")

	writeEnvVarsHelp(w)
	return nil
}

// writeEnvVarsHelp writes the environment variables help section to w.
func writeEnvVarsHelp(w io.Writer) {
	fmt.Fprintln(w, "Environment variables:")
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	for _, s := range config.Schema() {
		if s.Env == "" {
			continue
		}
		def := fmt.Sprintf("%v", s.Default)
		if s.Default == nil || def == "" {
			def = "(empty)"
		}
		fmt.Fprintf(tw, "  %s\t%s\t(default: %s)\n", s.Env, s.Type, def)
	}
	tw.Flush() // #nosec G104
}
