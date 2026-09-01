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

// sessionIDPrefix leads a tmux session ID. A generated name opens with the
// sanitised project fragment, so a fragment carrying this prefix would mint a
// name tmux resolves as an ID instead of a name — unaddressable either way.
// Only a leading occurrence carries that meaning.
const sessionIDPrefix = "$"

// SanitiseProjectName replaces the characters that would leave a generated
// session name untidy or unaddressable — periods, colons and a leading dollar —
// with hyphens.
func SanitiseProjectName(name string) string {
	r := strings.NewReplacer(".", "-", ":", "-")
	sanitised := r.Replace(name)

	if rest, found := strings.CutPrefix(sanitised, sessionIDPrefix); found {
		return "-" + rest
	}
	return sanitised
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
