## Attempt 1

ISSUES:
- `cmd/doctor_test.go:926-932` and `:1598-1608` still open-code `assertHooksFileUnchanged` inline, in the
  same file where three sibling sites were converted. `:1606-1607` carries the failure wording
  *byte-identical* to the converted site at `:1252` ("hooks.json mutated by diagnosis (read-only
  violated)"), and `:1577-1580` / `:1598-1601` are the raw `os.ReadFile` + `t.Fatalf` pair on both ends
  that item (a) names by shape. The executor extended into this file on the stated basis that the
  acceptance criterion is package-wide; under that reading the criterion is unmet, and a reader now
  meets both forms of the same assertion 350 lines apart in one file.
  FIX: Convert the two remaining sites the same way as the three already done. At `:1442-1445` replace
  the read pair with `hooksBefore := readFileBytes(t, hooksPath)`, and at `:926-932` replace the re-read
  and comparison with `assertHooksFileUnchanged(t, hooksPath, hooksBefore, "pruned on a down server
  (user commands must survive)")`. In `TestDoctorStaleChecksAreReadOnly`, replace `:1577-1580` with
  `hooksBefore := readFileBytes(t, hooksPath)`, delete the `hooksAfter` read at `:1598-1601`, and
  replace `:1606-1608` with `assertHooksFileUnchanged(t, hooksPath, hooksBefore, "mutated by diagnosis
  (read-only violated)")`. Leave the `projects.json` halves alone — there is no projects-side helper and
  that is a separate consolidation. Keep the `bytes` import (still used at `:1609`) and re-check `os`
  usage in the file after the edit.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/state_hydrate_test.go:918-920` — the universal "anything left unset takes the suite default" is
  false for `Logger` and `HookStore`, and a nil `HookStore` is not an unspecified default but the
  load-bearing bare-shell scenario several cases depend on.
  OLD: // hydrateCfgOpts names the parts a hydrate case varies. Anything left unset
       // takes the suite default: a discarded stdout, a fresh recording commander, and
       // a stub exec whose recordings nobody reads.
  NEW: // hydrateCfgOpts names the parts a hydrate case varies. Stdout, Commander and
       // ExecShell default when unset — a discarded stdout, a fresh recording
       // commander, and a stub exec whose recordings nobody reads. Logger and HookStore
       // do not: a nil Logger falls through to the package logger, and a nil HookStore
       // is the bare-shell scenario rather than an omission.

NOTES:
- `hookstest.AssertSidecarFree` uses `t.Errorf` where the `internal/hooks/lock_test.go` original used
  `t.Fatalf`. The choice is right (two of the four call sites run inside a callback the store invokes),
  but `TestMutationLockRelease` does not lower the lock bound, so a genuine regression there now costs
  the full 2s production bound on the follow-on mutation before failing, instead of stopping at the
  probe. Behaviour is still correct; only time-to-failure changes.
- `AssertSidecarFree` inherits `O_CREATE` from `openSidecar`, so it creates the sidecar when absent.
  Both new call sites stage it first (`cmd/hook_sweep_snapshot_order_test.go` previously opened without
  `O_CREATE`), so nothing is weakened today — but the doc comment does not name the side effect, and
  `TestHookSweepTakesNoLockWithNothingPersisted` (`cmd/hook_sweep_snapshot_order_test.go:116`) exists
  precisely to assert the sidecar is *not* created. A future caller reaching for this helper to prove
  absence would silently defeat itself.
- `cmd/state_hydrate_test.go:1343,1394,1427` still assign `cfg.HookStore = store` after the builder call
  even though `hydrateCfgOpts` now carries a `HookStore` field (which `state_hydrate_empty_hookkey_test.go`
  uses). Those lines are untouched by this diff, so it is not a regression, but the two styles now sit
  side by side in one file.
- `assertReturnsAtLockBound`'s `run func() (string, error)` discards the string; `func() error` would
  state what the helper actually consumes. Both call sites already wrap in a closure, so the signature
  costs nothing to narrow.
