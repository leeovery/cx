// Package transienttest provides shared scaffolding for tests that drive a
// failing `list-panes -a`, the hook-key seed vocabulary those and the reaper's
// own suites are written against, and the hooks.json lock fixture — sidecar
// creation, exclusive and shared holds, and the degraded-read breadcrumb
// assertion — that the store's and the CLI's lock suites share. It lives
// outside _test.go so any package's tests can import it; production code must
// not.
package transienttest
