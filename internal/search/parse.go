package search

import "strings"

// tagPrefix marks a query token as a tag filter (e.g. "tag:deployment").
const tagPrefix = "tag:"

// parseQuery splits a raw search query into tag filters and free-text terms.
//
// A token is a tag filter only when the raw token (before tokenization) has a
// non-empty "tag:" prefix; its value is lowercased to match metadata.Index
// storage. The literal token "tag:" (empty value) is dropped entirely.
// Remaining tokens are rejoined with single spaces as the free-text query.
//
// Tag values are extracted before tokenization, so they bypass the tokenizer's
// 2-char minimum and letter/digit-only normalization — a single-char tag or a
// tag containing dots/hyphens filters correctly.
func parseQuery(query string) (tags []string, freeText string) {
	var textTokens []string
	for tok := range strings.FieldsSeq(query) {
		if value, ok := strings.CutPrefix(tok, tagPrefix); ok {
			if value != "" {
				tags = append(tags, strings.ToLower(value))
			}
			continue
		}
		textTokens = append(textTokens, tok)
	}
	return tags, strings.Join(textTokens, " ")
}
