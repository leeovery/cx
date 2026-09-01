# Analysis Report: resume-hooks-silently-lost (Cycle 3)

## Stats

- Total findings: 148 (18 from the three analysis agents, 130 banked residue)
- Deduplicated findings: 130
- Banked residue: 109 verified in, 6 resolved, 10 discarded as beyond remit or without a remedy, 5 routed to Spec Defects
- Proposed tasks: 49

## Summary

The production code that this work unit set out to change is sound: the staleness rule has one home, the pane-token option literal has one declaration, `internal/hooks` is a genuine leaf, and the five stand-down reasons render on both surfaces. What the three agents and the bank agree on is that the pressure has moved to the edges — the tmux target/session-name vocabulary is half-converted (only `ShowEnvironment` classifies an unaddressable name; two production sites still hand bare pane targets to the client), the test scaffolding carries the same abstraction re-authored between four and eleven times in six separate families, and the test-isolation machinery has three holes that reach the developer's own machine, one of them reproduced: `go test ./cmd` moves the developer's real legacy config out of `~/Library/Application Support/portal`.

Six banked entries were resolved by work that shipped after they were deposited (`ActivePaneCurrentPath`'s dead wrap and lying doc, the saver's bare-name respawn, restore's bare `PaneTarget`, `cmd/open_test.go`'s capture-handler twin, `internal/restore`'s `captureSink`, and the absolute-value lock-bound test seam). Five bank entries are stale specification claims rather than code defects and are recorded below; four of them were deposited independently by different reviewers asking for the same corrigendum.

## Spec Defects

### S1: The specification names two tmux client methods that no longer exist

- **Claim**: §1.3, line 43 — "**The positional siblings that share the pattern but are not failing** — `state.SanitizePaneKey` … and `internal/tmux`'s `StructuralKeyFormat` / `ResolveStructuralKey` / `ListAllPanes`. … They are checked against the change (§9), not changed by it."
- **Observed**: `grep -rn "ResolveStructuralKey\|ListAllPanes\b" internal cmd` returns nothing outside the spec — both were deleted during this work unit, `ListPanes` with them (task 7-31). `StructuralKeyFormat` survives. The standards agent independently reached the same reading: the deletions are benign because their only consumers were the hook paths this work unit replaced, but §1.3 asserts they were untouched.
- **Read**: spec stale. The code is right — a method whose only callers were the replaced hook paths has no reason to survive the replacement. The section's argument (positional siblings are rebuilt per bootstrap, so drift has no window) still holds for what remains.

### S2: The second corrigendum — presented as the correction — names a file that was deleted and a shape that changed

- **Claim**: Corrigendum 2026-08-30 — "the generator and the predicate both live in `internal/nanoid`, a stdlib-only leaf holding `Alphabet`, the unexported `width`, `NewGenerator` and `IsTokenShaped`. `internal/session/panetoken.go` forwards to it".
- **Observed**: `internal/session/` holds no `panetoken.go` (task 7-30 deleted the forwarder); `cmd/hooks.go:100` reaches `nanoid.NewPaneTokenGenerator()` directly. `internal/nanoid` holds **two** unexported widths — a general-purpose `width` behind `NewGenerator` and `paneTokenWidth` behind `NewPaneTokenGenerator` (`internal/nanoid/nanoid.go:33-38`) — not the single `width` the corrigendum names.
- **Read**: spec stale. Both deviations preserve the property the section argued for and the second width is an improvement: it decouples `hooks.json`'s on-disk key-recognition contract from the session-name suffix width, so moving the suffix width is no longer a migration event. A stale corrigendum is the worst kind of stale spec text, because every agent dispatch is told to read the corrections first.

### S3: The lane rule is stated with two clauses; it has three

- **Claim**: §9.1, line 479 — "The project's lane rule is unchanged and binding: every test that spawns a `portal state daemon` or execs a built `portal` binary carries `//go:build integration`".
- **Observed**: Task 7-33 widened the rule to cover a test that **builds** a portal binary, and CLAUDE.md now states it in four places. No test placement the spec prescribes is invalidated — every integration test it names still qualifies under the wider rule.
- **Read**: spec stale. The widening is correct (a `go build` in the unit lane is the cost the lane rule exists to keep out) and the spec simply predates it.

### S4: §5.4 cites a source location the guard has since left

- **Claim**: §5.4, line 298 — "an empty live set is treated as a bad tmux read rather than as authority, and the sweep defers to the next run with a WARN (`cmd/run_hook_stale_cleanup.go:41-47`)".
- **Observed**: lines 41-47 of that file are now two entries of the `skippedPrunePhrases` map. The guard lives in `judgeAgainstLivePanes` at `cmd/run_hook_stale_cleanup.go:141-162`, unchanged in substance.
- **Read**: spec stale. Prose location only; the invariant is intact and shared by the sweep and the diagnosis.

### S5: §6.3 describes the snapshot as a call-site read; `CleanStale` sequences both reads itself

- **Claim**: §6.3, lines 355-365 — "The sweep reads `hooks.json` twice: once at its call site (`runHookStaleCleanup`, to decide whether there is anything to do) and once inside `CleanStale`. `CleanStale` receives the **live token set** and the **call-site snapshot's key set**" … "`runHookStaleCleanup` takes its call-site snapshot first (§6.3)".
- **Observed**: `CleanStale(enumerateLive func(Snapshot) ([]string, error))` (`internal/hooks/store.go:302`) takes the *enumeration* as a callback and performs `loadSnapshot` itself before invoking it; the call site hands over no snapshot. The ordering the section argues for — snapshot strictly before enumeration, narrowing only — is preserved and is now structural rather than a call-site convention.
- **Read**: spec stale. The move strengthens the invariant (a caller can no longer get the order wrong), so the prose is what needs correcting, not the code.

## Discarded Findings

- **The saver-respawn daemon race (bank 7-9, three entries)** — a measured product defect: `BootstrapPortalSaver` respawns a new daemon with no wait for the outgoing one to release `daemon.lock`, `defaultDaemonRun` stands down on `ErrDaemonLockHeld` with no retry, and its exit destroys the saver pane, leaving no daemon at all. Verified still present (`cmd/state_daemon.go:113-119`, `internal/tmux/portal_saver.go:300`, `waitForSaverDaemonReady` returns `nil` on timeout). Discarded here only because it is beyond this work unit's remit — it is a saver/daemon lifecycle defect with no connection to hook keys, and the depositing executor recorded that it is being logged as its own bugfix work unit. It must not be lost: it is the highest-stakes item in the entire bank.
- **The composite-e2e suite fails 3/3 on clean HEAD (bank 7-15)** — downstream of the defect above (the executor's own measurement traced it to the same lock stand-down). Proposing a load-tolerant budget or a build tag now would paper over the product defect; it belongs with that work unit.
- **`run.build-tags` selects one build configuration (bank 7-10, two entries)** — enabling `integration` swaps `internal/state/pgrep_sandbox_prod.go` out of the linted set, and `unused` counts a production symbol referenced only by an integration test as live. The depositing reviewer recorded that no code remedy exists short of reverting the settled single-invocation decision. Knowledge, not work.
- **A prior cycle's analysis record names the wrong twin (bank 7-20)** — `analysis-duplication-c2.md:36` calls `store_shape_test.go:86` a trimmed copy of `:18` when `:86` is the claim-preserving one. Correct, and mutation-proven, but the artefact is a superseded workflow record rather than code; nothing downstream reads it.
- **"Which of the repo's ~20 guards could derive rather than restate" (bank 7-25)** — a direction, not a bounded unit of work. Its concrete instances are all proposed separately (the target-composition package list, the pane-token width guard's scope, the leaf-guard family).
- **`logtest.Sink` cannot model the production level gate (bank 7-27)** — one consumer (`TestExecMarker_VisibleAtWARN`), deliberately bespoke, already rebuilt to gate-then-forward into a Sink so only the gate is hand-rolled. Adding a gated variant for a single caller is speculative.
- **The re-homed pane-token width guard scans `cmd` only (bank 7-30)** — the depositing reviewer classified it as a scope property of pinning the seam default rather than a defect, and `cmd` is the whole production mint surface today (`internal/restore/session.go:166` re-stamps a saved token, it does not mint).
- **The two byte-identical decline branches in `declinedSweep` (bank 7-12, first half)** — at two instances a shared helper is the premature abstraction the project's own code-quality guidance warns against, and each branch comment carries rationale a table would flatten. The reviewer declined it deliberately and set the trigger at a sixth decline reason. The second half of that entry (a narrower absence assertion) is carried into the stand-down test-corpus proposal.

### Resolved by work that shipped after the entry was banked

- **`ActivePaneCurrentPath` documents a sentinel its route cannot produce, and its wrap is dead (bank 7-2, two entries)** — resolved by task 7-32: the `wrapNoSuchSession` call is gone and the doc now states the measured truth ("A session no pane answers to returns `("", nil)`, NOT an error").
- **The saver's own respawn addresses tmux by a bare session name (bank 7-2)** — resolved: `internal/tmux/portal_saver.go:300` now composes `ExactCoordTarget(PortalSaverName)`.
- **Restore arms panes through the bare `tmux.PaneTarget` (bank 7-7)** — resolved: `internal/restore/session.go:140` now builds `tmux.PaneTargetExact(...)` and feeds it to both `SetPaneOption` and `RespawnPane`.
- **`cmd/open_test.go`'s `capturingHandler` is a fourth capture twin (bank 7-4)** — resolved: `capturingHandler`, `capturedRecord`, `snapshot()` and `recordComponent` are gone; only the sanctioned `warnBypassHandler` survives. (The sibling entry's `main_panic_test.go` `captureHandler` is likewise gone; its `errorAttrRecorder` half survives and is carried.)
- **`internal/restore`'s `captureSink` is the last survivor of a dropped wrapper pattern (bank 7-11)** — resolved: `internal/restore/logging_capture_test.go` no longer exists. The CLAUDE.md clause it was coupled to is *not* resolved and is proposed separately.
- **The pre-read bound was pinned to an absolute value of its own (bank 7-35)** — resolved by task 7-35 itself: `SetSnapshotLockTimeoutForTest` is gone, replaced by the read-only `SnapshotLockBoundForTest` accessor.
