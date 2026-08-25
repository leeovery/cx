# Review Tracking: Resume Hooks Silently Lost - Gap Analysis

## Findings

### 1. An empty key can never be deleted under the delete-set rule

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §6.3 (the stale decision under the exclusive lock), §5.2 (deletion becomes shape-aware)

**Problem**:
The reaper is told two incompatible things about an empty key. §5.2 makes deleting it the rule — an empty key is "neither token-shaped nor old-format", and "deletion is its only route out of the file short of a hand edit". §6.3 then defines the delete set as keys that are token-shaped and absent from the live set; an empty key is not token-shaped (§3.2 requires exactly `suffixLen` characters), so a builder implementing §6.3 literally never deletes one. Whichever rule wins, the other is dead text: either an empty entry parks in `hooks.json` permanently with no route out but a hand edit, or the delete set is wider than the section that defines it says. The empty entry is the one the firing path needs two guards against (§3.4), so leaving it resident forever is the outcome §5.2 set out to prevent.

**Proposal**:
§5.2 owns the deletion rule and states three cases; §6.3 operationalises it and carries only two. Widen §6.3's delete-set definition to admit the empty key it already agrees is malformed. The empty key is trivially absent from the live set (which holds non-empty tokens only, §5.4), so the liveness and snapshot conditions need no change.

**Current**:
The delete set is every key that is in the file under the lock **and** in the call-site snapshot **and** token-shaped **and** absent from the live set. The call-site read may narrow that set; it may never widen it.

**Proposed Text**:
The delete set is every key that is in the file under the lock **and** in the call-site snapshot **and** absent from the live set **and** either token-shaped or empty (§5.2). The call-site read may narrow that set; it may never widen it.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. Self-inflicted: cycle 5's empty-key deletion rule collided with cycle 4's delete-set definition, which required token-shaped — so the malformed entry the firing path guards against twice would have parked in the file permanently.

---

### 2. Whether `CleanStale` calls `StaleKeys` or re-derives the rule is unresolved, and one reading stalls the sweep

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §5.2 (the rule has one implementation), §6.3 (locks are never nested; the stale decision under the exclusive lock)

**Problem**:
§5.2 says every reader of staleness reaches the rule "through that one query" — `StaleKeys`, the exported read-only query — and in the same breath says `CleanStale` "derives its own delete set under its own lock ... by the identical test". Those are two different builds. A builder who takes the first reading has `CleanStale` call `StaleKeys` while holding the exclusive lock; §6.3 says a lock is never nested and a second acquisition from the same process blocks against the caller's own hold until the acquisition bound, and §6.5 says a write that cannot take the lock does not write — so the sweep would stand down on every cycle for the life of the install, and no stale entry would ever be pruned again. A builder who takes the second reading has the rule written twice, which is the drift §5.2 exists to prevent. Nothing in the specification says which build is intended.

**Proposal**:
Both stated goals — one implementation of the rule, and no nested acquisition — hold only if the single home is a lock-free function over an already-loaded key set, with the exported query and the under-lock derivation as its two callers. That is the one arrangement that satisfies §6.3's non-reentrancy rule and §5.2's single-implementation rule together, so it is the reading the specification's own constraints leave standing.

**Current**:
**The rule has one implementation.** `internal/hooks`'s `StaleKeys` (`internal/hooks/store.go:184`) — the read-only staleness query that sits beside `CleanStale` on the sweep's path — carries the shape test, and every reader of staleness reaches it through that one query rather than restating the rule. Both already do: `CleanStale` consumes it at `:208`, and `checkStaleHooks` at `cmd/doctor.go:299`. `CleanStale` derives its own delete set under its own lock (§6.3) by the identical test, and `checkStaleHooks`'s count (§5.4) is that test again, not a second reading of it. Three call sites applying the rule from three copies is how the retention protection comes to hold in one of them and not another — the same drift §3.2 removes from the predicate itself by deriving it from the generator's constants.

**Proposed Text**:
**The rule has one implementation.** The staleness test lives in a single unexported function in `internal/hooks` that judges an already-loaded key set and takes no lock of its own, and every reader of staleness applies the rule through that one function rather than restating it. `StaleKeys` (`internal/hooks/store.go:184`) — the exported read-only query that sits beside `CleanStale` on the sweep's path, consumed by `checkStaleHooks` at `cmd/doctor.go:299` — is that function behind the shared lock. `CleanStale` calls the same function directly, on the key set it loaded under its own exclusive lock (§6.3), and never through `StaleKeys`: an acquisition from inside the exclusive hold is not re-entrant (§6.3), so it would block against the sweep's own lock to the §6.5 bound and stand the prune down on every cycle. Three call sites applying the rule from three copies is how the retention protection comes to hold in one of them and not another — the same drift §3.2 removes from the predicate itself by deriving it from the generator's constants.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. Also self-inflicted, from this cycle's own input-review finding 1: as written, `CleanStale` calling the exported `StaleKeys` from inside the exclusive hold would block against its own lock to the 2s bound, and since a write that cannot take the lock does not write, the prune would stand down on every cycle for the life of the install. The lock-free rule function with two callers is the only arrangement satisfying both stated constraints.

---

### 3. A sweep that stands down has no route to `portal doctor --fix`'s output

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §5.4 (the sweep is suppressed for the duration of a restore), §6.5 (a timeout degrades rather than wedges), §5.1 (what the reaper does now)

**Problem**:
The specification requires `portal doctor --fix` to tell the user when the prune did not run — once for a restore window (§5.4) and once for a lock timeout (§6.5) — but the sweep has no way to say so. Its only channel to the caller today is the per-removed-key `onRemoved` callback (§5.1), which by definition never fires on a cycle that deleted nothing, and a skip is indistinguishable at the call site from a clean run over a file with nothing stale in it. A builder therefore has to invent both the reporting seam and the words that reach the user, and the two skip reasons will be reported through whatever each half of the work lands on. The user-visible consequence is the one §6.5 names as unacceptable: someone asks for a repair, the repair does not run, and `doctor --fix` prints its usual nothing.

**Proposal**:
The prune-skipped report needs the same shape the prune-performed report already has: a callback the caller supplies, carrying the reason, printed on one line beside `Pruned stale hook: <key>`. The daemon call site supplies none and keeps its WARN/DEBUG. The third reason the sweep already stands down for — the empty-live-set defer (§5.4) — travels the same seam, because §6.5's rationale for naming a skipped repair does not distinguish between the reasons it was skipped.

**Proposed Text**:
Add to §5.1, after the paragraph describing the two call sites:

`runHookStaleCleanup` reports a stood-down cycle to its caller the way it reports a removal: alongside `onRemoved`, the caller may supply an `onSkipped` callback taking the reason the cycle did not run. The daemon supplies none — its skip is already in the log. `portal doctor --fix` supplies one and prints its line beside the `Pruned stale hook: <key>` lines, one of:

```
Skipped stale hook prune: restore in progress
Skipped stale hook prune: hooks.json is locked
Skipped stale hook prune: could not read live panes
```

covering the restore marker (§5.4), the lock timeout (§6.5) and the empty-live-set guard (§5.4) respectively. None of them affects the exit code, which stays driven by the post-repair diagnosis.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. The spec required `doctor --fix` to report a skipped prune in two places and gave the sweep no channel to say so — `onRemoved` cannot fire on a cycle that removed nothing, and a skip is indistinguishable at the call site from a clean run.

---

### 4. The log cannot say why the sweep stood down

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: choice
**Priority**: Important
**Affects**: §5.4 (the sweep is suppressed for the duration of a restore), §6.5 (what each of those emits)

**Problem**:
The sweep now stands down for three distinct reasons — a restore window, a lock timeout, a bad tmux read — and the specification gives the restore skip no identity of its own: it "emits one DEBUG line on the shape the empty-live-set guard's WARN already uses", and §6.5's accounting closes the door on a distinguishing `op` by declaring the whole amendment to be two `op` values and no new attr key. An operator raising the level to investigate a hook that vanished is then looking at a line that does not say which of the three happened, on the one subsystem this work unit exists to make legible. The specification does not say whether the skip is meant to be identifiable in the log at all.

**Options**:
- Give the restore skip its own `op` value (e.g. `clean-stale-skipped`, carrying the reason), and correct §6.5's accounting to three `op` values *(recommended)*
- Keep `op=clean-stale` and distinguish the skip by message text alone, leaving §6.5's accounting as written
- State outright that the skip is not separately identifiable in the log, and rely on `portal doctor --fix`'s output (finding 3) as the only signal a prune did not run

**Resolution**: Approved
**Notes**: Option 1 chosen. Verified `reason` is an existing attr key in the closed vocabulary (`internal/theme/events.go:83,102,121`, `internal/tmux/portal_saver.go:171,207`), so this adds one `op` value and reuses an existing key. All three stand-down reasons now share one line shape with `reason` naming which; the restore skip stays DEBUG and the other two keep their WARN. Accounting corrected to three `op` values, with a note that `reason` and `value` are existing keys newly carried by `hooks` rather than additions.

---

### 5. `hook rm`'s "removed nothing" messages are unwritten

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Minor
**Affects**: §4.2 (removal verifies the same way), §4.3 (`--pane-key` stays a literal pass-through)

**Problem**:
`hook rm` now fails in three ways that all mean "nothing was removed", and two of them speak in Portal's own words — a live pane carrying no token, and a live stamped pane whose token has no entry — plus the `--pane-key` key that names no entry (§4.3). The specification fixes the exit status and names the cases but writes none of the words, while specifying the copy elsewhere on this surface exactly (`Pruned stale hook: <key>`, the `hook list` column). Since §4.2 establishes that this failure fires routinely — near-daily, on every SessionEnd against a closed pane — the line the user reads is the one thing standing between a non-zero exit and the impression that something broke.

**Proposal**:
Two messages cover the three cases: one for a pane that has never been stamped, and one shared shape for a key that names no entry, differing only in whether the key was resolved or handed over. The gone-pane case already has its words — tmux's, passed through unaltered (§4.1).

**Proposed Text**:
Append to §4.2, after the paragraph ending "…an exit code that says nothing about whether anything happened.":

The words are fixed. A live pane carrying no token exits with `no resume hook registered for this pane`. A key that names no entry exits with `no resume hook registered for <key>` — the resolved token on the `$TMUX_PANE` path, the literal key on the `--pane-key` path (§4.3). A gone pane exits with tmux's own words (§4.1).

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. The failure fires near-daily on SessionEnd, so the line the user reads is what separates a non-zero exit from the impression something broke.

---

### 6. §9.2 leaves two of the specification's own guarantees uncovered

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Minor
**Affects**: §9.2 (new tests)

**Problem**:
Two rules the specification argues for at length have no test row, so nothing in the plan produces work for them. §5.3 makes the case that the token alone answers nothing at the moment of deletion and that the reaped entry's command must ride the INFO line in `value` — the test row asks only that the deletion "names the key". And §2.2 makes a point of the dirty-flag touch never affecting the exit status, which is exactly the kind of best-effort rule an implementer collapses into a returned error; no row holds it. Both are cheap to assert and both are load-bearing: the first is what makes a reaped hook recoverable from one log line, the second is what keeps a successful registration from being reported as a failure.

**Proposal**:
Amend the reaper row to name the command, and add a row for the touch. Both are unit-lane, on surfaces §9.2 already drives.

**Current**:
| **Reaper shape-awareness** | An old-format (non-token) key is retained by both the daemon sweep and `portal doctor --fix`; a token-shaped key whose token is absent is still deleted; the deletion names the key at INFO rather than only counting it (§5.2, §5.3). | unit |

**Proposed Text**:
| **Reaper shape-awareness** | An old-format (non-token) key is retained by both the daemon sweep and `portal doctor --fix`; a token-shaped key whose token is absent is still deleted; the deletion names the key **and the removed entry's command** at INFO rather than only counting it (§5.2, §5.3). | unit |
| **A failed dirty-flag touch does not fail `hook set`** | With the state directory unresolvable, `hook set` still exits 0 with the entry written, and emits the WARN under `op=touch-save-requested` (§2.2). | unit |

**Resolution**: Approved
**Notes**: Applied verbatim as proposed, both rows. Both assert rules the spec argues at length and nothing was producing work for.

---

### 7. The no-rollback rationale is stated twice

**Source**: Specification analysis
**Category**: Duplication
**Move**: settled
**Priority**: Minor
**Affects**: §4.2 (removal verifies the same way)

**Problem**:
Why an orphaned token is left alone has its home in §4.1: the orphan costs nothing and the next registration on that pane reads it back and reuses it. §4.2 cites §4.1 for that reason and then restates it word for word anyway, so the same argument sits in two places and only one of them is the home. The copies agree today; an edit to §4.1's reasoning leaves §4.2 arguing the old case under a cross-reference that claims otherwise.

**Proposal**:
Keep the cross-reference and the one reason §4.2 adds that §4.1 does not have — that clearing the token adds a tmux write that can fail after the entry is already gone — and drop the restatement.

**Current**:
A pane whose entry is removed keeps its token, for the reason §4.1 gives for not rolling back a stamp: the orphan costs nothing, the next registration on that pane reads it back and reuses it, and clearing it would add a tmux write that can fail after the entry is already gone.

**Proposed Text**:
A pane whose entry is removed keeps its token, for the reason §4.1 gives for not rolling back a stamp, and for one more: clearing it would add a tmux write that can fail after the entry is already gone.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed.

---

### 8. The current logging shape of the prune is described in two sections

**Source**: Specification analysis
**Category**: Duplication
**Move**: settled
**Priority**: Minor
**Affects**: §1.1 (the defect), §5.3 (the reaper names what it deleted)

**Problem**:
That the automatic prune records the removed key only at DEBUG while production INFO gets the batch count alone is stated in §1.1 and again in §5.3, both citing `internal/hooks/store.go:220`. §5.3 is the home — it needs the exact current shape to specify the promotion, and it carries the verbatim call and the summary emitter. §1.1's copy carries the same claim and the same source line with less detail, so a change to that line's level or shape has to be found in two places, and the defect narrative goes on describing a logging arrangement the change section has since redefined.

**Proposal**:
§1.1 needs the consequence — the loss is unrecorded at the level an operator runs at — not the mechanism. Leave the consequence there and let the citation and the level detail live once, in §5.3.

**Current**:
and the automatic prune logs the removed key only at DEBUG (`internal/hooks/store.go:220`), with production INFO getting the batch count alone.

**Proposed Text**:
and the automatic prune does not name the removed key at the level production runs at (§5.3).

**Resolution**: Approved
**Notes**: Applied verbatim as proposed.

---
