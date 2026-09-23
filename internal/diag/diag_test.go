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
	a := Finding{Severity: Error, Code: "c", File: "f", Line: 1, Key: "k", Message: "first"}
	dup := a
	dup.Message = "second"
	otherKey := a
	otherKey.Key = "k2"
	got := Dedupe([]Finding{a, dup, otherKey})
	if !slices.Equal(got, []Finding{a, otherKey}) {
		t.Errorf("Dedupe = %+v", got)
	}

	// A producer without line information and a check with it report the same
	// problem: one finding survives, the located one, in the first one's place.
	unlocated := Finding{Code: "c", File: "f", Key: "redirect_from", Message: "producer"}
	located := unlocated
	located.Line, located.Message = 4, "check"
	if got := Dedupe([]Finding{unlocated, otherKey, located}); !slices.Equal(got, []Finding{located, otherKey}) {
		t.Errorf("Dedupe(unlocated, located) = %+v", got)
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
