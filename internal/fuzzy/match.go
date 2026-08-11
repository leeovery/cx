// Package fuzzy provides subsequence-based fuzzy matching.
package fuzzy

import "strings"

// Match reports whether pattern is a subsequence of text: every character
// appears in order, though not necessarily consecutively.
func Match(text, pattern string) bool {
	pi := 0
	for i := 0; i < len(text) && pi < len(pattern); i++ {
		if text[i] == pattern[pi] {
			pi++
		}
	}
	return pi == len(pattern)
}

// Filter returns the items whose nameOf value fuzzy-matches filter,
// case-insensitively. An empty filter returns everything.
func Filter[T any](items []T, filter string, nameOf func(T) string) []T {
	if filter == "" {
		return items
	}
	lowerFilter := strings.ToLower(filter)
	var result []T
	for _, item := range items {
		if Match(strings.ToLower(nameOf(item)), lowerFilter) {
			result = append(result, item)
		}
	}
	return result
}
