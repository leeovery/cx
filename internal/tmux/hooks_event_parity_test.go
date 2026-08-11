package tmux_test

import (
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

func TestPortalManagedEventSetParity(t *testing.T) {
	registration := tmux.ManagedEventNames()
	teardown := tmux.PortalTeardownEvents()

	if len(registration) != len(teardown) {
		t.Fatalf("managed-event-set size mismatch: registration has %d events %v, teardown has %d events %v",
			len(registration), registration, len(teardown), teardown)
	}

	for i := range registration {
		if registration[i] != teardown[i] {
			t.Errorf("managed-event-set divergence at index %d: registration=%q teardown=%q\nregistration=%v\nteardown=%v",
				i, registration[i], teardown[i], registration, teardown)
		}
	}
}

func TestPortalTeardownFingerprintParity(t *testing.T) {
	teardown := tmux.PortalTeardownFingerprints()
	teardownSet := make(map[string]bool, len(teardown))
	for _, fp := range teardown {
		teardownSet[fp] = true
	}

	for _, fp := range tmux.ManagedEventFingerprintUnion() {
		if !teardownSet[fp] {
			t.Errorf("teardown fingerprint set %v is missing managedEvents fingerprint %q — "+
				"a registered category is unreachable by UnregisterPortalHooks (AC #5 seam)",
				teardown, fp)
		}
	}

	if !teardownSet[commitNowFingerprint] {
		t.Errorf("teardown fingerprint set %v is missing %q — the converged session-closed "+
			"commit-now hook would survive UnregisterPortalHooks", teardown, commitNowFingerprint)
	}

	if !teardownSet[tmux.MigrateRenameSubstring] {
		t.Errorf("teardown fingerprint set %v is missing the explicitly-retained legacy substring %q — "+
			"stale migrate-rename entries from old binaries would survive teardown",
			teardown, tmux.MigrateRenameSubstring)
	}

	for _, fp := range tmux.ManagedEventFingerprintUnion() {
		if fp == tmux.MigrateRenameSubstring {
			t.Errorf("managedEvents fingerprint union %v contains %q — registration must never "+
				"install/converge migrate-rename (it is teardown-retained only)",
				tmux.ManagedEventFingerprintUnion(), tmux.MigrateRenameSubstring)
		}
	}
}
