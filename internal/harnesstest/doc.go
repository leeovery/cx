// Package harnesstest provides the stand-in a fatal-on-failure test helper is
// driven by when its own failing paths are the subject: TestingT, the subset of
// *testing.T such a helper needs, and Recorder, which records what the helper
// reported instead of failing the test running it.
//
// It is subject-neutral on purpose. The stand-in belongs to no one domain, so
// homing it here keeps a package whose subject is bytes, or Go source, or tmux
// from importing an unrelated helper package for it.
//
// It depends on stdlib alone and carries no build tag, so every suite it serves
// runs in the unit lane. Test-only: production code must not import it.
package harnesstest
