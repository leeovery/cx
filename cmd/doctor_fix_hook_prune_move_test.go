package cmd

import (
	"bytes"
	"testing"

	"github.com/leeovery/portal/internal/hookstest"
)

// The cycle moved out of this package, and every line it drives a user to see
// is rendered here. These are the literal lines `portal doctor --fix` prints,
// written out rather than composed from the vocabularies that render them, so a
// re-homing that changed the words could not agree with its own tables and pass.
func TestDoctorFixHookPruneRendersTheSameLinesAfterTheMove(t *testing.T) {
	t.Run("it renders the same doctor --fix output after the move", func(t *testing.T) {
		cases := []struct {
			name string
			deps func(*testing.T) *DoctorDeps
			want string
		}{
			{
				name: "a reaped key",
				deps: func(t *testing.T) *DoctorDeps {
					deps, _, _, _, _ := seedStalePruneFixture(t, t.TempDir(), staleHookLister())
					return deps
				},
				want: "Pruned stale hook: " + hookstest.ReapableSeedA + "\n",
			},
			{
				name: "a stand-down",
				deps: func(t *testing.T) *DoctorDeps {
					deps, _, _, _, _ := seedStalePruneFixture(t, t.TempDir(), restoringHookLister())
					return deps
				},
				want: "Skipped stale hook prune: restore in progress\n",
			},
			{
				name: "a failed sweep",
				deps: failingSweepDeps,
				want: "Skipped stale hook prune: the sweep could not complete\n",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var out bytes.Buffer
				pruneDoctorStaleHooks(&out, tc.deps(t))

				if got := out.String(); got != tc.want {
					t.Errorf("doctor --fix hook prune wrote %q, want %q", got, tc.want)
				}
			})
		}
	})
}
