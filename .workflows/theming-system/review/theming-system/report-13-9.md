TASK: theming-system-13-9 (tick-1249a1) — Resolve themeState.nomination's Unread Post-Commit Refresh

ACCEPTANCE CRITERIA:
- The field's doc comment and its actual maintenance agree.
- No write-only state remains on `themeState`.
- Commit behaviour is otherwise unchanged: badges refresh, the palette on screen does not swap, and nothing moves on a resolve error.
- The exit-path canvas restore is unaffected.
- `go test ./internal/tui ./internal/capture` passes.

STATUS: complete

SPEC CONTEXT:
§8.4/§8.8 (specification.md:811-826, 891) make the constructor take the loaded *nomination* and pin the gate's single resolution: "the model holds that nomination, the raw keys, and which member is currently active", where the threaded palette is always exactly one theme. §9.2 (specification.md:1018-1031) governs the commit: a successful commit recomputes the panel's full row set and badges from the construction-time snapshot plus this instance's own mutation, and "a commit is a **write, not a navigation**" — committing to a non-active slot changes nothing on screen; the display resolves from persisted state only on close. §8.4 (specification.md:802-809) additionally requires the *opposite* slot to be loaded at commit time on a constant → adaptive conversion, which is the one theme load outside construction. §11.4 (specification.md:1400-1401) pins the exit-time restore to the retained startup canvas hex precisely because a mid-session commit and an uncommitted preview both diverge from it. Nothing in the spec requires a post-gate reader of the held nomination.

IMPLEMENTATION:
- Status: Implemented — via the task's alternate branch (keep-and-document), not the directed drop path. Not drift against the spec; a deliberate deviation from the task's step 2.
- Location:
  - internal/tui/theme_panel_commit.go:120-127 — `applyCommittedSetting`; the assignment `m.themeState.nomination = resolution.Nomination` survives at :126, with the badge refresh at :125 and the on-error early return at :122-124 untouched.
  - internal/tui/theme_state.go:35-37 — the field's doc comment, rewritten by this task ("Describes what is persisted, not what is rendered — the palette in force is active") and later compressed by the topic-wide comment sweeps (e3fa1503 / 915e7fcb).
  - internal/tui/theme_panel_commit.go:93-96, :117-119 — the surrounding commit comments; the "keeps its contract through one site" phrase the task flagged (step 4) is gone.
  - Commit 9b7d092f touched comments + one new test only: no production statement was added or removed, so commit behaviour is unchanged by construction.
- Notes:
  - I re-ran the task's step-1 reader survey against HEAD. It holds exactly as the task predicted: two writes (model.go:555 in `WithThemeNomination`, theme_panel_commit.go:126) and three reads (model.go:837 `newNominationGate`, :853 `hasNomination`, :861 `nomination.Select` — the latter two inside `syncResolvedMode`). `syncResolvedMode`'s only callers are `New` (model.go:839), `armAppearanceDetection` (:845) and the two once-only gate arms in `Update` (:1495, :1500). The close/open paths resolve from `themeState.keys` through the seam (theme_panel.go:172, :201; theme_panel_commit.go:100, :121), never from the held nomination, and `themeSetting()` reads keys too. So there is still **no production reader past the gate**, and the task's step-2 condition ("if that holds, take the drop path") was met yet the drop was not taken.
  - The keep branch is nonetheless defensible and I would not block on it. Two facts the task's analysis did not weigh: (a) `loadNewlyLiveSlot` (theme_panel_confirm.go:72-77) discards `LoadSlot`'s error and takes no palette, so the post-commit re-resolution is the model's **only** in-memory record of the pair a constant → adaptive conversion makes live — dropping it would leave §8.4's commit-time load with no in-model product at all; (b) a body of pre-existing conversion tests asserts exactly that record (theme_panel_commit_load_test.go:135-175, 490-521, 580-600, 845-855), so the drop path would have deleted spec-anchored assertions rather than dead ones. The residual is a maintenance write whose only observers are tests, now honestly described rather than contractually over-claimed.
  - AC-by-AC: comment ↔ maintenance agree (yes — the field is maintained to the persisted setting and the comment says exactly that); no write-only state on `themeState` (yes at field level — I checked every field: `nomination`, `keys`, `source`, `persister`, `gate`, `canvasMode`, `active`, `startupCanvasHex`, `reply`, `commitFailed` (theme_panel.go:249, :268) and the three capture seeds (theme_panel.go:151, :159, :163) all have readers); commit behaviour unchanged (yes — comment-only production diff); exit-path restore unaffected (yes, and now pinned by a new test).
  - Comment accuracy: the current comments hold against the code. No process-artifact references (no task ids, phases or spec sections) survive in the touched comments.

TESTS:
- Status: Adequate
- Coverage:
  - internal/tui/theme_panel_commit_load_test.go:771-793 — `TestCommitSlotLoad_RestoreStaysAnchoredAfterACommit`, added by this task. Drives a real constant → adaptive conversion through the panel's own keypress path (`openConversionPanel` → `convertToSlot` → confirm `y`), fatals the fixture unless the two values actually diverge (`startupCanvasHex` = the constant's `#101010`, `active.Canvas` = nord's `#2E3440`), then asserts both directions of the canvas-echo guard through `RestoreTerminalBackground`: an echo of the startup canvas is skipped, the active theme's canvas is set back (helpers at restore_divergence_test.go:36-65). This is the task's second Tests bullet and it would fail if the guard were ever re-derived from the active theme.
  - internal/tui/theme_panel_commit_load_test.go:724-758 — `TestCommitSlotLoad_ConversionDoesNotMoveStartupCanvasHex` covers the third Tests bullet's freeze half across light/dark/no-reply, with negative controls proving the fixture can detect a re-capture from either the previewed or the newly-loaded palette; its sub-test at :760-768 structurally pins `syncResolvedMode`'s caller set to {New, Update, armAppearanceDetection}, which is what keeps the nomination's readers pre-gate and would fail loudly if a future post-gate reader were wired in.
  - internal/tui/theme_panel_commit_load_test.go:795-831 — `TestCommitSlotLoad_AnswerIsIndependentOfTheLoad` covers the `canvasMode` half of the documented divergence after a mid-session conversion (in-force answer flips to light from the retained reply, load landing or fatal).
  - The "commit behaviour otherwise unchanged" criteria are covered by the existing suite: badges/rows recompute (`recomputeThemePanel` tests), no palette swap on commit, and the resolve-error no-op (:490-521, :845-855 assert the nomination, keys and active theme all stand still on a failed write / broken-builtin fatal).
- Notes:
  - The new test overlaps `TestRestoreBackground_CommittedThemeDivergence` (restore_divergence_test.go:67-90) in shape, but not in substance: that one drives `ApplyTheme` directly on a synthetic model, while this one composes the *real* commit path with the exit restore. The composition was previously untested, so this is a closed gap rather than a redundant case. Not over-tested — ~22 lines, four assertions, no new mocking.
  - Per my instructions I did not execute the suite; adequacy is judged by reading. The production diff is comment-only, so no existing assertion can have been invalidated by it.

CODE QUALITY:
- Project conventions: Followed. Comment-only production change; comments state *why* rather than restating code, and carry no spec-section or task citations (the topic's own standard, tick-56c2d3 / tick-8f9d1a). Test carries no `t.Parallel()`, uses the shared `themetest`/`logtest` helpers and the package's fixture constructors rather than hand-rolling a model — matches golang-testing conventions and CLAUDE.md's DI/testing pattern.
- SOLID principles: Good. `applyCommittedSetting` stays the single owner of the post-commit resolution products (badges + nomination), which theme_panel_confirm.go:66-71 explicitly relies on to avoid a second writer.
- Complexity: Low. No branching added.
- Modern idioms: Yes (no opportunities in a comment-only diff).
- Readability: Good, with one erosion — see the note below: the reader-set clause this task added to the field's doc comment was removed by the later topic-wide comment sweeps, leaving the "not rendered from" half but not the "no reader past the gate" half that is the actual conclusion of this task's analysis.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_state.go:35-36 — the later comment sweep dropped the clause that records this task's finding, so the comment no longer warns a future reader off treating the field as post-gate-readable, and the reason the refresh is kept is now only in a commit message. Replace the two-line comment with:
  `// Zero value is the "nothing was injected" sentinel. Describes what is`
  `// persisted, not what is rendered — the palette in force is active. Read only`
  `// by the gate and syncResolvedMode, both before or at the gate's single`
  `// resolution; the post-commit refresh has no reader past it and stays because`
  `// it is the model's only record of the pair a constant → adaptive conversion`
  `// loads.`
