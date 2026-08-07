# Analysis Tasks: theming-system (Cycle 1)

## Task 1: Extract the shared per-list canvas/colourless restyle sequence
severity: high
sources: duplication

**Problem**: `applyCanvasMode` (internal/tui/model.go:1752), `applyProjectCanvasMode` (internal/tui/model.go:1867) and `applyThemePanelCanvasMode` (internal/tui/theme_panel.go:1146) each run the identical six-step restyle on a `list.Model` — `SetDelegate(...)`, `canvas|colourlessHelpStyles`, `canvas|colourlessNoItemsStyle`, `canvas|colourlessPaginationDots`, `Styles.TitleBar` background/unset, `Styles.Title = stripListTitleColours(...)` — each behind its own `if m.colourless` fork, so every step exists six times. The session/project pair pre-existed at two copies; this feature added the third (the panel) and added the `NoItemsStyle` step to all three. Drift is already visible: the panel arm omits the shared `listTitleBarStyle(...)` geometry wrapper the other two apply, and orders the `Title` strip before the fork rather than inside each branch. This is the exact "a surface silently keeps the previous theme's cached `bubbles/list` styles" hazard the design names as the completeness risk, and it now runs on every arrow keypress in the panel.

**Solution**: Extract one helper carrying the whole sequence including the colourless fork, and reduce all three call sites to a delegate expression plus whatever genuinely differs, passed explicitly.

**Outcome**: The list-restyle sequence has exactly one home. A fourth list, or a fifth cached `bubbles/list` style, is one edit in one place. The panel/session/project differences become explicit parameters rather than incidental divergence.

**Do**:
1. In `internal/tui/model.go`, add `applyListCanvasMode(l *list.Model, delegate list.ItemDelegate, th theme.Theme, colourless bool)` containing the full six-step sequence with the `colourless` fork inside it, in the order the session/project arms currently use (title strip inside each branch).
2. Make the title-bar geometry difference explicit: either pass the `listTitleBarStyle(...)` wrapper as an argument or apply it at the call site. Do not silently change what the panel renders — determine the panel's current rendered output first and preserve it byte-for-byte; if the panel legitimately needs the wrapper, that is a separate decision, not part of this extraction.
3. Rewrite `applyCanvasMode`, `applyProjectCanvasMode` and `applyThemePanelCanvasMode` as calls to the helper, each supplying its own delegate expression.
4. Keep the panel's `if !m.themePanel.open` guard at its own call site — it is a panel concern, not part of the restyle sequence.
5. Verify no other surface performs part of this sequence inline.

**Acceptance Criteria**:
- `SetDelegate` + help/no-items/pagination/title-bar/title-strip restyle appears in exactly one function in `internal/tui`.
- All three former functions delegate to it; none retains its own `if m.colourless` fork over these steps.
- Panel, sessions and projects rendered output is unchanged (existing committed capture references and swap fixtures match).
- The panel's open-state guard remains at the panel call site, not inside the shared helper.

**Tests**:
- Existing theme swap-and-diff completeness guard and all committed panel/session/project capture fixtures stay green with no reference regeneration.
- A test that commits a theme while the panel is open and asserts no cached previous-theme colour survives in any of the three lists' rendered output (extend the existing swap guard if it already covers sessions/projects; add the panel list to its scope).

## Task 2: Single-source the theme-file authoring test helpers into `internal/themetest`
severity: high
sources: duplication

**Problem**: The "write a valid 19-token theme file" fixture format is re-implemented six times across four packages. `cmd/capturetool/main_test.go:627/638/650/658` (`themeFileLines` / `withTokenValue` / `withoutTokenLine` / `writeThemeFile`) is a verbatim copy of `internal/theme/load_test.go:613/641/653/660` (`themeLines` / `withValue` / `withoutKey` / `writeTheme`) — identical bodies, identical doc comments, identical `#abcd%02x` value generator, only the identifiers differ. Three further independent writers exist: `internal/tui/theme_panel_open_test.go:202` (`writeThemeFileForTest`), `internal/tui/theme_row_test.go:405` (`writeThemeFile`) — both in the same package, already diverging pointlessly on file mode (0o644 vs 0o600) — and `cmd/open_theme_construction_test.go:651` (`writeThemeFile`). A loader-format change has six places to follow.

**Solution**: Add a test-only helper package `internal/themetest` following the established `logtest` / `spawntest` / `transienttest` convention and collapse all six sites onto it.

**Outcome**: One definition of the theme-file fixture format. A change to the loader's file format is one edit; the two same-package `tui` writers stop disagreeing on file mode.

**Do**:
1. Create `internal/themetest` (test-only; production code must not import it, matching the `logtest` / `spawntest` convention — document that in the package doc comment).
2. Expose `Lines() []string` (a complete valid theme file: one `key = value` line per token, in the canonical table order, each carrying a distinct value), `WithValue(lines []string, key, value string) []string`, `WithoutKey(lines []string, key string) []string`, and `Write(t *testing.T, dir, base string, lines []string) string` returning the written path.
3. Pick one file mode for `Write` and use it everywhere (the stricter 0o600 unless a test asserts otherwise).
4. Replace `internal/theme/load_test.go`'s four helpers, `cmd/capturetool/main_test.go`'s four helpers, and the three standalone writers in `internal/tui/theme_panel_open_test.go`, `internal/tui/theme_row_test.go` and `cmd/open_theme_construction_test.go` with calls to the package. Delete the local definitions.
5. Scope note: this task covers theme-*file authoring* helpers only. The built-in-palette loading helpers in `internal/capture` and `internal/tui/theme_testing_test.go` are deliberately out of scope here and are handled locally elsewhere — do not move them.

**Acceptance Criteria**:
- `internal/themetest` exists with the four exported helpers and a test-only package doc.
- No package declares its own theme-file line builder, value substituter, key remover or writer.
- Exactly one file mode is used for written fixture theme files.
- No production (non-`_test.go`) file imports `internal/themetest`.

**Tests**:
- All existing tests in `internal/theme`, `internal/tui`, `cmd` and `cmd/capturetool` pass unchanged in behaviour after the swap (`go test ./...`).
- A test in `internal/themetest` asserting `Lines()` produces a file the real loader accepts, and that `WithoutKey` / `WithValue` produce exactly the missing-token / bad-value rejection classes the loader defines.

## Task 3: Strip spec-section, phase/task and design-argument citations from production comments
severity: medium
sources: standards

**Problem**: The project's code-quality standard names three things that may never appear in a comment, and this implementation carries all three at scale. (1) Spec-section citations: ~2,200 `§x.y` references across production files, the overwhelming majority added by this feature (untouched packages carry ~194 total). (2) Workflow vocabulary: ~50 `Phase N` / `task N-M` citations in the theming files — e.g. `internal/theme/resolve.go:93` "it is what task 5-7 hands over…", `internal/theme/builtins.go:28` "PRODUCTION constants rather than test-file literals because Phase 5 consumes them", `internal/capture/theme_fake.go:24` "Task 8-8's panel OPEN applies…". (3) The design argument itself, sometimes as verbatim spec quotation — `internal/tui/notice_band.go:418-424` embeds a block-quoted spec paragraph, and many doc comments re-argue rejected alternatives at length rather than stating the conclusion the code needs. The rule's own reason is load-bearing: the comment must hold for a reader with no knowledge of the process that produced the code. Once `.workflows/theming-system/` is archived, every `§9.13` and `task 8-9` is a dangling pointer. Comment density in the new files (56–71%) is measurably above the codebase norm for comparable files (37–56%).

**Solution**: Sweep the feature's production files, restating every surviving claim in its own terms and deleting the citations, the quoted spec prose and the re-argued alternatives.

**Outcome**: Every production comment in the theming surface stands on its own for a reader who has never seen the design artifacts. No comment points at a document that will not exist.

**Do**:
1. Enumerate the production (non-`_test.go`) files this feature touched — at minimum `internal/theme/*.go`, `internal/tui/theme_*.go`, `internal/tui/builtin_themes.go`, `internal/tui/notice_band.go`, `internal/tui/appearance_gate.go`, `internal/tui/model.go`, `internal/tui/build.go`, `internal/tui/theme_seams.go`, `cmd/theme*.go`, `cmd/doctor_theme.go`, `cmd/capturetool/main.go`, `internal/prefs/store.go`, `internal/capture/fixtures.go`, `internal/capture/theme_fake.go`.
2. Remove every `§x.y` token. Where the sentence needs the referent, restate it as the fact itself — "the rejection ladder short-circuits at the first failure", not "§6.2's ladder"; "the shipped fallback pair", not "§8.5's per-slot fallback".
3. Remove every `Phase N` / `task N-M` / `Task N-M` reference, rewriting the sentence around what the code does or guarantees. Delete the clause entirely if the only content was provenance.
4. Delete block-quoted spec paragraphs outright (`internal/tui/notice_band.go:418-424` is the clearest case), keeping only the conclusion the code depends on.
5. Delete comments whose only content is re-arguing a rejected alternative. Keep comments that state a live invariant, a non-obvious constraint, or a "do not do X because it breaks Y" warning — those are load-bearing and must survive the sweep verbatim in meaning (in particular the `startupCanvasHex` / OSC 11 set-back warning, the hysteresis justification block, and any "do not add/remove this" guard note).
6. Do not change any code, only comments. Pre-existing citations in files this feature did not touch are out of scope.

**Acceptance Criteria**:
- `grep -rn '§' --include='*.go'` returns no hits in the files listed in step 1.
- No `Phase [0-9]` / `task [0-9]+-[0-9]+` (any case) remains in those files.
- No block-quoted spec prose remains; `internal/tui/notice_band.go` states its conclusion in its own words.
- The load-bearing warnings named in step 5 survive with their meaning intact.
- No production code changed — the diff is comments only.

**Tests**:
- `go build ./...` and the full unit lane (`go test ./...`) stay green; the integration lane is unaffected (comment-only diff).
- Existing source-guard tests (colour-literal guard, `canvasHexFor` absence guard, log-ownership guard) stay green.

## Task 4: Add the missing panel → persister → prefs.json commit round trip and drop the AST wiring guards
severity: medium
sources: architecture

**Problem**: The feature's central user-visible promise — press `Enter`/`d`/`l` in the panel, the choice lands in `prefs.json`, the next launch renders it — is verified in three disjoint pieces that never meet. `internal/tui` drives the commit keys against a fake `ThemePersister`; `cmd/theme_persister_test.go:287` calls `themePersister.CommitTheme` / `CommitThemeSlot` directly against a real store; the join between them is asserted *syntactically* by two AST tests (`TestOpenTUI_ThemePersisterWiredOnlyWithAStore` counts `cfg.themePersister = …` assignments inside an `if prefsStore != nil` block; `TestOpenTUI_ThreadsThePanelConstructorSlots` counts `cfg.themeKeys` / `cfg.themeEnumerator` assignments). Nothing constructs a model with the real persister and asserts what reaches disk. The composed path crosses four packages and three type translations (`themeRowItem.Row.Slug` → `prefs.ThemeSlot` → `prefsFile.ThemeLight/ThemeDark` → `theme.RawKeys` on the next launch) and the AST guards prove none of it — they prove an assignment exists, and they break on any ordinary refactor (renaming the local `prefsStore`, extracting the wiring into a helper) without the behaviour having changed. `TestThemePanelOpen_WiredThroughBuildTUIModel` shows the pattern that is missing for the write half.

**Solution**: Add one behavioural test in `cmd` that wires the real persister and real enumerator into `buildTUIModel` against a temp prefs file, drives the commit keys, and asserts both what reaches disk and what would render on relaunch — then delete the two AST wiring guards it subsumes.

**Outcome**: The commit promise is proven end to end by behaviour. The two AST guards, which break on refactors and prove nothing about the data path, are gone rather than maintained.

**Do**:
1. In `cmd`, add a test that creates a temp prefs file and a temp themes directory, constructs `newThemePersister(store)` and `newThemeEnumerator(loader)`, and wires both into `buildTUIModel` the way production does.
2. Drive the model: send `t` to open the panel, move the cursor to a known row, then send `Enter`; read `prefs.json` back and assert the persisted keys.
3. Add a second case (same test or a sibling) sending `d` on an adaptive/constant row and asserting the per-slot key that lands.
4. Feed the persisted keys back through `themeResolution` and assert the theme that would render on the next launch is the one chosen.
5. Delete `TestOpenTUI_ThemePersisterWiredOnlyWithAStore` and `TestOpenTUI_ThreadsThePanelConstructorSlots` once the behavioural test covers the wiring they stood in for. Keep any AST guard that proves something the behavioural test cannot (e.g. the nil-store carve-out) — if the nil-store case is not covered behaviourally, add a case that builds with no prefs store and asserts the panel commits nothing rather than keeping the AST assertion.
6. Follow the package's test rules: no `t.Parallel()`, inject every tmux-touching `*Deps` seam, use a temp dir for prefs (never the developer's config path).

**Acceptance Criteria**:
- One test in `cmd` drives `t` → `Enter` (and `d`) through a model built by `buildTUIModel` with the real persister and enumerator, and asserts the resulting `prefs.json` contents.
- The same test asserts the theme resolved from those persisted keys on a simulated relaunch.
- Both named AST wiring guards are deleted; the nil-prefs-store carve-out remains covered by a behavioural assertion.
- The test touches no real config path and spawns no tmux server.

**Tests**:
- The new round-trip test above (commit via `Enter`, commit via `d`, relaunch resolution, nil-store no-op).
- Full `cmd` package suite stays green after the guard deletions.

## Task 5: Split `theme.Loader`'s panel-assembly responsibility and single-source the enumerator adapter
severity: medium
sources: architecture, duplication

**Problem**: `theme.Loader` (internal/theme/load.go:19) exposes ten methods spanning four distinct jobs: file parse/validate (`LoadFile`, `LoadPath`, `LoadBuiltin`), directory enumeration (`Enumerate`), panel row-model assembly (`Open`, `Reassemble` — building `Row`/`Union`/ordering/labels, which loads nothing), and setting resolution (`ResolveByName`, `ResolveNomination`, `ResolveNominationFrom`, `ResolveSlot`). Only the first matches the type's name. Every consumer holds the whole surface for a slice of it (`cmd/theme.go` one method, `internal/tui/builtin_themes.go` one, `cmd/capturetool` two, `cmd/doctor_theme.go` two, the panel adapter four), and the accretion is why `LoadPath`'s receiver is unused and documented as such. Compounding it, the four-method loader delegation that satisfies `tui.ThemeEnumerator` is hand-written three times: the production `cmd/theme_enumerator.go:28` adapter plus two test re-implementations, `internal/tui/theme_panel_open_test.go:77` (`realThemeEnumerator`) and `internal/tui/theme_seams_test.go:49` (`loaderThemeEnumerator`) — so the adapter that must not drift exists in three places, one unreachable from the others by import.

**Solution**: Leave parse/enumerate/resolve on `Loader`; move the panel row-model assembly onto its own value constructed from a `Loader`, and export the four-method enumerator adapter from `internal/theme` so production and both tests share one definition.

**Outcome**: `tui.ThemeEnumerator` maps one-to-one onto a single exported type. Doctor, export and capturetool stop holding methods they must not call. The adapter has one home instead of three.

**Do**:
1. In `internal/theme`, introduce a value that owns `Open` / `Reassemble` and the `Row` / `Union` / badge assembly, constructed from a `Loader` (the loader stays the parse/enumerate/resolve type). Keep the package boundary — the assembly stays in `internal/theme`.
2. Export an adapter satisfying the panel seam's four methods (`Open`, `Reassemble`, `ResolveNominationFrom`, `ResolveSlot`) — e.g. `theme.DirEnumerator{Loader, Dir}` — composing the loader and the new assembly value.
3. Have `cmd.newThemeEnumerator` return that exported adapter instead of the hand-written `cmd/theme_enumerator.go` struct; delete the hand-written struct.
4. Delete `realThemeEnumerator` (internal/tui/theme_panel_open_test.go:77) and `loaderThemeEnumerator` (internal/tui/theme_seams_test.go:49) and use the exported adapter in both tests.
5. Update the remaining consumers (`cmd/theme.go`, `cmd/doctor_theme.go`, `cmd/capturetool/main.go`, `internal/tui/builtin_themes.go`) to hold only the type they need.
6. Resolve `LoadPath`'s unused receiver as part of the split — make it a function or give it a real receiver; delete the "it stays a method so a caller reaches every entry point through the one Loader" comment either way.

**Acceptance Criteria**:
- `Loader` no longer exposes `Open` / `Reassemble`; the panel row-model assembly lives on its own type constructed from a `Loader`.
- Exactly one type in the repo implements the four-method `tui.ThemeEnumerator` delegation over a loader, and it is exported from `internal/theme`.
- `cmd/theme_enumerator.go`'s hand-written adapter and both test re-implementations are deleted.
- No consumer holds methods it does not call (doctor, export and capturetool each take the narrower type).
- `LoadPath` has no unused receiver.

**Tests**:
- Existing `internal/theme` loader, union and resolution suites pass unchanged.
- `TestThemePanelOpen_WiredThroughBuildTUIModel` and the seam tests pass against the exported adapter.
- A test asserting the exported adapter satisfies `tui.ThemeEnumerator` (compile-time assertion is sufficient) so the seam cannot silently drift from the production wiring.

## Task 6: Single-source the light/dark slot vocabulary and site the badge glyphs with the panel copy
severity: medium
sources: duplication, architecture

**Problem**: The `light` / `dark` word is defined in three places and the badge glyphs in a fourth, misplaced one. `theme.slotAttr` (internal/theme/events.go:267) and `cmd.themeSlotAttr` (cmd/theme_persister.go:109) are the same function — a light/dark switch returning the attr name plus a "named" bool — differing only in which of the two parallel slot enums they take (`theme.Slot` vs `prefs.ThemeSlot`); both feed the same `theme` log component's `slot` attr, so the vocabulary a reader greps for is defined twice. `cmd/doctor_theme.go:69-73` then declares `themeSlotLight` / `themeSlotDark` / `themeSlotBoth` as a third literal set. Separately, every user-facing string the panel renders is a pinned constant in `internal/tui` (`Themes`, `⚠ dir unreadable`, `clear constant %s?  y / n`, `⚠ couldn't save theme`, the flashes, the footer labels) *except* the badge vocabulary (`●`, `● light`, `● dark`, `● both`), which lives in the leaf `internal/theme` behind `Badge.Text()` for exactly one consumer — nothing outside the panel renders a badge (doctor renders its own slot words, the log component renders its own again).

**Solution**: Give `theme.Slot` one exported name mapping that the log path, the persister path and doctor all source from, and move the four badge glyph strings into the panel's copy block while leaving the badge *derivation* in `internal/theme`.

**Outcome**: One definition of the light/dark attr vocabulary, and one answer to "where does the panel's copy live?" — shared-across-surfaces copy in `theme`, panel-only copy in `tui`.

**Do**:
1. Add an exported name mapping on `theme.Slot` (e.g. `func (s Slot) AttrName() (string, bool)`) carrying the light/dark switch and the named bool.
2. Rewrite `theme.slotAttr` to delegate to it.
3. Rewrite `cmd.themeSlotAttr` to convert `prefs.ThemeSlot` → `theme.Slot` and delegate to the same mapping. Keep `prefs.ThemeSlot` — `prefs` is a deliberate no-logging leaf and must not import `internal/theme`; the conversion belongs at the `cmd` boundary.
4. Source doctor's `themeSlotLight` / `themeSlotDark` (cmd/doctor_theme.go:69-73) from the same mapping rather than from fresh literals. `themeSlotBoth` has no slot to map from — leave it as a doctor-local literal but site it beside the derived pair so the relationship is visible.
5. Move the four badge glyph strings out of `internal/theme/badge.go:56,66` into `internal/tui`'s panel copy block beside the header label and message-slot constants, rendering the enum at the row delegate.
6. Keep `theme.Badge` and `theme.Badges()` — the derivation is a fact about the setting, not a rendering concern.

**Acceptance Criteria**:
- One function maps a slot to its `light` / `dark` attr name; `theme.slotAttr`, `cmd.themeSlotAttr` and doctor's constants all reach it.
- `prefs.ThemeSlot` still exists and `internal/prefs` imports neither `internal/theme` nor `internal/log`.
- The strings `● light`, `● dark`, `● both` and the bare `●` badge appear only in `internal/tui`'s panel copy block.
- `theme.Badge` / `theme.Badges()` still derive the badge; only the glyph text moved.
- The emitted `theme` log lines and doctor's rendered slot words are unchanged.

**Tests**:
- Existing log-catalogue tests asserting the `slot` attr values pass unchanged for both the theme-side and persister-side emitters.
- Existing doctor output tests pass unchanged.
- Panel capture fixtures render byte-identical badges (committed references unchanged).

## Task 7: Type the light/dark selector at the `Nomination` boundary
severity: medium
sources: architecture

**Problem**: The light/dark axis is represented by `tui.canvasAppearance`, `theme.Slot`, `prefs.ThemeSlot` and `theme.Badge`, connected by seven hand-written mappings (`inForceSlot`, `oppositeThemeSlot`, `fallbackSlugFor`, `slotBadge`, `slotAttr`, `themeSlotAttr`, `persistedSlotSlug`) plus `joinNomination`'s positional light/dark ordering. The `prefs` copy is genuinely forced (leaf package), but `canvasAppearance` and `theme.Slot` are visible from the same package and the seam between them throws the type away entirely: `Nomination.Select(dark bool)` (internal/theme/nomination.go:98) takes an untyped boolean, so `m.nomination.Select(m.canvasMode == appearanceDarkCanvas)` flattens a named enum into a bool the callee immediately re-interprets. That boolean parameter sits on an exported cross-package API where the concrete type is known at design time; `joinNomination(newlyLive, loaded, assigned)` compounds it by relying on argument *position* to say which palette is light. `theme.Slot` carries a third value (`SlotConstant`) meaningless as a gate answer, which is why the bool was reached for — but the fix is a narrower type, not no type.

**Solution**: Give `Nomination` a typed selector and have `AdaptivePair` / `joinNomination` take that same type instead of relying on argument position, so the `canvasAppearance → theme.Slot` conversion happens once at the gate.

**Outcome**: No boolean parameter on the exported selection API, no positional light-then-dark contract, and one conversion point between the TUI's appearance enum and the theme package's slot type.

**Do**:
1. Change `Nomination.Select` to take a typed selector rather than `dark bool` — either `theme.Slot` (treating `SlotConstant` as "the constant", which `Select` already does implicitly) or a two-valued member type exported alongside `Nomination`. Pick one and use it consistently.
2. Change `AdaptivePair` / `joinNomination` (internal/theme) to take that same type instead of relying on positional light-then-dark ordering.
3. Update `internal/tui/theme_panel.go:806` (`inForceSlot`) so the `canvasAppearance → theme.Slot` mapping is a single conversion at the appearance gate rather than a rule restated per call site.
4. Update the model call site that currently passes `m.canvasMode == appearanceDarkCanvas` and any other `Select` caller (including capture fixtures) to pass the typed value.
5. Leave `prefs.ThemeSlot` alone — the leaf constraint is real.

**Acceptance Criteria**:
- `Nomination.Select` takes no boolean parameter.
- `AdaptivePair` / `joinNomination` identify light and dark by type, not argument position.
- Exactly one site converts `canvasAppearance` to the theme-side slot type.
- Resolution behaviour is unchanged for all three slot values, including the constant case.

**Tests**:
- Existing nomination/resolution unit tests pass unchanged in outcome, updated only for the new signature.
- A test that swapping the two arguments of `joinNomination` (or its replacement) is now a compile error or produces a distinguishable value — i.e. light/dark cannot be silently transposed.
- Panel and appearance-gate tests covering auto/light/dark pinned appearance still resolve the same themes.

## Task 8: Extract the shared key-column row used by the panel footer and the help modal
severity: medium
sources: duplication

**Problem**: `themePanelFooterRow` (internal/tui/theme_panel_footer.go:107) and `helpModalRow` (internal/tui/help_modal.go:157) compose the same row by the same algorithm: render `helpKeyGlyph(e)` in an accent token, measure it, pad the key column to a fixed width with `headerCanvasBg(...)`, render a canvas-backed gap, render the action label in a text token, and `JoinHorizontal` the four segments. The bodies differ only in the key token (destructive-aware vs always `AccentKey`), boldness, label token, column-width constant and a trailing `headerPadRight`. The copy relationship is acknowledged in-source (theme_panel_footer.go:35 references `helpKeyColumnWidth`) — the copy-paste-with-a-comment shape that drifts.

**Solution**: Extract one row builder parameterised on the tokens, column width and gap; both callers supply their own.

**Outcome**: The key-column row geometry has one implementation, so a padding or measurement fix lands in both surfaces at once.

**Do**:
1. Add `keyColumnRow(glyph, label string, keyTok, labelTok theme.Token, columnWidth int, gap string, th theme.Theme, colourless bool) string` beside `renderKeyHint` in `internal/tui/modal_footer.go` (or in `help_modal.go` if that is the closer home).
2. Move the measure/pad/gap/join body into it verbatim, keeping the existing measurement helper and canvas-background calls.
3. Rewrite `helpModalRow` to call it, resolving its destructive-aware key token and boldness at the call site before the call.
4. Rewrite `themePanelFooterRow` to call it, keeping its `headerPadRight` wrap at its own call site.
5. Delete the theme_panel_footer.go:35 comment that points at `helpKeyColumnWidth` as a copied constant — the width is now an argument.

**Acceptance Criteria**:
- One function builds the key-column row; both callers delegate.
- Both surfaces render byte-identical output to before (committed help-modal and panel capture references unchanged).
- The destructive-aware key token and the boldness difference are resolved at the help-modal call site, not by a branch inside the shared builder.

**Tests**:
- Existing help-modal and theme-panel footer render tests pass with no expectation changes.
- Committed capture fixtures for the help modal and all panel fixtures match their references without regeneration.

## Task 9: Collapse the two duplicate `ThemeEnumerator` fakes in package `tui` into one configurable fake
severity: medium
sources: duplication

**Problem**: `stubThemeEnumerator` (internal/tui/theme_panel_cursor_test.go:77) and `splitThemeEnumerator` (internal/tui/theme_panel_commit_recompute_test.go:487) are two types in the same package with the same ~15-line `ResolveSlot` body — record the ask, error-out, scan `resolution.Slots` for the slot, else synthesise from `Nomination.Select(slot == SlotDark)` — and near-identical `Open` / `Reassemble` / `Resolve` arms; the second's comment even says "see stubThemeEnumerator.ResolveSlot". The recompute suite's split-union case is a configuration difference, not a second type.

**Solution**: One configurable fake in a shared test file, with the split-union case expressed as a field.

**Outcome**: The panel seam has one test double in package `tui`; a seam-contract change is one edit.

**Do**:
1. Add a shared test file in `internal/tui` (e.g. `theme_enumerator_fake_test.go`) declaring one fake with fields for the opened union, the reassembled union, the resolution, an injectable error, and the recorded asks.
2. Move the `ResolveSlot` body (record → error → scan `resolution.Slots` → synthesise from the nomination) into it once.
3. Express the recompute suite's split-union behaviour as a field on the fake (a distinct reassembled union), not a second type.
4. Update all call sites in `theme_panel_cursor_test.go`, `theme_panel_commit_recompute_test.go` and any other consumer, and delete both original types.
5. Scope note: the two *production-adapter* re-implementations (`realThemeEnumerator`, `loaderThemeEnumerator`) are handled separately — do not touch them here.

**Acceptance Criteria**:
- Package `tui` declares exactly one `ThemeEnumerator` fake.
- The split-union recompute case is driven by a field, not a distinct type.
- The recorded-asks assertions in the cursor and recompute suites still assert the same asks.
- No `see <other type>.ResolveSlot` style cross-reference comments remain.

**Tests**:
- The full `internal/tui` panel suite (cursor, commit, recompute, open) passes unchanged in outcome.

## Task 10: Single-source the silent theme loader and the shipped dark default slug
severity: medium
sources: duplication, standards

**Problem**: Two single-sourcing gaps in the same surface. (1) `theme.NewLoader(theme.NewEventLogger(log.Discard()))` — the "diagnose without emitting" loader — is written out identically at `cmd/theme.go:120`, `cmd/doctor_theme.go:112` and `cmd/capturetool/main.go:161`; compounding it, two packages each define a `newThemeLoader()` with different bodies (`cmd/open.go:760` emits through `themeLogger`; `cmd/capturetool/main.go:160` discards), so the name no longer identifies the behaviour. (2) `cmd/capturetool/main.go:48` declares `const defaultThemeSlug = "tokyo-night"` while `theme.DefaultDarkSlug` exists as the single definition of the shipped pair (`internal/theme/builtins.go:35`; the same file derives `BuiltinSlugs()` rather than restating it, on exactly this reasoning). Every unflagged capture depends on that default, and a change to the shipped dark default would leave captures rendering a palette the product no longer defaults to with nothing failing.

**Solution**: Add one constructor for the silent loader and route all three sites through it; set capturetool's default slug from the exported constant.

**Outcome**: The silent-loader shape has one definition, the `newThemeLoader` name stops meaning two things, and the capture default follows the shipped dark default automatically.

**Do**:
1. Add `theme.NewSilentLoader()` in `internal/theme`, building the loader over `log.Discard()`. Confirm first that `internal/theme` may import `internal/log` (the only stated prohibition on `internal/log` is that it must not import `internal/state`); it must use `log.Discard()` rather than constructing a `*slog.Logger`, which the log-ownership guard forbids.
2. Route `cmd/theme.go:120`, `cmd/doctor_theme.go:112` and `cmd/capturetool/main.go:161` through it.
3. Rename capturetool's `newThemeLoader` to name what it builds (or delete it if it becomes a one-line call), so the two packages no longer disagree on what `newThemeLoader` means. Leave `cmd/open.go:760`'s emitting constructor as the only `newThemeLoader`.
4. Set `cmd/capturetool/main.go:48`'s `defaultThemeSlug` from `theme.DefaultDarkSlug` instead of the `"tokyo-night"` literal.
5. Leave the neighbouring `themeFileExtension` restatement alone — it is justified in-source because `internal/theme` keeps its extension unexported.

**Acceptance Criteria**:
- `theme.NewLoader(theme.NewEventLogger(log.Discard()))` is written exactly once in the repo.
- No two packages define `newThemeLoader` with different behaviour.
- `capturetool` has no `"tokyo-night"` literal; its default flows from `theme.DefaultDarkSlug`.
- Doctor, export and capturetool still emit no `theme` log lines on their diagnose paths.

**Tests**:
- A test asserting the silent loader emits nothing through the `theme` log component while still returning the same rejection reasons as the emitting loader (extend the existing doctor/export silence assertions if present).
- A capturetool test asserting the no-flag default resolves to `theme.DefaultDarkSlug`, so a change to the shipped default cannot silently repoint every unflagged capture.

## Task 11: De-duplicate `cmd`'s invalid-theme fixture builders
severity: medium
sources: duplication

**Problem**: Package `cmd` builds each invalid-theme fixture class twice. `cmd/doctor_theme_test.go:98/112/131` (`sourceMissingTokens` / `sourceBadColours` / `sourceDuplicateKeyAt`) and `cmd/theme_test.go:647/634/624` (`missingTokenSource` / `badColourSource` / `duplicateKeySource`) produce the same three rejection classes from the same `themeKeyLines` / `themeSourceFromLines` primitives. Each pair differs only in parameterisation — the doctor set takes the key(s)/line to break, the export set hardcodes "the first key" / "drop line 0". A change to what makes a file `bad colour` has to land in two families that already share their inputs.

**Solution**: Keep the parameterised trio, site it beside the shared primitives, and delete the hardcoded trio.

**Outcome**: One builder per rejection class in `cmd`, parameterised, beside the primitives it composes.

**Do**:
1. Move `sourceMissingTokens`, `sourceBadColours` and `sourceDuplicateKeyAt` beside `themeKeyLines` / `themeSourceFromLines` in `cmd/theme_test.go`.
2. Rewrite the export-side call sites to call the parameterised forms with the keys/line they want (first key, line 0, etc.).
3. Delete `missingTokenSource`, `badColourSource` and `duplicateKeySource`.
4. Leave the doctor call sites untouched apart from the file move.

**Acceptance Criteria**:
- Package `cmd` declares one builder per rejection class (missing tokens / bad colours / duplicate key), each parameterised.
- The three hardcoded builders are deleted.
- Doctor and export tests assert the same rejection reasons on the same fixture shapes as before.

**Tests**:
- `cmd/doctor_theme_test.go` and `cmd/theme_test.go` suites pass with unchanged assertions.

## Task 12: Make the docs example theme provably equal to the shipped dark built-in
severity: medium
sources: duplication

**Problem**: `docs/theming.md:90-124` reproduces all 19 values of `internal/theme/builtins/tokyo-night.theme` byte-for-byte (only the header comment differs) and asserts the identity in prose — "These are the values of Portal's dark built-in." The guard `TestThemingDocExampleThemeIsValid` (internal/theme/docs_guard_test.go:144) only checks the example parses as a *valid* theme, not that it still equals the built-in. Any value that moves in the `.theme` file — and re-derivation of those values is an explicitly contemplated change — leaves the doc silently claiming to be the built-in while showing different colours.

**Solution**: Strengthen the guard so the doc block must equal the embedded built-in source, modulo the header comment.

**Outcome**: The doc cannot silently start lying about the built-in; a palette change in the `.theme` file fails the guard until the doc follows.

**Do**:
1. Extend `TestThemingDocExampleThemeIsValid` (or add a sibling guard) to read the doc's fenced example block and compare it against the embedded dark built-in source — `theme.BuiltinBytes(theme.DefaultDarkSlug)` if that accessor exists, otherwise the existing embedded-source accessor the loader uses for built-ins; export a minimal read-only accessor if none is reachable from the test.
2. Compare modulo the header comment only: normalise away the leading comment line(s) on both sides and compare the remaining `key = value` lines exactly (including order).
3. Keep the existing validity assertion — the guard should prove both "parses" and "is the built-in".
4. Do not change the doc's values or prose; the doc is currently correct.

**Acceptance Criteria**:
- A guard test fails if any token value in `tokyo-night.theme` changes without the doc block following.
- The guard tolerates the differing header comment and nothing else.
- The existing "example parses as a valid theme" assertion survives.
- `docs/theming.md` is unchanged by this task.

**Tests**:
- The strengthened guard, plus a negative case proving it fails on a mutated value (mutate an in-memory copy of one side and assert the comparison rejects it).

## Task 13: De-duplicate the capture fixture and test-harness helpers
severity: low
sources: duplication

**Problem**: Two clusters of restated helpers in `internal/capture`. (1) In package `capture_test`: `builtinPalette(t, slug)` (theme_panel_fixture_render_test.go:375), `darkBuiltinTheme(t)` and `lightBuiltinTheme(t)` (swap_harness_test.go:144/208) are the same ten-line "LoadBuiltin, Fatal on not-found, Fatal on rejection" body with the slug varying; `bgSeq` (swap_harness_test.go:223) and `fgSeq` (theme_panel_remaining_fixtures_test.go:186) are the same eight-line SGR probe as the general `sgrParameterRun` (theme_swap_guard_test.go:212), specialised to Background/Foreground; `panelFixtureFrame` (theme_panel_fixture_render_test.go:90) and `panelFrameAt` (theme_panel_remaining_fixtures_test.go:108) are the same body, one hardcoding the harness size — its doc even opens "It exists beside panelFixtureFrame". (2) In `internal/capture/fixtures.go`: `themePanelEnumeration` (:762) hand-assembles the `Entry`+`Enumeration` shape that `themePanelDirEntry` (:995) / `themePanelDirEnumeration` (:1001) exist to build, and re-writes the themes-directory path as a string literal twice although `themesDirPath` (:985) is declared for exactly that ("shared so no fixture invents a second, different-looking one"); `themePanelPaginatedFixture` (:1234) repeats `themePanelAdaptivePairFixture`'s (:804) keys, two-record `themeSlots`, cursor and `captureKeys` verbatim, differing only in union and enumeration, while every other derived panel fixture correctly starts from a base fixture and overrides.

**Solution**: Reduce each restated helper to a one-line call to its general form, and build the derived fixture from its base.

**Outcome**: The capture harness has one built-in loader, one SGR probe, one panel-frame renderer, one themes-dir path and one adaptive-pair fixture body.

**Do**:
1. Keep `builtinPalette(t, slug)`; redefine `darkBuiltinTheme(t)` and `lightBuiltinTheme(t)` as one-line calls to it with the shipped dark/light slugs.
2. Redefine `bgSeq` and `fgSeq` as one-line calls to `sgrParameterRun` with the appropriate style; delete their bodies.
3. Redefine `panelFixtureFrame(t, fixture, palette)` as `panelFrameAt(t, fixture, palette, harnessWidth, harnessHeight)`; delete the duplicated body and the "It exists beside panelFixtureFrame" doc line.
4. Redefine `themePanelEnumeration` as `themePanelDirEnumeration(themePanelDirEntry(themePanelDropInSlug+".theme", themePanelDropInSlug))`.
5. Delete both themes-directory path string literals in favour of `themesDirPath`.
6. Rebuild `themePanelPaginatedFixture` from `themePanelAdaptivePairFixture()`, overriding only `name`, `themeUnion` and `themeEnumeration`, matching how the narrow / commit-failed / min-height fixtures are already derived.
7. Scope note: the *cross-package* built-in loader copies (`internal/capture/fixture_test.go:725`, `internal/tui/theme_testing_test.go:36`) stay where they are — this task is intra-package only.

**Acceptance Criteria**:
- `capture_test` has one "load a built-in or Fatal" body, one SGR-parameter probe body and one panel-frame renderer body.
- No string literal for the themes directory path remains in `fixtures.go`.
- `themePanelPaginatedFixture` is derived from `themePanelAdaptivePairFixture()` and states only its three differences.
- All nine panel fixtures render byte-identical to their committed references.

**Tests**:
- The full `internal/capture` suite passes, including the swap guard and every panel fixture render test, with no reference regeneration.

## Task 14: Collapse the remaining two-copy sequences in production code
severity: low
sources: duplication

**Problem**: Three places where a short sequence is written twice instead of shared. (1) `internal/prefs/store.go:212` (`readFile`) and `:264` (`readFileStrict`) share an identical eight-line prologue — `os.ReadFile`, ENOENT → `(zero, false, nil)`, other error → `(zero, false, err)`, then `json.Unmarshal` into a `prefsFile` — and differ only in the decode-error policy, which is load-bearing and must stay different; the shared half being copied means an ENOENT/permission policy change has two homes. (2) `themePanelMinHeight` (internal/tui/theme_panel.go:279) and `themePanelListSize` (:1459) both sum `themePanelHeaderRows() + themePanelDirRowHeight(...) + <message rows> + themePanelFooterHeight(...)`; the components are single-sourced individually but the *set* is not, and a floor that stops matching the render is exactly the "opens a broken frame" failure the floor's own doc forbids. (3) `setSuccessFlash` (internal/tui/model.go:2530) repeats all four of `setFlash`'s statements (`flashGen++`, `flashText`, `flashOrigin = flashOriginDefault`, `resyncPageLayouts()`) to change one field, while its sibling `setThemeFlash` (:2519) correctly composes — and this feature added the new `flashOrigin` field to both copies, which is the drift the copy invites.

**Solution**: Extract the shared half in each case, leaving only the genuine difference at each call site.

**Outcome**: The prefs read policy, the panel chrome component set, and the flash-raising sequence each have one home.

**Do**:
1. In `internal/prefs/store.go`, extract `func (s *Store) readBytes() ([]byte, bool, error)` carrying the read and its ENOENT branch; have both `readFile` and `readFileStrict` start from it and keep their own `json.Unmarshal` error policy — the tolerant/strict split must remain intact and is the only thing that may differ.
2. In `internal/tui/theme_panel.go`, extract `themePanelChromeRows(dirUnusable bool, messageRows int, footer []keymapEntry) int` returning the sum; have `themePanelMinHeight` call it with the floor's fixed message rows and standing scope, and `themePanelListSize` call it with the measured message height and live scope.
3. In `internal/tui/model.go`, redefine `setSuccessFlash` as `m.setFlash(text)` followed by its one field assignment, matching `setThemeFlash`'s shape.

**Acceptance Criteria**:
- `internal/prefs` reads the file and handles ENOENT in exactly one place; tolerant and strict decode policies are unchanged and still distinguishable.
- One function sums the panel chrome rows; both the floor and the body arithmetic call it.
- `setSuccessFlash` composes `setFlash` and assigns only the field that differs.
- No behaviour change in any of the three: same prefs decode outcomes, same panel floor and list height, same flash state.

**Tests**:
- Existing prefs tolerant-decode and strict-decode tests pass unchanged (including corrupt-file, missing-file and permission cases).
- Existing panel minimum-height / refuse-to-open tests and the min-height capture fixture pass unchanged.
- Existing flash tests (success flash, theme flash, flash origin) pass unchanged.

## Task 15: Make the theme seam gating and model-level theme state consistent with their siblings
severity: low
sources: architecture

**Problem**: Two consistency gaps in the TUI wiring. (1) `WithThemeEnumerator` routes through `liveThemeEnumerator` (internal/tui/theme_seams.go:79), which uses `reflect` to reject an interface holding a typed nil pointer, while every other optional seam in `Deps` — `Enumerator`, `Reader`, `PreviewAttacher`, `ModePersister`, `ThemePersister`, `Detector`, `AckChannel` — is gated by a plain `if deps.X != nil` in `Build` (internal/tui/build.go:282) and carries the identical exposure with no guard; neither production (`cmd/open.go` passes a struct value) nor any fixture can produce the shape being defended against. A reader now cannot tell whether the plain gates on the other seams are an oversight or a decision. (2) The panel's own state was correctly grouped into the `themePanel` struct, but the model-level theme state was not: `themePersister`, `themeCommitFailed`, `themeKeys`, `themeEnumerator`, `nomination`, `canvasMode`, `activeTheme`, `gate`, `startupCanvasHex`, `bgReplyArrived`, `bgReplyDark`, `flashOrigin` and three `initialTheme*` capture seeds are sixteen sibling fields spread across a 5,700-line `model.go`, with the invariants that bind them documented only in prose on individual fields. `Deps` has correspondingly grown to 42 fields, nine of them capture-harness-only seeds, so the production wiring struct and the fixture-authoring struct are the same type with no separation.

**Solution**: Gate `ThemeEnumerator` the way its siblings are gated, group the model-level theme state into a struct beside `themePanel`, and group the capture-only seeds into a nested `Deps.Capture`.

**Outcome**: One gating idiom across all optional seams; the theme state's invariants stated once on a type; the production seam and the fixture seam visibly distinct.

**Do**:
1. Delete the `reflect` use in `internal/tui/theme_seams.go:79` and gate `ThemeEnumerator` with a plain `if deps.ThemeEnumerator != nil` in `Build`, matching its siblings. Leave `openThemePanel`'s existing nil check as the runtime guard.
2. Group the model-level theme fields listed above into a struct field on `Model` beside `themePanel`, and state the invariants that bind them once on that type: `canvasMode` deliberately diverges from `gate.appearance` after a mid-session conversion, and `startupCanvasHex` deliberately does *not* move with `activeTheme`.
3. Preserve the `startupCanvasHex` contract exactly — it must stay the retained startup value and must never be re-derived from the active theme; the source guard keeping `canvasHexFor` deleted must stay green.
4. Group the nine `Initial*` capture-harness seeds into a nested `Deps.Capture` struct and update `internal/capture` fixture authoring accordingly.
5. Mechanical only — no behaviour change in gating, canvas selection, restore-on-exit or fixture rendering.

**Acceptance Criteria**:
- `internal/tui` uses no `reflect` for seam gating; all optional `Deps` seams use the same plain nil gate.
- The model-level theme state is one grouped struct whose type doc states the `canvasMode`/`gate.appearance` divergence and the `startupCanvasHex` retention rule.
- `Deps` exposes the capture seeds under a nested struct, visibly separate from production wiring fields.
- The OSC 11 set-back behaviour (canvas-echo guard, `NO_COLOR` carve-out) is unchanged.

**Tests**:
- Existing seam tests covering a nil/absent `ThemeEnumerator` still assert the panel refuses to open rather than panicking.
- Existing restore-on-exit tests (canvas-echo guard, `NO_COLOR` no-write, mid-session theme commit, quit with uncommitted preview) pass unchanged.
- The `canvasHexFor`-absence source guard and all capture fixtures pass unchanged.
