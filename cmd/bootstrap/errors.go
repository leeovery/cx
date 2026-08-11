package bootstrap

import "github.com/leeovery/portal/internal/warning"

// FatalError is the typed sentinel for unrecoverable bootstrap conditions.
// UserMessage is the single line the top-level error path prints to stderr;
// Cause is exposed via Unwrap for errors.Is / errors.As. Soft failures are not
// wrapped in it — they degrade locally and continue.
type FatalError struct {
	UserMessage string
	Cause       error
}

// Error returns the UserMessage verbatim, so the default Cobra error path
// prints exactly the user-facing line.
func (e *FatalError) Error() string { return e.UserMessage }

// Unwrap exposes the underlying cause to errors.Is and errors.As.
func (e *FatalError) Unwrap() error { return e.Cause }

// NewFatal pairs a user-facing message with its underlying cause. Both are
// stored verbatim; formatting userMsg is the caller's responsibility.
func NewFatal(userMsg string, cause error) *FatalError {
	return &FatalError{UserMessage: userMsg, Cause: cause}
}

// Warning is a soft bootstrap failure that must not terminate Portal. It
// aliases internal/warning.Warning so cmd, cmd/bootstrap and tui share one
// canonical shape.
type Warning = warning.Warning

// CorruptSessionsJSONWarning returns the canonical warning for a sessions.json
// that exists but cannot be used, covering unreadable files as well as
// unparseable ones.
func CorruptSessionsJSONWarning() Warning {
	return Warning{Lines: []string{
		"Portal state file unusable — restoration skipped.",
		"Check `portal doctor` or ~/.config/portal/state/portal.log.",
	}}
}

// SaverDownWarning returns the canonical warning for _portal-saver failing to
// start after retries.
func SaverDownWarning() Warning {
	return Warning{Lines: []string{
		"Portal save daemon failed to start — sessions won't be captured.",
		"Run `portal doctor` for details.",
	}}
}
