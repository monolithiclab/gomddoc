package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"

	"github.com/alecthomas/kong"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
)

//go:embed assets
var embeddedAssets embed.FS

// version is set at build time via ldflags.
var version = "dev"

// CLI is the top-level Kong command struct.
type CLI struct {
	Version kong.VersionFlag `name:"version" help:"Show version and exit."`
	Serve   ServeCmd         `cmd:"" help:"Start the HTTP server to serve markdown files as HTML."`
	Build   BuildCmd         `cmd:"" help:"Build a static site from markdown files."`
}

// infoProviderAdapter adapts provider.Provider to breadcrumb.InfoProvider
type infoProviderAdapter struct {
	p provider.Provider
}

func (pa *infoProviderAdapter) IsDir(path string) bool {
	info, err := pa.p.Stat(context.Background(), path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Ensure infoProviderAdapter implements breadcrumb.InfoProvider at compile time.
var _ breadcrumb.InfoProvider = (*infoProviderAdapter)(nil)

// helpPrinter wraps Kong's default help to append environment variables for subcommands.
func helpPrinter(options kong.HelpOptions, ctx *kong.Context) error {
	if err := kong.DefaultHelpPrinter(options, ctx); err != nil {
		return err
	}

	if ctx.Command() != "serve" {
		return nil
	}

	vars := config.EnvVars()
	fmt.Fprintln(ctx.Stdout)
	fmt.Fprintln(ctx.Stdout, "Environment variables:")
	w := tabwriter.NewWriter(ctx.Stdout, 0, 0, 3, ' ', 0)
	for _, v := range vars {
		def := v.DefaultValue
		if def == "" {
			def = "(empty)"
		}
		fmt.Fprintf(w, "  %s\t%s\t(default: %s)\n", v.Name, v.Type, def)
	}
	return w.Flush()
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
