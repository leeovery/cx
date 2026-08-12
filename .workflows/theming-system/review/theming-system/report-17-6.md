TASK: theming-system-17-6 — Derive The Capture Panel Fixtures' Unions From Their Declared Entries

ACCEPTANCE CRITERIA:
- No `theme.Union` composite literal remains in `internal/capture`.
- Every panel fixture's `Count` and `Rejected` are computed, not typed.
- Rendering each `theme-panel-*` fixture through `capturetool` produces the same frame as before (row order, badges, chrome).
- A fixture that declares no panel still yields a nil theme source, so `t` remains a no-op there.
- The swap-and-diff completeness guard stays green and enumerates the same fixture set.

STATUS: complete

SPEC CONTEXT:
Spec §13.3 makes `internal/capture` the only pre-release visual-verification route and fixes a panel fixture's four inputs (`--theme` palette, raw persisted keys, the faked enumerator's row set, cursor position); it also pins that `internal/theme` owns union assembly so `count`/`rejected` are computable where emitted and the panel does no merging of its own. §9.4/§1091 fix the `rowBefore` fold→bytes→built-in-first order and state that deterministic ordering is what makes the panel fixtures reproducible. §13.4's swap-and-diff guard enumerates whatever fixtures exist and diffs colours, so it is structurally blind to a drifted row set — which is exactly the exposure this task closes. Deriving the union from declared entries keeps the third input declarative (entries rather than rows) without changing what the seam returns, so §13.3's seam contract is untouched.

IMPLEMENTATION:
- Status: Implemented (matches all seven "Do" steps)
- Location:
  - `internal/capture/fixtures.go:418-423` — `themePanelUnionFrom(e, keys)` = `theme.Assembler{Loader: theme.NewSilentLoader()}.Reassemble(e, keys)`. `Reassemble` (`internal/theme/union.go:131-138`) touches no filesystem — file rows are copied from `Entry` verbatim and only `LoadBuiltin` runs, off the `go:embed` set — and `theme: enumerated` is emitted from `Open` alone (`union.go:121`), so the "no directory read, no enumerated event" constraint holds. `NewSilentLoader` also still reserves built-in slugs, so verdicts match production.
  - `internal/capture/fixtures.go:507-513` — `themePanelRejectedEntry` moves each candidate's verdict onto its `theme.Entry`; the `bad name` candidate keeps the `Slug: ""` shape (`:535-536`).
  - `themePanelUnion`, `themePanelInvalidRowUnion`, `themePanelDirUnreadableUnion`, `themePanelPaginatedUnion` are all deleted (no references remain repo-wide).
  - Derivation is assigned after the enumeration and keys in each panel fixture: `:437`, `:457`, `:542`, `:567`, `:600`, `:632`. Fixtures that only re-skin an ancestor (`confirm`, `commit-failed`, `min-height-message`, `narrow`) correctly inherit the derived union unchanged.
  - Derivation stays inside the panel fixtures; `Fixture.themeSource` (`:109-116`) keeps its `len(f.themeUnion.Rows) == 0` gate untouched, so a fixture declaring no panel still gets a nil source.
  - `newFakeThemeSource`'s `repaintUnion` (`internal/capture/theme_fake.go:57-67`) is untouched, so `--theme` stays live and rejected rows keep their zero palette.
  - The backwards rationale at the old `:550` is gone; the corrected one sits at `:418-420` and `:507-509`.
- Frame-equivalence (AC 3) reasoning, since I do not execute anything: three built-ins ship (`nord`, `tokyo-night`, `tokyo-night-day`), so each derived union reproduces the deleted literal exactly —
  - adaptive-pair/constant-previewing: 3 built-ins + the `catppuccin-latte` file row, both keys already listed ⇒ 4 rows, `Rejected 0`, order `catppuccin-latte, nord, tokyo-night, tokyo-night-day` (identical to the deleted literal);
  - invalid-row: 3 built-ins + 3 rejected file rows, `nord-lee` already listed as a file row ⇒ 6 rows, `Rejected 3`, order `aurora-glow, My Gorgeous Midnight Palette.theme, nord, nord-lee, tokyo-night, tokyo-night-day`;
  - dir-unreadable: 3 built-ins + 2 persisted rows minted with `ReasonUnreadable` by `unresolvedRejection` ⇒ 5 rows, `Rejected 2`, `DirUnusable true`;
  - paginated: 3 built-ins + 31 file rows ⇒ 34 rows, matching the old 4 + 30.
  The existing render suite independently pins these frames: `theme_panel_fixture_render_test.go:79-137` (badges, cursor row, unbadged rows), `theme_panel_remaining_fixtures_test.go:66-174` (rendered rejection reasons, page-1 `nord-lee` vs page-2 `solarized-lee`/`tokyo-night`, the persisted `● light` badge) and `:209-233` (pagination dots) would fail on any membership or ordering change.
- Notes: two `theme.Union` composite literals survive in `internal/capture`, both in `theme_panel_fixture_test.go` (`:204` a zero value, `:287-290` a deliberately minimal two-row union with one rejected row). Neither is fixture data — they drive `newFakeThemeSource`'s repaint contract (`TestFakeThemeSource_RowsCarryTheInjectedPalette`), which needs a controlled valid/rejected pair rather than a fixture's derived list. AC 1's stated harm (a hand-maintained copy of membership/dedup/ordering inside the fixture set) is fully removed, so I read this as in-intent rather than a miss.
- The repeated `fx.themeUnion = themePanelUnionFrom(fx.themeEnumeration, fx.themeKeys)` line carries an implicit ordering requirement (must follow both assignments). That hazard is closed by `TestPanelFixture_UnionIsProductionAssembled`, which re-derives independently and `reflect.DeepEqual`s — a fixture that mutated keys after deriving would fail. Good structural choice, not a finding.

TESTS:
- Status: Adequate
- Coverage:
  - `theme_panel_fixture_test.go:97-113` `TestPanelFixture_UnionIsProductionAssembled` — the requested pin; re-derives via `theme.Assembler{Loader: theme.NewSilentLoader()}.Reassemble` (deliberately not via `themePanelUnionFrom`, so the assertion is independent of the helper) and compares the whole `Union`, so `Count`, `Rejected`, `DirUnusable`, row order and per-row rejections are all covered. A hand-authored union creeping back fails here.
  - `:36-84` retargeted `TestPanelFixture_FourInputs` — the vacuous "declared union comes back" check is gone; it now asserts the declared cursor row exists in the assembled union *and* is selectable (the arrow-skip invariant), and that every declared slot's `Requested` has a row to carry its badge. Both are real coherence properties the old assertion could not express.
  - `:115-140` invalid-row rejections and reasons + `Rejected == 3`; `:142-161` dir-unreadable `DirUnusable`, `SourcePersisted` and `ReasonUnreadable` on both persisted rows; `:163-178` the paginated fixture's last synthetic row is pushed off the frame at the harness drive size.
  - `TestPanelFixture_NoConfigAccess` (`theme_panel_fixture_render_test.go:70-77`) renders every panel fixture with `PORTAL_THEMES_DIR`/`XDG_CONFIG_HOME` re-pointed at a decoy tree and asserts the decoy never appears — behavioural confirmation that the new derivation reads no directory.
  - `TestPanelPaginatedEntries_DeriveFromBase` (`:590-619`) was correctly re-pointed from rows to entries, and its now-meaningless "count re-derived from rows" / "base row set is fresh per call" subtests were dropped rather than left asserting a derived value.
  - Registry/guard coverage unchanged: no fixture added or removed (`fixtureBuilders` untouched), and `TestPanelFixture_RegistryHoldsTheSpecifiedPanelSet` still pins the exact panel set.
- Notes:
  - AC 4 ("a fixture that declares no panel still yields a nil theme source") holds in code but has no assertion anywhere in the package — the only nil check (`:61-63`) runs over panel fixtures and asserts non-nil. Nothing would fail if derivation later moved into `Deps`/`themeSource`, which is the exact regression the task's Do-4 warns about. See the note below.
  - Mild layer overlap between `TestPanelFixture_PaginatedOverflowsOnePage` (union + 120×40 drive) and the existing `TestPanelFixture_PaginatedDrawsDots` (harness size, dot glyphs); the two run at different sizes and assert different observables, so I would not call it over-testing.
  - `reflect.DeepEqual` matches established repo usage (`internal/theme/builtins_test.go`, `internal/state/schema_test.go`, several `internal/tui` tests); `Union` holds only strings, ints and `*Rejection`, so the comparison is well-defined.

CODE QUALITY:
- Project conventions: Followed. Test-only helpers stay in `_test.go`; no `t.Parallel()`; unit lane only (no tmux, no daemon, no built binary); the fake keeps its no-I/O contract and the `theme_fake.go` loader-field guard is unaffected (the assembler lives in `fixtures.go`, not on the fake).
- SOLID principles: Good. Single derivation point, production rules single-sourced in `internal/theme`; the fixture layer now declares data only.
- Complexity: Low. Two small helpers replace four hand-maintained constructors; net −110/+? in `fixtures.go` with no new branching.
- Modern idioms: Yes (`slices.Clone`, range-over-int in `themePanelPaginatedEntries`).
- Readability: Good. Every remaining comment checked against the code it describes and holds: `themePanelUnionFrom`'s "Reassemble, never Open" claim is true of `union.go:117-138`; `themePanelDirEntry`'s "no palette — the fake repaints" matches `repaintUnion`; `themePanelRejectedEntry`'s "an empty slug is the `bad name` shape" matches the `My Gorgeous Midnight Palette` entry; the `Fixture.themeKeys`/`themeUnion` field comment now correctly says the union is assembled from the enumeration and keys.
- Issues: one stale test label (below). No security or performance concerns — derivation runs per fixture construction against the embedded set, off any hot path.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/capture/theme_panel_fixture_test.go:48 — the `input 3a` assertion was deleted with the hand-written unions, leaving `input 3b` as a dangling sub-letter with no sibling. Replace the message with `"input 3: the fixture declares no slot resolutions, so no row carries a ● at all"`.
- [quickfix] internal/capture/theme_panel_fixture_test.go:36-84 — AC 4 is unasserted: add a case (e.g. alongside `TestPanelFixture_FourInputs`) that a non-panel fixture such as `sessions-flat` yields `fx.Deps(themetest.Builtin(t, "nord")).ThemeSource == nil`, so moving the derivation into `Deps`/`themeSource` — which would hand every fixture a slide-over — fails loudly instead of silently.
