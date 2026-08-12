TASK: theming-system-7-2 — The Advisory Line Class: A Trailing ⚠ Block Outside The Exit Code

ACCEPTANCE CRITERIA (from the plan task):
1. With advisories present the rendered output is exactly: header, every check line in catalog order, every advisory line, then the summary — verified by index, not by substring presence.
2. No advisory line ever appears between two check lines, and the host-terminal info line is the last line of the catalog region.
3. `7 checks passed · 1 advisory` at M=1 and `... · 3 advisories` at M≥2; the same suffix appends to the `6 of 7 checks passed` form.
4. At M=0 the summary is byte-identical to task 7-1's output — no ` · `, no `0 advisories`.
5. An all-passing catalog plus 5 advisories Executes with exit 0 and a nil error; `doctorUnhealthy` is not consulted with advisory input.
6. A failing check plus advisories renders one summary line carrying both counts, and exits non-zero.
7. An advisory line renders with no pass/fail marker and no name column — its text begins with `⚠` after the indent.
8. Zero advisories render zero bytes between the last check line and the summary.
9. `<M>` is the length of the slice the renderer was handed — never recomputed from a producer's raw counts.

STATUS: complete

SPEC CONTEXT:
Spec §12.2 amends doctor's contract with a second class of report line: user-content diagnostics carry `⚠`, are glyph-backed (survive a colourless terminal) and must NOT drive the scriptable exit code, because there is deliberately no repair path for a user's broken theme file and the exit code speaks about the resurrection machinery. §12.2 further pins that advisories render as a trailing block after the ordered catalog and before the summary and never interleave, because the catalog is fixed-order/fixed-length while the theme class is 0..N lines whose cardinality depends on directory contents. §14A pins the summary suffix copy: either summary form plus ` · <M> advisory` at M=1 / ` · <M> advisories` above, "suppressed entirely at M=0", with `<M>` counting lines (problems, not detections). §14A also gives each advisory's copy WITH its leading `⚠`, so producers own the whole string and the renderer only indents. §12.2 additionally requires the advisories and the suffix on the `--fix` path.

IMPLEMENTATION:
- Status: Implemented (with two deliberate later-phase supersessions, detailed below)
- Location:
  - `cmd/doctor.go:49-53` — the `advisory` type and its doc comment.
  - `cmd/doctor.go:448-459` — `renderDoctorReport(w io.Writer, results []checkResult, advisories []advisory)`: header → catalog loop → advisory loop (two-space indent, `a.line` verbatim, nothing prepended/appended) → summary. The interleaving rule is doc-commented exactly at the region-2 → region-3 boundary (`cmd/doctor.go:453-454`).
  - `cmd/doctor.go:504-514` — `doctorSummaryLine(results, advisories)`: the suffix is appended to whichever form applies, gated on `if m := len(advisories); m > 0`, so M=0 is suppressed entirely rather than rendered as ` · 0 advisories`. `<M>` is `len()` of the slice handed to the renderer — never a producer count.
  - `cmd/doctor.go:476-483` — `doctorUnhealthy(results []checkResult)` untouched: advisories are structurally unable to reach the exit decision because they are never a parameter.
  - `cmd/doctor.go:155,176` — both `RunE` render calls (plain path and post-repair path) pass an advisory slice, satisfying §12.2's "the theme scan runs on the `--fix` path too".
- Notes on drift (both are intentional later-plan revisions, not defects):
  1. The task's `Do` block specified `advisory{line, slug, fromPrefs}`. The shipped struct carries only `line`; the `slug`/`fromPrefs` dedup identity lives on `themeAdvisory` (`cmd/doctor_theme.go:29-33`) and is dropped at the handover in `collectThemeAdvisories` (`cmd/doctor_theme.go:38-46`). This *strengthens* the task's own stated intent ("the renderer reads only `line`") and is pinned by `TestAdvisories_CarryOnlyTheRenderedLine`. The change came from the later comment/code-quality remediation cycles (`git log -L49,53:cmd/doctor.go` → `a4bc7bd5`, `915e7fcb`).
  2. The task said production supplies an **empty** slice here, with producers arriving in 7-3..7-6. Production now calls `collectThemeAdvisories(deps)` — the phase-boundary note explicitly anticipated this, and `cmd/doctor_fix_theme_test.go:276-297` holds an AST guard that both call sites are handed a fresh `collectThemeAdvisories(deps)` call rather than a reused variable.
- No colour is applied anywhere in the renderer (plain `fmt.Fprintf`), so the class is genuinely glyph-backed.

TESTS:
- Status: Adequate
- Coverage: all nine named micro-acceptance tests exist in `cmd/doctor_advisory_test.go`, plus one extra structural test:
  - `TestAdvisories_BlockPositionIsFixed` (`:64`) — exact full-report comparison by index (length check + per-index equality), covering AC 1.
  - `TestAdvisories_NeverInterleave` (`:89`) — pins each check line at its index over a catalog including `checkNotEvaluable`, pins the advisory block contiguous from `len(results)+1`, then sweeps every line for `⚠` inside the catalog region. AC 2 (first half).
  - `TestAdvisories_HostTerminalStaysInCatalog` (`:125`) — runs the **real** `runDoctorDiagnosis` and asserts the last catalog entry is the info host-terminal line, then that the first advisory follows it. AC 2 (second half); this is stronger than a hand-built fixture because it would fail if the catalog order changed.
  - `TestAdvisories_SummarySuffix` (`:148`) — table over M=1/2/5 against both summary forms, asserting `doctorSummaryLine` and the rendered last line. AC 3.
  - `TestAdvisories_SuffixSuppressedAtZero` (`:186`) — nil and empty slice, plus explicit "no `·`, no `advisor`" assertions. AC 4.
  - `TestAdvisories_NeverDriveExitCode` (`:209`) — AC 5.
  - `TestAdvisories_FailingCheckAndAdvisoriesShareOneSummary` (`:233`) — Executes with a failing saver check expecting `ErrDoctorUnhealthy`, then counts summary lines (exactly 1) and pins `6 of 7 checks passed · 2 advisories`. AC 6.
  - `TestAdvisories_GlyphBackedNoMarker` (`:262`) — indent-only, `⚠` immediately after the indent, and no `✓`/`✗` anywhere on the line. AC 7.
  - `TestAdvisories_EmptyBlockRendersNothing` (`:281`) — byte-exact whole-report comparison for the zero-advisory case plus nil/empty parity. AC 8 and AC 4's byte-identity claim (the expected string matches task 7-1's `TestDoctorSummary_IsTheLastLine` shape in `cmd/doctor_summary_test.go:146-166`).
  - `TestAdvisories_CarryOnlyTheRenderedLine` (`:49`) — reflection over the struct shape, pinning the renderer-side type to one field. Not in the plan's test list; it is the guard for the supersession above and is justified.
  - `cmd/doctor_advisory_test.go:14` — `var _ func([]checkResult) bool = doctorUnhealthy` is a compile-time pin that widening the signature to take advisories breaks the build. This is the strongest possible form of "advisories never reach the exit decision" and directly discharges the second half of AC 5.
- Notes:
  - AC 5's "Executes with exit 0 **plus 5 advisories** in one call" is discharged across two files rather than one: `TestAdvisories_NeverDriveExitCode` Executes over a zero-advisory healthy run and then does the 5-advisory part as a direct render + `doctorUnhealthy` re-check, while the genuine end-to-end (real advisories present, `Execute` → nil error, report closing `7 checks passed · 2 advisories`) lives at `cmd/doctor_persisted_theme_test.go:697-728`. Combined with the compile-time signature pin the criterion is genuinely met; no action needed.
  - Not over-tested. `BlockPositionIsFixed` and `NeverInterleave` overlap partially but differ in what fails them (exact whole-report bytes vs contiguity + a glyph scan over a different catalog shape including a not-evaluable row), and `EmptyBlockRendersNothing` adds nil/empty parity over `TestDoctorSummary_IsTheLastLine`. No redundant mocking: the fixtures are plain literals, and the two tests that need the real catalog use `healthyDoctorDeps` rather than fabricating one.
  - Test isolation holds: `cmd/testmain_isolation_test.go:19` poisons `PORTAL_THEMES_DIR` package-wide, so an Execute-based doctor test can never enumerate the developer's real themes directory.

CODE QUALITY:
- Project conventions: Followed. Renderer writes through the injected `io.Writer` and swallows write errors with `_, _ =` exactly as the pre-existing lines in the same function do; no `*slog.Logger` construction; `cmd` `*Deps` seam wiring untouched; tests carry no `t.Parallel()`; the new tests sit in the unit lane (no daemon spawn, no built binary), which is correct for a pure-render change.
- SOLID principles: Good. The renderer/summary split is preserved (`doctorSummaryLine` computes, `renderDoctorReport` emits); the advisory type is a pure carrier with the producer-side identity quarantined in `doctor_theme.go`, so the exit-code path and the diagnostic path have genuinely separate reasons to change.
- Complexity: Low. `renderDoctorReport` is three straight-line loops; `doctorSummaryLine` is two branches.
- Modern idioms: Yes — `for i := range n`, `reflect.TypeFor[advisory]()` in tests; `pluralCount` reused rather than a second pluraliser.
- Readability: Good. The doc comments carry the *why* (why the block trails, why the checks count skips `pluralCount` while the advisory count takes it) rather than restating the code, and contain no spec-section or task-id references.
- Comment accuracy: Verified. `cmd/doctor.go:49-50` ("sits outside the pass/fail catalog, and so outside the exit code"; "line is the whole rendered string, glyph included — the renderer only indents") holds against both the struct and `renderDoctorReport`. `cmd/doctor.go:453-454` holds against the emission order. `cmd/doctor.go:502-503` holds against `doctorSummaryLine`'s two counts. `cmd/doctor.go:174-175` ("re-collected, not carried down") holds against the two `collectThemeAdvisories(deps)` calls.
- Security: N/A — no new input parsing; advisory strings are producer-owned and the renderer only indents.
- Performance: Negligible. `--fix` enumerates the themes directory twice (once per report), which is the deliberate freshness contract, not an inefficiency.
- Issues: none.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] `cmd/doctor_advisory_test.go:18` — rename the fixture builder `themeAdvisories(n int) []advisory` to a producer-neutral name (e.g. `fakeAdvisories`), because `themeAdvisory` (`cmd/doctor_theme.go:29`) and `themeAdvisoriesFor` (`cmd/doctor_theme_test.go:46`) are the producer-side names for a different type; three near-identical identifiers spanning the two sides of the handover invite a reader to assume this helper exercises the theme producers when it does not.
