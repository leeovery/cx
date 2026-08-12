TASK: theming-system-16-2 — Extract The Panel Open/Close "Writes Nothing" Proof (tick-a5355a, phase 16 / analysis cycle 6, severity medium, source: duplication)

ACCEPTANCE CRITERIA:
1. The ~45-line staging/assertion body exists once; neither test file declares the seed constant, the fixture staging or the read-back comparison.
2. Both tests still name their own action and still fail with a message naming which action wrote.
3. Both subtest names are unchanged.
4. The helper fails when the panel is made to write on either path.
5. `go test ./internal/tui` passes.

STATUS: complete

SPEC CONTEXT:
Specification §9.2 (line 1012) states the invariant being proven: "**Every write is an explicit keypress; nothing writes on close.**" — eliminating the "applied but not persisted" state reachable under persist-on-close. §5.8 (line 412) pins directory enumeration to *every panel open*, and §9.6/§9.2 make open a pure read (it re-enumerates and may re-resolve a fallback, but persists nothing). The staged fixtures match those paths deliberately: the present-prefs subtest stages an **invalid** `sunset.theme` so open/close run over §8.4/§8.5's rejection-fallback route — precisely the path where a stray write would overwrite the slug the user set (spec line 1489 confirms a fallback re-resolves "at construction, again on every panel open and again on every `Esc`"). The absent-prefs subtest stages a valid theme and asserts neither directory gained an entry.

IMPLEMENTATION:
- Status: Implemented (commit 8c274a7c; later amended in-place by 17-5 and the comment-audit commits, which is the intended supersession pattern for this plan)
- Location:
  - Helper + parameter type: `internal/tui/theme_testing_test.go:146-200` (`panelReadOnlyPath`, `requireNoPrefsOrThemesWrite`)
  - Open caller: `internal/tui/theme_panel_cursor_test.go:405-412` (`TestPanelOpen_WritesNothing`)
  - Close caller: `internal/tui/theme_panel_close_test.go:274-280` (`TestPanelClose_WritesNothing`)
- Notes:
  - Criterion 1 holds. Diff of 8c274a7c removes both ~45-line bodies (−46 / −48 lines) and adds one 74-line home. The seed constant appears exactly once repo-wide (`grep 'session_list_mode":"by-project"' internal/tui/*_test.go` → `theme_testing_test.go:156` only). Both callers dropped their now-unused `path/filepath` import; `theme_panel_close_test.go` correctly retains `os` (still used at :166/:182).
  - Criterion 2 holds and is *strengthened*: the present-prefs failure message previously read `prefs.json =\n%s`, with nothing naming the action; it now reads `prefs.json after %s =` composed from `path.verb` ("opening" / "closing"). The absent-case messages carry the verb as before.
  - Criterion 3 holds exactly. The two callers' absent-case subtest names genuinely differed before the extraction ("…and the directory is untouched" for open, "…and the themes directory is untouched" for close). Rather than unify them (which would move a maintainer's grep surface), the implementation carries the name as a struct field. This is a justified deviation from the plan's literal `verb string` signature — the struct is what makes criteria 2 and 3 satisfiable simultaneously.
  - Criterion 6 of the Do list ("take the stronger of the two") holds: the close caller's present-prefs act was open→close and is now open→`↓`→close, so the discard-an-active-preview path is exercised in both subtests instead of one. No assertion was weakened; entry-count and byte-comparison assertions are identical to the stronger prior copy.
  - The plan's `act` contract is honoured literally — `act` stages nothing, receives `(dir, keys)` and differs only in constructor and keypresses (`themeCursorModel` + `pressThemeKey` vs `newClosePanelModel` + `pressThemeKey`/`arrowDown`/`closeThemePanelForTest`).
  - Later-phase edits landed on the single body exactly as the task's Outcome predicted: 17-5 swapped `writeThemeFileForTest(t, dir, name, value)` for `themetest.Write(t, dir, name, themetest.MonochromeLines(value))` in one place, and `writeThemeFileForTest` is now gone repo-wide. Not drift.

TESTS:
- Status: Adequate (this task *is* test code; the "test" is that both proofs still run, unchanged in name and outcome)
- Coverage:
  - Both subtests run under both callers, with identical staging and identical assertions: present prefs byte-compared against the seed; absent prefs asserted via a zero-entry config dir; themes dir asserted to hold exactly the one seeded drop-in.
  - Not over-tested: the extraction removes ~90 duplicated lines and adds no new assertion. The close test's genuinely distinct first subtest ("no persister is reached", `theme_panel_close_test.go:245-272`) is correctly left in place rather than forced through the shared helper — it counts seam calls and carries its own positive control, which is a different proof.
  - Scoping is right: no `t.Parallel()` (repo rule), `t.Setenv` used inside the subtest closures, and every assertion inside `t.Run` uses the subtest's own `t` — no parent-`t` assert-scope leak (golang-testing SKILL.md §"Assert Scope Leaking into Subtests").
- Notes:
  - Criterion 4 ("the helper fails when the panel is made to write") holds for a **direct file write** on either path — that is the mutation the helper is shaped to catch, and the `PORTAL_PREFS_FILE` set is the tripwire for `internal/tui` ever growing its own path resolution (which the sibling helper at :73-74 asserts it does not have). It does **not** catch a *seam-mediated* write, because a nil `ThemePersister` is inert by design (`theme_panel_commit.go:32`) and neither constructor injects one. That property is inherited verbatim from both pre-extraction copies, and the task's Do list item 6 forbade changing assertion meaning, so it is not drift. For the close path the gap is closed by the sibling "no persister is reached" subtest; for the open path there is no equivalent — see the non-blocking note.
  - Neither caller asserts the panel actually opened/closed, so a fixture regression would make both proofs vacuous. Also inherited, also noted below.
  - `go test ./internal/tui` was not executed (verifier does not run tests). Verified by reading: the helper's identifiers (`os`, `filepath`, `themetest`, `theme.RawKeys`) are all imported in `theme_testing_test.go:3-15`; both callers' remaining imports are all still referenced; every helper it calls (`themetest.Write`, `themetest.MonochromeLines`, `pressThemeKey`, `pressPanelKey`, `closeThemePanelForTest`, `newClosePanelModel`, `themeCursorModel`, `arrowDown`) exists with the signature used.

CODE QUALITY:
- Project conventions: Followed. Test-only helper lives in the package's shared `theme_testing_test.go` home alongside `requireCommitDoesNoOtherIO`, `t.Helper()` on the exported-shape helper, `require…` naming consistent with its neighbour, no `t.Parallel()`, no testify.
- SOLID principles: Good. The helper owns staging + assertion; the callers own construction + action. The seam between them (`func(t, dir, keys)`) is the minimum that lets the two paths differ, and `panelReadOnlyPath` isolates the only two strings that must vary.
- Complexity: Low. Two linear subtests, no branching beyond error checks.
- Modern idioms: Yes. `t.TempDir()`, `t.Setenv`, closure-parameterised subtests.
- Readability: Good. One mild nit (parameter named `path` in a body that also builds filesystem paths) below.
- Comment accuracy: Accurate. `theme_testing_test.go:152-153` ("act stages nothing of its own…") is true of both callers; `:166-167` ("Deliberately invalid, so the path runs over the rejection fallback") matches the `"not-a-colour"` fixture and spec §8.4/§8.5. No spec-section or task-id references survive (the comment-audit commits stripped them). No stale claims found.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/theme_panel_cursor_test.go:409-411 and internal/tui/theme_panel_close_test.go:277-280 — both `act` closures discard the returned `Model` and assert nothing about panel state, so if `t` stopped opening the panel (or `Esc` stopped closing it) both subtests would pass while proving nothing. Add a fixture guard per caller: open → `m := pressThemeKey(t, themeCursorModel(...)); if !m.themePanel.open { t.Fatal("fixture: the panel did not open, so an untouched prefs.json says nothing about the open path") }`; close → assert `m.themePanel.open` after `pressThemeKey` and `!m.themePanel.open` after `closeThemePanelForTest`. Precedent for the exact shape is in the sibling proof at internal/tui/theme_panel_arrow_test.go:516-518 and :542-544.
- [quickfix] internal/tui/theme_panel_cursor_test.go:405 — `TestPanelClose_WritesNothing` opens with a "no persister is reached" subtest (theme_panel_close_test.go:245-272) that counts seam calls and carries a positive control; `TestPanelOpen_WritesNothing` has no mirror, so a regression where *open* reaches `ThemePersister`/`ModePersister` is caught on neither path (a nil persister is inert per theme_panel_commit.go:32, so the file-based proof cannot see it). Add the mirror subtest to the open test: inject `&countingModePersister{}` / `&fakeThemePersister{}` via `WithModePersister`/`WithThemePersister`, press `t`, assert zero calls, then the same `s`-keypress positive control the close test uses.
- [idea] internal/tui/theme_panel_arrow_test.go:532-553 — a third copy of the "the themes directory holds %d entries … want only the seeded drop-in" assertion (with its own `themetest.Write` staging) survives on the arrow-preview path, i.e. the same duplication class this task retired for open/close. Decide whether the arrow path should route through `requireNoPrefsOrThemesWrite` (it would gain the present-prefs byte-comparison it currently lacks, at the cost of renaming its "the themes directory is untouched" subtest to the helper's two fixed names) or stay separate because its fixture guard and preview-specific control do not fit the helper's `act` contract.
- [do-now] internal/tui/theme_testing_test.go:146-149 — `panelReadOnlyPath` carries no comment saying why `absentSubtest` is a field rather than a composed string, so the next maintainer is likely to "simplify" it away and silently rename one caller's subtest. Add above the field: `// absentSubtest is carried, not composed: the two paths word it differently, and each is the literal string a maintainer greps its failure by.`
- [do-now] internal/tui/theme_testing_test.go:153, 177, 190, 197 — the parameter is named `path` in a body that also builds filesystem paths with `filepath.Join`, so `path.verb` reads momentarily as a path operation. Rename the parameter to `readOnly` (call sites are positional, so only these four references change).
