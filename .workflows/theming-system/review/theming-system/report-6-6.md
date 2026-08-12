TASK: theming-system-6-6 — Persist The Translation Best-Effort And Emit `theme: appearance migrated`

ACCEPTANCE CRITERIA (from plan):
1. Pending translation with `appearance: dark` + no theme keys persists `theme = tokyo-night` AND `theme_migrated = true`, emitting exactly one INFO `theme: appearance migrated` with `slug=tokyo-night`.
2. A second launch against the resulting file writes nothing and emits nothing.
3. A marker-only run (`appearance: auto`, or a theme key already set) writes the marker and emits nothing.
4. A failed write (malformed `prefs.json`, unwritable directory) emits nothing at all and leaves the condition true for retry.
5. The persist does not block the load (proven through the seam, not by timing); the load's result is identical whether the write succeeds or fails.
6. A theme committed between compute and persist survives.
7. Two concurrent instances converge on the same file content, at most one event per instance that actually persisted.
8. Event is INFO, message verbatim from §12.3's catalogue, attrs inside the closed set.
9. No test sleeps: seam substituted synchronously, restored with `t.Cleanup`.
10. The exec path (`portal open <target>`) reaches neither the dispatch nor the saver.

STATUS: complete

SPEC CONTEXT:
§10.5 splits *computing* from *persisting*: the translation is applied in memory at prefs load so the launch renders the right theme, while the write is best-effort and non-blocking — a failure means retry next launch rather than a wrong-theme flip. The theme key and the marker must land in **one** write (a gap between two writes would persist the key with the marker unset, after which the next launch writes marker-only and never emits, i.e. "translation succeeded, log says it failed"). §12.3 pins `theme: appearance migrated` at INFO, emitted on **successful persist** only — never on compute, never on a marker-only write — with its *absence* as the failure signal. §8.9 states the migration write inherits only the abort half of the persister contract: it emits **no** `theme: commit failed` (that event stays single-sited on the panel's persister) and its RMW re-read absorbs a commit another instance made in between. §10.5/§6.3 require the translation be silent to the user at runtime (no flash/notice band/banner); the CHANGELOG is the compensating channel. Ownership sits in `cmd/config.go` because `prefs` is a leaf that must not import `internal/log`.

IMPLEMENTATION:
- Status: Implemented (shape improved over the plan's sketch, no drift in behaviour)
- Location:
  - `cmd/config.go:149` — `persistTranslation(store, load.TranslatedSlug)` dispatched from `loadPrefsStore` inside the `!state.Migrated` branch, after the in-memory application; return value ignored, error not propagated, `prefsLoad` unaffected.
  - `cmd/config.go:154-158` — the seam: `var persistTranslation = func(store *prefs.Store, slug string) { go runTranslationPersist(store, slug) }`.
  - `cmd/config.go:160-171` — `runTranslationPersist`: `SaveTranslation` → return silently on `err != nil || !persisted` → otherwise `themeLogger.Info("appearance migrated", "slug", slug)`.
  - `cmd/open.go:27` — `themeLogger = log.For("theme")`, the single `cmd` binding reused (guarded by `TestThemeComponent_BoundOnceInCmd`, `cmd/open_theme_nomination_test.go:108`).
  - `internal/prefs/store.go:266-294` — task 6-4's `SaveTranslation`: one RMW write for key+marker, `persisted` only set when a theme key was actually written, and forced to `false` on a write error.
- Notes:
  - The plan sketched an inline goroutine closure; the implementation hoists the body into the named `runTranslationPersist` so the tests substitute the *production* body synchronously rather than re-implementing it. This is a strict improvement (no assert-against-your-own-copy) and preserves the seam's substitutability.
  - The plan asked for three explanatory comments (AtomicWrite's no-partial-file guarantee, retry-next-launch, the RMW re-read absorbing an in-between commit) plus a note on the `slug`/no-`slot` attr choice. Two are present (`cmd/config.go:112-116`, `146-148`); the other two were removed by the plan's own later comment-standard remediations (`e30939b2` 11-3, `c69101ca` 12-7, `a4bc7bd5`, `915e7fcb` — which explicitly strip spec citations and design essays). Per the amended intent, this is a deliberate later supersession, not drift.
  - Every criterion is honoured by the code path: marker-only and absent-file runs return `persisted=false` (`store.go:268-279`) → silent; a write error returns `false, err` → silent and the marker stays unset → the next `loadPrefsStore` re-enters the pending branch; `SaveTranslation` re-reads strictly, so a theme committed in between makes it a marker-only write.
  - Reachability: `loadPrefsStore` has exactly one production caller (`cmd/open.go:601`, `openTUI`), so the exec path and `portal doctor` (which uses `loadPrefsStoreNoMigrate`, `cmd/config.go:93`) never dispatch.
  - Concurrency: `prefs.Store` is `{path string}` only, `logtest.Sink`/`slog` are mutex-guarded, and `fileutil.AtomicWrite` uses `os.CreateTemp` unique names — so the goroutine sharing the store with the model's theme persister is safe, and simultaneous instances converge on identical bytes.

TESTS:
- Status: Adequate
- Coverage: `cmd/prefs_translation_persist_test.go` carries all eleven planned tests, one per criterion, each asserting on real on-disk bytes plus the captured `theme` event stream:
  - `TestPersistTranslation_WritesAndEmits:64` (crit. 1 — file content AND the single `INFO appearance migrated slug=tokyo-night`).
  - `TestPersistTranslation_OneShot:85` (crit. 2 — byte-unchanged file + zero events on the second load).
  - `TestPersistTranslation_MarkerOnlyIsSilent:102` — four table cases (`auto`, absent `appearance`, constant already set, slot already set) plus the absent-`prefs.json` case asserting the file is never created.
  - `TestPersistTranslation_FailureIsSilentAndRetryable:139` (malformed file; unwritable dir) + `assertStillPending`.
  - `TestLoadPrefsStore_PersistIsNonBlocking:204` — the seam records the dispatch without running it (no timing), the result is asserted identical on both success and failure, and an AST subtest pins the production var's body to exactly one `go runTranslationPersist(...)` statement — the only way to prove "off the launch path" without sleeping.
  - `TestPersistTranslation_DoesNotRevertAConcurrentCommit:258` — commits `nord` between dispatch and run, then asserts the deferred write degrades to marker-only and stays silent (drives 6-4's re-read end to end).
  - `TestPersistTranslation_ConcurrentInstancesConverge:276` — same-value determinism, loser-writes-nothing, and a genuine `sync.WaitGroup` race converging on one file with 1..N events.
  - `TestPersistTranslation_EventShape:347` — INFO, verbatim message, attr keys exactly `{component, slug}`, and an explicit closed-set membership check.
  - `TestPersistTranslation_NeverEmitsCommitFailed:174`, `TestPersistTranslation_NoFlashOrNoticeBand:382` (translating launch's frame is byte-identical to the settled launch's, with a non-empty-frame tripwire so the comparison can't be vacuous) + the compile-time `var _ func(*prefs.Store, string) = persistTranslation` at :401 proving the seam can hand the model no signal.
  - `TestOpenExecPath_NoTranslation:403` — drives the real `open` body with the tmux seams injected, asserting zero dispatches, byte-unchanged prefs and zero `theme` records.
- Notes:
  - No sleeps anywhere; every substitution restores via `t.Cleanup` (`syncPersistTranslation:28`, `recordPersistTranslation:429`). `cmd/testmain_isolation_test.go:24` additionally neutralises the seam package-wide so an unrelated test reaching `loadPrefsStore` cannot leave a goroutine writing into its own teardown — a good structural safeguard consistent with the package's `TMUX`/`PORTAL_*` poison.
  - Failure injection is real (`makePrefsDirUnwritable:504` strips the dir's write bit and restores it before `t.TempDir` cleanup, with the ordering rationale commented), not mocked.
  - Mild redundancy only: `TestPersistTranslation_NeverEmitsCommitFailed` overlaps `..._FailureIsSilentAndRetryable` — see the non-blocking note.

CODE QUALITY:
- Project conventions: Followed. Single `log.For("theme")` binding per package reused (CLAUDE.md's bind-once-per-package rule, with the third package legitimised by §8.9); package-level mutable seam matches cmd's established `*Deps` idiom; `prefs` stays a leaf (no `internal/log` import); `theme` attrs stay inside the spec-governed closed set; comments carry no spec-section or task-id references, consistent with the repo's post-remediation comment standard.
- SOLID principles: Good. Compute (`loadPrefsStore`), dispatch (`persistTranslation`), persist-and-report (`runTranslationPersist`) and merge (`prefs.SaveTranslation`) are cleanly separated; the emission decision is derived from the store's `persisted` return rather than re-deciding policy in `cmd`.
- Complexity: Low. `runTranslationPersist` is four lines with one guard; `loadPrefsStore` has two guards and one call.
- Modern idioms: Yes. `go` dispatch of a named func (not an inline closure), named results on `SaveTranslation` for the `persisted` contract, `sync.WaitGroup.Go` in the race test (Go 1.26).
- Readability: Good. Each comment states a non-obvious *why* (why `TranslatedSlug` is not zeroed, why the seam is a var, why emission is gated on `persisted`) and each holds true against the code — verified line by line, including the "only path resolution can fail the load" claim against `readFile`'s tolerant decode (`internal/prefs/store.go:125-140`).
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] cmd/prefs_translation_persist_test.go:174 — `TestPersistTranslation_NeverEmitsCommitFailed`'s two cases are covered by `TestPersistTranslation_FailureIsSilentAndRetryable` (both make the write fail and both end in `assertThemeEvents(t, sink)` with zero wants, which already proves no `commit failed` record; the explicit `rec.Msg == "commit failed"` loop is subsumed by it). Decide whether to fold the two cases in as subtests of the failure test, or keep the separate function as a named witness for §8.9's single-siting invariant.
- [idea] cmd/config.go:103-110 — `prefsLoad.TranslationPending` is written by production and read only inside `loadPrefsStore` itself and by tests (`TranslatedSlug` likewise: consumed at :141/:149 and otherwise test-only). Decide whether both stay as the load's observable contract for the migration, or drop `TranslationPending` from the returned struct and let the tests assert through the recorded dispatch instead.
