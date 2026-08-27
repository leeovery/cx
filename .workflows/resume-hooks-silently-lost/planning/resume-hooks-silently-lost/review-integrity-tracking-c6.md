# Review Tracking: Resume Hooks Silently Lost - Integrity

## Findings

### 1. The daemon's only unit-lane proof that a live pane keeps its hook can go green while proving nothing

**Severity**: Important
**Plan Reference**: Phase 2, task `resume-hooks-silently-lost-2-1`
**Category**: Acceptance Criteria Quality / Task Self-Containment
**Move**: settled
**Change Type**: add-to-task

**Problem**:
`TestMaybeRunHookCleanup_RunsAndResetsOnceIntervalElapsed` (`cmd/state_daemon_hook_cleanup_test.go:59`) is the daemon sweep's only unit-lane evidence that a hook belonging to a *live* pane survives a prune: it seeds `stale:0.0` and `live:0.0`, hands the sweep a live enumeration of `live:0.0`, and asserts the first is reaped and the second is not. When this task turns the enumeration into two-field rows, that fixture's `panesOut: "live:0.0"` stops parsing — `parsePaneHookRows` rejects a row with no separator, `ListAllPaneHookKeys` returns `(nil, err)`, the sweep takes its enumeration-error branch and reaps nothing, and the test fails. The task's inventory names the two *integration* fixtures that carry a live key (`cleanstale_transient_listpanes_doctorfix_integration_test.go:91-94` and `state_daemon_hook_cleanup_integration_test.go:80-81`) but not this one, so the executor meets it as an unexplained failure rather than a directed re-point. The cheapest repair — writing the row as `|live:0.0`, an unstamped pane — turns both assertions green again: with no live tokens at all, the token-shaped `stale` key is reaped and `live:0.0` survives because Phase 1 retains non-token-shaped keys, not because its pane is alive. The suite is then left with no unit-lane check that the daemon protects a living pane's hook — the guarantee the whole work unit rests on — and nothing anywhere fails to say so. That is the same shape of blind spot the work unit exists to close: coverage that passes because it stopped measuring the thing.

**Proposal**:
Name the fixture in the task's re-point inventory beside the two integration ones and spell out the required shape — the live pane's row carries a token and the retained `hooks.json` key is that same token — then pin the property with an acceptance criterion and a test name so the coverage cannot be dropped by leaving the row unstamped. The rule itself is already settled in this task's own Edge Cases ("Fixtures asserting an entry is *preserved because its pane is live* must be re-pointed to a stamped token, or they silently start passing on Phase 1's retention rule instead of on liveness"); the only gap is that the inventory never names the file the rule lands on. Task 1-1 already touches this file and explicitly defers it ("the live enumeration is still positional in this phase"), so this task is where the deferral is owed its follow-through.

**Current**:

Do — final bullet, tail of the sites list (fragment):

```
the live key in `cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:91-94` (`live:0.0`); and the `tmux.StructuralKeyFormat` live-key read at `cmd/state_daemon_hook_cleanup_integration_test.go:80-81`, which stamps a token on the live pane and seeds under it instead.
```

Acceptance Criteria — anchor criterion:

```
- [ ] A token-shaped key absent from the live token set is still deleted when rows are present
```

Tests — anchor entry:

```
- `"it keeps a restored token-keyed hook across a rename and the sweep"` — the re-pointed `cmd/rename_restore_cleanup_survival_integration_test.go`
```

**Proposed Text**:

Do — final bullet, tail of the sites list (fragment):

```
the live key in `cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:91-94` (`live:0.0`); `cmd/state_daemon_hook_cleanup_test.go`, whose six `daemonFakeCommander{panesOut: "live:0.0"}` fixtures must render the two-field row format or the enumeration errors and the sweep reaps nothing — and whose `TestMaybeRunHookCleanup_RunsAndResetsOnceIntervalElapsed` (`:59`) is the only unit-lane proof that a live pane's hook survives, so its row must carry a **token** (`<token>|live:0.0`) with the `live:0.0` seed re-pointed at that same token, never an unstamped `|live:0.0` row, which would leave the entry surviving on Phase 1's shape retention with both assertions still green; and the `tmux.StructuralKeyFormat` live-key read at `cmd/state_daemon_hook_cleanup_integration_test.go:80-81`, which stamps a token on the live pane and seeds under it instead.
```

Acceptance Criteria — anchor criterion, with the new criterion appended after it:

```
- [ ] A token-shaped key absent from the live token set is still deleted when rows are present
- [ ] The daemon sweep's live-pane fixture still measures liveness: `TestMaybeRunHookCleanup_RunsAndResetsOnceIntervalElapsed` enumerates a row carrying a token and keeps its retained entry keyed by that same token, so the entry survives because its pane is live and not because its key is unjudgeable
```

Tests — anchor entry, with the new entry appended after it:

```
- `"it keeps a restored token-keyed hook across a rename and the sweep"` — the re-pointed `cmd/rename_restore_cleanup_survival_integration_test.go`
- `"it keeps a live pane's token-keyed hook across the daemon sweep"` — the re-pointed `TestMaybeRunHookCleanup_RunsAndResetsOnceIntervalElapsed`: a stamped row plus an entry keyed by that token survives, while a token-shaped key naming no row is reaped in the same cycle
```

**Resolution**: Pending
**Notes**:

---

### 2. A new file appears in the user's config directory with nothing in the user's documentation naming it

**Severity**: Minor
**Plan Reference**: Phase 5, task `resume-hooks-silently-lost-5-1`
**Category**: Task Self-Containment / Acceptance Criteria Quality
**Move**: settled
**Change Type**: add-to-task

**Problem**:
The first `hook set` after this task creates `hooks.json.lock` beside `hooks.json` in `~/.config/portal/`, and nothing ever removes it. The task directs a clause in CLAUDE.md's `hooks` row "so a new on-disk file is not later read as cruft" — the right instinct, aimed at the wrong reader. CLAUDE.md is the agent's map; the file a user actually opens is README, whose Configuration section is an explicit inventory of the config directory, one row per file, and which already names Portal's own machinery artifacts (the `state/` row lists `daemon.pid` and `daemon.version` as liveness markers). After this task that table is wrong by omission: a user who lists their config directory finds a file the only documentation of the directory does not mention, on the exact directory this work unit is teaching them to inspect when a hook goes missing. The plan's own argument — a new on-disk file must not be readable as cruft — applies with more force to the person who might delete it than to the agent that would not.

**Proposal**:
Extend the same Do step to cover README's Configuration table alongside CLAUDE.md, and add an acceptance criterion so both land. The task carries no documentation criterion at all today, so the CLAUDE.md clause it already directs is unverified as well; one criterion covers both. Scope stays minimal — the existing `hooks.json` row gains a clause, no other README passage is touched, matching how task 4-2 treats the same file.

**Current**:

Do — final bullet:

```
- Add a clause to CLAUDE.md's `hooks` row naming the sidecar lock file beside `hooks.json`, so a new on-disk file is not later read as cruft.
```

Acceptance Criteria — anchor criterion:

```
- [ ] `error_class=write-failed-temp-create` is still exercised by the amended write-failure fixtures
```

**Proposed Text**:

Do — final bullet:

```
- Add a clause to CLAUDE.md's `hooks` row naming the sidecar lock file beside `hooks.json`, so a new on-disk file is not later read as cruft, and extend README's Configuration table the same way. That table is the user-facing inventory of the config directory and already names Portal's own machinery files — the `state/` row lists `daemon.pid` and `daemon.version` — so its `hooks.json` row gains a clause naming the `hooks.json.lock` sidecar Portal creates beside it on the first mutation and never removes. Touch no other README passage.
```

Acceptance Criteria — anchor criterion, with the new criterion appended after it:

```
- [ ] `error_class=write-failed-temp-create` is still exercised by the amended write-failure fixtures
- [ ] CLAUDE.md's `hooks` row and README's Configuration table both name the `hooks.json.lock` sidecar, so neither the architecture description nor the user-facing config inventory leaves a file Portal creates undocumented
```

**Resolution**: Pending
**Notes**:
