# Review Tracking: Resume Hooks Silently Lost - Input Review

## Findings

### 1. Why the key had to change at all — retention alone was never a repair

**Source**: Investigation › Fix Direction › "Deciding factor for A over the alternatives"; Options Explored › "Stop the daemon deleting entries (retention alone)"; Discussion › "The exploration turned on one finding: **drift breaks the lookup, not just the storage.**"
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §5.1

**Problem**:
The specification states that the reaper is not converted to full retention, but never says why stopping the deletion would not have fixed the reported bug on its own. Read as it stands, §1.1 and §5.1 leave the sweep looking like the loss mechanism — which invites the cheap reading that a gentler reaper (or a call-site guard) would have been enough, and makes the whole of A look like scope beyond the defect. The record's actual finding is the opposite and is the one the fix direction turns on: after a move the entry is already unusable regardless of the sweep, because firing looks up a key baked from *saved* state. Capture stores the pane at its new coordinates, so restore bakes the new key while `hooks.json` still holds the old one — the entry survives and still does not fire. Anyone re-opening the "do we really need to change every key on disk?" question during planning has nothing in the specification to answer them with.

**Proposal**:
Carry the source's deciding factor into §5.1, beside the sentence that already declines full retention: drift breaks the lookup, not only the storage, so no amount of retention repairs it and the key itself has to stop being positional.

**Current**:
**Whether the reaper deletes is not changed** — it is not converted to full retention. What changes is what it can identify (§5.2), what it records (§5.3), when it declines to run at all (§5.4), and how it takes the file it mutates (§6).

**Proposed Text**:
**Whether the reaper deletes is not changed** — it is not converted to full retention. What changes is what it can identify (§5.2), what it records (§5.3), when it declines to run at all (§5.4), and how it takes the file it mutates (§6).

Retention alone would not have repaired the defect in any case. Firing looks up a key baked from saved state (`collectArmInfos`), and a moved pane is captured at its new coordinates — so restore bakes the *new* key while `hooks.json` still holds the old one. Under a positional key the entry would survive the sweep and still not fire: drift breaks the lookup, not only the storage, which is why the key itself has to stop being positional rather than the reaper being made gentler.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. This is the deciding argument for change A and the spec had no answer to "why not just stop the reaper deleting?" — a question planning would reasonably reopen.

---

### 2. tmux's own `%N` pane id is not ruled out anywhere in the specification

**Source**: Investigation › Options Explored › "Re-key against tmux's own `%N` pane id instead of a Portal token — *rejected.* `%N` is stable only within a server lifetime and can be recycled by tmux, so it needs the same carry-and-re-stamp machinery as a minted token while being less trustworthy."
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §2.1 (with §2.3 as the passage that removes the old objection)

**Problem**:
The only reason the specification gives for not using tmux's own `%N` pane id is that it does not survive a server restart (§2.3) — and §2.3 then closes exactly that gap with carry-in-`sessions.json`-and-re-stamp. So the document argues an implementer straight into the substitution it means to forbid: `%N` is free, already unique, already on every pane, and the stated objection to it no longer applies once the reboot gap is closed. The record rejects it for a second reason the specification never states — tmux recycles `%N` within a server lifetime — and building on it would put the wrong pane's hook behind a live id, which is a worse failure than the one being fixed. The composite key gets its rejection recorded in §3.1; this one does not.

**Proposal**:
Record the `%N` rejection where the token is introduced, with the source's reason: recycling makes it less trustworthy than a minted token while still requiring the identical carry-and-re-stamp machinery, so it buys nothing.

**Current**:
Minting is reached through an exported function in `internal/session`, beside the shape predicate (§3.2) and reading the same `suffixLen` and `NanoIDAlphabet` directly. `hook set` calls it and names no width of its own: generation and recognition derive from one pair of constants, so they move together or not at all.

**Proposed Text**:
Minting is reached through an exported function in `internal/session`, beside the shape predicate (§3.2) and reading the same `suffixLen` and `NanoIDAlphabet` directly. `hook set` calls it and names no width of its own: generation and recognition derive from one pair of constants, so they move together or not at all.

**tmux's own `%N` pane id is not the identity.** It is stable only within a server lifetime and tmux is free to recycle it, so it needs the identical carry-and-re-stamp machinery a minted token needs (§2.3) while being less trustworthy than one: a recycled id can name a pane that is not the one the entry was written for, which fires a hook on the wrong pane rather than losing it.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. The finding is right that the spec argued a reader *into* the substitution: the only stated objection to `%N` was the server-restart gap, which the carry-and-re-stamp design then closes. Recycling is the reason that survives, and its failure mode is worse than the bug being fixed — a hook firing on the wrong pane rather than not firing.

---

### 3. The repository's own architecture document keeps describing the deleted key scheme

**Source**: Investigation › Finding 5 (records `CLAUDE.md` as already stale against the tmux version this work was verified on); no source addresses the architecture document's account of the hook key. Surfaced as a gap.
**Category**: Gap/Ambiguity
**Move**: settled
**Affects**: §3.3 (and the §7.2 removals it would carry)

**Problem**:
`CLAUDE.md` is the operative description of this machinery — it states the hook key as `<@portal-id or session_name>:window.pane`, documents the `@portal-id` stamp at session creation, `Session.PortalID`, the `#{@portal-id}` capture column, and the `HookKey` / `HookKeyFormat` / `ResolveHookKey` / `ListAllPaneHookKeys` surface. Every one of those is deleted or redefined by this work, and nothing in the specification says the document changes. The specification is otherwise exact about text that would go stale — it names the `--hook-key` help string and the `AllPaneLister` doc comment specifically — so the one document future work actually reads first is the one left describing a key format that no longer exists, and the next person (or agent) to touch hooks starts from a wrong model of the system.

**Proposal**:
Name `CLAUDE.md` in §3.3 alongside the other text surfaces that carry the old key form, so the architecture description moves with the change rather than by habit.

**Proposed Text**:
**`CLAUDE.md`** describes the hook key as `<@portal-id or session_name>:window.pane` and documents the `@portal-id` stamp, `Session.PortalID`, the `#{@portal-id}` capture column and the `HookKey` / `HookKeyFormat` / `ResolveHookKey` / `ListAllPaneHookKeys` surface. Every passage naming those is rewritten to the pane token and the §7.2 removals, so the repository's own architecture description does not go on naming a key scheme that no longer exists.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed, placed with the other text surfaces carrying the old key form. The agent's note is right to leave `CLAUDE.md`'s tmux version alone: some 3.6b references record measurements genuinely taken on 3.6b, and a blanket rewrite would falsify them. The investigation also notes in passing that `CLAUDE.md`'s tmux version (3.6b) is stale against the 3.7c the sandbox work was done on. Deliberately left out of the proposed text — some 3.6b references record measurements genuinely taken on 3.6b (the hysteresis constant, the `show-hooks -g` blind spot) and a blanket version rewrite would falsify them.

---
