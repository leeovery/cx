# Analysis Tasks: Resume Hooks Silently Lost (Cycle 4)

## Task 1: The saver-readiness barrier fails open on a fixed 2s budget, so a cold boot can proceed with no daemon

severity: high
sources: bank

**Problem**: `waitForSaverDaemonReady` (`internal/tmux/portal_saver.go:226`) polls `isSaverDaemonReady` against `saver.Readiness.Timeout`, hardcoded to `2 * time.Second` at `portal_saver.go:109`. On expiry it emits one WARN and **returns nil**, so `BootstrapPortalSaver` continues as if the daemon came up and every step after it addresses a saver that never started — the state-capture loop silently does not run. Under host IO contention, fork+exec+flock+PID-file-write is not reliably a 2s operation. Two independent investigations (executor and reviewer of task 8-15) reached this from opposite directions, and the log signature is unambiguous: a failing run shows `respawn-daemon to_pid=X` with no `daemon ready` line and all daemons dead. It is also the single root cause of the composite-e2e flake family banked five separate times across this phase — `TestCompositeBootstrap_ConvergesPgrepToOneDaemon`, `_FObservables`, `_FreshAcquireDaemonLockRefusesPostBootstrap` and `_ExternalSaverKillTriggersSelfEject` all fail with the same `pgrep did not converge to 1 … count=0` signature, reproduced on a pristine `git archive` export of HEAD (3-in-10 under load, 0-in-10 idle), which rules out any change in this work unit.

**Solution**: Replace the fixed wall-clock readiness budget with a progress-based wait bounded by a hard ceiling — the same `Stall`/`Ceiling` separation `harnesstest.ProgressWait` already expresses for the test side — so a machine that is merely slow does not decide the verdict, and give the expiry a consequence: the barrier reports that the daemon did not come up rather than returning nil. The seam is already injectable (`SaverReadinessSeams`, `SaverOperationSeams.WaitForReady`, exported through `internal/tmux/export_test.go`), so the pacing stays drivable from tests. Whether the expiry becomes a returned error or stays best-effort-with-a-louder-signal follows from what `BootstrapPortalSaver`'s caller can do about it — bootstrap never escalates a saver failure to a fatal, so the honest shape is a `SaverDownWarning` the user sees rather than a silent continue.

**Outcome**: A cold boot on a loaded machine either brings the daemon up or tells the user it did not; the four composite-bootstrap integration tests stop flaking against the thing they wait on having given up.

**Do**:
- Replace `SaverReadinessSeams.Timeout` (`internal/tmux/portal_saver.go:61-64`, defaulted at `:107-110`) with a `Stall`/`Ceiling` pair beside the existing `PollInterval`, keeping the struct itself as the injection point `internal/tmux/export_test.go` already exposes so the pacing stays drivable.
- Rewrite `waitForSaverDaemonReady` (`:226`) as a progress wait over `isSaverDaemonReady`'s observation — the pid-file read's error-or-pid and the `IdentifyDaemon` verdict — resetting the stall deadline whenever that observation changes and returning nil the moment it reads ready; the ceiling remains an unconditional hard stop.
- Give the expiry a consequence: keep the WARN and return a new exported `ErrSaverDaemonNotReady` naming the last observation, replacing today's bare `return nil`.
- Stop `BootstrapPortalSaver` discarding the result at `:306` — return the wait's error — so `cmd/bootstrap/bootstrap.go:178` converts it into the existing `SaverDownWarning` and the user is told, without any step becoming fatal.
- Set the shipped figures so the stall covers a healthy fork+exec+flock+pid-write (the 2s the old fixed budget carried) and the ceiling bounds the whole step well above it; both declared beside `PollInterval` in the same seam literal.

**Acceptance Criteria**:
- [ ] A daemon that comes up after longer than the stall but with the pid file appearing part-way through is waited out and reported ready, where the old fixed budget gave up.
- [ ] A daemon that never comes up at all fails the wait at the ceiling, not at the stall, and `BootstrapPortalSaver` returns that error rather than nil.
- [ ] A cold-boot bootstrap whose saver never becomes ready surfaces exactly one `SaverDownWarning` and no fatal abort; every other bootstrap step still runs.
- [ ] An immediately-ready daemon returns on the first poll with no stall or ceiling wait, and still emits the `daemon ready` INFO carrying its pid.
- [ ] `TestCompositeBootstrap_ConvergesPgrepToOneDaemon`, `_FObservables`, `_FreshAcquireDaemonLockRefusesPostBootstrap` and `_ExternalSaverKillTriggersSelfEject` pass in the integration lane under load as well as idle.

**Tests**:
- `"it returns ready as soon as the pid file names a live portal state daemon"`
- `"it keeps waiting while the readiness observation is still changing, past the stall figure"`
- `"it gives up at the ceiling when nothing ever changes, and returns ErrSaverDaemonNotReady"`
- `"it returns the readiness failure from BootstrapPortalSaver instead of reporting success"`
- `"it surfaces a saver that never came up as a SaverDownWarning and aborts no bootstrap step"`

## Task 2: A period-bearing session name is unaddressable through `SessionTargetExact` and is silently dropped from `sessions.json`

severity: high
sources: bank

**Problem**: Measured on tmux 3.7c against isolated `-L` sockets: `show-environment -t =a.b` succeeds on a server holding only `a.b`, but fails with `no such session: =a.b` as soon as any **other** session name is a longer prefix-extension of the pre-period component (`anchor`, `apple` reproduce it; `a`, `b`, `zzz` do not). tmux splits a colon-free target on `.` into `window.pane`, so the session lookup is a fallback the prefix candidate displaces. Reproduced Portal-shaped: with `_portal-bootstrap`, `my-cool-app-abc123` and a user session `my.app` live, `show-environment -t =my.app` exits 1 — which `internal/state/capture.go:70-74` classifies as `ErrNoSuchSession`, counts as natural churn, logs `capture skipping vanished session` and **drops a live session and all its scrollback from `sessions.json`**. `has-session` and `kill-session` fail identically. This is the work unit's own failure mode — a silent loss whose only trace is a line that says the thing was gone — reached through a different door. Generated names are safe (`session.SanitiseProjectName` replaces `.` with `-`), so the exposure is user-created and user-renamed sessions, and `ValidateSessionName` accepts a period by design, pinned at `internal/tmux/session_name_test.go:225`.

**Solution**: Audit the remaining `SessionTargetExact` call sites per command and move each one tmux parses as a window-or-pane target onto `CoordTargetExact` (`=name:`), which the same measurement resolves correctly in every tested case — `internal/tmux/tmux.go:88,97,272,287,295,589,780` and `internal/tmux/clients.go:19`, plus the two hand-composed argv sites at `cmd/open.go:88` and `internal/session/quickstart.go:62`. The choice is per command, measured, exactly as `SessionTargetExact`'s own doc comment instructs ("Measure a command before choosing"); the validator is not widened, so `a.b` stays a legal session name and its acceptance test stays green.

**Outcome**: A user session whose name contains a period is captured, killed, renamed and switched to correctly whatever else is live on the server, instead of being reported as vanished by whichever command reads it first.

**Do**:
- Measure each remaining `SessionTargetExact` site against tmux with a prefix-extension sibling live, per command: `has-session` (`internal/tmux/tmux.go:88`, `:97`), `kill-session` (`:272`), `rename-session`'s `-t` (`:287`), `switch-client` (`:295`), `show-environment` (`:589`), `set-environment` (`:780`), `list-clients` (`internal/tmux/clients.go:19`), and the two hand-composed `attach-session` argvs at `cmd/open.go:88` and `internal/session/quickstart.go:62`.
- Move every site the measurement shows resolving correctly under `=name:` onto `CoordTargetExact`, leaving on `SessionTargetExact` only those a measurement shows must keep the bare form.
- Leave `ValidateSessionName` untouched: `a.b` stays a legal session name and `internal/tmux/session_name_test.go:225` stays green.
- Update `SessionTargetExact`'s and `CoordTargetExact`'s doc comments so the call-site kinds each names match the tree after the move, and update the same enumeration in CLAUDE.md's `tmux` row.
- Add a real-tmux case that reproduces the loss end to end: a live `my.app` beside `_portal-bootstrap` and a prefix-extension sibling, captured through `internal/state`'s capture loop, must appear in `sessions.json` rather than be logged as a vanished session.

**Acceptance Criteria**:
- [ ] With a period-bearing session and a prefix-extension sibling both live, `HasSession`, `KillSession`, `RenameSession`, `SwitchClient`, `ShowEnvironment` and `SetSessionEnvironment` all address the period-bearing session and not the sibling.
- [ ] A period-bearing session survives one capture cycle into `sessions.json` with its scrollback, and no `capture skipping vanished session` line is emitted for it.
- [ ] `ValidateSessionName` still returns nil for `a.b`, and its existing acceptance test is unmodified.
- [ ] Every moved site's argv is composed through a target helper, so `internal/tmux/target_composition_guard_test.go` stays green with no widening.
- [ ] The two hand-composed `attach-session` argvs pin their target through the same helper the client uses for that command kind.

**Tests**:
- `"it kills the period-bearing session and not its prefix-extension sibling"`
- `"it reads a period-bearing session's environment while a longer prefix sibling is live"`
- `"it captures a period-bearing session into sessions.json instead of reporting it vanished"`
- `"it still accepts a period-bearing name as addressable"`
- `"it renames and switches to a period-bearing session with a prefix sibling live"`

## Task 3: `cmd`'s `TestMain` does not poison `HOME`, so a test resolving a default config path moves the developer's real config files

severity: high
sources: bank

**Problem**: `cmd/config.go`'s `configFilePath` runs the one-shot Application Support migration as a side effect of **any** non-overridden resolve, and that migration `os.Rename`s files under `~/Library/Application Support/portal/` into `~/.config/portal/`. `cmd/testmain_isolation_test.go` poisons the five `PORTAL_*` path variables and `TMUX` package-wide precisely so a missed isolation fails loudly — but not `HOME`, which is the boundary this whole class needs. Isolation there is per-subtest discipline (`cmd/config_test.go:15-17` carries a comment asking every subtest to pin a temp `HOME`), and this work unit contains a task that exists because that discipline slipped. The project's ABSOLUTE INVARIANT is that a test must never mutate the real filesystem outside its temp dirs; today one forgotten `t.Setenv("HOME", …)` in a new `cmd` test does exactly that, silently, and only on a machine that still has the old macOS directory.

**Solution**: Poison `HOME` package-wide in `cmd`'s `TestMain` the way the `PORTAL_*` variables and `TMUX` already are — a per-run temp directory rather than a `/nonexistent` path, since the migration and the default resolve both need a home that exists to be observed against. Every subtest that pins its own temp `HOME` keeps working unchanged; a subtest that forgets one reads the poisoned home instead of the developer's.

**Outcome**: The migration-against-the-real-home hazard is structural rather than disciplinary, in the same shape as the existing tmux poison, and the per-subtest comment asking for the pin can say what it protects rather than being the only thing that protects it.

**Do**:
- In `cmd/testmain_isolation_test.go`'s `TestMain`, create a per-run temp directory with `os.MkdirTemp` and `os.Setenv("HOME", …)` to it alongside the existing `PORTAL_*` and `TMUX` poisons — a real directory rather than a `/nonexistent` path, since both the Application Support migration and the default resolve need a home that exists.
- Remove the temp directory before exiting: capture `m.Run()`'s code into a variable, run the removal, then `os.Exit(code)`, so the deferred-cleanup trap does not leak a directory per run.
- Update `TestMain`'s file comment so it names `HOME` among the poisoned boundaries.
- Re-voice `cmd/config_test.go:15-17`'s comment from "every subtest must pin a temp HOME" to what the pin now buys that subtest, and leave every existing per-subtest `t.Setenv("HOME", …)` in place.
- Add a subtest proving the poison bites: with no `HOME` pinned locally, a default-path resolve lands under the package-wide temp home and not under `os.UserHomeDir()`.

**Acceptance Criteria**:
- [ ] `HOME` is set package-wide in `cmd`'s `TestMain` to a per-run temp directory that exists at the time every test runs.
- [ ] A `cmd` test that resolves a non-overridden config path without pinning its own `HOME` reads the poisoned home; nothing is created or renamed under the developer's real `~/Library/Application Support/portal/` or `~/.config/portal/`.
- [ ] The temp home is removed after `m.Run()` returns, with the run's exit code preserved.
- [ ] Every subtest that already pins its own temp `HOME` passes unchanged.
- [ ] `go test ./cmd` leaves no `HOME`-rooted artifacts outside the per-run temp directory.

**Tests**:
- `"it resolves a default config path under the package-wide poisoned HOME when a subtest pins none"`
- `"it runs the Application Support migration against the poisoned home rather than the developer's"`
- `"it still honours a subtest's own HOME pin over the package-wide poison"`

## Task 4: `ValidateSessionName` accepts a leading `-`, which tmux refuses as a rename target

severity: medium
sources: bank

**Problem**: Measured on tmux 3.7c: `rename-session -t =old -bar-abc123` fails with `unknown flag -b` — the new name is a bare positional at `internal/tmux/tmux.go:287` and Portal composes no `--` terminator. `ValidateSessionName` returns nil for such a name, so the picker's `r` modal lets it through to a failure worded by tmux as a flag error rather than by Portal as a refusal. The name is reachable in ordinary use: `session.SanitiseProjectName` maps a `$`-leading project directory to a `-`-leading fragment, and an empty project name yields `-abc123`, so Portal's own generator mints names of this shape (safe where they land, as `-s`/`-t` optargs, but they then show in the picker as rename sources and as names a user may retype). This is the same two-definitions-of-an-unwritable-name class as the `:` and `$` rules, on a third character.

**Solution**: A session name may not begin with `-`. `ValidateSessionName` refuses one through the existing `ErrUnaddressableSessionName` machinery with a rule sentinel of its own, so `renameRefusalFlash` names the offending character the way it already does for `:` and `$`, and the picker reports the refusal in its notice band rather than letting tmux word it as a flag error. `SanitiseProjectName` stays the single home of what a generated fragment may contain and stops minting a `-`-leading fragment.

That last part carries a sub-decision this task settles rather than raises: the leading `-` is currently Portal's own escape for the `$` rule (`internal/session/naming.go:31-33` rewrites a leading `$` to a leading `-`), so refusing the hyphen means that escape needs replacing. Drop the `$` rather than substituting for it — the two rules then read as one rule, "a generated fragment carries none of the unwritable characters", instead of one unwritable character escaping to another. An empty sanitised name is the same case and takes the same treatment: the nanoid suffix alone is a valid name.

Derivation for the direction: Portal's precedent is to shrink the set of names it accepts rather than engineer around tmux's parsing, which it already does for `:` and `$`. The alternative — supporting the name by terminating every argv that passes a session name as a bare positional — buys the user a name almost nobody types and leaves a class that must be re-checked whenever a new command is composed. Both sides leave the user with a working rename and a clear message, so the ongoing obligation is the deciding cost.

**Outcome**: A rename to a `-`-leading name is refused by Portal in its own words naming the hyphen, and Portal never generates a name of that shape.

**Do**:
- In `internal/tmux/errors.go`, add a `flagPrefix = "-"` const beside `targetSeparator` and `sessionIDPrefix`, and an `ErrSessionNameFlagPrefix` sentinel in the same `var` block carrying its own clause; extend `ValidateSessionName` with a third `strings.HasPrefix` rule wrapping `ErrUnaddressableSessionName` alongside it.
- In `internal/tui/sessions_flash.go`, add a `renameFlagPrefixRefusedFlash` const beside its two siblings and a third `errors.Is` arm in `renameRefusalFlash` discriminating on the new sentinel.
- In `internal/session/naming.go`, change `SanitiseProjectName` to **drop** a leading `$` (`strings.TrimPrefix`) rather than rewrite it to `-`, so the function stops minting a `-`-leading fragment at all.
- In `GenerateSessionName`, compose the candidate as the suffix alone when the sanitised fragment is empty, so an empty project name yields `abc123` rather than `-abc123`.
- Add the refusal to README's `hook` section wording set and to the picker's `r`-key documentation, matching how `:` and `$` are already described.

**Acceptance Criteria**:
- [ ] `ValidateSessionName("-bar")` returns an error satisfying `errors.Is(err, tmux.ErrUnaddressableSessionName)` and `errors.Is(err, tmux.ErrSessionNameFlagPrefix)`, and names the hyphen in its message.
- [ ] A hyphen appearing anywhere but the first byte is still accepted — `my-cool-app-abc123` validates.
- [ ] `RenameSession` refuses a `-`-leading new name before composing its argv, so tmux never sees it and no `unknown flag` error can surface.
- [ ] The picker's `r` modal reports the refusal in its notice band naming the hyphen, distinct from the `:` and `$` wordings.
- [ ] `SanitiseProjectName("$work")` returns `work`, and `GenerateSessionName("", …)` returns a name that is the nanoid alone with no leading hyphen.
- [ ] No production path can produce a session name beginning with `-`.

**Tests**:
- `"it refuses a session name beginning with a hyphen"`
- `"it accepts a hyphen that is not the first character"`
- `"it reports the hyphen refusal with the flag-prefix wording in the rename band"`
- `"it drops a leading dollar from a project fragment rather than substituting a hyphen"`
- `"it generates the nanoid alone when the sanitised project fragment is empty"`
- `"it refuses the rename before composing the tmux argv"`

## Task 5: The `on-resume` event is an untyped literal the store now reaches for internally

severity: medium
sources: architecture

**Problem**: `Store.Set(key, event, command, via)` and `Store.Remove(key, event, via)` model the event dimension as a caller-supplied plain `string`, and the only value passed anywhere is the literal `"on-resume"` — spelled at eight sites (`internal/hooks/store.go:113,163,354`, `internal/hooks/lookup.go:25`, `cmd/hooks.go:219,293`, `internal/hookstest/staging.go:130`, `internal/hookstest/hooks.go:79`), two of them inside `internal/hooks` itself. The worst is new: `deleteStale`'s audit line sources its `value` attr with `h[key]["on-resume"]`, so the store hardcodes one event to read a map whose whole shape exists to be event-generic. An entry persisted under any other event would be deleted and logged with an **empty** `value` — the exact "what did I lose?" breadcrumb this work unit added to make a reaped hook recoverable, silently wrong. The inconsistency is sharpest against `Via`, which this same work unit gave a closed integer enum for a strictly weaker reason: `Via` is a log attr, while the event string is the second-level key into persisted state, where an invented literal writes an entry nothing will ever fire.

**Solution**: Give the event the same treatment `Via` got — a `hooks.Event` type (or at minimum an exported `hooks.EventOnResume` constant) taken by `Set`, `Remove` and `LookupOnResume`, so the vocabulary is closed at the boundary and spelled once. Separately, stop `deleteStale` reaching into the map for one event name: the pre-delete map is already in hand, so the removal record carries the removed entry's events and the audit line reports what was actually removed rather than what the store assumed was there.

**Outcome**: `"on-resume"` is spelled once in the tree, no caller can persist an entry under an invented event, and a reaped entry's `value` breadcrumb reports the command that was actually removed whatever event it was filed under.

**Do**:
- Add `event.go` to `internal/hooks` declaring `type Event string` with the single `EventOnResume Event = "on-resume"` const and a `String()` accessor, in the shape `via.go` already establishes for `Via`.
- Change `Store.Set` and `Store.Remove` to take `event Event`, and `classifySet` with them; the map index converts at the one point it is written.
- Re-point the eight literal sites onto the constant: `internal/hooks/store.go:113,163,354`, `internal/hooks/lookup.go:25`, `cmd/hooks.go:219,293`, `internal/hookstest/staging.go:130`, `internal/hookstest/hooks.go:79`.
- In `deleteStale` (`internal/hooks/store.go:334-355`), stop indexing `h[key]["on-resume"]`: build the removal record from the pre-delete map's own event map for each removed key, so the `value` attr reports the removed commands rather than one assumed event.
- Where a removed key holds more than one event, render the `value` attr from what that key actually held rather than picking one.

**Acceptance Criteria**:
- [ ] `Set`, `Remove` and the on-resume lookup take the `Event` type; a plain untyped string literal at a call site no longer compiles unless it is an untyped constant.
- [ ] `grep -rn '"on-resume"' --include='*.go'` finds exactly one declaration site.
- [ ] A stale key whose entry is filed under `on-resume` is reaped with the removed command carried in the `value` attr, exactly as today.
- [ ] A stale key whose entry is filed under a non-`on-resume` event is reaped with a non-empty `value` naming what it held, where today it logs empty.
- [ ] Every existing hooks store, cmd and hookstest suite passes with only the call-site type change.

**Tests**:
- `"it persists and reads back an entry through the Event constant"`
- `"it logs the removed command in the value attr when a stale on-resume entry is reaped"`
- `"it logs a non-empty value when the reaped entry is filed under another event"`
- `"it reports the events a reaped key actually held rather than one assumed name"`

## Task 6: One sweep cycle's log lines are split across three components by an injected logger

severity: medium
sources: architecture, bank

**Problem**: `runHookStaleCleanup` emits one cycle's output through two loggers by design: every stand-down goes out under the `hooks` component via `standDown.emit()` (`cmd/run_hook_stale_cleanup.go:141`), while the cycle's two DEBUG counts go to whatever `countsLogger` names — `daemon` from `maybeRunHookCleanup` (`cmd/state_daemon.go:211`), `bootstrap` from `pruneDoctorStaleHooks` (`cmd/doctor.go:200`). A third slice escapes both: an unclassified sweep error is worded by each caller separately (`daemon: hooks stale-cleanup failed`, `bootstrap: doctor --fix: stale-hook prune failed`). The function's own doc comment names the consequence — "a caller observing this cycle through countsLogger alone sees the counts and none of the stand-downs" — which is precisely the property this work unit exists to remove: the original loss was silent because reconstructing one cycle meant correlating across sources. An operator now needs three greps to reconstruct one 10s sweep, and the closed `clean-stale-skipped` vocabulary built to make declines identifiable covers only the subset routed through `emit()`. The `bootstrap` attribution is doubly odd, since the same phase ruled that a bootstrap step is forbidden from calling the sweep at all.

**Solution**: Drop the `countsLogger` seam and emit the whole cycle — counts, stand-downs and failures alike — under `hooks`, the component that owns the subsystem being swept. The two callers keep whatever cycle-summary line their own component owes and stop wording the sweep's internals in their own vocabulary.

**Outcome**: `grep 'hooks:'` reconstructs one sweep cycle whole, which is what the closed reason vocabulary was built for.

**Do**:
- Drop the `countsLogger` parameter from `runHookStaleCleanup` and `liveTokenEnumeration` (`cmd/run_hook_stale_cleanup.go:252,296`) and delete `countsOrDefault` (`:280`), emitting the `stale-hook cleanup counts` and `stale-hook cleanup removed` DEBUG lines through `hooksLogger` directly.
- Move the sweep's own failure line into the cycle: emit it under `hooks` from `runHookStaleCleanup`'s unclassified path, so both callers stop wording it themselves.
- Update `maybeRunHookCleanup` (`cmd/state_daemon.go:211-213`) to call the one-argument form and drop its `daemon: hooks stale-cleanup failed` WARN.
- Update `pruneDoctorStaleHooks` (`cmd/doctor.go:200-205`) the same way — drop the `bootstrap: doctor --fix: stale-hook prune failed` WARN, keep the `reportSkippedPrune(w, skipReasonSweepFailed)` user-facing line, which is the caller's own output rather than the sweep's internals.
- Update `runHookStaleCleanup`'s doc comment to state the one component the whole cycle emits under, replacing the countsLogger split it currently describes.

**Acceptance Criteria**:
- [ ] One sweep cycle driven from the daemon emits every line — counts, reaped count, stand-downs and failures — with `component=hooks`.
- [ ] The same cycle driven from `portal doctor --fix` emits the identical component and messages; only the caller's rendered stdout differs.
- [ ] No `daemon:` or `bootstrap:` line describes the sweep's internals any more; each caller keeps only its own cycle-summary output.
- [ ] A sweep that fails for an unclassified reason produces exactly one WARN, under `hooks`, and no second line from either caller.
- [ ] `runHookStaleCleanup` takes no logger argument.

**Tests**:
- `"it emits a cycle's counts, stand-down and failure lines all under the hooks component"`
- `"it emits the same component and messages from the daemon and from doctor --fix"`
- `"it emits exactly one record for an unclassified sweep failure"`
- `"it still renders the caller-facing skipped-prune line for a failed sweep"`

## Task 7: `pane_key` names two different values inside one daemon loop body

severity: medium
sources: bank

**Problem**: In the same iteration of the capture loop, `cmd/state_daemon.go:276` and `:295` log `pane_key` as the sanitized scrollback key (`work__0.0`), while `:289` logs `pane_key` as a tmux coordinate (`work:0.0`), recomposed through `tmux.PaneTarget` purely for that line. One attr key in the closed vocabulary, two value shapes, on adjacent lines of one loop — so an operator correlating a failed capture against the scrollback file it should have written has to know which line means which. The split is pinned by an assertion at `cmd/state_daemon_capture_logging_test.go:249`, so it is currently load-bearing.

**Solution**: Settle on one meaning for `pane_key` — the sanitized key, since that is what every other emission and every on-disk artifact uses — and give the failed-capture WARN its coordinate under a differently-named attr, or drop the coordinate where the key already identifies the pane. The closed attr vocabulary gains at most one name, spec-governed like its siblings.

**Outcome**: `pane_key` carries the sanitized scrollback key on every line the capture loop emits, so an operator correlating a failed capture against the file it should have written reads one value shape and needs no per-line rule.

**Do**:
- In `cmd/state_daemon.go:285-290`, log `pane_key` as `paneKey` — the `state.SanitizePaneKey` value already in scope — on the `capture pane failed` WARN, matching `:276` and `:295`.
- Drop the recomposed `coord := tmux.PaneTarget(...)` at `:288` and the comment above it: the sanitized key names the same session, window and pane, so the closed attr vocabulary gains no new name.
- Update the assertion at `cmd/state_daemon_capture_logging_test.go:249` to expect the sanitized key on the failed-capture WARN, and re-voice its case name so it pins one meaning rather than the split.
- Sweep the capture loop for any remaining `pane_key` emission whose value is not a `SanitizePaneKey` result and bring it onto the same value.

**Acceptance Criteria**:
- [ ] Every `pane_key` attr emitted from the daemon capture loop carries a `SanitizePaneKey` value; none carries a `session:window.pane` coordinate.
- [ ] The `capture pane failed` WARN for a given pane carries the same `pane_key` value as that pane's `pane captured` DEBUG and its `write scrollback failed` WARN.
- [ ] The failed-capture WARN still carries its `error` attr unchanged.
- [ ] No new attr key is added to the closed vocabulary.

**Tests**:
- `"it logs the sanitized pane key on a failed capture, not the tmux coordinate"`
- `"it emits one pane_key value shape across the capture, failure and write lines of a single pane"`
- `"it still carries the underlying error on the failed-capture warning"`

## Task 8: The sweep reads `hooks.json` twice and only one read's failure has a reason

severity: low
sources: architecture

**Problem**: `CleanStale` classifies its advisory pre-read failure as `ErrSnapshotRead` (`internal/hooks/store.go:305`), which `declinedSweep` maps to `store-read-failed`. But `deleteStale` performs a second `s.load()` of the same file under its exclusive hold (`internal/hooks/store.go:330`), and that failure is wrapped as a bare `fmt.Errorf("failed to load hooks: %w", …)` with no sentinel — so it falls through `declinedSweep`'s switch to the unclassified default and leaves the closed reason vocabulary entirely, reported instead by whatever words the caller happens to hold. The same file, failing to read for the same reason, is identifiable in one phase and anonymous in the other, which undercuts the premise that every decline is identifiable rather than only the guard cases.

**Solution**: Wrap `deleteStale`'s load failure in the same `ErrSnapshotRead` sentinel (or a sibling `ErrStoreRead` both map through) so both reads of `hooks.json` classify to `store-read-failed`, leaving only genuine save failures on the unclassified path.

**Outcome**: Either read of `hooks.json` failing lands the cycle on `store-read-failed` with the vocabulary's own words, so the only cycles left on the unclassified path are the ones that genuinely failed to write.

**Do**:
- Rename `ErrSnapshotRead` (`internal/hooks/store.go:282`) to a read-phase sentinel both reads share, or add `ErrStoreRead` beside it and have `CleanStale`'s pre-read wrap that; whichever form, one sentinel must satisfy `errors.Is` for both failures.
- Wrap `deleteStale`'s `s.load()` failure (`internal/hooks/store.go:330-332`) in that sentinel, replacing the bare `fmt.Errorf("failed to load hooks: %w", …)`.
- Leave `deleteStale`'s `s.save` failure (`:345-348`) unwrapped by any read sentinel, so a genuine write failure still reaches `declinedSweep`'s unclassified default.
- Confirm `declinedSweep` (`cmd/run_hook_stale_cleanup.go:330-334`) needs no new arm — the existing `hooks.ErrSnapshotRead` case classifies both once they share a sentinel — and update its comment to say the read it names rather than the phase.
- Update the sentinel's doc comment and CLAUDE.md's `hooks` row, which currently describes `ErrSnapshotRead` as the pre-read's alone.

**Acceptance Criteria**:
- [ ] A `hooks.json` unreadable at the pre-read stands the cycle down under `store-read-failed`, as today.
- [ ] A `hooks.json` unreadable only at `deleteStale`'s load — readable when the snapshot was taken, unreadable under the exclusive hold — also stands the cycle down under `store-read-failed`, where today it escapes the vocabulary.
- [ ] A failing save still returns an unclassified error, and `portal doctor --fix` still renders it through `skipReasonSweepFailed`.
- [ ] `errors.Is` against the shared sentinel matches both read failures and neither save failure.

**Tests**:
- `"it classifies a failed pre-read as store-read-failed"`
- `"it classifies a failed delete-phase load as store-read-failed"`
- `"it leaves a failed save on the unclassified path"`
- `"it renders the same stand-down phrase for both read failures on both surfaces"`

## Task 9: The stand-down reason vocabulary is guarded three ways and carries members no surface reaches

severity: low
sources: architecture

**Problem**: Seven `skipReason` constants are backed by two exhaustive phrase maps, an enumerable `skipReasons` slice, an AST guard proving the slice matches the const block, a coverage guard proving both maps are exhaustive and carry no extras, a copy-uniqueness guard with a `notStandDownReasons` subtraction list, **and** a runtime fallback in `phraseFor` returning the raw reason for an unmapped one. The exhaustiveness guard makes the fallback unreachable; the fallback makes the guard optional. The vocabulary is also not internally consistent: `notEvaluableDetails[skipReasonLockTimeout]` is documented in-source as unreachable and present "for vocabulary completeness", and `skipReasonSweepFailed` is a member of a type whose own doc calls it "the closed vocabulary of reasons a cycle removed nothing", whose log op is `clean-stale-skipped`, yet it is never a skip and never logged — which is why the copy guard needs a subtraction list to compensate for a member the type's contract excludes.

**Solution**: Pick one enforcement — keep the exhaustiveness guard and delete `phraseFor`'s fallback branch, so an unmapped reason fails a test rather than printing a raw token. Separately, move `skipReasonSweepFailed` out of `skipReason` (it is a failure the `--fix` renderer reports, not a stand-down) so the copy guard no longer needs its subtraction list.

**Outcome**: `skipReason` holds only reasons a cycle declined under, one enforcement mechanism governs its copy, and the copy-uniqueness guard needs no subtraction list to compensate for a member the type excludes.

**Do**:
- Delete `phraseFor`'s fallback branch (`cmd/run_hook_stale_cleanup.go:105-110`), leaving the map lookup as the whole body, so the exhaustiveness guard is the single enforcement.
- Remove `skipReasonSweepFailed` from the const block and from `skipReasons` (`:29,60`), and drop its rows from `skippedPrunePhrases` and `notEvaluableDetails` (`:87,99`).
- Keep `sweepFailedStandDownPhrase` as a plain const and give `cmd/doctor.go` a dedicated `reportFailedPrune(w)` renderer that prints the byte-identical `Skipped stale hook prune: the sweep could not complete` line, replacing `reportSkippedPrune(w, skipReasonSweepFailed)` at `cmd/doctor.go:203`.
- Delete the `notStandDownReasons` subtraction list from the copy-uniqueness guard in `cmd/doctor_stand_down_copy_test.go` and let it range over `skipReasons` whole.
- Update the `skipReasons` doc block and the type's own doc so they describe the six-member set that remains.

**Acceptance Criteria**:
- [ ] `skipReasons` holds six members and every one of them is a reason a cycle declined under.
- [ ] `phraseFor` has no fallback branch; a reason absent from a map renders empty and the coverage guard fails.
- [ ] The `--fix` output for a failed sweep is byte-identical to today's.
- [ ] The copy-uniqueness guard ranges over `skipReasons` with no subtraction list and passes.
- [ ] Both phrase maps are exhaustive over the six and carry no extras.

**Tests**:
- `"it renders the same failed-sweep line for --fix after sweep-failed leaves the reason type"`
- `"it fails the coverage guard when a declared reason is absent from a phrase map"`
- `"it enumerates every stand-down reason with no subtraction list"`
- `"it renders a distinct phrase for each of the six reasons on both surfaces"`

## Task 10: Stand-down copy is written outside its declared home on three surfaces

severity: low
sources: bank

**Problem**: The phrase tables in `cmd/run_hook_stale_cleanup.go` are meant to be the one place a stand-down's words live, but three surfaces still spell them. (a) The lock phrase is an inline literal at `:86` and `:98` where every other shared phrase composes from a const, which is why `cmd/doctor_stand_down_copy_test.go`'s lock row carries an empty `sharedPhrase` and is skipped by the shared-const subtest. (b) `checkStaleProjects` writes the bare literal `could not read projects.json` at `cmd/doctor.go:339` and `:343` for its two branches, with no reason vocabulary at all on the projects surface and no guard that can see it. (c) `cmd/doctor_test.go` literal-pins the rendered copy at `:968-969`, `:1022-1023`, `:1144`, `:1166-1167`, `:1178-1179` and `:1236-1237`, each a second home for words `cmd/doctor_stand_down_copy_test.go:60-126` owns — so re-wording one phrase is several edits found only by grep.

**Solution**: Give the lock phrase a const beside its siblings and fill in the copy test's `sharedPhrase` for that row; render the `doctor_test` assertions through `phraseFor(notEvaluableDetails, <reason>)` the way `doctor_fix_hook_prune_report_test.go:38` already does, keeping each case's distinct subject; and either give the projects reaper a reason vocabulary of its own or route it through the hooks one, so its copy is reachable by the phrase guards.

**Outcome**: Every stand-down phrase on every surface — hooks and projects alike — is written once in the declared home and reachable by the phrase guards, so a re-wording is one edit rather than a grep.

**Do**:
- Add a `lockStandDownPhrase` const beside its four siblings (`cmd/run_hook_stale_cleanup.go:69-75`) and compose both `skippedPrunePhrases[skipReasonLockTimeout]` (`:86`) and `notEvaluableDetails[skipReasonLockTimeout]` (`:98`) from it, then fill in the lock row's `sharedPhrase` in `cmd/doctor_stand_down_copy_test.go` so the shared-const subtest stops skipping it.
- Replace the literal-pinned copy at `cmd/doctor_test.go:968-969,1022-1023,1144,1166-1167,1178-1179,1236-1237` with `phraseFor(notEvaluableDetails, <reason>)` calls, keeping each case's own subject and expectations otherwise unchanged.
- Give `checkStaleProjects` (`cmd/doctor.go:336-349`) the same treatment as its hooks sibling: a `projectStoreReadStandDownPhrase` const both branches at `:339,343` render through, replacing the two bare literals.
- Bring the projects phrase into the copy and phrase guards' reach so a re-wording of either surface's read-failure line is caught, rather than being invisible to them as the projects copy is today.

**Acceptance Criteria**:
- [ ] No stand-down phrase is an inline literal in `cmd/run_hook_stale_cleanup.go` or `cmd/doctor.go`; every one composes from a const in the declared home.
- [ ] The copy test's lock row carries a non-empty `sharedPhrase` and is exercised by the shared-const subtest rather than skipped.
- [ ] `cmd/doctor_test.go` contains no literal copy of a stand-down phrase; each assertion renders through the phrase tables.
- [ ] The projects reaper's read-failure copy is declared once and reachable by the phrase guards.
- [ ] Every rendered user-facing line is byte-identical to today's.

**Tests**:
- `"it renders the lock stand-down from the shared const on both surfaces"`
- `"it renders every doctor stale-hooks assertion through the phrase tables"`
- `"it renders the projects read-failure line from its declared const"`
- `"it fails the copy guard when a phrase is spelled outside its declared home"`

## Task 11: `internal/hooks`'s read API is inconsistent across its three read entry points

severity: low
sources: architecture

**Problem**: The package offers three ways to read, each shaped differently. `Load(via)` and `List(via)` are methods taking the caller's `Via`. `LookupOnResume(store, hookKey)` is a package-level function taking `*Store` as its first parameter and hardcoding `ViaHydrate` internally — so the one read whose caller is fixed is the one that does not take the parameter, and it is a free function where its siblings are methods despite reaching an unexported method on the receiver it is handed. `StaleKeys(persisted, live)` is an exported function whose entire body is `return staleKeys(persisted, live)` with an identical signature — an exported/unexported twin where one export would do. The shape of a call carries no information about what it does.

**Solution**: Make `LookupOnResume` a method on `*Store` taking a `Via` like its siblings, with `ViaHydrate` supplied by the one caller that means it, and collapse the `StaleKeys`/`staleKeys` pair to the single exported function both `deleteStale` and `checkStaleHooks` call.

**Outcome**: `internal/hooks`'s three read entry points share one shape — a method on `*Store` taking the caller's `Via` — and the staleness rule has one exported name rather than an exported/unexported twin.

**Do**:
- Convert `LookupOnResume` (`internal/hooks/lookup.go:13`) into `func (s *Store) LookupOnResume(hookKey string, via Via) (string, bool, error)`, dropping the hardcoded `ViaHydrate` in favour of the parameter, and keeping the empty-key refusal and the degrade-to-no-hook behaviour exactly as they are.
- Update the hydrate call site to pass `hooks.ViaHydrate` explicitly, and every test caller with it.
- Delete `StaleKeys`'s wrapper body (`internal/hooks/store.go:275-277`), export the implementation at `:243` under the `StaleKeys` name, and re-point `deleteStale` (`:334`) onto it; `checkStaleHooks` (`cmd/doctor.go:329`) already calls the exported name and needs no change.
- Carry the two doc comments onto the surviving declarations, so the staleness rule's statement and the retention rule's rationale live on the one exported function.
- Update CLAUDE.md's `hooks` row where it names `LookupOnResume` and the `StaleKeys`/`staleKeys` pair.

**Acceptance Criteria**:
- [ ] `LookupOnResume` is a method on `*Store` taking a `Via`, and no read entry point resolves a `Via` internally.
- [ ] The hydrate helper's lookup still records `via=hydrate` on its degradation breadcrumb.
- [ ] `internal/hooks` exports exactly one staleness function, and the unexported twin is gone.
- [ ] Every existing behaviour is unchanged: empty key refused before the read, missing or malformed file degrades to "no hook", empty command degrades to "no hook", genuine I/O error returned.

**Tests**:
- `"it returns the registered on-resume command through the store method"`
- `"it records the caller's via on a degraded lookup rather than a hardcoded one"`
- `"it refuses an empty hook key before reading the file"`
- `"it applies the staleness rule through the single exported function from both callers"`

## Task 12: The hook-staleness cycle is a self-contained domain living in `cmd`

severity: low
sources: architecture

**Problem**: `cmd/run_hook_stale_cleanup.go` grew from ~60 lines to 345 and now holds five types (`skipReason`, `standDown`, `stalenessView`, `sweepOutcome`, `declinedError`), a closed reason vocabulary, two user-facing copy tables and the emission policy for a subsystem — none of which is cobra-shaped. Its copy tables sit in this file while both renderers that consume them (`reportSkippedPrune`, `staleHooksNotEvaluable`) live in `doctor.go`, and its component binding is borrowed from `cmd`'s `hooksLogger`. It is reached by exactly two callers, both passing a seam interface rather than a command. The pieces compose, but the policy has outgrown the package it sits in, and that home is why the component question in Task 6 was reachable at all.

**Solution**: Lift the cycle into its own package — the reason vocabulary, `standDown`, `stalenessView`, `sweepOutcome` and `runHookStaleCleanup` — with its own `hooks` component binding, leaving `cmd` holding the two call sites and the rendering of an outcome. The component question then answers itself, and the copy tables sit beside the renderers that print them.

**Outcome**: The hook-staleness policy lives in a package of its own with its own `hooks` component binding, and `cmd` holds two call sites plus the rendering of an outcome — nothing cobra-shaped left behind, nothing non-cobra carried.

**Do**:
- Create `internal/hooksweep` and move the cycle into it: the reason type and its enumerable set, `standDown` with `declineDebug`/`declineWarn`/`emit`, `stalenessView`, `sweepOutcome`, `declinedError`, `errNothingPersisted`, `hookStalenessStandDown`, `judgeAgainstLivePanes`, `liveTokensFrom`, `liveTokenEnumeration`, `declinedSweep`, `standDownOutcome` and `runHookStaleCleanup`, plus the `staleSweepReader`/`PaneHookLister` seam interfaces.
- Bind `log.For("hooks")` in the new package — the same per-package binding rule the `theme` component already spans two packages under — and delete the borrowed `hooksLogger` reference from the moved code.
- Export the surface the two callers need — the reason type, its constants, the enumerable set, the outcome type and the entry point — and keep everything else unexported.
- Leave `cmd` holding `pruneDoctorStaleHooks`, `maybeRunHookCleanup`, `reportSkippedPrune`, `staleHooksNotEvaluable`, `phraseFor` and both copy tables, re-keyed onto the exported reason type so the tables sit beside the renderers that print them.
- Move the cycle's own unit suites into the new package and leave the `cmd` suites that assert on rendered output where they are; add a leaf-style deps guard if the new package's dependency set warrants one.

**Acceptance Criteria**:
- [ ] `cmd/run_hook_stale_cleanup.go` no longer exists; `cmd` holds only the two call sites and the outcome rendering.
- [ ] The new package binds the `hooks` component itself and emits the whole cycle under it.
- [ ] Both copy tables and `phraseFor` sit in `cmd` beside `reportSkippedPrune` and `staleHooksNotEvaluable`, keyed on the exported reason type.
- [ ] The daemon's throttled sweep and `portal doctor --fix` produce byte-identical output and byte-identical log lines to before the move.
- [ ] Every existing stand-down, phrase, coverage and copy guard passes, re-pointed at the new home.

**Tests**:
- `"it runs one sweep cycle from the new package and reports what it removed"`
- `"it stands the cycle down under each reason from the new package"`
- `"it emits the whole cycle under the hooks component from the new package's own binding"`
- `"it renders the same doctor --fix output after the move"`

## Task 13: Two production comment blocks carry the design argument and cardinality claims the project's comment standard forbids

severity: comments
sources: standards

**Problem**: `code-quality.md` names two things a comment must never carry — the design argument ("State the conclusion the code needs, not the debate; the reasoning lives in the project's design artifacts") and cardinality claims ("the single caller", "the only site that…", "nothing consumes this yet"). `snapshotLockBound` (`internal/hooks/lock.go:29-61`) is a three-line function under a ~33-line comment reproducing the whole of the specification's corrigendum argument — the two-bounds justification, the headroom-versus-thousandth comparison, the behaviour across three bound ranges, and a floor rationale stated twice in the same block. The `skipReasons` block (`cmd/run_hook_stale_cleanup.go:32-52`) asserts cardinality outright — "sweep-failed is the --fix repair's line alone", "lock-timeout cannot reach the read-only diagnosis at all" — both falsifiable by ordinary additive change far from the comment. `cmd/hooks.go:81-83`'s "A new seam costs one fill line here rather than a builder of its own" is the same class, smaller.

**Solution**: Reduce `snapshotLockBound`'s comment to the conclusions the code needs — the pre-read is advisory and may degrade, the bound is derived from `lockTimeout` so lowering one lowers both, and the floor of one poll interval is load-bearing because the acquire re-tests its deadline only after a sleep — and let the specification hold the argument. Strip the two cardinality claims from `skipReasons`, keeping a reachability fact only where it changes what a reader must do. Drop the `hookSeams` aside.

**Outcome**: The three comment blocks carry only conclusions the code needs, with no design argument and no cardinality claim ordinary additive change would falsify.

**Do**:
- Reduce `snapshotLockBound`'s comment (`internal/hooks/lock.go:29-58`) to three conclusions — the pre-read is advisory and may degrade to an unlocked read, the bound is derived from `lockTimeout` so lowering one lowers both, and the floor of one poll interval is load-bearing because the acquire re-tests its deadline only after a sleep — dropping the two-bounds justification, the headroom-versus-thousandth comparison, the three-bound-ranges walkthrough and the duplicated floor rationale.
- Strip the two cardinality claims from the `skipReasons` block (`cmd/run_hook_stale_cleanup.go:32-52`): "sweep-failed is the --fix repair's line alone" and "lock-timeout cannot reach the read-only diagnosis at all", keeping only the fact that changes what a reader must do — that the lock reason's not-evaluable phrase exists for vocabulary completeness.
- Delete the "A new seam costs one fill line here rather than a builder of its own" sentence from `hookSeams`'s doc comment (`cmd/hooks.go:81-83`), leaving the sentence that says what the function returns.
- Change no code and no test in this task: the edits are comment text alone.

**Acceptance Criteria**:
- [ ] `snapshotLockBound`'s comment states the three conclusions and no argument; the function body is untouched.
- [ ] The `skipReasons` block carries no claim about how many surfaces reach a reason.
- [ ] `hookSeams`'s doc comment carries no cardinality aside.
- [ ] No non-comment byte changes in the three files; the full unit and integration lanes pass unchanged.

**Tests**: comment-only edit — no behaviour changes and no test semantics change.
- `internal/hooks`'s lock and read-lock suites stay green with no edit, including the bound-relation pins in `read_lock_test.go`.
- `cmd`'s stand-down phrase, coverage and copy guards stay green with no edit.
- `cmd`'s hook-seam suites stay green with no edit.

## Task 14: `(*Client).SendKeys` is exported on the production client with no production caller

severity: medium
sources: bank

**Problem**: `internal/tmux/tmux.go:658-663` exports `SendKeys` solely for test consumption and says so in its doc comment — "It has no production caller and is exported anyway" — which is exactly the cardinality claim `code-quality.md` forbids, and a comment that must be deleted in the same edit as the first production caller that ever appears. The driver is consumed from `cmd/bootstrap/eager_signal_hydrate_integration_test.go:154,237`, `internal/restore/exit_closes_pane_integration_test.go:36,59` and `internal/tmux/tmux_test.go`. `internal/tmuxtest` already owns the per-test `-L`/`-S` socket and hands out `ts.Client()`, so it can compose the same `send-keys -t <target> <cmd> Enter` against its own socket with no access to the client's unexported fields.

**Solution**: Relocate the driver to `internal/tmuxtest` as a `Socket` method and drop `SendKeys` from the production client, retiring both the export with no production caller and the comment that has to explain it.

**Outcome**: The production tmux client exports nothing without a production caller, and the send-keys driver lives in the harness that already owns the socket it runs against.

**Do**:
- Add `func (s *Socket) SendKeys(t *testing.T, target, keys string)` to `internal/tmuxtest/socket.go`, composing `send-keys -t <target> <keys> Enter` through the existing `socketArgs`/`cmd` path so it runs on the test's own socket.
- Delete `(*Client).SendKeys` (`internal/tmux/tmux.go:658-663`) and its doc comment.
- Re-point the three consumers onto the new method: `cmd/bootstrap/eager_signal_hydrate_integration_test.go:154,237`, `internal/restore/exit_closes_pane_integration_test.go:36,59` and `internal/tmux/tmux_test.go`.
- Where `internal/tmux/tmux_test.go`'s case is about the argv the client composes rather than about driving a real pane, drop it with the method rather than relocating it.

**Acceptance Criteria**:
- [ ] `internal/tmux` exports no `SendKeys`, and `grep -rn 'SendKeys' internal/tmux` finds nothing.
- [ ] `tmuxtest.Socket.SendKeys` sends against the fixture's own `-S` socket with `-f /dev/null`, never the ambient `TMUX`.
- [ ] The three consuming integration tests pass unchanged in behaviour.
- [ ] No production file references the driver.

**Tests**:
- `"it sends keys to a live pane on the fixture's own socket"`
- `"it appends Enter so the pane runs the command"`
- `"it drives the eager-signal-hydrate and exit-closes-pane fixtures through the harness method"`

## Task 15: `session.BuildShellCommand` leaves the leading shell word unquoted

severity: medium
sources: bank

**Problem**: `internal/session/create.go:31` renders `fmt.Sprintf("%s -ic %s", shell, shellquote.Single(script))` — the script is quoted through the shared leaf, but the first `%s` is interpolated raw. A `$SHELL` of `/My Apps/zsh` composes `/My Apps/zsh -ic '…'`, which tmux hands to a shell that word-splits it into `/My`, and the session's first command dies on a path that does not exist. `internal/shellquote` exists precisely to declare this rule once and the fix is a one-token change, but it is a behaviour change (the composed string differs for every shell path, quoted or not), so the task that extracted the leaf correctly refused to make it under a byte-identity criterion.

**Solution**: Route the leading shell word through `shellquote.Single` alongside the script, so the whole composition is well-formed for any `$SHELL` path.

**Outcome**: A `$SHELL` whose path contains a space (or a quote) composes a session command the shell runs as one word, instead of dying on a fragment of the path.

**Do**:
- In `internal/session/create.go:31`, quote the leading shell word too: render `fmt.Sprintf("%s -ic %s", shellquote.Single(shell), shellquote.Single(script))`.
- Leave the `exec ` + `shell` interpolation inside `script` (`:30`) as it is only if it is already safe; if it is not, route that occurrence through `shellquote.Single` in the same edit so the inner `exec` target survives word-splitting too.
- Update the existing byte-identity expectations in `internal/session`'s `BuildShellCommand` suite to the newly-quoted composition — the composed string now differs for every shell path, which is the deliberate behaviour change this task makes.
- Leave `internal/shellquote` untouched: the rule is already declared there and this task only routes through it.

**Acceptance Criteria**:
- [ ] `BuildShellCommand([...], "/My Apps/zsh")` renders the shell as one quoted word, and the composition run through a shell invokes `/My Apps/zsh` rather than `/My`.
- [ ] A shell path with no metacharacters still composes a working command; only its quoting changes.
- [ ] A single quote inside either the shell path or the command is re-quoted so the outer quoting survives.
- [ ] An empty command still returns the empty string with no quoting applied.
- [ ] The `exec $SHELL` tail refers to the same shell the leading word names.

**Tests**:
- `"it quotes a shell path containing a space so it survives word-splitting"`
- `"it quotes a shell path containing a single quote"`
- `"it still renders a metacharacter-free shell path as a working command"`
- `"it returns the empty string for an empty command"`

## Task 16: `cmd/uninstall.go` substring-matches tmux stderr to detect an absent session

severity: medium
sources: bank

**Problem**: `isSessionAbsentError` (`cmd/uninstall.go:109-111`) does `strings.Contains(strings.ToLower(err.Error()), "can't find session")` on a `KillSession` error. `internal/tmux/errors.go` states the rule this breaks: layers above `internal/tmux` must not substring-match tmux stderr, because tmux's phrasing is not a stable contract. `KillSession` now carries `ErrNoSuchSession`, so the check collapses to `errors.Is`. It also has a live wrong answer today: an unaddressable saver name produces the same "can't find session" stderr as a vanished one, so the substring match reports a **live** session as absent and `portal uninstall` logs a clean removal for a saver it did not kill.

**Solution**: Replace the substring match with `errors.Is(err, tmux.ErrNoSuchSession)`, which the wrapping already provides and which the unaddressable-name classification in `wrapSessionTargetErr` deliberately keeps distinct.

**Outcome**: `portal uninstall` reports a saver it did not kill as a failure rather than as a clean removal, and no layer above `internal/tmux` reads tmux's stderr wording.

**Do**:
- Replace `isSessionAbsentError`'s body (`cmd/uninstall.go:109-111`) with `errors.Is(err, tmux.ErrNoSuchSession)`, or inline the check at `:96` and delete the helper along with its substring-stability comment.
- Drop the now-unused `strings` import if nothing else in the file needs it.
- Keep the two outcomes distinct at `killSaver`: a genuinely absent session still logs `killSaverInfoMessage` and returns nil, while any other kill failure — an unaddressable name among them — still WARNs and returns the error.
- Add a case pinning the live wrong answer: a saver name `ValidateSessionName` refuses produces the same tmux stderr as a vanished session and must now be reported as a failure, not a removal.

**Acceptance Criteria**:
- [ ] `portal uninstall` treats a `KillSession` error wrapping `tmux.ErrNoSuchSession` as an absent saver and exits reporting a clean removal.
- [ ] A `KillSession` error carrying "can't find session" stderr but classified as an unaddressable name is reported as a failure, and no removal is logged.
- [ ] `cmd/uninstall.go` contains no `strings.Contains` over an error string.
- [ ] An arbitrary non-absence kill failure still WARNs and returns the error.

**Tests**:
- `"it reports a clean removal when the saver session is genuinely absent"`
- `"it reports a failure when the kill failed on an unaddressable saver name"`
- `"it surfaces any other kill failure as an error"`
- `"it matches on the sentinel rather than tmux's stderr wording"`

## Task 17: The four exact-target forms return a bare string, so only a name-based source scan tells a pinned target from a hand-composed one

severity: medium
sources: bank

**Problem**: `SessionTargetExact`, `CoordTargetExact`, `PaneTargetExact` and `windowTargetExact` all return a plain `string`, so a hand-composed target is assignable wherever a pinned one is, and the whole rule rests on `internal/tmux/target_composition_guard_test.go` — a source scan matching parameter names and callee names. `SessionTargetExact`'s own doc comment records the settled answer in-source: "A named `type Target string` returned by these four and taken by every `-t` parameter is the settled answer: it moves the rule into the type system, where a laundered or reassigned target cannot pass. Landing it is a signature change across the client's whole surface and its callers in cmd, internal/state and internal/restore — deferred for that reach, not because the string form is preferred." A sibling task recorded the same type-over-convention move as pending. The guard's own history shows the cost of leaving it: it has already been widened twice this phase for shapes it could not see.

**Solution**: Land the deferred `type Target string`: the four constructors return it, every `-t`-taking client method and every cross-package function holding an already-composed target takes it, and the source guard reduces to whatever the type cannot express. The reach is the client's whole surface plus its callers in `cmd`, `internal/state` and `internal/restore`, which is why it is a task of its own rather than a rider on a guard change.

**Outcome**: A hand-composed tmux target no longer type-checks where a pinned one is required, so the composition rule is enforced by the compiler rather than by a source scan that has been widened twice for shapes it could not see.

**Do**:
- Declare `type Target string` in `internal/tmux` and change `SessionTargetExact`, `CoordTargetExact`, `PaneTargetExact` and `windowTargetExact` to return it; leave the unpinned `PaneTarget`/`windowTarget` returning plain strings, since those are for display and name-based keys.
- Change every `-t`-taking client method to accept `Target` (or to compose one internally from a name it is given), and convert at the single `c.cmd.Run` boundary in each.
- Re-point the cross-package holders of an already-composed target in `cmd`, `internal/state` and `internal/restore` onto `Target`, including the hand-composed argv sites at `cmd/open.go:88` and `internal/session/quickstart.go:62`.
- Reduce `internal/tmux/target_composition_guard_test.go` to the residue the type cannot express — a target built by string concatenation inside `internal/tmux` itself, say — and delete the arms the type now covers, including the widenings added for the method/function shapes.
- Replace `SessionTargetExact`'s doc paragraph recording the deferral with what the type now guarantees, and update CLAUDE.md's `tmux` row to describe the typed form.

**Acceptance Criteria**:
- [ ] A plain `string` no longer satisfies a `-t` parameter anywhere in `internal/tmux`'s exported surface; the compiler rejects a hand-composed target.
- [ ] All four exact-target constructors return `Target`, and `PaneTarget`/`windowTarget` still return `string`.
- [ ] Every composed argv is byte-identical to before the change — the type is a compile-time property with no runtime effect.
- [ ] The source guard no longer restates any rule the type expresses, and what remains of it passes.
- [ ] `cmd`, `internal/state`, `internal/restore` and `internal/session` compile and pass with no behaviour change.

**Tests**:
- `"it renders each exact target form to the same string as before the type change"`
- `"it composes byte-identical argv for every -t-taking client method"`
- `"it fails the reduced source guard when a target is concatenated inside the tmux package"`
- `"it pins a hand-composed exec-chain target through the typed constructor"`

## Task 18: The repo-wide source-scan preamble is re-authored at every guard

severity: duplication
sources: duplication, bank

**Problem**: The same ~28-line opening — `portalbintest.ProjectRoot()` with a "resolve project root" fatal, `sourceguardtest.GoSourceFiles(root)` with an "enumerate .go files" fatal, a `_test.go` suffix filter, `sourceguardtest.ParseSources`, and a per-file relativisation so findings read as repo paths — stands at `internal/portalbintest/lane_guard_test.go:55-88`, `internal/portaltest/teardown_guard_coverage_test.go:161-193`, `internal/restoretest/literal_guard_scan_test.go:29-58`, `internal/logtest/install_guard_test.go:57-72`, `internal/theme/loader_construction_guard_test.go:18-38` and `internal/prefs/appearance_api_guard_test.go:28-46`, the first two near line-for-line identical. The relativisation has already drifted three ways: two `relTo*` helpers are declared privately (`internal/logtest/install_guard_test.go:170`, `internal/theme/slug_collapse_guard_test.go:52`), `filepath.Rel` is inlined at seven more sites (`internal/portalbintest/lane_guard_test.go:74`, `internal/prefs/appearance_api_guard_test.go:42`, `internal/log/discard_guard_test.go:38`, `internal/log/migration_guard_test.go:41`, `internal/portaltest/teardown_guard_coverage_test.go:180`, `internal/tui/restore_source_guard_test.go:103,182`, `internal/tui/theme_source_guard_test.go:28`), and one guard uses `strings.TrimPrefix(finding, root+separator)` instead — two behaviours for one rule. Beneath the repo-wide form sits the same skeleton narrowed to a package or a file: `internal/theme` declares three identical six-line `find-the-package-source-named-X` helpers (`badge_test.go:238`, `resolution_test.go:351`, `setting_test.go:601`), and roughly a dozen further guards re-author an enumerate→parse loop under their own parse mode. `internal/sourceguardtest` is the declared home for exactly these primitives and was extended twice by this work unit without absorbing the driver every consumer writes on top of them.

**Solution**: Promote the driver into `internal/sourceguardtest` as one entry point — a `RepoSources(t, …)` returning the resolved root alongside `[]ParsedSource` already narrowed to test or non-test files with root-relative `Path` set — and re-point the guards onto it, retiring the two private `relTo*` helpers, the inline `filepath.Rel` sites, the `TrimPrefix` variant and the three `internal/theme` twins. The scanned-nothing tripwire then has one home rather than one per guard, and the parse mode stops varying by site.

**Outcome**: The repo-wide source-scan driver is declared once in the package that owns the primitives, so a guard states which lane it polices and nothing else, and root-relativisation has one behaviour rather than two.

**Do**:
- Add `RepoSources(t harnesstest.TestingT, sel Selection) (root string, sources []ParsedSource)` to `internal/sourceguardtest`, composing `portalbintest.ProjectRoot` → `GoSourceFiles` → the test/non-test narrowing → `ParseSources`, and setting each `ParsedSource.Path` root-relative through one `filepath.Rel` with a fatal on failure; keep `ParseSources`'s scanned-nothing fatal as the single tripwire.
- Re-point the six repo-wide guards onto it: `internal/portalbintest/lane_guard_test.go:55-88`, `internal/portaltest/teardown_guard_coverage_test.go:161-193`, `internal/restoretest/literal_guard_scan_test.go:29-58`, `internal/logtest/install_guard_test.go:57-72`, `internal/theme/loader_construction_guard_test.go:18-38` and `internal/prefs/appearance_api_guard_test.go:28-46`.
- Delete the two private `relTo*` helpers (`internal/logtest/install_guard_test.go:170`, `internal/theme/slug_collapse_guard_test.go:52`), the `strings.TrimPrefix(finding, root+separator)` variant, and the seven inline `filepath.Rel` sites listed in the Problem, re-pointing each onto the driver's relative `Path`.
- Add a package-narrowed sibling for the same shape at a single directory and collapse `internal/theme`'s three identical six-line `find-the-package-source-named-X` helpers (`badge_test.go:238`, `resolution_test.go:351`, `setting_test.go:601`) onto it.
- Re-point the roughly-a-dozen guards that re-author an enumerate→parse loop under their own parse mode, so `ParseMode` is applied at one place.

**Acceptance Criteria**:
- [ ] `internal/sourceguardtest` exports one repo-wide scan entry point, and `portalbintest.ProjectRoot` is called from exactly one place across the guard family.
- [ ] Findings from every re-pointed guard read as root-relative repo paths, produced by one implementation.
- [ ] No guard declares its own relativisation helper, inline `filepath.Rel` or `TrimPrefix` variant.
- [ ] Each re-pointed guard still fails on exactly the violations it failed on before, with its own wording preserved.
- [ ] The scanned-nothing tripwire fires from one place, and a driver handed an empty tree is fatal.

**Tests**: pure refactor — the guards' verdicts and wordings are unchanged; the shared driver gains its own coverage.
- `"it returns root-relative paths for every parsed source"`
- `"it narrows to test sources or to non-test sources as asked"`
- `"it fatals when the selection matches no source at all"`
- Every re-pointed guard's existing suite stays green with no assertion edited.

## Task 19: The build-tag question has three readers, and every leaf guard but one is blind to it

severity: medium
sources: duplication, bank

**Problem**: Two halves of one gap. First, build-constraint extraction from an `*ast.File` is implemented three times in three packages — `compiledInUnitLane` (`internal/portalbintest/lane_guard_test.go:112-128`), `isIntegrationTagged` (`internal/restoretest/session_restorer_literal_guard_test.go:174-188`) and `buildConstraintLine` (`internal/sourceguardtest/leaf_guard_test.go:45-58`) each walk `file.Comments`, break at `group.Pos() > file.Package` and run `constraint.Parse`, all three then evaluate or report the `integration` tag, and all three disagree on the details: one treats an unparseable constraint as in-lane, one skips a parse failure and keeps looking, one returns the first parseable comment whatever it says. Three readings of one file can classify it three ways, and the tag literal is declared separately in two of them. Second, `AssertDepsWithin` resolves through `go list -deps` under the default tags (`internal/sourceguardtest/packagedeps.go:55-63`), so a build-tagged file can hide a forbidden dependency from **any** leaf guard — proven silent for `sourceguardtest`, and nothing about the mechanism is package-specific. Only `internal/sourceguardtest` has the untagged companion check; `internal/nanoid`, `internal/shellquote`, `internal/harnesstest`, `internal/xdg`, `internal/prefs`, `internal/hooks` and `internal/theme` leaf guards are all blind the same way.

**Solution**: Add one build-constraint primitive to `internal/sourceguardtest` — a reader returning the file's parsed constraint plus a tag evaluator, with the `integration` tag name declared once beside it — and route the three copies through it, each keeping only its own policy. Give `AssertDepsWithin` the tag-awareness option the untagged companion check hand-rolls, and apply it across the seven leaf guards so a tagged file cannot hide a dependency from any of them.

**Outcome**: One reading of a file's build constraint answers for every guard that asks, and a build-tagged file can no longer hide a forbidden dependency from any leaf guard in the tree.

**Do**:
- Add a build-constraint primitive to `internal/sourceguardtest`: a reader over `*ast.File` returning the parsed `constraint.Expr` and whether one was found, plus a `SatisfiedWith(expr, tags...)` evaluator, with the `integration` tag name declared once beside them; settle the unparseable-constraint case in the primitive so the three current disagreements collapse to one answer.
- Route `compiledInUnitLane` (`internal/portalbintest/lane_guard_test.go:112-128`), `isIntegrationTagged` (`internal/restoretest/session_restorer_literal_guard_test.go:174-188`) and `buildConstraintLine` (`internal/sourceguardtest/leaf_guard_test.go:45-58`) through it, each keeping only the lane it polices; delete the two separate `"integration"` literals.
- Add a `DepsOption` to `AssertDepsWithin`/`PackageDeps` that passes build tags through to `go list -deps` (`internal/sourceguardtest/packagedeps.go:55-63`), so a guard can resolve the tagged configuration as well as the default one.
- Apply the tag-aware assertion across the seven leaf guards — `internal/nanoid`, `internal/shellquote`, `internal/harnesstest`, `internal/xdg`, `internal/prefs`, `internal/hooks` and `internal/theme` — and retire `internal/sourceguardtest`'s hand-rolled untagged companion check in favour of the option.

**Acceptance Criteria**:
- [ ] Build-constraint extraction is implemented once; the three former copies call it and hold only their own policy.
- [ ] Three readings of the same file now classify it identically, including a file whose constraint fails to parse.
- [ ] The `integration` tag name is declared once in the tree's guard family.
- [ ] Each of the seven leaf guards fails when a forbidden dependency is reachable only from an `integration`-tagged file in that package.
- [ ] `internal/sourceguardtest`'s own leaf guard keeps its coverage with the hand-rolled companion check removed.

**Tests**: pure refactor of the readers plus a widened dependency assertion; the guards' policies are unchanged.
- `"it returns the parsed build constraint for a tagged file and reports none for an untagged one"`
- `"it evaluates the integration tag against a parsed constraint"`
- `"it classifies an unparseable constraint the same way for every caller"`
- `"it reports a forbidden dependency reachable only from an integration-tagged file"`

## Task 20: The two restore-`Exe` literal guards are a copy-paste pair that each walk the repo

severity: duplication
sources: duplication, bank

**Problem**: `orchestratorLiteralsIn` (`internal/restoretest/orchestrator_literal_guard_test.go:151`) and `sessionRestorerLiteralsIn` (`internal/restoretest/session_restorer_literal_guard_test.go:190`) are identical bodies differing only in the type-name constant they pass to `isRestorePkgType`. `scanTestOrchestratorLiterals` and `scanIntegrationSessionRestorerLiterals` are identical apart from the include filter and the wording of the scanned-nothing fatal. `TestOrchestratorLiteralGuard_FatalsWhenItEnumeratesNoTestFiles` and `TestSessionRestorerLiteralGuard_FatalsWhenItEnumeratesNoIntegrationTestFiles` are the same ~25-line test twice. The session-restorer half was written a task later than the orchestrator half against the shared `scanGuardTestFiles` seam and reproduced everything above it rather than parameterising it — and because each calls `scanGuardTestFiles` separately, the repo-wide walk and parse run twice per unit-lane run of the package.

**Solution**: Collapse the pair into one parameterised guard — a `literalGuard{typeName, include, constructors, fatalWording}` descriptor driving a single scan that owns the scanned-zero fatal and a single composite-literal collector, with the two guards and the two fatals-when-empty tests table-driven over the two descriptors, and one walk feeding both.

**Outcome**: The two restore-`Exe` literal guards are one parameterised guard over two descriptors, and the repo-wide walk and parse run once per unit-lane run of the package rather than twice.

**Do**:
- Declare a `literalGuard{typeName, include, constructors, fatalWording}` descriptor in `internal/restoretest` and instantiate it twice — the orchestrator and the session-restorer types — carrying each half's include filter and its own scanned-nothing wording.
- Collapse `orchestratorLiteralsIn` (`orchestrator_literal_guard_test.go:151`) and `sessionRestorerLiteralsIn` (`session_restorer_literal_guard_test.go:190`) into one composite-literal collector taking the type name from the descriptor.
- Collapse `scanTestOrchestratorLiterals` and `scanIntegrationSessionRestorerLiterals` into one scan that owns the scanned-zero fatal and takes its wording from the descriptor.
- Make the two guards and the two `…FatalsWhenItEnumeratesNo…` tests table-driven over the descriptor pair, keeping each case's `harnesstest.Recorder` assertions and its `"stopped looking"` check.
- Take the repo walk once — through the shared driver — and feed both descriptors from that one result rather than calling `scanGuardTestFiles` twice.

**Acceptance Criteria**:
- [ ] One collector, one scan and one fatals-when-empty test body serve both descriptors.
- [ ] Each descriptor's scanned-nothing fatal keeps its own wording, including the `"stopped looking"` phrase both tests check for.
- [ ] Both guards still fail on exactly the unpinned-`Exe` literals they failed on before, for their own type and their own file set.
- [ ] The package's unit-lane run performs one repo walk and parse for the pair.

**Tests**: pure refactor — both guards' verdicts and fatal wordings are unchanged.
- `"it fails for an unpinned Exe literal of either restore type"`
- `"it fatals when a descriptor's file set enumerates nothing"`
- `"it keeps each descriptor's own scanned-nothing wording"`
- `"it walks the repo once for both descriptors"`

## Task 21: The restore-`Exe` rationale paragraph is restated six times

severity: comments
sources: duplication

**Problem**: The same explanation — `Exe` falls back to `os.Executable()`, which under `go test` is the test binary, so an armed pane re-runs the suite inside itself, exits 0 and takes the session with it — is written out in full at `internal/restoretest/orchestrator.go:11-19`, `orchestrator_staged.go:14-22`, `session_restorer_staged.go:11-20`, `orchestrator_literal_guard_test.go:22-31`, `session_restorer_literal_guard_test.go:21-32` and `restoretest.go:55-58`, plus twice more in the two guards' `t.Fatalf` messages. It is one fact about one field, and each restatement is an independent claim that has to be kept true; the wordings have already diverged ("re-runs the suite inside itself" / "re-runs its own suite inside the pane" / "respawns into the suite itself").

**Solution**: State the trap once, on `StagedHydrateExe` — the helper every pinned route ends at — and have the four constructor and guard doc comments say only what their own site does, keeping the guards' user-facing `t.Fatalf` text as the one other place the consequence is spelled out.

**Outcome**: One fact about one field is stated once, on the helper every pinned route ends at, with each other site saying only what that site does.

**Do**:
- Write the full statement on `StagedHydrateExe` — that `Exe` falls back to `os.Executable()`, which under `go test` is the test binary, so an armed pane re-runs the suite inside itself, exits 0 and takes the session with it.
- Reduce `internal/restoretest/orchestrator.go:11-19`, `orchestrator_staged.go:14-22` and `session_restorer_staged.go:11-20` to what each constructor itself does, pointing at `StagedHydrateExe` rather than restating its reasoning.
- Reduce `orchestrator_literal_guard_test.go:22-31` and `session_restorer_literal_guard_test.go:21-32` the same way, and drop `restoretest.go:55-58`'s restatement.
- Keep both guards' `t.Fatalf` text as written: it is user-facing, read by whoever trips the guard, and is the one other place the consequence is spelled out.
- Change no code and no assertion: the edits are comment text alone.

**Acceptance Criteria**:
- [ ] The `Exe`/`os.Executable()` trap is stated in full at exactly one place in `internal/restoretest`.
- [ ] The five reduced comments each describe only their own site and point at the shared statement.
- [ ] Both guards' `t.Fatalf` wordings are unchanged.
- [ ] No non-comment byte changes; the package's unit and integration suites pass unchanged.

**Tests**: comment-only edit — no behaviour changes and no test semantics change.
- Both restore-`Exe` literal guards stay green with no assertion edited, including their `"stopped looking"` checks.
- `internal/restoretest`'s staged-hydrate-exe suite stays green with no edit.
- The restore reboot and roundtrip integration fixtures stay green with no edit.

## Task 22: The Cobra root-command drive is written four ways in the `cmd` test suite

severity: duplication
sources: duplication, bank

**Problem**: `runRootCmd` (`cmd/root_test.go:92-101`) is the package's shared driver, introduced this phase as "one root-command driver". `runHookSet`, `runHookRm` and `runHookList` (`cmd/testhelpers_test.go:176,189,202`) each repeat its body verbatim — new buffer, `resetRootCmd()`, `SetOut`, `SetErr`, `SetArgs`, `Execute` — differing only in the argv they build and in routing both streams to one buffer. `cmd/hooks_test.go` inlines the same four-line sequence 20 times while calling the purpose-built drivers 13 times in the same file, and the shape recurs package-wide: 192 `rootCmd.Execute()` calls across 30 files, nine of which are literally `runRootCmd(t, args…)` after un-substitution (`cmd/bootstrap_orchestrator_test.go:164-166`, `cmd/root_test.go:498-500`, `cmd/version_guard_test.go:153-155`, `cmd/state_test.go:155,171,201`, `cmd/state_hydrate_empty_hookkey_test.go:26-30`, `cmd/state_signal_hydrate_test.go:374,404`). Every sibling hook suite added by this work unit routes exclusively through the drivers, which makes `hooks_test.go` the drifted copy rather than the convention.

**Solution**: Reimplement `runHookSet`/`runHookRm`/`runHookList` as thin argv-composing wrappers over `runRootCmd` — adding a combined-stream variant there if the single-buffer return is load-bearing — and convert the inline sequences that are exact matches onto the driver, starting with the 20 in `cmd/hooks_test.go` and the nine identified above, so the package has one route to running a command.

**Outcome**: The `cmd` suite has one route to running a Cobra command; the hook drivers compose argv and nothing else, and `hooks_test.go` follows the same convention as every sibling hook suite.

**Do**:
- Add a combined-stream variant beside `runRootCmd` (`cmd/root_test.go:92-101`) — the same body routing both streams to one buffer — so a caller that needs the merged output has a shared route to it.
- Reimplement `runHookSet`, `runHookRm` and `runHookList` (`cmd/testhelpers_test.go:176,189,202`) as argv-composing wrappers over that variant, keeping each one's existing return shape and `runHookList`'s fatal-on-non-zero behaviour.
- Convert the 20 inline drive sequences in `cmd/hooks_test.go` (lines 46-49, 66-69, 109-112, 290-293, 311-314, 335-339, 359-363, 379-382, 386-389, 406-409, 463-466, 485-489, 505-509, 527-530, 556, 584, 611, 680, 703, 911) onto the purpose-built drivers.
- Convert the nine exact-match inline sequences outside that file onto `runRootCmd`: `cmd/bootstrap_orchestrator_test.go:164-166`, `cmd/root_test.go:498-500`, `cmd/version_guard_test.go:153-155`, `cmd/state_test.go:155,171,201`, `cmd/state_hydrate_empty_hookkey_test.go:26-30` and `cmd/state_signal_hydrate_test.go:374,404`.
- Leave in place any inline sequence that is not an exact match — one that sets a stream to something other than a fresh buffer, or that drives without `resetRootCmd` — and name it where it sits rather than forcing it onto the driver.

**Acceptance Criteria**:
- [ ] No hook driver repeats `runRootCmd`'s body; each composes argv and delegates.
- [ ] `cmd/hooks_test.go` contains no inline `resetRootCmd`/`SetOut`/`SetErr`/`SetArgs`/`Execute` sequence.
- [ ] The nine identified sequences outside that file route through the shared driver.
- [ ] Every converted assertion reads the same streams it read before — a caller that merged both still gets one buffer.
- [ ] The whole `cmd` suite passes with no assertion's expectation changed.

**Tests**: pure refactor — no command behaviour or assertion semantics change.
- `cmd/hooks_test.go`'s cases stay green with only their drive lines rewritten.
- `cmd/hooks_rm_exit_test.go`, `hooks_write_lock_test.go`, `hooks_read_lock_test.go` and `hooks_seams_test.go` stay green with no edit.
- The nine converted sites' suites stay green with only their drive lines rewritten.
- `"it captures both streams in one buffer through the combined-stream driver"`

## Task 23: `hookstest.AssertDegradedRead` restates the shared record assertion its file-neighbour delegates to

severity: duplication
sources: duplication

**Problem**: `AssertLockWarn` (`internal/hookstest/hooks_lock.go:107-131`) routes its level, message, component, op and via checks through `logtest.AssertRecord` and adds only the attrs specific to the lock WARN. `AssertDegradedRead`, ten lines below it in the same file, hand-rolls the level, op and via comparisons instead — the same three checks `AssertRecord` owns, with its own `t.Errorf` wordings — and consequently never asserts the message or the component at all. `cmd/logging_capture_test.go:51`'s `assertHooksRecord` shows the intended shape, so this is one site out of step rather than an accepted second route.

**Solution**: Rewrite `AssertDegradedRead` to call `logtest.AssertRecord` with the degraded read's `RecordWant`, keeping only the non-empty `error` attr check locally, so the `load-unlocked` breadcrumb is pinned by the same assertion as every other audit-trail line.

**Outcome**: The `load-unlocked` breadcrumb is pinned by the same five-property assertion as every other audit-trail line, so its message and component are checked where today they are not.

**Do**:
- Rewrite `AssertDegradedRead` (`internal/hookstest/hooks_lock.go:132-150`) to call `logtest.AssertRecord` with `RecordWant{Level: slog.LevelDebug, Msg: "load-unlocked", Component: "hooks", Op: "load-unlocked", Via: wantVia}`, deleting the hand-rolled level, op and via comparisons.
- Keep the non-empty `error` attr check locally, as the attr that belongs to this emission alone — the shape `AssertLockWarn` ten lines above already follows.
- Narrow its parameter to `harnesstest.TestingT` if `UnlockedRecords` allows, matching `AssertLockWarn`'s signature; otherwise leave it and say so in the review.
- Confirm the message and component assertions the rewrite adds actually hold against every current caller before landing, since neither was checked before.

**Acceptance Criteria**:
- [ ] `AssertDegradedRead` delegates its level, message, component, op and via checks to `logtest.AssertRecord`.
- [ ] It asserts the message and the component, which it did not before.
- [ ] It keeps its own non-empty `error` attr check and reports it in its own words.
- [ ] Every existing caller passes unchanged.
- [ ] Its failure output reports each mismatched property separately, as `AssertRecord` does.

**Tests**: pure refactor of the helper, strengthened by the two properties it was not checking.
- `"it passes for a well-formed load-unlocked breadcrumb"`
- `"it reports a wrong via"`
- `"it reports a wrong component or message, which it previously ignored"`
- `"it reports an empty error attr"`

## Task 24: `hooks.json` and sidecar paths are still hand-composed at ~60 sites now that `hookstest` owns them

severity: duplication
sources: bank

**Problem**: `hookstest.HooksPath` and `hookstest.SidecarPath` exist, but the inline composition survives everywhere the task that introduced them did not name: 49 `filepath.Join(dir, "hooks.json")` sites remain, across `internal/hooks/lock_test.go` (13), `lookup_test.go` (3), `read_lock_test.go` (2), `cmd/hooks_read_lock_test.go:56`, `cmd/state_daemon_test.go:803`, `cmd/config_seeder_parity_test.go:106` and five stat-assertions in `cmd/state_hydrate_test.go`. The `.lock` suffix is hand-appended at 11 more (`internal/hooks/lock_test.go` ×8, `read_lock_test.go` ×2, `cmd/run_hook_stale_cleanup_snapshot_order_test.go:116`). Two further gaps sit beside them: `cmd/testhelpers_test.go:200`'s `readHooksJSON` is a read-and-unmarshal counterpart to the byte read `hookstest` already owns the encode half of, and `cmd/doctor_stand_down_copy_test.go:256-290` adds a second "hooks.json unchanged" vocabulary (`hooksPathState`/`assertHooksPathUnchanged`) beside `assertHooksFileUnchanged`, existing only because `hookstest.HooksFileBytes` fatals on EISDIR and the `Unreadable` staging axis puts a directory at the path.

**Solution**: Re-point the remaining path compositions onto `HooksPath`/`SidecarPath`; add the decode counterpart (`HooksFileEntries`) beside the encode half so `readHooksJSON` and the `internal/hooks` suites that decode the same shape share one home; and teach `hookstest` to describe a directory standing at the hooks.json path so the local `hooksPathState` pair retires into the shared one.

**Outcome**: `hooks.json` and its sidecar are named by the shared vocabulary at every site, and the two local workarounds that existed because the vocabulary was incomplete are gone.

**Do**:
- Re-point the 49 `filepath.Join(dir, "hooks.json")` sites onto `hookstest.HooksPath`: `internal/hooks/lock_test.go` (13), `lookup_test.go` (3), `read_lock_test.go` (2), `cmd/hooks_read_lock_test.go:56`, `cmd/state_daemon_test.go:803`, `cmd/config_seeder_parity_test.go:106` and the five stat-assertions in `cmd/state_hydrate_test.go`.
- Re-point the 11 hand-appended `.lock` sites onto `hookstest.SidecarPath`: `internal/hooks/lock_test.go` ×8, `read_lock_test.go` ×2, `cmd/run_hook_stale_cleanup_snapshot_order_test.go:116`.
- Add `HooksFileEntries(t, path)` to `hookstest` beside `HooksFileBytes` — the decode counterpart returning the store's `{key: {event: command}}` shape — and re-point `cmd/testhelpers_test.go:200`'s `readHooksJSON` and the `internal/hooks` suites that decode the same shape onto it.
- Teach `HooksFileBytes` (or a sibling) to describe a directory standing at the hooks.json path rather than fatalling on EISDIR, then retire `cmd/doctor_stand_down_copy_test.go:256-290`'s `hooksPathState`/`assertHooksPathUnchanged` pair into `AssertHooksFileUnchanged`.
- Where `internal/hooks`'s own suites cannot import `hookstest` without a cycle, say so at those sites rather than leaving them silently unconverted.

**Acceptance Criteria**:
- [ ] No test file composes `filepath.Join(dir, "hooks.json")` or appends `.lock` by hand.
- [ ] `hookstest` exports one decode counterpart to its encode half, and no suite hand-rolls a read-and-unmarshal of the hooks file.
- [ ] The `Unreadable` staging axis can be asserted "unchanged" through the shared helper, and the local `hooksPathState` pair is deleted.
- [ ] Every converted assertion reads the same path and the same content it read before.
- [ ] `internal/hooks`'s own suites take no import cycle from the conversion.

**Tests**: pure refactor — every converted site's expectations are unchanged.
- `"it returns the decoded hooks.json entries for a staged file"`
- `"it returns nil entries for an absent hooks.json"`
- `"it describes a directory standing at the hooks.json path instead of fatalling"`
- The `internal/hooks` lock, read-lock and lookup suites and the `cmd` hooks suites stay green with only their path compositions rewritten.

## Task 25: Six `internal/hooks` cases drifted off the degraded unlocked read, leaving an in-file branch describing no remaining case

severity: duplication
sources: bank

**Problem**: Six `internal/hooks/store_test.go` cases (two in `TestLoad`, the malformed/empty-event-map pair in `TestRemove`, the sorted `TestList` case, and three in `TestCleanStaleLogging`) seeded straight into the file and so took the degraded unlocked read; routing them through the shared stager gave them a sidecar and the shared lock. No assertion in any of them is about the read mode and the degraded-read contract is owned elsewhere, so no coverage was lost — but the `load-unlocked` skip branch in `partitionCleanStaleRecords` (`internal/hooks/store_test.go:871`) is now unexercised within that file and its comment describes no case in it. The unlocked read is the shape most installs are actually in (no install carries a sidecar until its first mutation), so leaving these fixtures on the locked path also moves them off the common case.

**Solution**: Pass `SidecarAbsent: true` at those six sites, restoring both the degraded read they exercised and the branch whose comment currently describes nothing.

**Outcome**: The six cases sit back on the degraded unlocked read they were written against — the shape most installs are in — and the `load-unlocked` skip branch in `partitionCleanStaleRecords` describes a case that exists in its own file again.

**Do**:
- Set `SidecarAbsent: true` on the two `TestLoad` cases in `internal/hooks/store_test.go` that seeded straight into the file before the stager conversion.
- Set it on the malformed and empty-event-map pair in `TestRemove`.
- Set it on the sorted `TestList` case.
- Set it on the three `TestCleanStaleLogging` cases, and confirm the `load-unlocked` skip branch at `internal/hooks/store_test.go:871` is exercised by at least one of them.
- Leave every other stager call site on the default (sidecar present), which is the written-to install those fixtures model.

**Acceptance Criteria**:
- [ ] The six named cases stage no sidecar and take the degraded unlocked read.
- [ ] `partitionCleanStaleRecords`'s `load-unlocked` skip branch is reached by at least one case in its own file.
- [ ] No assertion in any of the six is changed — none of them is about the read mode.
- [ ] Every other case in the file keeps its sidecar.

**Tests**: pure refactor of fixture staging — no assertion semantics change.
- `internal/hooks/store_test.go`'s `TestLoad`, `TestRemove`, `TestList` and `TestCleanStaleLogging` stay green with only their staging descriptions edited.
- `"it partitions clean-stale records with the load-unlocked breadcrumb skipped"` exercises the branch again.
- The degraded-read contract's own suite is untouched and stays green.

## Task 26: Token-shaped key literals and one-entry stale seeds are hand-authored outside the `hookstest` vocabulary

severity: duplication
sources: bank

**Problem**: The named seed vocabulary exists so a pane-token width move carries every fixture with it, but hand-rolled token-shaped literals survive in four packages: `tok123`/`tok999` throughout `internal/hooks/lock_write_test.go`, and the `aaa111`/`tok123` family in `internal/restore/session_test.go`, `internal/restore/rename_reboot_shared_test.go`, `internal/tmux/hookkey_format_realtmux_test.go` and `internal/tmux/resolve_hookkey_realtmux_test.go` — each carrying the same reclassification risk the vocabulary was built to remove. Separately, the identical single-entry stale-seed body is re-authored inline six times (`cmd/state_daemon_hook_cleanup_test.go:37,95,149`, `cmd/state_daemon_run_test.go:598,631,662`) where `StaleHookSeed` already owns the two-entry body next door. And the vocabulary's own completeness check (`internal/hookstest/hooks_test.go:15-35`) duplicates the seed names into two hand-maintained maps claiming to name every seed the vocabulary mints — an eighth seed added without touching them is unverified and the claim false, which is the silent-shrink failure mode relocated one level up.

**Solution**: Route the hand-rolled literals through the named seeds — noting that `internal/tmux` would gain a `hookstest` edge it has no other reason for, so that package's sites want a ruling rather than a rename — add the one-entry seed beside `StaleHookSeed` so the six inline copies become a name, and derive the self-test's completeness from a source scan of the `Seed[A-Z]` declarations rather than from a duplicated list.

**Outcome**: No fixture authors a token-shaped literal by hand, the one-entry stale seed has a name, and the vocabulary's completeness check derives from the declarations rather than restating them.

**Do**:
- Re-point `internal/hooks/lock_write_test.go`'s `tok123`/`tok999` and `internal/restore/session_test.go` / `rename_reboot_shared_test.go`'s `aaa111`/`tok123` family onto the named `hookstest` seeds whose role each site means.
- Rule on the two `internal/tmux` sites (`hookkey_format_realtmux_test.go`, `resolve_hookkey_realtmux_test.go`) the other way: their subject is a token stamped onto a pane, not a `hooks.json` key, so route them through `internal/nanoid`'s mint rather than taking a `hookstest` edge that package has no other reason for.
- Add a one-entry stale seed beside `StaleHookSeed` (`internal/hookstest/hooks.go:201-209`) and re-point the six inline copies at `cmd/state_daemon_hook_cleanup_test.go:37,95,149` and `cmd/state_daemon_run_test.go:598,631,662`.
- Rewrite `internal/hookstest/hooks_test.go:15-35`'s completeness check to enumerate the package's `Seed[A-Z]…` declarations through `sourceguardtest`'s value-spec walk and assert each is covered, deleting the two hand-maintained maps.
- Keep every seed's token-shape assertion and its `panic` on an id charset change: a width or charset move must still fail loudly rather than reclassify a fixture.

**Acceptance Criteria**:
- [ ] No hand-authored token-shaped literal survives in `internal/hooks`, `internal/restore` or `internal/tmux` test files.
- [ ] `internal/tmux` takes no `hookstest` import; its tokens come from the mint.
- [ ] The one-entry stale seed is declared once and used at all six former inline sites.
- [ ] Adding an eighth seed to the vocabulary without touching the self-test fails that test.
- [ ] A pane-token width change carries every converted fixture with it, and an alphabet change panics at seed construction.

**Tests**: pure refactor of fixture vocabulary — no assertion semantics change.
- `"it derives the vocabulary completeness check from the declared seeds"`
- `"it fails the completeness check for a seed the check does not cover"`
- `"it stages the one-entry stale seed as a single named body"`
- The converted `internal/hooks`, `internal/restore`, `internal/tmux` and `cmd` suites stay green with only their key literals rewritten.

## Task 27: `logtest`'s query surface is missing the dimensions its consumers hand-roll

severity: duplication
sources: bank

**Problem**: The exported filters express `(component, message)`, level and message, and the whole capture — and consumers write the rest by hand. Twelve sites walk `sink.Records()` with an inline `r.Level == … && r.Msg == …` predicate the surface now expresses directly (`cmd/bootstrap/clean_sweep_summary_test.go:57,60`, `internal/hooks/store_test.go:1035`, `internal/state/fifo_sweep_summary_test.go:192`, `internal/hooks/store_shape_test.go:97`, `cmd/bootstrap/latch_test.go:122`, and others). Four sites re-author a partition loop splitting records into two sets by level plus a `strings.HasPrefix` on the message (`cmd/open_burst_run_test.go:689,753,830`, `internal/project/store_logging_test.go:370`, `internal/hooks/store_test.go:897`) — the absent prefix query is the root of those. Roughly six sites hand-write a non-fatal `if got := len(<query>); got != 1` because `Records.Only` is fatal by construction (`cmd/run_hook_stale_cleanup_test.go:39,67,185,189`, `cmd/run_hook_stale_cleanup_single_report_test.go:127`, `cmd/doctor_fix_hook_prune_report_test.go:45,70`, `cmd/hooks_test.go:974`, `cmd/theme_persister_test.go:200`). And `cmd/theme_source_test.go:219` spells out `rec.Msg == msg && rec.HasAttr("component") && rec.AttrString(…) == "theme"`, a second declaration of the predicate `Record.Matches` documents itself as the single home of.

**Solution**: Close the surface's gaps rather than the call sites' — a message-prefix filter, and a non-fatal cardinality terminal beside the fatal `Only` — then re-point the sites each one absorbs, and route `cmd/theme_source_test.go:219` through `Matching`. Sites that are genuinely inexpressible stay as they are and are named where they sit: the attr-presence classification in `internal/tui/burst_observability_test.go:314` and the shared-sequence appends in `cmd/bootstrap/latch_test.go`.

**Outcome**: The query surface expresses the dimensions its consumers actually ask for, so the twelve inline predicates, four partition loops and six hand-written cardinality checks become chained filters, and the genuinely inexpressible sites are named rather than mistaken for drift.

**Do**:
- Add `WithMessagePrefix(prefix)` to `internal/logtest`'s `Records` filters, as an orthogonal dimension beside `WithMessage` — never a combination of existing filters, which the surface's own rule forbids.
- Add a non-fatal cardinality terminal beside `Only` — one that reports rather than fatals when the count is not what was asked — so a caller with an assertion of its own after the check has a shared route to it.
- Re-point the twelve inline `r.Level == … && r.Msg == …` walkers onto the chained filters, including `cmd/bootstrap/clean_sweep_summary_test.go:57,60`, `internal/hooks/store_test.go:1035`, `internal/state/fifo_sweep_summary_test.go:192`, `internal/hooks/store_shape_test.go:97` and `cmd/bootstrap/latch_test.go:122`.
- Re-point the four partition loops (`cmd/open_burst_run_test.go:689,753,830`, `internal/project/store_logging_test.go:370`, `internal/hooks/store_test.go:897`) onto the prefix filter plus a level filter, and the roughly six hand-written `len(<query>) != 1` checks onto the non-fatal terminal.
- Route `cmd/theme_source_test.go:219` through `Matching`, and leave `internal/tui/burst_observability_test.go:314` and the `cmd/bootstrap/latch_test.go` sequence appends as they are.

**Acceptance Criteria**:
- [ ] `logtest` gains exactly two members — a message-prefix filter and a non-fatal cardinality terminal — and no combination of existing filters becomes a method.
- [ ] No consumer walks `sink.Records()` with an inline level-and-message predicate the surface expresses.
- [ ] No consumer re-authors a level partition loop over a message prefix.
- [ ] No consumer hand-writes a `len(<query>) != 1` check that the non-fatal terminal covers.
- [ ] The two inexpressible sites are unchanged and stated as such.

**Tests**: pure refactor of consumers plus two new query members.
- `"it keeps only the records whose message carries the given prefix"`
- `"it returns nil when no record carries the prefix"`
- `"it reports rather than fatals when the set does not hold exactly one record"`
- Every re-pointed suite stays green with only its query lines rewritten.

## Task 28: A component-alone query exists in the tree, which is the stated trip-wire for splitting `Matching`

severity: duplication
sources: bank

**Problem**: `Matching(component, msg)` was kept as one filter on the premise that the pair jointly names one event and nothing queries the component alone. `cmd/open_theme_construction_test.go:521`'s `themeEvents` walks `sink.Records()` filtering on `HasAttr("component") && AttrString(t, "component") == "theme"` with no message constraint — the one shape the exported surface cannot express, so the premise is already false. The surface's own rule is that no combination of filters may itself be a method, because that is what holds the property a caller never has two routes to one set.

**Solution**: Add `WithComponent` and delete `Matching` in the same change — with `WithComponent` present, `Matching` is a composition and therefore a second route — re-pointing every `Matching` call site onto the chained pair, and keep `Record.Matches` for the capture-order walkers that need the predicate rather than a filtered slice.

**Outcome**: Component and message are two orthogonal filters a caller chains, with no combination of them offered as a method, so the surface's own rule holds against the component-alone query the tree already contains.

**Do**:
- Add `WithComponent(component)` to `internal/logtest`'s `Records` filters, keying on the `component` attr the handler binds.
- Delete `Records.Matching` (`internal/logtest/capture.go:149-151`) in the same change, since with `WithComponent` present it is a composition and therefore a second route to one set.
- Re-point every `Matching(component, msg)` call site onto `.WithComponent(component).WithMessage(msg)`, including the sites Task 27 routes there.
- Re-point `cmd/open_theme_construction_test.go:521`'s `themeEvents` onto `WithComponent` alone, deleting its inline `HasAttr`/`AttrString` predicate.
- Keep `Record.Matches` as it is: the capture-order walkers need the predicate over one record rather than a filtered slice, which is what its doc already says it is for.

**Acceptance Criteria**:
- [ ] `logtest` exports `WithComponent` and no longer exports `Matching`.
- [ ] Every former `Matching` site reads as a chained pair and returns the same set.
- [ ] The component-alone query in `cmd/open_theme_construction_test.go` routes through the exported filter with no inline predicate.
- [ ] `Record.Matches` survives with its capture-order consumers unchanged.
- [ ] No combination of the exported filters is itself a method.

**Tests**: pure refactor of the query surface — every consumer's set is unchanged.
- `"it keeps only the records emitted under the given component"`
- `"it returns nil when no record carries that component"`
- `"it returns the same set from the chained pair as the deleted combined filter did"`
- Every re-pointed suite stays green with only its query lines rewritten.

## Task 29: The failed-write tail is hand-rolled in the `cmd` migrate suite in a weaker form than the shared helper enforces

severity: duplication
sources: bank

**Problem**: `cmd/config_migrate_logging_test.go:168` and `:208` already use `logtest.AssertRecord` for the five shared properties and then spell the tail inline — an `error_class` string check followed by `rec.HasAttr("error")`. `HasAttr` is strictly weaker than `AssertWriteFailure`: it never checks that the carried error wraps the named `fileutil` write-phase sentinel (`ErrWriteRename` at `:168`, `ErrWriteTempCreate` at `:208`), so a misclassified error passes. The helper does not fit as-is, because `cmd/config.go`'s migrate WARNs log the raw OS error with a hardcoded `error_class` and wrap no sentinel. Note the site that looks like a third is not one: `internal/storelog/clean_stale_test.go:59-66` asserts `loggedErr != saveErr` — an identity check pinning that the emitter passes the caller's error through un-rewrapped, a stronger and different property `AssertWriteFailure`'s `errors.Is` would weaken. It must not be folded in by pattern-match.

**Solution**: Wrap the migrate path's write failures in the `fileutil` phase sentinels they already classify themselves as, then route both assertions through `AssertWriteFailure` — removing the last two copies and strengthening them in the same change. Leave `internal/storelog/clean_stale_test.go` alone.

**Outcome**: Every failed-write breadcrumb in the tree is pinned by the shared helper, so a misclassified migrate error fails a test instead of passing an `error_class` string check.

**Do**:
- Wrap the migrate path's write failures in `cmd/config.go` in the `fileutil` phase sentinels their hardcoded `error_class` already names — `ErrWriteRename` for the rename failure, `ErrWriteTempCreate` for the temp-create failure — so the logged `error` attr carries a sentinel to match against.
- Route `cmd/config_migrate_logging_test.go:168` and `:208` through `logtest.AssertWriteFailure` with those two sentinels, deleting the inline `error_class` string check and the `rec.HasAttr("error")` weaker check.
- Leave the `error_class` values themselves unchanged, so the rendered breadcrumb is byte-identical.
- Leave `internal/storelog/clean_stale_test.go:59-66` exactly as it is: its `loggedErr != saveErr` identity check pins a stronger and different property than `errors.Is` would, and folding it in by pattern-match would weaken it.

**Acceptance Criteria**:
- [ ] Both migrate WARNs carry an `error` attr wrapping the named `fileutil` write-phase sentinel.
- [ ] Both assertions route through `AssertWriteFailure` and no longer use `HasAttr` for the error.
- [ ] A migrate error misclassified against its sentinel now fails the test.
- [ ] The rendered `error_class` tokens are unchanged.
- [ ] `internal/storelog/clean_stale_test.go` is byte-unchanged.

**Tests**:
- `"it wraps the migrate rename failure in the rename write-phase sentinel"`
- `"it wraps the migrate temp-create failure in the temp-create write-phase sentinel"`
- `"it fails when the carried error does not wrap the classified phase sentinel"`
- `"it renders the same error_class token as before"`

## Task 30: `internal/logtest` states a dependency-shape invariant that no guard enforces

severity: medium
sources: bank

**Problem**: `internal/logtest/assert.go:44-47` documents that `logtest` takes no dependency on the package declaring the write-phase sentinels — which is why `AssertWriteFailure` takes the sentinel as a parameter rather than resolving it — and CLAUDE.md's `logtest` row repeats the claim. Unlike the sibling test-support leaf `internal/harnesstest`, which pins its dependency set through `sourceguardtest.AssertDepsWithin` (`internal/harnesstest/leaf_guard_test.go`), `internal/logtest` has no deps guard at all, so the first `internal/fileutil` import would compile silently and falsify both statements. `logtest` is reachable from every test package in the tree, which is what makes the property load-bearing.

**Solution**: Add the leaf guard `internal/harnesstest` already carries, declaring `logtest`'s allowed dependency set so the documented shape is enforced rather than asserted in prose.

**Outcome**: `logtest`'s documented dependency shape is enforced by a test, so an `internal/fileutil` import fails the build rather than silently falsifying two written claims.

**Do**:
- Add `internal/logtest/leaf_guard_test.go` in the shape of `internal/harnesstest/leaf_guard_test.go`, calling `sourceguardtest.AssertDepsWithin` for `github.com/leeovery/portal/internal/logtest`.
- Declare the allowlist from what the package actually reaches today — `internal/harnesstest` and `internal/log` — and nothing more, so an `internal/fileutil` import fails.
- Pass the tag-awareness option so an `integration`-tagged file in the package cannot hide a dependency from the guard.
- Re-voice `internal/logtest/assert.go:44-47`'s comment so it states the conclusion the parameter shape needs rather than asserting a property nothing checked, and update CLAUDE.md's `logtest` row to say the shape is guarded.

**Acceptance Criteria**:
- [ ] `internal/logtest` carries a leaf guard asserting its transitive dependency set.
- [ ] Adding an `internal/fileutil` import to `internal/logtest` fails that guard.
- [ ] The guard resolves the tagged configuration as well as the default one.
- [ ] The guard refuses to pass over an empty or drifted dependency set, as `AssertDepsWithin` already enforces.

**Tests**:
- `"it holds logtest's transitive dependencies inside the declared allowlist"`
- `"it reports a dependency outside the allowlist"`
- `"it sees a dependency reachable only from an integration-tagged file"`

## Task 31: Three `ConfigFileID` filenames are asserted by nothing

severity: medium
sources: bank

**Problem**: `xdg.AliasesFile.Filename`, `xdg.ProjectsFile.Filename` and `xdg.TerminalsFile.Filename` are pinned by no test. Their env vars are covered behaviourally (`cmd/alias_test.go`, `cmd/spawn_seams_test.go`, `cmd/version_guard_test.go` set them and assert the resolved store is read), but `cmd`'s `TestMain` poisons those variables package-wide, so no test ever exercises those three files at the default config base. A typo in any of them would ship silently and point a user's aliases or projects at a file nothing writes — the file would simply appear empty. `cmd/config_identity_test.go:49` already establishes the pattern for `HooksFile`, pinning env var, filename and log component together.

**Solution**: Extend that subtest into a table over all five `ConfigFileID`s and both `ConfigDirID`s, so every identity's env var, filename and log component are pinned by one assertion.

**Outcome**: Every config file and directory identity's env var, filename and log component is pinned, so a typo in any of them fails a test rather than shipping a store that reads a file nothing writes.

**Do**:
- Turn `cmd/config_identity_test.go:49`'s `HooksFile` subtest into a table over all five `ConfigFileID`s — `ProjectsFile`, `AliasesFile`, `HooksFile`, `PrefsFile`, `TerminalsFile` — asserting each identity's `EnvVar`, `Filename` and `LogComponent`.
- Extend the same table shape to both `ConfigDirID`s — `StateDir` and `ThemesDir` — asserting `EnvVar` and `Dirname`.
- Pin the deliberate empty log components (`PrefsFile`, `TerminalsFile`) explicitly, so a later change that gives one a component is a visible edit rather than a silent one.
- Make the table's completeness derivable rather than hand-listed where the package allows, so a sixth identity added without a row is caught.

**Acceptance Criteria**:
- [ ] All five `ConfigFileID` filenames are asserted, including the three that are asserted by nothing today.
- [ ] All five env vars and all five log components are asserted in the same rows.
- [ ] Both `ConfigDirID`s are covered for env var and directory name.
- [ ] A typo in any filename, env var, directory name or component fails the test.
- [ ] Adding a new identity without a table row fails the test.

**Tests**:
- `"it pins every config file identity's env var, filename and log component"`
- `"it pins both config directory identities' env var and directory name"`
- `"it pins the deliberately empty log components rather than skipping them"`
- `"it fails when an identity is declared with no table row"`

## Task 32: `XDG_CONFIG_HOME` and the state-directory rule are each named in two places

severity: duplication
sources: bank

**Problem**: `internal/xdg` is now the shared home of every config file and directory identity, but two rules it owns are restated outside it. `internal/xdg/xdg.go:25` hardcodes the `XDG_CONFIG_HOME` literal inside `ConfigBaseFrom` while `internal/hookstest/hooks.go:37` declares its own `const xdgConfigHome = "XDG_CONFIG_HOME"` for the isolation-regression check — so the seeder that must resolve by the same rule as the binary under test names the variable independently. Separately, `internal/portaltest` composes `<base>/portal/state` by hand at `isolated_env.go:68` and `fingerprint.go:288,291`, and spells `PORTAL_STATE_DIR` literally at `isolated_env.go:76,93-94` and `spawn_daemon.go:32`, all now duplicating `xdg.StateDir`/`xdg.ConfigDirPath`.

**Solution**: Export the environment variable name from `internal/xdg` and have `hookstest` read it; re-point `portaltest`'s state-dir composition onto `xdg.StateDir`, **except** the `fingerprint.go` copy, which is a deliberate independent re-derivation used as a backstop and must stay independent — the point of a backstop being that it does not share a bug with what it checks.

**Outcome**: The `XDG_CONFIG_HOME` name and the state-directory rule each have one home in `internal/xdg`, with the fingerprint backstop's re-derivation deliberately and visibly kept independent.

**Do**:
- Export the `XDG_CONFIG_HOME` variable name from `internal/xdg` and use it inside `ConfigBaseFrom` (`internal/xdg/xdg.go:25`), replacing the hardcoded literal.
- Delete `internal/hookstest/hooks.go:37`'s `const xdgConfigHome` and read the exported name at `:56-57` instead.
- Re-point `internal/portaltest/isolated_env.go:68`'s hand-composed `<base>/portal/state` onto `xdg.ConfigDirPath` over `xdg.StateDir`, and replace the literal `PORTAL_STATE_DIR` at `:76,93-94` and `internal/portaltest/spawn_daemon.go:32` with `xdg.StateDir`'s own env-var name.
- Leave `internal/portaltest/fingerprint.go:288,291` composing the path independently, and state at that site that the independence is the point — a backstop that shares a derivation with what it checks shares its bugs.
- Confirm the new `internal/xdg` edge does not break `portaltest`'s or `hookstest`'s dependency shape, and adjust any leaf guard the edge reaches.

**Acceptance Criteria**:
- [ ] `XDG_CONFIG_HOME` is declared once in `internal/xdg` and read from there by `ConfigBaseFrom` and `hookstest`.
- [ ] `PORTAL_STATE_DIR` is spelled once, in `internal/xdg`'s `StateDir` identity, and read from there by `portaltest`.
- [ ] `isolated_env.go` composes the state directory through `xdg.ConfigDirPath` and resolves to the same path it does today.
- [ ] `fingerprint.go`'s composition remains independent and is stated as deliberate.
- [ ] The isolation regression tripwire in `ResolveHooksFilePathFromEnv` still fires for a slice carrying neither variable.

**Tests**: pure refactor — every resolved path is unchanged.
- `"it resolves the same state directory through the shared identity as the hand-composed path did"`
- `"it fatals when the env slice carries neither the hooks file nor the config-home variable"`
- `"it keeps the fingerprint backstop's path derivation independent of the shared one"`
- `internal/portaltest` and `internal/hookstest` suites stay green with no assertion edited.

## Task 33: The test wait budgets have no single home, so four shapes coexist and three sites inherit an unrelated ceiling

severity: duplication
sources: bank

**Problem**: The progress-based wait landed for fifteen daemon-lifecycle sites, and everything around it stayed fixed wall-clock. `tmuxtest.Socket.WaitForSession` (`internal/tmuxtest/socket.go:114-127`) polls `has-session` against a bare timeout and is called with an undeclared `2 * time.Second` at ~34 sites across `internal/restore` and `cmd/bootstrap`; deleting `singletonRecycleTimeout` also left `internal/tmux/portal_saver_integration_test.go:54,67,108` passing a 45s `ProgressWait.Ceiling` where 5s stood, coupling a session-appearance budget to a ceiling chosen for a different observable and stretching a genuinely-red run ninefold. `cmd/bootstrap/eager_signal_hydrate_integration_test.go:256` declares its own deadline/tick/sleep loop over `CapturePane`, the only budget consumer in the family that is not a shared helper. `cmd/bootstrap/reboot_roundtrip_test.go:492` hand-writes `10*time.Second` for the marker-clear observable while `:506` in the same function reads the declared `HydrateBudget`/`HydrateTick` pair. `internal/restoretest/restoretest.go:100-103` declares a second local 10s/50ms pair inside `DriveSignalHydrate` for a different observable, in the file next door to the declaration. And `AssertMarkerCount` hardcodes `HydrateBudget` (`internal/restoretest/marker_count.go:49`), so a structurally identical post-marker-clear hook wait runs on the long budget in `cmd/bootstrap` while its twin in `internal/restore` runs on the short one. Two further fixed-budget `PollUntil` consumers survive outside the converted set (`internal/state/capture_colon_session_realtmux_test.go:79`, `internal/portaltest/tmux_server_wait.go:30`).

**Solution**: Give each observable one named budget in the package that owns it — the session-appearance wait belongs with `tmuxtest`, whose subject is the socket — restore a named appearance budget in place of the borrowed ceiling, let `AssertMarkerCount` take a budget rather than hardcoding one, route the pane-text wait through the shared poll helper, and settle the second hydrate pair deliberately (kept with a stated reason, or collapsed) rather than leaving it a near-miss beside its own declaration.

**Outcome**: Each waited-for observable has one named budget declared in the package that owns it, and no site inherits a ceiling chosen for a different observable.

**Do**:
- Declare a named session-appearance budget and tick in `internal/tmuxtest` beside `WaitForSession` (`socket.go:114-127`) and default the method to them, so the ~34 callers across `internal/restore` and `cmd/bootstrap` stop passing an undeclared `2 * time.Second`.
- Replace the 45s `ProgressWait.Ceiling` at `internal/tmux/portal_saver_integration_test.go:54,67,108` with that named appearance budget, restoring a figure chosen for the observable being waited on.
- Give `AssertMarkerCount` (`internal/restoretest/marker_count.go:43-52`) a budget parameter (or a budget-taking sibling) rather than hardcoding `HydrateBudget`, and pass the appropriate one at the `cmd/bootstrap` and `internal/restore` call sites so structurally identical waits share a budget.
- Route `cmd/bootstrap/eager_signal_hydrate_integration_test.go:256`'s hand-written deadline/tick/sleep loop over `CapturePane` through `harnesstest.PollUntil` against a named budget, and replace `cmd/bootstrap/reboot_roundtrip_test.go:492`'s hand-written `10*time.Second` with the declared pair its sibling at `:506` already reads.
- Settle `internal/restoretest/restoretest.go:100-103`'s second local 10s/50ms pair deliberately: collapse it onto `HydrateBudget`/`HydrateTick`, or keep it as a separate named budget for its own observable with the reason stated where it is declared — not left as an unnamed near-miss beside the declaration next door.
- Bring the two surviving fixed-budget `PollUntil` consumers (`internal/state/capture_colon_session_realtmux_test.go:79`, `internal/portaltest/tmux_server_wait.go:30`) onto named budgets in their own packages.

**Acceptance Criteria**:
- [ ] No call site passes a bare `time.Duration` literal to a shared wait helper; every budget is a named declaration.
- [ ] The session-appearance budget is declared in `internal/tmuxtest` and is what the saver integration fixtures wait on, replacing the borrowed 45s ceiling.
- [ ] `AssertMarkerCount` takes its budget rather than hardcoding one, and the two structurally identical post-marker-clear hook waits run on the same budget.
- [ ] The second hydrate pair is either collapsed onto the declared one or named with its reason stated at its declaration.
- [ ] A genuinely red run of the saver fixtures fails at the appearance budget, not ninefold later.

**Tests**: pure refactor of test budgets — no assertion semantics change.
- `"it waits for a session on the package's own declared appearance budget"`
- `"it asserts a marker count on the budget its caller names"`
- `"it waits for pane text through the shared poll helper"`
- The saver, reboot-roundtrip, eager-signal-hydrate and colon-session fixtures stay green with only their budget references rewritten.

## Task 34: The tmux-server teardown wait sits outside the one place that knows the socket, and its argv convention exists twice

severity: duplication
sources: bank

**Problem**: `internal/portaltest/tmux_server_wait.go:35-37` hand-writes `-S <socketPath> -f /dev/null`, which is exactly `internal/tmuxtest/socket.go`'s unexported `socketArgs` — and a wrong copy fails **silently**, because an unreachable socket reads as a server that is gone. The wait itself is registered per fixture, where `internal/tmuxtest/socket.go:39-42` already registers the one cleanup closure that knows the socket and calls `KillServer`; folding the wait in there would cover every server fixture at zero call-site cost and let `reapTmuxServer` in the coldboot suite collapse to nothing. Beside it, `restoretest.OpenRebootGap` (`internal/restoretest/reboot.go:22-28`) probes once immediately after `KillServer` and fatals if it succeeds, with no wait at all — so every reboot fixture routed through it re-opens the shells-outlive-the-server window that the teardown work closed elsewhere. Note also that the HOME-side quiescence guard (`internal/portaltest/teardown_guard.go:83-93`) returns as soon as two consecutive 50ms snapshots match, so over an empty temp HOME it returns at t=50ms having observed nothing — it can miss the very case it names, unlike the state-dir guard, which gates on the daemon pid dying first.

**Solution**: Move the server-unreachable wait into `tmuxtest`, folded into the cleanup closure `New` already registers, and export the socket-args composition so the probe stops restating it; compose `OpenRebootGap` over the shared wait so it gets both the wait and its assertion from one source; and give the HOME quiescence guard a gate on something observable, the way the state-dir guard has one, or state in its own words that it is belt-and-braces over the env pins.

**Outcome**: The server-gone wait lives with the one closure that knows the socket, so every server fixture gets it at zero call-site cost, and the reboot gap is opened with the wait rather than with a bare probe.

**Do**:
- Move the server-unreachable wait into `internal/tmuxtest` and fold it into the cleanup closure `New` registers (`socket.go:41-44`), between `KillServer` and the `RemoveAll`, so every server fixture is covered without a call site.
- Export the socket-args composition (`socket.go:17-19`) and re-point the probe at `internal/portaltest/tmux_server_wait.go:35-37` onto it — or retire that file entirely if `tmuxtest` now owns the whole wait — so the `-S <socket> -f /dev/null` convention exists once.
- Rewrite `restoretest.OpenRebootGap` (`internal/restoretest/reboot.go:22-28`) to compose over the shared wait: wait for the server to become unreachable, then fatal if `list-sessions` still succeeds, so the gap-opening assertion and the wait come from one source.
- Collapse `reapTmuxServer` in the coldboot suite now that the fixture's own cleanup carries the wait.
- Rule on the HOME quiescence guard (`internal/portaltest/teardown_guard.go:75-79`): with the server-gone wait now in `tmuxtest`'s cleanup — which LIFO-runs before this guard, since a fixture isolates before it starts its server — the gate this guard lacked is supplied externally, so state that at the guard rather than adding a socket-aware gate it has no socket for.

**Acceptance Criteria**:
- [ ] The `-S <socket> -f /dev/null` argv convention is composed in exactly one place.
- [ ] Every `tmuxtest` server fixture waits for its server to become unreachable during cleanup, with no call-site change.
- [ ] `OpenRebootGap` waits before asserting, so a reboot fixture no longer re-opens the shells-outlive-the-server window.
- [ ] `reapTmuxServer` is gone from the coldboot suite.
- [ ] The HOME quiescence guard states what supplies its gate, and the state-dir guard's own pid gate is unchanged.

**Tests**: pure refactor of harness teardown — no fixture assertion changes.
- `"it waits for the server to stop answering before removing the socket directory"`
- `"it fatals when list-sessions still succeeds after the wait"`
- `"it composes the socket argv through the single exported helper"`
- The reboot, coldboot and restore integration fixtures stay green with no assertion edited.

## Task 35: The isolated-env shell pins are inert on this platform, and the sandbox temp dir leaks on every run

severity: medium
sources: bank

**Problem**: Measured with a sandboxed `-S` server and an interactive `/bin/zsh` pane under a temp HOME: (a) `/etc/zshrc` sets `HISTFILE=${ZDOTDIR:-$HOME}/.zsh_history` unconditionally, overriding `IsolateStateForTest`'s `HISTFILE=os.DevNull` pin (`internal/portaltest/isolated_env.go:41`), so `.zsh_history` lands in the temp HOME on every run — that is the writer the teardown actually races; (b) `.zsh_sessions` never appeared even with `TERM_PROGRAM=Apple_Terminal` set, so `SHELL_SESSIONS_DISABLE=1` (`:42`) neutralises nothing; (c) `ZDOTDIR=homeDir` (`:43`) is a no-op by construction, since `${ZDOTDIR:-$HOME}` already resolves to the temp HOME. The lever that works is `ZDOTDIR` pointed **outside** the framework TempDir tree — with it set, the probe temp HOME came back empty. So three env pins that read as the isolation boundary are decorative, and the quiescence wait is carrying the whole fix. Separately, `internal/portaltest/isolated_env_test.go`'s `TestMain` registers `defer os.RemoveAll(sandbox)` and then ends with `os.Exit(m.Run())`, which skips deferred functions — every run of that package leaves a `portaltest-self-sandbox-*` directory in `$TMPDIR`.

**Solution**: Point `ZDOTDIR` outside the framework temp tree so the shell writes nothing into the directory the framework is about to remove, and retire or re-voice the two pins that neutralise nothing so the helper does not read as protecting more than it does. In the self-test's `TestMain`, capture the run's code, run the cleanup, then exit.

**Outcome**: The isolated env's shell pins actually keep the shell's per-session writes out of the directory the framework is about to remove, the pins that neutralise nothing no longer read as protection, and the self-test stops leaking a sandbox directory per run.

**Do**:
- Point `ZDOTDIR` (`internal/portaltest/isolated_env.go:43`) at a directory created outside the framework's `t.TempDir()` tree — one the helper creates and removes itself — so `${ZDOTDIR:-$HOME}` resolves there and `/etc/zshrc`'s unconditional `HISTFILE` assignment lands outside the tree the framework removes.
- Retire or re-voice the `HISTFILE=os.DevNull` pin (`:41`), which `/etc/zshrc` overrides unconditionally, and `SHELL_SESSIONS_DISABLE=1` (`:42`), which was measured to neutralise nothing on this platform — whichever is kept, its comment must say what it actually buys.
- Update the comment block at `:39-42` so it names the writer the quiescence wait is actually racing, rather than describing pins that do not stop it.
- In `internal/portaltest/isolated_env_test.go`'s `TestMain`, capture `m.Run()`'s code into a variable, remove the sandbox directory, then `os.Exit(code)` — replacing the `defer` that `os.Exit` skips.
- Verify with a real interactive shell pane under a temp HOME that the probe directory comes back empty after the change, rather than asserting the fix by construction.

**Acceptance Criteria**:
- [ ] `ZDOTDIR` points outside the framework temp tree, and the temp HOME holds no `.zsh_history` after a fixture whose tmux server hosted an interactive shell.
- [ ] Every remaining shell pin has a stated effect on this platform; the ones that neutralise nothing are gone or re-voiced.
- [ ] `go test ./internal/portaltest` leaves no `portaltest-self-sandbox-*` directory in `$TMPDIR`.
- [ ] The self-test's exit code is the run's code, not the cleanup's.
- [ ] The existing quiescence wait and fingerprint backstop are unchanged in behaviour.

**Tests**:
- `"it points ZDOTDIR outside the framework temp tree"`
- `"it leaves the temp HOME empty after an interactive shell pane exits"`
- `"it removes the self-test sandbox directory and preserves the run's exit code"`
- `"it still registers the quiescence guard over the isolated HOME"`

## Task 36: The shared Commander fake's headline safety property is opted out of at ~90% of its call sites

severity: duplication
sources: bank

**Problem**: `commandertest.New(t, …)` reports an unscripted argv instead of answering it — the property the fake's own doc names ("a fake that answers an unscripted argv with `("", nil)` is how a test passes while exercising nothing"). It is used at 40 sites tree-wide. `internal/tmux` and `internal/restore` use `commandertest.Quiet` at 185 sites and `FromFunc` at ~190 more, with zero loud sites in either — faithful to the pre-refactor semantics those packages' own silent catch-all fakes had, so nothing regressed, but those two packages do not have the protection the shared fake exists to give. A fifth such fake also survives un-repointed: `internalMockCommander` (`internal/tmux/option_discriminator_internal_test.go:9-20`) returns `(m.Output, m.Err)` identically from `Run` and `RunRaw`, missed by the original sweep because it lives in the in-package test rather than the external one, and re-pointable at no cost since `commandertest` is structurally typed and stdlib-only.

**Solution**: Re-point `internalMockCommander` onto the shared fake, and convert the `Quiet` sites to the loud default package by package, scripting each site's real argv set — expecting the conversion to surface tests asserting on argv production never issues, which is the finding rather than an obstacle to it. `FromFunc` sites model tmux rather than scripting argv and stay as they are.

**Outcome**: `internal/tmux` and `internal/restore` get the unscripted-argv protection the shared fake exists to give, and the fifth un-repointed fake is gone.

**Do**:
- Re-point `internalMockCommander` (`internal/tmux/option_discriminator_internal_test.go:9-20`) onto `commandertest`, which is structurally typed and stdlib-only so the in-package test takes no cycle.
- Convert `internal/tmux`'s `Quiet` sites to the loud default one file at a time, scripting each site's real argv set; where a site genuinely needs a catch-all, keep `Quiet` and say at that site what it is opting out of.
- Do the same for `internal/restore`'s `Quiet` sites.
- Treat any test the conversion reveals as asserting on an argv the production path never issues as a finding to report, not an obstacle: fix the assertion or the production expectation, and name each one.
- Leave every `FromFunc` site as it is: those model tmux rather than scripting argv.

**Acceptance Criteria**:
- [ ] `internal/tmux` and `internal/restore` have no `Quiet` site that has not been considered; each surviving one states what it opts out of.
- [ ] `internalMockCommander` is deleted and its consumers drive the shared fake.
- [ ] An unscripted argv in a converted site reports through the `TestingT` and returns an error, rather than answering `("", nil)`.
- [ ] Any test found to assert on an argv the production path never issues is named and resolved rather than papered over with a catch-all.
- [ ] `FromFunc` sites are unchanged.

**Tests**: pure refactor of fakes, strengthened by the loud default.
- `"it reports an unscripted argv in the converted tmux fixtures"`
- `"it reports an unscripted argv in the converted restore fixtures"`
- `"it answers the option-discriminator fixture through the shared fake"`
- Every converted suite stays green with only its fake construction rewritten.

## Task 37: `RunRaw` has a single production consumer, so the two-method Commander contract protects one call

severity: low
sources: bank

**Problem**: `internal/tmux/tmux.go:720` (`CapturePane`) is the sole `RunRaw` caller in production; every other client method reads through `Run`. The `Commander` interface's two-method shape, the shared fake's two-method contract, its `Strict` mode (making any `RunRaw` call fatal for a package whose production path reads through `Run` alone), and the trim-versus-verbatim contract exported as `Trim`/`Verbatim` all exist to protect that one call — and the drift between fakes over which method returns what is what motivated the fake's consolidation in the first place.

**Solution**: Weigh expressing the split as one `Run` plus a dedicated verbatim-capture seam, which would retire the interface property, the `Strict` mode and the class of drift, against keeping the general two-method interface. If the interface stays, the reason it stays belongs beside it, since the current shape reads as a general property rather than as one call's requirement.

**Outcome**: The two-method `Commander` contract is either retired in favour of a seam that names the one call it protects, or kept with its reason stated where the shape is declared — so it no longer reads as a general property nothing needs.

**Do**:
- Establish the true reach first: enumerate every `RunRaw` consumer in production and in the fakes, confirming `CapturePane` (`internal/tmux/tmux.go:720`) is still the only production one.
- Weigh the two shapes against stated criteria — how many declarations the split costs, whether any planned tmux read needs verbatim output, and what the `Strict` mode and the `Trim`/`Verbatim` export would become under each.
- If the split lands: reduce `Commander` to `Run`, give `CapturePane` a dedicated verbatim-capture seam, and retire `Strict` and the two-method contract from `commandertest` along with the drift class they exist to police.
- If the interface stays: state at its declaration that the second method exists for the one verbatim read, so the shape is read as that call's requirement rather than as a general property.
- Either way, leave `CapturePane`'s output byte-identical: this is a seam reshape, not a change to what tmux is asked for or what is returned.

**Acceptance Criteria**:
- [ ] The decision is recorded with the reach it was taken against.
- [ ] `CapturePane` returns byte-identical output, untrimmed, under the chosen shape.
- [ ] Under the split: `Commander` has one method, no fake declares a two-method contract, and `Strict` is gone.
- [ ] Under the keep: the second method's reason is stated at the interface declaration and the `Trim`/`Verbatim` export is described as that call's contract.
- [ ] No production tmux read changes its argv or its trimming.

**Tests**:
- `"it returns capture-pane output verbatim, trailing whitespace included"`
- `"it trims every other client read's output"`
- `"it drives the whole client surface through the chosen commander shape"`
- `"it reports a fake that answers the verbatim read through the trimming path"`

## Task 38: A fifth callee-name unwrapper survives in `internal/capture`

severity: duplication
sources: bank

**Problem**: `countCalls` (`internal/capture/swap_harness_test.go:224-244`) switches on `call.Fun` with a `*ast.SelectorExpr` arm and an `*ast.Ident` arm that both do `== name; count++`, which is exactly `sourceguardtest.CalleeName(call) == name`. It was missed by the sweep that collapsed the other four, and an independent 42-site sweep has since confirmed it is the only one left: its three call sites pass string literals and never `""`, so the empty-unwrap case cannot change a count, and the package's test set already imports `sourceguardtest`, so there is no new package edge. The same sweep cleared `internal/theme/loader_construction_guard_test.go:64` (it switches on a type expression) and `internal/tui/theme_panel_commit_slot_test.go:448`, and confirmed `internal/portaltest`'s `selectorName`/`localCallName` must **not** be routed through `CalleeName` — they render selectors as `pkg.func` so qualified and unqualified calls stay distinguishable, which that guard depends on. Those three are not candidates and should not be re-flagged.

**Solution**: Collapse `countCalls`'s switch to the shared unwrapper, closing the family tree-wide.

**Outcome**: The callee-name unwrapper has one home tree-wide, with the three deliberate non-candidates left where they are.

**Do**:
- Replace `countCalls`'s `*ast.SelectorExpr` / `*ast.Ident` switch (`internal/capture/swap_harness_test.go:224-244`) with `sourceguardtest.CalleeName(call) == name`, keeping the counting loop and its three call sites unchanged.
- Take no new package edge: the package's test set already imports `sourceguardtest`.
- Leave the three cleared non-candidates alone and do not re-flag them: `internal/theme/loader_construction_guard_test.go:64` switches on a type expression, `internal/tui/theme_panel_commit_slot_test.go:448` is not one, and `internal/portaltest`'s `selectorName`/`localCallName` render selectors as `pkg.func` so qualified and unqualified calls stay distinguishable — a property that guard depends on.

**Acceptance Criteria**:
- [ ] `countCalls` routes through `sourceguardtest.CalleeName` and declares no unwrapping switch of its own.
- [ ] The three call sites' counts are unchanged; each passes a string literal and never `""`, so the empty-unwrap case cannot move a count.
- [ ] No new import is added to `internal/capture`'s test set.
- [ ] `internal/portaltest`'s two qualified-name helpers are untouched.

**Tests**: pure refactor — the counts and the guard's verdict are unchanged.
- `internal/capture`'s swap-harness suite stays green with no assertion edited.
- `"it counts a qualified call and a bare call the same way it did before"`
- `"it leaves the portaltest qualified-name helpers distinguishing pkg.func from func"`

## Task 39: Three further AST primitives are re-authored across the guard family

severity: duplication
sources: bank

**Problem**: Beyond the enumerate-and-parse driver, three shapes recur with no shared home. (a) The const/var-spec walk `forEachValueSpec` (`cmd/doctor_stand_down_phrase_guard_test.go:210`) is re-authored at eight further files (`cmd/seam_guard_test.go`, `cmd/open_theme_nomination_test.go`, `cmd/prefs_translation_persist_test.go`, `cmd/capturetool/main_test.go`, `internal/tui/theme_source_guard_test.go`, `internal/tui/theme_flash_precedence_test.go`, `internal/theme/leaf_guard_test.go`, `internal/theme/theme_test.go`); `sourceguardtest` holds `ForEachFuncCall` but no value-spec sibling. (b) The import-list scan is re-authored at four guards (`internal/hooks/leaf_guard_test.go:31`, `internal/tui/theme_source_guard_test.go:26`, `internal/tui/restore_source_guard_test.go:48`, `internal/log/init_test.go:226`), each walking `file.Imports`, unquoting and matching against its own vocabulary, and the last additionally hand-rolling `parser.ParseFile(…, ImportsOnly)` rather than routing through the shared parse; an `AssertImportsWithin`-shaped sibling to `AssertDepsWithin` would carry the direct-import property the way that one carries the transitive one. (c) The in-memory fixture parse — `token.NewFileSet()` + `parser.ParseFile(…, sourceguardtest.ParseMode)` wrapped into a `ParsedSource` — is repeated at six sites (`internal/logtest/install_guard_test.go:158-166`, `internal/portalbintest/lane_guard_test.go:166-179`, `internal/portaltest/teardown_guard_coverage_rule_test.go:148`, `internal/sourceguardtest/foreachfunccall_test.go:91`, `internal/capture/theme_panel_fixture_test.go:478`, `cmd/state_daemon_lock_pid_ordering_test.go:115`). One further hand-rolled `go list` invocation also sits outside the family (`internal/tmux/target_composition_guard_test.go:60-93`) — a genuinely different query (whole-module listing with directories and immediate imports rather than a transitive closure), but now the only copy of the `go list` seam, error wrapping and empty-result fatal that `sourceguardtest` owns.

**Solution**: Add the three missing primitives to `internal/sourceguardtest` — a value-spec walk beside `ForEachFuncCall`, an import-set assertion beside `AssertDepsWithin`, and a fixture-parse entry point beside `ParseSources` — plus a module-listing primitive beside `PackageDeps`, and re-point the sites each one absorbs.

**Outcome**: The four AST and `go list` shapes the guard family repeats each have one home in `internal/sourceguardtest`, so a guard states its rule and reaches for a primitive rather than re-authoring one.

**Do**:
- Add a value-spec walk beside `ForEachFuncCall` and re-point `forEachValueSpec` (`cmd/doctor_stand_down_phrase_guard_test.go:210`) plus its eight re-authored copies in `cmd/seam_guard_test.go`, `cmd/open_theme_nomination_test.go`, `cmd/prefs_translation_persist_test.go`, `cmd/capturetool/main_test.go`, `internal/tui/theme_source_guard_test.go`, `internal/tui/theme_flash_precedence_test.go`, `internal/theme/leaf_guard_test.go` and `internal/theme/theme_test.go`.
- Add an `AssertImportsWithin`-shaped sibling to `AssertDepsWithin` carrying the direct-import property, and re-point the four import-list scans (`internal/hooks/leaf_guard_test.go:31`, `internal/tui/theme_source_guard_test.go:26`, `internal/tui/restore_source_guard_test.go:48`, `internal/log/init_test.go:226`), including the last one's hand-rolled `parser.ParseFile(…, ImportsOnly)`.
- Add an in-memory fixture-parse entry point beside `ParseSources` returning a `ParsedSource` under the shared `ParseMode`, and re-point the six sites: `internal/logtest/install_guard_test.go:158-166`, `internal/portalbintest/lane_guard_test.go:166-179`, `internal/portaltest/teardown_guard_coverage_rule_test.go:148`, `internal/sourceguardtest/foreachfunccall_test.go:91`, `internal/capture/theme_panel_fixture_test.go:478` and `cmd/state_daemon_lock_pid_ordering_test.go:115`.
- Add a module-listing primitive beside `PackageDeps` for the whole-module query with directories and immediate imports, and re-point `internal/tmux/target_composition_guard_test.go:60-93` onto it so the `go list` seam, error wrapping and empty-result fatal exist once.
- Take each primitive's fatal-on-nothing behaviour from the package's existing convention rather than restating it per site.

**Acceptance Criteria**:
- [ ] Each of the four shapes is declared once in `internal/sourceguardtest` and reached by every former copy.
- [ ] No guard hand-rolls `parser.ParseFile`, `token.NewFileSet` or a `go list` invocation.
- [ ] `ParseMode` is applied at one place for both on-disk and in-memory parses.
- [ ] Each re-pointed guard's verdict and wording are unchanged.
- [ ] Each new primitive refuses to pass over an empty result.

**Tests**: pure refactor of guard scaffolding — every guard's verdict is unchanged.
- `"it walks every value spec in a parsed file"`
- `"it reports a direct import outside the declared set"`
- `"it parses an in-memory fixture under the shared parse mode"`
- `"it lists the module's packages with their directories and immediate imports"`
- Every re-pointed guard's suite stays green with no assertion edited.

## Task 40: The `internal/log` discard guard is blind to test files and hand-rolls the shared walk

severity: duplication
sources: bank

**Problem**: `internal/log/discard_guard_test.go:35` returns early for any path ending `_test.go`, so the route-through-`log.OrDiscard`/`log.Discard` rule is production-only — which is how six open-coded `slog.NewTextHandler(io.Discard, nil)` copies survived in test files until they were fixed by hand, with nothing to stop the seventh. The same guard also walks the tree itself (`filepath.WalkDir` at `:24` with its own `.git`/`vendor`/`node_modules` exclusions) rather than through `sourceguardtest.GoSourceFiles`, the primitive introduced this phase for exactly that. Beside it, two capture handlers in `internal/log` remain structural twins of `logtest.Sink` — `recordingHandler` (`log_test.go`) and `componentCapture` (`rotate_test.go`) — kept apart only because both files are `package log` while `logtest` imports `internal/log`; CLAUDE.md's claim that `Sink` is the capture handler for every suite outside `internal/log` reads as universal because of them.

**Solution**: Widen the guard to `_test.go` files with an exemption for `internal/log`'s own in-package tests and a stated ruling on `internal/prefs` (which must not import `internal/log`), and route its enumeration through the shared walk. Separately weigh moving the two affected `internal/log` tests to an external test package so both handlers fold onto `Sink` — they touch unexported seams, so it is not a free move.

**Outcome**: The route-through-`OrDiscard`/`Discard` rule covers test files too, so a seventh open-coded discard handler fails a test rather than surviving until someone notices it.

**Do**:
- Delete the `_test.go` early return at `internal/log/discard_guard_test.go:35` so the rule covers test files, and exempt `internal/log`'s own in-package tests, which cannot route through the package they are inside.
- Rule explicitly on `internal/prefs`: it must not import `internal/log`, so its test files are exempt for a stated reason rather than by omission.
- Replace the guard's own `filepath.WalkDir` (`:24`) and its `.git`/`vendor`/`node_modules` exclusions with `sourceguardtest.GoSourceFiles`.
- Weigh moving `internal/log`'s `log_test.go` and `rotate_test.go` to an external test package so `recordingHandler` and `componentCapture` fold onto `logtest.Sink`; both touch unexported seams, so state the reach before deciding, and if they stay, re-voice CLAUDE.md's `logtest` row so its "every suite outside `internal/log`" claim is not read as universal.

**Acceptance Criteria**:
- [ ] The guard scans test files as well as production files and fails on an open-coded `slog.NewTextHandler(io.Discard, nil)` in either.
- [ ] `internal/log`'s in-package tests and `internal/prefs` are exempt for stated reasons, and no other package is.
- [ ] The guard enumerates through `sourceguardtest.GoSourceFiles` and declares no walk of its own.
- [ ] The two `internal/log` capture handlers are either folded onto `Sink` or left with the reach that prevented the move stated.
- [ ] CLAUDE.md's `logtest` row matches whichever outcome lands.

**Tests**:
- `"it fails for an open-coded discard handler in a test file"`
- `"it exempts internal/log's own in-package tests"`
- `"it exempts internal/prefs for its stated reason"`
- `"it enumerates through the shared repo walk"`

## Task 41: The TUI preview surface audit has become a repo-wide new-package tripwire

severity: low
sources: bank

**Problem**: `internal/tui/pagepreview_surface_audit_test.go` enumerates `internal/` and fails on any directory absent from a hardcoded ~30-entry allow-list, so a package added for **any** reason fails a TUI test with a message about the scrollback-preview feature. Its forbidden-name check (preview/scrollback/snapshot) is the part that actually pins the feature; the allow-list is maintenance drag on every future package, and this phase alone has edited it twice (`commandertest`, `harnesstest`).

**Solution**: Keep the forbidden-name check, which pins what the audit exists to pin, and retire the allow-list arm — or replace it with a rule that names the property rather than the inventory, so an unrelated package addition is not a TUI test failure.

**Outcome**: Adding a package to `internal/` for an unrelated reason no longer fails a TUI test about the scrollback preview, while the property that audit exists to pin still holds.

**Do**:
- Keep the forbidden-name check in `internal/tui/pagepreview_surface_audit_test.go` — the preview/scrollback/snapshot name rule — unchanged, since that is what pins the feature.
- Retire the hardcoded ~30-entry allow-list arm, or replace it with a rule stated as a property of the package rather than an inventory of directory names.
- Re-voice the test's failure message so it names the property a violation breaks rather than the allow-list it fell outside.
- Verify the surviving check still fails for a package or symbol that would reintroduce a second preview surface, so the retirement removes drag rather than coverage.

**Acceptance Criteria**:
- [ ] Adding an unrelated package under `internal/` does not fail this test.
- [ ] A package or symbol named for preview, scrollback or snapshot still fails it.
- [ ] The hardcoded directory inventory is gone, or is replaced by a stated property.
- [ ] The failure message names the property, not the list.

**Tests**:
- `"it passes when an unrelated package is added under internal/"`
- `"it fails for a package whose name claims a second preview surface"`
- `"it reports the property a violation breaks rather than the allow-list"`

## Task 42: The `cmd` doctor stand-down test inventory carries four line-counting scaffolds and two duplicated cases

severity: duplication
sources: bank

**Problem**: `countExactLines`/`countPrefixedLines` (`cmd/doctor_fix_hook_prune_report_test.go:97-112`) duplicate the inline split-and-scan loops in `assertStalePrunesApplied` (`cmd/doctor_test.go:859-869`), `assertSkippedPruneLine` (`cmd/doctor_test.go:1654-1667`) and `renderStaleHooksLine` (`cmd/doctor_stand_down_copy_test.go:153-158`) — one shape, four homes. Two of the four subtests in `doctor_fix_hook_prune_report_test.go` restate coverage that already exists: `:16-28` re-asserts what `assertStalePrunesApplied` pins at `cmd/doctor_test.go:860-869` for the same fixture, and `:30-36` is the same fixture and assertion as `cmd/doctor_test.go:1579-1582`. Separately, the `--fix` exit-code contract is asserted twice per reason from two directions in `cmd/doctor_stand_down_copy_test.go` (`:300-311` and `:398-425`), and `cmd/run_hook_stale_cleanup_outcome_test.go` now holds only `TestStaleHookVerdictParity`, so its filename names a subject it no longer covers.

**Solution**: Give the line-counting shape one pair of package-local helpers all four sites use; drop the two restated subtests in favour of the sibling assertions that already pin them; collapse the healthy half of the exit-code contract into one arm; and rename the outcome file to the subject it now holds.

**Outcome**: The doctor stand-down test inventory has one line-counting helper pair, no subtest restating coverage a sibling already carries, and a file name that matches what the file holds.

**Do**:
- Promote `countExactLines`/`countPrefixedLines` (`cmd/doctor_fix_hook_prune_report_test.go:97-112`) to package-local helpers and re-point the three inline split-and-scan loops onto them: `assertStalePrunesApplied` (`cmd/doctor_test.go:859-869`), `assertSkippedPruneLine` (`cmd/doctor_test.go:1654-1667`) and `renderStaleHooksLine` (`cmd/doctor_stand_down_copy_test.go:153-158`).
- Delete `cmd/doctor_fix_hook_prune_report_test.go:16-28` and `:30-36`, whose coverage `cmd/doctor_test.go:860-869` and `:1579-1582` already carry for the same fixtures.
- Collapse the healthy half of the `--fix` exit-code contract in `cmd/doctor_stand_down_copy_test.go` into one arm, keeping both directions' distinct subjects where they genuinely differ (`:300-311` and `:398-425`).
- Rename `cmd/run_hook_stale_cleanup_outcome_test.go` to the subject it now holds — the stale-hook verdict parity it is left with.
- Before deleting either subtest, confirm the sibling assertion covers the same fixture and the same property, so no coverage is lost to a name match.

**Acceptance Criteria**:
- [ ] One helper pair serves all four line-counting sites; no inline split-and-scan loop remains.
- [ ] The two restated subtests are gone and their properties are still pinned by the named sibling assertions.
- [ ] The healthy exit-code contract is asserted once per reason, with any genuinely distinct second direction kept and its distinction stated.
- [ ] The outcome file's name matches its remaining subject.
- [ ] The `cmd` doctor suite's overall coverage is unchanged.

**Tests**: pure refactor of the test inventory — no production behaviour and no surviving assertion's semantics change.
- `"it counts exact and prefixed lines through the shared helper pair"`
- `cmd/doctor_test.go`'s stale-prune and skipped-prune assertions stay green with only their counting lines rewritten.
- `cmd/doctor_stand_down_copy_test.go`'s exit-code contract stays green over the collapsed arm.
- The renamed verdict-parity file's test stays green unchanged.

## Task 43: `verifyLiveStructure` is authored twice across packages

severity: duplication
sources: bank

**Problem**: `internal/restore/integration_full_test.go:208` and `cmd/bootstrap/reboot_roundtrip_test.go:245` both list sessions, assert each expected name is present, then assert each expected window:pane coordinate — differing only in how the expected coordinates are generated (a fixed 2×2 versus a base-index-offset config). The coordinate read itself is already one `restoretest` helper; the surrounding "restore put these sessions and these panes here" assertion is the remaining duplicate.

**Solution**: Promote the assertion into `restoretest` over a shared expectation shape both callers construct, so the two packages assert restore's structural outcome through one description.

**Outcome**: "Restore put these sessions and these panes here" is asserted through one description in `restoretest`, with each caller supplying only the expectation its fixture generates.

**Do**:
- Declare an expectation shape in `internal/restoretest` — session names each with the window:pane coordinates expected under it — and an `AssertLiveStructure` over it that lists sessions, asserts each expected name is present, then asserts each expected coordinate through the existing coordinate-read helper.
- Re-point `internal/restore/integration_full_test.go:208` onto it, constructing its fixed 2×2 expectation.
- Re-point `cmd/bootstrap/reboot_roundtrip_test.go:245` onto it, constructing its base-index-offset expectation.
- Keep each caller's own failure context where it adds something the shared assertion cannot know, and let the shared one report the structural mismatch.

**Acceptance Criteria**:
- [ ] One `verifyLiveStructure`-shaped assertion exists, in `restoretest`, and both packages call it.
- [ ] Each caller constructs its own expectation — fixed grid and base-index-offset alike — and neither restates the assertion.
- [ ] Both fixtures fail on the same structural violations they failed on before, with a message naming the missing session or coordinate.
- [ ] The coordinate read still routes through the existing `restoretest` helper.

**Tests**: pure refactor — both fixtures' verdicts are unchanged.
- `"it reports a missing restored session"`
- `"it reports a missing window:pane coordinate under a restored session"`
- `"it passes for a fully reconstructed structure under either expectation shape"`
- Both callers' integration fixtures stay green with only their assertion call rewritten.

## Task 44: Every `cmd/bootstrap` and `internal/tmux` fixture re-sets the state dir the helper already sets, and a harness field is dead

severity: duplication
sources: bank

**Problem**: `IsolateStateForTest` now owns the `PORTAL_STATE_DIR` value, and the fixtures still restate it: `t.Setenv("PORTAL_STATE_DIR", stateDir)` appears at 222 sites tree-wide, including `cmd/bootstrap/helpers_integration_test.go:21`, `upgrade_path_integration_test.go:31,107,153`, `orphan_sweep_integration_test.go:48,105,160`, `composition_abc_integration_test.go:29`, `composition_e2e_harness_integration_test.go:115`, `internal/tmux/portal_saver_endstate_integration_test.go:35,99,168` and `kill_barrier_escalation_no_final_flush_integration_test.go:40,60` — each setting the variable to the value it already holds. Deleting them has one live consequence worth naming in the same change: the teardown-guard coverage rule's `NamesStateDir` trigger keys on the literal string, so a file losing its last mention qualifies via the `Isolates` trigger alone — every one of these files calls `IsolateStateForTest`, so coverage is unchanged, but the reasoning belongs in one deliberate pass. Separately, `compositeHarness.Env` (`cmd/bootstrap/composition_e2e_harness_integration_test.go:25`) is set at `:154` and read nowhere.

**Solution**: Sweep the redundant re-sets in one pass, stating the coverage-trigger reasoning once, and delete the dead harness field.

**Outcome**: A fixture states its state directory once — through `IsolateStateForTest` — and the coverage rule that keyed on the literal string is understood to be satisfied by the `Isolates` trigger, recorded once rather than rediscovered per file.

**Do**:
- Delete the redundant `t.Setenv("PORTAL_STATE_DIR", stateDir)` calls that re-set the value `IsolateStateForTest` already owns, across `cmd/bootstrap/helpers_integration_test.go:21`, `upgrade_path_integration_test.go:31,107,153`, `orphan_sweep_integration_test.go:48,105,160`, `composition_abc_integration_test.go:29`, `composition_e2e_harness_integration_test.go:115`, `internal/tmux/portal_saver_endstate_integration_test.go:35,99,168`, `kill_barrier_escalation_no_final_flush_integration_test.go:40,60` and the remaining sites of the 222 that match the same shape.
- Keep every `t.Setenv` that deliberately overrides the helper's value with a different directory — those are the fixtures the helper's own doc invites to override.
- Before the sweep, confirm each affected file still qualifies for the teardown-guard coverage rule through the `Isolates` trigger now that it loses its last `NamesStateDir` mention, and record that reasoning once rather than per file.
- Delete `compositeHarness.Env` (`cmd/bootstrap/composition_e2e_harness_integration_test.go:25`) and its assignment at `:154`.

**Acceptance Criteria**:
- [ ] No fixture sets `PORTAL_STATE_DIR` to the value `IsolateStateForTest` already set.
- [ ] Every fixture that deliberately overrides it with a different directory still does.
- [ ] Every file that loses its last literal mention still satisfies the teardown-guard coverage rule, and the reasoning is recorded once.
- [ ] `compositeHarness.Env` is gone, with no reader left behind.
- [ ] The integration lane passes with no fixture's resolved state directory changed.

**Tests**: pure refactor of fixture setup — no assertion semantics change.
- `"it keeps teardown-guard coverage through the Isolates trigger for a file with no state-dir literal"`
- The `cmd/bootstrap` composition, upgrade-path and orphan-sweep integration suites stay green with no assertion edited.
- The `internal/tmux` saver end-state and kill-barrier integration suites stay green with no assertion edited.

## Task 45: `openTestLogger` in `internal/state` takes an argument nothing reads

severity: dead-code
sources: bank

**Problem**: `internal/state/fifo_sweep_test.go:15` declares `openTestLogger(t *testing.T, dir string)` whose body is `_ = dir; return logtest.NewCaptureLogger(t)`. Four call sites pass a directory that is discarded — `fifo_sweep_test.go:140` and `capture_test.go:850` pass a real one, `capture_colon_session_test.go:23` and `capture_colon_session_realtmux_test.go:38` pass a throwaway `t.TempDir()` purely to satisfy the parameter, so the pass-through is manufacturing work at two of its four sites. `internal/restore` took exactly this fix already.

**Solution**: Delete the pass-through and call `logtest.NewCaptureLogger(t)` directly at the four sites, dropping the two throwaway temp dirs with it.

**Outcome**: `internal/state`'s test logger is constructed the way `internal/restore`'s already is, with no parameter nothing reads and no temp directory manufactured to satisfy one.

**Do**:
- Delete `openTestLogger` (`internal/state/fifo_sweep_test.go:15`).
- Call `logtest.NewCaptureLogger(t)` directly at `fifo_sweep_test.go:140`, `capture_test.go:850`, `capture_colon_session_test.go:23` and `capture_colon_session_realtmux_test.go:38`.
- Drop the two throwaway `t.TempDir()` calls at the last two sites, which existed only to satisfy the parameter.
- Leave the two sites that pass a real directory using it for whatever else they need it for, if anything; if a site's directory is now unused, drop that too.

**Acceptance Criteria**:
- [ ] `openTestLogger` no longer exists in `internal/state`.
- [ ] All four sites construct the capture logger directly.
- [ ] No `t.TempDir()` remains whose only consumer was the deleted parameter.
- [ ] The four suites' captured records and assertions are unchanged.

**Tests**: dead-parameter removal — no assertion semantics change.
- `internal/state`'s FIFO sweep suite stays green with no assertion edited.
- `internal/state`'s capture suite stays green with no assertion edited.
- `internal/state`'s colon-session suites, unit and real-tmux, stay green with no assertion edited.

## Task 46: The function-seam staging pattern is guarded and helped in `cmd` only

severity: duplication
sources: bank

**Problem**: `withFuncSeam` plus `cmd/seam_guard_test.go` make the capture-and-restore of a package-level seam structural inside `cmd`, and three gaps sit outside that. (a) The same seam pattern exists elsewhere with no guard and no helper: `internal/state/daemon_identity.go:35` (`identifyPS`) and `internal/state/pgrep.go:13` (`pgrepCommand`) are installed by hand across eight sites in that package's tests, and `internal/portaltest/isolated_env.go:117` and `teardown_guard.go:69` the same way. (b) Inside `cmd`, the identical install-and-restore is still hand-written for package-level state that is not a function — `fatalErrorStderr` (`root_test.go:618,651`), `hydrateLogger`, `version`, `daemonLockFile`, and the `bootstrapOnce`/`bootstrapStarted`/`bootstrapWarnings*` block in `bootstrap_reset_test.go:14-24` — even though `withFuncSeam[F any]` constrains nothing but its name. (c) Six per-seam wrappers are now one-line pass-throughs to `withFuncSeam`.

**Solution**: Hoist the staging helper into the neutral test leaf that now exists (`internal/harnesstest`) and point a guard at the packages outside `cmd` that carry seams; then decide the non-function case deliberately — a family-neutral name plus a third guard recogniser arm, which has a real false-positive surface (every `log.For` binding and every cobra command var is a package-level var), or an explicit ruling that non-function state stays hand-staged. The six thin wrappers earn their keep as named vocabulary at their call sites and are a judgement call rather than an obvious deletion.

**Outcome**: The capture-and-restore of a package-level seam is a shared helper with a guard behind it in every package that carries seams, not a `cmd`-only convention with three gaps beside it.

**Do**:
- Move `withFuncSeam` into `internal/harnesstest` under a family-neutral name, keeping its restore-the-captured-production-default behaviour, and re-point `cmd`'s call sites and its six thin per-seam wrappers onto it.
- Point a guard at the packages outside `cmd` that carry seams — `internal/state` (`daemon_identity.go:35`'s `identifyPS`, `pgrep.go:13`'s `pgrepCommand`) and `internal/portaltest` (`isolated_env.go:117`, `teardown_guard.go:69`) — failing any `*_test.go` that assigns one directly, and convert the eight-plus hand-written install-and-restore sites in those packages.
- Decide the non-function case deliberately and record the decision: either extend the guard with a third recogniser arm covering package-level non-function state (`fatalErrorStderr`, `hydrateLogger`, `version`, `daemonLockFile`, and the `bootstrapOnce`/`bootstrapStarted`/`bootstrapWarnings*` block in `cmd/bootstrap_reset_test.go:14-24`), accepting the false-positive surface every `log.For` binding and cobra command var presents, or rule explicitly that non-function state stays hand-staged.
- Judge the six thin wrappers on whether their names earn their keep at their call sites; keep them if they do, and say so rather than deleting them for being one line.
- Keep `TestMain`'s exemption intact: it installs package-wide with no `*testing.T` to restore into.

**Acceptance Criteria**:
- [ ] The staging helper lives in `internal/harnesstest` and is the route every seam install takes in `cmd`, `internal/state` and `internal/portaltest`.
- [ ] A `*_test.go` assigning a seam directly in any of those packages fails a guard.
- [ ] The non-function case is settled either way, with the decision and its false-positive reasoning recorded.
- [ ] Every restore puts back the captured production default, never a zero value.
- [ ] `cmd/seam_guard_test.go` still derives its seam families from the production sources rather than a hand list.

**Tests**: pure refactor of test staging, plus a widened guard.
- `"it restores the captured production default rather than the zero value"`
- `"it fails when a test in internal/state assigns a package-level seam directly"`
- `"it fails when a test in internal/portaltest assigns a package-level seam directly"`
- `"it exempts TestMain's package-wide installs"`

## Task 47: The `hook rm` cases are split across three files by no visible rule, and one duplicate hides a safety assertion

severity: duplication
sources: bank

**Problem**: `cmd/hooks_test.go:600` ("--pane-key unset falls back to resolveCurrentPaneKey") is a strict subset of `cmd/hooks_rm_exit_test.go:246`, the resolved-token removal home. **A second pair is not a duplicate and must not be collapsed naively**: `cmd/hooks_test.go:571` removes a token-shaped key and asserts an unjudgeable-shaped sibling **survives** (`:594-595`), while `cmd/hooks_rm_exit_test.go:274` removes the unjudgeable key and asserts nothing about its sibling. That surviving-sibling assertion is the sole remaining home of the unjudgeable-key retention property — a stated safety invariant of this codebase, where deleting such an entry is data loss with no route back — so a pick-one-home collapse would silently drop it. Separately, `cmd/hooks_write_lock_test.go:224-236` hand-rolls its staging (SetLockTimeoutForTest + temp file + Setenv + sidecar hold + deps) duplicating `runRmCase` (`cmd/hooks_rm_exit_test.go:76-105`), solely because `assertReturnsAtLockBound` takes a `func() error` it must time — and that helper is shared with the `hook set` timeout test, so the fix reaches both sides.

**Solution**: Collapse the confirmed duplicate into the removal home, **carrying the unjudgeable-sibling assertion into the surviving case**; give the timed case a route through the shared staging by timing the drive from inside it (an elapsed field on the outcome, or a reshaped bound assertion) rather than staging by hand; and settle what `hooks_test.go` keeps now that `hooks_rm_exit_test.go` owns the `hook rm` contract and `hooks_write_lock_test.go` owns the locked routes.

**Outcome**: The `hook rm` cases sit in files whose ownership is visible, the unjudgeable-key retention property still has a home, and the timed case stages through the same helper as its siblings.

**Do**:
- Delete `cmd/hooks_test.go:600` ("--pane-key unset falls back to resolveCurrentPaneKey"), a strict subset of `cmd/hooks_rm_exit_test.go:246`, keeping the latter as the resolved-token removal home.
- Do **not** collapse `cmd/hooks_test.go:571` into `cmd/hooks_rm_exit_test.go:274`: carry its surviving-sibling assertion (`:594-595`) into the surviving case, so the unjudgeable-key retention property — a stated safety invariant where deletion is data loss with no route back — keeps a home.
- Give `runRmCase` (`cmd/hooks_rm_exit_test.go:76-105`) an elapsed measurement (a field on its outcome, or a reshaped bound assertion) so `assertReturnsAtLockBound` can time a drive from inside the shared staging, and re-point `cmd/hooks_write_lock_test.go:224-236` off its hand-rolled staging; carry the same reshape to the `hook set` timeout test that shares the helper.
- Settle and record what `cmd/hooks_test.go` keeps now that `hooks_rm_exit_test.go` owns the `hook rm` exit contract and `hooks_write_lock_test.go` owns the locked routes, moving any remaining case that belongs to one of those owners.
- Verify the retention property is still asserted by exactly one case after the move, by name.

**Acceptance Criteria**:
- [ ] The confirmed duplicate is gone and its property is covered by the surviving case.
- [ ] The unjudgeable-key retention assertion survives in exactly one named case and is not lost to the collapse.
- [ ] The timed lock-bound case stages through `runRmCase` and hand-rolls nothing.
- [ ] The `hook set` side of `assertReturnsAtLockBound` still times its drive and still asserts the bound.
- [ ] Each of the three files' ownership is stated and its remaining cases match it.

**Tests**: test-inventory refactor — the surviving assertions' semantics are unchanged.
- `"it removes a token-shaped key and retains its unjudgeable-shaped sibling"`
- `"it removes an entry resolved from the current pane when --pane-key is unset"`
- `"it returns at the lock bound when the sidecar is held, staged through the shared helper"`
- `"it returns at the lock bound on the hook set path through the same assertion"`

## Task 48: `cmd` config-path tests carry a provably duplicated subtest and hardcoded absolute paths

severity: duplication
sources: bank

**Problem**: `cmd/config_test.go:13` and `:58` have byte-identical bodies — same temp HOME, same `XDG_CONFIG_HOME=""`, same `configFilePath(TEST_CONFIG_UNSET, "projects.json")`, same expectation — because `t.Setenv` cannot express genuinely-unset, so the "no vars at all" versus "explicitly empty" distinction the two names imply does not exist. Separately, eight subtests across three files assert against hardcoded absolute paths rather than `t.TempDir()`: `cmd/config_test.go:45,75,91`, `cmd/prefs_path_test.go:13,29` and `cmd/config_themes_test.go:14,29,44` use `/tmp/xdg-config`, `/tmp/cfg` and `/tmp/h`. Inert today, but it is the shared-mutable-path shape, in the same files as the isolation gap in Task 3.

**Solution**: Fold the two identical subtests into one whose name states the case that actually exists, and re-point the hardcoded paths onto per-test temp dirs.

**Outcome**: The config-path suite has one subtest per case that actually exists, and no subtest asserts against a shared absolute path under `/tmp`.

**Do**:
- Fold `cmd/config_test.go:13` and `:58` — byte-identical bodies — into one subtest whose name states the case that exists (`XDG_CONFIG_HOME` explicitly empty), since `t.Setenv` cannot express genuinely unset and the two names imply a distinction that does not exist.
- Replace the hardcoded `/tmp/xdg-config`, `/tmp/cfg` and `/tmp/h` paths at `cmd/config_test.go:45,75,91`, `cmd/prefs_path_test.go:13,29` and `cmd/config_themes_test.go:14,29,44` with per-test `t.TempDir()` values.
- Derive each expectation from the temp path rather than from a literal, so the assertions stay exact.
- Confirm each converted subtest still asserts the same resolution rule it did before — the change is the path's provenance, not the rule.

**Acceptance Criteria**:
- [ ] `cmd/config_test.go` has one subtest for the empty-`XDG_CONFIG_HOME` case, named for what it actually covers.
- [ ] No subtest in the three files asserts against a shared absolute path under `/tmp`.
- [ ] Every converted expectation is derived from the test's own temp directory.
- [ ] The resolution rules asserted are unchanged.

**Tests**: pure refactor of the test inventory — no resolution behaviour changes.
- `"it resolves the default config base when XDG_CONFIG_HOME is empty"`
- `"it resolves under a per-test XDG_CONFIG_HOME temp directory"`
- `"it resolves the prefs and themes paths under per-test temp directories"`

## Task 49: The read-lock bound pin asserts a claim its own probe falsifies, and its half-relation is integer-division-fragile

severity: low
sources: bank

**Problem**: `internal/hooks/read_lock_test.go:369-371` fails `preRead <= 0` with the message "it must still grant an uncontended lock" — but an empirical probe shows an uncontended acquire is granted at bound 0 **and** at negative bounds, so a positive figure is not what grants it. The assertion actually pins the floor, and its message names something else. Separately, `:373` asserts `preRead >= mutation/2` using Go integer division, which trips at a `lockTimeout` of exactly 10ms (`10000001ns/2 == 5000000ns == bound`) even though the claim holds in real arithmetic; the three sampled values (2s/300ms/60ms) are all far from that edge, so the fragility is latent rather than live.

**Solution**: Re-voice the first assertion to say what it pins — the floor — and express the half-relation so it does not turn on integer division at the crossover.

**Outcome**: The read-lock bound pin says what it actually pins, and its half-relation holds at every `lockTimeout` rather than tripping at one integer-division crossover.

**Do**:
- Re-voice `internal/hooks/read_lock_test.go:369-371`'s failure message so it names the floor it pins, replacing "it must still grant an uncontended lock" — a claim the empirical probe falsifies, since an uncontended acquire is granted at bound 0 and at negative bounds.
- Re-express the `preRead >= mutation/2` assertion at `:373` so it does not turn on Go integer division — compare `2*preRead >= mutation`, or the equivalent — so a `lockTimeout` of exactly 10ms no longer trips a claim that holds in real arithmetic.
- Keep the three sampled `lockTimeout` values and add the crossover value the current form trips at, so the fragility is covered rather than left latent.
- Change no production value: `lockTimeout`, `lockPollInterval` and `snapshotLockFraction` are untouched.

**Acceptance Criteria**:
- [ ] The floor assertion's message names the floor.
- [ ] The half-relation holds at a `lockTimeout` of exactly 10ms, which the current form fails.
- [ ] The three existing sampled values still pass, and the crossover value is among the sampled set.
- [ ] No production bound value changes.

**Tests**:
- `"it pins the pre-read bound at the poll-interval floor"`
- `"it holds the half-relation at the integer-division crossover bound"`
- `"it holds the half-relation at the production bound and the lowered test bounds"`
- `"it grants an uncontended acquire whatever the bound"`

## Task 50: The rename-refusal copy is a literal in four places with nothing tying them together

severity: medium
sources: bank

**Problem**: The refusal wording lives as constants at `internal/tui/sessions_flash.go:59-60`, and again as literals in the fixture tables at `internal/capture/capture_test.go:926,930` and `internal/capture/swap_harness_test.go:45-46`, and again as prose in `README.md:197`. The test copies are unavoidable today (the constants are unexported and the tables live in another package), but a wording change currently leaves the README stale with **no failing test** — and CLAUDE.md states the coupling as discipline ("user-visible copy, so a change to either must move both"), which is exactly the shape this work unit exists to replace with something structural.

**Solution**: Either export the two strings as a small vocabulary the capture tables read, or add a unit-lane guard asserting the README contains both — the same shape as the repo's other source guards — so the discipline is enforced rather than written down. While there, the refusal is documented only in the README's `hook` section: a user who never registers a hook and meets the refusal from the picker's `r` will not find it, and the keymap table carries no pointer.

**Outcome**: A change to the rename-refusal wording fails a test rather than leaving the README stale, and a user who meets the refusal from the picker's `r` can find it documented where they are looking.

**Do**:
- Export the rename-refusal wordings from `internal/tui/sessions_flash.go:59-60` as a small vocabulary, and have the `internal/capture` fixture tables (`capture_test.go:926,930`, `swap_harness_test.go:45-46`) read them instead of restating the literals.
- Add a unit-lane guard, in the shape of the repo's other source guards, asserting `README.md` contains each exported refusal string, so a re-wording that leaves the README behind fails.
- Have the guard range over the exported vocabulary rather than a hand list, so a refusal added by a sibling task is covered without editing it.
- Document the refusal where a user who never registers a hook will meet it: alongside the picker's `r` key in the README, and give the keymap table a pointer to it.
- Replace CLAUDE.md's "user-visible copy, so a change to either must move both" discipline sentence with what the guard now enforces.

**Acceptance Criteria**:
- [ ] The refusal wordings are exported once and read by the capture fixture tables.
- [ ] A re-wording that does not move the README fails the guard.
- [ ] The guard enumerates the exported vocabulary, so a newly added refusal is covered with no guard edit.
- [ ] The README documents the refusal in the picker's `r`-key context, not only in the `hook` section, and the keymap table points at it.
- [ ] CLAUDE.md states the enforcement rather than the discipline.

**Tests**:
- `"it fails when the README omits a rename-refusal wording"`
- `"it covers a refusal added to the vocabulary without editing the guard"`
- `"it renders the capture fixtures from the exported wordings"`
- `"it passes for a README carrying every exported refusal"`

## Task 51: The source-reading guards certified under `go test -overlay` are unverified

severity: medium
sources: bank

**Problem**: Confirmed with a standalone module rather than by argument: `-overlay` substitutes the go command's **build** inputs, while `sourceguardtest.ParsePackageSources` reaches disk directly (`parser.ParseFile(fset, path, nil, ParseMode)` with a nil `src`, `internal/sourceguardtest/parsesources.go:50`) and the test binary's working directory is the real package directory. A probe test that reads a map value at runtime **and** parses its own source from disk, run under an overlay, logs the overlaid runtime value alongside the original parsed-from-disk literal. So an overlay probe of a source-reading guard proves only the no-false-positive half; the biting half passes vacuously, and for a guard that compares parsed literals against sibling values from the compiled package it is worse than useless — the overlay makes the two halves disagree and the result is uninterpretable in either direction. Any round in this phase that certified a source-reading guard by overlay should be treated as unverified, which reaches the ~20 `sourceguardtest`-driven guards.

**Solution**: Re-verify each source-reading guard by the method that works — a scratch-copy edit that introduces the violation and observes the failure — and record the method beside the guard family so the next round does not reach for an overlay again.

**Outcome**: Every source-reading guard in the tree has been observed to fail against a real violation, by a method that actually reaches what the guard reads, and the method is recorded where the next round will look.

**Do**:
- Enumerate the source-reading guards driven by `sourceguardtest` — roughly twenty — and treat any certified by a `go test -overlay` probe as unverified.
- Re-verify each by scratch-copy: copy the tree to a scratch location, introduce the violation the guard exists to catch, run that guard there, and observe the failure; revert by discarding the copy, never by editing back.
- Give particular attention to guards that compare parsed literals against sibling values from the compiled package, where an overlay makes the two halves disagree and the result is uninterpretable in either direction.
- Record the verification method beside the guard family — in `internal/sourceguardtest`'s package documentation — stating that `-overlay` substitutes the go command's build inputs while `ParsePackageSources` reaches disk directly (`internal/sourceguardtest/parsesources.go:50`, a nil `src`), so an overlay probe proves only the no-false-positive half.
- Report any guard the re-verification finds does not actually fail against its violation as a defect, with the guard named.

**Acceptance Criteria**:
- [ ] Every `sourceguardtest`-driven source-reading guard has been observed failing against a real violation introduced in a scratch copy.
- [ ] No guard's certification rests on a `go test -overlay` probe.
- [ ] The verification method and the reason an overlay cannot serve are recorded in `internal/sourceguardtest`'s package documentation.
- [ ] Any guard found not to bite is named and either fixed or raised.
- [ ] The working tree is left unmodified by the verification itself.

**Tests**:
- `"it fails each source-reading guard against a violation introduced in a scratch copy"`
- `"it passes each source-reading guard against the unmodified tree"`
- `"it names any guard that did not fail against its own violation"`

## Task 52: CLAUDE.md's architecture rows carry three claims the tree has moved past

severity: comments
sources: bank

**Problem**: Three documented statements are now inaccurate or unenforceable. (a) The `tmux` row's `SessionTargetExact` call-site enumeration lists has-session, kill-session, rename-session's `-t`, switch-client, show-environment, set-environment and list-clients, and reads as complete — but `attach-session` also takes one, at `cmd/open.go:88` and `internal/session/quickstart.go:62`, the latter being precisely the "a caller composing a tmux argv the client does not run — an exec chain, say" case the helper's own doc calls out. (b) The `session` row still describes name generation as `{project}-{nanoid}` with no mention that the fragment is sanitised (`SanitiseProjectName` replaces `.` and `:` with `-` and maps a leading `$`), nor that generation is pinned to the recogniser — machinery three sibling tasks in this work unit built. (c) The `logtest` row carries two count-shaped claims of the class the consolidation existed to remove — "the two `slog.Handler`s still declared under `cmd`" and "the five properties every audit-trail line shares" — each true today and each silently falsifiable by an addition with nothing to catch it; and its "`Sink` is the capture handler for every suite outside `internal/log`" is slightly overstated, since `internal/hooks/store_test.go:1239` deliberately captures through a stdlib JSON handler to assert the JSON rendering a `Sink` does not produce.

**Solution**: Add `attach-session` to the enumeration; give the `session` row the sanitisation and generation-pinning sentence; and re-voice the `logtest` row's claims so they name without counting, accounting for the deliberate JSON-handler exception.

**Outcome**: CLAUDE.md's three drifted claims match the tree, and none of the three is expressed as a count an ordinary addition would falsify.

**Do**:
- Add `attach-session` to the `tmux` row's `SessionTargetExact` call-site enumeration, naming both sites — `cmd/open.go:88` and `internal/session/quickstart.go:62` — and note the latter as the "a caller composing a tmux argv the client does not run" case the helper's own doc calls out.
- Give the `session` row a sentence covering `SanitiseProjectName` — that a generated name's project fragment is sanitised before the nanoid is appended — and that generation is pinned to the token recogniser.
- Re-voice the `logtest` row's two count-shaped claims — "the two `slog.Handler`s still declared under `cmd`" and "the five properties every audit-trail line shares" — so they name what the handlers and properties are without asserting how many there are.
- Correct the `logtest` row's "`Sink` is the capture handler for every suite outside `internal/log`" to account for the deliberate exception at `internal/hooks/store_test.go:1239`, which captures through a stdlib JSON handler to assert a rendering a `Sink` does not produce.
- Re-read each edited row against the tree before landing, so a correction does not introduce a fresh claim.

**Acceptance Criteria**:
- [ ] The `tmux` row's enumeration includes `attach-session` and both of its sites.
- [ ] The `session` row describes sanitisation and the generation-to-recogniser pinning.
- [ ] Neither re-voiced `logtest` claim states a count.
- [ ] The `Sink`-everywhere claim accounts for the JSON-handler exception by name.
- [ ] Every statement in the edited rows is true against the tree at the time of the edit.

**Tests**: documentation-only edit — no code and no test semantics change.
- The `internal/tmux` target-composition guard stays green with no edit, and its call-site set matches the corrected enumeration.
- `internal/session`'s naming and generation suites stay green with no edit, and match the corrected row.
- `internal/logtest`'s and `internal/hooks`'s suites stay green with no edit, including the deliberate JSON-handler case.
