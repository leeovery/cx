# Consolidation Tasks: Resume Hooks Silently Lost (Phase 1)

## Task 1: Single-source the restore-window rule and its phrase
placement: phase 1
severity: duplication

**Problem**: The restore-window predicate is written twice, once per reader. `cmd/run_hook_stale_cleanup.go:53-64` and `cmd/doctor.go:307-313` both spell `if restoring, err := state.IsRestoringSet(lister); restoring || err != nil`, each under a near-identical rationale comment. The load-bearing half is the failed-read-counts-as-set rule, and a change to one copy is silent in the other. Separately, the stand-down phrase is two literals in one file: `skippedPrunePhrases[skipReasonRestoring] = "restore may be in progress"` (`cmd/doctor.go:208`) and `"restore may be in progress (not evaluable)"` (`cmd/doctor.go:312`), the same phrase re-typed with a suffix.

**Solution**: One unexported predicate in `cmd` holding the rule and its rationale exactly once, and one base phrase constant the two renderings compose from.

**Outcome**: The rule and the phrase each exist once. Both call sites keep their current position in their guard ladders, so `checkStaleHooks`'s order (store-nil → load → restore marker → enumeration) is untouched, and every existing assertion passes unchanged.

**Do**:
- Add `func restoreWindowActive(checker state.RestoringChecker) bool` to `cmd/run_hook_stale_cleanup.go`, returning `restoring || err != nil` from `state.IsRestoringSet(checker)`. Move the failed-read-counts-as-set rationale comment onto it verbatim — one copy, on the predicate.
- Call it from `runHookStaleCleanup` at its current position, and from `checkStaleHooks` (`cmd/doctor.go`) at its current position. Delete the second rationale comment; leave a one-line pointer at each call site only if the site is unreadable without one.
- Add `const restoreStandDownPhrase = "restore may be in progress"` beside `skippedPrunePhrases` in `cmd/doctor.go`. Use it as the map value, and compose the check's detail as `restoreStandDownPhrase + " (not evaluable)"`.
- Change nothing else: not the read's position, not the returned strings, not the log lines, not a single test.

**Acceptance Criteria**:
- [ ] `state.IsRestoringSet` is called from exactly one place in package `cmd`
- [ ] The failed-read-counts-as-set rationale comment exists exactly once
- [ ] `restore may be in progress` appears as one string literal in `cmd`; both renderings derive from it
- [ ] `checkStaleHooks` still reads the marker after the store guards and before the enumeration
- [ ] Every existing test passes with no assertion, fixture or name changed
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- The existing coverage proves the refactor safe unchanged: `TestHookSweepStandsDownWhileRestoring`, `TestHookSweepReportsStandDown`, `TestDoctorFixReportsSkippedHookPrune` and `TestDoctorStaleHooksCheck`'s restore-window subtests already pin both call sites' behaviour and both exact strings.
- No new test: the extracted predicate has no behaviour of its own beyond what those suites already exercise from both sides.

## Task 2: Correct the claims the shape rule narrowed
placement: phase 1
severity: comments

**Problem**: Three prose claims were made false-in-part by phase 1's own changes, and each sits in a region a task edited without touching it. `cmd/run_hook_stale_cleanup.go:80-81` says an empty live set reaching `CleanStale` "would delete every entry" — after task 1-1 it deletes only judgeable keys and retains the rest; task 1-4 rewrote the WARN on the very next line. `cmd/doctor.go:295-297` says an unreadable or empty live set "would otherwise report every entry stale" — `checkStaleHooks` now counts only judgeable keys, pinned by `cmd/doctor_test.go:1106-1115`; task 1-5 edited the function directly beneath it. `CLAUDE.md:100` says cycle summaries emit "one INFO per cycle with per-item detail at DEBUG" — task 1-2 promoted the hooks clean-stale per-key line to INFO, making the reaper a counterexample to the generalisation, and no plan task owns that passage.

**Solution**: Narrow each claim to what is now true. Prose only.

**Outcome**: No comment or doc line in the phase's surface overstates a hazard the shape rule already narrowed, and `CLAUDE.md`'s logging section describes what the reaper actually emits.

**Do**:
- `cmd/run_hook_stale_cleanup.go:80-81`: narrow the claim to the judgeable subset — an empty live set reaching `CleanStale` would delete every key it can judge. Do not re-argue the guard; the hazard is real, only its scope moved.
- `cmd/doctor.go:295-297`: narrow the same way — an unreadable or empty live set would report every judgeable entry stale.
- `CLAUDE.md:100`: either add the hooks clean-stale per-key INFO to the instrumented bullet, or qualify the per-item-at-DEBUG generalisation to the components it still describes. Pick whichever reads as one sentence rather than a caveat.
- Touch no code, no test, no assertion.

**Acceptance Criteria**:
- [ ] Neither in-source comment claims a scope wider than the staleness rule now permits
- [ ] `CLAUDE.md`'s logging bullet is true of the hooks clean-stale sweep
- [ ] The diff contains no non-comment source change
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- None. Comment and documentation text carries no test surface; the existing suites prove nothing changed.

## Task 3: A shared test vocabulary for hook-key seeds
placement: phase 1
severity: duplication

**Problem**: After task 1-1 a seed key's *shape* silently decides whether a test exercises reaping or retention, and roughly eighteen sites each spell their own literal — `sessA1`, `keyA00`, `gone01`, `stalA0`, `alpha1`, `smoke1` and the rest across `cmd/doctor_test.go`, `cmd/run_hook_stale_cleanup_test.go`, the four new `cmd/hook_*_test.go` files, `cmd/state_daemon_hook_cleanup_test.go`, `cmd/state_daemon_run_test.go`, five integration seeds, and `internal/hooks/store_{test,shape_test}.go`. A literal that drifts to seven bytes, or gains a `-`, flips a reap test into a retention test and stays green for the wrong reason. Nothing enforces the shape at any site.

**Solution**: Name the two roles instead of spelling them, in one shared declaration all three surfaces can reach, with the shape asserted at construction.

**Outcome**: A mis-shaped seed is impossible: the reapable constructor asserts `session.IsTokenShaped` on the value it returns. Every seed site names the role it needs, and the next re-point is one edit rather than eighteen.

**Do**:
- Add `ReapableHookKey(n int) string` and `UnjudgeableHookKey(n int) string` to `internal/transienttest`, beside `SeedHooksJSON`, so `cmd`, `cmd/bootstrap` and `internal/hooks` all reach one declaration. `ReapableHookKey` must produce a value satisfying `session.IsTokenShaped` and panic (or fail via a `*testing.T` parameter, matching the package's existing shape) if it does not — the assertion is the point of the helper.
- Re-point every seed site listed in `consolidation-findings-p1.md` F3 to the appropriate constructor. A seed whose key must match a live pane keeps its positional literal — the live enumeration is still positional in this phase, and converting one would invert that test's meaning.
- Change no assertion, no expected count, no test name. Values may change (a constructor's output need not equal the old literal) provided every assertion that named a literal is re-pointed with it.

**Acceptance Criteria**:
- [ ] Both constructors live in one package reachable from `cmd`, `cmd/bootstrap` and `internal/hooks`
- [ ] `ReapableHookKey` fails loudly if its output is not token-shaped
- [ ] No hand-rolled token-shaped hook-key literal survives at a seed site
- [ ] Live-set-coupled positional keys are unchanged
- [ ] Every test keeps its current name, cases and expected counts
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- The existing hook-cleanup, retention and doctor suites are the proof: they pass unchanged in count and meaning after the re-point.
- One new test for `ReapableHookKey`'s own guarantee: its output is token-shaped, and successive `n` values differ.

## Task 4: Consolidate the phase's cmd test files and fixtures
placement: phase 1
severity: drift

**Problem**: Each phase-1 task added its own test file named off the concern rather than the source, so `cmd/run_hook_stale_cleanup.go` and `cmd/doctor.go` are now tested from six files between them — `cmd/hook_prune_output_test.go` (34 lines, one subtest), `cmd/hook_retention_shape_test.go`, `cmd/hook_sweep_restore_standdown_test.go`, `cmd/hook_sweep_standdown_report_test.go`, plus the two source-named files. The `hook_` prefix points at `cmd/hooks.go`, which is not the source under test. The phase's fixtures are scattered across five files, with `restoringOption` declared in one file and consumed by three fakes living in three others. Two derived duplications follow: `assertStalePrunesApplied` asserts the pruned line by substring while `cmd/hook_prune_output_test.go` asserts the same output by exact equality, and `seedStalePruneFixture` has been inlined twice more (`cmd/hook_sweep_restore_standdown_test.go:139-144`, `cmd/hook_sweep_standdown_report_test.go:222-244`) solely to swap the lister.

**Solution**: Merge the four concern-named files into the two source-named ones, park each helper beside what it serves, parameterise the one fixture the three variants differ on, and keep the stronger of the two duplicated assertions.

**Outcome**: Every test of these two sources lives in a file named after its source; one fixture serves all three lister variants; the pruned-line output is asserted once, at its exact-equality strength.

**Do**:
- Move the contents of `cmd/hook_prune_output_test.go`, `cmd/hook_retention_shape_test.go`, `cmd/hook_sweep_restore_standdown_test.go` and `cmd/hook_sweep_standdown_report_test.go` into `cmd/run_hook_stale_cleanup_test.go` and `cmd/doctor_test.go` by which source each exercises, and delete the four files. If a merged file grows unwieldy, split only under a source-derived name (`doctor_stale_hooks_test.go`).
- Move `restoringOption` beside the fakes it serves.
- Give `seedStalePruneFixture` a lister parameter so the two inlined copies collapse into it.
- Fold the exact-equality pruned-line assertion into `assertStalePrunesApplied`, replacing its substring check; drop the now-redundant standalone assertion. Verify the three existing `assertStalePrunesApplied` call sites still pass under the stricter form — if one legitimately cannot, keep the substring check for that site and say why in a comment rather than weakening the shared helper.
- No test gains or loses a case, and no test name changes except where a merge would collide.

**Acceptance Criteria**:
- [ ] The four concern-named files are gone; every test they held survives with its cases intact
- [ ] Every remaining test file in `cmd` touching these two sources is named after a source file
- [ ] `seedStalePruneFixture` is the only implementation of that fixture
- [ ] The pruned-line output is asserted in one place, by exact equality
- [ ] The unit-lane test count for package `cmd` is unchanged
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- Pure movement: the existing cases are the coverage, and their survival is the acceptance criterion.
- No new test.

## Task 5: One deny-the-write fixture in internal/hooks
placement: phase 1
severity: near-miss

**Problem**: `internal/hooks/store_test.go` now holds three ways to deny a write: `readOnlyDirPath` (`:27-37`), a hand-rolled seed-then-`chmod 0500` block at `:1024-1039`, and a second one at `:936-948` added by task 1-2 — which carries the same two-line explanatory comment verbatim. The phase's addition is what took this to a triplicate.

**Solution**: One helper that seeds, denies, and registers the restoring cleanup.

**Outcome**: The seed-then-deny sequence exists once, with its explanation attached to it rather than copied beside each use.

**Do**:
- Add `seedThenDenyWrites(t *testing.T, body []byte) (*hooks.Store, string)` to `internal/hooks/store_test.go`: write the seed, `chmod 0500` the parent directory, register a `t.Cleanup` restoring the mode, return the store and path.
- Re-point both hand-rolled blocks to it, including the pre-existing one at `:1024` — the phase's addition is what made it a triplicate, so it is in scope.
- Move the shared explanatory comment onto the helper; delete both copies.
- Leave `readOnlyDirPath` as it is if it serves a different case; fold it in only if it does not.

**Acceptance Criteria**:
- [ ] One implementation of the seed-then-deny sequence in `internal/hooks/store_test.go`
- [ ] Both subtests still assert the same failure, error class and file state
- [ ] The cleanup restores the directory mode on every path
- [ ] `go test ./...` passes

**Tests**:
- The two save-failure subtests are the proof; both keep their current assertions.

## Task 6: Remove inert test scaffolding and a subsumed assertion
placement: phase 1
severity: dead-code

**Problem**: Three pieces of the phase's test surface do nothing. `cmd/state_daemon_hook_cleanup_test.go:128-129` and `:154-155` each construct a named `logtest.Sink`, install it, and never read it — neither test's path emits on the `hooks` component (the first returns before any `hooksLogger` call, the second logs on the injected daemon logger only). `cmd/doctor_test.go:1086-1088` checks `strings.Contains(got.detail, "stale hook entr")` after `assertRestoreWindowResult` has already asserted `got.detail` by exact equality, so it cannot fire independently.

**Solution**: Delete all three. Where the intent was to silence rather than capture, say so with an unnamed sink.

**Outcome**: No test holds scaffolding a reader would mistake for an assertion.

**Do**:
- At `cmd/state_daemon_hook_cleanup_test.go:128-129` and `:154-155`, drop the named sink. If the install was silencing output rather than capturing it, replace it with an unnamed `log.SetTestHandler(t, &logtest.Sink{})` so the intent reads. Leave the third install at `:213-214` — it is read at `:233`.
- Drop the subsumed `strings.Contains` at `cmd/doctor_test.go:1086-1088`.
- Remove nothing else: in each case the surviving assertion must already subsume what is removed.

**Acceptance Criteria**:
- [ ] No `logtest.Sink` is bound to a name it never reads
- [ ] No assertion is removed whose failure the surviving assertions would not also catch
- [ ] Every affected test keeps its current name and cases
- [ ] `go test ./...` passes

**Tests**:
- Removals only; the surviving assertions in each affected subtest are the coverage.

## Bank Disposition

- Hook-key test seeds are hand-rolled literals across a dozen files — folded into task 3
  ```json
  {"task":"resume-hooks-silently-lost-1-1","source":"executor","summary":"Hook-key test seeds are hand-rolled literals across a dozen files, and every later phase re-points all of them again.","detail":"Key shape is now semantically load-bearing in tests (token-shaped = reapable, anything else = retained), but each site spells its own literal: internal/hooks/store_test.go, cmd/run_hook_stale_cleanup_test.go:157-161,209-213, cmd/doctor_test.go:854,903,946,963,980,997,1054-1057,1100-1101,1231,1302,1455, cmd/state_daemon_hook_cleanup_test.go, cmd/state_daemon_run_test.go:529,566, plus five integration seeds. A shared test-side vocabulary (reapable key / unjudgeable old-format key helper next to internal/transienttest.SeedHooksJSON) would make the next re-point one edit and make a mis-shaped seed impossible.","files":["internal/hooks/store_test.go","cmd/run_hook_stale_cleanup_test.go","cmd/doctor_test.go","cmd/doctor_summary_test.go","cmd/doctor_fix_theme_test.go","cmd/state_daemon_hook_cleanup_test.go","cmd/state_daemon_run_test.go","cmd/cleanstale_transient_listpanes_shared_test.go","cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go","cmd/state_daemon_hook_cleanup_integration_test.go","cmd/bootstrap/transient_listpanes_helpers_integration_test.go","internal/transienttest/hooks.go"]}
  ```

- Two mass-deletion-guard rationale comments now overstate their hazard — folded into task 2
  ```json
  {"task":"resume-hooks-silently-lost-1-1","source":"reviewer","summary":"Two mass-deletion-guard rationale comments now overstate their hazard, in files this task deliberately did not touch.","detail":"cmd/run_hook_stale_cleanup.go:41 says an empty live set reaching CleanStale would delete every entry, and cmd/doctor.go:278 says it would otherwise report every entry stale. Post-change both are true only of judgeable entries — on today's install, where every key is old-format, an empty live set would now delete nothing. The claims are narrowed rather than flatly false, and both regions are rewritten by the later work (the restoring check moves into runHookStaleCleanup, the onSkipped callback and the lock-timeout branch land alongside), so the correction belongs with that task rather than as a drive-by edit here.","files":["cmd/run_hook_stale_cleanup.go","cmd/doctor.go"]}
  ```

- CLAUDE.md logging bullet now misdescribes the hooks clean-stale sweep — folded into task 2
  ```json
  {"task":"resume-hooks-silently-lost-1-2","source":"reviewer","summary":"CLAUDE.md logging bullet now misdescribes the hooks clean-stale sweep.","detail":"CLAUDE.md:100 states cycle summaries emit one INFO per cycle with per-item detail at DEBUG. After this change the hooks clean-stale per-item detail is INFO. No task in the plan owns this passage — phases 2/3/4/5 each assign specific CLAUDE.md rewrites (hook-key scheme, @portal-id removal, hook rm exit code, lock sidecar) and none touches the logging section. It will otherwise ship false.","files":["CLAUDE.md"]}
  ```

- The doctor --fix prune-stdout assertion now exists twice, at two strengths — folded into task 4
  ```json
  {"task":"resume-hooks-silently-lost-1-2","source":"reviewer","summary":"The doctor --fix prune-stdout assertion now exists twice, at two strengths.","detail":"cmd/doctor_test.go:888 (inside the shared assertStalePrunesApplied, used by three call sites) asserts strings.Contains(out, \"Pruned stale hook: sessA1\"); the new cmd/hook_prune_output_test.go:29 asserts exactly-one-line equality over the same fixture shape that seedStalePruneFixture (doctor_test.go:850) already builds. Folding the exact assertion into the shared helper would single-source it and let the standalone file go, but it reaches a helper three other tests depend on.","files":["cmd/hook_prune_output_test.go","cmd/doctor_test.go"]}
  ```

- Third instance of the deny-the-write fixture in internal/hooks/store_test.go — folded into task 5
  ```json
  {"task":"resume-hooks-silently-lost-1-2","source":"reviewer","summary":"Third instance of the deny-the-write fixture in internal/hooks/store_test.go.","detail":"readOnlyDirPath (:27-37) plus two hand-rolled seed-then-chmod-0500 blocks — the pre-existing one at :1024-1035 and the new one at :936-947, which even carries the same two-line comment. Rule of Three now applies: a seedThenDenyWrites(t, body) helper would single-source it, but the fix touches a pre-existing subtest outside this task's diff.","files":["internal/hooks/store_test.go"]}
  ```

- The doctor --fix fixture is being re-implemented per stand-down reason instead of parameterised — folded into task 4
  ```json
  {"task":"resume-hooks-silently-lost-1-3","source":"reviewer","summary":"The doctor --fix fixture is being re-implemented per stand-down reason instead of parameterised — 1-4 and 1-5 will each want another variant.","detail":"cmd/hook_sweep_restore_standdown_test.go:138-144 inlines a copy of seedStalePruneFixture (cmd/doctor_test.go:856-869) solely to swap in fakeHookLister{restoring: true}. Tasks 1-4 (onSkipped phrase lines) and 1-5 (checkStaleHooks not-evaluable) will each need the same fixture with a different lister, so the natural consolidation is one fixture taking the lister as a parameter, once all three exist.","files":["cmd/hook_sweep_restore_standdown_test.go","cmd/doctor_test.go"]}
  ```

- Pre-existing repo-wide lint/format debt outside this task's surface — residue — wholly pre-existing; no phase-1 commit touches any named file. Rides to the end-of-implementation analysis.
  ```json
  {"task":"resume-hooks-silently-lost-1-4","source":"executor","summary":"Pre-existing repo-wide lint/format debt outside this task's surface.","detail":"gofmt -l . reports internal/tui/help_modal_test.go as unformatted on a clean tree (not touched by this work), and golangci-lint run ./... reports 31 modernize findings across the repo (errorsastype, stringscut, stringsseq), 9 of them in cmd — e.g. cmd/root.go:199, cmd/bootstrap_progress.go:133, main.go:64, main.go:75. All predate this work unit; a single sweep would clear them.","files":["internal/tui/help_modal_test.go","cmd/root.go","cmd/bootstrap_progress.go","main.go"]}
  ```

- runHookStaleCleanup now takes 5 parameters — residue — plan-authored, not consolidation: phase 3's task text pins the five-argument call form verbatim and phase 5 keeps the signature, so collapsing to a callbacks struct would require re-pointing plan text for a phase not yet run. Rides to the end-of-implementation analysis.
  ```json
  {"task":"resume-hooks-silently-lost-1-4","source":"reviewer","summary":"runHookStaleCleanup now takes 5 parameters, one over the project convention.","detail":".claude/skills/golang-code-style/SKILL.md:174 sets <=4 parameters, options-struct beyond. The task mandated onSkipped as a positional parameter, so it is correct as delivered; folding onRemoved/onSkipped (and whatever the lock phase needs) into a small callbacks struct would touch cmd/run_hook_stale_cleanup.go:37-43 plus ~14 call sites across files owned by sibling tasks 1-1 through 1-3.","files":["cmd/run_hook_stale_cleanup.go","cmd/doctor.go","cmd/state_daemon.go","cmd/run_hook_stale_cleanup_test.go","cmd/hook_sweep_restore_standdown_test.go","cmd/hook_retention_shape_test.go","cmd/hookkey_no_regression_upgrade_test.go","cmd/rename_restore_cleanup_survival_integration_test.go","cmd/hook_sweep_standdown_report_test.go"]}
  ```

- The "restore window is active" rule is now written twice, once per reader — folded into task 1
  ```json
  {"task":"resume-hooks-silently-lost-1-5","source":"reviewer","summary":"The \"restore window is active\" rule is now written twice, once per reader.","detail":"cmd/run_hook_stale_cleanup.go:56 and cmd/doctor.go:311 both spell `if restoring, err := state.IsRestoringSet(lister); restoring || err != nil`, each under a near-identical rationale comment. The failed-read-counts-as-set rule is the load-bearing half and it exists as two copies; a change to one is silent in the other. A single unexported predicate in cmd (e.g. restoreWindowActive(lister AllPaneLister) bool) would single-source it the way the phase single-sourced the staleness rule in internal/hooks. Cross-scope: the sweep copy is task 1-3 output.","files":["cmd/run_hook_stale_cleanup.go","cmd/doctor.go"]}
  ```

- The restore stand-down phrase exists as two literals in one file — folded into task 1
  ```json
  {"task":"resume-hooks-silently-lost-1-5","source":"reviewer","summary":"The restore stand-down phrase exists as two literals in one file.","detail":"skippedPrunePhrases[skipReasonRestoring] = \"restore may be in progress\" (cmd/doctor.go:208, task 1-4) and the check's \"restore may be in progress (not evaluable)\" (cmd/doctor.go:312). Both are exactly test-pinned so a rewording cannot pass silently, but a shared base constant would make the coupling explicit rather than by copy. Deliberate for this task — the instruction was to use the literal verbatim.","files":["cmd/doctor.go"]}
  ```

- Phase-1 test files accreted against the file-per-source convention — folded into task 4
  ```json
  {"task":"resume-hooks-silently-lost-1-5","source":"reviewer","summary":"Phase-1 test files accreted against the file-per-source convention.","detail":"cmd/hook_prune_output_test.go, cmd/hook_retention_shape_test.go, cmd/hook_sweep_restore_standdown_test.go and cmd/hook_sweep_standdown_report_test.go are four new files, each holding one or two test functions, all exercising cmd/run_hook_stale_cleanup.go and cmd/doctor.go. The project testing skill states tests are named after the source file, not the symbol; the shared fixtures (staleDeps, seedHooksJSON, restoringOption, runDoctorFixCmd) are now spread across five files. Consolidating into cmd/run_hook_stale_cleanup_test.go and cmd/doctor_test.go is a phase-boundary cleanup.","files":["cmd/hook_prune_output_test.go","cmd/hook_retention_shape_test.go","cmd/hook_sweep_restore_standdown_test.go","cmd/hook_sweep_standdown_report_test.go","cmd/run_hook_stale_cleanup_test.go","cmd/doctor_test.go"]}
  ```

- Two post-repair test assertions are weaker than their names promise — folded into task 6 (the cmd/doctor_test.go half; the cmd/hook_sweep_standdown_report_test.go half strengthens what a test pins and is recorded in the findings' Observations)
  ```json
  {"task":"resume-hooks-silently-lost-1-5","source":"reviewer","summary":"Two post-repair test assertions are weaker than their names promise.","detail":"cmd/hook_sweep_standdown_report_test.go:179 (\"it reports not evaluable in the post-repair diagnosis after a stand-down\") asserts strings.Contains over the whole doctor --fix output, which carries two rendered reports; the pre-repair report alone already satisfies the positive assertion, so the subtest does not isolate the post-repair pass its name promises. Splitting the output on the second `Portal doctor:` would make the assertion match the name. Separately, cmd/doctor_test.go:1080 adds a strings.Contains check after assertRestoreWindowResult has already asserted the detail by exact equality; the second check cannot fire independently.","files":["cmd/hook_sweep_standdown_report_test.go","cmd/doctor_test.go"]}
  ```
