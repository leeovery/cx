TASK: theming-system-17-12 — Make The Capture Fixture Registry One Authoritative List (tick-f451e6)

ACCEPTANCE CRITERIA:
- Fixture names are enumerated in exactly one place in `internal/capture`.
- `FixtureNames()` returns the same sorted set as today.
- `FixtureByName` returns the same fixture for every existing name and the same error text for an unknown one.
- Adding a builder to the list makes it reachable by name and counted by the guard in one edit.

STATUS: complete

SPEC CONTEXT:
The specification touches the capture harness only at line 1586 (`capturetool --fixture` is "loaded in a real terminal at the human-in-the-loop gate and judged as the real thing"), so this is an internal-duplication remediation with no spec behaviour attached. The governing project constraint is CLAUDE.md's: "The Go fixture definitions in `internal/capture` and the harness itself are permanent — the swap-and-diff completeness guard drives the fixture renderer and its coverage assertion enumerates whatever fixtures exist, so deleting one silently shrinks the guard rather than failing it." That silent-shrink failure mode is exactly what this task removes at the registry level.

IMPLEMENTATION:
- Status: Implemented
- Location: `internal/capture/fixtures.go:130-185` (`fixtureBuilders`, `FixtureByName`, `FixtureNames`); test at `internal/capture/fixture_registry_test.go`; obsolete AST drift-check removed from `internal/capture/theme_swap_guard_test.go` (commit 6e5d97a7).
- Notes:
  - AC1 satisfied. The 27-case switch and the 27-name literal slice are both gone; `fixtureBuilders()` (`fixtures.go:133-163`) is the sole enumeration and both lookups derive from it. Verified mechanically: the 27 `func …Fixture() *Fixture` declarations in the file and the 27 entries of the registry list are a one-to-one match — no declared builder is unregistered, no entry is duplicated.
  - AC2 satisfied. Diffing the removed literal slice against the new builder-derived names shows an identical set (27 names + `ContrastValidationFixture`); nothing added, nothing dropped. Sort semantics are unchanged — `sort.Strings` → `slices.Sort` is identical for `[]string` and is the modernisation the repo's `modernize` linter wants.
  - AC3 satisfied. The builder list preserves the old switch's order exactly (`sessions-flat` … `loading-error`), each builder still assigns its own `name` (per Do #4), and the unknown-name error string is character-identical to the old `default` branch, including the `available:` list built from `FixtureNames()`.
  - AC4 satisfied. One list entry now makes a fixture both resolvable and enumerated.
  - `ContrastValidationFixture` behaviour is preserved: it has no builder (as it had no switch case), so it stays unresolvable through `FixtureByName` while still being appended to `FixtureNames()` — still pinned by `theme_swap_guard_test.go:155-159`.
  - Callers unaffected: `cmd/capturetool/main.go:75,170,172` use only the two exported functions, and `main.go:91` still special-cases the swatch before `resolveModel`.
  - Deleting `fixtureByNameCases`/`TestFixtureRegistry_ByNameCasesMatchFixtureNames` (the `go/ast` switch-vs-slice drift check) is correct rather than a coverage loss — the drift it detected is now structurally impossible. `strconv` remains used elsewhere in that file (lines 202, 244), so the import removal is consistent.
  - `FixtureByName` now constructs fixtures until a name matches, and `FixtureNames` constructs all of them. All builders are pure in-memory constructors (the theme-panel ones assemble via `theme.Assembler` over embedded built-ins and declared entries; CLAUDE.md and the in-file comment both confirm the harness reads no directory), so there is no side-effect or I/O change — only a negligible cost in an offline test/harness path.

TESTS:
- Status: Adequate (with minor redundancy against pre-existing tests — see notes)
- Coverage:
  - `fixture_registry_test.go:9` uniqueness — the genuinely falsifiable and highest-value test here: fixtures are authored copy-constructor style (`fx := sessionsFlatFixture(); fx.name = "…"`, e.g. `fixtures.go:334-344`), so a forgotten rename is the live failure mode, and it would now shadow a fixture out of both lookups. Also guards the empty-name case and fatals on an empty builder list so it cannot pass vacuously.
  - `fixture_registry_test.go:28` pins that `FixtureNames()` is builder-derived, sorted, and appends the swatch.
  - `fixture_registry_test.go:41` pins the plan's "every enumerated name but the swatch resolves" contract.
  - `fixture_registry_test.go:57` pins the unknown-name error text and its `available:` list verbatim — this is what protects AC3's error-text half.
  - The swap-and-diff completeness guard is untouched in substance and still enumerates `FixtureNames()` → `FixtureByName` (`theme_swap_guard_test.go:95-110,137-160`), over a provably identical fixture set.
- Notes:
  - Not under-tested: every acceptance criterion has a corresponding assertion, except "same sorted set as today", which is deliberately (and correctly) not pinned by a golden name list — such a list would re-introduce the very hand-maintained duplicate this task removed. Set identity was confirmed at review time from the commit diff.
  - Mild over-testing: two resolve-every-name assertions now exist (new `TestFixtureByName_ResolvesEveryEnumeratedName` and the older `TestThemeSwapGuard_EnumeratesRegistry` sub-test), and both are structurally guaranteed post-refactor. They still have value as regression guards against re-introducing divergence, but one of the two is redundant. See notes below.

CODE QUALITY:
- Project conventions: Followed. `internal/capture` stays out of the production binary (no new imports), the exported surface is unchanged, and `slices.Sort` aligns with the `modernize` linter the repo runs.
- SOLID principles: Good. Single source of truth for fixture identity; each builder remains the sole owner of its own name, so the registry derives rather than restates.
- Complexity: Low. A 27-entry data list plus two trivial derivations replaces a 55-line switch — the cyclomatic complexity of `FixtureByName` drops from 28 to 2.
- Modern idioms: Yes. `[]func() *Fixture` as data, range-over-slice, `slices.Sort`, correctly pre-sized `make([]string, 0, len(builders)+1)`.
- Readability: Good. The registry reads as a manifest and the two derivations are three lines each.
- Comment accuracy: Accurate. `fixtures.go:130-132` describes exactly what the code does and why; `fixtures.go:174-175` ("FixtureNames includes the contrast-validation swatch, which is a standalone tea.Model rather than a *Fixture") still holds against `fixtures.go:182`. No process-artifact references (no task ids/phases) were introduced.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/capture/theme_panel_fixture_render_test.go:225-227 — this `slices.Contains(capture.FixtureNames(), name)` assertion can no longer fail: line 222 already resolved the name through `FixtureByName`, and post-refactor both lookups derive from `fixtureBuilders()`, so a resolvable name is necessarily enumerated. Delete lines 225-227, keeping the `FixtureByName` resolve check (222-224) and the guarded-set check (228-230), which remain falsifiable.
- [quickfix] internal/capture/theme_swap_guard_test.go:138-153 — the "every enumerated name but the swatch resolves to a fixture" sub-test is now duplicated by `fixture_registry_test.go:41` (`TestFixtureByName_ResolvesEveryEnumeratedName`), and `registryFixtures` (theme_swap_guard_test.go:104-106) already fatals on any resolve failure. Delete this sub-test and keep only the "the swatch is the only skip" sub-test (155-159), which asserts something the registry test does not.
- [quickfix] internal/capture/capture_test.go:107-117, 134-142, 185-193, 235-243, 314-322, 360-368, 411-419, 482-490, 531-539, 587-595 — ten near-identical hand-written "FixtureNames() includes <name>" loops, each sitting in a test that already called `FixtureByName(<same name>)`. They pre-date this task but are now single-cause with the resolve call. Collapse each to the one-liner form already used at capture_test.go:726 (`if !slices.Contains(capture.FixtureNames(), name) { … }`), or drop them where the enclosing test already resolves the name.
