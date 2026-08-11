package project_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/project"
)

func TestAddTag(t *testing.T) {
	t.Run("it adds a normalised tag to a project and persists it", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		if err := store.Upsert("/code/portal", "portal", "internal"); err != nil {
			t.Fatalf("unexpected error on upsert: %v", err)
		}

		if err := store.AddTag("/code/portal", "  Work "); err != nil {
			t.Fatalf("unexpected error on AddTag: %v", err)
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		if len(projects) != 1 {
			t.Fatalf("got %d projects, want 1", len(projects))
		}
		want := []string{"Work"}
		if len(projects[0].Tags) != len(want) {
			t.Fatalf("Tags = %#v, want %#v", projects[0].Tags, want)
		}
		if projects[0].Tags[0] != "Work" {
			t.Errorf("Tags[0] = %q, want %q", projects[0].Tags[0], "Work")
		}
	})

	t.Run("it preserves internal whitespace through the store (trim edges, not collapse)", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		if err := store.Upsert("/code/portal", "portal", "internal"); err != nil {
			t.Fatalf("unexpected error on upsert: %v", err)
		}

		if err := store.AddTag("/code/portal", "  Code Review "); err != nil {
			t.Fatalf("unexpected error on AddTag: %v", err)
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		if len(projects) != 1 || len(projects[0].Tags) != 1 {
			t.Fatalf("got %d projects with tags %#v, want 1 project with 1 tag", len(projects), projects)
		}
		if projects[0].Tags[0] != "Code Review" {
			t.Errorf("Tags[0] = %q, want %q", projects[0].Tags[0], "Code Review")
		}
	})

	t.Run("it is a no-op when adding the exact same tag again (trimmed)", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		if err := store.Upsert("/code/portal", "portal", "internal"); err != nil {
			t.Fatalf("unexpected error on upsert: %v", err)
		}
		if err := store.AddTag("/code/portal", "work"); err != nil {
			t.Fatalf("unexpected error on first AddTag: %v", err)
		}

		infoBefore, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}

		if err := store.AddTag("/code/portal", "  work "); err != nil {
			t.Fatalf("unexpected error on duplicate AddTag: %v", err)
		}

		infoAfter, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
			t.Error("file was modified on a dedup no-op AddTag (Save should be skipped)")
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		want := []string{"work"}
		if len(projects[0].Tags) != len(want) {
			t.Fatalf("Tags = %#v, want %#v (no duplicate)", projects[0].Tags, want)
		}
	})

	t.Run("it treats case-variant tags as distinct (case-sensitive)", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		if err := store.Upsert("/code/portal", "portal", "internal"); err != nil {
			t.Fatalf("unexpected error on upsert: %v", err)
		}
		if err := store.AddTag("/code/portal", "work"); err != nil {
			t.Fatalf("unexpected error on first AddTag: %v", err)
		}
		if err := store.AddTag("/code/portal", "WORK"); err != nil {
			t.Fatalf("unexpected error on second AddTag: %v", err)
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		want := []string{"work", "WORK"}
		if len(projects[0].Tags) != len(want) || projects[0].Tags[0] != "work" || projects[0].Tags[1] != "WORK" {
			t.Errorf("Tags = %#v, want %#v (case-variant tags kept distinct)", projects[0].Tags, want)
		}
	})

	t.Run("it rejects a blank or whitespace-only add as a no-op", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		if err := store.Upsert("/code/portal", "portal", "internal"); err != nil {
			t.Fatalf("unexpected error on upsert: %v", err)
		}

		infoBefore, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}

		if err := store.AddTag("/code/portal", "   "); err != nil {
			t.Fatalf("unexpected error on blank AddTag: %v", err)
		}

		infoAfter, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
			t.Error("file was modified on a blank AddTag (Save should be skipped)")
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		if len(projects[0].Tags) != 0 {
			t.Errorf("Tags = %#v, want empty", projects[0].Tags)
		}
	})

	t.Run("it returns ErrProjectNotFound for an unknown path without writing", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		err := store.AddTag("/code/absent", "work")
		if !errors.Is(err, project.ErrProjectNotFound) {
			t.Fatalf("err = %v, want ErrProjectNotFound", err)
		}

		if _, statErr := os.Stat(filePath); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("projects.json was created on an unknown-path AddTag: stat err = %v", statErr)
		}
	})
}

func TestRemoveTag(t *testing.T) {
	t.Run("it removes a present tag and persists", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		if err := store.Upsert("/code/portal", "portal", "internal"); err != nil {
			t.Fatalf("unexpected error on upsert: %v", err)
		}
		if err := store.AddTag("/code/portal", "work"); err != nil {
			t.Fatalf("unexpected error on AddTag: %v", err)
		}

		if err := store.RemoveTag("/code/portal", "  work "); err != nil {
			t.Fatalf("unexpected error on RemoveTag: %v", err)
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		if len(projects[0].Tags) != 0 {
			t.Errorf("Tags = %#v, want empty after removal", projects[0].Tags)
		}
	})

	t.Run("it is a no-op when removing a case-variant of a present tag", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		if err := store.Upsert("/code/portal", "portal", "internal"); err != nil {
			t.Fatalf("unexpected error on upsert: %v", err)
		}
		if err := store.AddTag("/code/portal", "work"); err != nil {
			t.Fatalf("unexpected error on AddTag: %v", err)
		}

		if err := store.RemoveTag("/code/portal", "Work"); err != nil {
			t.Fatalf("unexpected error on case-variant RemoveTag: %v", err)
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		if len(projects[0].Tags) != 1 || projects[0].Tags[0] != "work" {
			t.Errorf("Tags = %#v, want [work] (case-variant removal must be a no-op)", projects[0].Tags)
		}
	})

	t.Run("it is a no-op when removing an absent tag", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		if err := store.Upsert("/code/portal", "portal", "internal"); err != nil {
			t.Fatalf("unexpected error on upsert: %v", err)
		}
		if err := store.AddTag("/code/portal", "work"); err != nil {
			t.Fatalf("unexpected error on AddTag: %v", err)
		}

		infoBefore, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}

		if err := store.RemoveTag("/code/portal", "personal"); err != nil {
			t.Fatalf("unexpected error on absent RemoveTag: %v", err)
		}

		infoAfter, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
			t.Error("file was modified on an absent-tag RemoveTag (Save should be skipped)")
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		want := []string{"work"}
		if len(projects[0].Tags) != len(want) {
			t.Fatalf("Tags = %#v, want %#v (unchanged)", projects[0].Tags, want)
		}
	})

	t.Run("it returns ErrProjectNotFound for an unknown path without writing", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		err := store.RemoveTag("/code/absent", "work")
		if !errors.Is(err, project.ErrProjectNotFound) {
			t.Fatalf("err = %v, want ErrProjectNotFound", err)
		}

		if _, statErr := os.Stat(filePath); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("projects.json was created on an unknown-path RemoveTag: stat err = %v", statErr)
		}
	})
}
