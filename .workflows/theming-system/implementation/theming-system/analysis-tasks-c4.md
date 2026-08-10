# Analysis Tasks: theming-system (Cycle 4)

## Task 1: Single-source the 19-token synthetic probe palette into internal/themetest
severity: high
sources: duplication

**Problem**: The synthetic swap-probe palette is authored twice, independently, in two packages. `syntheticTheme(red int)` (internal/capture/theme_swap_guard_test.go:117) and `syntheticProbePalette(red uint8)` (internal/tui/restyle_repoint_test.go:49) are the same function written twice: the same `v(i)` closure over `fmt.Sprintf("#%02X%02X%02X", red, 0x80+i, 0xC8+i)`, the same 19-field `theme.Theme` literal in the same order with the same indices, and near-identical rationale comments. They are the probe palettes for the two halves of one mechanism — the swap-and-diff completeness guard and the restyle re-point guard — so drift between them is drift between the two guards that protect the live theme swap. The failure mode is silent in exactly the way both comments warn about: `theme.Theme` is a plain struct, so adding a 20th token to the canonical table leaves BOTH literals compiling with a zero-valued field, and a guard probing a zero token can neither diff nor detect a stale value. One edit is currently two edits in two packages, with nothing linking them.

**Solution**: One builder in `internal/themetest` (the existing shared theme-test vocabulary, imported by both packages), with a completeness assertion driven by the canonical token table so a new token fails both guards loudly instead of leaving either silently probing a zero value.

**Outcome**: The probe palette is authored once. Adding a 20th token to `theme.Theme` breaks the shared builder with a named failure rather than quietly voiding both guards, and the two halves of the swap mechanism cannot drift apart.

**Do**:
1. Add `internal/themetest/synthetic.go` beside `theme_file.go` / `builtin.go`, exporting `SyntheticPalette(t *testing.T, red uint8) theme.Theme`: the `v(i)` closure over `fmt.Sprintf("#%02X%02X%02X", red, 0x80+i, 0xC8+i)` and the 19-field `theme.Theme` literal, in canonical table order.
2. Give it the completeness assertion the two hand-written literals cannot have: after building, range `p.All()` and `t.Fatalf` if any token's `Value` is empty, and fatal if `len(p.All()) != len(theme.TokenNames())`. State in the doc comment that this is what makes a newly added token a loud failure in every guard that probes with it rather than a silent zero.
3. Pin the three-decimal-digit property the tui copy relies on (green `0x80+i` and blue `0xC8+i` are always ≥ 100 decimal; red must be too, so one token's rendered SGR core can never be a substring of another's): fatal when `red < 0x64`, and say why in the comment.
4. Add `SyntheticPair(t *testing.T, redA, redB uint8) (a, b theme.Theme)` for the disjoint-pair case, fatalling when `redA == redB`.
5. Merge the two rationale comments into one doc block on `SyntheticPalette` — shipped palettes are deliberately not used (a hex both palettes happen to share survives a swap legitimately; a token with the same value either side renders identically, so the assertion passes whether or not the site updated), plus the fixed-width-core reason from the tui copy. Do not carry any workflow vocabulary (task ids, phase numbers, spec-section citations) into the new comments.
6. Delete `syntheticTheme` / `syntheticPalettes` (internal/capture/theme_swap_guard_test.go:117, :146) and `syntheticProbePalette` (internal/tui/restyle_repoint_test.go:49); re-point `syntheticPalettes()`, `probeThemeBefore()` and `probeThemeAfter()` at the shared builder.
7. Keep the per-guard constants at their call sites — `syntheticRedA = 0x6E` / `syntheticRedB = 0xD2` (internal/capture/theme_swap_guard_test.go:93) and the tui probe's `0xAA` / `0xBB` — with their existing comments; only the builder moves.
8. `internal/tui/restyle_repoint_test.go` is in `package tui` (not `tui_test`); confirm the `internal/themetest` import introduces no cycle (`themetest` imports `internal/theme` and `testing` only) and that the unit lane is unchanged.
9. Change no assertion in either guard: the palettes produced must be value-identical to today's, so both suites pass with their claims untouched.

**Acceptance Criteria**:
- Exactly one synthetic-palette builder exists in the repo; neither `internal/capture` nor `internal/tui` declares a 19-field `theme.Theme` probe literal.
- The builder fails loudly (naming the empty token) when a token is added to `theme.Theme` and not to the builder.
- The builder fails loudly on `red < 0x64` and on a same-red pair.
- The palettes are value-identical to the ones the two guards use today; every existing assertion in both suites is unchanged and passes.
- `go test ./internal/capture ./internal/tui ./internal/themetest` passes; no test changes lane.

**Tests**:
- Both existing guard suites pass unchanged.
- A `themetest` test that `SyntheticPalette` gives all 19 tokens a distinct non-empty value in canonical order.
- A `themetest` test that `SyntheticPair` produces palettes sharing no token value.
- Temporarily add a 20th field to `theme.Theme`'s canonical table and confirm the builder fatals naming the empty token (proving both guards now fail loudly); revert.
- Temporarily set both guard pairs to the same red and confirm the pair helper fatals; revert.

## Task 2: Carry the token rename into the render layer's own comments
severity: medium
sources: standards

**Problem**: The token rename retired eleven names (`accent.violet`, `accent.blue`, `accent.cyan`, `accent.orange`, `state.green`, `state.red`, `text.strong`, `text.muted-bright`, `text.detail`, `text.dim`, `bg.warning`, `bg.track`, `text.on-warning`, `border.separator`, `border.footer`) and made the new names a public contract every theme file is written against. Every Go field, every `.theme` key and every call site was migrated — but the doc comments that describe which token each surface paints in were not. 328 references to retired names survive in production comments and a further ~446 in test comments and failure strings, across `internal/tui` and `internal/capture`: `header.go:33` still says the caret is `accent.violet`; `section_header.go:14` says `Sessions` is `accent.cyan` and the count `state.green`; `notice_band.go:45-47` maps all four band roles to retired names; `edit_modal.go:17` names `border.separator`, a token the consolidation removed; `build.go:164` and `capture/fixtures.go:531` describe the flash as sitting on `bg.warning` with a `text.on-warning` message. A maintainer or agent reading these files learns a vocabulary that no longer exists and would author a drop-in theme against it, with no compiler, guard or test catching it — the docs guard covers `docs/theming.md`, not source comments. The project standard is explicit: when a change makes a nearby comment false, fix it in the same edit.

**Solution**: Sweep `internal/tui` and `internal/capture` for the retired names and rewrite each reference to the token the adjacent code actually reads, then add a guard so the next reader cannot learn a dead vocabulary from these packages.

**Outcome**: Every comment in the render layer names a token that exists. The public contract has one vocabulary, and a retired name reappearing in these packages fails a test rather than surviving as documentation.

**Do**:
1. Sweep every `.go` file under `internal/tui` and `internal/capture` (production and test, comments and `t.Errorf`/`t.Fatalf` message strings) for the retired names, substituting: `accent.violet`→`accent.primary`, `accent.blue`→`accent.key`, `accent.cyan`→`accent.mode`, `accent.orange`→`accent.attention`, `state.green`→`state.positive`, `state.red`→`state.destructive`, `text.strong`→`text.secondary`, `text.muted-bright`→`text.tertiary`, `text.detail`→`text.muted`, `text.dim`→`text.subtle`, `bg.warning`→`bg.attention`, `bg.track`→`bg.subtle`, `text.on-warning`→`text.on-attention`, `border.separator`/`border.footer`→`border`.
2. Verify each substitution against the field the adjacent code reads rather than applying it blind — the comment must name the token that line actually paints from, and a few sites read a different token than their stale comment claims.
3. Leave the DELIBERATE references alone. Some comments name a retired token precisely because it is retired — the border-consolidation guards at internal/tui/active_theme_test.go:63,79,83,86 and internal/tui/help_modal_frame_test.go:42 assert a retired shade no longer renders. Rewrite the prose so each reads as a statement about a name that was removed, and keep the name.
4. Do NOT touch `internal/theme` (its retired-name references are all deliberate: absence guards and consolidation records), `docs/theming.md` (it names a retired token as a counter-example of a bad name), or anything under `testdata/vhs/`.
5. Add a guard in the same shape as the existing `colour_literal_guard_test.go` glob machinery: enumerate the `.go` files of `internal/tui` (and `internal/capture`) and fail when any retired name appears, with a small named exemption set for the deliberate sites from step 3, each carrying its reason in-source. The retired list must be one declared table.
6. Change no non-comment line other than the failure-message strings in step 1 and the new guard. No behaviour, no assertion, no test name changes.
7. Do not introduce workflow vocabulary (task ids, phase numbers, spec-section citations) into any comment written or rewritten here.

**Acceptance Criteria**:
- No retired token name appears in `internal/tui` or `internal/capture` except the exempted absence guards.
- Every rewritten comment names the token the adjacent code reads.
- The new guard fails when a retired name is reintroduced anywhere in the covered packages, and its exemption set is explicit and justified in-source.
- `internal/theme`, `docs/theming.md` and `testdata/vhs/` are untouched.
- `go test ./...` passes; `golangci-lint run` is clean.

**Tests**:
- The full unit lane passes — the change is comments, message strings and one new guard.
- Temporarily reintroduce a retired name in a non-exempt render file and confirm the guard fails naming file and token; revert.
- Confirm the guard does not fire on the exempted absence guards.
- A repository grep for each retired name over `internal/tui` and `internal/capture` returns only the exempted sites.

## Task 3: Set the in-force light/dark answer independently of the newly-live slot's load
severity: medium
sources: architecture

**Problem**: On a confirmed `d`/`l` over a constant setting, `loadNewlyLiveSlot` (internal/tui/theme_panel_confirm.go:190) calls `ThemeSource.ResolveSlot`, discards the returned `SlotResolution` entirely, and keeps only the error — then assigns `m.themeState.canvasMode = m.retainedCanvasAnswer()` only if that call succeeded. That couples two independent facts. `retainedCanvasAnswer` (theme_panel_confirm.go:235) is a pure read of the OSC 11 reply this launch already received — a fact about the terminal, computed from `bgReplyArrived`/`bgReplyDark` and nothing else — while the resolution it is gated behind is a palette load that can fail on the broken-builtin fatal. On that failure path the model silently keeps the pre-conversion dark answer, so a light terminal that has just converted to an adaptive pair resolves `inForceSlot` (theme_panel.go:621) onto the dark slot for the rest of the session, and nothing says so. The load itself is still required — it is the one theme load outside construction and the site that emits the commit-time `theme: loaded` — but its success has no bearing on which member the terminal is.

**Solution**: Assign the retained answer unconditionally on the conversion, and make the discarded-value call read as what it is: an announcement plus a fatal-surfacing site, not a source of the answer.

**Outcome**: The light/dark answer a conversion records is the terminal's classification in every case, including a failed slot load. The one keypress no longer has a path where the palette that failed to load also decides which half of the pair is in force.

**Do**:
1. In `loadNewlyLiveSlot` (internal/tui/theme_panel_confirm.go:190), move `m.themeState.canvasMode = m.retainedCanvasAnswer()` ABOVE the `ResolveSlot` call so it runs unconditionally on the conversion, and keep the early return for the error case as the load's own degrade path.
2. Rewrite the function's doc block so the two responsibilities are stated separately: (a) the conversion records the retained classification, which is independent of any load and cannot fail; (b) the load announces the newly-live slot and surfaces the broken-builtin fatal, and its palette is deliberately discarded because `applyCommittedSetting`'s own re-resolution is what puts both members in the nomination. Keep the existing statement that assigning the palette here as well would give one field two writers.
3. Keep the four ordering conditions the call site already documents (after a write that landed, past `commitSlot`'s own mirror and recompute, never on the nil-persister path, nothing renders between the two) — they still govern the LOAD. State plainly that they no longer govern the answer assignment, which needs only the conversion to have happened.
4. Do not change `ThemeSource`'s method set, `theme.ResolveSlot`, `commitPass`/`enumerationPass`, or the `theme: loaded` cadence. The seam stays as it is.
5. Do not touch `startupCanvasHex`, `themeState.active` or `gate.appearance` — their documented divergence from `canvasMode` is load-bearing and unaffected.

**Acceptance Criteria**:
- `canvasMode` is assigned on every constant → adaptive conversion that commits, including one where the slot load returns an error.
- The commit-time `theme: loaded` still fires exactly once per converting commit, carrying the same slug it does today.
- No `theme: loaded` fires on any other panel path (open, `Esc`, re-resolution, a non-converting slot commit).
- The seam's method set and every call signature are unchanged.
- `go test ./internal/tui ./internal/capture ./internal/theme` passes.

**Tests**:
- A test that a constant → adaptive conversion in a terminal whose OSC 11 reply was LIGHT resolves `canvasMode` to the light answer even when the slot load returns an error (fails before the change).
- A test that the same conversion with a successful load is unchanged in outcome.
- Existing commit/confirm tests pass unchanged: the palette does not swap on commit, badges refresh, and nothing moves on a resolve error.
- The existing `theme: loaded` cadence tests pass unchanged.

## Task 4: Single-source the per-package Go source-guard file enumeration
severity: medium
sources: duplication

**Problem**: The theming work added a family of AST source guards and each author wrote their own enumerate-and-parse front end. `parseCmdFiles` (cmd/open_theme_nomination_test.go:337) and `parsePackageFilesByName` (internal/tui/nomination_test.go:320) are byte-identical bodies — same `os.ReadDir(".")`, same `IsDir()`/`.go`/`_test.go` skip, same `parser.SkipObjectResolution`, same two `t.Fatalf` messages — differing only in the doc comment's package name. Inside `internal/tui` there are three more variants: `themeFlashProductionFiles` (theme_flash_precedence_test.go:259, glob-based with its own empty-match check), `parseProductionFiles` (theme_source_guard_test.go:105, named-subset) and an inline glob-parse loop in `themeRowDelegateLiteralSites` (theme_panel_test.go:917). `internal/theme` adds two more (`themeSourceFiles`/`parseThemeSources` at leaf_guard_test.go:214/194, and `exportedSymbols`' own ReadDir+parse at theme_test.go:402). The variants differ in ways that change what a guard covers — glob versus ReadDir, empty-match fatal versus silent, one shared `FileSet` versus one per file — so guards that read as siblings are quietly scoped differently, and a guard that silently matched no files would pass vacuously in three of them. The class was already recognised here: `portalbintest.GoSourceFiles` (internal/portalbintest/build.go) was added to single-source the repo-wide walk with exactly this rationale, but the per-package scans never joined it.

**Solution**: One package-local enumeration with one stated scope, in the same home as the repo-wide walk, with every per-package guard front end reduced to a projection over it.

**Outcome**: "What a package-local source guard covers" is one decision in one place. No guard is narrower than its siblings by accident, and none can pass vacuously on an empty match.

**Do**:
1. Add `PackageGoFiles(dir string, includeTests bool) ([]string, error)` beside `ProjectRoot` and `GoSourceFiles` in `internal/portalbintest` — one non-recursive enumeration of a package directory's `.go` files with the test-source rule as an explicit parameter, returning an error (never zero files silently). Document it as the single decision about what a package-local guard covers, the way `GoSourceFiles` documents the repo-wide one. `portalbintest` is stdlib-only and unit-lane reachable, so every caller below can import it.
2. Re-express `parsePackageFilesByName` (internal/tui/nomination_test.go:320) as: enumerate through the new helper, parse each with `parser.SkipObjectResolution` against one shared `FileSet`, fatal on an empty enumeration. Keep its signature and return shape.
3. Within `package tui`, re-point `themeFlashProductionFiles` (theme_flash_precedence_test.go:259) and `themeRowDelegateLiteralSites`' inline loop (theme_panel_test.go:917) at `parsePackageFilesByName`, keeping each guard's own assertions and failure wording. Delete the two private glob loops.
4. Within `package tui_test`, have `parseProductionFiles` (theme_source_guard_test.go:105) and `centralisedColourSites` (colour_literal_guard_test.go) take their file list from the shared enumeration. `parseProductionFiles`' named-subset parameter may stay — only the enumeration moves.
5. Within `package theme_test`, have `themeSourceFiles` (leaf_guard_test.go:214) and `exportedSymbols` (theme_test.go:402) share one enumerator through the same helper; delete the second ReadDir+parse.
6. In `cmd`, delete `parseCmdFiles` (cmd/open_theme_nomination_test.go:337) and route its callers through the shared enumeration + the same parse mode.
7. Make the vacuity rule uniform: every front end fatals when the enumeration is empty, naming the directory.
8. Change no guard's claim. Only root resolution, enumeration and parse-mode plumbing move; every forbidden-identifier / forbidden-literal / forbidden-import assertion stays exactly as it is.

**Acceptance Criteria**:
- One package-local enumeration exists in `internal/portalbintest`; no test declares its own `os.ReadDir(".")` or `filepath.Glob("*.go")` package scan.
- Every guard fatals rather than passing vacuously on an empty enumeration.
- Test-source inclusion is an explicit argument at each call site, matching what that guard covered before.
- Every guard still fails when its forbidden construct is reintroduced.
- `go test ./...` passes and no test moves lane.

**Tests**:
- A `portalbintest` test pinning the enumeration (files found, directories and non-`.go` skipped, test sources included/excluded per the flag, error on a missing directory).
- Temporarily reintroduce each guard's forbidden construct and confirm the guard still fails; revert each.
- Temporarily point one guard at an empty directory and confirm it fatals rather than passing; revert.

## Task 5: Extract the shared commit "no other I/O" test body in internal/tui
severity: medium
sources: duplication

**Problem**: `TestPanelEnter_NoOtherIO` (internal/tui/theme_panel_commit_test.go:541) and `TestPanelSlotCommit_NoOtherIO` (internal/tui/theme_panel_commit_slot_test.go:904) are the same 65-line body twice in the same package: identical doc comment word for word, identical prefs-file env setup, identical `writeThemeFileForTest` seed, identical `newCountingStores` + `fakeThemePersister` + `countingEnumeratorOver` wiring, an identical ten-field `Build(Deps{…})` literal, identical open/reset/opens capture, and five identical assertion blocks (seam-call count with the same seven-value message, enumeration count, nil cmd, empty config dir, single themes-dir entry). Exactly four things differ: the `theme.RawKeys` fixture, the keypress, the commit assertion, and the noun in the failure strings. Because the assertion messages are duplicated too, a change to what "no other I/O" means has to be applied twice and can be applied to one only — leaving the other commit path asserting a contract that has moved.

**Solution**: One helper in the shared panel test file holding the setup, the counters and the five assertions, with each test reduced to its own fixture, keypress and commit expectation.

**Outcome**: The "the prefs write is the only I/O this keypress may make" contract is stated once and both commit paths are held to whatever it says next.

**Do**:
1. Add `requireCommitDoesNoOtherIO(t *testing.T, keys theme.RawKeys, subject string, press func(*testing.T, Model) (Model, tea.Cmd), assertCommitted func(*testing.T, *fakeThemePersister))` to the shared panel test file (`internal/tui/theme_testing_test.go`).
2. Move into it: the `PORTAL_PREFS_FILE` env setup, the themes-dir seed, `newCountingStores`, the recording `fakeThemePersister`, `countingEnumeratorOver`, the construction-time resolution, the `Build(Deps{…})` literal, the size/sessions setup, the `t` keypress and open assertion, and the `stores.reset()` / `opens` capture.
3. Move the five assertions in too, taking the differing noun from `subject` so each failure still reads about the commit under test ("the commit" / "the slot commit").
4. Reduce `TestPanelEnter_NoOtherIO` and `TestPanelSlotCommit_NoOtherIO` to their own doc comment plus one call: their `RawKeys` fixture, their press helper (`pressCommitKey` / `pressSlotKey` with `slotDarkPress`), and their commit assertion (`requireCommitted` / `requireSlotCommits`).
5. Keep the shared doc comment's substance on the helper — the prefs call is the one write, everything else is asserted absent, and `internal/tui` resolves no config path so every route to disk runs through a counted seam. Keep each test's own comment down to what makes its case distinct. Do not carry workflow vocabulary into the moved comments.
6. Prefer the existing construction helpers in `theme_testing_test.go` for the `Build(Deps{…})` sequence if one already covers it, rather than adding a second construction path.
7. Change no claim: the seam count, the enumeration count, the nil-cmd, the empty-config-dir and the single-themes-dir-entry assertions all stay, with their information-carrying wording.

**Acceptance Criteria**:
- The "no other I/O" setup and its five assertions exist once in `internal/tui`.
- Both tests still assert their own commit landed with the value they expect.
- Failure messages still name which commit failed.
- `go test ./internal/tui` passes.

**Tests**:
- Both existing tests pass through the helper with unchanged expectations.
- Temporarily make one commit path touch a second seam and confirm only that test fails, naming it; revert.
- Temporarily make a commit path schedule a `tea.Cmd` and confirm the deferred-write assertion fires; revert.

## Task 6: Single-source the prefs absent-path case table and its "nothing was created" assertion
severity: medium
sources: duplication

**Problem**: `TestSaveMigrationMarker_DoesNotCreateAbsentFile` (internal/prefs/migration_marker_test.go:396) and `TestSaveTranslation_DoesNotCreateAbsentFile` (internal/prefs/translation_saver_test.go:161) are the same ~28-line body twice: the same two-case table (`{"the file is absent", []string{"prefs.json"}}`, `{"the parent directory is absent too", []string{"sub","nested","prefs.json"}}`), the same `filepath.Join(append([]string{dir}, c.rel...)...)`, the same `os.Stat` / `errors.Is(os.ErrNotExist)` assertion, the same `os.ReadDir` empty-tree assertion with the same message shape, and the same `assertNoTempFiles`. Only the call under test and one extra `persisted` check differ. The same case table appears a third time in `TestSave_CreatesAbsentFile` (store_write_path_test.go:165), which asserts the OPPOSITE outcome over identical inputs — so the inputs that decide create-versus-decline are written three times and can be edited in one. The package already has a shared-helper home (`seedPrefsFile`, `assertUntouched`, `assertNoTempFiles`, `undecodablePrefsCases` in store_write_path_test.go), which is where the table and the assertion belong.

**Solution**: One `absentPathCases()` table and one `assertNothingCreated` helper beside the existing shared prefs helpers, with all three tests reduced to the table plus their own outcome assertion.

**Outcome**: The absent-path inputs are stated once, so the create-on-absent saver and the decline-on-absent savers are proven against the same shapes, and a third shape added later covers all three with one edit.

**Do**:
1. Add `absentPathCases()` beside `undecodablePrefsCases()` in internal/prefs/store_write_path_test.go, returning the two-case table (file absent; parent directory absent too) with the `rel []string` shape both copies already use. Document it as the single statement of the absent-path shapes every saver is judged on.
2. Add `assertNothingCreated(t *testing.T, dir, path string)` beside `assertUntouched`: the `os.Stat` / `errors.Is(os.ErrNotExist)` check, the `os.ReadDir(dir)` empty-tree check with its existing message shape, and `assertNoTempFiles(t, dir)`.
3. Re-point `TestSaveMigrationMarker_DoesNotCreateAbsentFile` and `TestSaveTranslation_DoesNotCreateAbsentFile` at both, keeping each test's own call under test, its "declining to write is not a failure" fatal, and `SaveTranslation`'s extra `persisted == false` assertion inside the loop body.
4. Re-point `TestSave_CreatesAbsentFile` (store_write_path_test.go:165) at `absentPathCases()`, keeping its own opposite outcome assertion (`assertWrittenValue` on the decoded file) exactly as it is.
5. If any other saver in the package carries the same decline-on-absent contract without a test, do not add one here — this task moves shared scaffolding and adds no claim.
6. Keep every existing failure message that carries information about which saver failed.

**Acceptance Criteria**:
- One absent-path case table and one "nothing was created" assertion exist in `internal/prefs`; the duplicated bodies are gone.
- All three tests range over the same table.
- Each test keeps its own outcome assertion (two decline, one creates).
- `go test ./internal/prefs` passes.

**Tests**:
- All three existing tests pass through the shared table with unchanged expectations.
- Temporarily add a third absent-path shape and confirm all three tests exercise it with no other edit; revert.
- Temporarily make one decline-path saver create the file and confirm only that test fails, naming the created path; revert.

## Task 7: Share the doctor --fix stale-prune fixture and assertions between the two tests that make the claim
severity: medium
sources: duplication

**Problem**: `TestDoctorFix_ExistingRepairsUnchanged` (cmd/doctor_fix_theme_test.go:523) reproduces roughly 35 lines of `TestDoctorFixPrunesStaleEntriesThenRediagnosesClean` (cmd/doctor_test.go:1297) verbatim: the same `seedHealthyStateDir` / `seedHooksJSON("sessA:0.0")` / `seedProjectsJSON(liveDir, goneDir)` / `staleDeps(..., fakeHookLister{keys: []string{"sessB:0.0"}}, ...)` fixture, and the same five assertion blocks (hooks.json no longer contains `sessA:0.0`; projects.json no longer contains `goneDir`; projects.json still contains `liveDir`; the pruned-hook breadcrumb; the pruned-project breadcrumb) with byte-identical failure messages. The new test's own subject is only what it adds — the themes dir, the stale rotated log and the advisory line. Two tests now assert one repair contract, so a change to a breadcrumb string or a prune rule has to be made twice and can be made once.

**Solution**: Extract the shared fixture and the shared assertions into cmd's doctor test helpers, leaving each test holding only what it uniquely claims.

**Outcome**: The stale-prune repair contract is asserted from one place. The theme test states the theme surface's additive claim and inherits the existing-repairs claim rather than restating it.

**Do**:
1. Add `seedStalePruneFixture(t *testing.T, stateDir string) (deps doctorDeps, hooksPath, projectsPath, liveDir, goneDir string)` to cmd's doctor test helpers, holding the `seedHooksJSON` / `seedProjectsJSON` / `staleDeps` + `fakeHookLister` wiring both tests use. Keep `seedHealthyStateDir` at the call site if the two tests seed the state dir differently.
2. Add `assertStalePrunesApplied(t *testing.T, hooksPath, projectsPath, liveDir, goneDir, out string)` holding the five assertion blocks with their existing failure wording verbatim.
3. Re-point `TestDoctorFixPrunesStaleEntriesThenRediagnosesClean` (cmd/doctor_test.go:1297) at both, keeping any further claims it makes (the re-diagnose-clean exit, and anything else it asserts) in the test.
4. Re-point `TestDoctorFix_ExistingRepairsUnchanged` (cmd/doctor_fix_theme_test.go:523) at both, leaving in the test only its own subject: the themes-dir seed, the stale rotated log and its sweep assertion, and the advisory line.
5. Have `fixThemeFixture` (cmd/doctor_fix_theme_test.go:245) call `themesDirWith` (cmd/doctor_theme_test.go:32) instead of repeating its map-write loop.
6. Keep both tests' doc comments accurate to what each now claims — in particular the down-server / mass-deletion-hazard arm of the theme test stays exactly as it is.
7. Change no claim, no failure wording that carries information, and no `--fix` behaviour.

**Acceptance Criteria**:
- The stale-prune fixture and its five assertions exist once in `cmd`'s doctor test helpers; neither test restates them.
- Both tests still fail when a prune rule or a breadcrumb string changes.
- One themes-dir staging helper is used by `fixThemeFixture`; its inline write loop is gone.
- `go test ./cmd` passes.

**Tests**:
- Both existing tests pass through the shared helpers with unchanged expectations.
- Temporarily change a pruned-* breadcrumb string and confirm both tests fail from the one assertion site; revert.
- Temporarily disable the stale-project prune and confirm both tests fail; revert.

## Task 8: Delegate cmd's theme-file fixture helpers to internal/themetest and stage themes dirs one way
severity: low
sources: duplication

**Problem**: `internal/themetest` is documented as the single definition of the theme-file fixture format every consumer stages files in, exporting `Lines`, `WithValue`, `WithoutKey`, `Body` and `Write`, and `internal/theme` and `internal/tui` use it. The `cmd` theme suites grew a parallel set over built-in-derived lines: `themeSourceFromLines` (cmd/theme_test.go:614 — the same `strings.Join(lines,"\n")+"\n"` render, also open-coded at internal/theme/resolution_test.go:342), `sourceMissingTokens` (cmd/theme_test.go:641 — `WithoutKey` by another name), `sourceBadColours` (cmd/theme_test.go:655 — `WithValue` by another name) and `themeLineIndex` (cmd/theme_test.go:620). Their stated justification — no hex restated in Go, immune to a vocabulary change — is what `themetest.Lines()` already delivers by deriving names from `theme.TokenNames()`. Separately, three themes-dir staging helpers coexist in `cmd`: `themesDirWith` (doctor_theme_test.go:32), the inline map-write loop inside `fixThemeFixture` (doctor_fix_theme_test.go:245) and `seedThemesDir` (theme_test.go:116).

**Solution**: Give `internal/themetest` the one renderer and duplicate-key mutator `cmd` reaches past it for, delegate `cmd`'s wrappers to the shared mutators, and leave one themes-dir staging helper.

**Outcome**: The fixture-format vocabulary is defined once, so a change to what a valid theme file looks like reaches every suite. `cmd`'s helpers become named wrappers over shared behaviour rather than a second implementation of it.

**Do**:
1. Add `Render(lines []string) []byte` to `internal/themetest` — the body `Body()` already computes — and a duplicate-key mutator (e.g. `WithDuplicateKey(lines []string, key string) []string`) for the case `cmd` currently hand-rolls.
2. Re-express `sourceMissingTokens` (cmd/theme_test.go:641) over `themetest.WithoutKey` + `Render`, `sourceBadColours` (cmd/theme_test.go:655) over `themetest.WithValue` + `Render`, and any duplicate-key source over the new mutator. Keep the `cmd`-side names and signatures — they read well at their call sites; only the bodies delegate.
3. Delete `themeSourceFromLines` (cmd/theme_test.go:614) in favour of `themetest.Render`, and re-point the open-coded render at internal/theme/resolution_test.go:342 at it too.
4. Keep `themeLineIndex` only if a caller still needs a line position that the shared mutators do not express; otherwise delete it with its callers' use.
5. Leave `validThemeSource` (cmd/theme_test.go:93) as it is — the export tests genuinely need the built-in's own bytes.
6. Collapse the themes-dir staging to one helper: keep `themesDirWith` (cmd/doctor_theme_test.go:32), have `fixThemeFixture` (cmd/doctor_fix_theme_test.go:245) call it, and either express `seedThemesDir` (cmd/theme_test.go:116) as a one-file call into it or delete it if its callers can use `themesDirWith` directly.
7. Change no assertion and no fixture content: every staged file must be byte-identical to what it is today.

**Acceptance Criteria**:
- `cmd`'s theme-file fixture helpers delegate to `internal/themetest`; no second implementation of the render or the key mutations remains in `cmd` or `internal/theme`'s tests.
- One themes-dir staging helper is used across the `cmd` theme suites.
- Staged fixture bytes are unchanged.
- `go test ./cmd ./internal/theme ./internal/themetest` passes.

**Tests**:
- All existing `cmd` theme / doctor-theme / doctor-fix-theme tests pass with unchanged expectations.
- A `themetest` test that `Render` produces exactly `Body()`'s bytes for `Lines()`.
- A `themetest` test that the duplicate-key mutator produces the rejection the `cmd` suite expects.
- Temporarily rename a token in the canonical table and confirm the `cmd` fixtures follow without a `cmd` edit; revert.

## Task 9: Collapse the per-built-in test scaffolding so a new built-in enrols itself
severity: low
sources: duplication

**Problem**: The per-built-in test files carry parallel copies of the same scaffolding. `readNord` (internal/theme/builtins_nord_test.go:303) and `readTokyoNightDay` (builtins_tokyo_night_day_test.go:302) are the same helper twice (read the committed built-in's path, `t.Fatalf` on error, return the text) with near-identical doc comments, and `builtins_test.go` open-codes the same read twice more against `tokyoNightPath`. `TestNord_IsEnrolledInFloorChecks` (builtins_nord_test.go:102) and `TestTokyoNightDay_IsEnrolledInFloorChecks` (builtins_tokyo_night_day_test.go:93) are the same five-line assertion parameterised only by the slug constant. The three "the record lives in a `#` comment" guards repeat one loop shape — assert the shipped value via `valueFor`, require a `commentBlockAbove`, require each figure/marker within it, then sweep the remaining tokens for a falsely-claimed marker — over three differently-shaped record structs, two of which have identical fields. This cuts against the project's "adding a theme is adding a file" property: a fourth built-in needs a fourth copy of the reader and a fourth enrolment test rather than being picked up by enumeration.

**Solution**: One reader keyed by slug, one enrolment test ranging over the embedded set, and one assert-value-then-assert-comment-figures helper the three record guards call with their own records.

**Outcome**: Adding a built-in adds a file and its own value records; the reader and the enrolment coverage come for free from enumeration, which is the property the package claims for itself.

**Do**:
1. Add `readBuiltinFile(t *testing.T, slug string) string` beside the shared `valueFor` / `commentBlockAbove` / `declaresKey` helpers, resolving the committed built-in's path from the slug. Delete `readNord` and `readTokyoNightDay` and re-point their callers, including the two open-coded reads in `builtins_test.go`.
2. Replace `TestNord_IsEnrolledInFloorChecks` and `TestTokyoNightDay_IsEnrolledInFloorChecks` with ONE test ranging over `theme.BuiltinSlugs()` (internal/theme/builtins.go:100), asserting every embedded slug is enrolled in the contrast floor checks. A built-in absent from the floor checks must fail naming its slug.
3. Unify the three record structs into one shape (token, shipped value, required figures, optional marker) and lift the shared loop — assert the shipped value via `valueFor`, require a `commentBlockAbove`, require each figure/marker within it, then sweep the remaining tokens for a falsely-claimed marker — into one helper the three guards call with their own record slices.
4. Keep each guard's own records, its own test name and its own failure wording where the wording says which built-in and which derivation is at fault; only the loop and the reader move.
5. Do not weaken any claim: every value, figure and marker asserted today must still be asserted, and the falsely-claimed-marker sweep must still run per guard.
6. Do not carry workflow vocabulary into the moved comments.

**Acceptance Criteria**:
- One built-in file reader exists; the two per-theme readers and the open-coded reads are gone.
- One enrolment test ranges over the embedded set; adding a built-in requires no new enrolment test.
- One record shape and one assert-value-then-comment loop back the three comment guards.
- Every value/figure/marker asserted before is still asserted.
- `go test ./internal/theme` passes.

**Tests**:
- All existing built-in tests pass with unchanged claims.
- Temporarily add a fourth built-in file that is absent from the contrast floor checks and confirm the enrolment test fails naming its slug; revert.
- Temporarily change one shipped value without updating its comment record and confirm the corresponding guard fails; revert.
- Temporarily add a derivation marker to a token that has no record and confirm the falsely-claimed-marker sweep fails; revert.

## Task 10: Type the appearance gate's answer as theme.Member and delete the local enum
severity: low
sources: architecture

**Problem**: `internal/tui` keeps `canvasAppearance` (appearance_gate.go:46) — two-valued, dark deliberately first so the zero value is the no-answer fallback, documented as the gate's light/dark answer — alongside `theme.Member` (internal/theme/member.go:17), which is documented as "the light/dark ANSWER as a type", is also two-valued, is also dark-as-zero-value and is also load-bearingly ordered, bridged by `canvasAppearance.member()` (appearance_gate.go:65). `internal/tui` already imports `internal/theme`, so the domain type is reachable directly; the local enum buys no boundary and costs a second statement of an invariant both types call load-bearing, plus a conversion every consumer must route through (`syncResolvedMode` model.go:1405-1407, `inForceSlot` theme_panel.go:621, `retainedCanvasAnswer` theme_panel_confirm.go:235). This is unlike `theme.Slot` (three-valued — a position, not an answer) and `prefs.ThemeSlot` (forced by prefs being a no-logging leaf that must not import the token layer), both of which earn their separate existence.

**Solution**: Hold `theme.Member` as the gate's answer and delete `canvasAppearance` with its `member()` bridge, so "dark is the zero value" has exactly one home.

**Outcome**: One type carries the light/dark answer from detection through to member selection. No conversion site can read the correspondence the wrong way round, and the zero-value rule is stated once.

**Do**:
1. Change `appearanceGate.appearance` (internal/tui/appearance_gate.go:103) and `themeState.canvasMode` to `theme.Member`, and change `resolve`/`resolveDark`/`resolveFromDark` to take and store `theme.MemberDark` / `theme.MemberLight`.
2. Change `retainedCanvasAnswer` (theme_panel_confirm.go:235) to return `theme.Member`.
3. Delete `canvasAppearance`, `appearanceDarkCanvas`, `appearanceLightCanvas` and `canvasAppearance.member()`; re-point every reference (~191 occurrences across `internal/tui`, mostly test assertions) at the domain constants.
4. Move the load-bearing zero-value rationale out of the deleted enum's comment: `theme.Member` already states it, so the gate's comment should say only that the gate's unresolved value is the dark no-answer fallback BECAUSE the member's zero value is, and point at the domain type rather than restating the rule.
5. Preserve every documented divergence exactly: `canvasMode` still diverges from `gate.appearance` after a mid-session constant → adaptive conversion, `startupCanvasHex` still does not move with `active`, and the NO_COLOR carve-out still paints nothing.
6. Do not touch `theme.Slot` or `prefs.ThemeSlot` — both are separately justified and stay.
7. This is a type change only: no behaviour, no resolution order, no emission changes.

**Acceptance Criteria**:
- The identifiers `canvasAppearance`, `appearanceDarkCanvas`, `appearanceLightCanvas` and the `member()` bridge appear nowhere in the repo.
- The "dark is the zero value" rule is stated once, on `theme.Member`.
- Detection, first-paint gating, member selection, the mid-session divergence and the NO_COLOR carve-out are behaviourally unchanged.
- `go build ./... && go test ./...` pass; `golangci-lint run` is clean.

**Tests**:
- Existing appearance-detection, canvas-paint, panel-cursor and background-restore tests pass with the renamed constants and no behavioural change.
- A test that an unresolved gate's answer is `theme.MemberDark` (the zero value carries the fallback).
- A test that the mid-session conversion's `canvasMode` divergence from `gate.appearance` still holds.
- `grep -rn "canvasAppearance"` over the repo returns nothing.

## Task 11: Split theme_panel.go along the concern lines its siblings and its tests already follow
severity: low
sources: architecture

**Problem**: The theme panel is otherwise split along clean concern lines — commit (351 lines), message (312), confirm (273), row (286), footer (103), seams (37), state (153) — but `internal/tui/theme_panel.go` alone holds five concerns in 1459 lines, four times the next-largest panel file: pinned copy and geometry constants (:37), the whole width/height/floor arithmetic (`themePanelWidthFor`, `themePanelMinHeight`, `themePanelChromeRows`, `themePanelFloor`, `themePanelListSize`, `themePanelInnerWidth`), list construction and open/close lifecycle (:216), key routing and cursor skipping, and the entire render/composite stack (`renderThemePanel`, `themePanelHeaderBlock`, `themePanelDirRow`, `themePanelBlock`, `themePanelPadRow`, the painter, `overlayThemePanel` — :988 onward). The test surface has already split along the natural seams (`theme_panel_geometry_test.go`, `theme_panel_chrome_test.go`, `theme_panel_keymap_test.go`) while production has not, so the layout arithmetic a reader most often needs is interleaved with input routing.

**Solution**: Lift the geometry block and the render/composite block into files named for what the tests already call them, leaving `theme_panel.go` as the panel's state, lifecycle and input routing.

**Outcome**: A reader looking for the panel's layout arithmetic or its composite opens the file named for it, and each panel file carries one concern as every sibling does.

**Do**:
1. Create `internal/tui/theme_panel_geometry.go` and move the width/height/floor arithmetic into it: `themePanelWidthFor`, `themePanelMinHeight`, `themePanelChromeRows`, `themePanelFloor`, `themePanelListSize`, `themePanelInnerWidth`, and the geometry constants they read.
2. Create `internal/tui/theme_panel_render.go` and move the render/composite stack into it: `renderThemePanel`, `themePanelHeaderBlock`, `themePanelDirRow`, `themePanelBlock`, `themePanelPadRow`, the panel painter and `overlayThemePanel` (with its `overlayThemePanelOnContent` companion if it lives beside it).
3. Leave in `theme_panel.go`: the panel's state struct, the pinned copy constants, list construction, `newThemePanelList` with its `pinArrowOnlyNav` pin, open/close lifecycle, key routing and cursor skipping.
4. Give each new file a package-level doc comment naming its single concern, and keep every moved function's existing doc comment verbatim.
5. Move code only. No signature, no visibility, no behaviour and no comment content changes — this must be a pure relocation, reviewable as such.
6. Keep the `theme_panel*.go` prefix so the panel's files still group together, and do not rename any function.
7. Confirm the source guards that enumerate this package's files (the colour-literal guard, the theme-flash precedence guard, the row-delegate literal guard) still cover the moved code — they glob the package, so a moved function must stay in scope.

**Acceptance Criteria**:
- `theme_panel.go` holds state, lifecycle and input routing only; geometry and render/composite live in their own files.
- No function is renamed, no signature changes and no comment content changes.
- Every package-file-enumerating guard still covers the moved functions.
- Rendered output is byte-identical: the capture fixture frames and the swap-and-diff guard pass unchanged.
- `go build ./... && go test ./...` pass; `golangci-lint run` is clean.

**Tests**:
- All existing panel geometry / chrome / keymap / fixture tests pass unchanged.
- `internal/capture` panel fixture frames are byte-identical.
- Temporarily introduce a raw hex literal in each new file and confirm the colour-literal guard fails on it; revert.

## Task 12: Make NewSilentLoader the only route to a silent theme loader
severity: low
sources: architecture

**Problem**: `NewSilentLoader` (internal/theme/load.go:74) exists precisely so "this caller writes nothing" is an explicit, greppable decision — "a constructor rather than a shape each assembles for itself", because the `theme` component records where a theme is USED and never where one is DIAGNOSED. But `NewLoader(nil)` (load.go:59) produces a behaviourally identical loader (a nil `*EventLogger` is a valid silent seam), and that is what PRODUCTION reaches for: `internal/tui/builtin_themes.go:20` seeds the shipped dark built-in through `theme.NewLoader(nil)`. With the zero-value `Loader{}` (silent but reserving nothing) that is three shapes of silence with two different reservation semantics behind one loosely-typed constructor parameter. The consequence today is that the emission policy cannot be audited by grepping for the named constructor.

**Solution**: One public route to silence — `NewSilentLoader` — with `NewLoader` no longer accepting the omission.

**Outcome**: Every silent loader in the repo is a stated decision at its call site, so "where does Portal deliberately write no `theme` records" is answerable by one grep.

**Do**:
1. Make `NewLoader` reject a nil seam rather than silently accepting it: either take a non-optional `EventLogger` value, or keep the pointer and panic on nil with a message pointing at `NewSilentLoader`. Prefer the type-level route if it does not force churn on the emitting call sites.
2. Re-point `internal/tui/builtin_themes.go:20` (`defaultDarkTheme`) at `theme.NewSilentLoader()`, and extend its existing comment to say the seed writes nothing because it is a seed rather than a use.
3. Re-point `internal/themetest/builtin.go:23` and the test call sites currently using `theme.NewLoader(nil)` (internal/tui/theme_panel_commit_test.go:552, internal/tui/theme_panel_behaviour_test.go:104, internal/tui/theme_row_test.go:513, internal/theme/resolution_test.go:38) at `NewSilentLoader()`.
4. Keep the reservation semantics distinction explicit: `NewSilentLoader` reserves every built-in slug exactly as `NewLoader` does, and the zero-value `Loader{}` reserves nothing. Say so where the zero value is legitimately used, if it is used anywhere.
5. Do not change what any loader judges. Reservation, the rejection ladder and every emission that does happen stay exactly as they are.

**Acceptance Criteria**:
- `NewLoader(nil)` is not constructible without a loud failure; `grep -rn "NewLoader(nil)"` returns nothing.
- Every silent loader in the repo is constructed through `NewSilentLoader`.
- The built-in seed still resolves the shipped dark palette and still writes no records.
- `go build ./... && go test ./...` pass; `golangci-lint run` is clean.

**Tests**:
- Existing loader, resolution and emission tests pass unchanged.
- A test that `NewSilentLoader` reserves every built-in slug (a user file cannot shadow a built-in under it).
- A test that the built-in seed path emits no `theme` records, with a vacuity guard proving the sink is live.

## Task 13: Report the bad-name extension cause only when the slug portion is actually legal
severity: low
sources: standards

**Problem**: Two distinct `bad name` lines exist, and the split's whole justification is that the extension-casing message is a distinct message BECAUSE the slug portion is already legal — sending the user to fix the one thing that is fine is the misdirection the surrounding copy rules discriminate against. But `SlugFromFilename` (internal/theme/name.go:148) checks the extension first and short-circuits, so a file whose stem is ALSO illegal (`My Theme.THEME`, `Nord.THEME`) is reported as `BadNameExtension`, and doctor prints "extension must be lowercase .theme" (cmd/doctor_theme.go:440). The user renames to `My Theme.theme`, re-runs doctor, and is told the slug is wrong — a two-step correction on a surface whose whole purpose is naming the single fix. The line's premise is unverified at the point it is claimed.

**Solution**: Claim the extension cause only where it is true — the stem must also satisfy `ValidSlug` — and let the general slug message answer a name that is wrong in more than one way.

**Outcome**: The extension advisory means exactly what it says: fix the extension and the file is usable. A name wrong in two ways gets the general message and one correction step.

**Do**:
1. In `SlugFromFilename` (internal/theme/name.go:148), when the exact lowercase `.theme` suffix does not match, derive the stem by stripping a case-insensitive `.theme` suffix and check it against `ValidSlug`: legal stem → `BadNameExtension`; illegal stem → `BadNameSlug`. A base name with no `.theme`-shaped suffix at all stays `BadNameExtension`.
2. Keep the ladder order, the single `bad name` reason class and `badName` as the one rejection constructor. Only which cause is reported changes.
3. Keep the no-normalisation property exactly as it is: the case-insensitive strip is used ONLY to judge the stem, is never returned, and no slug is ever minted from a non-exact extension. On any rejection the returned slug stays empty.
4. Rewrite the ordering paragraph in the function's doc comment, which currently says the extension is decided "before anything is asked of its stem" — that is what changes. State the new rule and why: the extension message asserts the stem is fine, so it is claimed only where that has been checked.
5. Leave `badNameAdvisoryLine` (cmd/doctor_theme.go:440), both advisory formats and the panel row's reason label untouched — they already render whichever cause they are given.
6. Update any test that pins a doubly-illegal name to the extension cause, and state in its comment which single fix the message now names.

**Acceptance Criteria**:
- `Nord.THEME` and `My Theme.THEME` report `BadNameSlug`; `nord.THEME` and `sunset.Theme` still report `BadNameExtension`.
- A name with no `.theme`-shaped suffix still reports `BadNameExtension`.
- No slug is minted from a non-exact extension, and nothing is normalised or returned lowercased.
- The rejection ladder order and the single `bad name` reason class are unchanged.
- `go test ./internal/theme ./cmd` passes.

**Tests**:
- A table over: legal stem + wrong-case extension (extension cause), illegal stem + wrong-case extension (slug cause), illegal stem + exact extension (slug cause), legal stem + exact extension (slug returned), no `.theme`-shaped suffix (extension cause).
- A doctor test that a doubly-illegal filename renders the slug advisory line, not the extension one.
- A test that the reserved-name check stays exact string equality and `Nord.theme` beside the built-in `nord` still yields no slug.

## Task 14: Correct CLAUDE.md's keymap claim and add the theme slide-over to the render inventory
severity: low
sources: standards

**Problem**: CLAUDE.md is what an implementing agent reads first, so a stale entry actively misdescribes the subsystem. Two entries are now false. (a) The "Modern Vivid TUI → Keymap revision" bullet (CLAUDE.md:177) states `pinArrowOnlyNav` "strips them from the live `bubbles/list` keymap on **both** Sessions and Projects" — there are three call sites: model.go:1264, model.go:1302 and theme_panel.go:406, where the in-source comment records that the pin is load-bearing for the panel specifically, because the v2 default binds `l` and `d` to NextPage and those collide with the panel's own commit keys. (b) The same bullet's render-structure inventory (CLAUDE.md:176) enumerates every modal and chrome file in `internal/tui` and omits the theme slide-over entirely, so the feature's largest new render surface has no entry in the architecture doc.

**Solution**: Amend the `pinArrowOnlyNav` clause to name all three lists with the panel's own reason, and add the slide-over to the render-structure inventory alongside the modals.

**Outcome**: Both entries describe the code as it is, and an agent reading CLAUDE.md first learns that the theme panel exists and why its nav pin is load-bearing.

**Do**:
1. Amend the keymap-revision clause at CLAUDE.md:177 to say `pinArrowOnlyNav` strips them from the live `bubbles/list` keymap on all three lists — Sessions, Projects and the theme slide-over — noting the `l`/`d` collision with the panel's commit keys as the panel-specific reason.
2. Add the theme slide-over to the render-structure inventory at CLAUDE.md:176, in the same register as the modal entries: name its files (`theme_panel.go` and its `_commit` / `_confirm` / `_message` / `_footer` / `theme_row.go` / `theme_seams.go` / `theme_state.go` siblings, plus any file Task 11 creates if that task lands first) and its composite relationship to the page beneath it. One or two sentences — match the density of the surrounding bullet.
3. Verify the rest of that bullet against the code while editing it and correct anything else it now misdescribes; do not expand its scope beyond render structure and the keymap revision.
4. Change no other CLAUDE.md section.
5. State facts checkable against the code — no workflow vocabulary, no task or phase references.

**Acceptance Criteria**:
- CLAUDE.md names all three `pinArrowOnlyNav` call sites and the panel-specific reason.
- The render-structure inventory includes the theme slide-over.
- Every claim in the amended bullet is checkable against `internal/tui` as it stands.
- No other section of CLAUDE.md is modified.

**Tests**:
- Manual verification: `grep -rn "pinArrowOnlyNav" --include=*.go` returns exactly the three call sites CLAUDE.md now names.
- Manual verification: every file named in the added slide-over entry exists in `internal/tui`.
- `go test ./...` passes (documentation-only change).
