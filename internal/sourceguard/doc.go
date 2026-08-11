// Package sourceguard provides the Go-source scanning primitives portal's
// source guards are written against — the tests that police a structural rule
// by reading the repository's own .go files rather than by executing them.
//
// It holds the decisions a guard would otherwise each make for itself, so
// guards that read as siblings cannot cover subtly different ground:
//
//   - GoSourceFiles — the .go enumeration a repo-wide guard walks.
//   - PackageGoFiles — the .go enumeration a package-local guard walks.
//   - ForEachFuncCall — the call-expression traversal a call-scanning guard
//     applies to a parsed file.
//
// The package depends on stdlib alone (go/ast plus filesystem plumbing), so any
// test package can import it without dragging in build machinery, tmux or state
// fixtures, and it carries no build tag — every guard it serves runs in the
// unit lane.
//
// Test-only: production code MUST NOT import this package. Enforcement is
// contributor discipline (matches the precedent for tmuxtest / restoretest /
// portalbintest).
package sourceguard
