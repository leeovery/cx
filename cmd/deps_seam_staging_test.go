package cmd

import "testing"

// seamStagingCase names one package-level seam alongside the helper that stages
// it and a read of whether it is currently installed. The table ties each seam
// to its helper: the coverage guard compares these names against the seams the
// production sources actually declare, so a seam added without a helper fails
// rather than passing unnoticed.
type seamStagingCase struct {
	name    string
	install func(t *testing.T)
	isSet   func() bool
}

func seamStagingCases() []seamStagingCase {
	return []seamStagingCase{
		{"bootstrapDeps", func(t *testing.T) { withBootstrapDeps(t, BootstrapDeps{}) }, func() bool { return bootstrapDeps != nil }},
		{"commitNowDeps", func(t *testing.T) { withCommitNowDeps(t, CommitNowDeps{}) }, func() bool { return commitNowDeps != nil }},
		{"doctorDeps", func(t *testing.T) { withDoctorDeps(t, DoctorDeps{}) }, func() bool { return doctorDeps != nil }},
		{"hooksDeps", func(t *testing.T) { withHooksDeps(t, HooksDeps{}) }, func() bool { return hooksDeps != nil }},
		{"killDeps", func(t *testing.T) { withKillDeps(t, KillDeps{}) }, func() bool { return killDeps != nil }},
		{"listDeps", func(t *testing.T) { withListDeps(t, ListDeps{}) }, func() bool { return listDeps != nil }},
		{"openBurstDeps", func(t *testing.T) { withOpenBurstDeps(t, OpenBurstDeps{}) }, func() bool { return openBurstDeps != nil }},
		{"openDeps", func(t *testing.T) { withOpenDeps(t, OpenDeps{}) }, func() bool { return openDeps != nil }},
		{"uninstallDeps", func(t *testing.T) { withUninstallDeps(t, UninstallDeps{}) }, func() bool { return uninstallDeps != nil }},
	}
}

func TestSeamStagingHelpers(t *testing.T) {
	for _, sc := range seamStagingCases() {
		t.Run(sc.name, func(t *testing.T) {
			t.Run("it restores the seam when the test that installed it finishes", func(t *testing.T) {
				if sc.isSet() {
					t.Fatalf("%s was already installed before the case that installs it ran", sc.name)
				}
				t.Run("installs it", func(t *testing.T) {
					sc.install(t)
					if !sc.isSet() {
						t.Fatalf("%s is unset inside the test that installed it", sc.name)
					}
				})
				// The inner test's cleanups have run by the time t.Run returns.
				if sc.isSet() {
					t.Errorf("%s outlived the test that installed it; the staging helper did not register its restore", sc.name)
				}
			})
		})
	}
}

// TestWithoutHooksDeps pins the counterpart route: a case whose subject is the
// production default states that precondition rather than inheriting it, and
// leaves the seam unset behind it either way. A seam gets a without-helper when
// a suite's subject is its production default.
func TestWithoutHooksDeps(t *testing.T) {
	t.Run("it leaves the seam unset for a test that asks for the production default", func(t *testing.T) {
		t.Run("installs, then asks for the default", func(t *testing.T) {
			withHooksDeps(t, HooksDeps{})
			if hooksDeps == nil {
				t.Fatal("hooksDeps is unset inside the test that installed it")
			}
			withoutHooksDeps(t)
			if hooksDeps != nil {
				t.Error("hooksDeps is still installed after the test asked for the production default")
			}
		})
		if hooksDeps != nil {
			t.Error("hooksDeps outlived the test that asked for the production default")
		}
	})
}
