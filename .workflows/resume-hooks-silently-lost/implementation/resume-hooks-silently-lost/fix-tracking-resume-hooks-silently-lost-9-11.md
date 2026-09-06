## Attempt 1

ISSUES:
- `cmd/state_hydrate.go:179` — the hydrate `via` is now a bare literal at a call site with no test at that layer. `cmd/hooks_read_lock_test.go` pins the degraded-read `via` for the other three callers (`doctor` at :48, `cli` at :91, `internal` at :140); `hydrate` is now the only caller-supplied `via` in `cmd` with no equivalent. Change `hooks.ViaHydrate` to `hooks.ViaCLI` there and the whole suite stays green — which is precisely the regression class the task's AC #2 names. The `internal/hooks` subtest proves the parameter is honoured, not that this call site passes the right value.
  FIX: add a subtest to `cmd/hooks_read_lock_test.go` alongside `TestHookListDegradedRead`: stage a store via `hookstest.StageStore`, `hookstest.HoldHooksSidecar`, `hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)`, `logtest.Install(t)`, then call `execShellOrHookAndExit(hydrateCfg(t, hydrateCfgOpts{HookKey: hookstest.SubjectSeedA, HookStore: store, ExecShell: (&stubExecShell{}).fn(), Logger: …}))` (the helpers exist — see `cmd/state_hydrate_exec_log_test.go:29-45`) and assert `hookstest.AssertDegradedRead(t, sink, "hydrate")`.
  CONFIDENCE: high

- `internal/hooks/lookup_test.go:152-163` — `"it returns the registered on-resume command through the store method"` is a strict subset of `"returns the command verbatim when on-resume is registered"` (`:66-80`), which asserts the same return path *and* verbatim handling of shell metacharacters. The fold-don't-duplicate rule was applied to the empty-key pair in this same file and not to this one. Related: the new `TestStoreLookupOnResume` is a second top-level test function for the one method the file's `TestLookupOnResume` already covers.
  FIX: rename the existing `:66` subtest to the task's mandated name `"it returns the registered on-resume command through the store method"` (keeping its metacharacter fixture), delete the new `:152` subtest, and move the surviving two new subtests (`"it records the caller's via…"`, `"it refuses an empty hook key before reading the file"`) into `TestLookupOnResume`, dropping the now-empty `TestStoreLookupOnResume`.
  ALTERNATIVE: keep `TestStoreLookupOnResume` as a separate function if the split reads better, but the redundant happy-path subtest should go either way — the merge is the tidier of the two and matches the file's existing single-function-per-method shape.
  CONFIDENCE: medium

COMMENT_CORRECTIONS: none.

NOTES:
- The replaced source guard was verified correct to remove: `StaleKeys` is pure over an already-loaded map, acquires nothing, and never did — at the pre-feature commit it already took a plain map. The re-entrancy property the spec cares about is guarded elsewhere and untouched by `TestMutationsDoNotCallExportedLoadOrSave`, which forbids the mutation paths from calling the actual locking front doors. A spec corrigendum is owed and the orchestrator is applying it.
- All four `via` values verified unchanged, including the sweep's internal pre-read.
- The deleted subtest was a genuine duplicate — identical fixture, identical assertion, comment included. The materially different empty-key case survives.
- `TestMutationsDoNotCallExportedLoadOrSave`'s `forbidden` map lists `"Get"`, which `internal/hooks` does not declare. Pre-existing dead entry, untouched by this diff.
- The file name `cleanstale_staleness_guard_test.go` no longer matches its lead test's subject. Cosmetic; the file also holds the mutation guard, so a rename is a judgement call.
