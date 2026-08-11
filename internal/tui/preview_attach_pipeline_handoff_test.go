package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPreviewAttachPipeline_SuccessReturnsSelectedMsg(t *testing.T) {
	tm := &fakePreviewAttachTmux{hasPresent: true}
	logger, _ := newTestLogger(t)
	p := &previewAttachPipeline{tmux: tm, logger: logger}

	msg := runPipelineCmd(t, p.Run("foo", 2, 5))

	got, ok := msg.(previewAttachSelectedMsg)
	if !ok {
		t.Fatalf("message type = %T, want previewAttachSelectedMsg", msg)
	}
	if got.Session != "foo" {
		t.Errorf("Session = %q, want %q", got.Session, "foo")
	}
	if len(tm.calls) != 3 {
		t.Errorf("expected 3 tmux calls, got %d: %#v", len(tm.calls), tm.calls)
	}
}

func TestPreviewAttachIntegration_ConnectInvokedAfterQuit(t *testing.T) {
	tm := &fakePreviewAttachTmux{hasPresent: true}
	logger, _ := newTestLogger(t)
	pipeline := &previewAttachPipeline{tmux: tm, logger: logger}

	msg := pipeline.Run("foo", 1, 0)()
	sel, ok := msg.(previewAttachSelectedMsg)
	if !ok {
		t.Fatalf("expected previewAttachSelectedMsg, got %T", msg)
	}

	m := modelWithSeams(t, nil, &stubEnumerator{}, &recordingReader{})
	updated, cmd := m.Update(sel)
	got := updated.(Model)
	if got.Selected() != "foo" {
		t.Fatalf("expected Selected()=%q, got %q", "foo", got.Selected())
	}
	if cmd == nil || cmd() != tea.Quit() {
		t.Fatalf("expected tea.Quit cmd from selected handler")
	}

	conn := &fakePreviewConnector{}
	if got.Selected() != "" {
		_ = conn.Connect(got.Selected())
	}
	if len(conn.calls) != 1 || conn.calls[0] != "foo" {
		t.Errorf("connector.Connect calls = %#v, want [foo]", conn.calls)
	}
}
