package hookstest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/xdg"
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
	// exclusive with Seed and Body.
	Entries map[string]string
	// Body seeds the store's whole {key: {event: command}} layout, for a caller
	// whose subject spans more than the on-resume event Entries covers. An empty
	// non-nil map stages an empty hooks.json, which is a different state from
	// the absent file a nil one leaves. Mutually exclusive with Seed and
	// Entries.
	Body map[string]map[string]string
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
	// instead of erroring. Mutually exclusive with every seed.
	Unreadable bool
}

// StageStore stages a hooks.json to the given description and returns a store
// over it alongside its path.
func StageStore(t *testing.T, staging Staging) (*hooks.Store, string) {
	t.Helper()
	if seedWays(staging) > 1 {
		t.Fatalf("hookstest.StageStore: more than one of Seed, Entries and Body given — a fixture seeds its file one way")
	}
	if staging.Unreadable && seedWays(staging) > 0 {
		t.Fatalf("hookstest.StageStore: Unreadable staged alongside a seed — a directory holds no content to read")
	}

	dir := staging.Dir
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("hookstest.StageStore: mkdir fixture dir: %v", err)
	}
	path := HooksPath(t, dir)

	switch {
	case staging.Unreadable:
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("hookstest.StageStore: mkdir unreadable hooks path: %v", err)
		}
	case staging.Entries != nil:
		writeSeed(t, path, marshalBody(t, shapeEntries(staging.Entries)))
	case staging.Body != nil:
		writeSeed(t, path, marshalBody(t, staging.Body))
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

// HooksPath returns the path a hooks.json takes in dir and stages nothing —
// neither the file nor its sidecar — for a fixture whose subject is the
// absence of one or the other, or their creation by the code under test.
func HooksPath(t *testing.T, dir string) string {
	t.Helper()
	return filepath.Join(dir, xdg.HooksFile.Filename)
}

// seedWays counts the mutually exclusive ways a staging describes its seed.
func seedWays(staging Staging) int {
	ways := 0
	for _, given := range []bool{staging.Seed != "", staging.Entries != nil, staging.Body != nil} {
		if given {
			ways++
		}
	}
	return ways
}

func writeSeed(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("hookstest.StageStore: write seed hooks.json: %v", err)
	}
}

// shapeEntries lifts {key: on-resume command} pairs into the store's own
// {key: {event: command}} layout.
func shapeEntries(entries map[string]string) map[string]map[string]string {
	shaped := make(map[string]map[string]string, len(entries))
	for key, command := range entries {
		shaped[key] = map[string]string{hooks.EventOnResume.String(): command}
	}
	return shaped
}

func marshalBody(t *testing.T, body map[string]map[string]string) []byte {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("hookstest.StageStore: marshal seed body: %v", err)
	}
	return encoded
}
