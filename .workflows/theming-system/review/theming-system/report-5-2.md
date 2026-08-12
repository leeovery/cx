TASK: theming-system 5-2 — Derive The Theme Setting — A Constant Or A Defaulted Pair, With `theme` Winning (tick-17da3e)

ACCEPTANCE CRITERIA:
1. `("nord","","")` → constant `nord`; `("","","")` → `{tokyo-night-day, tokyo-night}`; `("","nord","")` → `{nord, tokyo-night}`; `("","","nord")` → `{tokyo-night-day, nord}`.
2. `("nord","solarized","gruvbox")` → constant `nord` with `Setting.Light`/`Setting.Dark` empty, `RawKeys` carrying all three.
3. Both defaults expressed as `DefaultLightSlug` / `DefaultDarkSlug`, asserted against those constants (no hardcoded `"tokyo-night"`).
4. Newline / tab / CR / ANSI escape control-stripped in **both** `Setting` and `RawKeys`; result single-line.
5. A control-only value strips to empty and is treated as unset.
6. Interior spaces and case preserved (`"  nord"`, `"Nord"` unchanged).
7. No file or environment access, no error return, deterministic across repeat calls.
8. Nothing references a `Theme`, palette or canvas — slugs only.

STATUS: complete

SPEC CONTEXT:
§8.2 (specification.md:751–766) pins the two-state model: non-empty `theme` ⇒ constant, otherwise the pair; "nothing set" and "pair nominated" are the same state; a hand-edit carrying both resolves `theme`-wins, and **`theme` winning means the slots are not read at all** — which is what makes §9.5's "the two setting states never coexist on screen" a resolution rule rather than a file constraint. Stale slots are left on disk with nothing pruning them (:764), and the one visible consequence is that `d`/`l` later clears the constant and surfaces the stale other slot (:766).
§8.3 (:790) pins "partial pairs do not exist" — the shipped values are the slots' *defaults*, so `theme_dark = nord` yields `{tokyo-night-day, nord}`.
§9.5 (:1094) pins control-stripping **at the point the value is read, not where it is drawn** — a property of the value inherited by both consumers (panel row, doctor advisory), with truncation explicitly separate and panel-local.

IMPLEMENTATION:
- Status: Implemented (mechanism intentionally superseded twice by later in-plan tasks — see Notes)
- Location:
  - `internal/theme/setting.go:9-13` (`RawKeys`), `:19-21` (`NewRawKeys`), `:23-29` (`stripped`), `:51-60` (`Setting`), `:67-80` (`ResolveSetting`)
  - `internal/theme/name.go:52-59` (`StripControl`, the reused helper), `:34-45` (`ValidSlug` — documents the empty-slug/unset-sentinel anchoring)
  - `internal/theme/resolution.go:208-216` (`defaultSlugFor`), `internal/theme/builtins.go:12-15` (the shipped constants)
  - Read-point wiring: `cmd/open.go:501`, `cmd/doctor_theme.go:97` — both construct through `NewRawKeys`
- Notes:
  - **Signature superseded deliberately.** The task specified `ResolveSetting(theme, light, dark string)`; the shipped signature is `ResolveSetting(keys RawKeys)`. Commit `0ae29e82` is this task; commit `0e726ecb`/`0ae29e82` ordering shows task 13-4 ("Take The Typed RawKeys Value In ResolveSetting Instead Of Three Positional Strings", `tick-457f75`) made the change, with `NewRawKeys` taking over the three-positional-string entry point. Not drift — a later plan task, and `setting_test.go:290-300` now pins the whole-value signature as the invariant ("no call site can hand the slots over transposed").
  - **Defaults are routed through `defaultSlugFor(slot)`** rather than naming `DefaultLightSlug`/`DefaultDarkSlug` inline. This is task 17-3 (`8b67b87a`, "single-source the slot to shipped-default pairing") and is *stronger* than the criterion asked: `TestSlotDefault_IsPairedWithASlotInOneFunction` (`slot_default_test.go:67-93`) is a source guard failing if the constants are named in more than one function, and `defaultSlugFor` is the identical function task 5-4's fallback uses (`resolution.go:176`). The "same constants the fallback resolves to" coincidence §8.3 rests on is therefore structural, not a comment.
  - **Missing spec-section citations in comments are intentional**, not a gap. The task asked comments to cite §9.5/§8.3; commits `e30939b2` (task 11-3) and `25626754`/`915e7fcb` deliberately stripped spec-section and phase/task citations from production comments to meet the code-quality standard. The behavioural claims survive without the citations (`setting.go:62-66`, `name.go:47-51`), which is also what the review checklist requires (no references to process artifacts).
  - The flagged ambiguity (strip-before-tiebreak) **is** recorded in source as required: `setting.go:15-18` — "A value that is only control characters strips to empty and so counts as unset rather than as an illegal slug".
  - §9.5's read-point rule holds end-to-end: both production entry points strip via `NewRawKeys` before anything else sees the value, and the TUI receives the already-stripped `raw` (`cmd/open.go:501,511` → `WithThemeKeys`). `ResolveSetting` additionally re-strips (`setting.go:70`) to cover callers that build the struct as a literal, with the reason stated inline.
  - Purity holds: `setting.go` imports only `cmp`; no I/O, no logging, no error return, no `Theme`/palette/canvas reference anywhere in the file.
  - Same-file API added by *later* tasks (not 5-2 scope, verified as correct in passing): `Setting.Slug`/`SlugForSlot` (task 17-4, `62a7c974`), `InForceKeys` (task 12-1, `8b83715f`), `WithConstant`/`WithMember` (task 12-2, `5c7e5d1f`). `WithMember` over a constant-plus-stale-slots correctly yields the pair with the constant dropped and the untouched slot surviving (`setting.go:41-46`), which is exactly §8.2:766's stale-slot-surfacing behaviour.

TESTS:
- Status: Adequate
- Coverage: `internal/theme/setting_test.go` carries all nine test functions the task named, under the exact names specified:
  - AC1 → `TestResolveSetting_ConstantWins:14`, `TestResolveSetting_UnsetSlotsTakeShippedDefaults:42` (table: neither / light-only / dark-only)
  - AC2 → `TestResolveSetting_ConstantIgnoresSlots:28` — asserts both `Setting` slots empty *and* all three raw keys present
  - AC3 → `TestResolveSetting_DefaultsAreTheSharedConstants:93`, which first fails if the two constants are equal (:94-96), so a swapped substitution cannot pass undetected. No hardcoded `"tokyo-night"` anywhere in the assertions.
  - AC4 → `TestResolveSetting_ControlStripsAllThree:135` — 3 keys × 5 payloads (`\n`, interior `\t`, `\r`, `\x1b[31m…`, mixed), asserting the stripped value in **both** the `Setting` slug and the `RawKeys`, plus `assertSingleLine` (:628-634) via `strings.ContainsFunc(unicode.IsControl)`.
  - AC5 → `TestResolveSetting_ControlOnlyValueIsUnset:166` — both the constant-does-not-win case and the slot-takes-default case.
  - AC6 → `TestResolveSetting_NoTrimOrLowercase:193` — leading/trailing/interior space and case, across all three keys.
  - AC7 → `TestResolveSetting_IsPureAndDeterministic:251` — repeat-call equality with an interleaved different call (:253-255, so a memoised or stateful implementation would be caught), plus an AST import guard against a 13-package impure-import list (:265-275, :583-597).
  - AC8 → `:277-301` — `reflect` field-kind assertions pinning both structs to bool/string only, and signature assertions pinning ins/outs (no error, nothing carrying a palette).
  - Edge cases from the task all land: partial pairs (`:56-70`), default-never-reaches-raw-keys (`:220-233`), raw keys identical whichever state resolved (`:235-248`), unstripped-literal input (`TestResolveSetting_StripsKeysItIsHandedUnstripped:570`), idempotence (`TestNewRawKeys_IsIdempotent:562`).
- Notes: Tests would fail if the feature broke — the tiebreak, the per-slot default, the strip and the purity each have a dedicated failing assertion rather than a shared happy-path. No unnecessary mocking (the function is pure; the only "seam" is an AST parse). One mild redundancy noted below.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays a leaf (only `cmp` imported here; the package-level leaf guard at `leaf_guard_test.go:23-35` still holds), no logging from this file, no `t.Parallel()`, external test package `theme_test`, and the repo's comment standard (why-not-what, no process-artifact citations) is met.
- SOLID principles: Good. Single responsibility — the file owns the raw-keys→setting collapse and nothing else; resolution/loading/fallback live in `resolution.go`. The strip helper is reused from `name.go` rather than re-implemented.
- Complexity: Low. `ResolveSetting` is one branch plus two `cmp.Or` substitutions; no nesting beyond one level anywhere in the file.
- Modern idioms: Yes — `cmp.Or` for the unset-slot substitution (`:77-78`, `:89-91`) instead of if-chains; value receivers and copy-returning transformations throughout, which `TestRawKeys_TransformationsLeaveTheReceiverAlone:520` pins.
- Readability: Good. Every doc comment states the *reason* rather than restating the code (`:5-8` why raw keys travel alongside the setting; `:31-32` why the receiver does not survive `WithConstant`; `:69` why the re-strip exists; `:123-125` why only in-force keys are reported).
- Comment accuracy: Verified against the code. `RawKeys`' "control-stripped, as read" is honest given both production constructors go through `NewRawKeys` and `ResolveSetting` re-strips defensively. `Setting`'s "non-empty iff" invariant (`:52-53`) holds on every return path. No stale or contradicted comments found.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/setting_test.go:542-560 — `TestNewRawKeys_ControlOnlyValueStripsToEmpty` re-asserts what `TestResolveSetting_ControlOnlyValueIsUnset:166-191` already pins (IsConstant false, both slots at the shipped defaults, empty `RawKeys`). Drop the `ResolveSetting` half (lines 549-559) and keep only the `NewRawKeys` assertion (lines 543-547), leaving the resolution behaviour to the test that owns it.
