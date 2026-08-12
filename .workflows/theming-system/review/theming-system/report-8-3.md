TASK: theming-system-8-3 — The ● Badge Derivation Table (tick-5e1ffb, Phase 8)

ACCEPTANCE CRITERIA (from the plan task):
- A constant `"theme": "nord"` yields exactly `{"nord": BadgeConstant}` whose text is the bare `●` with no slot word.
- A pair `{light: tokyo-night-day, dark: nord}` yields `● light` on `tokyo-night-day` and `● dark` on `nord`.
- Both slots naming the same slug yield a single entry with `BadgeBoth`, never two.
- A slot that fell back keeps its badge on `Requested` and puts no badge on `Resolved`.
- A never-set slot badges the shipped default's slug (virgin install → two badges).
- A charset-rejected persisted value is badged on that raw value.
- `Row.BadgeKey()` returns the slug for a built-in / valid file / `not found` persisted row; the raw persisted string for a charset-rejected row; the filename for a `bad name` row; `""` for a `reserved name` row.
- With `"theme": "nord"` persisted plus a `nord.theme` drop-in, exactly one row's `BadgeKey()` matches the badge map (the built-in's).
- `lipgloss.Width(BadgeBoth.Text()) <= lipgloss.Width(BadgeLight.Text())`.
- The function is pure: identical input → identical output, no I/O, references no `Theme`/canvas/palette.
- A nil or empty slice returns an empty map with no panic.

STATUS: complete

SPEC CONTEXT:
§9.5 (specification.md:1116–1139) pins the marker vocabulary and the derivation table verbatim: right-aligned `● dark`/`● light` on assigned rows; `● both` on the single row when both slots name the same slug (chosen over `● dark light`, and "no wider than `● light`" so it does not move the truncation budget); a bare `●` for a constant; and "the two setting states never coexist on screen". The three-row table at :1129–1135 is exactly what the task encodes — set-and-loadable → persisted slug, set-but-unloadable → *still* the persisted slug (the fallback's own row carries none), never-set → the shipped default's slug, which is what makes §9.4's "the `●` always has something to sit on" true on a virgin install (§8.1 leaves `prefs.json` absent). §9.5:1098–1104 fixes the four-element row-composition priority in which the badge outranks the terse reason. §9.1 assigns the badge `accent.primary` (never `state.positive`, which would read as Sessions-list liveness), and §9.5:1123 pins badge-vs-cursor as independent signals. §9.5:1084/1091 give the `reserved name` row its shared-slug collision, which is why the row→badge lookup must exclude it.

IMPLEMENTATION:
- Status: Implemented (with one deliberate, plan-sanctioned relocation — see below)
- Location:
  - `internal/theme/badge.go:1-73` — `Badge` enum (`BadgeNone/Constant/Light/Dark/Both`), `Badges([]SlotResolution) map[string]Badge`, `Row.BadgeKey()`, plus the `isConstantSetting`/`collapsed`/`slotBadge` helpers.
  - `internal/theme/resolution.go:30-48` — `SlotResolution.Requested` documented as "the nominated slug, or the shipped default's where the slot was unset", which is the field the badge keys on and what makes the three-row table one field rather than three branches.
  - `internal/theme/union.go:41-52` — `Row.Identity()` (which `BadgeKey()` returns) and its separation from `SortKey()`.
  - `internal/tui/theme_panel.go:21-27` — the verbatim badge-text constants (`●`, `● light`, `● dark`, `● both`) with the "no wider than `● light`" comment; `internal/tui/theme_row.go:109-122` maps `theme.Badge` → text; `internal/tui/theme_row.go:100-104` applies `AccentPrimary` with the in-source "never state.positive" justification; `internal/tui/theme_panel.go:283-288` looks badges up through `row.BadgeKey()`, never `Slug`.
- Notes:
  - **Sanctioned relocation, not drift.** The plan authored `Badge.Text()` and the glyph constants inside `internal/theme`; they now live in `internal/tui` (`themePanelBadge*` + `themePanelBadgeText`). Phase 11's `tick-60a015` ("Single-Source The Light/Dark Slot Vocabulary And Site The Badge Glyphs With The Panel Copy") deliberately superseded that placement, and it is the better siting — it keeps `internal/theme` free of render copy while the derivation (the actual subject of this task) stays pure. The width-relation test moved with it (`internal/tui/theme_row_test.go:201`). Task outcome is intact.
  - Likewise, the in-source `§`/phase citations the task text asked for are absent by design — Phase 11's `tick-56c2d3` stripped spec-section and design-argument citations from production comments. The *substance* the task required survives: "never Resolved — Resolved would move the badge onto a fallback theme the user never chose" (`badge.go:19-21`), the badge-is-not-selectability note (`badge.go:3-5`), the mixed-slice-is-a-programming-error note (`badge.go:49-53`), and the `BadgeKey`-not-`Slug` warning (`badge.go:38-41`, echoed at `theme_panel.go:283`).
  - Behaviour matches the spec table exactly. `Badges` keys solely on `Requested`; the constant arm is gated on `len==1 && Slot==SlotConstant`, so a slice mixing a constant with a slot renders one form only; `collapsed` yields the single `BadgeBoth` entry; nil/empty returns a non-nil empty map.
  - Nothing here moves a badge — the recompute stays Phase 9's (`theme_panel_commit.go:125` re-runs `theme.Badges`), correctly.

TESTS:
- Status: Adequate
- Coverage: `internal/theme/badge_test.go` carries all eleven named tests bar the width relation (which moved to the TUI with the glyphs):
  - `TestBadges_ConstantIsBareDot` (:12), `TestBadges_PairBadgesLightAndDark` (:23), `TestBadges_SameSlugInBothSlotsIsBoth` (:37 — asserts `len(badges)==1`, so the "never two entries" half is real), `TestBadges_FallbackDoesNotMoveTheBadge` (:53 — asserts both the badge on `Requested` *and* the absence of one on `Resolved`), `TestBadges_UnsetSlotBadgesShippedDefault` (:70 — drives real `ResolveSetting`+`ResolveNomination` off empty `RawKeys`, so it exercises the virgin-install path rather than a hand-built slice), `TestBadges_CharsetRejectedValueKeepsItsBadge` (:86 — meets the union row from task 8-1 at `row.BadgeKey()`, proving the two derivations converge), `TestBadges_KeyedOnRequestedNotResolved` (:111 — four-shape table, each row additionally asserting no badge landed on `Resolved`), `TestBadges_PureAndTotal` (:169 — nil/empty/lone-slot/mixed shapes, repeat-call determinism, and an AST guard that fails on any `.Theme` selector in badge.go), `TestBadgeKey_MatchesRowIdentity` (:249 — the five-row table, each also cross-checked against `Identity()`), `TestBadgeKey_ReservedNameRowHasNone` (:294 — the `""` key, plus the end-to-end "only one row of a collided pair can render the dot" case with a real `nord.theme` drop-in beside the built-in).
  - `internal/tui/theme_row_test.go:201` `TestThemePanelBadgeText_BothIsNoWiderThanLight` asserts the width relation with `lipgloss.Width` against both slot badges, and :180-197 pins the four glyph strings verbatim.
  - Downstream coverage confirms the derivation is actually consumed: `internal/tui/theme_panel_behaviour_test.go:385`, `theme_panel_cursor_test.go:102` (`BadgeBoth` rendered), `internal/capture/theme_panel_fixture_render_test.go:79-123` and `swap_harness_test.go:51-62` (rendered `● light`/`● dark`/bare `●` in fixtures).
- Notes: No over-testing worth flagging. `TestBadges_KeyedOnRequestedNotResolved` overlaps `TestBadges_FallbackDoesNotMoveTheBadge` on the single-fallback shape, but both were separately mandated and the table adds three shapes (constant-fell-back, both-fell-back-same-slug, charset-rejected-beside-loadable) the focused test does not reach — the overlap is one row, not a duplicate test. All tests are unit-lane, hermetic (`t.TempDir` only), no `t.Parallel()`, no tmux, no daemon — correct per CLAUDE.md's lane rule.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays a leaf and log-free (`Badges` emits nothing, imports nothing); the colour-literal rule is respected (no hex, and the glyph text sits with the panel copy in `tui`); test naming and the prose-subtest style match the surrounding package; failure messages state got/want plus the reason the invariant exists.
- SOLID principles: Good. The derivation is a pure function over the Phase 5 resolution record with a single reason to change; the render decision (glyph text, token) is separated into the TUI layer; `Row.BadgeKey()` sits beside `Identity()`/`SortKey()` as a third, deliberately distinct projection of the same row.
- Complexity: Low. Three tiny helpers, one loop, no nesting past one level; `collapsed` reduces the "two slots on one slug" case to a two-line function instead of an equality branch at the call site.
- Modern idioms: Yes — `maps.Equal` in tests, `cmp.Or` in the identity chain it reuses, map preallocated to `len(slots)`.
- Readability: Good. Each comment carries a *why* that the code cannot state (why the length check exists, why `SlotConstant` maps to `BadgeNone`, why a badge implies nothing about selectability). No restated code, no process-artifact references, and every comment holds true against the code.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/badge.go:23,33 — skip a slot whose `Requested` is empty (`if slot.Requested == "" { continue }`, and the same guard on the constant arm) so a `""` key can never enter the badge map. Today it is unreachable (`ResolveSetting` substitutes a shipped default into every slot, so `Requested` is always non-empty), but a `""` key is the one value that would collide with a `reserved name` row's `BadgeKey()` and re-open the exact double-badge bug this task exists to prevent — a one-line guard makes the invariant local to the function instead of borrowed from a caller two packages away.
- [quickfix] internal/theme/badge_test.go:91-92 — hoist one `dir := t.TempDir()` and pass it to both `Assembler{...}.Open(...)` and `loader.ResolveNomination(setting, ...)`. The two calls currently get two *different* temp dirs; the assertions still hold (both are empty), but the test reads as though the union and the resolution observed the same directory when they did not, which would quietly weaken the test if a future edit staged a file into "the" dir.
- [idea] internal/theme/badge_test.go:221-234 — the purity subtest pins "references no `Theme`" via the AST selector scan but leaves the acceptance criterion's "performs no I/O" to inspection. A structural pin is possible (badge.go currently has an empty import block), but the shape needs deciding: asserting zero imports is brittle against a legitimate future `strings`/`cmp` import, while a deny-list of I/O packages needs its own maintenance rule. Worth deciding rather than adding blind.
