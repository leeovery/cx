package tui

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// modalState tracks which modal overlay is currently active.
type modalState int

const (
	modalNone modalState = iota
	modalKillConfirm
	modalRename
	modalDeleteProject
	modalEditProject
	modalHelp
)

// Centres an already-built panel on the cleared canvas. This layer only
// centres — the outer fillCanvas wrap in View() paints the backdrop.
func placeModalOnClearedCanvas(panel string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}

func renderHelpModalOnClearedCanvas(entries []keymapEntry, width, height int, th theme.Theme, colourless bool) string {
	panel := renderHelpModalContent(entries, th, colourless)
	return placeModalOnClearedCanvas(panel, width, height)
}

func renderKillModalOnClearedCanvas(name string, windows int, width, height int, th theme.Theme, colourless bool) string {
	panel := renderKillModalContent(name, windows, th, colourless)
	return placeModalOnClearedCanvas(panel, width, height)
}

func renderDeleteModalOnClearedCanvas(name, path string, width, height int, th theme.Theme, colourless bool) string {
	panel := renderDeleteModalContent(name, path, th, colourless)
	return placeModalOnClearedCanvas(panel, width, height)
}

func renderRenameModalOnClearedCanvas(input textinput.Model, oldName string, width, height int, th theme.Theme, colourless bool) string {
	panel := renderRenameModalContent(input, oldName, th, colourless)
	return placeModalOnClearedCanvas(panel, width, height)
}

// The panel is already fully framed, so it is placed directly — a lipgloss
// auto-border wrap would add a redundant second border.
func renderEditModalOnClearedCanvas(m Model, width, height int, th theme.Theme, colourless bool) string {
	panel := m.renderEditProjectContent()
	return placeModalOnClearedCanvas(panel, width, height)
}
