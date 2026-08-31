package hookstest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
)

// Staging describes how StageStore stages a hooks.json.
type Staging struct {
	// Dir holds the hooks.json; empty stages it in a fresh temp directory. A
	// named directory that does not exist yet is created, so a caller can choose
	// a name that then appears in the paths a failing write reports.
	Dir string
	// Seed is the file's raw initial content; empty writes no file at all. It
	// takes an arbitrary body, including one no mutation would produce.
	Seed string
	// Entries seeds {key: on-resume command} pairs in the store's own layout, so
	// a caller wanting specific hooks need not hand-write JSON. Mutually
	// exclusive with Seed.
	Entries map[string]string
	// SidecarAbsent stages no lock file beside the hooks.json, so a read under
	// the fixture degrades to an unlocked one. It is off by default because
	// these fixtures model a written-to install — the steady state, where the
	// sidecar the first mutation created still sits beside the file it was
	// created for — and because a fixture that degrades without asking to
	// leaves a breadcrumb in its sink it never meant to assert on. The absence
	// is a state an install can hold: nothing creates a sidecar until the first
	// mutation, and the config-directory migration moves hooks.json without it.
	// A fixture whose subject is that state, or the degraded read it produces,
	// asks for the absence here.
	SidecarAbsent bool
	// WritesDenied strips write permission from the directory once staging is
	// complete, so a mutation takes its lock and reads cleanly but fails at the
	// temp create.
	WritesDenied bool
	// Unreadable stages a directory where the hooks.json belongs, so every read
	// of it fails. Malformed JSON would not do: it decodes to an empty map
	// instead of erroring. Mutually exclusive with Seed and Entries.
	Unreadable bool
}

// StageStore stages a hooks.json to the given description and returns a store
// over it alongside its path.
func StageStore(t *testing.T, staging Staging) (*hooks.Store, string) {
	t.Helper()
	if staging.Seed != "" && staging.Entries != nil {
		t.Fatalf("hookstest.StageStore: Seed and Entries both given — a fixture seeds its file one way")
	}
	if staging.Unreadable && (staging.Seed != "" || staging.Entries != nil) {
		t.Fatalf("hookstest.StageStore: Unreadable staged alongside a seed — a directory holds no content to read")
	}

	dir := staging.Dir
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("hookstest.StageStore: mkdir fixture dir: %v", err)
	}
	path := filepath.Join(dir, "hooks.json")

	switch {
	case staging.Unreadable:
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("hookstest.StageStore: mkdir unreadable hooks path: %v", err)
		}
	case staging.Entries != nil:
		writeSeed(t, path, marshalEntries(t, staging.Entries))
	case staging.Seed != "":
		writeSeed(t, path, []byte(staging.Seed))
	}

	if !staging.SidecarAbsent {
		// Created before any denial, so a denied write fails at the temp create
		// rather than earlier at the sidecar's own open.
		CreateHooksSidecar(t, path)
	}
	if staging.WritesDenied {
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatalf("hookstest.StageStore: chmod fixture dir: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	}
	return hooks.NewStore(path), path
}

func writeSeed(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("hookstest.StageStore: write seed hooks.json: %v", err)
	}
}

func marshalEntries(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	shaped := make(map[string]map[string]string, len(entries))
	for key, command := range entries {
		shaped[key] = map[string]string{"on-resume": command}
	}
	body, err := json.Marshal(shaped)
	if err != nil {
		t.Fatalf("hookstest.StageStore: marshal seed entries: %v", err)
	}
	return body
}
