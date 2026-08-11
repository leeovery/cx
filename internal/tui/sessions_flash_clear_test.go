package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

func TestSessionsFlashClear_FirstKeystrokeClearsFlash_AndLandsInFilterInput(t *testing.T) {
	m := flashModelWithSessions("alpha", "beta")
	m.setFlash("attach failed — session gone")
	if m.flashText == "" {
		t.Fatalf("setup invariant: flashText empty before keystroke")
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	mm, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}

	if mm.flashText != "" {
		t.Fatalf("flashText after actionable keystroke: want %q, got %q", "", mm.flashText)
	}
	if !mm.sessionList.SettingFilter() {
		t.Fatalf("sessionList.SettingFilter() after '/' keystroke: want true, got false (keystroke was swallowed)")
	}
}

func TestSessionsFlashClear_WindowSizeMsgDoesNotClearFlash(t *testing.T) {
	m := flashModelWithSessions("alpha")
	m.setFlash("flash text")

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	mm, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	if mm.flashText != "flash text" {
		t.Fatalf("flashText after WindowSizeMsg: want %q (unchanged), got %q", "flash text", mm.flashText)
	}
}

func TestSessionsFlashClear_FocusMsgDoesNotClearFlash(t *testing.T) {
	m := flashModelWithSessions("alpha")
	m.setFlash("flash text")

	updated, _ := m.Update(tea.FocusMsg{})
	mm, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	if mm.flashText != "flash text" {
		t.Fatalf("flashText after FocusMsg: want %q (unchanged), got %q", "flash text", mm.flashText)
	}
}

func TestSessionsFlashClear_BlurMsgDoesNotClearFlash(t *testing.T) {
	m := flashModelWithSessions("alpha")
	m.setFlash("flash text")

	updated, _ := m.Update(tea.BlurMsg{})
	mm, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	if mm.flashText != "flash text" {
		t.Fatalf("flashText after BlurMsg: want %q (unchanged), got %q", "flash text", mm.flashText)
	}
}

func TestSessionsFlashClear_MouseMsgDoesNotClearFlash(t *testing.T) {
	m := flashModelWithSessions("alpha")
	m.setFlash("flash text")

	updated, _ := m.Update(tea.MouseClickMsg{})
	mm, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	if mm.flashText != "flash text" {
		t.Fatalf("flashText after MouseMsg: want %q (unchanged), got %q", "flash text", mm.flashText)
	}
}

func TestSessionsFlashClear_KeystrokeWithNoFlashIsNormalNoOverhead(t *testing.T) {
	m := flashModelWithSessions("alpha", "beta")
	if m.flashText != "" || m.flashGen != 0 {
		t.Fatalf("setup invariant: want empty flash with gen=0, got text=%q gen=%d", m.flashText, m.flashGen)
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	mm, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	if mm.flashText != "" {
		t.Fatalf("flashText after keystroke with no flash: want %q, got %q", "", mm.flashText)
	}
	if mm.flashGen != 0 {
		t.Fatalf("flashGen after keystroke with no flash: want 0 (no side effect), got %d", mm.flashGen)
	}
	if !mm.sessionList.SettingFilter() {
		t.Fatalf("sessionList.SettingFilter() after '/' keystroke: want true, got false")
	}
}

func TestSessionsFlashClear_SuccessiveKeystrokesAllLandNormally(t *testing.T) {
	m := flashModelWithSessions("alpha", "beta")
	m.setFlash("bail")

	updated, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = updated.(Model)
	if m.flashText != "" {
		t.Fatalf("flashText after first key: want empty, got %q", m.flashText)
	}
	if !m.sessionList.SettingFilter() {
		t.Fatalf("first key '/' did not open filter input")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(Model)
	if m.flashText != "" {
		t.Fatalf("flashText after second key: want empty, got %q", m.flashText)
	}
	if !m.sessionList.SettingFilter() {
		t.Fatalf("sessionList exited filter mode after second key; filter swallowed")
	}
	if got := m.sessionList.FilterInput.Value(); got != "a" {
		t.Fatalf("filter input value after typing 'a': want %q, got %q", "a", got)
	}
}

func TestSessionsFlashClear_FlashClearingKeystrokeAlsoReachesListBindings(t *testing.T) {
	m := flashModelWithSessions("alpha", "beta", "gamma")
	m.setFlash("bail")
	cursorBefore := m.sessionList.Index()

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	mm := updated.(Model)
	if mm.flashText != "" {
		t.Fatalf("flashText after KeyDown: want empty, got %q", mm.flashText)
	}
	if mm.sessionList.Index() == cursorBefore {
		t.Fatalf("sessionList.Index() did not advance on KeyDown: cursor stuck at %d", cursorBefore)
	}
}

func TestSessionsFlashClear_EscWithActiveFlashClearsFlashAndQuits(t *testing.T) {
	m := flashModelWithSessions("alpha")
	m.setFlash("bail")

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	mm := updated.(Model)
	if mm.flashText != "" {
		t.Fatalf("flashText after Esc: want empty, got %q", mm.flashText)
	}
	if cmd == nil {
		t.Fatalf("Esc on Sessions page with no filter applied must return tea.Quit cmd; got nil")
	}
}

func TestSessionsFlashClear_EnterWithActiveFlashClearsFlashAndRunsEnterHandler(t *testing.T) {
	m := flashModelWithSessions("alpha")
	m.setFlash("bail")
	if m.sessionList.FilterState() == list.FilterApplied {
		t.Fatalf("setup invariant: filter unexpectedly applied")
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	mm := updated.(Model)
	if mm.flashText != "" {
		t.Fatalf("flashText after Enter: want empty, got %q", mm.flashText)
	}
}

func TestIsActionableKey_Defensive(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyPressMsg
		want bool
	}{
		{name: "KeyRunes with rune", msg: tea.KeyPressMsg{Code: 'a', Text: "a"}, want: true},
		{name: "named KeyEnter", msg: tea.KeyPressMsg{Code: tea.KeyEnter}, want: true},
		{name: "named KeyEsc", msg: tea.KeyPressMsg{Code: tea.KeyEsc}, want: true},
		{name: "named KeyDown", msg: tea.KeyPressMsg{Code: tea.KeyDown}, want: true},
		{name: "zero KeyMsg", msg: tea.KeyPressMsg{}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isActionableKey(tc.msg); got != tc.want {
				t.Fatalf("isActionableKey(%+v): want %v, got %v", tc.msg, tc.want, got)
			}
		})
	}
}
