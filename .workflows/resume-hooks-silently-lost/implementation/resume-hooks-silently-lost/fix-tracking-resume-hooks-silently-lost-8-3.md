## Attempt 1

ISSUES:
- `internal/tui/model.go:2667-2671` — the TUI rename modal is the one consumer of `ValidateSessionName` that does not derive its message from the validator: any refusal sets the hardcoded `renameColonRefusedFlash` (`internal/tui/sessions_flash.go:55`, `":" isn't allowed in a session name — tmux reads it as a separator`). After this widening, a user who renames a session to `$foo` — a name with no colon in it — is told colons are not allowed. The change makes existing user-facing copy false; the executor's report accounted only for the `wrapSessionTargetErr` consumers and missed this one.
  FIX: keep `internal/tmux` the single rule owner and let the TUI select copy from *which* rule fired. In `internal/tmux/errors.go`, add two sentinels (`ErrSessionNameSeparator`, `ErrSessionNameIDPrefix`) wrapped alongside `ErrUnaddressableSessionName` in their respective branches; in `internal/tui/sessions_flash.go` pair `renameColonRefusedFlash` with `renameIDPrefixRefusedFlash` (`"$" isn't allowed at the start of a session name — tmux reads it as a session ID`), renaming the first to match; branch on `errors.Is` at `model.go:2667`. Extend `internal/tui/rename_colon_refusal_test.go` with a `$foo` case asserting the new text and the existing no-⚠-glyph rule.
  ALTERNATIVE: render the flash straight from `err.Error()`, which removes the duplicated copy entirely — but it swaps the band's user-facing voice for the developer-toned `"$foo" begins with "$", which tmux reserves as a session ID prefix` and falsifies the wording assertions at `rename_colon_refusal_test.go:29-36`. The reviewer recommends the sentinel approach: the rule stays single-sourced and the band keeps its own voice.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- internal/session/naming_test.go:204-205 — the doc claims every entry is a character tmux's grammar reads as structure rather than as part of a session name, which `a.b` falsifies: the task measured `a.b` as addressable both bare and exact, and it is in the table precisely because the generator replaces it for tidiness alone.
  OLD: // hostileProjectNames are directory names carrying the characters tmux's target
// grammar reads as structure rather than as part of a session name.
  NEW: // hostileProjectNames are directory names carrying the characters the generator
// replaces: the target separator and the leading ID prefix tmux cannot address,
// plus the period, which is addressable and replaced only for tidiness.

NOTES:
- `internal/session/naming_test.go:209-215` — the subtest `"it sanitises a project directory whose name begins with $"` exercises `SanitiseProjectName`, not `GenerateSessionName`, yet sits under `TestGenerateSessionNameProducesAddressableNames`. Its natural home is a row in the existing `TestSanitiseProjectName` table at `naming_test.go:15-45`, beside the `.` and `:` rows. Non-blocking — the assertion itself is correct.
- `internal/state/capture_colon_session_test.go` / `capture_colon_session_realtmux_test.go` — the file name now understates its contents (the unit sibling tables over `a:b` and `$foo`), and the real-tmux sibling still covers the colon case only. Beyond the task's stated Do list.
- `hostileProjectNames` is a package-level `var` in `naming_test.go` with one consumer; a local slice inside the guard would keep the test package's namespace tighter. Cosmetic.

## Attempt 2

ISSUES:
- No test anywhere pins the tmux behaviour the whole `$` rule is derived from. The sibling colon rule earned exactly such a test (`internal/state/capture_colon_session_realtmux_test.go:17`, unit lane, gated by `tmuxtest.SkipIfNoTmux`), because a refusal of a name tmux itself accepts is only correct while the external fact holds. The `$` rule now rests solely on prose — and this task's *other* prose measurement is demonstrably wrong (the period claim, banked), so prose is not a reliable basis here. If tmux's behaviour differed, Portal would silently refuse a rename that works and nothing would fail.
  FIX: extend `capture_colon_session_realtmux_test.go` to a table over `colonSession` and `dollarSession` (the consts already exist in the same package at `capture_colon_session_test.go:14-17`), mirroring the widening already applied to the mocked sibling. The existing `if capturedSession(idx, name) { return }` early-out already tolerates a tmux version that can address the name, so the test stays version-robust. Premise confirmed on 3.7c: with `$foo` live, `show-environment -t '=$foo'` exits 1 with `no such session: =$foo`, so the anomalous-not-vanished assertion will pass.
  ALTERNATIVE: an `internal/tmux/*_realtmux_test.go` client-level test asserting `ShowEnvironment("$foo")` on a live `$foo` session returns an error wrapping `ErrUnaddressableSessionName`. Narrower and closer to the rule, but it leaves the capture-loop classification — the outcome the task states — unproven against real tmux. The reviewer recommends extending the existing state-level test.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- internal/session/naming_test.go:216-219 — claims the period is addressable; measured on tmux 3.7c it is not, through the target form Portal composes.
  OLD:
		// hostileProjectNames are directory names carrying the characters the
		// generator replaces: the target separator and the leading ID prefix tmux
		// cannot address, plus the period, which is addressable and replaced only
		// for tidiness.
  NEW:
		// Directory names carrying every character the generator replaces: the
		// target separator, the leading ID prefix, and the period.

NOTES:
- `renameRefusalFlash` (`internal/tui/sessions_flash.go:65-70`) defaults to the separator wording for any error that is not `ErrSessionNameIDPrefix`. Correct today with two rules; a third rule would silently report `":"` — the exact wrong-character bug this change fixed for `$`. An explicit `errors.Is` branch per rule with a generic fallback would foreclose it. No current defect, so no change asked for.
- `$` is now declared as a literal in two packages (`internal/tmux/errors.go:74` and `internal/session/naming.go:22`), the tmux one unexported so it cannot be shared. This is the shape the task asked for and the generator guard is what pins them together — worth knowing the guard is the only thing holding it.
- The guard's 5-row table cannot establish criterion 3's universal claim. The invariant is provable by reading the two functions, so the table is adequate; a `FuzzGenerateSessionName` seeded with the same rows would prove it outright if ever worth hardening.
- Three test files now cover both rules under "colon" names. The executor renamed the functions and left the file names; a rename would be tidier but costs git history on files under active change.
