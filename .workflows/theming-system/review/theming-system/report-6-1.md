TASK: theming-system-6-1 — Strict Write-Path Decode With Create-On-Absent And Abort-On-Undecodable

ACCEPTANCE CRITERIA:
- An absent prefs.json is created by Save, including when its parent directory does not exist (AtomicWrite MkdirAlls).
- A present but malformed file (`{`, `{"a":1,}`, `not json`) aborts: error returned, no temp file survives, bytes byte-identical.
- A zero-byte file aborts (present, not valid JSON) — pinned deliberately.
- A non-ErrNotExist read error (a 0000-mode file) aborts like malformed and returns the OS error.
- A wrong type on a declared field (`{"session_list_mode":5,"theme":"nord"}`) does not abort: the write proceeds and `theme` is preserved.
- A top-level non-object document (`[1,2]`, `"x"`) aborts.
- Unrecognised values in valid JSON (`{"session_list_mode":"sideways"}`) do not abort.
- mutate's fn returning false writes nothing, returns nil, leaves the file byte-identical (including when absent).
- The tolerant readFile / Load / LoadThemeKeys behaviour is unchanged — Phase 5 task 5-1's tests pass unedited.
- Writes still go through fileutil.AtomicWrite; the strict read is reachable only from the savers.
- internal/prefs still imports only the standard library and internal/fileutil.

STATUS: complete

SPEC CONTEXT: §8.9 ("Concurrent instances and prefs writes") requires every prefs writer to read-modify-write immediately before the write, and discriminates two conditions: an **absent** file proceeds and creates ("nothing to merge and nothing to lose" — the most common write in the product, since §8.1 leaves a fresh install with no prefs file), while a **present-but-unusable** file (malformed JSON or I/O failure) aborts and never becomes an overwrite. The spec is explicit that this needs *two decodes that differ* — the load path stays tolerant per §8.1, the write-path re-read judges syntax — because a tolerant decode removes the abort's only trigger, letting one `s` keypress erase `session_list_mode`, `theme_migrated`, every theme key and the retained raw `appearance`. Unrecognised *values* in syntactically valid JSON are explicitly not "unusable" (hand-editing must not lock the user out). §8.9 also pins that the merge lives inside `prefs` behind field-specific save methods (the rejected alternative was an exported whole-record Load/Save that any caller could use to clobber the file), and that writes continue through `fileutil.AtomicWrite` so all keys land in one atomic write. §13.6 names this the one path whose failure mode is silent, permanent destruction of a user's config and requires file *creation* coverage, not just merge coverage.

IMPLEMENTATION:
- Status: Implemented (mechanism intact; later phases refactored around it without changing its contract)
- Location:
  - `internal/prefs/store.go:111` `readBytes` — shared read step (absent → `(nil,false,nil)`; other read error returned verbatim). Extracted by a later dedupe task; both decodes now share it.
  - `internal/prefs/store.go:125` `readFile` — tolerant load-path decode, unchanged in behaviour (unmarshal error → zero record, no error).
  - `internal/prefs/store.go:145` `readFileStrict` — strict write-path decode with the `errors.As(*json.UnmarshalTypeError) && typeErr.Field != ""` carve-out returning the partially-populated record; every other decode error returned verbatim.
  - `internal/prefs/store.go:168` `mutate` — strict re-read → apply fn(record, existed) → write; `false` skips the write, a strict-read error is returned verbatim with nothing written.
  - `internal/prefs/store.go:211` `Save` — routed through `mutate`.
  - `internal/prefs/store.go:299` `write` → `atomicWrite` (`fileutil.AtomicWrite`, which MkdirAlls the parent — `internal/fileutil/atomic.go:54`).
- Notes:
  - All five savers in the phase (`Save`, `SaveTheme`, `SaveThemeSlot`, `SaveMigrationMarker`, `SaveTranslation`) inherit create-on-absent and abort-on-undecodable through the single `mutate` chokepoint — no re-implementation anywhere, matching the phase-boundary intent.
  - `readFileStrict` has exactly one production caller (`mutate`); `mutate` has exactly five (the savers). Criterion "the strict read is reachable only from the savers" holds.
  - The `Field != ""` discriminator is correct against `encoding/json` semantics: a field-level type error is saved and decoding continues (siblings still populated), while a top-level mismatch yields an empty `Field` and a wholly zero record. The rejected-alternative shape (exported whole-record Load/Save) was not introduced — `prefsFile` and `mutate` stay unexported.
  - Later-phase revisions that touched this code are amendments, not drift: the `migrationMarker` total unmarshaler (6-3) sits *upstream* of the carve-out so the marker never reaches it; comment-trim passes (11-3, 12-7, `fee1927d`, `915e7fcb`) replaced the original essay-length doc blocks — including the "DELIBERATE BEHAVIOUR CHANGE" annotation the task asked for on `Save` — with one-line godocs. The surviving `Save` comment still states the abort behaviour, so the substantive requirement is met under the amended comment standard.
  - The behaviour-change consumer is as the task described: `internal/tui/model.go:2457` is `_ = m.modePersister.Save(m.sessionListMode)`, so the new abort is non-fatal and, unlike before, non-destructive.

TESTS:
- Status: Adequate (with two small redundancy/consistency notes below)
- Coverage:
  - `internal/prefs/store_write_path_test.go:168` `TestSave_CreatesAbsentFile` — both absent-path cases (file absent; parent directory absent too) via the shared `absentPathCases()` table.
  - `:184` `TestSave_AbortsOnUndecodable` — the consolidated `undecodablePrefsCases()` table covering truncated object, trailing comma, junk, unterminated-object-carrying-real-values, **zero-byte file**, and four top-level mismatches (array, string, number, boolean). Each case asserts the *error class* (`*json.SyntaxError` vs `*json.UnmarshalTypeError` with empty `Field`), plus byte-identity and no `.atomic-` leftovers via `assertUntouched`. The task's separately-named `TestSave_AbortsOnMalformedJSON` / `_AbortsOnEmptyFile` / `_TopLevelTypeMismatchAborts` were folded into this table by task 13-2 — an intentional later single-sourcing, and every named case survives as a labelled row.
  - `:202` `TestSave_AbortsOnReadError` — 0000-mode file, asserts `errors.Is(err, os.ErrPermission)` and byte-identity, skipping when the mode bits deny nothing.
  - `:232` `TestSave_WrongTypedFieldDoesNotAbort` — proves both halves: the write proceeds preserving `theme`, and (second subtest) the offending field normalises to zero while `theme_dark`/`appearance` survive — which is the accepted consequence the task recorded.
  - `:264` `TestSave_UnrecognisedValueIsNotUnusable` — unrecognised mode/appearance/slug plus an undeclared key.
  - `:311` `TestMutate_DecliningMutatorWritesNothing` and `:342` `TestMutate_ReportsFileExistence` — the decline path for present *and* absent files, and the `existed` flag both ways.
  - `internal/prefs/read_shared_test.go:63` `TestDecodePaths_ShareTheReadAndSplitOnTheDecode` — the load/write split pinned head-to-head on the same corrupt file (tolerant absorbs, strict returns `*json.SyntaxError`). This is the single most valuable test for this task: it pins the *difference* the abort depends on, which no single-path test can.
  - Tolerant-path regression: `internal/prefs/store_test.go:12` `TestLoad` (missing/empty/corrupt/unrecognised → ModeFlat) and `internal/prefs/theme_keys_test.go:91` `TestLoadThemeKeys_TolerantDecode` are present and unedited by this task; `theme_keys_test.go:176` `TestSave_PreservesThemeKeys` still pins the RMW merge.
  - Leaf: `internal/prefs/leaf_guard_test.go` unedited and non-vacuous (it asserts `internal/fileutil` is still a dependency). `go list -deps` excludes test files, so `read_shared_test.go`'s `themetest` import does not weaken it.
- Notes:
  - Not over-tested overall: the undecodable table's nine rows each exercise a distinct decoder outcome, and the error-class assertion means a row cannot pass for the wrong reason (e.g. a top-level array accidentally taking the syntax branch).
  - One genuine duplicate and one fixture inconsistency, both non-blocking — see the notes below.

CODE QUALITY:
- Project conventions: Followed. `internal/prefs` stays the leaf CLAUDE.md mandates (no `internal/log`; the abort is reported by returning). Writes stay on `fileutil.AtomicWrite`. No `t.Parallel()`. No new logging or error classification inside the leaf.
- SOLID principles: Good. `mutate` is a single-responsibility chokepoint; the savers hold only their own field policy. Keeping `prefsFile` and `mutate` unexported preserves §8.9's "no caller can clobber the file wholesale".
- Complexity: Low. `readFileStrict` is one branch deeper than `readFile`; `mutate` is three statements.
- Modern idioms: Yes — `errors.Is`/`errors.As` typed discrimination, a closure-based RMW rather than a copied read-then-write in five places, `os.ReadFile`.
- Readability: Good. The `existed` parameter reads clearly at each save site (`_` where irrelevant, named where load-bearing).
- Issues: None blocking. One comment on the adjacent `migrationMarker` type makes a claim about `readFileStrict` that the carve-out falsifies (note below).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `internal/prefs/store.go:56-57` — the comment "a field-level error would zero the tolerant load's whole record and abort every write's strict re-read" is half false: `readFileStrict`'s `Field != ""` carve-out (`store.go:157`) *absorbs* a field-level `*json.UnmarshalTypeError` rather than aborting, which is exactly what `TestMigrationMarker_WrongTypeDoesNotZeroTheRecord` (`migration_marker_test.go:208`) pins for other declared fields. Replace with: `// Decodes any JSON value without error: a field-level error would zero the tolerant load's whole record.` (The claim entered with task 6-3 and survived the comment trims; it describes 6-1's function, hence the note here.)
- [quickfix] `internal/prefs/store_test.go:114-145` — `TestSaveWritesAtomically` now duplicates `TestSave_CreatesAbsentFile`'s "the parent directory is absent too" case (`store_write_path_test.go:168`): same absent parent, same `Save(ModeByTag)`, same by-tag assertion, same `.atomic-` leftover scan. Delete it, or reduce it to the one assertion the newer test does not make — that the on-disk bytes are indented (`"session_list_mode": "by-tag"` with a space).
- [quickfix] `internal/prefs/store_write_path_test.go:202-228` — `TestSave_AbortsOnReadError` hand-rolls the chmod-0000 / probe-read / skip / restore fixture that `themetest.DenyRead` now owns and that the sibling `read_shared_test.go:48` (same package) already uses. Swap the staging block for `themetest.DenyRead(t, path)`, keeping the explicit `os.Chmod(path, 0o644)` before `assertUntouched` since `DenyRead` only restores at cleanup.
