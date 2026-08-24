# Review Tracking: Resume Hooks Silently Lost - Gap Analysis

## Findings

### 1. A hook registered mid-sweep is still deleted by the sweep

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: choice
**Priority**: Critical
**Affects**: §6.3, §6.4, §9.2 (Lost update row)

**Problem**:
A hook can be deleted within seconds of `portal hook set` reporting success — the exact silent loss this work unit exists to remove — and the specification as written produces that outcome rather than preventing it.

The sweep's live token set is read from tmux *before* any lock is taken (§6.4: no tmux call may sit inside the lock), and §6.3 has `CleanStale` derive the delete set under the lock *from that earlier read*. So a `hook set` that stamps its pane after the enumeration and writes its entry before the sweep acquires the lock produces an entry whose token is absent from the set the sweep is judging against. It is token-shaped, it is not in the live set, so §5.2's rule deletes it. Taking the lock makes this more likely rather than less: a `hook set` that wins the lock first guarantees the sweep's mutation lands after its write.

§6.3 closes only half of the interleaving it diagrams. Re-reading `hooks.json` under the lock makes the sweep *see* the new entry; it does not make the sweep *believe* it, because the evidence the entry is judged against is the pre-lock tmux read. §9.2's Lost update row asserts the outcome the design does not deliver — "an entry registered between the sweep's call-site read and its locked mutation survives" — so an implementer will either write a test that passes only because its fake enumeration was pre-seeded with the token, or discover the rule and the assertion disagree.

Re-enumerating panes under the lock is not available: §6.4 rules it out, and for a stated reason (one hung tmux read would block every `hook set` on the machine).

**Options**:
- Take the advisory `hooks.json` read *before* the pane enumeration and confine deletion to keys present in that snapshot — an entry written after the snapshot was necessarily stamped after it too, so it can never be judged against an enumeration that predates its token; §6.3's "nothing it computes may reach the mutation" is restated as "nothing it computes may *widen* the delete set" (recommended)
- Require a token-shaped key to be absent from the live set on two consecutive sweeps before deleting it, so no single enumeration can delete on its own
- Accept the window as a residual, state it in §6.3 beside the file-snapshot race it already closes, and drop §9.2's assertion that the entry survives

**Resolution**: Pending
**Notes**:

---

### 2. §6.3 attributes the empty-live-set guard to the wrong read

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §6.3 (colliding reading in §5.4)

**Problem**:
The mass-deletion guard is the one thing standing between a bad tmux read and every hook on the machine, and the specification names two different inputs for it. §5.4 is emphatic that the guard's question is "did the tmux read succeed?", answered by the **pane row count**, and that conflating it with any other set is the mistake the section exists to prevent. §6.3 then says the sweep's call-site read of `hooks.json` is taken "to feed the empty-live-set guard" — a read of the hooks file cannot answer a question about live panes. An implementer wiring the guard from §6.3 gates the sweep on an empty hooks file, which fires when there is nothing to delete and stays silent in exactly the case §5.4 protects against.

**Proposal**:
§5.4 owns the guard and states its input unambiguously; §6.3's clause is a mis-attribution of purpose, not a second rule. The call-site read exists only to decide whether there is anything to do, and the sentence is corrected to say so and to point at §5.4 for the guard's input.

**Current**:
The sweep reads `hooks.json` twice: once at its call site (`runHookStaleCleanup`, to decide whether there is anything to do and to feed the empty-live-set guard) and once inside `CleanStale`.

**Proposed Text**:
The sweep reads `hooks.json` twice: once at its call site (`runHookStaleCleanup`, to decide whether there is anything to do) and once inside `CleanStale`. Neither read feeds the empty-live-set guard, which counts pane rows from the tmux enumeration (§5.4).

**Resolution**: Pending
**Notes**:

---

### 3. `portal doctor --fix` has no stated behaviour when the lock times out

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §6.5 (write-side degradation), §5.1

**Problem**:
§5.1 names two call sites for the same sweep — the daemon's idle branch and `portal doctor --fix` — and §6.5's write-side degradation covers only the daemon and the two `hook` verbs. A user who reaches for `doctor --fix` because a reboot looked wrong, and whose prune happens to time out on the lock, gets a run that repairs nothing, says nothing about why, and then fails its own post-repair check for stale hooks. The command whose job is to tell the user what happened would be silent about the one thing that did not.

**Proposal**:
The rule for the daemon already settles the substance — a write that cannot take the lock does not write — and the only open part is what the user sees. `doctor --fix` already prints a line per repair (`Pruned stale hook: <key>`), and this work unit's thesis is that a reaper which does not record its answer is the defect; so the skip is printed alongside those lines and logged on the same WARN as the daemon's skip, with the exit code left to the post-repair diagnosis exactly as documented. No addition to the closed logging vocabulary.

**Current**:
- the **daemon sweep** skips this cycle with a WARN and retries on the next 10s cadence — a deferred prune costs nothing, since stale entries are inert;
- `hook set` and `hook rm` exit non-zero with the reason, rather than hanging a shell the user is sitting in.

**Proposed Text**:
- the **daemon sweep** skips this cycle with a WARN and retries on the next 10s cadence — a deferred prune costs nothing, since stale entries are inert;
- `portal doctor --fix`, the sweep's other call site (§5.1), skips the same way and emits the same WARN (`op=clean-stale`, `via=internal`), and additionally prints one line naming the skipped prune alongside its `Pruned stale hook: <key>` output — a repair that silently did not run is the silence this work unit exists to remove. It does not fail the command: the exit code stays driven solely by the post-repair diagnosis, which reports whatever went un-pruned as stale;
- `hook set` and `hook rm` exit non-zero with the reason, rather than hanging a shell the user is sitting in.

**Resolution**: Pending
**Notes**:

---

### 4. No test covers the restore window, where the sweep would delete every hook on the machine

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §9.2, §5.4

**Problem**:
§5.4 names the worst failure this design can produce: a sweep landing between skeleton construction and the pane re-stamp sees a full pane list carrying no tokens and deletes every token-keyed entry on the machine. The defence is a new `@portal-restoring` read inside `runHookStaleCleanup` and a matching read in `checkStaleHooks`. §9.2 enumerates a test for nearly every other rule in the specification, down to the `hook list` fourth column, and names none for this one — so the guard against total hook loss is the only new behaviour that ships unverified, and a later refactor that drops the read fails nothing.

The unit lane also needs a way to drive the marker read. The sweep's tmux reads reach it through a seam today; the specification says which tmux calls the marker read joins (§5.4: outside the lock, alongside the pane enumeration) but not that it is reachable the same way, which is what makes a unit-lane test possible at all.

**Proposal**:
§5.4 already fixes the behaviour on both surfaces, so the test row asserts exactly what that section states, and the marker read is reached through the seam the pane enumeration already uses — the pattern §3.3 applies to the pane-option write ("reached through the `hook` command's existing `*Deps` seam like every other tmux call the CLI makes"). Unit lane: no daemon, no built binary.

**Proposed Text**:
Append to §5.4's restore-suppression paragraph, after "The read is a tmux call and sits outside the lock (§6.4), alongside the pane enumeration.":

The marker read is reached through the same seam as the pane enumeration, so both surfaces are drivable from the unit lane.

Add to the §9.2 table:

| **The sweep and the check stand down during a restore** | With `@portal-restoring` set, `runHookStaleCleanup` deletes nothing and `checkStaleHooks` reports not-evaluable rather than counting every token-shaped key as stale; a failed marker read produces the same result as a set marker (§5.4). | unit |

**Resolution**: Pending
**Notes**:

---

### 5. Two stated rules ship without a named test: the lock-timeout ladder and non-zero-on-nothing-removed

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §9.2, §6.5, §4.2, §4.3

**Problem**:
Two rules the specification argues at length have no row in §9.2.

§6.5 splits lock-timeout behaviour by side — writes refuse, reads proceed unlocked — and calls the blocking path the single riskiest part of this work unit, with a wedged daemon as the precedent. Nothing asserts the split holds. Getting it backwards is silent in both directions: a read that refuses forfeits a restored pane's hook (the symptom this work unit removes), and a write that proceeds unlocked reopens the lost update D exists to close.

§4.2 establishes that `hook rm` exits 0 iff it removed an entry, across all three ways of removing nothing, and §4.3 extends it to `--pane-key`. The §9.2 row covers only the unresolvable-`$TMUX_PANE` way. The two untested ways are the routine ones — deregistering twice, and pruning a `--pane-key` that is already gone — and they are the reason the external integration has to change (§4.2, §8.4), so the contract they encode is the one most likely to be softened back to a silent zero.

**Proposal**:
Both rules are fully specified; what is missing is the assertion. Two unit-lane rows, each asserting what its section already states and nothing more.

**Proposed Text**:
Add to the §9.2 table:

| **A lock timeout degrades by side** | With the sidecar lock held elsewhere: `hook set` and `hook rm` exit non-zero and leave `hooks.json` byte-identical, and the sweep deletes nothing and WARNs, while `LookupOnResume`, `checkStaleHooks` and `hook list` return their data anyway and log `op=load-unlocked` at DEBUG (§6.5). | unit |
| **Removing nothing is non-zero every way** | `hook rm` on a live, stamped pane whose token has no entry exits non-zero, and `hook rm --pane-key <key naming no entry>` exits non-zero, both leaving `hooks.json` byte-identical (§4.2, §4.3). | unit |

**Resolution**: Pending
**Notes**:

---

### 6. §9.4 restates §2.1's single-home argument

**Source**: Specification analysis
**Category**: Duplication
**Move**: settled
**Priority**: Minor
**Affects**: §9.4 (fact's home: §2.1)

**Problem**:
Which packages can reach the `@portal-pane-id` constant, and why that means every site composes it rather than spelling it, is decided in §2.1. §9.4 states it a second time in its own words. If the constant's home ever moves, the two accounts of why it can move there drift apart, and a reader has to work out which one is current before they can trust either.

**Proposal**:
§2.1 is the home — it declares the constant, lists the composing sites, and already points forward to the guards being retired. §9.4 keeps its own fact (both guards are deleted, not re-pointed) and points at §2.1 for the reason.

**Current**:
**Both are deleted, not re-pointed.** They exist because the `@portal-id` literal was written in two places that could not import one another. Homing the new constant in `internal/state` (§2.1) removes that condition: `captureFormat` is in the same package, `internal/tmux` already imports `internal/state`, and `cmd` imports both — so `@portal-pane-id` is written once and every site composes it. A guard binding two copies of a literal has nothing left to bind when there is one copy.

**Proposed Text**:
**Both are deleted, not re-pointed.** They exist because the `@portal-id` literal was written in two places that could not import one another. The single home in `internal/state` (§2.1) removes that condition. A guard binding two copies of a literal has nothing left to bind when there is one copy.

**Resolution**: Pending
**Notes**:

---

### 7. A production seam change is specified only inside the testing section

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Minor
**Affects**: §3.3, §9.3

**Problem**:
`AllPaneLister` is production code in `cmd`, and its signature and doc comment change with the enumeration. §3.3 specifies the enumeration change on the `internal/tmux` side and never mentions the `cmd`-side mirror; the only statement that the mirror changes sits in §9.3, a list of tests to re-point. Whoever breaks §3 into tasks builds the enumeration and leaves its only caller-side seam behind, and whoever takes §9.3 finds a production edit inside the test work.

**Proposal**:
Production surface changes belong with the surface they mirror. §3.3 states the seam change beside the enumeration it mirrors; §9.3's bullet keeps what is actually test work — the three fakes that stop compiling and the old-format key literals they carry.

**Current**:
- **The `cmd`-side enumeration seam and its three fakes** — `AllPaneLister` (`cmd/run_hook_stale_cleanup.go:13`) mirrors `ListAllPaneHookKeys`, so its method signature and its doc comment (which names the `<@portal-id or session_name>:window.pane` form) move to the two-field rows of §3.3 with it. Three unit-lane files in `cmd` implement or drive that seam and do not compile until they do:

**Proposed Text**:
Add to §3.3, immediately after the `ListAllPaneHookKeys` bullet:

- The `cmd`-side seam that mirrors it — `AllPaneLister` (`cmd/run_hook_stale_cleanup.go`) — changes signature with it, and its doc comment, which names the `<@portal-id or session_name>:window.pane` form, describes the two-field rows instead.

And replace the opening of the §9.3 bullet with:

- **The three fakes behind the `cmd`-side enumeration seam** — the seam itself changes with the enumeration (§3.3), and three unit-lane files in `cmd` implement or drive it and do not compile until they follow:

**Resolution**: Pending
**Notes**:

---

### 8. The restore-window skip's own observability is unstated

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Minor
**Affects**: §5.4

**Problem**:
The sweep gains a second reason to do nothing, and the specification does not say whether it says so. The neighbouring reason — an empty live set — emits a WARN, so an implementer reasonably matches it, and the daemon then WARNs about a mass-deletion hazard every 10 seconds through every restore window: exactly the false alarm §5.4 rejects one paragraph earlier when it refuses to count tokens. The opposite guess, total silence, leaves the sweep's behaviour through the window unreconstructable from a log at the moment an operator is investigating a lost hook.

**Proposal**:
§5.4's own reasoning settles the level: a WARN is for an anomaly, and a restore window is an expected state, so the skip must not raise one. §5.3's thesis — the reaper records what it did — settles that it is not silent either. One DEBUG line, on the shape the empty-live-set guard already uses, adding nothing to the closed vocabulary.

**Current**:
`portal doctor --fix` has no such gate, and it is the command a user reaches for when a reboot looks wrong. The check therefore moves **into `runHookStaleCleanup`**, so it travels with the rule the way shape-awareness does (§5.2) rather than sitting at one call site: the sweep reads `@portal-restoring` before it loads the store and skips the cycle when set.

**Proposed Text**:
`portal doctor --fix` has no such gate, and it is the command a user reaches for when a reboot looks wrong. The check therefore moves **into `runHookStaleCleanup`**, so it travels with the rule the way shape-awareness does (§5.2) rather than sitting at one call site: the sweep reads `@portal-restoring` before it loads the store and skips the cycle when set. The skip emits one DEBUG line on the shape the empty-live-set guard's WARN already uses, and never a WARN — a restore window is an expected state, and warning through every one of them names a hazard that is being avoided rather than encountered.

**Resolution**: Pending
**Notes**:
