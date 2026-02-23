---
status: complete
created: 2026-02-23
cycle: 2
phase: Plan Integrity Review
topic: portal
---

# Review Tracking: Portal - Integrity

## Findings

### 1. Task portal-2-5 (Session Creation from Project) Missing Outcome Field

**Severity**: Important
**Plan Reference**: Phase 2 / portal-2-5 (tick-f924e3)
**Category**: Task Template Compliance
**Change Type**: update-task

**Details**:
Task portal-2-5 is missing the required **Outcome** field. Cycle 1 fixed the missing Outcome fields on tasks 2-2, 2-3, and 2-4 but this task was missed. Without the Outcome field, the implementer has no clear statement of what success looks like for this orchestration task.

**Current**:
```
**Problem**: Portal needs to create a new tmux session from a project directory, orchestrating git root resolution, name generation, project store update, and tmux command.

**Solution**: Implement SessionCreator.CreateFromDir that coordinates the full flow: resolve git root, derive project name, generate session name, upsert project, create tmux session.

**Do**:
- Create internal/session/create.go
- SessionCreator struct with dependencies: GitResolver, ProjectStore, TmuxClient
- CreateFromDir(dir): resolve git root, derive name from basename, generate session name, upsert project, call tmux NewSession
- Extend TmuxClient: HasSession(name) bool, NewSession(name, dir) error
- Validate directory exists before tmux call

**Acceptance Criteria**:
- [ ] Resolves directory to git root
- [ ] Generates session name from basename with nanoid
- [ ] Upserts project in store
- [ ] Creates tmux session with correct name and dir
- [ ] Works when no tmux server running
- [ ] Returns error when directory missing

**Tests**:
- creates session with git-root-resolved directory
- derives project name from basename
- generates unique session name
- upserts project in store
- handles tmux server not running
- returns error for non-existent directory

**Spec Reference**: .workflows/specification/portal/specification.md -- New Session Flow, How Directories are Added
```

**Proposed**:
```
**Problem**: Portal needs to create a new tmux session from a project directory, orchestrating git root resolution, name generation, project store update, and tmux command.

**Solution**: Implement SessionCreator.CreateFromDir that coordinates the full flow: resolve git root, derive project name, generate session name, upsert project, create tmux session.

**Outcome**: Tested SessionCreator that orchestrates git root resolution, project name derivation, session name generation, project store upsert, and tmux session creation from a single directory input.

**Do**:
- Create internal/session/create.go
- SessionCreator struct with dependencies: GitResolver, ProjectStore, TmuxClient
- CreateFromDir(dir): resolve git root, derive name from basename, generate session name, upsert project, call tmux NewSession
- Extend TmuxClient: HasSession(name) bool, NewSession(name, dir) error
- Validate directory exists before tmux call

**Acceptance Criteria**:
- [ ] Resolves directory to git root
- [ ] Generates session name from basename with nanoid
- [ ] Upserts project in store
- [ ] Creates tmux session with correct name and dir
- [ ] Works when no tmux server running
- [ ] Returns error when directory missing

**Tests**:
- creates session with git-root-resolved directory
- derives project name from basename
- generates unique session name
- upserts project in store
- handles tmux server not running
- returns error for non-existent directory

**Spec Reference**: .workflows/specification/portal/specification.md -- New Session Flow, How Directories are Added
```

**Resolution**: Fixed
**Notes**: Applied to tick-f924e3

---

### 2. Task portal-2-6 (Project Picker TUI View) Missing Outcome Field

**Severity**: Important
**Plan Reference**: Phase 2 / portal-2-6 (tick-4c54e1)
**Category**: Task Template Compliance
**Change Type**: update-task

**Details**:
Task portal-2-6 is missing the required **Outcome** field. This was missed in cycle 1 alongside the other Phase 2 tasks.

**Current**:
```
**Problem**: Portal needs a project picker TUI showing remembered projects sorted by recency with browse option at bottom and empty state handling.

**Solution**: Implement ProjectPickerModel as Bubble Tea component. Display projects from store, always show browse option, handle empty state. Support filter mode via / key.

**Do**:
- Create internal/ui/projectpicker.go with ProjectPickerModel
- Init: call store.CleanStale(), store.List()
- Render: header, project list with cursor, separator, browse option
- Navigation: Up/Down/j/k, Enter selects, Esc returns
- Empty state: "No saved projects yet." with browse still visible
- Return selection message (project path or browse action)
- / key activates filter mode with fuzzy matching against project names
- Filter mode: browse option always visible, Backspace/Esc behavior

**Acceptance Criteria**:
- [ ] Displays projects sorted by last_used
- [ ] browse option always visible at bottom
- [ ] Arrow keys and j/k navigate list
- [ ] Enter on project returns selection
- [ ] Enter on browse returns browse action
- [ ] Esc returns to session list
- [ ] Empty state with browse still selectable
- [ ] / activates filter mode; typing fuzzy-matches against project names
- [ ] Filter mode: [n]/browse option always visible regardless of filter
- [ ] Filter mode: Backspace removes last char; on empty filter exits filter mode
- [ ] Filter mode: Esc clears filter and exits filter mode

**Tests**:
- displays projects sorted by last_used
- shows browse option at bottom
- empty state shows message
- enter on project emits path
- esc emits back message
- slash activates filter mode in project picker
- typing narrows project list by fuzzy match
- browse option always visible during filter
- backspace on empty filter exits filter mode
- esc clears filter and exits filter mode

**Spec Reference**: .workflows/specification/portal/specification.md -- Project Picker Interaction, Empty States
```

**Proposed**:
```
**Problem**: Portal needs a project picker TUI showing remembered projects sorted by recency with browse option at bottom and empty state handling.

**Solution**: Implement ProjectPickerModel as Bubble Tea component. Display projects from store, always show browse option, handle empty state. Support filter mode via / key.

**Outcome**: Project picker TUI component that displays remembered projects sorted by recency, supports navigation, selection, browse option, empty state, and filter mode with fuzzy matching.

**Do**:
- Create internal/ui/projectpicker.go with ProjectPickerModel
- Init: call store.CleanStale(), store.List()
- Render: header, project list with cursor, separator, browse option
- Navigation: Up/Down/j/k, Enter selects, Esc returns
- Empty state: "No saved projects yet." with browse still visible
- Return selection message (project path or browse action)
- / key activates filter mode with fuzzy matching against project names
- Filter mode: browse option always visible, Backspace/Esc behavior

**Acceptance Criteria**:
- [ ] Displays projects sorted by last_used
- [ ] browse option always visible at bottom
- [ ] Arrow keys and j/k navigate list
- [ ] Enter on project returns selection
- [ ] Enter on browse returns browse action
- [ ] Esc returns to session list
- [ ] Empty state with browse still selectable
- [ ] / activates filter mode; typing fuzzy-matches against project names
- [ ] Filter mode: [n]/browse option always visible regardless of filter
- [ ] Filter mode: Backspace removes last char; on empty filter exits filter mode
- [ ] Filter mode: Esc clears filter and exits filter mode

**Tests**:
- displays projects sorted by last_used
- shows browse option at bottom
- empty state shows message
- enter on project emits path
- esc emits back message
- slash activates filter mode in project picker
- typing narrows project list by fuzzy match
- browse option always visible during filter
- backspace on empty filter exits filter mode
- esc clears filter and exits filter mode

**Spec Reference**: .workflows/specification/portal/specification.md -- Project Picker Interaction, Empty States
```

**Resolution**: Fixed
**Notes**: Applied to tick-4c54e1

---

### 3. Task portal-2-7 (Main TUI New in Project Integration) Missing Outcome Field

**Severity**: Important
**Plan Reference**: Phase 2 / portal-2-7 (tick-444a76)
**Category**: Task Template Compliance
**Change Type**: update-task

**Details**:
Task portal-2-7 is missing the required **Outcome** field. This completes the set of Phase 2 tasks that were missing Outcome -- cycle 1 caught 2-2, 2-3, 2-4 but missed 2-5, 2-6, and 2-7.

**Current**:
```
**Problem**: Main session list needs [n] new in project... option that transitions to project picker. After selection, create session and attach.

**Solution**: Extend Phase 1 session list with new option below divider, wire n shortcut, implement view switching between session list and project picker.

**Do**:
- Add [n] new in project... option below sessions with divider
- n shortcut jumps to the option
- View states: viewSessionList, viewProjectPicker
- Enter on option switches to project picker
- Esc in picker returns to session list
- Project selection triggers SessionCreator.CreateFromDir then tmux exec
- Browse action is placeholder for Phase 3

**Acceptance Criteria**:
- [ ] [n] new in project... appears below sessions with divider
- [ ] n key jumps to new option
- [ ] Enter on option transitions to project picker
- [ ] Selecting project creates session and attaches
- [ ] Esc in picker returns to session list
- [ ] Empty session list still shows option
- [ ] Combined empty state (no sessions + no projects) navigable

**Tests**:
- session list includes new in project option
- n key jumps to option
- enter switches to project picker
- esc returns to session list
- project selection triggers creation
- empty session list shows option

**Spec Reference**: .workflows/specification/portal/specification.md -- TUI Design, Keyboard Shortcuts
```

**Proposed**:
```
**Problem**: Main session list needs [n] new in project... option that transitions to project picker. After selection, create session and attach.

**Solution**: Extend Phase 1 session list with new option below divider, wire n shortcut, implement view switching between session list and project picker.

**Outcome**: Session list TUI includes [n] new in project... option that transitions to the project picker. Selecting a project creates a session and attaches. Esc returns to the session list.

**Do**:
- Add [n] new in project... option below sessions with divider
- n shortcut jumps to the option
- View states: viewSessionList, viewProjectPicker
- Enter on option switches to project picker
- Esc in picker returns to session list
- Project selection triggers SessionCreator.CreateFromDir then tmux exec
- Browse action is placeholder for Phase 3

**Acceptance Criteria**:
- [ ] [n] new in project... appears below sessions with divider
- [ ] n key jumps to new option
- [ ] Enter on option transitions to project picker
- [ ] Selecting project creates session and attaches
- [ ] Esc in picker returns to session list
- [ ] Empty session list still shows option
- [ ] Combined empty state (no sessions + no projects) navigable

**Tests**:
- session list includes new in project option
- n key jumps to option
- enter switches to project picker
- esc returns to session list
- project selection triggers creation
- empty session list shows option

**Spec Reference**: .workflows/specification/portal/specification.md -- TUI Design, Keyboard Shortcuts
```

**Resolution**: Fixed
**Notes**: Applied to tick-444a76

---
