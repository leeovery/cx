package tmux_test

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

const showHooksWarnMessage = "show-hooks failed"

func showHooksWarnRecords(recs []slog.Record) []slog.Record {
	var out []slog.Record
	for _, r := range recs {
		if r.Level == slog.LevelWarn && r.Message == showHooksWarnMessage {
			out = append(out, r)
		}
	}
	return out
}

func assertShowHooksWarnShape(t *testing.T, rec slog.Record, wantErr error) {
	t.Helper()
	var gotComponent, gotErrorClass string
	var gotErr error
	var sawError, sawErrorClass bool
	rec.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "component":
			gotComponent = a.Value.String()
		case "error_class":
			gotErrorClass = a.Value.String()
			sawErrorClass = true
		case "error":
			sawError = true
			if e, ok := a.Value.Any().(error); ok {
				gotErr = e
			}
		}
		return true
	})
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

	rec := &recordingSlogHandler{}
	err := tmux.RegisterPortalHooks(client, slog.New(rec).With("component", "bootstrap"))

	if err == nil {
		t.Fatal("expected error from RegisterPortalHooks, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap sentinel %v", err, sentinel)
	}

	assertNoSetHookCalls(t, mock.Calls)

	warns := showHooksWarnRecords(rec.records)
	if len(warns) == 0 {
		t.Fatalf("expected at least one %q WARN, got none: %v", showHooksWarnMessage, rec.records)
	}
	assertShowHooksWarnShape(t, warns[0], sentinel)
}

func TestRegisterPortalHooks_SessionClosedReadFailureEmitsCanonicalWarn(t *testing.T) {
	sentinel := errors.New("tmux show-hooks failure (session-closed)")
	mock := &MockCommander{RunFunc: perEventDispatchWithFaults(t, "", nil,
		map[string]error{"session-closed": sentinel}, nil)}
	client := tmux.NewClient(mock)

	rec := &recordingSlogHandler{}
	err := tmux.RegisterPortalHooks(client, slog.New(rec).With("component", "bootstrap"))

	if err == nil {
		t.Fatal("expected aggregate error wrapping the session-closed sentinel, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap sentinel %v", err, sentinel)
	}

	warns := showHooksWarnRecords(rec.records)
	if len(warns) != 1 {
		t.Fatalf("expected exactly 1 %q WARN (session-closed), got %d: %v", showHooksWarnMessage, len(warns), rec.records)
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

	rec := &recordingSlogHandler{}
	if err := tmux.RegisterPortalHooks(client, slog.New(rec).With("component", "bootstrap")); err == nil {
		t.Fatal("expected error, got nil")
	}

	warns := showHooksWarnRecords(rec.records)
	if len(warns) != 1 {
		t.Fatalf("expected exactly 1 %q WARN (only session-created's read fails), got %d: %v", showHooksWarnMessage, len(warns), rec.records)
	}

	var gotErr error
	warns[0].Attrs(func(a slog.Attr) bool {
		if a.Key == "error" {
			if e, ok := a.Value.Any().(error); ok {
				gotErr = e
			}
		}
		return true
	})
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

	rec := &recordingSlogHandler{}
	injected := slog.New(rec).With("component", "bootstrap")

	err := tmux.RegisterPortalHooks(client, injected)
	if err == nil {
		t.Fatal("expected aggregate error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("aggregate error %v does not wrap sentinel %v", err, sentinel)
	}

	assertNoSetHookCalls(t, mock.Calls)

	wantSiblingFailures := expectedManagedEventCount
	warns := showHooksWarnRecords(rec.records)
	if len(warns) != wantSiblingFailures {
		t.Fatalf("expected exactly %d %q WARNs (one per managed event, no aggregate double-log), got %d: %v",
			wantSiblingFailures, showHooksWarnMessage, len(warns), rec.records)
	}
	for i, w := range warns {
		t.Run("warn-"+string(rune('0'+i)), func(t *testing.T) {
			assertShowHooksWarnShape(t, w, sentinel)
		})
	}
}
