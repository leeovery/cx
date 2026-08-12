TASK: theming-system-17-13 — Add The Styled-Blank Helper To The Contrast Swatch (tick-3c936d, severity low, sources: duplication)

ACCEPTANCE CRITERIA:
1. `lipgloss.NewStyle().Background(...).Render(strings.Repeat(" ", ...))` appears once in `internal/capture/swatch.go`.
2. The contrast-validation swatch renders byte-identically before and after at both a sized and a zero-sized model.
3. `fillCanvas` still owns its own loop and does not call into `internal/tui`.

STATUS: complete

SPEC CONTEXT:
The specification touches this surface at line 1613 only: the contrast-validation swatch is `capturetool`'s standalone labelled-tint branch (MV spec §16.5 lock-in/bail surface) that deliberately does NOT route through `tui.Build`, and is the surface satisfying the human-eyeball gate §7.5/§13.5 requires for a new light theme's pinned tints. The spec's only demand on it in this topic was re-pointing it from `--appearance` to `--theme` (delivered earlier in the plan). This task is a phase-17 analysis-remediation refactor with a zero-behaviour-change contract — which is exactly the right contract for an eyeball-gate surface, since any byte drift silently changes what a human is asked to sign off on.

IMPLEMENTATION:
- Status: Implemented
- Location: `internal/capture/swatch.go:160-165` (the new `fill` helper, placed directly beside `onTint` at `:154-156` as instructed); call sites at `:72` (`fillCanvas` blank), `:79` (`fillCanvas` gap pad), `:188`/`:189` (`subtleBand` bar + track), `:198` (`padBand` gap). Commit `01c3cb78`.
- Notes:
  - AC1 verified: `lipgloss.NewStyle().Background(...).Render(strings.Repeat(" ", ...))` occurs exactly once, at `:164`. The only other `strings.Repeat` in the file is `:194`'s `─` border rule, a different (styled-text) idiom correctly left on `onCanvas`.
  - AC2 verified by construction at all five sites. `subtleBand`'s two widths are positive constants (18, 56-18) so those renders are unconditionally unchanged. `fillCanvas`'s pad and `padBand`'s gap previously guarded with `if gap > 0` and now rely on `fill`'s `n <= 0 -> ""` early return, which reproduces the guarded behaviour exactly.
  - **The deviation from the plan's suggested mechanism is correct and load-bearing.** The task's Do-step 1 proposed `strings.Repeat(" ", max(n, 0))`; the implementer used an early return instead. I checked `charmbracelet/x/ansi@v0.10.4` `Style.Styled` (`style.go:50-55`): a non-empty style returns `s.String() + str + ResetStyle`, so `lipgloss.NewStyle().Background(tint).Render("")` yields an escape-only, zero-width but **non-empty** span. The literal `max(n, 0)` reading of the plan would therefore have appended bytes at every non-positive pad and violated AC2. Do-step 4 ("whichever leaves the rendered bytes identical") sanctions the choice, and the `fill` doc comment states the reason precisely. This is the one place the task could have gone wrong, and it did not.
  - AC3 verified: `fillCanvas` (`:63-86`) retains its clamp-then-pad-then-backfill loop verbatim — only the two blank *segments* moved; `internal/capture/swatch.go` imports no `internal/tui` (grep confirms, in the file and its test).
  - No collateral risk: `renderSwatch`/`subtleBand`/`padBand` are package-private with no consumer outside `internal/capture`; external callers touch only `NewContrastValidationModel`/`ContrastValidationFixture` (`cmd/capturetool/main.go:91-92`). No committed golden depends on these bytes.
  - Incidental rename `empty` -> `track` (`:189`) now agrees with the caption at `:132` ("bar: accent.primary over the track") — a small net readability gain, not scope creep.

TESTS:
- Status: Adequate
- Coverage:
  - `internal/capture/swatch_test.go:168-207` `TestSwatchViewFillsTheCanvasVerbatim` — table-driven, named subtests, compares `s.View().Content` byte-for-byte against `wantFilledCanvas` (`:149-166`), an independent re-expression of the pre-refactor `if gap > 0` shape. Three cases: explicit size (100x30), truncating size (40x5), and the sizeless 80x24 fallback. This satisfies the task's "at an explicit size and at the 80x24 fallback" requirement, plus a truncation case the task did not ask for.
  - The truncating case is the sharpest assertion in the set and is not redundant: at w=40 the swatch's content lines exceed the width, so the gap is negative — the reference adds nothing while a `max(n, 0)` implementation would add an escape-only span. That case is what mechanically pins the AC2 rationale and would fail loudly if a future "simplification" reintroduced the plan's literal suggestion.
  - `internal/capture/swatch_test.go:209-217` `TestFillIsEmptyForNonPositiveWidth` — pins `fill(tint, n) == ""` for 0, -1, -12, exactly as the task's second test bullet requires.
  - The positive path is not left unpinned by omission: pre-existing `TestSwatchBandsCoverEveryPinnedTint:140-144` asserts `subtleBand`'s exact bytes as `bar+track` built independently, which is two `fill` calls' output verbatim.
- Notes:
  - Not over-tested. The line-count (`:196`) and per-line-width (`:200-204`) assertions that follow the byte-equality check are redundant *conditional on the oracle being right* — but they guard the hand-written oracle itself (a reference that silently stopped padding, or emitted the wrong number of rows, would make `got` and `want` wrong-and-equal). That is a deliberate and correct use of a second axis, and the `:199` comment explains why the width check is `>=` rather than `==`. I would not remove them.
  - `wantFilledCanvas` follows the file's existing `wantStyle` (`:40`) helper-naming convention. No `t.Parallel()` (per CLAUDE.md). `.View().Content` matches established repo usage (`cmd/open_theme_commit_test.go:109`, `cmd/prefs_translation_persist_test.go:385`).
  - One genuine residual gap, non-blocking: `padBand`'s non-positive branch — the one clamp site the task's Do-step 4 explicitly called out — is not independently pinned. `TestSwatchBandsCoverEveryPinnedTint:132` checks `lipgloss.Width(surface) == bandWidth`, which a zero-width escape-only span would satisfy unchanged, so a regression to `max(n, 0)` would be caught at `fillCanvas` but not at `padBand`. See the note below.

CODE QUALITY:
- Project conventions: Followed. Comment style matches the file's existing prose-rationale form (`:62`, `:88-89`, `:100-101`) rather than the name-prefixed doc form — consistent with the surrounding package, and appropriate for an unexported helper. Table test uses named subtests per `.claude/skills/golang-testing` rule 1.
- SOLID principles: Good. `fill` is a single-responsibility leaf that mirrors `onTint`'s shape (`(token-ish, tint) -> styled output`); no new coupling introduced, and the swatch's deliberate independence from `tui.Build` is preserved.
- Complexity: Low. Net -6 lines of production code; `fillCanvas`'s loop body dropped a three-line conditional for one expression, and `padBand` collapsed from five lines to one.
- Modern idioms: Yes. The early-return guard is the idiomatic Go shape here and, per the analysis above, the only correct one.
- Readability: Good. `canvas := s.th.Canvas.Color()` reads better than the previous hoisted `canvasStyle`, and the `empty` -> `track` rename aligns the identifier with the user-visible caption.
- Comment accuracy: Verified true against the code and against the dependency. `fill`'s comment ("A non-positive width renders nothing at all — not an empty styled span — so callers can hand over an unguarded width difference without adding bytes") is accurate on both clauses: the guard does return `""`, and the contrasted alternative genuinely would emit bytes (`ansi.Style.Styled` at `style.go:50-55`). No process-artifact references, no restated code.
- Security: N/A — offline dev-only capture surface, no input handling.
- Performance: `fill` constructs a `lipgloss.Style` per call where `fillCanvas` previously hoisted one across the loop. This is an offline validation renderer bounded by terminal height (<= a few dozen lines); the allocation is immaterial, and hoisting would defeat the task's entire purpose. Not a finding.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/capture/swatch_test.go:104-145 — add a case to `TestSwatchBandsCoverEveryPinnedTint` (or a small dedicated `padBand` test) asserting `padBand(content, tint) == content` when `lipgloss.Width(content) >= bandWidth`. `padBand` is the pad site Do-step 4 named, but its non-positive branch is currently pinned only indirectly: the existing `lipgloss.Width(surface) == bandWidth` check at `:132` is blind to a zero-width escape-only span, so a future change of `fill`'s guard to `max(n, 0)` would be caught at `fillCanvas` (via the "truncating size" case) and silently pass here.
