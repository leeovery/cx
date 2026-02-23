package tmux_test

import (
	"fmt"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

// MockCommander implements Commander for testing.
type MockCommander struct {
	Output string
	Err    error
}

// Run returns the configured output and error.
func (m *MockCommander) Run(args ...string) (string, error) {
	return m.Output, m.Err
}

func TestListSessions(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		err      error
		want     []tmux.Session
		wantErr  bool
	}{
		{
			name:   "parses multiple sessions correctly",
			output: "dev|3|1\nwork|5|0\nmisc|1|0",
			want: []tmux.Session{
				{Name: "dev", Windows: 3, Attached: true},
				{Name: "work", Windows: 5, Attached: false},
				{Name: "misc", Windows: 1, Attached: false},
			},
		},
		{
			name:   "parses single session",
			output: "main|2|0",
			want: []tmux.Session{
				{Name: "main", Windows: 2, Attached: false},
			},
		},
		{
			name:   "returns empty slice when tmux server is not running",
			output: "",
			err:    fmt.Errorf("exit status 1"),
			want:   []tmux.Session{},
		},
		{
			name:   "returns empty slice when output is empty",
			output: "",
			want:   []tmux.Session{},
		},
		{
			name:   "attached is true when session_attached > 0",
			output: "session1|2|3",
			want: []tmux.Session{
				{Name: "session1", Windows: 2, Attached: true},
			},
		},
		{
			name:   "attached is false when session_attached is 0",
			output: "session1|2|0",
			want: []tmux.Session{
				{Name: "session1", Windows: 2, Attached: false},
			},
		},
		{
			name:   "handles session name with special characters",
			output: "my-project.v2|4|1",
			want: []tmux.Session{
				{Name: "my-project.v2", Windows: 4, Attached: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockCommander{Output: tt.output, Err: tt.err}
			client := tmux.NewClient(mock)

			got, err := client.ListSessions()

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d sessions, want %d", len(got), len(tt.want))
			}

			for i, session := range got {
				if session.Name != tt.want[i].Name {
					t.Errorf("session[%d].Name = %q, want %q", i, session.Name, tt.want[i].Name)
				}
				if session.Windows != tt.want[i].Windows {
					t.Errorf("session[%d].Windows = %d, want %d", i, session.Windows, tt.want[i].Windows)
				}
				if session.Attached != tt.want[i].Attached {
					t.Errorf("session[%d].Attached = %v, want %v", i, session.Attached, tt.want[i].Attached)
				}
			}
		})
	}
}
