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

### Folded-in Sighting (unconfirmed fit)

The 2026-06-29 capture — after a crash-recovery reboot with 32 Claude sessions, ~28 resumed
and ~4 did not, tmux sessions present but on-resume behaviour absent — was folded in as a
symptom-level sighting of the same failure. The user's hunch at the time was that the
non-resuming ones were older sessions; an older pane has had more opportunity to be
rearranged, so its key drifts and the sweep reaps it, which matches both the minority-subset
shape and the age correlation. **Residual risk carried deliberately:** if those four panes
failed for a different reason — legacy sessions missing markers the restore path expects,
which that capture also floats — then an unrelated symptom is being carried through this
fix. The investigation must say so rather than force the fit.

### Known-adjacent, explicitly out of scope

The subagent-SessionEnd clobber in `~/.claude/hooks/portal-resume-hook.sh` (a subagent's
SessionEnd deletes the pane's real resume hook; needs a `session_id` guard) is a separate
failure mode on the caller side, not in Portal.

### References

- `seeds/2026-08-16-hook-set-persists-degenerate-pane-key.md`
- `seeds/2026-08-21-resume-hooks-vanish-when-pane-moves.md`
- `seeds/2026-06-29-some-sessions-skip-resume-on-reboot.md`
- `.workflows/resume-hooks-silently-lost/discovery/sessions/session-001.md`

---

## Analysis

### Hypotheses

**Checkpoint depth:** {straight-through | check-ins}

(to be established at the investigation plan)

### Code Trace

(pending)

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
