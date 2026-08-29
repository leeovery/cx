# Consolidation Tasks: resume-hooks-silently-lost (Phase 5)

*Grouped by destination file rather than one-per-finding: F2+F4 both land in `cmd/hookkey_vocabulary_test.go`, F3+F5+F6 all land in `cmd/testhelpers_test.go`, F7+F8 are both one-line string corrections. Running eight sequential tasks over the same overlapping `cmd` test files would churn line numbers for no benefit.*

## Task 1: One home for the lock fixture and the degraded-read assertion
placement: phase 5
severity: duplication

**Problem**: Phase 5 wrote its lock test scaffolding twice, once per package. The sidecar hold is byte-identical in both bodies — open `<path>.lock` `O_RDWR|O_CREATE`, `LOCK_EX|LOCK_NB`, `sync.Once` release, `t.Cleanup` — at `internal/hooks/lock_test.go:37-56` (`holdSidecar`) and `cmd/hooks_read_lock_test.go:25-42` (`holdHooksSidecar`). Sidecar creation is one named helper against three inline copies of the same `os.WriteFile(path+".lock", nil, 0o600)`: `internal/hooks/store_test.go:53-58` (`createSidecar`) vs `cmd/bootstrap_production_test.go:128`, `cmd/hook_sweep_lock_timeout_test.go:218`, `cmd/hooks_read_lock_test.go:88`. The degraded-read breadcrumb assertion makes the same four checks — exactly one `load-unlocked` record, DEBUG, `op`, `via`, non-empty `error` — at `internal/hooks/read_lock_test.go:56-90` and `cmd/hooks_read_lock_test.go:54-74`. All six sites are new in phase 5.

**Solution**: One home for the sidecar fixture and the degraded-read assertion, consumed by both packages.

**Outcome**: A change to how the sidecar is held, created or asserted on is one edit, not two.

**Do**:
- Site them in `internal/transienttest` — already the hooks-domain test-only package both sides import (`internal/hooks/store_shape_test.go:11` and `cmd` both do). Widen its `doc.go` sentence to say so.
- Export `HoldHooksSidecar(t, hooksPath) func()`, `HoldHooksSidecarShared(t, hooksPath)`, `CreateHooksSidecar(t, hooksPath)` and `AssertDegradedRead(t, sink, wantVia)`.
- If widening `transienttest` is unwanted on reading, a new `internal/hookstest` is the alternative — say which you took and why.
- `holdSidecarShared` (`internal/hooks/read_lock_test.go:23-35`) has no counterpart and needs none; export it anyway as the shared-hold variant.
- Pure movement: the assertion set, the flock modes and the bounds are unchanged, and no test loses a check.

**Acceptance Criteria**:
- One sidecar hold, one shared hold, one sidecar create and one degraded-read assertion exist, in one package, consumed by both `internal/hooks` and `cmd`
- The three inline `os.WriteFile(path+".lock", …)` copies are gone
- No assertion is dropped: the degraded-read helper still checks exactly-one-record, DEBUG level, `op`, `via` and a non-empty `error`
- No test changes its verdict
- `go test ./...`, `go test -race ./internal/hooks/` and `go test -tags integration -p 1 ./...` all pass

**Tests**:
- Existing: both packages' lock suites must stay green unchanged
- No new test — fixture movement earns none. Prove the moved degraded-read assertion still discriminates by mutation: make a degraded read emit at INFO instead of DEBUG and confirm it reddens.

## Task 2: The seam fakes and the stale seed go to their declared home
placement: phase 5
severity: near-miss

**Problem**: Two findings with one destination. **(a)** `cmd/hook_sweep_snapshot_order_test.go:21-35` (`sideEffectPaneLister`, new in 5-4, 3 uses) implements `AllPaneLister` exactly as `cmd/bootstrap_production_test.go:100-113` (`stubAllPaneLister`, 24 uses) does, differing only in carrying a `during func()` instead of `err`/`restoring`/`restoringErr`; both delegate `TryGetServerOption` to the shared `restoringOption`. Meanwhile `cmd/hookkey_vocabulary_test.go:1-4` declares itself the home for "the seam fakes that answer with the hook-key vocabulary" and holds only two of them — `stubAllPaneLister`, `mockKeyResolver` (`cmd/hooks_test.go:314`, 57 uses), `recordingPaneStamper` (`cmd/hooks_pane_token_test.go:20`, 12 uses) and `stampedPane` (`cmd/hooks_write_lock_test.go:89-101`, new in 5-3) are all sited elsewhere. **(b)** The same two-entry stale seed literal `{reapableSeedA: "cmd-gone", liveSeedA: "cmd-live"}` is declared three times byte-identically — `cmd/hook_sweep_lock_timeout_test.go:19-23` (`lockedStaleSeed`, new in 5-5), `cmd/run_hook_stale_cleanup_test.go:382-385` and `:570-573` — while the seed vocabulary it draws from is single-sourced one file over at `cmd/hookkey_vocabulary_test.go:15-31`.

**Solution**: Fold `sideEffectPaneLister` into `stubAllPaneLister`, move the four scattered fake declarations into the file that declares itself their home, and single-source the stale seed beside the seed vocabulary.

**Outcome**: One fake per seam, all four sited where the package says they live, and one stale-seed literal beside the vocabulary it is built from.

**Do**:
- Add an optional `during func()` to `stubAllPaneLister`, invoked at the top of `ListAllPaneHookKeys`, and delete `sideEffectPaneLister` — 3 call sites re-point.
- Move the **type declarations** of `stubAllPaneLister`, `mockKeyResolver`, `recordingPaneStamper` and `stampedPane` into `cmd/hookkey_vocabulary_test.go`. Declaration moves only — zero call-site churn.
- **DO NOT MERGE the three hook-key seam fakes.** `mockKeyResolver`'s fixed key and `recordingPaneStamper`'s must-not-be-called error field are what several cases discriminate on, and `stampedPane`'s stamp→resolve round-trip would change what a fixed-key resolver returns after a stamp. Relocation only.
- **The pre-existing four-way merge** of `stubAllPaneLister` / `recordingHookKeyLister` / `fakeHookLister` (76 construction sites, one a value receiver) **stays banked** — not in scope.
- Add one package-level `staleHookSeed` beside the seed vocabulary in `cmd/hookkey_vocabulary_test.go`; delete all three literals and re-point their five uses.

**Acceptance Criteria**:
- `sideEffectPaneLister` is gone and `stubAllPaneLister` carries an optional `during` hook
- All four named fakes are declared in `cmd/hookkey_vocabulary_test.go`
- The three hook-key seam fakes remain three distinct types — none merged
- One `staleHookSeed` literal remains; the three copies are gone and all five uses re-point
- No assertion changes and no test changes its verdict
- `go test ./...` and `go test -tags integration -p 1 ./cmd/...` pass

**Tests**:
- Existing: the sweep suites, the snapshot-order suite and the hook-key suites must stay green unchanged
- No new test. Prove the folded `during` hook is live by mutation: remove its invocation and confirm the snapshot-order suite reddens.

## Task 3: The staging helpers reach their declared home, and one seeder serves every caller
placement: phase 5
severity: drift

**Problem**: Three findings with one destination. **(a)** `cmd/hook_sweep_lock_timeout_test.go:211-231` (`saveDeniedStore`, new in 5-5) repeats `cmd/bootstrap_production_test.go:117-134` (`newTempHooksStore`) line for line — write seed, create sidecar, `hooks.NewStore` — adding only a caller-chosen directory and a `chmod 0500`. And `newTempHooksStore` and `readFileBytes` sit in a suite file that claims no staging role, while `cmd/testhelpers_test.go:1-4` declares itself the home for staging helpers and already reaches back out for the reader at `:99`; phase 5 added four more consumers of that pair. `cmd/hooks_write_lock_test.go:19` declares `lockBound`, consumed from `cmd/hook_sweep_lock_timeout_test.go` at 6 sites — the same siting shape one constant over. **(b)** `cmd/hooks_write_lock_test.go:23-32` (`runHookSetCapturing`, new in 5-3, 7 uses) differs from `cmd/testhelpers_test.go:28-35` (`runHookSet`, 10 uses) only in returning the captured buffer — which `runHookRm` already does, so one package drives two hook verbs through three helpers with two return shapes. **(c)** Task 5-3 uses the named `assertHooksFileUnchanged` at three sites while 5-4/5-5 write the same before/after `bytes.Equal` inline at `cmd/hook_sweep_lock_timeout_test.go:65`, `:195`, `:290`.

**Solution**: Move the staging helpers into the file that declares itself their home, give the seeder a directory and a deny-writes option so the third seeder disappears, unify the two `hook set` drivers on `runHookRm`'s shape, and give the file-unchanged assertion a context parameter so the three inline copies can use it without losing their messages.

**Outcome**: One staging home, one temp-store seeder, one `hook set` driver, and one way to assert a file was untouched.

**Do**:
- Give the seeder a directory and a deny-writes option — `newTempHooksStoreIn(t, dir, seed)` plus the `chmod`, or a small options struct — so `saveDeniedStore` becomes a two-line caller and is deleted.
- Move `newTempHooksStore`, `readFileBytes` and `lockBound` into `cmd/testhelpers_test.go`.
- Give `runHookSet` the `(string, error)` signature `runHookRm` already has, delete `runHookSetCapturing`, and re-point its 7 uses plus the 10 existing `runHookSet` uses (`_, err :=`).
- Give `assertHooksFileUnchanged` a **variadic context string** (existing five callers unchanged), then re-point phase 5's three inline sites, passing their present message as the context — "rewritten under a held lock", "rewritten by the daemon under a held lock", "rewritten on a lock stand-down". Those bespoke messages must survive; a fixed-message helper would flatten them.
- **Leave the pre-existing inline sites in `cmd/run_hook_stale_cleanup_test.go` alone.** The three `reflect.DeepEqual` ones there differ semantically from `bytes.Equal` on the absent-vs-empty edge; that is banked and is not a mechanical swap.
- Optional follow-on inside the same edit, take only if it stays clean: have `seedHooksJSON` (`cmd/doctor_test.go:811`) build its JSON and delegate, retiring the pre-existing second seeder too.

**Acceptance Criteria**:
- `saveDeniedStore` is gone; one seeder serves every caller including the deny-writes case
- `newTempHooksStore`, `readFileBytes` and `lockBound` live in `cmd/testhelpers_test.go`
- One `hook set` driver remains, with the same return shape as `runHookRm`
- Phase 5's three inline file-unchanged assertions route through the helper and each keeps its own failure message
- `cmd/run_hook_stale_cleanup_test.go`'s inline sites are untouched
- No assertion changes and no test changes its verdict
- `go test ./...` and `go test -tags integration -p 1 ./cmd/...` pass

**Tests**:
- Existing: the whole `cmd` hooks and sweep suites must stay green unchanged
- No new test. Confirm the deny-writes path still reaches its subject: `error_class=write-failed-temp-create` must still be exercised after the seeder change — that fixture has silently stopped testing its own subject once before in this work unit.

## Task 4: Two strings name things that are no longer true
placement: phase 5
severity: comments

**Problem**: **(a)** `cmd/run_hook_stale_cleanup.go:103` reads through `store.LoadSnapshot("internal")` — task 5-2 replaced the plain `Load` — but `:105` still warns `"stale-hook cleanup: hookStore.Load failed"`, pinned by the test constant `loadWarnFmt` at `cmd/run_hook_stale_cleanup_test.go:25`. A reader greping the log for the failing read is sent to the wrong method, and the two reads now have genuinely different bounds (`snapshotLockTimeout` vs `lockTimeout`), so the distinction matters. **(b)** `internal/hooks/lock.go:42` wraps with `fmt.Errorf("open hooks lock %s: %w", path, err)` while the wrapped `*os.PathError` already renders the path, so the DEBUG breadcrumb carries `error=open hooks lock /…/hooks.json.lock: open /…/hooks.json.lock: no such file or directory`. Task 5-1 wrote that wrap when it could only fire on a rare mutation failure; 5-2 made it the shape **every** read on a pre-sidecar install logs, because `acquireSharedLock` passes no `O_CREATE` and an absent sidecar is the ordinary case on an install that has never mutated.

**Solution**: Rename the WARN to name `LoadSnapshot`, and drop the redundant `%s` from the open-branch wrap.

**Outcome**: The log names the method that actually ran, and the routine ENOENT breadcrumb renders the sidecar path once.

**Do**:
- Rename the message to name `LoadSnapshot` and update the one test constant. The assertion is unchanged and no structured attr moves.
- Drop the `%s` from the open-branch wrap only: `fmt.Errorf("open hooks lock: %w", err)`. The path survives inside the `*os.PathError`.
- **Leave `internal/hooks/lock.go:53` and `:57` as they are** — `unix.Errno` and `ErrLockHeld` carry no path of their own, so their wraps are not redundant.
- Neither string is spec-governed: the spec fixes the structured `op`/`reason`/`via` surface, not these freeform lines. This is not an observability-vocabulary change.
- Do **not** write a CHANGELOG entry.

**Acceptance Criteria**:
- The sweep's load-failure WARN names `LoadSnapshot`, and its test constant matches
- The open-branch wrap renders the sidecar path once, not twice
- `internal/hooks/lock.go`'s other two wraps are unchanged
- The existing assertion over the lock text (`internal/hooks/lock_test.go:342`, `strings.Contains(err.Error(), "hooks lock")`) still holds — no test changes
- No structured attr, `op`, `reason` or `via` value changes anywhere
- `go test ./...` and `go test -tags integration -p 1 ./...` pass

**Tests**:
- Existing: the sweep's load-failure case and the lock suites must stay green, with only the one constant updated
- No new test. Show the rendered error before and after so the doubled path is visibly gone.

## Bank Disposition

- Pre-existing repo-wide lint/format debt outside this task's surface. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-1-4",
    "source": "executor",
    "summary": "Pre-existing repo-wide lint/format debt outside this task's surface.",
    "detail": "gofmt -l . reports internal/tui/help_modal_test.go as unformatted on a clean tree (not touched by this work), and golangci-lint run ./... reports 31 modernize findings across the repo (errorsastype, stringscut, stringsseq), 9 of them in cmd — e.g. cmd/root.go:199, cmd/bootstrap_progress.go:133, main.go:64, main.go:75. All predate this work unit; a single sweep would clear them.",
    "files": [
      "internal/tui/help_modal_test.go",
      "cmd/root.go",
      "cmd/bootstrap_progress.go",
      "main.go"
    ]
  }
  ```

- runHookStaleCleanup now takes 5 parameters, one over the project convention. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-1-4",
    "source": "reviewer",
    "summary": "runHookStaleCleanup now takes 5 parameters, one over the project convention.",
    "detail": ".claude/skills/golang-code-style/SKILL.md:174 sets <=4 parameters, options-struct beyond. The task mandated onSkipped as a positional parameter, so it is correct as delivered; folding onRemoved/onSkipped (and whatever the lock phase needs) into a small callbacks struct would touch cmd/run_hook_stale_cleanup.go:37-43 plus ~14 call sites across files owned by sibling tasks 1-1 through 1-3.",
    "files": [
      "cmd/run_hook_stale_cleanup.go",
      "cmd/doctor.go",
      "cmd/state_daemon.go",
      "cmd/run_hook_stale_cleanup_test.go",
      "cmd/hook_sweep_restore_standdown_test.go",
      "cmd/hook_retention_shape_test.go",
      "cmd/hookkey_no_regression_upgrade_test.go",
      "cmd/rename_restore_cleanup_survival_integration_test.go",
      "cmd/hook_sweep_standdown_report_test.go"
    ]
  }
  ```

- Three parallel fakes for the one AllPaneLister seam in package cmd. — residue — pre-existing; rides to the end-of-implementation analysis

  ```json
  {
    "source": "finder",
    "pre_existing": true,
    "summary": "Three parallel fakes for the one AllPaneLister seam in package cmd.",
    "detail": "stubAllPaneLister (cmd/bootstrap_production_test.go:86-100), recordingHookKeyLister (cmd/run_hook_stale_cleanup_test.go:13-30, a strict superset adding hookKeyCalls) and fakeHookLister (cmd/doctor_test.go:797-808, value receiver) all predate this phase; phase 1 added the same restoring/restoringErr pair and the same TryGetServerOption delegation to each, and two of them are now used interchangeably inside one test function (cmd/hook_sweep_standdown_report_test.go:26 vs :51). The shared restoringOption helper single-sources the semantics, so the fresh duplication is 4 lines x 3; the three-fakes situation itself is pre-existing. Phase 2 task text re-points all three by name, so a merge is cheapest there or at the end-of-implementation pass.",
    "files": [
      "cmd/bootstrap_production_test.go",
      "cmd/run_hook_stale_cleanup_test.go",
      "cmd/doctor_test.go"
    ]
  }
  ```

- Two temp hooks-store seeders in package cmd, neither derived from the other. — residue — pre-existing; rides to the end-of-implementation analysis

  ```json
  {
    "source": "finder",
    "pre_existing": true,
    "summary": "Two temp hooks-store seeders in package cmd, neither derived from the other.",
    "detail": "newTempHooksStore(t, rawJSON) (cmd/bootstrap_production_test.go:102) and seedHooksJSON(t, keys...) (cmd/doctor_test.go:810) both write a hooks.json into a t.TempDir() and return (*hooks.Store, path); both predate this phase and this phase consumed both heavily. A future key-shape change re-points each independently.",
    "files": [
      "cmd/bootstrap_production_test.go",
      "cmd/doctor_test.go"
    ]
  }
  ```

- The restore-window rule is now single-sourced in cmd, but the daemon states the inverse error policy twice with no shared name. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-1-6",
    "source": "executor",
    "summary": "The restore-window rule is now single-sourced in cmd, but the daemon states the inverse error policy twice with no shared name.",
    "detail": "cmd/state_daemon.go:172-180 (tick) and :337-348 (defaultShutdownFlush) both read state.IsRestoringSet(deps.Client) and, on error, log a WARN and return without work — the same read-failed-stand-down-loudly shape, spelled twice, each under its own rationale comment. It is a genuine policy sibling of restoreWindowActive (which stands down silently and folds the error in), so the two now sit in one package as unnamed opposites: a reader cannot tell from the call sites that the divergence is deliberate. Consolidating would touch daemon lifecycle code owned by other phases.",
    "files": [
      "cmd/state_daemon.go",
      "cmd/run_hook_stale_cleanup.go"
    ]
  }
  ```

- A third reader of the same failed-read-counts-as-set posture sits in cmd, in a file the daemon entry does not name. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-1-6",
    "source": "reviewer",
    "summary": "A third reader of the same failed-read-counts-as-set posture sits in cmd, in a file the daemon entry does not name.",
    "detail": "cmd/state_commit_now.go:108-118 takes the identical posture (presuming @portal-restoring set to protect in-flight restore) behind the IsRestoring func() (bool, error) seam, with a third reporting policy. With restoreWindowActive (cmd/run_hook_stale_cleanup.go:44) now naming the rule for one of four readers, the natural end-state is one named home for the posture — plausibly internal/state beside IsRestoringSet, taking the report/quiet policy from the caller — rather than one named predicate and three anonymous restatements. Extends, does not duplicate, the executor entry, which lists only cmd/state_daemon.go.",
    "files": [
      "cmd/state_commit_now.go",
      "cmd/state_daemon.go",
      "cmd/run_hook_stale_cleanup.go",
      "internal/state/markers.go"
    ]
  }
  ```

- TestDoctorFixPrunedHookOutput is now fully subsumed by a sibling test. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-1-9",
    "source": "executor",
    "summary": "TestDoctorFixPrunedHookOutput is now fully subsumed by a sibling test.",
    "detail": "With the exact-equality pruned-line assertion single-sourced into assertStalePrunesApplied (cmd/doctor_test.go:909), TestDoctorFixPrunedHookOutput (\"it leaves doctor --fix stdout unchanged\") asserts a strict subset of TestDoctorFixPrunesStaleEntriesThenRediagnosesClean (cmd/doctor_test.go:1385) — same fixture, same helper, no additional assertion. Deleting it is the honest consolidation but costs a case, which task 1-9 acceptance bar forbids; it needs an owner who can approve the count change.",
    "files": [
      "cmd/doctor_test.go"
    ]
  }
  ```

- Four further cmd test files touching doctor.go / run_hook_stale_cleanup.go still carry concern-derived names. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-1-9",
    "source": "executor",
    "summary": "Four further cmd test files touching doctor.go / run_hook_stale_cleanup.go still carry concern-derived names.",
    "detail": "Task 1-9 named four files; the convention breach is wider. cmd/hooks_cleanstale_single_caller_guard_test.go and cmd/hookkey_no_regression_upgrade_test.go (unit lane) plus cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go and cmd/rename_restore_cleanup_survival_integration_test.go (integration lane) all exercise these two sources under names derived from the concern. All four predate this phase, so they are outside the four the task was scoped to, but the acceptance criterion every remaining test file in cmd touching these two sources is named after a source file is only satisfied for the phase own output.",
    "files": [
      "cmd/hooks_cleanstale_single_caller_guard_test.go",
      "cmd/hookkey_no_regression_upgrade_test.go",
      "cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go",
      "cmd/rename_restore_cleanup_survival_integration_test.go"
    ]
  }
  ```

- staleSeed is now declared twice, byte-identical, in one file. — folded into task 2

  ```json
  {
    "task": "resume-hooks-silently-lost-1-9",
    "source": "reviewer",
    "summary": "staleSeed is now declared twice, byte-identical, in one file.",
    "detail": "cmd/run_hook_stale_cleanup_test.go:428 and :625, ~200 lines apart. The merge is what turned two cross-file copies into a same-file duplicate. Task 1-9 Do list scoped fixture parameterisation to seedStalePruneFixture only, so this is not a criterion miss, but it is the same class of thing the task exists to remove and the fix is entirely local — a package-level staleHookSeed beside the seed vocabulary at :25-33 would close it in one edit. Also: doctor_test.go now carries two helper regions (:856-922 and :1697-1750) with the latter calling the former across ~800 lines, so a reader tracing the fixture crosses the file twice.",
    "files": [
      "cmd/run_hook_stale_cleanup_test.go",
      "cmd/doctor_test.go"
    ]
  }
  ```

- The removal leaves two byte-identical subtests in TestDoctorStaleHooksCheck. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-1-11",
    "source": "reviewer",
    "summary": "The removal leaves two byte-identical subtests in TestDoctorStaleHooksCheck.",
    "detail": "cmd/doctor_test.go:1102-1108 (\"it reads the marker before counting\") is now character-for-character identical to cmd/doctor_test.go:1078-1084 (\"it reports not evaluable while the restore marker is set\") — same seedHooksJSON(t, reapableSeedA), same fakeHookLister{keys: []string{\"sessB:0.0\"}, restoring: true}, same assertRestoreWindowResult. The strings.Contains check was the only thing distinguishing them, and it was inert, so no coverage is lost — but the intent the name carries (the marker is read BEFORE the count is computed) is now indistinguishable from its neighbour. Task 1-11 could not resolve it: its instruction is Remove nothing else and its criteria mandate every affected test keeps its cases. Two remedies: delete the duplicate (coverage unchanged), or differentiate it by seeding a fixture where a count would otherwise be produced and asserting its absence by a route the equality check does not already cover. The sibling \"it reads the marker before the empty-live-set branch\" (:1094-1100) is genuinely distinct and should stay.",
    "files": [
      "cmd/doctor_test.go"
    ]
  }
  ```

- The gone-pane message reaching the user is three wraps deep, and Phase 4 reworks the same call site for hook rm wording. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-2-3",
    "source": "reviewer",
    "summary": "The gone-pane message reaching the user is three wraps deep, and Phase 4 reworks the same call site for hook rm wording.",
    "detail": "portal hook set on a dead pane renders: failed to resolve hook key for current pane: no pane answers to \"%999\": tmux show-options -p -t %999: exit 1: no such pane: %999 — cmd/hooks.go:66 prefixes internal/tmux/tmux.go:245, which prefixes CommandError.Error(). Spec 4.1 is satisfied (tmux words survive unaltered at the tail), but 4.2 fixes exact removal wording at the same resolveCurrentPaneKey call site, so trimming the redundant layer belongs in one pass across both verbs rather than half-done here.",
    "files": [
      "cmd/hooks.go",
      "internal/tmux/tmux.go"
    ]
  }
  ```

- ResolveStructuralKey and ListAllPanes have no production callers. — residue — pre-existing; rides to the end-of-implementation analysis

  ```json
  {
    "source": "finder",
    "pre_existing": true,
    "summary": "ResolveStructuralKey and ListAllPanes have no production callers.",
    "detail": "internal/tmux/tmux.go:226 and :590 are reached only from internal/tmux/tmux_test.go:1569-1611 and :1423-1556. Production reaches the structural shape through StructuralKeyFormat + ListAllPanesWithFormat (cmd/bootstrap/stale_marker_cleanup.go:57) instead. Both were already test-only before phase 2 — it did not orphan them — but CLAUDE.md:60 describes all three as serving non-hook structural use, which holds only for the constant. Verify before deleting: phase 3 retires the positional hook machinery and may reach these.",
    "files": [
      "internal/tmux/tmux.go",
      "cmd/bootstrap/stale_marker_cleanup.go",
      "CLAUDE.md"
    ]
  }
  ```

- A saver-hosting integration fixture omits the teardown guard CLAUDE.md prescribes for exactly its shape. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-2-7",
    "source": "reviewer",
    "summary": "A saver-hosting integration fixture omits the teardown guard CLAUDE.md prescribes for exactly its shape.",
    "detail": "cmd/state_daemon_hook_cleanup_integration_test.go:52 spawns a _portal-saver-hosted daemon that flushes on SIGHUP at teardown but never calls portaltest.RegisterStateDirTeardownGuard. This is the direct cause of the TempDir RemoveAll: directory not empty failure the reviewer reproduced (1 in 4 runs) — every assertion logs success first, then cleanup unlinks a state dir the daemon is still writing. Its siblings register the guard (cmd/state_commit_now_reentrancy_integration_test.go:44, internal/tmux/portal_saver_endstate_integration_test.go:36). The file is one task 2-7 edited, but the fix is a new setup call, which its pure-movement brief excluded.",
    "files": [
      "cmd/state_daemon_hook_cleanup_integration_test.go",
      "internal/portaltest/teardown_guard.go"
    ]
  }
  ```

- stubAllPaneLister is the last piece of the hook-sweep seam vocabulary still sited with a single consumer. — folded into task 2

  ```json
  {
    "task": "resume-hooks-silently-lost-2-8",
    "source": "executor",
    "summary": "stubAllPaneLister is the last piece of the hook-sweep seam vocabulary still sited with a single consumer.",
    "detail": "cmd/bootstrap_production_test.go:101-115 declares stubAllPaneLister, an AllPaneLister fake structurally identical to recordingHookKeyLister (now in cmd/hookkey_vocabulary_test.go:62) minus the call counter, and it reaches cross-file for restoringOption. It is used 3 times in its own file and 19 times in cmd/run_hook_stale_cleanup_test.go. Moving it — or collapsing the two fakes into one — belongs with whatever next consolidates the sweep seam fakes.",
    "files": [
      "cmd/bootstrap_production_test.go",
      "cmd/run_hook_stale_cleanup_test.go",
      "cmd/hookkey_vocabulary_test.go"
    ]
  }
  ```

- Two package-level names in package cmd now hold one key value. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-2-8",
    "source": "reviewer",
    "summary": "Two package-level names in package cmd now hold one key value.",
    "detail": "renameRestoreToken (cmd/rename_restore_cleanup_survival_integration_test.go:21) and reapableSeedB (cmd/hookkey_vocabulary_test.go:15) are both ReapableHookKey(1) and both package-scope in package cmd. Nothing composes them today, so it is latent rather than live. Same for liveKey at cleanstale_transient_listpanes_doctorfix_integration_test.go:99. The end state is those two in-package fixtures reaching the vocabulary directly (reapableSeedB / a new liveSeed*) rather than re-deriving the index. The task text prescribed ReapableHookKey(n) literally, so the executor is correct as delivered; closing this touches the seed vocabulary that other phases re-point.",
    "files": [
      "cmd/rename_restore_cleanup_survival_integration_test.go",
      "cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go",
      "cmd/hookkey_vocabulary_test.go"
    ]
  }
  ```

- A few hook-file preamble sites outside the three hooks test files could take the bare helper now that it exists in the shared file. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-2-9",
    "source": "reviewer",
    "summary": "A few hook-file preamble sites outside the three hooks test files could take the bare helper now that it exists in the shared file.",
    "detail": "cmd/state_daemon_test.go:792 and :813 are t.Setenv(\"PORTAL_HOOKS_FILE\", filepath.Join(t.TempDir(), \"hooks.json\")) — byte-equivalent to a bare hooksFileInTempDir(t); cmd/version_guard_test.go:146 is the same modulo separator. Deliberately outside task 2-9 scope (F4 enumerated only cmd/hooks_test.go), and the fix reaches suites it did not touch. cmd/cleanstale_transient_listpanes_shared_test.go:26 is NOT a candidate — it nests under a portal/ subdir.",
    "files": [
      "cmd/state_daemon_test.go",
      "cmd/version_guard_test.go"
    ]
  }
  ```

- The last hand-rolled level filter now lives in internal/spawn, and it is not the same filter. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-2-10",
    "source": "reviewer",
    "summary": "The last hand-rolled level filter now lives in internal/spawn, and it is not the same filter.",
    "detail": "warnRecords at internal/spawn/terminalsconfig_test.go:21-29 is a second copy of the same helper, consumed 16 times across terminalsconfig_test.go, logemit_test.go, recipe_test.go, configadapter_script_test.go and resolver_config_test.go. It filters on r.Level == slog.LevelWarn, NOT >=, so it is not a drop-in for the new RecordsAtLevel — a re-point either widens 16 assertions to include ERROR or needs an exact-level sibling accessor in logtest. Out of task 2-10 scope (which named the two cmd sites), but it is what stops logtest owns the level filter from being true repo-wide.",
    "files": [
      "internal/logtest/capture.go",
      "internal/spawn/terminalsconfig_test.go",
      "internal/spawn/logemit_test.go",
      "internal/spawn/recipe_test.go",
      "internal/spawn/configadapter_script_test.go",
      "internal/spawn/resolver_config_test.go"
    ]
  }
  ```

- The same five-property audit-breadcrumb block is still written inline outside this task two helpers. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-2-10",
    "source": "reviewer",
    "summary": "The same five-property audit-breadcrumb block is still written inline outside this task two helpers.",
    "detail": "cmd/config_migrate_logging_test.go:42-57 is a verbatim fourth copy in the same package — level / msg / component == hooks / op / via, then its own path attr — and would be one assertHooksRecord call except that its sibling subtests parameterise component across hooks/aliases/projects, so the helper would need component as a field rather than a constant. The same shape recurs per-package in internal/alias/store_logging_test.go, internal/project/store_logging_test.go, internal/hooks/store_test.go and internal/storelog/clean_stale_test.go, each with its own component; the end state is a component-parameterised record assertion in logtest beside the typed accessors. Generalising hooksRecordWant that far is a design change to the helper task 2-10 was scoped to deliver.",
    "files": [
      "cmd/config_migrate_logging_test.go",
      "cmd/logging_capture_test.go",
      "internal/alias/store_logging_test.go",
      "internal/project/store_logging_test.go",
      "internal/hooks/store_test.go",
      "internal/storelog/clean_stale_test.go",
      "internal/logtest/capture.go"
    ]
  }
  ```

- Two near-duplicate TMUX_PANE-unset subtests in TestHooksSetCommand, one a strict subset of the other. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-2-12",
    "source": "reviewer",
    "summary": "Two near-duplicate TMUX_PANE-unset subtests in TestHooksSetCommand, one a strict subset of the other.",
    "detail": "cmd/hooks_test.go:188-211 (returns error when TMUX_PANE is not set) and cmd/hooks_test.go:346-365 (it errors when TMUX_PANE is unset for set) share their entire fixture (hooksFileInTempDir, TMUX_PANE=\"\", mockKeyResolver{key: \"unus00\"}, hooks set --on-resume some-cmd) and their whole assertion set; the second asserts a strict subset — it omits the os.Stat(hooksFile) not-created check the first makes. Identical shape to F7, one function over. Both predate this work unit (git log -S dates them to resume-sessions-after-reboot-1-4 and session-rename-orphans-resume-hook T2-2), so pre-existing debt rather than phase-2 residue, and folding them reaches subtests task 2-12 does not own. Cheapest at the end-of-implementation pass; phase 4 already reworks this file.",
    "files": [
      "cmd/hooks_test.go"
    ]
  }
  ```

- assertHookFireCount reads the hook-fire file with no wait, racing the hydrate helper across the whole restore hook-fire family. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-3-2",
    "source": "reviewer",
    "summary": "assertHookFireCount reads the hook-fire file with no wait, racing the hydrate helper across the whole restore hook-fire family.",
    "detail": "internal/restore/rename_reboot_shared_test.go:90-100 reads the file immediately after rebootAndHydrate returns; that function last wait is WaitForSkeletonMarkersCleared, which clears BEFORE the helper execs sh -c <HOOK>; exec $SHELL. Reproduced at 1-in-5 iterations under -count=5 on the current tree (TestRenameRebootHook_PaneProcessKeptRunning) AND at HEAD (TestRenameRebootHook_ExternalRename), plus TestMultiPaneLegacy_PerPaneHookRouting and TestRenameRebootHook_DurableAcrossRepeatedReboots under ./internal/.... Pre-existing and not introduced by 3-2, but its new assertHookFireCount(t, hookFireFile, 2) inherits it, so the work unit headline guarantee now rides a racy read. Remedy is to poll to a deadline inside the shared helper — reaches four-plus tests owned by other tasks. The task other new assertions (capturedPaneToken after restore, readPaneToken after cycle 2) are deterministic and unaffected.",
    "files": [
      "internal/restore/rename_reboot_shared_test.go",
      "internal/restore/rename_reboot_durability_integration_test.go",
      "internal/restore/rename_reboot_hook_integration_test.go",
      "internal/restore/multipane_legacy_integration_test.go"
    ]
  }
  ```

- Unit-lane real-tmux restore tests silently depend on whichever portal is installed; root cause is a bare portal in the hydrate argv. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-3-3",
    "source": "reviewer",
    "summary": "Unit-lane real-tmux restore tests silently depend on whichever portal is installed; root cause is a bare portal in the hydrate argv.",
    "detail": "internal/restore/integration_test.go is unit-lane (no build tag) but runs a real restore against a real tmux server, so armed panes exec portal state hydrate from the tmux server PATH — the developer installed binary. Confirmed: at HEAD those tests pass against homebrew 0.11.0; on the 3-3 tree TestPhase3Integration_SaveRestoreRoundTrip (:31) and TestPhase3Integration_RestoreUsesLiveIndicesUnderBaseIndexDrift (:186) fail with expected alpha in list-sessions until a current binary is staged. Nothing in the change is wrong — the tests assert against the wrong binary. The reviewer view: not acceptable to leave, because the failure mode is silent and misattributing (a stale install reports as a restore regression), and it collides with CLAUDE.md own lane rule that any test exec-ing a built portal binary lives behind -tags integration. Root cause is one line: buildHydrateCommand bakes a bare portal (internal/restore/session.go:312) where internal/spawn deliberately uses os.Executable() for exactly this version-pinning reason (internal/spawn/command.go:12). Fixing it there closes the test skew AND the narrow production hazard of a shadowed portal on PATH together.",
    "files": [
      "internal/restore/session.go",
      "internal/restore/integration_test.go",
      "internal/spawn/command.go"
    ]
  }
  ```

- CLAUDE.md never describes the shape-aware reaper or internal/session new token surfaces, so the retain-old-format-forever rule is undocumented. — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-3-4",
    "source": "reviewer",
    "summary": "CLAUDE.md never describes the shape-aware reaper or internal/session new token surfaces, so the retain-old-format-forever rule is undocumented.",
    "detail": "Phase 1 made the sweep retain any key that is not token-shaped, and 3-1 added internal/session/panetoken.go (NewPaneToken) and internal/session/tokenshape.go (IsTokenShaped). CLAUDE.md only mention of any of this is NewPaneToken in passing at CLAUDE.md:166; IsTokenShaped, token-shaped and the retention rule appear nowhere, and the session row (CLAUDE.md:64) still describes the package as session-creation-only. The consequence is the dangerous one CLAUDE.md exists to prevent: an old-format entry sitting inert forever reads as cruft to a future agent, who deletes exactly the entries the spec 8.3 safety argument depends on. Out of scope for 3-4 (whose CLAUDE.md criteria are closed over the @portal-id passages, the state row and the Resume-hooks section) and unassigned — phase-1-tasks.md carries no CLAUDE.md acceptance criterion. Tasks 4-2 and 5-1 both already prescribe further edits to the same Resume-hooks paragraph, so this consolidates naturally with them.",
    "files": [
      "CLAUDE.md",
      "internal/session/tokenshape.go",
      "internal/session/panetoken.go",
      "internal/hooks/store.go"
    ]
  }
  ```

- Four integration tests exec portal state hydrate from the ambient PATH instead of staging a binary — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-3-5",
    "summary": "Four integration tests exec portal state hydrate from the ambient PATH instead of staging a binary",
    "detail": "internal/restore/integration_test.go (TestPhase3Integration_SaveRestoreRoundTrip, TestPhase3Integration_RestoreUsesLiveIndicesUnderBaseIndexDrift) and cmd/bootstrap/phase5_integration_test.go:106 / cmd/bootstrap/phase5_marker_suppression_integration_test.go:58 drive a real restore whose panes respawn into portal state hydrate, but never call restoretest.BuildPortalBinaryDir + PrependPATH. They pass or fail on whatever portal the developer has installed, and today fail with the misleading expected-alpha-in-list-sessions message. The fix is one shared prologue across four files in two packages, outside this task surface.",
    "files": [
      "internal/restore/integration_test.go",
      "cmd/bootstrap/phase5_integration_test.go",
      "cmd/bootstrap/phase5_marker_suppression_integration_test.go"
    ]
  }
  ```

- Session-level -t targets in the tmux client that bypass the package own exactness rule — residue — pre-existing; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-3-5",
    "pre_existing": true,
    "summary": "Session-level -t targets in the tmux client that bypass the package own exactness rule",
    "detail": "internal/tmux/tmux.go:419 states every session-level -t target must route through PaneTargetExact, and :412 repeats it, but seven sites pass a bare session name: :259 (display-message), :315 (set-option), :426 (ListPanesInSession), :483, :534, :555 (show-environment), :756 (set-environment). tmux prefix-matches session names, so list-panes -s -t foo resolves to a live foo-2 once foo is gone — the same class the exactTarget helper was introduced to close on the kill path. Pre-existing; no effect on the 3-5 test, whose server holds one user session.",
    "files": [
      "internal/tmux/tmux.go"
    ]
  }
  ```

- The two rename-reboot integration tests duplicate the whole capture-persist-reboot-hydrate bracket; one file names it in helpers, the other inlines it — residue — pre-existing; rides to the end-of-implementation analysis

  ```json
  {
    "source": "finder",
    "pre_existing": true,
    "summary": "The two rename-reboot integration tests duplicate the whole capture-persist-reboot-hydrate bracket; one file names it in helpers, the other inlines it",
    "detail": "internal/restore/rename_reboot_durability_integration_test.go:105-121 (captureAndPersist) and :123-153 (rebootAndHydrate) are re-implemented line for line inside runRenameRebootFire at internal/restore/rename_reboot_hook_integration_test.go:118-145 and :147-173, in the same package. The two also share a ~35-line setup preamble verbatim at rename_reboot_durability_integration_test.go:26-56 and rename_reboot_hook_integration_test.go:77-107. Phase 3 edited both sides (tasks 3-1, 3-2) but the duplication predates it in the same shape.",
    "files": [
      "internal/restore/rename_reboot_durability_integration_test.go",
      "internal/restore/rename_reboot_hook_integration_test.go",
      "internal/restore/rename_reboot_shared_test.go"
    ]
  }
  ```

- Four fixture helpers in restore_test are pairwise generalisations of one another — residue — pre-existing; rides to the end-of-implementation analysis

  ```json
  {
    "source": "finder",
    "pre_existing": true,
    "summary": "Four fixture helpers in restore_test are pairwise generalisations of one another",
    "detail": "seedScrollback(t, stateDir, name) (internal/restore/rename_reboot_shared_test.go:78-88) is exactly seedPaneScrollback(t, stateDir, name, 0, 0) (internal/restore/multipane_legacy_integration_test.go:213-223) — same body, same payload bytes. assertHookFireCount (rename_reboot_shared_test.go:90-100) hardcodes the marker HOOK_FIRED and parameterises the count; assertMarkerFiredOnce (multipane_legacy_integration_test.go:225-235) parameterises the marker and fixes the count at 1. One assertMarkerCount serves both. All four predate phase 3; consolidation task 3 subsumes the scrollback pair as a side effect.",
    "files": [
      "internal/restore/rename_reboot_shared_test.go",
      "internal/restore/multipane_legacy_integration_test.go"
    ]
  }
  ```

- TestLookupOnResume repeats a five-line seed preamble and the same three-assertion no-hook block across eleven subtests — residue — pre-existing; rides to the end-of-implementation analysis

  ```json
  {
    "source": "finder",
    "pre_existing": true,
    "summary": "TestLookupOnResume repeats a five-line seed preamble and the same three-assertion no-hook block across eleven subtests",
    "detail": "internal/hooks/lookup_test.go — every subtest opens with dir := t.TempDir() / filePath := filepath.Join(dir, hooks.json) / os.WriteFile / hooks.NewStore, and eight close with the identical err/ok/cmd triple (:17-27, :37-47, :58-68, :79-89, :100-110, :165-175, :187-197, :239-252). Eight predate phase 3; task 3-3 added three more in the established shape, so the subject is pre-existing.",
    "files": [
      "internal/hooks/lookup_test.go"
    ]
  }
  ```

- Four hand-rolled (component,msg) exactly-one-record sink filters remain across three packages; internal/logtest is the natural home — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-3-6",
    "summary": "Four hand-rolled (component,msg) exactly-one-record sink filters remain across three packages; internal/logtest is the natural home",
    "detail": "cmd/bootstrap/clean_sweep_summary_test.go:18 and internal/state/fifo_sweep_summary_test.go:19 are byte-for-byte the same summariesFor(comp,msg) + onlySummary(t,comp,msg) pair; cmd/state_daemon_cycle_summary_test.go:21 is the same body with capture/tick-complete hardcoded; cmd/state_hydrate_test.go:51 (task 3-6 survivor) is the same filter with hydrate hardcoded. internal/logtest already owns OnlyRecord and RecordsAtLevel (internal/logtest/capture.go:162,172), so RecordsWith(component,msg) / OnlyRecordWith(t,component,msg) would subsume all four. Crosses three packages and changes a shared test-helper API task 3-6 does not own.",
    "files": [
      "internal/logtest/capture.go",
      "cmd/state_hydrate_test.go",
      "cmd/state_daemon_cycle_summary_test.go",
      "cmd/bootstrap/clean_sweep_summary_test.go",
      "internal/state/fifo_sweep_summary_test.go"
    ]
  }
  ```

- The marker bracket set half is unobserved by every caller, not just internal/restore — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-3-8",
    "summary": "The marker bracket set half is unobserved by every caller, not just internal/restore",
    "detail": "Deleting client.SetServerOption(state.RestoringMarkerName, \"1\") from internal/restoretest/restore_marker.go:21-23 leaves go test ./internal/restore/ AND go test -tags integration ./internal/restore/ green, and leaves TestNonContiguousWindowReboot_KeepsTokenKeyedHooks green too. Only the unset is asserted (internal/restore/integration_test.go:80-82, integration_full_test.go:282-288). Pre-existing — the old restoreWithMarker had the same hole — but now that the bracket is one shared function, one assertion that the marker is set during Restore() would cover every caller at once.",
    "files": [
      "internal/restoretest/restore_marker.go",
      "internal/restore/integration_test.go",
      "internal/restore/integration_full_test.go",
      "cmd/noncontiguous_window_reboot_integration_test.go"
    ]
  }
  ```

- Two more copies of the scrollback seeder survive in cmd/bootstrap, in the one package that actually asserts on the payload — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-3-8",
    "summary": "Two more copies of the scrollback seeder survive in cmd/bootstrap, in the one package that actually asserts on the payload",
    "detail": "cmd/bootstrap/reboot_roundtrip_test.go:110-116 and :624-629 each re-author the same literal + SanitizePaneKey -> ScrollbackFile -> WriteFile (minus the MkdirAll), and verifyANSIScrollback (:418-436) is the assertion the deleted comments were reaching for. Folding these two into restoretest.SeedScrollback would finish the consolidation and give the shared const a caller that justifies its ANSI prefix. Out of task 3-8 named scope (a third package).",
    "files": [
      "cmd/bootstrap/reboot_roundtrip_test.go",
      "internal/restoretest/scrollback.go",
      "internal/restore/rename_reboot_shared_test.go"
    ]
  }
  ```

- readHookKey in internal/tmux is a third fixture pane-token read that must NOT be consolidated — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-3-9",
    "summary": "readHookKey in internal/tmux is a third fixture pane-token read that must NOT be consolidated",
    "detail": "internal/tmux/hookkey_format_realtmux_test.go:11-15 holds readHookKey, a fixture-level pane-token read via display-message + tmux.HookKeyFormat. It is correctly out of scope for the ReadPaneToken consolidation and must stay as it is: the format read IS its subject under test and is the production mechanism ResolveHookKey uses, so folding it into tmuxtest.ReadPaneToken would stop it testing anything. Recorded so a later consolidation sweep does not mistake it for a fourth copy.",
    "files": [
      "internal/tmux/hookkey_format_realtmux_test.go"
    ]
  }
  ```

- A third production bare -t issuer exists in the daemon capture loop, exposed to the rename class this workflow is about — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-3-10",
    "summary": "A third production bare -t issuer exists in the daemon capture loop, exposed to the rename class this workflow is about",
    "detail": "cmd/state_daemon.go:278 builds tmux.PaneTarget(sess.Name, win.Index, pane.Index) and passes it to state.CaptureAndHashPane -> Client.CapturePane -> capture-pane -e -p -S - -t <bare target> (internal/tmux/tmux.go:700). Session names come from state.CaptureStructure live enumeration in the same tick (cmd/state_daemon.go:249), so the target is known-live only as of that read. If a session is renamed mid-tick and a prefix-sibling exists, capture-pane -t foo:0.0 silently captures the OTHER session pane into the scrollback file keyed for the renamed one — the rename-class failure this workflow is about. Measured on tmux 3.7c: with foo killed and foo-2 live, set-option -p -t foo:0.0 exits 0 and writes to foo-2; the = form fails correctly. Narrow and pre-existing; task 3-10 forbids changing call sites. The restore sites are not exposed the same way: armPanes holds the session it just created.",
    "files": [
      "cmd/state_daemon.go",
      "internal/tmux/tmux.go"
    ]
  }
  ```

- project.Store.Remove still carries the identical defect task 4-1 just removed from hooks — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-4-1",
    "summary": "project.Store.Remove still carries the identical defect task 4-1 just removed from hooks",
    "detail": "internal/project/store.go:211-213 — its doc comment says it verbatim (\"It rewrites the file even when the path is absent, so the breadcrumb is emitted either way\"), so an absent path creates projects.json as {} and emits an INFO op=rm naming a removal that did not happen. Same class of falsehood as the hooks store, different store. Outside this work unit scope entirely, so it needs a scoping decision rather than a fix here.",
    "files": [
      "internal/project/store.go",
      "internal/tui/model.go"
    ]
  }
  ```

- cmd/hooks.go now carries four near-identical nil-check dependency builders — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-4-3",
    "summary": "cmd/hooks.go now carries four near-identical nil-check dependency builders",
    "detail": "buildHookKeyResolver (hooks.go:76), buildPaneHookLister (hooks.go:83), buildPaneStamper (hooks.go:90) and buildTokenMinter (hooks.go:97) are the same three-line if hooksDeps != nil && hooksDeps.X != nil shape. Task 4-3 added the fourth, crossing Rule of Three. A single generic picker would collapse them, but the other three serve the set/rm paths owned by sibling tasks, so the edit reaches past 4-3 surface.",
    "files": [
      "cmd/hooks.go"
    ]
  }
  ```

- The package one file reader lives in a suite file, and the shared staging home now depends on it across the boundary — folded into task 3

  ```json
  {
    "task": "resume-hooks-silently-lost-4-5",
    "summary": "The package one file reader lives in a suite file, and the shared staging home now depends on it across the boundary",
    "detail": "readFileBytes sits at cmd/bootstrap_production_test.go:130, beside bootstrap-specific fixtures (newTempHooksStore at :117, stubAllPaneLister). Task 4-5 correctly reused it rather than moving it — the Do-list said re-point, not relocate — but the result is that cmd/testhelpers_test.go:94 and :99 reach out of the staging home for the reader, while bootstrap_production_test.go carries no header claiming that role and run_hook_stale_cleanup_test.go consumes it 16 times. Moving readFileBytes into testhelpers_test.go finishes the consolidation 4-5 started. Related to but distinct from the 2-8 entry covering stubAllPaneLister siting in the same file.",
    "files": [
      "cmd/bootstrap_production_test.go",
      "cmd/testhelpers_test.go",
      "cmd/run_hook_stale_cleanup_test.go"
    ]
  }
  ```

- The hooks.json-unchanged assertion 4-5 consolidated is written nine more times, in three forms, one file over — folded into task 3

  ```json
  {
    "task": "resume-hooks-silently-lost-4-5",
    "summary": "The hooks.json-unchanged assertion 4-5 consolidated is written nine more times, in three forms, one file over",
    "detail": "cmd/run_hook_stale_cleanup_test.go asserts the same property inline at :43, :76, :110 (reflect.DeepEqual), :331 (bytes.Equal over a hand-rolled os.ReadFile pair at :309/:327) and :397, :419, :595, :624, :740 (bytes.Equal over readFileBytes). assertHooksFileUnchanged now names exactly this. TWO CAVEATS: each site carries a bespoke failure message (rewritten during a restore, rewritten on a failed marker read, ...) that a fixed-message helper would flatten, so the helper likely needs a message or context parameter; and the three reflect.DeepEqual sites are NOT equivalent to the six bytes.Equal ones — reflect.DeepEqual(nil, []byte{}) is false where bytes.Equal is true, so re-pointing them is a semantic change on the absent-vs-empty edge, not a mechanical swap.",
    "files": [
      "cmd/run_hook_stale_cleanup_test.go",
      "cmd/testhelpers_test.go"
    ]
  }
  ```

- internal/restore multipane-legacy hook-fire assertions read their marker file once, unpolled, so they flake under full-lane load — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-5-1",
    "summary": "internal/restore multipane-legacy hook-fire assertions read their marker file once, unpolled, so they flake under full-lane load",
    "detail": "assertMarkerFiredOnce / assertMarkerAbsent (internal/restore/multipane_legacy_integration_test.go:213, :224) do a bare os.ReadFile right after WaitForSkeletonMarkersCleared returns — the hooked shell redirect may not have flushed yet. Every caller in the file inherits it, which is why the already-banked TestMultiPaneLegacy_UnstampedNoHookLandsOnBareShell and a TestMultiPaneLegacy_PerPaneHookRouting blip share one root cause. The fix is a bounded poll in the shared helper, matching WaitForSkeletonMarkersCleared own shape, and it touches a suite outside task 5-1.",
    "files": [
      "internal/restore/multipane_legacy_integration_test.go"
    ]
  }
  ```

- internal/project carries the identical unlocked read-modify-write window task 5-1 just closed for hooks — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-5-1",
    "summary": "internal/project carries the identical unlocked read-modify-write window task 5-1 just closed for hooks",
    "detail": "internal/project/store.go:74 (Upsert), :159 (CleanStale), :189 (Rename), :213 (Remove) and internal/project/tags.go:38,65 (AddTag/RemoveTag) are all Load() -> mutate -> AtomicWrite, with no locking anywhere in the package. The concurrent writers are real and already wired: the daemon throttled stale-projects.json prune (cmd/state_daemon.go:227), portal doctor --fix (cmd/doctor.go:229), and the TUI edit modal immediate-persist path (internal/tui/model.go:1391,1432). Same lost-update class, same 10s daemon cadence, and internal/hooks/lock.go is now a drop-in pattern for it. Out of this work unit scope entirely. NOTE: this is the same store already banked for the unconditional-write Remove defect — one store, two distinct defects.",
    "files": [
      "internal/project/store.go",
      "internal/project/tags.go",
      "internal/hooks/lock.go"
    ]
  }
  ```

- The degraded read error attr renders the sidecar path twice on the now-routine ENOENT path — folded into task 4

  ```json
  {
    "task": "resume-hooks-silently-lost-5-2",
    "summary": "The degraded read error attr renders the sidecar path twice on the now-routine ENOENT path",
    "detail": "internal/hooks/lock.go:36 wraps with fmt.Errorf(\"open hooks lock %s: %w\", path, err) while the wrapped *os.PathError already carries the path, so a live probe emitted error=open hooks lock /.../hooks.json.lock: open /.../hooks.json.lock: no such file or directory. Harmless when it was a rare mutation failure (task 5-1 code, untouched by 5-2); it is now the shape every read on a pre-sidecar install logs. The fix belongs to acquireLock, which 5-1 owns and both acquire paths share.",
    "files": [
      "internal/hooks/lock.go"
    ]
  }
  ```

- The sweep load-failure WARN text still names a method the sweep no longer calls — folded into task 4

  ```json
  {
    "task": "resume-hooks-silently-lost-5-2",
    "summary": "The sweep load-failure WARN text still names a method the sweep no longer calls",
    "detail": "cmd/run_hook_stale_cleanup.go:101 warns \"stale-hook cleanup: hookStore.Load failed\" on the read that is now LoadSnapshot. The string is pinned by a test constant (loadWarnFmt, cmd/run_hook_stale_cleanup_test.go:25) and sits in the sweep spec-governed observability surface alongside the clean-stale-skipped reasons, so renaming it is an observability decision rather than a rename — better made in one pass with whatever else touches the sweep log strings.",
    "files": [
      "cmd/run_hook_stale_cleanup.go",
      "cmd/run_hook_stale_cleanup_test.go"
    ]
  }
  ```

- Two hook set drivers in package cmd, one a strict superset of the other — folded into task 3

  ```json
  {
    "task": "resume-hooks-silently-lost-5-3",
    "summary": "Two hook set drivers in package cmd, one a strict superset of the other",
    "detail": "runHookSetCapturing (cmd/hooks_write_lock_test.go:24) differs from runHookSet (cmd/testhelpers_test.go:28) only in returning the captured buffer — which is what runHookRm (cmd/testhelpers_test.go:39) already does. The consolidation is to give runHookSet the (string, error) signature and drop the twin, but that re-points nine call sites in files owned by sibling tasks (cmd/hooks_test.go:490,975, cmd/hooks_pane_token_test.go:43,78,99,124,147,171,230). testhelpers_test.go own header declares itself the home for driver helpers, so the new one is also sited away from its stated home.",
    "files": [
      "cmd/hooks_write_lock_test.go",
      "cmd/testhelpers_test.go",
      "cmd/hooks_test.go",
      "cmd/hooks_pane_token_test.go"
    ]
  }
  ```

- Three hook-key seam fakes in package cmd, the newest a stateful superset of the other two — folded into task 2

  ```json
  {
    "task": "resume-hooks-silently-lost-5-3",
    "summary": "Three hook-key seam fakes in package cmd, the newest a stateful superset of the other two",
    "detail": "mockKeyResolver (cmd/hooks_test.go:314, resolver only), recordingPaneStamper (cmd/hooks_pane_token_test.go:20, stamper only) and now stampedPane (cmd/hooks_write_lock_test.go:93, both, with the token actually round-tripping). cmd/hookkey_vocabulary_test.go:1-4 declares itself the home for the seam fakes that answer with the hook-key vocabulary, and none of the three live there. Same shape as the already-banked AllPaneLister-fakes item, so it consolidates cheaply alongside it.",
    "files": [
      "cmd/hooks_test.go",
      "cmd/hooks_pane_token_test.go",
      "cmd/hooks_write_lock_test.go",
      "cmd/hookkey_vocabulary_test.go"
    ]
  }
  ```

- Two near-identical AllPaneLister fakes in package cmd that should be one — folded into task 2

  ```json
  {
    "task": "resume-hooks-silently-lost-5-4",
    "summary": "Two near-identical AllPaneLister fakes in package cmd that should be one",
    "detail": "sideEffectPaneLister (cmd/hook_sweep_snapshot_order_test.go:21-35) and stubAllPaneLister (cmd/bootstrap_production_test.go:100-113) implement the same two-method seam and differ only by a during func() hook versus err/restoring/restoringErr fields. Adding during func() to stubAllPaneLister (invoked at the top of its ListAllPaneHookKeys) subsumes the new fake entirely and lets the new file delete it. The consolidation touches a fixture several sibling tasks in this phase drive.",
    "files": [
      "cmd/hook_sweep_snapshot_order_test.go",
      "cmd/bootstrap_production_test.go"
    ]
  }
  ```

- CleanStale emits its per-key clean-stale INFO lines before the save, so a failed save leaves INFO lines naming deletions that did not happen — residue — outside phase 5 surface or outside this work unit; rides to the end-of-implementation analysis

  ```json
  {
    "task": "resume-hooks-silently-lost-5-5",
    "summary": "CleanStale emits its per-key clean-stale INFO lines before the save, so a failed save leaves INFO lines naming deletions that did not happen",
    "detail": "internal/hooks/store.go:321 logs one INFO per key in removed, then :325 attempts the save and :326 emits the WARN summary carrying the error. At the production default level an operator greps hooks: and reads N deleted-X-command-was-Y lines followed by one WARN. Recoverable by reading the next line, but the per-key lines assert a deletion that did not occur — the same class of falsehood this work unit exists to remove. Task 5-5 made the failure classification explicit, which is what surfaces it; the fix reaches into a sibling task output.",
    "files": [
      "internal/hooks/store.go"
    ]
  }
  ```

- A third temp hooks-store seeder in package cmd, duplicating the existing one body — folded into task 3

  ```json
  {
    "task": "resume-hooks-silently-lost-5-5",
    "summary": "A third temp hooks-store seeder in package cmd, duplicating the existing one body",
    "detail": "cmd/hook_sweep_lock_timeout_test.go:211-231 (saveDeniedStore) repeats newTempHooksStore write-seed / create-sidecar / NewStore sequence (cmd/bootstrap_production_test.go:117-133) with only a caller-chosen directory and a chmod added. A newTempHooksStoreIn(dir, seed) would subsume both. Extends the existing bank entry Two temp hooks-store seeders in package cmd.",
    "files": [
      "cmd/hook_sweep_lock_timeout_test.go",
      "cmd/bootstrap_production_test.go"
    ]
  }
  ```

- The two-entry stale seed is now declared three times byte-identically, across two cmd test files — folded into task 2

  ```json
  {
    "task": "resume-hooks-silently-lost-5-5",
    "summary": "The two-entry stale seed is now declared three times byte-identically, across two cmd test files",
    "detail": "cmd/hook_sweep_lock_timeout_test.go:19-23 (lockedStaleSeed), cmd/run_hook_stale_cleanup_test.go:382-385 and :570-574 are the same {reapableSeedA: cmd-gone, liveSeedA: cmd-live} literal. Extends the existing 1-9 bank entry, which recorded two.",
    "files": [
      "cmd/hook_sweep_lock_timeout_test.go",
      "cmd/run_hook_stale_cleanup_test.go"
    ]
  }
  ```

