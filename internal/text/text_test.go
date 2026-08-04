package text

import (
	"strings"
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

// referenceCompareTitles is the definition CompareTitles is optimized from.
// The fast path must agree with it on every input, including the sign.
func referenceCompareTitles(a, b string) int {
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// TestCompareTitles_MatchesReference pins the allocation-free comparator to the
// strings.ToLower definition it replaced: same ordering, same sign, on every
// pair drawn from a corpus of case, length, Unicode and prefix variations.
func TestCompareTitles_MatchesReference(t *testing.T) {
	t.Parallel()

	corpus := []string{
		"", "a", "A", "b", "B", "ab", "AB", "Ab", "aB", "abc", "ABC",
		"apple", "Apple", "APPLE", "applesauce", "Apple Pie", "apple pie",
		"Zebra", "zebra", "zebras",
		"école", "École", "ÉCOLE", "ecole",
		"Ünicode", "ünicode",
		"日本語", "Ω", "ω", "ﬀ",
		"page 1", "Page 1", "page 10", "Page 2",
		"_underscore", "-dash", "1digit", " leading space",
	}

	for _, a := range corpus {
		for _, b := range corpus {
			got, want := CompareTitles(a, b), referenceCompareTitles(a, b)
			if got != want {
				t.Errorf("CompareTitles(%q, %q) = %d, want %d", a, b, got, want)
			}
		}
	}
}

// TestCompareTitles_Allocates guards the property the optimization exists for:
// the whole point is that sorting titles no longer copies them.
// No t.Parallel: testing.AllocsPerRun panics in a parallel test.
func TestCompareTitles_Allocates(t *testing.T) {
	if allocs := testing.AllocsPerRun(100, func() {
		_ = CompareTitles("Getting Started With Éclairs", "getting started with eclairs")
	}); allocs != 0 {
		t.Errorf("CompareTitles allocated %v times per run, want 0", allocs)
	}
}

func BenchmarkCompareTitles(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = CompareTitles("Getting Started With Éclairs", "Getting Started With Eclairs")
	}
}
