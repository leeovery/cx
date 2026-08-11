// Package transienttest provides shared scaffolding for tests that drive a
// failing `list-panes -a`: the Commander injection primitive, a socket-anchored
// pass-through Commander, and hooks.json seeding.
//
// It lives outside `_test.go` so any package's tests can import it; production
// code must not.
package transienttest
