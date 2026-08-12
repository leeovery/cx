TASK: theming-system-16-8 — Initialise The Five Nil Slice Accumulators Explicitly (tick-9992b8, severity low, source: standards)

ACCEPTANCE CRITERIA:
- The five accumulators are explicitly initialised, with capacity preallocated where the bound is known.
- No caller or test distinguishes a nil result from an empty one.
- Enumeration, union assembly, in-force key derivation and doctor's advisory output are behaviourally unchanged.
- `go build ./... && go test ./...` pass; `golangci-lint run` is clean.

STATUS: complete

SPEC CONTEXT: This is a standards-sourced consistency task, not a spec-behaviour task — the specification says nothing about slice nil-vs-empty shape (grep for "nil" in `.workflows/theming-system/specification/theming-system/specification.md` returns no hits). The governing authority is `.claude/skills/golang-code-style/SKILL.md:62-73` ("Slices and maps MUST be initialized explicitly, never nil"; `make([]T, 0, len(ids))` where the capacity is known; "Do not preallocate speculatively"). The surrounding feature context is the theme enumeration/union/doctor-advisory pipeline: `Loader.Enumerate` classifies a themes directory, `Assembler.Reassemble` composes built-in + file + persisted rows, `InForceKeys` selects the prefs keys a surface reports on, and `cmd/doctor_theme.go` renders the advisory block beneath doctor's check catalog.

IMPLEMENTATION:
- Status: Implemented (commit `b65b8cdd`, verified against HEAD)
- Location:
  - `internal/theme/enumerate.go:35` — `entries := make([]Entry, 0, len(dirEntries))`
  - `internal/theme/union.go:177-179` — `inForce := InForceKeys(keys)` hoisted, then `rows := make([]Row, 0, len(inForce))`
  - `internal/theme/setting.go:136` — `inForce := make([]InForceKey, 0, 2)`
  - `cmd/doctor_theme.go:99-101` — `nominations := persistedThemeNominations(raw)` hoisted, then `advisories := make([]themeAdvisory, 0, len(nominations))`
  - `cmd/doctor_theme.go:163` — `advisories := make([]themeAdvisory, 0, len(enumeration.Entries))`
- Notes:
  - All five sites converted; a HEAD-wide scan of `internal/theme/*.go` and `cmd/doctor_theme.go` finds no remaining `var xs []T` accumulator, so nothing regressed under the later phase-17 refactors (line numbers moved — `doctor_theme.go`'s two sites are now :101/:163, not :252/:372 — because subsequent tasks reordered the file; the edits themselves survived intact).
  - No sweep beyond the five sites, as instructed.
  - Both hoists are strict improvements, not just bound-plumbing: each previously-inline call is now made exactly once and named, which is what makes the capacity available.
  - Preallocation bounds are real upper bounds, not speculative: `len(dirEntries)` (directory entries, superset of `.theme` candidates), `len(inForce)` (≤2), the literal `2` (the light/dark slot pair — the only two entries this branch can append), `len(nominations)` and `len(enumeration.Entries)`. None trips the skill's "do not preallocate speculatively" caveat.
  - Behaviour: no control flow changed at any site; every edit is a declaration form plus a hoist of an already-single call. `make([]T, 0, 0)` is non-nil by language guarantee, so the zero-bound cases (`Enumeration{}`, `RawKeys{}`) return empty-not-nil as intended.
  - Nil-vs-empty caller audit (task Do item 5) — verified independently, all clear:
    - `Enumerate` has exactly one production caller, `OpenEnumeration` (`enumerate.go:63-64`), which discriminates on `rejection != nil`, never on the slice.
    - `InForceKeys` callers (`union.go:177`, `cmd/doctor_theme.go:116`) only range.
    - `persistedRows` result is `append`ed at `union.go:134`.
    - Both advisory producers feed `assembleThemeAdvisories` (`cmd/doctor_theme.go:55, 61-71`), which only ranges/appends; `renderDoctorReport` (`cmd/doctor.go:455`) ranges and `doctorSummaryLine` (`cmd/doctor.go:510`) takes `len`.
    - Repo-wide grep for `Entries == nil` / `Rows == nil` / `entries != nil` etc. finds only the new assertion in `enumerate_test.go:401`. No pre-existing assertion pinned nil, so no assertion had to be flipped — the task's "if any assertion pins nil, say so in the report" clause is vacuous here, correctly reported as nothing to declare.
    - The three `reflect.DeepEqual` sites that could have been shape-sensitive (`cmd/doctor_theme_enumeration_test.go:20`, `internal/theme/enumerate_test.go:455/458`, `internal/capture/theme_panel_fixture_test.go:106`) compare production-derived values on *both* sides, so the shape change moves both operands together. The `slices.Equal` assertions in `setting_test.go`/`union_test.go` treat nil and empty as equal by definition.

TESTS:
- Status: Adequate
- Coverage: One targeted test per accumulator, five in total, each asserting both non-nil and zero-length:
  - `internal/theme/enumerate_test.go:393` `TestEnumerate_UsableDirectoryWithNoCandidatesIsEmptyNotNil` (readable temp dir, no candidates)
  - `internal/theme/union_internal_test.go:31` `TestPersistedRows_NothingContributedIsEmptyNotNil` (internal test — the function is unexported, correctly placed)
  - `internal/theme/setting_test.go:636` `TestInForceKeys_NoKeySetIsEmptyNotNil`
  - `cmd/doctor_persisted_theme_test.go:738` `TestPersistedThemeAdvisories_NothingToReportIsEmptyNotNil` (a resolvable built-in slug ⇒ no advisory)
  - `cmd/doctor_persisted_theme_test.go:747` `TestScanThemesDirectory_NothingToReportIsEmptyNotNil` (zero-value `theme.Enumeration`)
  - Acceptance criterion 3's "doctor's rendered theme block is byte-identical with no advisories" is covered by the pre-existing `cmd/doctor_advisory_test.go:281` `TestAdvisories_EmptyBlockRendersNothing`, which renders the report with `nil` and with `[]advisory{}` and asserts both against the same golden string *and* against each other. Nothing new was needed, and adding a second byte-identity test would have been redundant — the right call.
- Notes:
  - Each test would fail if the change were reverted (the `== nil` arm fires), so they are load-bearing rather than decorative.
  - Not over-tested: no test enumerates permutations of a shape-only property, and the cmd tests reuse the file's existing `persistedAdvisoriesFor`/`requireNoAdvisories` helpers rather than restating fixture setup.
  - Not under-tested: every one of the five sites has its own pin, so a future refactor cannot silently re-nil one of them.
  - The two cmd tests' doc comments were added by this task's commit and later stripped by `a4bc7bd5 chore(comments): strip cmd to the code-quality standard` — a deliberate later supersession, not drift. The remaining test names are self-describing (`..._NothingToReportIsEmptyNotNil`), so nothing was lost.
  - No test executed as part of this review (verification is by reading, per the review protocol).

CODE QUALITY:
- Project conventions: Followed. Conforms to `golang-code-style` §"Slice & Map Initialization" exactly, including the "preallocate when capacity is known" clause and avoiding speculative capacity. Unit-lane placement is correct: all five tests are hermetic, spawn no daemon and exec no binary, so none needed the `integration` tag. `union_internal_test.go` correctly stays an in-package test for the unexported `persistedRows`.
- SOLID principles: Good — no responsibility moved; the change is confined to declaration form.
- Complexity: Low. Two call-hoists slightly reduce inline density and remove a duplicated evaluation opportunity.
- Modern idioms: Yes — `make([]T, 0, n)` with a derived bound is the idiomatic Go form the skill prescribes.
- Readability: Good. The hoisted `inForce`/`nominations` locals name the bound at the point it is used, so the reader can see the capacity is exact rather than guessed.
- Issues: None. Comments in the changed regions were re-read and none makes a claim the code falsifies; no comment references a task id, phase or spec section.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/doctor_theme.go:93 — `persistedThemeAdvisories` still answers with a bare `return nil` on its `deps.PrefsStore == nil` guard, so the one function this task normalised now returns an empty slice down its main path and nil down its guard path — the exact split the task set out to remove, one line above the accumulator it fixed. Change it to `return []themeAdvisory{}`. Safe: the sole production caller (`cmd/doctor_theme.go:55`, via `assembleThemeAdvisories`) only ranges and takes `len`, and no test exercises or pins the nil-`PrefsStore` path (repo-wide grep for `PrefsStore: nil` / `DoctorDeps{}` finds no construction with a nil store). Deliberately left out of scope by the task's "do not sweep beyond these five sites" instruction, so this is a follow-up rather than an omission.
