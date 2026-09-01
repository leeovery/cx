# Analysis Tasks: resume-hooks-silently-lost (Cycle 3)

## Task 1: `go test ./cmd` moves the developer's real legacy config
severity: high
sources: bank

**Problem**: `cmd/config_test.go:10` and `:57` call the real `configFilePath("TEST_CONFIG_UNSET", "projects.json")` with `t.Setenv("XDG_CONFIG_HOME", "")` and the developer's real `$HOME`; `cmd/prefs_path_test.go:41` does the same through `prefsFilePath()`. `configFilePath` calls `migrateConfigFile(oldPath, resolved.Path, ...)` on the non-overridden path (`cmd/config.go:77-78`), so on any machine still holding `~/Library/Application Support/portal/` the **unit lane** silently `os.Rename`s `projects.json` and `prefs.json` into `~/.config/portal/`. A reviewer reproduced this on both the pre-change and post-change trees under a staged fake HOME, three runs each. This is a direct breach of the project's absolute invariant that a test must never mutate the real filesystem outside its temp dirs, and it is invisible because the migration is a one-shot that succeeds silently.

**Solution**: Point those subtests at a temp HOME (`t.Setenv("HOME", t.TempDir())`) and assert against that temp home rather than the ambient one, so the migration they trigger lands inside the test's own sandbox. Sweep the rest of `cmd` for any other subtest that exercises a real config-path resolver against the ambient `$HOME`.

**Outcome**: The unit lane can be run on a machine holding the legacy macOS config directory without moving the developer's files.

**Do**:
1. In `cmd/config_test.go`, add `t.Setenv("HOME", t.TempDir())` to the two subtests at `:10` and `:57` before they call `configFilePath("TEST_CONFIG_UNSET", "projects.json")`, and assert the resolved path against that temp home rather than the ambient one.
2. Do the same for `cmd/prefs_path_test.go:41`'s `prefsFilePath()` subtest.
3. Sweep every `*_test.go` in `cmd` for a subtest reaching a real config-path resolver against the ambient `$HOME` — `configFilePath`, `prefsFilePath`, `themesDirPath`, `loadHookStore`, `loadProjectStore`, `loadPrefsStoreNoMigrate` — and give each the same temp HOME.
4. Verify by hand: stage a fake `$HOME` holding `Library/Application Support/portal/projects.json` and `prefs.json`, run `go test ./cmd` against it, and confirm both files are still where they were.

**Acceptance Criteria**:
- [ ] Every `cmd` subtest that resolves a real config path runs against a `t.TempDir()` HOME, never the ambient one.
- [ ] The home-fallback assertions compare against the temp home.
- [ ] `go test ./cmd` run against a staged HOME holding a legacy Application Support config leaves that directory untouched.
- [ ] No production path-resolution behaviour changes — only the environment the tests resolve against.

**Tests**:
- `"it resolves projects.json under the temp home when XDG_CONFIG_HOME is empty"`
- `"it resolves prefs.json under the temp home when XDG_CONFIG_HOME is empty"`
- `"it migrates only the temp home's legacy config directory"` — stages the legacy dir under the temp HOME and asserts the migration lands there

## Task 2: Only `ShowEnvironment` classifies an unaddressable session name; every other per-session op keeps the blind spot
severity: medium
sources: bank

**Problem**: `wrapSessionTargetErr` — the classifier that gates on `ValidateSessionName` before falling through to `wrapNoSuchSession`, so a live colon-named session is not reported as absent — is wired into exactly one call site, `ShowEnvironment` (`internal/tmux/tmux.go:582`). `HasSession`, `KillSession`, `SwitchClient`, `SetSessionEnvironment`, `ListClients` and `saverPanePID` (`internal/tmux/saver_pane_pid.go:15`, which uses the bare `wrapNoSuchSession`) all compose an exact target and will report a live colon-named session as absent, indistinguishably from a real one. `SaverPanePIDOrAbsent` is exported with a free-form session name, so the hole is reachable rather than merely theoretical. Two supporting gaps: `TestSessionTargetsAreComposedExactly` (`internal/tmux/exact_session_target_test.go`) tables six methods and omits `SaverPaneID`, and the prefix-sibling route enumeration in `exact_session_target_realtmux_test.go` omits both saver reads — which is why this class survived to phase 7 in that file at all.

**Solution**: Route every per-session operation through `wrapSessionTargetErr` rather than the bare `wrapNoSuchSession`, add the missing `SaverPaneID` row to `TestSessionTargetsAreComposedExactly`, and derive the real-tmux route enumeration from the set of per-session reads so the next route added is guarded by construction rather than by a later analysis pass. Derivation: the two wrappers differ only when the name contains `:` — for every other name `wrapSessionTargetErr` falls straight through to `wrapNoSuchSession` — so uniform routing changes classification for exactly the class the gate exists to catch and is inert everywhere else. All four production discriminators improve under it: `internal/state/capture.go:71`, `cmd/state_daemon.go:324`, `internal/tmux/saver_pane_pid.go:56` and `internal/session/dirresolve.go:39` each currently read a live colon-named session as absent, which is the misreport `wrapSessionTargetErr`'s own doc comment names. Selective application would leave `KillSession`, `SwitchClient`, `SetSessionEnvironment` and `ListClients` misreporting.

**Outcome**: A live session whose name tmux cannot address exactly is reported as unaddressable by every per-session operation that can report anything, never as absent, and the two guards covering the composition rule enumerate the routes rather than list them.

**Do**:
1. In `internal/tmux/tmux.go`, classify through `wrapSessionTargetErr(<name>, err)` before the outer `fmt.Errorf` on each per-session op that returns an error: `HasSessionProbe` (:96), `KillSession` (:270), `RenameSession` (:280, on `oldName`), `SwitchClient` (:293), `SetSessionEnvironment` (:767).
2. In `internal/tmux/saver_pane_pid.go`, replace `wrapNoSuchSession` at `:15` with `wrapSessionTargetErr(sessionName, err)` and classify `SaverPaneID`'s failure (`:38`) the same way, so `SaverPanePIDOrAbsent`'s absence collapse can no longer swallow a live colon-named session.
3. Leave `ListClients` (`internal/tmux/clients.go:18`) swallowing its error as the zero-clients signal — there is no error for the classifier to carry.
4. Add the `SaverPaneID` row to `TestSessionTargetsAreComposedExactly` (`internal/tmux/exact_session_target_test.go`), command `list-panes`, want `coordTargetForm`.
5. Derive the prefix-sibling route enumeration in `exact_session_target_realtmux_test.go` from one declared set of per-session reads shared with the mock-level table, so a route added tomorrow is covered by construction.

**Acceptance Criteria**:
- [ ] No production site calls `wrapNoSuchSession` directly; every per-session error routes through `wrapSessionTargetErr`.
- [ ] Each converted method fails a live colon-named session with `ErrUnaddressableSessionName` and never with `ErrNoSuchSession`.
- [ ] A genuinely absent session still classifies as `ErrNoSuchSession` on every converted method.
- [ ] `TestSessionTargetsAreComposedExactly` tables `SaverPaneID`, and the real-tmux route enumeration is derived rather than hand-listed.

**Tests**:
- `"it classifies a colon-named session as unaddressable rather than absent"` — table across the converted methods
- `"it still reports a vanished session as no-such-session"`
- `"it collapses a genuine absence but not an unaddressable name in SaverPanePIDOrAbsent"`
- `"it composes SaverPaneID's target exactly"`
- `"it covers every per-session read in the real-tmux prefix-sibling routes"`

## Task 3: Two disagreeing definitions of an unwritable session name, and a `$`-leading name is still silently lost
severity: medium
sources: bank

**Problem**: `internal/session/naming.go:21` strips `:` **and** `.` at generation time through its own `strings.NewReplacer`, with no reference to the target grammar; `tmux.ValidateSessionName` (`internal/tmux/errors.go:74`) is the recognition side and covers `:` only. They agree today by coincidence, not by construction — the same generation/recognition drift the pane-token work consolidated into `internal/nanoid`. The gap is not academic: measured on tmux 3.7c against an isolated socket, a session named `$foo` cannot be addressed by name at all (both `-t "$foo"` and `-t "=$foo"` fail — tmux reads a leading `$` as a session-ID prefix), `ValidateSessionName` returns nil for it, so `wrapSessionTargetErr` falls through and `internal/state/capture.go` logs a vanished session and drops a live one. That end state is precisely the silent loss this work unit exists to remove, reached by a different character. Also measured: `.` alone is fine (`a.b` resolves both bare and exact), so the generator sanitises something the target grammar does not forbid.

**Solution**: Widen `ValidateSessionName` to refuse a leading `$` alongside the embedded `:`, and pin the generator to it with a guard asserting every name `GenerateSessionName` produces passes `ValidateSessionName` — which forces `SanitiseProjectName` to handle a project directory whose name begins with `$`, reachable today because it replaces only `.` and `:`, so `$foo` mints the unaddressable `$foo-abc123`. Derivation: the two rules are deliberately not one rule and must not be collapsed. Generation sanitises a project *fragment* for tidiness and may strip a superset — `.` is measured addressable, so stripping it is a naming choice rather than a correctness one — while recognition classifies an *arbitrary* name, including one a user created outside Portal, and must refuse exactly what tmux cannot address, including a positional rule (`$` only when leading) that has no meaning for a fragment placed mid-name. The generated-name guard is what closes the drift the finding names, without forcing either rule into the other's shape. The staged import objection does not arise: `internal/session` already imports `internal/tmux`.

**Outcome**: A session named `$foo` is refused where Portal mints names and classified as unaddressable where Portal reads them, and no name `GenerateSessionName` can produce is one `ValidateSessionName` rejects.

**Do**:
1. In `internal/tmux/errors.go`, widen `ValidateSessionName` to refuse a leading `$` alongside the embedded `:`, wrapping `ErrUnaddressableSessionName` and naming the offending character the way the separator case already does. The `$` rule is positional — only a leading one is refused.
2. In `internal/session/naming.go`, make `SanitiseProjectName` handle a fragment beginning with `$` so `$foo` cannot mint `$foo-abc123`; leave the existing `.` and `:` replacements alone.
3. Add a guard in `internal/session` asserting every name `GenerateSessionName` returns passes `tmux.ValidateSessionName`, over a table of hostile project directory names (`$foo`, `$`, `a:b`, `a.b`, `$a:b`).
4. Keep the two rules distinct: generation may strip a superset (a `.` is addressable and stays sanitised for tidiness), recognition refuses exactly what tmux cannot address.

**Acceptance Criteria**:
- [ ] `ValidateSessionName("$foo")` returns an error wrapping `ErrUnaddressableSessionName`; `ValidateSessionName("a$b")` returns nil.
- [ ] `ValidateSessionName` still accepts a name containing `.`, and `SanitiseProjectName` still replaces it.
- [ ] No project directory name makes `GenerateSessionName` produce a name `ValidateSessionName` refuses.
- [ ] The capture loop's discriminator reports a live `$`-leading session as unaddressable rather than vanished.

**Tests**:
- `"it refuses a session name beginning with $"`
- `"it accepts a $ that is not leading"`
- `"it accepts a name containing a period"`
- `"it generates only names ValidateSessionName accepts"` — table of hostile project names
- `"it sanitises a project directory whose name begins with $"`

## Task 4: Bare pane targets still reach the tmux client from two production sites, and the guard cannot see either
severity: medium
sources: bank

**Problem**: `cmd/state_daemon.go:277` composes `tmux.PaneTarget(sess.Name, win.Index, pane.Index)` — the deliberately unpinned form — and hands it to `CapturePane` through `state.CaptureAndHashPane`, so a session killed between the index build and the capture leaves the read resolving onto a prefix sibling's pane, whose scrollback is then written under the gone pane's key. `cmd/open.go:90` hand-rolls the exact-target rule itself (`[]string{"tmux", "attach-session", "-t", "=" + name}`) rather than calling `tmux.ExactSessionTarget`, making it the fifth site to author that rule by hand. Neither is caught because `targetComposingPackages` (`internal/tmux/target_composition_guard_test.go:18`) is hardcoded to `{".", "../session", "../restore"}` — `cmd` is not in it, and nothing stops the next package from composing targets unguarded either.

**Solution**: Route the daemon capture loop's pane target through the pinned `PaneTargetExact` and `cmd/open.go` through `tmux.ExactSessionTarget`, then widen the guard's package set so `cmd` is covered and a new target-composing package cannot appear outside it — deriving the set (every package importing `internal/tmux`, say) rather than extending the literal list.

**Outcome**: No production site hands the tmux client a bare target, and the guard's package set follows the import graph rather than a hand-maintained list.

**Do**:
1. In `cmd/state_daemon.go:277`, compose the capture read's target with `tmux.PaneTargetExact(sess.Name, win.Index, pane.Index)`; leave `state.SanitizePaneKey` as the scrollback key.
2. In `cmd/open.go:90`, build the attach argv's target with `tmux.ExactSessionTarget(name)` instead of the hand-rolled `"=" + name`.
3. In `internal/tmux/target_composition_guard_test.go:18`, derive `targetComposingPackages` from the module's import graph — every package importing `internal/tmux`, plus `internal/tmux` itself — so `cmd` is covered and a new composer joins by construction.
4. Run the widened guard and settle whatever else it surfaces in `cmd`: pin the target, or add the parameter to the pass-through allow-list where the target genuinely arrives composed.

**Acceptance Criteria**:
- [ ] The daemon's capture read addresses `=session:window.pane`.
- [ ] `cmd/open.go` composes no exact-target prefix by hand.
- [ ] The guard's package set is derived, includes `cmd`, and the derivation fails rather than silently empties if the import scan resolves nothing.
- [ ] The guard flags a bare target composed in `cmd` (proven by a staged probe).

**Tests**:
- `"it captures a pane through the pinned pane target"`
- `"it attaches through ExactSessionTarget"`
- `"it derives the scanned package set from the packages importing internal/tmux"`
- `"it flags a bare target composed in cmd"`

## Task 5: A failed `@portal-restoring` read is reported to the user as "restore in progress"
severity: medium
sources: standards

**Problem**: `hookStalenessStandDown` (`cmd/run_hook_stale_cleanup.go:136-141`) folds a failed marker read into the `restoring` reason via `state.RestoreWindowActive` — the correct fail-safe posture — but that reason then drives both user-facing surfaces: `portal doctor --fix` prints `Skipped stale hook prune: restore in progress` and the read-only diagnosis reports `restore in progress (not evaluable)`. `doctor` is bootstrap-exempt and starts no server, so with the tmux server down — the routine state after a reboot, and precisely when a user reaches for `doctor` — `TryGetServerOption` fails, no pattern matches "no server running", and the user is told a restore is running when none is. Before this change that path rendered the honest "zero live panes with hooks present". The specification's own 2026-09-01 corrigendum withdrew a stand-down phrase for exactly this class of mismatch — words that describe a condition other than the one that produced them — and its remedy was to give each condition its own phrase rather than let one borrow another's.

**Solution**: Keep the fail-safe fold — a failed read must still stand the cycle down — but separate the rendered phrase from the marker-set phrase by carrying the read failure as its own reason, so `skippedPrunePhrases` and `notEvaluableDetails` can say the marker could not be read rather than asserting a restore. The log line and the stand-down semantics are unaffected.

**Outcome**: With the tmux server down, `portal doctor` and `portal doctor --fix` say the restore marker could not be read; a restore that is genuinely running still says so.

**Do**:
1. Add `skipReasonMarkerReadFailed` beside the other reasons in `cmd/run_hook_stale_cleanup.go` and enrol it in `skipReasons`.
2. In `hookStalenessStandDown` (`:136`), keep the fail-safe fold but pick the reason from the read: a non-nil error out of `state.RestoreWindowActive` declines under the new reason, a clean read reporting the marker set declines under `skipReasonRestoring`. The DEBUG level and the `error` attr stay as they are.
3. Give the new reason its phrase in both vocabularies — one for `Skipped stale hook prune: …` and one for the not-evaluable detail — neither borrowing `restoreStandDownPhrase`.
4. Leave `state.RestoreWindowActive`'s fail-safe semantics, the `op`/`via` attrs and the stand-down's effect on the cycle untouched.

**Acceptance Criteria**:
- [ ] A failed `@portal-restoring` read renders a phrase naming the read failure on both surfaces, and never asserts a restore.
- [ ] A marker that is genuinely set still renders the restore phrase on both surfaces.
- [ ] The cycle still stands down on a failed read — no key is judged and nothing is written.
- [ ] The completeness guards over `skipReasons` and both phrase maps cover the new reason.

**Tests**:
- `"it reports a failed marker read as its own reason"`
- `"it reports an in-progress restore as a restore"`
- `"it stands the sweep down when the marker read fails"`
- `"it renders doctor's not-evaluable detail for a failed marker read"`
- `"it names a phrase for every stand-down reason on both surfaces"`

## Task 6: An unspecified session-name refusal with new user-facing copy shipped inside this work unit, documented nowhere
severity: medium
sources: standards

**Problem**: The implementation introduced `ErrUnaddressableSessionName` / `ValidateSessionName`, made `Client.RenameSession` refuse any name containing `:` before composing its argv (`internal/tmux/tmux.go:283`), added a matching refusal to the TUI rename modal with new flash copy (`internal/tui/model.go:2667`, `sessions_flash.go:54` — `":" isn't allowed in a session name — tmux reads it as a separator`), and converted a broad set of previously-bare `-t` targets to the exact-match forms. None of it is in the specification, which scopes the positional/structural addressing siblings as checked against the change rather than changed by it. It is a user-visible behaviour change — a rename the picker accepted yesterday is now refused — riding a release whose stated contract is the hook-key repair, and the copy went through no visual gate. README:195 still tells the user hooks stay attached "even if you rename it, whether from the picker's `r` modal" with no mention that some names are now refused, and neither README nor CLAUDE.md's `tmux` row records the refusal or the new exact-target helpers.

**Solution**: The refusal stays. Document it in README's rename section — which today promises hooks survive a rename "whether from the picker's `r` modal" with no mention that some names are now refused — and in CLAUDE.md's `tmux` row alongside the exact-target helpers, and put the flash copy through the same visual gate every other picker string gets. Derivation: the fork is not live. Tasks 2 and 3 of this cycle, both approved, route every per-session operation through `wrapSessionTargetErr` and widen `ValidateSessionName` to the leading `$`, so the classification machinery the refusal sits on is load-bearing whatever happens here. With it in place, removing the picker-side refusal does not restore the old behaviour: the modal accepts the rename and the client then refuses it with a raw error instead of the flash. Side 2 buys a worse surface for the same outcome, which leaves documentation and the visual gate as the whole of the work.

**Outcome**: The refusal and the exact-target vocabulary are described where the project describes behaviour, and the rename flash copy has been seen at a visual gate.

**Do**:
1. Amend README's rename section (around `:195`) so the promise that hooks survive a rename is stated alongside what is now refused: a name Portal's exact-match target cannot address — one containing `:`, and a leading `$` once task 3 lands — is rejected by both the picker's `r` modal and the client's `RenameSession`.
2. Amend CLAUDE.md's `tmux` row to record `ValidateSessionName` / `ErrUnaddressableSessionName`, `RenameSession`'s pre-argv refusal, and the exact-target vocabulary (`ExactSessionTarget`, `ExactCoordTarget`, `PaneTargetExact`, `windowTargetExact`) with the session-versus-coordinate distinction that decides which one a call site takes.
3. Put the flash copy at `internal/tui/sessions_flash.go:54` through the visual gate: render it via `go run ./cmd/capturetool --fixture <name>` (adding a fixture for the rename-refusal state if none renders it), capture the still, and check it against the picker's other flash copy for wording and framing.
4. Reconcile the README sentence and the flash string so the two say the same thing in the same words.

**Acceptance Criteria**:
- [ ] README's rename section names the refusal and exactly what it refuses.
- [ ] CLAUDE.md's `tmux` row names the refusal and the four exact-target helpers.
- [ ] The rename-refusal flash has been rendered and reviewed at a gate, with the fixture command recorded.
- [ ] README's wording and the flash string agree; no user-facing string ships un-gated.

**Tests**:
- `"it refuses a rename to a name containing a separator"` — existing `internal/tmux` and `internal/tui` coverage, unchanged by the documentation
- `"it lists the rename-refusal fixture"` — the capture package's fixture-coverage assertion, if a fixture is added

## Task 7: `internal/hooks` spells its one on-disk shape three different ways across its exported API
severity: duplication
sources: architecture

**Problem**: `map[string]map[string]string` — the `hooks.json` body — appears under three names on the package's public face. `hooksFile` is an *unexported alias* (`internal/hooks/store.go:29`) and the **exported** `Load(via Via) (hooksFile, error)` returns it, so godoc and every reader of the signature see an identifier they cannot name or follow. `Snapshot` is an exported defined type for the identical shape, taken by the exported `CleanStale`. And `StaleKeys` spells the raw map literally — it had to, because the alias it would naturally use is unexported. The consequence is visible where two of them meet (`cmd/doctor.go:305` feeding `store.Load(...)` into `hooks.StaleKeys(...)`): the value flows only because the alias is transparent, and nothing in the signatures tells the reader these are the same thing.

**Solution**: Name the shape once as an exported defined type — reuse `Snapshot`, or introduce `Entries` and document `Snapshot` as its role alias — and use it in `Load`, `StaleKeys`, `CleanStale` and the unexported `load`/`save`/`staleKeys` helpers. `Load` in particular must stop returning an unexported identifier.

**Outcome**: `internal/hooks` names the `hooks.json` body once, on its public face, and every signature that carries it says the same word.

**Do**:
1. Delete the unexported `hooksFile` alias (`internal/hooks/store.go:29`) and use the exported defined type `Snapshot` throughout: `Load`, `loadShared`, `loadSharedBounded`, `loadSnapshot`, `load`, `save`, `classifySet`, `staleKeys` and `deleteStale`.
2. Change `StaleKeys`'s first parameter from the raw `map[string]map[string]string` to `Snapshot`; leave `CleanStale`'s callback signature as it is.
3. Rewrite `Snapshot`'s doc comment so it names the on-disk shape first and the clean's older-view role second.
4. Re-point the callers the change touches (`cmd/doctor.go:305` and `:313`, the `internal/hooks` and `cmd` suites) — a raw map stays assignable to the defined type, so no call site needs a conversion.

**Acceptance Criteria**:
- [ ] `internal/hooks` declares one name for the on-disk shape; `hooksFile` is gone.
- [ ] `Load` returns an exported type.
- [ ] No call site introduces a conversion to satisfy the new signatures.
- [ ] `go build ./...` and both lanes pass.

**Tests**:
- Pure refactor: behaviour is unchanged, the existing `internal/hooks` and `cmd/doctor` suites stay green, and no test's semantics change.

## Task 8: `cmd` carries two opposing `*Deps` merge conventions, and the fail-silent one is unguarded
severity: medium
sources: architecture

**Problem**: `hookSeams()` (`cmd/hooks.go:83`) starts from whatever the test injected and *fills in* the production default per unset seam, so a field added to `HooksDeps` without a fill line is a nil dereference — loud and immediate. The two sibling resolvers beside it run the opposite direction: `resolveDoctorDeps` (`cmd/doctor.go:74`) and `resolveCommitNowDeps` build production defaults and then overwrite field-by-field from the injected struct (`if doctorDeps.X != nil { deps.X = doctorDeps.X }`, eleven times). A field added to `DoctorDeps` or `CommitNowDeps` without its matching override line is **silent** — the test's mock is ignored, the production default runs, and nothing anywhere fails. Two conventions for one job in one package is the drift; the fail-silent direction being the unguarded one is the sharper half.

**Solution**: Convert `resolveDoctorDeps` and `resolveCommitNowDeps` to the fill-in-defaults shape `hookSeams` already uses in the same package, so a field added to a `*Deps` struct without its resolver line is a nil dereference at first use rather than a silently ignored mock. Derivation: the two directions are not equally safe and the repo's stated house style is structural enforcement over contributor discipline — the loud direction needs no guard to be remembered, while side 2 keeps the fail-silent shape and adds a reflection guard that must itself be maintained and can rot. Converging on the shape already present in the package introduces nothing new.

**Outcome**: One `*Deps` merge convention across `cmd` — start from the injection, fill the unset — so a seam added without its resolver line fails loudly at first use instead of ignoring the test's mock.

**Do**:
1. Rewrite `resolveDoctorDeps` (`cmd/doctor.go:74`) in the `hookSeams` direction: start from `*doctorDeps` when non-nil, then fill each unset field with its production default, deleting the eleven `if doctorDeps.X != nil` overrides.
2. Construct the best-effort loads (`loadHookStore`, `loadProjectStore`, `loadPrefsStoreNoMigrate`, `themesDirPath`) only for a field left unset, so an injected store costs no config read; keep `loadPrefsStoreNoMigrate` as the doctor's non-migrating route.
3. Rewrite `resolveCommitNowDeps` (`cmd/state_commit_now.go:49`) the same way.
4. Keep the string fields' zero value as the unset signal (`StateDir`, `ThemesDir`), matching what the injection already means.

**Acceptance Criteria**:
- [ ] Neither resolver overwrites a production default from the injected struct; both fill unset fields only.
- [ ] Every field a test injects is the one the command runs against (existing doctor and commit-now suites pass unchanged).
- [ ] A resolver performs no config load for a field the test injected.
- [ ] A field added to either `*Deps` without a fill line is a nil dereference at first use, not a silently ignored mock.

**Tests**:
- `"it runs against the injected seam for every field a test sets"` — table over the exported fields of `DoctorDeps`
- `"it falls through to the production default for an unset field"`
- `"it loads no hook, project or prefs store when one is injected"`
- `"it resolves commit-now's seams by the same rule"`

## Task 9: `doctor --fix` prints nothing at all when the sweep genuinely fails
severity: medium
sources: bank

**Problem**: `pruneDoctorStaleHooks` (`cmd/doctor.go:197-210`) logs a WARN on a returned error and then renders from the zero `sweepOutcome` — so there is no `Pruned stale hook:` line and no `Skipped stale hook prune:` line, and a user who asked for a repair sees nothing at all. This is the same argument the stand-down lines exist to answer — a repair that printed nothing is indistinguishable from one that found nothing — now visible on the one path the stand-downs do not cover.

**Solution**: Give the error path its own user-facing line so every `--fix` invocation that touched the hook sweep says what happened, and settle the related default while the file is open: the nil-`countsLogger` fallback attributes the cycle's counts to the `bootstrap` component even though a guard forbids a bootstrap step from calling the sweep (reachable only from tests today, since both production callers supply a logger).

**Outcome**: Every `portal doctor --fix` run that reached the hook sweep prints exactly one line about it — pruned keys, a stand-down, or a failure — and the cycle's counts default to the component that owns them.

**Do**:
1. In `pruneDoctorStaleHooks` (`cmd/doctor.go:197-210`), render a user-facing line on the returned-error path as well, keeping the WARN, so the failure is reported rather than swallowed into a zero `sweepOutcome`.
2. Render it from the vocabulary rather than a literal: route through `phraseFor(skippedPrunePhrases, …)`, enrolling a reason in `skipReasons` and both phrase maps if none of the five fits the failure.
3. Change `runHookStaleCleanup`'s nil-`countsLogger` fallback (`cmd/run_hook_stale_cleanup.go:237-239`) from `bootstrapLogger` to the sweep's own `hooksLogger`, since a bootstrap step is forbidden from calling the sweep and both production callers pass a logger.
4. Leave the exit code alone — `--fix` drives it from the post-repair diagnosis.

**Acceptance Criteria**:
- [ ] A sweep that returns an error prints one line naming the failure, and still logs the WARN.
- [ ] No `--fix` invocation that reaches the hook sweep prints nothing about it.
- [ ] The printed phrase comes from `skippedPrunePhrases`, not an inline literal.
- [ ] `runHookStaleCleanup(reader, store, nil)` attributes the cycle's DEBUG counts to the hooks component.

**Tests**:
- `"it prints a skipped line when the stale-hook sweep fails"`
- `"it prints one pruned line per reaped key"`
- `"it prints a skipped line when the sweep stands down"`
- `"it attributes the cycle counts to the hooks component when no logger is supplied"`

## Task 10: Integration fixtures disagree about which state directory they isolated
severity: medium
sources: duplication, bank

**Problem**: Three fixtures call `portaltest.IsolateStateForTest(t)` for its side effects only, discarding both return values, and then point `PORTAL_STATE_DIR` at a fresh `t.TempDir()`: `internal/restore/rename_reboot_shared_test.go:52`, `internal/restore/prefix_sibling_integration_test.go:33` and `internal/restore/multipane_legacy_integration_test.go:60`. The sandbox registry and the in-process daemon-ownership registration therefore name a directory nothing writes to while the daemon writes to an unregistered one, so a subprocess orphan sweep cannot recognise the fixture's own daemon — the exact property the registry exists to provide. They pass the teardown-guard coverage rule because they make all three calls; the rule says nothing about the *value* `PORTAL_STATE_DIR` is set to. Underneath sits the reason the divergence was possible: the same six-line preamble (`BuildPortalBinaryDir` → `IsolateStateForTest` → `t.Setenv("PORTAL_STATE_DIR", …)` → `state.EnsureDir` → `RegisterStateDirTeardownGuard`) is written out verbatim at roughly ten sites in `internal/restore` alone, and `IsolateStateForTest` derives its returned env slice from `os.Environ()` *before* the caller's `Setenv` runs, so the slice it hands back never carries the real state dir (under `cmd/bootstrap` it carries the `TestMain` poison).

**Solution**: Have `IsolateStateForTest` own `PORTAL_STATE_DIR` in both the process env and the returned slice, and point the three divergent fixtures at the helper's returned `stateDir`. Derivation: this removes the divergence at its source — the helper derives its env slice from `os.Environ()` before the caller's own `Setenv` runs, which is exactly what let the sandbox registry and the daemon name different directories. The further extraction of the shared preamble into a `restoretest` helper is declined: it requires teaching the per-file coverage rule to follow calls into a package-local arrange, and `cmd/bootstrap`'s five fixtures are already invisible to that rule for precisely that reason, so it would widen an existing blind spot in the guard underwriting the project's absolute test-isolation invariant. The remaining preamble duplication is accepted residue — once the helper owns the value, the copies can no longer diverge on it.

**Outcome**: The directory the sandbox registry names, the directory the returned env slice names and the directory the fixture's daemon writes to are the same directory, in every fixture.

**Do**:
1. In `internal/portaltest/isolated_env.go`, `t.Setenv("PORTAL_STATE_DIR", stateDir)` immediately after the state dir is created and **before** the `os.Environ()` read at `:71`, and filter-then-append `PORTAL_STATE_DIR=<stateDir>` into the returned slice the way `XDG_CONFIG_HOME` and `TMUX` already are.
2. Document on `IsolateStateForTest` that the returned slice now carries the state dir, and that a fixture needing a different one overrides it after the call (process env via `t.Setenv`, slice by appending — `exec.Cmd` dedupe is last-wins).
3. Point the three divergent fixtures at the returned `stateDir`, deleting their `t.TempDir()` plus `t.Setenv("PORTAL_STATE_DIR", …)` pair: `internal/restore/rename_reboot_shared_test.go:52`, `internal/restore/prefix_sibling_integration_test.go:33`, `internal/restore/multipane_legacy_integration_test.go:60`.
4. Leave the rest of the preamble inline at each fixture — the extraction into a `restoretest` helper is declined, so the per-file coverage rule keeps seeing the calls.

**Acceptance Criteria**:
- [ ] `IsolateStateForTest` sets `PORTAL_STATE_DIR` in the process env and carries it in the returned slice, both naming the returned `stateDir`.
- [ ] The sandbox registry file's contents match the `PORTAL_STATE_DIR` the slice carries.
- [ ] The three fixtures write to, register and tear down one directory.
- [ ] `cmd/bootstrap`'s fixtures still resolve their own state dir by setting it after the helper.

**Tests**:
- `"it sets PORTAL_STATE_DIR to the returned state dir"`
- `"it carries PORTAL_STATE_DIR in the returned env slice"`
- `"it registers the same directory with the sandbox registry as the slice names"`
- `"it lets a caller override the state dir after the call"`

## Task 11: The teardown-guard coverage rule checks call presence, never call order
severity: medium
sources: bank

**Problem**: `auditFixtureCoverage` (`internal/portaltest/teardown_guard_coverage_test.go`) records four booleans and asks only whether the fixture isolates and registers the guard. A file that calls `IsolateStateForTest` and `RegisterStateDirTeardownGuard` **after** `tmuxtest.New` — the inverted-LIFO shape this phase repaired by hand three times, at `internal/restore/armed_restore_integration_test.go`, `internal/restore/integration_test.go` and `cmd/reattach_integration_test.go` — passes the rule unchanged. Separately, the five `cmd/bootstrap` fixtures routed through the shared `newIntegrationStateDir` arrange now name no `PORTAL_STATE_DIR` and call neither helper directly, so the rule's `qualifies()` is false for all of them and `helpers_integration_test.go` never starts a server, so it does not qualify either: correct by construction today, invisible tomorrow.

**Solution**: `fixtureCallsIn` already parses with positions, so compare the first server-creating call's position against the isolate and guard positions within a function and fail the inverted order. Settle in the same pass how the rule sees a fixture that reaches the helpers through a package-local arrange, so routing a suite through a shared setup stops being a way out of the guard.

**Outcome**: The coverage rule fails the inverted-LIFO shape this phase repaired by hand three times, and a suite routed through a shared arrange is judged rather than skipped.

**Do**:
1. Extend `fixtureCalls` (`internal/portaltest/teardown_guard_coverage_test.go:28`) to carry positions rather than booleans — the first `tmuxtest.New` position and the `IsolateStateForTest` / `RegisterStateDirTeardownGuard*` positions — recorded per enclosing function.
2. Fail a function whose first server-creating call precedes either the isolate or the guard registration, with a message naming the inverted order and the offending line.
3. Teach the rule to follow one level of package-local call: a function calling a same-package helper that isolates and registers counts as doing so itself, so `cmd/bootstrap`'s five `newIntegrationStateDir` fixtures and `helpers_integration_test.go` are judged.
4. Keep the scanned-nothing tripwire in `coverageFailure`, and cover both new arms with staged source fixtures rather than real files.

**Acceptance Criteria**:
- [ ] A file starting a server before isolating, or before registering the guard, fails the rule.
- [ ] A fixture reaching both helpers through a package-local arrange qualifies and passes; one whose arrange omits the guard fails.
- [ ] The rule still fails when it scans nothing.
- [ ] The tree passes under the extended rule.

**Tests**:
- `"it fails a fixture that starts a server before isolating"`
- `"it fails a fixture that registers the teardown guard after the server"`
- `"it counts a fixture that isolates through a package-local arrange"`
- `"it fails a package-local arrange that omits the guard"`
- `"it fails when no fixture qualifies"`

## Task 12: The fingerprint backstop is narrower than its own doc and CLAUDE.md claim
severity: medium
sources: bank

**Problem**: `IsolateStateForTest` re-points `HOME` at a temp dir and clears `XDG_CONFIG_HOME` (`internal/portaltest/isolated_env.go:28-29`) *before* calling `resolveDevStateDir()`, so `devStateDir` resolves to `<tempHOME>/.config/portal/state` rather than the developer's install. `resolveDevStateDir`'s own doc comment states the contract this violates: "reads the ambient env, so it must run before any override is installed or it resolves the per-test temp dir instead of the developer's install" (`internal/portaltest/fingerprint.go:279-281`), and CLAUDE.md likewise describes the backstop as walking the developer's state dir post-test. The current ordering is *deliberate* and argued in-source (a live host daemon would otherwise false-trip the backstop mid-test) — so the guarantee is real but narrower than two places claim: it catches a process that got the HOME scrub but resolved the default path, not one that escaped the scrub entirely.

**Solution**: Correct `fingerprint.go`'s comment and CLAUDE.md to the narrower guarantee the code deliberately provides: the backstop catches a process that took the HOME scrub and still resolved the default path, not one that escaped the scrub entirely. Derivation: the current ordering is argued in-source and the argument holds — a live host daemon would otherwise false-trip the backstop mid-test. Reordering the snapshot ahead of the scrub reintroduces that false trip and leaves "solve it another way" undesigned; keeping both snapshots doubles the backstop's cost on every isolated fixture to catch a class the primary defence already prevents. The project's own words settle the weight: the backstop is defence-in-depth, not a substitute for the env override, so the description is what should move.

**Outcome**: `fingerprint.go` and CLAUDE.md describe the guarantee the ordering actually provides, and the ordering itself is pinned so a future reorder is loud rather than silent.

**Do**:
1. Rewrite `resolveDevStateDir`'s doc comment (`internal/portaltest/fingerprint.go:279-281`) to state what it does: it runs *after* the HOME scrub deliberately, so the snapshotted path is the per-test temp HOME's state dir, and the backstop catches a process that took the scrub and still resolved the default path — not one that escaped the scrub entirely.
2. Keep the in-source argument for that ordering (a live host daemon would otherwise false-trip the backstop mid-test).
3. Correct CLAUDE.md's two claims to the same narrower guarantee — the `portaltest` row's "walks the developer's state dir post-test" and the matching sentence in the test-isolation section — keeping "defence-in-depth, not a substitute for the env override".
4. Pin the ordering with a test so a reorder fails rather than silently widening or narrowing the backstop.

**Acceptance Criteria**:
- [ ] `resolveDevStateDir`'s comment describes the post-scrub resolution and the reason for it.
- [ ] CLAUDE.md no longer claims the backstop walks the developer's real install.
- [ ] The snapshot ordering is asserted by a test.
- [ ] No behaviour changes — the backstop keeps its current reach.

**Tests**:
- `"it resolves the dev state dir under the scrubbed HOME"`
- `"it registers the backstop over the resolved dir"`

## Task 13: The temp-HOME teardown race is unguarded, and its remedy already exists unshared in one fixture
severity: medium
sources: bank

**Problem**: `IsolateStateForTest` re-points `HOME` at a `t.TempDir()` and neutralises only shell *history* (`HISTFILE=/dev/null`). A restored pane's `$SHELL` writes other per-session files there — zsh's `.zsh_sessions` — and `RegisterStateDirTeardownGuard` waits on the **state** dir only, so the framework's `RemoveAll` of HOME races those writers and fails the test after its assertions passed. Observed as `TempDir RemoveAll cleanup: unlinkat .../002: directory not empty` on `TestNonContiguousWindowReboot_KeepsTokenKeyedHooks` and proved by instrumentation to be the temp HOME rather than the state dir; a second reviewer corroborated it independently. The shape is reachable from every fixture that restores a pane into an interactive shell. The remedy is already written in this repo and never promoted: `reapTmuxServer` (`cmd/concurrent_coldboot_integration_test.go:65-76`) blocks until the tmux server is unreachable for exactly this reason, and its comment names the lingering-shell-into-isolated-HOME race.

**Solution**: Promote that wait into `internal/portaltest` — registered alongside `RegisterStateDirTeardownGuard`, or as a HOME-scoped quiescence wait beside the state-dir one — so the whole class closes for every restore fixture at once rather than one suite at a time. Pinning the shell's session directory the way `HISTFILE` is pinned is the alternative worth weighing in the same pass.

**Outcome**: A fixture that restores a pane into an interactive shell tears down cleanly, without each suite having to rediscover the wait.

**Do**:
1. Promote the server-unreachable wait out of `reapTmuxServer` (`cmd/concurrent_coldboot_integration_test.go:65-76`) into `internal/portaltest` as an exported, bounded helper that blocks until the named tmux socket answers nothing, and re-point that suite at it, deleting the local copy.
2. Register the wait for every fixture rather than per suite: either inside `RegisterStateDirTeardownGuard` (taking the socket) or as a HOME-scoped quiescence wait registered beside it, ordered so it runs between kill-server and the framework's `RemoveAll`.
3. Pin the shell's session directory the way `HISTFILE` is pinned in `IsolateStateForTest` (`:35`) — set `SHELL_SESSIONS_DISABLE=1` and point `ZDOTDIR` at the temp HOME, before the `os.Environ()` read so the returned slice carries them — so zsh writes no `.zsh_sessions` into the temp HOME at all.
4. Verify against the observed failure: run `TestNonContiguousWindowReboot_KeepsTokenKeyedHooks` ten consecutive times and confirm no `TempDir RemoveAll` failure.

**Acceptance Criteria**:
- [ ] One exported helper owns the wait; the local copy in the coldboot suite is gone.
- [ ] Every fixture registering the teardown guard also gets the wait, without opting in.
- [ ] The isolated env neutralises the shell's session directory as well as its history, in both the process env and the returned slice.
- [ ] Ten consecutive runs of the named fixture tear down cleanly.

**Tests**:
- `"it blocks until the tmux server is unreachable"`
- `"it returns at its bound rather than hanging on a server that will not exit"`
- `"it neutralises the shell session directory in the isolated env"`
- `"it carries the shell-session env in the returned slice"`

## Task 14: The restore-binary Exe pin has two unguarded routes
severity: medium
sources: bank

**Problem**: The source guard added for this class covers `restore.Orchestrator` composite literals in `*_test.go` only, and two doors past it remain. `internal/restore/prefix_sibling_integration_test.go` pins a bare `restore.SessionRestorer` rather than an Orchestrator and resolves the exe by hand, so a `SessionRestorer` built without a pinned exe hits the same silent test-binary-arming failure the guard exists to foreclose. `internal/bootstrapadapter/adapters.go:64` builds a `restore.Orchestrator` without requiring `Exe` and is not a composite literal in a test file, so it bypasses the guard entirely — latent today (its only callers are `restoretest.StagedRestoreAdapter` and production), but it is the second door.

**Solution**: Close both with the proportionate shape a reviewer already established: a `restoretest.NewSessionRestorer(t, client, stateDir, binDir)` constructor plus a guard **scoped to integration-tagged test files**, not a blanket extension of the Orchestrator guard — roughly 70 sites in `session_test.go` and its siblings compose a `SessionRestorer` bare and mock-driven, where an unset `Exe` is harmless, and only the one integration fixture drives it against a live server. Require `Exe` at the adapter constructor, or guard it there.

**Outcome**: No route to a live-server restore leaves `Exe` unpinned — neither the bare `SessionRestorer` fixture nor the adapter constructor — while the mock-driven unit-lane literals stay as they are.

**Do**:
1. Add `restoretest.NewSessionRestorer(t, client, stateDir, binDir)` composing a `restore.SessionRestorer` with `Exe` pinned to the staged portal binary.
2. Re-point `internal/restore/prefix_sibling_integration_test.go` at it and delete its hand-rolled exe resolution.
3. Add a guard scoped to **integration-tagged** `*_test.go` that fails a `restore.SessionRestorer` composite literal with no `Exe` field, leaving the ~70 unit-lane literals in `session_test.go` and its siblings untouched.
4. Close the second door at `internal/bootstrapadapter/adapters.go:64`: take the exe as a constructor parameter, or fail construction when it is empty, so a non-test caller cannot build an `Orchestrator` with an unset `Exe`.

**Acceptance Criteria**:
- [ ] No integration-tagged fixture composes a `SessionRestorer` without a pinned `Exe`.
- [ ] The unit lane's bare literals still compile and pass, unguarded.
- [ ] The adapter constructor cannot produce an orchestrator with an empty `Exe`.
- [ ] The new guard fails on a staged offending literal and passes on the real tree.

**Tests**:
- `"it pins the staged binary on the restorer it returns"`
- `"it flags an integration fixture composing a SessionRestorer without an Exe"`
- `"it ignores a unit-lane composite literal"`
- `"it refuses to build the restore adapter without an exe"`

## Task 15: Fifteen fixed-budget `PollUntil` sites are the flake class already fixed for seven
severity: medium
sources: bank

**Problem**: Task 7-9 converted seven daemon-lifecycle waits from a fixed wall-clock budget to progress-based waiting; fifteen more in the same suites were out of its scope and remain the next cliff to trip: `saverPanePIDTimeout` (`cmd/bootstrap/orphan_sweep_integration_test.go:219`), `upgradePathPIDFileTimeout` (two sites), `daemonAliveTimeout` (`cmd/state_daemon_integration_test.go:273`), `daemonReadyBudget` / `hookCleanupObservationBudget`, and eight across `internal/tmux/portal_saver_integration_test.go`, `portal_saver_endstate_integration_test.go` and `kill_barrier_escalation_no_final_flush_integration_test.go`. Each polls a monotonic observable against a fixed wall-clock budget on a machine with no CI. `internal/tmuxtest/progress_test.go:55` carries the same class of assertion in the unit lane (`got.Elapsed > wait.Ceiling+300ms`), where it is also partly redundant with the two assertions above it.

**Solution**: Convert the fifteen fixed-budget waits to the progress-based wait, loosen the unit-lane elapsed assertion to a `2*wait.Ceiling` tolerance so it keeps catching a mis-derived ceiling without failing on scheduler noise, and move `AwaitProgress` and `PollUntil` into the neutral test leaf task 24 establishes. Derivation: the pair references nothing tmux-related while `internal/tmuxtest` is documented as real-tmux socket fixtures, and after this conversion three packages depend on it — so the misnomer is load-bearing rather than cosmetic. The staged objection was that moving is churn for a naming argument, which held only while no neutral home existed; task 24 creates one for `TestingT` and its recording fataller on the same reasoning, so the pair lands in a home that exists rather than justifying one of its own. This task is therefore ordered after task 24.

**Outcome**: No daemon-lifecycle wait in the corpus is decided by wall-clock alone, and the progress pair lives in a home that is not about tmux.

**Do**:
1. Move `AwaitProgress` (`internal/tmuxtest/progress.go`) and `PollUntil` (`internal/tmuxtest/poll.go`), with their tests, into the neutral test leaf task 24 establishes, and re-point every consumer; leave `internal/tmuxtest` holding tmux-specific scaffolding only.
2. Convert the fifteen fixed-budget waits to `AwaitProgress`, each observing the monotonic value it already polls: `cmd/bootstrap/orphan_sweep_integration_test.go:219` (`saverPanePIDTimeout`), the two `upgradePathPIDFileTimeout` sites, `cmd/state_daemon_integration_test.go:273` (`daemonAliveTimeout`), `daemonReadyBudget`, `hookCleanupObservationBudget`, and the eight across `internal/tmux/portal_saver_integration_test.go`, `portal_saver_endstate_integration_test.go` and `kill_barrier_escalation_no_final_flush_integration_test.go`.
3. Quote the returned `ProgressResult` in each converted site's failure message, so a red run says what the system was doing when the wait gave up.
4. Loosen the unit-lane elapsed assertion (`internal/tmuxtest/progress_test.go:55`, moving with the file) to a `2*wait.Ceiling` tolerance, and drop the part of it the two assertions above it already make.

**Acceptance Criteria**:
- [ ] None of the fifteen sites polls a monotonic observable against a fixed wall-clock budget.
- [ ] `AwaitProgress` and `PollUntil` are declared once, in the neutral leaf; `internal/tmuxtest` declares neither.
- [ ] Each converted failure message quotes the last observation.
- [ ] The elapsed assertion tolerates `2*Ceiling` and still fails a mis-derived ceiling.
- [ ] Both lanes pass, the integration lane serially (`-p 1`).

**Tests**:
- `"it reaches the target once the observation converges"`
- `"it reports the last observation when the wait stalls"`
- `"it tolerates scheduler noise up to twice the ceiling"`
- The fifteen converted suites keep their existing assertions; only the wait changes.

*Ordered after task 24, which creates the neutral leaf.*

## Task 16: Four `Commander` fakes outside `cmd` collapse the `Run`/`RunRaw` trim-versus-verbatim split
severity: medium
sources: bank

**Problem**: The `Commander` interface's whole reason for two methods is the trim-versus-verbatim split, and no test outside `cmd` pins it: `internal/tmux/tmux_test.go:29` records and dispatches `RunRaw` identically to `Run`, `internal/restore/session_test.go:33` does the same, `internal/restoretest/restore_marker_test.go:37` delegates `RunRaw` straight to `Run`, and `internal/state/capture_test.go:67` fatals on it (a deliberate stance, but a fourth statement of the contract). A production change to the split therefore lands silently in four packages. The executor who audited `cmd`'s seven fakes found the same absence there — the contract was not divided seven ways, it was absent seven times — and built `scriptedCommander` to honour it properly, homed in `cmd`.

**Solution**: Promote the honouring fake to a shared home beside `internal/transienttest`'s `Commander`, so one declaration pins the split for every package that fakes the interface, and re-point the four collapsed fakes at it.

**Outcome**: One fake pins the `Run`/`RunRaw` trim-versus-verbatim contract, so a production change to the split fails somewhere instead of passing in four packages.

**Do**:
1. Promote `cmd`'s `scriptedCommander` — the fake that honours the split — into a shared test home beside `internal/transienttest`'s `Commander`, exported, stdlib-only and untagged.
2. Give it a mode that fatals on `RunRaw`, so `internal/state`'s deliberate stance is a setting on the shared fake rather than a fourth statement of the contract.
3. Re-point the four collapsed fakes: `internal/tmux/tmux_test.go:29`, `internal/restore/session_test.go:33`, `internal/restoretest/restore_marker_test.go:37`, `internal/state/capture_test.go:67`.
4. Delete `cmd`'s local copy and re-point its consumers.

**Acceptance Criteria**:
- [ ] One declaration honours the split; the four local fakes are gone.
- [ ] `Run` returns trimmed output and `RunRaw` verbatim, in every package that fakes the interface.
- [ ] `internal/state`'s fatal-on-`RunRaw` stance survives as a mode, not a declaration.
- [ ] Both lanes pass.

**Tests**:
- Pure refactor: the four suites keep their existing assertions and semantics.
- The shared fake carries the contract's own coverage: `"it trims Run output"`, `"it returns RunRaw output verbatim"`, `"it fatals on RunRaw in strict mode"`.

## Task 17: `tmuxtest.WaitForSession` prefix-matches, so every fixture staging a prefix sibling gets a readiness guard that does nothing
severity: near-miss
sources: bank

**Problem**: `internal/tmuxtest/socket.go:110` polls `has-session -t <name>` with no `=` prefix. Measured: with only `_portal-saver-old` live, that exits 0 for `_portal-saver`. So every fixture in the repo that seeds a sibling before the session under test silently loses its settle-window guard — the same prefix-matching defect this work unit fixed in production, one layer down in the scaffolding, and it is why one task now carries a local `waitForExactSession` duplicate. Around it sit three near-identical isolated-server seed helpers in the same package (`seedSaverServer`, `seedPrefixSiblingServer`, `seedHookKeyServer` — each running the same `SkipIfNoTmux` → `tmuxtest.New` → `EnsureServer` → `NewSession` → wait sequence, differing only in socket prefix and topology) plus the matching `livePanePID` / `sessionPaneIDs` pair. `hookkey_realtmux_shared_test.go` already exists as the package's shared-fixture home.

**Solution**: Switch the shared helper to the exact form (`"="+name`) so the guard works for all consumers at once, delete the local exact-form duplicate, and collapse the three seed helpers into the package's existing shared-fixture file. Touching shared scaffolding used well beyond this package is the cost, and is why it was banked rather than taken inline.

**Outcome**: `WaitForSession` waits for the session it was named, so a fixture that seeds a prefix sibling gets the settle-window guard it asked for.

**Do**:
1. In `internal/tmuxtest/socket.go:112`, poll `has-session -t` with `tmux.ExactSessionTarget(name)` so a live `_portal-saver-old` cannot satisfy a wait for `_portal-saver`.
2. Delete the local `waitForExactSession` duplicate that exists only because the shared helper prefix-matched, and re-point its consumer.
3. Collapse `seedSaverServer`, `seedPrefixSiblingServer` and `seedHookKeyServer` into one parameterised seed helper in `hookkey_realtmux_shared_test.go` (socket prefix and topology as parameters), and fold the `livePanePID` / `sessionPaneIDs` pair in with them.
4. Run both lanes: this tightens a readiness guard used well beyond the package, so a fixture that was passing on a prefix match now waits for its own session.

**Acceptance Criteria**:
- [ ] `WaitForSession` does not return for a prefix sibling of the named session.
- [ ] The local exact-form duplicate is gone.
- [ ] The three seed helpers are one, in the package's shared-fixture file.
- [ ] Both lanes pass, with no fixture's timeout newly exhausted.

**Tests**:
- Pure refactor at the call sites; the shared helper carries the new coverage: `"it does not return for a prefix sibling of the named session"`, `"it returns once the exact session exists"`, `"it fails at its timeout when the session never appears"`.

## Task 18: The `list-panes` live-coord read is written nine ways, six of them prefix-unsafe
severity: near-miss
sources: bank

**Problem**: The same `list-panes -s -F "#{window_index}:#{pane_index}"` read is authored at nine sites across the restore corpus — `assertLivePanes` beside `livePaneCoords` in the *same package*, plus `integration_full_test.go`, `rename_reboot_shared_test.go`, `exit_closes_pane_integration_test.go`, `armed_restore_integration_test.go` and three sites in `cmd/bootstrap/reboot_roundtrip_test.go`. Only three route through `tmux.ExactCoordTarget`; the other six carry the latent prefix-sibling trap that helper exists to close, in assertions whose whole job is to prove the restore put panes where it said it did.

**Solution**: One `restoretest` reader collapses all nine and makes prefix-safety uniform rather than per-author.

**Outcome**: The live-coord read is written once and is prefix-safe everywhere, so an assertion about where restore put a pane cannot be answered by a stranger's session.

**Do**:
1. Add a `restoretest` reader that runs `list-panes -s -F "#{window_index}:#{pane_index}"` against `tmux.ExactCoordTarget(session)` and returns the parsed coords.
2. Re-point all nine sites: `assertLivePanes` and `livePaneCoords` in `internal/restore`, plus `integration_full_test.go`, `rename_reboot_shared_test.go`, `exit_closes_pane_integration_test.go`, `armed_restore_integration_test.go`, and the three sites in `cmd/bootstrap/reboot_roundtrip_test.go`.
3. Leave each site's own assertion where it is — only the read moves.

**Acceptance Criteria**:
- [ ] One reader performs the live-coord enumeration; no site composes the format string itself.
- [ ] Every consumer's target is prefix-safe.
- [ ] Each site's assertions are unchanged.
- [ ] The integration lane passes.

**Tests**:
- Pure refactor: the nine sites keep their assertions and semantics.
- The reader carries its own coverage: `"it reads the coords of a session's live panes"`, `"it does not read a prefix sibling's panes"`.

## Task 19: Two daemon fixtures assert the stale seed is absent without first asserting it was present
severity: near-miss
sources: bank

**Problem**: `cmd/state_daemon_hook_cleanup_test.go:81` and `cmd/state_daemon_run_test.go:571` assert the stale key is gone after the sweep with no prior presence check. A reviewer proved by mutation in an out-of-tree copy that re-pointing `hookstest.StaleHookSeed`'s stale half at an unjudgeable key leaves **both** fixtures passing, while re-pointing the live half fails both loudly — because the live assertion is a presence check and the stale one is not. Now that the seed body lives in another package, a future edit to `StaleHookSeed` can silently defang both; the integration suite already carries the guard these two lack. Three related gaps in the same vocabulary: the two suites' single-entry seeds still say `cmd-stale` where the shared two-entry seed says `cmd-gone`, so the same concept has two spellings in one file; hand-rolled key literals (`aaa111`, `tok999`, `ghost9`) in four `cmd` suites sit outside the vocabulary and — unlike constructor-derived seeds, which panic if the pane-token width moves — would quietly become *unjudgeable*, flipping the class of every assertion built on them; and `internal/hookstest/hooks_test.go` still sweeps `for n := range 4` for its token-shape subtests while the named seeds now span indices 0-6.

**Solution**: Add the presence precondition to both fixtures, reconcile the two stale-command spellings, route the hand-rolled literals through the `hookstest` constructors so a width move stays loud, and extend the self-test's sweep to the range the vocabulary actually mints. Consider unexporting `ReapableHookKey` / `UnjudgeableHookKey` in the same pass — they have no call sites outside the package and its own self-test, so making the named seeds the only exported vocabulary would turn "no package re-derives a seed index inline" from currently-true into impossible.

**Outcome**: Re-pointing `hookstest.StaleHookSeed`'s stale half at an unjudgeable key fails both daemon fixtures loudly, and no `cmd` suite carries a hand-rolled hook key that a width move could silently reclassify.

**Do**:
1. Add the presence precondition to both fixtures — assert the stale key is present in the staged `hooks.json` before the sweep runs — at `cmd/state_daemon_hook_cleanup_test.go:81` and `cmd/state_daemon_run_test.go:571`.
2. Reconcile the two stale-command spellings: use the shared seed's `cmd-gone` in both single-entry seeds so one concept has one spelling in the file.
3. Route the hand-rolled key literals (`aaa111`, `tok999`, `ghost9` and the fourth site) through the `hookstest` constructors or the named seeds, so a pane-token width move panics rather than turning a reap fixture into a retention one.
4. Extend `internal/hookstest/hooks_test.go`'s `for n := range 4` token-shape sweep to the full index range the named seeds mint (0-6).
5. Unexport `ReapableHookKey` / `UnjudgeableHookKey` once nothing outside the package constructs one, leaving the named seeds as the exported vocabulary.

**Acceptance Criteria**:
- [ ] Mutating `StaleHookSeed`'s stale half to an unjudgeable key fails both fixtures (verified by hand, then reverted).
- [ ] No `cmd` suite hand-rolls a hook-key literal.
- [ ] One spelling for the stale command across the two suites and the shared seed.
- [ ] The `hookstest` self-test covers every index the named vocabulary mints.

**Tests**:
- Refactor plus the missing precondition; no existing assertion loses coverage.
- `"it seeds the stale key before the sweep runs"` — both daemon fixtures
- `"it mints a token-shaped key for every named seed index"`

## Task 20: The Go-source guard scan skeleton is re-authored at fourteen sites; two packages independently extracted their own local version
severity: duplication
sources: duplication, bank

**Problem**: `internal/sourceguardtest` owns the enumeration primitives (`GoSourceFiles`, `PackageGoFiles`) and the AST primitives (`ForEachFuncCall`, `CalleeName`) but stops short of the step between them, so every consumer writes the same ten-line loop itself: enumerate → `parser.ParseFile` per path → `t.Fatalf("parse %s: %v")` → count what was scanned → fatal when the count is zero. Eleven sites write it inline (across `cmd`, `internal/hooks`, `internal/theme`, `internal/tmux`, `internal/restoretest`, `internal/portaltest`), and three more carry variants (`internal/session/panetoken_test.go`, `internal/tmux/target_composition_guard_test.go`, `cmd/open_theme_nomination_test.go`). Two packages recognised the repetition and each extracted a private version — `internal/hooks`'s `scanPackageCalls` and `internal/theme`'s `parseThemeSources` — the same abstraction discovered twice and shared zero times. The copies have already drifted where it matters: the parse mode varies (`SkipObjectResolution` at most sites, `ImportsOnly|SkipObjectResolution` at one, bare `0` at three), and the scanned-nothing tripwire — the property that stops a guard passing by having stopped looking — is present at some sites and absent at others. A guard that silently scans nothing reports a safety it is not providing, and that invariant is currently a per-copy decision.

**Solution**: Move the parse step into `internal/sourceguardtest` beside the primitives it already owns — a `ParsedSource{Path, Fset, File}` value plus `ParsePackageSources(t, dir, includeTests)` and `ParseSources(t, paths)` that fatal both on an unparseable file and on an empty result, taking the existing `TestingT` subset so their own failure paths stay testable, with the parse mode expressed rather than assumed. Re-point all fourteen sites, delete `scanPackageCalls` and `parseThemeSources`, and re-aim `scanPackageCalls`'s coverage test at the shared helper — asserting *which* fatal it exercises, which that test currently records and never reads.

**Outcome**: Every source guard in the tree parses through one helper that cannot pass over an unparseable file or an empty scan, with one stated parse mode.

**Do**:
1. Add to `internal/sourceguardtest`, beside `PackageGoFiles` and `ForEachFuncCall`: a `ParsedSource{Path, Fset, File}` value, `ParsePackageSources(t TestingT, dir string, includeTests bool) []ParsedSource` and `ParseSources(t TestingT, paths []string) []ParsedSource`, both fatal on an unparseable file and on an empty result, taking the existing `TestingT` subset so their own failure paths stay testable, with the parse mode a stated constant rather than a per-caller assumption.
2. Re-point the eleven inline loops: `cmd/hooks_pane_token_width_guard_test.go:25`, `cmd/run_hook_stale_cleanup_decline_error_guard_test.go:17`, `cmd/doctor_stand_down_phrase_guard_test.go:96`, `cmd/deps_seam_guard_test.go:31` and `:100`, `internal/hooks/cleanstale_staleness_guard_test.go:63`, `internal/hooks/leaf_guard_test.go:34`, `internal/theme/leaf_guard_test.go:137`, `internal/tmux/target_composition_guard_test.go:179`, `internal/restoretest/orchestrator_literal_guard_test.go:148`, `internal/portaltest/teardown_guard_coverage_test.go:70`.
3. Re-point the three variants too: `internal/session/panetoken_test.go`, `internal/tmux/target_composition_guard_test.go` and `cmd/open_theme_nomination_test.go`.
4. Delete `internal/hooks`'s `scanPackageCalls` and `internal/theme`'s `parseThemeSources`, and re-aim the former's coverage test at the shared helper, asserting *which* fatal it exercised rather than recording it unread.

**Acceptance Criteria**:
- [ ] No guard writes its own enumerate-parse-count loop; all fourteen route through the shared helper.
- [ ] Both private extractions are gone.
- [ ] A guard whose scan yields nothing fatals, at every site.
- [ ] One parse mode across the tree, expressed at the helper.

**Tests**:
- Refactor at the fourteen sites: each guard keeps its predicate, its rationale and its verdict.
- The helper carries the invariants: `"it fatals on an unparseable file"`, `"it fatals when the package yields no source"`, `"it returns one ParsedSource per file"`, `"it includes test sources only when asked"`.

## Task 21: Four callee-name unwrappers survive beside the shared `CalleeName`
severity: duplication
sources: bank

**Problem**: `sourceguardtest.CalleeName` is the shared Ident/Selector callee unwrapper, and four functional duplicates remain outside it: `cmd/state_daemon_lock_pid_ordering_test.go:233-243` (the same two-arm switch), `internal/theme/slug_collapse_guard_test.go:83-92` (`calledName`), `internal/tmux/target_composition_guard_test.go:365-377` (`isExactTargetCall`, which folds the unwrap into a set-membership test) and `internal/tui/restore_source_guard_test.go:199-206`. Two reviewers separately checked the remaining ~30 `Sel.Name` sites across the tree and found them receiver-qualified *matchers* rather than unwrappers, so these four are the complete candidate set.

**Solution**: Collapse all four onto `sourceguardtest.CalleeName`, keeping the set-membership test in the tmux guard as its own predicate over the shared unwrap.

**Outcome**: One callee-name unwrapper in the tree; a guard states its own vocabulary and nothing else.

**Do**:
1. Re-point `cmd/state_daemon_lock_pid_ordering_test.go:233-243`, `internal/theme/slug_collapse_guard_test.go:83-92` (`calledName`) and `internal/tui/restore_source_guard_test.go:199-206` at `sourceguardtest.CalleeName`, deleting the local copies.
2. In `internal/tmux/target_composition_guard_test.go:365-377`, split `isExactTargetCall` into `CalleeName` plus a set-membership predicate over `exactTargetHelpers`.
3. Leave the ~30 receiver-qualified `Sel.Name` sites alone — they are matchers, not unwrappers.

**Acceptance Criteria**:
- [ ] `sourceguardtest.CalleeName` is the only Ident/Selector callee unwrapper in the tree.
- [ ] The tmux guard's vocabulary check is a predicate over the shared unwrap.
- [ ] All four guards report the same findings as before on the real tree and on their staged probes.

**Tests**:
- Pure refactor: the four guards keep their fixtures, predicates and verdicts unchanged.

## Task 22: Four leaf-package guards each restate the "transitive deps confined to an allowlist" check, and its vacuity check survives in one copy of four
severity: duplication
sources: duplication, bank

**Problem**: `internal/hooks`, `internal/nanoid`, `internal/prefs` and `internal/theme` each assert the same property over `sourceguardtest.PackageDeps` — that a package's transitive dependency set stays inside a declared allowlist — and each writes the loop itself in a different shape: a `map[string]bool` in hooks, a forbidden-list plus inline switch in prefs, a single forbidden path in theme, an implicit stdlib predicate in nanoid. The failure verb differs (`Errorf` in two, `Fatalf` in two), so an offending dep aborts two guards and lets the other two carry on reporting. Most consequentially the **vacuity check has drifted**: only prefs asserts it still sees a known dep, so only one of the four cannot pass over an empty or unresolved dep set — the property the check exists to protect is present in one copy of four. A fifth hand-rolled `go list -deps` sits outside the family at `cmd/capturetool/import_guard_test.go:22`, which cannot fold in as things stand because it sets `cmd.Dir = portalbintest.ProjectRoot()` and `PackageDeps` has no equivalent knob; two reviewers surfaced it independently.

**Solution**: Add the optional working-directory knob and fold `cmd/capturetool/import_guard_test.go` in through it, leaving the four existing callers' resolution untouched. Derivation: the task's substance is `AssertDepsWithin` and the drifted vacuity check, which no side disputes; the only open part was the fifth copy. A module-root anchor changes how four working callers resolve their package argument to accommodate one, which is risk without benefit, and leaving the copy as a commented exception preserves the fifth restatement of the very duplication being consolidated — a comment does not stop drift, and the vacuity check is precisely what drifted. An additive knob folds all five onto one assertion with no change to what already works.

**Outcome**: One assertion carries the leaf-package dependency property for all five guards, with the vacuity check restored for every one of them.

**Do**:
1. Add `sourceguardtest.AssertDepsWithin(t, pkg string, allowed []string)` beside `PackageDeps`: skips the package itself, reports every dep outside the allowlist under one failure verb, and fatals when the resolved set is empty or holds no allowlisted internal dep.
2. Add an optional working-directory knob to the enumeration — a variadic option or a `PackageDepsIn(t, dir, pkg)` sibling — leaving `PackageDeps`'s existing signature and the four current callers' resolution untouched.
3. Reduce `internal/hooks`, `internal/nanoid`, `internal/prefs` and `internal/theme`'s guards to an allowlist plus their rationale; keep nanoid's stdlib-only predicate as its own allowlist form if a path list cannot express it.
4. Fold `cmd/capturetool/import_guard_test.go:22` onto the shared assertion through the knob (it needs `cmd.Dir = portalbintest.ProjectRoot()`), deleting its hand-rolled `go list -deps`.

**Acceptance Criteria**:
- [ ] Five guards assert through one shared function; none writes the loop itself.
- [ ] Every guard fails on an empty or unresolved dep set.
- [ ] An offending dep is reported by all five, under one failure verb.
- [ ] The four existing callers resolve their package argument exactly as before, and capturetool's guard still fails on a forbidden import.

**Tests**:
- Refactor at the five guards; the shared assertion carries the invariants: `"it reports every dep outside the allowlist"`, `"it fatals on an empty dep set"`, `"it fatals when no allowlisted internal dep is present"`, `"it resolves a package relative to the given working directory"`.

## Task 23: The `logtest` accessor family is one member short in three directions, and two capture-handler twins survive
severity: duplication
sources: bank

**Problem**: Every accessor `logtest.Record` does not offer is re-authored per package. There is no non-fatal string accessor, so `attrOrEmpty` (`internal/tmux/portal_saver_test.go:1152`, consumed from `hooks_register_test.go` — a generic record helper homed in the saver file) exists alongside raw `rec.Attrs[...]` reads at `cmd/state_daemon_cycle_summary_test.go`, `cmd/bootstrap/latch_test.go` and `cmd/bootstrap/eager_signal_hydrate_test.go`. There is no duration value accessor, so two hydrate suites assert the `took` value by hand and a third re-derives `RequireDuration` exactly. `Record.HasAttr` exists but three absence checks re-derive the index (`internal/storelog/clean_stale_test.go:33`, `internal/hooks/store_test.go:1094`, `internal/state/fifo_sweep_summary_test.go:133`). `Record.ErrorAttr` exists but four sites still hand-roll the index-then-type-assert, one of them in the lossy `gotErr, _ :=` form that silently yields nil on a wrong kind. And no level-filtered chain has a terminal single-record accessor — `OnlyRecord` applies no level filter — so the `if len(x) != 1 { t.Fatalf(...) }` idiom over a filtered `Records` appears at roughly twenty sites across four packages. Two structural twins of `Sink` itself also survive: `RecordingLogger` in `cmd/bootstrap` (exported, asserted against from a second file in that package) and `errorAttrRecorder` in `cmd/state_daemon_capture_logging_test.go`, which retains one WARN's live error attr — something `Sink` already does, since it stores `slog.Value`.

**Solution**: Finish the family — an `AttrOrEmpty`, a kind-checked `DurationAttr`, and a level-preserving `Only(t, description)` terminal on `Records` — then adopt them at the sites listed above, migrate the two surviving handler twins onto `Sink`, and re-home `attrOrEmpty`'s replacement out of the saver test file. Two tidies belong in the same pass: `cmd/logging_capture_test.go:31` re-open-codes the exact expression behind `log.Discard()`, and `barrierLog := &barrierLog{}` shadows its own type at nine sites in `internal/tmux/portal_saver_lifecycle_events_test.go`, making the type name unusable for the rest of each function.

**Outcome**: A suite asking a question of a captured record reaches for an accessor rather than writing one, and the two surviving capture-handler twins are gone.

**Do**:
1. Finish the family in `internal/logtest`: `Record.AttrOrEmpty(key) string` (non-fatal), `Record.DurationAttr(t, key) time.Duration` (kind-checked, beside `RequireDuration`), and a level-preserving terminal `Records.Only(t TestingT, description string) Record`.
2. Adopt them: delete `attrOrEmpty` (`internal/tmux/portal_saver_test.go:1152`) and re-home its consumer's call; replace the raw `rec.Attrs[...]` reads in `cmd/state_daemon_cycle_summary_test.go`, `cmd/bootstrap/latch_test.go` and `cmd/bootstrap/eager_signal_hydrate_test.go`; route the two hand-rolled `took` assertions and the re-derived `RequireDuration` through `DurationAttr`; replace the three re-derived absence checks (`internal/storelog/clean_stale_test.go:33`, `internal/hooks/store_test.go:1094`, `internal/state/fifo_sweep_summary_test.go:133`) with `HasAttr`; replace the four hand-rolled error-attr reads with `ErrorAttr`, including the lossy `gotErr, _ :=` form; and replace the ~20 `if len(x) != 1` idioms over a filtered `Records` with `Only`.
3. Migrate the two handler twins onto `Sink`: `RecordingLogger` in `cmd/bootstrap` (asserted against from a second file in that package) and `errorAttrRecorder` in `cmd/state_daemon_capture_logging_test.go`, whose live error attr `Sink` already retains as a `slog.Value`.
4. Two tidies: have `cmd/logging_capture_test.go:31` call `log.Discard()`, and rename the nine `barrierLog := &barrierLog{}` shadows in `internal/tmux/portal_saver_lifecycle_events_test.go`.

**Acceptance Criteria**:
- [ ] No suite re-authors a string, duration, absence, error or exactly-one accessor `logtest` now offers.
- [ ] `Sink` is the only capture handler in the tree; both twins are gone.
- [ ] The lossy `gotErr, _ :=` read is gone, and its site fails on a wrong-kind value.
- [ ] `log.Discard()` has no open-coded copy, and no `barrierLog` shadow remains.

**Tests**:
- Refactor at the adoption sites: assertions and semantics are unchanged.
- The three new accessors carry their own coverage: `"it returns the empty string for an absent attr"`, `"it fatals when the attr is not a duration"`, `"it fatals unless exactly one record matched"`.

## Task 24: The general-purpose `TestingT` subset has two homes, and four packages each carry their own recording fataller
severity: duplication
sources: architecture, bank

**Problem**: `TestingT` — the subset of `*testing.T` a fatal-on-failure helper needs, so the helper's own failure path is testable — is declared independently in `internal/logtest/capture.go:23` and `internal/sourceguardtest/packagedeps.go:11`. Neither declaration has anything to do with logging or with source scanning. The consequence is that `internal/hookstest`, whose whole subject is `hooks.json` bytes, imports the *logging* capture package for two byte-comparison helpers, and `cmd`'s source-scanning seam guard does the same for a function that parses Go files; the two copies can also drift in which methods they require. A third byte-identical declaration sits at `internal/restoretest/marker_count.go:19` (`markerReporter`). The matching stand-in — a recorder that absorbs a fatal by panic-and-recover so the failure path is assertable — now exists in four copies: `fakeT`/`captureFailure` (`internal/logtest/capture_test.go`), `recordingReporter`/`fatalSentinel` (`internal/restoretest/marker_count_test.go`), `fakeFataller` (`internal/restoretest/waitfor_file_exists_test.go`, which has no `Errorf` and so cannot substitute for the others) and `recordingT`/`captureAssert` (`internal/hookstest/hooks_test.go`, whose recover is the broken one). None is importable by the others because the originals live in `_test.go` files.

**Solution**: Home `TestingT` and one exported recording fataller in a new neutral test leaf (`internal/harnesstest`, following the repo's `<subject>test` naming), then re-point `logtest`, `sourceguardtest`, `restoretest` and `hookstest` at the single declaration. Derivation: `TestingT` is generic assertion scaffolding with no subject of its own, and both existing homes are special-purpose — `logtest` is log capture and `sourceguardtest` is documented as the Go-source scanning primitives the unit-lane guards share. Keeping it in either is the category error that produced the smell: `hookstest`, whose subject is `hooks.json` bytes, imports the logging package for two byte comparisons. Moving it to `sourceguardtest` repeats that error one package over. The repo already runs a family of purpose-named test leaves, so adding a neutral one is convention rather than novelty, and it gives the four recording-fataller copies — one of which has a broken recover — a single importable home outside `_test.go` files.

**Outcome**: `TestingT` and one recording fataller are declared once in a package whose subject is neither logging nor source scanning, and no test package imports a logging helper for a type unrelated to logging.

**Do**:
1. Create `internal/harnesstest` — stdlib-only, untagged, test-only, following the repo's `<subject>test` naming — holding `TestingT` (the union of the methods the two existing declarations require: `Helper`, `Errorf`, `Fatalf`) and one exported recording fataller that absorbs a fatal by panic-and-recover and records the message alongside `Errorf` calls.
2. Re-point `internal/logtest/capture.go:23`, `internal/sourceguardtest/packagedeps.go:11` and `internal/restoretest/marker_count.go:19` (`markerReporter`) at the single declaration, keeping a local alias only where a signature change would ripple.
3. Re-point `internal/hookstest` at it too, so `hooks.go`'s byte helpers stop depending on `internal/logtest`.
4. Delete the four recording-fataller copies — `fakeT`/`captureFailure`, `recordingReporter`/`fatalSentinel`, `fakeFataller` and `recordingT`/`captureAssert` (whose recover is the broken one) — and re-point their suites at the shared one.
5. Add a leaf guard over the new package modelled on `internal/nanoid/leaf_guard_test.go`.

**Acceptance Criteria**:
- [ ] One `TestingT` declaration in the tree; `logtest`, `sourceguardtest`, `restoretest` and `hookstest` reference it.
- [ ] `internal/hookstest` no longer imports `internal/logtest`.
- [ ] One importable recording fataller; the four copies are gone and the broken recover with them.
- [ ] The new package's transitive deps are stdlib-only, pinned by its guard.

**Tests**:
- Refactor at the re-pointed packages; their suites keep their assertions.
- The shared fataller carries its own coverage: `"it records the fatal message and stops the helper"`, `"it records an Errorf without stopping"`, `"it reports no failure for a passing helper"`.
- `"it confines internal/harnesstest to the standard library"`

## Task 25: The failed-write audit-trail assertion is hand-rolled at six sites whose own file-siblings already use the shared helper
severity: duplication
sources: duplication

**Problem**: `logtest.AssertRecord` / `RecordWant` exist to pin the five properties every audit-trail line shares, and the happy-path cases in all three store-logging suites were migrated onto it. The WARN-on-failed-write cases were not: `internal/hooks/store_test.go:1288` and `:1441`, `internal/project/store_logging_test.go:102`, `:219` and `:320`, and `internal/alias/store_logging_test.go:224` each still spell out the level, message, `op` and `component` checks inline, then repeat the same two-part tail — an `error_class` string check followed by `ErrorAttr` plus an `errors.Is` against a `fileutil` sentinel — at five more line numbers. One contract asserted two ways inside the same file, with the divergence invisible: the project suite's hand-rolled blocks assert no `via` at all, so the failed-write breadcrumb's `via` attr is unpinned on three of the four project mutations while every successful one pins it.

**Solution**: Route the six blocks through `logtest.AssertRecord` exactly as their siblings do, and lift the recurring tail into one shared assertion beside it — an `AssertWriteFailure(t, rec, wantClass, sentinel)` checking the `error_class` value and that the carried error wraps the named `fileutil` sentinel — so the failed-write contract lands in one place for hooks, projects and aliases alike.

**Outcome**: The failed-write breadcrumb is asserted the same way as its happy-path sibling, in one place, with `via` pinned on every mutation.

**Do**:
1. Add `logtest.AssertWriteFailure(t, rec, wantClass string, sentinel error)` beside `AssertRecord`: it checks the `error_class` attr's value and that `rec.ErrorAttr(t, "error")` wraps the named `fileutil` sentinel.
2. Route the six hand-rolled blocks through `AssertRecord` + `AssertWriteFailure`, exactly as their file-siblings already do: `internal/hooks/store_test.go:1288` and `:1441`, `internal/project/store_logging_test.go:102`, `:219` and `:320`, `internal/alias/store_logging_test.go:224`.
3. Pin `via` on the project suite's three failed-write cases, which the hand-rolled blocks left unasserted while every successful mutation pins it.
4. Delete the five repeated `error_class` + `errors.Is` tails at the line numbers the finding names.

**Acceptance Criteria**:
- [ ] No store-logging suite spells the five shared record properties inline.
- [ ] Every failed-write case pins `via`.
- [ ] One assertion carries the `error_class`-plus-sentinel tail for hooks, projects and aliases.
- [ ] All three suites pass with no coverage lost.

**Tests**:
- Refactor at the six sites, plus the newly-pinned `via` on the three project cases.
- The shared assertion carries its own coverage: `"it fails when the error_class attr differs"`, `"it fails when the carried error does not wrap the sentinel"`.

## Task 26: The hooks-store lock WARN is asserted by two independently written helpers, plus three inline restatements of its negative half
severity: duplication
sources: duplication

**Problem**: Both suites assert the same emission — the single WARN `hooks.Store.Set`/`Remove` leaves when the sidecar acquire times out — through two helpers written independently and named almost the same thing: `assertLockWarn(t, sink, wantOp, wantKey, wantVia)` (`internal/hooks/lock_write_test.go:18`, 26 lines hand-rolling level/msg/op/component/via/hook_key/error) and `assertOneLockWarn(t, sink, wantOp, wantKey)` (`cmd/hooks_write_lock_test.go:45`, which pins the same properties through `logtest.AssertRecord` and additionally requires exactly one WARN). Neither covers what the other adds. The negative half of the contract — that a lock WARN carries no `error_class` and no `value`, because no write phase ran — is then restated verbatim three more times as inline attr-presence checks. `internal/hookstest` is already the cross-package home for exactly this shape: it owns `AssertDegradedRead`, the sibling assertion for the DEBUG `load-unlocked` breadcrumb, alongside the `HoldHooksSidecar` fixture both suites use to produce the timeout.

**Solution**: Add `hookstest.AssertLockWarn(t, sink, op, key, via)` next to `AssertDegradedRead`, covering the whole line in one call — level, message, `op`, `component=hooks`, `via`, `hook_key`, a non-empty `error`, and the absence of `error_class` and `value` — and have both suites call it, deleting the two local helpers and the three inline tails. Keep `cmd`'s exactly-one-WARN check at its call site if that count is specific to the command path.

**Outcome**: One assertion covers the whole lock WARN — its positive half and its negative half — for both suites that produce it.

**Do**:
1. Add `hookstest.AssertLockWarn(t, sink, op, key, via string)` next to `AssertDegradedRead` in `internal/hookstest/hooks_lock.go`, covering the line in one call: WARN level, the message, `op`, `component=hooks`, `via`, `hook_key`, a non-empty `error`, and the absence of `error_class` and `value`.
2. Delete `assertLockWarn` (`internal/hooks/lock_write_test.go:18`) and `assertOneLockWarn` (`cmd/hooks_write_lock_test.go:45`), calling the shared one from both suites.
3. Delete the three inline restatements of the negative half (`internal/hooks/lock_write_test.go:185` and `:203`, `cmd/hooks_write_lock_test.go:61`).
4. Keep `cmd`'s exactly-one-WARN count at its call site, where the count is specific to the command path.

**Acceptance Criteria**:
- [ ] One helper asserts the lock WARN; both local helpers are gone.
- [ ] Both halves of the contract are covered for both suites — neither loses what the other's helper added.
- [ ] The negative half is stated once.
- [ ] `cmd` still requires exactly one WARN on its path.

**Tests**:
- Refactor at the two suites; their subjects and outcomes are unchanged.
- The shared assertion carries its own coverage: `"it fails when the WARN carries an error_class"`, `"it fails when the hook_key differs"`.

## Task 27: There are four `hooks.json` staging routes where the code claims two, and the path composition is hand-rolled fifty times
severity: duplication
sources: duplication, bank

**Problem**: `hookstest.StageStore` is the described route for handing a staged `hooks.json` to a seam, and `cmd/testhelpers_test.go:150` documents `hooksFileInTempDir` as "the second of the two staging routes". There are four: `StageStore`, `hooksFileInTempDir` (plus `writeHooksJSON`), and `seedHookStore` (`cmd/state_hydrate_test.go:1234`) — a private marshal-and-write driven from roughly nineteen call sites across three hydrate suites, which stages **no sidecar**, so every hydrate fixture's `LookupOnResume` takes the degraded unlocked-read path while a `StageStore` fixture takes the shared lock. The doc comment asserting there are two routes is already false, which is how the drift stayed invisible. Underneath, the path composition itself is inline: 36 `filepath.Join(dir, "hooks.json")` sites in `internal/hooks/store_test.go` (8 paired with an `os.WriteFile` seed that `Staging{Seed}` now expresses) and 12 in `lock_write_test.go`, whose subject is genuinely the sidecar's absence and its creation, so they want a path-only sibling rather than the full stager. `internal/hookstest`'s own self-test hand-rolls two of the axes its package owns. A fifth, smaller instance of the same reach: `readFileBytes` (`cmd/testhelpers_test.go:114`) re-implements `hookstest.HooksFileBytes` — the same ENOENT-tolerant read, already path-generic and already taking the `TestingT` subset — in a file that imports `hookstest` and delegates its neighbouring assertion to it.

**Solution**: Give the package one stager and one path-only sibling, fold `seedHookStore` on with an explicit sidecar-absent option for the hydrate suites, fold the inline path compositions onto the sibling, delegate `readFileBytes` to the shared read, and make the "two routes" claim in `cmd/testhelpers_test.go` true again. Derivation: the sidecar is absent on every install in the wild until its first mutation acquire, so the degraded unlocked read is the common production shape rather than an edge case — staging a sidecar everywhere would move every hydrate fixture off the path most installs actually take, and would drop a sidecar into fixture dirs that also hold FIFOs and scrollback. Leaving `seedHookStore` alone keeps the fourth route, which is the drift being consolidated. An explicit option preserves today's behaviour and turns the absence from an accident of a private helper into a stated property of the fixture.

**Outcome**: Two staging routes exist and the code says two: one stager and one path-only sibling, with each hydrate fixture's sidecar absence a stated property rather than an accident.

**Do**:
1. Add a path-only sibling to `internal/hookstest` — a `HooksPath(t, dir) string` that joins and nothing more — and fold the inline path compositions onto it: 36 sites in `internal/hooks/store_test.go` and 12 in `lock_write_test.go`, whose subject is genuinely the sidecar's absence and creation.
2. Route the eight `store_test.go` path-plus-`os.WriteFile` pairs through `StageStore`'s `Staging{Seed}` instead, which now expresses them.
3. Extend `Staging` to seed the multi-event `map[string]map[string]string` body (a sibling field beside `Entries`), then fold `seedHookStore` (`cmd/state_hydrate_test.go:1234`) onto `StageStore` with `SidecarAbsent: true` at its ~19 call sites across the three hydrate suites — preserving today's degraded unlocked read, which is the shape most installs actually take.
4. Delegate `readFileBytes` (`cmd/testhelpers_test.go:114`) to `hookstest.HooksFileBytes`, keeping one pinning test over the survivor, and correct the "second of the two staging routes" comment so the claim is true.
5. Fix `internal/hookstest`'s own self-test where it hand-rolls two of the axes the package owns.

**Acceptance Criteria**:
- [ ] Two routes: the stager and the path-only sibling. `seedHookStore` and `writeHooksJSON` are gone.
- [ ] Every hydrate fixture asks for the sidecar's absence explicitly and still takes the degraded read (its `load-unlocked` breadcrumbs are unchanged).
- [ ] No inline `filepath.Join(dir, "hooks.json")` remains in the two `internal/hooks` suites.
- [ ] The ENOENT-tolerant read has one implementation, and the doc comment's route count matches the tree.

**Tests**:
- Refactor at the call sites: every fixture's staged bytes and read path are unchanged.
- The stager carries the new axes: `"it stages a multi-event hooks body"`, `"it stages no sidecar when asked"`, `"it returns the hooks path without staging a file"`.

## Task 28: The shell-quoting rule has three separately-maintained homes
severity: duplication
sources: duplication

**Problem**: `internal/restore/session.go:362`'s `shellQuoteSingle` and `internal/spawn/recipe.go:67`'s `shellQuote` are the same function to the byte — `return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"` — introduced independently in different packages, and `internal/session/create.go:29` performs the same close-escape-reopen a third time inline without wrapping it at all. This is a correctness rule, not a formatting convenience: all three compose a string a shell will later word-split (a tmux `respawn-pane` command, a `{command}` handed to a terminal recipe, a tmux shell-command), and the project's own architecture notes already record that naive concatenation on the spawn path corrupts the command. Three copies is three places for it to be fixed once and stay wrong twice.

**Solution**: Put the rule in a small stdlib-only leaf in the same mould as `nanoid`, holding the quote function and the `renderCommandString` join if that moves with it, then delete the two helpers and have `BuildShellCommand` call it rather than inlining the replace. Derivation: this is the shape the repo already uses for a rule that packages which must not import each other all need — `nanoid` is documented as a leaf precisely so `hooks`, `session` and `spawn` can each reach the vocabulary without an edge between them, and this is the same three-way problem with the same three packages. Exporting from `internal/spawn` would make `internal/restore` and `internal/session` depend on the host-terminal spawn package for a shell-quoting rule, which is both a category error and a new edge between packages that have none today.

**Outcome**: One home for the shell single-quoting rule, reachable by `restore`, `spawn` and `session` without an edge between them.

**Do**:
1. Add a stdlib-only leaf in the mould of `internal/nanoid` — `internal/shellquote` — exporting the single-quote rule (`Single(s string) string`), and move `renderCommandString`'s join with it if the join belongs beside the rule.
2. Delete `shellQuoteSingle` (`internal/restore/session.go:362`) and `shellQuote` (`internal/spawn/recipe.go:67`), re-pointing both packages at the leaf.
3. Have `session.BuildShellCommand` (`internal/session/create.go:29`) call the leaf rather than inlining the close-escape-reopen.
4. Add a leaf guard pinning the package's stdlib-only dependency set, modelled on `internal/nanoid/leaf_guard_test.go`.

**Acceptance Criteria**:
- [ ] One declaration of the quoting rule; all three sites call it.
- [ ] No new import edge between `internal/restore`, `internal/session` and `internal/spawn`.
- [ ] The leaf's transitive deps are stdlib-only.
- [ ] Every composed command is byte-identical to today's.

**Tests**:
- Refactor: the existing spawn recipe, restore respawn and session exec-chain argv assertions pass byte-identical.
- The leaf carries the rule's coverage: `"it wraps a plain string in single quotes"`, `"it re-quotes an embedded single quote"`, `"it quotes an empty string"`.

## Task 29: Two inline copies of a stand-down phrase bypass the vocabulary that exists to own it
severity: duplication
sources: duplication, bank

**Problem**: `cmd/run_hook_stale_cleanup.go` declares two surface vocabularies for stand-down copy, `skippedPrunePhrases` and `notEvaluableDetails`, and `restoreStandDownPhrase` was extracted as a const precisely because one phrase was shared between them. Its siblings were not: `"could not read hooks.json"` is authored in both maps and `"could not enumerate live panes"` likewise. Worse, `checkStaleHooks` re-authors the first a third and fourth time as bare literals in its two early returns (`cmd/doctor.go:295` for a nil store, `:299` for a failed `Load`) rather than rendering through `phraseFor(notEvaluableDetails, skipReasonStoreReadFailed)` the way its later returns do. Two guards cover the vocabularies and the rendered lines are pinned, but neither can see a literal that duplicates a map *value* — the guards bind map *keys* — so re-wording the entry leaves those two branches printing the old copy for the same condition, silently. (The literals arrived from a different work unit, which is why no task in this one owned them.)

**Solution**: Have both early returns render through `phraseFor`, and lift the two strings shared between the maps into consts beside `restoreStandDownPhrase` so each phrase is written once and both maps compose from them.

**Outcome**: Re-wording a stand-down phrase changes every branch that prints it, including `checkStaleHooks`'s two early returns.

**Do**:
1. Lift the two strings shared by `skippedPrunePhrases` and `notEvaluableDetails` into consts beside `restoreStandDownPhrase` (`cmd/run_hook_stale_cleanup.go:34`) — `"could not read hooks.json"` and `"could not enumerate live panes"` — and compose both maps from them.
2. Have `checkStaleHooks`'s two early returns render through `phraseFor(notEvaluableDetails, skipReasonStoreReadFailed)`: the nil-store branch (`cmd/doctor.go:295`) and the failed-`Load` branch (`:299`), as its later returns already do.
3. Leave the rendered output byte-identical.

**Acceptance Criteria**:
- [ ] Neither vocabulary map holds a string literal that its sibling also holds.
- [ ] No branch of `checkStaleHooks` authors a stand-down phrase inline.
- [ ] Re-wording a map entry changes both early returns' output.
- [ ] Every rendered line is byte-identical to today's.

**Tests**:
- Refactor: the existing copy tests pin the same strings.
- One assertion the guards cannot make today: `"it renders the nil-store and failed-load branches from the vocabulary"` — re-word the entry in the test and assert both branches follow.

## Task 30: `cmd` carries a second seam family — nine package-level function vars installed by hand at 113 sites
severity: duplication
sources: bank

**Problem**: `runOpenBurstFunc`, `openRawArgs`, `openTUIFunc`, `openPathFunc`, `openSessionFunc`, `daemonTickLoopFunc`, `osExit`, `hydrateRunFunc` and `signalHydrateRunFunc` are installed across 21 test files at roughly 113 sites, each a bare assignment plus a separate `t.Cleanup` — exactly the pattern the `*Deps` seam helper closed for the struct seams, and exactly the same leak vector when a cleanup is forgotten. They are invisible to the new seam guard because `declaredSeams` matches only `var xDeps *XDeps`. The shape differs in one way that matters: the production default is a real value rather than nil, so the helper must capture-and-restore rather than restore-to-nil, and the guard's recogniser needs a second arm.

**Solution**: Give the function-var family the same install helper and the same derived guard the struct seams got, so a seam declared tomorrow is guarded the day it appears and a forgotten restore cannot leak into a later test.

**Outcome**: A function-var seam is installed and restored in one call, and a direct assignment to one fails the guard the way a direct `*Deps` assignment already does.

**Do**:
1. Add an install helper to `cmd/testhelpers_test.go` for the function-var family that captures the current value, installs the replacement and registers the `t.Cleanup` restore in the same breath — capture-and-restore, not restore-to-nil, since the production default is a real value.
2. Re-point the ~113 sites across the 21 test files covering `runOpenBurstFunc`, `openRawArgs`, `openTUIFunc`, `openPathFunc`, `openSessionFunc`, `daemonTickLoopFunc`, `osExit`, `hydrateRunFunc` and `signalHydrateRunFunc`.
3. Give `cmd/deps_seam_guard_test.go`'s `declaredSeams` a second arm recognising a package-level function-var seam in the production sources, so the family is derived rather than listed.
4. Fail any `*_test.go` assigning one of those vars directly, as the struct seams already are, and cover the new arm with a staged probe.

**Acceptance Criteria**:
- [ ] No test file assigns a function-var seam directly.
- [ ] The guard derives the family from the production declarations, so a seam declared tomorrow is covered without editing a list.
- [ ] Every install restores the captured production default, not nil.
- [ ] The guard fails on a staged direct assignment and passes on the tree.

**Tests**:
- `"it restores the captured production default after the test"`
- `"it flags a direct assignment to a function-var seam"`
- `"it derives the function-var seam set from the production sources"`
- The ~113 re-pointed sites keep their existing assertions.

## Task 31: The stand-down copy is pinned in four files and the same case is written twice in two more
severity: duplication
sources: bank

**Problem**: `cmd/doctor_stand_down_copy_test.go` was meant to be the one place the stand-down copy is pinned, and it is not: `:142-145` duplicates `cmd/doctor_test.go:1568-1570` exactly (same helper, same lister, same expected string), `:138-140` duplicates `cmd/run_hook_stale_cleanup_outcome_test.go:93-99` in substance, and two more of the same lines are pinned in the lock-timeout and single-report suites — so a wording change needs four edits. The surrounding corpus repeats itself the same way: `TestHookSweepOutcomeNamesEveryDecline` tables all five decline reasons with a uniqueness check while `run_hook_stale_cleanup_test.go` separately pins four of the same five one at a time and the lock-timeout suite pins the fifth, each half carrying assertions the other lacks; the lock-timeout single-report case exists twice in two files from two tasks with the stronger assertions split between them; and one new doctor-print subtest duplicates a pre-existing one outright, staging the identical fixture, driving the identical command and making the identical assertion. Two smaller notes belong with the pass: `bogusHooksStore` lives in an outcome test file while `testhelpers_test.go` declares itself the home for staging helpers and is now consumed from three files, and `cmd/doctor_test.go:869`'s projects read has no existence precondition — not vacuous today only because the next assertion is positive, and it would fail as "live project wrongly pruned", misattributing a deleted file.

**Solution**: Give each decline reason one home carrying both halves of its coverage — the log level and attrs *and* the rendered line *and* the hooks.json-untouched check — delete the outright duplicate, fold the stronger of each split pair into the survivor, and move the staging helper to the file that claims it. One `borrows` check needs correcting while the file is open: it compares the *table's* expected values rather than the rendered ones, so it catches a copy-paste author rather than two production reasons converging on one phrase (proven: re-pointing a map entry at another reason's phrase was caught by the exact-line assertions and not by that check).

**Outcome**: Each decline reason has one home carrying its whole coverage, so a wording change is one edit and no assertion is split across two files.

**Do**:
1. Give each of the five decline reasons one home carrying all three halves of its coverage — the log level and attrs, the rendered line, and the hooks.json-untouched check — and delete the duplicates: `cmd/doctor_stand_down_copy_test.go:142-145` against `cmd/doctor_test.go:1568-1570`, `:138-140` against `cmd/run_hook_stale_cleanup_outcome_test.go:93-99`, and the lock-timeout and single-report pins in the two further files.
2. Fold the stronger half of each split pair into the survivor: `TestHookSweepOutcomeNamesEveryDecline`'s uniqueness check alongside the one-at-a-time suites' extra assertions, and the lock-timeout single-report case's stronger assertions into the one that survives.
3. Delete the new doctor-print subtest that duplicates a pre-existing one outright (identical fixture, command and assertion).
4. Move `bogusHooksStore` into `cmd/testhelpers_test.go`, which declares itself the home for staging helpers and is now consumed from three files.
5. Correct the `borrows` check to compare the *rendered* phrases rather than the table's expected values, and add an existence precondition to `cmd/doctor_test.go:869`'s projects read so a deleted file cannot be misreported as a wrongly-pruned live project.

**Acceptance Criteria**:
- [ ] A wording change to any stand-down phrase requires one edit.
- [ ] Every decline reason's home carries level+attrs, the rendered line and the file-untouched check.
- [ ] The `borrows` check fails when two production reasons render the same phrase (verified by re-pointing a map entry, then reverting).
- [ ] `bogusHooksStore` lives in the staging-helper file, and no coverage is lost anywhere in the collapse.

**Tests**:
- Consolidation: every surviving assertion already exists somewhere in the corpus.
- `"it fails when two decline reasons render the same phrase"`
- `"it asserts the seeded project exists before the prune"`

## Task 32: `internal/restore` fixture residue from this phase
severity: duplication
sources: bank

**Problem**: Parameterising the multipane arrange left `newLegacyFixture` and `newRenameRebootFixture` as the same fixture with a pane loop as the only difference — identical struct shape, identical `captureAndPersist` / `rebootAndHydrate` method names and bodies. The convergence was created by that task and left unresolved by it because merging reaches into three files it did not scope. Separately, `openTestLogger(t, dir)` in `restore_test.go` survives as a pure pass-through whose `dir` argument nothing reads, left behind when its consumers were routed at `logtest.NewCaptureLogger` directly.

**Solution**: Merge the two fixtures into one taking a pane count, and drop the parameter nothing reads.

**Outcome**: One restore fixture serves both suites, and no helper takes an argument nothing reads.

**Do**:
1. Merge `newLegacyFixture` and `newRenameRebootFixture` into one fixture taking a pane count, keeping the shared `captureAndPersist` and `rebootAndHydrate` methods, and re-point the three files that construct them.
2. Drop `openTestLogger`'s unread `dir` parameter (`internal/restore/restore_test.go`), or delete the pass-through outright and have its consumers call `logtest.NewCaptureLogger` directly.

**Acceptance Criteria**:
- [ ] One fixture type, parameterised by pane count; both former constructors are gone.
- [ ] No helper in the package takes an argument nothing reads.
- [ ] The integration lane passes with both suites' assertions unchanged.

**Tests**:
- Pure refactor: both suites keep their subjects, assertions and semantics.

## Task 33: Four command-driver twins and four call-log filters in one package
severity: duplication
sources: bank

**Problem**: `runStateCommitNow`, `runStateDaemon` and `runStateNotify` are three literally identical twelve-line bodies — same `t.Helper()`, two buffers, `resetRootCmd()` + `resetStateCmdFlags()`, `SetOut`/`SetErr`, `Execute` — differing only in the second argv element; `runUninstall` is the same body again in the variadic form, minus the state-flag reset. A single `runRootCmd(t, args ...string)` absorbs all four, with `resetStateCmdFlags()` the only genuine divergence and a candidate to fold into `resetRootCmd` alongside the flag resets already there. Beside them, four parallel filters over `[][]string` call logs coexist in the same package: `countOp`, `callIndex`, `setHookCalls` and `callsMatching` — the last of which currently has no consumer outside its own contract test, which the other three make plausible.

**Solution**: Collapse the four drivers onto one and the four filters onto one, which resolves the unconsumed helper at the same time.

**Outcome**: One driver executes a root command in a test and one filter queries a call log, so neither shape is re-authored for the next command.

**Do**:
1. Replace `runStateCommitNow`, `runStateDaemon`, `runStateNotify` and `runUninstall` with a single `runRootCmd(t *testing.T, args ...string)` carrying the shared body (two buffers, reset, `SetOut`/`SetErr`, `Execute`), and re-point every call site.
2. Fold `resetStateCmdFlags()` into `resetRootCmd()` alongside the flag resets already there, so the one driver needs no per-command divergence.
3. Collapse `countOp`, `callIndex`, `setHookCalls` and `callsMatching` onto one filter over `[][]string` call logs and re-point their consumers, which resolves `callsMatching`'s lack of a consumer at the same time.

**Acceptance Criteria**:
- [ ] One driver runs all four commands; the four twins are gone.
- [ ] `resetRootCmd` resets the state command's flags, so no caller sequences two resets.
- [ ] One call-log filter serves every query the four covered.
- [ ] No helper in the package is left without a consumer.

**Tests**:
- Pure refactor: every re-pointed test keeps its arguments, assertions and semantics.

## Task 34: The `hook rm` test corpus carries cross-file duplicates and one staging shape written three ways
severity: duplication
sources: bank

**Problem**: `cmd/hooks_test.go:617` and `cmd/hooks_rm_exit_test.go:149` are the same test with different key names — both seed a target plus a sibling, resolve the target, and assert removed-plus-sibling-survives; `cmd/hooks_test.go:497` is a strict subset of `:617`; and `cmd/hooks_test.go:590` and `cmd/hooks_rm_exit_test.go:116` both pin the no-hook-registered message on the resolved path. Two assertions in that set are genuinely unique and must survive any collapse: the raw-`%42`-not-used-as-key check and the last-event cleanup that asserts the file empties. Separately, both subtests of `TestHookRmLockTimeout` hand-roll the staging sequence `runRmCase` now owns, differing only in holding the sidecar lock and needing the captured stdout. And the `rmCase` row struct has two field combinations that are silently meaningless or panic — a row setting the pane-key path *and* a resolver silently discards the resolver, and a row setting neither passes a typed-nil through the interface, panicking inside the mock rather than falling back to the production default.

**Solution**: Collapse the duplicated cases into one home each, preserving the two unique assertions; give `rmCase` a lock-hold flag and `rmOutcome` a captured-output field so the lock-timeout subtests route through it; and close the two meaningless field combinations so the struct cannot be filled in a way that means nothing.

**Outcome**: Each `hook rm` behaviour is pinned in one place, every lock-timeout subtest runs through the shared case runner, and a `rmCase` row cannot be filled in a way that means nothing.

**Do**:
1. Collapse the cross-file duplicates into one home each, preserving the two unique assertions — the raw-`%42`-not-used-as-key check and the last-event cleanup that asserts the file empties: `cmd/hooks_test.go:617` with `cmd/hooks_rm_exit_test.go:149`, `cmd/hooks_test.go:497` (a strict subset of `:617`), and `cmd/hooks_test.go:590` with `cmd/hooks_rm_exit_test.go:116`.
2. Give `rmCase` a lock-hold flag and `rmOutcome` a captured-output field, and route both `TestHookRmLockTimeout` subtests through `runRmCase`, deleting their hand-rolled staging.
3. Reject a row that sets both the pane-key path and a resolver, so the silently-discarded resolver cannot be written.
4. Give a row that sets neither the production default rather than passing a typed-nil through the interface, so the mock cannot panic on a half-filled row.

**Acceptance Criteria**:
- [ ] Each duplicated behaviour is pinned once; the two unique assertions survive.
- [ ] Both lock-timeout subtests run through `runRmCase`.
- [ ] A row naming both a pane key and a resolver fails loudly.
- [ ] A row naming neither runs against the production default without panicking.

**Tests**:
- Consolidation: no surviving assertion is new.
- `"it rejects a case naming both a pane key and a resolver"`
- `"it falls back to the production resolver for a case naming neither"`

## Task 35: Each config file's identity is restated at every call site, and two production sites restate the resolution rule
severity: duplication
sources: bank

**Problem**: The precedence rule now has one home in `internal/xdg`, but the pair that *identifies* each config file — its env var and its filename — is still written out per site: `cmd/hooks.go:256` passes the literals, `cmd/config.go` re-lists the filenames in `configFileComponents`, and `internal/hookstest/hooks.go` declares its own copy of the same pair. Renaming an env var is therefore an N-site edit that reaches into a test-only package. Separately, two production sites re-author the shape the rule single-sourced for config *files*: `internal/state/paths.go:28-35` (`PORTAL_STATE_DIR` then `<base>/portal/state`) and `cmd/config.go:208-217` (`PORTAL_THEMES_DIR` then `<base>/portal/themes`), so the `portal/` literal now sits in three production sites.

**Solution**: Decide where a config file's identity lives — an exported table beside `configFileComponents`, or per-owning-package constants — and route `cmd/alias.go`, `cmd/hooks.go`, `cmd/config.go`, `internal/hookstest` and `internal/restoretest` through it. Bring the state-dir and themes-dir resolvers onto the shared rule too, respecting the themes directory's deliberate carve-out (a `_DIR` rather than a `_FILE`, with no migration).

**Outcome**: A config file's env var and filename are declared once, and the `portal/` path segment lives in one production place.

**Do**:
1. Declare each config file's identity once — an exported table beside `configFileComponents` in `cmd/config.go` holding `{EnvVar, Filename, Component}` per file (`projects.json`, `aliases`, `hooks.json`, `prefs.json`, `terminals.json`).
2. Route the literal-passing sites through it: `cmd/hooks.go:256`, `cmd/alias.go`, `cmd/config.go`'s own filename list, and the test-only copies in `internal/hookstest/hooks.go:39-43` and `internal/restoretest`.
3. Bring the two directory resolvers onto `internal/xdg`'s rule — `internal/state/paths.go:28-35` (`PORTAL_STATE_DIR` then `<base>/portal/state`) and `cmd/config.go:208-217` (`PORTAL_THEMES_DIR` then `<base>/portal/themes`) — adding a directory-shaped entry point beside `ConfigFilePath` so the `portal/` literal is written once.
4. Respect the carve-outs: the themes directory keeps no migration and no `configFileComponents` entry, both resolvers still create nothing, and `internal/xdg` stays a leaf.

**Acceptance Criteria**:
- [ ] Renaming a config file's env var is one edit, reaching `cmd` and the two test-only packages.
- [ ] `internal/hookstest` resolves `hooks.json` from the shared identity rather than its own pair.
- [ ] The `portal/` path segment appears in one production place.
- [ ] Themes-dir resolution still runs no migration and emits no breadcrumb; state-dir resolution is behaviourally unchanged.

**Tests**:
- Refactor with one identity move; resolution outcomes are unchanged.
- `"it resolves hooks.json from the shared file identity"`
- `"it resolves the themes dir with no migration"`
- `"it resolves the state dir through the shared config base"`

## Task 36: The declared-once hydrate-wait invariant is half-applied
severity: drift
sources: bank

**Problem**: `restoretest.HydrateBudget` / `HydrateTick` exist so the hydrate wait is declared once, and three families sit outside it. Two holdouts in `internal/restore` still pass the raw `10*time.Second, 50*time.Millisecond` pair (`integration_full_test.go:128`, `exit_closes_pane_integration_test.go:169`). The `cmd/bootstrap` hook-fire family uses a deliberately shorter `2s`/`50ms` pair at five sites, which is not the racy shape — routing it through the shared pair would change behaviour, so the invariant is left half-applied rather than decided. And `TestAssertMarkerCount_ExportedEntryPointUsesTheSharedBudget` writes its marker at 100ms, so any budget above ~150ms passes: the name claims a distinction the test does not draw. One related note for the same pass: the hydrate helper unsets the skeleton marker *before* the exec, so any test treating marker-clear as proof the hook ran is racing the hook's own runtime; `cmd/noncontiguous_window_reboot_integration_test.go:399-402` is safe against absence but pins no count, so a cross-fire would pass.

**Solution**: Bring the two `internal/restore` holdouts onto the shared pair, make the marker-count test's name true by moving its write past the probe budget, pin a count at `cmd/noncontiguous_window_reboot_integration_test.go:399-402` so a cross-fire cannot pass, and declare a second named short pair for the `cmd/bootstrap` hook-fire family. Derivation: there genuinely are two budgets, so "declared once" is satisfied by naming both and stating the difference, not by collapsing them. Routing the hook-fire family through the long pair would lengthen waits in a family the evidence says is not the racy shape — changing working behaviour to satisfy a naming invariant — while leaving the five sites as commented locals is the half-applied state the finding reports. A second named pair puts the difference at a declaration where the next reader meets it.

**Outcome**: Both hydrate budgets are declared in `restoretest` and named for what they wait on, and the two tests whose names claim a distinction actually draw it.

**Do**:
1. Route the two `internal/restore` holdouts through `restoretest.HydrateBudget` / `HydrateTick`, deleting the raw `10*time.Second, 50*time.Millisecond` pair: `integration_full_test.go:128` and `exit_closes_pane_integration_test.go:169`.
2. Declare a second named short pair in `restoretest` for the `cmd/bootstrap` hook-fire family and route its five `2s`/`50ms` sites through it, so the difference between the two budgets is stated at a declaration rather than implied by five locals.
3. Make `TestAssertMarkerCount_ExportedEntryPointUsesTheSharedBudget` true to its name by moving its marker write past the probe budget, so a budget shorter than the shared one fails it.
4. Pin a marker count at `cmd/noncontiguous_window_reboot_integration_test.go:399-402`, so a cross-fire cannot pass an absence-only assertion.

**Acceptance Criteria**:
- [ ] No raw budget/tick pair remains in `internal/restore`.
- [ ] The short budget is one named declaration used by all five `cmd/bootstrap` hook-fire sites, and their waits are unchanged in length.
- [ ] The marker-count test fails under a budget shorter than the shared one.
- [ ] The non-contiguous reboot fixture pins a count, not just an absence.

**Tests**:
- Refactor for the two holdouts and the five `cmd/bootstrap` sites; their waits are unchanged.
- `"it fails when the caller's budget is shorter than the shared one"`
- `"it fails when more markers fire than expected"`

## Task 37: `Client.SendKeys` has no production callers and is exported anyway
severity: dead-code
sources: bank

**Problem**: `internal/tmux/tmux.go:638` is called only from its own unit test and from real-tmux integration drivers in two other packages (`cmd/bootstrap/eager_signal_hydrate_integration_test.go`, `internal/restore/exit_closes_pane_integration_test.go`), where it types into live panes. That makes it materially unlike the pane-listing methods deleted alongside it — those had genuinely no consumer, this one is a shared test driver that happens to live on the production client. The claim made when the family was pruned, that the tmux client exports no method with zero production callers, is therefore not literally true tree-wide.

**Solution**: Keep `SendKeys`, documented at its declaration as a sanctioned shared test driver with no production caller, and narrow the tree-wide claim to the pane-listing family it was actually made about. Derivation: it is materially unlike the methods deleted beside it — three integration drivers in two packages type into live panes through it, so deleting it means re-authoring the same tmux invocation three times, reintroducing the duplication class this cycle exists to remove. What was overstated is a claim in prose, and correcting prose is both cheaper and truer than deleting a working shared helper to make the sentence retroactively correct.

**Outcome**: `SendKeys` survives with its role stated at the declaration, and the tree-wide claim is narrowed to the family it was actually made about.

**Do**:
1. Comment at `SendKeys`' declaration (`internal/tmux/tmux.go:638`) that it is a sanctioned shared test driver with no production caller — one line, wording the executor's.
2. Narrow the claim wherever this repository makes it — that the tmux client exports no method with zero production callers — to the pane-listing family it was made about, checking CLAUDE.md's `tmux` row and any in-source statement. Other work units' artefacts are not edited.
3. Leave `SendKeys` and its three integration drivers exactly as they are.

**Acceptance Criteria**:
- [ ] `SendKeys` is unchanged in behaviour and still exported.
- [ ] Its declaration says why it has no production caller.
- [ ] No claim in the repository asserts the client exports no method without a production caller.

**Tests**:
- No behaviour change: `SendKeys`' unit test and the two integration drivers stay exactly as they are.

## Task 38: `ResolveSessionDir` classifies sentinels its only production implementation cannot emit, for a caller that discards the error
severity: dead-code
sources: bank

**Problem**: Now that `ActivePaneCurrentPath` is established to answer an unmatched target with an empty expansion at exit 0, `*tmux.Client` cannot produce `ErrNoSuchSession` on this path — yet `internal/session/dirresolve.go` retains its `errors.Is(err, tmux.ErrNoSuchSession) || errors.Is(err, tmux.ErrEmptyPaneList)` branch. The executor kept it deliberately, as classification of the `PaneCurrentPathReader` *interface* contract rather than a claim about what tmux emits, and it has real coverage through a fake reader. The other end makes the question sharper: the seam's only consumer, `internal/tui/model.go:1163`, tests `ok && err == nil` and drops the error, so the unresolvable-versus-fatal distinction the branch draws is invisible — every non-ok outcome degrades identically.

**Solution**: Drop the `ErrNoSuchSession` / `ErrEmptyPaneList` branch and let the empty-path signal be the whole contract, simplifying the seam to what its caller consumes. Derivation: no production implementation of `PaneCurrentPathReader` can emit either sentinel on this path — `ActivePaneCurrentPath` answers an unmatched target with an empty expansion at exit 0 — and the seam's only consumer tests `ok && err == nil` and discards the error, so the distinction the branch draws is unreadable at both ends. Making the TUI consume it would invent a user-facing distinction nothing asked for, against a fallback that degrades identically by design and re-derives on the next picker launch. Keeping it as interface hygiene is the dead-branch class this cycle is removing, which is how the finding graded it.

**Outcome**: `ResolveSessionDir` classifies only what its production reader can produce and its caller can consume — the empty-path signal — and nothing asserts a distinction the seam no longer draws.

**Do**:
1. Delete the `errors.Is(err, tmux.ErrNoSuchSession) || errors.Is(err, tmux.ErrEmptyPaneList)` branch at `internal/session/dirresolve.go:39`, leaving the empty path as the whole unresolvable signal.
2. Rewrite `ResolveSessionDir`'s doc to state the contract that remains: a reader answering an empty expansion at exit 0 is the production shape, and an unresolvable session yields the not-ok result.
3. Drop or re-aim the fake-reader test that covered the deleted branch, so no test asserts the removed sentinels on this path.
4. Leave `internal/tui/model.go:1163`'s `ok && err == nil` consumer untouched.

**Acceptance Criteria**:
- [ ] The seam has one degradation path.
- [ ] No test asserts `ErrNoSuchSession` or `ErrEmptyPaneList` on this path.
- [ ] The TUI's lazy dir-resolution fallback behaves identically — a non-ok outcome degrades the same way it does today.
- [ ] `internal/tmux`'s sentinels are untouched; only this seam's classification changes.

**Tests**:
- Refactor: the remaining `dirresolve` tests keep their subjects, and the TUI's grouped-render fallback tests are unchanged.

## Task 39: CLAUDE.md's `logtest` row describes a surface that no longer exists
severity: comments
sources: bank

**Problem**: The row states that records are read through "the chainable `Records` filters (`AtExactLevel` / `AtOrAboveLevel` / `Msg` / `With`)" — those four are now unexported (`atExactLevel`, `atOrAboveLevel`, `withMessage`, `matching`), so a contributor writing against the row's names hits a compile error. It also ends with "`internal/restore`'s is the last one left", naming a wrapper that no longer exists: `internal/restore/logging_capture_test.go` was deleted, and the row now carries a false claim of exactly the kind the consolidation existed to remove. A third, smaller inaccuracy: the row's hands-back sentence does not mention that `NewCaptureLogger(t)` returns the sink alongside the logger.

**Solution**: Correct all three so the row describes the shipped surface. The filter-name clause is wrong as of today whichever way the query-surface question is settled, so this stands on its own; if that direction changes the surface, the row is corrected once more at that point rather than left wrong in the meantime.

**Outcome**: Every name and claim in CLAUDE.md's `logtest` row matches the shipped surface, so a contributor writing against the row compiles.

**Do**:
1. Read the row as it stands (CLAUDE.md, the `logtest` table row) and check each of the three clauses against `internal/logtest/capture.go` before editing — parts of the row have been rewritten since the finding was deposited, so correct only what is still wrong.
2. Correct the hands-back sentence, which still says `NewCaptureLogger(t)` "hands back a standalone `*slog.Logger`" without naming the `*Sink` it returns alongside it.
3. Check the row's listed method and accessor names against the package's exported surface and reconcile any name that no longer exists or is now unexported.
4. Check the row's closing claims against the tree — no wrapper it names may already be deleted.

**Acceptance Criteria**:
- [ ] Every identifier the row names exists and is exported.
- [ ] The row names both return values of `NewCaptureLogger`.
- [ ] The row names no wrapper that has been deleted.
- [ ] No code changes — documentation only.

**Tests**:
- Documentation only: `internal/logtest`'s suite is unchanged.

*Task 42 may rewrite this row again; these corrections stand in the meantime.*

## Task 40: `snapshotLockBound`'s justification lost its concrete "why" and gained a vacuous clause
severity: comments
sources: bank

**Problem**: The comment on `snapshotLockBound` (`internal/hooks/lock.go:29-45`) says the pre-read "is bounded at the cheapest figure that still grants an uncontended lock". A reviewer probed `acquireLock` directly against a held exclusive lock and found the first `Flock` attempt *precedes* any deadline check — so an uncontended acquire cannot time out at **any** bound, which makes the clause vacuous: every figure grants it. The same probe produced the stronger argument the comment should carry, that a short pre-read bound introduces no spurious-degradation surface at all. Meanwhile the concrete justification the prior comment carried — "four poll intervals above the sub-millisecond critical section", the sentence explaining why a hundredth rather than a thousandth — was dropped in the rewrite. A third clause asserts that the derivation "bounds the clean pre-read below the mutation bound" over three hand-picked values, a property that is false below roughly 10ms given the floor: a three-point sample dressed as an invariant.

**Solution**: Replace the vacuous clause with the loop-ordering argument that actually makes the short bound safe, restore the concrete figure justification, and either scope the below-the-mutation-bound claim to the range where it holds or drop it.

**Outcome**: `snapshotLockBound`'s comment carries the argument that actually makes the short bound safe, and no clause in it is vacuous or false.

**Do**:
1. Replace the "cheapest figure that still grants an uncontended lock" clause (`internal/hooks/lock.go:29-45`) with the loop-ordering argument: `acquireLock`'s first `Flock` attempt precedes any deadline check, so an uncontended acquire cannot time out at any bound — which is why a short pre-read bound adds no spurious-degradation surface.
2. Restore the concrete figure justification the rewrite dropped: four poll intervals above the sub-millisecond critical section, which is why a hundredth rather than a thousandth.
3. Scope the "bounds the clean pre-read below the mutation bound" claim to the range where it holds given the floor, or drop it — it is false below roughly 10ms and was sampled at three hand-picked values.
4. Change no code: the derivation, the floor and `SnapshotLockBoundForTest` stay exactly as they are.

**Acceptance Criteria**:
- [ ] No clause in the comment is vacuous or false at any `lockTimeout` the code admits.
- [ ] The concrete figure justification is present.
- [ ] The derivation and the floor are unchanged.

**Tests**:
- Comment only: `internal/hooks`' lock suite, including the `SnapshotLockBoundForTest` pins, is unchanged.

## Task 41: Two leaf packages have no guard, and one of them now underwrites a test-isolation guarantee
severity: low
sources: bank

**Problem**: `internal/xdg` and `internal/sourceguardtest` are both stdlib-only today (confirmed by `go list -deps`) and neither property is pinned, unlike `internal/nanoid`, `internal/prefs`, `internal/hooks` and `internal/theme`, which each pin their dependency set. Both have since acquired a reason to be pinned: `internal/hookstest` now depends on `internal/xdg` for seed/read path parity, so its leaf property underwrites a test-isolation guarantee rather than just tidiness; and `sourceguardtest`'s stdlib-only, untagged contract — stated in its own doc and in CLAUDE.md — is what keeps roughly twenty source guards in the unit lane, while the package now shells out via `os/exec`, making the next addition likelier to reach for a non-stdlib helper.

**Solution**: Add the two guards modelled on `internal/nanoid/leaf_guard_test.go`, over `sourceguardtest.PackageDeps` — roughly fifteen lines each, and the primitive already exists.

**Outcome**: `internal/xdg` and `internal/sourceguardtest` cannot acquire a dependency that breaks what their leaf status underwrites without a test saying so.

**Do**:
1. Add `internal/xdg/leaf_guard_test.go` over `sourceguardtest.PackageDeps` (or `AssertDepsWithin` once task 22 lands), asserting the transitive set is stdlib-only.
2. Add the same guard for `internal/sourceguardtest`, asserting stdlib-only and untagged.
3. State in each guard's rationale what the property underwrites — `hookstest`'s seed-and-read path parity for `xdg`, the unit-lane placement of the repo's ~20 source guards for `sourceguardtest`.
4. Make both fail on an empty or unresolved dep set rather than passing over one.

**Acceptance Criteria**:
- [ ] Both packages' transitive dependency sets are pinned by a guard.
- [ ] Each guard fails on an empty or unresolved set.
- [ ] Both guards run in the unit lane.
- [ ] Each guard states what its property underwrites.

**Tests**:
- `"it confines internal/xdg to the standard library"`
- `"it confines internal/sourceguardtest to the standard library"`

## Task 42: `logtest.Sink`'s query surface grows multiplicatively because the composable filters were closed off
severity: low
sources: architecture, bank

**Problem**: `Records` has four filter methods — `atExactLevel`, `atOrAboveLevel`, `withMessage`, `matching` — all unexported, with the stated rationale that a caller must never have two ways to ask one question. The cost is that every *combination* of those orthogonal dimensions needs its own exported `Sink` method, and two already exist purely as one-line compositions of others. The surface stands at seven `Records*` methods for two-and-a-bit dimensions and grows multiplicatively: the next axis doubles it rather than adding one, and the next at-or-above-level-plus-message need adds an eighth method rather than a composition. The single-route property that motivated the closure is worth keeping, but it is achievable by exporting the orthogonal filters and *not* exporting combinations, rather than the reverse.

**Solution**: Export the four orthogonal filters and keep only the base `Records()` plus the `OnlyRecord*` assertions, so callers compose one route per question themselves and the combination methods disappear; correct `TestRecords_FilterChainCombinesLevelAndComponent` and `TestRecords_MsgFiltersOnMessageAloneAcrossComponents`, whose names describe the chainable surface, in the same pass, and update CLAUDE.md's `logtest` row alongside task 39. Derivation: the property being protected is that a caller never has two routes to one set, and the closed surface no longer holds it — two of the seven exported methods already exist as one-line compositions of others, which is a second route by definition. Exporting the orthogonal dimensions and declining to export their combinations holds the same property while making the surface grow additively; keeping the closure prices in multiplicative growth to protect a property it has already lost.

**Outcome**: `logtest`'s query surface grows by one method per new dimension rather than doubling, and a caller still has exactly one route to any given set.

**Do**:
1. Export the four orthogonal filters on `Records`: `AtExactLevel`, `AtOrAboveLevel`, `WithMessage` and `Matching` (`internal/logtest/capture.go:114-132`).
2. Delete the six derived `Sink.Records*` methods, keeping `Sink.Records()` plus `OnlyRecord` / `OnlyRecordWith` (and `Only`, if task 23 landed it).
3. Re-point every caller across `cmd`, `cmd/bootstrap`, `internal/state`, `internal/hooks`, `internal/restore` and the store suites to compose the one route per question.
4. Rename `TestRecords_FilterChainCombinesLevelAndComponent` and `TestRecords_MsgFiltersOnMessageAloneAcrossComponents` to describe the exported surface, and update CLAUDE.md's `logtest` row alongside task 39.

**Acceptance Criteria**:
- [ ] `Sink` exposes one base query plus the exactly-one assertions; no combination method survives.
- [ ] The four filters are exported and composable, and no two routes reach the same set.
- [ ] Every caller composes its own route; none re-derives a filter.
- [ ] CLAUDE.md's `logtest` row matches the shipped surface.

**Tests**:
- `"it filters at exactly the given level"`
- `"it composes a level filter with a component filter"`
- `"it filters on message alone across components"`
- Existing consumer suites keep their assertions, expressed through the composed routes.

## Task 43: `--pane-key`'s help text and README narrow the flag below the contract it was given
severity: low
sources: standards

**Problem**: The flag help reads `Pane token of the pane whose hook should be removed (defaults to the current pane)` (`cmd/hooks.go:310`) and README:191 says it removes "a hook for any pane token". The flag is a literal pass-through with no validation of any kind, and it carries a specific second purpose: it is the route by which an old-format, non-token entry — the class the shape-aware reaper retains forever, by design, because reaping it would be a guess — is removed by hand. An old-format key is not a pane token, so a user following the help text has no indication that the one sanctioned removal route for a retained entry is this flag. CLAUDE.md gets this right; the CLI's own help and the README do not.

**Solution**: Reword the flag help and the README sentence to say the flag takes any hook key verbatim, defaulting to the current pane's token, so the retention design's escape hatch is discoverable from the CLI itself.

**Outcome**: A user reading `portal hook rm --help` can see that the flag removes an entry under any hook key, including the old-format entries the reaper retains forever.

**Do**:
1. Reword the flag help at `cmd/hooks.go:310` to say it takes any hook key verbatim, defaulting to the current pane's token.
2. Reword README:191 (and :200 where it repeats the claim) to match, naming this as the sanctioned route for removing a retained old-format entry by hand.
3. Keep the flag a literal pass-through — add no validation of any kind.

**Acceptance Criteria**:
- [ ] The flag help describes a verbatim hook key, not a pane token.
- [ ] README says the same, and names the retention escape hatch.
- [ ] No validation is added — a non-token key still passes through unchanged.
- [ ] Exit-code behaviour is untouched: removing nothing is still non-zero.

**Tests**:
- `"it removes an entry keyed on a non-token hook key via --pane-key"` — existing behaviour, pinned
- `"it exits non-zero when the named key matches no entry"`
- The usage-string assertion, if the suite pins help text.

## Task 44: The sweep enumerates every pane on the server before checking whether there is anything to sweep
severity: low
sources: architecture, bank

**Problem**: `liveTokenEnumeration` (`cmd/run_hook_stale_cleanup.go`) runs `judgeAgainstLivePanes` — a whole-server `list-panes -a -F` — and only afterwards short-circuits on `len(snapshot) == 0`. The comment on that short-circuit reasons carefully about the three costs it avoids (creating the config directory, creating the sidecar, taking an exclusive hold), but the tmux read it has already paid is the most expensive of the four, and on an install that has never registered a hook it can change nothing: with an empty snapshot there is no key to judge and none to protect, whatever the enumeration returns. The daemon pays it every ten seconds for the lifetime of the process. The cheapest and most decisive test is ordered last. Two smaller items in the same file: `liveTokenEnumeration` calls `logger.Debug` while the nil-logger default stays behind in `runHookStaleCleanup`, so an in-package caller passing nil now panics where the inline closure made that unreachable; and `emit()` followed by `return sweepOutcome{DeclineReason: …}` appears at four sites.

**Solution**: Take the empty-snapshot branch before the enumeration, and either drop the counts DEBUG for that case or emit it with the entry count alone — the pane count has nothing to report about a cycle that was never going to judge anything. Default the logger inside the factory, and collapse the four emit-then-return sites into one helper.

**Outcome**: An install with no persisted hook costs the daemon no tmux read at all, and the cycle's counts never report a pane figure it did not take.

**Do**:
1. In `liveTokenEnumeration` (`cmd/run_hook_stale_cleanup.go:207-226`), take the `len(snapshot) == 0` branch **before** `judgeAgainstLivePanes`, so an install that has never registered a hook pays no whole-server `list-panes -a`.
2. Either drop the counts DEBUG for that case or emit it with the entry count alone — a cycle that never enumerated has no pane count to report.
3. Default `countsLogger` inside the factory rather than in `runHookStaleCleanup` (`:237-239`), so an in-package caller passing nil cannot panic on the factory's own `logger.Debug`.
4. Collapse the four `emit()`-then-`return sweepOutcome{DeclineReason: …}` sites in `declinedSweep` into one helper.

**Acceptance Criteria**:
- [ ] With an empty `hooks.json`, one cycle issues zero pane enumerations (assertable on the fake lister's call count).
- [ ] The counts DEBUG never carries a pane count for a cycle that did not enumerate.
- [ ] `liveTokenEnumeration(reader, nil)` does not panic.
- [ ] `declinedSweep`'s four decline sites route through one emit-and-return, with each branch's reason and level unchanged.

**Tests**:
- `"it enumerates no panes when nothing is persisted"`
- `"it reports the entry count alone for an empty snapshot"`
- `"it tolerates a nil counts logger"`
- `"it emits the same reason and level for each decline"` — existing decline coverage, unchanged

## Task 45: Two stand-down reasons cannot reach the surface that renders them
severity: low
sources: bank

**Problem**: `lock-timeout` and `store-read-failed` cannot be reached through `checkStaleHooks` as the code stands: a lock timeout on a read degrades to an unlocked read rather than standing down, and a failed `Load` is reported with its own hardcoded detail rather than routing through the vocabulary. Their diagnosis lines are pinned through the real renderer over a synthetic result, so the assertions read the user's line — but the path is not exercised end to end, and the enum currently carries members the diagnosis surface can never produce. (The `store-read-failed` half is closed by routing the hardcoded literals through the vocabulary; this is about what remains after that.)

**Solution**: Record at the declaration that the reason vocabulary is deliberately complete rather than fully reachable, so a reader does not go hunting for the missing path. Derivation: the specification already settled this. Its 2026-09-01 corrigendum states that `notEvaluableDetails` gains its `lock-timeout` entry so all five reasons render a phrase on both surfaces — "vocabulary completeness rather than an observed leak, since `lock-timeout` cannot reach that path today (a lock acquisition failure degrades to an unlocked read)". Making the reasons reachable would reverse that decision and reclassify the degrade-to-unlocked read as a bug, which the record deliberately does not do; an annotation makes the settled position visible where the enum is declared.

**Outcome**: A reader meeting the reason vocabulary is told which members the diagnosis surface cannot produce and why, instead of hunting for a missing path.

**Do**:
1. Record at the `skipReasons` declaration (`cmd/run_hook_stale_cleanup.go:26-32`) that the vocabulary is deliberately complete rather than fully reachable: `lock-timeout` cannot reach the diagnosis path because a lock failure on a read degrades to an unlocked read, and `store-read-failed` reaches it only through the vocabulary once task 29 routes the hardcoded literals.
2. Change no behaviour: the degrade-to-unlocked read stays as the specification settled it.
3. Leave the synthetic-result renderer tests as the coverage for the unreachable pair.

**Acceptance Criteria**:
- [ ] The declaration states which reasons are unreachable on which surface, and why.
- [ ] No code path changes; the five-reason completeness guards still pass.
- [ ] The degrade-to-unlocked read is untouched.

**Tests**:
- No behaviour change: the decline-vocabulary guards and the renderer's synthetic-result tests are unchanged.

## Task 46: The target-composition guard's exemption is wider than its recognition
severity: low
sources: bank

**Problem**: The guard recognises an already-composed target purely by the *parameter name* (`target`, `paneID`), so a client method naming its parameter anything else silently leaves its call sites unchecked, and the exemption side is wider than the recognition side: `targetTakingMethods` skips functions with no receiver while `bindParams` exempts the parameter in **any** function, so a bare target reaching the client through a non-method helper produces no finding — staged and confirmed silent. Three further gaps sit in the same family: the wrong-helper subclass is uncaught (the guard enforces vocabulary membership, not target *kind*, so `list-panes -t ExactSessionTarget(x)` — the original defect: right vocabulary, wrong kind, still reaching the prefix sibling — passes); the binding is name-based and flow-insensitive, so reassignment and branch-dependent assignment both pass; and the `-t ends its argv` branch has no fixture, leaving the split-composition detector itself unexercised. The name-based rule has already cost something concrete: a parameter had to be renamed from `liveTarget` to `target` purely to satisfy the allow-list, losing the live-versus-saved distinction the old name carried. Two cosmetic residues: the exported vocabulary is split between two naming shapes (`ExactSessionTarget`/`ExactCoordTarget` prefix, `PaneTargetExact`/`windowTargetExact` suffix), and `exact_target_internal_test.go` is now an internal test for two exported functions, so `internal` in its filename has gone stale.

**Solution**: Land the verified cheap fixes now — drop the no-receiver skip and accept an identifier callee, closing the gap between what the guard recognises and what it exempts — add the missing fixture for the `-t ends its argv` split-composition branch, unify the vocabulary's naming shape while the name-keyed allow-list is being edited, rename `exact_target_internal_test.go` now that its subjects are exported, and record the named `type Target string` at the declaration as the durable answer. Derivation: the cheap fixes are prototyped and measured — the staged probe was caught with zero new findings on the real tree — so they close the demonstrated hole immediately. The named type subsumes the flow-insensitivity and laundering gaps, but it is a signature change across the client's whole surface reaching `cmd`, `internal/state` and `internal/restore`, which is a scope this bugfix work unit should not absorb in its third analysis cycle. Recording it at the declaration is what keeps it from being rediscovered as a finding rather than picked up as a decision.

**Outcome**: The guard's exemption is no wider than its recognition, its split-composition branch is exercised, and the durable answer is recorded where the next reader meets it.

**Do**:
1. In `internal/tmux/target_composition_guard_test.go`, drop `targetTakingMethods`' no-receiver skip and accept an identifier callee, so a bare target reaching the client through a non-method helper is a finding rather than an exemption.
2. Add the missing fixture for the `-t ends its argv` split-composition branch, which is currently unexercised.
3. Unify the vocabulary's naming shape while the name-keyed allow-list is being edited — `ExactSessionTarget` / `ExactCoordTarget` against `PaneTargetExact` / `windowTargetExact` — updating `exactTargetHelpers` and every call site.
4. Rename `exact_target_internal_test.go`, whose subjects are now exported.
5. Record the named `type Target string` at the vocabulary's declaration as the durable answer to the flow-insensitivity and laundering gaps, so it is picked up as a decision rather than rediscovered as a finding.

**Acceptance Criteria**:
- [ ] A bare target spent through a non-method helper is caught (proven by the staged probe).
- [ ] The real tree produces zero new findings under the tightened guard.
- [ ] The split-composition branch has a fixture and fails when broken.
- [ ] The four helpers share one naming shape, and the file name matches its subject.

**Tests**:
- `"it flags a bare target composed in a non-method helper"`
- `"it flags a target split across the end of an argv"`
- `"it passes on the real tree"`
- Existing guard fixtures keep their verdicts.

## Task 47: Hydrate-config residue: one fall-through has no test and one literal survives the builder
severity: low
sources: bank

**Problem**: `cmd/state_hydrate.go:122` and `:137` are the nil-`HandleFileMissing` fall-through branches, and neither has a test — at HEAD or now. They are unreachable in production (the cobra wiring always sets the handler), so this is a latent gap rather than a live one, but the *sibling* fall-through (nil `HandleTimeout`) does have a dedicated test, deliberately preserved as an inline literal precisely because it is the standing proof that path still exists; the file-missing side has no such proof. That surviving inline literal is also why the builder's stated outcome — a new required `hydrateConfig` field is added once rather than 52 times — is really two places, not one: the builder structurally cannot express an explicit nil. One further case outside the three converted suites carries the same load-bearing nil `HookStore` without saying so.

**Solution**: Give the file-missing fall-throughs the standing proof their sibling has, make the remaining implicit nil explicit, and settle whether the builder should grow a way to say explicit-nil — or state the two-places figure wherever the one-route claim is made.

**Outcome**: Both file-missing fall-throughs carry the standing proof their timeout sibling has, and no fixture leaves a load-bearing nil implicit.

**Do**:
1. Add a dedicated test for each nil-`HandleFileMissing` fall-through (`cmd/state_hydrate.go:122` and `:137`), matching the shape of the existing nil-`HandleTimeout` test that stands as proof its path still exists.
2. Make the remaining implicit nil explicit at the one case outside the three converted suites, so its load-bearing nil `HookStore` is stated rather than inherited from a zero value.
3. Settle the builder's gap: either give `hydrateConfig`'s builder a way to say explicit-nil, or state the two-places figure wherever the "a new required field is added once" claim is made.

**Acceptance Criteria**:
- [ ] Both file-missing fall-throughs fail if the branch is deleted.
- [ ] No fixture carries a load-bearing nil implicitly.
- [ ] The builder either expresses explicit-nil, or its stated reach matches the two places it actually is.

**Tests**:
- `"it falls through when no file-missing handler is set"` — one per branch
- `"it carries an explicit nil hook store"`

## Task 48: The stand-down reason vocabulary is closed by naming convention, not by the type system
severity: low
sources: bank

**Problem**: `declineDebug` and `declineWarn` take `reason string`, so a raw literal or an off-convention const name compiles. A reviewer probed it: a const named without the `skipReason` prefix, absent from the enumerable slice and from both phrase maps, passes **both** guards — because the source guard matches on the name prefix. The alternative was offered when the vocabulary was closed and not taken: a small named type would close the hole at compile time rather than by convention. The limitation is inherent to every name-based source guard in the repo, so the decision generalises beyond this vocabulary.

**Solution**: Make the vocabulary a `type skipReason string` so an off-convention or literal reason cannot compile, accepting that it touches the decline ladder and the typed outcome shipped by earlier tasks.

**Outcome**: A raw string cannot be a decline reason — the compiler closes the vocabulary the naming convention was standing in for.

**Do**:
1. Declare `type skipReason string` in `cmd/run_hook_stale_cleanup.go` and re-type the five consts, the `skipReasons` slice, both phrase maps, `declineDebug` / `declineWarn`, `standDown.reason`, `standDownAttrs` and `phraseFor`.
2. Re-type the shipped outcome — `sweepOutcome.DeclineReason` — and its consumers in `cmd/doctor.go`, converting to `string` only at the rendering and log-attr boundaries.
3. Narrow or retire the name-prefix source guard where the type now holds the property, keeping whatever it still adds (membership of the enumerable set and both phrase maps).

**Acceptance Criteria**:
- [ ] A raw string literal cannot be passed as a decline reason — it is a compile error.
- [ ] An off-convention const of the typed kind still fails the enumerable-set and phrase-map guards.
- [ ] Rendered lines and log attrs are byte-identical to today's.
- [ ] The guard that remains states what it still adds over the type.

**Tests**:
- Refactor plus a type change: rendered output and emitted attrs are unchanged.
- `"it renders the same phrase for every reason"` — existing copy tests
- `"it fails a reason absent from the enumerable set"`

## Task 49: Two stated outcomes are held by contributor discipline in a repo whose house style is source guards
severity: low
sources: bank

**Problem**: Two rules established this phase have no structural enforcement in a repo carrying roughly twenty source guards. First, the Install-only rule: `logtest.Install(t)` is asserted to be the only route to a package-level capture handler, paired by construction at every site, but a contributor can write the two lines by hand and nothing objects — a guard would need to encode the three sanctioned survivors (the pre-`Init` discard silencer, the level-gate handler, the JSON-rendering handler). Second, the widened lane rule: a test that *builds* a portal binary now belongs in the integration lane, asserted in four prose places in CLAUDE.md and nowhere else. The second is the sharper one, because verifying it empirically is awkward — a reviewer had to instrument the builder directly, since a PATH shim on `go` does not work (Go 1.24+ prepends the toolchain dir for test binaries) — so the guard would also replace a check that is otherwise hard to run.

**Solution**: Add both guards over the existing source-guard primitives: one failing when `log.SetTestHandler` is called with a `logtest.Sink` outside the sanctioned sites, one failing when an untagged `*_test.go` references the portal-binary build helpers.

**Outcome**: Both rules established this phase are held by a guard rather than by contributor discipline and four prose claims.

**Do**:
1. Add a source guard failing any call to `log.SetTestHandler` with a `logtest.Sink` outside `logtest.Install`, encoding the three sanctioned survivors — the pre-`Init` discard silencer, the level-gate handler and the JSON-rendering handler — so `Install(t)` is the only route to a package-level capture handler.
2. Add a source guard failing any **untagged** `*_test.go` that references the portal-binary build helpers (`portalbintest.BuildPortalBinary` / `StagePortalBinary`, `restoretest.BuildPortalBinaryDir` / `BuildPortalBinaryStable`), so the widened lane rule is structural.
3. Build both on the shared primitives — task 20's parse helper, `ForEachFuncCall`, `CalleeName`, `GoSourceFiles` — rather than new scanning code, and make each fail when it scans nothing.
4. Cover each with a staged probe so the guard's own failure path is exercised.

**Acceptance Criteria**:
- [ ] A hand-installed `SetTestHandler` over a `logtest.Sink` outside the sanctioned sites fails the first guard; the three survivors pass.
- [ ] An untagged test referencing a build helper fails the second guard.
- [ ] Both guards run in the unit lane and fail when they scan nothing.
- [ ] The tree passes both.

**Tests**:
- `"it flags a hand-installed capture handler"`
- `"it allows the three sanctioned handlers"`
- `"it flags a portal-binary build helper referenced from an untagged test"`
- `"it fails when it scans nothing"` — one per guard
