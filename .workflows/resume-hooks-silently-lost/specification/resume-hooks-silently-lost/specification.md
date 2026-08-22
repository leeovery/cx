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
- **Restore.** After the skeleton exists and before each pane is armed, the saved token is re-stamped onto the corresponding live pane with `set-option -p`.

#### 2.4 Restore re-stamp: failures are surfaced, mispairings are not stamped

Two rules distinguish the pane re-stamp from the session re-stamp it is modelled on.

**A failed re-stamp must not be swallowed.** The session stamp discards its error (`_ = r.Client.SetSessionOption(...)`, `internal/restore/session.go:79`) because a missed `@portal-id` costs only rename-immunity. For the pane token the stamp *is* the identity: a swallowed failure permanently orphans that pane's hook with no trace. A failed pane re-stamp logs at WARN under the `restore` component, naming the session and pane. It does not abort the restore — an unstamped pane is a lost hook, not a lost session.

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


---

## Working Notes
