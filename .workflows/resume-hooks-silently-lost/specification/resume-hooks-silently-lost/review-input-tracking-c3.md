# Review Tracking: Resume Hooks Silently Lost - Input Review

## Findings

### 1. The fact that makes the out-of-band conversion complete is not recorded

**Source**: `investigation/resume-hooks-silently-lost.md` — Standing Evidence, "The install is currently clean" (*"`hooks.json` holds 42 entries; every one matches a live pane key"*); Discussion (*"Working through it showed the migration did not need to be code at all — a single install, 41 entries, all currently resolving exactly"*); Blast Radius, "The 41 existing on-disk `hooks.json` keys"
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §8.1 (No migration code ships); supports §8.2 and §5.2's "which the out-of-band conversion (§8) clears"

**Problem**:
§8 asks the reader to accept two things together: that no migration code ships, and that a throwaway script which "resolve[s] each entry's positional key to its live pane" (§8.2) is sufficient to clear every old-format entry. That second claim only holds if every entry on disk actually resolves to a live pane — an entry whose pane closed before the script runs cannot be re-keyed, and under §5.2 it is retained forever rather than swept, invisible to `portal doctor` (§5.4). The specification never records that this condition holds on the install, so the sufficiency of the script — and §5.2's statement that the conversion clears the retained set — rests on an unstated premise. A reader weighing the no-migration decision, or deciding how much residue §5.2's retention rule can leave behind, has no way to see that the file is fully convertible.

The source treats this as the fact that settled the decision, not as background: the migration "did not need to be code at all" because there was a single install with every entry resolving exactly. §8.1 carries only the first half of that.

**Proposal**:
Carry the sourced on-disk fact into §8.1's justification paragraph, beside the one-install point: every existing entry resolves to a live pane, so the one-time conversion is a complete transformation of the file rather than a partial one. This is the source's own second reason for shipping no migration code, and it is what §8.2's "resolve each entry's positional key to its live pane" presupposes.

**Current**:
```
Portal has one install and no evidence of any other. The user's call is that a second install, if one exists, is not worth carrying compatibility code for.
```

**Proposed Text**:
```
Portal has one install and no evidence of any other, and every entry on that install resolves: `hooks.json` held 42 entries on 2026-08-22 and each one matched a live pane key. The conversion (§8.2) is therefore a complete transformation of the file rather than a partial one — no entry names a pane that is already gone and so could not be re-keyed. The user's call is that a second install, if one exists, is not worth carrying compatibility code for.
```

**Resolution**: Pending
**Notes**:
