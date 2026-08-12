TASK: theming-system-2-8 — The Build-Time Guarantee: Embedded-Set Validity And Fallback-Slug Resolution

ACCEPTANCE CRITERIA (from plan):
- Every embedded theme parses and validates; the failure message names the slug and the reason with its detail.
- `DefaultDarkSlug` and `DefaultLightSlug` both resolve within the embedded set; renaming a built-in file without updating a constant fails this test.
- The test enumerates and names no theme, so a built-in added by a later PR is enrolled automatically.
- The suite fails if `builtins/` is empty — no vacuous pass.
- An embedded-shaped parse failure returns an ordinary `*Rejection`; no panic, and no fatal-message path exists in `internal/theme`.
- Nothing walks the embedded set at package init (task 2-1's guard), and no exported eager-validation helper exists.
- The constants are production symbols in `internal/theme`, not test-file literals — Phase 5 consumes them for both the shipped pair and the fallback.

STATUS: complete

SPEC CONTEXT:
Specification §7.6 ("The build-time guarantee", lines 661-676): there is no runtime fallback to hardcoded values beneath the built-in fallback; a unit test must (1) parse and validate every embedded built-in against §6.1's validity rule and (2) assert every fallback slug and the shipped default pair resolve within that set — both halves load-bearing, because a renamed file or a typo'd constant leaves half one green while every fallback path is unresolvable. Mechanism: the loader returns an *ordinary error* for an embedded parse failure, never a panic; the fatal escalation (`built-in theme <slug> is missing or invalid — this binary is broken`, §14A line 1866) is raised where a fallback is *needed*. Validation is explicitly **not** startup-eager — nothing walks the embedded set at init. §8.5's per-slot mode-matched fallback (`theme_dark` → `tokyo-night`, `theme_light` → `tokyo-night-day`, constant → `tokyo-night`) is what the constants encode; §13.6 line 1743 lists this as its own new test.

IMPLEMENTATION:
- Status: Implemented (with one acceptance criterion deliberately amended by a later plan task — see Notes).
- Location:
  - `internal/theme/builtins.go:9-15` — `DefaultDarkSlug = "tokyo-night"` / `DefaultLightSlug = "tokyo-night-day"` as production constants, with a doc comment stating they serve both the shipped adaptive pair and the per-slot mode-matched fallback (and that an unloadable constant `theme` falls to `DefaultDarkSlug`).
  - `internal/theme/builtins.go:41-55` — `BuiltinSlugs()` derives the set from the embedded filenames (no restated Go list), so enrolment of a new file is automatic.
  - `internal/theme/embedded_test.go:23-156` — the two halves plus the non-empty and enumerates-rather-than-names guards.
  - `internal/theme/embedded_test.go:158-222` — `TestEmbeddedRejection_HasNoFatalPathInThePackage`: no `panic` (bar the documented `NewLoader` nil-seam guard), no `os.Exit`, no `log.Fatal*` anywhere in the package's production sources, and the fatal sentence single-sourced to one file.
  - `internal/theme/load_internal_test.go:8-80` — the in-package (`package theme`) `TestEmbeddedParseFailureIsAnOrdinaryError` driving `parseThemeBytes`.
  - Consumers (AC7 satisfied): `internal/theme/resolution.go:211-216` (`defaultSlugFor`), `cmd/config.go:179-181`, `internal/tui/builtin_themes.go:8`, `internal/themetest/builtin.go:28,33`, `cmd/capturetool/main.go:33`.
- Notes:
  - **Deliberate later amendment, not drift.** AC5's clause "no fatal-message path exists in `internal/theme`" was superseded by task 5-6 (commit `0d43dfb3`), which homed `BrokenBuiltinError` in `internal/theme/broken_builtin.go:5-14`. The guard was amended in lockstep: the fatal copy is permitted in exactly one file (`fatalCopyOwner = "broken_builtin.go"`, `owners != 1` fails), and the no-panic / no-`os.Exit` / no-`log.Fatal` half — the part §7.6 actually specifies — is intact. The spec's "ordinary error returned up the normal path" is honoured: `BrokenBuiltinError` returns an `error`, it does not terminate.
  - **Deliberate later amendment, not drift.** The task's "Do" asked for a file-header comment recording that validation is not startup-eager. That header existed in the original 2-8 commit (`5141dcf4`) and was removed by the repo-wide comment-strip chore `25626754` ("strip internal/theme to the code-quality standard"), consistent with the review checklist's "no references to process artifacts". The *structural* claim survives untouched in `internal/theme/leaf_guard_test.go:91-128` (`TestThemePackage_HasNoInitFunction`, which also rejects any package-level `var` initialised by a function call). Verified independently: the only `BuiltinSlugs()` / `builtinSlugSet()` call sites are `NewLoader`, `Enumerate` and `capturetool` — all on-demand; nothing runs at init. No exported eager-validation helper exists (checked the package's full exported surface).
  - The constants' doc comment matches the code: `defaultSlugFor` returns `DefaultLightSlug` only for `SlotLight`, so `SlotConstant` and `SlotDark` both fall to `DefaultDarkSlug` (`resolution.go:211-216`), exactly as §8.5 specifies.
  - Half two genuinely catches a renamed file: `LoadBuiltin` resolves through `builtinFS.ReadFile(builtins/<slug>.theme)`, so a rename yields `found == false` and `TestFallbackSlugsResolveWithinEmbeddedSet` fails naming the slot and the unresolvable slug.

TESTS:
- Status: Adequate (one mild redundancy, one tautological assertion — both non-blocking).
- Coverage:
  - Half one — `TestEveryEmbeddedThemeIsValid` (`embedded_test.go:23`): enumerates `BuiltinSlugs()`, fatals on an empty set, loads each through the production `NewSilentLoader()`, asserts `found`, no `*Rejection`, full token count, every token populated, and every value in canonical upper-case `#RRGGBB` (a real assertion — `validate.go:55` is the `strings.ToUpper` canonicalisation being pinned). Failure message names the slug plus `rejection.Reason` and `rejection.Detail` as AC1 requires.
  - Half two — `TestFallbackSlugsResolveWithinEmbeddedSet` (`:61`) covers all three §8.5 slots (`theme_dark`, `theme_light`, constant), and `TestShippedDefaultPairResolves` (`:86`) covers the shipped pair plus the dark≠light distinctness the adaptive default depends on.
  - No-vacuous-pass — `TestEmbeddedSetIsNonEmpty` (`:114`), reinforced by in-test fatals at `:26` and `:123`. (Belt-and-braces: `//go:embed builtins/*.theme` already fails the build on an empty directory.)
  - Enrolment — `TestEmbeddedValidity_EnumeratesRatherThanNames` (`:120`) AST-scans this file's own string literals for any built-in slug and fails if one is named, with a `scanned == 0` self-check so it cannot pass having looked at nothing. Verified by inspection that no literal in the file currently contains a slug.
  - Ordinary-error — `TestEmbeddedParseFailureIsAnOrdinaryError` (`load_internal_test.go:8`) derives all three corruptions from the *real* shipped bytes (via `BuiltinBytes(DefaultDarkSlug)`, so it names no theme either), and asserts reason (`ReasonBadSyntax` / `ReasonBadColour` / `ReasonMissingTokens`), a non-empty detail, and a zero `Theme` alongside the rejection. Expected reasons check out against `lex.go:47-49` (duplicate key → bad syntax) and the ladder order. `firstTokenLine` fatals if the anchor line is not unique, so the corruptions cannot silently become no-ops.
- Notes:
  - Would fail if the feature broke: rename a built-in file → half two fails; corrupt a shipped value → half one fails; add an `init()` or an eager walk → `leaf_guard_test.go` fails; add a `panic`/`os.Exit` or a second copy of the fatal sentence → `TestEmbeddedRejection_HasNoFatalPathInThePackage` fails.
  - Mild over-test: `TestShippedDefaultPairResolves`'s two resolution subtests are fully subsumed by `TestFallbackSlugsResolveWithinEmbeddedSet`; only the distinctness assertion at `:109-111` is unique. The plan mandated both tests by name, so this is planned redundancy, not accidental.
  - `embedded_test.go:44-46` compares `len(result.Theme.All())` with `len(theme.TokenNames())`; both derive from the same `Theme.fields()` table, so the comparison cannot fail. See the non-blocking note.
  - Conventions: no `t.Parallel()` (repo rule), named subtests throughout, black-box `theme_test` package for the enumeration halves and white-box `package theme` for the `parseThemeBytes` half — exactly the split the task asked for.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays a leaf that resolves no paths (`leaf_guard_test.go`), declares no hex literals, and returns errors rather than terminating (CLAUDE.md's "bare `os.Exit` outside `main` is prohibited"). Test-file naming by concern (`embedded_test.go`, `leaf_guard_test.go`, `docs_guard_test.go`) is the established repo pattern for source guards and was specified by the plan.
- SOLID principles: Good. `BuiltinBytes` / `BuiltinSlugs` / `LoadBuiltin` each do one thing; the built-in path reuses the same `resultFromBytes` content tail as disk loads, so no second parser can drift.
- Complexity: Low. `BuiltinSlugs` is a read-dir + trim + sort; the AST guards are linear scans with early returns.
- Modern idioms: Yes — `slices.Sort`, `strings.SplitSeq` (Go 1.24+), `go:embed`, `cmp.Or` in the neighbouring resolution code.
- Readability: Good. Failure messages explain the design consequence ("every `theme_dark` fallback is unresolvable"), not just the mismatch.
- Comment accuracy: Verified. `builtins.go:9-11` (constants serve both the pair and the per-slot fallback; constant falls to dark) matches `resolution.go:211-216`. `builtins.go:44-45` ("go:embed fails the build when its pattern matches no file") is correct. `builtins.go:66-70` ("skips the ladder's filename rungs… touches no filesystem") matches `LoadBuiltin`. No spec-section, phase or task references remain in production or test comments.
- Security: N/A — embedded reads only; `embed` rejects `..`, and the code notes a hostile slug can only miss.
- Performance: `NewLoader` walks the embedded dir per construction via `builtinSlugSet()`; negligible (embed FS, 3 entries) and deliberately not hoisted to a package-level var, which the no-init guard forbids.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/embedded_test.go:145-149 — the enrolment guard matches slugs with a raw `strings.Contains` over every string literal in the file. Slug charset is `[a-z0-9-]`, so a future built-in whose slug is an ordinary word (`night`, `day`, `light`, `dark`) would false-positive on the file's own prose and fail with a confusing message the moment the file is dropped in. Replace the `strings.Contains` with a boundary-aware match, e.g. `regexp.MustCompile("(^|[^a-z0-9-])" + regexp.QuoteMeta(slug) + "([^a-z0-9-]|$)")`, which still catches `builtins/tokyo-night.theme`-style path literals.
- [quickfix] internal/theme/embedded_test.go:169 and :209-222 — `inNilSeamConstructor` exempts a `panic` from the no-fatal-path guard purely on the enclosing function being named `NewLoader`, in any file of the package. Anchor it to its owner as well (pass `source.Name` in and require `load.go`, mirroring the `fatalCopyOwner` pattern already used at :158/:194) so a `panic` added to a differently-located constructor of the same name is not silently exempt.
- [quickfix] internal/theme/embedded_test.go:30 and :44-46 — `wantTokens := len(theme.TokenNames())` compared against `len(result.Theme.All())` is a tautology: both lengths come from `Theme.fields()`, so the check can never fail. The same test package already declares the real pin (`tokenCount = 19`, theme_test.go:15) — use `wantTokens := tokenCount` so the assertion pins the closed vocabulary rather than comparing the table with itself.
- [quickfix] internal/theme/embedded_test.go:86-112 — `TestShippedDefaultPairResolves`'s two resolution subtests re-assert what `TestFallbackSlugsResolveWithinEmbeddedSet` (:61-84) already covers for the same two slugs; only the dark≠light distinctness check at :109-111 is unique. If the plan-mandated test name is kept, trim its body to the distinctness assertion so the duplicated resolution loop is not maintained twice.
