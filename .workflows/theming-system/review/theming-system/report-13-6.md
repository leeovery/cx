TASK: theming-system-13-6 — Derive The Paginated Panel Fixture From Its Base Union And Enumeration (tick-544dbd, severity low, source: duplication)

ACCEPTANCE CRITERIA:
- The base row set and the shared drop-in entry are each declared exactly once in fixtures.go.
- The rendered `theme-panel-paginated` frame is byte-identical to before the change.
- `Count == len(Rows)` after derivation.
- Mutating the derived rows cannot affect `themePanelUnion()` (no slice aliasing).
- Appending a built-in to `themePanelUnion` flows into the paginated fixture with no second edit.
- `go test ./internal/capture` passes.

STATUS: complete

SPEC CONTEXT:
Spec §13.3 mandates a panel fixture "long enough to paginate" so §13.4's swap-and-diff guard can see the `bubbles/list` pagination dots (§11.2/§13.4: the dots are the exemplar of the cached-style class — read out of styles once at construction, so a missed restyle is invisible to a guard that has no paginating frame). §13.2/§13.6 make the Go fixture definitions in `internal/capture` permanent (images and tapes are the scaffolding; the fixtures are not), which is exactly why a silent divergence between two hand-declared row lists in the harness is worth closing. §13.5 states a panel fixture's four inputs (palette, raw keys, enumerated row set, cursor) — this task touches only how input 3 is *composed*, not the frame.

IMPLEMENTATION:
- Status: Implemented; the union half was later deliberately superseded by a stronger mechanism (see below).
- Location (task commit 4521e7e5): `internal/capture/fixtures.go` — `themePanelPaginatedUnion` rewritten to `slices.Clone(themePanelUnion().Rows)` + synthetics with `Count: len(rows)` re-derived; `themePanelPaginatedEntries` rewritten to `slices.Clone(themePanelEnumeration().Entries)` + synthetics; both doc comments rewritten to state the set is derived rather than restating membership.
- Location (current HEAD): `internal/capture/fixtures.go:596-621` — `themePanelPaginatedFixture` derives from `themePanelAdaptivePairFixture()`, sets `fx.themeEnumeration = themePanelDirEnumeration(themePanelPaginatedEntries()...)` and `fx.themeUnion = themePanelUnionFrom(fx.themeEnumeration, fx.themeKeys)`; `themePanelPaginatedEntries` (fixtures.go:614-621) still clones `themePanelEnumeration().Entries` before appending the 30 synthetics.
- Deliberate supersession (not drift): task 17-6 (commit 945ec8bc, "derive the capture panel fixtures' unions from their entries") removed `themePanelUnion` / `themePanelPaginatedUnion` entirely and replaced them with `themePanelUnionFrom` (fixtures.go:421-423), which routes every panel fixture's union through production `theme.Assembler{...}.Reassemble(enumeration, keys)`. This subsumes 13-6's union half with a strictly stronger version of the same outcome: no fixture declares built-in rows at all, so the built-in set can only come from the embedded `builtins/` directory. Verified `internal/theme/union.go:131-137` sets `Count: len(rows)` inside `Reassemble`.
- Byte-identical-frame check: at the task commit the removed literal list (`catppuccin-latte` file row, then `nord`, `tokyo-night`, `tokyo-night-day` built-in rows) was element-for-element identical to `themePanelUnion()`'s rows at `4521e7e5^` (fixtures.go:745-753 of the parent), so the derivation was a pure refactor of the frame's input. Confirmed by diffing the two revisions.
- Notes: no residual restatement of the base set survives — a repo grep for `theme.Row{` / `SourceBuiltin` under `internal/capture` finds only a test-local union in `theme_panel_fixture_test.go:287` that exercises the fake seam's repaint behaviour, not a fixture row set. The helper `rowSlugs` that 13-6 added for its union assertions was removed with the union test rather than left orphaned.

TESTS:
- Status: Adequate.
- Coverage:
  - `internal/capture/theme_panel_fixture_test.go:590-619` `TestPanelPaginatedEntries_DeriveFromBase` — the surviving half of 13-6's test: the paginating parse is strictly longer than the base, its leading entries equal `themePanelEnumeration().Entries` (`slices.Equal`, so the clone's field-wise identity is pinned), the synthetics follow in `themePanelSyntheticSlug` order (which is what keeps both badged rows on page 1), and the base builder mints fresh entries per call — the property the derivation's non-aliasing actually rests on.
  - `theme_panel_fixture_test.go:163-178` `TestPanelFixture_PaginatedOverflowsOnePage` — the last synthetic is in the union but absent from the rendered frame, so the fixture genuinely overflows.
  - `theme_panel_remaining_fixtures_test.go:209-233` `TestPanelFixture_PaginatedDrawsDots` — the four-row panel draws no dots at the same size (the control), the paginating one does, and the active/inactive dots take `accent.primary` / `text.faint`.
  - `TestPanelFixture_UnionIsProductionAssembled` (added by 17-6, `theme_panel_fixture_test.go`) now covers the criteria 13-6's union assertions carried: it DeepEquals every panel fixture's union against `Reassemble(fx.themeEnumeration, fx.themeKeys)`, which pins `Count == len(Rows)` and pins that the base membership can only come from the embedded built-ins — i.e. "adding a built-in flows in with no second edit" is now structural rather than test-asserted.
  - `swap_harness_test.go:56` keeps `theme-panel-paginated` in the swap-and-diff guard with `vivid-01` in its present-strings, so a broken derivation that dropped the synthetics fails there too.
- Notes: not over-tested — 13-6's union sub-tests were dropped when their subject was deleted rather than left as dead scaffolding, and the entries test's five legs each pin a distinct property (length, leading equality, synthetic order, fresh-per-call). No redundant assertion, no mocking beyond the existing fixture seams. All of it is in the unit lane (no tmux, no daemon, no built binary), which is correct for `internal/capture`.

CODE QUALITY:
- Project conventions: Followed. `internal/capture` stays out of the production binary (import guard untouched); the fixture keeps declaring inputs and lets production assemble, which is the direction CLAUDE.md and §13.5 describe. Test carries no `t.Parallel()`.
- SOLID principles: Good — single declaration point for the shared drop-in (`themePanelDropInSlug` → `themePanelEnumeration`), and composition of derived fixture from base fixture matches the file's existing pattern (`themePanelConfirmFixture`, `themePanelNarrowFixture`, `themePanelCommitFailedFixture`).
- Complexity: Low — one clone plus a bounded `for i := range themePanelSyntheticDropIns` loop.
- Modern idioms: Yes — `slices.Clone`, integer `range` over `themePanelSyntheticDropIns`, `slices.Equal` in the test.
- Readability: Good. `themePanelPaginatedEntries` lost its doc comment in the later `d939ae76 chore(comments): strip the capture harness to the code-quality standard` pass; that is a deliberate later sweep, and the `slices.Clone(themePanelEnumeration().Entries)` line states the derivation on its face, so item 4 of the task's Do-list is superseded rather than regressed. The surviving comments on `themePanelSyntheticDropIns` (margin, not the exact threshold) and `themePanelSyntheticSlug` (named to sort after every built-in) hold true against the code.
- Issues: none.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
