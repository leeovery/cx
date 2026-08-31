package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestRenameRefusesColonBearingName(t *testing.T) {
	t.Run("it refuses a colon-bearing rename from the TUI and reports why", func(t *testing.T) {
		rec := &recordingRenamer{}
		m := newRenameTestModel(rec, "alpha", "a:b")

		updated, _ := m.updateRenameModal(tea.KeyPressMsg{Code: tea.KeyEnter})
		um := updated.(Model)

		if rec.called {
			t.Error("a colon-bearing rename must reach no renamer")
		}
		if um.modal != modalNone {
			t.Errorf("modal = %v, want modalNone after a refusal", um.modal)
		}
		if um.flashText != renameColonRefusedFlash {
			t.Errorf("flashText = %q, want %q", um.flashText, renameColonRefusedFlash)
		}
	})

	t.Run("it names the offending character in the refusal message", func(t *testing.T) {
		if !strings.Contains(renameColonRefusedFlash, `":"`) {
			t.Errorf("refusal flash %q does not name the offending character", renameColonRefusedFlash)
		}
		if strings.Contains(renameColonRefusedFlash, flashWarningGlyph) {
			t.Errorf("the refusal text must not embed the %q glyph (the band prepends it): %q", flashWarningGlyph, renameColonRefusedFlash)
		}
	})

	t.Run("it renames a colon-free name unchanged", func(t *testing.T) {
		rec := &recordingRenamer{}
		m := newRenameTestModel(rec, "alpha", "renamed-alpha")

		updated, cmd := m.updateRenameModal(tea.KeyPressMsg{Code: tea.KeyEnter})
		um := updated.(Model)
		if cmd == nil {
			t.Fatal("expected a rename command for a colon-free name, got nil")
		}
		cmd()

		if rec.oldName != "alpha" || rec.newName != "renamed-alpha" {
			t.Errorf("want RenameSession(%q, %q); got RenameSession(%q, %q)", "alpha", "renamed-alpha", rec.oldName, rec.newName)
		}
		if um.flashText != "" {
			t.Errorf("an accepted rename must raise no flash; got %q", um.flashText)
		}
	})
}
