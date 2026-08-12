TASK: theming-system-6-3 — The `theme_migrated` Marker Field And `SaveMigrationMarker`

ACCEPTANCE CRITERIA: (from the plan)
- `theme_migrated: true` on disk survives `Save(mode)`, `SaveTheme` and `SaveThemeSlot` unchanged.
- `"theme_migrated": "yes"`, `1`, `0`, `null`, `[]`, `{}` each decode to `false`, error from neither decode path, and do not abort a subsequent write.
- A file with a wrong-typed marker still yields its `session_list_mode` and theme keys from the tolerant load path.
- `false` is absent from the encoded JSON; only `true` appears.
- `LoadMigrationState()` returns the raw `appearance` verbatim and the marker; every degenerate file yields a zero value + nil error; only a non-`ErrNotExist` read error propagates.
- `SaveMigrationMarker()` on an existing file writes `true` and preserves `session_list_mode`, `appearance` and all three theme keys.
- `SaveMigrationMarker()` on an absent file writes nothing, creates nothing, returns nil.
- `SaveMigrationMarker()` on a malformed file aborts (error returned, bytes byte-identical).
- A file created between the load-time snapshot and the save is written to (absence judged at the re-read).
- `SaveTheme` / `SaveThemeSlot` never set or clear the marker.
- `internal/prefs` still imports only the standard library and `internal/fileutil`.

STATUS: complete

SPEC CONTEXT: §8.1 (specification.md:736-743) makes `theme_migrated` a boolean one-shot gate with a tolerant decode ("anything that is not literal `true` … decodes to `false`"), written on the first post-upgrade load only when `prefs.json` already exists, omitted when empty, and explicitly outside mutual exclusion. §10.3 (line 1295) requires the translation to gate on this explicit marker rather than on the absence of theme keys, because absence-gating is re-armable against §9.9's hand-edit escape hatch. §8.8 is the reason the field must be *declared* before any writer exists: an undeclared key is dropped on re-encode, and §8.9 makes every writer re-encode the whole file. §10.4 is why `appearance` is handed back unparsed.

IMPLEMENTATION:
- Status: Implemented (amended by task 6-4, as the plan anticipated)
- Location:
  - `/Users/leeovery/Code/portal/internal/prefs/store.go:56-67` — `migrationMarker` with the total `UnmarshalJSON` (`bytes.Equal(bytes.TrimSpace(data), []byte("true"))`, always nil error) and `MarshalJSON`.
  - `store.go:69-79` — `prefsFile.ThemeMigrated migrationMarker` tagged `json:"theme_migrated,omitempty"`. `omitempty` is honoured because the named type's kind is `bool`, so `false` is omitted even though the field carries a custom marshaler.
  - `store.go:89-92` — `MigrationState`; `store.go:199-207` — `LoadMigrationState` over the tolerant `readFile`, returning `Appearance` verbatim.
  - `store.go:249-260` — `SaveMigrationMarker` over `mutate`, returning `false` from the mutator when `existed` is false (declines silently, creates nothing).
  - `store.go:168-177` — `mutate` re-reads strictly at write time, so absence is judged at the RMW re-read rather than a stale stat.
- Notes:
  - Field ordering and tolerance are correct against §8.1. `LoadMigrationState` inherits `readFile`'s policy verbatim: absent/empty/corrupt → zero value + nil error; only a non-`ErrNotExist` read failure propagates (`readBytes`, store.go:111-121).
  - `prefs` grew only `bytes` and `strconv` — still stdlib + `internal/fileutil`; no logging, no `appearance` interpretation, no regrown enum (`appearance_api_guard_test.go` keeps the enum dead while explicitly permitting the on-disk field).
  - Task 6-4's `SaveTranslation` (store.go:266-294) became the production writer of the marker; `cmd/config.go:124,165` consumes `LoadMigrationState` + `SaveTranslation`. `SaveMigrationMarker` consequently has **no production caller** — it survives as exported API exercised only by its own tests and as an "other writer" simulator in `translation_saver_test.go:220`. That is the plan's own shape (6-3 and 6-4 were authored as separate deliverables), not drift, but it leaves dead exported surface on a leaf store; noted below.
  - The comment on `migrationMarker` (store.go:56-57) overclaims — see NON-BLOCKING NOTES.

TESTS:
- Status: Adequate (mild redundancy)
- Coverage: `/Users/leeovery/Code/portal/internal/prefs/migration_marker_test.go` covers every named plan test:
  - `TestMigrationMarker_RoundTrips` (:57) drives all five writers over a seeded `true` marker.
  - `TestMigrationMarker_TolerantDecode` (:129) — 12 cases including `"yes"`, `"true"`, `1`, `0`, `null`, `""`, `[]`, `{}`, a populated object and absent; each asserts the tolerant decode, the strict decode, *and* that a subsequent `Save` succeeds and normalises the value. The strict-decode arm is the one that pins "a hand-edited marker can never lock a user out".
  - `TestMigrationMarker_WrongTypeDoesNotZeroTheRecord` (:183) asserts mode + all three theme keys survive both decode paths and a write.
  - `TestMigrationMarker_FalseIsAbsentOnDisk` (:73) checks omission structurally via a `map[string]any` decode rather than a substring — correct, since `false` and omitted are indistinguishable by substring.
  - `TestLoadMigrationState_ReturnsRawAppearanceUnparsed` (:366) crosses 7 raw values (`"Dark"`, `"  dark "`, `"sepia"`, `""`) × marker true/false — this is the assertion that stops an enum regrowing.
  - `TestLoadMigrationState_TolerantDecode` (:411) covers missing/empty/corrupt/top-level-array/top-level-string plus the EISDIR propagation case.
  - `TestSaveMigrationMarker_{PreservesEveryOtherField,DoesNotCreateAbsentFile,AbortsOnUndecodable,AbsenceJudgedAtReRead}` (:287, :313, :328, :345) — the absent case asserts nothing at all was created in the temp dir (including no `.atomic-` residue), the abort case asserts byte-identity plus the error class, and the re-read case genuinely stats-then-lets-another-writer-create before saving.
  - `TestMigrationMarker_NotTouchedByThemeSavers` (:239) covers both directions of "outside mutual exclusion", including the hand-cleared-keys case, and guards against vacuity by asserting the saver's own key landed.
  - Leaf constraint: `leaf_guard_test.go:17` runs `go list -deps` and fails on any internal dep besides `fileutil`, with an anti-vacuity check.
- Notes: Failure messages are behavioural and would tell a future reader *why* the assertion exists. Redundancy is mild but real: `TestMigrationMarker_FalseIsAbsentOnDisk`'s "no writer invents the key" subtest (:110-125) is a superset-duplicate of `TestMigrationMarker_NotTouchedByThemeSavers`'s "a theme saver never sets the marker" (:257-269) — same seed, same assertion, overlapping writer sets. Not under-tested anywhere I can find.

CODE QUALITY:
- Project conventions: Followed. `prefs` stays a leaf (no `internal/log`), `AtomicWrite` is the only write path, no `t.Parallel()`, tests are white-box only where the seam genuinely is unexported (`readFile`/`readFileStrict`/`mutate`) and state that reason in the file header.
- SOLID principles: Good. The marker's tolerance is a property of the *type*, not of each decode site, so neither `readFile` nor `readFileStrict` has to special-case it — one place to reason about, and the two callers stay dumb.
- Complexity: Low. `SaveMigrationMarker` is a four-line mutator over the shared RMW.
- Modern idioms: Yes — `strconv.AppendBool`, `bytes.Equal`, `errors.As`/`errors.Is`, no reflection.
- Readability: Good. Comments carry the *why* (re-encode erasure, tolerant-vs-strict split) rather than restating code.
- Issues: One comment claim is falsified by the sibling code path (below). No security or performance surface — a single small file read/write per call.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `/Users/leeovery/Code/portal/internal/prefs/store.go:56-57` — the type comment's second clause is false. `readFileStrict`'s carve-out (`store.go:157`, `errors.As(err, &typeErr) && typeErr.Field != ""`) deliberately absorbs a wrong-typed *declared* field and returns `(f, true, nil)`, so a field-level error would **not** "abort every write's strict re-read"; `store_write_path_test.go:230-231` states exactly this ("theme_migrated never reaches the carve-out — its own total decode absorbs a wrong type one layer earlier"). Replace the two lines with: `// Decodes any JSON value without error: a field-level error would zero the` / `// tolerant load's whole record, so a hand-edited marker would take the mode` / `// and every theme key down with it.`
- [quickfix] `/Users/leeovery/Code/portal/internal/prefs/store.go:249-260` — `SaveMigrationMarker` has no production caller: task 6-4's `SaveTranslation` writes the marker on every production path (`cmd/config.go:165`), including the marker-only case (`slug == ""`). Delete the method and `migration_marker_test.go`'s `TestSaveMigrationMarker_*` block plus the `SaveMigrationMarker` entry in `markerWriterCases` (:53), and switch the "other writer" at `translation_saver_test.go:220` to `SaveTranslation("")`. `unused` will not catch it (exported), so it will otherwise sit as permanent unreferenced store API on a leaf package.
- [quickfix] `/Users/leeovery/Code/portal/internal/prefs/migration_marker_test.go:110-125` — the "no writer invents the key" subtest duplicates `TestMigrationMarker_NotTouchedByThemeSavers`'s "a theme saver never sets the marker" (:257-269): identical seed, identical assertion, and its writer set is that one plus `Save`. Delete the subtest and add a `Save` case to the mutual-exclusion test's loop if the `Save` coverage is wanted.
- [do-now] `/Users/leeovery/Code/portal/internal/prefs/store.go:89` — `MigrationState` is exported with no doc comment, and the one property a future reader most needs is missing. Add above it: `// MigrationState is the one-shot translation gate's input: the retained raw` / `// appearance value, returned verbatim and unparsed, and whether the` / `// translation has already been recorded. Mapping dark/light to a slug belongs` / `// to the caller — prefs has no appearance enum and must not regrow one.`
