## Attempt 1

ISSUES:
- cmd/hook_sweep_lock_timeout_test.go:107-113 — the subtest "it survives a nil onSkipped on the lock branch" pins a parameter that no longer exists, and its body is now a strict subset of "it deletes nothing when the sweep cannot take the lock" (line 47): same fixture, same sweepErr(lister, store, nil) call, same nil-error assertion, minus the file-unchanged check. It measures nothing the sibling does not. Its failure message at line 111 repeats the dead name.
  FIX: Delete lines 107-113 outright. The nil-callback contract it guarded no longer exists, and the lock-branch nil-error path is already covered at line 47.
  ALTERNATIVE: Keep it and rename to something the code still has — but there is no distinct behaviour left for it to pin, so this leaves a duplicate. Deletion is the recommendation.
  CONFIDENCE: high

- cmd/hook_sweep_snapshot_order_test.go:72 — the subtest name "it feeds onRemoved exactly what was deleted" names the removed callback. The body reads outcome.Removed and its own failure message already says "the outcome names this sweep's deletions", so the name is the only thing left pointing at the old API.
  FIX: Rename to "it reports exactly what was deleted", matching the body and its failure message.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- cmd/hook_sweep_lock_timeout_test.go:163-164 — names a parameter the signature no longer has.
  OLD: 	// The daemon's call shape: a nil onSkipped, its own logger, and no second
	// WARN of its own over the one the sweep already emitted.
  NEW: 	// The daemon's call shape: its own logger, and no second WARN of its own
	// over the one the sweep already emitted.

NOTES:
- pruneDoctorStaleHooks (cmd/doctor.go:201-210) still prints nothing to stdout on a non-lock CleanStale failure: the outcome carries no reason, so only the WARN at line 203 records it. That is a sixth "the repair did not run" path outside the five the task enumerated, it mirrors pruneDoctorStaleProjects exactly, and inventing a sixth phrase would exceed the specification's fixed three — not blocking.
- notEvaluableDetails[skipReasonStoreReadFailed] (cmd/doctor.go:310) is unreachable: checkStaleHooks loads the store itself and returns before the ladder can produce that reason. Harmless defensive completeness.
- stubAllPaneLister (cmd/hookkey_vocabulary_test.go:111) still carries the removed interface's name. The task's Tests section names it explicitly, so leaving it is the right call.
- standDown.emit() (cmd/run_hook_stale_cleanup.go:54) is the codebase's first production slog.Logger.Log(context.Background(), level, …) call. Idiomatic slog, warranted by the level being data rather than a call site, but a new shape in this repo.
- staleSweepReader is read by checkStaleHooks, which is a diagnostic rather than a sweep. The doc comment covers both, so the name is a slight under-fit rather than a misnomer.
- Spec note: the specification maps `could not read live panes` to the empty-live-set guard; the task's Do step 2 explicitly re-wires it to the failed read and gives the empty read its own phrase. The executor followed the task, so §5.1 of the spec now describes a mapping the code no longer holds.
