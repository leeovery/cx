// Package hookstest provides the shared scaffolding for staging and
// interrogating hooks.json in tests — the seed hook-key vocabulary, the store
// seeders and path resolution, and the sidecar lock fixture with its
// degraded-read breadcrumb assertion. It lives outside _test.go so any
// package's tests can import it; production code must not.
package hookstest
