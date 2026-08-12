TASK: theming-system-5-4 — Per-Slot Mode-Matched Fallback For An Unloadable Nomination

ACCEPTANCE CRITERIA:
- A constant naming a deleted drop-in resolves `tokyo-night` with `FellBack=true`, `Requested` the persisted slug, `Reason` `not found`.
- A broken light slot falls back to `tokyo-night-day`; a broken dark slot to `tokyo-night`; a broken constant to `tokyo-night` — asserted against `DefaultLightSlug`/`DefaultDarkSlug`, not literals.
- Every cause takes the same path with only `Reason` differing (`not found`, `bad name`, `missing tokens`, `bad colour`, `bad syntax`, `unreadable` — directory or file).
- Both slots broken in one launch → two `SlotResolution`s with `FellBack=true` and the pair carries the two shipped defaults.
- One slot broken → the other slot's record has `FellBack=false` and an unchanged theme.
- An unset slot yields `FellBack=false`, `Requested` = the shipped default's slug; a virgin install produces zero fallbacks.
- `SlotResolution` declares no set-ness flag; unset and set-to-the-default produce identical records.
- `prefs.json` bytes identical before/after on every path; no file created when absent.
- `Slots` length 1 under a constant, 2 (light then dark) under a pair; `Nomination.IsConstant()` matches `Setting.IsConstant`.
- A fallback that cannot resolve returns an error rather than a second fallback or a zero-valued `Theme`.

STATUS: complete

SPEC CONTEXT:
§8.5 (specification.md:832-848) pins the per-slot mode-matched fallback table (`theme_dark` → `tokyo-night`, `theme_light` → `tokyo-night-day`, constant → `tokyo-night`), states it introduces no new mechanism (it is the "an unset slot holds the shipped default" rule applied to a set-but-unloadable slot), and warns that changing these values — or adopting the rejected single-fixed fallback — silently invalidates §8.3's "the adaptive pair degrades to a constant dark default" argument. §6.3 (line 472) forbids overwriting the persisted name on fallback. §8.4 (line 800) requires embedded-set-before-directory ordering, which is what carries §5.4's no-shadowing property on the non-enumerating construction path. §9.5 (line 1129-1134) needs the pre-fallback slug (`Requested`) separately from the resolved one so the `●` stays on the persisted slug. §7.6 (line 663-674) makes an unresolvable fallback a fatal returned error, never a panic and never a hardcoded last-resort palette.

IMPLEMENTATION:
- Status: Implemented (extended by later plan phases; extensions are consistent with this task's shape)
- Location:
  - `internal/theme/resolution.go:5-58` — `Slot` (`SlotConstant`/`SlotLight`/`SlotDark`), `SlotResolution`, `Resolution`, exactly the fields the plan specified and no others.
  - `internal/theme/resolution.go:65-67` — `ResolveNomination(Setting, themesDir)`; doc comment carries the never-writes-prefs rationale ("falling back must never overwrite the persisted name, so fixing the theme file restores it on the next launch").
  - `internal/theme/resolution.go:141-165` — `resolveNomination`: constant → one slot + `ConstantNomination`; pair → light then dark + `AdaptivePair`. No member is selected here (the gate owns that).
  - `internal/theme/resolution.go:170-190` — `resolveSlot`: one `pass.load`, on rejection one fallback through the *identical* loader, `FellBack=true` with the original `Reason` and `Requested` preserved; a failed fallback returns `BrokenBuiltinError(fallbackSlug)` with a zero `Resolution`. No second fallback, no hardcoded palette.
  - `internal/theme/resolution.go:208-216` — `defaultSlugFor(slot)` expressed in `DefaultLightSlug`/`DefaultDarkSlug`, never literals, with the rejected-single-fixed-fallback rationale in-comment.
  - `internal/theme/setting.go:77-95` — the same `defaultSlugFor` serves the unset-slot substitution, so "unset" and "set but unloadable" converge on one function rather than two mechanisms; `TestSlotDefault_IsPairedWithASlotInOneFunction` (`slot_default_test.go:67`) structurally pins that there is exactly one such function.
  - `internal/theme/builtins.go:12-15` — the shared default constants.
  - Call site: `cmd/open.go:500-512` (task 5-7) — resolution is read-only and a non-nil error aborts construction.
- Notes:
  - Later plan phases (9-6, 11-x, 15-x, 17-x) added `ResolveNominationFrom`, `ResolveSlot` and the `resolutionPass` (load + report) pairing on top of this task's core. That is intentional supersession, not drift: the fallback rule, per-slot record and error contract are unchanged, and routing the fallback through `pass.load` guarantees it resolves by the identical route the nomination did.
  - Task 5-5 later wired event emission into `reportSlot`/`reportFallback`, superseding this task's "emit nothing directly" instruction. Correct per the amended plan.
  - No `SlotResolution` set-ness flag; `RawKeys` (`setting.go:9-13`) remains the sole home for that distinction, and `ResolveNomination` is deliberately not given it.
  - Embedded-set-first ordering is inherited from `resolveNamed` (`resolve.go:27-37`), so a fallback slug can never be satisfied by a user drop-in — the no-shadowing property §8.5's fallback depends on.

TESTS:
- Status: Adequate
- Coverage (`internal/theme/resolution_test.go`, plus `slot_default_test.go`): every named test in the plan exists with matching intent —
  - `TestResolveNomination_FallbackIsModeMatched:68` (table: light/dark/constant, full-record equality against `DefaultLightSlug`/`DefaultDarkSlug`).
  - `TestResolveNomination_FallbackUsesSharedConstants:362` — behavioural half (a doubly-fallen-back pair paints exactly what a virgin install paints) plus an AST guard that `resolution.go` declares no literal equal to either default slug.
  - `TestResolveNomination_EveryCauseFallsBack:500` — nine causes (deleted, renamed, typo, illegal slug, missing token, bad colour, duplicate key, unreadable file, unreadable directory) mapping onto six reasons, with a completeness sub-test asserting the cause table covers every reachable `Reason`.
  - `TestResolveNomination_BothSlotsCanFallBack:541` (including two different reasons in one launch), `TestResolveNomination_SurvivingSlotUnaffected:581` (with a guard that the drop-in's palette differs from the fallback's, so the assertion cannot be vacuous).
  - `TestResolveNomination_UnsetSlotIsNotAFallback:644` across empty/absent/"" themes dirs; `TestResolveNomination_SetAndUnsetDefaultsAreIndistinguishable:674` including a `reflect`-based field-list guard that pins the six fields (no set-ness flag can be added silently).
  - `TestResolveNomination_NeverOverwritesPrefs:708` — real `prefs.NewStore` round-trip with a byte-for-byte comparison, an absent-prefs case asserting the config dir stays empty, and a call-graph guard proving no `os` write function is reachable from `ResolveNomination` (with an anti-vacuity check that `os.ReadFile` *is* reachable).
  - `TestResolveNomination_UnresolvableFallbackErrors:281` — missing and corrupt built-in variants, a zero-`Resolution` assertion (`requireZeroResolution:266`), and a call-recording sub-test proving exactly `[nomination, fallback]` is attempted — never a second fallback.
  - `TestResolveNomination_StructuredOutcome:134` and `_NominationShapeMatchesSetting:172` cover the record's structure and the constant/pair shape parity.
  - Fixture hygiene is good: `requireDistinctDefaults:34` fails loudly if the two shipped defaults ever converge, which would make a swapped fallback map undetectable.
- Notes: No over-testing found. The overlap between `TestResolveNomination_FallbackIsModeMatched` and `slot_default_test.go`'s per-slot assertions is small and each carries a distinct guard (record equality vs. single-pairing-site). No mocking beyond the `BuiltinSource` seam, which is the only way to stage a broken embedded set. No `t.Parallel()`, per the project rule.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays a leaf (resolution.go imports nothing); no raw hex; test file named after its source file; the `theme` component is emitted only through the injected `EventLogger` seam; no spec-section/task-id citations in production comments (task 11-3's standard).
- SOLID principles: Good. `resolutionPass` (resolution.go:89-95) is a small, well-motivated abstraction — it binds "where a slug loads from" to "how the resolved slot is reported", so a call site cannot pair a retained-parse read with the `theme: loaded` emission. `resolveSlot` has one job and is shared by all three entry points, so the three cannot drift on the fallback rule.
- Complexity: Low. `resolveSlot` is one branch plus one fallback; `resolveNomination` is one branch on `IsConstant`; no short-circuit after the first slot failure, as required.
- Modern idioms: Yes (`cmp.Or` for default substitution, `slices` in tests, function values rather than an interface for the one-method pass).
- Readability: Good. Comments state why rather than what; the `Requested` doc explains precisely why unset and set-to-the-default are one state here.
- Issues: None blocking. One comment regression noted below.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/builtins.go:9-11 — the "changing these values silently breaks the property" warning this task explicitly required (§8.5's trap) was present through task 11-3 and was then dropped by the later comment-audit commit 915e7fcb, even though the preceding strip commit 25626754 named "the shipped-pair degradation trap" among what it kept. The surviving text states the coincidence but not its consequence, so the one thing a future editor of these two constants needs to know is now only in a test failure message (resolution_test.go:379). Replace the block with: "// These are both the shipped adaptive pair and the per-slot mode-matched\n// fallbacks (an unloadable constant `theme` falls to DefaultDarkSlug). The two\n// roles sharing values is load-bearing: the adaptive pair degrades to a constant\n// dark default only because an unresolvable slot lands on the theme the shipped\n// default already nominates, so changing these values — or adopting a single\n// fixed fallback — silently breaks that property with nothing failing to\n// compile. Both must name a theme in the embedded set, or every fallback becomes\n// unresolvable."
