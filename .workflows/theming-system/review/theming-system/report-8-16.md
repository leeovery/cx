TASK: theming-system-8-16 — The Five Remaining Panel Fixtures (`theme-panel-invalid-row`, `theme-panel-dir-unreadable`, `theme-panel-narrow`, `theme-panel-paginated`, `theme-panel-projects`)

ACCEPTANCE CRITERIA (from plan):
1. `theme-panel-invalid-row` renders a `text.subtle` label with an `accent.attention` `⚠` and terse reason on one line, plus a persisted-and-invalid row where the badge is present and the reason absent, plus a truncated label ending in `…`.
2. `theme-panel-dir-unreadable` renders `⚠ dir unreadable` directly beneath the header on page 2, with built-in and persisted rows still rendering beneath it.
3. `theme-panel-narrow` renders the panel at a width strictly between `themePanelMinWidth` and `themePanelPreferredWidth`, every row still one line.
4. `theme-panel-paginated` overflows the panel body so the pagination dots render, and §13.4's guard exercises those dots.
5. `theme-panel-projects` renders the panel over Projects with badges visible and the Projects footer cut mid-label by the overlay.
6. All five appear in both registries and task 4-3's drift check passes.
7. All five are enumerated by §13.4's guard with no test edit.
8. No fixture reaches a real themes directory, `prefs.json` or an XDG lookup; both import guards stay green.
9. A `.tape` exists per fixture and each PNG is verified as a fresh write before review.
10. No confirm, failed-commit or minimum-height-with-message fixture is added in this phase.

STATUS: complete

SPEC CONTEXT:
§13.3 requires new slide-over fixtures so every specified panel surface is visible during implementation rather than at release, naming the invalid-theme row, the pinned `⚠ dir unreadable` row ("no other way to be checked"), the narrow degraded panel, a panel long enough to paginate, and the panel over Projects — closing with "a missing fixture is a blind spot the guard structurally cannot report: §13.4 enumerates whatever fixtures exist, so absence reads as coverage." §11.2 states the coverage consequence: pagination dots only render when the panel's list paginates, so one fixture must carry enough rows to overflow, or the guard is blind at the `bubbles/list` instance the panel adds. §9.5 pins the directory row as viewport chrome, not a list row, precisely because a list row would vanish on page-down. §14A requires one Projects-with-panel fixture because `t` is bound there. §13.3 also states the four-input fixture contract (palette, raw keys, faked union, cursor) and the coherence rule that `--theme` names the theme under the cursor; §13.2/§13.3 make tapes and PNGs scaffolding written as work proceeds and cleared at sign-off.

IMPLEMENTATION:
- Status: Implemented (with two criteria met as amended by later plan phases — see Notes)
- Location:
  - `/Users/leeovery/Code/portal/internal/capture/fixtures.go:523-552` (`themePanelInvalidRowFixture`), `:554` (`themePanelBrokenSlug`), `:560-577` (`themePanelDirUnreadableFixture`), `:581` (`themePanelUnreachableLightSlug`), `:587-592` (`themePanelNarrowFixture`), `:596-621` (`themePanelPaginatedFixture` + `themePanelSyntheticDropIns`/`themePanelSyntheticSlug`/`themePanelPaginatedEntries`), `:626-637` (`themePanelProjectsFixture`)
  - Registry: `/Users/leeovery/Code/portal/internal/capture/fixtures.go:133-163` (`fixtureBuilders`, the single list both `FixtureByName` and `FixtureNames` derive from after task 17-12) — all five present at `:151-155`
  - Key script + geometry seams: `/Users/leeovery/Code/portal/internal/capture/harness.go:22-56` (`ModelAt` replays `captureKeys`, `RenderSize` substitutes a declared dimension), `/Users/leeovery/Code/portal/internal/capture/fixtures.go:61-64` (`captureKeys`), `:15-19` (`width, height`)
  - Geometry under test: `/Users/leeovery/Code/portal/internal/tui/theme_panel_geometry.go:11-16, 76-78, 97-102`
- Notes:
  - Criterion 1: the fixture declares three rejected entries — reason-only (`aurora-glow`, `bad syntax`), badged-and-invalid (`nord-lee`, `bad colour`, carrying the dark slot's `Requested`), and an over-long `bad name` filename with an empty slug. Cursor seeded on `tokyo-night` (a valid row), matching the arrow-skip invariant. Coherent with `--theme tokyo-night` per §13.3's rule.
  - Criterion 2: `Enumeration{DirUnusable: true}` with both slots naming unresolvable drop-ins, so two persisted badged rows straddle the page break (`solarized-lee` is deliberately named to sort between the `nord*` and `tokyo-night*` rows); `captureKeys` = `t`, `Ctrl+↓`.
  - Criterion 3 met **as amended**: task 13-11 reconciled the width ladder with §9.8's 24–30 band, and `themePanelWidthFor` is a two-stage ladder (30 preferred / 24 minimum) with no intermediate value, so "strictly between min and preferred" is unsatisfiable by construction. The fixture instead declares `tui.ThemePanelMinWidthTerminal()` — the widest terminal whose content region steps the panel *down* — which is the only observable point on the ladder. That is the criterion's intent (the degraded band), not drift.
  - Criterion 4: 30 synthetic `vivid-NN` drop-ins are appended to the base enumeration and the union is re-derived through production `Assembler.Reassemble`, so 34 rows overflow the 120×40 harness body. Slugs sort after every built-in, keeping both badged rows on page 1.
  - Criterion 5: derived from `projectsFixture` with `captureKeys` = `x`, `t`, and the adaptive pair's four theme inputs copied.
  - Criterion 6/7: the registry is now one authoritative list (task 17-12), so both lookups and §13.4's guard (`registryFixtures` → `guardedFixtures`, `theme_swap_guard_test.go:95-119`) enumerate all five with no per-fixture test edit. The only test-side registration is `capturedStates()` (`swap_harness_test.go:53-57`), which is the state-reachability table, not the guard.
  - Criterion 8: every union is assembled in-process from declared `theme.Entry` values through `theme.NewSilentLoader()` (`fixtures.go:421-423`); no path is opened, and `themesDirPath` (`fixtures.go:498`) is a never-read literal. Enforced at runtime for every panel fixture by `TestPanelFixture_NoConfigAccess` (`theme_panel_fixture_render_test.go:70-77`), which stages a decoy themes dir + `PORTAL_PREFS_FILE` and asserts neither is reached. The binary import guard (`cmd/capturetool/import_guard_test.go`) is untouched.
  - Criterion 9: all five `.tape` + `.png` artifacts shipped in this task's commit `534909c9` and were deliberately cleared by task 10-10 (`71e24eef`) under §13.2's retention rule — the absence of `testdata/vhs/theme-panel-{invalid-row,dir-unreadable,projects}.*` today is the amended convention, not a gap. The post-load key script that lived in the tapes now lives on the fixture (`captureKeys`).
  - Criterion 10: satisfied at the time; the confirm / commit-failed / min-height-message fixtures were added later by Phase 9 as planned, so their presence now is not scope creep.
  - Fixture data is coherent with production assembly: rows come from `Assembler.Reassemble` (`internal/theme/union.go:131-137`), badges from declared `theme.SlotResolution` (`Requested`, not `Resolved`), and the fake source repaints only selectable rows (`internal/capture/theme_fake.go:57-67`), so a rejected row carrying a palette — a shape real assembly cannot produce — stays unreachable.
  - No process-artifact references in any of the added comments (grep for `task `/`§`/`Phase ` over `fixtures.go` and the test file returns nothing).

TESTS:
- Status: Adequate
- Coverage (`/Users/leeovery/Code/portal/internal/capture/theme_panel_remaining_fixtures_test.go`):
  - `TestPanelFixture_InvalidRowFrame:66-120` — reason on the label's line, `text.subtle` + `accent.attention` SGR runs present on that line, badged row keeps `● dark` and loses `bad colour`, over-long label truncated to `My Gorgeous Midnight Pa…`, cursor on a valid row and on neither invalid one.
  - `TestPanelFixture_InvalidPersistedRowDropsTheReason:122-131` — the composition priority with its control (the unbadged row *does* render a reason, so the badged row's missing one means something).
  - `TestPanelFixture_DirUnreadableIsChromeOnPageTwo:133-156` — renders at 120×16, fatals if page-1 rows are still present (so the `Ctrl+↓` cannot silently no-op), pins the verbatim copy, pins the row above every list row and below the header.
  - `TestPanelFixture_RowsBeneathDirRow:158-173` — a built-in row and a badged persisted row still render beneath the chrome row.
  - `TestPanelFixture_NarrowRendersTheStepDown:175-207` — measures both ladder ends off the base fixture, fatals if they are equal (no vacuous pass), asserts the narrow frame renders the stepped-down width, every union row on exactly one line, badges surviving.
  - `TestPanelFixture_PaginatedDrawsDots:209-234` — includes the negative control (the four-row panel draws no dots at the same size) and asserts both the active dot (`accent.primary`) and an inactive dot (`text.faint`) are painted from the live theme, which is the §11.2 restyle site.
  - `TestPanelFixture_OverProjects:249-289` — page identity via `ActivePage()`, badges over the composited page, and the footer cut asserted as a *prefix* of the bare footer (proving cut-not-reflow).
  - Registry/guard/config coverage is shared rather than duplicated: `TestPanelFixture_RegistryHoldsTheSpecifiedPanelSet:304-312`, `TestPanelFixture_Registered` and `TestPanelFixture_NoConfigAccess` (`theme_panel_fixture_render_test.go:70-77, 214-233`), `TestModelAt_ReachesCapturedState` (`swap_harness_test.go:76-116`, whose completeness subtest forces every fixture into the table), and `TestFixtureRenderSize_*` (`fixture_render_size_test.go`).
- Notes:
  - Test names diverge from the plan's stated names (`NarrowIsBetweenMinAndPreferred` → `NarrowRendersTheStepDown`, `AllRegistered` → `RegistryHoldsTheSpecifiedPanelSet`, `RemainingFramesNoConfigAccess` → generalised into `TestPanelFixture_NoConfigAccess`). Each renaming tracks a later-phase consolidation and every claim still has a home; `NoMessageSlotFixtures` is correctly gone because Phase 9 added those fixtures by plan.
  - Not over-tested at this layer: row-composition rules themselves live in `internal/tui/theme_row_test.go` / `theme_panel_*_test.go`; these tests assert only that the *frame* carries the surface, which is the fixture's job.
  - One genuine redundancy: the "badged row has no reason" claim is asserted twice (row-scoped at `:95-97`, frame-wide at `:128-130`).
  - Failure messages are behavioural and explain the invariant rather than restating the assertion — consistent with the rest of the package.

CODE QUALITY:
- Project conventions: Followed. Unit-lane only, no `t.Parallel()`, no real tmux/daemon/binary, test-only helpers (`themetest`) used rather than hand-rolled fixtures, `internal/capture` reaches no config, comments carry no task/phase/spec citations.
- SOLID principles: Good. Each fixture is a builder returning declarative data; membership, dedup and order are delegated to production `Assembler.Reassemble` rather than hand-written unions, so the harness cannot drift from the panel's real assembly. The registry is a single list, so the two lookups cannot disagree.
- Complexity: Low. The five builders are flat field assignments over a shared base; the only loop is the 30-row synthetic generator.
- Modern idioms: Yes — `for i := range n`, `slices.Clone`, `strings.SplitSeq`/`CutSuffix` in the tests.
- Readability: Good. Every non-obvious datum carries a rationale comment (why the cursor is on `nord` rather than the dark fallback, why `solarized-lee` sorts where it does, why 30 rows rather than the exact overflow threshold, why the invalid row's rejection is declared on the entry).
- Issues: see the non-blocking notes; the material one is that the dir-unreadable fixture's page-2 frame depends on a terminal height nothing in the repo declares.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/capture/fixtures.go:556-577 — `themePanelDirUnreadableFixture` declares no render height, so its captured frame is only the specified page-2 frame at a short terminal. Its 5-row union fits one page at the harness size (and at any normal terminal), where `Ctrl+↓` is a no-op and the frame is page 1 — indistinguishable from a warning implemented as a list delegate, which is the one thing the fixture exists to disprove (`swap_harness_test.go:54` renders it at 120×40 and sees every row; only `theme_panel_remaining_fixtures_test.go:34,134` gets page 2, by passing `dirUnreadablePanelTermHeight = 16`). The height that produced the reviewed PNG lived in the now-deleted tape (`Set Height 476`), so `capturetool --fixture theme-panel-dir-unreadable` no longer reproduces it. Set `fx.height` through the task-17-11 mechanism (`Fixture.height` / `RenderSize`) and enrol the fixture in `fixture_render_size_test.go:16-19`'s geometry set; the comment at fixtures.go:556-559 ("hence the `Ctrl+↓` and a capture height that forces a second page") then describes something the fixture actually declares.
- [do-now] internal/capture/theme_panel_remaining_fixtures_test.go:32 — `minimumPanelTermWidth = 28` is an unexplained literal. Add above it: `// The narrowest terminal whose content region (termW − 2*tui.Hinset) is exactly themePanelMinWidth, so the panel renders at the bottom of the ladder.`
- [quickfix] internal/capture/theme_panel_remaining_fixtures_test.go:95-97 — this row-scoped "the badge has no `bad colour`" assertion duplicates the frame-wide one at :128-130, which is the stronger form and carries the control. Delete the :95-97 block and leave the `● dark` assertion in that subtest.
- [quickfix] internal/capture/fixtures.go:632 — `themePanelProjectsFixture` re-derives a union identical to the one `pair` already computed one line above. Assign `fx.themeUnion = pair.themeUnion` alongside the other three copied theme inputs, so the fixture states "the pair's inputs" once rather than half-copying and half-recomputing them.
