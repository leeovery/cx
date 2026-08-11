package tui

import (
	tea "charm.land/bubbletea/v2"
)

func keyIsCode(msg tea.KeyPressMsg, code rune) bool {
	return msg.Code == code && msg.Mod == 0
}

func isRuneKey(msg tea.KeyPressMsg, ch string) bool {
	return msg.Mod == 0 && msg.Text == ch
}

func keyIsCtrlC(msg tea.KeyPressMsg) bool {
	return msg.Code == 'c' && msg.Mod.Contains(tea.ModCtrl)
}
