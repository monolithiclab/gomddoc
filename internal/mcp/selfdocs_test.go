package mcp

import (
	"context"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/monolithiclab/gomddoc/internal/capabilities"
	"github.com/monolithiclab/gomddoc/internal/config"
)

var selfDocsGuide = fstest.MapFS{
	"README.md":                {Data: []byte("---\ntitle: Guide\ndescription: Start here\n---\n# Guide\n\nRead the pages.\n")},
	"02-configuration.md":      {Data: []byte("---\ntitle: Configuration\ndescription: Settings\n---\n# Configuration\n\n## Environment variables\n\nSet GOMDDOC_SITE_THEME_NAME.\n\n## Priority order\n\nFlags win.\n")},
	"05-theming-and-assets.md": {Data: []byte("---\ntitle: Theming\ndescription: Themes\n---\n# Theming\n\nThemes and vars.\n")},
}

func newSelfDocs(guide fs.FS) *SelfDocs {
	return &SelfDocs{
		Report: capabilities.Describe(capabilities.Input{
			Version:  "test",
			Config:   config.New(),
			Commands: []capabilities.Command{{Name: "serve", Help: "Serve the site"}},
			Guide:    guide,
		}),
		Guide: guide,
	}
}

func setupSelfDocs(t *testing.T, withSelfDocs bool) *testFixture {
	t.Helper()
	var sd *SelfDocs
	if withSelfDocs {
		sd = newSelfDocs(selfDocsGuide)
	}
	return connect(t, NewServer(ServerDeps{
		Provider: &testProvider{fsys: fstest.MapFS{}, defaultIndex: "README.md"},
		SelfDocs: sd,
	}))
}

// TestSelfDocs_RegisteredOnlyWhenSet: the namespace is the operator's, served
// over stdio; serve's HTTP mount passes nil and must not expose it.
func TestSelfDocs_RegisteredOnlyWhenSet(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{true, false} {
		f := setupSelfDocs(t, on)
		defer f.close(t)
		ctx := context.Background()

		var toolNames, uris, templates, promptNames []string
		tools, err := f.session.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, tl := range tools.Tools {
			toolNames = append(toolNames, tl.Name)
		}
		res, err := f.session.ListResources(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range res.Resources {
			uris = append(uris, r.URI)
		}
		tmpl, err := f.session.ListResourceTemplates(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range tmpl.ResourceTemplates {
			templates = append(templates, r.URITemplate)
		}
		prompts, err := f.session.ListPrompts(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range prompts.Prompts {
			promptNames = append(promptNames, p.Name)
		}

		checks := []struct {
			list []string
			name string
		}{
			{toolNames, "gomddoc_capabilities"},
			{uris, "gomddoc://capabilities"},
			{uris, "gomddoc://schema/config"},
			{templates, "gomddoc://guide/{+path}"},
		}
		for _, c := range checks {
			if got := slices.Contains(c.list, c.name); got != on {
				t.Errorf("SelfDocs set=%v: %s registered=%v", on, c.name, got)
			}
		}
		// The site's own surface is there either way.
		if !slices.Contains(toolNames, "search_docs") || !slices.Contains(promptNames, "explain_concept") {
			t.Errorf("SelfDocs set=%v: site tools/prompts missing", on)
		}
	}
}

func TestSelfDocs_CapabilitiesAndSchema(t *testing.T) {
	t.Parallel()
	f := setupSelfDocs(t, true)
	defer f.close(t)
	ctx := context.Background()
	want := string(f.server.deps.SelfDocs.Report.JSON())

	res, err := f.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "gomddoc://capabilities"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Contents[0].Text != want || res.Contents[0].MIMEType != "application/json" {
		t.Errorf("capabilities resource: mime=%q, equal to Report.JSON=%v", res.Contents[0].MIMEType, res.Contents[0].Text == want)
	}

	tool, err := f.session.CallTool(ctx, &mcp.CallToolParams{Name: "gomddoc_capabilities", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if tool.IsError || tool.Content[0].(*mcp.TextContent).Text != want {
		t.Error("gomddoc_capabilities differs from the resource")
	}

	schema, err := f.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "gomddoc://schema/config"})
	if err != nil {
		t.Fatal(err)
	}
	if schema.Contents[0].Text != string(config.JSONSchema()) || schema.Contents[0].MIMEType != "application/schema+json" {
		t.Errorf("schema resource: mime=%q", schema.Contents[0].MIMEType)
	}
}

func TestSelfDocs_GuideResource(t *testing.T) {
	t.Parallel()
	f := setupSelfDocs(t, true)
	defer f.close(t)
	ctx := context.Background()

	res, err := f.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "gomddoc://guide/02-configuration.md"})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Contents[0].Text; !strings.HasPrefix(got, "# Configuration\n") || res.Contents[0].MIMEType != "text/markdown" {
		t.Errorf("guide page must be markdown with frontmatter stripped, got %q", got)
	}
	for _, bad := range []string{
		"gomddoc://guide/",
		"gomddoc://guide/nope.md",
		"gomddoc://guide/../go.mod",
		"gomddoc://guide//02-configuration.md",
		"gomddoc://guide/" + strings.Repeat("a", maxArgLen+1),
	} {
		if _, err := f.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: bad}); err == nil {
			t.Errorf("%s: want not-found error", bad)
		}
	}
}
