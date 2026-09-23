package guide

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/docs"
	"github.com/monolithiclab/gomddoc/internal/search"
	"github.com/monolithiclab/gomddoc/internal/testutil/fanout"
	"github.com/monolithiclab/gomddoc/internal/text"
)

var testFS = fstest.MapFS{
	"README.md":                             {Data: []byte("---\ntitle: Guide\ndescription: Start here\n---\n# Guide\n\nRead the pages.\n")},
	"02-configuration.md":                   {Data: []byte("---\ntitle: Configuration\ndescription: Settings\n---\n# Configuration\n\n## Priority order\n\nFlags win.\n\n```yaml\n# not a heading\n```\n\n## Environment variables\n\nSet GOMDDOC_X.\n")},
	"12-advanced/README.md":                 {Data: []byte("---\ntitle: Advanced\ndescription: Deep dives\n---\n# Advanced\n")},
	"12-advanced/02-markdown-extensions.md": {Data: []byte("---\ntitle: Markdown Extensions\ndescription: GFM and more\n---\n# Markdown Extensions\n")},
}

func TestTopicName(t *testing.T) {
	t.Parallel()
	for p, want := range map[string]string{
		"02-configuration.md":                   "configuration",
		"12-advanced/02-markdown-extensions.md": "markdown-extensions",
		"README.md":                             "overview",
		"12-advanced/README.md":                 "advanced",
		"14-doctor.md":                          "doctor",
	} {
		if got := TopicName(p); got != want {
			t.Errorf("TopicName(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestTopics(t *testing.T) {
	t.Parallel()
	g, err := New(testFS)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tp := range g.Topics() {
		names = append(names, tp.Name+"="+tp.Path+"="+tp.Title)
	}
	want := []string{"overview=README.md=Guide", "configuration=02-configuration.md=Configuration",
		"advanced=12-advanced/README.md=Advanced", "markdown-extensions=12-advanced/02-markdown-extensions.md=Markdown Extensions"}
	if !slices.Equal(names, want) {
		t.Errorf("topics:\n got %q\nwant %q", names, want)
	}
}

// TestTopics_UniqueInRealGuide: two guide pages mapping to one topic name
// would make one of them unreachable from gomddoc help.
func TestTopics_UniqueInRealGuide(t *testing.T) {
	t.Parallel()
	g, err := New(docs.Guide)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, tp := range g.Topics() {
		if prev, dup := seen[tp.Name]; dup {
			t.Errorf("topic %q is both %s and %s", tp.Name, prev, tp.Path)
		}
		seen[tp.Name] = tp.Path
	}
	if len(seen) < 15 {
		t.Errorf("only %d topics", len(seen))
	}
}

func TestPageAndSection(t *testing.T) {
	t.Parallel()
	g, _ := New(testFS)
	for _, ref := range []string{"configuration", "02-configuration.md"} {
		body, tp, err := g.Page(ref)
		if err != nil || tp.Name != "configuration" || !strings.HasPrefix(string(body), "# Configuration\n") || strings.Contains(string(body), "title:") {
			t.Errorf("Page(%q) = %q, %+v, %v", ref, body, tp, err)
		}
	}
	sec, err := g.Section("configuration", "priority-order")
	if err != nil || !strings.HasPrefix(string(sec), "## Priority order\n") || !strings.Contains(string(sec), "# not a heading") {
		t.Errorf("Section = %q, %v", sec, err)
	}
	if _, err := g.Section("configuration", "nope"); !errors.Is(err, text.ErrSectionNotFound) {
		t.Errorf("unknown section: %v", err)
	}
	if ids, _ := g.HeadingIDs("configuration"); !slices.Equal(ids, []string{"configuration", "priority-order", "environment-variables"}) {
		t.Errorf("HeadingIDs = %q", ids)
	}
	for _, bad := range []string{"nope", "../go.mod", "/02-configuration.md", "12-advanced", strings.Repeat("a", 2000)} {
		if _, _, err := g.Page(bad); !errors.Is(err, ErrUnknownTopic) {
			t.Errorf("Page(%q): %v, want ErrUnknownTopic", bad, err)
		}
	}
}

func TestSuggest(t *testing.T) {
	t.Parallel()
	g, _ := New(testFS)
	if s, ok := g.Suggest("configuraton"); !ok || s != "configuration" {
		t.Errorf("Suggest = %q, %v", s, ok)
	}
	if _, ok := g.Suggest("zzzzzzzz"); ok {
		t.Error("no suggestion expected")
	}
}

func TestSearch(t *testing.T) {
	t.Parallel()
	g, _ := New(testFS)
	hits, err := g.Search("priority", 10)
	if err != nil || len(hits) != 1 || hits[0].Topic != "configuration" || hits[0].Path != "02-configuration.md" || hits[0].Title != "Configuration" {
		t.Errorf("Search = %+v, %v", hits, err)
	}
	if hits, _ := g.Search("zzzqqq", 10); hits == nil || len(hits) != 0 {
		t.Errorf("no hits must be an empty, non-nil slice: %#v", hits)
	}
}

// TestSearch_ColdConcurrent: the lazy index is first used cold and
// concurrently; one build wins and every caller shares it. A pure function of
// the embedded guide, so pointer identity plus -race is the check.
func TestSearch_ColdConcurrent(t *testing.T) {
	t.Parallel()
	g, _ := New(testFS)
	got := make([]*search.Index, 50)
	fanout.Run(50, func(i int) {
		idx, err := g.index()
		if err != nil {
			t.Errorf("index: %v", err)
		}
		got[i] = idx
	})
	for i, idx := range got {
		if idx == nil || idx != got[0] {
			t.Fatalf("call %d got a different index", i)
		}
	}
}

func TestSearch_RealGuide(t *testing.T) {
	t.Parallel()
	g, _ := New(docs.Guide)
	hits, err := g.Search("environment variable naming precedence", 10)
	if err != nil || !slices.ContainsFunc(hits, func(h Hit) bool { return h.Topic == "configuration" }) {
		t.Errorf("real guide search: %+v, %v", hits, err)
	}
}
