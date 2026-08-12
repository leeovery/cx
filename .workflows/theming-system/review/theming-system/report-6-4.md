TASK: theming-system-6-4 — SaveTranslation: theme key and marker in one write, no-op re-evaluated at the re-read

ACCEPTANCE CRITERIA:
- `SaveTranslation("tokyo-night")` on `{"appearance":"dark"}` writes `theme` AND `theme_migrated` in ONE write, returns `persisted=true`.
- Same call on a file whose `theme_dark` is set writes the marker only, leaves `theme_dark` untouched, returns `persisted=false`.
- Same holds for `theme` / `theme_light`, and when the key was written AFTER this instance's load-time snapshot (condition evaluated at the re-read).
- `SaveTranslation("")` on an eligible file writes the marker alone, `persisted=false`.
- A file whose marker is already `true` is left byte-identical, `persisted=false`.
- Absent file: nothing written, nothing created, `persisted=false`, nil error.
- Malformed file: aborts with the error returned, bytes byte-identical, `persisted=false`.
- Writing branch clears both slots (empty by construction, asserted anyway) and preserves `session_list_mode` and raw `appearance` verbatim.
- Exactly one `AtomicWrite` in every writing branch — never two.
- Two calls in succession persist a theme key only on the first.
- The saver emits nothing — a process-wide `logtest.Sink` records zero entries.

STATUS: complete

SPEC CONTEXT:
§10.5 (specification.md:1333) requires the theme key and the marker to land in one write: two field-specific savers each RMW, and the translation's write is best-effort/non-blocking, so a failure between them persists the key with the marker unset — the next launch then finds the marker false, sees a key set, writes only the marker, and never emits `theme: appearance migrated` (translation succeeded, log says it failed). §10.5 also pins that the event fires only when a theme key was actually persisted. §8.9 (line 920) requires §10.3's no-op condition to be evaluated at the RMW re-read against the bytes about to be merged, never against the load-time snapshot, and notes the same re-read is what lets the migration observe another instance's marker; §8.9 (line 929) gives the migration write only the abort half of the write-path rules (no create-on-absent, no `theme: commit failed`, failure signalled by the absence of the event); §8.9 (line 931) defines "skip" as skipping the theme keys, not the whole write. §10.3 (line 1301ff) separates trigger (marker) from no-op condition (any theme key set) and states mutual exclusion is satisfied trivially because the write only runs when all three keys are empty.

IMPLEMENTATION:
- Status: Implemented (branch behaviour matches the plan exactly; the plan's prescribed source comments were deliberately stripped by later phases — see Notes)
- Location: `/Users/leeovery/Code/portal/internal/prefs/store.go:262-294` (`SaveTranslation`, over task 6-1's `mutate` at :168-177 and `readFileStrict` at :145-164). Caller: `/Users/leeovery/Code/portal/cmd/config.go:161-172` (`runTranslationPersist`), which is the sole consumer.
- Notes:
  - Branch order matches the plan's 1-4 exactly: `!existed` → decline (store.go:268-270); `f.ThemeMigrated` already true → decline (:271-273); marker set then `slug == "" || any theme key non-empty` → marker-only write returning `persisted=false` (:275-279); otherwise theme key + both slots cleared + marker, `persisted=true` (:281-286). Every decision is made inside the mutator, i.e. against `readFileStrict`'s re-read bytes — §8.9's requirement — and never against a caller-held snapshot.
  - Setting `f.ThemeMigrated = true` once before the branch split (:275) is what makes "skip means skip the theme keys, not the whole write" structurally true: there is exactly one writing path and one `atomicWrite` per call by construction, not by discipline.
  - `persisted` is a first-class named result, and the post-`mutate` error check resets it to false (:288-291) so a mutator that chose to write but whose commit failed cannot report a persist that is not on disk — this is what keeps task 6-6's `theme: appearance migrated` honest (`cmd/config.go:165-170` gates the emission on `err == nil && persisted`).
  - Abort half inherited, create half not: `!existed` declines rather than creating, and an undecodable re-read propagates `readFileStrict`'s error out of `mutate` before any write. `SaveTheme`/`SaveThemeSlot` (which do create on absent) are untouched, so the two policies stay separated per §8.9.
  - Emits nothing; `internal/prefs` still passes its `go list -deps` leaf guard (`leaf_guard_test.go`) — the `internal/log`/`internal/logtest` imports are confined to `_test.go` files, which `go list -deps` does not traverse.
  - Later plan phases (11-3, 12-7, `chore(comments)` fee1927d / 915e7fcb) intentionally stripped this function's originally-extensive design-argument comment block down to a four-line godoc. Per the review context that is amended intent, not drift; the two comment notes below are about what the surviving four lines now say, not about the volume removed.

TESTS:
- Status: Adequate
- Coverage: `/Users/leeovery/Code/portal/internal/prefs/translation_saver_test.go` holds all eleven named tests from the plan, and each acceptance criterion has a matching assertion:
  - one-write + both keys on the same committed bytes — `TestSaveTranslation_KeyAndMarkerInOneWrite:47` via `onlyCommit` (:19-31), which counts at the unexported `atomicWrite` seam (`recordWrites`, theme_savers_test.go:135) — the only place a second write is observable, since the second would overwrite the first on disk.
  - existing-key skip across all three keys — `TestSaveTranslation_ExistingKeySkipsThemeKeys:72` (table over `theme`/`theme_light`/`theme_dark`), asserting the pre-existing value survives, the other two keys stay absent, the marker still rides the single commit, and `session_list_mode`/`appearance` are preserved.
  - re-read evaluation — `TestSaveTranslation_NoOpEvaluatedAtReRead:190`: a second `*Store` commits `nord` after the first instance was constructed; the test asserts `nord` survives, `persisted=false`, and that the raw bytes never contain `tokyo-night`. Its second subtest drives the same shape with `SaveMigrationMarker` and asserts byte-identity. This is the criterion most likely to regress silently and it is tested at the right seam.
  - empty slug — `TestSaveTranslation_EmptySlugIsMarkerOnly:168` asserts marker-only on both the committed bytes and disk, all three theme keys absent, `appearance` preserved.
  - already-migrated no-op — `TestSaveTranslation_AlreadyMigratedIsANoOp:131` with two seeds (the dangerous fully-eligible one and a themed one), asserting zero `AtomicWrite` calls and byte-identity.
  - absent file — `TestSaveTranslation_DoesNotCreateAbsentFile:110` over `absentPathCases()` (file absent; parent dir absent too), asserting the file is still absent, the temp dir is empty, and no `.atomic-` temp remains.
  - undecodable — `TestSaveTranslation_AbortsOnUndecodable:263` over the shared nine-case `undecodablePrefsCases()` table (syntax classes plus the four top-level type mismatches), asserting error class, `persisted=false`, byte-identity.
  - returned contract — `TestSaveTranslation_ReportsPersisted:283` (7-case matrix over every branch) plus two subtests: a failed `AtomicWrite` (injected at the seam) reports `persisted=false` with the error returned verbatim and the file untouched; and twice-in-succession persists only on the first call, with `theme` still `tokyo-night` after the second call passed `gruvbox`.
  - mutual exclusion — `TestSaveTranslation_WritesAConstant:371` asserts both slots absent on the committed bytes and on disk, with `session_list_mode`/`appearance` preserved.
  - silence — `TestSaveTranslation_IsSilent:397` installs a `logtest.Sink` process-wide, drives all five branches (writing / marker-only / spent / absent / undecodable) and asserts zero records, then emits a control line to prove the sink is live (non-vacuous).
  Assertions are structural (`assertKeysAbsent`, `assertMarkerValue` decode the JSON rather than substring-matching), so `"theme"` cannot pass by matching `"theme_light"` and an omitted marker cannot pass as `false`. Every test would fail if the corresponding branch broke.
- Notes: Mild, deliberate overlap — `TestSaveTranslation_SkipStillRecordsTheMarker:237` re-asserts the marker-only outcome already covered by `ExistingKeySkipsThemeKeys`, and `ReportsPersisted`'s table re-states the `persisted` value each per-branch test already asserts. Both were named individually by the plan and each carries something the other does not (the skip test drives the two-call *sequence*; the table documents the whole return contract at a glance), so this is not over-testing worth unwinding. No unnecessary mocking: the only seam substitution is `atomicWrite`, which is the sole way to count writes.

CODE QUALITY:
- Project conventions: Followed. `prefs` stays a leaf (guard passes); the saver logs nothing and reports by returning, leaving the `theme` component single-sited in `cmd`; the write goes through `fileutil.AtomicWrite` via the existing `write`/`mutate` chain; no `t.Parallel()` in the tests; white-box test files carry the one-line justification for being in `package prefs`.
- SOLID principles: Good. The saver adds one field-combination policy over the existing `mutate` primitive and duplicates none of the read/merge/write machinery; policy about *what* `appearance` translates to stays in `cmd/config.go` (`translateAppearance`), so `prefs` gains no knowledge of slugs or of the migration's meaning.
- Complexity: Low — four guard branches, no nesting beyond the mutator closure.
- Modern idioms: Yes. Named results used for documentation, closure capture for the out-of-band `persisted` signal, error checked once at the boundary.
- Readability: Good, with one caveat — the surviving godoc's rationale (store.go:262-265) now reads as if leaving the marker unset were the *purpose* of the single write, which inverts §10.5's argument (see the first note below).
- Issues: None affecting behaviour.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `internal/prefs/store.go:262-265` — the godoc's second clause ("a failure between separate writes would persist the key with the marker unset, and the marker must stay unset so the next launch retries") presents the two-write failure state as the desired one; §10.5's actual harm is that the next launch sees a key already set, writes only the marker, and never emits the event. Replace the block with: `// SaveTranslation records the translated theme key and the migration marker in` / `// one write, not two: a failure between separate writes would persist the key` / `// with the marker unset, so the next launch would see a key already set, write` / `// only the marker, and never emit the appearance-migrated event. Writing` / `// all-or-nothing leaves the marker unset only when nothing was written, so the` / `// next launch retries the whole translation. An absent prefs.json is a silent` / `// no-op — the translation never creates the file.` (the closing sentence also records the absent-file choice, which currently has no comment at store.go:268 although the sibling `SaveMigrationMarker` godoc carries the equivalent).
- [do-now] `internal/prefs/store.go:277` — the `slug == ""` disjunct sits unexplained beside three "user already chose a theme" conditions, which reads as an unrelated case folded into the same branch. Add above the `if`: `// An empty slug means there was nothing to translate; like an already-set` / `// key it skips the theme keys, but the marker is still recorded so the` / `// translation does not stay pending forever.`
