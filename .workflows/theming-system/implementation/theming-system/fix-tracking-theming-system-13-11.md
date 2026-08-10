## Attempt 1

ISSUES:
- `internal/tui/theme_row.go:195-206` (`themeRowReason`) — at the new minimum the label-truncation floor can exceed what the composition has left, so the row renders ONE CELL WIDER THAN THE PANEL. `compose` returns `max(remaining, themeRowLabelFloor)` (:162); at inner 22 an unbadged `missing tokens` row (cost 15) with a label of ≤3 cells leaves `remaining = 3`, the floor clamps the budget up to 4, and the row renders 23 cells. Rendered through the real delegate and the real panel block:
  ```
  line  5 width=25 "│ ▌ nord                 "   <-- panel declares 24
  line  6 width=25 "│   ab   ⚠ missing tokens"
  ```
  The whole list body widens, so the slide-over overflows its declared width and its composite position. This is a REGRESSION INTRODUCED BY THIS TASK — at the old inner width of 25 the identical row rendered exactly 25 cells. It is reachable in production: a drop-in `ab.theme` (any ≤3-char slug) that is missing tokens, on any terminal 28–63 columns wide. `missing tokens` (14 cells) is the only reason long enough to trigger it; `reserved name` lands exactly on the floor and is safe. It also silently falsifies `theme_row.go:26-29`'s standing claim that "the panel refuses to open at all at those widths".
  FIX: charge the label at least its floor when deciding whether the reason fits, so the reason — which the row-rendering rule makes the first element dropped — is dropped rather than the row overflowing. In `themeRowReason`, change `if free-lipgloss.Width(it.Row.Label()) < cost` to `if free-max(lipgloss.Width(it.Row.Label()), themeRowLabelFloor) < cost`. Blast radius is nil for labels ≥ 4 cells (`max` is the identity there), so no existing expectation moves. Add a case to `internal/tui/theme_row_test.go` at `themeRowTestMinWidth`: a row with a ≤3-cell slug and `theme.ReasonMissingTokens` renders exactly `themeRowTestMinWidth` cells with only `⚠` trailing — the same shape as the existing badge-over-reason cases.
  ALTERNATIVE: clamp the budget to `remaining` in `compose` and let the label render below the floor. Cheaper, but it contradicts the stated floor and degrades the one string the user needs to identify their broken file, rather than the reason doctor repeats in full. The `themeRowReason` fix is recommended.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/capture/fixtures.go:1178-1184` — with the message fixtures now also capturing at the minimum (both 54 columns), "the step is rendered by nothing" is false.
  OLD:
  ```
  // IT IS THE ONLY OBSERVABLE CHECK ON THE LADDER. The shrink is staged — the
  // preferred width while the content region is at least twice it, the minimum below
  // that — and every other panel fixture bar the message ones is captured wide enough
  // to take the preferred width, so without this frame the step is rendered by
  // nothing. A fixture cannot resize itself either: it opens through captureKeys and
  // is a one-shot render, so the narrowed stage is reachable ONLY by capturing at a
  // narrower terminal.
  ```
  NEW:
  ```
  // IT IS THE LADDER'S OWN FRAME. The shrink is staged — the preferred width while
  // the content region is at least twice it, the minimum below that — and the panel
  // fixtures whose subject is the panel itself are all captured wide enough to take
  // the preferred width, so the step is what this frame is for. A fixture cannot
  // resize itself either: it opens through captureKeys and is a one-shot render, so
  // the narrowed stage is reachable ONLY by capturing at a narrower terminal.
  ```
- `internal/capture/theme_panel_remaining_fixtures_test.go:304-307` — same false claim after the message frames moved onto the same width.
  OLD:
  ```
  // The geometry rule's ladder is a STAGED shrink — the preferred width while the
  // content region affords it, the minimum below that — and this frame is its ONLY
  // observable check: every other panel fixture but the message ones renders at the
  // preferred width, so nothing else shows the step.
  ```
  NEW:
  ```
  // The geometry rule's ladder is a STAGED shrink — the preferred width while the
  // content region affords it, the minimum below that — and this frame is where the
  // step is checked: the panel fixtures whose subject is the panel itself all render
  // at the preferred width.
  ```
- `testdata/vhs/theme-panel-narrow.tape:1` — the frame no longer sits between the two widths; it sits on the minimum.
  OLD: `# vhs tape: the §9.8 slide-over DEGRADED between its preferred and minimum widths.`
  NEW: `# vhs tape: the §9.8 slide-over stepped DOWN to its minimum width.`
- `testdata/vhs/theme-panel-narrow.tape:13-19` — still states the deleted proportional rule ("takes half the content region") and "the middle of that ladder".
  OLD:
  ```
  # §9.8's doctrine for a space shortage is DEGRADE, NEVER REFUSE: the panel takes
  # half the content region, clamped between a minimum and a preferred width, and
  # refuses only once even the minimum cannot render. Every other panel fixture is
  # captured wide enough to take the preferred width, so the middle of that ladder
  # is rendered by nothing else. A fixture cannot resize itself either — it opens
  # through captureKeys and is a one-shot render — so the band is reachable ONLY by
  # capturing at a narrower terminal, which is what this tape is.
  ```
  NEW:
  ```
  # §9.8's doctrine for a space shortage is DEGRADE, NEVER REFUSE: the panel takes
  # its preferred width while the content region is at least twice it, steps down to
  # the minimum below that, and refuses only once even the minimum cannot render.
  # The panel fixtures whose subject is the panel itself are captured wide enough to
  # take the preferred width, so the step is what this frame is for. A fixture cannot
  # resize itself either — it opens through captureKeys and is a one-shot render — so
  # the stepped-down width is reachable ONLY by capturing at a narrower terminal,
  # which is what this tape is.
  ```
- `testdata/vhs/theme-panel-confirm.tape:90-94` — states the old formula, the old 27-column minimum, and a 60-column threshold that is now 64.
  OLD:
  ```
  # The width is the load-bearing half. 54 columns leaves 50 content columns, and
  # §9.8's ladder takes half of that — 25, below the 27-column minimum, so it clamps
  # UP to the minimum. That is the narrowest panel that renders at all, and the width
  # at which the confirm's copy wraps. Any terminal from 60 columns up walks back into
  # the degraded band and the line fits on one row again.
  ```
  NEW:
  ```
  # The width is the load-bearing half. 54 columns leaves 50 content columns, below
  # twice the 30-column preferred width, so §9.8's ladder steps down to its 24-column
  # minimum. That is the narrowest panel that renders at all, and the width at which
  # the confirm's copy wraps. A terminal of 64 columns or more takes the preferred
  # width and the line fits on one row again.
  ```
- `testdata/vhs/theme-panel-min-height-message.tape:34` — the minimum is 24, not 27.
  OLD: `# frame. Note the width too: it is the same 27-column minimum the confirm frame uses,`
  NEW: `# frame. Note the width too: it is the same 24-column minimum the confirm frame uses,`
- `testdata/vhs/theme-panel-min-height-message.tape:97-99` — same stale formula and minimum.
  OLD:
  ```
  # and the arithmetic stops being visible; 12 refuses to open at all. The width is
  # the same minimum theme-panel-confirm.tape uses — 54 columns leaves 50 content
  # columns and §9.8's ladder clamps up to the 27-column minimum — so the two frames
  ```
  NEW:
  ```
  # and the arithmetic stops being visible; 12 refuses to open at all. The width is
  # the same minimum theme-panel-confirm.tape uses — 54 columns leaves 50 content
  # columns, below twice the preferred width, so §9.8's ladder steps down to the
  # 24-column minimum — so the two frames
  ```

NOTES:
- The visual gate was PASSED by the user: they kept the narrower 24–30 band. Do not revisit step 8.
- The ladder is genuinely two-stage and the refusing path returns the clamped minimum — verified.
- Deleting `TestPanelChrome_WiderLadder` lost no claim: its refuse/clamp subtest is duplicated in the untouched `TestPanelGeometry_WidthFloor`, its monotonicity claim survives in the geometry suite, and its remaining subtests asserted the rule this task deletes.
- The two re-derived assertions did not lose force: `TestPanelMessage_ConfirmPinnedCopy` still byte-compares the copy including the double space, at a width where `nord` survives untruncated. The capture-side relaxation to `strings.Fields` is forced and honest — at inner 22 the wrap now breaks BEFORE `y / n`.
- Non-blocking: `slotConfirmCopy` (`theme_panel_confirm_test.go:149-151`) derives its expectation from `themePanelConfirmText`, the same production composer under render, so `strings.Contains` would be vacuous if that composer returned "". Force survives only because the message-slot suite pins it independently. Asserting the invariant halves (e.g. `"clear constant auro"` plus the `"?  y / n"` tail) would restore it at no maintenance cost. Use your judgement.
- Non-blocking, pre-existing: `internal/tui/theme_panel_footer.go:53` names `d set as dark` as the widest footer row at 15 cells; it is actually `l set as light` at 16. Still clears the new inner 22 — flagged only because this task narrowed the margin it reasons about.
- Measured arithmetic (all through the production renderers): inner widths 22 (min) / 28 (preferred); `⚠ dir unreadable` 16; `⚠ couldn't save theme` 21; confirm frame cost 23. `● both` (6) ≤ `● light` (7) still holds.
