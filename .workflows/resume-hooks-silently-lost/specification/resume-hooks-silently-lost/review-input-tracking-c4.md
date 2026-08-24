# Review Tracking: Resume Hooks Silently Lost - Input Review

## Findings

### 1. The rejected in-sweep adoption rule is the one migration alternative the specification leaves open

**Source**: `investigation/resume-hooks-silently-lost.md` — Fix Direction → Options Explored, final bullet: *"**Adoption rule inside the sweep** (\"an entry whose positional key resolves to exactly one live pane is re-keyed to that pane's token\") — rejected. Needs no removal, but once every key is a token the branch never fires again; it is dead code presented as general behaviour."*
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §8.1 (No migration code ships)

**Problem**:
The specification hands a planner two facts side by side — the sweep is being made shape-aware and already enumerates every live pane with its token (§5.2, §3.3), and no migration code ships, so someone must convert 42 entries by hand out of band (§8.1, §8.2). The obvious move from there is to let the sweep re-key an old-format entry that resolves to exactly one live pane, which costs no new tmux read and removes the manual step entirely. That option was raised and rejected in the record; the specification records the two migration alternatives that were rejected and not this one, so nothing in the document stops it being built. Building it puts a permanently dead branch in the reaper, and it re-keys blind: §8.3 establishes that a pane already carrying a token has a current entry under it and its old-format entry must be **dropped, not re-keyed** — a judgement the sweep cannot make, so it would overwrite the newer command with the superseded one on the pane where both exist.

**Proposal**:
Add the rejected alternative as a third bullet in §8.1's list, carrying the record's own reason — the branch can never fire again once every key is a token — and update the lead-in count from two to three.

**Current**:
```
Two alternatives were rejected:

- **Using `CleanStale` as the migration** — the precedent from spec `resume-hooks-lost-on-server-restart` (2026-04-30), which accepted a breaking key change and let the sweep absorb the old entries. Repeating it here would silently destroy every existing hook on the first sweep after upgrade.
- **A one-release migration, isolated and deleted in the next release** — ships code whose whole purpose is to become obsolete, and leaves a removal the user has to remember.
```

**Proposed Text**:
```
Three alternatives were rejected:

- **Using `CleanStale` as the migration** — the precedent from spec `resume-hooks-lost-on-server-restart` (2026-04-30), which accepted a breaking key change and let the sweep absorb the old entries. Repeating it here would silently destroy every existing hook on the first sweep after upgrade.
- **A one-release migration, isolated and deleted in the next release** — ships code whose whole purpose is to become obsolete, and leaves a removal the user has to remember.
- **An adoption rule inside the sweep** — re-keying an entry whose positional key resolves to exactly one live pane onto that pane's token, riding the enumeration the sweep already performs so the conversion needs no script. Rejected: once every key is a token the branch can never fire again, so it is dead code presented as general behaviour. It also cannot make the call §8.3 requires — dropping a superseded old-format entry rather than re-keying it over the newer one — because the sweep has no way to tell the two apart.
```

**Resolution**: Pending
**Notes**:
