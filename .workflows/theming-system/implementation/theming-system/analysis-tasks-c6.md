# Analysis Tasks: theming-system (Cycle 6)

## Task 1: Single-source the panel's stub theme-seam fixtures
severity: medium
sources: duplication

**Problem**: Four separate fixture builders in `internal/tui` hand-roll the same two `theme.Resolution` shapes and the same surrounding stub `Deps` skeleton. The constant shape — `Nomination: theme.ConstantNomination(target.Theme)` plus a single `{Slot: theme.SlotConstant, Requested: cursorSlug, Resolved: cursorSlug, Theme: target.Theme}` record — is written verbatim in `newArrowPanelDeps` (theme_panel_arrow_test.go:132) and `newSplitPanelModel` (theme_panel_commit_recompute_test.go:510). The adaptive-pair shape — a `ConstantNomination(dark.Theme)` plus `{Slot: SlotLight, …}` and `{Slot: SlotDark, …}` records — is written verbatim in `commitPairPanelDeps` (theme_panel_commit_test.go:106) and `newSlotSplitPanelModel` (theme_panel_commit_slot_test.go:136). Each site also re-assembles the same `Deps{Lister: fakeLister{}, Theme: …, ThemeSource: &fakeThemeSource{…}, ThemeKeys: …}` skeleton. `theme.Badges` and `inForceSlot` read `Requested` and `Resolved`, so a fixture that transposes those two fields in one file makes only that suite's badges wrong — the copies can disagree and each still passes, which is exactly the drift this fixture family cannot afford. The shared home already exists (`internal/tui/theme_testing_test.go`) and the sibling fixture consolidations already landed there.

**Solution**: Declare the two resolution shapes and the stub `Deps` skeleton once in `internal/tui/theme_testing_test.go`, and reduce the four builders to what genuinely differs between them.

**Outcome**: "What the panel's seam answers with" is stated once. A `Requested`/`Resolved` transposition is not writable per-file, and a new panel suite gets the correct shape by calling the helper rather than by copying a neighbour.

**Do**:
1. Add `constantResolution(slug string, th theme.Theme) theme.Resolution` to `internal/tui/theme_testing_test.go`, returning the constant nomination plus the single `SlotConstant` record with `Requested` and `Resolved` both set from `slug`.
2. Add `pairResolution(light, dark theme.Row) theme.Resolution` beside it, returning the pair's nomination plus the `SlotLight` and `SlotDark` records built from each row's slug and palette, in that order.
3. Add a `stubPanelDeps(source *fakeThemeSource, nomination theme.Nomination, keys theme.RawKeys) Deps` skeleton beside them, carrying the `Lister`/`Theme`/`ThemeSource`/`ThemeKeys` assembly the four builders share.
4. Re-point `newArrowPanelDeps` and `newSplitPanelModel` at `constantResolution`, and `commitPairPanelDeps` and `newSlotSplitPanelModel` at `pairResolution`; route all four through `stubPanelDeps`. Each builder then differs only in the rows it lists, its `enumeration`/`reassembled` priming, and whether a persister is wired.
5. Where a builder's existing literal disagrees with the shared derivation, the shared derivation is the correct one — if an assertion then fails, fix that fixture's rows, never re-hardcode a field back into the builder.
6. Change no assertion and no test name. Only how each fixture's resolution and `Deps` are constructed moves.
7. Do not add an options struct or a variadic-functional-option shape to the helpers; keep them plain functions, as the sibling helpers in this file are.

**Acceptance Criteria**:
- No `theme.Resolution` literal remains in `internal/tui`'s panel test files; all four builders construct through the two shared helpers.
- The four builders no longer restate the `Lister`/`Theme`/`ThemeSource`/`ThemeKeys` skeleton.
- `Requested` and `Resolved` are set in exactly one place per shape.
- Every existing panel arrow / commit / recompute / slot-commit assertion still holds.
- `go test ./internal/tui` passes.

**Tests**:
- Existing arrow, commit, commit-recompute and slot-commit suites pass unchanged against the shared fixtures.
- A test pinning `constantResolution` (one `SlotConstant` record, `Requested == Resolved == slug`) and `pairResolution` (a `SlotLight` and a `SlotDark` record carrying their own row's slug and palette).
- Temporarily transpose a row in one caller and confirm the badge assertion in that suite fails, proving the helper is load-bearing rather than cosmetic; revert.

## Task 2: Extract the panel open/close "writes nothing" proof
severity: medium
sources: duplication

**Problem**: `TestPanelOpen_WritesNothing` (internal/tui/theme_panel_cursor_test.go:538) and `TestPanelClose_WritesNothing` (internal/tui/theme_panel_close_test.go:342) each carry the same two subtests — "a present prefs.json survives byte for byte" and "an absent prefs.json stays absent and the directory is untouched" — with verbatim-parallel bodies for roughly 45 lines each: the same `{"session_list_mode":"by-project","theme":"sunset"}` seed constant, the same `os.WriteFile` + `t.Setenv("PORTAL_PREFS_FILE", …)` staging, the same `writeThemeFileForTest` fixtures (an invalid `"not-a-colour"` drop-in and a valid `"#101010"` one), the same read-back comparison and the same config-dir / themes-dir entry counts. The only differences are the model constructor (`themeCursorModel` vs `newClosePanelModel`), the action under test, and the noun in each error string. Both are proving one invariant — the panel's read-only paths touch no file — and the proof is authored twice. The moment the prefs-file seam or the theme-file staging changes, one copy keeps proving something about a fixture the other no longer uses, and the weaker copy passes without proving anything.

**Solution**: Own the proof once in the shared `internal/tui/theme_testing_test.go` home, parameterised by the model constructor, the action and the verb used in the messages.

**Outcome**: The panel's no-write guarantee is stated once and both entry points are held to the identical staging and the identical assertions. A change to the prefs seam or the theme fixtures updates one body.

**Do**:
1. Add `requireNoPrefsOrThemesWrite(t *testing.T, verb string, act func(t *testing.T, dir string, keys theme.RawKeys))` to `internal/tui/theme_testing_test.go`, owning both subtests, the persisted-JSON seed constant, the two directory fixtures and every assertion.
2. Have the helper stage: a themes directory containing the invalid and valid theme files via the existing `writeThemeFileForTest`, a `prefs.json` seeded with the constant (present case) or absent (absent case), and `PORTAL_PREFS_FILE` pointed at it.
3. Assert, in both subtests, that the prefs bytes are unchanged (or the file still absent), and that the config directory and themes directory entry counts are unchanged. Compose the failure messages from the `verb` argument so each caller's wording stays specific ("opening" / "closing").
4. Reduce `TestPanelOpen_WritesNothing` and `TestPanelClose_WritesNothing` to one call each, passing their own constructor (`themeCursorModel` / `newClosePanelModel`) and the keypress or action under test.
5. Keep both test names and both subtest names exactly as they are, so the failure surface a maintainer greps for does not move.
6. Change no assertion's meaning. If the two copies currently assert slightly different entry counts or comparisons, take the stronger of the two and confirm both callers still pass.

**Acceptance Criteria**:
- The ~45-line staging/assertion body exists once; neither test file declares the seed constant, the fixture staging or the read-back comparison.
- Both tests still name their own action and still fail with a message naming which action wrote.
- Both subtest names are unchanged.
- The helper fails when the panel is made to write on either path.
- `go test ./internal/tui` passes.

**Tests**:
- Both existing tests pass through the helper, unchanged in name and outcome.
- Temporarily make the panel's open path write `prefs.json`, confirm the open test fails naming the opening verb, and confirm the close test still passes; revert.
- Temporarily make the close path write, confirm the mirror; revert.

## Task 3: Share the doctor --fix down-server deferral fixture between the two tests that claim it
severity: medium
sources: duplication

**Problem**: The second subtest of `TestDoctorFix_ExistingRepairsUnchanged` (cmd/doctor_fix_theme_test.go:560-610, "the hazard guard still defers on a down server") is a near-verbatim clone of the pre-existing `TestDoctorFixDownServerPrunesProjectsButNotHooks` (cmd/doctor_test.go:1427): identical `seedHooksJSON` / `seedProjectsJSON` staging, an identical `DoctorDeps` literal (same `StateDir`, `ServerRunning: func() bool { return false }`, `HookLister: fakeHookLister{keys: []string{}}`, `Detector`, `Resolve`) down to the same inline comment, and the same three assertions (hooks byte-identical, `projects.json` no longer carrying the stale dir, `ErrDoctorUnhealthy`). Only the `ThemesDir` seam and the advisory-suffix expectations are new. The sibling subtest in the same test already routes through the shared `seedStalePruneFixture` / `assertStalePrunesApplied` helpers (cmd/doctor_test.go:950, :969), so the shared-fixture route exists and was simply not taken here. Two statements of the down-server repair contract can disagree — a seam flipped in one copy (a `ServerRunning` that answers true, a `HookLister` seeded differently) leaves that copy asserting a different claim under the same test name.

**Solution**: Give the down-server deferral one fixture and one assertion helper, beside the stale-prune pair they already sit next to, and have both tests consume them.

**Outcome**: The down-server repair contract is staged and asserted in one place. The theme test adds only what is genuinely its own — the `ThemesDir` seam and the advisory suffix.

**Do**:
1. Add `downServerDeferFixture(t *testing.T, stateDir string) (deps *DoctorDeps, hooksPath, projectsPath, goneDir string)` beside `seedStalePruneFixture` in cmd/doctor_test.go, owning the hooks/projects seeding and the full `DoctorDeps` literal including `ServerRunning` returning false.
2. Add `assertDownServerDeferral(t *testing.T, hooksBefore []byte, hooksPath, projectsPath, goneDir string, err error)` beside `assertStalePrunesApplied`, owning the three assertions (hooks byte-identical, `projects.json` no longer containing the stale dir, `ErrDoctorUnhealthy`).
3. Re-point `TestDoctorFixDownServerPrunesProjectsButNotHooks` at both helpers.
4. Re-point the "the hazard guard still defers on a down server" subtest at both helpers; it then only sets `deps.ThemesDir` and adds its own advisory-suffix checks after the shared assertion call.
5. Move the inline comment explaining WHY hooks are not pruned on a down server into the fixture helper, so the reason is stated once rather than copied.
6. Change no assertion's meaning and no test name. If the two copies differ anywhere today, take the stronger form and confirm both callers pass.
7. Keep the helpers in cmd/doctor_test.go beside their siblings; do not start a new helper file.

**Acceptance Criteria**:
- Neither test declares its own down-server `DoctorDeps` literal or its own hooks/projects seeding.
- The theme subtest's only additions over the shared helper are `ThemesDir` and the advisory-suffix assertions.
- The "why hooks are not pruned on a down server" reason appears once.
- Both test names are unchanged and both still fail if the down-server contract changes.
- `go test ./cmd` passes.

**Tests**:
- Both existing tests pass through the shared helpers with unchanged names and outcomes.
- Temporarily make `--fix` prune hooks on a down server and confirm BOTH tests fail; revert.
- Temporarily make the down-server path skip the projects prune and confirm both fail; revert.

## Task 4: Rewrite the topic's production comments that name a test or count call sites
severity: medium
sources: standards, duplication

**Problem**: `code-quality.md` bars two comment categories outright — "Claims about tests — what a test pins, catches, or proves. A renamed test or moved assertion turns the claim into a confident lie" and "Cardinality claims — 'the single caller', 'the only site that…'. Falsified by ordinary additive change far from the comment". Both classes are present again across the topic's production files. Test/guard claims: `internal/theme/load.go:26-30` ("a source guard keeps it out of production code"); `internal/theme/resolve.go:31-33` and `internal/theme/builtins.go:145-147` ("build-time validation of the embedded set is what makes it unreachable" / "exists to make impossible"); `internal/theme/union.go:267` ("Always valid — build-time validation of the embedded set makes that true"); `internal/tui/theme_panel.go:26` ("keeps the colour-literal guard and the swap-and-diff guard satisfied"), `:193` ("a paginating fixture exercises the dots so the swap-and-diff guard is not blind at that site"), `:726-727` ("identical before and after a swap, which the swap-and-diff guard structurally cannot see"), and `:544-549`, which describes what today's fixture set is built from and is falsified by the next fixture added. Cardinality claims: `internal/theme/load.go:207` ("the only implementation of it"); `internal/theme/theme.go:86-87` ("there is deliberately no second list of token names anywhere in production code"); `internal/theme/builtins.go:56` and `:161` and `internal/theme/union.go:183` (claims about which package can reach the built-in lookup); `cmd/theme_persister.go:22-23` ("the emission site for `theme: commit failed`, which otherwise has none"); `cmd/open.go:789` ("the ONE construction-time theme read"). Separately, `LoadPath`'s doc comment (internal/theme/load.go:190) claims "Rungs 3 to 6 are the same code LoadFile and LoadBuiltin run", which is true of rungs 4-6 (`parseThemeBytes`) but not of rung 3 — the read and its `unreadable` classification are written out in both functions, so the comment over-claims what is shared. Each of these is load-bearing as an *invariant*, but the guarantee lives in a test whose name and location are free to move, or in a call count ordinary additive change will alter, and the comment then states something false in the file a reader trusts most.

**Solution**: One sweep over the named sites, restating each as the invariant the code needs — without naming its enforcer, without counting call sites, and without describing today's fixture inventory — and correcting the one comment that over-claims what two functions share.

**Outcome**: Every surviving comment on these surfaces holds true for a reader with no knowledge of the tests, and stays true when a test is renamed, a caller is added or a fixture is introduced.

**Do**:
1. Rewrite each test/guard claim to state the property the code depends on, with no reference to a test, guard or fixture: "a built-in is always valid" rather than "build-time validation makes that true"; "the panel's list caches its styles at open, so the restyle path must re-point them" rather than citing a guard's blindness.
2. Rewrite each cardinality claim to state the rule rather than the count: at `cmd/theme_persister.go:22-23`, why the failure is recorded here (prefs is a leaf that must not log, the write needs prefs path resolution) without asserting it is recorded nowhere else; at `cmd/open.go:789`, that the keys handed to the panel are the construction-time snapshot and are never re-read, without asserting there is exactly one read; at `internal/theme/load.go:207`, `theme.go:86-87`, `builtins.go:56`/`:161` and `union.go:183`, the constraint each protects, without counting implementations, lists or reachable packages.
3. Delete outright any comment whose only content is a pointer at a test or a guard — `internal/tui/theme_panel.go:193` and the guard clause of `internal/theme/load.go:26-30`. The fixture and the guard already encode the relationship; the sentence adds nothing the code needs.
4. Rewrite `internal/tui/theme_panel.go:544-549` so it states the rule about unselectable rows and the cursor without describing what today's fixtures are built from.
5. Correct `LoadPath`'s doc comment (internal/theme/load.go:190) to claim only what is actually shared — the content rungs performed by `parseThemeBytes` — so the sentence stops asserting that the read is shared code when it is written in both functions.
6. Keep the genuine trap warnings intact: the `startupCanvasHex` freeze and canvas-echo guard rationale, the fallback-slug coupling, the two-decodes split in prefs, and every "this looks wrong and is deliberate" note. Trimming those is out of scope.
7. Do not re-run a general density sweep, do not delete rationale or a stated rejected alternative (both are permitted), and do not introduce workflow vocabulary (task ids, phase numbers, spec-section citations) into any rewritten comment.
8. Comments only. No non-comment line may change in this task.

**Acceptance Criteria**:
- No production comment in the named files names a test, a guard test, or a fixture set, or describes what a test proves.
- No production comment in the named files counts implementations, call sites, emission sites, readers or reachable packages.
- Each rewritten comment still states the invariant the original was protecting.
- `LoadPath`'s comment claims sharing only where the code is actually shared.
- The listed trap warnings survive verbatim.
- `git diff` shows comment-line changes only; `go build ./... && go test ./...` pass and `golangci-lint run` is clean.

**Tests**:
- The full unit lane passes — the change is comment-only, so any failure means code was touched.
- `golangci-lint run` clean (doc comments must still begin with the identifier name where the linters require it).
- A `git diff --stat` / diff review confirming no executable line changed.
- A grep over the named files for `Test[A-Z]`, `_test`, "guard", "the only", "nowhere else", "which otherwise has none" and "the ONE" returns nothing outside of deliberate identifier references.

## Task 5: Re-home the Go source-guard AST helpers out of the binary-build test package
severity: medium
sources: architecture

**Problem**: `internal/portalbintest` is documented — and named — as the helper that compiles the portal CLI and stages it on `$PATH` for integration tests (`ProjectRoot`, `BuildPortalBinary`, `StagePortalBinary`). It now also holds three generic Go-source-scanning helpers with no relationship to building or staging a binary: `GoSourceFiles`, `PackageGoFiles` and `ForEachFuncCall`. Roughly twenty unit-lane source guards across `internal/theme`, `internal/tui`, `internal/prefs`, `internal/capture`, `internal/log` and `cmd` now import the binary-build package to reach them, and the package doc had to grow an "Exported surface" list covering two unrelated families — the signal that the boundary moved rather than that the package grew. CLAUDE.md's package inventory still describes `portalbintest` as "general-purpose `go build` / PATH-stage helpers" and says nothing about source scanning, so the repo's own map of the package is now incomplete. Because this is the established home, every future guard lands here too. NOTE: this is not a further consolidation layer — the three helpers stay exactly as they are and keep their single-source role; only where they live changes.

**Solution**: Move the three source-scanning helpers and their tests into a small dedicated test-helper package, leaving `portalbintest` as the binary build/stage helper its name and doc describe.

**Outcome**: One package holds "build and stage the portal binary" and another holds "scan the repo's Go source", each with a name and doc that match its contents. A unit-lane guard imports neither the build machinery nor a package whose name suggests one.

**Do**:
1. Create `internal/sourceguard` (test-only, stdlib + `testing` only, no build tag) and move `GoSourceFiles`, `PackageGoFiles` and `ForEachFuncCall` into it, carrying each function's existing doc comment verbatim — those comments record the single decisions about what a repo-wide guard covers, what a package-local guard covers, and how a call-scanning guard walks a file.
2. Move `gosourcefiles_test.go`, `packagegofiles_test.go` and `foreachfunccall_test.go` with them; keep every existing assertion.
3. Write the new package doc as the source-scanning charter, and state that production code must not import it, in the same shape as the other test-only packages' docs.
4. Trim `internal/portalbintest`'s package doc back to the build/stage surface: `ProjectRoot`, `BuildPortalBinary`, `StagePortalBinary`. `ProjectRoot` stays where it is — it resolves the module root for the binary build and is consumed by both families.
5. Re-point every consumer's import and call site (`internal/theme`, `internal/tui`, `internal/prefs`, `internal/capture`, `internal/log`, `cmd`, `cmd/capturetool`). This is an import-path and qualifier change only; no guard's assertion, exclusion rule, parse mode or failure wording may change.
6. Update CLAUDE.md's `portalbintest` row so it again describes only the build/stage surface, and add the new package to the same inventory with its "production code must not import" statement, matching how the other test-only helper packages are listed.
7. Do not add, merge, widen or narrow any helper while moving it, and do not change which directories any guard walks.

**Acceptance Criteria**:
- `internal/portalbintest` exports only `ProjectRoot`, `BuildPortalBinary` and `StagePortalBinary`, and its doc describes only those.
- The three source-scanning helpers and their tests live in the new package with their docs intact.
- Every guard consumer compiles against the new import path with no behavioural change.
- Every guard still fails when its forbidden construct is reintroduced.
- CLAUDE.md's inventory describes both packages accurately.
- `go test ./...` passes, `go test -tags integration -p 1 ./...` passes, and no test changes lane.

**Tests**:
- The moved helper tests pass unchanged in their new home.
- Temporarily reintroduce a forbidden construct behind two different guards (one repo-wide, one package-local, one call-scanning) and confirm each still fails; revert each.
- Confirm every guard still runs in the unit lane with no `integration` tag.
- Confirm the integration-lane binary build/stage tests still pass.

## Task 6: Give the flash's tier ordering one home and its origin field the model
severity: low
sources: duplication, architecture

**Problem**: Two halves of one surface. (a) `activeNoticeBand` (internal/tui/notice_band.go:392-400) and `activeProjectNoticeBand` (:461-468) both open with the identical two-arm prologue — `themeFlashClaim()` then `flashClaim()`, each returning verbatim — and both carry a comment stating that the filter line sits between the two tiers. The claim predicates are already single-sourced; the ORDER between them is written twice, and that order is the load-bearing rule (a theme flash must outrank the filter line on both pages, or a failed-commit report is destroyed rather than deferred). Projects' arbiter is new in this feature and the ordering was copied into it. A third tier added later reaches whichever arbiter its author edits. (b) `flashText`, `flashGen` and `flashKind` are `Model` fields, but `flashOrigin` — a fourth attribute of the same single live flash, with the same lifetime — sits on `themeState` (internal/tui/theme_state.go:151). So the generic `setFlash` (model.go:2314) reaches into the theme subsystem's state to reset a tier it otherwise knows nothing about, and the general notice-band arbiter (notice_band.go:526) resolves precedence for NON-theme flashes by reading out of `m.themeState`. It also breaks `themeState`'s own documented charter (the loaded setting, the seams, the light/dark resolution, the palette). The `flashOrigin` TYPE is already correctly declared beside `flashKind` in sessions_flash.go — only the field is misplaced.

**Solution**: Extract the flash-tier prologue into one claim function both arbiters call, and move the `flashOrigin` field onto `Model` beside its three siblings.

**Outcome**: The flash tier order is written once and a new tier lands in one place; the live flash's four attributes are declared, set and reset together; and `themeState` holds only what its charter says it holds.

**Do**:
1. Add `func (m Model) flashSlotClaim() (noticeBandRole, string, bool)` beside `flashClaim` and `themeFlashClaim`, holding the theme-tier-then-ordinary-flash order and the comment explaining where the filter line sits between them.
2. Replace the two-arm prologue in `activeNoticeBand` and `activeProjectNoticeBand` with one call to it. Each arbiter keeps only its own page-specific arms below, in their current order.
3. Move the `flashOrigin` field from `themeState` (theme_state.go:151) to `Model`, beside `flashText`/`flashGen`/`flashKind`. Move its doc comment with it.
4. Re-point every reader and writer: `setFlash` / `setSuccessFlash` (model.go:2314) reset it, `setThemeFlash` (model.go:2330) stamps it, and the band predicate (notice_band.go:526) reads it — all as `m.flashOrigin`.
5. Leave the `flashOrigin` type and its constants where they are in sessions_flash.go; only the field moves.
6. Change no precedence and no rendering. The single-slot order (filter line → `Opening n/N…` → transient flash → multi-select banner → unsupported banner → no-tags signpost) and the theme tier's position within it are exactly as they are today, on both pages.
7. Keep `setThemeFlash` the only writer of the theme tier.

**Acceptance Criteria**:
- The theme-tier-then-ordinary-flash order appears once; neither arbiter restates it.
- `themeState` declares no flash field; `flashOrigin` sits with `flashText`/`flashGen`/`flashKind` on `Model`.
- `setFlash` no longer touches `m.themeState`.
- Band precedence on Sessions and on Projects is byte-identical to before on every rendered path.
- `go test ./internal/tui ./internal/capture` passes.

**Tests**:
- Existing notice-band precedence, theme-flash and projects-band tests pass unchanged.
- A test that an ordinary flash raised while a theme flash is live does not displace the theme tier, on BOTH pages, driven through the one claim function.
- A test that `setFlash` resets the origin to the default tier.
- The capture fixtures that render a flash, a theme signal and the command-pending banner render byte-identically before and after.

## Task 7: Declare the shared nav/page keymap entries once
severity: low
sources: duplication

**Problem**: `{Key: "↑↓", HelpKey: "↑/↓", Action: "navigate", HelpAction: "Move selection"}` and `{Key: "^↑/↓", Action: "page", HelpAction: "Next / prev page"}` are byte-identical in `sessionsKeymap` (internal/tui/keymap.go:106-107), `projectsKeymap` (:153-154) and — added by this feature — `themePanelKeymap` (:206-207). That is the third instance, in the file whose whole purpose is to be the single display source for the footer and the `?` help body. A copy edit to the nav glyph or its help label now needs three edits, and one descriptor diverging silently changes what one surface shows while the others stay correct. No comment states the restatement is deliberate.

**Solution**: Declare the pair once and have all three descriptors lead with it.

**Outcome**: The nav and page rows are written once, so all three surfaces cannot disagree about the arrows.

**Do**:
1. Add `func navKeymapEntries() []keymapEntry` in internal/tui/keymap.go returning the two non-core entries in their current order, with their current `Key`/`HelpKey`/`Action`/`HelpAction` values.
2. Have `sessionsKeymap`, `projectsKeymap` and `themePanelKeymap` each build their slice by leading with those entries, preserving each descriptor's existing order exactly — nav first, then the surface's own entries unchanged.
3. Leave `previewKeymap` alone; its page entry carries a different `HelpAction` and is not the same row.
4. Keep all three functions pure and static — no model access, no filtering — as their doc comments require; the `t`/`m` blocked-key filters stay at their call sites.
5. Change no rendered output: the footer's Core order and glyph forms, and the help body's row order, are identical before and after on all three surfaces.

**Acceptance Criteria**:
- The nav and page entries are written once; no descriptor restates either literal.
- All three descriptors still return their entries in their current order.
- `previewKeymap` is untouched.
- Footer and help-modal rendering is byte-identical on Sessions, Projects and the theme panel.
- `go test ./internal/tui ./internal/capture` passes, including the descriptor↔dispatch parity guard.

**Tests**:
- Existing keymap, footer, help-modal and descriptor↔dispatch parity tests pass unchanged.
- A test that all three descriptors' first two entries are equal to the shared pair.
- The footer and help-modal capture fixtures for all three surfaces render byte-identically.

## Task 8: Initialise the five nil slice accumulators explicitly
severity: low
sources: standards

**Problem**: `.claude/skills/golang-code-style` states "Slices and maps MUST be initialized explicitly, never nil", with `make([]T, 0, n)` where the capacity is known. The theme code applies this in most places — `lex.go:51`, `validate.go:81`, `validate.go:118`, `union.go:305`/`:306`, `enumerate.go:108`, `doctor_theme.go:192`/`:281` all initialise explicitly — but five accumulators still declare `var xs []T` and return nil on the empty path: `persistedRows` (internal/theme/union.go:357), `Enumerate` (internal/theme/enumerate.go:67), `InForceKeys` (internal/theme/setting.go:237), and the two `advisories` accumulators in cmd/doctor_theme.go (:252, :372). The consequence is contained — none is JSON-serialised or written to disk — so this is a consistency deviation rather than a defect, but the same package answers "nothing found" with `[]Entry{}` on one path and nil on another.

**Solution**: Initialise the five accumulators explicitly, preallocating where the bound is already known.

**Outcome**: Every accumulator in the feature answers "nothing found" the same way, and the style rule holds uniformly across the package.

**Do**:
1. `internal/theme/union.go:357` — `rows := make([]Row, 0, len(InForceKeys(keys)))` (or against whatever bound the loop iterates), replacing `var rows []Row`.
2. `internal/theme/enumerate.go:67` — `entries := make([]Entry, 0, len(dirEntries))`.
3. `internal/theme/setting.go:237` — `inForce := make([]InForceKey, 0, 2)`.
4. `cmd/doctor_theme.go:252` and `:372` — initialise each `advisories` slice against its loop's bound.
5. Change no behaviour and no caller. Verify no caller distinguishes nil from empty — check every `== nil` / `!= nil` test against these return values and every length or range use, and if any assertion pins nil, change the assertion to pin empty and say so in the task's report.
6. Do not sweep beyond these five sites.

**Acceptance Criteria**:
- The five accumulators are explicitly initialised, with capacity preallocated where the bound is known.
- No caller or test distinguishes a nil result from an empty one.
- Enumeration, union assembly, in-force key derivation and doctor's advisory output are behaviourally unchanged.
- `go build ./... && go test ./...` pass; `golangci-lint run` is clean.

**Tests**:
- Existing enumerate / union / setting / doctor-theme tests pass unchanged.
- A test that each of the five paths returns a non-nil empty slice when nothing is found.
- Doctor's rendered theme block is byte-identical for a run with no advisories.

## Task 9: Record the reference-frame retention carve-out in CLAUDE.md
severity: low
sources: standards

**Problem**: CLAUDE.md's visual-capture-harness paragraph states one rule without its exception: "The images and the tapes that produce them are scaffolding, not a durable asset… cleared out after sign-off rather than living in the repo." The implementation kept 30 committed PNGs under `testdata/vhs/reference/` and documented the exemption only in `testdata/vhs/README.md` ("A reference frame is the design the code was built against… Keep them, and keep pointing comments at them"), where its retention table marks `reference/*.png` as **Kept**. The exemption is sound — those frames are design exports rather than renders of the code — and roughly twenty production and test comments now point into that directory (`internal/tui/keymap.go:81`, `internal/tui/loading_view.go:17`, and a dozen sites in `internal/capture/fixtures.go`). But CLAUDE.md is what an implementing agent reads first, and following it alone at the next sign-off deletes the reference set and orphans every one of those pointers.

**Solution**: State the carve-out in CLAUDE.md so the two documents describe one rule.

**Outcome**: A contributor or agent reading only CLAUDE.md keeps the reference frames and knows why, and the sign-off sweep cannot orphan the comments that cite them.

**Do**:
1. Amend CLAUDE.md's capture-harness paragraph with one clause distinguishing `testdata/vhs/*.png` and `testdata/vhs/*.tape` (scaffolding, written as work proceeds, cleared at sign-off) from `testdata/vhs/reference/*.png` (committed design exports — the frames the code was built against — kept, and cited by production comments).
2. Keep the existing sentences about the harness and the Go fixture definitions in `internal/capture` being permanent; this is an addition, not a rewrite.
3. Do not restate `testdata/vhs/README.md`'s full retention table in CLAUDE.md — one clause plus the distinction is enough; the README stays the detailed home.
4. Change no code and no capture.

**Acceptance Criteria**:
- CLAUDE.md's capture-harness paragraph states the `reference/` carve-out and its reason.
- CLAUDE.md and `testdata/vhs/README.md` no longer disagree about what is cleared at sign-off.
- No file outside CLAUDE.md changes.

**Tests**:
- `go test ./...` passes (documentation-only change).
- A read-through confirming the two documents state the same rule, and that every directory named in the CLAUDE.md clause exists as described.
