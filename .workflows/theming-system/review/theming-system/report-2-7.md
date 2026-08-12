TASK: theming-system-2-7 — Port Nord, the first genuinely external palette

ACCEPTANCE CRITERIA:
1. `nord.theme` parses through `LoadBuiltin` with no rejection and 19 uppercase-canonical tokens.
2. Enrolled as dark in task 2-6's table; the enrolment-coverage assertion passes.
3. Clears every §13.5 rule through task 2-3's auto-enumeration — no test names it, no floor relaxed for it.
4. `state.positive` clears both canvas (6.515) and `bg.selection` (4.500345).
5. The two corrections and three inventions each carry a `#` comment recording their derivation; the two port notes (`text.on-attention` cool, `nord3` serving two roles) are recorded.
6. `BuiltinSlugs()` now contains three slugs and `nord` reserves its slug automatically through task 2-2 (no Go edit).
7. The visual gate was taken and its outcome recorded in the task's commit; anything rejected was re-derived, not shipped.
8. Nothing in `internal/theme`'s Go source mentions Nord — the palette is entirely file-resident.

STATUS: complete

SPEC CONTEXT:
§7.4 is the Nord port record: 13 values lifted directly from the 16-slot palette, two contrast corrections (`state.destructive` `#BF616A`→`#DD8188` retaining ~94% chroma; `state.positive` `#A3BE8C`→`#A7C492` at Oklab ΔE 0.018 with chroma marginally above the source), three inventions (`text.muted`, `text.subtle` interpolated on nord3's hue/saturation to fill the barrelled grey ramp; `bg.attention` an ~8% nord13-into-canvas blend settled at a visual gate after a 20% blend `#54524F` was rejected), and one functional maximum (`text.on-selection` `#FFFFFF`). §7.2 frames the port as a risk test of the 19-token vocabulary before the names become a public contract; §13.5 states the canonical, theme-independent floor rule set that auto-enumerates the embedded set, with the single-token dual clearance (`state.positive` on canvas *and* `bg.selection`) as the leg that caught the uncorrected green. §7.4's phase boundary defers `text.subtle`'s outstanding visual gate to Phase 3's grouped capture and the attribution record to Phase 10's `docs/theming.md`.

Note on later supersession: the plan text calls the first built-in "MV"; the shipped set is `tokyo-night` / `tokyo-night-day` / `nord`, so `nord` is the third built-in and the second *dark* one, which is exactly what the shipped file's header says. Task 14-9 later collapsed the per-built-in test scaffolding, replacing this task's `TestNord_IsEnrolledInFloorChecks` with the generic `TestBuiltins_AreEnrolledInFloorChecks` + `TestFloorsEnumerateTheEmbeddedSet`; that is an intentional strengthening (disk-derived slug set, no named theme), not drift.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - `internal/theme/builtins/nord.theme:1-97` — the palette, header attribution, and the five derivation records + two port notes.
  - `internal/theme/light_pins_test.go:12-16` — `themeIsLight["nord"] = false` (task 2-6's table).
  - `internal/theme/builtins_nord_test.go:1-151` — the port's Go-side guards.
  - Gate artefacts: `testdata/vhs/reference/kill-confirm-modal-nord.png`, `testdata/vhs/reference/sessions-inline-flash-nord.png` (committed Paper references, exported by 32bfc011 for this gate).
- Notes:
  - All 19 shipped values match §7.4's table exactly, and each of the 13 "taken directly" values is a genuine Nord slot (nord0/1/2/3/4/5/6/8/9/13/15, with nord3 and nord6 each serving two roles). 13 + 2 + 3 + 1 = 19 holds.
  - I independently recomputed the WCAG legs (sRGB linearisation + weighted luminance, the same math `go-colorful`'s `LinearRgb` applies) rather than trusting the planning figures. The two sub-0.003 legs both clear: `state.destructive` vs `#2E3440` ≈ 4.502 and `state.positive` on `#434C5E` ≈ 4.501. Spot-checked further: `state.positive` vs canvas ≈ 6.516, `text.muted` 4.624, `text.subtle` 3.177 (inside the 3.00–4.49 band), `text.faint` 1.693 (inside the >1.00/<3.00 band), `accent.key` 4.640, `bg.attention` fill 1.202, `bg.selection` fill 1.448. No leg is under its floor and nothing was rounded before comparison.
  - No floor carve-out exists: `contrast_test.go`'s floors are three constants (`4.5` / `3.0` / `1.10`) with no per-slug branch anywhere in the file.
  - `nord` reserves its slug with no Go edit — `BuiltinSlugs()` (`builtins.go:41`) derives from the embedded filenames, and `TestReservedSet_CoversEveryBuiltinSlug` (`reserved_test.go:90`) loops that set.
  - AC8 holds for production source: the only `nord` mentions outside test files are in `internal/capture/fixtures.go` and `cmd/capturetool/main.go` (later phases' fixture/CLI doc text), none in `internal/theme`'s non-test Go.
  - AC7: commit `a28559c4` records the gate outcome explicitly — user reviewed `testdata/vhs/contrast-validation-nord.png` against the two Paper frames and accepted all five judgements; nothing re-derived. The swatch PNG/tape have since been swept from `testdata/vhs/` per the retention policy in `testdata/vhs/README.md`; the two `reference/` frames are correctly retained.
  - `git log -- internal/theme/builtins/nord.theme` shows a single commit, so no later remediation altered a shipped value; the header's "Portal's second dark built-in" is still accurate against the current three-theme set.

TESTS:
- Status: Adequate
- Coverage:
  - `TestLoadBuiltin_NordIsValid` (`builtins_nord_test.go:38`) — parses with no rejection, slug identity, full token slice equality, plus a structural pin that `border == text.faint` (the nord3 dual role).
  - `TestNordFile_CorrectionsAndInventionsCarryComments` (:108) — via the shared `assertDerivationRecords`, asserts each judged token's shipped value, that its comment block carries the `Correction`/`Invention` marker, and that the load-bearing *figures* are present (`#BF616A`/`3.05`/`94%`/`Oklab`; `#A3BE8C`/`4.23`/`0.018`/`100.8%`; `nord3`/`interpolated`/`4.62`/`3.18`; `nord13`/`8%`/`1.20`/`#54524F`/`visual gate`). The marker sweep over `TokenNames()` also proves no *other* token claims a correction or invention, which is what keeps the 13+2+3+1 arithmetic honest — I verified the sweep cannot false-positive: the header block is unreachable from `commentBlockAbove` (the blank line at 29 breaks the walk) and the `border` note says "invented"/"invent", never the capitalised marker.
  - `TestNordFile_HeaderAttributesThePalette` (:116) — header names Nord, the upstream URL, the "corrected for Portal's contrast floors" sentence, and both corrected token names.
  - Floors: `embeddedThemes` (`contrast_test.go:223`) loads every `BuiltinSlugs()` member, so all ten §13.5 rule carriers run against nord automatically — foreground-vs-canvas, the `text.subtle` band, the `text.faint` decorative band, the three three-leg tint pair rules, foreground-on-tint pairings, preview chrome, and `TestStatePositiveClearsCanvasAndSelection`. I cross-read §13.5's rule table against the file: every stated rule has a carrier, so nord's clearance is genuinely complete rather than assumed.
  - Enrolment is guarded from both ends: `TestFloorsEnumerateTheEmbeddedSet` (equality against `BuiltinSlugs()`), `TestBuiltins_AreEnrolledInFloorChecks` (disk-derived slugs), `TestThemeAppearanceTableCoversEveryEmbeddedTheme` + `TestThemeAppearanceTableHasNoStaleEntries` (the light/dark table is exactly the embedded set), and `TestLightPins_SkipDarkThemes` (nord takes no light pin).
  - `TestEveryEmbeddedThemeIsValid` (`embedded_test.go:23`) independently proves 19 populated upper-case-canonical `#RRGGBB` values for nord, which is AC1's second half.
- Notes:
  - Not under-tested: every acceptance criterion has a failing-if-broken guard, and each is enumeration-driven so deleting the file or forgetting the table entry fails loudly rather than silently shrinking coverage.
  - Mild over-test: `wantNordTokens` restates all 19 shipped hexes in Go — see the non-blocking note. It is a change-detector for the 12 values that carry no derivation record, and §13.6 deleted `TestMVDarkVariantsPinned` for that exact reason.
  - The task's named `TestNord_IsEnrolledInFloorChecks` no longer exists; 14-9 replaced it with a strictly stronger generic assertion. Not a gap.

CODE QUALITY:
- Project conventions: Followed. External `theme_test` package, no `t.Parallel()`, table-driven subtests with `t.Run`, failure messages state got-vs-want and why the assertion exists. Vacuity guards (`len(slugs) == 0` → `t.Fatal`) are present on every enumeration, matching the repo's guard idiom.
- SOLID principles: Good. The palette is data, the loader is shared with drop-ins, and the test file adds no theme-specific machinery — it consumes the shared `derivationRecord` scaffolding.
- Complexity: Low. No production code changed; the deliverable is one data file plus one table entry.
- Modern idioms: Yes — `slices.Concat`, `slices.Equal`, `strings.SplitSeq`, `maps.Keys` with `slices.Sorted`.
- Readability: Good. The `.theme` file reads as a port record: header states the palette, source, correction disclosure and the 13+2+3+1 arithmetic; each judged value carries its derivation immediately above it; the three section comments (text ramp / accents and states / surfaces) match `tokyo-night.theme`'s layout, and key order matches the canonical table order.
- Issues: None material. The header is long (28 lines) for a file exported verbatim by `portal theme export`, but every paragraph is spec-mandated content (attribution, correction disclosure, derivation method, fidelity-vs-floors resolution) rather than restated code or process residue, and it carries no TODOs or workflow vocabulary.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] `internal/theme/builtins_nord_test.go:16-36` — `wantNordTokens` pins all 19 shipped hexes in Go, duplicating the `.theme` file that is their source of truth; `TestEveryEmbeddedThemeIsValid` (`embedded_test.go:23`) already proves 19 populated upper-case-canonical tokens for every built-in, and the derivation records already pin the 7 judged values. Decide whether to reduce the pin to the judged values plus a name-order assertion (and, if so, apply the same call to `wantTokyoNightTokens`/`wantTokyoNightDayTokens`) or to keep the full pin as a deliberate edit-detector for a shipped palette — §13.6 deleted `TestMVDarkVariantsPinned` on the former reasoning.
- [do-now] `internal/theme/builtins/nord.theme:78-79` — "matching the proportion Portal's other built-ins use" over-claims for a user-facing exported file: `tokyo-night`'s `bg.attention` is not an ~8% blend (its blue channel moves *away* from the accent, fill 1.15) and `tokyo-night-day`'s is a lifted dark anchor, not a blend at all (fill 1.11). §7.4's own wording is narrower ("matching MV's own proportion: MV's `bg.warning` measures only 1.15"). Replace the clause with "a fill in the same whisper band as Portal's other warning tints (1.11–1.15)" — keeps the `1.20`, `8%`, `nord13`, `#54524F` and `visual gate` figures the derivation guard asserts.
