# Analysis Report: resume-hooks-silently-lost (Cycle 2)

## Stats

- Total findings: 143 (22 from analysis agents, 121 banked residue)
- Deduplicated findings: 81 (20 agent findings after cross-agent merge, 61 verified bank entries)
- Banked residue: 61 verified in, 45 resolved, 15 discarded
- Proposed tasks: 33

## Summary

The four specified changes land as decided — token-only keys, the two-read existence probe, shape-aware retention with named deletions, and the sidecar-locked read-modify-write — and phase 6 cleared 45 of the 121 banked entries outright. What cycle 2 surfaces is a different shape from cycle 1: two **measured production regressions introduced by this work unit's own exactness change** (a colon-bearing session name now fails `show-environment` and is silently dropped from `sessions.json`; the `_portal-saver` pane probe still uses the helper the work unit's own doc comment names as wrong for `list-panes`, and resolves a prefix sibling's pid), plus a class of restore/reattach integration fixtures that drive real tmux servers and the hydrate helper with a bare `t.TempDir()` and no `IsolateStateForTest` — the ABSOLUTE INVARIANT surface CLAUDE.md exists to protect.

Both tmux findings were verified on this machine against tmux 3.7c on isolated `-L` sockets, not inferred: `list-panes -t =_portal-saver` returns `_portal-saver-old`'s pane pid with exit 0, and `show-environment -t =a:b` exits 1 with `no such session` for a live session literally named `a:b` where the pre-change bare form exited 0.

The rest is consolidation. The production surface is well single-sourced; the test surface grew parallel implementations of things the repo already owns — a second slog capture handler in `cmd` that this work unit *extended* rather than retired, a nine-site error-attr assertion that survived the very task meant to give `logtest` record ownership, seven ways to stage a `hooks.json`, and three doctor drivers.

## Spec Defects

### S1: The sweep's stand-down vocabulary and the `doctor --fix` copy diverge from the fixed set the specification decided

- **Claim**: §5.1 fixes exactly three user-facing lines — `Skipped stale hook prune: restore in progress` / `… hooks.json is locked` / `… could not read live panes` — "covering the restore marker (§5.4), the lock timeout (§6.5) and the empty-live-set guard (§5.4) respectively". §5.4 fixes the matching closed reason set: "The sweep now declines to run for three distinct reasons … `restoring` for the marker, `lock-timeout` for §6.5's bound, `empty-pane-read` for the guard above … Distinguishing the three is the point."
- **Observed**: `cmd/run_hook_stale_cleanup.go:15-21` declares **five** reasons — the three specified plus `store-read-failed` and `pane-read-failed`. `cmd/doctor.go:215-221` renders five phrases and reassigns the spec's pinned string: `pane-read-failed` now renders `could not read live panes`, and the empty-live-set guard the spec wrote that string for renders the invented `live pane list came back empty`. `cmd/doctor.go:308-313` carries a second, four-entry map in a different register, missing `lock-timeout` entirely.
- **Read**: **Split — spec stale on the enumeration, code wrong on the copy.** The two added conditions are real and pre-existing (a failed `hooks.json` snapshot read and a failed `list-panes` read both previously exited through other paths), and distinguishing five beats collapsing to three, so the enumeration is the spec falling behind a better implementation. The string reassignment is the other way round: a user-facing line that was signed off naming one condition now names a different one, silently. Either restore `could not read live panes` to `empty-pane-read` and give the new branch its own words, or record the reassignment as a deliberate amendment.

### S2: The token predicate's package home, and the refactor that moved it, depart from the specification's argued placement

- **Claim**: §3 places the token shape predicate in `internal/session` beside the generator, argues that is the only home permitting derivation from the unexported width, and records that `internal/hooks` may import `internal/session` cycle-free.
- **Observed**: `internal/nanoid/nanoid.go` is a new stdlib-only leaf holding `Alphabet`, the unexported `width`, `NewGenerator` and `IsTokenShaped`; `internal/hooks/leaf_guard_test.go:24-31` now *forbids* the very `internal/session` import the spec sanctioned. `internal/session/panetoken.go` survives as a one-line forwarder. `CLAUDE.md:78` documents the `nanoid` row as permanent architecture.
- **Read**: **Spec stale.** The invariant the spec cared about is preserved and strengthened — recognition reads the generator's own `width` and `Alphabet`, so generation and recognition cannot drift — and the leaf extraction was cycle 1's approved Task 3. The spec's stated home and its cycle analysis are now simply wrong about the tree; record the supersession.

### S3: A second lock-acquisition bound the specification did not decide

- **Claim**: §6.5 fixes one bound — "The bound is **2 seconds**" — and describes the sweep's advisory pre-read as degrading by the same rule as every other read.
- **Observed**: `internal/hooks/lock.go:17-33` declares `snapshotLockTimeout = 20 * time.Millisecond`, a second package-level bound applied only to the clean's pre-read, capping a clean's worst case at one `lockTimeout` rather than two.
- **Read**: **Genuinely open.** The reasoning is the spec's own (an unbounded or doubled wait parking the daemon's 1s tick) and correctness is unaffected, but it changes observable behaviour: under ordinary contention — a concurrent `hook set` holding the exclusive lock for a few milliseconds — the pre-read routinely falls through to the unlocked read and emits `op=load-unlocked` at DEBUG, where the specified single bound almost never would. Confirm and record it beside the 2s figure, or drop it and accept the doubled worst case.

## Discarded Findings

### Discarded as out of remit (`internal/project` / `internal/alias` — outside this work unit's surface, needing a scoping decision rather than a fold-in)

- **`project.Store.Remove` rewrites the file for an absent path and emits an INFO naming a removal that did not happen** (bank, task 4-1) — re-verified at `internal/project/store.go:211-213`; the doc comment is intact and unchanged since cycle 1. Same discard as cycle 1.
- **`internal/project` carries the identical unlocked read-modify-write window this work unit closed for hooks** (bank, task 5-1) — re-verified: no `flock` and no lock file anywhere in the package; `Upsert`/`CleanStale`/`Rename`/`Remove`/`AddTag`/`RemoveTag` are all `Load()` → mutate → `AtomicWrite`. Same discard as cycle 1.
- **The daemon capture loop issues a bare `-t` target exposed to the rename class** (bank, task 3-10) — `cmd/state_daemon.go` still composes `tmux.PaneTarget(...)` and passes it to `CapturePane`. Same discard as cycle 1. Task 7 below closes the composition class *inside* the client and adds the guard that would catch this site if it were brought in scope.
- **`internal/project`'s stale reaper logs its per-project deletions before the save** (bank, task 6-4) — verified live at `internal/project/store.go:174`, the exact ordering just fixed in `internal/hooks/store.go:353` (which now emits after the write). Symmetric with an in-remit fix, but the file is out of remit; it is also at DEBUG, so invisible at the production default.
- **`project.Store.Remove` and `internal/alias`'s store take an untyped `via string` into the closed log vocabulary** (bank, tasks 6-5 x2) — verified live. Two of the three sites are out of remit, and the third (`internal/storelog/clean_stale.go:20,25`, which hardcodes `"via", "internal"` on the *hooks* component's own summary) cannot be typed alone: `storelog` cannot import `hooks`, so the fix needs the `Via` type moved to a new shared leaf. The whole cluster stands or falls together and is therefore out of remit as a unit.
- **The project-staleness pair carries the shape this work unit removed from the hook pair** (bank, task 6-1) — `maybeRunProjectCleanup` and `pruneDoctorStaleProjects` still call `ProjectStore.CleanStale()` from two sites with no shared outcome value and cannot report a declined cycle at all. Spans the daemon, doctor and `internal/project`.
- **`readOnlyDirPath` exists as an independent third copy in `internal/project`** (bank, task 6-12) — verified at `internal/project/store_logging_test.go:17` alongside `readOnlyDirAliasPath` at `internal/alias/store_logging_test.go:15`; the `internal/hooks` copy is gone. Consolidating needs a home for a cross-package store fixture that neither `storelog` nor `hookstest` is.

### Discarded as record-only (a warning or a correction to the record, not a unit of work)

- **`readHookKey` in `internal/tmux` must NOT be consolidated** (bank, task 3-9) — re-verified present at `internal/tmux/hookkey_format_realtmux_test.go`; the `HookKeyFormat` read *is* its subject under test. Carried forward a second time so the warning survives the bank's retirement.
- **A sidecar-less `hooks.json` is a reachable production state, not an impossible one** (bank, task 6-12) — a correction to the orchestrator's cycle-1 reasoning, not work. It is preserved as context on Task 16's Decision, where the staging default it bears on is actually settled.
- **The teardown-guard coverage guard checks presence per file, not per test function** (bank, task 6-17) — verified, but `internal/portaltest/teardown_guard_coverage_test.go:30-36` now documents the per-file scoping as a deliberate, argued trade ("the value is mostly in catching a newly added file, which arranges both calls together"). The gap is owned, not overlooked. The companion half — the 3-line comment repeated at 20 registration sites in three treatments — is cosmetic and clusters with nothing.
- **The cmd test-file naming residue** (bank, tasks 6-23 x3, 6-23 reviewer) — `cmd/hooks_read_lock_test.go` and `cmd/noncontiguous_window_reboot_integration_test.go` were both judged by the phase-6 reviewer to need no action (`hooks_` derives from a real source file; prefixing the cross-cutting round-trip test would misdescribe it), and `cmd/doctor_fix_transient_listpanes_shared_integration_test.go` having one consumer is a two-file merge with no drift risk. Cosmetic, and the record already carries a no-action verdict.

### Resolved by phase 6 (verified against the current tree, no longer live)

45 of the 121 entries were closed by the 25 tasks cycle 1 proposed. Spot-verified rather than assumed:

- `gofmt -l .` is clean and `golangci-lint run ./...` reports **0 issues** — bank entries 6-5, 6-13, 6-23 and 1-4's unit-lane half are closed (the integration-tagged half survives as Task 10 below).
- `runHookStaleCleanup` now takes three parameters, not five (1-4).
- The three `AllPaneLister` fakes and the two temp hooks-store seeders are gone from `cmd`; `stubStaleSweepReader` and `newStagedHooksStore` replaced them (finder x2, 6-1).
- `state.RestoreWindowActive` is the named home for the restore-window posture, reached by all three `cmd` readers (1-6 x2).
- `hooks.Snapshot` is exported and named at the `cmd` boundary; no raw `map[string]map[string]string` survives in `cmd` production code (6-1 x2).
- `internal/hooks/store.go:353` emits its per-key deletion lines **after** the save (5-5).
- `assertMarkerCount` polls to `hydrateBudget`/`hydrateTick`; `restoretest.SeedScrollback` is the one scrollback seeder across three packages (3-2, 3-8, 5-1, 6-6, finder).
- The real-restore round-trip tests moved behind `//go:build integration` and pin the binary through `restoreAdapterFor` → `StagedHydrateExe` (3-3, 3-5).
- Every session-level `-t` in `internal/tmux` now routes through `exactTarget` / `exactCoordTarget` / `windowTargetExact`, and the `exactTarget` doc names the session/coordinate split explicitly (3-5, 6-7 x2). **This is what makes Tasks 1 and 2 below findings rather than pre-existing debt.**
- `logtest` gained `Install`, `RecordWant`/`AssertRecord`, `OnlyRecordWith`/`RecordsWith` and `RecordsAtExactLevel`; the eight named install helpers and the four `(component,msg)` filters are gone (2-10, 3-6, 5-6, 6-19).
- `CLAUDE.md` gained the `internal/nanoid` row, the shape-aware retention paragraph, and the rewritten `transienttest`/`hookstest` rows (3-4, 5-6, 6-3).
- The deny-writes fixture now asserts `errors.Is(err, fileutil.ErrWriteTempCreate)`, so it can detect its own trap (5-8).
- `newRenameRebootFixture` registers `RegisterStateDirTeardownGuard` (2-7, 6-15).
- The four nil-check dependency builders and the redundant error wrap are gone from `cmd/hooks.go` (2-3, 4-3).
