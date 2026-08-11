package tui

import (
	"fmt"

	"github.com/leeovery/portal/internal/theme"
)

const (
	killTitle       = "Kill session?"
	killConsequence = "Ends the tmux session and all its panes. Can't be undone."

	killKeyConfirm   = "y"
	killLabelConfirm = "kill"
)

func renderKillModalContent(name string, windows int, th theme.Theme, colourless bool) string {
	spec := destructiveConfirmSpec{
		title:        killTitle,
		targetName:   name,
		nameTrailer:  killWindowCount(windows),
		consequence:  killConsequence,
		confirmKey:   killKeyConfirm,
		confirmLabel: killLabelConfirm,
	}
	return renderDestructiveConfirm(spec, th, colourless)
}

func killWindowCount(windows int) string {
	unit := "windows"
	if windows == 1 {
		unit = "window"
	}
	return fmt.Sprintf("· %d %s", windows, unit)
}
