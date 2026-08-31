## Attempt 1

ISSUES:
- `internal/tmux/errors.go:29-37` declares a deliberate exclusion — `"can't find window"` must not be
  absorbed, because a missing window inside a live session is not a session absence — and nothing
  executable pins it. The subtest meant to cover the non-absence half,
  `internal/tmux/saver_pane_pid_test.go:135-146`, passes a plain `error` rather than a
  `*tmux.CommandError`, so it short-circuits at `errors.As` in `wrapNoSuchSession` (`errors.go:46`)
  and never reaches `reportsNoSuchSession` at all. Broaden the predicate to `"can't find "` — the exact
  loosening `cmd/state_daemon.go:326` already applies on its own path — and the whole suite stays
  green, including `internal/tmux/errors_test.go`'s negatives (empty stderr, mixed case,
  `connection refused`), none of which sit near the excluded phrasing. The consumer that would
  silently change behaviour is `internal/state/capture.go:71`, where `ErrNoSuchSession` means
  "natural churn, skip" and anything else means "anomalous", which gates the commit.
  FIX: add one subtest to `TestSaverPanePIDOrAbsent` in `internal/tmux/saver_pane_pid_test.go`, in the
  existing `it …` style (e.g. `"it does not read a missing window inside a live session as session
  absence"`), driving `&tmux.CommandError{Stderr: "can't find window: _portal-saver", Err:
  errors.New("exit status 1")}` through `tmux.SaverPanePIDOrAbsent` and asserting
  `errors.Is(err, tmux.ErrNoSuchSession)` is false and the return is `(0, false, non-nil)`. That makes
  the exclusion the widening rests on fail loudly if it is ever dropped, and gives criterion 5 a case
  that actually exercises the new predicate.
  ALTERNATIVE: pin it white-box with an internal test on `reportsNoSuchSession` over a phrasing table.
  Cheaper and covers all three routes at once, but couples the guard to an unexported helper instead
  of the observable tri-state; prefer the black-box subtest, which doubles as real coverage for the
  non-absence criterion.
  CONFIDENCE: high

NOTES:
- Composition with task 7-1 verified: `wrapSessionTargetErr` (`internal/tmux/errors.go:86-94`) runs
  `ValidateSessionName` before delegating, so the widened predicate cannot reclassify an unaddressable
  name as churn. `saverPanePID` wraps directly rather than through `wrapSessionTargetErr`, which is
  safe: every production caller passes the `PortalSaverName` constant, and under the coord form a
  colon-bearing name would be answered `can't find window`, which the predicate deliberately excludes.
- The report's claim that `ActivePaneCurrentPath`'s `ErrNoSuchSession` doc is "now reachable too" does
  not hold. Measured: `display-message -p -t '=gone:' -F '#{pane_current_path}'` returns empty output
  at exit 0, so `wrapNoSuchSession` still never fires there and callers rely on the empty-string guard
  (`internal/session/dirresolve.go:41`). The overstated doc at `internal/tmux/tmux.go:246-248`
  predates this diff and is untouched by it, so it is outside this task's scope — but the report's
  side-effect claim should not be carried forward as fact.
- `SaverPaneID`'s half of the fix is pinned only at argv level
  (`internal/tmux/portal_saver_lifecycle_events_test.go:37`), with no real-tmux prefix-sibling case.
  Acceptable — it is the same `list-panes` command through the same helper as the covered route, and
  its two call sites are observability-only.

## Attempt 2

ISSUES:
- `internal/tmux/saver_pane_pid_realtmux_test.go:38` — the readiness guard is structurally inert for the
  session the test cares about. `tmuxtest.WaitForSession` polls `has-session -t <name>` **without** the
  `=` exact prefix, so it prefix-matches. Measured on an isolated socket: with only `_portal-saver-old`
  live, `has-session -t _portal-saver` exits **0**. In the `"it returns the pane pid of a live
  _portal-saver"` subtest (`:86`) the sibling is seeded first, so by the time `seedSaverServer` waits
  for `PortalSaverName` the sibling already answers and the wait returns immediately — the settle
  window `WaitForSession`'s own doc says the project has observed goes unguarded. If it ever bites,
  `livePanePID` (`:47`, `ts.Run`) fatals with a raw `tmux [list-panes ...]` failure rather than a
  legible readiness error. This file's entire premise is "a live prefix sibling", which is the one
  condition that defeats this helper.
  FIX: make the wait exact inside `seedSaverServer`. Replace `ts.WaitForSession(t, name, 2*time.Second)`
  at `:38` with a poll on the exact form, using the helper already in the package:
  ```go
  if !tmuxtest.PollUntil(t, 2*time.Second, 20*time.Millisecond, func() bool {
      _, err := ts.TryRun("has-session", "-t", "="+name)
      return err == nil
  }) {
      t.Fatalf("session %q did not appear within 2s", name)
  }
  ```
  This is local to the file the task added and needs no change to shared test scaffolding.
  ALTERNATIVE: change `tmuxtest.WaitForSession` itself to poll `"="+name`. Strictly better in principle
  — the fuzzy wait is a latent hazard for every fixture that stages siblings — but it reaches into
  shared scaffolding consumed across the repo and would land inside another task's output, so it
  belongs at the phase boundary rather than here. Recommend the local fix now.
  CONFIDENCE: high

NOTES:
- The account of the first review's internal contradiction is correct on the substance. Broadening the
  predicate to `"can't find "` and asserting that `can't find window` must not classify as
  `ErrNoSuchSession` cannot both hold. Following the FIX rather than the body was the right
  resolution, and measurement supports the narrower set: `can't find window` is what tmux emits for a
  missing *pane* index too (`=live:0.9` → `can't find window: 0`), so it is not a session-absence
  signal.
- The `cmd/state_daemon.go:326` precedent the first review cited does classify a vanished *pane* on
  `"can't find "`, where a missing window is a legitimate positive. Different question, and not
  transplanting it was right.
- Minor style, not worth a change: `reportsNoSuchSession` (`errors.go:52-56`) wraps `strings.Contains`
  in a `slices.ContainsFunc` closure that inverts the argument order; a plain two-line range loop reads
  more directly. `golangci-lint` (including `modernize`) is silent on it either way.
