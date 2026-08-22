# Investigation: Resume Hooks Silently Lost

## Symptoms

### Problem Description

**Expected behavior:**

A per-pane resume hook registered with `portal hook set --on-resume "<cmd>"` stays bound to
that pane for the pane's lifetime, and fires when the pane is restored after a reboot. A
registration that cannot identify its pane fails loudly, so the caller can react.

**Actual behavior:**

Hooks go missing with no error and no user action, in two distinct ways:

1. **Degenerate key at registration.** `portal hook set --on-resume` exits 0 and persists an
   entry even when `$TMUX_PANE` names a pane that does not exist. The hook key collapses to
   the literal `:.` (empty session component, empty window and pane indices) and lands in
   `hooks.json` under that key. The pane the caller meant to protect has no hook; the caller
   sees success.
2. **Key drift over the pane's lifetime.** The hook key bakes `window.pane` coordinates at
   registration time and nothing re-keys the entry when the pane moves. tmux derives
   `pane_index` from layout position, so ordinary rearrangement (closing a sibling pane,
   closing a window with `renumber-windows on`, `break-pane`, `move-pane`) changes the
   coordinates. The key stops addressing its own pane, and the daemon's stale sweep removes
   it — correct by its own rule, since no live pane answers to that key.

Both failure modes end the same way: a pane comes back from a reboot to a bare shell instead
of resuming its work.

### Manifestation

- No error at registration — exit code 0, no output, entry present in `hooks.json`.
- Nothing at production INFO level names what was removed. The only trace of a prune is
  `hooks: clean-stale op=clean-stale entries=1` — a count, no key.
- Recurring `hooks: rm op=rm hook_key=:. via=cli` lines in `portal.log` across both 0.10.5
  and 0.11.0 (2026-08-11, 2026-08-14 ×2, 2026-08-15 ×2), each followed within seconds or
  minutes by a daemon `clean-stale entries=1`. The recurring single-entry stale sweeps in the
  log are this junk key being reaped, not genuine staleness — which makes the clean-stale
  signal misleading when reading the log for real problems.
- `portal doctor` reports all checks green throughout, including `stale hooks: no stale
  hooks`.
- Scrollback still restores, so a hookless pane looks healthy — only the auto-resume is gone,
  and it is invisible until the reboot that needed it.
- Detectable only by diffing `hooks.json` keys against the live pane list.

### Reproduction Steps

**Degenerate key** (reproduced 2026-08-16 against Portal 0.11.0):

1. Pick a `%<n>` pane id that is not live (e.g. `%165`).
2. Run `TMUX_PANE=%165 portal hook set --on-resume "noop"`.
3. Command prints nothing and exits 0; `hooks.json` gains `":.": { "on-resume": "noop" }`.
4. The entry is removable with `portal hook rm --on-resume --pane-key ':.'` — so the `rm`
   path accepts the degenerate key too.

**Key drift** (reproduced in an isolated `-L` socket):

1. Register a hook on a pane; note its `<id>:w.p` key.
2. `break-pane` the pane out, then kill an earlier window.
3. The unchanged pane goes `t:1.2` → `t:2.1` → `t:1.1` — three distinct keys for one pane.
4. The daemon's stale sweep reaps the entry at the original key.

**Reproducibility:** Always for both, given the trigger condition (unresolvable
`$TMUX_PANE`; any pane rearrangement).

### Environment

- **Affected environments:** local (the only environment — Portal is a local CLI).
- **Platform:** macOS, tmux 3.6b, `renumber-windows on` in the user's config.
- **Versions:** observed on 0.10.5 and 0.11.0.
- **User conditions:** any pane with a registered resume hook. Callers include the Claude
  Code SessionStart hook at `~/.claude/hooks/portal-resume-hook.sh`, which shells out to
  `portal hooks set` and logs the return code — so it records success against a `:.` write.

### Impact

- **Severity:** High — silent loss of session continuity across reboots, the exact case the
  resurrection feature exists to cover.
- **Scope:** Single user (personal tool), but a large working set. A 2026-08-16 audit found
  6 of 42 panes running Claude with no hook registered. A 2026-06-29 crash-recovery reboot
  had ~4 of 32 sessions come back without resuming.
- **Business impact:** Undercuts the "everything comes back" guarantee; the user must notice
  which panes did not resume and restart that work by hand.

### Observed Instances

- `agentic-workflows-refactor-wt` (pane `OCCvED:1.1`) — hook registered 2026-08-16T22:17:55
  under key `OCCvED:1.2`, pruned by the daemon at 22:22:16, ~4 minutes later. Pane still
  alive, command and session id correct; only its position changed.
- `dex` window 2 (pane `d5H0DK:2.1`) — hook registered 2026-08-20T22:48:07 under key
  `d5H0DK:1.2`, pruned 2026-08-21.

### Folded-in Sighting — resolved, out of scope (2026-08-22)

The 2026-06-29 capture — after a crash-recovery reboot with 32 Claude sessions, ~28 resumed
and ~4 did not, tmux sessions present but on-resume behaviour absent — is **explained, and is
not a Portal defect**. Claude Code garbage-collects session transcripts on a retention window
that defaulted to 30 days. The stored hook command is `cd "<launch-dir>" && claude --resume
<session_id>`; once the transcript for that id is gone, `claude --resume` fails and the hydrate
helper's chain (`sh -c '<HOOK>; exec $SHELL'`) falls through to a bare shell. Pane present,
no resume, nothing reported — exactly the observed shape, and it explains the age correlation
(only sessions older than the retention window are affected) and the minority subset.

The user has since raised Claude Code's retention to 3,500 days, which closes it at source.
Carried here for reference only; **no fix work in this unit addresses it**, and the residual
risk discovery flagged (that an unrelated symptom would be dragged through the fix) is
discharged rather than carried.

### Scope

Two mechanisms are in scope, both live and both Portal defects:

1. **Degenerate key at registration** — `hook set` accepts an unresolvable pane, writes `:.`,
   exits 0.
2. **Coordinate drift over the pane's lifetime** — the key bakes `window.pane`, tmux renumbers
   under ordinary rearrangement, the daemon reaps the orphaned entry. The user moves panes and
   promotes panes to windows deliberately and needs resume to survive it — so re-keying, not
   discouraging the rearrangement, is the target.

### Known-adjacent, explicitly out of scope

The subagent-SessionEnd clobber in `~/.claude/hooks/portal-resume-hook.sh` (a subagent's
SessionEnd deletes the pane's real resume hook; needs a `session_id` guard) is a separate
failure mode on the caller side, not in Portal.

### Standing Evidence (verified 2026-08-22, live install, read-only)

**The install is currently clean.** `hooks.json` holds 42 entries; every one matches a live
pane key. 45 live panes; the 3 without hooks are `_portal-bootstrap:1.1`, `_portal-saver:1.1`
and `los3Lo:1.2` (a second pane, not a Claude pane). So the bug is not currently manifesting
— it is intermittent and trigger-driven, not a standing corruption.

**Every live hook key is `<id>:1.1`.** The working set is overwhelmingly one window, one pane
per session, which is why drift is occasional rather than constant — the coordinates only move
when a session grows a second window/pane and later loses it.

**`portal.log` census across 2026-07-23 → 2026-08-22** (retention is 30 days; nothing earlier
survives):

- 63 lines carrying `hook_key=:.` — **61 `op=rm`, 2 `op=set`**. The two sets are
  2026-07-27T16:37:29 (`value="cd "/Users/leeovery" && claude --resume 9e4d…"`, v0.10.3 — a
  real registration lost) and 2026-08-16T19:06:49 (`value=noop`, v0.11.0 — the deliberate
  repro from the seed).
- The `rm :.` lines run near-daily from 07-23 to 2026-08-12 and stop after 2026-08-16. Each is
  a Claude SessionEnd whose `resolveCurrentPaneKey()` returned `:.` — so the degenerate
  resolution is common at SessionEnd, not a rarity. The matching SessionStart registrations
  were therefore also frequently degenerate; only 2 `set :.` lines survive in the retained
  window because most degenerate SessionStarts were suppressed by the caller (below) or fell
  outside retention.
- `hooks: clean-stale entries=1` continues **after** the `:.` writes stop — 13 prunes between
  2026-08-17 and 2026-08-21 inclusive. With no degenerate key left to reap, each of those is a
  genuine hook being removed. Independent confirmation that a second loss mechanism is live and
  distinct from the `:.` issue.

**A caller-side workaround already masks part of the symptom.** `~/.claude/hooks/portal-resume-hook.sh`
(dotfiles commit `26fded9`, 2026-08-16 19:44) added a pane-ownership guard for the nested/subagent
clobber and a `[[ -z "$key" || "$key" == ":." ]] && return 0` bail in `stored_resume_id`. That is
why `rm :.` noise stops on 08-16. **Portal itself is unchanged** — the degenerate write path is
still open; a caller without that guard still hits it, and even the guarded caller still calls
`portal hooks set` blind (it reads `rc`, which is 0).

### References

- `seeds/2026-08-16-hook-set-persists-degenerate-pane-key.md`
- `seeds/2026-08-21-resume-hooks-vanish-when-pane-moves.md`
- `seeds/2026-06-29-some-sessions-skip-resume-on-reboot.md`
- `.workflows/resume-hooks-silently-lost/discovery/sessions/session-001.md`

---

## Analysis

### Hypotheses

**Checkpoint depth:** check-ins

- **H1 — `hook set` persists a degenerate key because tmux reports success** [confirmed]
  Sandbox repro (2026-08-22, isolated `-L` socket): `tmux display-message -p -t %999
  '#{?@portal-id,#{@portal-id},#{session_name}}:#{window_index}.#{pane_index}'` **exits 0 and
  prints `:.`** — tmux does not error on an unresolvable pane target for a `-p` message; it
  expands the format against nothing and every field comes back empty. `ResolveHookKey`
  (`internal/tmux/tmux.go:236`) returns that verbatim, `resolveCurrentPaneKey`
  (`cmd/hooks.go:36`) passes it through, and `store.Set` writes it. No validation at any layer.
  **Confirmed** by full trace — see Code Trace finding 1. A repo-wide search finds no hook-key
  shape validation anywhere.

- **H2 — Coordinate drift orphans a live pane's hook, and the daemon then reaps it** [suspected]
  The key bakes `window_index.pane_index`; tmux renumbers on pane close, window close (with
  `renumber-windows on`), `break-pane` and `move-pane`. Nothing re-keys the entry.
  `runHookStaleCleanup` -> `hooks.CleanStale` then removes it, correct by its own rule.
  Evidence: 13 genuine `clean-stale entries=1` prunes 2026-08-17..08-21 (after `:.` writes had
  stopped), plus the two named instances in the seed.

- **H3 — The identity design is half-finished, and that is the single root cause** [suspected]
  `@portal-id` (spec `session-rename-orphans-resume-hook`, 2026-07-04) made the *session* half
  of the key immutable. The *pane* half is still a position. H1 and H2 are the same missing
  piece seen at two moments: identity unverified at write, identity unstable over the lifetime.

- **H4 — Whatever replaces coordinates must survive a tmux server restart** [suspected]
  Spec `resume-hooks-lost-on-server-restart` (2026-04-30) chose positional keys *deliberately*,
  replacing pane IDs, because `%N` is reassigned by the server on restart. Drift is the bill for
  that trade. Any fix has to pay the restart problem back, not re-open it.

- **H5 — The reaper and doctor are why this ran for months unnoticed** [suspected]
  The daemon prune emits `clean-stale entries=N` with no key at INFO (per-key detail is DEBUG,
  `internal/hooks/store.go:220`). `portal doctor`'s stale-hook check (`cmd/doctor.go:289`)
  measures against the same live-key rule that does the reaping, so it reports green while
  hooks are being deleted. Contributing factor rather than root cause.

### Code Trace

**Agreed trace lines (2026-08-22):**

1. `cmd/hooks.go` `resolveCurrentPaneKey` -> `tmux.ResolveHookKey` -> `Commander.Run` — the
   exit-0 acceptance, and whether validation exists anywhere downstream.
2. `internal/tmux/tmux.go` — `HookKeyFormat`, `HookKey`, `ListAllPaneHookKeys`: the three
   key-producing sites and what identity each can express.
3. `cmd/run_hook_stale_cleanup.go` -> `internal/hooks/store.go` `StaleKeys`/`CleanStale`, and
   the daemon call site — the reaper, its guards, what it logs.
4. `internal/state/capture.go` `captureFormat` + `internal/restore/session.go` — do saved
   window/pane indices match what restore recreates?
5. Sandbox — what per-pane identity survives a tmux server restart plus a Portal restore. The
   feasibility check the whole fix rests on.
6. `cmd/doctor.go` stale-hooks check — why it reports green.

**Findings:**

#### Finding 1 — the degenerate-key write path (H1 confirmed)

```
cmd/hooks.go:91   hooksSetCmd.RunE
  -> cmd/hooks.go:36   resolveCurrentPaneKey()
     -> cmd/hooks.go:23  requireTmuxPane()  — only checks TMUX_PANE is non-empty
     -> internal/tmux/tmux.go:236  Client.ResolveHookKey(paneID)
        -> internal/tmux/tmux.go:42   RealCommander.Run("display-message","-p","-t",paneID,HookKeyFormat)
           -> internal/tmux/tmux.go:49  runCommand -> exec.Cmd.Output()
              BUG: tmux exits 0 for an unresolvable pane target, printing ":."
                   so err is nil and the caller has no signal
  -> internal/hooks/store.go:81  Store.Set(":.", "on-resume", cmd, "cli")  — writes it
```

Every layer behaves correctly in isolation; the contract breaks at the tmux boundary. `Run`
maps a zero exit to `err == nil` (`internal/tmux/tmux.go:52`) — right for every other tmux
call. `ResolveHookKey`'s doc comment already warns that a *read failure* must never fall back
to a name-based key, but tmux never reports this as a failure, so the guard never engages.
There is no shape validation in `resolveCurrentPaneKey`, in `Store.Set`, or anywhere in the
repo (searched).

**Why `$TMUX_PANE` is unresolvable so often.** The 61 `rm :.` lines are all Claude `SessionEnd`
events, and all predate the caller-side guard. The pre-guard script (dotfiles `26fded9^`) called
`portal hooks rm --on-resume` unconditionally at SessionEnd. SessionEnd commonly fires *because
the pane was closed* — tmux destroys the pane, Claude gets SIGHUP and exits, and the hook then
runs against a pane id tmux has already reclaimed.

**The `rm` side has its own consequence, distinct from the `set` side.** When deregistration
resolves to `:.`, `store.Remove` deletes nothing that matters — so the pane's *real* hook entry
survives its Claude session. It is then reaped later by the stale sweep (pane genuinely gone),
so the end state is usually benign; but the same silent-success shape means Portal reports a
successful removal that removed nothing.

**Impact of the `set` side is real, not just noise.** 2026-07-27T16:37:29 wrote
`":." -> "cd \"/Users/leeovery\" && claude --resume 9e4d7901-…"` — a genuine registration that
went to the junk key instead of the pane. The caller logged `rc=0` and moved on.

### Root Cause

(pending)

### Contributing Factors

(pending)

### Why It Wasn't Caught

(pending)

### Blast Radius

(pending)

---

## Fix Direction

(pending — written when the fix discussion concludes)

---

## Notes

(none yet)
