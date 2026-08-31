## Attempt 1

ISSUES:
- `cmd/bootstrap/composition_e2e_harness_integration_test.go:114` (with `:57-63`) —
  `state.RegisterSandboxDaemonSource(saver.pid)` now receives the *fallback-bearing* probe. The previous
  inline closure returned `(0, false)` whenever the live `list-panes -t _portal-saver` read failed;
  `pid()` instead returns the cached `p.seen`. That puts a pid the test may no longer own into
  `sandboxFilterPgrep`'s `owned` set — the default-deny allowlist that decides which PIDs an in-process
  orphan sweep may SIGKILL. `internal/state/pgrep_sandbox.go:17-18` states the design rule this crosses
  verbatim: "Ownership is keyed on the state directory, not the pid". The window is real (bootstrap's
  `EnsureSaver` kill-barrier kills and respawns the saver, during which the live read fails and the
  stale dead pid is returned) even though the harm requires PID recycling onto another
  `portal state daemon`. The teardown guard genuinely needs the fallback; the ownership source does
  not, and the task only asked for the former.
  FIX: register the live-only read as the sandbox source and keep the fallback for the teardown wait —
  move the `p.seen = pid` assignment into `livePID()` on the success path, drop it from `pid()`, and
  change line 114 to `state.RegisterSandboxDaemonSource(saver.livePID)`. Ownership then never names a
  stale pid, and `seen` still refreshes on every sandbox enumeration so the teardown fallback survives
  a saver respawn.
  ALTERNATIVE: leave `pid()` as the sandbox source but bound the fallback to a pid the probe has
  verified alive at read time (`state.IsProcessAlive(p.seen)`). Cheaper diff, but it re-tests liveness
  rather than removing the stale-pid class, and still admits the recycled-pid case — the split is
  recommended.
  CONFIDENCE: high

- `internal/portaltest/teardown_guard_coverage_test.go:38-40` — `qualifies()`'s second arm
  (`|| c.Isolates`) is the deliberate widening beyond the task's literal trigger, and it is what keeps
  `cmd/bootstrap/transient_listpanes_helpers_integration_test.go` and
  `cmd/bootstrap/reboot_roundtrip_test.go` inside the rule's reach (both isolate and start a server but
  take their state dir from a shared arrange, so they name nothing). No test in
  `teardown_guard_coverage_rule_test.go` exercises it: the four synthetic fixtures all name
  `PORTAL_STATE_DIR`. Deleting `|| c.Isolates` — the obvious "simplify to what the instruction said"
  edit — leaves the whole suite green while silently shrinking the guard, which is the exact failure
  mode this task exists to close.
  FIX: add a fifth synthetic fixture to
  `internal/portaltest/teardown_guard_coverage_rule_test.go` that calls
  `portaltest.IsolateStateForTest` and `tmuxtest.New` with **no** `PORTAL_STATE_DIR` literal and **no**
  guard call, and assert `scanned == 1` with it reported uncovered — pinning both that the arm
  qualifies the file and that the coverage check still bites. Name it in the style of the existing
  four, e.g. `TestCoverageRuleFailsIsolatedFixtureTakingItsStateDirFromASharedArrange`.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/portaltest/isolated_env.go:32-34` — the claim covers `XDG_CONFIG_HOME`, which does *not*
  reach the returned slice through the `os.Environ()` read: `filterXDGConfigHome` strips that entry at
  line 71 and line 72 appends the config-dir value explicitly. Its own ordering requirement is the
  `resolveDevStateDir` read, already stated by the comment above it.
  OLD:
	// Shells hosted by a test's tmux server flush their history into HOME as
	// they exit, racing the framework's RemoveAll of that temp dir. Both this
	// and the two above reach the returned env slice through the os.Environ()
	// read below, so they must be set before it.
  NEW:
	// Shells hosted by a test's tmux server flush their history into HOME as
	// they exit, racing the framework's RemoveAll of that temp dir. This reaches
	// the returned env slice through the os.Environ() read below, so it must be
	// set before it.

- `internal/portaltest/teardown_guard_coverage_test.go:55-57` — "rather than the isolation call" reads
  as exclusive, but `qualifies()` at line 39 fires on the state dir **or** the isolation call.
  OLD:
// The trigger is the state directory rather than the isolation call, because a
// trigger keyed on the helper can only ever fire on files that already opted in
// — and skipping that call is exactly the defect worth catching.
  NEW:
// The state directory is a trigger in its own right, alongside the isolation
// call, because a trigger keyed only on the helper can fire on files that
// already opted in — and skipping that call is exactly the defect worth
// catching.

NOTES:
- The two BANK items in the executor's report are accurate and confirmed:
  `internal/restore/multipane_legacy_integration_test.go:31,156` and
  `internal/restore/rename_reboot_shared_test.go:58` call `portaltest.IsolateStateForTest(t)` and
  discard both return values, then point `PORTAL_STATE_DIR` at a separate bare `t.TempDir()`. The
  widened guard cannot see this (all three calls are present per file), so it is a genuinely distinct
  rule, not a regression here.
- The `reboot_roundtrip_test.go` consolidation is a larger win than the report frames it: the removed
  second `IsolateStateForTest` call was overwriting `PORTAL_TEST_SANDBOX_REGISTRY` with a registry
  naming a state dir the test never wrote to, so any subprocess orphan sweep in those three round-trip
  tests was operating with zero owned dirs.
- `saver.sock` is assigned after the probe closure is registered
  (`composition_e2e_harness_integration_test.go:96` then `:99`). Correct as written since the closure
  captures the pointer, but a cleanup running before line 99 would see a nil socket. Harmless today —
  `tmuxtest.New` is the very next statement.

## Attempt 2

ISSUES:
- `cmd/bootstrap/composition_e2e_harness_integration_test.go:59-84` — the asymmetry that makes the
  round-2 fix safe is asserted only by a comment. `livePID` must never yield a remembered pid (it feeds
  the SIGKILL allowlist at `:120`) while `pid` must yield one (it feeds the post-`kill-server` wait);
  nothing fails if a future edit collapses the two. This is the one property standing between the
  daemon-kill allowlist and a pid the test no longer owns, and the repo's own convention is to prove
  sandbox-ownership properties with a test — CLAUDE.md cites `TestPgrepSandbox_ExcludesUnregisteredPID`
  as the structural proof of the exclusion side; the source side has no counterpart. The residual harm
  today is bounded (a stale pid can only be killed if `pgrep -fx '^portal state daemon( |$)'`
  independently confirms it is a live portal daemon), so this is a preventive gap, not a live defect —
  but it is the highest-risk seam in the phase and the fix is local.
  FIX: add a short unit-style test beside the harness (same file, or a sibling `_integration_test.go`
  in `bootstrap_test`) that constructs `&saverPaneProbe{seen: 4242}` with a nil `sock` and asserts
  `livePID()` returns `ok == false` while `pid()` returns `(4242, true)` — the exact divergence, with a
  failure message naming the allowlist consequence. No tmux server needed, so it costs nothing in lane
  time.
  ALTERNATIVE: drop the latch from `livePID` and refresh `seen` only through `observe`, called once
  more from a `t.Cleanup` registered immediately *after* `tmuxtest.New` (LIFO runs it while the server
  is still alive). That removes the hidden side effect in a callback handed to
  `RegisterSandboxDaemonSource` and makes the last-live read explicit, but it adds a cleanup whose
  ordering is itself load-bearing and unpinned — so it trades one silent invariant for another. The
  test is recommended.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- `cmd/bootstrap/composition_e2e_harness_integration_test.go:45-48` — "Its two readers" is a
  cardinality claim, falsified by any additive caller.
  OLD:
  // saverPaneProbe reads the PID of the live _portal-saver pane, remembering the
  // last one it saw. Its two readers want different things from a failed read, so
  // they take different methods: livePID reports only what is live right now, and
  // pid falls back to the remembered one.
  NEW:
  // saverPaneProbe reads the PID of the live _portal-saver pane, remembering the
  // last one it saw. Its readers want different things from a failed read, so they
  // take different methods: livePID reports only what is live right now, and pid
  // falls back to the remembered one.

- `internal/portaltest/teardown_guard_test.go:12-13` — `cleanupRecorder` is a struct that implements
  the already-narrowed `cleanupT` and records; it narrows nothing, and the claim duplicates
  `cleanupT`'s own doc comment.
  OLD:
  // cleanupRecorder narrows *testing.T to the one method the guard registers
  // through, so a test can run the cleanup on demand instead of at test end.
  NEW:
  // cleanupRecorder stands in for *testing.T at the guard's registration seam,
  // holding the cleanups so a test can run them on demand instead of at test end.

NOTES:
- Your justification for the `|| Isolates` widening is now half stale: `reboot_roundtrip_test.go` lost
  its direct `IsolateStateForTest` call in the same change, so it is out of the scan regardless. The
  widening is still load-bearing for `cmd/bootstrap/transient_listpanes_helpers_integration_test.go`
  (isolates x5, names no `PORTAL_STATE_DIR`), so the decision stands — only the second example cited no
  longer applies. Worth correcting if it is written down anywhere as a rationale.
- The `scanned == 0` failure text (`teardown_guard_coverage_test.go:150-151`) reads "no file pairs
  `PORTAL_STATE_DIR` with `tmuxtest.New`", naming only one of the trigger's two arms. Harmless, but
  slightly misleading to whoever eventually trips it.
- `livePID`'s latch of `p.seen` is a side effect its name does not advertise, and it is invoked from
  inside `sandboxFilterPgrep`'s mutex on an arbitrary call path. Correct as written (no concurrency in
  the composite suite), but worth keeping in mind if any composite test ever grows a goroutine.
