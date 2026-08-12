TASK: theming-system-1-8 — The `theme` Log Component Behind An Injected Logger Seam

ACCEPTANCE CRITERIA:
- `NewEventLogger(log.Discard())` produces zero records for any sequence of calls, including a full reject set.
- `NewEventLogger(nil)` does not panic and produces zero records.
- Five successive `Enumerate` calls over the same broken directory emit exactly one `rejected` per distinct slug+reason and exactly one `directory unusable` per path+reason.
- The same slug reported with a different reason emits a second record.
- A `bad name` file (no slug) dedups on `path`+`reason` and carries a `path` attr, never an empty `slug`.
- Two separately constructed `EventLogger`s each emit once for the same input — dedup state is per-instance, not package state.
- An absent directory emits nothing at all.
- Every record is `WARN` with component `theme`, and every attr key used is drawn from the closed seven.
- `token` is present for `missing tokens` and `bad colour` and absent for every other reason.
- Concurrent calls from two goroutines do not race.

STATUS: complete

SPEC CONTEXT:
§12.3 adds `theme` to the closed, spec-governed log-component vocabulary (precedent: `spawn`, `resolve`) and pins the catalogue: `rejected` and `directory unusable` are WARN and deduplicated per process — `slug`+`reason` where a slug exists, `path`+`reason` where it does not (the `bad name` class); `token` is carried only where the reason names one, and this is that attr's only consumer; an *absent* directory emits nothing. Emission is controlled by an **injected** logger, not by the loader deciding: `cmd` passes a real component logger where a theme is *used* and `log.Discard` on `portal doctor` / `portal theme export` / `capturetool`, because "the component records where a theme is used, never where one is diagnosed" and doctor's read-only claim must stay literal. The per-process dedup state lives on the injected seam (not package state in the leaf) so the construction-time by-name read (§5.5) and the panel's re-reading enumeration (§5.8) cannot double up, while §8.9's concurrent burst processes each own their own; a test controls it by injecting a fresh one. Closed attr keys: `slug`, `slot`, `reason`, `path`, `token`, `count`, `rejected`.

IMPLEMENTATION:
- Status: Implemented (later phases extended the same seam as planned — Phase 5 added `Loaded`/`FallbackApplied`, Phase 8 added `Enumerated`; the Phase 11/13/17 comment-and-shape remediations rewrote the file's comments and the `token` rendering. This task's mechanism survives intact.)
- Location:
  - `/Users/leeovery/Code/portal/internal/theme/events.go:32-51` — `EventLogger` (injected `*slog.Logger`, instance-owned `seen` map behind a `sync.Mutex`), `NewEventLogger` guarding nil through `log.OrDiscard`.
  - `events.go:91-110` — `Rejected`: identity attr `slug`, or `path` when the file yields no slug; dedup key `(event, identity, reason)`; `token` appended only where `tokenAttr` says the reason names one.
  - `events.go:115-124` — `DirectoryUnusable`: WARN, `path` + `reason`, dedup on `path`+`reason`.
  - `events.go:128-137` — `firstSighting`: check-and-record in one critical section.
  - `events.go:154-163` — `tokenAttr`: comma-joined list for `missing tokens`, comma-joined `name = value` pairs for `bad colour` (rendered from the structured `Rejection.Tokens`/`Values` after task 13-7/17-9, never re-parsed from `Detail`), `(",", false)` for every other reason.
  - `/Users/leeovery/Code/portal/internal/theme/load.go:23-45` — the seam threaded onto `Loader` as a **pointer** field (so every by-value copy of a `Loader` shares one dedup set), `NewLoader(events)` panicking on a nil seam, `NewSilentLoader()` = `NewLoader(NewEventLogger(log.Discard()))`.
  - `/Users/leeovery/Code/portal/internal/theme/enumerate.go:28-52` — emission sites: one `DirectoryUnusable` for the directory verdict, one `Rejected` per rejected entry, and nothing on the absent-directory path (`statThemeDir` returns `(false, nil)` for ENOENT, so `Enumerate` returns before either call).
- Caller wiring holds the spec's contract end-to-end: `cmd/open.go:27` is the single `log.For("theme")` binding in `cmd`, `cmd/open.go:485-494` builds one loader per launch, and `cmd/open.go:610` + `themeSource: newThemeSource(themeLoader)` hand that **same** loader to both the construction-time resolution and the panel's enumerator — so §12.3's "shared by every path in a TUI process" dedup scope is real, not nominal. Every diagnose-shaped caller takes `NewSilentLoader()`: `cmd/doctor_theme.go:52`, `cmd/theme.go:54` (export), `cmd/capturetool/main.go:87`, plus `internal/capture/fixtures.go:422` and `internal/tui/builtin_themes.go:8`.
- Notes: the task's "Do" bullet asked for the seven attr keys and the five not-yet-implemented events to be enumerated in a source comment naming their phases. The current comment (`events.go:11-15`) keeps the closed/spec-governed statement and the "bound by the caller" rule but no longer lists the keys or the phases. That is the deliberate outcome of the later comment remediations (`e30939b2` task 11-3 stripped phase/spec citations from production comments; `25626754`/`915e7fcb` stripped restated content) and of Phases 5/8 actually implementing the named events — superseded intent, not drift. The closed key set is now pinned executably by `closedAttrKeys` in `events_test.go:18`.

TESTS:
- Status: Adequate
- Coverage: `/Users/leeovery/Code/portal/internal/theme/events_test.go` carries all twelve tests the task named, each mapping to an acceptance criterion:
  - discard silence (`:168`) — drives the full seven-reason reject set twice *and* two real `Enumerate` passes over a staged three-file broken directory, asserting zero records through an installed `logtest.Sink`; nil-logger safety (`:200`).
  - dedup across five real enumerations (`:212`), path-keyed dedup for the slug-less `Nord.THEME` case with an explicit "carries no `slug` attr" assertion (`:251`), directory-unusable dedup over a `themetest.DenyDir` fixture (`:279`), same-slug-different-reason emitting twice (`:304`), fresh-instance-fresh-state (`:326`), concurrent emission converging on exactly 3 records (`:341`).
  - absent directory emitting nothing across five enumerations (`:363`).
  - WARN level + `component=theme` (`:20`), `token` presence across all seven reasons in a table (`:48`), closed-attr-key set with exact per-record key ordering (`:138`).
  - `TestEventLogger_TokenAttrRendersFromTokensNotDetail` (`:97`) is the later 13-7/17-9 addition pinning that `token` is composed from the structured fields, not scraped from reworded `Detail` copy.
  - `silent_loader_test.go:14` additionally proves the silent loader judges *identically* to the emitting one and that the silence assertion is non-vacuous (it fails if the emitting loader writes nothing over the same directory).
  Tests would fail if the feature broke: they assert record counts, message strings, level, component, exact attr-key sets and dedup arithmetic, and the five-iteration loops are what make dedup (rather than "emitted once because called once") the thing under test.
- Notes: no over-testing found. The overlap between `TokenAttrOnlyWhereReasonNamesOne` and `TokenAttrRendersFromTokensNotDetail` is justified — the first covers presence/absence across all seven reasons, the second pins the source of the rendered value. The race criterion is a real concurrent driver (2 goroutines × 50 iterations × 3 calls) rather than a `-race` claim in prose. One test-fixture shape nit is in the notes below.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays a leaf that binds no component (`log.For` appears nowhere in the package; the single `theme` binding lives at `cmd/open.go:27`), resolves no paths and reads no env (`leaf_guard_test.go` enforces both), and declares no hex literals. `NewLoader`'s nil-seam panic plus `loader_construction_guard_test.go` make silence readable at the call site (`NewSilentLoader`) rather than an accidental nil — a stronger guarantee than the task asked for.
- SOLID principles: Good. The seam owns emission + dedup and nothing else; the loader emits but never decides; `cmd` decides but never dedups. Dependency inversion is via the injected `*slog.Logger`, matching the `internal/spawn` precedent the task cited.
- Complexity: Low. Every method is a nil-receiver guard, a dedup check and one emit; `firstSighting` is the only shared state and is a single locked check-and-set.
- Modern idioms: Yes — `map[eventKey]struct{}` keyed on a comparable struct, `log.OrDiscard` at entry, `wg.Go` in the concurrency test, `for range N` loops.
- Readability: Good. Intent is stated where it is non-obvious (why the dedup state is on the instance, why `Loader.events` is a pointer, why `tokenAttr` never parses `Detail`).
- Issues: two comment/structure nits in `events.go`, both non-blocking and both on lines introduced by the later phases that extended this file. See below.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/events.go:17-18 — the comment "Each constant is used twice: as the message and as the dedup key's discriminator." is falsified by the file: `eventLoaded` (`:60`) and `eventEnumerated` (`:71`) are used once each, as messages only, because neither event is deduplicated. Replace with: "// The message for each event. The deduplicated events reuse theirs as the dedup key's discriminator, so the two can never drift."
- [quickfix] internal/theme/events.go:146-148 — `slotAttr` is a single-caller pass-through returning `slot.AttrName()` verbatim. Inline it at `themeAttrs` (`:140`) as `if name, named := slot.AttrName(); named {` and delete the wrapper.
- [quickfix] internal/theme/events_test.go:142 — the `bad colour` fixture in `TestEventLogger_AttrKeysAreInTheClosedSet` carries no `Tokens`/`Values`, so the record it asserts on has `token=""`, a shape production never emits (`validate.go:61-66` always populates both lists for this reason). Give the fixture `Tokens: []string{"canvas"}, Values: []string{"blue"}` so the closed-key assertion runs over a production-shaped record.
