package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/docs"
	"github.com/monolithiclab/gomddoc/internal/guide"
)

type helpRun struct {
	out, errOut string
	code        int
	paged       string
}

func runHelp(t *testing.T, cmd HelpCmd, terminal bool) helpRun {
	t.Helper()
	var out, errOut bytes.Buffer
	var paged string
	cmd.out, cmd.errOut = &out, &errOut
	cmd.isTerminal = func() bool { return terminal }
	cmd.page = func(text string) error { paged = text; return nil }
	err := cmd.Run()
	code := 0
	var exit exitCodeError
	switch {
	case errors.As(err, &exit):
		code = int(exit)
	case err != nil:
		t.Fatalf("help returned %v", err)
	}
	return helpRun{out.String(), errOut.String(), code, paged}
}

func TestHelp_ListsTopics(t *testing.T) {
	t.Parallel()
	r := runHelp(t, HelpCmd{}, false)
	for _, want := range []string{"overview", "configuration", "doctor", "markdown-extensions",
		"Configuration Guide", "gomddoc help --search", "gomddoc <command> --help", "gomddoc doctor"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("topic list lacks %q:\n%s", want, r.out)
		}
	}
	if r.code != 0 || r.paged != "" {
		t.Errorf("code %d paged %q", r.code, r.paged)
	}
}

func TestHelp_TopicAndSection(t *testing.T) {
	t.Parallel()
	page := runHelp(t, HelpCmd{Topic: "configuration"}, false)
	if !strings.HasPrefix(page.out, "# Configuration\n") || strings.Contains(page.out, `title: "Configuration Guide"`) {
		t.Errorf("page must be the markdown without frontmatter:\n%.200s", page.out)
	}
	sec := runHelp(t, HelpCmd{Topic: "configuration", Section: "priority-order"}, false)
	if !strings.HasPrefix(sec.out, "## Priority Order\n") || strings.Contains(sec.out, "## Site Configuration") {
		t.Errorf("section:\n%.300s", sec.out)
	}
}

func TestHelp_Unknown(t *testing.T) {
	t.Parallel()
	topic := runHelp(t, HelpCmd{Topic: "configuraton"}, false)
	if topic.code != 1 || topic.out != "" || !strings.Contains(topic.errOut, "did you mean `configuration`?") ||
		!strings.Contains(topic.errOut, "overview") {
		t.Errorf("unknown topic: code %d out %q err %q", topic.code, topic.out, topic.errOut)
	}
	sec := runHelp(t, HelpCmd{Topic: "configuration", Section: "nope"}, false)
	if sec.code != 1 || !strings.Contains(sec.errOut, "priority-order") {
		t.Errorf("unknown section: code %d err %q", sec.code, sec.errOut)
	}
}

func TestHelp_Search(t *testing.T) {
	t.Parallel()
	r := runHelp(t, HelpCmd{Search: "environment variable naming precedence"}, false)
	if r.code != 0 || !strings.Contains(r.out, "configuration") || strings.Contains(r.out, "<mark>") || strings.Contains(r.out, "&#39;") {
		t.Errorf("search output:\n%s", r.out)
	}
	none := runHelp(t, HelpCmd{Search: "zzzqqqxxx"}, false)
	if none.code != 0 || !strings.Contains(none.out, "No matches") {
		t.Errorf("no-match output: %q", none.out)
	}
}

// TestHelp_Paging: a terminal gets the pager; piped output — what an agent
// reads — never does.
func TestHelp_Paging(t *testing.T) {
	t.Parallel()
	piped := runHelp(t, HelpCmd{Topic: "configuration"}, false)
	tty := runHelp(t, HelpCmd{Topic: "configuration"}, true)
	if piped.paged != "" || piped.out == "" {
		t.Error("piped output must be written directly")
	}
	if tty.out != "" || tty.paged != piped.out {
		t.Error("terminal output must go through the pager, unchanged")
	}
}

// TestCommandDetails_NameRealTopics: every command's --help points at a guide
// topic that exists.
func TestCommandDetails_NameRealTopics(t *testing.T) {
	t.Parallel()
	g := mustGuide(t)
	for _, n := range testModel(t).Children {
		if n.Name == "help" {
			continue
		}
		topic, ok := strings.CutPrefix(n.Detail, "Guide: gomddoc help ")
		if !ok {
			t.Errorf("%s: Detail %q does not name a guide topic", n.Name, n.Detail)
			continue
		}
		name, section, _ := strings.Cut(topic, " ")
		if _, found := g.Lookup(name); !found {
			t.Errorf("%s: Detail names %q, which is not a topic", n.Name, name)
			continue
		}
		if section != "" {
			if _, err := g.Section(name, section); err != nil {
				t.Errorf("%s: Detail names section %q of %s: %v", n.Name, section, name, err)
			}
		}
	}
}

func mustGuide(t *testing.T) *guide.Guide {
	t.Helper()
	g, err := guide.New(docs.Guide)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestHelp_PagerFailureFallsBack: a terminal whose $PAGER cannot run still
// gets the text. No t.Parallel: t.Setenv.
func TestHelp_PagerFailureFallsBack(t *testing.T) {
	t.Setenv("PAGER", "/definitely/not/a/pager")
	var out bytes.Buffer
	cmd := HelpCmd{Topic: "doctor", out: &out, isTerminal: func() bool { return true }}
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "# Doctor\n") {
		t.Errorf("fallback output:\n%.100s", out.String())
	}
	if stdoutIsTerminal() {
		t.Error("go test's stdout is not a terminal")
	}
}
