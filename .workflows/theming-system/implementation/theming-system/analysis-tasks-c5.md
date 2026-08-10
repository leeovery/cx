# Analysis Tasks: theming-system (Cycle 5)

## Task 1: Drive doctor's theme block off one enumeration and off theme.Slot's own labels
severity: medium
sources: architecture, duplication

**Problem**: `portal doctor`'s theme block runs two producers over the same directory and parses it twice. `scanThemesDirectory` calls `loader.Enumerate(dir)` — one ReadDir plus one parse per candidate — and `persistedThemeAdvisories` then calls `loader.ResolveByName(slug, themesDir)` once per in-force key (cmd/doctor_theme.go:313), which stats the directory again and re-reads and re-parses a file the enumeration parsed moments earlier. `internal/theme` already owns the primitive that avoids exactly this: `resolveFromEnumeration` / `entryResult` (internal/theme/resolution.go:284-295) run `ResolveByName`'s own ladder against a *retained* `Enumeration` with no I/O, and they exist because a second parse of the same slug "is free to disagree with the row the user is looking at" (resolution.go:155-157). Doctor is the one surface that reintroduces that hazard: its file line for slug X comes from parse A and its persisted line for the same slug from parse B, with the file line then dropped on the strength of a slug read from parse A. Separately, `persistedThemeSlotLabel` (cmd/doctor_theme.go:277) re-authors `theme.Slot.AttrName`'s body (internal/theme/resolution.go:36) as a `switch` — including the "a constant has no name" empty-string arm — directly under a comment at cmd/doctor_theme.go:60 stating that the light/dark words are read off `theme.Slot`'s own mapping "rather than restated". The two vars that derivation produces (`themeSlotLight`/`themeSlotDark`) exist solely to feed that switch, so a slot added to the vocabulary would compile here and silently render the constant's empty label.

**Solution**: One enumeration, taken once, driving both of doctor's producers; and the slot's word read from `theme.Slot.AttrName` at the one site that renders it.

**Outcome**: Doctor's two theme lines about one slug describe one parse of one file, the directory is read once per diagnosis, and the light/dark words exist in `internal/theme` alone.

**Do**:
1. Export the enumeration-backed by-name resolver from `internal/theme`: a thin `func (l Loader) ResolveByNameFrom(e Enumeration, slug string) (Result, *Rejection)` over the existing unexported `resolveFromEnumeration` (internal/theme/resolution.go:255-260). Document it as `ResolveByName`'s identical ladder — charset check, embedded set first, then the source — with the retained enumeration in place of the directory, performing no I/O and emitting nothing (matching `ResolveByName`, which reports at `reportSlot` and not in itself).
2. In `themeAdvisoryUnion` (cmd/doctor_theme.go:133), take the enumeration ONCE and hand it to both producers. Keep the single shared `theme.NewSilentLoader()` and its owned dedup set exactly as it is.
3. Re-point `persistedThemeAdvisory` (cmd/doctor_theme.go:313) from `loader.ResolveByName(slug, themesDir)` to the new enumeration-backed resolver. The `themesDir` argument becomes unnecessary on that path — drop it rather than leaving it unread.
4. Preserve every property doctor's comments claim for the persisted producer: it must still resolve by NAME and never through `ResolveNomination` (no mode-matched fallback substitution, no broken-built-in fatal), a persisted slug naming a `reserved name` file must still resolve to the built-in and emit no persisted line, and every reason discrimination must still be the resolver's own.
5. Reduce `persistedThemeSlotLabel` (cmd/doctor_theme.go:277) to the `Both` special case plus `name, _ := key.Slot.AttrName(); return name`, and delete the `themeSlotLight` / `themeSlotDark` vars whose only readers are the deleted switch arms. Keep `themeSlotBoth` — the setting has no slot for `AttrName` to name it by, so that label is genuinely doctor's own.
6. Leave doctor's assembly untouched: `assembleThemeAdvisories`' three pinned regions, its persisted-wins survivor choice, `persistedSlugs`' `fromPrefs`-keyed membership and the byte-identical-across-runs ordering all stay exactly as they are. The union's own `persistedRows`/`listedUnder` picks the OPPOSITE survivor deliberately, so do NOT attempt to share the membership rule between the two surfaces — only the resolution source moves.
7. Change no rendered line. Doctor's copy, line frames, slot labels, advisory ordering and exit code are unchanged by this task.

**Acceptance Criteria**:
- A doctor run over a themes directory parses each candidate file exactly once; no second `ReadDir` or per-slug file open happens after the enumeration.
- Doctor's file line and persisted line for the same slug are derived from the same parsed entry.
- `internal/theme` exposes one documented enumeration-backed by-name resolver; `resolveFromEnumeration` has no second implementation.
- `persistedThemeSlotLabel` contains no `case theme.SlotLight` / `case theme.SlotDark` arms, and `themeSlotLight` / `themeSlotDark` are gone.
- Doctor's rendered output is byte-identical to today's for: a usable directory with valid and invalid files, an unreadable directory, a persisted slug that is missing, one that is charset-illegal, one that names a `reserved name` file, and a constant setting.
- `go test ./...` and `go test -tags integration -p 1 ./cmd` pass.

**Tests**:
- A doctor test asserting the themes directory is read once per diagnosis — e.g. a counting `ThemesDir` fixture or a per-file open counter — failing if a slug is parsed twice.
- A test that a file whose content would differ between two reads cannot produce disagreeing file and persisted lines (stage the file, take the diagnosis, assert both lines describe one parse).
- Existing doctor theme-advisory tests pass unchanged, including the `reserved name` non-collision and the `bad name` no-slug case.
- A `theme` test pinning the new resolver against `ResolveByName`: same slug, same enumeration content, same `Result`/`Rejection` for a valid file, a rejected file, a missing slug, a built-in slug and a charset-illegal slug.
- Temporarily add a fourth `theme.Slot` value and confirm `persistedThemeSlotLabel` renders its `AttrName` rather than an empty label; revert.

## Task 2: Stop the panel's blank page-alignment rows from raising the height refuse threshold
severity: medium
sources: standards

**Problem**: §9.1 fixes the theme panel's header as the `Themes` label plus a one-row `border` rule — "two rows, which is what §9.8's minimum-height rule (header + footer + one row) resolves against". The implementation measures the header off the *page's* chrome instead: `themePanelHeaderRows()` = `themePanelHeaderLabelRow()` (`lipgloss.Height(renderHeaderBlock(...))` = band + rule + blank = 3) + `sectionHeaderBlockRows()` (2) = **5 rows**, of which only two carry anything — `themePanelHeaderBlock` (internal/tui/theme_panel_render.go:63-70) writes the rule at index 1 and the label at index 3 and leaves indices 0, 2 and 4 deliberately blank. `themePanelMinHeight` therefore computes 5 + footer(4) + message(1) + body(1) = **11** (12 with the pinned `⚠ dir unreadable` row) against §9.8's 2 + 4 + 1 + 1 = **8**, so `themePanelFloor` returns `dimHeight` and refuses `t` (and force-closes on resize) across a ~3-row band of content heights where §9.8 says the panel must still degrade and render. §9.8's doctrine is "degrade, don't refuse … refuses only when even the minimum panel cannot render", and here the refusal is driven entirely by rows the panel draws nothing in. Two in-source geometry claims are also stale: `themePanelBlock`'s "rows are padded but never truncated" argument lists "the widest footer row 15" (internal/tui/theme_panel_render.go:125) while `renderThemePanelFooter`'s own comment says 16 (internal/tui/theme_panel_footer.go:52) — the row is `themePanelFooterKeyColumnWidth`(3) + `footerKeyLabelGap`(1) + `"set as light"`(12) = 16, so the 15 is false, and both numbers are load-bearing for the same "no composed row can exceed the minimum inner width" argument.

**Solution**: Make the blank page-alignment rows a degradation step rather than a floor cost — one height-aware predicate consumed by both the layout arithmetic and the header renderer — so the refuse threshold equals §9.8's stated composition, and correct the false width figure.

**Outcome**: The panel refuses on height only when even a rule row, a label row, one list row, the message slot and the footer cannot fit. At every height that affords the page-aligned header today, the frame is byte-identical to today's. Both in-source width figures state the same, correct number.

**Do**:
1. **Scope guard — the width ladder does not move.** `themePanelPreferredWidth` (30), `themePanelMinWidth` (24), `themePanelPreferredAffordance` and the `dimWidth` threshold are OUT OF SCOPE and must be left byte-identical: the 24–30 column band was reviewed and chosen deliberately in task 13-11. This task changes the HEIGHT floor only.
2. Add one predicate in internal/tui/theme_panel_geometry.go answering "does this render height afford the page-aligned header?" — i.e. whether the full 5-row header fits alongside the panel's other chrome and one list row. It is the single decision; nothing else may re-derive it.
3. Make `themePanelHeaderRows` take the render height and return the page-aligned cost when the predicate says yes and the compact cost (the rule row plus the label row) when it says no, so `themePanelChromeRows`, `themePanelMinHeight` and `themePanelListSize` all follow it with no second edit — the discipline the file already applies to the footer and directory row.
4. Make `themePanelHeaderBlock` render the matching shape: the existing page-aligned block above the affordance, and below it a compact block of the rule row then the label row with no blank alignment rows. The reserved rows must by construction be the rows that render — the same rule stated over `themePanelDirRowHeight` and `themePanelFooterHeight`.
5. Let `themePanelBorderFromRow` keep deriving from `themePanelHeaderRuleRow` so the left `│` starts below the rule in both shapes, and confirm `themePanelBlock` still pads rather than truncates at the compact heights (the footer must never be cut from the bottom).
6. Recompute `themePanelMinHeight` from the compact header cost so the floor is header content rows + footer + message row + one body row (+ the directory row when unusable) — §9.8's arithmetic exactly.
7. Correct the stale figures: change `themePanelBlock`'s "the widest footer row 15" to 16, or drop the per-surface width list from that comment and point at `renderThemePanelFooter`, which owns the measurement. Touch only that paragraph.
8. Update the header measurement note above `themePanelHeaderRuleRow` and `themePanelHeaderBlock`'s doc comment to state the two shapes and why the blank rows are an alignment luxury rather than a floor cost.
9. Record the remaining deviation from §9.1 explicitly in-source at `themePanelHeaderBlock`: the specification composes the header as label-then-rule, and the implementation renders rule-above-label so the panel shares the page's rule lane. That inversion is a deliberate, already-reviewed visual decision and this task does NOT flip it — if the specification's literal order is wanted instead, that is a separate visual change to be chosen on the reference frames.

**Acceptance Criteria**:
- At every content height that affords the page-aligned header today, `renderThemePanel` output is byte-identical to today's (dark, light and colourless).
- `themePanelMinHeight` equals header content rows + footer rows + one message row + one body row, plus the directory row when unusable — §9.8's composition, with no blank alignment rows counted.
- Across the previously-refusing band the panel opens and renders a complete block: rule row, label row, at least one list row, the message slot and the FULL footer, all within the height, with nothing truncated.
- `themePanelFloor` still returns `dimWidth` before `dimHeight`, and the width ladder's constants and thresholds are unchanged.
- The header shape is decided in exactly one place; no renderer or arithmetic re-derives it.
- Both in-source references to the widest footer row state 16.
- `go test ./internal/tui` passes and the panel capture fixtures re-render unchanged.

**Tests**:
- A geometry test pinning `themePanelMinHeight` to the §9.8 arithmetic for both the usable and unusable-directory cases.
- A render test at the smallest height that clears the new floor, asserting the block is exactly that many rows, the footer's every Core row is present, and one list row renders.
- A render test at heights spanning the affordance boundary asserting the page-aligned frame is byte-identical above it and the compact frame appears below it.
- A test that `themePanelFloor` still reports `dimWidth` for a terminal failing both dimensions.
- A footer test asserting the widest rendered footer row is 16 cells, so the corrected comment is pinned by a measurement rather than by a reader.

## Task 3: Collapse the model's three light/dark representations onto one owned answer
severity: medium
sources: architecture

**Problem**: Three separate representations of "light or dark" coexist on the model, and the correct one to read at each site is enforced only by comments. `appearanceGate.appearance` (internal/tui/appearance_gate.go:51-64) is the gate's resolution, which on a pinned/constant gate is the standing dark fallback and is explicitly *not* a fact about the terminal. `themeState.canvasMode` (internal/tui/theme_state.go:58-69) is the answer in force, mirroring the gate via `syncResolvedMode` (model.go:1404-1408) and then deliberately diverging from it after a mid-session constant → adaptive conversion. `themeState.bgReplyArrived` / `bgReplyDark` (theme_state.go:96-107) is the raw OSC 11 classification, set in the `BackgroundColorMsg` arm (model.go:2609-2610) and read through `Model.retainedCanvasAnswer`. All three are `theme.Member`-valued or classify into one, and each carries a doc comment warning which of the other two must not be substituted for it — `inForceSlot`'s "the mode read here must not be `gate.appearance`" (theme_panel.go:488-493), `themeState`'s "two of its fields look like drift and are not", `retainedCanvasAnswer`'s "not read off the gate". The divergence is safe today only because a pinned gate can never re-resolve, so `syncResolvedMode` cannot run after a conversion and clobber `canvasMode` back to dark — a non-local coupling between `appearanceGate.pinned`, the `BackgroundColorMsg` arm and the conversion path that holds by construction without that construction being visible. Correctness of the converted-light-terminal case therefore rests on every future call site picking the right one of three near-identical fields, with no type distinguishing "what the terminal said" from "what the gate resolved" from "what is in force".

**Solution**: Model the terminal's reply once as an explicit optional value, and give callers one accessor for the answer in force, so no site has to choose among near-identical fields.

**Outcome**: There is one thing to read for "the light/dark answer in force" and one typed value for "what the terminal actually said". The pinned-gate coupling stops being load-bearing, and the "must not substitute" warnings become unnecessary rather than essential.

**Do**:
1. Replace the loose `bgReplyArrived` / `bgReplyDark` pair with one small optional value on `themeState` — e.g. `detectionAnswer{arrived bool, member theme.Member}` — fed solely by the `BackgroundColorMsg` arm. Keep the classification coming off `tea.BackgroundColorMsg.IsDark` (nil-safe) and NOT re-derived from the retained hex; keep arrival tracked separately from a non-empty `originalBg`, since a no-answer-shaped reply is still an arrival.
2. Re-express `Model.retainedCanvasAnswer` as a read of that value, so "did the terminal answer, and what did it say" is one question against one field.
3. Expose a single accessor on `themeState` for the answer in force — the gate's resolution for an armed adaptive gate, the retained reply for a converted constant — and route every consumer through it: `inForceSlot` (theme_panel.go:486-515), the confirm/conversion path (theme_panel_confirm.go:197-247), and any other site currently reading `canvasMode` or `gate.appearance` for this purpose.
4. Preserve the divergence semantics exactly. A converted constant → adaptive session in a LIGHT terminal must still resolve to the light slot, and the pinned gate must still keep its own fallback. If keeping `canvasMode` as the stored owned value is the simplest way to hold that, keep it — the requirement is that callers read ONE accessor, not that the field disappear.
5. Do **not** touch `themeState.nomination` or its post-commit assignment in `applyCommittedSetting`: task 13-9 settled that the assignment stays and the field's contract was made honest instead. This task must leave that field and its comment alone.
6. Do **not** move or re-derive `startupCanvasHex`. It is captured at the single moment the gate selects a member and must never follow the active theme; `RestoreTerminalBackground`'s canvas-echo guard depends on it and CLAUDE.md records that the guard must not be dropped.
7. Rewrite the doc comments that survive to describe the shape rather than warn against the alternatives, and delete the warnings the collapse makes untrue.

**Acceptance Criteria**:
- One typed optional holds the terminal's reply; `bgReplyArrived` / `bgReplyDark` no longer exist as a loose pair.
- One accessor answers "the light/dark answer in force", and no call site outside it reads `gate.appearance` or `canvasMode` to make that decision.
- The single-resolution rule is intact: the canvas is painted once and never flips; a late OSC 11 reply is still consumed but never re-themes.
- A constant → adaptive conversion in a light terminal still lands on the light slot; in a dark terminal and with no reply at all it still lands on dark.
- `startupCanvasHex` is captured at the same moment and from the same value as today, and `RestoreTerminalBackground`'s canvas-echo guard behaves identically.
- `themeState.nomination` and its commit-time assignment are unchanged.
- `go test ./internal/tui` passes; the appearance-gate and restore source guards still hold.

**Tests**:
- The existing converted-constant-in-a-light-terminal test still passes, driven through the new accessor.
- A test that a pinned (never-armed) gate plus an arrived light reply resolves the conversion to light, and that the gate's own field is untouched.
- A test that a no-answer-shaped reply (nil Color) records an arrival classified dark and leaves the retained hex empty.
- A test that a late reply after the timeout resolved the gate does not re-theme, but is still recorded as the terminal's answer.
- The `RestoreTerminalBackground` canvas-echo tests pass unchanged, including the quit-with-uncommitted-preview and commit-mid-session cases.

## Task 4: Make theme.Loader's unsafe zero value unreachable from production code
severity: medium
sources: architecture

**Problem**: `theme.Loader` (internal/theme/load.go:19-48) is an exported struct with exported `ReservedSlugs` and `BuiltinSource` fields, so `theme.Loader{}` is a legal expression in any package: it compiles, judges files through the full rejection ladder, and reserves NOTHING — meaning a user's `tokyo-night.theme` would shadow the built-in the per-slot fallback resolves to, which is the single property `NewLoader`'s doc calls out as needing to be impossible ("the built-in Portal falls back to must never be a file the user can supply"). `NewLoader` panics on a nil event seam specifically so silence is a readable decision at the call site (internal/theme/load.go:66-71), but the zero value bypasses that check entirely and fails silently rather than loudly. The fields are exported only so the package's own tests can drive the ladder with a synthetic reserved set — verified: `theme.Loader{...}` appears at ~20 test sites and at exactly one production site, `NewLoader` itself. The precedent for closing this is already in the codebase: task 14-12 made `NewSilentLoader` the only route to a silent loader with a guard of exactly this shape (internal/tui/builtin_themes_test.go, internal/theme/silent_loader_test.go), alongside `leaf_guard_test.go`, `restore_source_guard_test.go` and `colour_literal_guard_test.go`.

**Solution**: A source guard, in the shape the package already uses, asserting that no `theme.Loader` composite literal appears in production code — so the only routes to a loader are `NewLoader` and `NewSilentLoader`.

**Outcome**: A loader that reserves no built-in slugs is unreachable from the binary rather than merely documented as a test shape, and the no-shadowing guarantee holds by construction.

**Do**:
1. Add a source guard asserting no `theme.Loader{` / bare `Loader{` composite literal occurs in any non-`_test.go` file, with `NewLoader`'s own construction in internal/theme/load.go named as the single explicit exemption. Follow 14-12's guard shape and the file enumeration already single-sourced in `portalbintest` (task 14-4) — do not hand-roll a new package scan.
2. Scan the whole repository, not just `internal/theme`: the hazard is a call site in `cmd`, `internal/tui`, `internal/capture` or `cmd/capturetool` constructing the zero value, which is where it would actually be written.
3. Make the failure message say what is wrong and what to do — that a zero-value `Loader` reserves no built-in slugs and lets a drop-in shadow the built-in a slot falls back to, and that production callers take `NewLoader` or `NewSilentLoader`.
4. Fatal on an empty enumeration rather than passing vacuously, matching the vacuity rule the sibling guards already apply.
5. Leave the exported fields as they are. Unexporting them behind constructors would force `internal/themetest` and the package's own ladder tests through a new synthetic-set constructor for no gain the guard does not already deliver; the fields' doc comments already state that the zero value is a test shape — extend them to name the guard that now enforces it.

**Acceptance Criteria**:
- A guard exists that fails when a `theme.Loader` composite literal is introduced into any production file.
- `NewLoader`'s own construction is exempted explicitly and by name, not by a broad pattern.
- The guard fatals rather than passing when its enumeration is empty.
- Test files continue to construct `theme.Loader{...}` freely; no existing test changes.
- `go test ./...` passes.

**Tests**:
- Temporarily add a `theme.Loader{}` literal to a production file in `cmd` and confirm the guard fails naming the file; revert.
- Temporarily add one inside `internal/theme` (outside `NewLoader`) and confirm it also fails; revert.
- Confirm the guard passes on the current tree and that the exemption covers only `NewLoader`'s construction.
- Point the guard at an empty enumeration and confirm it fatals; revert.

## Task 5: Single-source the two-built-in subtest table across internal/tui's render suite
severity: medium
sources: duplication

**Problem**: Converting the render tests from a light/dark appearance parameter to a whole `theme.Theme` left the same anonymous-struct table copied verbatim across `internal/tui`. Fourteen occurrences in twelve files spell out `for _, tc := range []struct{ name string; th theme.Theme }{{"dark", testDarkTheme(t)}, {"light", testLightTheme(t)}} { t.Run(tc.name, …) }` — footer_test.go:77, header_test.go:19 and :225, help_modal_frame_test.go:23, help_modal_test.go:338, modal_footer_test.go:179, multi_select_banner_test.go:32, pagination_dots_test.go:58, panel_test.go:17, projects_footer_test.go:82, projects_header_test.go:26, section_header_test.go:29 and :289, unsupported_banner_test.go:46. A second variant over the answer rather than the palette — `{ name string; appearance theme.Member }{{"dark", theme.MemberDark}, {"light", theme.MemberLight}}` — is copied in four more (canvas_cell_background_test.go:142, canvas_paint_test.go:54, help_modal_test.go:386, modal_blank_screen_test.go:105). That is ~120 lines whose only content is "run this against both shipped built-ins", enumerating a pair that `internal/theme` already single-sources as `DefaultDarkSlug`/`DefaultLightSlug` — and `internal/theme/contrast_test.go` has already moved to auto-enumerating the embedded set for exactly this reason. The helpers the tables call (`testDarkTheme`/`testLightTheme`, internal/tui/theme_testing_test.go:233-240) already live in one shared place; only the loop around them was left per-file.

**Solution**: Two iterators beside the existing shared helpers, each running its own `dark`/`light` subtests, with the eighteen sites rewritten as one call taking the body as a closure.

**Outcome**: The built-in pair the render suite runs against has one declaration, and a change to which themes that means is one edit rather than eighteen.

**Do**:
1. Add `forEachBuiltinTheme(t *testing.T, fn func(t *testing.T, th theme.Theme))` to internal/tui/theme_testing_test.go beside `testDarkTheme`/`testLightTheme`, running the `dark` and `light` subtests itself with the loaded palette.
2. Add `forEachCanvasMode(t *testing.T, fn func(t *testing.T, m theme.Member))` for the four sites parameterised by the answer rather than the palette.
3. Rewrite all fourteen palette sites and all four mode sites as a single call taking the existing body as a closure. Keep each subtest's name (`dark` / `light`) so test output and any `-run` filters are unchanged.
4. Derive the pair from `internal/theme`'s own shipped-default slugs rather than restating them, so the suite follows a change to the shipped pair with no edit here.
5. Change no assertion. Every token check, byte comparison and failure message stays exactly as it is; only the loop moves.
6. Do not fold in sites that iterate something other than the two built-ins (a synthetic palette, a three-case table, a colourless variant) — those are not this table.

**Acceptance Criteria**:
- Two iterators exist in one place; no `internal/tui` test declares its own `{"dark", …}, {"light", …}` two-built-in table.
- Subtest names and the set of tests run are identical to today's.
- Every assertion and failure message is unchanged.
- The pair is read from `internal/theme`'s shipped-default slugs, not restated.
- `go test ./internal/tui` passes.

**Tests**:
- Full `go test ./internal/tui` with unchanged test names — compare `go test -v` test-name output before and after.
- Temporarily break one render surface for the light palette only and confirm the corresponding `light` subtest fails at each rewritten site; revert.
- Temporarily point the iterator at a single theme and confirm the suite's test count drops, proving the iterator is what enumerates.

## Task 6: Route the AST guards' call-walk preamble through one iterator
severity: medium
sources: duplication

**Problem**: The source guards this feature added repeat one traversal preamble at roughly a dozen sites: `for _, decl := range file.Decls` → assert `*ast.FuncDecl` → `ast.Inspect` → assert `*ast.CallExpr` → apply this guard's predicate. It is written out at cmd/open_theme_nomination_test.go:224 and :317, cmd/prefs_translation_test.go:397, cmd/doctor_persisted_theme_test.go:684, cmd/doctor_theme_union_test.go:399, internal/tui/theme_panel_commit_test.go:354, internal/tui/theme_panel_confirm_test.go:1209, internal/tui/theme_panel_close_test.go:702, internal/theme/resolve_test.go:826 and internal/capture/theme_swap_guard_test.go:381, among others. The copies already differ in ways nothing forces — some inspect `fn`, some `fn.Body`, some guard `fn.Body == nil` and some do not — so guards that read as siblings walk subtly different trees. Each guard's *query* (which identifier, which selector, which package qualifier) is legitimately its own; the traversal under it is not, and it is the half that silently diverges. Note the scope: the file *enumeration* half of this scaffolding was single-sourced by task 14-4 (`portalbintest.PackageGoFiles`) and the repo-wide walk by 13-3 (`GoSourceFiles`), and the residual per-package enumerate-and-parse loop is written twice by design — this task does NOT revisit either.

**Solution**: One call-walk iterator beside the existing enumeration helpers, with the dozen preambles reduced to their predicate.

**Outcome**: Every call-scanning guard walks the same tree the same way; only what it is looking for differs, which is the part that should differ.

**Do**:
1. Add a call-walk iterator to `internal/portalbintest`, beside `PackageGoFiles` / `GoSourceFiles` — e.g. `ForEachFuncCall(file *ast.File, fn func(funcName string, call *ast.CallExpr) bool)` — walking each `*ast.FuncDecl` and yielding every `*ast.CallExpr` inside it with the enclosing function's name. `portalbintest` is stdlib-only and unit-lane reachable, so `cmd`, `internal/tui`, `internal/theme` and `internal/capture` can all import it.
2. Pin ONE traversal decision in it and document it: whether the walk covers the whole `*ast.FuncDecl` or only its `Body`, and how a nil body is handled. Pick whichever choice preserves every current guard's coverage; where a guard would change coverage, keep that guard's coverage and say so at its call site.
3. Rewrite the ~12 preamble sites to call the iterator, keeping each guard's predicate, its collected shape (site strings, name pairs, counts) and its failure wording exactly as they are.
4. Leave the enumerate-and-parse front ends alone: `parsePackageFilesByName` in `cmd` and `internal/tui`, and `parseThemeSources`/`themeSourceFiles` in `internal/theme`, are the deliberate residue task 14-4 scoped out. This task touches only what happens AFTER a file is parsed.
5. Do not weaken any guard: every forbidden identifier, selector and package qualifier asserted today must still be asserted, and every guard that sorts its findings for a stable message must still sort them.

**Acceptance Criteria**:
- One call-walk iterator exists; no test re-authors the `Decls → FuncDecl → Inspect → CallExpr` preamble.
- The traversal decision (whole declaration versus body, nil-body handling) is stated once and identical at every site.
- Every guard still fails when its forbidden construct is reintroduced, with unchanged failure wording.
- The enumerate-and-parse helpers in `cmd`, `internal/tui` and `internal/theme` are untouched.
- `go test ./...` passes and no test moves lane.

**Tests**:
- A `portalbintest` test pinning the iterator: calls inside nested functions and closures are visited, the enclosing function name is reported, a nil body is handled, and `false` from the callback stops the walk.
- Temporarily reintroduce each rewritten guard's forbidden construct and confirm it still fails; revert each.
- Confirm each rewritten guard reports the same site strings as before on a deliberately-failing tree.

## Task 7: Derive every panel fixture's union from its rows
severity: low
sources: duplication

**Problem**: `themeRowsUnion(rows)` (internal/tui/theme_panel_commit_recompute_test.go:442-444) is the named form of "wrap hand-declared rows as the union the seam hands back", deriving `Count` from `len(rows)` and `Rejected` from the rows' own `Selectable()`. Nine sibling fixtures in the same package build the same value inline instead, and they have already drifted in the field that matters: theme_panel_arrow_test.go:139 derives `Rejected` through `arrowRejectedCount`, theme_panel_open_test.go:87 and theme_panel_cursor_test.go:364 hardcode `Rejected: 1`, and theme_panel_commit_test.go:113, theme_panel_cursor_test.go:331 and :412 and theme_panel_open_test.go:552 omit it entirely for unions that do contain rejected rows. `Rejected` is what `theme: enumerated` reports and what the panel's own assertions read, so a hand-carried count is a fixture that can disagree with its own rows — and a test can then pass while asserting a number the data does not support.

**Solution**: One derivation of `Count`/`Rejected` from the rows, used at every fixture site.

**Outcome**: A panel fixture cannot state a rejected count its own rows contradict, and `arrowRejectedCount` has one caller inside the helper that owns the derivation.

**Do**:
1. Route all nine inline `theme.Union{Rows: …, Count: …, Rejected: …}` literals through `themeRowsUnion`.
2. Give `themeRowsUnion` the `DirUnusable` variant that theme_panel_test.go:70 and theme_panel_entry_test.go:71 need — either a second `themeRowsUnionUnusable` helper or an explicit flag; do not add an options struct.
3. Leave `arrowRejectedCount` where it is, called only from inside the helper.
4. Where a fixture's hardcoded `Rejected` currently disagrees with its rows, the derived value is the correct one — fix the fixture's ROWS if an assertion then fails, never re-hardcode the count.
5. Change no assertion. Only how each fixture's union is constructed moves.

**Acceptance Criteria**:
- No `theme.Union` literal in `internal/tui`'s tests carries a hand-written `Count` or `Rejected`.
- Every fixture's `Rejected` equals the count of non-selectable rows it declares.
- `arrowRejectedCount` has exactly one caller.
- Every existing panel assertion still holds, with any fixture divergence resolved in the rows rather than the count.
- `go test ./internal/tui` passes.

**Tests**:
- Existing panel tests pass with derived unions.
- A test asserting `themeRowsUnion` derives `Count` and `Rejected` from the rows, including a rows slice with zero and with several rejected rows.
- Temporarily add a rejected row to one fixture and confirm the union's `Rejected` follows without a second edit.

## Task 8: Name theme.Row's identity separately from its sort key
severity: low
sources: architecture

**Problem**: `Row.SortKey()` (internal/theme/union.go:92-94) is documented as an ordering value — "the ordering consumes SortKey … neither may be re-derived from the other" — yet three consumers use it as the row's *identity*: `BadgeKey` returns it (internal/theme/badge.go:102-107), `themePanelRowIndex` matches on it to place the cursor (internal/tui/theme_panel.go:560-566), and `previewedThemeIdentity` captures it to re-anchor after a commit (internal/tui/theme_panel_commit.go:307-320). Each of those sites carries a comment explaining that a sort key is standing in for an identity, which is the tell. The two concepts coincide today (slug → filename → persisted string) but are independent: any future change to the ordering — sorting by label, grouping built-ins — would silently relocate the panel's cursor anchoring and badge lookup with it, and the failure mode is the invariant the panel is built around, the cursor sitting on a row other than the one being painted.

**Solution**: An explicit `Identity()` on `theme.Row` holding the slug/filename/persisted precedence, with `SortKey()` and `BadgeKey()` defined in terms of it and the three identity consumers reading it.

**Outcome**: Ordering and identity are two named values that can be changed independently, and the panel's cursor anchoring no longer depends on how the list happens to be sorted.

**Do**:
1. Add `func (r Row) Identity() string` to internal/theme/union.go carrying the `cmp.Or(r.Slug, r.Filename, r.Persisted)` precedence, documented as the row's stable identity — what the badge table keys on and what the cursor anchors to — and explicitly not an ordering value.
2. Define `SortKey()` as returning `Identity()` today, with a comment stating that they coincide by current choice and may diverge.
3. Define `BadgeKey()` in terms of `Identity()`, keeping the `ReasonReservedName` exclusion and the empty-string "no badge" answer exactly as they are.
4. Re-point `themePanelRowIndex` and `previewedThemeIdentity` at `Identity()`, and delete the apologetic comments at both sites that explain the stand-in.
5. Change no behaviour: the values are identical today, so ordering, badge placement and cursor anchoring must all be byte-identical.

**Acceptance Criteria**:
- `theme.Row` exposes a documented `Identity()`; `SortKey()` and `BadgeKey()` are defined in terms of it.
- The three identity consumers read `Identity()`, and no comment explains a sort key standing in for an identity.
- Panel ordering, badge placement and post-commit cursor anchoring are unchanged.
- `go test ./internal/theme ./internal/tui` passes.

**Tests**:
- A `theme` test pinning `Identity()`'s precedence across the row shapes: a slugged row, a `bad name` row with only a filename, and a charset-rejected persisted row with neither.
- A test that `BadgeKey()` still returns empty for a `reserved name` row and `Identity()` otherwise.
- Temporarily change `SortKey()` to order by `Label` and confirm the panel's cursor anchoring and badge lookup are unaffected; revert.

## Task 9: Make the ThemeSource seam uniform in what it consumes
severity: low
sources: architecture

**Problem**: The `ThemeSource` seam's four methods take three different shapes of the same state (internal/tui/theme_seams.go:32-37): `Open` and `Reassemble` take `theme.RawKeys`, `Resolve` takes an already-collapsed `theme.Setting`, and `ResolveSlot` takes a slot plus an already-*defaulted* slug. The consequence is that two rules belonging to `internal/theme` have to be applied on the TUI side before the seam can be called — `themeSetting()` (internal/tui/theme_panel.go:475-478) re-runs `theme.ResolveSetting` to collapse the keys, and `persistedSlotSlug` (theme_panel_confirm.go:197-220) switches on `theme.Slot` to pick the shipped-default-substituted slug out of the resolved `Setting`. Both are documented as having "a single site", but that site is in the render layer rather than behind the seam, so a fake is free to answer a differently-derived setting from the one the panel lists and marks. The asymmetry buys nothing: `Reassemble` already collapses `RawKeys` internally via `InForceKeys`, so `Resolve` taking the same input would be uniform at no cost.

**Solution**: One input shape across the seam — the raw keys — with the collapse and the shipped-default substitution done inside `internal/theme`.

**Outcome**: No resolution or defaulting rule is applied outside the package that owns it, and a fake cannot answer from a setting derived differently from the one the panel renders.

**Do**:
1. Change `Resolve` to take `theme.RawKeys` and collapse internally through `ResolveSetting`, matching what `Reassemble` already does.
2. Add a `Setting.Slug(Slot) string` accessor to `internal/theme` returning the slot's nominated slug with the shipped default already substituted, and change `ResolveSlot` to take the slot plus the keys (or the setting) rather than a pre-defaulted slug.
3. Delete `persistedSlotSlug` from `internal/tui` and re-point its caller at the accessor.
4. Keep `themeSetting()` only for the one thing the TUI genuinely decides — whether `d`/`l` must raise the confirm — and narrow its doc comment to that.
5. Update the production adapter and every fake in one pass so the seam's shape is uniform everywhere, and keep the `theme: loaded` emission cadence exactly as it is: `Resolve` emits no load line, `ResolveSlot` still emits one carrying the slug that actually rendered.
6. Preserve the no-I/O guarantee: none of the four methods may read the filesystem, and all must continue to resolve against the enumeration `Open` retained.
7. Change no behaviour. The panel opens on the same theme, the confirm raises on the same conditions, and the commit loads the same slot.

**Acceptance Criteria**:
- All four `ThemeSource` methods take `theme.RawKeys` (plus a slot where relevant); none takes a pre-collapsed `Setting` or a pre-defaulted slug.
- `persistedSlotSlug` is gone and no `theme.Slot` switch selecting a slug remains in `internal/tui`.
- `Setting.Slug(Slot)` performs the shipped-default substitution inside `internal/theme`.
- The `theme: loaded` emission cadence is unchanged, and no seam method reads the filesystem.
- Panel open, `Esc` close, badge recompute and the constant → adaptive commit behave identically.
- `go test ./internal/tui ./internal/theme` passes.

**Tests**:
- A `theme` test pinning `Setting.Slug(Slot)` for a constant, an adaptive pair, and a slot left unset (which must yield the shipped default).
- Existing panel open / close / commit / confirm tests pass unchanged against the reshaped seam.
- A test that the commit still emits exactly one `theme: loaded` carrying the slug that rendered, and that `Resolve` emits none.
- A fake driven with raw keys produces the same union and resolution the production adapter does for the same keys and enumeration.
