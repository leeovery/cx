TASK: theming-system-17-3 — Single-Source The Slot → Shipped-Default Pairing

ACCEPTANCE CRITERIA:
- `DefaultLightSlug` and `DefaultDarkSlug` are paired with a `Slot` in exactly one function in `internal/theme`.
- `ResolveSetting`, `Setting.Slug` and the fallback path all produce today's answers for every combination of set/unset slots and the constant.
- No behaviour change and no exported-surface change.

STATUS: complete

SPEC CONTEXT:
Spec §8.5 ("Fallback — per-slot and mode-matched") pins the mapping this task de-duplicates: `theme_dark` → `tokyo-night`, `theme_light` → `tokyo-night-day`, `theme` (constant) → `tokyo-night`. The spec is explicit that this "introduces **no new mechanism** — it is the already-decided 'an unset slot holds the shipped default' rule applied to a slot that is *set but unloadable* rather than unset. One rule covers both cases", and §8.5's rejected alternative is exactly the single-fixed-fallback the surviving comment argues against ("a light-terminal user with a typo in their light slot would be thrown to a dark theme"). §8.5 further warns that "changing these values … silently invalidates §8.3", and §8.3 depends on the shipped adaptive pair and the fallback defaults being *the same values*. The refactor therefore makes a spec-level "one rule" claim structural in the code rather than merely asserted in three parallel copies — good alignment, not incidental tidying.

IMPLEMENTATION:
- Status: Implemented (matches the plan's six `Do` steps exactly)
- Location:
  - `internal/theme/resolution.go:208-216` — `fallbackSlugFor` renamed to `defaultSlugFor(slot Slot) string`, comment reworded to the use-agnostic rule ("the slot's shipped default, mode-matched deliberately: one fixed default would throw a light-terminal user with a typo in their light slot onto a dark palette"). Body unchanged.
  - `internal/theme/resolution.go:176` — `resolveSlot`'s fallback call site now reads `defaultSlugFor(slot)`.
  - `internal/theme/setting.go:76-79` — `ResolveSetting` builds `cmp.Or(raw.Light, defaultSlugFor(SlotLight))` / `cmp.Or(raw.Dark, defaultSlugFor(SlotDark))`.
  - `internal/theme/setting.go:86-95` — `Setting.Slug`'s two slot arms route through `defaultSlugFor`; the `default` arm still returns `s.Constant` unchanged (unset constant → empty string, no constant default).
  - `internal/theme/builtins.go:12-14` — `DefaultLightSlug` / `DefaultDarkSlug` untouched, as instructed.
  - Commit `8b67b87a` (5 files: the two sources, the new test, plus task-tracking files). Diff is exactly the four call-site substitutions + the rename + comment.
- Notes:
  - Verified single-pairing: `DefaultLightSlug` / `DefaultDarkSlug` appear in non-test `internal/theme` sources only at their declaration (`builtins.go:13-14`) and inside `defaultSlugFor` (`resolution.go:213,215`). No other production function in the package names them.
  - No stale `fallbackSlugFor` reference survives anywhere in the tree; the only remaining hits are historical `.workflows/…/analysis-*.md` records of the finding, which correctly describe the pre-fix state and should not be rewritten.
  - Behaviour is bit-identical: `defaultSlugFor(SlotLight) == DefaultLightSlug`, and every other slot (`SlotDark`, `SlotConstant`) → `DefaultDarkSlug` — the same answers the three inlined copies gave, including the constant slot's §8.5 dark fallback.
  - No exported-surface change: `defaultSlugFor` is unexported, and `theme_test.go`'s exported-surface list (`internal/theme/theme_test.go:156-157`, asserted at :246) still names both slug constants and needed no edit — plan step 6 was correctly a no-op.
  - Placement is coherent: `Slot` itself is declared at the top of `resolution.go`, so the mapping function sits beside the type it maps from, and `setting.go` reaches it in-package.
  - Out of scope and correctly untouched: `cmd/config.go:176-185`'s `translateAppearance` maps the legacy `appearance` *strings* (`"light"`/`"dark"`) to the two slug constants. That is a different axis (migration value → slug, not `Slot` → slug), sits outside `internal/theme`, and is correct-by-construction since it references the same constants.

TESTS:
- Status: Adequate (with mild redundancy — see notes)
- Coverage:
  - `internal/theme/slot_default_test.go:11-65` `TestSlotDefault_IsTheSlotsOwnShippedDefaultOnEveryPath` — the requested single table over `SlotLight`/`SlotDark`, asserting all three paths (the `ResolveSetting` substitution, `Setting.Slug` on an unset slot, and the `resolveSlot` fallback for an unloadable nomination) land on that slot's own default. Each case pins the *other* slot to a real theme, so a swapped mapping is caught rather than masked.
  - `internal/theme/slot_default_test.go:67-93` `TestSlotDefault_IsPairedWithASlotInOneFunction` — the structural guard: an AST walk over the package's production sources counting distinct functions that name either constant, requiring exactly one. This is the assertion that actually enforces the acceptance criterion and is the genuinely new coverage.
  - Vacuity is pre-empted: `requireDistinctDefaults` (`resolution_test.go:34-43`) fatals if the two defaults are the same slug *or* parse to the same palette, so none of the "never the other slot's" assertions can pass trivially.
  - The guard scans production files only — `sourceguardtest.PackageGoFiles(".", false)` via `themeSourceFiles` (`leaf_guard_test.go:152-159`) drops `_test.go` — so the test's own `theme.DefaultLightSlug` references cannot trip it, and the const declaration in `builtins.go` is outside any `fn.Body` so it is correctly not counted as a pairing.
  - The constant-slot assertion the plan asked to keep survives: `setting_test.go:341-346` ("an unset constant has no default to substitute" → `""`).
  - `SlotConstant`'s fallback to `DefaultDarkSlug` (spec §8.5 row 3) remains covered by the pre-existing table at `resolution_test.go:98-106`.
  - `slug_collapse_guard_test.go:56-59` exempts `internal/theme/*_test.go`, so the new test's `ResolveSetting`-then-`Slug` sequence does not trip the one-collapse guard. Correct by construction, not by luck.
- Notes: two of the three per-slot behaviour assertions restate coverage that already exists elsewhere, and the first of them is weaker than its failure message claims — detailed under NON-BLOCKING NOTES. Neither is a coverage hole.

CODE QUALITY:
- Project conventions: Followed. Unexported helper, no logging added to a package whose events go through the injected `EventLogger`, no exported surface touched, guard test built on the shared `sourceguardtest` primitives (`PackageGoFiles`) per CLAUDE.md rather than a hand-rolled walk, and the test is stdlib-only and untagged so it runs in the unit lane.
- SOLID principles: Good. Single-responsibility improved — one function now owns the slot→default rule and two call sites express themselves through it rather than restating it.
- Complexity: Low. Net effect is four call-site substitutions; no new branching.
- Modern idioms: Yes. `cmp.Or` retained at both substitution sites; the helper stays a two-line `if`/return rather than being inflated into a map or switch.
- Readability: Good. The rename earns itself — `defaultSlugFor` reads correctly at the substitution sites where `fallbackSlugFor` would have been actively misleading (nothing has fallen back at `ResolveSetting`).
- Comment accuracy (changed code): Accurate. `defaultSlugFor`'s new comment states the rule once and its justification matches §8.5's rejected alternative verbatim in substance. `Setting.Slug`'s retained "the per-slot half of ResolveSetting's substitution" is still true and is now literally true — both halves call the same function. `builtins.go:10`'s "an unloadable constant `theme` falls to DefaultDarkSlug" still holds, since `defaultSlugFor(SlotConstant)` returns `DefaultDarkSlug`. No comment references a task id, phase or spec section.
- Issues: none.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/slot_default_test.go:42-46 — the assertion labelled "ResolveSetting substituted %q into the unset slot" reads the value back through `setting.Slug(tt.slot)`, and `Slug` performs the same default substitution itself, so a `ResolveSetting` that stopped substituting altogether would still pass this check (it catches a *wrong* substitution, not a *missing* one). Add a `field func(theme.Setting) string` column to the table returning `s.Light` / `s.Dark` and assert the struct field directly. Behaviour remains covered by `setting_test.go:42-91`, so this is assertion precision rather than a coverage gap.
- [quickfix] internal/theme/slot_default_test.go:70-87 — the pairing guard inspects only `fn.Body`, so a package-level `var slotDefaults = map[Slot]string{SlotLight: DefaultLightSlug, SlotDark: DefaultDarkSlug}` added *alongside* `defaultSlugFor` would re-duplicate the pairing and still leave `len(pairing) == 1`. Extend the walk to package-level `*ast.ValueSpec` values, excluding `builtins.go`'s own const declaration of the two slugs.
- [idea] internal/theme/slot_default_test.go:40-64 — two of the table's three assertions duplicate existing coverage: the unset-slot `Slug` answer is already pinned by `setting_test.go:329-340`, and the light/dark fallback by `resolution_test.go:82-97`, which additionally pins `Reason` and `Theme`. Decide whether the cross-path table's value (stating the one-rule invariant in a single place) justifies the triplication, or whether the older single-path rows should be trimmed toward it — noting the `resolution_test.go` rows assert strictly more and should not simply be deleted.
