## Attempt 1

ISSUES:

- `internal/tui/model.go:2715-2717` — the forced close's cmd propagation (the whole point of the named-return defer) has ZERO test coverage. The reviewer replaced the block with `_ = (&m).resizeThemePanel()` — swallowing §9.13's auto-clear tick on the forced close — and the entire suite passes: `./internal/tui`, `./cmd` and `./internal/capture` all green. The task's Do item "Both callers propagate the cmd out of `Update`" is therefore unenforced, and the executor's own doc comment ("§9.13's report … brings its own tick, which is the command returned here", `theme_panel.go:988-989`) is an unchecked claim. `resizeForTest` (`theme_panel_geometry_test.go:346`) discards the cmd, so no forced-close test can see it.

  FIX: add a cmd-returning resize helper beside `resizeForTest` (e.g. `resizeForTestCmd(t, m, contentW, contentH) (Model, tea.Cmd)` doing `m.Update(tea.WindowSizeMsg{...})` and returning both), then:
  (a) in `TestCloseReport_ForcedCloseCommitFlashWins`, drive the resize through it and assert the returned cmd yields `flashTickMsg{Gen: m.flashGen}` — the reviewer verified the report path returns the tick UNWRAPPED (`tea.Batch` compacts the nil sibling), so `cmd()` is directly a `flashTickMsg`; a batch-tolerant unwrap is still worth writing since a future arm could add a sibling cmd;
  (b) in `TestCloseReport_ForcedCloseGeometryFlashSurvives`, assert the negative — the returned cmd yields NO `flashTickMsg` (assert on the message, not `cmd == nil`: on the geometry-only path `Update` legitimately returns the session list's own cmd, which yields a nil msg). That pins §9.8's deliberate "geometry flash schedules no tick" half at the same time.

  The reviewer confirmed (a) passes as implemented and FAILS the moment the defer is removed, so the assertion has real teeth.

  CONFIDENCE: high

- `internal/tui/theme_panel_close_report_test.go:107` and `:404` — `cmd == nil` is the only thing asserted about the report's tick on the `Esc` and Projects paths; any non-nil cmd (a quit, a refetch) would satisfy it. The lifecycle is then driven by hand-constructed `flashTickMsg`s rather than by the returned cmd, so the cmd's IDENTITY is never checked. Same class of gap as above, same helper fixes it.

  FIX: assert the returned cmd resolves to `flashTickMsg{Gen: m.flashGen}` in both places (batch-tolerant unwrap), then keep the existing superseded/matching tick drive as the lifecycle half.

  CONFIDENCE: high

COMMENT_CORRECTIONS:

- `internal/tui/theme_panel.go:912` — the claim is falsified by the sibling documented 60 lines below in the same file: the two forced-close geometry flashes are raised WITHOUT an auto-clear tick (`theme_panel.go:985`, and `resizeThemePanel` returns nil on that path), so "every other flash" is not true.

  OLD:
  // rendered. The tick is the ORDINARY auto-clear every other flash schedules: this is
  // a main-screen flash like any other, unlike the panel's own message slot, whose
  // line deliberately takes no lifecycle at all.

  NEW:
  // rendered. The tick is the ORDINARY main-screen flash auto-clear, unlike the
  // panel's own message slot, whose line deliberately takes no lifecycle at all.

- `internal/tui/notice_band.go:458` — the diff edited this sentence (dropping the glyph) but left its future tense: the report lands in this slot NOW, so "in Phase 9" is both stale and workflow vocabulary.

  OLD:
  // flashText, whatever set it. §9.13's `theme not saved — see portal.log` lands
  // in this same slot in Phase 9, and closing the panel DISCHARGES that outstanding

  NEW:
  // flashText, whatever set it. §9.13's `theme not saved — see portal.log` lands
  // in this same slot, and closing the panel DISCHARGES that outstanding

NOTES:

- VERDICT context: SPEC_CONFORMANCE conformant against the AMENDED spec. `theme_panel.go:222` pins `theme not saved — see portal.log` with no `⚠`, identical to §14A's table row (spec:1814) and §9.13's prose (spec:1245); the test-side pin (`theme_panel_close_report_test.go:38`) is written out independently, not read from the constant. The reviewer rendered the band on BOTH pages: Sessions and Projects each render `▌ ⚠ theme not saved — see portal.log` — exactly ONE glyph, matching the five siblings' shape. The panel's own message slot correctly keeps its `⚠` (`theme_panel_message.go:49`). ACCEPTANCE_CRITERIA all met behaviourally. CONVENTIONS followed. ARCHITECTURE sound.
- 11 mutations run in an isolated copy; every behavioural one caught: discharge dropped → 8 tests fail; flag read AFTER the close (the named trap) → `ForcedCloseCommitFlashWins` + `SingleClosePath` fail; report removed entirely → 9 tests fail incl. 9-8's source guard; `setFlash` instead of `setThemeFlash` → 9-8's guard fails on BOTH halves; tick dropped → `RaisesTheFlash` + `ProjectsFlashSlot` fail; geometry flash raised unconditionally → `ForcedCloseCommitFlashWins` + `SingleClosePath` fail; `Ctrl-C` closes/discharges → `CtrlCIsAnUndeliveredReport` fails; copy regains the `⚠` → 9 tests fail; revert skipped when failed → `RevertStands` fails; report raised outside the close → `SingleClosePath`'s structural half fails.
- **The named-return defer was verified rigorously and is correct**: registered at most once per invocation (single `if` on the WindowSizeMsg pre-step); `Model.Update` is never re-entered recursively (only sub-model `.Update` calls exist); Go defers fire on every return including the `tea.Quit` arms; `tea.Batch` compacts nils and returns a single cmd unwrapped; `Batch` is order-free by contract so "mis-order" is not expressible; the two inner `cmd` shadows (`SessionsMsg`, `pagePreview`) assign the named result via their own `return`, so nothing is swallowed. It cannot swallow, duplicate, or mis-order on any reachable path.
- NOTE on the defer's necessity: the stated justification (splitting `Update` would break the `syncResolvedMode` caller guard in `theme_panel_commit_load_test.go:1029`) is weaker than presented — that guard's `want` list is a mechanical list of caller names, and adding a split helper's name would neither loosen nor weaken it. The reviewer is NOT asking for the refactor: the defer is correct, documented and contained, and moving a ~420-line body is the larger churn. But it must be covered.
- One close, one hook: `closeThemePanel` → `reportOutstandingCommitFailure`, guarded structurally at two levels (the new report-site guard plus 8-10's `panelDiscardSites`). No forked second close. The read-before-close ordering (`theme_panel.go:999`) is correct and pinned in-source both at the statement and in the doc block.
- 9-8's source guard is STRENGTHENED, not weakened: the added sixth table row (`theme_flash_precedence_test.go:201-210`) drives the real production path (`newCloseReportModel` → `closePanelForTest`), and the AST half still has teeth for the new copy because `themeCopyVocabulary` keys on the literal containing "theme", which survives the glyph's removal. The `setFlash` mutation failed BOTH halves. The test file diff is +10/-0: no assertion anywhere was weakened, moved or deleted (`git diff --numstat` confirms the only test change is additive).
- `Ctrl-C` verified independently: `updateThemePanel`'s arm returns `tea.Quit` with no close call, so the panel stays open, nothing is discharged, and no flash is raised — the mutation that added a `closeThemePanel()` there fails the test.
- The revert genuinely still happens: `closeThemePanel` re-resolves through `applyInForceTheme` before the report step, and gating that call on the failure flag fails `TestCloseReport_RevertStands`.
- `captureStderrForTest` (`theme_panel_close_report_test.go:327`) restores `os.Stderr` without a `defer`, so a panic inside `fn` would leave the process-global swap in place for the rest of the package. Non-blocking (nothing on the path panics).
- Minor style: `Update`'s first result is named `model` although only `cmd` needs a name for the defer; the naming skill treats named returns as documentation-only. `golangci-lint` is clean either way.
- The planning artefacts (`.workflows/.../phase-8-tasks.md`, `phase-9-tasks.md`, `planning.md`) still quote the pre-amendment glyphed copy. Out of scope for this task — the specification is the authority and it is amended in both places.
- Full unit lane `go test -count=1 ./...` green (34/34 packages, no flakes); `go vet ./...` clean; `golangci-lint run ./...` 0 issues; `gofmt -l .` reports only the three pre-existing `internal/spawn` files.
