package project_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/project"
)

func TestLoad(t *testing.T) {
	t.Run("returns empty list when file does not exist", func(t *testing.T) {
		dir := t.TempDir()
		store := project.NewStore(filepath.Join(dir, "nonexistent", "projects.json"))

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(projects) != 0 {
			t.Errorf("got %d projects, want 0", len(projects))
		}
	})

	t.Run("loads projects from valid JSON", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")

		lastUsed := time.Date(2026, 1, 22, 10, 30, 0, 0, time.UTC)
		content := `{"projects":[{"path":"/Users/lee/Code/myapp","name":"myapp","last_used":"2026-01-22T10:30:00Z"}]}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := project.NewStore(filePath)
		projects, err := store.Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(projects) != 1 {
			t.Fatalf("got %d projects, want 1", len(projects))
		}

		p := projects[0]
		if p.Path != "/Users/lee/Code/myapp" {
			t.Errorf("Path = %q, want %q", p.Path, "/Users/lee/Code/myapp")
		}
		if p.Name != "myapp" {
			t.Errorf("Name = %q, want %q", p.Name, "myapp")
		}
		if !p.LastUsed.Equal(lastUsed) {
			t.Errorf("LastUsed = %v, want %v", p.LastUsed, lastUsed)
		}
	})

	t.Run("handles malformed JSON", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")

		if err := os.WriteFile(filePath, []byte("{invalid json!!!"), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := project.NewStore(filePath)
		projects, err := store.Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(projects) != 0 {
			t.Errorf("got %d projects, want 0", len(projects))
		}
	})
}

func TestSave(t *testing.T) {
	t.Run("creates config directory on save", func(t *testing.T) {
		dir := t.TempDir()
		nested := filepath.Join(dir, "portal", "sub")
		filePath := filepath.Join(nested, "projects.json")
		store := project.NewStore(filePath)

		projects := []project.Project{
			{
				Path:     "/Users/lee/Code/myapp",
				Name:     "myapp",
				LastUsed: time.Date(2026, 1, 22, 10, 30, 0, 0, time.UTC),
			},
		}

		if err := store.Save(projects); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify the directory was created
		info, err := os.Stat(nested)
		if err != nil {
			t.Fatalf("directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Errorf("expected directory, got file")
		}

		// Verify the file is readable and contains correct data
		loaded, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load saved file: %v", err)
		}
		if len(loaded) != 1 {
			t.Fatalf("got %d projects, want 1", len(loaded))
		}
		if loaded[0].Path != "/Users/lee/Code/myapp" {
			t.Errorf("Path = %q, want %q", loaded[0].Path, "/Users/lee/Code/myapp")
		}
		if loaded[0].Name != "myapp" {
			t.Errorf("Name = %q, want %q", loaded[0].Name, "myapp")
		}
		if !loaded[0].LastUsed.Equal(time.Date(2026, 1, 22, 10, 30, 0, 0, time.UTC)) {
			t.Errorf("LastUsed = %v, want %v", loaded[0].LastUsed, time.Date(2026, 1, 22, 10, 30, 0, 0, time.UTC))
		}
	})
}

func TestUpsert(t *testing.T) {
	t.Run("adds new project to empty store", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		if err := store.Upsert("/Users/lee/Code/myapp", "myapp"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(projects) != 1 {
			t.Fatalf("got %d projects, want 1", len(projects))
		}

		if projects[0].Path != "/Users/lee/Code/myapp" {
			t.Errorf("Path = %q, want %q", projects[0].Path, "/Users/lee/Code/myapp")
		}
		if projects[0].Name != "myapp" {
			t.Errorf("Name = %q, want %q", projects[0].Name, "myapp")
		}
		if projects[0].LastUsed.IsZero() {
			t.Errorf("LastUsed should not be zero")
		}
	})

	t.Run("updates existing project by path", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		// Add initial project
		if err := store.Upsert("/Users/lee/Code/myapp", "myapp"); err != nil {
			t.Fatalf("unexpected error on first upsert: %v", err)
		}

		// Record the first timestamp
		firstLoad, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		firstLastUsed := firstLoad[0].LastUsed

		// Wait a tiny bit so time advances
		time.Sleep(10 * time.Millisecond)

		// Upsert with same path but different name
		if err := store.Upsert("/Users/lee/Code/myapp", "renamed-app"); err != nil {
			t.Fatalf("unexpected error on second upsert: %v", err)
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(projects) != 1 {
			t.Fatalf("got %d projects, want 1 (should update, not add)", len(projects))
		}

		if projects[0].Name != "renamed-app" {
			t.Errorf("Name = %q, want %q", projects[0].Name, "renamed-app")
		}
		if !projects[0].LastUsed.After(firstLastUsed) {
			t.Errorf("LastUsed should be updated: got %v, first was %v", projects[0].LastUsed, firstLastUsed)
		}
	})

	t.Run("adds second project without replacing first", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")
		store := project.NewStore(filePath)

		if err := store.Upsert("/Users/lee/Code/myapp", "myapp"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := store.Upsert("/Users/lee/Code/other", "other"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(projects) != 2 {
			t.Fatalf("got %d projects, want 2", len(projects))
		}
	})
}

func TestList(t *testing.T) {
	t.Run("returns projects sorted by last_used descending", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")

		// Write projects in non-sorted order
		content := `{"projects":[
			{"path":"/a","name":"oldest","last_used":"2026-01-01T00:00:00Z"},
			{"path":"/c","name":"newest","last_used":"2026-03-01T00:00:00Z"},
			{"path":"/b","name":"middle","last_used":"2026-02-01T00:00:00Z"}
		]}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := project.NewStore(filePath)
		projects, err := store.List()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(projects) != 3 {
			t.Fatalf("got %d projects, want 3", len(projects))
		}

		wantOrder := []string{"newest", "middle", "oldest"}
		for i, want := range wantOrder {
			if projects[i].Name != want {
				t.Errorf("projects[%d].Name = %q, want %q", i, projects[i].Name, want)
			}
		}
	})

	t.Run("returns empty list when file missing", func(t *testing.T) {
		dir := t.TempDir()
		store := project.NewStore(filepath.Join(dir, "nonexistent", "projects.json"))

		projects, err := store.List()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(projects) != 0 {
			t.Errorf("got %d projects, want 0", len(projects))
		}
	})
}

func TestRemove(t *testing.T) {
	t.Run("removes project by path", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")

		content := `{"projects":[
			{"path":"/a","name":"first","last_used":"2026-01-01T00:00:00Z"},
			{"path":"/b","name":"second","last_used":"2026-02-01T00:00:00Z"}
		]}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := project.NewStore(filePath)

		if err := store.Remove("/a"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(projects) != 1 {
			t.Fatalf("got %d projects, want 1", len(projects))
		}

		if projects[0].Path != "/b" {
			t.Errorf("remaining project Path = %q, want %q", projects[0].Path, "/b")
		}
	})

	t.Run("no error when removing nonexistent path", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "projects.json")

		content := `{"projects":[{"path":"/a","name":"first","last_used":"2026-01-01T00:00:00Z"}]}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := project.NewStore(filePath)

		if err := store.Remove("/nonexistent"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		projects, err := store.Load()
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(projects) != 1 {
			t.Fatalf("got %d projects, want 1 (original should remain)", len(projects))
		}
	})
}
