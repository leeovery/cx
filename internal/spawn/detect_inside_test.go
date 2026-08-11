package spawn

import (
	"errors"
	"testing"
)

type fakeClientLister struct {
	clients []ClientActivity
	err     error
	calls   []string
}

func (f *fakeClientLister) ListClients(session string) ([]ClientActivity, error) {
	f.calls = append(f.calls, session)
	return f.clients, f.err
}

var (
	ghosttyCommand  = "/Applications/Ghostty.app/Contents/MacOS/ghostty"
	ghosttyAppPath  = "/Applications/Ghostty.app"
	terminalCommand = "/Applications/Terminal.app/Contents/MacOS/Terminal"
	terminalAppPath = "/Applications/Terminal.app"
)

func localWalkSeams() (*fakeWalker, *fakeReader) {
	walker := &fakeWalker{procs: map[int]fakeProc{
		501: {ppid: 1, command: ghosttyCommand},
		502: {ppid: 1, command: terminalCommand},
		// A remote/mosh client walks to NULL.
		601: {ppid: 1, command: "mosh-server"},
		602: {ppid: 1, command: "mosh-server"},
	}}
	reader := &fakeReader{bundles: map[string]fakeBundle{
		ghosttyAppPath:  {bundleID: "com.mitchellh.ghostty", name: "Ghostty"},
		terminalAppPath: {bundleID: "com.apple.Terminal", name: "Terminal"},
	}}
	return walker, reader
}

func TestDetectInsideTmux(t *testing.T) {
	tests := []struct {
		name         string
		clients      []ClientActivity
		wantNull     bool
		wantBundleID string
		wantName     string
	}{
		{
			name: "it returns NULL when every client is remote or mosh",
			clients: []ClientActivity{
				{PID: 601, Activity: 100},
				{PID: 602, Activity: 200},
			},
			wantNull: true,
		},
		{
			name: "it returns the single local client's identity without a tiebreak",
			clients: []ClientActivity{
				{PID: 501, Activity: 0},
			},
			wantBundleID: "com.mitchellh.ghostty",
			wantName:     "Ghostty",
		},
		{
			name: "it picks the highest-client_activity local client among 2+ locals",
			clients: []ClientActivity{
				{PID: 501, Activity: 100},
				{PID: 502, Activity: 200},
			},
			wantBundleID: "com.apple.Terminal",
		},
		{
			name: "it picks the highest activity when the higher-activity client is listed first",
			clients: []ClientActivity{
				{PID: 502, Activity: 200},
				{PID: 501, Activity: 100},
			},
			wantBundleID: "com.apple.Terminal",
		},
		{
			name: "it prefers the first local client on an exact activity tie",
			clients: []ClientActivity{
				{PID: 501, Activity: 150},
				{PID: 502, Activity: 150},
			},
			wantBundleID: "com.mitchellh.ghostty",
		},
		{
			name: "it returns NULL when the most-active client is remote even with a local bystander",
			clients: []ClientActivity{
				{PID: 601, Activity: 9999},
				{PID: 501, Activity: 1},
			},
			wantNull: true,
		},
		{
			name: "it drives the local client when it is most-active despite an idle remote bystander",
			clients: []ClientActivity{
				{PID: 601, Activity: 50},
				{PID: 501, Activity: 200},
			},
			wantBundleID: "com.mitchellh.ghostty",
			wantName:     "Ghostty",
		},
		{
			name:     "it returns clean NULL for zero clients",
			clients:  nil,
			wantNull: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister := &fakeClientLister{clients: tt.clients}
			walker, reader := localWalkSeams()

			got, err := detectInsideTmux("dev", lister, walker, reader)
			if err != nil {
				t.Fatalf("detectInsideTmux returned error: %v, want nil", err)
			}
			if len(lister.calls) != 1 || lister.calls[0] != "dev" {
				t.Errorf("lister calls = %v, want exactly [dev]", lister.calls)
			}
			if tt.wantNull {
				if !got.IsNull() {
					t.Errorf("identity = %+v, want NULL", got)
				}
				return
			}
			if got.BundleID != tt.wantBundleID {
				t.Errorf("BundleID = %q, want %q", got.BundleID, tt.wantBundleID)
			}
			if tt.wantName != "" && got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}

	t.Run("it returns a transient error when list-clients fails", func(t *testing.T) {
		listFailure := errors.New("list-clients: server not found")
		lister := &fakeClientLister{err: listFailure}
		walker, reader := localWalkSeams()

		got, err := detectInsideTmux("dev", lister, walker, reader)
		if err == nil {
			t.Fatalf("detectInsideTmux returned nil error, want a transient error")
		}
		if !errors.Is(err, ErrDetectTransient) {
			t.Errorf("errors.Is(err, ErrDetectTransient) = false, want true; err = %v", err)
		}
		if !errors.Is(err, listFailure) {
			t.Errorf("underlying list-clients failure not preserved in the chain; err = %v", err)
		}
		if !got.IsNull() {
			t.Errorf("identity = %+v, want NULL alongside the transient error", got)
		}
	})

	t.Run("it returns a transient error when a walk fails and nothing local resolves", func(t *testing.T) {
		psFailure := errors.New("ps: operation not permitted")
		lister := &fakeClientLister{clients: []ClientActivity{
			{PID: 501, Activity: 100},
		}}
		walker := &fakeWalker{procs: map[int]fakeProc{
			501: {err: psFailure},
		}}
		reader := &fakeReader{bundles: map[string]fakeBundle{}}

		got, err := detectInsideTmux("dev", lister, walker, reader)
		if err == nil {
			t.Fatalf("detectInsideTmux returned nil error, want a transient error")
		}
		if !errors.Is(err, ErrDetectTransient) {
			t.Errorf("errors.Is(err, ErrDetectTransient) = false, want true; err = %v", err)
		}
		if !errors.Is(err, psFailure) {
			t.Errorf("underlying ps failure not preserved in the chain; err = %v", err)
		}
		if !got.IsNull() {
			t.Errorf("identity = %+v, want NULL alongside the transient error", got)
		}
	})

	t.Run("it fails safe to NULL when the most-active winner walk transiently fails", func(t *testing.T) {
		psFailure := errors.New("ps: operation not permitted")
		lister := &fakeClientLister{clients: []ClientActivity{
			{PID: 601, Activity: 100},
			{PID: 501, Activity: 50},
		}}
		walker := &fakeWalker{procs: map[int]fakeProc{
			601: {err: psFailure},
			501: {ppid: 1, command: ghosttyCommand},
		}}
		reader := &fakeReader{bundles: map[string]fakeBundle{
			ghosttyAppPath: {bundleID: "com.mitchellh.ghostty", name: "Ghostty"},
		}}

		got, err := detectInsideTmux("dev", lister, walker, reader)
		if err == nil {
			t.Fatalf("detectInsideTmux returned nil error, want an ErrDetectTransient failure")
		}
		if !errors.Is(err, ErrDetectTransient) {
			t.Errorf("errors.Is(err, ErrDetectTransient) = false, want true; err = %v", err)
		}
		if !errors.Is(err, psFailure) {
			t.Errorf("underlying ps failure not preserved in the chain; err = %v", err)
		}
		if !got.IsNull() {
			t.Errorf("identity = %+v, want NULL alongside the transient error", got)
		}
	})
}
