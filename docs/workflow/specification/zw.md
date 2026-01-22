# Specification: ZW (Zellij Workspaces)

**Status**: Building specification
**Type**: feature
**Last Updated**: 2026-01-22

---

## Overview

### What is ZW

ZW (Zellij Workspaces) is a Go CLI that provides an interactive session picker for Zellij. It runs at bare shell (before entering Zellij) and offers a mobile-friendly TUI for managing Zellij sessions.

### The Problem

When SSH/Mosh-ing to a machine (e.g., from phone to Mac), it's tedious to:
- Remember which Zellij sessions exist
- Type session names correctly to attach
- Navigate to the right directory to start new sessions

Zellij's built-in session manager only works *inside* an existing session and is too information-dense for mobile screens.

### The Solution

A single command (`zw`) that:
1. Shows existing sessions (running and exited/resurrectable)
2. Allows quick attachment with arrow keys + Enter
3. Remembers project directories for starting new sessions
4. Works at bare shell, optimized for small screens

### Value Proposition

1. **Interactive picker at bare shell** - What Zellij's session-manager does, but *before* you're inside Zellij
2. **Mobile-friendly** - Clean, minimal interface vs. Zellij's dense built-in manager
3. **Project memory** - Quick-start new sessions in remembered directories
4. **One command** - `zw` does everything vs. `zellij ls` + `zellij attach <name>`

## Core Model

### Sessions as Workspaces

ZW treats Zellij sessions as **workspaces**. A workspace may span multiple directories - Zellij allows multiple panes in a session, each potentially in different directories.

### Sessions and Projects are Separate Concerns

- **Sessions** = Live data queried from Zellij (`zellij ls`)
- **Projects** = ZW's memory of directories used to start new sessions

ZW does not track which project a session belongs to. Select a session → attach. Select a project → start a new session there.

### No Directory Change Before Attach

When attaching to an existing session, ZW does not change directories. Zellij restores shell state on reattach - each pane resumes exactly where it was.
