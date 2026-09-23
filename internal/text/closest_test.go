package text

import "testing"

func TestClosest(t *testing.T) {
	t.Parallel()
	candidates := []string{"description", "domain", "robots", "title"}
	tests := []struct {
		s, want string
		max     int
		ok      bool
	}{
		{"titel", "title", 2, true},
		{"domian", "domain", 2, true},
		{"zzz", "", 2, false},
		{"title", "title", 0, true},
		{"robot", "robots", 1, true},
	}
	for _, tt := range tests {
		got, ok := Closest(tt.s, candidates, tt.max)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Closest(%q, %d) = %q, %v; want %q, %v", tt.s, tt.max, got, ok, tt.want, tt.ok)
		}
	}
	// Ties go to the first candidate in order.
	if got, _ := Closest("ab", []string{"ax", "ay"}, 1); got != "ax" {
		t.Errorf("tie: %q", got)
	}
}
