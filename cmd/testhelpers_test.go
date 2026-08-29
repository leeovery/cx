// Staging helpers shared across the cmd test suites: how a test is set up and
// driven. Subject vocabulary — the domain values a suite asserts on and the
// fakes that answer with them — lives in a file named for its subject instead;
// the hook-key half lives in hookkey_vocabulary_test.go.
package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// hooksFileInTempDir points PORTAL_HOOKS_FILE at a hooks.json inside a fresh
// temp directory. The directory is returned alongside the file because several
// callers stage siblings of the hooks file in it.
func hooksFileInTempDir(t *testing.T) (dir, hooksFile string) {
	t.Helper()
	dir = t.TempDir()
	hooksFile = filepath.Join(dir, "hooks.json")
	t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
	return dir, hooksFile
}

// runHookSet drives `hook set --on-resume command` with both streams discarded,
// returning the command's own error.
func runHookSet(t *testing.T, command string) error {
	t.Helper()
	resetRootCmd()
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"hook", "set", "--on-resume", command})
	return rootCmd.Execute()
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

func assertHooksFileUnchanged(t *testing.T, path string, before []byte) {
	t.Helper()
	after := readFileBytes(t, path)
	if !bytes.Equal(before, after) {
		t.Errorf("hooks.json changed on a failing route:\nbefore %s\nafter  %s", before, after)
	}
}
