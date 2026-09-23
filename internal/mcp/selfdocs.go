package mcp

import (
	"context"
	"io/fs"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/monolithiclab/gomddoc/internal/capabilities"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/text"
)

func (s *MCPServer) registerSelfDocs() {
	s.server.AddResource(&mcp.Resource{
		URI:         capabilities.CapabilitiesURI,
		Name:        "gomddoc Capabilities",
		Title:       "gomddoc Capabilities",
		Description: "What this gomddoc binary can do and how it is configured: every setting (config key, env var, flags, default), commands, frontmatter fields, the active theme's features, guide pages.",
		MIMEType:    "application/json",
	}, s.handleCapabilitiesResource)

	s.server.AddResource(&mcp.Resource{
		URI:         capabilities.SchemaURI,
		Name:        "gomddoc Config Schema",
		Title:       "gomddoc Config Schema",
		Description: "JSON Schema for .gomddoc/config.yml. Read before writing a config file.",
		MIMEType:    "application/schema+json",
	}, s.handleSchemaResource)

	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: capabilities.GuideURIPrefix + "{+path}",
		Name:        "gomddoc Guide Page",
		Title:       "gomddoc Guide Page",
		Description: "A page of gomddoc's own user guide, as markdown. List pages with gomddoc_guide.",
		MIMEType:    "text/markdown",
	}, s.handleGuideResource)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "gomddoc_capabilities",
		Title:       "gomddoc Capabilities",
		Description: "Describe gomddoc itself (not the site's content): settings with config keys, env vars, flags and defaults; config precedence; commands; frontmatter fields; the active theme's feature toggles; guide pages.",
		Annotations: readOnlyAnnotations,
	}, s.handleCapabilitiesTool)
}

// guidePage reads a guide page by path. The report's page list is the
// allowlist: a path that is not a listed page is not found, whatever the
// fs.FS would make of it.
func (s *MCPServer) guidePage(p string) ([]byte, error) {
	if len(p) > maxArgLen || !slices.ContainsFunc(s.deps.SelfDocs.Report.Guide, func(g capabilities.GuidePage) bool { return g.Path == p }) {
		return nil, fs.ErrNotExist
	}
	return fs.ReadFile(s.deps.SelfDocs.Guide, p)
}

func (s *MCPServer) handleCapabilitiesResource(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return jsonResourceResult(req.Params.URI, string(s.deps.SelfDocs.Report.JSON()))
}

func (s *MCPServer) handleSchemaResource(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: "application/schema+json",
			Text:     string(config.JSONSchema()),
		}},
	}, nil
}

func (s *MCPServer) handleGuideResource(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	content, err := s.guidePage(strings.TrimPrefix(uri, capabilities.GuideURIPrefix))
	if err != nil {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     string(text.StripFrontmatter(content)),
		}},
	}, nil
}

// CapabilitiesInput is the (empty) input for gomddoc_capabilities.
type CapabilitiesInput struct{}

func (s *MCPServer) handleCapabilitiesTool(_ context.Context, _ *mcp.CallToolRequest, _ CapabilitiesInput) (*mcp.CallToolResult, any, error) {
	return textResult(string(s.deps.SelfDocs.Report.JSON())), nil, nil
}
