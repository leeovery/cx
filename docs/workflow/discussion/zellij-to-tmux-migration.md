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

- [ ] What are the tmux equivalents for all Zellij session operations?
- [ ] What happens to exited/resurrectable sessions (Zellij-native feature)?
- [ ] How should the layout system work with tmux?
- [ ] Should the tool be renamed (ZW = "Zellij Workspaces")?
- [ ] How does utility mode work with tmux?
- [ ] What session metadata can we display from outside tmux?
- [ ] How does process handoff (exec) work with tmux?
- [ ] What changes for the runtime dependency?

---

*Each question gets its own section below. Check off as concluded.*

---
