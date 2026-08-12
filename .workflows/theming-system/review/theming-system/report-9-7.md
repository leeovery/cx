TASK: theming-system-9-7 — A Failed Commit Write And The Outstanding-Failure State

ACCEPTANCE CRITERIA:
- A failed commit leaves the previewed theme applied and the composed frame's colours unchanged.
- A failed commit performs no key mutation and no recompute — the `●` is on the same row before and after, asserted on the rendered rows.
- The message renders exactly `⚠ couldn't save theme` in the panel's message slot.
- The message survives an intervening non-key event (a `tea.WindowSizeMsg` above the floor) and is cleared by the next keypress, which still performs its own action.
- The message does not auto-clear: past `flashAutoClearDuration` with no keypress it is still rendered, and no `flashTickCmd` is issued by the failure path.
- The outstanding flag remains true after the message is dismissed by an arrow, after paging, and after a confirm is raised and cancelled.
- A subsequent successful commit clears both the message and the outstanding state.
- The panel emits no `theme` log record on the failure path; the persister's single `theme: commit failed` WARN is the only record.
- A failed confirm-driven commit leaves the constant in the in-memory keys, shows the bare `●` still on it, restores the standing footer, and renders the failed-commit message with no confirm live.
- The confirm and the failed-commit message are never live simultaneously.
- A nil persister raises neither the message nor the outstanding state.
- Pressing the same commit key again after a failure retries; a success clears everything.

STATUS: complete

SPEC CONTEXT:
§9.13 (specification.md:1231-1251) defines the failed-write contract: report in the panel's message slot with `⚠` plus terse copy, persisting until the next keypress rather than on a timer; keep the theme applied in memory; do not move the `●` because the marker means "what is persisted". It then splits state from message — "outstanding" runs from the failed write until a subsequent commit succeeds and nothing else clears it, so arrowing away dismisses the message but not the state (which is what task 9-9's close-time flash discharges). §14A pins the copy `⚠ couldn't save theme` (line 1799) and the sibling close flash `theme not saved — see portal.log` (line 1814). §12.3/§8.9 make the `cmd`-owned persister the single emission site for `theme: commit failed`, closing the `theme` component at three emitters — so the panel must not log.

IMPLEMENTATION:
- Status: Implemented (mechanism as planned; one deliberate later refinement — see Notes)
- Location:
  - `internal/tui/theme_panel_commit.go:31-43` — `commit(write, mirror)`: the single chokepoint. Nil persister returns early (inert, never "failed"); on error it calls `applyCommitResult(err)` and returns before the key mirror and `recomputeThemePanel`, so no key mutation and no recompute happen on a failed write.
  - `internal/tui/theme_panel_commit.go:54-68` — `applyCommitResult(err)` / `reportCommitFailure()`: error raises the message and sets the flag as one act; success clears a live failed-commit message and clears the flag. No `ApplyTheme`, no logging.
  - `internal/tui/theme_panel.go:314-316` — `updateThemePanel` clears the failed-commit message at the top of the key arm and falls through to normal dispatch (one key, one intent). It takes `tea.KeyPressMsg` and is only reached from `model.go:1651-1655`, which intercepts keys only — so a `WindowSizeMsg`/refresh cannot clear the message.
  - `internal/tui/theme_panel_message.go:21` — copy `themePanelCommitFailedMessage = flashWarningGlyph + " couldn't save theme"` (glyph in the string, so it survives NO_COLOR); `:98-106` renders it in `th.AccentAttention` via `headerStyle` (foreground + canvas background only — no `bg.attention` band); `:58-62` `clearThemePanelCommitFailed` is kind-guarded so the pre-dispatch clear cannot take down a live confirm.
  - `internal/tui/theme_state.go:63-69` — the `commitFailed` flag on `themeState` (which outlives the panel struct the close discards).
  - `internal/tui/theme_panel_confirm.go:55-64` — `confirmSlotAssignment` resolves the confirm *before* the write, so the two message contenders can never be live together; a failed write returns before `loadNewlyLiveSlot`.
  - `cmd/theme_persister.go:43-50` — the single `theme: commit failed` WARN, which also returns the error.
- Notes:
  - The plan named the flag `m.themeCommitFailed` on the `Model`; the original 9-7 commit (680cde59) did exactly that, and a later remediation task (22beb3f3, 11-15 — the theme-state consolidation) moved it to `m.themeState.commitFailed`. Behaviour and lifetime are unchanged and the load-bearing rationale survives in the field comment. Intentional supersession, not drift.
  - The plan listed three callers of `applyCommitResult` (`commitConstant`, `commitSlot`, the confirm's `y`); the implementation funnels all three through `commit`, giving a single caller. That is a stronger form of "the failure semantics live in one place", and the seam-caller guard at `theme_panel_confirm_test.go:692-694` pins it.
  - Only two sites clear `commitFailed` (verified repo-wide): a successful commit here, and task 9-9's close-time report discharge (`theme_panel.go:248-255`) — exactly what §9.13 mandates.
  - `internal/tui` binds no `log.For` at all, so "the panel must not log" is structurally guaranteed rather than merely tested.
  - The `themeState.initialCommitFailed` capture-only seed (`theme_panel.go:157-166`, wired from `internal/capture/fixtures.go:478-490`) is a later phase's fixture route, not a production path; the task's actual rule — a nil persister never enters the reported-failure state — still holds.

TESTS:
- Status: Adequate (one duplication, noted below)
- Coverage: `internal/tui/theme_panel_commit_failure_test.go` implements all 13 planned tests under the planned names. Every test drives real dispatch through `m.Update(...)` rather than calling `updateThemePanel` directly, so the fall-through and interception behaviour is genuinely exercised.
  - `TestCommitFailure_ThemeStaysApplied:128-165` diffs the whole composed frame's colour set before/after, allows only `accent.attention` runs to appear, and asserts the previewed canvas sequence is still painted — a strong reading of "colours unchanged".
  - `TestCommitFailure_BadgeDoesNotMove:92-124` asserts the rendered `●` rows, the badges map, the raw keys and the row labels are all untouched, then repeats the fixture with the write landing to prove the comparison is not vacuous.
  - `TestCommitFailure_MessageClearsOnNextKeyAndFallsThrough:177-203` asserts the arrow both clears the message and moves the cursor/preview, and that the list body regains the row — the "one key, one intent" property.
  - `TestCommitFailure_MessageHasNoTickLifecycle:226-253` asserts a nil `tea.Cmd`, an unbumped `flashGen`, an empty main-screen flash, and message survival across a `flashTickMsg`. `flashTickMsg` is the model's only clock coupling, so this is a faithful stand-in for "advance the clock past `flashAutoClearDuration`".
  - `TestCommitFailure_StateOutlivesTheMessage:255-309` covers all three planned dismissals (arrow, page — with a fixture guard that the list actually paginates — and a confirm raised then cancelled).
  - `TestCommitFailure_PanelEmitsNoThemeRecord:462-487` guards against a dead sink by requiring the open to have emitted first, then asserts the record count is unchanged across the failure.
  - `TestCommitFailure_NilPersisterRaisesNothing:489-520` covers all three commit keys and additionally asserts the rendered frame is byte-identical.
  - The `⚠ couldn't save theme` copy is independently pinned from the capture side (`internal/capture/theme_panel_message_fixtures_test.go:128,138,173`), including a badge-unmoved frame assertion.
- Notes: `TestCommitFailure_ConfirmDrivenFailure` (`theme_panel_commit_failure_test.go:385-423`) and `TestSlotConfirm_FailedCommitKeepsTheConstant` (`theme_panel_confirm_test.go:653-702`) are near-identical: same fixture, same key sequence, and ~10 overlapping assertions (keys untouched, badges map untouched, row labels untouched, confirm gone, standing footer, theme still applied, nil cmd, failed-commit message, outstanding flag). The confirm-side test pre-existed this task and 9-7 extended it with the report assertions *and* added the dedicated duplicate. No coverage is missing; the cost is two places to update when the confirm-failure contract moves.

CODE QUALITY:
- Project conventions: Followed. Comments state the load-bearing "why" with no restated code and no process-artifact references (§/task ids were stripped from `internal/tui` by the later comment sweep). Tests carry no `t.Parallel()`, use table/subtests appropriately, and stay in the unit lane (no daemons, no built binaries).
- SOLID principles: Good. `applyCommitResult` is the single place the failure semantics live; `commit` parameterises only the two things that differ between a constant and a slot write (`write`, `mirror`); the persister owns logging and the panel owns reporting, with no overlap.
- Complexity: Low. `commit` is a five-line guard/branch; `applyCommitResult` and `reportCommitFailure` are three lines each; the key-arm clear is one guarded call.
- Modern idioms: Yes.
- Readability: Good. The state/message lifetime split is explained where each half lives (`theme_state.go:63-68` for the state, `theme_panel_commit.go:52-53` for the message), and `clearThemePanelCommitFailed`'s kind guard carries the reason it is not unconditional.
- Comment accuracy: Verified — every claim in the changed code holds against it. `theme_panel_commit.go:29-30` ("On error nothing moves, so the `●` cannot move") matches the early return before the mirror/recompute; `theme_panel_message.go:12-13` ("the confirm gates the write, so by the time a write can fail it has resolved") matches `confirmSlotAssignment`'s resolve-then-write order; `theme_panel_message.go:96-97` ("takes no bg.attention band") matches `headerStyle`'s foreground-plus-canvas style.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/theme_panel_confirm_test.go:653-702 — collapse the duplication with `TestCommitFailure_ConfirmDrivenFailure` (internal/tui/theme_panel_commit_failure_test.go:385-423): strip the ~10 assertions the two share (badges map equality, row-label equality, standing footer, previewed-theme equality, nil cmd, `requireCommitFailedMessage`, the outstanding-flag check) from `TestSlotConfirm_FailedCommitKeepsTheConstant`, leaving it its confirm-specific concerns — the constant surviving in the keys, `requireConfirmGone`, the `applyCommitResult` seam-caller guard, and the landed control — and let the failure-side test own the §9.13 report assertions.
