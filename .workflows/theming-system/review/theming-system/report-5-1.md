TASK: theming-system-5-1 — prefs.json Decodes The Three Theme Keys And Preserves Raw `appearance`

ACCEPTANCE CRITERIA (from plan task 5-1):
1. `LoadThemeKeys()` on `{"theme":"nord"}` returns `{Theme:"nord"}` with empty Light/Dark and nil error.
2. Missing file, empty file, corrupt JSON, and a file with none of the three keys all return a zero `ThemeKeys` with nil error; only a non-`ErrNotExist` read error propagates.
3. Unrecognised values (`"Nord"`, `"../evil"`, `"  nord "`) are returned verbatim — no rejection, trim or case change.
4. `Save(ModeByTag)` on a file holding `appearance`, `theme`, `theme_light`, `theme_dark` preserves all four; the written JSON re-decodes to identical `ThemeKeys`.
5. Unset keys are absent from the written JSON (`omitempty`).
6. An existing `"appearance":"dark"` survives ten successive `Save` calls byte-identically.
7. `theme_migrated` is neither declared nor written.
8. `internal/prefs` still imports only stdlib + `internal/fileutil` (leaf guard green, unedited).
9. `prefs.Appearance`, `LoadAppearance`, `SaveAppearance` still exist and behave as before (task 5-7 deletes them).

STATUS: complete

SPEC CONTEXT:
§8.1 (specification.md:722-749) fixes the on-disk shape as three flat string keys plus a `theme_migrated` marker alongside `session_list_mode`, with tolerant decode "exactly as dumb as today" (missing/empty/unrecognised falls to the shipped default per field, no type probing) and `omitempty` across the theme keys and the retained `appearance` so an unset key is absent rather than empty-stringed. It explicitly rejects a polymorphic `theme` field and an always-object form.
§8.8 (specification.md:893-895) makes the raw `appearance` field load-bearing: prefs.json decodes into a plain struct, so any undeclared key is dropped on re-encode and §8.9 makes every writer re-encode the whole file — deleting the field silently erases the user's pin at the first `s` keypress, defeating §10.4's downgrade guarantee. The field is "a plain string that is read and preserved, never parsed": the type and its API go, the slot in the file stays.
§8.9 (specification.md:897-915) assigns the merge, the field-specific savers and the strict write-path decode to Phase 6 — correctly out of scope here.

IMPLEMENTATION:
- Status: Implemented (two criteria intentionally superseded by later plan tasks — see below).
- Location:
  - `internal/prefs/store.go:69-79` — `prefsFile` carries `Appearance`, `Theme`, `ThemeLight`, `ThemeDark` (all `omitempty`), with the never-delete trap warning on `Appearance` (`:71-72`).
  - `internal/prefs/store.go:81-87` — `ThemeKeys` type, doc pinning that validation/defaulting/tiebreak belong to the resolver.
  - `internal/prefs/store.go:189-197` — `LoadThemeKeys` reading through the shared tolerant `readFile` (`:125-140`), returning the raw strings and propagating only a non-`ErrNotExist` read error.
  - Consumers wired: `cmd/config.go:123`, `cmd/doctor_theme.go:96`.
  - Task-commit provenance: `de3a5331` matches the task's `Do` list line for line (fields + `ThemeKeys` + `LoadThemeKeys`, `Save`/`write` untouched, `theme_migrated` deliberately undeclared with the boundary comment).
- Notes (deliberate later supersessions, not drift):
  - Criterion 7 (`theme_migrated` undeclared) was superseded by task 6-3 (`32d45cba`), which declares `ThemeMigrated migrationMarker` (`store.go:78`) — exactly the boundary 5-1's own comment predicted ("Phase 6 declares the field before its first writer exists"). The field is `omitempty` over a named bool, so an unset marker is still absent from the written JSON, preserving §8.1's omit-on-write rule.
  - Criterion 9 (`Appearance` enum/API still present) was superseded by task 5-7 (`59d966e6`), which deleted the enum and its accessors per §8.8's "Dies" row. The on-disk field survives as required, and `internal/prefs/appearance_api_guard_test.go` is an explicit source guard that bans the API identifiers while calling out that the on-disk field is deliberately not banned.
  - Task 6-3 also added `omitempty` to `session_list_mode`, which 5-1 asked to leave alone. This is a later task's change, and it is behaviourally inert for this task's contract: `SessionListMode.String()` always yields a canonical non-empty token, so the mode writer never omits it, and a theme-only write to an absent file omitting the key decodes back to `ModeFlat` — no data loss.
  - Later comment-trim passes (11-3, 12-7, `fee1927d`, `915e7fcb`) shortened the 5-1 doc comments. The two load-bearing trap warnings survive (`store.go:71-72` for `Appearance`, `:77` for `ThemeMigrated`), which is the part §8.8 makes mandatory.
- Leaf discipline holds: `internal/prefs` imports `bytes`, `encoding/json`, `errors`, `fmt`, `os`, `strconv` and `internal/fileutil` only. `leaf_guard_test.go`'s logic is unedited (the only post-5-1 change, `fee1927d`, is comment-only) and it is non-vacuous (asserts `fileutil` is still seen).

TESTS:
- Status: Adequate.
- Coverage (`internal/prefs/theme_keys_test.go`):
  - Criterion 1 → `TestLoadThemeKeys_DecodesAllThree` (:53) — constant alone, adaptive pair alone, all three alongside `session_list_mode`/`appearance`.
  - Criterion 2 → `TestLoadThemeKeys_TolerantDecode` (:91) table of missing/empty/corrupt/`{}`/no-keys/empty-string, plus `TestLoadThemeKeys_PropagatesReadError` (:124) using a directory at the prefs path for the EISDIR branch, asserting both the error and the zero `ThemeKeys` alongside it.
  - Criterion 3 → `TestLoadThemeKeys_NoValidationOrNormalisation` (:141) — uppercase, path traversal, surrounding space, embedded tab, each asserted against all three keys.
  - Criterion 4 → `TestSave_PreservesThemeKeys` (:176) asserts both the re-decoded `ThemeKeys` and the on-disk keys.
  - Criterion 5 → `TestSave_OmitsEmptyThemeKeysAndAppearance` (:240) asserts key *absence* structurally via a `map[string]any` decode, with a helper comment (:23-24) explaining why substring matching would be vacuous (`"theme"` matches `"theme_light"`). This is the right shape — a substring assertion here would silently pass forever.
  - Criterion 6 → `TestSave_PreservesRawAppearance` (:202) compares saves 2..10 against save 1's bytes (correctly anchored on the first re-encode, not the hand-written seed) and adds an unrecognised-value (`"sepia"`) subtest proving the value is never parsed.
  - Criterion 8 → `leaf_guard_test.go` unchanged in substance and still green.
  - Criterion 9's supersession is itself covered by `appearance_api_guard_test.go`, a repo-wide source scan.
  - Shared read policy is separately pinned white-box in `read_shared_test.go` (absent/present/unreadable, and the tolerant-vs-strict split on corrupt content).
- Notes: no over-testing — each test maps to a distinct criterion, tables are small and every case is a genuinely different input class. `TestPrefsFile_DeclaresNoMigrationMarker` (5-1's ninth test) was removed by task 6-3 when the field was legitimately declared; that is the correct disposal, not a lost test. The one unpinned load-path behaviour is a wrong-typed value (see notes below).

CODE QUALITY:
- Project conventions: Followed. `prefs` stays the leaf CLAUDE.md describes (no `internal/log`, no `internal/theme`, no `internal/xdg`); the store keeps its single-responsibility "dumb JSON leaf" shape; tests avoid `t.Parallel()` per the repo rule and use named table subtests.
- SOLID principles: Good. `LoadThemeKeys` is a pure read that adds no policy; validation/defaulting/tiebreak are pushed to the resolver, which is exactly §8.1's and §10.5's division. `ThemeKeys` is a comparable value struct, so tests can use `==` without a custom comparer.
- DRY: Good. `LoadThemeKeys` reuses `readFile`, so it inherits the tolerant policy verbatim rather than restating it — the property criterion 2 depends on.
- Complexity: Low. `LoadThemeKeys` is four lines with one branch.
- Modern idioms: Yes — `errors.Is(err, os.ErrNotExist)`, struct tags with `omitempty`, `for i := range 10` in the repeat-save test.
- Readability: Good. The surviving comments are the trap warnings and nothing else; intent is legible without them.
- Comment accuracy: Verified against the code. `store.go:71-72` (never delete the field / undeclared keys are dropped on re-encode) is true of the plain-struct decode in `readFile`/`readFileStrict` and the whole-record `write`. `store.go:81-82` (values verbatim; interpretation belongs to the resolver) is true — `LoadThemeKeys` performs no transformation. `store.go:189-190` ("same tolerant policy as Load") is true, both route through `readFile`. No spec-section, phase or task citations remain in production comments.
- Security: N/A — no execution, no path construction, no interpolation. Notably, refusing to normalise means `"../evil"` reaches the loader's charset check as an honest rejection rather than being laundered here, which is the safer direction.
- Performance: N/A — one small file read per call; `cmd/config.go` calls it once at construction.
- Issues: none.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/prefs/theme_keys_test.go:91 — add a wrong-typed-value case to `TestLoadThemeKeys_TolerantDecode`, e.g. `{name: "a wrong-typed theme value", content: `{"theme":42,"theme_dark":"nord"}`}`. The load path collapses the *whole* record to zero keys on any `UnmarshalTypeError` (`readFile`, store.go:135-137), while the write path deliberately preserves the surviving fields via the `typeErr.Field != ""` carve-out (store.go:156-159, pinned at store_write_path_test.go:233). Only the write half of that asymmetry is currently pinned, so a future "improvement" to `readFile` could make the two decodes agree and silently change what a hand-edited prefs.json renders as.
- [do-now] internal/prefs/theme_keys_test.go:141 — restore the deleted why above `TestLoadThemeKeys_NoValidationOrNormalisation`, which is now the only thing stopping a future reader adding a `strings.TrimSpace` to `LoadThemeKeys`. Suggested single line: `// No trimming or lowercasing: "  nord " must reach the charset check and fail as a bad name rather than silently becoming a different, valid slug.` (The rationale was removed by the comment-trim pass `fee1927d`; the assertion survived but its reason did not, and the failure it guards against is a silent slug substitution.)
