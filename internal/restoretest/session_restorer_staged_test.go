//go:build integration

package restoretest

import (
	"path/filepath"
	"testing"
)

func TestNewSessionRestorer(t *testing.T) {
	t.Run("it pins the staged binary on the restorer it returns", func(t *testing.T) {
		binDir := t.TempDir()
		stateDir := t.TempDir()

		r := NewSessionRestorer(t, nil, stateDir, binDir)

		if r.Exe == nil {
			t.Fatal("restorer Exe is nil; want the staged binary pinned")
		}
		got, err := r.Exe()
		if err != nil {
			t.Fatalf("Exe() returned err = %v; want nil", err)
		}
		if want := filepath.Join(binDir, "portal"); got != want {
			t.Errorf("Exe() = %q; want %q", got, want)
		}
		if r.StateDir != stateDir {
			t.Errorf("StateDir = %q; want %q", r.StateDir, stateDir)
		}
		if r.Logger == nil {
			t.Error("Logger is nil; want the state dir's test logger")
		}
	})
}
