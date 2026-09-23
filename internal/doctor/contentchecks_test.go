package doctor

import (
	"context"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/diag"
	"github.com/monolithiclab/gomddoc/internal/template"
)

var testScan = template.ThemeInfo{Name: "default", Source: template.ThemeSourceTemplateScan, Features: []string{"search", "toc"}}

func TestContentChecks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		page string
		want []string
	}{
		{"complete", "---\ntitle: A\ndescription: About A\ntags: [x]\ndate: 2025-06-15\nfeatures: {toc: false}\n---\n# A\n", nil},
		{"no frontmatter, heading", "# A\n\nText.\n", []string{"content.missing-description a.md:0 "}},
		{"no frontmatter, no heading", "Text only.\n", []string{"content.missing-description a.md:0 ", "content.missing-title a.md:0 "}},
		{"invalid yaml", "---\ntitle: A\ntags: [x\n---\n# A\n", []string{"content.frontmatter-invalid a.md:2 "}}, // yaml.v3 reports an unclosed flow one line early
		{"tags as string", "---\ntitle: A\ndescription: D\ntags: foo\n---\n", []string{"content.frontmatter-type a.md:4 tags"}},
		{"title not a string", "---\ntitle: 123\ndescription: D\n---\n# A\n", []string{"content.frontmatter-type a.md:2 title"}},
		{"bad date", "---\ntitle: A\ndescription: D\ndate: someday\n---\n", []string{"content.frontmatter-type a.md:4 date"}},
		{"redirect_from as string", "---\ntitle: A\ndescription: D\nredirect_from: /old\n---\n", []string{"content.frontmatter-type a.md:4 redirect_from"}},
		{"features not bool", "---\ntitle: A\ndescription: D\nfeatures: {toc: nope}\n---\n", []string{"content.frontmatter-type a.md:4 features"}},
		{"unknown feature", "---\ntitle: A\ndescription: D\nfeatures:\n  serach: false\n---\n", []string{"theme.unknown-feature a.md:5 features.serach"}},
		{"custom fields ignored", "---\ntitle: A\ndescription: D\nauthor: Me\ncategory: [a, b]\n---\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := contentChecks(context.Background(), PipelineView{Root: fstest.MapFS{"a.md": {Data: []byte(tt.page)}}}, testScan)
			diag.Sort(got)
			if !slices.Equal(ids(got), tt.want) {
				t.Errorf("findings = %q, want %q", ids(got), tt.want)
			}
		})
	}
}

func TestContentChecks_WalkScope(t *testing.T) {
	t.Parallel()
	root := fstest.MapFS{
		"drafts/bad.md":   {Data: []byte("---\ntags: [\n---\n")},
		".hidden/bad.md":  {Data: []byte("---\ntags: [\n---\n")},
		"image.png":       {Data: []byte{0x89}},
		"guide/page.md":   {Data: []byte("---\ntitle: P\ndescription: D\n---\n")},
		"guide/notes.txt": {Data: []byte("---\ntags: [\n---\n")},
	}
	got := contentChecks(context.Background(), PipelineView{Lang: "fr-FR", Root: root, Exclude: []string{"drafts/"}}, testScan)
	if len(got) != 0 {
		t.Errorf("excluded, hidden and non-markdown files must not be checked: %q", ids(got))
	}

	root["guide/page.md"] = &fstest.MapFile{Data: []byte("---\ntags: nope\n---\n# P\n")}
	got = contentChecks(context.Background(), PipelineView{Lang: "fr-FR", Root: root, Exclude: []string{"drafts/"}}, testScan)
	diag.Sort(got)
	want := []string{"content.frontmatter-type fr-FR/guide/page.md:2 tags", "content.missing-description fr-FR/guide/page.md:0 "}
	if !slices.Equal(ids(got), want) {
		t.Errorf("language pipeline paths: %q, want %q", ids(got), want)
	}
}

// TestRun_ProducerAndCheckMerge: a string redirect_from is reported by the
// redirect map (no line) and by the frontmatter check (with line): once.
func TestRun_ProducerAndCheckMerge(t *testing.T) {
	t.Parallel()
	root := fstest.MapFS{"a.md": {Data: []byte("---\ntitle: A\ndescription: D\nredirect_from: /old\n---\n")}}
	in := Input{
		Inspection: inspect(t, ""),
		Pipelines:  []PipelineView{{Root: root}},
		Producer:   []diag.Finding{diag.New("content.frontmatter-type", "a.md", 0, "redirect_from", "not a list", "use a list")},
	}
	got := Run(context.Background(), in, Options{Verbose: true})
	if want := []string{"content.frontmatter-type a.md:4 redirect_from"}; !slices.Equal(ids(got.Findings), want) {
		t.Errorf("findings = %q, want %q", ids(got.Findings), want)
	}
}

// failingOpen fails Open for one file, as an unreadable page would.
type failingOpen struct {
	fstest.MapFS
	fail string
}

func (f failingOpen) Open(name string) (fs.File, error) {
	if name == f.fail {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return f.MapFS.Open(name)
}

// ReadFile must fail too: MapFS's promoted ReadFile would bypass Open.
func (f failingOpen) ReadFile(name string) ([]byte, error) {
	if name == f.fail {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrPermission}
	}
	return f.MapFS.ReadFile(name)
}

func TestContentChecks_ReadErrorAndCancel(t *testing.T) {
	t.Parallel()
	root := failingOpen{MapFS: fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: A\ndescription: D\n---\n")},
		"b.md": {Data: []byte("---\ntitle: B\ndescription: D\n---\n")},
	}, fail: "b.md"}
	got := contentChecks(context.Background(), PipelineView{Root: root}, testScan)
	if want := []string{"target.read-error b.md:0 "}; !slices.Equal(ids(got), want) {
		t.Errorf("findings = %q, want %q (a.md must still be checked)", ids(got), want)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := contentChecks(ctx, PipelineView{Root: root}, testScan); len(got) != 0 {
		t.Errorf("a cancelled walk reported %q", ids(got))
	}
}
