package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"

	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
	"github.com/monolithiclab/gomddoc/internal/text"
)

var readOnlyAnnotations = &mcp.ToolAnnotations{
	ReadOnlyHint:   true,
	IdempotentHint: true,
}

func (s *MCPServer) registerTools() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_docs",
		Title:       "Search Documentation",
		Description: "Full-text search across all documentation pages. Returns ranked results with snippets. Supports tag: prefix syntax (e.g. \"tag:deployment kubernetes\") to filter by frontmatter tag.",
		Annotations: readOnlyAnnotations,
	}, s.handleSearchDocs)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "read_page",
		Title:       "Read Page",
		Description: "Read a documentation page with metadata. Returns clean markdown with frontmatter stripped and metadata prepended.",
		Annotations: readOnlyAnnotations,
	}, s.handleReadPage)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_pages",
		Title:       "List Pages",
		Description: "List documentation pages with metadata. Optionally filter by tag.",
		Annotations: readOnlyAnnotations,
	}, s.handleListPages)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_table_of_contents",
		Title:       "Get Table of Contents",
		Description: "Get the site-wide navigation tree showing all pages organized by directory.",
		Annotations: readOnlyAnnotations,
	}, s.handleGetTOC)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "read_section",
		Title:       "Read Section",
		Description: "Read a specific section of a page by heading anchor ID. Returns markdown from the heading to the next heading at the same or higher level.",
		Annotations: readOnlyAnnotations,
	}, s.handleReadSection)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "find_related",
		Title:       "Find Related Pages",
		Description: "Find documentation pages related to a given page based on shared tags.",
		Annotations: readOnlyAnnotations,
	}, s.handleFindRelated)
}

// Tool input types — JSON Schema is auto-generated from struct tags.

// SearchDocsInput is the input for search_docs.
type SearchDocsInput struct {
	Query string `json:"query" jsonschema:"search query text"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum results to return (default 20)"`
}

// maxArgLen bounds free-form string arguments (paths, heading IDs, tags). The
// MCP request body is already capped, but rejecting absurdly long values keeps
// processing bounded and is consistent with the HTTP handlers (REVIEW §9.9).
const maxArgLen = 1024

// ReadPageInput is the input for read_page.
type ReadPageInput struct {
	Path string `json:"path" jsonschema:"page file path (e.g. guide/configuration.md)"`
}

// ListPagesInput is the input for list_pages.
type ListPagesInput struct {
	Tag   string `json:"tag,omitempty" jsonschema:"filter by tag"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum pages to return (default 50)"`
}

// GetTOCInput is the input for get_table_of_contents.
type GetTOCInput struct {
	Path string `json:"path,omitempty" jsonschema:"subtree root path"`
}

// ReadSectionInput is the input for read_section.
type ReadSectionInput struct {
	Path      string `json:"path" jsonschema:"page file path"`
	HeadingID string `json:"heading_id" jsonschema:"heading anchor ID (e.g. installation)"`
}

// FindRelatedInput is the input for find_related.
type FindRelatedInput struct {
	Path string `json:"path" jsonschema:"page path to find related documents for"`
}

func (s *MCPServer) handleSearchDocs(_ context.Context, _ *mcp.CallToolRequest, input SearchDocsInput) (*mcp.CallToolResult, any, error) {
	if s.deps.SearchIndex == nil {
		return textResult("Search index is not available."), nil, nil
	}
	if input.Query == "" {
		return textResult("Query must not be empty."), nil, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	results := s.deps.SearchIndex.Search(input.Query, limit)
	if len(results) == 0 {
		return textResult("No results found."), nil, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d results for %q:\n\n", len(results), input.Query)
	for i, r := range results {
		fmt.Fprintf(&b, "%d. **%s** (`%s`)\n", i+1, r.Title, r.Path)
		if r.Description != "" {
			fmt.Fprintf(&b, "   %s\n", r.Description)
		}
		if r.Snippet != "" {
			fmt.Fprintf(&b, "   > %s\n", r.Snippet)
		}
		// Tag-only results are not scored; omit the line rather than print a
		// misleading "Score: 0.0".
		if r.Score != 0 {
			fmt.Fprintf(&b, "   Score: %.1f\n", r.Score)
		}
		b.WriteString("\n")
	}
	return textResult(b.String()), nil, nil
}

func (s *MCPServer) handleReadPage(ctx context.Context, _ *mcp.CallToolRequest, input ReadPageInput) (*mcp.CallToolResult, any, error) {
	if input.Path == "" {
		return textResult("Path must not be empty."), nil, nil
	}
	if len(input.Path) > maxArgLen || provider.IsRestrictedPath(input.Path, s.deps.ExcludePatterns) {
		return textResult(fmt.Sprintf("Page not found: %s", input.Path)), nil, nil
	}

	content, _, err := s.deps.Provider.ReadFile(ctx, input.Path)
	if err != nil {
		return textResult(fmt.Sprintf("Page not found: %s", input.Path)), nil, nil
	}

	body := text.StripFrontmatter(content)

	// Prepend metadata if available from the index.
	header := s.buildPageHeader(input.Path)
	if header != "" {
		return textResult(header + string(body)), nil, nil
	}
	return textResult(string(body)), nil, nil
}

func (s *MCPServer) handleListPages(_ context.Context, _ *mcp.CallToolRequest, input ListPagesInput) (*mcp.CallToolResult, any, error) {
	if s.deps.MetaIndex == nil {
		return textResult("Metadata index is not available."), nil, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	if len(input.Tag) > maxArgLen {
		return textResult(fmt.Sprintf("No pages found with tag %q.", input.Tag)), nil, nil
	}

	var pages []pageEntry
	if input.Tag != "" {
		for _, p := range s.deps.MetaIndex.ByTag(input.Tag) {
			pages = append(pages, pageEntry{p.Path, p.Title, p.Description, p.Tags})
		}
	} else {
		for _, p := range s.deps.MetaIndex.AllPages() {
			pages = append(pages, pageEntry{p.Path, p.Title, p.Description, p.Tags})
		}
	}

	if len(pages) > limit {
		pages = pages[:limit]
	}

	if len(pages) == 0 {
		if input.Tag != "" {
			return textResult(fmt.Sprintf("No pages found with tag %q.", input.Tag)), nil, nil
		}
		return textResult("No pages found."), nil, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d pages", len(pages))
	if input.Tag != "" {
		fmt.Fprintf(&b, " tagged %q", input.Tag)
	}
	b.WriteString(":\n\n")
	for _, p := range pages {
		fmt.Fprintf(&b, "- **%s** (`%s`)", p.Title, p.Path)
		if p.Description != "" {
			fmt.Fprintf(&b, " — %s", p.Description)
		}
		if len(p.Tags) > 0 {
			fmt.Fprintf(&b, " [%s]", strings.Join(p.Tags, ", "))
		}
		b.WriteByte('\n')
	}
	return textResult(b.String()), nil, nil
}

func (s *MCPServer) handleGetTOC(_ context.Context, _ *mcp.CallToolRequest, input GetTOCInput) (*mcp.CallToolResult, any, error) {
	if s.deps.NavGenerator == nil {
		return textResult("No navigation tree available."), nil, nil
	}

	// Shared, never constructed here: see navigation.Generator.
	root := s.deps.NavGenerator.Tree()
	if root == nil {
		return textResult("No navigation tree available."), nil, nil
	}

	var b strings.Builder
	b.WriteString("# Table of Contents\n\n")
	formatNavTree(&b, root.Children, 0)
	return textResult(b.String()), nil, nil
}

func (s *MCPServer) handleReadSection(ctx context.Context, _ *mcp.CallToolRequest, input ReadSectionInput) (*mcp.CallToolResult, any, error) {
	if input.Path == "" || input.HeadingID == "" {
		return textResult("Both path and heading_id are required."), nil, nil
	}
	if len(input.Path) > maxArgLen || len(input.HeadingID) > maxArgLen ||
		provider.IsRestrictedPath(input.Path, s.deps.ExcludePatterns) {
		return textResult(fmt.Sprintf("Page not found: %s", input.Path)), nil, nil
	}

	content, _, err := s.deps.Provider.ReadFile(ctx, input.Path)
	if err != nil {
		return textResult(fmt.Sprintf("Page not found: %s", input.Path)), nil, nil
	}

	section, err := ExtractSection(content, input.HeadingID)
	if err != nil {
		return textResult(fmt.Sprintf("Section %q not found in %s.", input.HeadingID, input.Path)), nil, nil
	}
	return textResult(string(section)), nil, nil
}

func (s *MCPServer) handleFindRelated(_ context.Context, _ *mcp.CallToolRequest, input FindRelatedInput) (*mcp.CallToolResult, any, error) {
	if input.Path == "" {
		return textResult("Path must not be empty."), nil, nil
	}
	if len(input.Path) > maxArgLen || provider.IsRestrictedPath(input.Path, s.deps.ExcludePatterns) {
		return textResult(fmt.Sprintf("Page not found: %s", input.Path)), nil, nil
	}
	if s.deps.MetaIndex == nil {
		return textResult("Metadata index is not available."), nil, nil
	}

	// Find the page's tags.
	normalizedPath := input.Path
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}

	page := s.deps.MetaIndex.ByPath(normalizedPath)
	var pageTags []string
	if page != nil {
		pageTags = page.Tags
	}

	if len(pageTags) == 0 {
		return textResult(fmt.Sprintf("No tags found for %s — cannot determine related pages.", input.Path)), nil, nil
	}

	// Collect related pages from all tags, deduplicated.
	seen := map[string]bool{normalizedPath: true}
	type relatedEntry struct {
		Path       string
		Title      string
		SharedTags []string
	}
	var related []relatedEntry

	for _, tag := range pageTags {
		// PagesByTag, not ByTag: this loop reads Path and Title and retains
		// neither pointer, so there is nothing to gain from a deep copy per page.
		for p := range s.deps.MetaIndex.PagesByTag(tag) {
			if seen[p.Path] {
				// Update shared tags for already-seen page.
				for i := range related {
					if related[i].Path == p.Path {
						related[i].SharedTags = append(related[i].SharedTags, tag)
						break
					}
				}
				continue
			}
			seen[p.Path] = true
			related = append(related, relatedEntry{p.Path, p.Title, []string{tag}})
		}
	}

	if len(related) == 0 {
		return textResult("No related pages found."), nil, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d pages related to %s:\n\n", len(related), input.Path)
	for _, r := range related {
		fmt.Fprintf(&b, "- **%s** (`%s`) — shared tags: %s\n", r.Title, r.Path, strings.Join(r.SharedTags, ", "))
	}
	return textResult(b.String()), nil, nil
}

// buildPageHeader constructs a metadata header block for a page.
func (s *MCPServer) buildPageHeader(filePath string) string {
	if s.deps.MetaIndex == nil {
		return ""
	}

	normalizedPath := filePath
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}

	p := s.deps.MetaIndex.ByPath(normalizedPath)
	if p == nil {
		return ""
	}

	header := struct {
		Title       string   `yaml:"title,omitempty"`
		Description string   `yaml:"description,omitempty"`
		Tags        []string `yaml:"tags,omitempty,flow"`
	}{
		Title:       p.Title,
		Description: p.Description,
		Tags:        p.Tags,
	}

	data, err := yaml.Marshal(header)
	if err != nil {
		return ""
	}
	return "---\n" + string(data) + "---\n\n"
}

type pageEntry struct {
	Path        string
	Title       string
	Description string
	Tags        []string
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func formatNavTree(b *strings.Builder, nodes []*navigation.NavNode, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, node := range nodes {
		if node.IsDir {
			fmt.Fprintf(b, "%s- **%s/**\n", indent, node.Label)
		} else {
			fmt.Fprintf(b, "%s- %s (`%s`)\n", indent, node.Label, node.Path)
		}
		if len(node.Children) > 0 {
			formatNavTree(b, node.Children, depth+1)
		}
	}
}
