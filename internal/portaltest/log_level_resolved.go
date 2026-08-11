package portaltest

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// AssertLogLevelResolved fails the test unless the log at logPath carries a
// `process: log-level resolved` line for pid with the expected level and source
// "env" — i.e. unless PORTAL_LOG_LEVEL reached the spawned process. Only
// text-mode logs are parsed.
func AssertLogLevelResolved(t *testing.T, logPath string, pid int, expected string) {
	t.Helper()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("AssertLogLevelResolved: cannot read log at %s: %v", logPath, err)
		return
	}
	content := string(data)

	resolved, source, found := findLogLevelResolved(content, pid)
	if !found {
		t.Errorf("PORTAL_LOG_LEVEL did not propagate: no process: log-level resolved line for pid=%d\n--- %s ---\n%s",
			pid, logPath, content)
		return
	}
	if source != "env" {
		t.Errorf("PORTAL_LOG_LEVEL was not the resolution source for pid=%d: source=%q (want env; default/fallback means the harness did not set it or set it invalidly)",
			pid, source)
	}
	if resolved != expected {
		t.Errorf("resolved log level for pid=%d = %q, want %q", pid, resolved, expected)
	}
}

// Several processes may share one day file, so the pid match is load-bearing.
func findLogLevelResolved(content string, pid int) (resolved, source string, found bool) {
	wantPID := strconv.Itoa(pid)
	for line := range strings.SplitSeq(content, "\n") {
		if !isLogLevelResolvedLine(line) {
			continue
		}
		attrs := parseLogAttrs(line)
		if attrs["pid"] != wantPID {
			continue
		}
		return attrs["resolved"], attrs["source"], true
	}
	return "", "", false
}

func isLogLevelResolvedLine(line string) bool {
	return strings.Contains(line, "process:") &&
		strings.Contains(line, "log-level resolved")
}

// parseLogAttrs does not reconstruct a quoted value containing spaces; the attrs
// read here are all single-token.
func parseLogAttrs(line string) map[string]string {
	attrs := make(map[string]string)
	for tok := range strings.FieldsSeq(line) {
		k, v, ok := strings.Cut(tok, "=")
		if !ok {
			continue
		}
		attrs[k] = strings.Trim(v, `"`)
	}
	return attrs
}
