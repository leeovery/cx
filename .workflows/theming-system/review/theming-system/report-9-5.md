TASK: theming-system-9-5 — The Slot-From-Constant Confirm's Three-Input Resolution And Atomic Commit

ACCEPTANCE CRITERIA:
- `d`/`l` with a constant set raises the confirm, writes nothing, swaps the footer to `y confirm` / `n cancel`.
- `y` and `Y` both commit: exactly one `CommitThemeSlot` with the pending slug + typed slot, constant cleared in the same write, confirm cleared, footer restored.
- `n`, `N`, `Esc` all cancel: no persister call, keys unchanged, confirm cleared, footer restored, panel still open.
- `Esc` during a live confirm does not close the panel (asserted against the close path's own observable effects).
- `Ctrl-C` during a live confirm quits.
- Every other key swallowed with the confirm still live (arrows inert, `Enter` no commit, second `d`/`l` no re-raise, preview unchanged).
- After a cancel: preview, cursor index, badge map and row set byte-identical to pre-raise.
- After a confirmed commit the badges reflect the new slot and the constant's bare `●` is gone.
- A failed confirmed commit keeps the constant in memory and routes to the 9-7 report.
- A forced close (below-floor resize) cancels silently: nothing written, no confirm-specific flash, close takes 8-10's path exactly.
- `Enter` over a pair raises no confirm.
- On a hand-edited `theme`-wins file the confirm names the constant; after `y` the stale opposite slot's badge is visible.

STATUS: complete

SPEC CONTEXT: §9.2 specifies the slot-from-constant confirm as an inline message-slot question (not a modal) naming the constant that will be cleared, key-exclusive within the panel, resolving on exactly three inputs (`y`/`Y` commit as ONE atomic prefs write; `n`/`N`/`Esc` cancel with the panel open and nothing written; `Ctrl-C` quits), with every other key swallowed and the footer substituted to the confirm scope (`y confirm` / `n cancel`) for its lifetime. §9.2 also fixes the asymmetry — `Enter` over a pair raises nothing. §8.2 pins the hand-edited `theme`-wins case: the confirm names the constant only, and the stale slot surfacing is visible in the badges once it resolves. §9.8 states the forced close silently cancels a live confirm. §14A pins the copy `clear constant <slug>?  y / n`.

IMPLEMENTATION:
- Status: Implemented (mechanism amended by later plan phases — see Notes)
- Location:
  - `internal/tui/theme_panel_confirm.go:12-90` — `themeSlotConfirm` (pending slug + typed `theme.Member`), `confirming()` (liveness read off the message slot, no second flag), `raiseSlotConfirm`, `resolveSlotConfirm` (clear-then-return so the question is down before the write), `updateSlotConfirm` (three-input resolution, default swallow), `confirmSlotAssignment`, `loadNewlyLiveSlot`, `themeConfirmAnswer` (case-insensitive with a lock-tolerant modifier mask).
  - `internal/tui/theme_panel.go:314-339` — the confirm arm sits second in `updateThemePanel`, immediately under `keyIsCtrlC` and ahead of the `Esc` close, the `Enter` commit, `d`/`l` and the nav arm. `Ctrl-C` first is required by the spec (global quit stays live); everything below the confirm arm is structurally unreachable while it is live, so a second `d`/`l` cannot re-raise.
  - `internal/tui/theme_panel_commit.go:73-91` — `handleSlotCommitKey` gates on `m.themeSetting().IsConstant` (routed through `theme.ResolveSetting`, so the `theme`-wins tiebreak cannot disagree with the panel) and raises instead of writing; `commitSlot` → `commit` performs the single `CommitThemeSlot` call and mirrors via `RawKeys.WithMember`.
  - `internal/theme/setting.go:41-46` — `WithMember` returns the named half plus the other half verbatim with `Theme` cleared, which is the in-memory half of the atomicity claim.
  - `internal/tui/theme_panel_message.go:44-46,74-79,110-121` — the confirm message kind, `Slug: m.themeState.keys.Theme` (the persisted constant, not the resolution), the `clear constant %s?  y / n` format with slug-only truncation, and `themePanelFooterScope` swapping to `themePanelConfirmKeymap()`.
  - `internal/tui/keymap.go:75-80` — the nested confirm scope (`y confirm` / `n cancel`, both Core).
  - `internal/tui/theme_panel_render.go:22-23`, `theme_panel_geometry.go:104-155` — the substituted scope drives both the rendered footer and the list's vertical budget, while the entry/refusal floor stays pinned to the standing keymap so the shorter footer cannot admit a terminal that then cannot render.
  - Forced close: `theme_panel.go:261-281` → `closeThemePanel` (`m.themePanel = themePanel{}`) discards `pending` and `message` with the struct, so the cancel is structural and raises no flash of its own.
- Notes:
  - Two deliberate later-phase amendments, both consistent with the "verify the outcome as amended" instruction: (a) the plan text types the slot as `prefs.ThemeSlot`; phases 12/14 retyped the panel's half-selector to the domain `theme.Member`, converted at the persister seam — the acceptance criteria read identically under it; (b) the plan's ordering "mirror → newly-live load → recompute" is implemented as mirror → recompute → load (`commit` recomputes, then `confirmSlotAssignment` loads). Not observable: the recompute derives badges/nomination from the mirrored raw keys only, and `loadNewlyLiveSlot` deliberately keeps the loaded palette out of the nomination (documented in-source at `theme_panel_confirm.go:66-71`). Phase 17-4 additionally narrowed the seam to `LoadSlot`.
  - The plan's "state in-source why the forced close is called out" was satisfied in the original commit (065386ce) and removed by the later comment-standard sweeps (11-3 / 12-7 / the internal/tui strip), which is where spec-citation and design-argument prose was deliberately stripped repo-wide. The behaviour itself is intact and directly tested; see the non-blocking note for the one clause worth keeping in code-standard form.
  - `seedThemePanelMessage` (`theme_panel.go:157-166`) raises the confirm from a capture-only seed; it is a fixture seam added by a later phase, not a second production raise path.

TESTS:
- Status: Adequate
- Coverage: `internal/tui/theme_panel_confirm_test.go` maps one test per acceptance criterion and every one drives the real `Model.Update` pipeline rather than the arm directly:
  - raise (`TestSlotConfirm_RaisedByDAndLOverAConstant`) asserts nothing written, pending recorded, keys still constant, confirm footer swapped, rendered copy contains `clear constant <slug>` + `?  y / n`, cursor unmoved, previewed palette unmoved, no scheduled cmd.
  - `TestSlotConfirm_ConfirmsOnEitherCase` covers `y`/`Y`/shift+`y`/capslock+`y`/numlock+`y`; `TestSlotConfirm_CancelsOnThreeInputs` covers `n`/`N`/shift+`n`/`Esc`.
  - `TestSlotConfirm_EscCancelsNotCloses` asserts against the close path's own observable effects (retained enumeration, union, badges, list items, width, live preview) and carries a control that proves the close really drops them — so the assertion cannot hold vacuously.
  - `TestSlotConfirm_SwallowsEverythingElse` is a 15-case table (arrows, paging, `Enter`, `d`, `l`, ctrl+`y`, alt+`n`, `t`, `?`, `k`, `x`, `m`, `/`) where each case first proves its control effect fires with no confirm live, then asserts the swallow across persister calls, keys, cursor index, active palette, page/modal/multi-select/filter state, scheduled cmd AND the composed frame byte-for-byte.
  - `TestSlotConfirm_CancelIsInert` pins the full pre/post-raise equality (keys, cursor, palette, badge map, row labels, rendered frame) with a fixture guard that the raise changed the frame in the first place.
  - `TestSlotConfirm_AtomicConstantClearPlusSlot` drives a real loader and asserts exactly one seam call, `{Light: nord, Dark: ""}`, the badge migration (`nord ● light`, `aurora ●` gone, default dark row appearing) and — via an AST caller scan — that `loadNewlyLiveSlot` has exactly one route in.
  - `TestSlotConfirm_FailedCommitKeepsTheConstant` pins keys/badges/rows untouched, the failure message + outstanding state, and closes with a landed-write control so the "untouched" assertions are not vacuous.
  - `TestSlotConfirm_ForcedCloseCancels`, `TestSlotConfirm_NotRaisedByEnter` (both directions), `TestSlotConfirm_HandEditedFileNamesTheConstant` (names the constant, does NOT mention the stale `ghost` slug, and the stale slot's `● light` is visible after `y`) complete the criteria.
  - Beyond the plan's list but justified: `TestSlotConfirm_UnselectableRowAsksNothing` (no question over a row with no committable slug), `TestSlotConfirm_NilPersisterIsInert` (both the answer and the newly-live load sit behind the persister), `TestSlotConfirm_ResizesTheListForTheSwappedLayout` (the shorter footer's budget round-trips).
  - Descriptor↔dispatch parity for the confirm scope (`keymap_dispatch_guard_theme_test.go:115-158, 278-295`) with a negative control proving the probes assert bound effects rather than mere consumption, plus `TestThemeConfirmKeymap_DoesNotLeakIntoPageSurfaces` and the rendered-fixture checks in `internal/capture/theme_panel_message_fixtures_test.go` (verbatim copy, `text.secondary` token, no band, exactly the two footer rows, 2-row wrap at minimum width).
- Notes: One narrow gap — the modifier mask in `themeConfirmAnswer` is not isolated by any case that would fail if it were deleted (see non-blocking notes). No over-testing found: the closest overlap is `TestThemePanelDispatch_EscMeansInnermostFirst`'s confirm sub-test against `TestSlotConfirm_EscCancelsNotCloses`, and it earns its place as the paired control for the no-confirm close inside the parity harness.

CODE QUALITY:
- Project conventions: Followed. No `t.Parallel()`; seams injected via the existing `WithThemePersister` / `ThemeSource` options; the `theme` log component is not bound in `internal/tui` (guarded by `TestCommitFailed_SingleEmissionSite`); the confirm renders through the shared descriptor/keymap machinery rather than a bespoke footer.
- SOLID principles: Good. Liveness is derived from the message slot rather than a duplicate flag; the raise/resolve/commit responsibilities are three small functions; `commit` remains the single write+mirror+recompute chokepoint so the confirm adds no second write path.
- Complexity: Low. `updateSlotConfirm` is a three-arm switch with a swallow default; key-exclusivity is achieved by arm ordering rather than by per-key guards.
- Modern idioms: Yes — `strings.EqualFold`, bit-mask on the typed `tea.KeyMod`, typed `theme.Member` over a bool/string slot.
- Readability: Good. Comments state the non-obvious reasons (clear-before-commit ordering, why the persister nil-check cannot be inferred from a nil error, why the mask is not `Mod == 0`) without restating code or citing spec sections.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/theme_panel_confirm.go:88 — no test isolates the modifier mask in `themeConfirmAnswer`: `confirmYesCtrl` / `confirmNoAlt` carry an empty `Text`, so they are rejected by `strings.EqualFold` alone and would still be swallowed with the mask deleted. Add a row to `TestSlotConfirm_SwallowsEverythingElse` pressing `tea.KeyPressMsg{Code: 'y', Text: "y", Mod: tea.ModCtrl}` with the existing `answerControl(confirmYes, …)` control, so the mask is the thing under test.
- [do-now] internal/tui/theme_panel.go:238-240 — `closeThemePanel`'s comment explains the discard but not that the discard is also what cancels a live confirm (the one non-keypress exit §9.8 calls out). Append to the existing comment block: `// Discarding the struct whole is also what cancels a live confirm: the question and the pending assignment it would write both live on it, so no separate clear is needed.`
