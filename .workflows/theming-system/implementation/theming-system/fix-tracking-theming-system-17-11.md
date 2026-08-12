## Attempt 1

ISSUES:
- `testdata/vhs/theme-panel-narrow.tape:63,76-82` and `testdata/vhs/theme-panel-min-height-message.tape:79,101-105` still size their vhs terminal to 54 columns (`Set Width 690`), while the fixtures now pin 63. The filter rewrites the resize unconditionally, so a re-run renders a 63-column frame into a 54-column terminal — the capture wraps (or clips the panel's right edge off entirely), i.e. the tape now produces a garbled PNG rather than a different one. `theme-panel-narrow.tape:17-20` also now asserts the opposite of the code ("A fixture cannot resize itself either … the stepped-down width is reachable ONLY by capturing at a narrower terminal"), and both live-view instructions ("narrow the window to about 54 columns") are obsolete. The height half of the min-height tape (366px → 10 rows) already matches the declared floor; only the width is wrong.
  FIX: bring both tapes onto the declared 63 columns — set `Set Width` to the pixel value that resolves to 63 columns (the tapes record the 690px → 54col mapping, so ≈805px; confirm by running `vhs` and reading the frame), delete the now-false "A fixture cannot resize itself" paragraph, and replace the "narrow the window to about 54 columns" live-view line with a note that the fixture pins its own width so no resize is needed.
  ALTERNATIVE: delete both tapes and their PNGs now. They are already declared scaffolding (§13.2), `capturetool` no longer needs an instruction to reach either frame, and this needs no `vhs` run to verify a pixel mapping — cheaper and zero-risk, at the cost of no producible PNG for those two frames until a new tape is written.
  ORCHESTRATOR DIRECTION: take the FIX, not the alternative. The feature has not reached sign-off, so the tapes are still the reproduction route for those two frames; a wrong tape is worse than no tape, but a correct one is better than neither. Verify the pixel→column mapping by running `vhs` on the tape and reading the rendered frame's column count rather than trusting the arithmetic. NOTE (project memory): vhs captures sometimes silently fail to write the PNG — confirm a fresh write (changed file hash) before trusting a capture.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- `internal/capture/fixtures.go:498-501` — cardinality claim ("observable on no other frame"), falsified by ordinary additive change far from the comment; the rest of the claim is the useful part.
  OLD:
  ```
  // Declares the terminal that lands exactly on the panel's height floor, at the
  // minimum width: the floor's arithmetic — one list row and one message row
  // beneath the header, with the standing footer intact — is observable on no
  // other frame. Capture with `--theme nord`.
  ```
  NEW:
  ```
  // Declares the terminal that lands exactly on the panel's height floor, at the
  // minimum width: what it exercises is the floor's arithmetic — one list row and
  // one message row beneath the header, with the standing footer intact. Capture
  // with `--theme nord`.
  ```

NOTES:
- `testdata/vhs/README.md:223-226` ("Adding (or removing) a fixture", step 2) still tells an author to fix `Width`/`Height` in the tape and record the column count in a comment, with no mention of the new declaration — the exact drift this task removed will be reintroduced by the next author following the documented procedure. Step 1 already carries the parallel sentence for `captureKeys` ("declare it in `captureKeys` rather than only in the tape, so the tape and the offline driver cannot drift"); one matching clause for a fixture whose frame is a geometry would close it. Non-blocking, and a natural companion to the tape fix — fold it in.
- Naming: `tui.ThemePanelMinWidthTerminal()` (63) reads as "the minimum terminal width the panel needs", which is what `minimumPanelTermWidth` (28) already means in `internal/capture/theme_panel_remaining_fixtures_test.go:32`. Both are now in scope in the same test file. The doc comment disambiguates, but something like `ThemePanelSteppedDownTerminalWidth` would say it without one.
- `themePanelConfirmFixture` (`internal/capture/fixtures.go:481-482`) still carries an external capture instruction for a size-dependent frame ("Capture at the panel's minimum width (the message slot may wrap there)") — its wrap is only visible at that width, and its tests pass 54 columns by hand. Out of this task's scope (its data differs from its base, so it is not the duplicate-frame problem), but the mechanism to fix it now exists.
- `capturetool` says nothing when the live terminal is *narrower* than a declared size — the frame silently wraps. The tapes used to carry the "resize your window" instruction that covered this by hand. A one-line stderr note when the terminal is smaller than the declared size would keep the human gate honest; not required by this task.
- Reviewer verification worth recording: it confirmed the derived ends evaluate to 63 columns × 10 rows, that Bubble Tea v2 routes the initial resize through the filter (so the live path genuinely honours the declaration), and that the consequential `capturedStates()` change (`● light` now absent at the floor) is a correct tightening — a tripwire that fires if the fixture ever reverts to the caller's size.
