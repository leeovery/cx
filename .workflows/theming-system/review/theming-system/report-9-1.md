TASK: theming-system-9-1 — Enter Commits A Constant Through The Persister

ACCEPTANCE CRITERIA:
1. `Enter` on a valid row calls `CommitTheme` with **that row's** slug (asserted against a recording fake), even when a different slug is persisted.
2. The panel is still open after `Enter`, on both pages.
3. `m.themeKeys` becomes `{Theme: slug, Light: "", Dark: ""}` after a successful commit.
4. `Model.ApplyTheme` is not called and the composed frame is byte-identical across the keypress.
5. `Esc` after a commit resolves the newly persisted state through the close path.
6. A failed commit leaves the keys untouched (the `●` cannot move) and returns the error.
7. A nil persister writes nothing, mutates nothing, does not panic and raises no failure state.
8. Committing the same slug twice is idempotent.
9. `Enter` writes nothing beyond the prefs call: no directory read, no prefs read, no enumeration, no tmux server option, no file.
10. `Enter` on a non-selectable row writes nothing.
11. `Enter` raises no confirm under any setting shape, including an adaptive pair.
12. `TestApplyTheme_PerformsNoFileRead`'s counting set includes a `countingThemePersister` (two methods, one counter) in `countingStores`/`reset`/`calls`/`exercise` (positive control 6 → 7) and wired into its `Build(Deps{…})`.

STATUS: complete

SPEC CONTEXT:
§9.2 (`.workflows/theming-system/specification/theming-system/specification.md:986-1031`) fixes the panel's key table: `Enter` "Commits a constant — writes `theme = <selection>`, clears both slots | **stays open**" (line 992), with the rationale pinned at line 1014 ("`Enter` does not close… `Esc` is the only way out — one exit key, no dual-purpose keys") and the accepted cost at 1016. Line 1029 pins "a commit is a **write, not a navigation** — the panel keeps previewing whatever the cursor is on; the display resolves from persisted state only on close", and line 1031 sharpens `Esc` to "the resolved persisted state", which after a commit is the newly persisted one. Line 1023 requires the recompute to use "the construction-time snapshot plus this instance's own mutation — never the merged bytes the RMW just read". §9.2:1045 states the reverse direction (a constant over a pair) needs **no** confirm; §8.2's mutual exclusion is performed on disk by `prefs.Store.SaveTheme` (`internal/prefs/store.go:220-227`, which clears both slots in the same `mutate`); §9.13:1251 makes a commit unconditionally re-attemptable with no retry affordance.

Note on supersession: §9.2:1018-1027 (delivered by task 9-2) makes a *landed* commit recompute the full row set, so in production the `●` does move on a successful `Enter`. AC 4's "frame byte-identical" is therefore the 9-1-era intent as amended — the surviving assertion is scoped to "no re-theme" (see TESTS below).

IMPLEMENTATION:
- Status: Implemented (mechanism intentionally reshaped by later phases: 9-2 added the post-commit recompute, 9-7 the failure report, 17-2 extracted the shared commit protocol, 12-9 swapped `prefs.ThemeSlot` for `theme.Member` at the seam, 11-15 renamed `m.themeKeys` → `m.themeState.keys`).
- Location:
  - `internal/tui/theme_panel.go:325-329` — the `Enter` arm in `updateThemePanel`, ahead of the swallow-everything `default` and behind the `confirming()` / `Esc` arms, so a pending confirm still swallows it (§9.2:1040).
  - `internal/tui/theme_panel_commit.go:11-17` — `commitSelected` takes the target from `p.list.SelectedItem()`, never from `m.themeState.keys`.
  - `internal/tui/theme_panel_commit.go:19-25` — `committableThemeSlug` refuses a non-`Selectable()` row or an empty `Slug`, returning "no write".
  - `internal/tui/theme_panel_commit.go:31-50` — `commit`/`commitConstant`: nil-persister short-circuit before any write, `CommitTheme(slug)` through the seam, mirror on success via `keys.WithConstant(slug)` (`internal/theme/setting.go:33-35`, which returns `RawKeys{Theme: slug}` — both slots cleared), error path mutates nothing and returns.
  - `internal/tui/model.go:76-79` — `ThemePersister` seam; `internal/tui/model.go:1651-1655` — panel key-exclusivity, so `Enter` never reaches the page beneath on either page.
  - `cmd/theme_persister.go:30-32` — the production writer (`prefs.Store.SaveTheme`, logs `theme: commit failed` and returns the error).
- Notes:
  - The in-memory mirror is applied to `m.themeState.keys` — the construction-time snapshot documented at `internal/tui/theme_state.go:38-41` ("A construction-time snapshot, never refreshed") — and never re-derived from the persister's merged bytes, matching §9.2:1023. `recomputeThemePanel` (`theme_panel_commit.go:97-107`) is likewise fed `m.themePanel.enumeration` + `m.themeState.keys`, not a fresh read.
  - No `ApplyTheme` anywhere on the commit path; `applyCommittedSetting` updates badges and `themeState.nomination` only (`theme_panel_commit.go:120-127`).
  - Nil persister is treated as absence-of-writer, not failure: it returns before `write()`, so no message and no `commitFailed` state (`theme_panel_commit.go:31-34`).
  - Guard-rationale comment the task asked for on the unselectable branch is absent from source but present on the test (`theme_panel_commit_test.go:369-370`); later comment-audit commits (11-3, 12-7, `915e7fcb`) deliberately pruned production comments to the standard, so this reads as an owned later decision, not a miss.

TESTS:
- Status: Adequate
- Coverage: All eleven named tests exist in `internal/tui/theme_panel_commit_test.go` with the planned names — `TestPanelEnter_CommitsTheCursorSlug` (120), `_DoesNotClose` (136, table over `entryPages` = Sessions + Projects), `_MutatesRawKeysToAConstant` (158, over an adaptive-pair fixture), `_IsAWriteNotANavigation` (174), `_EscResolvesTheCommittedTheme` (240), `_FailedWriteLeavesKeysAlone` (267), `_NilPersisterIsInert` (299), `_RepeatCommitIsIdempotent` (332), `_NoOtherIO` (360), `_UnselectableRowWritesNothing` (371), `_NoConfirmOverAPair` (418). Mapping to criteria: 1→120 (with a fixture precondition that persisted ≠ target, so the assertion cannot be vacuous), 2→136, 3→`requireConstantKeys` (98-103, exact struct compare so Light/Dark emptiness is asserted), 4→174, 5→240, 6→267, 7→299, 8→332, 9→360, 10→371, 11→418, 12→`internal/tui/apply_theme_test.go:103-136,178-185,195-205,214-227` (counter added to the struct, `calls`, `reset`, `exercise`, positive control now 7, and `ThemePersister: stores.themePersister` wired into `Build`).
- Notes:
  - Strong negative controls throughout: `_IsAWriteNotANavigation` pairs the byte comparison with "an arrow over the same fixture does change the frame" (198) and an AST scan that also asserts it *would* find the applies in `theme_panel.go` (214-218), so neither the comparison nor the scan can pass by having stopped looking. `_NilPersisterIsInert` ends with a wired-persister positive control (326-329).
  - AC 5 is the one test that deliberately refuses the stub seam and drives a real loader over a real themes directory (240-265), which is what makes "Esc resolves the *newly* persisted state" mean something.
  - AC 9 is delegated to the shared `requireCommitDoesNoOtherIO` (`theme_testing_test.go:75-144`): counts every file-touching seam, pins the enumeration count to the open's, asserts a nil `tea.Cmd` (the one deferred-write shape counters cannot see), and asserts both the config dir and the themes dir are untouched on disk.
  - Not over-tested overall. `TestCommitProtocol_*` (`theme_panel_commit_protocol_test.go`) overlaps the `Enter`-level failure/nil cases, but at a different layer (the shared constant/slot protocol including reassembly counts and outstanding-failure state) and it is what keeps the two commit shapes from drifting after 17-2's extraction — the residual duplication is small and load-bearing.
  - Layer caveat (see note 1 below): the byte-identity assertion at 193-195 holds because `newArrowPanelDeps` pins the fake source's resolution (`theme_panel_arrow_test.go:82-86`), so badges cannot move in that fixture. It therefore isolates the "no re-theme" property rather than proving a commit never changes the frame — production badge movement is owned by `TestPanelRecompute_VirginInstallBadgeCollapse`. Correct division of labour, but undocumented at the assertion.
  - Lane compliance: unit-lane, no daemon/binary/tmux, no `t.Parallel()`.

CODE QUALITY:
- Project conventions: Followed. Seam-based DI with a 2-method interface, nil-guarded like the `modePersister` precedent; no `log.For("theme")` binding in `internal/tui` (guarded structurally by `TestCommitFailed_SingleEmissionSite`, `theme_persister_seam_test.go:105-125`), so the closed log-component rule holds; the `cmd` package keeps the single emission site.
- SOLID principles: Good. `commit` isolates the one commit protocol (nil-guard → write → report → mirror → recompute) and is parameterised by the write and the mirror, so `commitConstant`/`commitSlot` differ only in those two functions — no duplicated protocol to drift.
- Complexity: Low. Every function in `theme_panel_commit.go` is under ten lines with a single branch.
- Modern idioms: Yes. Value-returning key transforms (`RawKeys.WithConstant`) rather than in-place mutation; explicit `_ =` on the deliberately discarded error at the dispatch site.
- Readability: Good. Comments explain the non-obvious *why* (why the selected row and not the persisted keys; why nil is inert rather than failed; why the mirror rather than a read-back; why nothing moves on error) and hold true against the code, with no process-artifact references.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_panel_commit_test.go:175 — the subtest name "the frame is byte-identical across the keypress" overstates production behaviour: a landed commit does move the `●`, and the identity only holds because the fake source's resolution is fixed. Add above line 175: `// The fake's resolution is fixed, so badges cannot move here and the comparison isolates one thing: the commit does not re-theme. A landed commit DOES move the ● — TestPanelRecompute_VirginInstallBadgeCollapse owns that.`
