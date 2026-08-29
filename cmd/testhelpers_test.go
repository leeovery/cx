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

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/transienttest"
)

// lockBound is the lowered acquisition bound the lock-timeout suites drive the
// timeout through, so no case waits out the production figure.
const lockBound = 60 * time.Millisecond

// hooksStoreStaging describes how newStagedHooksStore stages a hooks.json.
type hooksStoreStaging struct {
	// dir holds the hooks.json; empty stages it in a fresh temp directory. A
	// named directory that does not exist yet is created, so a caller can choose
	// a name that then appears in the paths a failing write reports.
	dir string
	// seed is the file's initial content; empty writes no file at all.
	seed string
	// writesDenied strips write permission from dir once staging is complete, so
	// a mutation takes its lock and reads cleanly but fails at the temp create.
	writesDenied bool
}

// newStagedHooksStore stages a hooks.json to the given description and returns
// a store over it alongside its path.
func newStagedHooksStore(t *testing.T, staging hooksStoreStaging) (*hooks.Store, string) {
	t.Helper()
	dir := staging.dir
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}
	path := filepath.Join(dir, "hooks.json")
	if staging.seed != "" {
		if err := os.WriteFile(path, []byte(staging.seed), 0o644); err != nil {
			t.Fatalf("write seed hooks.json: %v", err)
		}
	}
	// The sidecar stands in for the one a writer establishes on a real install,
	// so a read under this fixture takes its shared lock rather than degrading
	// and emitting a load-unlocked breadcrumb the fixture never meant to model.
	// It is created before any denial, so a denied write fails at the temp
	// create rather than earlier at the sidecar's own open.
	transienttest.CreateHooksSidecar(t, path)
	if staging.writesDenied {
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatalf("chmod fixture dir: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	}
	return hooks.NewStore(path), path
}

// newTempHooksStore stages a writable seeded hooks.json in a fresh temp
// directory — the plain case behind newStagedHooksStore.
func newTempHooksStore(t *testing.T, seed string) (*hooks.Store, string) {
	t.Helper()
	return newStagedHooksStore(t, hooksStoreStaging{seed: seed})
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

// assertHooksFileUnchanged proves a route left the file byte for byte as it
// found it. An optional context names the route in place of the default, so a
// caller reporting something more specific than a failing write keeps its own
// words in the failure.
func assertHooksFileUnchanged(t *testing.T, path string, before []byte, context ...string) {
	t.Helper()
	what := "changed on a failing route"
	if len(context) > 0 {
		what = context[0]
	}
	after := readFileBytes(t, path)
	if !bytes.Equal(before, after) {
		t.Errorf("hooks.json %s:\nbefore %s\nafter  %s", what, before, after)
	}
}
