TASK: theming-system-13-8 — Move The Theme Advisory Dedup Identity Onto A Theme-Local Record (tick-662eb1, severity low, source: architecture)

ACCEPTANCE CRITERIA:
- `advisory` carries only what the renderer reads.
- The one-slug-one-line dedup runs over `themeAdvisory` and is unreachable from a non-theme producer.
- Doctor's rendered output, line order and exit code are byte-for-byte unchanged for every existing case, including an unresolvable persisted slug outranking the same slug's file-validity line.
- `go test ./cmd` passes.

STATUS: complete

SPEC CONTEXT:
Spec §12.2 / §14A govern doctor's second report class. Key constraints the refactor had to preserve: theme lines are advisory and never drive the exit code (spec:1456); the closing summary carries a separate `· <M> advisor{y,ies}` count that is suppressed at M=0 (spec:1465, 1852); **one slug produces one advisory line**, and where a persisted theme is also the invalid file the unresolvable-persisted line wins because it carries strictly more — reason *and* slot (spec:1467); `<M>` counts lines, not distinct slugs (spec:1467); the scan runs on the `--fix` path too, in both passes (spec:1471); the slug renders untruncated because doctor has full width (spec:1094). This task changes none of that behaviour — it relocates where the dedup identity lives so the rule cannot be defeated by a future producer of the shared `advisory` class.

IMPLEMENTATION:
- Status: Implemented (subsequently revised by later in-plan phases — 15-1, 16-8, 17-7 and the two comment-audit sweeps — all of which preserved this task's split)
- Location:
  - `cmd/doctor.go:49-53` — `advisory` reduced to the single `line` field; doc comment keeps the "line is the whole rendered string, glyph included — the renderer only indents" rule and no longer mentions slug/fromPrefs/dedup.
  - `cmd/doctor_theme.go:28-33` — new `themeAdvisory{line, slug, fromPrefs}` with the union-identity paragraph on it.
  - `cmd/doctor_theme.go:38-46` — `collectThemeAdvisories` is the single boundary: it calls `themeAdvisoryUnion` and converts to `[]advisory` line-only.
  - `cmd/doctor_theme.go:51-56` — `themeAdvisoryUnion` (new, extracted) returns the still-identified `[]themeAdvisory`.
  - `cmd/doctor_theme.go:61-84` — `assembleThemeAdvisories` and `persistedSlugs` run over `[]themeAdvisory`.
  - `cmd/doctor_theme.go:91-108, 136-147, 158-170, 176-205` — all three producers (`persistedThemeAdvisories`/`persistedThemeAdvisory`, `scanThemesDirectory` incl. the slugless themes-dir-unreadable line at :160, `themeFileAdvisory`) return `themeAdvisory`.
  - Only consumer of `[]advisory` in production is `renderDoctorReport` (`cmd/doctor.go:448-459`) + `doctorSummaryLine` (:504), both reading `line` alone; both call sites (`cmd/doctor.go:155` plain run, `:176` post-`--fix` re-render) feed it from `collectThemeAdvisories`.
- Notes:
  - Do-step 6 verified: `collectThemeAdvisories` (`cmd/doctor_theme.go:43`) is the *only* production construction of an `advisory` value anywhere in the repo (grep across `cmd/` and `internal/`), so no non-theme producer ever set `slug`/`fromPrefs` — nothing was silently dropped, and there is no behaviour change to surface.
  - Byte-for-byte output invariance holds structurally: the conversion preserves order and copies `line` verbatim; the renderer and summary never read the removed fields. Persisted-outranks-file is untouched — rank is still read off `fromPrefs` in `persistedSlugs` (`:76-84`), not off argument position.
  - Nil/empty parity preserved: the conversion returns a non-nil empty slice at zero advisories, which `doctorSummaryLine`'s `len(...) > 0` gate and the renderer's range treat identically to nil (pinned by `TestAdvisories_EmptyBlockRendersNothing`).
  - `themeAdvisory` is package-scoped (Go has no file scope), so encapsulation is as tight as the language allows without extracting a package; the load-bearing half of the AC — that a producer of the *renderer's* class has no identity field to leave unset — is fully achieved and compile-enforced.
  - Later-phase revisions to this file (one retained `Enumeration` driving both producers, explicit slice pre-allocation, comment compression) all kept the `themeAdvisory` → `advisory` boundary intact. Not drift.

TESTS:
- Status: Adequate
- Coverage (all four micro-acceptance tests from the task are present):
  1. `cmd/doctor_advisory_test.go:49-62` `TestAdvisories_CarryOnlyTheRenderedLine` — reflect-based structural guard on field count and field name; directly pins AC1 and fails the moment a field is re-added.
  2. `cmd/doctor_theme_union_test.go:24` — compile-time signature pin `var _ func([]themeAdvisory, []themeAdvisory) []themeAdvisory = assembleThemeAdvisories`; widening the assembly back to the line-only class stops the file compiling. This is what makes AC2 ("unreachable from a non-theme producer") enforced rather than asserted.
  3. `cmd/doctor_theme_union_test.go:35-100` `TestThemeAdvisoryUnion_PersistedLineWins` — persisted + scan for one slug collapse to one line with the persisted one winning, over both the slot and constant prefs shapes; includes an end-to-end `runDoctorCmd` assertion on the rendered tail (`⚠ … (dark) does not resolve: bad colour` + `7 checks passed · 1 advisory`) and an explicit check that the dropped file line is absent, plus the rank-by-field-not-argument-position subtest.
  4. `cmd/doctor_theme_union_test.go:217-238` `TestThemeAdvisoryUnion_HandsTheRendererLinesOnly` (added by this task) — asserts the collected block equals a line-only `[]advisory`, then appends a *non-theme* producer's advisory whose line names the same slug and shows both lines render and both count (`2 advisories`): the non-theme line renders correctly and does not participate in theme dedup.
  5. `cmd/doctor_theme_union_test.go:204-215` `TestThemeAdvisoryUnion_DirectoryLineIsNeverDeduped` — the slugless themes-dir-unreadable line survives assembly alongside a persisted line; reinforced by the hand-built empty-slug guard case at `:153-163`.
- Regression surface kept green with unchanged expectations: order/region determinism (`TestThemeAdvisoryUnion_OrderIsDeterministic`, incl. the 10-run byte-identical repeat and the AST map-free guard over the four assembly functions), `<M>`-counts-lines (`TestThemeAdvisoryUnion_CountMatchesRenderedLines` + its same-slug pair subtest), bad-name/reserved-name/both-slots cases, and the whole `doctor_advisory_test.go` render-contract suite.
- Notes:
  - Correctly *removed* rather than left stale: the old `GlyphBackedNoMarker` subtest "the renderer reads only line" (which built two advisories differing only in slug/fromPrefs) is deleted — it is unrepresentable once the fields are gone, and `TestAdvisories_CarryOnlyTheRenderedLine` is its stronger replacement. This is not a coverage loss.
  - Test churn is proportionate: one new guard test, one new boundary test, one compile-time pin, and mechanical type substitutions across five test files. No new mocks, no new fixtures.
  - Not over-tested for this task. (`doctor_advisory_test.go` carries some pre-existing overlap between `BlockPositionIsFixed` / `NeverInterleave` / `HostTerminalStaysInCatalog` on block position, and between the four summary-suffix tests — but that file predates this task and this commit only retyped its helper.)
  - Not verified by execution (verification is read-only by design): `go test ./cmd` was not run. The change is type-level with a mechanical conversion; every reference to the removed fields in the package was retyped in the same commit and no stale `advisory{… slug:` / `.fromPrefs` reference on the `advisory` type survives anywhere in the repo.

CODE QUALITY:
- Project conventions: Followed. `cmd`'s `*Deps` DI seam is untouched; the `theme.Loader` seam is still injected; no logging added on doctor's read-only path (correctly still `theme.NewSilentLoader()` — the `theme` component records use, never diagnosis, per CLAUDE.md and spec §12.3). Comment style matches the audited house style in this package (rationale-first, not name-first).
- SOLID principles: Good — this is precisely an ISP/SRP fix. The renderer's class now carries exactly its render contract; the union's identity lives with the rule that reads it. `collectThemeAdvisories`/`themeAdvisoryUnion` splits "produce the union" from "hand the renderer its class" at one named boundary.
- Complexity: Low. The added conversion is a 5-line loop; no branch added anywhere.
- Modern idioms: Yes — `reflect.TypeFor[advisory]()` in the guard, `for i := range n`, `slices.Contains`/`slices.Equal`, compile-time `var _ func(...) = f` pins.
- Readability: Good. `themeAdvisoryUnion` gives the still-identified stage a name, which is what lets the tests observe the union without reaching through the renderer's class.
- Comment accuracy: Verified line by line against the current code. `cmd/doctor.go:49-50` (glyph/indent rule) holds; `cmd/doctor_theme.go:28` ("slug and fromPrefs are the dedup identity the assembly keys on") holds; `:74-75` ("A slice, not a map … determinism a property of its data structures") holds and is itself guarded by the AST test; `:58-60` (pinned region order, persisted drops the same slug's file line) holds. No stale claims, no restated code, no task-id/phase/spec-section references in source.
- Security: N/A — no new I/O, no new input handling.
- Performance: Unchanged bar one extra O(n) slice copy at the render boundary, where n is the number of advisory lines (typically 0–5).
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
