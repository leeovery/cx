---
phase: 5
phase_name: Serialise hooks.json access
total: 5
---

## resume-hooks-silently-lost-5-1

### Task 5-1: Every mutation takes the file under one exclusive hold

**Problem**: `internal/hooks` has no locking of any kind (`grep -rn 'sync\.\|Mutex\|flock\|Lock()' internal/hooks internal/fileutil | grep -v _test.go` → no matches). `Store.Set`, `Store.Remove` and `Store.CleanStale` are each `Load()` → mutate in memory → `fileutil.AtomicWrite`. `AtomicWrite` makes each *write* atomic; nothing guards the read-modify-write window, and the writers are in **different processes** — `portal hook set` fired by a Claude Code SessionStart against the daemon's sweep every 10s, and CLI against CLI (a SessionStart in one pane concurrent with a SessionEnd in another):

```
daemon  CleanStale  t0    Load() → 41 entries
CLI     hook set    t0+e  Load() → 41, add K, AtomicWrite → 42
daemon             t0+d  writes its t0 snapshot minus stale → 40      K is gone
```

The end state is indistinguishable from the drift symptom this work unit exists to remove: an INFO `hooks: set … hook_key=K` breadcrumb, K absent minutes later, and an intervening `clean-stale` naming a *different* key. No observed loss is proven to be a race, and none is ruled out; the daemon sweeps ~8,640 times a day against a 40+ pane working set.

**Solution**: Take an advisory `flock` on a dedicated sidecar file derived from the resolved `hooks.json` path, held exclusively across each mutation's **whole** load-mutate-save, reached through unexported non-locking load and save helpers so a lock is never nested, with acquisition bounded by a package-level value.

**Outcome**: Concurrent writers over the same `hooks.json` lose no entry, including across the `os.Rename` that replaces the target's inode on every write; each mutation acquires exactly once and releases on every return path including its early ones; and no second, unlocked write path survives on the exported surface.

**Do**:
- Add `internal/hooks/lock.go` holding: the exported sentinel `ErrLockHeld` (on the precedent of `state.ErrDaemonLockHeld`, so a caller can `errors.Is` a timeout apart from a save failure); package-level `lockTimeout = 2 * time.Second`, package-level `snapshotLockTimeout` (near-zero — a few tens of milliseconds) and a short poll interval, all `var` so the unit lane can lower them; `func (s *Store) lockPath() string` returning `s.path + ".lock"`; and unexported `acquireLock(path string, openFlags int, flockMode int, bound time.Duration) (*os.File, error)`, which opens `path` with the caller's flags and polls `unix.Flock(fd, flockMode|unix.LOCK_NB)` until it succeeds or `bound` elapses, returning a wrapped `ErrLockHeld` on expiry and closing the fd on every failure path. `flockMode`, `openFlags` and `bound` are all parameters because task 5-2's shared read path passes `unix.LOCK_SH`, opens without `O_CREAT`, and — for the sweep's advisory pre-read alone — passes `snapshotLockTimeout` rather than `lockTimeout`, so one sweep cycle can never spend two full bounds waiting.
- Do **not** reproduce `AcquireDaemonLock`'s inode cross-check, its 3-attempt retry ladder or its `FD_CLOEXEC` set: those exist because that lock file can be unlinked and recreated and because its caller retains the fd for the process lifetime. Nothing unlinks this sidecar and every caller here closes the fd when its operation returns.
- Add a test-only bound seam beside the vars — `func SetLockTimeoutForTest(t *testing.T, d time.Duration)`, taking `*testing.T` first and restoring the prior value through `t.Cleanup` — on the shape of `log.SetTestHandler`, so `cmd`-level tests in later tasks can drive a timeout without waiting out the production figure. Give `snapshotLockTimeout` its own seam of the same shape, so a test can raise it to make the sweep's pre-read contend deliberately.
- Split the file access in `internal/hooks/store.go`: move today's `Load` body into an unexported non-locking `load()` and today's `Save` body into an unexported non-locking `save(h hooksFile) error`, and have `Set`, `Remove` and `CleanStale` call **only** those two — never the exported `Load`/`Save`. Each of the three now begins with `os.MkdirAll(filepath.Dir(s.path), 0o755)`, then `acquireLock(s.lockPath(), os.O_RDWR|os.O_CREATE, unix.LOCK_EX, lockTimeout)` with `defer f.Close()`, and does its whole load-mutate-save inside that hold — including `Set`'s `set-noop` arm, `Remove`'s no-removal arm (task 4-1) and every failed-save return. Carry one line of comment on the `MkdirAll`-before-acquire ordering, wording left to the executor.
- **Discard `MkdirAll`'s error rather than returning it**, and let the acquire that follows fail on the same condition. A directory that could not be created is a sidecar that cannot be opened, so the two collapse to one branch — and `acquireLock` is the single point task 5-3 attaches its WARN to. Returning `MkdirAll`'s error directly would give an unwritable config directory a return path with no log record at all, quieter than today, where the same condition surfaces as the write WARN carrying `error_class=write-failed-temp-create`. There is no case where the discard hides a distinct outcome: `MkdirAll` returns nil for a directory that already exists, so any failure it reports leaves the sidecar's `O_CREAT` with nowhere to land.
- Delete the exported `Save` and `SaveAudited`: Phase 3 removed their last production caller with `portal state migrate-rename`, and an exported unlocked whole-file write is exactly the second write path this task exists to close. Re-point `internal/hooks/store_test.go`'s seeding (`:112`, `:135`, `:164`, `:1286`) at raw `os.WriteFile` JSON or at `Set`, re-point `TestSave`'s three subtests to drive `Set` (the `AtomicWrite` properties they assert are unchanged), and retire `TestSaveAuditedLogging` with the method.
- Amend the write-failure fixtures so they still fail where they mean to: `readOnlyDirPath` (`internal/hooks/store_test.go:28`), task 4-1's seeded variant of it, and the hand-rolled read-only-parent fixture inside `TestCleanStaleLogging`'s save-failure subtest (`:911-922`) must each create `<hooks.json path>.lock` (mode `0600`) **while the directory is still writable**, before the `chmod 0500`. Without it the mutation now fails at the sidecar's `O_CREAT` instead of at `os.CreateTemp`, and `error_class=write-failed-temp-create` stops being exercised anywhere.
- Add a clause to CLAUDE.md's `hooks` row naming the sidecar lock file beside `hooks.json`, so a new on-disk file is not later read as cruft.

**Acceptance Criteria**:
- [ ] `Set`, `Remove` and `CleanStale` each hold one exclusive `flock` on `<hooks.json path>.lock` across their whole load-mutate-save, acquired once and released on every return path
- [ ] The lock is never taken on `hooks.json` itself, and the sidecar's inode is unchanged across a mutation that replaces `hooks.json`'s inode
- [ ] The sidecar path is derived from the resolved store path, so a `PORTAL_HOOKS_FILE` override carries it wherever it points
- [ ] The lock file is opened `O_CREAT` by writers and is never unlinked by any code path
- [ ] `acquireLock` takes its bound as a parameter, and two package-level bounds exist: `lockTimeout` for mutations and ordinary reads, and a near-zero `snapshotLockTimeout` for the sweep's advisory pre-read
- [ ] The config directory is created before acquisition, so the first `hook set` on a fresh machine succeeds rather than timing out
- [ ] A mutation whose config directory cannot be created fails through `acquireLock` rather than through a bare `MkdirAll` return, so directory failure and sidecar failure are one branch
- [ ] No exported method is reached from inside another: `Set`, `Remove` and `CleanStale` use only the unexported `load`/`save`, and an uncontended mutation completes far inside a lowered bound rather than blocking against its own hold
- [ ] Interleaved writers across the `Load`→`AtomicWrite` window lose no entry: N concurrent `Set`s on distinct keys over separate `*Store` values leave all N entries in the file
- [ ] A mutation blocked by a held sidecar performs its **load** after the release, not before: it observes an entry written by the holder while it was blocked
- [ ] `Remove`'s `(bool, error)` answer still comes from the map loaded inside this hold, and its silence on a no-removal is unchanged
- [ ] Neither `Save` nor `SaveAudited` exists on the exported surface, and no production code writes `hooks.json` outside a held lock
- [ ] `error_class=write-failed-temp-create` is still exercised by the amended write-failure fixtures
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass; no test in this task uses `t.Parallel()`

**Tests**:
- `"it creates the sidecar beside hooks.json on the first mutation"` — assert `<path>.lock` exists and `hooks.json` was written
- `"it locks the sidecar rather than hooks.json"` — record the sidecar's inode, run several mutations, assert `hooks.json`'s inode changed each time while the sidecar's did not
- `"it loses no entry under concurrent writers"` — 20 goroutines, each with its own `*Store` over one path, each `Set`ting a distinct key; assert all 20 keys present
- `"it loads only after a held lock is released"` — hold the sidecar exclusively from a second fd, launch `Set(K2)` in a goroutine, write K1 into `hooks.json` directly, release; assert the finished file holds **both** K1 and K2 (a lock spanning only the write leaves K1 gone)
- `"it releases the lock on the set-noop arm"` — re-register the same command, then assert a following mutation acquires immediately
- `"it releases the lock when the save fails"` — the amended read-only-dir fixture, then assert a following mutation on a writable path is unaffected and the fd is not leaked
- `"it releases the lock when it removes nothing"` — task 4-1's no-removal path, followed by a successful mutation
- `"it acquires exactly once per mutation"` — with the bound lowered to a few tens of ms, assert `Set` returns well inside it (a nested acquire would block to the bound)
- `"it creates the config directory before acquiring"` — store path under a directory that does not exist; assert the mutation succeeds and both files land
- `"it fails through the sidecar acquire when the config directory cannot be created"` — a store path under a directory whose parent denies creation; assert the returned error is the acquisition failure, that `hooks.json` is absent, and that the same call would reach the emission point task 5-3 adds
- `"it never unlinks the sidecar"` — assert the file survives a mutation, a no-op mutation and a failed mutation
- `"it keeps the sidecar beside an overridden hooks path"` — a store at a non-default path; assert the lock file sits beside it
- `"it still classifies a write failure as write-failed-temp-create"` — the amended fixture

**Edge Cases**:
- Locking `hooks.json` itself would provide **no exclusion at all**: `fileutil.AtomicWrite` renames a temp file over the target, so the target's inode is replaced on every write and a lock held on the pre-rename inode is a lock on an unlinked file. The reproduction must exercise that rename specifically — two writers usually serialise by luck and would pass against a broken lock
- The directory is created **before** acquisition: the sidecar cannot be created inside a directory that does not exist, so acquiring first would make the very first `hook set` on a fresh machine fail permanently. `MkdirAll` is idempotent, races benignly, and mutates nothing the lock protects
- `MkdirAll`'s error is discarded, not returned: an uncreatable directory must fail through the acquire so directory failure and sidecar failure share one branch and one emission point. The amended write-failure fixtures all run against a directory that already exists, so nothing else in the suite covers this
- The hold spans the **whole** read-modify-write, never each file operation: taking a shared lock to read and an exclusive lock to write reopens the identical window
- Locks are never nested. `flock` is held per open file description, so a second acquisition from the same process blocks against the caller's own hold and resolves only at the bound — the exported methods must reach the file through the unexported helpers
- Every early return inside the hold releases it, and the fd is closed on every path — unlike `AcquireDaemonLock`, whose caller deliberately retains it for the process lifetime
- The read-only-directory write-failure fixtures now fail at the sidecar's `O_CREAT` unless the lock file is pre-created while the directory is writable; a `0500` directory still permits opening an existing file for writing
- Two `*Store` values in one process hold separate open file descriptions and are a faithful model of two processes; a duplicated fd would not be
- `Save`/`SaveAudited` have no production caller after Phase 3. Making them locking instead of deleting them was rejected: they take a whole-file map, so a caller composing load-then-`Save` would reintroduce the very window this closes
- The bounds and the poll interval are package-level `var`s the unit lane lowers through their test seams; no test asserts a production figure as a timing measurement
- There are **two** bounds, and the split is load-bearing: `lockTimeout` (2s) for every mutation and every ordinary read, and a near-zero `snapshotLockTimeout` for the sweep's advisory pre-read alone. Without it a sweep blocked by a wedged writer would spend one full bound on the shared acquire and another on the exclusive one, parking the daemon's 1s tick for twice the ceiling the design states
- Scope boundary: the shared read path, the timeout WARN and the CLI's exit behaviour belong to tasks 5-2 and 5-3. Here a timed-out mutation simply returns the sentinel having written nothing

**Context**:
> The lock is taken on a dedicated file (`<hooks.json path>.lock`) for a mechanical reason, not a stylistic one. The precedent this copies, `state.AcquireDaemonLock`, is correct precisely *because* it locks a dedicated `daemon.lock`.
>
> Acquisition is bounded rather than unbounded because `flock` being kernel-released on process death rules out a *leaked* lock but not a *held* one: a holder suspended by a signal or stuck on a hung filesystem keeps the lock for as long as it lives, and an unbounded acquire would park the daemon's **1s** tick loop behind it — so what stalls is the capture cycle itself, not the 10s-throttled prune that happens to sit on the same loop's idle branch. That is the blocking path the investigation named as the single riskiest part of this work unit, and the project has had a wedged daemon before.
>
> The bound is **2 seconds**: the critical section is one small-file read, a marshal and a rename — sub-millisecond in practice — so 2s sits roughly three orders of magnitude above the expected hold while staying well inside the sweep's own 10s cadence. A timeout at that bound means something is genuinely wrong rather than merely contended.
>
> This is included in the work unit because the cost is small, the pattern is already in the codebase, and the alternative is knowingly leaving a silent-data-loss path open in the one file this work unit exists to protect. It is untouched by the durable identity: a durable key does not close a lost update.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §6.1 (the open window and the interleaving), §6.2 (a sidecar lock file, never `hooks.json` itself; directory before acquisition; never unlinked), §6.3 (exclusive across the whole mutation; never nested), §6.5 (the bound and why it exists), §9.2 (the lost-update test and its inode-swap requirement).

## resume-hooks-silently-lost-5-2

### Task 5-2: A read shares the lock, and reads anyway when it cannot

**Problem**: After task 5-1 every mutation holds the sidecar exclusively, but every read still goes through the unlocked exported `Load` — so a reader has no way to wait out a mutation in progress, and the sweep's own advisory pre-read is indistinguishable from a mutation to the lock. Making reads exclusive instead would be worse than leaving them unlocked: during a restore of a 40+ pane working set every hydrate helper calls `LookupOnResume` at once, and a blanket-exclusive read lock would serialise all of them for no benefit. Reads also cannot be allowed to *fail* on a lock problem — a pane that came back and hydrated to a bare shell because a lock file was busy would be this work unit reintroducing its own symptom on the one command whose job is to restore it, and `portal doctor` reporting a hook problem because of a lock would be the same failure in diagnosis form.

**Solution**: Route every read through one unexported shared-lock helper that acquires `LOCK_SH` with the same bounded acquire, releases before it returns, and on any acquisition failure reads unlocked anyway after one DEBUG record — `op=load-unlocked`, with `via` naming the caller.

**Outcome**: Concurrent readers proceed in parallel and never block each other; a read that cannot take the lock returns its data regardless, leaving one DEBUG breadcrumb naming which caller degraded; no read holds a lock past its own return; and no read creates anything on disk, so `portal doctor` and `hook list` keep having no write side effect at all on a fresh install.

**Do**:
- In `internal/hooks/lock.go` add the shared acquire: the same `acquireLock` with `openFlags` of `os.O_RDONLY` — **no `O_CREAT`** — `flockMode` of `unix.LOCK_SH`, and the bound supplied by the caller. Carry one line of comment on the omitted `O_CREAT`, wording left to the executor.
- In `internal/hooks/store.go` add unexported `loadSharedBounded(via string, bound time.Duration) (hooksFile, error)`: attempt the shared acquire at that bound; on **any** error (absent sidecar, absent directory, unreadable file, or the bound elapsing) emit exactly one `logger.Debug("load-unlocked", "op", "load-unlocked", "via", via, "error", err)` and fall through to the non-locking `load()`; on success `defer f.Close()` and return `load()`, so the shared hold is released when the read returns and is never handed to the caller. Add `loadShared(via string) (hooksFile, error)` as its one-line delegation at `lockTimeout` — the bound every ordinary read takes.
- Give the exported reads a `via` parameter and route all three through `loadShared`: `Load(via string)`, `List(via string)`, `Get(key, via string)`. A per-call parameter rather than a `Store` field is required because `portal doctor` hands the *same* `*Store` value to the sweep (`internal`) and to `checkStaleHooks` (`doctor`) in one run.
- Add one further **exported** read, `LoadSnapshot(via string)`, delegating to `loadSharedBounded(via, snapshotLockTimeout)`. It exists because the sweep's advisory pre-read is in `cmd` and cannot reach an unexported helper, and it is the only read that takes the short bound; its doc comment says so and says why — a cycle that takes the sidecar twice must use it for the pre-read, or a wedged writer costs that cycle two full bounds. The bound must be a parameter and must never be derived from the `via` value: `via` is a log attr, `internal` happens to identify the pre-read uniquely today, and binding a blocking bound to a breadcrumb makes renaming one a change in the daemon's worst-case stall.
- `LookupOnResume` (`internal/hooks/lookup.go`) calls `store.loadShared("hydrate")` in-package, so its exported signature is unchanged. Task 3-3's empty-key early return stays ahead of it: an empty key takes no lock, reads no file and emits no record.
- Re-point the three production read call sites: `cmd/run_hook_stale_cleanup.go`'s advisory pre-read passes `"internal"` through the short-bound read, `cmd/doctor.go`'s `checkStaleHooks` passes `"doctor"`, and `cmd/hooks.go`'s `hooksListCmd` passes `"cli"`. Leave the non-locking `load()` inside a mutation's exclusive hold emitting nothing — it is not degraded, it is already exclusive.
- Re-point the read assertions across `internal/hooks/store_test.go`, `internal/hooks/lookup_test.go`, `cmd/hooks_test.go`, `cmd/doctor_test.go` and `cmd/run_hook_stale_cleanup_test.go` for the added parameter, and add the degradation coverage below.

**Acceptance Criteria**:
- [ ] `Load`, `List`, `Get` and `LookupOnResume` take a shared `flock` on the sidecar and release it before returning; none hands the fd to its caller
- [ ] Two concurrent reads both complete without either blocking to the bound
- [ ] A read while the sidecar is held exclusively elsewhere returns its data anyway, after its bound, rather than failing
- [ ] The sweep's advisory pre-read waits at `snapshotLockTimeout`, not `lockTimeout`, so a contended sweep cycle spends one full bound in total rather than two; every other read waits at `lockTimeout`
- [ ] The short bound is selected by an explicit parameter, never from the `via` value: `LoadSnapshot` is the only read that passes `snapshotLockTimeout`, `loadShared` is the only other caller of `loadSharedBounded` and passes `lockTimeout`, and no branch anywhere reads `via` to choose a bound
- [ ] A degraded read emits exactly **one** DEBUG record per read — never one per entry — under the `hooks` component with `op=load-unlocked`, the lock error in `error` and `via` naming the caller
- [ ] `via` is `hydrate` for `LookupOnResume`, `doctor` for `checkStaleHooks`, `cli` for `hook list` and `internal` for the sweep's advisory pre-read
- [ ] `op=load-unlocked` is the only new `op` value this task introduces; no new attr key, no new `via` beyond `hydrate` and `doctor`, and no new component binding
- [ ] A read **creates nothing**: after `Load` against a path whose directory does not exist, neither the directory, `hooks.json` nor the sidecar exists afterwards
- [ ] `portal doctor` (read-only) and `hook list` leave the config directory byte-for-byte as they found it, including on a fresh install with no `hooks.json` at all
- [ ] A read with the sidecar absent degrades and returns the file's contents, or an empty map when `hooks.json` is itself absent
- [ ] The non-locking `load()` inside a mutation's hold emits no `load-unlocked` record
- [ ] `LookupOnResume` returns the registered command under a concurrently held exclusive lock, so a hydrating pane never falls through to a bare shell for a lock reason
- [ ] `checkStaleHooks`'s status and `portal doctor`'s exit code are unchanged by a degraded read
- [ ] `hook list` still takes no tmux read with zero entries and still exits 0 with empty fourth fields on a failed enumeration
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- `"it takes a shared lock for a read"` — hold the sidecar `LOCK_SH` from a second fd; assert a read returns promptly and emits no `load-unlocked` record
- `"it lets two reads proceed concurrently"` — two overlapping reads, both complete inside a lowered bound
- `"it reads anyway when the lock cannot be taken"` — hold `LOCK_EX` from a second fd with the bound lowered; assert the data comes back and the returned error is nil
- `"it degrades the sweep pre-read at the short bound"` — hold `LOCK_EX` from a second fd with `snapshotLockTimeout` raised and `lockTimeout` far higher; assert the pre-read returns after roughly the short bound and emits its `op=load-unlocked` DEBUG with `via=internal`
- `"it logs one DEBUG per degraded read"` — assert exactly one record, `op=load-unlocked`, non-empty `error`, and that a file of several entries still produces one record
- `"it names the caller in via"` — table over `LookupOnResume` → `hydrate`, `checkStaleHooks` → `doctor`, `hook list` → `cli`, the sweep's pre-read → `internal`
- `"it creates nothing when it reads"` — read against a path under a non-existent directory; assert the directory, `hooks.json` and the sidecar are all still absent, and the read returned an empty map
- `"it leaves the config directory untouched across portal doctor"` — snapshot the directory listing before and after a read-only `doctor` run
- `"it degrades when the sidecar is absent"` — writable directory with `hooks.json` present and no lock file; assert the entries come back plus one `load-unlocked` record
- `"it emits no load-unlocked record from inside a mutation"` — run `Set` under a sink; assert no such record
- `"it returns the hook for a hydrating pane while a mutation holds the lock"` — `LookupOnResume` under a held `LOCK_EX`
- `"it keeps the stale-hooks check green under a degraded read"` — `checkStaleHooks` with the lock held; same status and detail as unlocked
- `"it lists hooks under a degraded read"` — `hook list` output and exit code unchanged, fourth column intact

**Edge Cases**:
- A read's shared lock is released before the read returns and is never handed back to its caller. Otherwise the sweep's advisory pre-read would still be held when `CleanStale` takes the exclusive one, and a sweep would wait on itself — a 2s stall on the daemon's 1s tick loop every ten seconds, the outcome the bound exists to prevent
- The bound is a parameter, not a function of `via`. `via=internal` identifies the pre-read uniquely today, so a `via`-driven branch would pass every test in this task and silently restore the two-bound stall the moment a breadcrumb was renamed
- The sweep takes the sidecar twice per cycle — shared for the pre-read, exclusive inside `CleanStale` — so the pre-read waits at the near-zero `snapshotLockTimeout`. At the full bound a wedged writer would park the daemon's 1s tick for twice the stated ceiling every ten seconds, which is the very outcome the bound exists to prevent, arriving by addition
- **A read creates nothing.** The shared acquire opens the sidecar without `O_CREAT` and degrades when the file or its directory is absent, so no read ever creates the config directory. The specification fixes only "writes fail, reads proceed unlocked" for a sidecar that cannot be opened; the no-`O_CREAT` reading is settled here — see Context
- Correctness never depends on the shared lock: `AtomicWrite` replaces the file by `os.Rename`, so a reader observes the pre-state or the post-state, never a torn one. That is what makes the degradation safe rather than a compromise
- The DEBUG record is one per read, never per entry — a 42-entry file degrading must not put 42 lines in the log
- The non-locking `load()` used inside a mutation's exclusive hold emits **no** `load-unlocked` line: it is not degraded, it is already under an exclusive hold
- A lock problem must never fail a read, and the same split covers the sidecar failing to open at all
- The read lock is shared because a restore of a 40+ pane working set has every hydrate helper calling `LookupOnResume` at once; a blanket-exclusive read lock would serialise them for no benefit
- `via` reaches the store through the exported reads while `LookupOnResume` names `hydrate` in-package, so its signature need not change
- `Store.Get` has no production caller but is an exported read and must not become a second unlocked path
- `checkStaleHooks` keeps the order task 1-5 fixed — the `store == nil` and load guards, then the restore-marker read, then the enumeration — so a restore window reports its detail after the shared acquire has already resolved one way or the other, and a degraded read must not change the check's status, its detail or `portal doctor`'s exit code
- Task 3-3's empty-key early return in `LookupOnResume` precedes the acquire entirely: an empty key takes no lock and logs nothing

**Context**:
> **Decision made in planning: the shared acquire opens without `O_CREAT` and degrades when the sidecar is absent.** The specification fixes the split — writes fail, reads proceed unlocked — for a sidecar that cannot be opened, but does not say whether a read may create one. It must not: `portal doctor`'s read-only path resolves the state directory read-only and creates nothing by design, and `hook list` is a display command; a read that created the config directory and a lock file on a fresh install would give both of them a write side effect they have never had. Nothing is lost by it, because correctness never depends on the shared lock — `AtomicWrite`'s rename gives a reader the pre-state or the post-state, never a torn one. A writer establishes the sidecar; until one has, every read degrades and says so at DEBUG.
>
> The degraded read is the one genuinely new emission in this work unit's logging: DEBUG, `op=load-unlocked`, the lock error in the existing `error` attr, and `via` naming the caller. That makes three `op` values across the whole work unit — `load-unlocked` here, `touch-save-requested` from Phase 2 and `clean-stale-skipped` from Phase 1 — plus two `via` values (`hydrate`, `doctor`) and **no** attr key. The vocabulary amendment is closed at exactly those; nothing further may be invented at a call site.
>
> Failing a read would forfeit a hook for nothing. `LookupOnResume` is the hydrate helper's one call per restored pane, under the very 40-helper burst that motivated the shared lock in the first place.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §6.3 (readers take a shared lock; never nested; released before returning; the shared lock is an ordering courtesy), §6.5 (a read that cannot take the lock reads anyway; the `op=load-unlocked` DEBUG and its `via` values; the closed vocabulary amendment), §9.2 (a lock timeout degrades by side).

## resume-hooks-silently-lost-5-3

### Task 5-3: A write that cannot take the lock writes nothing

**Problem**: Task 5-1 makes a mutation that cannot take the sidecar return the sentinel with nothing written, but that outcome is currently invisible on both surfaces that matter. The log holds no record of it, so an operator raising the level because a registration went missing sees the same silence this work unit exists to remove; and at the CLI a user sitting in a shell gets whatever cobra renders from a bare error, with no assurance that `hooks.json` was left alone. The two wrong alternatives are both worse than a loud failure: writing unlocked is the lost update the whole phase exists to close, and waiting indefinitely hangs the shell the user is sitting in — and, on the daemon, parks a 1s tick loop.

**Solution**: On an acquisition failure inside a mutation, emit exactly one WARN under the operation's own existing `op` carrying `hook_key`, `via` and the lock error, return the error unchanged, and let it reach the user through `RunE` by the route every other `hook` failure already takes.

**Outcome**: A `hook set` or `hook rm` that cannot take the lock exits non-zero with the reason on stderr and one WARN in the log, `hooks.json` is byte-identical afterwards (an absent file staying absent), no `save.requested` is touched, and the pane keeps whatever token was stamped before the write.

**Do**:
- In `Store.Set`, when `acquireLock` returns an error, emit `logger.Warn("set", "op", "set", "hook_key", key, "via", via, "error", err)` and return the error — before the load, so nothing is read, classified or written. In `Store.Remove`, the same with `op` and message `rm`, returning `(false, err)`.
- Carry exactly `op`, `hook_key`, `via` and `error` on this WARN. Do **not** add `error_class`: `fileutil.ClassifyWriteError`'s floor would attribute a lock failure to a write phase that never ran. Do not add `value`: it names the command a write carried, and this mutation never opened the file.
- Introduce no new `op` value. A lock timeout is an operation *failing*, for which the component already has a shape — and it is categorically **not** the silent no-removal settled in task 4-1, which is a call that legitimately changed nothing. Nothing about this branch licenses re-emitting a breadcrumb on `Remove`'s no-removal path; that path still writes nothing, returns `(false, nil)` and emits no record at all.
- Leave `CleanStale`'s acquisition failure emitting **no** store-side WARN: it returns the wrapped sentinel and the single stood-down line is emitted once at its call site in task 5-5, so a stood-down cycle produces one line rather than two.
- Return `ErrLockHeld` wrapped with `%w` everywhere it propagates, so `errors.Is` still matches at the sweep's call site; never match on the error's text, tmux's or the OS's.
- Leave `cmd/hooks.go` alone apart from what tasks 4-1/4-2 already settled: `hooksSetCmd` and `hooksRmCmd` return the store's error from `RunE`, `rootCmd` carries `SilenceUsage`/`SilenceErrors`, and `main`'s `classify` prints the message and exits 1. Add no retry, no soft-success arm and no new exit-code plumbing. The `save.requested` touch stays where task 2-4 put it — after a nil-returning write — so a timed-out `hook set` touches nothing.
- Drive the timeout in tests through `hooks.SetLockTimeoutForTest` with the sidecar held from a second fd in the same process, in both `internal/hooks` and `cmd`.

**Acceptance Criteria**:
- [ ] A timed-out `Set` returns the error without loading, classifying or writing; a timed-out `Remove` returns `(false, err)` on the same terms
- [ ] Exactly one WARN per timed-out mutation, under the `hooks` component, with `op=set` or `op=rm`, `hook_key`, `via` and the lock error in `error`
- [ ] The WARN carries no `error_class` and no `value`, and no new `op` value is introduced anywhere in this task
- [ ] `Remove`'s no-removal path is unchanged by this task: no write, no record, `(false, nil)`
- [ ] `hook set` and `hook rm` exit non-zero on a timeout with the reason reaching stderr, and no usage text is printed
- [ ] `hooks.json` is byte-identical after every timed-out mutation, and an absent `hooks.json` stays absent
- [ ] No `save.requested` is created by a timed-out `hook set`
- [ ] A timed-out `hook set` leaves the pane's stamped token in place — no unstamp, no rollback — and a retry after the lock frees reuses that token and writes one entry
- [ ] A sidecar that cannot be opened or created at all takes the same write-side branch: the mutation fails, nothing is written, one WARN is emitted
- [ ] The returned error satisfies `errors.Is(err, hooks.ErrLockHeld)` for a timeout, through every wrap between the store and the caller
- [ ] The command returns at the bound rather than hanging: a timed-out `hook set` completes within a small multiple of the lowered bound
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- `"it writes nothing when Set cannot take the lock"` — sidecar held, bound lowered; assert the error, `hooks.json` byte-identical, and an absent file staying absent
- `"it emits one WARN under op=set for a timed-out registration"` — assert the record count, level, `op`, `hook_key`, `via` and a non-empty `error`
- `"it emits one WARN under op=rm for a timed-out removal"` — the same for `Remove`, which also returns `false`
- `"it carries no error_class and no value on the lock WARN"` — assert both attrs are absent
- `"it still emits nothing when Remove removes nothing"` — the task 4-1 contract, re-asserted here so the WARN branch cannot be read as licence to break it
- `"it matches the sentinel through the wrap"` — `errors.Is(err, hooks.ErrLockHeld)` from both `Set` and `Remove`
- `"it exits non-zero from hook set on a lock timeout"` — `cmd`, injected seams, sidecar held; assert non-zero, the reason on stderr, no usage output
- `"it exits non-zero from hook rm on a lock timeout"` — including the `--pane-key` path, which still issues no tmux call
- `"it touches no save.requested on a timed-out registration"` — assert the flag file is absent
- `"it keeps the pane's token after a timed-out registration"` — assert the stamper recorded its one call and no unstamp, then retry after releasing and assert one entry under that same token
- `"it fails the write when the sidecar cannot be created"` — mutation under a directory that permits no file creation; assert no write and one WARN
- `"it returns at the bound rather than hanging"` — assert elapsed time is within a small multiple of the lowered bound

**Edge Cases**:
- `hook set` and `hook rm` exit non-zero **with the reason** rather than hanging a shell the user is sitting in; the log line and the stderr line are both present and neither stands in for the other
- The WARN files under the operation's own `op`. `Store.Set` uses `set` even where a completed call would have classified as `modify`: `classifySet` needs the loaded file, and the timeout is what prevented the load — see Context
- A lock timeout is an operation *failing*; the silent no-removal settled in task 4-1 is a call that legitimately changed nothing. The two must not be conflated in either direction, and `rm-noop` remains unavailable
- `hooks.json` is byte-identical after a timed-out mutation and an absent file stays absent, now for a second reason (the first being task 4-1's no-removal path)
- The timeout must be distinguishable by `errors.Is` through the exported sentinel, since task 5-5's stand-down has to tell a lock timeout from a save failure and must never match on error text
- On a timed-out `hook set` the pane keeps the token stamped before the write — no rollback and no unstamp, for the reason Phase 2 gives: unstamping races a concurrent registration that may already have read the token, and the next registration reads the token back and reuses it
- No `save.requested` touch on a failed write: the touch runs only after the write returns without error
- The sidecar failing to open or be created at all takes the same write-side branch as a timeout, though only the timeout carries the sentinel — see task 5-5 for what that means at the sweep
- A write still creates the config directory before acquiring, so a fresh machine's first `hook set` is not a timeout
- The unit lane drives the timeout by lowering the package-level bound; no test asserts the production figure as a timing measurement
- The fixture holds the lock from a second fd in the same process, which `flock`'s per-open-file-description semantics make a faithful model of a second process

**Context**:
> A write that cannot take the lock does not write, because an unlocked write is precisely the lost update this phase exists to close.
>
> Two of the three degradation surfaces need nothing new from the `hooks` component's vocabulary, because a lock timeout is an operation failing and the component already has a shape for that (`internal/hooks/store.go:73,103,144` — `logger.Warn(<op>, "op", <op>, …, "via", via, "error", err)`). `hook set` and `hook rm` emit that WARN under their own `op` and `hook_key`, and the error is returned up through cobra so the reason reaches the user on stderr by the route every other `hook` failure already takes.
>
> **Decision made in planning: `Store.Set`'s timeout WARN files under `op=set`, never `op=modify`.** The specification names the operation's own `op` as `set`/`modify`/`rm`, but the `set`-versus-`modify` distinction is `classifySet`'s verdict on the loaded file, and a mutation that timed out never loaded it. `set` is the method's own verb, is already in the closed vocabulary, and reports the truth available at the point of failure; inventing a lock verb to dodge the question is what the fixed three-value amendment forbids.
>
> **Decision made in planning: no `error_class` on this WARN.** `error_class` is the closed classification of `fileutil.AtomicWrite`'s phases, and its fallback attributes an unrecognised error to a write. A lock timeout happened before any write, so classifying it would put a false phase in the log; omitting an existing attr key is not a vocabulary change.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §6.5 (a write that cannot take the lock does not write; `hook set`/`hook rm` exit non-zero with the reason; what each surface emits; the sidecar failing to open takes the same split), §6.2 (the directory is created before acquisition), §4.1 (no rollback of a stamp on a failed write), §2.2 (the `save.requested` touch runs only after a successful write), §9.2 (a lock timeout degrades by side).

## resume-hooks-silently-lost-5-4

### Task 5-4: The sweep decides what is stale under its own lock

**Problem**: The sweep reads `hooks.json` twice — once at its call site in `runHookStaleCleanup` to decide whether there is anything to do, and once inside `CleanStale` — and it compares whatever it holds against a live token set that comes from a tmux enumeration. Two windows survive task 5-1's exclusive hold, and both delete entries that are not stale. If the delete set were computed from the call-site read and handed down, an entry written by `hook set` between the two reads would be deleted on the strength of a snapshot taken before it existed — the exact interleaving this phase closes. And because the pane enumeration is a tmux read that must sit outside the lock, it is always older than the mutation it feeds: today the enumeration runs *first*, so a `hook set` landing in the gap stamps its pane and writes a token-shaped entry the enumeration never saw, and the shape rule deletes it — a hook vanishing seconds after the command reported success, which is the loss this work unit exists to remove.

**Solution**: Give `CleanStale` both inputs — the live token set and the call-site snapshot's key set — and have it derive the delete set itself under its own exclusive lock, with the call-site snapshot taken **before** the pane enumeration so it can only ever narrow that set.

**Outcome**: An entry registered after the sweep's snapshot — and therefore after the enumeration that follows it — survives the sweep; every deletion is decided from the file as it stands under the exclusive hold; and no lock is held while tmux is called.

**Do**:
- Change the signature to `func (s *Store) CleanStale(liveTokens []string, snapshotKeys []string) ([]string, error)`. Inside the exclusive hold task 5-1 added: load through the non-locking `load()`, compute the candidates with Phase 1-1's single unexported `staleKeys(h, liveTokens)` — unchanged, still taking no lock of its own — then drop every candidate absent from a set built from `snapshotKeys`. Rewrite the doc comment to state that the snapshot may narrow the delete set and may never widen it.
- Never route through the exported `StaleKeys`: Phase 1-1's source guard still applies, and an acquisition from inside the exclusive hold is not re-entrant, so it would block to the bound and stand the prune down every cycle.
- Reorder `runHookStaleCleanup` (`cmd/run_hook_stale_cleanup.go`) to: the restore-marker stand-down (task 1-3) → the call-site snapshot taken through task 5-2's short-bound read at `via="internal"` (never the ordinary `Load`, whose full bound would let a wedged writer cost this cycle two bounds) → `lister.ListAllPaneHookKeys()` → the existing counts DEBUG line, still carrying `panes` (row count) and `entries` (snapshot size) → the row-counting empty-pane-read guard (tasks 1-4 / 2-1) → `liveTokensFrom(rows)` → `CleanStale(tokens, keys of the snapshot)`. Keep the load-error branch's existing WARN-and-return-err where it is, now above the enumeration.
- Keep the zero-persisted-entries silent early return inside the empty-live-set branch exactly as it is, and keep the enumeration-error branch's existing WARN-and-return-nil.
- Add one further silent early return immediately before `CleanStale`: when the snapshot holds no keys, return nil without calling it. Nothing can be stale when nothing is persisted, and after task 5-1 the call is no longer free — it creates the config directory, creates `<hooks.json path>.lock` and takes an exclusive hold. Without the return, an install that has never registered a hook gets a lock file beside a `hooks.json` that does not exist and a pointless exclusive hold in `hook set`'s way every ten seconds, for a sweep with nothing to sweep. Place it immediately before `CleanStale` and nowhere earlier, so the enumeration, the counts DEBUG line and every existing branch are untouched; the only thing lost is the `reaped=0` DEBUG line for that case.
- Feed `onRemoved` only from `CleanStale`'s returned slice, never from anything computed at the call site.
- Re-point `internal/hooks/store_test.go`'s `TestCleanStaleRemovesExactlyStaleKeys` (`:760`), which compares the result against a `StaleKeys` prediction, at the two-input signature, and the `TestCleanStale` / `TestCleanStaleLogging` fixtures that call the one-input form.

**Acceptance Criteria**:
- [ ] `CleanStale` takes the live token set and the snapshot key set and derives the delete set itself, inside its own exclusive hold, from the file it loaded there
- [ ] The delete set is every key in the file under the lock **and** in the snapshot **and** absent from the live set **and** either token-shaped or empty
- [ ] A key absent from the snapshot is never deleted, however stale it looks
- [ ] A key in the snapshot but no longer in the file under the lock is not reported as removed
- [ ] `runHookStaleCleanup` takes its snapshot **before** the pane enumeration: an entry written during the enumeration survives the sweep
- [ ] No lock is held during `ListAllPaneHookKeys` or the restore-marker read — the shared pre-read is released when it returns
- [ ] The snapshot is taken through the short-bound read, so a cycle blocked by a held sidecar spends one full bound in total rather than two
- [ ] Phase 1's rule is untouched: `staleKeys` is still the single implementation, `CleanStale` still never calls `StaleKeys`, and the source guard still passes
- [ ] The empty-live-set guard still counts pane **rows**, fed by neither `hooks.json` read
- [ ] The zero-persisted-entries early return still short-circuits silently, and the enumeration-error branch is unchanged
- [ ] A cycle whose snapshot holds no keys returns before `CleanStale` is called: no lock is taken, no config directory is created and no `<hooks.json path>.lock` appears
- [ ] `onRemoved` is invoked exactly for the keys `CleanStale` deleted, so `doctor --fix` never prints `Pruned stale hook:` for a removal that did not happen
- [ ] `runHookStaleCleanup` remains `CleanStale`'s only production caller
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- `"it deletes a key present in the file, in the snapshot and absent from the live set"` — the ordinary reap
- `"it retains a key the snapshot did not hold"` — seed the file with K after taking the snapshot; assert K survives with a live set that excludes it
- `"it retains an entry written during the pane enumeration"` — a fake lister whose `ListAllPaneHookKeys` writes a token-keyed entry as a side effect; assert the entry survives the sweep (it would be deleted if the snapshot were taken after the enumeration)
- `"it holds no lock while enumerating"` — the fake lister acquires the sidecar exclusively from a second fd during its call and fails the test if it cannot
- `"it derives the delete set from the file under the lock, not from the snapshot"` — a key in the snapshot that another writer removed before `CleanStale` runs is not reported as removed
- `"it still retains a non-token-shaped key"` — Phase 1's shape rule intact through the new signature
- `"it still deletes an empty key present in both the file and the snapshot"`
- `"it feeds onRemoved exactly what was deleted"` — assert the callback keys equal the returned slice
- `"it counts pane rows for the guard, not tokens"` — rows present with every token empty; no stand-down, nothing deleted
- `"it returns silently with zero persisted entries and an empty live set"` — no record, no callback
- `"it takes no lock when nothing is persisted"` — live rows present, no `hooks.json` and no config directory; assert the sweep returns nil and that neither the directory nor the sidecar exists afterwards
- `"it keeps the enumeration-error branch unchanged"` — the existing WARN and `return nil`, with no snapshot-driven side effect
- `"it forbids CleanStale calling StaleKeys"` — Phase 1-1's source guard, re-run against the new signature

**Edge Cases**:
- Handing `CleanStale` a delete list computed at the call site reopens the exact interleaving this phase closes: an entry written by `hook set` between the two reads would be deleted on the strength of a snapshot taken before it existed
- The call-site read is taken **before** the pane enumeration because the enumeration is a tmux read outside the lock and is therefore always older than the mutation it feeds. An entry written after the snapshot was necessarily stamped after it too, so a key the snapshot does not hold can never have been judged by that enumeration
- The snapshot may narrow the delete set; it may never widen it
- The shape rule stays Phase 1's single unexported `staleKeys`, which takes no lock of its own; `CleanStale` still never routes through `StaleKeys`, because an acquisition from inside its own exclusive hold is not re-entrant and would stand the prune down every cycle at the bound
- The live set is the non-empty token subset while the empty-live-set guard counts pane **rows** — the two questions must not be conflated, and neither `hooks.json` read feeds that guard
- No tmux call sits inside any lock: the shared pre-read releases when it returns, so the enumeration and the guard on it both resolve with no lock held, and the restore-marker read already precedes both
- The restore stand-down still runs first, so a restore window takes no lock at all
- The empty-live-set stand-down now fires with a snapshot already taken and released; that harmless discard is not a reason to reorder the snapshot back after the enumeration
- The zero-persisted-entries early return inside the empty-live-set branch still short-circuits. A zero-entry snapshot with a non-empty live set now short-circuits too, immediately before `CleanStale` — after task 5-1 that call creates the config directory and the sidecar and takes an exclusive hold, which the daemon would otherwise repeat every ten seconds on an install with no hooks at all
- `onRemoved` must be fed what was actually deleted under the lock rather than what the snapshot predicted, or `doctor --fix` prints `Pruned stale hook:` for removals that did not happen
- `runHookStaleCleanup` is `CleanStale`'s only production caller (pinned by `cmd/hooks_cleanstale_single_caller_guard_test.go`), so the signature change is contained

**Context**:
> The sweep reads `hooks.json` twice: once at its call site to decide whether there is anything to do, and once inside `CleanStale`. `CleanStale` receives the live token set and the call-site snapshot's key set, and derives the delete set itself, under its own lock.
>
> Ordering the call-site snapshot **before** the enumeration is what closes the gap the tmux read opens. A `hook set` landing in that gap stamps its pane and writes its entry *after* the enumeration; that entry's token is absent from the live set and is token-shaped, so the shape rule would delete it — a hook vanishing seconds after the command reported success.
>
> The shared lock on the pre-read is an ordering courtesy rather than a correctness requirement, and it is released when the read returns — which is also what keeps the sweep from waiting on itself when `CleanStale` takes the exclusive one.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §6.3 (the stale decision is computed under the exclusive lock, never from the pre-read; the call-site read is taken before the pane enumeration and may only narrow the delete set), §6.4 (the locked region covers the file only), §5.2 (the shape rule has one implementation), §5.4 (the guard counts pane rows), §9.2 (the sweep half of the lost-update test).

## resume-hooks-silently-lost-5-5

### Task 5-5: A locked-out sweep stands down and says so

**Problem**: After task 5-4 the sweep's mutation can fail for a reason that is not a defect in the store at all: another writer held the sidecar past the bound. Left as it stands, that error propagates through `runHookStaleCleanup`'s `return err`, where the daemon renders it as the generic `hooks stale-cleanup failed` WARN on its own logger and `portal doctor --fix` swallows it entirely — printing no prune lines, which a user reasonably reads as "nothing was stale" when in fact the prune never ran. That is the same silence this work unit exists to remove, and it is invisible to the one grep an operator has: the sweep already reports two stand-down reasons on a single line shape (`reason=restoring` at DEBUG, `reason=empty-pane-read` at WARN), and a lock timeout would be the odd one out.

**Solution**: Recognise the timeout by `errors.Is` on the exported sentinel, stand the cycle down on the shared `op=clean-stale-skipped` / `via=internal` line at WARN with `reason=lock-timeout`, report it through `onSkipped`, and add the third entry to the reason→phrase table so `portal doctor --fix` prints `Skipped stale hook prune: hooks.json is locked`.

**Outcome**: A locked-out cycle deletes nothing, leaves `hooks.json` byte-identical, emits exactly one WARN naming the reason and the lock error, tells a `--fix` user that the repair did not run without touching the exit code, and retries on the next 10s cadence with no retry loop, no backoff and no parked tick.

**Do**:
- Declare `skipReasonLockTimeout = "lock-timeout"` in `cmd/run_hook_stale_cleanup.go` beside `skipReasonRestoring` and `skipReasonEmptyPaneRead`, and use it for both the `reason` attr and the doctor-side rendering so the logged value and the printed line cannot drift.
- In `runHookStaleCleanup`, on a non-nil error from `store.CleanStale`: when `errors.Is(err, hooks.ErrLockHeld)`, emit through `hooksLogger` a **WARN** whose message and `op` are both `clean-stale-skipped`, with `via=internal`, `reason=skipReasonLockTimeout` and the lock error in the existing `error` attr; invoke `onSkipped(skipReasonLockTimeout)` nil-safely; and `return nil`. Every other error keeps today's `return err` unchanged.
- Add `lock-timeout` → `hooks.json is locked` to `pruneDoctorStaleHooks`'s reason→phrase table (task 1-4 built it as a table for exactly this).
- Change nothing about `checkStaleHooks`: in the same window it reads unlocked through task 5-2's degradation and reports whatever went un-pruned as stale, which is the honest answer and is what keeps the exit code driven solely by the post-repair diagnosis.
- Add no retry, no backoff and no escalation inside the cycle; the daemon's throttle already retries on the next 10s cadence and `maybeRunHookCleanup` must see nil so it emits no second WARN of its own.
- Drive the timeout in tests by lowering the bound through `hooks.SetLockTimeoutForTest` and holding the sidecar exclusively from a second fd, at both call sites (`runHookStaleCleanup` directly and `doctor --fix` through `Execute`).

**Acceptance Criteria**:
- [ ] A `CleanStale` lock timeout stands the cycle down: nothing is deleted, `hooks.json` is byte-identical, and `runHookStaleCleanup` returns nil
- [ ] Exactly one WARN per stood-down cycle, under the `hooks` component, with `op=clean-stale-skipped`, `via=internal`, `reason=lock-timeout` and the lock error in `error`; the daemon emits no second WARN
- [ ] The level is WARN, unlike the restore window's DEBUG — a lock that will not yield is an anomaly
- [ ] `onSkipped` is invoked once with `lock-timeout`, and a nil `onSkipped` (the daemon's call shape) is safe on this third branch
- [ ] `portal doctor --fix` prints `Skipped stale hook prune: hooks.json is locked` on the same writer and in the same repair block as its `Pruned stale hook:` lines
- [ ] `portal doctor --fix`'s exit code is unaffected by the stand-down — it stays driven solely by the post-repair diagnosis
- [ ] In the same window `checkStaleHooks` reads unlocked and reports whatever went un-pruned as stale, rather than reporting a lock problem as a hook problem
- [ ] A save failure (or any other non-sentinel error) still returns as an error and is never reported as a stand-down
- [ ] The stand-down is recognised by `errors.Is` on the exported sentinel and never by matching error text
- [ ] A fully contended cycle costs **one** `lockTimeout` in total, not two: the pre-read waits at the near-zero `snapshotLockTimeout` and only `CleanStale` waits at the full bound, so the daemon's 1s tick is not parked for twice the stated ceiling; there is no retry loop, backoff or escalation inside the cycle
- [ ] Both call sites behave identically — the daemon's throttled idle branch and `doctor --fix`
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- `"it deletes nothing when the sweep cannot take the lock"` — sidecar held, bound lowered, a genuinely stale token-shaped entry present; assert `hooks.json` bytes unchanged
- `"it logs the stand-down at WARN with reason=lock-timeout"` — assert level, component, `op`, `via`, `reason` and a non-empty `error`
- `"it emits exactly one WARN per stood-down cycle"` — assert the record count, and that the daemon's generic `hooks stale-cleanup failed` line is absent
- `"it reports the lock stand-down to the caller"` — one `onSkipped("lock-timeout")`, `onRemoved` never
- `"it survives a nil onSkipped on the lock branch"` — the daemon's call shape
- `"it prints the skipped-prune line for a locked file in doctor --fix"` — Execute and assert the exact string `Skipped stale hook prune: hooks.json is locked`
- `"it leaves the doctor --fix exit code to the post-repair diagnosis"` — a stand-down with an otherwise healthy install still exits 0; with a genuinely failing check it still exits non-zero
- `"it reports the un-pruned entry as stale in the same window"` — `checkStaleHooks` under the held lock returns its ordinary count via the degraded read
- `"it still returns an error for a save failure"` — a write-denied fixture; assert the error propagates and no skipped line or WARN under `clean-stale-skipped` is emitted
- `"it never matches on error text"` — a non-sentinel error whose message contains the sentinel's words is not treated as a stand-down
- `"it costs one bound for a fully contended cycle"` — hold the sidecar exclusively from a second fd with `snapshotLockTimeout` near-zero and `lockTimeout` lowered; assert the cycle's elapsed time is close to one `lockTimeout` and demonstrably under two
- `"it retries on the next cadence"` — release the lock and run the sweep again; the stale entry is reaped, with no state carried from the stood-down cycle

**Edge Cases**:
- `reason=lock-timeout` is the third value on the shared `op=clean-stale-skipped` / `via=internal` line shape and keeps the **WARN** level — a lock that will not yield is an anomaly, unlike the restore window's expected state
- The daemon supplies a nil `onSkipped` and must stay nil-safe on this third branch
- A save failure keeps its existing error return rather than being misreported as a stand-down, and the discrimination is `errors.Is` on the sentinel — never the error's text
- A stand-down and a removal cannot occur in the same cycle, so the skipped line and the `Pruned stale hook:` lines never interleave; assert placement in the `--fix` output rather than an interleaved sequence
- The exit code stays driven solely by the post-repair diagnosis: a stood-down prune neither fails nor passes `doctor`
- `checkStaleHooks` in the same window reads **unlocked** and reports whatever went un-pruned as stale — the read and write sides degrading in opposite directions is deliberate, not an inconsistency
- The sweep retries on the next 10s cadence with no retry loop, no backoff and no escalation inside the cycle, because a deferred prune costs nothing while stale entries are inert
- The daemon's 1s tick must not be parked, so a fully contended cycle costs one `lockTimeout` in total. The sweep takes the sidecar twice — shared for the pre-read, exclusive here — and task 5-2 gives the pre-read the near-zero `snapshotLockTimeout` for exactly this reason; a test asserting only "a small multiple of the bound" would not catch the two-bound regression
- `hooks.json` is byte-identical across a stood-down cycle
- **A sidecar that cannot be opened at all is not a stand-down.** The closed reason set carries one lock value, `lock-timeout`, which names the bound; an open failure keeps the existing error return, so the write still does not happen but it is reported as the anomaly it is rather than relabelled — see Context
- The unit lane drives the timeout by lowering the package-level bound and holding the sidecar from a second fd; no test waits out the production figure

**Context**:
> The daemon sweep skips this cycle with a WARN and retries on the next 10s cadence — a deferred prune costs nothing, since stale entries are inert. `portal doctor --fix`, the sweep's other call site, skips the same way, emits the same WARN, and additionally prints one line naming the skipped prune alongside its `Pruned stale hook: <key>` output: a repair that silently did not run is the silence this work unit exists to remove. It does not fail the command — the exit code stays driven solely by the post-repair diagnosis, which reports whatever went un-pruned as stale.
>
> Distinguishing the three reasons is the point: an operator raising the level because a hook vanished needs one grep to answer whether the prune stood down and why, rather than reading three indistinguishable lines by eye.
>
> **Decision made in planning: only the bound elapsing produces `reason=lock-timeout`.** The specification's split has writes fail when the sidecar cannot be opened at all, and this cycle's write does fail — but the reason vocabulary it fixes is closed at three values and `lock-timeout` names the bound specifically. Reporting an EACCES on the lock file as a timeout would put a false cause in the log for a condition that will not resolve on the next cadence, so that case keeps the existing error return, which the daemon already surfaces as a failed cleanup.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §6.5 (a write that cannot take the lock does not write; the sweep's skip and `doctor --fix`'s extra line; the exit code is untouched), §5.4 (one skip-line shape, three reasons, and their levels), §5.1 (the `onSkipped` callback and the three fixed `doctor --fix` lines), §9.2 (a lock timeout degrades by side).
