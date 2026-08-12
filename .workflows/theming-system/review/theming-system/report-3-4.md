TASK: theming-system-3-4 — `capturetool --theme <slug|path>` replaces `--appearance`

ACCEPTANCE CRITERIA:
1. `--appearance` no longer exists on `capturetool`, and no code path resolves a `prefs.Appearance`.
2. Omitting `--theme` renders `tokyo-night` (the shipped dark default).
3. Slug/path discrimination exactly as tabled: `nord` → slug; `nord.theme`, `./nord.theme`, `/abs/nord.theme`, `sub/x`, `./mytheme.txt` → path.
4. `--theme ./broken.theme` exits non-zero with the matching content reason (`bad syntax` / `bad colour` / `missing tokens` / `unreadable`); nothing renders.
5. `--theme not-a-theme` exits non-zero naming the slug; no fallback render on any failure path.
6. `--theme ./Nord.THEME` renders with a `bad name` stderr warning; `./nord.theme` renders with a `reserved name` warning; `nord` renders with no warning.
7. A theme loaded from a path carries no slug (`Result.Slug` empty; nothing derives identity from the filename).
8. Both branches driven by the flag: the fixture path gets the **constant** nomination shape (no gate, no wait); the swatch renders the same resolved theme.
9. `NO_COLOR=1` still renders the colourless native-bg frame and wins over the pinned theme.
10. Theme resolution in `capturetool` produces zero records through a `logtest.Sink`.
11. `internal/capture` performs no XDG lookup and no prefs read; `TestPortalBinaryDoesNotImportCapture` passes.

STATUS: complete

SPEC CONTEXT:
§13.3 mandates the flag: `--theme` accepts a built-in slug *and* an explicit path; default `tokyo-night`; slug-vs-path discriminated by a path separator *or* the `.theme` suffix (the separator half exists so a real file with an unexpected extension isn't misreported as an unknown built-in); only the **content** reasons apply to a path; invalid input is a hard error with the §6.2 reason at non-zero exit, **never a fallback**; a candidate slug is derived from the basename **solely** for the two filename warnings and never as identity; `bad name` / `reserved name` warn on stderr without blocking and on the **path form only** (a slug names a built-in by design, and blocking would break §12.1's export-then-rename workflow). `--appearance` is removed, not kept alongside — §8.8 deletes `prefs.Appearance`/`WithAppearance`, so there is no mode left to pin. §13.3 also pins the constant nomination shape (`capturetool` always passes a single pinned theme — no gate, no wait — which is what keeps captures byte-deterministic) and re-points the contrast swatch to the same flag. §3.2 gives `Theme` no identity field, which is what makes a path-loaded theme's empty slug honest. §12.3 makes `capturetool` the fifth loader caller, wired to `log.Discard`. §13.1 is why this matters disproportionately: the harness is the only route to seeing a visual change before release, and the only visual-verification route a drop-in author has.

IMPLEMENTATION:
- Status: Implemented (with two deliberate later-phase supersessions, both correct)
- Location:
  - `cmd/capturetool/main.go:33` — `const defaultThemeSlug = theme.DefaultDarkSlug` (derived, not restated).
  - `cmd/capturetool/main.go:36-44` — `--fixture` + `--theme` only; error → stderr + `os.Exit(1)`. `--appearance` is gone, so passing it now fails at `flag.Parse` rather than being silently ignored.
  - `cmd/capturetool/main.go:86-99` — `resolveProgram` resolves the theme **before** branching, so the swatch and the fixture path cannot be judged against different palettes.
  - `cmd/capturetool/main.go:110-123` — `resolveTheme` + `isThemePath` (separator **or** `.theme` suffix; comment records why the separator half is load-bearing).
  - `cmd/capturetool/main.go:127-136` — slug branch: `!found` → error naming the slug and listing the built-ins; rejection → error wrapping the §6.2 reason; no warnings.
  - `cmd/capturetool/main.go:141-163` — path branch: `theme.LoadPath` → hard error on any rejection (never a fallback), then `warnAboutFilename` derives a candidate slug from `filepath.Base` for the `bad name` / `reserved name` stderr lines only; `result.Slug` is discarded, so identity never leaks out of the filename.
  - `cmd/capturetool/main.go:168-185` — `resolveModel` hands the palette to `fx.Deps(pinned)` → `theme.ConstantNomination(th)` (`internal/capture/fixtures.go:70-76`), and applies the `NO_COLOR` carve-out after the theme resolves.
  - `internal/theme/load.go:91-101` — `LoadPath`: read + `resultFromBytes("")` — content rungs only, no filename rung, empty slug.
- Notes:
  - Two intentional later-phase revisions, both accounted for and not drift: (a) `LoadPath` is now a package-level function rather than the `Loader` method the task text specified — changed by task 11-5 ("Split theme.Loader's Panel-Assembly Responsibility"), since neither the reserved set nor the event seam bears on an explicit input; the exported-surface guard (`internal/theme/theme_test.go:174`) records the new shape. (b) The loader is built with `theme.NewSilentLoader()` instead of an inline `theme.NewEventLogger(log.Discard())` — task 14-12 made `NewSilentLoader` the only route to a silent loader, and `NewLoader` panics on a nil seam. Both preserve §12.3's "capturetool writes nothing".
  - Warnings are emitted from `resolveProgram`, which runs before `tea.NewProgram` (`main.go:49-56`), so they land on the primary screen as required.
  - Error paths return an explicit `nil` model (`main.go:95-97`) rather than the zero `tui.Model` — avoids a typed-nil-in-interface and lets the "nothing renders" assertion be meaningful.
  - `--appearance` and its backing API are fully gone repo-wide; absence is guarded (`internal/prefs/appearance_api_guard_test.go`, `internal/tui/nomination_test.go:39`). No stale `--appearance` in docs, CLAUDE.md or any `testdata/vhs/*.tape`; CLAUDE.md line 25 documents `--theme <slug|path>` correctly.

TESTS:
- Status: Adequate
- Coverage: every acceptance criterion has a named test, and every test in the task's list exists (one renamed for the better):
  - Default → `TestResolveTheme_DefaultsToTheShippedDarkBuiltin` (`main_test.go:75`) — the task named it `..._DefaultsToTokyoNight`; the rename came with task 11-10's "derive, don't restate" single-sourcing and asserts both the value and that it is declared as `theme.DefaultDarkSlug`.
  - Discrimination → `TestThemeArg_SlugVersusPath` (`main_test.go:124`), the exact six-row table from the spec.
  - Unknown slug → `main_test.go:147`; invalid built-in via a `rejectingLoader` fake → `main_test.go:160` (the only way to drive that branch, since the embedded set is validated at build time).
  - All four path content reasons → `TestResolveTheme_PathContentReasonsAreHardErrors` (`main_test.go:185`), each asserting the reason is carried **and** that the returned palette is zero.
  - No fallback → `TestResolveTheme_NoFallbackOnFailure` (`main_test.go:240`) crosses three failure inputs × both fixture branches and asserts `resolveProgram` returns a nil model.
  - Warnings → `main_test.go:277` (`bad name`, both causes), `main_test.go:307` (`reserved name`, which also asserts the rendered palette is **not** the built-in `nord` — the candidate-slug-is-not-identity claim), `main_test.go:334` (every built-in slug warns nothing).
  - No slug from a path → `internal/theme/load_test.go:352`, which first proves each fixture *would* trip a filename rung under `LoadFile`, so the "no filename rung" claim is not vacuous; `TestLoadPath_RunsTheContentRungs` (`load_test.go:415`) covers the four content reasons at the loader level including a real permission-denied read.
  - Constant nomination → `TestResolveModel_PassesConstantNomination` (`main_test.go:380`) asserts the **first** frame paints the pinned canvas, which is exactly what an adaptive pair (blank held frame) would fail.
  - `NO_COLOR` → `main_test.go:397`, both directions, and checks the frame emits no background SGR.
  - Silence → `TestCaptureTool_ThemeResolutionIsSilent` (`main_test.go:459`) runs four resolutions (built-in, reserved-name file, unknown slug, broken file) through a `logtest.Sink` and ends with a positive control proving the sink is wired to the `theme` component.
  - Flag surface → `TestFlags_AreFixtureAndThemeOnly` (`main_test.go:25`) is the guard that stops `--appearance` returning and pins the default to the constant.
  - Both branches from one flag → `TestResolveProgram_ThemeDrivesBothBranches` (`main_test.go:433`).
  - Guards → `TestPortalBinaryDoesNotImportCapture` + the vacuity check `TestCaptureToolDoesImportCapture` (`import_guard_test.go`); `internal/capture`'s no-config-access invariant is covered behaviourally by `TestPanelFixture_NoConfigAccess` (`internal/capture/theme_panel_fixture_render_test.go:70`), which stages a decoy drop-in under a poisoned `XDG_CONFIG_HOME` / `PORTAL_THEMES_DIR` / `PORTAL_PREFS_FILE` and asserts it never appears and nothing is written.
- Notes: tests read behaviour, not internals, except for the small set of deliberate AST/source guards this repo uses as a convention (flag registration, const derivation, program wiring) — each of those pins something with no runtime observation point. Minor redundancy in `swatch_test.go` (see notes below). No excessive mocking: the only fake is the one-method `rejectingLoader`, and it exists solely to reach an otherwise-unreachable branch.

CODE QUALITY:
- Project conventions: Followed. Small injected seam (`themeLoader`, one method) per the DI convention; no `t.Parallel()`; `log.Discard` routing matches §12.3 and CLAUDE.md's closed-taxonomy rule; the harness stays free of config discovery; `theme.Loader` is constructed only through `NewSilentLoader`, satisfying `TestLoader_HasNoProductionCompositeLiteral`.
- SOLID principles: Good. `resolveTheme` → `resolveBuiltinTheme` / `resolvePathTheme` / `warnAboutFilename` split one responsibility per function; `LoadPath` sits beside `LoadFile` and `LoadBuiltin` and shares the single `resultFromBytes` content tail, so the content rungs cannot diverge between entry points.
- Complexity: Low. Longest function is 18 lines; the branchiest construct is a two-arm `switch` with no nesting.
- Modern idioms: Yes — `strings.CutSuffix`-era stdlib, `slices.Contains`, `os.LookupEnv`, `%w` error wrapping, `io.Writer` injection for the warning sink.
- Readability: Good. Every non-obvious decision carries a short rationale comment (why the separator half of the discriminator exists, why the warning fires only after a successful load, why the interface exists, why the palette is handed to `Deps` rather than assigned).
- Comment accuracy: Verified line by line against the code in `cmd/capturetool/main.go` and `internal/theme/load.go` — no false claims, no restated code, no spec-section/task/phase citations (phases 11-3 and 12-7 stripped those).
- Issues: none.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] /Users/leeovery/Code/portal/cmd/capturetool/swatch_test.go:44-52 — delete the "an unknown theme is an error" subtest; `TestResolveTheme_NoFallbackOnFailure` (main_test.go:252-273) already drives `resolveProgram(capture.ContrastValidationFixture, "not-a-theme")` and asserts both the error and the nil model, across three failure inputs rather than one.
- [quickfix] /Users/leeovery/Code/portal/cmd/capturetool/swatch_test.go:55-63 — delete `TestResolveProgramSessionsFixture`; its "returns a non-nil model" claim is strictly subsumed by `TestResolveProgram_ThemeDrivesBothBranches` (main_test.go:441-447), which builds the same branch and additionally asserts the canvas.
- [idea] /Users/leeovery/Code/portal/cmd/capturetool/main.go:141 — decide whether the path branch should expand a leading `~` (e.g. via `resolver.ExpandTilde`) before `theme.LoadPath`: the documented invocation relies on the shell expanding it, so a quoted argument or a tape line fails as `unreadable: open ~/themes/x.theme: no such file or directory`. The error names the right class of problem, so this is a convenience call, not a correctness one — and it would add an `internal/resolver` import to a binary deliberately kept thin.
