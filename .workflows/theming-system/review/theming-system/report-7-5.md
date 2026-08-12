TASK: theming-system-7-5 — The Unresolvable-Persisted-Theme Line Off The Non-Migrating Prefs Read

ACCEPTANCE CRITERIA:
- `{"theme":"nord-lee"}` with no such file yields exactly `⚠ theme nord-lee does not resolve: not found` — no parenthetical.
- `{"theme_light":"solar"}` yields `(light)`; the same in `theme_dark` yields `(dark)`.
- `{"theme_light":"x","theme_dark":"x"}` yields exactly one line carrying `(both)`.
- `{"theme_light":"a","theme_dark":"b"}` yields two lines — `(light)` then `(dark)`.
- `{"theme":"nord","theme_dark":"broken"}` yields no line for `broken` and none for `nord`.
- Absent / empty / no-theme-keys / corrupt `prefs.json` each yield zero lines and never abort the diagnosis.
- `{"theme":"../evil"}` yields `bad name` and composes no path (proven by a planted file).
- Themes dir at mode `0000` yields `unreadable`, not `not found`.
- Empty `deps.ThemesDir` still reports `not found` and composes no path.
- A value carrying a newline/tab/ANSI escape renders stripped, on one line, untruncated.
- A control-only value yields no line (strips to empty ⇒ unset).
- After a full doctor run `prefs.json` is byte-identical, `theme_migrated` still absent; migrating `loadPrefsStore` is nowhere in doctor's call graph.
- A persisted slug naming a valid built-in or valid drop-in yields no line.

STATUS: complete

SPEC CONTEXT:
§14A pins the frame `⚠ theme <slug> (<slot>) does not resolve: <reason>`, with `light`/`dark` under a pair, `both` when the two slots name one slug, and the parenthetical omitted entirely under a constant — one line per slug in every case. §8.4 requires doctor to report the keys *in force* under §8.2's `theme`-wins rule (the constant alone when set, otherwise both slots), because reporting an ignored key sends the user to fix something Portal is not reading. §12.2 requires the read to go through the non-migrating prefs variant so the read-only claim holds literally. §8.6 requires the `[a-z0-9-]` charset check *before* the value becomes a path component. §9.5 requires control-stripping at read time and keeps truncation panel-local (doctor has full width). §5.5 requires `unreadable` (not `not found`) when a directory is unusable, since permissions is the actual problem.

IMPLEMENTATION:
- Status: Implemented (mechanism intentionally superseded by later phases; outcome matches).
- Location:
  - `cmd/doctor.go:67` (`DoctorDeps.PrefsStore`), `cmd/doctor.go:96-100` (non-migrating construction with the doc-comment the task required), `cmd/doctor.go:101-103` (`ThemesDir`), `cmd/doctor.go:155`/`176` (fresh collection per rendered report).
  - `cmd/doctor_theme.go:20-26` (frame + slot constants), `:51-56` (`themeAdvisoryUnion` — one silent loader, one enumeration), `:91-108` (`persistedThemeAdvisories`), `:110-132` (nomination + slot label), `:136-154` (`persistedThemeAdvisory` + suffix).
  - Supporting: `internal/theme/setting.go:126-144` (`InForceKeys`), `internal/theme/resolution.go:119-139` (`ResolveByNameFrom`), `internal/theme/union.go:205-210` (`unresolvedRejection`), `internal/theme/resolve.go:27-37` (charset-before-source ladder), `cmd/config.go:91-99` (inert `loadPrefsStoreNoMigrate`).
- Notes:
  - Two deliberate supersessions vs the task text, both improvements and both covered by the current criteria:
    1. The in-force selection is delegated to `theme.InForceKeys` rather than open-coded from `ResolveSetting` in `cmd` — the same selection the panel's union uses, so doctor and the panel cannot drift on which keys are reported.
    2. Resolution goes through `loader.ResolveByNameFrom(enumeration, slug)` against the single retained enumeration rather than `ResolveByName(slug, ThemesDir)`. This *strengthens* the "no path composed" criterion (no path is composed and no file syscall occurs at all) and preserves the `not found` vs `unreadable` discrimination via `unresolvedRejection`, which keys off `Enumeration.DirUnusable`. `TestThemeAdvisories_DirectoryIsReadOnce` pins the one-read property and forbids `ResolveByName`/`LoadFile`/`ReadDir` in the file.
  - Ordering/§8.2 correctness verified end to end: `NewRawKeys` strips control chars first, so a control-only constant strips to empty and never wins the tiebreak (`InForceKeys` → `ResolveSetting` sees `Theme == ""` and falls to the pair). A constant produces `SlotConstant`, whose `AttrName()` is `("", false)`, which `persistedThemeSlotSuffix` renders as no parenthetical.
  - Read-only contract holds structurally, not just by assertion: `loadPrefsStoreNoMigrate` only resolves a path and constructs a store, and `TestLoadPrefsStore_SingleProductionCaller` (`cmd/prefs_translation_test.go:287`) pins the migrating variant to exactly `open.go:openTUI` package-wide — closing the hole a name-prefix-only guard would leave (a doctor path reaching the migrating read through a non-`doctor*` helper).
  - Nil `PrefsStore` returns nil advisories, not an error; the store read's error is discarded by design so a broken `prefs.json` cannot fail the diagnosis that is meant to report it.
  - Slot vocabulary is not duplicated in `cmd`: the label comes from `theme.Slot.AttrName()`, and a guard asserts `doctor_theme.go` names no `theme.Slot*` constant and declares no `light`/`dark` literal.

TESTS:
- Status: Adequate.
- Coverage: `cmd/doctor_persisted_theme_test.go` (755 lines) implements all 14 named tests from the task, each mapping to at least one acceptance criterion:
  - Frame/slot: `_ConstantOmitsSlot`, `_SlotRendersLightOrDark`, `_BothSlots` (both the collapse and the two-slug two-line ordering), `_ConstantWinsOverSlots`.
  - Silence: `_VirginInstallIsSilent` (incl. a partial pair, and an unset slot verified against a deliberately broken-built-in loader so "check raw keys, not the Setting" is observable), `_ValidSlugIsSilent`, `_ControlOnlyValueIsUnset`.
  - Safety: `_CharsetFailureIsBadName` (6-case table + a planted `evil.theme` exactly where a naive join lands, asserting it is never reached).
  - Reason discrimination: `_NotFoundVersusUnreadable` (absent dir, empty dir, mode-0000 dir, regular-file-where-dir-belongs), `_UnresolvedThemesDirStillReports` (empty `ThemesDir` still reports, and the file producer stays silent on the same input).
  - Rendering: `_ControlStrippedUntruncated` (asserts the escape's printable tail `[31m` is gone too, i.e. a terminal-grammar parse not a byte filter).
  - Tolerance: `_TolerantOnDegeneratePrefs` (7 degenerate documents + unreadable file + nil store + a full `Execute` proving a corrupt file does not abort).
  - Read-only: `_UsesNonMigratingRead` — byte-compares `prefs.json` around a full doctor run over a fixture whose *pending* translation is first proven non-vacuous via the migrating loader, plus the AST guard over every `doctor*` file.
  - Resolution policy: `_NoFallbackAndNoFatal` — asserts the persisted slug is named (never the fallback), reports correctly on a loader staged so `ResolveNomination` provably *would* raise the fatal, and AST-guards `ResolveNomination` to zero calls with a positive counter-assertion that a by-name resolver is still called (so the guard cannot pass vacuously).
  - Silence of the log: `_EmitsNoThemeRecords` — first proves the same condition *does* emit through a real component logger (non-vacuity), then asserts zero `theme` records across a full run in which both emitting conditions are demonstrably reached.
- Notes:
  - Not over-tested. The five tests beyond the task's list each pin a distinct property: `_ValidSlugIsSilent` (the negative case), `_PersistedThemeSlotLabel_ReadsTheSlotsOwnName` (vocabulary ownership), `_FrameIsSingleSourced` (copy declared once), the two `_NothingToReportIsEmptyNotNil` (empty-not-nil contract), `TestThemeAdvisories_DirectoryIsReadOnce` (the one-read invariant). The only overlap — `_ConstantOmitsSlot` and `_ConstantWinsOverSlots`'s second subtest sharing an expected line — is justified: the second is about tiebreak precedence with a broken slot present.
  - Quality is high on the anti-vacuity axis: `requireDropInSlug`/`requireBuiltinSlug` fail loudly if a fixture slug's built-in status ever changes, and three tests explicitly prove their negative assertions could have failed.
  - Would fail if the feature broke: yes — the lines are asserted verbatim, and the read-only and no-fallback properties are pinned by AST guards that survive refactors of the producer body.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays log-free at the diagnosis site via `NewSilentLoader` (the `theme` component records use, never diagnosis — CLAUDE.md's stated rule); the leaf `prefs` package is untouched; no raw hex; no test executes a real command body without injected `*Deps`; the source guards use `sourceguardtest` as the repo's ~20 other guards do.
- SOLID principles: Good. The producer depends on the `theme.Loader`/`Enumeration` seams passed in, not on globals; the in-force selection lives in `internal/theme` where both consumers (doctor and the panel union) share it; `DoctorDeps` keeps the store injectable.
- Complexity: Low. `persistedThemeAdvisories` is a straight-line read → derive → map → filter; every helper is single-purpose and under ~15 lines.
- Modern idioms: Yes — `make([]T, 0, n)` sizing, `cmp.Or` in the setting collapse, small value types, no map iteration anywhere on the render path (determinism is a property of the data structures, as the comment claims).
- Readability: Good. Comments explain *why* (discarded error, no `ResolveNomination`, untruncated slug, slice-not-map for determinism) rather than restating code.
- Issues: Two minor comment-accuracy items, both in doc/test prose, no logic impact — see below.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] cmd/doctor_persisted_theme_test.go:356-357 — the fixture comment ("short enough that the composed basename stays inside the per-name limit — past that the resolver honestly answers `unreadable` (ENAMETOOLONG)") describes `ResolveByName`'s path-composing behaviour, which this producer no longer uses. Through `ResolveByNameFrom` no path is composed and no file syscall occurs, so an over-long slug (`ValidSlug` imposes no length limit) resolves to `not found`, never `unreadable`. Replace with: "// Long enough that no terminal width could carry it. Length is otherwise\n// unconstrained here: the enumeration path composes no path, so no per-name\n// filesystem limit applies."
- [do-now] cmd/doctor.go:55-57 — the `DoctorDeps` group comment ends "and a store left nil makes its check not-evaluable rather than aborting diagnosis", which is now false for the newly added `PrefsStore`: it backs no check, and nil yields zero advisory lines. Append a clause: "…rather than aborting diagnosis. PrefsStore backs the persisted-theme advisory rather than a check, so a nil one produces no lines."
