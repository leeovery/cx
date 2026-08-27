// Package transienttest provides shared scaffolding for tests that drive a
// failing `list-panes -a`, plus the hook-key seed vocabulary those and the
// reaper's own suites are written against. It lives outside _test.go so any
// package's tests can import it; production code must not.
package transienttest
