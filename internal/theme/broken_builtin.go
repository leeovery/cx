package theme

import "fmt"

const brokenBuiltinFormat = "built-in theme %s is missing or invalid — this binary is broken"

// BrokenBuiltinError returns the fatal error for a fallback slug that does
// not resolve within the embedded set. Deliberately no wrapped cause (no
// cause changes what the user can do), and the slug named is the fallback's,
// never the nomination's — naming the nomination would send a user to fix
// their prefs for a fault in the binary.
func BrokenBuiltinError(slug string) error {
	return fmt.Errorf(brokenBuiltinFormat, slug)
}
