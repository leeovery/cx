TASK: theming-system-8-14 — §14's Footer Revision On Both Pages With Lockstep Help Filtering

ACCEPTANCE CRITERIA:
1. Sessions footer renders exactly `⏎ attach · / filter · ␣ preview · s switch view · x projects · t theme · m multi` + right-aligned `? help` at the reference width.
2. Projects footer renders exactly `⏎ new session · x sessions · e edit · / filter · t theme` + right-aligned `? help`.
3. `↑↓ navigate` absent from both footers, present in both help bodies.
4. `m` footer label is `multi`; help label stays `Multi-select mode`.
5. Blocked `t` (NO_COLOR) absent from BOTH footer and help on BOTH pages; blocked `m` absent from both on Sessions.
6. Footer height budget and render resolve against the same filtered entries.
7. Narrowing drops entries from the right one at a time; `? help` survives every width at which it fits; empty only below the anchor's own width.
8. No footer label ever wrapped or truncated mid-word at any width.
9. `sessionsKeymap()` / `projectsKeymap()` stay unfiltered static functions; `keymap_dispatch_guard_test` passes including a non-vacuous `t` probe against a faked enumerator.
10. Every updated footer/help golden asserts the §14.2 copy verbatim.

STATUS: complete

SPEC CONTEXT:
§14.1 drops `↑↓ navigate` from the footer (arrows in a list are a given) and promotes both `t` and `m` to Core. §14.2 pins the two footer rows verbatim. §14.3 shortens the label to `m multi`, mandates lockstep filtering of footer and `?` help through ONE call-site filter, and states the Projects footer needs its own call-site filter (§9.10 names only `sessionsHelpKeymap()`). §14.4 inverts the narrow-degrade rule: drop entries from the right, never wrap or truncate a label, `? help` is NEVER dropped, and below the width where the anchor alone fits the row renders empty. §9.10 blocks `t` under NO_COLOR with a flash and filters the `t` help row, explicitly contrasting a capability absence (filter) with a space shortage (degrade, do not filter).

IMPLEMENTATION:
- Status: Implemented (with later plan phases intentionally reshaping some of its comments/helpers — see Notes)
- Location:
  - `internal/tui/keymap.go:29-44` — Sessions descriptor: nav entry non-core (now via the shared `navKeymapEntries()` extracted by a later plan task), `t` added Core, `m` Core with footer `Action: "multi"` / `HelpAction: "Multi-select mode"`, Core relative order `⏎ · / · ␣ · s · x · t · m · ?`.
  - `internal/tui/keymap.go:47-60` — Projects descriptor: `t` inserted after `/`, Core order `⏎ · x · e · / · t · ?`.
  - `internal/tui/model.go:3270-3288` — `sessionsHelpKeymap()` (drops `m` when `multiKeyBlocked()`, `t` when `themeKeyBlocked()`) and the new `projectsHelpKeymap()` (drops `t` only); `dropKeymapKey` at :3303.
  - `internal/tui/model.go:3292-3300` — `multiKeyBlocked()` / `themeKeyBlocked()` (`themeKeyBlocked() == m.colourless`, exactly the predicate `themePanelEntry()` gates on at `theme_panel.go:97`; the geometry floor deliberately does not filter, per §9.10's contrast).
  - `internal/tui/footer.go:22-28` — `renderSessionsFooter` / `renderProjectsFooter` now take `entries []keymapEntry`.
  - Call sites all pass the filtered slice: render `model.go:3192` (Projects), `model.go:3250` (Sessions); budget `model.go:1040` / `model.go:1063`; `?` help modal `model.go:3120` (Projects) / `model.go:3207` (Sessions). No production site passes the static descriptor to a page footer or page help modal.
  - `internal/tui/footer.go:158-171` — `assembleRightAnchoredRow` inverted into four rungs: no anchor → pad left; both fit → cluster+spacer+anchor; only the anchor fits → `headerPadLeft` (new mirror at `header.go:131-137`); below that → empty row of exactly w cells. `fitLeftCluster` (`footer.go:191-207`) and `fitClusterToWidth` (`footer.go:97-124`) are unchanged, so the cluster still drops whole entries with the `· …` marker and never truncates a label.
- Notes:
  - Rung arithmetic traced at the boundaries: `fitLeftCluster` reserves `rightWidth+1` (clamped ≥0), so rung 2 is taken for every `w ≥ rightWidth+1` (at `w == rightWidth+1` the cluster is empty and one spacer cell remains), rung 3 only at `w == rightWidth` exactly, rung 4 below. Mid-word truncation is structurally impossible because `fitClusterToWidth` only ever renders a whole prefix plus the ellipsis.
  - Empty-state footers (`empty_states.go:64-91`) select only `n`/`x`/`/`/`?`, so they need no filter; `filter_footer.go:87` reads the static descriptor purely to source the `? help` anchor glyph, which filtering never touches. Both correct.
  - The contextual filter footers gain a behaviour change from the inversion: `filterFooterRow` has no left-cluster fitter, so they now jump from full-cluster to anchor-alone in one step instead of overflowing the row. Net improvement and §14.4-conformant; see the non-blocking note.
  - Later plan phases superseded parts of this task's authored surface, correctly: the nav/page entries were factored into `navKeymapEntries()` (task 17-x), and the comment sweep stripped the spec-section/task references this task's commit put in `footer.go`/`header.go` doc comments. Current comments carry no process artifacts and no claim the code falsifies.
  - §14.3's width arithmetic does not survive conversion to cells: the pinned Sessions row is 80 cells + 1 spacer + 6 anchor = 87 content cells, so at the spec's 86-column reference `m multi` degrades to `· …`. Copy is spec-pinned and §14.4 handles it gracefully; the tests pin the row at `referenceFooterWidth = 120`. Recorded as a spec-record inaccuracy, not an implementation defect (see notes).

TESTS:
- Status: Adequate (mild redundancy with two pre-existing footer tests)
- Coverage:
  - `footer_revision_test.go:37-59` — pinned Sessions/Projects copy, asserted verbatim against constants that match the spec rows byte-for-byte (criteria 1, 2, 10).
  - `:61-96` — `navigate` absent from both footers, `Move selection` present in both help bodies, nav entry non-Core on both descriptors (criterion 3).
  - `:98-111` — `m multi` in the footer, no `multi-select` in the footer, `Multi-select mode` retained in help (criterion 4).
  - `:150-207` — theme-key lockstep on both pages, with the help half driven THROUGH the composed view (`Model.viewSessionList` / `Model.viewProjectList` with `modal = modalHelp`), so reverting the `projectsHelpKeymap()` wiring at `model.go:3120` fails the suite; `:209-235` — the `m` lockstep (criterion 5).
  - `:237-287` — composed footer == filtered render and `sessionFooterHeight`/`projectFooterHeight` == `lipgloss.Height(filtered)`, with a precondition that filtered ≠ unfiltered (criterion 6).
  - `:353-383` — contiguous width sweep 120 → anchorW asserting the anchor never disappears, the cluster is always a legal prefix (+ `· …`), the drop is monotonic AND at most one entry per cell; `:385-411` — anchor-alone at exactly anchorW and empty at anchorW−1, both with exact row width (criterion 7).
  - `:413-447` — both pages, widths 1–120: exactly 2 rows, exact line width, legal cluster at every step (criterion 8).
  - `:289-321` — static descriptors unaffected while the model-level filters are stripping `t`/`m`; `keymap_dispatch_guard_test.go:266-289` (`TestKeymapDispatchGuard_ThemeKeyProbe`) proves the `t` probe is non-vacuous on BOTH pages (unwired seam ⇒ no-op, faked `ThemeEnumerator` via `themeGuardModel` ⇒ panel opens), and `assertDescriptorDispatchParity` demands a probe for every non-RightAligned descriptor key in both directions (criterion 9).
  - Unit-level assembler coverage: `right_anchored_row_test.go` pins all four rungs directly plus the shared-assembler parity across standard/filtering/applied clusters.
  - Goldens moved with the change: `footer_test.go:28-51,114-127,133`, `projects_footer_test.go:20-26`, `keymap_test.go`, `projects_keymap_test.go`, `help_modal_m_suppression_test.go` (re-pointed to `TestSessionsFooter_ListsMultiOnlyWhenFunctional` on `m multi`, keeping the three-identity matrix).
- Notes:
  - Both issues raised in the task's fix-tracking (attempt 1) are resolved in the current tree: the superseded "footer never lists `m`" assertion is re-pointed, and the Projects help assertion now runs through the composed view.
  - Would the tests fail if the feature broke? Yes for every criterion — copy, order, lockstep filtering (through the render path), budget/render agreement, degrade ladder, and the dispatch probe are each pinned independently.
  - Gap: nothing pins the DELIBERATE non-filtering of `t` in the below-floor (too narrow/short) region — `themeKeyBlocked()` gaining a `|| !fits` leg would leave the whole suite green.
  - Mild over-test: `footer_test.go:159-178` is now strictly subsumed by the revision suite's 1–120 sweep; `footer_revision_test.go:289-299` compares a pure nullary function's output with itself.

CODE QUALITY:
- Project conventions: Followed. No `t.Parallel()`, table-driven with named subtests, unit-lane only (no daemons/binaries), descriptors stay nullary pure functions so the dispatch guard keeps probing the full binding set, filter lives at the call site (the documented seam).
- SOLID principles: Good. `assembleRightAnchoredRow` remains the single owner of right-anchor geometry for both footer families; `headerPadLeft` is a faithful mirror of `headerPadRight`; entries-as-parameter inverts the dependency so budget and render cannot disagree.
- Complexity: Low. The assembler is a four-branch ladder with no nesting; `dropKeymapKey` is O(n) and only the blocked path allocates.
- Modern idioms: Yes — `slices`/`maps` in tests, range-over-int, no reflection in production code.
- Readability: Good. Comments explain the WHY (why entries are a parameter, why the anchor survives, why the budget reserves `rightWidth + 1`) without restating code or citing spec sections/task ids.
- Issues: None blocking. `renderSessionsFooter` and `renderProjectsFooter` are now byte-identical delegations with identical signatures, so neither encodes page identity (see notes).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/help_modal_test.go:178-190, :213-222 — add `"Theme picker"` to both descriptor-completeness action lists; every other descriptor action is listed there, and the entry this task added is the only omission (it renders in both unblocked help bodies, so the assertion passes).
- [quickfix] internal/tui/model.go:3298 (`themeKeyBlocked`) — add a test asserting that a below-floor model (content width/height under `themePanelFloor`) still LISTS `t` in `sessionsHelpKeymap()` and in the rendered footer; today a `|| !fits` leg added to `themeKeyBlocked` would keep the whole suite green, leaving §9.10's space-shortage-is-not-capability-absence rule unpinned.
- [quickfix] internal/tui/footer_test.go:159-178 — delete `TestSessionsFooter_NarrowTruncationNoWrap`; it is strictly subsumed by `TestFooterRevision_LabelsAreNeverTruncated` (widths 1–120, exactly 2 rows, exact width, legal cluster) plus `TestFooterRevision_HelpAnchorSurvivesNarrowing`. Keep `TestSessionsFooter_NarrowTruncationKeepsHighestPriority`, which pins the concrete drop set at width 60.
- [quickfix] internal/tui/footer_revision_test.go:289-299 — drop the `reflect.DeepEqual(sessionsKeymap(), wantSessions)` / `projectsKeymap()` self-comparisons inside `TestFooterRevision_StaticDescriptorsUnfiltered`; a pure nullary function compared against its own earlier result can never differ. The `keymapHasKey` legs plus the blocked-model preconditions carry the actual claim, and the entry-by-entry shape is already pinned by `TestSessionsKeymap`, `TestProjectsKeymap` and `TestKeymapGuard_PageDescriptorsUnchanged`.
- [idea] internal/tui/footer.go:22-28 — `renderSessionsFooter` and `renderProjectsFooter` are byte-identical one-line delegations with identical signatures, so nothing prevents passing the Projects entries to the Sessions renderer; decide whether to collapse them into `renderCondensedFooter` at the call sites (churns ~10 test files) or keep the page-named seams for call-site readability.
- [idea] internal/tui/filter_footer.go:83-96 — `filterFooterRow` renders its cluster unfitted, so under the inverted assembler the contextual filter footers go from full cluster to anchor-alone in one step (for `filterAppliedFooterEntries`, every width below roughly 48 cells); decide whether to give it a `fitFilterCluster`-style fitter so they degrade one entry at a time like the standard footer, or accept the coarser step as the documented trade.
- [idea] .workflows/theming-system/specification/theming-system/specification.md:1766 (§14.3) — the "fits at the reference mock's 86 columns with ~5px spare" claim does not survive conversion to cells: the pinned row is 80 + 1 spacer + 6 anchor = 87 content cells, so at 86 the `m multi` entry degrades to `· …`. The copy is spec-pinned and §14.4 covers the degrade, so no code changes; decide whether to correct the arithmetic (or restate the reference width) so the record matches what renders.
