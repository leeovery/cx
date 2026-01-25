---
topic: zoxide-integration
status: in-progress
date: 2026-01-25
---

# Discussion: Zoxide Integration

## Context

ZW currently maintains its own project memory in `~/.config/zw/projects.json`. Users must explicitly add directories via the file browser or by starting sessions there.

Zoxide is a popular "smarter cd" tool that tracks frequently-visited directories. Many developers already have Zoxide installed with a curated history of project directories.

### References

- [ZW Specification](../specification/zw.md) - Current project memory design (lines 189-230)
- [Zesh](https://github.com/roberte777/zesh) - Uses Zoxide as its directory source

### The Proposal

Allow ZW to optionally pull directories from Zoxide as an additional source of projects, reducing friction for Zoxide users.

```bash
# Zoxide query command
zoxide query -l    # Lists all tracked directories
```

### Current Design

- Projects stored in `~/.config/zw/projects.json`
- Added via file browser or starting sessions
- Supports aliases, custom names, last_used timestamp

## Questions

- [ ] Should Zoxide be an alternative or additional source?
- [ ] How do we handle duplicates between sources?
- [ ] What happens if Zoxide isn't installed?
- [ ] How does this affect the project picker UX?
- [ ] Should this be configurable?

---

