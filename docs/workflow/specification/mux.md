---
topic: mux
status: in-progress
type: feature
date: 2026-02-10
sources:
  - name: cx-design
    status: pending
  - name: zellij-multi-directory
    status: pending
  - name: fzf-output-mode
    status: pending
  - name: git-root-and-completions
    status: pending
  - name: zellij-to-tmux-migration
    status: pending
---

# Specification: mux

## Overview

### What is mux

mux is a Go CLI that provides an interactive session picker for tmux. It runs at bare shell (before entering tmux) and offers a mobile-friendly TUI for managing tmux sessions.

### The Problem

When SSH/Mosh-ing to a machine (e.g., from phone to Mac), it's tedious to:
- Remember which tmux sessions exist
- Type session names correctly to attach
- Navigate to the right directory to start new sessions

tmux's built-in session management is command-line only (`tmux ls`, `tmux attach -t <name>`) with no interactive picker.

### The Solution

A single command (`mux`) that:
1. Shows existing running sessions
2. Allows quick attachment with arrow keys + Enter
3. Remembers project directories for starting new sessions
4. Works at bare shell, optimized for small screens

### Value Proposition

1. **Interactive picker at bare shell** - An interactive session picker that works outside tmux
2. **Mobile-friendly** - Clean, minimal interface optimized for small screens
3. **Project memory** - Quick-start new sessions in remembered directories
4. **One command** - `mux` does everything vs. `tmux ls` + `tmux attach -t <name>`

## Core Model

### Sessions as Workspaces

mux treats tmux sessions as **workspaces**. A workspace may span multiple directories — tmux allows multiple windows and panes in a session, each potentially in different directories.

### Sessions and Projects are Separate Concerns

- **Sessions** = Live data queried from tmux (`tmux list-sessions`)
- **Projects** = mux's memory of directories used to start new sessions

mux does not track which project a session belongs to. Select a session → attach. Select a project → start a new session there.

### No Directory Change Before Attach

When attaching to an existing session, mux does not change directories. tmux restores shell state on reattach — each pane resumes exactly where it was.

### Directory Change for New Sessions

When starting a **new** session, mux passes the resolved directory to tmux via the `-c` flag. The sequence is:

1. Resolve directory to git root (if inside a git repository)
2. Run `tmux new-session -A -s <session-name> -c <resolved-dir>`

tmux's `-c` flag sets the working directory at session creation — no `cd` needed in mux's process.

### Git Root Resolution

When a directory is selected for a new session (via `mux .`, `mux <path>`, or the file browser), mux resolves it to the git repository root before proceeding.

**Implementation**: Run `git -C <selected-dir> rev-parse --show-toplevel`. If it succeeds, use the output as the directory. If it fails (exit code 128 — not a git repo), use the original directory as-is.

**Behavior**:
- Resolution is automatic and silent — no prompt or confirmation
- Non-git directories are used unchanged, with no warning
- One resolution function applied uniformly at all three entry points (`mux .`, `mux <path>`, file browser)

## TUI Design

### Technology

Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea) (terminal UI framework).

### Layout

Full-screen picker optimized for small screens (mobile SSH use case).

```
┌─────────────────────────────────────┐
│                                     │
│           SESSIONS                  │
│                                     │
│    >  cx-03          ● attached     │
│       api-work       2 windows      │
│       client-proj                   │
│                                     │
│    ─────────────────────────────    │
│    [n] new in project...            │
│                                     │
└─────────────────────────────────────┘
```

### Sections

1. **SESSIONS** - Running tmux sessions
   - Shows session name
   - Shows attached indicator (`● attached`) when `session_attached > 0`
   - Shows window count (e.g., `2 windows`)

2. **New session option** - `[n] new in project...`
   - Opens project picker to start a new session

### Sorting

**Sessions**: Displayed in the order returned by `tmux list-sessions`. No additional sorting applied.

**Projects**: Sorted by `last_used` timestamp, most recent first.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Navigate list |
| `Enter` | Select (attach to session or open project picker) |
| `n` | Jump to "new session" option |
| `K` | Kill selected session |
| `/` | Enter filter mode |
| `q` / `Esc` | Quit |

**Kill confirmation**: Pressing `K` prompts for confirmation before killing the selected session: "Kill session 'myapp'? (y/n)"

### Filter Mode

The session list supports fuzzy filtering via a dedicated mode, activated by pressing `/`.

**Entering filter mode**: Press `/`. A filter input appears at the bottom of the list (e.g., `filter: _`). All subsequent keystrokes are treated as filter input — shortcut keys (`n`, `k`, `K`, etc.) lose their shortcut meaning and become typeable characters.

**While filtering**:
- Typing narrows the visible list by fuzzy-matching against session names
- `↑` / `↓` navigate the filtered results
- `Enter` selects the highlighted item (same as normal mode)
- `Backspace` deletes the last filter character; if the filter is already empty, exits filter mode
- `Esc` clears the filter and exits filter mode

**What gets filtered**: Running sessions only. The `[n] new in project...` option is always visible and never filtered out.

**Outside filter mode**: All single-key shortcuts (`n`, `k`, `j`, `K`, `q`) work as documented in the keyboard shortcuts table. No filtering occurs.

### Session Info Display

mux queries session metadata via tmux format strings:

```bash
tmux list-sessions -F '#{session_name}|#{session_windows}|#{session_attached}'
```

This provides session name, window count, and attached client count in a clean, structured format — no ANSI escape stripping needed.

For per-window detail (if needed):
```bash
tmux list-windows -t <name> -F '#{window_index}|#{window_name}|#{window_panes}'
```

### Empty States

**No sessions:**
```
┌─────────────────────────────────────┐
│                                     │
│           SESSIONS                  │
│                                     │
│       No active sessions            │
│                                     │
│    ─────────────────────────────    │
│    [n] new in project...            │
│                                     │
└─────────────────────────────────────┘
```

**No remembered projects (when opening project picker):**
```
Select a project:

  No saved projects yet.

  ─────────────────────────────
  browse for directory...
```

The file browser is always available to start sessions in new directories.

## Session Naming

### Auto-Generated from Project Name

Session names are auto-generated using the project name plus a short random suffix:

```
{project-name}-{nanoid}
```

**Example**: Project "mux" produces sessions like `mux-x7k2m9`, `mux-a3b8p1`.

The suffix is a 6-character nanoid, ensuring uniqueness without user input. Users are never prompted for a session name.

### Renaming

Session renaming is available both inside and outside tmux:

```bash
tmux rename-session -t <current-name> <new-name>
```

tmux's `rename-session` works from outside the session (unlike Zellij, which required being inside). This means renaming is available in all contexts — the main TUI and when running inside tmux.

### New Session Flow

**New project (directory not in projects.json):**
```
Selected: ~/Code/myapp

Project name: [myapp] _
  (Enter to accept, or type a custom name)

Aliases (optional): _
  (Comma-separated, e.g. "app, ma". Enter to skip)
```

The session is created immediately after naming — no layout selection step.

**Saved project:**

Session creation is immediate upon project selection — no prompts. The session name is auto-generated from the project name.

## Running Inside tmux

### Detection

mux detects if it's running inside an existing tmux session via the `TMUX` environment variable (set by tmux when inside a session).

### Behavior Inside tmux

When running inside tmux, the TUI is the same as outside — no restricted mode. The difference is the underlying operation:

- **Outside tmux**: `Enter` on a session → `exec tmux attach-session -t <name>`
- **Inside tmux**: `Enter` on a session → `tmux switch-client -t <name>`

tmux's `switch-client` switches the current client to a different session without nesting. This means all TUI operations work from inside tmux.

### Inside tmux: Session Actions

| Action | Command |
|--------|---------|
| Select existing session | `tmux switch-client -t <name>` |
| New session from project | `tmux new-session -d -s <name> -c <dir>` then `tmux switch-client -t <name>` |
| Kill session | `tmux kill-session -t <name>` (same as outside) |
| Rename session | `tmux rename-session -t <name> <new-name>` (same as outside) |

### TUI Differences When Inside tmux

- Current session is excluded from the session list (you're already in it)
- Header shows current session name for context (e.g., from `TMUX` env var parsing or `tmux display-message -p '#{session_name}'`)

### CLI Commands Inside tmux

CLI commands also use switch-client instead of attach:

- `mux .`, `mux <path>`, `mux <alias>` → create session detached, then switch-client
- `mux attach <name>` → switch-client to the named session
