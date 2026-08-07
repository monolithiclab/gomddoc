package mcp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/text"
)

func (s *MCPServer) registerResources() {
	// Static: site index (all pages with metadata)
	s.server.AddResource(&mcp.Resource{
		URI:         "docs://site/index",
		Name:        "Site Index",
		Title:       "Site Index",
		Description: "All documentation pages with paths, titles, descriptions, and tags.",
		MIMEType:    "application/json",
	}, s.handleSiteIndex)

	// Static: all tags
	s.server.AddResource(&mcp.Resource{
		URI:         "docs://site/tags",
		Name:        "All Tags",
		Title:       "All Tags",
		Description: "All unique tags used across documentation pages.",
		MIMEType:    "application/json",
	}, s.handleAllTags)

	// Template: read page by path
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "docs://site/page/{+path}",
		Name:        "Documentation Page",
		Title:       "Documentation Page",
		Description: "Read a documentation page by path. Returns clean markdown with frontmatter stripped.",
		MIMEType:    "text/markdown",
	}, s.handlePageResource)

	// Template: pages by tag
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "docs://site/tag/{tag}",
		Name:        "Pages by Tag",
		Title:       "Pages by Tag",
		Description: "List all pages with a specific tag.",
		MIMEType:    "application/json",
	}, s.handleTagResource)
}

func (s *MCPServer) handleSiteIndex(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if s.deps.MetaIndex == nil {
		return jsonResourceResult("docs://site/index", "[]")
	}
	pages := s.deps.MetaIndex.AllPages()
	data, err := json.Marshal(pages)
	if err != nil {
		return nil, err
	}
	return jsonResourceResult("docs://site/index", string(data))
}

func (s *MCPServer) handleAllTags(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if s.deps.MetaIndex == nil {
		return jsonResourceResult("docs://site/tags", "[]")
	}
	tags := s.deps.MetaIndex.AllTags()
	data, err := json.Marshal(tags)
	if err != nil {
		return nil, err
	}
	return jsonResourceResult("docs://site/tags", string(data))
}

func (s *MCPServer) handlePageResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	filePath := strings.TrimPrefix(uri, "docs://site/page/")
	if filePath == "" || filePath == uri {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	if provider.IsRestrictedPath(filePath, s.deps.ExcludePatterns) {
		return nil, mcp.ResourceNotFoundError(uri)
	}

	content, _, err := s.deps.Provider.ReadFile(ctx, filePath)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(uri)
	}

	body := text.StripFrontmatter(content)
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     string(body),
		}},
	}, nil
}

func (s *MCPServer) handleTagResource(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	tag := strings.TrimPrefix(uri, "docs://site/tag/")
	if tag == "" || tag == uri {
		return nil, mcp.ResourceNotFoundError(uri)
	}

	if s.deps.MetaIndex == nil {
		return jsonResourceResult(uri, "[]")
	}

	// Unknown tag is a missing resource, as an unknown page is eight lines
	// above — not a successful read of JSON null. Index.LookupTag is the same
	// answer /tags/{tag} and /api/tags/{tag} give.
	pages, ok := s.deps.MetaIndex.LookupTag(tag)
	if !ok {
		return nil, mcp.ResourceNotFoundError(uri)
	}

	data, err := json.Marshal(pages)
	if err != nil {
		return nil, err
	}
	return jsonResourceResult(uri, string(data))
}

func jsonResourceResult(uri, text string) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "application/json",
			Text:     text,
		}},
	}, nil
}
