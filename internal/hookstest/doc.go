// Package hookstest provides the shared scaffolding for staging and
// interrogating hooks.json in tests — the seed hook-key vocabulary, the store
// seeders and path resolution, the byte-identity assertion a route that must
// write nothing is held to, and the sidecar lock fixture with the two
// breadcrumb assertions its contract is read through: the degraded read's DEBUG
// line, and the WARN a mutation that could not take the sidecar leaves.
// StageStore is the one path-based route: a test that stages a hooks.json to
// hand a store to a seam describes it there rather than authoring the file, its
// sidecar and its permissions itself. It lives outside _test.go so any
// package's tests can import it; production code must not.
package hookstest
