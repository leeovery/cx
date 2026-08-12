TASK: theming-system-2-2 — Reserve The Built-In Slugs — The No-Shadowing Safety Property

ACCEPTANCE CRITERIA:
- A `themes/tokyo-night.theme` whose contents are perfectly valid is rejected `reserved name`.
- Same when the file is unreadable (mode 0000) or absent — decided from the slug alone, before any read, so it cannot report `unreadable`.
- `Nord.theme` beside the built-in `nord` reports `bad name`, never `reserved name`, and never yields the slug `nord`.
- The reserved check is exact string equality; no `strings.EqualFold`, no lowercasing, anywhere on the path.
- Every member of `BuiltinSlugs()` reserves — the test loops the enumerated set and names no theme.
- Safety property: a broken `tokyo-night.theme` in the directory enumerates as `reserved name` AND `LoadBuiltin("tokyo-night")` still returns the embedded, valid theme.
- `nord-lee.theme` (the published workaround) loads normally.
- A `Loader` constructed with an explicitly empty injected set still never rejects as reserved.

STATUS: complete

SPEC CONTEXT:
§5.4 (specification.md:357-367) makes built-in slugs reserved because an invalid theme falls back to a built-in: if a user file could shadow the fallback, the fallback itself could be broken. §5.2 (:341-351) fixes "reject, never normalise" so the reserved check stays exact string equality and `Nord.theme` is safe on a case-insensitive macOS filesystem. §6.2 (:447-456) pins the ladder: rung 1 `bad name` (filename, before open), rung 2 `reserved name` (slug alone, before any read, structurally unreachable for a `bad name` file), then `unreadable` and the content rungs. §8.4's half of the guarantee (embedded set consulted before the themes directory on the non-enumerating by-name path) was Phase 5 scope and is present today at `internal/theme/resolve.go:27-37`.

IMPLEMENTATION:
- Status: Implemented (mechanism subsequently hardened by later remediation phases; that is intended, not drift)
- Location:
  - `internal/theme/load.go:12-26` — `Loader.ReservedSlugs map[string]struct{}` seam retained and documented (zero value reserves nothing).
  - `internal/theme/load.go:31-45` — `NewLoader` populates the set once per loader via `builtinSlugSet()`; `NewSilentLoader` delegates to it, so a silent/diagnosing loader judges identically ("silence is about emission only").
  - `internal/theme/load.go:74-89` — `LoadFile` rung order: `SlugFromFilename` (rung 1) → `isReserved` (rung 2) → `os.ReadFile`. The reserved rejection returns before any read and carries no `Err`/`Detail`.
  - `internal/theme/load.go:117-120` — `isReserved` is a plain map lookup: exact equality, no folding.
  - `internal/theme/builtins.go:38-64` — `BuiltinSlugs()` derives slugs from the embedded `builtins/*.theme` filenames; `builtinSlugSet()` turns them into the reserved set. A new built-in file reserves its own slug with no Go edit.
  - `internal/theme/builtins.go:66-86` — `LoadBuiltin` skips the filename rungs and reads the embedded bytes, so a shadowing drop-in cannot change what the fallback returns.
  - `internal/theme/enumerate.go:86-99` — `classify` routes every directory candidate through `LoadFile`, so the enumerating path inherits the rung; a reserved entry keeps its slug for the row.
  - `internal/theme/loader_construction_guard_test.go:20-73` — later-phase source guard: no production file may hand-assemble a `theme.Loader` literal (only `NewLoader` is exempt), which closes the "a literal reserves nothing" hole structurally. Verified by grep: every production construction site (`cmd/open.go:486`, `cmd/theme.go:54`, `cmd/doctor_theme.go:52`, `cmd/capturetool/main.go:87`, `internal/capture/fixtures.go:422`, `internal/tui/builtin_themes.go:8`, `internal/themetest/builtin.go:16`) goes through `NewLoader`/`NewSilentLoader`.
- Notes:
  - Criterion "exact string equality, no `EqualFold`/lowercasing anywhere on the path" holds. The two remaining fold sites are off the slug path and spec-sanctioned: `enumerate.go:71-73` (`isCandidate`, deliberately looser so a mis-cased file is *visible* before being rejected, §5.6) and `name.go:83` (extension-cause classification only — the stem is never returned as a slug). `union.go:225` folds for display ordering, not identity.
  - "Build the set once per loader, not per file" is satisfied — `builtinSlugSet()` runs in the constructor; `LoadFile` does one map lookup.
  - Documentation "Do" items are met in substance: derivation is stated at `builtins.go:19-24` ("adding a theme is adding a file") and `builtins.go:38-40` ("Derived from the embedded filenames rather than a restated Go list"); the protected property is stated at `name.go:63-64` ("lowercasing a mis-cased name would let a user file shadow the built-in an invalid theme falls back to") and `load.go:13-16` / `load.go:114-116`. Comments carry no spec-section or task references, per the project standard.

TESTS:
- Status: Adequate (with some deliberate, plan-mandated overlap)
- Coverage — all seven named tests exist and assert the stated behaviour:
  - `TestLoadFile_ReservedSlugRejected` — `internal/theme/reserved_test.go:17` (valid contents, production loader).
  - `TestLoadFile_ReservedDecidedBeforeRead` — `reserved_test.go:25` (absent + `DenyRead` variants; asserts the fixture really is unreadable first, so the ordering claim is not vacuous, and asserts `rejection.Err == nil`).
  - `TestLoadFile_MixedCaseFilenameIsBadNameNotReserved` — `reserved_test.go:63` (loops every built-in slug × case variants; asserts `bad name` + the cause, and separately that `SlugFromFilename` yields no slug).
  - `TestReservedSet_CoversEveryBuiltinSlug` — `reserved_test.go:90` (loops `BuiltinSlugs()`, names no theme; `requireBuiltinSlugs` fails loudly on an empty set so the loop can't assert nothing).
  - `TestNoShadowing_BrokenDropInCannotReplaceBuiltin` — `reserved_test.go:104` (stages a broken-hex `tokyo-night.theme`, asserts the fixture differs from the embedded bytes, asserts `Enumerate` → `reserved name` with a zero `Theme`, then asserts `LoadBuiltin` returns the embedded bytes and not the drop-in's).
  - `TestLoadFile_RenamedCopyIsAccepted` — `reserved_test.go:158` (round-trips each built-in's own source to `<slug>-lee.theme` and compares tokens).
  - `TestLoadFile_EmptyInjectedReservedSetNeverRejects` — `internal/theme/load_test.go:177` (nil set and allocated-empty set × three files, including an invalid one, so the assertion is "never `reserved name`", not "never rejects").
  - Supporting: `TestNewSilentLoader_ReservesEveryBuiltinSlug` (`silent_loader_test.go:42`) pins that silence does not relax reservation; `TestLoader_HasNoProductionCompositeLiteral` (`loader_construction_guard_test.go:20`) pins that no production caller can bypass the set.
- Notes:
  - Would fail if the feature broke: dropping `builtinSlugSet()` from `NewLoader` breaks four tests; moving the reserved rung after the read breaks `TestLoadFile_ReservedDecidedBeforeRead` (both variants) and `TestLoadFile_LadderShortCircuits`'s "reserved name beats bad colour" case; lowercasing a filename breaks `TestLoadFile_MixedCaseFilenameIsBadNameNotReserved`.
  - Mild overlap: `TestLoadFile_ReservedSlugRejected` is a strict subset of `TestReservedSet_CoversEveryBuiltinSlug`, and `TestLoadFile_ReservedDecidedBeforeRead` repeats `TestLoadFile_ReservedNameDecidedFromSlugAlone` (`load_test.go:124`) over the production set rather than an injected one. Both duplications were explicitly requested by the plan (the rung test vs the populated-set test), so this is deliberate, not accidental bloat. No excessive mocking, no implementation-detail assertions.

CODE QUALITY:
- Project conventions: Followed. Small injectable seam on a value type, constructor-populated, DI without a framework; test-only helpers live in `internal/themetest`; source guards use `sourceguardtest`; comments carry no process artefacts.
- SOLID principles: Good. `Loader` stays the judge and resolves no paths; the reserved set is an injected collaborator with a real production default; `BuiltinSlugs()` is the single source of truth for both the embedded set and the reserved set, so the two cannot drift.
- Complexity: Low. `isReserved` is a one-line map lookup; `LoadFile` is a flat four-rung ladder.
- Modern idioms: Yes — `embed.FS` + `strings.TrimSuffix` + `slices.Sort`, `map[string]struct{}` set, `strings.CutSuffix` for exact-extension matching.
- Readability: Good. Each comment states *why* (rung order, exact equality, why a built-in skips the filename rungs) rather than restating the code.
- Comment accuracy: Verified — `load.go:13-16`, `load.go:28-30`, `load.go:39-42`, `load.go:114-116`, `builtins.go:38-40` and `builtins.go:66-70` all hold against the code.
- Security: No injection or traversal surface added; `ValidSlug` (`name.go:34-45`) still gates a slug before it is used as a path component, and `embed` cannot be escaped by a hostile slug.
- Performance: Set built once per loader; per-file cost is one map lookup. `BuiltinSlugs()` re-reads the embedded dir per loader construction — trivially cheap and bounded by three entries.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [bug] `internal/theme/resolve.go:44-63` — on a case-insensitive filesystem (macOS APFS default) the by-name path can accept a file the panel rejects: `loadFromThemesDir` composes `<themesDir>/<slug>.theme` and `LoadFile` derives the slug from that *composed* base, so a `Mine.theme` on disk is opened and accepted as slug `mine`, while `Enumerate` lists the same file as `bad name`. The no-shadowing property itself is unaffected (a built-in slug never reaches the directory — `resolve.go:32-34` consults the embedded set first), and spec §5.6 line 391 asserts the by-name path "looks for `<slug>.theme` and nothing else", which is what the code implements — so this is a spec-level edge rather than a coding mistake, and it is outside this task's criteria. Concrete change: after a successful `LoadFile` in `loadFromThemesDir`, confirm the on-disk entry name equals `slug+FileExtension` (one `os.ReadDir`/`filepath.Glob` name comparison) and return `notFound()` when it does not, so panel and launch cannot disagree about the same file.
- [quickfix] `internal/theme/reserved_test.go:17-23` — `TestLoadFile_ReservedSlugRejected` asserts exactly one case already covered by `TestReservedSet_CoversEveryBuiltinSlug` (`reserved_test.go:90`), which loops every built-in slug with the same valid fixture. Fold it away and let the enumerated loop carry the case (the plan mandated both names, so this is optional tidy-up rather than a defect).
- [quickfix] `internal/theme/load_test.go:124-161` and `internal/theme/reserved_test.go:25-61` — the same three states (valid contents / absent / unreadable) are asserted twice, once against an injected synthetic set and once against the production set. Merge into one table parameterised by loader so the ordering property is stated once.
