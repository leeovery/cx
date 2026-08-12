TASK: 12-10 — Collapse The Third Declared-Value ThemeEnumerator Fake In Package tui (tick-7a18f9, Phase 12: Analysis Cycle 2)

ACCEPTANCE CRITERIA:
- `package tui` declares exactly one declared-value `ThemeEnumerator` fake.
- `recordingThemeEnumerator` no longer exists.
- Every former consumer records the same calls it did before (opens, keys, settings, slot loads, reassembles).
- The two embedding enumerators and the `tui_test` fixture enumerator are untouched.

STATUS: complete

SPEC CONTEXT: This is an analysis-remediation (duplication) task from Phase 12, not a spec-behaviour task — the specification governs the theming feature's behaviour, and this task changes test scaffolding only ("Change no assertion"). The relevant project convention is `CLAUDE.md`'s DI/testing pattern (small seam interfaces, per-package fakes) plus `.claude/skills/golang-testing`'s single-shared-fake preference. Verified against the *current* codebase, where later phases (13–17) renamed the seam `ThemeEnumerator` → `ThemeSource`, `Resolve(…, theme.Setting)` → `Resolve(…, theme.RawKeys)`, `ResolveSlot` → `LoadSlot`, and the fake `fakeThemeEnumerator` → `fakeThemeSource` (file `theme_enumerator_fake_test.go` → `theme_source_fake_test.go`). Those are intentional supersessions, not drift from this task.

IMPLEMENTATION:
- Status: Implemented
- Location: commit 882d3ed9. Current state:
  - `internal/tui/theme_source_fake_test.go:10-61` — the single surviving declared-value fake, carrying `opens` (`:26`, incremented at `:37`) and `keys` (`:27`, appended at `:38`) added by this task, alongside the pre-existing `reassembles` / `reassembleKeys` / `resolves` / `slotLoads` recorders.
  - `internal/tui/theme_panel_open_test.go:22-27` — `newOpenEnumerator(union)` replaces the deleted `recordingThemeEnumerator`, preserving its `theme.Enumeration{DirPath: fixtureThemesDir}` answer.
  - `internal/tui/theme_panel_entry_test.go:35-43` — `newEntryEnumerator` now builds a `*fakeThemeSource`; `:54`, `:97`, `:142` re-typed to `*fakeThemeSource`.
  - `internal/tui/theme_panel_close_test.go:474` (at commit) re-pointed to `newOpenEnumerator`.
- Notes:
  - Grep of every `Open(theme.RawKeys)` implementation in the package confirms exactly four seam implementations remain and only one is declared-value: `fakeThemeSource` (`theme_source_fake_test.go:36`); `countingThemeSource` (`theme_panel_open_test.go:31-39`) and `behaviourEnumerator` (`theme_panel_behaviour_test.go:18-32`) both EMBED the production `theme.DirThemeSource`, and `fixtureThemeSource` (`theme_seams_test.go:14`) is in `package tui_test`. All three untouched by the commit — criterion 4 holds.
  - `recordingThemeEnumerator` does not exist anywhere in the repo (grep clean) — criterion 2 holds.
  - Behavioural equivalence for the re-pointed consumers checked method by method: `Reassemble` returns the single declared union while `reassembled` is nil (the re-pointed fixtures declare none), matching the old recorder exactly; `Resolve` returns `(resolution, err)` where these fixtures leave `err` nil, matching the old unconditional `nil`; the old `ResolveSlot`'s synthesised `SlotResolution{Slot, Requested: slug, Resolved: slug}` is subsumed by the fake's declared-record-then-nomination fallback, and no re-pointed test reaches a slot load anyway (entry/open cases never commit). Criterion 3 holds; the extra `reassembles`/`reassembleKeys` recording the consumers now inherit is additive and unasserted in those suites.
  - No assertion was changed. The only structural edit inside a test body is `TestThemePanelOpen_BadgesFromTheSeamsResolution` moving the declared `resolution` from a composite literal to a post-construction field assignment (`theme_panel_open_test.go:364-374`) — same value, required by the constructor helper.

TESTS:
- Status: Adequate (this task's product IS test code; assessed by reading, per the no-execution rule)
- Coverage: The task's own test requirement — that the migrated counters not be dead — is met by live assertions from more than one re-pointed suite: `opens` at `theme_panel_entry_test.go:147,180,207,237,265,288` and `theme_panel_open_test.go:144,180,202,221,230,257` (plus `theme_panel_arrow_test.go:412`, `theme_panel_close_test.go:179,239`, `theme_panel_commit_recompute_test.go:168`), and `keys` at `theme_panel_open_test.go:342-349` (the construction-time-prefs-snapshot pin, which is the only consumer of the recorded keys and would fail loudly if `Open` stopped recording).
- Notes: No redundancy introduced — every recorder field on the surviving fake has at least one live consumer (`resolves` at `theme_panel_close_test.go:74-96`, `slotLoads` at `theme_panel_commit_load_test.go:823,851`, `reassembles`/`reassembleKeys` at `theme_panel_commit_recompute_test.go:425-433` and `theme_panel_commit_protocol_test.go:59-99`), so the collapse left no dead scaffolding.

CODE QUALITY:
- Project conventions: Followed. One fake per seam per package, declared-value fixtures with no filesystem, `*testing.T`-first helpers, no `t.Parallel()`.
- SOLID principles: Good — the collapse strengthens the single-implementation-per-seam-per-package shape; a new seam method is now implemented once in `package tui`.
- Complexity: Low. The fake's methods are straight record-and-answer; `Reassemble`'s single nil-check branch is the only conditional.
- Modern idioms: Yes.
- Readability: Good. `newOpenEnumerator`'s comment correctly explains the zero-resolution degrade policy those cases rely on, and the migrated field comments (at the time of the commit) stated why each counter exists rather than restating the code.
- Issues: None blocking. See the notes below on the now-vestigial interface parameter in `newEntryModel` and the placement of `fixtureThemesDir`.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/theme_panel_entry_test.go:54,57 — narrow `newEntryModel(t *testing.T, e ThemeSource, o entryModelOpts)` to take `e *fakeThemeSource` and delete the `rec, _ := e.(*fakeThemeSource)` comma-ok assertion, returning `e` directly. All eleven call sites pass `newEntryEnumerator(...)`, so the interface parameter no longer buys anything, and the swallowed assertion means a non-fake source would hand back a nil `rec` that `requireSilentRefusal` (`:147`) and the `rec.opens` assertions dereference — a nil-pointer panic instead of a readable failure.
- [quickfix] internal/tui/theme_panel_open_test.go:18 — move `const fixtureThemesDir` into `theme_source_fake_test.go` beside the fake it seeds. It is now read from five files (`theme_panel_entry_test.go:39`, `theme_panel_commit_slot_test.go:84`, `theme_panel_commit_recompute_test.go:348`, plus the open suite), so declaring it inside one suite's file misstates its scope.
