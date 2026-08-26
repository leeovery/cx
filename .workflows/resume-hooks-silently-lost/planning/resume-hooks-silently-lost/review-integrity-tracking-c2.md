# Review Tracking: Resume Hooks Silently Lost - Integrity

## Findings

### 1. An unwritable config directory fails the mutation with nothing in the log

**Severity**: Important
**Plan Reference**: Phase 5, task `resume-hooks-silently-lost-5-1` (with consequences in `resume-hooks-silently-lost-5-3`)
**Category**: Task Self-Containment / Acceptance Criteria Quality
**Move**: settled
**Change Type**: add-to-task

**Problem**:
Today, when `~/.config/portal/` cannot be written, `hook set` fails loudly and leaves a record: the store emits its WARN under `op=set` carrying `hook_key`, `via`, `error` and `error_class=write-failed-temp-create`, because the failure happens inside `AtomicWrite`. Task 5-1 puts `os.MkdirAll(filepath.Dir(s.path), 0o755)` in front of every mutation and says nothing about what to do with its error. The obvious reading — return it — moves that whole failure class in front of the store's only emission point, so an unwritable config directory becomes a bare error return with no log line at all. That is quieter than the code the work unit started from, on the very failure it exists to make audible.

It also makes a later acceptance criterion unreachable. Task 5-3 requires that "a sidecar that cannot be opened or created at all takes the same write-side branch: the mutation fails, nothing is written, one WARN is emitted" — but 5-3 hangs that WARN on `acquireLock` returning an error, which never runs if `MkdirAll` returned first. An implementer working 5-1 in isolation has no way to know that, and the write-failure fixtures 5-1 amends all use a directory that already exists, so nothing catches it.

**Proposal**:
State in 5-1 that `MkdirAll`'s error is discarded and the acquire that follows carries the failure. A directory that could not be created is a sidecar that cannot be opened, so the two conditions collapse to one branch with one emission point — which is exactly the shape 5-3 already assumes. There is no case where `MkdirAll` fails and the acquire then succeeds: `MkdirAll` returns nil for an existing directory, so any failure leaves the sidecar's `O_CREAT` open with nowhere to land.

**Current**:
```markdown
- Split the file access in `internal/hooks/store.go`: move today's `Load` body into an unexported non-locking `load()` and today's `Save` body into an unexported non-locking `save(h hooksFile) error`, and have `Set`, `Remove` and `CleanStale` call **only** those two — never the exported `Load`/`Save`. Each of the three now begins with `os.MkdirAll(filepath.Dir(s.path), 0o755)`, then `acquireLock(s.lockPath(), os.O_RDWR|os.O_CREATE, unix.LOCK_EX)` with `defer f.Close()`, and does its whole load-mutate-save inside that hold — including `Set`'s `set-noop` arm, `Remove`'s no-removal arm (task 4-1) and every failed-save return. Carry one line of comment on the `MkdirAll`-before-acquire ordering, wording left to the executor.
```

and, in the same task's Edge Cases:

```markdown
- The directory is created **before** acquisition: the sidecar cannot be created inside a directory that does not exist, so acquiring first would make the very first `hook set` on a fresh machine fail permanently. `MkdirAll` is idempotent, races benignly, and mutates nothing the lock protects
```

**Proposed Text**:
```markdown
- Split the file access in `internal/hooks/store.go`: move today's `Load` body into an unexported non-locking `load()` and today's `Save` body into an unexported non-locking `save(h hooksFile) error`, and have `Set`, `Remove` and `CleanStale` call **only** those two — never the exported `Load`/`Save`. Each of the three now begins with `os.MkdirAll(filepath.Dir(s.path), 0o755)`, then `acquireLock(s.lockPath(), os.O_RDWR|os.O_CREATE, unix.LOCK_EX)` with `defer f.Close()`, and does its whole load-mutate-save inside that hold — including `Set`'s `set-noop` arm, `Remove`'s no-removal arm (task 4-1) and every failed-save return. Carry one line of comment on the `MkdirAll`-before-acquire ordering, wording left to the executor.
- **Discard `MkdirAll`'s error rather than returning it**, and let the acquire that follows fail on the same condition. A directory that could not be created is a sidecar that cannot be opened, so the two collapse to one branch — and `acquireLock` is the single point task 5-3 attaches its WARN to. Returning `MkdirAll`'s error directly would give an unwritable config directory a return path with no log record at all, quieter than today, where the same condition surfaces as the write WARN carrying `error_class=write-failed-temp-create`. There is no case where the discard hides a distinct outcome: `MkdirAll` returns nil for a directory that already exists, so any failure it reports leaves the sidecar's `O_CREAT` with nowhere to land.
```

and, in the same task's Edge Cases:

```markdown
- The directory is created **before** acquisition: the sidecar cannot be created inside a directory that does not exist, so acquiring first would make the very first `hook set` on a fresh machine fail permanently. `MkdirAll` is idempotent, races benignly, and mutates nothing the lock protects
- `MkdirAll`'s error is discarded, not returned: an uncreatable directory must fail through the acquire so directory failure and sidecar failure share one branch and one emission point. The amended write-failure fixtures all run against a directory that already exists, so nothing else in the suite covers this
```

and, in the same task's Acceptance Criteria:

```markdown
- [ ] A mutation whose config directory cannot be created fails through `acquireLock` rather than through a bare `MkdirAll` return, so directory failure and sidecar failure are one branch
```

and, in the same task's Tests:

```markdown
- `"it fails through the sidecar acquire when the config directory cannot be created"` — a store path under a directory whose parent denies creation; assert the returned error is the acquisition failure, that `hooks.json` is absent, and that the same call would reach the emission point task 5-3 adds
```

**Resolution**: Pending
**Notes**:

---

### 2. The daemon creates a lock file every ten seconds on installs that have no hooks

**Severity**: Minor
**Plan Reference**: Phase 5, task `resume-hooks-silently-lost-5-4`
**Category**: Scope and Granularity / Acceptance Criteria Quality
**Move**: settled
**Change Type**: add-to-task

**Problem**:
`runHookStaleCleanup` reaches `CleanStale` whenever the pane enumeration returns rows, regardless of how many entries are persisted — the zero-entry short-circuit that exists today sits inside the empty-live-set branch and never fires on the ordinary path. Before this work unit that was free: `CleanStale` loaded a missing file into an empty map, found nothing to delete and returned without writing. After task 5-1 it is not free. Every call now runs `MkdirAll` on the config directory, creates `<hooks.json path>.lock` with `O_CREAT` and takes an exclusive `flock`.

So on any install where the user has never registered a resume hook — no `hooks.json` at all — the daemon will create `hooks.json.lock` beside a file that does not exist, and re-take an exclusive hold on it every ten seconds for as long as Portal runs. The hold is brief, but it is the one thing that can block `hook set`, which is the interactive command this work unit exists to protect, and it can produce a `reason=lock-timeout` WARN for a sweep that had nothing to sweep. Task 5-4's Edge Cases record this case as "unchanged in shape from today", which was true of the pre-lock `CleanStale` and stopped being true when 5-1 landed.

Task 5-2 goes to some length to guarantee the mirror property on the read side — "no read ever creates the config directory", so `portal doctor` and `hook list` "keep having no write side effect at all on a fresh install". The sweep is the far higher-frequency caller and gets no equivalent.

**Proposal**:
Short-circuit in `runHookStaleCleanup` immediately before `CleanStale` when the snapshot holds no keys. Nothing can be stale when nothing is persisted, so the call has no work to do and its only effects are the directory, the lock file and the hold. Placing the return just before `CleanStale` — rather than earlier — leaves the enumeration, the counts DEBUG line and every existing branch untouched, so the only observable difference is the absence of the `reaped=0` DEBUG line in that one case. Task 5-4 is the right home: it already owns the reorder that puts the snapshot ahead of everything else, so the key set is in hand at that point.

**Current**:
```markdown
- Keep the zero-persisted-entries silent early return inside the empty-live-set branch exactly as it is, and keep the enumeration-error branch's existing WARN-and-return-nil.
```

and, in the same task's Edge Cases:

```markdown
- The zero-persisted-entries early return still short-circuits. A zero-entry snapshot with a non-empty live set still reaches `CleanStale`, which takes the lock, finds nothing to delete and writes nothing — unchanged in shape from today
```

**Proposed Text**:
```markdown
- Keep the zero-persisted-entries silent early return inside the empty-live-set branch exactly as it is, and keep the enumeration-error branch's existing WARN-and-return-nil.
- Add one further silent early return immediately before `CleanStale`: when the snapshot holds no keys, return nil without calling it. Nothing can be stale when nothing is persisted, and after task 5-1 the call is no longer free — it creates the config directory, creates `<hooks.json path>.lock` and takes an exclusive hold. Without the return, an install that has never registered a hook gets a lock file beside a `hooks.json` that does not exist and a pointless exclusive hold in `hook set`'s way every ten seconds, for a sweep with nothing to sweep. Place it immediately before `CleanStale` and nowhere earlier, so the enumeration, the counts DEBUG line and every existing branch are untouched; the only thing lost is the `reaped=0` DEBUG line for that case.
```

and, in the same task's Edge Cases:

```markdown
- The zero-persisted-entries early return inside the empty-live-set branch still short-circuits. A zero-entry snapshot with a non-empty live set now short-circuits too, immediately before `CleanStale` — after task 5-1 that call creates the config directory and the sidecar and takes an exclusive hold, which the daemon would otherwise repeat every ten seconds on an install with no hooks at all
```

and, in the same task's Acceptance Criteria:

```markdown
- [ ] A cycle whose snapshot holds no keys returns before `CleanStale` is called: no lock is taken, no config directory is created and no `<hooks.json path>.lock` appears
```

and, in the same task's Tests:

```markdown
- `"it takes no lock when nothing is persisted"` — live rows present, no `hooks.json` and no config directory; assert the sweep returns nil and that neither the directory nor the sidecar exists afterwards
```

**Resolution**: Pending
**Notes**:

---

### 3. The reboot test calls the sweep with the wrong number of arguments

**Severity**: Minor
**Plan Reference**: Phase 3, task `resume-hooks-silently-lost-3-5`
**Category**: Dependencies and Ordering / Task Self-Containment
**Move**: settled
**Change Type**: update-task

**Problem**:
Task 3-5 directs the test to run `runHookStaleCleanup(client, store, nil, nil)`. That is the function's shape today, but task 1-4 adds an `onSkipped` parameter to it, and 1-4 lands three phases earlier. By the time 3-5 is written the call takes five arguments, so the directed call does not compile. The implementer will fix it in seconds, but the instruction as written sends them to a wrong line first, and 3-5 is the one task in the plan whose whole value is that its assertions are exactly right.

**Proposal**:
Name the five-parameter form and what each nil stands for. 1-4's added parameter is nil-safe by its own acceptance criteria, so nil is the correct value for the test's purposes on all three.

**Current**:
```markdown
- Assert, in order: the restored window indices **differ** from the saved ones (fail the test outright if they match — a restore that happened not to renumber proves nothing); each stamped pane's own marker file exists with exactly its own payload and no other; the unstamped pane is live, hydrated and fired nothing; `@portal-restoring` is unset before the sweep runs; and after `runHookStaleCleanup(client, store, nil, nil)` both token-keyed entries survive while the seeded stale key is gone.
```

**Proposed Text**:
```markdown
- Assert, in order: the restored window indices **differ** from the saved ones (fail the test outright if they match — a restore that happened not to renumber proves nothing); each stamped pane's own marker file exists with exactly its own payload and no other; the unstamped pane is live, hydrated and fired nothing; `@portal-restoring` is unset before the sweep runs; and after `runHookStaleCleanup(client, store, nil, nil, nil)` — the five-parameter form task 1-4 established, with nil for the logger, `onRemoved` and `onSkipped`, all three of which are nil-safe — both token-keyed entries survive while the seeded stale key is gone.
```

**Resolution**: Pending
**Notes**:

---
