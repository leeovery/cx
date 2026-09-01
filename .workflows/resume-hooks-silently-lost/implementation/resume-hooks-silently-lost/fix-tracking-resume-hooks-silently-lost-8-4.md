## Attempt 1

ISSUES:
- `cmd/open_test.go:3364` — `TestAttachConnectorConnect_ComposesItsTargetThroughTheVocabulary` duplicates `TestAttachConnectorConnectArgv` (`cmd/open_test.go:2742`): same receiver, same `recordingExecer`, same `tmuxPath`, same `Connect("foo")`, same argv assertion. The new one is strictly weaker on both counts — it drops the `argv0` check, and its expectation is computed from the function under test (`want := […, tmux.ExactSessionTarget("foo")]`), so it passes unchanged against the pre-fix `"=" + name` and would keep passing if `ExactSessionTarget` stopped emitting the `=`. The existing test already pins the literal `"=foo"`. Net effect: two tests, no added coverage, and the added one cannot fail for the reason it is named after.
  FIX: Delete `TestAttachConnectorConnect_ComposesItsTargetThroughTheVocabulary` and carry the required subtest name onto the existing test — wrap the body of `TestAttachConnectorConnectArgv` in `t.Run("it attaches through ExactSessionTarget", func(t *testing.T) { … })`, keeping its literal `want := []string{"tmux", "attach-session", "-t", "=foo"}` and its `argv0` assertion (the element loop can collapse to `slices.Equal`, matching the deleted test's style).
  ALTERNATIVE: Keep the new test and delete `TestAttachConnectorConnectArgv` instead — but then the expectation must become the literal `"=foo"` and the `argv0` assertion must be carried across, which is more editing for the same end state. Prefer the first.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- cmd/state_daemon_capture_logging_test.go:256 — the diff added a pane-target consumer (`state_daemon_run_test.go:129`, where `=work:0.0` becomes `work:0.0`), so the comment's "session target" / "resolves the same session" no longer describes what the helper does.
  OLD: // sessionFromExactTarget undoes the "=" exact-match prefix the tmux client
// composes onto a session target, so a fake resolves the same session real tmux
// would.
  NEW: // sessionFromExactTarget undoes the "=" exact-match prefix the tmux client
// composes onto a session or pane target, so a fake resolves the same target
// real tmux would.

NOTES:
- The `bindParams` → `bindSignature` widening was measured with a build overlay disabling result binding: it silences exactly one finding, `cmd/hooks.go:72`, where `paneID` is a `%N` pane id read from `$TMUX_PANE` and is exact by construction. The settlement is correct and minimal; the escape hatch is no wider than the pre-existing one for plain functions' parameters.
- The derivation reads `.Imports` from the default (untagged) build. A package whose only `internal/tmux` import sat in a `//go:build integration` file would silently drop out of the scan. `go list` and `go list -tags integration` resolve the identical 12-package set today, so there is no live gap. Adding `-tags integration` to the `go list` invocation would close it pre-emptively; not required.
- Minor, no change requested: `sessionFromExactTarget` now returns a pane coordinate for a pane target, so its name under-describes it. A rename would fit better but is a three-file test-helper rename for little gain.

## Attempt 2

ISSUES:
- internal/tmux/target_composition_guard_test.go:147 — `cmdPackageDir` identifies the root `cmd` package by `filepath.Base(dir) == "cmd" && filepath.Base(filepath.Dir(dir)) == "portal"`, coupling the guard to the checkout directory being named `portal`. A clone or `git worktree` under any other name fatals both new subtests with "the derived package set holds no cmd directory", pointing at nothing. Line 109's `containsDirSuffix` has the same root cause in milder form (suffix matching where exact membership is available).
  FIX: have `importersOfTmux` return `map[string]string` (import path → dir), keep `targetComposingPackages` returning a sorted `slices.Sorted(maps.Values(...))` for the scan, and add a sibling accessor the two subtests use: `cmdDir := byPath["github.com/leeovery/portal/cmd"]`, and assert membership of the three wanted import paths directly. That deletes both `cmdPackageDir` and `containsDirSuffix` and removes the filesystem-name coupling entirely. Sorting keeps finding order deterministic once the dirs come out of a map.
  CONFIDENCE: high

- internal/tmux/target_composition_guard_test.go:44 — the `len(dirs) == 0` fatal is the third acceptance criterion's operative clause and is untested, so the one failure mode the derivation was added to prevent (a scan that has stopped looking) is asserted only by inspection. The file itself pins the two analogous guards (`TestBareTargetGuard_ErrorsWhenItEnumeratesNoFiles`, `…ErrorsWhenItFindsNoTargetTakingMethods`), and `sourceguardtest`'s `TestPackageDeps_FatalsWhenGoListCannotResolveThePackage` pins the same class for the helper this one is modelled on.
  FIX: move the decision into the pure parser — `importersOfTmux(listing string) ([]string, error)` returning an error when it resolves no importer — have `targetComposingPackages` `t.Fatalf` on it, and add `TestBareTargetGuard_ErrorsWhenTheImportScanResolvesNothing` feeding a synthetic listing (e.g. one `path\tdir\tfmt os` line) and asserting the error. This also gives the tab-parsing its first direct coverage.
  ALTERNATIVE: keep the emptiness check where it is and switch `targetComposingPackages` to the `sourceguardtest.TestingT` subset with a stub recorder, matching `PackageDeps` exactly — but the stub cannot make the real `go list` return nothing, so the pure-function split is needed anyway. Prefer the first.
  CONFIDENCE: medium

NOTES:
- `internal/tmuxtest/socket.go:116` is integration-lane-only infrastructure and the report cites only the unit lane. Every one of the ~20 `WaitForSession` callers passes a literal name it created itself, and the one colon-named session that `=` could not address deliberately avoids the helper (internal/state/capture_colon_session_realtmux_test.go:32 says so in a comment), so the risk is low — but the change was not executed. Run the integration lane for the packages that use it.
- The derived set covers *direct* importers of `internal/tmux`. A package that composes a target and hands it through an interface without importing tmux (the shape `state.CaptureAndHashPane(PaneCapturer, target)` has) stays outside the scan. Nothing does this today.
- The guard now shells `go list` over the whole module three times per unit-lane run (~0.25s each). Invisible against the lane's real cost, but it is new I/O in a previously hermetic guard.
- The `bindParams` → `bindSignature` widening silences exactly one site (cmd/hooks.go:66's `paneID` named result); a repo-wide scan finds no other function declaring a `target`/`paneID` named result. The rule is not materially looser.
