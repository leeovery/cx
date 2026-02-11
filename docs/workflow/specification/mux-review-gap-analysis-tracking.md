---
status: in-progress
created: 2026-02-11
phase: Gap Analysis
topic: mux
---

# Review Tracking: mux - Gap Analysis

## Findings

### 1. `mux <alias>` behavior contradicts between sections

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: CLI Interface > Argument Resolution, Configuration & Storage > Aliases
**Priority**: Important

**Details**:
Two sections describe `mux <alias>` behavior differently:
- **CLI Commands table + Quick-start shortcuts**: "Start new session for project with matching alias" / "For saved projects, session creation is immediate (no prompts)." → Implies direct session creation.
- **Aliases note** (under projects.json): "Enables quick session start: `mux app` opens the project picker for that project directly." → Implies opening the project picker.

An implementer wouldn't know whether `mux app` should (a) create a session immediately or (b) open the project picker with that project highlighted.

**Proposed Addition**:
(Pending discussion)

**Resolution**: Pending
**Notes**:

---

### 2. `last_used` timestamp update timing unspecified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Configuration & Storage > projects.json, Sorting
**Priority**: Important

**Details**:
The spec defines `last_used` for sorting projects by recency but never says when it's updated. Options:
- Only when a project is first added to projects.json
- Every time a session is started in that project (regardless of entry point)

This determines whether frequently-used projects stay at the top of the project picker. If only set on creation, ordering becomes stale over time.

**Proposed Addition**:
(Pending discussion)

**Resolution**: Pending
**Notes**:

---

### 3. Stale project cleanup timing vague

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Project Memory > Stale Project Cleanup
**Priority**: Minor

**Details**:
The spec says "If a remembered directory no longer exists on disk, mux removes it from the project list automatically when encountered." The word "encountered" is ambiguous — does this mean when loading projects.json at startup, when displaying the project picker, or when the project is specifically highlighted/selected?

**Proposed Addition**:
(Pending discussion)

**Resolution**: Pending
**Notes**:

---

### 4. Empty session list misleading when inside tmux with single session

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Running Inside tmux > TUI Differences, TUI Design > Empty States
**Priority**: Minor

**Details**:
When inside tmux and there's only one session (the current one), the session list is empty because the current session is excluded. The TUI would show "No active sessions" — which is technically incorrect (there IS an active session, just not one to switch TO). The header shows the current session name, providing context, but the empty state text is misleading.

**Proposed Addition**:
(Pending discussion)

**Resolution**: Pending
**Notes**:

---

### 5. Alias uniqueness enforcement unspecified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Project Memory > Project Naming, Configuration & Storage
**Priority**: Minor

**Details**:
The spec states aliases "must be unique across all projects" but doesn't specify:
- When uniqueness is enforced (project creation? editing? both?)
- What happens on violation (error message? prevent save?)
- How duplicate aliases are handled if introduced via manual JSON editing (first match? error at startup?)

This affects the argument resolution flow — `mux <alias>` expects a single match.

**Proposed Addition**:
(Pending discussion)

**Resolution**: Pending
**Notes**:
