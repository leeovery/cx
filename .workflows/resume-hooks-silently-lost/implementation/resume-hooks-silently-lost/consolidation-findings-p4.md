# Consolidation Findings: resume-hooks-silently-lost (Phase 4)


## Findings

### F1: A fourth fake for the pane-token enumeration, beside the file that exists to hold it
- **Class**: near-miss
- **Evidence**: `mockPaneHookLister` (cmd/hooks_test.go:129-138) declares `rows []tmux.PaneHookRow` / `err error` / `calls int` and counts every read. `recordingHookKeyLister` (cmd/hookkey_vocabulary_test.go:65-82) already declares exactly that trio (`hookKeyCalls` for the counter) plus `TryGetServerOption`, and it lives in the file whose header (cmd/hookkey_vocabulary_test.go:1-3) declares itself the home for "the enumeration rows they arrive in, and the seam fakes that answer with them". `*recordingHookKeyLister` already satisfies the new `PaneHookLister` interface (cmd/hooks.go:24-26) structurally — the extra method costs the list tests nothing. Consumers of the counter: cmd/run_hook_stale_cleanup_test.go:268, :401, :423, :450, :483.
- **Proposed shape**: site the plain `rows`/`err`/`calls` fake in cmd/hookkey_vocabulary_test.go and have `recordingHookKeyLister` embed it, keeping only `TryGetServerOption` and the `restoring`/`restoringErr` fields of its own; rename `hookKeyCalls` → `calls` at the five sweep-test sites. Nothing about the `PaneHookLister` seam being narrower than `AllPaneLister` changes — the interface stays as authored; only the fixture stops being written twice. Keep `loudPaneHookLister` (cmd/hooks_test.go:313-319) exactly as it is: its behaviour (fail on read) is not the same fixture, and F4 gives it more work. The pre-existing three-fake merge (`stubAllPaneLister`, `fakeHookLister`) is deliberately out of this finding.

### F2: The phase's two new command drivers land in two different homes, and its new file-unchanged assertion re-rolls the package's reader
- **Class**: drift
- **Evidence**: task 4-3 put `runHookList` in the shared staging home (cmd/testhelpers_test.go:60-73) though its only consumer is cmd/hooks_test.go; task 4-2 put `runHookRm` (cmd/hooks_rm_exit_test.go:16-25), `seedHooksFile` (:29-37) and `assertHooksFileUnchanged` (:39-48) in its own suite file. All three are staging by cmd/testhelpers_test.go:1-3's own definition ("how a test is set up and driven"), and `runHookSet` (cmd/testhelpers_test.go:27-34) is already there — so the phase leaves the three `hook` verb drivers under two different rules. Separately, both new helpers hand-roll `os.ReadFile` + `t.Fatalf` where cmd/bootstrap_production_test.go:129-141 already exposes `readFileBytes(t, path)` for the same package.
- **Proposed shape**: move `runHookRm`, `seedHooksFile` and `assertHooksFileUnchanged` into cmd/testhelpers_test.go beside `runHookSet` and `runHookList`, so one home holds all three verb drivers and their staging, and have both read through the existing `readFileBytes`. One nuance to carry deliberately: `readFileBytes` returns nil on ENOENT, so a deleted hooks.json would fail the byte comparison instead of fatalling on the read — same verdict, different message; no test changes colour. Two guards on the same edit: (a) do **not** re-point the remaining `hooks …` alias blocks in cmd/hooks_test.go at the shared drivers — they drive the back-compat alias verbatim (cmd/hooks_test.go:53, :71, :119, :343, :365, :391, :416, :436, :443, :464, :527, :549, :576, :602, :628, :649, :673, :702, :733, :763, :796, :825, :855, :880, :934, :958) and the drivers drive the canonical `hook`; (b) cross-reference the two helper-file headers so a reader landing on either learns where the other half lives.
- **Bank**: byte-identity assertions hand-rolled across the cmd suite, 4-2 adding a ninth (phase-caused half only); the two-helper-file rule stated in one file and the alias-collapse hazard 2-9 predicted; the two helper-only files that want a stated split.

### F3: The new `readFileBytes` in internal/hooks has an un-migrated twin in the same file
- **Class**: duplication
- **Evidence**: `readFileBytes` (internal/hooks/store_test.go:43-53, new in 4-1) versus the inline `os.ReadFile` + `t.Fatalf` + `bytes.Equal` at internal/hooks/store_test.go:1170-1176, which asserts the same thing (a failed save left the seeded file byte-identical) in the same file.
- **Proposed shape**: replace the inline read at :1170-1176 with `readFileBytes(t, seeded)`, leaving the comparison and its message as they are. The same helper would subsume four more copies in the same package (internal/hooks/store_shape_test.go:33-38, :55-60, :110-115, :130-135) — those are pre-existing and recorded under Pre-existing Debt rather than folded in.
- **Bank**: the new `readFileBytes` duplicating an inline read-and-compare left by an earlier task in this unit.

### F4: `hook list` became tmux-touching, and the tests that Execute it were not re-pointed
- **Class**: drift
- **Evidence**: after 4-3 the list body reaches tmux — cmd/hooks.go:147 calls `buildPaneHookLister()`, which falls back to `tmux.DefaultClient()` (cmd/hooks.go:83-88). Five sites Execute that body with no `PaneLister` injected: cmd/hooks_test.go:45-63 (:53), :65-81 (:71), :925-946 (:934), and cmd/root_test.go:299 and :302 (whose table injects `hooksDeps` only for the `set`/`rm` rows, cmd/root_test.go:315-327). All five stay off tmux only because their store is empty and the guard at cmd/hooks.go:143 returns before the read — an implicit dependency no assertion states. CLAUDE.md's rule is explicit ("A test that Executes a real command body MUST inject every tmux-touching `*Deps` seam"), and cmd's `TestMain` TMUX poison is the structural enforcement that would otherwise catch a missed injection loudly.
- **Proposed shape**: inject `hooksDeps = &HooksDeps{PaneLister: &loudPaneHookLister{t: t}}` (with the usual `t.Cleanup`) at the five sites. The fake already exists (cmd/hooks_test.go:313-319) and 4-3 wrote it for exactly this contract, so no new vocabulary appears; every site's store is empty, so no assertion changes and the injection turns today's incidental isolation into the stated one. If the orchestrator prefers the minimal form, a plain `&mockPaneHookLister{}` at the same five sites satisfies the injection rule without the added guard.

### F5: CLAUDE.md still describes the pane-token enumeration as stale-cleanup's alone
- **Class**: comments
- **Evidence**: CLAUDE.md:60 calls `ListAllPaneHookKeys` "the stale-cleanup `list-panes -a -F` enumeration returning one `PaneHookRow{Token, Location}` per live pane … and a display-only `<session>:w.p` `Location` that is never a key"; CLAUDE.md:166 repeats "stale-cleanup enumerates live tokens via `tmux.ListAllPaneHookKeys`". As of 4-3 there is a second consumer — cmd/hooks.go:160-184 (`paneLocationsByToken`), reached from cmd/hooks.go:147 through the new `PaneHookLister` seam (cmd/hooks.go:20-26) — and the `Location` half is now rendered to the user as `hook list`'s fourth column (cmd/hooks.go:150). The `cmd/hooks.go` row of the same file (the "Resume-hook command" entry) still describes the command set without any live read on the listing path.
- **Proposed shape**: correct the tmux row so the enumeration is named by what it returns rather than by one caller, and note in the Resume-hook command row that `hook list` resolves each entry's location through one live enumeration per invocation, rendering it as a fourth tab-separated column that is empty when no live pane carries the token, when the key is not token-shaped, or when the read fails (no server). Documentation only — no code moves, nothing changes colour. (This is a different sentence from the undocumented shape-aware reaper the phase-3 bank entry names; that one is phase 5's paragraph.)

## Bank Verdicts

- Pre-existing repo-wide lint/format debt outside this task's surface. — residue — the repo-wide lint/format backlog is banked and deliberately untouched for this work unit (new code matches the surrounding `errors.As` convention); it rides to the end-of-implementation pass, not a phase-4 task.
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

- runHookStaleCleanup now takes 5 parameters, one over the project convention. — residue — still true (`runHookStaleCleanup` takes 5 parameters at cmd/run_hook_stale_cleanup.go:65-71). The subject is phase-1 code phase 4 does not touch, and the sweep's call surface is what phase 5's lock work sits next to.
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

- Three parallel fakes for the one AllPaneLister seam in package cmd. — residue — pre-existing, rides to the end-of-implementation analysis. Confirmed still true (cmd/bootstrap_production_test.go:100, cmd/run_hook_stale_cleanup_test.go via cmd/hookkey_vocabulary_test.go:65, cmd/doctor_test.go:805). Phase 4 added a fourth copy; that copy alone is F1's subject and is NOT folded here.
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

- Two temp hooks-store seeders in package cmd, neither derived from the other. — residue — pre-existing, rides to the end-of-implementation analysis. Confirmed still true (`newTempHooksStore` cmd/bootstrap_production_test.go:~110, `seedHooksJSON` cmd/doctor_test.go:811).
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

- The restore-window rule is now single-sourced in cmd, but the daemon states the inverse error policy twice with no shared name. — residue — still true (cmd/state_daemon.go:174 and :339 both read `state.IsRestoringSet` and stand down loudly). Daemon lifecycle code outside this phase's surface.
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

- A third reader of the same failed-read-counts-as-set posture sits in cmd, in a file the daemon entry does not name. — residue — still true (cmd/state_commit_now.go:55 wires the third reader behind its `IsRestoring` seam). Outside this phase's surface.
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

- TestDoctorFixPrunedHookOutput is now fully subsumed by a sibling test. — residue — still present (cmd/doctor_test.go:1665). Deleting a case changes the count and needs an owner; the subject is phase-1 test code.
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

- Four further cmd test files touching doctor.go / run_hook_stale_cleanup.go still carry concern-derived names. — residue — naming convention across four pre-existing cmd test files phase 4 does not touch.
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

- staleSeed is now declared twice, byte-identical, in one file. — residue — still true (`staleSeed` declared at cmd/run_hook_stale_cleanup_test.go:382 and :570). Phase-1 test code outside this phase.
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

- The removal leaves two byte-identical subtests in TestDoctorStaleHooksCheck. — residue — still true (cmd/doctor_test.go:1116 and :1140 remain character-for-character the same case). Resolving it deletes or re-differentiates a case, which needs an owner.
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

- The gone-pane message reaching the user is three wraps deep, and Phase 4 reworks the same call site for hook rm wording. — residue — phase 4 landed the rm wording (cmd/hooks.go:277, :293) without trimming the wrap at cmd/hooks.go:70. Trimming changes user-visible error text and falsifies the `resolve` assertion at cmd/hooks_test.go:501, so it is not a pure refactor — a wording decision, not consolidation.
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

- ResolveStructuralKey and ListAllPanes have no production callers. — residue — pre-existing, rides to the end-of-implementation analysis. Confirmed still true after phase 3: internal/tmux/tmux.go:226 (`ResolveStructuralKey`) and :584 (`ListAllPanes`) have no non-test callers.
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

- A saver-hosting integration fixture omits the teardown guard CLAUDE.md prescribes for exactly its shape. — residue — still missing (no `RegisterStateDirTeardownGuard` in cmd/state_daemon_hook_cleanup_integration_test.go). An integration-fixture flake fix outside this phase, and adding a setup call is not a pure refactor.
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

- stubAllPaneLister is the last piece of the hook-sweep seam vocabulary still sited with a single consumer. — residue — the subject is the pre-existing `stubAllPaneLister` and its siting; it belongs with the three-fake merge at the end pass. F1 is deliberately narrower and touches only the copy phase 4 authored.
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

- Package cmd now holds two helper-only test files that want to be one. — confirmed → F1, F2 — both findings place phase 4's new helpers by each file's own declared rule (cmd/hookkey_vocabulary_test.go:1-3 for seam fakes, cmd/testhelpers_test.go:1-3 for staging). Whether cmd ends with one helper file or a deliberate two stays open for the end pass; neither finding re-opens it.
```json
{
  "task": "resume-hooks-silently-lost-2-8",
  "source": "reviewer",
  "summary": "Package cmd now holds two helper-only test files that want to be one.",
  "detail": "cmd/testhelpers_test.go (writeHooksJSON, readHooksJSON) and the new cmd/hookkey_vocabulary_test.go (seed vocabulary, row builders, restoringOption, recordingHookKeyLister) are both shared-helper homes with no source file behind them. Task 2-9 already plans to move hooksFileInTempDir/runHookSet from cmd/hooks_pane_token_test.go:37-51 into cmd/testhelpers_test.go — that pass should decide whether cmd ends with one helper file or a deliberate two, rather than accreting a third.",
  "files": [
    "cmd/testhelpers_test.go",
    "cmd/hookkey_vocabulary_test.go",
    "cmd/hooks_pane_token_test.go"
  ]
}
```

- Two package-level names in package cmd now hold one key value. — residue — still true (cmd/rename_restore_cleanup_survival_integration_test.go:21 holds `renameRestoreToken = transienttest.ReapableHookKey(1)`, the same value as `reapableSeedB`). Seed vocabulary outside this phase.
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

- A few hook-file preamble sites outside the three hooks test files could take the bare helper now that it exists in the shared file. — residue — still true (cmd/state_daemon_test.go:792 and :813, cmd/version_guard_test.go:146). Suites this phase does not touch.
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

- The two-helper-file rule is stated in only one of the two files, and phase 4 could silently collapse alias coverage. — confirmed → F2 — the predicted hazard materialised only in its narrow form: three list drives moved to the canonical verb via `runHookList` (cmd/hooks_test.go:38, :104 and cmd/testhelpers_test.go:68), while the `hooks` alias is still driven end-to-end at cmd/hooks_test.go:53, :71, :119, :934 and cmd/root_test.go:302, so alias coverage did not collapse. F2 carries the explicit do-not-re-point guard; the un-cross-referenced header at cmd/hookkey_vocabulary_test.go:1-3 is folded into F2's shape.
```json
{
  "task": "resume-hooks-silently-lost-2-9",
  "source": "reviewer",
  "summary": "The two-helper-file rule is stated in only one of the two files, and phase 4 could silently collapse alias coverage.",
  "detail": "cmd/testhelpers_test.go:1-3 states the staging/vocabulary split and points at the other file; cmd/hookkey_vocabulary_test.go:1-3 describes only its own contents and does not point back, so a reader landing there learns nothing about where staging goes. Separately: with runHookSet now in the shared file, a phase-4 contributor reworking cmd/hooks_test.go may re-point the ten `hooks set` alias blocks at it and silently collapse them onto the canonical `hook` verb, leaving alias coverage resting solely on TestHookCommandRename. The helper doc comment names the verb it drives, which is the available mitigation.",
  "files": [
    "cmd/hookkey_vocabulary_test.go",
    "cmd/testhelpers_test.go",
    "cmd/hooks_test.go"
  ]
}
```

- The last hand-rolled level filter now lives in internal/spawn, and it is not the same filter. — residue — still true (internal/spawn/terminalsconfig_test.go:21). Another package, outside this phase.
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

- The same five-property audit-breadcrumb block is still written inline outside this task two helpers. — residue — still true (cmd/config_migrate_logging_test.go:42-57 plus the per-package copies). A shared-helper design change spanning five packages, outside this phase.
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

- Two near-duplicate TMUX_PANE-unset subtests in TestHooksSetCommand, one a strict subset of the other. — residue — pre-existing by its own dating and still true (cmd/hooks_test.go:380-403 and :538-557). Recorded under Pre-existing Debt alongside the rm-side twins phase 4 merely sits next to.
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

- assertHookFireCount reads the hook-fire file with no wait, racing the hydrate helper across the whole restore hook-fire family. — residue — still true (internal/restore/rename_reboot_shared_test.go:47 reads with no wait). A real-tmux timing fix in another package; adding a poll is not a pure refactor.
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

- Unit-lane real-tmux restore tests silently depend on whichever portal is installed; root cause is a bare portal in the hydrate argv. — residue — still true (internal/restore/session.go:310-319 bakes a bare `portal`). A production change plus a lane move, outside this phase.
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

- CLAUDE.md never describes the shape-aware reaper or internal/session new token surfaces, so the retain-old-format-forever rule is undocumented. — residue — still true (CLAUDE.md holds no occurrence of `IsTokenShaped` or "token-shaped"). Caused by phases 1 and 3, and task 5-1 already edits the same Resume-hooks paragraph, so it lands there. F5 is a different, phase-4-caused sentence in the same file.
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

- Four integration tests exec portal state hydrate from the ambient PATH instead of staging a binary — residue — outside this phase's surface (internal/restore and cmd/bootstrap integration fixtures).
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

- Session-level -t targets in the tmux client that bypass the package own exactness rule — residue — pre-existing, rides to the end-of-implementation analysis. Confirmed still true (internal/tmux/tmux.go's session-level `-t` sites).
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

- The two rename-reboot integration tests duplicate the whole capture-persist-reboot-hydrate bracket; one file names it in helpers, the other inlines it — residue — pre-existing, rides to the end-of-implementation analysis. Confirmed still true: `captureAndPersist` / `rebootAndHydrate` at internal/restore/rename_reboot_durability_integration_test.go:105 and :123 are still re-implemented inside `runRenameRebootFire` (internal/restore/rename_reboot_hook_integration_test.go:73).
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

- Four fixture helpers in restore_test are pairwise generalisations of one another — residue — pre-existing, rides to the end-of-implementation analysis. Half is now moot: `seedScrollback`/`seedPaneScrollback` are gone (task 3-8 folded them into the shared seeder). The assertion pair survives — internal/restore/rename_reboot_shared_test.go:47 (`assertHookFireCount`) vs internal/restore/multipane_legacy_integration_test.go:213 (`assertMarkerFiredOnce`).
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

- TestLookupOnResume repeats a five-line seed preamble and the same three-assertion no-hook block across eleven subtests — residue — pre-existing, rides to the end-of-implementation analysis. Confirmed still true (internal/hooks/lookup_test.go still opens nine subtests with the same seed preamble).
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

- Four hand-rolled (component,msg) exactly-one-record sink filters remain across three packages; internal/logtest is the natural home — residue — still true (cmd/state_daemon_cycle_summary_test.go:33, cmd/bootstrap/clean_sweep_summary_test.go:18 and :30). A `logtest` API change across three packages, outside this phase.
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

- The marker bracket set half is unobserved by every caller, not just internal/restore — residue — outside this phase's surface (internal/restoretest and the restore suites).
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

- Two more copies of the scrollback seeder survive in cmd/bootstrap, in the one package that actually asserts on the payload — residue — outside this phase's surface (cmd/bootstrap).
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

- readHookKey in internal/tmux is a third fixture pane-token read that must NOT be consolidated — residue — a do-not-touch note, still true and honoured: no finding here proposes touching internal/tmux/hookkey_format_realtmux_test.go:11-15.
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

- A third production bare -t issuer exists in the daemon capture loop, exposed to the rename class this workflow is about — residue — a genuine production defect outside this work unit's scope (cmd/state_daemon.go:278). It needs a scoping decision, not a phase-4 task; not folded.
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

- project.Store.Remove still carries the identical defect task 4-1 just removed from hooks — residue — confirmed still present (internal/project/store.go:211-213, doc comment intact: "the breadcrumb is emitted either way"). The identical defect phase 4 removed from the hooks store, in a different store and outside this work unit's scope; it needs a scoping decision, not a phase-4 task. Not folded.
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

- The new readFileBytes helper duplicates an inline read-and-compare left by an earlier task in this unit — confirmed → F3
```json
{
  "task": "resume-hooks-silently-lost-4-1",
  "summary": "The new readFileBytes helper duplicates an inline read-and-compare left by an earlier task in this unit",
  "detail": "internal/hooks/store_test.go:1170-1176 does os.ReadFile + t.Fatalf + bytes.Equal inline, which is now exactly readFileBytes (:45). One-line consolidation; the fix touches a sibling task assertion, not 4-1 own.",
  "files": [
    "internal/hooks/store_test.go"
  ]
}
```

- Byte-identity file assertions are hand-rolled at eight-plus sites across the cmd test suite, and 4-2 adds a ninth — confirmed → F2 — the phase-caused half only: the new `assertHooksFileUnchanged`/`seedHooksFile` re-roll a reader the package already has and sit outside the staging home. The eight pre-existing raw `bytes.Equal` sites are not folded; they are recorded under Pre-existing Debt.
```json
{
  "task": "resume-hooks-silently-lost-4-2",
  "summary": "Byte-identity file assertions are hand-rolled at eight-plus sites across the cmd test suite, and 4-2 adds a ninth",
  "detail": "assertHooksFileUnchanged (cmd/hooks_rm_exit_test.go:39-48) and assertPrefsUnchanged (cmd/prefs_translation_persist_test.go:492) are the same function under two names, and raw bytes.Equal(before, after) comparisons appear at cmd/doctor_test.go:1293,1482,1750, cmd/run_hook_stale_cleanup_test.go:331,397,419,595,624 and cmd/cleanstale_transient_listpanes_shared_test.go:72. A shared readFileBytes already exists at cmd/bootstrap_production_test.go:130 and the new helper re-rolls the read rather than using it. One assertFileUnchanged(t, path, before) in cmd/testhelpers_test.go — whose header already declares itself the home for shared staging helpers, and which already holds runHookSet, runHookRm direct counterpart — would collapse all of them.",
  "files": [
    "cmd/testhelpers_test.go",
    "cmd/hooks_rm_exit_test.go",
    "cmd/prefs_translation_persist_test.go",
    "cmd/doctor_test.go",
    "cmd/run_hook_stale_cleanup_test.go",
    "cmd/cleanstale_transient_listpanes_shared_test.go",
    "cmd/bootstrap_production_test.go"
  ]
}
```

- cmd/hooks.go now carries four near-identical nil-check dependency builders — residue — real, but no behaviour-neutral collapse exists: the four builders span three interface types plus the func-typed `session.IDGenerator` (func types are not `comparable`, so a generic picker cannot cover it), and the per-field nil-check shape is the repo's own DI convention (cmd/open.go:269 and :490, cmd/root.go:70, cmd/doctor.go:104-131). A package-shape question for the end pass, and phase 5 adds no fifth.
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

## Pre-existing Debt

- `TestHooksRmCommand` carries two pairs of duplicate subtests, one pair byte-identical
  DETAIL: cmd/hooks_test.go:778-809 ("--pane-key flag removes specified key without requiring TMUX_PANE") and :837-867 ("it removes the verbatim key on rm --pane-key without consulting the resolver") share fixture, seed, loud resolver and every assertion — the same test twice under two names. cmd/hooks_test.go:617-636 ("returns error when TMUX_PANE is not set") and :869-888 ("it errors when TMUX_PANE is unset for the rm fallback") are the same pair one function over, the second a strict subset. Both predate this work unit and are the rm-side twins of the set-side pair already banked (:380-403 vs :538-557). Phase 4 added a third home for the same cases (cmd/hooks_rm_exit_test.go) but touched none of these.
  FILES: cmd/hooks_test.go

- Four inline read-and-compare blocks in internal/hooks share a package with the reader that would collapse them
  DETAIL: internal/hooks/store_shape_test.go:33-38, :55-60, :110-115, :130-135 each do `os.ReadFile` + `t.Fatalf` + a string/byte comparison; `readFileBytes` (internal/hooks/store_test.go:43-53) is in the same test package. The file predates this phase and the phase does not touch it; F3 covers only the same-file twin.
  FILES: internal/hooks/store_shape_test.go, internal/hooks/store_test.go

- `hooks.Store.Get` has no production caller
  DETAIL: internal/hooks/store.go:257-270 is reached only from its own TestGet (internal/hooks/store_test.go:623-660) and a restore fixture assertion (internal/restore/rename_reboot_shared_test.go:38). The production read path is `LookupOnResume` (internal/hooks/lookup.go:13-30), which walks the loaded map itself. Its last production caller predates this work unit — phase 4 did not orphan it, and its own removal path deliberately does not use it. Worth knowing before phase 5 wraps the store's mutations: an exported read with no caller is exactly the shape a lock design gets asked about.
  FILES: internal/hooks/store.go, internal/hooks/lookup.go

- Two list subtests stub the bootstrap orchestrator for a bootstrap-exempt command
  DETAIL: cmd/hooks_test.go:25 and :88 set `bootstrapDeps = &BootstrapDeps{Orchestrator: &nopRunner{}}` under the comment "Stub bootstrap so the real orchestrator never runs against the test's tmux server", but `hook` is in the `skipTmuxCheck` set, so `PersistentPreRunE` never reaches the orchestrator — inert scaffolding carrying a claim that reads as load-bearing. Both stubs predate this phase (unchanged by its diff) and the three list subtests beside them carry no such stub.
  FILES: cmd/hooks_test.go

## Observations

- README.md:201's `hook list` line still reads "list all hooks" and never mentions the fourth column the command now prints — plan-authorable (a doc requirement task 4-3 could have carried), so not consolidation. It would land naturally in F5's edit.
- The four seam builders in cmd/hooks.go:76-102 repeat one nil-check shape a fourth time, crossing Rule of Three — but no behaviour-neutral collapse exists (they span three interface types plus the func-typed `session.IDGenerator`, which is not `comparable`), and the per-field shape is the repo's own DI convention (cmd/open.go:269, :490; cmd/root.go:70; cmd/doctor.go:104-131). Not proposed: no candidate refactor is an improvement.
- The hooks store's three mutators now disagree about whether a no-op is observable — `Set` emits a DEBUG `set-noop` (internal/hooks/store.go:93-96) while `Remove` (:139-145) and `CleanStale` (:233-235) emit nothing. Fails the no-behaviour-change test in either direction, and Remove's silence is pinned by internal/hooks/store_test.go's zero-record subtest.
- The skip-the-empty-token rule is now spelled twice in package cmd — `liveTokensFrom` (cmd/run_hook_stale_cleanup.go:43-51) and `paneLocationsByToken` (cmd/hooks.go:170-175) — each with its own comment. Two instances, under Rule of Three, and neither can derive from the other (one yields tokens, the other a token→location map). No extraction proposed.
- `paneLocationsByToken` takes its lister as a parameter while its three sibling seams are fetched inside their users (cmd/hooks.go:68, :107, :112) — an inconsistency, but the parameterised form is the better one and nothing depends on the difference. Not proposed.
- `seedHooksFile` now names two unrelated helpers in two packages (cmd/hooks_rm_exit_test.go:29 seeds from a map and returns bytes; internal/hooks/store_shape_test.go:16 seeds a raw string and returns a store). Different packages, no collision, no drift in behaviour — naming only.
- `hook list`'s swallowed enumeration error (cmd/hooks.go:161-167) emits no breadcrumb, unlike `doctor`'s not-evaluable line for the same failed read (cmd/doctor.go:314-317). Deliberate and spec-shaped (a listing renders, a diagnosis reports), and changing it is a behaviour change — not consolidation.
- The gone-pane error reaching the user from `hook rm` is still three wraps deep (cmd/hooks.go:70 over internal/tmux's resolve error over `CommandError.Error()`). Trimming a layer changes user-visible text and falsifies cmd/hooks_test.go:501 — fails the no-behaviour-change test.
