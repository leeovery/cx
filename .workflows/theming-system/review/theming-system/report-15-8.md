TASK: theming-system-15-8 (tick-43bdf2) — Name theme.Row's Identity Separately From Its Sort Key

ACCEPTANCE CRITERIA:
- `theme.Row` exposes a documented `Identity()`; `SortKey()` and `BadgeKey()` are defined in terms of it.
- The three identity consumers read `Identity()`, and no comment explains a sort key standing in for an identity.
- Panel ordering, badge placement and post-commit cursor anchoring are unchanged.
- `go test ./internal/theme ./internal/tui` passes.

STATUS: complete

SPEC CONTEXT:
Specification §9.2 (line 1027) states the panel "re-anchors the cursor to the previewed theme's **identity**, never to its index" — anchoring by index would break the §9.2 invariant (line 1010: "the cursor is always on a selectable row, and that row is always what is painted behind the panel") the moment a commit recompute inserts a row above the cursor. §9.5 separately owns the sort-key rules (the guaranteed `reserved name`/built-in tie, built-in-first resolution) and the `●` badge derivation, where the badge marks what is *set* and the cursor marks what is *previewed* (lines 1123, 1266). The spec therefore already treats ordering and identity as two distinct concerns; this task makes the code say so. §13.4/line 1741 pins panel behaviour (union rules, identity-anchored cursor, badge table) as seam-driven test territory.

IMPLEMENTATION:
- Status: Implemented (comments later condensed by the phase 16/17 comment cycles — outcome intact)
- Location:
  - internal/theme/union.go:41-46 — new `Row.Identity()` carrying `cmp.Or(r.Slug, r.Filename, r.Persisted)`, documented as what the badge table keys on and what the cursor anchors to, and explicitly "not an ordering value".
  - internal/theme/union.go:48-52 — `SortKey()` now returns `Identity()`, with the "coincides by choice, not necessity" note; `rowBefore` (union.go:224) still consumes `SortKey()`, so ordering remains the sole SortKey consumer.
  - internal/theme/badge.go:38-47 — `BadgeKey()` returns `Identity()`, `ReasonReservedName` → `""` exclusion and the empty-string "no badge" answer unchanged.
  - internal/tui/theme_panel.go:230-236 — `themePanelRowIndex` matches on `row.Identity() && row.Selectable()`.
  - internal/tui/theme_panel_commit.go:109-115 — `previewedThemeIdentity` returns `row.Identity()`.
  - internal/theme/theme_test.go:217 — `Row.Identity` enrolled in the closed exported-surface guard (`wantExports`), so the new method is pinned rather than incidental.
- Notes:
  - All three identity consumers named in the task read `Identity()`; the only remaining `SortKey()` call site in the tree is `rowBefore` (verified by repo-wide grep — no other production caller in cmd/, internal/tui, internal/capture).
  - The apologetic "a sort key is standing in for an identity" comments are gone at both TUI sites. theme_panel.go:227-229 now explains only the `Selectable` filter's job ("both rows share an identity"), and `previewedThemeIdentity` carries no stand-in explanation. Repo-wide grep for "sort key"/"SortKey" in production sources leaves only union.go's three legitimate ordering references.
  - Behaviour is unchanged by construction: `SortKey()` and `BadgeKey()` return the identical value they returned before (same `cmp.Or` precedence, same reserved-name exclusion), so ordering, badge placement and cursor anchoring are byte-identical.
  - The slug/identity distinction is correctly *not* over-applied: `committableThemeSlug` (theme_panel_commit.go:19-25) still reads `row.Slug`, which is right — only a real slug can be persisted, and a `bad name`/charset-rejected row must not be committable.
  - Commit 09d26c44 also re-pointed the `internal/capture` fixture helper (`rowSortKeys` → `rowIdentities`, using `Identity()`) and the cursor-seed test, so the test vocabulary follows the production one.

TESTS:
- Status: Adequate
- Coverage:
  - internal/theme/union_identity_test.go:9-39 — `TestRowIdentity_Precedence` pins all three arms across exactly the row shapes the task names: a slugged row, a `bad name` file with only a filename, and a charset-rejected persisted row with neither.
  - internal/theme/union_identity_test.go:41-54 — `TestRowIdentity_SortKeyIsTodayTheIdentity` pins the current coincidence across four row shapes (built-in, reserved-name, bad-name, charset-rejected).
  - internal/theme/badge_test.go:249-292 — `TestBadgeKey_MatchesRowIdentity` asserts both the literal expected key and `BadgeKey() == Identity()` for five shapes; badge_test.go:294-312 keeps the reserved-name `""` answer and additionally pins that such a row's `Identity()`/`SortKey()` are still `"nord"` (the row still IS the slug it collides on, and still sorts beside the built-in).
  - internal/theme/badge_test.go:314-339 — the collided-pair test still proves exactly one row can render the `●`, so the reserved-name exclusion is covered behaviourally, not just by unit assertion.
  - internal/theme/theme_test.go:245-248 — the exported-surface equality guard fails if `Identity` is added/removed/renamed, which is a stronger pin than a bespoke test.
  - Ordering is left covered by the existing internal/theme/union_order_test.go suite (unchanged and still green-relevant, since `SortKey()`'s value did not move), and cursor anchoring by internal/tui/theme_panel_cursor_test.go (`TestPanelOpenCursor_AnchoredByIdentity` at :229 drives a fake source whose rows are declared in explicit order, so anchoring is exercised independently of the real sort).
- Notes:
  - Not over-tested: the new file is 55 lines, table-driven, no mocks, no redundant restatement of the ordering suite.
  - The task's third listed test ("temporarily change `SortKey()` to order by `Label`, confirm anchoring/badges unaffected, revert") is by construction a throwaway experiment and leaves nothing in the tree — correctly so. Nothing durable currently prevents a future edit re-pointing an identity consumer back at `SortKey()`; see the first non-blocking note.

CODE QUALITY:
- Project conventions: Followed. `cmp.Or` is the modern-idiom choice the package already uses (`Label` at union.go:57-62 does the same); the new method is a value receiver like every other `Row` method; the closed exported-surface guard was updated in the same commit rather than left to drift; comments explain *why* (the identity/ordering split) rather than restating code, matching the repo's comment discipline.
- SOLID principles: Good. This is a textbook single-responsibility split — one value per question (what a row IS vs where it sits vs what it displays), with `SortKey`/`BadgeKey` now composed from `Identity` so the three cannot silently diverge in the wrong direction. `Label` was already independent and stays so.
- Complexity: Low. One new one-line method; two call-site substitutions; zero branching added.
- Modern idioms: Yes.
- Readability: Good. `Identity()` at the call sites reads as the intent (`row.Identity() == slug`) where `row.SortKey() == slug` previously required a paragraph of comment to justify — the deletion of those paragraphs is itself the readability win the task was after.
- Comment accuracy: Verified. union.go:41-43, union.go:48-49, badge.go:38-41, theme_panel.go:227-229 and theme_panel_commit.go's doc all hold true against the code, carry no process-artifact references, and the "It is not an ordering value" claim on `Identity` is correctly disambiguated by `SortKey`'s adjacent "coincides by choice, not necessity" note.
- Security / performance: N/A — pure in-memory string selection, no new allocation or I/O.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/ (new `theme_panel_identity_guard_test.go`) — add a source guard, in the style of internal/theme/slug_collapse_guard_test.go:24-64, asserting that no production file in `internal/tui` calls `theme.Row.SortKey`. The separation's whole value is that the two values may diverge, but because they are equal today a future edit re-pointing `themePanelRowIndex` or `previewedThemeIdentity` at `SortKey()` would pass every existing test; the repo already uses exactly this guard shape to hold comparable invariants.
- [do-now] internal/theme/union_order_test.go:303-309 — rename the test helper `rowIdentities` to `rowFingerprints` (and update its two call sites, union_order_test.go:190 and badge_test.go:334). It formats `SortKey|Label|Source`, which is a row fingerprint, not the row's `Identity()`; the name predates this task but is now a direct misnomer against the newly precise package vocabulary, and `internal/capture/theme_panel_fixture_test.go:190` holds a same-named helper that genuinely does map `Identity()`.
