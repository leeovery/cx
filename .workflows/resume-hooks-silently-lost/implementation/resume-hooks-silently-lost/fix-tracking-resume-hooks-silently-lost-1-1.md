## Attempt 1

ISSUES:
- `cmd/run_hook_stale_cleanup_test.go:189-193` — the subtest `"nil onRemoved is safe under normal removal"` no longer performs a removal. Its stale seed `b:0.0` is non-token-shaped, so it is now retained, `removed` is empty, and the `if onRemoved != nil` branch in `runHookStaleCleanup` is never entered. The test that exists to prove a nil callback is safe *when entries are removed* now proves only that the zero-removal early return does not panic. Mutation-verified both ways: deleting the nil guard from `cmd/run_hook_stale_cleanup.go:55-59` leaves this subtest green against the current tree, and panics against `HEAD`. This is the same vacuity the task singled out for `TestCleanStaleRemovesExactlyStaleKeys`, in a file the task listed as a site, and it is the only fixture in the sweep that regressed. (The nil-callback-with-removal path stays covered indirectly by `TestMaybeRunHookCleanup_RunsAndResetsOnceIntervalElapsed`, which is why the lane stayed green — the loss is silent, not fatal.)
  FIX: re-point that subtest's stale seed to a token-shaped literal, matching the convention already used in the sibling subtests at `:157-161` and `:209-213` — change `"b:0.0"` on line 192 to `"keyB00"` (6 chars from `[A-Za-z0-9]`, absent from the `panes: []string{"a:0.0"}` live set). No assertion changes are needed; the subtest asserts only the completion DEBUG count.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/hooks/cleanstale_staleness_guard_test.go:12-15` — asserts an exclusive lock hold that does not exist: `internal/hooks` contains no locking of any kind at this point (the sidecar lock is later work), so `CleanStale` loads under no hold and reaching `StaleKeys` would take nothing a second time and stand nothing down. The invariant the guard enforces is real; only the stated mechanism is not yet true.
  OLD:
```
// CleanStale judges staleness on the key set it loaded under its own exclusive
// hold. Reaching the rule through the exported StaleKeys would take that hold a
// second time and stand the prune down on every cycle, so the two readers share
// the unexported implementation rather than one calling the other.
```
  NEW:
```
// The staleness rule has one implementation and both readers reach it directly.
// CleanStale must not be layered on the exported StaleKeys: it judges the key
// set it has already loaded, and the exported query is free to grow
// caller-facing behaviour that must not run inside CleanStale's own pass.
```
- `internal/hooks/store_shape_test.go:12-13` — `Store.Set` places no constraint on the key it is handed; `Set("", …)` writes an empty key just as readily as any other, so it is not what the raw seed route works around.
  OLD:
```
// seedHooksFile writes the raw on-disk map so a fixture can hold keys the Set
// API would never mint, such as the empty key.
```
  NEW:
```
// seedHooksFile writes the raw on-disk map so a fixture can hold an arbitrary
// key, including the empty one.
```

NOTES:
- `internal/hooks` now transitively depends on `internal/state`, `internal/tmux`, `internal/project`, `internal/resolver` and `internal/xdg` (confirmed with `go list -deps`) to reach a 15-line pure predicate. The specification directs this home explicitly and there is no cycle today, so it is not a finding — but it does mean any future need for `internal/state` or `internal/tmux` to read `hooks` would now cycle, which is worth knowing before the concurrency and pane-token phases land.
- `internal/hooks/store_test.go:741` — the subtest name `"returns every persisted key when the live set is empty"` now slightly overclaims: `StaleKeys` returns every *judgeable* persisted key. The seed change to token-shaped keys was exactly what the task directed, and the assertion is correct; only the name reads broader than the behaviour.
- The subtest rename from `"old pane-ID entries cleaned on first run after upgrade"` to `"cleans stale entries seeded straight into the file"` (`internal/hooks/store_test.go:610`) was the right call — the `%0` / `%3` shapes it asserted are retained under the new rule, so the old name would have been a lie. No test now pins that a `%N` key is retained, but that is a consequence of the rule rather than a gap this task created.
- `cmd/run_hook_stale_cleanup.go:9-11` — the `AllPaneLister` doc comment still describes the `<@portal-id or session_name>:window.pane` form. Correct for this phase (the enumeration is still positional) and scheduled for rewrite when the enumeration changes; flagged only so it is not mistaken for an oversight.

## Attempt 2

ISSUES:
- `cmd/run_hook_stale_cleanup_test.go:34-37` — the empty-live-set hazard-guard fixture was not re-pointed. Its seed is still `a:0.0` / `b:0.0`, so the subtest's strongest assertion (`hooks.json modified by hazard-guard branch`, `:49-51`) is now satisfied by the new shape rule rather than by the guard. Confirmed by overlay-mutating `runHookStaleCleanup` to delete the `len(livePanes) == 0` block: the subtest still fails, but only on the two log-count assertions — the file-equality assertion does not fire. This is precisely the case the task's re-pointing rule names ("a fixture asserting the empty-live-set guard's retention should seed token-shaped keys so the guard, not the new shape rule, is what it measures"), and it is the site the task's `cmd/run_hook_stale_cleanup_test.go` (`a:0.0` / `b:0.0`) literal pair points at. The two sibling fixtures for the same guard — `TestMaybeRunHookCleanup_ReusesMassDeletionGuard` (`cmd/state_daemon_hook_cleanup_test.go:196-199`) and `TestDoctorFixProtectsUserHooksWhenLiveSetEmptyOrErrored` (`cmd/doctor_test.go:1304`) — were both re-pointed, so this is also an internal inconsistency. The same confound applies to `:98`, where the `ListAllPanes`-error subtest's `hooks.json modified on ListAllPanes-error path` assertion is likewise now double-caused by the seed `{"a:0.0": …}`.
  FIX: seed token-shaped keys at both sites, changing nothing else. At `:34-37` use `keyA00` / `keyB00` (the literals the sibling guard fixture in `cmd/state_daemon_hook_cleanup_test.go` already uses, so the two guard tests read the same); at `:98` use `keyA00`. Both subtests stay green — the hazard branch and the lister-error branch each return before `CleanStale`, so no removal occurs either way — and each then measures its own branch rather than the shape rule. No live-set coupling exists at either site (`:42` passes `panes: []string{}`, `:104` passes `panes: nil` with an error), so arm (b) of the re-pointing rule does not apply.
  CONFIDENCE: high

NOTES:
- The two judgement calls the executor took beyond the task's site list are both correct (the `%0`/`%3` subtest re-point + rename, and the `gone-session:0.0` seeds in `cmd/hookkey_no_regression_upgrade_test.go` and `cmd/rename_restore_cleanup_survival_integration_test.go`).
- The source guard cannot be mutation-verified through `-overlay` (it reads the real files from disk). Its detection logic was checked by hand: `ForEachFuncCall` reports the enclosing `FuncDecl` name, and `calleeName` covers both the bare `StaleKeys(...)` identifier and a `hooks.StaleKeys(...)` selector.
- `cmd/run_hook_stale_cleanup.go:9-11`'s `AllPaneLister` doc comment and CLAUDE.md's `hooks` row still describe the old key scheme; both are explicitly scheduled for the later phases that change the enumeration.
