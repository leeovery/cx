---
checksum: 0a01251b5552b61085c51b43b502c6ca
generated: 2026-01-25T16:47:00Z
discussion_files:
  - cx-design.md
  - fzf-output-mode.md
  - zellij-multi-directory.md
---

# Discussion Consolidation Analysis

## Recommended Groupings

### zw
- **cx-design**: Original CX design - TUI, session management, data storage, Zellij integration, configuration, CLI, distribution. Foundation for the tool.
- **zellij-multi-directory**: Pivots from project-centric to workspace-centric model. Renames CX → ZW. Supersedes key assumptions from cx-design but retains TUI, distribution, file browser design.
- **fzf-output-mode**: Adds `zw list` and `zw attach <name>` commands for fzf/scripting integration. Feature addition to existing ZW spec.

**Coupling**: All three discussions are about the same tool (ZW, formerly CX). cx-design and zellij-multi-directory are tightly coupled - the latter explicitly pivots and supersedes parts of the former. fzf-output-mode is a feature addition that references the ZW specification directly and proposes CLI command additions.

## Independent Discussions

(none)

## Analysis Notes

An existing "zw" specification (concluded) already exists. These three discussions represent the source material and subsequent refinements that should be incorporated:

1. **cx-design** - foundational design decisions
2. **zellij-multi-directory** - model pivot (session = workspace, not project)
3. **fzf-output-mode** - CLI additions for scripting

The zellij-multi-directory discussion explicitly states it supersedes parts of cx-design, so the specification should reflect the evolved workspace-centric model while preserving unchanged decisions (distribution, file browser, etc.).
