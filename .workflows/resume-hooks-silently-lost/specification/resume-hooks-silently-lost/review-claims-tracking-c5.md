# Review Tracking: Resume Hooks Silently Lost - Claims Verification

## Findings

### 1. Removal manifest points at a function that does not exist

**Source**: Tree measurement — `grep -rno '\bCreateSession\b' --include='*.go' internal cmd | wc -l`
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §7.2 What is removed

**Problem**:
The removal manifest for the `@portal-id` machinery sends the reader to a `CreateSession` function in `internal/session/create.go` to delete the session stamp. No such function exists anywhere in the tree. The stamp lives in `(*SessionCreator).CreateFromDir`, which is the only session-creation entry point in that file. The manifest is the list the delivery works from; a row naming a symbol that isn't there is the one row that cannot be followed.

**Proposal**:
Name the actual home of the stamp. `grep -n '^func ' internal/session/create.go` returns four functions, none called `CreateSession`, and the `SetSessionOption(prepared.SessionName, PortalIDOption, token)` call sits at `internal/session/create.go:92` inside `CreateFromDir` (declared at `:75`). Replace the function name in the table row.

**Evidence**:
Claim (§7.2, specification line 383):
```
| `internal/session/create.go` | `PortalIDOption` const; the `SetSessionOption` stamp in `CreateSession` |
```

Command and output:
```
$ grep -rno '\bCreateSession\b' --include='*.go' internal cmd | wc -l
0

$ grep -n '^func ' internal/session/create.go
20:func ShellFromEnv() string {
31:func BuildShellCommand(command []string, shell string) string {
63:func NewSessionCreator(git GitResolver, store ProjectStore, tmux TmuxClient, gen IDGenerator) *SessionCreator {
75:func (sc *SessionCreator) CreateFromDir(dir string, command []string) (string, error) {

$ sed -n '90,93p' internal/session/create.go
	if token, genErr := sc.gen(); genErr == nil {
		_ = sc.tmux.SetSessionOption(prepared.SessionName, PortalIDOption, token)
	}
```

Not present in the source material — `grep -n 'CreateSession\|CreateFromDir' .workflows/resume-hooks-silently-lost/investigation/resume-hooks-silently-lost.md` returns no matches (exit 1), so the name originates in the specification.

**Current**:
```
| `internal/session/create.go` | `PortalIDOption` const; the `SetSessionOption` stamp in `CreateSession` |
```

**Proposed Text**:
```
| `internal/session/create.go` | `PortalIDOption` const; the `SetSessionOption` stamp in `CreateFromDir` |
```

**Resolution**: Pending
**Notes**:
