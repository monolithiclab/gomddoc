package mcp

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/search"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

// ServerDeps holds all dependencies for creating an MCP server.
type ServerDeps struct {
	Provider    provider.Provider
	MetaIndex   *metadata.Index
	SearchIndex *search.Index

	// NavGenerator is the pipeline's navigation generator, shared rather than
	// rebuilt: its tree is cached behind a sync.Once and its titles come from
	// the metadata index. get_table_of_contents returns "no navigation tree"
	// when it is nil. DefaultIndex and Resolver are baked into it at
	// construction, which is why they are not fields here.
	NavGenerator *navigation.Generator

	// ExcludePatterns gates direct reads (IsRestrictedPath). This is
	// cfg.Site.Exclude, the author's access-control intent — deliberately not
	// the pipeline's effective exclude. Under `serve` the two differ: the
	// default pipeline additionally hides every detected language directory, so
	// translated pages are absent from get_table_of_contents (as they already
	// were from list_pages and search_docs, which read that pipeline's indexes)
	// yet remain readable by path through read_page. They are not secret.
	ExcludePatterns []string

	SiteName string
	Version  string
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
