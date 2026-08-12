TASK: theming-system-9-9 — Closing With A Failure Outstanding Raises And Discharges The Report

ACCEPTANCE CRITERIA:
- `Esc` with a failure outstanding raises exactly the pinned copy in the active page's flash slot and returns the standard flash tick cmd.
- Raising it discharges the state: a second open + `Esc` with no new failure raises nothing.
- `Esc` with nothing outstanding raises nothing.
- A forced close with a failure outstanding raises the commit flash and not the geometry flash, and discharges the state.
- A forced close with nothing outstanding raises the pinned geometry flash (task 8-11's behaviour, unchanged).
- `Ctrl-C` with a failure outstanding quits, raises no flash, writes nothing to stderr, leaves the log as the only record.
- The close still re-resolves persisted state — the unsaved theme is reverted.
- The report renders on Projects as well as Sessions.
- The flash is theme-origin, so it claims the band with a filter applied.
- Exactly one close implementation: forced close and `Esc` produce identical model state apart from which flash is raised.
- A failed commit followed by a successful commit then `Esc` raises nothing.

STATUS: complete

SPEC CONTEXT:
§9.13 makes a failed commit write an *outstanding state* (not just a message): it runs from the failed write until a later commit succeeds, and closing the panel with it outstanding must raise a main-screen flash, because `Esc` re-resolves from persisted state (§9.2) and would otherwise revert the user's theme silently after one keystroke. Raising the flash *discharges* the state. On a forced close (§9.8) both the geometry flash and the report are due into a one-slot band and the report wins. `Ctrl-C` (§9.7) is an accepted undelivered report — the log (`theme: commit failed`, §12.3) is the record, and a post-TUI stderr warning is explicitly refused. §14A's table pins the copy as `theme not saved — see portal.log` **without** a leading `⚠`, because the notice band's warning role prepends the status glyph.

Note on the task-vs-spec copy discrepancy: the task text says to pin `⚠ theme not saved — see portal.log`; the implementation pins it without the glyph and the 9-9 commit (8d72cb28) amended §9.13/§14A accordingly ("the notice band's warning role supplies one"). Verified this is correct, not drift: `notice_band.go:59` returns `flashWarningGlyph` ("⚠") for `bandWarning`, and the report is raised as a warning-kind flash, so the rendered band reads `⚠ theme not saved — see portal.log` exactly once. The in-source comment at `internal/tui/theme_panel.go:38-39` records the double-glyph reasoning.

IMPLEMENTATION:
- Status: Implemented (with two later, intentional supersessions noted below)
- Location:
  - `internal/tui/theme_panel.go:38-40` — pinned copy constant `themeNotSavedFlash` beside the other flash strings, with the no-glyph rationale.
  - `internal/tui/theme_panel.go:241-245` — `closeThemePanel` now returns `tea.Cmd`; its single post-close step is `reportOutstandingCommitFailure()`. Still `applyInForceTheme` (the revert) then zero the panel struct — the revert stands, unchanged.
  - `internal/tui/theme_panel.go:247-255` — `reportOutstandingCommitFailure`: no-op returning nil when nothing outstanding; otherwise `setThemeFlash(themeNotSavedFlash)`, clear `themeState.commitFailed` (the discharge), return `flashTickCmd(m.flashGen)` (generation read *after* the bump, per the guard's contract).
  - `internal/tui/theme_panel.go:261-281` — forced close: `willReport := m.themeState.commitFailed` captured **before** `closeThemePanel()`, geometry flash raised only when it was false. The load-bearing ordering is pinned in-source (lines 266-267).
  - `internal/tui/theme_panel.go:318-319` — `Ctrl-C` returns `tea.Quit` from inside the panel: no close hook runs, so nothing is raised and nothing is discharged. Correct behaviour; see the comment note below.
  - `internal/tui/theme_panel.go:322-324` — `Esc` propagates the close's cmd out of `Update`.
  - `internal/tui/model.go:1466-1481` — `WindowSizeMsg` arm defers `cmd = tea.Batch(closed, cmd)` onto the named return so the forced close's command is neither swallowed nor short-circuits the message's delivery to the arms below. Correct use of a named return + defer; the ordering comment (lists sized first, because `setFlash` → `resyncPageLayouts` re-syncs them) is accurate.
  - `internal/tui/theme_state.go:66-69` — `commitFailed` lives on `themeState` (which outlives `themePanel`), with a comment stating exactly why.
- Notes:
  - **Single close path holds.** `closeThemePanel` is called from exactly `updateThemePanel` (Esc) and `resizeThemePanel` (forced), and `reportOutstandingCommitFailure` from exactly `closeThemePanel` — both pinned by an AST source guard (`theme_panel_close_report_test.go:383-391`, production files only via `sourceguardtest.PackageGoFiles(".", false)`). No forked second close.
  - **Two later supersessions, both intentional, neither drift:** task 11-15 (22beb3f3) moved `themeCommitFailed` from the Model onto `themeState`; task 17-1 (268fdddc) gave the *geometry* flash its own auto-clear tick, so the "unchanged 8-11 behaviour" criterion now means the pinned copy, not the tick-less lifecycle. The test named in the plan as `…GeometryFlashSurvives` is correspondingly `TestCloseReport_ForcedCloseGeometryFlashSelfClears` and asserts the amended behaviour.
  - The verbose per-decision comment block the 9-9 commit shipped was condensed by the phase-17 comment-standard sweep (e3fa1503); the surviving comments retain the load-bearing facts (discharge-on-raise, read-before-close ordering, one-slot precedence, no-glyph copy).

TESTS:
- Status: Adequate
- Coverage: `internal/tui/theme_panel_close_report_test.go` (418 lines) carries all eleven planned tests, one per acceptance criterion, with no redundant duplicates:
  - `TestCloseReport_RaisesTheFlash` (89) — copy verbatim against a locally declared constant (`wantThemeNotSavedFlash`, line 18, with a comment explaining why it is not the production constant), theme origin, warning kind, discharge, band actually rendered in the composed frame, `flashGen` bumped by exactly one, exactly one tick at the live generation, superseded tick ignored, matching tick clears.
  - `TestCloseReport_ForcedCloseCommitFlashWins` (127) — both floor crossings (width and height), asserts the geometry copy is *not* what landed.
  - `TestCloseReport_ForcedCloseGeometryFlashSelfClears` (145) — the nothing-outstanding forced close keeps its geometry copy, leaves no failure outstanding, and its in-flight tick is dropped by the generation guard when superseded.
  - `TestCloseReport_DischargedOnRaise` (190), `…_SilentWhenNothingOutstanding` (210), `…_SuccessfulRetryIsSilent` (222) — the discharge/silence trio, each asserting the flash text, the untouched generation *and* a nil cmd.
  - `TestCloseReport_CtrlCIsAnUndeliveredReport` (241) — quit returned, panel still open (so no close hook ran), no flash, generation untouched, **stderr captured by swapping the real `os.Stderr` fd** (helper at 273, with a comment justifying the fd swap over a writer seam), state left undischarged.
  - `TestCloseReport_RevertStands` (297) — fixture asserts the previewed theme differs from the persisted one before closing, so the revert is observable; keys untouched; the single failed commit recorded.
  - `TestCloseReport_ProjectsFlashSlot` (318) — Projects page, band role/message via `activeProjectNoticeBand`, plus the composed `viewProjectList()` frame.
  - `TestCloseReport_OutranksFilterLine` (347) — applied filter survives, band owns the slot, band sits above the filter row.
  - `TestCloseReport_SingleClosePath` (383) — the AST caller guard plus a behavioural equivalence check (identical `themeState.active`, zeroed panel struct, byte-identical `View().Content` between the forced close and `Esc`).
- Notes:
  - Failure-mode quality is high: every fixture asserts its own premise (`t.Fatal("fixture: …")`) before the behaviour, so a fixture that stops failing the commit cannot make a test pass vacuously.
  - Not over-tested: the eleven tests map one-to-one onto the eleven criteria; the shared assertions are factored into `requireReportRaised` / `requireCloseIsSilent` / `requireSingleFlashTick` rather than repeated.
  - Wall-time cost (non-blocking, see notes): `collectFlashTicks` evaluates the returned commands, and `flashTickCmd` wraps `tea.Tick(3s, …)` whose func blocks on the timer channel — six evaluations across this file, ~3s each. The file acknowledges it (line 49). The pattern is pre-existing in the package (`preview_attach_bail_flash_test.go:28`), so this is not a regression introduced here.

CODE QUALITY:
- Project conventions: Followed. No `t.Parallel()`; seams (`ThemePersister`, `ThemeSource`) injected via `tui.Deps` fakes; no tmux, no daemon, no binary — correctly in the unit lane. No logging added at this site (the `theme: commit failed` emission stays single-sited on `cmd`'s persister, guarded by `theme_persister_seam_test.go:119`).
- SOLID principles: Good. The report is a single named step (`reportOutstandingCommitFailure`) with one caller, not an inlined branch duplicated across two close sites; the forced close decides only whether its own flash is still due.
- Complexity: Low. `reportOutstandingCommitFailure` is a five-line guard-and-act; `resizeThemePanel`'s below-floor arm is one capture + one branch.
- Modern idioms: Yes. Pointer receivers for the mutating steps, `tea.Cmd` returned rather than a bool out-param, named-return + `defer` in `Update` for the deferred batch (idiomatic and the only shape that both preserves message delivery and returns the cmd).
- Readability: Good. Comments state the non-obvious *why* (why the flag is read before the close, why the copy carries no glyph, why the state lives on `themeState`) without restating the code. Spot-checked every comment in the changed region against the code — all hold true after the 11-15 and 17-1 supersessions; none reference task ids or plan phases.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_panel.go:318 — Add a one-line comment above the `keyIsCtrlC` branch recording the deliberate non-action, which is currently undocumented and reads as an omission a future reader could "fix" by discharging: `// Raises nothing and discharges nothing: the main screen is going away, and the log's` / `// theme: commit failed is the record. A post-TUI stderr warning is refused — colour` / `// preferences do not belong on the channel reserved for bootstrap failures.` (The original 9-9 commit carried this rationale; the phase-17 comment sweep removed it wholesale while keeping the neighbouring `Enter` and `Esc` branch comments.)
- [idea] internal/tui/theme_panel_close_report_test.go:50 — `collectFlashTicks` blocks on `flashAutoClearDuration` (3s) per evaluated tick, six times in this file (~18s of unit-lane wall time, plus siblings elsewhere in the package). Decide whether to make the duration test-settable (const → package var) or otherwise avoid evaluating the timer command, and apply it package-wide rather than in this one file; the mechanism is a judgement call about touching production state for test speed, which is why this is not a mechanical fix.
