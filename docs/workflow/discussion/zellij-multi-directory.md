---
topic: zellij-multi-directory
status: in-progress
date: 2026-01-22
---

# Discussion: Zellij Multi-Directory Sessions

## Context

CX's current design assumes a 1:1 relationship between Zellij sessions and project directories. However, Zellij sessions can have **multiple panes spread across multiple directories/projects**.

This challenges the core model and could dramatically change the tool's architecture.

### References

- [CX Design Discussion](cx-design.md) - Concluded design assuming simpler model
- [CX Tool Plan](../research/cc-tool-plan.md) - Initial research

### Current Model Assumptions (from cx-design.md)

1. **sessions.json** maps each session to exactly one project path
2. **cd before attach** - CX changes to project dir before attaching
3. **Project as anchor** - TUI organizes around projects, sessions as children
4. **Session naming** - `{project-name}-{NN}` implies project ownership

## Questions

- [ ] How do users actually use Zellij multi-pane sessions?
- [ ] Does the session → project mapping need to change?
- [ ] What should "cd before attach" mean for multi-directory sessions?
- [ ] How does this affect the TUI model?
- [ ] Is the naming convention still valid?

---

*Each question gets its own section below. Check off as concluded.*

---

## How do users actually use Zellij multi-pane sessions?

### The Problem

Zellij allows multiple panes per session, each potentially in a different directory. Need to understand how this is actually used in practice to determine if CX's model needs fundamental changes.

### Journey

*Exploring with user...*

