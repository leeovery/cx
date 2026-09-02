## Attempt 1

ISSUES:
- `internal/portaltest/tmux_server_wait_test.go:8` and `:30` — the two named test cases are satisfied in name only. Both drive `awaitServerGone` (`tmux_server_wait.go:27-30`), which is a one-line delegate to `tmuxtest.PollUntil`, with a fake counter for `gone`; they are near-verbatim duplicates of `internal/tmuxtest/poll_test.go:9` and `:28`, which already pin exactly those two behaviours of `PollUntil`. Neither test reaches the code this task actually introduced. `tmuxServerUnreachable` (`tmux_server_wait.go:34-38`) has no coverage at all, and it is new code, not a verbatim move: it replaces the previous route through `ts.TryRun` with a hand-rolled `exec.Command("tmux", "-S", socketPath, "-f", "/dev/null", "list-sessions")`, restating the argv that `tmuxtest.socketArgs` (`internal/tmuxtest/socket.go:16`) already owns. The failure mode is silent and self-concealing: if that argv were wrong, *every* probe would error, `gone` would be true on its first call, `AwaitTmuxServerGone` would return instantly, and the coldboot suite would quietly lose the wait this task just promoted — with no test failing and only an intermittent teardown flake to show for it. The `awaitServerGone` seam exists solely to enable these two tests and buys nothing else.
  FIX: Point the tests at the real helper. In `tmux_server_wait_test.go`, add a case that guards `tmuxtest.SkipIfNoTmux(t)`, stands up `ts := tmuxtest.New(t, "ptl-await-gone-")` with one `new-session -d … sleep infinity`, asserts `tmuxServerUnreachable(ts.SocketPath())()` is **false** while the server answers (this is the half that pins the argv and the error⇒gone direction), then calls `ts.KillServer()` and asserts `AwaitTmuxServerGone(t, ts.SocketPath())` returns with the probe now reporting true. A real-tmux client test is sanctioned in the unit lane (precedent: `internal/tmux/*_realtmux_test.go`), builds no binary and spawns no daemon, so no `integration` tag is needed. To keep the bound covered without a hang, give `AwaitTmuxServerGone` an unexported `awaitTmuxServerGone(t, socketPath, budget, tick)` that it delegates to with the package constants, and drive the bound case with a live server and a 100ms budget — that pins the bound over the real probe rather than over a fake. Then delete `awaitServerGone` and its two fake-driven tests, which the above supersedes.
  ALTERNATIVE: Keep `awaitServerGone` and its two tests as they stand and add only the single real-tmux case for `tmuxServerUnreachable`. Cheaper and less churn, but it leaves the duplicate coverage of `PollUntil` in place and leaves `AwaitTmuxServerGone`'s own budget/tick wiring untested. The reviewer recommends the first: the seam's only justification was tests that do not test this package's code, so removing both is the smaller end state.
  CONFIDENCE: high (on the gap), medium (on which of the two shapes)

COMMENT_CORRECTIONS:
- `internal/portaltest/isolated_env.go:45-47` — states a mechanism the `testing` package does not have: `homeDir := t.TempDir()` on the line above registers no cleanup at all when the fixture called `TempDir` earlier (which the task's own named fixture does, via `BuildPortalBinaryDir`), and the kill-server half of the ordering comes from the coverage rule, not from that call.
  OLD:
  	// Registered after the t.TempDir above owns homeDir's RemoveAll, so LIFO runs
	// this wait between a fixture's kill-server and that RemoveAll — the writers
	// the env pins above cannot reach are given their window to finish.
  NEW:
  	// Registered after the test's first t.TempDir, whose lone cleanup removes the
	// parent holding homeDir, so LIFO runs this wait before that RemoveAll; a
	// fixture must isolate before starting its server, so kill-server runs earlier
	// still. The writers the env pins above cannot reach get their window to finish.

NOTES:
- The placement deviation was judged and upheld: the LIFO ordering claim holds, but not for the reason the code states — Go's `testing` registers the `TempDir` RemoveAll cleanup exactly once, on the test's *first* `TempDir()` call, over the shared parent. The conclusion survives regardless (that registration is necessarily at or before the `homeDir` line, so LIFO always runs the guard first), and the kill-server half holds because the coverage rule forbids starting a server before isolating. Hence the comment correction.
- Cost of registering for every isolated test: measured, negligible, no correctness consequence. ~50ms per call site at cleanup; only ~9 of the 75 sites are unit-lane, so the fast lane gains under half a second and the integration lane ~3.5s against a multi-minute wall time.
- Ten-run claim verified independently, and under load average 134 (a stress condition, not idle): all rc=0, no `TempDir RemoveAll` line.
- The "every teardown-guard file also isolates" survey was verified and strengthened: `teardown_guard_coverage_test.go`'s source rule already fails any fixture that starts a tmux server without isolating first, so the correspondence is structurally enforced rather than incidental.
- `TestDirQuiescenceGuardReturnsOnceTheDirStopsChanging` (`teardown_guard_test.go:88`) asserts only that the guard does not burn its budget over a quiescent dir — replacing `awaitDirQuiescent`'s body with `return` would still pass it. Mirrors the pre-existing state-dir test exactly, so it follows convention rather than breaking it; recorded because the shared `awaitDirQuiescent` now carries both guards and neither pins its wait.
- The `ZDOTDIR=homeDir` pin is inert for the writer it names: `HOME` is already the temp dir, so zsh resolves its rc files there either way, and macOS's `.zsh_sessions` is written by `/etc/zshrc_Apple_Terminal` keyed on `SHELL_SESSIONS_DISABLE` and `$HOME`, not `ZDOTDIR`. Harmless, and what the task asked for; `SHELL_SESSIONS_DISABLE=1` is the pin that bites.
- Neither guard covers the whole removal target. Go's single RemoveAll spans the shared parent, so it also takes the config `TempDir` and every fixture-local `TempDir`. No evidence any of them races today; noted since the guards read as if they covered the RemoveAll.
- Independent confirmation of the diagnosis fell out of the ordering analysis: with `BuildPortalBinaryDir` taking `001`, `homeDir` is `002` in that fixture — the directory the reported `unlinkat .../002: directory not empty` names.

## Attempt 2

ISSUES:
- `internal/portaltest/tmux_server_wait_realtmux_test.go:44-56` — the subtest named `"it blocks until the tmux server is unreachable"` does not test blocking, and nothing else covers the exported `AwaitTmuxServerGone`. `ts.KillServer()` runs synchronously and returns with the server already unreachable, so the wait returns on its first probe whether it polls or not. Proved by overlay mutation: replacing the whole body of `AwaitTmuxServerGone` with a no-op leaves all three subtests green (subtest 2 exercises the unexported `awaitTmuxServerGone` directly, so it does not cover the wrapper's budget/tick wiring either). The one function `cmd/concurrent_coldboot_integration_test.go:66` depends on can be gutted with the suite still passing — the same self-concealing shape the prior round named, moved one level up.
  FIX: make the server's death asynchronous relative to the call, in the same subtest. Replace the leading `ts.KillServer()` with a scheduled kill and assert the wait spent that time:
  ```go
  const settle = 300 * time.Millisecond
  go func() {
      time.Sleep(settle)
      ts.KillServer()
  }()

  start := time.Now()
  AwaitTmuxServerGone(t, ts.SocketPath())
  elapsed := time.Since(start)

  if elapsed < settle {
      t.Fatalf("the wait returned after %s, before the server exited at %s", elapsed, settle)
  }
  ```
  keeping the existing `!probe()` and `elapsed >= tmuxServerGoneBudget` assertions after it. The reviewer ran this exact edit through `go test -overlay`: green against the real implementation, and it fails with `the wait returned after 1.875µs, before the server exited at 300ms` against the gutted wrapper. Adds ~300ms to the unit lane.
  ALTERNATIVE: seed the session as `new-session -d -s await-gone sleep 0.3` in a second `tmuxtest.Socket` and let the server self-exit when its last session ends (tmux's `exit-empty` default), with no goroutine. Cleaner control flow, but it costs a second server and couples the test to `exit-empty`; the reviewer recommends the scheduled kill.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/concurrent_coldboot_integration_test.go:61-62` — the rewritten summary claims a wait the code does not perform: the helper waits for the *server* to stop answering, and a pane shell can outlive it (which is precisely why the HOME quiescence wait exists as well).
  OLD:
  // reapTmuxServer kills the whole server, not just a session, so every pane
  // shell gets SIGHUP, then waits the shells out.
  NEW:
  // reapTmuxServer kills the whole server, not just a session, so every pane
  // shell gets SIGHUP, then waits for the server to stop answering.

NOTES:
- What the fix round did pin, checked by mutation rather than reading: rewriting the probe's `-S <socketPath>` to a bogus `-L` name fails two subtests; inverting the `!= nil` error direction fails all three. Argv drift and direction inversion are caught. The gap is only the exported wrapper.
- The `.zsh_sessions` writer named in the task is unlikely to be the actual one on this machine, though the outcome is achieved regardless. `/etc/zshrc_Apple_Terminal` (which owns `.zsh_sessions` and does honour `SHELL_SESSIONS_DISABLE=1`) is sourced by nothing under `/etc` here, while `/etc/zshrc` — which every interactive zsh reads — unconditionally reassigns `HISTFILE=${ZDOTDIR:-$HOME}/.zsh_history`, overriding the `HISTFILE=/dev/null` pin and writing into the temp HOME. The env pins are therefore defensive rather than operative, and the quiescence wait is what closes the race. Banked.
- `ZDOTDIR=<temp HOME>` is a no-op unless a host `ZDOTDIR` is inherited (zsh defaults it to `$HOME`, already the temp dir). The decoy in the env test makes the inherited-value case the pinned one, which is the case worth pinning.
- `awaitDirQuiescent`'s waiting direction remains untested — a no-op body passes both the new dir-quiescence test and the pre-existing state-dir one, since both only assert a fast return over a quiescent dir. Moved-verbatim pre-existing code, so inherited rather than introduced, but the new HOME wait now rests on it; a background writer touching the dir every 100ms for 400ms would pin it in one test for both guards.
- `TestIsolateStateForTest_RegistersAQuiescenceWaitOverTheTempHome` and `captureHomeQuiescenceGuard` sit in `teardown_guard_test.go`, while the sibling `TestIsolateStateForTest_*` tests and the near-identical `captureBackstop` helper live in `backstop_ordering_test.go`. Both are internal-package files, so either placement works; the split just means two files now answer "what does isolation register".
- LIFO placement was verified structurally: `t.TempDir()` registers exactly one parent-`RemoveAll` cleanup on its first call and the guard is registered after it, while the coverage rule already fails any fixture starting a server before isolating — so kill-server → state-dir guard → HOME wait → RemoveAll holds repo-wide rather than by convention.
- Ten-run and env-pin claims re-verified independently by mutation (removing the two `t.Setenv` lines fails both subtests, naming the decoy value).
