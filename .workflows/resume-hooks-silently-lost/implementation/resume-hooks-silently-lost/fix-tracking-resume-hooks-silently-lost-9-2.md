## Attempt 1

ISSUES:
- `internal/restore/multipane_legacy_integration_test.go:35,36,71` still compose `tmux.SessionTargetExact` for `rename-session -t` and `has-session -t` — the two commands this task measured as broken in that form. They pass only because the fixture names carry no period and no prefix sibling is live, and nothing will flag them: the composition guard allowlists `SessionTargetExact` and does not scan test files. The audit reached the peer test helper (`internal/tmuxtest/socket.go:120` moved for exactly this reason) but not these, so the change is internally inconsistent — and they falsify the "no call site takes it" claim the same diff added to `internal/tmux/tmux.go:419` and CLAUDE.md.
  FIX: move all three to `tmux.CoordTargetExact` (identical to the `internal/tmuxtest/socket.go:120` edit). The fixture then composes the same wire form production does, and stays correct if a fixture name ever gains a period.
  ALTERNATIVE: leave them and reword both claims to "no production call site" instead. Cheaper, but it keeps a known-defective target form live in a real-tmux fixture for the very commands the task moved, and leaves the only remaining consumers of the helper being ones that shouldn't be. The reviewer recommends moving them.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- internal/tmux/tmux.go:419 — a cardinality claim of exactly the kind code-quality.md forbids ("nothing consumes this yet"), falsified today by the three call sites above and by any future additive change far from the comment. The paragraph's measured content stands on its own without it.
  OLD: // No call site takes it today, and that is a measured result rather than an
// oversight. Every per-session command Portal runs — has-session, kill-session,
  NEW: // Every per-session command Portal runs — has-session, kill-session,
- internal/tmux/exact_session_target_test.go:25 — stale: the diff collapsed the two form constants above into the single `coordTargetForm`, but this line still speaks of two.
  OLD: // invoked against; the two target forms above are its exact renderings.
  NEW: // invoked against; the coordinate form above is its exact rendering.
- CLAUDE.md:60 — same false cardinality claim in the `tmux` row.
  OLD: SessionTargetExact` (`=name`) is therefore currently **used by no call site**;
  NEW: SessionTargetExact` (`=name`) is therefore taken by no production call site;

NOTES:
- `perSessionRoutes` (`internal/tmux/exact_session_target_test.go:42`) is the table that keeps the mock-level and real-tmux target guards in step, and it still enrols only the eight routes that were already on the coordinate form. The six moved here (HasSession, HasSessionProbe, KillSession, RenameSession, SwitchClient, ListClients) rely on their own scattered argv assertions instead. Now that every per-session route takes one form, enrolling them would put the moved sites under the standing guard — worth considering, though the table is documented as a maintained set rather than the whole surface, so this is a pre-existing scoping choice rather than a regression.
- `ListClients` swallows every failure into `([], nil)`, so the negative assertion at `internal/tmux/period_session_target_realtmux_test.go:129-132` cannot distinguish "resolved, no clients" from "target failed". The positive assertion two lines above carries the real weight; no change needed, but the negative one proves less than it reads.
- Retaining `SessionTargetExact` as a now-unused exported helper is task-sanctioned (the Do list presupposes it survives), but it stays in the composition guard's allowlist — so a future hand-composed `=name` would pass the standing guard silently while being the form no measurement supports.
- CLAUDE.md's lane carve-out sentence still names `internal/tmux/*_realtmux_test.go` as the unit lane's real-tmux set; two such tests now live in `internal/state` (one pre-dating this change). Pre-existing drift, not this task's to fix.

## Attempt 2

ISSUES:
- `internal/tmux/clients_test.go:122` — the subtest "it targets the session exactly and requests pid+activity" still asserts `strings.Contains(args, "-t =dev")`, which is satisfied by both the old `=dev` and the new `=dev:`, so the one site this task moved outside `tmux.go` has no unit-level pin on its wire form, and the failure message still names `=dev` as "the session exactly" — the very form this task established is not exact. Every other moved site got an exact expectation updated (`tmux_test.go:345,443,630,696,870,1411`, `session_name_test.go:64`, `cmd/open_test.go`, `quickstart_test.go`). Real-tmux coverage does catch a revert (the period suite's `ListClients(renamed)` would return zero clients), but the assertion that exists to pin the form no longer can.
  FIX: replace the three substring checks with one exact argv comparison — `want := []string{"list-clients", "-t", "=dev:", "-F", "#{client_pid} #{client_activity}"}` compared with `slices.Equal(mock.Calls()[0], want)`, reporting `got`/`want`. Note `strings` is used only by this subtest (lines 118-126), so the import drops and `slices` is added.
  ALTERNATIVE: minimally change the substring to `"-t =dev:"` and the message to match. Cheaper, but leaves a partial-match assertion where the composed argv is fully deterministic — the exact comparison is what the rest of the package does and is what the reviewer recommends.
  CONFIDENCE: high

COMMENT_CORRECTIONS: none.

NOTES:
- `SessionTargetExact` now has zero production callers; it survives referenced only by its own form test and the composition guard's allowlist. Retention is task-conformant. Worth flagging for the phase boundary rather than now: the guard still admits `SessionTargetExact` as a legal composition, so a future call site can reintroduce exactly this defect class and stay green. Tightening that (allowlist narrowing, or the `type Target string` move the doc already names as the settled answer) is a larger change than this task.
- Four integration fixtures still hand-compose the bare probe form — `cmd/state_daemon_self_supervision_integration_test.go:378`, `cmd/noncontiguous_window_reboot_integration_test.go:95`, `internal/restore/rename_reboot_durability_integration_test.go:24`, `internal/restore/rename_reboot_hook_integration_test.go:75` (`"has-session", "-t", "="+name`). None of those fixture names carries a period, so they are inert, and the diff did not touch them. They are now the only test probes composing a form production no longer uses.
- The two capture fakes' `sessionFromExactTarget` now strips a trailing `:` but still resolves a bare `=name`, so those fakes tolerate the old form. The exact-form pins live in the wire-form tests, not the fakes, but it means the `cmd`/`state` capture suites are not themselves regression detectors for this move.
- The Go doc on `CoordTargetExact` omits `respawn-pane` from its enumeration while the CLAUDE.md row includes it. Neither statement is false, so no correction is warranted — but the two enumerations the task asked to keep in step are not identical.
