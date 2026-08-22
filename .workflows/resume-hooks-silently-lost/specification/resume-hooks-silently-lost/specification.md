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


---

## Working Notes
