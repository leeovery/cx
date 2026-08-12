TASK: theming-system-14-12 — Make `NewSilentLoader` The Only Route To A Silent Theme Loader (tick-c55836, severity low, source: architecture)

ACCEPTANCE CRITERIA:
1. `NewLoader(nil)` is not constructible without a loud failure; `grep -rn "NewLoader(nil)"` returns nothing.
2. Every silent loader in the repo is constructed through `NewSilentLoader`.
3. The built-in seed still resolves the shipped dark palette and still writes no records.
4. `go build ./... && go test ./...` pass; `golangci-lint run` is clean.

STATUS: complete

SPEC CONTEXT:
The specification does not name the constructor pair — the governing contract is the `theme` log component's emission policy recorded in CLAUDE.md: the component "records where a theme is *used*, never where one is *diagnosed*", so `portal doctor`, `portal theme export` and `capturetool` construct the loader with `log.Discard()` and write nothing. The `theme` component is spec-governed (closed component/attr vocabulary, bound once per package, `cmd` holding the single `log.For("theme")` binding). The task is an architecture-quality change to that emission policy's *auditability*: silence must be a named, greppable decision rather than an accidental nil, without changing what any loader judges (reservation, the six-rung rejection ladder, and every emission that does happen stay as-is).

IMPLEMENTATION:
- Status: Implemented (commit 42bb0e2e), with later phases (17-5 fixture ownership, 915e7fcb comment audit) touching the same files — supersession, not drift.
- Location:
  - `internal/theme/load.go:28-37` — `NewLoader` panics on a nil `*EventLogger` with a message naming `theme.NewSilentLoader`; the doc comment states the reason ("silence must be readable at the call site as NewSilentLoader, not an accidental nil").
  - `internal/theme/load.go:39-45` — `NewSilentLoader` now routes *through* `NewLoader(NewEventLogger(log.Discard()))`, so the two constructors cannot drift in reservation semantics by construction.
  - `internal/theme/load.go:12-26` — the `ReservedSlugs` doc states the zero-value/`NewLoader`/`NewSilentLoader` distinction ("the zero value reserves nothing…"), satisfying Do-item 4.
  - `internal/tui/builtin_themes.go:5-9` — the dark seed takes `theme.NewSilentLoader()`; the comment states *why* ("Seeding is not a 'use', hence the silent loader").
  - `internal/themetest/builtin.go:16` — re-pointed.
  - Test call sites re-pointed: `internal/tui/theme_panel_behaviour_test.go:25`, `theme_row_test.go:390`, `theme_testing_test.go:50,59,91,370`, `theme_panel_commit_load_test.go:61,595`, `theme_panel_confirm_test.go:736`, `internal/theme/resolution_test.go:21`, `embedded_test.go:20`.
  - `internal/theme/embedded_test.go:160-222` — the package's "no fatal path" guard gained a by-name, single-name exemption (`inNilSeamConstructor`) so the sanctioned panic does not defeat the guard for every other function.
- Notes:
  - Criterion 1 holds. `grep -rn "NewLoader(nil)"` returns exactly one hit, `internal/theme/silent_loader_test.go:67`, which is the test asserting the panic — the only construction that *must* remain. Production has one emitting loader (`cmd/open.go:486`, via `NewEventLogger`, which never returns nil — `internal/theme/events.go:49-51`) and five silent ones, all `NewSilentLoader` (`cmd/theme.go:54`, `cmd/doctor_theme.go:52`, `cmd/capturetool/main.go:87`, `internal/capture/fixtures.go:422`, `internal/tui/builtin_themes.go:8`). The stated Outcome — one grep answers "where does Portal deliberately write no `theme` records" — is achieved for production.
  - Criterion 2 holds for production; two *test* helpers still reach silence unnamed via `theme.NewLoader(theme.NewEventLogger(nil))` (`internal/theme/reserved_test.go:189-191`, `cmd/open_theme_construction_test.go:383`). Behaviourally identical to `NewSilentLoader` (`log.OrDiscard(nil)` → discard), so the criterion is not *literally* met repo-wide. Non-blocking: neither was in the task's enumerated Do-list (which named the `NewLoader(nil)` sites), and neither affects production auditability. Noted below.
  - The panic route was taken over the type-level route; the task explicitly permitted it ("prefer the type-level route if it does not force churn"). The type-level route is in fact unavailable here: `EventLogger` carries a `sync.Mutex` + shared dedup map (`events.go:32-37`) and the pointer is load-bearing so every by-value copy of a `Loader` shares one dedup set — taking it by value would copy a mutex (vet failure) and split the dedup state. The choice is correct and is justified in-source.
  - Panic-on-programmer-error is compatible with the project's error-handling skill rule ("NEVER use `panic` for expected error conditions — reserve for truly unrecoverable states"): a nil seam is API misuse at a compile-visible call site, not a runtime condition, and no production path can reach it.
  - Reservation, the rejection ladder and emission behaviour are untouched — `NewSilentLoader` delegates to `NewLoader`, so both populate `ReservedSlugs` from `builtinSlugSet()` identically. Do-item 5 respected.
  - Guard coverage is intact and unweakened: `internal/theme/loader_construction_guard_test.go` still forbids any production `theme.Loader` composite literal (with its own `exempted != 1` non-vacuity counter), and no production `DirThemeSource`/`Assembler` literal leaves `Loader` zero (`cmd/theme_source.go:11` takes the constructed loader; `internal/theme/dir_theme_source.go:42` forwards it).

TESTS:
- Status: Adequate.
- Coverage:
  - `internal/theme/silent_loader_test.go:56-68` `TestNewLoader_NilSeamPanics` — pins criterion 1, and asserts the panic *value names `NewSilentLoader`*, so the message stays a signpost rather than a bare crash. Would fail if the guard clause were removed.
  - `internal/theme/silent_loader_test.go:42-54` `TestNewSilentLoader_ReservesEveryBuiltinSlug` — the reservation test the task asked for; enumerates `theme.BuiltinSlugs()` (with `requireBuiltinSlugs` fatalling on an empty set, so the loop cannot assert nothing) and requires `ReasonReservedName` for a user file taking each built-in slug.
  - `internal/tui/builtin_themes_test.go:13-34` `TestDefaultDarkTheme_SeedsTheShippedPaletteSilently` — the seed test the task asked for; asserts both halves (palette equals `themetest.DefaultDark`, sink holds zero records) and carries the requested vacuity guard: a second, *emitting* loader over a staged bad-name directory must reach the same process sink, otherwise the silence assertion is declared unwired. Would fail on either half regressing.
  - `internal/theme/silent_loader_test.go:14-40` — parity of verdicts between the silent and emitting loaders across a directory spanning the ladder (valid / bad-name / bad-colour / reserved), with `stagedVerdicts()` asserted first so the parity comparison cannot silently cover less than it reads. This is what pins Do-item 5 ("do not change what any loader judges").
  - Regression safety for the re-pointed sites is carried by the existing loader/resolution/emission suites, which compile against the new constructor.
- Notes:
  - Not over-tested. The two silence assertions (`silent_loader_test.go` and `builtin_themes_test.go`) sit at different layers — loader parity vs. the TUI seed call site — and the seed one is the only thing that would catch `builtin_themes.go` regressing to an emitting loader.
  - `internal/theme/events_test.go:693-699` deliberately retains the *equivalence table* over four silent shapes (silent constructor / discard seam / nil `*slog.Logger` / zero-value `Loader`). That is intentional coverage that the alternative silences do not panic on paths tests drive, not a violation of criterion 2.
  - Criterion 4 (`go build`/`go test`/`golangci-lint`) was assessed by reading only — running the suite is out of scope for this review. Nothing read suggests a compile or lint problem: all re-pointed helpers exist (`themetest.Write`, `themetest.MonochromeLines`, `themetest.Lines`), and `internal/tui/builtin_themes_test.go:28`'s `log.For("theme")` does not trip `internal/tui`'s component-binding guard, which scans production files only (`sourceguardtest.PackageGoFiles(".", false)` via `nomination_test.go:252-273`).

CODE QUALITY:
- Project conventions: Followed. Test-only helpers stay test-only; `internal/theme` still emits solely through the injected `EventLogger` seam and binds no component name; the sanctioned panic is explicitly carved out of the package's no-fatal-path guard rather than left to silently coexist with it.
- SOLID principles: Good. `NewSilentLoader` delegating to `NewLoader` makes the two constructors one source of reservation truth; the seam remains injected rather than resolved internally.
- Complexity: Low. One guard clause plus one delegation; the AST exemption helper is a linear scan with a single early return.
- Modern idioms: Yes. No opportunities missed.
- Readability: Good. Both constructors state their contract and its rationale in a doc comment; the panic message points at the remedy.
- Comment accuracy: Accurate, with one soft edge — `internal/theme/load.go:23-25` still asserts "nil is a valid silent seam" on the `events` field. True of the zero-value `Loader`, but read against the constructor three lines below it now reads as licence for the construction this task outlawed.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/load.go:23-25 — the `events` field comment ends "nil is a valid silent seam", which reads as licence for `NewLoader(nil)` immediately above the clause that panics on it. Replace the field comment with: "// A pointer so every copy of a Loader (used by value) shares one dedup set. A\n// nil seam is silent, which is why NewLoader rejects one: the zero-value Loader\n// is the only shape that legitimately carries it."
- [quickfix] internal/theme/reserved_test.go:189-191 — `productionLoader()` returns `theme.NewLoader(theme.NewEventLogger(nil))`, a silent loader reached by an unnamed route (the exact shape criterion 2 sets out to eliminate), under a name that implies the emitting production loader. Change the body to `return theme.NewSilentLoader()` (identical reservation, identical silence) and rename the helper to `reservingLoader`.
- [quickfix] cmd/open_theme_construction_test.go:381-390 — `brokenBuiltinLoader` builds the same unnamed silent shape via `theme.NewLoader(theme.NewEventLogger(nil))`. Change that line to `loader := theme.NewSilentLoader()`, keeping the `BuiltinSource` override that follows; that leaves `silent_loader_test.go:67` (the panic assertion) and `events_test.go`'s deliberate equivalence table as the only non-`NewSilentLoader` silent constructions in the repo.
- [quickfix] internal/theme/embedded_test.go:160-207 — the panic exemption has no non-vacuity counter, unlike its sibling guard (`loader_construction_guard_test.go:70-72` asserts `exempted != 1`). Count the calls where `inNilSeamConstructor` returns true and fail when that count is not exactly 1, with a message saying the exemption names a construction that no longer exists.
- [idea] internal/theme/loader_construction_guard_test.go — `NewLoader` now rejects a literal nil seam, but a future production call site could still reach unnamed silence through `theme.NewEventLogger(nil)` or `theme.NewEventLogger(log.Discard())`, which the grep in criterion 1 would not surface. Decide whether to extend this production-only scan to reject those two argument shapes outside `NewSilentLoader`; that would make the criterion's outcome ("one grep answers where Portal writes no `theme` records") structurally enforced rather than currently-true.
