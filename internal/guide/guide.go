// Package guide is gomddoc's embedded user guide as a set of topics: list,
// read, read one section, search. `gomddoc help` and the gomddoc_guide MCP
// tool both go through it, so a topic name, a page and a search hit are the
// same answer in the terminal and over MCP.
//
// The contract a caller gets wrong is the lookup: a page is reachable only by
// a topic name or a path from Topics — the topic list is the allowlist, so
// "../go.mod" or a directory is ErrUnknownTopic whatever the fs.FS would make
// of it.
package guide

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/search"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// ErrUnknownTopic is returned for a name or path that is not a guide page.
var ErrUnknownTopic = errors.New("unknown guide topic")

// Topic is one guide page.
type Topic struct {
	Name        string `json:"topic"` // "configuration"
	Path        string `json:"path"`  // "02-configuration.md"
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Hit is one search result.
type Hit struct {
	Topic   string `json:"topic"`
	Path    string `json:"path"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

// Guide is the embedded guide, indexed.
type Guide struct {
	fsys   fs.FS
	topics []Topic
	meta   *metadata.Index

	// index builds the search index on first use and shares it
	// (sync.OnceValues): a cold burst of searches builds it once.
	index func() (*search.Index, error)
}

var numberPrefix = regexp.MustCompile(`^\d+-`)

// TopicName is a page's topic: the file name without its number prefix and
// extension ("02-configuration.md" → "configuration"); a README names its
// directory ("12-advanced/README.md" → "advanced", the root one "overview").
func TopicName(p string) string {
	base := path.Base(p)
	if base == "README.md" {
		dir := path.Dir(p)
		if dir == "." {
			return "overview"
		}
		base = path.Base(dir)
	}
	return numberPrefix.ReplaceAllString(strings.TrimSuffix(base, ".md"), "")
}

// New indexes fsys's pages by their frontmatter.
func New(fsys fs.FS) (*Guide, error) {
	meta, err := metadata.BuildIndex(context.Background(), fsys, nil)
	if err != nil {
		return nil, fmt.Errorf("guide index: %w", err)
	}
	g := &Guide{fsys: fsys, meta: meta}
	for _, p := range meta.AllPages() {
		rel := strings.TrimPrefix(p.Path, "/")
		g.topics = append(g.topics, Topic{TopicName(rel), rel, p.Title, p.Description})
	}
	slices.SortFunc(g.topics, func(a, b Topic) int { return cmp.Compare(readingOrder(a.Path), readingOrder(b.Path)) })
	g.index = sync.OnceValues(func() (*search.Index, error) {
		idx, err := search.BuildIndex(context.Background(), fsys, meta, nil)
		if err != nil {
			return nil, fmt.Errorf("guide search index: %w", err)
		}
		return idx, nil
	})
	return g, nil
}

// readingOrder sorts a directory's README before its pages, numbered pages
// in their numbered order.
func readingOrder(p string) string {
	return strings.TrimSuffix(p, "README.md")
}

// Topics lists every page in reading order.
func (g *Guide) Topics() []Topic { return slices.Clone(g.topics) }

// Lookup finds a topic by name or path.
func (g *Guide) Lookup(ref string) (Topic, bool) {
	i := slices.IndexFunc(g.topics, func(t Topic) bool { return t.Name == ref || t.Path == ref })
	if i < 0 {
		return Topic{}, false
	}
	return g.topics[i], true
}

// Suggest is the topic name nearest to ref, for "did you mean".
func (g *Guide) Suggest(ref string) (string, bool) {
	names := make([]string, len(g.topics))
	for i, t := range g.topics {
		names[i] = t.Name
	}
	return text.Closest(ref, names, 3)
}

// Page returns a topic's markdown with the frontmatter stripped.
func (g *Guide) Page(ref string) ([]byte, Topic, error) {
	raw, t, err := g.raw(ref)
	if err != nil {
		return nil, Topic{}, err
	}
	return text.StripFrontmatter(raw), t, nil
}

// Section returns one section of a topic by heading anchor; an unknown anchor
// is text.ErrSectionNotFound.
func (g *Guide) Section(ref, id string) ([]byte, error) {
	raw, _, err := g.raw(ref)
	if err != nil {
		return nil, err
	}
	return text.ExtractSection(raw, id)
}

// HeadingIDs lists a topic's section anchors.
func (g *Guide) HeadingIDs(ref string) ([]string, error) {
	raw, _, err := g.raw(ref)
	if err != nil {
		return nil, err
	}
	return text.HeadingIDs(raw), nil
}

func (g *Guide) raw(ref string) ([]byte, Topic, error) {
	t, ok := g.Lookup(ref)
	if !ok {
		return nil, Topic{}, fmt.Errorf("%w: %q", ErrUnknownTopic, truncate(ref))
	}
	data, err := fs.ReadFile(g.fsys, t.Path)
	return data, t, err
}

// Search ranks the guide's pages for query. No hits is an empty slice.
func (g *Guide) Search(query string, limit int) ([]Hit, error) {
	idx, err := g.index()
	if err != nil {
		return nil, err
	}
	hits := []Hit{}
	for _, r := range idx.Search(query, limit) {
		p := strings.TrimPrefix(r.Path, "/")
		hits = append(hits, Hit{TopicName(p), p, r.Title, r.Snippet})
	}
	return hits, nil
}

// truncate keeps an echoed argument short.
func truncate(s string) string {
	const n = 80
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "") + "…"
}
