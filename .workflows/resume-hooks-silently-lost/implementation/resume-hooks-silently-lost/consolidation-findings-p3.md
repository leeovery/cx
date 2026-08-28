# Consolidation Findings: resume-hooks-silently-lost (Phase 3)

## Findings

### F1: Three hydrate log-record filters in package `cmd`, one of them the parameterised form of the other two

- **Class**: near-miss
- **Evidence**:
  - `cmd/state_hydrate_empty_hookkey_test.go:171-184` — `hydrateWarnRecord(t, sink, msg)`, added by task 3-3; sole call site `:160`
  - `cmd/state_hydrate_timeout_log_test.go:20-34` — `signalTimeoutRecord(t, sink)`, message hardcoded to `"signal timeout"`; call site `:64`
  - `cmd/state_hydrate_replayed_log_test.go:144-158` — `scrollbackReplayedRecord(t, sink)`, message hardcoded to `"scrollback replayed"`; call sites `:87`, `:177`
  - All three are the same body: iterate `sink.Records()`, keep records whose `component` attr is `"hydrate"` and whose `Msg` equals one string, fatal unless exactly one survives, return it. They differ only in that literal.
  - The three messages are real and distinct in production — `cmd/state_hydrate.go:147` (`scrollback replayed`, INFO), `:210` (`timeout waiting for hydrate signal`, WARN), `:212` (`signal timeout`, INFO) — so the three helpers are legitimately three lookups, not three claims.
- **Proposed shape**: keep the parameterised helper, delete the two hardcoded ones, re-point their three call sites. Rename on the way: `hydrateWarnRecord` filters on component and message only — it does not filter level, and one of the records it would now serve (`scrollback replayed`) is INFO — so the `Warn` in the name is false the moment it is shared. Site the survivor beside the other shared `cmd` hydrate test helpers rather than in the 3-3 file. No assertion moves: each call site keeps the record it already gets, and the exactly-one cardinality check is identical in all three.
- **Bank**: "Three near-identical logtest.Record filter helpers in cmd" (3-3 reviewer) — confirmed verbatim; the Rule of Three it names fires exactly.

### F2: `extractHookKey`'s new second return is discarded by its only caller, and the same edit swapped a loud failure for a silent empty string

- **Class**: dead-code
- **Evidence**:
  - `internal/restore/session_test.go:429-441` — `extractHookKey(t, hydrate) (string, bool)`; task 3-3 changed the signature from `string` and replaced `t.Fatalf("hydrate cmd %q lacks a %q token", …)` with `return "", false` at `:433-434`
  - `internal/restore/session_test.go:427-428` — the comment added to justify it: "The second return distinguishes 'no --hook-key flag at all' - what an untokened pane is armed with - from a flag carrying an empty value"
  - `internal/restore/session_test.go:421` — the sole caller, inside `respawnPaneHookKeys` (`:413-425`), writes `key, _ := extractHookKey(t, args[4])`; nothing consumes the bool anywhere in the package
  - Consumers of `respawnPaneHookKeys`: `TestSessionRestorer_HydrateBakesOneTokenPerPane` (`:333-366`) and `TestSessionRestorer_HydrateBakesKeyFromSavedStateOnly` (`:367-399`). Both arm only tokened panes, so nothing is vacuous today — but a pane armed with no `--hook-key` now contributes a silent `""` to the returned slice where it used to stop the test.
- **Proposed shape**: consume the bool at `:421` — fatal when a `respawn-pane` argv carries no `--hook-key` at all, which restores the pre-3-3 loudness and makes the comment true. Every current consumer arms only tokened panes, so no case changes and no assertion moves. If the reviewer prefers the other direction, drop the second return and the comment together; what must not survive is a documented distinction the code does not make.
- **Bank**: none.

### F3: Task 3-5's `cmd` fixture re-authors four `restore_test` reboot helpers across the package boundary

- **Class**: duplication
- **Evidence**:
  - restore-under-marker bracket: `cmd/noncontiguous_window_reboot_integration_test.go:417-426` (`restoreUnderMarker`) vs `internal/restore/integration_test.go:17-29` (`restoreWithMarker`) — same set/`Restore()`/unset of `@portal-restoring`
  - captured-session lookup: `cmd/noncontiguous_window_reboot_integration_test.go:492-503` (`(*divergentRebootFixture).capturedSession`) vs `internal/restore/rename_reboot_shared_test.go:43-54` (`findCapturedSession`) — same scan, same collected-names fatal; the `cmd` copy hardcodes the session name
  - scrollback seeder: `cmd/noncontiguous_window_reboot_integration_test.go:530-541` (`divergentSeedScrollback`) vs `internal/restore/multipane_legacy_integration_test.go:213-223` (`seedPaneScrollback`) and `internal/restore/rename_reboot_shared_test.go:78-88` (`seedScrollback`) — same `SanitizePaneKey` → `ScrollbackFile` → `MkdirAll` → `WriteFile`, differing only in the payload bytes
  - index persist: `cmd/noncontiguous_window_reboot_integration_test.go:355-361` (inline `EncodeIndex` + `WriteFile(state.SessionsJSON(...))`) vs `internal/restore/rename_reboot_shared_test.go:67-76` (`persistIndex`) and the unexported `writeIndex` that task 3-1 extracted at `internal/restoretest/sessions_json.go:67-79`
- **Proposed shape**: move the four into `internal/restoretest` as exported siblings of the reboot scaffolding both packages already import (`BuildPortalBinaryDir`, `DriveSignalHydrate`, `WaitForSkeletonMarkersCleared`, `SortedKeySet`); re-point the `cmd` fixture and the `restore_test` files at them. Details that decide the task:
  - keep the pre-existing `defer` + `t.Logf` unset shape from `restoreWithMarker` so no existing failure path moves. The `cmd` copy returns the unset error, which no assertion distinguishes from a restore error today (both land on the same `t.Fatalf`).
  - the scrollback seeder takes the payload as a parameter, which subsumes the intra-`restore_test` `seedScrollback`/`seedPaneScrollback` pair (pre-existing, see Pre-existing Debt) in the same move.
  - export `writeIndex` as the shared index writer and have `persistIndex` and the `cmd` tail call it.
  - build-tag constraint: `internal/restoretest/restoretest.go` is `//go:build integration`, but `internal/restore/integration_test.go` — a caller of `restoreWithMarker` — carries no tag, so the shared marker bracket must land in an untagged `restoretest` file. `logger.go`, `sessions_json.go` and `waitfor_file_exists.go` are the untagged precedents.
- **Bank**: "The restore-under-marker bracket is written twice, in two packages that cannot share a test file" (3-5) — confirmed, and widened here: the bracket is one of four, not the only one.

### F4: Reading a pane token has no shared home while writing one does

- **Class**: near-miss
- **Evidence**:
  - the write has a home: `internal/tmuxtest/stamp.go:11-14` — `(*Socket).StampPaneToken(t, target, token)`, `set-option -p -t <target> @portal-pane-id <token>`
  - read shape A: `internal/restore/rename_reboot_shared_test.go:23-33` — `readPaneToken(t, ts, sessionName)`, a `show-options -p -t <PaneTarget(name,0,0)> -v @portal-pane-id` read returning `""` on non-zero exit; added by task 3-1. Call sites `internal/restore/rename_reboot_hook_integration_test.go:114` and `internal/restore/rename_reboot_durability_integration_test.go:95`
  - read shape B: `cmd/state_daemon_hook_cleanup_integration_test.go:88-90` — an inline `display-message -p -t <target> '#{@portal-pane-id}'`, written on the line directly after that fixture's own `sock.StampPaneToken(...)` call at `:87`
  - shape B uses the mechanism task 3-5 established as unreliable for this purpose: `display-message -p -t <target no pane answers to>` does not fail — tmux falls back to the session's current pane and exits 0 (the reason for the comment at `cmd/noncontiguous_window_reboot_integration_test.go:437-443`). Benign at that call site, where the target was just created and the value is compared against the expected token, but it is the wrong default for the next fixture to copy.
- **Proposed shape**: `(*Socket).ReadPaneToken(t, target) string` in `internal/tmuxtest` beside `StampPaneToken`, taking a pane *target* rather than a session name (shape A hardcodes pane `0.0`), implemented as shape A's `show-options -p -t <target> -v` read with its existing rationale comment about an unset option exiting non-zero. Re-point both sites; `internal/restore`'s two callers pass `tmux.PaneTarget(name, 0, 0)` explicitly. Explicitly out of scope, and must stay as they are:
  - `internal/tmux/resolve_hookkey_realtmux_test.go:81`, `:98`, `:103` — those assert on raw tmux exit status and message text and must not route through a helper that swallows either
  - `(*divergentRebootFixture).livePaneRows` (`cmd/noncontiguous_window_reboot_integration_test.go:444-462`) — a deliberate single whole-server `ListAllPanesWithFormat` enumeration; a per-pane `-t` read is wrong there for the reason above
- **Bank**: "One name for reading a pane token in fixtures, as StampPaneToken is for writing one" (3-1 reviewer) — confirmed. The same entry's second note (four new fixture tokens are not token-shaped under `session.IsTokenShaped`) is not folded: it is inert, and phase 3's own sweep fixture uses `transienttest.ReapableHookKey(n)` throughout, which is token-shaped.

### F5: `tmux.PaneTarget`'s doc comment states a rule the package's own production callers do not follow, including a line this phase added

- **Class**: comments
- **Evidence**:
  - `internal/tmux/tmux.go:404-405` — "PaneTarget formats a plain 'session:window.pane' target, for display and name-based keys. Anything issuing a tmux `-t` flag must use PaneTargetExact."
  - `internal/restore/session.go:129` builds `liveTarget` with `tmux.PaneTarget`, then `:139` passes it to `RespawnPane` (`respawn-pane -k -t`, `internal/tmux/tmux.go:651-652`) and `:155` to `SetPaneOption` (`set-option -p -t`, `:303-304`). The `SetPaneOption` call is the stamp task 3-2 added, so the phase put a second production `-t` issuer on a non-exact target.
  - `internal/tmuxtest/stamp.go:13` likewise issues `set-option -p -t <target>`, and every caller passes a non-exact `PaneTarget`: `cmd/noncontiguous_window_reboot_integration_test.go:309`, `cmd/bootstrap/reboot_roundtrip_test.go:106`, `internal/restore/multipane_legacy_integration_test.go:81-82`, `internal/restore/rename_reboot_hook_integration_test.go:107`, `cmd/rename_restore_cleanup_survival_integration_test.go:40`
  - the accurate rule is already stated one function down, at `internal/tmux/tmux.go:416-418` (`exactTarget`), where the named hazard is *session-level* prefix matching: `-t foo` silently resolving to a live `foo-2` once `foo` is gone
- **Proposed shape**: reword `PaneTarget`'s comment to say when exactness matters — a `-t` naming a session that may have been renamed or destroyed, where tmux's prefix match can silently retarget a different session — instead of the blanket prohibition the package does not honour. Pure comment edit; no call site moves and no behaviour changes. If the reviewer instead concludes the call sites are wrong, that is a behaviour change (every `-t` pane target gains an `=` prefix) and is not this pass's work.
- **Bank**: "tmux.PaneTarget doc comment forbids what ~15 test call sites do" (3-5) — confirmed and sharpened: the contradiction is not only in test call sites, it is in `internal/restore/session.go` on the line phase 3 wrote.

## Bank Verdicts

- Pre-existing repo-wide lint/format debt outside this task's surface. — residue — still true (`gofmt -l .` still reports `internal/tui/help_modal_test.go` on a clean tree). Repo-wide lint/format debt; no phase-3 file is involved, so it is not this pass's to act on.

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

- runHookStaleCleanup now takes 5 parameters, one over the project convention. — residue — still true: `runHookStaleCleanup` still takes 5 positional parameters (cmd/run_hook_stale_cleanup.go:65-71). Phase 5's `hooks.json` lock sidecar reworks this call chain, so the callbacks-struct decision belongs with it rather than here.

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

- Three parallel fakes for the one AllPaneLister seam in package cmd. — residue (pre-existing, rides to the end-of-implementation analysis, not folded) — all three fakes still stand: `stubAllPaneLister` (cmd/bootstrap_production_test.go:100-115), `recordingHookKeyLister` (cmd/hookkey_vocabulary_test.go:65-80), `fakeHookLister` (cmd/doctor_test.go:798-808). Phase 3 added no fourth and re-pointed none.

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

- Two temp hooks-store seeders in package cmd, neither derived from the other. — residue (pre-existing, rides to the end-of-implementation analysis, not folded) — both seeders still stand: `newTempHooksStore` (cmd/bootstrap_production_test.go:117), `seedHooksJSON` (cmd/doctor_test.go:811). Untouched by phase 3.

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

- The restore-window rule is now single-sourced in cmd, but the daemon states the inverse error policy twice with no shared name. — residue — still true: cmd/state_daemon.go:174 (tick) and :339 (shutdown flush) each restate the read-failed-stand-down-loudly posture, against the named `restoreWindowActive` at cmd/run_hook_stale_cleanup.go:61. Daemon lifecycle, outside phase 3's surface.

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

- A third reader of the same failed-read-counts-as-set posture sits in cmd, in a file the daemon entry does not name. — residue — still true: the third reader is at cmd/state_commit_now.go:55, behind the `IsRestoring` seam. Same ground as the executor entry above; outside phase 3's surface.

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

- TestDoctorFixPrunedHookOutput is now fully subsumed by a sibling test. — residue — still true: `TestDoctorFixPrunedHookOutput` (cmd/doctor_test.go:1665) still asserts a strict subset of `TestDoctorFixPrunesStaleEntriesThenRediagnosesClean` (:1428). Deleting a case needs an owner who can approve the count change; phase-1 ground.

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

- Four further cmd test files touching doctor.go / run_hook_stale_cleanup.go still carry concern-derived names. — residue, and partly stale as written — three of the four named files still exist (cmd/hooks_cleanstale_single_caller_guard_test.go, cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go, cmd/rename_restore_cleanup_survival_integration_test.go); `cmd/hookkey_no_regression_upgrade_test.go` is gone, renamed during phase 2. The naming convention across the survivors is pre-existing ground phase 3 did not touch.

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

- staleSeed is now declared twice, byte-identical, in one file. — residue — still true: `staleSeed` is declared twice in cmd/run_hook_stale_cleanup_test.go, at :382 and :570. Phase-1 ground; phase 3 did not touch the file.

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

- The removal leaves two byte-identical subtests in TestDoctorStaleHooksCheck. — residue — still true, and still byte-identical: cmd/doctor_test.go:1140-1146 ("it reads the marker before counting") repeats :1116-1122 ("it reports not evaluable while the restore marker is set") exactly. The sibling at :1132-1138 is genuinely distinct. Phase-1 ground.

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

- The gone-pane message reaching the user is three wraps deep, and Phase 4 reworks the same call site for hook rm wording. — residue — phase 4 fixes the exact removal wording at the same `resolveCurrentPaneKey` call site (cmd/hooks.go), so trimming the wrap layer belongs in one pass across both verbs.

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

- ResolveStructuralKey and ListAllPanes have no production callers. — residue (pre-existing, rides to the end-of-implementation analysis, not folded) — still true, and its "verify before deleting" condition is now discharged: `ResolveStructuralKey` (internal/tmux/tmux.go:226) and `ListAllPanes` (:579) are still reached only from internal/tmux/tmux_test.go, and phase 3 retired the positional hook machinery without giving either a production caller.

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

- A saver-hosting integration fixture omits the teardown guard CLAUDE.md prescribes for exactly its shape. — residue — still true: `portaltest.RegisterStateDirTeardownGuard` is still absent from cmd/state_daemon_hook_cleanup_integration_test.go. The remedy is a new setup call that removes a flake, not a pure refactor, and the file is a phase-2 output.

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

- stubAllPaneLister is the last piece of the hook-sweep seam vocabulary still sited with a single consumer. — residue — still true: `stubAllPaneLister` (cmd/bootstrap_production_test.go:100-115) still reaches cross-file for `restoringOption` while its heavy consumer is cmd/run_hook_stale_cleanup_test.go. Same ground as the pre-existing three-fakes entry; belongs with whatever next consolidates the sweep-seam fakes.

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

- Package cmd now holds two helper-only test files that want to be one. — residue — still true: cmd/testhelpers_test.go and cmd/hookkey_vocabulary_test.go both still exist as helper-only homes. Outside phase 3's surface.

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

- Two package-level names in package cmd now hold one key value. — residue — still true: `renameRestoreToken` (cmd/rename_restore_cleanup_survival_integration_test.go:21) and `reapableSeedB` (cmd/hookkey_vocabulary_test.go:18) both resolve to `transienttest.ReapableHookKey(1)`. Phase 3's own fixture calls `ReapableHookKey(0/1/2)` directly (cmd/noncontiguous_window_reboot_integration_test.go:243, :249, :257) rather than adding a fourth alias, so it neither worsened nor closed this.

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

- A few hook-file preamble sites outside the three hooks test files could take the bare helper now that it exists in the shared file. — residue — still true: cmd/state_daemon_test.go:792 and :813 and cmd/version_guard_test.go:146 still inline the hook-file preamble. Suites phase 3 did not touch.

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

- The two-helper-file rule is stated in only one of the two files, and phase 4 could silently collapse alias coverage. — residue — phase 4 reworks cmd/hooks_test.go and the alias blocks the entry warns about, so the doc-comment mitigation lands there.

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

- The last hand-rolled level filter now lives in internal/spawn, and it is not the same filter. — residue — internal/spawn is outside this phase's surface entirely; nothing in phase 3 touched `warnRecords` or its 16 call sites.

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

- The same five-property audit-breadcrumb block is still written inline outside this task two helpers. — residue — the per-package audit-breadcrumb blocks are outside this phase's surface, and generalising `hooksRecordWant` is a design change to a phase-2 helper.

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

- Two near-duplicate TMUX_PANE-unset subtests in TestHooksSetCommand, one a strict subset of the other. — residue — still true (cmd/hooks_test.go:188-211 and the subset at :346-365 both stand). The entry itself dates both subtests to earlier work units, and phase 4 reworks this file.

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

- Restore the capture-driven second reboot cycle once the production pane re-stamp exists. — mooted — task 3-2 restored the capture-driven second cycle under the approved plan amendment. internal/restore/rename_reboot_durability_integration_test.go:70-73 now takes a fresh `state.CaptureStructure` after cycle 1, :75-82 asserts `capturedPaneToken(t, sess) == renamePaneToken` under "it re-persists the pane token on the next capture after restore", :88 persists that post-restore index (not the original snapshot), and :101 keeps `assertHookFireCount(t, hookFireFile, 2)`.

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

- One name for reading a pane token in fixtures, as StampPaneToken is for writing one. — confirmed → F4

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

- assertHookFireCount reads the hook-fire file with no wait, racing the hydrate helper across the whole restore hook-fire family. — residue — still true: internal/restore/rename_reboot_shared_test.go:90-100 still reads the hook-fire file with no wait. The remedy adds a poll to a deadline, which removes a race rather than preserving behaviour, so it fails the pure-refactor bar; the entry itself dates the race to before 3-2. Recorded under Pre-existing Debt so it survives to the end-of-implementation pass.

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

- Unit-lane real-tmux restore tests silently depend on whichever portal is installed; root cause is a bare portal in the hydrate argv. — residue — still true: internal/restore/integration_test.go carries no build tag yet drives a real restore whose panes exec `portal state hydrate` off the ambient PATH, and internal/restore/session.go:310-319 still bakes a bare `portal` (the literal at :312). The root-cause fix changes the production hydrate argv and re-tags a unit-lane suite — a behaviour change, not consolidation. Surfaced in Observations.

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

- Three near-identical logtest.Record filter helpers in cmd. — confirmed → F1

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

- CLAUDE.md never describes the shape-aware reaper or internal/session new token surfaces, so the retain-old-format-forever rule is undocumented. — residue — still true (`IsTokenShaped` and "token-shaped" appear nowhere in CLAUDE.md; `NewPaneToken` appears once, at CLAUDE.md:166). The attribution is off — internal/session/tokenshape.go landed at task 1-1 and internal/session/panetoken.go at 2-2, not 3-1 — so the cause is phases 1-2, and tasks 4-2 and 5-1 already prescribe edits to the same Resume-hooks paragraph.

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

- Four integration tests exec portal state hydrate from the ambient PATH instead of staging a binary — residue — same root cause as the 3-3 reviewer entry above. internal/restore/integration_test.go, cmd/bootstrap/phase5_integration_test.go and cmd/bootstrap/phase5_marker_suppression_integration_test.go are all outside phase 3's surface, and staging a binary into PATH changes which binary the tests exercise rather than preserving behaviour.

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

- The restore-under-marker bracket is written twice, in two packages that cannot share a test file — confirmed → F3

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

- tmux.PaneTarget doc comment forbids what ~15 test call sites do — confirmed → F5

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

- Session-level -t targets in the tmux client that bypass the package own exactness rule — residue (pre-existing, rides to the end-of-implementation analysis, not folded) — still true: internal/tmux/tmux.go:259, :315, :426, :483, :534, :555 and :756 all pass a bare session name to a `-t` flag, against the rule stated at :416-418. No effect on phase 3's fixtures, whose servers hold one user session.

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

## Pre-existing Debt

- The two rename-reboot integration tests duplicate the whole capture → persist → reboot → hydrate bracket; one file names it in helpers, the other inlines it.
  DETAIL: `internal/restore/rename_reboot_durability_integration_test.go:105-121` (`captureAndPersist`) and `:123-153` (`rebootAndHydrate`) are re-implemented line for line inside `runRenameRebootFire` at `internal/restore/rename_reboot_hook_integration_test.go:118-145` and `:147-173`, in the same package. The two functions also share a ~35-line setup preamble verbatim (`BuildPortalBinaryDir` + `PrependPATH` + `IsolateStateForTest` + `PORTAL_STATE_DIR` + `EnsureDir` + `PORTAL_HOOKS_FILE` + hook-fire file + `store.Set` + `tmuxtest.New` + `new-session` + `WaitForSession` + `StampPaneToken`) at `rename_reboot_durability_integration_test.go:26-56` and `rename_reboot_hook_integration_test.go:77-107`. Phase 3 edited both sides (tasks 3-1 and 3-2) but the duplication predates it in the same shape — before 3-1 the two spelled the same steps against `session.PortalIDOption` instead of the pane token.
  FILES: internal/restore/rename_reboot_durability_integration_test.go, internal/restore/rename_reboot_hook_integration_test.go, internal/restore/rename_reboot_shared_test.go

- Four fixture helpers in `restore_test` are pairwise generalisations of one another.
  DETAIL: `seedScrollback(t, stateDir, name)` (`internal/restore/rename_reboot_shared_test.go:78-88`) is exactly `seedPaneScrollback(t, stateDir, name, 0, 0)` (`internal/restore/multipane_legacy_integration_test.go:213-223`) — same body, same payload bytes. `assertHookFireCount(t, path, want)` (`rename_reboot_shared_test.go:90-100`) hardcodes the marker `HOOK_FIRED` and parameterises the count; `assertMarkerFiredOnce(t, path, marker)` (`multipane_legacy_integration_test.go:225-235`) parameterises the marker and fixes the count at 1. One `assertMarkerCount(t, path, marker, want)` serves both. All four predate phase 3; F3's move would subsume the scrollback pair as a side effect.
  FILES: internal/restore/rename_reboot_shared_test.go, internal/restore/multipane_legacy_integration_test.go

- `assertHookFireCount` reads the hook-fire file with no wait, racing the hydrate helper across the whole restore hook-fire family.
  DETAIL: `internal/restore/rename_reboot_shared_test.go:90-100` reads immediately after `rebootAndHydrate` returns, whose last wait is `WaitForSkeletonMarkersCleared` — the marker clears before the helper execs `sh -c '<HOOK>; exec $SHELL'`. Phase 3's own `cmd` fixture does this correctly (`restoretest.WaitForFileExists` at `cmd/noncontiguous_window_reboot_integration_test.go:433`), so the correct pattern now sits beside the racy one. Affects `TestRenameRebootHook_ExternalRename`, `TestRenameRebootHook_RenameSessionEquivalent`, `TestRenameRebootHook_PaneProcessKeptRunning`, `TestRenameRebootHook_DurableAcrossRepeatedReboots` and `TestMultiPaneLegacy_PerPaneHookRouting`. Excluded from Findings because the remedy adds a poll — it removes a race rather than preserving behaviour.
  FILES: internal/restore/rename_reboot_shared_test.go, internal/restore/rename_reboot_hook_integration_test.go, internal/restore/rename_reboot_durability_integration_test.go, internal/restore/multipane_legacy_integration_test.go

- `TestLookupOnResume` repeats a five-line seed preamble and the same three-assertion no-hook block across eleven subtests.
  DETAIL: `internal/hooks/lookup_test.go` — every subtest opens with `dir := t.TempDir()` / `filePath := filepath.Join(dir, "hooks.json")` / `os.WriteFile(filePath, …)` / `hooks.NewStore(filePath)`, and eight of them close with the identical `err != nil` / `ok` / `cmd != ""` triple (`:17-27`, `:37-47`, `:58-68`, `:79-89`, `:100-110`, `:165-175`, `:187-197`, `:239-252`). Eight subtests predate phase 3; task 3-3 added three more (`:156`, `:177`, `:199`) in the file's established shape, so the subject is pre-existing.
  FILES: internal/hooks/lookup_test.go

- Three parallel `AllPaneLister` fakes in package `cmd`.
  DETAIL: `stubAllPaneLister` (cmd/bootstrap_production_test.go:100-115), `recordingHookKeyLister` (cmd/hookkey_vocabulary_test.go:65-80) and `fakeHookLister` (cmd/doctor_test.go:798-808) all implement the one seam, with `recordingHookKeyLister` a strict superset of `stubAllPaneLister`. Verbatim carry-over of the banked entry; phase 3 added no fourth and re-pointed none.
  FILES: cmd/bootstrap_production_test.go, cmd/hookkey_vocabulary_test.go, cmd/doctor_test.go

- Two temp hooks-store seeders in package `cmd`, neither derived from the other.
  DETAIL: `newTempHooksStore(t, rawJSON)` (cmd/bootstrap_production_test.go:117) and `seedHooksJSON(t, keys...)` (cmd/doctor_test.go:811) both write a `hooks.json` into a `t.TempDir()` and return `(*hooks.Store, path)`. Verbatim carry-over of the banked entry.
  FILES: cmd/bootstrap_production_test.go, cmd/doctor_test.go

- `ResolveStructuralKey` and `ListAllPanes` still have no production callers.
  DETAIL: `internal/tmux/tmux.go:226` and `:579` are reached only from `internal/tmux/tmux_test.go:1418-1556` and `:1569-1613`. Production reaches the structural shape through `StructuralKeyFormat` + `ListAllPanesWithFormat`. The banked entry's open question is now answered: phase 3 retired the positional hook machinery without giving either a production caller, so the "verify before deleting" condition is discharged. `CLAUDE.md:60` still describes all three as serving non-hook structural use, which holds only for the constant.
  FILES: internal/tmux/tmux.go, cmd/bootstrap/stale_marker_cleanup.go, CLAUDE.md

- Session-level `-t` targets in the tmux client that bypass the package's own exactness rule.
  DETAIL: `internal/tmux/tmux.go:416-418` states every session-level `-t` target must route through `exactTarget`, but seven sites pass a bare session name: `:259` (display-message), `:315` (set-option), `:426` (list-panes -s), `:483` (list-panes -s), `:534` (list-panes), `:555` (show-environment), `:756` (set-environment). Verbatim carry-over of the banked entry; F5 is the pane-level sibling of the same inconsistency but is a comment fix only.
  FILES: internal/tmux/tmux.go

## Observations

- **Unit-lane real-tmux restore tests exec whatever `portal` is installed on the developer's machine.** `internal/restore/integration_test.go` carries no build tag yet drives a real restore whose panes exec `portal state hydrate` from the tmux server's PATH, and `internal/restore/session.go:310-319` bakes a bare `portal` where `internal/spawn/command.go` deliberately uses `os.Executable()`. This is a live `CLAUDE.md` lane-rule violation ("every test that execs a built `portal` binary lives behind `-tags integration`") and a narrow production hazard, but the root-cause fix changes the production hydrate argv and re-tags a suite — fails "no behaviour change", and is plan-authorable. Same root cause reaches `cmd/bootstrap/phase5_integration_test.go:106` and `cmd/bootstrap/phase5_marker_suppression_integration_test.go:58`. Worth an orchestrator decision on its own terms.
- `internal/state/capture.go:26` composes `"#{" + PortalPaneIDOption + "}"` and `internal/tmux/tmux.go:573` composes the identical expression as `HookKeyFormat`. Not proposed: `internal/state` cannot import `internal/tmux` (cycle), so the shared home would have to be `state`, and extracting one of `captureFormat`'s eleven `#{…}` columns into a named constant leaves the other ten inline — net readability loss. Fails the value bar, not the exclusion bar.
- `TestStateMigrateRenameIsRetired` (`cmd/state_test.go:291-299`) sits beside a `removed` list at `:82` naming `status` and `cleanup`. Adding `migrate-rename` there would be additive coverage of a different claim (help listing vs `Find` resolution), not consolidation — plan-authorable.
- Phase 3 added six tests whose whole body is a single `t.Run` wrapper (`cmd/state_hydrate_empty_hookkey_test.go:49`, `:100`, `:148`; `cmd/state_test.go:292`; `internal/restore/session_test.go:334`, `:368`). The surrounding files mix both styles already and no project skill pins one, so there is no convention to converge on — not consolidation.
- The "empty token means no key" rule appears at four sites (`internal/restore/session.go:152-154` skip the stamp, `:315-317` omit the flag, `internal/hooks/lookup.go:11-13` refuse the lookup, `cmd/state_hydrate.go:294-299` drop the required-flag mark). These are four different operations under one rule, not one rule spelled four times; each carries its own rationale. No consolidation — recorded so a later pass does not mistake it for drift.
- No drift found where it would have been easy: `stampPaneToken`'s WARN (`internal/restore/session.go:156`) matches its sibling `set skeleton marker failed` (`:283`) exactly — same level, same `session`/`pane_key`/`error` attrs, same degrade-locally posture — and `pane_key` is an established attr across `internal/state/commit.go`, `cmd/state_daemon.go` and `cmd/bootstrap/stale_marker_cleanup.go`.
- The `@portal-id` retirement is clean: no `@portal-id`, `PortalID`, `portal_id` or `portalID` identifier survives outside `.workflows/` archives, the `internal/state` literal guard test was correctly deleted (its cycle-avoidance premise is gone now that `captureFormat` composes from `state.PortalPaneIDOption` directly), no test helper was orphaned in `internal/session`, and `migrateRenameSubstring` (`internal/tmux/hooks_register.go:45`, `:64-67`) is correctly retained in the teardown fingerprint union with no registration path.
- `CLAUDE.md` still does not describe `session.IsTokenShaped` or the retain-any-key-that-is-not-token-shaped rule (banked at 3-4). Caused by phases 1-2, not phase 3, and tasks 4-2 and 5-1 already prescribe edits to the same Resume-hooks paragraph — fails the cause-vs-subject test.
