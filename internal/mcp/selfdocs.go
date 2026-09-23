package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/monolithiclab/gomddoc/internal/capabilities"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/search"
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

	mcp.AddTool(s.server, &mcp.Tool{
		Name:  "gomddoc_guide",
		Title: "gomddoc Guide",
		Description: "gomddoc's own user guide (how to configure, theme, deploy and use gomddoc — not the site's content). " +
			"No arguments: list pages. query: search. path: read a page. path + section: read one section.",
		Annotations: readOnlyAnnotations,
	}, s.handleGuideTool)

	s.server.AddPrompt(&mcp.Prompt{
		Name:        "learn_gomddoc",
		Title:       "Learn gomddoc",
		Description: "Onboard to gomddoc itself: what it does, how configuration works, and where to find details.",
	}, s.handleLearnGomddoc)
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

// GuideInput is the input for gomddoc_guide. Which fields are set chooses the
// mode: none lists pages, query searches, path reads, path + section reads one
// section.
type GuideInput struct {
	Query   string `json:"query,omitempty" jsonschema:"search gomddoc's guide; returns ranked pages"`
	Path    string `json:"path,omitempty" jsonschema:"guide page to read, e.g. 02-configuration.md (list pages by calling with no arguments)"`
	Section string `json:"section,omitempty" jsonschema:"heading anchor within path, e.g. environment-variables"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum search results (default 10, max 50)"`
}

type guideHit struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

func buildGuideSearch(guide fs.FS) (*search.Index, error) {
	ctx := context.Background()
	meta, err := metadata.BuildIndex(ctx, guide, nil)
	if err != nil {
		return nil, fmt.Errorf("guide metadata index: %w", err)
	}
	idx, err := search.BuildIndex(ctx, guide, meta, nil)
	if err != nil {
		return nil, fmt.Errorf("guide search index: %w", err)
	}
	return idx, nil
}

func (s *MCPServer) handleGuideTool(_ context.Context, _ *mcp.CallToolRequest, in GuideInput) (*mcp.CallToolResult, any, error) {
	switch {
	case in.Query != "" && in.Path != "":
		return errorResult("Set either query (search) or path (read), not both."), nil, nil
	case in.Section != "" && in.Path == "":
		return errorResult("section needs a path: the page that contains it."), nil, nil
	case in.Query != "":
		return s.guideSearchResult(in.Query, in.Limit), nil, nil
	case in.Path != "":
		return s.guideReadResult(in.Path, in.Section), nil, nil
	default:
		return jsonTextResult(s.deps.SelfDocs.Report.Guide), nil, nil
	}
}

func (s *MCPServer) guideSearchResult(query string, limit int) *mcp.CallToolResult {
	if len(query) > maxArgLen {
		return errorResult(fmt.Sprintf("query is longer than %d bytes.", maxArgLen))
	}
	limit = min(max(limit, 0), 50)
	if limit == 0 {
		limit = 10
	}
	idx, err := s.guideSearch()
	if err != nil {
		return errorResult(err.Error())
	}
	hits := []guideHit{}
	for _, r := range idx.Search(query, limit) {
		hits = append(hits, guideHit{strings.TrimPrefix(r.Path, "/"), r.Title, r.Snippet})
	}
	return jsonTextResult(hits)
}

func (s *MCPServer) guideReadResult(p, section string) *mcp.CallToolResult {
	content, err := s.guidePage(p)
	if err != nil {
		paths := make([]string, 0, len(s.deps.SelfDocs.Report.Guide))
		for _, g := range s.deps.SelfDocs.Report.Guide {
			paths = append(paths, g.Path)
		}
		return errorResult(fmt.Sprintf("Unknown guide page %q. Pages: %s", truncateArg(p), strings.Join(paths, ", ")))
	}
	if section != "" {
		body, err := ExtractSection(content, section)
		if errors.Is(err, ErrSectionNotFound) || len(section) > maxArgLen {
			return errorResult(fmt.Sprintf("No section %q in %s. Sections: %s",
				truncateArg(section), p, strings.Join(HeadingIDs(content), ", ")))
		}
		if err != nil {
			return errorResult(err.Error())
		}
		return textResult(string(body))
	}
	i := slices.IndexFunc(s.deps.SelfDocs.Report.Guide, func(g capabilities.GuidePage) bool { return g.Path == p })
	page := s.deps.SelfDocs.Report.Guide[i] // guidePage succeeded, so p is listed
	return textResult(pageHeader(page.Title, page.Description, nil) + string(text.StripFrontmatter(content)))
}

// truncateArg keeps an echoed argument short enough to be useful in an error.
func truncateArg(s string) string {
	const n = 80
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "") + "…" // a byte cut can split a rune
}

func errorResult(msg string) *mcp.CallToolResult {
	r := textResult(msg)
	r.IsError = true
	return r
}

func jsonTextResult(v any) *mcp.CallToolResult {
	data, err := json.Marshal(v)
	if err != nil {
		return errorResult(err.Error())
	}
	return textResult(string(data))
}

// handleLearnGomddoc is the "teach me how to use you" entry point: the guide's
// overview plus a compact capabilities summary and how to go deeper.
func (s *MCPServer) handleLearnGomddoc(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	r := s.deps.SelfDocs.Report
	var b strings.Builder
	fmt.Fprintf(&b, "You are working with gomddoc %s, which serves a directory of Markdown files as a documentation "+
		"site, builds it to static HTML, or exposes it over MCP.\n\n", r.Version)
	if readme, err := s.guidePage("README.md"); err == nil {
		b.WriteString("## Overview (from gomddoc's guide)\n\n")
		b.Write(bytes.TrimSpace(text.StripFrontmatter(readme)))
		b.WriteString("\n\n")
	}

	b.WriteString("## How configuration works\n\n")
	fmt.Fprintf(&b, "Precedence: %s.\n", strings.Join(r.Config.Precedence, " > "))
	fmt.Fprintf(&b, "%s\n", r.Config.FileKeys)
	fmt.Fprintf(&b, "%d settings are documented in %s (key, env var, flags, default, description); "+
		"the config file's JSON Schema is %s.\n\n", len(r.Settings), capabilities.CapabilitiesURI, r.Config.SchemaURI)

	b.WriteString("## Commands\n\n")
	for _, c := range r.Commands {
		fmt.Fprintf(&b, "- %s: %s\n", c.Name, c.Help)
	}

	fmt.Fprintf(&b, "\n## Active theme\n\n%s (%s). Feature toggles: %s.\n\n",
		r.Theme.Name, r.Theme.Source, strings.Join(r.Theme.Features, ", "))

	b.WriteString("## How to proceed\n\n" +
		"- Read " + r.Config.SchemaURI + " before writing .gomddoc/config.yml; unknown keys are rejected.\n" +
		"- Use the gomddoc_guide tool (query, or path + section) for anything not covered here.\n" +
		"- Put secrets and deployment-specific values (ports, domain, auth file) in env vars or flags; " +
		"site identity (title, theme, exclude) in config.yml.\n" +
		"- After writing a config, run `gomddoc info` in the site directory: it reports whether the config loads and why not.\n")

	return &mcp.GetPromptResult{
		Description: "Learn gomddoc",
		Messages: []*mcp.PromptMessage{{
			Role:    mcp.Role("user"),
			Content: &mcp.TextContent{Text: b.String()},
		}},
	}, nil
}
