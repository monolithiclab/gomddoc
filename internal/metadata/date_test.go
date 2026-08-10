package metadata

import (
	"testing"
	"time"
)

func TestParseFrontmatterDate(t *testing.T) {
	t.Parallel()

	want := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		in   any
		want time.Time
	}{
		// An unquoted `date: 2024-01-02` reaches us already decoded.
		{"time.Time passes through", want, want},
		// A quoted one does not.
		{"date-only string", "2024-01-02", want},
		{"rfc3339 string", "2024-01-02T03:04:05Z", time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)},
		{"unparseable string", "last Tuesday", time.Time{}},
		{"absent key", nil, time.Time{}},
		// YAML yields an int for `date: 2024`; the type switch must not panic
		// or coerce it into a year.
		{"wrong type", 2024, time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ParseFrontmatterDate(tt.in); !got.Equal(tt.want) {
				t.Errorf("ParseFrontmatterDate(%#v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
