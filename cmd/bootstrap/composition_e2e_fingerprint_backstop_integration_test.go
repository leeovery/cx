//go:build integration

package bootstrap_test

import (
	"testing"
)

func TestCompositeBootstrap_FingerprintBackstopRunsClean(t *testing.T) {
	h := setupCompositeHarness(t)

	if h.StateDir == "" {
		t.Fatalf("harness returned empty StateDir; setupCompositeHarness contract broken")
	}
}
