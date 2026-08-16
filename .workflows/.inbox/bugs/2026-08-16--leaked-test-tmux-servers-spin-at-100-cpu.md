# Integration-test tmux servers outlive the test run and can pin a core indefinitely

Test tmux servers created by `tmuxtest.New` can survive the test run that spawned them, and once orphaned at least one of them wedged and burned a full CPU core continuously for five days without anything noticing.

Found on 2026-08-16 on the developer machine. Three orphaned test servers were running, all reparented to PID 1 with no clients and no children:

```
13198  99.1% CPU  TIME 5372:19  up 5d 02h  tmux -S …/T/ptl-p5-1489450932/s -f /dev/null new-session -d -s _portal-bootstrap
13196   0.0%                    up 5d 02h  (same socket)
 3741   0.0%                    up 13d 04h  tmux -S …/T/ptl-p5-776103860/s …
```

`13198` had accumulated roughly 89 hours of CPU time. It ignored SIGTERM entirely and had to be SIGKILLed, so it would not have gone away on a clean shutdown either. Alongside the servers, 31 stale `ptl-*` directories were left in `TMPDIR` holding ~46MB of built test binaries — the same leak seen from the filesystem side. `ptl-p5-` comes from `cmd/bootstrap/phase5_integration_test.go` and `cmd/bootstrap/phase5_marker_suppression_integration_test.go`; `ptl-bin-` from `internal/restoretest/restoretest.go`.

The socket directories still being present is the significant detail: `tmuxtest.New` (`internal/tmuxtest/socket.go:29-45`) registers a `t.Cleanup` that runs `kill-server` and then `os.RemoveAll(dir)`. The directory surviving means the cleanup never ran at all, rather than `kill-server` having failed — consistent with the test binary being killed outright (Ctrl-C on `go test`, a panic, a timeout kill) so `t.Cleanup` never fires.

Three things make this bite harder than an ordinary temp-file leak. The orphan is invisible: it lives on a private `-S` socket under `TMPDIR`, so it never appears in the developer's `tmux ls` and there is no reason to go looking. It is unbounded in cost: a wedged server pins a core for as long as the machine stays up, which on a 31-day uptime meant days of thermal and battery load attributed to nothing in particular. And nothing reaps it: cleanup is purely per-test `t.Cleanup`, with no run-level or repo-level sweep of stray `ptl-*` sockets, so every interrupted run adds another one permanently.

This sits in the same category the repo already guards elsewhere — `RegisterSubprocessCleanup`, `RegisterStateDirTeardownGuard`, and the daemon-pgrep sandbox all exist to stop test machinery affecting the real system — but the leaked tmux server has no equivalent guard.

Worth separating in triage: the leak is ours, while the 100% spin is tmux's own behaviour once its client vanishes. The spin is what converts a dormant leak into a real cost, but the actionable half is that the server is abandoned in the first place and never swept afterwards.
