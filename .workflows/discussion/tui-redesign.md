---
topic: tui-redesign
status: in-progress
work_type: feature
date: 2026-02-27
---

# Discussion: TUI Redesign

## Context

The Portal TUI is functional but visually basic. All three views (session list, project picker, file browser) use raw `strings.Builder` concatenation with minimal lipgloss styling. The specification calls for a full-screen, bordered, centered layout built with Bubble Tea and lipgloss.

### Current State

- Three views: session list (`internal/tui/model.go`), project picker (`internal/ui/projectpicker.go`), file browser (`internal/ui/browser.go`)
- Manual `strings.Builder` concatenation with basic cursor prefix (`> ` / `  `)
- 5 lipgloss styles defined (cursor, name, detail, attached, divider colors)
- No borders, centering, padding, or terminal size adaptation
- Hardcoded divider strings instead of dynamic borders
- Kill/rename/filter prompts rendered as inline text

### Spec Expectation

Full-screen framed layout per the specification mockups — bordered, centered, padded, responsive to terminal size.

### References

- [Portal Specification — TUI Design section](.workflows/specification/portal/specification.md#tui-design)
- [tui-redesign.md](tui-redesign.md) — initial notes

## Questions

- [ ] Should all three views share a common frame/layout component, or be styled independently?
- [ ] How should prompts (kill confirm, rename, filter) be rendered — modal overlays or styled inline?
- [ ] What border style and color scheme should be used?
- [ ] Should terminal size tracking be handled at the top-level Model or within each sub-view?
- [ ] How should the "SESSIONS" / "PROJECTS" title be rendered?

---

*Each question above gets its own section below. Check off as concluded.*

---
