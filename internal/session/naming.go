package session

import (
	"fmt"
	"strings"

	"github.com/leeovery/portal/internal/nanoid"
)

const maxRetries = 10

// IDGenerator mints one opaque nanoid per call; a generated session name ends
// in one.
type IDGenerator = nanoid.Generator

type ExistsFunc func(name string) bool

// SanitiseProjectName replaces the characters tmux rejects in a session name —
// periods and colons — with hyphens.
func SanitiseProjectName(name string) string {
	r := strings.NewReplacer(".", "-", ":", "-")
	return r.Replace(name)
}

// GenerateSessionName produces a session name of the form {project}-{nanoid},
// retrying a bounded number of times on collision with an existing session.
func GenerateSessionName(projectName string, gen IDGenerator, exists ExistsFunc) (string, error) {
	sanitised := SanitiseProjectName(projectName)

	for range maxRetries {
		suffix, err := gen()
		if err != nil {
			return "", fmt.Errorf("failed to generate session ID: %w", err)
		}

		candidate := sanitised + "-" + suffix
		if !exists(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique session name after %d attempts", maxRetries)
}
