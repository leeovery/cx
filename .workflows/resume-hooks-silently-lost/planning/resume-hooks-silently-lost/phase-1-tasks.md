---
phase: 1
phase_name: Make the reaper shape-aware and audible
total: 5
---

## resume-hooks-silently-lost-1-1

### Task 1-1: The stale rule retains keys it cannot judge

**Problem**: `hooks.Store.CleanStale` (`internal/hooks/store.go:200`) deletes every persisted key absent from the live pane set, via the exported `StaleKeys` (`:184`) which applies that unqualified rule. The rule is about to become destructive: once the hook key becomes a pane token, every entry already on disk is old-format, absent from the live set, and reaped within ~10s of the upgrade — the "let the sweep absorb the old entries" outcome the specification rejects outright for this change, because it silently destroys every existing hook on the install. The reaper also cannot tell "the pane is gone" from "this entry has not been converted yet"; nothing anywhere in the repo can currently distinguish the two.

**Solution**: Introduce one shape predicate (in `internal/session`, derived from the nanoid generator's own width and alphabet) and one unexported staleness function (in `internal/hooks`) that applies it: a key is stale iff it is absent from the live set **and** is either token-shaped or empty. Route the exported `StaleKeys` and `CleanStale` through that single function, with `CleanStale` calling it directly rather than back through `StaleKeys`.

**Outcome**: A non-token-shaped key is retained untouched on every run by both the daemon sweep and `portal doctor --fix`; a token-shaped key absent from the live set is still deleted; an empty key is deleted; `portal doctor` exits 0 with retained non-token-shaped entries present; and the staleness rule exists in exactly one place, applied by every reader of staleness.

**Do**:
- Add `func IsTokenShaped(s string) bool` to `internal/session`, in a new file beside `naming.go`: true iff the byte length of `s` equals `suffixLen` **and** every byte of `s` occurs in `NanoIDAlphabet`. Read both values from their existing declarations — no literal `6`, no restated charset, no regexp — so a change to either moves generation and recognition together. Use bytes for both the length and the membership test so no multi-byte input can satisfy the pair.
- Add unexported `func staleKeys(persisted hooksFile, live []string) []string` to `internal/hooks/store.go`: build the live set from `live`, then collect each persisted key that is absent from it **and** satisfies `key == "" || session.IsTokenShaped(key)`. This adds an `internal/session` import to `internal/hooks`; it is acyclic (`session`'s transitive dependencies do not include `hooks`).
- Reduce exported `StaleKeys` to a delegation to `staleKeys`, signature unchanged, and make `CleanStale` call `staleKeys` directly — never `StaleKeys`. Its doc comment must state that a key it cannot judge is retained.
- Add a unit-lane source guard in `internal/hooks` built on `sourceguardtest.PackageGoFiles(dir, false)` + `sourceguardtest.ForEachFuncCall`, failing if a call named `StaleKeys` appears inside the `CleanStale` declaration.
- Re-point the existing fixtures so both lanes stay green, by this rule: a fixture that seeds a key and asserts it **is removed** must seed a token-shaped literal (6 characters from `[A-Za-z0-9]`, absent from that fixture's live set); a fixture whose key is seeded to **match a live pane** keeps the positional key it already matches (the live enumeration is still positional in this phase); a fixture asserting the empty-live-set guard's retention should seed token-shaped keys so the guard, not the new shape rule, is what it measures. Sites: `internal/hooks/store_test.go` (`TestCleanStale`, `my-session:0.1` / `:0.2`), `cmd/run_hook_stale_cleanup_test.go` (`a:0.0` / `b:0.0`), `cmd/doctor_test.go` (`seedStalePruneFixture`, `TestDoctorStaleHooksCheck`, `sessA:0.0` / `sessB:0.0`), `cmd/doctor_fix_theme_test.go:104-106`, `cmd/state_daemon_hook_cleanup_test.go` (`stale:0.0` / `live:0.0` / `a:0.0` / `b:0.0`), and the integration seeds at `cmd/cleanstale_transient_listpanes_shared_test.go:48-50`, `cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:91-94`, `cmd/bootstrap/transient_listpanes_helpers_integration_test.go:104-105`, `cmd/state_daemon_hook_cleanup_integration_test.go:43,89-92`.

**Acceptance Criteria**:
- [ ] `session.IsTokenShaped` derives its width from `suffixLen` and its charset from `NanoIDAlphabet`; neither value is restated as a literal anywhere in the predicate
- [ ] `internal/hooks` holds exactly one staleness implementation; `StaleKeys` delegates to it and `CleanStale` calls it directly, proven by a source guard that fails on a `StaleKeys` call inside `CleanStale`
- [ ] A non-token-shaped key absent from the live set is retained by `CleanStale` and is not reported by `StaleKeys`, on every run and from both call sites (daemon sweep and `portal doctor --fix`)
- [ ] A token-shaped key absent from the live set is still deleted; a token-shaped key present in the live set is still retained
- [ ] An empty (`""`) key is deleted
- [ ] A cycle whose every candidate is retained writes no file and emits no batch summary (`hooks.json` bytes unchanged)
- [ ] `portal doctor` reports the stale-hooks check as passing and exits 0 with only retained non-token-shaped entries present, because `checkStaleHooks` inherits the rule through `StaleKeys` with no change at its call site
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass with the re-pointed fixtures

**Tests**:
- `"it accepts a six-character alphanumeric key as token-shaped"`
- `"it rejects a five-character and a seven-character key"` — the width boundaries either side of `suffixLen`
- `"it rejects a key carrying a character outside the alphabet"` — table over `:`, `.`, `-`, `_`, ` `, and an otherwise-valid value with punctuation
- `"it rejects an empty key as token-shaped"`
- `"it rejects a multi-byte input"` — a string of accented or CJK runes whose byte length is 6 and one whose rune count is 6; neither is token-shaped
- `"it rejects every old-format key shape"` — `<name>:0.0`, `<portal-id>:12.3`
- `"it retains a non-token-shaped key absent from the live set"` — `CleanStale` returns it in neither the removed set nor the file delta
- `"it deletes a token-shaped key absent from the live set"`
- `"it deletes an empty key"`
- `"it writes no file and emits no summary when every candidate is retained"` — assert `hooks.json` bytes are identical and the sink holds no `clean-stale` record
- `"it retains a non-token-shaped key across the daemon sweep"` — through `runHookStaleCleanup` with a non-empty live set
- `"it retains a non-token-shaped key across doctor --fix"` — Execute `doctor --fix` with injected `DoctorDeps`; the entry survives and no `Pruned stale hook:` line names it
- `"it exits 0 from portal doctor with only retained non-token-shaped entries present"`
- `"it forbids CleanStale calling StaleKeys"` — the source guard, which must also fail loudly if it scans no files

**Edge Cases**:
- Predicate width boundaries: 5 and 7 characters both rejected; the empty string rejected as token-shaped (its deletion comes from the explicit empty arm, not from the predicate)
- Characters outside `NanoIDAlphabet` — `:` and `.` in particular, since their absence from the alphabet is what makes an old-format key permanently unjudgeable
- Multi-byte input counted by bytes rather than runes: the length test and the membership test must use the same unit, so a 6-byte multi-byte string fails membership and a 6-rune string fails length
- An all-retained cycle must take the existing zero-removals early return: no write, no per-key line, no summary
- Fixtures seeded with a live positional key must not be converted to tokens in this phase — the live enumeration is still positional, so a token seed there would invert the test's meaning

**Context**:
> The retention rule is on shape, not on a call site, deliberately: the protection travels with the rule, so there is no "guard at the daemon versus inside `CleanStale`" split and no window in which one `doctor --fix` run destroys every unconverted entry. Whether the reaper deletes is **not** otherwise changed — this is not a conversion to full retention.
>
> The predicate's home is `internal/session` because `suffixLen` is unexported there; putting it in `internal/hooks` beside a restated `^[A-Za-z0-9]{6}$` is the named failure mode — a later change to the width would silently stop every key being token-shaped and the reaper would start retaining what it should delete, with nothing failing anywhere. Because the values are derived rather than duplicated, no literal-binding guard test is needed for the predicate itself.
>
> An empty key is neither token-shaped nor old-format (an old-format key always carries `:` and `.`), so the retention rule has nothing to protect in it; deletion is its only route out of the file short of a hand edit.
>
> Old-format entries do not accumulate without bound: a live pane's entry is never stale, a closed pane's entry is absorbed as it always has been, and the retained set is the old-format residue only, which a one-time out-of-band conversion clears. No migration code ships as part of this work.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §3.2 (key shape and the predicate's home), §5.2 (shape-aware deletion, one implementation), §8.1/§8.3 (why no migration code ships and what makes that safe), §9.2/§9.3 (reaper shape-awareness test, fixtures to re-point).

## resume-hooks-silently-lost-1-2

### Task 1-2: The reaper names the entry it reaped

**Problem**: When a hook disappears, the log cannot say what was lost. `CleanStale` emits one DEBUG line per removed key carrying only `hook_key` (`internal/hooks/store.go:220`), and at the production default level the only surviving record is the batch summary `hooks: clean-stale op=clean-stale entries=N`. Both named instances in the investigation had to be reconstructed by correlating a registration breadcrumb against a bare count. The key alone would not answer the question anyway: at the moment of deletion the pane is by definition absent from the live enumeration, so there is nothing to resolve the key against, and the entry holding the command is the thing being removed.

**Solution**: Promote the existing per-key DEBUG line to INFO and have it carry the removed entry's `on-resume` command in the existing `value` attr alongside `hook_key`, keeping the batch summary as it is.

**Outcome**: At the production default level, one INFO line per removed key names both the key and the command that was lost, so a reaped hook is recoverable from a single log line; no duplicate DEBUG line survives, and the batch summary and `portal doctor --fix`'s `Pruned stale hook: <key>` stdout are unchanged.

**Do**:
- In `CleanStale` (`internal/hooks/store.go`), change the per-key `logger.Debug("clean-stale", …)` to `logger.Info` and add the existing `value` attr carrying the entry's `on-resume` command, read from the loaded map `h` (which still holds the entry — deletion happens on the `kept` clone) before `Save`.
- Keep the message and `op` as `clean-stale`, `via` as `internal`, and the emission position ahead of `Save` unchanged. Add no new `op` value and no new attr key: the per-key line and the batch summary are distinguished by their attrs (`hook_key` + `value` versus `entries` + `took`), not by a new verb.
- Leave the zero-removals early return, `storelog.EmitCleanStaleSummary` and its failure arm exactly as they are.
- Re-point the existing log assertions in `internal/hooks/store_test.go` (the clean-stale logging subtest around `:809-880`, which currently expects 2 DEBUG + 1 INFO) at 3 INFO records, partitioned by the presence of `hook_key` versus `entries`, and compared as a set because map iteration fixes no order.

**Acceptance Criteria**:
- [ ] Each removed key produces exactly one INFO record under the `hooks` component with `op=clean-stale`, `via=internal`, `hook_key=<key>` and `value=<the removed on-resume command>`
- [ ] No DEBUG record is emitted for a removed key — the line is promoted, not duplicated
- [ ] The batch summary `op=clean-stale entries=N via=internal took=…` is still emitted once per cycle with removals
- [ ] A removed key whose entry carries no `on-resume` event still produces its line, with `value` empty
- [ ] A removed key carrying several events produces exactly one line, whose `value` is its `on-resume` command
- [ ] A cycle removing nothing emits neither per-key lines nor a batch summary
- [ ] When `Save` fails after the per-key lines were emitted, those lines stand and the summary's WARN arm fires alongside them, and `CleanStale` still returns the error with the file unchanged
- [ ] `portal doctor --fix` stdout is byte-identical to before for the same removals

**Tests**:
- `"it logs one INFO per removed key carrying the key and the removed command"` — two removals, three INFO records, no DEBUG
- `"it emits an empty value for a removed entry with no on-resume event"` — seed `{"<token>": {"on-exit": "x"}}`
- `"it emits one line for a key holding several events"` — seed a key with `on-resume` plus another event; assert a single record and `value` equal to the `on-resume` command
- `"it emits no per-key line and no summary when nothing is removed"`
- `"it keeps the per-key lines and warns in the summary when the save fails"` — deny the write (read-only dir or an undeletable path), assert the per-key INFO records plus the WARN summary carrying `error`/`error_class`, and the returned error
- `"it leaves doctor --fix stdout unchanged"` — Execute `doctor --fix` over a stale token-shaped entry and assert the `Pruned stale hook: <key>` line

**Edge Cases**:
- An entry with no `on-resume` event: the line is still emitted with an empty `value` — silence about a removal is the failure this task exists to remove
- A key holding several events: one line per key, not per event
- Save failure after emission: the per-key lines have already claimed removals that did not land; the summary's WARN arm is the correction and both must be present in the same capture
- Zero removals: the existing early return must still short-circuit before any emission
- Per-key line ordering is map-iteration order; assertions must compare sets, never sequences

**Context**:
> The command is what was actually lost, the store is holding it at the instant it deletes it, and `value` is already in the `hooks` component's vocabulary (it rides `op=set` today), so recording it adds no attr key and makes a reaped hook recoverable from one line rather than from a correlation hunt.
>
> Keeping the DEBUG line alongside the INFO one would put two lines per key in the log at exactly the level an operator raises to when investigating a loss, and the DEBUG line carries nothing the INFO line does not.
>
> The `hook list` location column added later in this work unit does not reach this case — it renders locations for entries that still exist.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §5.3 (the reaper names what it deleted), §9.2 (reaper shape-awareness / naming test).

## resume-hooks-silently-lost-1-3

### Task 1-3: The sweep stands down while a restore is in flight

**Problem**: A restore window is a hole in the reaper's judgement. Between skeleton construction and the pane re-stamp a later phase adds, every live pane carries no token — a sweep landing there sees a full pane list, no tokens, and deletes every token-keyed entry on the machine. The daemon is already immune: `tick` reads `@portal-restoring` and returns before reaching `maybeRunHookCleanup` (`cmd/state_daemon.go:174`). `portal doctor --fix` has no such gate, and it is exactly the command a user reaches for when a reboot looks wrong. The row-counting mass-deletion guard is silent through this window by design, so it cannot cover it.

**Solution**: Widen the `cmd`-side enumeration seam with the restore-marker read and have `runHookStaleCleanup` read `@portal-restoring` before it loads the store, standing the whole cycle down when the marker is set — or when the read fails — with one DEBUG record under the `hooks` component.

**Outcome**: With `@portal-restoring` set, `runHookStaleCleanup` enumerates nothing, loads nothing and deletes nothing from either call site, and logs `op=clean-stale-skipped via=internal reason=restoring` at DEBUG; a failed marker read produces the identical outcome; the daemon's own behaviour is unchanged.

**Do**:
- Widen `AllPaneLister` (`cmd/run_hook_stale_cleanup.go:12`) to also carry the marker read by embedding `state.RestoringChecker` alongside `ListAllPaneHookKeys()`. Production callers already satisfy it — the daemon passes `deps.Client` and doctor passes `deps.HookLister`, both `*tmux.Client`. One added line in the seam's doc comment must name the marker read it now carries.
- At the top of `runHookStaleCleanup`, before the enumeration and before `store.Load`, call `state.IsRestoringSet(lister)`. On `true` **or** on a non-nil error, emit through the package's existing `hooksLogger` binding a DEBUG record: message and `op` both `clean-stale-skipped`, `via=internal`, `reason=restoring`, plus the existing `error` attr carrying the read error when the read is what failed. Then return nil.
- Never emit a WARN on this branch, by either route.
- Add the method to the three `cmd` fakes behind the seam, each defaulting to "option absent" (not restoring) and each gaining fields to drive set / read-error: `recordingHookKeyLister` (`cmd/run_hook_stale_cleanup_test.go:13`), `fakeHookLister` (`cmd/doctor_test.go:797`, a value receiver), `stubAllPaneLister` (`cmd/bootstrap_production_test.go:86`).
- Leave `tick`, `maybeRunHookCleanup` and `pruneDoctorStaleHooks` untouched by this task.

**Acceptance Criteria**:
- [ ] `AllPaneLister` carries both the pane enumeration and the restore-marker read, and `*tmux.Client` satisfies it unchanged
- [ ] With the marker set, `runHookStaleCleanup` returns nil having called neither `ListAllPaneHookKeys` nor `store.Load`, and `hooks.json` is byte-identical
- [ ] With the marker read failing, the outcome is identical to the marker being set, and the record carries the read error in `error`
- [ ] The stand-down record is DEBUG under the `hooks` component with `op=clean-stale-skipped`, `via=internal`, `reason=restoring`; no WARN is emitted on this branch
- [ ] With the marker absent, the sweep behaves exactly as it does today
- [ ] The daemon still early-returns from `tick` on the marker, so its idle branch never reaches the new read; no daemon test changes meaning
- [ ] `doctor` remains bootstrap-exempt: the marker read starts no tmux server (`show-options -g` against a dead socket errors, exit 1)

**Tests**:
- `"it deletes nothing while the restore marker is set"` — seed a stale token-shaped entry plus a disjoint live set; assert `hooks.json` bytes unchanged and the enumeration was never called
- `"it treats a failed marker read as a set marker"` — seam returns an error; same outcome, plus the `error` attr on the record
- `"it skips before loading the store"` — assert via a store path that would fail loudly if read, or by asserting zero enumeration and zero removals
- `"it logs the stand-down at DEBUG and never WARN"` — capture with `logtest.Sink`; assert level, `op`, `via`, `reason`, component
- `"it sweeps normally when the marker is absent"` — the not-found read leaves today's behaviour intact
- `"it stands the doctor --fix prune down while restoring"` — Execute `doctor --fix` with a restoring seam; the seeded stale entry survives and no `Pruned stale hook:` line is printed
- `"it leaves the daemon's tick behaviour unchanged"` — with the marker set the daemon never reaches `maybeRunHookCleanup`; with it clear, the existing hook-cleanup tests still pass

**Edge Cases**:
- A failed marker read is treated as set — the conservative direction, since a deferred prune costs nothing
- No tmux server running: the read fails and the sweep skips, which is the same end state the empty-live-set guard already produced for a down server
- The skip must precede the store load so nothing is written and no snapshot is taken
- The daemon reaches `runHookStaleCleanup` only from the idle branch that the marker already suppresses, so the added read is a second, harmless check there — its `daemonFakeCommander` returns a `CommandError` for an unknown option, which resolves as not-restoring; watch for daemon tests asserting exact tmux call sequences, which now see one extra `show-option`
- The three seam fakes are compile-time blockers: the package does not build until all three carry the method

**Context**:
> The daemon's early return in `tick` protected only the capture path before this change; it is now load-bearing for hook retention and must not be relaxed or reordered.
>
> The check goes **into** `runHookStaleCleanup` rather than onto one call site, so it travels with the rule the way shape-awareness does — `doctor --fix` inherits it without a second guard of its own.
>
> The stand-down is DEBUG and never a WARN: a restore window is an expected state, and warning through every one of them names a hazard that is being avoided rather than encountered. The other two stand-down reasons (an empty pane read, and a lock timeout added by a later phase) keep the WARN they already warrant.
>
> The marker read is reached through the same seam as the pane enumeration so both surfaces are drivable from the unit lane; it sits outside any file lock a later phase adds, alongside the pane enumeration.
>
> Consequence accepted by the specification: because a failed read counts as set, a down server stands the sweep down under `reason=restoring` rather than naming the server. The presumption is deliberate and the cost is a deferred prune.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §5.4 (sweep suppressed for the duration of a restore; one skip-line shape; the DEBUG level), §6.4 (no tmux call inside the lock), §9.2 (the sweep and the check stand down during a restore).

## resume-hooks-silently-lost-1-4

### Task 1-4: A stood-down cycle reaches the caller

**Problem**: `runHookStaleCleanup` reports removals to its caller through `onRemoved`, which `portal doctor --fix` renders as `Pruned stale hook: <key>`, but a cycle that declined to run reports nothing at all. A user who asked for a repair is shown no prune lines and reasonably reads that as "nothing was stale", when in fact the prune never ran — a repair that silently did not run is the silence this work unit exists to remove. The two ways the sweep can stand down are also indistinguishable to an operator: the restore stand-down is a new DEBUG record while the empty-live-set guard emits free-text prose on the injected bootstrap/daemon logger (`cmd/run_hook_stale_cleanup.go:46`), so no single grep answers "did the prune stand down, and why".

**Solution**: Add an `onSkipped` callback beside `onRemoved`, move the empty-live-set guard's WARN onto the shared `op=clean-stale-skipped` / `via=internal` / `reason=…` line shape under the `hooks` component (keeping its WARN level), and have `pruneDoctorStaleHooks` render the reason as one fixed line in the `--fix` output.

**Outcome**: Both stand-down reasons emit the same machine-greppable line shape at their own levels — `reason=restoring` at DEBUG, `reason=empty-pane-read` at WARN — and `portal doctor --fix` prints `Skipped stale hook prune: restore in progress` or `Skipped stale hook prune: could not read live panes` in its repair output, with the exit code still driven solely by the post-repair diagnosis.

**Do**:
- Declare the reason values as constants in `cmd/run_hook_stale_cleanup.go` — `skipReasonRestoring = "restoring"` and `skipReasonEmptyPaneRead = "empty-pane-read"` — and use them for both the `reason` attr and the doctor-side rendering, so the logged value and the printed line cannot drift.
- Add `onSkipped func(reason string)` as a further parameter of `runHookStaleCleanup`, nil-safe at every call site (the daemon at `cmd/state_daemon.go:212` passes nil; its skip is already in the log).
- Replace the empty-live-set guard's `logger.Warn("stale-hook cleanup: zero live panes parsed …")` with a `hooksLogger.Warn` carrying message and `op` `clean-stale-skipped`, `via=internal`, `reason=skipReasonEmptyPaneRead` and the existing `entries` count, then invoke `onSkipped(skipReasonEmptyPaneRead)` and return nil. Keep the WARN level and keep the zero-persisted-entries early return above it silent — no line, no callback.
- Invoke `onSkipped(skipReasonRestoring)` on the restore stand-down branch, beside its DEBUG record.
- In `pruneDoctorStaleHooks` (`cmd/doctor.go:196`) pass an `onSkipped` that prints `Skipped stale hook prune: <phrase>` to the same writer as the `Pruned stale hook:` lines, resolving the phrase from a reason→phrase table (`restoring` → `restore in progress`, `empty-pane-read` → `could not read live panes`), falling back to the raw reason value for an unmapped reason so no stand-down can print nothing. Structure it as a table rather than an if/else — a later phase adds `hooks.json is locked` as a third entry.

**Acceptance Criteria**:
- [ ] Both stand-down branches invoke `onSkipped` exactly once with their reason, and a nil `onSkipped` is safe on both
- [ ] The empty-live-set stand-down logs at WARN under the `hooks` component with `op=clean-stale-skipped`, `via=internal`, `reason=empty-pane-read` and the persisted `entries` count; the restore stand-down keeps its DEBUG level
- [ ] An empty live set with zero persisted entries returns silently: no log record, no `onSkipped` call, no output
- [ ] The enumeration-error branch is unchanged — its existing WARN on the injected logger, `return nil`, and no skipped line
- [ ] `portal doctor --fix` prints `Skipped stale hook prune: restore in progress` when the marker is set and `Skipped stale hook prune: could not read live panes` when the live set is empty with entries present, on the same writer and in the same repair block as `Pruned stale hook:` lines
- [ ] `portal doctor --fix`'s exit code is unchanged by a stand-down — it stays driven solely by the post-repair diagnosis
- [ ] `hooks.json` is byte-identical across every stand-down path

**Tests**:
- `"it reports the restore stand-down to the caller"` — `runHookStaleCleanup` with a restoring seam invokes `onSkipped("restoring")` once and `onRemoved` never
- `"it reports the empty-pane-read stand-down to the caller"` — empty live set with entries present; one `onSkipped("empty-pane-read")`, WARN record asserted by `op`/`via`/`reason`/`entries`
- `"it survives a nil onSkipped on both stand-down branches"` — the daemon's call shape
- `"it reports nothing when the live set is empty and no entries are persisted"` — no record, no callback
- `"it emits no skipped line when the enumeration errors"` — the existing WARN only
- `"it prints the skipped-prune line for a restore window in doctor --fix"` — Execute and assert the exact string
- `"it prints the skipped-prune line for an empty live read in doctor --fix"` — Execute and assert the exact string
- `"it leaves the doctor --fix exit code to the post-repair diagnosis"` — a stand-down with an otherwise healthy install still exits 0; a stand-down with a genuinely failing check still exits non-zero

**Edge Cases**:
- A nil `onSkipped` (the daemon supplies none) must not panic on either branch
- Empty live set with zero persisted entries: the existing early return stays silent — there is nothing to protect and nothing to report
- The enumeration-error branch keeps its existing WARN-and-return and emits no skipped line; the three stand-down reasons are a closed set and a hard read error is not one of them
- A stand-down and a removal cannot occur in the same cycle, so "alongside the `Pruned stale hook:` lines" means the same writer and the same repair block, not interleaving — assert placement in the `--fix` output rather than an interleaved sequence
- The empty-live-set guard must keep its WARN level while moving onto the shared shape; only the restore reason is DEBUG
- A reason with no table entry must still print a line

**Context**:
> The `reason=empty-pane-read` value is included here rather than left to a later phase deliberately: the specification defines one skip-line shape covering three reasons, the third (`lock-timeout`) belongs to the phase that introduces the lock, and no other phase touches this function — so the existing empty-live-set guard is converted here, alongside the callback that reports it.
>
> Distinguishing the reasons is the point: an operator raising the level because a hook vanished needs one grep to answer whether the prune stood down and why, rather than reading three indistinguishable lines by eye. A lock that will not yield and a tmux read that came back empty are both anomalies and keep their WARN; a restore window is an expected state and stays DEBUG.
>
> `op=clean-stale-skipped` is a new `op` value in the closed `hooks` vocabulary and is spec-governed; `reason` is an existing attr key newly carried by this component, not an addition to it. No new component binding is introduced — `cmd` already holds `hooksLogger`.
>
> A user who asked for a repair is told it did not run, but the exit code is unaffected: it stays driven by the post-repair diagnosis, whose stale-hooks check reports not-evaluable in the same window.
>
> Scope boundary: a later phase reorders the store snapshot ahead of the pane enumeration for locking reasons. Do not pre-empt that reordering here — this task keeps today's order.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §5.1 (the `onSkipped` callback and the three fixed `doctor --fix` lines), §5.4 (one line shape, three reasons, levels), §6.5 (the stand-down does not affect the exit code).

## resume-hooks-silently-lost-1-5

### Task 1-5: The stale-hook check tells the truth

**Problem**: `checkStaleHooks` (`cmd/doctor.go:280`) counts every persisted key absent from the live pane list as stale, and during a restore window that count is a lie. Its live set is a full pane list carrying no tokens, so the empty-set branch does not fire and every token-shaped key counts as stale — a read-only `portal doctor` run in that window would report every hook on the machine as lost and exit non-zero, on the command whose whole job is to tell the user whether that happened. It is also the diagnosis half of `--fix`: once the sweep stands down for a restore, the check must not contradict the repair's own report by counting what the prune deliberately did not judge.

**Solution**: Read `@portal-restoring` through the same widened seam the sweep uses, by the sweep's rule (a failed read treated as set), ordered before the live enumeration, the empty-live-set branch and the stale count, and report the check's existing not-evaluable result rather than counting.

**Outcome**: With the marker set — or its read failing — `portal doctor` reports the stale-hooks check as not evaluable with the detail `restore in progress (not evaluable)`, never contributing to the exit code; outside that window the check behaves as it does today, counting only the keys the reaper is willing to judge.

**Do**:
- In `checkStaleHooks`, after the `store == nil` and `store.Load` guards and before `lister.ListAllPaneHookKeys()`, call `state.IsRestoringSet(lister)`. On `true` or on a non-nil error, return `checkResult{name: name, status: checkNotEvaluable, detail: "restore in progress (not evaluable)"}`.
- Use that detail string verbatim: the specification fixes the `doctor --fix` output line but leaves this detail unset, and this task fixes it to match the existing `zero live panes with hooks present (not evaluable)` phrasing convention on the same check.
- Leave every other branch untouched — the hooks.json guards, the empty-live-set not-evaluable branch, the pass/fail arms and the `pluralCount` detail. The count already narrows to keys the reaper will judge, because it routes through `hooks.StaleKeys`; add no second shape filter at this call site.
- Add no nil guard for `lister`: a nil seam panics at the marker read exactly as it panics at the enumeration today, and no behaviour change is intended here.
- Extend `cmd/doctor_test.go`'s `TestDoctorStaleHooksCheck` and the `--fix` fixtures with the restore-window cases, driving the marker through the `fakeHookLister` fields added with the widened seam.

**Acceptance Criteria**:
- [ ] With the marker set, the stale-hooks check reports `checkNotEvaluable` with detail `restore in progress (not evaluable)` and no count is computed
- [ ] A failed marker read produces the identical result
- [ ] The marker read is ordered before the live enumeration, before the empty-live-set branch and before the stale count: with the marker set **and** an empty live set with entries present, the restore detail is what is reported
- [ ] Not-evaluable never drives the exit code — an otherwise-healthy install in a restore window still exits 0
- [ ] Outside a restore window, a genuinely stale token-shaped key still fails the check with its existing `N stale hook entr…` detail, even with retained non-token-shaped entries present alongside it
- [ ] With no server running, the marker read fails and the check reports not evaluable, starting no tmux server
- [ ] After a `--fix` stand-down, the post-repair diagnosis reports the check not evaluable rather than counting, and the exit code comes from the rest of the diagnosis

**Tests**:
- `"it reports not evaluable while the restore marker is set"` — assert status and the exact detail string
- `"it treats a failed marker read as a set marker"` — same status and detail
- `"it reads the marker before the empty-live-set branch"` — marker set plus an empty live set with entries; assert the restore detail, not `zero live panes with hooks present (not evaluable)`
- `"it reads the marker before counting"` — marker set plus a live set that would make a token-shaped key stale; assert not evaluable and that no failure detail is produced
- `"it keeps portal doctor at exit 0 in a restore window"` — Execute with an otherwise-healthy fixture
- `"it still fails on a genuinely stale token-shaped key alongside retained non-token-shaped entries"` — one stale token, two old-format keys; assert `checkFail` with a count of 1
- `"it reports not evaluable with no server running"` — the seam returns a connection error for both reads
- `"it reports not evaluable in the post-repair diagnosis after a stand-down"` — `doctor --fix` in a restore window: the skipped-prune line prints, the post-repair check is not evaluable, and the exit code is driven by the other checks

**Edge Cases**:
- A failed marker read is treated as set, matching the sweep and the posture `portal state commit-now` already takes
- Ordering: the marker read must precede the empty-live-set branch and the stale count, so a restore window is never reported as either "zero live panes" or "N stale"
- Not-evaluable is outside the pass/fail catalog and must never drive the exit code
- Retained non-token-shaped entries must not be counted as stale — they are inherited exclusions from the shared staleness rule, not a second filter here
- No server running: the read errors, so the check reports not evaluable rather than counting; `doctor` must still start no server

**Context**:
> The check takes the same reading as the sweep, for the same window and a different reason: the sweep must not delete what it cannot judge, and the check must not report what it cannot see. Without this, the one command whose job is to tell the user whether their hooks were lost would report exactly the loss it exists to detect, at the exact moment it is most likely to be run.
>
> The detail wording is a decision made in this task, not in the specification: `restore in progress (not evaluable)`, chosen to match both the fixed `--fix` output phrase (`restore in progress`) and this check's existing not-evaluable phrasing.
>
> The count narrowing to token-shaped and empty keys is inherited, not implemented here — both readers of staleness apply the one shared rule, which is what keeps retained old-format entries from pushing a healthy install to a non-zero exit code.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §5.4 (`checkStaleHooks` takes the same reading; count over token-shaped keys only), §5.2 (`portal doctor` stays green and keeps its "exit 0 iff all pass" contract), §9.2 (the check stands down during a restore; the doctor exit-code test).
