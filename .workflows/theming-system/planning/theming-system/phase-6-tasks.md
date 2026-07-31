# Phase 6: Prefs write path and the `appearance` upgrade — 7 tasks

## theming-system-6-1

### Task 6.1: Strict write-path decode with create-on-absent and abort-on-undecodable

**Problem**: Every `prefs.json` write is a whole-file re-encode performed over a **tolerant** decode. `Store.readFile` (`internal/prefs/store.go`) collapses corrupt JSON to a zero-valued `prefsFile` and returns **no error**, so `Save(mode)` merges the new mode into an empty record and commits it. Before this feature that lost one field nobody had set; after Phase 5 the file carries five independently-mutated values (`session_list_mode`, the retained raw `appearance`, `theme`, `theme_light`, `theme_dark`) and Phase 6 adds a sixth, with three writing surfaces. A single stray comma in a hand-edited file therefore turns the next `s` keypress into a silent, permanent erasure of the user's theme *and* their downgrade pin. The abort has no trigger at all unless the write path decodes **differently** from the load path: a tolerant decode returns no error, so the writer has nothing to abort on.

**Solution**: Add a strict, **syntax-judging** re-read plus one private read-modify-write mutator inside `internal/prefs`, and route the existing `Save` through it. An absent file proceeds and creates; malformed JSON or a non-`ErrNotExist` read failure aborts with the on-disk bytes untouched; unrecognised or wrong-typed *values* inside syntactically valid JSON are absorbed exactly as today.

**Outcome**: `Save` on a file containing `{"session_list_mode":"flat",}` returns an error and leaves the file byte-identical; `Save` on an absent file creates it; `Save` on `{"session_list_mode":5,"theme":"nord"}` still writes and preserves `theme`. Every later saver in this phase inherits all three behaviours without re-implementing them.

**Do**:
- Add `func (s *Store) readFileStrict() (prefsFile, bool, error)` to `internal/prefs/store.go`, beside (never replacing) the existing tolerant `readFile`:
  - `os.ReadFile`; `errors.Is(err, os.ErrNotExist)` → return `(prefsFile{}, false, nil)` — **the create-on-absent path**. Any other read error → return it (abort).
  - `json.Unmarshal` error → abort by returning it, **except** an error that `errors.As`-matches `*json.UnmarshalTypeError` **whose `Field` is non-empty**: that is a wrong-typed value on a declared field, which the decode absorbs (the field keeps its zero value, every other field is still populated by `encoding/json`), so return `(f, true, nil)`.
  - Pin the `Field != ""` discriminator in a comment: a top-level type mismatch (`[1,2]`, `"x"`, `3` as the whole document) also yields an `UnmarshalTypeError` but with an **empty** `Field` and a zero-valued struct — absorbing that would merge into an empty record and commit it, which is the exact destruction this task exists to prevent.
- Add the single private mutator every saver in this phase routes through:
  ```go
  // mutate performs the write-path read-modify-write: strict re-read immediately
  // before the write, apply fn to the decoded record, write the merged result.
  // fn receives whether the file existed and returns false to skip the write
  // entirely (no bytes touched, no error).
  func (s *Store) mutate(fn func(f *prefsFile, existed bool) bool) error
  ```
  On a `readFileStrict` error, return it **verbatim** and write nothing.
- Convert `Save(mode SessionListMode)` to `mutate` so the `s`-key persister comes under the same rule. Record in its doc comment that this is a deliberate behaviour change: a malformed `prefs.json` now aborts the mode write instead of silently overwriting it, and the caller (`internal/tui/model.go`'s `_ = m.modePersister.Save(...)`) already swallows the error, so the failure is non-fatal *and* non-destructive.
- Leave the tolerant `readFile`, `Load` and `LoadThemeKeys` **exactly as they are**. Document the split at both functions: the *load* path stays tolerant per §8.1 (missing / empty / unrecognised falls to the shipped default per field), the *write-path re-read* judges syntax only, and neither may be used for the other's job.
- Keep every write on `fileutil.AtomicWrite` through the existing `write` helper, so all six keys land in one atomic write and partial failure stays impossible.
- Keep `internal/prefs` a leaf: no `internal/log`, no error classification, no reporting. An abort is reported **by returning**; the caller decides non-fatality. `internal/prefs/leaf_guard_test.go` must stay green unedited.

**Acceptance Criteria**:
- [ ] An **absent** `prefs.json` is created by `Save`, including when its parent directory does not exist (`AtomicWrite` MkdirAlls).
- [ ] A **present but malformed** file (`{`, `{"a":1,}`, `not json`) aborts: an error is returned, no temp file survives, and the file's bytes are **byte-identical** before and after.
- [ ] A **zero-byte** file aborts (present, not valid JSON) — the rule is syntax-driven and this case is pinned deliberately rather than falling out.
- [ ] A non-`ErrNotExist` read error (a `0000`-mode file) aborts like malformed and returns the OS error.
- [ ] A wrong **type** on a declared field (`{"session_list_mode":5,"theme":"nord"}`) does **not** abort: the write proceeds and `theme` is preserved.
- [ ] A top-level non-object document (`[1,2]`, `"x"`) **aborts** — it is not a field type error, and absorbing it would merge into an empty record.
- [ ] Unrecognised *values* in valid JSON (`{"session_list_mode":"sideways"}`) do not abort.
- [ ] `mutate`'s `fn` returning `false` writes nothing, returns nil, and leaves the file byte-identical (including when the file is absent).
- [ ] The tolerant `readFile` / `Load` / `LoadThemeKeys` behaviour is unchanged — Phase 5 task 5-1's tests pass unedited.
- [ ] Writes still go through `fileutil.AtomicWrite`; the strict read is reachable only from the savers.
- [ ] `internal/prefs` still imports only the standard library and `internal/fileutil`.

**Tests**:
- `"it creates an absent prefs file on write"` — `TestSave_CreatesAbsentFile` (plus an absent parent directory)
- `"it aborts rather than overwriting a malformed file"` — `TestSave_AbortsOnMalformedJSON` (table: truncated object, trailing comma, junk; byte-compare before/after)
- `"it aborts on a zero-byte file"` — `TestSave_AbortsOnEmptyFile`
- `"it aborts on an unreadable file"` — `TestSave_AbortsOnReadError`
- `"it absorbs a wrong-typed field and still writes"` — `TestSave_WrongTypedFieldDoesNotAbort`
- `"it aborts on a top-level type mismatch"` — `TestSave_TopLevelTypeMismatchAborts`
- `"it absorbs unrecognised values"` — `TestSave_UnrecognisedValueIsNotUnusable`
- `"it skips the write when the mutator declines"` — `TestMutate_DecliningMutatorWritesNothing`
- `"it leaves the tolerant load path untouched"` — Phase 5's `TestLoadThemeKeys_TolerantDecode`, unedited and green
- `"it stays a leaf"` — existing `internal/prefs/leaf_guard_test.go`, unedited

**Edge Cases**:
- An **absent** `prefs.json` is created by the write — a brand-new user's first `Enter` is the most common write in the product, and an abort here would be permanent, since nothing else creates the file (the `s`-key persister is under this same rule and §8.1 bars the migration from creating it).
- A **present but malformed** file aborts and never becomes an overwrite, leaving the bytes byte-identical.
- The two decodes must **differ** or the abort has no trigger at all — a tolerant decode turns a stray comma into a zero-value struct and returns no error, so the writer merges into it and one `s` keypress erases `session_list_mode`, every theme key, `theme_migrated` and the retained raw `appearance`.
- The load path stays tolerant **exactly as today** and is not touched.
- Unrecognised *values* in syntactically valid JSON are **not** "unusable" — treating them as fatal would make hand-editing prefs a way to lock yourself out, so the strict decode judges syntax only.
- A wrong *type* on a declared field is a value problem, not a syntax one, and must not abort — task 6-3's `theme_migrated` is the live case. **Consequence, accepted and recorded**: the offending field decodes to its zero value and is re-encoded as such, so a wrong-typed value is normalised away on the next write. That is §8.1's tolerant absorption, not a loss the abort rule is meant to catch.
- A **top-level** type mismatch also produces an `UnmarshalTypeError`, but with an empty `Field` and a zero struct — it must abort, which is why the discriminator is `Field != ""` and not the error type alone.
- A zero-byte file is present-but-not-valid-JSON — the rule is syntax-driven so it aborts, and the choice is pinned deliberately.
- A non-`ErrNotExist` read error (permission denied) aborts like malformed.
- The re-read happens **immediately before** the write, not at load — a stale in-memory snapshot is what reverts another instance's commit, and `AtomicWrite` does not help because this is a lost update, not a partial write.
- Writes still go through `fileutil.AtomicWrite`, so every key lands in one atomic write.
- The strict read is used by the field-specific savers and nothing else.
- The error is returned verbatim so the caller decides non-fatality, and `prefs` stays a leaf with no `internal/log`, so an abort is reported by returning and never logged here.

**Context**:
> §8.9: "**But a stale whole-file write can silently revert a theme.** Before this feature `prefs.json` had one field with a production writer. It now holds five independently-mutated fields written from three surfaces: instance A, constructed ten minutes ago, presses `s` and writes *its* in-memory prefs, silently reverting the theme instance B just committed. `AtomicWrite` does not help — this is a lost update, not a partial write. **Every writer must read-modify-write**: re-read `prefs.json` immediately before writing, mutate only its own field(s), and write the merged result."
> §8.9: "**An absent file and an unusable one are different conditions, and only the second aborts.** … **`prefs.json` present but unusable** — malformed JSON or an I/O failure — **aborts the write; it never becomes an overwrite.** This needs **two decodes, and they must differ**… Without that split the abort has no trigger at all: a tolerant decode turns a stray comma into a zero-value struct and returns no error, so the writer merges into it. Merging into an empty struct and committing it would erase `session_list_mode`, `theme_migrated`, every untouched theme key and the retained raw `appearance` in a single `s` keypress… **Unrecognised *values* in a syntactically valid file are not 'unusable'**… The strict decode judges syntax only. The field-specific save methods use the strict read internally; nothing else does."
> §8.9: "**The merge itself lives inside `prefs`, behind field-specific save methods**… matching `SaveSessionListMode`, which already performs its own internal read-modify-write. So do the two rules below: **create-on-absent and abort-on-undecodable are persistence semantics, not policy**, and they belong beside the decode they depend on." The rejected alternative was "exporting a whole-record type with `Load`/`Save` so `cmd` performs the merge literally — it would give any caller an API that can clobber the file wholesale."
> §13.6's new prefs+migration test: "This is the one part of the feature whose failure mode is silent, permanent destruction of a user's config, and none of it is observable at the moment it goes wrong… **§13.6's prefs test covers file creation as well as merge and round-trip** — a suite built only around merging would not catch an abort-on-absent implementation."
> Phase boundary: this task ships the decode split and the mutator only. `SaveTheme`/`SaveThemeSlot` are task 6-2, the marker is 6-3, `SaveTranslation` is 6-4 — each inherits create-on-absent and abort-on-undecodable rather than re-implementing them.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §8.9, §8.1, §13.6

## theming-system-6-2

### Task 6.2: `SaveTheme` and `SaveThemeSlot` — mutual exclusion in one atomic write

**Problem**: §8.2's "two states, not three" is enforced **on write**: committing a constant clears both slots, and assigning a slot clears the constant. Nothing writes theme keys today, so the rule exists only as prose — and the two obvious wrong shapes both break decided behaviour. Issuing two writes (set one key, clear another) leaves a reachable window where a file holds both forms, which §8.2 says cannot arise from Portal's own writes. Exporting a whole-record save so `cmd` performs the clearing hands every caller an API that can clobber the file wholesale — the opposite of what keeping the merge single-sited inside the leaf protects, and it would make §8.8's raw `appearance` round-trip a rule every future caller has to remember rather than a property of the store.

**Solution**: Two field-specific savers on `*prefs.Store`, each performing its own read-modify-write through task 6-1's strict re-read and committing its key **plus** the mutual-exclusion clear in a single `AtomicWrite`, with the slot expressed as a typed value so no caller can mint a third slot.

**Outcome**: `SaveTheme("nord")` writes `theme: nord` and removes `theme_light` / `theme_dark`; `SaveThemeSlot("nord", SlotDark)` writes `theme_dark: nord`, removes `theme`, and leaves `theme_light` exactly as it was; both leave `session_list_mode`, `theme_migrated` and the raw `appearance` untouched; and a concurrent `s` keypress from another instance cannot revert either.

**Do**:
- Declare the slot type in `internal/prefs`, zero value deliberately invalid:
  ```go
  // ThemeSlot names one half of the adaptive pair. The zero value is invalid so a
  // forgotten argument cannot silently write the light slot.
  type ThemeSlot int
  const (
      SlotLight ThemeSlot = iota + 1
      SlotDark
  )
  ```
- Add `func (s *Store) SaveTheme(slug string) error` over task 6-1's `mutate`: set `f.Theme = slug`, set `f.ThemeLight = ""` and `f.ThemeDark = ""`, return true. One write.
- Add `func (s *Store) SaveThemeSlot(slug string, slot ThemeSlot) error` over the same mutator: `switch slot { case SlotLight: f.ThemeLight = slug; case SlotDark: f.ThemeDark = slug; default: <write nothing> }`, plus `f.Theme = ""`. An out-of-range slot writes **nothing** and returns an error naming the invalid slot — the structural half of "no caller can mint a third slot" (the typed constant is the other half).
- Comment at both savers that clearing is **writing the empty string**, which `omitempty` (Phase 5 task 5-1) renders as **key-absent** — matching §8.3's "an unset slot holds the shipped default" and keeping a hand-edited file clean.
- Add **no slug knowledge** to `prefs`: no `ValidSlug` check, no trimming, no lowercasing, no default substitution, no `theme`-wins tiebreak. Those are read-side resolution rules owned by `internal/theme` (Phase 5 tasks 5-2/5-3). State it in a comment so a later reader does not "helpfully" add validation that would diverge from the resolver.
- Neither saver consults §10.3's no-op condition — that belongs to `SaveTranslation` (task 6-4) alone. Both are **unconditional** writes, which is what makes §9.13's "a commit is always re-attemptable" free.
- Do not touch `Save`, `Load`, `LoadThemeKeys`, the tolerant `readFile` or the leaf guard.

**Acceptance Criteria**:
- [ ] `SaveTheme("nord")` on a file holding both slots writes `theme` and **removes** both slot keys from the encoded JSON.
- [ ] `SaveThemeSlot("nord", SlotDark)` on a file holding `theme: gruvbox` writes `theme_dark` and **removes** `theme`, leaving `theme_light` byte-unchanged.
- [ ] `SaveThemeSlot(…, SlotLight)` then `SaveThemeSlot(…, SlotDark)` on the same slug leaves both slots set to it — the §9.5 `● both` state.
- [ ] Exactly **one** write happens per call, asserted by counting file mtimes/writes or by an injected write counter — never two.
- [ ] `session_list_mode`, `theme_migrated` and the raw `appearance` round-trip untouched through both savers.
- [ ] A cleared key is **absent** from the encoded JSON, not present as `""`.
- [ ] Clearing an already-empty key is not an error and produces byte-identical output.
- [ ] Committing the same value twice is idempotent (byte-identical file after the second call).
- [ ] Both savers inherit create-on-absent (absent file → created) and abort-on-undecodable (malformed file → error, bytes unchanged) without re-implementing either.
- [ ] Writer A's `SaveTheme` does not revert writer B's `Save(ModeByTag)` written in between — the RMW re-read is what proves it.
- [ ] `SaveThemeSlot(slug, ThemeSlot(0))` and any out-of-range value write nothing and return an error.
- [ ] `prefs` performs no slug validation: `SaveTheme("../evil")`, `SaveTheme("Nord")` and `SaveTheme("  nord")` all persist verbatim.
- [ ] `internal/prefs` still imports only the standard library and `internal/fileutil`.

**Tests**:
- `"it clears both slots when committing a constant"` — `TestSaveTheme_ClearsBothSlots`
- `"it clears the constant when assigning a slot"` — `TestSaveThemeSlot_ClearsConstant`
- `"it leaves the other slot untouched"` — `TestSaveThemeSlot_OtherSlotUnaffected`
- `"it produces the both-slots state from two saves"` — `TestSaveThemeSlot_LightThenDarkYieldsBoth`
- `"it writes once, atomically"` — `TestSaveTheme_SingleAtomicWrite`
- `"it round-trips every field it does not own"` — `TestThemeSavers_PreserveUnrelatedFields` (mode, marker, raw appearance)
- `"it omits a cleared key rather than writing an empty string"` — `TestThemeSavers_ClearedKeysAreAbsent`
- `"it is idempotent"` — `TestThemeSavers_RepeatedCommitIsByteIdentical`
- `"it inherits create-on-absent and abort-on-undecodable"` — `TestThemeSavers_InheritWritePathRules`
- `"it does not revert another writer's field"` — `TestThemeSavers_RMWDoesNotLoseAnotherWritersField`
- `"it rejects an invalid slot"` — `TestSaveThemeSlot_InvalidSlotWritesNothing`
- `"it validates no slug"` — `TestThemeSavers_NoSlugKnowledge`

**Edge Cases**:
- Committing a **constant clears both slots** and assigning a **slot clears the constant**, so "both a constant and a pair are present" cannot arise from Portal's own writes and §8.2's two-state model holds as a rule rather than a type.
- Both land in **one** atomic write, never two, so no partial state is reachable.
- A slot save leaves the **other** slot untouched, which is what makes `d` then `l` produce §9.5's `● both` row.
- Clearing is writing the empty string, which `omitempty` renders as **key-absent**, keeping a hand-edited file clean and matching "an unset slot holds the shipped default".
- Clearing an already-empty key is not an error and produces identical bytes.
- Committing the same value twice is idempotent, which is what makes §9.13's "a commit is always re-attemptable" free.
- `theme_migrated` **never participates** in mutual exclusion and round-trips untouched, and so do raw `appearance` and `session_list_mode`.
- Each saver performs its **own** RMW through task 6-1's strict re-read, so writer A does not revert writer B's field — the lost-update rule this phase exists for.
- Create-on-absent and abort-on-undecodable are inherited, not re-implemented.
- `prefs` gains **no slug knowledge** — no charset check, no default substitution, no `theme`-wins tiebreak (those are read-side resolution rules from Phase 5).
- The slot is a **typed** value (light/dark), not a caller-supplied key name, so no caller can mint a third slot; the zero value is invalid so a forgotten argument cannot silently write light.
- Neither saver consults §10.3's no-op condition, which belongs to `SaveTranslation` alone.
- `prefs` stays a leaf and the existing leaf guard stays green unedited.

**Context**:
> §8.2: "**Mutual exclusion is enforced on write.** Committing a constant clears both slots; assigning a slot clears the constant. Whichever was set last wins, so 'both a constant and a pair are present' cannot arise from Portal's own writes… **If a hand-edit leaves both present, `theme` wins** — a documented deterministic rule. The 'only two states' model stays a *rule* rather than being encoded in a type… The stale slots are left untouched on disk; nothing prunes them."
> §8.9: "**The merge itself lives inside `prefs`, behind field-specific save methods** — `SaveTheme`, `SaveThemeSlot`, `SaveMigrationMarker`, and **`SaveTranslation`**… The alternative — exporting a whole-record type with `Load`/`Save` so `cmd` performs the merge literally — was rejected: it would give any caller an API that can clobber the file wholesale, which is the opposite of what '`prefs` stays dumb' is protecting. Keeping the merge single-sited inside the leaf is what makes §8.8's raw `appearance` round-trip a property of the store rather than a rule every caller has to remember."
> §8.9: "`prefs.json` continues to go through `fileutil.AtomicWrite`, so all three theme keys land in one atomic write and partial failure is impossible."
> §9.5: "**When both slots name the same slug, that one row carries `● both`.** This is reachable in two keypresses (`d` then `l` on one row) and is a likely path."
> §9.13: "**A commit is always re-attemptable.** The commit keys are unconditional writes, so pressing `d`/`l`/`Enter` again simply retries — no special retry affordance, and no state to clear first."
> Phase boundary: **nothing calls these savers in this phase** except their tests and (from task 6-7) the `cmd`-owned persister's direct-call verification. The panel that presses `Enter`/`d`/`l` is Phase 9.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §8.2, §8.9, §8.3, §9.5, §9.13

## theming-system-6-3

### Task 6.3: The `theme_migrated` marker field and `SaveMigrationMarker`

**Problem**: The one-shot `appearance` translation needs a trigger, and §10.3 rules out the obvious one: gating on the *absence* of theme keys is re-armable, so a user who follows §9.9's documented escape hatch — hand-delete the theme keys to return to the shipped adaptive pair — would be silently re-translated and re-pinned on the next launch, Portal reinstating exactly what they just undid. The trigger must therefore be an explicit `theme_migrated` marker. Phase 5 deliberately left the field **undeclared** (nothing wrote it), which means that from the moment anything writes one, an undeclared marker would be dropped on the next re-encode — the same silent-erasure mechanism §8.8 documents for `appearance`. And the marker is the one field a user is most likely to hand-edit wrongly (`"theme_migrated": "yes"`), so its decode must absorb a wrong type without making the whole record undecodable — which under task 6-1's rule would abort **every** subsequent write and lock the user out of their own prefs.

**Solution**: Declare the marker on `prefsFile` behind a total, never-erroring decode (anything that is not literal `true` is `false`), add the tolerant read accessor the translation consumes, and add `SaveMigrationMarker` — a marker-only RMW that inherits the abort half of task 6-1's rule but explicitly **not** the create half.

**Outcome**: `theme_migrated` round-trips through every write; `"theme_migrated": "yes"`, `1`, `null` and `{}` all decode to `false` without erroring anything; `SaveMigrationMarker()` on an existing file writes `true` and preserves all five other keys; on an absent file it writes nothing and creates nothing.

**Do**:
- Add the field to `prefsFile` with a total decode of its own, so neither the tolerant load path nor task 6-1's strict re-read has to special-case it:
  ```go
  // migrationMarker decodes §8.1's marker: anything that is not literal true —
  // absent, empty, corrupt, wrong-typed, unrecognised — is false. It never
  // returns an error, so a hand-edited value can neither abort a write (task 6-1)
  // nor zero the tolerant load (readFile).
  type migrationMarker bool
  func (m *migrationMarker) UnmarshalJSON(b []byte) error // true iff the trimmed bytes are exactly `true`
  func (m migrationMarker) MarshalJSON() ([]byte, error)
  ```
  and `ThemeMigrated migrationMarker` tagged `json:"theme_migrated,omitempty"` on `prefsFile`, so a `false` marker is **absent** on disk and only a `true` marker appears.
- Add the tolerant read accessor the translation consumes — one read covering both of its inputs:
  ```go
  // MigrationState is the §10.3 one-shot gate's input: the retained raw appearance
  // value and whether the translation has already been recorded.
  type MigrationState struct {
      Appearance string
      Migrated   bool
  }
  func (s *Store) LoadMigrationState() (MigrationState, error)
  ```
  Read through the existing tolerant `readFile` so it inherits today's policy verbatim (missing / empty / corrupt file → zero value, nil error; only a non-`ErrNotExist` read error propagates). Document that `Appearance` is returned **verbatim and unparsed** — `prefs` has no `Appearance` enum any more (Phase 5 task 5-7 deleted it) and must not grow one; the `dark`/`light`/`auto` mapping belongs to `cmd/config.go` (task 6-5).
- Add `func (s *Store) SaveMigrationMarker() error` over task 6-1's `mutate`: when `existed` is false, return **false** from the mutator (write nothing, return nil) — a fresh install has no `appearance` to translate, and creating the file purely to record a marker would add a side effect to a path this feature otherwise leaves free. Otherwise set the marker true and write.
- Comment that absence is judged at **that same RMW re-read**, never against a stale `os.Stat` taken at load — the file can appear between load and write (another instance's first commit).
- Comment that the marker **never participates in mutual exclusion**: `SaveTheme` / `SaveThemeSlot` do not touch it, and clearing theme keys by hand does not clear it. That is precisely the property §10.3 exists to guarantee against a re-armable absence gate.
- Record in a comment why the field is declared **before** its first writer exists (task 6-4): an undeclared key is dropped on re-encode, so declaring it here is what stops an on-disk marker written by a newer instance being erased by an older code path in the same release.
- `prefs` stays a leaf — no logging, no interpretation of `appearance`, no knowledge of what a translation is.

**Acceptance Criteria**:
- [ ] `theme_migrated: true` on disk survives `Save(mode)`, `SaveTheme` and `SaveThemeSlot` unchanged.
- [ ] `"theme_migrated": "yes"`, `1`, `0`, `null`, `[]` and `{}` each decode to `false`, produce **no** error from either decode path, and do **not** abort a subsequent write.
- [ ] A file with a wrong-typed marker still yields its `session_list_mode` and theme keys from the tolerant load path (the total decode is what stops the record zeroing).
- [ ] `false` is **absent** from the encoded JSON; only `true` appears.
- [ ] `LoadMigrationState()` returns the raw `appearance` verbatim (`"dark"`, `"Dark"`, `"  dark"`, `"sepia"` all unchanged) and the marker; every degenerate file yields a zero value with a nil error; only a non-`ErrNotExist` read error propagates.
- [ ] `SaveMigrationMarker()` on an existing file writes `true` and preserves `session_list_mode`, `appearance` and all three theme keys.
- [ ] `SaveMigrationMarker()` on an **absent** file writes nothing, creates nothing, and returns nil.
- [ ] `SaveMigrationMarker()` on a malformed file aborts (error returned, bytes byte-identical).
- [ ] A file created between the load-time snapshot and the save is written to — absence is judged at the re-read, not from a stale stat.
- [ ] `SaveTheme` / `SaveThemeSlot` never set or clear the marker.
- [ ] `internal/prefs` still imports only the standard library and `internal/fileutil`.

**Tests**:
- `"it round-trips a true marker through every writer"` — `TestMigrationMarker_RoundTrips`
- `"it decodes anything but literal true as false"` — `TestMigrationMarker_TolerantDecode` (table: `"yes"`, `1`, `0`, `null`, `[]`, `{}`, absent, `"true"` as a string)
- `"it never makes the record undecodable"` — `TestMigrationMarker_WrongTypeDoesNotZeroTheRecord` (asserts mode + theme keys survive both decode paths and a write)
- `"it omits a false marker"` — `TestMigrationMarker_FalseIsAbsentOnDisk`
- `"it reads the raw appearance verbatim"` — `TestLoadMigrationState_ReturnsRawAppearanceUnparsed`
- `"it tolerates every degenerate file"` — `TestLoadMigrationState_TolerantDecode`
- `"it writes only the marker"` — `TestSaveMigrationMarker_PreservesEveryOtherField`
- `"it never creates an absent file"` — `TestSaveMigrationMarker_DoesNotCreateAbsentFile`
- `"it aborts on a malformed file"` — `TestSaveMigrationMarker_AbortsOnUndecodable`
- `"it judges absence at the re-read"` — `TestSaveMigrationMarker_AbsenceJudgedAtReRead` (file created after the load snapshot)
- `"it is untouched by mutual exclusion"` — `TestMigrationMarker_NotTouchedByThemeSavers`

**Edge Cases**:
- The type is **boolean**, not a version string or timestamp — the translation is a single event with no successor, so there is nothing to version.
- **Tolerant decode**: anything that is not literal `true` — absent, empty, corrupt, unrecognised — decodes to `false`, keeping decode as dumb as the string keys, with the failure direction "run the translation again", which is idempotent, so a corrupt marker costs one redundant write rather than a wrong theme.
- A **wrong-typed** value (`"theme_migrated": "yes"`) must not make the whole struct fail to unmarshal, because under task 6-1's rule that would abort every subsequent write and lock the user out — the field's own decode absorbs it, so the strict decode still passes the file as syntactically valid and the tolerant load still yields every other field.
- The marker **never participates in mutual exclusion**, and clearing theme keys by hand does not clear it — precisely the property §10.3 exists to guarantee against a re-armable absence gate.
- `SaveMigrationMarker` writes **only** the marker, so all three theme keys, `session_list_mode` and the raw `appearance` round-trip through it.
- It does **not** create the file when absent (a fresh install has no `appearance` to translate, and §5.5/§12.3 keep this feature from adding a side effect to a path it otherwise leaves free) — and absence is judged at the **same RMW re-read** as everything else, never against a stale stat.
- A `false` marker is absent on disk rather than written, and only a `true` marker appears.
- Phase 5 deliberately left the field undeclared, so declaring it here — before its first writer exists — is what stops an on-disk marker being dropped on re-encode.
- The `appearance` value is returned **unparsed**; `prefs` has no `Appearance` enum any more and must not regrow one.
- `prefs` stays a leaf.

**Context**:
> §8.1: "**`theme_migrated`** is not a theme setting — it is the one-shot gate for the `appearance` translation. **Type: boolean.** Not a version string or timestamp… **Tolerant decode:** anything that is not literal `true` — absent, empty, corrupt, unrecognised — decodes to `false`… the failure direction is 'run the translation again', and the translation is idempotent by §10.5, so a corrupt marker costs one redundant write rather than a wrong theme. **Written on the first post-upgrade prefs load whenever `prefs.json` already exists**, including when there is nothing to translate… **Not written when `prefs.json` does not exist.** … **Never participates in mutual exclusion.** It is orthogonal to which theme keys are set, and clearing theme keys by hand does not clear it — that is precisely the property §10.3 exists to guarantee."
> §10.3: "**The translation is gated on an explicit `theme_migrated` marker in `prefs.json`, not on the absence of theme keys.** Gating on absence would be re-armable, and it composes badly with the 'no unset' acceptance (§9.9), whose documented escape hatch is to hand-edit `prefs.json`: an upgraded user who deletes their theme keys to return to the shipped adaptive pair would get **silently re-translated and re-pinned** on the next launch."
> §8.8: "`prefs.json` decodes into a plain Go struct, so **any key not declared as a field is dropped on re-encode** — and §8.9 makes every writer re-encode the whole file."
> §10.4: `appearance` "is retained, not dropped… a **frozen legacy value**" — which is why `LoadMigrationState` returns it unparsed and nothing in the new binary interprets it except the translation.
> Phase boundary: Phase 5 task 5-1 declared the three theme keys and left `theme_migrated` out deliberately; this task declares it **with** its first writer in the same release, and task 6-4 combines it with the theme key into the single write §10.5 requires.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §8.1, §10.3, §10.4, §8.8, §8.9

## theming-system-6-4

### Task 6.4: `SaveTranslation` — theme key and marker in one write, with the no-op re-evaluated at the re-read

**Problem**: The translation must persist **two** things — the constant theme key and the `theme_migrated` marker — and §10.5 forbids doing it in two calls: each field-specific saver performs its own RMW, so two calls leave a reachable window, and the translation's write is explicitly best-effort and liable to be cut short. A failure landing between them persists the theme key with the marker unset; the next launch then finds the marker false, sees a theme key already set, writes only the marker, and **never emits the event** — the translation succeeded while the log says it failed, the one reading §12.3 designs the event to make impossible. Separately, §10.3's no-op condition cannot be evaluated against the load-time snapshot on the write half: the write is non-blocking, so a user can commit a theme in the window between compute and persist, and a pending translation evaluated against the stale snapshot would write `theme = tokyo-night` over the `nord` they just committed and clear the slots — §10.3's own failure, displaced from cross-launch to intra-process.

**Solution**: One `SaveTranslation` saver on `*prefs.Store` that re-evaluates the whole no-op condition **inside** task 6-1's RMW, against the bytes about to be merged, and commits the theme key plus the marker (or the marker alone) in exactly one atomic write — reporting back whether a theme key was actually persisted.

**Outcome**: On a file holding `appearance: dark` and no theme keys, `SaveTranslation("tokyo-night")` writes `theme` plus the marker in one write and reports `persisted=true`; on a file where any theme key is already set — including one committed by another instance moments earlier — it writes the marker alone and reports `persisted=false`; on an absent file it writes nothing; on a malformed file it aborts.

**Do**:
- Add `func (s *Store) SaveTranslation(slug string) (persisted bool, err error)` over task 6-1's `mutate`, with the entire decision made inside the mutator against the re-read record:
  1. `existed == false` → write nothing, `persisted=false`, nil error. It inherits the **abort** half of task 6-1's rule but **not** the create half.
  2. `f.ThemeMigrated` already true → write nothing, `persisted=false`. Another instance recorded the translation between this instance's load and this write; the trigger fires exactly once ever.
  3. any of `f.Theme` / `f.ThemeLight` / `f.ThemeDark` non-empty, **or** `slug == ""` → set the marker only, `persisted=false`. "Skip" means skip the **theme keys**, not the whole write — the marker is still recorded so the translation does not stay pending forever.
  4. otherwise → `f.Theme = slug`, clear both slots (mutual exclusion applies — it writes a constant), set the marker, `persisted=true`.
- Document the `slug == ""` case explicitly: an empty slug means "there was nothing to translate" (`appearance` was `auto`, absent or unrecognised) and is a **marker-only** write, which is §8.1's "the marker is still set, so the condition is not re-evaluated forever". It is not an error and must not be rejected.
- Comment at branch 3 that there is never anything to clear on the writing path — the translation only writes theme keys when all three are empty — so mutual exclusion is satisfied trivially rather than by a second rule.
- Return `persisted` as a first-class result, not an inference from the error: task 6-6's `theme: appearance migrated` fires **only** when a theme key was actually persisted, and a marker-only run translated nothing.
- Emit **nothing**: `prefs` is a leaf. Comment that the migration's failure signal is the *absence* of `theme: appearance migrated` (task 6-6) and that `theme: commit failed` stays single-sited on task 6-7's persister — this saver never logs and never reports.
- Comment that an aborted or failed write leaves the condition true, so the next launch retries, which is what makes best-effort safe.
- Keep it a single `AtomicWrite` in every writing branch — assert this in the test rather than trusting the shape.

**Acceptance Criteria**:
- [ ] `SaveTranslation("tokyo-night")` on `{"appearance":"dark"}` writes `theme` **and** `theme_migrated` in **one** write and returns `persisted=true`.
- [ ] The same call on a file whose `theme_dark` is set writes the **marker only**, leaves `theme_dark` untouched, and returns `persisted=false`.
- [ ] The same holds when the pre-existing key is `theme` or `theme_light`, and when it was written **after** this instance's load-time snapshot (the condition is evaluated at the re-read).
- [ ] `SaveTranslation("")` on an eligible file writes the marker alone and returns `persisted=false`.
- [ ] A file whose marker is already `true` is left **byte-identical** and returns `persisted=false`.
- [ ] An **absent** file: nothing written, nothing created, `persisted=false`, nil error.
- [ ] A malformed file: aborts with the error returned, bytes byte-identical, `persisted=false`.
- [ ] The writing branch clears both slots (they are empty by construction, asserted anyway) and preserves `session_list_mode` and the raw `appearance` verbatim.
- [ ] Exactly one `AtomicWrite` occurs in every writing branch — never two.
- [ ] Calling `SaveTranslation` twice in succession persists a theme key only on the first call (the second sees the marker).
- [ ] The saver emits nothing — a `logtest.Sink` installed process-wide records zero entries.

**Tests**:
- `"it writes the theme key and the marker in one write"` — `TestSaveTranslation_KeyAndMarkerInOneWrite`
- `"it skips the theme key when one is already set"` — `TestSaveTranslation_ExistingKeySkipsThemeKeys` (table: `theme`, `theme_light`, `theme_dark`)
- `"it evaluates the no-op at the re-read"` — `TestSaveTranslation_NoOpEvaluatedAtReRead` (key committed between snapshot and save; asserts the committed value survives)
- `"it still records the marker when it skips"` — `TestSaveTranslation_SkipStillRecordsTheMarker`
- `"it treats an empty slug as marker-only"` — `TestSaveTranslation_EmptySlugIsMarkerOnly`
- `"it does nothing when another instance already migrated"` — `TestSaveTranslation_AlreadyMigratedIsANoOp`
- `"it never creates an absent file"` — `TestSaveTranslation_DoesNotCreateAbsentFile`
- `"it aborts on an undecodable file"` — `TestSaveTranslation_AbortsOnUndecodable`
- `"it reports whether a theme key was persisted"` — `TestSaveTranslation_ReportsPersisted`
- `"it clears the slots when it writes a constant"` — `TestSaveTranslation_WritesAConstant`
- `"it emits nothing"` — `TestSaveTranslation_IsSilent`

**Edge Cases**:
- The theme key and the marker land in **one** write — issuing two leaves a reachable window (the translation's write is explicitly liable to be cut short), and a failure between them persists the key with the marker unset, so the next launch finds the marker false, sees a key already set, writes only the marker and never emits the event: the translation succeeded while the log says it failed.
- §10.3's **no-op condition is evaluated at the RMW re-read**, against the bytes about to be merged and never against the load-time snapshot — because the write is non-blocking a user can commit a theme in between, and against a stale snapshot the pending translation would write `theme = tokyo-night` over the `nord` they just committed and clear the slots.
- The same re-read is what lets the migration observe that **another instance already set `theme_migrated`**.
- **"Skip" means skip the theme keys, not the whole write** — the marker is still recorded so the translation does not stay pending forever.
- It reports back **whether a theme key was actually persisted**, because task 6-6's event fires only on that.
- It writes a **constant**, so mutual exclusion applies and the slots are cleared — there is simply never anything to clear, since it only writes when all three keys are empty.
- It inherits the **abort** half (a re-read that does not decode aborts rather than overwrites) but **not** the create half — it never creates an absent file.
- It emits nothing at all, `prefs` being a leaf: the migration's failure signal is the *absence* of `theme: appearance migrated`, and `theme: commit failed` stays single-sited on task 6-7's persister.
- An aborted or failed write leaves the condition true so the next launch retries, which is what makes best-effort safe.
- An empty slug (nothing to translate) is a legitimate marker-only call, not an error.

**Context**:
> §10.5: "**The theme key and the marker land in one write.** §8.9's field-specific save methods each perform their own read-modify-write, so issuing two would leave a reachable window — §10.5's write is best-effort and non-blocking, i.e. explicitly liable to be cut short. A failure between them persists the theme key with the marker unset, and the next launch then finds the marker false, sees a theme key already set, writes only the marker, and therefore never emits the event: the translation succeeded while the log says it failed, which is the one reading §12.3 designs the event to make impossible. The migration therefore uses a combined save rather than two calls."
> §8.9: "**§10.3's no-op condition is evaluated at the RMW re-read, against the bytes about to be merged — never against the load-time snapshot.** … The same re-read is what lets the migration observe that another instance already set `theme_migrated`." And: "**The migration write inherits only the abort half.** … What it does inherit is the rule that matters: **a re-read that does not decode aborts rather than overwrites.**"
> §8.9: "**'Skip' means skip the theme keys, not the whole write** — the marker is still recorded, so the translation does not stay pending forever. And once a mid-session commit supersedes the translated value, **the commit is the model's active theme**."
> §10.3: "**If any theme key is already set, the translation writes no theme key** — it only sets the marker. This is not absence-gating the trigger; it is refusing to clobber a choice the user has already made… **Mutual exclusion still applies** when the translation *does* write: it writes a constant, so it clears the slots. There is simply never anything to clear, because it only writes when all three keys are empty."
> §12.3: `theme: appearance migrated` is "Emitted on **successful persist**, not on compute… Tied to the persist, it fires exactly once — and its absence after a translation is itself the signal that the write failed."
> **Ambiguity flagged**: the spec does not say whether an *absent* `prefs.json` at the translation's re-read is an error or a silent no-op. It is specified here as a silent no-op returning nil, because §8.1 bars the migration from creating the file, the write is best-effort with no reporting surface (§8.9), and an error would invite a caller to treat a normal fresh-install state as a failure. Record the choice in a source comment.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §10.5, §10.3, §8.9, §8.1, §12.3

## theming-system-6-5

### Task 6.5: Split the prefs read and compute the marker-gated `appearance` translation in memory

**Problem**: Phase 5 task 5-7 deleted `prefs.Appearance` and its API with its last caller, so an install carrying `"appearance": "dark"` and no theme keys currently renders the shipped **adaptive pair** — §10.1's silent flip, live in the tree, for exactly the population who expressed a preference (the README today *recommends* pinning it). The value is still on disk untouched (task 5-1) but nothing reads it. Two further constraints meet at the same call site: the translation must run where a TUI is constructed and be able to log under the `theme` component, which rules out `prefs` (a leaf that must not import `internal/log`) and points at `cmd/config.go`'s `loadPrefsStore`; and the moment `loadPrefsStore` gains that behaviour, `portal doctor` — which must read `prefs.json` to report an unresolvable theme while **healing nothing on its read-only path** — needs a variant that does not inherit it, or running a diagnosis mutates the user's config as a side effect.

**Solution**: Split `loadPrefsStore` into a **migrating** loader and a **non-migrating** one. The migrating loader resolves the path, reads the theme keys and the migration state once, applies §10.2's exact mapping under §10.3's marker gate and §10.3's no-op condition (evaluated against the load-time snapshot), and returns the **post-translation** keys `openTUI` hands to `theme.ResolveSetting`. Nothing is written in this task.

**Outcome**: An install with `"appearance": "dark"`, no theme keys and no marker renders `tokyo-night` **on this launch** — the silent flip closed — while `prefs.json` is still byte-unchanged; an install that already set `theme_dark = nord` renders Nord and translates nothing; and `loadPrefsStoreNoMigrate` gives Phase 7's doctor the same store with no computation at all.

**Do**:
- Rename today's `loadPrefsStore` body to `func loadPrefsStoreNoMigrate() (*prefs.Store, error)` in `cmd/config.go` — unchanged (`prefsFilePath` + `prefs.NewStore`). Document it as the read `portal doctor` uses (Phase 7) and pin the contract: it must never gain behaviour, because doctor's read-only claim is what a one-shot config mutation during a diagnosis would break.
- Add the migrating loader and its result:
  ```go
  // prefsLoad is what the migrating prefs load produces. Keys is §8.4's "as read":
  // the POST-translation in-memory value, not the on-disk bytes.
  type prefsLoad struct {
      Store              *prefs.Store
      Keys               prefs.ThemeKeys
      TranslationPending bool   // the marker is not yet recorded (task 6-6 persists)
      TranslatedSlug     string // §10.2's mapping; "" when there is nothing to translate
  }
  func loadPrefsStore() (prefsLoad, error)
  ```
- Compute inside it, in this order:
  1. `prefsFilePath()` → `prefs.NewStore`. A path error returns the error unchanged; `openTUI` already degrades rather than blocking.
  2. `keys, _ := store.LoadThemeKeys()` and `st, _ := store.LoadMigrationState()` — both tolerant, both discarding the error exactly as the existing initial-mode read does (every degenerate case yields empty values, which is the shipped pair).
  3. `st.Migrated` → `TranslationPending = false`, `TranslatedSlug = ""`, `Keys = keys`. Done.
  4. Otherwise `TranslationPending = true` and `TranslatedSlug = translateAppearance(st.Appearance)`.
  5. **In-memory half, gated on the load-time snapshot**: apply `TranslatedSlug` to `Keys` (as `ThemeKeys{Theme: slug}`) **only** when the slug is non-empty **and** all three raw keys are empty. Otherwise `Keys = keys` unchanged.
- Single-source §10.2's mapping in `func translateAppearance(raw string) string`: exact `"dark"` → `theme.DefaultDarkSlug`, exact `"light"` → `theme.DefaultLightSlug`, everything else (`"auto"`, absent, `"Dark"`, `"  dark"`, `"sepia"`) → `""`. Use Phase 2 task 2-8's constants, never string literals. Comment the exact-match rule's justification: the translation must reproduce **the old binary's** reading of the value, and the deleted `parseAppearance` matched exactly — so anything the old binary treated as `auto` must translate to nothing.
- Comment that `TranslatedSlug` is **not** zeroed by the load-time no-op check: §10.5 checks the condition twice against two reads deliberately — at load for the in-memory half, and again at task 6-4's RMW re-read for the write half — so collapsing them into one would lose the second read's job (absorbing another instance's commit).
- Re-point `cmd/open.go`'s `openTUI` (Phase 5 task 5-7's wiring) at the new shape: take `load.Keys` instead of calling `prefsStore.LoadThemeKeys()` directly, keep the same single `*prefs.Store` instance serving the initial mode read and (from task 6-7) the theme persister, and keep the existing degrade-not-block tolerance — a `loadPrefsStore` error leaves a nil store, zero keys and the shipped pair, never a blocked TUI.
- Update the existing test callers of `loadPrefsStore` (`cmd/prefs_path_test.go`, `cmd/open_initial_mode_test.go`) to the new signature; they assert path resolution and the initial mode, neither of which changes.
- **Write nothing in this task.** No saver is called, no goroutine is started; task 6-6 owns the persist and the event.

**Acceptance Criteria**:
- [ ] `{"appearance":"dark"}` with no theme keys and no marker yields `Keys = {Theme: "tokyo-night"}`, `TranslationPending=true`, `TranslatedSlug="tokyo-night"` — and `prefs.json` is **byte-identical** afterwards.
- [ ] `{"appearance":"light"}` yields `tokyo-night-day` under the same conditions; `"auto"`, absent and `"sepia"` yield `TranslatedSlug=""` with `TranslationPending` still **true** (the marker is owed even when nothing translates).
- [ ] `"Dark"`, `" dark"` and `"DARK"` translate to nothing — the exact-match rule reproduces the old binary's reading.
- [ ] A file with the marker `true` performs no mapping at all: `TranslationPending=false`, `Keys` exactly as read, whatever `appearance` holds.
- [ ] With `appearance: dark` **and** any theme key already set (`theme`, `theme_light` or `theme_dark`), `Keys` is returned **unchanged** — the user's setting is what renders — while `TranslatedSlug` is still populated for the write half to re-evaluate.
- [ ] The reachable loss-of-setting sequence is closed end to end: `{"appearance":"dark","theme_dark":"nord"}` with no marker renders Nord this launch (not Tokyo Night).
- [ ] An **absent** `prefs.json` yields zero keys, `TranslatedSlug=""`, and creates nothing.
- [ ] A corrupt `prefs.json` yields zero keys with no error from the load (tolerant path untouched) and does not block TUI construction.
- [ ] Both default slugs come from `theme.DefaultDarkSlug` / `theme.DefaultLightSlug`; a test that hardcodes `"tokyo-night"` is not acceptable.
- [ ] `loadPrefsStoreNoMigrate()` performs **no** read of `prefs.json` beyond what `prefs.NewStore` does (which is none) and computes no translation.
- [ ] The migrating loader has exactly one production caller (`openTUI`), so "runs only where a TUI is constructed" holds structurally — asserted by a source-level guard over `cmd`.
- [ ] `portal open <target>` still reads no prefs and does no theme work (Phase 5 task 5-7's exec-path test stays green).

**Tests**:
- `"it translates a dark pin to the equivalent constant"` — `TestLoadPrefsStore_TranslatesDark`
- `"it translates a light pin"` — `TestLoadPrefsStore_TranslatesLight`
- `"it translates nothing for auto, absent or unrecognised"` — `TestLoadPrefsStore_NoTranslationCases` (table incl. `sepia`, and asserting the marker is still pending)
- `"it matches the appearance value exactly"` — `TestTranslateAppearance_ExactMatchOnly` (table: `Dark`, ` dark`, `DARK`, `dark\n`)
- `"it does not translate once the marker is set"` — `TestLoadPrefsStore_MarkerGatesTheTranslation`
- `"it applies the no-op condition in memory"` — `TestLoadPrefsStore_ExistingKeySuppressesTheInMemoryValue` (table: `theme`, `theme_light`, `theme_dark`)
- `"it renders the hand-edited slot this launch"` — `TestLoadPrefsStore_HandEditedSlotWinsOnTheTranslatingLaunch`
- `"it writes nothing"` — `TestLoadPrefsStore_ComputesWithoutWriting` (byte-compare; absent file stays absent)
- `"it uses the shared default slugs"` — `TestTranslateAppearance_UsesSharedConstants`
- `"it tolerates an absent or corrupt file"` — `TestLoadPrefsStore_TolerantOnDegenerateFiles`
- `"it keeps the non-migrating read inert"` — `TestLoadPrefsStoreNoMigrate_ComputesAndWritesNothing`
- `"it has one migrating caller"` — `TestLoadPrefsStore_SingleProductionCaller` (source guard over `cmd`)

**Edge Cases**:
- The mapping is **exact**: `dark` → `theme = tokyo-night`, `light` → `theme = tokyo-night-day`, `auto` → nothing, absent → nothing — a pinned mode becomes a pinned theme and detection stays off for them just as it was.
- A **present-but-unrecognised** `appearance` translates nothing yet is still a case the marker covers (§10.2's "Nothing" refers to the *theme keys*), so the condition is not re-evaluated forever.
- Exact matching is deliberate: anything the deleted `parseAppearance` read as `auto` (`Dark`, `" dark"`) must translate to nothing, or the translation would change the meaning of a value rather than preserve it.
- The trigger is the **marker, never the absence of theme keys** — absence-gating is re-armable and composes badly with §9.9's hand-edit escape hatch, silently re-pinning a user who deleted their keys to return to the shipped pair.
- The **no-op condition governs the in-memory half too**, evaluated against the **load-time snapshot** (the only moment early enough to affect what is painted): scoping it to the write alone produces a one-launch silent flip on §10.3's own reachable sequence — hand-edit `theme_dark = nord`, launch, and the pending translation renders Tokyo Night for that launch and Nord thereafter.
- **"As read" means the post-translation in-memory value**, so the keys handed to `theme.ResolveSetting` (and, in Phase 8, to the panel) are the translated ones — handing over the disk bytes would make a migrated user's badges claim two shipped defaults while a constant paints the screen and would stop `d`/`l` raising §9.2's confirm.
- An **absent** `prefs.json` translates nothing and writes nothing.
- Ownership is `cmd/config.go`'s `loadPrefsStore` because `prefs` is a leaf that must not import `internal/log`, the translation happens at prefs load, and the `theme` component records it — `prefs` stays dumb.
- The **non-migrating variant** ships in the same act so doctor (Phase 7) heals nothing on its read-only path, a one-shot config mutation as a side effect of running a diagnosis being exactly what breaks that contract.
- `loadPrefsStore` has one production caller, so "runs only where a TUI is constructed" falls out structurally — `portal open <target>` constructs no TUI and reads no prefs.
- Prefs is still read **once** per process through the one store instance that serves the initial mode, the theme keys and task 6-7's persister.
- `TranslatedSlug` survives the load-time no-op check unzeroed, because the condition is checked twice against two reads by design.

**Context**:
> §10.1: "Real installs hold `"appearance": "dark"` or `"light"` today — the README currently *recommends* pinning it. Deleting `prefs.Appearance` makes that field unknown… a user who deliberately pinned `dark` on a light terminal upgrades into the shipped adaptive pair and **silently gets a light Portal** with nothing explaining why. That is the worst outcome for precisely the group who expressed a preference."
> §10.2's table: `dark` → `"theme": "tokyo-night"`; `light` → `"theme": "tokyo-night-day"`; `auto` → Nothing; absent → Nothing. "Intent is preserved precisely rather than approximately: a pinned mode becomes a pinned theme, and detection stays off for them just as it was."
> §10.5: "**`cmd/config.go`'s `loadPrefsStore` owns the translation.** Three decided constraints meet here: `prefs` is a deliberate leaf that must not import `internal/log`; the translation happens at prefs load; the `theme` log component records it… **`prefs` stays dumb.** **`cmd/config.go` also exposes a non-migrating read variant, for `portal doctor`.** … a one-shot config mutation as a side effect of running a diagnosis breaks that."
> §10.5: "**Separate *computing* from *persisting*.** At prefs load, read `appearance`, compute the translated theme, and **use it in memory immediately**… **§10.3's no-op condition governs both halves, and for the in-memory half it is evaluated against the load-time snapshot.** … The condition is therefore checked twice against two reads, deliberately."
> §8.4: "**'As read' means the post-translation in-memory value, not the on-disk bytes**… Handing the panel the disk bytes instead would make a migrated user's badges claim two shipped defaults while a constant is actually painting the screen, and would stop `d`/`l` raising §9.2's confirm — silently doing the thing the confirm exists to prevent, to the one population §10.1 identifies as needing protection."
> Phase boundary: Phase 5 task 5-7's note records that deleting `LoadAppearance` reintroduces §10.1's silent flip for exactly one phase — **this task closes it**, and the `appearance` value survives untouched on disk (task 5-1) precisely so the translation is exact. The persist and its event are task 6-6; doctor's consumption of the non-migrating read is Phase 7.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §10.1, §10.2, §10.3, §10.5, §8.4, §12.2

## theming-system-6-6

### Task 6.6: Persist the translation best-effort and emit `theme: appearance migrated`

**Problem**: Task 6-5 computes the translation and uses it in memory, but nothing records the marker — so the translation stays pending forever, recomputing on every launch, and §10.3's "fires exactly once ever" is unenforceable. The persist cannot simply be made part of the load, either: §10.5 requires it to be **best-effort and non-blocking**, so that a write failure means Portal renders the correct theme this launch and retries next launch rather than flipping the user to a wrong theme — which was the translation's entire purpose. And the event has exactly one honest firing point: emitted on *compute* it could legitimately fire on several consecutive launches, making "one-shot" false; emitted on a *marker-only* write it would announce a migration that translated nothing; and if it fired on failure at all, its absence would stop being the failure signal §10.5 designs it to be.

**Solution**: Dispatch task 6-4's `SaveTranslation` from `loadPrefsStore` behind a package-level seam that runs it off the launch path, and emit `theme: appearance migrated` (INFO) from `cmd` **only** when the saver reports a persisted theme key.

**Outcome**: A user upgrading with `"appearance": "dark"` launches once, sees Tokyo Night immediately, and finds `theme` plus `theme_migrated` written to `prefs.json` with one `theme: appearance migrated` line in `portal.log`; every subsequent launch does nothing and logs nothing; and a failed write leaves both the file and the log silent, with the condition still true for next time.

**Do**:
- Reuse `cmd`'s package-level `themeLogger`, bound by Phase 3 task 3-2 and shared by this task and task 6-7. Do **not** add a second `log.For("theme")` call: CLAUDE.md's rule is bind once *per package*, and §8.9 explicitly legitimises the `theme` component being emitted from three packages (the loader, this translation, the persister) — which is a per-package rule, not a per-call-site licence.
- Add the dispatch seam in `cmd/config.go`:
  ```go
  // persistTranslation performs §10.5's best-effort, non-blocking persist of the
  // one-shot appearance translation. A package-level var so tests substitute a
  // synchronous implementation and restore it with t.Cleanup (cmd's established
  // *Deps idiom) rather than sleeping on a goroutine.
  var persistTranslation = func(store *prefs.Store, slug string) {
      go func() {
          persisted, err := store.SaveTranslation(slug)
          if err != nil || !persisted {
              return // absence of the event IS the failure signal (§10.5)
          }
          themeLogger.Info("appearance migrated", "slug", slug)
      }()
  }
  ```
- Call it from `loadPrefsStore` when `TranslationPending` is true, passing `TranslatedSlug` (which may be `""` — task 6-4 turns that into a marker-only write). Do **not** wait on it, do not propagate its error, and do not let it affect the returned `prefsLoad`.
- Comment the three properties the shape rests on: the process may exit mid-write and `AtomicWrite` guarantees no partial file; the condition remains true so the next launch retries; and task 6-4's RMW re-read is what stops the deferred write reverting a commit the user made in between.
- Emit **nothing** on failure — no `theme: commit failed` (that event is single-sited on task 6-7's persister and belongs to the panel's commits), no WARN, no user-facing surface. Record in a comment that the translation is deliberately **silent to the user at runtime**: it runs before any surface exists, intent is preserved exactly so there is nothing to explain, and §6.3 already refuses the single-slot notice band a permanent extra contender. The CHANGELOG (Phase 10) is the compensating channel.
- Keep attrs inside §12.3's closed set and the message verbatim from the catalogue (`appearance migrated`, rendered `theme: appearance migrated`). Carry `slug` — the constant actually persisted — and no `slot` (the translation always writes a constant).
- Do not touch the exec path: `loadPrefsStore`'s only production caller is `openTUI`, so the dispatch is unreachable from `portal open <target>`; keep Phase 5's exec-path assertion green.

**Acceptance Criteria**:
- [ ] A pending translation with `appearance: dark` and no theme keys persists `theme = tokyo-night` **and** `theme_migrated = true`, and emits exactly one INFO `theme: appearance migrated` carrying `slug=tokyo-night`.
- [ ] A second launch against the resulting file writes nothing and emits **nothing**.
- [ ] A marker-only run (`appearance: auto`, or a theme key already set) writes the marker and emits **nothing** — the event fires only on a persisted theme key.
- [ ] A failed write (malformed `prefs.json`, unwritable directory) emits **nothing at all** — no `appearance migrated`, no `commit failed`, no WARN — and leaves the condition true so the next launch retries.
- [ ] The persist does not block the load: `loadPrefsStore` returns before the write completes (proven through the seam, not by timing), and its result is identical whether the write succeeds or fails.
- [ ] A theme committed between compute and persist survives — the deferred write does not revert it (drives task 6-4's re-read end to end).
- [ ] Two instances translating concurrently produce the same file content and at most one `appearance migrated` per instance that actually persisted; the loser writes nothing.
- [ ] The event is INFO, its message matches §12.3's catalogue verbatim, and every attr key is drawn from the closed set (`slug`, `slot`, `reason`, `path`, `token`, `count`, `rejected`).
- [ ] No test sleeps: the seam is substituted synchronously and restored with `t.Cleanup`.
- [ ] The exec path (`portal open <target>`) reaches neither the dispatch nor the saver — zero `theme` records and a byte-unchanged `prefs.json`.

**Tests**:
- `"it persists the translation and emits the event"` — `TestPersistTranslation_WritesAndEmits` (`logtest.Sink`)
- `"it emits nothing on a subsequent launch"` — `TestPersistTranslation_OneShot`
- `"it emits nothing for a marker-only write"` — `TestPersistTranslation_MarkerOnlyIsSilent`
- `"it emits nothing when the write fails"` — `TestPersistTranslation_FailureIsSilentAndRetryable`
- `"it does not block the load"` — `TestLoadPrefsStore_PersistIsNonBlocking`
- `"it does not revert a commit made in between"` — `TestPersistTranslation_DoesNotRevertAConcurrentCommit`
- `"it is idempotent across concurrent instances"` — `TestPersistTranslation_ConcurrentInstancesConverge`
- `"it emits at INFO with closed attrs"` — `TestPersistTranslation_EventShape`
- `"it never emits commit failed"` — `TestPersistTranslation_NeverEmitsCommitFailed`
- `"it is silent to the user"` — `TestPersistTranslation_NoFlashOrNoticeBand` (no model-facing signal is produced)
- `"it never runs on the exec path"` — `TestOpenExecPath_NoTranslation`

**Edge Cases**:
- The write is **best-effort and non-blocking** — a failure means Portal renders the correct theme this launch and retries next launch (the condition is still true), so it can never flip the user to the wrong theme, which was the translation's entire purpose.
- Non-blocking must not delay first paint and must not race the model's own writes in a way that reverts them — task 6-4's re-read is what absorbs a commit made in between, and once a mid-session commit supersedes the translated value **the commit is the model's active theme**.
- The process may exit before the write lands; `AtomicWrite` guarantees there is no partial file, and the pending condition survives.
- `theme: appearance migrated` is **INFO, one-shot, and fires only on a successful theme-key persist** — never on compute (which could legitimately fire on several consecutive launches, making "one-shot" false) and never on a marker-only write (a run that writes the marker alone translated nothing, so announcing a migration would be false).
- Its **absence after a translation is itself the signal** that the write failed, which is what keeps `theme: commit failed` single-sited on task 6-7's persister — the migration emits it **never**.
- An already-`true` marker means the translation does not run, so no event on any subsequent launch.
- Several burst-launched instances hitting the condition simultaneously all compute **the same value from the same input**, so it is idempotent regardless, and the loser observes the marker at the re-read and writes nothing.
- The emission site is `cmd`, so the `theme` component is legally bound in a third package (the loader, this translation, and the persister) under CLAUDE.md's bind-once-*per-package* rule.
- Attrs stay inside the closed set and the message matches §12.3's catalogue verbatim.
- The translation is **silent to the user at runtime** — no flash, no notice band, no banner: it runs before any surface exists, there is nothing to explain since intent is preserved exactly, and §6.3 already refuses the single-slot notice band a permanent seventh contender (the CHANGELOG is the compensating channel, Phase 10).

**Context**:
> §10.5: "**Separate *computing* from *persisting*.** … the write is **best-effort and non-blocking**. A failed write means Portal renders the correct theme this launch and retries next launch (the condition is still true), so it can never flip the user to the wrong theme — which was the translation's entire purpose."
> §10.5: "**Concurrency is doubly safe here:** the write goes through §8.9's read-modify-write like every other, and beyond that several burst-launched instances hitting the condition simultaneously all compute **the same value from the same input**, so it is idempotent regardless. It never runs on the exec path, which constructs no TUI and reads no prefs."
> §10.5: "**The translation is silent to the user at runtime.** No flash, no notice band, no banner — the log line is a forensic trail with **no user-facing interruption**… **There is nothing to explain.** The translation preserves intent exactly — a pinned mode becomes a pinned theme and detection stays off, just as it was… **It runs at prefs load, before any surface exists** to render a notice into. **The notice band is a single-slot arbiter with six contenders already.**"
> §12.3: "`theme: appearance migrated` | INFO | Emitted on **successful persist**, not on compute. §10.5's write is best-effort and retries next launch, so a compute-time emission could legitimately fire on several consecutive launches and 'one-shot' would be false. Tied to the persist, it fires exactly once — and its absence after a translation is itself the signal that the write failed."
> §8.9: "**The migration write inherits only the abort half.** It runs at prefs load, before any panel exists, so it has no reporting surface and needs none… It emits **no** `theme: commit failed` — its failure signal is the *absence* of `theme: appearance migrated`… which keeps the commit-failed event single-sited on the theme persister."
> **Ambiguity flagged**: §12.3 pins the event's level and cadence but not its attrs. `slug` (the persisted constant) is carried because it is inside the closed key set and is what makes the line greppable per theme; no `slot` is carried, since the translation always writes a constant. Record the choice beside the emission.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §10.5, §12.3, §8.9, §6.3, §10.3

## theming-system-6-7

### Task 6.7: The `WithThemePersister` seam and the `cmd`-owned theme persister

**Problem**: The panel's commit write has nowhere to live yet, and the two obvious homes are both barred. `prefs` cannot own it: it is a leaf that must not import `internal/log`, so it can neither resolve the prefs path nor record `theme: commit failed`. The model cannot own it either: if `internal/tui` logged the failure the event would either double up or become a fourth package emitting the `theme` component, and §8.9 closes that set at three. Yet without a site, `theme: commit failed` has **no** emission site at all — §12.3 lists it as a WARN with `slug` / `slot` / `reason` and §8.9 pins it to the persister specifically because nothing else can produce it. And Phase 9's outstanding-failure state machine needs the failure back as a **value**: a persister that only logs would leave the panel unable to render `⚠ couldn't save theme` and unable to hold the failure outstanding, recreating the silent "applied but not persisted" state the picker idiom exists to close.

**Solution**: A `ThemePersister` seam on `internal/tui` — interface, `Deps` field and `WithThemePersister` option, nil-tolerant — implemented by a small `cmd`-owned type that wraps the shared `*prefs.Store` and the `theme` component logger, emits `theme: commit failed` on error **and returns the error**, wired beside the existing `modePersister` under the same typed-nil guard.

**Outcome**: `cmd`'s persister commits a constant or a slot through task 6-2's savers, logs one WARN per failed write with `slug` / `slot` / `reason`, and hands the error back to its caller; the model holds the seam and logs nothing; a fixture or `capturetool` model gets a nil persister and writes nowhere.

**Do**:
- Declare the seam in `internal/tui/model.go`, immediately after `ModePersister` so the parallel is visible:
  ```go
  // ThemePersister commits the theme panel's choice. Unlike ModePersister the
  // production implementation is NOT *prefs.Store: cmd owns the write so it can
  // resolve the prefs path and emit `theme: commit failed` from a single site.
  // The method names differ from the store's deliberately, so *prefs.Store cannot
  // accidentally satisfy this interface and silently drop the emission.
  type ThemePersister interface {
      CommitTheme(slug string) error
      CommitThemeSlot(slug string, slot prefs.ThemeSlot) error
  }
  ```
  plus `m.themePersister`, `WithThemePersister(p ThemePersister) Option`, a `Deps.ThemePersister` field, and the nil-guarded option append in `build.go` — mirroring `WithModePersister` line for line (`if deps.ThemePersister != nil { … }`).
- Add `cmd/theme_persister.go`:
  ```go
  type themePersister struct {
      store  *prefs.Store
      logger *slog.Logger
  }
  func (p themePersister) CommitTheme(slug string) error
  func (p themePersister) CommitThemeSlot(slug string, slot prefs.ThemeSlot) error
  ```
  Each calls the matching task 6-2 saver; on error emit `p.logger.Warn("commit failed", "slug", slug, "reason", err.Error())` — plus `"slot", "light"|"dark"` on the slot method, **absent** on the constant method — and **return the error verbatim**. On success emit nothing (the commit-time `theme: loaded` belongs to the loader, Phase 9).
- Single-source the `prefs.ThemeSlot` → `"light"`/`"dark"` attr rendering in one small helper beside the persister, so the string can never drift from the one task 5-5 already uses for `theme.Slot`.
- Wire it in `cmd/open.go` beside the existing `cfg.modePersister = prefsStore` assignment, inside the same `if prefsStore != nil` block, and repeat the existing comment's reasoning: a typed-nil `*prefs.Store` boxed into an interface is non-nil, which would defeat `buildTUIModel`'s nil check.
- Leave `cmd/capturetool` and every fixture path passing **no** persister (nil), matching `ModePersister` today — a commit during a capture must write nowhere.
- Add nothing to the model beyond holding the seam: no logging, no retry, no state. Phase 9 owns the keypresses, the outstanding-failure state machine and `⚠ couldn't save theme`.
- Verify the seam **by direct call** in this phase — nothing presses a key until Phase 9.

**Acceptance Criteria**:
- [ ] `CommitTheme("nord")` writes `theme` and clears both slots through `SaveTheme`; `CommitThemeSlot("nord", prefs.SlotDark)` writes `theme_dark` and clears the constant through `SaveThemeSlot`.
- [ ] A failed write emits exactly one WARN `theme: commit failed` carrying `slug` and `reason`, **plus** `slot=dark`/`slot=light` for a slot commit and **no** `slot` attr for a constant commit.
- [ ] The persister **returns the error** as well as logging it — asserted explicitly, since Phase 9's report depends on the value.
- [ ] A successful commit emits **nothing**.
- [ ] The model emits no `theme` record on any path — `theme: commit failed` has exactly one emission site, asserted by a source guard over `internal/tui`.
- [ ] `*prefs.Store` does **not** satisfy `tui.ThemePersister` (compile-time assertion in a test), so the logging site cannot be bypassed by wiring the store directly.
- [ ] A nil `Deps.ThemePersister` leaves the model with a nil persister and no option applied; a model built that way does not panic when the seam is exercised.
- [ ] The persister is wired only when `prefsStore != nil`, so a typed-nil store is never boxed into a non-nil interface.
- [ ] `capturetool` and the fixture harness build models with a nil theme persister; a capture writes no `prefs.json`.
- [ ] The seam is exercised by direct call only — no keypress path exists yet.
- [ ] Each instance persists its own change with no file watch and no cross-instance sync: two persisters over two stores on the same file leave last-write-wins semantics with every other field intact (task 6-1's RMW).

**Tests**:
- `"it commits a constant through the store"` — `TestThemePersister_CommitTheme`
- `"it commits a slot through the store"` — `TestThemePersister_CommitThemeSlot` (light and dark)
- `"it logs and returns a failed write"` — `TestThemePersister_FailedCommitLogsAndReturns` (`logtest.Sink`; malformed prefs file)
- `"it carries slot only for a slot commit"` — `TestThemePersister_CommitFailedAttrs`
- `"it emits nothing on success"` — `TestThemePersister_SuccessIsSilent`
- `"it is the only emission site"` — `TestCommitFailed_SingleEmissionSite` (source guard: `internal/tui` emits no `theme` record)
- `"the store does not satisfy the seam"` — `TestPrefsStore_DoesNotSatisfyThemePersister` (compile-time assertion)
- `"it tolerates a nil persister"` — `TestBuild_NilThemePersisterIsTolerated`
- `"it guards the typed-nil store"` — `TestOpenTUI_ThemePersisterWiredOnlyWithAStore`
- `"it writes nowhere in a capture"` — `TestCapturetool_NoThemePersister`
- `"it does not sync across instances"` — `TestThemePersister_PerInstanceLastWriteWins`

**Edge Cases**:
- Mirrors `WithModePersister` **exactly** in shape — an interface in `internal/tui`, a `Deps` field, an option, and the same nil-guard at the wiring site so a typed-nil `*prefs.Store` boxed into the interface is not mistaken for a live persister (the existing `modePersister` guard is the precedent).
- It is **owned by `cmd`**, which resolves the prefs path, calls the field-specific savers and is the **single emission site** for `theme: commit failed` — the model must not also log or the event doubles, and without this site the event has none.
- The persister **returns the error as well as logging it**, because Phase 9's outstanding-failure state machine and its `⚠ couldn't save theme` message need it — logging alone would leave the panel unable to report, recreating the silent "applied but not persisted" state the picker idiom exists to close.
- `theme: commit failed` is **WARN** carrying `slug`, `slot` (**absent** when committing a constant) and `reason`.
- The three constraints that decide ownership are unchanged from the translation's — `prefs` is a leaf that must not import `internal/log`, the write needs prefs path resolution, and the `theme` component records the failure.
- The rejected alternative was exporting a whole-record type with `Load`/`Save` so `cmd` performs the merge literally — it hands any caller an API that can clobber the file wholesale, the opposite of what keeping the merge single-sited inside the leaf protects, and it is what makes the raw `appearance` round-trip a property of the store rather than a rule every caller must remember.
- The slot is the **existing typed** value (`prefs.ThemeSlot`), not a new string, so the seam cannot mint a third slot.
- Interface method names deliberately differ from the store's (`CommitTheme` vs `SaveTheme`) so `*prefs.Store` cannot silently satisfy the seam and bypass the single emission site.
- Wiring sits beside the existing `modePersister` assignment on the same store instance read once per process.
- A fixture / `capturetool` model gets a **nil** persister so a commit during a capture writes nowhere, matching `ModePersister` today.
- Each instance persists its own change with **no file watch and no cross-instance sync** — other instances are unaffected until relaunch, exactly as `session_list_mode` already behaves.
- **Nothing presses a key in this phase** — the panel that drives the seam is Phase 8/9, so the seam is verified by a direct call.

**Context**:
> §8.9: "**The panel's commit write is owned by `cmd`, not by `prefs` or by the TUI** — a theme persister injected at construction through a `WithThemePersister` option, exactly the shape `WithModePersister` already has. The same three constraints that decided §10.5's ownership apply unchanged here: `prefs` is a leaf that must not import `internal/log`, the write needs prefs path resolution, and the `theme` component records its failure. The persister resolves the path, calls `prefs`, and is **the emission site for `theme: commit failed`**, which otherwise has none."
> §8.9: "This means the `theme` component is emitted from more than one package — the loader (`internal/theme`), the translation (`cmd/config.go`), and this persister. That is legal and normal: CLAUDE.md's rule is *bind once per package*."
> §8.9: "Each instance loads its theme at construction; an instance that changes theme persists it; **other instances are unaffected until relaunch.** There is no file watch. This is exactly how `session_list_mode` already behaves — the `s` toggle persists per-instance with no cross-instance sync, via the existing `ModePersister` seam that a theme persister follows."
> §12.3: "`theme: commit failed` | WARN | Per failed write. Carries `slug`, `slot` (absent when committing a constant), and `reason`."
> §9.13: a failed write "**Reports in the panel's message slot** … **Keeps the theme applied in memory** … **Does not move the `●`**… This recreates 'applied but not persisted', but as a *reported* state rather than a silent one, which is the distinction the picker idiom was buying." (Phase 9 consumes the returned error to do this.)
> Phase boundary: this task closes Phase 6 on the seam Phase 8/9 builds against. Phase 8 opens the panel and previews; Phase 9 presses `Enter`/`d`/`l`, raises the slot-from-constant confirm, and owns the outstanding-failure state machine and its flashes.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §8.9, §12.3, §9.13, §10.5
