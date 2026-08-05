## Attempt 1

ISSUES:

- `internal/capture/theme_panel_fixture_test.go:310-312` — **the comment on the banned-import list claims a guard that does not exist.** It says "A loader is covered by the same net: `theme.Loader` is the only I/O-capable type in `internal/theme`, and **the field assertion below closes it**". There is no field assertion below — the only thing below is `declaresFakeThemeEnumerator`, a non-vacuity check on the type's existence.

  The structural half therefore does **not** prove what the test's own header comment claims ("A behavioural check can only prove the fake did not read the directories the test thought to poison; the import scan proves it cannot read any", `:299-300`): the scan cannot ban `internal/theme`, so a `fakeThemeEnumerator` holding a `theme.Loader` field would pass it silently. **This is the exact "absence reads as coverage" shape the task is written around, asserted about the guard rather than by it.**

  FIX: add the field assertion the comment names — AST-walk the `fakeThemeEnumerator` `TypeSpec` (`declaresFakeThemeEnumerator` already resolves it) and fail if any field's type resolves to `theme.Loader` or `*theme.Loader`. That makes the structural half actually close the loader case and makes both comments true.

  ALTERNATIVE: leave the scan as-is and correct the two comments to say the loader case is closed by the **behavioural** subtest (which it genuinely is — a fake using a real loader against the poisoned dir would return the built-ins union, not the declared rows, and the `rowSortKeys` comparison would fail). Cheaper, but it weakens a guard whose stated design is "checked structurally and behaviourally, because neither alone is enough". The field assertion is recommended.

  CONFIDENCE: high

- `internal/capture/theme_fake.go:122-132` (`repaintUnion`) — **the row repaint has zero test coverage.** Verified by tracing consumers: `Row.Theme` is read only by `applyThemePanelPreview` (`internal/tui/theme_panel.go:1105-1108`), no capture test drives an arrow key, and `rowSortKeys` deliberately compares identities only — so **emptying `repaintUnion`'s loop body fails nothing in the suite.**

  The executor's own justification names a real silent hazard ("`↓` in the live view applies a zero `Theme` and paints the frame colourless"), and the task requires the *sibling* hazard on `Resolve` to be driven rather than described. The same standard should apply here, particularly since §13.1 makes `capturetool --fixture` the human's route at the gate.

  FIX: add a subtest beside `TestFakeThemeEnumerator_ResolveReportsTheInjectedPalette` that drives the live path: a `driveToPanel`-style build with `--theme nord`, press `t` then `tea.KeyPressMsg{Code: tea.KeyDown}`, and assert the frame still carries `backgroundSGR(t, pinned.Canvas)` and is not colourless — the same two-legged shape the zero-palette subtest already uses. Optionally pair it with a direct `repaintUnion` unit assertion that every selectable row's `Theme` equals the injected palette while rejected rows keep a nil `Rejection`/zero `Theme`.

  CONFIDENCE: high

NOTES:

- **(a) Both flagged deviations judged correct by the reviewer.**
  - The `Deps()` signature change (33 call sites) is justified and the reviewer would have made the same call: the task's own instruction is "thread the palette in at the site where the constant nomination is built", and making `Deps` that site collapses the nomination and the panel seam into one construction so they cannot disagree. A second `DepsAt(th)` would have left `Deps()` returning a panel-less seam set — the "absence reads as coverage" shape §13.3 warns about — reachable from any future call site. Churn is purely mechanical and compiler-enforced.
  - The row repaint **masks nothing**: the only consumer of `Row.Theme` is `applyThemePanelPreview` (the arrow-preview). No still frame reads it, so it cannot affect any capture or guard assertion, and it cannot hide a product bug. Leaving rows zero-valued would genuinely paint the live `capturetool` view colourless on `↓`. Second-order effect checked: because all rows carry the same palette, `applyThemePanelPreview`'s `row.Theme == m.activeTheme` short-circuit makes arrowing a no-op rather than a wrong-colour flip — the safe degradation. (Its lack of coverage is ISSUE 2 above.)
- **(b) The palette-injection contract holds.** `TestPanelFixture_PaletteFollowsTheThemeFlag` renders each fixture under two palettes and asserts each frame carries its own canvas and *not* the other's — traced, and it catches **any** hard-coded palette, because a hard-coded third theme makes both "carries its own canvas" legs fail. The zero-resolution case is driven, not described: the subtest asserts the frame carries no `48;2;` background at all after a zero apply. "No built-in's hex in a synthetically-themed frame" is covered transitively and the coverage is real — a fake reporting a built-in would apply it over the synthetic palette, emptying the A-observed set, which fires both `TestThemeSwapGuard_RenderIsTruecolor` and the emptiness floor in `TestThemeSwapGuard_EveryBValuePresentInUnion`.
- **(c) The guard proof is genuine, not assumed.** `TestPanelFixture_PanelIsCompositedUnderTheGuard` takes the A-frame from the real `RenderSwapRender(a, b, harnessWidth, harnessHeight)` — the identical drive the guard uses — and asserts the `│` left border, the `Themes` header, and the **absence of both §14A blocked-entry flashes**, with the refusal-copy strings matching `themePanelNarrowEntryFlash`/`themePanelShortEntryFlash` verbatim.
- **SPEC_CONFORMANCE conformant.** §13.3's four declared inputs all present and separated as required. The union is faked wholesale as an ordinary `theme.Union` value — no loader, no directory — keeping §7.1's guard intact. §8.2 honoured in the constant fixture (exactly one `SlotConstant` record, so `theme.Badges` yields the bare `●`). §9.2's cursor rules hold. §9.1's `Themes` header copy correctly overrides the mock's `Theme`. The coherence rule is in both doc comments and both tapes.
- **ACCEPTANCE_CRITERIA all met.** `TestPanelFixture_NoConfigAccess` stages a *valid* decoy drop-in and asserts it is not listed, plus asserts nothing was written. `TestPanelFixture_CursorSeedDoesNotApplyATheme` drives a deliberately incoherent pairing, which is the only form that can fail. The registry drift check AST-scans the switch against the slice. `theme_swap_guard_test.go` is untouched — the only test-table edit is `capturedStates()`, a different test's completeness table where the edit is mandatory.
- **ARCHITECTURE sound.** `themeEnumerator()` returning a nil interface for panel-less fixtures composes correctly with `liveThemeEnumerator`'s typed-nil guard. The `initialThemeCursor` seed is not one-shot (unlike `initialCursor`, which `evaluateDefaultPage` clears); re-opening the panel in the live view cannot strand the cursor off the painted theme precisely because the fake reports one palette everywhere — which is why the one-palette fake and the persistent seed have to stay paired.

**VISUAL — the reviewer's own comparison against the Paper frames:**

What matches, and matters most:
- **Badge vocabulary and placement** — the part §9.14 says has no prior art. `● light` on `tokyo-night-day`, `● dark` on `nord`, both right-aligned to the same column; the constant frame's bare `●` sits at exactly the column where `dark`/`light` end, which is the mock's own treatment. This is the frame's whole subject and it is right.
- **Cursor treatment** — `▌` + `bg.selection` tint + bold `text.on-selection` name, band spanning the panel's inner width. Matches the mock.
- **Cross-element alignment**, checked at pixel level: the panel's `Themes` header baseline is on the same row as the `PORTAL` wordmark, and the panel's header rule is on the same row as the main title rule. The header label aligns with the cursor-bar column while row labels indent past it — the same relationship the mock uses.
- **Vertical footer** — all four labels sharing one left edge (the 3-cell fixed key column resolves `esc` flush without stepping its label).
- **Left-border-only, full-height slide-over**, cutting the main footer mid-label (`m mult`) — which §9.1 explicitly sanctions.

Differences, with the reviewer's judgement:
1. **Header reads `Themes`, the mock reads `Theme`.** Implementation is right; §9.1 pins `Themes`. The executor did not list this one — flagged so nobody "fixes" it toward the mock at the gate.
2. **Panel body/border resolve to `canvas`/`border`, not the mock's literals.** Correct — §9.1 refuses both by name. Consequence worth the human's eye: in the **light** capture the panel/list separation is genuinely faint, since the border is the only distinguishing surface. The spec's decided outcome, not a defect, but the one place the mock's extra contrast was doing visible work.
3. **Four rows vs the mock's five.** Fine — one drop-in is sufficient to demonstrate §9.5's built-in/drop-in indistinguishability. Minor: `catppuccin-latte` names a real-world theme Portal does not ship, so a screenshot reader could momentarily take it for a built-in; the mock's invented `nord-lee` avoided that. Cosmetic only.
4. **Constant frame renders light where the mock is dark.** Forced by the coherence rule given the chosen cursor row, and judged a **net gain**: it is the only frame in the set showing the panel chrome under a light palette, and it makes §9.2's mixed-mode flash visible.
5. **One-row rhythm vs the mock's px gaps.** Correct per the project's terminal-native spacing decision.
6. **Main footer content differs from the mock.** Expected — task 8-14's §14 revision post-dates the artboard.
7. **No pagination dots** (12 sessions fit one page) vs the mock's three. Not a panel property.
8. **Sessions cursor on row 1 vs the mock's `fab-flowx-explore`.** Cosmetic. The fixture already has an `initialCursor` field, so matching the mock would cost one line — optional.

Further non-blocking notes:
- `builtinPalette` (`theme_panel_fixture_render_test.go:375`) duplicates `darkBuiltinTheme`/`lightBuiltinTheme` (`swap_harness_test.go:112,176`) — same package, same body. The generic form supersedes both; collapsing would leave one loader per package instead of three. DRY nit in a package otherwise fastidious about single-sourcing helpers.
- `themePanelEnumeration()` declares an `Entry` with a nil `Rejection` and a zero `Theme` — a shape the real `Enumerate` cannot produce. Inert today (the fake's `Reassemble` ignores the enumeration and the doc says so); noted in case a later task makes the enumeration load-bearing.
- `Fixture.themeEnumerator` gates the seam on `len(themeUnion.Rows) == 0`. §13.3 still owes a `⚠ dir unreadable` fixture, which sets `Union.DirUnusable` — that fixture will still have built-in rows so the gate holds, but it is the one input combination worth a second look when that task lands.
- "Fresh write" of the PNGs can only be confirmed retrospectively: both files are new, today-dated, carry distinct content hashes, and their contents match what the in-process render asserts. Consistent with the executor's report; the reviewer could not independently re-run VHS (loopback networking with the sandbox disabled).
- Only `theming-system-8-15` moved in `.tick/tasks.jsonl`; no unrelated edits swept in.
