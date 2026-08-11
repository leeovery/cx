package statetest

import "github.com/leeovery/portal/internal/state"

// RecordingFIFOSignaler records every signalled path, with optional error
// injection.
type RecordingFIFOSignaler struct {
	// Calls holds every path passed to SendSignal, including calls that then
	// returned an injected error.
	Calls []string
	ErrOn map[string]error
	Err   error
}

// SendSignal's non-nil Err takes precedence over ErrOn.
func (r *RecordingFIFOSignaler) SendSignal(path string) error {
	r.Calls = append(r.Calls, path)
	if r.Err != nil {
		return r.Err
	}
	if e, ok := r.ErrOn[path]; ok {
		return e
	}
	return nil
}

var _ state.FIFOSignaler = (*RecordingFIFOSignaler)(nil)
