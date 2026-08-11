package tui

import "testing"

func TestFlashSlotClaim_TierOrder(t *testing.T) {
	t.Run("no flash claims nothing", func(t *testing.T) {
		m := noticeBandModel("alpha-row")

		if _, _, ok := m.flashSlotClaim(); ok {
			t.Error("the flash slot was claimed with no flash live")
		}
	})

	t.Run("a theme flash claims the slot through the theme tier", func(t *testing.T) {
		m := noticeBandModel("alpha-row")
		(&m).setThemeFlash(themePanelNoColorFlash)

		role, message, ok := m.flashSlotClaim()
		wantRole, wantMessage, wantOK := m.themeFlashClaim()
		if !wantOK {
			t.Fatal("the theme tier did not claim a theme flash; the fixture is wrong")
		}
		if role != wantRole || message != wantMessage || ok != wantOK {
			t.Errorf("flashSlotClaim = (%v, %q, %v), want the theme tier's (%v, %q, %v)", role, message, ok, wantRole, wantMessage, wantOK)
		}
	})

	t.Run("an ordinary flash claims the slot through the flash tier", func(t *testing.T) {
		m := noticeBandModel("alpha-row")
		m.setFlash("__ORDINARY__")

		role, message, ok := m.flashSlotClaim()
		wantRole, wantMessage, wantOK := m.flashClaim()
		if !wantOK {
			t.Fatal("the flash tier did not claim an ordinary flash; the fixture is wrong")
		}
		if role != wantRole || message != wantMessage || ok != wantOK {
			t.Errorf("flashSlotClaim = (%v, %q, %v), want the flash tier's (%v, %q, %v)", role, message, ok, wantRole, wantMessage, wantOK)
		}
	})

	t.Run("the theme tier is not displaced while its flash is live", func(t *testing.T) {
		m := noticeBandModel("alpha-row")
		m.setFlash("__ORDINARY__")
		(&m).setThemeFlash(themePanelNoColorFlash)

		if _, message, ok := m.flashSlotClaim(); !ok || message != themePanelNoColorFlash {
			t.Errorf("flashSlotClaim = (%q, %v), want the live theme flash %q", message, ok, themePanelNoColorFlash)
		}
		if m.flashOrigin != flashOriginTheme {
			t.Errorf("flash origin = %v, want the theme origin", m.flashOrigin)
		}
	})

	t.Run("setFlash resets the origin to the default tier", func(t *testing.T) {
		m := noticeBandModel("alpha-row")
		(&m).setThemeFlash(themePanelNoColorFlash)

		m.setFlash("__ORDINARY__")

		if m.flashOrigin != flashOriginDefault {
			t.Errorf("flash origin after setFlash = %v, want the default tier", m.flashOrigin)
		}
	})
}

func TestFlashSlotClaim_BothArbitersOpenWithIt(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(m *Model)
	}{
		{"theme flash", func(m *Model) { m.setThemeFlash(themePanelNoColorFlash) }},
		{"ordinary flash", func(m *Model) { m.setFlash("__ORDINARY__") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := noticeBandModel("alpha-row")
			m.byTagSignpost = true
			m.commandPending = true
			tc.set(&m)

			wantRole, wantMessage, wantOK := m.flashSlotClaim()
			if !wantOK {
				t.Fatal("the shared claim did not take the slot; the fixture is wrong")
			}

			role, message, ok := m.activeNoticeBand()
			if role != wantRole || message != wantMessage || ok != wantOK {
				t.Errorf("Sessions band = (%v, %q, %v), want the shared claim's (%v, %q, %v)", role, message, ok, wantRole, wantMessage, wantOK)
			}

			role, message, ok = m.activeProjectNoticeBand()
			if role != wantRole || message != wantMessage || ok != wantOK {
				t.Errorf("Projects band = (%v, %q, %v), want the shared claim's (%v, %q, %v)", role, message, ok, wantRole, wantMessage, wantOK)
			}
		})
	}
}
