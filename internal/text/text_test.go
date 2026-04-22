package text

import (
	"testing"
)

func TestTitleCase(t *testing.T) {
	// Note: Cannot use t.Parallel() - TitleCase() uses shared package-level state (caser with sync.Once)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple lowercase",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "multiple words",
			input:    "hello world",
			expected: "Hello World",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single character",
			input:    "a",
			expected: "A",
		},
		{
			name:     "unicode characters",
			input:    "école",
			expected: "École",
		},
		{
			name:     "mixed case",
			input:    "hELLo WoRLD",
			expected: "Hello World",
		},
		{
			name:     "with dashes",
			input:    "my-project",
			expected: "My-Project",
		},
		{
			name:     "with underscores",
			input:    "my_project",
			expected: "My_project", // Note: cases.Title only capitalizes after whitespace
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TitleCase(tt.input)
			if result != tt.expected {
				t.Errorf("TitleCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTitleCase_Caching(t *testing.T) {
	// Call TitleCase multiple times to ensure the caser is initialized only once
	// This is more of a smoke test - actual verification would require instrumentation
	for i := range 100 {
		result := TitleCase("test")
		if result != "Test" {
			t.Errorf("TitleCase failed after %d iterations: got %q, want %q", i, result, "Test")
		}
	}
}

func BenchmarkTitleCase(b *testing.B) {
	for b.Loop() {
		_ = TitleCase("hello world")
	}
}

func BenchmarkTitleCase_Unicode(b *testing.B) {
	for b.Loop() {
		_ = TitleCase("école français")
	}
}
