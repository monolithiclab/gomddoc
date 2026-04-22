package mcp

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/search"
)

// ServerDeps holds all dependencies for creating an MCP server.
type ServerDeps struct {
	Provider     provider.Provider
	MetaIndex    *metadata.Index
	SearchIndex  *search.Index
	ContentRoot  fs.FS
	DefaultIndex string
	SiteName     string
	Version      string
}

// MCPServer wraps the MCP SDK server with gomddoc-specific handlers.
type MCPServer struct {
	server *mcp.Server
	deps   ServerDeps
}

// NewServer creates a new MCP server wired to gomddoc internals.
func NewServer(deps ServerDeps) *MCPServer {
	version := deps.Version
	if version == "" {
		version = "dev"
	}

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "gomddoc",
			Version: version,
		},
		nil,
	)

	s := &MCPServer{server: server, deps: deps}
	s.registerResources()
	s.registerTools()
	s.registerPrompts()
	return s
}

// Run starts the server on stdio transport (blocks until context cancellation).
func (s *MCPServer) Run(ctx context.Context) error {
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

// HTTPHandler returns an http.Handler for Streamable HTTP transport.
func (s *MCPServer) HTTPHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcp.Server { return s.server },
		nil,
	)
}

// Server returns the underlying MCP server for testing.
func (s *MCPServer) Server() *mcp.Server {
	return s.server
}
