TASK: theming-system-14-1 — Single-Source The 19-Token Synthetic Probe Palette Into internal/themetest

ACCEPTANCE CRITERIA:
1. Exactly one synthetic-palette builder exists in the repo; neither `internal/capture` nor `internal/tui` declares a 19-field `theme.Theme` probe literal.
2. The builder fails loudly (naming the empty token) when a token is added to `theme.Theme` and not to the builder.
3. The builder fails loudly on `red < 0x64` and on a same-red pair.
4. The palettes are value-identical to the ones the two guards use today; every existing assertion in both suites is unchanged and passes.
5. `go test ./internal/capture ./internal/tui ./internal/themetest` passes; no test changes lane.

STATUS: complete

SPEC CONTEXT:
Spec §13.4 (specification.md:1647-1685) defines the swap-and-diff completeness guard: render every fixture under theme A, swap live through the production restyle path, re-render, and scan for surviving A values. It mandates "two synthetic themes constructed inside the test, all 38 values deliberately unique" and gives the rationale this task consolidates — a shipped pair fails two ways (a coincidentally-shared hex fails permanently for a non-bug; a token equal on both sides renders identically and the guard passes whether or not the site updated). Spec:191 pins the enabling property: `Theme` is an ordinary struct, constructible in a test without the loader, "which is what the swap-and-diff guard's synthetic themes need". Moving construction into `internal/themetest` (a test-only package that imports only `theme`/`testing`/stdlib) preserves the substance of "constructed inside the test" — no loader, no file, no dependency on the shipped palettes — so this is not spec drift.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - `internal/themetest/synthetic.go:27` `SyntheticPalette(t, red)` — the single builder; `:74` `SyntheticPair(t, redA, redB)`.
  - `internal/themetest/theme_file.go:1-3` — package doc extended to cover the new half.
  - `internal/capture/theme_swap_guard_test.go:28-31` — `syntheticPalettes(t)` now delegates to `themetest.SyntheticPair`; `syntheticTheme` / the 19-field literal / the `syntheticGreenBase`,`syntheticBlueBase` consts deleted (commit f0f1d37b).
  - `internal/tui/restyle_repoint_test.go:15-28` — `syntheticProbePalette` deleted; `probeThemeBefore/After(t)` delegate to `themetest.SyntheticPalette`.
  - Per-guard reds kept at their call sites as required: `syntheticRedA = 0x6E` / `syntheticRedB = 0xD2` (capture:24-25), `probeRedBefore = 0xAA` / `probeRedAfter = 0xBB` (tui:16-17).
- Notes:
  - Value identity holds. The moved code is byte-equivalent in behaviour: same `v(i)` closure, same `#%02X%02X%02X` format, same `0x80`/`0xC8` bases, same indices 1..19 in the same field order (verified against the deleted bodies in `git show f0f1d37b`). The signature widened `red int` → `red uint8`, which renders identically under `%02X` for every legal input.
  - Completeness assertion is real. `theme.Theme.All()` (internal/theme/theme.go:85) takes each token's `Name` from the canonical `fields()` table and its `Value` from the struct, so a 20th token added to the table but not to the builder enumerates with an empty `Value` and the loop at synthetic.go:63-67 fatals naming it — criterion 2 met, and it fires for both guards at once, which is the task's whole point.
  - Criterion 1, literal reading: one 19-field `theme.Theme` literal still exists inside `internal/capture` — `swatchTestPalette` (internal/capture/swatch_test.go:13-38). It is **not** a swap probe: it predates this plan (created under `spectrum-tui-design`), takes a caller-supplied canvas plus a `#%s%04d` value format, and serves the contrast-swatch render tests. It was deliberately outside this task's Do list (which named exactly three functions to delete, all three of which are gone). I judge the criterion met in substance — the two halves of the swap mechanism are single-sourced — and record the residual as a non-blocking note, since it carries the same silent-zero-fill exposure the task set out to remove.
  - No cycle introduced (Do item 8): `themetest` imports only `theme`, `testing`, `fmt`, `os`, `path/filepath`, `slices`, `strings`; it does not import `tui` or `capture`, so the `package tui` (internal) test file importing it is safe. No build tags anywhere in `themetest`, so nothing changed lane (criterion 5).
  - Every call site was re-pointed: 13 `syntheticPalettes(t)` calls across `theme_swap_guard_test.go` and `theme_panel_fixture_render_test.go`, and 10 `probeThemeBefore/After(t)` calls across `restyle_repoint_test.go`, `apply_theme_test.go`, `theme_panel_arrow_test.go`. No assertion text changed (criterion 4) — the diff is purely the `t` threading.

TESTS:
- Status: Adequate
- Coverage:
  - `internal/themetest/synthetic_test.go:11` — every token filled, in canonical `TokenNames()` order, at the exact expected value, with an in-palette duplicate check. Covers the task's "all 19 tokens a distinct non-empty value in canonical order" test.
  - `:37` — the three-decimal-digit channel property across `0x64` (the floor), `0xAA`, `0xFF`; this is the property the tui guard's substring scanning depends on and it was previously only asserted for one fixed pair inside `internal/capture`.
  - `:53` — `SyntheticPair` disjointness.
  - Both guard suites carry their assertions unchanged, so the builder is additionally exercised by every swap/restyle assertion in `internal/capture` and `internal/tui`.
- Notes:
  - The two fatal paths (`red < 0x64` at synthetic.go:30, equal reds at :77) have no permanent test — the helpers take a concrete `*testing.T`, so a fatal cannot be captured. The plan scoped these as one-off manual checks ("temporarily set both guard pairs to the same red … revert"), so this matches the plan rather than departing from it, but acceptance criterion 3 rests on a check nothing re-runs. Raised as an idea, not a blocker.
  - Mild redundancy: `TestSyntheticThemes_AllValuesUnique` (capture:162-193) now re-asserts cross-pair uniqueness and the three-digit range that `synthetic_test.go` owns. Retaining it was mandated by Do item 9 ("change no assertion in either guard"), and its `38` count pin is directly spec-anchored (§13.4), so this is a follow-up rather than a defect.

CODE QUALITY:
- Project conventions: Followed. `themetest` is documented in CLAUDE.md as test-only with `*testing.T`-first helpers; `SyntheticPalette`/`SyntheticPair` match that shape and both call `t.Helper()`. External `themetest_test` package for the new tests, consistent with `builtin_test.go` / `deny_test.go` / `theme_file_test.go`. No `t.Parallel()`. Unit lane preserved.
- SOLID principles: Good. One builder with one responsibility; the pair helper composes it rather than duplicating the literal. The per-guard reds stay at their call sites, so the shared helper carries no knowledge of either guard.
- Complexity: Low. One closure, one struct literal, two guard clauses, one validation loop.
- Modern idioms: Yes. Named `syntheticGreenBase`/`syntheticBlueBase`/`syntheticRedFloor` consts replace the magic numbers; `%#x` in the fatal messages prints the offending byte in the same base the call sites are written in.
- Readability: Good. The merged doc block states the two reasons a shipped pair is unusable and the fixed-width-SGR-core reason in four sentences, where the two originals took ~25 lines between them.
- Comment accuracy: Verified clean. Every claim holds against the code — per-token ramps stay unique within a palette and disjoint across differing reds; `0x64` is exactly the lowest three-decimal-digit red; the green/blue ramps stay in 129–147 / 201–219 for 19 tokens. All workflow vocabulary is gone: the deleted tui comment's `§13.4` / `§9.2` citations were not carried over, and no task id or phase number appears in the new file (Do item 5 satisfied).
- Issues: None blocking. One unreachable assertion (see notes).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/restyle_repoint_test.go:20-28 — `probeThemeBefore`/`probeThemeAfter` call `SyntheticPalette` twice with two independent consts, so they bypass `SyntheticPair`'s equal-reds fatal. If `probeRedBefore` and `probeRedAfter` ever converge, every "no stale value survives" assertion in `restyle_repoint_test.go`, `apply_theme_test.go` and `theme_panel_arrow_test.go` (10 call sites) passes vacuously — the exact failure `SyntheticPair` exists to prevent, on the one side of the mechanism that does not use it. Derive both from a single `themetest.SyntheticPair(t, probeRedBefore, probeRedAfter)`.
- [quickfix] internal/themetest/synthetic.go:60-62 — the `len(tokens) != len(theme.TokenNames())` guard can never fire: `All()` and `TokenNames()` both enumerate the same fixed `fields()` slice (internal/theme/theme.go:57-79), which is value-independent. Delete the three lines; the empty-value loop at :63-67 is the assertion that actually catches a token added to the vocabulary but not to the builder.
- [quickfix] internal/capture/swatch_test.go:13-38 — a third hand-written 19-field `theme.Theme` literal survives in `internal/capture`, with the same silent failure mode the task removed elsewhere (a 20th token compiles as a zero-valued field). Rebuild it on the shared builder — `p := themetest.SyntheticPalette(t, red); p.Canvas = theme.Token{Value: canvas}` — so it inherits the completeness assertion; its assertions read tokens off the injected palette rather than hardcoded hexes (only the canvas argument is value-significant), so the substitution is contained.
- [quickfix] internal/capture/theme_swap_guard_test.go:162-193 — `TestSyntheticThemes_AllValuesUnique`'s two sub-assertions (cross-pair duplicates, three-digit channels) are now owned by `internal/themetest/synthetic_test.go:37,53`. Narrow it to the spec-anchored `38` count pin and name `themetest` as the owner of the rest. Out of scope in-task (Do item 9 forbade touching guard assertions) — follow-up only.
- [idea] internal/themetest/synthetic.go:30,77 — the red-floor and equal-reds fatals are structurally untestable while the helpers take a concrete `*testing.T`. Decide whether to widen `themetest`'s signatures to `testing.TB` and add a recorder fake so acceptance criterion 3 has permanent coverage; the trade is the `*testing.T`-first parameter that structurally keeps these helpers out of production code (the convention CLAUDE.md records for `portaltest`).
