# Review Tracking: Resume Hooks Silently Lost - Gap Analysis

## Findings

### 1. The sweep's two reads are ordered two different ways

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Critical
**Affects**: §6.4 "The locked region covers the file only"; colliding reading in §6.3 "Readers take a shared lock, writers exclusive"

**Problem**:
The stale sweep does two reads before it deletes anything — a snapshot of `hooks.json` and a live pane enumeration — and the specification puts them in opposite orders in two adjacent sections. The concurrency section requires the file snapshot to be taken **first**, and rests a whole loss-prevention argument on it: an entry written by `hook set` after the snapshot was necessarily stamped after the enumeration too, so it can never be judged stale by that enumeration. The lock-scope section states the reverse — that the sweep "performs its `ListAllPaneHookKeys` read before it touches the store at all". A builder who follows the lock-scope section enumerates first, and a hook registered in the gap between the enumeration and the mutation is token-shaped, absent from the live set, and deleted seconds after the command reported success — the exact loss this work unit exists to remove. The same sentence also asserts that the guard on the enumeration "resolves before any lock is taken", which cannot hold once the snapshot (and its shared lock) precedes it.

**Proposal**:
Correct the lock-scope section to the ordering the concurrency section's argument depends on. The rule that section actually owns — no tmux call inside the lock — still holds under that ordering, because the snapshot's shared lock is released when the read returns, so the enumeration and its guard run with no lock held and the exclusive lock is taken only by the mutation.

**Current**:
> This falls out naturally at the sweep's call site: `runHookStaleCleanup` performs its `ListAllPaneHookKeys` read before it touches the store at all, and the guard on that read (§5.4) resolves before any lock is taken.

**Proposed Text**:
This falls out naturally at the sweep's call site: `runHookStaleCleanup` takes its call-site snapshot first (§6.3) and the shared lock is released when that read returns, so the `ListAllPaneHookKeys` read and the guard on it (§5.4) both resolve with no lock held, and the only lock the sweep holds while calling tmux is none.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. Self-inflicted: cycle 4's ordering fix rewrote §6.3 and left §6.4 asserting the opposite order, so a builder following §6.4 would have reopened the very race cycle 4 closed. The no-tmux-call-inside-the-lock rule survives the correction because the snapshot's shared lock is released when the read returns.

---

### 2. What `CleanStale` is handed does not match the delete-set rule it must satisfy

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §6.3 "Readers take a shared lock, writers exclusive"

**Problem**:
One sentence says the deleting call receives the live token set and derives the delete set from it under its own lock. Two paragraphs later the delete set is defined as every key that is in the file under the lock **and** in the call-site snapshot **and** token-shaped **and** absent from the live set. The second rule cannot be evaluated from the first sentence's inputs — the snapshot's key set is never handed over. A builder implementing the first sentence gets a sweep with no snapshot narrowing, which reopens the "registered during the enumeration gap, deleted moments later" loss the narrowing exists to close, and fails the sweep case the test list pins.

**Proposal**:
Name both inputs where the call is described. The distinction the surrounding text draws is preserved: what may not be handed over is a *delete list* computed outside the lock; what must be handed over is the snapshot's *key set*, which can only narrow the delete set the locked derivation produces.

**Current**:
> `CleanStale` receives the **live token set** and derives the delete set itself, under its own lock.

**Proposed Text**:
`CleanStale` receives the **live token set** and the **call-site snapshot's key set**, and derives the delete set itself, under its own lock.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. Also self-inflicted: the four-way delete-set rule I wrote in cycle 4 cannot be evaluated from the inputs the same section says `CleanStale` receives. Handing over the snapshot's key set (not a delete list) preserves the distinction the section draws.

---

### 3. Nothing says where `hook rm`'s exit code gets its answer

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §4.2 "Removal verifies the same way — on the `$TMUX_PANE` path only"; §6.3 "Readers take a shared lock, writers exclusive"

**Problem**:
`hook rm` must exit 0 if and only if it removed an entry — the specification makes that the one-line rule and rejects the idempotent reading outright. It never says where the "did anything get removed" answer comes from. The obvious build is to read `hooks.json`, check for the key, then call the removal — which decides the exit status from a snapshot the mutation never saw. Between the two, a concurrent sweep or another pane's `hook set` can change the file, so the command reports a removal that did not happen, or reports nothing removed after removing something. That is an exit code that says nothing about whether anything happened, which is the property this work unit exists to remove, reintroduced on the command whose contract is that exit code.

**Proposal**:
State that the answer comes from inside the locked mutation. This is the only reading consistent with the concurrency section's rule that the whole read-modify-write happens under one exclusive hold, and it covers all three of the "removed nothing" routes uniformly, including the `--pane-key` pass-through, which has no other way to know.

**Proposed Text**:
Add to §4.2, after the paragraph ending "an exit code that says nothing about whether anything happened.":

Whether anything was removed is reported by the locked removal itself, not by a read taken before it: the store's removal answers from the file it mutated, under the exclusive hold of §6.3, and that answer alone drives the exit status. A check taken before the mutation would decide the exit status from a snapshot the mutation never saw — a concurrent sweep or another pane's `hook set` between the two would make the report wrong in either direction. This is what lets the `--pane-key` path (§4.3) carry the same rule while reading nothing of its own.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. The exit-status rule was settled in cycle 1 and never said where the answer comes from; deciding it from a pre-read reinstates the uninformative exit code on the one command whose contract is that exit code.

---

### 4. `portal doctor --fix` stands down during a restore and tells the user nothing

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §5.4 "The mass-deletion guard keys off live *panes*, not live *tokens*"; §6.5 "Acquisition is bounded, and a timeout degrades rather than wedges"

**Problem**:
The stale prune skips its cycle whenever the restoring marker is set, and that skip is deliberately logged at DEBUG and never as a warning. The command that reaches the skip is `portal doctor --fix` — named in the same paragraph as the one a user runs when a reboot looks wrong, which is precisely when the marker is most likely set. On that path the user asked for a repair, the repair silently did not run, and the terminal output is identical to a run where there was nothing to prune. The specification already rejects that outcome for the other reason a prune can be skipped (a lock timeout, where `doctor --fix` prints a line naming the skipped prune) but never says what happens on this one, so a builder will match the DEBUG-only rule and print nothing.

**Proposal**:
Apply the rule the specification already fixed for the skipped prune to this skip too, and say so where the restore stand-down is defined. The log stays DEBUG — the reasoning that a restore window is an expected state is about the log level, not about the command's own report — and the exit code is unaffected, since the post-repair diagnosis reports the same window as not evaluable.

**Proposed Text**:
Add to §5.4, at the end of the paragraph beginning "`portal doctor --fix` has no such gate":

When the sweep skips for a set marker, `portal doctor --fix` names the skipped prune in its output the way §6.5 has it name a prune skipped for a lock timeout — the log line stays DEBUG, but a user who asked for a repair is told it did not run. The exit code is unaffected: it stays driven by the post-repair diagnosis, whose `checkStaleHooks` reports not evaluable in the same window.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. The same silence the lock-timeout skip already rejects, on the path most likely to hit it — a user running `--fix` because a reboot looked wrong is running it exactly when the marker is set.

---

### 5. An empty key has no way out of `hooks.json`, and `hook list` gives it somebody else's location

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Minor
**Affects**: §5.2 "Deletion becomes shape-aware"; §4.4 "`portal hook list` renders the resolved location"

**Problem**:
The specification accepts that an empty key can reach `hooks.json` — a hand edit or a bug in the out-of-band conversion — and guards the firing path against it at two boundaries. It then makes that entry permanent: the reaper retains everything that is not token-shaped, and an empty string never is, so nothing on any code path ever removes it. Separately, `hook list` builds its token-to-location map from an enumeration whose rows carry an empty token for every unstamped pane; built naively that map has an entry under the empty token, so the malformed line renders the location of whichever unstamped pane happened to land there — a real pane name against an entry that belongs to no pane.

**Proposal**:
The retention rule exists to protect unconverted old-format keys, and an empty key is provably not one: old-format keys always contain `:` and `.`. Deleting it costs nothing that the retention rule was written to protect and gives the malformed entry the exit route it otherwise lacks. The display map is built from non-empty tokens for the same reason it is built at all — a location is only meaningful for a pane that carries the token.

**Proposed Text**:
Add a third bullet to §5.2's list:

- An **empty** key is deleted. It is neither token-shaped nor old-format — an old-format key always carries `:` and `.` (§3.2) — so the retention rule has nothing to protect in it. It is the malformed entry §3.4 guards the firing path against, and deletion is its only route out of the file short of a hand edit.

And add to §4.4, after "the token → location mapping is built once from that read and reused across all rows":

The mapping is built from **non-empty** tokens only, so an unstamped pane's row cannot lend its location to an entry that names no pane.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed, both edits. The empty key had no exit route at all under the retention rule, and the display map would have lent it a real pane's location.

---

### 6. The 2s lock bound leaves the timeout tests waiting on the clock

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Minor
**Affects**: §6.5 "Acquisition is bounded, and a timeout degrades rather than wedges"; §9.2 "New tests"

**Problem**:
The acquisition bound is fixed at 2 seconds and the test list requires a unit-lane test that drives six surfaces through a timeout — two commands that must exit non-zero, a sweep that must skip and warn, and three readers that must degrade and log. Each of those waits out the full bound, so the case adds the better part of ten seconds to a lane the project keeps fast and hermetic, and a builder who notices will invent a seam that the specification never sanctions. The specification is otherwise explicit about which surfaces must be drivable from the unit lane and how.

**Proposal**:
Say that the bound is a value the unit lane can lower. The 2s figure is a production tuning choice justified against the sweep's cadence and the daemon's tick, not a property any test asserts — what the tests assert is which side of the split each surface falls on.

**Proposed Text**:
Add to §6.5, at the end of the paragraph beginning "The bound is **2 seconds**":

The bound is a package-level value the unit lane can lower, so the §9.2 timeout cases assert which side of the split each surface falls on rather than waiting out the production figure.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. The 2s figure is a production tuning choice, not a property any test asserts; without this a builder either burns ~10s in the unit lane or invents an unsanctioned seam.

---

### 7. The dirty-flag warning has no stated emitter

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Minor
**Affects**: §2.2 "Stamping is lazy, at `hook set`"; §6.5 "Acquisition is bounded, and a timeout degrades rather than wedges"

**Problem**:
The warning for a failed `save.requested` touch is specified down to its message, `op`, and attrs, and is described as sitting on the store's existing failure shape — but the touch does not happen in the store. It happens after the store write, in the command, and the store cannot reach the state package without a layering change. So the line is emitted from a package that the specification never says binds the `hooks` component, in a project where components are a closed vocabulary bound once per package and never invented at a call site. The specification's own accounting of what this work unit adds to that vocabulary lists two `op` values, two `via` values and no attr key — a binding in a new package is not among them, so a builder is left to either add one unannounced or push the touch down a layer it does not belong in.

**Proposal**:
Name the emitter where the warning is specified, and account for the binding alongside the other vocabulary amendments. The command is the only site that can emit it: the touch is the command's, and the store has no path to the state directory.

**Proposed Text**:
Add to §2.2, at the end of the paragraph beginning "The touch is **best-effort and never affects the exit status.**":

The line is emitted from `cmd`, where the touch happens — the store has no path to the state directory — so the `hooks` component gains a binding in that package. The store's own emissions are unaffected.

And amend §6.5's accounting sentence to read:

That adds **two `op` values** — `load-unlocked` here and `touch-save-requested` for the dirty-flag touch (§2.2) — **two `via` values, one component binding in `cmd` (§2.2) and no attr key**: the whole of this work unit's amendment to the closed logging vocabulary.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed, both edits. The touch happens in `cmd` and the store has no path to the state directory, so the component binding is real and belongs in the vocabulary accounting.

---

### 8. The shared-constants rule for the token is argued twice

**Source**: Specification analysis
**Category**: Duplication
**Move**: settled
**Priority**: Minor
**Affects**: §2.1 "The token and its tmux option"; home of the fact is §3.2 "Key shape"

**Problem**:
That minting and recognition both read `suffixLen` and `NanoIDAlphabet` directly, so a change to either moves them together, is stated in the key-shape section — where it is the reason that section gives for the predicate's home and for needing no guard test — and again in the token section, in its own words. Two copies of one rule drift apart under later edits: a width or alphabet change made against one copy leaves the other asserting the opposite, and the failure mode the argument warns about is exactly the one that returns silently.

**Proposal**:
Keep the fact where it does work — the key-shape section, which uses it to place the predicate and to justify the absence of a guard — and let the token section say only what is its own: where the mint function lives, and that the caller names no width.

**Current**:
> Minting is reached through an exported function in `internal/session`, beside the shape predicate (§3.2) and reading the same `suffixLen` and `NanoIDAlphabet` directly. `hook set` calls it and names no width of its own: generation and recognition derive from one pair of constants, so they move together or not at all.

**Proposed Text**:
Minting is reached through an exported function in `internal/session`, beside the shape predicate (§3.2). `hook set` calls it and names no width of its own.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. The shared-constants rule does its work in §3.2, where it places the predicate and justifies the absent guard; §2.1 keeps only what is its own.

---

### 9. A sentence in the removal section points at something the document does not contain

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Minor
**Affects**: §4.2 "Removal verifies the same way — on the `$TMUX_PANE` path only"

**Problem**:
The removal section closes by claiming coverage of "the half of the CLI surface the blast radius named and the original framing of B did not". Neither referent exists in the document: nothing here is called a blast radius, and B has exactly one framing. A reader cannot resolve either, and what the sentence is trying to say — that both halves of the CLI now sit behind one rule — is lost in the reference.

**Proposal**:
Say the substance directly. The point worth keeping is that verification covers removal as well as registration; the reference adds nothing a reader of this document can use.

**Current**:
> `portal hook rm --on-resume` run from a pane resolves the key with the same two reads as §4.1 steps 2–3, and fails non-zero on the same non-zero exit. This is literally the same guard, not an analogue of it — which is what lets removal carry it without minting anything. It covers the half of the CLI surface the blast radius named and the original framing of B did not.

**Proposed Text**:
`portal hook rm --on-resume` run from a pane resolves the key with the same two reads as §4.1 steps 2–3, and fails non-zero on the same non-zero exit. This is literally the same guard, not an analogue of it — which is what lets removal carry it without minting anything, and it puts both halves of the CLI surface behind one rule rather than only the write side.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. Both referents were unresolvable from within the document — a leftover from the investigation's vocabulary.

---

### 10. Whether a read's lock outlives the read is left open

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Minor
**Affects**: §6.3 "Readers take a shared lock, writers exclusive"

**Problem**:
The concurrency section says the sweep "releases its advisory pre-read" before it deletes, which reads as though a read hands back a lock its caller must release — a shape that would change every reader on the file, since the hydrate helper, `hook list` and the doctor check all take the same read and none of them is given a release step. The alternative reading, that the shared lock lives and dies inside the read call, makes that sentence describe nothing a builder can act on. Which of the two is intended decides whether the store's read has one return value or two, so it cannot be left open.

**Proposal**:
The lock lives and dies inside the read. The section already establishes that the shared lock is an ordering courtesy rather than a correctness requirement — a reader observes a complete snapshot whatever a writer is doing — so there is nothing a caller would hold it past the read for, and no other reader is given a release step to match.

**Current**:
> And `runHookStaleCleanup` releases its advisory pre-read before it calls `CleanStale`, so a sweep never waits on itself — which would put a 2s stall on the daemon's 1s tick loop every ten seconds, the outcome the bound exists to prevent.

**Proposed Text**:
And a read's shared lock is released when the read returns, never handed back to its caller, so the sweep's advisory pre-read is no longer held by the time `CleanStale` takes the exclusive one and a sweep never waits on itself — which would put a 2s stall on the daemon's 1s tick loop every ten seconds, the outcome the bound exists to prevent.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. 'Releases its pre-read' implied a caller-held lock, which would have changed every reader on the file; the lock lives and dies inside the read.

---
