package bootstrap

import "github.com/leeovery/portal/internal/warning"

// FatalError is the typed sentinel for unrecoverable bootstrap conditions.
// UserMessage is printed to stderr verbatim by the top-level error path. Soft
// failures are never wrapped in it — they degrade locally and continue.
type FatalError struct {
	UserMessage string
	Cause       error
}

func (e *FatalError) Error() string { return e.UserMessage }

func (e *FatalError) Unwrap() error { return e.Cause }

func NewFatal(userMsg string, cause error) *FatalError {
	return &FatalError{UserMessage: userMsg, Cause: cause}
}

// Warning is an alias, not a local type: cmd, cmd/bootstrap and tui must share
// one canonical shape across the import boundary.
type Warning = warning.Warning

// CorruptSessionsJSONWarning covers an unreadable sessions.json as well as an
// unparseable one.
func CorruptSessionsJSONWarning() Warning {
	return Warning{Lines: []string{
		"Portal state file unusable — restoration skipped.",
		"Check `portal doctor` or ~/.config/portal/state/portal.log.",
	}}
}

func SaverDownWarning() Warning {
	return Warning{Lines: []string{
		"Portal save daemon failed to start — sessions won't be captured.",
		"Run `portal doctor` for details.",
	}}
}
