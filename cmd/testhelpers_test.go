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
	// sidecarAbsent stages no lock file beside the hooks.json, so a read under
	// the fixture degrades to an unlocked one. It is off by default because
	// these fixtures model a written-to install — the steady state, where the
	// sidecar the first mutation created still sits beside the file it was
	// created for — and because a fixture that degrades without asking to
	// leaves a breadcrumb in its sink it never meant to assert on. The absence
	// is a state an install can hold: nothing creates a sidecar until the first
	// mutation, and the config-directory migration moves hooks.json without it.
	// A fixture whose subject is that state, or the degraded read it produces,
	// asks for the absence here.
	sidecarAbsent bool
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
	if !staging.sidecarAbsent {
		// Created before any denial, so a denied write fails at the temp create
		// rather than earlier at the sidecar's own open.
		transienttest.CreateHooksSidecar(t, path)
	}
	if staging.writesDenied {
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatalf("chmod fixture dir: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	}
	return hooks.NewStore(path), path
}

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
