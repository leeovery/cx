---
checksum: 9a066926c69722116624c2476e469db9
generated: 2026-02-10T14:50:00Z
discussion_files:
  - cx-design.md
  - fzf-output-mode.md
  - git-root-and-completions.md
  - zellij-multi-directory.md
  - zellij-to-tmux-migration.md
---

# Discussion Consolidation Analysis

## Recommended Groupings

### mux (formerly zw)
- **cx-design**: Original tool design — TUI, CLI structure, session management, project naming, distribution
- **zellij-multi-directory**: Model pivot from project-centric to workspace-centric, renamed CX → ZW
- **fzf-output-mode**: Added `list` and `attach` subcommands for scripting/fzf integration
- **git-root-and-completions**: Git root auto-resolution for new sessions, shell completion subcommand
- **zellij-to-tmux-migration**: Migrates from Zellij to tmux, renames ZW → mux, drops layouts and exited sessions, replaces utility mode with switch-client

**Coupling**: All 5 discussions define the same tool at different stages of its evolution. The tmux migration discussion explicitly references and modifies decisions from all other 4 discussions. Inseparable.

## Independent Discussions

None.

## Analysis Notes

**Naming conflict**: The anchored specification name is `zw`, but the zellij-to-tmux-migration discussion renames the tool from ZW to `mux`. The specification name should change to `mux` to match the new tool identity. The existing `zw.md` spec will need to be superseded by `mux.md`.

User confirmed: tmux migration discussion belongs in the existing grouped spec.
