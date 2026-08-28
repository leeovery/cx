# Consolidation Findings: resume-hooks-silently-lost (Phase 2)

## Findings

### F1: The pane-token read format is composed twice in `internal/tmux`
- **Class**: duplication
- **Evidence**: the phase's central invariant is that registration and enumeration read the *same* option, and after two tasks the expression that does so exists as two independent literals 28 lines apart:
  - `internal/tmux/tmux.go:584` — `const HookKeyFormat = "#{" + state.PortalPaneIDOption + "}"` (task 2-2, the one-pane read used by `ResolveHookKey` at `:236`)
  - `internal/tmux/tmux.go:612-613` — `const paneHookRowFormat = "#{" + state.PortalPaneIDOption + "}" + paneHookRowSeparator + "#{session_name}:#{window_index}.#{pane_index}"` (task 2-1, the all-pane read used by `ListAllPaneHookKeys` at `:621`)
  - The two are held in agreement only by a real-tmux test — `internal/tmux/hookkey_cross_site_realtmux_test.go:13-53`, whose whole subject is "the two must answer with the same token for the same pane". Nothing in the source couples them; the shared `state.PortalPaneIDOption` constant guarantees only that both name the same option, not that both wrap it identically.
  - The location half at `:613` restates `StructuralKeyFormat` (`:579`) verbatim, 34 lines below it. The spec makes that non-reuse deliberate — the location is display-only, so rendering the same shape "couples nothing" to the positional siblings — but nothing at `:612` says so, and the two constants now sit close enough that a later reader will either couple them or delete one as a copy-paste.
- **Proposed shape**: `const paneHookRowFormat = HookKeyFormat + paneHookRowSeparator + "#{session_name}:#{window_index}.#{pane_index}"`. Identical resulting string, so no behaviour changes and no test re-points; the enumeration's token half is then the same constant registration reads, by construction rather than by a real-tmux assertion. Add one line above it recording that the location half deliberately does not reuse `StructuralKeyFormat` because the location is never a key — the warning the code cannot carry on its own.
- **Bank**: the `paneHookRowFormat` half of "Three hand-written token literals bypass the ReapableHookKey drift guard…" — the entry flags that the intentional restatement is unmarked and that the two constants sit ~40 lines apart.

### F2: Five server-bootstrap preambles and two stamp routes across the hook-key real-tmux suites
- **Class**: near-miss
- **Evidence**: every task in the phase added or rewrote a real-tmux fixture, and all five open with the same `tmuxtest.New → Client() → EnsureServer → NewSession → WaitForSession` block, diverging only in the topology seeded after it:
  - `internal/tmux/hookkey_realtmux_shared_test.go:16-36` — `seedThreePaneStampedSession` (session + split + new window + stamp); the file that exists to be the shared one
  - `internal/tmux/resolve_hookkey_realtmux_test.go:16-35` — `liveHookKeyPane` (one session, one pane)
  - `internal/tmux/hookkey_moved_pane_realtmux_test.go:139-172` — `newStampedPaneFixture` (three windows, one split, stamp) — task 2-5
  - `internal/tmux/hookkey_cross_site_realtmux_test.go:17-34` — inline, and the topology it builds (session + `SplitWindow(":0")` + `NewWindow`) is *exactly* `seedThreePaneStampedSession`'s, which is declared in the shared file this test's own suite owns
  - `internal/tmux/list_all_pane_hookkeys_realtmux_test.go:16-30` and `:49-55`, `internal/tmux/hookkey_format_realtmux_test.go:22-29` and `:45-52` — inline two-pane and one-pane variants
  - Divergences that are drift rather than intent: `hookkey_format_realtmux_test.go` alone never calls `EnsureServer`; the socket prefix is `ptl-xsite-` / `ptl-hookkeys-` / `ptl-hookkey-` / `ptl-panemove-` / `hookkey-` (the last dropping the shared `ptl-` prefix).
  - **Two ways to stamp a pane, with no stated rule.** Raw tmux: `stampPaneToken` (`internal/tmux/list_all_pane_hookkeys_realtmux_test.go:73-76`, consumed there and from `hookkey_cross_site_realtmux_test.go:36-37`). Through the client: `client.SetPaneOption(...)` at `internal/tmux/hookkey_realtmux_shared_test.go:32`, `hookkey_format_realtmux_test.go:31`, `resolve_hookkey_realtmux_test.go:78`, `hookkey_moved_pane_realtmux_test.go:166`. The split does not track "avoid the SUT in setup": the enumeration test uses raw while the *format* test — which is equally not testing `SetPaneOption` — uses the client. `cmd` adds three more raw copies of the same argv (`cmd/state_daemon_hook_cleanup_integration_test.go:86`, `cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:101`, `cmd/rename_restore_cleanup_survival_integration_test.go:40`), all against a `*tmuxtest.Socket`.
- **Proposed shape**: two moves, both pure test-scaffolding relocation with no assertion touched.
  1. One parameterised seeder in `internal/tmux/hookkey_realtmux_shared_test.go` — the common preamble plus a topology argument — that `seedThreePaneStampedSession`, `liveHookKeyPane`, `newStampedPaneFixture` and the four inline blocks all reach. Each caller keeps its own assertions and its own token; `newStampedPaneFixture` keeps its `renumber-windows on` line, which is genuinely its own.
  2. Promote the stamp to `internal/tmuxtest` as a `(*Socket).StampPaneToken(t, target, token)` raw-tmux method (`tmuxtest` already imports `internal/tmux`, which now imports `internal/state`, so the option constant is reachable with no new cycle) and re-point all eight sites — the five in `internal/tmux` and the three in `cmd` — so "stamp a pane token in a test" is spelled once and no suite sets its subject up through its subject. `resolve_hookkey_realtmux_test.go:111`'s raw `set-option` stays as it is: that one is asserting tmux's own exit status, not staging a fixture.
- **Bank**: "Three near-miss real-tmux fixture builders across the hook-key suites share an identical server-bootstrap preamble." — confirmed and extended: the entry named three builders plus `stampPaneToken`; the four inline preambles and the three `cmd` stamp copies are the rest of the set.

### F3: The phase's new row vocabulary is sited away from the seed vocabulary, and bypassed at five sites
- **Class**: drift
- **Evidence**: task 1-9 deliberately gathered the `cmd` hook-key test vocabulary into `cmd/run_hook_stale_cleanup_test.go:25-70` — `reapableSeedA..D` / `unjudgeableSeedA..B` / `liveSeedA..C`, `restoringOption`, `recordingHookKeyLister`. Phase 2's own additions reopened the scatter:
  - `tokenRows` (`cmd/bootstrap_production_test.go:115-121`) and `unstampedRows` (`:123-129`) — task 2-1's `[]tmux.PaneHookRow` builders, the vocabulary every re-pointed assertion in the phase now composes with `liveSeedA..C`. They are used **once** in their own file and **43 times elsewhere**: `cmd/run_hook_stale_cleanup_test.go` (23 `tokenRows`, 4 `unstampedRows`), `cmd/doctor_test.go` (17 + 2), `cmd/doctor_summary_test.go:20,173`, `cmd/doctor_fix_theme_test.go:106`. A reader tracing what `tokenRows(liveSeedB)` means crosses from the assertion, to the seed block in one file, to the builder in a third that has nothing to do with the sweep.
  - Three hand-written token literals bypass the drift guard in files that already import `transienttest`: `liveHookToken = "livetk"` (`cmd/state_daemon_hook_cleanup_integration_test.go:45`, one line below `transienttest.ReapableHookKey(0)`), `liveKey = "livetk"` (`cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:100`) and `renameRestoreToken = "tokrst"` (`cmd/rename_restore_cleanup_survival_integration_test.go:18`). All three are now stamped into real panes as `state.PortalPaneIDOption` values, so each genuinely measures liveness *only while it stays token-shaped* — and `transienttest.ReapableHookKey` exists precisely to panic when a width or alphabet change would silently turn such a fixture vacuous (`internal/transienttest/hooks.go:76-93`).
  - `"live:0.0"` at `cmd/run_hook_stale_cleanup_test.go:788` and `:865` — hand-written, named *live*, and surviving purely on unjudgeability. At `:786-789` it sits inside the same seed literal as `unjudgeableSeedA`, one composed through the vocabulary and one not. Under the phase's own change, no live pane can ever carry a `<name>:w.p` key, so the name now asserts the opposite of the reason the entry survives.
- **Proposed shape**: move `tokenRows` / `unstampedRows` to `cmd/run_hook_stale_cleanup_test.go` beside the seed vocabulary they are always used with (or move the whole block to a shared `cmd` test-helper file, taking `restoringOption` with it); re-point the three integration literals to `transienttest.ReapableHookKey(n)`; re-point the two `"live:0.0"` literals to `unjudgeableSeedB`. Values stay token-shaped and legacy-shaped respectively, so every assertion keeps its current meaning — this is naming and siting only.
- **Bank**:
  - "Three hand-written token literals bypass the ReapableHookKey drift guard in files that already import it." (the `paneHookRowFormat` half of the same entry lands in F1)
  - the recurrence the 1-8 entry warned about: the shared vocabulary block travelling with its consumers rather than being sited on its own

### F4: `hook set`'s test setup is encapsulated in one file and open-coded 33 times in its sibling
- **Class**: duplication
- **Evidence**: task 2-2 wrote the helpers; task 2-4, two commits later, added eight more subtests that open-code them.
  - `hooksFileInTempDir(t)` (`cmd/hooks_pane_token_test.go:37-42`) and `runHookSet(t, command)` (`:44-51`) — 8 and 7 uses respectively, all inside their own file.
  - `cmd/hooks_test.go` spells `dir := t.TempDir(); hooksFile := filepath.Join(dir, "hooks.json"); t.Setenv("PORTAL_HOOKS_FILE", hooksFile)` at 33 sites and `resetRootCmd(); SetOut; SetErr; SetArgs; Execute` at 30. Eight of the file-path sites are **new in this phase** — `cmd/hooks_test.go:892, 908, 933, 962, 981, 998, 1027, 1049`, all in task 2-4's `TestHooksSetTouchesSaveRequested`, all written while the helper existed in the sibling file.
  - The cross-file reach already happened in the other direction: `runHookSetForKey` (`cmd/hooks_test.go:825-834`) calls task 2-2's `runHookSet` across the file boundary, so the two files are already coupled through a helper sited in the feature-named one.
  - Phase 4 reworks `cmd/hooks_test.go` for `hook rm` wording and the `hook list` location column, so the count grows again before anything shrinks.
- **Proposed shape**: move both helpers to `cmd/testhelpers_test.go` beside `writeHooksJSON` / `readHooksJSON` (`:9`, `:20`), where all three hooks test files already reach. Return the temp dir alongside the file path — three of the phase's own new sites (`:908` stages a blocker file in it, `:933` a locked state dir, `:998` a directory at the hooks path) need `dir` for their own purposes and cannot use a path-only helper. Re-point the 33 sites. Mechanical; no subtest gains or loses an assertion.
- **Bank**: "The hooks CLI test setup is now duplicated 28 ways beside two helpers that encapsulate it."

### F5: Two near-parallel hooks-component record assertions, and the same level filter written twice
- **Class**: near-miss
- **Evidence**: two tasks of this phase each needed "assert one `hooks`-component record" and each wrote it:
  - `standDownRecord` (`cmd/run_hook_stale_cleanup_test.go:598-627`, task 2-1) — asserts level DEBUG, `Msg == "clean-stale-skipped"`, `component == "hooks"`, `op`, `via == "internal"`, `reason`
  - `assertTouchWarn` (`cmd/hooks_test.go:857-887`, task 2-4) — asserts level WARN, `Msg == "touch-save-requested"`, `component == "hooks"`, `op`, `via == "cli"`, a non-empty `error`
  - Five of each helper's assertions are the same four-attr shape under a different expected level, op and via.
  - The level filter is also written twice: `warnRecords` (`cmd/hooks_test.go:836-844`) and the inline `for … if r.Level >= slog.LevelWarn` loop at `cmd/run_hook_stale_cleanup_test.go:600-604`. `internal/logtest` owns every other typed accessor (`OnlyRecord`, `AttrString`, `IntAttr`, `HasAttr`, `RequireDuration` — `internal/logtest/capture.go:39-167`) and has no level filter, so both copies exist because the one place that should hold it does not.
- **Proposed shape**: add `func (s *Sink) RecordsAtLevel(min slog.Level) []Record` to `internal/logtest/capture.go` beside `Records`/`OnlyRecord`, and have both `cmd` sites call it. In `cmd`, one `assertHooksRecord(t, rec, wantLevel, wantOp, wantVia)` carrying the level/message/component/op/via block, called from both helpers, each keeping its own extra assertion (`reason` for the stand-down, non-empty `error` for the touch). Pure extraction; every assertion that runs today still runs.
- **Bank**:
  - "A level-filtering accessor belongs in logtest beside OnlyRecord."
  - "Two near-parallel hooks-component log-record assertion helpers in cmd tests."

### F6: `cmd/hooks.go` reaches its three `*Deps` seams two different ways
- **Class**: drift
- **Evidence**: task 2-2 added two seams to `HooksDeps` (`cmd/hooks.go:31-35`) and gave each a named accessor, leaving the pre-existing third as an inline branch — so one file now holds both shapes of one pattern:
  - `cmd/hooks.go:58-63` — `var keyResolver HookKeyResolver; if hooksDeps != nil && hooksDeps.KeyResolver != nil { … } else { … }`, inline inside `resolveCurrentPaneKey`
  - `cmd/hooks.go:73-78` — `hooksPaneStamper()`
  - `cmd/hooks.go:80-85` — `hooksTokenMinter()`
  - The package convention is the named accessor, and it is `buildX`: `buildHooksTmuxClient` (`cmd/hooks.go:46`), `buildAckWriter` (`cmd/open.go:268-274`), `buildThemeLoader` (`cmd/open.go:489-494`). The two new accessors follow the convention's shape but not its name.
- **Proposed shape**: extract `buildHookKeyResolver() HookKeyResolver` holding the inline branch, and rename the two new accessors to `buildPaneStamper` / `buildTokenMinter`. `resolveCurrentPaneKey` loses six lines and its remaining body is the two reads it is named for. Unexported, no seam semantics change, no test re-points (tests set `hooksDeps` fields, never call the accessors).

### F7: The new unresolvable-pane subtest re-covers a pre-existing one
- **Class**: duplication
- **Evidence**: `cmd/hooks_pane_token_test.go:236-262` ("it exits non-zero from hook set on an unresolvable pane", task 2-3) and `cmd/hooks_test.go:315-340` ("it aborts hooks set when the hook-key read fails") share their whole fixture — hooks file in a temp dir, `TMUX_PANE` set, `mockKeyResolver{err: …}`, `hook set --on-resume` — and both assert a non-nil error and that `hooks.json` was never created. The only divergence is which property of the error each checks: the pre-existing one `strings.Contains(err.Error(), "resolve")`, the new one `errors.As(&tmux.CommandError{})` plus tmux's own words surviving in the message.
- **Proposed shape**: one subtest carrying all four assertions, in `cmd/hooks_test.go` beside the other `hook set` failure cases. No property currently asserted is dropped — the union is strictly stronger than either. (Phase 4 reworks `hook rm` messaging in the same file but does not reach this `hook set` pair, so it will not absorb this on its own.)
- **Bank**: "The new cmd propagation subtest substantially re-covers a pre-existing one; consolidation reaches a file this task did not touch."

## Bank Verdicts

- Pre-existing repo-wide lint/format debt outside this task's surface — **residue — still wholly pre-existing. `gofmt -l .` at this boundary reports exactly `internal/tui/help_modal_test.go`, which no phase-2 commit touches, and the named `modernize` sites (`cmd/root.go`, `cmd/bootstrap_progress.go`, `main.go`) are not phase files. Re-recorded under Pre-existing Debt.**
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

- runHookStaleCleanup now takes 5 parameters, one over the project convention — **residue — plan-authored and unchanged by this phase. The signature still stands at five (`cmd/run_hook_stale_cleanup.go:66-72`), phase 3's task text pins the five-argument call form and phase 5 reorders the body while keeping it. Two of the files the entry lists no longer exist (`cmd/hookkey_no_regression_upgrade_test.go` deleted by task 2-1; the `hook_sweep_*` files merged away by task 1-9), so the call-site count it cites is stale. Carried for the end-of-implementation pass.**
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

- Three parallel fakes for the one `AllPaneLister` seam in package `cmd` — **residue — still three, and the three-fakes situation is still pre-existing. Phase 2 re-pointed all three identically (`rows []tmux.PaneHookRow` + the same one-line method: `cmd/doctor_test.go:798-805`, `cmd/bootstrap_production_test.go:101-110`, `cmd/run_hook_stale_cleanup_test.go:56-67`), which deepens the debt without causing it; phase 3 re-points them again. Re-recorded under Pre-existing Debt.**
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

- Two temp hooks-store seeders in package `cmd`, neither derived from the other — **residue — both survive and both are pre-existing: `newTempHooksStore` (`cmd/bootstrap_production_test.go:137`, 38 uses) and `seedHooksJSON` (`cmd/doctor_test.go:810`, 30 uses). Phase 2 consumed both again without changing either. Re-recorded under Pre-existing Debt.**
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

- The restore-window rule is single-sourced in cmd, but the daemon states the inverse error policy twice — **residue — ground outside this phase. `cmd/state_daemon.go` is not a phase-2 file and neither reader changed; the entry's own note that consolidating touches daemon lifecycle code owned by other phases still holds. Carried for the end-of-implementation pass.**
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

- A third reader of the same failed-read-counts-as-set posture sits in cmd — **residue — same ground. `cmd/state_commit_now.go` and `cmd/state_daemon.go` are outside the phase's surface and unchanged; `restoreWindowActive` (`cmd/run_hook_stale_cleanup.go:60-63`) is still the one named reader of four. Carried for the end-of-implementation pass.**
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

- The shared cmd hook-key vocabulary landed in the file F4 already flags as the fixture-scatter site — **mooted — task 1-9's merge moved it. The `reapableSeedA..D` / `unjudgeableSeedA..B` block now sits at `cmd/run_hook_stale_cleanup_test.go:25-40`, beside `restoringOption` (`:46`) and `recordingHookKeyLister` (`:56`), which is exactly the destination the entry named; `cmd/doctor_test.go` holds no vocabulary block. The four `hook_*` files it expected the merge to consume no longer exist. Phase 2 opened a fresh instance of the same class — see F3.**
  ```json
  {
    "task": "resume-hooks-silently-lost-1-8",
    "source": "reviewer",
    "summary": "The shared cmd hook-key vocabulary landed in the file F4 already flags as the fixture-scatter site.",
    "detail": "The reapableSeedA..D / unjudgeableSeedA..B block sits at cmd/doctor_test.go:812-823 but is consumed from ten cmd test files, most of which exercise cmd/run_hook_stale_cleanup.go and cmd/state_daemon.go rather than cmd/doctor.go — e.g. cmd/run_hook_stale_cleanup_test.go:43, cmd/state_daemon_run_test.go:531, cmd/hook_retention_shape_test.go:15, cmd/hook_sweep_restore_standdown_test.go:24. It joins staleDeps / seedHooksJSON / fakeHookLister / runDoctorFixCmd, which finding F4 already banks for a file-merge pass. The vocabulary block should travel with that merge (to cmd/run_hook_stale_cleanup_test.go or a source-derived doctor_stale_hooks_test.go) rather than be moved on its own.",
    "files": [
      "cmd/doctor_test.go",
      "cmd/run_hook_stale_cleanup_test.go",
      "cmd/hook_prune_output_test.go",
      "cmd/hook_retention_shape_test.go",
      "cmd/hook_sweep_restore_standdown_test.go",
      "cmd/hook_sweep_standdown_report_test.go",
      "cmd/state_daemon_run_test.go",
      "cmd/state_daemon_hook_cleanup_test.go"
    ]
  }
  ```

- TestDoctorFixPrunedHookOutput is now fully subsumed by a sibling test — **residue — still real and still phase-1-caused. The test survives at `cmd/doctor_test.go:1665`, still asserting a strict subset of its sibling through the same shared helper; phase 2 neither caused nor changed it, and deleting it costs a case, which needs an owner who can approve the count change. Recorded under Pre-existing Debt.**
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

- Four further cmd test files touching doctor.go / run_hook_stale_cleanup.go still carry concern-derived names — **residue — partially resolved, remainder pre-existing. `cmd/hookkey_no_regression_upgrade_test.go` was deleted whole by task 2-1. The other three (`cmd/hooks_cleanstale_single_caller_guard_test.go`, `cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go`, `cmd/rename_restore_cleanup_survival_integration_test.go`) survive under names that predate the work unit; phase 2 edited two of them but did not create the naming. Recorded under Pre-existing Debt.**
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

- staleSeed is now declared twice, byte-identical, in one file — **residue — still true, cause still phase 1. The two declarations sit at `cmd/run_hook_stale_cleanup_test.go:434-438` and `:632-636`, byte-identical; phase 2 re-pointed both to `liveSeedA` in lockstep, which maintained the duplication rather than causing it. Recorded under Pre-existing Debt.**
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

- The removal leaves two byte-identical subtests in TestDoctorStaleHooksCheck — **residue — still byte-identical and still phase-1-caused. `cmd/doctor_test.go:1116-1122` and `:1140-1146` are now character-for-character identical again after task 2-1 re-pointed both listers to `tokenRows(liveSeedB)` in lockstep. The sibling at `:1132-1138` remains genuinely distinct. Phase 2 maintained the pair rather than creating it. Recorded under Pre-existing Debt.**
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

- The two real-tmux enumeration tests now overlap and want merging once 2-2 restores the cross-site comparison — **mooted — task 2-2 restored the comparison and the overlap resolved itself into the end state the entry predicted. `internal/tmux/hookkey_cross_site_realtmux_test.go:13-53` now compares `ResolveHookKey` against the enumeration row-by-row across three panes, and `internal/tmux/list_all_pane_hookkeys_realtmux_test.go:13-45` pins the mixed stamped/unstamped population in one read — one genuine cross-site test plus one focused enumeration test, and the file name matches its single subject. The shared-preamble half of the entry's concern survives and is folded into F2.**
  ```json
  {
    "task": "resume-hooks-silently-lost-2-1",
    "source": "reviewer",
    "summary": "The two real-tmux enumeration tests now overlap and want merging once 2-2 restores the cross-site comparison.",
    "detail": "internal/tmux/hookkey_cross_site_realtmux_test.go:13 (TestPaneTokenEnumeration_PerPaneTokensAreDistinct) and internal/tmux/list_all_pane_hookkeys_realtmux_test.go:13 (TestListAllPaneHookKeys_StampedAndUnstampedInOneRead) each spin a real server, stamp panes and assert the stamped/unstamped token split; the former is the narrowed remnant of a cross-site test that 2-2 re-points at ResolveHookKey versus the enumeration. Consolidating now would collide with that task; consolidating after it would leave one focused enumeration test plus one genuine cross-site test. The file name also no longer matches its single test subject.",
    "files": [
      "internal/tmux/hookkey_cross_site_realtmux_test.go",
      "internal/tmux/list_all_pane_hookkeys_realtmux_test.go"
    ]
  }
  ```

- Three hand-written token literals bypass the ReapableHookKey drift guard in files that already import it — **confirmed → F3 (the literals and the `"live:0.0"` naming) and → F1 (the `paneHookRowFormat` half). All three literals are unchanged and all three are now stamped into real panes as `@portal-pane-id` values, so the drift exposure is live.**
  ```json
  {
    "task": "resume-hooks-silently-lost-2-1",
    "source": "reviewer",
    "summary": "Three hand-written token literals bypass the ReapableHookKey drift guard in files that already import it.",
    "detail": "liveHookToken = \"livetk\" (cmd/state_daemon_hook_cleanup_integration_test.go:45, one line above staleHookKey = transienttest.ReapableHookKey(0)), liveKey = \"livetk\" (cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:101) and renameRestoreToken = \"tokrst\" (cmd/rename_restore_cleanup_survival_integration_test.go:42). All three are token-shaped today and all three genuinely measure liveness, but they bypass the panic-on-drift the helper exists for; a suffixLen/alphabet change would turn each into a vacuous pass. Also: \"live:0.0\" at cmd/run_hook_stale_cleanup_test.go:788 and :865 is named live while surviving purely on unjudgeability — unjudgeableSeedB would read straight. And paneHookRowFormat intentionally restates the location shape rather than reusing StructuralKeyFormat (spec 3.3), but nothing at internal/tmux/tmux.go:594 says so and the two constants sit ~40 lines apart — a plausible future simplification target.",
    "files": [
      "cmd/state_daemon_hook_cleanup_integration_test.go",
      "cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go",
      "cmd/rename_restore_cleanup_survival_integration_test.go",
      "cmd/run_hook_stale_cleanup_test.go",
      "internal/tmux/tmux.go"
    ]
  }
  ```

- The hooks CLI test setup is now duplicated 28 ways beside two helpers that encapsulate it — **confirmed → F4. The count grew rather than shrank: task 2-4 added eight further open-coded sites after task 2-2 landed the helpers, so 33 file-path sites and 30 command-drive sites now sit beside two helpers used only inside their own file.**
  ```json
  {
    "task": "resume-hooks-silently-lost-2-2",
    "source": "reviewer",
    "summary": "The hooks CLI test setup is now duplicated 28 ways beside two helpers that encapsulate it.",
    "detail": "cmd/hooks_pane_token_test.go:34-48 adds hooksFileInTempDir and runHookSet, which capture exactly the shape cmd/hooks_test.go open-codes 28 times (filepath.Join(t.TempDir(), \"hooks.json\") + t.Setenv(\"PORTAL_HOOKS_FILE\", ...)) and 29 times (resetRootCmd() + SetOut/SetErr/SetArgs). The consolidation reaches into subtests this task had no reason to touch, so it belongs at the phase boundary — and the helpers likely belong beside writeHooksJSON/readHooksJSON in cmd/testhelpers_test.go rather than in a feature-specific file. Phase 2 adds more hook set / hook rm cases in 2-3 and 4-2, so the count grows before it shrinks.",
    "files": [
      "cmd/hooks_pane_token_test.go",
      "cmd/hooks_test.go",
      "cmd/testhelpers_test.go"
    ]
  }
  ```

- The gone-pane message reaching the user is three wraps deep, and Phase 4 reworks the same call site — **residue — a later phase's own rewrite owns it, and trimming a wrap is not a pure refactor. Phase 4 fixes the exact removal wording at this same `resolveCurrentPaneKey` call site, and the message the entry wants shortened is user-visible text the spec pins (tmux's words survive unaltered at the tail), so changing it is a behaviour change, not consolidation.**
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

- The new cmd propagation subtest substantially re-covers a pre-existing one — **confirmed → F7. Both subtests stand (`cmd/hooks_pane_token_test.go:236-262` and `cmd/hooks_test.go:315-340`) with identical fixtures and identical error-plus-no-file assertions. Phase 4's rework of `cmd/hooks_test.go` is on the `hook rm` side and will not reach this `hook set` pair, so it does not absorb this on its own.**
  ```json
  {
    "task": "resume-hooks-silently-lost-2-3",
    "source": "reviewer",
    "summary": "The new cmd propagation subtest substantially re-covers a pre-existing one; consolidation reaches a file this task did not touch.",
    "detail": "cmd/hooks_pane_token_test.go:238-262 (it exits non-zero from hook set on an unresolvable pane) and cmd/hooks_test.go:310-334 (it aborts hooks set when the hook-key read fails) share their whole setup and their error-plus-no-file assertions; the new one adds only the errors.As and tmux-words checks. Phase 4 is already reworking hook rm messaging in cmd/hooks_test.go, so folding the two is cheapest there.",
    "files": [
      "cmd/hooks_pane_token_test.go",
      "cmd/hooks_test.go"
    ]
  }
  ```

- A level-filtering accessor belongs in logtest beside OnlyRecord — **confirmed → F5. `warnRecords` (`cmd/hooks_test.go:836-844`) and the inline scan in `standDownRecord` (`cmd/run_hook_stale_cleanup_test.go:600-604`) both stand, and `internal/logtest/capture.go` still has no level filter among its accessors.**
  ```json
  {
    "task": "resume-hooks-silently-lost-2-4",
    "source": "reviewer",
    "summary": "A level-filtering accessor belongs in logtest beside OnlyRecord.",
    "detail": "warnRecords (cmd/hooks_test.go:839) and the inline `if r.Level >= slog.LevelWarn` scan in standDownRecord (cmd/run_hook_stale_cleanup_test.go:598-605) are the same filter written twice, in two files owned by different tasks of this phase. internal/logtest/capture.go already owns the typed accessors (OnlyRecord, AttrString, HasAttr) and has no level filter; a Sink.RecordsAtLevel(min slog.Level) []Record would serve both and any third consumer.",
    "files": [
      "internal/logtest/capture.go",
      "cmd/hooks_test.go",
      "cmd/run_hook_stale_cleanup_test.go"
    ]
  }
  ```

- Two near-parallel hooks-component log-record assertion helpers in cmd tests — **confirmed → F5. `assertTouchWarn` (`cmd/hooks_test.go:857-887`) and `standDownRecord` (`cmd/run_hook_stale_cleanup_test.go:598-627`) both stand, differing only in expected level, message, op and via across five otherwise identical assertions.**
  ```json
  {
    "task": "resume-hooks-silently-lost-2-4",
    "source": "reviewer",
    "summary": "Two near-parallel hooks-component log-record assertion helpers in cmd tests.",
    "detail": "assertTouchWarn (cmd/hooks_test.go:857) and standDownRecord (cmd/run_hook_stale_cleanup_test.go:598, landed by task 2-1) both assert the same record shape — level, message, component=hooks, op, via — differing only in expected level and op. Spec 6.5 adds a third op (load-unlocked) still to come, which will want the same assertions again. One assertHooksRecord(t, rec, wantLevel, wantOp, wantVia) would carry all three. Consolidation edits another task output, so it was not an issue at review time.",
    "files": [
      "cmd/hooks_test.go",
      "cmd/run_hook_stale_cleanup_test.go"
    ]
  }
  ```

- Three near-miss real-tmux fixture builders across the hook-key suites share an identical server-bootstrap preamble — **confirmed → F2, and extended: the three named builders plus `stampPaneToken` are joined by four inline copies of the same preamble (`hookkey_cross_site_realtmux_test.go:17-34`, `list_all_pane_hookkeys_realtmux_test.go:16-30` and `:49-55`, `hookkey_format_realtmux_test.go:22-29` and `:45-52`) and three further raw stamp copies in `cmd` integration tests. The token-literal width note is folded into F3's re-point.**
  ```json
  {
    "task": "resume-hooks-silently-lost-2-5",
    "source": "reviewer",
    "summary": "Three near-miss real-tmux fixture builders across the hook-key suites share an identical server-bootstrap preamble.",
    "detail": "newStampedPaneFixture (internal/tmux/hookkey_moved_pane_realtmux_test.go:139), liveHookKeyPane (internal/tmux/resolve_hookkey_realtmux_test.go:16) and seedThreePaneStampedSession (internal/tmux/hookkey_realtmux_shared_test.go:16) each open with the same tmuxtest.New -> Client() -> EnsureServer -> NewSession -> WaitForSession block and diverge only in the seeded topology. stampPaneToken (internal/tmux/list_all_pane_hookkeys_realtmux_test.go:73) is a raw-tmux twin of client.SetPaneOption used elsewhere in the same suites. A single parameterised seeder in the shared file would collapse all four; the fix reaches into sibling tasks files. Also minor: token literals in the moved-pane suite are tokMove/tokRespawn/tokSplit (7-10 chars), not token-shaped under the 6-char rule the sibling suite uses (tokMix) — irrelevant at the tmux layer but non-uniform.",
    "files": [
      "internal/tmux/hookkey_moved_pane_realtmux_test.go",
      "internal/tmux/resolve_hookkey_realtmux_test.go",
      "internal/tmux/hookkey_realtmux_shared_test.go",
      "internal/tmux/list_all_pane_hookkeys_realtmux_test.go"
    ]
  }
  ```

## Pre-existing Debt

- Repo-wide format and modernize debt untouched by this work unit
  DETAIL: `gofmt -l .` on a clean tree at this boundary reports exactly `internal/tui/help_modal_test.go`, which no phase-2 commit touches. The `modernize` findings named at the phase-1 boundary (`cmd/root.go:199`, `cmd/bootstrap_progress.go:133`, `main.go:64`, `main.go:75`) are all outside the phase surface. One sweep clears the set.
  FILES: internal/tui/help_modal_test.go, cmd/root.go, cmd/bootstrap_progress.go, main.go

- Three parallel fakes for the one `AllPaneLister` seam in package `cmd`
  DETAIL: `fakeHookLister` (`cmd/doctor_test.go:798-808`, value receiver), `stubAllPaneLister` (`cmd/bootstrap_production_test.go:101-113`) and `recordingHookKeyLister` (`cmd/run_hook_stale_cleanup_test.go:56-70`, a strict superset adding `hookKeyCalls`) all predate the work unit. Phase 2 re-pointed all three identically — same `rows []tmux.PaneHookRow` field, same one-line `ListAllPaneHookKeys` body, same `restoringOption` delegation — and phase 3 re-points them again. A merge is an end-of-implementation move.
  FILES: cmd/doctor_test.go, cmd/bootstrap_production_test.go, cmd/run_hook_stale_cleanup_test.go

- Two temp hooks-store seeders in package `cmd`, neither derived from the other
  DETAIL: `newTempHooksStore(t, rawJSON)` (`cmd/bootstrap_production_test.go:137`, 38 call sites) and `seedHooksJSON(t, keys...)` (`cmd/doctor_test.go:810`, 30 call sites) both write a `hooks.json` into a `t.TempDir()` and return `(*hooks.Store, path)`. Both predate the work unit; this phase consumed both heavily without changing either. A future key-shape change re-points each independently.
  FILES: cmd/bootstrap_production_test.go, cmd/doctor_test.go

- Two byte-identical subtests and two byte-identical seed declarations left by phase 1's merges
  DETAIL: `cmd/doctor_test.go:1116-1122` ("it reports not evaluable while the restore marker is set") and `:1140-1146` ("it reads the marker before counting") are character-for-character identical; phase 2 re-pointed both listers to `tokenRows(liveSeedB)` in lockstep, restoring the identity rather than causing it. Separately `staleKey` / `staleSeed` is declared twice byte-identically in one file at `cmd/run_hook_stale_cleanup_test.go:434-438` and `:632-636`. Deleting the duplicate subtest costs a named case, so it needs an owner who can approve the count change; the seed duplicate closes with one package-level declaration beside the vocabulary block at `:25-40`.
  FILES: cmd/doctor_test.go, cmd/run_hook_stale_cleanup_test.go

- `TestDoctorFixPrunedHookOutput` is subsumed by its sibling
  DETAIL: `cmd/doctor_test.go:1665` ("it leaves doctor --fix stdout unchanged") drives the same fixture through the same shared assertion helper as `TestDoctorFixPrunesStaleEntriesThenRediagnosesClean` and adds no assertion of its own. Phase-1-caused and untouched by phase 2; removing it loses a named case.
  FILES: cmd/doctor_test.go

- Three `cmd` test files still named after their concern rather than their source
  DETAIL: `cmd/hooks_cleanstale_single_caller_guard_test.go`, `cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go` and `cmd/rename_restore_cleanup_survival_integration_test.go` all exercise `cmd/run_hook_stale_cleanup.go` and `cmd/doctor.go` under concern-derived names (`.claude/skills/golang-testing/SKILL.md:62,85` — a split test file's name must still derive from the source file name). All three predate the work unit; the fourth in the original set, `cmd/hookkey_no_regression_upgrade_test.go`, was deleted whole by this phase.
  FILES: cmd/hooks_cleanstale_single_caller_guard_test.go, cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go, cmd/rename_restore_cleanup_survival_integration_test.go

- The failed-read-counts-as-set posture has one named home and three anonymous restatements
  DETAIL: `restoreWindowActive` (`cmd/run_hook_stale_cleanup.go:60-63`) names the rule for one of four readers; `cmd/state_daemon.go` states the inverse error policy twice (tick and shutdown flush) and `cmd/state_commit_now.go` takes the same posture behind its own seam with a third reporting policy. None of those files is a phase-2 file and none changed. The natural end state is one named home — plausibly `internal/state` beside `IsRestoringSet`, taking the report/quiet policy from the caller.
  FILES: cmd/state_daemon.go, cmd/state_commit_now.go, cmd/run_hook_stale_cleanup.go, internal/state/markers.go

- `ResolveStructuralKey` and `ListAllPanes` have no production callers
  DETAIL: `internal/tmux/tmux.go:226` and `:590` are reached only from `internal/tmux/tmux_test.go:1569-1611` and `:1423-1556`. Production reaches the structural shape through `StructuralKeyFormat` + `ListAllPanesWithFormat` (`cmd/bootstrap/stale_marker_cleanup.go:57`) instead. Both were already test-only before this phase — phase 2 did not orphan them — but `CLAUDE.md:60` describes all three as serving "non-hook structural use", which holds only for the constant. Verify before deleting: phase 3 retires the positional hook machinery and may reach these.
  FILES: internal/tmux/tmux.go, cmd/bootstrap/stale_marker_cleanup.go, CLAUDE.md

## Observations

- `internal/state/schema.go:20-22` ("Session's PortalID persists the immutable @portal-id so a renamed session's hook key survives a reboot") and `internal/restore/session.go:31-34` now describe a key shape registration no longer writes — fails **plan-authorable**: phase 3 owns retiring `@portal-id` entirely, including the passages that describe it. Flagged so the orchestrator can confirm phase 3's task text reaches these two Go doc comments and not only the `CLAUDE.md` prose.
- `hook rm --on-resume` from an unstamped pane now resolves an empty hook key and calls `store.Remove("", …)` (`cmd/hooks.go:210-214`) — fails **plan-authorable**: the spec's removal section fixes the exact words and the exit-0-iff-removed rule for exactly this case, and phase 4 owns it.
- `internal/tmux` holds 40 test files against 13 source files, most named after a symbol rather than a source file, and phase 2 added four more in that style (`pane_hook_rows_test.go`, `pane_option_test.go`, `resolve_hookkey_test.go`, `hookkey_moved_pane_realtmux_test.go`) — not consolidation: the naming is a long-established package-wide practice, and re-deriving 40 file names from `tmux.go` is architecture re-litigation, not a phase sweep.
- `parsePaneHookRows` (`internal/tmux/tmux.go:630-643`) and `parsePaneOutput` (`:382-397`) share a skip-blank-lines loop — not consolidation: the shared part is five lines and the two differ in return shape and in whether a malformed line is an error, so a common helper would carry a bool or an error channel for no gain.
- `runHookStaleCleanup` still takes five parameters against the project's ≤4 convention (`cmd/run_hook_stale_cleanup.go:66-72`) — fails **plan-authorable**: phase 3's task text pins the five-argument call form and phase 5 reorders the body while keeping the signature.
- The socket prefix passed to `tmuxtest.New` across the hook-key suites is `ptl-xsite-` / `ptl-hookkeys-` / `ptl-hookkey-` / `ptl-panemove-` / `hookkey-`, and `internal/tmux/hookkey_format_realtmux_test.go` alone omits `EnsureServer` — folded into F2 as evidence rather than proposed on its own.
