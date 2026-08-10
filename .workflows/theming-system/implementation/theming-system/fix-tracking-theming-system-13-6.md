## Attempt 1

ISSUES:
- `internal/capture/theme_panel_fixture_test.go:714-719` and `:730-733` — the two aliasing legs cannot fail. Both mutate the derived slice and then compare against a **fresh call** of `themePanelUnion()` / `themePanelEnumeration()`, which build a new slice literal every call (`fixtures.go:746-754`, `:758-760`), so the diagnosis they print ("the derived union aliases the base's backing array") is unreachable. Empirically proven by the reviewer: with both `slices.Clone` calls removed, all five sub-tests still PASS. This is the one acceptance criterion whose "proof" is a false assurance. Secondary: `:715` mutates the parent-scope `paginated` mid-test, so any sub-test added after it silently reads `{Slug:"mutated"}` — an order coupling to avoid.
  FIX: Replace the two aliasing legs with one leg that pins the property the derivation's safety actually rests on — that the base builders hand back a fresh value per call:
  ```go
  t.Run("the base builders mint a fresh row set per call", func(t *testing.T) {
      first := themePanelUnion()
      first.Rows[0] = theme.Row{Slug: "mutated"}
      if themePanelUnion().Rows[0].Slug == "mutated" {
          t.Error("themePanelUnion hands back a shared row set, so every fixture derived from it would alias the others")
      }
  })
  ```
  That leg *does* fail if `themePanelUnion` is ever converted to a package-level value, which is the only arrangement in which the derived append could reach a base anyone else holds. Keep `slices.Clone` in `fixtures.go` (the task asked for it; it is correct defence-in-depth). If any leg keeps mutating a derived value, take that value inside the sub-test rather than mutating the shared `paginated`.
  ALTERNATIVE: Delete the two aliasing legs outright and leave the clone unremarked. Simpler and removes the false assurance in the fewest lines, but it leaves nothing at all guarding the fresh-value assumption the base builders currently satisfy by accident of construction. The replacement is recommended over the deletion for that reason.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- `internal/capture/theme_panel_fixture_test.go:685-686` — the drift claim is wrong in the sharpest case: a base row renamed or swapped diverges the two lists with the counts still equal, so counts are not what would disagree.
  OLD:
  ```
  // A re-declared list would drift silently: both frames would still render, and
  // only the row counts would disagree.
  ```
  NEW:
  ```
  // A re-declared list would drift silently: both frames would still render, and
  // the paginating one would list the stale set.
  ```

NOTES:
- Frame byte-identity independently confirmed by the reviewer via an overlay dump against both HEAD and the working tree — identical.
- The RED probe was independently reproduced: HEAD's hand-written list plus one extra base row makes the new test fail for the right reason.
- A built-in added to the base flows through to the paginated union with no second edit — independently confirmed (35 rows, Count == 35, probe present).
- `panelUnionSlugs()` at `internal/capture/theme_panel_fixture_render_test.go:320-322` is a THIRD hand-written restatement of the same base row set. Out of scope (the criterion is scoped to `fixtures.go`, and that helper lives in the external `capture_test` package which cannot see the unexported builder) — do not fix it here.
- `themePanelPaginatedFixture` (`fixtures.go:1231`) derives the entries from the base but re-wraps them with `themePanelDirEnumeration`, discarding the base enumeration's `DirPath`. Harmless today (both resolve to the same constant) — noted only, no change required.
- `internal/capture/theme_panel_fixture_test.go:701` slices `paginated.Rows[len(base.Rows):]` unguarded; the length guard lives in a sibling sub-test, and a `t.Fatalf` in one sub-test does not stop its siblings. A parent-level length check would be the clean shape.
- The new `rowSlugs` helper is a good call — a mismatch on `[]theme.Row` would otherwise print a wall of zero-valued palettes.
