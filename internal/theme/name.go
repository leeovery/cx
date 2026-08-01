package theme

import "strings"

// themeExtension is the one extension a theme file carries (§5.3). It is
// compared by exact bytes and never case-folded — see SlugFromFilename.
const themeExtension = ".theme"

// BadNameCause records WHICH of `bad name`'s rules a name broke.
//
// The reason class is deliberately ONE (§6.2): the user-facing fact is the same
// in all three causes — the name is not usable as a theme identity — and the
// panel row has no width to discriminate. The cause exists solely so the two
// surfaces that DO have the width can name which: §14A gives doctor a distinct
// line per cause (`slug must be lowercase letters, digits and hyphens` versus
// `extension must be lowercase .theme`, a separate message precisely because
// the slug portion is already legal), and `portal theme export` frames a
// non-file input differently again.
//
// It is a discriminator, never rendered: unlike Reason, whose values ARE the
// terse user-facing labels, no cause value reaches a user. The surfaces switch
// on it and compose their own copy.
type BadNameCause int

const (
	// BadNameNone is the zero value: this rejection is not a bad-name one. It
	// is what every Rejection built elsewhere in the package carries, so a
	// caller may read the field unconditionally rather than pairing the read
	// with a Reason check.
	BadNameNone BadNameCause = iota
	// BadNameSlug — the stem does not match §5.2's charset.
	BadNameSlug
	// BadNameExtension — the extension is not exactly lowercase `.theme`
	// (§5.6).
	BadNameExtension
)

// ValidSlug reports whether s matches §5.2's `^[a-z0-9][a-z0-9-]*$` — lowercase
// letters, digits and hyphens, at least one character, not starting with a
// hyphen.
//
// The anchoring closes three edges a bare character class leaves open: the
// EMPTY slug is illegal, so the empty string stays unambiguously §8.1's *unset*
// sentinel; a LEADING HYPHEN is illegal, because it reads as a flag in every
// context a slug is typed into; and a TRAILING hyphen is legal but pointless.
// There is NO LENGTH BOUND — the slug is an identity, and §9.5/§9.8's
// truncation is a display concern that must not silently become a validity
// rule.
//
// It is exported because two callers apply the same rule to a NON-FILE input,
// where no extension is involved and SlugFromFilename would be the wrong entry
// point: a slug persisted in prefs.json (§8.6), which is used verbatim as a
// path component on a by-name lookup that does not enumerate — so this check is
// what stops a hand-edited `../something` escaping the themes directory — and
// the argument to `portal theme export` (§12.1). Both use ValidSlug directly.
//
// A multi-byte rune cannot slip through the byte-wise scan: each of its bytes
// is ≥ 0x80 and so fails both classes.
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

// SlugFromFilename derives a theme's slug from one directory entry's base name,
// or returns the one `bad name` rejection saying which rule the name broke
// (§5.1, §5.2, §5.6).
//
// The two rules are checked in this order because a name that is not a theme
// filename at all is decided before anything is asked of its stem: the
// extension must be EXACTLY lowercase `.theme`, compared by bytes and never
// case-folded, and only then must the remaining stem satisfy ValidSlug.
//
// NOTHING IS EVER NORMALISED — not lowercased, not trimmed. That is a safety
// property, not a style choice (§5.2): lowercasing `Nord.theme` to `nord` would
// let a user file shadow the built-in `nord`, and since an invalid theme falls
// back to a built-in (§5.4/§8.5), a typo'd drop-in could break the very thing
// Portal falls back to. Rejecting rather than normalising removes the case
// question outright instead of defining case-insensitive matching, which is
// what keeps the reserved-name check EXACT STRING EQUALITY and makes
// `Nord.theme` sitting beside a built-in `nord` safe on a case-insensitive
// macOS filesystem.
//
// Accepting only the exact extension is likewise load-bearing: enumeration
// (§5.6) matches the extension case-insensitively so a `Nord.THEME` is still
// VISIBLE rather than silently absent, and this function is what then refuses
// it. A non-exact extension therefore never contributes a slug, so a duplicate
// slug cannot be minted and no precedence rule or ordering tie-break is needed.
//
// On rejection the slug is empty. A caller never sees a name alongside an
// error, and `Nord.theme` in particular yields no slug at all rather than a
// quietly corrected one.
func SlugFromFilename(base string) (string, *Rejection) {
	stem, exact := strings.CutSuffix(base, themeExtension)
	if !exact {
		return "", badName(BadNameExtension)
	}
	if !ValidSlug(stem) {
		return "", badName(BadNameSlug)
	}

	return stem, nil
}

// badName builds the one rejection a name failure produces.
//
// Both causes route through here, so the reason cannot drift apart from the
// cause. There is no detail: §14A composes a whole line per cause from the
// cause plus the filename the caller already holds, and the panel row renders
// the reason label alone, so a detail here would be a second, competing copy of
// that copy.
func badName(cause BadNameCause) *Rejection {
	return &Rejection{Reason: ReasonBadName, BadNameCause: cause}
}

// isSlugStartByte reports whether c may open a slug: a lowercase letter or a
// digit, the class the leading-hyphen anchor excludes.
func isSlugStartByte(c byte) bool {
	lower := c >= 'a' && c <= 'z'
	digit := c >= '0' && c <= '9'
	return lower || digit
}

// isSlugByte reports whether c may appear after a slug's first character: the
// opening class, plus the hyphen.
func isSlugByte(c byte) bool {
	return isSlugStartByte(c) || c == '-'
}
