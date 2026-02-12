---
topic: session-launch-command
status: in-progress
date: 2026-02-12
---

# Discussion: Session Launch with Command Execution

## Context

When launching a new tmux session via `x <project>`, the user wants the ability to also execute a command inside that session after it's created. Current `cx` hardcodes running `claude` (or `claude --continue`, `claude --resume` depending on variant). The new `portal`/`x` design should generalise this — any command can be passed through, and the whole thing should be aliasable.

Use case: `x myproject --flag claude` or similar, where the session opens at the project dir and immediately runs the specified command. Could be aliased system-wide, e.g., `alias xc='x --exec claude'`.

### References

- [mux specification](../specification/mux.md)
- [x-xctl-split discussion](x-xctl-split.md)
- [cx-design discussion](cx-design.md)

## Questions

- [ ] What CLI syntax for passing the command through?
- [ ] Should the command replace the shell or run then drop to shell?
- [ ] How does this interact with the TUI flow?
- [ ] Should projects.json support default commands per project?
- [ ] How does this relate to the existing cx variants (cx, cx c, cx r)?

---
