// Package diag is the one shape a detected problem takes: a Finding with a
// stable code, a severity, a location and a fix.
//
// The contract a producer gets wrong is where logging happens. A producer
// (config normalization, the clean-URL resolver, the redirect map, …) returns
// its findings and keeps its runtime behaviour; it does not log them. The
// caller logs them through Log. That split is what lets `gomddoc doctor`
// collect exactly the problems serve would log, instead of re-implementing the
// checks. Severity is never chosen at the call site: New looks it up in
// Catalogue, so a code means the same thing everywhere it is reported.
package diag

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"path"
	"slices"
)

// Severity ranks a finding.
type Severity string

// Severities, most severe first.
const (
	Error   Severity = "error"
	Warning Severity = "warning"
	Info    Severity = "info"
)

// Finding is one detected problem.
type Finding struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	File     string   `json:"file,omitempty"` // content-root relative
	Line     int      `json:"line,omitempty"` // 1-based; 0 when not applicable
	Key      string   `json:"key,omitempty"`  // config key, env var or frontmatter field
	Message  string   `json:"message"`
	Fix      string   `json:"fix,omitempty"`
}

// CodeInfo documents one finding code.
type CodeInfo struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Summary  string   `json:"summary"`
}

// Catalogue lists every finding code. The guide's doctor page is held to it by
// a drift test, in both directions.
var Catalogue = []CodeInfo{
	{"config.parse-error", Error, "config.yml is not valid YAML, or holds more than one document"},
	{"config.unknown-key", Error, "a config.yml key gomddoc does not define"},
	{"config.wrong-type", Error, "a config.yml value of the wrong type"},
	{"config.invalid-value", Error, "a config value gomddoc rejects"},
	{"config.value-replaced", Warning, "a config value replaced by its default because it was out of range"},
	{"env.invalid-value", Warning, "a GOMDDOC_* value that does not parse and is ignored"},
	{"env.unknown", Warning, "a GOMDDOC_* variable gomddoc does not read"},
	{"theme.not-installed", Warning, "the configured theme is not installed; the default theme renders"},
	{"theme.unknown-feature", Warning, "a feature toggle the active theme never reads"},
	{"theme.unknown-var", Info, "a theme variable the active theme's CSS never reads"},
	{"content.frontmatter-invalid", Error, "page frontmatter that is not valid YAML"},
	{"content.frontmatter-type", Error, "a special frontmatter field of the wrong type"},
	{"content.redirect-conflict", Error, "a redirect_from source claimed twice, or naming a real page"},
	{"content.path-collision", Warning, "two files claim one clean URL, or a file shadows a directory"},
	{"content.tags-collision", Warning, "content shadowed by the auto-generated /tags routes"},
	{"content.missing-title", Info, "a page with no title and no heading to derive one from"},
	{"content.missing-description", Info, "a page with no description"},
	{"exclude.matches-nothing", Warning, "an exclude pattern that matches no file or directory"},
	{"target.unreachable", Error, "the content directory or repository could not be opened"},
	{"target.read-error", Error, "a file could not be read while checking"},
}

// New builds a finding, taking its severity from Catalogue. An unknown code
// is a programming error and panics; the tests reach every call site.
func New(code, file string, line int, key, message, fix string) Finding {
	i := slices.IndexFunc(Catalogue, func(c CodeInfo) bool { return c.Code == code })
	if i < 0 {
		panic(fmt.Sprintf("diag: unknown finding code %q", code))
	}
	return Finding{Catalogue[i].Severity, code, file, line, key, message, fix}
}

// Log writes findings through slog: Error at Error, Warning at Warn, Info at
// Debug (informational findings are not worth a line in a server log).
func Log(findings []Finding) {
	for _, f := range findings {
		level := slog.LevelDebug
		switch f.Severity {
		case Error:
			level = slog.LevelError
		case Warning:
			level = slog.LevelWarn
		}
		attrs := []any{slog.String("code", f.Code)}
		if f.File != "" {
			attrs = append(attrs, slog.String("file", f.File))
		}
		if f.Line != 0 {
			attrs = append(attrs, slog.Int("line", f.Line))
		}
		if f.Key != "" {
			attrs = append(attrs, slog.String("key", f.Key))
		}
		slog.Log(context.Background(), level, f.Message, attrs...)
	}
}

func rank(s Severity) int {
	switch s {
	case Error:
		return 0
	case Warning:
		return 1
	default:
		return 2
	}
}

// Sort orders findings by severity, then file, line, code and key.
func Sort(findings []Finding) {
	slices.SortStableFunc(findings, func(a, b Finding) int {
		return cmp.Or(
			cmp.Compare(rank(a.Severity), rank(b.Severity)),
			cmp.Compare(a.File, b.File),
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Code, b.Code),
			cmp.Compare(a.Key, b.Key),
		)
	})
}

// Dedupe keeps one finding per code, file and key. Two producers can see one
// problem — a string redirect_from is both the redirect map's and doctor's
// frontmatter check's business — and only one of them may know the line, so
// the line is not part of the identity: the first located duplicate replaces
// an unlocated one, in the unlocated one's position.
func Dedupe(findings []Finding) []Finding {
	type id struct{ code, file, key string }
	at := map[id]int{}
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		k := id{f.Code, f.File, f.Key}
		if i, seen := at[k]; seen {
			if out[i].Line == 0 && f.Line != 0 {
				out[i] = f
			}
			continue
		}
		at[k] = len(out)
		out = append(out, f)
	}
	return out
}

// WithFilePrefix returns copies of findings with prefix joined onto each
// non-empty File: a language pipeline's paths are relative to its directory.
func WithFilePrefix(findings []Finding, prefix string) []Finding {
	out := slices.Clone(findings)
	if prefix == "" {
		return out
	}
	for i := range out {
		if out[i].File != "" {
			out[i].File = path.Join(prefix, out[i].File)
		}
	}
	return out
}
