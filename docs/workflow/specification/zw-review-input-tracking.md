---
status: in-progress
created: 2026-01-25
phase: Input Review
topic: zw
---

# Review Tracking: ZW - Input Review

## Findings

### 1. Layout Location Discrepancy

**Source**: zellij-multi-directory.md lines 340-341
**Category**: Potential conflict with existing topic
**Affects**: Session Naming > New Session Flow, Zellij Integration > Layout Discovery

**Details**:
The discussion states layouts live in `~/.config/cx/layouts/` (now `~/.config/zw/layouts/`) - ZW-managed layout files.

The current spec says ZW uses Zellij's native layouts from Zellij's config directory and "ZW does not create or manage layouts."

This is a deliberate simplification vs the discussion, but should be confirmed.

**Proposed Addition**: None (confirm current spec is intentional)

**Resolution**: Pending
**Notes**:

---

### 2. Fuzzy Filtering in TUI

**Source**: cx-design.md line 77
**Category**: Enhancement to existing topic
**Affects**: TUI Design > Keyboard Shortcuts

**Details**:
cx-design mentions "Arrow keys / typing for navigation + fuzzy filter" for the main list.

The current spec doesn't mention fuzzy filtering when typing in the session list - only arrow key navigation.

**Proposed Addition**:
Add to Keyboard Shortcuts table:
| Typing | Fuzzy filter sessions |

**Resolution**: Pending
**Notes**:

---
