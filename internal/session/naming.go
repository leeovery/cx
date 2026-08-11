// Package session provides session management utilities for Portal.
package session

import (
	"crypto/rand"
	"fmt"
	"strings"
)

const (
	maxRetries = 10
	suffixLen  = 6
)

// NanoIDAlphabet is the shared option-name-safe charset for generated ids:
// letters and digits only. The absence of "-" is load-bearing — it keeps the
// "<batch>-<token>" spawn-marker split unambiguous.
const NanoIDAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// IDGenerator produces a random string suitable for use as a session name suffix.
type IDGenerator func() (string, error)

// ExistsFunc reports whether a tmux session with the given name already exists.
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

// NewNanoIDGenerator returns an IDGenerator producing 6-character alphanumeric
// strings from crypto/rand.
func NewNanoIDGenerator() IDGenerator {
	return func() (string, error) {
		bytes := make([]byte, suffixLen)
		if _, err := rand.Read(bytes); err != nil {
			return "", fmt.Errorf("failed to read random bytes: %w", err)
		}
		for i := range bytes {
			bytes[i] = NanoIDAlphabet[int(bytes[i])%len(NanoIDAlphabet)]
		}
		return string(bytes), nil
	}
}
