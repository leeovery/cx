package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestRenameRefusesUnaddressableName(t *testing.T) {
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
		if um.flashText != renameSeparatorRefusedFlash {
			t.Errorf("flashText = %q, want %q", um.flashText, renameSeparatorRefusedFlash)
		}
	})

	t.Run("it names the offending character in the refusal message", func(t *testing.T) {
		if !strings.Contains(renameSeparatorRefusedFlash, `":"`) {
			t.Errorf("refusal flash %q does not name the offending character", renameSeparatorRefusedFlash)
		}
		if strings.Contains(renameSeparatorRefusedFlash, flashWarningGlyph) {
			t.Errorf("the refusal text must not embed the %q glyph (the band prepends it): %q", flashWarningGlyph, renameSeparatorRefusedFlash)
		}
	})

	t.Run("it refuses a rename to a name beginning with $ and reports why", func(t *testing.T) {
		rec := &recordingRenamer{}
		m := newRenameTestModel(rec, "alpha", "$foo")

		updated, _ := m.updateRenameModal(tea.KeyPressMsg{Code: tea.KeyEnter})
		um := updated.(Model)

		if rec.called {
			t.Error("a rename to an ID-prefixed name must reach no renamer")
		}
		if um.modal != modalNone {
			t.Errorf("modal = %v, want modalNone after a refusal", um.modal)
		}
		if um.flashText != renameIDPrefixRefusedFlash {
			t.Errorf("flashText = %q, want %q", um.flashText, renameIDPrefixRefusedFlash)
		}
	})

	t.Run("it names the offending character in the ID-prefix refusal message", func(t *testing.T) {
		if !strings.Contains(renameIDPrefixRefusedFlash, `"$"`) {
			t.Errorf("refusal flash %q does not name the offending character", renameIDPrefixRefusedFlash)
		}
		if strings.Contains(renameIDPrefixRefusedFlash, flashWarningGlyph) {
			t.Errorf("the refusal text must not embed the %q glyph (the band prepends it): %q", flashWarningGlyph, renameIDPrefixRefusedFlash)
		}
	})

	t.Run("it reports the hyphen refusal with the flag-prefix wording in the rename band", func(t *testing.T) {
		rec := &recordingRenamer{}
		m := newRenameTestModel(rec, "alpha", "-bar")

		updated, _ := m.updateRenameModal(tea.KeyPressMsg{Code: tea.KeyEnter})
		um := updated.(Model)

		if rec.called {
			t.Error("a rename to a hyphen-leading name must reach no renamer")
		}
		if um.modal != modalNone {
			t.Errorf("modal = %v, want modalNone after a refusal", um.modal)
		}
		if um.flashText != renameFlagPrefixRefusedFlash {
			t.Errorf("flashText = %q, want %q", um.flashText, renameFlagPrefixRefusedFlash)
		}
	})

	t.Run("it names the offending character in the flag-prefix refusal message", func(t *testing.T) {
		if !strings.Contains(renameFlagPrefixRefusedFlash, `"-"`) {
			t.Errorf("refusal flash %q does not name the offending character", renameFlagPrefixRefusedFlash)
		}
		if strings.Contains(renameFlagPrefixRefusedFlash, flashWarningGlyph) {
			t.Errorf("the refusal text must not embed the %q glyph (the band prepends it): %q", flashWarningGlyph, renameFlagPrefixRefusedFlash)
		}
		if renameFlagPrefixRefusedFlash == renameIDPrefixRefusedFlash || renameFlagPrefixRefusedFlash == renameSeparatorRefusedFlash {
			t.Errorf("the flag-prefix refusal %q must be distinct from its siblings", renameFlagPrefixRefusedFlash)
		}
	})

	t.Run("it renames an addressable name unchanged", func(t *testing.T) {
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
