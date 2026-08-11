// Package sourceguard provides the Go-source scanning primitives shared by the
// guards that police a structural rule by reading the repository's own .go files
// rather than by executing them. It depends on stdlib alone and carries no build
// tag, so every guard it serves runs in the unit lane. Test-only: production code
// must not import it.
package sourceguard
