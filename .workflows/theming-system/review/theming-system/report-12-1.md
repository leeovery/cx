TASK: theming-system-12-1 — Single-Source The "Which Persisted Theme Keys Are In Force" Rule Across The Panel Union And Doctor

ACCEPTANCE CRITERIA:
1. One exported function in `internal/theme` holds the constant-alone, non-empty-slot and same-value-collapse clauses; neither `internal/theme/union.go` nor `cmd/doctor_theme.go` restates any of them.
2. The panel union's persisted rows and doctor's persisted lines are both derived from that function.
3. Doctor's rendered advisory output is byte-identical to today's for every existing test case, and the panel's row set and ordering are unchanged.
4. No comment in either file asserts agreement with the other surface as a prose invariant.

STATUS: complete

SPEC CONTEXT:
- Spec line 828: `portal doctor` reports the keys *in force* — under §8.2's `theme`-wins rule that is the constant alone when one is set, and the slots otherwise; reporting an ignored key would send the user to fix something Portal is not reading.
- Spec line 1467: "One slug produces one advisory line", mirroring §9.4's "one slug is one row, always" — "the two surfaces render the same union and must not disagree about how many problems exist", and `<M>` counts problems rather than detections.
The task's failure mode (panel listing a row doctor does not report, or the reverse) is exactly the spec invariant at line 1467, which was previously held only by prose in two packages. Note the implementation's "only slots with a non-empty raw value" refinement is output-equivalent to the spec's "both slots otherwise": an unset slot resolves to a shipped built-in default, which always resolves and so could never produce a line/row.

IMPLEMENTATION:
- Status: Implemented (with one later, intentional in-plan refinement)
- Location:
  - Selector: `internal/theme/setting.go:106-144` — `InForceKey{Value, Slot, Both}` + `InForceKeys(RawKeys) []InForceKey`, holding all three clauses: `ResolveSetting` tiebreak → constant alone with slots unread (`:128-130`), same-value collapse to a single `Both` entry (`:132-134`), non-empty-slots-only in light-then-dark order (`:136-143`).
  - Panel union: `internal/theme/union.go:176-187` — `persistedRows` now maps over `InForceKeys(keys)` reading only `key.Value`; the former `inForceValues` is deleted (confirmed absent repo-wide).
  - Doctor: `cmd/doctor_theme.go:115-132` — `persistedThemeNominations` maps over `InForceKeys(keys)` and does nothing but attach a label via `persistedThemeSlotLabel`.
- AC1: met. Grepping the repo for the three clauses (`raw.Light != ""`, `Light == Dark` collapse, `IsConstant` branch on persisted keys) finds them only in `setting.go:126-144`. `internal/tui`'s `inForceSlot` / `inForceMode` are a different concept (which slot's theme the detected mode selects) and are not a third restatement.
- AC2: met — both surfaces call `theme.InForceKeys` and nothing else selects keys.
- AC3: met by construction and pinned by the pre-existing doctor line tests, which are unchanged and byte-exact: `cmd/doctor_persisted_theme_test.go:78` (constant, no parenthetical), `:100/:105` (`(light)` / `(dark)`), `:128` (`(both)`), `:139-142` (two slugs, two lines, slot order). Panel rows/ordering unchanged — `union_test.go:258-401` (persisted built-in / invalid file / not-found / unreadable / bad-name / constant-only / both-slots-one-row) all still assert against the same expectations.
- AC4: met. `union.go` no longer carries the "Doctor's persisted line makes the identical call … so the two surfaces cannot disagree" assertion (deleted in this task's commit `8b83715f`), and `setting.go`'s selector doc states the rule without naming a peer surface. The two surviving cross-references — `union.go:204` ("the verbatim OS error belongs to doctor") and `doctor_theme.go:134` ("truncation stays panel-local") — are statements of ownership/division, not prose agreement invariants, so they do not violate this criterion.
- Later in-plan supersede (not drift): task 15-1 (`381e57ca`) replaced doctor's `switch key.Slot { SlotLight/SlotDark }` with `key.Slot.AttrName()` and deleted the `themeSlotLight`/`themeSlotDark` constants. The task's Do-step 3 named those constants, but `AttrName()` returns exactly "light"/"dark" (`internal/theme/resolution.go:19-28`) and `SlotConstant` returns `("", false)` → empty label → no parenthetical, so rendered output is unchanged and the label mapping is now single-sourced too. `themeSlotBoth` correctly survives as doctor-local (`doctor_theme.go:26`), since no slot names that state.
- Notes: `InForceKey.Slot` is documented (`setting.go:112-115`) as carrying `SlotLight` when `Both` is set. Doctor reads `Both` first so it renders `both`, and the panel ignores `Slot` entirely — correct today, and the field comment states the trap for a future consumer.

TESTS:
- Status: Adequate
- Coverage:
  - Selector unit table `internal/theme/setting_test.go:358-418` covers all three clauses and their interactions: constant alone, constant leaves slots unread, light-only, dark-only, two differing slots (order pinned), two-slots-same-value collapse, collapse of a value that is no legal slug (`../evil`), no keys, and a control-only constant that strips to unset and hands force back to the slots.
  - `setting_test.go:420-432` proves an unset slot's substituted default is never in force, and guards itself against vacuity by first asserting `Setting.Light` really is the default.
  - `setting_test.go:434-447` pins the idempotent re-strip the union comment relies on (already-resolved keys give identical output), with a fixture-not-vacuous guard.
  - Cross-surface parity: `cmd/theme_in_force_parity_test.go:14-53` — the shared table has all nine shapes the task asked for (constant, constant+both slots, light only, dark only, both differing, both identical, an illegal value in both, an unresolvable slot beside a resolvable one, all empty) and drives the real assembler (`theme.Assembler.Open`) against doctor's real producer (`persistedThemeAdvisories`), asserting both name the same slug set and that the set matches an independent expectation. Sited in `package cmd`, which is the only package that can reach both.
  - `cmd/theme_in_force_parity_test.go:99-116` guards the parity table against vacuity twice over: it fails if no shape expects a reported value (which would make the parity assertions hold over two silent surfaces) and if no expected value is an illegal slug (the case that proves the collapse keys on the persisted value, not on a derived slug).
- Would it fail if the feature broke? Yes. Re-authoring any clause on either side moves that side's slug set for at least one table shape (constant+slots, both-identical, or an unset slot), and the parity assertion plus the per-side `want` both fire. The pre-existing byte-exact doctor line tests catch a label regression, and `union_test.go` catches a row-set/ordering regression.
- Over-testing: none found. The parity test deliberately uses only unresolvable slugs (comment at `:10-13`) — a resolvable table would agree on the empty set everywhere — and the "unresolvable slot beside a resolvable one" shape covers the mixed case. It compares slug sets rather than rendered text, which is the right granularity: the labels are already pinned by doctor's own tests, and re-asserting them here would duplicate them.
- Notes: The parity test sorts both sides, so it cannot detect an ordering divergence — correct, since the two surfaces order differently by design (panel alphabetical, doctor slot order) and the task's test spec asks only for the same slug set. The panel's own light-then-dark order is pinned in `setting_test.go:385-388`.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays a leaf with no logging (the selector is pure and total); the exported names are enrolled in the package's public-surface guard list (`internal/theme/theme_test.go:172-173`), so removing or renaming them fails a test rather than silently changing the contract. The empty-not-nil return (`setting.go:136`) matches the repo-wide convention pinned by four sibling tests.
- SOLID principles: Good. The selector owns the *decision*, each surface owns only its *rendering* — the union reads `Value` alone, doctor adds a label. Single responsibility is now structural rather than asserted in prose.
- Complexity: Low. `InForceKeys` is three guarded returns and two appends; both call sites are flat loops.
- Modern idioms: Yes — `cmp.Or` for the default substitution, `slices.Equal`/`slices.ContainsFunc`, pre-sized `make(...)` accumulators, table-driven subtests.
- Readability: Good. `InForceKey`'s field comments state the two non-obvious facts (the value is unvalidated-by-design; `Both` occupies the light slot) at the point a consumer meets them.
- Comment accuracy: Comments hold against the code, with one wording nit noted below. The doc comment on `InForceKeys` (`setting.go:120-125`) is the single surviving explanation of why the rule is what it is, as the task required, and carries no process-artifact references.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/union.go:173 — the comment opens "The rule is 'resolves', not 'has a file'", but the implemented rule is "is already listed": a drop-in that fails the ladder (e.g. `bad colour`) still suppresses the persisted row, as `union_test.go:278-298` asserts. Replace the first clause so the words match the code: "The rule is 'is already listed', not 'has a file': a persisted value matching a listed row contributes nothing, or every persisted built-in slug would mint a second `⚠ not found` row."
- [idea] cmd/doctor_theme.go:110-123 — `persistedThemeNomination` is now a 1:1 shadow of `theme.InForceKey` plus a rendered label, produced by a loop that does nothing else. Consider dropping the struct and passing `theme.InForceKey` straight to `persistedThemeAdvisory`, labelling at the format call. Whether doctor keeps its own nomination vocabulary is a design call, so this is a decision rather than a mechanical edit.
