# Saver respawn can leave the install with no daemon at all

A `_portal-saver` respawn intermittently ends with no `portal state daemon` running anywhere, no saver session, and nothing that retries or re-checks. The CLI reports success. Discovered while investigating what was assumed to be a timing flake in the `cmd/bootstrap` composite-e2e suite; the measurement disproved that premise and surfaced this instead.

## What happens

The saver respawn launches a new daemon while the outgoing saver daemon is still mid-SIGHUP shutdown and still holding `daemon.lock`. The new process stands down four milliseconds in — one millisecond before the outgoing daemon completes its flush and releases the lock — and exits 0. Nothing retries. Because that process *was* the saver pane's process, tmux destroys the pane, and with it the session.

From a failing run's own `portal.log`:

```
19:39:27.437113 INFO daemon: lock acquired                              pid=72243
19:39:27.704453 INFO process: start args="state daemon"                 pid=72292
19:39:27.708076 WARN daemon: another daemon holds the lock; exiting     pid=72292
19:39:27.708133 INFO process: exit code=0 took=4.049459ms               pid=72292
19:39:27.709059 INFO daemon: shutdown reason=sighup flush_completed=true pid=72243
```

## Observable end state

Captured live at failure time: `list-panes -t _portal-saver` answers `can't find window: _portal-saver`; `capture-pane` answers `can't find pane`; `list-sessions` shows no `_portal-saver` at all; `daemon.pid` still names the pid of an orphan the sweep had already killed; and `PgrepPortalDaemons()` returns empty, permanently. The capture loop is simply gone — sessions stop being saved and nothing surfaces that.

## Reproduction

`go test -tags integration -p 1 -count=N -v ./cmd/bootstrap -run 'TestCompositeBootstrap_FObservables'` on a loaded machine (10 cores, load average 20–41; ambient workload was enough, no synthetic load needed). Fires roughly 1 in 8 executions. Any member of the composite family can trip it; `FObservables` and `ConvergesPgrepToOneDaemon` tripped most often.

The signature is bimodal and that is the decisive evidence it is not slowness: with the wait widened to 60 seconds across 14 executions, convergence was either 36–42 ms or never — three runs sat unchanged for the full 60 seconds. Six, ten and sixty seconds of waiting produce the identical verdict.

## Where the ordering sits

Between `internal/tmux/portal_saver.go:300`, where `BootstrapPortalSaver`'s `RespawnPane` launches the new daemon with no wait for the outgoing one to release the lock — its follow-up readiness barrier `waitForSaverDaemonReady` (same file, ~`:226`) is 2s and WARN-only, returning nil, so the vanished daemon never becomes an error at the bootstrap layer — and `cmd/state_daemon.go:113-119`, where `defaultDaemonRun`'s single-shot `acquireDaemonLock` treats `ErrDaemonLockHeld` as a WARN and a bare `return nil`, with no retry and no backoff. (`AcquireDaemonLock` itself is `internal/state/daemon_lock.go:36`; its retry loop covers the inode race, not a held lock.)

One caveat on the evidence: the daemon-pgrep sandbox is default-deny keyed on the state dir's `daemon.pid`, so a live-but-unregistered daemon would also read as zero. Independent tmux evidence excludes that for the one run traced in full; the never-moved shape is established across roughly 45 runs, the mechanism on one.
