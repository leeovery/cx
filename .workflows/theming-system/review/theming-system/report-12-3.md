TASK: theming-system-12-3 (tick-838f51) — Single-Source The Built-In-Loading Test Helper Into internal/themetest

ACCEPTANCE CRITERIA:
1. `internal/themetest.Builtin` exists with two distinct Fatal messages, one per failure class.
2. No package declares its own built-in-loading test helper; all seven former sites call the shared one.
3. No production file imports `internal/themetest`.
4. `go test ./...` passes with no behavioural change.

STATUS: complete

SPEC CONTEXT:
§7.6 "The build-time guarantee" (specification.md:661-676) is the reason the two failure classes must stay separate. The guarantee has two load-bearing halves: (1) every embedded built-in parses and validates against the §6.1 rule, and (2) every fallback slug and the shipped default pair *resolve within that set*. `!found` is a violation of half 2 ("the slug names no built-in" — a renamed file or a typo'd constant), `rejection != nil` is a violation of half 1 ("the shipped file is broken"). §7.6 explicitly warns that a rename can leave every embedded theme validating while every fallback path becomes unresolvable — which is exactly the discrimination a merged failure message destroys. The spec also states the loader returns an ordinary error rather than panicking on an embedded parse failure (specification.md:674), which is why the shared helper Fatals rather than falling back: there is no degraded palette a test could carry on with.

IMPLEMENTATION:
- Status: Implemented (with one intentional later supersession)
- Location:
  - `internal/themetest/builtin.go:13-24` — `Builtin(t, slug) theme.Theme`, distinct Fatalf per failure class.
  - `internal/themetest/builtin.go:26-34` — `DefaultDark` / `DefaultLight` wrappers.
  - `internal/themetest/theme_file.go:1-7` — package doc updated to name the built-in-loading half.
  - Commit `4d241e56` deleted all seven local helpers and re-pointed callers:
    - `internal/tui/theme_testing_test.go` — `testBuiltinTheme` deleted; `testDarkTheme`/`testLightTheme` (now :305-313) are one-line wrappers over `themetest.DefaultDark/DefaultLight`, as the plan's Do-3 prescribed.
    - `internal/tui/model_test.go` — `darkBuiltinTheme` deleted, call site now `themetest.DefaultDark(t)` (:28).
    - `internal/capture/theme_panel_fixture_test.go` — `builtinTheme` deleted, ~10 sites re-pointed.
    - `internal/capture/theme_panel_fixture_render_test.go` — `builtinPalette` deleted, sites re-pointed.
    - `internal/capture/swap_harness_test.go:71-74, 118-121` — `darkBuiltinTheme`/`lightBuiltinTheme` retained as one-line wrappers over the shared helper (explicitly sanctioned by Do-3).
    - `cmd/open_theme_nomination_test.go` — `builtinThemeForTest` deleted.
    - `cmd/capturetool/main_test.go` — `builtinForTest` deleted (one of the two drifted merged-message copies).
    - `internal/theme/resolution_test.go` — `builtinTheme` deleted, ~12 sites re-pointed.
- Criterion 1: MET. Two Fatalf calls, one per class, with the discriminating wording preserved ("not found in the embedded set" / "was rejected: <reason>"). Neither drifted copy's merged form was adopted.
- Criterion 2: MET. A repo-wide grep for `LoadBuiltin` outside `internal/theme` leaves only: `internal/themetest/builtin.go:16` (the single definition), `internal/tui/builtin_themes.go:14` (production), `internal/tui/theme_testing_test.go:370` (`builtinCanvasValue` — a deliberately *different* shape: no `*testing.T`, returns `""` rather than Fatalling, because it is evaluated inside an argument list; the divergence is documented in-source at :367-369 and is not a re-mint of the deleted helper), and two inline call sites in `cmd/capturetool/main_test.go` (:88, :324) that are not declared helpers but are residual — see NON-BLOCKING NOTES. A sweep of every `func …(t *testing.T…) theme.Theme` in the tree confirms all remaining theme-producing test helpers (`testNordTheme`, `arrowPalette`, `probeThemeBefore/After`, `themeForAppearance`) route through `themetest.Builtin` / `themetest.SyntheticPalette` / the wrappers.
- Criterion 3: MET. Grep for `themetest` across non-`_test.go` files returns only the package's own source files. The `*testing.T`-first signature on `Builtin`/`DefaultDark`/`DefaultLight` gives the same structural enforcement CLAUDE.md relies on for the other test-only packages.
- Criterion 5 (Do-5, import cycle): MET. `themetest` imports only `internal/theme` + stdlib. The two in-directory consumers are `package tui` (`theme_testing_test.go`) and `package capture` (`theme_panel_fixture_test.go`) — neither is imported by `themetest`, so no cycle. Every `internal/theme` consumer is `package theme_test` (verified across all 11 theme test files that import themetest).
- Criterion 4 (Do-4, loader consistency): MET as amended. The task shipped `theme.NewLoader(nil)`; task 14-12 (`42bb0e2e`, "make NewSilentLoader the only route to a silent theme loader") converted it to `theme.NewSilentLoader()`. That is a deliberate later-phase supersession, not drift. No test asserts on theme events emitted from these loads — `Builtin` returns only the palette and exposes no logger, and every event-asserting test in `internal/theme/events_test.go` / `silent_loader_test.go` builds its own loader and uses only `themetest.Write`/`Lines`/`DenyDir`.
- Notes: `internal/themetest/builtin.go` was further touched by two later comment-audit commits (`25626754`, `915e7fcb`) which compressed the doc comment and dropped the `DefaultDark`/`DefaultLight` doc comments. The surviving comment still states the load-bearing rationale ("The two failure classes must stay separately reported: a message that could mean either hides a broken shipped file behind a typo in the slug"), and the dropped ones would have restated one-line wrapper bodies — consistent with the project's audited comment standard. CLAUDE.md's `themetest` row already documents both halves accurately.

TESTS:
- Status: Adequate
- Coverage:
  - `internal/themetest/builtin_test.go:10-18` `TestBuiltin_ReturnsTheParsedPalette` — asserts `Builtin("tokyo-night").Canvas.Value == "#0B0C14"`. Exactly the plan's prescription: one distinguishing token, not a whole-struct literal. It is non-vacuous in two ways: the file on disk declares `canvas = #0b0c14` (lower case, `internal/theme/builtins/tokyo-night.theme:24`), so the assertion also pins the loader's canonical upper-casing rather than echoing the file; and a zero `Theme` (the shape a merged-and-swallowed failure would yield) fails it.
  - `internal/themetest/builtin_test.go:20-32` `TestDefaultDarkAndDefaultLight_ResolveTheShippedPair` — pins each wrapper to its slug (catches a copy-paste slot swap, which the distinctness check alone would not) and asserts the two palettes are distinguishable (catches a collapsed pair that would let a light/dark assertion pass on the wrong canvas). Three assertions, each catching a different failure; no redundancy.
- Notes: The failure-path branches (`!found`, `rejection != nil`) are not directly tested. That is correct rather than a gap — testing them would require either a fake loader (defeating the point of the helper being a thin pass-through to the production loader) or deliberately breaking an embedded file, and the branches are covered as *behaviour* by `internal/theme`'s own `broken_builtin_test.go` and the §7.6 build-time guarantee tests. The swap itself is verified by the 7 packages' existing suites compiling and passing against the new call sites — the plan's stated micro-acceptance.
- Not over-tested: two focused tests, no mocking, no setup.

CODE QUALITY:
- Project conventions: Followed. Test-only package with `*testing.T`-first exported signatures (CLAUDE.md's structural test-only enforcement); `t.Helper()` on all three functions so failures point at the caller; no `t.Parallel()`; leaf-safe imports (stdlib + `internal/theme` only, no `internal/log`); the hex literal in `builtin_test.go` is outside the `internal/tui`-scoped `colour_literal_guard_test.go`'s reach (`sourceguardtest.PackageGoFiles(".")`) and is a test expectation rather than a render-site colour, so it does not undercut the no-raw-hex rule.
- SOLID principles: Good. Single responsibility (load one built-in, fail loudly); `DefaultDark`/`DefaultLight` compose over `Builtin` rather than duplicating it, so the loader entry point remains one edit.
- Complexity: Low. Two guard clauses and a return.
- Modern idioms: Yes. Idiomatic `t.Helper()` + `t.Fatalf`; the three-value `(Result, *Rejection, bool)` return is destructured rather than smuggled through a sentinel.
- Readability: Good. The retained comment explains *why* the two Fatals cannot be merged — the one thing the code itself cannot say.
- Comment accuracy: Accurate. The `builtin.go` doc comment holds true against the code (two separately-reported classes), restates nothing, and carries no task/phase/spec-section references (the deleted copies' `§7.6` citations went with them).
- Security / performance: N/A — test-only, no I/O beyond the embedded FS.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/capturetool/main_test.go:88-91 — replace the inline `theme.NewSilentLoader().LoadBuiltin(theme.DefaultDarkSlug)` + merged `t.Fatalf("LoadBuiltin(%s) found=%v rejection=%v", …)` block with `want := themetest.Builtin(t, theme.DefaultDarkSlug)` and compare `got != want` (drop the `.Theme` deref at :92). This is the same merged-failure-class form the task deleted from `builtinForTest` in this very file, left behind at an inline site; the file already imports `themetest` (:313).
- [quickfix] cmd/capturetool/main_test.go:324 — replace `builtin, _, _ := theme.NewSilentLoader().LoadBuiltin("nord")` with `builtin := themetest.Builtin(t, "nord")` (and `got == builtin` at :325). Discarding both `found` and `rejection` makes the assertion vacuous if nord ever fails to load: `builtin.Theme` would be the zero palette, `got` is already asserted non-zero at :321, so the `got == builtin.Theme` check would pass without comparing anything.
