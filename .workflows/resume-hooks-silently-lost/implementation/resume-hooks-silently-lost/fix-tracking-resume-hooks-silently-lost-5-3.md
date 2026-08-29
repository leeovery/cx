## Attempt 1

All twelve acceptance criteria behaviourally met; the production code is correct — **do not change it**. Two test issues and two comment corrections.

ISSUES:
- `internal/hooks/lock_write_test.go:131` — **no test discriminates `op=set` from `classifySet`'s verdict**, so the decision this task exists to make is unverified. Proven: replacing the WARN branch in `Store.Set` with a version that loads the file, calls `classifySet`, and files the WARN under that verdict leaves the **entire unit lane passing** (`go test ./...` clean), as do both `cmd` lock suites. The cause is fixture shape: every timeout fixture either has no `hooks.json` at all or holds a *different* key (`tok999` while setting `tok123`), so `classifySet` returns `"set"` anyway and the wrong implementation is indistinguishable. This also means acceptance criterion 1's "without loading, classifying" has no observable pin — the `op` value is the only surface on which loading-then-classifying is detectable, and it is not exercised.
  FIX (**take this one — the reviewer's recommended ALTERNATIVE**): add a second, separately named subtest for the case, e.g. `"it files under op=set even where a completed call would have classified as modify"` — seed the same key *and* event with a different command (`{"tok123":{"on-resume":"cmd-a"}}`) before `holdSidecar`, keep the `Set("tok123", "on-resume", "npm start", "cli")` call and the `assertLockWarn(t, sink, "set", "tok123", "cli")` assertion. This keeps both fixture shapes under the WARN assertion and names the decision in the test list where a future reader will look for it — the task text calls the decision out twice, so it deserves its own case.
  (The narrower alternative — just re-seeding the existing subtest's fixture — also works but loses the fresh-file shape and does not name the decision.)
  VERIFY by re-running the mutation above: it must now FAIL.
  CONFIDENCE: high

- `cmd/hooks_write_lock_test.go:64` — `assertOneLockWarn` hand-rolls the level/component/op/via checks that `assertHooksRecord` (`cmd/logging_capture_test.go:55`, documented as "the parts every hooks-component record shares") already owns, and both existing hooks-WARN assertions in the package go through it (`assertTouchWarn` at `cmd/hooks_test.go:988`, `standDownWant` at `cmd/run_hook_stale_cleanup_test.go:562`). The hand-rolled version also silently drops the `msg` assertion the shared helper enforces, so the cmd surface would not catch a WARN emitted with the right `op` and the wrong message — a contract `internal/hooks/store.go:19` states explicitly ("The op verb is both the slog message and a required `op` attr").
  FIX: delegate the shared half — `assertHooksRecord(t, rec, hooksRecordWant{level: slog.LevelWarn, msg: wantOp, op: wantOp, via: "cli"})` — and keep only the emission-specific checks (`hook_key`, and the `error_class`/`value` absence) local, which is exactly the split `assertTouchWarn` uses.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- `cmd/hooks_write_lock_test.go:17-19` — the stated rationale describes a hazard that cannot occur: `acquireLock` takes the lock on its first non-blocking `Flock` when uncontended and never enters the poll loop, so an uncontended acquire cannot read as a timeout at any bound, however loaded the machine.
  OLD:
	// lockBound is the lowered acquisition bound every test here drives the timeout
	// through, low enough to keep the suite fast and high enough that a machine
	// under load cannot make an uncontended acquire look like a timeout.
  NEW:
	// lockBound is the lowered acquisition bound every test here drives the timeout
	// through, so no case waits out the production figure.

- `internal/hooks/lock_write_test.go:15-17` — the helper asserts the record's op, key, via, level, message and a non-empty error; it does **not** assert the absence of `error_class` or `value`, which its callers do separately at `:167-190`.
  OLD:
	// assertLockWarn pins the single record a mutation that could not take the lock
	// leaves: the operation's own op, its key, its caller, and the lock error —
	// and nothing that names a write phase that never ran.
  NEW:
	// assertLockWarn pins the single record a mutation that could not take the lock
	// leaves: the operation's own op, its key, its caller, and the lock error.

NOTES (context — not work items):
- **Your "already green from 5-1/4-1" framing is honest and was verified.** With both WARN lines deleted, exactly the six WARN-asserting cases fail; write-nothing, byte-identity, sentinel-through-the-wrap and no-removal-silence all still pass, which is what you said they were.
- The `CleanStale` silence pin genuinely discriminates: adding a `logger.Warn("clean-stale", …)` to its acquire branch fails `it emits no store-side WARN when CleanStale cannot take the lock`.
- Token retention verified independently: first `hook set` mints `tok000`, stamps once, fails on the lock; the retry after `release()` reads the token back, mints nothing, stamps nothing, writes exactly one entry under `tok000`. It would fail on any rollback or unstamp.
- The no-hang lower bound is structurally safe (the acquire's deadline is computed strictly after `start`); ceiling is 20× (1.2s) against a 60ms bound; `-count=5` showed no drift. **Optional, take only if free**: `hook rm`'s no-hang case asserts only the ceiling while the `hook set` twin also asserts `elapsed >= lockBound` — the lower bound is the half that proves the acquire actually waited.
- `assertOneLockWarn`'s "One line, so the dirty-flag touch cannot have run behind it either" is true only because `cmd`'s `TestMain` poisons `PORTAL_STATE_DIR`. Under a real state dir a *successful* touch emits nothing, so the inference would not hold. Currently correct; worth knowing before anyone reuses the helper with a live state dir.
- `holdHooksSidecar`'s doc comment (`cmd/hooks_read_lock_test.go:19-24`) still frames the fixture entirely in terms of reads now that it also serves the write suite. Not false, just half the picture. **Optional.**

## Attempt 1

## Attempt 1

All twelve acceptance criteria behaviourally met; the production code is correct — **do not change it**. Two test issues and two comment corrections.

ISSUES:
- `internal/hooks/lock_write_test.go:131` — **no test discriminates `op=set` from `classifySet`'s verdict**, so the decision this task exists to make is unverified. Proven: replacing the WARN branch in `Store.Set` with a version that loads the file, calls `classifySet`, and files the WARN under that verdict leaves the **entire unit lane passing** (`go test ./...` clean), as do both `cmd` lock suites. The cause is fixture shape: every timeout fixture either has no `hooks.json` at all or holds a *different* key (`tok999` while setting `tok123`), so `classifySet` returns `"set"` anyway and the wrong implementation is indistinguishable. This also means acceptance criterion 1's "without loading, classifying" has no observable pin — the `op` value is the only surface on which loading-then-classifying is detectable, and it is not exercised.
  FIX (**take this one — the reviewer's recommended ALTERNATIVE**): add a second, separately named subtest for the case, e.g. `"it files under op=set even where a completed call would have classified as modify"` — seed the same key *and* event with a different command (`{"tok123":{"on-resume":"cmd-a"}}`) before `holdSidecar`, keep the `Set("tok123", "on-resume", "npm start", "cli")` call and the `assertLockWarn(t, sink, "set", "tok123", "cli")` assertion. This keeps both fixture shapes under the WARN assertion and names the decision in the test list where a future reader will look for it — the task text calls the decision out twice, so it deserves its own case.
  (The narrower alternative — just re-seeding the existing subtest's fixture — also works but loses the fresh-file shape and does not name the decision.)
  VERIFY by re-running the mutation above: it must now FAIL.
  CONFIDENCE: high

- `cmd/hooks_write_lock_test.go:64` — `assertOneLockWarn` hand-rolls the level/component/op/via checks that `assertHooksRecord` (`cmd/logging_capture_test.go:55`, documented as "the parts every hooks-component record shares") already owns, and both existing hooks-WARN assertions in the package go through it (`assertTouchWarn` at `cmd/hooks_test.go:988`, `standDownWant` at `cmd/run_hook_stale_cleanup_test.go:562`). The hand-rolled version also silently drops the `msg` assertion the shared helper enforces, so the cmd surface would not catch a WARN emitted with the right `op` and the wrong message — a contract `internal/hooks/store.go:19` states explicitly ("The op verb is both the slog message and a required `op` attr").
  FIX: delegate the shared half — `assertHooksRecord(t, rec, hooksRecordWant{level: slog.LevelWarn, msg: wantOp, op: wantOp, via: "cli"})` — and keep only the emission-specific checks (`hook_key`, and the `error_class`/`value` absence) local, which is exactly the split `assertTouchWarn` uses.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- `cmd/hooks_write_lock_test.go:17-19` — the stated rationale describes a hazard that cannot occur: `acquireLock` takes the lock on its first non-blocking `Flock` when uncontended and never enters the poll loop, so an uncontended acquire cannot read as a timeout at any bound, however loaded the machine.
  OLD:
	// lockBound is the lowered acquisition bound every test here drives the timeout
	// through, low enough to keep the suite fast and high enough that a machine
	// under load cannot make an uncontended acquire look like a timeout.
  NEW:
	// lockBound is the lowered acquisition bound every test here drives the timeout
	// through, so no case waits out the production figure.

- `internal/hooks/lock_write_test.go:15-17` — the helper asserts the record's op, key, via, level, message and a non-empty error; it does **not** assert the absence of `error_class` or `value`, which its callers do separately at `:167-190`.
  OLD:
	// assertLockWarn pins the single record a mutation that could not take the lock
	// leaves: the operation's own op, its key, its caller, and the lock error —
	// and nothing that names a write phase that never ran.
  NEW:
	// assertLockWarn pins the single record a mutation that could not take the lock
	// leaves: the operation's own op, its key, its caller, and the lock error.

NOTES (context — not work items):
- **Your "already green from 5-1/4-1" framing is honest and was verified.** With both WARN lines deleted, exactly the six WARN-asserting cases fail; write-nothing, byte-identity, sentinel-through-the-wrap and no-removal-silence all still pass, which is what you said they were.
- The `CleanStale` silence pin genuinely discriminates: adding a `logger.Warn("clean-stale", …)` to its acquire branch fails `it emits no store-side WARN when CleanStale cannot take the lock`.
- Token retention verified independently: first `hook set` mints `tok000`, stamps once, fails on the lock; the retry after `release()` reads the token back, mints nothing, stamps nothing, writes exactly one entry under `tok000`. It would fail on any rollback or unstamp.
- The no-hang lower bound is structurally safe (the acquire's deadline is computed strictly after `start`); ceiling is 20× (1.2s) against a 60ms bound; `-count=5` showed no drift. **Optional, take only if free**: `hook rm`'s no-hang case asserts only the ceiling while the `hook set` twin also asserts `elapsed >= lockBound` — the lower bound is the half that proves the acquire actually waited.
- `assertOneLockWarn`'s "One line, so the dirty-flag touch cannot have run behind it either" is true only because `cmd`'s `TestMain` poisons `PORTAL_STATE_DIR`. Under a real state dir a *successful* touch emits nothing, so the inference would not hold. Currently correct; worth knowing before anyone reuses the helper with a live state dir.
- `holdHooksSidecar`'s doc comment (`cmd/hooks_read_lock_test.go:19-24`) still frames the fixture entirely in terms of reads now that it also serves the write suite. Not false, just half the picture. **Optional.**
