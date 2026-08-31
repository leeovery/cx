// Staging helpers shared across the cmd test suites: how a test is set up and
// driven. Subject vocabulary — the domain values a suite asserts on and the
// fakes that answer with them — lives in a file named for its subject instead;
// the hook-key half lives in hookkey_vocabulary_test.go.
package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hookstest"
)

// lockBound is the lowered acquisition bound the lock-timeout suites drive the
// timeout through, so no case waits out the production figure.
const lockBound = 60 * time.Millisecond

// withHooksDeps installs deps as the package-level hooks seam for the rest of
// the test and registers the restore in the same breath. The seam outlives the
// test that sets it, so the install and its restore are written together here
// rather than left to each call site to pair.
func withHooksDeps(t *testing.T, deps HooksDeps) {
	t.Helper()
	hooksDeps = &deps
	t.Cleanup(func() { hooksDeps = nil })
}

// withoutHooksDeps leaves the hooks seam unset for the rest of the test, so a
// case whose subject is the production default states that precondition instead
// of inheriting it.
func withoutHooksDeps(t *testing.T) {
	t.Helper()
	hooksDeps = nil
	t.Cleanup(func() { hooksDeps = nil })
}

// readFileBytes returns nil on ENOENT, so callers can distinguish "file absent"
// from "file empty".
func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// TestReadFileBytes pins the read rule the hooks/projects assertion pairs both
// depend on: exact bytes when the file is there, nil when it is not, whichever
// of the two config files is named.
func TestReadFileBytes(t *testing.T) {
	t.Run("it reads projects.json by the same rule", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "projects.json")

		if got := readFileBytes(t, path); got != nil {
			t.Errorf("readFileBytes of an absent projects.json = %q, want nil", got)
		}

		body := []byte(`{"projects":[{"path":"/tmp/proj","name":"proj0"}]}`)
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatalf("seed projects.json: %v", err)
		}
		if got := readFileBytes(t, path); !bytes.Equal(got, body) {
			t.Errorf("readFileBytes = %s, want %s", got, body)
		}
	})
}

// hooksFileInTempDir points PORTAL_HOOKS_FILE at a hooks.json inside a fresh
// temp directory, and stages the sidecar beside it. It is the second of the two
// staging routes: this one leaves the path to the command body's own
// resolution, where hookstest.StageStore hands a store straight to a seam. A
// case whose subject is the sidecar's absence stages its own file through
// hookstest.StageStore rather than reaching for this route.
//
// The directory is returned alongside the file because several callers stage
// siblings of the hooks file in it.
func hooksFileInTempDir(t *testing.T) (dir, hooksFile string) {
	t.Helper()
	dir = t.TempDir()
	hooksFile = filepath.Join(dir, "hooks.json")
	t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
	// Created while the directory still permits it: a fixture that strips the
	// directory's permissions afterwards could not stage it.
	hookstest.CreateHooksSidecar(t, hooksFile)
	return dir, hooksFile
}

// runHookSet drives `hook set --on-resume command` with both streams captured,
// returning what the command wrote alongside its own error.
func runHookSet(t *testing.T, command string) (string, error) {
	t.Helper()
	buf := new(bytes.Buffer)
	resetRootCmd()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"hook", "set", "--on-resume", command})
	err := rootCmd.Execute()
	return buf.String(), err
}

// runHookRm drives `hook rm --on-resume [extra…]` with both streams captured,
// returning what the command wrote alongside its own error.
func runHookRm(t *testing.T, extra ...string) (string, error) {
	t.Helper()
	buf := new(bytes.Buffer)
	resetRootCmd()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(append([]string{"hook", "rm", "--on-resume"}, extra...))
	err := rootCmd.Execute()
	return buf.String(), err
}

// runHookList drives `hook list`, failing the test on a non-zero exit and
// returning what the command wrote to stdout.
func runHookList(t *testing.T) string {
	t.Helper()
	buf := new(bytes.Buffer)
	resetRootCmd()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"hook", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return buf.String()
}

func writeHooksJSON(t *testing.T, path string, data map[string]map[string]string) {
	t.Helper()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal hooks JSON: %v", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("failed to write hooks file: %v", err)
	}
}

func readHooksJSON(t *testing.T, path string) map[string]map[string]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read hooks file: %v", err)
	}
	var data map[string]map[string]string
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatalf("failed to unmarshal hooks JSON: %v", err)
	}
	return data
}

// seedHooksFile writes data and returns the bytes on disk, so a caller can prove
// a failing route left the file untouched byte for byte.
func seedHooksFile(t *testing.T, path string, data map[string]map[string]string) []byte {
	t.Helper()
	writeHooksJSON(t, path, data)
	return readFileBytes(t, path)
}

// assertHooksFileUnchanged names the shared byte-identity assertion in this
// package's own vocabulary, so a cmd test reads the same as its neighbours.
func assertHooksFileUnchanged(t *testing.T, path string, before []byte, context ...string) {
	t.Helper()
	hookstest.AssertHooksFileUnchanged(t, path, before, context...)
}
