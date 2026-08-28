package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHooksListCommand(t *testing.T) {
	t.Run("outputs hooks in tab-separated format", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)

		// Stub bootstrap so the real orchestrator never runs against the test's
		// tmux server.
		bootstrapDeps = &BootstrapDeps{Orchestrator: &nopRunner{}}
		t.Cleanup(func() { bootstrapDeps = nil })

		data := map[string]map[string]string{
			"my-project-abc123:0.0": {"on-resume": "claude --resume abc123"},
		}
		writeHooksJSON(t, hooksFile, data)

		buf := new(bytes.Buffer)
		resetRootCmd()
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"hooks", "list"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := buf.String()
		want := "my-project-abc123:0.0\ton-resume\tclaude --resume abc123\n"
		if got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("produces empty output when no hooks registered", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)

		writeHooksJSON(t, hooksFile, map[string]map[string]string{})

		buf := new(bytes.Buffer)
		resetRootCmd()
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"hooks", "list"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := buf.String()
		if got != "" {
			t.Errorf("output = %q, want empty string", got)
		}
	})

	t.Run("produces empty output when hooks file does not exist", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)

		buf := new(bytes.Buffer)
		resetRootCmd()
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"hooks", "list"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := buf.String()
		if got != "" {
			t.Errorf("output = %q, want empty string", got)
		}
	})

	t.Run("outputs hooks sorted by key then event", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)

		// Stub bootstrap so the real orchestrator never runs against the test's
		// tmux server.
		bootstrapDeps = &BootstrapDeps{Orchestrator: &nopRunner{}}
		t.Cleanup(func() { bootstrapDeps = nil })

		data := map[string]map[string]string{
			"proj-abc:1.0":   {"on-resume": "claude --resume def456"},
			"proj-abc:0.0":   {"on-resume": "claude --resume abc123"},
			"other-proj:0.0": {"on-resume": "npm start"},
		}
		writeHooksJSON(t, hooksFile, data)

		buf := new(bytes.Buffer)
		resetRootCmd()
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"hooks", "list"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := buf.String()
		want := "other-proj:0.0\ton-resume\tnpm start\nproj-abc:0.0\ton-resume\tclaude --resume abc123\nproj-abc:1.0\ton-resume\tclaude --resume def456\n"
		if got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("accepts no arguments", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "list", "extraarg"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error for extra argument, got nil")
		}
	})
}

type mockKeyResolver struct {
	key string
	err error
}

func (m *mockKeyResolver) ResolveHookKey(_ string) (string, error) {
	return m.key, m.err
}

func TestHooksSetCommand(t *testing.T) {
	t.Run("sets hook for current pane", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "aaa111"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "set", "--on-resume", "claude --resume abc123"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if data["aaa111"]["on-resume"] != "claude --resume abc123" {
			t.Errorf("hook command = %q, want %q", data["aaa111"]["on-resume"], "claude --resume abc123")
		}
	})

	t.Run("reads pane ID from TMUX_PANE environment variable", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%99")

		resolver := &mockKeyResolver{key: "bbb222"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "set", "--on-resume", "some-cmd"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data["bbb222"]; !ok {
			t.Error("expected hook entry for hook key bbb222, not found")
		}
		if _, ok := data["%99"]; ok {
			t.Error("raw pane ID %99 should not be used as key")
		}
	})

	t.Run("returns error when TMUX_PANE is not set", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "")

		resolver := &mockKeyResolver{key: "unus00"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "set", "--on-resume", "some-cmd"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must be run from inside a tmux pane") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "must be run from inside a tmux pane")
		}

		if _, statErr := os.Stat(hooksFile); statErr == nil {
			t.Error("hooks file should not have been created")
		}
	})

	t.Run("returns error when on-resume flag is not provided", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "aaa111"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "set"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error for missing --on-resume flag, got nil")
		}
		if !strings.Contains(err.Error(), "on-resume") {
			t.Errorf("error = %q, want it to mention %q", err.Error(), "on-resume")
		}
	})

	t.Run("overwrites existing hook for same pane idempotently", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "aaa111"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "set", "--on-resume", "old-cmd"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("first set: unexpected error: %v", err)
		}

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "set", "--on-resume", "new-cmd"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("second set: unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if data["aaa111"]["on-resume"] != "new-cmd" {
			t.Errorf("hook command = %q, want %q", data["aaa111"]["on-resume"], "new-cmd")
		}
	})

	t.Run("writes correct JSON structure to hooks file", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "aaa111"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "set", "--on-resume", "claude --resume abc123"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)

		if len(data) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(data))
		}

		events, ok := data["aaa111"]
		if !ok {
			t.Fatal("expected entry for hook key aaa111")
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 event for aaa111, got %d", len(events))
		}
		if events["on-resume"] != "claude --resume abc123" {
			t.Errorf("on-resume = %q, want %q", events["on-resume"], "claude --resume abc123")
		}
	})

	t.Run("it aborts hooks set when the hook-key read fails", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{err: fmt.Errorf("tmux not responding")}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "set", "--on-resume", "some-cmd"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "resolve") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "resolve")
		}

		if _, statErr := os.Stat(hooksFile); statErr == nil {
			t.Error("hooks file should not have been created")
		}
	})

	t.Run("it stores the hook under the resolved hook key", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "tok123"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "set", "--on-resume", "claude --resume abc123"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if data["tok123"]["on-resume"] != "claude --resume abc123" {
			t.Errorf("hook command = %q, want %q", data["tok123"]["on-resume"], "claude --resume abc123")
		}
	})

	t.Run("it errors when TMUX_PANE is unset for set", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "")

		resolver := &mockKeyResolver{key: "unus00"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "set", "--on-resume", "some-cmd"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must be run from inside a tmux pane") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "must be run from inside a tmux pane")
		}
	})

}

func TestHooksRmCommand(t *testing.T) {
	t.Run("removes hook for current pane", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%3")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111": {"on-resume": "claude --resume abc123"},
		})

		resolver := &mockKeyResolver{key: "aaa111"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "rm", "--on-resume"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data["aaa111"]; ok {
			t.Error("expected hook key aaa111 entry to be removed from hooks file")
		}
	})

	t.Run("reads pane ID from TMUX_PANE and resolves hook key", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%42")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"bbb222": {"on-resume": "some-cmd"},
		})

		resolver := &mockKeyResolver{key: "bbb222"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "rm", "--on-resume"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data["bbb222"]; ok {
			t.Error("expected hook key bbb222 entry to be removed")
		}
		if _, ok := data["%42"]; ok {
			t.Error("raw pane ID %42 should not be used as key")
		}
	})

	t.Run("returns error when TMUX_PANE is not set", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "")

		resolver := &mockKeyResolver{key: "unus00"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "rm", "--on-resume"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must be run from inside a tmux pane") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "must be run from inside a tmux pane")
		}
	})

	t.Run("returns error when on-resume flag is not provided", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "aaa111"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "rm"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error for missing --on-resume flag, got nil")
		}
		if !strings.Contains(err.Error(), "on-resume") {
			t.Errorf("error = %q, want it to mention %q", err.Error(), "on-resume")
		}
	})

	t.Run("silent no-op when no hook exists for pane", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%99")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{})

		resolver := &mockKeyResolver{key: "ccc333"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		buf := new(bytes.Buffer)
		resetRootCmd()
		rootCmd.SetOut(buf)
		rootCmd.SetArgs([]string{"hooks", "rm", "--on-resume"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("expected no error for non-existent hook, got: %v", err)
		}

		if buf.String() != "" {
			t.Errorf("output = %q, want empty string", buf.String())
		}
	})

	t.Run("removes correct JSON entry from hooks file", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%3")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111":         {"on-resume": "claude --resume abc123"},
			"other-proj:0.0": {"on-resume": "npm start"},
		})

		resolver := &mockKeyResolver{key: "aaa111"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "rm", "--on-resume"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)

		if _, ok := data["aaa111"]; ok {
			t.Error("expected hook key aaa111 to be removed")
		}

		if data["other-proj:0.0"]["on-resume"] != "npm start" {
			t.Errorf("other-proj:0.0 on-resume = %q, want %q", data["other-proj:0.0"]["on-resume"], "npm start")
		}
	})

	t.Run("cleans up pane key when last event removed", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%5")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111": {"on-resume": "some-cmd"},
		})

		resolver := &mockKeyResolver{key: "aaa111"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "rm", "--on-resume"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data["aaa111"]; ok {
			t.Error("expected hook key aaa111 to be removed when last event deleted")
		}
		if len(data) != 0 {
			t.Errorf("expected empty hooks file, got %d entries", len(data))
		}
	})

	t.Run("it aborts hooks rm when the hook-key read fails and leaves the entry intact", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%3")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111": {"on-resume": "some-cmd"},
		})

		resolver := &mockKeyResolver{err: fmt.Errorf("tmux not responding")}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "rm", "--on-resume"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "resolve") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "resolve")
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data["aaa111"]; !ok {
			t.Error("hook should not have been removed on resolver failure")
		}
	})

	t.Run("--pane-key flag removes specified key without requiring TMUX_PANE", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"eee555":         {"on-resume": "claude --resume xyz"},
			"other-proj:0.0": {"on-resume": "npm start"},
		})

		// A resolver that fails loudly if consulted, so an accidental fallback on
		// the flag-set branch cannot pass silently.
		resolver := &mockKeyResolver{err: fmt.Errorf("resolver must not be called when --pane-key is set")}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "rm", "--pane-key", "eee555", "--on-resume"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data["eee555"]; ok {
			t.Error("expected eee555 entry to be removed via --pane-key")
		}
		if data["other-proj:0.0"]["on-resume"] != "npm start" {
			t.Errorf("other-proj:0.0 on-resume = %q, want %q", data["other-proj:0.0"]["on-resume"], "npm start")
		}
	})

	t.Run("--pane-key unset falls back to resolveCurrentPaneKey", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%7")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"ddd444": {"on-resume": "some-cmd"},
		})

		resolver := &mockKeyResolver{key: "ddd444"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "rm", "--on-resume"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data["ddd444"]; ok {
			t.Error("expected ddd444 entry to be removed via fallback")
		}
	})

	t.Run("it removes the verbatim key on rm --pane-key without consulting the resolver", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"eee555":         {"on-resume": "claude --resume xyz"},
			"other-proj:0.0": {"on-resume": "npm start"},
		})

		// A resolver that fails loudly if consulted, so an accidental fallback on
		// the --pane-key branch cannot pass silently.
		resolver := &mockKeyResolver{err: fmt.Errorf("resolver must not be called when --pane-key is set")}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "rm", "--pane-key", "eee555", "--on-resume"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data["eee555"]; ok {
			t.Error("expected verbatim key eee555 to be removed via --pane-key")
		}
		if data["other-proj:0.0"]["on-resume"] != "npm start" {
			t.Errorf("other-proj:0.0 on-resume = %q, want %q", data["other-proj:0.0"]["on-resume"], "npm start")
		}
	})

	t.Run("it errors when TMUX_PANE is unset for the rm fallback", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "")

		resolver := &mockKeyResolver{key: "unus00"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "rm", "--on-resume"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must be run from inside a tmux pane") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "must be run from inside a tmux pane")
		}
	})
}

func TestHookCommandRename(t *testing.T) {
	t.Run("exposes hook as the canonical command name", func(t *testing.T) {
		if hookCmd.Name() != "hook" {
			t.Errorf("hookCmd.Name() = %q, want %q", hookCmd.Name(), "hook")
		}
	})

	t.Run("declares hooks as an alias of hook", func(t *testing.T) {
		found := false
		for _, a := range hookCmd.Aliases {
			if a == "hooks" {
				found = true
			}
		}
		if !found {
			t.Errorf("hookCmd.Aliases = %v, want to contain %q", hookCmd.Aliases, "hooks")
		}
	})

	t.Run("resolves the hooks alias to the hook subcommands", func(t *testing.T) {
		for _, sub := range []string{"list", "set", "rm"} {
			c, _, err := rootCmd.Find([]string{"hooks", sub})
			if err != nil {
				t.Fatalf("Find([hooks %s]) error: %v", sub, err)
			}
			if c.Name() != sub {
				t.Errorf("Find([hooks %s]) resolved to %q, want %q", sub, c.Name(), sub)
			}
			if c.Parent() == nil || c.Parent().Name() != "hook" {
				t.Errorf("Find([hooks %s]) parent = %v, want parent named hook", sub, c.Parent())
			}
		}
	})

	t.Run("keeps hooks working as a silent cobra alias", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{})

		outBuf := new(bytes.Buffer)
		errBuf := new(bytes.Buffer)
		resetRootCmd()
		rootCmd.SetOut(outBuf)
		rootCmd.SetErr(errBuf)
		rootCmd.SetArgs([]string{"hooks", "list"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		combined := strings.ToLower(outBuf.String() + errBuf.String())
		if strings.Contains(combined, "deprecat") {
			t.Errorf("hooks alias produced deprecation text: out=%q err=%q", outBuf.String(), errBuf.String())
		}
		if strings.Contains(combined, "warning") {
			t.Errorf("hooks alias produced warning text: out=%q err=%q", outBuf.String(), errBuf.String())
		}
	})

	t.Run("machine-generated hooks set still persists via the alias", func(t *testing.T) {
		dir := t.TempDir()
		hooksFile := filepath.Join(dir, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "aaa111"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "set", "--on-resume", "claude --resume abc123"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if data["aaa111"]["on-resume"] != "claude --resume abc123" {
			t.Errorf("hook command = %q, want %q", data["aaa111"]["on-resume"], "claude --resume abc123")
		}
	})
}
