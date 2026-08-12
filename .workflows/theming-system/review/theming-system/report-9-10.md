TASK: theming-system-9-10 — `keymap_dispatch_guard_test` Covers The Panel And Confirm Scopes

ACCEPTANCE CRITERIA:
- `assertDescriptorDispatchParity` runs over `themePanelKeymap()` with a probe for every one of the six entries, all honouring their bound effect.
- Removing any panel dispatch arm (arrows, paging, `Enter`, `d`, `l`, `Esc`) makes the corresponding probe fail.
- Adding a seventh descriptor entry with no probe fails the descriptor-coverage direction.
- The confirm suite probes `y` and `n` and additionally asserts `Y` and `N` reach the same arms.
- The panel `esc` probe asserts a close; the confirm `Esc` assertion asserts a cancel with the panel still open.
- Every probe asserts a state change, not consumption — asserted by construction and stated in the suite's doc comment.
- No probe writes to a real prefs file or reads a real themes directory.
- The panel scope has no `RightAligned` entry and the `?`-help allow-list is unchanged.
- `?` has no descriptor entry and no probe in either scope.
- `Ctrl-C` is not a descriptor entry in either scope.
- The three page descriptors and their probe maps are byte-identical before and after this task.
- Both suites are unit-lane and carry no `t.Parallel()`.

STATUS: complete

SPEC CONTEXT:
§9.12 (specification.md:1222-1230) pins the panel scope as "all six" keys — `↑`/`↓`, `Ctrl+↑`/`Ctrl+↓`, `Enter`, `d`, `l`, `Esc` — with the footer rendering only the four `Core` entries, and states "`keymap_dispatch_guard_test` covers them" plus "`?` does nothing inside the panel" (swallowed, no help modal, no descriptor entry). §9.2 (line 1042) makes the confirm a *second, nested* scope whose `y`/`Y`/`n`/`N` the same guard must cover, explicitly clarifying that "all six" is the panel scope's own membership rather than a six-plus-four list. §13.6 (line 1747) records the guard as "Extended to cover the panel scope (§9.12)". The existing guard's contract (keymap_dispatch_guard_test.go:24-51) is two-way parity with a single allow-list: `RightAligned` entries are skipped, derived from the flag rather than a hand-listed glyph.

IMPLEMENTATION:
- Status: Implemented
- Location: `/Users/leeovery/Code/portal/internal/tui/keymap_dispatch_guard_theme_test.go` (new, 454 lines). Descriptors under test: `/Users/leeovery/Code/portal/internal/tui/keymap.go:62-82`. Live dispatch: `/Users/leeovery/Code/portal/internal/tui/theme_panel.go:314-344` and `/Users/leeovery/Code/portal/internal/tui/theme_panel_confirm.go:39-50`.
- Notes:
  - Both suites live in a **new file** alongside the shared guard rather than inside it, so `keymap_dispatch_guard_test.go`'s three page probe maps are structurally untouched by this task (the `t` probes and `themeGuardModel` there belong to an earlier phase that added the `t` descriptor entries).
  - Panel seed (`themePanelGuardModel`, :23-45) uses `commitPairPanelDeps` → `stubPanelDeps` with a `*fakeThemeSource` and an injected `*fakeThemePersister`; rows are 4 valid + 1 invalid (`themePanelGuardRows`, :18-21), matching "at least two selectable rows and one invalid row". `arrowPagingTermH`/`arrowPagingPerPage` give the paging probe somewhere to move, guarded by a `requireArrowPanelPageSize` fixture Fatal. The `colourless` and `open` preconditions are both fixture-Fatal'd, so a seed that silently stopped exercising the panel fails loudly rather than passing vacuously.
  - The adaptive-pair seed is the correct choice for `d`/`l`: `handleSlotCommitKey` (theme_panel_commit.go:73-82) only raises the confirm when the setting `IsConstant`, so no confirm intervenes, and the probes additionally assert `!after.themePanel.confirming()`.
  - The `themePanelDispatch` function value (:48-50) parameterises both probe maps over the dispatch, which is what makes the guard-of-the-guard (`themePanelArmRemoved`, :160-167; the swallow stand-in at :219) possible without duplicating the probes. Good design — the negative tests drive the *same* probe bodies the positive parity test drives, so they cannot drift from each other.
  - Confirm seed (`themeConfirmGuardModel`, :145-150) reuses `newSlotConfirmModel` (constant setting) and raises via the live `Update` path (`raiseSlotConfirmForTest` → `pressSlotKey` → `m.Update`), so the question under test is a real one.

TESTS:
- Status: Adequate
- Coverage: All nine test names the task specified are present and each maps to its acceptance criterion:
  - `TestThemePanelDescriptorDispatchParity` (:152) / `TestThemeConfirmDescriptorDispatchParity` (:156) — both scopes through the shared `assertDescriptorDispatchParity`.
  - Six panel probes (:63-96), one per descriptor entry, each asserting a bound effect: `↑↓` moves `list.Index()`; `^↑/↓` requires both `NextPage`/`PrevPage` bound **and** `Paginator.Page` to move (strictly stronger than the sessions/projects probes, which only check the bindings are non-empty); `⏎` asserts `CommitTheme` recorded with the cursor slug **and** the panel still open (both halves of §9.2); `d`/`l` assert `CommitThemeSlot` with the matching typed `theme.Member`; `esc` asserts closed + no confirm + `themePanelStateDropped`.
  - `TestThemePanelParity_DetectsARemovedArm` (:169) — table over all six arms via a stubbed dispatch, plus an unprobed-seventh-entry case and an unadvertised-probe case, each asserted through `requireDriftNaming` to report **exactly one** violation naming the right key. I traced all six rows: each removal falsifies only its own probe, so the "exactly one" assertion holds.
  - `TestThemePanelParity_ProbesAssertEffects` (:218) — runs *both* scopes' probes against a dispatch that swallows everything and changes nothing; every probe must return false. This is the criterion "a probe that merely swallowed the key would fail", verified by construction rather than by assertion prose. I confirmed each of the eight probes returns false under the swallow (notably the `n` probe, whose `len(slugs)==0` half is trivially true — it is the `!confirming()` half that fails).
  - `TestThemePanelDispatch_EscMeansInnermostFirst` (:238) — separate close and cancel assertions, exactly as the edge case demanded.
  - `TestThemeConfirmDispatch_UppercaseReachesTheSameArm` (:278) — `Y`/`N` plus the `shift+y`/`shift+n` shapes, routed through the *same* `themeConfirm{Yes,No}Honoured` helpers the parity probes use, so uppercase cannot drift from the lowercase descriptor entry.
  - `TestThemePanelKeymap_NoRightAlignedEntry` (:297) — no `RightAligned` in either theme scope, with a positive control that sessions and projects each still carry exactly one (so the assertion is not passing because the flag became meaningless).
  - `TestThemePanelKeymap_NoHelpEntry` (:329) — no `?` in either descriptor *or* either probe map; a live-dispatch subtest proving `?` is swallowed (panel stands, cursor unmoved, nothing written); and a `Ctrl-C` subtest with a live control that `Ctrl-C` still quits before asserting neither descriptor lists it.
  - `TestKeymapGuard_PageDescriptorsUnchanged` (:393) — key + `Core`/`RightAligned`/`Destructive` shape pinned as literals for all three pages.
- Notes:
  - No `t.Parallel()`, no build tag — unit lane, per the project rule.
  - No real I/O: both seeds route through `stubPanelDeps` (`fakeThemeSource`) and `fakeThemePersister`; nothing touches `prefs.json` or a themes directory.
  - Not under-tested. The one place I looked for a hole — `themeConfirmNoHonoured` asserting `!confirming()` without also asserting `themePanel.pending` is zeroed (`confirming()` reads only the message kind, theme_panel_confirm.go:19-21) — is already covered by `requireConfirmGone` in `TestSlotConfirm_CancelsOnThreeInputs`, so this is not a gap in the suite's own remit.
  - Deliberate, task-mandated overlap with the existing theme suites: `TestThemePanelDispatch_EscMeansInnermostFirst` restates `TestSlotConfirm_EscCancelsNotCloses` (theme_panel_confirm_test.go:346), `TestThemeConfirmDispatch_UppercaseReachesTheSameArm` restates `TestSlotConfirm_ConfirmsOnEitherCase`/`CancelsOnThreeInputs`, and the two `NoRightAlignedEntry`/`NoHelpEntry` descriptor loops restate `theme_panel_keymap_test.go:30-39,118-130`. The 9-10 copies each add a guard-specific angle (probe-map coverage, the stubbed-dispatch negative, the page controls) that the originals do not carry, so this is not bloat — but the pure-descriptor loops are genuinely duplicated (see notes).

CODE QUALITY:
- Project conventions: Followed. Unit-lane placement, no `t.Parallel()`, fakes over real seams, fixture preconditions raised as `t.Fatal` with an explanation of why the downstream assertion would be vacuous — the house style throughout this package.
- SOLID principles: Good. Parameterising the probe maps over a `themePanelDispatch` function value is a clean inversion that lets the positive suite, the removed-arm suite and the swallow suite share one probe body.
- Complexity: Low. Every helper is short and single-purpose; the deepest nesting is the two-level table in `DetectsARemovedArm`.
- Modern idioms: Yes — `slices.Equal`, `slices.Contains`, `slices.Sorted(maps.Keys(...))` for deterministic subtest ordering, `strconv.Quote` for the drift-message match.
- Readability: Good. The file-level doc comment (:15-16) states the anti-vacuity contract the task required, and the failure messages explain *why* a failure matters rather than only what differed.
- Issues: None blocking. Three small structural nits and one duplication question, below.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/keymap_dispatch_guard_theme_test.go:360-373 — the "neither scope advertises the global quit" subtest is nested inside `TestThemePanelKeymap_NoHelpEntry`, whose name is about `?`. Extract it into its own `TestThemePanelKeymap_NoGlobalQuitEntry` so the `Ctrl-C` guard is findable by name.
- [quickfix] internal/tui/keymap_dispatch_guard_theme_test.go:378-391 — `themeKeymapScopes()` spells the same three-field anonymous struct twice (return type and composite literal). Declare `type themeKeymapScope struct { name string; entries []keymapEntry; probes map[string]dispatchProbe }` and return `[]themeKeymapScope`.
- [quickfix] internal/tui/keymap_dispatch_guard_theme_test.go:90 — the `esc` probe presses `keyEsc`, declared in `internal/tui/edit_modal_state_machine_test.go:133`, while the theme suite's own identical `confirmEsc` sits at `internal/tui/theme_panel_confirm_test.go:31`. Collapse the two duplicate `tea.KeyPressMsg{Code: tea.KeyEscape}` vars to one and use it in both places, so the theme guard does not depend on the edit-modal fixture file.
- [quickfix] internal/tui/keymap_dispatch_guard_theme_test.go:447-453 — `descriptorShape` renders only `Key` plus the three booleans, so `TestKeymapGuard_PageDescriptorsUnchanged` would not notice an `Action`/`HelpKey`/`HelpAction` copy change despite its name. Add those three fields to the shape string and update the three `want` tables. (Sessions and projects are struct-pinned in `keymap_test.go:9-24` and `projects_keymap_test.go`; `previewKeymap()` is only pinned indirectly through the rendered-footer byte test at `pagepreview_keymap_constants_test.go:10-16`, so preview is the field this actually tightens.)
- [idea] internal/tui/keymap_dispatch_guard_theme_test.go:297-341 vs internal/tui/theme_panel_keymap_test.go:30-39,118-130 — the pure-descriptor "no `RightAligned`, no `?` entry" loops for both theme scopes are now asserted in two files. Decide whether to drop the 8-5 copies (leaving the guard-scoped versions, which also cover the probe maps and the live swallow) or keep both as independent statements.
