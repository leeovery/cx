---
checksum: 7afaa148a994fb0c691514df82fc0b1f
generated: 2026-01-22
research_files:
  - cc-tool-plan.md
---

# Research Analysis Cache

## Topics

### 1. Session Types & Zellij Integration
- **Source**: cc-tool-plan.md (lines 28-48)
- **Summary**: Defines two session types - Zellij (persistent, crash-resistant) and Raw (ephemeral). Sessions tracked via Zellij queries.
- **Key questions**: Is the Zellij-first approach correct? Should raw mode be equally supported?

### 2. Project Registry Design
- **Source**: cc-tool-plan.md (lines 32-48, 120-138)
- **Summary**: Projects stored in projects.json with path, name, alias, timestamps, usage count.
- **Key questions**: Is alias needed? Should use_count drive sorting/display?

### 3. CLI Command Structure
- **Source**: cc-tool-plan.md (lines 54-86)
- **Summary**: Unified `cx` command with subcommands (list, kill, clean, projects, config). Modifiers like --raw, -c, -r.
- **Key questions**: Is the command hierarchy right? Any missing commands?

### 4. Unified Interactive View
- **Source**: cc-tool-plan.md (lines 168-216)
- **Summary**: Single TUI showing projects + sessions. Projects with active sessions show indicators. Sub-picker for multiple sessions.
- **Key questions**: Is the sub-picker UX right? What about the [.] and [n] shortcuts?

### 5. Session → Project Mapping
- **Source**: cc-tool-plan.md (lines 140-160)
- **Summary**: Three options proposed: encode in name, sessions.json mapping, query Zellij CWD. Recommends sessions.json.
- **Key questions**: Confirm sessions.json approach? How to handle orphaned mappings?

### 6. Session Naming Convention
- **Source**: cc-tool-plan.md (lines 248-260)
- **Summary**: Format `{project}-{NN}`, numbers increment even after sessions deleted to avoid reuse confusion.
- **Key questions**: Is this format right? What about very long project names?

### 7. Configuration Design
- **Source**: cc-tool-plan.md (lines 90-116)
- **Summary**: YAML config at ~/.config/cx/config.yaml with defaults, session naming, UI prefs, Claude args.
- **Key questions**: Are these the right config options? Any missing?

### 8. Open Question: Zellij Layout Location
- **Source**: cc-tool-plan.md (lines 435-441)
- **Summary**: Where should custom Claude layout live? Options: bundled, user-maintained, setup command.
- **Key questions**: Confirm user-maintains-own approach?

### 9. Open Question: Auto-registration
- **Source**: cc-tool-plan.md (lines 452-458)
- **Summary**: Should projects auto-register when sessions start?
- **Key questions**: Confirm yes with manual forget?

### 10. Open Question: Missing Zellij
- **Source**: cc-tool-plan.md (lines 459-466)
- **Summary**: What if Zellij not installed? Options: error, warn+fallback, configurable.
- **Key questions**: Confirm warn+fallback approach?

### 11. Open Question: Continue/Resume Flags
- **Source**: cc-tool-plan.md (lines 468-475)
- **Summary**: Should -c and -r work with existing sessions or only new Claude instances?
- **Key questions**: Confirm new-process-only behavior?

### 12. Technical Architecture
- **Source**: cc-tool-plan.md (lines 272-356)
- **Summary**: Go project structure with cmd/, internal/ layout. Cobra CLI, Bubbletea TUI, Lipgloss styling.
- **Key questions**: Is dependency list complete? Any concerns with architecture?

### 13. Distribution Strategy
- **Source**: cc-tool-plan.md (lines 387-428)
- **Summary**: Homebrew tap at leeovery/tools, formula with Zellij dependency, manual release process.
- **Key questions**: Should Zellij be a hard homebrew dependency? GoReleaser for automation?

### 14. Implementation Phasing
- **Source**: cc-tool-plan.md (lines 477-504)
- **Summary**: Four phases - Core MVP, Session Management, Polish, Distribution.
- **Key questions**: Is the phasing right? Anything moved between phases?
