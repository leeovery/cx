TASK: theming-system-4-5 — RestoreTerminalBackground's Two Divergence Cases Through The Real Swap Path

ACCEPTANCE CRITERIA:
1. Both divergence cases covered by named tests: theme committed mid-session, and quit with an uncommitted preview active.
2. Every swap goes through task 4-2's `ApplyTheme`; no test assigns `activeTheme` or `startupCanvasHex` directly.
3. `startupCanvasHex` captured once at gate resolution, unchanged after one swap and after a preview run of three.
4. Startup `tokyo-night` + active `tokyo-night-day`: original `#0b0c14` → no write; original `#E1E2E7` → set-back written.
5. Startup `tokyo-night` + active `nord`: original `#2E3440` → set-back written.
6. Echo guard still skips for `#0b0c14`, `#0B0C14`, `#0b0c14ff`, `0b0c14` after any number of swaps.
7. Non-hex (`rgb:`) reply still emits the set-back; empty capture still writes nothing.
8. `RestoreTerminalBackground` still reads only the retained startup hex — task 3-3's source guard passes unchanged.
9. No panel, no commit path, no prefs write introduced — the preview case is modelled as a swap with no commit.
10. `go test ./...` green; unit lane; no `t.Parallel()`.

STATUS: complete

SPEC CONTEXT:
§11.4 (specification.md:1389-1403) requires the exit-time canvas restore to anchor its comparison to a *retained startup canvas hex* rather than the active theme's, and mandates its own named verification because §13.4's swap-and-diff guard "scans rendered fixture output, so it structurally cannot cover an OSC 11 write that happens after the last render" (line 1682). The two named divergences are a theme committed mid-session and a quit with an uncommitted preview active (`Ctrl-C` with the panel open, §9.7/§9.8 line 1187) — the latter being "the only path on which a colour the user never chose can be left stuck in their terminal after Portal exits". §9.10 (line 1208) explicitly states the anchor test needs no `NO_COLOR` case ("the value is defined and unused"). §4.3 (line 285) pins the uppercase hex canonicalisation the assertions rest on. §11.3 (line 1387) confirms a later swap issues no new OSC 11 query, so the guard only ever compares against the startup-window canvas.

IMPLEMENTATION:
- Status: Implemented (test-only task; no production change was in scope).
- Location:
  - `internal/tui/restore_divergence_test.go:1-199` — the new named verification (landed by 77c0d20b; later re-shaped by the package-wide comment strip e3fa1503 and the `themetest.Builtin` single-sourcing 4d241e56).
  - `internal/tui/restore.go:20-32` — the anchored comparison under test (`sameHexColour(original, m.themeState.startupCanvasHex)`).
  - `internal/tui/model.go:897-900` (`ApplyTheme`, the production swap entry point the test drives) and `model.go:858-875` (`syncResolvedMode` → `captureStartupCanvasHex`, the single write site).
  - `internal/tui/theme_state.go:59-62` — `startupCanvasHex` with the "frozen when the gate resolves" contract.
- Notes:
  - AC 8's wording ("`m.startupCanvasHex`") was superseded by task 11-15, which moved the field under `Model.themeState`. The source guard `internal/tui/restore_source_guard_test.go:26-32` was re-pointed to `themeState.startupCanvasHex` and still enforces the anchor (plus a vacuity check at :89-92 that fails if the helper reads no anchor at all). Not drift.
  - The task's "record the NO_COLOR omission in a comment" instruction produced a file preamble in the original commit; the deliberate package-wide comment strip (e3fa1503, phase 16/17) removed it. The carve-out itself remains pinned by `internal/tui/restore_test.go:91-109` (`TestNoColor_HexCapturedAndSetBackIsANoOp`) and by `restore.go:21-23`'s `m.colourless` early return, so nothing is unverified. Superseded by a later remediation, not drift.
  - AC 9 holds: the file touches no panel, no persister and no prefs — "preview" is exactly N `ApplyTheme` calls with no commit.

TESTS:
- Status: Adequate.
- Coverage (mapping to acceptance criteria):
  - AC 1/4 — `TestRestoreBackground_CommittedThemeDivergence` (:67-90): startup dark, `ApplyTheme(light)`, then a guard-fatal that the active canvas really moved (:72-74, so the case cannot silently degrade into restore_test.go's un-swapped pair), then the skip (`#0b0c14`) and emit (`#E1E2E7`) subtests plus a startup-hex-unmoved subtest.
  - AC 1/3 — `TestRestoreBackground_UncommittedPreviewDivergence` (:92-115): three `ApplyTheme` calls with no commit, ending on nord; asserts the startup hex after *each* of the three; fatals unless the run ends on `#2E3440`; then asserts the previewed canvas the user never chose IS set back and the startup echo is still skipped.
  - AC 5 (the naive-implementation trap) — `TestRestoreBackground_ActiveCanvasEqualsOriginalStillSetsBack` (:117-137): the genuine original is captured through the production `Update`/`tea.BackgroundColorMsg` path (not by field assignment), then `ApplyTheme` lands on the theme whose canvas equals it. Two setup fatals (:130-135) prove the coincidence exists and that the case is not inverted. A comparison re-derived from the active theme fails here; this is the single discriminator the task asked for, and it is its own named test rather than a table row, as required.
  - AC 3 — `TestRestoreBackground_StartupHexSurvivesSwaps` (:139-153): five swaps, asserting both the frozen hex and (behaviourally) the echo skip after each.
  - AC 6 — `TestRestoreBackground_EchoGuardShapesAfterSwap` (:155-175): named table over the four shapes, run after two swaps.
  - AC 7 — `TestRestoreBackground_NonHexReplyAfterSwapStillSetsBack` (:177-185) and `TestRestoreBackground_EmptyCaptureAfterSwapWritesNothing` (:187-199, with a fatal proving nothing had been captured and a fatal proving the swap happened). The pre-existing `TestRestoreTerminalBackground_EmptyWritesNothing` is kept at `internal/tui/background_restore_test.go:158`.
  - AC 2 — verified by reading: every theme change in the file is `m.ApplyTheme(...)`; the only field the test writes is `originalBg` (the captured terminal reply, not theme state). `startupCanvasHex` is only ever read, and `startupModel` (:27-34) fatals unless construction produced it — so a regression that started writing it on swap fails here.
  - AC 8 — `TestRestorePath_ReadsNoTheme` (`restore_source_guard_test.go:39-109`) still parses `restore.go`, forbids any `theme` import/selector, whitelists exactly `{OriginalBackground, colourless, themeState.startupCanvasHex}`, and additionally sweeps the whole tree for the retired `canvasHexFor`.
  - AC 10 — no `t.Parallel()`; unit lane (no build tag, no daemon, no binary); constants match the embedded files (`builtins/nord.theme:74` `#2E3440`, `tokyo-night.theme:24` `#0b0c14` → uppercase-canonicalised, pinned by `active_theme_test.go:15-21`).
  - The gate-at-construction premise holds: `newSwapFrameModel` (`apply_theme_test.go:266-295`) builds with `theme.ConstantNomination`, `newNominationGate` returns `pinned` (`appearance_gate.go:31-36`), so `syncResolvedMode` captures the hex once at `Build` and a later `BackgroundColorMsg` cannot re-resolve it (`model.go:1485-1497`, `appearance_gate.go:64-71`). The complementary adaptive-reply capture is covered by task 3-3's `TestRestoreTerminalBackground_AnchoredToStartupHex` (`restore_test.go:42-77`).
  - Every assertion would fail under the naive (active-theme-anchored) implementation in at least one direction, and each test carries a positive-control/setup fatal so it cannot pass vacuously.
- Notes:
  - Deliberate, task-mandated overlap with the un-swapped equivalents (`restore_test.go`'s echo-guard table; `apply_theme_test.go:297-318`'s 51-swap hex-freeze). The divergence variants add the post-swap behavioural half rather than repeating the same assertion, so this is not over-testing.
  - `assertSetBack` derives its expectation from the captured original rather than restating a literal, which keeps the contract ("set back what the terminal reported") as the assertion.
  - The intent survived into the phases it was written for: `theme_panel_commit_load_test.go:771-793` reuses `assertSkipped`/`assertSetBack`/`withCapturedOriginal`/`nordCanvas` to pin the same anchor across the *real* panel commit path — the task's stated purpose ("drives the same path Phases 8–9 will drive for real") is realised.

CODE QUALITY:
- Project conventions: Followed. In-package `tui` test (needs the unexported gate and `themeState`), file named off `restore.go` with a concern suffix, no `t.Parallel()` (CLAUDE.md prohibition), built-ins loaded via `themetest.Builtin` rather than a hand-rolled loader call, no production code imports test helpers.
- SOLID principles: Good (test-only; helpers are single-purpose and the two silent-return reasons are distinguished by the `why` parameter on `assertNothingWritten`, so a no-capture failure can't report the echo-guard contract).
- Complexity: Low. Longest test is 24 lines; no nesting beyond one `t.Run`.
- Modern idioms: Yes — `for _, th := range []theme.Theme{…}` with index-named fatals, anonymous-struct table with named subtests, `strings.EqualFold` where the reply's case is production-determined.
- Readability: Good. Failure messages state the contract and the three relevant values (captured original, startup canvas, active canvas), so a failure is diagnosable without reading the test.
- Comment accuracy: N/A — the file carries no comments after the package-wide strip; nothing stale, no process-artifact references (the spec-citation strip f1e2e95f also cleared them from string literals).
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `internal/tui/restore_divergence_test.go:36` — add one line above `withCapturedOriginal`: `// Set directly: Update renders every reply as lower-case #rrggbb, so three of the four echo shapes below (upper case, trailing alpha, no leading '#') cannot be produced through it.` The original commit carried this rationale and the package-wide comment strip removed it; without it, a maintainer "fixing" the helper to route through `Update` would silently delete three of the four shapes AC 6 exists to pin.
- [quickfix] `internal/tui/restore_divergence_test.go:13-65` — move the now-shared exit-path helpers (`nordSlug`, `nordCanvas`, `testNordTheme`, `withCapturedOriginal`, `assertNothingWritten`, `assertSkipped`, `assertSetBack`) into the package's shared theme test-helper file `internal/tui/theme_testing_test.go`, where `testDarkTheme`/`testLightTheme`/`themeLabel` already live. `theme_panel_commit_load_test.go:777-791` depends on four of them, so they are no longer local to this concern file.
