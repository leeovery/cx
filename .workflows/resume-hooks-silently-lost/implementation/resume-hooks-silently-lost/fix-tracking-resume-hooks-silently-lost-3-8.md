## Attempt 1

Verdict was **approved**. No ISSUES. Three comment corrections to apply, all proven false by the reviewer rather than asserted.

COMMENT_CORRECTIONS:
- `internal/restoretest/scrollback.go:12-15` — claims about tests, and false: none of the six call sites this task created asserts on the replayed bytes (verified by grep across `internal/restore` and `cmd`, and by a `window+7` mutation, which killed only one caller and for a reason unrelated to content). The only ANSI-survives assertion in the repo is `verifyANSIScrollback` in `cmd/bootstrap`, a package this helper has no caller in.
  OLD:
	// SeedScrollback writes payload as one pane's saved scrollback, at the path a
	// restore of that session will replay from. The payload is a parameter because
	// callers assert on the replayed bytes: some want ANSI to prove it survives the
	// round trip, others a per-pane marker to prove the right pane got the right
	// scrollback.
  NEW:
	// SeedScrollback writes payload as one pane's saved scrollback, at the path a
	// restore of that session will replay from.

- `internal/restore/rename_reboot_shared_test.go:22-23` — claims about tests, and false: there is no replay assertion in `internal/restore`; the three consumers of this const only need the file to exist so the hydrate helper has something to replay.
  OLD:
	// The ANSI prefix is load-bearing: the replay assertions prove escape
	// sequences survive the save/restore round trip byte for byte.
  NEW:
	(delete the two lines)

- `internal/restoretest/doc.go:4-8` — the diff falsified the stated rule: `restore_marker.go` needs tmux (it takes a `*tmux.Client` and writes a server option) yet is deliberately untagged, and the criterion that actually governs is the callers' lanes, not the dependency surface.
  OLD:
	// Mixed build-tag layout: helpers needing tmux or a built portal binary live in
	// integration-tagged files; pure stdlib + testing helpers omit the tag so they
	// run in the unit lane. A new helper goes in whichever file matches its
	// dependency surface, and general-purpose `go build` plumbing belongs in the
	// sibling internal/portalbintest instead.
  NEW:
	// Mixed build-tag layout: a helper carries the integration tag only when every
	// caller does, so an untagged unit-lane test can still reach it; helpers that
	// need a built portal binary are integration-only. A new helper goes in
	// whichever file matches its callers' lanes, and general-purpose `go build`
	// plumbing belongs in the sibling internal/portalbintest instead.

NOTES (context — not work items):
- The scope extension (folding the two inline duplicates in `rename_reboot_hook_integration_test.go`) was judged **in scope**: they are the same two duplicates named in the task's Problem statement, spelled inline rather than as named helpers. Not to be reverted.
- `WriteIndex(t, stateDir, state.Index)` judged a defensible API decision, not a regression: the `(savedAt, sessions)` form cannot express a captured index at all, and `EncodeIndex` copies before `Canonicalize` so a caller's reused index is not mutated.
- Orchestrator decision on the reviewer's NOTE 1: the `cmd` side's marker-unset error now lands on `t.Logf` rather than `t.Fatalf`. The task's Do bullet prescribed exactly this shared shape, so the executor followed the operative instruction; the acceptance criterion's "on each side" wording was imprecise. **Accepted as delivered — do not change the code for this.**
- Three of six seeder call sites gained a more informative failure message (`"write fixture scrollback %s: %v"` over `"write fixture scrollback: %v"`). A superset, not an assertion, no verdict affected. Accepted.
- `internal/restoretest` now imports `internal/restore` for the first time, foreclosing any in-package `package restore` test from importing `restoretest`. Recorded as a constraint; no change asked for.
- `internal/tmux`'s `TestKillBarrierEscalation_NoScrollbackDeltaIn200msPostExit` passed 8 consecutive times for the reviewer. Not reproducible; if it recurs, capture the output rather than re-running.

## Attempt 1

## Attempt 1

Verdict was **approved**. No ISSUES. Three comment corrections to apply, all proven false by the reviewer rather than asserted.

COMMENT_CORRECTIONS:
- `internal/restoretest/scrollback.go:12-15` — claims about tests, and false: none of the six call sites this task created asserts on the replayed bytes (verified by grep across `internal/restore` and `cmd`, and by a `window+7` mutation, which killed only one caller and for a reason unrelated to content). The only ANSI-survives assertion in the repo is `verifyANSIScrollback` in `cmd/bootstrap`, a package this helper has no caller in.
  OLD:
	// SeedScrollback writes payload as one pane's saved scrollback, at the path a
	// restore of that session will replay from. The payload is a parameter because
	// callers assert on the replayed bytes: some want ANSI to prove it survives the
	// round trip, others a per-pane marker to prove the right pane got the right
	// scrollback.
  NEW:
	// SeedScrollback writes payload as one pane's saved scrollback, at the path a
	// restore of that session will replay from.

- `internal/restore/rename_reboot_shared_test.go:22-23` — claims about tests, and false: there is no replay assertion in `internal/restore`; the three consumers of this const only need the file to exist so the hydrate helper has something to replay.
  OLD:
	// The ANSI prefix is load-bearing: the replay assertions prove escape
	// sequences survive the save/restore round trip byte for byte.
  NEW:
	(delete the two lines)

- `internal/restoretest/doc.go:4-8` — the diff falsified the stated rule: `restore_marker.go` needs tmux (it takes a `*tmux.Client` and writes a server option) yet is deliberately untagged, and the criterion that actually governs is the callers' lanes, not the dependency surface.
  OLD:
	// Mixed build-tag layout: helpers needing tmux or a built portal binary live in
	// integration-tagged files; pure stdlib + testing helpers omit the tag so they
	// run in the unit lane. A new helper goes in whichever file matches its
	// dependency surface, and general-purpose `go build` plumbing belongs in the
	// sibling internal/portalbintest instead.
  NEW:
	// Mixed build-tag layout: a helper carries the integration tag only when every
	// caller does, so an untagged unit-lane test can still reach it; helpers that
	// need a built portal binary are integration-only. A new helper goes in
	// whichever file matches its callers' lanes, and general-purpose `go build`
	// plumbing belongs in the sibling internal/portalbintest instead.

NOTES (context — not work items):
- The scope extension (folding the two inline duplicates in `rename_reboot_hook_integration_test.go`) was judged **in scope**: they are the same two duplicates named in the task's Problem statement, spelled inline rather than as named helpers. Not to be reverted.
- `WriteIndex(t, stateDir, state.Index)` judged a defensible API decision, not a regression: the `(savedAt, sessions)` form cannot express a captured index at all, and `EncodeIndex` copies before `Canonicalize` so a caller's reused index is not mutated.
- Orchestrator decision on the reviewer's NOTE 1: the `cmd` side's marker-unset error now lands on `t.Logf` rather than `t.Fatalf`. The task's Do bullet prescribed exactly this shared shape, so the executor followed the operative instruction; the acceptance criterion's "on each side" wording was imprecise. **Accepted as delivered — do not change the code for this.**
- Three of six seeder call sites gained a more informative failure message (`"write fixture scrollback %s: %v"` over `"write fixture scrollback: %v"`). A superset, not an assertion, no verdict affected. Accepted.
- `internal/restoretest` now imports `internal/restore` for the first time, foreclosing any in-package `package restore` test from importing `restoretest`. Recorded as a constraint; no change asked for.
- `internal/tmux`'s `TestKillBarrierEscalation_NoScrollbackDeltaIn200msPostExit` passed 8 consecutive times for the reviewer. Not reproducible; if it recurs, capture the output rather than re-running.
