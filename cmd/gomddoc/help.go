package main

import (
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/monolithiclab/gomddoc/docs"
	"github.com/monolithiclab/gomddoc/internal/guide"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// HelpCmd reads gomddoc's embedded guide in the terminal.
type HelpCmd struct {
	Topic   string `arg:"" optional:"" help:"Guide topic; run without arguments to list them."`
	Section string `arg:"" optional:"" help:"Section anchor within the topic, e.g. priority-order."`
	Search  string `name:"search" short:"s" help:"Search the guide."`

	out, errOut io.Writer          // nil: os.Stdout, os.Stderr
	isTerminal  func() bool        // nil: stdout is a character device
	page        func(string) error // nil: pipe through $PAGER
}

// Help is the command's detail line in `gomddoc help --help`.
func (h *HelpCmd) Help() string { return "Guide: gomddoc help overview" }

// Run lists topics, prints a topic or one of its sections, or searches. Output
// to a terminal goes through $PAGER; piped output — what an agent reads — is
// plain markdown, never paged.
func (h *HelpCmd) Run() error {
	g, err := guide.New(docs.Guide)
	if err != nil {
		return err
	}
	var b strings.Builder
	switch {
	case h.Search != "":
		writeSearch(&b, g, h.Search)
	case h.Topic == "":
		writeTopics(&b, g)
	default:
		if msg, ok := writeTopic(&b, g, h.Topic, h.Section); !ok {
			_, err := io.WriteString(orDefault(h.errOut, os.Stderr), msg)
			if err != nil {
				return err
			}
			return exitCodeError(1)
		}
	}
	return h.emit(b.String())
}

func (h *HelpCmd) emit(s string) error {
	isTerminal := h.isTerminal
	if isTerminal == nil {
		isTerminal = stdoutIsTerminal
	}
	if isTerminal() {
		page := h.page
		if page == nil {
			page = runPager
		}
		if err := page(s); err == nil {
			return nil
		}
		// No usable pager: fall through to plain output.
	}
	_, err := io.WriteString(orDefault(h.out, os.Stdout), s)
	return err
}

func writeTopics(b *strings.Builder, g *guide.Guide) {
	b.WriteString("gomddoc guide — gomddoc help <topic> [section]\n\n")
	tw := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	for _, t := range g.Topics() {
		fmt.Fprintf(tw, "  %s\t%s — %s\n", t.Name, t.Title, t.Description)
	}
	_ = tw.Flush() // a strings.Builder cannot fail
	b.WriteString("\nSearch the guide:        gomddoc help --search <query>\n" +
		"A command's flags:       gomddoc <command> --help\n" +
		"Every setting:           gomddoc info\n" +
		"Check a site:            gomddoc doctor\n")
}

// writeTopic writes a page or section; on a bad reference it returns the
// message for stderr instead.
func writeTopic(b *strings.Builder, g *guide.Guide, topic, section string) (string, bool) {
	if section != "" {
		sec, err := g.Section(topic, section)
		switch {
		case errors.Is(err, text.ErrSectionNotFound):
			ids, _ := g.HeadingIDs(topic)
			return fmt.Sprintf("No section %q in %s. Sections: %s\n", section, topic, strings.Join(ids, ", ")), false
		case err != nil:
			return unknownTopic(g, topic), false
		}
		b.Write(sec)
		b.WriteString("\n")
		return "", true
	}
	body, _, err := g.Page(topic)
	if err != nil {
		return unknownTopic(g, topic), false
	}
	b.WriteString(strings.TrimLeft(string(body), "\n")) // the blank line after the frontmatter
	return "", true
}

func unknownTopic(g *guide.Guide, topic string) string {
	var names []string
	for _, t := range g.Topics() {
		names = append(names, t.Name)
	}
	hint := ""
	if best, ok := g.Suggest(topic); ok {
		hint = fmt.Sprintf(" — did you mean `%s`?", best)
	}
	return fmt.Sprintf("Unknown topic %q%s\nTopics: %s\n", topic, hint, strings.Join(names, ", "))
}

var htmlTag = regexp.MustCompile(`</?mark>`)

func writeSearch(b *strings.Builder, g *guide.Guide, query string) {
	hits, err := g.Search(query, 10)
	if err != nil || len(hits) == 0 {
		fmt.Fprintf(b, "No matches for %q. List topics: gomddoc help\n", query)
		return
	}
	for _, h := range hits {
		// Snippets are built for the web search box: highlighted and escaped.
		snippet := html.UnescapeString(htmlTag.ReplaceAllString(h.Snippet, ""))
		fmt.Fprintf(b, "%s — %s\n    %s\n", h.Topic, h.Title, strings.Join(strings.Fields(snippet), " "))
	}
	b.WriteString("\nRead one: gomddoc help <topic>\n")
}

func stdoutIsTerminal() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// runPager pipes s through $PAGER (default `less -FRX`, which exits at once
// when the text fits on one screen).
func runPager(s string) error {
	fields := strings.Fields(os.Getenv("PAGER"))
	if len(fields) == 0 {
		fields = []string{"less", "-FRX"}
	}
	cmd := exec.Command(fields[0], fields[1:]...) // #nosec G204 G702 -- running the user's own $PAGER is the point, as git and man do
	cmd.Stdin = strings.NewReader(s)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func orDefault(w, def io.Writer) io.Writer {
	if w == nil {
		return def
	}
	return w
}
