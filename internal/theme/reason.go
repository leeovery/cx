package theme

// Reason is why a theme is not usable. Each constant's string value is
// user-facing copy, rendered verbatim by every surface, not an internal
// identifier.
type Reason string

// The ladder classes come first, in evaluation order: the first failure
// short-circuits, which is what guarantees a theme has exactly one reason.
// ReasonNotFound sits outside the ladder — a nominated slug with no
// corresponding file, where there is nothing to check.
const (
	ReasonBadName       Reason = "bad name"
	ReasonReservedName  Reason = "reserved name"
	ReasonUnreadable    Reason = "unreadable"
	ReasonBadSyntax     Reason = "bad syntax"
	ReasonBadColour     Reason = "bad colour"
	ReasonMissingTokens Reason = "missing tokens"
	ReasonNotFound      Reason = "not found"
)

// Rejection carries exactly one reason, never two, with a Detail rendered where
// the rejection is produced, in the exact form its surfaces print — nothing
// downstream re-derives or re-parses it. Line, BadNameCause, Tokens and Err are
// the structured sources behind Detail, each populated only for the reason it
// belongs to and zero on every other.
type Rejection struct {
	Reason       Reason
	Detail       string
	Line         int
	BadNameCause BadNameCause
	Tokens       []string
	Err          error
}

// Error renders the rejection as "<reason>: <detail>", or the bare reason when
// there is no detail. For generic propagation — user-facing surfaces compose
// their own line from Reason and Detail rather than parsing this.
func (r *Rejection) Error() string {
	if r.Detail == "" {
		return string(r.Reason)
	}
	return string(r.Reason) + ": " + r.Detail
}
