// Package restoretest provides shared test scaffolding for portal's reboot
// round-trip integration tests. Production code must not import it.
//
// The package has a mixed build-tag layout: helpers needing tmux or a built
// portal binary live in integration-tagged files, and pure stdlib + testing
// helpers (the sessions.json seeders, the file poller, the on-disk audit-trail
// logger) omit the tag so they run under the unit lane. A new helper goes in
// whichever file matches its dependency surface.
//
// General-purpose `go build` plumbing lives in the sibling
// internal/portalbintest package instead — it has no semantic tie to restore.
package restoretest
