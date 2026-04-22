package text

import (
	"testing"
)

func TestTitleCase(t *testing.T) {
	t.Parallel()

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

func TestTitleCase_Concurrent(t *testing.T) {
	t.Parallel()

	// Test concurrent access to TitleCase - this will catch race conditions
	// when run with -race flag
	const goroutines = 100
	const iterations = 100

	done := make(chan bool, goroutines)

	for range goroutines {
		go func() {
			for range iterations {
				result := TitleCase("hello world")
				if result != "Hello World" {
					t.Errorf("TitleCase concurrent access failed: got %q, want %q", result, "Hello World")
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for range goroutines {
		<-done
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
