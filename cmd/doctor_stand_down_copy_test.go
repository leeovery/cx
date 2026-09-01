// The user-facing copy for a stood-down hook prune, pinned on both surfaces it
// reaches: the repair line `portal doctor --fix` prints for a user who asked
// for a repair, and the not-evaluable detail the read-only diagnosis reports
// for the same window. They are pinned together because the two must name the
// same condition in their own register, and because a branch that silently
// borrows another's words is exactly the drift this suite exists to catch.
package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/hookstest"
)

// standDownCopyCase is one stand-down reason and the exact line each surface
// renders for it. skippedLine completes "Skipped stale hook prune: …";
// notEvaluableLine is the whole rendered check line, glyph included.
type standDownCopyCase struct {
	name             string
	reason           string
	deps             func(t *testing.T) *DoctorDeps
	skippedLine      string
	notEvaluableLine string
}

func standDownCopyCases() []standDownCopyCase {
	return []standDownCopyCase{
		{
			name:             "restore window",
			reason:           skipReasonRestoring,
			deps:             func(t *testing.T) *DoctorDeps { return restoringStandDownDeps(t) },
			skippedLine:      "Skipped stale hook prune: restore in progress",
			notEvaluableLine: "  · stale hooks: restore in progress (not evaluable)",
		},
		{
			name:             "restore marker unreadable",
			reason:           skipReasonMarkerReadFailed,
			deps:             func(t *testing.T) *DoctorDeps { return markerReadFailedStandDownDeps(t) },
			skippedLine:      "Skipped stale hook prune: could not read the restore marker",
			notEvaluableLine: "  · stale hooks: could not read the restore marker",
		},
		{
			name:             "hooks.json unreadable",
			reason:           skipReasonStoreReadFailed,
			deps:             func(t *testing.T) *DoctorDeps { return storeReadFailedStandDownDeps(t) },
			skippedLine:      "Skipped stale hook prune: could not read hooks.json",
			notEvaluableLine: "  · stale hooks: could not read hooks.json",
		},
		{
			name:             "pane enumeration failed",
			reason:           skipReasonPaneReadFailed,
			deps:             func(t *testing.T) *DoctorDeps { return paneReadFailedStandDownDeps(t) },
			skippedLine:      "Skipped stale hook prune: could not enumerate live panes",
			notEvaluableLine: "  · stale hooks: could not enumerate live panes",
		},
		{
			name:             "empty pane list",
			reason:           skipReasonEmptyPaneRead,
			deps:             func(t *testing.T) *DoctorDeps { return emptyPaneReadStandDownDeps(t) },
			skippedLine:      "Skipped stale hook prune: live pane list came back empty",
			notEvaluableLine: "  · stale hooks: zero live panes with hooks present (not evaluable)",
		},
		{
			name:             "hooks.json locked",
			reason:           skipReasonLockTimeout,
			deps:             func(t *testing.T) *DoctorDeps { return lockTimeoutStandDownDeps(t) },
			skippedLine:      "Skipped stale hook prune: hooks.json is locked",
			notEvaluableLine: "  · stale hooks: hooks.json is locked (not evaluable)",
		},
	}
}

// healthyStandDownDeps stages an install whose only anomaly is the stand-down
// itself — a live runtime, a fresh state dir and a live project record — so an
// exit code that moves can only have moved because of the stand-down.
func healthyStandDownDeps(t *testing.T, lister *stubStaleSweepReader, store *hooks.Store) *DoctorDeps {
	t.Helper()

	dir := t.TempDir()
	seedHealthyStateDir(t, dir)
	projectStore, _ := seedProjectsJSON(t, t.TempDir())
	return staleDeps(dir, lister, store, projectStore)
}

func restoringStandDownDeps(t *testing.T) *DoctorDeps {
	t.Helper()
	store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hooksBody(hookstest.LiveSeedA)})
	return healthyStandDownDeps(t, restoringHookLister(), store)
}

// The server is down, so the marker read fails outright — the state a user
// reaches for doctor in after a reboot.
func markerReadFailedStandDownDeps(t *testing.T) *DoctorDeps {
	t.Helper()
	store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hooksBody(hookstest.LiveSeedA)})
	lister := staleHookLister()
	lister.restoringErr = errors.New("no server running")
	return healthyStandDownDeps(t, lister, store)
}

func storeReadFailedStandDownDeps(t *testing.T) *DoctorDeps {
	t.Helper()
	store, _ := hookstest.StageStore(t, hookstest.Staging{Unreadable: true})
	return healthyStandDownDeps(t, staleHookLister(), store)
}

func paneReadFailedStandDownDeps(t *testing.T) *DoctorDeps {
	t.Helper()
	store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hooksBody(hookstest.LiveSeedA)})
	return healthyStandDownDeps(t, &stubStaleSweepReader{err: errors.New("tmux transient")}, store)
}

// The guard fires on a completed read of no panes with entries to protect, so
// the fixture seeds an entry alongside the empty enumeration.
func emptyPaneReadStandDownDeps(t *testing.T) *DoctorDeps {
	t.Helper()
	store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hooksBody(hookstest.LiveSeedA)})
	return healthyStandDownDeps(t, &stubStaleSweepReader{rows: tokenRows()}, store)
}

// The seeded entry is live, so the post-repair diagnosis reads the file (a read
// degrades to unlocked) and finds nothing stale: the lock stands down the
// prune and nothing else.
func lockTimeoutStandDownDeps(t *testing.T) *DoctorDeps {
	t.Helper()
	hooks.SetLockTimeoutForTest(t, lockBound)
	store, path := hookstest.StageStore(t, hookstest.Staging{Seed: hooksBody(hookstest.LiveSeedA)})
	hookstest.HoldHooksSidecar(t, path)
	return healthyStandDownDeps(t, &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, store)
}

// renderStaleHooksLine renders the not-evaluable check line exactly as the
// report does, so the assertion reads the user's line rather than a map value.
func renderStaleHooksLine(reason string) string {
	buf := new(bytes.Buffer)
	renderDoctorReport(buf, []checkResult{{
		name:   "stale hooks",
		status: checkNotEvaluable,
		detail: phraseFor(notEvaluableDetails, reason),
	}}, nil)
	for line := range strings.SplitSeq(buf.String(), "\n") {
		if strings.Contains(line, "stale hooks:") {
			return line
		}
	}
	return ""
}

func TestStandDownCopy(t *testing.T) {
	t.Run("it names a failed pane enumeration in the skipped-prune line", func(t *testing.T) {
		out := runDoctorFixWithLister(t, &stubStaleSweepReader{err: errors.New("tmux transient")})
		assertSkippedPruneLine(t, out, "Skipped stale hook prune: could not enumerate live panes")
	})

	t.Run("it names an empty pane list in the skipped-prune line", func(t *testing.T) {
		out := runDoctorFixWithLister(t, &stubStaleSweepReader{rows: tokenRows()})
		assertSkippedPruneLine(t, out, "Skipped stale hook prune: live pane list came back empty")
	})

	t.Run("it names a phrase for every stand-down reason on both surfaces", func(t *testing.T) {
		cases := standDownCopyCases()
		if len(cases) != len(skipReasons) {
			t.Fatalf("stand-down copy cases = %d, want one per declared reason (%d)", len(cases), len(skipReasons))
		}

		skipped := map[string]string{}
		notEvaluable := map[string]string{}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				outBuf, _, _ := runDoctorWith(t, tc.deps(t), "--fix")
				assertSkippedPruneLine(t, outBuf.String(), tc.skippedLine)

				if got := renderStaleHooksLine(tc.reason); got != tc.notEvaluableLine {
					t.Errorf("not-evaluable line = %q, want %q", got, tc.notEvaluableLine)
				}
			})

			if prior, ok := skipped[tc.skippedLine]; ok {
				t.Errorf("reason %q borrows the skipped-prune line already used by %q: %q", tc.reason, prior, tc.skippedLine)
			}
			if prior, ok := notEvaluable[tc.notEvaluableLine]; ok {
				t.Errorf("reason %q borrows the not-evaluable line already used by %q: %q", tc.reason, prior, tc.notEvaluableLine)
			}
			skipped[tc.skippedLine] = tc.reason
			notEvaluable[tc.notEvaluableLine] = tc.reason
		}
	})

	t.Run("it renders doctor's not-evaluable detail for a failed marker read", func(t *testing.T) {
		if got := renderStaleHooksLine(skipReasonMarkerReadFailed); got != "  · stale hooks: could not read the restore marker" {
			t.Errorf("not-evaluable line = %q, want the failed marker read named in the diagnosis", got)
		}
	})

	t.Run("it renders a not-evaluable detail for a lock-timeout stand-down", func(t *testing.T) {
		if got := renderStaleHooksLine(skipReasonLockTimeout); got != "  · stale hooks: hooks.json is locked (not evaluable)" {
			t.Errorf("not-evaluable line = %q, want the lock named in the diagnosis", got)
		}
	})

	t.Run("it renders no raw reason slug on either surface", func(t *testing.T) {
		for _, tc := range standDownCopyCases() {
			if phrase := phraseFor(skippedPrunePhrases, tc.reason); phrase == tc.reason || strings.Contains(phrase, tc.reason) {
				t.Errorf("phraseFor(skippedPrunePhrases, %q) = %q; want a user-facing phrase, not the enum value", tc.reason, phrase)
			}
			if detail := phraseFor(notEvaluableDetails, tc.reason); detail == tc.reason || strings.Contains(detail, tc.reason) {
				t.Errorf("phraseFor(notEvaluableDetails, %q) = %q; want a user-facing phrase, not the enum value", tc.reason, detail)
			}
		}
	})

	// Withdrawn: it was signed off for the empty-live-set guard — a successful
	// read that answered nothing — while reading as a failed read. Neither
	// branch keeps it; each names its own condition.
	t.Run("it renders the withdrawn pane-read phrase on neither surface", func(t *testing.T) {
		const withdrawn = "could not read live panes"
		for _, tc := range standDownCopyCases() {
			if strings.Contains(phraseFor(skippedPrunePhrases, tc.reason), withdrawn) {
				t.Errorf("reason %q still renders the withdrawn phrase %q in the skipped-prune line", tc.reason, withdrawn)
			}
			if strings.Contains(phraseFor(notEvaluableDetails, tc.reason), withdrawn) {
				t.Errorf("reason %q still renders the withdrawn phrase %q in the diagnosis", tc.reason, withdrawn)
			}
		}
	})

	// A stand-down that renders as an empty line is the silence this reporting
	// removes, so an unmapped reason still prints something.
	t.Run("it renders an unmapped reason as itself on both surfaces", func(t *testing.T) {
		const unmapped = "unmapped-reason"
		if got := phraseFor(skippedPrunePhrases, unmapped); got != unmapped {
			t.Errorf("phraseFor(skippedPrunePhrases, %q) = %q, want the raw reason", unmapped, got)
		}
		if got := phraseFor(notEvaluableDetails, unmapped); got != unmapped {
			t.Errorf("phraseFor(notEvaluableDetails, %q) = %q, want the raw reason", unmapped, got)
		}
	})

	// The copy is a rendering concern and must stay one: a stand-down is not a
	// failed check on either surface, and both exit codes stay the post-repair
	// diagnosis's to drive.
	t.Run("it leaves the exit code to the post-repair diagnosis for every stand-down", func(t *testing.T) {
		for _, tc := range standDownCopyCases() {
			t.Run(tc.name, func(t *testing.T) {
				fixBuf, _, fixErr := runDoctorWith(t, tc.deps(t), "--fix")
				if fixErr != nil {
					t.Errorf("doctor --fix err = %v; want nil over a healthy post-repair diagnosis\n%s", fixErr, fixBuf.String())
				}
				if !strings.Contains(fixBuf.String(), tc.skippedLine) {
					t.Fatalf("fixture did not stand the prune down:\n%s", fixBuf.String())
				}

				readBuf, _, readErr := runDoctorWith(t, tc.deps(t))
				if readErr != nil {
					t.Errorf("doctor err = %v; want nil over a stand-down the diagnosis reports as not evaluable\n%s", readErr, readBuf.String())
				}

				// The diagnosis still owns both exit codes: one genuinely
				// failing check under the same stand-down is non-zero again.
				failFix := tc.deps(t)
				failFix.SaverPresent = func() (bool, error) { return false, nil }
				fixFailBuf, _, fixFailErr := runDoctorWith(t, failFix, "--fix")
				if !errors.Is(fixFailErr, ErrDoctorUnhealthy) {
					t.Errorf("doctor --fix err = %v; want ErrDoctorUnhealthy with a failing check\n%s", fixFailErr, fixFailBuf.String())
				}

				failRead := tc.deps(t)
				failRead.SaverPresent = func() (bool, error) { return false, nil }
				readFailBuf, _, readFailErr := runDoctorWith(t, failRead)
				if !errors.Is(readFailErr, ErrDoctorUnhealthy) {
					t.Errorf("doctor err = %v; want ErrDoctorUnhealthy with a failing check\n%s", readFailErr, readFailBuf.String())
				}
			})
		}
	})
}
