TASK: theming-system-15-3 — Collapse The Model's Three Light/Dark Representations Onto One Owned Answer (tick-23efb4)

ACCEPTANCE CRITERIA:
1. One typed optional holds the terminal's reply; `bgReplyArrived` / `bgReplyDark` no longer exist as a loose pair.
2. One accessor answers "the light/dark answer in force", and no call site outside it reads `gate.appearance` or `canvasMode` to make that decision.
3. The single-resolution rule is intact: the canvas is painted once and never flips; a late OSC 11 reply is still consumed but never re-themes.
4. A constant → adaptive conversion in a light terminal still lands on the light slot; in a dark terminal and with no reply at all it still lands on dark.
5. `startupCanvasHex` is captured at the same moment and from the same value as today, and `RestoreTerminalBackground`'s canvas-echo guard behaves identically.
6. `themeState.nomination` and its commit-time assignment are unchanged.
7. `go test ./internal/tui` passes; the appearance-gate and restore source guards still hold.

STATUS: complete

SPEC CONTEXT:
- §8.8 (specification.md:885-889): the OSC 11 *query* survives independently of detection (restore-on-exit needs it); "the gate resolves exactly once. A reply that arrives after the timeout has resolved it does not re-resolve it" — the reply is still consumed but never flips the active theme.
- §9.3 (specification.md:1053) and §8.8 (:887): the query is issued from `Init` regardless of setting shape, which is precisely what makes a mid-session constant → adaptive conversion work with "no new query, no race, no gate" — the terminal's answer is already in hand.
- §11.3/§11.4 (:1387-1398): the canvas-echo guard compares against a *retained startup canvas hex*, never a theme re-derived at exit, and carries an explicit "do not drop" warning plus its own named verification.
The task is a structural collapse of the three in-model representations of that answer; the spec's behavioural contract is what must survive unchanged.

IMPLEMENTATION:
- Status: Implemented (commit 8874e631; comments subsequently condensed by the phase-17 comment pass, which is intended supersession, not drift)
- Location:
  - `internal/tui/theme_state.go:11-31` — new `terminalReply{arrived, member}` optional + `terminalReplyFrom(tea.BackgroundColorMsg)` (classification off nil-safe `IsDark`, not re-derived from the hex) + `answer()` (no arrival → dark).
  - `internal/tui/theme_state.go:64` — `reply terminalReply` replaces the `bgReplyArrived`/`bgReplyDark` pair.
  - `internal/tui/theme_state.go:85-97` — the single accessor `inForceMode()` plus the two establishing mutators `adoptGateAnswer()` / `adoptRetainedReply()`.
  - `internal/tui/model.go:1485-1497` — the `BackgroundColorMsg` arm retains the classified reply unconditionally, then offers the *same* verdict to the gate (`gate.resolve(reply.member)`); the retired `resolveFromDark` is gone (`internal/tui/appearance_gate.go:58-71`).
  - `internal/tui/model.go:566`, `:859-861` — `WithCanvasMode` and `syncResolvedMode` now establish/read through `adoptGateAnswer` / `inForceMode`.
  - `internal/tui/theme_panel.go:176` — `inForceSlot` is fed `m.themeState.inForceMode()`.
  - `internal/tui/theme_panel_confirm.go:72-77` — the conversion path calls `adoptRetainedReply()`; `Model.retainedCanvasAnswer` deleted.
- Criteria check:
  1. MET — `bgReplyArrived` / `bgReplyDark` / `retainedCanvasAnswer` / `resolveFromDark` return zero hits across `internal/` and `cmd/`.
  2. MET — in production code `gate.appearance` is read at exactly one site (`theme_state.go:90`, inside `adoptGateAnswer`) and written at one (`appearance_gate.go:68`); `canvasMode` is read only by `inForceMode` and written only by the two adopt methods. Every decision site (`syncResolvedMode`, `applyInForceTheme`/`inForceSlot`, and the panel/render paths reached from them) reads the accessor. The remaining `theme.MemberLight/Dark` production references (`theme_panel.go:161,331,333`, `theme_panel_commit.go`, `cmd/theme_persister.go`) are the *user's chosen commit slot*, a different question.
  3. MET — `appearanceGate.resolve` is still first-call-only; the arm retains before offering, so a late reply reaches `originalBg`/`reply` yet cannot re-theme.
  4. MET — `adoptRetainedReply` writes `reply.answer()`, which falls back to dark with no arrival; behaviour is pinned by the tests below.
  5. MET — `captureStartupCanvasHex` (`model.go:869-875`) and its single call site inside `syncResolvedMode` are untouched, still taking `active.Canvas.Value` at gate resolution; `restore.go` is byte-unchanged by this commit and still compares `OriginalBackground()` against `themeState.startupCanvasHex` via `sameHexColour`.
  6. MET — `theme_panel_commit.go` (`applyCommittedSetting`, the nomination's single owner) is not in the commit's file list; `themeState.nomination` and its doc comment are untouched. `loadNewlyLiveSlot`'s comment still records that `applyCommittedSetting` is the sole nomination writer.
  7. MET by reading — no stale identifiers remain, the restore source guard (`restore_source_guard_test.go`, including the tree-wide `canvasHexFor` ban and the `startupCanvasHex` anchor assertion) is unchanged and still satisfied by the untouched `restore.go`, and the appearance-gate suite (`appearance_detection_test.go`, `nomination_test.go`, `startup_canvas_hex_test.go`) was migrated onto the accessor rather than weakened. (Not executed — verification by reading, per the reviewer's no-test-execution rule.)
- Notes: Do-items 5 and 6 (leave `nomination` and `startupCanvasHex` alone) were respected exactly. Keeping `canvasMode` as the stored owned value is explicitly sanctioned by Do-item 4.

TESTS:
- Status: Adequate
- Coverage (each of the task's five required tests is present):
  - Converted-constant-in-a-light-terminal, driven through the new accessor — `theme_panel_commit_load_test.go:608-639` (`TestCommitSlotLoad_ConversionUsesTheRetainedAnswer`, light + dark table, asserting both `inForceMode()` and the palette the closed panel lands on).
  - Pinned (never-armed) gate + arrived light reply → light, gate's own field untouched — `theme_answer_test.go:38-55` (fixture-asserts `gate.pinned` first, so the never-armed case cannot silently stop being driven, then asserts `gate.appearance` stays dark).
  - No-answer-shaped reply (nil Color) → arrival classified dark, retained hex empty — `theme_answer_test.go:10-25`.
  - Late reply after the timeout resolved the gate → recorded but no re-theme — `theme_answer_test.go:57-75` (also asserts `active` unchanged), reinforced by `appearance_detection_test.go:107-124`.
  - Canvas-echo tests unchanged — `restore_test.go`, `restore_divergence_test.go` (commit-mid-session and quit-with-uncommitted-preview cases) and `startup_canvas_hex_test.go` are not in the commit's file list and still read `themeState.startupCanvasHex` directly.
  - Additional real coverage retained: `TestCommitSlotLoad_ConversionWithNoReplyIsDark` (:696) covers the no-reply → dark half of criterion 4; `TestCommitSlotLoad_ConversionIssuesNoQuery` (:670) pins "no new query, no new gate" — the invariant the divergence's safety rests on; `TestGate_ConstantRetainsReplyWithoutClassifying` (`nomination_test.go:142`) pins retain-without-answering.
- Notes: the migrated assertions are behavioural (answer in force, painted palette, gate untouched) rather than field-shape assertions, so they would fail if the collapse regressed. No redundant or over-mocked cases found; the apparent overlap between `theme_answer_test.go:38` and `TestCommitSlotLoad_ConversionIssuesNoQuery` is not duplication — the latter's answer assertions are its own vacuity guard, the former adds the never-armed fixture check.

CODE QUALITY:
- Project conventions: Followed. The new type is unexported and package-local, the seam-free classification helper keeps `tea` types at the message boundary, and the trimmed comment style matches the phase-17 house style applied across `internal/tui`.
- SOLID principles: Good. `terminalReply` owns "what the terminal said"; `appearanceGate` owns the single-resolution race; `themeState` owns "what is in force". The three concepts now have one owner each instead of three parallel fields on one struct.
- Complexity: Low. No branch was added; `resolveFromDark`'s two-branch wrapper was folded into `terminalReplyFrom` + the existing `resolve`, so the message arm is shorter than before.
- Modern idioms: Yes. Value-type optional with a total `answer()` accessor is the right Go shape here (no pointer, no `*theme.Member`, zero value is meaningful).
- Readability: Good. `adoptGateAnswer` / `adoptRetainedReply` name *which fact is being adopted*, which is the distinction the old code could only express in warnings.
- Issues: none blocking. The residual coupling noted below is a design observation, not a defect.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/tui/theme_state.go:89-97 — the divergence's safety is still structural-by-convention: `adoptGateAnswer` would silently clobber a converted session's answer back to the pinned gate's dark fallback if it ever ran after `adoptRetainedReply`. Today it cannot (a pinned gate is permanently resolved, so `arm`/`resolve` no-op and `syncResolvedMode` is unreachable), and `TestCommitSlotLoad_ConversionIssuesNoQuery` pins that, but the task's stated outcome was for the coupling to stop being load-bearing. Consider making it structural — e.g. record that the answer was adopted from the reply and have `adoptGateAnswer` refuse to overwrite it — which needs a decision on whether the extra state is worth more than the current test pin.
- [do-now] internal/tui/theme_state.go:51-53 — the trimmed field comment reads as if the *zero value* is what the adopt calls establish, and it dropped the name of the accessor the whole contract now rests on. Replace with: "// Zero value is the standing no-answer fallback. The value is established through\n// adoptGateAnswer / adoptRetainedReply and read through inForceMode, so which fact\n// is in force is decided at those sites rather than at each reader."
