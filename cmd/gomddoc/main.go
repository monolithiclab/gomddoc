package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
)

//go:embed assets
var embeddedAssets embed.FS

// version is set at build time via ldflags.
var version = "dev"

// CLI is the top-level Kong command struct.
type CLI struct {
	Version kong.VersionFlag `name:"version" help:"Show version and exit."`
	Build   BuildCmd         `cmd:"" help:"Build a static site from markdown files."`
	Info    InfoCmd          `cmd:"" help:"Show environment variables, config file location, and version."`
	Init    InitCmd          `cmd:"" help:"Initialize a .gomddoc/ directory with default configuration."`
	MCP     MCPCmd           `cmd:"" help:"Start MCP server for AI model integration (stdio transport)."`
	Preview PreviewCmd       `cmd:"" help:"Quick local preview with auto-port and browser open."`
	Serve   ServeCmd         `cmd:"" help:"Start the HTTP server to serve markdown files as HTML."`
}

func main() {
	cli := CLI{}
	ctx := kong.Parse(&cli,
		kong.Name("gomddoc"),
		kong.Description("A production-ready HTTP server that serves Markdown files as HTML."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
	)
	if err := ctx.Run(); err != nil {
		slog.Error("Fatal error", slog.Any("error", err))
		os.Exit(1)
	}
}
