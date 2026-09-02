## Attempt 1

ISSUES:
- `internal/hooks/store_test.go:1041-1045` — the hooks clean-stale failed-save summary is the one failed-write case left without a `via` pin, and the only one of the ten `AssertWriteFailure` sites with no paired `AssertRecord`. It still hand-checks the level inline and asserts `via` nowhere, while its INFO summary sibling at `:979` pins `via = internal` and the analogous project failed-save block at `:472` now pins it through `AssertRecord`. `storelog.EmitCleanStaleSummary` (`internal/storelog/clean_stale.go:19`) emits `"via", "internal"` on the WARN branch, so the pin is available and will pass. This is the same "one contract asserted two ways inside the same file" the task exists to remove, in a block the executor already edited.
  FIX: replace the inline level check with the shared assertion, matching `:1095`:
  ```go
  summary := summaries.Only(t, "clean-stale summary record")
  logtest.AssertRecord(t, summary, logtest.RecordWant{
      Level:     slog.LevelWarn,
      Msg:       "clean-stale",
      Component: "hooks",
      Op:        "clean-stale",
      Via:       "internal",
  })
  logtest.AssertWriteFailure(t, summary, "write-failed-temp-create", fileutil.ErrWriteTempCreate)
  ```
  The `op`/`component` re-check against `partitionCleanStaleRecords` is redundant but harmless and matches how `:1095` and `project:465` read.
  ALTERNATIVE: add only an inline `via` check. Cheaper, but re-introduces an inline spelling of a shared property in the file the task is clearing of them. Not recommended.
  CONFIDENCE: high

NOTES:
- All four verification asks were answered. (a) The scope expansion was right and each extra site genuinely carries the same contract — but it was **four** sites, not the three reported; `internal/hooks/store_test.go:1045` is the fourth. The code is right; only the report's count is off. (b) The `via` gap is confirmed a test gap, not a production defect: `internal/project/store.go:106,199,233` each emit `"via", via` on the failed-write WARN. The new pins bite — dropping `via` from all three emissions failed exactly the three newly-pinned cases with `record missing attr "via"`, and the file was restored from a pre-mutation copy with an empty diff. (c) The alias `SetAndSave` change is a strengthening, not a change of subject: same record, same attr, presence → wraps-the-sentinel, and the already-asserted `error_class` is that sentinel's own class; `internal/alias/store.go:137` persists through `AtomicWrite`, so the wrap is real. (d) No coverage lost at any site; several tightenings.
- Both clean-stale WARN blocks replaced a last-match-wins `for` loop with a single-record terminal, which now fails on a second matching record — a tightening; both pass.
- The four remaining `errors.Is(err, fileutil.ErrWriteTempCreate)` in these files assert the store call's **returned** error, a different subject, correctly left alone.
- The residual inline `op`/`component`/`via` checks in the happy-path partition loops (`hooks:910,913,963,979`; `project:349,352,371,389,395`) are per-set assertions over multiple records rather than the five-property spelling the criteria target. Pre-existing and outside scope; worth folding per record if anyone revisits those blocks.
- Keeping the sentinel as a parameter was judged right: resolving `fileutil` inside would put a domain edge on a package every test package reaches, and the helper's contract is genuinely sentinel-agnostic, which is what lets other emissions reuse it later.
- `AssertRecord`'s `AttrString` is fatal on an absent attr, so a missing `component` aborts the subtest before `op`/`via` are reported. Pre-existing behaviour of the shared helper, unchanged here; the "reports every mismatched property" guarantee holds only for present-but-wrong attrs.
