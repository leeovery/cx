package spawn

import "testing"

func TestResultOutcomes_AllThreeDistinct(t *testing.T) {
	seen := map[Outcome]bool{}
	for _, r := range []Result{
		Success("s"),
		SpawnFailed("f"),
		PermissionRequired("p", "g"),
	} {
		seen[r.Outcome] = true
	}
	if len(seen) != 3 {
		t.Fatalf("expected 3 distinct Outcome values, got %d: %v", len(seen), seen)
	}

	if got := Success("").Outcome; got != OutcomeSuccess {
		t.Errorf("Success().Outcome = %v, want OutcomeSuccess", got)
	}
	if got := SpawnFailed("").Outcome; got != OutcomeSpawnFailed {
		t.Errorf("SpawnFailed().Outcome = %v, want OutcomeSpawnFailed", got)
	}
	if got := PermissionRequired("", "").Outcome; got != OutcomePermissionRequired {
		t.Errorf("PermissionRequired().Outcome = %v, want OutcomePermissionRequired", got)
	}
}

func TestResultOK_TrueOnlyForSuccess(t *testing.T) {
	if !Success("ok").OK() {
		t.Errorf("Success(...).OK() = false, want true")
	}
	for _, r := range []Result{
		SpawnFailed("f"),
		PermissionRequired("p", "g"),
	} {
		if r.OK() {
			t.Errorf("Result{Outcome: %v}.OK() = true, want false", r.Outcome)
		}
	}
}

func TestResultZeroValue_IsUnknownNotSuccess(t *testing.T) {
	var zero Outcome
	if zero != OutcomeUnknown {
		t.Errorf("zero Outcome = %v, want OutcomeUnknown", zero)
	}
	if (Result{}).OK() {
		t.Errorf("Result{}.OK() = true, want false (zero value must not be success)")
	}
}

func TestResult_RoundTripsDetailAndGuidance(t *testing.T) {
	r := PermissionRequired("evt -1743", "grant Automation for Ghostty")
	if r.Detail != "evt -1743" {
		t.Errorf("Detail = %q, want %q", r.Detail, "evt -1743")
	}
	if r.Guidance != "grant Automation for Ghostty" {
		t.Errorf("Guidance = %q, want %q", r.Guidance, "grant Automation for Ghostty")
	}

	for _, tc := range []struct {
		name   string
		got    Result
		detail string
	}{
		{"Success", Success("clean exit 0"), "clean exit 0"},
		{"SpawnFailed", SpawnFailed("AppleScript error body"), "AppleScript error body"},
	} {
		if tc.got.Detail != tc.detail {
			t.Errorf("%s Detail = %q, want %q", tc.name, tc.got.Detail, tc.detail)
		}
		if tc.got.Guidance != "" {
			t.Errorf("%s Guidance = %q, want empty", tc.name, tc.got.Guidance)
		}
	}
}
