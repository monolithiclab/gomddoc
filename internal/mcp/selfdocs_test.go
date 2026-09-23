package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/monolithiclab/gomddoc/docs"
	"github.com/monolithiclab/gomddoc/internal/capabilities"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/diag"
	"github.com/monolithiclab/gomddoc/internal/doctor"
	"github.com/monolithiclab/gomddoc/internal/search"
	"github.com/monolithiclab/gomddoc/internal/testutil/fanout"
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
			{toolNames, "gomddoc_guide"},
			{uris, "gomddoc://capabilities"},
			{uris, "gomddoc://schema/config"},
			{templates, "gomddoc://guide/{+path}"},
			{promptNames, "learn_gomddoc"},
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

func callGuide(t *testing.T, f *testFixture, args map[string]any) (string, bool) {
	t.Helper()
	res, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{Name: "gomddoc_guide", Arguments: args})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return res.Content[0].(*mcp.TextContent).Text, res.IsError
}

func TestGuideTool(t *testing.T) {
	t.Parallel()
	f := setupSelfDocs(t, true)
	defer f.close(t)

	t.Run("list", func(t *testing.T) {
		out, isErr := callGuide(t, f, map[string]any{})
		var got []capabilities.GuidePage
		if err := json.Unmarshal([]byte(out), &got); isErr || err != nil {
			t.Fatalf("list: isErr=%v err=%v out=%s", isErr, err, out)
		}
		if !slices.Equal(got, f.server.deps.SelfDocs.Report.Guide) || len(got) != 3 {
			t.Errorf("list = %+v", got)
		}
	})
	t.Run("search", func(t *testing.T) {
		// "priority" occurs only in 02-configuration.md.
		out, isErr := callGuide(t, f, map[string]any{"query": "priority"})
		var hits []struct{ Path, Title, Snippet string }
		if err := json.Unmarshal([]byte(out), &hits); isErr || err != nil {
			t.Fatalf("search: isErr=%v err=%v out=%s", isErr, err, out)
		}
		if len(hits) != 1 || hits[0].Path != "02-configuration.md" || hits[0].Title != "Configuration" {
			t.Errorf("hits = %+v", hits)
		}
	})
	t.Run("search no hits", func(t *testing.T) {
		out, isErr := callGuide(t, f, map[string]any{"query": "zzzqqq"})
		if isErr || out != "[]" {
			t.Errorf("no hits: isErr=%v out=%q, want []", isErr, out)
		}
	})
	t.Run("page", func(t *testing.T) {
		out, isErr := callGuide(t, f, map[string]any{"path": "02-configuration.md"})
		want := "---\ntitle: Configuration\ndescription: Settings\n---\n\n# Configuration\n"
		if isErr || !strings.HasPrefix(out, want) || strings.Count(out, "title: Configuration") != 1 || !strings.Contains(out, "Flags win.") {
			t.Errorf("page: isErr=%v out=%q", isErr, out)
		}
	})
	t.Run("section", func(t *testing.T) {
		out, isErr := callGuide(t, f, map[string]any{"path": "02-configuration.md", "section": "priority-order"})
		if isErr || out != "## Priority order\n\nFlags win." {
			t.Errorf("section: isErr=%v out=%q", isErr, out)
		}
	})

	errs := []struct {
		name string
		args map[string]any
		want []string // every substring must appear in the error text
	}{
		{"unknown page", map[string]any{"path": "nope.md"}, []string{"nope.md", "02-configuration.md", "README.md"}},
		{"traversal", map[string]any{"path": "../go.mod"}, []string{"02-configuration.md"}},
		{"leading slash", map[string]any{"path": "/02-configuration.md"}, []string{"02-configuration.md"}},
		{"directory", map[string]any{"path": "12-advanced"}, []string{"README.md"}},
		{"unknown section", map[string]any{"path": "02-configuration.md", "section": "nope"},
			[]string{"nope", "environment-variables", "priority-order"}},
		{"section without path", map[string]any{"section": "priority-order"}, []string{"path"}},
		{"query and path", map[string]any{"query": "x", "path": "README.md"}, []string{"query", "path"}},
		{"oversized path", map[string]any{"path": strings.Repeat("a", maxArgLen+1)}, []string{"README.md"}},
		{"oversized query", map[string]any{"query": strings.Repeat("a", maxArgLen+1)}, []string{"query"}},
	}
	for _, tt := range errs {
		t.Run(tt.name, func(t *testing.T) {
			out, isErr := callGuide(t, f, tt.args)
			if !isErr {
				t.Fatalf("want IsError, got %q", out)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("error %q lacks %q", out, w)
				}
			}
		})
	}
}

// TestGuideSearch_ColdConcurrent: the lazy guide index is first used cold and
// concurrently; exactly one build must win and every caller gets it. The
// index is a pure function of the embedded guide, so no count can tell one
// build from fifty — pointer identity plus -race (make test) is the check.
func TestGuideSearch_ColdConcurrent(t *testing.T) {
	t.Parallel()
	s := NewServer(ServerDeps{
		Provider: &testProvider{fsys: fstest.MapFS{}, defaultIndex: "README.md"},
		SelfDocs: newSelfDocs(selfDocsGuide),
	})
	got := make([]*search.Index, 50)
	fanout.Run(50, func(i int) {
		idx, err := s.guideSearch()
		if err != nil {
			t.Errorf("guideSearch: %v", err)
		}
		got[i] = idx
	})
	for i, idx := range got {
		if idx == nil || idx != got[0] {
			t.Fatalf("call %d returned a different index (%p vs %p)", i, idx, got[0])
		}
	}
}

// TestGuideTool_RealEmbed: the shipped guide indexes and answers a real query.
func TestGuideTool_RealEmbed(t *testing.T) {
	t.Parallel()
	s := NewServer(ServerDeps{
		Provider: &testProvider{fsys: fstest.MapFS{}, defaultIndex: "README.md"},
		SelfDocs: newSelfDocs(docs.Guide),
	})
	idx, err := s.guideSearch()
	if err != nil {
		t.Fatal(err)
	}
	hits := idx.Search("environment variable naming precedence", 10)
	if !slices.ContainsFunc(hits, func(h search.SearchResult) bool { return strings.TrimPrefix(h.Path, "/") == "02-configuration.md" }) {
		t.Errorf("real guide search missed 02-configuration.md: %+v", hits)
	}
}

func TestLearnGomddocPrompt(t *testing.T) {
	t.Parallel()
	f := setupSelfDocs(t, true)
	defer f.close(t)
	res, err := f.session.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "learn_gomddoc"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Messages) != 1 {
		t.Fatalf("want one message, got %d", len(res.Messages))
	}
	msg := res.Messages[0].Content.(*mcp.TextContent).Text
	r := f.server.deps.SelfDocs.Report
	for _, want := range []string{
		"gomddoc test",                // version
		"Read the pages.",             // guide README body
		"flag > env > file > default", // precedence
		r.Config.FileKeys,             // file-key rule
		fmt.Sprintf("%d settings", len(r.Settings)),
		"gomddoc://capabilities",
		"gomddoc://schema/config", // where the schema is
		"gomddoc_guide",           // where depth is
		"gomddoc_doctor",          // how to check an edit
		"serve: Serve the site",   // a command
		"gomddoc doctor --json",   // the CLI form of the check
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Contains(msg, "title: Guide") {
		t.Error("README frontmatter must be stripped")
	}
}

func TestDoctorTool(t *testing.T) {
	t.Parallel()
	var calls []bool
	sd := newSelfDocs(selfDocsGuide)
	sd.Doctor = func(_ context.Context, verbose bool) doctor.Report {
		calls = append(calls, verbose)
		return doctor.Report{Target: "site", Summary: doctor.Summary{Errors: len(calls)}, Findings: []diag.Finding{}}
	}
	f := connect(t, NewServer(ServerDeps{Provider: &testProvider{fsys: fstest.MapFS{}, defaultIndex: "README.md"}, SelfDocs: sd}))
	defer f.close(t)

	for _, verbose := range []bool{false, true} {
		res, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{Name: "gomddoc_doctor", Arguments: map[string]any{"verbose": verbose}})
		if err != nil {
			t.Fatal(err)
		}
		var r doctor.Report
		if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &r); err != nil || res.IsError {
			t.Fatalf("report: %v isError=%v", err, res.IsError)
		}
		if r.Target != "site" || r.Summary.Errors != len(calls) {
			t.Errorf("report %+v after %d calls", r, len(calls))
		}
	}
	// Every call re-runs the checks; verbose is passed through.
	if !slices.Equal(calls, []bool{false, true}) {
		t.Errorf("calls = %v", calls)
	}
}

func TestDoctorTool_AbsentWithoutDoctor(t *testing.T) {
	t.Parallel()
	f := setupSelfDocs(t, true) // SelfDocs without a Doctor func
	defer f.close(t)
	tools, err := f.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(tools.Tools, func(tl *mcp.Tool) bool { return tl.Name == "gomddoc_doctor" }) {
		t.Error("gomddoc_doctor registered without a Doctor func")
	}
}
