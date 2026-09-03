package tmux_test

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/leeovery/portal/internal/commandertest"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/tmux"
)

const showHooksWarnMessage = "show-hooks failed"

func showHooksWarnRecords(sink *logtest.Sink) logtest.Records {
	return sink.Records().WithMessage(showHooksWarnMessage).AtExactLevel(slog.LevelWarn)
}

func assertShowHooksWarnShape(t *testing.T, rec logtest.Record, wantErr error) {
	t.Helper()
	if gotComponent := rec.AttrOrEmpty("component"); gotComponent != "bootstrap" {
		t.Errorf("WARN component = %q, want %q", gotComponent, "bootstrap")
	}
	if gotErrorClass := rec.AttrString(t, "error_class"); gotErrorClass != "unexpected" {
		t.Errorf("WARN error_class = %q, want %q", gotErrorClass, "unexpected")
	}
	if gotErr := rec.ErrorAttr(t, "error"); !errors.Is(gotErr, wantErr) {
		t.Errorf("WARN error attr %v does not wrap expected error %v", gotErr, wantErr)
	}
}

func TestRegisterPortalHooks_HydrationReadFailureEmitsCanonicalWarn(t *testing.T) {
	sentinel := errors.New("tmux show-hooks failure (hydration)")
	mock := commandertest.FromFunc(perEventDispatchWithFaults(t, "", nil, readErrForAllManagedEvents(sentinel), nil))
	client := tmux.NewClient(mock)

	sink := &logtest.Sink{}
	err := tmux.RegisterPortalHooks(client, slog.New(sink).With("component", "bootstrap"))

	if err == nil {
		t.Fatal("expected error from RegisterPortalHooks, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap sentinel %v", err, sentinel)
	}

	assertNoSetHookCalls(t, mock.Calls())

	warns := showHooksWarnRecords(sink)
	if len(warns) == 0 {
		t.Fatalf("expected at least one %q WARN, got none: %v", showHooksWarnMessage, sink.Records())
	}
	assertShowHooksWarnShape(t, warns[0], sentinel)
}

func TestRegisterPortalHooks_SessionClosedReadFailureEmitsCanonicalWarn(t *testing.T) {
	sentinel := errors.New("tmux show-hooks failure (session-closed)")
	mock := commandertest.FromFunc(perEventDispatchWithFaults(t, "", nil,
		map[string]error{"session-closed": sentinel}, nil))
	client := tmux.NewClient(mock)

	sink := &logtest.Sink{}
	err := tmux.RegisterPortalHooks(client, slog.New(sink).With("component", "bootstrap"))

	if err == nil {
		t.Fatal("expected aggregate error wrapping the session-closed sentinel, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap sentinel %v", err, sentinel)
	}

	warn := showHooksWarnRecords(sink).Only(t, showHooksWarnMessage+" WARN (session-closed)")
	assertShowHooksWarnShape(t, warn, sentinel)

	for _, c := range setHookCalls(mock.Calls()) {
		if c[0] == "session-closed" {
			t.Errorf("session-closed must not be appended when its read fails: %v", c)
		}
	}
}

func TestShowHooksWarn_ErrorAttrCarriesCommandErrorChain(t *testing.T) {
	cmdErr := &tmux.CommandError{
		Stderr: "no server running on /tmp/tmux-1000/default",
		Err:    errors.New("exit status 1"),
		Args:   []string{"show-hooks", "-g", "session-created"},
	}
	mock := commandertest.FromFunc(perEventDispatchWithFaults(t, "", nil,
		map[string]error{"session-created": cmdErr}, nil))
	client := tmux.NewClient(mock)

	sink := &logtest.Sink{}
	if err := tmux.RegisterPortalHooks(client, slog.New(sink).With("component", "bootstrap")); err == nil {
		t.Fatal("expected error, got nil")
	}

	warn := showHooksWarnRecords(sink).Only(t, showHooksWarnMessage+" WARN (only session-created's read fails)")

	gotErr := warn.ErrorAttr(t, "error")
	var asCmdErr *tmux.CommandError
	if !errors.As(gotErr, &asCmdErr) {
		t.Fatalf("WARN error attr %v does not unwrap to *tmux.CommandError", gotErr)
	}
	if asCmdErr.Stderr != cmdErr.Stderr {
		t.Errorf("recovered CommandError.Stderr = %q, want %q", asCmdErr.Stderr, cmdErr.Stderr)
	}
}

func TestRegisterPortalHooks_ShowHooksFailureLoggedExactlyOnce(t *testing.T) {
	sentinel := errors.New("tmux show-hooks fails everywhere")
	mock := commandertest.FromFunc(perEventDispatchWithFaults(t, "", nil, readErrForAllManagedEvents(sentinel), nil))
	client := tmux.NewClient(mock)

	sink := &logtest.Sink{}
	injected := slog.New(sink).With("component", "bootstrap")

	err := tmux.RegisterPortalHooks(client, injected)
	if err == nil {
		t.Fatal("expected aggregate error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("aggregate error %v does not wrap sentinel %v", err, sentinel)
	}

	assertNoSetHookCalls(t, mock.Calls())

	wantSiblingFailures := expectedManagedEventCount
	warns := showHooksWarnRecords(sink)
	if len(warns) != wantSiblingFailures {
		t.Fatalf("expected exactly %d %q WARNs (one per managed event, no aggregate double-log), got %d: %v",
			wantSiblingFailures, showHooksWarnMessage, len(warns), sink.Records())
	}
	for i, w := range warns {
		t.Run("warn-"+string(rune('0'+i)), func(t *testing.T) {
			assertShowHooksWarnShape(t, w, sentinel)
		})
	}
}
