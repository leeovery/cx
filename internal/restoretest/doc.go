// Package restoretest provides shared test scaffolding for portal's reboot
// round-trip integration tests. Production code must not import it.
//
// Mixed build-tag layout: helpers needing tmux or a built portal binary live in
// integration-tagged files; pure stdlib + testing helpers omit the tag so they
// run in the unit lane. A new helper goes in whichever file matches its
// dependency surface, and general-purpose `go build` plumbing belongs in the
// sibling internal/portalbintest instead.
package restoretest
