---
topic: x-xctl-split
status: in-progress
date: 2026-02-11
---

# Discussion: Rename & Binary Split (mux → x / xctl)

## Context

Current tool is specced as `mux` — single binary with subcommands (`mux`, `mux .`, `mux <path>`, `mux list`, `mux attach`, `mux clean`, etc.). Not yet implemented.

Proposal from external AI conversation: split into two binaries:
- `x` — interactive TUI launcher (muscle-memory, daily driver)
- `xctl` — non-interactive control plane (scripting, maintenance, explicit verbs)

Motivation: clean separation between interactive UX and scripting surface. Avoids verb/path ambiguity in the launcher. Keeps the daily command ultra-simple.

### References

- [Current mux spec](../specification/mux.md)
- [tmux-session-managers-analysis](../research/tmux-session-managers-analysis.md)
- [zellij-to-tmux-migration discussion](zellij-to-tmux-migration.md)

## Questions

- [ ] Should we split into two binaries or keep single binary?
- [ ] What should the binaries be named?
- [ ] What behaviour belongs in `x` (interactive)?
- [ ] What behaviour belongs in `xctl` (control plane)?
- [ ] Should `x` accept `<query-or-path>` args or stay pure TUI + `.`?
- [ ] Output conventions for xctl (machine-friendly, --json, exit codes)
- [ ] How does this affect the existing mux spec?

---

*Each question above gets its own section below. Check off as concluded.*
