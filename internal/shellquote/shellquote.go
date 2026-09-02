// Package shellquote holds the rule for rendering a string a shell will later
// word-split as exactly one word. It depends on the standard library alone, so
// packages that must not import each other — the restore engine, the session
// tree and the host-terminal spawn service among them — can each reach the rule
// without an edge between them.
package shellquote

import "strings"

// Single wraps s in POSIX single quotes so it survives as one word when a shell
// word-splits the string it is composed into. An embedded single quote is
// escaped with the close-escape-reopen idiom, since single quotes admit no
// backslash escape of their own; an empty s renders as an empty quoted word
// rather than disappearing.
func Single(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
