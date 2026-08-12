TASK: theming-system-9-4 — The Panel Message Slot And The Confirm's Nested Keymap Scope

ACCEPTANCE CRITERIA:
- Confirm renders exactly `clear constant nord?  y / n` (byte-compared, double space) in `text.secondary`, no background tint on the row.
- Failed-commit line renders exactly `⚠ couldn't save theme`, glyph + text in `accent.attention`, no `bg.attention` band.
- Setting either contender clears the other; a test asserts both cannot be live simultaneously.
- With `themeMessageNone` the slot contributes zero rows; list body one row taller.
- A message wrapping to two rows shrinks the list body by two (height measured, not assumed).
- At the minimum panel height the message renders on exactly one line (truncated); at the minimum width above the floor it may occupy two.
- A long persisted slug is truncated inside the confirm copy by §9.5's rule; row still renders at the minimum width.
- The confirm names the persisted `theme` key even when that slug failed to load.
- While the confirm is live the footer renders exactly two rows (`y confirm`, `n cancel`) and reverts to the four standing rows, through the same renderer.
- `themePanelFooterHeight` reports the substituted footer's height; the layout absorbs the difference in the list body.
- `themePanelMinHeight` is unchanged by the confirm (standing scope); no row added to the floor.
- The confirm scope carries no `RightAligned`, no `?` entry, and does not leak into either page footer or help body.
- Under `colourless` both messages drop hue and keep their glyphs.

STATUS: complete

SPEC CONTEXT:
§9.1 defines the message slot as a single-row region directly above the vertical keymap footer, unreserved when empty, a single-slot arbiter with exactly two contenders (slot-from-constant confirm §9.2, failed commit write §9.13) that can never be live at once because a confirm resolves before any write happens; at the minimum width it may wrap to two rows, at the minimum *height* it truncates. §9.1's token table pins the confirm to `text.secondary`, no band, and the failed-commit line to `⚠` and text in `accent.attention` with explicitly **no** `bg.attention` band. §9.2 pins the footer switch to `y confirm` / `n cancel` while the confirm is live, with the confirm's keys living in the descriptor as a nested scope beneath the panel scope, and pins the confirm as inline (never a modal). §8.4/§9.2 require the confirm to name the *persisted* constant, which may be the slug that failed to load. §9.8 counts exactly one message row in the height floor because both contenders are non-suppressible. §14A pins the two copy strings verbatim.

IMPLEMENTATION:
- Status: Implemented (matches the plan's intent; no drift found)
- Location:
  - `internal/tui/theme_panel_message.go:15-152` — the two pinned copy constants (`themePanelConfirmFormat` with the deliberate double space, `themePanelCommitFailedMessage` built from `flashWarningGlyph`), `themePanelMessageKind` (zero value = `themeMessageNone`), the free-text-free `themePanelMessage{Kind, Slug}`, the raise/clear helpers, the token/renderer pair, the §9.5-style slug truncation, the wrap/truncate degrade, the measured height, and `themePanelFooterScope`.
  - `internal/tui/theme_panel.go:74` — `message themePanelMessage` replaces task 8-6's `message string`.
  - `internal/tui/keymap.go:73-80` — `themePanelConfirmKeymap()` placed immediately beneath `themePanelKeymap()`, two Core entries, no `RightAligned`, no `?`.
  - `internal/tui/theme_panel_render.go:22-23` — the message block is appended directly above the footer, and the footer renders through the *same* `renderThemePanelFooter` with `themePanelFooterScope(p.message)` substituted.
  - `internal/tui/theme_panel_geometry.go:107-155` — `themePanelMinHeight` still takes (and every production call site still passes) `themePanelKeymap()`, with the reasoning recorded in-source; `themePanelListSize` reserves the *measured* message height and the *live* footer scope.
  - `internal/tui/theme_panel_confirm.go:19-35,55-64` — liveness is read off the message slot itself (`confirming()`), and `resolveSlotConfirm` clears the message *before* the write, which is what makes the exclusion true by construction rather than by precedence.
- Notes:
  - Exclusion is structural, not ranked: a single struct field assigned whole, so a confirm's `Slug` cannot survive into a failed-commit line as residue. `clearThemePanelCommitFailed` is deliberately kind-checked so the pre-dispatch clear in `updateThemePanel` (`theme_panel.go:315`) cannot take the confirm question down — correct, since the confirm's answers are themselves keys.
  - The confirm reads `m.themeState.keys.Theme` (the raw persisted key), not the nomination — verified against `theme.ResolveSetting`, which makes `IsConstant` true iff the raw `theme` key is non-empty, so the confirm can never render an empty slug on the `d`/`l` path.
  - Slug truncation reuses the row layer's `themeRowEllipsis`/`themeRowLabelFloor`, and the fixed width is derived from the format string (`themePanelConfirmFixedWidth`) rather than restated, so a reworded confirm cannot leave a stale literal.
  - Width/height degrade correctly split: `themePanelMessageWraps` compares the height against the floor computed from the *standing* scope and the header shape actually being drawn, so at the floor it truncates and above it wraps, capped at two rows.
  - Render and budget resolve the wrap decision through the same function with the same arguments, so the reserved rows are the rows drawn.
  - No hex literals, no spec-section/task-id references in comments, no `t.Parallel()`, unit-lane (untagged) tests only — all consistent with CLAUDE.md.

TESTS:
- Status: Adequate
- Coverage: All fifteen named tests exist and each maps to an acceptance criterion — `internal/tui/theme_panel_message_test.go` holds `TestPanelMessage_{ConfirmPinnedCopy,CommitFailedPinnedCopy,SingleSlotExclusion,UnreservedWhenEmpty,WrappedMessageCostsTwoRows,TruncatesAtFloorHeight,ConfirmSlugTruncation,ConfirmReadsRawKeys,ConfirmTokens,CommitFailedTokens,FloorUsesStandingScope,Colourless}` and `TestPanelFooter_{ConfirmScopeSubstitution,RevertsAfterConfirm}`; `internal/tui/theme_panel_keymap_test.go:114` holds `TestThemeConfirmKeymap_DoesNotLeakIntoPageSurfaces`. The copy is asserted verbatim against locally-declared string constants (`messageTestConfirmCopy` / `messageTestFailedCopy`), not re-derived from the production format string, so a reworded constant fails rather than co-varying; the double space carries its own assertion. Token assertions use `themeRowRunAfter`, which proves *which text* a given SGR run painted (not merely that the sequence appears), and `requireNoBand` rejects `bg.attention` specifically *and* any non-canvas cell background. The wrap/truncate split is asserted on both sides of the floor and, separately, through the real `renderThemePanel` at the floor (row-position check against the footer). The floor test asserts the confirm footer is strictly shorter before relying on it, so it cannot pass vacuously. The substitution/reversion tests drive the real panel render and check the standing rows are absent, with the confirm footer's copy also pinned verbatim in `theme_panel_footer_test.go:135-153`. Uppercase `Y`/`N` dispatch parity against this scope is covered downstream by `keymap_dispatch_guard_theme_test.go:156,278`, as the task intended.
- Notes: No redundancy of consequence — the two footer tests look adjacent but assert different things (substitution + list-body absorption vs. revert-through-the-same-renderer). No over-mocking: fixtures are plain `themePanel`/`Model` constructions. Nothing tests implementation details in place of behaviour; the one structural assertion (`Slug` cleared on swap) is exactly the residue bug the value shape exists to prevent.

CODE QUALITY:
- Project conventions: Followed (token-only colour, glyph-backed state, no `t.Parallel()`, comments carry the non-obvious *why* with no process-artifact references, test helpers are functions rather than shared package-level vars per the local convention).
- SOLID principles: Good — one renderer parameterised by scope rather than a forked confirm footer; liveness derived from the single message value rather than a second flag; the floor and body budget share one `themePanelChromeRows` authority.
- Complexity: Low. Every function in the changed set is short with a single decision.
- Modern idioms: Yes (`max`, typed enum with a meaningful zero, value-semantics struct assignment).
- Readability: Good. The comments explain the load-bearing choices (why the kind check exists on the clear, why the slug is truncated rather than the line, why width and height degrade differently, why the floor keeps the standing scope).
- Issues: None material.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `internal/tui/keymap.go:73-74` — the comment records only the uppercase-dispatch half of what the task asked to document. Replace the two comment lines with: `// A second scope beside the panel's, not an addition to it: it replaces the` / `// standing keys while the confirm is live, and the panel scope's own membership` / `// is unchanged. Uppercase Y/N dispatches but is not restated. No right anchor` / `// and no ? entry, for the same reason the panel scope has neither — the footer` / `// is vertical, and ? does nothing inside the panel.`
- [quickfix] `internal/tui/theme_panel_message.go:132-134` — `append(head, tail)` writes into `lines[1]` because `head` is a sub-slice of `lines` with spare capacity. Harmless today (nothing reads `lines` afterwards) but a latent trap for a future edit; make the slice non-extendable with a three-index expression: `head := lines[:themePanelMessageWrapRows-1 : themePanelMessageWrapRows-1]`.
