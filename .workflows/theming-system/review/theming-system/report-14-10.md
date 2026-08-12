TASK: theming-system-14-10 — Type The Appearance Gate's Answer As theme.Member And Delete The Local Enum

ACCEPTANCE CRITERIA:
1. The identifiers `canvasAppearance`, `appearanceDarkCanvas`, `appearanceLightCanvas` and the `member()` bridge appear nowhere in the repo.
2. The "dark is the zero value" rule is stated once, on `theme.Member`.
3. Detection, first-paint gating, member selection, the mid-session divergence and the NO_COLOR carve-out are behaviourally unchanged.
4. `go build ./... && go test ./...` pass; `golangci-lint run` is clean.

STATUS: complete

SPEC CONTEXT:
The spec keeps the detect-or-timeout first-paint gate as a conditional mechanism (specification.md:884): a constant nomination paints immediately, an adaptive pair races an OSC 11 query against a ~50ms timeout with a **dark** no-answer fallback, and the fallback is load-bearing under split themes because it selects a whole named theme rather than a variant. Two further spec sites depend on that standing fallback: the NO_COLOR carve-out skips the gate entirely and lets the standing dark member select the active theme (:1207), and the mid-session constant → adaptive conversion needs a light/dark answer the launch deliberately never waited for (:1051). The spec deletes the old `theme.Mode` / `ColorFor(mode)` variant model outright (:1565), which is what leaves a single two-valued "answer" concept to be typed. The task is the architectural consequence: with the domain already owning that answer as `theme.Member`, `internal/tui`'s parallel `canvasAppearance` restated a load-bearing zero-value invariant a second time and forced a conversion at every consumer.

IMPLEMENTATION:
- Status: Implemented (later intentionally extended by task 15-3, see Notes)
- Location:
  - `internal/tui/appearance_gate.go:17` — `appearanceGate.appearance` is `theme.Member`; `resolveDark` (:58) and `resolve` (:64) take/store `theme.MemberDark` / `theme.MemberLight`.
  - `internal/tui/theme_state.go:53` — `themeState.canvasMode theme.Member`.
  - `internal/tui/theme_panel.go:207` — `inForceSlot(r theme.Resolution, mode theme.Member)` derives the slot through the single owned rule `mode.Slot()` rather than a restated light/dark comparison.
  - `internal/tui/model.go:559` — `WithCanvasMode(appearance theme.Member)`; `syncResolvedMode` (:858) selects with `inForceMode()` directly, no conversion.
  - `internal/theme/member.go:6-9` — the sole statement of the ordering rule ("MemberDark must stay first, so it is the zero value … reordering inverts that silently").
  - Task commit: `6f15069b` (29 files, +200/−250; the four non-test files are type-only edits — verified line by line against the diff).
- Notes:
  - AC1 verified: repo-wide `grep --include="*.go"` for `canvasAppearance`, `appearanceDarkCanvas`, `appearanceLightCanvas` and `.member()` returns nothing. The only surviving hits are historical prose in `.workflows/` planning and review documents, which is correct.
  - AC2 verified: the ordering/zero-value rule is stated exactly once, on `theme.Member` (`internal/theme/member.go:6-7`). `theme_state.go:51` and `terminalReply.answer()` state a *consequence* for their own field ("zero value is the standing no-answer fallback"), not the ordering rule, so nothing restates the invariant.
  - AC3 verified by reading the diff: the four production hunks are signature/constant substitutions with no branch, order or emission change. The documented divergences are intact — `canvasMode` still diverges from `gate.appearance` after the mid-session conversion (`theme_state.go:95` `adoptRetainedReply` writes the retained reply while the pinned gate keeps its fallback), `startupCanvasHex` is still frozen at gate resolution and never moved with `active` (`model.go:868-875`), and the NO_COLOR carve-out still installs `newColourlessGate()` ahead of every nomination shape (`model.go:830-832`).
  - Do-step 6 respected: the commit touches no file under `internal/theme` or `internal/prefs`, so `theme.Slot` and `prefs.ThemeSlot` are untouched.
  - **Deliberately superseded downstream (not drift).** Task 15-3 (`8874e631`, "collapse the model's light/dark representations onto one owned answer") later removed `resolveFromDark` — the OSC 11 path now calls `gate.resolve(m.themeState.reply.member)` (`model.go:1494`) — and replaced `Model.retainedCanvasAnswer` + the `bgReplyArrived`/`bgReplyDark` bool pair with the typed `terminalReply` / `adoptRetainedReply` (`theme_state.go:11-31, 95`). Both moves are continuations of this task's intent (one typed answer, zero conversions), and 14-10's own Do-steps 1-2 were satisfied at its commit. Judged against the amended intent, the outcome is stronger than the task asked for: there are now zero conversions between a TUI-local appearance representation and the domain type, and no light/dark bool survives in `internal/tui`.
  - Do-step 4's gate-side comment (added by this commit as "…already carries the no-answer fallback, because that is theme.Member's zero value") was later stripped by the package-wide comment-standard sweeps (`e3fa1503`, `915e7fcb`), leaving `appearance theme.Member` uncommented. That is a deliberate later revision, and AC2 — the rule stated once on the domain type — remains satisfied; the field's type is now the pointer to it.

TESTS:
- Status: Adequate
- Coverage:
  - Zero-value fallback (task's second requested test): `internal/tui/appearance_detection_test.go:69` `TestUnresolvedGateCarriesDarkFallback` pins all three unresolved shapes — a bare `appearanceGate`, an armed-but-open adaptive gate, and the model built over one — as `theme.MemberDark`. It replaces the deleted `TestCanvasAppearance_ZeroValueIsDark` at equal strength.
  - Domain-side rule: `internal/theme/nomination_test.go:101-102` pins `the zero Member is MemberDark — the no-answer fallback`, so the invariant is tested where it is now stated.
  - Mid-session divergence (task's third requested test): `internal/tui/theme_answer_test.go:38` `TestInForceMode_ConversionOnAPinnedGateTakesTheRetainedReply` asserts `inForceMode() == MemberLight` while `gate.appearance` stays `MemberDark` on the never-armed gate; `theme_panel_commit_load_test.go:670` additionally asserts the conversion issues no query, moves neither gate nor retained reply, and guards its own fixture (`:691` fails if the gate's appearance ever equals the answer, so the test cannot silently stop distinguishing the two).
  - No-flip / late-reply invariant: `theme_answer_test.go:57` (a late reply is recorded but never re-themes).
  - Behavioural regression surface: the renamed constants flow through the existing appearance-detection, canvas-paint, content-inset, header/footer, help-modal, background-restore, colourless/NO_COLOR and panel-cursor suites (219 `MemberDark`/`MemberLight` references across `internal/tui` tests), so a wrong-way-round reading at any consumer would fail a painted-canvas assertion rather than pass silently.
  - Structural: `theme_panel_commit_slot_test.go:416` `TestPanelSlotCommit_TypedSlotOnly` asserts the package performs no `theme.Member(...)` conversion, names no `prefs` slot, and names exactly `MemberDark`/`MemberLight` — the durable half of "one type carries the answer".
- Notes:
  - The task's own fourth "test" (`grep -rn "canvasAppearance"` returns nothing) is a one-shot manual check rather than a durable guard; see the non-blocking note below.
  - Not over-tested: the assertions added are one focused zero-value test plus type substitutions in existing tests; no redundant suite was introduced and no test was left asserting a deleted mechanism.
  - Per my role I did not execute `go build`/`go test`/`golangci-lint`; AC4 is assessed by reading — every reference site is updated consistently, the change is type-only, and 15 later commits build on this file.

CODE QUALITY:
- Project conventions: Followed. The change moves `internal/tui` onto the domain vocabulary it already imports, matching CLAUDE.md's stated rationale for keeping `prefs.ThemeSlot` separate (prefs is a no-logging leaf) while the TUI is not — the asymmetry the task cites is real and correctly acted on. No new package, no new log component, no test-lane change.
- SOLID principles: Good. Removes a duplicated invariant and a per-consumer conversion; the light/dark → slot rule now has one owner (`theme.Member.Slot`), so `inForceSlot` matches on a derived value instead of restating a comparison.
- Complexity: Low. Net −50 lines; one type, one bridge method and two constants deleted with no branch added anywhere.
- Modern idioms: Yes. Idiomatic Go named-integer enum on the domain type with the zero value carrying the fallback; `Opposite()`/`Slot()` keep the derivations as methods on the type.
- Readability: Good. `syncResolvedMode`, `inForceSlot` and `loadNewlyLiveSlot` now read in one vocabulary end to end; the surviving comments (`theme_state.go:51`, `:83`, `:93`; `appearance_gate.go:18`, `:30`, `:62`) all hold true against the current code and carry no task/phase/spec citations.
- Issues: None found.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/tui/theme_panel_commit_slot_test.go:416 — `TestPanelSlotCommit_TypedSlotOnly` would not catch a reintroduced TUI-local light/dark enum: such a type's `member()` bridge names `MemberDark`/`MemberLight` (so `members` still matches) and branches rather than converting (so `conversions` stays empty). The repo already keeps deleted-helper guards for exactly this shape (CLAUDE.md records one keeping `canvasHexFor` gone), and this task's own check for it was a one-shot grep. Decide whether to extend this guard — e.g. assert `internal/tui` declares no unexported two-valued type whose method returns `theme.Member` — or accept the grep as sufficient.
- [idea] internal/tui/model.go:559 — `WithCanvasMode` is exported but has no caller outside `internal/tui`'s own tests (`internal/capture` and `cmd` use only `WithServerStarted`/`WithProgressReceiver`/`WithThemeNomination`). Pre-existing, and this task only retyped its parameter; consider unexporting it to `withCanvasMode` so the exported Option set matches the seams external packages actually wire.
