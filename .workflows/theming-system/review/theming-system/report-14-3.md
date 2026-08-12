TASK: 14-3 — Set the in-force light/dark answer independently of the newly-live slot's load (tick-61fe2f, ref theming-system-14-3)

ACCEPTANCE CRITERIA:
1. `canvasMode` is assigned on every constant → adaptive conversion that commits, including one where the slot load returns an error.
2. The commit-time `theme: loaded` still fires exactly once per converting commit, carrying the same slug it does today.
3. No `theme: loaded` fires on any other panel path (open, `Esc`, re-resolution, a non-converting slot commit).
4. The seam's method set and every call signature are unchanged.
5. `go test ./internal/tui ./internal/capture ./internal/theme` passes.

STATUS: complete

SPEC CONTEXT:
§9.3 "Mid-session constant → adaptive" is the governing section: the OSC 11 query is issued from `Init` regardless of the setting shape (§11.3/§8.8), so a conversion "starts using an answer that already arrived: no new query, no race, no gate", falling back to dark if no reply has landed. §9.3 explicitly separates the two halves of the transition — the classification (an answer already in hand) and "the transition's other half is a file, not an answer" (the slot the user did not assign, loaded per §8.4). The log table (spec:1485) pins `theme: loaded` as construction-time per nominated theme, "also fires at commit time" for exactly that one out-of-construction load. §8.8 pins resolve-once; §11.4 pins the retained startup canvas hex as frozen at gate resolution — which the task explicitly must not disturb. The change is precisely the code catching up with §9.3's separation: the palette load's success had been gating the terminal's classification.

IMPLEMENTATION:
- Status: Implemented (mechanism later refactored by in-plan tasks 15-3, 15-9, 17-4 and the comment-standard chores; outcome intact).
- Location:
  - `internal/tui/theme_panel_confirm.go:72-77` — `loadNewlyLiveSlot`: `m.themeState.adoptRetainedReply()` runs first and unconditionally; the load's error is fully discarded (`_ = m.themeState.source.LoadSlot(...)`).
  - `internal/tui/theme_state.go:95-97` — `adoptRetainedReply` (the 15-3 replacement for the task-era `retainedCanvasAnswer`), a pure read of `themeState.reply` with `terminalReply.answer()` (theme_state.go:26-31) supplying the dark no-answer fallback.
  - `internal/tui/theme_panel_confirm.go:55-64` — `confirmSlotAssignment` still gates the whole thing on a landed write and a non-nil persister, so a failed commit (no conversion) records no answer, as the criterion intends.
  - Original delta: commit `255196b2`; superseded by `8874e631` (15-3, one owned answer), `aeb614b4` (15-9), `62a7c974` (17-4, `ResolveSlot` → `LoadSlot` on the seam).
- Notes:
  - AC1 holds: the assignment precedes the load and nothing between them can fail; the fatal path leaves the classification intact.
  - AC2/AC3 hold structurally, not just by test: `theme: loaded` is emitted only by `Loader.commitPass` (`internal/theme/resolution.go:78-84`), reached only through `DirThemeSource.LoadSlot` (`internal/theme/dir_theme_source.go:36-39`), which is called from exactly one site — `loadNewlyLiveSlot` — itself pinned to one caller by the guard at `internal/tui/theme_panel_confirm_test.go:648`. Every other panel path (`Open`, `Reassemble`, `Resolve`) routes through `enumerationPass`, which reports fallbacks only.
  - AC4 ("seam method set unchanged") is the one criterion no longer literally true: task 17-4 (`62a7c974`) deliberately narrowed `ThemeSource.ResolveSlot(...) (SlotResolution, error)` to `LoadSlot(...) error`. That is a later in-plan supersession of exactly the "discarded value" this task described, not drift — and it strengthens the task's intent, since the discard is now enforced by the seam's type rather than by an `_` at the call site.
  - The task's "do not touch" list is respected: `startupCanvasHex` is still captured only in `syncResolvedMode` (`internal/tui/model.go:858-865`), which the conversion does not call; `gate.appearance` is untouched by `adoptRetainedReply`; `themeState.active` is not re-themed on commit (the close's `applyInForceTheme`, theme_panel.go:171-185, is what consumes the new answer).
  - No orphans left behind: `retainedCanvasAnswer` and `persistedSlotSlug` are gone from the tree.

TESTS:
- Status: Adequate.
- Coverage:
  - `internal/tui/theme_panel_commit_load_test.go:795-831` `TestCommitSlotLoad_AnswerIsIndependentOfTheLoad` — the task's own test, table-driven over a landing load and `errThemeResolveFatal`, driven with a LIGHT OSC 11 reply and a fixture guard (line 814) proving the answer starts on the standing dark fallback so a light outcome is the conversion's doing. It also asserts the load actually ran (`len(seam.slotLoads) != 1`), so the fatal case cannot pass by never loading. This is the test that fails before the change (pre-change, the fatal path skipped the assignment and left dark).
  - Cadence, unchanged and still covered: `requireLoadedLine` (exactly 1 per conversion, slug + slot), `TestCommitSlotLoad_EmitsLoadedOncePerConversion` (2 conversions → 2 lines, not deduplicated), `TestCommitSlotLoad_NonConvertingCommitIsSilent` (`d`/`l`/`Enter` over a pair and `Enter` over a constant emit none, with a non-vacuous "exactly one commit" fixture guard), `TestCommitSlotLoad_FailedCommitLoadsNothing`, `TestCommitSlotLoad_DiscardSilencesLoaded`, and the close-path counts in `theme_panel_close_test.go:303,314`.
  - Surrounding invariants the change could have broken are pinned independently: `TestCommitSlotLoad_ActiveThemeUnchanged` (no palette swap on commit), `TestCommitSlotLoad_ConversionDoesNotMoveStartupCanvasHex` (incl. the `syncResolvedMode` caller-set scan at :760-768), `TestCommitSlotLoad_ConversionIssuesNoQuery`, `TestCommitSlotLoad_ConversionWithNoReplyIsDark`, `TestCommitSlotLoad_ConversionUsesTheRetainedAnswer` (answer → close-time in-force member), `TestCommitSlotLoad_BrokenBuiltinDegrades` (nomination/active/panel unmoved on the fatal).
- Notes:
  - Not over-tested: each conversion test carries a distinct axis (answer source, query/gate untouched, anchor frozen, cadence, degrade). The two-case table here is the minimum that distinguishes "assigned before the load" from "assigned after a successful load".
  - One assertion message in the neighbouring degrade test has gone stale in meaning — see the `[do-now]` note.

CODE QUALITY:
- Project conventions: Followed. Seam-based DI (`ThemeSource` stays a 4-method interface), no logging added in `internal/tui` (the `theme` component is emitted from `internal/theme` and `cmd` only, per CLAUDE.md), no `t.Parallel()`, unit-lane test with no tmux/daemon touch.
- SOLID principles: Good — the change is a de-coupling: a terminal fact and a palette load no longer share a success condition. `themeState` owns both mode adoptions (`adoptGateAnswer` / `adoptRetainedReply`), so `canvasMode` has one writer type.
- Complexity: Low — the function is two statements with no branch.
- Modern idioms: Yes.
- Readability: Good, with one naming caveat (below): `loadNewlyLiveSlot` performs the classification adoption as well as the load, so the identifier under-describes the body. The doc block leads with the classification, which mitigates it.
- Comment accuracy: The surviving doc block (theme_panel_confirm.go:68-71) holds against the code — "assigned first and unconditionally", "the load's error is nothing this site can act on" (matches `_ =`), "applyCommittedSetting is its single owner" (matches theme_panel_commit.go:120-127), "Never ApplyTheme" (true). The task-era claim that this call "surfaces the fatal" is correctly gone: with the error discarded, nothing here surfaces anything, and the degrade-silently policy is `applyInForceTheme`'s.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/theme_panel_confirm.go:72 — `loadNewlyLiveSlot` names only one of the two acts its body performs. Hoist `m.themeState.adoptRetainedReply()` into `confirmSlotAssignment` (theme_panel_confirm.go:60-63), placed after the `m.themeState.persister == nil` guard and before the call, so the conversion's two independent facts read as two statements and the function's name matches its body. Keep the order (adopt, then load) and keep it below the nil-persister guard — moving it above would record an answer on the writer-less path where no conversion happened. If a rename is preferred instead, the caller-set guard at internal/tui/theme_panel_confirm_test.go:648 pins the name and must be updated in the same edit.
- [do-now] internal/tui/theme_panel_commit_load_test.go:857-859 — the failure message ("the fatal moved the light/dark answer to %v, want the untouched %v") reads as a general contract that a load fatal must leave the answer alone, which `TestCommitSlotLoad_AnswerIsIndependentOfTheLoad` pins as false for a terminal that replied; it only holds here because this fixture delivers no reply. Replace with: `t.Errorf("the fatal left the light/dark answer %v, want the no-reply dark fallback %v — the answer is recorded from the retained reply before the load runs, and this fixture receives none", m.themeState.inForceMode(), mode)`.
