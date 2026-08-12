TASK: theming-system-3-3 — Retain the startup canvas hex and re-anchor the exit-time background restore

ACCEPTANCE CRITERIA:
1. `canvasHexFor` does not exist anywhere in the tree; `RestoreTerminalBackground`'s body references no theme token or nomination — a source guard proves the comparison reads only the retained hex.
2. Under an adaptive nomination the hex is captured when the gate resolves and equals the SELECTED member's canvas (dark → `#0B0C14`, light → `#E1E2E7`).
3. Under a constant nomination it is captured at construction, before any frame.
4. Mutating the active theme after capture does not change what `RestoreTerminalBackground` compares against.
5. The echo guard still skips for `#0b0c14`, `#0B0C14`, `#0b0c14ff`, `0b0c14` against canonical `#0B0C14`.
6. A non-hex reply (`rgb:…`) still falls through and emits the set-back.
7. An empty capture (`OriginalBackground() == ""`) still writes nothing.
8. Under `NO_COLOR` the hex is captured as normal, no canvas painted, set-back is a no-op.
9. Both launch sites (`cmd/open.go`, `cmd/capturetool/main.go`) call it with the program's output writer and behave identically.
10. CLAUDE.md's `tui` row is corrected and still carries the "do not drop this guard" warning.

STATUS: complete

SPEC CONTEXT: §11.4 requires the exit-time comparison value be captured and retained as model state rather than re-derived at exit from the active theme (which, under a switchable theme, is either a mid-session commit's palette or an uncommitted preview's — the one path where a colour the user never chose can be left stuck in their terminal after Portal exits). §11.3 pins that the guard needs no new race handling: the OSC 11 query is issued once from `Init`, so the comparison only ever needs the canvas in force during the startup window. §8.4 pins the capture moment — the theme the gate SELECTED, at the single moment the gate resolves, which is also when the first frame is composed. §13.4's swap-and-diff guard structurally cannot cover an exit-time OSC 11 write, so the mechanic carries its own named verification. §12.6 flags CLAUDE.md's `tui` row as the entry whose staleness is most dangerous.

IMPLEMENTATION:
- Status: Implemented (mechanism later re-homed by task 11-15 — the field moved from `Model` to the `themeState` sub-struct; contract unchanged, which the verifier context marks as intentional supersession, not drift).
- Location:
  - `internal/tui/theme_state.go:62` — `startupCanvasHex string` on `themeState`, documented as frozen at gate resolution and deliberately not moved with `active`.
  - `internal/tui/model.go:858-875` — `syncResolvedMode` selects the active member then calls `captureStartupCanvasHex`, which writes `themeState.active.Canvas.Value` (parsed, canonical) once the gate is resolved and leaves it empty while the gate is open.
  - `internal/tui/model.go:897-904` — `ApplyTheme` (the mid-session restyle/preview path) re-points only `active` + `applyCanvasMode`, with `startupCanvasHex` named as a deliberate exclusion.
  - `internal/tui/restore.go:20-32` — `RestoreTerminalBackground` early-returns under `colourless`, then compares `sameHexColour(original, m.themeState.startupCanvasHex)`; `sameHexColour`/`normaliseHex6`/`isHexDigit` untouched.
  - Launch sites: `cmd/open.go:691`, `cmd/capturetool/main.go:65` — both `os.Stdout`.
  - `CLAUDE.md:67` — `tui` row rewritten: "anchored to the retained startup canvas hex … must NEVER be re-derived from the active theme … the old `canvasHexFor` helper is deleted, and a source guard keeps it gone … Do not drop the guard".
- Notes:
  - Capture point is provably single-shot: `appearanceGate.resolve` returns true only on the first resolution (`internal/tui/appearance_gate.go:64-71`), so the two `syncResolvedMode()` call sites in `Update` (`model.go:1495`, `model.go:1500`) cannot re-fire on a late reply. The only other callers are `New` (`model.go:839`) and `armAppearanceDetection` (`model.go:845`), both pre-first-frame. The Phase-8/9 constant→adaptive conversion goes through `themeState.adoptRetainedReply` (`theme_panel_confirm.go:73`), which does not touch the anchor — so the retained hex genuinely cannot move mid-session.
  - `canvasHexFor` is absent from every `.go` file in the tree (verified by grep and by the tree-wide guard below). Its only surviving mentions are workflow docs and the CLAUDE.md sentence that records its deletion — correct.
  - The `colourless` early return (writes zero bytes under `NO_COLOR`) was folded in here as task 3-2's hand-off. It is stronger than the criterion's "no-op because nothing was painted" and is consistent with it; documented in the file comment and covered by test.
  - AC 2's exact hex values are anchored: `TestBuiltinCanvasValuesPinned` (`active_theme_test.go:15`) pins `testDarkThemeCanvas`/`testLightThemeCanvas` against the loaded built-ins, so the constants the anchor tests assert on cannot silently drift from the shipped `.theme` files.

TESTS:
- Status: Adequate.
- Coverage (one-to-one against the task's named tests):
  - `TestStartupCanvasHex_CapturedAtGateResolution` (`startup_canvas_hex_test.go:10`) — dark reply / light reply / timeout; asserts empty while the gate is open (so the test cannot pass vacuously) and that the value equals the SELECTED member's canvas, not the nomination's. AC 2.
  - `TestStartupCanvasHex_ConstantCapturedAtConstruction` (`startup_canvas_hex_test.go:39`) — dark + light constants, with a `modeResolved()` precondition proving construction-time resolution. AC 3.
  - `TestRestoreTerminalBackground_AnchoredToStartupHex` (`restore_test.go:42`) — captures via a real gate resolution, then mutates `themeState.active` to the other built-in and asserts both directions: an echo of the STARTUP canvas is still skipped, and an echo of the now-ACTIVE canvas is set back. This is the load-bearing test and it is written to fail loudly if the anchor slips to the active theme. AC 4.
  - `TestRestoreTerminalBackground_CanvasEchoGuard` (`restore_test.go:12`) — exact / lower case / trailing alpha / no leading hash / light exact, driven off loaded built-ins plus a seeded `startupCanvasHex` (successfully migrated off the retired `theme.MV.Canvas.Dark/Light`). AC 5.
  - `TestRestoreTerminalBackground_NonHexReplyStillSetsBack` (`restore_test.go:79`). AC 6.
  - `TestRestoreTerminalBackground_EmptyWritesNothing` (`background_restore_test.go:158`) — pre-existing, kept. AC 7.
  - `TestNoColor_HexCapturedAndSetBackIsANoOp` (`restore_test.go:91`) — asserts the hex IS captured, asserts `OriginalBackground()` is non-empty first ("or this test would pass vacuously"), then asserts zero bytes written. AC 8.
  - `TestRestorePath_ReadsNoTheme` (`restore_source_guard_test.go:39`) — three subtests: `restore.go` imports no `internal/theme`; the helper body's selector reads are confined to `{OriginalBackground, colourless, themeState.startupCanvasHex}` with an explicit `anchored` flag so a helper comparing against nothing cannot pass; and the retired helper name is absent from every `.go` file in the tree. AC 1.
  - `TestLaunchSites_RestoreIdentically` (`restore_source_guard_test.go:111`) — both sites present, called exactly once each, first arg `os.Stdout`, plus a "no third site" subtest. AC 9.
  - Downstream reinforcement (later phases, not this task's obligation but they exercise the anchor): `apply_theme_test.go:301-316` (50 swaps leave the anchor byte-identical), `theme_panel_arrow_test.go:482-500` (50 arrow previews), `theme_panel_commit_load_test.go:740-780` (constant→adaptive conversion), and `restore_divergence_test.go` (§11.4's two divergence cases).
- Notes:
  - No under-testing found: every acceptance criterion has a named test, and each assertion is written with a failure message that states the invariant rather than just the mismatch.
  - No meaningful over-testing. The one overlap — `TestSameHexColour`'s table (`restore_test.go:111`) covering case, trailing alpha and non-hex, versus the same shapes exercised through `TestRestoreTerminalBackground_CanvasEchoGuard` — is a legitimate helper-unit vs. behaviour split, and `TestSameHexColour` pre-dates this task.
  - Tests assert behaviour through the exported `RestoreTerminalBackground` plus the model's public construction path (`Build`/`New` + `Update`); the only internal reads are the anchor field itself, which is the thing under test.

CODE QUALITY:
- Project conventions: Followed. Unit-lane only, no `t.Parallel()`, no daemon/binary spawning; the source guard routes its repo walk through `sourceguardtest.GoSourceFiles` (the single-sourced scaffolding, post 13-3) rather than hand-rolling a walk; `themetest.DefaultDark/DefaultLight` are used instead of package-level theme vars; helper accessors are functions, not package-level vars, consistent with the "no package-level theme state" rule.
- SOLID principles: Good. `captureStartupCanvasHex` is a single-responsibility private mutator with exactly one caller; `restore.go` now depends on nothing but `io`, `strings` and `ansi` — the theme dependency is severed at the file level, which is what makes the source guard's import check meaningful.
- Complexity: Low. `RestoreTerminalBackground` is three guard clauses and a write; `captureStartupCanvasHex` is one branch.
- Modern idioms: Yes. No issues.
- Readability: Good. The doc comments carry the rationale (why set-back rather than reset, why `NO_COLOR` writes nothing, why the anchor must never be re-derived) without restating the code, and the guard test's failure messages explain the invariant they defend.
- Comment accuracy: Comments hold. `restore.go:10-19` matches the body exactly (including the `colourless` early return and the anchor). `theme_state.go:59-61` and `model.go:867-868`, `893-896`, `902-904` are all true of the current code. No spec-section, phase or task citations survive in production comments (11-3's sweep held here).
- Issues: None blocking. Two minor notes below.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] CLAUDE.md:67 (`tui` row) — the row says "A **canvas-echo guard** (`sameHexColour` against `Model.startupCanvasHex`)", but task 11-15 moved the field onto the `themeState` sub-struct, so `Model.startupCanvasHex` no longer exists. Replace that one occurrence with `Model.themeState.startupCanvasHex`. Worth fixing precisely because §12.6 names this row as the one an implementer reads immediately before touching this code; the rest of the row (retained startup hex, `canvasHexFor` deleted + source-guarded, "Do not drop the guard") is accurate.
- [quickfix] internal/tui/restore_source_guard_test.go:26-32 — `"themeState.startupCanvasHex"` is spelled twice, once in the `restoreComparisonReads` permitted-read list and once as the `restoreAnchorRead` const. Edit the slice to `{"OriginalBackground", "colourless", restoreAnchorRead}` so the anchor string cannot drift out of the permitted set (which would make the guard fail with a misleading "reads a forbidden selector" message rather than the intended one).
