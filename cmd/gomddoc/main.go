// Command gomddoc serves a directory of Markdown files as HTML over HTTP,
// renders it to a static site, or exposes it to agents over MCP.
//
// The four content subcommands — serve, preview, build and mcp — funnel
// through setupPipeline, which always builds the path resolver and adds the
// metadata index, navigation generator and search index for whichever
// subcommand asked for them. Each is built from Pipeline.Exclude, never
// cfg.Site.Exclude:
// a per-language pipeline excludes the directories the other languages own, so
// an index reading the raw config walks content its own resolver knows nothing
// about.
package main

import (
	"embed"
	"errors"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
)

//go:embed assets
var embeddedAssets embed.FS

// CLI is the top-level Kong command struct.
type CLI struct {
	Version kong.VersionFlag `name:"version" help:"Show version and exit."`
	Build   BuildCmd         `cmd:"" help:"Build a static site from markdown files."`
	Doctor  DoctorCmd        `cmd:"" help:"Check a site's configuration and content; report every problem with a fix."`
	Info    InfoCmd          `cmd:"" help:"Describe gomddoc: settings, env vars, flags, theme features and guide pages (--json for agents)."`
	Init    InitCmd          `cmd:"" help:"Initialize a .gomddoc/ directory with default configuration."`
	MCP     MCPCmd           `cmd:"" help:"Start MCP server for AI model integration (stdio). Also serves gomddoc's own guide and capabilities under gomddoc://."`
	Preview PreviewCmd       `cmd:"" help:"Quick local preview with auto-port and browser open."`
	Schema  SchemaCmd        `cmd:"" help:"Print the JSON Schema for .gomddoc/config.yml."`
	Serve   ServeCmd         `cmd:"" help:"Start the HTTP server to serve markdown files as HTML."`
}

// parserOptions configures the Kong parser. Tests build the model through the
// same options, so the commands the capabilities report lists are the ones a
// user runs.
func parserOptions() []kong.Option {
	return []kong.Option{
		kong.Name("gomddoc"),
		kong.Description("A production-ready HTTP server that serves Markdown files as HTML."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
	}
}

func main() {
	cli := CLI{}
	ctx := kong.Parse(&cli, parserOptions()...)
	// Binding the model lets a command's Run take *kong.Application (info and
	// mcp describe the CLI); commands whose Run takes nothing are unaffected.
	if err := ctx.Run(ctx.Model); err != nil {
		// A command that already printed its result (doctor) only sets the status.
		if exit, ok := errors.AsType[exitCodeError](err); ok {
			os.Exit(int(exit))
		}
		slog.Error("Fatal error", slog.Any("error", err))
		os.Exit(1)
	}
}
