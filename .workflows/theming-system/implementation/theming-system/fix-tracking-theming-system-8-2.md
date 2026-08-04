## Attempt 1

ISSUES:

- `internal/theme/enumerate.go:42-44` and `internal/theme/enumerate_test.go:381-383` still assert that §9.5's sort "belongs to the panel and is deliberately NOT applied here". This task moved sort ownership to the assembler, and the executor corrected exactly this sentence in `union.go` (the old `Union.Rows` comment read "§9.5's display sort is the panel's and is deliberately NOT applied here") while leaving its twin in the same package. The package now contradicts itself on who owns the sort, and task 8-4 builds the panel delegate — the reader most likely to hit the stale pointer is the one who must not add a second sort. The load-bearing clause ("NOT applied here") stays true; only the ownership attribution is wrong.

  FIX: In `enumerate.go:42-44` replace "belongs to the panel" with "belongs to `Reassemble` (see `sortRows`)". Apply the same one-clause correction at `enumerate_test.go:381-383` and `:388` ("a panel sort key applied here" → "the assembler's sort key applied here"). No code or assertion changes.

  CONFIDENCE: high

- `internal/theme/union_test.go:671` and `:680` — the `unionSlugs` and `rowsWithSlug` helper doc comments both say "in assembly order", which is no longer what the union hands back. Same class of staleness, same cause; the new `union_order_test.go` helpers correctly say "in the order the union hands them back".

  FIX: Restate both as "in §9.5's display order" (or reuse the new helpers' phrasing).

  CONFIDENCE: high

NOTES:

- **Deviation 2 is correct and independently verified.** The criterion "`Label()` and `SortKey()` are never equal by construction for a `reserved name` OR `bad name` row" is unachievable for a `bad name` row: §9.5 mandates the filename for both ("A `bad name` row is labelled by its filename… The same applies to its position in the list: ordering is alphabetical by slug, falling back to the filename for a row that has no slug"), and the task's own Do section restates both. The task text, not the implementation, is at fault. Asserting non-equality for `reserved name` (`union_order_test.go:38-40`) plus the rename-independence table is the right reading of the intent.
- **The four pre-existing test updates are genuine, not weakenings.** Each was checked: `theme_test.go`'s `wantExports` is an exact-equality guard so the two additions are mandatory; the `BothSlotsSameMissingSlug` `want` flip is `ghost` < `phantom` under the new order with the same `slices.Equal` on the same set; `DirUnusableIsAFlagNotAMember`'s `ghost` < `nord` is correct (and the new `append([]string{...}, BuiltinSlugs()...)` form also removes a latent aliasing hazard in the old `append(BuiltinSlugs(), …)`); the tui seam test trades a now-meaningless positional claim for an identity lookup and gains an assertion. `TestUnion_ReservedNameIsTheOnlyTwoRowCase`, which already pinned built-in-first, was correctly left untouched and still passes.
- **Deviation 1 (the internal test) is justified and confirmed.** The built-in-first leg is dead through the public assembler; the drop-the-leg mutant was killed only by `TestSortRows_BuiltinFirstIsARuleNotAnArtefactOfAssemblyOrder`. Without it the leg would be untested.
- **Non-blocking test gap:** swapping `sort.SliceStable` → `sort.Slice` survives the suite. Stability is only observable on a genuine double-tie (e.g. a bad-name file `Zed.theme` alongside a charset-rejected persisted `Zed.theme` — identical key, both non-builtin), which no fixture stages; the existing case-2 fixture uses `zEd.theme`/`Zed.theme`, which the byte-wise leg settles. `sortRows`' own comment claims stability is what makes fixtures reproducible, so it is arguably worth pinning — but nothing in the task or §9.5 requires it, and the prescribed `SliceStable` is what shipped. Suggested only if a later fixture task needs the guarantee.
- **Deviation 3 (file split) is within the golang-testing exception** and matches the package's own `builtins_test.go` / `builtins_nord_test.go` / `builtins_tokyo_night_day_test.go` precedent.
- `.tick/tasks.jsonl` carries a single status flip (`theming-system-8-2`: open → in_progress); no other task line changed.

MUTATION EVIDENCE (reviewer-run, all against `internal/theme/union.go`, restored bit-identical afterwards — `shasum` verified): 13 mutations, 12 killed, 1 survived.

| Mutation | Result |
|---|---|
| drop built-in-first leg | killed (internal test) |
| invert built-in-first leg | killed (4 tests) |
| byte-wise-only comparison | killed |
| drop byte-wise leg | killed |
| `SortKey` filename-first | killed |
| `Label` = `SortKey()` | killed |
| remove `sortRows` from `Reassemble` | killed |
| `SliceStable` → `Slice` | **survived** |
| drop `Filename == ""` guard in `labelledByFilename` | killed |
| drop reserved-name arm | killed |
| drop bad-name arm | killed |
| `SortKey` drops persisted arm | killed |
| `Label` drops persisted arm | killed |
