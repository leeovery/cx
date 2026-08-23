# Review Tracking: Resume Hooks Silently Lost - Input Review

## Findings

### 1. Restore re-stamp behaviour for a saved pane with no token

**Source**: `investigation/resume-hooks-silently-lost.md` — Fix Direction, "Stamping is lazy, at `hook set`" ("a pane with no hook needs no token"); Refinements, "The restore re-stamp must not be a swallowed error"; Refinements, "The mass-deletion guard keys off live *panes*, not live *tokens*" ("under lazy stamping, 'zero stamped panes' is the ordinary steady state during the upgrade window")
**Category**: Gap/Ambiguity
**Affects**: §2.3 (Carrying the token across the reboot gap), §2.4 (Restore re-stamp)

**Details**:
The source's lazy-stamping decision makes "saved pane with an empty `PortalPaneID`" the *majority* case, not an edge case: every pane that never had a hook carries no token, and the source explicitly names "zero stamped panes" as the ordinary steady state during the upgrade window. The specification specifies the re-stamp only for the populated case — §2.3 says "the saved token is re-stamped onto the corresponding live pane with `set-option -p`", and §2.4 adds that a failed re-stamp logs a WARN naming the session and pane.

Neither section says what happens when the saved token is empty. Two behaviours are consistent with the text as written and they differ materially:

- Restore skips the `set-option -p` for an empty token (silent, no WARN, no option written); or
- Restore issues `set-option -p @portal-pane-id ""`, writing an empty pane option on every unstamped pane of every restored session — and, if that call errors, §2.4's rule produces a WARN per unstamped pane on every boot.

§3.4 covers the empty key at the *bake* and *lookup* boundaries but is explicitly about `collectArmInfos` and `LookupOnResume`, not about the re-stamp, so it does not close this. The source's guard-conflation refinement (§5.4 in the spec) shows the authors were alert to "unstamped is normal" elsewhere; the re-stamp path did not receive the same treatment.

**Proposed Change**:
Skip the `set-option -p` entirely when the saved `PortalPaneID` is empty — no option written, nothing logged — stated in §2.3's Restore bullet, with §2.4's WARN paragraph scoped so it can only fire for a genuine tmux failure.

**Resolution**: Approved
**Notes**: Applied to §2.3 and §2.4. Skip chosen over writing an empty option: an empty value is indistinguishable on read-back from absence, and would turn the lost-identity WARN into per-boot noise across the unstamped majority.

---

### 2. `hook rm`'s existence check has no stated mechanism, and cannot use §4.1's

**Source**: `investigation/resume-hooks-silently-lost.md` — Refinements, "B is stronger than first described, and nearly free" (`set-option -p` exits 1, unlike `display-message -p` which exits 0); Refinements, "`hook rm` gets the same verification as `hook set` — but only on the `$TMUX_PANE` path"; Finding 1 (`display-message -p` against an unresolvable target exits 0 with every field empty)
**Category**: Gap/Ambiguity
**Affects**: §4.2 (Removal verifies the same way)

**Details**:
§4.1 is explicit that verification is tmux-native rather than a shape heuristic *because* the stamp precedes the write and `set-option -p` against a bogus target exits 1. §4.2 then says removal "resolves the key by reading `@portal-pane-id`, and fails non-zero when the pane does not exist. This is the same guard as §4.1" — but immediately forbids the mechanism that guard rests on: "Removal does **not** mint."

On the read path the source's own evidence says tmux gives nothing to detect: `display-message -p` exits 0 and returns empty both for a pane that does not exist *and* for a live pane that carries no token. So "the same guard as §4.1" is not available to `hook rm`, and the specification does not say what replaces it — whether removal issues a read-only existence probe (e.g. a `list-panes`/`display-message` target check that does error), or whether it simply treats an empty token as "no entry to remove" and exits non-zero without distinguishing the two cases.

The second paragraph of §4.2 hints at the latter ("A pane with no token has no entry to remove; `hook rm` reports that and exits non-zero"), which would make the first paragraph's "fails non-zero when the pane does not exist" an outcome rather than a check. If that collapse is intended it should be stated, because the §9.2 test row asserts the behaviour ("`hook rm` on an unresolvable `$TMUX_PANE` — exits non-zero and writes nothing") and an implementer needs to know which mechanism is being pinned.

**Proposed Change**:
Both commands read the pane's token with `show-options -p`, which exits non-zero for a pane that does not exist — a read-only probe, so removal carries the identical guard without minting. §4.1's step list is rewritten around that read (check precedes mint); §4.2's first paragraph points at the same read.

**Resolution**: Approved
**Notes**: Applied to §4.1 and §4.2. The contradiction was real — §4.1's guard was the mint, which §4.2 forbids. Measured on tmux 3.7c: `show-options -p` distinguishes gone (exit 1, `no such pane`), live-unstamped (exit 1, `invalid option`) and stamped (exit 0); `display-message -p` exits 0 for the first two alike, so the naive read cannot tell them apart. Portal treats the non-zero exit as the whole signal and never parses tmux's message text.

---

### 3. The `rm`-on-a-closed-pane path is the routine case, and the caller reads its exit code

**Source**: `investigation/resume-hooks-silently-lost.md` — Standing Evidence ("63 lines carrying `hook_key=:.` — **61 `op=rm`**, 2 `op=set`… run near-daily… so the degenerate resolution is common at SessionEnd, not a rarity"); Finding 1, "Why `$TMUX_PANE` is unresolvable so often" (SessionEnd commonly fires *because the pane was closed*) and "The `rm` side has its own consequence, distinct from the `set` side" (the end state "is usually benign"); Contributing Factors ("The registering caller cannot verify its own success… reads `rc`… which is 0 on both paths")
**Category**: Enhancement to existing topic
**Affects**: §4.2 (Removal verifies the same way), and touches §8.4 (The user's own integration)

**Details**:
§4.2 justifies extending B to `hook rm` purely on shape ("the same silent-success shape as the `:.` bug on the write side"). The source carries two facts about that path that the specification does not, and both change what shipping it looks like in practice:

1. **It is the ordinary event, not an error case.** 61 of the 63 `:.` log lines in a month were `op=rm`, near-daily, every one a Claude Code SessionEnd — and the source explains why: SessionEnd commonly fires *because the pane was closed*, so tmux has already reclaimed the pane id by the time the deregistration runs. After §4.2, that routine sequence exits non-zero every time.
2. **The current end state is benign, and a caller is watching the exit code.** The source notes that a `:.` removal "deletes nothing that matters", the real entry is absorbed later by the sweep, "so the end state is usually benign" — and that the registering caller reads `rc`, which is 0 on both paths today.

The specification's expected-behaviour framing (registration failing loudly so the caller can react) is sourced and unchanged by this. What is missing is the statement that the loud failure on the `rm` path will fire routinely against an already-closed pane, so a caller that treats non-zero as an error will begin seeing it as a matter of course. §8.4 already sets the expectation that the external script needs updating in step; this is the second reason it does, and it is not currently recorded anywhere in the spec.

**Current**:
```
#### 4.2 Removal verifies the same way — on the `$TMUX_PANE` path only

`portal hook rm --on-resume` run from a pane resolves the key by reading `@portal-pane-id`, and fails non-zero when the pane does not exist. This is the same guard as §4.1, applied to the half of the CLI surface the blast radius named and the original framing of B did not cover.

Removal does **not** mint. A pane with no token has no entry to remove; `hook rm` reports that and exits non-zero rather than silently succeeding, which is the same silent-success shape as the `:.` bug on the write side.
```

**Proposed Change**:
Two paragraphs appended to §4.2 recording that deregistration against an already-closed pane is the routine SessionEnd case (61 of 63 `:.` lines were `op=rm`), that its current benign exit-0 end state is why it went unnoticed, and that a caller reading `rc` will now see non-zero as a matter of course — the second reason §8.4's external script needs updating in step.

**Resolution**: Approved
**Notes**: Auto-approved. Records the operational consequence only; the decision to extend B to `hook rm` is settled in the source and is not reopened.

---

### 4. Bounded lock acquisition and the 2-second bound are decided by no source

**Source**: No source decides this. Checked: `investigation/resume-hooks-silently-lost.md` — Fix Direction D ("A cross-process file lock around `hooks.json` load→mutate→write… the in-house precedent is `state.AcquireDaemonLock`'s `flock` on `daemon.lock`, and **`flock` is kernel-released on process death, so there is no stale-lock hazard**. Constraint: the locked region covers the file only"); Refinements, "The lock is a sidecar, never `hooks.json` itself"; Refinements, "Readers take a shared lock, writers exclusive"; Risk Assessment ("The riskiest piece is D, which introduces a blocking path into a loop the daemon runs every 10 seconds"); Discussion (the user's "is there a reason not to" exchange on D)
**Category**: Unsourced decision
**Affects**: §6.5 (Acquisition is bounded, and a timeout degrades rather than wedges)

**Details**:
The specification introduces three normative choices in §6.5 that no source makes:

- that acquisition is **bounded** at all, rather than a plain blocking `LOCK_EX`;
- the **2-second** bound;
- the per-caller **degradation policy** — daemon sweep skips the cycle with a WARN and retries on the next 10s cadence; CLI exits non-zero with the reason.

The investigation's treatment of D is the opposite in emphasis: it names the absence of a stale-lock hazard (`flock` is kernel-released on process death) as a *reason the lock is safe*, and states the single constraint as "the locked region covers the file only, never the tmux enumeration that precedes it" — which the spec captures separately in §6.4. It records the daemon's 10s loop as the risk, but draws no timeout from it. The spec's §6.5 reasoning ("An unbounded `LOCK_EX` is simpler and carries no stale-lock hazard… It is rejected because…") is an argument the source record never had.

This is a real design choice with consequences on both sides: a bound converts a wedged holder into a skipped sweep or a failed CLI command, and 2s sets where "contended" becomes "genuinely wrong". The spec quotes "roughly three orders of magnitude above the expected hold" as the derivation, but the expected hold, the bound and the degradation behaviour all originate in the spec rather than in the investigation.

**Proposed Change**:
(Blank — the fix belonged to the source record, not the specification.)

**Resolution**: Routed
**Notes**: Raised as a documented-sides conflict; the user chose bounded. Landed in the investigation as a new Settled refinement bullet ("Acquisition is bounded at 2 seconds, and a timeout degrades rather than wedges"), reindexed and committed at `06b4bc79`. The spec's §6.5 already stated bounded/2s/degradation, so only its rejection argument was re-aligned: kernel-release-on-death rules out a leaked lock, not a held one, and the midnight day-roll deadlock is the project's own precedent for a wedged daemon.

---

### 5. The lock's failure policy omits the hook-firing read path

**Source**: `investigation/resume-hooks-silently-lost.md` — Refinements, "Readers take a shared lock, writers exclusive" ("`Store.Load` is on the path of `hook list`, `LookupOnResume`, doctor's check and the sweep's own pre-read. During a restore of a 40+ pane working set every hydrate helper calls `LookupOnResume` at once"); Finding 5 / Fix Direction A (hook firing runs inside the hydrate helper's exec chain)
**Category**: Gap/Ambiguity
**Affects**: §6.5 (Acquisition is bounded, and a timeout degrades rather than wedges), §6.3 (Readers take a shared lock, writers exclusive)

**Details**:
The source names four `Store.Load` consumers, and §6.3 carries all four across faithfully — including the one the source singles out for its concurrency profile: 40+ hydrate helpers calling `LookupOnResume` simultaneously during a restore. §6.5 then states the failure policy for only two of them: "the **daemon sweep**" and "the **CLI** (`hook set`, `hook rm`, `hook list`)". `LookupOnResume` (the hydrate helper, run per restored pane through `respawn-pane -k`) and `checkStaleHooks` (doctor's read-only diagnosis) have no stated behaviour when the lock cannot be acquired.

The hydrate path is the one that matters most: it is the moment the hook actually fires, and it runs under exactly the burst the source flagged. A failure or timeout there means the pane hydrates to a bare shell with no resume — the precise user-visible failure this work unit exists to eliminate — and the spec does not say whether that path exits non-zero (which would abort the hydrate chain), proceeds as "no hook", retries, or falls back to an unlocked read. §3.4 establishes that "no hook" is a survivable outcome for a pane (it "restores and hydrates as normal, it simply has nothing to resume"), so a policy exists to be chosen; it just is not chosen here.

The same silence covers the case where the sidecar lock file cannot be opened or created at all (§6.2 specifies `O_CREAT` and never unlinked, but not what any caller does if that open fails) — an I/O path D newly introduces on every one of these callers.

**Proposed Change**:
Split the failure policy by direction. A write that cannot take the lock does not write (sweep skips with a WARN; `hook set`/`hook rm` exit non-zero). A read that cannot take the lock reads anyway, unlocked, at DEBUG — covering `LookupOnResume`, `checkStaleHooks` and `hook list`. §6.3 gains the paragraph establishing why that is safe: `AtomicWrite`'s `os.Rename` means a reader always sees a complete snapshot, so the shared lock is an ordering courtesy, not a correctness requirement. The same split governs the sidecar lock file failing to open at all.

**Resolution**: Approved
**Notes**: Applied to §6.3 and §6.5. The deciding argument is that failing the hydrate read would make this work unit reintroduce its own symptom — a restored pane dropping to a bare shell — for no correctness gain.
