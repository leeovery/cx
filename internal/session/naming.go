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

// unwritableLeadingChars are the characters a session name may not open with:
// tmux resolves a leading "$" as a session ID rather than as a name, and parses
// a leading "-" as a command flag. A generated name opens with the sanitised
// project fragment, so a fragment leading with either would mint a name that
// cannot be handed back.
// They are dropped rather than substituted, so one unwritable character never
// escapes to another.
const unwritableLeadingChars = "$-"

// SanitiseProjectName reduces a project name to a fragment a generated session
// name can open with: periods and colons become hyphens, and a leading
// character a session name may not open with is dropped. The result may be
// empty.
func SanitiseProjectName(name string) string {
	r := strings.NewReplacer(".", "-", ":", "-")

	return strings.TrimLeft(r.Replace(name), unwritableLeadingChars)
}

// GenerateSessionName produces a session name of the form {project}-{nanoid} —
// the nanoid alone when the project name sanitises to nothing — retrying a
// bounded number of times on collision with an existing session.
func GenerateSessionName(projectName string, gen IDGenerator, exists ExistsFunc) (string, error) {
	sanitised := SanitiseProjectName(projectName)

	for range maxRetries {
		suffix, err := gen()
		if err != nil {
			return "", fmt.Errorf("failed to generate session ID: %w", err)
		}

		// An empty fragment contributes no separator either: a leading hyphen
		// is exactly what tmux refuses.
		candidate := suffix
		if sanitised != "" {
			candidate = sanitised + "-" + suffix
		}
		if !exists(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique session name after %d attempts", maxRetries)
}
