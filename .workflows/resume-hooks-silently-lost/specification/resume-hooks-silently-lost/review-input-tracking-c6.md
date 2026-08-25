# Review Tracking: Resume Hooks Silently Lost - Input Review

## Findings

### 1. The staleness test has three readers and no named home

**Source**: `investigation/resume-hooks-silently-lost.md` — Analysis / Code Trace, agreed trace line 3: *"`cmd/run_hook_stale_cleanup.go` -> `internal/hooks/store.go` `StaleKeys`/`CleanStale`, and the daemon call site — the reaper, its guards, what it logs."* Also Fix Direction / "C reduced": *"it **retains any key it cannot parse as a token**"*, and the Refinements' framing that the protection holds *"wherever the rule lives"*.
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §5.2 (primary); §5.4 and §6.3 read from it

**Problem**:
"Stale" now means *token-shaped and absent from the live set* — a compound test with three separate readers: the sweep's decision that there is anything to do (§6.3's call-site snapshot), the deletion itself (§5.2, derived under the lock in §6.3), and `portal doctor`'s count (§5.4). The specification states the right answer at each of the three and never says where the test lives, so it is built three times. The source names the store's own staleness query — `StaleKeys`, sitting beside `CleanStale` on the reaper's path — and the specification does not mention it anywhere, so it is the one key-consuming function in the reaper whose fate is unstated: it can ship still applying the old "absent from the live set" rule while the two places the specification does describe apply the new one. The failure that follows is quiet and lands on exactly the entries this work unit exists to protect: a `portal doctor` run that counts a retained old-format entry as stale, or a sweep whose pre-read reports work that the locked derivation then declines to do. §3.2 goes to real trouble to make the shape test underivable-twice inside `internal/session`; that care is wasted if the *use* of it is copied into three call sites.

**Proposal**:
The shape test has one home — the store's own staleness query — and every reader inherits it rather than restating it. This is §5.2's own argument ("the protection travels with the rule, so it holds wherever the rule lives... there is no 'guard at the daemon call site versus inside `CleanStale`' split") applied to the function the source names alongside `CleanStale`, and it is the same single-home reasoning §2.1 applies to the option literal and §3.2 to the predicate.

**Current**:
> - An **empty** key is deleted. It is neither token-shaped nor old-format — an old-format key always carries `:` and `.` (§3.2) — so the retention rule has nothing to protect in it. It is the malformed entry §3.4 guards the firing path against, and deletion is its only route out of the file short of a hand edit.
>
> The justification is that A removes the reaper's false positives: the indistinguishability §1.1 names is what had it acting correctly on false information.

**Proposed Text**:
Insert as a new paragraph in §5.2 between the three bullets and the paragraph beginning "The justification is that A removes the reaper's false positives":

> **The rule has one implementation.** `internal/hooks`'s `StaleKeys` — the read-only staleness query that sits beside `CleanStale` on the sweep's path — carries the shape test, and every reader of staleness reaches it through that one query rather than restating the rule. `CleanStale` derives its own delete set under its own lock (§6.3) by the identical test, and `checkStaleHooks`'s count (§5.4) is that test again, not a second reading of it. Three call sites applying the rule from three copies is how the retention protection comes to hold in one of them and not another — the same drift §3.2 removes from the predicate itself by deriving it from the generator's constants.

**Resolution**: Pending
**Notes**:

---

### 2. The reaper names a token that identifies nothing

**Source**: Specification analysis against `investigation/resume-hooks-silently-lost.md` — the source decides both halves separately (Fix Direction A: *"The key becomes the pane token alone... readability is recoverable by rendering the resolved location in `portal hook list`"*; Fix Direction C: *"it names the deleted key at INFO rather than only counting it"*) and never weighs the second against the first. The `value` attr is evidenced in the source's own log census: *"2026-07-27T16:37:29 (`value="cd "/Users/leeovery" && claude --resume 9e4d…"`, v0.10.3 — a real registration lost)"*.
**Category**: Gap/Ambiguity
**Move**: choice
**Affects**: §5.3

**Problem**:
§5.3 exists so the log can answer "what did I lose?" after the fact. After the key change the answer it gives is a six-character token. At the moment of deletion that token names nothing recoverable: the pane is by definition absent from the live enumeration, so there is no session, no window, no directory and no command to resolve it against — and the entry carrying the command is the thing being deleted. The operator is left doing exactly the reconstruction §5.3 names as the failure — correlating the removal line against an earlier registration breadcrumb elsewhere in the log — only now with a token instead of a bare count. The specification already recognises this shape one section earlier: §2.4 gives the restore re-stamp WARN a `pane_key` precisely because "the token that failed to land is not a location". The reaper's line has no equivalent, and §4.4's readability repair does not reach it — `hook list` renders a location for entries that still exist, which a reaped entry does not.

**Options**:
- Carry the removed entry's `on-resume` command on the INFO line alongside `hook_key`, using the `value` attr the `hooks` component already emits on `op=set` — the store holds the command at the moment it deletes it, it is the thing the user actually lost, and it costs no new attr key (recommended).
- Leave the line naming the token alone and accept that recovering what was lost means correlating against the registration breadcrumb — cheaper and quieter in the log, at the cost of §5.3 delivering a weaker answer than it claims.

**Resolution**: Pending
**Notes**:

---

### 3. `ResolveHookKey`'s doc comment documents a fallback that no longer exists

**Source**: `investigation/resume-hooks-silently-lost.md` — Code Trace finding 1: *"`ResolveHookKey`'s doc comment already warns that a *read failure* must never fall back to a name-based key, but tmux never reports this as a failure, so the guard never engages"*; repeated in Contributing Factors.
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §3.3

**Problem**:
The specification is careful about every other text surface that would otherwise ship describing the retired key scheme — the `AllPaneLister` doc comment, `CLAUDE.md`, the `--hook-key` help text — but omits the one the source singles out. `ResolveHookKey`'s own doc comment warns against falling back to a name-based key on a read failure. After §7.2 there is no name-based key to fall back to, and the failure the comment guards against is now a real non-zero exit rather than a case tmux declines to report — so the comment survives a rewritten body asserting a guarantee about machinery that has been deleted, on the one function whose documented contract should have caught this bug and could not.

**Proposal**:
Fold the correction into §3.3's existing `ResolveHookKey` bullet, alongside the other surface changes, so the doc surfaces the specification enumerates are complete.

**Current**:
> - `ResolveHookKey(paneID)` becomes the two-call live resolution of §4.1 for one pane target — an existence probe followed by a read of the pane's token.

**Proposed Text**:
> - `ResolveHookKey(paneID)` becomes the two-call live resolution of §4.1 for one pane target — an existence probe followed by a read of the pane's token. Its doc comment, which warns that a read failure must never fall back to a name-based key, is rewritten with it: §7.2 leaves no name-based key to fall back to, and the failure it guarded against is now a non-zero exit the function genuinely returns rather than one tmux declines to report.

**Resolution**: Pending
**Notes**:
