# Consolidation Tasks: Resume Hooks Silently Lost (Phase 2)

## Task 1: Compose the enumeration's token half from the registration format
placement: phase 2
severity: duplication

**Problem**: The phase's central invariant is that registration and enumeration read the *same* pane option, and the expression that does so exists as two independent literals 28 lines apart. `HookKeyFormat` (`internal/tmux/tmux.go:584`, task 2-2) is `"#{" + state.PortalPaneIDOption + "}"`; `paneHookRowFormat` (`:612-613`, task 2-1) opens with the same composition. The shared constant guarantees only that both name the same option, not that both wrap it identically, so the agreement is held by a real-tmux assertion rather than by construction. Separately, the location half at `:613` restates `StructuralKeyFormat` (`:579`) verbatim 34 lines below it — deliberate, because the location is display-only and must not couple to the structural key, but nothing at the site says so and a later reader will either couple them or delete one as a copy-paste.

**Solution**: Build the enumeration format from the registration format, and record why the location half does not reuse its neighbour.

**Outcome**: The two reads agree by construction. The deliberate non-reuse of `StructuralKeyFormat` is stated where a reader meets it.

**Do**:
- Rewrite `paneHookRowFormat` as `HookKeyFormat + paneHookRowSeparator + "#{session_name}:#{window_index}.#{pane_index}"`. The resulting string is byte-identical, so no behaviour changes and no test re-points.
- Add one line above it recording that the location half deliberately does not reuse `StructuralKeyFormat`: the location is never a key, so coupling the two would tie a display column to a key contract.
- Change nothing else in the file.

**Acceptance Criteria**:
- [ ] `paneHookRowFormat` derives its token half from `HookKeyFormat` rather than recomposing it
- [ ] The rendered format string is unchanged — every existing test passes untouched
- [ ] The non-reuse of `StructuralKeyFormat` is stated at the site
- [ ] Both lanes pass

**Tests**:
- The existing coverage proves it safe: `internal/tmux/hookkey_cross_site_realtmux_test.go` already asserts the two reads answer with the same token for the same pane, and `internal/tmux/pane_hook_rows_test.go` asserts the composed argv.
- No new test — the change is a constant refactor with an identical result.

## Task 2: One real-tmux fixture preamble and one way to stamp a pane
placement: phase 2
severity: near-miss

**Problem**: Every task in the phase added or rewrote a real-tmux fixture, and all five open with the same `tmuxtest.New` → `Client()` → `EnsureServer` → `NewSession` → `WaitForSession` block, diverging only in the topology seeded after it: `seedThreePaneStampedSession` (`internal/tmux/hookkey_realtmux_shared_test.go:16-36` — the file that exists to be the shared one), `liveHookKeyPane` (`resolve_hookkey_realtmux_test.go:16-35`), `newStampedPaneFixture` (`hookkey_moved_pane_realtmux_test.go:139-172`), plus inline blocks in `hookkey_cross_site_realtmux_test.go:17-34` (whose topology is *exactly* `seedThreePaneStampedSession`'s), `list_all_pane_hookkeys_realtmux_test.go` and `hookkey_format_realtmux_test.go`. Drift has already set in: only one of them omits `EnsureServer`, and the socket prefixes are `ptl-xsite-` / `ptl-hookkeys-` / `ptl-hookkey-` / `ptl-panemove-` / `hookkey-`. Separately there are **two ways to stamp a pane with no stated rule** — raw tmux via `stampPaneToken` (`list_all_pane_hookkeys_realtmux_test.go:73-76`) and `client.SetPaneOption` at four other sites — and the split does not track "avoid the subject under test in setup": the enumeration test uses raw while the format test, equally not testing `SetPaneOption`, uses the client. `cmd` holds three more raw copies of the same argv.

**Solution**: One parameterised seeder in the shared file, and one stamp helper on the test socket.

**Outcome**: A reader meets one preamble and one stamp route. No suite stages its fixture through the call it is testing.

**Do**:
- Add a parameterised seeder to `internal/tmux/hookkey_realtmux_shared_test.go` — the common preamble plus a topology argument — and re-point `seedThreePaneStampedSession`, `liveHookKeyPane`, `newStampedPaneFixture` and the four inline blocks at it. Each caller keeps its own assertions and its own token; `newStampedPaneFixture` keeps its `renumber-windows on` line, which is genuinely its own.
- Give the seeder one socket-prefix convention and apply it to every caller.
- Promote the stamp to `internal/tmuxtest` as a `(*Socket).StampPaneToken(t, target, token)` raw-tmux method — `tmuxtest` already imports `internal/tmux`, which imports `internal/state`, so the option constant is reachable with no new cycle — and re-point all eight sites: the five in `internal/tmux` and the three in `cmd` (`state_daemon_hook_cleanup_integration_test.go:86`, `cleanstale_transient_listpanes_doctorfix_integration_test.go:101`, `rename_restore_cleanup_survival_integration_test.go:40`).
- Leave `resolve_hookkey_realtmux_test.go:111`'s raw `set-option` exactly as it is — that one asserts tmux's own exit status rather than staging a fixture.
- Touch no assertion.

**Acceptance Criteria**:
- [ ] One server-bootstrap preamble serves every hook-key real-tmux fixture
- [ ] Every fixture calls `EnsureServer`; socket prefixes follow one convention
- [ ] `StampPaneToken` is the single way a test stamps a pane token, at all eight staging sites
- [ ] The exit-status assertion at `resolve_hookkey_realtmux_test.go:111` still uses raw tmux
- [ ] No test gains or loses a case, and no assertion changes
- [ ] Both lanes pass

**Tests**:
- Pure movement: the existing real-tmux suites are the coverage, and their unchanged passing is the criterion.
- No new test.

## Task 3: Site the row vocabulary with the seeds, and close the literal bypasses
placement: phase 2
severity: drift

**Problem**: Task 1-9 deliberately gathered the `cmd` hook-key test vocabulary into `cmd/run_hook_stale_cleanup_test.go:25-70`. Phase 2 reopened the scatter. `tokenRows` and `unstampedRows` (`cmd/bootstrap_production_test.go:115-129`) are the row builders every re-pointed assertion in the phase composes with `liveSeedA..C` — used **once** in their own file and **43 times elsewhere**, so a reader tracing `tokenRows(liveSeedB)` crosses three files, one of which has nothing to do with the sweep. Three hand-written token literals bypass the drift guard in files that already import `transienttest`: `liveHookToken = "livetk"` (`cmd/state_daemon_hook_cleanup_integration_test.go:45`, one line below a `ReapableHookKey` call), `liveKey = "livetk"` (`cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:100`), `renameRestoreToken = "tokrst"` (`cmd/rename_restore_cleanup_survival_integration_test.go:18`) — each now stamped into a real pane, so each measures liveness only while it stays token-shaped, which is exactly what `ReapableHookKey` exists to guarantee. And `"live:0.0"` (`cmd/run_hook_stale_cleanup_test.go:788`, `:865`) is named *live* while surviving purely on unjudgeability — under this phase's own change no live pane can carry that shape, so the name asserts the opposite of the reason the entry survives.

**Solution**: Move the row builders beside the seeds they are always used with, and route every remaining hand-written key through the vocabulary.

**Outcome**: One place declares what a test hook key is. A width or alphabet change fails loudly at every fixture instead of silently emptying three of them.

**Do**:
- Move `tokenRows` / `unstampedRows` to `cmd/run_hook_stale_cleanup_test.go` beside the seed vocabulary, or move the whole block to a shared `cmd` test-helper file taking `restoringOption` with it — whichever leaves fewer cross-file reaches.
- Re-point the three integration literals at `transienttest.ReapableHookKey(n)`.
- Re-point the two `"live:0.0"` literals at `unjudgeableSeedB`.
- Values stay token-shaped and legacy-shaped respectively, so every assertion keeps its current meaning. This is naming and siting only.

**Acceptance Criteria**:
- [ ] The row builders sit with the seed vocabulary they compose with
- [ ] No hand-written token literal survives at a fixture that stamps or seeds one
- [ ] No fixture named for liveness survives on unjudgeability
- [ ] Every test keeps its name, cases and expected counts
- [ ] Both lanes pass

**Tests**:
- The existing sweep, doctor and daemon suites are the proof — unchanged in count and meaning after the re-point.
- No new test.

## Task 4: One home for the hook-set test setup
placement: phase 2
severity: duplication

**Problem**: Task 2-2 wrote `hooksFileInTempDir` and `runHookSet` (`cmd/hooks_pane_token_test.go:37-51`); task 2-4, two commits later, added eight subtests that open-code them. `cmd/hooks_test.go` now spells the temp-dir-plus-env preamble at 33 sites and the reset-and-execute block at 30, eight of the former written in this phase while the helper existed in the sibling file. The coupling already runs the other way: `runHookSetForKey` (`cmd/hooks_test.go:825-834`) calls task 2-2's helper across the file boundary. Phase 4 reworks this file for `hook rm` wording and the `hook list` location column, so the count grows again before anything shrinks.

**Solution**: Move both helpers where all three hooks test files already reach, and re-point the open-coded sites.

**Outcome**: One declaration of how a `hook set` test is staged, in the file the package's other shared test helpers live in.

**Do**:
- Move `hooksFileInTempDir` and `runHookSet` to `cmd/testhelpers_test.go` beside `writeHooksJSON` / `readHooksJSON`.
- Return the temp dir alongside the file path — three of the phase's own new sites need `dir` for their own purposes (staging a blocker file, a locked state dir, a directory at the hooks path) and cannot use a path-only helper.
- Re-point the open-coded sites.
- No subtest gains or loses an assertion.

**Acceptance Criteria**:
- [ ] Both helpers live in the shared test-helper file
- [ ] The temp-dir-plus-env preamble and the reset-and-execute block are not open-coded where the helper serves
- [ ] Sites needing the directory itself still get it
- [ ] The unit-lane test count for package `cmd` is unchanged
- [ ] Both lanes pass

**Tests**:
- Pure movement: the existing cases are the coverage.
- No new test.

## Task 5: One level filter and one hooks-record assertion
placement: phase 2
severity: near-miss

**Problem**: Two tasks of this phase each needed "assert one `hooks`-component record" and each wrote it. `standDownRecord` (`cmd/run_hook_stale_cleanup_test.go:598-627`, task 2-1) asserts level, message, component, `op`, `via` and `reason`; `assertTouchWarn` (`cmd/hooks_test.go:857-887`, task 2-4) asserts the same first five under a different expected level, op and via, plus a non-empty `error`. The level filter is likewise written twice — `warnRecords` (`cmd/hooks_test.go:836-844`) and an inline scan at `cmd/run_hook_stale_cleanup_test.go:600-604` — and both copies exist because `internal/logtest`, which owns every other typed accessor, has no level filter. A third `op` arrives in a later phase and will want the same assertions again.

**Solution**: Put the level filter where the other accessors live, and share the record-shape assertion.

**Outcome**: The record shape is asserted in one place; a third consumer adds a call rather than a third copy.

**Do**:
- Add `func (s *Sink) RecordsAtLevel(min slog.Level) []Record` to `internal/logtest/capture.go` beside `Records` / `OnlyRecord`, and have both `cmd` sites call it.
- Extract one `assertHooksRecord(t, rec, wantLevel, wantOp, wantVia)` in `cmd` carrying the level / message / component / op / via block, called from both helpers; each keeps its own extra assertion (`reason` for the stand-down, non-empty `error` for the touch).
- Pure extraction — every assertion that runs today still runs.

**Acceptance Criteria**:
- [ ] `internal/logtest` owns the level filter; neither `cmd` site reimplements it
- [ ] One assertion carries the shared record shape, called from both helpers
- [ ] Each helper keeps the assertion that is its own
- [ ] No assertion is dropped
- [ ] `go test ./...` passes

**Tests**:
- The stand-down and touch-failure subtests are the proof; both keep their current assertions.
- No new test.

## Task 6: One way to reach the hooks seams
placement: phase 2
severity: drift

**Problem**: Task 2-2 added two seams to `HooksDeps` and gave each a named accessor, leaving the pre-existing third as an inline branch — so `cmd/hooks.go` now holds both shapes of one pattern. `KeyResolver` is resolved inline inside `resolveCurrentPaneKey` (`:58-63`), while `hooksPaneStamper()` (`:73-78`) and `hooksTokenMinter()` (`:80-85`) are accessors. The package convention is the named accessor and it is `buildX` — `buildHooksTmuxClient` (`:46`), `buildAckWriter` (`cmd/open.go:268-274`), `buildThemeLoader` (`cmd/open.go:489-494`) — so the two new accessors follow the convention's shape but not its name.

**Solution**: Extract the third accessor and bring all three onto the package's naming convention.

**Outcome**: All three seams are reached the same way, under the name the rest of the package uses. `resolveCurrentPaneKey`'s body is the two reads it is named for.

**Do**:
- Extract `buildHookKeyResolver() HookKeyResolver` holding the inline branch.
- Rename `hooksPaneStamper` / `hooksTokenMinter` to `buildPaneStamper` / `buildTokenMinter`.
- Unexported, no seam semantics change, no test re-points — tests set `hooksDeps` fields and never call the accessors.

**Acceptance Criteria**:
- [ ] All three seams are reached through named accessors following the package's `buildX` convention
- [ ] `resolveCurrentPaneKey` no longer holds the resolver branch
- [ ] No seam semantics change and no test is re-pointed
- [ ] Both lanes pass

**Tests**:
- The existing `hook set` / `hook rm` suites are the coverage, including the injected-seam cases that drive every accessor's non-default arm.
- No new test.

## Task 7: One unresolvable-pane subtest
placement: phase 2
severity: duplication

**Problem**: `cmd/hooks_pane_token_test.go:236-262` ("it exits non-zero from hook set on an unresolvable pane", task 2-3) and `cmd/hooks_test.go:315-340` ("it aborts hooks set when the hook-key read fails") share their whole fixture — hooks file in a temp dir, `TMUX_PANE` set, a failing `mockKeyResolver`, `hook set --on-resume` — and both assert a non-nil error and that `hooks.json` was never created. They diverge only in which property of the error each checks: the pre-existing one a substring, the new one the typed error plus tmux's own words surviving in the message.

**Solution**: One subtest carrying all four assertions.

**Outcome**: The unresolvable-pane contract is asserted once, at the union of both strengths.

**Do**:
- Fold the two into one subtest in `cmd/hooks_test.go` beside the other `hook set` failure cases, carrying every assertion both currently make — the union is strictly stronger than either.
- Delete the redundant one.
- Note for scope: phase 4 reworks `hook rm` messaging in the same file but does not reach this `hook set` pair, so it will not absorb this on its own.

**Acceptance Criteria**:
- [ ] One subtest covers the unresolvable-pane case
- [ ] Every assertion either currently makes still runs
- [ ] Both lanes pass

**Tests**:
- The surviving subtest is the coverage.
- No new test.

## Bank Disposition

- Three hand-written token literals bypass the ReapableHookKey drift guard in files that already import it — folded into task 3 (the literals and the `"live:0.0"` naming) and task 1 (the `paneHookRowFormat` half)
  ```json
  {"task":"resume-hooks-silently-lost-2-1","source":"reviewer","summary":"Three hand-written token literals bypass the ReapableHookKey drift guard in files that already import it.","detail":"liveHookToken = \"livetk\" (cmd/state_daemon_hook_cleanup_integration_test.go:45, one line above staleHookKey = transienttest.ReapableHookKey(0)), liveKey = \"livetk\" (cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:101) and renameRestoreToken = \"tokrst\" (cmd/rename_restore_cleanup_survival_integration_test.go:42). All three are token-shaped today and all three genuinely measure liveness, but they bypass the panic-on-drift the helper exists for; a suffixLen/alphabet change would turn each into a vacuous pass. Also: \"live:0.0\" at cmd/run_hook_stale_cleanup_test.go:788 and :865 is named live while surviving purely on unjudgeability — unjudgeableSeedB would read straight. And paneHookRowFormat intentionally restates the location shape rather than reusing StructuralKeyFormat (spec 3.3), but nothing at internal/tmux/tmux.go:594 says so and the two constants sit ~40 lines apart — a plausible future simplification target.","files":["cmd/state_daemon_hook_cleanup_integration_test.go","cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go","cmd/rename_restore_cleanup_survival_integration_test.go","cmd/run_hook_stale_cleanup_test.go","internal/tmux/tmux.go"]}
  ```

- The hooks CLI test setup is now duplicated 28 ways beside two helpers that encapsulate it — folded into task 4
  ```json
  {"task":"resume-hooks-silently-lost-2-2","source":"reviewer","summary":"The hooks CLI test setup is now duplicated 28 ways beside two helpers that encapsulate it.","detail":"cmd/hooks_pane_token_test.go:34-48 adds hooksFileInTempDir and runHookSet, which capture exactly the shape cmd/hooks_test.go open-codes 28 times (filepath.Join(t.TempDir(), \"hooks.json\") + t.Setenv(\"PORTAL_HOOKS_FILE\", ...)) and 29 times (resetRootCmd() + SetOut/SetErr/SetArgs). The consolidation reaches into subtests this task had no reason to touch, so it belongs at the phase boundary — and the helpers likely belong beside writeHooksJSON/readHooksJSON in cmd/testhelpers_test.go rather than in a feature-specific file. Phase 2 adds more hook set / hook rm cases in 2-3 and 4-2, so the count grows before it shrinks.","files":["cmd/hooks_pane_token_test.go","cmd/hooks_test.go","cmd/testhelpers_test.go"]}
  ```

- The new cmd propagation subtest substantially re-covers a pre-existing one — folded into task 7
  ```json
  {"task":"resume-hooks-silently-lost-2-3","source":"reviewer","summary":"The new cmd propagation subtest substantially re-covers a pre-existing one; consolidation reaches a file this task did not touch.","detail":"cmd/hooks_pane_token_test.go:238-262 (it exits non-zero from hook set on an unresolvable pane) and cmd/hooks_test.go:310-334 (it aborts hooks set when the hook-key read fails) share their whole setup and their error-plus-no-file assertions; the new one adds only the errors.As and tmux-words checks. Phase 4 is already reworking hook rm messaging in cmd/hooks_test.go, so folding the two is cheapest there.","files":["cmd/hooks_pane_token_test.go","cmd/hooks_test.go"]}
  ```

- A level-filtering accessor belongs in logtest beside OnlyRecord — folded into task 5
  ```json
  {"task":"resume-hooks-silently-lost-2-4","source":"reviewer","summary":"A level-filtering accessor belongs in logtest beside OnlyRecord.","detail":"warnRecords (cmd/hooks_test.go:839) and the inline `if r.Level >= slog.LevelWarn` scan in standDownRecord (cmd/run_hook_stale_cleanup_test.go:598-605) are the same filter written twice, in two files owned by different tasks of this phase. internal/logtest/capture.go already owns the typed accessors (OnlyRecord, AttrString, HasAttr) and has no level filter; a Sink.RecordsAtLevel(min slog.Level) []Record would serve both and any third consumer.","files":["internal/logtest/capture.go","cmd/hooks_test.go","cmd/run_hook_stale_cleanup_test.go"]}
  ```

- Two near-parallel hooks-component log-record assertion helpers in cmd tests — folded into task 5
  ```json
  {"task":"resume-hooks-silently-lost-2-4","source":"reviewer","summary":"Two near-parallel hooks-component log-record assertion helpers in cmd tests.","detail":"assertTouchWarn (cmd/hooks_test.go:857) and standDownRecord (cmd/run_hook_stale_cleanup_test.go:598, landed by task 2-1) both assert the same record shape — level, message, component=hooks, op, via — differing only in expected level and op. Spec 6.5 adds a third op (load-unlocked) still to come, which will want the same assertions again. One assertHooksRecord(t, rec, wantLevel, wantOp, wantVia) would carry all three. Consolidation edits another task output, so it was not an issue at review time.","files":["cmd/hooks_test.go","cmd/run_hook_stale_cleanup_test.go"]}
  ```

- Three near-miss real-tmux fixture builders across the hook-key suites share an identical server-bootstrap preamble — folded into task 2
  ```json
  {"task":"resume-hooks-silently-lost-2-5","source":"reviewer","summary":"Three near-miss real-tmux fixture builders across the hook-key suites share an identical server-bootstrap preamble.","detail":"newStampedPaneFixture (internal/tmux/hookkey_moved_pane_realtmux_test.go:139), liveHookKeyPane (internal/tmux/resolve_hookkey_realtmux_test.go:16) and seedThreePaneStampedSession (internal/tmux/hookkey_realtmux_shared_test.go:16) each open with the same tmuxtest.New -> Client() -> EnsureServer -> NewSession -> WaitForSession block and diverge only in the seeded topology. stampPaneToken (internal/tmux/list_all_pane_hookkeys_realtmux_test.go:73) is a raw-tmux twin of client.SetPaneOption used elsewhere in the same suites. A single parameterised seeder in the shared file would collapse all four; the fix reaches into sibling tasks files. Also minor: token literals in the moved-pane suite are tokMove/tokRespawn/tokSplit (7-10 chars), not token-shaped under the 6-char rule the sibling suite uses (tokMix) — irrelevant at the tmux layer but non-uniform.","files":["internal/tmux/hookkey_moved_pane_realtmux_test.go","internal/tmux/resolve_hookkey_realtmux_test.go","internal/tmux/hookkey_realtmux_shared_test.go","internal/tmux/list_all_pane_hookkeys_realtmux_test.go"]}
  ```

- The shared cmd hook-key vocabulary landed in the file F4 already flags as the fixture-scatter site — mooted — task 1-9's merge already moved the block; the surviving concern is the row builders, which is task 3's
  ```json
  {"task":"resume-hooks-silently-lost-1-8","source":"reviewer","summary":"The shared cmd hook-key vocabulary landed in the file F4 already flags as the fixture-scatter site.","detail":"The reapableSeedA..D / unjudgeableSeedA..B block sits at cmd/doctor_test.go:812-823 but is consumed from ten cmd test files, most of which exercise cmd/run_hook_stale_cleanup.go and cmd/state_daemon.go rather than cmd/doctor.go — e.g. cmd/run_hook_stale_cleanup_test.go:43, cmd/state_daemon_run_test.go:531, cmd/hook_retention_shape_test.go:15, cmd/hook_sweep_restore_standdown_test.go:24. It joins staleDeps / seedHooksJSON / fakeHookLister / runDoctorFixCmd, which finding F4 already banks for a file-merge pass. The vocabulary block should travel with that merge (to cmd/run_hook_stale_cleanup_test.go or a source-derived doctor_stale_hooks_test.go) rather than be moved on its own.","files":["cmd/doctor_test.go","cmd/run_hook_stale_cleanup_test.go","cmd/hook_prune_output_test.go","cmd/hook_retention_shape_test.go","cmd/hook_sweep_restore_standdown_test.go","cmd/hook_sweep_standdown_report_test.go","cmd/state_daemon_run_test.go","cmd/state_daemon_hook_cleanup_test.go"]}
  ```

- The two real-tmux enumeration tests now overlap and want merging once 2-2 restores the cross-site comparison — mooted — task 2-2 restored the comparison and the overlap resolved into the predicted end state; the shared-preamble half survives in task 2
  ```json
  {"task":"resume-hooks-silently-lost-2-1","source":"reviewer","summary":"The two real-tmux enumeration tests now overlap and want merging once 2-2 restores the cross-site comparison.","detail":"internal/tmux/hookkey_cross_site_realtmux_test.go:13 (TestPaneTokenEnumeration_PerPaneTokensAreDistinct) and internal/tmux/list_all_pane_hookkeys_realtmux_test.go:13 (TestListAllPaneHookKeys_StampedAndUnstampedInOneRead) each spin a real server, stamp panes and assert the stamped/unstamped token split; the former is the narrowed remnant of a cross-site test that 2-2 re-points at ResolveHookKey versus the enumeration. Consolidating now would collide with that task; consolidating after it would leave one focused enumeration test plus one genuine cross-site test. The file name also no longer matches its single test subject.","files":["internal/tmux/hookkey_cross_site_realtmux_test.go","internal/tmux/list_all_pane_hookkeys_realtmux_test.go"]}
  ```

- Pre-existing repo-wide lint/format debt outside this task's surface — residue
- runHookStaleCleanup now takes 5 parameters — residue (plan-authored; phases 3 and 5 pin the five-argument form)
- Three parallel fakes for the one AllPaneLister seam in package cmd — residue (pre-existing; phase 3 re-points them again)
- Two temp hooks-store seeders in package cmd — residue (pre-existing)
- The restore-window rule is single-sourced in cmd, but the daemon states the inverse error policy twice — residue (ground outside this phase)
- A third reader of the same failed-read-counts-as-set posture sits in cmd — residue (same ground)
- TestDoctorFixPrunedHookOutput is now fully subsumed by a sibling test — residue (phase-1-caused; deleting it costs a case)
- Four further cmd test files touching doctor.go / run_hook_stale_cleanup.go still carry concern-derived names — residue (remainder pre-existing)
- staleSeed is now declared twice, byte-identical, in one file — residue (phase-1-caused)
- The removal leaves two byte-identical subtests in TestDoctorStaleHooksCheck — residue (phase-1-caused)
- The gone-pane message reaching the user is three wraps deep — residue (phase 4 owns the same call site; trimming a wrap is not a pure refactor)
