TASK: theming-system-11-10 — Single-Source The Silent Theme Loader And The Shipped Dark Default Slug (tick-70c515)

ACCEPTANCE CRITERIA:
1. `theme.NewLoader(theme.NewEventLogger(log.Discard()))` is written exactly once in the repo.
2. No two packages define `newThemeLoader` with different behaviour.
3. `capturetool` has no `"tokyo-night"` literal; its default flows from `theme.DefaultDarkSlug`.
4. Doctor, export and capturetool still emit no `theme` log lines on their diagnose paths.

STATUS: complete

SPEC CONTEXT: The `theme` log component is a closed, spec-governed vocabulary that records where a theme is *used*, never where one is *diagnosed* — hence `portal doctor`, `portal theme export` and `capturetool` construct their loader over a discarding seam. The `theme` package takes an injected `EventLogger` (which also owns per-process WARN dedup state) and hardcodes no slugs; `internal/theme/builtins.go` names the shipped pair (`DefaultDarkSlug`/`DefaultLightSlug`) and derives `BuiltinSlugs()` from the embedded directory rather than restating it — the same single-definition reasoning this task extends to capturetool's capture default. Both properties are recorded in CLAUDE.md (`theme` package row, logging taxonomy).

IMPLEMENTATION:
- Status: Implemented (commit 738027a2)
- Location:
  - `internal/theme/load.go:39-45` — `NewSilentLoader()` = `NewLoader(NewEventLogger(log.Discard()))`, the sole occurrence of that construction in production.
  - `cmd/theme.go:54` (`resolveThemeSource`), `cmd/doctor_theme.go:52` (`themeAdvisoryUnion`, renamed from `collectThemeAdvisories` by a later phase — still routed through the constructor), `cmd/capturetool/main.go:87` (`resolveProgram`) — all three sites route through it; each dropped its `internal/log` import.
  - `cmd/capturetool/main.go:33` — `const defaultThemeSlug = theme.DefaultDarkSlug`; `main.go:37` interpolates the constant into the `--theme` usage string rather than restating `tokyo-night`.
  - capturetool's package-local `newThemeLoader` is deleted (was `main.go:160`); `cmd/open.go:485` is now the repo's only `newThemeLoader`, and it is the emitting one. `cmd/open.go:489 buildThemeLoader` is a distinctly-named deps-override wrapper, not a competing definition.
  - `themeFileExtension` left alone as instructed (`cmd/capturetool/main.go:~54`).
- Notes:
  - AC1 holds for production. One test-side occurrence exists at `internal/theme/events_test.go:696`, inside `TestEvents_DiscardSilencesResolution`'s equivalence table (silent constructor / discard-backed seam / nil logger / zero-value loader all silent). That is the construction *under test*, not a duplicated call site — correct to keep.
  - Import legality confirmed: `internal/theme` already imported `internal/log` (`events.go` uses `log.OrDiscard`), so no new dependency edge; the prohibition runs the other way (`internal/log` must not import `internal/state`). `internal/theme/leaf_guard_test.go` constrains only xdg/paths/hex/init, none of which this touches, and `internal/log/discard_guard_test.go` (`TestNoDiscardLoggerConstructionInProductionSource`) is satisfied because the constructor calls `log.Discard()` rather than building a `*slog.Logger`.
  - Later phases hardened and extended the outcome rather than superseding it: `NewLoader(nil)` now panics naming `NewSilentLoader` (`load.go:31-37`), `internal/theme/loader_construction_guard_test.go` fails any production `theme.Loader` composite literal outside `NewLoader`, and three further sites adopted the constructor (`internal/tui/builtin_themes.go:8`, `internal/capture/fixtures.go:422`, `internal/themetest/builtin.go:16`). The silent shape is now structurally the only route to a silenced production loader.

TESTS:
- Status: Adequate
- Coverage:
  - `internal/theme/silent_loader_test.go:14-40` — the required parity+silence test: enumerates a directory staged to span the ladder (valid / bad name / bad colour / reserved) with the silent loader, asserts zero records, then re-enumerates with an emitting loader and asserts (a) it *did* write, so the silence assertion is not vacuous, and (b) the two verdict maps are equal. It also pins the fixture's expected verdict set (`stagedVerdicts`, :70-77) so a fixture that stops spanning the ladder fails loudly instead of quietly narrowing the parity claim.
  - `internal/theme/silent_loader_test.go:42-54` — proves silence did not cost slug reservation (every built-in slug still rejected as `ReasonReservedName`), covering the constructor's stated "silence is about emission only" invariant.
  - `cmd/capturetool/main_test.go:75-95` — the required default test: asserts the value equals `theme.DefaultDarkSlug` *and* (via `declaredConstSource`, an AST read of main.go) that the constant is *declared as* `theme.DefaultDarkSlug`, which is what actually forbids a restated literal that happens to match today. Then resolves the no-flag default and compares against the embedded palette. `TestFlags_AreFixtureAndThemeOnly:37` closes the loop by asserting the flag's default expression is the constant.
  - AC4 silence, all three surfaces, each with a positive control that makes the zero-record assertion non-vacuous: capturetool `main_test.go:458-487` (drives four resolution outcomes — default slug, reserved-name file, unknown slug, broken file — then emits a real `theme` record to prove the sink is wired); export `cmd/theme_test.go:~390` (`assertNoThemeRecords` re-arms a live sink and emits through `log.For("theme")`); doctor `cmd/doctor_persisted_theme_test.go:692-729` (a "the condition emits through a real component logger" subtest first proves the fixture reaches an emitting condition, then a full `runDoctorCmd` asserts both advisories rendered *and* zero theme records).
- Notes: Not over-tested — each assertion covers a distinct failure mode (value, declaration form, judgement parity, emission, reservation), and the silence tests are shared/parameterised rather than repeated per call site. No redundant duplication found. One small weakness noted below in `swatch_test.go`.

CODE QUALITY:
- Project conventions: Followed. Log ownership respected (`log.Discard()`, no `*slog.Logger` construction); the `theme` component stays bound in `cmd`; `internal/theme` still resolves no paths and hardcodes no slug; the constructor sits beside `NewLoader` in `load.go`.
- SOLID principles: Good. The constructor packages a caller-owned dependency's silent configuration without the package deciding anything about logging; the `EventLogger` seam is unchanged.
- Complexity: Low — a two-line constructor and three one-line call-site swaps.
- Modern idioms: Yes. `New*` constructor naming, constant aliasing an exported constant, usage string built by concatenation from the constant.
- Readability: Good. The doc comment on `NewSilentLoader` states the three things a reader needs (why a constructor and not a local shape, why reservation survives silencing, why `log.Discard()` specifically) without restating the code; the three call sites each carry a one-line "the loader is the silent one" rationale, and `cmd/capturetool/main.go:31-32` says why the default is read off rather than restated.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/events.go:11-15 — the header comment ends "…this package takes an injected logger, and diagnose-shaped callers inject log.Discard()", which the code no longer does: those callers take `NewSilentLoader`, which does the injection. (This commit wrote the accurate wording; a later comment-trimming pass reverted it to the pre-task phrasing, dropping the only pointer to the constructor this task created.) Replace the trailing clause with: "and a diagnose-shaped caller takes NewSilentLoader, whose seam carries log.Discard()."
- [quickfix] cmd/capturetool/swatch_test.go:15,23 — the "a built-in slug pins the swatch" subtest passes `"tokyo-night"`, which is now exactly `defaultThemeSlug`, so it cannot distinguish "honours the slug argument" from "ignores it and loads the shipped dark default"; it is also the last `"tokyo-night"` literal in capturetool that carries meaning tied to the default. Change both the argument and the `themetest.Builtin` expectation to a non-default built-in (`"nord"`), which strengthens the assertion and removes the coincidental coupling.
