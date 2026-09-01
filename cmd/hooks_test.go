package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

func TestHooksListCommand(t *testing.T) {
	t.Run("outputs hooks in tab-separated format", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)

		// Stub bootstrap so the real orchestrator never runs against the test's
		// tmux server.
		withBootstrapDeps(t, BootstrapDeps{Orchestrator: &nopRunner{}})

		data := map[string]map[string]string{
			"aaa111": {"on-resume": "claude --resume abc123"},
		}
		writeHooksJSON(t, hooksFile, data)

		withHooksDeps(t, HooksDeps{PaneLister: &recordingPaneHookLister{rows: []tmux.PaneHookRow{
			{Token: "aaa111", Location: "my-project-abc123:0.0"},
		}}})

		got := runHookList(t)
		want := "aaa111\ton-resume\tclaude --resume abc123\tmy-project-abc123:0.0\n"
		if got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("produces empty output when no hooks registered", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)

		writeHooksJSON(t, hooksFile, map[string]map[string]string{})

		withHooksDeps(t, HooksDeps{PaneLister: &loudPaneHookLister{t: t}})

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
		hooksFileInTempDir(t)

		withHooksDeps(t, HooksDeps{PaneLister: &loudPaneHookLister{t: t}})

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

	t.Run("it keeps the sort by key then event", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)

		// Stub bootstrap so the real orchestrator never runs against the test's
		// tmux server.
		withBootstrapDeps(t, BootstrapDeps{Orchestrator: &nopRunner{}})

		data := map[string]map[string]string{
			"ccc333": {"on-resume": "claude --resume def456"},
			"bbb222": {"on-resume": "claude --resume abc123"},
			"aaa111": {"on-resume": "npm start"},
		}
		writeHooksJSON(t, hooksFile, data)

		withHooksDeps(t, HooksDeps{PaneLister: &recordingPaneHookLister{rows: []tmux.PaneHookRow{
			{Token: "ccc333", Location: "proj-abc:1.0"},
			{Token: "bbb222", Location: "proj-abc:0.0"},
			{Token: "aaa111", Location: "other-proj:0.0"},
		}}})

		got := runHookList(t)
		want := "aaa111\ton-resume\tnpm start\tother-proj:0.0\n" +
			"bbb222\ton-resume\tclaude --resume abc123\tproj-abc:0.0\n" +
			"ccc333\ton-resume\tclaude --resume def456\tproj-abc:1.0\n"
		if got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("accepts no arguments", func(t *testing.T) {
		hooksFileInTempDir(t)

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "list", "extraarg"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error for extra argument, got nil")
		}
	})
}

func TestHooksListLocationColumn(t *testing.T) {
	t.Run("it appends the resolved location as a fourth column", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111": {"on-resume": "claude --resume abc123"},
		})

		withHooksDeps(t, HooksDeps{PaneLister: &recordingPaneHookLister{rows: []tmux.PaneHookRow{
			{Token: "aaa111", Location: "my-project-abc123:0.0"},
		}}})

		got := runHookList(t)
		want := "aaa111\ton-resume\tclaude --resume abc123\tmy-project-abc123:0.0\n"
		if got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("it does not let an unstamped pane's row lend its location", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"": {"on-resume": "npm start"},
		})

		withHooksDeps(t, HooksDeps{PaneLister: &recordingPaneHookLister{rows: []tmux.PaneHookRow{
			{Token: "", Location: "unstamped-sess:0.0"},
		}}})

		got := runHookList(t)
		want := "\ton-resume\tnpm start\t\n"
		if got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("it renders one line for a token carried by two rows", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"dup777": {"on-resume": "npm start"},
		})

		withHooksDeps(t, HooksDeps{PaneLister: &recordingPaneHookLister{rows: []tmux.PaneHookRow{
			{Token: "dup777", Location: "first-sess:0.0"},
			{Token: "dup777", Location: "second-sess:1.2"},
		}}})

		got := runHookList(t)
		want := "dup777\ton-resume\tnpm start\tfirst-sess:0.0\n"
		if got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("it renders every fourth field empty when the enumeration fails", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111": {"on-resume": "claude --resume abc123"},
			"bbb222": {"on-resume": "npm start"},
		})

		// Rows alongside the error: a failed read must be judged by the error, not
		// by whether the lister also handed back rows.
		withHooksDeps(t, HooksDeps{PaneLister: &recordingPaneHookLister{
			rows: []tmux.PaneHookRow{{Token: "aaa111", Location: "my-project:0.0"}},
			err:  errors.New("no server running"),
		}})

		got := runHookList(t)
		want := "aaa111\ton-resume\tclaude --resume abc123\t\nbbb222\ton-resume\tnpm start\t\n"
		if got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("it takes no enumeration read when there are no entries", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{})

		withHooksDeps(t, HooksDeps{PaneLister: &loudPaneHookLister{t: t}})

		got := runHookList(t)
		if got != "" {
			t.Errorf("output = %q, want empty string", got)
		}
	})

	t.Run("it renders an empty fourth field for a token no row carries", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"ghost9": {"on-resume": "npm start"},
		})

		withHooksDeps(t, HooksDeps{PaneLister: &recordingPaneHookLister{rows: []tmux.PaneHookRow{
			{Token: "aaa111", Location: "my-project:0.0"},
			{Token: "bbb222", Location: "my-project:0.1"},
		}}})

		got := runHookList(t)
		want := "ghost9\ton-resume\tnpm start\t\n"
		if got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("it renders an empty fourth field for an old-format key", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111":   {"on-resume": "claude --resume abc123"},
			"sess:0.0": {"on-resume": "npm start"},
		})

		withHooksDeps(t, HooksDeps{PaneLister: &recordingPaneHookLister{rows: []tmux.PaneHookRow{
			{Token: "aaa111", Location: "sess:0.0"},
		}}})

		got := runHookList(t)
		want := "aaa111\ton-resume\tclaude --resume abc123\tsess:0.0\n" +
			"sess:0.0\ton-resume\tnpm start\t\n"
		if got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("it renders a location whose session name carries a pipe verbatim", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111": {"on-resume": "npm start"},
		})

		withHooksDeps(t, HooksDeps{PaneLister: &recordingPaneHookLister{rows: []tmux.PaneHookRow{
			{Token: "aaa111", Location: "a|b:0.0"},
		}}})

		got := runHookList(t)
		want := "aaa111\ton-resume\tnpm start\ta|b:0.0\n"
		if got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("it takes exactly one enumeration read for many entries", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111": {"on-resume": "one"},
			"bbb222": {"on-resume": "two"},
			"ccc333": {"on-resume": "three"},
			"ddd444": {"on-resume": "four"},
		})

		lister := &recordingPaneHookLister{rows: []tmux.PaneHookRow{
			{Token: "aaa111", Location: "my-project:0.0"},
		}}
		withHooksDeps(t, HooksDeps{PaneLister: lister})

		runHookList(t)

		if lister.calls != 1 {
			t.Errorf("enumeration reads = %d, want 1", lister.calls)
		}
	})
}

// loudPaneHookLister fails the test the moment it is read from: nothing to
// resolve must mean no tmux read at all.
type loudPaneHookLister struct{ t *testing.T }

func (l *loudPaneHookLister) ListAllPaneHookKeys() ([]tmux.PaneHookRow, error) {
	l.t.Helper()
	l.t.Error("enumeration read taken with no entries to resolve")
	return nil, nil
}

func TestHooksSetCommand(t *testing.T) {
	t.Run("sets hook for current pane", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "aaa111"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%99")

		resolver := &mockKeyResolver{key: "bbb222"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "")

		resolver := &mockKeyResolver{key: "unus00"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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
		hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "aaa111"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "aaa111"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "aaa111"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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

	t.Run("it aborts hook set when the hook-key read fails", func(t *testing.T) {
		const stderr = "no such pane: %999"

		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%999")

		resolver := &mockKeyResolver{err: &tmux.CommandError{Stderr: stderr}}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver, PaneStamper: &recordingPaneStamper{}})

		_, err := runHookSet(t, "some-cmd")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != stderr {
			t.Errorf("error = %q, want tmux's own words %q unaltered", err.Error(), stderr)
		}
		if _, ok := errors.AsType[*tmux.CommandError](err); !ok {
			t.Errorf("error %v is not a recoverable *tmux.CommandError (errors.As failed)", err)
		}

		if _, statErr := os.Stat(hooksFile); statErr == nil {
			t.Error("hooks file should not have been created")
		}
	})

	t.Run("it stores the hook under the resolved hook key", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "tok123"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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
}

func TestHooksRmCommand(t *testing.T) {
	t.Run("removes hook for current pane", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111": {"on-resume": "claude --resume abc123"},
		})

		resolver := &mockKeyResolver{key: "aaa111"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%42")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"bbb222": {"on-resume": "some-cmd"},
		})

		resolver := &mockKeyResolver{key: "bbb222"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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
		hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "")

		resolver := &mockKeyResolver{key: "unus00"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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
		hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "aaa111"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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

	t.Run("it exits non-zero when no hook exists for pane", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%99")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{})

		resolver := &mockKeyResolver{key: "ccc333"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

		buf := new(bytes.Buffer)
		resetRootCmd()
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		rootCmd.SetArgs([]string{"hooks", "rm", "--on-resume"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected an error when nothing was removed, got nil")
		}
		if err.Error() != "no resume hook registered for ccc333" {
			t.Errorf("error = %q, want %q", err.Error(), "no resume hook registered for ccc333")
		}

		if buf.String() != "" {
			t.Errorf("output = %q, want empty string", buf.String())
		}
	})

	t.Run("removes correct JSON entry from hooks file", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111":         {"on-resume": "claude --resume abc123"},
			"other-proj:0.0": {"on-resume": "npm start"},
		})

		resolver := &mockKeyResolver{key: "aaa111"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%5")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111": {"on-resume": "some-cmd"},
		})

		resolver := &mockKeyResolver{key: "aaa111"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111": {"on-resume": "some-cmd"},
		})

		resolver := &mockKeyResolver{err: fmt.Errorf("tmux not responding")}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hooks", "rm", "--on-resume"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "tmux not responding" {
			t.Errorf("error = %q, want the read's own words %q unaltered", err.Error(), "tmux not responding")
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data["aaa111"]; !ok {
			t.Error("hook should not have been removed on resolver failure")
		}
	})

	t.Run("--pane-key flag removes specified key without requiring TMUX_PANE", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"eee555":         {"on-resume": "claude --resume xyz"},
			"other-proj:0.0": {"on-resume": "npm start"},
		})

		resolver, stamper := paneKeyPathSeams()
		withHooksDeps(t, HooksDeps{KeyResolver: resolver, PaneStamper: stamper})

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
		assertNoPaneTmuxCalls(t, resolver, stamper)
	})

	t.Run("--pane-key unset falls back to resolveCurrentPaneKey", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%7")

		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"ddd444": {"on-resume": "some-cmd"},
		})

		resolver := &mockKeyResolver{key: "ddd444"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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
		_, hooksFile := hooksFileInTempDir(t)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{})

		withHooksDeps(t, HooksDeps{PaneLister: &loudPaneHookLister{t: t}})

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
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		resolver := &mockKeyResolver{key: "aaa111"}
		withHooksDeps(t, HooksDeps{KeyResolver: resolver})

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

// runHookSetForKey drives `hook set` with the pane resolver stubbed to answer key,
// so the command writes under it without a stamp.
func runHookSetForKey(t *testing.T, key, command string) error {
	t.Helper()

	t.Setenv("TMUX_PANE", "%3")
	withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: key}})

	_, err := runHookSet(t, command)
	return err
}

func saveRequestedExists(t *testing.T, stateDir string) bool {
	t.Helper()
	_, err := os.Stat(state.SaveRequested(stateDir))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("stat save.requested: %v", err)
	}
	return err == nil
}

// assertTouchWarn asserts the sink holds exactly one record at WARN or above and
// that it is the dirty-flag touch failure for wantKey.
func assertTouchWarn(t *testing.T, sink *logtest.Sink, wantKey string) {
	t.Helper()

	warns := sink.RecordsAtOrAboveLevel(slog.LevelWarn)
	if len(warns) != 1 {
		t.Fatalf("WARN record count = %d, want 1: %+v", len(warns), warns)
	}

	rec := warns[0]
	assertHooksRecord(t, rec, hooksRecordWant{
		level: slog.LevelWarn,
		msg:   "touch-save-requested",
		op:    "touch-save-requested",
		via:   "cli",
	})
	if got := rec.AttrString(t, "hook_key"); got != wantKey {
		t.Errorf("hook_key = %q, want %q", got, wantKey)
	}
	if got := rec.AttrString(t, "error"); got == "" {
		t.Error("error attr is empty, want the failure it carries")
	}
}

func TestHooksSetTouchesSaveRequested(t *testing.T) {
	t.Run("it touches save.requested after a successful registration", func(t *testing.T) {
		hooksFileInTempDir(t)
		stateDir := t.TempDir()
		t.Setenv("PORTAL_STATE_DIR", stateDir)

		if err := runHookSetForKey(t, "tok123", "claude --resume abc"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !saveRequestedExists(t, stateDir) {
			t.Error("save.requested was not created in the state directory")
		}
	})

	t.Run("it exits 0 when the state directory cannot be resolved", func(t *testing.T) {
		dir, hooksFile := hooksFileInTempDir(t)

		blocker := filepath.Join(dir, "not-a-dir")
		if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
			t.Fatalf("write blocker file: %v", err)
		}
		t.Setenv("PORTAL_STATE_DIR", filepath.Join(blocker, "state"))

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		if err := runHookSetForKey(t, "tok123", "claude --resume abc"); err != nil {
			t.Fatalf("hook set failed on an unresolvable state directory: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if data["tok123"]["on-resume"] != "claude --resume abc" {
			t.Errorf("hook command = %q, want %q", data["tok123"]["on-resume"], "claude --resume abc")
		}
		assertTouchWarn(t, sink, "tok123")
	})

	t.Run("it exits 0 when the touch itself fails", func(t *testing.T) {
		dir, hooksFile := hooksFileInTempDir(t)

		stateDir := filepath.Join(dir, "state")
		if err := os.MkdirAll(filepath.Join(stateDir, "scrollback"), 0o700); err != nil {
			t.Fatalf("mkdir state dir: %v", err)
		}
		if err := os.Chmod(stateDir, 0o500); err != nil {
			t.Fatalf("chmod state dir: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(stateDir, 0o700) })
		t.Setenv("PORTAL_STATE_DIR", stateDir)

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		if err := runHookSetForKey(t, "tok123", "claude --resume abc"); err != nil {
			t.Fatalf("hook set failed on a failing touch: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if data["tok123"]["on-resume"] != "claude --resume abc" {
			t.Errorf("hook command = %q, want %q", data["tok123"]["on-resume"], "claude --resume abc")
		}
		assertTouchWarn(t, sink, "tok123")
	})

	t.Run("it emits no set WARN when only the dirty-flag touch fails", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("PORTAL_STATE_DIR", filepath.Join(hooksFile, "state"))

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		if err := runHookSetForKey(t, "tok123", "claude --resume abc"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, r := range sink.RecordsAtOrAboveLevel(slog.LevelWarn) {
			if r.HasAttr("op") && r.AttrString(t, "op") == "set" {
				t.Errorf("a set WARN was emitted for a registration that succeeded: %+v", r)
			}
		}
	})

	t.Run("it emits exactly one warn per failing hook set", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("PORTAL_STATE_DIR", filepath.Join(hooksFile, "state"))

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		if err := runHookSetForKey(t, "tok123", "claude --resume abc"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if warns := sink.RecordsAtOrAboveLevel(slog.LevelWarn); len(warns) != 1 {
			t.Errorf("WARN record count = %d, want 1: %+v", len(warns), warns)
		}
	})

	t.Run("it does not touch when the write fails", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		// A directory at the hooks.json path makes the store's own read fail, so
		// the command aborts before it could touch anything.
		if err := os.MkdirAll(hooksFile, 0o700); err != nil {
			t.Fatalf("mkdir bogus hooks path: %v", err)
		}
		stateDir := t.TempDir()
		t.Setenv("PORTAL_STATE_DIR", stateDir)

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		if err := runHookSetForKey(t, "tok123", "claude --resume abc"); err == nil {
			t.Fatal("expected an error from a failed write, got nil")
		}

		if saveRequestedExists(t, stateDir) {
			t.Error("save.requested was touched despite the write failing")
		}
		for _, r := range sink.Records() {
			if r.Msg == "touch-save-requested" {
				t.Errorf("touch emission on a failed write: %+v", r)
			}
		}
	})

	t.Run("it touches on a no-op re-registration", func(t *testing.T) {
		hooksFileInTempDir(t)
		stateDir := t.TempDir()
		t.Setenv("PORTAL_STATE_DIR", stateDir)

		if err := runHookSetForKey(t, "tok123", "same-cmd"); err != nil {
			t.Fatalf("first set: unexpected error: %v", err)
		}
		if err := os.Remove(state.SaveRequested(stateDir)); err != nil {
			t.Fatalf("remove save.requested: %v", err)
		}

		if err := runHookSetForKey(t, "tok123", "same-cmd"); err != nil {
			t.Fatalf("second set: unexpected error: %v", err)
		}

		if !saveRequestedExists(t, stateDir) {
			t.Error("a no-op re-registration did not touch save.requested")
		}
	})

	t.Run("it does not touch from hook rm", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"tok123": {"on-resume": "claude --resume abc"},
		})
		stateDir := t.TempDir()
		t.Setenv("PORTAL_STATE_DIR", stateDir)
		t.Setenv("TMUX_PANE", "%3")

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: "tok123"}})

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hook", "rm", "--on-resume"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if saveRequestedExists(t, stateDir) {
			t.Error("hook rm touched save.requested")
		}
	})
}
