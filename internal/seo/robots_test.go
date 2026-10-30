package seo

import "testing"

func TestNoindex(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		meta map[string]any
		want bool
	}{
		{"noindex", map[string]any{"robots": "noindex"}, true},
		{"noindex among directives", map[string]any{"robots": "noindex, nofollow"}, true},
		{"index", map[string]any{"robots": "index, follow"}, false},
		{"not a string", map[string]any{"robots": []any{"noindex"}}, false},
		{"absent", map[string]any{}, false},
		{"nil meta", nil, false},
	} {
		if got := Noindex(tt.meta); got != tt.want {
			t.Errorf("%s: Noindex(%v) = %v, want %v", tt.name, tt.meta, got, tt.want)
		}
	}
}
