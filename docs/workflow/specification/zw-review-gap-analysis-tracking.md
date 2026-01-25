---
status: in-progress
created: 2026-01-25
phase: Gap Analysis
topic: zw
---

# Review Tracking: ZW - Gap Analysis

## Findings

### 1. CLI Argument Disambiguation

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: CLI Interface > Commands
**Priority**: Critical

**Details**:
`zw <path>` and `zw <alias>` use the same positional argument. How does ZW distinguish between them?

Example: User runs `zw myapp`
- Is this an alias?
- Is this a relative path `./myapp`?
- Is this a project name?

The spec doesn't define the resolution order.

**Proposed Addition**: (pending discussion)

**Resolution**: Pending
**Notes**:

---

### 2. File Browser Navigation Details

**Source**: Specification analysis
**Category**: Insufficient detail
**Affects**: File Browser > Behavior
**Priority**: Important

**Details**:
The file browser section says "Navigate directories using arrow keys" but doesn't specify:
- Starting directory (home? current working directory?)
- How to go up a directory level (Backspace? Left arrow? Esc?)
- How to confirm directory selection (Enter?)
- How to cancel and return to project picker

**Proposed Addition**: (pending discussion)

**Resolution**: Pending
**Notes**:

---

### 3. Utility Mode TUI

**Source**: Specification analysis
**Category**: Insufficient detail
**Affects**: Running Inside Zellij > Utility Mode
**Priority**: Important

**Details**:
Utility mode lists allowed/blocked operations but doesn't describe the UI:
- What does the screen look like?
- Is it the same layout with attach disabled?
- Is there a visual indicator that you're in utility mode?
- How do you access rename/kill operations?

**Proposed Addition**: (pending discussion)

**Resolution**: Pending
**Notes**:

---

### 4. Empty State Handling

**Source**: Specification analysis
**Category**: Gap
**Affects**: TUI Design
**Priority**: Important

**Details**:
What does the TUI show when:
- No sessions exist (running or exited)?
- No remembered projects exist?

Should show something helpful, not just empty sections.

**Proposed Addition**: (pending discussion)

**Resolution**: Pending
**Notes**:

---

### 5. Session List Sorting

**Source**: Specification analysis
**Category**: Ambiguity
**Affects**: TUI Design > Sections
**Priority**: Minor

**Details**:
Sessions are queried from `zellij list-sessions`. In what order are they displayed in the TUI?
- As returned by Zellij?
- Alphabetically?
- By attached status (attached first)?

Projects have `last_used` for sorting, but sessions don't have defined sort order.

**Proposed Addition**: (pending discussion)

**Resolution**: Pending
**Notes**:

---
