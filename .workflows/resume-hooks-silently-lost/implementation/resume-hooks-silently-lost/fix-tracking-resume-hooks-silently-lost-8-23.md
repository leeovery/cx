## Attempt 1

ISSUES:
- Five open-coded copies of `log.Discard()` survive, so the criterion "`log.Discard()` has no open-coded copy" is unmet. The task named only `cmd/logging_capture_test.go:31` (fixed), but the identical expression `slog.New(slog.NewTextHandler(io.Discard, nil))` — byte-for-byte what `internal/log/discard.go:8` holds behind `log.Discard()` — remains at `cmd/state_daemon_hook_cleanup_test.go:33`, `cmd/state_daemon_run_test.go:171`, `cmd/state_daemon_run_test.go:1231`, `cmd/open_burst_seams_test.go:32`, `cmd/open_burst_seams_test.go:101` and `internal/tui/preview_attach_test.go:227`. `internal/log/discard_guard_test.go:35` skips `_test.go` files, so nothing catches these. Both packages already import `internal/log` from other test files, so there is no reachability obstacle.
  FIX: Replace each of the six expressions with `log.Discard()`, dropping the now-unused `io` import where it was only there for this (mirroring the edit already made at `cmd/logging_capture_test.go:31`). `cmd/state_daemon_hook_cleanup_test.go:33` becomes `return log.Discard()`; the three `Logger:`/`injectedLogger :=`/`silent :=` sites take `log.Discard()` directly.
  ALTERNATIVE: extend `internal/log/discard_guard_test.go` to cover `_test.go` files. Stronger — it prevents recurrence — but widens the task beyond the tidy it was given. Do the six replacements now; the guard widening is banked.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/bootstrap/orphan_sweep_test.go:15-16` — the second clause is a claim about what tests assert, which code-quality.md's comment discipline forbids; it is falsified the moment either assertion moves.
  OLD:
  // listPanesWarnMessage is the sweep's saver-read failure WARN, asserted both
  // present and absent across the tri-state paths.
  NEW:
  // listPanesWarnMessage is the message of the sweep's saver-read failure WARN.
- `cmd/bootstrap/stale_marker_cleanup_test.go:14-15` — the constant is the WARN's message text, not the guard itself; as written the comment misidentifies what the identifier holds.
  OLD:
  // massUnsetDeferralWarnMessage is the guard that skips a sweep which would
  // otherwise unset every marker.
  NEW:
  // massUnsetDeferralWarnMessage is the message of the WARN emitted when the
  // guard defers a sweep that would otherwise unset every marker.

NOTES:
- All four verification asks came back clean. (a) The sweep is faithful or stronger everywhere sampled — the reviewer read the full diff of every `internal/*` file plus ten `cmd` files. The `RecordingLogger` migration is a net strengthening: rendered-line `strings.Contains` probes became exact message matches plus typed attr reads, kind-checked where the old code did a bare `.Int64()`. Two sites are slightly stronger than intended and pass. (b) Deleting `OnlyRecord`/`OnlyRecordWith` was the right call — they and `Records().Only` are the same set reached two ways, exactly what the one-route rule forbids; keeping them as thin wrappers would have violated the rule the task exists to serve. Every substitution is set-identical. (c) The three surviving raw reads and the two surviving handlers are genuinely different questions, verified individually. (d) The mutation probe was re-run independently: both former lossy sites now fail on a wrong-kind value.
- `Records.Only`'s failure message dumps only the filtered set, where the deleted `Sink.only` dumped the whole sink. On the common failure — nothing matched — the new message prints `[]` and says nothing about what *was* logged, across ~100 migrated sites. Structural to putting the terminal on a plain `Records` slice, which has no back-reference to the sink. A real if minor debuggability cost of an otherwise correct design; not worth reversing.
- The mechanical sweep left several single `Record` values bound to plural or set-shaped names: `internal/restore/session_test.go:937` and `:964` (`recs := …Only(…)`), `main_panic_test.go:67` and `:89` (`exits := …`), plus redundant one-line aliases at `cmd/hooks_test.go:867-869` and `cmd/run_hook_stale_cleanup_test.go:530-531`. Cosmetic.
- `sink.Records().Only(t, "log record")` appears at ~50 sites with an identical, information-free description — the ergonomic price of the one-route rule. Correct as delivered.
- Three sites call `Only(…)` purely for its assertion and discard the record; a `_ =` would signal the intent.
- One cold-cache integration run of `cmd/bootstrap` failed and three subsequent runs passed. Not attributable: the only edits to those files are logger injection and a switch to `sink.Lines()`/`Body()`, both of which would fail deterministically rather than intermittently.

## Attempt 2

ISSUES:
- The exactly-one idiom the task set out to retire survives at three sites, all over locally-derived record slices the sweep never opened: `internal/hookstest/hooks_lock.go:106-109` (`len(got) != 1` → `t.Fatalf` → `got[0]`, inside `AssertDegradedRead`, whose own doc comment says "exactly one record"), `cmd/open_burst_run_test.go:698-701` (→ `w := warns[0]`) and `cmd/open_burst_run_test.go:767-770` (→ `s := summaries[0]`). `cmd/open_burst_run_test.go:708-710` is the count-only variant of the same. AC1 is stated absolutely, and `hookstest` is the most consequential: it is a shared cross-package helper other suites read as the house pattern. Compounding it, `UnlockedRecords` (`internal/hookstest/hooks_lock.go:90-99`) hand-writes a message filter that `Sink.RecordsWithMessage("load-unlocked")` already offers.
  FIX: Re-type `UnlockedRecords` to return `logtest.Records` and implement it as `return sink.RecordsWithMessage("load-unlocked")` — its other call sites (`cmd/hooks_read_lock_test.go:82`, `internal/hookstest/staging_test.go:33`, `internal/hooks/read_lock_test.go:47,122,215,339`) only take `len(...)`, so the type change is source-compatible — then replace `AssertDegradedRead`'s three lines with `r := UnlockedRecords(t, sink).Only(t, "load-unlocked record")`. In `cmd/open_burst_run_test.go`, declare the partition slices as `logtest.Records` (`var warns, summaries logtest.Records` at :689, `var summaries, windows logtest.Records` at :758, `var perms, summaries logtest.Records` at :838) and terminate with `w := warns.Only(t, "corrective WARN record")` / `s := summaries.Only(t, "INFO opened summary")`; the prefix-matched partition loop itself must stay, since `HasPrefix` on `Msg` is not an offered query. Converting :708 to `_ = summaries.Only(t, …)` matches what the sweep did at the equivalent sites in `cmd/bootstrap/clean_sweep_summary_test.go:99` and `internal/state/fifo_sweep_summary_test.go:235`.
  CONFIDENCE: high

- `internal/restore/session_test.go:945` reads `rec := sink.Records()[0]` two lines after `warn` was isolated by `Only` — a positional read that returns the wrong record the moment any earlier emission appears. A one-word substitution (`warn`) the sweep passed over. (Raised from the reviewer's NOTES: it is a latent defect in a file this task swept, not a pre-existing condition left untouched.)
  FIX: use `warn` rather than re-reading positionally.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/logtest/capture.go:155-158` — the doc makes a tree-wide cardinality claim ("no caller re-authors the exactly-one check") that is false today at three sites and that ordinary additive change falsifies; state what the method does, not what the rest of the tree does.
  OLD:
  // Only fails the test unless the set holds exactly one record, and returns it.
  // It is the terminal of every query chain — including the level-filtered ones,
  // whose level survives into the returned record — so no caller re-authors the
  // exactly-one check. The description names the set in the failure message.
  NEW:
  // Only fails the test unless the set holds exactly one record, and returns it.
  // It terminates any query chain, the level-filtered ones included: the level
  // survives into the returned record. The description names the set in the
  // failure message.
- `CLAUDE.md:82` — the amended `logtest` row asserts `Sink` is the tree's only capture handler, which `internal/log/log_test.go:111` (`recordingHandler`) and `internal/log/rotate_test.go` (`componentCapture`) falsify; those tests are `package log` and `logtest` imports `internal/log`, so they structurally cannot adopt `Sink`. A contributor following the rule into `internal/log` hits an import cycle with no explanation.
  OLD: `Sink` is the tree's only capture handler: a suite needing captured records constructs one rather than declaring its own `slog.Handler`.
  NEW: `Sink` is the capture handler for every suite outside `internal/log`: one needing captured records constructs a `Sink` rather than declaring its own `slog.Handler`. `internal/log`'s own in-package tests are the exception and keep their local handlers — `logtest` imports `internal/log`, so importing it back from `package log` is an import cycle.

NOTES:
- The fix round's own subject is confirmed done: all six open-coded discard constructions now call `log.Discard()`, only the canonical one and the guard's constant survive, each edited site drops `io` and adds `internal/log` with both vet lanes clean, and behaviour is identical (the shared package-level logger discards everything and `*slog.Logger` is concurrency-safe). All nine `barrierLog` shadows are renamed.
- The whole-task criteria were re-verified, not just the fix. Both named twins are gone with no residual references. The four remaining handlers are not twins: one *wraps* a `Sink` to model the production level gate, one records interleaving against non-log events, and two are structurally barred by the import cycle.
- The adoption is faithful or stronger at the ~150 sites sampled: substring matching over rendered lines became exact-message queries plus structured attr reads, `errors.Is` now runs against the live error value rather than a rendered string, first-match semantics became exactly-one, and a hand-rolled `step=` string parser is gone. Every value assertion traced survives.
- `Only`'s failure message prints the filtered set where the deleted `Sink.only` printed everything captured, so a zero-match failure shows `[]`. Prescribed by the shape the task asked for — a terminal on `Records` cannot reach the sink. A diagnostic trade, not a defect.
- Several `len(records) != 0/1` checks were deliberately left and that reads as right: they assert a count with `t.Errorf` and never retrieve the record, which `Only` (fatal, value-returning) would change.
- Where a count-only check *was* converted, `t.Errorf` became `t.Fatalf` via `Only`. Strictly a change of failure semantics against "assertions unchanged", but it only shortens a failing run and is inherent to the consolidation.
- The two surviving `Kind() != slog.KindAny` checks are subsumed by the `ErrorAttr` call on the next line, and on an absent attr the zero value reports `KindAny` and passes silently before `ErrorAttr` fatals. Harmless, and each keeps a named message for the specific regression. Leave them.
- `_ = …Only(t, …)` is redundant in Go, but reads as a deliberate assert-and-discard and the linter is silent on it.

## Attempt 3

ISSUES:
- `internal/tui/theme_slot_collapse_test.go:25-31` — the exactly-one-then-index idiom over a record set survives: `loaded := sink.RecordsWithMessage("loaded")` → `if len(loaded) != 1 { t.Fatalf(…) }` → `loaded[0].AttrString(t, "slug")`. It is the same shape migrated at ~100 other sites, in a file the pass never opened, so the report's "a re-scan now reports zero remaining exactly-one-then-index sites over any record set" does not hold.
  FIX: collapse the three statements to the terminal, exactly as done elsewhere:
  ```go
  loaded := sink.RecordsWithMessage("loaded").Only(t, "production adapter `theme: loaded` line")
  if got := loaded.AttrString(t, "slug"); got != want {
  ```
  (The adjacent `fake.slotLoads` block at `:37-42` is a plain struct slice, not a record set — leave it.)
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `CLAUDE.md` (the `logtest` table row) — the new sentence names `internal/log`'s in-package tests as *the* exception, but two `slog.Handler`s are legitimately declared outside it; a future duplication sweep greps `slog.Handler`, hits them, and has nothing to check them against.
  OLD: `Sink` is the capture handler for every suite outside `internal/log`: one needing captured records constructs a `Sink` rather than declaring its own `slog.Handler`. `internal/log`'s own in-package tests are the exception and keep their local handlers — `logtest` imports `internal/log`, so importing it back from `package log` is an import cycle.
  NEW: `Sink` is the capture handler for every suite outside `internal/log`: one needing captured records constructs a `Sink` rather than declaring its own `slog.Handler`. `internal/log`'s own in-package tests are the exception and keep their local handlers — `logtest` imports `internal/log`, so importing it back from `package log` is an import cycle. The two `slog.Handler`s still declared under `cmd` are not twins and capture no records: `cmd/open_test.go`'s `warnBypassHandler` models the production level gate (which a `Sink`, admitting every level, cannot) and forwards what survives into a `Sink`, and `cmd/bootstrap/latch_test.go`'s `orchestrationSeqHandler` appends one ordering marker into a sequence shared with non-log events.

NOTES:
- The sweep method is now checkable rather than asserted. The reviewer did not grep: it built a `go/packages` type-checked walker (scratchpad module, repo `go.mod` untouched) loading every package with `Tests: true`, reporting each `len(X)` and `X[i]` whose operand's *static type* is `logtest.Records` or `[]logtest.Record`. Run untagged (127 packages / 1168 files) and `-tags=integration` (129 / 1234), zero load errors either way — 358 sites, 49 of them indexes, every index site outside `internal/logtest` hand-read. It separately confirmed no package-local named type aliases a record slice, so the type filter cannot silently miss a set. Result: exactly the one site above.
- The `UnlockedRecords` signature change was verified genuinely source-compatible and non-weakening at all six call sites, and the filter is byte-identical in effect (`r.Msg == "load-unlocked"` either way, no component narrowing). `AssertDegradedRead` keeps all four downstream assertions.
- Criteria 2, 3 and 4 re-verified whole-task. Both named twins gone; the two remaining handlers outside `internal/log` capture nothing. Both lossy reads gone and both sites now fatal on a non-error value. The only surviving discard construction is the implementation itself and the guard's constant; no `barrierLog` shadow remains.
- Three unguarded `[0]` reads into a filtered record set remain and would panic rather than fail cleanly if the set emptied: `internal/spawn/logemit_test.go:86,89,90` (pre-existing, the sibling of a block this pass migrated), `internal/theme/union_test.go:446`, `internal/tui/theme_panel_commit_load_test.go:363`. None re-authors an accessor, so none is in the task's criteria — but `.Only(t, …)` is the natural remedy while the blocking fix is being made.
- `internal/theme/union_test.go:482` is a compound non-fatal assertion; migrating it would turn an `Errorf` into a `Fatalf`, which earlier rounds explicitly declined for count-only sites. Correct to leave.
- `cmd/bootstrap/bootstrap_test.go` asserts emptiness as `… == nil` at five sites. Correct against the documented "nil when none match" contract, but `len(…) == 0` is conventional and does not depend on that contract holding.
- `Only`'s failure message prints only the filtered set where the deleted `Sink.only` printed the whole sink, so an empty-match failure reads sparse. Inherent to the prescribed design; the `description` argument carries the diagnostic weight instead.
