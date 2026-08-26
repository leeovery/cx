# Review Tracking: Resume Hooks Silently Lost - Traceability

## Findings

### 1. Two different repair messages are specified for the same stand-down

**Type**: Incomplete coverage
**Spec Reference**: §5.1 (the three fixed `Skipped stale hook prune: …` lines `portal doctor --fix` prints), against the cycle-1 integrity decision that deliberately widened the restore phrase
**Plan Reference**: Phase 1 — the phase's **Acceptance** block in `planning.md` (7th bullet), against task `resume-hooks-silently-lost-1-4`
**Move**: settled
**Change Type**: update-task

**Problem**:
When a user runs the repair command and the hook prune declines to run, they are shown one line explaining why. The plan specifies that line twice, in two different wordings: the phase's own acceptance check demands `Skipped stale hook prune: restore in progress`, while the task that builds the line and its test both demand `Skipped stale hook prune: restore may be in progress`. Whichever an implementer follows, the other fails — the phase cannot be signed off against its own tasks. The weaker wording is the one that was deliberately chosen, because the same line prints on a machine with no tmux server running at all, where telling the user a restore is under way would be a fresh false statement on the one command that exists to report honestly on a broken install. Leaving the stronger wording in the phase check invites it back at sign-off, undoing that decision without anyone noticing they undid it.

**Proposal**:
Carry the settled wording into the phase acceptance so the plan states the message once. The choice was made and recorded in cycle 1 (widen only the plan-owned prose; leave the logged reason value and the closed three-reason vocabulary untouched) and applied to tasks 1-4 and 1-5 — the phase-level check is the one place it was not applied. Nothing else in the bullet changes: the callback, and the rule that a stand-down never moves the exit code, are unaffected.

**Current**:
```markdown
- [ ] `portal doctor --fix` prints `Skipped stale hook prune: restore in progress` through the new `onSkipped` callback, with the exit code still driven solely by the post-repair diagnosis
```

**Proposed Text**:
```markdown
- [ ] `portal doctor --fix` prints `Skipped stale hook prune: restore may be in progress` through the new `onSkipped` callback, with the exit code still driven solely by the post-repair diagnosis
```

**Resolution**: Fixed
**Notes**:

---

### 2. The architecture doc keeps teaching that a hook key is a pane coordinate

**Type**: Missing from plan
**Spec Reference**: §3.3 (`CLAUDE.md` describes the retired key scheme, and every passage naming it is rewritten so the repository's own architecture description does not go on naming a key scheme that no longer exists)
**Plan Reference**: Phase 3, task `resume-hooks-silently-lost-3-4` — the CLAUDE.md **Do** bullet, its **Acceptance Criteria** and its **Edge Cases**
**Move**: settled
**Change Type**: add-to-task

**Problem**:
After this work a hook key is an opaque pane token that carries no window or pane number at all — that is the entire repair. The resume-hooks section of the repository's architecture description still ends with the sentence *"The key uses **saved** window/pane indices so lookups stay addressable across base-index drift."* No task names it, and it survives every check the plan does apply: it contains none of the three strings (`@portal-id`, `PortalID`, `portal_id`) the final grep looks for, so the plan can report a clean sweep while the document still explains the hook key as a coordinate. The next person to touch this code — or the next agent briefed from that document — reads the defect's own root cause back as the current design, which is exactly the "source comments cross-referencing a key format that no longer exists" outcome the removal of the old machinery was justified on avoiding.

**Proposal**:
Name the sentence in the same pass that rewrites the rest of that section, and add a check that does not depend on the grep. The specification's instruction is that every passage naming the retired scheme is rewritten, not merely every occurrence of the retired option name; this sentence is such a passage and is the only one the plan's grep-shaped criterion structurally cannot catch. The replacement states what is true instead — the key is the pane's own token, so a lookup is addressable regardless of how tmux renumbers windows and panes — which is the property the surrounding paragraph exists to explain.

**Current**:

*Do — the CLAUDE.md bullet:*
```markdown
- Rewrite CLAUDE.md's remaining `@portal-id` passages in one pass: the `session` row's two-stamp description and its QuickStart chain (both now `@portal-dir` only); the `state` row, which loses `Session.PortalID` and the `#{@portal-id}` capture column and gains what 3-1 and 3-2 built — `Pane.PortalPaneID` (`json:"portal_pane_id"`, additive, tolerant-decode, no `SchemaVersion` bump), the trailing `#{@portal-pane-id}` `captureFormat` column lifted per-pane at unchanged arity, and restore's per-pane re-stamp; the `tmux` row's `HookKey` clause; and the "Resume hooks" section's remaining re-stamp and "must never read the live `@portal-id`" claims. End with zero `@portal-id` occurrences while leaving `@portal-dir`, `@portal-pane-id`, `@portal-skeleton-`, `@portal-restoring` and `@portal-spawn-` intact — a blanket find-and-replace would corrupt all five.
```

*Acceptance Criteria — the CLAUDE.md line:*
```markdown
- [ ] CLAUDE.md contains zero occurrences of `@portal-id`, `PortalID` and `portal_id`, still contains `@portal-dir`, `@portal-pane-id`, `@portal-skeleton-`, `@portal-restoring` and `@portal-spawn-`, and its `state` row describes `Pane.PortalPaneID`, the per-pane capture column and the restore re-stamp
```

*Edge Cases — the blanket-replace line:*
```markdown
- A blanket `@portal-id` find-and-replace across CLAUDE.md corrupts `@portal-dir`, `@portal-pane-id`, `@portal-skeleton-`, `@portal-restoring` and `@portal-spawn-`; the passages are rewritten by hand
```

**Proposed Text**:

*Do — the CLAUDE.md bullet:*
```markdown
- Rewrite CLAUDE.md's remaining `@portal-id` passages in one pass: the `session` row's two-stamp description and its QuickStart chain (both now `@portal-dir` only); the `state` row, which loses `Session.PortalID` and the `#{@portal-id}` capture column and gains what 3-1 and 3-2 built — `Pane.PortalPaneID` (`json:"portal_pane_id"`, additive, tolerant-decode, no `SchemaVersion` bump), the trailing `#{@portal-pane-id}` `captureFormat` column lifted per-pane at unchanged arity, and restore's per-pane re-stamp; the `tmux` row's `HookKey` clause; and the "Resume hooks" section's remaining re-stamp and "must never read the live `@portal-id`" claims. In that same section, rewrite the sentence "The key uses **saved** window/pane indices so lookups stay addressable across base-index drift" — the token carries no window or pane number, so the claim is false and the property it explains now comes from the token itself: a baked key is the pane's own saved token, addressable regardless of how tmux renumbers windows and panes on restore. It holds none of the three strings the grep below looks for, so it must be found by reading the section rather than by searching it. End with zero `@portal-id` occurrences while leaving `@portal-dir`, `@portal-pane-id`, `@portal-skeleton-`, `@portal-restoring` and `@portal-spawn-` intact — a blanket find-and-replace would corrupt all five.
```

*Acceptance Criteria — the CLAUDE.md line, plus one added below it:*
```markdown
- [ ] CLAUDE.md contains zero occurrences of `@portal-id`, `PortalID` and `portal_id`, still contains `@portal-dir`, `@portal-pane-id`, `@portal-skeleton-`, `@portal-restoring` and `@portal-spawn-`, and its `state` row describes `Pane.PortalPaneID`, the per-pane capture column and the restore re-stamp
- [ ] CLAUDE.md's "Resume hooks" section makes no claim that a hook key uses saved window/pane indices, and describes the baked key as the pane's saved token instead
```

*Edge Cases — the blanket-replace line, plus one added below it:*
```markdown
- A blanket `@portal-id` find-and-replace across CLAUDE.md corrupts `@portal-dir`, `@portal-pane-id`, `@portal-skeleton-`, `@portal-restoring` and `@portal-spawn-`; the passages are rewritten by hand
- The zero-occurrences grep is not a completeness check on the rewrite: the "Resume hooks" section's saved-window/pane-indices sentence names none of the three strings, so a document that passes the grep can still describe the hook key as a coordinate
```

**Resolution**: Fixed
**Notes**:

---
