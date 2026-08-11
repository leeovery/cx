package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

func TestSharedConstructorUsedByBothPaths(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}

	const sharedConstructorCall = "tui.Build("
	files := map[string]string{
		"production (cmd/open.go)":       filepath.Join(root, "cmd", "open.go"),
		"capture tool (cmd/capturetool)": filepath.Join(root, "cmd", "capturetool", "main.go"),
	}

	for label, path := range files {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s (%s): %v", label, path, readErr)
		}
		if !strings.Contains(string(data), sharedConstructorCall) {
			t.Errorf("%s does not call the shared %s constructor — the capture frame must be built the same way production builds it", label, sharedConstructorCall)
		}
	}
}
