TASK: theming-system-6-2 — SaveTheme And SaveThemeSlot — Mutual Exclusion In One Atomic Write

ACCEPTANCE CRITERIA:
1. `SaveTheme("nord")` on a file holding both slots writes `theme` and removes both slot keys.
2. `SaveThemeSlot("nord", SlotDark)` on a file holding `theme: gruvbox` writes `theme_dark`, removes `theme`, leaves `theme_light` byte-unchanged.
3. `SaveThemeSlot(…, SlotLight)` then `SaveThemeSlot(…, SlotDark)` on one slug leaves both slots set (§9.5 `● both`).
4. Exactly one write per call, asserted by an injected write counter.
5. `session_list_mode`, `theme_migrated` and raw `appearance` round-trip untouched.
6. A cleared key is absent from the encoded JSON, not present as `""`.
7. Clearing an already-empty key is not an error and produces byte-identical output.
8. Committing the same value twice is idempotent.
9. Both savers inherit create-on-absent and abort-on-undecodable without re-implementing either.
10. Writer A's `SaveTheme` does not revert writer B's `Save(ModeByTag)`.
11. `SaveThemeSlot(slug, ThemeSlot(0))` and any out-of-range value write nothing and return an error.
12. `prefs` performs no slug validation (`../evil`, `Nord`, `  nord` persist verbatim).
13. `internal/prefs` still imports only the stdlib and `internal/fileutil`.

STATUS: complete

SPEC CONTEXT:
§8.2 ("Two states, not three") makes mutual exclusion a *write-time rule* rather than a type: committing a constant clears both slots, assigning a slot clears the constant, so "constant and pair both present" cannot arise from Portal's own writes; a hand-edit holding both resolves `theme`-wins on the read side, and stale slots are never pruned. §8.9 places the merge inside `prefs` behind field-specific savers (`SaveTheme`, `SaveThemeSlot`, `SaveMigrationMarker`, `SaveTranslation`) precisely so no caller gets a clobber-the-whole-record API, mandates read-modify-write against a strict write-path decode (create-on-absent, abort-on-undecodable), and requires all theme keys to land in one `fileutil.AtomicWrite`. §8.3's "an unset slot holds the shipped default" is what makes clearing-to-absent (via `omitempty`) the correct on-disk shape. §9.13 makes commits unconditional so a retry is free; §10.3's no-op condition belongs to `SaveTranslation` alone.

IMPLEMENTATION:
- Status: Implemented (comments later trimmed by the deliberate phases 11/12/17 comment-strip remediation — outcome unchanged)
- Location:
  - `internal/prefs/store.go:94-101` — `ThemeSlot` typed enum, `iota + 1` so the zero value is invalid; doc comment states the "forgotten argument cannot silently write light" rationale.
  - `internal/prefs/store.go:218-227` — `SaveTheme`: sets `Theme`, clears `ThemeLight`/`ThemeDark`, returns true, one `mutate` → one `write` → one `atomicWrite`.
  - `internal/prefs/store.go:229-247` — `SaveThemeSlot`: guards `slot != SlotLight && slot != SlotDark` **before** `mutate` (so an invalid slot neither reads nor writes the file) and returns `fmt.Errorf("prefs: invalid theme slot %d", slot)`; the mutator writes the addressed slot and clears `Theme`.
  - `internal/prefs/store.go:296-306` — `atomicWrite` package-level indirection over `fileutil.AtomicWrite` so the write count is assertable; production never reassigns it.
  - `internal/prefs/store.go:69-79` — `omitempty` on all three theme keys is what turns the empty-string clear into key-absent.
  - Consumer (out of this task's scope, task 6-7): `cmd/theme_persister.go:30-39,59-64`.
- Notes:
  - Every criterion holds. The one deviation from the task's **Do** text is an improvement, not drift: the task sketched the invalid slot as a `default:` arm inside the mutator; the implementation guards up front, which additionally guarantees the file is never even read on an invalid slot (criterion 11's "writes nothing" is strictly stronger, and the test asserts an absent file is not created).
  - Mutual exclusion, the commit, and the clear ride one `mutate` closure and therefore exactly one `AtomicWrite` — no reachable both-forms window (§8.9).
  - No slug knowledge added: no trim, no case-fold, no charset check, no default substitution, no `theme`-wins tiebreak. The "verbatim / resolution belongs elsewhere" statement survives on `SaveTheme`'s doc comment (`store.go:218`) and on `ThemeKeys` (`store.go:81-82`).
  - Neither saver consults §10.3's no-op condition; both are unconditional (§9.13). `SaveTranslation` (`store.go:266-294`, task 6-4) owns that condition separately, as specified.
  - The savers do not touch `Save`, `Load`, `LoadThemeKeys`, `readFile`, or the leaf guard; `leaf_guard_test.go` is unedited and `store.go` imports only stdlib + `internal/fileutil`.
  - The original commit (`b1f546eb`) carried long spec-citing doc comments on both savers; commits `e30939b2`, `c69101ca`, `fee1927d` and `915e7fcb` (later planned remediation) trimmed them to the current two-line form. Judged against the amended intent, that is not drift.

TESTS:
- Status: Adequate (with two mild over-assertion spots, both non-blocking)
- Coverage: `internal/prefs/theme_savers_test.go` implements all twelve named tests from the plan, and every one maps onto a criterion:
  - `TestSaveTheme_ClearsBothSlots:25` — four starting shapes (pair, hand-edited constant+pair, single slot, constant alone) → criterion 1.
  - `TestSaveThemeSlot_ClearsConstant:64` — `theme` absent, `theme_dark` written, `theme_light` preserved → criterion 2.
  - `TestSaveThemeSlot_OtherSlotUnaffected:78` — both directions → criterion 2/3 support.
  - `TestSaveThemeSlot_LightThenDarkYieldsBoth:118` — the §9.5 `● both` state, constant cleared → criterion 3.
  - `TestSaveTheme_SingleAtomicWrite:151` + `recordWrites:135` — counts commits at the `atomicWrite` seam and asserts `len == 1` per saver, then decodes the committed bytes to prove the clear rode the same write → criterion 4. This is the right seam: a second write leaves no filesystem trace, so a post-hoc check could only prove "at least one".
  - `TestThemeSavers_PreserveUnrelatedFields:176` (mode + unrecognised `appearance` "sepia") plus `TestMigrationMarker_NotTouchedByThemeSavers` (`migration_marker_test.go:239-269`, a seeded `theme_migrated:true` survives all three savers, and no saver sets it) → criterion 5 fully.
  - `TestThemeSavers_ClearedKeysAreAbsent:213` — structural key-absence + the already-empty byte-identity case → criteria 6 and 7.
  - `TestThemeSavers_RepeatedCommitIsByteIdentical:260` — three commits, bytes pinned → criterion 8.
  - `TestThemeSavers_InheritWritePathRules:285` — create-on-absent through a missing parent dir (plus no leftover `.atomic-` temp files) and abort-on-undecodable across the shared `undecodablePrefsCases()` with bytes unchanged → criterion 9.
  - `TestThemeSavers_RMWDoesNotLoseAnotherWritersField:323` → criterion 10.
  - `TestSaveThemeSlot_InvalidSlotWritesNothing:343` — zero, `3`, `-1`; exact error string; present file byte-identical; absent file not created → criterion 11.
  - `TestThemeSavers_NoSlugKnowledge:388` — `../evil`, `Nord`, `  nord`, embedded tab persist verbatim for all three savers → criterion 12.
  - `leaf_guard_test.go:17` — criterion 13 (and it is non-vacuous: it fails if `internal/fileutil` ever disappears from the dep set).
  - The `themeSaverCases()` table (`:421`) drives all three savers through the shared bodies, so `SaveTheme`/light/dark cannot drift apart.
- Notes:
  - Would fail if the feature broke: dropping either clear fails `ClearsBothSlots` / `ClearsConstant`; splitting into two writes fails `SingleAtomicWrite`; removing `omitempty` fails `ClearedKeysAreAbsent`; removing the slot guard fails `InvalidSlotWritesNothing`; caching a snapshot in `Store` fails `RMWDoesNotLoseAnotherWritersField`.
  - `TestThemeSavers_RMWDoesNotLoseAnotherWritersField` is structurally hard to fail today (`Store` holds only a path), so it acts as a regression guard against a future in-memory snapshot rather than as a live demonstration. Keeping it is correct — the criterion names it.
  - Two mild redundancies, both listed as non-blocking notes below (the raw-bytes needle loop duplicating `assertKeysAbsent`; the 3 × 9 undecodable matrix where inheritance needs one representative case per saver).
  - No mocking beyond the single `atomicWrite` seam; no test asserts an implementation detail that isn't a stated contract.

CODE QUALITY:
- Project conventions: Followed. `prefs` stays a leaf (no `internal/log`), the merge is single-sited inside the store as §8.9 requires, `fileutil.AtomicWrite` is the only write path, error string uses the codebase's `<pkg>: …` prefix form (matches `internal/spawn/ackid.go`), no `t.Parallel()` in the tests (mandatory here — `recordWrites` swaps a package-level var).
- SOLID principles: Good. Two narrow field-specific methods over one shared `mutate`; no whole-record type is exported, so no caller can clobber the file — exactly the API shape §8.9 rejected the alternative for.
- Complexity: Low. `SaveTheme` is four statements; `SaveThemeSlot` is a guard plus a two-arm switch.
- Modern idioms: Yes. Typed enum with an invalid zero value; `iota + 1`; tests use `for i := range 3` and `bytes.Clone` (go 1.26 module).
- Readability: Good. Intent is clear at both call sites; the `atomicWrite` indirection is explained where it is declared.
- Comment accuracy: Every surviving comment in the changed code holds. `store.go:94-95` (zero value invalid), `:218-219` (verbatim + clears in the same atomic write), `:229-231` (clears the constant, other slot untouched, invalid slot writes nothing) and `:296` (indirection for counting, never reassigned in production) all match the code. No spec-section or task-id citations remain in production comments.
- Security: N/A beyond the deliberate no-validation contract — `prefs` persisting `../evil` verbatim is specified (§8.2/§8.9 put charset rejection on the read side, `internal/theme`'s `bad name` rung), and the value is a JSON string, never a path this package opens.
- Performance: One read + one atomic write per save; nothing repeated.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/prefs/theme_savers_test.go:227-232 — drop the raw-bytes needle loop (`bytes.Contains(raw, []byte("\""+key+"\":"))`); `assertKeysAbsent(t, decodeWritten(t, path), c.clearedKeys...)` on line 223 already proves the key is absent from the encoded object, and `encoding/json` cannot emit a duplicate key for the raw scan to catch.
- [quickfix] internal/prefs/theme_savers_test.go:301-320 — in the `abort-on-undecodable` subtest, iterate one representative case (the trailing-comma syntax error) per saver instead of the full `undecodablePrefsCases()` set: the nine-case matrix is already exhaustively asserted against the shared `mutate` seam in `TestSave_AbortsOnUndecodable` (`store_write_path_test.go:184`), and this test's stated job is only to prove the savers *inherit* it, so 3 subtests carry the same signal as the current 27.
- [do-now] internal/prefs/store.go:218-219 — extend `SaveTheme`'s doc comment to name how the clear reaches disk, e.g. `// SaveTheme persists slug verbatim as the constant theme, clearing the adaptive\n// slots in the same atomic write. A cleared key is written empty, which\n// omitempty omits, so an unset slot holds the shipped default.` The property is non-obvious from the saver alone (it depends on the struct tags 60 lines above) and was stated in the original commit before the comment-strip remediation removed the whole paragraph.
