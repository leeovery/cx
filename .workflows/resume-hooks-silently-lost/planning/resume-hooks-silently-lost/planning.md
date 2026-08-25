# Plan: Resume Hooks Silently Lost

## Phases

### Phase 1: Make the reaper shape-aware and audible
status: draft

**Goal**: The stale sweep stops deleting keys it cannot judge, stands down for the duration of a restore, and names every entry it removes.

**Why this order**: This is the precondition for changing the key format at all — the moment keys become tokens, every entry in `hooks.json` is unmatched by the live set and today's reaper deletes all of them within ~10s, which is the outcome the specification explicitly rejects for the migration. Retention has to be in place before the format flips. It also closes the silence half of the defect on its own, and it depends on nothing from any later phase.

**Acceptance**:
- [ ] A non-token-shaped key is retained by both the daemon sweep and `portal doctor --fix`, on every run
- [ ] A token-shaped key whose token is absent from the live set is still deleted; an empty key is deleted
- [ ] The staleness rule has one implementation: `StaleKeys` and `CleanStale` both apply the same unexported function, and `CleanStale` never routes through `StaleKeys`
- [ ] Each removed key is logged once at INFO with `hook_key` and the removed entry's command in `value`; the per-key DEBUG line is promoted rather than duplicated; the batch summary is retained
- [ ] With `@portal-restoring` set — or its read failing — `runHookStaleCleanup` deletes nothing and logs `op=clean-stale-skipped reason=restoring` at DEBUG, and `checkStaleHooks` reports not-evaluable instead of counting
- [ ] `portal doctor --fix` prints `Skipped stale hook prune: restore in progress` through the new `onSkipped` callback, with the exit code still driven solely by the post-repair diagnosis
- [ ] `portal doctor` exits 0 with retained non-token-shaped entries present
- [ ] The shape predicate lives in `internal/session` and derives its width and alphabet from `suffixLen` / `NanoIDAlphabet` rather than restating them

### Phase 2: The pane token becomes the hook key
status: draft

**Goal**: A hook key is a pane's `@portal-pane-id` token and nothing else, minted and stamped lazily at `hook set`, with registration refusing a `$TMUX_PANE` that names no live pane.

**Why this order**: This is the repair for the two moments a positional key creates, and it lands on Phase 1's retention rule so the existing on-disk entries survive the format change. The key format and the all-pane enumeration must flip in the same phase — a token-keyed entry written against a positional enumeration is stale the instant it is written.

**Acceptance**:
- [ ] `@portal-pane-id` is written as a literal in exactly one place (`state.PortalPaneIDOption`); every other site composes it from that constant
- [ ] `hook set` in a live pane stamps a minted token, or reuses an existing one and issues no `set-option`, and writes the entry under the token alone
- [ ] `hook set` against a `$TMUX_PANE` that names no live pane exits non-zero with tmux's own words and leaves `hooks.json` byte-identical
- [ ] Against a real tmux server, the `show-options -p` probe that names no option separates gone / live-unstamped / live-stamped, and `set-option -p` against a bogus target exits non-zero where `display-message -p` exits 0
- [ ] A hook registered on a pane still resolves after `break-pane`, a `kill-window` under `renumber-windows on`, `move-pane` back, and `move-pane` into another session
- [ ] The all-pane enumeration returns one row per live pane carrying its token (empty when unstamped) and its location; the mass-deletion guard counts rows, and a server with hooks present and zero stamped panes emits no guard WARN
- [ ] A failed `save.requested` touch logs one WARN under `op=touch-save-requested` and `hook set` still exits 0 with the entry written
- [ ] CLAUDE.md's hook-key passages describe the pane token rather than `<@portal-id or session_name>:window.pane`

### Phase 3: Carry the token across the reboot gap, and retire `@portal-id`
status: draft

**Goal**: The pane token survives the tmux server dying — captured into `sessions.json` and re-stamped by restore before each pane is armed — and the superseded `@portal-id` machinery is removed in the same edits.

**Why this order**: It must follow Phase 2 immediately: until the token is captured and re-stamped, a reboot leaves no live pane answering to any key. The capture column swap and the `@portal-id` removal are the same edits, and `tmux.HookKey` can only be deleted once `collectArmInfos` reads the saved token instead — so replacement and removal cannot be separated into different phases.

**Acceptance**:
- [ ] `state.Pane` carries `portal_pane_id`; `captureFormat`'s trailing column is `#{@portal-pane-id}` lifted per-pane; `captureFieldCount` stays 11 and `SchemaVersion` is unchanged
- [ ] Restore re-stamps each saved token before arming, skips a saved pane whose token is empty without writing or logging anything, and stamps only panes within `pairCount`
- [ ] A failed pane re-stamp logs one WARN under the `restore` component (`set pane token failed`, carrying `session` / `pane_key` / `error`) and does not abort the restore
- [ ] A session whose saved window indices are non-contiguous, restored under `renumber-windows off`, fires its hook and the entry survives the post-restore sweep
- [ ] `collectArmInfos` bakes `p.PortalPaneID` and omits the `--hook-key` flag entirely when it is empty; `LookupOnResume` returns no hook for an empty key before the map is consulted
- [ ] No `PortalIDOption`, `@portal-id` stamp, `Session.PortalID`, `tmux.HookKey` or `portal state migrate-rename` remains; `migrateRenameSubstring` and its comment stay
- [ ] Both `@portal-id` literal-binding guard tests are deleted rather than re-pointed
- [ ] CLAUDE.md describes neither `@portal-id`, `Session.PortalID` nor the `#{@portal-id}` capture column

### Phase 4: Removal and listing report the truth
status: draft

**Goal**: `portal hook rm` exits 0 iff it removed an entry, by all three routes and on both paths, and `portal hook list` renders where each token lives.

**Why this order**: Both are the payback halves of the earlier phases — removal mirrors registration's guard, listing consumes the enumeration Phase 2 introduced — and nothing depends on either, so they follow the durable identity rather than blocking it.

**Acceptance**:
- [ ] `hook rm` exits 0 iff an entry was removed, with the answer coming from the store's own removal rather than from a read taken before it
- [ ] The three no-removal cases exit non-zero with their fixed words — tmux's own words for a gone pane, `no resume hook registered for this pane` for a live pane carrying no token, `no resume hook registered for <key>` for a key naming no entry — each leaving `hooks.json` byte-identical
- [ ] `hook rm --pane-key <key>` validates nothing and issues no tmux call: it removes a seeded key and exits 0, and exits non-zero when the key names no entry
- [ ] `hook list` appends a fourth tab-separated `<session>:<window>.<pane>` column resolved from one enumeration read, mapped from non-empty tokens only, leaving the existing three field positions undisturbed
- [ ] A token no live pane carries — including with no server running, and any old-format key — renders an empty fourth field, never a dropped one

### Phase 5: Serialise `hooks.json` access
status: draft

**Goal**: The unlocked cross-process read-modify-write on `hooks.json` is closed by a bounded sidecar lock that degrades by side rather than wedging the daemon.

**Why this order**: Last because it is independent of the key format and blocks nothing else, and because it is the highest-risk surface in the unit — an unbounded acquire would park the daemon's 1s tick loop. Landing it against a settled `internal/hooks` surface means the locking discipline is written once, over rules that are already final.

**Acceptance**:
- [ ] Every mutation holds an exclusive `flock` on `<hooks.json path>.lock` across its whole read-modify-write; reads take a shared lock; the config directory is created before acquisition and the lock file is never unlinked
- [ ] Locks are never nested: the exported methods reach the file through unexported non-locking load and save helpers, and a read's shared lock is released before the read returns
- [ ] Interleaved writers across the `Load`→`AtomicWrite` window lose no entry, demonstrated across the rename that swaps the inode
- [ ] `CleanStale` derives the delete set under its own exclusive lock from the live token set and the call-site snapshot, and the snapshot is taken before the pane enumeration, so an entry registered after it survives the sweep
- [ ] No tmux call is made while any lock is held
- [ ] Acquisition is bounded at 2s through a package-level value the unit lane can lower; on timeout `hook set` and `hook rm` exit non-zero and write nothing, the sweep skips with `op=clean-stale-skipped reason=lock-timeout` and `doctor --fix` names the skipped prune without affecting its exit code, while `LookupOnResume`, `checkStaleHooks` and `hook list` return their data and log `op=load-unlocked` at DEBUG with the correct `via`
- [ ] The degraded read adds the only new `op` value this phase introduces, with no new attr key and no new log component
