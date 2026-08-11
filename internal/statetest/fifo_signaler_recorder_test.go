package statetest_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/statetest"
)

func TestRecordingFIFOSignaler_GlobalErrTakesPrecedence(t *testing.T) {
	sentinel := errors.New("global boom")
	perPath := errors.New("per-path boom")
	r := &statetest.RecordingFIFOSignaler{
		Err:   sentinel,
		ErrOn: map[string]error{"/state/alpha.fifo": perPath},
	}

	err := r.SendSignal("/state/alpha.fifo")
	if !errors.Is(err, sentinel) {
		t.Errorf("SendSignal err = %v; want global sentinel %v (must dominate ErrOn)", err, sentinel)
	}

	err = r.SendSignal("/state/beta.fifo")
	if !errors.Is(err, sentinel) {
		t.Errorf("SendSignal err = %v; want global sentinel %v", err, sentinel)
	}

	want := []string{"/state/alpha.fifo", "/state/beta.fifo"}
	if !reflect.DeepEqual(r.Calls, want) {
		t.Errorf("Calls = %v; want %v (recording is unconditional)", r.Calls, want)
	}
}

func TestRecordingFIFOSignaler_PerPathErrOnReturnsConfiguredError(t *testing.T) {
	sentinel := errors.New("write fifo: i/o error")
	failPath := "/state/broken.fifo"
	r := &statetest.RecordingFIFOSignaler{
		ErrOn: map[string]error{failPath: sentinel},
	}

	if err := r.SendSignal(failPath); !errors.Is(err, sentinel) {
		t.Errorf("SendSignal(failPath) err = %v; want sentinel %v", err, sentinel)
	}
	if err := r.SendSignal("/state/healthy.fifo"); err != nil {
		t.Errorf("SendSignal(non-failing path) err = %v; want nil", err)
	}

	want := []string{failPath, "/state/healthy.fifo"}
	if !reflect.DeepEqual(r.Calls, want) {
		t.Errorf("Calls = %v; want %v", r.Calls, want)
	}
}

func TestRecordingFIFOSignaler_DefaultRecordsAndReturnsNil(t *testing.T) {
	r := &statetest.RecordingFIFOSignaler{}

	paths := []string{"/state/a.fifo", "/state/b.fifo", "/state/c.fifo"}
	for _, p := range paths {
		if err := r.SendSignal(p); err != nil {
			t.Errorf("SendSignal(%q) err = %v; want nil", p, err)
		}
	}

	if !reflect.DeepEqual(r.Calls, paths) {
		t.Errorf("Calls = %v; want %v", r.Calls, paths)
	}
}

func TestRecordingFIFOSignaler_SatisfiesFIFOSignaler(t *testing.T) {
	var _ state.FIFOSignaler = (*statetest.RecordingFIFOSignaler)(nil)
}
