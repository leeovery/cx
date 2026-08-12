TASK: theming-system-11-7 (tick-130ec3) — Type The Light/Dark Selector At The Nomination Boundary

ACCEPTANCE CRITERIA:
1. `Nomination.Select` takes no boolean parameter.
2. `AdaptivePair` / `joinNomination` identify light and dark by type, not argument position.
3. Exactly one site converts `canvasAppearance` to the theme-side slot type.
4. Resolution behaviour is unchanged for all three slot values, including the constant case.

STATUS: complete

SPEC CONTEXT:
The specification governs the *model* of the light/dark axis, not the Go signatures: §8.4/§8.8 state that `tui.Build` takes the loaded **nomination** (one theme under a constant, both under an adaptive pair) with "the active member selected by the gate, not supplied by the caller" (spec:1605), that the nomination "carries no provisional active member under adaptive" (spec:816), that the model holds "which member is currently active" (spec:824), and that resolution is single (§8.8 "never paint-then-flip", spec:891) with a standing **dark** no-answer fallback (spec:1207). The task is a code-quality refactor of how that answer is typed at the `tui → theme` seam; it changes no spec-visible behaviour, and the "active member" vocabulary the spec already uses is what the new `theme.Member` type names.

IMPLEMENTATION:
- Status: Implemented (criterion 2's mechanism deliberately superseded by a later plan task — see below).
- Location:
  - `internal/theme/member.go:3-24` — the new two-valued `Member` type (`MemberDark` first so the zero value is the dark no-answer fallback, matching the old `canvasAppearance` zero) with `Opposite()` and `Slot()`.
  - `internal/theme/nomination.go:55-66` — `Select(m Member) Theme`; no boolean.
  - `internal/theme/nomination.go:34-36` — `AdaptivePair(light, dark Theme)`.
  - `internal/theme/resolution.go:162`, `internal/theme/setting.go:41-46` (`RawKeys.WithMember`) — theme-side consumers.
  - `internal/tui/appearance_gate.go:17,58-71`, `internal/tui/theme_state.go:11-31,53,85-97`, `internal/tui/model.go:861`, `internal/tui/theme_panel.go:176,207-215`, `internal/tui/theme_panel_confirm.go:57,63,72-77` — TUI side.
  - `cmd/theme_persister.go:34-64` — the leaf-forced `theme.Member → prefs.ThemeSlot` conversion the task explicitly said to leave alone.
  - Task commit: `59c21223`.
- Notes (criterion by criterion, judged against the current tree):
  - **C1 met.** `Nomination.Select(m Member)`. The old `Select(dark bool)` is gone; no `dark bool` parameter survives anywhere (repo-wide grep for `dark bool` returns nothing).
  - **C2 superseded, not drifted.** `59c21223` implemented it exactly as written — `Member.Palette(Theme) MemberPalette` tagged each half and `AdaptivePair(a, b MemberPalette)` was transposition-proof. A later task in this same plan, `theming-system-17-10` / tick-5354f7 "Collapse AdaptivePair's Tagged-Palette Machinery To A Named Constructor" (commit `1ae04028`), intentionally reverted to the named positional `AdaptivePair(light, dark Theme)` and removed `MemberPalette`/`Member.Palette`. Per the verifier context's amendment rule this is the plan's own revision of the mechanism, not drift; the transposition-observability guard the task asked for survived the collapse (see TESTS). The exported-surface guard was kept in step — `internal/theme/theme_test.go:185-189` now enrols `Member`, `Member.Opposite`, `Member.Slot`, `MemberDark`, `MemberLight` and no longer lists `Member.Palette`/`MemberPalette`.
  - **C3 met and exceeded.** The task's target was one conversion site (`canvasAppearance.member()`). Later plan work collapsed `canvasAppearance` entirely — the gate now stores `theme.Member` directly (`appearance_gate.go:17`) and `themeState.canvasMode` is a `theme.Member` (`theme_state.go:53`), so there are now **zero** conversions between a TUI appearance enum and the theme-side type: repo-wide grep for `canvasAppearance` / `appearanceDarkCanvas` returns nothing. `inForceSlot` derives the slot through the single owned rule `mode.Slot()` (`theme_panel.go:208`) rather than restating a light/dark comparison, which was Do-step 3's point.
  - **C4 met.** `Select` still short-circuits `nominationUnset → zero Theme` and `nominationConstant → constant` before consulting the member, so the constant case is unchanged and a stray gate answer cannot move it. The zero-value semantics are preserved across the type change: the pre-change `canvasAppearance` zero was `appearanceDarkCanvas` (verified at `59c21223^:internal/tui/appearance_gate.go`) and `Member`'s zero is `MemberDark`, so the old `Select(canvasMode == appearanceDarkCanvas)` and the new `Select(inForceMode())` agree on every input, including the unset one. No silent light/dark inversion was introduced.
  - Do-step 5 respected: `prefs.ThemeSlot` is untouched; the conversion lives at `cmd/theme_persister.go:59-64` with the leaf constraint stated.
  - Do-step 4 (capture fixtures) is vacuously satisfied — `internal/capture` never calls `Select`; it seeds `RawKeys` and lets the resolver build the nomination.
  - No orphans or stale references: repo-wide grep for `joinNomination`, `MemberPalette`, `canvasAppearance` (code and docs, excluding `.workflows`) returns nothing.

TESTS:
- Status: Adequate (two small redundancies, noted non-blocking).
- Coverage:
  - `internal/theme/nomination_test.go:16-30` — constant nomination returns the constant for **both** members (criterion 4's constant case).
  - `:32-47` — adaptive pair returns each member's own palette.
  - `:49-68` `TestAdaptivePair_ArgumentOrderIsLightThenDark` — this is the task's required "swapping the two arguments … produces a distinguishable value" test, and it is the guard that made 17-10's collapse back to positional arguments safe: it asserts `inOrder != swapped` (so the order is observable at all) and pins what each order selects. Light/dark cannot be silently transposed without this failing.
  - `:98-104` `TestMember_ZeroValueIsDark` — pins the load-bearing zero value that keeps the no-answer fallback dark. This is the test that would catch a reordering of the `iota` block.
  - `:80-96` `TestMember_NamesItsSlotAndItsOpposite` — the `Slot()`/`Opposite()` correspondence, table-driven over both members.
  - `:110-124`, `:138-148` — zero `Nomination` answers with the zero Theme for both members and stays distinguishable from both real states.
  - `internal/theme/theme_test.go:185-189` — the exported-surface guard enumerates the new type, so an accidental widening of the light/dark surface fails a test rather than shipping.
  - Criterion 4 end-to-end (all three slot values, through the panel): `internal/tui/theme_panel_cursor_test.go:48-55` (constant), `:57-91` (light terminal → light slot, dark terminal → dark slot, table-driven), `:93-107` (both slots one slug), `:119-136` (constant under a broken-theme fallback), `:138-148` (badge stays on the persisted slug under an adaptive fallback).
  - Gate/appearance behaviour after the retype: `internal/tui/appearance_detection_test.go:69,91,99,107,126` (dark fallback on an unresolved gate, detect-dark, detect-light, no paint-then-flip, timeout → dark); `internal/tui/theme_answer_test.go:49,66` (mid-session conversion adopts the retained reply as a `theme.Member`); `internal/tui/nomination_test.go:170` (NO_COLOR still loads and holds the light member).
  - Commit round-trip through the typed member: `cmd/theme_persister_test.go:43-69,118-124,166,187,214` (member → prefs slot, both directions, plus the log attr), `cmd/open_theme_commit_test.go:279-280,326-327` and `cmd/open_theme_construction_test.go:239-240,398-401` (both members select their own palette after a real commit round trip).
  - The one production conversion the task created and later work deleted (`canvasAppearance.member()`) had its own pinning test at the time; it was correctly removed with the thing it pinned rather than left asserting a dead rule.
- Notes: The tests verify behaviour (which palette is selected, which slot is in force), not the shape of the API, so they read the same before and after 17-10's signature change — which is why that later collapse was cheap. No excessive mocking; the theme-side tests construct plain values.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` remains free of hex literals and log construction; `prefs` stays a leaf (the `Member → prefs.ThemeSlot` conversion is in `cmd`, per CLAUDE.md's leaf rule); the `theme` log component's `slot` attr is still derived from one place (`Member.Slot().AttrName()`, `cmd/theme_persister.go:52-55`) rather than a second hand-written mapping.
- SOLID principles: Good. The type carries its own two correspondences (`Slot()`, `Opposite()`) instead of leaving them as free functions restated per caller; `Nomination` keeps a single responsibility (hold the loaded palettes, answer with one). Interface segregation is respected — `Member` is deliberately two-valued rather than reusing three-valued `Slot`, which is exactly the "narrower type, not no type" the task asked for.
- Complexity: Low. `Select` is a four-arm switch; `Member`'s methods are single comparisons; `inForceSlot` lost a branch (`theme_panel.go:207-215`).
- Modern idioms: Yes. Idiomatic Go typed enum over `iota` with a deliberate, documented zero value; methods on the value type; no stringly-typed or boolean-typed parameters left on the selection API.
- Readability: Good. `m.themeState.nomination.Select(m.themeState.inForceMode())` (`model.go:861`) and `assigned.Opposite()` (`theme_panel_confirm.go:75`) read as the domain sentence they are, where the old code read `Select(m.canvasMode == appearanceDarkCanvas)`.
- Comment accuracy: Comments hold true against the current code. `nomination.go:32-33` ("from its two palettes, light first") correctly describes the post-17-10 positional constructor; `member.go:6-7`'s zero-value rationale matches the constant order; no process-artifact references (task ids, phases, spec sections) in any of the touched production comments.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/member.go:3 — add a type doc comment to `Member`, whose sibling enums `Slot` (resolution.go:3-4) and `Badge` (badge.go:3-5) both carry one and whose distinction from `Slot` is the whole point of the type: `// Member is the light/dark ANSWER: which half of an adaptive pair is in force.\n// It is two-valued where Slot is three-valued — a slot is a position in the\n// setting and the setting has a constant position, but "light or dark?" has no\n// third answer.`
- [quickfix] internal/theme/nomination_test.go:70-78 — delete `TestAdaptivePair_FillsBothMembers`. It is residue of the tagged-`MemberPalette` constructor (where naming one member twice could leave the other zero); with the positional `AdaptivePair(light, dark Theme)` restored by 17-10 it can no longer fail unless `TestAdaptivePair_HoldsBothWithNoActiveMember` (:32-47) fails first, since that test already asserts each member equals its own non-zero palette.
- [quickfix] internal/theme/nomination_test.go:56-61 — drop the two `inOrder` assertions from `TestAdaptivePair_ArgumentOrderIsLightThenDark`; they restate :41-46 verbatim. The test's distinct value is the `inOrder != swapped` inequality and the `swapped` assertions (:53-55, :62-67), which is what actually pins that light/dark cannot be transposed unobserved.
