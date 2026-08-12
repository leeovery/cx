TASK: theming-system-8-8 — Opening Lands The Cursor On The Theme Actually Rendering

ACCEPTANCE CRITERIA:
1. Under a constant, the cursor lands on the constant's row.
2. Under a pair, the cursor lands on the in-force slot's row while the other slot's row keeps its badge.
3. Both slots on one slug ⇒ cursor on the single `● both` row.
4. Fallback ⇒ cursor on the fallback's row; persisted row present, unselectable, reasoned; `●` still on the persisted slug.
5. Unchanged directory ⇒ rendered theme byte-identical (ApplyTheme not called, or called with the active theme).
6. Edited-but-valid active theme re-renders with new values on open, no arrowing.
7. Edit that invalidates the active theme flips to the §8.5 fallback on open.
8. Repaired theme applies on open (mirror case), cursor moves onto the now-selectable persisted row, no relaunch.
9. `Resolve` performs no directory read and writes nothing to prefs.json.
10. Cursor anchored by identity, not index.
11. `Deps.ThemeSlots` / `WithThemeSlots` no longer exist; every `●` derives from a `Resolve` result.
12. Missing identity ⇒ clamp to the first selectable row, no panic, no out-of-range.
13. A `Resolve` fatal degrades (badges/active theme/cursor unchanged, panel still opens, nothing written, no quit).
14. After every open the cursor's row is selectable and equals the applied theme.
15. In-force answer comes from the gate's single resolution — no OSC 11 query on open.

STATUS: complete

SPEC CONTEXT:
§9.2 ("Opening state: the cursor lands on the theme that is actually rendering, and opening previews nothing") specifies the four opening cases, the "`●` is what is set, the cursor is what is previewed" split, the edited-and-still-valid / edited-and-now-invalid pair with the flip landing on **open** (never deferred to `Esc`), the surviving invariant ("the cursor is always on a selectable row, and that row is always what is painted behind the panel"), and identity-not-index anchoring for the commit recompute. §5.8 pins the mirror case (fixing a previously-invalid theme takes effect on the next open, no relaunch). §8.4 forbids a commit-time/open-time directory read because it would produce a third parse free to disagree with the row on screen. §16's log table expects a panel open to emit `theme: fallback applied` (deduped) and no `theme: loaded`.

IMPLEMENTATION:
- Status: Implemented (seam signature intentionally superseded by a later phase — see Notes)
- Location:
  - `internal/theme/resolution.go:69-76` — `Loader.ResolveNominationFrom(e Enumeration, s Setting) (Resolution, error)`, beside `ResolveNomination` (`:65`). Shared body factored through `resolveNomination(s, pass)` (`:141`) + the `resolutionPass` type (`:89-114`), so the by-name and by-enumeration entry points cannot drift; `ResolveByNameFrom` (`:119`) reuses `resolveNamed` (`resolve.go:27`) for the identical charset → embedded-set-first → source ladder. Per-slot mode-matched fallback (`:170-216`), fatal only when the *fallback* fails (`:179`), writes nothing.
  - `internal/theme/resolution.go:104-106` — `enumerationPass` pairs the retained-parse load with `reportFallback`, so a panel open emits the fallback WARN and never `theme: loaded` (the type makes the wrong pairing unrepresentable at the call site).
  - `internal/tui/theme_seams.go:11-16` — `ThemeSource.Resolve(e theme.Enumeration, keys theme.RawKeys) (theme.Resolution, error)`.
  - `internal/theme/dir_theme_source.go:22-29` — production adapter; resolves `Setting` from the raw keys and delegates to `ResolveNominationFrom`, never touching `Dir`.
  - `internal/tui/theme_panel.go:139-236` — `armThemePanel` (ordering documented), `applyInForceTheme` (degrade-on-error, `ApplyTheme` only when the palette differs), `applyThemePanelResolution` (badges from `theme.Badges(resolution.Slots)`, returns the resolved slug), `inForceSlot` (constant-or-matching-slot, false when no slot names), `anchorThemePanelCursor` + `themePanelRowIndex` (identity + selectable match, clamp to first selectable, then `max(..., 0)`).
  - `internal/tui/theme_panel.go:242` — the close reuses `applyInForceTheme`, so open and close share one policy and one resolution route.
  - Retirement: `Deps` (`internal/tui/build.go:16-63`) carries `ThemeKeys` and no `ThemeSlots`; no `WithThemeSlots` anywhere in the tree; `cmd/open.go:534` passes only `ThemeSource` + `ThemeKeys`; fixtures declare slots through the faked seam's `Resolve` (`internal/capture/theme_fake.go:42-47`).
- Notes:
  - The seam takes `theme.RawKeys` rather than the planned `theme.Setting`. This is a later-phase revision, documented on the interface ("the tiebreak and shipped-default substitution belong to internal/theme, so this package never emits the `theme` log component") and applied uniformly across all four seam methods. Outcome-equivalent and better-layered — not drift.
  - The in-force member comes from `m.themeState.inForceMode()` → `themeState.canvasMode`, established once via `adoptGateAnswer`/`adoptRetainedReply`; the open path never re-runs detection and never touches the gate.
  - Nomination is deliberately not refreshed on open (only the commit recompute writes `themeState.nomination`). Verified inert: post-construction readers are `hasNomination`/`syncResolvedMode` only, and the gate resolves exactly once (`appearanceGate.resolve` short-circuits), so a stale nomination cannot repaint over the open's applied theme.
  - The plan asked the new entry point's comment to name Phase 9 / task 8-10 as reusers. The implementation states the same rule without process-artifact references, which is the correct call under the repo's comment convention — not a gap.

TESTS:
- Status: Adequate
- Coverage:
  - `internal/theme/resolution_test.go:786-822` `TestResolveNominationFrom_ReadsNothing` — resolves a drop-in after `os.RemoveAll` of its directory, plus a call-graph guard asserting zero `os.*` call sites reachable from `ResolveNominationFrom` (with a non-vacuity check against `ResolveNomination`'s `os.ReadFile`). Criterion 9 (read half).
  - `resolution_test.go:824-952` — entries-based resolution, per-reason fallback table (bad colour / not found / unreadable) with `Requested` staying on the persisted slug, embedded-set-first precedence, unresolvable-fallback fatal returning a zero Resolution, and the `theme: loaded` suppression / single `fallback applied` cadence.
  - `resolution_test.go:954+` `TestResolveByNameFrom_MatchesResolveByName` — parity between the two ladders (anti-drift).
  - `internal/tui/theme_panel_cursor_test.go` — one directly named test per criterion, all driven through the **production** `DirThemeSource` over a real temp themes dir (`newDirBackedPanelModel`, `theme_testing_test.go:48`), with the light/dark answer pinned rather than raced: `_Constant` (1), `_InForceSlot` light+dark subtests (2), `_BothSlotsSameSlug` incl. a "exactly one row" assertion (3), `_FallbackRow` + `_BadgeStaysOnPersisted` (4), `TestPanelOpen_DoesNotChangeTheRenderedTheme` (5), `_AppliesMidSessionEdit` (6), `_InvalidatedActiveThemeFlipsOnOpen` (7), `_RepairedThemeAppliesOnOpen` — constructed on a broken slug so the launch really is on the fallback, with a precondition assert (8), `_AnchoredByIdentity` — two fixtures resolving the identical setting differing only by a row inserted above the target, so an index anchor cannot pass both (10), `TestDeps_HasNoThemeSlots` — reflect + AST guard (11), `_DegradesOnMissingIdentity` incl. the empty-union case (12), `TestPanelOpen_ResolveErrorDegrades` — fake returns a *fully populated* resolution alongside the error, so an ignored error would visibly badge/repaint/move (13), `TestPanelOpen_CursorInvariant` — 4-case table asserting selectable AND `row.Theme == active` (14), `TestPanelOpen_NoNewOSC11Query` — scans the returned cmds for a background-colour request and a detection timeout, and asserts the gate stayed resolved (15), `TestPanelOpen_WritesNothing` — byte-compares a present prefs.json (staged over a *rejected* theme, the case where a write would clobber the user's name) and asserts an absent one stays absent plus the themes dir is untouched (9, write half).
  - Seam conformance: `internal/tui/theme_seams_test.go:36-41` compile-time assertions for the fixture fake and the exported adapter; `internal/theme/dir_theme_source_test.go:64-80` asserts the adapter's `Resolve` records no load.
- Notes:
  - Would fail if the feature broke: the invariant table and `_CursorInvariant`'s `row.Theme == m.themeState.active` comparison catch both a mis-anchored cursor and a missing `ApplyTheme`; `_ResolveErrorDegrades` catches a swallowed error.
  - Deliberate overlap between the four case tests and `_CursorInvariant` is what the task asked for (the invariant asserted directly rather than inferred) and the table asserts a different property, so it is not redundancy.
  - No over-testing found: no test asserts an internal call sequence, and the fakes are thin recorders.

CODE QUALITY:
- Project conventions: Followed. Seam-per-capability DI with a production adapter plus test/fixture fakes; `internal/theme` stays log-component-owning and `internal/tui` emits none; no raw hex; no package-level theme state; comments carry no task ids/phase/§ references; tests avoid `t.Parallel()`.
- SOLID principles: Good. `resolutionPass` is a clean strategy pair (where a slug loads from × how the slot is reported) that makes the "retained parse must not emit `theme: loaded`" rule unrepresentable-if-wrong. The seam is 4 focused methods; the adapter adds no policy.
- Complexity: Low. `resolveSlot` is one branch plus a fallback; `themePanelRowIndex` is two `slices.IndexFunc` calls; `applyInForceTheme` has two guarded early returns.
- Modern idioms: Yes — `slices.IndexFunc`, `max`, `cmp.Or` for identity, method values as `slugLoader`.
- Readability: Good. Every non-obvious ordering decision carries a why-comment (arm order, badge key vs slug, identity-not-index, degrade-not-escalate).
- Issues: None blocking. Comment accuracy spot-checked against the code in `resolution.go`, `dir_theme_source.go`, `theme_seams.go`, `theme_panel.go` — all hold.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_seams.go:10 — the last doc line reads "Resolve's error is the broken-builtin fatal." The task asked the degrade policy be pinned once for all three panel call sites; it is currently restated separately at `theme_panel.go:170` and `theme_panel_commit.go:119`. Extend the seam doc to: "Resolve's error is the broken-builtin fatal, which every panel call site degrades on — the open, the close and the commit recompute leave badges, the active theme and the cursor untouched rather than escalating."
- [do-now] internal/tui/theme_panel.go:150-151 — two back-to-back `anchorThemePanelCursor` calls read as a duplicated line; the second is the capture-only seed. Add above line 151: "// The capture seed anchors after the resolved slug so a fixture's cursor wins; empty in production, where it is a no-op."
- [quickfix] internal/tui/theme_panel_cursor_test.go:96-106 — `TestPanelOpenCursor_BothSlotsSameSlug` loops over `[]theme.Member{MemberLight, MemberDark}` inline, so a failure does not name the mode. Route it through the existing `forEachCanvasMode(t, func(t *testing.T, mode theme.Member) { ... })` helper (`theme_testing_test.go:337`), which the sibling tests use and which names the subtest.
