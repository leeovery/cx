# Analysis Tasks: theming-system (Cycle 2)

## Task 1: Single-source the "which persisted theme keys are in force" rule across the panel union and doctor
severity: high
sources: duplication, architecture

**Problem**: `inForceValues` (`internal/theme/union.go:383`) and `persistedThemeNominations` (`cmd/doctor_theme.go:259`) are independent implementations of one rule with three identical clauses: run `ResolveSetting` for the `theme`-wins tiebreak; under a constant yield the constant alone with the slots unread; otherwise yield only the slots with a NON-EMPTY raw value; and collapse two slots naming the same value to one. Only the first clause is actually shared — both call `ResolveSetting`, then each re-authors the emptiness rule and the collapse. They differ solely in rendering: the union yields one deduped value, doctor yields one nomination labelled `both`. `inForceValues` is unexported, so doctor structurally could not compose on it and had to re-author. Both files' comments assert the two must agree ("Doctor's persisted line makes the identical call for the identical reason, so the two surfaces cannot disagree about which slug is live", union.go:369-372) — an invariant stated in prose across a package boundary, which is exactly the shape that drifts. There is no cross-surface parity test: every existing test pins one surface against hand-written expectations, so a change to the tiebreak, the emptiness rule or the collapse fails nothing on the other side. The failure mode is the panel listing a row doctor does not report, or the reverse, against two surfaces the spec requires to report the same problems ("one slug is one row" / "one slug is one line").

**Solution**: Export one in-force selector from `internal/theme` carrying the persisted value and the slot(s) it occupies, and derive both surfaces from it — the union mapping over the value, doctor mapping over the pair and keeping only its slot-label rendering.

**Outcome**: The three clauses have exactly one home. A change to the tiebreak, the emptiness rule or the collapse moves both surfaces at once, and a parity test fails if they are ever made to disagree again.

**Do**:
1. In `internal/theme`, beside `ResolveSetting`, add an exported selector — e.g. `type InForceKey struct { Value string; Slot Slot; Both bool }` and `func InForceKeys(keys RawKeys) []InForceKey` — holding all three clauses (constant-alone, non-empty-slots-only, same-value collapse). Take `RawKeys` and call `ResolveSetting` internally: stripping is idempotent and the resolution is pure and total, which union.go's existing comment already relies on. If doctor's call site has `setting` and `raw` already in hand, either reuse them via a second entry point or let it pass `raw` — do not let doctor keep its own copy of any clause.
2. Reduce `inForceValues` to a map over the selector's `Value` field (or delete it and inline the map at `persistedRows`, `union.go:354`).
3. Reduce `persistedThemeNominations` to a map over the selector's results, converting slot to its rendered label through the existing `themeSlotLight` / `themeSlotDark` / `themeSlotBoth` labels. Doctor keeps only the label mapping.
4. Move the surviving explanation of WHY the rule is what it is onto the exported selector; delete the now-duplicated prose from both former sites, including the cross-package "the two surfaces cannot disagree" assertion, which the shared function now makes structurally true.
5. Preserve behaviour exactly: constant yields one entry with no slot; a pair yields entries in light-then-dark order; `raw.Light == raw.Dark` (non-empty) yields one entry marked both.

**Acceptance Criteria**:
- One exported function in `internal/theme` holds the constant-alone, non-empty-slot and same-value-collapse clauses; neither `internal/theme/union.go` nor `cmd/doctor_theme.go` restates any of them.
- The panel union's persisted rows and doctor's persisted lines are both derived from that function.
- Doctor's rendered advisory output is byte-identical to today's for every existing test case, and the panel's row set and ordering are unchanged.
- No comment in either file asserts agreement with the other surface as a prose invariant.

**Tests**:
- A parity test over a shared table of key shapes (constant only; constant plus both slots; light only; dark only; both slots differing; both slots identical; all empty; a slot holding an unresolvable slug) asserting the panel union's persisted rows and doctor's persisted lines name the same slug set. Site it where both are reachable (`package cmd`, which can call the exported selector, the assembler and doctor's producer).
- Existing `internal/theme` union tests and `cmd` doctor theme tests pass unchanged.

## Task 2: Bind the commit path's in-memory key mirror to what prefs actually writes
severity: medium
sources: architecture

**Problem**: A theme commit writes through the persister and then re-states prefs' mutual-exclusion rules in memory so the panel can recompute rows and badges. `commitConstant` (`internal/tui/theme_panel_commit.go:131`) sets `theme.RawKeys{Theme: slug}` — a constant clears both slots — and `mirrorThemeSlot` (`theme_panel_commit.go:310`) sets one slot, drops the constant and carries the other slot across. Those are exactly the rules `prefs.SaveTheme` (`internal/prefs/store.go:436`) and `prefs.SaveThemeSlot` (`internal/prefs/store.go:458`) apply to the file. The two implementations are only ever checked in isolation: `internal/tui`'s commit tests inject a fake persister and assert the mirrored keys against literals, while `cmd/open_theme_commit_test.go` asserts the bytes on disk and never the post-commit panel state. Nothing fails if they drift, and the failure mode is the invariant the whole panel is built around — the `●` badge claiming a persisted state the file does not hold, with no other surface to contradict it until relaunch. Full single-sourcing is blocked by the prefs leaf rule (prefs cannot import `internal/theme`), so the fix is a shared statement on the TUI side plus one binding assertion, not one shared mutator.

**Solution**: Express the in-memory transformation once as methods on `theme.RawKeys` that both commit handlers call, and extend an existing cmd round-trip test to assert the model's post-commit keys equal what `prefs` reads back off disk.

**Outcome**: The mutual-exclusion rules are stated once on the TUI side, and one test binds them to the bytes prefs writes — so a divergence between the badge and the file fails a test rather than shipping.

**Do**:
1. Add `WithConstant(slug string) RawKeys` and `WithSlot(slot Slot, slug string) RawKeys` (or the equivalent named methods) to `theme.RawKeys` in `internal/theme`, each returning a new value that applies the mutual exclusion structurally — the constant form holding only the constant, the slot form holding only the pair with the named half replaced.
2. Rewrite `commitConstant`'s `m.themeState.keys = theme.RawKeys{Theme: slug}` and `mirrorThemeSlot`'s switch as calls to those methods. Keep the structural-clear property (construct a fresh value; never mutate a field in place), which is what the existing comment on `mirrorThemeSlot` is protecting.
3. Extend one existing `cmd` round-trip test (`cmd/open_theme_commit_test.go`) so a commit asserts BOTH the bytes on disk and the model's post-commit `themeState.keys` (or the rendered badge set) against a single read-back through `prefsStore.LoadThemeKeys()`. Cover both directions: a constant commit over a persisted pair, and a slot commit over a persisted constant.
4. Do not change what is written or when — the write path, the nil-persister guard, and the failed-write-does-not-recompute rule are unchanged.

**Acceptance Criteria**:
- The constant-clears-slots and slot-clears-constant transformations appear once each in `internal/theme`, and both TUI commit handlers call them.
- No `theme.RawKeys` literal in `internal/tui/theme_panel_commit.go` re-states either rule.
- At least one test asserts the post-commit model keys equal the keys read back from the written `prefs.json`, in both commit directions.
- A failed write still leaves the keys untouched and does not recompute.

**Tests**:
- The extended cmd round-trip assertions above (disk bytes and model state from one read-back).
- Existing `internal/tui` commit tests pass unchanged; existing `internal/prefs` write-semantics tests pass unchanged.

## Task 3: Single-source the built-in-loading test helper into `internal/themetest`
severity: medium
sources: duplication

**Problem**: "Load an embedded built-in by slug in a test" is written seven times across five packages: `testBuiltinTheme` (`internal/tui/theme_testing_test.go:36`, `package tui`), `darkBuiltinTheme` (`internal/tui/model_test.go:24`, `package tui_test`), `builtinTheme` (`internal/capture/theme_panel_fixture_test.go:725`, `package capture`), `builtinPalette` (`internal/capture/theme_panel_fixture_render_test.go:370`, `package capture_test`), `builtinThemeForTest` (`cmd/open_theme_nomination_test.go:185`), `builtinForTest` (`cmd/capturetool/main_test.go:627`) and `builtinTheme` (`internal/theme/resolution_test.go:43`, `package theme_test`). Each does `theme.NewLoader(nil).LoadBuiltin(slug)` (or `NewSilentLoader()`), Fatal on `!found`, Fatal on `rejection != nil`, return `result.Theme`; three pairs are byte-identical bodies with near-identical doc comments. Cycle 1's task 11-13 collapsed two of these inside `package capture_test` but could not reach `builtinTheme`, which lives in `package capture` — the same in-directory split separates `testBuiltinTheme` (`package tui`) from `darkBuiltinTheme` (`package tui_test`). That split is why the duplication regenerates: there is no importable home, so each package pair mints its own. Two of the seven have already drifted — `darkBuiltinTheme` and `builtinForTest` collapse `!found` and `rejection != nil` into one failure message, so they cannot tell "the slug names no built-in" from "the shipped file is broken", which is precisely the discrimination the build-time guarantee exists to surface.

**Solution**: Add the helper to the existing test-only `internal/themetest` package (already home to `Lines`/`WithValue`/`WithoutKey`/`Write` from cycle 1) with the two-way failure discrimination preserved, and delete all seven local copies.

**Outcome**: One definition of built-in loading in tests, with one failure message per failure class. A change to the loader's built-in entry point is one edit, and no copy can silently merge the two failure classes again.

**Do**:
1. Add `Builtin(t *testing.T, slug string) theme.Theme` to `internal/themetest`, keeping DISTINCT Fatal messages for `!found` ("the slug names no built-in") and `rejection != nil` ("the shipped file is broken") — do not adopt the drifted merged form.
2. Optionally add `DefaultDark(t *testing.T) theme.Theme` and `DefaultLight(t *testing.T) theme.Theme` wrappers over it for the call sites that only want the shipped pair.
3. Delete all seven local helpers and re-point their callers. `internal/tui`'s `testDarkTheme`/`testLightTheme` and `internal/capture`'s `darkBuiltinTheme`/`lightBuiltinTheme`, where they exist, become one-line wrappers over the shared helper rather than separate implementations.
4. Use `NewLoader(nil)` or `NewSilentLoader()` consistently inside the one helper — pick whichever the majority of call sites rely on and confirm no test asserts on emitted theme events from these loads.
5. Verify no import cycle: `themetest` imports `internal/theme`, and every consumer is either a different package or an external `_test` package (`internal/theme/resolution_test.go` is `package theme_test`, so it can import `themetest`).
6. Keep `internal/themetest` test-only — no production (non-`_test.go`) file may import it.

**Acceptance Criteria**:
- `internal/themetest.Builtin` exists with two distinct Fatal messages, one per failure class.
- No package declares its own built-in-loading test helper; all seven former sites call the shared one.
- No production file imports `internal/themetest`.
- `go test ./...` passes with no behavioural change.

**Tests**:
- A test in `internal/themetest` asserting `Builtin` returns the parsed palette for a shipped slug (compare against a distinguishing token, not a whole-struct literal).
- All existing tests in `internal/theme`, `internal/tui`, `internal/capture`, `cmd` and `cmd/capturetool` pass unchanged after the swap.

## Task 4: Single-source the SGR parameter-run probe in `internal/tui` and `internal/capture`
severity: medium
sources: duplication

**Problem**: "Render a one-cell probe through a lipgloss style, find `'['` and `'m'`, slice out the parameter run" is implemented eight times plus two inline copies. In `package tui`: `tokenFgSeq` (`header_test.go:19`), `tokenBgSeq` (`header_test.go:36`), `bgSeq` (`edit_modal_test.go:687`), `sgrForegroundCore` (`active_theme_test.go:93`), `canvasSeq` (`canvas_paint_test.go:15`), `selectionBgParams` (`session_row_anatomy_test.go:52`), plus the derivation inlined again inside `assertActiveTheme` (`active_theme_test.go:52`) and `editFieldFocused` (`model_test.go:38`, `package tui_test`). In `internal/capture`: `sgrParameterRun` (`theme_swap_guard_test.go:212`, `package capture_test`), `backgroundSGR` (`theme_panel_fixture_test.go:740`, `package capture`) and `bgSeq` (`swap_harness_test.go:208`, `package capture_test`). `tokenBgSeq` and `bgSeq` are byte-identical bodies in the SAME package, separated only by the file they sit in; `sgrForegroundCore` is the same body with a raw hex instead of a token. The derivation returns three different shapes for one operation (bare parameter run; full `ESC[…m`; params via TrimPrefix/TrimSuffix), which is what makes a call site pick the wrong one, and the failure messages have already drifted.

**Solution**: One `sgrParams` per package, returning the bare parameter run, with thin wrappers for the two shapes call sites actually want.

**Outcome**: One derivation per package instead of six-plus-two and three, and one obvious return shape — a new probe site has nothing to pick wrongly.

**Do**:
1. In `internal/tui`'s shared test-helper file (`theme_testing_test.go`, `package tui`), add `sgrParams(t *testing.T, style lipgloss.Style) string` returning the bare parameter run, with one clear failure message when the rendered probe carries no SGR sequence.
2. Re-express the `package tui` helpers over it: keep `tokenFgSeq(t, tok)` and `tokenBgSeq(t, tok)` as thin wrappers; delete `bgSeq` (`edit_modal_test.go`) and re-point its callers at `tokenBgSeq`; fold `sgrForegroundCore(t, hex)` into `sgrParams(t, lipgloss.NewStyle().Foreground(lipgloss.Color(hex)))`; express `canvasSeq` as `"\x1b[" + sgrParams(...) + "m"` and `selectionBgParams` as `sgrParams(...)`.
3. Replace the inline probe copies inside `assertActiveTheme` and `editFieldFocused` with a call. `editFieldFocused` is `package tui_test` — if it cannot reach the `package tui` helper, note that explicitly and leave it as the one unavoidable copy rather than exporting test scaffolding.
4. In `internal/capture`, do the same within each package: `capture_test` keeps one copy (collapse `bgSeq` in `swap_harness_test.go` into `sgrParameterRun` in `theme_swap_guard_test.go`, or move the shared one into the package's helper file), and `package capture` keeps its own single `backgroundSGR` expressed the same way. The in-directory split means capture legitimately holds two, not one — do not export a helper from production code to avoid it.
5. Do not change what any assertion checks; only where the derivation lives.

**Acceptance Criteria**:
- `package tui` contains exactly one implementation of the probe-and-slice derivation.
- `package capture_test` contains exactly one, and `package capture` at most one.
- No test file inlines the `'['`/`'m'` index-and-slice sequence.
- Every former helper name that survives is a one- or two-line wrapper over the shared derivation.

**Tests**:
- All existing `internal/tui` and `internal/capture` tests pass unchanged — the assertions are identical, only the derivation moved.
- Deliberately break one token's value locally and confirm the affected probes still fail with a message naming the token (manual verification during the change; do not commit the break).

## Task 5: Collapse the three no-config-access fixture tests and the re-implemented theme-file body and fixture-name lists
severity: medium
sources: duplication

**Problem**: `TestPanelFixture_NoConfigAccess` (`internal/capture/theme_panel_fixture_test.go:628`, `package capture`), `TestPanelFixture_RemainingFramesNoConfigAccess` (`theme_panel_remaining_fixtures_test.go:595`, `package capture_test`) and `TestPanelFixture_MessageFramesNoConfigAccess` (`theme_panel_message_fixtures_test.go:654`, `package capture_test`) each carry the same ~28-line body: TempDir, `MkdirAll(dir/portal/themes)`, write a decoy `.theme`, `t.Setenv` of `XDG_CONFIG_HOME` / `PORTAL_THEMES_DIR` / `PORTAL_PREFS_FILE`, render, assert the frame does not contain `"decoy-drop-in"`, assert the config dir holds exactly one entry named `portal`. They differ only in which fixture-name list they range over and which of three render entry points they call. Two of the three are in the same package and already share `validThemeFileBody`, so nothing prevents sharing the setup. Separately, `decoyThemeFile` (`theme_panel_fixture_test.go:669`) and `validThemeFileBody` (`theme_panel_remaining_fixtures_test.go:627`) are identical loops over `theme.TokenNames()` emitting `name = #123456` — a third and fourth implementation of the valid-theme-file body that cycle 1 single-sourced into `internal/themetest.Lines()` and every other package now reaches. And `panelFixtureNames()` (`theme_panel_fixture_test.go:45`, ten names) restates the union of `capturePanelFixtureNames()` + `remainingPanelFixtureNames()` + `messagePanelFixtureNames()` that `allPanelFixtureNames()` (`theme_panel_remaining_fixtures_test.go:66`) already composes, so adding a panel fixture needs both lists edited.

**Solution**: Extract the staging and assertion halves into the `capture_test` helper file, replace both hand-rolled theme-file bodies with `themetest.Lines()`, and derive the fixture-name list from one statement.

**Outcome**: The no-config-access contract is stated once, the valid-file shape has the single home cycle 1 gave it, and a new panel fixture is enrolled by editing one list.

**Do**:
1. In the `capture_test` helper file add `stageDecoyConfig(t *testing.T) (configDir string)` — TempDir, `MkdirAll`, decoy write, the three `t.Setenv`s — and `requireConfigUntouched(t *testing.T, configDir, frame string)` — the decoy-absent and one-entry assertions.
2. Rewrite `TestPanelFixture_RemainingFramesNoConfigAccess` and `TestPanelFixture_MessageFramesNoConfigAccess` as a range over their own names calling those two helpers plus their own render entry point.
3. Replace `validThemeFileBody` and `decoyThemeFile` with a body built from `themetest.Lines()` — either `[]byte(strings.Join(themetest.Lines(), "\n") + "\n")` at the call sites, or add `themetest.Body() []byte` returning exactly that and use it everywhere. If `themetest.Body()` is added, re-point any other site that joins `Lines()` by hand.
4. Derive `panelFixtureNames()` from `allPanelFixtureNames()` — or from `capture.FixtureNames()` filtered on the `theme-panel-` prefix, as `TestPanelFixture_RegistryHoldsTheSpecifiedPanelSet` already does — so the set is stated once and adding a fixture is one edit.
5. `TestPanelFixture_NoConfigAccess` lives in `package capture` and cannot import the `capture_test` helpers. Check whether anything in it needs unexported access; if not, move it to `capture_test` and fold it in. If it does, leave it as the one intentional copy and say so in a short comment — do not export test scaffolding from production code to reach it.

**Acceptance Criteria**:
- The decoy staging and the config-untouched assertions each exist once in `capture_test`; at most one test retains its own copy, and only if it provably needs unexported access.
- No package re-implements the valid-theme-file body; every site routes through `internal/themetest`.
- The panel fixture-name set is stated once; `panelFixtureNames()` derives from it rather than restating it.
- All three tests still assert the same thing: no config read, no themes-dir read, one entry in the config dir.

**Tests**:
- Existing `internal/capture` tests pass with no reference regeneration.
- A negative check during the change: point one fixture at real config discovery locally and confirm each surviving no-config-access test fails (manual verification; do not commit the break).
- Add a panel fixture name to the single list locally and confirm all consumers pick it up without a second edit (manual verification).

## Task 6: Extract the two repeated theme-panel model-construction sequences
severity: medium
sources: duplication

**Problem**: Two distinct sequences are each written out several times across nine `package tui` test files. (1) THE OPEN SEQUENCE — `Build(deps)`, assign `termWidth`/`termHeight`, `applySessions(...)`, `applySessionListSize(...)` (sometimes `applyProjectListSize`), optionally assert the content region resolved, `pressThemeKey`, `t.Fatal` unless `themePanel.open` — appears in `newGeometryPanelModel` (`theme_panel_geometry_test.go:381`), `newChromePanelModel` (`theme_panel_chrome_test.go:47`), `newArrowPanelModelAt` (`theme_panel_arrow_test.go:174`), `behaviourPanelAt` (`theme_panel_behaviour_test.go:159`) and `openCommitPanel` (`theme_panel_commit_test.go:66`). `newGeometryPanelModel` and `newChromePanelModel` are the closest pair — same `newArrowPanelDeps` source, same `geometryTerm` dims, same three-session seed shape, same two list sizings, same two content-region Fatal guards, same open assertion — differing only in one width assertion and their session names. (2) THE CONSTRUCTION-TIME RESOLUTION SEQUENCE — `theme.ResolveSetting(keys...)`, `loader.ResolveNomination(setting, dir)`, Fatal on error, then `New(fakeLister{}, WithThemeEnumerator(countingEnumeratorOver(loader, dir)), WithThemeKeys(keys), WithThemeNomination(resolution.Nomination), WithCanvasMode(...))` — appears verbatim in `themeCursorModel` (`theme_panel_cursor_test.go:37`) and `newClosePanelModel` (`theme_panel_close_test.go:56`), with `behaviourNomination` (`theme_panel_behaviour_test.go:190`) a third variant of the resolve-then-Fatal step. All are `package tui`, so nothing structural separates them.

**Solution**: Extract the two sequences into the package's shared theme test-helper file and reduce each site to its own Deps plus its own extra assertions.

**Outcome**: The panel-open ritual and the construction-time resolution each have one home. A change to how the panel is opened, sized or seeded in tests is one edit rather than five.

**Do**:
1. Add `openPanelForTest(t *testing.T, m Model, contentW, contentH int) Model` to the shared theme test-helper file: assigns the dims via `geometryTerm`, seeds the standard session set, sizes both lists, asserts the content region resolved, presses the theme key and asserts `themePanel.open`.
2. Reduce `newGeometryPanelModel`, `newChromePanelModel`, `newArrowPanelModelAt`, `behaviourPanelAt` and `openCommitPanel` to a call plus their own Deps construction and their own extra assertions (the chrome width assertion, the arrow cursor slug, the commit page/cursor arguments). Where a site genuinely needs different session names, pass them as a parameter rather than forking the helper.
3. Add `newDirBackedPanelModel(t *testing.T, dir string, keys theme.RawKeys, mode canvasAppearance) (Model, *countingThemeEnumerator)` for sequence (2), and reduce `themeCursorModel` and `newClosePanelModel` to one call each — `newClosePanelModel` adding only its sink-bound loader.
4. Fold `behaviourNomination`'s resolve-then-Fatal step into the shared resolution step if it can share it without contorting; if its enumerator differs materially, leave it and note why in one line.
5. Do not change what any test asserts, what dims are used, or the seeded session set — this is a move, and every existing test must pass with no expectation edits.

**Acceptance Criteria**:
- The `Build` → dims → seed → size → assert-region → press → assert-open sequence appears in exactly one function in `package tui`.
- The `ResolveSetting` → `ResolveNomination` → Fatal → `New(...)` sequence appears in exactly one function.
- Every former constructor is reduced to its Deps, its call, and only the assertions unique to it.
- No test expectation, dimension or session-seed value changes.

**Tests**:
- All `internal/tui` theme panel tests pass unchanged.
- The shared helpers Fatal (not skip or silently continue) when the content region does not resolve or the panel does not open — verify by inverting one precondition locally during the change.

## Task 7: Trim the topic's production comments to the quality standard
severity: medium
sources: standards

**Problem**: `code-quality.md`'s Comments section is explicit that a comment may not carry "the design argument … State the conclusion the code needs, not the debate; the reasoning lives in the project's design artifacts", and that a comment must earn its place. Across this topic's production files the comment block IS the specification prose re-narrated: rejected alternatives ("Rejected: mutable package state", "A first arithmetic answer … was rejected at a visual gate"), the debate behind a choice, and multi-paragraph restatements of what the code below plainly does. Comment-to-line ratios in the new production files are 56–85% — `theme_state.go` 186/220, `theme_seams.go` 50/60, `theme_panel_commit.go` 308/432, `load.go` 205/289, `theme_panel.go` 1224/1745, `setting.go` 80/115, `badge.go` 120/176, `resolution.go` 326/493, `union.go` 319/504, `doctor_theme.go` 278/457, `prefs/store.go` 364/625 — against 30–50% in comparable pre-existing subsystems (`spawn/burst.go` 108/225, `notice_band.go` 332/579, `restore/restore.go` 79/209, `state/capture.go` 144/497, `cmd/doctor.go` 408/884). Single doc comments run 40–60 lines (`resolveSlot`, `SaveTranslation`, `recomputeThemePanel`, `Loader`, `ResolveSetting`). None of it is checked by compiler or test, the spec is archived at sign-off, and these paragraphs stay and go stale on the first behavioural edit. Two never-allowed comment classes are also present: (1) claims about tests — "anywhere in cmd is what TestThemeComponent_BoundOnceInCmd exists to catch" (`cmd/theme_persister.go:48`), "the exact drift class keymap_dispatch_guard_test exists to guard" (`internal/tui/theme_panel_footer.go:20`), "is the parity keymap_dispatch_guard_test's contract exists to hold" (`internal/tui/theme_panel_confirm.go:309`) — a renamed or moved test turns each into a confident lie; and (2) cardinality claims — "so nothing consumes it today" (`internal/tui/theme_row.go:54`, the standard's own verbatim example of what not to write), "IT RUNS ON A SUCCESSFUL COMMIT AND NOWHERE ELSE … and nothing else does" (`theme_panel_commit.go:324-325`), "it has TWO callers by design" (`theme_panel.go:314`), "THE READ HAPPENS HERE … AND NOWHERE ELSE" (`theme_panel.go:567`), "The read happens here and NOWHERE ELSE on this path" (`internal/theme/union.go:225`) — all falsified by ordinary additive change far from the comment.

**Solution**: One sweep over the topic's production files, trimming to what the standard admits and re-wording the two never-allowed classes into the invariant they protect.

**Outcome**: The topic's production comment density sits in the surrounding codebase's band, every surviving comment states a conclusion the code needs rather than the debate behind it, and no comment can be falsified by renaming a test or adding a caller.

**Do**:
1. Sweep the topic's production files: `internal/tui/theme_state.go`, `theme_seams.go`, `theme_panel.go`, `theme_panel_commit.go`, `theme_panel_confirm.go`, `theme_panel_footer.go`, `theme_row.go`; `internal/theme/load.go`, `resolution.go`, `union.go`, `badge.go`, `setting.go`; `internal/prefs/store.go`; `cmd/doctor_theme.go`, `cmd/config.go`, `cmd/theme_persister.go`.
2. Delete rejected-alternative narration and re-argued rationale outright — it lives in the specification.
3. Collapse the 40–60 line doc comments (`resolveSlot`, `SaveTranslation`, `recomputeThemePanel`, `Loader`, `ResolveSetting` and their peers) to what a doc comment owes: what it does, inputs, outputs, errors — plus a why only where the why is not inferable from the code.
4. Delete the three test-name references. The guard tests are discoverable by name; the comment adds nothing the code needs.
5. Re-word the cardinality claims into the invariant they protect, without counting call sites — e.g. "a commit that did not land must not recompute" instead of "runs on a successful commit and nowhere else". Delete "so nothing consumes it today" entirely.
6. KEEP the genuine trap warnings — the `startupCanvasHex` freeze, the canvas-echo guard, the fallback-slug coupling, and the two-decodes split in prefs. These are the class the standard preserves; do not trim them in the sweep.
7. Target the surrounding codebase's 30–50% density as a guide, not a quota. Do not delete a comment that carries a why, a deliberate-looking-wrong warning, or an opaque-what, merely to hit a ratio.
8. Change no code in this task — comments only. Any comment whose claim you cannot verify against the code is either wrong (delete it) or a finding for a separate task.

**Acceptance Criteria**:
- No production comment in the topic's files narrates a rejected alternative or restates the design debate.
- No production comment names a test.
- No production comment counts call sites or consumers, or asserts "nowhere else"/"nothing else does".
- The four named trap warnings survive intact.
- Comment-to-line ratio in each swept file is materially reduced and broadly in line with the comparable existing subsystems.
- No non-comment line changes; `go build ./...` and `go test ./...` behave identically.

**Tests**:
- `go test ./...` and `go test -tags integration -p 1 ./...` pass — the change is comment-only, so any failure means code was touched.
- `golangci-lint run` clean (doc comments must still begin with the identifier name where the linters require it).
- A `git diff --stat` review confirming only comment lines changed.

## Task 8: Strip workflow vocabulary from the topic's test comments
severity: low
sources: standards

**Problem**: `code-quality.md` forbids "any workflow vocabulary — task ids, phase numbers, spec-section citations" in comments, and states the reason: the comment must hold true for a reader with no knowledge of the process that produced the code, long after its artifacts are archived. Cycle 1's task swept the production files clean of `§` citations, but the topic's TEST files carry them densely — `internal/tui/theme_panel_behaviour_test.go` 71 occurrences, `theme_panel_confirm_test.go` 69, `theme_panel_commit_load_test.go` 66, `theme_panel_commit_slot_test.go` 56, `theme_panel_commit_failure_test.go` 54, `internal/capture/theme_panel_message_fixtures_test.go` 52, and more across `internal/theme/name_test.go:118`, `enumerate_test.go:350`, `theme_test.go:123`, `internal/capture/theme_swap_guard_test.go:626` — plus three task-id references ("Enumeration (task 1.7)", "from task 1.8", "from task 1.3"). The standard draws no production/test distinction, and the spec these citations point at is archived at sign-off, so `§9.5` will name nothing a future reader can open. Pre-existing test files carry a handful of the same citations, so this is an amplification of an existing pattern rather than a new one.

**Solution**: Replace each citation with the rule it names, and delete the task-id references outright.

**Outcome**: Every test comment holds true for a reader with no access to the workflow artifacts, and each one names the rule it is pinning rather than pointing at an archived section number.

**Do**:
1. Enumerate every `§`, `Phase N` and `task N-M` occurrence in the topic's test files (`internal/tui/theme_*_test.go`, `internal/theme/*_test.go`, `internal/capture/theme_*_test.go`, `cmd/*theme*_test.go`, `cmd/capturetool/main_test.go`).
2. Replace each citation with the RULE it names — "pins the one-slug-one-row union rule" rather than "pins §9.4". Where the surrounding sentence already states the rule, delete the citation and keep the sentence.
3. Delete the three task-id references outright; they name nothing durable.
4. Where a citation is the comment's entire content and the assertion below is self-evident from its test name, delete the comment rather than inventing prose for it.
5. Scope to the topic's test files. Do not sweep unrelated pre-existing test files in the same pass.
6. Change no assertion, no test name and no non-comment line.

**Acceptance Criteria**:
- No `§`, `Phase N` or `task N-M` reference remains in the topic's test files.
- Every replacement names a rule or behaviour that is checkable against the code, not a document.
- No test name, assertion or non-comment line changed.

**Tests**:
- `go test ./...` passes — the change is comment-only.
- A repository grep for `§`, `Phase ` and `task ` across the topic's test files returns nothing.

## Task 9: Hold the domain light/dark type in the panel and convert once at the persister seam
severity: low
sources: architecture

**Problem**: The light/dark distinction is modelled three times — `theme.Slot` (3-valued, includes constant, `internal/theme/resolution.go:12`), `theme.Member` (2-valued, zero = dark, `internal/theme/member.go:17`) and `prefs.ThemeSlot` (2-valued, zero invalid, `internal/prefs/store.go:177`). Each is individually justified (prefs is a leaf and cannot import `internal/theme`), but the PERSISTENCE type is the one threaded through the panel: `themeSlotConfirm.slot`, `handleSlotCommitKey`, `commitSelectedSlot`, `commitSlot` and `mirrorThemeSlot` (`internal/tui/theme_panel_commit.go:310`) all carry `prefs.ThemeSlot`, and it is converted back to domain types at three sites — `oppositeThemeMember` (`internal/tui/theme_panel_confirm.go:250`), the switch inside `mirrorThemeSlot`, and `themeSlotFor` (`cmd/theme_persister.go:111`). That is the wrong direction of travel for a layer boundary: the domain owns light/dark, the store is the edge. It also produces one silent arm — `themeSlotFor`'s `default` maps an out-of-range slot to `theme.SlotConstant`, so an invalid slot would log a commit failure with no `slot` attr rather than being impossible.

**Solution**: Hold `theme.Slot` (or `theme.Member`) in the panel and convert exactly once, at the persister seam.

**Outcome**: The panel speaks the domain's light/dark type end to end, two of the three conversions disappear, and the silent `default` arm that could log a slot-less commit failure is gone.

**Do**:
1. Change the panel's carried type: `themeSlotConfirm.slot`, `handleSlotCommitKey`, `commitSelectedSlot`, `commitSlot` and `mirrorThemeSlot` hold `theme.Slot` (or `theme.Member` if the panel never needs to express constant in these positions — pick one and apply it consistently).
2. Convert exactly once, at the seam: either have `ThemePersister.CommitThemeSlot` take the domain slot and let the `cmd` adapter map it to `prefs.ThemeSlot`, or map at the single call into the persister. Do not leave a second conversion anywhere.
3. Delete `oppositeThemeMember`'s `prefs.ThemeSlot` dependency (it becomes a domain-to-domain operation, or disappears into `Member.Opposite()` if that already exists) and delete the `prefs.ThemeSlot` switch inside `mirrorThemeSlot`.
4. Remove `themeSlotFor`'s silent `default` arm. At the one surviving conversion, an out-of-range value must be impossible by type, or must fail loudly — never map to `SlotConstant`. `prefs.SaveThemeSlot` already rejects an invalid slot before writing, so the store-side guard stays as the structural half.
5. Preserve behaviour: the commit log line must still carry its `slot` attr with the same rendered value for both valid slots, and prefs must receive the same `ThemeSlot` it does today.

**Acceptance Criteria**:
- No `prefs.ThemeSlot` value appears in `internal/tui` outside the single persister-seam call (or not at all, if the seam takes the domain type).
- Exactly one domain→persistence slot conversion exists in the codebase.
- No conversion has a `default` arm that maps an unknown slot to a valid one.
- The `theme` component's commit log lines carry the same `slot` attr values as today.

**Tests**:
- Existing `internal/tui` commit and confirm tests pass with their slot arguments retyped, asserting the same outcomes.
- A test asserting the persister receives `prefs.SlotLight` for a light commit and `prefs.SlotDark` for a dark commit through the retyped path.
- A test asserting the commit log line carries the expected `slot` attr for both slots (the attr must not be absent).

## Task 10: Collapse the third declared-value `ThemeEnumerator` fake in package `tui`
severity: low
sources: duplication

**Problem**: `fakeThemeEnumerator` (`internal/tui/theme_enumerator_fake_test.go:19`) and `recordingThemeEnumerator` (`internal/tui/theme_panel_open_test.go:48`) are both `package tui`, both answer all four seam methods from declared values with no filesystem, and both record what they were asked. Their `ResolveSlot` bodies are byte-identical (`SlotResolution{Slot: slot, Requested: slug, Resolved: slug}`) and their `Reassemble` bodies differ only in the split-union branch. The only real difference is WHICH calls each records: the fake counts `reassembles` and records `settings` and `slotLoads`; the recorder counts `opens` and records `keys`. Cycle 1's task 11-9 collapsed two other fakes into `fakeThemeEnumerator` for exactly this reason and left this third one behind, so the package now carries one configurable fake plus a second that is a strict subset of it bar two counters.

**Solution**: Move the two missing counters onto `fakeThemeEnumerator` and delete `recordingThemeEnumerator`.

**Outcome**: One declared-value `ThemeEnumerator` fake in `package tui`. A new seam method is implemented once, and a test picking a fake has one to pick.

**Do**:
1. Add `opens` (a counter) and `keys` (a recorded slice) to `fakeThemeEnumerator`, incremented and appended in its existing `Open`.
2. Re-point `internal/tui/theme_panel_open_test.go` and `theme_panel_entry_test.go`'s `newEntryEnumerator` (`theme_panel_entry_test.go:63`) at `fakeThemeEnumerator`, preserving the split-union branch behaviour their `Reassemble` relies on (make it a configurable field if the two branches genuinely differ).
3. Delete `recordingThemeEnumerator`.
4. Leave `countingThemeEnumerator` (`theme_panel_open_test.go:83`) and `behaviourEnumerator` (`theme_panel_behaviour_test.go:95`) alone — both EMBED the production `theme.DirEnumerator` and are deliberately not declared-value fakes. Leave `fixtureThemeEnumerator` (`theme_seams_test.go`, `package tui_test`) alone — it is unavoidable across the package split.
5. Change no assertion.

**Acceptance Criteria**:
- `package tui` declares exactly one declared-value `ThemeEnumerator` fake.
- `recordingThemeEnumerator` no longer exists.
- Every former consumer records the same calls it did before (opens, keys, settings, slot loads, reassembles).
- The two embedding enumerators and the `tui_test` fixture enumerator are untouched.

**Tests**:
- All `internal/tui` theme panel open and entry tests pass unchanged.
- The surviving fake's `opens`/`keys` recording is asserted by at least one re-pointed test, so the counters are not dead.

## Task 11: Export the theme-file extension and drop the restatements
severity: low
sources: duplication

**Problem**: `internal/theme` keeps `themeExtension = ".theme"` unexported (`internal/theme/name.go:12`), so every other package that needs it restates it. `cmd/capturetool/main.go:55` declares its own `themeFileExtension` constant with a comment acknowledging the restatement ("a local restatement of the theme-file extension because internal/theme keeps its own copy unexported"), and `internal/capture/fixtures.go` carries twelve inline `".theme"` literals composing fixture filenames (lines 747, 758, 1035, 1036, 1037, 1070, 1077, 1084, 1257, 1264, 1272, 1275) — several of them concatenating a slug constant with the literal, so a filename can silently disagree with the slug declared beside it. The extension is a published user-facing contract — it is what `docs/theming.md` and the drop-in workflow print — so it is a magic string with more than one home rather than a package-private detail.

**Solution**: Export it as `theme.FileExtension` and route both packages through it.

**Outcome**: The extension is stated once, in the package that owns the file format, and the fixture filenames are composed from the same constant the loader compares against.

**Do**:
1. Export `FileExtension = ".theme"` from `internal/theme`, keeping the unexported `themeExtension` as an alias inside the package if the existing call sites read better that way. Move the "compared by exact bytes, never case-folded" note onto the exported constant.
2. Delete `cmd/capturetool`'s `themeFileExtension` and its restatement comment; `isThemePath` reads `theme.FileExtension`.
3. Replace the twelve inline literals in `internal/capture/fixtures.go` with `slug + theme.FileExtension` (or the exported constant directly where no slug is concatenated).
4. Do not change any resolved value, filename or comparison semantics — this is a constant move.

**Acceptance Criteria**:
- `theme.FileExtension` is exported and documented.
- No `".theme"` literal remains in `cmd/capturetool` or `internal/capture`.
- Every fixture filename that pairs with a slug constant is composed from that constant plus the exported extension.
- `internal/theme`'s own comparison behaviour is unchanged (exact bytes, no case folding).

**Tests**:
- Existing `internal/capture`, `cmd/capturetool` and `internal/theme` tests pass with no reference regeneration.
- A test asserting `theme.FileExtension` is what `SlugFromFilename` accepts and what an uppercase variant is rejected against (extend the existing name test rather than adding a parallel one).

## Task 12: Make the `Nomination` contract structural at both ends
severity: low
sources: architecture

**Problem**: Two ends of the same type carry a silent half-truth. (1) `AdaptivePair(a, b MemberPalette)` (`internal/theme/nomination.go:78`) folds its two arguments through `hold` (`nomination.go:86`), which assigns by the member tag each palette carries. The tagging prevents a swapped pair but not two palettes tagged with the SAME member: `AdaptivePair(MemberLight.Palette(x), MemberLight.Palette(y))` compiles, leaves `dark` as the zero `Theme`, and `Select(MemberDark)` then returns a palette whose every value resolves through `lipgloss.Color("")` — a silently colourless render, the exact failure mode the dark-default fallback exists to prevent elsewhere. Both current callers are provably safe (one uses the constants directly, one uses `m` and `m.Opposite()`), so correctness rests on caller discipline rather than on the constructor, and the doc comment's "BOTH MEMBERS MUST BE NAMED" is an unenforced instruction. (2) `themeState.nomination` (`internal/tui/theme_state.go:35`) is read in exactly two places, both inside the appearance gate's lifecycle (`newNominationGate` at arm, `Select` in `syncResolvedMode` — `internal/tui/model.go:1351`, `model.go:1406`), and the gate resolves once before first paint; after that nothing reads it. The constant-commit path leaves it holding the pre-commit pair; the constant→adaptive conversion rebuilds it from the newly-loaded slot and the currently PREVIEWED palette (`theme_panel_confirm.go:235`). So the field is stale on one path and freshly written-but-unread on the other, and the asymmetry gives no signal about which is authoritative — a future reader (a re-detect, a second gate arm) gets a half-truth from either.

**Solution**: Make the invalid pair unconstructible or loud, and pick one contract for the panel's nomination field and make it structural.

**Outcome**: A same-member pair cannot silently produce a colourless half, and the panel's nomination field either provably describes the current setting or is provably construction-time-only — with the choice stated on the field rather than inferred.

**Do**:
1. In `internal/theme`, make `AdaptivePair` unable to yield a half-empty nomination. Prefer taking the two halves as distinct parameters internally — `pairFor(light, dark Theme)` — with a thin `Member`-driven wrapper for the dynamic caller, so the type system carries the invariant. If that reshapes too many call sites, have `AdaptivePair` fall back to a defined half when both arguments name the same member, and make that fallback observable (it must never leave the zero `Theme` in a member slot).
2. Update the doc comment to state the enforced property rather than instructing the caller.
3. In `internal/tui`, choose ONE contract for `themeState.nomination` and implement it: either treat it as construction-time-only input — drop the assignment in `loadNewlyLiveSlot`, keeping the slot load for its `theme: loaded` event and the `canvasMode` update beside it — or maintain it on BOTH commit paths so the field always describes the current setting.
4. State the chosen contract on the field in one short line, so a later reader knows whether it may be trusted post-gate. Do not restate the reasoning; state the contract.
5. Preserve today's rendered behaviour exactly: the gate still resolves once before first paint, the `theme: loaded` event still fires on the constant→adaptive conversion, and the `canvasMode` update beside it is unchanged.

**Acceptance Criteria**:
- No call to the adaptive-pair constructor can produce a nomination with a zero `Theme` in either member.
- `themeState.nomination` follows one stated contract on both commit paths, and the contract is written on the field.
- The appearance gate's single-resolution behaviour, the `theme: loaded` event and the `canvasMode` update are unchanged.
- No rendered output changes.

**Tests**:
- A test asserting a same-member pair either cannot be expressed (compile-time, via the reshaped signature) or yields a nomination whose every member resolves to a non-empty palette — never the zero `Theme`.
- A test asserting `Select` returns a fully-populated palette for both members of every constructible adaptive nomination.
- A test pinning the chosen `themeState.nomination` contract: under "construction-time-only", the field is unchanged after a slot commit; under "always current", it matches the newly-persisted setting after BOTH a constant commit and a slot commit.
- Existing appearance-gate, capture fixture and swap-guard tests pass with no reference regeneration.
