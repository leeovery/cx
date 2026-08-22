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
  6 of 42 panes running Claude with no hook registered. *(Qualified 2026-08-22.)* That count is
  **not wholly attributable to the Portal defects in scope** — the caller-side subagent
  SessionEnd clobber (out of scope, fixed in dotfiles `26fded9` the same day) and an ordinary
  SessionEnd removal on a live pane both produce the same end state. It evidences that panes go
  unprotected at a material rate; it does not size this bug. The 2026-06-29 reboot instance is
  separately explained and out of scope (below).
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
  (`cmd/hooks.go:37`) passes it through, and `store.Set` writes it. No validation at any layer.
  **Confirmed** by full trace — see Code Trace finding 1. A repo-wide search finds no hook-key
  shape validation anywhere.

- **H2 — Coordinate drift orphans a live pane's hook, and the daemon then reaps it** [confirmed]
  The key bakes `window_index.pane_index`; tmux renumbers on pane close, window close (with
  `renumber-windows on`), `break-pane` and `move-pane`. Nothing re-keys the entry.
  `runHookStaleCleanup` -> `hooks.CleanStale` then removes it, correct by its own rule.
  Evidence: the two named instances in the seed (pane demonstrably alive, only its coordinates
  changed), plus a direct sandbox reproduction. The 13 `clean-stale` prunes of 08-17..08-21 are
  *consistent* with this but prove nothing on their own — see the correction in Standing
  Evidence.

- **H3 — The identity design is half-finished, and that is the single root cause** [suspected]
  `@portal-id` (spec `session-rename-orphans-resume-hook`, 2026-07-04) made the *session* half
  of the key immutable. The *pane* half is still a position. H1 and H2 are the same missing
  piece seen at two moments: identity unverified at write, identity unstable over the lifetime.

- **H4 — Whatever replaces coordinates must survive a tmux server restart** [confirmed, and satisfiable]
  Spec `resume-hooks-lost-on-server-restart` (2026-04-30) chose positional keys *deliberately*,
  replacing pane IDs, because `%N` is reassigned by the server on restart. Drift is the bill for
  that trade. Any fix has to pay the restart problem back, not re-open it. **Confirmed as the
  binding constraint, and shown satisfiable** — see finding 5: a tmux *pane* user-option
  survives every in-server rearrangement, and the reboot gap is closed the same way the session
  half already closes it (carry in `sessions.json`, re-stamp at restore).

- **H6 — Restore renumbers windows, orphaning the key it just fired** [confirmed]
  Raised by root-cause validation, 2026-08-22; independently reproduced. A third occurrence of
  the same defect, at a moment the original three hypotheses did not cover. See finding 6.

- **H7 — `hooks.json` has no cross-process locking; a lost update loses hooks** [confirmed
  as present; unquantified as a cause]
  Raised by root-cause validation, 2026-08-22. Verified: no lock of any kind in
  `internal/hooks` or `internal/fileutil`. A separate loss path that survives the fix as
  framed. See finding 7.

- **H5 — The reaper and doctor are why this ran for months unnoticed** [confirmed]
  The daemon prune emits `clean-stale entries=N` with no key at INFO (per-key detail is DEBUG,
  `internal/hooks/store.go:220`). `portal doctor`'s stale-hook check (`cmd/doctor.go:289`)
  measures against the same live-key rule that does the reaping, so it reports green while
  hooks are being deleted. Contributing factor rather than root cause. **Confirmed** — see
  finding 4. The reaper runs every 10s, so by the time a human runs `doctor` the loss has
  already been normalised away; doctor is structurally incapable of seeing it.

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

#### Finding 2 — the four key-producing sites agree; the format is the problem (trace line 2)

Every site derives the key by the same rule, so there is no cross-site drift — the July
`session-rename-orphans-resume-hook` invariant holds:

| Site | Call | Source of truth |
|---|---|---|
| Registration | `cmd/hooks.go:50` -> `ResolveHookKey` | live tmux read |
| Stale sweep | `cmd/run_hook_stale_cleanup.go:26` -> `ListAllPaneHookKeys` | live tmux read |
| Doctor | `cmd/doctor.go:289` -> `ListAllPaneHookKeys` | live tmux read |
| Restore / firing | `internal/restore/session.go:62` -> `tmux.HookKey` | **saved** `sessions.json` |

The defect is in the format itself, `internal/tmux/tmux.go:565`:

```
#{?@portal-id,#{@portal-id},#{session_name}}:#{window_index}.#{pane_index}
└──────────── immutable identity ───────────┘ └──── mutable position ────┘
```

The session half was made rename-immune in July. The pane half is still a coordinate tmux
recomputes from layout. **H3 confirmed**: one root cause, two symptoms.

#### Finding 3 — how fast the reaper acts (H2 confirmed)

`hookCleanupInterval = 10 * time.Second` (`cmd/state_daemon.go:105`), run from
`maybeRunHookCleanup` on the daemon's **idle** branch (`cmd/state_daemon.go:174-180`) — i.e.
precisely when nothing else is happening, which is when a user is rearranging panes. So a hook
is reaped within ~10s of the pane moving. This is not slow decay; the entry is gone almost
immediately, minutes or months before the reboot that needed it.

`runHookStaleCleanup` carries exactly one guard — an empty live set is treated as a bad read
and skipped (`cmd/run_hook_stale_cleanup.go:41-47`). Nothing protects a *single* entry whose
pane merely moved, and nothing could: at the point of comparison a moved pane and a dead pane
are indistinguishable. The reaper is behaving correctly on false information.

Timing matches the seed's instance exactly: `OCCvED` registered 22:17:55, pruned 22:22:16 —
the pane was rearranged somewhere in that window and the next idle sweep took it.

#### Finding 4 — why nothing reported it (H5 confirmed)

Two independent blind spots:

- **The daemon's prune is count-only at production level.** `internal/hooks/store.go:220` logs
  each removed key at DEBUG; the only INFO line is the batch summary `clean-stale entries=N`
  (`internal/storelog/clean_stale.go`). At the production default the key that was deleted is
  never recorded, so the log cannot answer "what did I lose?".
  *(Corrected 2026-08-22.)* This is true of the **automatic** path only. `portal doctor --fix`
  routes through the same `runHookStaleCleanup` with an `onRemoved` callback that prints
  `Pruned stale hook: <key>` (`cmd/doctor.go:200-202`). The manual backstop names the key; the
  reaper that actually does the deleting does not.
- **`portal doctor` measures against the rule that does the reaping.** `checkStaleHooks`
  (`cmd/doctor.go:280`) asks "does every persisted key match a live pane?". The daemon has
  already deleted every entry that failed that test, within 10s. Doctor therefore reports
  `no stale hooks` *because* the loss completed, not despite it.

The deeper gap: **nothing anywhere asks the inverse question** — "is a live pane that should
have a hook missing one?". Portal holds no record of which panes are meant to be protected, so
an unprotected pane is invisible to every diagnostic. That is why the 2026-08-16 audit had to
be done by hand, diffing `hooks.json` against the live pane list.

#### Finding 5 — durable pane identity is available (H4 confirmed and satisfiable)

Sandbox, isolated `-L` socket, tmux 3.7c (note: `CLAUDE.md` says 3.6b — stale). A tmux **pane**
user-option `@portal-pane-id` was stamped on one pane and tracked through every mutation the
seed names:

| Operation | Key coords before -> after | Stamp |
|---|---|---|
| initial | `t:1.2` | `STAMP1` |
| `break-pane` | `t:1.2` -> `t:3.1` | survives |
| `kill-window` + `renumber-windows on` | `t:3.1` -> `t:2.1` | survives |
| `move-pane` back | `t:2.1` -> `t:1.2` | survives |
| **`respawn-pane -k`** | `t:1.2` (unchanged) | **survives** |
| `rename-session` | `t:1.2` -> `newname:1.2` | survives |

Four distinct hook keys for one pane that never changed; the stamp held throughout.
`respawn-pane -k` surviving is load-bearing — that is exactly what restore's arm phase does to
every pane (`internal/restore`), so a stamp applied before arming is still there afterwards.

Also verified: **no inheritance** — a pane created by `split-window` or `new-window` from a
stamped pane reads back empty, so a split cannot duplicate an id. `set-option -p -u` clears
cleanly.

The one thing a pane option cannot survive is the tmux server dying — which is the exact
constraint that made spec `resume-hooks-lost-on-server-restart` choose coordinates in 2026-04-30.
That gap is already solved once, for the session half: `sessions.json` carries `Session.PortalID`
and `internal/restore/session.go:76` re-stamps it on the recreated session. The pane half is the
same move one level down — `state.Pane` (`internal/state/schema.go:40`) takes an additive field
exactly as `Session.PortalID` did (`json` tolerant-decode, no `SchemaVersion` bump), capture
appends one more `captureFormat` column, and restore re-stamps per pane.

**So H4's constraint is real and is satisfiable without re-opening the 2026-04-30 bug.**

#### Finding 1 — the degenerate-key write path (H1 confirmed)

```
cmd/hooks.go:91   hooksSetCmd.RunE
  -> cmd/hooks.go:37   resolveCurrentPaneKey()
     -> cmd/hooks.go:25  requireTmuxPane()  — only checks TMUX_PANE is non-empty
     -> internal/tmux/tmux.go:236  Client.ResolveHookKey(paneID)
        -> internal/tmux/tmux.go:42   RealCommander.Run("display-message","-p","-t",paneID,HookKeyFormat)
           -> internal/tmux/tmux.go:50  runCommand -> exec.Cmd.Output()
              BUG: tmux exits 0 for an unresolvable pane target, printing ":."
                   so err is nil and the caller has no signal
  -> internal/hooks/store.go:81  Store.Set(":.", "on-resume", cmd, "cli")  — writes it
```

Every layer behaves correctly in isolation; the contract breaks at the tmux boundary. `Run`
maps a zero exit to `err == nil` (`internal/tmux/tmux.go:55`) — right for every other tmux
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

#### Finding 6 — restore renumbers windows and orphans the key it just fired (H6)

*Raised by root-cause validation (`root-cause-validation-001`), traced and reproduced
independently 2026-08-22. This answers agreed trace line 4, which the first analysis pass
listed and then never executed.*

Capture stores the pane's **real** `#{window_index}` (`internal/state/capture.go:26`). Restore
recreates each extra window with `NewWindow(target, …)` where `target` is `"<session>:"` and
**no index is passed** (`internal/restore/session.go:95`, `internal/tmux/tmux.go:676`), so tmux
assigns the next free index rather than the saved one.

Whenever the saved window indices are non-contiguous, the restored ones do not match.
Sandbox reproduction (`tmux -f /dev/null`, pristine config — `renumber-windows off`, tmux's
default):

```
build windows          0 1 2
kill middle window     0 2      <- what capture saves
                                   saved hook key = t:2.0
restore replay:
  new-session -d -s t
  new-window -t 't:'   0 1      <- what restore recreates
  live pane keys:      t:0.0  t:1.0
  => nothing answers to t:2.0
```

The consequence is a distinct third moment in a hook's life:

1. The hook **fires correctly** on the first reboot — `collectArmInfos`
   (`internal/restore/session.go:62`) bakes the key from saved state, so firing is a pure
   function of `sessions.json` and is unaffected by the live renumbering.
2. Once `@portal-restoring` clears, the daemon's 10s sweep enumerates live keys, finds nothing
   answering to the saved key, and reaps the entry.
3. The **second** reboot has no hook.

This is exactly the shape recorded in the project's own standing note — *reboot hooks DO fire;
the real issue is hooks going missing between reboots* — and it had never been connected to a
mechanism.

**Why it has not dominated this user's evidence:** their tmux config runs `renumber-windows on`,
which keeps window indices contiguous, so saved and restored indices agree. It is a general
defect on tmux's default settings, latent here.

**The pane half is not affected.** `pane_index` is always contiguous within a window (tmux
recomputes it from layout on every close), and restore recreates panes by repeated
`SplitWindow` in saved order, so pane indices come back identical.

**What it changes about a fix.** Making pane identity durable is necessary but not sufficient:
at restore, the entry on disk is keyed by the *old* coordinates, so `hooks.json` itself has to
be re-keyed — or keyed off something restore re-establishes — rather than only re-stamping
identity onto tmux. A fix that stops at "stamp a durable id" still loses the hook here.

#### Finding 7 — `hooks.json` has no cross-process locking (H7)

*Raised by root-cause validation, verified independently 2026-08-22.*

`grep` for `sync.`/`Mutex`/`flock`/`Lock()` across `internal/hooks` and `internal/fileutil`
returns nothing. `Store.Set`, `Store.Remove` and `Store.CleanStale` are each `Load()` → mutate
in memory → `fileutil.AtomicWrite`. `AtomicWrite` makes each *write* atomic; nothing guards the
read-modify-write window, and the writers are in **different processes** — the CLI
(`portal hook set`, fired by a Claude SessionStart) against the daemon's sweep every 10s.

```
daemon  CleanStale  t0    Load() -> 41 entries
CLI     hook set    t0+e  Load() -> 41, add K, AtomicWrite -> 42
daemon             t0+d  writes its t0 snapshot minus stale -> 40      K is gone
```

The end state is indistinguishable from the drift symptom: an INFO `hooks: set … hook_key=K`
breadcrumb, K absent minutes later, and the intervening `clean-stale entries=1` naming a
*different* key even at DEBUG.

**Status: present, not quantified.** The window is two file reads plus a marshal, so per-event
probability is low — but the daemon sweeps ~8,640 times a day against a 40+ pane working set,
and the same race exists CLI-against-CLI (a SessionStart in one pane concurrent with a
SessionEnd in another). The two named instances have ~4-minute gaps between registration and
prune, which rules a race out *for them*; it is not ruled out for any other instance.

**It is untouched by the key fix.** Durable identity does not close a lost update. Whether it is
in scope is a fix-direction decision, not a diagnosis one.

### Root Cause

**The hook key is not a durable, verified reference to a pane.** It is half identity and half
position, and Portal never checks that a key it writes down identifies anything at all.

`internal/tmux/tmux.go:565`:

```
#{?@portal-id,#{@portal-id},#{session_name}}:#{window_index}.#{pane_index}
└──────────── immutable identity ───────────┘ └──── mutable position ────┘
```

The reported symptoms are that one defect observed at three moments in a hook's life:

- **At write time** — tmux expands the format against an unresolvable target and exits **0**
  with `:.`. `Run` maps a zero exit to `err == nil`, and no layer validates shape, so a hook
  that identifies no pane is persisted and reported as success.
- **Over the pane's lifetime** — tmux recomputes `window_index.pane_index` from layout. Any
  rearrangement changes the key, nothing re-keys the entry, and the daemon's sweep — which
  cannot distinguish a moved pane from a dead one — removes it, correctly by its own rule.
- **At the reboot boundary** — restore recreates windows without passing the saved index, so
  tmux renumbers them. The baked key still fires (it is a pure function of saved state), but no
  live pane answers to it afterwards, and the sweep reaps it within ~10s. The hook survives
  exactly one reboot. See finding 6.

**Why this happens:** a hook is a promise attached to one pane, but the thing Portal writes
down to name that pane is derived from where the pane currently sits rather than from what it
is — and *where it sits* is recomputed by tmux on layout change, and reassigned again by Portal
itself at restore. `hooks.json` is written once at registration and never revisited, so any value in the key
that tmux is free to recompute rots the moment it changes. Half of the key was already fixed —
`@portal-id` (spec `session-rename-orphans-resume-hook`, 2026-07-04) made the session component
immune to renames for exactly this reason. The pane component was left positional, so the same
class of bug persisted one level down.

### Contributing Factors

- **tmux reports success for an unresolvable pane target.** `display-message -p -t %999 <fmt>`
  exits 0 printing `:.` (tmux 3.7c, sandbox-verified). There is no error for Portal to detect,
  so `ResolveHookKey`'s own documented guard — never synthesise a key on read failure — never
  engages, because this is not a read failure as far as tmux is concerned.
- **The reaping cadence is 10 seconds, on the daemon's idle branch.** `hookCleanupInterval`
  (`cmd/state_daemon.go:105`) fires precisely when the user is rearranging panes rather than
  working, so the gap between drift and deletion is negligible.
- **`renumber-windows on`** in the user's tmux config widens the drift surface: closing any
  window renumbers every later one, moving panes that were never touched.
- **The 2026-04-30 constraint was never revisited.** Spec `resume-hooks-lost-on-server-restart`
  chose positional keys deliberately, because pane IDs (`%N`) do not survive a server restart.
  That reasoning was sound and remains true; what changed since is that Portal gained a restore
  layer able to carry and re-apply identity across the reboot gap (`Session.PortalID`), which
  removes the constraint that forced positions.
- **The registering caller cannot verify its own success.** `~/.claude/hooks/portal-resume-hook.sh`
  reads `rc` from `portal hooks set`, which is 0 on both paths.

### Why It Wasn't Caught

- **The `:.` behaviour was found and then filed as a testing obstacle, not a bug.**
  `internal/tmux/resolve_hookkey_realtmux_test.go:73` states it outright: *"tmux 3.7's
  display-message tolerates a bogus -t target (it returns `:.` with exit 0), so killing the
  server first is the only reliable way to drive the read-failure path."* The test then kills
  the server to force a genuine error — testing the path that does work, and routing around the
  one that does not. The knowledge was in the repo the whole time, framed as an inconvenience.
- **No test moves a pane.** The cross-site consistency tests
  (`internal/tmux/hookkey_cross_site_realtmux_test.go`) prove every site derives the *same* key
  for a pane at rest. Nothing registers a hook, rearranges the pane, and re-reads — the exact
  sequence that breaks.
- **Both diagnostics ask the reaper's own question.** `portal doctor`'s `checkStaleHooks`
  (`cmd/doctor.go:280`) and the daemon sweep apply the same live-key rule; the daemon has
  already acted on it within 10s, so doctor reports `no stale hooks` *because* the deletion
  completed.
- **The prune records the count, not the key.** Per-key detail is DEBUG
  (`internal/hooks/store.go:220`); production INFO gets `clean-stale entries=N`. The log cannot
  answer "what did I lose?" after the fact.
- **Nothing records intent.** Portal holds no notion of which panes are *meant* to be protected,
  so an unprotected pane is invisible to every check. The 2026-08-16 audit that found 6 of 42
  panes hookless had to be done by hand.

### Blast Radius

**Directly affected:**

- `hooks.json` entries and the `on-resume` promise they carry — the user-visible loss.
- `portal hook set` / `portal hook rm` (`cmd/hooks.go`) — both accept a key identifying nothing
  and report success.
- The daemon stale sweep (`cmd/run_hook_stale_cleanup.go`, `internal/hooks/store.go`).
- `portal doctor`'s stale-hook check and `--fix` prune (`cmd/doctor.go:280`).
- Hook firing at restore (`internal/restore/session.go:62`, `portal state hydrate`).
- `internal/state` capture/schema and `internal/restore` — the carrier for any durable pane
  identity, additive alongside `Session.PortalID`.

**Shares the pattern, not currently failing:**

- `state.SanitizePaneKey(session, window, pane)` keys the hydrate FIFO paths and the
  `@portal-skeleton-*` markers off the same positional addressing. These exist only for the
  duration of one bootstrap and are rebuilt from live coords each time, so drift has no window
  in which to occur — but the addressing assumption is identical, and a change to the pane-key
  concept should be checked against them rather than assumed separate.
- `internal/tmux` `StructuralKeyFormat` / `ResolveStructuralKey` / `ListAllPanes` — the
  name-based positional siblings, now used only for the marker/cleanup paths above.

**Added after root-cause validation (2026-08-22):**

- **The 41 existing on-disk `hooks.json` keys.** Every live entry is keyed `<portal-id>:w.p`;
  changing the pane half changes every key on disk. Precedent matters here: the 2026-04-30 spec
  accepted a breaking key change and used `CleanStale` *as* the migration
  (`resume-hooks-lost-on-server-restart/specification.md:67`). Repeating that would silently
  destroy all 41 hooks on the first sweep after upgrade. Migration is a fix-direction question
  that must be answered, not assumed.
- **A fifth key-producing site, outside this repo.** `~/.claude/hooks/portal-resume-hook.sh:93-95`
  re-implements `HookKeyFormat` verbatim and matches it against `portal hook list`'s
  tab-separated output. The script documents the coupling and fails safe (an unrecognised scheme
  yields empty, and its guards then refuse to remove anything) — so a key-scheme change degrades
  it silently rather than erroring. Any change to the key rule or to `hook list` output has to be
  assessed against that file.
- **`buildHydrateCommand` (`internal/restore/session.go:141`).** The key is interpolated into
  `portal state hydrate --hook-key %s` via `shellQuoteSingle` and baked into a `respawn-pane -k`
  command line. A change to the key's character set or length lands on this quoting boundary.
- **`internal/transienttest` `SeedHooksJSON` / `HooksJSONBytes`.** The single-sourced hook seeder
  for the two destructive integration suites; a key-shape change must route through it.
- **`hooks.json` concurrency (finding 7).** Unlocked read-modify-write across processes — in the
  blast radius of any fix that adds a writer or changes write frequency.

**Not affected:** scrollback capture and replay (positions are re-derived live each restore),
`sessions.json` correctness (regenerated from live tmux every tick), session grouping, spawn.

---

## Fix Direction

(pending — written when the fix discussion concludes)

---

## Notes

(none yet)
