## Attempt 1

ISSUES:
- `cmd/run_hook_stale_cleanup.go:40-51` — the empty-token filter in `liveTokensFrom` has no test. Removing it keeps `go test ./...` green. If it regressed, an empty `""` key in `hooks.json` would become permanently un-reapable whenever any pane is unstamped (the ordinary state under lazy stamping), and that entry fires its command on every unstamped restored pane.
  FIX: add a subtest to `TestHookSweepGuardCountsPaneRowsNotTokens` (`cmd/run_hook_stale_cleanup_test.go:777`) seeding `{"": {"on-resume": "cmd-empty"}, unjudgeableSeedA: {"on-resume": "cmd-old"}}` against `unstampedRows(2)`, asserting the `""` entry is removed and the unjudgeable one survives. With the filter dropped, `""` joins the live set and the entry is retained, so the subtest fails.
  CONFIDENCE: high

- `cmd/doctor_test.go:1063,1184-1187` and `cmd/run_hook_stale_cleanup_test.go:200-208,252-260,321-327` — the live-set literals were kept in old-format shape (`sessA:0.0`, `a:0.0`, `tok123:0.0`) and re-pointed only by being wrapped in `tokenRows(...)`, contrary to §9.3. Because those persisted keys are unjudgeable by shape, every "the live entry survives" assertion on these paths now passes on Phase 1's retention rule rather than on liveness — the exact trap the task's edge-case list names — and the doctor's staleness comparison ends up with no coverage at all. `TestRunHookStaleCleanup/it preserves a stamped-session hook whose id-key matches the live set` and `TestDoctorStaleHooksCheck/all persisted keys live passes` both survive `liveTokensFrom` returning an empty slice.
  FIX: re-point both sides of these fixtures at `transienttest.ReapableHookKey(n)` values — the persisted seed key and the matching live row — so a preserved entry is token-shaped and therefore preserved only because its token is in the live set. `TestDoctorStaleHooksParityWithPredicate`'s `tc.live` needs the same treatment, or its `want` stays a tautology.
  ALTERNATIVE: leave the bulk mechanical and add one targeted doctor subtest asserting a token-shaped persisted key whose token is live reports `checkPass`. Smaller diff and it closes the mutation hole, but it leaves the remaining fixtures passing for the wrong reason and leaves §9.3 unmet. The reviewer recommends the full re-point.
  CONFIDENCE: high

- `cmd/doctor.go:317` — the doctor's row-versus-token split is untested. Changing the zero-row branch to count tokens leaves the entire lane green, yet it would make `checkStaleHooks` return "zero live panes with hooks present (not evaluable)" on every install with hooks and no stamped pane, which is every install until 2-2 ships.
  FIX: add a subtest to `TestDoctorStaleHooksCheck` using `fakeHookLister{rows: unstampedRows(3)}` with a token-shaped key seeded via `seedHooksJSON(t, reapableSeedA)`, asserting `checkFail` with the one-entry detail — i.e. the check evaluates rather than standing down. Pair it with an `unstampedRows` + no-hooks case asserting it does not report "no hooks" through the zero-row door if you want both halves pinned.
  CONFIDENCE: high

- `cmd/run_hook_stale_cleanup_test.go:857-869` — `it counts the rows, not the tokens, on the counts line` asserts only that the DEBUG line fired once. The counts line is emitted before the guard, so it fires either way, and `recordedLog` (`cmd/bootstrap_production_test.go:57-82`) discards attrs, so the value is unreachable. Mutating `"panes", len(rows)` to `"panes", len(liveTokensFrom(rows))` leaves it green. A test whose name states the inverse of what it checks is a worse signal than no test.
  FIX: extend `recordedLog` with the record's attrs (the handler already walks them in `Handle`) and assert `panes == 4` for the `unstampedRows(4)` case.
  ALTERNATIVE: delete the subtest and rely on the guard subtest, which is mutation-effective. Cheaper, but then the `panes=` row-count instruction has no test at all and the DEBUG line can silently drift to a token count. The reviewer recommends extending the recorder.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- cmd/doctor.go:297-299 — the rationale now describes a guard the code deliberately no longer has: with rows present and every token empty, the live token set *is* empty and the check proceeds to count on purpose. The guard is on live panes, not on the live set.
  OLD:
// The guards below must precede the stale count: an unreadable or empty live
// set would otherwise report every judgeable entry stale and mislead a --fix
// into mass-deleting user-authored on-resume commands.
  NEW:
// The guards below must precede the stale count: an unreadable pane list, or a
// server with no panes at all, would otherwise report every judgeable entry
// stale and mislead a --fix into mass-deleting user-authored on-resume
// commands. A live pane carrying no token is not that case — under lazy
// stamping it is the ordinary one, and the count proceeds.

NOTES:
- On the `internal/restore` flake: the reviewer shares the executor's read. `internal/restore` references none of `ListAllPaneHookKeys`, `PaneHookRow` or `PortalPaneIDOption`, and `internal/tmux`'s import of `internal/state` already existed via `portal_saver.go`. `TestMultiPaneLegacy_GracefulLegacyDegradation` passed `-count=3` and the full package passed. Separately `cmd/bootstrap` failed once in a combined four-package run and passed alone and on an uncached re-run — the lane carries at least two timing-sensitive fixtures in packages this diff does not touch.
- `internal/tmux/tmux.go:582` names `@portal-pane-id` in prose inside `PaneHookRow`'s doc comment — a documentation reference, not a second code literal, so the "exactly one place" criterion holds.
- `parsePaneHookRows` returns `[]PaneHookRow{}` rather than a nil slice, which is the load-bearing half of the contract; a future "simplification" to `var rows []PaneHookRow` would silently break it. The unit test catches it.
