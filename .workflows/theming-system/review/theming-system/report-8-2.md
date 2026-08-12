TASK: theming-system-8-2 — Row Ordering — Sort Key, Comparison And The Built-In-First Tie

ACCEPTANCE CRITERIA (from plan):
- `nord` (built-in) sorts immediately before `nord.theme` (`reserved name`), with no other row able to fall between them.
- A `reserved name` row's `SortKey()` is its slug while its `Label()` is its filename, asserted on the same row.
- A `bad name` file sorts by filename; a charset-rejected persisted string sorts by itself; a `not found` persisted row sorts by its slug.
- `Zed.theme` sorts between `tokyo-night-day` and any `zz`-prefixed row, not ahead of `nord`.
- Two rows whose sort keys differ only in case order deterministically (case-insensitive first, byte-wise second), identical across runs.
- The order is total: shuffling the pre-sort input in a table-driven test always yields the identical output sequence.
- `Label()` and `SortKey()` are never equal by construction for a `reserved name`/`bad name` row, and a test asserts neither is computed from the other.
- `Union.Rows` is ordered on return from both `Open` and `Reassemble`; no caller sorts.
- Nothing in the comparator reads a `Theme`, a canvas or any palette value.

STATUS: complete

SPEC CONTEXT: §9.5 (specification.md:1086-1092) fixes the ordering: sort key is the slug wherever one exists (including a `reserved name` row, which is *labelled* by filename); only a `bad name` row falls back to filename; a charset-rejected persisted string sorts by itself ("there is exactly one thing to sort it by, and using it keeps the ordering total"); comparison is case-insensitive with a byte-wise tie-break (or `Zed.theme` files ahead of every valid theme); the one guaranteed tie — a `reserved name` row against the built-in it collides with — is settled built-in first; the `⚠ dir unreadable` condition is outside the ordering entirely (viewport chrome). §9.2 (:1047) pins "alphabetical by slug" and records same-mode-first as rejected. §9.5 (:1084) gives the filename-label rationale for the reserved-name row.

IMPLEMENTATION:
- Status: Implemented (with one deliberate later revision — see Notes)
- Location:
  - `internal/theme/union.go:50-52` — `Row.SortKey()`
  - `internal/theme/union.go:54-71` — `Row.Label()` + `labelledByFilename()`
  - `internal/theme/union.go:214-232` — `sortRows` / `rowBefore` (the three-leg comparator)
  - `internal/theme/union.go:131-138` — sort applied inside `Reassemble`, which `Open` (`:117-124`) routes through
  - `internal/theme/union.go:44-46` — `Row.Identity()` (the later 15-8 revision `SortKey` now delegates to)
- Notes:
  - Every acceptance criterion is met against the current tree. Sort key precedence is slug → filename → persisted string; `Label()` returns the filename only for `bad name`/`reserved name` rows *that have a file* (`labelledByFilename` guards on `Filename != ""`, which correctly keeps the charset-rejected persisted row — also `bad name` — out of that arm and labels it by its raw string).
  - Comparator legs are exactly the three specified, in the specified order, and read only `SortKey()` and `Source` — no `Theme`, canvas or palette value is reachable from it.
  - Adjacency holds structurally: nothing can fall between `nord` and `nord.theme` because their keys are byte-identical, so only the built-in-first leg separates them (`union_order_test.go:50-57` pins the whole ordered label sequence with a `nord-lee` neighbour proving it sorts after the pair, not between).
  - `sort.SliceStable` (not `sort.Slice`) plus deterministic assembly order (built-ins from a sorted `BuiltinSlugs()`, files in `os.ReadDir` order, persisted rows from `InForceKeys`, which is branch-based with no map iteration) is what closes the residual case the three legs cannot settle — e.g. a persisted string byte-equal to a `bad name` filename. The `sortRows` comment states exactly this; the reasoning is sound and honest rather than over-claimed.
  - The ordering is applied inside `Reassemble` as required, so both `Open` and every recompute return an ordered union. Verified by grep that no production consumer sorts: `internal/tui/theme_row.go`, `theme_panel.go`, `theme_panel_commit.go` and `internal/capture/fixtures.go:419-422` all consume the union as given (fixtures deliberately route through `Reassemble` so membership/dedup/order have one implementation).
  - Deliberate later revision (not drift): task 15-8 ("Name theme.Row's Identity Separately From Its Sort Key") introduced `Row.Identity()` and redefined `SortKey()` as returning it, so the slug/filename/persisted precedence the plan asked to document on `SortKey` now lives on `Identity` (`union.go:42-46`), with `SortKey` carrying the "coincides by choice, not necessity" comment 15-8 specified. Behaviour is unchanged and the independence of `SortKey` and `Label` is preserved. Judged against the amended intent, this is correct.
  - `Union.DirUnusable` remains a flag on the union rather than a row (`union.go:95-98`), so the `⚠ dir unreadable` condition is outside the ordering entirely, as §9.5 requires.
  - No palette/variant concept, no same-mode-first ordering, nothing beyond alphabetical-by-slug entered the comparator.

TESTS:
- Status: Adequate
- Coverage: All nine prescribed tests exist with the prescribed names in `internal/theme/union_order_test.go`, plus a white-box complement in `union_internal_test.go`:
  - `TestRowOrder_ReservedNameSortsBySlugLabelsByFilename` (:12) — asserts slug key, filename label, and that the two values differ, on the same row.
  - `TestRowOrder_BuiltinFirstOnTheGuaranteedTie` (:31) — proves the keys are identical first (so the tie is real), then built-in first, then adjacency, then the whole ordered label sequence.
  - `TestRowOrder_BadNameSortsByFilename` (:60) — asserts the row has no slug, sorts and labels by filename, and lands among the slugs rather than ahead of the list.
  - `TestRowOrder_CharsetRejectedSortsByItself` (:84) — `../evil`, asserting neither slug nor filename exists first.
  - `TestRowOrder_CaseInsensitiveThenByteWise` (:107) — `Zed.theme` among the `z`s (case-insensitive leg) and `Zed.theme` before `zEd.theme` (byte-wise leg, `Z`=0x5A < `z`=0x7A). Both legs genuinely exercised.
  - `TestRowOrder_TotalAndDeterministic` (:149) — five permutations of the enumeration over a fixture spanning all six row shapes (valid built-ins, bad-colour file, bad-name file, reserved-name file, not-found persisted slug, charset-rejected persisted string); compares a `sortkey|label|source` triple per row, so a permutation that preserved keys but swapped rows would still fail. Also guards the built-in list itself so a new built-in forces the canonical sequence to be updated rather than silently weakening the assertion.
  - `TestRowOrder_SortKeyAndLabelAreSeparateValues` (:197) — renames the reserved-name row's file to `AAA.theme`/`zzz.theme` (filenames that would sort to either end) and asserts the label follows the file while the position does not. This is the strongest form of the "neither derived from the other" criterion.
  - `TestRowOrder_UnionIsOrderedOnReturn` (:234) — both `Open` and `Reassemble`.
  - `TestRowOrder_NoVariantConcept` (:255) — light/dark/no-palette permutations yield the identical order.
  - `union_internal_test.go:8` `TestSortRows_BuiltinFirstIsARuleNotAnArtefactOfAssemblyOrder` — feeds the collider *first* so a passing result cannot come from `SliceStable` preserving input order. This is the assertion that makes the built-in-first leg real rather than incidental; its absence would have been the notable gap.
- Notes:
  - Tests would fail if the feature broke: they pin whole ordered sequences (not spot checks), and the tie tests fail fast if their fixture stops producing a tie.
  - Lane/convention compliance: untagged (unit lane), no `t.Parallel()`, `t.TempDir()` only, no tmux and no daemon. Fixtures go through `internal/themetest`, matching the project's single-source fixture convention.
  - Minor over-testing: the 5× re-derivation loop at `union_order_test.go:140-144` adds nothing — assembly involves no map iteration and `sort.SliceStable` is deterministic for identical input, so the repeat cannot diverge; permutation-level determinism is `TestRowOrder_TotalAndDeterministic`'s job. See notes.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays a leaf; the ordering is pure and I/O-free (`Reassemble`'s "must perform no I/O and emit nothing" contract holds — `sortRows` touches nothing external). Comments carry no §-references, task ids or test names, matching the topic's comment-hygiene rule.
- SOLID principles: Good. `SortKey`/`Label`/`Identity` are three separately named values on `Row` with one reason to change each; the comparator is one function; sorting is applied at the single assembly chokepoint so no consumer can forget it.
- Complexity: Low. `rowBefore` is three sequential comparisons; `labelledByFilename` is a two-condition guard.
- Modern idioms: Good — `cmp.Or` for the precedence chain. One residual: `sort.SliceStable` in a file that already imports `slices` and `cmp` (see notes).
- Readability: Good. Each comment states the *why* (why case-insensitive first, why byte-wise second, why the built-in wins the guaranteed tie, why stable) rather than restating the code, and `labelledByFilename`'s comment names the exact trap it guards (the charset-rejected row also carries `bad name`).
- Comment accuracy: Verified against the code. `union.go:213` ("Stable, so a pair the legs still tie on holds its assembly order") is true and is the only claim about totality that could have been over-stated — it is not. `union.go:92` ("Rows arrive already in display order — no consumer has to sort") is true of both `Open` and `Reassemble`.
- Security: N/A for the ordering itself; the charset rejection that keeps `../evil` a row rather than a path is upstream (`ValidSlug`) and correctly precedes any slug treatment.
- Performance: Fine. O(n log n) over a handful of rows; `strings.ToLower` allocates per comparison but the row count is bounded by built-ins + directory entries.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/union.go:48-49 — `SortKey`'s doc no longer carries §9.5's totality rationale (it moved to `Identity`, which frames it as findability, not ordering). Extend the existing comment to: "// SortKey coincides with Identity by choice, not necessity: changing the order\n// must change this value alone and leave identity untouched. Its last arm — the\n// persisted string itself — is the only thing a charset-rejected row has, and\n// using it is what keeps the ordering total."
- [quickfix] internal/theme/union.go:214-216 — replace `sort.SliceStable(rows, func(i, j int) bool { return rowBefore(rows[i], rows[j]) })` with `slices.SortStableFunc(rows, ...)` and convert `rowBefore` to a `cmp.Compare`-style int comparator. The file already imports `slices` and `cmp`, so the `sort` import goes away entirely; this is the one `sort.Slice*` site in the repo where the modern form costs no new import (the project's `golang-modernize` skill lists this replacement).
- [quickfix] internal/theme/union_order_test.go:140-144 — collapse the `for again := range 5` re-derivation loop to a single `Reassemble` call. Repeating an identical input against a deterministic assembly (no map iteration) and a stable sort re-proves nothing; input-permutation determinism is already `TestRowOrder_TotalAndDeterministic`'s subject and `Open`/`Reassemble` parity is `TestRowOrder_UnionIsOrderedOnReturn`'s.
- [do-now] internal/theme/union_order_test.go:158 — `themetest.WriteWithCanvas(t, dir, "aa-early.theme", "blue")` reads as "a valid theme with a blue canvas" when `blue` is in fact unparseable, making the row a `bad colour` rejection — the sixth row shape this fixture deliberately spans. Add a trailing comment on that line: `// an unparseable colour: the bad-colour shape, labelled and sorted by slug`.
