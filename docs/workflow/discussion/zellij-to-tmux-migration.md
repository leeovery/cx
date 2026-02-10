---
topic: zellij-to-tmux-migration
status: in-progress
date: 2026-02-10
---

# Discussion: Migrating from Zellij to tmux

## Context

ZW (Zellij Workspaces) has a concluded specification built around Zellij as the terminal multiplexer. The user has decided to switch to tmux. The core UX design, project memory, TUI architecture, CLI structure, and distribution approach are all multiplexer-agnostic and carry forward. The Zellij-specific integration layer needs reworking.

This discussion identifies what changes, what stays, and resolves the tmux-specific design decisions before revising the specification.

### References

- [ZW Specification](../specification/zw.md) - Current Zellij-based spec (concluded)
- [cx-design discussion](cx-design.md) - Original design discussion
- [zellij-multi-directory discussion](zellij-multi-directory.md) - Model pivot to workspace-centric
- [fzf-output-mode discussion](fzf-output-mode.md) - `zw list` and `zw attach`
- [git-root-and-completions discussion](git-root-and-completions.md) - Git root resolution, shell completions
- [tmux-session-managers-analysis](../research/tmux-session-managers-analysis.md) - Comparative analysis of tmux session managers

## Questions

- [x] What are the tmux equivalents for all Zellij session operations?
- [x] What happens to exited/resurrectable sessions (Zellij-native feature)?
- [ ] How should the layout system work with tmux?
- [ ] Should the tool be renamed (ZW = "Zellij Workspaces")?
- [ ] How does utility mode work with tmux?
- [ ] What session metadata can we display from outside tmux?
- [ ] How does process handoff (exec) work with tmux?
- [ ] What changes for the runtime dependency?

---

*Each question gets its own section below. Check off as concluded.*

---

## What are the tmux equivalents for all Zellij session operations?

### Context

The current spec references Zellij CLI commands throughout. Need verified tmux 3.6a equivalents.

### Decision

Verified against `man tmux` on the target system (tmux 3.6a):

| Operation | Zellij | tmux (verified) |
|---|---|---|
| Create or attach | `zellij attach -c <name>` | `tmux new-session -A -s <name>` (alias: `new`) |
| Create w/ start dir | cd + create | `tmux new-session -A -s <name> -c <dir>` |
| Attach to existing | `zellij attach <name>` | `tmux attach-session -t <name>` (alias: `attach`) |
| List sessions | `zellij list-sessions` | `tmux list-sessions` (alias: `ls`) |
| Kill session | `zellij kill-session <name>` | `tmux kill-session -t <name>` |
| Delete exited session | `zellij delete-session <name>` | N/A — tmux sessions don't persist after exit |
| Check session exists | N/A | `tmux has-session -t <name>` (alias: `has`) — exit 0/1 |
| Query tab/window names | `zellij --session <name> action query-tab-names` | `tmux list-windows -t <name>` (alias: `lsw`) |
| Rename session | `zellij action rename-session <new>` (inside only) | `tmux rename-session -t <name> <new>` (alias: `rename`) — works from outside |

**Key differences from Zellij:**
- tmux `new-session -A` combines create-or-attach in one command
- tmux supports `-c <dir>` to set the working directory at creation — no need to `cd` first
- tmux `rename-session` works from outside the session (Zellij could only rename from inside)
- tmux `has-session` provides a clean existence check (useful for argument resolution)
- tmux `list-sessions` output is structured, no ANSI codes to strip

**Directory change for new sessions**: The spec's current model (cd to dir, then create) simplifies to just passing `-c <resolved-dir>` on `new-session`. No directory change needed in ZW's process. Git root resolution still applies — resolve first, then pass to `-c`.

---

## What happens to exited/resurrectable sessions?

### Context

The Zellij spec had an "EXITED" section in the TUI showing dead-but-resurrectable sessions. Zellij natively persists session state after exit and allows individual session resurrection. tmux doesn't — sessions are alive or gone.

### Journey

Initially explored whether tmux-resurrect (installed on target system with tmux-continuum auto-saving every 10 min) could fill this gap. Research found:

- Resurrect stores snapshots at `~/.local/share/tmux/resurrect/` as tab-delimited text files
- The `last` symlink points to the most recent save
- Files contain session names, window info, working directories, running commands
- Detection is possible via directory existence or `tmux list-keys | grep resurrect`

However, resurrect's restore is **all-or-nothing** — it restores the entire saved state, not individual sessions. This is fundamentally different from Zellij's model where exited sessions are individually addressable objects.

**Key realisation**: Resurrect is disaster recovery (machine crash, tmux server dies), not a session management workflow. Users don't interact with dead sessions — they either have running sessions or they don't. The "exited sessions" concept was a Zellij-specific feature that doesn't map to tmux's model.

### Decision

**Drop the EXITED section entirely.**

- TUI shows only running sessions + new session option
- No resurrect integration — it's outside ZW's scope
- `zw clean` simplifies to only cleaning stale projects (directories that no longer exist on disk) — no "delete exited sessions" operation
- `zellij delete-session` has no tmux equivalent and is removed from the command mapping

---
