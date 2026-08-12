TASK: theming-system-7-6 — One Slug, One Advisory Line — The Persisted Line Wins (tick-3b3d5c)

ACCEPTANCE CRITERIA:
1. `theme_dark = nord-lee` + `nord-lee.theme` with a bad hex → exactly one advisory (`⚠ theme nord-lee (dark) does not resolve: bad colour`), no file line.
2. Same collision under a constant → one line, no parenthetical.
3. `Nord.theme` (`bad name`, no slug) + any persisted key → both lines.
4. `theme = nord` + drop-in `nord.theme` → only the file's `reserved name` line, no persisted line.
5. A persisted slug naming a valid drop-in → neither line.
6. Two slots naming the same broken slug → one `(both)` line, file line dropped.
7. An unusable-directory line is never dropped.
8. Two runs over unchanged directory + prefs are byte-identical; no map iterated in the assembly.
9. Pinned region order: directory line → file lines → persisted lines.
10. `<M>` equals the number of lines actually rendered, in every scenario.

STATUS: complete

SPEC CONTEXT:
§12.2 (specification.md:1467): "**One slug produces one advisory line**, mirroring §9.4's *'one slug is one row, always'* — the two surfaces render the same union and must not disagree about how many problems exist. When a persisted theme is *also* the invalid file … the **unresolvable-persisted line wins**: it carries strictly more — the reason *and* which slot is affected — so the file-validity line would add nothing but a second entry in `<M>`. `<M>` counts lines, so it counts problems rather than detections." §12.2 also pins the advisory block as a trailing, non-interleaved block before the summary, read-only, never driving the exit code, and present on the `--fix` path too. §14A pins the `both` case to one line. The spec does not pin the sequence of the three regions — the plan task records directory → files → persisted as an owned decision, which is what was built.

IMPLEMENTATION:
- Status: Implemented (mechanism later refined by tasks 13-8, 15-1 and the 11-3/12-7 comment audits — outcome unchanged).
- Location:
  - `cmd/doctor_theme.go:38-46` — `collectThemeAdvisories` calls the assembly and maps the internal `themeAdvisory` records down to the renderer's line-only `advisory` type.
  - `cmd/doctor_theme.go:51-56` — `themeAdvisoryUnion` opens ONE enumeration (`theme.NewSilentLoader().OpenEnumeration`) and drives both producers off it, then hands both results to the assembly.
  - `cmd/doctor_theme.go:61-72` — `assembleThemeAdvisories`: drops any scanned line whose non-empty slug is covered, then appends the persisted region. Region order pinned; no sort, no map.
  - `cmd/doctor_theme.go:76-84` — `persistedSlugs`: membership keyed on the record's own `fromPrefs` + non-empty `slug` (not argument position), returned as a slice.
  - `cmd/doctor_theme.go:29-33` — `themeAdvisory` carries `line`/`slug`/`fromPrefs`; `advisory` (`cmd/doctor.go:51-53`) stays line-only, so the identity fields cannot leak into the renderer.
  - `cmd/doctor.go:448-459, 504-514` — `renderDoctorReport` prints the block and passes the SAME slice to `doctorSummaryLine`, which uses `len(advisories)`; `<M>` is structurally the rendered count.
  - `cmd/doctor.go:155,176` — both `--fix` renders call `collectThemeAdvisories(deps)` freshly, so the assembled union rides both passes (task 7-7's boundary, verified only as non-regression here).
- Criteria trace:
  - AC1/AC2 — drop rule at `doctor_theme.go:66`; persisted line built at `:136-147` with the slot parenthetical suppressed for a constant (`:149-154`).
  - AC3 — `themeFileAdvisory`'s `ReasonBadName` arm (`:188-193`) sets `slug: ""` explicitly (stated, not copied from `Entry.Slug`), and the drop guard requires a non-empty slug, so a bad-name line can never be dropped.
  - AC4 — `theme.Loader.resolveNamed` (`internal/theme/resolve.go:27-37`) resolves built-ins at step 2 before consulting the directory/enumeration, so a persisted reserved slug returns no rejection and the persisted producer emits nothing; the file's reserved-name line survives.
  - AC6 — `theme.InForceKeys` (`internal/theme/setting.go:126-144`) collapses light==dark to one `Both` key and fixes the order (constant alone; else light then dark), so the persisted region cannot emit two lines for one slug and its order is pinned upstream.
  - AC7 — the directory line (`:158-161`) carries no slug, so the same guard protects it.
  - AC8/AC9 — `Enumerate` walks `os.ReadDir` order (`internal/theme/enumerate.go:28-52`), the assembly appends in region order, nothing sorts, and no map type appears in the assembly (guarded by a source scan, below).
  - AC10 — single-sourced through `renderDoctorReport`.
- Notes: `scanThemesDirectory` yields the directory line OR the file lines, never both (an unusable directory enumerates nothing), so the "three regions" arrive in two slices — consistent, not a gap. `slices.Contains` over a ≤2-element slice is O(n·m) but deliberate: the no-map property is the determinism argument and the sets are tiny.

TESTS:
- Status: Adequate.
- Coverage (`cmd/doctor_theme_union_test.go`, unit lane, no build tag, no daemon/binary — correct lane):
  - `TestThemeAdvisoryUnion_PersistedLineWins` (AC1/AC2) — slot and constant sub-cases; first asserts the file line IS produced in isolation (`scanThemesDirectory`, line 57) so the drop is proven rather than assumed, then asserts the single surviving line and `fromPrefs`. An end-to-end sub-test runs the real `doctor` body and asserts the report closes with the one line + `1 advisory` and does NOT contain the dropped line. A third sub-test hand-builds an unranked persisted advisory to prove rank comes from the `fromPrefs` field, not the argument slice.
  - `TestThemeAdvisoryUnion_BadNameFileNeverCollides` (AC3) — both lines, plus a check that the file advisory's slug is empty; a sub-test hand-builds a slugless persisted line to pin the non-empty-slug guard (unreachable from the producers, so only a hand-built pair reaches it).
  - `TestThemeAdvisoryUnion_ReservedNameResolvesToBuiltin` (AC4) — one line, `fromPrefs == false`, and no "does not resolve" text.
  - `TestThemeAdvisoryUnion_ValidPersistedFileIsSilent` (AC5) — includes a non-`.theme` file the enumeration must ignore.
  - `TestThemeAdvisoryUnion_BothSlotsStayOneLine` (AC6) — `requireOneAdvisory` proves the file line was also dropped, not just that the pair collapsed.
  - `TestThemeAdvisoryUnion_DirectoryLineIsNeverDeduped` (AC7) — a regular file where the directory belongs, with a persisted slug naming it.
  - `TestThemeAdvisoryUnion_OrderIsDeterministic` (AC8/AC9) — `everyRegionFixture` exercises all three regions plus the drop in one fixture and asserts the region boundary by `fromPrefs` index rather than by how the slugs happen to sort; a 10-run byte-identical render comparison; a reverse-alphabetical seeding case pinning enumeration order; and an AST guard asserting no `MapType` appears in `collectThemeAdvisories`/`themeAdvisoryUnion`/`assembleThemeAdvisories`/`persistedSlugs`, erroring if any of the four is absent so a rename cannot silently shrink the guard.
  - `TestThemeAdvisoryUnion_CountMatchesRenderedLines` (AC10) — four scenarios (nothing, covered, two empty-slug lines, disjoint slugs) each comparing rendered `⚠` lines to `len(union)` and to the summary's singular/plural suffix; plus a same-(empty)-slug pair proving `<M>` counts lines and not distinct slugs.
  - Compile-time guard at line 24 (`var _ func([]themeAdvisory, []themeAdvisory) []themeAdvisory = assembleThemeAdvisories`) — widening the assembly to the renderer's line-only type breaks the build, so the dedup identity cannot be dropped.
- Notes: every planned test name from the task exists with matching semantics. Not over-tested: each test maps to a distinct acceptance criterion, and the three hand-built-value cases are justified in-comment as the only route to guards the producers cannot reach. The 10-run determinism loop is more repetition than the "twice" the task asked for, but it is the standard idiom for smoking out map-iteration randomness and costs only a few small directory scans — left alone deliberately. Divergence between `<M>` and the rendered block is additionally impossible by construction (`renderDoctorReport` hands `doctorSummaryLine` the same slice it printed), so the per-scenario assertions are belt-and-braces rather than the only defence.

CODE QUALITY:
- Project conventions: Followed. Unit-lane placement is correct (no tmux/daemon/binary); the end-to-end sub-test drives the Cobra body through `runDoctorCmd` with `doctorDeps` injected and `isolateTerminalsFile`, and doctor is bootstrap-exempt. Doctor's theme path stays read-only: `NewSilentLoader` (Discard-backed) keeps the `theme` log component recording use and never diagnosis, and the prefs read goes through `loadPrefsStoreNoMigrate`.
- SOLID principles: Good. Three clean layers with one job each — `assembleThemeAdvisories` is a pure function of two slices (directly unit-testable, no deps), `themeAdvisoryUnion` wires the producers to one enumeration, `collectThemeAdvisories` adapts to the renderer's narrower type. The internal-vs-rendered type split (`themeAdvisory` vs `advisory`) keeps the dedup identity out of the report layer.
- DRY: Good. Persisted-key order and the `both` collapse are single-sourced in `theme.InForceKeys`; the slot label goes through `Slot.AttrName`; both surfaces read one enumeration.
- Complexity: Low. The assembly is one loop and one guard; no branching on producer identity.
- Modern idioms: Yes — `slices.Contains`, pre-sized `make(...,0,n)` accumulators, `for range n` in tests.
- Readability: Good. Minor: `collectThemeAdvisories` / `themeAdvisoryUnion` / `assembleThemeAdvisories` are three similar names in one file, though each doc comment disambiguates the job.
- Comment accuracy: Comments hold against the code and carry no spec-section or task/phase citations (the original 7-6 comments cited §12.2/§9.4/§6.2 and were deliberately stripped by tasks 11-3/12-7 and the comment audits — intended supersession, not drift). One documentation gap survives that strip: see the first non-blocking note.
- Security / performance: Nothing of note. Persisted slugs are control-stripped upstream (`theme.NewRawKeys`) and every producer line is single-line by construction, so no injected newline can make `<M>` disagree with the printed line count.
- Issues: none blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] cmd/doctor_theme.go:58-60 — the doc comment on `assembleThemeAdvisories` pins the order and the win rule but no longer states the reserved-name non-collision, so a reader hitting `theme = nord` + a drop-in `nord.theme` has nothing in the file explaining why only one line appears and why no special case is needed. Append to that comment: `// A persisted slug naming a reserved-name file resolves to the built-in, so no persisted line exists to drop it against and the file keeps its own.`
- [do-now] cmd/doctor_theme.go:58 — the comment says "Region order is pinned (directory, files, persisted)" while the assembly receives two slices, because `scanThemesDirectory` returns the directory line or the file lines and never both. Replace that opening clause with: `// Region order is pinned — the directory line or the file lines it explains away (scanThemesDirectory yields one or the other), then the persisted lines.`
