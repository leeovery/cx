## Attempt 1

ISSUES:
- internal/tmux/exact_session_target_realtmux_test.go:38-42 — seedPrefixSiblingServer creates a session with one window holding one pane, so the live-session assertions for the three list-panes sites (:151-159, :161-171, :173-183) assert len == 1 and cannot distinguish "resolved the whole session" from "resolved only the current window". Whole-session scope is the entire open question about the new =<session>: coordinate form, and it is the one thing the argv table cannot answer — a form change is caught by the exact-string unit table, so tmux-side scoping drift is the only failure this real-tmux layer uniquely exists to catch, and the fixture is blind to it. Blast radius is real: internal/restore/session.go:121 pairs saved panes positionally against ListPanesInSession output, and per the spec a token written onto the wrong pane does not self-correct. This repeats the class of gap the amendment was written to close — a fixture where the correct and the broken answer coincide.
  FIX: extend seedPrefixSiblingServer to give prefixSibling a second window with a split pane (new-window + split-window against the exact target), then assert the full set: ListPanesInSession returns all four coords across both windows, and ListWindowsAndPanesInSession returns two WindowGroups with the expected PaneIndices. Note that ListPanes passes no -s, so its expectation must become the current window's panes rather than siblingPaneKey — which is worth pinning explicitly, since it documents the no--s semantics the shared helper otherwise leaves implicit.
  ALTERNATIVE: leave the shared seeder single-window and add a separate multi-window seeder used only by the two -s subtests. This keeps ListPanes/ShowEnvironment/SetSessionOption expectations untouched at the cost of a second fixture. The reviewer recommends the first: one fixture that exercises the real shape keeps the ListPanes no--s contrast visible in the same file, and the extra window costs nothing at this test's runtime.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- internal/tmux/tmux.go:426-429 — the paragraph attributes one failure mode to both commands, but measurement shows they differ, and it omits the more dangerous of the two: list-panes -t '=foo' does fall through and reach the prefix sibling, whereas display-message -p -t '=foo' returns empty with exit 0 and never reaches it — including for a live session, which is the regression that blocked this task.
  OLD:
// It is the wrong helper for a `-t` tmux parses as a window or pane target
// (list-panes, display-message): there a bare "=foo" is read as a window or
// pane name, misses, and falls through to the same fuzzy lookup, so the prefix
// sibling is reached anyway. Route those through exactCoordTarget.
  NEW:
// It is the wrong helper for a `-t` tmux parses as a window or pane target:
// there a bare "=foo" is read as a window or pane name and misses. list-panes
// then falls through to the same fuzzy lookup and reaches the prefix sibling
// anyway; display-message instead returns empty with exit 0 — for a live
// session as much as a gone one. Route those through exactCoordTarget.

NOTES:
- The banked saver_pane_pid.go sites are genuinely broken, not merely stylistically inconsistent: measured list-panes -t '=sib' returning the prefix sibling's pane, so exactTarget at :13 and :37 pins nothing. The new exactTarget doc comment now names that violation explicitly.
- ActivePaneCurrentPath's doc claim that a mid-read kill surfaces as ErrNoSuchSession is false on tmux 3.7c and was false before this change. Correctly scoped out and banked. dirresolve.go:41's empty-string guard means the caller behaves identically either way.
- internal/restore integration flaked twice on the reviewer's runs (TestMultiPaneLegacy_PerPaneHookRouting, then TestRenameRebootHook_PaneProcessKeptRunning, both "hook fire file absent"), then passed twice clean, each failure hitting a different test. Unrelated to this change. Task 6-16 scopes this fix.
- The task text's :488 label (ListAllPanesWithFormat) was a mislabel — that method composes no -t at all; the line pointed at ListWindowsAndPanesInSession, which is what was covered. Seven sites, all seven routed.

## Attempt 2

ISSUES:
- internal/tmux/tmux.go:794 — SelectLayout composes fmt.Sprintf("%s:%d", session, window) and passes it bare to select-layout -t. Measured: with sib renamed away and sib-2 live, select-layout -t 'sib:0' even-horizontal returns exit 0 and rewrites sib-2's window layout (c195,80x24,... -> 8205,80x24,...); -t '=sib:0' fails cleanly with can't find session: sib, and -t '=sib-2:0' works for the live session. This is a silent wrong-session WRITE, the exact failure class the task exists to remove, and it violates the task's own criterion 1 and Outcome ("every session-level target it composes"). It is not a PaneTarget/PaneTargetExact call site (it inlines its own fmt.Sprintf), so the task's fence does not cover it, and it sits directly above SelectWindow, which does pin exactly.
  FIX: change line 794 to target := fmt.Sprintf("=%s:%d", session, window) — but keep the un-prefixed form for the error message, matching the bareTarget/target pairing SelectWindow (:803-805), SelectPane (:817-818) and ResizePaneZoom (:831-832) already use, so "failed to select-layout work:1" does not start printing "=work:1". Update internal/tmux/tmux_test.go:2284 (want becomes "=work:1"), and add a SelectLayout subtest to both halves of exact_session_target_realtmux_test.go: gone-session asserts the call errors AND that display-message -p -t '=sib-2:0' '#{window_layout}' is unchanged across it (the argv assertion alone cannot see the wrong-session write); live-session asserts a layout change lands on sib-2.
  ALTERNATIVE: extract a windowTargetExact(session, window) helper next to exactCoordTarget and route both SelectLayout and SelectWindow (:805, which hand-rolls "=" + bareTarget) through it. Costs a third helper but removes the ad-hoc "=" concatenation and gives the window-target form the same documented home as the other two.
  CONFIDENCE: high

- internal/tmux/exact_session_target_realtmux_test.go:118-129 — acceptance criterion 2 ("each of the seven sites returns the same error class for a genuinely missing session") is asserted nowhere. Every existing ErrNoSuchSession test (internal/tmux/errors_test.go, realcommander_test.go:66, tmux_test.go:3176, cmd/state_daemon_capture_logging_test.go:86) hand-supplies the "no such session" stderr to a mock, so none can observe what real tmux emits under the new =name wire form. That chain is load-bearing: internal/state/capture.go:71 discriminates ShowEnvironment's error to separate natural session churn from anomaly, and :87-89 aborts the whole capture commit when every session fails as anomalous. Measured that show-environment -t '=ghost' still says "no such session: =ghost", so the code is correct today — but the one assertion that would catch a future drift is absent, and the fixture already has the error in hand.
  FIX: in the ShowEnvironment subtest of TestSessionTargets_GoneSessionDoesNotReachPrefixSibling, replace the bare err == nil fatal with a class assertion — if !errors.Is(err, tmux.ErrNoSuchSession) { t.Fatalf(...) } — which subsumes the nil check and pins the sentinel against real tmux. Add "errors" to the file's imports. Optionally pin the other side: the ActivePaneCurrentPath gone-session subtest currently permits both ("", nil) and an error; asserting err == nil && got == "" would pin that site's class explicitly rather than by omission.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- internal/tmux/tmux.go:558-559 — the doc claims every pane in the session, but the call composes list-panes without -s, which addresses one window; the new test pins exactly the opposite (siblingCurrentWindowPaneKeys, "the current window's panes, not the session's"). Measured: list-panes -t '=sib-2:' returns 1:0 1:1 while -s returns all four. Pre-existing, but the diff touched this function's body and the task requires its doc comments to be true.
  OLD: // ListPanes returns the structural key of every pane in the named session.
// Unlike pane IDs, these survive a server restart.
  NEW: // ListPanes returns the structural key of every pane in the named session's
// current window: list-panes without -s addresses one window. Unlike pane IDs,
// these keys survive a server restart.

- internal/tmux/exact_session_target_realtmux_test.go:22-26 — the trailing clause is both a claim about what another test's assertions cannot prove and a cardinality claim ("the one thing"), which code-quality.md forbids; the preceding sentence already carries the whole rationale for the fixture's shape.
  OLD: // The shape seedPrefixSiblingServer builds: two windows of two panes each, with
// the second window current. A single-window sibling would let a form that
// resolves only the current window pass as if it had resolved the whole
// session, which is the one thing the composed-argv assertions cannot tell
// apart.
  NEW: // The shape seedPrefixSiblingServer builds: two windows of two panes each, with
// the second window current. A single-window sibling would let a form that
// resolves only the current window pass as if it had resolved the whole
// session.

NOTES:
- ListPanes (tmux.go:560) has no production callers — only tests reach it. Routing it was required by the task and is correct; flagged only so a later dead-code sweep knows it is exported-but-unconsumed.
- sessionFromExactTarget is declared twice with identical bodies (cmd/state_daemon_capture_logging_test.go:262, internal/state/capture_test.go:64). Two instances across two packages is under the Rule of Three; correct call to leave duplicated.
- ActivePaneCurrentPath's ErrNoSuchSession doc claim is confirmed false against real tmux and was equally false before this change. Already banked; correctly untouched. internal/session/dirresolve.go:41-43 guards the empty-string return.
