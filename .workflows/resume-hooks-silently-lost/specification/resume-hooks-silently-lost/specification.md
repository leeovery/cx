# Specification: Resume Hooks Silently Lost

## Specification

### 1. Defect and Scope

#### 1.1 The defect

A resume hook is stored in `hooks.json` under a key that is half durable identity and half tmux-recomputed position (`internal/tmux/tmux.go:565`):

```
#{?@portal-id,#{@portal-id},#{session_name}}:#{window_index}.#{pane_index}
└──────────── immutable identity ───────────┘ └──── mutable position ────┘
```

The session half was made rename-immune by spec `session-rename-orphans-resume-hook` (2026-07-04). The pane half is a coordinate tmux recomputes from layout, and Portal never verifies that a key it writes down identifies any pane at all. That is one defect, and it surfaces at three moments in a hook's life:

- **At write time.** tmux exits 0 and prints `:.` for a `display-message -p` against a pane that does not exist (`tmux -L <sock> display-message -p -t %999 '#{?@portal-id,#{@portal-id},#{session_name}}:#{window_index}.#{pane_index}'` → `:.`, exit 0; tmux 3.7c). `tmux.ResolveHookKey` returns that verbatim, `cmd/hooks.go` `resolveCurrentPaneKey` passes it through, `hooks.Store.Set` persists it, and `portal hook set` exits 0. No layer validates the key's shape — there is no such validation anywhere in the repo.
- **Over the pane's lifetime.** Any rearrangement — closing a sibling pane, closing a window under `renumber-windows on`, `break-pane`, `move-pane` — changes `window_index.pane_index`. Nothing re-keys the entry, and the daemon's stale sweep removes it. The sweep is correct by its own rule: at the point of comparison a moved pane and a dead pane are indistinguishable. The sweep runs on the daemon's idle branch at `hookCleanupInterval = 10 * time.Second` (`cmd/state_daemon.go:105`) — precisely when a user is rearranging rather than working — so the entry is gone within ~10s of the move.
- **At the reboot boundary.** Restore recreates each extra window with `NewWindow(target, …)` where `target` is `"<session>:"` and no index is passed (`internal/restore/session.go:98`), so tmux assigns the next free index. Where the saved window indices are non-contiguous, the restored ones differ. The baked key still fires — `collectArmInfos` derives it purely from saved state — but no live pane answers to it afterwards and the sweep reaps it. The hook survives exactly one reboot. This is latent on the reporting user's install, whose `renumber-windows on` keeps saved and restored indices contiguous; it is a general defect on tmux's default settings.

The loss is silent because both diagnostics ask the reaper's own question and the reaper does not record its answer: `portal doctor`'s `checkStaleHooks` (`cmd/doctor.go:280`) tests the same live-key rule the daemon acted on within 10s, so it reports `no stale hooks` *because* the deletion completed; and the automatic prune logs the removed key only at DEBUG (`internal/hooks/store.go:220`), with production INFO getting the batch count alone.

#### 1.2 What this work unit fixes

Four changes, delivered together in one release:

| | Change | Home |
|---|---|---|
| **A** | A durable per-pane identity replaces the positional pane component of the key | §2, §3 |
| **B** | Registration verifies the pane exists and fails loudly when it does not | §4 |
| **C** | The stale sweep becomes shape-aware and names what it deleted | §5 |
| **D** | The unlocked cross-process read-modify-write on `hooks.json` is closed | §6 |

A is the repair — it closes all three moments above. B, C and D close paths that made the loss silent or that can lose an entry independently of the key format. A carries the removal of the superseded `@portal-id` machinery with it (§7).

#### 1.3 Out of scope

- **`~/.claude/hooks/portal-resume-hook.sh`** — the user's own Claude Code integration. It re-implements `HookKeyFormat` verbatim and matches it against `portal hook list` output (`:87-95`), so it needs updating in step with this change, but it is not part of the Portal product and no work item here covers it.
- **The one-time conversion of the existing `hooks.json`** — no migration code ships; see §8.
- **Claude Code transcript retention.** The 2026-06-29 sighting — sessions present after a crash-recovery reboot with resume behaviour absent — is explained by Claude Code garbage-collecting session transcripts on a retention window, after which the stored `claude --resume <id>` fails and the hydrate chain `sh -c '<HOOK>; exec $SHELL'` falls through to a bare shell. Closed at source by raising that retention. Not a Portal defect and no fix work here addresses it.
- **The inverse diagnostic** — "a live pane that should have a hook does not have one". Portal records no notion of which panes are meant to be protected, so an unprotected pane is invisible to every check. A real diagnostic gap, but not part of this causal chain.
- **The positional siblings that share the pattern but are not failing** — `state.SanitizePaneKey` (hydrate FIFO paths, `@portal-skeleton-*` markers) and `internal/tmux`'s `StructuralKeyFormat` / `ResolveStructuralKey` / `ListAllPanes`. These live for the duration of one bootstrap and are rebuilt from live coordinates each time, so drift has no window in which to occur. They are checked against the change (§9), not changed by it.



### 2. Durable Pane Identity

#### 2.1 The token and its tmux option

Each pane that carries a resume hook is stamped with an opaque token held in the tmux **pane** user-option `@portal-pane-id`.

A tmux pane user-option was verified in an isolated `-L` socket (tmux 3.7c) to survive every mutation that moves a pane's coordinates, and to survive the one Portal itself performs:

| Operation | Coordinates before → after | Stamp |
|---|---|---|
| initial | `t:1.2` | `STAMP1` |
| `break-pane` | `t:1.2` → `t:3.1` | survives |
| `kill-window` under `renumber-windows on` | `t:3.1` → `t:2.1` | survives |
| `move-pane` back | `t:2.1` → `t:1.2` | survives |
| `respawn-pane -k` | `t:1.2` (unchanged) | survives |
| `rename-session` | `t:1.2` → `newname:1.2` | survives |

(`tmux -L <sock> set-option -p -t <pane> @portal-pane-id STAMP1`, then `tmux -L <sock> display-message -p -t <pane> '#{@portal-pane-id}'` after each operation.)

`respawn-pane -k` surviving is load-bearing: that is exactly what restore's arm phase does to every pane (`internal/restore/session.go` `armPanes`), so a stamp applied before arming is still there afterwards.

There is **no inheritance**: a pane created by `split-window` or `new-window` from a stamped pane reads back empty (`tmux -L <sock> split-window -t t -d`, then `tmux -L <sock> list-panes -t t -F '#{pane_index}=#{@portal-pane-id}'` → `0=STAMP1 1=`). A split therefore cannot duplicate an id.

**Token generation** reuses the in-house nanoid: `session.NewNanoIDGenerator()` over `session.NanoIDAlphabet` (62 alphanumerics, no `-`), width `suffixLen = 6`. There is no uniqueness check beyond the generator's width — the same call the `@portal-id` stamp makes today (`internal/session/create.go`), which that code documents as deliberate.

#### 2.2 Stamping is lazy, at `hook set`

Portal does not create panes — the user splits them — so there is no creation point to stamp at, and a pane with no hook needs no token. The stamp is therefore applied by `portal hook set`, immediately before the `hooks.json` write.

- **A pane already carrying a token keeps it.** `hook set` reads `@portal-pane-id` first and mints only when it reads back empty. Re-registering on the same pane must not mint a second token, which would orphan the first entry.
- **The stamp precedes the write.** Under B (§4) the stamp doubles as the existence check, so ordering is not optional.
- **`hook set` touches the dirty flag.** After a successful stamp it calls `state.TouchSaveRequested(<state dir>)` so the next daemon tick captures the new token rather than waiting up to `MaxGap` (30s, `cmd/state_daemon.go:425`) for the gap branch. This narrows the window in which a crash between registration and capture loses the stamp; it does not close it, and that residual is accepted.
- **`hook` stays bootstrap-exempt.** It starts no tmux server. A `$TMUX_PANE` that is set implies a live server, and the `--pane-key` path (§4.3) touches tmux not at all.

#### 2.3 Carrying the token across the reboot gap

A tmux pane option does not survive the server dying — the exact constraint that made spec `resume-hooks-lost-on-server-restart` (2026-04-30) choose positional keys over `%N` pane ids. That gap is closed the same way the session half already closes it:

- **Schema.** `state.Pane` gains `PortalPaneID string \`json:"portal_pane_id"\`` (`internal/state/schema.go`). This is an additive field on a tolerant-decode struct: **no `SchemaVersion` bump, no migration**. An older `sessions.json` decodes with the field empty.
- **Capture.** `captureFormat` (`internal/state/capture.go:26`) has its trailing `#{@portal-id}` column **replaced** by `#{@portal-pane-id}`, and the value is lifted into `Pane.PortalPaneID` rather than `Session.PortalID`. `captureFieldCount` stays `11` — one column swaps for another, and it changes from session-scoped (repeated on every pane row of a session) to genuinely per-pane.
- **Restore.** After the skeleton exists and before each pane is armed, the saved token is re-stamped onto the corresponding live pane with `set-option -p`. **A saved pane whose `PortalPaneID` is empty is skipped entirely** — no option is written and nothing is logged. Under lazy stamping (§2.2) an unstamped pane is the ordinary case rather than an anomaly: it is every pane that has never carried a hook, and every pane on the install during the upgrade window. Writing `@portal-pane-id ""` instead would stamp a value indistinguishable on read-back from absence onto most panes on the machine.

#### 2.4 Restore re-stamp: failures are surfaced, mispairings are not stamped

Two rules distinguish the pane re-stamp from the session re-stamp it is modelled on.

**A failed re-stamp must not be swallowed.** The session stamp discards its error (`_ = r.Client.SetSessionOption(...)`, `internal/restore/session.go:79`) because a missed `@portal-id` costs only rename-immunity. For the pane token the stamp *is* the identity: a swallowed failure permanently orphans that pane's hook with no trace. A failed pane re-stamp logs at WARN under the `restore` component, naming the session and pane. It does not abort the restore — an unstamped pane is a lost hook, not a lost session. Because an empty saved token never reaches the stamp, this WARN can only fire for a genuine tmux failure — it is a signal that one pane's identity was lost, not a per-boot report on the unstamped majority.

**An unpaired pane must not be stamped.** `armPanes` (`internal/restore/session.go:125-128`) warns and pairs up to the shorter list when the live and saved pane counts differ. Today a mispairing misplaces a FIFO and a hook key for one boot and self-corrects at the next capture. A durable token written onto the wrong pane does not self-correct — the hook then fires on the wrong pane on every subsequent reboot. Only panes within `pairCount` are stamped; the unpaired remainder is left untouched.


### 3. The Hook Key

#### 3.1 The key is the pane token alone

A hook key is the pane's `@portal-pane-id` token (§2.1) and nothing else. It carries no session component and no coordinates.

The alternative considered and rejected was a composite `<portal-id>:<pane-token>`, which keeps `hooks.json` readable and preserves session grouping. It was rejected because `move-pane -t <other-session>` changes the session half, so drift returns for exactly the operation the user performs deliberately. A token-only key has no component tmux is free to recompute, so no rearrangement — within a window, across windows, or across sessions — can change it.

This closes all three moments in §1.1:

- **Write time** is closed by §4, not by the format.
- **Lifetime drift** cannot occur: the token is stamped once and tmux never recomputes it.
- **The reboot boundary** closes because restore re-establishes the token itself (§2.3). The key on disk and the key the live pane answers to are the same value regardless of how tmux renumbered the windows, so nothing needs re-keying and the post-restore sweep finds the entry live.

#### 3.2 Key shape

A hook key is **token-shaped** iff it is exactly `suffixLen` characters drawn from `NanoIDAlphabet` (§2.1) — currently `^[A-Za-z0-9]{6}$`.

Old-format keys are `<@portal-id or session_name>:<window>.<pane>` and therefore always contain `:` and `.`, neither of which is in the alphabet. No old-format key can ever be token-shaped, which is what makes the reaper's shape test in §5 sound.

The predicate has **one home** and the shape it tests must not be able to drift from the width and alphabet the generator uses. Where that home sits is an implementation call — `internal/hooks` (which owns the reaper's judgement) does not currently import `internal/session` (which owns `NanoIDAlphabet`), and no cycle would result from it doing so (`grep -rn "leeovery/portal/internal" internal/session/*.go internal/hooks/*.go | grep -v _test.go` → `session` imports `project`, `resolver`, `tmux`; `hooks` imports `fileutil`, `log`, `storelog`).

#### 3.3 Key-producing and key-consuming sites

Every site's source of truth is unchanged from today; what each derives is now a token rather than a composite:

| Site | Path | Source of truth |
|---|---|---|
| Registration (`hook set`) | `cmd/hooks.go` `resolveCurrentPaneKey` → a live read of `@portal-pane-id` for `$TMUX_PANE` | live tmux |
| Removal (`hook rm`, `$TMUX_PANE` path) | same | live tmux |
| Stale sweep | `cmd/run_hook_stale_cleanup.go` → the all-pane token enumeration | live tmux |
| `portal doctor` `checkStaleHooks` | `cmd/doctor.go` → the same enumeration | live tmux |
| Restore / firing | `internal/restore/session.go` `collectArmInfos` → `p.PortalPaneID` | saved `sessions.json` |

The July invariant — *every site that produces or consumes a key derives it by the identical rule* — now holds trivially: there is no derivation, only a read of one value.

**`internal/tmux` surface changes:**

- `HookKeyFormat` becomes the pane-token format `#{@portal-pane-id}`.
- `HookKey(portalID, name, window, pane)` is **deleted**. The saved-state bake is a struct field read, not a formatting call.
- `ResolveHookKey(paneID)` becomes a live read of the pane's token for one pane target.
- `ListAllPaneHookKeys()` becomes an all-pane token enumeration returning **one entry per live pane, empty string for an unstamped pane**. Both properties are load-bearing: the stale comparison uses the non-empty subset, while the mass-deletion guard counts rows (§5.3).

The name-based positional siblings — `StructuralKeyFormat`, `ResolveStructuralKey`, `ListAllPanes` — are untouched. They serve the `@portal-skeleton-*` marker and cleanup paths only (§1.3).

**`portal state hydrate --hook-key`** keeps its flag and its meaning; its help text, which currently reads `Saved structural identifier (<session>:<window>.<pane>)`, is corrected to describe the token.

**`buildHydrateCommand`** (`internal/restore/session.go`) interpolates the key into `portal state hydrate --hook-key %s` through `shellQuoteSingle` and bakes it into a `respawn-pane -k` command line. The quoting is **retained** — the new character set contains no shell metacharacters, so this boundary becomes strictly safer than it is today rather than needing new care.

#### 3.4 An empty key is rejected at every boundary

`hooks.LookupOnResume` (`internal/hooks/lookup.go`) does a bare map index on the key, so a single `""` entry in `hooks.json` would fire its command on **every unstamped restored pane on the machine**. §4 stops the CLI writing one, but a hand-edit or a bug in the out-of-band conversion script (§8) still could.

Two independent guards, both required:

- **`collectArmInfos` must not bake an empty key.** A saved pane with an empty `PortalPaneID` is armed with no hook — the pane restores and hydrates as normal, it simply has nothing to resume.
- **`LookupOnResume` must not honour an empty key.** An empty `hookKey` argument returns "no hook" before the map is consulted, regardless of what the file holds.


### 4. Registration, Removal and Listing

#### 4.1 Registration verifies the pane exists

`portal hook set --on-resume "<cmd>"` refuses a `$TMUX_PANE` that names no live pane, exits non-zero, and writes nothing to `hooks.json`.

This is the only change that closes the write-time moment, because tmux offers no error to detect on the read path: `display-message -p` against a bogus target exits 0 (§1.1). Under §2.2 the stamp precedes the write, and the stamp *is* the check — `set-option -p` against a bogus target exits **1**:

```
tmux -L <sock> set-option -p -t %999 @portal-pane-id X
→ no such pane: %999   (exit 1; tmux 3.7c)
```

So verification is **tmux-native** rather than a shape heuristic on a returned string, and it is available on the read as well as the write. `show-options -p` separates all three cases in one read-only call (tmux 3.7c):

| `tmux show-options -p -t <target> @portal-pane-id` | Result |
|---|---|
| pane does not exist | exit 1, `no such pane: %999` |
| live pane, no token | exit 1, `invalid option: @portal-pane-id` |
| live pane, stamped | exit 0, `@portal-pane-id TOKEN1` |

Portal never parses that message text: a non-zero exit is the whole signal, and tmux's words are passed through to the user unaltered. The sequence is:

1. `requireTmuxPane` — `$TMUX_PANE` must be non-empty (unchanged).
2. Read the pane's token with `show-options -p`. A gone pane fails here, before anything is written. A live pane with a token reuses it (§2.2).
3. A live pane with no token is minted one and stamped with `set-option -p`, which errors on a bogus target in its own right — a second, redundant guard that costs nothing.
4. Write the entry under the token key.
5. Touch `save.requested` (§2.2).

Steps 3 and 4 must not be reordered: a write that precedes the stamp would persist an entry keyed to a token no pane carries.

#### 4.2 Removal verifies the same way — on the `$TMUX_PANE` path only

`portal hook rm --on-resume` run from a pane resolves the key with the same `show-options -p` read as §4.1 step 2, and fails non-zero on the same non-zero exit. This is literally the same guard, not an analogue of it — which is what lets removal carry it without minting anything. It covers the half of the CLI surface the blast radius named and the original framing of B did not.

Removal does **not** mint. A pane with no token has no entry to remove; `hook rm` reports that and exits non-zero rather than silently succeeding, which is the same silent-success shape as the `:.` bug on the write side.

#### 4.3 `--pane-key` stays a literal pass-through

`portal hook rm --on-resume --pane-key <key>` performs **no validation of any kind** and touches tmux not at all. The key is used verbatim.

This is deliberate and is not weakened by B. Spec `hooks-rm-pane-key-flag` (2026-05-26) made the flag a pass-through precisely so an entry whose pane no longer exists can be pruned — validating it would defeat its only purpose. The rule that separates the two paths: **a key Portal resolves must identify a live pane; a key the caller hands over explicitly must not be second-guessed.**

`--pane-key` is also the route by which an old-format entry can be removed by hand after this change, since no live pane will ever resolve to one.

#### 4.4 `portal hook list` renders the resolved location

`hook list` gains a fourth tab-separated column carrying the token's resolved `<session>:<window>.<pane>` location:

```
k3Bq7z	on-resume	cd "/x" && claude --resume 9e4d…	agentic-workflows-refactor-wt:1.1
```

The column is **appended**, so the existing `key` / `event` / `command` field positions are undisturbed for any caller parsing the output.

This is what pays back the readability the token-only key costs (§3.1). Without it the file is a list of opaque six-character tokens with no way to answer "which pane is this?" short of a manual `list-panes` diff — the same hand audit this defect already forced once.

Resolution is one `list-panes -a` read over the token format (§3.3), reused across all rows. A token that resolves to no live pane renders the column **empty** rather than failing the command — including the case where no tmux server is running at all, which `hook` is bootstrap-exempt from starting. An old-format key likewise renders empty, since no live pane can answer to one.


### 5. Stale Cleanup

#### 5.1 What the reaper does now

`runHookStaleCleanup` (`cmd/run_hook_stale_cleanup.go`) enumerates live pane keys, loads `hooks.json`, and deletes every persisted key absent from the live set. It runs from two call sites over the same code path: the daemon's idle branch every 10s (`cmd/state_daemon.go` `maybeRunHookCleanup`), and `portal doctor --fix` (`cmd/doctor.go` `pruneDoctorStaleHooks`), which supplies an `onRemoved` callback printing `Pruned stale hook: <key>`.

Two changes, and no more. **Whether the reaper deletes is not changed** — only what it can identify, and what it records.

#### 5.2 Deletion becomes shape-aware

- A **token-shaped** key (§3.2) whose token is absent from the live set is deleted, exactly as today.
- A key that is **not token-shaped** is **retained**, untouched, on every run.

The justification is that A removes the reaper's false positives. Under a positional key a moved pane and a dead pane are indistinguishable at the point of comparison, so the reaper was acting correctly on false information. Under a token key a moved pane keeps its token, so an absent token now means a genuinely absent pane and the reaper's judgement becomes trustworthy. What remains worth protecting is the one case it still cannot judge: an unconverted old-format key, which is not evidence of a dead pane but of an unconverted entry — and is distinguishable by shape.

Three consequences follow from putting the protection on shape rather than on a call site:

- **`portal doctor --fix` keeps its documented prune unchanged.** The protection travels with the rule, so it holds wherever the rule lives. There is no "guard at the daemon call site versus inside `CleanStale`" split, and no window in which one `doctor --fix` run destroys every unconverted entry.
- **`portal doctor` stays green and keeps its "exit 0 iff all pass" contract.** A closed pane's entry is still deleted, so retained old-format entries are the only thing that persists — and `checkStaleHooks` (§5.4) is amended not to count them as failures.
- **Retained entries do not accumulate without bound.** A live pane's entry is never stale, and a closed pane's entry is absorbed as it always has been. The retained set is the old-format residue only, which the out-of-band conversion (§8) clears.

**Accepted cost:** full retention — deleting nothing, ever — would have made a *future* key defect visible instead of silent, because the entries would still be sitting there. Shape-aware deletion does not preserve that property. What survives of it is §5.3, which is the part that was actually missing: the reaper never recorded what it took.

#### 5.3 The reaper names what it deleted

Each removed key is logged at **INFO** under the `hooks` component, not only at DEBUG.

Today the per-key line is `logger.Debug("clean-stale", "op", "clean-stale", "hook_key", key, "via", "internal")` (`internal/hooks/store.go:220`) and production INFO gets only the batch summary `hooks: clean-stale op=clean-stale entries=N` from `storelog.EmitCleanStaleSummary`. At the production default level the log therefore cannot answer "what did I lose?" after the fact — which is why the two named instances in the investigation had to be reconstructed by correlating a registration breadcrumb against a bare count.

The batch summary is retained alongside. The existing `hooks` component attr vocabulary (`op`, `hook_key`, `via`, `entries`) is sufficient — no new component and no new attr key.

`portal doctor --fix`'s `Pruned stale hook: <key>` output is unchanged; it already named the key.

#### 5.4 The mass-deletion guard keys off live *panes*, not live *tokens*

`runHookStaleCleanup` carries one guard: an empty live set is treated as a bad tmux read rather than as authority, and the sweep defers to the next run with a WARN (`cmd/run_hook_stale_cleanup.go:41-47`). `checkStaleHooks` carries the matching not-evaluable branch (`cmd/doctor.go`), and `pruneDoctorStaleHooks` documents that it deliberately adds no second guard.

Under lazy stamping (§2.2), **zero stamped panes is the ordinary steady state** — every pane before its first `hook set`, and the whole install during the upgrade window. If the guard tested the token set it would fire every 10s with a WARN naming a mass-deletion hazard that does not exist.

The guard's question is *"did the tmux read succeed?"*, which the **pane row count** answers. The token set answers a different question — *"which panes are protected?"* — and the two must not be conflated. This is why the enumeration returns one row per live pane including empties (§3.3): the guard counts rows, the stale comparison uses the non-empty subset.

`checkStaleHooks` is amended in step: its stale count is taken over token-shaped keys only, so retained old-format entries never push a healthy install to a non-zero exit code.


### 6. `hooks.json` Concurrency

#### 6.1 The open window

`internal/hooks` has no locking of any kind (`grep -rn 'sync\.\|Mutex\|flock\|Lock()' internal/hooks internal/fileutil | grep -v _test.go` → no matches). `Store.Set`, `Store.Remove` and `Store.CleanStale` are each `Load()` → mutate in memory → `fileutil.AtomicWrite`. `AtomicWrite` makes each *write* atomic; nothing guards the read-modify-write window, and the writers are in **different processes** — the CLI (`portal hook set`, fired by a Claude Code SessionStart) against the daemon's sweep every 10s, and CLI against CLI (a SessionStart in one pane concurrent with a SessionEnd in another).

```
daemon  CleanStale  t0    Load() → 41 entries
CLI     hook set    t0+e  Load() → 41, add K, AtomicWrite → 42
daemon             t0+d  writes its t0 snapshot minus stale → 40      K is gone
```

The end state is indistinguishable from the drift symptom: an INFO `hooks: set … hook_key=K` breadcrumb, K absent minutes later, and the intervening `clean-stale entries=1` naming a *different* key even at DEBUG.

**This is present but unquantified.** No observed loss is proven to be a race — the two named instances have ~4-minute gaps between registration and prune, which rules a race out for them. It is not ruled out for any other instance. The window is two file reads plus a marshal, so per-event probability is low; the daemon sweeps ~8,640 times a day against a 40+ pane working set.

It is untouched by A: durable identity does not close a lost update. It is included because the cost is small, the pattern is already in the codebase, and the alternative is knowingly leaving a silent-data-loss path open in the one file this work unit exists to protect.

#### 6.2 A sidecar lock file, never `hooks.json` itself

The lock is taken on a dedicated file derived from the resolved `hooks.json` path (`<hooks.json path>.lock`), so it follows a `PORTAL_HOOKS_FILE` override wherever it points.

Locking `hooks.json` itself would provide **no exclusion at all**. `fileutil.AtomicWrite` writes a temp file and `os.Rename`s it over the target (`internal/fileutil/atomic.go:77`), so the target's inode is replaced on every write — a lock held on the pre-rename inode is a lock on an unlinked file. The precedent this copies, `state.AcquireDaemonLock`, is correct precisely *because* it locks a dedicated `daemon.lock`.

The lock file is opened `O_CREAT` and **never unlinked**. `AcquireDaemonLock`'s inode cross-check and bounded retry ladder exist to absorb an unlink-and-recreate race; nothing here unlinks, so that machinery is deliberately not reproduced.

#### 6.3 Readers take a shared lock, writers exclusive

`flock` shared (`LOCK_SH`) for reads, exclusive (`LOCK_EX`) for the whole read-modify-write.

`Store.Load` is on the path of `hook list`, `LookupOnResume`, `checkStaleHooks` and the sweep's own pre-read. During a restore of a 40+ pane working set every hydrate helper calls `LookupOnResume` at once; a blanket-exclusive lock on `Load` would serialise them for no benefit.

The exclusive hold must span the **whole** mutation, not each file operation. `Set`, `Remove` and `CleanStale` each read, mutate and write; taking a shared lock to read and an exclusive lock to write would reopen the identical window. The exported methods acquire once and hold across their internal load and save.

#### 6.4 The locked region covers the file only

**No tmux call may sit inside the lock.** A lock spanning a tmux enumeration would let one hung tmux read block every `hook set` on the machine behind it.

This falls out naturally at the sweep's call site: `runHookStaleCleanup` performs its `ListAllPaneHookKeys` read before it touches the store at all, and the guard on that read (§5.4) resolves before any lock is taken.

#### 6.5 Acquisition is bounded, and a timeout degrades rather than wedges

Acquisition waits, but not indefinitely. On timeout:

- the **daemon sweep** skips this cycle with a WARN and retries on the next 10s cadence — a deferred prune costs nothing, since stale entries are inert;
- the **CLI** (`hook set`, `hook rm`, `hook list`) exits non-zero with the reason, rather than hanging a shell the user is sitting in.

An unbounded `LOCK_EX` is simpler and carries no stale-lock hazard — `flock` is kernel-released on process death. It is rejected because D introduces a blocking path into a loop the daemon runs every 10 seconds, which the investigation named as the single riskiest part of this work unit; a holder wedged by a stopped process or a hung filesystem would take the daemon's tick loop with it.

The bound is **2 seconds**. The critical section is one small-file read, a marshal, and a rename — sub-millisecond in practice — so 2s sits roughly three orders of magnitude above the expected hold while staying well inside the sweep's own 10s cadence. A timeout at that bound means something is genuinely wrong, not merely contended, which is what makes the WARN worth emitting.


### 7. Removing the `@portal-id` Machinery

#### 7.1 Why it goes, and why now

`@portal-id` exists for exactly one reason: so a session rename cannot orphan a resume hook (spec `session-rename-orphans-resume-hook`, 2026-07-04). A token-only key (§3.1) carries no session identity at all, so **renames become irrelevant by construction** — A subsumes that fix's purpose, not merely its machinery.

Every non-test consumer of `@portal-id` exists to build the hook key and nothing else (`grep -rn "PortalIDOption\|@portal-id\|PortalID" internal cmd --include="*.go" | grep -v _test.go` → 21 lines across 7 files, one of them a doc comment in `cmd/run_hook_stale_cleanup.go`). A token-only pane key makes all of it dead at once.

The deciding argument is the supersession, not the dead weight. Retaining it would leave two identity systems, one inert, with source comments cross-referencing a key format that no longer exists — the exact "ship it and remember to delete it later" pattern rejected for the migration (§8).

#### 7.2 What is removed

| Site | What goes |
|---|---|
| `internal/session/create.go` | `PortalIDOption` const; the `SetSessionOption` stamp in `CreateSession` |
| `internal/session/quickstart.go` | the `set-option -t <name> @portal-id <token>` link in the chained `ExecArgs` |
| `internal/state/capture.go` | the `#{@portal-id}` `captureFormat` column (replaced per §2.3) and the session-scoped lift into `Session.PortalID` |
| `internal/state/schema.go` | `Session.PortalID` |
| `internal/restore/session.go` | the `@portal-id` re-stamp; the `tmux.HookKey(sess.PortalID, …)` bake (replaced per §3.3) |
| `internal/tmux/tmux.go` | `HookKey`; the `@portal-id` conditional inside `HookKeyFormat` (replaced per §3.3) |
| `cmd/state_migrate_rename.go` | the whole file — see §7.3 |

`Session.PortalID` leaving the schema is a field **removal** from a tolerant-decode struct: an existing `sessions.json` carrying `portal_id` decodes fine and the value is ignored. No `SchemaVersion` bump, no migration — the mirror image of the field addition in §2.3.

`@portal-dir` is **untouched**. It is a separate stamp serving session grouping in the TUI, unrelated to hook identity.

#### 7.3 `portal state migrate-rename` goes; one reference to it stays

`cmd/state_migrate_rename.go` rewrites hook keys by `<oldName>:` prefix (`runMigrateRename`). Under a token key no key can carry a session-name prefix, so it can match nothing. It is already inert in production — registration never installs its hook, and `managedEvents` binds `session-renamed` to `notifyCommand` (`internal/tmux/hooks_register.go:22`) — but it is still registered as a hidden subcommand (`cmd/state_migrate_rename.go:71`). The command and its implementation are deleted.

**`migrateRenameSubstring` is not removable** (`internal/tmux/hooks_register.go:45`, consumed by `teardownFingerprints` at `:64-67`). It exists so `portal uninstall` reaps the inert `session-renamed` hooks that *older binaries* installed on the user's tmux server. That job outlives the command itself and must stay, with its comment intact.

#### 7.4 Replacement and removal ship together

Both halves land in one release.

**Accepted cost:** a misbehaving release cannot be bisected between "the new key works" and "the old machinery is gone". This is deliberate. Splitting them is the pattern rejected in §7.1 and §8, and this is a single-install tool that can be rolled back wholesale.


### 8. Migration

#### 8.1 No migration code ships

Existing `hooks.json` entries are keyed `<@portal-id or session_name>:<window>.<pane>`; the token-only key (§3.1) changes every key on disk. **No migration code is written, and none ships.**

Portal has one install and no evidence of any other. The user's call is that a second install, if one exists, is not worth carrying compatibility code for.

Two alternatives were rejected:

- **Using `CleanStale` as the migration** — the precedent from spec `resume-hooks-lost-on-server-restart` (2026-04-30), which accepted a breaking key change and let the sweep absorb the old entries. Repeating it here would silently destroy every existing hook on the first sweep after upgrade.
- **A one-release migration, isolated and deleted in the next release** — ships code whose whole purpose is to become obsolete, and leaves a removal the user has to remember.

#### 8.2 The conversion is a throwaway script, outside this work unit

The re-key is a one-time transformation of one file on one machine: resolve each entry's positional key to its live pane, stamp a token on that pane, rewrite the entry under the token. It is authored as a script **outside Portal, and outside this specification and its plan**. It is not a deliverable here and no work item covers it.

#### 8.3 What makes that safe rather than reckless

§5.2. The reaper retains any key it cannot parse as a token, and an old-format key never can be, so:

- an unconverted entry sits **inert** rather than being deleted;
- a partial conversion costs nothing — the converted entries work, the unconverted ones wait;
- the protection is by **key shape**, so it holds wherever the rule lives, including `portal doctor --fix`, which shares the same code path.

The one thing not covered by code is ordering: an entry registered *between* the upgrade and the script's run is already token-keyed and needs no conversion, while one registered before it is old-format and does. Running the script after upgrading is the mitigation, not a code path.

#### 8.4 The user's own integration

`~/.claude/hooks/portal-resume-hook.sh` re-implements `HookKeyFormat` verbatim (`:87-95`) and matches it against `portal hook list` output. It needs updating in step with this change. It is out of scope (§1.3) and named here only because the conversion script and the hook script are the same person's job on the same day.

It fails safe in the meantime: the script documents the coupling, an unrecognised key scheme yields an empty key, and its guards then refuse to remove anything. So a key-scheme change degrades it silently rather than erroring — its SessionEnd branch stops deregistering, and those entries are absorbed by the reaper exactly as they always have been.


### 9. Testing

#### 9.1 Lane placement

The project's lane rule is unchanged and binding: every test that spawns a `portal state daemon` or execs a built `portal` binary carries `//go:build integration`; the unit lane holds the rest, including the fast real-tmux **client** tests under `internal/tmux/*_realtmux_test.go` (per-test `-L` sockets, no daemons, no subprocess binaries).

Two consequences for this work:

- The tmux behaviours B rests on are pinned in `internal/tmux` real-tmux client tests, where that pattern already lives. A `cmd`-level test then asserts the CLI propagates the failure, with its `*Deps` seam injected — `cmd`'s `TestMain` poisons `TMUX` package-wide, so a real-tmux `cmd` test would have to supply its own socket and is avoided.
- Anything driving restore's arm phase execs `portal state hydrate` through `respawn-pane -k`, so it is integration-lane and builds its binary via `portalbintest`.

#### 9.2 New tests

**The gap that let this bug live for months** is that no test ever moved a pane. The existing cross-site tests prove every site derives the same key for a pane *at rest* — the case that works.

| Test | What it asserts | Lane |
|---|---|---|
| **A pane that moves keeps its hook** | Register a hook; `break-pane` it out, close an earlier window under `renumber-windows on`, `move-pane` it back, `move-pane` it to another session; the hook still resolves after each. | unit (real-tmux) |
| **Non-contiguous saved window indices** | Restore a session whose saved window indices are non-contiguous, with `renumber-windows off` (tmux's default, not the user's setting); assert the hook fires **and** survives the post-restore sweep. This is the H6 case — it fires correctly once today and then dies. | integration |
| **`set-option -p` rejects a bogus pane** | `set-option -p -t %999 @portal-pane-id X` exits non-zero, unlike `display-message -p` against the same target, which exits 0. The behaviour B is built on. | unit (real-tmux) |
| **`hook set` on an unresolvable `$TMUX_PANE`** | Exits non-zero and writes nothing to `hooks.json`. | unit |
| **`hook rm` on an unresolvable `$TMUX_PANE`** | Exits non-zero and writes nothing, while `hook rm --pane-key <anything>` still succeeds unchanged (§4.3). | unit |
| **`hook set` reuses an existing token** | A second registration on the same pane writes under the same key and mints nothing (§2.2). | unit (real-tmux) |
| **Reaper shape-awareness** | An old-format (non-token) key is retained by both the daemon sweep and `portal doctor --fix`; a token-shaped key whose token is absent is still deleted; the deletion names the key at INFO rather than only counting it (§5.2, §5.3). | unit |
| **`portal doctor` exit code stays 0** | With retained old-format entries present (§5.4). | unit |
| **No spurious mass-deletion WARN** | A server with hooks present and zero stamped panes does not emit the guard's WARN (§5.4). | unit |
| **An empty key fires on nothing** | A `""` entry in `hooks.json` fires on no restored pane; `collectArmInfos` bakes no empty key (§3.4). | unit |
| **Lost update** | Interleaved writers across the `Load`→`AtomicWrite` window; no entry disappears. **The assertion must exercise the `AtomicWrite` rename specifically** — two writers will usually serialise by luck and pass against a broken lock, so the test must show exclusion holds across an inode swap, which is what a lock taken on `hooks.json` itself would fail (§6.2). | unit |
| **Restore re-stamp failures are surfaced** | A failed pane re-stamp produces a WARN naming the session and pane, and does not abort the restore (§2.4). | unit |
| **`armPanes` short-list stamps nothing** | With live and saved pane counts differing, the unpaired remainder carries no token (§2.4). | unit |
| **`hook list` fourth column** | Renders the resolved location for a live token, and empty for a token that resolves to no live pane (§4.4). | unit (real-tmux) |

#### 9.3 Existing tests to re-point or retire

- **`internal/tmux/resolve_hookkey_realtmux_test.go`** — its comment states the `:.` behaviour outright: *"tmux 3.7's display-message tolerates a bogus -t target (it returns `:.` with exit 0), so killing the server first is the only reliable way to drive the read-failure path."* The knowledge was in the repo the whole time, filed as a testing obstacle rather than a bug. Its premise changes: the bogus-target case is now a real failure, via `set-option -p`.
- **`internal/tmux/hookkey_cross_site_realtmux_test.go`**, `hookkey_format_realtmux_test.go`, `hookkey_realtmux_shared_test.go`, `hookkey_test.go`, `list_all_pane_hookkeys_realtmux_test.go` — re-pointed at the token key and the token enumeration.
- **`cmd/hookkey_no_regression_upgrade_test.go`** — asserts an un-stamped session's name-keyed hook survives, which is precisely the `@portal-id` fallback branch being deleted. Retired.
- **`cmd/rename_restore_cleanup_survival_integration_test.go`**, `internal/restore/rename_reboot_hook_integration_test.go`, `rename_reboot_durability_integration_test.go` — these prove the July rename fix. Renames are now irrelevant by construction (§7.1), so what they should assert is that a rename *still* cannot orphan a hook, now for a different reason. Re-pointed, not retired: the user-visible guarantee is unchanged and is worth keeping under test.
- **`internal/transienttest`** `SeedHooksJSON` / `HooksJSONBytes` (`internal/transienttest/hooks.go:38,57`) — the single-sourced hook seeder for the two destructive integration suites. The key-shape change routes through it, so it is amended once rather than at each consumer.
- **`internal/session/create_test.go`**, `quickstart_test.go`, `internal/state/*`, `internal/restore/session_test.go`, `cmd/hooks_test.go`, `cmd/state_hydrate_test.go` — updated in step with §7.2.

#### 9.4 Guards

Two literal-binding guards exist for `@portal-id` and must be re-pointed at `@portal-pane-id` rather than deleted — the hazard they encode is unchanged:

- **`cmd/portal_id_binding_guard_test.go`** asserts `session.PortalIDOption == "@portal-id"` and that `tmux.HookKeyFormat` contains it. `cmd` can import both packages cycle-free, which is what lets the constant and the format string be compared at all.
- **`internal/state/portal_id_literal_guard_test.go`** asserts `captureFormat` contains the literal, spelled out rather than imported because `internal/session` transitively depends on `internal/state` and the import would cycle. **That cycle is unchanged**, so the `@portal-pane-id` literal remains duplicated between the option constant and `captureFormat`, and the guard remains the only thing binding them.

#### 9.5 The positional siblings are checked, not assumed separate

`state.SanitizePaneKey` and `internal/tmux`'s `StructuralKeyFormat` / `ResolveStructuralKey` / `ListAllPanes` key the hydrate FIFO paths and `@portal-skeleton-*` markers off the same positional addressing (§1.3). They are not changed, but the addressing assumption is identical, so their existing coverage is run against the change rather than assumed unaffected — specifically that a restore whose window indices are renumbered still pairs FIFOs and markers correctly, which is the §9.2 non-contiguous-index test observing a second surface.


---

## Working Notes
