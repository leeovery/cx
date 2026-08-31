## Attempt 1

ISSUES:
- `cmd/run_hook_stale_cleanup_single_report_test.go:94-96` hand-rolls the lock fixture
  (`hooks.SetLockTimeoutForTest` + `newStagedHooksStore{seed: staleHookSeed}` +
  `hookstest.HoldHooksSidecar`) that already exists as `lockedSweepFixture` in the same package
  (`cmd/run_hook_stale_cleanup_lock_timeout_test.go:21-26`). A second spelling of the same staging can
  drift from the first silently — one case would then measure a different condition under the same
  name. This is the same smell the task otherwise cleaned up by collapsing the bogus-store copies onto
  `bogusHooksStore`.
  FIX: replace lines 94-96 with `store, _, _ := lockedSweepFixture(t, lockBound)` and drop the
  now-unused `hooks`/`hookstest` imports if nothing else in the file needs them (the `hookstest` import
  exists only for this call).
  ALTERNATIVE: drop the subtest entirely — `TestHookSweepStandsDownOnLockTimeout/"it stands the
  daemon's throttled sweep down without a second WARN"`
  (`cmd/run_hook_stale_cleanup_lock_timeout_test.go:140-157`) already drives the identical fixture
  through `maybeRunHookCleanup` and asserts the same stand-down plus the absence of the daemon's second
  WARN, and additionally asserts the file is untouched — so the new case is a strict subset apart from
  a marginally broader WARN-count check, and costs a full lock bound (~0.09s) of unit-lane wall clock
  to re-prove it. The fixture swap is recommended over the deletion, because the task's own test list
  names this case and keeping it named is worth one line.
  CONFIDENCE: high

NOTES:
- Your rename-over-drop decision holds and was verified: every `hooks`-component emission carries an
  `op` attr (`internal/hooks/store.go:70,119,131,141,146,175,199,204,356`; `standDownAttrs` likewise),
  so binding two `op`-less DEBUG counts there would extend a spec-governed closed vocabulary. Dropping
  the parameter also has a behavioural cost the task's framing understates: the counts are currently
  attributed to the calling process's component (`daemon` from the tick, `bootstrap` from
  `doctor --fix`), and a drop collapses that to one.
- Both directions were mutation-tested with `go test -overlay`: restoring the `, err` return produces
  exactly the two-WARN output the task describes (3 subtests fail); making the default branch swallow
  genuine failures fails 3 more. The deduplication does not silence, and genuine failures still return.
- `cmd/run_hook_stale_cleanup_single_report_test.go:31` asserts the absence of the daemon's WARN by
  message, while the lock-timeout subtest at `:106` asserts the absence of *any* WARN on the injected
  sink. The broader form is the stronger guard; the narrower one passes if the daemon ever adds a
  differently-worded WARN on that path. Mutation testing confirms the current assertion does catch the
  regression it exists for, so this is preference, not a gap.
- The two decline branches in `declinedSweep` (`cmd/run_hook_stale_cleanup.go:225-240`) are now
  byte-identical apart from the reason constant. Deliberately not raised as an issue: at two instances
  a shared helper is the premature abstraction `code-quality.md` warns against, and each branch's
  comment carries different rationale that a table would flatten.

## Attempt 2

ISSUES:
- `cmd/run_hook_stale_cleanup_test.go:365-382` — `TestHookSweepStandsDownWhileRestoring/"it skips before
  loading the store"` no longer measures its subject. Its oracle was "the store fails loudly on any
  read, so a nil return proves it was never loaded" — true only while `ErrSnapshotRead` returned
  non-nil. With the branch now returning nil, invert the order in `runHookStaleCleanup` (load first,
  restore-check second) and the subtest still passes: `CleanStale` fails at `loadSnapshot` before
  calling the enumeration closure, so `err == nil` (via `declinedSweep`), `lister.calls == 0` (the
  closure never ran) and `len(outcome.Removed) == 0` all hold. No other case covers the ordering — the
  sibling `"it deletes nothing while the restore marker is set"` (`:324`) uses a readable store, where
  an unchanged file does not prove an unread one. You rewrote this comment in this task rather than
  noticing the claim had just been falsified, so both the guard and the comment are in scope.
  FIX: assert the reason, which is what now discriminates — add
  `if outcome.DeclineReason != skipReasonRestoring { t.Errorf("DeclineReason = %q, want %q (a store read would have declined with %q)", outcome.DeclineReason, skipReasonRestoring, skipReasonStoreReadFailed) }`
  after the existing `err` check, and replace the comment so it names the live oracle.
    OLD:
		// The store must fail loudly on any read, so a nil return proves it was
		// never loaded.
    NEW:
		// The store fails loudly on any read, so a decline naming the restore
		// marker — rather than the store read — proves it was never loaded.
  ALTERNATIVE: install the process sink and use
  `assertStandDown(t, sink, slog.LevelDebug, skipReasonRestoring)`, which additionally proves no WARN
  was emitted (a reached store read would have emitted one). Slightly stronger, one more line of setup,
  and it duplicates the level assertion already made by the sibling `"it logs the stand-down at DEBUG
  and never WARN"` subtest. The `DeclineReason` assertion is recommended — it is the minimal
  restoration of the oracle and keeps each subtest to one subject.
  CONFIDENCE: high

NOTES:
- `cmd/doctor.go:202-204`'s generic `doctor --fix: stale-hook prune failed` WARN is now reachable only
  for an unnamed store failure (a save/lock-open error) and has no test on the doctor route; its daemon
  counterpart does, via the repointed `TestMaybeRunHookCleanup_LogsWarnAndSwallowsCleanupError`.
  Pre-existing, and the branch is correct as it stands — worth knowing only because the store-read case
  that used to exercise it no longer does.
- The `ErrLockHeld` and `ErrSnapshotRead` arms of `declinedSweep` are now structurally identical
  (declineWarn → emit → nil, differing only in reason) and their comments both close on the same
  sentence about the nil return. Two instances is under the Rule of Three and each arm's first sentence
  carries a genuinely different rationale, so leaving them as two explicit cases reads better than a
  table. No change wanted; flagged only so a third sentinel arriving later is recognised as the trigger
  to collapse them.
- Everything else was verified and holds: the doctor assertion is stronger than reported (it requires
  exactly one record at-or-above WARN in the process sink, so the generic line reappearing fails the
  case), the exit code is unaffected on both directions, the countsLogger doc comment is accurate, and
  the four untouched decline branches are byte-unchanged in the diff.
