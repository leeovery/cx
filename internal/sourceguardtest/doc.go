// Package sourceguardtest provides the Go-source scanning primitives shared by
// the guards that police a structural rule by reading the repository's own .go
// files rather than by executing them. It depends on stdlib alone and carries no
// build tag, so every guard it serves runs in the unit lane.
//
// Test-only: production code must not import it. The trailing "test" in the name
// carries that boundary at the import line, as it does for every sibling helper
// package.
//
// These primitives stay here rather than folding back into portalbintest: guards
// across several packages share them, and portalbintest builds binaries, which a
// package serving unit-lane guards must not drag in.
package sourceguardtest
