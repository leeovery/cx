package session

import "strings"

// IsTokenShaped reports whether s could have been produced by the nanoid
// generator: exactly suffixLen bytes, each drawn from NanoIDAlphabet. Both the
// width and the charset are read from the generator's own declarations, so a
// change to either moves generation and recognition together.
//
// Length and membership are both counted in bytes, so no multi-byte input can
// satisfy the pair.
func IsTokenShaped(s string) bool {
	if len(s) != suffixLen {
		return false
	}
	for i := range len(s) {
		if strings.IndexByte(NanoIDAlphabet, s[i]) < 0 {
			return false
		}
	}
	return true
}
