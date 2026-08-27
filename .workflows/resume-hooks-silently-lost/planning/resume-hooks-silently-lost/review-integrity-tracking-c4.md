# Review Tracking: Resume Hooks Silently Lost - Integrity

## Findings

### 1. Three reboot tests that prove a hook fires are left keyed to a coordinate

**Severity**: Important
**Plan Reference**: Phase 3, task `resume-hooks-silently-lost-3-1`
**Category**: Task Self-Containment
**Move**: settled
**Change Type**: add-to-task

**Problem**:
Three tests are the only executable proof in the repository that a resume hook actually fires after a reboot, and all three seed `hooks.json` under the positional `<session>:window.pane` key:

- `cmd/bootstrap/phase2_hook_fire_integration_test.go` — `betaHookKey := tmux.PaneTarget("beta", 0, 0)`, saved state seeded through `restoretest.SeedSessionsJSON`
- `cmd/bootstrap/reboot_roundtrip_test.go` — `savedHookKey := tmux.PaneTarget("alpha", cfg.saveBase+0, cfg.savePaneBase+0)`, saved state captured live through `runDaemonTick`, asserted by `verifyHookFiredOnce`
- `internal/restore/exit_closes_pane_integration_test.go` — `hookKey := tmux.PaneTarget(sessionName, 0, 0)` in `setupExitClosesPane`, saved state captured live through `state.CaptureStructure`

The moment `collectArmInfos` bakes `p.PortalPaneID`, none of the three fixtures has a saved token, so the baked key is empty, the hook fires on nothing, and all three fail. None of them is named anywhere in the plan — not in this task's fixture inventory, not in any other task, and no Phase 3 task otherwise touches `cmd/bootstrap` or `exit_closes_pane_integration_test.go`. The plan names every other blocked fixture down to the line number, so an implementer reads the inventory as complete and meets three red integration tests with no instruction on how to re-point them — and the re-pointing is not mechanical: one fixture seeds `sessions.json` directly and needs a saved token in the seed, while the other two capture from a live server and need the pane stamped before the capture. The cheap way out of a red test here is to stop asserting the hook fires, which would delete the only coverage that the whole work unit's outcome is real.

**Proposal**:
Add the three files to task 3-1's fixture work, with the re-pointing route stated per file (stamp-before-capture for the two live-capture fixtures, a token-carrying seeder for the one that writes `sessions.json` directly), an acceptance criterion that each keeps asserting the hook fires exactly once, and an edge case recording that these break at 3-1 rather than at 3-3. The breakage lands at 3-1 because that is where the bake changes; 3-3 only turns an empty key from `--hook-key ''` into an absent flag, which does not change firing.

**Current**:
The closing two lines of task 3-1's **Acceptance Criteria**:

```
- [ ] `internal/state/portal_id_literal_guard_test.go` is deleted, not re-pointed, and no replacement guard is added
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass
```

**Proposed Text**:

Add as a new final bullet of task 3-1's **Do**:

```
- Re-point the three reboot fixtures that seed a positional key and assert the hook fires — none of which any other task in this phase touches, and all of which restore from saved state, so after this task the baked `--hook-key` is that pane's saved `PortalPaneID` and a positional seed fires nothing. Where the fixture captures from a live server, stamp the pane with `client.SetPaneOption(target, state.PortalPaneIDOption, token)` **before** the capture and seed `hooks.json` under that same token: `cmd/bootstrap/reboot_roundtrip_test.go`'s `runRebootRoundTrip` (`savedHookKey` at `:80-81`, captured by `runDaemonTick`, asserted by `verifyHookFiredOnce`) and `internal/restore/exit_closes_pane_integration_test.go`'s `setupExitClosesPane` (`hookKey` at `:137`, captured by `state.CaptureStructure`). Where the fixture writes `sessions.json` directly — `cmd/bootstrap/phase2_hook_fire_integration_test.go` (`betaHookKey` at `:42`, seeded by `restoretest.SeedSessionsJSON`) — add a token-carrying sibling seeder in `internal/restoretest`, `SeedSessionsJSONWithPaneTokens(t, stateDir, tokens map[string]string)`, which sets `Pane.PortalPaneID` on each named session's single pane and leaves `SeedSessionsJSON`'s signature and its other callers untouched, and seed `hooks.json` under that token. All three keep asserting that the hook fires exactly once, on that pane.
```

Add to task 3-1's **Acceptance Criteria**, in place of the two lines quoted above:

```
- [ ] `internal/state/portal_id_literal_guard_test.go` is deleted, not re-pointed, and no replacement guard is added
- [ ] `cmd/bootstrap/phase2_hook_fire_integration_test.go`, `cmd/bootstrap/reboot_roundtrip_test.go` and `internal/restore/exit_closes_pane_integration_test.go` each carry a saved pane token and a `hooks.json` keyed by it, and each still asserts its hook fires exactly once — no fixture is repaired by dropping the firing assertion
- [ ] `restoretest.SeedSessionsJSON`'s signature and its existing callers are unchanged; the token-carrying seed is a sibling helper
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass
```

Add to task 3-1's **Edge Cases**:

```
- The three reboot firing fixtures break at *this* task and not at 3-3: the bake changing to `p.PortalPaneID` is what makes a positionally-seeded entry fire nothing, and 3-3 only changes an empty key from `--hook-key ''` to an absent flag
```

**Resolution**: Pending
**Notes**:

---

### 2. The sweep is told to use the short lock bound but has no way to ask for it

**Severity**: Important
**Plan Reference**: Phase 5, task `resume-hooks-silently-lost-5-2` (consumed by `resume-hooks-silently-lost-5-4`)
**Category**: Task Self-Containment
**Move**: settled
**Change Type**: add-to-task

**Problem**:
Phase 5 rests on the sweep taking `hooks.json` twice per cycle — a shared advisory pre-read, then the exclusive hold inside `CleanStale` — and on the pre-read waiting only the near-zero `snapshotLockTimeout`, so a wedged writer cannot park the daemon's 1s tick for two full bounds every ten seconds. Task 5-1 threads the bound through `acquireLock` as a parameter for exactly this. But task 5-2, which owns the read path, defines only `loadShared(via string)` with no bound of its own and three exported reads — `Load(via)`, `List(via)`, `Get(key, via)` — that all route through it, and `Load` is the same method `checkStaleHooks` calls, which task 5-2 requires to wait the full bound. The sweep's call site lives in `cmd` and can reach nothing else. Task 5-4 then instructs the implementer to take the snapshot "through task 5-2's short-bound read", naming something that does not exist on the store's surface.

The implementer has to invent the missing API, and the invention that costs least keystrokes is the wrong one: branch inside `loadShared` on `via == "internal"`. That binds a blocking bound to a log attribute — renaming a breadcrumb would silently double the sweep's worst-case stall — and the two-bound regression it reintroduces is precisely the failure the plan's own timing test is written to catch.

**Proposal**:
Give the bound an explicit parameter and one exported short-bound entry point the sweep can name. `loadSharedBounded(via, bound)` carries the acquire, `loadShared(via)` becomes its `lockTimeout` delegation for every ordinary read, and `LoadSnapshot(via)` is the exported `snapshotLockTimeout` read whose only caller is the sweep's pre-read — after which task 5-4's existing "task 5-2's short-bound read" resolves to a named method. The prohibition on deriving the bound from `via` is stated where the temptation is, since `via=internal` uniquely identifies the pre-read today and so the shortcut would work until someone renamed it.

**Current**:
Task 5-2's **Do**, second and third bullets:

```
- In `internal/hooks/store.go` add unexported `loadShared(via string) (hooksFile, error)`: attempt the shared acquire; on **any** error (absent sidecar, absent directory, unreadable file, or the bound elapsing) emit exactly one `logger.Debug("load-unlocked", "op", "load-unlocked", "via", via, "error", err)` and fall through to the non-locking `load()`; on success `defer f.Close()` and return `load()`, so the shared hold is released when the read returns and is never handed to the caller.
- Give the exported reads a `via` parameter and route all three through `loadShared`: `Load(via string)`, `List(via string)`, `Get(key, via string)`. A per-call parameter rather than a `Store` field is required because `portal doctor` hands the *same* `*Store` value to the sweep (`internal`) and to `checkStaleHooks` (`doctor`) in one run.
```

**Proposed Text**:

```
- In `internal/hooks/store.go` add unexported `loadSharedBounded(via string, bound time.Duration) (hooksFile, error)`: attempt the shared acquire at `bound`; on **any** error (absent sidecar, absent directory, unreadable file, or the bound elapsing) emit exactly one `logger.Debug("load-unlocked", "op", "load-unlocked", "via", via, "error", err)` and fall through to the non-locking `load()`; on success `defer f.Close()` and return `load()`, so the shared hold is released when the read returns and is never handed to the caller. Add `loadShared(via string) (hooksFile, error)` as its one-line delegation at `lockTimeout` — the bound every ordinary read takes.
- Give the exported reads a `via` parameter and route all three through `loadShared`: `Load(via string)`, `List(via string)`, `Get(key, via string)`. A per-call parameter rather than a `Store` field is required because `portal doctor` hands the *same* `*Store` value to the sweep (`internal`) and to `checkStaleHooks` (`doctor`) in one run.
- Add one further exported read, `LoadSnapshot(via string) (hooksFile, error)`, delegating to `loadSharedBounded(via, snapshotLockTimeout)`. It exists because the sweep's advisory pre-read is in `cmd` and cannot reach an unexported helper, and it is the only read that takes the short bound; its doc comment says so and says why — a cycle that takes the sidecar twice must use it for the pre-read, or a wedged writer costs that cycle two full bounds. The bound must be a parameter and must never be derived from the `via` value: `via` is a log attr, `internal` happens to identify the pre-read uniquely today, and binding a blocking bound to a breadcrumb makes renaming one a change in the daemon's worst-case stall.
```

Add to task 5-2's **Acceptance Criteria**, immediately after the existing line `- [ ] The sweep's advisory pre-read waits at \`snapshotLockTimeout\`, not \`lockTimeout\`, so a contended sweep cycle spends one full bound in total rather than two; every other read waits at \`lockTimeout\``:

```
- [ ] The short bound is selected by an explicit parameter, never from the `via` value: `LoadSnapshot` is the only read that passes `snapshotLockTimeout`, `loadShared` is the only other caller of `loadSharedBounded` and passes `lockTimeout`, and no branch anywhere reads `via` to choose a bound
```

Add to task 5-2's **Edge Cases**:

```
- The bound is a parameter, not a function of `via`. `via=internal` identifies the pre-read uniquely today, so a `via`-driven branch would pass every test in this task and silently restore the two-bound stall the moment a breadcrumb was renamed
```

**Resolution**: Pending
**Notes**: With `LoadSnapshot` named, task 5-4's existing Do wording ("the call-site snapshot taken through task 5-2's short-bound read at `via="internal"`") resolves to a real method and needs no edit.

---

### 3. A reaper test keeps passing after it stops testing the reaper

**Severity**: Minor
**Plan Reference**: Phase 1, task `resume-hooks-silently-lost-1-1`
**Category**: Acceptance Criteria Quality
**Move**: settled
**Change Type**: add-to-task

**Problem**:
`internal/hooks/store_test.go` holds three clean-stale fixtures seeded with positional keys, and the task's site list names only one of them (`TestCleanStale`). Two of the unnamed ones fail loudly once retention lands, which the task's "both lanes pass" criterion catches. The third does not: `TestCleanStaleRemovesExactlyStaleKeys` (`:763`) seeds `a:0.0`–`d:0.0`, computes a `StaleKeys` prediction and asserts `CleanStale`'s result equals it. Under the shape rule both sides become empty and the equality holds, so the test goes green while proving nothing at all — and the one test whose whole job is that the two staleness readers agree stops being able to detect them disagreeing. Nothing in the task or its criteria would reveal it.

**Proposal**:
Name all three fixtures in the site list, and say of the prediction-comparison one that it passes vacuously so the reason for re-pointing it is on the page rather than left to be rediscovered. Add a criterion that pins a non-empty removal set, since "both lanes pass" cannot.

**Current**:
Within task 1-1's final **Do** bullet:

```
Sites: `internal/hooks/store_test.go` (`TestCleanStale`, `my-session:0.1` / `:0.2`), `cmd/run_hook_stale_cleanup_test.go` (`a:0.0` / `b:0.0`),
```

**Proposed Text**:

```
Sites: `internal/hooks/store_test.go` — `TestCleanStale` (`my-session:0.1` / `:0.2`), `TestCleanStaleLogging`'s per-entry subtest (`my-session:0.0` / `:0.1` / `:0.2`, which asserts two removals), its save-failure subtest (`a:0.0` / `b:0.0` at `:917`, which has no write left to fail unless something is removed), and `TestCleanStaleRemovesExactlyStaleKeys` (`a:0.0`–`d:0.0` at `:763`), which compares `CleanStale`'s result against a `StaleKeys` prediction and therefore **passes vacuously** once the shape rule empties both sides — it must seed token-shaped keys or the one test that pins the two readers agreeing silently stops testing anything; `cmd/run_hook_stale_cleanup_test.go` (`a:0.0` / `b:0.0`),
```

Add to task 1-1's **Acceptance Criteria**:

```
- [ ] `TestCleanStaleRemovesExactlyStaleKeys` still compares a non-empty removal set against a non-empty prediction — an equality between two empty slices does not satisfy it
```

Add to task 1-1's **Edge Cases**:

```
- A fixture whose assertion is an equality between `CleanStale`'s result and a `StaleKeys` prediction passes vacuously once the shape rule empties both sides; seeding token-shaped keys is what keeps it measuring the agreement it exists to pin
```

**Resolution**: Pending
**Notes**:
