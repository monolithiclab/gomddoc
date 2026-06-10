package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/text"
)

func (s *MCPServer) registerPrompts() {
	s.server.AddPrompt(&mcp.Prompt{
		Name:        "explain_concept",
		Title:       "Explain Concept",
		Description: "Explain a concept using the documentation as source material.",
		Arguments: []*mcp.PromptArgument{{
			Name:        "concept",
			Description: "The concept to explain",
			Required:    true,
		}},
	}, s.handleExplainConcept)

	s.server.AddPrompt(&mcp.Prompt{
		Name:        "troubleshoot",
		Title:       "Troubleshoot Issue",
		Description: "Help troubleshoot an issue using relevant documentation.",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "issue",
				Description: "Description of the issue",
				Required:    true,
			},
			{
				Name:        "error_message",
				Description: "Error message if available",
				Required:    false,
			},
		},
	}, s.handleTroubleshoot)

	s.server.AddPrompt(&mcp.Prompt{
		Name:        "summarize_page",
		Title:       "Summarize Page",
		Description: "Provide a concise summary of a documentation page.",
		Arguments: []*mcp.PromptArgument{{
			Name:        "path",
			Description: "Path to the documentation page",
			Required:    true,
		}},
	}, s.handleSummarizePage)
}

func (s *MCPServer) handleExplainConcept(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	concept := req.Params.Arguments["concept"]
	if concept == "" {
		return nil, fmt.Errorf("concept argument is required")
	}

	docContext := s.gatherSearchContext(concept, 5)

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Explain %q using documentation", concept),
		Messages: []*mcp.PromptMessage{
			{
				Role: mcp.Role("user"),
				Content: &mcp.TextContent{
					Text: fmt.Sprintf(
						"Using the following documentation as source material:\n\n%s\n\n"+
							"Explain the concept %q based on the documentation above. "+
							"Cite specific sections and provide examples from the docs where relevant.",
						docContext, concept,
					),
				},
			},
		},
	}, nil
}

func (s *MCPServer) handleTroubleshoot(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	issue := req.Params.Arguments["issue"]
	if issue == "" {
		return nil, fmt.Errorf("issue argument is required")
	}

	query := issue
	if errMsg := req.Params.Arguments["error_message"]; errMsg != "" {
		query = issue + " " + errMsg
	}

	docContext := s.gatherSearchContext(query, 5)

	var prompt strings.Builder
	fmt.Fprintf(&prompt, "Using the following documentation as reference:\n\n%s\n\n", docContext)
	fmt.Fprintf(&prompt, "Help troubleshoot this issue: %s\n", issue)
	if errMsg := req.Params.Arguments["error_message"]; errMsg != "" {
		fmt.Fprintf(&prompt, "Error message: %s\n", errMsg)
	}
	prompt.WriteString("\nProvide step-by-step guidance based on the documentation.")

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Troubleshoot: %s", issue),
		Messages: []*mcp.PromptMessage{
			{
				Role:    mcp.Role("user"),
				Content: &mcp.TextContent{Text: prompt.String()},
			},
		},
	}, nil
}

func (s *MCPServer) handleSummarizePage(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	pagePath := req.Params.Arguments["path"]
	if pagePath == "" {
		return nil, fmt.Errorf("path argument is required")
	}

	if provider.IsRestrictedPath(pagePath, s.deps.ExcludePatterns) {
		return nil, fmt.Errorf("page not found: %s", pagePath)
	}

	content, _, err := s.deps.Provider.ReadFile(ctx, pagePath)
	if err != nil {
		return nil, fmt.Errorf("page not found: %s", pagePath)
	}

	body := text.StripFrontmatter(content)

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Summarize %s", pagePath),
		Messages: []*mcp.PromptMessage{
			{
				Role: mcp.Role("user"),
				Content: &mcp.TextContent{
					Text: fmt.Sprintf(
						"Here is a documentation page:\n\n%s\n\n"+
							"Provide a concise summary of this page. "+
							"Include: key topics, important concepts, and actionable takeaways.",
						string(body),
					),
				},
			},
		},
	}, nil
}

// gatherSearchContext searches for a query and returns formatted content from
// the top results. Returns empty string if search index is unavailable.
func (s *MCPServer) gatherSearchContext(query string, limit int) string {
	if s.deps.SearchIndex == nil {
		return "(No search index available)"
	}

	results := s.deps.SearchIndex.Search(query, limit)
	if len(results) == 0 {
		return "(No relevant documentation found)"
	}

	var b strings.Builder
	for _, r := range results {
		fmt.Fprintf(&b, "### %s (`%s`)\n", r.Title, r.Path)
		// Tag-only results have no snippet; fall back to the description so the
		// prompt still carries page content rather than bare headings.
		if r.Snippet != "" {
			fmt.Fprintf(&b, "%s\n\n", r.Snippet)
		} else if r.Description != "" {
			fmt.Fprintf(&b, "%s\n\n", r.Description)
		}
	}
	return b.String()
}
