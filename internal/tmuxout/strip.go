// Package tmuxout provides dependency-free helpers for parsing `tmux show-*`
// output. It must stay a leaf — importing any other internal package would
// reintroduce the cycle it exists to break.
package tmuxout

// StripMatchedOuterQuotes removes s's first and last bytes only when they are a
// matched pair of single or double quotes, leaving inner content — including
// nested quotes — verbatim. Anything else is returned unchanged.
func StripMatchedOuterQuotes(s string) string {
	if len(s) < 2 {
		return s
	}
	first := s[0]
	last := s[len(s)-1]
	if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}
