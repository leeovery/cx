TASK: theming-system-11-13 — De-Duplicate The Capture Fixture And Test-Harness Helpers (severity low, source: duplication)

ACCEPTANCE CRITERIA:
- `capture_test` has one "load a built-in or Fatal" body, one SGR-parameter probe body and one panel-frame renderer body.
- No string literal for the themes directory path remains in `fixtures.go`.
- `themePanelPaginatedFixture` is derived from `themePanelAdaptivePairFixture()` and states only its three differences.
- All nine panel fixtures render byte-identical to their committed references.

STATUS: complete

SPEC CONTEXT: §13.1–13.4 make `capturetool` + `internal/capture` the only route to seeing a visual change before release; the fixture *definitions* and the harness are permanent (the swap-and-diff completeness guard drives the fixture renderer and enumerates whatever fixtures exist), while the PNGs/tapes are scaffolding with no visual-regression obligation. §13.3 also pins that one panel fixture must carry enough rows to paginate, or the pagination-dots site is uncovered — which is exactly the fixture (`theme-panel-paginated`) this task rebuilt from the adaptive pair. The spec endorses deliberately shallow fixtures ("about look, not behaviour"), so a dedup that preserves the declared inputs verbatim is squarely within its intent.

IMPLEMENTATION:
- Status: Implemented (all seven "Do" items; two of them later superseded by an intended cross-package extraction)
- Location: commit `7ec90991`; current state at `internal/capture/fixtures.go:425-426, 498, 502-503, 515-516, 596-602`, `internal/capture/swap_harness_test.go:71-74, 118-126`, `internal/capture/theme_panel_fixture_render_test.go:21-24`, `internal/capture/theme_panel_remaining_fixtures_test.go:37-50`, `internal/capture/theme_swap_guard_test.go:53-62`
- Notes:
  - Item 1 (built-in loader): the commit collapsed `darkBuiltin`, `nordBuiltin`, `darkBuiltinTheme` and `lightBuiltinTheme` onto the single `builtinPalette(t, slug)` body — more than the task asked (two extra copies in `capture_test.go` and `grouped_subtle_locus_test.go` were folded in too). A later plan task then hoisted that body into `internal/themetest` (`themetest.Builtin` / `DefaultDark` / `DefaultLight`), so at HEAD `internal/capture` holds **zero** `LoadBuiltin` bodies (verified: no `LoadBuiltin` / `NewLoader(nil)` occurrence anywhere in the package). That is the intended supersession, and it satisfies the criterion more strongly than the task's own wording.
  - Item 2: `bgSeq` (`swap_harness_test.go:123`) and `fgSeq` (`theme_panel_remaining_fixtures_test.go:47`) are now one-line calls to `sgrParameterRun` (`theme_swap_guard_test.go:53`); the two duplicated eight-line bodies are gone.
  - Item 3: `panelFixtureFrame` (`theme_panel_fixture_render_test.go:21`) is a one-line call to `panelFrameAt`; the duplicated body and the "It exists beside panelFixtureFrame" doc line are gone (the doc was rewritten to justify the explicit size, then dropped entirely by the later comment sweep).
  - Item 4: `themePanelEnumeration` is `themePanelDirEnumeration(themePanelDirEntry(themePanelDropInSlug+theme.FileExtension, themePanelDropInSlug))` (`fixtures.go:425`) — it also picked up `theme.FileExtension` over the `".theme"` literal.
  - Item 5: `themesDirPath` (`fixtures.go:498`) is the only occurrence of the themes-directory path in the file — grep for `/themes` / `portal/themes` in `fixtures.go` returns the const declaration and nothing else.
  - Item 6: `themePanelPaginatedFixture` (`fixtures.go:596`) starts from `themePanelAdaptivePairFixture()` and overrides exactly `name`, `themeEnumeration` and `themeUnion` — three differences, matching how narrow / commit-failed / min-height are derived. I diffed the pre-change body field-for-field against the base (`git show 7ec90991^`): `themeKeys`, `themeSlots`, `initialThemeCursor` and `captureKeys` were verbatim identical, so the derivation is behaviour-preserving and implies no re-capture (criterion 4).
  - Item 7 (scope note) respected: no cross-package loader copy was moved by this task.
  - No drift: nothing outside the capture harness was touched, and `internal/capture` is import-guarded out of the production binary, so there is no production-behaviour surface here at all.

TESTS:
- Status: Adequate
- Coverage: A pure de-duplication needs no new tests, and none were added — correct. Existing coverage exercises every redirected helper: `panelFixtureFrame`/`panelFrameAt` back ~15 assertions across `theme_panel_fixture_render_test.go`, `theme_panel_remaining_fixtures_test.go`, `theme_panel_message_fixtures_test.go` and `fixture_render_size_test.go`; `bgSeq`/`fgSeq` back the canvas-carrying and token-locus assertions; `sgrParameterRun` additionally backs the swap-and-diff guard's per-token form table (`theme_swap_guard_test.go:46-47`). A broken redirect therefore fails loudly rather than silently passing.
- Notes: the riskiest edit (item 4/6 — routing the base enumeration through a shared constructor and then cloning it for the paginated set) has a directly matching guard: `TestPanelPaginatedEntries_DeriveFromBase` (`theme_panel_fixture_test.go:590`) pins that the base entries lead the paginating parse in order, that the synthetics follow, and — the aliasing case this refactor could have introduced — that `themePanelEnumeration` mints fresh entries per call (`:612-618`). `TestFixtureRenderSize_GeometryFixturesDifferFromTheirBases` (`fixture_render_size_test.go:28`) covers the sibling risk that a derived fixture becomes indistinguishable from its base. Not over-tested: no assertion duplicates another, and no test was added merely to observe the refactor.

CODE QUALITY:
- Project conventions: Followed. Test-only helpers stay in `_test.go` files; the later `themetest` extraction matches CLAUDE.md's test-only-package rule (`*testing.T`-first, production must not import). No `t.Parallel()` introduced. No production code path touched.
- SOLID principles: Good — each helper now has one body and one reason to change; the specialised probes (`bgSeq`/`fgSeq`) remain as intention-revealing names over the general `sgrParameterRun` rather than being deleted, which keeps call sites readable.
- Complexity: Low. Net −79 lines across the commit.
- Modern idioms: Yes (`slices.Clone` in `themePanelPaginatedEntries`, `theme.FileExtension` over the literal).
- Readability: Good. The derived-fixture chain (`sessionsFlat → adaptivePair → {commitFailed → minHeight, narrow, paginated}`) is now uniform, so a reader can see at a glance what each frame varies.
- Issues: none found in the changed code. Comments in the touched regions hold true against it: `themesDirPath`'s "Never resolved, opened or stat'ed; shared so no fixture invents a second one" is now literally true (it was aspirational before this task, with two literals bypassing it), and the reworded `panelFrameAt` doc no longer claims a sibling that no longer has its own body.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/capture/swap_harness_test.go:71,118 — after the `themetest` extraction, `darkBuiltinTheme`/`lightBuiltinTheme` are pure one-line aliases of `themetest.DefaultDark`/`DefaultLight`, and package `capture_test` now spells the same two palettes three ways: the wrappers, and `themetest.Builtin(t, theme.DefaultDarkSlug)` / `themetest.Builtin(t, theme.DefaultLightSlug)` at theme_panel_fixture_render_test.go:74,113,141,157 and theme_panel_message_fixtures_test.go:52,98,259,313,341. Settle on one — either delete the two wrappers and call `themetest.DefaultDark(t)`/`DefaultLight(t)` at every site (including the ~25 `darkBuiltinTheme(t)` calls in capture_test.go), or keep the wrappers and route those nine `themetest.Builtin(t, theme.Default*Slug)` call sites through them.
- [idea] internal/capture/theme_panel_fixture_test.go:579 — `backgroundSGR` is a fourth verbatim copy of the SGR-probe body (identical semantics to `bgSeq`), but it lives in the internal test package `capture`, not `capture_test`, so it cannot call `sgrParameterRun` and is outside this task's stated criterion. Consolidating it means deciding where the probe should live — e.g. hoisting it into `internal/themetest` (or a `logtest`-style shared test package) and having both test packages call it — which is a placement decision rather than a mechanical edit, and is the same cross-package class this task's item 7 deliberately scoped out.
