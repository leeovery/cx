## Attempt 1

ISSUES:
- `internal/tmuxtest/progress.go:51` — `ProgressResult.Changes` is exported, is incremented by new logic,
  and is rendered into every failure message, but no test constrains it. Every sibling field is pinned;
  this one would survive being stuck at zero or off-by-one without a single red test. That matters more
  than it looks: `changes=0` is the exact discriminator used to establish that the residual failures
  never converge rather than converging slowly, and it is what the follow-up race work will triage on.
  A silently-wrong counter would mislead precisely that judgement.
  FIX: add the two assertions to the existing tests — `if got.Changes == 0 { t.Fatalf(...) }` in
  `TestAwaitProgress_ExtendsTheDeadlineWhileTheObservationKeepsChanging`
  (`internal/tmuxtest/progress_test.go:41`, after the `Stalled` check) and
  `if got.Changes != 0 { t.Fatalf(...) }` in
  `TestAwaitProgress_FailsWithinTheCeilingWhenTheObservationStopsChanging` (`progress_test.go:68`),
  following the `t.Fatalf("AwaitProgress returned %s; want …", got)` message shape already used in both.
  Add `"changes="` to the `[]string` substring list at `progress_test.go:95` so the field cannot
  silently drop out of `String()`.
  ALTERNATIVE: delete `Changes` entirely and let `Stalled` carry the distinction. Cheaper, and the task
  never asked for a counter — but it throws away the one number that separates "still moving" from
  "never moved" when `Stalled` is false because the ceiling fired, and it is the number the race triage
  already depends on. Pin it, don't drop it.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/bootstrap/orphan_sweep_integration_test.go:27` — denies that the wait bounds total convergence,
  which the `Ceiling: 45 * time.Second` line three lines below does exactly.
  OLD:
// The daemon population converges through several observable steps (orphans
// dying, the respawned saver re-registering), so this bounds how long it may
// sit unchanged rather than how long the whole convergence may take: a loaded
// machine makes convergence slower without making it wrong.
  NEW:
// The daemon population converges through several observable steps (orphans
// dying, the respawned saver re-registering), so Stall bounds how long it may
// sit unchanged rather than how long the whole convergence takes, and Ceiling
// only backstops a population that churns without ever settling: a loaded
// machine makes convergence slower without making it wrong.

- `cmd/state_daemon_integration_test.go:32` — same denial against the adjacent `Ceiling: 120 * time.Second`.
  OLD:
// Scrollback accumulates continuously, so the budget bounds how long a pane may
// gain no lines rather than how long it may take to fill: on a loaded host the
// seq writer is slower, not stuck.
  NEW:
// Scrollback accumulates continuously, so Stall bounds how long a pane may gain
// no lines rather than how long it may take to fill, and Ceiling only backstops
// a pane that keeps gaining lines without ever reaching the target: on a loaded
// host the seq writer is slower, not stuck.

NOTES:
- Your bimodal evidence holds and was checked independently. `changes=0` across a 50ms tick over 10s
  means ~200 consecutive identical samples; a population genuinely walking 3 to 1 cannot be sampled 200
  times with zero transitions unless it never moved, and a convergence completing between two polls
  would have reached the target. The 60s-stall control is the right experiment. The mechanism checks out
  against source at all three sites you named.
- The full `cmd/bootstrap` integration package was re-run at load average 55 on 10 cores: green, 56.7s.
- On whether the wait can mask a real failure: looked for specifically, not found. `Changes` feeds no
  control flow, so a flapping observation extends to the ceiling and still fails rather than passing.
  Nothing absorbs "no daemon ever appeared" — `count=0` is not the target, so it stalls and fails.
- `withDefaults` plus the three exported `DefaultProgress*` constants serve no current caller. Normally
  speculative, but a zero `Tick` would busy-spin at 100% CPU and a zero `Ceiling` would exit on the
  first poll, so the guard earns its place on a shared helper. No change wanted.
- The unit lane gains ~1.1s, 0.9s of it in `ExtendsTheDeadlineWhileTheObservationKeepsChanging`. Scaling
  that test's durations down 10x would keep the same proof for ~90ms. Not required.
