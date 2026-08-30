// Package transienttest provides the shared scaffolding for tests that drive a
// failing `list-panes -a`. It lives outside _test.go so any package's tests can
// import it; production code must not.
package transienttest
