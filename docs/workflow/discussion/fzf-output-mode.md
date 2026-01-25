---
topic: fzf-output-mode
status: in-progress
date: 2026-01-25
---

# Discussion: fzf-Compatible Output Mode

## Context

ZW is TUI-first by design. However, power users may want to pipe session/project data to fzf or use it in scripts. This discussion explores adding a non-interactive output mode.

### References

- [ZW Specification](../specification/zw.md) - Current CLI commands (lines 279-297)
- [Zesh README](https://github.com/roberte777/zesh) - Inspiration: `zesh l | fzf` pattern

### Current CLI

From the spec:
- `zw` — Launch TUI
- `zw .` — New session in current directory
- `zw <path>` — New session in specified directory
- `zw <alias>` — New session for project with alias
- `zw clean` — Remove exited sessions
- `zw version` / `zw help`

No plain-text listing for scripting.

### Proposal

Add output that can be piped to fzf or used in scripts:

```bash
zw --list                      # Output session names
zw attach $(zw --list | fzf)   # Pipe to fzf
```

## Questions

- [ ] What should the output format be?
- [ ] Should this be a flag (`--list`) or subcommand (`zw list`)?
- [ ] What data should be included?
- [ ] Do we need an `attach` subcommand to complement this?

---

