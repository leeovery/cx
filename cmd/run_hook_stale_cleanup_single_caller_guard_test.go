package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The \bCleanStale\b word boundary is deliberate: the marker-sweep method
// CleanStaleMarkers carries a trailing word char, so only the bare hooks
// method matches. Only production files are scanned — test files legitimately
// mention the name in prose.
func TestHooksCleanStale_NoBootstrapStepIsAnAutomaticCaller(t *testing.T) {
	forbidden := regexp.MustCompile(`\bCleanStale\b|\brunHookStaleCleanup\b`)

	const bootstrapDir = "bootstrap"
	entries, err := os.ReadDir(bootstrapDir)
	if err != nil {
		t.Fatalf("read %s: %v", bootstrapDir, err)
	}

	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(bootstrapDir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		scanned++
		if loc := forbidden.FindIndex(src); loc != nil {
			line := 1 + strings.Count(string(src[:loc[0]]), "\n")
			t.Errorf("%s:%d references the hooks CleanStale path; the daemon (maybeRunHookCleanup) "+
				"and `doctor --fix` (pruneDoctorStaleHooks) are the only permitted callers — a bootstrap step must not clean hooks",
				path, line)
		}
	}

	// The directory must actually contain production source, else a layout
	// change would leave this guard scanning nothing.
	if scanned == 0 {
		t.Fatalf("no production .go files scanned under %q; guard is vacuous", bootstrapDir)
	}
}
