package text

// Closest is the candidate nearest to s within maxDist edits (Levenshtein);
// ties go to the first in candidates' order. It backs "did you mean" hints.
func Closest(s string, candidates []string, maxDist int) (string, bool) {
	best, bestDist := "", maxDist+1
	for _, c := range candidates {
		if d := levenshtein(s, c); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best, bestDist <= maxDist
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
