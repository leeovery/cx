TASK: theming-system-12-4 — Single-Source The SGR Parameter-Run Probe In internal/tui And internal/capture (tick-b13755, Phase 12 / Analysis Cycle 2 remediation)

ACCEPTANCE CRITERIA:
1. `package tui` contains exactly one implementation of the probe-and-slice derivation.
2. `package capture_test` contains exactly one, and `package capture` at most one.
3. No test file inlines the `'['`/`'m'` index-and-slice sequence.
4. Every former helper name that survives is a one- or two-line wrapper over the shared derivation.

STATUS: complete

SPEC CONTEXT:
No specification clause governs this task directly — Phase 12 is an analysis-remediation cycle and this is a test-scaffolding duplication fix. The derivation it single-sources is what nearly every theming assertion in `internal/tui` and `internal/capture` uses to prove the spec's token contract (a rendered frame carries the `38;2;…` / `48;2;…` run a role token resolves to). The correctness property that matters is therefore *assertion-neutrality*: the refactor must move where the derivation lives without changing a single byte any assertion compares. It does — see IMPLEMENTATION.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - `internal/tui/theme_testing_test.go:279-301` — the single `package tui` derivation `sgrParams(t, style)` plus the `tokenFgSeq` / `tokenBgSeq` one-line wrappers (moved out of `header_test.go`).
  - `internal/tui/canvas_paint_test.go:12-15` — `canvasSeq` is now `"\x1b[" + sgrParams(...) + "m"`.
  - `internal/tui/session_row_anatomy_test.go:32-35` — `selectionBgParams` is now a one-line `sgrParams` call.
  - `internal/tui/active_theme_test.go:29,46` — the inline copy inside `assertActiveTheme` and the deleted `sgrForegroundCore` both re-expressed over `sgrParams`.
  - `internal/tui/edit_modal_test.go:671,675` — `bgSeq` deleted, callers re-pointed at `tokenBgSeq`; `internal/tui/sessions_flash_reskin_test.go:75,319` likewise.
  - `internal/tui/canvas_cell_background_test.go:99` — the pre-existing, unrelated `sgrParams(seq string) []string` renamed `splitSGRParams` to free the name; its one call site (`:87`) updated.
  - `internal/capture/theme_swap_guard_test.go:53-62` (`sgrParameterRun`, `package capture_test`) and `internal/capture/theme_panel_fixture_test.go:579-588` (`backgroundSGR`, `package capture`) — the two legitimate in-directory copies; `internal/capture/swap_harness_test.go:123-126` (`bgSeq`) is a one-line wrapper over `sgrParameterRun`.
  - Commit `ea04d15f`.
- Notes:
  - AC1 holds: the only probe-and-slice body in `package tui` is `theme_testing_test.go:282`. The other `strings.IndexByte(…, 'm')` sites in the package (`theme_row_test.go:61,554,574`) parse *rendered output* for the text a run painted — a different operation, correctly left alone.
  - AC2 holds: `package capture_test` has exactly one (`sgrParameterRun`), `package capture` exactly one (`backgroundSGR`). Step 4's capture work was already satisfied before this task — task 11-13 had collapsed `swap_harness_test.go`'s `bgSeq` into a wrapper (verified at `ea04d15f^`), so the task description's capture inventory was stale and the commit correctly touching only `internal/tui` is not an omission.
  - AC4 holds: every surviving name (`tokenFgSeq`, `tokenBgSeq`, `canvasSeq`, `selectionBgParams`, capture's `bgSeq`) is a single-line wrapper; `sgrForegroundCore` and tui's `bgSeq` are gone (no dangling references anywhere, and the stale `header_test.go's tokenFgSeq` comment reference in `internal/capture/swatch_test.go:58` was updated in the same commit).
  - AC3 holds with one exception the task itself sanctioned: `internal/tui/model_test.go:21-36` (`editFieldFocused`) still inlines the derivation. It is `package tui_test` and genuinely cannot reach `package tui`'s unexported `sgrParams`, so leaving it was the instructed outcome ("leave it as the one unavoidable copy rather than exporting test scaffolding"). The instruction to *note that explicitly* was not carried out — there is no comment at the site and no note in the commit message. Documentation gap only; recorded below.
  - Assertion-neutrality verified by reading each replaced body against its replacement: `canvasSeq`/`assertActiveTheme` previously took `probe[:IndexByte(probe,' ')]` of a `Render(" ")` probe, which for a background-only style is exactly `\x1b[` + params + `m`; `selectionBgParams` previously Trim-stripped the same slice to the params. Old and new produce identical bytes (SGR params are digits and semicolons, so the first `m` can never fall inside them). `bgSeq`→`tokenBgSeq` and `sgrForegroundCore`→`sgrParams` were byte-identical bodies. No assertion changed.

TESTS:
- Status: Adequate
- Coverage: Correctly no new tests — this is a test-scaffolding consolidation, and the shared derivation is exercised by several hundred existing call sites across `internal/tui` (header, footer, modals, theme panel, flash, canvas, row anatomy) and `internal/capture`. Adding a test *for the helper* would be over-testing: any breakage in it fails those call sites immediately and loudly.
- Notes:
  - The consolidated `sgrParams` keeps a `t.Fatalf` on an underivable probe, so the "would fail if it broke" property survives the move; the token-naming that the task's manual break-a-token check relies on lives in the call-site `t.Errorf` messages, none of which the commit touched.
  - The one behavioural regression risk in the move — the merged failure message replacing the old distinct "foreground SGR core" / "background SGR core" wordings — is immaterial: the message only fires when lipgloss emits no SGR at all, and it quotes the probe.
  - The manual "deliberately break one token" verification cannot be evidenced from the tree (by design — it must not be committed); nothing in the diff contradicts it.

CODE QUALITY:
- Project conventions: Followed. Test-only scaffolding stays in `_test.go` files, nothing exported from production code to serve tests (the explicit constraint in steps 3-4), no `t.Parallel()` introduced, helpers take `*testing.T` first and call `t.Helper()`.
- SOLID principles: Good. One derivation, one reason to change; the wrappers name intent (`tokenFgSeq` / `canvasSeq` / `selectionBgParams`) without re-implementing it.
- Complexity: Low. Every surviving wrapper is a single expression.
- Modern idioms: Yes. No opportunities missed.
- Readability: Good. `sgrParams`'s doc comment states the non-obvious reason the bare run (not the whole `ESC[…m`) is the wanted shape, and it holds true against the code. The `sgrParams` → `splitSGRParams` rename is an accurate distinction (probe a style vs. parse a sequence), though the two names now sit close together in one package.
- Issues:
  - The task's stated Outcome ("one obvious return shape") is only partly reached. Two shapes remain — bare run and full `ESC[…m` — which is deliberate, but only the bare-run shape got an owner. The full-sequence shape is built by `canvasSeq` and re-built inline at `active_theme_test.go:29`, and two sites round-trip it *back* to params with `TrimPrefix`/`TrimSuffix` (`canvas_cell_background_test.go:68`, `theme_panel_message_test.go:322`) rather than calling `tokenBgSeq`. That is the residue of the exact shape-confusion the task targeted, though at a much smaller scale than before.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/model_test.go:21 — add the note step 3 required, above `editFieldFocused`: "// The one inlined copy of the SGR probe: this is package tui_test, which cannot reach package tui's unexported sgrParams, and test scaffolding is not exported to make it reachable."
- [bug] internal/tui/model_test.go:26-28 — `editFieldFocused` returns `false` when the probe carries no SGR sequence, so a colourless render (or a profile downgrade) reports "field not focused" instead of "probe broken", sending the reader after the wrong defect. Replace the `return false` with `t.Fatalf("could not derive an SGR parameter run from %q", probe)`, matching the shared helper.
- [quickfix] internal/tui/canvas_cell_background_test.go:65-73 — `wantCanvasBgParams` derives the full sequence via `canvasSeq` then strips it back to params; replace the body with `return tokenBgSeq(t, th.Canvas)` (byte-identical result, one shape conversion removed).
- [quickfix] internal/tui/theme_panel_message_test.go:322 — same round-trip, recomputed per cell inside the loop; hoist `wantCanvas := tokenBgSeq(t, th.Canvas)` above the `for` and compare `cell.params != wantCanvas`.
- [quickfix] internal/tui/theme_testing_test.go:291 — add the second thin wrapper the task's Solution called for, `sgrSequence(t, style) string { return "\x1b[" + sgrParams(t, style) + "m" }`, and express both `canvas_paint_test.go:14` (`canvasSeq`) and `active_theme_test.go:29` (`assertActiveTheme`) over it, so the full-sequence shape has one owner instead of one wrapper plus one inline concatenation.
- [idea] internal/themetest — decide whether to host a single exported `SGRParams(t *testing.T, style lipgloss.Style) string` there, so `package tui`, `package tui_test`, `package capture` and `package capture_test` all share one derivation: it would remove the last inline copy (`model_test.go`) and the capture two-copy split without exporting anything from production code (`themetest` is test-only). The task deliberately scoped to per-package single-sourcing, so this is a decision to widen `themetest`'s remit (currently theme-file/palette authoring, no lipgloss dependency), not a defect in the delivered work.
