TASK: theming-system-17-2 (tick-d286ef) — Extract The One Commit Protocol Behind commitConstant And commitSlot

ACCEPTANCE CRITERIA:
- The five-step protocol appears once in the package; `commitConstant` and `commitSlot` are each a single delegating call.
- One `committableThemeSlug` guard-and-delegate wrapper, not two.
- Behaviour is byte-identical: same write order, same failure handling, same recompute timing, same return values on the nil-persister and failed-write paths.
- No new exported symbols.

STATUS: complete

SPEC CONTEXT:
The protocol being deduplicated encodes several spec-pinned rules from the theme slide-over sections. §9.13 ("A failed commit write"): a failed commit does not move the `●`, the failure state is outstanding until a *later* commit succeeds, and the persister — not the TUI — owns the `theme: commit failed` emission. §9.x recompute rules: "a successful commit recomputes the panel's full row set, not just the badges", and "the recompute uses the construction-time snapshot plus this instance's own mutation — never the merged bytes the RMW just read" (which is precisely why the mirror is a `RawKeys` transform on the keys in hand rather than a re-read of prefs). §11.x / §9.2: a commit is a write, not a navigation — hence no `ApplyTheme` on the commit path. The nil-persister case is the writer-less construction path, which must be inert rather than reported as a failure. All of these rules survive the refactor unchanged; they are now stated once instead of twice.

IMPLEMENTATION:
- Status: Implemented, exactly as specified. No later commit in the plan supersedes it (`git log a26b2378..HEAD -- internal/tui/theme_panel_commit.go` is empty), so this is the final state.
- Location:
  - `internal/tui/theme_panel_commit.go:31-43` — the extracted `func (m *Model) commit(write func() error, mirror func(theme.RawKeys) theme.RawKeys) error` holding the whole five-step protocol in the original order: nil-persister short-circuit → `write()` → `applyCommitResult(err)` → bail on error → mirror `m.themeState.keys` → `recomputeThemePanel()` → nil.
  - `internal/tui/theme_panel_commit.go:45-50` — `commitConstant` reduced to a single `m.commit(CommitTheme closure, WithConstant mirror)` call.
  - `internal/tui/theme_panel_commit.go:86-91` — `commitSlot` reduced to the same call with `CommitThemeSlot` / `WithMember`. The false "Carries commitConstant's rules" lead-in is deleted; the substantive `● both` note is kept.
  - `internal/tui/theme_panel_commit.go:9-17` — the two selected-row wrappers collapsed into `commitSelected(commit func(slug string) error)`, retaining the "the target is the selected row, never `m.themeState.keys`" comment.
  - Call sites updated: `internal/tui/theme_panel.go:328` (`Enter` → `commitSelected((&m).commitConstant)`) and `internal/tui/theme_panel_commit.go:75` (`handleSlotCommitKey` → `commitSelected(func(slug string) error { return (&m).commitSlot(slug, member) })`).
  - `internal/tui/theme_panel_confirm.go:55-64` — `confirmSlotAssignment`'s separate persister-nil check left in place per Do-step 6; its comment ("the nil-check cannot be inferred from the nil error") remains true.
- Notes:
  - Byte-identical behaviour verified statement-by-statement against the pre-refactor code in commit `a26b2378`: same ordering, same early returns, same error propagation. `commitSelected` still returns `nil` on an uncommittable row, and `commit` still returns `nil` on a nil persister and the persister's error verbatim on a failed write.
  - The nil-persister ordering is safe: both write closures dereference `m.themeState.persister`, and both are only invoked after `commit`'s nil check — the closures are constructed, not called, at `commitConstant`/`commitSlot`. The doc comment states this explicitly.
  - Acceptance criterion 1 is now *structurally* enforced, not just visually: `internal/tui/theme_panel_confirm_test.go:693` asserts the AST-level caller set of `applyCommitResult` is exactly `[commit]`, so re-splitting the protocol into two paths fails a test.
  - No new exported symbols — `commit` and `commitSelected` are both unexported methods.
  - No stale references remain: `commitSelectedConstant` / `commitSelectedSlot` are absent repo-wide from Go source (grep hits are confined to `.workflows/` narrative and `.tick/tasks.jsonl` history).
  - Deliberately unchanged: `handleSlotCommitKey:78` still calls `committableThemeSlug` directly for the *confirm* branch. That is a different operation (raise a question, not commit), not a third copy of the guard-and-delegate wrapper — criterion 2 holds.

TESTS:
- Status: Adequate (one minor redundant assertion — see notes).
- Coverage:
  - New `internal/tui/theme_panel_commit_protocol_test.go` is table-driven over both commit shapes (`commitShapes()` at :19, parameterised by write recorder and expected key mirror), covering the three protocol invariants the task asked for:
    - `TestCommitProtocol_FailedWriteMovesNothing` (:52) — the persister's error is returned, the write *was* attempted, `m.themeState.keys` are untouched, zero reassemblies, the failed-commit message is raised and `themeState.commitFailed` is set.
    - `TestCommitProtocol_LandedWriteMirrorsThenRecomputes` (:80) — asserts *ordering*, not just outcomes: exactly one reassembly, and `source.reassembleKeys[0] == mirrored`, which pins that the keys move BEFORE the panel is rebuilt from them. This assertion is genuinely new coverage that neither pre-existing per-shape test had; it required the `reassembleKeys` recorder added to `fakeThemeSource` in the same commit.
    - `TestCommitProtocol_NilPersisterIsInert` (:111) — nil return, no key mutation, no reassembly, no message, no outstanding failure, frame unchanged.
  - Expected mirrors are correct against the fixture: the fixture opens on `RawKeys{Theme: persisted}`, so `WithConstant(target)` → `{Theme: target}` and `WithMember(MemberDark, target)` → `{Dark: target}` (verified against `internal/theme/setting.go:33,41` — `WithMember` clears the constant and carries the other half).
  - The fixture-shape guard at :42-50 (`commitProtocolSource` Fatals if the model's source is not the recording fake) means the reassembly counters cannot silently no-op.
  - Existing tests pass unchanged in substance; only call-site renames were applied (`theme_panel_commit_test.go:293`, `theme_panel_commit_slot_test.go:411,532` via the new `commitSelectedDarkSlot` helper), plus the structural-guard expectation update at `theme_panel_confirm_test.go:693`. No assertion was weakened or deleted — the slot-failure subtest still asserts the constant survives and badges do not move; the keypress-level `NilPersisterIsInert` / `FailedWriteLeavesKeysAlone` tests still drive through `updateThemePanel` and assert panel-open, no scheduled `tea.Cmd`, and theme-stays-applied, which the protocol test does not and should not cover.
  - Would fail if the feature broke: yes on every axis. Reordering mirror/recompute breaks the `reassembleKeys[0]` assertion; dropping the nil check breaks the inert test (and would nil-panic in the closure); dropping the error bail breaks the failed-write test; re-duplicating the protocol breaks the `applyCommitResult` caller guard.
- Notes: one small over-test — see NON-BLOCKING NOTES. Nothing under-tested.

CODE QUALITY:
- Project conventions: Followed. Unexported seam-style function parameters rather than an interface (appropriate for a 2-parameter intra-package variation point); comments carry the *why* in the codebase's established style with no process-artifact references (no task ids, phases or spec section numbers in source); unit-lane test with no `t.Parallel()`, no tmux/daemon/binary touch, so the lane rule is respected.
- SOLID principles: Good. The refactor is a textbook template-method-by-parameterisation: the invariant protocol has one owner, the two variable steps are injected. Single responsibility improves — `commitConstant`/`commitSlot` now say only *what varies*.
- Complexity: Low. `commit` is 7 statements with a single branch; both callers are one expression each.
- Modern idioms: Yes. Function-typed parameters over an interface or a bool/enum discriminator is the right Go call for two variation points inside one package; the closures capture exactly what they need.
- Readability: Good. The comment relocation is accurate: "the write is never reached, so no closure dereferences it" is the non-obvious safety argument a reader needs at the new abstraction, and the surviving `commitSlot` comment (`● both` reachable via `d` then `l`) is the only remaining shape-specific fact.
- Comment accuracy: Verified. Every retained comment holds against the code it now describes; the one comment the refactor falsified ("Carries commitConstant's rules") was deleted rather than left behind. `theme_panel_confirm.go:53-54`'s reference to `commitSlot` returning nil on the writer-less path is still true through the delegation.
- Security / Performance: N/A — no I/O, allocation or control-flow cost change; two closures per commit keypress.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/theme_panel_commit_protocol_test.go:122,138-140 — delete the `before := m.View().Content` capture and the trailing frame-comparison assertion from `TestCommitProtocol_NilPersisterIsInert`. It is redundant three ways: the same render-unchanged fact is already asserted at the keypress level by `theme_panel_commit_test.go:322-324` and `theme_panel_commit_slot_test.go:584-586`, and at this level it is strictly implied by the three assertions immediately above it (keys untouched :128, zero reassemblies :129-131, no message :132-134) — with no key handling in play, nothing else in the direct-call path can move a pixel. Removing it keeps the protocol test stating the protocol rather than re-testing the renderer.
