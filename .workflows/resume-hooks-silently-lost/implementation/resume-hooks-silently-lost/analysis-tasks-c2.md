# Analysis Tasks: resume-hooks-silently-lost (Cycle 2)

## Task 1: A colon-bearing session name is now silently dropped from `sessions.json`
severity: high
sources: bank

**Problem**: `Client.ShowEnvironment` (`internal/tmux/tmux.go:585`) now composes `show-environment -t =<session>`. tmux accepts `:` in a session name, and the exact form fails on one: measured on tmux 3.7c on an isolated socket, with a live session literally named `a:b`, `show-environment -t =a:b` exits **1** with `no such session: =a:b`, where the pre-change bare `-t a:b` exited 0 and returned 14 lines. That stderr matches `noSuchSessionStderrSubstr` (`internal/tmux/errors.go:30`), so `internal/state/capture.go:69-73` classifies it as **natural churn**, logs a WARN naming a "vanished session", and `continue`s — the session is omitted from the capture. It is therefore never saved and never restored. The failure moved from *mis-targeted* to *silently absent*, which is the exact class this work unit exists to remove, arriving through the change the work unit made. Portal's own `GenerateSessionName` strips `:`, so the exposure is a user-renamed session — and `internal/tui/model.go`'s rename path passes the typed name straight through with no validation, as does `Client.RenameSession`.

**Solution**: Close the gap at whichever end the walk chooses (see Decision) so a live session with a colon in its name is captured and restored again. Whichever end is taken, add a real-tmux regression test that creates a colon-named session, runs a capture, and asserts the session is present in `sessions.json` — the current suite has no colon-named session anywhere, which is why the change shipped green.

**Decision**: Where does the fix belong?
1. Reject `:` in a session name at the write boundary — validate in `Client.RenameSession` and the TUI rename path, so a name the target vocabulary cannot round-trip never enters the server. Narrowest blast radius; changes a user-facing behaviour (a rename is refused).
2. Make the target vocabulary round-trip a colon — quote or escape the session component in `exactTarget`/`exactCoordTarget`/`PaneTarget` so every command reaches the right session whatever the name. Widest fix, touches every target site, and `StructuralKeyFormat` still cannot round-trip such a name.
3. Stop treating this class of `show-environment` failure as natural churn — narrow the classification in `internal/state/capture.go` so an unresolvable-by-our-own-target-form session becomes an anomalous error rather than a silent drop. Fixes the silence without fixing the targeting.

## Task 2: The `_portal-saver` pane probe uses the helper the package's own doc calls wrong, and resolves a prefix sibling
severity: high
sources: bank

**Problem**: `internal/tmux/saver_pane_pid.go:13` and `:37` pass `exactTarget(sessionName)` to `list-panes -t`. `list-panes` takes a *window* target, so `=_portal-saver` is not an exact session reference. Measured on tmux 3.7c on an isolated socket: with only `_portal-saver-old` live, `list-panes -t =_portal-saver -F '#{pane_pid}'` returns **the sibling's pid with exit 0**, while `list-panes -t '=_portal-saver:'` correctly fails with `can't find session`. The `exactTarget` doc comment this work unit itself wrote (`internal/tmux/tmux.go:416-421`) says exactly this — "It is the wrong helper for a `-t` tmux parses as a window or pane target … list-panes then falls through to the same fuzzy lookup and reaches the prefix sibling anyway … Route those through `exactCoordTarget`" — so the package documents the defect and the two call sites contradict it. `SaverPanePIDOrAbsent` is the tri-state feeding the bootstrap orphan sweep, the daemon's Component D self-supervision probe and the `_portal-saver` end-state assertions: a prefix-sibling saver session can make a divergent daemon believe it is still the live saver pane process, or make the orphan sweep spare the wrong pid.

**Solution**: Route both sites through `exactCoordTarget`, and cover them with a real-tmux subtest that stages a prefix sibling (`_portal-saver-old` live, `_portal-saver` absent) and asserts the read fails rather than returning the sibling's pid. While there, resolve the second half the reviewer measured: `wrapNoSuchSession` at `:15` cannot fire on this path — `list-panes` emits `can't find session` / `can't find window`, never `no such session` — so `SaverPanePIDOrAbsent`'s documented collapse of `ErrNoSuchSession` into `present=false` is dead on this route and the tri-state's real absence signal is something else. Either widen the sentinel's stderr match to cover `list-panes`'s wording or correct the contract to describe what actually happens.

## Task 3: Restore and reattach integration fixtures escape the mandated state isolation
severity: high
sources: bank

**Problem**: A class of fixtures starts a real tmux server and drives a real restore whose panes exec the hydrate helper, using a bare `t.TempDir()` + `t.Setenv("PORTAL_STATE_DIR", …)` and never calling `portaltest.IsolateStateForTest`. Verified live: `internal/restore/armed_restore_integration_test.go:22-24`, `internal/restore/integration_test.go:53-55`, `internal/restore/integration_full_test.go:41,68`, `internal/restore/exit_closes_pane_integration_test.go:110,120`, `cmd/reattach_integration_test.go:80-82`, `cmd/bootstrap/eager_signal_hydrate_integration_test.go`, `cmd/bootstrap/phase2_hook_fire_integration_test.go`, `cmd/bootstrap/phase5_integration_test.go`, `cmd/bootstrap/phase5_marker_suppression_integration_test.go`, `cmd/bootstrap/scrollback_resumption_test.go:56`. They therefore get no `HOME`/`XDG_CONFIG_HOME` scrub, no fingerprint backstop, no cross-process daemon-pgrep sandbox registry, and no teardown guard — CLAUDE.md's ABSOLUTE INVARIANT surface. The new coverage guard (`internal/portaltest/teardown_guard_coverage_test.go`) is structurally blind to every one of them because its rule keys on `IsolateStateForTest`. Three are strictly worse than the shape task 6-17 fixed: `armed_restore_integration_test.go`, `integration_test.go` and `reattach_integration_test.go` call `tmuxtest.New` **before** `t.TempDir()`, so LIFO runs the state-dir `RemoveAll` *before* `kill-server`. Two adjacent gaps ride the same fix: `IsolateStateForTest` re-points `HOME` at a `t.TempDir()` but does not set `HISTFILE`, so tmux-hosted shells race the same `RemoveAll` writing shell history — worked around locally at `cmd/abridged_integration_test.go:32` and `cmd/concurrent_coldboot_integration_test.go:41,380` with three separate comments; and `cmd/bootstrap/composition_e2e_harness_integration_test.go:58` loses the shared teardown helper's wait-for-the-biggest-writer phase entirely, because the harness deliberately overwrites `daemon.pid` with a pid it SIGKILLs earlier in the LIFO chain, leaving only the 50ms-sampled directory-quiescence loop against a measured ~52ms daemon-alive window.

**Solution**: Retrofit the isolation across the class and widen the guard so it can see them. Set `HISTFILE` to `os.DevNull` inside `IsolateStateForTest` alongside the `HOME` re-point and delete the three local workarounds. Fix the three inverted orderings so the state dir outlives `kill-server`. Give the harness fixture the pid-source variant of the teardown helper it already models at `:74`.

**Decision**: How far does the guard's rule widen?
1. Widen it from "calls `IsolateStateForTest`" to "sets `PORTAL_STATE_DIR` at all" — catches every fixture in the class, including any future one that hand-rolls a state dir.
2. Keep the rule as-is and instead convert every fixture in the class onto `IsolateStateForTest`, so the existing rule reaches them by construction.

## Task 4: A second slog capture handler lives beside `logtest.Sink`, and this work unit extended it
severity: duplication
sources: duplication, bank

**Problem**: `recordingLogger` (`cmd/bootstrap_production_test.go:14-131`) is a hand-rolled `slog.Handler` reproducing `logtest.Sink` structure for structure — the same `shared`/`bound`/`owner()` indirection, the same `WithAttrs`/`WithGroup` re-binding, the same per-record attr map, its own level-to-string table. `cmd` already has the shared route for exactly this job (`newCaptureLoggerForComponent`, ~75 call sites; `logtest.NewCaptureLogger` beneath it). This work unit did not merely leave the twin in place — it *grew* it: `recordedLog.attrs`, `recordedLog.intAttr` and `onlyMatching` are line-for-line re-implementations of `logtest.Record.Attrs`, `Record.IntAttr` and `Sink.OnlyRecordWith`, and the new sweep suites now capture logs two different ways in the same file (`recordingLogger` for the injected logger, `logtest.Install` for the package-level one — `cmd/run_hook_stale_cleanup_test.go:31,711-718`, `cmd/run_hook_stale_cleanup_lock_timeout_test.go:61`). `internal/logtest` is the declared single source of truth for the capture handler and the rendered-record contract, so a change to the Sink's semantics now silently leaves the sweep suites asserting against a different one. The same class exists one package over: `internal/tmux`'s `recordingSlogHandler` backs `showHooksWarnRecords` (`hooks_register_warn_test.go:13`) and `recordingMigrationLogger` (used at ~10 sites across `hooks_migration_test.go` and `hooks_register_realtmux_test.go`), each with its own re-authored filters.

**Solution**: Delete `recordedLog`, `recordingLogger`, `intAttr`, `countMatching` and `onlyMatching` from `cmd/bootstrap_production_test.go`; replace every `&recordingLogger{}` with `newCaptureLoggerForComponent(t, …)`, `countMatching(...)` with `len(sink.RecordsWith(comp, msg).AtExactLevel(level))`, `onlyMatching(...)` with `sink.OnlyRecordWith(t, …)`, and `rec.intAttr(t, "panes")` with `rec.IntAttr(t, "panes")`. Migrate `internal/tmux`'s two hand-rolled capture types onto `logtest.Sink` in the same pass, so no second handler survives the sweep.

## Task 5: The generated-id width is a `hooks.json` on-disk contract shaped as a generic generator knob
severity: medium
sources: architecture

**Problem**: `nanoid.width` (`internal/nanoid/nanoid.go:22`) is read by three unrelated id domains — session-name suffixes via `session.GenerateSessionName`, spawn batch/ack ids via `spawn.NewSpawnID` and `burst.go:58`, and pane tokens via `session.NewPaneToken` — **and** by `IsTokenShaped`, which the reaper uses to decide whether a persisted `hooks.json` key may be deleted at all. That makes `width` a persisted-format constant, but its doc comment scopes the coupling narrowly ("read by both the generator and `IsTokenShaped`, so generation and recognition cannot drift apart") and says nothing about the other two consumers or about disk. Change the width for an unrelated reason — longer session-name suffixes to reduce collisions, say — and every six-character token already in `hooks.json` stops being token-shaped: the reaper retains it forever, `checkStaleHooks` counts through the same rule and keeps reporting "no stale hooks", and `portal doctor` stays green while the file accumulates entries no pane answers to. That is the same silence this work unit exists to remove, arriving through a different door. `hookstest.ReapableHookKey` panics on a width change, so the *fixture vocabulary* fails loudly — but nothing in the suite exercises a persisted key authored under the old width, so once the fixture is repaired the data-classification change is invisible.

**Solution**: Make the on-disk contract explicit and hard to break by accident.

**Decision**: How is the coupling narrowed?
1. Document it — state in `internal/nanoid`'s package doc and above `width` that the value is part of `hooks.json`'s key-recognition contract and that changing it reclassifies every persisted key, so a change is a migration event rather than a tuning change. Add a test pinning a persisted old-width key's classification so a width change fails loudly on the data, not just on the fixture.
2. Split it — give the pane token its own named width beside `IsTokenShaped`, read by both the pane-token mint and the predicate, leaving session-name and spawn-id widths free to move independently. Keeps the spec's no-drift property while narrowing the coupling to the one domain with durable storage.

## Task 6: `CleanStale`'s enumeration callback carries its decline reason out-of-band
severity: medium
sources: architecture

**Problem**: `Store.CleanStale(enumerateLive func(Snapshot) ([]string, error))` (`internal/hooks/store.go:302`) documents that a returned error "is returned unwrapped, so a caller can carry its own reasons through" — an invitation to put the reason *in* the error. The only caller does not: `runHookStaleCleanup` returns the bare sentinel `errCycleDeclined` from the closure (`cmd/run_hook_stale_cleanup.go:165,173`) while writing the actual reason, level and attrs into a `decline standDown` variable captured from the enclosing scope, which `declinedSweep` (`:197`) re-joins afterwards. Sentinel and payload are two independent channels kept in step by hand. A second decline path added inside that closure that returns `errCycleDeclined` without assigning `decline` yields the zero `standDown`: `emit()` writes `hooks: clean-stale-skipped reason=` at INFO with an empty reason, and `sweepOutcome{DeclineReason: ""}` is indistinguishable from a cycle that ran and found nothing — so `portal doctor --fix` prints no skip line and the daemon reports nothing. Correctness rests on caller discipline at a package boundary, which is the property §4 and §5 removed everywhere else.

**Solution**: Fold the reason into the error so the two cannot separate — a small typed error in `cmd` wrapping the `standDown` (`type declinedError struct{ standDown }`), returned from the closure and recovered in `declinedSweep` with `errors.As`. `errCycleDeclined` and the captured `decline` both disappear, and `CleanStale`'s documented "carry your own reasons through" contract becomes the mechanism actually used.

**Outcome**: A decline path that forgets to name its reason no longer compiles into a silent empty-reason line.

## Task 7: Bare `-t` targets are composed outside the tmux client, with nothing enforcing the rule
severity: medium
sources: bank

**Problem**: The exactness rule now holds inside `internal/tmux`, but two production sites compose their own targets outside it and bypass it entirely. `internal/session/quickstart.go:51-52` builds a chained tmux argv carrying `set-option -t <name>` and `attach-session -t <name>`, both unprefixed — the name is generated-unique and created in the same chain, but tmux continues a `;` chain past a failed `new-session`, so a failed create lets the stamp land on a prefix sibling. `internal/restore/session.go:85-104` builds `target := fmt.Sprintf("%s:", sess.Name)` and hands it to `SplitWindow`/`NewWindow` — the exact `<session>:` shape whose unpinned form resolves to a prefix sibling. Beyond those two, nothing enumerates `-t` composition sites at all: the class has now been rediscovered **three** times (the original seven client sites, the two `saver_pane_pid.go` sites fixed with the wrong helper, and `SelectLayout` caught in review), which is the signature of an invariant resting on author discipline.

**Solution**: Pin both call sites through the client's exactness vocabulary, and add a source guard that enumerates `-t` argv composition across `internal/tmux`, `internal/session` and `internal/restore` and requires each to route through `exactTarget` / `exactCoordTarget` / `windowTargetExact` / `PaneTargetExact`. The guard is what turns a rediscovered class into a caught one; `internal/sourceguardtest` already owns the primitives it needs.

## Task 8: The hook-fire assertion in `cmd/bootstrap` is the unpolled read already fixed one package over
severity: medium
sources: bank

**Problem**: `verifyHookFiredOnce` (`cmd/bootstrap/reboot_roundtrip_test.go:437-448`) does a bare `os.ReadFile` + `strings.Count("HOOK_FIRED")` with a `t.Fatalf` on ENOENT, called at `:164` right after the same `WaitForSkeletonMarkersCleared` at `:158` — the exact shape and exact race that was removed from `internal/restore` (only the intervening `verifyANSIScrollback` narrows the window). The markers clear when the helper reaches its exec step, *before* the hook's `echo >>` completes. `internal/restore` now has the polled `assertMarkerCount` (`marker_assert_test.go:30`) bounded by `hydrateBudget`/`hydrateTick`, and both packages already import `internal/restoretest`. Four pieces of residue ride the same edit: `internal/restore/multipane_legacy_integration_test.go:139,209` still pass raw `10*time.Second, 50*time.Millisecond` where the rename fixture passes the named constants; `hydrateBudget`/`hydrateTick` serve both families but still live in `rename_reboot_shared_test.go` rather than beside the assertion; `internal/restore/marker_assert_meta_test.go:57` panics inside a writer goroutine, which would abort the whole integration binary rather than fail one test; and `cmd/noncontiguous_window_reboot_integration_test.go` carries its own `divergentHydrateBudget`/`divergentPollTick` pair.

**Solution**: Promote the polled marker assertion and its budget/tick pair into `internal/restoretest`, route `verifyHookFiredOnce` and the two literal call sites and the divergent pair through it, and replace the writer-goroutine panic with a plain error drop that fails via the helper's deadline message.

## Task 9: Integration-lane timing budgets fail on a normally-loaded developer machine
severity: medium
sources: bank

**Problem**: Three independent agents observed the same class. **(1)** Six `cmd/bootstrap` constants declare a 6s pgrep-convergence budget (`composition_abc_integration_test.go:19`, `composition_e2e_convergence_integration_test.go:16`, `composition_e2e_f_observables_integration_test.go:19`, `composition_e2e_fresh_acquire_integration_test.go:16`, `composition_e2e_self_eject_integration_test.go:25`, `upgrade_path_integration_test.go:19`); under ~18 load on 10 cores the observed elapsed clusters at 6.05–6.12s — just over budget — and which member trips rotates between runs. Confirmed on a stashed clean tree, so unrelated to any recent change. **(2)** `cmd/state_daemon_integration_test.go:42` seeds `scrollbackLines = 500000` against a fixed 10s budget and fails 3/3 standalone on a loaded box, reproducing identically with the file stashed back to HEAD. Because there is no CI, every run is on a machine carrying the developer's real workload, so a budget with ~2% headroom manufactures false failures and erodes trust in the lane — which then misattributes real regressions.

**Solution**: Replace the fixed budgets with something that survives contention, across all seven sites in one pass so the constants cannot diverge again.

**Decision**: What replaces the fixed budget?
1. Widen the budgets and single-source them — one shared constant per class with generous headroom, so a loaded machine passes and a genuine hang still fails.
2. Make the wait contention-tolerant — poll to a deadline that extends while progress is observable (pgrep count falling, scrollback lines accumulating), so the assertion measures convergence rather than wall-clock throughput.

## Task 10: Integration-tagged source carries untouched lint debt, and the issue cap hides findings
severity: drift
sources: bank

**Problem**: `golangci-lint run ./...` analyses only the default build tags, so every file behind `//go:build integration` has never been in scope. Verified now: the unit lane reports **0 issues**, while `golangci-lint run --build-tags integration --max-same-issues 0 --max-issues-per-linter 0 ./...` reports **21** — 14 `rangeint`, 4 `stringsseq`, 1 `stringscutprefix`, 1 `stringscut`, 1 `minmax` — concentrated in `internal/restore/integration_full_test.go` (10× `rangeint`) with the rest across `cmd/bootstrap` and `cmd`. Same mechanical, autofixable class as the 77 the repo-wide sweep already cleared. Separately, the default `max-same-issues` cap is what made that sweep's finding count read as 30 instead of 77, and it will keep hiding real findings from every future run.

**Solution**: Clear the 21 integration-tagged findings and settle how the linter is invoked so the debt cannot re-accumulate invisibly.

**Decision**: How does the integration lane stay linted?
1. Pin `max-same-issues: 0` in `.golangci.yml` and document a second lint invocation with `--build-tags integration` alongside the existing one in CLAUDE.md's Build & Test block.
2. Configure `.golangci.yml` to analyse the integration tag by default, so one command covers both lanes and there is nothing to remember.

## Task 11: CLAUDE.md drift left by this work unit
severity: drift
sources: bank

**Problem**: Four claims in CLAUDE.md no longer match the tree, all of them in the passages this work unit changed. **(1)** The `logtest` row (`:82`) still enumerates the old accessor set (`AttrString` / `IntAttr` / `RequireDuration` / `HasAttr` / `OnlyRecord`) and says consumption is "via thin embedded-field wrappers across cmd / state / restore / tui / store test surfaces" — the `cmd`, `cmd/bootstrap` and `internal/state` wrappers are gone, and the package now also owns `Install`, `RecordWant`/`AssertRecord` and the `Records` filter chain. **(2)** The Multi-window spawn bullet (`:213`) claims "the nanoid alphabet … [is] single-sourced in `internal/spawn`"; the alphabet has never lived there and now lives in `internal/nanoid`, so it is the one place in the file pointing an agent at the wrong home for the id vocabulary — directly contradicting the new `nanoid` row three sections above. **(3)** The word "snapshot" appears nowhere in the file outside the unrelated `portaltest` row, so `CleanStale`'s snapshot-before-enumeration narrowing — the machinery that keeps a `hook set` landing in the enumeration gap from being reaped — is entirely undocumented; the hooks row's new sentence is accurate but silent on it. **(4)** Nothing records the `hooks.json.lock` sidecar being absent on every install in the wild until the next release takes a mutation lock, nor that `migrateConfigFile` moves `hooks.json` without its sidecar — so the degraded read is the common path after the next release rather than an edge case.

**Solution**: Correct the `logtest` row against the package's current surface, delete the alphabet claim from the spawn bullet, and add the snapshot-narrowing invariant plus the sidecar-absence note to the hooks row and the Resume hooks section.

## Task 12: The sweep's stand-down reporting reports one condition twice and takes a logger that governs half its output
severity: low
sources: standards, architecture

**Problem**: Two defects in the same reporting path. **(1)** `declinedSweep` (`cmd/run_hook_stale_cleanup.go:203-219`) deliberately returns nil for the lock-timeout branch, with the stated reason that "the nil error keeps the caller from adding a second report for the same event" — then does the opposite for `ErrSnapshotRead`: it emits the `clean-stale-skipped reason=store-read-failed` WARN **and** returns the error, so the daemon adds `hooks stale-cleanup failed` and `doctor --fix` adds `doctor --fix: stale-hook prune failed` for the same event. Two WARN lines for one condition, inside a work unit whose stated purpose is that a single grep answers why the prune declined; the project's log-or-return convention is waived by the spec only for `hook set` / `hook rm`, where the two lines serve different audiences. **(2)** `runHookStaleCleanup(reader, store, logger *slog.Logger)` (`:153`) takes a logger, defaults it to `bootstrapLogger` when nil, and uses it for exactly two DEBUG count lines (`:169`, `:189`). Every stand-down line — the ones answering the question the sweep's observability exists for — is written by `standDown.emit()` to the package-global `hooksLogger`, bypassing the parameter. A caller injecting a logger to observe the sweep captures the counts and none of the stand-downs, which is why the suite additionally installs a global handler.

**Solution**: Apply the lock-timeout branch's own reasoning to the store-read branch so the event is reported once, and either rename the parameter for what it governs with a doc comment stating that stand-downs always carry the `hooks` component, or drop it and bind the counts to their own component directly.

## Task 13: The doctor renderers are unbound to the reason constants and neither is exhaustive
severity: low
sources: architecture, duplication

**Problem**: `skippedPrunePhrases` (`cmd/doctor.go:215`, five entries) and `notEvaluableDetails` (`:308`, four) both key off the same `skipReason*` const block declared in another file, render it in two registers, and both fall through to the raw slug when a reason is unmapped. Nothing binds either map to the const block, so adding a sixth reason compiles and ships and the user sees the internal slug (`store-read-failed`) in `portal doctor` output — on the command whose whole purpose is telling the user what happened. The key sets have **already** diverged (`skipReasonLockTimeout` is in one and absent from the other) with nothing to notice it. Separately, the two lookup functions `skippedPrunePhrase` and `notEvaluableDetail` have byte-identical bodies and repeat the same "an unmapped reason falls through to its raw value" promise twice; the wordings genuinely differ per surface so the maps must stay two, but the lookup and its fallback rule need not.

**Solution**: Home the phrases with the reasons (move both maps beside the const block, or give the reason a small type), collapse the two lookups into one `phraseFor(m map[string]string, reason string) string`, and add a table-driven guard asserting every declared reason has an entry in each renderer. The fall-through stays as the runtime safety net; the omission moves from a cosmetic production defect to a test failure.

## Task 14: `logtest.Record` has no error accessor, so nine sites hand-roll it
severity: duplication
sources: duplication

**Problem**: Nine sites across five files write the same eight-line block to get an error out of a record — index `rec.Attrs["error"]`, fatal on absent, type-assert `.Any().(error)`, fatal on the wrong kind — each with its own failure wording, and six of the nine then follow with the identical `errors.Is(loggedErr, fileutil.ErrWriteTempCreate)` check: `internal/hooks/lock_write_test.go:39-51`, `internal/hooks/store_test.go:1182,1358,1514`, `internal/project/store_logging_test.go:124,245,350,544`, `internal/storelog/clean_stale_test.go:75-85`. All five files were rewritten by the task whose stated purpose was that `logtest` owns the sink, the filters and the record assertion. The same files also hand-roll `if _, ok := rec.Attrs["took"]; !ok` at six sites (`internal/hooks/store_test.go:1024,1179`; `internal/project/store_logging_test.go:450,541`; `internal/storelog/clean_stale_test.go:32,68`) although `Record.RequireDuration` already exists and is used for exactly this in `cmd/bootstrap` and `internal/state`. `Record` carries `AttrString`, `IntAttr`, `RequireDuration` and `HasAttr` — the error accessor is the one member of the family never added, so every caller re-derives it.

**Solution**: Add `func (r Record) ErrorAttr(t TestingT, key string) error` to `internal/logtest/capture.go` beside `IntAttr`, fatal on absent and on a non-error value, and route all nine sites through it. Replace the six hand-rolled `took` presence checks with the existing `RequireDuration`.

## Task 15: `logtest`'s filter surface advertises an unused idiom and lacks the one three packages re-author
severity: duplication
sources: architecture, duplication, bank

**Problem**: Three related gaps in one exported surface. **(1)** The `Records` named slice type exports three chainable filters (`AtExactLevel`, `AtOrAboveLevel`, `With` — `capture.go:82-96`) alongside three `Sink` forwarders that call them; across the whole repository the chained form is used **zero** times outside the package, so half the surface is a second way to do what the other half does, kept alive only by its own forwarders. **(2)** The filter three packages actually need does not exist: a *message-only* record filter is re-authored as `themeEventRecords` (`internal/tui/theme_panel_commit_load_test.go:110`, consumed from four files), `recordsNamed` (`internal/theme/events_test.go`) and `captureSink.recordsWithMessage` (`internal/restore/logging_capture_test.go:20`, which also carries a `capturedRecord` projection and a `newCaptureLogger` wrapper existing only to host it) — `Records.With` requires a component, so none of them can route through the package. **(3)** `markerReporter` (`internal/restore/marker_assert_test.go:14-18`) is a fresh declaration of the narrowed-`*testing.T` seam `logtest.TestingT` already declares with the identical method set, and `recordingReporter` + its `fatalSentinel` panic-and-recover trick (`marker_assert_meta_test.go:16-45`) is a second copy of `restoretest`'s `fakeFataller` pattern.

**Solution**: Unexport the three chainable filters (keeping `Records` as the return type, so every existing caller is untouched), add `Sink.RecordsWithMessage(msg)`, and route the three re-authored message filters through it — which lets `internal/restore` drop its wrapper type entirely. Point `markerReporter` at `logtest.TestingT` and reuse `restoretest`'s recording-fataller rather than redeclaring it.

## Task 16: `hooks.json` test staging is reimplemented seven ways across two packages
severity: duplication
sources: duplication, bank

**Problem**: `internal/hookstest` was created by this work unit as the shared home for `hooks.json` scaffolding, and it holds the sidecar half plus the env-based seeders. The store-at-a-path half was left to each suite, and seven independent versions exist: `storeWithContent` (`internal/hooks/lookup_test.go:14`) and `seedHooksFile` (`store_shape_test.go:17`) have **identical bodies** in the same test package, differing only in return arity; `bogusHooksStore` (`cmd/run_hook_stale_cleanup_outcome_test.go:15`) and `seedHooksDirectory` (`internal/hooks`) both stage a *directory* at the hooks.json path with near-identical explanatory comments; and `seedReadFixture`, `seedThenDenyWrites` and `newStagedHooksStore` (`cmd/testhelpers_test.go:50`) are the same base staging plus one axis each, which `newStagedHooksStore`'s `hooksStoreStaging` struct already models as a parameterised shape. The cost is drift, not verbosity: the sidecar-creation rule (create it before any chmod denial, or the mutation fails at the wrong place) is encoded in two of the seven and absent from the rest. Two adjacent sites ride the same fix: `internal/hooks/read_lock_test.go:353-355` stages a sidecar inline beside its own `seedReadFixture` purely to get one specific key/command pair, and `cmd` keeps a **second** staging route — `hooksFileInTempDir` (`cmd/testhelpers_test.go:114`) + `writeHooksJSON` (`:163`) — that points `PORTAL_HOOKS_FILE` and stages *no* sidecar, so every read through it degrades unasked (visible at `cmd/hooks_read_lock_test.go:80-89`, where the `want` baseline is itself taken from a degraded read).

**Solution**: Move `hooksStoreStaging` + `newStagedHooksStore` into `internal/hookstest` as the one path-based staging entry point, with its existing `dir`/`seed`/`sidecarAbsent`/`writesDenied` axes plus an `unreadable` axis for the directory-at-the-path case and an entries-shaped seed parameter for the read-lock site. Retire `storeWithContent`, `seedHooksFile`, `seedReadFixture`, `seedThenDenyWrites`, `seedHooksDirectory` and `bogusHooksStore` onto it, keeping a thin `cmd` wrapper only where call sites read better for it.

**Decision**: What is the sidecar default, and does the env-pointing route fold in? A phase-6 reviewer falsified the assumption that a sidecar-less `hooks.json` is unreachable: the sidecar shipped in commits later than v0.11.0, so every install in the wild is sidecar-less until something takes the mutation lock, and `migrateConfigFile` moves `hooks.json` without its lock file. The degraded read is the common path after the next release, not an edge case.
1. Default to **sidecar present** (today's `newStagedHooksStore` behaviour) and fold `hooksFileInTempDir`/`writeHooksJSON` onto it — one route, fixture hygiene preserved (no incidental `load-unlocked` breadcrumb across ~30 doctor fixtures), with the degraded path pinned by the three tests that already cover it deliberately. Reaches ~60 call sites in `cmd/hooks_test.go`.
2. Default to **sidecar present** but leave the env-pointing route as a separate, documented axis — the two model genuinely different things (an injected store vs. a path the real command body resolves), so they stay two entry points in one package rather than one.
3. Default to **sidecar absent**, matching the post-release production reality, and have the fixtures that need the locked path ask for it — most honest to production, noisiest for the existing suite.

## Task 17: The `hooks.json` byte-identity assertion has three homes across two packages
severity: duplication
sources: bank

**Problem**: `cmd` owns `assertHooksFileUnchanged` (`cmd/testhelpers_test.go:199`). `internal/hooks/store_test.go` open-codes the same `readFileBytes` + `bytes.Equal(before, after)` comparison four times (`:414`, `:449`, `:503`, `:525`) with near-identical failure messages, because package `hooks_test` cannot reach `cmd`'s helper. `cmd/cleanstale_transient_listpanes_shared_test.go:76` writes a third form over `hookstest.HooksJSONBytes`. A related asymmetry rides the same edit: `cmd` has `readFileBytes` for `hooks.json` but no `projects.json` counterpart, so three symmetric hooks/projects read pairs (`cmd/doctor_test.go:860`, `cmd/run_hook_stale_cleanup_test.go:305,448`) stay raw — converting only the hooks half would split each pair.

**Solution**: Promote the byte-identity assertion into `internal/hookstest` beside `AssertSidecarFree` — a package both `cmd` and `internal/hooks` already import — and have `cmd/testhelpers_test.go:199` delegate rather than duplicate. Add the `projects.json` read counterpart so the three symmetric pairs can convert together.

## Task 18: Three doctor drivers, two of them byte-identical apart from one argv element
severity: duplication
sources: duplication

**Problem**: `runDoctorFixCmd` (`cmd/doctor_test.go:1317-1332`) and `runDoctorCmd` (`:1334-1349`) are the same sixteen lines — same `isolateTerminalsFile` call with the same comment, same `doctorDeps` install and cleanup, same two buffers, same `resetRootCmd`/`SetOut`/`SetErr`/`Execute` — differing only in whether `"--fix"` is appended to `SetArgs`. `runDoctor` (`:111-127`) is a third copy that additionally builds the deps from a state dir. Any change to how a doctor run is isolated or driven (a new eagerly-read config file, a third stream) has to be made in three places and will be made in one.

**Solution**: Collapse to one `runDoctorWith(t, deps *DoctorDeps, args ...string) (*bytes.Buffer, *bytes.Buffer, error)` that installs the deps, isolates `terminals.json` and executes `append([]string{"doctor"}, args...)`. Keep `runDoctor(t, dir)` as a two-line wrapper building `withHealthyRuntime(&DoctorDeps{StateDir: dir})` and delegating.

## Task 19: The hook rm exit suite restates four subtests and carries two latent traps
severity: duplication
sources: duplication, bank

**Problem**: Four issues in one file. **(1)** The `it leaves hooks.json byte-identical on every failing route` table (`cmd/hooks_rm_exit_test.go:114-162`) drives five rows, four of which are already staged and asserted individually earlier in the same function (`:13`, `:52`, `:75`, `:94`), each ending in the same `assertHooksFileUnchanged` call the table exists to make — so the table adds one new case while re-staging four. **(2)** The two tables at `:162` and `:230` carry the same row struct (`name`/`paneID`/`resolver`/`stamper`/`extra`), the same "supply a plain stamper when the row did not set one" comment, and the same eight-line loop preamble. **(3)** `:306` is named "it touches no dirty flag on either path" but both its rows drive `runHookRm` with `TMUX_PANE=%3` and no `--pane-key`, i.e. the resolved-token path twice — they vary the *outcome*, not the path — while the sibling 70 lines above (`:230`, "it mints and stamps nothing on either path") uses the identical phrase to mean resolved-token vs `--pane-key`. **(4)** `:30` asserts `strings.Contains(err.Error(), "no such pane: %999")` where the output is now deterministic after the outer wrap was removed, and the suite's own newer test already pins the exact string for both verbs.

**Solution**: Delete the four already-covered rows from the byte-identity table (or, preferably, drop the four standalone subtests' duplicated staging and let the table own the byte-identity claim while they keep only their message-text assertions); extract the shared row struct and loop preamble into one `runRmCase(t, tt)`; give `:306` a `--pane-key` row or rename it to say "on either outcome"; and tighten `:30` to an exact-string comparison. While in the loop, build the poisoned tmux-call pair *inside* it off a `paneKeyPath` bool rather than hoisting it per parent subtest, so a second `--pane-key` row added later cannot silently share counters.

## Task 20: Two shape-rule subtests are duplicated verbatim across two files in `internal/hooks`
severity: duplication
sources: duplication

**Problem**: `cleanstale_snapshot_test.go`'s "it still deletes an empty key present in both the file and the snapshot" (`:177-191`) is the same test as `store_shape_test.go`'s "it deletes an empty key" (`:31-62`): same seed string, same `enumerating(liveKey)` call, same `slices.Equal(removed, []string{""})` assertion, same reload-and-check — only the failure wording differs. "it still retains a non-token-shaped key" (`:193-211`) is likewise a trimmed copy of "it retains a non-token-shaped key absent from the live set" (`:88-106`). Neither snapshot-file copy varies the snapshot at all — both use the ordinary path where the file and the snapshot agree — so they exercise no behaviour the shape file does not. Two files now assert one rule, and a change to the rule will land in whichever file the author happens to open.

**Solution**: Delete the two "it still …" subtests from `cleanstale_snapshot_test.go`; that file's own subject — the snapshot narrowing — is already covered by "it retains a key written after the snapshot" and "it derives the delete set from the file under the lock, not from the snapshot". `store_shape_test.go` stays the single home for the shape rule.

## Task 21: Two pairs of line-identical subtests in `cmd/hooks_test.go`
severity: duplication
sources: bank

**Problem**: `cmd/hooks_test.go:705` ("--pane-key flag removes specified key without requiring TMUX_PANE") and `:761` ("it removes the verbatim key on rm --pane-key without consulting the resolver") have identical setup — same `eee555`/`other-proj:0.0` seed, same empty `TMUX_PANE`, same argv — and identical assertions, so they are line-for-line the same test under two names; both also overlap `cmd/hooks_rm_exit_test.go:143`, which asserts the same behaviour with a better-named subject. Separately, `:791` ("it errors when TMUX_PANE is unset for the rm fallback") is fixture-for-fixture and assertion-for-assertion identical to `:550` ("returns error when TMUX_PANE is not set") — same discarded-return `hooksFileInTempDir(t)`, same empty `TMUX_PANE`, same `mockKeyResolver{key: "unus00"}`, same argv, same non-nil-error + substring pair, with neither adding anything. It is the rm-side twin of the set-side case a phase-6 task already deleted; the bank entry that produced that task named only the set side.

**Solution**: Delete `:761` and `:791`, keeping `:705` and `:550`, which retain identical coverage. Record the subtest-count reduction in the commit so the loss is deliberate rather than silent.

## Task 22: The reboot bracket and its `Orchestrator` literals are open-coded at five sites
severity: duplication
sources: bank

**Problem**: `restoretest.RebootServer` and `restoretest.RestoreFromState` (`internal/restoretest/reboot.go:22,40`) now own the reboot bracket, and `newRenameRebootFixture` routes through them — but `internal/restore/multipane_legacy_integration_test.go:111-128` and `:184-204` still open-code `KillServer` → list-sessions must-fail guard → `EnsureServer` → `restore.Orchestrator{…}` → `RestoreWithMarker` verbatim, and the same file carries a third and fourth copy of the ~25-line arrange (differing by pane count and hook vocabulary, so it needs a parameterised variant). Beyond that, five files hand-build a bare `restore.Orchestrator{Client, StateDir, Logger, Exe}` literal (`armed_restore_integration_test.go:62,140`; `exit_closes_pane_integration_test.go:164`; `integration_full_test.go:116`; `multipane_legacy_integration_test.go:122,198`) — correct today, but `Exe` is **opt-in at every one of them**, and a forgotten field is silent: with `os.Executable()` as the default, a test that drives a real restore without setting `Exe` arms its panes with the *test binary*, which stops flag parsing at the `state` positional, re-runs its own suite inside the tmux pane and exits 0. The symptom is a vanished session, not an error. Two smaller pieces ride the same edit: `restoretest.StagedHydrateExe`, `restoreAdapterFor` (`cmd/bootstrap/helpers_integration_test.go:19`) and `stagedRestoreAdapter` (`cmd/reattach_integration_test.go:55`) are three helpers doing one job; and eight `restoretest.PrependPATH` sites survive whose stated reason (the hydrate helper) is now pinned through `Exe`, while others (a fixture whose orchestrator registers real `portal state notify` global hooks) still genuinely need it.

**Solution**: Route the remaining open-coded reboot sequences through `RebootServer`/`RestoreFromState` with a parameterised arrange for the multipane variants, and make `Exe` structurally impossible to omit — a `restoretest` constructor taking `binDir` and always setting it, as the only supported route to a real-restore `Orchestrator`, backed by a source guard over bare `restore.Orchestrator{` literals in `*_test.go`. Audit the eight surviving `PrependPATH` sites per-test and drop the dead ones. While in `restoretest`, close the marker bracket's other half: only the *unset* of `@portal-restoring` is asserted anywhere (`integration_full_test.go:285`, `armed_restore_integration_test.go:83`) — deleting the `SetServerOption` from `restore_marker.go:21` leaves both lanes green, so one assertion that the marker is set during `Restore()` would cover every caller at once.

## Task 23: The seed-key vocabulary is re-declared in four packages
severity: duplication
sources: duplication, bank

**Problem**: `internal/hookstest` exports `ReapableHookKey(n)` but not the named seeds built on it, so each consumer restates the naming convention and each is free to re-point a name at a different index. `cmd/hookkey_vocabulary_test.go:22-35` declares `reapableSeedA`…`reapableSeedD` plus `liveSeedA`…`liveSeedC`; `internal/hooks/store_test.go:598-605`, `cleanstale_snapshot_test.go:27-29` and `store_shape_test.go:27-28` re-derive the same indices under their own names — and two of them bind a local named `liveKey` to `ReapableHookKey(0)`, the exact name-asserting-live-on-a-reapable-seed pattern a phase-6 task removed from `cmd`. `cmd/state_daemon_hook_cleanup_integration_test.go:47,51` (package `cmd_test`, so it cannot see the `cmd` vocabulary) and `cmd/bootstrap/transient_listpanes_helpers_integration_test.go:108-109` re-derive indices 0 and 1 inline. Two daemon fixtures (`cmd/state_daemon_hook_cleanup_test.go:68`, `cmd/state_daemon_run_test.go:555`) hand-build a two-entry `hooks.json` body that `staleHookSeed` already names, differing only in the stale entry's command text.

**Solution**: Export the named live/reapable seed vars (and the shared two-entry seed body) from `internal/hookstest`, and point all four packages at them. Reconciling the stale entry's command string across the two daemon suites is part of the work.

## Task 24: The repo's source guards hand-roll the primitives `sourceguardtest` exists to own
severity: duplication
sources: duplication, bank

**Problem**: Two families. **(1)** `TestCleanStaleDoesNotCallStaleKeys` (`internal/hooks/cleanstale_staleness_guard_test.go:16-42`) and `TestMutationsDoNotCallExportedLoadOrSave` (`:61-100`) share a twenty-line skeleton written twice — `sourceguardtest.PackageGoFiles(".", false)` with the same fatal wording, the per-path `parser.ParseFile(… SkipObjectResolution)` with the same fatal wording, the `scanned++` counter, the `ForEachFuncCall` visit, and the closing empty-check — with only the predicate differing; and `calleeName` (`:44-52`) is a copy of the `callName` helper `sourceguardtest`'s own test carries, and is the natural companion to the exported `ForEachFuncCall` it is always used with. **(2)** Four leaf/import guards each hand-roll the same `go list -deps` exec-and-parse with the same fatal: `internal/nanoid/leaf_guard_test.go:29` (wrapped as `packageDeps`), `internal/hooks/leaf_guard_test.go:59`, `internal/prefs/leaf_guard_test.go:20`, `internal/theme/leaf_guard_test.go:25`. `internal/sourceguardtest` is the declared home for guard primitives — stdlib-only and untagged so every guard it drives runs in the unit lane — and has neither a call-scan skeleton nor a dependency enumerator.

**Solution**: Extract a local `scanPackageCalls(t, visit func(path, funcName string, call *ast.CallExpr))` in the hooks guard file owning the enumerate/parse/count/empty-check skeleton, leaving each guard only its predicate; promote `calleeName` to `sourceguardtest.CalleeName`; and add `sourceguardtest.PackageDeps(t, pkg) []string`, routing all four leaf guards through it.

## Task 25: Nine `*Deps` seams in `cmd`, one of them guarded
severity: duplication
sources: bank

**Problem**: `cmd` declares nine package-level test seams (`cmd/doctor.go:72`, `cmd/hooks.go:39`, `cmd/kill.go:9`, `cmd/list.go:11`, `cmd/open_burst_run.go:13`, `cmd/root.go:36`, `cmd/open.go:36`, `cmd/state_commit_now.go:30`, `cmd/uninstall.go:21`). This work unit solved the install-and-restore pairing for `hooksDeps` alone — `withHooksDeps` plus `cmd/hooks_deps_guard_test.go`. The other eight carry the identical unguarded pattern: bare `X = &XDeps{...}` + a separate `t.Cleanup` at 108 sites across 22 files for `bootstrapDeps`, 54 across 7 for `openDeps`, 4 for `doctorDeps`. Same leak vector — a missed or mis-ordered cleanup leaks a mock into the next test in a package whose `TestMain` poison is the only other line of defence — and now with a proven helper-plus-guard pattern to generalise.

**Solution**: Give each remaining seam a `withXDeps(t, deps)` helper that installs and registers its own restore, convert the call sites, and parameterise the existing guard over the identifier list so every seam is covered by one test. A single generic `withDeps` is not reachable without changing the seam representation, so the realistic form is one helper per seam plus one guard over all nine.

## Task 26: Seven bespoke `Commander` fakes across the `cmd` test files
severity: duplication
sources: bank

**Problem**: `fakeCommander` (`cmd/state_daemon_test.go:254`), `membershipFakeCommander` (`:714`), `envFailingCommander` (`cmd/state_daemon_capture_logging_test.go:269`), `daemonFakeCommander` (`cmd/state_daemon_run_test.go:26`), `recordingCommander` (`cmd/uninstall_test.go:26`), `stubCommander` (`cmd/open_test.go:1726`) and `gonePaneCommander` (`cmd/hooks_seams_test.go:93`) all fake the same `tmux.Commander` interface, alongside `transienttest.Commander` which already exists as a shared one. One scripted fake — argv pattern in, canned result out, with optional call recording — would serve most of them, and the interface is small enough that seven independent implementations of it will diverge on the `Run`/`RunRaw` trim-vs-verbatim split the moment that contract changes.

**Solution**: Introduce one scripted `Commander` fake (in `cmd`'s test helpers, or promoted beside `transienttest.Commander`) and retire the seven onto it, keeping a bespoke type only where a test genuinely needs behaviour a script cannot express.

## Task 27: 76 inline `logtest.Sink` installs survive outside `logtest.Install`
severity: duplication
sources: bank

**Problem**: `logtest.Install(t)` now exists and is used at ~190 sites, but the two-line `sink := &logtest.Sink{}` + `log.SetTestHandler(t, sink)` is still written inline at 76 remaining `SetTestHandler` call sites across ~20 files — concentrated in `cmd/open_test.go` (20), `internal/theme/events_test.go` (6), `cmd/run_hook_stale_cleanup_test.go` (6), `main_panic_test.go` (5) and `cmd/hooks_test.go` (5). Three declared fixtures also open-code the same two lines inside a broader helper (`cmd/state_hydrate_exec_failure_test.go:16`, `cmd/state_daemon_self_eject_log_test.go:20`, `internal/theme/resolve_test.go:21`), and two sites wrap the sink in a local struct (`internal/tmux/portal_saver_lifecycle_events_test.go:43`, `internal/restore/logging_capture_test.go:32`) that must be unpicked first.

**Solution**: Convert the remaining sites to `logtest.Install(t)`, unpicking the two wrapper structs as part of the pass. Mechanical, but it spans `cmd`, `tmux`, `theme`, `tui`, `restore`, `prefs` and `capturetool` — which is why it needs to be one deliberate sweep rather than opportunistic drift.

## Task 28: 54 inline `hydrateConfig` literals and a superseded builder
severity: duplication
sources: bank

**Problem**: `hydrateCfg` + `hydrateCfgOpts` (`cmd/state_hydrate_test.go:939`) was created as the one route to a `hydrateConfig`, but 54 inline `hydrateConfig{...}` literals remain across the hydrate suites (42 in `state_hydrate_test.go`, 7 in `state_hydrate_exec_log_test.go`, 5 in `state_hydrate_file_missing_log_test.go`). `fileMissingCfg` (`cmd/state_hydrate_file_missing_log_test.go:21`) is a fourth suite-local builder taking the identical seven positional parameters its two now-retired siblings took, differing only in setting `HandleFileMissing` and omitting `HandleTimeout` — and since `hydrateCfg` wires both handlers, it fully expresses all three of `fileMissingCfg`'s call sites, which all pass `openFIFOWithTimeout` and never reach the timeout path. Two styles also sit side by side within one file: `cmd/state_hydrate_test.go:1343,1394,1427` assign `cfg.HookStore` *after* the builder call although `hydrateCfgOpts` already carries the field.

**Solution**: Route the inline literals through `hydrateCfg`, delete `fileMissingCfg`, and move the three post-hoc `cfg.HookStore` assignments into the opts struct so one style survives.

## Task 29: `hookstest` re-implements `cmd/config.go`'s hooks-path resolution chain
severity: duplication
sources: bank

**Problem**: `hookstest.ResolveHooksFilePathFromEnv` (`internal/hookstest/hooks.go:21-42`) walks an env slice for `PORTAL_HOOKS_FILE` then `XDG_CONFIG_HOME`, duplicating the precedence `configFilePath` owns in `cmd/config.go`. A third env layer, or an ordering change in production, silently leaves the seeder resolving the old path — and every destructive integration suite that uses it then seeds into a file the code under test never reads, so the test passes by asserting on a file nothing touched. The two are not merely similar; the helper exists precisely to answer "where will the binary under test look?", which makes any divergence a false green rather than a mismatch.

**Solution**: Expose an env-slice-taking resolver from `cmd/config.go` (or factor the precedence into a leaf both can import) and have `hookstest` delegate to it, so the seeder and the binary resolve by the same rule by construction.

## Task 30: `session.NewPaneToken` is a vestigial forwarder that keeps pane identity in the session package
severity: dead-code
sources: duplication, architecture

**Problem**: `internal/session/panetoken.go:8-10` is a one-line pass-through over `nanoid.NewGenerator()()`, added when the id vocabulary lived in `internal/session`. The vocabulary has since moved to the `internal/nanoid` leaf and every other minting site calls `nanoid.NewGenerator()` directly (`cmd/open.go:414,567`, `internal/spawn/burst.go:58`). The wrapper adds no width, no charset and no validation, so the package exposes a second name for the same value — and because `HooksDeps.TokenMinter` is typed `session.IDGenerator` (itself an alias of `nanoid.Generator`), `cmd/hooks.go` imports `internal/session` solely for a type alias and a forwarder concerning *pane* identity, in a package the architecture description scopes to the session-creation pipeline. Two conventions for one thing, and the one that routes through `session` puts pane identity where it does not belong.

**Solution**: Type the seam as `nanoid.Generator`, default it to `nanoid.NewGenerator()`, and delete `internal/session/panetoken.go` and the `internal/session` import from `cmd/hooks.go`. Both id-minting call sites in `cmd` then read the same way.

## Task 31: `Client.ListPanes` has no production caller
severity: dead-code
sources: bank

**Problem**: `internal/tmux/tmux.go:573` is reachable only from tests (`tmux_test.go:1295-1401`, plus `exact_session_target_test.go:81` and `exact_session_target_realtmux_test.go:126,234`) and satisfies no seam interface. It is the third exported client method in that family with zero production callers — the other two were deleted, leaving the task's outcome statement ("the tmux client exports no method with zero production callers") only two-thirds true. It is not costless: this work unit corrected its doc, pinned its wire form and gave it two real-tmux subtests, all maintained for an unused export.

**Solution**: Settle it rather than leaving it half-cleaned.

**Decision**: Delete or keep?
1. Delete it, and re-point the two `exactCoordTarget` routing proofs at a method with a production caller — the coverage those subtests provide is about the target vocabulary, not about `ListPanes`, so it survives the move.
2. Keep it and record why in the doc comment — an intentionally-retained read primitive with test-only use — so the next dead-export sweep does not rediscover it.

## Task 32: Claims the rewrite left behind — stale prose and an unreachable error promise
severity: comments
sources: bank

**Problem**: Two claims in the tree are now false. **(1)** Eight sites in `cmd` name the deleted `ListAllPanes` in test names and failure messages, including a subtest literally called "it enumerates live keys via ListAllPaneHookKeys not ListAllPanes" — a contrast against a method that no longer exists (`cmd/run_hook_stale_cleanup_test.go:85,96,102,105,222`; `cmd/state_daemon_hook_cleanup_test.go:167,171`; `cmd/state_daemon_hook_cleanup_integration_test.go:210`). The assertions are correct — they describe the `ListAllPanesWithFormat` seam's error path — only the prose is stale. **(2)** `ActivePaneCurrentPath`'s doc (`internal/tmux/tmux.go:245-247`) states "A session killed mid-read surfaces as `errors.Is(err, ErrNoSuchSession)`, which callers can treat as unresolvable rather than fatal". Measured on tmux 3.7c, `display-message -p -t <unmatched>` exits **0** with an empty expansion and no stderr, so the method returns `("", nil)` and `wrapNoSuchSession` is unreachable on this path — a fact the `exactTarget` doc 170 lines below in the same file now states outright ("display-message instead returns empty with exit 0 — for a live session as much as a gone one"). The caller is the TUI's lazy dir-resolution fallback, which must already treat empty-and-nil as unresolved.

**Solution**: Rename the eight test names and failure messages onto `ListAllPanesWithFormat`, and correct `ActivePaneCurrentPath`'s contract to describe the empty-and-nil return the method actually produces — with the caller's handling of that case verified rather than assumed.

## Task 33: `internal/portalbintest` compiles the portal binary in the unit lane
severity: low
sources: bank

**Problem**: `internal/portalbintest/build_test.go` runs a real `go build` of the CLI on every `go test ./...`. It only `exec.LookPath`s the result rather than running it, so it does not breach the lane rule as written ("every test that spawns a `portal state daemon` or execs a built `portal` binary lives behind `-tags integration`") — but it is the one remaining unit-lane test that *produces* a portal binary, and it costs a full build on every unit run, on the lane whose whole promise is being fast and hermetic.

**Solution**: Settle whether the lane rule covers builds as well as execs, and align the test with the answer.

**Decision**: Which way does the lane rule go?
1. Widen the rule to cover builds — move the test behind `//go:build integration` and state the widened rule in CLAUDE.md, so the unit lane never shells out to the toolchain.
2. Keep the rule as-is and keep the test — it is the only coverage `portalbintest` has, and losing it from the fast lane means a broken build helper is discovered later; record the exemption at the test so it is not rediscovered as a breach.
