package diag

import (
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/testutil/logcapture"
)

func TestNew_SeverityFromCatalogue(t *testing.T) {
	t.Parallel()
	for code, want := range map[string]Severity{
		"config.unknown-key":          Error,
		"config.value-replaced":       Warning,
		"content.missing-description": Info,
	} {
		f := New(code, "a.md", 3, "k", "msg", "fix")
		if f.Severity != want || f.Code != code || f.File != "a.md" || f.Line != 3 || f.Key != "k" || f.Message != "msg" || f.Fix != "fix" {
			t.Errorf("New(%q) = %+v", code, f)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("New with an unknown code must panic")
		}
	}()
	New("nope", "", 0, "", "", "")
}

func TestCatalogue(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, c := range Catalogue {
		if seen[c.Code] {
			t.Errorf("duplicate code %s", c.Code)
		}
		seen[c.Code] = true
		if !slices.Contains([]Severity{Error, Warning, Info}, c.Severity) || c.Summary == "" {
			t.Errorf("%s: severity %q summary %q", c.Code, c.Severity, c.Summary)
		}
	}
	if len(Catalogue) != 20 {
		t.Errorf("catalogue has %d codes, the spec lists 20", len(Catalogue))
	}
}

func TestSort(t *testing.T) {
	t.Parallel()
	got := []Finding{
		{Severity: Info, Code: "c", File: "a.md"},
		{Severity: Warning, Code: "b", File: "b.md", Line: 2},
		{Severity: Error, Code: "z", File: "b.md", Line: 1},
		{Severity: Warning, Code: "a", File: "b.md", Line: 2},
		{Severity: Error, Code: "a", File: "a.md", Line: 9},
		{Severity: Warning, Code: "a", File: "b.md", Line: 2, Key: "x"},
	}
	Sort(got)
	want := []Finding{
		{Severity: Error, Code: "a", File: "a.md", Line: 9},
		{Severity: Error, Code: "z", File: "b.md", Line: 1},
		{Severity: Warning, Code: "a", File: "b.md", Line: 2},
		{Severity: Warning, Code: "a", File: "b.md", Line: 2, Key: "x"},
		{Severity: Warning, Code: "b", File: "b.md", Line: 2},
		{Severity: Info, Code: "c", File: "a.md"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Sort:\n got %+v\nwant %+v", got, want)
	}
}

func TestDedupe(t *testing.T) {
	t.Parallel()
	f := func(key string, line int, msg string) Finding {
		return Finding{Code: "c", File: "f", Key: key, Line: line, Message: msg}
	}
	tests := []struct {
		name string
		in   []Finding
		want []Finding
	}{
		{"exact repeat", []Finding{f("k", 1, "m"), f("k", 1, "m")}, []Finding{f("k", 1, "m")}},
		{"different key", []Finding{f("k", 1, "m"), f("k2", 1, "m")}, []Finding{f("k", 1, "m"), f("k2", 1, "m")}},
		// A producer without a line and a check with one report the same
		// problem: the located one survives, in the first one's place.
		{"unlocated merges into located", []Finding{f("redirect_from", 0, "producer"), f("x", 0, "other"), f("redirect_from", 4, "check")},
			[]Finding{f("redirect_from", 4, "check"), f("x", 0, "other")}},
		// Two different problems on one key, neither located (the resolver's
		// shadowing and extension collision on one clean path; two bad
		// strip_extensions entries): both kept.
		{"distinct unlocated problems", []Finding{f("a", 0, "shadows directory"), f("a", 0, "extension collision")},
			[]Finding{f("a", 0, "shadows directory"), f("a", 0, "extension collision")}},
		{"distinct located problems", []Finding{f("k", 2, "one"), f("k", 5, "two")}, []Finding{f("k", 2, "one"), f("k", 5, "two")}},
	}
	for _, tt := range tests {
		if got := Dedupe(tt.in); !slices.Equal(got, tt.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", tt.name, got, tt.want)
		}
	}
}

func TestWithFilePrefix(t *testing.T) {
	t.Parallel()
	in := []Finding{{File: "a.md"}, {File: ""}}
	got := WithFilePrefix(in, "fr-FR")
	if got[0].File != "fr-FR/a.md" || got[1].File != "" || in[0].File != "a.md" {
		t.Errorf("WithFilePrefix = %+v (input %+v)", got, in)
	}
	if same := WithFilePrefix(in, ""); !slices.Equal(same, in) {
		t.Errorf("empty prefix changed findings: %+v", same)
	}
}

func TestLog(t *testing.T) {
	log := logcapture.Install(t, slog.LevelDebug)
	Log([]Finding{
		New("config.invalid-value", ".gomddoc/config.yml", 4, "meta.domain", "bad domain", ""),
		New("config.value-replaced", "", 0, "server.http.write_timeout", "too long", ""),
		New("content.missing-title", "a.md", 0, "", "no title", ""),
	})
	if !log.Has("bad domain", slog.String("code", "config.invalid-value"), slog.String("file", ".gomddoc/config.yml"),
		slog.Int("line", 4), slog.String("key", "meta.domain")) {
		t.Errorf("error finding not logged with its attrs:\n%s", log)
	}
	out := log.String()
	for _, want := range []string{"ERROR bad domain", "WARN too long", "DEBUG no title"} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q:\n%s", want, out)
		}
	}
}

func TestYAMLLine(t *testing.T) {
	t.Parallel()
	for msg, want := range map[string]int{
		"yaml: line 3: did not find expected key": 3,
		"line 12: cannot unmarshal !!str":         12,
		"something else":                          0,
	} {
		if got := YAMLLine(msg); got != want {
			t.Errorf("YAMLLine(%q) = %d, want %d", msg, got, want)
		}
	}
}
