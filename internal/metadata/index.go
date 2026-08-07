package metadata

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"iter"
	"log/slog"
	"maps"
	stdpath "path"
	"runtime"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// PageInfo holds metadata extracted from a Markdown file's frontmatter.
type PageInfo struct {
	Path        string         `json:"path"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Date        time.Time      `json:"date"`
	Meta        map[string]any `json:"meta,omitempty"`
}

// Index aggregates metadata from all Markdown files in a content root.
type Index struct {
	pages  []PageInfo
	byTag  map[string][]int // tag -> page indices
	byPath map[string]int   // path -> page index
}

// BuildIndex walks the given filesystem, extracts frontmatter from all
// Markdown files, and builds a metadata index. Hidden directories and
// files (starting with '.') are skipped. Parse errors on individual
// files are logged but do not cause the build to fail.
//
// Frontmatter parsing runs concurrently, bounded by runtime.NumCPU().
func BuildIndex(ctx context.Context, rootFS fs.FS, excludePatterns []string) (*Index, error) {
	// Phase 1: Collect all markdown file paths sequentially.
	var paths []string
	err := fs.WalkDir(rootFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk error at %s: %w", path, err)
		}

		if skip, skipErr := provider.SkipWalkEntry(path, d.Name(), d.IsDir(), excludePatterns); skip {
			return skipErr
		}

		if d.IsDir() {
			return nil
		}

		// Only collect Markdown files
		if ext := stdpath.Ext(d.Name()); ext != ".md" && ext != ".markdown" {
			return nil
		}

		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("building metadata index: %w", err)
	}

	// Phase 2: Parse frontmatter in parallel.
	type parseResult struct {
		page PageInfo
		ok   bool // false if file should be skipped
	}

	results := make([]parseResult, len(paths))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	for i, p := range paths {
		g.Go(func() error {
			if gctx.Err() != nil {
				return gctx.Err()
			}

			content, readErr := fs.ReadFile(rootFS, p)
			if readErr != nil {
				slog.Debug("skipping unreadable file during index build", "path", p, "error", readErr)
				return nil
			}

			fm, parseErr := extractFrontmatter(content)
			if parseErr != nil {
				slog.Debug("skipping file with malformed frontmatter", "path", p, "error", parseErr)
				return nil
			}
			if fm == nil {
				return nil // no frontmatter present
			}

			results[i] = parseResult{page: pageFromFrontmatter(p, fm), ok: true}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("building metadata index: %w", err)
	}

	// Phase 3: Merge results sequentially into the final index.
	idx := &Index{
		byTag:  make(map[string][]int),
		byPath: make(map[string]int),
	}

	for _, r := range results {
		if !r.ok {
			continue
		}
		pageIdx := len(idx.pages)
		idx.pages = append(idx.pages, r.page)
		idx.byPath[r.page.Path] = pageIdx
		for _, tag := range r.page.Tags {
			idx.byTag[tag] = append(idx.byTag[tag], pageIdx)
		}
	}

	return idx, nil
}

// pageFromFrontmatter converts parsed frontmatter into a PageInfo.
func pageFromFrontmatter(path string, fm map[string]any) PageInfo {
	page := PageInfo{
		Path: "/" + path,
		Meta: make(map[string]any),
	}

	if v, ok := fm["title"]; ok {
		if s, ok := v.(string); ok {
			page.Title = s
		}
	}
	if v, ok := fm["description"]; ok {
		if s, ok := v.(string); ok {
			page.Description = s
		}
	}
	if v, ok := fm["date"]; ok {
		switch d := v.(type) {
		case time.Time:
			page.Date = d
		case string:
			if t, err := time.Parse(time.DateOnly, d); err == nil {
				page.Date = t
			}
		}
	}
	if v, ok := fm["tags"]; ok {
		if tags, ok := v.([]any); ok {
			for _, tag := range tags {
				s, ok := tag.(string)
				if !ok {
					continue
				}
				normalized := normalizeTag(s)
				if normalized == "" {
					if t := strings.TrimSpace(s); t != "" && strings.ContainsAny(t, "/\\") {
						slog.Warn("Skipping tag with invalid character",
							slog.String("tag", s),
							slog.String("path", path),
							slog.String("reason", "tags may not contain '/' or '\\'"))
					}
					continue
				}
				if slices.Contains(page.Tags, normalized) {
					continue // drop duplicates so byTag holds each page at most once per tag
				}
				page.Tags = append(page.Tags, normalized)
			}
		}
	}

	// Store remaining fields in Meta
	knownKeys := map[string]bool{"title": true, "description": true, "date": true, "tags": true}
	for k, v := range fm {
		if !knownKeys[k] {
			page.Meta[k] = v
		}
	}
	if len(page.Meta) == 0 {
		page.Meta = nil
	}

	return page
}

// clonePage returns a deep copy of p. The Tags slice and Meta map are cloned so
// callers cannot mutate the index's internal state through the returned value.
func clonePage(p PageInfo) PageInfo {
	p.Tags = slices.Clone(p.Tags)
	p.Meta = maps.Clone(p.Meta)
	return p
}

// AllPages returns a defensive deep copy of all indexed pages: the slice and
// each page's Tags/Meta are cloned, so callers may freely mutate the result.
func (idx *Index) AllPages() []PageInfo {
	out := make([]PageInfo, len(idx.pages))
	for i := range idx.pages {
		out[i] = clonePage(idx.pages[i])
	}
	return out
}

// AllTags returns all unique tags, sorted alphabetically.
func (idx *Index) AllTags() []string {
	tags := make([]string, 0, len(idx.byTag))
	for tag := range idx.byTag {
		tags = append(tags, tag)
	}
	slices.Sort(tags)
	return tags
}

// ByPath returns the page at the given path, or nil if not found.
// The path should include a leading slash (e.g., "/docs/guide.md").
func (idx *Index) ByPath(path string) *PageInfo {
	i, ok := idx.byPath[path]
	if !ok {
		return nil
	}
	p := idx.pages[i]
	return &p
}

// TitleForPath returns the indexed title for an fs-relative path
// ("guide/setup.md"), or "" when the page is not indexed or has no frontmatter
// title. It is the title lookup navigation.Generator wants — pass it as a
// method value (navGen.SetTitleLookup(idx.TitleForPath)) rather than restating
// the leading-slash key convention at each call site. Unlike ByPath it reads
// the title in place, so labelling a tree does not copy a PageInfo per leaf.
func (idx *Index) TitleForPath(filePath string) string {
	i, ok := idx.byPath["/"+filePath]
	if !ok {
		return ""
	}
	return idx.pages[i].Title
}

// MaxTagLength bounds a tag arriving from outside the index — a URL path
// segment or a tool argument. Real tags are short, and nothing longer than this
// can match an indexed key, so LookupTag rejects it before normalizing a
// possibly huge input.
const MaxTagLength = 128

// LookupTag answers "is this a known tag, and what carries it" for every caller
// that takes a tag from a request: /tags/{tag}, /api/tags/{tag}, and the MCP
// tag resource. The bool is the single definition of a known tag — the three
// used to answer differently (404, 200 with [], and JSON null), which is the
// disagreement this method exists to make impossible. Pages come back sorted by
// title, so the HTML page, the JSON array and the static build agree on order
// too.
//
// A tag is in the index only because some page carries it, so !ok and an empty
// slice are the same condition; the bool is there to be read at the call site.
func (idx *Index) LookupTag(tag string) ([]PageInfo, bool) {
	if len(tag) > MaxTagLength {
		return nil, false
	}
	pages := idx.ByTag(tag)
	if len(pages) == 0 {
		return nil, false
	}
	slices.SortFunc(pages, CompareTitles)
	return pages, true
}

// ByTag returns all pages with the given tag, normalized the same way indexed
// tags are, so a lookup cannot miss a key over whitespace or case. Returns nil
// if no pages match. Callers holding a tag from a request want LookupTag.
func (idx *Index) ByTag(tag string) []PageInfo {
	indices, ok := idx.byTag[normalizeTag(tag)]
	if !ok {
		return nil
	}
	result := make([]PageInfo, len(indices))
	for i, pageIdx := range indices {
		result[i] = idx.pages[pageIdx]
	}
	return result
}

// PagesByTag iterates the pages carrying the given tag, normalized as ByTag
// normalizes it. Unlike ByTag it copies nothing — the index owns the
// PageInfo and the caller must not retain or mutate the pointer past the
// iteration. Prefer it when the caller only reads a few fields, or discards
// most of what it sees.
func (idx *Index) PagesByTag(tag string) iter.Seq[*PageInfo] {
	return func(yield func(*PageInfo) bool) {
		for _, pageIdx := range idx.byTag[normalizeTag(tag)] {
			if !yield(&idx.pages[pageIdx]) {
				return
			}
		}
	}
}

// normalizeTag lowercases and trims a single tag, returning "" if the result is
// empty or contains a path separator ('/' or '\'). It is the single rule shared
// by the metadata index and the tag-chip template helper.
func normalizeTag(s string) string {
	n := strings.ToLower(strings.TrimSpace(s))
	if n == "" || strings.ContainsAny(n, "/\\") {
		return ""
	}
	return n
}

// NormalizeTags normalizes a YAML-decoded frontmatter "tags" value into a
// canonical, deduplicated []string using normalizeTag. It accepts the []any
// (and []string) shapes yaml/v3 produces and returns nil for any other type or
// when no valid tags remain. This is the single source of truth for tag
// normalization, so chips, links, related docs, and the index always agree.
func NormalizeTags(raw any) []string {
	appendTag := func(out []string, s string) []string {
		if n := normalizeTag(s); n != "" && !slices.Contains(out, n) {
			return append(out, n)
		}
		return out
	}

	var out []string
	switch v := raw.(type) {
	case []any:
		out = make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = appendTag(out, s)
			}
		}
	case []string:
		out = make([]string, 0, len(v))
		for _, s := range v {
			out = appendTag(out, s)
		}
	default:
		return nil
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// CountByTag returns the number of pages with the given tag, normalized as
// ByTag normalizes it. Unlike ByTag it allocates nothing, so it is preferred
// when only the count is needed (e.g. the tags index page).
func (idx *Index) CountByTag(tag string) int {
	return len(idx.byTag[normalizeTag(tag)])
}

// CompareTitles is a case-insensitive comparator for use with slices.SortFunc.
// Pages with equal titles are ordered by path, so the result is stable and
// deterministic across builds.
func CompareTitles(a, b PageInfo) int {
	if c := text.CompareTitles(a.Title, b.Title); c != 0 {
		return c
	}
	return cmp.Compare(a.Path, b.Path)
}

// frontmatter delimiter
var fmDelimiter = []byte("---")

// extractFrontmatter extracts YAML frontmatter from Markdown content.
// Frontmatter must be delimited by --- at the start of the file.
// Returns nil, nil if no frontmatter is found.
func extractFrontmatter(content []byte) (map[string]any, error) {
	if !bytes.HasPrefix(bytes.TrimLeftFunc(content, isSpace), fmDelimiter) {
		return nil, nil
	}

	// Find opening delimiter
	start := bytes.Index(content, fmDelimiter)
	if start < 0 {
		return nil, nil
	}
	afterOpen := start + len(fmDelimiter)

	// Must be followed by newline
	if afterOpen >= len(content) || (content[afterOpen] != '\n' && content[afterOpen] != '\r') {
		return nil, nil
	}
	afterOpen++ // skip newline

	// Find closing delimiter
	rest := content[afterOpen:]
	closeIdx := -1
	for i := 0; i < len(rest); {
		lineEnd := bytes.IndexByte(rest[i:], '\n')
		var line []byte
		if lineEnd < 0 {
			line = rest[i:]
		} else {
			line = rest[i : i+lineEnd]
		}
		line = bytes.TrimRight(line, "\r")
		if bytes.Equal(bytes.TrimSpace(line), fmDelimiter) {
			closeIdx = i
			break
		}
		if lineEnd < 0 {
			break
		}
		i += lineEnd + 1
	}

	if closeIdx < 0 {
		return nil, nil
	}

	yamlContent := rest[:closeIdx]
	var result map[string]any
	if err := yaml.Unmarshal(yamlContent, &result); err != nil {
		return nil, fmt.Errorf("parsing frontmatter YAML: %w", err)
	}

	return result, nil
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}
