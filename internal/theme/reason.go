package theme

// Reason is one of §6.2's seven reject classes — why a theme is not usable.
//
// The string value of each constant is the terse label the surfaces render
// VERBATIM: the panel row prints it behind §14A's "⚠ " prefix, doctor prints it
// before the detail, and the theme log component carries it as the `reason`
// attr. They are user-facing copy rather than internal identifiers.
type Reason string

// The seven reject classes, declared in §6.2's evaluation order.
//
// The first six are a ladder: they are evaluated in this order and the first
// failure short-circuits, which is what guarantees a theme always has exactly
// one reason and the panel's single-reason row is never a choice. The name is
// decided from the filename before the file is opened, so a bad-name file can
// never also report on its contents; a lexical failure aborts the parse before
// any value is validated; and the presence check runs last, on a file that
// parsed and whose every known value is well-formed.
//
// ReasonNotFound is deliberately outside that ladder: it applies to a slug named
// by prefs.json with no corresponding file, where there is nothing to check.
const (
	ReasonBadName       Reason = "bad name"
	ReasonReservedName  Reason = "reserved name"
	ReasonUnreadable    Reason = "unreadable"
	ReasonBadSyntax     Reason = "bad syntax"
	ReasonBadColour     Reason = "bad colour"
	ReasonMissingTokens Reason = "missing tokens"
	ReasonNotFound      Reason = "not found"
)

// Rejection is why one theme is not usable: exactly one reason, plus the detail
// naming what is wrong within that reason.
//
// The invariant is §6.2's, and it is what the whole rejection machinery is
// shaped around: a Rejection always carries EXACTLY ONE reason, and its detail
// never spans two. A file that is both duplicate-keyed and missing tokens is
// `bad syntax`, and its detail says nothing whatsoever about presence. Detail
// enumerates WITHIN the reason — every missing token, or every bad-coloured
// pair — never across reasons.
//
// Detail is rendered by whoever produces the rejection, in the exact §14A form
// its surfaces print: nothing downstream re-derives, re-orders, re-wraps or
// re-prefixes it. Line carries the same line number in machine-readable form
// for the one reason that has one (`bad syntax`), and is 0 for every other.
//
// BadNameCause is the same shape of narrowing for the one reason whose surfaces
// need to say WHICH rule was broken while the reason class stays single (see
// BadNameCause in name.go). It is BadNameNone on every rejection that is not a
// `bad name` one.
type Rejection struct {
	Reason       Reason
	Detail       string
	Line         int
	BadNameCause BadNameCause
}

// Error renders the rejection as "<reason>: <detail>", or as the bare reason
// when there is no detail.
//
// This is the generic propagation form only. Every surface that shows a
// rejection to a user composes its own line from Reason and Detail — the panel
// has room for the label alone (§9.5), doctor frames the pair differently again
// per §14A — so none of them parses this string back apart.
func (r *Rejection) Error() string {
	if r.Detail == "" {
		return string(r.Reason)
	}
	return string(r.Reason) + ": " + r.Detail
}
