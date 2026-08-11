package cmd

// Tests driving PersistentPreRunE must call this first: a gate carried over
// from a prior test short-circuits the orchestrator call and surfaces as a
// flaky "Run was not called" assertion. It also re-resets on cleanup so the
// next test starts clean.

import (
	"sync"
	"testing"
)

func resetBootstrapOnce(t *testing.T) {
	t.Helper()
	bootstrapOnce = sync.Once{}
	bootstrapStarted = false
	bootstrapWarningsSlice = nil
	bootstrapErr = nil
	bootstrapWarnings = &BootstrapWarningsSink{}
	t.Cleanup(func() {
		bootstrapOnce = sync.Once{}
		bootstrapStarted = false
		bootstrapWarningsSlice = nil
		bootstrapErr = nil
		bootstrapWarnings = &BootstrapWarningsSink{}
	})
}
