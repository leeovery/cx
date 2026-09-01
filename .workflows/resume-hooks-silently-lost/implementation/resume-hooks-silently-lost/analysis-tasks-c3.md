# Analysis Tasks: resume-hooks-silently-lost (Cycle 3)

## Task 1: `go test ./cmd` moves the developer's real legacy config
severity: high
sources: bank

**Problem**: `cmd/config_test.go:10` and `:57` call the real `configFilePath("TEST_CONFIG_UNSET", "projects.json")` with `t.Setenv("XDG_CONFIG_HOME", "")` and the developer's real `$HOME`; `cmd/prefs_path_test.go:41` does the same through `prefsFilePath()`. `configFilePath` calls `migrateConfigFile(oldPath, resolved.Path, ...)` on the non-overridden path (`cmd/config.go:77-78`), so on any machine still holding `~/Library/Application Support/portal/` the **unit lane** silently `os.Rename`s `projects.json` and `prefs.json` into `~/.config/portal/`. A reviewer reproduced this on both the pre-change and post-change trees under a staged fake HOME, three runs each. This is a direct breach of the project's absolute invariant that a test must never mutate the real filesystem outside its temp dirs, and it is invisible because the migration is a one-shot that succeeds silently.

**Solution**: Point those subtests at a temp HOME (`t.Setenv("HOME", t.TempDir())`) and assert against that temp home rather than the ambient one, so the migration they trigger lands inside the test's own sandbox. Sweep the rest of `cmd` for any other subtest that exercises a real config-path resolver against the ambient `$HOME`.

**Outcome**: The unit lane can be run on a machine holding the legacy macOS config directory without moving the developer's files.

## Task 2: Only `ShowEnvironment` classifies an unaddressable session name; every other per-session op keeps the blind spot
severity: medium
sources: bank

**Problem**: `wrapSessionTargetErr` — the classifier that gates on `ValidateSessionName` before falling through to `wrapNoSuchSession`, so a live colon-named session is not reported as absent — is wired into exactly one call site, `ShowEnvironment` (`internal/tmux/tmux.go:582`). `HasSession`, `KillSession`, `SwitchClient`, `SetSessionEnvironment`, `ListClients` and `saverPanePID` (`internal/tmux/saver_pane_pid.go:15`, which uses the bare `wrapNoSuchSession`) all compose an exact target and will report a live colon-named session as absent, indistinguishably from a real one. `SaverPanePIDOrAbsent` is exported with a free-form session name, so the hole is reachable rather than merely theoretical. Two supporting gaps: `TestSessionTargetsAreComposedExactly` (`internal/tmux/exact_session_target_test.go`) tables six methods and omits `SaverPaneID`, and the prefix-sibling route enumeration in `exact_session_target_realtmux_test.go` omits both saver reads — which is why this class survived to phase 7 in that file at all.

**Solution**: Settle the classification uniformly rather than per call site, add the missing `SaverPaneID` row to the central target table, and derive the real-tmux route enumeration from the set of per-session reads so the next route added is guarded by construction rather than by a later analysis pass.

**Decision**: Which per-session operations carry the unaddressable-name gate?
1. Every per-session op routes through `wrapSessionTargetErr` — uniform, but it changes error classification for callers across `cmd`, `internal/restore` and `internal/spawn`.
2. Only the ops whose callers actually discriminate absence from failure (`ShowEnvironment`, `SaverPanePIDOrAbsent`, `HasSession`) carry the gate, and the rest document that they do not classify.

## Task 3: Two disagreeing definitions of an unwritable session name, and a `$`-leading name is still silently lost
severity: medium
sources: bank

**Problem**: `internal/session/naming.go:21` strips `:` **and** `.` at generation time through its own `strings.NewReplacer`, with no reference to the target grammar; `tmux.ValidateSessionName` (`internal/tmux/errors.go:74`) is the recognition side and covers `:` only. They agree today by coincidence, not by construction — the same generation/recognition drift the pane-token work consolidated into `internal/nanoid`. The gap is not academic: measured on tmux 3.7c against an isolated socket, a session named `$foo` cannot be addressed by name at all (both `-t "$foo"` and `-t "=$foo"` fail — tmux reads a leading `$` as a session-ID prefix), `ValidateSessionName` returns nil for it, so `wrapSessionTargetErr` falls through and `internal/state/capture.go` logs a vanished session and drops a live one. That end state is precisely the silent loss this work unit exists to remove, reached by a different character. Also measured: `.` alone is fine (`a.b` resolves both bare and exact), so the generator sanitises something the target grammar does not forbid.

**Solution**: Give the rule one definition that both sides read, and widen it to cover the leading-`$` case so a name tmux cannot address is refused or classified rather than silently reported absent.

**Decision**: Where does the definition live and how far does it reach?
1. One predicate in `internal/tmux`, with `internal/session`'s generator sanitiser derived from it — closes the drift, but needs the `internal/session` → `internal/tmux` import direction and the `.` case (a coord separator, not a session one) decided first.
2. Widen `ValidateSessionName` to cover the leading `$` and leave the generator's own sanitiser alone — closes the live hole without taking on the import question.
3. Both, in that order.

## Task 4: Bare pane targets still reach the tmux client from two production sites, and the guard cannot see either
severity: medium
sources: bank

**Problem**: `cmd/state_daemon.go:277` composes `tmux.PaneTarget(sess.Name, win.Index, pane.Index)` — the deliberately unpinned form — and hands it to `CapturePane` through `state.CaptureAndHashPane`, so a session killed between the index build and the capture leaves the read resolving onto a prefix sibling's pane, whose scrollback is then written under the gone pane's key. `cmd/open.go:90` hand-rolls the exact-target rule itself (`[]string{"tmux", "attach-session", "-t", "=" + name}`) rather than calling `tmux.ExactSessionTarget`, making it the fifth site to author that rule by hand. Neither is caught because `targetComposingPackages` (`internal/tmux/target_composition_guard_test.go:18`) is hardcoded to `{".", "../session", "../restore"}` — `cmd` is not in it, and nothing stops the next package from composing targets unguarded either.

**Solution**: Route the daemon capture loop's pane target through the pinned `PaneTargetExact` and `cmd/open.go` through `tmux.ExactSessionTarget`, then widen the guard's package set so `cmd` is covered and a new target-composing package cannot appear outside it — deriving the set (every package importing `internal/tmux`, say) rather than extending the literal list.

## Task 5: A failed `@portal-restoring` read is reported to the user as "restore in progress"
severity: medium
sources: standards

**Problem**: `hookStalenessStandDown` (`cmd/run_hook_stale_cleanup.go:136-141`) folds a failed marker read into the `restoring` reason via `state.RestoreWindowActive` — the correct fail-safe posture — but that reason then drives both user-facing surfaces: `portal doctor --fix` prints `Skipped stale hook prune: restore in progress` and the read-only diagnosis reports `restore in progress (not evaluable)`. `doctor` is bootstrap-exempt and starts no server, so with the tmux server down — the routine state after a reboot, and precisely when a user reaches for `doctor` — `TryGetServerOption` fails, no pattern matches "no server running", and the user is told a restore is running when none is. Before this change that path rendered the honest "zero live panes with hooks present". The specification's own 2026-09-01 corrigendum withdrew a stand-down phrase for exactly this class of mismatch — words that describe a condition other than the one that produced them — and its remedy was to give each condition its own phrase rather than let one borrow another's.

**Solution**: Keep the fail-safe fold — a failed read must still stand the cycle down — but separate the rendered phrase from the marker-set phrase by carrying the read failure as its own reason, so `skippedPrunePhrases` and `notEvaluableDetails` can say the marker could not be read rather than asserting a restore. The log line and the stand-down semantics are unaffected.

## Task 6: An unspecified session-name refusal with new user-facing copy shipped inside this work unit, documented nowhere
severity: medium
sources: standards

**Problem**: The implementation introduced `ErrUnaddressableSessionName` / `ValidateSessionName`, made `Client.RenameSession` refuse any name containing `:` before composing its argv (`internal/tmux/tmux.go:283`), added a matching refusal to the TUI rename modal with new flash copy (`internal/tui/model.go:2667`, `sessions_flash.go:54` — `":" isn't allowed in a session name — tmux reads it as a separator`), and converted a broad set of previously-bare `-t` targets to the exact-match forms. None of it is in the specification, which scopes the positional/structural addressing siblings as checked against the change rather than changed by it. It is a user-visible behaviour change — a rename the picker accepted yesterday is now refused — riding a release whose stated contract is the hook-key repair, and the copy went through no visual gate. README:195 still tells the user hooks stay attached "even if you rename it, whether from the picker's `r` modal" with no mention that some names are now refused, and neither README nor CLAUDE.md's `tmux` row records the refusal or the new exact-target helpers.

**Solution**: Settle whether the refusal ships with this release, and make the repository's own description match whatever ships.

**Decision**: Does the session-name refusal stay in this work unit?
1. It stays — document the refusal and the exact-target hardening in README's rename section and CLAUDE.md's `tmux` row, and put the flash copy through the same visual gate every other picker string gets.
2. It is lifted out of this work unit and lands with its own scope, leaving the exact-target hardening (which is not user-visible) documented in CLAUDE.md alone.

## Task 7: `internal/hooks` spells its one on-disk shape three different ways across its exported API
severity: duplication
sources: architecture

**Problem**: `map[string]map[string]string` — the `hooks.json` body — appears under three names on the package's public face. `hooksFile` is an *unexported alias* (`internal/hooks/store.go:29`) and the **exported** `Load(via Via) (hooksFile, error)` returns it, so godoc and every reader of the signature see an identifier they cannot name or follow. `Snapshot` is an exported defined type for the identical shape, taken by the exported `CleanStale`. And `StaleKeys` spells the raw map literally — it had to, because the alias it would naturally use is unexported. The consequence is visible where two of them meet (`cmd/doctor.go:305` feeding `store.Load(...)` into `hooks.StaleKeys(...)`): the value flows only because the alias is transparent, and nothing in the signatures tells the reader these are the same thing.

**Solution**: Name the shape once as an exported defined type — reuse `Snapshot`, or introduce `Entries` and document `Snapshot` as its role alias — and use it in `Load`, `StaleKeys`, `CleanStale` and the unexported `load`/`save`/`staleKeys` helpers. `Load` in particular must stop returning an unexported identifier.

## Task 8: `cmd` carries two opposing `*Deps` merge conventions, and the fail-silent one is unguarded
severity: medium
sources: architecture

**Problem**: `hookSeams()` (`cmd/hooks.go:83`) starts from whatever the test injected and *fills in* the production default per unset seam, so a field added to `HooksDeps` without a fill line is a nil dereference — loud and immediate. The two sibling resolvers beside it run the opposite direction: `resolveDoctorDeps` (`cmd/doctor.go:74`) and `resolveCommitNowDeps` build production defaults and then overwrite field-by-field from the injected struct (`if doctorDeps.X != nil { deps.X = doctorDeps.X }`, eleven times). A field added to `DoctorDeps` or `CommitNowDeps` without its matching override line is **silent** — the test's mock is ignored, the production default runs, and nothing anywhere fails. Two conventions for one job in one package is the drift; the fail-silent direction being the unguarded one is the sharper half.

**Solution**: Pick one direction for the whole package. The tools to hold either choice already exist: `cmd/deps_seam_guard_test.go` derives the guarded seam set from the production sources, and the repo already uses reflection-based struct-field completeness guards (`internal/theme/theme_test.go`, `cmd/doctor_advisory_test.go`).

**Decision**: Which direction does the package settle on?
1. Convert `resolveDoctorDeps` and `resolveCommitNowDeps` to the fill-in-defaults shape `hookSeams` established, so the compiler holds the invariant.
2. Keep the overwrite-merge shape and add a reflection guard asserting every exported field of each `*Deps` struct is named in its resolver.

## Task 9: `doctor --fix` prints nothing at all when the sweep genuinely fails
severity: medium
sources: bank

**Problem**: `pruneDoctorStaleHooks` (`cmd/doctor.go:197-210`) logs a WARN on a returned error and then renders from the zero `sweepOutcome` — so there is no `Pruned stale hook:` line and no `Skipped stale hook prune:` line, and a user who asked for a repair sees nothing at all. This is the same argument the stand-down lines exist to answer — a repair that printed nothing is indistinguishable from one that found nothing — now visible on the one path the stand-downs do not cover.

**Solution**: Give the error path its own user-facing line so every `--fix` invocation that touched the hook sweep says what happened, and settle the related default while the file is open: the nil-`countsLogger` fallback attributes the cycle's counts to the `bootstrap` component even though a guard forbids a bootstrap step from calling the sweep (reachable only from tests today, since both production callers supply a logger).

## Task 10: Integration fixtures disagree about which state directory they isolated
severity: medium
sources: duplication, bank

**Problem**: Three fixtures call `portaltest.IsolateStateForTest(t)` for its side effects only, discarding both return values, and then point `PORTAL_STATE_DIR` at a fresh `t.TempDir()`: `internal/restore/rename_reboot_shared_test.go:52`, `internal/restore/prefix_sibling_integration_test.go:33` and `internal/restore/multipane_legacy_integration_test.go:60`. The sandbox registry and the in-process daemon-ownership registration therefore name a directory nothing writes to while the daemon writes to an unregistered one, so a subprocess orphan sweep cannot recognise the fixture's own daemon — the exact property the registry exists to provide. They pass the teardown-guard coverage rule because they make all three calls; the rule says nothing about the *value* `PORTAL_STATE_DIR` is set to. Underneath sits the reason the divergence was possible: the same six-line preamble (`BuildPortalBinaryDir` → `IsolateStateForTest` → `t.Setenv("PORTAL_STATE_DIR", …)` → `state.EnsureDir` → `RegisterStateDirTeardownGuard`) is written out verbatim at roughly ten sites in `internal/restore` alone, and `IsolateStateForTest` derives its returned env slice from `os.Environ()` *before* the caller's `Setenv` runs, so the slice it hands back never carries the real state dir (under `cmd/bootstrap` it carries the `TestMain` poison).

**Solution**: Settle on the helper's returned `stateDir` — the directory the sandbox registry and the fingerprint backstop are keyed on — at all three divergent sites, and remove the trap that let them diverge by having the helper own `PORTAL_STATE_DIR` in both the process env and the returned slice.

**Decision**: How far does the consolidation reach?
1. Hoist the `PORTAL_STATE_DIR` set into `IsolateStateForTest` and fix the three divergent sites, leaving the preamble inline at each fixture so the per-file coverage rule keeps seeing it.
2. Additionally extract the preamble into one `restoretest` fixture helper returning binDir, stateDir and socket — which requires teaching the coverage rule to follow calls into a package-local arrange, since `cmd/bootstrap`'s five fixtures are already invisible to it for exactly that reason.

## Task 11: The teardown-guard coverage rule checks call presence, never call order
severity: medium
sources: bank

**Problem**: `auditFixtureCoverage` (`internal/portaltest/teardown_guard_coverage_test.go`) records four booleans and asks only whether the fixture isolates and registers the guard. A file that calls `IsolateStateForTest` and `RegisterStateDirTeardownGuard` **after** `tmuxtest.New` — the inverted-LIFO shape this phase repaired by hand three times, at `internal/restore/armed_restore_integration_test.go`, `internal/restore/integration_test.go` and `cmd/reattach_integration_test.go` — passes the rule unchanged. Separately, the five `cmd/bootstrap` fixtures routed through the shared `newIntegrationStateDir` arrange now name no `PORTAL_STATE_DIR` and call neither helper directly, so the rule's `qualifies()` is false for all of them and `helpers_integration_test.go` never starts a server, so it does not qualify either: correct by construction today, invisible tomorrow.

**Solution**: `fixtureCallsIn` already parses with positions, so compare the first server-creating call's position against the isolate and guard positions within a function and fail the inverted order. Settle in the same pass how the rule sees a fixture that reaches the helpers through a package-local arrange, so routing a suite through a shared setup stops being a way out of the guard.

## Task 12: The fingerprint backstop is narrower than its own doc and CLAUDE.md claim
severity: medium
sources: bank

**Problem**: `IsolateStateForTest` re-points `HOME` at a temp dir and clears `XDG_CONFIG_HOME` (`internal/portaltest/isolated_env.go:28-29`) *before* calling `resolveDevStateDir()`, so `devStateDir` resolves to `<tempHOME>/.config/portal/state` rather than the developer's install. `resolveDevStateDir`'s own doc comment states the contract this violates: "reads the ambient env, so it must run before any override is installed or it resolves the per-test temp dir instead of the developer's install" (`internal/portaltest/fingerprint.go:279-281`), and CLAUDE.md likewise describes the backstop as walking the developer's state dir post-test. The current ordering is *deliberate* and argued in-source (a live host daemon would otherwise false-trip the backstop mid-test) — so the guarantee is real but narrower than two places claim: it catches a process that got the HOME scrub but resolved the default path, not one that escaped the scrub entirely.

**Solution**: Make the guarantee and its description agree. This is a decision about which guarantee is wanted, not a mechanical edit, and it changes what every isolated fixture relies on.

**Decision**: Which way do the doc and the behaviour meet?
1. Correct `fingerprint.go`'s comment and CLAUDE.md to the narrower guarantee the code deliberately provides.
2. Reorder the snapshot ahead of the HOME scrub so the backstop watches the developer's real install, and solve the live-daemon false-trip another way.
3. Keep both snapshots — the pre-scrub developer install and the post-scrub temp HOME — and describe each.

## Task 13: The temp-HOME teardown race is unguarded, and its remedy already exists unshared in one fixture
severity: medium
sources: bank

**Problem**: `IsolateStateForTest` re-points `HOME` at a `t.TempDir()` and neutralises only shell *history* (`HISTFILE=/dev/null`). A restored pane's `$SHELL` writes other per-session files there — zsh's `.zsh_sessions` — and `RegisterStateDirTeardownGuard` waits on the **state** dir only, so the framework's `RemoveAll` of HOME races those writers and fails the test after its assertions passed. Observed as `TempDir RemoveAll cleanup: unlinkat .../002: directory not empty` on `TestNonContiguousWindowReboot_KeepsTokenKeyedHooks` and proved by instrumentation to be the temp HOME rather than the state dir; a second reviewer corroborated it independently. The shape is reachable from every fixture that restores a pane into an interactive shell. The remedy is already written in this repo and never promoted: `reapTmuxServer` (`cmd/concurrent_coldboot_integration_test.go:65-76`) blocks until the tmux server is unreachable for exactly this reason, and its comment names the lingering-shell-into-isolated-HOME race.

**Solution**: Promote that wait into `internal/portaltest` — registered alongside `RegisterStateDirTeardownGuard`, or as a HOME-scoped quiescence wait beside the state-dir one — so the whole class closes for every restore fixture at once rather than one suite at a time. Pinning the shell's session directory the way `HISTFILE` is pinned is the alternative worth weighing in the same pass.

## Task 14: The restore-binary Exe pin has two unguarded routes
severity: medium
sources: bank

**Problem**: The source guard added for this class covers `restore.Orchestrator` composite literals in `*_test.go` only, and two doors past it remain. `internal/restore/prefix_sibling_integration_test.go` pins a bare `restore.SessionRestorer` rather than an Orchestrator and resolves the exe by hand, so a `SessionRestorer` built without a pinned exe hits the same silent test-binary-arming failure the guard exists to foreclose. `internal/bootstrapadapter/adapters.go:64` builds a `restore.Orchestrator` without requiring `Exe` and is not a composite literal in a test file, so it bypasses the guard entirely — latent today (its only callers are `restoretest.StagedRestoreAdapter` and production), but it is the second door.

**Solution**: Close both with the proportionate shape a reviewer already established: a `restoretest.NewSessionRestorer(t, client, stateDir, binDir)` constructor plus a guard **scoped to integration-tagged test files**, not a blanket extension of the Orchestrator guard — roughly 70 sites in `session_test.go` and its siblings compose a `SessionRestorer` bare and mock-driven, where an unset `Exe` is harmless, and only the one integration fixture drives it against a live server. Require `Exe` at the adapter constructor, or guard it there.

## Task 15: Fifteen fixed-budget `PollUntil` sites are the flake class already fixed for seven
severity: medium
sources: bank

**Problem**: Task 7-9 converted seven daemon-lifecycle waits from a fixed wall-clock budget to progress-based waiting; fifteen more in the same suites were out of its scope and remain the next cliff to trip: `saverPanePIDTimeout` (`cmd/bootstrap/orphan_sweep_integration_test.go:219`), `upgradePathPIDFileTimeout` (two sites), `daemonAliveTimeout` (`cmd/state_daemon_integration_test.go:273`), `daemonReadyBudget` / `hookCleanupObservationBudget`, and eight across `internal/tmux/portal_saver_integration_test.go`, `portal_saver_endstate_integration_test.go` and `kill_barrier_escalation_no_final_flush_integration_test.go`. Each polls a monotonic observable against a fixed wall-clock budget on a machine with no CI. `internal/tmuxtest/progress_test.go:55` carries the same class of assertion in the unit lane (`got.Elapsed > wait.Ceiling+300ms`), where it is also partly redundant with the two assertions above it.

**Solution**: Convert the fifteen to the progress-based wait, and loosen the unit-lane elapsed assertion to something with scheduler headroom (a `2*wait.Ceiling` tolerance catches a mis-derived ceiling just as well). The conversion reaches across `cmd`, `cmd/bootstrap` and `internal/tmux`, which raises where the helpers should live: `AwaitProgress` and `PollUntil` reference nothing tmux-related but sit in `internal/tmuxtest` beside `socket.go` and `stamp.go`, which do.

**Decision**: Where do `AwaitProgress` and `PollUntil` live once three packages depend on them?
1. Stay in `internal/tmuxtest` — no import cycle blocks it (`internal/tmux`'s integration tests are `package tmux_test`), and moving them costs churn for a naming argument.
2. Move the pair together into a neutral test-helper home, leaving `internal/tmuxtest` holding only tmux-specific scaffolding.

## Task 16: Four `Commander` fakes outside `cmd` collapse the `Run`/`RunRaw` trim-versus-verbatim split
severity: medium
sources: bank

**Problem**: The `Commander` interface's whole reason for two methods is the trim-versus-verbatim split, and no test outside `cmd` pins it: `internal/tmux/tmux_test.go:29` records and dispatches `RunRaw` identically to `Run`, `internal/restore/session_test.go:33` does the same, `internal/restoretest/restore_marker_test.go:37` delegates `RunRaw` straight to `Run`, and `internal/state/capture_test.go:67` fatals on it (a deliberate stance, but a fourth statement of the contract). A production change to the split therefore lands silently in four packages. The executor who audited `cmd`'s seven fakes found the same absence there — the contract was not divided seven ways, it was absent seven times — and built `scriptedCommander` to honour it properly, homed in `cmd`.

**Solution**: Promote the honouring fake to a shared home beside `internal/transienttest`'s `Commander`, so one declaration pins the split for every package that fakes the interface, and re-point the four collapsed fakes at it.

## Task 17: `tmuxtest.WaitForSession` prefix-matches, so every fixture staging a prefix sibling gets a readiness guard that does nothing
severity: near-miss
sources: bank

**Problem**: `internal/tmuxtest/socket.go:110` polls `has-session -t <name>` with no `=` prefix. Measured: with only `_portal-saver-old` live, that exits 0 for `_portal-saver`. So every fixture in the repo that seeds a sibling before the session under test silently loses its settle-window guard — the same prefix-matching defect this work unit fixed in production, one layer down in the scaffolding, and it is why one task now carries a local `waitForExactSession` duplicate. Around it sit three near-identical isolated-server seed helpers in the same package (`seedSaverServer`, `seedPrefixSiblingServer`, `seedHookKeyServer` — each running the same `SkipIfNoTmux` → `tmuxtest.New` → `EnsureServer` → `NewSession` → wait sequence, differing only in socket prefix and topology) plus the matching `livePanePID` / `sessionPaneIDs` pair. `hookkey_realtmux_shared_test.go` already exists as the package's shared-fixture home.

**Solution**: Switch the shared helper to the exact form (`"="+name`) so the guard works for all consumers at once, delete the local exact-form duplicate, and collapse the three seed helpers into the package's existing shared-fixture file. Touching shared scaffolding used well beyond this package is the cost, and is why it was banked rather than taken inline.

## Task 18: The `list-panes` live-coord read is written nine ways, six of them prefix-unsafe
severity: near-miss
sources: bank

**Problem**: The same `list-panes -s -F "#{window_index}:#{pane_index}"` read is authored at nine sites across the restore corpus — `assertLivePanes` beside `livePaneCoords` in the *same package*, plus `integration_full_test.go`, `rename_reboot_shared_test.go`, `exit_closes_pane_integration_test.go`, `armed_restore_integration_test.go` and three sites in `cmd/bootstrap/reboot_roundtrip_test.go`. Only three route through `tmux.ExactCoordTarget`; the other six carry the latent prefix-sibling trap that helper exists to close, in assertions whose whole job is to prove the restore put panes where it said it did.

**Solution**: One `restoretest` reader collapses all nine and makes prefix-safety uniform rather than per-author.

## Task 19: Two daemon fixtures assert the stale seed is absent without first asserting it was present
severity: near-miss
sources: bank

**Problem**: `cmd/state_daemon_hook_cleanup_test.go:81` and `cmd/state_daemon_run_test.go:571` assert the stale key is gone after the sweep with no prior presence check. A reviewer proved by mutation in an out-of-tree copy that re-pointing `hookstest.StaleHookSeed`'s stale half at an unjudgeable key leaves **both** fixtures passing, while re-pointing the live half fails both loudly — because the live assertion is a presence check and the stale one is not. Now that the seed body lives in another package, a future edit to `StaleHookSeed` can silently defang both; the integration suite already carries the guard these two lack. Three related gaps in the same vocabulary: the two suites' single-entry seeds still say `cmd-stale` where the shared two-entry seed says `cmd-gone`, so the same concept has two spellings in one file; hand-rolled key literals (`aaa111`, `tok999`, `ghost9`) in four `cmd` suites sit outside the vocabulary and — unlike constructor-derived seeds, which panic if the pane-token width moves — would quietly become *unjudgeable*, flipping the class of every assertion built on them; and `internal/hookstest/hooks_test.go` still sweeps `for n := range 4` for its token-shape subtests while the named seeds now span indices 0-6.

**Solution**: Add the presence precondition to both fixtures, reconcile the two stale-command spellings, route the hand-rolled literals through the `hookstest` constructors so a width move stays loud, and extend the self-test's sweep to the range the vocabulary actually mints. Consider unexporting `ReapableHookKey` / `UnjudgeableHookKey` in the same pass — they have no call sites outside the package and its own self-test, so making the named seeds the only exported vocabulary would turn "no package re-derives a seed index inline" from currently-true into impossible.

## Task 20: The Go-source guard scan skeleton is re-authored at fourteen sites; two packages independently extracted their own local version
severity: duplication
sources: duplication, bank

**Problem**: `internal/sourceguardtest` owns the enumeration primitives (`GoSourceFiles`, `PackageGoFiles`) and the AST primitives (`ForEachFuncCall`, `CalleeName`) but stops short of the step between them, so every consumer writes the same ten-line loop itself: enumerate → `parser.ParseFile` per path → `t.Fatalf("parse %s: %v")` → count what was scanned → fatal when the count is zero. Eleven sites write it inline (across `cmd`, `internal/hooks`, `internal/theme`, `internal/tmux`, `internal/restoretest`, `internal/portaltest`), and three more carry variants (`internal/session/panetoken_test.go`, `internal/tmux/target_composition_guard_test.go`, `cmd/open_theme_nomination_test.go`). Two packages recognised the repetition and each extracted a private version — `internal/hooks`'s `scanPackageCalls` and `internal/theme`'s `parseThemeSources` — the same abstraction discovered twice and shared zero times. The copies have already drifted where it matters: the parse mode varies (`SkipObjectResolution` at most sites, `ImportsOnly|SkipObjectResolution` at one, bare `0` at three), and the scanned-nothing tripwire — the property that stops a guard passing by having stopped looking — is present at some sites and absent at others. A guard that silently scans nothing reports a safety it is not providing, and that invariant is currently a per-copy decision.

**Solution**: Move the parse step into `internal/sourceguardtest` beside the primitives it already owns — a `ParsedSource{Path, Fset, File}` value plus `ParsePackageSources(t, dir, includeTests)` and `ParseSources(t, paths)` that fatal both on an unparseable file and on an empty result, taking the existing `TestingT` subset so their own failure paths stay testable, with the parse mode expressed rather than assumed. Re-point all fourteen sites, delete `scanPackageCalls` and `parseThemeSources`, and re-aim `scanPackageCalls`'s coverage test at the shared helper — asserting *which* fatal it exercises, which that test currently records and never reads.

## Task 21: Four callee-name unwrappers survive beside the shared `CalleeName`
severity: duplication
sources: bank

**Problem**: `sourceguardtest.CalleeName` is the shared Ident/Selector callee unwrapper, and four functional duplicates remain outside it: `cmd/state_daemon_lock_pid_ordering_test.go:233-243` (the same two-arm switch), `internal/theme/slug_collapse_guard_test.go:83-92` (`calledName`), `internal/tmux/target_composition_guard_test.go:365-377` (`isExactTargetCall`, which folds the unwrap into a set-membership test) and `internal/tui/restore_source_guard_test.go:199-206`. Two reviewers separately checked the remaining ~30 `Sel.Name` sites across the tree and found them receiver-qualified *matchers* rather than unwrappers, so these four are the complete candidate set.

**Solution**: Collapse all four onto `sourceguardtest.CalleeName`, keeping the set-membership test in the tmux guard as its own predicate over the shared unwrap.

## Task 22: Four leaf-package guards each restate the "transitive deps confined to an allowlist" check, and its vacuity check survives in one copy of four
severity: duplication
sources: duplication, bank

**Problem**: `internal/hooks`, `internal/nanoid`, `internal/prefs` and `internal/theme` each assert the same property over `sourceguardtest.PackageDeps` — that a package's transitive dependency set stays inside a declared allowlist — and each writes the loop itself in a different shape: a `map[string]bool` in hooks, a forbidden-list plus inline switch in prefs, a single forbidden path in theme, an implicit stdlib predicate in nanoid. The failure verb differs (`Errorf` in two, `Fatalf` in two), so an offending dep aborts two guards and lets the other two carry on reporting. Most consequentially the **vacuity check has drifted**: only prefs asserts it still sees a known dep, so only one of the four cannot pass over an empty or unresolved dep set — the property the check exists to protect is present in one copy of four. A fifth hand-rolled `go list -deps` sits outside the family at `cmd/capturetool/import_guard_test.go:22`, which cannot fold in as things stand because it sets `cmd.Dir = portalbintest.ProjectRoot()` and `PackageDeps` has no equivalent knob; two reviewers surfaced it independently.

**Solution**: Add one assertion to `internal/sourceguardtest` beside `PackageDeps` — an `AssertDepsWithin(t, pkg, allowed)` that skips the package itself, reports every dep outside the allowlist, and fails when the resolved set is empty or contains no allowlisted internal dep — and reduce each of the four guards to its allowlist plus its rationale comment. Keep nanoid's stdlib-only predicate as its own allowlist form if a path list cannot express it.

**Decision**: How does `cmd/capturetool`'s copy fold in?
1. Give `PackageDeps` a module-root anchor so the capturetool guard folds in unchanged — which changes how the other four callers resolve their package argument.
2. Add an optional working-directory knob used only by capturetool, leaving the four existing callers' resolution untouched.
3. Leave capturetool's copy as a deliberate exception and say so in its comment.

## Task 23: The `logtest` accessor family is one member short in three directions, and two capture-handler twins survive
severity: duplication
sources: bank

**Problem**: Every accessor `logtest.Record` does not offer is re-authored per package. There is no non-fatal string accessor, so `attrOrEmpty` (`internal/tmux/portal_saver_test.go:1152`, consumed from `hooks_register_test.go` — a generic record helper homed in the saver file) exists alongside raw `rec.Attrs[...]` reads at `cmd/state_daemon_cycle_summary_test.go`, `cmd/bootstrap/latch_test.go` and `cmd/bootstrap/eager_signal_hydrate_test.go`. There is no duration value accessor, so two hydrate suites assert the `took` value by hand and a third re-derives `RequireDuration` exactly. `Record.HasAttr` exists but three absence checks re-derive the index (`internal/storelog/clean_stale_test.go:33`, `internal/hooks/store_test.go:1094`, `internal/state/fifo_sweep_summary_test.go:133`). `Record.ErrorAttr` exists but four sites still hand-roll the index-then-type-assert, one of them in the lossy `gotErr, _ :=` form that silently yields nil on a wrong kind. And no level-filtered chain has a terminal single-record accessor — `OnlyRecord` applies no level filter — so the `if len(x) != 1 { t.Fatalf(...) }` idiom over a filtered `Records` appears at roughly twenty sites across four packages. Two structural twins of `Sink` itself also survive: `RecordingLogger` in `cmd/bootstrap` (exported, asserted against from a second file in that package) and `errorAttrRecorder` in `cmd/state_daemon_capture_logging_test.go`, which retains one WARN's live error attr — something `Sink` already does, since it stores `slog.Value`.

**Solution**: Finish the family — an `AttrOrEmpty`, a kind-checked `DurationAttr`, and a level-preserving `Only(t, description)` terminal on `Records` — then adopt them at the sites listed above, migrate the two surviving handler twins onto `Sink`, and re-home `attrOrEmpty`'s replacement out of the saver test file. Two tidies belong in the same pass: `cmd/logging_capture_test.go:31` re-open-codes the exact expression behind `log.Discard()`, and `barrierLog := &barrierLog{}` shadows its own type at nine sites in `internal/tmux/portal_saver_lifecycle_events_test.go`, making the type name unusable for the rest of each function.

## Task 24: The general-purpose `TestingT` subset has two homes, and four packages each carry their own recording fataller
severity: duplication
sources: architecture, bank

**Problem**: `TestingT` — the subset of `*testing.T` a fatal-on-failure helper needs, so the helper's own failure path is testable — is declared independently in `internal/logtest/capture.go:23` and `internal/sourceguardtest/packagedeps.go:11`. Neither declaration has anything to do with logging or with source scanning. The consequence is that `internal/hookstest`, whose whole subject is `hooks.json` bytes, imports the *logging* capture package for two byte-comparison helpers, and `cmd`'s source-scanning seam guard does the same for a function that parses Go files; the two copies can also drift in which methods they require. A third byte-identical declaration sits at `internal/restoretest/marker_count.go:19` (`markerReporter`). The matching stand-in — a recorder that absorbs a fatal by panic-and-recover so the failure path is assertable — now exists in four copies: `fakeT`/`captureFailure` (`internal/logtest/capture_test.go`), `recordingReporter`/`fatalSentinel` (`internal/restoretest/marker_count_test.go`), `fakeFataller` (`internal/restoretest/waitfor_file_exists_test.go`, which has no `Errorf` and so cannot substitute for the others) and `recordingT`/`captureAssert` (`internal/hookstest/hooks_test.go`, whose recover is the broken one). None is importable by the others because the originals live in `_test.go` files.

**Solution**: Home `TestingT` once and export one recording fataller beside it, then re-point `logtest`, `sourceguardtest`, `restoretest` and `hookstest` at the single declaration so no package imports a logging helper for a type unrelated to logging.

**Decision**: Where does the shared declaration live?
1. `internal/sourceguardtest` — already the stdlib-only shared-primitives package, and already one of the two declaring homes.
2. A new neutral test leaf, leaving `sourceguardtest` about source scanning and nothing else.
3. Keep it in `internal/logtest` and have the others reference that one, accepting the import edge and re-scoping the doc comment.

## Task 25: The failed-write audit-trail assertion is hand-rolled at six sites whose own file-siblings already use the shared helper
severity: duplication
sources: duplication

**Problem**: `logtest.AssertRecord` / `RecordWant` exist to pin the five properties every audit-trail line shares, and the happy-path cases in all three store-logging suites were migrated onto it. The WARN-on-failed-write cases were not: `internal/hooks/store_test.go:1288` and `:1441`, `internal/project/store_logging_test.go:102`, `:219` and `:320`, and `internal/alias/store_logging_test.go:224` each still spell out the level, message, `op` and `component` checks inline, then repeat the same two-part tail — an `error_class` string check followed by `ErrorAttr` plus an `errors.Is` against a `fileutil` sentinel — at five more line numbers. One contract asserted two ways inside the same file, with the divergence invisible: the project suite's hand-rolled blocks assert no `via` at all, so the failed-write breadcrumb's `via` attr is unpinned on three of the four project mutations while every successful one pins it.

**Solution**: Route the six blocks through `logtest.AssertRecord` exactly as their siblings do, and lift the recurring tail into one shared assertion beside it — an `AssertWriteFailure(t, rec, wantClass, sentinel)` checking the `error_class` value and that the carried error wraps the named `fileutil` sentinel — so the failed-write contract lands in one place for hooks, projects and aliases alike.

## Task 26: The hooks-store lock WARN is asserted by two independently written helpers, plus three inline restatements of its negative half
severity: duplication
sources: duplication

**Problem**: Both suites assert the same emission — the single WARN `hooks.Store.Set`/`Remove` leaves when the sidecar acquire times out — through two helpers written independently and named almost the same thing: `assertLockWarn(t, sink, wantOp, wantKey, wantVia)` (`internal/hooks/lock_write_test.go:18`, 26 lines hand-rolling level/msg/op/component/via/hook_key/error) and `assertOneLockWarn(t, sink, wantOp, wantKey)` (`cmd/hooks_write_lock_test.go:45`, which pins the same properties through `logtest.AssertRecord` and additionally requires exactly one WARN). Neither covers what the other adds. The negative half of the contract — that a lock WARN carries no `error_class` and no `value`, because no write phase ran — is then restated verbatim three more times as inline attr-presence checks. `internal/hookstest` is already the cross-package home for exactly this shape: it owns `AssertDegradedRead`, the sibling assertion for the DEBUG `load-unlocked` breadcrumb, alongside the `HoldHooksSidecar` fixture both suites use to produce the timeout.

**Solution**: Add `hookstest.AssertLockWarn(t, sink, op, key, via)` next to `AssertDegradedRead`, covering the whole line in one call — level, message, `op`, `component=hooks`, `via`, `hook_key`, a non-empty `error`, and the absence of `error_class` and `value` — and have both suites call it, deleting the two local helpers and the three inline tails. Keep `cmd`'s exactly-one-WARN check at its call site if that count is specific to the command path.

## Task 27: There are four `hooks.json` staging routes where the code claims two, and the path composition is hand-rolled fifty times
severity: duplication
sources: duplication, bank

**Problem**: `hookstest.StageStore` is the described route for handing a staged `hooks.json` to a seam, and `cmd/testhelpers_test.go:150` documents `hooksFileInTempDir` as "the second of the two staging routes". There are four: `StageStore`, `hooksFileInTempDir` (plus `writeHooksJSON`), and `seedHookStore` (`cmd/state_hydrate_test.go:1234`) — a private marshal-and-write driven from roughly nineteen call sites across three hydrate suites, which stages **no sidecar**, so every hydrate fixture's `LookupOnResume` takes the degraded unlocked-read path while a `StageStore` fixture takes the shared lock. The doc comment asserting there are two routes is already false, which is how the drift stayed invisible. Underneath, the path composition itself is inline: 36 `filepath.Join(dir, "hooks.json")` sites in `internal/hooks/store_test.go` (8 paired with an `os.WriteFile` seed that `Staging{Seed}` now expresses) and 12 in `lock_write_test.go`, whose subject is genuinely the sidecar's absence and its creation, so they want a path-only sibling rather than the full stager. `internal/hookstest`'s own self-test hand-rolls two of the axes its package owns. A fifth, smaller instance of the same reach: `readFileBytes` (`cmd/testhelpers_test.go:114`) re-implements `hookstest.HooksFileBytes` — the same ENOENT-tolerant read, already path-generic and already taking the `TestingT` subset — in a file that imports `hookstest` and delegates its neighbouring assertion to it.

**Solution**: Give the package one stager and one path-only sibling, fold `seedHookStore` and the inline compositions onto them, delegate `readFileBytes` to the shared read (renaming it if the hooks-flavoured name is what discouraged reuse), and make the "two routes" claim in `cmd/testhelpers_test.go` true again.

**Decision**: What does folding `seedHookStore` in do to the hydrate fixtures' sidecar?
1. Stage the sidecar by default everywhere — uniform, but it changes the hydrate fixtures' read path from degraded-unlocked to locked, and injects a sidecar into fixture dirs that also hold FIFOs and scrollback.
2. Fold it on with an explicit sidecar-absent option for the hydrate suites, preserving today's behaviour while removing the fourth route.
3. Add the path-only sibling for the inline sites and leave `seedHookStore` alone, accepting three routes and correcting the doc comment to say so.

## Task 28: The shell-quoting rule has three separately-maintained homes
severity: duplication
sources: duplication

**Problem**: `internal/restore/session.go:362`'s `shellQuoteSingle` and `internal/spawn/recipe.go:67`'s `shellQuote` are the same function to the byte — `return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"` — introduced independently in different packages, and `internal/session/create.go:29` performs the same close-escape-reopen a third time inline without wrapping it at all. This is a correctness rule, not a formatting convenience: all three compose a string a shell will later word-split (a tmux `respawn-pane` command, a `{command}` handed to a terminal recipe, a tmux shell-command), and the project's own architecture notes already record that naive concatenation on the spawn path corrupts the command. Three copies is three places for it to be fixed once and stay wrong twice.

**Solution**: Give the rule one home the three callers can reach, delete the two helpers in favour of it, and have `BuildShellCommand` call it rather than inlining the replace.

**Decision**: Where does the rule live? `internal/spawn`, `internal/restore` and `internal/session` share no leaf today — `tmuxout` is tmux-output parsing and `nanoid` is the id vocabulary.
1. A small stdlib-only leaf in the same mould as `nanoid`, holding the quote function and — if it is worth moving with it — the existing `renderCommandString` join.
2. Export the rule from `internal/spawn`, which already owns the composed-argv surface, and have `restore` and `session` import it.

## Task 29: Two inline copies of a stand-down phrase bypass the vocabulary that exists to own it
severity: duplication
sources: duplication, bank

**Problem**: `cmd/run_hook_stale_cleanup.go` declares two surface vocabularies for stand-down copy, `skippedPrunePhrases` and `notEvaluableDetails`, and `restoreStandDownPhrase` was extracted as a const precisely because one phrase was shared between them. Its siblings were not: `"could not read hooks.json"` is authored in both maps and `"could not enumerate live panes"` likewise. Worse, `checkStaleHooks` re-authors the first a third and fourth time as bare literals in its two early returns (`cmd/doctor.go:295` for a nil store, `:299` for a failed `Load`) rather than rendering through `phraseFor(notEvaluableDetails, skipReasonStoreReadFailed)` the way its later returns do. Two guards cover the vocabularies and the rendered lines are pinned, but neither can see a literal that duplicates a map *value* — the guards bind map *keys* — so re-wording the entry leaves those two branches printing the old copy for the same condition, silently. (The literals arrived from a different work unit, which is why no task in this one owned them.)

**Solution**: Have both early returns render through `phraseFor`, and lift the two strings shared between the maps into consts beside `restoreStandDownPhrase` so each phrase is written once and both maps compose from them.

## Task 30: `cmd` carries a second seam family — nine package-level function vars installed by hand at 113 sites
severity: duplication
sources: bank

**Problem**: `runOpenBurstFunc`, `openRawArgs`, `openTUIFunc`, `openPathFunc`, `openSessionFunc`, `daemonTickLoopFunc`, `osExit`, `hydrateRunFunc` and `signalHydrateRunFunc` are installed across 21 test files at roughly 113 sites, each a bare assignment plus a separate `t.Cleanup` — exactly the pattern the `*Deps` seam helper closed for the struct seams, and exactly the same leak vector when a cleanup is forgotten. They are invisible to the new seam guard because `declaredSeams` matches only `var xDeps *XDeps`. The shape differs in one way that matters: the production default is a real value rather than nil, so the helper must capture-and-restore rather than restore-to-nil, and the guard's recogniser needs a second arm.

**Solution**: Give the function-var family the same install helper and the same derived guard the struct seams got, so a seam declared tomorrow is guarded the day it appears and a forgotten restore cannot leak into a later test.

## Task 31: The stand-down copy is pinned in four files and the same case is written twice in two more
severity: duplication
sources: bank

**Problem**: `cmd/doctor_stand_down_copy_test.go` was meant to be the one place the stand-down copy is pinned, and it is not: `:142-145` duplicates `cmd/doctor_test.go:1568-1570` exactly (same helper, same lister, same expected string), `:138-140` duplicates `cmd/run_hook_stale_cleanup_outcome_test.go:93-99` in substance, and two more of the same lines are pinned in the lock-timeout and single-report suites — so a wording change needs four edits. The surrounding corpus repeats itself the same way: `TestHookSweepOutcomeNamesEveryDecline` tables all five decline reasons with a uniqueness check while `run_hook_stale_cleanup_test.go` separately pins four of the same five one at a time and the lock-timeout suite pins the fifth, each half carrying assertions the other lacks; the lock-timeout single-report case exists twice in two files from two tasks with the stronger assertions split between them; and one new doctor-print subtest duplicates a pre-existing one outright, staging the identical fixture, driving the identical command and making the identical assertion. Two smaller notes belong with the pass: `bogusHooksStore` lives in an outcome test file while `testhelpers_test.go` declares itself the home for staging helpers and is now consumed from three files, and `cmd/doctor_test.go:869`'s projects read has no existence precondition — not vacuous today only because the next assertion is positive, and it would fail as "live project wrongly pruned", misattributing a deleted file.

**Solution**: Give each decline reason one home carrying both halves of its coverage — the log level and attrs *and* the rendered line *and* the hooks.json-untouched check — delete the outright duplicate, fold the stronger of each split pair into the survivor, and move the staging helper to the file that claims it. One `borrows` check needs correcting while the file is open: it compares the *table's* expected values rather than the rendered ones, so it catches a copy-paste author rather than two production reasons converging on one phrase (proven: re-pointing a map entry at another reason's phrase was caught by the exact-line assertions and not by that check).

## Task 32: `internal/restore` fixture residue from this phase
severity: duplication
sources: bank

**Problem**: Parameterising the multipane arrange left `newLegacyFixture` and `newRenameRebootFixture` as the same fixture with a pane loop as the only difference — identical struct shape, identical `captureAndPersist` / `rebootAndHydrate` method names and bodies. The convergence was created by that task and left unresolved by it because merging reaches into three files it did not scope. Separately, `openTestLogger(t, dir)` in `restore_test.go` survives as a pure pass-through whose `dir` argument nothing reads, left behind when its consumers were routed at `logtest.NewCaptureLogger` directly.

**Solution**: Merge the two fixtures into one taking a pane count, and drop the parameter nothing reads.

## Task 33: Four command-driver twins and four call-log filters in one package
severity: duplication
sources: bank

**Problem**: `runStateCommitNow`, `runStateDaemon` and `runStateNotify` are three literally identical twelve-line bodies — same `t.Helper()`, two buffers, `resetRootCmd()` + `resetStateCmdFlags()`, `SetOut`/`SetErr`, `Execute` — differing only in the second argv element; `runUninstall` is the same body again in the variadic form, minus the state-flag reset. A single `runRootCmd(t, args ...string)` absorbs all four, with `resetStateCmdFlags()` the only genuine divergence and a candidate to fold into `resetRootCmd` alongside the flag resets already there. Beside them, four parallel filters over `[][]string` call logs coexist in the same package: `countOp`, `callIndex`, `setHookCalls` and `callsMatching` — the last of which currently has no consumer outside its own contract test, which the other three make plausible.

**Solution**: Collapse the four drivers onto one and the four filters onto one, which resolves the unconsumed helper at the same time.

## Task 34: The `hook rm` test corpus carries cross-file duplicates and one staging shape written three ways
severity: duplication
sources: bank

**Problem**: `cmd/hooks_test.go:617` and `cmd/hooks_rm_exit_test.go:149` are the same test with different key names — both seed a target plus a sibling, resolve the target, and assert removed-plus-sibling-survives; `cmd/hooks_test.go:497` is a strict subset of `:617`; and `cmd/hooks_test.go:590` and `cmd/hooks_rm_exit_test.go:116` both pin the no-hook-registered message on the resolved path. Two assertions in that set are genuinely unique and must survive any collapse: the raw-`%42`-not-used-as-key check and the last-event cleanup that asserts the file empties. Separately, both subtests of `TestHookRmLockTimeout` hand-roll the staging sequence `runRmCase` now owns, differing only in holding the sidecar lock and needing the captured stdout. And the `rmCase` row struct has two field combinations that are silently meaningless or panic — a row setting the pane-key path *and* a resolver silently discards the resolver, and a row setting neither passes a typed-nil through the interface, panicking inside the mock rather than falling back to the production default.

**Solution**: Collapse the duplicated cases into one home each, preserving the two unique assertions; give `rmCase` a lock-hold flag and `rmOutcome` a captured-output field so the lock-timeout subtests route through it; and close the two meaningless field combinations so the struct cannot be filled in a way that means nothing.

## Task 35: Each config file's identity is restated at every call site, and two production sites restate the resolution rule
severity: duplication
sources: bank

**Problem**: The precedence rule now has one home in `internal/xdg`, but the pair that *identifies* each config file — its env var and its filename — is still written out per site: `cmd/hooks.go:256` passes the literals, `cmd/config.go` re-lists the filenames in `configFileComponents`, and `internal/hookstest/hooks.go` declares its own copy of the same pair. Renaming an env var is therefore an N-site edit that reaches into a test-only package. Separately, two production sites re-author the shape the rule single-sourced for config *files*: `internal/state/paths.go:28-35` (`PORTAL_STATE_DIR` then `<base>/portal/state`) and `cmd/config.go:208-217` (`PORTAL_THEMES_DIR` then `<base>/portal/themes`), so the `portal/` literal now sits in three production sites.

**Solution**: Decide where a config file's identity lives — an exported table beside `configFileComponents`, or per-owning-package constants — and route `cmd/alias.go`, `cmd/hooks.go`, `cmd/config.go`, `internal/hookstest` and `internal/restoretest` through it. Bring the state-dir and themes-dir resolvers onto the shared rule too, respecting the themes directory's deliberate carve-out (a `_DIR` rather than a `_FILE`, with no migration).

## Task 36: The declared-once hydrate-wait invariant is half-applied
severity: drift
sources: bank

**Problem**: `restoretest.HydrateBudget` / `HydrateTick` exist so the hydrate wait is declared once, and three families sit outside it. Two holdouts in `internal/restore` still pass the raw `10*time.Second, 50*time.Millisecond` pair (`integration_full_test.go:128`, `exit_closes_pane_integration_test.go:169`). The `cmd/bootstrap` hook-fire family uses a deliberately shorter `2s`/`50ms` pair at five sites, which is not the racy shape — routing it through the shared pair would change behaviour, so the invariant is left half-applied rather than decided. And `TestAssertMarkerCount_ExportedEntryPointUsesTheSharedBudget` writes its marker at 100ms, so any budget above ~150ms passes: the name claims a distinction the test does not draw. One related note for the same pass: the hydrate helper unsets the skeleton marker *before* the exec, so any test treating marker-clear as proof the hook ran is racing the hook's own runtime; `cmd/noncontiguous_window_reboot_integration_test.go:399-402` is safe against absence but pins no count, so a cross-fire would pass.

**Solution**: Bring the two restore holdouts onto the shared pair, make the marker-count test's name true by moving its write past the probe budget, and settle the `cmd/bootstrap` question rather than leaving the invariant partly applied.

**Decision**: What happens to the deliberately shorter hook-fire budget?
1. One named pair for everything — `cmd/bootstrap` routes through it and its waits lengthen.
2. A second named short pair for the hook-fire family, so both budgets are declared once and the difference is stated rather than implied.
3. The `cmd/bootstrap` locals stay as deliberate exceptions, documented at the declaration.

## Task 37: `Client.SendKeys` has no production callers and is exported anyway
severity: dead-code
sources: bank

**Problem**: `internal/tmux/tmux.go:638` is called only from its own unit test and from real-tmux integration drivers in two other packages (`cmd/bootstrap/eager_signal_hydrate_integration_test.go`, `internal/restore/exit_closes_pane_integration_test.go`), where it types into live panes. That makes it materially unlike the pane-listing methods deleted alongside it — those had genuinely no consumer, this one is a shared test driver that happens to live on the production client. The claim made when the family was pruned, that the tmux client exports no method with zero production callers, is therefore not literally true tree-wide.

**Solution**: Take the decision deliberately rather than leaving it as an assumption either way, and make whichever answer is chosen visible at the declaration.

**Decision**: Does `SendKeys` stay?
1. Keep it, documented at the declaration as a sanctioned shared test driver with no production caller, and narrow the tree-wide claim to the pane-listing family it was made about.
2. Delete it and rewrite the three integration drivers to drive tmux directly, restoring the claim in full.

## Task 38: `ResolveSessionDir` classifies sentinels its only production implementation cannot emit, for a caller that discards the error
severity: dead-code
sources: bank

**Problem**: Now that `ActivePaneCurrentPath` is established to answer an unmatched target with an empty expansion at exit 0, `*tmux.Client` cannot produce `ErrNoSuchSession` on this path — yet `internal/session/dirresolve.go` retains its `errors.Is(err, tmux.ErrNoSuchSession) || errors.Is(err, tmux.ErrEmptyPaneList)` branch. The executor kept it deliberately, as classification of the `PaneCurrentPathReader` *interface* contract rather than a claim about what tmux emits, and it has real coverage through a fake reader. The other end makes the question sharper: the seam's only consumer, `internal/tui/model.go:1163`, tests `ok && err == nil` and drops the error, so the unresolvable-versus-fatal distinction the branch draws is invisible — every non-ok outcome degrades identically.

**Solution**: Settle whether a seam should classify sentinels its only production implementation cannot emit and its only caller does not read.

**Decision**: What happens to the branch?
1. Keep it as interface-contract hygiene, with the annotation and test naming that already say no production client produces it — and accept that the distinction is currently unread.
2. Drop the branch and let the empty-path signal be the whole contract, simplifying the seam to what its caller consumes.
3. Keep the branch and make the TUI consume the distinction, so the classification earns its place.

## Task 39: CLAUDE.md's `logtest` row describes a surface that no longer exists
severity: comments
sources: bank

**Problem**: The row states that records are read through "the chainable `Records` filters (`AtExactLevel` / `AtOrAboveLevel` / `Msg` / `With`)" — those four are now unexported (`atExactLevel`, `atOrAboveLevel`, `withMessage`, `matching`), so a contributor writing against the row's names hits a compile error. It also ends with "`internal/restore`'s is the last one left", naming a wrapper that no longer exists: `internal/restore/logging_capture_test.go` was deleted, and the row now carries a false claim of exactly the kind the consolidation existed to remove. A third, smaller inaccuracy: the row's hands-back sentence does not mention that `NewCaptureLogger(t)` returns the sink alongside the logger.

**Solution**: Correct all three so the row describes the shipped surface. The filter-name clause is wrong as of today whichever way the query-surface question is settled, so this stands on its own; if that direction changes the surface, the row is corrected once more at that point rather than left wrong in the meantime.

## Task 40: `snapshotLockBound`'s justification lost its concrete "why" and gained a vacuous clause
severity: comments
sources: bank

**Problem**: The comment on `snapshotLockBound` (`internal/hooks/lock.go:29-45`) says the pre-read "is bounded at the cheapest figure that still grants an uncontended lock". A reviewer probed `acquireLock` directly against a held exclusive lock and found the first `Flock` attempt *precedes* any deadline check — so an uncontended acquire cannot time out at **any** bound, which makes the clause vacuous: every figure grants it. The same probe produced the stronger argument the comment should carry, that a short pre-read bound introduces no spurious-degradation surface at all. Meanwhile the concrete justification the prior comment carried — "four poll intervals above the sub-millisecond critical section", the sentence explaining why a hundredth rather than a thousandth — was dropped in the rewrite. A third clause asserts that the derivation "bounds the clean pre-read below the mutation bound" over three hand-picked values, a property that is false below roughly 10ms given the floor: a three-point sample dressed as an invariant.

**Solution**: Replace the vacuous clause with the loop-ordering argument that actually makes the short bound safe, restore the concrete figure justification, and either scope the below-the-mutation-bound claim to the range where it holds or drop it.

## Task 41: Two leaf packages have no guard, and one of them now underwrites a test-isolation guarantee
severity: low
sources: bank

**Problem**: `internal/xdg` and `internal/sourceguardtest` are both stdlib-only today (confirmed by `go list -deps`) and neither property is pinned, unlike `internal/nanoid`, `internal/prefs`, `internal/hooks` and `internal/theme`, which each pin their dependency set. Both have since acquired a reason to be pinned: `internal/hookstest` now depends on `internal/xdg` for seed/read path parity, so its leaf property underwrites a test-isolation guarantee rather than just tidiness; and `sourceguardtest`'s stdlib-only, untagged contract — stated in its own doc and in CLAUDE.md — is what keeps roughly twenty source guards in the unit lane, while the package now shells out via `os/exec`, making the next addition likelier to reach for a non-stdlib helper.

**Solution**: Add the two guards modelled on `internal/nanoid/leaf_guard_test.go`, over `sourceguardtest.PackageDeps` — roughly fifteen lines each, and the primitive already exists.

## Task 42: `logtest.Sink`'s query surface grows multiplicatively because the composable filters were closed off
severity: low
sources: architecture, bank

**Problem**: `Records` has four filter methods — `atExactLevel`, `atOrAboveLevel`, `withMessage`, `matching` — all unexported, with the stated rationale that a caller must never have two ways to ask one question. The cost is that every *combination* of those orthogonal dimensions needs its own exported `Sink` method, and two already exist purely as one-line compositions of others. The surface stands at seven `Records*` methods for two-and-a-bit dimensions and grows multiplicatively: the next axis doubles it rather than adding one, and the next at-or-above-level-plus-message need adds an eighth method rather than a composition. The single-route property that motivated the closure is worth keeping, but it is achievable by exporting the orthogonal filters and *not* exporting combinations, rather than the reverse.

**Solution**: Whichever direction is chosen, two test names are stale for it — `TestRecords_FilterChainCombinesLevelAndComponent` and `TestRecords_MsgFiltersOnMessageAloneAcrossComponents` both name the chainable surface that was unexported — and should be corrected in the same pass.

**Decision**: Which way does the single-route property get held?
1. Export the four filters and keep only the base `Records()` plus the `OnlyRecord*` assertions, so callers compose the one route per question themselves and the combination methods disappear.
2. Keep the closed combination surface as directed, accepting the multiplicative growth as the priced cost of one route per query.

## Task 43: `--pane-key`'s help text and README narrow the flag below the contract it was given
severity: low
sources: standards

**Problem**: The flag help reads `Pane token of the pane whose hook should be removed (defaults to the current pane)` (`cmd/hooks.go:310`) and README:191 says it removes "a hook for any pane token". The flag is a literal pass-through with no validation of any kind, and it carries a specific second purpose: it is the route by which an old-format, non-token entry — the class the shape-aware reaper retains forever, by design, because reaping it would be a guess — is removed by hand. An old-format key is not a pane token, so a user following the help text has no indication that the one sanctioned removal route for a retained entry is this flag. CLAUDE.md gets this right; the CLI's own help and the README do not.

**Solution**: Reword the flag help and the README sentence to say the flag takes any hook key verbatim, defaulting to the current pane's token, so the retention design's escape hatch is discoverable from the CLI itself.

## Task 44: The sweep enumerates every pane on the server before checking whether there is anything to sweep
severity: low
sources: architecture, bank

**Problem**: `liveTokenEnumeration` (`cmd/run_hook_stale_cleanup.go`) runs `judgeAgainstLivePanes` — a whole-server `list-panes -a -F` — and only afterwards short-circuits on `len(snapshot) == 0`. The comment on that short-circuit reasons carefully about the three costs it avoids (creating the config directory, creating the sidecar, taking an exclusive hold), but the tmux read it has already paid is the most expensive of the four, and on an install that has never registered a hook it can change nothing: with an empty snapshot there is no key to judge and none to protect, whatever the enumeration returns. The daemon pays it every ten seconds for the lifetime of the process. The cheapest and most decisive test is ordered last. Two smaller items in the same file: `liveTokenEnumeration` calls `logger.Debug` while the nil-logger default stays behind in `runHookStaleCleanup`, so an in-package caller passing nil now panics where the inline closure made that unreachable; and `emit()` followed by `return sweepOutcome{DeclineReason: …}` appears at four sites.

**Solution**: Take the empty-snapshot branch before the enumeration, and either drop the counts DEBUG for that case or emit it with the entry count alone — the pane count has nothing to report about a cycle that was never going to judge anything. Default the logger inside the factory, and collapse the four emit-then-return sites into one helper.

## Task 45: Two stand-down reasons cannot reach the surface that renders them
severity: low
sources: bank

**Problem**: `lock-timeout` and `store-read-failed` cannot be reached through `checkStaleHooks` as the code stands: a lock timeout on a read degrades to an unlocked read rather than standing down, and a failed `Load` is reported with its own hardcoded detail rather than routing through the vocabulary. Their diagnosis lines are pinned through the real renderer over a synthetic result, so the assertions read the user's line — but the path is not exercised end to end, and the enum currently carries members the diagnosis surface can never produce. (The `store-read-failed` half is closed by routing the hardcoded literals through the vocabulary; this is about what remains after that.)

**Solution**: Settle which it is, and make the code say so.

**Decision**: Should the unreachable reasons become reachable?
1. Yes — the degrade-to-unlocked behaviour on the diagnosis path is then the bug, and the read failure should stand the diagnosis down with its own reason.
2. No — record at the declaration that the vocabulary is deliberately complete rather than fully reachable, so a reader does not go looking for the missing path.

## Task 46: The target-composition guard's exemption is wider than its recognition
severity: low
sources: bank

**Problem**: The guard recognises an already-composed target purely by the *parameter name* (`target`, `paneID`), so a client method naming its parameter anything else silently leaves its call sites unchecked, and the exemption side is wider than the recognition side: `targetTakingMethods` skips functions with no receiver while `bindParams` exempts the parameter in **any** function, so a bare target reaching the client through a non-method helper produces no finding — staged and confirmed silent. Three further gaps sit in the same family: the wrong-helper subclass is uncaught (the guard enforces vocabulary membership, not target *kind*, so `list-panes -t ExactSessionTarget(x)` — the original defect: right vocabulary, wrong kind, still reaching the prefix sibling — passes); the binding is name-based and flow-insensitive, so reassignment and branch-dependent assignment both pass; and the `-t ends its argv` branch has no fixture, leaving the split-composition detector itself unexercised. The name-based rule has already cost something concrete: a parameter had to be renamed from `liveTarget` to `target` purely to satisfy the allow-list, losing the live-versus-saved distinction the old name carried. Two cosmetic residues: the exported vocabulary is split between two naming shapes (`ExactSessionTarget`/`ExactCoordTarget` prefix, `PaneTargetExact`/`windowTargetExact` suffix), and `exact_target_internal_test.go` is now an internal test for two exported functions, so `internal` in its filename has gone stale.

**Solution**: Close the gap between what the guard recognises and what it exempts, add the missing fixture for the split-composition branch, and unify the vocabulary's naming shape while the allow-list is being edited (it is name-keyed, so the two changes land together).

**Decision**: How is the recognition made reliable?
1. The verified cheap fixes only — drop the no-receiver skip and accept an identifier callee (prototyped: the staged probe was caught, with zero new findings on the real tree).
2. A named `type Target string` on the client's target-taking method signatures, which makes a bare string target a compile error at every call site in `cmd`, `internal/state` and `internal/restore` and subsumes the flow-insensitivity and the laundering gap — a signature change across the client's whole surface.
3. The cheap fixes now, with the named type recorded as the durable answer.

## Task 47: Hydrate-config residue: one fall-through has no test and one literal survives the builder
severity: low
sources: bank

**Problem**: `cmd/state_hydrate.go:122` and `:137` are the nil-`HandleFileMissing` fall-through branches, and neither has a test — at HEAD or now. They are unreachable in production (the cobra wiring always sets the handler), so this is a latent gap rather than a live one, but the *sibling* fall-through (nil `HandleTimeout`) does have a dedicated test, deliberately preserved as an inline literal precisely because it is the standing proof that path still exists; the file-missing side has no such proof. That surviving inline literal is also why the builder's stated outcome — a new required `hydrateConfig` field is added once rather than 52 times — is really two places, not one: the builder structurally cannot express an explicit nil. One further case outside the three converted suites carries the same load-bearing nil `HookStore` without saying so.

**Solution**: Give the file-missing fall-throughs the standing proof their sibling has, make the remaining implicit nil explicit, and settle whether the builder should grow a way to say explicit-nil — or state the two-places figure wherever the one-route claim is made.

## Task 48: The stand-down reason vocabulary is closed by naming convention, not by the type system
severity: low
sources: bank

**Problem**: `declineDebug` and `declineWarn` take `reason string`, so a raw literal or an off-convention const name compiles. A reviewer probed it: a const named without the `skipReason` prefix, absent from the enumerable slice and from both phrase maps, passes **both** guards — because the source guard matches on the name prefix. The alternative was offered when the vocabulary was closed and not taken: a small named type would close the hole at compile time rather than by convention. The limitation is inherent to every name-based source guard in the repo, so the decision generalises beyond this vocabulary.

**Solution**: Make the vocabulary a `type skipReason string` so an off-convention or literal reason cannot compile, accepting that it touches the decline ladder and the typed outcome shipped by earlier tasks.

## Task 49: Two stated outcomes are held by contributor discipline in a repo whose house style is source guards
severity: low
sources: bank

**Problem**: Two rules established this phase have no structural enforcement in a repo carrying roughly twenty source guards. First, the Install-only rule: `logtest.Install(t)` is asserted to be the only route to a package-level capture handler, paired by construction at every site, but a contributor can write the two lines by hand and nothing objects — a guard would need to encode the three sanctioned survivors (the pre-`Init` discard silencer, the level-gate handler, the JSON-rendering handler). Second, the widened lane rule: a test that *builds* a portal binary now belongs in the integration lane, asserted in four prose places in CLAUDE.md and nowhere else. The second is the sharper one, because verifying it empirically is awkward — a reviewer had to instrument the builder directly, since a PATH shim on `go` does not work (Go 1.24+ prepends the toolchain dir for test binaries) — so the guard would also replace a check that is otherwise hard to run.

**Solution**: Add both guards over the existing source-guard primitives: one failing when `log.SetTestHandler` is called with a `logtest.Sink` outside the sanctioned sites, one failing when an untagged `*_test.go` references the portal-binary build helpers.
