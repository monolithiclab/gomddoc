package negotiate

import "testing"

func BenchmarkParseAccept(b *testing.B) {
	tests := []struct {
		name   string
		header string
	}{
		{"Empty", ""},
		{"Single", "text/html"},
		{"Multiple", "text/html, application/json;q=0.9, */*;q=0.1"},
		{"Browser", "text/html, application/xhtml+xml, application/xml;q=0.9, image/webp, */*;q=0.8"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				ParseAccept(tt.header)
			}
		})
	}
}
