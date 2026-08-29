## Attempt 1

ISSUES:
- cmd/hooks.go:238 and cmd/run_hook_stale_cleanup.go:28 still emit the hooks component's via attr as bare string literals ("via", "cli" and "via", "internal"). These are the same closed vocabulary Via now types, and a typo at either is precisely the silent-grep failure the task exists to close. The change also leaves cmd/hooks.go internally inconsistent — line 211 passes hooks.ViaCLI, line 238 spells "cli" — an inconsistency this diff created.
  FIX: cmd/hooks.go:238 → "hook_key", hookKey, "via", hooks.ViaCLI.String(), "error", err. cmd/run_hook_stale_cleanup.go:28 → []any{"op", standDownMsg, "via", hooks.ViaInternal.String(), "reason", reason}. Both files already import internal/hooks; the wire values are unchanged, so cmd/hooks_read_lock_test.go and the stand-down assertions stay green untouched.
  CONFIDENCE: high

- internal/hooks/via.go:32-34 documents a behavioural promise — "the unset zero value reads as absent rather than impersonating whichever surface happens to be first" — that nothing verifies. It rests entirely on the iota + 1 in the const block; changing that to iota makes Via(0).String() return "cli" and every zero-valued Via silently impersonate the CLI surface, with no test failing.
  FIX: add one row to the via_test.go table (internal/hooks/via_test.go:14) — {hooks.Via(0), ""} — with the subtest name guarded (t.Run on an empty tc.want needs a label, e.g. name the case explicitly or add a name field). One line of coverage for a deliberate safety property.
  CONFIDENCE: high

NOTES:
- The spec amendment for the second bound exists only in the executor's report text — no durable record in the manifest. Worth capturing wherever spec reconciliation is tracked.
- Via implements fmt.Stringer but every emission site must remember .String(); a future site passing via bare would render correctly under the text handler and as an integer under the JSON handler. Adding func (v Via) LogValue() slog.Value { return slog.StringValue(v.String()) } would make the bare form correct by construction. Not required by the criteria, and the current code is correct at every existing site.
- viaNames is a map, so a fifth constant added without a map entry silently renders ""; TestViaWireValues enumerates a hardcoded four and would not notice. Low risk given the vocabulary is spec-closed.
- The loadSnapshot doc (internal/hooks/store.go:46-49) and the snapshotLockTimeout justification (internal/hooks/lock.go:23-31) now both explain the twice-taken sidecar. The task prescribed both edits, so this is compliant, but it is two copies of one rationale that can drift.
