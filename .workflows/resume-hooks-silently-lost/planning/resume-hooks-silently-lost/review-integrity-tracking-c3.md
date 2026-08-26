# Review Tracking: Resume Hooks Silently Lost - Integrity

## Findings

### 1. A wedged writer parks the daemon for twice the bound the design allows

**Severity**: Important
**Plan Reference**: Phase 5, tasks `resume-hooks-silently-lost-5-2`, `resume-hooks-silently-lost-5-4` and `resume-hooks-silently-lost-5-5`
**Category**: Acceptance Criteria Quality / Task Self-Containment
**Move**: choice
**Change Type**: update-task

**Problem**:
After tasks 5-2 and 5-4 the stale sweep takes the sidecar lock **twice** in one cycle: a bounded shared acquire for the call-site snapshot (`store.Load("internal")`, routed through `loadShared`), then a bounded exclusive acquire inside `CleanStale`. Both use the same package-level bound, and 5-2 fixes that a read blocked by a held exclusive lock "returns its data anyway, **after the bound**, rather than failing".

So under the one condition the bound exists for — a writer holding the sidecar and not letting go — a single sweep cycle burns the full bound waiting for the shared lock, then the full bound again waiting for the exclusive one, and only then stands down. `maybeRunHookCleanup` runs synchronously inside the daemon's `tick`, so that is roughly **4 seconds of a 1-second tick loop parked, every 10 seconds**, for as long as the holder stays wedged — and the capture cycle is what stalls, not just the prune.

That is worse than the outcome the plan says the design prevents, and three places in the plan say so:

- 5-2's Edge Cases name "a 2s stall on the daemon's 1s tick loop every ten seconds" as "the outcome the bound exists to prevent".
- 5-1's Context calls the blocking path "the single riskiest part of this work unit" and notes "the project has had a wedged daemon before".
- 5-5's acceptance requires that "the cycle returns at the bound rather than waiting longer, so the daemon's 1s tick is not parked".

Nothing in the plan reckons with the two acquisitions adding up, and nothing would catch it: 5-5's test for that criterion asserts only "elapsed time is within a small multiple of the lowered bound", which a 2× cycle satisfies. An implementer following the plan as written ships the 2× behaviour and has no way to tell whether it was intended.

The three ways out differ in what they cost — one adds surface to `acquireLock`, one relaxes a stated guarantee, one reopens a rule 5-2 sets deliberately — so the pick is yours.

**Options**:
- Give the shared acquire a caller-supplied bound and have `runHookStaleCleanup` pass a near-zero one for its snapshot, so the pre-read degrades immediately under contention and the cycle's worst case stays a single bound, with the read still routed through `loadShared` and still emitting its `op=load-unlocked` DEBUG (recommended)
- Accept two bounds per sweep cycle and say so in the plan: correct 5-5's acceptance criterion and its `"it returns at the bound"` test to name twice the bound, and correct 5-2's "the outcome the bound exists to prevent" reasoning so the stated ceiling matches what is built
- Drop the shared lock from the sweep's pre-read altogether on the grounds the plan already gives — correctness never depends on it and it is "an ordering courtesy" — accepting that this reintroduces an unlocked exported read path, which 5-2 forbids for `Store.Get` in the same breath

**Resolution**: Pending
**Notes**:

---

### 2. Two phase gates check a list that no longer names what the phase delivers

**Severity**: Minor
**Plan Reference**: `planning.md` — Phase 4 and Phase 5 **Acceptance** blocks
**Category**: Phase Structure / Acceptance Criteria Quality
**Move**: settled
**Change Type**: update-task

**Problem**:
Three decisions accepted in earlier review cycles were applied to the phase task files but never carried back into `planning.md`, so the phase acceptance lists — the checklists a reader works through at a gate — no longer describe what the phase ships:

- **Phase 4** is where the shipped user documentation gets corrected: `hook rm --pane-key`'s flag help stops saying "Structural key", and README's `xctl hook` section drops its `--pane-key 'sess:0.1'` example and warns that `hook rm` now exits non-zero when it removes nothing. Task 4-2 carries all of it. Phase 4's acceptance list mentions none of it, so a gate can pass with the README still teaching a key shape nothing can write and a breaking exit-code change undocumented.
- **Phase 5** gained two properties in cycle 2: a sweep whose snapshot holds no keys returns before `CleanStale`, so an install that has never registered a hook grows no `hooks.json.lock`; and an uncreatable config directory fails through the sidecar acquire rather than a bare `MkdirAll` return, so it lands in the log instead of returning silently. Tasks 5-4 and 5-1 carry both. Phase 5's acceptance list mentions neither — including the on-disk one, a new file appearing beside a `hooks.json` that does not exist.

The same drift runs through the task tables' Edge Cases column, which is a digest of each task's Edge Cases and is missing the same three items. That half is left alone deliberately: an implementer reads the task file, which is correct, so rewriting three very long table cells buys nothing. The acceptance lists are different — they are the only thing a reader checks a finished phase against.

**Proposal**:
Add the missing properties to the Phase 4 and Phase 5 acceptance lists, worded from the task files that already carry them (4-2's documentation criteria, 5-4's no-lock criterion, 5-1's one-branch criterion).

**Current**:

Phase 4's acceptance block:

```markdown
**Acceptance**:
- [ ] `hook rm` exits 0 iff an entry was removed, with the answer coming from the store's own removal rather than from a read taken before it
- [ ] The three no-removal cases exit non-zero with their fixed words — tmux's own words for a gone pane, `no resume hook registered for this pane` for a live pane carrying no token, `no resume hook registered for <key>` for a key naming no entry — each leaving `hooks.json` byte-identical
- [ ] `hook rm --pane-key <key>` validates nothing and issues no tmux call: it removes a seeded key and exits 0, and exits non-zero when the key names no entry
- [ ] `hook list` appends a fourth tab-separated `<session>:<window>.<pane>` column resolved from one enumeration read, mapped from non-empty tokens only, leaving the existing three field positions undisturbed
- [ ] A token no live pane carries — including with no server running, and any old-format key — renders an empty fourth field, never a dropped one
```

Phase 5's acceptance block:

```markdown
**Acceptance**:
- [ ] Every mutation holds an exclusive `flock` on `<hooks.json path>.lock` across its whole read-modify-write; reads take a shared lock; the config directory is created before acquisition and the lock file is never unlinked
- [ ] Locks are never nested: the exported methods reach the file through unexported non-locking load and save helpers, and a read's shared lock is released before the read returns
- [ ] Interleaved writers across the `Load`→`AtomicWrite` window lose no entry, demonstrated across the rename that swaps the inode
- [ ] `CleanStale` derives the delete set under its own exclusive lock from the live token set and the call-site snapshot, and the snapshot is taken before the pane enumeration, so an entry registered after it survives the sweep
- [ ] No tmux call is made while any lock is held
- [ ] Acquisition is bounded at 2s through a package-level value the unit lane can lower; on timeout `hook set` and `hook rm` exit non-zero and write nothing, the sweep skips with `op=clean-stale-skipped reason=lock-timeout` and `doctor --fix` names the skipped prune without affecting its exit code, while `LookupOnResume`, `checkStaleHooks` and `hook list` return their data and log `op=load-unlocked` at DEBUG with the correct `via`
- [ ] The degraded read adds the only new `op` value this phase introduces, with no new attr key and no new log component
```

**Proposed Text**:

Phase 4's acceptance block:

```markdown
**Acceptance**:
- [ ] `hook rm` exits 0 iff an entry was removed, with the answer coming from the store's own removal rather than from a read taken before it
- [ ] The three no-removal cases exit non-zero with their fixed words — tmux's own words for a gone pane, `no resume hook registered for this pane` for a live pane carrying no token, `no resume hook registered for <key>` for a key naming no entry — each leaving `hooks.json` byte-identical
- [ ] `hook rm --pane-key <key>` validates nothing and issues no tmux call: it removes a seeded key and exits 0, and exits non-zero when the key names no entry
- [ ] `hook list` appends a fourth tab-separated `<session>:<window>.<pane>` column resolved from one enumeration read, mapped from non-empty tokens only, leaving the existing three field positions undisturbed
- [ ] A token no live pane carries — including with no server running, and any old-format key — renders an empty fourth field, never a dropped one
- [ ] The shipped user-facing texts stop teaching the retired key form: `hook rm --pane-key`'s flag help names the pane token, README's `xctl hook` section carries no positional `--pane-key` example, and that section states that `hook rm` exits non-zero when it removes nothing; no other README passage is edited
```

Phase 5's acceptance block:

```markdown
**Acceptance**:
- [ ] Every mutation holds an exclusive `flock` on `<hooks.json path>.lock` across its whole read-modify-write; reads take a shared lock; the config directory is created before acquisition and the lock file is never unlinked
- [ ] Locks are never nested: the exported methods reach the file through unexported non-locking load and save helpers, and a read's shared lock is released before the read returns
- [ ] Interleaved writers across the `Load`→`AtomicWrite` window lose no entry, demonstrated across the rename that swaps the inode
- [ ] `CleanStale` derives the delete set under its own exclusive lock from the live token set and the call-site snapshot, and the snapshot is taken before the pane enumeration, so an entry registered after it survives the sweep
- [ ] A sweep whose snapshot holds no keys returns before `CleanStale` is called, so an install that has never registered a hook grows neither a config directory nor a `<hooks.json path>.lock`
- [ ] A mutation whose config directory cannot be created fails through the sidecar acquire rather than through a bare `MkdirAll` return, so directory failure and sidecar failure are one branch with one WARN
- [ ] No tmux call is made while any lock is held
- [ ] Acquisition is bounded at 2s through a package-level value the unit lane can lower; on timeout `hook set` and `hook rm` exit non-zero and write nothing, the sweep skips with `op=clean-stale-skipped reason=lock-timeout` and `doctor --fix` names the skipped prune without affecting its exit code, while `LookupOnResume`, `checkStaleHooks` and `hook list` return their data and log `op=load-unlocked` at DEBUG with the correct `via`
- [ ] The degraded read adds the only new `op` value this phase introduces, with no new attr key and no new log component
```

**Resolution**: Pending
**Notes**:

---
