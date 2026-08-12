package theme

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// Acceptance compares this by exact bytes and never case-folds; a case-folded
// match is used only to keep a mis-cased file visible before it is rejected.
const FileExtension = ".theme"

// BadNameCause records which of `bad name`'s rules a name broke. It is a
// discriminator, never rendered.
type BadNameCause int

const (
	// BadNameNone is the zero value: not a bad-name rejection, so a caller may
	// read the field unconditionally.
	BadNameNone BadNameCause = iota
	// BadNameSlug — the stem does not match the slug charset.
	BadNameSlug
	// BadNameExtension — the extension is not exactly lowercase `.theme`, over a
	// stem that is a legal slug.
	BadNameExtension
)

// ValidSlug reports whether s matches `^[a-z0-9][a-z0-9-]*$`. The empty slug is
// illegal so the empty string stays the unset sentinel. It runs on non-file
// inputs too — a persisted slug is used verbatim as a path component, and this
// is what stops a hand-edited `../something` escaping the themes directory.
func ValidSlug(s string) bool {
	if s == "" || !isSlugStartByte(s[0]) {
		return false
	}

	for i := 1; i < len(s); i++ {
		if !isSlugByte(s[i]) {
			return false
		}
	}
	return true
}

// StripControl removes ANSI escape sequences and control characters from a theme
// name that will be echoed back to the user, and normalises nothing else. Apply
// it where the value is read, not where it is drawn, so every consumer inherits
// it. Neither pass subsumes the other: stripping an ESC alone would leave a
// printable `[31m`, while a bare newline or tab opens no sequence at all.
func StripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, ansi.Strip(s))
}

// SlugFromFilename derives a theme's slug from one directory entry's base name,
// or returns the `bad name` rejection saying which rule it broke, with an empty
// slug. Nothing is ever normalised: lowercasing a mis-cased name would let a
// user file shadow the built-in an invalid theme falls back to.
func SlugFromFilename(base string) (string, *Rejection) {
	stem, exact := strings.CutSuffix(base, FileExtension)
	if !exact {
		return "", badName(misCasedExtensionCause(base))
	}
	if !ValidSlug(stem) {
		return "", badName(BadNameSlug)
	}

	return stem, nil
}

// The stem is inspected to place the cause, never to derive a slug.
func misCasedExtensionCause(base string) BadNameCause {
	if len(base) < len(FileExtension) {
		return BadNameExtension
	}
	split := len(base) - len(FileExtension)
	if !strings.EqualFold(base[split:], FileExtension) {
		return BadNameExtension
	}
	if !ValidSlug(base[:split]) {
		return BadNameSlug
	}
	return BadNameExtension
}

// No Detail: rendering surfaces compose their own copy from the cause and
// filename, and one here would compete with it.
func badName(cause BadNameCause) *Rejection {
	return &Rejection{Reason: ReasonBadName, BadNameCause: cause}
}

func isSlugStartByte(c byte) bool {
	lower := c >= 'a' && c <= 'z'
	digit := c >= '0' && c <= '9'
	return lower || digit
}

func isSlugByte(c byte) bool {
	return isSlugStartByte(c) || c == '-'
}
