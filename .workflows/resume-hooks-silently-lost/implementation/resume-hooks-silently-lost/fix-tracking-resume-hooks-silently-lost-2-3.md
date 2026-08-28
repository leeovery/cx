## Attempt 1

ISSUES:
- `internal/tmux/tmux.go:248-251` — the `display-message` read-failure branch has no test. It was covered by `TestResolveHookKey_ReadFailureWrapsError`, deleted in this change and not replaced; coverage of `ResolveHookKey` fell from 100% to 83.3% with that block at zero hits. The branch is live in production (a pane closing between the probe and the read — the SessionEnd race the spec calls routine) and holds the invariant that a failed read must not surface as a resolved key.
  FIX: Add a third subtest to `TestResolveHookKey_ProbeOrdering` in `internal/tmux/resolve_hookkey_test.go`, following the two existing subtests: a `MockCommander` whose `RunFunc` returns `("", nil)` for `show-options` and `("", &tmux.CommandError{Stderr: "..."})` for `display-message`; assert the key is `""`, the error is non-nil, and `errors.As(err, &cmdErr)` recovers `*tmux.CommandError`. No server, no new fixture.
  ALTERNATIVE: reinstate a real-tmux server-down case. It exercises the wrap through the real `Commander`, but the failure mode it drives (no server) is not the scenario the branch exists for (pane dies mid-sequence), and it costs a server per run. The fake models the actual race directly — the reviewer recommends the fake.
  CONFIDENCE: high

NOTES:
- On the dropped server-down case: the reviewer agrees the server-down *flavour* asserts nothing distinct post-probe, but the executor's stated reason was inverted, and the consequence was missed — the second read's error branch is now untested.
- The two `cmd` propagation tests do exercise the real command body: swallowing the resolver error in `resolveCurrentPaneKey` makes both fail. The executor's claim holds.
- The no-option probe rule was independently reproduced on an isolated `-L` socket (tmux 3.7c). Making the probe name the option collapses the discrimination and is caught by the real-tmux subtests, exactly as the task predicted a fake could not.
- The probe's wrap text `no pane answers to %q` asserts a cause the exit status does not prove — a server-down or transient fault renders the same phrase. Harmless in practice.
- The `cmd` test proves "writes nothing" by asserting `hooks.json` was never created rather than byte-identity; sufficient here, marginally weaker than a pre-seeded check.
- In `it pins the raw tmux facts the probe rests on`, `liveHookKeyPane` is called with all three returns discarded; the fixture is load-bearing but nothing says so.
