# Review Tracking: Resume Hooks Silently Lost - Gap Analysis

## Findings

### 1. §1.2 credits change A with a fix only change B delivers

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §1.2 (What this work unit fixes); collides with §3.1 and §4.1

**Problem**:
The summary that a planner reads first says the durable pane identity (A) closes all three failure moments, and that B, C and D only close paths that made the loss *silent*. §3.1 says the opposite in its first bullet — "Write time is closed by §4, not by the format" — and §4.1 states outright that registration verification "is the only change that closes the write-time moment". A reader sizing the work from §1.2 can conclude the write-time probe is a nice-to-have hardening item that could be dropped or deferred without leaving a defect open, when dropping it leaves `portal hook set` still writing a junk key and exiting 0.

**Proposal**:
Correct §1.2 to the attribution §3.1 and §4.1 already agree on: A closes lifetime drift and the reboot boundary; B closes write time and nothing else can.

**Current**:
> A is the repair — it closes all three moments above. B, C and D close paths that made the loss silent or that can lose an entry independently of the key format.

**Proposed Text**:
> A is the repair for the two moments a positional key creates — lifetime drift and the reboot boundary. B closes the write-time moment, which no key format can close (§4.1). C and D close the path that made the loss silent and the path that can lose an entry independently of the key format.

**Resolution**: Pending
**Notes**:

---

### 2. `hook rm --pane-key` on a key that is not in the file — exit 0 or non-zero

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §4.3, §9.2 (test table); collides with §4.2

**Problem**:
§4.2 states the removal contract as one unconditional line — `hook rm` exits 0 iff it removed an entry — and rejects the idempotent reading by name. The §9.2 test row for the same command says `hook rm --pane-key <anything>` "still succeeds unchanged", which under the plain reading of "anything" means a key absent from `hooks.json` exits 0. Both readings are implementable, and they are the two ends of the property this work unit exists to remove: an exit code that says nothing about whether anything happened. The conversion script and the user's own integration (§8.2, §8.4) both drive this path, so the guess lands in the one place a wrong answer is invisible.

**Proposal**:
§4.2's rule governs both paths. `--pane-key` waives *validation of the key* — it does not waive the guarantee that the exit status reports whether an entry was removed. State that in §4.3, and pin the §9.2 row to a seeded key that exists.

**Current**:
§4.3:
> `portal hook rm --on-resume --pane-key <key>` performs **no validation of any kind** and touches tmux not at all. The key is used verbatim.

§9.2:
> | **`hook rm` on an unresolvable `$TMUX_PANE`** | Exits non-zero and writes nothing, while `hook rm --pane-key <anything>` still succeeds unchanged (§4.3). | unit |

**Proposed Text**:
§4.3, replacing the paragraph above:
> `portal hook rm --on-resume --pane-key <key>` performs **no validation of any kind** and touches tmux not at all. The key is used verbatim. The §4.2 rule still governs the exit status here: the pass-through waives validation of the key, not the guarantee that the code reports whether anything happened, so a `--pane-key` that names no entry in `hooks.json` exits non-zero like every other way of removing nothing.

§9.2, replacing the row above:
> | **`hook rm` on an unresolvable `$TMUX_PANE`** | Exits non-zero and writes nothing, while `hook rm --pane-key <a seeded key>` still removes it and exits 0 with no tmux read at all (§4.3). | unit |

**Resolution**: Pending
**Notes**:

---

### 3. No entry point for minting a token — the width has nowhere to come from

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §2.1 (Token generation), §3.2

**Problem**:
Minting happens in `cmd`'s `hook set` (§4.1 step 4), and the token's width is `suffixLen` — which §3.2 establishes is unexported in `internal/session`, and uses that fact to argue the shape predicate cannot live anywhere else. An implementer in `cmd` therefore has no legal way to reach the width and will write `6` at the call site, which is precisely the drift §3.2 rejects: move `suffixLen` later and generation drifts from recognition, every existing key silently stops being token-shaped, the reaper starts retaining what it should delete, and nothing fails anywhere. The spec closes this hole for the predicate and leaves it open for the generator that feeds it.

**Proposal**:
Symmetry with §3.2: minting is reached through an exported function in `internal/session` that reads the same two constants, so generation, recognition and the width move together. Determined by §3.2's own rule that the shape is derived from the generator's constants rather than restated.

**Current**:
> **Token generation** reuses the in-house nanoid: `session.NewNanoIDGenerator()` over `session.NanoIDAlphabet` (62 alphanumerics, no `-`), width `suffixLen = 6`. There is no uniqueness check beyond the generator's width — the same call the `@portal-id` stamp makes today (`internal/session/create.go`), which that code documents as deliberate.

**Proposed Text**:
> **Token generation** reuses the in-house nanoid: `session.NewNanoIDGenerator()` over `session.NanoIDAlphabet` (62 alphanumerics, no `-`), width `suffixLen = 6`. There is no uniqueness check beyond the generator's width — the same call the `@portal-id` stamp makes today (`internal/session/create.go`), which that code documents as deliberate.
>
> Minting is reached through an exported function in `internal/session`, beside the shape predicate (§3.2) and reading the same `suffixLen` and `NanoIDAlphabet` directly. `hook set` calls it and names no width of its own: generation and recognition are derived from one pair of constants, so they move together or not at all.

**Resolution**: Pending
**Notes**:

---

### 4. A second lock acquisition from the same process blocks against the first

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §6.3, §6.5

**Problem**:
`flock` is held per open file description, not per process, so a second acquisition from inside the same process is not re-entrant — it blocks against the caller's own hold. The design invites exactly that twice: `Set`/`Remove`/`CleanStale` hold the exclusive lock "across their internal load and save", and the natural implementation of "internal load" is a call to the exported `Store.Load`, which takes the shared lock; and `runHookStaleCleanup` does a call-site read and then calls `CleanStale`. Built the obvious way, every sweep parks for the full 2s bound before degrading, on the daemon's 1s tick loop — the wedge §6.5 spends a paragraph arguing the bound exists to prevent, reintroduced by the lock that was supposed to be safe. Nothing in the spec tells the implementer this is a trap, and the failure is a latency regression in the capture loop rather than a test failure.

**Proposal**:
State the non-nesting rule where the acquisition rules are: exported methods read and write through unexported non-locking helpers, and the sweep's advisory pre-read is released before `CleanStale` is called. Determined by §6.3's "acquire once and hold" and by §6.5's rejection of anything that can park the tick loop.

**Current**:
> The exclusive hold must span the **whole** mutation, not each file operation. `Set`, `Remove` and `CleanStale` each read, mutate and write; taking a shared lock to read and an exclusive lock to write would reopen the identical window. The exported methods acquire once and hold across their internal load and save.

**Proposed Text**:
> The exclusive hold must span the **whole** mutation, not each file operation. `Set`, `Remove` and `CleanStale` each read, mutate and write; taking a shared lock to read and an exclusive lock to write would reopen the identical window. The exported methods acquire once and hold across their internal load and save.
>
> **A lock is acquired once per operation and never nested.** `flock` is held per open file description rather than per process, so a second acquisition from the same process is not re-entrant: it blocks against the caller's own hold and resolves only at the §6.5 bound. Two rules follow. The exported methods reach the file through unexported non-locking load and save helpers, never by calling `Load` back through the front door. And `runHookStaleCleanup` releases its advisory pre-read before it calls `CleanStale`, so a sweep never waits on itself — which would put a 2s stall on the daemon's 1s tick loop every ten seconds, the outcome the bound exists to prevent.

**Resolution**: Pending
**Notes**:

---

### 5. `portal doctor` reports every hook on the machine as stale during a restore

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §5.4; interacts with §5.2's "doctor stays green" claim

**Problem**:
§5.4 identifies the window between skeleton construction and the §2.3 re-stamp, where live panes exist and carry no token, and suppresses the *sweep* through it. The read-only diagnosis is left in that window unguarded. `checkStaleHooks` sees a full pane list — so the not-evaluable empty-set branch does not fire — and an empty token set, so every token-shaped key counts as stale and `portal doctor` reports a hook failure and exits non-zero. `doctor` is bootstrap-exempt and is, in §5.4's own words, the command a user reaches for when a reboot looks wrong, so this fires precisely when someone is watching, and it tells them their hooks are gone at the one moment they are not. It also breaks §5.2's stated commitment that doctor stays green and keeps its exit-0-iff-all-pass contract.

**Proposal**:
Give `checkStaleHooks` the same `@portal-restoring` reading as the sweep, resolving to its existing not-evaluable result rather than to a count. Determined by §5.4's own rule that the check travels with the restore window rather than sitting at one call site, and by §5.2's doctor-stays-green commitment.

**Current**:
> `portal doctor --fix` has no such gate, and it is the command a user reaches for when a reboot looks wrong. The check therefore moves **into `runHookStaleCleanup`**, so it travels with the rule the way shape-awareness does (§5.2) rather than sitting at one call site: the sweep reads `@portal-restoring` before it loads the store and skips the cycle when set.

**Proposed Text**:
> `portal doctor --fix` has no such gate, and it is the command a user reaches for when a reboot looks wrong. The check therefore moves **into `runHookStaleCleanup`**, so it travels with the rule the way shape-awareness does (§5.2) rather than sitting at one call site: the sweep reads `@portal-restoring` before it loads the store and skips the cycle when set.
>
> `checkStaleHooks` takes the same reading, for the same window and a different reason. Its live set is a full pane list carrying no tokens, so the empty-set branch does not fire and every token-shaped key counts as stale — a read-only `portal doctor` run in that window would report every hook on the machine as lost and exit non-zero, on the command whose whole job is to tell the user whether that happened. It reads `@portal-restoring` by the sweep's rule, a failed read treated as set, and reports its existing not-evaluable result when the marker is set rather than counting.

**Resolution**: Pending
**Notes**:

---

### 6. `hook list` field count is not fixed when a token resolves to nothing

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §4.4

**Problem**:
The location column "renders empty" for an unresolved token, an old-format key, or a machine with no tmux server — but whether an empty value is an empty fourth field or a dropped one is left to the implementer. The two guesses produce different field counts on the same line, and `hook list` output is parsed: the spec names the user's own integration matching against it (§1.3) and rests the whole "the column is appended, so existing field positions are undisturbed" argument on positional stability. On a machine with no server every line takes the empty path at once, so a dropped field is not an edge case there — it is the whole output.

**Proposal**:
The column is always emitted; an unresolved token renders as an empty fourth field. Determined by §4.4's own guarantee that field positions are undisturbed for any caller parsing the output, which only holds if the count is constant.

**Current**:
> Resolution is one `list-panes -a` read over the §3.3 enumeration, whose rows already carry the token alongside its location; the token → location mapping is built once from that read and reused across all rows. A token that resolves to no live pane renders the column **empty** rather than failing the command — including the case where no tmux server is running at all, which `hook` is bootstrap-exempt from starting. An old-format key likewise renders empty, since no live pane can answer to one.

**Proposed Text**:
> Resolution is one `list-panes -a` read over the §3.3 enumeration, whose rows already carry the token alongside its location; the token → location mapping is built once from that read and reused across all rows. A token that resolves to no live pane renders the column **empty** rather than failing the command — including the case where no tmux server is running at all, which `hook` is bootstrap-exempt from starting. An old-format key likewise renders empty, since no live pane can answer to one. The column is always emitted: an empty value is an empty fourth field, never a dropped one, so every line carries the same three separators whatever resolution produced.

**Resolution**: Pending
**Notes**:

---

### 7. §5.1's "two changes, and no more" undercounts what §5 and §6 do to the sweep

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Minor
**Affects**: §5.1; collides with §5.4 and §6

**Problem**:
The sentence tells a planner that the reaper takes exactly two changes, and the sections beneath it deliver four: shape-awareness (§5.2), INFO naming (§5.3), a new `@portal-restoring` read that skips the cycle entirely (§5.4), and the exclusive lock spanning the mutation (§6). Someone breaking §5 into tasks off that sentence writes two and misses the restore-window skip, which is the one whose absence silently deletes every token-keyed entry on the machine.

**Proposal**:
Drop the count and keep the claim it was protecting — that the reaper is not converted to full retention — pointing at the homes for what does change.

**Current**:
> Two changes, and no more. **Whether the reaper deletes is not changed** — only what it can identify, and what it records.

**Proposed Text**:
> **Whether the reaper deletes is not changed** — it is not converted to full retention. What changes is what it can identify (§5.2), what it records (§5.3), when it declines to run at all (§5.4), and how it takes the file it mutates (§6).

**Resolution**: Pending
**Notes**:

---

### 8. A stamp that fails on a live pane has no stated outcome

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Minor
**Affects**: §4.1

**Problem**:
Step 4 of the registration sequence mints and stamps, and the spec describes `set-option -p` only as "a second, redundant guard" against a bogus target. What `hook set` does when the stamp fails on a pane the probe just proved live — a transient tmux failure — is unstated. An implementer who treats step 4's error as redundant-and-therefore-ignorable proceeds to step 5 and writes an entry keyed to a token no pane carries, which is the exact state the ordering rule beneath the list exists to prevent, arrived at from the other direction. §4.1 specifies the mirror failure (stamp succeeds, write fails) in detail and leaves this one open.

**Proposal**:
A failed stamp ends the command before the write, on the same terms as a failed probe. Determined by the ordering rule already stated ("a write that precedes the stamp would persist an entry keyed to a token no pane carries") — a write that *follows a failed* stamp persists the same thing.

**Current**:
> Steps 4 and 5 must not be reordered: a write that precedes the stamp would persist an entry keyed to a token no pane carries.

**Proposed Text**:
> Steps 4 and 5 must not be reordered, and step 4 failing ends the command: a write that precedes the stamp, or follows one that failed, persists an entry keyed to a token no pane carries. A failed stamp exits non-zero with tmux's words and writes nothing, the same shape as a failed probe at step 2.

**Resolution**: Pending
**Notes**:

---

### 9. The degraded read's `via` values are unnamed

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Minor
**Affects**: §6.5

**Problem**:
The one new log emission this work unit adds is specified down to its level and `op` value, then says `via` names "the caller" without naming any of the three. The project's logging vocabulary is closed and amended only by specification, so each of `LookupOnResume`, `checkStaleHooks` and `hook list` gets a call-site-invented string, and the line whose whole purpose is to say which reader ran unlocked will not group across the three sites. The claim in the next sentence — that the amendment is one `op` value and no attr key — also skips the `via` values it is introducing.

**Proposal**:
Name the three values and count them into the amendment.

**Current**:
> The degraded read is the one genuinely new emission: DEBUG, `op=load-unlocked`, `via` naming the caller, the lock error in `error`. That adds **one `op` value and no attr key** — the whole of this work unit's amendment to the closed logging vocabulary.

**Proposed Text**:
> The degraded read is the one genuinely new emission: DEBUG, `op=load-unlocked`, the lock error in `error`, and `via` naming the caller — `hydrate` for `LookupOnResume`, `doctor` for `checkStaleHooks`, and the existing `cli` for `hook list`. That adds **one `op` value, two `via` values and no attr key** — the whole of this work unit's amendment to the closed logging vocabulary.

**Resolution**: Pending
**Notes**:

---

### 10. Two §9.2 rows put a `cmd`-level test on a real tmux server, which §9.1 rules out

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Minor
**Affects**: §9.2 (test table); §9.1

**Problem**:
§9.1 settles the split — real-tmux behaviour is pinned in `internal/tmux` client tests, and `cmd` tests inject their `*Deps` seam because `cmd`'s `TestMain` poisons `TMUX` package-wide, so a real-tmux `cmd` test would have to supply its own socket "and is avoided". Two rows in the table beneath it then mark `cmd`-level behaviour — `hook set` reusing an existing token, and `hook list`'s fourth column — as `unit (real-tmux)`. Whoever writes those tests either supplies a socket in `cmd` against the stated rule or restructures the test, and picks differently from whoever writes the other one.

**Proposal**:
Both behaviours are `cmd`-level decisions over data a seam supplies — the token the read returned, and the enumeration rows — so both are plain unit tests with the seam injected, per §9.1. The raw tmux facts underneath them are already pinned by the existence-probe row.

**Current**:
> | **`hook set` reuses an existing token** | A second registration on the same pane writes under the same key and mints nothing (§2.2). | unit (real-tmux) |
>
> | **`hook list` fourth column** | Renders the resolved location for a live token, and empty for a token that resolves to no live pane (§4.4). | unit (real-tmux) |

**Proposed Text**:
> | **`hook set` reuses an existing token** | With the seam returning a token for the pane, a second registration writes under that same key and issues no `set-option` (§2.2). | unit |
>
> | **`hook list` fourth column** | Over a fixed enumeration: renders the resolved location for a live token, and an empty fourth field for a token no row carries (§4.4). | unit |

**Resolution**: Pending
**Notes**:

---

### 11. §2.1's list of sites that compose the option name omits the restore re-stamp

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Minor
**Affects**: §2.1; underpins §9.4

**Problem**:
§9.4 retires both literal-binding guards on the strength of the literal being written in exactly one place, and §2.1's enumeration of the sites that compose it from the constant is what carries that claim. The enumeration names three sites and misses the restore re-stamp (§2.3), which writes `@portal-pane-id` with `set-option -p`, and the all-pane enumeration format (§3.3), which reads it. An implementer working from the list has no instruction covering the site they are in and spells the literal — reintroducing the second copy the guards existed to bind, with the guards now deleted.

**Proposal**:
Complete the enumeration so every writing and reading site is covered by the compose-from-the-constant rule.

**Current**:
> Every site that needs the literal composes it from that constant: `captureFormat` in the same package, `HookKeyFormat` in `internal/tmux` (which already imports `internal/state`), and the `hook set` stamp in `cmd` (which imports both).

**Proposed Text**:
> Every site that needs the literal composes it from that constant: `captureFormat` in the same package, `HookKeyFormat` and the all-pane enumeration format in `internal/tmux` (which already imports `internal/state`), the re-stamp in `internal/restore`, and the probe, read and stamp in `cmd` (which imports both).

**Resolution**: Pending
**Notes**:

---

### 12. §5.2 restates §1.1's indistinguishability finding

**Source**: Specification analysis
**Category**: Duplication
**Move**: settled
**Priority**: Minor
**Affects**: §5.2; fact's home is §1.1

**Problem**:
The reason a moved pane and a dead pane cannot be told apart at the point of comparison is established in §1.1, where the defect is described. §5.2 states it again nearly verbatim as the premise of its own argument. Two copies of a load-bearing premise drift apart under later edits and come back as a contradiction between the defect statement and the justification for changing the reaper.

**Proposal**:
Point at §1.1 and keep only what §5.2 adds — that the token key is what makes the reaper's judgement trustworthy.

**Current**:
> The justification is that A removes the reaper's false positives. Under a positional key a moved pane and a dead pane are indistinguishable at the point of comparison, so the reaper was acting correctly on false information.

**Proposed Text**:
> The justification is that A removes the reaper's false positives: the indistinguishability §1.1 names is what had it acting correctly on false information.

**Resolution**: Pending
**Notes**:

---

### 13. §3.3 re-enumerates the positional siblings §1.3 already lists

**Source**: Specification analysis
**Category**: Duplication
**Move**: settled
**Priority**: Minor
**Affects**: §3.3; fact's home is §1.3

**Problem**:
The three name-based positional helpers, and the fact that they serve only the skeleton-marker and cleanup paths, are listed in §1.3's out-of-scope section and again in §3.3, which then cross-references §1.3 anyway. A later change to what those helpers serve — or to which of them survive — has to be made in two places, and a reader who finds only one copy cannot tell whether it is current.

**Proposal**:
Keep the pointer, drop the second copy of the list and its rationale.

**Current**:
> The name-based positional siblings — `StructuralKeyFormat`, `ResolveStructuralKey`, `ListAllPanes` — are untouched. They serve the `@portal-skeleton-*` marker and cleanup paths only (§1.3).

**Proposed Text**:
> The name-based positional siblings are untouched (§1.3).

**Resolution**: Pending
**Notes**:
