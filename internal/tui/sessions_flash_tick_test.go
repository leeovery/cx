package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestFlashTickMsg_ClearsFlashWhenGenMatches(t *testing.T) {
	var m Model
	m.setFlash("hello")
	if m.flashText != "hello" || m.flashGen != 1 {
		t.Fatalf("setup invariant: want text=%q gen=1, got text=%q gen=%d", "hello", m.flashText, m.flashGen)
	}

	updated, cmd := m.Update(flashTickMsg{Gen: 1})
	mm, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	if mm.flashText != "" {
		t.Fatalf("flashText after matching tick: want %q, got %q", "", mm.flashText)
	}
	if mm.flashGen != 1 {
		t.Fatalf("flashGen after matching tick: want 1 (preserved), got %d", mm.flashGen)
	}
	if cmd != nil {
		t.Fatalf("flashTickMsg handler returned non-nil cmd: %v", cmd)
	}
}

func TestFlashTickMsg_NoOpWhenGenStale(t *testing.T) {
	var m Model
	m.setFlash("first")
	m.setFlash("second")

	updated, cmd := m.Update(flashTickMsg{Gen: 1})
	mm, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	if mm.flashText != "second" {
		t.Fatalf("flashText after stale tick: want %q (unchanged), got %q", "second", mm.flashText)
	}
	if mm.flashGen != 2 {
		t.Fatalf("flashGen after stale tick: want 2 (unchanged), got %d", mm.flashGen)
	}
	if cmd != nil {
		t.Fatalf("flashTickMsg handler returned non-nil cmd: %v", cmd)
	}
}

func TestFlashTickMsg_IdempotentAfterManualClear(t *testing.T) {
	var m Model
	m.setFlash("x")
	m.clearFlash()

	updated, cmd := m.Update(flashTickMsg{Gen: 1})
	mm, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	if mm.flashText != "" {
		t.Fatalf("flashText after tick on already-cleared: want %q, got %q", "", mm.flashText)
	}
	if mm.flashGen != 1 {
		t.Fatalf("flashGen after tick on already-cleared: want 1 (unchanged), got %d", mm.flashGen)
	}
	if cmd != nil {
		t.Fatalf("flashTickMsg handler returned non-nil cmd: %v", cmd)
	}
}

func TestFlashTickMsg_StaleTickAgainstFreshModelDropped(t *testing.T) {
	var m Model
	updated, cmd := m.Update(flashTickMsg{Gen: 1})
	mm, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	if mm.flashText != "" {
		t.Fatalf("flashText on fresh model: want %q, got %q", "", mm.flashText)
	}
	if mm.flashGen != 0 {
		t.Fatalf("flashGen on fresh model: want 0, got %d", mm.flashGen)
	}
	if cmd != nil {
		t.Fatalf("flashTickMsg handler returned non-nil cmd: %v", cmd)
	}
}

func TestFlashTickCmd_ReturnsNonNilCmd(t *testing.T) {
	cmd := flashTickCmd(7)
	if cmd == nil {
		t.Fatal("flashTickCmd returned nil tea.Cmd")
	}
}

func TestFlashTickCmd_InvokeProducesFlashTickMsgWithCapturedGen(t *testing.T) {
	const wantGen uint64 = 42
	cmd := flashTickCmd(wantGen)
	if cmd == nil {
		t.Fatal("flashTickCmd returned nil tea.Cmd")
	}

	type result struct {
		msg tea.Msg
	}
	ch := make(chan result, 1)
	go func() { ch <- result{msg: cmd()} }()

	select {
	case r := <-ch:
		ftm, ok := r.msg.(flashTickMsg)
		if !ok {
			t.Fatalf("flashTickCmd produced %T, want flashTickMsg", r.msg)
		}
		if ftm.Gen != wantGen {
			t.Fatalf("flashTickMsg.Gen: want %d, got %d", wantGen, ftm.Gen)
		}
	case <-time.After(flashAutoClearDuration + 2*time.Second):
		t.Fatalf("flashTickCmd did not fire within %v", flashAutoClearDuration+2*time.Second)
	}
}

func TestFlashAutoClearDuration_InSanityRange(t *testing.T) {
	if flashAutoClearDuration < 1*time.Second {
		t.Errorf("flashAutoClearDuration too short: %v (want >= 1s)", flashAutoClearDuration)
	}
	if flashAutoClearDuration > 10*time.Second {
		t.Errorf("flashAutoClearDuration too long: %v (want <= 10s)", flashAutoClearDuration)
	}
}
