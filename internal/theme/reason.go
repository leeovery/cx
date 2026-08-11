package theme

// Reason is one of the seven reject classes — why a theme is not usable. Each
// constant's string value is user-facing copy, rendered verbatim by every
// surface, not an internal identifier.
type Reason string

// The seven reject classes. The first six are a ladder, evaluated in this
// order with the first failure short-circuiting, which is what guarantees a
// theme has exactly one reason. ReasonNotFound sits outside the ladder: a
// nominated slug with no corresponding file, where there is nothing to
// check.
const (
	ReasonBadName       Reason = "bad name"
	ReasonReservedName  Reason = "reserved name"
	ReasonUnreadable    Reason = "unreadable"
	ReasonBadSyntax     Reason = "bad syntax"
	ReasonBadColour     Reason = "bad colour"
	ReasonMissingTokens Reason = "missing tokens"
	ReasonNotFound      Reason = "not found"
)

// Rejection is why one theme is not usable: exactly one reason, never two,
// with a Detail that enumerates within that reason and never across reasons.
// Detail is rendered where the rejection is produced, in the exact form its
// surfaces print — nothing downstream re-derives or re-parses it. Line (only
// `bad syntax`), BadNameCause (only `bad name`), Tokens (`missing tokens` and
// `bad colour`) and Err (only `unreadable`) are the structured sources behind
// Detail; each is zero on every other reason.
type Rejection struct {
	Reason       Reason
	Detail       string
	Line         int
	BadNameCause BadNameCause
	Tokens       []string
	Err          error
}

// Error renders the rejection as "<reason>: <detail>", or the bare reason
// when there is no detail. Generic propagation only — user-facing surfaces
// compose their own line from Reason and Detail rather than parsing this.
func (r *Rejection) Error() string {
	if r.Detail == "" {
		return string(r.Reason)
	}
	return string(r.Reason) + ": " + r.Detail
}
