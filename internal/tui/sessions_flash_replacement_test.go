package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func applyBail(t *testing.T, m Model, name string) Model {
	t.Helper()
	updated, _ := m.Update(previewAttachBailMsg{Session: name})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T after bail, want tui.Model", updated)
	}
	return got
}

func applyTick(t *testing.T, m Model, gen uint64) Model {
	t.Helper()
	updated, _ := m.Update(flashTickMsg{Gen: gen})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T after tick, want tui.Model", updated)
	}
	return got
}

func TestFlashReplacement_TwoSuccessiveBailsReflectLatestText(t *testing.T) {
	var m Model
	m = applyBail(t, m, "foo")
	if want, got := `session "foo" no longer exists`, m.flashText; got != want {
		t.Fatalf("after first bail: flashText=%q, want %q", got, want)
	}
	if m.flashGen != 1 {
		t.Fatalf("after first bail: flashGen=%d, want 1", m.flashGen)
	}

	m = applyBail(t, m, "bar")
	if want, got := `session "bar" no longer exists`, m.flashText; got != want {
		t.Errorf("after second bail: flashText=%q, want %q (latest bail wins)", got, want)
	}
	if m.flashGen != 2 {
		t.Errorf("after second bail: flashGen=%d, want 2 (bumped per bail)", m.flashGen)
	}
}

func TestFlashReplacement_PriorTickDoesNotClearNewerFlash(t *testing.T) {
	var m Model
	m = applyBail(t, m, "foo")
	m = applyBail(t, m, "bar")
	m = applyTick(t, m, 1)

	if want, got := `session "bar" no longer exists`, m.flashText; got != want {
		t.Errorf("stale tick must not clear newer flash: flashText=%q, want %q", got, want)
	}
	if m.flashGen != 2 {
		t.Errorf("stale tick must not touch flashGen: got %d, want 2", m.flashGen)
	}
}

func TestFlashReplacement_CurrentTickClearsItsOwnFlash(t *testing.T) {
	var m Model
	m = applyBail(t, m, "foo")
	m = applyBail(t, m, "bar")
	m = applyTick(t, m, 2)

	if m.flashText != "" {
		t.Errorf("live tick must clear its own flash: flashText=%q, want %q", m.flashText, "")
	}
	if m.flashGen != 2 {
		t.Errorf("clearFlash must preserve flashGen: got %d, want 2", m.flashGen)
	}
}

func TestFlashReplacement_FiveSuccessiveBailsOnlyLatestSurvives(t *testing.T) {
	var m Model
	names := []string{"a", "b", "c", "d", "e"}
	for _, n := range names {
		m = applyBail(t, m, n)
	}
	if want, got := `session "e" no longer exists`, m.flashText; got != want {
		t.Fatalf("after 5 bails: flashText=%q, want %q (latest wins)", got, want)
	}
	if m.flashGen != 5 {
		t.Fatalf("after 5 bails: flashGen=%d, want 5", m.flashGen)
	}

	for gen := uint64(1); gen <= 4; gen++ {
		m = applyTick(t, m, gen)
		if want, got := `session "e" no longer exists`, m.flashText; got != want {
			t.Errorf("stale tick Gen=%d cleared the live flash: flashText=%q, want %q", gen, got, want)
		}
		if m.flashGen != 5 {
			t.Errorf("stale tick Gen=%d mutated flashGen: got %d, want 5", gen, m.flashGen)
		}
	}

	m = applyTick(t, m, 5)
	if m.flashText != "" {
		t.Errorf("live tick (Gen=5) must clear: flashText=%q, want %q", m.flashText, "")
	}
	if m.flashGen != 5 {
		t.Errorf("clearFlash must preserve flashGen: got %d, want 5", m.flashGen)
	}
}

func TestFlashReplacement_ManualClearBetweenBailsLeavesStaleTicksAsNoOps(t *testing.T) {
	var m Model
	m = applyBail(t, m, "foo")
	m.clearFlash()
	if m.flashText != "" {
		t.Fatalf("manual clear: flashText=%q, want %q", m.flashText, "")
	}
	if m.flashGen != 1 {
		t.Fatalf("manual clear must preserve flashGen: got %d, want 1", m.flashGen)
	}

	m = applyBail(t, m, "bar")
	if want, got := `session "bar" no longer exists`, m.flashText; got != want {
		t.Fatalf("post-manual-clear bail: flashText=%q, want %q", got, want)
	}
	if m.flashGen != 2 {
		t.Fatalf("post-manual-clear bail: flashGen=%d, want 2", m.flashGen)
	}

	m = applyTick(t, m, 1)
	if want, got := `session "bar" no longer exists`, m.flashText; got != want {
		t.Errorf("stale tick (Gen=1) after manual clear + new bail must not clear: flashText=%q, want %q", got, want)
	}
	if m.flashGen != 2 {
		t.Errorf("stale tick must not touch flashGen: got %d, want 2", m.flashGen)
	}
}

func TestFlashReplacement_SetFlashBumpsGenByExactlyOnePerCall(t *testing.T) {
	var m Model
	if m.flashGen != 0 {
		t.Fatalf("zero-value flashGen: got %d, want 0", m.flashGen)
	}

	m.setFlash("x")
	if m.flashGen != 1 {
		t.Errorf("after setFlash #1: flashGen=%d, want 1", m.flashGen)
	}
	m.setFlash("y")
	if m.flashGen != 2 {
		t.Errorf("after setFlash #2: flashGen=%d, want 2", m.flashGen)
	}

	m = applyBail(t, m, "foo")
	if m.flashGen != 3 {
		t.Errorf("after bail #1 post-setFlash: flashGen=%d, want 3", m.flashGen)
	}
	m = applyBail(t, m, "bar")
	if m.flashGen != 4 {
		t.Errorf("after bail #2 post-setFlash: flashGen=%d, want 4", m.flashGen)
	}
}

func TestFlashReplacement_SameNameBailsStillBumpGen(t *testing.T) {
	var m Model
	m = applyBail(t, m, "foo")
	m = applyBail(t, m, "foo")
	if m.flashGen != 2 {
		t.Fatalf("same-name bail #2: flashGen=%d, want 2", m.flashGen)
	}

	m = applyTick(t, m, 1)
	if want, got := `session "foo" no longer exists`, m.flashText; got != want {
		t.Errorf("stale tick on same-name bail: flashText=%q, want %q", got, want)
	}
	if m.flashGen != 2 {
		t.Errorf("stale tick on same-name bail mutated flashGen: got %d, want 2", m.flashGen)
	}

	m = applyTick(t, m, 2)
	if m.flashText != "" {
		t.Errorf("live tick (Gen=2) must clear: flashText=%q, want %q", m.flashText, "")
	}
}

var _ tea.Msg = flashTickMsg{}
