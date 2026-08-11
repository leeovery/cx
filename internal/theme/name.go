package theme

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// FileExtension is the one extension a theme file carries. Acceptance compares
// it by exact bytes and never case-folds (see SlugFromFilename); enumeration
// alone matches it case-insensitively so a mis-cased file is visible before it
// is rejected.
const FileExtension = ".theme"

// BadNameCause records which of `bad name`'s rules a name broke, so surfaces
// with the width can name which. It is a discriminator, never rendered.
type BadNameCause int

const (
	// BadNameNone is the zero value: not a bad-name rejection, so a caller
	// may read the field unconditionally.
	BadNameNone BadNameCause = iota
	// BadNameSlug — the stem does not match the slug charset.
	BadNameSlug
	// BadNameExtension — the extension is not exactly lowercase `.theme`,
	// over a stem that is a legal slug or that leaves nothing to judge.
	BadNameExtension
)

// ValidSlug reports whether s matches `^[a-z0-9][a-z0-9-]*$`. The empty slug
// is illegal so the empty string stays the unset sentinel, and there is
// deliberately no length bound — truncation is a display concern. Applied to
// non-file inputs too (a persisted slug used verbatim as a path component),
// where it is what stops a hand-edited `../something` escaping the themes
// directory.
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

// StripControl removes ANSI escape sequences and control characters from a
// theme name that will be echoed back to the user. Apply it where the value
// is read, not where it is drawn, so every consumer inherits it. Neither pass
// subsumes the other: stripping the ESC of `\x1b[31m` alone would leave a
// printable `[31m`, while a bare newline or tab opens no sequence at all. It
// normalises nothing else.
func StripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, ansi.Strip(s))
}

// SlugFromFilename derives a theme's slug from one directory entry's base
// name, or returns the `bad name` rejection saying which rule it broke; on
// rejection the slug is empty. Nothing is ever normalised — lowercasing
// `Nord.theme` to `nord` would let a user file shadow the built-in an invalid
// theme falls back to, so rejecting is what keeps the reserved-name check
// exact string equality on a case-insensitive filesystem.
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

// The stripped stem is a judgement only — never returned, never a slug.
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

// No detail: the rendering surfaces compose their own copy from the cause and
// filename, and a detail here would be a second, competing one.
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
