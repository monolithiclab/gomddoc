// Package mcp exposes the content tree to agents over the Model Context
// Protocol: six read-only tools, docs:// resources, and a set of prompts —
// plus, when ServerDeps.SelfDocs is set, gomddoc's own capabilities and guide
// under gomddoc://. Only stdio sets it: a public site's HTTP endpoint does not
// advertise its generator's manual.
//
// Every entry point that reads a path runs it through
// provider.IsRestrictedPath first, so a hidden or excluded file is as
// unreachable here as it is over HTTP. get_table_of_contents is not an
// exception to that: it reads no path, serving the pipeline's already
// exclude-filtered tree. Cached objects — the navigation
// generator above all — are taken from the pipeline rather than constructed
// per call: a fresh generator re-walks the content and line-scans every file
// for its title.
package mcp

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/monolithiclab/gomddoc/internal/capabilities"
	"github.com/monolithiclab/gomddoc/internal/doctor"
	"github.com/monolithiclab/gomddoc/internal/guide"
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

	// SelfDocs, when non-nil, registers the gomddoc:// namespace. `gomddoc
	// mcp` (stdio, the operator's agent) sets it; the HTTP mount in serve
	// passes nil, because a public site's readers have no use for the
	// generator's manual.
	SelfDocs *SelfDocs

	Version string
}

// SelfDocs is gomddoc describing itself: the capabilities report, built once at
// startup, and the embedded guide it lists.
type SelfDocs struct {
	Report capabilities.Report
	Guide  fs.FS

	// Doctor checks the served site afresh — config and content reloaded
	// from disk, or the provider's clone for a Git source — on every call.
	// Nil leaves gomddoc_doctor unregistered.
	Doctor func(ctx context.Context, verbose bool) doctor.Report
}

// MCPServer wraps the MCP SDK server with gomddoc-specific handlers.
type MCPServer struct {
	server *mcp.Server
	deps   ServerDeps

	// guide is the embedded guide (nil unless SelfDocs is set); guideErr is
	// why it could not be indexed, reported by every tool that needs it.
	guide    *guide.Guide
	guideErr error
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
	if deps.SelfDocs != nil {
		s.guide, s.guideErr = guide.New(deps.SelfDocs.Guide)
		s.registerSelfDocs()
	}
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
