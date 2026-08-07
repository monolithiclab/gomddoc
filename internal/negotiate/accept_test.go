package negotiate

import (
	"testing"
)

func TestParseAccept(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		header    string
		wantOrder []string // Full MIME types in expected order
	}{
		{
			name:      "empty header defaults to */*",
			header:    "",
			wantOrder: []string{"*/*"},
		},
		{
			name:      "single type",
			header:    "text/html",
			wantOrder: []string{"text/html"},
		},
		{
			name:      "multiple types preserve order",
			header:    "text/html, application/json",
			wantOrder: []string{"text/html", "application/json"},
		},
		{
			name:      "quality factors sorted descending",
			header:    "text/html;q=0.8, application/json;q=0.9",
			wantOrder: []string{"application/json", "text/html"},
		},
		{
			name:      "stable sort with same q-values",
			header:    "a/a;q=0.5, b/b;q=0.5, c/c;q=0.8, d/d;q=0.8",
			wantOrder: []string{"c/c", "d/d", "a/a", "b/b"},
		},
		{
			name:      "mixed q-values with default",
			header:    "text/html;q=0.9, application/json;q=1.0, text/plain;q=0.8, image/png",
			wantOrder: []string{"application/json", "image/png", "text/html", "text/plain"},
		},
		{
			name:      "wildcard */*",
			header:    "*/*",
			wantOrder: []string{"*/*"},
		},
		{
			name:      "complex browser-like header",
			header:    "text/html, application/xhtml+xml, application/xml;q=0.9, image/webp, */*;q=0.8",
			wantOrder: []string{"text/html", "application/xhtml+xml", "image/webp", "application/xml", "*/*"},
		},
		{
			name:      "q=0 filtered out with other types",
			header:    "text/html;q=0, application/json",
			wantOrder: []string{"application/json"},
		},
		{
			name:      "single type with q=0 returns empty",
			header:    "text/html;q=0",
			wantOrder: []string{},
		},
		{
			name:      "mix of q=0 and q>0 entries",
			header:    "text/html;q=0, application/json;q=0.9, text/plain;q=0, image/png;q=0.5",
			wantOrder: []string{"application/json", "image/png"},
		},
		{
			name:      "quality factor with decimal places",
			header:    "text/html;q=0.95, application/json;q=0.85",
			wantOrder: []string{"text/html", "application/json"},
		},
		{
			name:      "whitespace handling",
			header:    "  text/html  ,  application/json  ",
			wantOrder: []string{"text/html", "application/json"},
		},
		{
			name:      "invalid entries skipped",
			header:    "text/html, invalid, application/json",
			wantOrder: []string{"text/html", "application/json"},
		},
		{
			name:      "other parameters ignored",
			header:    "text/html; charset=utf-8; q=0.9",
			wantOrder: []string{"text/html"},
		},
		{
			name:      "q>1 clamped to 1",
			header:    "text/html;q=1.5, application/json;q=0.9",
			wantOrder: []string{"text/html", "application/json"},
		},
		{
			name:      "negative q clamped to 0 and filtered",
			header:    "text/html;q=-0.5, application/json",
			wantOrder: []string{"application/json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseAccept(tt.header)

			if len(result) != len(tt.wantOrder) {
				t.Fatalf("ParseAccept() returned %d types, want %d", len(result), len(tt.wantOrder))
			}

			if len(tt.wantOrder) > 0 {
				for i, want := range tt.wantOrder {
					if got := result[i].Type + "/" + result[i].Subtype; got != want {
						t.Errorf("ParseAccept() result[%d] = %q, want %q", i, got, want)
					}
				}
			}
		})
	}
}
