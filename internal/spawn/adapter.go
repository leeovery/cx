package spawn

// Adapter opens one new host-terminal window running a given command. It
// quarantines every OS/terminal-specific concern (AppleScript, AppleEvent
// codes, TCC) so general code switches on the Result taxonomy alone.
type Adapter interface {
	// OpenWindow runs command verbatim as a real argv. Adapters are not
	// session-aware: never bake in `portal open`, never parse the argv.
	OpenWindow(command []string) Result
}

// Outcome is the terminal-agnostic classification of an OpenWindow attempt, and
// a closed taxonomy. "Unsupported" is deliberately not a member: whether a host
// terminal has a driver is a resolution-tier decision taken before any
// OpenWindow call.
type Outcome int

const (
	// OutcomeUnknown is the unset zero-value sentinel, so a bare Result{}
	// fails OK() instead of passing as a success that would gate a
	// self-attach. An adapter must never return it.
	OutcomeUnknown Outcome = iota
	OutcomeSuccess
	// OutcomeSpawnFailed — a driver was available but the window failed to
	// open.
	OutcomeSpawnFailed
	// OutcomePermissionRequired — the OS refused for a permission reason;
	// Guidance carries the driver-composed hint.
	OutcomePermissionRequired
)

// Result is the typed outcome of an OpenWindow attempt. Detail and Guidance are
// opaque driver-owned payloads — Detail rides up only as a log attr, Guidance is
// populated only on the permission path — and general code classifies solely on
// Outcome.
type Result struct {
	Outcome  Outcome
	Detail   string
	Guidance string
}

func Success(detail string) Result {
	return Result{Outcome: OutcomeSuccess, Detail: detail}
}

func SpawnFailed(detail string) Result {
	return Result{Outcome: OutcomeSpawnFailed, Detail: detail}
}

func PermissionRequired(detail, guidance string) Result {
	return Result{Outcome: OutcomePermissionRequired, Detail: detail, Guidance: guidance}
}

// OK reports whether the window opened successfully.
func (r Result) OK() bool {
	return r.Outcome == OutcomeSuccess
}
