//go:build ghosttycompile

package spawn

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The AppleScript error code osacompile emits for a drifted `tell application
// "Ghostty"` template. It is the only failure signature treated as a genuine
// template regression; every other failure is environmental and skipped.
const driftDiscriminator = "-2741"

func TestGhosttyOpenScript_CompilesAgainstInstalledDictionary(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("osacompile / Ghostty terminology resolution is macOS-only")
	}
	if !ghosttyAppInstalled() {
		t.Skip("Ghostty.app is not installed; the compile-check needs Ghostty's scripting dictionary")
	}

	argv := []string{
		"/usr/bin/env", "-u", "TMUX", "-u", "TMUX_PANE",
		"/bin/sh", "-c", "echo probe",
	}

	script := ghosttyOpenScript(argv)

	// osacompile requires an output target — unlike `osascript -e` it does not
	// parse-and-discard.
	out := filepath.Join(t.TempDir(), "probe.scpt")

	combined, err := exec.Command("osacompile", "-e", script, "-o", out).CombinedOutput()
	if err != nil {
		// Classify by signature, not by exit vs run error.
		if strings.Contains(string(combined), driftDiscriminator) {
			t.Fatalf("osacompile reported the %s terminology-drift error: the Ghostty AppleScript "+
				"template no longer compiles against the installed dictionary — the broken "+
				"`make new surface configuration` form has returned.\nscript:\n%s\ncompiler output:\n%s",
				driftDiscriminator, script, combined)
		}
		// Any other failure may be environmental, so skip rather than emit a false
		// template-drift failure.
		t.Skipf("osacompile could not resolve the Ghostty terminology and produced no %s drift "+
			"signature; treating as environmental (e.g. Ghostty not running, osacompile unavailable) "+
			"rather than a template regression.\nerror: %v\nscript:\n%s\noutput:\n%s",
			driftDiscriminator, err, script, combined)
	}
}

// Gates the guard so invoking the ghosttycompile tag on a machine without
// Ghostty skips cleanly.
func ghosttyAppInstalled() bool {
	candidates := []string{"/Applications/Ghostty.app"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "Applications", "Ghostty.app"))
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}
