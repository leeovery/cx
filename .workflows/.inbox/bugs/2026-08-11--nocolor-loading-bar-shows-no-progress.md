# The NO_COLOR loading bar shows no progress

Under `NO_COLOR`, the cold-boot loading page's progress bar carries no progress
information at all. It renders identically at 0% and at 100%, so a user running
colourless sees a static block for the whole of the loading window and has no
way to tell whether Portal is making progress, stalled, or nearly done.

The bar is drawn by `renderLoadingBar` in `internal/tui/loading_view.go`. It
takes a `colourless` flag, and on that branch it returns the filled glyph
repeated `filled` times followed by the track glyph repeated `barW-filled`
times, with no styling applied to either run. The two glyph constants declared
at the top of the same file — `loadingBarFilledGlyph` and `loadingBarTrackGlyph`
— are both `"█"`. The filled and track portions are therefore the same
character with the same (absent) styling, and the rendered string is `barW`
identical block glyphs regardless of what `fraction` is passed in. On the
coloured path the two runs are distinguished by `AccentPrimary` and `BgSubtle`
backgrounds, which is where the distinction actually comes from.

This contradicts the `NO_COLOR` carve-out as described in CLAUDE.md, which says
that under `NO_COLOR` state stays glyph-backed — `●`/`▌`/`⚠`/`✓` — and never
colour-only. The loading bar is the one piece of state that is currently
colour-only.

The window this affects is not brief. The loading page is shown on the cold +
TUI startup path, gated by `LoadingMinDuration` at 1.2 seconds and then by
however long the ten-step bootstrap takes — restore of saved sessions in
particular can run well beyond that. The `N/M` counter on the `Restoring
sessions` step still conveys something, but the bar itself is inert.

The existing tests appear to encode the current behaviour rather than catch it:
`internal/tui/loading_view_test.go` has assertions at roughly lines 80 and 144
that check every rune of the bar equals `loadingBarFilledGlyph`, which passes
precisely because the track glyph is the same character.

This surfaced during the repo-wide comment audit on the `chore/comment-strip`
branch. A comment above the colourless branch asserted that the bar "drops both
backgrounds and renders filled glyphs over track glyphs — still determinate",
which is not what the code does. That false comment has been removed in PR #5,
but the behaviour it described was left untouched, since changing it is a
behaviour change and that PR is comment-only.
