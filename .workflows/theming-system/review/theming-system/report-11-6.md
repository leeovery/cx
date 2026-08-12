TASK: theming-system-11-6 — Single-Source The Light/Dark Slot Vocabulary And Site The Badge Glyphs With The Panel Copy

ACCEPTANCE CRITERIA:
- One function maps a slot to its `light` / `dark` attr name; `theme.slotAttr`, `cmd.themeSlotAttr` and doctor's constants all reach it.
- `prefs.ThemeSlot` still exists and `internal/prefs` imports neither `internal/theme` nor `internal/log`.
- The strings `● light`, `● dark`, `● both` and the bare `●` badge appear only in `internal/tui`'s panel copy block.
- `theme.Badge` / `theme.Badges()` still derive the badge; only the glyph text moved.
- The emitted `theme` log lines and doctor's rendered slot words are unchanged.

STATUS: complete

SPEC CONTEXT:
Spec §9.5 (specification.md:1101, 1119) fixes the badge vocabulary as user-facing panel copy: `● dark` / `● light` / bare `●`, with `● both` for one slug in both slots, chosen *because* it is no wider than `● light` and therefore cannot move the row truncation budget. §9.x/§10 fix the `theme` log component's closed attr vocabulary, in which `slot` names one half of an adaptive pair and a constant carries no `slot` attr at all. Doctor's persisted-theme advisory renders the same two words as a parenthetical (`⚠ theme solar (dark) does not resolve: not found`), so the log attr and the doctor parenthetical are one vocabulary that must not drift.

IMPLEMENTATION:
- Status: Implemented (commit 677c5b30; the `cmd` arm was later reshaped by 11-7 and the doctor arm by 15-1 — both intentional supersessions, see Notes)
- Location:
  - `internal/theme/resolution.go:17-28` — the single mapping `func (s Slot) AttrName() (string, bool)`; light/dark named, constant reports false.
  - `internal/theme/events.go:139-148` — `themeAttrs` → `slotAttr` → `slot.AttrName()`.
  - `cmd/theme_persister.go:52-55` — `themeSlotAttr(member theme.Member)` → `member.Slot().AttrName()`; `prefsSlotFor` (`:59-64`) keeps the `theme.Member` → `prefs.ThemeSlot` conversion at the `cmd` boundary, with the "prefs must not import internal/theme" reason stated at `:57-58`.
  - `cmd/doctor_theme.go:125-132` — `persistedThemeSlotLabel` reads `key.Slot.AttrName()`; the former `themeSlotLight` / `themeSlotDark` literals are gone. `themeSlotBoth` survives as the doctor-local literal at `:24-26`.
  - `internal/tui/theme_panel.go:21-27` — the four glyph strings sited in the panel copy block beside `themePanelHeaderLabel` / `themePanelDirUnreadable`, carrying the "`● both` is no wider than `● light`" rationale with them.
  - `internal/tui/theme_row.go:86, 109-122` — `themePanelBadgeText` renders the enum at the row delegate; `theme.Badge` / `theme.Badges()` / `Row.BadgeKey` derivation untouched (`internal/theme/badge.go`).
- Notes:
  - Criterion 1 holds in an amended shape. Task step 3 asked for `cmd.themeSlotAttr(prefs.ThemeSlot)`; task 11-7 ("type the light/dark selector at the nomination boundary") introduced `theme.Member`, so the function now takes a `Member` and reaches the same mapping via `member.Slot()`. Same single source, better-typed boundary — an amendment, not drift.
  - Task step 4 asked that `themeSlotBoth` be sited "beside the derived pair". The original commit did exactly that (two derived vars + the const adjacent); task 15-1 then deleted the vars and derived the pair inline inside `persistedThemeSlotLabel`, leaving `themeSlotBoth` ~100 lines above its only use. The relationship survives in prose (`cmd/doctor_theme.go:24-25`) but not in physical adjacency — see non-blocking notes.
  - Criterion 2 verified structurally: `prefs.ThemeSlot` still exists (`internal/prefs/store.go:94-100`) and `internal/prefs/leaf_guard_test.go` fails on *any* internal dependency other than `internal/fileutil`, so `internal/theme` and `internal/log` are both covered by a live guard, not by discipline.
  - Criterion 3 verified by grep across production source: the only declarations of `●` / `● light` / `● dark` / `● both` are `internal/tui/theme_panel.go:23-26`. `internal/theme` retains the glyph only inside explanatory comments (`badge.go:3,10,18,40`, `union.go:19`), which is description, not copy. `cmd/open_theme_commit_test.go:50-56` restates the four strings in a *test*, with a comment stating why (the tui constants are unexported and the test parses rendered output from another package) — a deliberate, documented test-side restatement, not a second definition.
  - Criterion 5: the words are byte-identical (`"light"` / `"dark"`), the constant still carries no attr, and doctor's `(%s)` parenthetical is unchanged.
  - `cmd/config.go:176-185` still matches the raw strings `"dark"` / `"light"` directly. This is correctly *not* sourced from `AttrName` — it parses the legacy persisted `appearance` value, a different vocabulary that merely shares two words, and its exact-match semantics (no trim, no lowercase) are load-bearing for the one-shot translation. Folding it into the slot mapping would be a defect.

TESTS:
- Status: Adequate
- Coverage:
  - `internal/theme/resolution_test.go:45-66` — `TestSlot_AttrName` covers all three slots including the constant's `(""/false)`.
  - `internal/theme/theme_test.go:224-228, 245-248` — the exported-surface guard now lists `Slot.AttrName` and no longer lists `Badge.Text`, so re-adding the render method to the leaf fails the build guard.
  - `cmd/theme_persister_test.go:104-175` — per-slot attr assertions plus the closed-key-set check, and `:163-175` reads the loader's rendering back off a *real* emission and compares it to `themeSlotAttr`, so the two emitters cannot drift apart without a failure.
  - `cmd/doctor_persisted_theme_test.go:410-440` — `TestPersistedThemeSlotLabel_ReadsTheSlotsOwnName` asserts the label equals `Slot.AttrName()` for all three slots plus `both`, and an AST/literal sweep asserts doctor's own file declares neither word. That is the strongest form of the criterion-1 doctor arm.
  - `internal/tui/theme_row_test.go:177-208` — `TestThemePanelBadgeText_RendersTheFourBadges` (four badges plus `BadgeNone` → `""`) and `TestThemePanelBadgeText_BothIsNoWiderThanLight`, which carried the width invariant across with the strings and measures it in terminal cells via `lipgloss.Width`, not `len` — the right measure for multi-byte glyphs.
  - Panel render/capture assertions still pin the badges end-to-end (`internal/capture/theme_panel_fixture_render_test.go:82-91`, `theme_panel_remaining_fixtures_test.go`, `swap_harness_test.go:51-62`), and `internal/tui/theme_panel_commit_recompute_test.go` / `theme_panel_commit_slot_test.go` still count rendered badges — so "capture fixtures render byte-identical badges" is genuinely covered.
- Notes:
  - Coverage was *moved*, not lost: the deleted `Badge.Text` tests in `internal/theme/badge_test.go` are replaced one-for-one by the `themePanelBadgeText` tests in `internal/tui`, and `badge_test.go` keeps the full derivation suite (constant/pair/both/fallback/unset/reserved-name/purity).
  - Not over-tested: each of the four surfaces has one focused test, and the drift guards (persister-vs-loader emission, doctor literal sweep) assert the *relationship* rather than restating the literals a second time.
  - One uncovered corner of criterion 3: nothing structurally prevents the glyph strings reappearing as unexported literals inside `internal/theme`. The doctor arm has exactly such a guard; the tui/theme arm does not. See non-blocking notes.

CODE QUALITY:
- Project conventions: Followed. `prefs` stays a leaf (guard-enforced); the type conversion sits at the `cmd` boundary as the architecture requires; the `theme` log component's closed attr vocabulary is unchanged; the panel's user-facing copy is pinned as constants in `internal/tui` per the repo's copy convention.
- SOLID principles: Good. The change moves a rendering concern out of a leaf domain package and leaves the derivation (`Badge`, `Badges`) where the fact lives — a clean SRP split, and the row delegate is the only site that knows glyphs.
- Complexity: Low. Every touched function is a flat switch or a one-line delegation.
- Modern idioms: Yes. Method-on-enum with a `(value, ok)` pair is the idiomatic Go shape for "may have no name", and the constant's `false` is what keeps the attr absent rather than empty.
- Readability: Good. The retained comments state the non-obvious constraints (why `● both` must not be wider, why the two slot types stay separate, why `themeSlotBoth` is not a third slot) and no comment restates code. Comment accuracy re-checked against the current source: no stale claims found — in particular `internal/tui/theme_panel.go:21-22`'s width claim is true and test-pinned, and `cmd/theme_persister.go:57-58`'s import-boundary claim matches the live leaf guard.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/events.go:146-148 — delete the `slotAttr` wrapper and call `slot.AttrName()` directly at `themeAttrs` (events.go:140), its only caller. The task prescribed delegation, but the later comment-strip pass removed the doc comment that explained the indirection, leaving an undocumented one-line pass-through that reads as vestigial.
- [do-now] cmd/doctor_theme.go:24-26 — move the `themeSlotBoth` const declaration down to sit immediately above `persistedThemeSlotLabel` (doctor_theme.go:125), its only use. Task step 4 required it be sited "beside the derived pair"; 15-1 moved the derivation into that function and left the const stranded ~100 lines above, so the adjacency the step asked for no longer exists.
- [quickfix] internal/theme — add a source guard asserting the four badge glyph strings are not declared as string literals anywhere in `internal/theme` (mirroring `cmd/doctor_persisted_theme_test.go:422-429`'s `cmdLiteralSites` sweep for the light/dark words). Criterion 3's doctor arm is guarded; the theme/tui arm rests on the exported-surface test, which would miss an unexported restatement. Scan string literals only — `●` legitimately appears in `badge.go`'s comments.
