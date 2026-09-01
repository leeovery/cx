## Attempt 1

ISSUES:
- `cmd/bootstrap/reboot_roundtrip_test.go:112-118`, `:462-465`, `:604-610` — three verbatim copies of the
  reboot bracket survive open-coded, in a file this change already edits, so acceptance criterion 3 ("No
  reboot bracket … is open-coded outside `restoretest`") is unmet.

  Your half-bracket defence holds only at `:462`. At `:112` and `:604` the fixture starts the fresh server
  itself — `ts.Run(t, "new-session", "-d", "-s", "_seed")` + `ts.WaitForSession(...)` is `EnsureServer`
  spelled differently, so that is the whole bracket. The reviewer notes you converted the identical shape
  (`KillServer` + `new-session _bootstrap` + `ApplyBaseIndices`) in
  `internal/restore/armed_restore_integration_test.go` in this very change — so the same site was judged
  convertible in one package and exempt in another.

  FIX (verified working by the reviewer): replace `:112-118` and `:604-610` with
  ```go
  restoretest.RebootServer(t, ts, client)
  ```
  leaving the following `tmuxtest.ApplyBaseIndices(...)` in place. No assertion change is required:
  `verifyPostBootstrapSessionSet`'s `allowedReserved` is an allow-list, not a required set
  (`reboot_roundtrip_test.go:289-304`), so the now-absent `_seed` at `:138` and `:483` is harmless — drop it
  from those two slices for tidiness or leave it. The reviewer applied exactly this substitution and ran
  `go test -tags integration -p 1 -run 'TestRebootRoundTrip|TestPhase5RebootRoundTrip' ./cmd/bootstrap/` —
  ok, 5.6s — then reverted.

  For `:462`, `RebootServer` is genuinely wrong: inserting an `EnsureServer` there changes what the test
  exercises (bootstrap step 1 finding a live server instead of a cold one). Preferred treatment — extract
  the kill-plus-confirm pair into `restoretest` as its own half (`OpenRebootGap(t, ts)`, with `RebootServer`
  composing it), which removes the third copy and satisfies the criterion literally. Acceptable alternative:
  leave `:462` and state the *correct* reason at the call site — it is the only one of the three where the
  half-bracket claim is true.
  CONFIDENCE: high on `:112`/`:604` (empirically verified); medium on the preferred treatment of `:462`.

- `internal/restore/multipane_legacy_integration_test.go:45-47` — the comment carries a cardinality claim
  falsified by adding a third test, plus changelog narration about what the refactor deleted.
  OLD:
  ```
  // newLegacyFixture is the one arrange both tests in this file share. What they
  // vary — how many panes, and which of those carry a token and a hook — arrives
  // as parameters rather than as a second copy of the same twenty-five lines.
  ```
  NEW:
  ```
  // newLegacyFixture builds the live session both multipane fixtures arrange
  // against. What they vary — how many panes, and which of those carry a token
  // and a hook — arrives as parameters.
  ```

- Two of the six tests the task names were not created and their absence was not declared:
  `"it opens the reboot gap before restoring"` and
  `"it restores a multipane legacy session through the shared arrange"`.
  The reviewer judged the behaviour covered in substance (`RebootServer`'s own must-fail guard fires on
  every caller; `TestMultiPaneLegacy_*` do run through the shared arrange), so this is a reporting gap, not
  a coverage hole. Either name them and say where they live, or state plainly why they were not created —
  the way you declared your other three corrections.

- `internal/restore/multipane_legacy_integration_test.go:87` — `for range panes[1:]` panics on an empty
  `panes` slice. Not reachable from either caller today; a one-line `t.Fatalf` in `newLegacyFixture` makes
  the fixture's own contract explicit. Optional.

NOTES:
- Every proof you reported was re-run independently and held: the guard mutation (a real
  `&restore.Orchestrator{StateDir: "/tmp"}` probe was caught with file:line), the marker mutation (deleting
  the `SetServerOption` fails two unit subtests through `assertRestoringSet`, and the third — the
  guard-on-the-guard — correctly still passes), `d692e96b` really is where `assertRestoringSet` landed, and
  the full unit lane, `golangci-lint` (0 issues) and the integration lane for `internal/restore`,
  `internal/restoretest` and `cmd` all pass.
- **Your PrependPATH audit was checked against the code and is correct.** `NewWithDefaults`
  (`cmd/bootstrap/defaults.go:86-97`) does default `hooks` to `NoOpHooks{}` and `saver` to `NoOpSaver{}`;
  `DriveSignalHydrateBinary` composes its own `PATH=` from `portalBinaryDir` (`restoretest.go:147`);
  `hydrateExe()` only falls back to the bare-name lookup when the resolver errors or returns `""`. All 12
  dropped sites were grepped for `exec.Command`, `run-shell` and `portal`-by-name — the only hit is a
  read-only `pgrep -fl`. The one kept site is the only fixture passing a real `Hooks:` registrar.
- The `cmd/bootstrap` composite-e2e failure is confirmed not yours: it reproduces on a clean `HEAD`
  worktree and passes alone and on a second run of the package.
- Two report counts were wrong and are worth getting right in your fix report: HEAD carried 12 bare literals
  across **8** test files (not 11), and the guard scans **648** `_test.go` files (not "646/388").
- `orchestrator_literal_guard_test.go:199-200` keys on the literal package identifier `restore`, so an
  aliased or dot import would evade it. No such import exists today — noted as a known edge, not a defect.
- `NewFakeExeOrchestrator(t, nil, "", nil)` at `orchestrator_builder_eager_default_test.go:21,44` was judged
  the right trade for making the rule unconditional. Leave it.

## Attempt 2

ISSUES:
- `internal/restoretest/restore_marker_test.go:107-113` — you introduced a second test double for an
  interface this package already has one for. `recordingFataller` (Helper/Name/Fatalf) is a near-copy of
  `fakeFataller` at `internal/restoretest/waitfor_file_exists_test.go:12-25` — same package, same untagged
  lane, same three-method `fataller` interface — and the existing one strictly subsumes it: its
  `fatalCalled` field is exactly what `:96-103` needs. Duplication introduced inside the scope of a task
  whose subject is collapsing near-duplicate helpers.
  FIX (the reviewer applied it and ran `go test ./internal/restoretest/ -run TestRestoreWithMarker -count=1
  -v` — all three subtests pass — then restored the file): delete the `recordingFataller` type and its three
  methods, and change the third subtest to
  ```go
  fake := &fakeFataller{}
  assertRestoringSet(fake, tmux.NewClient(rec))
  if !fake.fatalCalled {
  ```
  CONFIDENCE: high

COMMENT CORRECTIONS:
- `internal/restoretest/reboot.go:30-31` — falsified by two callers you added in this same change
  (`cmd/bootstrap/reboot_roundtrip_test.go:112` and `:599` reboot and then run a *bootstrap* orchestrator).
  OLD:
  ```
  // RebootServer opens that gap and starts a fresh empty server across it, for a
  // fixture that drives the restore itself rather than through a bootstrap.
  ```
  NEW:
  ```
  // RebootServer opens that gap and starts a fresh empty server across it, for a
  // fixture that needs a live server on the far side — whether it drives the
  // restore directly or through a bootstrap that must find the server already up.
  ```

- `cmd/reattach_integration_test.go:32-34` — states the wrong reason. No `cmd` fixture registers real global
  hooks: `buildConcurrentColdBootOrchestrator` comments that hook registration "stays NoOp"
  (`concurrent_coldboot_integration_test.go:110-112`) and `buildReattachOrchestrator` passes no
  `WithHooks`. What actually needs PATH there is the `_portal-saver` pane's `portal state daemon`.
  OLD:
  ```
  // ensurePortalOnPATH stages a built portal ahead of the ambient PATH, for the
  // fixtures whose bootstrap registers the real global hooks (whose bodies invoke
  // `portal` by name). It returns the staging directory, so a caller that also
  ```
  NEW:
  ```
  // ensurePortalOnPATH stages a built portal ahead of the ambient PATH, for the
  // fixtures whose bootstrap starts the `_portal-saver` pane, which runs `portal
  // state daemon` by name. It returns the staging directory, so a caller that also
  ```

- `internal/restoretest/orchestrator_staged.go:14-15` — falsified by `StagedRestoreAdapter` twenty lines
  below in the same file, which also produces a real-restore orchestrator (as `Inner`).
  OLD:
  ```
  // NewRestoreOrchestrator is the only supported route to an orchestrator that
  // drives a real restore against a live server. binDir must hold a built portal
  ```
  NEW:
  ```
  // NewRestoreOrchestrator is how a test builds an orchestrator it will drive a
  // real restore with against a live server. binDir must hold a built portal
  ```

NOTES:
- A fresh reviewer took a full pass over the whole change, not a delta check, and re-derived your claims
  rather than reading them. Everything else held.
- **Your `_seed` characterisation was verified and is right.** `verifyPostBootstrapSessionSet`
  (`reboot_roundtrip_test.go:285-300`) builds `allowed = allowedReserved ∪ expectedRestored` and errors on
  any raw session name outside it, with the `expectedRestored` presence loop untouched — so removing an
  entry can only *add* failures. `_seed` survives only as a pre-reboot seed (`:88`, `:445`, `:581`), which
  the kill destroys, so a surviving `_seed` post-bootstrap is now a failure where it used to be tolerated.
  It cannot make a regression pass.
- **Your two uncreated tests were judged correctly declined.** `OpenRebootGap` carries its own must-fail
  guard, so the property is asserted at all eight call sites rather than one; and both `TestMultiPaneLegacy_*`
  now run through `newLegacyFixture`, so a third test would assert file structure. No coverage hole.
- **Your PrependPATH audit was re-derived from scratch per site and holds.** The only `portal`-by-name
  resolvers are global hook bodies via `run-shell`, the `_portal-saver` pane command
  (`internal/tmux/portal_saver.go:35`) and `hydrateExe()`'s `hydrateFallbackExe`
  (`internal/restore/session.go:318`, made unreachable by `StagedHydrateExe`). No dropped site passes by
  accident.
- Your three surviving `KillServer` sites were each checked and are not brackets; a raw `kill-server` grep
  turned up only one more, a harness `shutdown()` at `cmd/state_daemon_hysteresis_measurement_test.go:290`.
- Both mutation proofs were re-run independently: removing the `SetServerOption` fails three tests (both
  `TestRestoreWithMarker_BracketsTheRestore` subtests plus `TestPhase3Integration_CorruptSessionsJSON`), and
  a bare-literal probe is caught with file:line and the 648-file scan count.
- Acceptance criterion 5 reads as met in substance rather than literally — the two *duplicated* helpers are
  gone, leaving `StagedHydrateExe` as the primitive with two thin constructors over it, and it must stay
  exported for `prefix_sibling_integration_test.go:52`. Accepted; no change asked.
- The build-tag split (`orchestrator.go` untagged, `orchestrator_staged.go` integration-only) was checked
  against `internal/restoretest/doc.go`'s stated rule and is right.
- `OpenRebootGap`'s must-fail check has no guard-on-the-guard, unlike `assertRestoringSet`. Testing it would
  need `tmuxtest.Socket` behind an interface — a bigger change than the property is worth. Leave it.
