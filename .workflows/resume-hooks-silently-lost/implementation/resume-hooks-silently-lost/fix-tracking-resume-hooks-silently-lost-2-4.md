## Attempt 1

ISSUES:
- `cmd/hooks_test.go:860-885` — `assertTouchWarn` never asserts `hook_key`, so the acceptance criterion "the WARN … carries `op`, `hook_key`, `via=cli` and `error`" is unverified. Mutation-proven twice: replacing the value with `"WRONG-KEY-NOT-THE-TOKEN"` keeps the suite green, and deleting the attr from the emission entirely keeps it green. The same helper pins the level only as `>= slog.LevelWarn` (via `warnRecords`, `cmd/hooks_test.go:842`), so switching the emission to `hooksLogger.Error` also passes — the criterion says WARN. Separately, the helper's returned `logtest.Record` is used by neither call site (lines 926, 955), and its doc comment claims a purpose ("returning it for per-case assertions") that nothing fulfils.
  FIX: inside `assertTouchWarn`, add `if rec.Level != slog.LevelWarn { t.Errorf(...) }` and an exact `hook_key` assertion. Both call sites register under `"tok123"`, so give the helper a `wantKey string` parameter and assert `rec.AttrString(t, "hook_key") == wantKey`. Drop the unused return value and the "returning it for per-case assertions" clause from the comment in the same edit — `standDownRecord` (`cmd/run_hook_stale_cleanup_test.go:598`) is the in-package precedent for an exact-level assertion.
  CONFIDENCE: high

- `cmd/hooks_test.go:824` — the new `runHooksSet(t, key, command)` sits one character away from the existing `runHookSet(t, command)` (`cmd/hooks_pane_token_test.go:44`) in the same package, and re-implements that helper's four-line Execute block verbatim. Two same-package drivers distinguishable only by a plural is a call-the-wrong-one trap, and the duplicated block will drift.
  FIX: rename the new helper to name what it adds (e.g. `runHookSetForKey`) and delegate the execute half: `t.Setenv("TMUX_PANE", "%3")`, wire `hooksDeps` + cleanup, then `return runHookSet(t, command)`. This touches only this task's code — `hooks_pane_token_test.go` stays as it is.
  ALTERNATIVE: add the key parameter to the existing `runHookSet` and update its four call sites. Rejected as the recommendation: those call sites wire their own `PaneStamper` deps, so the parameter would only be half the setup and the helper would grow an argument most callers pass empty.
  CONFIDENCE: high

NOTES:
- The "touch itself fails" fixture is genuine, not a collapsed duplicate: instrumenting the two branches shows `it exits 0 when the touch itself fails` fails at `touch save.requested: ... permission denied` while `it exits 0 when the state directory cannot be resolved` fails at `failed to create state directory ... not a directory`. If a run were ever rooted, that subtest fails loudly (zero WARNs) rather than false-passing.
- All acceptance criteria hold as behaviour, mutation-verified: double-emission fails 3 subtests; returning the touch error fails 4 new plus 9 pre-existing tests; touching before the write-error check fails one; changing `op` to `set` fails 3; removing the touch fails 5.
- `hook rm` is structurally safe: `TouchSaveRequested` has exactly one call site, inside `hooksSetCmd`'s path.
- Bootstrap exemption intact — `hook` is in `skipTmuxCheck`, both state calls are pure filesystem, and `cmd/root_test.go:287` already drives `hook set` through `PersistentPreRunE` asserting zero orchestrator runs.
- `it emits no set WARN when only the touch fails` and `it emits exactly one warn per failing hook set` both point `PORTAL_STATE_DIR` under a regular file, so they exercise the `EnsureDir` branch — the touch never runs, despite the first name saying "only the touch fails". Coverage is not short (the touch-branch subtest asserts an exact WARN count of 1, proving both properties there), but the first subtest is worth renaming if touched anyway.
- `hooksFileInTempDir` would replace the three-line hooks-file preamble in three of the eight new subtests; the other five legitimately need the `dir` variable.
- CLAUDE.md line 45 reads correctly against the code and drops nothing else from the bullet.
