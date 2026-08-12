TASK: theming-system-8-15 — Panel Fixture Inputs And The Two Setting-State Frames

ACCEPTANCE CRITERIA (from the plan task):
- A fixture can declare all four inputs and render the panel with no real themes directory and no prefs file.
- The fake enumerator's `Resolve` returns the fixture's declared `themeSlots`, and a fixture that declares none renders no badge on any row — asserted directly.
- `theme-panel-adaptive-pair` renders two badge rows — `● light` on `tokyo-night-day`, `● dark` on `nord` — cursor on `nord`.
- `theme-panel-constant-previewing` renders a single bare `●` on `nord` with the cursor on a *different* row, and no slot badges anywhere.
- The seeded cursor changes the highlighted row only — palette is `--theme`'s (asserted by rendering one fixture under two `--theme` values and diffing).
- The fake's `Resolve` reports the injected palette so the open-time `ApplyTheme` is a no-op; a zero `Resolution` renders colourless and fails.
- Both fixtures appear in `FixtureByName` **and** `FixtureNames()`; the registry drift check passes.
- Both fixtures are picked up by §13.4's swap-and-diff guard with no test edit (the guard enumerates).
- Inside that guard the panel actually opens: A-frame contains the panel's left border and its `Themes` header.
- The fake performs no file or directory access on any method.
- `internal/capture`'s no-real-config guard and `TestPortalBinaryDoesNotImportCapture` both pass.
- A `.tape` exists per fixture; each captured PNG verified as a fresh write.
- Each `.tape` types its fixture's declared `captureKeys` before `Screenshot`; `capturetool` replays no keys.
- Each fixture's doc comment states its coherence pairing (`--theme` ↔ cursor row).

STATUS: complete

SPEC CONTEXT:
Spec §13.3 (lines 1637-1645) requires a panel fixture to declare four inputs — the `--theme` palette, raw persisted theme keys, the faked enumerator's row set, and the cursor position — with the raw keys declared independently of `--theme` (because `capturetool` always passes the constant nomination shape, and §8.2 makes a non-empty `theme` render a bare `●`, so a nomination-derived fixture could only ever reach one state). §13.3 also states the coherence rule (`--theme` must name the theme under the cursor; adaptive-pair resolves to the dark slot at open because `capturetool` runs no gate; constant-previewing names the *previewed* theme, not the marked constant) and notes it is an authoring rule the harness cannot check. §13.4 requires the swap-and-diff guard to *enumerate* rather than name fixtures, so a missing fixture is a blind spot that reads as coverage. §9.14 makes the two Paper artboards + these fixtures the only reference that exists for the slot half. §13.3 also mandates procedural fresh-write verification of every capture (VHS fails silently on write).

Amended-mechanism note (per the shared verifier inputs): later phases renamed the seam `ThemeEnumerator` → `ThemeSource` (task 13-10), made it uniform in what it consumes (15-9), narrowed the slot seam to `LoadSlot` (17-4), and a `chore(comments)` commit (d939ae76) deliberately compressed the harness comments to the project's code-quality standard. The fake is therefore `fakeThemeSource` with four methods, and its doc comment states the invariant without the task's full three-failure narrative. Both are intentional supersessions, not drift; the task's named tests are present under correspondingly renamed identifiers.

IMPLEMENTATION:
- Status: Implemented (as amended by later phases)
- Location:
  - `internal/capture/fixtures.go:36-45` — the four declared inputs (`themeKeys`, `themeUnion` + `themeEnumeration` + `themeSlots`, `initialThemeCursor`).
  - `internal/capture/fixtures.go:70-116` — `Deps` threads the palette to the two seams that must agree (`theme.ConstantNomination(th)` and `f.themeSource(th)`), wires `ThemeKeys` and `Capture.ThemeCursor`; `themeSource` returns a true nil (not a typed nil) when no panel is declared, so `t` is a silent no-op.
  - `internal/capture/theme_fake.go:1-76` — `fakeThemeSource` (compile-time asserted against `tui.ThemeSource` at :20), answering from declared values, repainting every selectable row and every `SlotResolution.Theme` onto the injected palette and returning a `ConstantNomination` of it, so the open-time `ApplyTheme` is a no-op. Rejected rows deliberately keep their zero `Theme` (a half-populated rejected row is a shape the real assembly cannot produce).
  - `internal/capture/fixtures.go:421-427` — `themePanelUnionFrom` derives the union through production `theme.Assembler.Reassemble` with the silent loader (embedded built-ins only; no directory read).
  - `internal/capture/fixtures.go:429-447` — `themePanelAdaptivePairFixture` (keys `{Light: tokyo-night-day, Dark: nord}`, three built-ins + one drop-in, two slot records, cursor `nord`, `captureKeys` = `t`).
  - `internal/capture/fixtures.go:449-466` — `themePanelConstantPreviewingFixture` (keys `{Theme: nord}`, exactly one `SlotConstant` record, cursor `tokyo-night-day`, `captureKeys` = `t`).
  - `internal/capture/fixtures.go:149-150` — both registered in the single `fixtureBuilders()` source that `FixtureByName` and `FixtureNames()` both derive from, so the two lookups cannot disagree by construction.
  - `internal/tui/theme_panel.go:139-153` — `armThemePanel` anchors the open's row then re-anchors on `themeState.initialCursor` by row identity, placement only.
  - `cmd/capturetool/main.go:168-185` — the resolved `--theme` palette is handed to `fx.Deps(pinned)` (the one site), and the tool replays no keys of its own.
  - `testdata/vhs/theme-panel-adaptive-pair.{tape,png}` and `testdata/vhs/theme-panel-constant-previewing.{tape,png}` — each tape types the declared `t` before `Screenshot` (`:92-95` / `:95-98`), each carries the coherence pairing and the fresh-write warning.
- Notes:
  - Coherence verified end-to-end against the captured images: the adaptive-pair PNG renders in nord's palette (canvas `#2E3440`, `accent.primary #B48EAD`) with `● dark` on `nord` (cursor there) and `● light` on `tokyo-night-day`; the constant-previewing PNG renders in tokyo-night-day with a single bare `●` on `nord` and the cursor bar on `tokyo-night-day`. Both frames show the panel — no panel-less capture.
  - No config reach: the fake holds no loader and does no I/O; `themePanelUnionFrom` uses `LoadBuiltin` (embed) and the declared entries; `themesDirPath` is a shared literal that is never resolved, opened or stat'ed (`fixtures.go:498`).
  - Scope-adjacent work delivered by later tasks (invalid-row, dir-unreadable, narrow, paginated, projects, confirm, commit-failed, min-height-message fixtures) reuses this task's four inputs unchanged — the input set generalised correctly.

TESTS:
- Status: Adequate
- Coverage (task's named tests, mapped to current identifiers):
  - `TestPanelFixture_FourInputs` (`theme_panel_fixture_test.go:36`) — runs over *every* registered panel fixture; asserts each declares keys/slots/cursor, that `Deps` forwards them, that the cursor row exists in the union and is selectable, and that each declared slot's `Requested` names a listed row (so a badge always has a row to sit on).
  - `TestPanelFixture_AdaptivePairBadges` (`theme_panel_fixture_render_test.go:79`) — `● light` / `● dark` placement, cursor on `nord` and *only* on `nord`, unassigned rows unbadged.
  - `TestPanelFixture_ConstantWhilePreviewing` (:112) — bare `●` on `nord`, no slot word anywhere, cursor on a different row and not on the marked one.
  - `TestPanelFixture_CursorSeedDoesNotApplyATheme` (:139) — cursor on the seeded row while the frame carries `--theme`'s canvas and *not* the seeded row's canvas.
  - `TestPanelFixture_PaletteFollowsTheThemeFlag` (:156) — renders both fixtures under two palettes and diffs; asserts each frame carries its own canvas SGR, lacks the other's, and that the row set does not follow the flag.
  - `TestFakeThemeSource_ResolveReportsTheInjectedPalette` (`theme_panel_fixture_test.go:198`) — declared slot identity preserved, every slot's `Theme` and the nomination report the injected palette, plus two hazard-driving sub-tests: a fake reporting another palette *does* repaint the frame (proving the assertion is not vacuous) and a fake reporting a zero palette renders with no 24-bit background at all.
  - `TestFakeThemeSource_ResolveIsTheOnlyBadgeSource` (:323) — declaring no slots renders no badge on any row.
  - `TestFakeThemeSource_NoIO` (:377) — AST scan of `theme_fake.go` for filesystem imports and for any `theme.Loader` field, a control source proving the loader scan is not vacuous, and a runtime pass over all four methods with `PORTAL_THEMES_DIR` / `XDG_CONFIG_HOME` / `PORTAL_PREFS_FILE` poisoned, asserting the poisoned dir is never created.
  - `TestPanelFixture_NoConfigAccess` (`theme_panel_fixture_render_test.go:70`) — stages a decoy themes dir + drop-in for every panel fixture and asserts the decoy never appears and nothing is written.
  - `TestPanelFixture_Registered` (:214) — resolvable by name, present in `FixtureNames()`, and present in the *guarded* set (so enumeration coverage is asserted, not assumed); `TestFixtureRegistry_*` covers the drift check.
  - `TestPanelFixture_PanelIsCompositedUnderTheGuard` (:235) — drives `RenderSwapRender` with the synthetic palettes at the guard's pinned size and asserts the A-frame carries the panel's left border and `Themes`, and carries neither entry-refusal flash.
  - `TestFakeThemeSource_RowsCarryTheInjectedPalette` (:257) — one `↓` still paints `--theme`'s canvas (the arrow-preview seam), and a rejected row keeps its rejection and takes no palette.
  - `TestModelAt_ReachesCapturedState` (`swap_harness_test.go:51-52`) — pins the two frames' captured content (`Themes`, `● light`/`● dark`; and for the constant frame `●` present with `● light`/`● dark` absent) at the guard's pinned size.
  - Registry/guard integration: `TestThemeSwapGuard_EnumeratesRegistry` proves the guard covers every enumerated name with the swatch as the only skip.
- Notes:
  - Would fail if the feature broke: yes at every criterion — a lost cursor seed, a badge source regression, a repainting fake, an unregistered fixture, or a pinned size below the panel floor each have a named failing assertion with an explicit message.
  - Not over-tested in substance: each assertion is anchored to a distinct stated hazard (silent colourlessness, absence-reads-as-coverage, incoherent palette). The two "control" sub-tests (loader-scan control, other-palette repaint) are anti-vacuity checks rather than redundancy.
  - One vacuity gap: the negative badge assertions index `panelRows` by literal slug, and a map miss yields the zero `panelRow` whose badge is `""` — so an absent or truncated row would pass those sub-tests silently (see non-blocking notes). The positive assertions in the same tests do catch a missing row.
  - Test-execution not attempted (verification by reading, per instructions).

CODE QUALITY:
- Project conventions: Followed. Test-only fakes stay in `internal/capture` (never production); the harness reaches no config, honouring §7.1's guard (`cmd/capturetool/import_guard_test.go` unchanged and still non-vacuous); no raw hex at any call site; comments carry no task/phase/spec citations in production files (the tapes do, but they are scaffelding artifacts cleared at sign-off and every tape in the tree follows that idiom); no `t.Parallel()`.
- SOLID principles: Good. The fake is a narrow seam implementation with a compile-time interface assertion; fixture data is declarative and the union is derived through the *production* assembler, so membership/dedup/order has one implementation; `Deps` is the single wiring point for both palette-carrying seams.
- Complexity: Low. Every added function is short and branch-free bar one nil guard; the repaint helpers copy before mutating, so a fixture's declared union is never aliased or mutated.
- Modern idioms: Yes (`slices`, `strings.SplitSeq`, range-over-int in the synthetic entry builder, value receivers on the immutable fake).
- Readability: Good. Each fixture's doc comment states its `--theme` pairing and why the pairing matters; `themeSource`'s nil-return rationale and the `initialThemeCursor` "placement only" note are both stated where they are read.
- Comment accuracy: Verified against the code — `theme_fake.go:8-10` (injected palette + no I/O), `:35-38` (recompute inputs ignored), `:40-41` (constant nomination keeps the apply a no-op), `:56-57` (rejected rows untouched, matching the `Selectable()` branch), `fixtures.go:36-45`, `:68-69`, `:109-110`, `:429-431`, `:449-451` all hold. No stale or self-contradicting comment found.
- Security / performance: N/A — offline, test-only render path with no I/O, no loops of concern.
- Issues: none blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/capture/theme_panel_fixture_render_test.go:103-108 and :121-127 — the negative badge sub-tests read `rows[label]` from the `panelRows` map, so a row that is absent (or rendered truncated with an ellipsis) yields the zero `panelRow` and the assertion passes vacuously. Route both loops through the existing `panelSlugRow(t, frame, slug)` helper (`panel_frame_test.go:115`), which fails when the row is missing and already tolerates truncation.
- [do-now] internal/capture/swap_harness_test.go:19-22 — the guard's single pinned render size carries no comment recording why the value is what it is; this task requires the panel's entry floor to be one of the recorded reasons. Add above the const block: `// The single pinned size every fixture is driven at. 120×40 clears the theme` / `// panel's entry floor — minimum width, and header + footer + one list row +` / `// one message row of height — so a panel fixture opens its slide-over here` / `// instead of rendering a panel-less frame with a blocked-entry flash, which` / `// every guard assertion would still pass. Raise it, never lower it.`
- [quickfix] internal/capture/theme_panel_fixture_test.go:558 — `panelModel` hardcodes `Width: 120, Height: 40`, duplicating the external test package's `harnessWidth`/`harnessHeight` without naming them (the internal/external test package split blocks sharing the constant). Declare a named pair in the internal test package and use it, so the two drives are visibly the same size.
- [quickfix] internal/capture/fixtures.go — the file is 760 lines holding every fixture family; the theme-panel builders and their helpers (lines ~414-637) are a self-contained ~225-line block. Extract them into `internal/capture/theme_panel_fixtures.go` (pure move, no behaviour change) so the panel catalogue is readable on its own.
