---
status: complete
created: 2026-02-23
cycle: 1
phase: Plan Integrity Review
topic: portal
---

# Review Tracking: Portal - Integrity

## Findings

### 1. Session Name Generation Task Missing Outcome Field

**Severity**: Important
**Plan Reference**: Phase 2 / portal-2-2 (tick-4ebf36)
**Category**: Task Template Compliance
**Change Type**: update-task

**Details**:
Task portal-2-2 (Session Name Generation) is missing the required **Outcome** field. Every task must have Problem, Solution, and Outcome per the task template. Without it, the implementer has no clear statement of what success looks like.

**Current**:
```
**Problem**: Portal needs to auto-generate unique tmux session names in format {project-name}-{nanoid}. Project name must be sanitised (no periods or colons), and collisions must be handled.

**Solution**: Implement GenerateSessionName function that sanitises name, appends 6-char nanoid, checks for collisions, retries up to 10 times.

**Do**:
- Create internal/session/naming.go
- SanitiseProjectName: replace . and : with -
- GenerateSessionName(projectName, exists func) returns unique name
- Injectable nanoid generator for testing

**Acceptance Criteria**:
- [ ] Returns name matching pattern {project}-[a-zA-Z0-9]{6}
- [ ] Periods replaced with hyphens
- [ ] Colons replaced with hyphens
- [ ] Retries on collision
- [ ] Error after max retries

**Tests**:
- generates name in correct format
- sanitises periods/colons
- retries on collision
- returns error after max retries

**Spec Reference**: .workflows/specification/portal/specification.md -- Session Naming section
```

**Proposed**:
```
**Problem**: Portal needs to auto-generate unique tmux session names in format {project-name}-{nanoid}. Project name must be sanitised (no periods or colons), and collisions must be handled.

**Solution**: Implement GenerateSessionName function that sanitises name, appends 6-char nanoid, checks for collisions, retries up to 10 times.

**Outcome**: Tested session name generation that produces sanitised, unique names in {project}-{nanoid} format with collision retry.

**Do**:
- Create internal/session/naming.go
- SanitiseProjectName: replace . and : with -
- GenerateSessionName(projectName, exists func) returns unique name
- Injectable nanoid generator for testing

**Acceptance Criteria**:
- [ ] Returns name matching pattern {project}-[a-zA-Z0-9]{6}
- [ ] Periods replaced with hyphens
- [ ] Colons replaced with hyphens
- [ ] Retries on collision
- [ ] Error after max retries

**Tests**:
- generates name in correct format
- sanitises periods/colons
- retries on collision
- returns error after max retries
- handles empty project name

**Edge Cases**:
- Empty project name: produces -{nanoid} format (or error, to be decided)
- Max collision retries exhausted: returns descriptive error

**Spec Reference**: .workflows/specification/portal/specification.md -- Session Naming section
```

**Resolution**: Fixed
**Notes**: Applied to tick-4ebf36. Also added the missing "empty project name" edge case.

---

### 2. Project Store Task Missing Outcome Field

**Severity**: Important
**Plan Reference**: Phase 2 / portal-2-3 (tick-248cf4)
**Category**: Task Template Compliance
**Change Type**: update-task

**Details**:
Task portal-2-3 (Project Store) is missing the required **Outcome** field.

**Current**:
```
**Problem**: Portal needs to persist remembered project directories in projects.json at ~/.config/portal/.

**Solution**: Implement ProjectStore with CRUD operations. Load handles missing/malformed files gracefully. Save uses atomic writes. Upsert by path. List sorted by last_used.

**Do**:
- Create internal/project/store.go
- Project struct: Path, Name, LastUsed
- Load(): handle missing file, malformed JSON
- Save(): create dir, atomic write (temp+rename)
- Upsert(path, name): add/update project
- List(): sorted by last_used descending
- Remove(path): delete entry

**Acceptance Criteria**:
- [ ] Load returns empty list when file missing
- [ ] Load returns empty list for malformed JSON
- [ ] Save creates config directory
- [ ] Upsert adds new or updates existing
- [ ] List sorted by last_used descending

**Tests**:
- loads empty list when file does not exist
- loads projects from valid JSON
- handles malformed JSON
- creates config directory on save
- upsert adds/updates project
- list returns sorted projects

**Spec Reference**: .workflows/specification/portal/specification.md -- Configuration & Storage, projects.json Structure
```

**Proposed**:
```
**Problem**: Portal needs to persist remembered project directories in projects.json at ~/.config/portal/.

**Solution**: Implement ProjectStore with CRUD operations. Load handles missing/malformed files gracefully. Save uses atomic writes. Upsert by path. List sorted by last_used.

**Outcome**: Tested ProjectStore with Load, Save, Upsert, List, and Remove operations persisting projects to ~/.config/portal/projects.json with graceful handling of missing/malformed files.

**Do**:
- Create internal/project/store.go
- Project struct: Path, Name, LastUsed
- Load(): handle missing file, malformed JSON
- Save(): create dir, atomic write (temp+rename)
- Upsert(path, name): add/update project
- List(): sorted by last_used descending
- Remove(path): delete entry

**Acceptance Criteria**:
- [ ] Load returns empty list when file missing
- [ ] Load returns empty list for malformed JSON
- [ ] Save creates config directory
- [ ] Upsert adds new or updates existing
- [ ] List sorted by last_used descending

**Tests**:
- loads empty list when file does not exist
- loads projects from valid JSON
- handles malformed JSON
- creates config directory on save
- upsert adds/updates project
- list returns sorted projects

**Spec Reference**: .workflows/specification/portal/specification.md -- Configuration & Storage, projects.json Structure
```

**Resolution**: Fixed
**Notes**: Applied to tick-248cf4

---

### 3. Stale Project Cleanup Task Missing Outcome Field

**Severity**: Important
**Plan Reference**: Phase 2 / portal-2-4 (tick-275824)
**Category**: Task Template Compliance
**Change Type**: update-task

**Details**:
Task portal-2-4 (Stale Project Cleanup) is missing the required **Outcome** field.

**Current**:
```
**Problem**: Project directories can be deleted after being remembered. Stale projects should be automatically removed when project picker is displayed.

**Solution**: Implement CleanStale() method on ProjectStore. Check each directory with os.Stat, remove non-existent ones, retain permission-denied dirs.

**Do**:
- Add CleanStale() (int, error) to ProjectStore
- Iterate projects, os.Stat each path
- If os.IsNotExist, mark for removal
- If permission error, keep the project
- Save only if changes made
- Return count of removed

**Acceptance Criteria**:
- [ ] Removes projects with non-existent directories
- [ ] Retains projects with existing directories
- [ ] Retains projects with permission denied
- [ ] Returns 0 when no stale projects

**Tests**:
- removes project with non-existent directory
- retains project with existing directory
- retains project with permission denied
- returns zero on empty list
- removes multiple stale in single call

**Spec Reference**: .workflows/specification/portal/specification.md -- Stale Project Cleanup section
```

**Proposed**:
```
**Problem**: Project directories can be deleted after being remembered. Stale projects should be automatically removed when project picker is displayed.

**Solution**: Implement CleanStale() method on ProjectStore. Check each directory with os.Stat, remove non-existent ones, retain permission-denied dirs.

**Outcome**: CleanStale() removes projects with non-existent directories, retains those with permission errors, and returns the count of removed entries.

**Do**:
- Add CleanStale() (int, error) to ProjectStore
- Iterate projects, os.Stat each path
- If os.IsNotExist, mark for removal
- If permission error, keep the project
- Save only if changes made
- Return count of removed

**Acceptance Criteria**:
- [ ] Removes projects with non-existent directories
- [ ] Retains projects with existing directories
- [ ] Retains projects with permission denied
- [ ] Returns 0 when no stale projects

**Tests**:
- removes project with non-existent directory
- retains project with existing directory
- retains project with permission denied
- returns zero on empty list
- removes multiple stale in single call

**Spec Reference**: .workflows/specification/portal/specification.md -- Stale Project Cleanup section
```

**Resolution**: Fixed
**Notes**: Applied to tick-275824

---

### 4. Task portal-1-6 Contains tmux Dependency Check That Should Be in Task portal-6-6

**Severity**: Important
**Plan Reference**: Phase 1 / portal-1-6 (tick-9776d9) and Phase 6 / portal-6-6 (tick-a4b5c6)
**Category**: Scope and Granularity
**Change Type**: update-task

**Details**:
Task portal-1-6 (Attach on Enter) includes "If LookPath fails, print 'Portal requires tmux. Install with: brew install tmux' and exit 1" in its Do section. However, Phase 6 has a dedicated task (portal-6-6: tmux Runtime Dependency Check) that centralises this check and explicitly says "Remove any ad-hoc LookPath checks from other commands (e.g., the one in Task 1-6 attach flow) and centralise here." This creates contradictory instructions: task 1-6 tells the implementer to add an ad-hoc check, and task 6-6 tells them to remove it. The implementer of task 1-6 should use LookPath to find the tmux binary path for exec (which is necessary), but should not implement the user-facing dependency error message -- that is task 6-6's scope.

**Current**:
(Task portal-1-6 Do section)
```
**Do**:
- In internal/tui/model.go, add selected string field
- Handle tea.KeyEnter: if sessions loaded and cursor valid, set m.selected = m.sessions[m.cursor].Name, return tea.Quit
- Add Selected() string method for caller to retrieve chosen session
- In cmd/open.go, after tea.NewProgram().Run() returns, check model.Selected()
- If non-empty, call syscall.Exec with tmux attach-session -t <name>
- Use exec.LookPath("tmux") to find binary path
- If LookPath fails, print "Portal requires tmux. Install with: brew install tmux" and exit 1
- If Selected() empty (user quit), exit cleanly with code 0
- Add tests in internal/tui/model_test.go
```

**Proposed**:
(Task portal-1-6 Do section)
```
**Do**:
- In internal/tui/model.go, add selected string field
- Handle tea.KeyEnter: if sessions loaded and cursor valid, set m.selected = m.sessions[m.cursor].Name, return tea.Quit
- Add Selected() string method for caller to retrieve chosen session
- In cmd/open.go, after tea.NewProgram().Run() returns, check model.Selected()
- If non-empty, call syscall.Exec with tmux attach-session -t <name>
- Use exec.LookPath("tmux") to find binary path for syscall.Exec (required to get absolute path)
- If LookPath fails, return error (centralised tmux dependency check is added in Phase 6, task portal-6-6)
- If Selected() empty (user quit), exit cleanly with code 0
- Add tests in internal/tui/model_test.go
```

**Resolution**: Fixed
**Notes**: Applied to tick-9776d9. LookPath still needed for path, user-facing message moved to task 6-6.

---

### 5. Keyboard Navigation Edge Case: Mismatch Between Spec and Task

**Severity**: Minor
**Plan Reference**: Phase 1 / portal-1-4 (tick-27a6f2)
**Category**: Acceptance Criteria Quality
**Change Type**: update-task

**Details**:
Task portal-1-4 says "navigation is no-op with single session" in tests. This is correct. However, the acceptance criteria say "Cursor does not go above first item" and "Cursor does not go below last item" which describes clamping, not wrapping. The task description says "Cursor wraps at boundaries" in the Outcome, contradicting the Do section which says to clamp. The Outcome should say "clamps" not "wraps."

**Current**:
```
**Outcome**: Arrow keys and j/k move cursor through session list. Cursor wraps at boundaries. View reflects cursor position after each keypress.
```

**Proposed**:
```
**Outcome**: Arrow keys and j/k move cursor through session list. Cursor clamps at boundaries (does not wrap). View reflects cursor position after each keypress.
```

**Resolution**: Fixed
**Notes**: Applied to tick-27a6f2. Changed "wraps" to "clamps" in Outcome.

---

### 6. Filter Mode Tasks (5-7, 5-8) Would Benefit From Being a Single Task

**Severity**: Minor
**Plan Reference**: Phase 5 / portal-5-7 (tick-c1d2e3) and portal-5-8 (tick-f4a5b6)
**Category**: Scope and Granularity
**Change Type**: update-task

**Details**:
Tasks 5-7 (Filter Mode Activation and Fuzzy Matching) and 5-8 (Filter Mode Exit Behaviour) are tightly coupled -- filter mode exit behaviour cannot be tested without filter mode activation. Task 5-8 handles Backspace and Esc in filter mode, but these keys only have meaning when filter mode is active (task 5-7). While the natural ordering means 5-7 executes first, this is borderline on the "too small" side for task 5-8 -- it is essentially "add two key handlers to an existing mode." However, since both tasks have sufficient test coverage independently and the natural order is correct, this is minor. No change proposed -- just flagged for awareness that an implementer might reasonably combine them.

**Current**:
N/A -- observation only.

**Proposed**:
N/A -- no change proposed. Keeping as two tasks is acceptable since both have meaningful test sets.

**Resolution**: Acknowledged
**Notes**: Observation only. No change needed - tasks work fine given natural ordering.

---

### 7. Project Picker Filter Mode Added to Task 2-6 But Not in Phase 2 Acceptance Criteria

**Severity**: Important
**Plan Reference**: Phase 2 / portal-2-6 (tick-4c54e1) and Phase 2 acceptance criteria
**Category**: Phase Structure
**Change Type**: update-task

**Details**:
Task portal-2-6 (Project Picker TUI View) includes filter mode via `/` key with fuzzy matching, Backspace/Esc behaviour. However, the Phase 2 acceptance criteria do not mention project picker filter mode at all. This means Phase 2 could be considered "accepted" without the filter mode being verified. The filter mode in the project picker is distinct from the session list filter mode (Phase 5) -- this is the project picker's own filter for narrowing projects. Since the task already defines this behaviour, the phase acceptance criteria should reflect it.

**Current**:
(Phase 2 acceptance criteria)
```
**Acceptance**:
- [ ] Selecting "[n] new in project..." from main TUI opens the project picker
- [ ] Project picker lists remembered projects sorted by last_used, with "browse for directory..." always at bottom
- [ ] Selecting a project creates a new tmux session with auto-generated name ({project}-{nanoid}) and attaches
- [ ] Session name sanitisation replaces `.` and `:` with `-`; collision with existing tmux session regenerates nanoid
- [ ] Git root resolution applied: `git -C <dir> rev-parse --show-toplevel`; non-git directories used as-is
- [ ] projects.json created/updated at ~/.config/portal/ with path, name, last_used fields
- [ ] Stale project cleanup runs automatically when project picker is displayed
- [ ] Empty state ("No saved projects yet.") displays when no projects remembered, with browse option still visible
```

**Proposed**:
(Phase 2 acceptance criteria)
```
**Acceptance**:
- [ ] Selecting "[n] new in project..." from main TUI opens the project picker
- [ ] Project picker lists remembered projects sorted by last_used, with "browse for directory..." always at bottom
- [ ] Selecting a project creates a new tmux session with auto-generated name ({project}-{nanoid}) and attaches
- [ ] Session name sanitisation replaces `.` and `:` with `-`; collision with existing tmux session regenerates nanoid
- [ ] Git root resolution applied: `git -C <dir> rev-parse --show-toplevel`; non-git directories used as-is
- [ ] projects.json created/updated at ~/.config/portal/ with path, name, last_used fields
- [ ] Stale project cleanup runs automatically when project picker is displayed
- [ ] Empty state ("No saved projects yet.") displays when no projects remembered, with browse option still visible
- [ ] `/` activates filter mode in project picker; fuzzy-matches against project names; browse option always visible
```

**Resolution**: Fixed
**Notes**: Applied to plan.md - added filter mode to Phase 2 acceptance criteria.

---

### 8. Homebrew Tap Formula Reference in Phase 6 Acceptance Criteria Missing from GoReleaser Task

**Severity**: Minor
**Plan Reference**: Phase 6 / portal-6-9 (tick-c4d5e6)
**Category**: Task Self-Containment
**Change Type**: update-task

**Details**:
Phase 6 acceptance includes "Homebrew formula in `leeovery/homebrew-tools` auto-updated by GoReleaser". Task portal-6-8 (GoReleaser Configuration) covers the brew section configuration. Task portal-6-9 (GitHub Actions Release Workflow) references HOMEBREW_TAP_TOKEN but does not mention that the workflow must include the environment variable in the GoReleaser step specifically. The task should make explicit that the HOMEBREW_TAP_TOKEN must be passed as an env var to the GoReleaser action step (not just "referenced") so the Homebrew tap push succeeds.

**Current**:
(Task portal-6-9 Do section, relevant excerpt)
```
- Set GITHUB_TOKEN env var for release creation
- Set a separate HOMEBREW_TAP_TOKEN (repo secret) for cross-repo Homebrew tap push
```

**Proposed**:
(Task portal-6-9 Do section, relevant excerpt)
```
- Set GITHUB_TOKEN env var on the GoReleaser step for release creation
- Set HOMEBREW_TAP_GITHUB_TOKEN env var on the GoReleaser step (sourced from repo secret HOMEBREW_TAP_TOKEN) for cross-repo Homebrew tap push (GoReleaser reads this env var to authenticate when pushing the formula update)
```

**Resolution**: Fixed
**Notes**: Applied to tick-c4d5e6. Added exact env var name HOMEBREW_TAP_GITHUB_TOKEN.

---

### 9. All Tasks Have Same Priority -- No Foundation Tasks Prioritised Higher

**Severity**: Minor
**Plan Reference**: All phases
**Category**: Dependencies and Ordering
**Change Type**: update-task

**Details**:
Every task across all 6 phases has priority 2. The tick format uses priority to determine execution order (lower number = higher priority). While natural ordering (by creation date) within a phase works correctly here since tasks were created in the intended execution order, the first task of each phase could benefit from a higher priority (e.g., priority 1) to signal it is a foundation task. However, since all tasks within a phase follow natural ordering and there are no cross-phase dependencies that would break execution order, this is cosmetic rather than functional. The tick `ready` command sorts by priority then creation date, so the current setup produces correct ordering.

**Current**:
N/A -- all tasks at priority 2.

**Proposed**:
N/A -- no change required. Natural ordering produces correct execution sequence. Flagged for awareness only.

**Resolution**: Acknowledged
**Notes**: Observation only. Natural ordering produces correct execution - no change needed.

---

### 10. Task portal-6-6 (tmux Runtime Dependency Check) Placed Late in Phase 6

**Severity**: Important
**Plan Reference**: Phase 6 / portal-6-6 (tick-a4b5c6)
**Category**: Dependencies and Ordering
**Change Type**: update-task

**Details**:
The tmux runtime dependency check (portal-6-6) is task 6 of 9 in Phase 6, meaning it executes after command flag parsing, command-aware session creation, command-pending TUI mode, project picker edit mode, and project picker remove with confirmation. This is a cross-cutting concern that should execute early -- ideally as the first task in Phase 6, or even in Phase 1 alongside the initial project setup. The task explicitly says it should "Remove any ad-hoc LookPath checks from other commands" and centralise the check, which means it is a refactoring task that should come before adding new features that depend on tmux. Placing it after feature tasks 6-1 through 6-5 means those tasks are implemented without the centralised check, then task 6-6 has to refactor them.

The most practical fix is to move its priority higher so it executes before the other Phase 6 tasks.

**Current**:
Task tick-a4b5c6 (tmux Runtime Dependency Check) at priority 2, positioned 6th in Phase 6 by creation order.

**Proposed**:
Change priority of tick-a4b5c6 to 1 so it executes before other Phase 6 tasks. This ensures the centralised tmux check is in place before implementing command execution and TUI mode features that depend on tmux.

**Resolution**: Fixed
**Notes**: Changed tick-a4b5c6 priority from 2 to 1. tmux check now executes first in Phase 6.

---
