package text

import (
	"log/slog"
	"testing"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "clean string", input: "/docs/readme.md", want: "/docs/readme.md"},
		{name: "empty string", input: "", want: ""},
		{name: "tabs preserved", input: "key\tvalue", want: "key\tvalue"},
		{name: "newline escaped", input: "line1\nline2", want: `line1\nline2`},
		{name: "carriage return escaped", input: "line1\rline2", want: `line1\rline2`},
		{name: "crlf escaped", input: "line1\r\nline2", want: `line1\r\nline2`},
		{name: "null byte escaped", input: "before\x00after", want: `before\x00after`},
		{name: "other control char", input: "hello\x07world", want: `hello\x07world`},
		{name: "C1 control char", input: "test\u0085end", want: `test\x85end`},
		{name: "unicode preserved", input: "/docs/café.md", want: "/docs/café.md"},
		{name: "emoji preserved", input: "hello 😀 world", want: "hello 😀 world"},
		{name: "multiple control chars", input: "\x01\x02\x03", want: `\x01\x02\x03`},
		{name: "mixed content", input: "/path\nfake-log-entry\x00end", want: `/path\nfake-log-entry\x00end`},
		{
			name:  "log injection attempt",
			input: "/safe\n2024-01-01 INFO Fake log entry",
			want:  `/safe\n2024-01-01 INFO Fake log entry`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sanitize(tt.input)
			if got != tt.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitize_NoAllocation_CleanString(t *testing.T) {
	clean := "/docs/readme.md"
	allocs := testing.AllocsPerRun(100, func() {
		_ = Sanitize(clean)
	})
	if allocs > 0 {
		t.Errorf("Sanitize(clean string) allocated %v times, want 0", allocs)
	}
}

func TestSafeString_LogValue(t *testing.T) {
	s := SafeString("path/with\nnewline")
	val := s.LogValue()
	if val.Kind() != slog.KindString {
		t.Fatalf("LogValue().Kind() = %v, want String", val.Kind())
	}
	want := `path/with\nnewline`
	if val.String() != want {
		t.Errorf("LogValue().String() = %q, want %q", val.String(), want)
	}
}

func TestSafe_Attr(t *testing.T) {
	attr := Safe("path", "/test\r\ninjection")
	if attr.Key != "path" {
		t.Errorf("attr.Key = %q, want %q", attr.Key, "path")
	}
	// Resolve the LogValuer
	val := attr.Value.Resolve()
	want := `/test\r\ninjection`
	if val.String() != want {
		t.Errorf("resolved value = %q, want %q", val.String(), want)
	}
}

func BenchmarkSanitize_Clean(b *testing.B) {
	s := "/docs/architecture/overview.md"
	for b.Loop() {
		_ = Sanitize(s)
	}
}

func BenchmarkSanitize_WithControlChars(b *testing.B) {
	s := "/docs/path\nwith\rnewlines\x00and\x07controls"
	for b.Loop() {
		_ = Sanitize(s)
	}
}
