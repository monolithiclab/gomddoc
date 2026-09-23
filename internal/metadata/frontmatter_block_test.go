package metadata

import "testing"

func TestFrontmatterBlock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		in     string
		want   string
		wantOK bool
	}{
		{"none", "# Title\n", "", false},
		{"valid", "---\ntitle: A\ntags: [x]\n---\n# A\n", "title: A\ntags: [x]\n", true},
		{"crlf", "---\r\ntitle: A\r\n---\r\nbody", "title: A\r\n", true},
		{"leading spaces", "  ---\ntitle: A\n---\n", "title: A\n", true},
		{"unterminated", "---\ntitle: A\n", "", false},
		{"empty block", "---\n---\nbody", "", true},
		{"delimiter not alone", "---x\ntitle: A\n---\n", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := FrontmatterBlock([]byte(tt.in))
			if string(got) != tt.want || ok != tt.wantOK {
				t.Errorf("FrontmatterBlock(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
