TASK: theming-system-11-8 — Extract The Shared Key-Column Row Used By The Panel Footer And The Help Modal (tick-a1a369, done)

ACCEPTANCE CRITERIA:
- One function builds the key-column row; both callers delegate.
- Both surfaces render byte-identical output to before (committed help-modal and panel capture references unchanged).
- The destructive-aware key token and the boldness difference are resolved at the help-modal call site, not by a branch inside the shared builder.

STATUS: complete

SPEC CONTEXT:
The task is a duplication-remediation item from analysis phase 11, not a spec-behaviour item. The relevant spec anchor is §(line 952) of `.workflows/theming-system/specification/theming-system/specification.md`: the theme slide-over carries "a vertical keymap footer (`⏎ set theme` / `d set as dark` / `l set as light` / `esc close`) rather than Portal's horizontal footer row … the vertical form matches the help modal's key-column idiom." The spec therefore mandates that the two surfaces share a visual idiom while keeping their own column sizing (§952 plus §1229 — the panel has no help modal of its own, so the panel footer is the only in-panel keymap surface). Extraction of the geometry is consistent with that; the spec constrains only the rendered result, which criterion 2 pins byte-for-byte.

IMPLEMENTATION:
- Status: Implemented (with a deliberate, well-justified signature refinement over the plan's "Do" step 1)
- Location:
  - `internal/tui/modal_footer.go:32-41` — `keyColumnRow(glyph, label string, keyStyle, labelStyle lipgloss.Style, columnWidth int, gap string, th theme.Theme, colourless bool) string`. Single measure/pad/gap/join body; the pad-omission branch (`keyWidth < columnWidth`) is carried over verbatim.
  - `internal/tui/help_modal.go:72-83` — `helpModalRow` resolves the destructive-aware key token and `.Bold(true)` into a ready `lipgloss.Style` before delegating, and passes `helpKeyColumnWidth` (10) + `helpColumnGap`.
  - `internal/tui/theme_panel_footer.go:38-46` — `themePanelFooterRow` delegates with `AccentKey`/`TextMuted` (unbolded), `themePanelFooterKeyColumnWidth` (3) + `footerKeyLabelGap`, and keeps its `headerPadRight` wrap at its own call site.
  - Commit `e985a3d0`; comments in all three files were later trimmed by the phase-17 comment remediation (`git diff e985a3d0 HEAD` shows comment-only deletions), which is expected per the "later phases superseded" note.
- Criterion 1: met — one builder, both callers delegate, no third site left composing this geometry (`grep KeyColumnWidth` over non-test sources returns only the two constants and their two call sites; `filter_footer.go:108` and `footer.go` compose horizontal hints without a fixed key column, a genuinely different shape correctly left alone).
- Criterion 2: met — the two callers' segment order, measurement (`lipgloss.Width`), canvas painting (`headerCanvasBg`) and pad arithmetic are preserved. The only textual change inside the moved body is `strings.Repeat(" ", n)` → `spaces(n)` on the help path; `spaces` (`internal/tui/session_item.go:298`) returns exactly n spaces for n > 0 and the call is guarded by `keyWidth < columnWidth`, so the two are equivalent on every reachable input. The commit touched no `testdata/` or capture assets, and no existing render-test expectations, so the "references unchanged / no regeneration" half of the criterion holds by construction.
- Criterion 3: met — `keyColumnRow` contains no per-surface branch; the destructive token (`isDestructiveHelpKey` → `th.StateDestructive`) and the bold weight are both resolved at `help_modal.go:73-79`.
- "Do" step 5 (delete the copied-constant comment): done. `theme_panel_footer.go`'s width comment no longer references `helpKeyColumnWidth`; `grep helpKeyColumnWidth` over non-test sources now hits only `help_modal.go`.
- Notes: the plan prescribed `keyTok, labelTok theme.Token`; the implementation takes `keyStyle, labelStyle lipgloss.Style`. This is not drift — a token-typed signature cannot satisfy criterion 3 without either a `bold bool` parameter or a branch inside the builder. The style-typed seam is the correct resolution of the conflict between "Do" step 1 and acceptance criterion 3, and the criteria are the governing artifact.

TESTS:
- Status: Adequate, with one redundant test
- Coverage (`internal/tui/key_column_row_test.go`, added by this commit):
  - `TestHelpModalRow_ByteIdenticalAcrossExtraction` (:56) and `TestThemePanelFooterRow_ByteIdenticalAcrossExtraction` (:70) compare each caller against a verbatim reproduction of its pre-extraction body (`preHelpModalRow` :11, `preThemePanelFooterRow` :27) across both palettes × colourless on/off × the full descriptor set (`keyColumnRowEntries` :41 unions the sessions/projects/preview/panel/confirm scopes) — and, for the panel, across widths {0, min inner, preferred inner}. This is the direct proof of criterion 2 and would fail on any drift in pad arithmetic, measurement, canvas painting or segment order.
  - `TestKeyColumnRow_PadsOnlyWhenTheGlyphIsNarrowerThanTheColumn` (:119) pins the load-bearing edge case the builder documents — narrower / exactly-at / wider than the column — including the stray-SGR-pair hazard of unconditional padding.
  - Existing suites were left untouched with no expectation changes, as the task's test criterion required: `help_modal_test.go` (8 tests incl. `TestHelpModalGlyphs`, `TestHelpModalColourRoles`, `TestHelpModalDestructiveKill`), `help_modal_frame_test.go` (6 tests incl. `TestHelpModalBodyContiguousRows`), and `theme_panel_footer_test.go` (8 tests incl. `TestThemePanelFooter_KeyColumnIsFixedWidth` :95, `TestThemePanelFooter_KeyIsAccentKeyLabelIsTextMuted` :75, `TestThemePanelFooter_Colourless` :202). The commit stat confirms no test file other than the new one was modified.
  - The destructive-token and bold-weight divergences (criterion 3) are covered behaviourally — both goldens reproduce them, and `keyColumnRowEntries` includes the `k` (kill) and `d` (delete) `Destructive: true` entries.
- Notes: over-testing — `TestKeyColumnRow_BuildsBothSurfacesRows` (:86-117) asserts each caller's output equals `keyColumnRow` invoked with that caller's arguments. Every failure mode it can detect is already detected byte-for-byte by the two golden tests (a changed call-site token or width breaks the golden too), and it cannot detect what its name implies: if a caller re-inlined the body verbatim, the bytes would still match and the test would still pass. It is a duplicate assertion, not a delegation guard. Non-blocking.

CODE QUALITY:
- Project conventions: Followed. No `t.Parallel()` in the new tests (mandatory in this repo); no raw hex at the call sites (`colour_literal_guard_test.go` unaffected — everything routes through `theme.Token` via `headerStyle`); the builder is pure rendering with no logging, no state and no new package surface. Naming matches the codebase's `render*`/`*Row` house style.
- SOLID: Good. Single responsibility (one row's geometry), and the style-typed parameters invert the token/weight decision to the call sites so the builder is open to a third key-column surface without modification — exactly the shape criterion 3 asks for.
- Complexity: Low. One `if` in a 9-line function; both callers shrank to a single delegating expression.
- Modern idioms: Yes. `spaces()` is the repo's existing allocation-cheap helper and is preferred over `strings.Repeat` here for consistency with the panel path; the help path's `strings` import remains legitimately used by `helpModalHeader` (`help_modal.go:52`).
- Readability: Good. The retained comment at `modal_footer.go:29-31` explains the non-obvious pad-omission rule (styled empty run → stray escape pair) accurately, which is the one thing a reader would otherwise "simplify" wrongly.
- Comment accuracy: the comments on and around the changed code hold true. One pre-existing comment in a touched file is now factually wrong — see the `[do-now]` note below (it predates this task and is not caused by it).
- Issues: none blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/help_modal.go:15 — the comment `// Sized for the widest glyph ("^↑/↓") so labels share a left edge.` names the wrong glyph: `^↑/↓` is 4 cells, while the widest glyph the help body renders is `Home/End` (8 cells, `keymap.go:97`, preview scope). Replace with: `// Sized past the widest glyph ("Home/End", 8 cells) so labels share a left edge.`
- [quickfix] internal/tui/key_column_row_test.go:86-117 — delete `TestKeyColumnRow_BuildsBothSurfacesRows`. It re-asserts output already pinned byte-for-byte by the two pre-extraction golden tests above it, and it cannot fail on a re-inlined body, so it adds no failure mode while duplicating both call sites' argument lists a third time.
- [quickfix] internal/tui/modal_footer.go:32-41 — move `keyColumnRow` into a new `internal/tui/key_column_row.go`. Its test already lives at `key_column_row_test.go` (Go's file-pairing convention is broken as it stands, and `modal_footer_test.go` is a separate existing file), and its second consumer — the theme slide-over footer — is not a modal, so `modal_footer.go` mis-homes it.
- [idea] internal/tui/modal_footer.go:32 — `th` and `colourless` are used for nothing but `headerCanvasBg(th, colourless)`; passing a prepared canvas `lipgloss.Style` instead would cut the signature from 8 parameters to 7 and make the builder theme-agnostic, matching how the key and label styles are already resolved at the call site. Decide whether the builder should keep ownership of canvas painting (it currently paints pad and gap) before making the change.
