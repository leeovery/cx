# Analysis Tasks: Resume Hooks Silently Lost (Cycle 4)

## Task 1: The saver-readiness barrier fails open on a fixed 2s budget, so a cold boot can proceed with no daemon

severity: high
sources: bank

**Problem**: `waitForSaverDaemonReady` (`internal/tmux/portal_saver.go:226`) polls `isSaverDaemonReady` against `saver.Readiness.Timeout`, hardcoded to `2 * time.Second` at `portal_saver.go:109`. On expiry it emits one WARN and **returns nil**, so `BootstrapPortalSaver` continues as if the daemon came up and every step after it addresses a saver that never started — the state-capture loop silently does not run. Under host IO contention, fork+exec+flock+PID-file-write is not reliably a 2s operation. Two independent investigations (executor and reviewer of task 8-15) reached this from opposite directions, and the log signature is unambiguous: a failing run shows `respawn-daemon to_pid=X` with no `daemon ready` line and all daemons dead. It is also the single root cause of the composite-e2e flake family banked five separate times across this phase — `TestCompositeBootstrap_ConvergesPgrepToOneDaemon`, `_FObservables`, `_FreshAcquireDaemonLockRefusesPostBootstrap` and `_ExternalSaverKillTriggersSelfEject` all fail with the same `pgrep did not converge to 1 … count=0` signature, reproduced on a pristine `git archive` export of HEAD (3-in-10 under load, 0-in-10 idle), which rules out any change in this work unit.

**Solution**: Replace the fixed wall-clock readiness budget with a progress-based wait bounded by a hard ceiling — the same `Stall`/`Ceiling` separation `harnesstest.ProgressWait` already expresses for the test side — so a machine that is merely slow does not decide the verdict, and give the expiry a consequence: the barrier reports that the daemon did not come up rather than returning nil. The seam is already injectable (`SaverReadinessSeams`, `SaverOperationSeams.WaitForReady`, exported through `internal/tmux/export_test.go`), so the pacing stays drivable from tests. Whether the expiry becomes a returned error or stays best-effort-with-a-louder-signal follows from what `BootstrapPortalSaver`'s caller can do about it — bootstrap never escalates a saver failure to a fatal, so the honest shape is a `SaverDownWarning` the user sees rather than a silent continue.

**Outcome**: A cold boot on a loaded machine either brings the daemon up or tells the user it did not; the four composite-bootstrap integration tests stop flaking against the thing they wait on having given up.

## Task 2: A period-bearing session name is unaddressable through `SessionTargetExact` and is silently dropped from `sessions.json`

severity: high
sources: bank

**Problem**: Measured on tmux 3.7c against isolated `-L` sockets: `show-environment -t =a.b` succeeds on a server holding only `a.b`, but fails with `no such session: =a.b` as soon as any **other** session name is a longer prefix-extension of the pre-period component (`anchor`, `apple` reproduce it; `a`, `b`, `zzz` do not). tmux splits a colon-free target on `.` into `window.pane`, so the session lookup is a fallback the prefix candidate displaces. Reproduced Portal-shaped: with `_portal-bootstrap`, `my-cool-app-abc123` and a user session `my.app` live, `show-environment -t =my.app` exits 1 — which `internal/state/capture.go:70-74` classifies as `ErrNoSuchSession`, counts as natural churn, logs `capture skipping vanished session` and **drops a live session and all its scrollback from `sessions.json`**. `has-session` and `kill-session` fail identically. This is the work unit's own failure mode — a silent loss whose only trace is a line that says the thing was gone — reached through a different door. Generated names are safe (`session.SanitiseProjectName` replaces `.` with `-`), so the exposure is user-created and user-renamed sessions, and `ValidateSessionName` accepts a period by design, pinned at `internal/tmux/session_name_test.go:225`.

**Solution**: Audit the remaining `SessionTargetExact` call sites per command and move each one tmux parses as a window-or-pane target onto `CoordTargetExact` (`=name:`), which the same measurement resolves correctly in every tested case — `internal/tmux/tmux.go:88,97,272,287,295,589,780` and `internal/tmux/clients.go:19`, plus the two hand-composed argv sites at `cmd/open.go:88` and `internal/session/quickstart.go:62`. The choice is per command, measured, exactly as `SessionTargetExact`'s own doc comment instructs ("Measure a command before choosing"); the validator is not widened, so `a.b` stays a legal session name and its acceptance test stays green.

**Outcome**: A user session whose name contains a period is captured, killed, renamed and switched to correctly whatever else is live on the server, instead of being reported as vanished by whichever command reads it first.

## Task 3: `cmd`'s `TestMain` does not poison `HOME`, so a test resolving a default config path moves the developer's real config files

severity: high
sources: bank

**Problem**: `cmd/config.go`'s `configFilePath` runs the one-shot Application Support migration as a side effect of **any** non-overridden resolve, and that migration `os.Rename`s files under `~/Library/Application Support/portal/` into `~/.config/portal/`. `cmd/testmain_isolation_test.go` poisons the five `PORTAL_*` path variables and `TMUX` package-wide precisely so a missed isolation fails loudly — but not `HOME`, which is the boundary this whole class needs. Isolation there is per-subtest discipline (`cmd/config_test.go:15-17` carries a comment asking every subtest to pin a temp `HOME`), and this work unit contains a task that exists because that discipline slipped. The project's ABSOLUTE INVARIANT is that a test must never mutate the real filesystem outside its temp dirs; today one forgotten `t.Setenv("HOME", …)` in a new `cmd` test does exactly that, silently, and only on a machine that still has the old macOS directory.

**Solution**: Poison `HOME` package-wide in `cmd`'s `TestMain` the way the `PORTAL_*` variables and `TMUX` already are — a per-run temp directory rather than a `/nonexistent` path, since the migration and the default resolve both need a home that exists to be observed against. Every subtest that pins its own temp `HOME` keeps working unchanged; a subtest that forgets one reads the poisoned home instead of the developer's.

**Outcome**: The migration-against-the-real-home hazard is structural rather than disciplinary, in the same shape as the existing tmux poison, and the per-subtest comment asking for the pin can say what it protects rather than being the only thing that protects it.

## Task 4: `ValidateSessionName` accepts a leading `-`, which tmux refuses as a rename target

severity: medium
sources: bank

**Problem**: Measured on tmux 3.7c: `rename-session -t =old -bar-abc123` fails with `unknown flag -b` — the new name is a bare positional at `internal/tmux/tmux.go:287` and Portal composes no `--` terminator. `ValidateSessionName` returns nil for such a name, so the picker's `r` modal lets it through to a failure worded by tmux as a flag error rather than by Portal as a refusal. The name is reachable in ordinary use: `session.SanitiseProjectName` maps a `$`-leading project directory to a `-`-leading fragment, and an empty project name yields `-abc123`, so Portal's own generator mints names of this shape (safe where they land, as `-s`/`-t` optargs, but they then show in the picker as rename sources and as names a user may retype). This is the same two-definitions-of-an-unwritable-name class as the `:` and `$` rules, on a third character.

**Solution**: Settled: the refusal, wherever it lands, is reported through the existing `ErrUnaddressableSessionName` machinery with a rule sentinel of its own so `renameRefusalFlash` names the offending character the way it already does for `:` and `$`, and `SanitiseProjectName` stays the single home of what a generated fragment may contain. What is not settled is whether a leading hyphen is a name Portal supports at all.

**Decision**: Does Portal support a session name beginning with `-`, or refuse it?

**Stakes**: Refusing costs the user a name tmux itself accepts, and requires `SanitiseProjectName` to stop minting one (a `$`-leading or empty project directory would then produce a differently-shaped name than it does today, which is a visible change to generated names). Supporting it costs an audit of every argv that passes a session name as a bare positional — `rename-session`'s new-name slot today, and any future one — with a `--` terminator or equivalent at each, and leaves a class that must be re-checked whenever a new command is composed. No measurement settles it: both behaviours are implementable and neither is observably wrong. The convention leans toward refusal — Portal already refuses `:` and `$` rather than engineering around them — but those two are genuinely unaddressable, whereas `-foo` addresses fine as `=-foo` and fails only in the positional slot, so the precedent does not decide this one. The tie-break is product intent: whether the set of names Portal accepts is "what tmux accepts" or "what Portal can address cleanly everywhere".

1. A session name may not begin with `-`: the picker refuses the rename in its notice band naming the hyphen, and Portal never generates one. (recommended)
2. A session name may begin with `-`: the picker renames it successfully, and every Portal command that names a session composes an argv that survives it.

## Task 5: The `on-resume` event is an untyped literal the store now reaches for internally

severity: medium
sources: architecture

**Problem**: `Store.Set(key, event, command, via)` and `Store.Remove(key, event, via)` model the event dimension as a caller-supplied plain `string`, and the only value passed anywhere is the literal `"on-resume"` — spelled at eight sites (`internal/hooks/store.go:113,163,354`, `internal/hooks/lookup.go:25`, `cmd/hooks.go:219,293`, `internal/hookstest/staging.go:130`, `internal/hookstest/hooks.go:79`), two of them inside `internal/hooks` itself. The worst is new: `deleteStale`'s audit line sources its `value` attr with `h[key]["on-resume"]`, so the store hardcodes one event to read a map whose whole shape exists to be event-generic. An entry persisted under any other event would be deleted and logged with an **empty** `value` — the exact "what did I lose?" breadcrumb this work unit added to make a reaped hook recoverable, silently wrong. The inconsistency is sharpest against `Via`, which this same work unit gave a closed integer enum for a strictly weaker reason: `Via` is a log attr, while the event string is the second-level key into persisted state, where an invented literal writes an entry nothing will ever fire.

**Solution**: Give the event the same treatment `Via` got — a `hooks.Event` type (or at minimum an exported `hooks.EventOnResume` constant) taken by `Set`, `Remove` and `LookupOnResume`, so the vocabulary is closed at the boundary and spelled once. Separately, stop `deleteStale` reaching into the map for one event name: the pre-delete map is already in hand, so the removal record carries the removed entry's events and the audit line reports what was actually removed rather than what the store assumed was there.

## Task 6: One sweep cycle's log lines are split across three components by an injected logger

severity: medium
sources: architecture, bank

**Problem**: `runHookStaleCleanup` emits one cycle's output through two loggers by design: every stand-down goes out under the `hooks` component via `standDown.emit()` (`cmd/run_hook_stale_cleanup.go:141`), while the cycle's two DEBUG counts go to whatever `countsLogger` names — `daemon` from `maybeRunHookCleanup` (`cmd/state_daemon.go:211`), `bootstrap` from `pruneDoctorStaleHooks` (`cmd/doctor.go:200`). A third slice escapes both: an unclassified sweep error is worded by each caller separately (`daemon: hooks stale-cleanup failed`, `bootstrap: doctor --fix: stale-hook prune failed`). The function's own doc comment names the consequence — "a caller observing this cycle through countsLogger alone sees the counts and none of the stand-downs" — which is precisely the property this work unit exists to remove: the original loss was silent because reconstructing one cycle meant correlating across sources. An operator now needs three greps to reconstruct one 10s sweep, and the closed `clean-stale-skipped` vocabulary built to make declines identifiable covers only the subset routed through `emit()`. The `bootstrap` attribution is doubly odd, since the same phase ruled that a bootstrap step is forbidden from calling the sweep at all.

**Solution**: Drop the `countsLogger` seam and emit the whole cycle — counts, stand-downs and failures alike — under `hooks`, the component that owns the subsystem being swept. The two callers keep whatever cycle-summary line their own component owes and stop wording the sweep's internals in their own vocabulary.

**Outcome**: `grep 'hooks:'` reconstructs one sweep cycle whole, which is what the closed reason vocabulary was built for.

## Task 7: `pane_key` names two different values inside one daemon loop body

severity: medium
sources: bank

**Problem**: In the same iteration of the capture loop, `cmd/state_daemon.go:276` and `:295` log `pane_key` as the sanitized scrollback key (`work__0.0`), while `:289` logs `pane_key` as a tmux coordinate (`work:0.0`), recomposed through `tmux.PaneTarget` purely for that line. One attr key in the closed vocabulary, two value shapes, on adjacent lines of one loop — so an operator correlating a failed capture against the scrollback file it should have written has to know which line means which. The split is pinned by an assertion at `cmd/state_daemon_capture_logging_test.go:249`, so it is currently load-bearing.

**Solution**: Settle on one meaning for `pane_key` — the sanitized key, since that is what every other emission and every on-disk artifact uses — and give the failed-capture WARN its coordinate under a differently-named attr, or drop the coordinate where the key already identifies the pane. The closed attr vocabulary gains at most one name, spec-governed like its siblings.

## Task 8: The sweep reads `hooks.json` twice and only one read's failure has a reason

severity: low
sources: architecture

**Problem**: `CleanStale` classifies its advisory pre-read failure as `ErrSnapshotRead` (`internal/hooks/store.go:305`), which `declinedSweep` maps to `store-read-failed`. But `deleteStale` performs a second `s.load()` of the same file under its exclusive hold (`internal/hooks/store.go:330`), and that failure is wrapped as a bare `fmt.Errorf("failed to load hooks: %w", …)` with no sentinel — so it falls through `declinedSweep`'s switch to the unclassified default and leaves the closed reason vocabulary entirely, reported instead by whatever words the caller happens to hold. The same file, failing to read for the same reason, is identifiable in one phase and anonymous in the other, which undercuts the premise that every decline is identifiable rather than only the guard cases.

**Solution**: Wrap `deleteStale`'s load failure in the same `ErrSnapshotRead` sentinel (or a sibling `ErrStoreRead` both map through) so both reads of `hooks.json` classify to `store-read-failed`, leaving only genuine save failures on the unclassified path.

## Task 9: The stand-down reason vocabulary is guarded three ways and carries members no surface reaches

severity: low
sources: architecture

**Problem**: Seven `skipReason` constants are backed by two exhaustive phrase maps, an enumerable `skipReasons` slice, an AST guard proving the slice matches the const block, a coverage guard proving both maps are exhaustive and carry no extras, a copy-uniqueness guard with a `notStandDownReasons` subtraction list, **and** a runtime fallback in `phraseFor` returning the raw reason for an unmapped one. The exhaustiveness guard makes the fallback unreachable; the fallback makes the guard optional. The vocabulary is also not internally consistent: `notEvaluableDetails[skipReasonLockTimeout]` is documented in-source as unreachable and present "for vocabulary completeness", and `skipReasonSweepFailed` is a member of a type whose own doc calls it "the closed vocabulary of reasons a cycle removed nothing", whose log op is `clean-stale-skipped`, yet it is never a skip and never logged — which is why the copy guard needs a subtraction list to compensate for a member the type's contract excludes.

**Solution**: Pick one enforcement — keep the exhaustiveness guard and delete `phraseFor`'s fallback branch, so an unmapped reason fails a test rather than printing a raw token. Separately, move `skipReasonSweepFailed` out of `skipReason` (it is a failure the `--fix` renderer reports, not a stand-down) so the copy guard no longer needs its subtraction list.

## Task 10: Stand-down copy is written outside its declared home on three surfaces

severity: low
sources: bank

**Problem**: The phrase tables in `cmd/run_hook_stale_cleanup.go` are meant to be the one place a stand-down's words live, but three surfaces still spell them. (a) The lock phrase is an inline literal at `:86` and `:98` where every other shared phrase composes from a const, which is why `cmd/doctor_stand_down_copy_test.go`'s lock row carries an empty `sharedPhrase` and is skipped by the shared-const subtest. (b) `checkStaleProjects` writes the bare literal `could not read projects.json` at `cmd/doctor.go:339` and `:343` for its two branches, with no reason vocabulary at all on the projects surface and no guard that can see it. (c) `cmd/doctor_test.go` literal-pins the rendered copy at `:968-969`, `:1022-1023`, `:1144`, `:1166-1167`, `:1178-1179` and `:1236-1237`, each a second home for words `cmd/doctor_stand_down_copy_test.go:60-126` owns — so re-wording one phrase is several edits found only by grep.

**Solution**: Give the lock phrase a const beside its siblings and fill in the copy test's `sharedPhrase` for that row; render the `doctor_test` assertions through `phraseFor(notEvaluableDetails, <reason>)` the way `doctor_fix_hook_prune_report_test.go:38` already does, keeping each case's distinct subject; and either give the projects reaper a reason vocabulary of its own or route it through the hooks one, so its copy is reachable by the phrase guards.

## Task 11: `internal/hooks`'s read API is inconsistent across its three read entry points

severity: low
sources: architecture

**Problem**: The package offers three ways to read, each shaped differently. `Load(via)` and `List(via)` are methods taking the caller's `Via`. `LookupOnResume(store, hookKey)` is a package-level function taking `*Store` as its first parameter and hardcoding `ViaHydrate` internally — so the one read whose caller is fixed is the one that does not take the parameter, and it is a free function where its siblings are methods despite reaching an unexported method on the receiver it is handed. `StaleKeys(persisted, live)` is an exported function whose entire body is `return staleKeys(persisted, live)` with an identical signature — an exported/unexported twin where one export would do. The shape of a call carries no information about what it does.

**Solution**: Make `LookupOnResume` a method on `*Store` taking a `Via` like its siblings, with `ViaHydrate` supplied by the one caller that means it, and collapse the `StaleKeys`/`staleKeys` pair to the single exported function both `deleteStale` and `checkStaleHooks` call.

## Task 12: The hook-staleness cycle is a self-contained domain living in `cmd`

severity: low
sources: architecture

**Problem**: `cmd/run_hook_stale_cleanup.go` grew from ~60 lines to 345 and now holds five types (`skipReason`, `standDown`, `stalenessView`, `sweepOutcome`, `declinedError`), a closed reason vocabulary, two user-facing copy tables and the emission policy for a subsystem — none of which is cobra-shaped. Its copy tables sit in this file while both renderers that consume them (`reportSkippedPrune`, `staleHooksNotEvaluable`) live in `doctor.go`, and its component binding is borrowed from `cmd`'s `hooksLogger`. It is reached by exactly two callers, both passing a seam interface rather than a command. The pieces compose, but the policy has outgrown the package it sits in, and that home is why the component question in Task 6 was reachable at all.

**Solution**: Lift the cycle into its own package — the reason vocabulary, `standDown`, `stalenessView`, `sweepOutcome` and `runHookStaleCleanup` — with its own `hooks` component binding, leaving `cmd` holding the two call sites and the rendering of an outcome. The component question then answers itself, and the copy tables sit beside the renderers that print them.

## Task 13: Two production comment blocks carry the design argument and cardinality claims the project's comment standard forbids

severity: comments
sources: standards

**Problem**: `code-quality.md` names two things a comment must never carry — the design argument ("State the conclusion the code needs, not the debate; the reasoning lives in the project's design artifacts") and cardinality claims ("the single caller", "the only site that…", "nothing consumes this yet"). `snapshotLockBound` (`internal/hooks/lock.go:29-61`) is a three-line function under a ~33-line comment reproducing the whole of the specification's corrigendum argument — the two-bounds justification, the headroom-versus-thousandth comparison, the behaviour across three bound ranges, and a floor rationale stated twice in the same block. The `skipReasons` block (`cmd/run_hook_stale_cleanup.go:32-52`) asserts cardinality outright — "sweep-failed is the --fix repair's line alone", "lock-timeout cannot reach the read-only diagnosis at all" — both falsifiable by ordinary additive change far from the comment. `cmd/hooks.go:81-83`'s "A new seam costs one fill line here rather than a builder of its own" is the same class, smaller.

**Solution**: Reduce `snapshotLockBound`'s comment to the conclusions the code needs — the pre-read is advisory and may degrade, the bound is derived from `lockTimeout` so lowering one lowers both, and the floor of one poll interval is load-bearing because the acquire re-tests its deadline only after a sleep — and let the specification hold the argument. Strip the two cardinality claims from `skipReasons`, keeping a reachability fact only where it changes what a reader must do. Drop the `hookSeams` aside.

## Task 14: `(*Client).SendKeys` is exported on the production client with no production caller

severity: medium
sources: bank

**Problem**: `internal/tmux/tmux.go:658-663` exports `SendKeys` solely for test consumption and says so in its doc comment — "It has no production caller and is exported anyway" — which is exactly the cardinality claim `code-quality.md` forbids, and a comment that must be deleted in the same edit as the first production caller that ever appears. The driver is consumed from `cmd/bootstrap/eager_signal_hydrate_integration_test.go:154,237`, `internal/restore/exit_closes_pane_integration_test.go:36,59` and `internal/tmux/tmux_test.go`. `internal/tmuxtest` already owns the per-test `-L`/`-S` socket and hands out `ts.Client()`, so it can compose the same `send-keys -t <target> <cmd> Enter` against its own socket with no access to the client's unexported fields.

**Solution**: Relocate the driver to `internal/tmuxtest` as a `Socket` method and drop `SendKeys` from the production client, retiring both the export with no production caller and the comment that has to explain it.

## Task 15: `session.BuildShellCommand` leaves the leading shell word unquoted

severity: medium
sources: bank

**Problem**: `internal/session/create.go:31` renders `fmt.Sprintf("%s -ic %s", shell, shellquote.Single(script))` — the script is quoted through the shared leaf, but the first `%s` is interpolated raw. A `$SHELL` of `/My Apps/zsh` composes `/My Apps/zsh -ic '…'`, which tmux hands to a shell that word-splits it into `/My`, and the session's first command dies on a path that does not exist. `internal/shellquote` exists precisely to declare this rule once and the fix is a one-token change, but it is a behaviour change (the composed string differs for every shell path, quoted or not), so the task that extracted the leaf correctly refused to make it under a byte-identity criterion.

**Solution**: Route the leading shell word through `shellquote.Single` alongside the script, so the whole composition is well-formed for any `$SHELL` path.

## Task 16: `cmd/uninstall.go` substring-matches tmux stderr to detect an absent session

severity: medium
sources: bank

**Problem**: `isSessionAbsentError` (`cmd/uninstall.go:109-111`) does `strings.Contains(strings.ToLower(err.Error()), "can't find session")` on a `KillSession` error. `internal/tmux/errors.go` states the rule this breaks: layers above `internal/tmux` must not substring-match tmux stderr, because tmux's phrasing is not a stable contract. `KillSession` now carries `ErrNoSuchSession`, so the check collapses to `errors.Is`. It also has a live wrong answer today: an unaddressable saver name produces the same "can't find session" stderr as a vanished one, so the substring match reports a **live** session as absent and `portal uninstall` logs a clean removal for a saver it did not kill.

**Solution**: Replace the substring match with `errors.Is(err, tmux.ErrNoSuchSession)`, which the wrapping already provides and which the unaddressable-name classification in `wrapSessionTargetErr` deliberately keeps distinct.

## Task 17: The four exact-target forms return a bare string, so only a name-based source scan tells a pinned target from a hand-composed one

severity: medium
sources: bank

**Problem**: `SessionTargetExact`, `CoordTargetExact`, `PaneTargetExact` and `windowTargetExact` all return a plain `string`, so a hand-composed target is assignable wherever a pinned one is, and the whole rule rests on `internal/tmux/target_composition_guard_test.go` — a source scan matching parameter names and callee names. `SessionTargetExact`'s own doc comment records the settled answer in-source: "A named `type Target string` returned by these four and taken by every `-t` parameter is the settled answer: it moves the rule into the type system, where a laundered or reassigned target cannot pass. Landing it is a signature change across the client's whole surface and its callers in cmd, internal/state and internal/restore — deferred for that reach, not because the string form is preferred." A sibling task recorded the same type-over-convention move as pending. The guard's own history shows the cost of leaving it: it has already been widened twice this phase for shapes it could not see.

**Solution**: Land the deferred `type Target string`: the four constructors return it, every `-t`-taking client method and every cross-package function holding an already-composed target takes it, and the source guard reduces to whatever the type cannot express. The reach is the client's whole surface plus its callers in `cmd`, `internal/state` and `internal/restore`, which is why it is a task of its own rather than a rider on a guard change.

## Task 18: The repo-wide source-scan preamble is re-authored at every guard

severity: duplication
sources: duplication, bank

**Problem**: The same ~28-line opening — `portalbintest.ProjectRoot()` with a "resolve project root" fatal, `sourceguardtest.GoSourceFiles(root)` with an "enumerate .go files" fatal, a `_test.go` suffix filter, `sourceguardtest.ParseSources`, and a per-file relativisation so findings read as repo paths — stands at `internal/portalbintest/lane_guard_test.go:55-88`, `internal/portaltest/teardown_guard_coverage_test.go:161-193`, `internal/restoretest/literal_guard_scan_test.go:29-58`, `internal/logtest/install_guard_test.go:57-72`, `internal/theme/loader_construction_guard_test.go:18-38` and `internal/prefs/appearance_api_guard_test.go:28-46`, the first two near line-for-line identical. The relativisation has already drifted three ways: two `relTo*` helpers are declared privately (`internal/logtest/install_guard_test.go:170`, `internal/theme/slug_collapse_guard_test.go:52`), `filepath.Rel` is inlined at seven more sites (`internal/portalbintest/lane_guard_test.go:74`, `internal/prefs/appearance_api_guard_test.go:42`, `internal/log/discard_guard_test.go:38`, `internal/log/migration_guard_test.go:41`, `internal/portaltest/teardown_guard_coverage_test.go:180`, `internal/tui/restore_source_guard_test.go:103,182`, `internal/tui/theme_source_guard_test.go:28`), and one guard uses `strings.TrimPrefix(finding, root+separator)` instead — two behaviours for one rule. Beneath the repo-wide form sits the same skeleton narrowed to a package or a file: `internal/theme` declares three identical six-line `find-the-package-source-named-X` helpers (`badge_test.go:238`, `resolution_test.go:351`, `setting_test.go:601`), and roughly a dozen further guards re-author an enumerate→parse loop under their own parse mode. `internal/sourceguardtest` is the declared home for exactly these primitives and was extended twice by this work unit without absorbing the driver every consumer writes on top of them.

**Solution**: Promote the driver into `internal/sourceguardtest` as one entry point — a `RepoSources(t, …)` returning the resolved root alongside `[]ParsedSource` already narrowed to test or non-test files with root-relative `Path` set — and re-point the guards onto it, retiring the two private `relTo*` helpers, the inline `filepath.Rel` sites, the `TrimPrefix` variant and the three `internal/theme` twins. The scanned-nothing tripwire then has one home rather than one per guard, and the parse mode stops varying by site.

## Task 19: The build-tag question has three readers, and every leaf guard but one is blind to it

severity: medium
sources: duplication, bank

**Problem**: Two halves of one gap. First, build-constraint extraction from an `*ast.File` is implemented three times in three packages — `compiledInUnitLane` (`internal/portalbintest/lane_guard_test.go:112-128`), `isIntegrationTagged` (`internal/restoretest/session_restorer_literal_guard_test.go:174-188`) and `buildConstraintLine` (`internal/sourceguardtest/leaf_guard_test.go:45-58`) each walk `file.Comments`, break at `group.Pos() > file.Package` and run `constraint.Parse`, all three then evaluate or report the `integration` tag, and all three disagree on the details: one treats an unparseable constraint as in-lane, one skips a parse failure and keeps looking, one returns the first parseable comment whatever it says. Three readings of one file can classify it three ways, and the tag literal is declared separately in two of them. Second, `AssertDepsWithin` resolves through `go list -deps` under the default tags (`internal/sourceguardtest/packagedeps.go:55-63`), so a build-tagged file can hide a forbidden dependency from **any** leaf guard — proven silent for `sourceguardtest`, and nothing about the mechanism is package-specific. Only `internal/sourceguardtest` has the untagged companion check; `internal/nanoid`, `internal/shellquote`, `internal/harnesstest`, `internal/xdg`, `internal/prefs`, `internal/hooks` and `internal/theme` leaf guards are all blind the same way.

**Solution**: Add one build-constraint primitive to `internal/sourceguardtest` — a reader returning the file's parsed constraint plus a tag evaluator, with the `integration` tag name declared once beside it — and route the three copies through it, each keeping only its own policy. Give `AssertDepsWithin` the tag-awareness option the untagged companion check hand-rolls, and apply it across the seven leaf guards so a tagged file cannot hide a dependency from any of them.

## Task 20: The two restore-`Exe` literal guards are a copy-paste pair that each walk the repo

severity: duplication
sources: duplication, bank

**Problem**: `orchestratorLiteralsIn` (`internal/restoretest/orchestrator_literal_guard_test.go:151`) and `sessionRestorerLiteralsIn` (`internal/restoretest/session_restorer_literal_guard_test.go:190`) are identical bodies differing only in the type-name constant they pass to `isRestorePkgType`. `scanTestOrchestratorLiterals` and `scanIntegrationSessionRestorerLiterals` are identical apart from the include filter and the wording of the scanned-nothing fatal. `TestOrchestratorLiteralGuard_FatalsWhenItEnumeratesNoTestFiles` and `TestSessionRestorerLiteralGuard_FatalsWhenItEnumeratesNoIntegrationTestFiles` are the same ~25-line test twice. The session-restorer half was written a task later than the orchestrator half against the shared `scanGuardTestFiles` seam and reproduced everything above it rather than parameterising it — and because each calls `scanGuardTestFiles` separately, the repo-wide walk and parse run twice per unit-lane run of the package.

**Solution**: Collapse the pair into one parameterised guard — a `literalGuard{typeName, include, constructors, fatalWording}` descriptor driving a single scan that owns the scanned-zero fatal and a single composite-literal collector, with the two guards and the two fatals-when-empty tests table-driven over the two descriptors, and one walk feeding both.

## Task 21: The restore-`Exe` rationale paragraph is restated six times

severity: comments
sources: duplication

**Problem**: The same explanation — `Exe` falls back to `os.Executable()`, which under `go test` is the test binary, so an armed pane re-runs the suite inside itself, exits 0 and takes the session with it — is written out in full at `internal/restoretest/orchestrator.go:11-19`, `orchestrator_staged.go:14-22`, `session_restorer_staged.go:11-20`, `orchestrator_literal_guard_test.go:22-31`, `session_restorer_literal_guard_test.go:21-32` and `restoretest.go:55-58`, plus twice more in the two guards' `t.Fatalf` messages. It is one fact about one field, and each restatement is an independent claim that has to be kept true; the wordings have already diverged ("re-runs the suite inside itself" / "re-runs its own suite inside the pane" / "respawns into the suite itself").

**Solution**: State the trap once, on `StagedHydrateExe` — the helper every pinned route ends at — and have the four constructor and guard doc comments say only what their own site does, keeping the guards' user-facing `t.Fatalf` text as the one other place the consequence is spelled out.

## Task 22: The Cobra root-command drive is written four ways in the `cmd` test suite

severity: duplication
sources: duplication, bank

**Problem**: `runRootCmd` (`cmd/root_test.go:92-101`) is the package's shared driver, introduced this phase as "one root-command driver". `runHookSet`, `runHookRm` and `runHookList` (`cmd/testhelpers_test.go:176,189,202`) each repeat its body verbatim — new buffer, `resetRootCmd()`, `SetOut`, `SetErr`, `SetArgs`, `Execute` — differing only in the argv they build and in routing both streams to one buffer. `cmd/hooks_test.go` inlines the same four-line sequence 20 times while calling the purpose-built drivers 13 times in the same file, and the shape recurs package-wide: 192 `rootCmd.Execute()` calls across 30 files, nine of which are literally `runRootCmd(t, args…)` after un-substitution (`cmd/bootstrap_orchestrator_test.go:164-166`, `cmd/root_test.go:498-500`, `cmd/version_guard_test.go:153-155`, `cmd/state_test.go:155,171,201`, `cmd/state_hydrate_empty_hookkey_test.go:26-30`, `cmd/state_signal_hydrate_test.go:374,404`). Every sibling hook suite added by this work unit routes exclusively through the drivers, which makes `hooks_test.go` the drifted copy rather than the convention.

**Solution**: Reimplement `runHookSet`/`runHookRm`/`runHookList` as thin argv-composing wrappers over `runRootCmd` — adding a combined-stream variant there if the single-buffer return is load-bearing — and convert the inline sequences that are exact matches onto the driver, starting with the 20 in `cmd/hooks_test.go` and the nine identified above, so the package has one route to running a command.

## Task 23: `hookstest.AssertDegradedRead` restates the shared record assertion its file-neighbour delegates to

severity: duplication
sources: duplication

**Problem**: `AssertLockWarn` (`internal/hookstest/hooks_lock.go:107-131`) routes its level, message, component, op and via checks through `logtest.AssertRecord` and adds only the attrs specific to the lock WARN. `AssertDegradedRead`, ten lines below it in the same file, hand-rolls the level, op and via comparisons instead — the same three checks `AssertRecord` owns, with its own `t.Errorf` wordings — and consequently never asserts the message or the component at all. `cmd/logging_capture_test.go:51`'s `assertHooksRecord` shows the intended shape, so this is one site out of step rather than an accepted second route.

**Solution**: Rewrite `AssertDegradedRead` to call `logtest.AssertRecord` with the degraded read's `RecordWant`, keeping only the non-empty `error` attr check locally, so the `load-unlocked` breadcrumb is pinned by the same assertion as every other audit-trail line.

## Task 24: `hooks.json` and sidecar paths are still hand-composed at ~60 sites now that `hookstest` owns them

severity: duplication
sources: bank

**Problem**: `hookstest.HooksPath` and `hookstest.SidecarPath` exist, but the inline composition survives everywhere the task that introduced them did not name: 49 `filepath.Join(dir, "hooks.json")` sites remain, across `internal/hooks/lock_test.go` (13), `lookup_test.go` (3), `read_lock_test.go` (2), `cmd/hooks_read_lock_test.go:56`, `cmd/state_daemon_test.go:803`, `cmd/config_seeder_parity_test.go:106` and five stat-assertions in `cmd/state_hydrate_test.go`. The `.lock` suffix is hand-appended at 11 more (`internal/hooks/lock_test.go` ×8, `read_lock_test.go` ×2, `cmd/run_hook_stale_cleanup_snapshot_order_test.go:116`). Two further gaps sit beside them: `cmd/testhelpers_test.go:200`'s `readHooksJSON` is a read-and-unmarshal counterpart to the byte read `hookstest` already owns the encode half of, and `cmd/doctor_stand_down_copy_test.go:256-290` adds a second "hooks.json unchanged" vocabulary (`hooksPathState`/`assertHooksPathUnchanged`) beside `assertHooksFileUnchanged`, existing only because `hookstest.HooksFileBytes` fatals on EISDIR and the `Unreadable` staging axis puts a directory at the path.

**Solution**: Re-point the remaining path compositions onto `HooksPath`/`SidecarPath`; add the decode counterpart (`HooksFileEntries`) beside the encode half so `readHooksJSON` and the `internal/hooks` suites that decode the same shape share one home; and teach `hookstest` to describe a directory standing at the hooks.json path so the local `hooksPathState` pair retires into the shared one.

## Task 25: Six `internal/hooks` cases drifted off the degraded unlocked read, leaving an in-file branch describing no remaining case

severity: duplication
sources: bank

**Problem**: Six `internal/hooks/store_test.go` cases (two in `TestLoad`, the malformed/empty-event-map pair in `TestRemove`, the sorted `TestList` case, and three in `TestCleanStaleLogging`) seeded straight into the file and so took the degraded unlocked read; routing them through the shared stager gave them a sidecar and the shared lock. No assertion in any of them is about the read mode and the degraded-read contract is owned elsewhere, so no coverage was lost — but the `load-unlocked` skip branch in `partitionCleanStaleRecords` (`internal/hooks/store_test.go:871`) is now unexercised within that file and its comment describes no case in it. The unlocked read is the shape most installs are actually in (no install carries a sidecar until its first mutation), so leaving these fixtures on the locked path also moves them off the common case.

**Solution**: Pass `SidecarAbsent: true` at those six sites, restoring both the degraded read they exercised and the branch whose comment currently describes nothing.

## Task 26: Token-shaped key literals and one-entry stale seeds are hand-authored outside the `hookstest` vocabulary

severity: duplication
sources: bank

**Problem**: The named seed vocabulary exists so a pane-token width move carries every fixture with it, but hand-rolled token-shaped literals survive in four packages: `tok123`/`tok999` throughout `internal/hooks/lock_write_test.go`, and the `aaa111`/`tok123` family in `internal/restore/session_test.go`, `internal/restore/rename_reboot_shared_test.go`, `internal/tmux/hookkey_format_realtmux_test.go` and `internal/tmux/resolve_hookkey_realtmux_test.go` — each carrying the same reclassification risk the vocabulary was built to remove. Separately, the identical single-entry stale-seed body is re-authored inline six times (`cmd/state_daemon_hook_cleanup_test.go:37,95,149`, `cmd/state_daemon_run_test.go:598,631,662`) where `StaleHookSeed` already owns the two-entry body next door. And the vocabulary's own completeness check (`internal/hookstest/hooks_test.go:15-35`) duplicates the seed names into two hand-maintained maps claiming to name every seed the vocabulary mints — an eighth seed added without touching them is unverified and the claim false, which is the silent-shrink failure mode relocated one level up.

**Solution**: Route the hand-rolled literals through the named seeds — noting that `internal/tmux` would gain a `hookstest` edge it has no other reason for, so that package's sites want a ruling rather than a rename — add the one-entry seed beside `StaleHookSeed` so the six inline copies become a name, and derive the self-test's completeness from a source scan of the `Seed[A-Z]` declarations rather than from a duplicated list.

## Task 27: `logtest`'s query surface is missing the dimensions its consumers hand-roll

severity: duplication
sources: bank

**Problem**: The exported filters express `(component, message)`, level and message, and the whole capture — and consumers write the rest by hand. Twelve sites walk `sink.Records()` with an inline `r.Level == … && r.Msg == …` predicate the surface now expresses directly (`cmd/bootstrap/clean_sweep_summary_test.go:57,60`, `internal/hooks/store_test.go:1035`, `internal/state/fifo_sweep_summary_test.go:192`, `internal/hooks/store_shape_test.go:97`, `cmd/bootstrap/latch_test.go:122`, and others). Four sites re-author a partition loop splitting records into two sets by level plus a `strings.HasPrefix` on the message (`cmd/open_burst_run_test.go:689,753,830`, `internal/project/store_logging_test.go:370`, `internal/hooks/store_test.go:897`) — the absent prefix query is the root of those. Roughly six sites hand-write a non-fatal `if got := len(<query>); got != 1` because `Records.Only` is fatal by construction (`cmd/run_hook_stale_cleanup_test.go:39,67,185,189`, `cmd/run_hook_stale_cleanup_single_report_test.go:127`, `cmd/doctor_fix_hook_prune_report_test.go:45,70`, `cmd/hooks_test.go:974`, `cmd/theme_persister_test.go:200`). And `cmd/theme_source_test.go:219` spells out `rec.Msg == msg && rec.HasAttr("component") && rec.AttrString(…) == "theme"`, a second declaration of the predicate `Record.Matches` documents itself as the single home of.

**Solution**: Close the surface's gaps rather than the call sites' — a message-prefix filter, and a non-fatal cardinality terminal beside the fatal `Only` — then re-point the sites each one absorbs, and route `cmd/theme_source_test.go:219` through `Matching`. Sites that are genuinely inexpressible stay as they are and are named where they sit: the attr-presence classification in `internal/tui/burst_observability_test.go:314` and the shared-sequence appends in `cmd/bootstrap/latch_test.go`.

## Task 28: A component-alone query exists in the tree, which is the stated trip-wire for splitting `Matching`

severity: duplication
sources: bank

**Problem**: `Matching(component, msg)` was kept as one filter on the premise that the pair jointly names one event and nothing queries the component alone. `cmd/open_theme_construction_test.go:521`'s `themeEvents` walks `sink.Records()` filtering on `HasAttr("component") && AttrString(t, "component") == "theme"` with no message constraint — the one shape the exported surface cannot express, so the premise is already false. The surface's own rule is that no combination of filters may itself be a method, because that is what holds the property a caller never has two routes to one set.

**Solution**: Add `WithComponent` and delete `Matching` in the same change — with `WithComponent` present, `Matching` is a composition and therefore a second route — re-pointing every `Matching` call site onto the chained pair, and keep `Record.Matches` for the capture-order walkers that need the predicate rather than a filtered slice.

## Task 29: The failed-write tail is hand-rolled in the `cmd` migrate suite in a weaker form than the shared helper enforces

severity: duplication
sources: bank

**Problem**: `cmd/config_migrate_logging_test.go:168` and `:208` already use `logtest.AssertRecord` for the five shared properties and then spell the tail inline — an `error_class` string check followed by `rec.HasAttr("error")`. `HasAttr` is strictly weaker than `AssertWriteFailure`: it never checks that the carried error wraps the named `fileutil` write-phase sentinel (`ErrWriteRename` at `:168`, `ErrWriteTempCreate` at `:208`), so a misclassified error passes. The helper does not fit as-is, because `cmd/config.go`'s migrate WARNs log the raw OS error with a hardcoded `error_class` and wrap no sentinel. Note the site that looks like a third is not one: `internal/storelog/clean_stale_test.go:59-66` asserts `loggedErr != saveErr` — an identity check pinning that the emitter passes the caller's error through un-rewrapped, a stronger and different property `AssertWriteFailure`'s `errors.Is` would weaken. It must not be folded in by pattern-match.

**Solution**: Wrap the migrate path's write failures in the `fileutil` phase sentinels they already classify themselves as, then route both assertions through `AssertWriteFailure` — removing the last two copies and strengthening them in the same change. Leave `internal/storelog/clean_stale_test.go` alone.

## Task 30: `internal/logtest` states a dependency-shape invariant that no guard enforces

severity: medium
sources: bank

**Problem**: `internal/logtest/assert.go:44-47` documents that `logtest` takes no dependency on the package declaring the write-phase sentinels — which is why `AssertWriteFailure` takes the sentinel as a parameter rather than resolving it — and CLAUDE.md's `logtest` row repeats the claim. Unlike the sibling test-support leaf `internal/harnesstest`, which pins its dependency set through `sourceguardtest.AssertDepsWithin` (`internal/harnesstest/leaf_guard_test.go`), `internal/logtest` has no deps guard at all, so the first `internal/fileutil` import would compile silently and falsify both statements. `logtest` is reachable from every test package in the tree, which is what makes the property load-bearing.

**Solution**: Add the leaf guard `internal/harnesstest` already carries, declaring `logtest`'s allowed dependency set so the documented shape is enforced rather than asserted in prose.

## Task 31: Three `ConfigFileID` filenames are asserted by nothing

severity: medium
sources: bank

**Problem**: `xdg.AliasesFile.Filename`, `xdg.ProjectsFile.Filename` and `xdg.TerminalsFile.Filename` are pinned by no test. Their env vars are covered behaviourally (`cmd/alias_test.go`, `cmd/spawn_seams_test.go`, `cmd/version_guard_test.go` set them and assert the resolved store is read), but `cmd`'s `TestMain` poisons those variables package-wide, so no test ever exercises those three files at the default config base. A typo in any of them would ship silently and point a user's aliases or projects at a file nothing writes — the file would simply appear empty. `cmd/config_identity_test.go:49` already establishes the pattern for `HooksFile`, pinning env var, filename and log component together.

**Solution**: Extend that subtest into a table over all five `ConfigFileID`s and both `ConfigDirID`s, so every identity's env var, filename and log component are pinned by one assertion.

## Task 32: `XDG_CONFIG_HOME` and the state-directory rule are each named in two places

severity: duplication
sources: bank

**Problem**: `internal/xdg` is now the shared home of every config file and directory identity, but two rules it owns are restated outside it. `internal/xdg/xdg.go:25` hardcodes the `XDG_CONFIG_HOME` literal inside `ConfigBaseFrom` while `internal/hookstest/hooks.go:37` declares its own `const xdgConfigHome = "XDG_CONFIG_HOME"` for the isolation-regression check — so the seeder that must resolve by the same rule as the binary under test names the variable independently. Separately, `internal/portaltest` composes `<base>/portal/state` by hand at `isolated_env.go:68` and `fingerprint.go:288,291`, and spells `PORTAL_STATE_DIR` literally at `isolated_env.go:76,93-94` and `spawn_daemon.go:32`, all now duplicating `xdg.StateDir`/`xdg.ConfigDirPath`.

**Solution**: Export the environment variable name from `internal/xdg` and have `hookstest` read it; re-point `portaltest`'s state-dir composition onto `xdg.StateDir`, **except** the `fingerprint.go` copy, which is a deliberate independent re-derivation used as a backstop and must stay independent — the point of a backstop being that it does not share a bug with what it checks.

## Task 33: The test wait budgets have no single home, so four shapes coexist and three sites inherit an unrelated ceiling

severity: duplication
sources: bank

**Problem**: The progress-based wait landed for fifteen daemon-lifecycle sites, and everything around it stayed fixed wall-clock. `tmuxtest.Socket.WaitForSession` (`internal/tmuxtest/socket.go:114-127`) polls `has-session` against a bare timeout and is called with an undeclared `2 * time.Second` at ~34 sites across `internal/restore` and `cmd/bootstrap`; deleting `singletonRecycleTimeout` also left `internal/tmux/portal_saver_integration_test.go:54,67,108` passing a 45s `ProgressWait.Ceiling` where 5s stood, coupling a session-appearance budget to a ceiling chosen for a different observable and stretching a genuinely-red run ninefold. `cmd/bootstrap/eager_signal_hydrate_integration_test.go:256` declares its own deadline/tick/sleep loop over `CapturePane`, the only budget consumer in the family that is not a shared helper. `cmd/bootstrap/reboot_roundtrip_test.go:492` hand-writes `10*time.Second` for the marker-clear observable while `:506` in the same function reads the declared `HydrateBudget`/`HydrateTick` pair. `internal/restoretest/restoretest.go:100-103` declares a second local 10s/50ms pair inside `DriveSignalHydrate` for a different observable, in the file next door to the declaration. And `AssertMarkerCount` hardcodes `HydrateBudget` (`internal/restoretest/marker_count.go:49`), so a structurally identical post-marker-clear hook wait runs on the long budget in `cmd/bootstrap` while its twin in `internal/restore` runs on the short one. Two further fixed-budget `PollUntil` consumers survive outside the converted set (`internal/state/capture_colon_session_realtmux_test.go:79`, `internal/portaltest/tmux_server_wait.go:30`).

**Solution**: Give each observable one named budget in the package that owns it — the session-appearance wait belongs with `tmuxtest`, whose subject is the socket — restore a named appearance budget in place of the borrowed ceiling, let `AssertMarkerCount` take a budget rather than hardcoding one, route the pane-text wait through the shared poll helper, and settle the second hydrate pair deliberately (kept with a stated reason, or collapsed) rather than leaving it a near-miss beside its own declaration.

## Task 34: The tmux-server teardown wait sits outside the one place that knows the socket, and its argv convention exists twice

severity: duplication
sources: bank

**Problem**: `internal/portaltest/tmux_server_wait.go:35-37` hand-writes `-S <socketPath> -f /dev/null`, which is exactly `internal/tmuxtest/socket.go`'s unexported `socketArgs` — and a wrong copy fails **silently**, because an unreachable socket reads as a server that is gone. The wait itself is registered per fixture, where `internal/tmuxtest/socket.go:39-42` already registers the one cleanup closure that knows the socket and calls `KillServer`; folding the wait in there would cover every server fixture at zero call-site cost and let `reapTmuxServer` in the coldboot suite collapse to nothing. Beside it, `restoretest.OpenRebootGap` (`internal/restoretest/reboot.go:22-28`) probes once immediately after `KillServer` and fatals if it succeeds, with no wait at all — so every reboot fixture routed through it re-opens the shells-outlive-the-server window that the teardown work closed elsewhere. Note also that the HOME-side quiescence guard (`internal/portaltest/teardown_guard.go:83-93`) returns as soon as two consecutive 50ms snapshots match, so over an empty temp HOME it returns at t=50ms having observed nothing — it can miss the very case it names, unlike the state-dir guard, which gates on the daemon pid dying first.

**Solution**: Move the server-unreachable wait into `tmuxtest`, folded into the cleanup closure `New` already registers, and export the socket-args composition so the probe stops restating it; compose `OpenRebootGap` over the shared wait so it gets both the wait and its assertion from one source; and give the HOME quiescence guard a gate on something observable, the way the state-dir guard has one, or state in its own words that it is belt-and-braces over the env pins.

## Task 35: The isolated-env shell pins are inert on this platform, and the sandbox temp dir leaks on every run

severity: medium
sources: bank

**Problem**: Measured with a sandboxed `-S` server and an interactive `/bin/zsh` pane under a temp HOME: (a) `/etc/zshrc` sets `HISTFILE=${ZDOTDIR:-$HOME}/.zsh_history` unconditionally, overriding `IsolateStateForTest`'s `HISTFILE=os.DevNull` pin (`internal/portaltest/isolated_env.go:41`), so `.zsh_history` lands in the temp HOME on every run — that is the writer the teardown actually races; (b) `.zsh_sessions` never appeared even with `TERM_PROGRAM=Apple_Terminal` set, so `SHELL_SESSIONS_DISABLE=1` (`:42`) neutralises nothing; (c) `ZDOTDIR=homeDir` (`:43`) is a no-op by construction, since `${ZDOTDIR:-$HOME}` already resolves to the temp HOME. The lever that works is `ZDOTDIR` pointed **outside** the framework TempDir tree — with it set, the probe temp HOME came back empty. So three env pins that read as the isolation boundary are decorative, and the quiescence wait is carrying the whole fix. Separately, `internal/portaltest/isolated_env_test.go`'s `TestMain` registers `defer os.RemoveAll(sandbox)` and then ends with `os.Exit(m.Run())`, which skips deferred functions — every run of that package leaves a `portaltest-self-sandbox-*` directory in `$TMPDIR`.

**Solution**: Point `ZDOTDIR` outside the framework temp tree so the shell writes nothing into the directory the framework is about to remove, and retire or re-voice the two pins that neutralise nothing so the helper does not read as protecting more than it does. In the self-test's `TestMain`, capture the run's code, run the cleanup, then exit.

## Task 36: The shared Commander fake's headline safety property is opted out of at ~90% of its call sites

severity: duplication
sources: bank

**Problem**: `commandertest.New(t, …)` reports an unscripted argv instead of answering it — the property the fake's own doc names ("a fake that answers an unscripted argv with `("", nil)` is how a test passes while exercising nothing"). It is used at 40 sites tree-wide. `internal/tmux` and `internal/restore` use `commandertest.Quiet` at 185 sites and `FromFunc` at ~190 more, with zero loud sites in either — faithful to the pre-refactor semantics those packages' own silent catch-all fakes had, so nothing regressed, but those two packages do not have the protection the shared fake exists to give. A fifth such fake also survives un-repointed: `internalMockCommander` (`internal/tmux/option_discriminator_internal_test.go:9-20`) returns `(m.Output, m.Err)` identically from `Run` and `RunRaw`, missed by the original sweep because it lives in the in-package test rather than the external one, and re-pointable at no cost since `commandertest` is structurally typed and stdlib-only.

**Solution**: Re-point `internalMockCommander` onto the shared fake, and convert the `Quiet` sites to the loud default package by package, scripting each site's real argv set — expecting the conversion to surface tests asserting on argv production never issues, which is the finding rather than an obstacle to it. `FromFunc` sites model tmux rather than scripting argv and stay as they are.

## Task 37: `RunRaw` has a single production consumer, so the two-method Commander contract protects one call

severity: low
sources: bank

**Problem**: `internal/tmux/tmux.go:720` (`CapturePane`) is the sole `RunRaw` caller in production; every other client method reads through `Run`. The `Commander` interface's two-method shape, the shared fake's two-method contract, its `Strict` mode (making any `RunRaw` call fatal for a package whose production path reads through `Run` alone), and the trim-versus-verbatim contract exported as `Trim`/`Verbatim` all exist to protect that one call — and the drift between fakes over which method returns what is what motivated the fake's consolidation in the first place.

**Solution**: Weigh expressing the split as one `Run` plus a dedicated verbatim-capture seam, which would retire the interface property, the `Strict` mode and the class of drift, against keeping the general two-method interface. If the interface stays, the reason it stays belongs beside it, since the current shape reads as a general property rather than as one call's requirement.

## Task 38: A fifth callee-name unwrapper survives in `internal/capture`

severity: duplication
sources: bank

**Problem**: `countCalls` (`internal/capture/swap_harness_test.go:224-244`) switches on `call.Fun` with a `*ast.SelectorExpr` arm and an `*ast.Ident` arm that both do `== name; count++`, which is exactly `sourceguardtest.CalleeName(call) == name`. It was missed by the sweep that collapsed the other four, and an independent 42-site sweep has since confirmed it is the only one left: its three call sites pass string literals and never `""`, so the empty-unwrap case cannot change a count, and the package's test set already imports `sourceguardtest`, so there is no new package edge. The same sweep cleared `internal/theme/loader_construction_guard_test.go:64` (it switches on a type expression) and `internal/tui/theme_panel_commit_slot_test.go:448`, and confirmed `internal/portaltest`'s `selectorName`/`localCallName` must **not** be routed through `CalleeName` — they render selectors as `pkg.func` so qualified and unqualified calls stay distinguishable, which that guard depends on. Those three are not candidates and should not be re-flagged.

**Solution**: Collapse `countCalls`'s switch to the shared unwrapper, closing the family tree-wide.

## Task 39: Three further AST primitives are re-authored across the guard family

severity: duplication
sources: bank

**Problem**: Beyond the enumerate-and-parse driver, three shapes recur with no shared home. (a) The const/var-spec walk `forEachValueSpec` (`cmd/doctor_stand_down_phrase_guard_test.go:210`) is re-authored at eight further files (`cmd/seam_guard_test.go`, `cmd/open_theme_nomination_test.go`, `cmd/prefs_translation_persist_test.go`, `cmd/capturetool/main_test.go`, `internal/tui/theme_source_guard_test.go`, `internal/tui/theme_flash_precedence_test.go`, `internal/theme/leaf_guard_test.go`, `internal/theme/theme_test.go`); `sourceguardtest` holds `ForEachFuncCall` but no value-spec sibling. (b) The import-list scan is re-authored at four guards (`internal/hooks/leaf_guard_test.go:31`, `internal/tui/theme_source_guard_test.go:26`, `internal/tui/restore_source_guard_test.go:48`, `internal/log/init_test.go:226`), each walking `file.Imports`, unquoting and matching against its own vocabulary, and the last additionally hand-rolling `parser.ParseFile(…, ImportsOnly)` rather than routing through the shared parse; an `AssertImportsWithin`-shaped sibling to `AssertDepsWithin` would carry the direct-import property the way that one carries the transitive one. (c) The in-memory fixture parse — `token.NewFileSet()` + `parser.ParseFile(…, sourceguardtest.ParseMode)` wrapped into a `ParsedSource` — is repeated at six sites (`internal/logtest/install_guard_test.go:158-166`, `internal/portalbintest/lane_guard_test.go:166-179`, `internal/portaltest/teardown_guard_coverage_rule_test.go:148`, `internal/sourceguardtest/foreachfunccall_test.go:91`, `internal/capture/theme_panel_fixture_test.go:478`, `cmd/state_daemon_lock_pid_ordering_test.go:115`). One further hand-rolled `go list` invocation also sits outside the family (`internal/tmux/target_composition_guard_test.go:60-93`) — a genuinely different query (whole-module listing with directories and immediate imports rather than a transitive closure), but now the only copy of the `go list` seam, error wrapping and empty-result fatal that `sourceguardtest` owns.

**Solution**: Add the three missing primitives to `internal/sourceguardtest` — a value-spec walk beside `ForEachFuncCall`, an import-set assertion beside `AssertDepsWithin`, and a fixture-parse entry point beside `ParseSources` — plus a module-listing primitive beside `PackageDeps`, and re-point the sites each one absorbs.

## Task 40: The `internal/log` discard guard is blind to test files and hand-rolls the shared walk

severity: duplication
sources: bank

**Problem**: `internal/log/discard_guard_test.go:35` returns early for any path ending `_test.go`, so the route-through-`log.OrDiscard`/`log.Discard` rule is production-only — which is how six open-coded `slog.NewTextHandler(io.Discard, nil)` copies survived in test files until they were fixed by hand, with nothing to stop the seventh. The same guard also walks the tree itself (`filepath.WalkDir` at `:24` with its own `.git`/`vendor`/`node_modules` exclusions) rather than through `sourceguardtest.GoSourceFiles`, the primitive introduced this phase for exactly that. Beside it, two capture handlers in `internal/log` remain structural twins of `logtest.Sink` — `recordingHandler` (`log_test.go`) and `componentCapture` (`rotate_test.go`) — kept apart only because both files are `package log` while `logtest` imports `internal/log`; CLAUDE.md's claim that `Sink` is the capture handler for every suite outside `internal/log` reads as universal because of them.

**Solution**: Widen the guard to `_test.go` files with an exemption for `internal/log`'s own in-package tests and a stated ruling on `internal/prefs` (which must not import `internal/log`), and route its enumeration through the shared walk. Separately weigh moving the two affected `internal/log` tests to an external test package so both handlers fold onto `Sink` — they touch unexported seams, so it is not a free move.

## Task 41: The TUI preview surface audit has become a repo-wide new-package tripwire

severity: low
sources: bank

**Problem**: `internal/tui/pagepreview_surface_audit_test.go` enumerates `internal/` and fails on any directory absent from a hardcoded ~30-entry allow-list, so a package added for **any** reason fails a TUI test with a message about the scrollback-preview feature. Its forbidden-name check (preview/scrollback/snapshot) is the part that actually pins the feature; the allow-list is maintenance drag on every future package, and this phase alone has edited it twice (`commandertest`, `harnesstest`).

**Solution**: Keep the forbidden-name check, which pins what the audit exists to pin, and retire the allow-list arm — or replace it with a rule that names the property rather than the inventory, so an unrelated package addition is not a TUI test failure.

## Task 42: The `cmd` doctor stand-down test inventory carries four line-counting scaffolds and two duplicated cases

severity: duplication
sources: bank

**Problem**: `countExactLines`/`countPrefixedLines` (`cmd/doctor_fix_hook_prune_report_test.go:97-112`) duplicate the inline split-and-scan loops in `assertStalePrunesApplied` (`cmd/doctor_test.go:859-869`), `assertSkippedPruneLine` (`cmd/doctor_test.go:1654-1667`) and `renderStaleHooksLine` (`cmd/doctor_stand_down_copy_test.go:153-158`) — one shape, four homes. Two of the four subtests in `doctor_fix_hook_prune_report_test.go` restate coverage that already exists: `:16-28` re-asserts what `assertStalePrunesApplied` pins at `cmd/doctor_test.go:860-869` for the same fixture, and `:30-36` is the same fixture and assertion as `cmd/doctor_test.go:1579-1582`. Separately, the `--fix` exit-code contract is asserted twice per reason from two directions in `cmd/doctor_stand_down_copy_test.go` (`:300-311` and `:398-425`), and `cmd/run_hook_stale_cleanup_outcome_test.go` now holds only `TestStaleHookVerdictParity`, so its filename names a subject it no longer covers.

**Solution**: Give the line-counting shape one pair of package-local helpers all four sites use; drop the two restated subtests in favour of the sibling assertions that already pin them; collapse the healthy half of the exit-code contract into one arm; and rename the outcome file to the subject it now holds.

## Task 43: `verifyLiveStructure` is authored twice across packages

severity: duplication
sources: bank

**Problem**: `internal/restore/integration_full_test.go:208` and `cmd/bootstrap/reboot_roundtrip_test.go:245` both list sessions, assert each expected name is present, then assert each expected window:pane coordinate — differing only in how the expected coordinates are generated (a fixed 2×2 versus a base-index-offset config). The coordinate read itself is already one `restoretest` helper; the surrounding "restore put these sessions and these panes here" assertion is the remaining duplicate.

**Solution**: Promote the assertion into `restoretest` over a shared expectation shape both callers construct, so the two packages assert restore's structural outcome through one description.

## Task 44: Every `cmd/bootstrap` and `internal/tmux` fixture re-sets the state dir the helper already sets, and a harness field is dead

severity: duplication
sources: bank

**Problem**: `IsolateStateForTest` now owns the `PORTAL_STATE_DIR` value, and the fixtures still restate it: `t.Setenv("PORTAL_STATE_DIR", stateDir)` appears at 222 sites tree-wide, including `cmd/bootstrap/helpers_integration_test.go:21`, `upgrade_path_integration_test.go:31,107,153`, `orphan_sweep_integration_test.go:48,105,160`, `composition_abc_integration_test.go:29`, `composition_e2e_harness_integration_test.go:115`, `internal/tmux/portal_saver_endstate_integration_test.go:35,99,168` and `kill_barrier_escalation_no_final_flush_integration_test.go:40,60` — each setting the variable to the value it already holds. Deleting them has one live consequence worth naming in the same change: the teardown-guard coverage rule's `NamesStateDir` trigger keys on the literal string, so a file losing its last mention qualifies via the `Isolates` trigger alone — every one of these files calls `IsolateStateForTest`, so coverage is unchanged, but the reasoning belongs in one deliberate pass. Separately, `compositeHarness.Env` (`cmd/bootstrap/composition_e2e_harness_integration_test.go:25`) is set at `:154` and read nowhere.

**Solution**: Sweep the redundant re-sets in one pass, stating the coverage-trigger reasoning once, and delete the dead harness field.

## Task 45: `openTestLogger` in `internal/state` takes an argument nothing reads

severity: dead-code
sources: bank

**Problem**: `internal/state/fifo_sweep_test.go:15` declares `openTestLogger(t *testing.T, dir string)` whose body is `_ = dir; return logtest.NewCaptureLogger(t)`. Four call sites pass a directory that is discarded — `fifo_sweep_test.go:140` and `capture_test.go:850` pass a real one, `capture_colon_session_test.go:23` and `capture_colon_session_realtmux_test.go:38` pass a throwaway `t.TempDir()` purely to satisfy the parameter, so the pass-through is manufacturing work at two of its four sites. `internal/restore` took exactly this fix already.

**Solution**: Delete the pass-through and call `logtest.NewCaptureLogger(t)` directly at the four sites, dropping the two throwaway temp dirs with it.

## Task 46: The function-seam staging pattern is guarded and helped in `cmd` only

severity: duplication
sources: bank

**Problem**: `withFuncSeam` plus `cmd/seam_guard_test.go` make the capture-and-restore of a package-level seam structural inside `cmd`, and three gaps sit outside that. (a) The same seam pattern exists elsewhere with no guard and no helper: `internal/state/daemon_identity.go:35` (`identifyPS`) and `internal/state/pgrep.go:13` (`pgrepCommand`) are installed by hand across eight sites in that package's tests, and `internal/portaltest/isolated_env.go:117` and `teardown_guard.go:69` the same way. (b) Inside `cmd`, the identical install-and-restore is still hand-written for package-level state that is not a function — `fatalErrorStderr` (`root_test.go:618,651`), `hydrateLogger`, `version`, `daemonLockFile`, and the `bootstrapOnce`/`bootstrapStarted`/`bootstrapWarnings*` block in `bootstrap_reset_test.go:14-24` — even though `withFuncSeam[F any]` constrains nothing but its name. (c) Six per-seam wrappers are now one-line pass-throughs to `withFuncSeam`.

**Solution**: Hoist the staging helper into the neutral test leaf that now exists (`internal/harnesstest`) and point a guard at the packages outside `cmd` that carry seams; then decide the non-function case deliberately — a family-neutral name plus a third guard recogniser arm, which has a real false-positive surface (every `log.For` binding and every cobra command var is a package-level var), or an explicit ruling that non-function state stays hand-staged. The six thin wrappers earn their keep as named vocabulary at their call sites and are a judgement call rather than an obvious deletion.

## Task 47: The `hook rm` cases are split across three files by no visible rule, and one duplicate hides a safety assertion

severity: duplication
sources: bank

**Problem**: `cmd/hooks_test.go:600` ("--pane-key unset falls back to resolveCurrentPaneKey") is a strict subset of `cmd/hooks_rm_exit_test.go:246`, the resolved-token removal home. **A second pair is not a duplicate and must not be collapsed naively**: `cmd/hooks_test.go:571` removes a token-shaped key and asserts an unjudgeable-shaped sibling **survives** (`:594-595`), while `cmd/hooks_rm_exit_test.go:274` removes the unjudgeable key and asserts nothing about its sibling. That surviving-sibling assertion is the sole remaining home of the unjudgeable-key retention property — a stated safety invariant of this codebase, where deleting such an entry is data loss with no route back — so a pick-one-home collapse would silently drop it. Separately, `cmd/hooks_write_lock_test.go:224-236` hand-rolls its staging (SetLockTimeoutForTest + temp file + Setenv + sidecar hold + deps) duplicating `runRmCase` (`cmd/hooks_rm_exit_test.go:76-105`), solely because `assertReturnsAtLockBound` takes a `func() error` it must time — and that helper is shared with the `hook set` timeout test, so the fix reaches both sides.

**Solution**: Collapse the confirmed duplicate into the removal home, **carrying the unjudgeable-sibling assertion into the surviving case**; give the timed case a route through the shared staging by timing the drive from inside it (an elapsed field on the outcome, or a reshaped bound assertion) rather than staging by hand; and settle what `hooks_test.go` keeps now that `hooks_rm_exit_test.go` owns the `hook rm` contract and `hooks_write_lock_test.go` owns the locked routes.

## Task 48: `cmd` config-path tests carry a provably duplicated subtest and hardcoded absolute paths

severity: duplication
sources: bank

**Problem**: `cmd/config_test.go:13` and `:58` have byte-identical bodies — same temp HOME, same `XDG_CONFIG_HOME=""`, same `configFilePath(TEST_CONFIG_UNSET, "projects.json")`, same expectation — because `t.Setenv` cannot express genuinely-unset, so the "no vars at all" versus "explicitly empty" distinction the two names imply does not exist. Separately, eight subtests across three files assert against hardcoded absolute paths rather than `t.TempDir()`: `cmd/config_test.go:45,75,91`, `cmd/prefs_path_test.go:13,29` and `cmd/config_themes_test.go:14,29,44` use `/tmp/xdg-config`, `/tmp/cfg` and `/tmp/h`. Inert today, but it is the shared-mutable-path shape, in the same files as the isolation gap in Task 3.

**Solution**: Fold the two identical subtests into one whose name states the case that actually exists, and re-point the hardcoded paths onto per-test temp dirs.

## Task 49: The read-lock bound pin asserts a claim its own probe falsifies, and its half-relation is integer-division-fragile

severity: low
sources: bank

**Problem**: `internal/hooks/read_lock_test.go:369-371` fails `preRead <= 0` with the message "it must still grant an uncontended lock" — but an empirical probe shows an uncontended acquire is granted at bound 0 **and** at negative bounds, so a positive figure is not what grants it. The assertion actually pins the floor, and its message names something else. Separately, `:373` asserts `preRead >= mutation/2` using Go integer division, which trips at a `lockTimeout` of exactly 10ms (`10000001ns/2 == 5000000ns == bound`) even though the claim holds in real arithmetic; the three sampled values (2s/300ms/60ms) are all far from that edge, so the fragility is latent rather than live.

**Solution**: Re-voice the first assertion to say what it pins — the floor — and express the half-relation so it does not turn on integer division at the crossover.

## Task 50: The rename-refusal copy is a literal in four places with nothing tying them together

severity: medium
sources: bank

**Problem**: The refusal wording lives as constants at `internal/tui/sessions_flash.go:59-60`, and again as literals in the fixture tables at `internal/capture/capture_test.go:926,930` and `internal/capture/swap_harness_test.go:45-46`, and again as prose in `README.md:197`. The test copies are unavoidable today (the constants are unexported and the tables live in another package), but a wording change currently leaves the README stale with **no failing test** — and CLAUDE.md states the coupling as discipline ("user-visible copy, so a change to either must move both"), which is exactly the shape this work unit exists to replace with something structural.

**Solution**: Either export the two strings as a small vocabulary the capture tables read, or add a unit-lane guard asserting the README contains both — the same shape as the repo's other source guards — so the discipline is enforced rather than written down. While there, the refusal is documented only in the README's `hook` section: a user who never registers a hook and meets the refusal from the picker's `r` will not find it, and the keymap table carries no pointer.

## Task 51: The source-reading guards certified under `go test -overlay` are unverified

severity: medium
sources: bank

**Problem**: Confirmed with a standalone module rather than by argument: `-overlay` substitutes the go command's **build** inputs, while `sourceguardtest.ParsePackageSources` reaches disk directly (`parser.ParseFile(fset, path, nil, ParseMode)` with a nil `src`, `internal/sourceguardtest/parsesources.go:50`) and the test binary's working directory is the real package directory. A probe test that reads a map value at runtime **and** parses its own source from disk, run under an overlay, logs the overlaid runtime value alongside the original parsed-from-disk literal. So an overlay probe of a source-reading guard proves only the no-false-positive half; the biting half passes vacuously, and for a guard that compares parsed literals against sibling values from the compiled package it is worse than useless — the overlay makes the two halves disagree and the result is uninterpretable in either direction. Any round in this phase that certified a source-reading guard by overlay should be treated as unverified, which reaches the ~20 `sourceguardtest`-driven guards.

**Solution**: Re-verify each source-reading guard by the method that works — a scratch-copy edit that introduces the violation and observes the failure — and record the method beside the guard family so the next round does not reach for an overlay again.

## Task 52: CLAUDE.md's architecture rows carry three claims the tree has moved past

severity: comments
sources: bank

**Problem**: Three documented statements are now inaccurate or unenforceable. (a) The `tmux` row's `SessionTargetExact` call-site enumeration lists has-session, kill-session, rename-session's `-t`, switch-client, show-environment, set-environment and list-clients, and reads as complete — but `attach-session` also takes one, at `cmd/open.go:88` and `internal/session/quickstart.go:62`, the latter being precisely the "a caller composing a tmux argv the client does not run — an exec chain, say" case the helper's own doc calls out. (b) The `session` row still describes name generation as `{project}-{nanoid}` with no mention that the fragment is sanitised (`SanitiseProjectName` replaces `.` and `:` with `-` and maps a leading `$`), nor that generation is pinned to the recogniser — machinery three sibling tasks in this work unit built. (c) The `logtest` row carries two count-shaped claims of the class the consolidation existed to remove — "the two `slog.Handler`s still declared under `cmd`" and "the five properties every audit-trail line shares" — each true today and each silently falsifiable by an addition with nothing to catch it; and its "`Sink` is the capture handler for every suite outside `internal/log`" is slightly overstated, since `internal/hooks/store_test.go:1239` deliberately captures through a stdlib JSON handler to assert the JSON rendering a `Sink` does not produce.

**Solution**: Add `attach-session` to the enumeration; give the `session` row the sanitisation and generation-pinning sentence; and re-voice the `logtest` row's claims so they name without counting, accounting for the deliberate JSON-handler exception.
