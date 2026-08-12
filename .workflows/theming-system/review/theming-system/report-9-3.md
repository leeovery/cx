TASK: theming-system-9-3 — `d` / `l` Commit A Slot Under An Adaptive Setting (tick-79eeb1)

ACCEPTANCE CRITERIA:
1. On an adaptive setting, `d` calls `CommitThemeSlot(cursorSlug, dark)` and `l` calls it with `light` — asserted against a recording fake.
2. A slot commit leaves the other raw slot key untouched, and `d` then `l` on the same row yields a single `● both` badge row after the recompute.
3. The constant is cleared in the same write (one `CommitThemeSlot` call, no second call) and cleared in memory.
4. With a constant set, `d` and `l` write nothing in this task and leave the model unchanged (confirm seam a no-op until task 9-5).
5. The panel stays open and the previewed theme is unchanged: previewing a light theme in a dark terminal and pressing `l` leaves the composed frame byte-identical.
6. Committing a slot to the row already carrying that slot's badge is idempotent — same call, same resulting keys, no error.
7. The slot argument is the typed slot value; no code path constructs a slot from a string.
8. A failed slot commit mutates nothing and returns the error.
9. A nil persister is inert (no write, no mutation, no failure state).
10. No directory read, no prefs read, no enumeration, no tmux write on either key.
11. `Enter` after a slot commit clears both slots again and still raises no confirm.

STATUS: complete

SPEC CONTEXT:
§9.2 (specification.md:993-994) pins the keymap: `d` writes `theme_dark = <selection>` clearing the constant, `l` writes `theme_light = <selection>` clearing the constant, both leaving the panel open. §9.2:1029 pins that committing to a non-active slot changes nothing on screen — "a commit is a write, not a navigation". §9.2:1033 pins that assigning a slot while a constant is set asks for confirmation first (the loss this task deliberately refuses to risk). §9.5:1119 pins the `● both` collapse as reachable in two keypresses and a likely path. §9.14:1258 records that the slot half is genuinely novel — no surveyed tool assigns to a light/dark slot from inside a picker.

IMPLEMENTATION:
- Status: Implemented (mechanism partially superseded by later in-plan tasks — see Notes)
- Location:
  - `internal/tui/theme_panel.go:330-333` — the `d` / `l` dispatch arms, ahead of the nav arm and the swallow-everything default, inside the `themePanel.open`-gated route (`internal/tui/model.go:1651-1653`).
  - `internal/tui/theme_panel_commit.go:73-82` — `handleSlotCommitKey`, the setting-shape gate.
  - `internal/tui/theme_panel_commit.go:86-91` — `commitSlot`, over the shared `commit` protocol.
  - `internal/tui/theme_panel_commit.go:31-43` — `commit`: nil-persister inertness, write, `applyCommitResult`, in-memory mirror, recompute-on-success-only.
  - `internal/tui/theme_panel_commit.go:11-25` — `commitSelected` / `committableThemeSlug`: selected row with the defensive `Selectable()` + non-empty-slug guard.
  - `internal/theme/setting.go:41-46` — `RawKeys.WithMember`: the in-memory mirror; writes the named half, carries the other across verbatim, clears the constant.
  - `internal/prefs/store.go:229-247` — `SaveThemeSlot`: one atomic mutate that sets the slot and clears `theme`, `omitempty` rendering a cleared key absent.
  - `cmd/theme_persister.go:34-64` — the single `theme.Member` → `prefs.ThemeSlot` conversion site.
- Notes:
  - **Criterion 7 — intentional supersession, not drift.** The task text specified `prefs.ThemeSlot` threaded through the seam; the seam now carries the domain type `theme.Member` (`internal/tui/model.go:78`), converted exactly once at the persistence boundary (`cmd/theme_persister.go:59-64`). Commit `4051241f` (task 12-9, "hold the domain light/dark type in the panel and convert once at the persister seam") made this change deliberately. The criterion's *intent* — a typed value threaded end to end, never minted from a string, so no path can name a third slot — holds strictly harder than the original wording: `TestPanelSlotCommit_TypedSlotOnly` now bans `prefs.Slot*` from the package entirely as well as banning `theme.Member(...)` conversions. It also keeps `internal/prefs` (a deliberate leaf) out of `internal/tui`'s theme path.
  - **Criterion 4 — intentional supersession.** The `raiseSlotConfirm` seam this task left as a no-op is now the real confirm (`internal/tui/theme_panel_confirm.go:23-26`, task 9-5, commit `065386ce`). The load-bearing half of the criterion — over a constant `d`/`l` write **nothing** — still holds exactly (`TestPanelSlotCommit_InertOverAConstant:276-278` asserts zero persister calls); only the "leave the model unchanged" half is superseded by the confirm being raised, which is the specified end state (§9.2:1033).
  - The setting-shape gate is routed through `themeSetting()` → `theme.ResolveSetting` (`theme_panel.go:200-203`) rather than the literal `m.themeKeys.Theme != ""` the task prescribed. Better: the gate cannot disagree with the seam's `theme`-wins tiebreak (§8.2), and it is one comparison rather than a re-implemented rule.
  - Single derivation site honoured: `commitSlot` adds no badge logic; badges move only through `recomputeThemePanel` → `applyCommittedSetting` (`theme_panel_commit.go:97-127`).
  - `recomputeThemePanel` runs on landed writes only (`commit` returns before the mirror and recompute on error), so a failed write cannot move the `●`.
  - No `ApplyTheme` on any commit path — the previewed theme survives the write, as §9.2:1029 requires.

TESTS:
- Status: Adequate
- Coverage: `internal/tui/theme_panel_commit_slot_test.go` (637 lines) carries all twelve named tests from the plan plus two justified additions, each mapping to a distinct criterion:
  - C1 — `TestPanelSlotCommit_DarkWritesTheDarkSlot:108` / `_LightWritesTheLightSlot:129`. Asserted separately so a transposed slot argument cannot hide; `requireSlotCommits:29-43` additionally fails if a slot key recorded a *constant* call.
  - C2 — `_OtherSlotSurvives:150` (the other slot deliberately names an unresolvable slug, so a re-derived pair would show up), `_EmptyOtherSlotStaysEmpty:168` (unset-stays-unset — a genuine extra, covering the `omitempty` half), `_DThenLYieldsBoth:188` with `requireBothBadge:95-106` counting exactly one `● both` and asserting no `● light`/`● dark` survives.
  - C3 — `_ClearsTheConstantAtomically:218`, both subtests asserting `len(persister.slugs) == 1` (the one-call, not-two property) and the in-memory clear.
  - C4 — `_InertOverAConstant:258`, table-driven over both keys, with a positive control (`:293-298`) proving the same keypress *does* write under an adaptive pair — the assertion that stops the test passing because the key was dead.
  - C5 — `_NonActiveSlotIsVisuallyInert:303`: byte-identical `View().Content` across the keypress, a palette-set comparison for the badge-moving variant, and an `Esc`-still-resolves-the-dark-slot subtest.
  - C6 — `_RepeatIsIdempotent:377`: two recorded calls (so a "don't rewrite what is set" shortcut fails), identical keys, identical badges, identical frame, nil error.
  - C7 — `_TypedSlotOnly:416`: AST guard over production files only (`PackageGoFiles(".", false)`, `internal/sourceguardtest/packagegofiles.go:15`), banning `theme.Member(...)` conversions and any `prefs.Slot*`/`prefs.ThemeSlot` reference, pinning the member set to exactly two, plus a reflect check on the seam's parameter type.
  - C8 — `_FailedWriteLeavesKeysAlone:483`: keys, rows, badges and the applied theme untouched, zero reassemblies, panel open, failure message raised, error returned from the helper; a third subtest proves the same fixture *does* recompute when the write lands (guards against a vacuous negative).
  - C9 — `_NilPersisterIsInert:558`: no mutation, no message (explicitly distinguishing absence-of-writer from failed write), byte-identical frame, plus a wired positive control.
  - C10 — `_NoOtherIO:627` over the shared `requireCommitDoesNoOtherIO` (`theme_testing_test.go:75-144`): counts every file-touching seam, pins the enumeration count to the open's, asserts a nil `tea.Cmd` (the one shape counters cannot see), and asserts the config dir and themes dir are untouched on disk.
  - C11 — `_EnterAfterSlotNeedsNoConfirm:594`: the reverse direction clears both slots, raises no message, and leaves the footer byte-identical.
- Notes:
  - Assertions are behavioural throughout: frames, badges, recorded seam calls and persisted-key structs, not internal call ordering. Failure messages state the invariant, so a break is diagnosable from the output alone.
  - No redundancy found. The two extras (`_EmptyOtherSlotStaysEmpty`, `_NoOtherIO`) each cover a criterion the named list does not reach.
  - Not flagged as a gap: `d`/`l` on an unselectable row has no dedicated test, but both keys share the `committableThemeSlug` chokepoint that `TestPanelEnter_UnselectableRowWritesNothing` (`theme_panel_commit_test.go:371`) already drives, and the arrows structurally cannot park the cursor there. A duplicate would be over-testing.

CODE QUALITY:
- Project conventions: Followed. Small seam interfaces with a nil-tolerant default (`ThemePersister`, `model.go:78`); the `internal/prefs` leaf stays free of `internal/theme`, with the one conversion quarantined at `cmd/theme_persister.go:59`; logging stays out of `internal/tui` (the persister owns the `theme: commit failed` emission, `theme_panel_commit.go:54-55`); the value-receiver-plus-`(&m)` update idiom matches the rest of the package.
- SOLID principles: Good. `commit` (`theme_panel_commit.go:31`) is the single commit protocol; `commitConstant` and `commitSlot` differ only in the write closure and the mirror closure, so the two keys cannot drift in nil-handling, failure reporting or recompute policy. The mirror is delegated to `theme.RawKeys.WithMember`, so the panel re-implements no merge rule.
- Complexity: Low. `handleSlotCommitKey` is one branch; `commitSlot` is a two-argument delegation; the dispatch arms are flat cases in an existing switch.
- Modern idioms: Yes — `slices.IndexFunc`, `cmp.Or`, `reflect.TypeFor`, `maps.Clone`/`maps.Equal` in tests.
- Readability: Good. Comments state the *why* (why the target is the selected row, why the other slot survives, why a failed write must not recompute) with no restated code, no task/phase/spec citations, and no claim the code falsifies.
- Issues: None material.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `internal/tui/theme_panel_commit.go:75` — the `_ =` discard of `commitSelected`'s error carries no explanation here, while the sibling `Enter` arm (`theme_panel.go:326-327`) carries exactly that reasoning. Add the same one-line comment above line 75: `// The report is raised inside the commit; a failed write leaves the keys untouched.`
- [do-now] `internal/tui/theme_panel_commit_slot_test.go:452-453` — the mixed `&&`/`||` condition relies on precedence without parentheses. Replace with `if (isPackageSelector(node, "prefs", "") && strings.HasPrefix(node.Sel.Name, "Slot")) || isPackageSelector(node, "prefs", "ThemeSlot") {` — same semantics, the grouping made explicit.
