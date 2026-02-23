package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAliasSetCommand(t *testing.T) {
	t.Run("sets new alias with absolute path", func(t *testing.T) {
		dir := t.TempDir()
		aliasFile := filepath.Join(dir, "aliases")
		t.Setenv("PORTAL_ALIASES_FILE", aliasFile)

		resetRootCmd()
		rootCmd.SetArgs([]string{"alias", "set", "myproject", "/Users/lee/Code/project"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(aliasFile)
		if err != nil {
			t.Fatalf("failed to read aliases file: %v", err)
		}

		got := string(data)
		want := "myproject=/Users/lee/Code/project\n"
		if got != want {
			t.Errorf("aliases file content = %q, want %q", got, want)
		}
	})

	t.Run("expands tilde in path", func(t *testing.T) {
		dir := t.TempDir()
		aliasFile := filepath.Join(dir, "aliases")
		t.Setenv("PORTAL_ALIASES_FILE", aliasFile)

		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("failed to get home dir: %v", err)
		}

		resetRootCmd()
		rootCmd.SetArgs([]string{"alias", "set", "m2api", "~/Code/mac2/api"})
		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(aliasFile)
		if err != nil {
			t.Fatalf("failed to read aliases file: %v", err)
		}

		got := string(data)
		want := "m2api=" + filepath.Join(home, "Code/mac2/api") + "\n"
		if got != want {
			t.Errorf("aliases file content = %q, want %q", got, want)
		}
	})

	t.Run("resolves relative path to absolute", func(t *testing.T) {
		dir := t.TempDir()
		aliasFile := filepath.Join(dir, "aliases")
		t.Setenv("PORTAL_ALIASES_FILE", aliasFile)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get cwd: %v", err)
		}

		resetRootCmd()
		rootCmd.SetArgs([]string{"alias", "set", "proj", "relative/path"})
		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(aliasFile)
		if err != nil {
			t.Fatalf("failed to read aliases file: %v", err)
		}

		got := string(data)
		want := "proj=" + filepath.Join(cwd, "relative/path") + "\n"
		if got != want {
			t.Errorf("aliases file content = %q, want %q", got, want)
		}
	})

	t.Run("overwrites existing alias silently", func(t *testing.T) {
		dir := t.TempDir()
		aliasFile := filepath.Join(dir, "aliases")
		t.Setenv("PORTAL_ALIASES_FILE", aliasFile)

		// Set initial alias
		resetRootCmd()
		rootCmd.SetArgs([]string{"alias", "set", "proj", "/first/path"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error on first set: %v", err)
		}

		// Overwrite with new path
		buf := new(bytes.Buffer)
		resetRootCmd()
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		rootCmd.SetArgs([]string{"alias", "set", "proj", "/second/path"})
		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error on overwrite: %v", err)
		}

		data, err := os.ReadFile(aliasFile)
		if err != nil {
			t.Fatalf("failed to read aliases file: %v", err)
		}

		got := string(data)
		want := "proj=/second/path\n"
		if got != want {
			t.Errorf("aliases file content = %q, want %q", got, want)
		}
	})

	t.Run("aliases file contains absolute path after set", func(t *testing.T) {
		dir := t.TempDir()
		aliasFile := filepath.Join(dir, "aliases")
		t.Setenv("PORTAL_ALIASES_FILE", aliasFile)

		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("failed to get home dir: %v", err)
		}

		resetRootCmd()
		rootCmd.SetArgs([]string{"alias", "set", "work", "~/Code/work"})
		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(aliasFile)
		if err != nil {
			t.Fatalf("failed to read aliases file: %v", err)
		}

		got := string(data)
		want := "work=" + filepath.Join(home, "Code/work") + "\n"
		if got != want {
			t.Errorf("aliases file content = %q, want %q", got, want)
		}
		if !filepath.IsAbs(filepath.Join(home, "Code/work")) {
			t.Errorf("resolved path is not absolute")
		}
	})

	t.Run("exits 0 on success", func(t *testing.T) {
		dir := t.TempDir()
		aliasFile := filepath.Join(dir, "aliases")
		t.Setenv("PORTAL_ALIASES_FILE", aliasFile)

		resetRootCmd()
		rootCmd.SetArgs([]string{"alias", "set", "test", "/some/path"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("expected exit 0 (no error), got: %v", err)
		}
	})

	t.Run("requires exactly two arguments", func(t *testing.T) {
		dir := t.TempDir()
		aliasFile := filepath.Join(dir, "aliases")
		t.Setenv("PORTAL_ALIASES_FILE", aliasFile)

		resetRootCmd()
		rootCmd.SetArgs([]string{"alias", "set", "onlyname"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected error for missing path argument, got nil")
		}
	})
}
