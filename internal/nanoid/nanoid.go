// Package nanoid holds the vocabulary of Portal's generated ids: the charset,
// the width, the generator that mints them and the predicate that recognises
// one. It depends on the standard library alone, so packages that must not
// import each other — the hooks store and the session tree among them — can
// each reach the vocabulary without an edge between them.
package nanoid

import (
	"crypto/rand"
	"fmt"
	"strings"
)

// Alphabet is the option-name-safe charset for generated ids. The absence of
// "-" is load-bearing — it keeps the "<batch>-<token>" spawn-marker split
// unambiguous.
const Alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// width is the byte length of every generated id. It lives beside Alphabet and
// is read by both the generator and IsTokenShaped, so generation and
// recognition cannot drift apart.
const width = 6

// Generator mints one id per call, erroring only when the system entropy source
// fails.
type Generator func() (string, error)

// NewGenerator returns a Generator drawing from Alphabet.
func NewGenerator() Generator {
	return func() (string, error) {
		bytes := make([]byte, width)
		if _, err := rand.Read(bytes); err != nil {
			return "", fmt.Errorf("failed to read random bytes: %w", err)
		}
		for i := range bytes {
			bytes[i] = Alphabet[int(bytes[i])%len(Alphabet)]
		}
		return string(bytes), nil
	}
}

// IsTokenShaped reports whether s could have been produced by the generator:
// exactly width bytes, each drawn from Alphabet.
//
// Length and membership are both counted in bytes, so no multi-byte input can
// satisfy the pair.
func IsTokenShaped(s string) bool {
	if len(s) != width {
		return false
	}
	for i := range len(s) {
		if strings.IndexByte(Alphabet, s[i]) < 0 {
			return false
		}
	}
	return true
}
