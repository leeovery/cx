# Consolidation Findings: resume-hooks-silently-lost (Phase 1)

## Findings

### F1: The restore-window stand-down is written twice, once per reader
- **Class**: duplication
- **Evidence**:
  - `cmd/run_hook_stale_cleanup.go:53-64` — `if restoring, err := state.IsRestoringSet(lister); restoring || err != nil` under a 3-line rationale comment (task 1-3)
  - `cmd/doctor.go:307-313` — the identical predicate under a near-identical 4-line rationale comment (task 1-5)
  - `cmd/doctor.go:208` — `skippedPrunePhrases[skipReasonRestoring] = "restore may be in progress"` (task 1-4)
  - `cmd/doctor.go:312` — `"restore may be in progress (not evaluable)"`, the same phrase re-typed with a suffix (task 1-5)
- **Proposed shape**: one unexported predicate in `cmd` — e.g. `func restoreWindowActive(lister state.RestoringChecker) bool` — holding the failed-read-counts-as-set rule and its rationale comment exactly once, called from both sites. One `const restoreStandDownPhrase = "restore may be in progress"` consumed by the phrase map and composed as `restoreStandDownPhrase + " (not evaluable)"` for the check's detail. Both call sites keep their current position in their guard ladders, so `checkStaleHooks`'s order (store-nil → load → restore marker → enumeration) is untouched — the order phase 5's task text says task 1-5 fixed. Pure extraction: identical reads, identical strings, no test re-pointing beyond none.
- **Bank**:
  - "The \"restore window is active\" rule is now written twice, once per reader." — the load-bearing `restoring || err != nil` half exists as two copies; a change to one is silent in the other.
  - "The restore stand-down phrase exists as two literals in one file."

### F2: Comments the shape rule narrowed, left uncorrected by the tasks that narrowed them
- **Class**: comments
- **Evidence**:
  - `cmd/run_hook_stale_cleanup.go:80-81` — "An empty live set is a bad read, not authority: it must never reach `CleanStale`, **which would delete every entry**." After task 1-1, `CleanStale` deletes only the keys the staleness rule can judge (token-shaped or empty) and retains the rest (`internal/hooks/store.go:182-199`). Task 1-4 rewrote the WARN on the very next line (`:86`) without touching the claim above it.
  - `cmd/doctor.go:295-297` — "an unreadable or empty live set **would otherwise report every entry stale** and mislead a `--fix` into mass-deleting user-authored on-resume commands." `checkStaleHooks` now counts only judgeable keys — pinned by `cmd/doctor_test.go:1106-1115`, where two retained old-format keys sit alongside one stale token-shaped key and the detail is `1 stale hook entry`. Task 1-5 edited this function directly beneath the comment.
  - `CLAUDE.md:100` — "cycle summaries … emit one INFO per cycle **with per-item detail at DEBUG**". Task 1-2 promoted the hooks clean-stale per-key line to INFO (`internal/hooks/store.go:235-236`), so the reaper — the one sweep an operator greps when a hook vanishes — is now a counterexample to the generalisation. No plan task owns this passage: phase 2 assigns the hook-key rewrite, phase 3 the `@portal-id` removal, phase 4 the `hook rm` exit code, phase 5 the lock sidecar; none touches the logging section.
- **Proposed shape**: narrow the two in-source claims to the judgeable subset ("would delete every key it can judge" / "would report every judgeable entry stale") rather than re-arguing them; add the hooks clean-stale per-key INFO to `CLAUDE.md:100`'s instrumented bullet, or qualify the per-item-at-DEBUG generalisation to the components it still describes. Comment-and-doc text only, zero code.
- **Bank**:
  - "Two mass-deletion-guard rationale comments now overstate their hazard, in files this task deliberately did not touch." — the reviewer deferred the correction to later work; phase 2's task text keeps the row-counting guard but mandates no comment rewrite there, and phase 5 keeps the branch, so the claim can survive to the end unfixed.
  - "CLAUDE.md logging bullet now misdescribes the hooks clean-stale sweep."

### F3: The hook-key seed shape is load-bearing semantics carried by ~18 hand-rolled literals
- **Class**: duplication
- **Evidence**: after task 1-1 a seed key's *shape* silently decides whether a test exercises reaping or retention, and every site spells its own literal, each hand-verified to be exactly 6 bytes of `session.NanoIDAlphabet`:
  - `cmd/doctor_test.go:861,880,894,954,971,988,1005,1055,1063,1071,1080,1092,1108,1163-1165,1209-1210,1340,1411,1564,1578` — `sessA1`, `sessB1`, `sessC1`
  - `cmd/run_hook_stale_cleanup_test.go:40-41,165-167,219,286,291` — `keyA00`, `keyB00`, `keyC00`, `keyD00`, `orpha1`, `tok123:0.0`
  - `cmd/hook_sweep_restore_standdown_test.go:24,141`, `cmd/hook_sweep_standdown_report_test.go:19,227`, `cmd/hook_prune_output_test.go:14,29` — `gone01`, `sessA1`
  - `cmd/hook_retention_shape_test.go:15-16,49,73` — `live:0.0`, `old-session:3.1`, `another-session:0.0`
  - `cmd/state_daemon_hook_cleanup_test.go:33,63,93,147,205-206`, `cmd/state_daemon_run_test.go:529,566,599,630` — `stale1`, `keyA00`, `keyB00`
  - `cmd/cleanstale_transient_listpanes_shared_test.go:48-50` — `alpha1`, `beta02`, `gamma1`
  - `cmd/state_daemon_hook_cleanup_integration_test.go:43`, `cmd/rename_restore_cleanup_survival_integration_test.go:54`, `cmd/hookkey_no_regression_upgrade_test.go:37`, `cmd/bootstrap/transient_listpanes_helpers_integration_test.go:104-105`, `cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:101` — `gonXxX`, `gone01`, `smoke1`, `smoke2`
  - `internal/hooks/store_shape_test.go:25,59,82,102`, `internal/hooks/store_test.go:891,916,941,1029` — `aBc123`, `Zx9Q0p`, `stalA0`, `stalB0`, `stale1`
  A literal that drifts to 7 bytes, or gains a `-`, silently flips a reap test into a retention test and stays green for the wrong reason. Nothing enforces the shape at any of these sites.
- **Proposed shape**: a small shared test vocabulary naming the two roles instead of spelling them — `reapableHookKey(n)` (asserted `session.IsTokenShaped` at construction) and `unjudgeableHookKey(n)` — sited beside `internal/transienttest.SeedHooksJSON` so the `cmd`, `cmd/bootstrap` and `internal/hooks` surfaces all reach one declaration; re-point the seed sites to it. Test-only, values unchanged, every assertion keeps its current meaning. Timing note for the orchestrator: phase 2's task text re-points every one of these files again by name — landing this first turns that re-point into one edit and removes the chance of a mis-shaped seed passing silently.
- **Bank**:
  - "Hook-key test seeds are hand-rolled literals across a dozen files, and every later phase re-points all of them again."

### F4: Four new `cmd` test files named off the concern, with the phase's fixtures scattered across five
- **Class**: drift
- **Evidence**: each task added its own file rather than the file matching its source, so `cmd/run_hook_stale_cleanup.go` and `cmd/doctor.go` are now tested from six files between them:
  - `cmd/hook_prune_output_test.go` (34 lines, one subtest), `cmd/hook_retention_shape_test.go`, `cmd/hook_sweep_restore_standdown_test.go`, `cmd/hook_sweep_standdown_report_test.go` — the `hook_` prefix points at `cmd/hooks.go`, which is not the source under test. `.claude/skills/golang-testing/SKILL.md:62,77,87`: a split test file's name must still derive from the source file name, never from the concern or symbol.
  - Fixture scatter: `staleDeps` / `seedHooksJSON` / `seedStalePruneFixture` / `assertStalePrunesApplied` / `runDoctorFixCmd` / `fakeHookLister` in `cmd/doctor_test.go:797,810,847,856,872,1300`; `newTempHooksStore` / `readFileBytes` / `keysOf` / `stubAllPaneLister` in `cmd/bootstrap_production_test.go:86-100`; `recordingHookKeyLister` in `cmd/run_hook_stale_cleanup_test.go:13-30`; `restoringOption` + `standDownRecord` in `cmd/hook_sweep_restore_standdown_test.go:190-232` — where `restoringOption` is consumed by all three fakes, declared in three other files.
  - Duplicated assertion: `cmd/doctor_test.go:894` asserts `strings.Contains(out, "Pruned stale hook: sessA1")` inside the shared `assertStalePrunesApplied` (three call sites), while `cmd/hook_prune_output_test.go:23-32` asserts exactly-one-line equality over the same fixture shape `seedStalePruneFixture` already builds. Deterministic output asserted by substring at one site and by equality at the other (code-quality: substring assertions where exact output is deterministic).
  - Duplicated fixture: `cmd/hook_sweep_restore_standdown_test.go:139-144` inlines a copy of `seedStalePruneFixture` (`cmd/doctor_test.go:856-869`) solely to swap the lister for `fakeHookLister{restoring: true}`; `cmd/hook_sweep_standdown_report_test.go:222-244`'s `runDoctorFixWithLister` is a third variant of the same fixture.
- **Proposed shape**: merge the four files into `cmd/run_hook_stale_cleanup_test.go` and `cmd/doctor_test.go`, splitting only under source-derived names if size demands it (`doctor_stale_hooks_test.go`); park `restoringOption` beside the fakes it serves; give `seedStalePruneFixture` a lister parameter so the two inlined variants collapse into it; single-source the pruned-line assertion at its strongest form. All movement — no test gains or loses a case.
- **Bank**:
  - "Phase-1 test files accreted against the file-per-source convention."
  - "The doctor --fix prune-stdout assertion now exists twice, at two strengths."
  - "The doctor --fix fixture is being re-implemented per stand-down reason instead of parameterised — 1-4 and 1-5 will each want another variant." (1-4's and 1-5's variants both now exist, so the parameterisation the entry was waiting on is due.)

### F5: Third copy of the deny-the-write fixture in `internal/hooks/store_test.go`
- **Class**: near-miss
- **Evidence**: `readOnlyDirPath` (`internal/hooks/store_test.go:27-37`) plus two hand-rolled seed-then-`chmod 0500` blocks — the pre-existing one at `:1024-1039` and the new one at `:936-948` (task 1-2), which carries the same two-line comment verbatim ("The seed write must succeed before the directory is locked, so this cannot use readOnlyDirPath"). Rule of Three reached by this phase.
- **Proposed shape**: `seedThenDenyWrites(t *testing.T, body []byte) (*hooks.Store, string)` — seed, `chmod 0500` the parent, register the restoring cleanup, return store and path; both blocks call it. The fix touches the pre-existing subtest at `:1024`, which is in scope: the phase's addition is what made it a triplicate.
- **Bank**:
  - "Third instance of the deny-the-write fixture in internal/hooks/store_test.go."

### F6: Inert test scaffolding and a subsumed assertion
- **Class**: dead-code
- **Evidence**:
  - `cmd/state_daemon_hook_cleanup_test.go:128-129` and `:154-155` construct a named `logtest.Sink` and install it, then never read it. Neither test's path emits on the `hooks` component: the first forces `store.Load` to fail (returns before any `hooksLogger` call) and the second fails the pane enumeration (which logs on the injected daemon logger only). The third install at `:213-214` is genuinely read at `:233`.
  - `cmd/doctor_test.go:1086-1088` — `if strings.Contains(got.detail, "stale hook entr")` runs after `assertRestoreWindowResult` (`:1146-1154`) has already asserted `got.detail` equals `"restore may be in progress (not evaluable)"` by exact equality. The second check cannot fire independently of the first.
- **Proposed shape**: drop the two unread sinks (if the intent was to silence rather than capture, say so with an unnamed `log.SetTestHandler(t, &logtest.Sink{})`); drop the subsumed `Contains`. Removals only — no case loses coverage, because in each instance the surviving assertion already subsumes the removed one.
- **Bank**:
  - "Two post-repair test assertions are weaker than their names promise." — only the `cmd/doctor_test.go:1080` half lands here; the `cmd/hook_sweep_standdown_report_test.go:179` half is in Observations (its fix strengthens what the test pins).

## Bank Verdicts

- Hook-key test seeds are hand-rolled literals across a dozen files — **confirmed → F3**
  ```json
  {
    "task": "resume-hooks-silently-lost-1-1",
    "source": "executor",
    "summary": "Hook-key test seeds are hand-rolled literals across a dozen files, and every later phase re-points all of them again.",
    "detail": "Key shape is now semantically load-bearing in tests (token-shaped = reapable, anything else = retained), but each site spells its own literal: internal/hooks/store_test.go, cmd/run_hook_stale_cleanup_test.go:157-161,209-213, cmd/doctor_test.go:854,903,946,963,980,997,1054-1057,1100-1101,1231,1302,1455, cmd/state_daemon_hook_cleanup_test.go, cmd/state_daemon_run_test.go:529,566, plus five integration seeds. A shared test-side vocabulary (reapable key / unjudgeable old-format key helper next to internal/transienttest.SeedHooksJSON) would make the next re-point one edit and make a mis-shaped seed impossible.",
    "files": [
      "internal/hooks/store_test.go",
      "cmd/run_hook_stale_cleanup_test.go",
      "cmd/doctor_test.go",
      "cmd/doctor_summary_test.go",
      "cmd/doctor_fix_theme_test.go",
      "cmd/state_daemon_hook_cleanup_test.go",
      "cmd/state_daemon_run_test.go",
      "cmd/cleanstale_transient_listpanes_shared_test.go",
      "cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go",
      "cmd/state_daemon_hook_cleanup_integration_test.go",
      "cmd/bootstrap/transient_listpanes_helpers_integration_test.go",
      "internal/transienttest/hooks.go"
    ]
  }
  ```

- Two mass-deletion-guard rationale comments now overstate their hazard — **confirmed → F2**
  ```json
  {
    "task": "resume-hooks-silently-lost-1-1",
    "source": "reviewer",
    "summary": "Two mass-deletion-guard rationale comments now overstate their hazard, in files this task deliberately did not touch.",
    "detail": "cmd/run_hook_stale_cleanup.go:41 says an empty live set reaching CleanStale would delete every entry, and cmd/doctor.go:278 says it would otherwise report every entry stale. Post-change both are true only of judgeable entries — on today's install, where every key is old-format, an empty live set would now delete nothing. The claims are narrowed rather than flatly false, and both regions are rewritten by the later work (the restoring check moves into runHookStaleCleanup, the onSkipped callback and the lock-timeout branch land alongside), so the correction belongs with that task rather than as a drive-by edit here.",
    "files": [
      "cmd/run_hook_stale_cleanup.go",
      "cmd/doctor.go"
    ]
  }
  ```
  The deferral the entry proposed has now expired: both regions landed in this phase (the restoring check at `cmd/run_hook_stale_cleanup.go:53-64`, the `onSkipped` wiring at `:47-51,86-87`) and neither comment moved. Phase 5's remaining edit to the branch reorders it and keeps the guard; no later task's text mandates the comment rewrite.

- CLAUDE.md logging bullet now misdescribes the hooks clean-stale sweep — **confirmed → F2**
  ```json
  {
    "task": "resume-hooks-silently-lost-1-2",
    "source": "reviewer",
    "summary": "CLAUDE.md logging bullet now misdescribes the hooks clean-stale sweep.",
    "detail": "CLAUDE.md:100 states cycle summaries emit one INFO per cycle with per-item detail at DEBUG. After this change the hooks clean-stale per-item detail is INFO. No task in the plan owns this passage — phases 2/3/4/5 each assign specific CLAUDE.md rewrites (hook-key scheme, @portal-id removal, hook rm exit code, lock sidecar) and none touches the logging section. It will otherwise ship false.",
    "files": [
      "CLAUDE.md"
    ]
  }
  ```
  Verified: the bullet's enumerated list (`capture:`, `bootstrap:`, `restore:`, `spawn:`, the `clean:` sweeps) does not name the hooks store, so the sentence is narrowed rather than flatly false — but the generalisation it makes is now contradicted by the reaper, which is the sweep the bullet's own audience greps.

- The doctor --fix prune-stdout assertion now exists twice, at two strengths — **confirmed → F4**
  ```json
  {
    "task": "resume-hooks-silently-lost-1-2",
    "source": "reviewer",
    "summary": "The doctor --fix prune-stdout assertion now exists twice, at two strengths.",
    "detail": "cmd/doctor_test.go:888 (inside the shared assertStalePrunesApplied, used by three call sites) asserts strings.Contains(out, \"Pruned stale hook: sessA1\"); the new cmd/hook_prune_output_test.go:29 asserts exactly-one-line equality over the same fixture shape that seedStalePruneFixture (doctor_test.go:850) already builds. Folding the exact assertion into the shared helper would single-source it and let the standalone file go, but it reaches a helper three other tests depend on.",
    "files": [
      "cmd/hook_prune_output_test.go",
      "cmd/doctor_test.go"
    ]
  }
  ```

- Third instance of the deny-the-write fixture in internal/hooks/store_test.go — **confirmed → F5**
  ```json
  {
    "task": "resume-hooks-silently-lost-1-2",
    "source": "reviewer",
    "summary": "Third instance of the deny-the-write fixture in internal/hooks/store_test.go.",
    "detail": "readOnlyDirPath (:27-37) plus two hand-rolled seed-then-chmod-0500 blocks — the pre-existing one at :1024-1035 and the new one at :936-947, which even carries the same two-line comment. Rule of Three now applies: a seedThenDenyWrites(t, body) helper would single-source it, but the fix touches a pre-existing subtest outside this task's diff.",
    "files": [
      "internal/hooks/store_test.go"
    ]
  }
  ```

- The doctor --fix fixture is being re-implemented per stand-down reason instead of parameterised — **confirmed → F4**
  ```json
  {
    "task": "resume-hooks-silently-lost-1-3",
    "source": "reviewer",
    "summary": "The doctor --fix fixture is being re-implemented per stand-down reason instead of parameterised — 1-4 and 1-5 will each want another variant.",
    "detail": "cmd/hook_sweep_restore_standdown_test.go:138-144 inlines a copy of seedStalePruneFixture (cmd/doctor_test.go:856-869) solely to swap in fakeHookLister{restoring: true}. Tasks 1-4 (onSkipped phrase lines) and 1-5 (checkStaleHooks not-evaluable) will each need the same fixture with a different lister, so the natural consolidation is one fixture taking the lister as a parameter, once all three exist.",
    "files": [
      "cmd/hook_sweep_restore_standdown_test.go",
      "cmd/doctor_test.go"
    ]
  }
  ```
  All three variants now exist (`cmd/hook_sweep_restore_standdown_test.go:139-144`, `cmd/hook_sweep_standdown_report_test.go:222-244`, `cmd/doctor_test.go:856-869`), so the condition the entry named as its trigger is met.

- Pre-existing repo-wide lint/format debt outside this task's surface — **residue — wholly pre-existing; the phase only sits next to it. `gofmt -l .` still reports exactly `internal/tui/help_modal_test.go`, which no phase-1 commit touched, and the `modernize` findings are in `cmd/root.go`, `cmd/bootstrap_progress.go` and `main.go` — none a phase file. Recorded under Pre-existing Debt.**
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

- runHookStaleCleanup now takes 5 parameters, one over the project convention — **residue — plan-authored, not consolidation. The plan mandated `onSkipped` as a positional parameter, and phase 3's task text pins the five-argument call form verbatim (`runHookStaleCleanup(client, store, nil, nil, nil)` — "the five-parameter form task 1-4 established"), while phase 5 reorders the body and keeps the signature. Collapsing to a callbacks struct would require re-pointing plan text for a phase not yet run. Kept in Observations for the end-of-implementation pass.**
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

- The "restore window is active" rule is now written twice, once per reader — **confirmed → F1**
  ```json
  {
    "task": "resume-hooks-silently-lost-1-5",
    "source": "reviewer",
    "summary": "The \"restore window is active\" rule is now written twice, once per reader.",
    "detail": "cmd/run_hook_stale_cleanup.go:56 and cmd/doctor.go:311 both spell `if restoring, err := state.IsRestoringSet(lister); restoring || err != nil`, each under a near-identical rationale comment. The failed-read-counts-as-set rule is the load-bearing half and it exists as two copies; a change to one is silent in the other. A single unexported predicate in cmd (e.g. restoreWindowActive(lister AllPaneLister) bool) would single-source it the way the phase single-sourced the staleness rule in internal/hooks. Cross-scope: the sweep copy is task 1-3 output.",
    "files": [
      "cmd/run_hook_stale_cleanup.go",
      "cmd/doctor.go"
    ]
  }
  ```

- The restore stand-down phrase exists as two literals in one file — **confirmed → F1**
  ```json
  {
    "task": "resume-hooks-silently-lost-1-5",
    "source": "reviewer",
    "summary": "The restore stand-down phrase exists as two literals in one file.",
    "detail": "skippedPrunePhrases[skipReasonRestoring] = \"restore may be in progress\" (cmd/doctor.go:208, task 1-4) and the check's \"restore may be in progress (not evaluable)\" (cmd/doctor.go:312). Both are exactly test-pinned so a rewording cannot pass silently, but a shared base constant would make the coupling explicit rather than by copy. Deliberate for this task — the instruction was to use the literal verbatim.",
    "files": [
      "cmd/doctor.go"
    ]
  }
  ```

- Phase-1 test files accreted against the file-per-source convention — **confirmed → F4**
  ```json
  {
    "task": "resume-hooks-silently-lost-1-5",
    "source": "reviewer",
    "summary": "Phase-1 test files accreted against the file-per-source convention.",
    "detail": "cmd/hook_prune_output_test.go, cmd/hook_retention_shape_test.go, cmd/hook_sweep_restore_standdown_test.go and cmd/hook_sweep_standdown_report_test.go are four new files, each holding one or two test functions, all exercising cmd/run_hook_stale_cleanup.go and cmd/doctor.go. The project testing skill states tests are named after the source file, not the symbol; the shared fixtures (staleDeps, seedHooksJSON, restoringOption, runDoctorFixCmd) are now spread across five files. Consolidating into cmd/run_hook_stale_cleanup_test.go and cmd/doctor_test.go is a phase-boundary cleanup.",
    "files": [
      "cmd/hook_prune_output_test.go",
      "cmd/hook_retention_shape_test.go",
      "cmd/hook_sweep_restore_standdown_test.go",
      "cmd/hook_sweep_standdown_report_test.go",
      "cmd/run_hook_stale_cleanup_test.go",
      "cmd/doctor_test.go"
    ]
  }
  ```

- Two post-repair test assertions are weaker than their names promise — **confirmed → F6 (the `cmd/doctor_test.go` half only; the `cmd/hook_sweep_standdown_report_test.go:179` half fails the no-behaviour-change bar and is recorded in Observations)**
  ```json
  {
    "task": "resume-hooks-silently-lost-1-5",
    "source": "reviewer",
    "summary": "Two post-repair test assertions are weaker than their names promise.",
    "detail": "cmd/hook_sweep_standdown_report_test.go:179 (\"it reports not evaluable in the post-repair diagnosis after a stand-down\") asserts strings.Contains over the whole doctor --fix output, which carries two rendered reports; the pre-repair report alone already satisfies the positive assertion, so the subtest does not isolate the post-repair pass its name promises. Splitting the output on the second `Portal doctor:` would make the assertion match the name. Separately, cmd/doctor_test.go:1080 adds a strings.Contains check after assertRestoreWindowResult has already asserted the detail by exact equality; the second check cannot fire independently.",
    "files": [
      "cmd/hook_sweep_standdown_report_test.go",
      "cmd/doctor_test.go"
    ]
  }
  ```

## Pre-existing Debt

- Repo-wide format and modernize debt untouched by this work unit
  DETAIL: `gofmt -l .` on a clean tree reports exactly `internal/tui/help_modal_test.go` (verified at the phase boundary; no phase-1 commit touches it). `golangci-lint run ./...` reports ~31 `modernize` findings (errorsastype, stringscut, stringsseq), including `cmd/root.go:199`, `cmd/bootstrap_progress.go:133`, `main.go:64`, `main.go:75`. One sweep clears the set.
  FILES: internal/tui/help_modal_test.go, cmd/root.go, cmd/bootstrap_progress.go, main.go

- Three parallel fakes for the one `AllPaneLister` seam in package `cmd`
  DETAIL: `stubAllPaneLister` (`cmd/bootstrap_production_test.go:86-100`), `recordingHookKeyLister` (`cmd/run_hook_stale_cleanup_test.go:13-30`, a strict superset adding `hookKeyCalls`) and `fakeHookLister` (`cmd/doctor_test.go:797-808`, value receiver) all predate this phase; the phase added the same `restoring`/`restoringErr` pair and the same `TryGetServerOption` delegation to each, and two of them are now used interchangeably inside one test function (`cmd/hook_sweep_standdown_report_test.go:26` vs `:51`). The shared `restoringOption` helper single-sources the semantics, so the fresh duplication is 4 lines × 3; the three-fakes situation itself is pre-existing. Phase 2's task text re-points all three by name, so a merge is cheapest there or at the end-of-implementation pass.
  FILES: cmd/bootstrap_production_test.go, cmd/run_hook_stale_cleanup_test.go, cmd/doctor_test.go

- Two temp hooks-store seeders in package `cmd`, neither derived from the other
  DETAIL: `newTempHooksStore(t, rawJSON)` (`cmd/bootstrap_production_test.go:102`) and `seedHooksJSON(t, keys...)` (`cmd/doctor_test.go:810`) both write a `hooks.json` into a `t.TempDir()` and return `(*hooks.Store, path)`; both predate this phase and this phase consumed both heavily. A future key-shape change re-points each independently.
  FILES: cmd/bootstrap_production_test.go, cmd/doctor_test.go

## Observations

- `runHookStaleCleanup` now takes 5 parameters against the project's ≤4 convention (`.claude/skills/golang-code-style/SKILL.md:174`) — fails **plan-authorable**: the plan mandated `onSkipped` as a positional parameter and phase 3's and phase 5's task text both pin the five-argument form.
- `runHookStaleCleanup` now speaks two logging vocabularies — prose messages on the injected bootstrap/daemon logger (`cmd/run_hook_stale_cleanup.go:68,74,78,95`) and `op=`-shaped structured lines on the package `hooksLogger` (`:61,86`) — fails **no behaviour change**: unifying them alters the emitted component and message text, which the spec's logging contract and `cmd/cleanstale_transient_listpanes_shared_test.go:117-123` pin.
- `cmd/hook_sweep_standdown_report_test.go:179-188`'s "post-repair diagnosis" subtest asserts `strings.Contains` over the whole two-report `doctor --fix` output, which the pre-repair report alone satisfies — fails **no behaviour change**: splitting on the second `Portal doctor:` strengthens what the test pins rather than moving it.
- The staleness rule is stated in three doc comments in `internal/hooks/store.go` (`:182-186` unexported, `:203-206` and `:215-218` exported) — not consolidation: two are exported-API contracts the quality bar explicitly warrants, and only the unexported one carries rationale.
- `checkStaleHooks` (`cmd/doctor.go:298-329`) and `runHookStaleCleanup` keep parallel guard ladders beyond the restore read (nil/load guard → marker → enumerate → empty-live guard → judge) — deliberate, not drift: the spec has the read and write sides degrading in opposite directions and phase 5 restates it. No consolidation proposed beyond F1.
- `internal/session/tokenshape.go`'s `IsTokenShaped` reads the package-private `suffixLen` and `NanoIDAlphabet`, so its placement in `session` (with `internal/hooks` as its only consumer) is forced rather than a layering choice — nothing to consolidate.
