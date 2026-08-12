TASK: theming-system-11-2 — Single-Source The Theme-File Authoring Test Helpers Into internal/themetest

ACCEPTANCE CRITERIA:
- `internal/themetest` exists with the four exported helpers and a test-only package doc.
- No package declares its own theme-file line builder, value substituter, key remover or writer.
- Exactly one file mode is used for written fixture theme files.
- No production (non-`_test.go`) file imports `internal/themetest`.

STATUS: complete

SPEC CONTEXT: This is an analysis-remediation task (Phase 11, cycle 1), not a spec-behaviour task — its source is `duplication`, not a specification clause. The relevant spec context is only indirect: the theme-file format the fixtures are written against is the loader's six-rung rejection ladder (`bad name` → `reserved name` → `unreadable` → `bad syntax` → `bad colour` → `missing tokens`) and the closed 19-token vocabulary, both of which the new helpers derive from `theme.TokenNames()` rather than restating — so a vocabulary change moves every fixture with it. The task's own scope note explicitly excludes the built-in-palette loading helpers (later handled by 12-3) and the synthetic probe palette (14-1).

IMPLEMENTATION:
- Status: Implemented (and legitimately extended by later plan tasks 12-3 / 12-5 / 14-1 / 14-8 / 17-5, which added `Builtin`/`DefaultDark`/`DefaultLight`, `Render`/`Body`, `SyntheticPalette`/`SyntheticPair`, `MonochromeLines`, `LinesWithCanvas`/`WriteWithCanvas`/`WithDuplicateKeyAt` and `DenyRead` to the same package).
- Location:
  - `internal/themetest/theme_file.go:1-7` (test-only package doc), `:22` (`fixtureMode = 0o600`), `:28` (`Lines`), `:50` (`WithValue`), `:62` (`WithoutKey`), `:110` (`Write`).
  - Call-site swaps: `internal/theme/load_test.go` (local `themeLines`/`withValue`/`withoutKey`/`writeTheme` gone), `cmd/capturetool/main_test.go:195,202,209,288,313,434,464,493` (its four verbatim copies gone), `internal/tui/theme_panel_open_test.go:215-316`, `internal/tui/theme_row_test.go:388`, `cmd/open_theme_construction_test.go:497-501`.
- Notes:
  - All four named criteria hold in the current tree. A repo-wide search for the six retired identifiers (`themeFileLines`/`withTokenValue`/`withoutTokenLine`/`writeThemeFile`/`themeLines`/`withValue`/`withoutKey`/`writeTheme`/`writeThemeFileForTest`) returns no surviving *definition* of a line builder, substituter, key remover or line writer. The two survivors are thin delegations, not re-implementations:
    - `cmd/open_theme_construction_test.go:497` `writeThemeFile` is now a 1-line slug→filename adapter over `themetest.WriteWithCanvas` (a shape 14-8 deliberately settled on).
    - `cmd/theme_test.go:502/517/531` (`missingTokenLines`/`badColourLines`/`duplicateKeyLines`) wrap `themetest.WithoutKey`/`WithValue`/`WithDuplicateKeyAt` purely to add "the built-in still declares this key" tripwires — a documented, deliberate layer over the shared mutators, not a second implementation.
  - `internal/theme/load_test.go:662` `writeThemeBytes` is a raw-`[]byte` writer (the `loadEntryPoints` table needs to hand the same bytes to `LoadFile`/`LoadPath`/`LoadBuiltin`), a signature `themetest` does not expose. It writes at the same `0o600`, so it does not violate the one-mode rule.
  - Behavioural drift in the swap, both benign and doc-updated at the time: `cmd`'s `writeThemeFile` fixture body changed from "a built-in's own source with the canvas line swapped" to `themetest.Lines()` with the canvas swapped, and `internal/tui/theme_row_test.go`'s reserved-name-row fixture stopped carrying `testDarkTheme`'s values. Both tests assert on structure (which file parsed / whether a badge is attached), not on the palette's contents, so neither weakens.
  - Test-only enforcement is contributor discipline plus the `*testing.T` first parameter on `Write`/`WriteWithCanvas` — matching the stated `logtest`/`spawntest`/`transienttest` precedent, which likewise ship no import guard. The pure-`[]string` helpers (`Lines`, `Render`, `WithValue`, …) carry no structural barrier, but nothing imports them from production today.
  - `Write` (`theme_file.go:110`) has no doc comment; that is not an oversight — commit `915e7fcb` (comment audit) removed it because it restated the signature, which is the project's stated comment standard. Package doc lives in exactly one file (`theme_file.go`); `builtin.go`/`synthetic.go`/`deny.go` correctly carry none.

TESTS:
- Status: Adequate (one small redundancy).
- Coverage: `internal/themetest/theme_file_test.go` covers the acceptance-listed cases directly — `Lines()` produces a file the real loader accepts and round-trips to the expected canonicalised tokens (`:15`), every token gets a distinct value (`:32`), `WithValue` substitutes in place without reordering (`:49`) and yields exactly `ReasonBadColour` (`:67`), `WithoutKey` yields exactly `ReasonMissingTokens` and removes exactly one line (`:75`), the mutators do not write through to their input (`:119`), and `Write` returns the joined path at the single `0o600` mode (`:203`). The later-added surface (`WithDuplicateKeyAt`, `MonochromeLines`, `LinesWithCanvas`, `WriteWithCanvas`, `Body`, `Render`) is covered to the same standard. Assertions go through the real `theme.Loader`, so the fixtures are pinned against the production parser rather than a restated format.
- Notes: `TestBody_IsRenderOfLines` (`:245`) is a tautology — `Body()` *is* `Render(Lines())` — and is already implied by `TestBody_IsTheBytesWriteStages` (`:220`) plus `TestRender_IsTheBytesWriteStages` (`:232`), which both anchor against what `Write` actually stages. Everything else earns its place; no vacuous or duplicated assertions. The suite was not executed (reading only, per the review protocol), but the swaps are type-directed and every consumer call site type-checks against the new signatures on inspection.

CODE QUALITY:
- Project conventions: Followed. Test-only package with a `*Test`-suffixed name matching `logtest`/`spawntest`/`transienttest`; package doc states the test-only rule explicitly; external test package (`themetest_test`) for its own tests; no `t.Parallel()`; no production import.
- SOLID principles: Good. One responsibility (author `.theme` fixture bytes); the mutators compose over `Lines()` rather than each re-deriving a file; token names come from `theme.TokenNames()` so the vocabulary has a single owner.
- Complexity: Low. Every helper is a short, branch-light transform; `WithoutKey`/`WithDuplicateKeyAt` use `slices.DeleteFunc`/`Insert`/`IndexFunc` rather than hand-rolled index loops.
- Modern idioms: Yes — `slices.Clone`/`DeleteFunc`/`Insert`/`IndexFunc`, `strings.Cut` in the tests, `fmt.Sprintf` only where a generator is genuinely needed.
- Readability: Good. The value-generator's intent (lower-case hex, so a parse proves canonicalisation rather than echoing the file) and the one-mode rationale are both stated where they are decided, not restated elsewhere.
- Comment accuracy: Verified. `fixtureMode`'s "so no test depends on a permission difference" claim holds for everything routed through `Write`; `WithValue`/`WithoutKey`'s "leaving the input untouched" claim is true (both `slices.Clone` first) and is itself pinned by `TestMutatorsLeaveTheirInputAlone`; the package doc's "Test-only" claim holds. No comment references a task id, phase or spec section.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/themetest/theme_file_test.go:245 — delete `TestBody_IsRenderOfLines`; it asserts `Body() == Render(Lines())`, which is `Body`'s one-line body restated, and is already covered transitively by `TestBody_IsTheBytesWriteStages` and `TestRender_IsTheBytesWriteStages`.
- [quickfix] cmd/doctor_theme_test.go:37 — the one-mode rule stops at the line-based helpers: `themesDirIn` (0o644), `internal/capture/theme_panel_fixture_render_test.go:44` (0o644, staging `themetest.Body()`), `internal/theme/reserved_test.go:170` (0o644) and `internal/theme/load_test.go:666` (0o600) all hand-write theme-fixture bytes outside `themetest.Write`. Add `themetest.WriteBytes(t, dir, base string, data []byte) string` writing at `fixtureMode` and route those four through it, so byte-bodied fixtures share the single mode too. Out of this task's six named sites (all four predate it), hence non-blocking.
- [do-now] CLAUDE.md — the `themetest` inventory row still says "Two halves" and documents only the built-in accessors and the theme-file helpers; the package now also exports `Render`, `MonochromeLines`, `LinesWithCanvas`, `WriteWithCanvas`, `WithDuplicateKeyAt`, `SyntheticPalette`/`SyntheticPair` (synthetic.go) and `DenyRead` (deny.go). Change "Two halves" to "Three halves" and add a sentence: "The probe-palette helpers build the synthetic palettes a swap guard diffs between — `SyntheticPalette(t, red)` and `SyntheticPair(t, redA, redB)`, whose token values are unique within and across the pair — and `DenyRead(t, path)` stages an unreadable fixture and returns the OS error a caller asserts against." Adjacent to task 13-14 (which authored the row) rather than to 11-2.
