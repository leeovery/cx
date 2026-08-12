TASK: 11-9 — Collapse the two duplicate `ThemeEnumerator` fakes in package `tui` into one configurable fake (tick-97c325)

ACCEPTANCE CRITERIA:
- Package `tui` declares exactly one `ThemeEnumerator` fake.
- The split-union recompute case is driven by a field, not a distinct type.
- The recorded-asks assertions in the cursor and recompute suites still assert the same asks.
- No `see <other type>.ResolveSlot` style cross-reference comments remain.
- Tests: the full `internal/tui` panel suite (cursor, commit, recompute, open) passes unchanged in outcome.

STATUS: complete

SPEC CONTEXT: The specification governs the panel's theme seam only at the wiring level — §5.6/§8.4 ("the panel receives the directory through the `ThemeEnumerator` seam (§13.3), wired at construction"; enumeration on every panel open, parse results retained for the panel's lifetime, `Esc` re-resolving persisted state against the panel's enumeration). The seam's *test doubles* are not spec-governed; this task is an analysis-remediation (duplication) item whose subject is the package's test scaffolding. Spec relevance is indirect: the fake is what drives the degrade paths a real loader cannot produce (§7.6's fatal `Resolve`, a resolved slug the union has no row for, a selectable row vanishing under the cursor).

IMPLEMENTATION:
- Status: Implemented (mechanism later superseded in-plan — see notes)
- Location:
  - Commit `4a58b9b8` created `internal/tui/theme_enumerator_fake_test.go` with the single `fakeThemeEnumerator`, deleting `stubThemeEnumerator` (was `theme_panel_cursor_test.go`) and `splitThemeEnumerator` (was `theme_panel_commit_recompute_test.go`).
  - Current state: `/Users/leeovery/Code/portal/internal/tui/theme_source_fake_test.go:10-61` — the one declared-values fake, now named `fakeThemeSource` after later in-plan tasks renamed the seam (`13-10`: `ThemeEnumerator` → `ThemeSource`), made it uniform in what it consumes (`15-9`: `theme.Setting` → `theme.RawKeys`), narrowed the slot method (`17-4`: `ResolveSlot` → `LoadSlot`), and collapsed a *third* declared-value fake the original analysis had not spotted (`12-10`).
  - Split-union as a field: `theme_source_fake_test.go:24` (`reassembled *theme.Union`), consumed at `theme_source_fake_test.go:45-48`; the two split fixtures now set it — `theme_panel_commit_recompute_test.go:346-352` and `theme_panel_commit_slot_test.go:83-90`.
  - Seam interface: `internal/tui/theme_seams.go:11-16`.
- Notes:
  - Criterion 1 holds in the current tree. Package `tui` declares exactly one declared-values fake for the seam (`fakeThemeSource`). The two other in-package types that touch the seam are *not* duplicates: `countingThemeSource` (`theme_panel_open_test.go:31-39`) and `behaviourEnumerator` (`theme_panel_behaviour_test.go:18-32`) both **embed the production `theme.DirThemeSource`** and override only `Open`, so no derivation is restated. `fixtureThemeSource` (`theme_seams_test.go:14-34`) lives in the **external `tui_test` package** — it exists to carry the compile-time `var _ tui.ThemeSource` assertions and structurally cannot reuse an unexported in-package fake. No `Reassemble`/`Resolve`/`LoadSlot` body is duplicated anywhere in the package.
  - Criterion 1 was only *fully* true after task `12-10` collapsed the third fake; at `11-9`'s own commit two of three were merged. Judged against the current tree as instructed, it is met.
  - Semantic conversion was done correctly at the one site where the two originals genuinely differed: `splitThemeEnumerator.Resolve` returned the **zero** `Resolution` alongside its error, while the unified fake returns the *declared* resolution alongside `err`. The one fixture that relied on the zero shape now declares it explicitly (`theme_panel_commit_recompute_test.go:394-397`: `enumerator.resolution = theme.Resolution{}` before `enumerator.err = …`), preserving the non-vacuousness of the "badges survive a failed re-resolution" assertion (an empty slot slice yields an empty badge map, so a refresh that ignored the error would wipe every `●`). `git grep` of the pre-commit tree confirms that was the **only** `resolveErr` assignment, so no other fixture silently changed shape.
  - `splitThemeEnumerator.Open`'s hardcoded `theme.Enumeration{DirPath: fixtureThemesDir}` was preserved as a declared field at both converted call sites (`recompute_test.go:348`, `commit_slot_test.go:84`).
  - Scope note respected: `theme.DirThemeSource` (the production adapter, ex-`realThemeEnumerator`/`loaderThemeEnumerator`) is untouched by this commit.
  - No orphaned configuration: every field on the fake has at least one live consumer — `opens`/`keys` (`theme_panel_open_test.go:342-345`, `theme_testing_test.go:118-130`), `reassembles` (`theme_panel_commit_recompute_test.go:425-434`), `reassembleKeys` (`theme_panel_commit_protocol_test.go:87-101`), `resolves` (`theme_panel_close_test.go:74-96`), `slotLoads` (`theme_panel_commit_slot_test.go:57`, `theme_panel_commit_load_test.go:823-852`, `theme_slot_collapse_test.go:37-40`), `reassembled`/`enumeration`/`union`/`resolution`/`err` (split + degrade fixtures).

TESTS:
- Status: Adequate
- Coverage: This is test-scaffolding consolidation, so the coverage question is whether the suites' assertions survived byte-for-byte in meaning. They did:
  - Recorded-ask assertions preserved verbatim across the commit — `theme_panel_close_test.go` still asserts 1 / 1 / 2 resolution counts (`stub.settings` at the commit, `stub.resolves` after `15-9`'s signature change), `requireNoSlotLoad` still asserts `len(seam.slotLoads) == 0` (`theme_panel_commit_slot_test.go:53-59`), and the recompute suite still asserts `reassembles == 0` on a failed commit plus the `== 1` positive control (`theme_panel_commit_recompute_test.go:425-434`).
  - The split-union fixtures still isolate the recompute's effect (`requireRowLabels` over the reassembled union vs the opened one) — `TestPanelRecompute_CursorClampsOnMissingIdentity`, `TestPanelRecompute_ResolveErrorKeepsBadges`, `TestPanelRecompute_SkippedOnFailedCommit`, `TestPanelRecompute_ItemsReplacedNotRebuilt`.
  - The unified fake gained one capability the old `stub` lacked (Reassemble counting). That is additive — no existing assertion changes meaning, and it is what let the third fake be collapsed later.
  - The fake's own non-trivial behaviour has a guard: `theme_slot_collapse_test.go:33-40` asserts the recorded slot-load slug.
- Notes: No new tests were added or needed — the acceptance test is the unchanged outcome of the existing panel suites. Not run (verification is by reading, per role); nothing in the diff is a behaviour change that reading cannot settle. No over-testing introduced: the commit added zero assertions and deleted ~60 lines of duplicated double.

CODE QUALITY:
- Project conventions: Followed. Test-only type lives in a `_test.go` file in package `tui` (no production leakage); no `t.Parallel()`; the DI/`Deps` seam pattern is used unchanged; comments carry no spec-section or task-id references (grep for `§` across the touched files returns nothing — later comment sweeps cleared them and nothing regressed).
- SOLID principles: Good. One double per seam; the configuration axis (split reassembly) is a field, which is the open/closed shape the task asked for.
- Complexity: Low. `Reassemble`'s single `if e.reassembled != nil` is the only branch in the whole double after `17-4` removed the `ResolveSlot` scan/synthesise ladder.
- Modern idioms: Yes — pointer receiver for a recording double, `append`-based ask recording, `*theme.Union` as the "unset vs declared" discriminator rather than a parallel bool.
- Readability: Good. Comments explain *why* the double exists (degrade paths a real loader cannot reach) rather than restating the code; all verified true against the current bodies.
- Issues: None blocking. Naming consistency drifted slightly around the renames (see notes).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/theme_testing_test.go:226 and internal/tui/theme_panel_close_test.go:41,44,51,67,74,83,95 — this task renamed the type `stubThemeEnumerator` → `fakeThemeEnumerator` (now `fakeThemeSource`) but left its helpers and locals on the old vocabulary: rename `stubPanelDeps` → `fakePanelDeps`, `newClosePanelStubModel` → `newClosePanelFakeModel`, and the `stub` local → `fake`, updating the four call sites (`theme_panel_commit_slot_test.go:90`, `theme_panel_commit_recompute_test.go:354`, `theme_panel_close_test.go:67`, plus any other `stubPanelDeps` caller).
- [quickfix] internal/tui/theme_panel_open_test.go:22,41, internal/tui/theme_panel_entry_test.go:35, internal/tui/theme_panel_behaviour_test.go:18 — helper/type names still say "Enumerator" after the seam became `ThemeSource` (task 13-10's residue, surfacing on this task's file): rename `newOpenEnumerator` → `newOpenSource`, `countingEnumeratorOver` → `countingSourceOver`, `newEntryEnumerator` → `newEntrySource`, `behaviourEnumerator` → `behaviourSource` (+ `newBehaviourEnumerator`), and the `enumerator` locals to `source`, matching the already-renamed `fakeThemeSource`/`countingThemeSource`.
- [do-now] internal/tui/theme_source_fake_test.go:20 — the `err` field comment reads as a garden-path sentence ("Returned by both Resolve, alongside the declared resolution, and LoadSlot."). Replace with: `// Returned by Resolve — alongside the declared resolution, not instead of it — and by LoadSlot.`
