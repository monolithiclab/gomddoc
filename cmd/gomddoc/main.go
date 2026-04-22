package main

import (
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"text/tabwriter"

	"github.com/alecthomas/kong"

	"github.com/monolithiclab/gomddoc/internal/config"
)

//go:embed assets
var embeddedAssets embed.FS

// version is set at build time via ldflags.
var version = "dev"

// CLI is the top-level Kong command struct.
type CLI struct {
	Version kong.VersionFlag `name:"version" help:"Show version and exit."`
	Build   BuildCmd         `cmd:"" help:"Build a static site from markdown files."`
	Init    InitCmd          `cmd:"" help:"Initialize a .gomddoc/ directory with default configuration."`
	MCP     MCPCmd           `cmd:"" help:"Start MCP server for AI model integration (stdio transport)."`
	Preview PreviewCmd       `cmd:"" help:"Quick local preview with auto-port and browser open."`
	Serve   ServeCmd         `cmd:"" help:"Start the HTTP server to serve markdown files as HTML."`
}

// helpPrinter wraps Kong's default help to append environment variables for subcommands.
func helpPrinter(options kong.HelpOptions, ctx *kong.Context) error {
	if err := kong.DefaultHelpPrinter(options, ctx); err != nil {
		return err
	}
	return writeEnvVarsHelp(ctx.Stdout)
}

// writeEnvVarsHelp writes the environment variables help section to w.
func writeEnvVarsHelp(w io.Writer) error {
	vars := config.EnvVars()
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Environment variables:")
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	for _, v := range vars {
		def := v.DefaultValue
		if def == "" {
			def = "(empty)"
		}
		fmt.Fprintf(tw, "  %s\t%s\t(default: %s)\n", v.Name, v.Type, def)
	}
	return tw.Flush()
}

func main() {
	cli := CLI{}
	ctx := kong.Parse(&cli,
		kong.Name("gomddoc"),
		kong.Description("A production-ready HTTP server that serves Markdown files as HTML."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
		kong.Help(helpPrinter),
	)
	if err := ctx.Run(); err != nil {
		slog.Error("Fatal error", slog.Any("error", err))
		os.Exit(1)
	}
}
