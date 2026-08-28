# Consolidation Tasks: resume-hooks-silently-lost (Phase 3)

## Task 1: One hydrate log-record filter, parameterised by message
placement: phase 3
severity: near-miss

**Problem**: Package `cmd` holds three helpers that filter a `logtest.Sink` for one hydrate log record, and they are the same body — iterate `sink.Records()`, keep records whose `component` attr is `"hydrate"` and whose `Msg` equals one string, fatal unless exactly one survives, return it. They differ only in that literal. `hydrateWarnRecord` (`cmd/state_hydrate_empty_hookkey_test.go:171-184`, added by task 3-3) takes the message as a parameter; `signalTimeoutRecord` (`cmd/state_hydrate_timeout_log_test.go:20-34`) and `scrollbackReplayedRecord` (`cmd/state_hydrate_replayed_log_test.go:144-158`) hardcode theirs. Task 3-3 wrote the third instance and the Rule of Three now fires exactly.

**Solution**: Keep the parameterised helper, delete the two hardcoded ones, re-point their three call sites at it.

**Outcome**: One named way to pull a single hydrate record out of a sink, sited with the other shared `cmd` hydrate test helpers rather than in the file one task happened to write it in. The three messages stay three distinct lookups — they are genuinely three different production emissions.

**Do**:
- Rename the survivor. `hydrateWarnRecord` filters on component and message only; it does not filter level, and one record it will now serve (`scrollback replayed`, `cmd/state_hydrate.go:147`) is INFO. The `Warn` in the name is false the moment it is shared. Name it for what it does — find the one hydrate record with a given message.
- Move it out of `cmd/state_hydrate_empty_hookkey_test.go` and beside the other shared `cmd` hydrate test helpers.
- Delete `signalTimeoutRecord` (`cmd/state_hydrate_timeout_log_test.go:20-34`) and `scrollbackReplayedRecord` (`cmd/state_hydrate_replayed_log_test.go:144-158`).
- Re-point the three call sites, passing the message each already filters on: `cmd/state_hydrate_timeout_log_test.go:64` (`"signal timeout"`), `cmd/state_hydrate_replayed_log_test.go:87` and `:177` (`"scrollback replayed"`), and the existing site at `cmd/state_hydrate_empty_hookkey_test.go:160`.

**Acceptance Criteria**:
- One record-filter helper remains in package `cmd` for hydrate records; the other two are gone
- Its name makes no claim about log level
- Every call site asserts on exactly the record it asserted on before — no assertion moves, no message changes
- The exactly-one-record cardinality check is preserved at every site
- `go test ./cmd/` passes; `go test -tags integration -p 1 ./cmd/...` passes

**Tests**:
- The existing coverage proves the refactor safe: `cmd/state_hydrate_timeout_log_test.go`, `cmd/state_hydrate_replayed_log_test.go` and `cmd/state_hydrate_empty_hookkey_test.go` all assert on records pulled through these helpers and must stay green unchanged
- No new test — the helper earns none of its own; it is a lookup, and its failure mode is already exercised whenever a message is wrong

## Task 2: `extractHookKey` either makes its distinction or drops it
placement: phase 3
severity: dead-code

**Problem**: Task 3-3 changed `extractHookKey` (`internal/restore/session_test.go:429-441`) from returning `string` to `(string, bool)`, replacing a `t.Fatalf("hydrate cmd %q lacks a %q token", …)` with `return "", false`, and added a comment at `:427-428` explaining that the second return "distinguishes 'no --hook-key flag at all' — what an untokened pane is armed with — from a flag carrying an empty value". The only caller discards it: `respawnPaneHookKeys` writes `key, _ := extractHookKey(t, args[4])` at `:421`. Nothing in the package consumes the bool. So the documented distinction is not made anywhere, and the same edit swapped a loud failure for a silent `""` — a pane armed with no `--hook-key` now contributes an empty string to the returned slice where it used to stop the test.

**Solution**: Consume the bool at the call site — fatal when a `respawn-pane` argv carries no `--hook-key` at all — restoring the pre-3-3 loudness and making the comment true.

**Outcome**: The signature's second return is used, the comment describes something the code does, and an argv missing the flag entirely fails loudly instead of contributing a silent empty key.

**Do**:
- At `internal/restore/session_test.go:421`, consume the second return: fatal with a message naming the argv when it is false.
- Verify no current case changes: both consumers of `respawnPaneHookKeys` — `TestSessionRestorer_HydrateBakesOneTokenPerPane` (`:333-366`) and `TestSessionRestorer_HydrateBakesKeyFromSavedStateOnly` (`:367-399`) — arm only tokened panes, so nothing is vacuous today and nothing should go red.
- If on reading the code the opposite direction is clearly better, drop the second return and the comment together instead. What must not survive is a documented distinction the code does not make. State which you took and why.

**Acceptance Criteria**:
- `extractHookKey`'s second return is either consumed or gone, along with the comment describing it
- A `respawn-pane` argv carrying no `--hook-key` fails the test loudly rather than yielding `""`
- No existing subtest changes its verdict; no assertion moves
- `go test ./internal/restore/` passes

**Tests**:
- Existing: `TestSessionRestorer_HydrateBakesOneTokenPerPane` and `TestSessionRestorer_HydrateBakesKeyFromSavedStateOnly` must stay green unchanged
- Prove the restored loudness by mutation rather than by a new test: arm a pane with no `--hook-key` and confirm the helper now fatals. Revert.

## Task 3: The reboot fixture helpers get one home in `internal/restoretest`
placement: phase 3
severity: duplication

**Problem**: Task 3-5's `cmd` fixture re-authored four helpers that already existed in `restore_test`, across a package boundary neither side can cross with a test file. The restore-under-marker bracket (`cmd/noncontiguous_window_reboot_integration_test.go:417-426`) duplicates `restoreWithMarker` (`internal/restore/integration_test.go:17-29`) — same set / `Restore()` / unset of `@portal-restoring`. The captured-session lookup (`:492-503`) duplicates `findCapturedSession` (`internal/restore/rename_reboot_shared_test.go:43-54`) — same scan, same collected-names fatal. The scrollback seeder (`:530-541`) duplicates `seedPaneScrollback` (`internal/restore/multipane_legacy_integration_test.go:213-223`) and `seedScrollback` (`rename_reboot_shared_test.go:78-88`) — same `SanitizePaneKey` → `ScrollbackFile` → `MkdirAll` → `WriteFile`, differing only in payload bytes. The index persist (`:355-361`) inlines what `persistIndex` (`rename_reboot_shared_test.go:67-76`) and the unexported `writeIndex` (`internal/restoretest/sessions_json.go:67-79`, extracted by task 3-1) already do.

**Solution**: Move the four into `internal/restoretest` as exported siblings of the reboot scaffolding both packages already import — `BuildPortalBinaryDir`, `DriveSignalHydrate`, `WaitForSkeletonMarkersCleared`, `SortedKeySet` — and re-point both the `cmd` fixture and the `restore_test` files at them.

**Outcome**: One home for the reboot-fixture vocabulary, reachable from both packages. A future change to how a restore is bracketed, a captured session found, scrollback seeded or an index persisted is one edit, not four.

**Do**:
- **Build-tag constraint, load-bearing**: `internal/restoretest/restoretest.go` is `//go:build integration`, but `internal/restore/integration_test.go` — a caller of `restoreWithMarker` — carries no tag. The shared marker bracket must therefore land in an **untagged** `restoretest` file. `logger.go`, `sessions_json.go` and `waitfor_file_exists.go` are the untagged precedents; follow them.
- Move the restore-under-marker bracket, keeping the pre-existing `defer` + `t.Logf` unset shape from `restoreWithMarker` so no existing failure path moves. The `cmd` copy returns the unset error, which no assertion distinguishes from a restore error today — both land on the same `t.Fatalf`.
- Move the captured-session lookup, taking the session name as a parameter (the `cmd` copy hardcodes it).
- Move the scrollback seeder, taking the payload as a parameter. That subsumes the pre-existing `seedScrollback` / `seedPaneScrollback` pair in the same move — `seedScrollback(t, dir, name)` is exactly `seedPaneScrollback(t, dir, name, 0, 0)`.
- Export `writeIndex` as the shared index writer; have `persistIndex` and the `cmd` fixture's inline tail both call it.
- Re-point every call site and delete the duplicates.

**Acceptance Criteria**:
- The four helpers exist once each, in `internal/restoretest`, and both packages call them
- The marker bracket sits in an untagged file, and `go test ./internal/restore/` (unit lane, untagged) still compiles and passes
- No failure path moves: the marker-unset error is reported exactly as it is today on each side
- The `seedScrollback` / `seedPaneScrollback` pair is subsumed rather than left beside the new helper
- No assertion changes and no test changes its verdict
- `go test ./...` passes; `go test -tags integration -p 1 ./...` passes

**Tests**:
- Existing coverage proves it safe: `TestNonContiguousWindowReboot_KeepsTokenKeyedHooks` (`cmd`), the rename-reboot family and `TestMultiPaneLegacy_PerPaneHookRouting` (`internal/restore`), and the untagged `internal/restore/integration_test.go` suite must all stay green unchanged
- No new tests — these are fixture helpers, and their failure modes are exercised by every caller

## Task 4: One name for reading a pane token in fixtures
placement: phase 3
severity: near-miss

**Problem**: Writing a pane token in a fixture has a shared home — `(*Socket).StampPaneToken` (`internal/tmuxtest/stamp.go:11-14`), which task 2-7 consolidated eight sites onto. Reading one does not. There are two shapes: `readPaneToken` (`internal/restore/rename_reboot_shared_test.go:23-33`, added by task 3-1) does a `show-options -p -t <target> -v` read returning `""` on non-zero exit, called from `rename_reboot_hook_integration_test.go:114` and `rename_reboot_durability_integration_test.go:95`; and an inline `display-message -p -t <target> '#{@portal-pane-id}'` at `cmd/state_daemon_hook_cleanup_integration_test.go:88-90`, written on the line directly after that fixture's own `StampPaneToken` call. The second uses the mechanism task 3-5 established as unreliable for this purpose: `display-message -p -t <target no pane answers to>` does not fail — tmux falls back to the session's current pane and exits 0, with or without the `=` prefix. It is benign at that call site, where the target was just created and the value is compared against an expected token, but it is the wrong default for the next fixture to copy.

**Solution**: Add `(*Socket).ReadPaneToken(t, target) string` to `internal/tmuxtest` beside `StampPaneToken`, implemented as the `show-options -p -t <target> -v` read, and re-point both sites.

**Outcome**: Reading and writing a pane token in a fixture are one named pair in one package, and the read that discriminates a bad target is the one a new fixture reaches for.

**Do**:
- Add the helper beside `StampPaneToken`, taking a pane **target** rather than a session name — `readPaneToken` hardcodes pane `0.0`, which is why it takes a session name today.
- Implement it as shape A's `show-options -p -v` read. Carry over its existing rationale comment about an unset option exiting non-zero, since that is exactly why this form and not `display-message`.
- Re-point `internal/restore`'s two callers, passing `tmux.PaneTarget(name, 0, 0)` explicitly, and delete `readPaneToken`.
- Re-point the inline read at `cmd/state_daemon_hook_cleanup_integration_test.go:88-90`.
- **Leave these alone — they are correct as they are.** `internal/tmux/resolve_hookkey_realtmux_test.go:81`, `:98`, `:103` assert on raw tmux exit status and message text and must not route through a helper that swallows either. `(*divergentRebootFixture).livePaneRows` (`cmd/noncontiguous_window_reboot_integration_test.go:444-462`) is a deliberate single whole-server `ListAllPanesWithFormat` enumeration; a per-pane `-t` read is wrong there for the reason in the Problem.

**Acceptance Criteria**:
- `internal/tmuxtest` exposes one pane-token read beside the write, taking a pane target
- `readPaneToken` and the inline `display-message` read are both gone
- The three raw-tmux assertions in `internal/tmux/resolve_hookkey_realtmux_test.go` and the `livePaneRows` enumeration are untouched
- No assertion changes its verdict
- `go test ./...` passes; `go test -tags integration -p 1 ./...` passes

**Tests**:
- Existing: the rename-reboot family and `cmd/state_daemon_hook_cleanup_integration_test.go` must stay green unchanged
- No new test — it is a fixture read, exercised by every caller

## Task 5: `tmux.PaneTarget`'s comment states the rule the package actually follows
placement: phase 3
severity: comments

**Problem**: `internal/tmux/tmux.go:404-405` says "PaneTarget formats a plain 'session:window.pane' target, for display and name-based keys. Anything issuing a tmux `-t` flag must use PaneTargetExact." The package's own production code does not follow it. `internal/restore/session.go:129` builds `liveTarget` with `tmux.PaneTarget`, then passes it to `RespawnPane` at `:139` (`respawn-pane -k -t`) and to `SetPaneOption` at `:155` (`set-option -p -t`) — and that second call is the stamp task 3-2 added, so phase 3 put a second production `-t` issuer on a non-exact target. `internal/tmuxtest/stamp.go:13` does the same, with five callers. The accurate rule is already stated one function down at `:416-418` (`exactTarget`), where the named hazard is *session-level* prefix matching: `-t foo` silently resolving to a live `foo-2` once `foo` is gone.

**Solution**: Reword `PaneTarget`'s comment to say when exactness actually matters — a `-t` naming a session that may have been renamed or destroyed, where tmux's prefix match can silently retarget a different session — instead of the blanket prohibition the package does not honour.

**Outcome**: The comment describes the rule the code follows, so the next reader of a `PaneTarget` call site is not left deciding whether five test fixtures and two production lines are all wrong.

**Do**:
- Reword the comment at `internal/tmux/tmux.go:404-405`. Take the substance from `exactTarget`'s comment at `:416-418`, which already states the real hazard correctly.
- Pure comment edit. No call site moves, no behaviour changes, no test changes.
- **Do not "fix" the call sites instead.** Adding an `=` prefix to every `-t` pane target is a behaviour change and is not this pass's work. If reading the code convinces you the call sites are genuinely wrong, say so and stop — that is a plan decision, not a consolidation.

**Acceptance Criteria**:
- `PaneTarget`'s doc comment states when exactness matters rather than prohibiting what the package does
- No non-comment line changes anywhere
- `go test ./...` and `go test -tags integration -p 1 ./...` pass, unchanged

**Tests**:
- None. A comment edit changes no behaviour; the existing suites passing unchanged is the whole proof.

## Bank Disposition

- Pre-existing repo-wide lint/format debt outside this task's surface. — residue — pre-existing repo-wide lint/format debt; no phase-3 file involved

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

- runHookStaleCleanup now takes 5 parameters, one over the project convention. — residue — phase 5 reworks this call chain — the callbacks-struct decision belongs with it

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

- The restore-window rule is now single-sourced in cmd, but the daemon states the inverse error policy twice with no shared name. — residue — daemon lifecycle, outside phase 3 surface

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

- A third reader of the same failed-read-counts-as-set posture sits in cmd, in a file the daemon entry does not name. — residue — same ground as the sibling entry above; outside phase 3 surface

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

- TestDoctorFixPrunedHookOutput is now fully subsumed by a sibling test. — residue — phase-1 ground; deleting a case needs an owner who can approve the count change

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

- Four further cmd test files touching doctor.go / run_hook_stale_cleanup.go still carry concern-derived names. — residue — pre-existing naming convention across surviving files; phase 3 did not touch it

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

- staleSeed is now declared twice, byte-identical, in one file. — residue — phase-1 ground, outside phase 3 surface

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

- The removal leaves two byte-identical subtests in TestDoctorStaleHooksCheck. — residue — phase-1 ground, outside phase 3 surface

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

- The gone-pane message reaching the user is three wraps deep, and Phase 4 reworks the same call site for hook rm wording. — residue — phase 4 reworks the same call site

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

- A saver-hosting integration fixture omits the teardown guard CLAUDE.md prescribes for exactly its shape. — residue — phase-2 ground, outside phase 3 surface

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

- stubAllPaneLister is the last piece of the hook-sweep seam vocabulary still sited with a single consumer. — residue — phase-2 ground, outside phase 3 surface

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

- Package cmd now holds two helper-only test files that want to be one. — residue — phase-2 ground; phase 4 may collapse the two files anyway

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

- Two package-level names in package cmd now hold one key value. — residue — phase-2 ground, outside phase 3 surface

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

- A few hook-file preamble sites outside the three hooks test files could take the bare helper now that it exists in the shared file. — residue — phase-2 ground, outside phase 3 surface

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

- The two-helper-file rule is stated in only one of the two files, and phase 4 could silently collapse alias coverage. — residue — phase 4 could silently collapse them; belongs with phase 4

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

- The last hand-rolled level filter now lives in internal/spawn, and it is not the same filter. — residue — internal/spawn, outside phase 3 surface

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

- The same five-property audit-breadcrumb block is still written inline outside this task two helpers. — residue — outside phase 3 surface

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

- Two near-duplicate TMUX_PANE-unset subtests in TestHooksSetCommand, one a strict subset of the other. — residue — phase-2 ground, outside phase 3 surface

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

- Restore the capture-driven second reboot cycle once the production pane re-stamp exists. — mooted — task 3-2 restored the capture-driven second cycle under the approved plan amendment; verified in internal/restore/rename_reboot_durability_integration_test.go:70-101

  ```json
  {
    "task": "resume-hooks-silently-lost-3-1",
    "source": "reviewer",
    "summary": "Restore the capture-driven second reboot cycle once the production pane re-stamp exists.",
    "detail": "internal/restore/rename_reboot_durability_integration_test.go:63-83 previously ran cycle 2 from a fresh CaptureStructure taken after cycle 1 restore, asserting the identity was re-persisted — the end-to-end proof that a hook survives two reboots in a row via live capture, which is this work unit headline guarantee. It now replays the same snapshot (persistIndex at :74, re-writing bytes already on disk), so cycle 2 no longer exercises capture-after-restore. Not repairable at 3-1: production has no pane re-stamp until 3-2. Task 3-2 tests are all unit-level against the mock Commander and 3-5 covers the post-restore sweep, so neither restores the composite. After 3-2 lands, revert cycle 2 to state.CaptureStructure -> assert capturedPaneToken == renamePaneToken -> persistIndex -> reboot, keeping assertHookFireCount(t, hookFireFile, 2).",
    "files": [
      "internal/restore/rename_reboot_durability_integration_test.go",
      "internal/restore/session.go"
    ]
  }
  ```

- One name for reading a pane token in fixtures, as StampPaneToken is for writing one. — folded into task 4

  ```json
  {
    "task": "resume-hooks-silently-lost-3-1",
    "source": "reviewer",
    "summary": "One name for reading a pane token in fixtures, as StampPaneToken is for writing one.",
    "detail": "StampPaneToken gave the write one home, but the read has two ad-hoc shapes: show-options -p -t <target> -v <opt> at internal/restore/rename_reboot_shared_test.go:22-30, and an inline display-message -p -t <target> #{...} at cmd/state_daemon_hook_cleanup_integration_test.go:89-90. Both want the pane token, or empty if unstamped, which one tmuxtest.Socket.ReadPaneToken sibling would serve. The fix crosses task 3-1 boundary — the cmd site is a phase-2 task output. Also noted: four new fixture tokens (alphaPaneToken, betaPaneToken, mpPaneToken0/1, exitClosesPaneToken) are not token-shaped under session.IsTokenShaped, inert today because none of those fixtures runs a stale sweep, but worth watching if a later phase adds one to those paths.",
    "files": [
      "internal/tmuxtest/stamp.go",
      "internal/restore/rename_reboot_shared_test.go",
      "cmd/state_daemon_hook_cleanup_integration_test.go"
    ]
  }
  ```

- assertHookFireCount reads the hook-fire file with no wait, racing the hydrate helper across the whole restore hook-fire family. — residue — remedy adds a poll — removes a race rather than preserving behaviour, so not a pure refactor

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

- Unit-lane real-tmux restore tests silently depend on whichever portal is installed; root cause is a bare portal in the hydrate argv. — residue — root-cause fix changes the production hydrate argv and re-tags a suite; plan-authorable, surfaced for decision

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

- Three near-identical logtest.Record filter helpers in cmd. — folded into task 1

  ```json
  {
    "task": "resume-hooks-silently-lost-3-3",
    "source": "reviewer",
    "summary": "Three near-identical logtest.Record filter helpers in cmd.",
    "detail": "hydrateWarnRecord (cmd/state_hydrate_empty_hookkey_test.go:171) is the parameterised form of signalTimeoutRecord (cmd/state_hydrate_timeout_log_test.go:20) and scrollbackReplayedRecord (cmd/state_hydrate_replayed_log_test.go:144) — same loop, same component==hydrate filter, same exactly-one fatal, differing only in the hardcoded message. The new one is the third instance, so Rule of Three fires; the other two collapse into it cleanly but live in files task 3-3 does not own.",
    "files": [
      "cmd/state_hydrate_empty_hookkey_test.go",
      "cmd/state_hydrate_timeout_log_test.go",
      "cmd/state_hydrate_replayed_log_test.go"
    ]
  }
  ```

- CLAUDE.md never describes the shape-aware reaper or internal/session new token surfaces, so the retain-old-format-forever rule is undocumented. — residue — caused by phases 1-2; tasks 4-2 and 5-1 already prescribe edits to the same paragraph

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

- Four integration tests exec portal state hydrate from the ambient PATH instead of staging a binary — residue — same root cause as entry 24; plan-authorable, surfaced for decision

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

- The restore-under-marker bracket is written twice, in two packages that cannot share a test file — folded into task 3

  ```json
  {
    "task": "resume-hooks-silently-lost-3-5",
    "summary": "The restore-under-marker bracket is written twice, in two packages that cannot share a test file",
    "detail": "restoreUnderMarker (cmd/noncontiguous_window_reboot_integration_test.go:394-403) duplicates restoreWithMarker (internal/restore/integration_test.go:17-29) — same set/restore/unset of @portal-restoring around o.Restore(). internal/restoretest is the natural home (both callers already import it) and would also be the place for the hydrate-and-await trio these reboot tests repeat.",
    "files": [
      "cmd/noncontiguous_window_reboot_integration_test.go",
      "internal/restore/integration_test.go",
      "internal/restoretest/restoretest.go"
    ]
  }
  ```

- tmux.PaneTarget doc comment forbids what ~15 test call sites do — folded into task 5

  ```json
  {
    "task": "resume-hooks-silently-lost-3-5",
    "summary": "tmux.PaneTarget doc comment forbids what ~15 test call sites do",
    "detail": "internal/tmux/tmux.go:404-405 states anything issuing a tmux -t flag must use PaneTargetExact, yet tmuxtest.StampPaneToken (internal/tmuxtest/stamp.go:13) issues set-option -p -t <target> and every caller passes the non-exact PaneTarget — cmd/rename_restore_cleanup_survival_integration_test.go:40, cmd/bootstrap/reboot_roundtrip_test.go:106, internal/restore/multipane_legacy_integration_test.go:81-82, internal/restore/rename_reboot_hook_integration_test.go:107, and cmd/noncontiguous_window_reboot_integration_test.go:287. Either the doc comment is over-broad (the exact-match hazard is session-name prefix matching, which single-session fixtures cannot hit) or the call sites are wrong. Both readings cannot be right, and the next reviewer hits the same ambiguity.",
    "files": [
      "internal/tmux/tmux.go",
      "internal/tmuxtest/stamp.go",
      "cmd/rename_restore_cleanup_survival_integration_test.go",
      "cmd/bootstrap/reboot_roundtrip_test.go",
      "internal/restore/multipane_legacy_integration_test.go",
      "internal/restore/rename_reboot_hook_integration_test.go"
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

