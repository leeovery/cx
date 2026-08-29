# Analysis Tasks: resume-hooks-silently-lost (Cycle 1)

## Task 1: Give the stale-hook sweep one decline ladder and one outcome value
severity: high
sources: duplication, architecture, bank

**Problem**: Three defects sit on one call chain. (a) `checkStaleHooks` (`cmd/doctor.go:303-334`) and `runHookStaleCleanup` (`cmd/run_hook_stale_cleanup.go:69-128`) each carry a private copy of the same three-guard ladder — restore-window stand-down, enumeration failure, zero-rows-with-entries-persisted — that the design *requires* to agree: the sweep must not delete on a pane-less read and the check must not report those entries stale. Divergence is silent and asymmetric, and is exactly the "diagnostic asks the reaper's question and gets a different answer" failure this work unit set out to close. (b) `runHookStaleCleanup` has five outcomes in which no pruning happens — restore window set, empty pane read, lock timeout, `LoadSnapshot` failure, `ListAllPaneHookKeys` failure — but only the first three call `reportSkip`; the two read failures return having invoked no callback, and `pruneDoctorStaleHooks` discards the return with `_ =`. A `portal doctor --fix` whose pane enumeration errors therefore prints neither a `Pruned stale hook:` line nor a `Skipped stale hook prune:` line: the repair silently did not run. (c) The emissions split across two loggers — the injected `*slog.Logger` for the two read failures, the package-level `hooksLogger` for the three stand-downs — so `grep "hooks:"`, the grep the closed `op=clean-stale-skipped` vocabulary exists for, answers "did the prune stand down and why" for three of five cases. The root cause of all three is the shape: five parameters (one over the project's `<=4` convention), two of them optional callbacks the daemon passes `nil, nil` for, plus an `error` return — "what happened this cycle" is spread across four out-channels and one caller can observe three. Separately, the stand-down copy deviates from the fixed specification: `restoreStandDownPhrase` (`cmd/doctor.go:207`) renders `restore may be in progress` where the spec fixes `restore in progress`.

**Solution**: Collapse the out-channels into one returned value, derive both consumers' guard decisions from one function, and restore the specified copy.

**Outcome**: `runHookStaleCleanup` returns a single value describing what the cycle did or why it declined; `doctor --fix` renders both halves from that value and can never print nothing; a newly added decline path cannot be forgotten because it must populate the field the renderer already reads; `checkStaleHooks` maps the same reason set onto its not-evaluable detail rather than restating the ladder; all five decline paths emit under the `hooks` component.

**Do**:
1. In `cmd/run_hook_stale_cleanup.go`, extract the guard ladder as one function answering "what can be judged right now" — e.g. `evaluateHookStaleness(reader, persisted) (liveTokens []string, standDownReason string, err error)`, returning an empty reason when the set is judgeable. Preserve the deliberate ordering difference (the sweep gates on the restore marker before loading the store; the check loads first) as an explicit parameter or by having each caller do its own load, not by forking the ladder.
2. Add the two read failures (`LoadSnapshot` failure, `ListAllPaneHookKeys` failure) to the reason set. Correct `skippedPrunePhrases[skipReasonEmptyPaneRead]`, whose current wording "could not read live panes" describes a *failed* read better than the successful-but-empty read it is wired to — give the failed read that phrase and the empty read its own.
3. Change the signature to `runHookStaleCleanup(reader, store, logger) (sweepOutcome, error)` where `sweepOutcome` carries the removed keys and, when the cycle declined, the reason. Drop the `onRemoved`/`onSkipped` callbacks. The daemon (`cmd/state_daemon.go`) discards the outcome; `pruneDoctorStaleHooks` (`cmd/doctor.go:200-213`) renders both halves and must stop discarding the return with `_ =`.
4. Route every decline emission through the package-level hooks logger so all five appear under `grep "hooks:"` with the closed `op=clean-stale-skipped` vocabulary.
5. Replace the two seam interfaces with one embedded composition: `type staleSweepReader interface { PaneHookLister; state.RestoringChecker }`. `AllPaneLister` (`cmd/run_hook_stale_cleanup.go:39`) currently restates `PaneHookLister`'s (`cmd/hooks.go:24`) method signature rather than embedding it, so both declarations and both copies of the same "one row per live pane, empty token for an unstamped pane, row count answers whether the read succeeded" doc paragraph must be edited together. Rename away from `AllPaneLister`, which names one of two capabilities and is left over from an enumeration that no longer exists — the `@portal-restoring` read is discoverable only from the doc comment.
6. Set `restoreStandDownPhrase = "restore in progress"`, matching the specification's fixed copy. The in-source hedge's justification (a failed marker read counts as a set marker, so a server-less machine stands down without a restore underway) is a fact the specification already stated in the same section and still chose the unhedged wording for.

**Acceptance Criteria**:
- No guard decision is spelled out in more than one function; adding a fourth guard requires editing one place.
- All five decline paths populate the outcome's reason and emit under the `hooks` component.
- `pruneDoctorStaleHooks` prints exactly one line for every possible outcome of the sweep, including both read failures.
- The seam interface embeds `PaneHookLister` and `state.RestoringChecker` rather than restating either, and carries one copy of the enumeration doc paragraph.
- `runHookStaleCleanup` takes no more than four parameters.
- The three `Skipped stale hook prune:` phrases match the specification verbatim: `restore in progress` / `hooks.json is locked` / `could not read live panes`.

**Tests**:
- Unit: each of the five decline paths produces a distinct, non-empty reason on the returned outcome, driven through the existing `stubAllPaneLister` seam.
- Unit: `doctor --fix` stdout carries a `Skipped stale hook prune:` line when the pane enumeration errors and when `LoadSnapshot` fails — the two cases that currently print nothing.
- Unit: `checkStaleHooks` and the sweep return the same judgeable/not-judgeable verdict for a shared table of fixtures (empty rows with entries present, enumeration error, restore marker set, live rows with unstamped panes).
- Unit: the exact-equality assertion on the `Skipped stale hook prune: restore in progress` line.

## Task 2: Make the hooks store own `CleanStale`'s ordering invariant instead of documenting it
severity: high
sources: architecture

**Problem**: `CleanStale(liveTokens, snapshotKeys []string)` (`internal/hooks/store.go:295`) takes two adjacent `[]string` parameters whose roles are inverses — one is "keys that must be protected", the other "keys that may be considered" — and the correctness of the whole anti-reap protection depends on a temporal fact the signature cannot express: `snapshotKeys` must have been read *before* whatever enumeration produced `liveTokens`. That fact lives only in a comment at the single call site (`cmd/run_hook_stale_cleanup.go:95-102`). A transposed call `CleanStale(snapshot, tokens)` compiles and type-checks, and its effect is not benign: it computes staleness against the snapshot and narrows to the live set, which can delete entries whose panes are alive — the exact loss this work unit exists to prevent. Any second caller, or a reorder of the two reads at the existing one, silently inverts the protection with nothing failing. `cmd/hook_sweep_snapshot_order_test.go` pins the ordering behaviourally at *this* call site; nothing pins it for the API.

**Solution**: Move the sequence into the store so the caller cannot get it wrong, or at minimum make the two arguments non-interchangeable at the type level.

**Outcome**: The read-snapshot-then-enumerate-then-derive order is enforced by the code's shape rather than by a comment, and a transposition is a compile error rather than a silent mass deletion.

**Do**:
1. Preferred form: change the signature to `CleanStale(enumerate func() ([]string, error))`. The store reads its own advisory snapshot under the shared hold, releases it, calls `enumerate` with **no lock held** (which also enforces the "no tmux inside the lock" rule structurally instead of by convention), then takes the exclusive hold and derives. The sweep's empty-row guard rides along by having the closure return a sentinel error.
2. If a callback is unwanted, take the cheaper form: give the snapshot a named type (`type Snapshot []string`, returned by `LoadSnapshot` rather than a bare map) so the two arguments stop being interchangeable.
3. Update `cmd/run_hook_stale_cleanup.go:95-103,141` to the new shape and delete the ordering comment that the code now enforces (keep a one-line note naming *why* the order matters, not *that* it must be kept).
4. Preserve `narrowToSnapshot`'s semantics exactly: a key the snapshot does not hold was written after the enumeration and so was never offered to it for protection, making it unjudgeable however stale its shape looks.

**Acceptance Criteria**:
- The two same-typed `[]string` parameters no longer sit adjacent on `CleanStale`'s signature; a transposed call does not compile.
- No tmux read happens while the store holds either lock.
- The existing anti-reap behaviour is unchanged for every fixture in the current suite.

**Tests**:
- Unit: a fixture proving the store performs its snapshot read before invoking the enumeration closure (record call order).
- Unit: a fixture proving no lock is held while the closure runs (the closure asserts the sidecar is free).
- Unit: the closure returning an error aborts the clean with nothing written and nothing logged as removed.
- Unit: the existing `cmd/hook_sweep_snapshot_order_test.go` behaviour survives the re-shape.

## Task 3: Restore `internal/hooks` to a leaf by moving the id vocabulary into its own package
severity: medium
sources: architecture

**Problem**: `staleKeys` reaches the shape rule through `session.IsTokenShaped`, so `internal/hooks` now imports `internal/session` (`internal/hooks/store.go:14,253`). Before this change the store's dependency set was `{fileutil, log, storelog}`; `go list -deps ./internal/hooks` now also returns `session`, `tmux`, `state`, `project`, `resolver`, `xdg`, `tmuxerr`, `tmuxout` — verified. A JSON persistence store, the one package on the hydrate hot path called once per restored pane, drags in the whole tmux client and session-creation tree to reach a ten-line pure function over two constants (`suffixLen`, `NanoIDAlphabet`). Two consequences beyond weight. The edge is one-way and permanent: `internal/state` and `internal/tmux` can no longer import `internal/hooks` without a cycle, and `internal/state` is precisely the package that owns `PortalPaneIDOption` and the `captureFormat` column carrying the token — the most plausible future consumer of "is this a well-formed token?", now structurally locked out of asking. And the repo has an established pattern for exactly this (`internal/tmuxerr`, `internal/tmuxout`, `internal/xdg`, `internal/storelog` are all dependency-free leaves factored out so two packages that must not import each other can share a value) which this change did not use.

**Solution**: Factor the id vocabulary into a stdlib-only leaf both packages import.

**Outcome**: `internal/hooks` is a leaf again; the shape predicate stays derived from the generator that produces the tokens; `internal/state` and `internal/tmux` can reach it should either ever need to.

**Do**:
1. Create a stdlib-only leaf (e.g. `internal/nanoid`) holding `NanoIDAlphabet`, the token width, `NewNanoIDGenerator` and `IsTokenShaped`. Keeping the width and alphabet beside each other preserves the derive-shape-from-the-generator property the current design is built on, and both stay unexported outside the leaf.
2. Point `internal/hooks/store.go` at the leaf; point `internal/session` at it too.
3. Keep `session.NewPaneToken` and `session.IDGenerator` where they are as thin re-exports if their call sites are worth preserving.
4. Re-verify with `go list -deps ./internal/hooks` that the dependency set is back to `{fileutil, log, storelog, <new leaf>}`.

**Acceptance Criteria**:
- `go list -deps ./internal/hooks` returns no `tmux`, `state`, `session`, `project` or `resolver`.
- The new package imports nothing outside the standard library.
- The shape predicate and the generator's alphabet/width remain in one package, so a width change cannot desynchronise them.
- Every existing call site of `NewPaneToken` / `IsTokenShaped` behaves identically.

**Tests**:
- Unit: a dependency guard asserting `internal/hooks`'s import set (source-walking, in the style of the repo's ~20 existing guards, driven by `sourceguardtest`).
- Unit: the existing `IsTokenShaped` table moves with the predicate and passes unchanged.
- Unit: a generated token is token-shaped, pinning the derive-from-the-generator property in the new home.

## Task 4: Emit the reaper's per-key deletion lines only after the write that performs the deletion
severity: medium
sources: standards, bank

**Problem**: `CleanStale` (`internal/hooks/store.go:320-329`) logs one INFO `clean-stale` line per removed key, then calls `s.save(kept)`. When the save fails the keys are still in the file, but the log has already named each of them as removed. At the production default level an operator greps `hooks:` and reads N `deleted X, command was Y` lines followed by one WARN. The ordering predates the work unit and was harmless while the line was DEBUG; the specification promotes it to INFO precisely so "what did I lose?" is answerable from the production-default log, and a line claiming a deletion that did not happen is the same class of misleading record this work unit exists to remove. The batch summary does carry the save error, so the truth is recoverable by reading the next line — but the per-key line is the one the spec added for standalone forensic value.

**Solution**: Move the per-key INFO loop below the successful save.

**Outcome**: A `clean-stale` INFO line exists if and only if the key was actually removed from the file.

**Do**:
1. In `internal/hooks/store.go`, move the `for _, key := range removed { logger.Info("clean-stale", ...) }` loop to after `s.save(kept)` returns nil.
2. Leave the failure path emitting only `storelog.EmitCleanStaleSummary(logger, len(removed), start, err)`.
3. Keep the removed-key values read from `h` (the pre-delete map) so the `value` attr still carries the reaped `on-resume` command.

**Acceptance Criteria**:
- A failed save produces zero `op=clean-stale` INFO lines and exactly one WARN summary carrying the error.
- A successful save produces one INFO line per removed key, each carrying the removed `on-resume` command, plus the INFO summary.
- The line's attrs and ordering relative to the summary are otherwise unchanged.

**Tests**:
- Unit: with writes denied, the sink holds no `clean-stale` INFO record and exactly one WARN summary.
- Unit: on success, the sink holds one `clean-stale` INFO per removed key with the `value` attr carrying the reaped command.

## Task 5: Tighten the hooks store's widened public surface
severity: medium
sources: standards, architecture

**Problem**: Three items on one surface. (a) The specification declares exactly one acquisition bound ("The bound is 2 seconds ... a package-level value the unit lane can lower") and has the sweep's advisory pre-read degrade "by the same rule" as every other read. The implementation adds a second bound, `snapshotLockTimeout = 20 * time.Millisecond` (`internal/hooks/lock.go:25`), reachable only through a new exported `Store.LoadSnapshot` and a second test seam `SetSnapshotLockTimeoutForTest`. The stated rationale is sound and serves the spec's intent, but the spec reasoned about exactly this stall and resolved it by releasing the shared hold before `CleanStale` rather than by adding a bound. This is a design decision made at implementation time: it widens the public surface (a second read entry point a future caller can pick by accident) and the tuning surface (two constants that can now drift). (b) `via` names one of a fixed set of calling surfaces (`cli`, `internal`, `hydrate`, `doctor`) that the logging spec treats as closed, and it is now a bare `string` on `Load`, `LoadSnapshot`, `List`, `Get` and the mutators, supplied as a literal at every call site. A typo or an invented value compiles, ships, and produces a log attr no grep will ever match — silent, and only visible in a log read after the fact. `LookupOnResume` already hardcodes `"hydrate"` internally, so the value is half-owned by the package. The clearest symptom of the parameter having been threaded mechanically: `Store.Get` (`internal/hooks/store.go:336`) has no production caller at all — verified, only tests and one restore test helper — and still gained the parameter. (c) Two new doc comments make cardinality claims the quality standard lists under "Never in a comment": `LoadSnapshot`'s "it is the only read taken at snapshotLockTimeout rather than lockTimeout" and `staleKeys`'s "staleKeys is the single implementation of the staleness rule".

**Solution**: Decide the second bound explicitly, type the `via` vocabulary, delete the dead API, and rewrite the two comments as constraints rather than counts.

**Outcome**: The store's public surface carries no dead exported method, no untyped closed vocabulary, and no undocumented second tuning constant; a `via` typo is a compile error.

**Do**:
1. **Decision required at approval**: confirm the second bound is wanted. If yes, it belongs alongside the 2s figure with its justification so the two values stay explained together — record the justification in-source above `snapshotLockTimeout` in the shape the daemon's hysteresis constant uses, and flag the spec amendment. If no, collapse `LoadSnapshot` onto the single `lockTimeout` and delete `snapshotLockTimeout` / `SetSnapshotLockTimeoutForTest`, keeping the release-before-`CleanStale` ordering the spec relies on.
2. Add `type Via string` to `internal/hooks` with the four exported constants, and change every store method taking `via` to take it. Convert all call sites from string literals to constants.
3. Delete `Store.Get`. Its two test usages read the same data `Load` already returns — re-point them.
4. Rewrite the two comments as constraints: "reads at the sweep's short bound, so one cycle never spends two full bounds waiting" and "the staleness rule; every reader of staleness must route through here rather than restate it".

**Acceptance Criteria**:
- `via` is a named type; no call site passes a string literal.
- `Store.Get` no longer exists and nothing references it.
- Whichever bound decision is taken, no bound exists without an in-source justification naming the stall it prevents.
- Neither new doc comment states a count a reader cannot verify locally.

**Tests**:
- Unit: the existing read/write lock-degradation table survives the `Via` type change unmodified in behaviour.
- Unit: the `via` attr on every emitted breadcrumb still matches the closed vocabulary (existing assertions re-pointed at the constants).

## Task 6: Version-pin the hydrate helper and put the real-restore tests behind the integration lane
severity: high
sources: bank

**Problem**: `buildHydrateCommand` bakes a bare `portal state hydrate` (`internal/restore/session.go:312`), so an armed pane resolves `portal` from the tmux server's PATH — the developer's *installed* binary, not the one under test. `internal/spawn/command.go:12` deliberately uses `os.Executable()` for exactly this version-pinning reason. Two consequences. In production it is a narrow hazard: a shadowed `portal` earlier on PATH hydrates restored panes with the wrong version, and the warm-command latch reasoning that makes spawned opens an abridged fast-path does not hold for restore. In tests it is worse than a hazard, it is misattribution: `internal/restore/integration_test.go` is **unit-lane** (verified: no build tag) yet drives a real restore against a real tmux server, so it silently asserts against whatever is installed. Confirmed at HEAD those tests pass against homebrew 0.11.0 and fail with a misleading `expected alpha in list-sessions` until a current binary is staged. That also collides with CLAUDE.md's own lane rule that any test exec-ing a built `portal` binary lives behind `-tags integration`. Four tests are exposed: `TestPhase3Integration_SaveRestoreRoundTrip` and `TestPhase3Integration_RestoreUsesLiveIndicesUnderBaseIndexDrift` (`internal/restore/integration_test.go`), plus `cmd/bootstrap/phase5_integration_test.go:106` and `cmd/bootstrap/phase5_marker_suppression_integration_test.go:58`, none of which call `restoretest.BuildPortalBinaryDir` + `PrependPATH`.

**Solution**: Pin the hydrate argv to the running executable, and give the four real-restore tests the staged-binary prologue and the correct build tag.

**Outcome**: A restored pane hydrates with the binary that armed it; a stale developer install can no longer report as a restore regression; no test exec-ing a built `portal` sits in the unit lane.

**Do**:
1. In `internal/restore/session.go`, replace the bare `portal` in `buildHydrateCommand` with `os.Executable()`, matching `internal/spawn/command.go`. Handle the error path by falling back to the bare name rather than failing the restore — an unresolvable executable path must not abort a reboot recovery.
2. Move the two real-restore tests out of `internal/restore/integration_test.go` into an `//go:build integration` file (or tag the file, relocating the tests that do not need a binary if any remain untagged).
3. Add the `restoretest.BuildPortalBinaryDir(t)` + `restoretest.PrependPATH(t, binDir)` prologue to all four tests, matching the shape already used at `internal/restore/integration_full_test.go:37-38` and `internal/restore/multipane_legacy_integration_test.go:29-30`.
4. Since `os.Executable()` now pins the path, verify whether the PATH prologue is still load-bearing for each of the four; keep it where the test drives the binary by name from tmux, drop it where it is now dead setup.

**Acceptance Criteria**:
- No production code composes a `portal` invocation as a bare PATH lookup.
- `go test ./...` (unit lane) contains no test that execs a built `portal` binary.
- The four named tests pass on a machine whose installed `portal` is an older release.
- `internal/restore`'s integration lane still passes with `-p 1`.

**Tests**:
- Unit: `buildHydrateCommand` renders the running executable's absolute path, not `portal`.
- Unit: the fallback path (executable unresolvable) still produces a runnable command rather than an error.
- Integration: the two `TestPhase3Integration_*` tests pass with a deliberately stale binary earlier on PATH.

## Task 7: Route every session-level `-t` target in the tmux client through the exactness rule the package states
severity: medium
sources: bank

**Problem**: `internal/tmux/tmux.go:419-427` states the rule twice — "Every session-level `-t` target must route through here. tmux otherwise prefix-matches, so `-t foo` silently resolves to a live `foo-2` once `foo` is gone" — and seven sites in the same file ignore it, verified: `:259` (`display-message`), `:315` (`set-option`), `:431` (`ListPanesInSession`), `:488` (`ListAllPanesWithFormat`), `:539` (`list-panes -t`), `:560` (`show-environment`), `:761` (`set-environment`). This is the same class the `exactTarget` helper was introduced to close on the kill path, and it is the rename class this work unit is about: once a session is renamed away, a bare `-t foo` resolves to a prefix sibling and the call silently operates on the wrong session. Measured on tmux 3.7c: with `foo` killed and `foo-2` live, `set-option -p -t foo:0.0` exits 0 and writes to `foo-2`; the `=` form fails correctly.

**Solution**: Route all seven through `exactTarget`.

**Outcome**: The package's stated invariant holds for every session-level target it composes, so a renamed-away session produces a clean tmux error rather than a silent wrong-session operation.

**Do**:
1. Wrap each of the seven bare `-t <session>` arguments in `exactTarget(...)`.
2. Check each call's error handling: several currently treat a tmux failure as "no such session"; with the `=` form, a renamed-away session now *reaches* that path instead of silently succeeding. Confirm each site's error class still maps correctly (notably `ErrNoSuchSession` discrimination via `internal/tmuxerr`).
3. Leave `PaneTarget` / `PaneTargetExact` call sites alone — the pane-level half is a separate decision made per call site, and the daemon capture-loop site is out of this work unit's scope.
4. Do not change the doc comments' claims; make them true.

**Acceptance Criteria**:
- No `-t` argument in `internal/tmux/tmux.go` passes a bare session name.
- Each of the seven sites returns the same error class for a genuinely missing session as it does today.
- A prefix-sibling session cannot be reached by any of the seven when the named session is gone.

**Tests**:
- Unit: a fake-commander table asserting each of the seven composes `=<session>`.
- Real-tmux (unit lane, per-test `-L` socket, no daemon): with `foo` killed and `foo-2` live, each affected read/write fails rather than operating on `foo-2`.

## Task 8: Give the restoring-marker posture one named home
severity: medium
sources: bank

**Problem**: Four readers in `cmd` take a posture on `@portal-restoring` and only one of them names it. `restoreWindowActive` (`cmd/run_hook_stale_cleanup.go:65`) stands down silently and folds the error in. `cmd/state_daemon.go:174` (tick) and `:339` (`defaultShutdownFlush`) both read `state.IsRestoringSet` and, on error, log a WARN and return without work — the same read-failed-stand-down-loudly shape spelled twice, each under its own rationale comment. `cmd/state_commit_now.go:108-118` takes the identical failed-read-counts-as-set posture behind the `IsRestoring func() (bool, error)` seam, with a third reporting policy. All four share the substantive rule (a failed read presumes the marker set, to protect an in-flight restore) and differ only in how loudly they report it — but the divergence is invisible at the call sites, so a reader cannot tell it is deliberate, and the two daemon copies are unnamed opposites of the one named predicate sitting in the same package.

**Solution**: One named home for the posture, taking the report/quiet policy from the caller.

**Outcome**: The presume-set-on-failure rule has one implementation; each of the four readers declares its reporting policy explicitly rather than restating the rule.

**Do**:
1. Put the named predicate beside `IsRestoringSet` in `internal/state/markers.go` — e.g. a function returning "should I stand down?" plus the underlying error, so the caller decides between swallow-and-proceed-quietly and log-a-WARN.
2. Re-point `restoreWindowActive` at it, preserving today's silent stand-down.
3. Re-point `cmd/state_daemon.go:174` and `:339` at it, preserving today's WARN-and-return. Collapse the two rationale comments into one reference to the shared home.
4. Re-point `cmd/state_commit_now.go:108` at it through its existing `IsRestoring` seam, preserving today's reporting.
5. This is a behaviour-preserving consolidation: no reader's user-visible behaviour changes.

**Acceptance Criteria**:
- The "a failed read counts as set" rule appears in exactly one place.
- Each of the four call sites is one line naming its reporting policy, not a restatement of the rule.
- Every existing test of daemon stand-down, commit-now suppression and sweep stand-down passes unchanged.

**Tests**:
- Unit: the shared predicate's three-way table (set / unset / read error) in `internal/state`.
- Unit: each of the four call sites still emits (or suppresses) exactly the records it does today, asserted through the existing sinks.

## Task 9: Delete `ResolveStructuralKey` and `ListAllPanes`, and correct the claim CLAUDE.md makes about them
severity: low
sources: bank

**Problem**: `internal/tmux/tmux.go:226` (`ResolveStructuralKey`) and the `ListAllPanes` method are reached only from `internal/tmux/tmux_test.go:1423-1611` — verified: no production caller anywhere. Production reaches the structural shape through `StructuralKeyFormat` + `ListAllPanesWithFormat` (`cmd/bootstrap/stale_marker_cleanup.go:57`). Both were already test-only before this work unit, but CLAUDE.md:60 describes all three as serving non-hook structural use, which now holds only for the constant. The bank entry deferred deletion pending phase 3's retirement of the positional hook machinery; that has landed, and re-verification confirms neither is reached.

**Solution**: Delete both methods and their tests; correct the CLAUDE.md sentence.

**Outcome**: The tmux client exports no method with zero production callers, and the architecture doc's description of the structural-key surface is true.

**Do**:
1. Delete `Client.ResolveStructuralKey` and `Client.ListAllPanes` from `internal/tmux/tmux.go`.
2. Delete `TestResolveStructuralKey` and the `ListAllPanes` cases in `internal/tmux/tmux_test.go:1423-1611`.
3. Keep `StructuralKeyFormat` and `ListAllPanesWithFormat` — both have live production callers.
4. Amend the `tmux` row in CLAUDE.md so the "serve only non-hook structural use" sentence names the constant and `ListAllPanesWithFormat`, not the two deleted methods.

**Acceptance Criteria**:
- Neither method exists; the build and both lanes are green.
- CLAUDE.md's `tmux` row names only surfaces that exist.
- `cmd/bootstrap/stale_marker_cleanup.go`'s path is untouched.

**Tests**:
- Both lanes green after deletion is the test; no new test is warranted for removed code.

## Task 10: Tidy `cmd/hooks.go` — collapse the four nil-check builders and trim the redundant error wrap
severity: low
sources: bank

**Problem**: Two items in one file. (a) `buildHookKeyResolver` (`cmd/hooks.go:76`), `buildPaneHookLister` (`:83`), `buildPaneStamper` (`:90`) and `buildTokenMinter` (`:97`) are the same three-line `if hooksDeps != nil && hooksDeps.X != nil` shape; the fourth crossed the Rule of Three. (b) The gone-pane message reaching the user is three wraps deep: `portal hook set` on a dead pane renders `failed to resolve hook key for current pane: no pane answers to "%999": tmux show-options -p -t %999: exit 1: no such pane: %999` — `cmd/hooks.go:70` prefixes `internal/tmux/tmux.go:245`, which prefixes `CommandError.Error()`. tmux's own words survive unaltered at the tail as specified, but the two Portal-added layers say the same thing twice.

**Solution**: One generic picker for the four builders; drop the outer wrap that restates the inner one.

**Outcome**: A fifth seam costs one line rather than a fourth copy, and the dead-pane error reads as one Portal sentence followed by tmux's verbatim words.

**Do**:
1. Replace the four builders with a single generic helper that takes the production default and the override, returning the override when both `hooksDeps` and the field are non-nil.
2. At `cmd/hooks.go:70`, drop the `failed to resolve hook key for current pane:` prefix (or the inner `no pane answers to %q:` layer — keep exactly one) so the user sees one Portal-authored clause plus tmux's unaltered tail.
3. Keep the tmux tail byte-identical: the specification requires tmux's own words survive.

**Acceptance Criteria**:
- One builder helper serves all four seams.
- The dead-pane message carries exactly one Portal-authored prefix and tmux's verbatim words.
- `hook rm`'s wording, which shares this call site, is unchanged.

**Tests**:
- Unit: each of the four seams still resolves to its injected fake and to its production default.
- Unit: exact-string assertion on the dead-pane error for `hook set`, and a guard that `hook rm`'s existing exact-string assertions still pass.

## Task 11: Collapse the duplicate `AllPaneLister` fakes and the duplicated seed-key aliases in `cmd`
severity: medium
sources: duplication, bank

**Problem**: `fakeHookLister` (`cmd/doctor_test.go:798-809`) and `stubAllPaneLister` (`cmd/hookkey_vocabulary_test.go:111-130`) are both fakes over the identical four fields (`rows`, `err`, `restoring`, `restoringErr`), both implement `ListAllPaneHookKeys` and `TryGetServerOption`, and both delegate the marker answer to the shared `restoringOption` helper. They differ only in a value vs pointer receiver and `stubAllPaneLister`'s optional `during` hook. `cmd/hookkey_vocabulary_test.go` declares itself the home for "the seam fakes that answer with them", and `fakeHookLister` sits outside it with 41 references in `doctor_test.go` and 2 in `hook_sweep_lock_timeout_test.go` — the two are already interleaved inside single test files (`cmd/hook_sweep_lock_timeout_test.go:271` uses one, `:290` the other), which is copy-paste drift arriving in real time. Separately, several package-level names in `cmd` re-derive the same seed value rather than reaching the vocabulary: `renameRestoreToken` (`cmd/rename_restore_cleanup_survival_integration_test.go:21`), `liveHookToken` (`cmd/state_daemon_hook_cleanup_integration_test.go:51`) and `liveKey` (`cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:99`) are all `transienttest.ReapableHookKey(1)`, which is also `reapableSeedB` (`cmd/hookkey_vocabulary_test.go:20`) — so a "live" fixture and a "reapable" fixture silently hold one value.

**Solution**: One fake, and in-package fixtures reaching the seed vocabulary by name instead of re-deriving the index.

**Outcome**: `cmd` has one `AllPaneLister` fake; no two names in the package hold the same seed value by coincidence.

**Do**:
1. Delete `fakeHookLister` and re-point its 43 call sites at `stubAllPaneLister`. The doctor-side factories `staleHookLister()` / `restoringHookLister()` (`cmd/doctor_test.go:876-887`) and `seedStalePruneFixture`'s parameter type move with it.
2. Re-point `renameRestoreToken`, `liveHookToken` and `liveKey` at the named seed constants in `cmd/hookkey_vocabulary_test.go`. Where a fixture genuinely needs a *live* key distinct from the reapable set, use `liveSeedA/B/C` — do not leave a name asserting "live" bound to `ReapableHookKey(1)`.
3. Note the bank's related observation that two of the three original fakes (`recordingHookKeyLister`, `stubAllPaneLister`) have already merged into `hookkey_vocabulary_test.go`; this closes the third.

**Acceptance Criteria**:
- One `AllPaneLister` fake exists in package `cmd`, in `cmd/hookkey_vocabulary_test.go`.
- No test file declares a package-level hook-key name that duplicates a seed the vocabulary already names.
- Every fixture named "live" holds a key from the live seed set.
- Verdict count is unchanged: no case is lost in the re-point.

**Tests**:
- The re-pointed suites passing with the same verdict count is the test; no new case is warranted.

## Task 12: Single-source the `hooks.json` staging helpers, sidecar decision included
severity: medium
sources: duplication, bank

**Problem**: Three staging paths, already drifted. `seedHooksJSON(t, keys...)` (`cmd/doctor_test.go:811`) and `newStagedHooksStore` / `newTempHooksStore` (`cmd/testhelpers_test.go:39,70`) all stage a `hooks.json` in a temp directory and return `(*hooks.Store, path)`. `newStagedHooksStore` creates the sidecar lock file deliberately, so a read takes its shared lock instead of degrading; `seedHooksJSON` does not. The drift is visible at a call site: `cmd/hooks_read_lock_test.go:29-33` has to add `transienttest.CreateHooksSidecar` by hand with a three-line comment explaining that "seedHooksJSON stages no sidecar, so without this the baseline would itself degrade on ENOENT". Every other `seedHooksJSON` caller silently gets a degraded read it did not ask for, and `cmd/hook_sweep_lock_timeout_test.go` uses both helpers inside one test function. A third variant exists for denied writes: `readOnlyDirPath` and `seedThenDenyWrites` (`internal/hooks/store_test.go:30-51,73-92`) and `newStagedHooksStore{writesDenied: true}` (`cmd/testhelpers_test.go:39-66`) all perform the same sequence — make the directory writable, create the sidecar, chmod to 0500, register a 0700 cleanup, return the path. `seedThenDenyWrites`'s doc comment claims it "cannot use readOnlyDirPath" because the seed write must precede the lock, but `readOnlyDirPath` already creates the directory at 0700 and denies only at the end; the two differ solely in whether a body is written before the chmod.

**Solution**: One staging helper per package with the sidecar decision made once — and make the modelling decision the consolidation forces, deliberately.

**Outcome**: A hooks-store fixture's sidecar state is chosen, not inherited; no call site compensates by hand.

**Do**:
1. **Decision required at approval**: folding `seedHooksJSON` into `newStagedHooksStore` turns ~30 doctor fixtures from a sidecar-less fresh install into a written-to install, and falsifies `cmd/hooks_read_lock_test.go:29-31`'s baseline. Measured as verdict-neutral today (1577 verdicts identical, all green), but it is a modelling change, not a helper move. Decide whether doctor's fixtures model a fresh install or a written-to one, and record the answer in the helper's doc comment.
2. Give `hooksStoreStaging` a `keys []string` field (or keep the JSON `seed` and have callers build the body from the seed vocabulary) so `seedHooksJSON`'s key-list ergonomics survive the fold. Note the two are not the same shape — `seedHooksJSON` takes keys and builds JSON; only its last six lines duplicate.
3. Delete the hand-rolled `CreateHooksSidecar` compensation at `cmd/hooks_read_lock_test.go:33` and its explanatory comment.
4. In `internal/hooks`, collapse `readOnlyDirPath` and `seedThenDenyWrites` into one helper taking an optional seed body (`seedThenDenyWrites(t, nil)` giving today's `readOnlyDirPath`), and correct the comment asserting the split is necessary.
5. Leave `newTempHooksStore` only if it still has callers the folded helper cannot serve; otherwise delete it too.

**Acceptance Criteria**:
- Package `cmd` has one hooks.json staging helper; package `internal/hooks` has one deny-writes helper.
- Every fixture's sidecar state is set by an explicit field, never by which helper was picked.
- No call site creates a sidecar by hand.
- No doc comment claims a split is necessary that is not.

**Tests**:
- Unit: a fixture with `sidecar: false` degrades on read (records the degradation breadcrumb); one with `sidecar: true` does not. Both directions asserted, so the field cannot silently stop mattering.
- The re-pointed suites passing with an unchanged verdict count.

## Task 13: Make the `hooksDeps` install-and-restore pair inseparable
severity: medium
sources: duplication

**Problem**: Every seam-injecting test writes the same two statements — `hooksDeps = &HooksDeps{…}` followed by `t.Cleanup(func() { hooksDeps = nil })` — at 69 sites: `cmd/hooks_test.go` (38), `cmd/hooks_rm_exit_test.go` (11), `cmd/hooks_write_lock_test.go` (8), `cmd/hooks_pane_token_test.go` (8), `cmd/hooks_read_lock_test.go` (2), `cmd/root_test.go` (2). That is ~140 lines of pure boilerplate, and the pairing is load-bearing rather than cosmetic: the package-level `hooksDeps` var leaks into every later test in the package if the cleanup is ever omitted, and nothing makes the pair inseparable. `cmd/testhelpers_test.go` already exists as the staging home and already owns the `runHookSet` / `runHookRm` / `runHookList` drivers these same tests use.

**Solution**: One helper that assigns and registers the cleanup together.

**Outcome**: The restore cannot be forgotten, and a leaked `hooksDeps` becomes structurally impossible.

**Do**:
1. Add `func withHooksDeps(t *testing.T, deps HooksDeps)` to `cmd/testhelpers_test.go`, assigning `hooksDeps = &deps` and registering `t.Cleanup(func() { hooksDeps = nil })`.
2. Convert all 69 call sites.
3. Grep the package afterwards for any remaining bare `hooksDeps =` assignment outside the helper.

**Acceptance Criteria**:
- No test file assigns `hooksDeps` directly.
- Every conversion preserves the exact deps struct the site installed.
- The package's verdict count is unchanged.

**Tests**:
- Unit: a guard test asserting no `*_test.go` in `cmd` assigns `hooksDeps` outside the helper (source-walking, via `sourceguardtest`, matching the repo's existing guard style).

## Task 14: Single-source the `--pane-key` no-tmux-call poison fixture
severity: medium
sources: duplication

**Problem**: The same fixture — a `mockKeyResolver` and a `recordingPaneStamper` armed with "must not be called on the `--pane-key` path" errors, followed by the paired `resolver.calls != 0` / `len(stamper.calls) != 0` assertions — is written out at seven declaration sites across three files (`cmd/hooks_rm_exit_test.go:109-127,162-178,215,262`, `cmd/hooks_pane_token_test.go:170-187`, `cmd/hooks_write_lock_test.go:236-248`). The drift is already present: `cmd/hooks_write_lock_test.go:236-237` uses `errors.New` where the other five use `fmt.Errorf` for the identical message, and three sites poison only the resolver while three poison both — so *which seam is actually guarded* varies by site rather than by intent. The `--pane-key` path taking no tmux call is a specified behaviour; a site that poisons only one seam does not prove it.

**Solution**: One helper returning both poisoned seams, and one assertion helper for the pair.

**Outcome**: Every `--pane-key` case guards the same two seams, so the specified "no tmux call on this path" behaviour is proved uniformly.

**Do**:
1. Add `paneKeyPathSeams() (*mockKeyResolver, *recordingPaneStamper)` beside the other hook-key fakes in `cmd/hookkey_vocabulary_test.go`, returning both armed with the poison error.
2. Add `assertNoPaneTmuxCalls(t, resolver, stamper)` for the assertion half.
3. Convert all seven sites. The three that poisoned only the resolver gain the stamper guard — verify each still passes, and if one fails, that is a real gap the duplication was hiding: report it rather than weakening the helper.

**Acceptance Criteria**:
- One declaration of the poison fixture and one of its assertion.
- All seven `--pane-key` cases guard both seams.
- The poison error message is identical at every site.

**Tests**:
- Unit: the seven converted cases pass. A deliberate temporary edit making the `--pane-key` path call the resolver must fail all seven (verify once during implementation; do not commit the edit).

## Task 15: Finish the restore reboot-fixture consolidation
severity: medium
sources: duplication, bank

**Problem**: Three overlapping duplications in the restore test surface. (a) The ~30-line arrange — build the binary, prepend PATH, `IsolateStateForTest`, state dir + `EnsureDir`, `PORTAL_HOOKS_FILE`, hook-fire file, `store.Set(stableKey, …)`, `tmuxtest.New`, `new-session`, `WaitForSession`, `StampPaneToken` — appears verbatim in `runRenameRebootFire` (`internal/restore/rename_reboot_hook_integration_test.go:76-106`) and `TestRenameRebootHook_DurableAcrossRepeatedReboots` (`internal/restore/rename_reboot_durability_integration_test.go:26-56`), differing only in the socket prefix. (b) The ~27-line reboot-and-hydrate — `KillServer`, the `list-sessions` must-fail guard, `EnsureServer`, `OpenTestLogger`, `restore.Orchestrator{…}`, `RestoreWithMarker`, the `list-panes "0:0"` check, `DriveSignalHydrate`, `WaitForSkeletonMarkersCleared` — is open-coded at `rename_reboot_hook_integration_test.go:132-158` even though `rebootAndHydrate` in the *same package* (`rename_reboot_durability_integration_test.go:123`) already is that function; `cmd/noncontiguous_window_reboot_integration_test.go`'s `reboot` method is a third copy of its first half. These two files share `rename_reboot_shared_test.go` as a declared home, so the split is accidental. (c) Two loose ends in the same neighbourhood: `cmd/bootstrap/reboot_roundtrip_test.go:110-116` and `:624-629` each re-author the literal + `SanitizePaneKey` → `ScrollbackFile` → `WriteFile` that `restoretest.SeedScrollback` now owns; and the marker bracket's *set* half is unobserved — deleting `client.SetServerOption(state.RestoringMarkerName, "1")` from `internal/restoretest/restore_marker.go:21-23` leaves both lanes of `./internal/restore/` green, because only the unset is asserted (`internal/restore/integration_test.go:80-82`, `integration_full_test.go:282-288`).

**Solution**: One shared arrange and one shared reboot-and-hydrate in the declared home; finish the scrollback seeder consolidation; assert the marker set half once, in the shared bracket.

**Outcome**: The reboot fixture is authored once per package; the shared bracket's set half is covered for every caller at once.

**Do**:
1. Move the arrange into `internal/restore/rename_reboot_shared_test.go` as `newRenameRebootFixture(t, socketPrefix) (*tmuxtest.Socket, *tmux.Client, hooksPath, hookFireFile string)`.
2. Move `rebootAndHydrate` into the shared file beside it and have `runRenameRebootFire` call it instead of open-coding it.
3. Promote the kill-server → `EnsureServer` → `Orchestrator` → `RestoreWithMarker` sequence into `internal/restoretest` beside `RestoreWithMarker` (e.g. `RebootAndRestore(t, ts, client, stateDir) error`), so `cmd/noncontiguous_window_reboot_integration_test.go` can drop its third copy.
4. Fold `cmd/bootstrap/reboot_roundtrip_test.go:110-116` and `:624-629` into `restoretest.SeedScrollback`. Keep `verifyANSIScrollback` (`:418-436`) as the assertion; the shared const's ANSI prefix now has a caller that justifies it.
5. Add one assertion inside `restoretest.RestoreWithMarker` (or a test of it) that the marker is *set* while `Restore()` runs — one assertion covering every caller.

**Acceptance Criteria**:
- The arrange and the reboot-and-hydrate each exist once in `internal/restore`.
- `cmd/noncontiguous_window_reboot_integration_test.go` reaches the shared reboot sequence rather than reimplementing its first half.
- No test in `cmd/bootstrap` re-authors the scrollback seed.
- Deleting the `SetServerOption` line from `restore_marker.go` makes a test fail.

**Tests**:
- Integration: the existing rename-reboot and non-contiguous-window suites pass unchanged after the moves (`-p 1`).
- Integration/unit: the new set-half assertion fails when the set is removed (verify during implementation).

## Task 16: Poll the hook-fire marker assertions to a deadline
severity: high
sources: bank

**Problem**: Two helper families read a hook-fire marker file with no wait, racing the hydrate helper — and both flakes have been reproduced. `assertHookFireCount` (`internal/restore/rename_reboot_shared_test.go:47-57`) does a bare `os.ReadFile` immediately after `rebootAndHydrate` returns; that function's last wait is `WaitForSkeletonMarkersCleared`, which clears **before** the helper execs `sh -c '<HOOK>; exec $SHELL'`. Reproduced at 1-in-5 iterations under `-count=5` on the current tree (`TestRenameRebootHook_PaneProcessKeptRunning`) and at HEAD (`TestRenameRebootHook_ExternalRename`), plus `TestMultiPaneLegacy_PerPaneHookRouting` and `TestRenameRebootHook_DurableAcrossRepeatedReboots`. The same shape sits in `assertMarkerFiredOnce` / `assertMarkerAbsent` (`internal/restore/multipane_legacy_integration_test.go:213,224`), where every caller in the file inherits it. This matters more than an ordinary flake: the work unit's headline guarantee — a resume hook fires after a reboot across a rename — now rides a racy read, so a genuine regression and a scheduling blip are indistinguishable. The two helpers are also pairwise generalisations: one hardcodes the marker `HOOK_FIRED` and parameterises the count, the other parameterises the marker and fixes the count at 1. One helper serves both.

**Solution**: One polled marker-count helper with a bounded deadline, matching `WaitForSkeletonMarkersCleared`'s own shape.

**Outcome**: A hook that fires late passes; a hook that never fires still fails, at the bound, with the same diagnostic.

**Do**:
1. Write one `assertMarkerCount(t, path, marker string, want int)` that polls to a bounded deadline, in the shape `WaitForSkeletonMarkersCleared` already uses in this package (same bound source, same tick).
2. Handle the absent-file case as "count 0 so far, keep polling" rather than an immediate `t.Fatalf` — today's `assertHookFireCount` fatals on ENOENT, which is precisely the race.
3. Preserve both current diagnostics: the bare-shell-miss hint on a zero count, and the CROSS-FIRE message when a marker leaks into the wrong pane's file.
4. Replace `assertHookFireCount`, `assertMarkerFiredOnce` and `assertMarkerAbsent` with calls to it (`assertMarkerAbsent` becomes want-0 — note it must still poll, then assert absence at the deadline, not return early).
5. Re-run the four named tests under `-count=5` to confirm the flake is gone.

**Acceptance Criteria**:
- One polled helper serves the hook-fire and multipane marker assertions.
- No marker assertion reads the file exactly once.
- The absent-file case does not fail before the deadline.
- `TestRenameRebootHook_PaneProcessKeptRunning`, `TestRenameRebootHook_ExternalRename`, `TestRenameRebootHook_DurableAcrossRepeatedReboots` and `TestMultiPaneLegacy_PerPaneHookRouting` pass 5/5 under `-count=5`.

**Tests**:
- Integration: the four named tests under `-count=5`, serially (`-p 1`).
- Integration: a want-0 case still fails when the marker *does* appear (the absence assertion must not become vacuous).

## Task 17: Register the state-dir teardown guard on the saver-hosting hook-cleanup fixture
severity: high
sources: bank

**Problem**: `cmd/state_daemon_hook_cleanup_integration_test.go:52` spawns a `_portal-saver`-hosted daemon that flushes on SIGHUP at teardown but never calls `portaltest.RegisterStateDirTeardownGuard` — verified: the file contains no such call, while its siblings all register it (`cmd/state_commit_now_reentrancy_integration_test.go:44`, `internal/tmux/portal_saver_endstate_integration_test.go:36`, and six more). This is the direct cause of a reproduced `TempDir RemoveAll: directory not empty` failure (1 in 4 runs): every assertion logs success first, then cleanup unlinks a state dir the daemon is still writing. CLAUDE.md prescribes the guard for exactly this fixture shape — "fixtures whose tmux server can host writers at teardown (a saver daemon's SIGHUP flush, session-closed hook subprocesses)".

**Solution**: Register the guard, in the prescribed position.

**Outcome**: The bounded saver-exit and write-quiescence wait runs between kill-server and the TempDir RemoveAll, and the fixture stops failing on a clean tree.

**Do**:
1. Add `portaltest.RegisterStateDirTeardownGuard(t, stateDir)` to the fixture — registered **after** `IsolateStateForTest`, **before** `tmuxtest.New`, matching the documented ordering and the sibling fixtures.
2. Sweep the rest of `cmd` and `internal` for other saver-hosting integration fixtures missing the guard and register it there too.

**Acceptance Criteria**:
- The fixture registers the guard in the prescribed position.
- No saver-hosting integration fixture in the repo lacks the guard.
- The suite passes 5/5 consecutive runs.

**Tests**:
- Integration: `cmd/state_daemon_hook_cleanup_integration_test.go` run 5 times consecutively with no teardown failure (`-p 1`).

## Task 18: Split the hooks scaffolding out of `internal/transienttest`
severity: medium
sources: architecture, bank

**Problem**: `transienttest` was the shared `list-panes -a` failure-mode scaffolding for the two destructive integration suites — one job, named for it. It now also holds (a) the hook-key seed vocabulary (`ReapableHookKey`, `UnjudgeableHookKey`), (b) the `hooks.json` sidecar lock fixture (`CreateHooksSidecar`, `HoldHooksSidecar`, `HoldHooksSidecarShared` in `internal/transienttest/hooks_lock.go`) and (c) a log-assertion helper (`AssertDegradedRead`, `UnlockedRecords`) that pulls in a dependency on `internal/logtest`. Its package doc had to be rewritten into a three-clause list to describe what it is, which is the tell. Its consumers are now unit-lane tests in `internal/hooks` and `cmd` — packages with no relationship to a transient `list-panes` failure. Every other test-helper package in this repo is single-purpose and named for its one job (`tmuxtest`, `portalbintest`, `sourceguardtest`, `logtest`, `themetest`, `spawntest`, `restoretest`), and the pattern holds because it makes "where does this helper go?" answerable. This one is now the dumping ground for anything two hook suites happen to share.

**Solution**: A new `internal/hookstest` for the hooks scaffolding; `transienttest` keeps only what its name describes.

**Outcome**: Each test-helper package has one subject and a one-sentence doc; the sidecar fixture, the seed vocabulary and the degraded-read assertion sit together where a hooks test looks for them.

**Do**:
1. Create `internal/hookstest` holding the seed key vocabulary, the sidecar fixture (`hooks_lock.go` in full) and the degraded-read assertion. They are one coherent subject — how a test stages and interrogates `hooks.json` — with a different consumer set from the transient-`list-panes` commander.
2. Leave `transienttest` holding only `Commander` / `FailureMode` / `PassThrough` / `FailExitNonZero` / `FailEmptyStdout` / `SocketCommander`, and restore its package doc to one sentence.
3. Decide where `SeedHooksJSON` / `HooksJSONBytes` / `ResolveHooksFilePathFromEnv` belong — they are hooks seeders consumed by the two destructive suites, so `hookstest` with `transienttest`'s consumers importing both is the honest split.
4. Re-point every consumer.
5. Update CLAUDE.md's helper-package row to describe both packages accurately — the current row omits `hooks_lock.go` entirely.

**Acceptance Criteria**:
- `transienttest`'s package doc is one sentence naming one job.
- `internal/hookstest` holds the hooks staging/interrogation surface and nothing else.
- `transienttest` no longer imports `internal/logtest`.
- CLAUDE.md's row describes what each package actually holds.
- Both lanes green.

**Tests**:
- Both lanes passing with an unchanged verdict count is the test for a pure move; no new case is warranted.

## Task 19: Promote the shared log-test helpers into `internal/logtest`
severity: medium
sources: bank

**Problem**: Four families of duplicated log-test scaffolding, all with `internal/logtest` as their natural home (it already owns `Sink`, `OnlyRecord` and `RecordsAtLevel`). (a) The four-line sink-install helper `func install…(t *testing.T) *logtest.Sink { t.Helper(); sink := &logtest.Sink{}; log.SetTestHandler(t, sink); return sink }` is duplicated verbatim under seven names across six packages: `cmd/hooks_read_lock_test.go:17`, `cmd/config_migrate_logging_test.go:13`, `cmd/state_commit_now_test.go:19`, `internal/hooks/store_test.go:23`, `internal/alias/store_logging_test.go:14`, `internal/project/store_logging_test.go:16`, `internal/storelog/clean_stale_test.go:15`, `internal/spawn/terminalsconfig_test.go:14`. The `logtest → internal/log` edge is feasible with no cycle: `internal/log` never imports `logtest` and its own tests are almost all `package log`. (b) Four hand-rolled `(component, msg)` exactly-one-record filters: `cmd/bootstrap/clean_sweep_summary_test.go:18,30` and `internal/state/fifo_sweep_summary_test.go:19,31` are byte-for-byte the same `summariesFor` + `onlySummary` pair; `cmd/state_daemon_cycle_summary_test.go:21,33` is the same body with `capture`/`tick complete` hardcoded; `cmd/state_hydrate_test.go`'s `hydrateRecord` is the same filter with `hydrate` hardcoded. A `RecordsWith(component, msg)` / `OnlyRecordWith(t, component, msg)` pair subsumes all four. (c) `warnRecords` (`internal/spawn/terminalsconfig_test.go:21-29`), consumed 16 times across five spawn test files, filters on `r.Level == slog.LevelWarn` — **not** `>=` — so it is not a drop-in for `RecordsAtLevel`; it needs an exact-level sibling accessor, or 16 assertions widen to include ERROR. (d) The same five-property audit-breadcrumb block (level / msg / component / op / via, then a per-case attr) is written inline at `cmd/config_migrate_logging_test.go:35-60` and recurs per-package in `internal/alias/store_logging_test.go`, `internal/project/store_logging_test.go`, `internal/hooks/store_test.go` and `internal/storelog/clean_stale_test.go`, each with its own component.

**Solution**: Move the four families into `internal/logtest` beside the typed accessors.

**Outcome**: `internal/logtest` owns the sink lifecycle, the level filter and the record-shape assertion for the whole repo; a change to the rendered-body contract reaches every consumer.

**Do**:
1. Add `logtest.Install(t) *Sink` (or similarly named) performing the four-line install, and re-point all eight copies. Confirm the `logtest → internal/log` import introduces no cycle before converting.
2. Add `RecordsWith(component, msg)` and `OnlyRecordWith(t, component, msg)`; re-point the four hand-rolled filters.
3. Add an **exact-level** accessor beside `RecordsAtLevel` and re-point `internal/spawn`'s `warnRecords` to it. Do not widen the 16 spawn assertions to `>=`.
4. Add a component-parameterised record assertion (the five-property block with `component` as a field rather than a constant) and re-point the five inline copies. This is a design change to the assertion helper, so keep the existing `hooksRecordWant` ergonomics working for its current callers.
5. Each sub-item is independently completable; land them in whatever order suits, but leave no half-converted family.

**Acceptance Criteria**:
- No test file outside `internal/logtest` declares a sink-install helper, a `(component, msg)` filter, or a level filter.
- The exact-level and at-or-above-level accessors are distinct and named so the difference is unmissable.
- `internal/logtest` imports `internal/log` with no cycle.
- Verdict count unchanged across all affected packages.

**Tests**:
- Unit: `internal/logtest`'s own tests cover the new accessors, including the exact-vs-at-or-above distinction (a record at ERROR must not satisfy the exact-WARN accessor).

## Task 20: Close the low-severity helper duplication in the hooks test surface
severity: low
sources: duplication, bank

**Problem**: Seven small duplications of the same class — an extracted helper exists and half the call sites bypass it, or two near-identical helpers sit side by side. (a) `assertHooksFileUnchanged(t, path, before, context...)` (`cmd/testhelpers_test.go:177-187`) is the shared "prove the route left the file byte for byte" assertion with 13 callers, yet six sites open-code it inline (`cmd/run_hook_stale_cleanup_test.go:331,391,413,583,612,728`); one of them additionally re-implements `readFileBytes` with a raw `os.ReadFile` + `t.Fatalf` pair on both ends. An improvement to the failure message reaches half the suite. (b) `standDownRecord` (`cmd/run_hook_stale_cleanup_test.go:538-554`) and `lockStandDownRecord` (`cmd/hook_sweep_lock_timeout_test.go:30-42`) both assert exactly one stand-down record and run it through `assertHooksRecord(t, rec, standDownWant(level))`; they differ only in the expected level (DEBUG vs WARN), the expected reason, and whether "exactly one" is taken over all records or over WARN records. (c) `cmd/hooks_write_lock_test.go:180-204` and `:252-277` are line-for-line identical ~25-line subtests apart from `runHookSet` vs `runHookRm` and two failure-message strings — same lowered bound, same held sidecar, same assertions, and the 20× "generous multiple" now lives in two places. (d) The sidecar-is-free probe ("open `<path>.lock`, take `LOCK_EX|LOCK_NB` from a fresh fd, then unlock") is written three ways: `assertSidecarFree` (`internal/hooks/lock_test.go:36-49`), inline inside the enumeration `during` hook (`cmd/hook_sweep_snapshot_order_test.go:50-61`), and as the `openSidecar` half of the `transienttest` holders — it is the one member of that fixture family that was never promoted. (e) `replayCfg` (`cmd/state_hydrate_replayed_log_test.go:16-29`) and `timeoutCfg` (`cmd/state_hydrate_test.go:938-951`) build a `hydrateConfig` from the same seven positional parameters, differing only in the `OpenFIFO` seam, whether `HandleFileMissing` is set, and whether the `Commander` is supplied; `cmd/state_hydrate_empty_hookkey_test.go:73-74` bypasses both and writes the struct literal inline. (f) `cmd/state_daemon_test.go:792,813` and `cmd/version_guard_test.go:146` write `t.Setenv("PORTAL_HOOKS_FILE", filepath.Join(t.TempDir(), "hooks.json"))`, byte-equivalent to the existing bare `hooksFileInTempDir(t)` (`cmd/testhelpers_test.go:92`). (`cmd/cleanstale_transient_listpanes_shared_test.go:26` is NOT a candidate — it nests under a `portal/` subdir.) (g) `internal/hooks/lookup_test.go` repeats a five-line seed preamble across 11 subtests and the identical err/ok/cmd no-hook triple across eight.

**Solution**: Route each bypassing site through the helper that already exists; collapse the near-identical pairs.

**Outcome**: One home per assertion, so an improvement to a failure message reaches every caller.

**Do**:
1. Replace the six inline comparisons with `assertHooksFileUnchanged(t, path, before, "<existing per-case wording>")`, and the raw `os.ReadFile` pair with `readFileBytes` + the same helper.
2. Collapse `standDownRecord` / `lockStandDownRecord` into one `assertStandDown(t, sink, level, reason)` in the file that owns `standDownWant`, selecting the record set from the level argument.
3. Extract `assertReturnsAtLockBound(t, verb string, run func() (string, error))` in `cmd/hooks_write_lock_test.go` and drive both subtests from it, so the 20× multiplier lives once.
4. Export `AssertSidecarFree(t, hooksPath)` beside the sidecar holders (in whichever package owns them after Task 18), implemented over the existing `openSidecar`, and use it from `internal/hooks/lock_test.go` and `cmd/hook_sweep_snapshot_order_test.go`.
5. Keep one `hydrateConfig` builder taking the `OpenFIFO` seam and the commander as its varying arguments; construct the timeout, replay and empty-hook-key suites through it. Pre-existing inline literals elsewhere in `cmd/state_hydrate_test.go` are outside this work unit's changes and need not be swept.
6. Re-point the three `PORTAL_HOOKS_FILE` sites at `hooksFileInTempDir(t)`.
7. Extract the seed preamble and the no-hook triple in `internal/hooks/lookup_test.go` as two helpers and convert the 11 subtests.

**Acceptance Criteria**:
- No inline re-implementation of an assertion the package already exports as a helper remains in the hooks test surface.
- The 20× lock-bound multiplier appears once.
- The sidecar-free probe appears once.
- Verdict count unchanged.

**Tests**:
- The converted suites passing with an unchanged verdict count.

## Task 21: Give the deny-writes fixture a guard that detects its own trap
severity: medium
sources: bank

**Problem**: Both subtests of `TestHookSweepDiscriminatesLockTimeoutFromFailure` (`cmd/hook_sweep_lock_timeout_test.go:194,223`) still pass with the sidecar staging removed from the `writesDenied` path — the control was run. The first only asserts `!errors.Is(err, hooks.ErrLockHeld)`, which a lock-open `EACCES` also satisfies; the second's self-guard checks the error text carries the sentinel, which the directory name supplies on either path. The fixture's actual subject — `error_class=write-failed-temp-create` — is asserted nowhere at this level. This fixture has now silently stopped testing its own subject once (caught during phase 5) and been shown undetectable twice.

**Solution**: Assert the write-phase classification the fixture exists to produce.

**Outcome**: A staging change that turns the save failure into a lock-open failure fails the test rather than passing it.

**Do**:
1. Add to the first subtest an assertion that the error carries `fileutil.ErrWriteTempCreate` (`errors.Is`), or that the sink holds the WARN `clean-stale` record with `error_class=write-failed-temp-create`.
2. Keep the existing `!errors.Is(err, hooks.ErrLockHeld)` assertion — it guards the discrimination the test is named for; the new assertion guards the fixture.
3. Verify by control: remove the sidecar staging from the `writesDenied` path and confirm the test now fails, then restore it.

**Acceptance Criteria**:
- The write-phase error class is asserted at this level.
- Removing the sidecar staging from `writesDenied` makes the test fail.
- The lock-timeout-vs-failure discrimination assertions are unchanged.

**Tests**:
- Unit: the two subtests, plus the control run described above performed during implementation.

## Task 22: Retire the three subsumed test cases
severity: low
sources: bank

**Problem**: Three cases assert a strict subset of a sibling and can no longer carry the intent their names claim; each was left in place because the owning task's acceptance bar forbade a case-count change, so they need an owner who can approve it — which this staging is. (a) `TestDoctorFixPrunedHookOutput` (`cmd/doctor_test.go:1665`, "it leaves doctor --fix stdout unchanged") asserts a strict subset of `TestDoctorFixPrunesStaleEntriesThenRediagnosesClean` — same fixture, same `assertStalePrunesApplied` helper, no additional assertion — since the exact-equality pruned-line assertion was single-sourced into that helper. (b) `cmd/doctor_test.go:1140` ("it reads the marker before counting") is character-for-character identical to `:1116` ("it reports not evaluable while the restore marker is set") — same seed, same `stubAllPaneLister{restoring: true}`, same assertion; the `strings.Contains` check that distinguished them was inert and removed, so no coverage is lost, but the intent its name carries (the marker is read *before* the count is computed) is now indistinguishable from its neighbour. The sibling at `:1132` ("it reads the marker before the empty-live-set branch") is genuinely distinct and stays. (c) `cmd/hooks_test.go:520` ("it errors when TMUX_PANE is unset for set") shares its entire fixture and assertion set with `cmd/hooks_test.go:362` ("returns error when TMUX_PANE is not set") and asserts a strict subset — verified: it omits the `os.Stat(hooksFile)` not-created check the first makes.

**Solution**: Either delete each duplicate, or differentiate it so it earns its name.

**Outcome**: No test case in the suite is a strict subset of a sibling under a name promising something it does not check.

**Do**:
1. Delete `TestDoctorFixPrunedHookOutput`.
2. For `cmd/doctor_test.go:1140`, take one of two remedies: delete it (coverage unchanged), or differentiate it by seeding a fixture where a count *would* otherwise be produced and asserting its absence by a route the equality check does not already cover. Prefer differentiation if the "marker read precedes the count" ordering is worth pinning; otherwise delete.
3. Delete `cmd/hooks_test.go:520`, whose surviving sibling makes every assertion it makes plus one.
4. Record the case-count delta in the commit message so the reduction is deliberate and visible.

**Acceptance Criteria**:
- Each of the three is deleted or differentiated; none remains as a strict subset.
- No assertion present today is lost: for each deletion, name the surviving test that makes it.
- `cmd/doctor_test.go:1132` is untouched.

**Tests**:
- The surviving suites passing, with the case-count delta stated.

## Task 23: Rename the remaining concern-named test files in `cmd`
severity: low
sources: bank

**Problem**: The convention is that a test file in `cmd` touching `doctor.go` / `run_hook_stale_cleanup.go` is named after the source file it exercises, not after the concern. Three files still breach it — `cmd/hooks_cleanstale_single_caller_guard_test.go`, `cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go` and `cmd/rename_restore_cleanup_survival_integration_test.go` (a fourth, `cmd/hookkey_no_regression_upgrade_test.go`, named in the bank entry, no longer exists — verified). All three predate this work unit, which is why they fell outside the scoped rename, so the acceptance criterion "every remaining test file in `cmd` touching these two sources is named after a source file" is satisfied only for the phase's own output.

**Solution**: Rename to match the convention.

**Outcome**: The convention holds for the whole package, not just the files this work unit authored.

**Do**:
1. Rename each of the three to a source-derived name (e.g. `doctor_*_test.go` / `run_hook_stale_cleanup_*_test.go`), preserving the `_integration_test.go` suffix and build tag on the two integration-lane files.
2. Do not change file contents beyond any package-level comment that names the old filename.
3. Re-check the package for any further breach introduced since, and rename those too.

**Acceptance Criteria**:
- Every test file in `cmd` exercising `doctor.go` or `run_hook_stale_cleanup.go` is named after a source file.
- Build tags and lane membership are unchanged.
- No test content changed.

**Tests**:
- Both lanes green; the integration-tagged files still excluded from the unit lane.

## Task 24: Document the shape-aware reaper and the token vocabulary in CLAUDE.md
severity: medium
sources: bank

**Problem**: CLAUDE.md never describes the retain-old-format-forever rule or the token-shape surfaces, and the consequence is the dangerous one CLAUDE.md exists to prevent. The sweep now retains any key that is not token-shaped, and `internal/session/panetoken.go` (`NewPaneToken`) and `internal/session/tokenshape.go` (`IsTokenShaped`) were added. Verified: CLAUDE.md mentions `NewPaneToken` once in passing (line 166) and "not token-shaped" once in the `hook list` sentence (line 45); `IsTokenShaped` and the retention rule appear nowhere, and the `session` row (line 64) still describes the package as session-creation-only. An old-format entry sitting inert forever therefore reads as cruft to a future agent, who deletes exactly the entries the safety argument depends on. Separately, the helper-package row (line 85) describing `transienttest` omits `internal/transienttest/hooks_lock.go` entirely (`CreateHooksSidecar`, `HoldHooksSidecar`, `HoldHooksSidecarShared`, `UnlockedRecords`, `AssertDegradedRead`).

**Solution**: Amend CLAUDE.md so the retention rule and the token vocabulary are stated where a future agent will read them before touching either.

**Outcome**: An agent reading CLAUDE.md learns that a non-token-shaped `hooks.json` entry is deliberately retained forever, and why deleting it would be wrong.

**Do**:
1. Add the retention rule to the "Resume hooks" section: the sweep retains any key that is not token-shaped, permanently, because such a key can only have come from a pre-token install and its pane cannot be judged — deleting it destroys a user-authored `on-resume` command with no way to recover it.
2. Amend the `session` package row to name `panetoken.go` / `NewPaneToken` and `tokenshape.go` / `IsTokenShaped`, so the row stops describing the package as session-creation-only.
3. Amend the `hooks` package row to name the shape-aware staleness rule as the reason the store consults the token vocabulary.
4. Amend the helper-package row so `transienttest`'s described surface matches its files. (If Task 18 lands first, describe the post-split packages instead.)
5. Keep the amendments in CLAUDE.md's existing register: dense, load-bearing, no section numbers, no spec cross-references.

**Acceptance Criteria**:
- The retain-forever rule is stated in CLAUDE.md with its justification.
- `IsTokenShaped` and `NewPaneToken` are both named in the `session` row.
- The helper-package row describes every file in the package(s) it covers.
- No CLAUDE.md statement about these surfaces is false after the edit.

**Tests**:
- Documentation only; no test. Verify by grep that each named symbol exists at the path the doc claims.

## Task 25: Clear the repo-wide lint and format debt
severity: low
sources: bank

**Problem**: On a clean tree, `gofmt -l .` reports `internal/tui/help_modal_test.go` as unformatted, and `golangci-lint run ./...` reports 30 `modernize` findings (`errorsastype`, `stringscut`, `stringsseq`) across the repo — verified, including `cmd/root.go:199`, `cmd/bootstrap_progress.go:133`, `main.go:64` and `main.go:75`. All predate this work unit. Lint is not wired into CI, so the debt only surfaces when someone runs it locally, and it makes a real new finding harder to spot in the noise.

**Solution**: One sweep.

**Outcome**: `gofmt -l .` is silent and `golangci-lint run ./...` reports zero issues on a clean tree.

**Do**:
1. Run `gofmt -w internal/tui/help_modal_test.go`.
2. Apply the 30 `modernize` fixes. These are mechanical (`errors.As` → `AsType[T]`, `strings.Cut`, `strings.SplitSeq`); apply them by hand or via the linter's autofix, then read the diff — do not accept an autofix unread.
3. Confirm both lanes stay green: `go test ./...` and `go test -tags integration -p 1 ./...`.

**Acceptance Criteria**:
- `gofmt -l .` prints nothing.
- `golangci-lint run ./...` reports zero issues.
- Both lanes green.
- No behavioural change in the diff.

**Tests**:
- Both lanes green after the sweep; the diff reviewed for behaviour-neutrality.
