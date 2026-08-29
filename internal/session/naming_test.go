package session_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/leeovery/portal/internal/session"
)

func TestSanitiseProjectName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "replaces periods with hyphens",
			input: "my.app",
			want:  "my-app",
		},
		{
			name:  "replaces colons with hyphens",
			input: "my:app",
			want:  "my-app",
		},
		{
			name:  "replaces multiple periods and colons",
			input: "my.cool:app.v2",
			want:  "my-cool-app-v2",
		},
		{
			name:  "leaves clean name unchanged",
			input: "portal",
			want:  "portal",
		},
		{
			name:  "handles empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := session.SanitiseProjectName(tt.input)
			if got != tt.want {
				t.Errorf("SanitiseProjectName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateSessionName(t *testing.T) {
	validNamePattern := regexp.MustCompile(`^.+-[a-zA-Z0-9]{6}$`)

	t.Run("generates name in correct format", func(t *testing.T) {
		gen := func() (string, error) { return "abc123", nil }
		exists := func(name string) bool { return false }

		got, err := session.GenerateSessionName("portal", gen, exists)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "portal-abc123" {
			t.Errorf("got %q, want %q", got, "portal-abc123")
		}

		if !validNamePattern.MatchString(got) {
			t.Errorf("name %q does not match pattern {project}-[a-zA-Z0-9]{6}", got)
		}
	})

	t.Run("sanitises periods in project name", func(t *testing.T) {
		gen := func() (string, error) { return "abc123", nil }
		exists := func(name string) bool { return false }

		got, err := session.GenerateSessionName("my.app", gen, exists)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "my-app-abc123" {
			t.Errorf("got %q, want %q", got, "my-app-abc123")
		}
	})

	t.Run("sanitises colons in project name", func(t *testing.T) {
		gen := func() (string, error) { return "abc123", nil }
		exists := func(name string) bool { return false }

		got, err := session.GenerateSessionName("my:app", gen, exists)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "my-app-abc123" {
			t.Errorf("got %q, want %q", got, "my-app-abc123")
		}
	})

	t.Run("retries on collision", func(t *testing.T) {
		callCount := 0
		gen := func() (string, error) {
			callCount++
			if callCount == 1 {
				return "aaaaaa", nil
			}
			return "bbbbbb", nil
		}
		exists := func(name string) bool {
			return name == "portal-aaaaaa"
		}

		got, err := session.GenerateSessionName("portal", gen, exists)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "portal-bbbbbb" {
			t.Errorf("got %q, want %q", got, "portal-bbbbbb")
		}

		if callCount != 2 {
			t.Errorf("expected 2 generator calls, got %d", callCount)
		}
	})

	t.Run("returns error after max retries", func(t *testing.T) {
		gen := func() (string, error) { return "aaaaaa", nil }
		exists := func(name string) bool { return true }

		_, err := session.GenerateSessionName("portal", gen, exists)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		want := "failed to generate unique session name after 10 attempts"
		if err.Error() != want {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	})

	t.Run("handles empty project name", func(t *testing.T) {
		gen := func() (string, error) { return "abc123", nil }
		exists := func(name string) bool { return false }

		got, err := session.GenerateSessionName("", gen, exists)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "-abc123" {
			t.Errorf("got %q, want %q", got, "-abc123")
		}
	})

	t.Run("returns error when generator fails", func(t *testing.T) {
		gen := func() (string, error) { return "", fmt.Errorf("random source exhausted") }
		exists := func(name string) bool { return false }

		_, err := session.GenerateSessionName("portal", gen, exists)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
