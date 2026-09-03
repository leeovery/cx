# Analysis Tasks: resume-hooks-silently-lost (Cycle 2)

## Task 1: A colon-bearing session name is now silently dropped from `sessions.json`
severity: high
sources: bank

**Problem**: `Client.ShowEnvironment` (`internal/tmux/tmux.go:585`) now composes `show-environment -t =<session>`. tmux accepts `:` in a session name, and the exact form fails on one: measured on tmux 3.7c on an isolated socket, with a live session literally named `a:b`, `show-environment -t =a:b` exits **1** with `no such session: =a:b`, where the pre-change bare `-t a:b` exited 0 and returned 14 lines. That stderr matches `noSuchSessionStderrSubstr` (`internal/tmux/errors.go:30`), so `internal/state/capture.go:69-73` classifies it as **natural churn**, logs a WARN naming a "vanished session", and `continue`s — the session is omitted from the capture. It is therefore never saved and never restored. The failure moved from *mis-targeted* to *silently absent*, which is the exact class this work unit exists to remove, arriving through the change the work unit made. Portal's own `GenerateSessionName` strips `:`, so the exposure is a user-renamed session — and `internal/tui/model.go`'s rename path passes the typed name straight through with no validation, as does `Client.RenameSession`.

**Solution**: Close the gap at **both** ends — refuse the name Portal would otherwise write, and stop the capture loop from swallowing a name that arrived some other way.

**Making the target vocabulary round-trip a colon was considered and rejected on measurement.** tmux offers no escape for a colon in a session name under exact targeting; against a live session named `a:b` on an isolated socket, every form fails identically:

```
tmux -L <sock> show-environment -t '=a:b'    → no such session: =a:b
tmux -L <sock> show-environment -t '=a\:b'   → no such session: =a\:b
tmux -L <sock> show-environment -t '=a:b:'   → no such session: =a:b:
```

The colon is a structural separator in tmux's target grammar, so quoting or escaping the session component is building something tmux does not support. Do not attempt it.

**Outcome**: Portal refuses to write a colon into a session name, naming the character in the refusal; and a colon-named session that reached the server by some other route is either captured or reported as an anomalous error — never counted as natural churn and silently absent from `sessions.json`.

**Do**:
1. Reject `:` in a session name at Portal's write boundary — validate in `Client.RenameSession` and the TUI rename path, so a name the target vocabulary cannot round-trip never enters the server through Portal. A refused rename is a deliberate user-facing behaviour change: it needs a clear message naming the character and why.
2. Narrow the classification in `internal/state/capture.go` so a session unresolvable by Portal's own target form becomes an anomalous error rather than natural churn. Step 1 covers only names Portal writes; a session renamed with `tmux rename-session` directly, or created before Portal, still carries a colon and would otherwise keep vanishing silently. Removing silent loss is this work unit's whole subject, so the half that stays silent is the half that matters.
3. Give step 2's discrimination a home the capture loop can classify against without an import cycle: a sentinel in the `internal/tmuxerr` leaf for a session name Portal's exact-target vocabulary cannot round-trip, returned by `Client.ShowEnvironment` (`internal/tmux/tmux.go:585-589`) in place of the `wrapNoSuchSession` classification when the name carries a colon. `internal/state/capture.go:69-79` then keeps its `errors.Is(err, tmuxerr.ErrNoSuchSession)` natural-churn branch untouched and the new sentinel falls through to `anomalousErrs`.
4. Add a real-tmux regression test on an isolated `-L` socket that creates a session literally named `a:b`, runs `CaptureStructure` against it, and asserts the session is surfaced rather than dropped — the current suite has no colon-named session anywhere, which is why the change shipped green.

**Acceptance Criteria**:
- [ ] `Client.RenameSession` returns an error naming `:` and issues no `rename-session` argv when `newName` contains one.
- [ ] The TUI rename path refuses the same name and surfaces the reason to the user rather than renaming or failing silently.
- [ ] A `ShowEnvironment` failure against a colon-bearing session name does not satisfy `errors.Is(err, tmuxerr.ErrNoSuchSession)`.
- [ ] `CaptureStructure` routes that session to `anomalousErrs` and its WARN does not describe a vanished session.
- [ ] A live colon-named session is present in the `Index` a real-tmux capture returns, or the capture reports the error — it is never absent from a successful capture.
- [ ] Names with no colon rename and capture exactly as before, on both the client and TUI paths.

**Tests**:
- `"it refuses a rename to a name containing a colon"`
- `"it names the offending character in the refusal message"`
- `"it issues no tmux command for a refused rename"`
- `"it refuses a colon-bearing rename from the TUI and reports why"`
- `"it classifies an unaddressable session name as anomalous rather than natural churn"`
- `"it surfaces a colon-named session in a real-tmux capture rather than dropping it"`
- `"it still treats a genuinely vanished session as natural churn"`
- `"it renames a colon-free name unchanged"`

## Task 2: The `_portal-saver` pane probe uses the helper the package's own doc calls wrong, and resolves a prefix sibling
severity: high
sources: bank

**Problem**: `internal/tmux/saver_pane_pid.go:13` and `:37` pass `exactTarget(sessionName)` to `list-panes -t`. `list-panes` takes a *window* target, so `=_portal-saver` is not an exact session reference. Measured on tmux 3.7c on an isolated socket: with only `_portal-saver-old` live, `list-panes -t =_portal-saver -F '#{pane_pid}'` returns **the sibling's pid with exit 0**, while `list-panes -t '=_portal-saver:'` correctly fails with `can't find session`. The `exactTarget` doc comment this work unit itself wrote (`internal/tmux/tmux.go:416-421`) says exactly this — "It is the wrong helper for a `-t` tmux parses as a window or pane target … list-panes then falls through to the same fuzzy lookup and reaches the prefix sibling anyway … Route those through `exactCoordTarget`" — so the package documents the defect and the two call sites contradict it. `SaverPanePIDOrAbsent` is the tri-state feeding the bootstrap orphan sweep, the daemon's Component D self-supervision probe and the `_portal-saver` end-state assertions: a prefix-sibling saver session can make a divergent daemon believe it is still the live saver pane process, or make the orphan sweep spare the wrong pid.

**Solution**: Route both sites through `exactCoordTarget`, and cover them with a real-tmux subtest that stages a prefix sibling (`_portal-saver-old` live, `_portal-saver` absent) and asserts the read fails rather than returning the sibling's pid. While there, resolve the second half the reviewer measured: `wrapNoSuchSession` at `:15` cannot fire on this path — `list-panes` emits `can't find session` / `can't find window`, never `no such session` — so `SaverPanePIDOrAbsent`'s documented collapse of `ErrNoSuchSession` into `present=false` is dead on this route and the tri-state's real absence signal is something else. Either widen the sentinel's stderr match to cover `list-panes`'s wording or correct the contract to describe what actually happens.

**Outcome**: Both `_portal-saver` pane reads address the session exactly, so a live `_portal-saver-old` with no `_portal-saver` fails the read instead of yielding the sibling's pid; and `SaverPanePIDOrAbsent`'s tri-state names the absence signal the route actually produces.

**Do**:
1. Replace `exactTarget(sessionName)` with `exactCoordTarget(sessionName)` at `internal/tmux/saver_pane_pid.go:13` (`saverPanePID`) and `:37` (`SaverPaneID`).
2. Measure what `list-panes -t '=_portal-saver:'` writes to stderr against a server with no such session on an isolated socket, then settle the absence signal one of two ways: widen `noSuchSessionStderrSubstr` / `wrapNoSuchSession` (`internal/tmux/errors.go:28-43`) to cover that wording so `ErrNoSuchSession` stays the single discriminator, or drop the now-dead `wrapNoSuchSession` call at `:15` and correct `SaverPanePIDOrAbsent`'s doc (`:49-52`) to name the sentinel this route does produce. Either way `SaverPanePIDOrAbsent` must still collapse a genuinely absent session to `(0, false, nil)`.
3. Add a real-tmux subtest on an isolated `-L` socket staging `_portal-saver-old` live with `_portal-saver` absent, asserting the read does not return the sibling's pid.
4. Re-run the three consumers' existing coverage — the bootstrap orphan sweep, the daemon's Component D self-supervision probe, and the `_portal-saver` end-state assertions — against the changed target form.

**Acceptance Criteria**:
- [ ] Neither `saver_pane_pid.go` call site passes `exactTarget`.
- [ ] With `_portal-saver-old` live and `_portal-saver` absent, `SaverPanePIDOrAbsent` reports absence and never the sibling's pid.
- [ ] A live `_portal-saver` still resolves to its own pane pid.
- [ ] A genuinely missing session (no sibling present) still collapses to `(0, false, nil)`.
- [ ] A non-absence failure still returns `(0, false, err)`.
- [ ] The doc comment on `SaverPanePIDOrAbsent` describes the sentinel the route emits, with no unreachable claim left.

**Tests**:
- `"it does not resolve a prefix-sibling saver session"`
- `"it reports absence when only a prefix sibling is live"`
- `"it returns the pane pid of a live _portal-saver"`
- `"it collapses a missing session to present=false"`
- `"it collapses an empty pane list to present=false"`
- `"it passes a non-absence error through to the caller"`

## Task 3: Restore and reattach integration fixtures escape the mandated state isolation
severity: high
sources: bank

**Problem**: A class of fixtures starts a real tmux server and drives a real restore whose panes exec the hydrate helper, using a bare `t.TempDir()` + `t.Setenv("PORTAL_STATE_DIR", …)` and never calling `portaltest.IsolateStateForTest`. Verified live: `internal/restore/armed_restore_integration_test.go:22-24`, `internal/restore/integration_test.go:53-55`, `internal/restore/integration_full_test.go:41,68`, `internal/restore/exit_closes_pane_integration_test.go:110,120`, `cmd/reattach_integration_test.go:80-82`, `cmd/bootstrap/eager_signal_hydrate_integration_test.go`, `cmd/bootstrap/phase2_hook_fire_integration_test.go`, `cmd/bootstrap/phase5_integration_test.go`, `cmd/bootstrap/phase5_marker_suppression_integration_test.go`, `cmd/bootstrap/scrollback_resumption_test.go:56`. They therefore get no `HOME`/`XDG_CONFIG_HOME` scrub, no fingerprint backstop, no cross-process daemon-pgrep sandbox registry, and no teardown guard — CLAUDE.md's ABSOLUTE INVARIANT surface. The new coverage guard (`internal/portaltest/teardown_guard_coverage_test.go`) is structurally blind to every one of them because its rule keys on `IsolateStateForTest`. Three are strictly worse than the shape task 6-17 fixed: `armed_restore_integration_test.go`, `integration_test.go` and `reattach_integration_test.go` call `tmuxtest.New` **before** `t.TempDir()`, so LIFO runs the state-dir `RemoveAll` *before* `kill-server`. Two adjacent gaps ride the same fix: `IsolateStateForTest` re-points `HOME` at a `t.TempDir()` but does not set `HISTFILE`, so tmux-hosted shells race the same `RemoveAll` writing shell history — worked around locally at `cmd/abridged_integration_test.go:32` and `cmd/concurrent_coldboot_integration_test.go:41,380` with three separate comments; and `cmd/bootstrap/composition_e2e_harness_integration_test.go:58` loses the shared teardown helper's wait-for-the-biggest-writer phase entirely, because the harness deliberately overwrites `daemon.pid` with a pid it SIGKILLs earlier in the LIFO chain, leaving only the 50ms-sampled directory-quiescence loop against a measured ~52ms daemon-alive window.

**Solution**: Retrofit the isolation across the class and widen the guard so it can see them. Set `HISTFILE` to `os.DevNull` inside `IsolateStateForTest` alongside the `HOME` re-point and delete the three local workarounds. Fix the three inverted orderings so the state dir outlives `kill-server`. Give the harness fixture the pid-source variant of the teardown helper it already models at `:74`.

**The guard's rule widens to the state dir itself.** Its trigger changes from "the file calls `portaltest.IsolateStateForTest`" to "the file sets `PORTAL_STATE_DIR` at all" — the current trigger only ever fires on files that already opted into the helper, which is exactly the call this whole class skips, and that property is how a class of ten came to exist unseen. Keying on the state dir catches every fixture in the class and any future one that hand-rolls its own.

**Outcome**: Every restore/reattach fixture that drives a real server against a state dir runs under `IsolateStateForTest` with the teardown guard registered before `tmuxtest.New`, `HISTFILE` is scrubbed once centrally rather than three times locally, and the coverage guard fails any file pairing `PORTAL_STATE_DIR` with a live server that skips either call.

**Do**:
1. Set `HISTFILE` to `os.DevNull` inside `IsolateStateForTest` (`internal/portaltest/isolated_env.go`) beside the `HOME` re-point at `:28`, and add it to the returned env slice so subprocess shells inherit it. Delete the three local workarounds and their comments at `cmd/abridged_integration_test.go:32` and `cmd/concurrent_coldboot_integration_test.go:41,380`.
2. Retrofit the ten fixtures in the class onto `portaltest.IsolateStateForTest` + `t.Setenv("PORTAL_STATE_DIR", stateDir)` + `portaltest.RegisterStateDirTeardownGuard`, replacing the bare `t.TempDir()`: `internal/restore/armed_restore_integration_test.go:22-24`, `internal/restore/integration_test.go:53-55`, `internal/restore/integration_full_test.go:41,68`, `internal/restore/exit_closes_pane_integration_test.go:110,120`, `cmd/reattach_integration_test.go:80-82`, `cmd/bootstrap/eager_signal_hydrate_integration_test.go`, `cmd/bootstrap/phase2_hook_fire_integration_test.go`, `cmd/bootstrap/phase5_integration_test.go`, `cmd/bootstrap/phase5_marker_suppression_integration_test.go`, `cmd/bootstrap/scrollback_resumption_test.go:56`.
3. Fix the three inverted orderings — `armed_restore_integration_test.go`, `integration_test.go`, `reattach_integration_test.go` call `tmuxtest.New` before staging the state dir — so isolation and the guard are both registered before the server is started and LIFO runs `kill-server` before the `RemoveAll`.
4. Add a pid-source variant of the teardown guard beside `RegisterStateDirTeardownGuard` (`internal/portaltest/teardown_guard.go:24`) taking the saver pid from a caller-supplied source rather than `state.ReadPIDFile`, and use it at `cmd/bootstrap/composition_e2e_harness_integration_test.go:58` with the live `_portal-saver` pane-pid reader the harness already builds at `:74` — the harness overwrites `daemon.pid` with a pid it SIGKILLs, so the file-based source loses the biggest writer.
5. Widen `TestTeardownGuardCoversEveryServerHostingFixture` (`internal/portaltest/teardown_guard_coverage_test.go:37-87`): trigger on a file that names `PORTAL_STATE_DIR` at all and calls `tmuxtest.New`, and require both `portaltest.IsolateStateForTest` and `portaltest.RegisterStateDirTeardownGuard`, so a hand-rolled state dir fails rather than being invisible. Keep the per-file scoping and its stated rationale.

**Acceptance Criteria**:
- [ ] No fixture in the named class points `PORTAL_STATE_DIR` at a bare `t.TempDir()`.
- [ ] In every retrofitted fixture, `IsolateStateForTest` and `RegisterStateDirTeardownGuard` are both called before `tmuxtest.New`.
- [ ] `IsolateStateForTest` sets `HISTFILE` in both the process env and the returned slice, and no local `HISTFILE` workaround survives in `cmd`.
- [ ] The composite harness's teardown wait observes the live `_portal-saver` pane pid, not the overwritten `daemon.pid`.
- [ ] The widened guard fails on a fixture that sets `PORTAL_STATE_DIR` and starts a server without either required call, and passes over the retrofitted tree.
- [ ] The guard still fatals when it scans zero qualifying files.
- [ ] The full integration lane (`go test -tags integration -p 1 ./...`) passes, and the fingerprint backstop reports no delta against the developer's real state dir.

**Tests**:
- `"it fails a file that pairs PORTAL_STATE_DIR with a tmux server and no teardown guard"`
- `"it fails a file that sets PORTAL_STATE_DIR by hand without IsolateStateForTest"`
- `"it passes a file that makes all three calls"`
- `"it fatals when no file qualifies rather than passing"`
- `"it points HISTFILE at the null device in the returned env"`
- `"it waits out a live saver pid supplied by the caller's source"`
- `"it returns once the state dir stops changing"`

## Task 4: A second slog capture handler lives beside `logtest.Sink`, and this work unit extended it
severity: duplication
sources: duplication, bank

**Problem**: `recordingLogger` (`cmd/bootstrap_production_test.go:14-131`) is a hand-rolled `slog.Handler` reproducing `logtest.Sink` structure for structure — the same `shared`/`bound`/`owner()` indirection, the same `WithAttrs`/`WithGroup` re-binding, the same per-record attr map, its own level-to-string table. `cmd` already has the shared route for exactly this job (`newCaptureLoggerForComponent`, ~75 call sites; `logtest.NewCaptureLogger` beneath it). This work unit did not merely leave the twin in place — it *grew* it: `recordedLog.attrs`, `recordedLog.intAttr` and `onlyMatching` are line-for-line re-implementations of `logtest.Record.Attrs`, `Record.IntAttr` and `Sink.OnlyRecordWith`, and the new sweep suites now capture logs two different ways in the same file (`recordingLogger` for the injected logger, `logtest.Install` for the package-level one — `cmd/run_hook_stale_cleanup_test.go:31,711-718`, `cmd/run_hook_stale_cleanup_lock_timeout_test.go:61`). `internal/logtest` is the declared single source of truth for the capture handler and the rendered-record contract, so a change to the Sink's semantics now silently leaves the sweep suites asserting against a different one. The same class exists one package over: `internal/tmux`'s `recordingSlogHandler` backs `showHooksWarnRecords` (`hooks_register_warn_test.go:13`) and `recordingMigrationLogger` (used at ~10 sites across `hooks_migration_test.go` and `hooks_register_realtmux_test.go`), each with its own re-authored filters.

**Solution**: Delete `recordedLog`, `recordingLogger`, `intAttr`, `countMatching` and `onlyMatching` from `cmd/bootstrap_production_test.go`; replace every `&recordingLogger{}` with `newCaptureLoggerForComponent(t, …)`, `countMatching(...)` with `len(sink.RecordsWith(comp, msg).AtExactLevel(level))`, `onlyMatching(...)` with `sink.OnlyRecordWith(t, …)`, and `rec.intAttr(t, "panes")` with `rec.IntAttr(t, "panes")`. Migrate `internal/tmux`'s two hand-rolled capture types onto `logtest.Sink` in the same pass, so no second handler survives the sweep.

**Outcome**: `internal/logtest` holds the only `slog.Handler` capture implementation in the tree; `cmd` and `internal/tmux` assert against the Sink's rendered-record contract with no second level table, attr map or filter set of their own. Behaviour is unchanged — every migrated assertion covers the same records it covered before.

**Do**:
1. Delete `recordedLog`, `recordingLogger`, `intAttr`, `countMatching` and `onlyMatching` from `cmd/bootstrap_production_test.go:14-131`.
2. Convert the call sites in that file and its dependents: `&recordingLogger{}` → `newCaptureLoggerForComponent(t, "bootstrap"|"daemon")` (`cmd/logging_capture_test.go:35`), `countMatching(entries, level, comp, msg)` → `len(sink.RecordsWith(comp, msg).AtExactLevel(level))`, `onlyMatching(...)` → `sink.OnlyRecordWith(t, comp, msg)`, `rec.intAttr(t, "panes")` → `rec.IntAttr(t, "panes")`.
3. Migrate `internal/tmux`'s `recordingSlogHandler` (`hooks_register_warn_test.go:13`, backing `showHooksWarnRecords`) and `recordingMigrationLogger` (~10 sites across `hooks_migration_test.go` and `hooks_register_realtmux_test.go`) onto `logtest.Sink`, folding each re-authored filter onto `Sink.RecordsWith` / `Sink.Records`.
4. Leave the two capture routes in `cmd/run_hook_stale_cleanup_test.go:31,711-718` and `cmd/run_hook_stale_cleanup_lock_timeout_test.go:61` as two installs — the injected logger and the package-level one are genuinely different subjects — but route both through `logtest`.
5. Grep the tree for a surviving `slog.Handler` implementation outside `internal/logtest` and remove or justify each hit.

**Acceptance Criteria**:
- [ ] `cmd/bootstrap_production_test.go` declares no `slog.Handler` and no record/attr accessor of its own.
- [ ] `internal/tmux` declares no `slog.Handler` of its own; both former capture types are gone with their filters.
- [ ] Every migrated assertion asserts on the same component, message, level and attrs it did before the change.
- [ ] No production file imports `internal/logtest`.
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` pass with no test renamed and no assertion weakened.

**Tests**: no new behaviour, so no new test — the existing suites in `cmd`, `cmd/bootstrap` and `internal/tmux` are the verification and must stay green with their semantics untouched. Specifically these keep asserting exactly what they assert today:
- `cmd/bootstrap_production_test.go`'s bootstrap/daemon count and single-record cases
- `internal/tmux`'s `showHooksWarnRecords` WARN cases and the hooks-migration record cases
- `cmd/run_hook_stale_cleanup_test.go`'s injected-logger and package-logger cases

## Task 5: The generated-id width is a `hooks.json` on-disk contract shaped as a generic generator knob
severity: medium
sources: architecture

**Problem**: `nanoid.width` (`internal/nanoid/nanoid.go:22`) is read by three unrelated id domains — session-name suffixes via `session.GenerateSessionName`, spawn batch/ack ids via `spawn.NewSpawnID` and `burst.go:58`, and pane tokens via `session.NewPaneToken` — **and** by `IsTokenShaped`, which the reaper uses to decide whether a persisted `hooks.json` key may be deleted at all. That makes `width` a persisted-format constant, but its doc comment scopes the coupling narrowly ("read by both the generator and `IsTokenShaped`, so generation and recognition cannot drift apart") and says nothing about the other two consumers or about disk. Change the width for an unrelated reason — longer session-name suffixes to reduce collisions, say — and every six-character token already in `hooks.json` stops being token-shaped: the reaper retains it forever, `checkStaleHooks` counts through the same rule and keeps reporting "no stale hooks", and `portal doctor` stays green while the file accumulates entries no pane answers to. That is the same silence this work unit exists to remove, arriving through a different door. `hookstest.ReapableHookKey` panics on a width change, so the *fixture vocabulary* fails loudly — but nothing in the suite exercises a persisted key authored under the old width, so once the fixture is repaired the data-classification change is invisible.

**Solution**: Make the on-disk contract explicit and hard to break by accident.

**The pane token gets its own named width.** Session-name suffixes, spawn ids and pane tokens are unrelated concerns that happen to share a value today, and only one of them is a persisted-format constant. Declare a pane-token width beside `IsTokenShaped`, read by both the pane-token mint and the predicate, so generation and recognition still cannot drift while session-name and spawn-id widths move independently. Documenting the coupling was the alternative and was rejected: it leaves the landmine in place with a sign on it.

Add the data-level test regardless — pin a persisted key authored under the old width against the classifier, so a width change fails on the data rather than only on `hookstest.ReapableHookKey`'s panic.

**Outcome**: The pane token's width is a named constant read by both its mint and `IsTokenShaped`, documented as part of `hooks.json`'s key-recognition contract; the generic generator width is free to move without reclassifying a single persisted key; and a change to the token width fails on a literal persisted key rather than passing silently.

**Do**:
1. Declare a pane-token width constant in `internal/nanoid` beside `IsTokenShaped` (`internal/nanoid/nanoid.go:42-57`), documented as an on-disk contract: changing it reclassifies every key already in `hooks.json`, so it is a migration event rather than a tuning change. Have `IsTokenShaped` read it.
2. Point the pane-token mint at that width — a generator constructed at the pane-token width rather than the package `width` — so generation and recognition still cannot drift.
3. Leave `width` (`:19-22`) serving session-name suffixes and spawn ids, and correct its doc comment so it no longer claims a recognition coupling it no longer carries.
4. Add a data-level test in `internal/hooks` (or beside the reaper) that seeds a `hooks.json` under a **literal** six-character key — not one derived from `nanoid.Alphabet` — and asserts both that `IsTokenShaped` judges it and that the reaper deletes it when it is absent from the live set. A width change then fails on the persisted data, not only on `hookstest.ReapableHookKey`'s panic.

**Acceptance Criteria**:
- [ ] The pane-token width is a separate named constant from the generic generator width, and `IsTokenShaped` reads the pane-token one.
- [ ] The pane-token mint and `IsTokenShaped` read the same constant, so a token the mint produces is always token-shaped.
- [ ] Changing the generic `width` alone leaves every existing `hooks.json` key token-shaped and the reaper's behaviour unchanged.
- [ ] Changing the pane-token width alone fails the new literal-key test.
- [ ] The pane-token width's doc names the `hooks.json` consequence; `width`'s doc no longer claims the recognition coupling.
- [ ] Session-name suffixes and spawn/ack ids are unchanged in width and charset.

**Tests**:
- `"it judges a literal six-character persisted key token-shaped"`
- `"it reaps a persisted key authored at the pane-token width when the pane is gone"`
- `"it recognises every token the pane-token mint produces"`
- `"it rejects a key one byte short of the pane-token width"`
- `"it rejects a key carrying a character outside the alphabet"`
- `"it leaves session-name suffix width independent of the pane-token width"`

## Task 6: `CleanStale`'s enumeration callback carries its decline reason out-of-band
severity: medium
sources: architecture

**Problem**: `Store.CleanStale(enumerateLive func(Snapshot) ([]string, error))` (`internal/hooks/store.go:302`) documents that a returned error "is returned unwrapped, so a caller can carry its own reasons through" — an invitation to put the reason *in* the error. The only caller does not: `runHookStaleCleanup` returns the bare sentinel `errCycleDeclined` from the closure (`cmd/run_hook_stale_cleanup.go:165,173`) while writing the actual reason, level and attrs into a `decline standDown` variable captured from the enclosing scope, which `declinedSweep` (`:197`) re-joins afterwards. Sentinel and payload are two independent channels kept in step by hand. A second decline path added inside that closure that returns `errCycleDeclined` without assigning `decline` yields the zero `standDown`: `emit()` writes `hooks: clean-stale-skipped reason=` at INFO with an empty reason, and `sweepOutcome{DeclineReason: ""}` is indistinguishable from a cycle that ran and found nothing — so `portal doctor --fix` prints no skip line and the daemon reports nothing. Correctness rests on caller discipline at a package boundary, which is the property §4 and §5 removed everywhere else.

**Solution**: Fold the reason into the error so the two cannot separate — a small typed error in `cmd` wrapping the `standDown` (`type declinedError struct{ standDown }`), returned from the closure and recovered in `declinedSweep` with `errors.As`. `errCycleDeclined` and the captured `decline` both disappear, and `CleanStale`'s documented "carry your own reasons through" contract becomes the mechanism actually used.

**Outcome**: A decline path that forgets to name its reason no longer compiles into a silent empty-reason line.

**Do**:
1. Declare `type declinedError struct{ standDown }` in `cmd/run_hook_stale_cleanup.go` with an `Error()` naming the reason, so a decline cannot be constructed without one.
2. Return `declinedError{view.Decline}` from the `CleanStale` closure at `:171-174` in place of `errCycleDeclined`, and delete the captured `decline standDown` at `:165`.
3. Recover it in `declinedSweep` (`:197-221`) with `errors.As`, replacing the `errors.Is(err, errCycleDeclined)` branch, and emit from the recovered value.
4. Delete the `errCycleDeclined` sentinel at `:149`, leaving `errNothingPersisted` as the only bare sentinel on the path.

**Acceptance Criteria**:
- [ ] `errCycleDeclined` and the captured `decline` variable are both gone from the file.
- [ ] A declined cycle still emits exactly one `clean-stale-skipped` line at the reason's own level with its own attrs.
- [ ] `sweepOutcome.DeclineReason` still carries the reason for every decline path, and remains empty for a cycle that ran.
- [ ] The `errNothingPersisted`, `ErrLockHeld` and `ErrSnapshotRead` branches are unchanged in behaviour.
- [ ] A decline carrying an empty reason is not constructible from the closure — the reason travels with the error rather than beside it.

**Tests**:
- `"it carries the decline reason inside the error the closure returns"`
- `"it emits one clean-stale-skipped line naming the restore reason"`
- `"it emits one clean-stale-skipped line naming the empty-pane-read reason"`
- `"it reports DeclineReason on the outcome for every decline path"`
- `"it leaves DeclineReason empty for a cycle that ran and removed nothing"`
- `"it still returns nothing-persisted without a stand-down line"`

## Task 7: Bare `-t` targets are composed outside the tmux client, with nothing enforcing the rule
severity: medium
sources: bank

**Problem**: The exactness rule now holds inside `internal/tmux`, but two production sites compose their own targets outside it and bypass it entirely. `internal/session/quickstart.go:51-52` builds a chained tmux argv carrying `set-option -t <name>` and `attach-session -t <name>`, both unprefixed — the name is generated-unique and created in the same chain, but tmux continues a `;` chain past a failed `new-session`, so a failed create lets the stamp land on a prefix sibling. `internal/restore/session.go:85-104` builds `target := fmt.Sprintf("%s:", sess.Name)` and hands it to `SplitWindow`/`NewWindow` — the exact `<session>:` shape whose unpinned form resolves to a prefix sibling. Beyond those two, nothing enumerates `-t` composition sites at all: the class has now been rediscovered **three** times (the original seven client sites, the two `saver_pane_pid.go` sites fixed with the wrong helper, and `SelectLayout` caught in review), which is the signature of an invariant resting on author discipline.

**Solution**: Pin both call sites through the client's exactness vocabulary, and add a source guard that enumerates `-t` argv composition across `internal/tmux`, `internal/session` and `internal/restore` and requires each to route through `exactTarget` / `exactCoordTarget` / `windowTargetExact` / `PaneTargetExact`. The guard is what turns a rediscovered class into a caught one; `internal/sourceguardtest` already owns the primitives it needs.

**Outcome**: No production `-t` target is composed by hand outside the client's exactness vocabulary in the three packages that compose them, and a source guard fails the build the next time one is.

**Do**:
1. Measure each of the two sites' target kind before choosing a helper — `exactTarget`'s doc (`internal/tmux/tmux.go:411-424`) records that `set-option` resolves a *pane* target, so `=foo` fails there even for a live session, while `attach-session` takes a session target. Do not pick by symmetry.
2. Route `internal/session/quickstart.go:51-52`'s `set-option -t <name>` and `attach-session -t <name>` through the measured helpers, exporting the two the package needs from `internal/tmux` (or moving the argv composition into the client) rather than re-implementing the `=` prefix in `internal/session`.
3. Route `internal/restore/session.go:85-104`'s `target := fmt.Sprintf("%s:", sess.Name)` — fed to `SplitWindow`/`NewWindow` — through the same vocabulary, keeping the trailing-colon "session's active window" semantics the comment at `:85-86` relies on.
4. Add a source guard over `internal/tmux`, `internal/session` and `internal/restore` built on `sourceguardtest.PackageGoFiles` + `ForEachFuncCall`: every argv element literal `"-t"` must be followed by a call to one of `exactTarget` / `exactCoordTarget` / `windowTargetExact` / `PaneTargetExact` (or a variable assigned from one), and the guard fatals if it scans zero files.

**Acceptance Criteria**:
- [ ] `internal/session/quickstart.go` composes no unprefixed `-t` target; a `new-session` that fails no longer lets the chained `set-option` land on a prefix sibling.
- [ ] `internal/restore/session.go` composes no unprefixed `<session>:` target; splits and new windows still land in the restored session's active window.
- [ ] The source guard passes over the three packages after the two fixes and fails when a bare `-t` composition is reintroduced in any of them.
- [ ] The guard fatals rather than passing when it enumerates no files.
- [ ] Quickstart still stamps `@portal-dir` before attaching, and restore still reconstructs multi-window/multi-pane skeletons identically.

**Tests**:
- `"it pins the quickstart set-option target to the exact session"`
- `"it pins the quickstart attach-session target to the exact session"`
- `"it does not stamp a prefix sibling when new-session fails mid-chain"`
- `"it pins the restore split and new-window targets to the exact session"`
- `"it restores a multi-window session with a prefix sibling live"`
- `"it fails a package composing a bare -t target"`
- `"it fatals when the guard enumerates no files"`

## Task 8: The hook-fire assertion in `cmd/bootstrap` is the unpolled read already fixed one package over
severity: medium
sources: bank

**Problem**: `verifyHookFiredOnce` (`cmd/bootstrap/reboot_roundtrip_test.go:437-448`) does a bare `os.ReadFile` + `strings.Count("HOOK_FIRED")` with a `t.Fatalf` on ENOENT, called at `:164` right after the same `WaitForSkeletonMarkersCleared` at `:158` — the exact shape and exact race that was removed from `internal/restore` (only the intervening `verifyANSIScrollback` narrows the window). The markers clear when the helper reaches its exec step, *before* the hook's `echo >>` completes. `internal/restore` now has the polled `assertMarkerCount` (`marker_assert_test.go:30`) bounded by `hydrateBudget`/`hydrateTick`, and both packages already import `internal/restoretest`. Four pieces of residue ride the same edit: `internal/restore/multipane_legacy_integration_test.go:139,209` still pass raw `10*time.Second, 50*time.Millisecond` where the rename fixture passes the named constants; `hydrateBudget`/`hydrateTick` serve both families but still live in `rename_reboot_shared_test.go` rather than beside the assertion; `internal/restore/marker_assert_meta_test.go:57` panics inside a writer goroutine, which would abort the whole integration binary rather than fail one test; and `cmd/noncontiguous_window_reboot_integration_test.go` carries its own `divergentHydrateBudget`/`divergentPollTick` pair.

**Solution**: Promote the polled marker assertion and its budget/tick pair into `internal/restoretest`, route `verifyHookFiredOnce` and the two literal call sites and the divergent pair through it, and replace the writer-goroutine panic with a plain error drop that fails via the helper's deadline message.

**Outcome**: One polled marker-count assertion serves both `internal/restore` and `cmd/bootstrap`; no hook-fire read races the helper's append; no literal or divergent budget/tick pair survives; and the meta test's writer goroutine reports its failure instead of aborting the integration binary.

**Do**:
1. Move `assertMarkerCount` / `assertMarkerCountOn` / `waitForMarkerCount` / `readMarkerCount` (`internal/restore/marker_assert_test.go:30-86`) into `internal/restoretest` as an exported polled assertion, taking the budget and tick from the promoted `hydrateBudget` / `hydrateTick` pair moved out of `rename_reboot_shared_test.go` and homed beside it. Keep the want-of-0 semantics (wait out the whole budget) and the three distinct failure messages.
2. Replace `verifyHookFiredOnce` (`cmd/bootstrap/reboot_roundtrip_test.go:437-448`, called at `:164`) with the promoted assertion, deleting the bare `os.ReadFile` + `strings.Count` + ENOENT `t.Fatalf`.
3. Route `internal/restore/multipane_legacy_integration_test.go:139,209`'s raw `10*time.Second, 50*time.Millisecond` and `cmd/noncontiguous_window_reboot_integration_test.go`'s `divergentHydrateBudget`/`divergentPollTick` through the promoted pair, deleting both local declarations.
4. Replace the `panic` inside the writer goroutine at `internal/restore/marker_assert_meta_test.go:57` with a recorded error the test body reports, so a failure fails one test rather than aborting the binary.

**Acceptance Criteria**:
- [ ] `cmd/bootstrap`'s hook-fire assertion polls to the shared budget and no longer reads once immediately after `WaitForSkeletonMarkersCleared`.
- [ ] `hydrateBudget` and `hydrateTick` are declared once, in `internal/restoretest`, and every consumer reads them.
- [ ] No `10*time.Second` / `50*time.Millisecond` literal and no `divergent*` duplicate remains at the four named sites.
- [ ] An absent marker file still reads as a count of 0 rather than fatalling.
- [ ] The meta test's writer goroutine cannot panic the process.
- [ ] The integration lane passes and the hook-fire assertion still detects a hook that fires twice (cross-fire) and one that never fires.

**Tests**:
- `"it waits for a hook marker that appears after the skeleton markers clear"`
- `"it fails when the marker never appears within the budget"`
- `"it fails immediately when the marker fires more times than wanted"`
- `"it waits out the full budget for a want of zero"`
- `"it treats an absent marker file as a count of zero"`
- `"it reports a writer failure without panicking the binary"`

## Task 9: Integration-lane timing budgets fail on a normally-loaded developer machine
severity: medium
sources: bank

**Problem**: Three independent agents observed the same class. **(1)** Six `cmd/bootstrap` constants declare a 6s pgrep-convergence budget (`composition_abc_integration_test.go:19`, `composition_e2e_convergence_integration_test.go:16`, `composition_e2e_f_observables_integration_test.go:19`, `composition_e2e_fresh_acquire_integration_test.go:16`, `composition_e2e_self_eject_integration_test.go:25`, `upgrade_path_integration_test.go:19`); under ~18 load on 10 cores the observed elapsed clusters at 6.05–6.12s — just over budget — and which member trips rotates between runs. Confirmed on a stashed clean tree, so unrelated to any recent change. **(2)** `cmd/state_daemon_integration_test.go:42` seeds `scrollbackLines = 500000` against a fixed 10s budget and fails 3/3 standalone on a loaded box, reproducing identically with the file stashed back to HEAD. Because there is no CI, every run is on a machine carrying the developer's real workload, so a budget with ~2% headroom manufactures false failures and erodes trust in the lane — which then misattributes real regressions.

**Solution**: Replace the fixed budgets with something that survives contention, across all seven sites in one pass so the constants cannot diverge again.

**The wait becomes contention-tolerant.** Poll to a deadline that extends while progress is observable — the pgrep count falling, scrollback lines accumulating — so the assertion measures convergence rather than wall-clock throughput. Widening the budgets was the alternative and was rejected: it moves the cliff rather than removing it, fails again on a busier machine, and makes a genuine hang take proportionally longer to surface. There is no CI, so every run competes with whatever else the developer is doing; a fixed budget cannot be picked correctly under an unbounded ambient load.

**Outcome**: All seven timing assertions measure convergence rather than wall-clock throughput — a loaded machine makes them slower, not red — while a process that stops making progress still fails inside a bounded ceiling.

**Do**:
1. Add a progress-tolerant wait beside `tmuxtest.PollUntil` (the one helper both `cmd` and `cmd/bootstrap` already import): it polls an observation function, extends its deadline whenever the observation changes, and fails at an absolute ceiling so a hang cannot wait forever. Return the last observation for the caller's failure message.
2. Route `waitForPgrepCount` (`cmd/bootstrap/orphan_sweep_integration_test.go:251-260`) through it, observing the pgrep count so a count that is still falling extends the wait.
3. Delete the six per-file 6s constants and point their call sites at the shared wait: `composition_abc_integration_test.go:19`, `composition_e2e_convergence_integration_test.go:16`, `composition_e2e_f_observables_integration_test.go:19`, `composition_e2e_fresh_acquire_integration_test.go:16`, `composition_e2e_self_eject_integration_test.go:25`, `upgrade_path_integration_test.go:19`.
4. Convert `cmd/state_daemon_integration_test.go`'s fixed 10s scrollback budget (`:42`, `scrollbackLines = 500000`) to the same shape, observing lines accumulated so a slow host extends rather than fails; keep the existing sub-2s aggregate skip at `:93-96`.
5. Rename any assertion whose name pins a wall-clock figure that no longer applies (`TestCompositeBootstrap_ConvergesPgrepToOneWithin6s`).

**Acceptance Criteria**:
- [ ] No `6 * time.Second` convergence constant survives in `cmd/bootstrap`.
- [ ] Every one of the seven sites reaches its assertion through the shared progress-tolerant wait.
- [ ] The wait extends its deadline on an observed change and fails at a stated absolute ceiling.
- [ ] A process making no progress fails within the ceiling, with the last observation in the message.
- [ ] The seven tests pass on a machine under load comparable to the measured ~18 load average on 10 cores.
- [ ] No assertion's subject changes — the same end states are still asserted, only the waiting differs.

**Tests**:
- `"it returns as soon as the observation reaches the target"`
- `"it extends the deadline while the observation keeps changing"`
- `"it fails at the absolute ceiling when the observation stops changing"`
- `"it reports the last observation in its failure message"`
- `"it converges pgrep to one daemon under sustained load"`
- `"it measures a slow scrollback tick without failing on wall-clock alone"`

## Task 10: Integration-tagged source carries untouched lint debt, and the issue cap hides findings
severity: drift
sources: bank

**Problem**: `golangci-lint run ./...` analyses only the default build tags, so every file behind `//go:build integration` has never been in scope. Verified now: the unit lane reports **0 issues**, while `golangci-lint run --build-tags integration --max-same-issues 0 --max-issues-per-linter 0 ./...` reports **21** — 14 `rangeint`, 4 `stringsseq`, 1 `stringscutprefix`, 1 `stringscut`, 1 `minmax` — concentrated in `internal/restore/integration_full_test.go` (10× `rangeint`) with the rest across `cmd/bootstrap` and `cmd`. Same mechanical, autofixable class as the 77 the repo-wide sweep already cleared. Separately, the default `max-same-issues` cap is what made that sweep's finding count read as 30 instead of 77, and it will keep hiding real findings from every future run.

**Solution**: Clear the 21 integration-tagged findings and settle how the linter is invoked so the debt cannot re-accumulate invisibly.

**`.golangci.yml` analyses the integration tag by default**, so one command covers both lanes and there is nothing to remember. A documented second invocation was the alternative and was rejected: with no CI, a lint step that depends on someone recalling a flag is a lint step that stops running.

Pin `max-same-issues: 0` in the same change regardless of the tag question — the default cap is what made task 6-25's finding count read as 30 when it was 77, and it will keep hiding real findings whichever invocation runs.

**Outcome**: A bare `golangci-lint run` analyses both lanes with no capped output and reports 0 issues across the tree, so integration-tagged lint debt cannot re-accumulate out of sight.

**Do**:
1. Add the integration build tag to `.golangci.yml` so a bare `golangci-lint run` covers both lanes, and pin `max-same-issues: 0` and `max-issues-per-linter: 0` alongside it. Keep the existing `testmain_isolation_test.go` errcheck exclusion.
2. Clear the 21 integration-tagged findings — 14 `rangeint`, 4 `stringsseq`, 1 `stringscutprefix`, 1 `stringscut`, 1 `minmax` — concentrated in `internal/restore/integration_full_test.go` (10× `rangeint`) with the rest across `cmd/bootstrap` and `cmd`. `golangci-lint run --fix` autofixes the class; read each hunk rather than trusting the rewrite wholesale.
3. Update CLAUDE.md's lint sentence (`Lint via golangci-lint run …`) to state that the config analyses the integration tag, so no second invocation has to be remembered.

**Acceptance Criteria**:
- [ ] `golangci-lint run ./...` reports 0 issues with no flags.
- [ ] The config sets the integration build tag and both issue caps to 0.
- [ ] The 21 findings are fixed, not excluded or nolint-suppressed.
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass after the rewrites — the fixed files are test files, and a `rangeint` rewrite changes loop variable semantics.
- [ ] CLAUDE.md's lint line matches how the config now behaves.

**Tests**: mechanical rewrites with no behaviour change, so no new test — verification is `golangci-lint run` reporting 0 and both lanes staying green. In particular `internal/restore/integration_full_test.go`'s restore round-trip assertions and the `cmd/bootstrap` composite suites must pass unchanged after their loop rewrites.

## Task 11: CLAUDE.md drift left by this work unit
severity: drift
sources: bank

**Problem**: Four claims in CLAUDE.md no longer match the tree, all of them in the passages this work unit changed. **(1)** The `logtest` row (`:82`) still enumerates the old accessor set (`AttrString` / `IntAttr` / `RequireDuration` / `HasAttr` / `OnlyRecord`) and says consumption is "via thin embedded-field wrappers across cmd / state / restore / tui / store test surfaces" — the `cmd`, `cmd/bootstrap` and `internal/state` wrappers are gone, and the package now also owns `Install`, `RecordWant`/`AssertRecord` and the `Records` filter chain. **(2)** The Multi-window spawn bullet (`:213`) claims "the nanoid alphabet … [is] single-sourced in `internal/spawn`"; the alphabet has never lived there and now lives in `internal/nanoid`, so it is the one place in the file pointing an agent at the wrong home for the id vocabulary — directly contradicting the new `nanoid` row three sections above. **(3)** The word "snapshot" appears nowhere in the file outside the unrelated `portaltest` row, so `CleanStale`'s snapshot-before-enumeration narrowing — the machinery that keeps a `hook set` landing in the enumeration gap from being reaped — is entirely undocumented; the hooks row's new sentence is accurate but silent on it. **(4)** Nothing records the `hooks.json.lock` sidecar being absent on every install in the wild until the next release takes a mutation lock, nor that `migrateConfigFile` moves `hooks.json` without its sidecar — so the degraded read is the common path after the next release rather than an edge case.

**Solution**: Correct the `logtest` row against the package's current surface, delete the alphabet claim from the spawn bullet, and add the snapshot-narrowing invariant plus the sidecar-absence note to the hooks row and the Resume hooks section.

**Outcome**: The four claims match the tree: the `logtest` row names the package's actual surface and consumption route, no line points an agent at `internal/spawn` for the id vocabulary, the snapshot narrowing that protects a `hook set` landing in the enumeration gap is documented, and the sidecar's absence on every existing install is recorded as the common post-release path.

**Do**:
1. Rewrite the `logtest` row (`CLAUDE.md:82`): drop the thin-embedded-field-wrappers claim (the `cmd`, `cmd/bootstrap` and `internal/state` wrappers are gone), and name the current surface — `Install`, `RecordWant`/`AssertRecord`, the `Records` filter chain and the `Sink.Records*` / `OnlyRecordWith` accessors alongside the record accessors.
2. Delete "the nanoid alphabet" from the single-sourced-in-`internal/spawn` list in the Multi-window spawn bullet (`:213`); the alphabet lives in `internal/nanoid`, which the row three sections above already documents.
3. Add the snapshot-before-enumeration narrowing to the `hooks` row and the Resume hooks section: the call-site snapshot is taken before the pane enumeration and may only narrow the delete set, so a `hook set` landing in the enumeration gap cannot be reaped.
4. Record in the same passage that `hooks.json.lock` is absent on every install in the wild until that install's first mutation after the next release, and that `migrateConfigFile` moves `hooks.json` without its sidecar — so the degraded unlocked read is the common path immediately after release rather than an edge case.

**Acceptance Criteria**:
- [ ] The `logtest` row's accessor list and consumption sentence match `internal/logtest`'s exported surface.
- [ ] No line in CLAUDE.md attributes the nanoid alphabet to `internal/spawn`.
- [ ] The snapshot narrowing appears in the file with its invariant stated (narrow only, never widen).
- [ ] The sidecar-absence window and the migration's sidecar-less move are both recorded.
- [ ] No claim is added that the tree does not support — each of the four edits is checkable against a named file.

**Tests**: documentation only, so no test. Verification is a read-back of each edited passage against the file it describes: `internal/logtest/capture.go` and `install.go` for the row, `internal/nanoid/nanoid.go` for the alphabet, `internal/hooks/store.go`'s `CleanStale` for the narrowing, and `internal/hooks/lock.go` plus `cmd/config.go`'s `migrateConfigFile` for the sidecar note.

## Task 12: The sweep's stand-down reporting reports one condition twice and takes a logger that governs half its output
severity: low
sources: standards, architecture

**Problem**: Two defects in the same reporting path. **(1)** `declinedSweep` (`cmd/run_hook_stale_cleanup.go:203-219`) deliberately returns nil for the lock-timeout branch, with the stated reason that "the nil error keeps the caller from adding a second report for the same event" — then does the opposite for `ErrSnapshotRead`: it emits the `clean-stale-skipped reason=store-read-failed` WARN **and** returns the error, so the daemon adds `hooks stale-cleanup failed` and `doctor --fix` adds `doctor --fix: stale-hook prune failed` for the same event. Two WARN lines for one condition, inside a work unit whose stated purpose is that a single grep answers why the prune declined; the project's log-or-return convention is waived by the spec only for `hook set` / `hook rm`, where the two lines serve different audiences. **(2)** `runHookStaleCleanup(reader, store, logger *slog.Logger)` (`:153`) takes a logger, defaults it to `bootstrapLogger` when nil, and uses it for exactly two DEBUG count lines (`:169`, `:189`). Every stand-down line — the ones answering the question the sweep's observability exists for — is written by `standDown.emit()` to the package-global `hooksLogger`, bypassing the parameter. A caller injecting a logger to observe the sweep captures the counts and none of the stand-downs, which is why the suite additionally installs a global handler.

**Solution**: Apply the lock-timeout branch's own reasoning to the store-read branch so the event is reported once, and either rename the parameter for what it governs with a doc comment stating that stand-downs always carry the `hooks` component, or drop it and bind the counts to their own component directly.

**Outcome**: An unreadable `hooks.json` produces one WARN naming the stand-down, on both the daemon and the `doctor --fix` route; and `runHookStaleCleanup`'s logger parameter either names what it governs or is gone.

**Do**:
1. Change the `hooks.ErrSnapshotRead` branch of `declinedSweep` (`cmd/run_hook_stale_cleanup.go:212-218`) to return nil like the `ErrLockHeld` branch above it, keeping the `clean-stale-skipped reason=store-read-failed` WARN and the `DeclineReason` on the outcome.
2. Confirm the two callers no longer double-report: `cmd/state_daemon.go:211-213`'s `hooks stale-cleanup failed` and `cmd/doctor.go:201-204`'s `doctor --fix: stale-hook prune failed` must not fire for this condition, while `pruneDoctorStaleHooks` still prints its `Skipped stale hook prune: …` line from the reason.
3. Settle the logger parameter at `:153`: either rename it to name the two DEBUG count lines it governs (`:169`, `:189`) with a doc comment stating stand-downs always emit under the `hooks` component through `standDown.emit()`, or drop it entirely and bind the counts to their own component. Update the two call sites either way.
4. If the parameter is dropped, remove any test scaffolding that exists only to inject it, keeping the global `logtest.Install` capture the stand-down assertions need.

**Acceptance Criteria**:
- [ ] An unreadable `hooks.json` yields exactly one WARN for the event across the whole call chain, on both the daemon and `doctor --fix` paths.
- [ ] `doctor --fix` still prints the `Skipped stale hook prune:` line for a store-read stand-down, and its exit code stays driven solely by the post-repair diagnosis.
- [ ] The daemon logs no `hooks stale-cleanup failed` line for a store-read stand-down.
- [ ] `runHookStaleCleanup`'s signature either has no logger parameter or has one whose name and doc match the two DEBUG lines it governs.
- [ ] The lock-timeout, restore, empty-pane-read and pane-read-failed branches are unchanged.

**Tests**:
- `"it emits one WARN for an unreadable hooks.json on the daemon path"`
- `"it emits one WARN for an unreadable hooks.json on the doctor --fix path"`
- `"it still prints the skipped-prune line for a store-read stand-down"`
- `"it leaves the doctor exit code to the post-repair diagnosis"`
- `"it emits the count lines under the component the signature names"`
- `"it still reports a lock-timeout stand-down exactly once"`

## Task 13: The doctor renderers are unbound to the reason constants and neither is exhaustive
severity: low
sources: architecture, duplication

**Problem**: `skippedPrunePhrases` (`cmd/doctor.go:215`, five entries) and `notEvaluableDetails` (`:308`, four) both key off the same `skipReason*` const block declared in another file, render it in two registers, and both fall through to the raw slug when a reason is unmapped. Nothing binds either map to the const block, so adding a sixth reason compiles and ships and the user sees the internal slug (`store-read-failed`) in `portal doctor` output — on the command whose whole purpose is telling the user what happened. The key sets have **already** diverged (`skipReasonLockTimeout` is in one and absent from the other) with nothing to notice it. Separately, the two lookup functions `skippedPrunePhrase` and `notEvaluableDetail` have byte-identical bodies and repeat the same "an unmapped reason falls through to its raw value" promise twice; the wordings genuinely differ per surface so the maps must stay two, but the lookup and its fallback rule need not.

**Solution**: Home the phrases with the reasons (move both maps beside the const block, or give the reason a small type), collapse the two lookups into one `phraseFor(m map[string]string, reason string) string`, and add a table-driven guard asserting every declared reason has an entry in each renderer. The fall-through stays as the runtime safety net; the omission moves from a cosmetic production defect to a test failure.

**Outcome**: Adding a sixth stand-down reason fails a test until both renderers carry a phrase for it, and no user-facing surface can ship an internal slug through a silent omission.

**Do**:
1. Declare the reasons as an enumerable set beside the `skipReason*` const block (`cmd/run_hook_stale_cleanup.go:15-21`) — a slice of the declared values, or a small named type with an `All()` — so a guard can range over them rather than restating them.
2. Move `skippedPrunePhrases` (`cmd/doctor.go:215-221`) and `notEvaluableDetails` (`:308-313`) beside that block so the reason and its two renderings are read together. Keep them two maps: the wordings genuinely differ per surface.
3. Replace `skippedPrunePhrase` (`:226-231`) and `notEvaluableDetail` (`:315-320`) with one `phraseFor(m map[string]string, reason string) string`, keeping the raw-value fall-through and stating the promise once.
4. Add the table-driven guard: for every declared reason, assert both maps hold a non-empty entry, and assert neither map holds a key that is not a declared reason.

**Acceptance Criteria**:
- [ ] Every declared `skipReason*` value has a non-empty entry in both maps, including `lock-timeout` in `notEvaluableDetails` (which is absent today).
- [ ] Neither map holds a key outside the declared set.
- [ ] The two lookup functions are replaced by one, with the fall-through preserved as the runtime net.
- [ ] Adding a reason without a phrase fails the guard rather than compiling.
- [ ] Every rendered phrase is byte-identical to what it renders today except where Task 34 deliberately changes it.

**Tests**:
- `"it renders a phrase for every declared stand-down reason"`
- `"it renders a not-evaluable detail for every declared stand-down reason"`
- `"it holds no phrase for a reason that is not declared"`
- `"it falls through to the raw reason for an unmapped value"`
- `"it fails when a newly declared reason has no phrase"`

## Task 14: `logtest.Record` has no error accessor, so nine sites hand-roll it
severity: duplication
sources: duplication

**Problem**: Nine sites across five files write the same eight-line block to get an error out of a record — index `rec.Attrs["error"]`, fatal on absent, type-assert `.Any().(error)`, fatal on the wrong kind — each with its own failure wording, and six of the nine then follow with the identical `errors.Is(loggedErr, fileutil.ErrWriteTempCreate)` check: `internal/hooks/lock_write_test.go:39-51`, `internal/hooks/store_test.go:1182,1358,1514`, `internal/project/store_logging_test.go:124,245,350,544`, `internal/storelog/clean_stale_test.go:75-85`. All five files were rewritten by the task whose stated purpose was that `logtest` owns the sink, the filters and the record assertion. The same files also hand-roll `if _, ok := rec.Attrs["took"]; !ok` at six sites (`internal/hooks/store_test.go:1024,1179`; `internal/project/store_logging_test.go:450,541`; `internal/storelog/clean_stale_test.go:32,68`) although `Record.RequireDuration` already exists and is used for exactly this in `cmd/bootstrap` and `internal/state`. `Record` carries `AttrString`, `IntAttr`, `RequireDuration` and `HasAttr` — the error accessor is the one member of the family never added, so every caller re-derives it.

**Solution**: Add `func (r Record) ErrorAttr(t TestingT, key string) error` to `internal/logtest/capture.go` beside `IntAttr`, fatal on absent and on a non-error value, and route all nine sites through it. Replace the six hand-rolled `took` presence checks with the existing `RequireDuration`.

**Outcome**: `Record` carries the error accessor its family was missing, and no suite hand-rolls getting an error or asserting a duration out of a captured record. Behaviour and coverage are unchanged.

**Do**:
1. Add `func (r Record) ErrorAttr(t TestingT, key string) error` to `internal/logtest/capture.go` beside `IntAttr` (`:49-60`): fatal on an absent attr and on a value whose `Any()` is not an `error`, with the same failure register as its siblings.
2. Route the nine hand-rolled blocks through it: `internal/hooks/lock_write_test.go:39-51`, `internal/hooks/store_test.go:1182,1358,1514`, `internal/project/store_logging_test.go:124,245,350,544`, `internal/storelog/clean_stale_test.go:75-85`. Keep each site's following `errors.Is(loggedErr, …)` assertion exactly as it stands.
3. Replace the six `if _, ok := rec.Attrs["took"]; !ok` checks with `rec.RequireDuration(t, "took")`: `internal/hooks/store_test.go:1024,1179`, `internal/project/store_logging_test.go:450,541`, `internal/storelog/clean_stale_test.go:32,68`. Note this strengthens the assertion from presence to kind, which is the accessor's documented point.
4. Cover the accessor's own failure paths through `logtest.TestingT` so its fatals are unit-testable without aborting the harness, matching how the sibling accessors are covered.

**Acceptance Criteria**:
- [ ] `ErrorAttr` fatals on an absent attr and on a non-error value, and returns the error otherwise.
- [ ] None of the nine sites indexes `rec.Attrs["error"]` or type-asserts `.Any().(error)` directly.
- [ ] All six `took` presence checks read through `RequireDuration`, and each still passes.
- [ ] Every `errors.Is` assertion downstream of the nine sites is unchanged.
- [ ] `go test ./...` passes with no test renamed.

**Tests**:
- `"it returns the error carried by the named attr"`
- `"it fails when the record carries no attr of that name"`
- `"it fails when the attr value is not an error"`
- `"it reports the record's attrs in its failure message"`

Plus the existing suites in `internal/hooks`, `internal/project` and `internal/storelog` stay green with their subjects untouched.

## Task 15: `logtest`'s filter surface advertises an unused idiom and lacks the one three packages re-author
severity: duplication
sources: architecture, duplication, bank

**Problem**: Three related gaps in one exported surface. **(1)** The `Records` named slice type exports three chainable filters (`AtExactLevel`, `AtOrAboveLevel`, `With` — `capture.go:82-96`) alongside three `Sink` forwarders that call them; across the whole repository the chained form is used **zero** times outside the package, so half the surface is a second way to do what the other half does, kept alive only by its own forwarders. **(2)** The filter three packages actually need does not exist: a *message-only* record filter is re-authored as `themeEventRecords` (`internal/tui/theme_panel_commit_load_test.go:110`, consumed from four files), `recordsNamed` (`internal/theme/events_test.go`) and `captureSink.recordsWithMessage` (`internal/restore/logging_capture_test.go:20`, which also carries a `capturedRecord` projection and a `newCaptureLogger` wrapper existing only to host it) — `Records.With` requires a component, so none of them can route through the package. **(3)** `markerReporter` (`internal/restore/marker_assert_test.go:14-18`) is a fresh declaration of the narrowed-`*testing.T` seam `logtest.TestingT` already declares with the identical method set, and `recordingReporter` + its `fatalSentinel` panic-and-recover trick (`marker_assert_meta_test.go:16-45`) is a second copy of `restoretest`'s `fakeFataller` pattern.

**Solution**: Unexport the three chainable filters (keeping `Records` as the return type, so every existing caller is untouched), add `Sink.RecordsWithMessage(msg)`, and route the three re-authored message filters through it — which lets `internal/restore` drop its wrapper type entirely. Point `markerReporter` at `logtest.TestingT` and reuse `restoretest`'s recording-fataller rather than redeclaring it.

**Outcome**: `logtest`'s exported filter surface is exactly one route per query, it carries the message-only filter three packages currently re-author, and no package redeclares the narrowed-`*testing.T` seam or the recording-fataller pattern.

**Do**:
1. Unexport `AtExactLevel`, `AtOrAboveLevel` and `With` on `Records` (`internal/logtest/capture.go:82-101`), keeping `Records` as the return type and the three `Sink` forwarders (`:194-207`) identical for every existing caller.
2. Before unexporting, sweep for external chained callers. Task 4 converts `countMatching` to `sink.RecordsWith(comp, msg).AtExactLevel(level)`; if that task has landed, add the `Sink` forwarder that composes component, message and level in one call and route it there rather than leaving a filter exported — the settled direction is one route per query, not a filter kept alive for one caller.
3. Add `Sink.RecordsWithMessage(msg)` and route the three re-authored message filters through it: `themeEventRecords` (`internal/tui/theme_panel_commit_load_test.go:110`, consumed from four files), `recordsNamed` (`internal/theme/events_test.go`) and `captureSink.recordsWithMessage` (`internal/restore/logging_capture_test.go:20`). Deleting the last lets `internal/restore` drop its `capturedRecord` projection and the `newCaptureLogger` wrapper that exists only to host it.
4. Replace `markerReporter` (`internal/restore/marker_assert_test.go:14-18`) with `logtest.TestingT`, and replace `recordingReporter` + its `fatalSentinel` panic-and-recover trick (`marker_assert_meta_test.go:16-45`) with `restoretest`'s existing recording-fataller.

**Acceptance Criteria**:
- [ ] The three chainable filters are unexported and no caller outside `internal/logtest` chains them.
- [ ] `Sink.RecordsWithMessage` exists and returns the records carrying that message under any component.
- [ ] `themeEventRecords`, `recordsNamed` and `captureSink.recordsWithMessage` are all gone, along with `internal/restore`'s wrapper type and projection.
- [ ] `internal/restore` declares no reporter interface and no recording-fataller of its own.
- [ ] Every converted assertion covers the same records it covered before.
- [ ] `go test ./...` and the integration lane pass with no test renamed.

**Tests**:
- `"it returns every record carrying the message regardless of component"`
- `"it returns nothing for a message no record carries"`
- `"it keeps the Sink forwarders behaving identically after the filters are unexported"`
- `"it drives the marker assertion through the shared reporter seam"`
- `"it records a helper's fatal without failing the driving test"`

## Task 16: `hooks.json` test staging is reimplemented seven ways across two packages
severity: duplication
sources: duplication, bank

**Problem**: `internal/hookstest` was created by this work unit as the shared home for `hooks.json` scaffolding, and it holds the sidecar half plus the env-based seeders. The store-at-a-path half was left to each suite, and seven independent versions exist: `storeWithContent` (`internal/hooks/lookup_test.go:14`) and `seedHooksFile` (`store_shape_test.go:17`) have **identical bodies** in the same test package, differing only in return arity; `bogusHooksStore` (`cmd/run_hook_stale_cleanup_outcome_test.go:15`) and `seedHooksDirectory` (`internal/hooks`) both stage a *directory* at the hooks.json path with near-identical explanatory comments; and `seedReadFixture`, `seedThenDenyWrites` and `newStagedHooksStore` (`cmd/testhelpers_test.go:50`) are the same base staging plus one axis each, which `newStagedHooksStore`'s `hooksStoreStaging` struct already models as a parameterised shape. The cost is drift, not verbosity: the sidecar-creation rule (create it before any chmod denial, or the mutation fails at the wrong place) is encoded in two of the seven and absent from the rest. Two adjacent sites ride the same fix: `internal/hooks/read_lock_test.go:353-355` stages a sidecar inline beside its own `seedReadFixture` purely to get one specific key/command pair, and `cmd` keeps a **second** staging route — `hooksFileInTempDir` (`cmd/testhelpers_test.go:114`) + `writeHooksJSON` (`:163`) — that points `PORTAL_HOOKS_FILE` and stages *no* sidecar, so every read through it degrades unasked (visible at `cmd/hooks_read_lock_test.go:80-89`, where the `want` baseline is itself taken from a degraded read).

**Solution**: Move `hooksStoreStaging` + `newStagedHooksStore` into `internal/hookstest` as the one path-based staging entry point, with its existing `dir`/`seed`/`sidecarAbsent`/`writesDenied` axes plus an `unreadable` axis for the directory-at-the-path case and an entries-shaped seed parameter for the read-lock site. Retire `storeWithContent`, `seedHooksFile`, `seedReadFixture`, `seedThenDenyWrites`, `seedHooksDirectory` and `bogusHooksStore` onto it, keeping a thin `cmd` wrapper only where call sites read better for it.

**The default is sidecar present, and the env-pointing route stays a separate documented axis.** Sidecar-absent is transitional — real after the next release until an install's first mutation, permanent for nobody — so defaulting the fixtures to it would model a state that decays, at the cost of an incidental `load-unlocked` breadcrumb across ~30 doctor fixtures. The degraded path stays pinned by the three tests that cover it deliberately, which is where a transitional state belongs.

The two entry points are not folded into one: an injected store and a path the real command body resolves are genuinely different seams, and collapsing them would make a test exercising the command's own resolution indistinguishable from one that injects. The finding was seven ways of staging `hooks.json`, not two — reaching two is the fix.

**Outcome**: Two documented staging routes remain — one path-based store in `internal/hookstest` and one env-pointing route in `cmd` — and the sidecar-before-denial rule is encoded once, in the shared helper, rather than in two of seven copies.

**Do**:
1. Move `hooksStoreStaging` + `newStagedHooksStore` (`cmd/testhelpers_test.go:24-76`) into `internal/hookstest` as the exported path-based staging entry point, keeping the `dir` / `seed` / `sidecarAbsent` / `writesDenied` axes and their doc comments — including the sidecar-created-before-any-chmod ordering, which is the rule the other six copies drop.
2. Add two axes it lacks: `unreadable`, staging a directory at the hooks.json path so every read fails (the `bogusHooksStore` / `seedHooksDirectory` case), and an entries-shaped seed so a caller wanting one specific key/command pair need not hand-write JSON (the `read_lock_test.go:353-355` case).
3. Retire the six onto it: `storeWithContent` (`internal/hooks/lookup_test.go:14`), `seedHooksFile` (`internal/hooks/store_shape_test.go:17`), `seedReadFixture`, `seedThenDenyWrites`, `seedHooksDirectory` (`internal/hooks`) and `bogusHooksStore` (`cmd/run_hook_stale_cleanup_outcome_test.go:15`). Keep a thin `cmd` wrapper only where a call site reads better for it.
4. Delete the inline sidecar staging at `internal/hooks/read_lock_test.go:353-355` in favour of the new seed axis.
5. Leave `hooksFileInTempDir` + `writeHooksJSON` (`cmd/testhelpers_test.go:114,163`) as the second, env-pointing route, but document it as such and give it the sidecar by default — then re-baseline `cmd/hooks_read_lock_test.go:80-89`, whose `want` is currently taken from a degraded read, and keep the three tests whose subject *is* the degraded path pointing at the sidecar-absent axis deliberately.

**Acceptance Criteria**:
- [ ] `internal/hookstest` exports one path-based staging entry point carrying all six axes, and it is the only path-based stager in `internal/hooks` and `cmd`.
- [ ] The sidecar is created before any permission denial in every fixture that stages one.
- [ ] The six retired helpers are gone; no suite hand-writes a hooks.json path plus sidecar inline.
- [ ] The env-pointing route stages a sidecar by default, and `cmd/hooks_read_lock_test.go`'s baseline is taken from a locked read.
- [ ] The three tests whose subject is the degraded read still exercise it, explicitly.
- [ ] No `load-unlocked` breadcrumb appears in a fixture that did not ask for the sidecar-absent axis.
- [ ] `go test ./...` passes with every converted assertion unchanged.

**Tests**:
- `"it stages a hooks.json with its sidecar by default"`
- `"it stages no sidecar when the fixture asks for the absence"`
- `"it creates the sidecar before denying writes to the directory"`
- `"it stages a directory at the hooks.json path so every read fails"`
- `"it seeds the entries a caller names"`
- `"it stages into a named directory that does not exist yet"`

Plus the retired suites keep their existing subjects and assertions.

## Task 17: The `hooks.json` byte-identity assertion has three homes across two packages
severity: duplication
sources: bank

**Problem**: `cmd` owns `assertHooksFileUnchanged` (`cmd/testhelpers_test.go:199`). `internal/hooks/store_test.go` open-codes the same `readFileBytes` + `bytes.Equal(before, after)` comparison four times (`:414`, `:449`, `:503`, `:525`) with near-identical failure messages, because package `hooks_test` cannot reach `cmd`'s helper. `cmd/cleanstale_transient_listpanes_shared_test.go:76` writes a third form over `hookstest.HooksJSONBytes`. A related asymmetry rides the same edit: `cmd` has `readFileBytes` for `hooks.json` but no `projects.json` counterpart, so three symmetric hooks/projects read pairs (`cmd/doctor_test.go:860`, `cmd/run_hook_stale_cleanup_test.go:305,448`) stay raw — converting only the hooks half would split each pair.

**Solution**: Promote the byte-identity assertion into `internal/hookstest` beside `AssertSidecarFree` — a package both `cmd` and `internal/hooks` already import — and have `cmd/testhelpers_test.go:199` delegate rather than duplicate. Add the `projects.json` read counterpart so the three symmetric pairs can convert together.

**Outcome**: One byte-identity assertion serves both packages, and the three hooks/projects read pairs convert together rather than being split down the middle.

**Do**:
1. Promote the byte-identity assertion into `internal/hookstest` beside `AssertSidecarFree`, keeping the optional route-naming context argument `cmd/testhelpers_test.go:199-209` already supports and the ENOENT-tolerant read `readFileBytes` (`:97-109`) provides.
2. Have `cmd`'s `assertHooksFileUnchanged` delegate to it rather than duplicate, so existing `cmd` call sites are untouched.
3. Convert `internal/hooks/store_test.go`'s four open-coded comparisons (`:414`, `:449`, `:503`, `:525`) and `cmd/cleanstale_transient_listpanes_shared_test.go:76`'s third form onto the promoted assertion.
4. Add the `projects.json` read counterpart beside it and convert the three symmetric hooks/projects pairs together: `cmd/doctor_test.go:860`, `cmd/run_hook_stale_cleanup_test.go:305,448`.

**Acceptance Criteria**:
- [ ] One byte-identity assertion exists, in `internal/hookstest`, reachable from both `cmd` and `internal/hooks`.
- [ ] No suite open-codes `bytes.Equal(before, after)` over a hooks.json read.
- [ ] The optional context argument still replaces the default failure wording.
- [ ] An absent file before and after still compares equal rather than fatalling.
- [ ] The three hooks/projects pairs both read through helpers — neither half is left raw.
- [ ] `go test ./...` passes with no assertion weakened.

**Tests**:
- `"it passes when the file is byte-identical before and after"`
- `"it fails when a single byte changed"`
- `"it uses the caller's context in the failure message"`
- `"it treats an absent file as absent rather than fatalling"`
- `"it reads projects.json by the same rule"`

## Task 18: Three doctor drivers, two of them byte-identical apart from one argv element
severity: duplication
sources: duplication

**Problem**: `runDoctorFixCmd` (`cmd/doctor_test.go:1317-1332`) and `runDoctorCmd` (`:1334-1349`) are the same sixteen lines — same `isolateTerminalsFile` call with the same comment, same `doctorDeps` install and cleanup, same two buffers, same `resetRootCmd`/`SetOut`/`SetErr`/`Execute` — differing only in whether `"--fix"` is appended to `SetArgs`. `runDoctor` (`:111-127`) is a third copy that additionally builds the deps from a state dir. Any change to how a doctor run is isolated or driven (a new eagerly-read config file, a third stream) has to be made in three places and will be made in one.

**Solution**: Collapse to one `runDoctorWith(t, deps *DoctorDeps, args ...string) (*bytes.Buffer, *bytes.Buffer, error)` that installs the deps, isolates `terminals.json` and executes `append([]string{"doctor"}, args...)`. Keep `runDoctor(t, dir)` as a two-line wrapper building `withHealthyRuntime(&DoctorDeps{StateDir: dir})` and delegating.

**Outcome**: One place decides how a doctor run is isolated and driven, so a new eagerly-read config file or a third stream is a one-line change rather than three.

**Do**:
1. Add `runDoctorWith(t *testing.T, deps *DoctorDeps, args ...string) (*bytes.Buffer, *bytes.Buffer, error)` in `cmd/doctor_test.go`: install `doctorDeps` with its own `t.Cleanup` restore, call `isolateTerminalsFile` once, wire two buffers through `resetRootCmd`/`SetOut`/`SetErr`, and execute `append([]string{"doctor"}, args...)`.
2. Delete `runDoctorFixCmd` (`:1317-1332`) and `runDoctorCmd` (`:1334-1349`), converting their call sites to `runDoctorWith(t, deps, "--fix")` and `runDoctorWith(t, deps)`.
3. Reduce `runDoctor` (`:111-127`) to a wrapper building `withHealthyRuntime(&DoctorDeps{StateDir: dir})` and delegating.
4. Keep the deps install paired with its restore inside the helper, so no call site can leak a mock into the next test.

**Acceptance Criteria**:
- [ ] `cmd/doctor_test.go` holds one doctor driver plus the `runDoctor` wrapper.
- [ ] `terminals.json` isolation and the `doctorDeps` install/restore each appear once.
- [ ] Every converted call site drives the same argv and reads the same streams it did before.
- [ ] The `--fix` and read-only paths are distinguished only by the args passed.
- [ ] `go test ./cmd` passes with no doctor test renamed or weakened.

**Tests**: no behaviour change, so no new test — the existing `cmd/doctor_test.go` suite is the verification, covering both the read-only diagnosis and the `--fix` repair paths through the collapsed driver, with stdout and stderr still asserted separately.

## Task 19: The hook rm exit suite restates four subtests and carries two latent traps
severity: duplication
sources: duplication, bank

**Problem**: Four issues in one file. **(1)** The `it leaves hooks.json byte-identical on every failing route` table (`cmd/hooks_rm_exit_test.go:114-162`) drives five rows, four of which are already staged and asserted individually earlier in the same function (`:13`, `:52`, `:75`, `:94`), each ending in the same `assertHooksFileUnchanged` call the table exists to make — so the table adds one new case while re-staging four. **(2)** The two tables at `:162` and `:230` carry the same row struct (`name`/`paneID`/`resolver`/`stamper`/`extra`), the same "supply a plain stamper when the row did not set one" comment, and the same eight-line loop preamble. **(3)** `:306` is named "it touches no dirty flag on either path" but both its rows drive `runHookRm` with `TMUX_PANE=%3` and no `--pane-key`, i.e. the resolved-token path twice — they vary the *outcome*, not the path — while the sibling 70 lines above (`:230`, "it mints and stamps nothing on either path") uses the identical phrase to mean resolved-token vs `--pane-key`. **(4)** `:30` asserts `strings.Contains(err.Error(), "no such pane: %999")` where the output is now deterministic after the outer wrap was removed, and the suite's own newer test already pins the exact string for both verbs.

**Solution**: Delete the four already-covered rows from the byte-identity table (or, preferably, drop the four standalone subtests' duplicated staging and let the table own the byte-identity claim while they keep only their message-text assertions); extract the shared row struct and loop preamble into one `runRmCase(t, tt)`; give `:306` a `--pane-key` row or rename it to say "on either outcome"; and tighten `:30` to an exact-string comparison. While in the loop, build the poisoned tmux-call pair *inside* it off a `paneKeyPath` bool rather than hoisting it per parent subtest, so a second `--pane-key` row added later cannot silently share counters.

**Outcome**: Each claim in the file is staged once and asserted once, each table subtest's name describes the axis its rows actually vary, and a `--pane-key` row added later cannot silently share another row's call counters.

**Do**:
1. Strip the duplicated staging from the four standalone subtests (`cmd/hooks_rm_exit_test.go:13`, `:52`, `:75`, `:94`), leaving them their message-text assertions, and let the byte-identity table (`:114-162`) own the byte-identity claim across all five rows.
2. Extract the shared row struct (`name`/`paneID`/`resolver`/`stamper`/`extra`) and the eight-line loop preamble of the two tables at `:162` and `:230` into one `runRmCase(t, tt)`, including the default-plain-stamper rule the comment states twice.
3. Build the poisoned tmux-call pair inside `runRmCase` off a `paneKeyPath` bool on the row, rather than hoisting it per parent subtest.
4. Settle `:306` ("it touches no dirty flag on either path"): give it a `--pane-key` row so it earns the name, or rename it to "on either outcome" — its two rows vary the outcome, not the path, while its sibling at `:230` uses the same phrase for resolved-token versus `--pane-key`.
5. Tighten `:30` from `strings.Contains(err.Error(), "no such pane: %999")` to an exact-string comparison, matching the newer test in the same suite that already pins the exact string for both verbs.

**Acceptance Criteria**:
- [ ] No route is staged twice in the file: the four standalone subtests no longer re-stage what the table stages.
- [ ] Every failing route still ends with a byte-identity assertion, from the table.
- [ ] Both tables drive their rows through one `runRmCase`, with the row struct and preamble declared once.
- [ ] The tmux-call counters are built per row, and a second `--pane-key` row cannot share another row's pair.
- [ ] `:306`'s name matches the axis its rows vary.
- [ ] `:30` compares the error text exactly.
- [ ] Coverage is unchanged: every route, message and exit code asserted before is still asserted.

**Tests**: the file's own subtests are the verification, unchanged in subject:
- `"it leaves hooks.json byte-identical on every failing route"` (five rows, staged once)
- `"it mints and stamps nothing on either path"`
- `"it touches no dirty flag on either <path|outcome>"` (per the settled naming)
- `"it reports tmux's own words for a pane no live pane answers to"` (now exact-string)

## Task 20: Two shape-rule subtests are duplicated verbatim across two files in `internal/hooks`
severity: duplication
sources: duplication

**Problem**: `cleanstale_snapshot_test.go`'s "it still deletes an empty key present in both the file and the snapshot" (`:177-191`) is the same test as `store_shape_test.go`'s "it deletes an empty key" (`:31-62`): same seed string, same `enumerating(liveKey)` call, same `slices.Equal(removed, []string{""})` assertion, same reload-and-check — only the failure wording differs. "it still retains a non-token-shaped key" (`:193-211`) is likewise a trimmed copy of "it retains a non-token-shaped key absent from the live set" (`:88-106`). Neither snapshot-file copy varies the snapshot at all — both use the ordinary path where the file and the snapshot agree — so they exercise no behaviour the shape file does not. Two files now assert one rule, and a change to the rule will land in whichever file the author happens to open.

**Solution**: Delete the two "it still …" subtests from `cleanstale_snapshot_test.go`; that file's own subject — the snapshot narrowing — is already covered by "it retains a key written after the snapshot" and "it derives the delete set from the file under the lock, not from the snapshot". `store_shape_test.go` stays the single home for the shape rule.

**Outcome**: One file asserts the shape rule and one asserts the snapshot narrowing, so a change to either lands where its coverage lives rather than in whichever file the author opened.

**Do**:
1. Confirm before deleting: read both pairs and verify the snapshot copies vary nothing — same seed string, same `enumerating(liveKey)` call, same `slices.Equal(removed, []string{""})` assertion, same reload-and-check, with the file and the snapshot in agreement in both.
2. Delete "it still deletes an empty key present in both the file and the snapshot" (`internal/hooks/cleanstale_snapshot_test.go:177-191`) and "it still retains a non-token-shaped key" (`:193-211`).
3. Leave `store_shape_test.go:31-62` and `:88-106` as the single home for the shape rule, unchanged.
4. Record the subtest-count reduction in the commit message so the loss is deliberate rather than silent.

**Acceptance Criteria**:
- [ ] The two "it still …" subtests are gone from `cleanstale_snapshot_test.go`.
- [ ] The snapshot file still covers its own subject: a key written after the snapshot is retained, and the delete set is derived from the file under the lock rather than from the snapshot.
- [ ] The empty-key deletion rule and the non-token-shaped retention rule are each still asserted exactly once, in `store_shape_test.go`.
- [ ] `go test ./internal/hooks` passes and the commit names the reduction.

**Tests**: coverage moves, it does not grow. These must all still pass and remain the only assertions of their rules:
- `store_shape_test.go`'s `"it deletes an empty key"`
- `store_shape_test.go`'s `"it retains a non-token-shaped key absent from the live set"`
- `cleanstale_snapshot_test.go`'s `"it retains a key written after the snapshot"`
- `cleanstale_snapshot_test.go`'s `"it derives the delete set from the file under the lock, not from the snapshot"`

## Task 21: Two pairs of line-identical subtests in `cmd/hooks_test.go`
severity: duplication
sources: bank

**Problem**: `cmd/hooks_test.go:705` ("--pane-key flag removes specified key without requiring TMUX_PANE") and `:761` ("it removes the verbatim key on rm --pane-key without consulting the resolver") have identical setup — same `eee555`/`other-proj:0.0` seed, same empty `TMUX_PANE`, same argv — and identical assertions, so they are line-for-line the same test under two names; both also overlap `cmd/hooks_rm_exit_test.go:143`, which asserts the same behaviour with a better-named subject. Separately, `:791` ("it errors when TMUX_PANE is unset for the rm fallback") is fixture-for-fixture and assertion-for-assertion identical to `:550` ("returns error when TMUX_PANE is not set") — same discarded-return `hooksFileInTempDir(t)`, same empty `TMUX_PANE`, same `mockKeyResolver{key: "unus00"}`, same argv, same non-nil-error + substring pair, with neither adding anything. It is the rm-side twin of the set-side case a phase-6 task already deleted; the bank entry that produced that task named only the set side.

**Solution**: Delete `:761` and `:791`, keeping `:705` and `:550`, which retain identical coverage. Record the subtest-count reduction in the commit so the loss is deliberate rather than silent.

**Outcome**: Two subtests fewer and not one assertion fewer — each of the two behaviours is asserted once, under the better-named subject.

**Do**:
1. Diff each pair before deleting, confirming the survivor asserts everything its twin does: `:761` against `:705` (same `eee555`/`other-proj:0.0` seed, same empty `TMUX_PANE`, same argv, same assertions) and `:791` against `:550` (same discarded-return `hooksFileInTempDir`, same empty `TMUX_PANE`, same `mockKeyResolver{key: "unus00"}`, same argv, same non-nil-error plus substring pair).
2. Delete `cmd/hooks_test.go:761` ("it removes the verbatim key on rm --pane-key without consulting the resolver") and `:791` ("it errors when TMUX_PANE is unset for the rm fallback").
3. Leave `:705` and `:550` unchanged; note that `cmd/hooks_rm_exit_test.go:143` independently covers the `--pane-key` behaviour under a better-named subject, so nothing rests on `:761` alone.
4. Record the subtest-count reduction in the commit message.

**Acceptance Criteria**:
- [ ] `:761` and `:791` are deleted; `:705` and `:550` are untouched.
- [ ] The `--pane-key`-without-`TMUX_PANE` removal is still asserted, in `cmd/hooks_test.go:705` and `cmd/hooks_rm_exit_test.go:143`.
- [ ] The unset-`TMUX_PANE` rm error is still asserted, in `cmd/hooks_test.go:550`.
- [ ] `go test ./cmd` passes and the commit names the reduction.

**Tests**: coverage is retained by the survivors, which must stay green and unchanged:
- `"--pane-key flag removes specified key without requiring TMUX_PANE"`
- `"returns error when TMUX_PANE is not set"`
- `cmd/hooks_rm_exit_test.go:143`'s `--pane-key` route case

## Task 22: The reboot bracket and its `Orchestrator` literals are open-coded at five sites
severity: duplication
sources: bank

**Problem**: `restoretest.RebootServer` and `restoretest.RestoreFromState` (`internal/restoretest/reboot.go:22,40`) now own the reboot bracket, and `newRenameRebootFixture` routes through them — but `internal/restore/multipane_legacy_integration_test.go:111-128` and `:184-204` still open-code `KillServer` → list-sessions must-fail guard → `EnsureServer` → `restore.Orchestrator{…}` → `RestoreWithMarker` verbatim, and the same file carries a third and fourth copy of the ~25-line arrange (differing by pane count and hook vocabulary, so it needs a parameterised variant). Beyond that, five files hand-build a bare `restore.Orchestrator{Client, StateDir, Logger, Exe}` literal (`armed_restore_integration_test.go:62,140`; `exit_closes_pane_integration_test.go:164`; `integration_full_test.go:116`; `multipane_legacy_integration_test.go:122,198`) — correct today, but `Exe` is **opt-in at every one of them**, and a forgotten field is silent: with `os.Executable()` as the default, a test that drives a real restore without setting `Exe` arms its panes with the *test binary*, which stops flag parsing at the `state` positional, re-runs its own suite inside the tmux pane and exits 0. The symptom is a vanished session, not an error. Two smaller pieces ride the same edit: `restoretest.StagedHydrateExe`, `restoreAdapterFor` (`cmd/bootstrap/helpers_integration_test.go:19`) and `stagedRestoreAdapter` (`cmd/reattach_integration_test.go:55`) are three helpers doing one job; and eight `restoretest.PrependPATH` sites survive whose stated reason (the hydrate helper) is now pinned through `Exe`, while others (a fixture whose orchestrator registers real `portal state notify` global hooks) still genuinely need it.

**Solution**: Route the remaining open-coded reboot sequences through `RebootServer`/`RestoreFromState` with a parameterised arrange for the multipane variants, and make `Exe` structurally impossible to omit — a `restoretest` constructor taking `binDir` and always setting it, as the only supported route to a real-restore `Orchestrator`, backed by a source guard over bare `restore.Orchestrator{` literals in `*_test.go`. Audit the eight surviving `PrependPATH` sites per-test and drop the dead ones. While in `restoretest`, close the marker bracket's other half: only the *unset* of `@portal-restoring` is asserted anywhere (`integration_full_test.go:285`, `armed_restore_integration_test.go:83`) — deleting the `SetServerOption` from `restore_marker.go:21` leaves both lanes green, so one assertion that the marker is set during `Restore()` would cover every caller at once.

**Outcome**: A test cannot build a real-restore `Orchestrator` without pinning `Exe`, no reboot bracket is open-coded, and deleting the `@portal-restoring` set from the restore path fails a test instead of leaving both lanes green.

**Do**:
1. Add a `restoretest` constructor taking `binDir` that builds `restore.Orchestrator` with `Exe` always set through `StagedHydrateExe`, and convert the six bare literals to it: `armed_restore_integration_test.go:62,140`, `exit_closes_pane_integration_test.go:164`, `integration_full_test.go:116`, `multipane_legacy_integration_test.go:122,198`. Add a source guard failing any bare `restore.Orchestrator{` composite literal in a `*_test.go`, so a forgotten `Exe` — which silently arms panes with the test binary and exits 0 — cannot recur.
2. Route `internal/restore/multipane_legacy_integration_test.go:111-128` and `:184-204` through `restoretest.RebootServer` / `RestoreFromState`, and give the file's third and fourth ~25-line arrange copies one parameterised variant taking the pane count and hook vocabulary they differ by.
3. Collapse the three staged-exe helpers to one: `restoretest.StagedHydrateExe`, `restoreAdapterFor` (`cmd/bootstrap/helpers_integration_test.go:19`) and `stagedRestoreAdapter` (`cmd/reattach_integration_test.go:55`).
4. Audit the eight surviving `restoretest.PrependPATH` sites one test at a time and delete those whose stated reason was the hydrate helper (now pinned through `Exe`), keeping the ones whose orchestrator registers real `portal state notify` global hooks.
5. Add the missing half of the marker bracket in `restoretest`: assert `@portal-restoring` is set during `Restore()`, so deleting the `SetServerOption` at `restore_marker.go:21` fails rather than passing both lanes.

**Acceptance Criteria**:
- [ ] No `*_test.go` composes a bare `restore.Orchestrator{…}` literal; the source guard fails one that does and fatals when it scans no files.
- [ ] Every real-restore orchestrator in a test has `Exe` set, and a pane armed by one never respawns into the test binary.
- [ ] No reboot bracket (`KillServer` → must-fail guard → `EnsureServer` → restore) is open-coded outside `restoretest`.
- [ ] The multipane arranges share one parameterised variant differing only by pane count and hook vocabulary.
- [ ] One staged-exe helper survives, reached from `internal/restore`, `cmd` and `cmd/bootstrap`.
- [ ] Deleting the `SetServerOption` from `restore_marker.go:21` fails at least one test.
- [ ] Each surviving `PrependPATH` call has a reason that still holds; the integration lane passes without the deleted ones.

**Tests**:
- `"it fails a test file composing a bare restore.Orchestrator literal"`
- `"it fatals when the orchestrator guard scans no files"`
- `"it sets @portal-restoring for the duration of a restore"`
- `"it clears @portal-restoring when the restore returns"`
- `"it opens the reboot gap before restoring"`
- `"it restores a multipane legacy session through the shared arrange"`

## Task 23: The seed-key vocabulary is re-declared in four packages
severity: duplication
sources: duplication, bank

**Problem**: `internal/hookstest` exports `ReapableHookKey(n)` but not the named seeds built on it, so each consumer restates the naming convention and each is free to re-point a name at a different index. `cmd/hookkey_vocabulary_test.go:22-35` declares `reapableSeedA`…`reapableSeedD` plus `liveSeedA`…`liveSeedC`; `internal/hooks/store_test.go:598-605`, `cleanstale_snapshot_test.go:27-29` and `store_shape_test.go:27-28` re-derive the same indices under their own names — and two of them bind a local named `liveKey` to `ReapableHookKey(0)`, the exact name-asserting-live-on-a-reapable-seed pattern a phase-6 task removed from `cmd`. `cmd/state_daemon_hook_cleanup_integration_test.go:47,51` (package `cmd_test`, so it cannot see the `cmd` vocabulary) and `cmd/bootstrap/transient_listpanes_helpers_integration_test.go:108-109` re-derive indices 0 and 1 inline. Two daemon fixtures (`cmd/state_daemon_hook_cleanup_test.go:68`, `cmd/state_daemon_run_test.go:555`) hand-build a two-entry `hooks.json` body that `staleHookSeed` already names, differing only in the stale entry's command text.

**Solution**: Export the named live/reapable seed vars (and the shared two-entry seed body) from `internal/hookstest`, and point all four packages at them. Reconciling the stale entry's command string across the two daemon suites is part of the work.

**Outcome**: One package names the seed keys, so no suite can re-point a name at a different index and no local `liveKey` can be bound to a reapable seed.

**Do**:
1. Export the named seeds from `internal/hookstest` beside `ReapableHookKey` / `UnjudgeableHookKey`: the reapable set (`ReapableSeedA`…`ReapableSeedD`) and the live set (`LiveSeedA`…`LiveSeedC`), each documented for what it means rather than for which index it wraps.
2. Export the shared two-entry `hooks.json` seed body the daemon fixtures hand-build (`cmd/state_daemon_hook_cleanup_test.go:68`, `cmd/state_daemon_run_test.go:555`), and reconcile the stale entry's command string across the two so one value survives.
3. Point the four packages at the exported names, deleting their local declarations: `cmd/hookkey_vocabulary_test.go:22-35`, `internal/hooks/store_test.go:598-605`, `internal/hooks/cleanstale_snapshot_test.go:27-29`, `internal/hooks/store_shape_test.go:27-28`.
4. Delete the two locals named `liveKey` bound to `ReapableHookKey(0)` — a name asserting live on a reapable seed is the pattern already removed from `cmd` — and use a live seed where the test means live.
5. Convert the inline index re-derivations at `cmd/state_daemon_hook_cleanup_integration_test.go:47,51` (package `cmd_test`, which cannot see `cmd`'s vocabulary) and `cmd/bootstrap/transient_listpanes_helpers_integration_test.go:108-109`.

**Acceptance Criteria**:
- [ ] `internal/hookstest` is the only declaration site for the named seeds and the two-entry seed body.
- [ ] No package re-derives a seed index inline or re-declares a seed name.
- [ ] No local named `liveKey` is bound to a reapable seed.
- [ ] The two daemon suites seed the same stale command string.
- [ ] Every converted assertion judges the same key it judged before — a reapable seed stays reapable, a live seed stays live.
- [ ] Both lanes pass.

**Tests**: vocabulary consolidation with no behaviour change, so no new test — the existing suites verify it, and their subjects are unchanged:
- `internal/hooks`'s shape and snapshot suites, still judging the same keys
- `cmd`'s hook-key vocabulary and stale-cleanup suites
- the two daemon hook-cleanup suites, now over one seed body
- `hookstest.ReapableHookKey`'s existing token-shape panic still guards the vocabulary

## Task 24: The repo's source guards hand-roll the primitives `sourceguardtest` exists to own
severity: duplication
sources: duplication, bank

**Problem**: Two families. **(1)** `TestCleanStaleDoesNotCallStaleKeys` (`internal/hooks/cleanstale_staleness_guard_test.go:16-42`) and `TestMutationsDoNotCallExportedLoadOrSave` (`:61-100`) share a twenty-line skeleton written twice — `sourceguardtest.PackageGoFiles(".", false)` with the same fatal wording, the per-path `parser.ParseFile(… SkipObjectResolution)` with the same fatal wording, the `scanned++` counter, the `ForEachFuncCall` visit, and the closing empty-check — with only the predicate differing; and `calleeName` (`:44-52`) is a copy of the `callName` helper `sourceguardtest`'s own test carries, and is the natural companion to the exported `ForEachFuncCall` it is always used with. **(2)** Four leaf/import guards each hand-roll the same `go list -deps` exec-and-parse with the same fatal: `internal/nanoid/leaf_guard_test.go:29` (wrapped as `packageDeps`), `internal/hooks/leaf_guard_test.go:59`, `internal/prefs/leaf_guard_test.go:20`, `internal/theme/leaf_guard_test.go:25`. `internal/sourceguardtest` is the declared home for guard primitives — stdlib-only and untagged so every guard it drives runs in the unit lane — and has neither a call-scan skeleton nor a dependency enumerator.

**Solution**: Extract a local `scanPackageCalls(t, visit func(path, funcName string, call *ast.CallExpr))` in the hooks guard file owning the enumerate/parse/count/empty-check skeleton, leaving each guard only its predicate; promote `calleeName` to `sourceguardtest.CalleeName`; and add `sourceguardtest.PackageDeps(t, pkg) []string`, routing all four leaf guards through it.

**Outcome**: `sourceguardtest` owns the callee-name and dependency-enumeration primitives its siblings already share, and each guard in the tree reads as its predicate rather than as a re-authored scan.

**Do**:
1. Extract `scanPackageCalls(t, visit func(path, funcName string, call *ast.CallExpr))` in `internal/hooks/cleanstale_staleness_guard_test.go`, owning the `sourceguardtest.PackageGoFiles(".", false)` enumeration, the per-path `parser.ParseFile(… SkipObjectResolution)`, the `scanned++` counter, the `ForEachFuncCall` visit and the closing empty-check — with one fatal wording each. Reduce `TestCleanStaleDoesNotCallStaleKeys` (`:16-42`) and `TestMutationsDoNotCallExportedLoadOrSave` (`:61-100`) to their predicates.
2. Promote `calleeName` (`:44-52`) to `sourceguardtest.CalleeName(call *ast.CallExpr) string` beside `ForEachFuncCall`, which it is always used with, and delete the copy in `sourceguardtest`'s own test.
3. Add `sourceguardtest.PackageDeps(t *testing.T, pkg string) []string` wrapping the `go list -deps` exec-and-parse with one fatal wording.
4. Route all four leaf guards through it, deleting their local copies: `internal/nanoid/leaf_guard_test.go:29` (`packageDeps`), `internal/hooks/leaf_guard_test.go:59`, `internal/prefs/leaf_guard_test.go:20`, `internal/theme/leaf_guard_test.go:25`.

**Acceptance Criteria**:
- [ ] Both hooks guards keep their own predicate and share one scan skeleton, including the scanned-zero fatal.
- [ ] `CalleeName` and `PackageDeps` are exported from `sourceguardtest` with their own coverage.
- [ ] No package hand-rolls `go list -deps` or a callee-name unwrapper.
- [ ] `sourceguardtest` stays stdlib-only and untagged, so every guard it drives still runs in the unit lane.
- [ ] Each converted guard still fails on the violation it was written to catch, and still fatals rather than passing when it scans nothing.

**Tests**:
- `"it unwraps an identifier call to its name"`
- `"it unwraps a selector call to its name"`
- `"it returns empty for a call expression with neither shape"`
- `"it enumerates a package's transitive dependencies"`
- `"it fails when go list cannot resolve the package"`
- `"it fatals when the shared scan enumerates no files"`

Plus each converted guard still catching its own violation.

## Task 25: Nine `*Deps` seams in `cmd`, one of them guarded
severity: duplication
sources: bank

**Problem**: `cmd` declares nine package-level test seams (`cmd/doctor.go:72`, `cmd/hooks.go:39`, `cmd/kill.go:9`, `cmd/list.go:11`, `cmd/open_burst_run.go:13`, `cmd/root.go:36`, `cmd/open.go:36`, `cmd/state_commit_now.go:30`, `cmd/uninstall.go:21`). This work unit solved the install-and-restore pairing for `hooksDeps` alone — `withHooksDeps` plus `cmd/hooks_deps_guard_test.go`. The other eight carry the identical unguarded pattern: bare `X = &XDeps{...}` + a separate `t.Cleanup` at 108 sites across 22 files for `bootstrapDeps`, 54 across 7 for `openDeps`, 4 for `doctorDeps`. Same leak vector — a missed or mis-ordered cleanup leaks a mock into the next test in a package whose `TestMain` poison is the only other line of defence — and now with a proven helper-plus-guard pattern to generalise.

**Solution**: Give each remaining seam a `withXDeps(t, deps)` helper that installs and registers its own restore, convert the call sites, and parameterise the existing guard over the identifier list so every seam is covered by one test. A single generic `withDeps` is not reachable without changing the seam representation, so the realistic form is one helper per seam plus one guard over all nine.

**Outcome**: Every `cmd` test seam is installed through a helper that registers its own restore in the same breath, and one guard covers all nine — so a mock cannot leak into the next test in the package through a missed or mis-ordered cleanup.

**Do**:
1. Add a `withXDeps(t, deps)` helper per remaining seam, modelled on `withHooksDeps` (`cmd/testhelpers_test.go:82-86`): install the package-level pointer and register the `t.Cleanup` restore inside the helper. Cover `doctorDeps`, `bootstrapDeps`, `openDeps`, `killDeps`, `listDeps`, `openBurstRunDeps`, `rootDeps`, `commitNowDeps` and `uninstallDeps` (`cmd/doctor.go:72`, `cmd/root.go:36`, `cmd/open.go:36`, `cmd/kill.go:9`, `cmd/list.go:11`, `cmd/open_burst_run.go:13`, `cmd/state_commit_now.go:30`, `cmd/uninstall.go:21`).
2. Add the `withoutXDeps(t)` counterpart wherever a suite's subject is the production default, matching `withoutHooksDeps` (`:91-95`).
3. Convert the call sites — 108 across 22 files for `bootstrapDeps`, 54 across 7 for `openDeps`, 4 for `doctorDeps`, plus the remainder — deleting each bare assignment and its separate `t.Cleanup`.
4. Parameterise `cmd/hooks_deps_guard_test.go` over the nine seam identifiers so one test asserts that no `*_test.go` assigns a seam outside its helper.

**Acceptance Criteria**:
- [ ] Each of the nine seams has an install helper that registers its own restore.
- [ ] No `*_test.go` in `cmd` assigns a seam pointer directly.
- [ ] The guard covers all nine identifiers and fails when a bare assignment is reintroduced for any of them.
- [ ] The guard fatals rather than passing when it scans no files.
- [ ] Every converted test drives the same deps it drove before, and `go test ./cmd` plus the integration lane pass.

**Tests**:
- `"it restores the seam when the test that installed it finishes"`
- `"it leaves the seam unset for a test that asks for the production default"`
- `"it fails a test file assigning a seam outside its helper"`
- `"it covers every declared seam identifier"`
- `"it fatals when the guard scans no files"`

## Task 26: Seven bespoke `Commander` fakes across the `cmd` test files
severity: duplication
sources: bank

**Problem**: `fakeCommander` (`cmd/state_daemon_test.go:254`), `membershipFakeCommander` (`:714`), `envFailingCommander` (`cmd/state_daemon_capture_logging_test.go:269`), `daemonFakeCommander` (`cmd/state_daemon_run_test.go:26`), `recordingCommander` (`cmd/uninstall_test.go:26`), `stubCommander` (`cmd/open_test.go:1726`) and `gonePaneCommander` (`cmd/hooks_seams_test.go:93`) all fake the same `tmux.Commander` interface, alongside `transienttest.Commander` which already exists as a shared one. One scripted fake — argv pattern in, canned result out, with optional call recording — would serve most of them, and the interface is small enough that seven independent implementations of it will diverge on the `Run`/`RunRaw` trim-vs-verbatim split the moment that contract changes.

**Solution**: Introduce one scripted `Commander` fake (in `cmd`'s test helpers, or promoted beside `transienttest.Commander`) and retire the seven onto it, keeping a bespoke type only where a test genuinely needs behaviour a script cannot express.

**Outcome**: One scripted `Commander` fake serves the `cmd` suites, so the `Run`/`RunRaw` trim-versus-verbatim split is honoured in one implementation rather than seven that will diverge the moment it changes.

**Do**:
1. Write one scripted fake: an argv-pattern-to-result table with a recorded call log and an explicit default for an unmatched argv, honouring the `Run`/`RunRaw` trim-versus-verbatim contract in one place. Home it beside `transienttest.Commander` if a package outside `cmd` needs it, otherwise in `cmd`'s test helpers.
2. Retire the seven onto it: `fakeCommander` (`cmd/state_daemon_test.go:254`), `membershipFakeCommander` (`:714`), `envFailingCommander` (`cmd/state_daemon_capture_logging_test.go:269`), `daemonFakeCommander` (`cmd/state_daemon_run_test.go:26`), `recordingCommander` (`cmd/uninstall_test.go:26`), `stubCommander` (`cmd/open_test.go:1726`) and `gonePaneCommander` (`cmd/hooks_seams_test.go:93`).
3. Keep a bespoke type only where a test needs behaviour a script cannot express — a per-call state machine, a blocking call — and state the reason at its declaration.
4. Leave `transienttest.Commander`'s `FailureMode` contract as the single canonical declaration it already is; do not fold it into the new fake.

**Acceptance Criteria**:
- [ ] One scripted `Commander` fake exists and serves the retired sites.
- [ ] An unmatched argv produces an explicit, stated outcome rather than a silent zero value.
- [ ] Each surviving bespoke fake names the behaviour a script cannot express.
- [ ] Every converted test asserts on the same argv and the same results it did before.
- [ ] `transienttest`'s failure-mode contract is unchanged.
- [ ] `go test ./cmd` and the integration lane pass.

**Tests**:
- `"it returns the scripted result for a matching argv"`
- `"it records every call in order"`
- `"it takes the stated default for an unmatched argv"`
- `"it trims Run output and leaves RunRaw verbatim"`
- `"it returns the scripted error for a failing argv"`

## Task 27: 76 inline `logtest.Sink` installs survive outside `logtest.Install`
severity: duplication
sources: bank

**Problem**: `logtest.Install(t)` now exists and is used at ~190 sites, but the two-line `sink := &logtest.Sink{}` + `log.SetTestHandler(t, sink)` is still written inline at 76 remaining `SetTestHandler` call sites across ~20 files — concentrated in `cmd/open_test.go` (20), `internal/theme/events_test.go` (6), `cmd/run_hook_stale_cleanup_test.go` (6), `main_panic_test.go` (5) and `cmd/hooks_test.go` (5). Three declared fixtures also open-code the same two lines inside a broader helper (`cmd/state_hydrate_exec_failure_test.go:16`, `cmd/state_daemon_self_eject_log_test.go:20`, `internal/theme/resolve_test.go:21`), and two sites wrap the sink in a local struct (`internal/tmux/portal_saver_lifecycle_events_test.go:43`, `internal/restore/logging_capture_test.go:32`) that must be unpicked first.

**Solution**: Convert the remaining sites to `logtest.Install(t)`, unpicking the two wrapper structs as part of the pass. Mechanical, but it spans `cmd`, `tmux`, `theme`, `tui`, `restore`, `prefs` and `capturetool` — which is why it needs to be one deliberate sweep rather than opportunistic drift.

**Outcome**: `logtest.Install(t)` is the only route to a package-level capture handler, so the sink construction and the handler swap are paired by construction at every site.

**Do**:
1. Convert the 76 remaining inline `sink := &logtest.Sink{}` + `log.SetTestHandler(t, sink)` pairs to `sink := logtest.Install(t)`, concentrated in `cmd/open_test.go` (20), `internal/theme/events_test.go` (6), `cmd/run_hook_stale_cleanup_test.go` (6), `main_panic_test.go` (5) and `cmd/hooks_test.go` (5).
2. Convert the three declared fixtures that open-code the same two lines inside a broader helper: `cmd/state_hydrate_exec_failure_test.go:16`, `cmd/state_daemon_self_eject_log_test.go:20`, `internal/theme/resolve_test.go:21`.
3. Unpick the two local wrapper structs first — `internal/tmux/portal_saver_lifecycle_events_test.go:43` and `internal/restore/logging_capture_test.go:32` — routing their consumers at the `Sink` directly. Coordinate with Task 15, which deletes the `internal/restore` wrapper's reason for existing.
4. Sweep for surviving `log.SetTestHandler` calls afterwards and justify each remaining one — `cmd/logging_capture_test.go:29` installs a discard handler before `log.Init`, which is a different job.

**Acceptance Criteria**:
- [ ] No `*_test.go` constructs a `logtest.Sink` and calls `log.SetTestHandler` as two adjacent statements.
- [ ] The two wrapper structs are gone and their consumers read the `Sink` directly.
- [ ] Every surviving `log.SetTestHandler` call is doing something `Install` does not.
- [ ] Every converted assertion captures the same records it captured before.
- [ ] `go test ./...` and the integration lane pass with no test renamed.

**Tests**: mechanical conversion with no behaviour change, so no new test — the existing suites across `cmd`, `internal/tmux`, `internal/theme`, `internal/tui`, `internal/restore`, `internal/prefs` and `cmd/capturetool` are the verification, with `logtest.Install`'s own existing coverage (it installs a fresh sink and restores the prior handler on cleanup) unchanged.

## Task 28: 54 inline `hydrateConfig` literals and a superseded builder
severity: duplication
sources: bank

**Problem**: `hydrateCfg` + `hydrateCfgOpts` (`cmd/state_hydrate_test.go:939`) was created as the one route to a `hydrateConfig`, but 54 inline `hydrateConfig{...}` literals remain across the hydrate suites (42 in `state_hydrate_test.go`, 7 in `state_hydrate_exec_log_test.go`, 5 in `state_hydrate_file_missing_log_test.go`). `fileMissingCfg` (`cmd/state_hydrate_file_missing_log_test.go:21`) is a fourth suite-local builder taking the identical seven positional parameters its two now-retired siblings took, differing only in setting `HandleFileMissing` and omitting `HandleTimeout` — and since `hydrateCfg` wires both handlers, it fully expresses all three of `fileMissingCfg`'s call sites, which all pass `openFIFOWithTimeout` and never reach the timeout path. Two styles also sit side by side within one file: `cmd/state_hydrate_test.go:1343,1394,1427` assign `cfg.HookStore` *after* the builder call although `hydrateCfgOpts` already carries the field.

**Solution**: Route the inline literals through `hydrateCfg`, delete `fileMissingCfg`, and move the three post-hoc `cfg.HookStore` assignments into the opts struct so one style survives.

**Outcome**: One builder produces every `hydrateConfig` in the hydrate suites, so a new required field is added once rather than 54 times, and one style of setting a field survives.

**Do**:
1. Route the 54 inline `hydrateConfig{...}` literals through `hydrateCfg` + `hydrateCfgOpts` (`cmd/state_hydrate_test.go:939`): 42 in `state_hydrate_test.go`, 7 in `state_hydrate_exec_log_test.go`, 5 in `state_hydrate_file_missing_log_test.go`. Extend `hydrateCfgOpts` with any field an inline literal sets that it does not yet carry.
2. Delete `fileMissingCfg` (`cmd/state_hydrate_file_missing_log_test.go:21`) and convert its three call sites to `hydrateCfg` — all three pass `openFIFOWithTimeout` and never reach the timeout path, so the builder wiring both handlers fully expresses them.
3. Move the three post-hoc `cfg.HookStore` assignments (`cmd/state_hydrate_test.go:1343,1394,1427`) into the opts struct, which already carries the field.
4. Check the converted suites for a case whose subject is a *missing* config field — that case must set the field to its zero value explicitly through the opts rather than relying on an inline literal's omission.

**Acceptance Criteria**:
- [ ] No inline `hydrateConfig{...}` literal survives in the three hydrate suites.
- [ ] `fileMissingCfg` is deleted and its three call sites read through `hydrateCfg`.
- [ ] No `cfg.HookStore` is assigned after the builder call.
- [ ] Every converted test drives the same config values it drove before, including any deliberate zero value.
- [ ] `go test ./cmd` passes with no hydrate test renamed or weakened.

**Tests**: no behaviour change, so no new test — the hydrate suites are the verification and keep their subjects:
- `cmd/state_hydrate_test.go`'s replay, exec-chain and hook-lookup cases
- `cmd/state_hydrate_exec_log_test.go`'s exec-chain log cases
- `cmd/state_hydrate_file_missing_log_test.go`'s file-missing cases, now built by `hydrateCfg`

## Task 29: `hookstest` re-implements `cmd/config.go`'s hooks-path resolution chain
severity: duplication
sources: bank

**Problem**: `hookstest.ResolveHooksFilePathFromEnv` (`internal/hookstest/hooks.go:21-42`) walks an env slice for `PORTAL_HOOKS_FILE` then `XDG_CONFIG_HOME`, duplicating the precedence `configFilePath` owns in `cmd/config.go`. A third env layer, or an ordering change in production, silently leaves the seeder resolving the old path — and every destructive integration suite that uses it then seeds into a file the code under test never reads, so the test passes by asserting on a file nothing touched. The two are not merely similar; the helper exists precisely to answer "where will the binary under test look?", which makes any divergence a false green rather than a mismatch.

**Solution**: Expose an env-slice-taking resolver from `cmd/config.go` (or factor the precedence into a leaf both can import) and have `hookstest` delegate to it, so the seeder and the binary resolve by the same rule by construction.

**Outcome**: The seeder and the binary under test resolve `hooks.json` by one rule, so a change to the production precedence cannot leave a destructive integration suite seeding a file nothing reads and passing on it.

**Do**:
1. Factor `configFilePath`'s precedence (`cmd/config.go:57-77`) into a form that takes its env lookups as a parameter — an env-slice-taking resolver exported from `cmd`, or the precedence moved into a leaf both `cmd` and `internal/hookstest` can import. Keep the one-shot Application Support migration on the production path only; a seeder must not trigger it.
2. Have `hookstest.ResolveHooksFilePathFromEnv` (`internal/hookstest/hooks.go:21-42`) delegate to it, keeping its `*testing.T`-first signature and its fatal when the slice carries neither `PORTAL_HOOKS_FILE` nor `XDG_CONFIG_HOME` — that fatal is an isolation-regression tripwire, not a precedence rule.
3. Pin the delegation with a test that drives both routes over the same env and asserts the same path: one env with `PORTAL_HOOKS_FILE` set, one with only `XDG_CONFIG_HOME`, and one with both (the file variable wins).
4. Check the consumers still resolve correctly: `SeedHooksJSON` and `HooksJSONBytes` in the same file, and the destructive integration suites that use them.

**Acceptance Criteria**:
- [ ] The precedence — `PORTAL_HOOKS_FILE`, then `XDG_CONFIG_HOME`, then the home fallback — is declared once and read by both routes.
- [ ] `hookstest` no longer walks the env slice by its own rule.
- [ ] The seeder path performs no config migration and creates nothing the production read would not.
- [ ] The seeder still fatals when the env slice carries neither variable.
- [ ] Adding a third env layer to the production precedence changes both routes together, provably by the new test.
- [ ] Both lanes pass.

**Tests**:
- `"it resolves PORTAL_HOOKS_FILE ahead of XDG_CONFIG_HOME"`
- `"it resolves under XDG_CONFIG_HOME when no file variable is set"`
- `"it resolves the same path as the production rule for the same env"`
- `"it fatals when the env slice carries neither variable"`
- `"it triggers no config migration on the seeder path"`

## Task 30: `session.NewPaneToken` is a vestigial forwarder that keeps pane identity in the session package
severity: dead-code
sources: duplication, architecture

**Problem**: `internal/session/panetoken.go:8-10` is a one-line pass-through over `nanoid.NewGenerator()()`, added when the id vocabulary lived in `internal/session`. The vocabulary has since moved to the `internal/nanoid` leaf and every other minting site calls `nanoid.NewGenerator()` directly (`cmd/open.go:414,567`, `internal/spawn/burst.go:58`). The wrapper adds no width, no charset and no validation, so the package exposes a second name for the same value — and because `HooksDeps.TokenMinter` is typed `session.IDGenerator` (itself an alias of `nanoid.Generator`), `cmd/hooks.go` imports `internal/session` solely for a type alias and a forwarder concerning *pane* identity, in a package the architecture description scopes to the session-creation pipeline. Two conventions for one thing, and the one that routes through `session` puts pane identity where it does not belong.

**Solution**: Type the seam as `nanoid.Generator`, default it to `nanoid.NewGenerator()`, and delete `internal/session/panetoken.go` and the `internal/session` import from `cmd/hooks.go`. Both id-minting call sites in `cmd` then read the same way.

**Outcome**: Pane identity is minted from the id leaf everywhere, `internal/session` holds nothing about panes, and `cmd/hooks.go` no longer imports the session-creation package for a type alias and a forwarder.

**Do**:
1. Retype `HooksDeps.TokenMinter` (`cmd/hooks.go:45`) from `session.IDGenerator` to `nanoid.Generator`.
2. Default it to `nanoid.NewGenerator()` at `cmd/hooks.go:99-101` in place of `session.NewPaneToken`, matching how `cmd/open.go:414,567` already mint.
3. Delete `internal/session/panetoken.go` and the `internal/session` import from `cmd/hooks.go` (check nothing else in the file needs it).
4. Update any test that injects a `TokenMinter` to the new type, and check `internal/session`'s own suite for a `NewPaneToken` case to remove.

**Acceptance Criteria**:
- [ ] `internal/session/panetoken.go` no longer exists and nothing references `session.NewPaneToken`.
- [ ] `cmd/hooks.go` does not import `internal/session`.
- [ ] `HooksDeps.TokenMinter` is `nanoid.Generator` and defaults to `nanoid.NewGenerator()`.
- [ ] `hook set` mints and stamps exactly as before: a pane with no token gets one, a pane already carrying one keeps it and no `set-option` is issued.
- [ ] A minting failure still ends the command non-zero with nothing written to `hooks.json`.
- [ ] Both lanes pass.

**Tests**: the mint is unchanged in width, charset and behaviour, so the verification is the existing `hook set` coverage staying green with its subjects intact:
- `"it mints and stamps a token for a pane carrying none"`
- `"it reuses the token a pane already carries and issues no set-option"`
- `"it exits non-zero and writes nothing when the mint fails"`
- `"it recognises the minted token as token-shaped"`

## Task 31: `Client.ListPanes` has no production caller
severity: dead-code
sources: bank

**Problem**: `internal/tmux/tmux.go:573` is reachable only from tests (`tmux_test.go:1295-1401`, plus `exact_session_target_test.go:81` and `exact_session_target_realtmux_test.go:126,234`) and satisfies no seam interface. It is the third exported client method in that family with zero production callers — the other two were deleted, leaving the task's outcome statement ("the tmux client exports no pane-listing method with zero production callers") only two-thirds true. It is not costless: this work unit corrected its doc, pinned its wire form and gave it two real-tmux subtests, all maintained for an unused export.

**Solution**: Settle it rather than leaving it half-cleaned.

**Delete it.** `Client.ListPanes` has no production caller, satisfies no seam interface, and is the third of its family — the other two were already deleted, which is what makes the outcome statement "the tmux client exports no pane-listing method with zero production callers" currently two-thirds true. This work unit paid maintenance on it: a corrected doc, a pinned wire form and two real-tmux subtests, all for an unused export. Re-point the two `exactCoordTarget` routing proofs at a method with a production caller; the coverage they give is about the target vocabulary, not about `ListPanes`, so it survives the move.

**Outcome**: The tmux client exports no pane-listing method with zero production callers, and the exact-coordinate-target routing proofs survive on a method that is actually used.

**Do**:
1. Confirm the emptiness before deleting: grep the tree for `ListPanes(` and verify every hit is a test or the declaration itself, and that no seam interface in `cmd`, `internal/state`, `internal/restore` or `internal/tui` declares the method.
2. Delete `Client.ListPanes` (`internal/tmux/tmux.go:560-569`) and its own coverage at `tmux_test.go:1295-1401`.
3. Re-point the two `exactCoordTarget` routing proofs — `exact_session_target_test.go:81` and `exact_session_target_realtmux_test.go:126,234` — at a method with a production caller that routes through `exactCoordTarget` (`ListPanesInSession` and `ActivePaneCurrentPath` both do), keeping their subject: that the target vocabulary pins the session and does not resolve a prefix sibling.
4. Check `parsePaneOutput` for a remaining caller; if `ListPanes` was its last one, it goes too.

**Acceptance Criteria**:
- [ ] `Client.ListPanes` is gone and no seam interface declares it.
- [ ] The exact-coordinate-target routing proofs still exist and still assert the same target-vocabulary property, on a method with a production caller.
- [ ] No helper is orphaned by the deletion.
- [ ] `go build ./...`, `go test ./...` and the integration lane all pass.

**Tests**: coverage moves rather than shrinking — these keep asserting what the deleted method's tests asserted about the target vocabulary:
- `"it pins the session component of a coordinate target"`
- `"it does not resolve a prefix sibling through a coordinate target"`
- `"it fails against a gone session rather than reaching a sibling"` (real tmux)

## Task 32: Claims the rewrite left behind — stale prose and an unreachable error promise
severity: comments
sources: bank

**Problem**: Two claims in the tree are now false. **(1)** Eight sites in `cmd` name the deleted `ListAllPanes` in test names and failure messages, including a subtest literally called "it enumerates live keys via ListAllPaneHookKeys not ListAllPanes" — a contrast against a method that no longer exists (`cmd/run_hook_stale_cleanup_test.go:85,96,102,105,222`; `cmd/state_daemon_hook_cleanup_test.go:167,171`; `cmd/state_daemon_hook_cleanup_integration_test.go:210`). The assertions are correct — they describe the `ListAllPanesWithFormat` seam's error path — only the prose is stale. **(2)** `ActivePaneCurrentPath`'s doc (`internal/tmux/tmux.go:245-247`) states "A session killed mid-read surfaces as `errors.Is(err, ErrNoSuchSession)`, which callers can treat as unresolvable rather than fatal". Measured on tmux 3.7c, `display-message -p -t <unmatched>` exits **0** with an empty expansion and no stderr, so the method returns `("", nil)` and `wrapNoSuchSession` is unreachable on this path — a fact the `exactTarget` doc 170 lines below in the same file now states outright ("display-message instead returns empty with exit 0 — for a live session as much as a gone one"). The caller is the TUI's lazy dir-resolution fallback, which must already treat empty-and-nil as unresolved.

**Solution**: Rename the eight test names and failure messages onto `ListAllPanesWithFormat`, and correct `ActivePaneCurrentPath`'s contract to describe the empty-and-nil return the method actually produces — with the caller's handling of that case verified rather than assumed.

**Outcome**: No test name or doc comment in the tree names a method that no longer exists or promises an error the code cannot produce, and the TUI's handling of the return `ActivePaneCurrentPath` actually gives is verified rather than assumed.

**Do**:
1. Rename the eight stale references from `ListAllPanes` to `ListAllPanesWithFormat` in test names and failure messages: `cmd/run_hook_stale_cleanup_test.go:85,96,102,105,222`, `cmd/state_daemon_hook_cleanup_test.go:167,171`, `cmd/state_daemon_hook_cleanup_integration_test.go:210`. The subtest literally named "it enumerates live keys via ListAllPaneHookKeys not ListAllPanes" contrasts against a deleted method — recast the contrast against the seam that exists. The assertions themselves are correct and must not change.
2. Correct `ActivePaneCurrentPath`'s doc (`internal/tmux/tmux.go:246-248`) to describe the `("", nil)` return the method produces: `display-message -p -t <unmatched>` exits 0 with an empty expansion and no stderr, so `wrapNoSuchSession` is unreachable on this path — the fact `exactTarget`'s doc already states at `:419-420`.
3. Verify the caller rather than assuming it: the TUI's lazy dir-resolution fallback (`internal/session/dirresolve.go`'s `PaneCurrentPathReader` seam and its `internal/tui` consumer) must treat empty-and-nil as unresolved. Add coverage for that case if none pins it, and fix the caller if it does not.
4. Decide whether the now-dead `wrapNoSuchSession` call on this path stays; if it goes, keep the outer error wrap so a genuine command failure still reports.

**Acceptance Criteria**:
- [ ] No test name or failure message in `cmd` names `ListAllPanes`.
- [ ] Every renamed assertion asserts exactly what it asserted before.
- [ ] `ActivePaneCurrentPath`'s doc describes the empty-and-nil return and makes no unreachable `ErrNoSuchSession` promise.
- [ ] The lazy dir-resolution fallback treats an empty path with a nil error as unresolved, and that is covered by a test.
- [ ] A genuine `display-message` command failure still returns a wrapped error.

**Tests**:
- `"it returns empty and nil for a session no pane answers to"`
- `"it treats an empty current path as unresolved in the grouped render"`
- `"it returns a wrapped error when display-message itself fails"`

Plus the eight renamed `cmd` subtests, whose subjects and assertions are unchanged.

## Task 33: `internal/portalbintest` compiles the portal binary in the unit lane
severity: low
sources: bank

**Problem**: `internal/portalbintest/build_test.go` runs a real `go build` of the CLI on every `go test ./...`. It only `exec.LookPath`s the result rather than running it, so it does not breach the lane rule as written ("every test that spawns a `portal state daemon` or execs a built `portal` binary lives behind `-tags integration`") — but it is the one remaining unit-lane test that *produces* a portal binary, and it costs a full build on every unit run, on the lane whose whole promise is being fast and hermetic.

**Solution**: Settle whether the lane rule covers builds as well as execs, and align the test with the answer.

**Move it behind `//go:build integration` and widen the lane rule to cover builds**, stated in CLAUDE.md. The unit lane's whole promise is fast and hermetic, and a full CLI build on every `go test ./...` is neither, whatever the letter of the current rule permits. Keeping it in the fast lane was the alternative and was rejected: every consumer of `portalbintest` lives in the integration lane, so a broken build helper fails there immediately anyway — the fast-lane copy buys earlier notice of a failure that cannot stay hidden.

**Outcome**: `go test ./...` compiles no portal binary, and CLAUDE.md's lane rule says so — a test that builds the CLI is as much an integration test as one that execs it.

**Do**:
1. Add `//go:build integration` to `internal/portalbintest/build_test.go`.
2. Widen the lane rule in CLAUDE.md from "every test that spawns a `portal state daemon` or execs a built `portal` binary" to include one that *builds* it, and say why: the fast lane's promise is fast and hermetic.
3. Check `internal/portalbintest` still has unit-lane coverage for whatever needs none of a build — `ProjectRoot` in particular, which the source guards call from the unit lane.
4. Measure `go test ./...` before and after and record the saving in the commit message, so the change's whole justification is checkable.

**Acceptance Criteria**:
- [ ] `go test ./...` compiles no portal binary; `internal/portalbintest`'s build test runs only under `-tags integration`.
- [ ] `go test -tags integration -p 1 ./...` still covers the build helper.
- [ ] `ProjectRoot` keeps unit-lane coverage, since unit-lane source guards depend on it.
- [ ] CLAUDE.md's lane rule names builds alongside daemon spawns and binary execs.
- [ ] The unit lane's wall time is measurably lower and the figure is in the commit message.

**Tests**: the moved test keeps its subject — `"it builds the portal binary and returns a runnable path"` — and now runs in the integration lane only. `ProjectRoot`'s unit-lane coverage stays where it is.

## Task 34: The `doctor --fix` stand-down copy reassigns a signed-off user-facing string
severity: medium
sources: analysis-standards-c2 (S1)

**Problem**: The specification fixed three user-facing lines and named the condition each covers: `Skipped stale hook prune: could not read live panes` was written for the **empty-live-set guard**. `cmd/doctor.go:215-221` now renders five phrases and gives that exact string to the new `pane-read-failed` branch, while the empty-live-set guard it was written for renders the invented `live pane list came back empty`. A user who saw the signed-off line and learned what it meant now sees it for a different condition, with nothing announcing the swap. Separately, `cmd/doctor.go:308-313` carries a second stand-down phrase map in a different register (`could not enumerate live panes`, `zero live panes with hooks present (not evaluable)`) that is missing `lock-timeout` entirely — so a lock-timeout stand-down falls through to its raw reason value on the read-only diagnosis path.

The five-reason enumeration itself is settled and the specification has been corrected to match it: the two added conditions are real, previously exited through paths that recorded nothing, and distinguishing five beats collapsing to three. Only the string assignment is open.

**Solution**: Settle the assignment per the Decision, then close the second half regardless of which side is taken — give `notEvaluableDetails` its missing `lock-timeout` entry so no stand-down reaches a user as a raw enum value, and add a case per reason to whichever test pins this copy, so a future branch cannot silently borrow another's words.

**Both branches get explicit words and the disputed string is retired.** `could not read live panes` reads most naturally as a failed read, which is what `pane-read-failed` is — but it was signed off for the empty-live-set guard, which is a *successful* read of an empty list. Rather than argue which branch owns an ambiguous phrase, neither keeps it: the guard gets wording that says the list came back empty, the failure gets wording that says the enumeration failed, and the retirement is recorded as a deliberate amendment in the specification. Silently reassigning it (today's state) and restoring it to a branch it describes poorly were both rejected.

Close the second half regardless of wording: give `notEvaluableDetails` its missing `lock-timeout` entry so no stand-down reaches a user as a raw enum value, and add a case per reason to whichever test pins this copy, so a future branch cannot silently borrow another's words.

**Outcome**: `Skipped stale hook prune: could not read live panes` no longer appears anywhere; both the empty-live-set guard and the pane-read failure carry wording that names their own condition; every reason renders a phrase on both surfaces; and the retirement is recorded as a deliberate amendment rather than a silent reassignment.

**Do**:
1. Retire `could not read live panes` from `skippedPrunePhrases` (`cmd/doctor.go:215-221`). Give `skipReasonPaneReadFailed` wording that says the enumeration failed and `skipReasonEmptyPaneRead` wording that says the list came back empty, so neither branch borrows the other's condition. Neither keeps the disputed string.
2. Add the missing `skipReasonLockTimeout` entry to `notEvaluableDetails` (`:308-313`), so a lock-timeout stand-down never reaches the read-only diagnosis as a raw enum value.
3. Read the phrases in both maps against each other for register: `skippedPrunePhrases` completes "Skipped stale hook prune: …" for a user who asked for a repair; `notEvaluableDetails` names why the count could not be taken.
4. Add a case per reason to the test that pins this copy, asserting the exact rendered line for each of the five on both surfaces, so a future branch cannot silently borrow another's words.
5. Record the retirement in the specification's Corrigenda section, in the register of the two entries already there: the §5.1 line `Skipped stale hook prune: could not read live panes` is withdrawn, both conditions get wording naming themselves, and the reason it is withdrawn rather than restored is that it was signed off for a *successful* read of an empty list while reading most naturally as a failed read.

**Acceptance Criteria**:
- [ ] The string `could not read live panes` appears in no renderer.
- [ ] `pane-read-failed` renders wording naming a failed enumeration; `empty-pane-read` renders wording naming an empty result. The two are not interchangeable.
- [ ] All five reasons render a non-empty phrase in `skippedPrunePhrases` and in `notEvaluableDetails`, `lock-timeout` included.
- [ ] The exact rendered line for each reason is pinned by a test on both surfaces.
- [ ] The specification's Corrigenda carries the withdrawal, naming what replaced it and why.
- [ ] Neither the `--fix` exit code nor the read-only exit code changes: both stay driven by the post-repair diagnosis.

**Tests**:
- `"it names a failed pane enumeration in the skipped-prune line"`
- `"it names an empty pane list in the skipped-prune line"`
- `"it renders a distinct phrase for each of the five stand-down reasons"`
- `"it renders a not-evaluable detail for a lock-timeout stand-down"`
- `"it renders no raw reason slug on either surface"`
- `"it leaves the exit code to the post-repair diagnosis for every stand-down"`

## Task 35: A second lock bound the specification never decided
severity: low
sources: analysis-standards-c2 (S3)

**Problem**: The specification fixes one lock-acquisition bound — 2 seconds — and describes the sweep's advisory pre-read as degrading by the same rule as every other read. `internal/hooks/lock.go:17-33` declares a second package-level bound, `snapshotLockTimeout = 20 * time.Millisecond`, applied only to the clean's pre-read. Correctness is unaffected and the reasoning is the specification's own — a doubled wait would park the daemon's 1s tick — but the observable behaviour differs from what was signed off: under ordinary contention, such as a concurrent `hook set` holding the exclusive lock for a few milliseconds, the pre-read routinely falls through to the unlocked read and emits `op=load-unlocked` at DEBUG, where a single 2s bound almost never would.

**Solution**: Settle the bound per the Decision. Whichever side is taken, the two bounds must not both sit as bare package-level constants with no stated relationship — the one that survives carries a comment naming what it protects and why its value is what it is, in the register the 2s bound already uses.

**Derive the pre-read bound from `lockTimeout` rather than declaring an unrelated constant.** The defect named is two bare package-level bounds with no stated relationship; deriving makes the relationship visible at the declaration and stops them drifting. The worst-case argument the 20ms figure buys — one `lockTimeout` per clean rather than two — is preserved, and the more frequent `load-unlocked` fallback under contention is the accepted price, recorded in the specification beside the 2s figure. Dropping the separate bound entirely was the alternative and was rejected: it doubles a clean's worst case against a daemon that ticks every second.

**Outcome**: The pre-read bound is written as a fraction of `lockTimeout`, so the relationship between the two is visible at the declaration and lowering the bound in a test moves both together; and the specification records the second bound and the more frequent degraded read it produces.

**Do**:
1. Derive `snapshotLockTimeout` (`internal/hooks/lock.go:31`) from `lockTimeout` rather than declaring an independent duration, keeping the effective figure at or near today's 20ms so a clean's worst case stays one `lockTimeout` rather than two.
2. Rewrite the doc comment above it in the register the `lockTimeout` comment already uses (`:17-21`): what it protects (the daemon's 1s tick against a doubled wait), why it is derived from `lockTimeout` rather than declared beside it, and that the pre-read is advisory so degrading costs nothing but a DEBUG breadcrumb.
3. Check the unit lane's bound-lowering still works: the suites lower `lockTimeout` to exercise the timeout split (`cmd/testhelpers_test.go:22`'s `lockBound` is the driving figure), and a derived pre-read bound must move with it rather than staying at a production figure.
4. Record the bound in the specification beside the 2s figure in §6.5, including the observable consequence the analysis measured: under ordinary contention the pre-read routinely falls through to the unlocked read and emits `op=load-unlocked` at DEBUG, where a single 2s bound almost never would.

**Acceptance Criteria**:
- [ ] `snapshotLockTimeout` is expressed in terms of `lockTimeout`, not as an independent literal.
- [ ] Its comment names what it protects and why its value is what it is, matching the `lockTimeout` comment's register.
- [ ] A clean's worst case is one `lockTimeout`, not two.
- [ ] Lowering `lockTimeout` in a test lowers the pre-read bound with it.
- [ ] The pre-read still degrades to an unlocked read on timeout, emitting `op=load-unlocked` at DEBUG with `via=internal`, and never fails the clean.
- [ ] The specification records the second bound and the more frequent degraded read beside the 2s figure.

**Tests**:
- `"it bounds the clean's pre-read below the mutation bound"`
- `"it degrades the pre-read to an unlocked read when the sidecar is held"`
- `"it emits load-unlocked at DEBUG with via=internal for the degraded pre-read"`
- `"it still takes the exclusive lock at the full bound for the deletion"`
- `"it caps a clean's worst case at one lock timeout"`
- `"it lowers the pre-read bound with the mutation bound under test"`

