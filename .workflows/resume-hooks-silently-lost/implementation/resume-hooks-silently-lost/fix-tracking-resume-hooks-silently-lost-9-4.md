## Attempt 1

ISSUES:
- `internal/session/naming_test.go:214-236` (and the `SanitiseProjectName` table at `:14-65`) — nothing pins the **replacement-produced** leading hyphen, which is the highest-traffic real trigger of the bug being fixed and the one case whose correctness rests entirely on statement order inside `SanitiseProjectName`. A git root named `.dotfiles` (one exists on this machine) sanitised under the old code to `-dotfiles` → `-dotfiles-abc123`; under the new code `.` → `-` → trimmed → `dotfiles`, correct only because `TrimLeft` runs after `strings.NewReplacer`. Swap those two operations and every dotfile-named project mints a refused name again, with the whole suite still green: the hostile table's `-lead` covers a literal leading hyphen and `a.b` covers a mid-string period, but no input carries a *leading* period. The table's own comment claims the period among "the characters the generator drops or replaces", which overstates what it exercises.
  FIX: add `".dotfiles"` to `hostileProjectNames` at `internal/session/naming_test.go:220`, and a case to the `TestSanitiseProjectName` table — `{name: "drops the hyphen a leading period replaces into", input: ".dotfiles", want: "dotfiles"}` — so the ordering dependency is pinned at the unit that owns it as well as through the validator.
  ALTERNATIVE: replace the hostile-name table with a `go test -fuzz` target or a generated-input loop over the unwritable character set, which would cover the whole class rather than enumerated members. Heavier than this defect warrants and it would move an exhaustively-reasoned invariant into a probabilistic check; the reviewer recommends the table addition.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/tmux/errors.go:93-94` — the exported doc comment states the function's whole subject as exact-target addressability, which the third rule falsifies: `-bar` *is* expressible as `=-bar:`, and it is refused because the name also travels as a bare positional. CLAUDE.md was corrected for exactly this; the source contract was not.
  OLD: // ValidateSessionName reports whether a session name can be addressed by the
// exact-match target every per-session operation composes. The returned error
  NEW: // ValidateSessionName reports whether a session name can be handed back to tmux —
// addressed by the exact-match target every per-session operation composes, and
// carried as the bare positional a rename passes it as. The returned error
- `internal/tmux/tmux.go:282-283` — the refusal reason given is a name "tmux accepts" but Portal cannot address; the hyphen case is the opposite (tmux itself refuses it, with `unknown flag`).
  OLD: 	// Refused before the argv is composed: a name tmux accepts but Portal's own
	// target form cannot address afterwards is a session it can no longer read.
  NEW: 	// Refused before the argv is composed: the new name goes on as a bare
	// positional, and one Portal's target form cannot address afterwards is a
	// session it can no longer read.
- `internal/tmux/errors.go:81-86` — the sentinel-name insertion left the block's third line unwrapped and running well past its neighbours.
  OLD: // ErrSessionNameSeparator, ErrSessionNameIDPrefix and ErrSessionNameFlagPrefix
// report which rule refused a name; discriminate with errors.Is when the answer
// selects wording of its own. A refusal wraps one of them alongside ErrUnaddressableSessionName, so a caller
// that only cares that the name is unaddressable still matches. Each carries the
// clause it contributes to the refusal message rather than a heading of its own,
// so nothing is said twice.
  NEW: // ErrSessionNameSeparator, ErrSessionNameIDPrefix and ErrSessionNameFlagPrefix
// report which rule refused a name; discriminate with errors.Is when the answer
// selects wording of its own. A refusal wraps one of them alongside
// ErrUnaddressableSessionName, so a caller that only cares that the name is
// unaddressable still matches. Each carries the clause it contributes to the
// refusal message rather than a heading of its own, so nothing is said twice.

NOTES:
- `wrapSessionTargetErr` classifies on the *addressed* name, so the new rule silently widens what counts as unaddressable for legacy live sessions the old generator already minted as `-abc123`. When such a session genuinely vanishes mid-tick, `internal/state/capture.go:71` will now count it anomalous instead of natural churn (and, in the degenerate all-sessions-failed case, abort that one commit). The daemon's own pane-vanished classifier is unaffected. Transient, self-correcting on the next tick, and it shrinks as legacy names are renamed away; not worth a carve-out, but a consequence the task body did not anticipate.
- User-visible copy verified in step: the hyphen wording is a byte-for-byte structural mirror of the `$` wording, carries no glyph, and README quotes all three verbatim. Every carrier grepped — `sessions_flash.go`, README:197, `internal/capture/capture_test.go`, `internal/capture/swap_harness_test.go`. The README `r`-key table row genuinely needs no change.
- The generator-cannot-mint-a-refused-name invariant was established independently by construction plus 18 hostile inputs the enrolled table does not cover; all produced accepted names.
