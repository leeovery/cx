TASK: theming-system-3-5 — Nord's Grouped Capture And The Outstanding `text.subtle` Visual Gate

ACCEPTANCE CRITERIA:
1. The capture is produced through `capturetool --theme nord` on a **grouped** fixture (task 3-4 must have landed).
2. The captured frame demonstrably contains a `text.subtle` locus (a group `··· N` count), proven by a render assertion rather than by eye alone.
3. The PNG's hash is verified to have changed on this run; a silent VHS non-write is retried, never reviewed.
4. The human gate was taken with the capture shown inline, the `capturetool --fixture … --theme nord` command given, and the Nord Paper frame available as reference.
5. The outcome is recorded in the task's commit message — pass or re-derive.
6. If the value moved: `nord.theme` carries the new hex with an updated `#` comment; value ≥ 3.00 and < 4.50 against `#2E3440`; Phase 2's floor suite passes over all three built-ins with nothing relaxed.
7. `text.muted` is untouched.
8. `nord`'s enrolment (dark) and the four light pins are unaffected.
9. The tape and PNG are committed as scaffolding with a note that they clear at sign-off in Phase 10; the Go fixture definitions stay untouched.

STATUS: complete

SPEC CONTEXT:
§7.4 closes the Nord port with one judgement open: `text.subtle` is an *invention* (Nord's greys are barrelled at the ends, so `#73819B` was interpolated on nord3's hue/saturation) measuring ~3.18 against Nord's own mid-dark canvas `#2E3440` — inside §13.5's 3.00–4.49 band with only ~0.18 of headroom on the tightest-headroom palette in the shipped set. §7.4 also fixes the derivation method: an invention's constraints are "landing in the right band and looking right", so it is settled at a visual gate, never by arithmetic (the `bg.attention` precedent). The token has no locus on a flat Sessions frame — it paints group `··· N` counts and the loading page's pending steps — so only a grouped capture can carry the gate, and that needed the render layer to take a `Theme` first (task 3-4's `--theme` flag). §13.3 makes hash-verify-and-retry a mandatory procedural mitigation because VHS fails silently on write and the image is the entire signal here. §13.2 makes the tape and its PNG scaffolding, committed while collaborated on and cleared at sign-off.

IMPLEMENTATION:
- Status: Implemented (with a later, intentional supersession of the scaffolding artefacts)
- Location:
  - `internal/capture/grouped_subtle_locus_test.go:65` — `TestGroupedRender_CarriesTextSubtleCountLocus`, the locus proof.
  - `internal/capture/fixtures.go:227` — `sessionsByProjectFixture` (5 projects + an Unknown catch-all ⇒ 6 group headings, one of them a count of 2), untouched by this task as required.
  - `internal/theme/builtins/nord.theme:34-46` — `text.subtle = #73819B` and its invention comment, unchanged (the gate passed).
  - `testdata/vhs/sessions-by-project-nord.tape` + `.png` at commit `fb2a79ce`; both deleted by `71e24eef` (task 10-10, the §13.2 sign-off clearance). `testdata/vhs/reference/sessions-nord-port.png` — the Paper frame committed by this task — is correctly retained under the `reference/` carve-out.
- Notes:
  - AC1: the committed tape drives `go run ./cmd/capturetool --fixture sessions-by-project --theme nord` on the grouped fixture — the theme is an explicit input, no config discovery (`cmd/capturetool/main.go:110` `resolveTheme`). ✅
  - AC2: the render assertion renders the *same* fixture under the *same* palette through the *same* `tui.Build` constructor, so the gate could not have been taken on a locus-free frame. It also carries a precondition loop (`grouped_subtle_locus_test.go:68`) that fails if any other Nord token shares `text.subtle`'s value, which is what makes "this run is text.subtle" decidable at all. ✅
  - AC3/AC4/AC5: recorded in the commit body of `fb2a79ce` — "Visual gate taken and PASSED… accepted text.subtle #73819B unchanged… Nothing re-derived", plus "VHS silently failed to write on 1 of 3 runs this session — the hash-verify-and-retry mitigation is not theoretical on this machine." The tape header itself carries the live-view command and the hash-verify mandate. The Paper reference frame landed in the same commit. Procedural criteria; the commit message is the only available evidence and it is explicit and non-boilerplate. ✅
  - AC6: N/A — the value did not move.
  - AC7/AC8: `nord.theme` is absent from `fb2a79ce`'s file list, so `text.muted`, the enrolment and the light pins are untouched by construction. ✅
  - AC9: the tape's own header states the scaffolding status and the Phase-10 clearance, and explicitly exempts the Go fixture definitions. The clearance then actually happened at `71e24eef`. Their absence today is the plan's intent, not drift. ✅
  - No production code changed — correctly so. This task's deliverable is a gate outcome plus the assertion that makes the gate honest.

TESTS:
- Status: Adequate
- Coverage:
  - `TestGroupedRender_CarriesTextSubtleCountLocus` (`internal/capture/grouped_subtle_locus_test.go:65`) reads every `··· N` run off the stripped frame and asserts each is painted as `text.subtle`-on-`canvas`, built independently from the two tokens rather than through the delegate's own style helper — so a count that stopped being `text.subtle`, or one painted over a foreign background, fails. It would also fail loudly (`t.Fatalf`) if the frame carried no count at all, which is the exact failure the task exists to prevent.
  - The expectation is derived from `lipgloss.NewStyle().Foreground(fg).Background(bg)`, byte-identical to production's `headerStyle` (`internal/tui/header.go:40`) as consumed at `internal/tui/session_item.go:143`. Dropping the trailing reset is correct: the outer canvas fill rewrites it.
  - `TestTextSubtleBand` (`internal/theme/contrast_test.go:96`) auto-enumerates the embedded set and asserts `≥ floorLargeUI` **and** `< floorNormal` per theme — the band, not a floor, and enrolment is itself guarded (`contrast_test.go:60-64`) against a vacuous pass.
  - `TestNordFile_CorrectionsAndInventionsCarryComments` (`internal/theme/builtins_nord_test.go:108`) pins `text.subtle`'s shipped `#73819B` and its `3.18` figure in the file comment, so a silent value move breaks the record test as well as the band test.
  - Lane: unit, untagged, no tmux/daemon/binary — correct per CLAUDE.md.
- Notes:
  - Mild overlap with the later swap-and-diff guard (`internal/capture/theme_swap_guard_test.go:524` pins `text.subtle`'s locus to `sessions-by-project`), but the mechanisms differ — synthetic-palette coverage scan vs. this task's nord-pinned, `··· N`-specific assertion — and only the latter proves the *gated* frame carried the token. Not redundant; no action.
  - `renderFixtureFrame` drives the fixture's real `Init → Update` flow rather than `Fixture.ModelAt`. That is the right choice, not duplication: `capturetool` runs a real `tea.Program` (`cmd/capturetool/main.go:56`), so the Init-driven path is the closer mirror of what was screenshotted.
  - The frame is rendered at a hardcoded 120×40 rather than through `fx.RenderSize`. Inert today (`sessions-by-project` declares no size and no capture keys) but see the non-blocking note.

CODE QUALITY:
- Project conventions: Followed. External test package, no `t.Parallel()`, no real tmux, helper loading via `themetest.Builtin` (the single-sourced accessor), no hex literals at call sites.
- SOLID principles: Good — helpers are single-purpose and the expectation is built independently of the code under test, so the assertion cannot move with a mutation of the pairing.
- Complexity: Low. `flattenCmd`'s recursion over `tea.BatchMsg` is the minimum needed to reach the leaf messages.
- Modern idioms: Yes — `regexp.MustCompile` at package level, `ansi.Strip` for the visible-text pass, `t.Helper()` throughout.
- Readability: Good, with one loss: the later repo-wide comment strip (`d939ae76`) removed the explanation of *why* `styledRunOpening` drops the trailing reset, which is the one non-obvious step in the file.
- Issues: None blocking. `model.(tui.Model)` at `grouped_subtle_locus_test.go:52` is an unchecked assertion, but `tui.Build` and its `Update` cannot return another type, so it is not reachable.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/builtins/nord.theme:41-45 — the `text.subtle` invention comment records only the interpolation arithmetic, while the file header claims an invention "is settled at a visual gate rather than by arithmetic" (a claim `bg.attention` substantiates and this token now can too, since the gate was taken and passed). Append one sentence to the existing comment block, before `text.subtle = #73819B`: `# Settled at a visual gate on a grouped frame, where it read as a quieter tally than the heading beside it while staying legible on the canvas.` Verify `TestNordFile_CorrectionsAndInventionsCarryComments` still passes (it asserts substring presence, so an addition is safe).
- [do-now] internal/capture/grouped_subtle_locus_test.go:55 — `styledRunOpening` drops the trailing reset with no stated reason since the comment strip; restore the one-line trap above the function: `// Only the opening SGR plus the text identifies a run: the outer canvas fill rewrites the trailing reset.`
- [quickfix] internal/capture/grouped_subtle_locus_test.go:37-48 — `renderFixtureFrame` hardcodes the caller's width/height and ignores `fx.RenderSize(width, height)` and `fx.captureKeys`, so the assertion would silently render a different frame from the capture if `groupedNordFixture` were ever pointed at a size-declaring or key-driven fixture (the task itself names `sessions-by-tag` as the fallback surface). Route the size through `w, h := fx.RenderSize(width, height)` before the `tea.WindowSizeMsg`.
