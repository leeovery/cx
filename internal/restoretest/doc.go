// Package restoretest provides shared test scaffolding for portal's reboot
// round-trip integration tests. Production code must not import it.
//
// Mixed build-tag layout: a helper carries the integration tag only when every
// caller does, so an untagged unit-lane test can still reach it; helpers that
// need a built portal binary are integration-only. A new helper goes in
// whichever file matches its callers' lanes, and general-purpose `go build`
// plumbing belongs in the sibling internal/portalbintest instead.
package restoretest
