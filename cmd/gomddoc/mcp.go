package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/mcp"
	"github.com/monolithiclab/gomddoc/internal/provider"
)

// MCPCmd holds flags for the mcp subcommand.
type MCPCmd struct {
	Dir       string `arg:"" optional:"" default:"." env:"GOMDDOC_SERVER_DIR" help:"Markdown directory or Git URL."`
	GitSSHKey string `name:"git-key-file" default:"" env:"GOMDDOC_SERVER_GIT_SSH_KEY" help:"Path to SSH private key file for Git authentication."`
}

// Run executes the mcp command (stdio transport).
func (m *MCPCmd) Run() error {
	cfg, err := config.NewFromDir(m.Dir)
	if err != nil {
		return err
	}

	var gitCfg provider.GitProviderConfig
	if m.GitSSHKey != "" {
		gitCfg.SSHKeyFile = m.GitSSHKey
	}

	prov, err := provider.NewProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, cfg.Site.Exclude, gitCfg)
	if err != nil {
		return err
	}
	defer prov.Close()

	pipeline, err := setupPipeline(cfg, prov, PipelineOptions{
		EnableCache:      true,
		EnableNavigation: false,
		EnableMetadata:   true,
		EnableSearch:     true,
	})
	if err != nil {
		return err
	}

	mcpServer := mcp.NewServer(mcp.ServerDeps{
		Provider:        prov,
		MetaIndex:       pipeline.MetaIndex,
		SearchIndex:     pipeline.SearchIndex,
		DefaultIndex:    cfg.Site.DefaultIndex,
		ExcludePatterns: cfg.Site.Exclude,
		SiteName:        cfg.Site.Meta.Title,
		Version:         version,
	})

	sigCtx, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	return mcpServer.Run(sigCtx)
}
