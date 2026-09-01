package tmux_test

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/tmux"
)

const showHooksWarnMessage = "show-hooks failed"

func showHooksWarnRecords(sink *logtest.Sink) logtest.Records {
	return sink.RecordsAtExactLevelWithMessage(slog.LevelWarn, showHooksWarnMessage)
}

func assertShowHooksWarnShape(t *testing.T, rec logtest.Record, wantErr error) {
	t.Helper()
	gotComponent := ""
	if v, ok := rec.Attrs["component"]; ok {
		gotComponent = v.String()
	}
	errorClass, sawErrorClass := rec.Attrs["error_class"]
	gotErrorClass := errorClass.String()
	errorValue, sawError := rec.Attrs["error"]
	gotErr, _ := errorValue.Any().(error)
	if gotComponent != "bootstrap" {
		t.Errorf("WARN component = %q, want %q", gotComponent, "bootstrap")
	}
	if !sawErrorClass {
		t.Fatalf("WARN missing error_class attr: %v", rec)
	}
	if gotErrorClass != "unexpected" {
		t.Errorf("WARN error_class = %q, want %q", gotErrorClass, "unexpected")
	}
	if !sawError {
		t.Fatalf("WARN missing error attr: %v", rec)
	}
	if gotErr == nil {
		t.Fatalf("WARN error attr is not an error value (was passed .Error()?): %v", rec)
	}
	if !errors.Is(gotErr, wantErr) {
		t.Errorf("WARN error attr %v does not wrap expected error %v", gotErr, wantErr)
	}
}

func TestRegisterPortalHooks_HydrationReadFailureEmitsCanonicalWarn(t *testing.T) {
	sentinel := errors.New("tmux show-hooks failure (hydration)")
	mock := &MockCommander{
		RunFunc: perEventDispatchWithFaults(t, "", nil, readErrForAllManagedEvents(sentinel), nil),
	}
	client := tmux.NewClient(mock)

	sink := &logtest.Sink{}
	err := tmux.RegisterPortalHooks(client, slog.New(sink).With("component", "bootstrap"))

	if err == nil {
		t.Fatal("expected error from RegisterPortalHooks, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap sentinel %v", err, sentinel)
	}

	assertNoSetHookCalls(t, mock.Calls)

	warns := showHooksWarnRecords(sink)
	if len(warns) == 0 {
		t.Fatalf("expected at least one %q WARN, got none: %v", showHooksWarnMessage, sink.Records())
	}
	assertShowHooksWarnShape(t, warns[0], sentinel)
}

func TestRegisterPortalHooks_SessionClosedReadFailureEmitsCanonicalWarn(t *testing.T) {
	sentinel := errors.New("tmux show-hooks failure (session-closed)")
	mock := &MockCommander{RunFunc: perEventDispatchWithFaults(t, "", nil,
		map[string]error{"session-closed": sentinel}, nil)}
	client := tmux.NewClient(mock)

	sink := &logtest.Sink{}
	err := tmux.RegisterPortalHooks(client, slog.New(sink).With("component", "bootstrap"))

	if err == nil {
		t.Fatal("expected aggregate error wrapping the session-closed sentinel, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap sentinel %v", err, sentinel)
	}

	warns := showHooksWarnRecords(sink)
	if len(warns) != 1 {
		t.Fatalf("expected exactly 1 %q WARN (session-closed), got %d: %v", showHooksWarnMessage, len(warns), sink.Records())
	}
	assertShowHooksWarnShape(t, warns[0], sentinel)

	for _, c := range setHookCalls(mock.Calls) {
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
	mock := &MockCommander{
		RunFunc: perEventDispatchWithFaults(t, "", nil,
			map[string]error{"session-created": cmdErr}, nil),
	}
	client := tmux.NewClient(mock)

	sink := &logtest.Sink{}
	if err := tmux.RegisterPortalHooks(client, slog.New(sink).With("component", "bootstrap")); err == nil {
		t.Fatal("expected error, got nil")
	}

	warns := showHooksWarnRecords(sink)
	if len(warns) != 1 {
		t.Fatalf("expected exactly 1 %q WARN (only session-created's read fails), got %d: %v", showHooksWarnMessage, len(warns), sink.Records())
	}

	gotErr, _ := warns[0].Attrs["error"].Any().(error)
	if gotErr == nil {
		t.Fatal("WARN error attr is not an error value (was passed .Error()?)")
	}
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
	mock := &MockCommander{
		RunFunc: perEventDispatchWithFaults(t, "", nil, readErrForAllManagedEvents(sentinel), nil),
	}
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

	assertNoSetHookCalls(t, mock.Calls)

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
