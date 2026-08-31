package portaltest_test

import (
	"strings"
	"testing"
)

// The rule reads source, so each case below is a whole miniature fixture file.
const (
	fixtureFullyCovered = `package x

import "testing"

func TestX(t *testing.T) {
	env, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)
	ts := tmuxtest.New(t, "ptl-")
	_ = env
	_ = ts
}
`

	fixtureNoTeardownGuard = `package x

import "testing"

func TestX(t *testing.T) {
	env, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	ts := tmuxtest.New(t, "ptl-")
	_ = env
	_ = ts
}
`

	fixtureHandRolledStateDir = `package x

import "testing"

func TestX(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)
	ts := tmuxtest.New(t, "ptl-")
	_ = ts
}
`

	fixtureSharedArrangeStateDir = `package x

import "testing"

func TestX(t *testing.T) {
	env, stateDir := portaltest.IsolateStateForTest(t)
	ts := tmuxtest.New(t, "ptl-")
	_ = env
	_ = stateDir
	_ = ts
}
`

	fixtureNoServer = `package x

import "testing"

func TestX(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", stateDir)
}
`
)

func callsIn(t *testing.T, src string) fixtureCalls {
	t.Helper()
	calls, err := fixtureCallsIn(src)
	if err != nil {
		t.Fatalf("scan fixture source: %v", err)
	}
	return calls
}

func TestCoverageRuleFailsServerFixtureWithoutTeardownGuard(t *testing.T) {
	scanned, uncovered := auditFixtureCoverage(map[string]fixtureCalls{
		"no_guard_test.go": callsIn(t, fixtureNoTeardownGuard),
	})

	if scanned != 1 {
		t.Fatalf("scanned = %d, want 1", scanned)
	}
	if len(uncovered) != 1 || uncovered[0] != "no_guard_test.go" {
		t.Fatalf("uncovered = %v, want [no_guard_test.go]", uncovered)
	}
}

func TestCoverageRuleFailsHandRolledStateDirWithoutIsolation(t *testing.T) {
	scanned, uncovered := auditFixtureCoverage(map[string]fixtureCalls{
		"hand_rolled_test.go": callsIn(t, fixtureHandRolledStateDir),
	})

	if scanned != 1 {
		t.Fatalf("scanned = %d, want 1; a hand-rolled state dir must not be invisible", scanned)
	}
	if len(uncovered) != 1 || uncovered[0] != "hand_rolled_test.go" {
		t.Fatalf("uncovered = %v, want [hand_rolled_test.go]", uncovered)
	}
}

func TestCoverageRulePassesFixtureMakingAllThreeCalls(t *testing.T) {
	scanned, uncovered := auditFixtureCoverage(map[string]fixtureCalls{
		"covered_test.go": callsIn(t, fixtureFullyCovered),
	})

	if scanned != 1 {
		t.Fatalf("scanned = %d, want 1", scanned)
	}
	if len(uncovered) != 0 {
		t.Fatalf("uncovered = %v, want none", uncovered)
	}
	if msg := coverageFailure(scanned, uncovered); msg != "" {
		t.Fatalf("coverageFailure = %q, want empty", msg)
	}
}

func TestCoverageRuleFailsIsolatedFixtureTakingItsStateDirFromASharedArrange(t *testing.T) {
	calls := callsIn(t, fixtureSharedArrangeStateDir)
	if calls.NamesStateDir {
		t.Fatalf("fixture names %s; it must not, or it qualifies on the other arm", stateDirEnv)
	}

	scanned, uncovered := auditFixtureCoverage(map[string]fixtureCalls{
		"shared_arrange_test.go": calls,
	})

	if scanned != 1 {
		t.Fatalf("scanned = %d, want 1; a fixture that isolates and starts a server stays in reach "+
			"even when the state dir it uses is named in a shared arrange", scanned)
	}
	if len(uncovered) != 1 || uncovered[0] != "shared_arrange_test.go" {
		t.Fatalf("uncovered = %v, want [shared_arrange_test.go]", uncovered)
	}
}

func TestCoverageRuleFailsWhenNothingQualifies(t *testing.T) {
	scanned, uncovered := auditFixtureCoverage(map[string]fixtureCalls{
		"no_server_test.go": callsIn(t, fixtureNoServer),
	})

	if scanned != 0 {
		t.Fatalf("scanned = %d, want 0", scanned)
	}
	msg := coverageFailure(scanned, uncovered)
	if msg == "" {
		t.Fatal("coverageFailure = empty over zero qualifying files; the guard must fatal rather than pass")
	}
	if !strings.Contains(msg, "stopped looking") {
		t.Errorf("coverageFailure = %q; want it to say the guard has stopped looking", msg)
	}
}
