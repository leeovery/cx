# Analysis Tasks: theming-system (Cycle 3)

## Task 1: Single-source the theme-panel fixture test scaffolding in internal/capture
severity: medium
sources: duplication

**Problem**: Three task-partitioned test files in `package capture_test` each authored their own copy of the same scaffolding. (a) Three independent panel-frame parsers — `panelRows` (theme_panel_fixture_render_test.go:63), `panelLinesOf` (theme_panel_message_fixtures_test.go:87), and `uniquePanelLine`/`panelLineWith`/`panelRowLine` (theme_panel_remaining_fixtures_test.go:160/127/142) — all implement the identical primitive: split the frame on newlines, `strings.Cut(line, panelBorder)` to take the panel side, `ansi.Strip`, tokenise with `strings.Fields`, fatal with the stripped frame when nothing matched. Worse than the boilerplate, the row-shape rule ("drop the leading `▌` field, then the first remaining field is the label") is stated twice independently — `panelRows` (render:76-80) and `panelRowLine` (remaining:152-156) — and that rule is a fact about theme_row.go's composition, so two copies can disagree the moment the cursor column changes, in tests whose entire job is to catch such a change. The three also disagree today on whether they strip ANSI before matching (`panelRows`/`uniquePanelLine` do; `panelLinesOf` keeps a raw copy alongside), so an assertion's meaning depends on which file it lives in. (b) Five near-identical registration/guard tests partitioned only by which task added the fixture: `TestPanelFixture_RegisteredInBothRegistries` (render:381) and `TestPanelFixture_MessageFramesRegistered` (message:606) are byte-identical bodies differing only in the name accessor they range over; `TestPanelFixture_UnderTheGuard` (render:399) and `TestPanelFixture_MessageFramesUnderTheGuard` (message:626) likewise; `TestPanelFixture_AllRegistered` (remaining:562) folds the same claims over the third subset. Six parallel name accessors exist (`panelFixtureNames`, `capturePanelFixtureNames`, `messagePanelFixtureNames`, `remainingPanelFixtureNames`, `allPanelFixtureNames`, `registeredPanelFixtureNames`), with the `"theme-panel-"` prefix literal restated between the last two.

**Solution**: Extract one panel-frame projection into a shared file in `package capture_test` that owns the row-shape rule and the strip policy, re-express the five existing parsers as thin projections over it, and collapse the five registration/guard tests into one test ranging over the registry-derived name set.

**Outcome**: One statement of how a panel frame is read and one statement of what makes a `theme-panel-*` fixture correctly registered. A change to the panel's cursor column breaks one helper rather than silently disagreeing with a second copy, and a new panel fixture is covered by the registration guard without adding a fourth partitioned test.

**Do**:
1. Create `internal/capture/panel_frame_test.go` in `package capture_test`. Move the `panelLine` type there and give it both a `raw` (un-stripped) and a `visible` (ANSI-stripped) form. Expose `panelLines(t, frame) []panelLine` as the single frame→lines projection: split on `"\n"`, `strings.Cut` on the panel border to take the panel side, `ansi.Strip` for `visible`, retain the un-stripped side as `raw`, and fatal with the stripped frame when no line matched.
2. Add one `fields()` accessor on `panelLine` that owns the drop-the-cursor-bar rule (drop the leading `▌` field; the first remaining field is the label). This must be the only statement of that rule in the package — cite theme_row.go's composition in its comment as the thing it mirrors.
3. Re-express `panelRows`, `panelLineIndex`, `panelLineWith`, `panelRowLine` and `uniquePanelLine` as thin projections over `panelLines`/`fields`. Keep each function's existing signature and return shape so no call site changes. Delete the three private line loops and the second copy of the cursor rule.
4. Make the strip policy uniform: every matcher matches on `visible`; `raw` stays available for the style assertions that need it. Where a test previously matched raw text, re-point it at `visible` and confirm the claim is unchanged.
5. Single-source the `"theme-panel-"` prefix as one constant, used by both `allPanelFixtureNames()` (remaining:65) and `registeredPanelFixtureNames()` (render:104).
6. Replace `TestPanelFixture_RegisteredInBothRegistries`, `TestPanelFixture_UnderTheGuard`, `TestPanelFixture_MessageFramesRegistered`, `TestPanelFixture_MessageFramesUnderTheGuard` and `TestPanelFixture_AllRegistered` with ONE test ranging over `registeredPanelFixtureNames()` that asserts each name resolves, is enumerated by the fixture-name accessor, and is covered by the swap-and-diff guard. Keep the per-file name slices only as inputs to `allPanelFixtureNames()`.
7. Leave `TestPanelFixture_RegistryHoldsTheSpecifiedPanelSet` (remaining:594) alone — it is the separate exact-set claim and stays separate.
8. Do not change what any assertion claims. This task moves where a shared primitive lives; it does not add, remove or weaken a claim.

**Acceptance Criteria**:
- Exactly one frame→lines parser exists in `package capture_test`; no other file splits a panel frame or re-derives the row shape.
- The drop-the-cursor-bar rule appears exactly once in the package.
- Strip behaviour is uniform across all matchers; `raw` remains reachable for style assertions.
- One registration/guard test covers every `theme-panel-*` fixture; the five partitioned tests are gone.
- The `"theme-panel-"` prefix literal appears once.
- Every claim made by the deleted tests is still made by the consolidated test.
- `go test ./internal/capture` passes.

**Tests**:
- All existing `internal/capture` panel tests pass with their claims unchanged.
- Temporarily drop a fixture from the registry and confirm the consolidated registration test fails naming it; revert.
- Temporarily drop a fixture from the swap-and-diff guard's coverage and confirm the consolidated test fails; revert.
- Temporarily change the panel's cursor column width and confirm exactly one helper needs updating (proves the rule is single-sourced); revert.

## Task 2: Single-source the prefs abort-on-undecodable case table
severity: medium
sources: duplication

**Problem**: Four newly-authored `package prefs` test files each restate the corrupt-`prefs.json` case table and the seed→save→assert-untouched loop for their own saver, and the tables have already diverged — which is exactly the failure mode. `store_write_path_test.go:132` carries four cases (truncated object, trailing comma, junk, unterminated-with-real-values) and pushes the zero-byte and top-level-array shapes into two separate tests (`:169`, `:263`); `migration_marker_test.go:440` and `translation_saver_test.go:372` each carry five (adding zero-byte and top-level array, dropping unterminated-with-real-values); `theme_savers_test.go:351` covers the contract with a single inline literal. Only two of the four assert the decoder's error is returned verbatim as `*json.SyntaxError`. Since `mutate` is the single write-path chokepoint every saver routes through, these are five statements of one contract that can no longer be read as one — a corruption shape added to any one table leaves the other savers unproven against it. All four files already share `seedPrefsFile`, `readRaw` and `assertUntouched`, so the shared home already exists.

**Solution**: One `undecodablePrefsCases()` table beside the existing shared helpers holding the union of the six shapes, with all four abort tests ranging over it and keeping only their saver-specific assertions in the loop body.

**Outcome**: The abort-on-undecodable contract is stated once. Adding a seventh corruption shape proves every saver against it with a single edit, and no saver can silently fall out of coverage.

**Do**:
1. Add `undecodablePrefsCases()` beside `seedPrefsFile`/`readRaw`/`assertUntouched` in `internal/prefs/store_write_path_test.go`, returning a slice of a named case type holding the union of the six shapes: truncated object, trailing comma, junk/non-JSON, unterminated-with-real-values, zero-byte/empty file, top-level array (type mismatch).
2. Give the case type a field recording which decoder error class the shape produces — the syntax shapes yield `*json.SyntaxError`, the top-level array yields a type-mismatch error — so the error-class assertion runs for every saver on every shape that carries one, rather than being dropped for everyone to accommodate the outliers.
3. Document the table as the single statement of the shapes every saver aborts on, so a new shape is added once.
4. Re-point `TestSave_AbortsOnMalformedJSON` (store_write_path_test.go:127) at the table, folding `TestSave_AbortsOnEmptyFile` (:169) and `TestSave_TopLevelTypeMismatchAborts` (:263) into it; delete those two once their claims are made in the loop body.
5. Re-point the migration-marker saver's abort test (migration_marker_test.go:440), `SaveTranslation`'s (translation_saver_test.go:372) and the theme savers' (theme_savers_test.go:351, replacing the single inline literal) at the same table.
6. Keep each saver's specific assertions inside the loop body: `persisted=false` for `SaveTranslation`, no-error-variance for the marker, and `assertUntouched` for every saver/case pairing.
7. Do not weaken any existing claim — the union is additive; every shape any table covered today is covered for every saver after the change.

**Acceptance Criteria**:
- One case table exists; the three restated tables and the inline literal are gone.
- Every saver routing through `mutate` is proven against all six shapes.
- The decoder error-class assertion runs for every saver on every shape that carries one.
- `assertUntouched` runs for every case/saver pairing — the file is byte-identical after each aborted save.
- `go test ./internal/prefs` passes.

**Tests**:
- All existing prefs abort tests pass through the shared table with unchanged expectations.
- Temporarily add a seventh shape to the table and confirm all four savers exercise it with no other edit; revert.
- A saver-specific assertion (e.g. `persisted=false` for `SaveTranslation`) still fails when deliberately broken; revert.

## Task 3: Single-source the repo-wide Go source-guard scaffolding
severity: medium
sources: duplication

**Problem**: Two related duplications in the newly-authored repo-wide source guards. (a) Module-root resolution: `internal/prefs/appearance_api_guard_test.go:49` calls the existing shared `portalbintest.ProjectRoot()` (internal/portalbintest/build.go:38), but `internal/tui/theme_source_guard_test.go:259` (`repoRoot`) and `internal/theme/broken_builtin_test.go:441` (`themeRepoRoot`) each hand-rolled the same ~18-line upward `go.mod` walk, down to the same failure wording. Three executors met the same need; one found the shared helper, two did not. `portalbintest` is not integration-tagged and is already consumed from the unit lane, so both copies are removable with no lane change. (b) Source enumeration: four guards each carry their own `filepath.WalkDir` closure collecting `.go` files, and the walks no longer agree on what they walk — three skip every dot-directory, while `internal/prefs/appearance_api_guard_test.go:58` skips only `.git`, `vendor` and `node_modules` and therefore descends into `.workflows`, `.claude` and `.github`. The sharpest case is inside `tui_test`: `forEachGoFile` (theme_source_guard_test.go:279) and `allGoFiles` (restore_source_guard_test.go:295) sit in the SAME package with line-for-line identical walk bodies, differing only in the per-file action. Since these guards are the enforcement layer for "this identifier/import must not exist anywhere", a divergence in which directories are walked is a silent hole in whichever guard is narrower.

**Solution**: Delete both hand-rolled module-root walks in favour of the shared helper, and promote one `.go` enumeration with one stated exclusion rule beside it, pointing all four guards at it.

**Outcome**: "What the repo-wide guards cover" becomes a single decision recorded in one place. No guard can be narrower than its siblings by accident, and `tui_test` has one walk feeding both of its guards.

**Do**:
1. Delete `repoRoot` (internal/tui/theme_source_guard_test.go:259) and `themeRepoRoot` (internal/theme/broken_builtin_test.go:441). Call `portalbintest.ProjectRoot()` at both sites, wrapping its error in the caller's `t.Fatalf` exactly as internal/prefs/appearance_api_guard_test.go:49 already does.
2. Add `GoSourceFiles(root string) ([]string, error)` beside `ProjectRoot` in `internal/portalbintest` — one `filepath.WalkDir` returning every `.go` file under root, with ONE stated exclusion rule: skip every directory whose name begins with `"."`, plus `vendor` and `node_modules`. Document it as the single decision about what the repo-wide guards cover.
3. Re-point all four guards at it: internal/tui/restore_source_guard_test.go:295, internal/tui/theme_source_guard_test.go:279, internal/theme/broken_builtin_test.go:392, internal/prefs/appearance_api_guard_test.go:58.
4. Within `tui_test`, re-express `forEachGoFile` as a loop over the shared enumeration so one walk feeds both guards. Keep its imports-only PARSE mode — the second parse mode is justified in-source at restore_source_guard_test.go:293 and is real; only the enumeration is duplicated.
5. Confirm the prefs guard's coverage change (dot-directories now excluded) does not drop a directory it was meaningfully guarding — if any `.go` file lives under an excluded directory, state that in the helper's comment rather than adding a per-guard exception.
6. Do not weaken any guard's claim. The forbidden-identifier and forbidden-import assertions are unchanged; only root resolution and enumeration move.

**Acceptance Criteria**:
- No hand-rolled `go.mod` walk remains in any test; both former copies call `portalbintest.ProjectRoot()`.
- One `.go` enumeration helper with one exclusion rule; all four guards consume it.
- The prefs guard's coverage matches the other three.
- `tui_test` performs one walk, not two.
- Every guard still fails when its forbidden symbol or import is reintroduced.
- `go test ./...` (unit lane) passes and no test moves lane.

**Tests**:
- Temporarily reintroduce each guard's forbidden construct (the deleted `canvasHexFor` helper in internal/tui, the `appearance` API in internal/prefs, the broken-builtin case in internal/theme) and confirm the guard still fails; revert each.
- Add a scratch `.go` file under an excluded directory, confirm no guard trips on it, and remove it.
- Confirm the guards run in the unit lane (no `integration` tag required) after the change.

## Task 4: Take the typed RawKeys value in ResolveSetting instead of three positional strings
severity: medium
sources: architecture

**Problem**: `theme.ResolveSetting(theme, light, dark string)` (internal/theme/setting.go:114) is the single collapse point for the whole theme setting, and every caller already holds a struct with exactly those three fields and destructures it into three same-typed positional arguments: `cmd/open.go:804` destructures `prefs.ThemeKeys`, `internal/tui/theme_panel.go:660` destructures `m.themeState.keys` (`theme.RawKeys`), and `InForceKeys` (setting.go:184) destructures its own `RawKeys` parameter. Three adjacent `string` parameters where light and dark are interchangeable at the type level is exactly the inversion hazard the package elsewhere went to real lengths to close — `AdaptivePair`/`MemberPalette`/`Member.Palette` exist precisely so a palette cannot reach the pair untagged, with a comment warning that "reading it the wrong way round at one call site inverts light and dark for the whole product — with every palette still loading, every token still resolving and nothing failing anywhere". The same failure is reachable one layer down through this signature and nothing in the type system objects. The signature also forces the control-strip to be re-run on already-stripped values at three of four call sites, and forces `RawKeys` to be rebuilt field-by-field at the prefs boundary (cmd/doctor_theme.go:202, and structurally in cmd/open.go).

**Solution**: Take the value, not the parts — `ResolveSetting(keys RawKeys)` — with control-stripping moved into a single `RawKeys` constructor/normaliser that the two prefs-boundary sites in `cmd` call once.

**Outcome**: A transposed light/dark is no longer expressible at any `ResolveSetting` call site, stripping is a property of constructing the value rather than a step each consumer re-runs, and the prefs→theme key mapping is written once.

**Do**:
1. Change the signature to `func ResolveSetting(keys RawKeys) (Setting, RawKeys)` in internal/theme/setting.go, reading `keys.Theme`/`keys.Light`/`keys.Dark` internally. Keep the returned `RawKeys` (the stripped form) so no caller loses the post-strip value.
2. Move the control-strip into one `RawKeys` constructor/normaliser in the same package — `NewRawKeys(theme, light, dark string) RawKeys` or `func (k RawKeys) Stripped() RawKeys` — and have `ResolveSetting` call it. It stays idempotent, so a pre-stripped value passes through unchanged.
3. Update the `RawKeys` doc comment (setting.go:5-23) so the stripping rule is stated where it now happens, and trim the corresponding paragraph from `ResolveSetting`'s comment so the rule is not stated twice.
4. Update `InForceKeys` (setting.go:183) to pass its `keys` straight through instead of destructuring.
5. Update `themeResolution` (cmd/open.go:804) to build the `RawKeys` once at the prefs boundary from `prefs.ThemeKeys` via the new constructor, then pass the value.
6. Update cmd/doctor_theme.go:202 to use the same constructor instead of the field-by-field `theme.RawKeys{Theme: keys.Theme, Light: keys.Light, Dark: keys.Dark}` literal.
7. Update internal/tui/theme_panel.go:660 to pass `m.themeState.keys` whole.
8. Do not add a dependency between `internal/prefs` and `internal/theme` in either direction — the `prefs.ThemeKeys` → `theme.RawKeys` mapping stays in `cmd`, which already owns the prefs boundary. `internal/theme` must stay a leaf.

**Acceptance Criteria**:
- No call site passes three positional strings; a transposed light/dark is not expressible at any `ResolveSetting` call.
- Control-stripping is performed in exactly one place.
- The field-by-field `theme.RawKeys{...}` rebuilds at the prefs boundary are gone; both `cmd` sites go through the constructor.
- Resolution behaviour is unchanged: the `theme`-wins tiebreak, per-slot shipped defaults, and empty-after-strip counting as unset all hold.
- `internal/theme` gains no imports and stays a leaf.
- `go build ./... && go test ./...` pass; `golangci-lint run` is clean.

**Tests**:
- Existing `internal/theme` setting tests pass, adapted to the new signature.
- A constructor-level test that a value of only control characters strips to empty and therefore counts as unset (not as an illegal slug).
- A test that the returned `RawKeys` are the stripped form.
- A test that constructing from already-stripped values is a no-op (idempotence).
- Existing `cmd/open` and `cmd/doctor` theme tests unchanged in outcome, proving the prefs-boundary round trip is preserved.

## Task 5: Replace the hand-rolled cmd theme-test helpers with the shared ones
severity: low
sources: duplication

**Problem**: Two duplications in the `cmd` theme test files. (a) `snapshotTree` (cmd/theme_test.go:488) walks a directory returning path → "mode + sha256" so three tests can assert a command created, deleted or rewrote nothing (cmd/theme_test.go:601, cmd/doctor_theme_test.go:540, cmd/doctor_fix_theme_test.go:280). `internal/portaltest` already exports exactly that capability — `SnapshotStateDir` (Lstat-based, sha256-hashed, symlink-aware) plus `DiffFingerprints`/`FormatDelta`, whose documented purpose is comparing two snapshots at caller-chosen points — and the `cmd` test package already imports `portaltest` elsewhere. The hand-rolled version is weaker on exactly the properties the assertions care about: it uses `entry.Info()` rather than Lstat, so a file-to-symlink swap is invisible to it, and a `maps.Equal` failure prints two whole maps where `FormatDelta` names the changed path. (b) The nine-line "the sink captures a theme event when one is emitted" vacuity subtest is byte-identical in cmd/theme_test.go:474 and cmd/doctor_theme_test.go:498, and a third silent-path test (`TestDoctorFix_EmitsNoThemeRecords`, cmd/doctor_fix_theme_test.go:427) makes the same silence claim with no vacuity guard at all.

**Solution**: Use `portaltest`'s snapshot/diff for the no-write assertions, and extract the vacuity guard into one helper called by all three silent-path tests.

**Outcome**: The read-only claims are enforced by the sharper shared implementation with a failure message that names the offending path, and every silent-path test proves its own harness is live — including the one that currently does not.

**Do**:
1. Replace `snapshotTree` + `maps.Equal` with `portaltest.SnapshotStateDir` + `DiffFingerprints` at all three call sites (cmd/theme_test.go:601, cmd/doctor_theme_test.go:540, cmd/doctor_fix_theme_test.go:280), reporting failures via `FormatDelta`.
2. Delete `snapshotTree` (cmd/theme_test.go:486-…) and any imports it alone required (`io/fs`, `maps`, the hashing imports).
3. Extract the vacuity guard into one helper in a single `cmd` test file. Prefer the fuller shape: `assertNoThemeRecords(t, run func())` — install a `logtest.Sink`, run the command, assert zero `theme` records, then prove the sink live by emitting one `theme.NewEventLogger(log.For("theme")).Rejected(...)` and requiring exactly one captured record.
4. Call it from all three silent-path tests, including `TestDoctorFix_EmitsNoThemeRecords` (cmd/doctor_fix_theme_test.go:427) which currently has no guard.
5. Keep every existing claim and failure wording where the wording carries information; the change is where the shared primitive lives.

**Acceptance Criteria**:
- `snapshotTree` is gone; the three no-write assertions run on `portaltest.SnapshotStateDir` and report via `FormatDelta`.
- A file-to-symlink swap under an asserted root now fails the no-write assertion.
- One vacuity guard exists and is called by all three silent-path tests.
- `go test ./cmd` passes.

**Tests**:
- Temporarily make one of the three commands write a file under the asserted root; confirm the assertion fails and names that path; revert.
- Temporarily replace a file under an asserted root with a symlink of identical content; confirm the assertion now catches it; revert.
- Temporarily make a silent path emit a `theme` record; confirm the corresponding test fails; revert.

## Task 6: Derive the paginated panel fixture from its base union and enumeration
severity: low
sources: duplication

**Problem**: `themePanelPaginatedUnion` (internal/capture/fixtures.go:1255) re-declares the same four base rows that `themePanelUnion` (fixtures.go:745) declares — the one drop-in plus the three built-ins, in the same display order, with the same `Source` tags — before appending its synthetic drop-ins. The same holds for the retained parse: `themePanelPaginatedEntries` re-states the shared drop-in entry that `themePanelEnumeration` already builds. Every other derived panel fixture in this file composes from its base (`themePanelConfirmFixture` from the constant frame, `themePanelNarrowFixture` and `themePanelPaginatedFixture` from the adaptive-pair frame), and the file's own comments lean on those pairs being controlled. A built-in added to the embedded set changes one of the two lists and not the other, silently.

**Solution**: Build the paginated union by appending the synthetic rows to `themePanelUnion().Rows` (re-deriving `Count`), and the paginated entries by appending to `themePanelEnumeration()`'s entries, so the base set is declared once.

**Outcome**: The base row set and the shared drop-in entry are each declared exactly once, and adding a built-in flows into the paginated fixture with no second edit — closing a silent divergence in the permanent harness.

**Do**:
1. Rewrite `themePanelPaginatedUnion` (fixtures.go:1255) to start from `themePanelUnion().Rows`, append the synthetic drop-in rows, and re-derive `Count` from the resulting length. Copy the base slice before appending so the base fixture cannot be aliased or mutated by the derived one.
2. Rewrite `themePanelPaginatedEntries` to start from `themePanelEnumeration()`'s entries and append the synthetic entries, so the shared drop-in entry is declared once. Copy before appending for the same reason.
3. Keep display order identical — base rows first, synthetics after. `themePanelSyntheticSlug`'s existing comment already states the synthetics are named to sort after every built-in, so the two badged rows stay on page 1; preserve that property.
4. Update both doc comments to say the set is derived from the base rather than restating its membership.
5. Change nothing about the rendered frame — this is a derivation change, not a content change.

**Acceptance Criteria**:
- The base row set and the shared drop-in entry are each declared exactly once in fixtures.go.
- The rendered `theme-panel-paginated` frame is byte-identical to before the change.
- `Count == len(Rows)` after derivation.
- Mutating the derived rows cannot affect `themePanelUnion()` (no slice aliasing).
- Appending a built-in to `themePanelUnion` flows into the paginated fixture with no second edit.
- `go test ./internal/capture` passes.

**Tests**:
- `TestPanelFixture_PaginatedDrawsDots` and the registry/guard tests pass unchanged.
- A test asserting the paginated union's leading rows equal `themePanelUnion().Rows` (pins the derivation).
- A test asserting `Count == len(Rows)`.
- Temporarily add a built-in row to `themePanelUnion` and confirm it appears in the paginated fixture; revert.

## Task 7: Carry the missing-token list as structured data on Rejection instead of parsing rendered copy
severity: low
sources: architecture

**Problem**: `tokenAttr` (internal/theme/events.go:287-296) derives the `theme` component's structured `token` attr with `strings.TrimPrefix(r.Detail, detailMissingTokensLeadIn)` — parsing a pinned user-facing sentence back apart to recover a list the producer already had in hand a few lines earlier: `requireEveryToken` (internal/theme/validate.go:115-130) builds `missing []string`, joins it, prefixes it and discards the slice. The comment beside `tokenAttr` forbids doing exactly this for `bad colour` ("that would mean parsing rendered user-facing copy back apart, which is exactly what `Rejection.Detail`'s contract says nothing downstream does"), so the type's own rule is broken by one of its two consumers. `Rejection` (internal/theme/reason.go:59-65) already carries a structured narrowing alongside the rendered detail for every other reason that needs one — `Line` for `bad syntax`, `BadNameCause` for `bad name`, `Err` for `unreadable` — and the token list is the one case left to be re-derived from copy, held together only by a shared lead-in constant. A copy edit to `missing ` silently changes a log attr.

**Solution**: Give `Rejection` the structured datum its siblings already have, populate it where the list is built, and render both the doctor detail and the `token` attr from it.

**Outcome**: No structured value in the theming system is recovered by parsing user-facing copy. The lead-in constant serves rendering only, and a copy edit cannot reach a log attr.

**Do**:
1. Add `Tokens []string` to `Rejection` (internal/theme/reason.go:59-65), documented alongside `Line`/`BadNameCause`/`Err` as the structured narrowing for the token-bearing reasons — the SOURCE the attr renders from, with `Detail` being its rendered form.
2. Populate it in `requireEveryToken` (validate.go:115-130) from the `missing` slice it already builds, leaving the rendered `Detail` byte-identical.
3. Populate it for `bad colour` in `applyPairs` with the offending `key = value` pairs, so both token-bearing reasons render their attr from structure rather than one from structure and one from `Detail`.
4. Rewrite `tokenAttr` (events.go:287-296) to render from `r.Tokens` — comma-joined for `missing tokens`, and the pairs joined identically for `bad colour` — with no `TrimPrefix` and no reference to `detailMissingTokensLeadIn`. The emitted attr value must be byte-identical to today for both reasons.
5. Reduce `detailMissingTokensLeadIn` (validate.go:23-27) to a rendering-only constant, and update the comment at validate.go:15-27 which currently justifies the constant by the read-back.
6. Update `tokenAttr`'s comment: the prohibition on parsing rendered copy stays, but it no longer needs to explain an exception for itself.
7. Confirm `Tokens` is empty for every reason that is not `missing tokens` or `bad colour`, and confirm no other consumer parses `Detail` anywhere in the repo.

**Acceptance Criteria**:
- No code derives a log attr or any structured value by parsing `Rejection.Detail`.
- `Rejection.Tokens` is populated for `missing tokens` and `bad colour` and empty for every other reason.
- The emitted `token` attr value is byte-identical to today for both reasons.
- The rendered `Detail` for both reasons is byte-identical to today.
- Editing the `missing ` lead-in changes only rendered copy, never the attr.
- `go test ./internal/theme ./cmd` passes.

**Tests**:
- The `token` attr for a missing-tokens rejection equals `strings.Join(rejection.Tokens, ", ")` and carries no `missing ` prefix.
- The `token` attr for a bad-colour rejection is unchanged (the `key = value` pairs).
- A test that changing the lead-in constant does not change the attr.
- A test that `Tokens` is empty for `bad name`, `reserved name`, `unreadable` and `bad syntax`.
- Doctor's rendered detail for both reasons is unchanged (existing cmd doctor theme tests).

## Task 8: Move the theme advisory dedup identity onto a theme-local record
severity: low
sources: architecture

**Problem**: `advisory` (cmd/doctor.go:95-99) is doctor's second, general report-line class, but two of its three fields (`slug`, `fromPrefs`) exist solely to serve the theme producers' one-slug-one-line union, and the renderer reads neither. The consequence is stated in the code itself — "A producer setting `line` alone would silently defeat that dedup" — i.e. the dedup's correctness rests on every future producer of an advisory remembering to populate identity fields that mean nothing to it. Theme-only policy (`assembleThemeAdvisories` cmd/doctor_theme.go:141-152, `persistedSlugs` :165-173) also operates over the shared type rather than over its own records, so the second member of the advisory class inherits a half-generic contract.

**Solution**: Keep the union's identity on a theme-local record, run the assembly over that, and convert to `[]advisory` — line-only — at the boundary where the block is handed to the renderer.

**Outcome**: `advisory` carries exactly what doctor renders, and the one-slug-one-line dedup cannot be silently defeated by an unrelated future producer.

**Do**:
1. Declare `themeAdvisory struct { line string; slug string; fromPrefs bool }` in cmd/doctor_theme.go, moving the union-identity doc paragraph currently on `advisory`'s `slug`/`fromPrefs` fields onto it.
2. Change the theme producers to return `themeAdvisory`: `persistedThemeAdvisory` (cmd/doctor_theme.go:281-291), the scan producer (:367-396), and the themes-dir-unreadable line (:331 — line-only, empty slug).
3. Run `assembleThemeAdvisories` (:141) and `persistedSlugs` (:165) over `[]themeAdvisory`.
4. Convert to `[]advisory` — line only — at the single boundary where the assembled block is handed to the renderer.
5. Reduce `advisory` (cmd/doctor.go:95-99) to the line it renders, and rewrite its doc comment: keep the paragraph about the line carrying its own leading glyph (that rule still holds), delete the slug/fromPrefs/dedup paragraph now living on `themeAdvisory`.
6. Confirm no non-theme advisory producer set `slug` or `fromPrefs`; if one did, that is a behaviour change and must be surfaced rather than silently dropped.

**Acceptance Criteria**:
- `advisory` carries only what the renderer reads.
- The one-slug-one-line dedup runs over `themeAdvisory` and is unreachable from a non-theme producer.
- Doctor's rendered output, line order and exit code are byte-for-byte unchanged for every existing case, including an unresolvable persisted slug outranking the same slug's file-validity line.
- `go test ./cmd` passes.

**Tests**:
- Existing doctor theme advisory tests pass with unchanged expectations (same lines, same order, same dedup).
- A test that a persisted-slug advisory and a scan advisory for the same slug still collapse to one line, with the persisted one winning.
- A test that a line-only advisory from a non-theme producer renders correctly and does not participate in theme dedup.
- A test that the themes-dir-unreadable line (no slug) survives assembly.

## Task 9: Resolve themeState.nomination's unread post-commit refresh
severity: low
sources: architecture

**Problem**: `applyCommittedSetting` (internal/tui/theme_panel_commit.go:342-349) maintains `themeState.nomination` post-commit, and the field carries a stated contract (internal/tui/theme_state.go:20-28): "it describes the setting CURRENTLY IN FORCE, not only the one injected at construction — every commit that lands re-resolves it, so a reader past the gate may trust it". But the field has exactly three readers, all construction- or gate-transition-only: `newNominationGate` (model.go:1352, reachable from `New`), `hasNomination` (model.go:1387) and `Nomination.Select` (model.go:1407), the latter two inside `syncResolvedMode`, which is reachable only from `New`, `armAppearanceDetection` and the two gate arms guarded by the once-only `resolve()`. Nothing can read the field after the gate has resolved, so the refresh is write-only state and the invariant is a promise maintained for a reader that does not exist. The cost today is one assignment; the cost tomorrow is a future reader trusting a contract nothing exercises — and the deliberate `canvasMode`/`gate.appearance` divergence documented on the same struct means the refreshed nomination and the live answer are not obviously composable.

**Solution**: Make the field honest — drop the assignment and the "currently in force" clause, leaving the nomination honestly a construction-time value, unless a real post-gate reader is found.

**Outcome**: No write-only state with an asserted invariant remains on `themeState`. A future contributor reads what the field actually is, rather than a contract nothing exercises.

**Do**:
1. Confirm the reader set first: grep `themeState.nomination` across `internal/tui` non-test sources. The expected result is two writes (model.go:874 in `WithThemeNomination`, theme_panel_commit.go:348) and three reads (model.go:1352, :1387, :1407), all construction- or gate-transition-only.
2. If that holds, take the drop path: delete `m.themeState.nomination = resolution.Nomination` at theme_panel_commit.go:348. Keep the badge refresh (`m.themePanel.badges = theme.Badges(resolution.Slots)`) and the error-path behaviour (on a non-nil error nothing moves) exactly as they are.
3. Rewrite the field's doc comment in theme_state.go:20-28 to state honestly that it is the construction-time nomination, read by the gate and by `syncResolvedMode` only, and NOT maintained past the gate. State why: the palette in force past the gate is `themeState.active`, and `canvasMode`/`startupCanvasHex` deliberately diverge from it.
4. Update the comment at theme_panel_commit.go:327 which currently says the commit "keeps its contract through one site".
5. If step 1 turns up a genuine reader past the gate, take the other path instead: keep the assignment, keep the contract, and add the missing test that exercises the post-commit read — do not leave the contract untested either way.
6. Do not touch `startupCanvasHex`, `canvasMode` or `active` — their deliberate divergence is documented and load-bearing.

**Acceptance Criteria**:
- The field's doc comment and its actual maintenance agree.
- No write-only state remains on `themeState`.
- Commit behaviour is otherwise unchanged: badges refresh, the palette on screen does not swap, and nothing moves on a resolve error.
- The exit-path canvas restore is unaffected.
- `go test ./internal/tui ./internal/capture` passes.

**Tests**:
- Existing commit tests pass unchanged (badges refresh; no palette swap on commit; nothing moves on a resolve error).
- A test that `RestoreTerminalBackground` still uses the retained `startupCanvasHex` after a mid-session commit and is unaffected by the change — this is the invariant the nomination refresh could be mistaken for supporting.
- A test that a constant→adaptive conversion mid-session still leaves `canvasMode` where the documented divergence puts it.

## Task 10: Rename the panel's theme seam to cover resolution as well as enumeration
severity: low
sources: architecture

**Problem**: `ThemeEnumerator` (internal/tui/theme_seams.go:29-34) / `DirEnumerator` (internal/theme/enumerator.go:20-77) expose four methods, of which only `Open` and `Reassemble` are enumeration/assembly; `Resolve` and `ResolveSlot` are setting resolution — per-slot fallback, the broken-builtin fatal, the commit-time `theme: loaded` cadence — and are what the panel's open, close, commit-recompute and slot-conversion paths actually turn on. The name tells a reader the seam lists themes, which understates what a fake must answer for; this is visible in `internal/capture`'s fake, whose largest doc block is about why the *resolution* returns must report the injected palette. The seam is also over the project's stated 1-3 method DI convention, but splitting it would be worse — the four methods share one retained enumeration and one dedup scope.

**Solution**: Rename rather than split, so the seam's name covers resolution as well as enumeration and a fixture author reads the contract off the name.

**Outcome**: A fake author knows from the name that the seam answers resolution questions, not just listing ones. No method signature, method set or behaviour changes.

**Do**:
1. Rename the tui-side interface `ThemeEnumerator` → `ThemeSource` (internal/tui/theme_seams.go) and the theme-side concrete type `DirEnumerator` → `DirThemeSource` (internal/theme/enumerator.go).
2. Update both doc comments so the first line names BOTH responsibilities — assembly of the finished union AND resolution of the setting against the retained enumeration — keeping the existing per-method paragraphs intact.
3. Rename every reference: the `tui.Deps` field / constructor option and the `themeState.enumerator` field, `cmd/open.go:694` (`themeEnumerator`) and `:847` (`ThemeEnumerator:`), the production adapter `newThemeEnumerator` (cmd/theme_enumerator.go:25) and its file name, and every test reference.
4. Update the string-literal guard entry at cmd/open_theme_nomination_test.go:200 (`"newThemeEnumerator": true`) and the related comment at :58 — a rename that misses a guard's string literal silently disables the guard.
5. Rename the capture fake correspondingly (e.g. `newFakeThemeEnumerator` → `newFakeThemeSource`) and lift its resolution-returns doc block to the top of the fake, since the name now sets that expectation.
6. Rename the test names that carry the old identifier (cmd/theme_enumerator_test.go's `TestThemeEnumerator_*`) so grep stays useful.
7. Do not change any method signature, the method set, or any behaviour — this is a naming change only.
8. Update CLAUDE.md only if it names the seam; do not otherwise edit it (a separate task owns the CLAUDE.md inventory).

**Acceptance Criteria**:
- The identifiers `ThemeEnumerator` and `DirEnumerator` appear nowhere in the repo, including comments, test names and string literals.
- The string-literal guard at cmd/open_theme_nomination_test.go still guards the renamed constructor and still fails when its invariant is broken.
- Method set, signatures and behaviour are unchanged.
- `go build ./... && go test ./...` pass; `golangci-lint run` is clean.

**Tests**:
- All existing tui / capture / cmd theme tests pass with the renamed symbols and no behavioural change.
- Temporarily break the constructor-binding invariant the string-literal guard protects and confirm it still fails after the rename; revert.
- `grep -rn "ThemeEnumerator\|DirEnumerator"` over the repo returns nothing.

## Task 11: Reconcile the panel width ladder with the specified column band
severity: low
sources: standards

**Problem**: §9.1 and §9.8 pin the slide-over at "a fixed preferred width of ~24–30 columns", and §14A treats that range as the layout constraint every pinned panel string is written against ("it has to fit 24–30 columns"). The implementation uses `themePanelMinWidth = 27` / `themePanelPreferredWidth = 34` (internal/tui/theme_panel.go:46-47), and `themePanelWidthFor` computes `min(max(contentW/2, 27), 34)` (:210-212) — a proportional half-of-content rule clamped at both ends, rather than the preferred-width-then-staged-shrink shape §9.8 describes and justifies ("A fixed width is predictable to lay out against"). Three separate comments now restate the budget as "~27–34 columns" (theme_panel.go:22-24, theme_row.go:130, theme_panel_message.go:225), so the divergence is recorded only as a contradiction of the spec it is derived from. Consequences are small but real: the panel covers ~4 more columns of the previewed page at wide terminals (the §9.1 trade-off explicitly weighed against preview value), it refuses entry at content widths 24–26 that the spec's floor would have degraded into, and the panel's width changes on every resize step between ~54 and ~68 terminal columns. §9.8 delegates "exact thresholds … at implementation", but the delegation was for the degradation steps, not for the preferred-width band the copy decisions rest on.

**Solution**: Bring the ladder inside the decided band with a staged shrink; if the visual gate rejects the narrower panel, record the widened band as a deliberate spec amendment instead. Either way, code and spec must stop contradicting each other.

**Outcome**: The panel's width ladder and the specification state the same band, the ladder is staged rather than proportional so the panel stops resizing on every terminal step, and no code comment contradicts the spec it derives from.

**Do**:
1. Set `themePanelPreferredWidth = 30` and `themePanelMinWidth = 24` (internal/tui/theme_panel.go:46-47).
2. Replace the proportional rule in `themePanelWidthFor` (:210-212) with the preferred-then-staged-shrink shape §9.8 describes: render at the preferred width when the content region affords it, step down to the minimum when it does not, and refuse below the minimum. Keep the existing return contract — `w` clamped on the refusing path, callers taking `w` and ignoring `ok` because `themePanelFloor` has already refused.
3. Update the three comments restating "~27–34 columns" (theme_panel.go:22-24, theme_row.go:130, theme_panel_message.go:225) to the decided band.
4. Re-check the row composition priority at the new minimum: the 2-cell cursor column, the always-present `⚠`, the badges and the label truncation order must still resolve deterministically.
5. Re-check the message slot's truncation and the pinned confirm / commit-failed copy at the new minimum — §9.1 warns these may wrap there, and §14A makes the wording a layout constraint.
6. Update the width-dependent assertions: `TestPanelFixture_NarrowIsBetweenMinAndPreferred` (internal/capture/theme_panel_remaining_fixtures_test.go:378) and any `internal/tui` width tests.
7. Re-render and visually check the affected fixtures — `theme-panel-narrow`, `theme-panel-min-height-message`, the confirm frame, `theme-panel-paginated`, and the two badge frames. Live view: `go run ./cmd/capturetool --fixture <name>`. Verify a fresh PNG write (hash changed) before trusting any capture — the harness fails silently on write.
8. If the visual gate rejects the narrower panel, revert the code change and instead amend §9.1/§9.8/§14A to record 27–34 as the decided band, leaving the code comments consistent with the amended spec. Do not leave code comments contradicting the spec in either direction.

**Acceptance Criteria**:
- The width ladder and the specification state the same band; no comment contradicts §9.1/§9.8/§14A.
- The ladder is staged (preferred, then minimum) rather than proportional — the panel's width no longer changes on every resize step across the mid range.
- Every pinned panel string still fits, or truncates by its stated rule, at the minimum width.
- The panel refuses only below the minimum, and the refusing path still returns a clamped width.
- The visual gate is passed on the re-rendered narrow and minimum-height frames.
- `go test ./internal/tui ./internal/capture` passes.

**Tests**:
- A width-for-content-region table covering: wide (preferred), mid (staged), exactly at minimum, and below minimum (refuse, clamped `w`).
- The confirm and commit-failed message lines at the minimum width (`TestPanelFixture_ConfirmWrapsAtMinWidth`, `TestPanelFixture_MinHeightMessageTruncates`).
- Row composition at the minimum: cursor column, `⚠`, badge and label truncation order.
- Visual check of the re-rendered narrow and minimum-height frames, with a verified fresh write.

## Task 12: Restore the bold on the panel cursor row
severity: low
sources: standards

**Problem**: §9.1 pins the panel's cursor row as "the shipped selection treatment (`▌` + tint + white bold name), so the panel's list reads as the same kind of list as Sessions". The Sessions delegate renders the selected name through `nameBase` (lipgloss bold, internal/tui/session_item.go:19) with `text.on-selection` (:495-499); the panel's delegate renders its label through `rowToken` with an explicitly empty base style (internal/tui/theme_row.go:112-116, :253-256), and its comment states the opposite of the spec's requirement — "Panel rows carry no non-colour attribute of their own, so there is no base style to pass". The tint, the `▌` and the token are all correct; only the weight differs, so the panel's cursor row reads slightly lighter than the Sessions cursor row it was specified to mirror.

**Solution**: Pass the shared bold base the Sessions delegate uses for the selected name when the panel row is selected, so all three elements of the shipped selection treatment are reproduced.

**Outcome**: The panel's list reads as the same kind of list as Sessions, which is the stated reason §9.1 pins that treatment.

**Do**:
1. Pass the shared bold base — the same style the Sessions delegate uses for the selected name (`nameBase`, session_item.go:19) — into `rowTokenStyle` from `themeRowDelegate.rowToken` (theme_row.go:253-256), and only when the row is selected.
2. Apply it to the LABEL only, not to the trailing segments (badges, `⚠`, slot indicators): the Sessions treatment bolds the name, and bolding a badge would change a second thing the spec did not ask for. Split the call sites in `renderRow` (theme_row.go:112-116) — the label run versus the trailing loop — rather than bolding everything the delegate renders.
3. Replace the comment at theme_row.go:253-256 which currently asserts the opposite; state the §9.1 requirement it now satisfies.
4. Decide the NO_COLOR path explicitly and state it in the comment: bold is a non-colour attribute, so match whatever the Sessions delegate does for the selected name under NO_COLOR, so the two lists stay the same kind of list under the carve-out too.
5. Re-render and visually check the panel fixtures showing a cursor row — `theme-panel-adaptive-pair` and `theme-panel-constant-previewing`. Live view: `go run ./cmd/capturetool --fixture theme-panel-adaptive-pair`. Verify a fresh PNG write (hash changed) before trusting the capture.

**Acceptance Criteria**:
- The panel's cursor row reproduces all three elements of the shipped selection treatment: `▌`, tint, bold label.
- Only the label is bolded; trailing segments are unchanged.
- Unselected rows are unchanged.
- NO_COLOR behaviour matches the Sessions delegate's for the selected name.
- The colour-literal guard and the swap-and-diff completeness guard stay green.
- `go test ./internal/tui ./internal/capture` passes.

**Tests**:
- A row-anatomy test asserting the selected panel label carries bold and an unselected label does not.
- A test asserting trailing segments are not bolded on the cursor row.
- A parity test that the panel's selected-label base is the same style the Sessions delegate uses for the selected name.
- A NO_COLOR render assertion for the cursor row.
- Visual check of the two re-rendered cursor-row frames, with a verified fresh write.

## Task 13: Perform §13.2's start-of-feature capture deletion and drop the artifact citations
severity: low
sources: standards

**Problem**: §13.2 draws the retention rule and puts two bounded acts in scope: "Everything that exists today as an image or tape is deleted — the committed reference PNGs and the VHS tapes that produce them … delete today's images and tapes once at the start, clear this feature's own once at sign-off." The first act did not happen: two dozen pre-feature Modern Vivid reference PNGs are still committed under `testdata/vhs/reference/` (kill-confirm-modal-mv.png, loading-mv.png, sessions-modern-vivid-v2.png, the edit-modal-*, filtering-*, projects-*, preview-screen-* frames), including ones the spec says "could not survive the token rename and the theme split without a full recapture". They are additionally cited by name from production comments (internal/capture/fixtures.go:376, :528, :559, :592, :638, :695, :1319, :1390, :1413, :1455; internal/tui/keymap.go:81; internal/tui/loading_view.go:17) and from test comments (internal/capture/capture_test.go:446, :510; internal/tui/keymap_test.go:21, projects_view_reskin_test.go:16, projects_keymap_test.go:12, project_row_anatomy_test.go:22), so the surviving set reads as the durable visual-regression baseline §13.2 states does not exist — and this feature's own citations (fixtures.go:770, :823) will become dangling the moment the sign-off clearance runs. code-quality.md bars artifact references in comments for exactly this reason.

**Solution**: Perform the bounded start-of-feature deletion §13.2 scopes, and drop the `testdata/vhs/reference/*.png` citations from doc comments.

**Outcome**: The retention rule §13.2 draws is actually in force — no pre-feature image or tape survives as an accidental regression baseline, and no comment points at an artifact the rule deletes.

**Do**:
1. Delete the pre-feature reference PNGs under `testdata/vhs/reference/` — every image that predates this feature — and any pre-feature `.tape` files.
2. Leave this feature's own `theme-panel-*` frames in place: §13.2 makes clearing those a separate bounded act at sign-off, not part of this one.
3. Do not touch the Go fixture definitions in `internal/capture` or the harness itself — §13.2 is explicit that the deletion covers images and tapes, NOT fixtures, and the swap-and-diff guard's coverage assertion enumerates whatever fixtures exist, so deleting one silently shrinks the guard.
4. Remove the "mirrors / Reference: `testdata/vhs/reference/<name>.png`" citations from the doc comments at internal/capture/fixtures.go:376, :528, :559, :592, :638, :695, :1319, :1390, :1413, :1455, internal/tui/loading_view.go:17 and internal/tui/keymap.go:81, and from the test comments at internal/capture/capture_test.go:446, :510, internal/tui/keymap_test.go:21, projects_view_reskin_test.go:16, projects_keymap_test.go:12 and project_row_anatomy_test.go:22.
5. Keep what each comment says about the DESIGN it mirrors — the frame's content, the established key order, the named reference in prose. Drop only the path to a deleted artifact; do not delete the design rationale along with it.
6. Remove this feature's own two citations (fixtures.go:770, :823) under the same rule, so nothing dangles at sign-off.
7. Do not take on a general repo-wide capture cleanup beyond the images and tapes §13.2 names — it is explicitly out of scope.
8. Leave `testdata/vhs/README.md` and `LOCK-IN.md` in place; update them only where they name a deleted file, and do not change their purpose.

**Acceptance Criteria**:
- No pre-feature reference PNG or tape remains committed.
- This feature's own `theme-panel-*` frames are untouched.
- `grep -rn "testdata/vhs/reference" --include="*.go" .` returns nothing.
- Design rationale in the affected comments survives; only artifact paths are removed.
- The fixture definitions and harness are untouched; `capture.FixtureNames()` returns the same set.
- `go test ./...` passes.

**Tests**:
- The swap-and-diff completeness guard and the fixture-registry parity tests pass with no change to the fixture set.
- `capture.FixtureNames()` is unchanged before and after.
- A grep check that no `.go` file references a `testdata/vhs/reference` path.

## Task 14: Add internal/themetest to CLAUDE.md's internal-packages inventory
severity: low
sources: standards

**Problem**: This feature adds a new internal package, `internal/themetest` (`Builtin` / `DefaultDark` / `DefaultLight` in builtin.go, plus `Lines` / `Body` / `WithValue` / `WithoutKey` / `Write` in theme_file.go), consumed from `cmd`, `cmd/capturetool`, `internal/tui` and `internal/capture` test files. CLAUDE.md's internal-packages table documents every other test-only helper package explicitly, and each carries the "production code must not import" rule in writing (portaltest, logtest, spawntest, restoretest / tmuxtest / portalbintest / transienttest). §12.6 makes CLAUDE.md this feature's own responsibility on the grounds that "CLAUDE.md is what an implementing agent reads first", and all seven named corrections were made correctly — but the new package was not added, so its test-only contract rests solely on the `*testing.T`-first signature convention with nothing written down.

**Solution**: Add a `themetest` row to CLAUDE.md's internal-packages table alongside the other test-only helpers, stating its exports and the production-must-not-import rule.

**Outcome**: The test-only contract for `themetest` is written where an implementing agent reads it first, matching how every other test-only helper package is documented.

**Do**:
1. Read `internal/themetest/builtin.go` and `internal/themetest/theme_file.go` and confirm the exported surface before writing anything: `Builtin(t, slug)`, `DefaultDark(t)`, `DefaultLight(t)`, `Lines()`, `Body()`, `WithValue(lines, key, value)`, `WithoutKey(lines, key)`, `Write(t, dir, base, lines)`.
2. Add a `themetest` row to CLAUDE.md's internal-packages table, placed with the other test-only helper rows.
3. State the two halves of the package: the built-in accessors that hand back a parsed `theme.Theme` for a slug (and the shipped dark/light defaults), and the theme-file helpers that build a valid `.theme` body, mutate one key, drop one key, and write a file into a temp themes directory.
4. State the "test-only — production code must not import" rule in the same words the sibling rows use.
5. Name its consumers (`cmd`, `cmd/capturetool`, `internal/tui`, `internal/capture` test files) so a reader knows where the contract is exercised.
6. Change nothing else in CLAUDE.md.

**Acceptance Criteria**:
- CLAUDE.md's internal-packages table has a `themetest` row whose stated exports match the package's actual public API exactly.
- The production-must-not-import rule is written down for `themetest` as it is for every other test-only helper.
- The row names the package's consumers.
- No other CLAUDE.md content changes.

**Tests**:
- Verify the listed exports against `internal/themetest`'s exported symbols (no code change, so the check is a read-back).
- Confirm no production (non-`_test.go`) file imports `internal/themetest`.
- `go test ./...` still passes (unchanged).
