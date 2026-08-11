// Package portaltest provides test-only helpers for running the portal CLI as a
// subprocess under per-test state-directory isolation, so a spawned process
// writes only inside a t.TempDir() and never to the developer's real install.
// A daemon-spawning test typically pairs it with portalbintest, which stages the
// binary on PATH.
//
// Test-only: production code must not import this package.
package portaltest
