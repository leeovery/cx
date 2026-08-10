## Attempt 1

ISSUES:
- `internal/theme/setting.go:133` — `ResolveSetting` destructures the `RawKeys` it was just handed back into a positional triple: `raw := NewRawKeys(keys.Theme, keys.Light, keys.Dark)`. This is precisely the pattern the task's Problem statement names ("every caller already holds a struct with exactly those three fields and destructures it into three same-typed positional arguments"), reintroduced inside the very function whose signature was hardened to prevent it — and it is a regression at that line, which previously built the value with named fields (`RawKeys{Theme: StripControl(theme), …}`) and was transposition-proof. `NewRawKeys(k.Theme, k.Dark, k.Light)` compiles here exactly as `ResolveSetting(theme, dark, light)` did before. The task offered two shapes ("`NewRawKeys(…)` **or** `func (k RawKeys) Stripped() RawKeys`") and did not foresee that picking the constructor forces this destructure; both shapes together cost three lines and leave no triple inside the package.
  FIX: add an **unexported** normaliser and route both through it, in `internal/theme/setting.go`:
  ```go
  func (k RawKeys) stripped() RawKeys {
      return RawKeys{Theme: StripControl(k.Theme), Light: StripControl(k.Light), Dark: StripControl(k.Dark)}
  }

  func NewRawKeys(theme, light, dark string) RawKeys {
      return RawKeys{Theme: theme, Light: light, Dark: dark}.stripped()
  }
  ```
  then `ResolveSetting` reads `raw := keys.stripped()`. Unexported keeps `wantExports` (and its narrative) exactly as this task already left them, and `StripControl` still has one call body. The inline comment at setting.go:130-132 stays true as written; if reworded, keep the "a plain literal is normalised too" clause, which `TestResolveSetting_StripsKeysItIsHandedUnstripped` pins. No test changes are needed — all four new tests and the reflect pin pass unchanged.
  ALTERNATIVE: export `Stripped()` as the task's second option names it. Tradeoff: it adds a fifth entry to the exported-surface pin for a method with no cross-package consumer, which contradicts the rule that pin's own comment states ("exported for a REASON RATHER THAN FOR A CALLER'S CONVENIENCE"). The unexported form is recommended.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/theme/theme_test.go:205` — the diff appended `NewRawKeys` to the addition list directly above, which falsifies the recency claim in the next paragraph (`InForceKeys` is no longer the latest addition). Per code-quality.md, prefer deleting the claim to re-arguing it.
  OLD:
  ```
  // InForceKeys and the InForceKey it yields are the latest such addition, and
  // they are exported for a REASON RATHER THAN FOR A CALLER'S CONVENIENCE: which
  ```
  NEW:
  ```
  // InForceKeys and the InForceKey it yields are exported for a REASON RATHER THAN
  // FOR A CALLER'S CONVENIENCE: which
  ```
- `internal/theme/theme_test.go:200-201` — new text carries a cardinality claim ("the one place ..."), which code-quality.md lists under *Never in a comment* and which ordinary additive change falsifies; the meaning survives without the count.
  OLD:
  ```
  // taking the raw keys as a value added NewRawKeys — the one place the three keys
  // are control-stripped, which ResolveSetting reads instead of taking three
  ```
  NEW:
  ```
  // taking the raw keys as a value added NewRawKeys — where the three keys are
  // control-stripped, which ResolveSetting reads instead of taking three
  ```

NOTES:
- The stripping-rule doc moved to `NewRawKeys` rather than onto `RawKeys` as the task's step 3 literally suggested. Judged correct — the rule is stated where it now happens, the constructor sits immediately below the type, and nothing states it twice.
- The new comments are otherwise clean against the swept standard: no section citations, no task ids, no test names, no rejected-alternative narration.
- `cmd/open.go:804` now strips twice (constructor, then `ResolveSetting`). Harmless and pinned idempotent — noted only so it is not later "optimised" by deleting the strip inside `ResolveSetting`, which `TestResolveSetting_StripsKeysItIsHandedUnstripped` exists to prevent.
- The exported-surface pin was extended honestly — exactly one entry for exactly one new export, under an exact-set assertion.
- The reflect pin was materially strengthened: from "every parameter is a string" to exact type equality, which fails if the signature ever regresses to a triple.
- Every converted call site was checked field-by-field against the old positional order — no value silently changed fields, in production or in tests.
