TASK: theming-system-17-1 — Schedule The Forced-Close Geometry Flash's Auto-Clear Tick

ACCEPTANCE CRITERIA:
- `resizeThemePanel` returns a non-nil command on the below-floor path in both branches: the commit-failure report (unchanged) and the geometry flash (new).
- The forced-close flash clears when its tick fires with a matching generation, and does not clear when a superseded generation's tick fires.
- Exactly one tick is scheduled per forced close — the commit-failure branch does not gain a second.
- No change to the flash strings, to the close ordering (read `commitFailed` before the close), or to the `Update` deferral shape at `internal/tui/model.go:1478-1480`.

STATUS: complete

SPEC CONTEXT:
Specification §14A (`.workflows/theming-system/specification/theming-system/specification.md:1806-1816`) tables six theme flashes — the three `t` entry refusals, the two forced-close geometry strings (`terminal too narrow — theme picker closed` / `terminal too short — theme picker closed`), and the close report (`theme not saved — see portal.log`) — and states: "All six theme signals route through the **transient flash** slot". In Portal a transient flash is `flashAutoClearDuration` (3s) plus a scheduled `flashTickCmd` generation-guarded tick (`internal/tui/sessions_flash.go:28-40`). The two forced-close strings were the one arm raising the band without scheduling the tick, so they persisted until the next actionable key or a superseding flash — non-conformant with §14A, on the path an ordinary window resize reaches. (Note the panel's own *message slot* failed-commit report, spec:1235, is deliberately persist-until-keypress — that is a different surface and is untouched.)

IMPLEMENTATION:
- Status: Implemented (commit 268fdddc, 2-line production change)
- Location: `internal/tui/theme_panel.go:261-281` (`resizeThemePanel`), specifically the new `cmd = tea.Batch(cmd, flashTickCmd(m.flashGen))` at :274 and its comment at :271-272; comment amendment at :259-260.
- Notes:
  - Criterion 1 holds. In the `!willReport` branch `closeThemePanel()` returns nil (`reportOutstandingCommitFailure` short-circuits on `!commitFailed`, :248-251), so `tea.Batch(nil, tick)` compacts to the tick itself — `tea.Batch` drops nils and returns a lone survivor directly (`bubbletea/v2@v2.0.7/commands.go:15-35`), exactly as the task's step 3 assumed. The `willReport` branch is byte-identical to before.
  - Criterion 2 holds. `m.setThemeFlash(...)` → `setFlash` bumps `m.flashGen` *before* the tick is constructed (`internal/tui/model.go:1330-1338`, :1342-1345), so the captured generation is the live one and `Update`'s guard (`model.go:1607-1611`) admits it. A later flash bumps the counter again, so this tick is dropped rather than early-clearing.
  - Criterion 3 holds. Exactly one tick per forced close: the report branch's single tick, or the geometry branch's single tick — never both, because the `!willReport` guard is the same read that decided which flash was raised.
  - Criterion 4 holds. `model.go` is untouched by the commit; the flash constants (`theme_panel.go:31-32`) and the `willReport := m.themeState.commitFailed` read-before-close ordering (:268) are unchanged.
  - Batching rather than replacing is the right call for the stated forward-compat reason and costs nothing today.
  - Completeness check across the six §14A signals: three entry refusals tick via `blockThemePanel` (:116-119, also the `openThemePanel` re-check path at :130), the close report ticks via `reportOutstandingCommitFailure` (:254), and the two geometry strings now tick here. No remaining non-transient theme flash.

TESTS:
- Status: Adequate
- Coverage:
  - `internal/tui/theme_panel_close_report_test.go:145-171` (`TestCloseReport_ForcedCloseGeometryFlashSelfClears`, renamed from `…FlashSurvives` — the old test asserted the *absence* of a tick, so the rename plus inversion is the correct expression of the behaviour change). Runs both floor crossings and asserts: exactly one tick with the live generation, that feeding it back clears the band, and that a flash raised afterwards is NOT cleared by the in-flight tick. That is all three of the task's named tests bar one.
  - `TestCloseReport_ForcedCloseCommitFlashWins` (:127-143) is the fourth: with a failure outstanding it now asserts, via `requireSingleFlashTick`, that exactly one tick reaches `Update`'s return and that the copy is `theme not saved — see portal.log`, not the geometry string — i.e. the commit branch did not gain a second tick.
  - The helper rework is what makes the "exactly one" claim real: `flashTickFrom` (first-match, short-circuiting) became `collectFlashTicks` (full batch-tree walk) + `requireSingleFlashTick` (`len(ticks) != 1` is now a failure). The three existing call sites were migrated, so the tightened assertion is applied package-wide, not just to the new test.
  - `internal/tui/theme_panel_geometry_test.go:268-272` and `internal/tui/theme_panel_entry_test.go:367-371` add presence-only (`cmd == nil`) checks across their six-region tables.
- Notes:
  - The presence-only checks are weaker than the task text's "assert the returned command yields a `flashTickMsg`", but the deviation is sound and is documented in-place: evaluating a `tea.Tick` command blocks for the real `flashAutoClearDuration` (the timer is created at command construction, `commands.go:154-164`), so full evaluation across those two tables would add ~6 blocking waits for coverage the close-report test already provides for both dims. It also matches the file's own established idiom — `requireBlocked` (`theme_panel_entry_test.go:114-125`) has always checked entry-block ticks by presence for the same reason (`theme_panel_entry_test.go:473-474` states it explicitly).
  - The presence check is genuinely attributable, not vacuous: `resizeForTestCmd` drives a real `Update(tea.WindowSizeMsg)`, and on the Sessions page that otherwise returns nil — `list.Model.Update` handles only key/filter/spinner/status messages and `handleBrowsing` returns nil for a `WindowSizeMsg` (`bubbles/v2@v2.1.0/list/list.go:819-856`), and `tea.Batch` of all-nil is nil. So the assertion fails if the tick is removed.
  - Not over-tested: no redundant re-assertion of the flash strings, and the two table tests add one line each rather than duplicating the lifecycle test.
  - `TestCloseReport_SingleClosePath` (:383-417) still pins that `closeThemePanel` has exactly two callers and `reportOutstandingCommitFailure` exactly one, so the new batching cannot become a second raise site.

CODE QUALITY:
- Project conventions: Followed. Two-line change inside the existing arm; no new package, seam or state; comments follow the codebase's "explain the non-obvious rule" style rather than restating the code. No `t.Parallel()`, no new tmux/daemon touch, so the unit-lane rules are unaffected.
- SOLID principles: Good. The tick stays a responsibility of the site that raises the flash, which is the invariant every other flash site already obeys.
- Complexity: Low — unchanged control flow, one added statement.
- Modern idioms: Yes. `tea.Batch`'s nil-compaction is used as documented rather than hand-rolled with a nil check.
- Readability: Good. The two comments are accurate against the code: the generation *is* read after the raise bumps it, and the `willReport` branch *does* carry the tick the geometry branch otherwise supplies.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] `internal/tui/theme_panel_close_report_test.go:49-78` — `collectFlashTicks` evaluates real `tea.Tick` commands, so each `requireSingleFlashTick` call blocks for `flashAutoClearDuration` (3s). There are now six such call sites in the package (`TestCloseReport_RaisesTheFlash`, `…ForcedCloseCommitFlashWins` ×2, `…ForcedCloseGeometryFlashSelfClears` ×2, `…ProjectsFlashSlot`), ≈18s of pure sleep in a lane CLAUDE.md describes as "fast, hermetic"; this task added two of them. Evaluation is load-bearing for the generation assertion (a synthesized `flashTickMsg` proves nothing about what was *scheduled*), so the fix is a seam decision, not a rewrite: e.g. make `flashAutoClearDuration` a package-level `var` that tests shrink under `t.Cleanup` (safe here — `t.Parallel()` is banned repo-wide), or route `flashTickCmd` through a swappable tick constructor. Decide which before touching it.
- [idea] `internal/tui/theme_flash_precedence_test.go:100-188` — the §14A conformance table raises all six theme signals and asserts copy + `flashOriginTheme`, but discards the returned command, which is exactly why this defect survived to a remediation cycle. Extending that table to also pin "every theme signal schedules an auto-clear tick" would make the six-signal transience structural rather than per-site. It needs a design call first because the raise funcs currently return only the model, and asserting the tick by evaluation compounds the sleep cost in the note above.
