# Phase 5: The theme setting — resolution, fallback and detection — 7 tasks

## theming-system-5-1

### Task 5.1: `prefs.json` decodes the three theme keys and preserves raw `appearance`

**Problem**: `prefs.json`'s on-disk struct (`prefsFile` in `internal/prefs/store.go`) declares exactly two fields — `session_list_mode` and `appearance`. The theme setting is three new flat string keys (`theme`, `theme_light`, `theme_dark`), and nothing can read them today. Worse, `prefs.json` decodes into a plain Go struct, so **any key not declared as a field is dropped on re-encode**, and every writer re-encodes the whole file: the moment the theme keys exist on disk without being declared, the first `s` keypress blanks them. The mirror hazard runs the other way for `appearance` — §8.8 keeps it on disk as a frozen legacy value for downgrade, so deleting the *field* (rather than just the enum, task 5-7) would silently erase a user's pin on their first grouping-mode toggle after upgrade, invisible until they downgrade.

**Solution**: Add three `omitempty` string fields to `prefsFile` plus a tolerant, read-only accessor returning all three raw values, and re-declare the existing `appearance` field as an `omitempty` **preserved raw string** with a doc comment pinning why it must never be deleted. No validation, no trimming, no defaulting, no slug knowledge — `prefs` stays the dumbest possible leaf.

**Outcome**: `Store.LoadThemeKeys()` returns the three raw strings exactly as written (or empty strings for every degenerate input), an existing `appearance` value survives an arbitrary number of `Save` calls byte-for-byte, and unset keys are *absent* from the written JSON rather than empty-stringed.

**Do**:
- Extend `prefsFile` in `internal/prefs/store.go`:
  ```go
  type prefsFile struct {
      SessionListMode string `json:"session_list_mode"`
      Appearance      string `json:"appearance,omitempty"`
      Theme           string `json:"theme,omitempty"`
      ThemeLight      string `json:"theme_light,omitempty"`
      ThemeDark       string `json:"theme_dark,omitempty"`
  }
  ```
  Leave `session_list_mode` exactly as it is (it always marshals a canonical non-empty token, so `omitempty` would be inert there and changing it is out of scope).
- Add `type ThemeKeys struct { Theme, Light, Dark string }` and `func (s *Store) LoadThemeKeys() (ThemeKeys, error)` reading through the existing `readFile()` so it inherits today's tolerant policy verbatim: missing file, empty file, corrupt JSON and missing fields all yield zero-valued strings with **no** error; only a non-`ErrNotExist` read error propagates (alongside the zero `ThemeKeys`).
- Document on `LoadThemeKeys` that it performs **no** interpretation: no `ValidSlug` check, no trimming, no lowercasing, no default substitution, no `theme`-wins tiebreak. An unrecognised value is a *resolution* problem (tasks 5-2/5-3/5-4), not a decode one, and trimming would convert a stray-space value into a silently-different slug instead of the honest `bad name` §5.2 wants.
- Rewrite the `Appearance` field's doc comment to state the retention contract in force from task 5-7 onward: the field is **read and preserved, never parsed** — its `Appearance` enum, `parseAppearance`, `LoadAppearance` and `SaveAppearance` die with their last caller in task 5-7, while the *slot in the file* stays so a downgraded binary still honours the user's pin (§8.8, §10.4). Add the reason a future reader needs: an undeclared key is dropped on re-encode and every writer re-encodes.
- Leave `Save` / `SaveAppearance` / `write` unchanged — the existing tolerant read-modify-write stands. The strict write-path decode, the four field-specific savers (`SaveTheme`, `SaveThemeSlot`, `SaveMigrationMarker`, `SaveTranslation`) and the create-on-absent / abort-on-undecodable split are **Phase 6**.
- Do **not** declare `theme_migrated`. Add a one-line comment recording the boundary: nothing writes the marker until Phase 6, which declares the field before its first writer exists, so no on-disk marker can be dropped in the interim.
- Keep `internal/prefs` a leaf: no `internal/log`, no `internal/theme`, no `internal/xdg` import. The existing `internal/prefs/leaf_guard_test.go` must stay green unedited.

**Acceptance Criteria**:
- [ ] `LoadThemeKeys()` on a file containing `{"theme":"nord"}` returns `{Theme:"nord"}` with empty `Light`/`Dark` and a nil error.
- [ ] A missing file, an empty file, a corrupt-JSON file and a file with none of the three keys all return a zero `ThemeKeys` with a **nil** error; only a non-`ErrNotExist` read error propagates.
- [ ] An unrecognised value (`"theme":"Nord"`, `"theme":"../evil"`, `"theme":"  nord "`) is returned **verbatim** — no rejection, no trim, no case change.
- [ ] `Save(ModeByTag)` on a file holding `appearance`, `theme`, `theme_light` and `theme_dark` preserves all four values unchanged, and the written JSON re-decodes to identical `ThemeKeys`.
- [ ] Unset keys are **absent** from the written JSON (`omitempty`): saving a mode onto a file that never had theme keys writes no `theme`, `theme_light`, `theme_dark` or `appearance` key at all.
- [ ] An existing `"appearance":"dark"` survives ten successive `Save` calls byte-identically.
- [ ] `theme_migrated` is neither declared nor written; the encoded output never contains the key.
- [ ] `internal/prefs` still imports only the standard library and `internal/fileutil` (leaf guard green, unedited).
- [ ] `prefs.Appearance`, `LoadAppearance` and `SaveAppearance` still exist and still behave exactly as today — task 5-7 deletes them with their last caller.

**Tests**:
- `"it decodes the three theme keys"` — `TestLoadThemeKeys_DecodesAllThree`
- `"it returns empty keys for every degenerate input"` — `TestLoadThemeKeys_TolerantDecode` (table: missing file, empty file, corrupt JSON, `{}`, keys present but empty-string)
- `"it propagates a non-ErrNotExist read error"` — `TestLoadThemeKeys_PropagatesReadError`
- `"it returns unrecognised values verbatim"` — `TestLoadThemeKeys_NoValidationOrNormalisation` (table: uppercase, path traversal, leading/trailing space, embedded tab)
- `"it preserves the theme keys across a mode save"` — `TestSave_PreservesThemeKeys`
- `"it preserves a raw appearance value across repeated saves"` — `TestSave_PreservesRawAppearance`
- `"it omits unset keys on write"` — `TestSave_OmitsEmptyThemeKeysAndAppearance`
- `"it declares no migration marker"` — `TestPrefsFile_DeclaresNoMigrationMarker`
- `"it stays a leaf"` — existing `internal/prefs/leaf_guard_test.go`, unedited

**Edge Cases**:
- Three **flat, independent** string fields — never a polymorphic `theme` (string *or* object) and never an always-object form; tolerant decode stays exactly as dumb as today, with no type probing and no "what does a corrupt value degrade to" question.
- Missing, empty or unrecognised falls to the shipped default **per field** — but that defaulting happens in task 5-2, not here: `prefs` returns the empty string and knows nothing about defaults.
- Decode never validates a slug. An unrecognised value is a resolution problem, not a decode one.
- No trimming or normalisation, so `"  nord"` reaches §8.6's charset check and fails as `bad name` — the reason that maps to the mistake — rather than silently becoming `nord`.
- `appearance` stays a **declared** field precisely because any undeclared key is dropped on re-encode and every writer re-encodes the whole file. Its named failure is that the first `s` keypress after upgrade erases the user's pin, invisible until a downgrade.
- `omitempty` across the theme keys and `appearance` so an unset key is *absent* rather than empty-stringed — this keeps a hand-edited file clean, matches §8.3's "an unset slot holds the shipped default" semantics, and lets a downgraded binary read an absent `appearance` as absent rather than as an empty string.
- `theme_migrated` is deliberately **not** declared here — nothing writes it until Phase 6, so there is no on-disk marker to round-trip and declaring it early would add a field with no consumer.
- The existing `Save` keeps its current tolerant read-modify-write; Phase 6 owns the strict write-path decode and the abort-on-undecodable rule.

**Context**:
> §8.1: "`prefs.json` gains three theme keys and one migration marker alongside the existing `session_list_mode`… Three flat keys match what `prefs.json` already is — a flat map of scalars — so **tolerant decode stays exactly as dumb as today**: missing, empty or unrecognised falls to the shipped default *per field*, with no type probing." Rejected: a polymorphic `theme` field and an always-object form.
> §8.1: "**Empty values are omitted on write** (`omitempty` across the theme keys and the retained `appearance`). The §8.1 example above shows the full schema, not the on-disk shape: a key the user has never set is *absent*."
> §8.8: "**`prefsFile` keeps a raw `appearance string` field, so the on-disk value round-trips.** This is load-bearing, not tidiness: `prefs.json` decodes into a plain Go struct, so **any key not declared as a field is dropped on re-encode** — and §8.9 makes every writer re-encode the whole file. Delete the field and the first `s`-keypress or theme commit after upgrade silently erases the user's `appearance` pin… The field is a **plain string that is read and preserved, never parsed**."
> §13.6's new prefs test: "This is the one part of the feature whose failure mode is silent, permanent destruction of a user's config, and none of it is observable at the moment it goes wrong." This task carries the round-trip half of that suite; Phase 6 carries the merge, marker and migration halves.
> Phase boundary: task 5-1 **adds fields only**. `prefs.Appearance` cannot die here because `cmd/open.go` still calls `LoadAppearance()` (Phase 3 task 3-2's in-memory mapping) until task 5-7 replaces the caller — the same discipline 3-2 applied to `WithAppearance`.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §8.1, §8.8, §8.9, §10.4, §13.6

## theming-system-5-2

### Task 5.2: Derive the theme setting — a constant or a defaulted pair, with `theme` winning

**Problem**: Three independent raw strings are not a setting. §8.2 collapses them into exactly **two** states — constant or adaptive pair — and the collapse carries four rules that nothing implements: a non-empty `theme` wins and the slots are then *not read at all*; "nothing set" and "pair nominated" are the **same** state, so there is no unconfigured branch, only a default per slot; an unset slot holds `tokyo-night-day` / `tokyo-night`, the same constants the fallback resolves to; and every value reaching the panel or doctor must be **control-stripped at the point it is read**, since a pasted newline, tab or ANSI escape in `prefs.json` would otherwise corrupt whichever surface the user is reading to *find* the problem. Getting the tiebreak wrong on a hand-edited file carrying both forms makes §9.5's "the two setting states never coexist on screen" unenforceable.

**Solution**: One pure function in `internal/theme` mapping the three raw strings onto a `Setting` value (constant slug, or a light/dark slug pair with the shipped defaults substituted per unset slot) **and** the control-stripped raw keys, so what the panel later *lists* and what it *marks* derive from a single evaluation and cannot disagree.

**Outcome**: `theme.ResolveSetting("nord", "", "")` yields a constant; `("", "", "nord")` yields `{light: tokyo-night-day, dark: nord}`; `("", "", "")` yields the shipped pair; `("nord", "x", "y")` yields the constant `nord` with the slots ignored but the raw keys still returned verbatim (control-stripped) for later surfaces.

**Do**:
- Add `internal/theme/setting.go` declaring:
  ```go
  type RawKeys struct{ Theme, Light, Dark string } // control-stripped, as read
  type Setting struct {
      IsConstant  bool
      Constant    string // slug, non-empty iff IsConstant
      Light, Dark string // slugs, both non-empty iff !IsConstant
  }
  func ResolveSetting(theme, light, dark string) (Setting, RawKeys)
  ```
- Apply `StripControl` (Phase 2 task 2-10's helper) to each of the three inputs **first**, and return them as `RawKeys`. Document the reuse contract: this is the §9.5 "stripped at the point it is read, not at the point it is drawn" rule — a property of the value, inherited by the panel row and doctor's advisory line alike. **Truncation is separate and stays panel-local** (Phase 8); doctor has full width and wants the whole value.
- Tiebreak: if the stripped `theme` is non-empty, return `{IsConstant: true, Constant: theme}` and **do not read the slots at all** — leave `Setting.Light`/`Setting.Dark` empty. State in-source that this is what makes §9.5's "the two setting states never coexist on screen" a *resolution rule* rather than a file constraint, and that the stale slots are left untouched on disk with nothing pruning them.
- Otherwise return the pair, substituting `DefaultLightSlug` for an empty light and `DefaultDarkSlug` for an empty dark (Phase 2 task 2-8's constants — never string literals). Comment that these are the **same constants** task 5-4's fallback resolves to, and that §8.3's "the adaptive pair degrades to a constant dark default" argument rests on that coincidence.
- Document that partial pairs do not exist: the shipped values are the slots' *defaults*, so `theme_dark = nord` alone is `{tokyo-night-day, nord}` and there is no incomplete-pair state to validate, explain or render around.
- Document that the empty string is the unambiguous **unset** sentinel because §5.2's anchoring makes an empty slug illegal, so it can never be a real name.
- Add no variant concept anywhere: the function never inspects a palette, and it cannot — it deals in slugs only. The **slot classifies the theme** (§4.7).
- Keep the function pure and total: no I/O, no logging, no error return, deterministic for a given input triple.

**Acceptance Criteria**:
- [ ] `("nord","","")` → constant `nord`; `("","","")` → pair `{tokyo-night-day, tokyo-night}`; `("","nord","")` → `{nord, tokyo-night}`; `("","","nord")` → `{tokyo-night-day, nord}`.
- [ ] `("nord","solarized","gruvbox")` → constant `nord`, with `Setting.Light`/`Setting.Dark` **empty** (the slots were not read), while `RawKeys` still carries all three values.
- [ ] Both defaults are expressed as `DefaultLightSlug` / `DefaultDarkSlug`, asserted by comparing against those constants — a test that hardcodes `"tokyo-night"` is not acceptable.
- [ ] A value carrying a newline, tab, carriage return or ANSI escape is control-stripped in **both** the `Setting` and the `RawKeys`, and the result is single-line.
- [ ] A value that is **only** control characters strips to empty and is therefore treated as unset (see Edge Cases — the ambiguity is flagged and resolved this way deliberately).
- [ ] Interior spaces and case are **preserved** — `"  nord"` and `"Nord"` come back unchanged, so task 5-3's charset check is what rejects them, as `bad name`.
- [ ] The function performs no file or environment access and returns no error; calling it twice with the same input yields identical results.
- [ ] Nothing in the function references a `Theme`, a palette or a canvas value — slugs only.

**Tests**:
- `"it treats a non-empty theme as a constant"` — `TestResolveSetting_ConstantWins`
- `"it leaves the slots unread under a constant"` — `TestResolveSetting_ConstantIgnoresSlots` (both slots set, both absent from `Setting`, both present in `RawKeys`)
- `"it defaults an unset slot to the shipped slug"` — `TestResolveSetting_UnsetSlotsTakeShippedDefaults` (table: neither set, light only, dark only)
- `"it expresses the defaults as the shared constants"` — `TestResolveSetting_DefaultsAreTheSharedConstants`
- `"it control-strips every key at the point it is read"` — `TestResolveSetting_ControlStripsAllThree` (table: `\n`, `\t`, `\r`, `\x1b[31m`, a mixed payload)
- `"it treats a control-only value as unset"` — `TestResolveSetting_ControlOnlyValueIsUnset`
- `"it preserves spaces and case for the charset check"` — `TestResolveSetting_NoTrimOrLowercase`
- `"it returns the raw keys alongside the setting"` — `TestResolveSetting_ReturnsRawKeysForTheSameEvaluation`
- `"it is pure and deterministic"` — `TestResolveSetting_IsPureAndDeterministic`

**Edge Cases**:
- "Nothing set" and "pair nominated" are the **same state** — the shipped default *is* an implicit pair, so there is no unconfigured branch, only a default value per slot.
- A non-empty `theme` wins and the slots are **not read at all**; stale slots are left untouched on disk and nothing prunes them.
- Partial pairs do not exist — `theme_dark = nord` alone yields light `tokyo-night-day` / dark `nord`, because light was never overridden.
- An unset slot defaults to `DefaultLightSlug` / `DefaultDarkSlug`, the **same constants** task 5-4's fallback resolves to — the coincidence §8.3's degrades-to-a-constant-dark-default argument rests on.
- The empty string is the unambiguous *unset* sentinel because §5.2's anchoring makes an empty slug illegal.
- Control-stripping is a property of the value and happens where it is read, so the panel row and doctor's line both inherit it; truncation is separate and stays panel-local.
- Mutual exclusion is a **write** rule (Phase 6). This task implements only the read-side `theme`-wins tiebreak for a hand-edited file carrying both.
- No variant concept anywhere — the slot classifies the theme and nothing inspects a palette.

**Context**:
> §8.2: "A theme setting is either **Constant** — `"theme": "nord"`. Detection is never consulted — or **Adaptive**… 'Nothing set' and 'pair nominated' are **the same state**… **If a hand-edit leaves both present, `theme` wins** — a documented deterministic rule… **`theme` winning means the slots are not read at all**, so the panel renders a single bare `●` on the constant's row and no slot badges — §9.5's 'the two setting states never coexist on screen' holds because the resolution rule makes the pair invisible, not because the file cannot contain both."
> §8.3: "**Partial pairs do not exist.** The adaptive form always has two slots and the shipped values are their *defaults*… There is no incomplete-pair state to validate, explain, or render around, and the shipped default and a partially-overridden pair are **the same mechanism** rather than two."
> §9.5: "**A slug that came from `prefs.json` is control-stripped at the point it is read, not at the point it is drawn** — it is a property of the value, so every consumer inherits it… A pasted newline, tab or ANSI escape would otherwise corrupt whichever of them the user is reading to find the problem. **Truncation is separate and stays panel-local** — doctor has full width and wants the whole value."
> §8.4: "**The constructor also takes the raw persisted theme keys**… The nomination alone is insufficient for the panel" — a slug that never loaded is not in the nomination, a badge needs the *persisted* slug, and §14A's confirm renders the persisted constant. This task produces those raw keys; Phase 8 owns the constructor slot and every consumer of them.
> **Ambiguity flagged**: the spec pins control-stripping and pins "non-empty `theme` wins", but does not say which happens first for a value that is *only* control characters. Stripping first (so such a value is unset, not an illegal slug) is chosen because §9.5 makes the stripped form "the value" for every consumer, and because the alternative would mint a panel row labelled with an empty string. Record the choice in a source comment so a later phase can revisit it deliberately.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §8.1–§8.3, §9.5, §4.7, §5.2

## theming-system-5-3

### Task 5.3: By-name theme resolution — charset check, embedded set, then the themes directory

**Problem**: A nominated slug comes from a **hand-editable file** and is used to locate a file *by name* on a path that deliberately never enumerates — so `"theme": "../../etc/passwd"` would be used verbatim as a path component. Ordering matters just as much: construction cannot *detect* a shadowing collision because it never lists the directory, so §5.4's no-shadowing safety property (the fallback built-in must never be resolvable to a user's broken file) has to come from **resolving the embedded set first**. And the three not-loadable outcomes must not be conflated: `not found` sends the user to check a filename, `unreadable` sends them to check permissions, `bad name` sends them to fix what they typed — reporting the wrong one sends them looking in the wrong place. Today the only by-name resolver is the inline sequence inside `portal theme export` (task 2-9), which would silently drift from construction's.

**Solution**: One `Loader` entry point in `internal/theme` — charset-validate, try the embedded set, only then compose `<themesDir>/<slug>.theme` — returning either a loaded theme or exactly one §6.2 reason, and re-point `portal theme export` onto it so the two by-name resolvers cannot diverge.

**Outcome**: `ResolveByName("tokyo-night", dir)` returns the embedded theme without touching `dir` even when `dir` is mode `0000`; `ResolveByName("../evil", dir)` returns `bad name` without composing any path; a missing file returns `not found`; an unreadable directory returns `unreadable` and emits one deduplicated `theme: directory unusable`.

**Do**:
- Add `func (l Loader) ResolveByName(slug, themesDir string) (Result, *Rejection)` to `internal/theme` (alongside `LoadFile` / `LoadBuiltin` / `LoadPath`), documented as the single by-name resolver shared by construction (task 5-7) and `portal theme export`.
- **Step 1 — charset.** `ValidSlug(slug)` (Phase 1 task 1-4's exported rule, applied to a non-file input with no extension involved). Failure → `bad name`, **before any path is composed**. Comment that this is what stops `../something` becoming a path component, and that `bad name` — never `not found` — is the reason, per §9.4's discrimination.
- **Step 2 — embedded set.** `LoadBuiltin(slug)`; `found == true` → return that result (or its rejection, which a correct binary never produces). A nominated built-in therefore **never reads the themes directory at all** — state in-source that this ordering *is* the no-shadowing safety property on the non-enumerating path, and that construction is where the fallback resolves, which is the exact thing no-shadowing exists to protect.
- **Step 3 — the directory.** With an **empty** `themesDir` (the injected value could not be resolved, task 5-7) return `not found` immediately: compose no path, emit nothing. Otherwise `os.Stat(themesDir)`:
  - `os.IsNotExist` → `not found`, **silently** — an absent directory is the common case and never an error or a log line.
  - Not a directory (a regular file where a directory belongs), or any other stat error → `unreadable` carrying the OS error, plus one `theme: directory unusable` through the injected event logger.
  - Otherwise `LoadFile(filepath.Join(themesDir, slug+".theme"))`, and **map its `unreadable`-with-`os.ErrNotExist` to `not found`** — the file simply is not there. Every other read failure stays `unreadable` with the OS error verbatim. Comment this as the same absent-versus-unusable discrimination §5.5 draws for the directory and §12.1 draws for export.
- Content reasons (`bad syntax`, `bad colour`, `missing tokens`) pass back from `LoadFile` **unchanged**; `reserved name` is structurally unreachable here (a built-in slug returned at step 2) and a filename `bad name` is unreachable (the composed basename is always `<valid-slug>.theme`) — assert both so the impossibility is pinned rather than assumed.
- **No `ReadDir` on any path**, and exactly **one** file read per resolution. Enumeration belongs to panel open (Phase 8), and a directory read here would pay §5.7's rejected startup scan on the cold path.
- The loader still never resolves the directory itself — `themesDir` stays an injected value from `cmd/config.go`'s `themesDirPath` (Phase 1 task 1-6), which is what keeps the embedded set reachable with no path at all and `internal/capture`'s no-real-config import guard satisfiable.
- **Re-point `portal theme export`** (`cmd/theme.go`, task 2-9): delete its inline `ValidSlug` → `LoadBuiltin` → `filepath.Join` sequence and call `ResolveByName(slug, themesDir)`. Its §14A frames map off the returned reason unchanged (`not found` → `no theme named <slug>`; `bad name` → `theme <slug> is not valid: bad name`; a content reason → `theme <slug> is not valid: <reason>`; `unreadable` → `theme <slug> could not be read: <OS error>`), and its `log.Discard`-backed loader keeps it emitting no `theme` events — including the new `directory unusable` line.

**Acceptance Criteria**:
- [ ] `ResolveByName("tokyo-night", dir)` succeeds with `dir` at mode `0000`, with `dir` absent, and with `dir` holding a deliberately broken `tokyo-night.theme` — in every case returning the **embedded** theme, and in no case reading the directory.
- [ ] `ResolveByName("../evil", dir)`, `("-nord", dir)`, `("Nord", dir)`, `("nord lee", dir)` and `("", dir)` each return `bad name`; no path is composed and no file is opened (proven by placing a readable file where a naive join would land).
- [ ] A valid drop-in at `<dir>/nord-lee.theme` resolves with its slug and theme.
- [ ] An absent `<dir>/nord-lee.theme` (directory present) returns `not found`, **not** `unreadable`, and emits nothing.
- [ ] An absent directory returns `not found` and emits nothing; an empty `themesDir` string does the same without composing a path.
- [ ] An unreadable directory and a regular file where the directory belongs each return `unreadable` with the OS error, plus exactly one `theme: directory unusable` record; five successive calls still emit one (dedup on `path`+`reason` lives on the injected logger).
- [ ] A drop-in with a duplicate key returns `bad syntax`; a bad hex `bad colour`; a missing token `missing tokens` — unchanged from `LoadFile`.
- [ ] `reserved name` is never returned; a filename `bad name` is never returned.
- [ ] Exactly one file read occurs per resolution and no `ReadDir` call occurs on any path.
- [ ] `portal theme export` routes through `ResolveByName`, its four §14A frames are byte-unchanged from task 2-10's assertions, and it still emits zero `theme` records.

**Tests**:
- `"it validates the persisted slug before composing a path"` — `TestResolveByName_CharsetCheckedBeforePathComposition` (table incl. `../evil`, asserting `bad name` and that no read occurred)
- `"it resolves the embedded set before the directory"` — `TestResolveByName_BuiltinNeverReadsDirectory` (0000-mode dir, absent dir, shadowing broken file)
- `"it resolves a valid drop-in by name"` — `TestResolveByName_DropInResolves`
- `"it returns not found for an absent file"` — `TestResolveByName_AbsentFileIsNotFound`
- `"it returns not found for an absent or empty directory"` — `TestResolveByName_AbsentOrEmptyDirectoryIsNotFound`
- `"it returns unreadable for an unusable directory"` — `TestResolveByName_UnusableDirectoryIsUnreadable` (table: 0000 directory, regular file in its place)
- `"it emits one directory-unusable record per process"` — `TestResolveByName_DirectoryUnusableIsDeduped`
- `"it passes content reasons through unchanged"` — `TestResolveByName_ContentReasonsPassThrough`
- `"it never mints reserved name or a filename bad name"` — `TestResolveByName_UnreachableReasonsAreUnreachable`
- `"it reads at most one file and never lists a directory"` — `TestResolveByName_NoReadDirAndSingleRead`
- `"it shares one resolver with export"` — `TestThemeExport_UsesSharedByNameResolver` (task 2-10's four frames re-run through the re-pointed path)

**Edge Cases**:
- The persisted slug is charset-validated **before** use as a path component, so `../something` never becomes one — the same `ValidSlug` rule Phase 1 exposed, applied to a non-file input with no extension involved.
- A charset failure is **`bad name`**, never `not found` — telling a user their file is missing when they typed an illegal name sends them looking in the wrong place.
- The **embedded set resolves first** and a nominated built-in never reads the themes directory at all; construction does not enumerate, so there is no collision to *detect* and the no-shadowing safety property has to come from ordering.
- Neither embedded nor a file yields **`not found`** — the one reason deliberately outside §6.2's ladder, because there is nothing to check.
- An unreadable directory, or a regular file where a directory belongs, yields **`unreadable`** rather than `not found` — permissions is the actual problem, the same discrimination export already draws.
- A `theme: directory unusable` entry fires from this construction-time read through the same per-process-deduped logger the panel's enumeration uses, so a user who never opens the panel still gets a log record and the two never double up.
- An **absent** directory is silent and simply yields `not found`; an empty injected directory string behaves identically and composes no path.
- Exactly one file read per resolution and **no `ReadDir` on any path** — enumeration belongs to panel open.
- Content reasons come back from `LoadFile` unchanged, while `reserved name` is structurally unreachable because a built-in slug never reaches the directory.
- The loader still never resolves the directory itself — it stays an injected value from `themesDirPath`.
- `portal theme export`'s inline ordering from task 2-9 re-points onto this single entry point so the two by-name resolvers cannot drift.
- The `0000`-mode directory test needs a `chmod` cleanup and should skip when the suite runs as root, where mode bits do not deny.

**Context**:
> §8.4: "**Resolution order on the by-name path: the embedded set first, then the themes directory.** A nominated slug that names a built-in resolves to the built-in and **never reads the themes directory at all**. This is what makes §5.4's no-shadowing guarantee implementable on the path that matters — construction does not enumerate, so there is no collision to *detect* there; the safety property has to come from ordering. And construction is where the fallback resolves, which is the exact thing no-shadowing exists to protect."
> §8.6: "The persisted value comes from a hand-editable file and is used to **locate a file by name** on a path that deliberately does not enumerate — so `../something` would be used as a path component. **Validate the persisted slug against the same `[a-z0-9-]` charset before use**, and treat an invalid one as unresolvable."
> §5.5: "**A theme made unreachable by an unusable directory carries the reason `unreadable`**, not `not found`… `not found` sends the user to check the filename, `unreadable` sends them to check permissions — and permissions is the actual problem." The `theme: directory unusable` entry is "emitted both by the panel's enumeration and by the construction-time by-name read… deduplicated per process so the two never double up. Emitting from construction too is what gives a user who never opens the panel a log record at all."
> §5.7: "**At construction**, Portal loads **only the nominated themes by name** — one file read for a constant, two for an adaptive pair. No enumeration." Rejected: a startup scan and `fsnotify` watching.
> §9.4: "A persisted slug **rejected by the charset check** (§8.6) before any file is sought gets a row with reason **`bad name`**, not `not found` — the reason maps to the actual failure, and each §6.2 reason has exactly one condition."
> Phase boundary: this task resolves **one slug**. Per-slot fallback selection is task 5-4, its events task 5-5, the fatal unresolvable-fallback task 5-6, and the wiring into construction task 5-7. The §9.4 union behind the `ThemeEnumerator` seam is Phase 8.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §8.4, §8.6, §5.4, §5.5, §5.7, §6.2, §9.4, §12.1

## theming-system-5-4

### Task 5.4: Per-slot mode-matched fallback for an unloadable nomination

**Problem**: A nominated theme can fail to load for six unrelated reasons — a deleted file, a renamed file, a typo in `prefs.json`, an illegal slug, a missing token, a bad colour — and Portal must still paint *something* without inventing a mechanism per cause and without ever overwriting the user's persisted choice (which would turn a transient failure into a destructive one: fixing the file must restore the theme on the next launch with no re-selection). The fallback also cannot be a single fixed theme: a light-terminal user with a typo in their light slot would be thrown to a dark palette, a bigger surprise than falling to the light default. And every later surface — task 5-5's events, Phase 7's doctor line, Phase 8's rule that the `●` stays on the *persisted* slug while the cursor sits on the fallback's row — needs to know **which** slot fell back, **from** which slug and **for** which reason, which is unrecoverable if the fallback is applied silently inside a resolver that returns only a `Theme`.

**Solution**: A resolution entry point in `internal/theme` that walks the `Setting`'s one-or-two slots through task 5-3's by-name resolver, substitutes the **mode-matched** default on failure (`theme_light` → `DefaultLightSlug`, `theme_dark` / a constant → `DefaultDarkSlug`), assembles the loaded `Nomination`, and returns a **structured per-slot record** of what was asked for, what loaded, and why they differ.

**Outcome**: `ResolveNomination(setting, dir)` returns a `Nomination` ready for `tui.Build` plus one `SlotResolution` per slot; a broken light slot renders `tokyo-night-day` while the dark slot keeps `nord`; `prefs.json` is byte-unchanged on every path.

**Do**:
- Declare in `internal/theme`:
  ```go
  type Slot int // SlotConstant, SlotLight, SlotDark
  type SlotResolution struct {
      Slot      Slot
      Requested string   // the slug that was nominated (shipped default when the slot was unset)
      WasSet    bool     // true when Requested came from prefs rather than the shipped default
      Resolved  string   // the slug actually loaded
      FellBack  bool
      Reason    Reason   // populated iff FellBack
      Theme     Theme
  }
  type Resolution struct {
      Nomination Nomination
      Slots      []SlotResolution // exactly 1 under a constant, exactly 2 (light, dark) under a pair
  }
  func (l Loader) ResolveNomination(s Setting, themesDir string) (Resolution, error)
  ```
- Per slot: call `ResolveByName`. On success record `FellBack=false`. On any rejection record the reason, then resolve the **fallback slug for that slot** — `SlotLight → DefaultLightSlug`, `SlotDark → DefaultDarkSlug`, `SlotConstant → DefaultDarkSlug` — through the same resolver, and record `FellBack=true` with `Resolved` set to the fallback slug.
- Express the fallback map in terms of `DefaultLightSlug` / `DefaultDarkSlug` (never literals) and carry §8.5's warning verbatim in substance in a comment: the fallback values **are** the shipped default's values, and *changing them, or adopting the rejected single-fixed-fallback alternative, silently invalidates §8.3's "the adaptive pair degrades to a constant dark default" argument.*
- Treat an **unset** slot as ordinary resolution, not a fallback: task 5-2 already substituted the shipped default into `Setting`, so the slot resolves normally with `WasSet=false` and `FellBack=false`. State in-source that this is why unset and unloadable converge with no second mechanism — one rule ("an unset slot holds the shipped default") applied to a slot that is *set but unloadable*.
- Assemble the `Nomination` from the resolved themes using Phase 3 task 3-2's constructors: `ConstantNomination(t)` under a constant, `AdaptivePair(light, dark)` under a pair. Nothing here selects a member — the gate does that (task 5-7).
- **Write nothing.** Add an explicit comment that falling back never touches `prefs.json`: the persisted name is kept so fixing the file restores it on the next launch, and overwriting would make a transient failure destructive (§6.3).
- Both slots may fall back independently in one launch; the surviving slot must be unaffected. Do not short-circuit after the first failure.
- If a **fallback itself** fails to resolve, return the error — task 5-6 owns that fatal and its pinned copy. Never fall back a second time, and never substitute a hardcoded palette.
- Emit nothing from this function directly; task 5-5 wires `theme: loaded` / `theme: fallback applied` onto exactly these outcomes.

**Acceptance Criteria**:
- [ ] A constant naming a deleted drop-in resolves `tokyo-night` with `FellBack=true`, `Requested` the persisted slug, `Reason` `not found`.
- [ ] A broken **light** slot falls back to `tokyo-night-day`; a broken **dark** slot to `tokyo-night`; a broken constant to `tokyo-night` — each asserted against `DefaultLightSlug`/`DefaultDarkSlug`, not literals.
- [ ] Every cause takes the same path with only `Reason` differing: `not found` (deleted/renamed file), `bad name` (illegal persisted slug), `missing tokens`, `bad colour`, `bad syntax`, `unreadable` (directory or file).
- [ ] Both slots broken in one launch → both fall back, two `SlotResolution`s with `FellBack=true`, and the returned pair carries the two shipped defaults.
- [ ] One slot broken → the other slot's `SlotResolution` has `FellBack=false` and its theme is unchanged.
- [ ] An **unset** slot yields `WasSet=false`, `FellBack=false` and the shipped default's theme — a virgin install produces **zero** fallbacks.
- [ ] `prefs.json` bytes are identical before and after resolution on every path, and no file is created when it is absent.
- [ ] `Slots` has length 1 under a constant and 2 (light then dark) under a pair; `Nomination.IsConstant()` matches `Setting.IsConstant`.
- [ ] A fallback that cannot resolve returns an error rather than a second fallback or a zero-valued `Theme` (task 5-6 pins the message).

**Tests**:
- `"it falls back per slot to the mode-matched default"` — `TestResolveNomination_FallbackIsModeMatched` (table: light slot, dark slot, constant)
- `"it uses the shared default constants as the fallback"` — `TestResolveNomination_FallbackUsesSharedConstants`
- `"it takes one path for every cause"` — `TestResolveNomination_EveryCauseFallsBack` (table over the six reasons)
- `"it falls back on both slots independently"` — `TestResolveNomination_BothSlotsCanFallBack`
- `"it leaves the surviving slot untouched"` — `TestResolveNomination_SurvivingSlotUnaffected`
- `"it treats an unset slot as a default, not a fallback"` — `TestResolveNomination_UnsetSlotIsNotAFallback`
- `"it never writes the persisted name"` — `TestResolveNomination_NeverOverwritesPrefs` (byte-compare before/after; absent file stays absent)
- `"it records which slot fell back, from what, and why"` — `TestResolveNomination_StructuredOutcome`
- `"it assembles the matching nomination shape"` — `TestResolveNomination_NominationShapeMatchesSetting`
- `"it returns an error when the fallback itself fails"` — `TestResolveNomination_UnresolvableFallbackErrors` (driven through task 5-6's seam)

**Edge Cases**:
- **One not-loadable path serves every cause** — deleted file, renamed file, typo in `prefs.json`, illegal slug, missing token, bad colour, unreadable — so no cause gets its own branch.
- The fallback is per slot: `theme_light` → `tokyo-night-day`, `theme_dark` → `tokyo-night`, a constant → `tokyo-night`.
- A single fixed fallback regardless of mode was **rejected** because it throws a light-terminal user with a typo'd light slot to a dark theme, a bigger surprise than falling to the light default.
- The fallback values **are** the shipped default's values — changing them, or adopting the rejected single-fixed alternative, silently invalidates §8.3's degrades-to-a-constant-dark-default argument.
- Falling back **never overwrites the persisted name** — fixing the file restores it on the next launch with no re-selection, and overwriting would make a transient failure destructive.
- Both slots can fall back independently in a single launch, and the surviving slot is unaffected.
- An **unset** slot is not a fallback at all but the same default rule, so unset and unloadable converge with no second mechanism.
- The outcome is **structured** (which slot, from which slug, for which reason) so task 5-5's events, Phase 7's doctor line and Phase 8's `●`-stays-on-the-persisted-slug rule all read one record instead of re-deriving it.
- A fallback that itself cannot resolve is task 5-6's fatal, never a second fallback and never a hardcoded palette.

**Context**:
> §8.5: "When a nominated theme is unloadable (invalid file, missing file, bad persisted slug): `theme_dark` → `tokyo-night`; `theme_light` → `tokyo-night-day`; `theme` (constant) → `tokyo-night`. This introduces **no new mechanism** — it is the already-decided 'an unset slot holds the shipped default' rule applied to a slot that is *set but unloadable* rather than unset." And: "**§8.3's second reason depends on that coincidence**… So **changing these values, or adopting the single-fixed-fallback alternative rejected below, silently invalidates §8.3.**"
> §8.5: "**One not-loadable path serves every cause** — a deleted file, a renamed file, a typo in `prefs.json`, a missing token, a bad colour. All fall back, keep the persisted name (§6.3), and surface through the panel, doctor and the log."
> §6.3: "**Falling back must never overwrite the persisted theme name in `prefs.json`.** Portal keeps the user's choice and renders the fallback; fixing the theme file restores it on the next launch without the user re-selecting. Overwriting would make the failure destructive rather than transient."
> §9.5's badge table needs exactly this record: a slot "Set but unloadable" keeps the badge on the **persisted** slug while "the fallback's own row carries no badge", and a "Never set" slot badges the **shipped default's** slug — which is why `WasSet` is carried separately from `Requested`.
> §12.2/§14A need the same record for doctor's `⚠ theme <slug> (<slot>) does not resolve: <reason>` line (Phase 7).
> Phase boundary: this task produces the outcome; task 5-5 emits it, task 5-6 escalates the unresolvable-fallback case, task 5-7 wires it into construction. Phase 8's panel consumes the same record for its cursor/badge rules.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §8.3, §8.5, §6.3, §9.5, §12.2

## theming-system-5-5

### Task 5.5: `theme: loaded` and `theme: fallback applied` on the injected logger

**Problem**: The `theme` log component exists to be **the record that survives without the user going looking** — the panel's row needs the panel opened and doctor needs invoking — but on a broken install a `grep "theme:"` currently answers nothing about what is actually rendering: Phase 1 shipped only `rejected` and `directory unusable`. Two failure modes are specific and both silent. If `theme: loaded` names the *nominated* slug when that nomination failed, then both it and `theme: fallback applied` name the slug that did **not** load and the log cannot say which palette is on screen — which is the greppability the whole component is justified on. And a persistently broken active theme re-resolves at construction, again on every panel open and again on every `Esc`, so an undeduplicated WARN turns a passive forensic trail into running commentary.

**Solution**: Two new methods on Phase 1's `EventLogger` — `Loaded` (INFO, per load, **not** deduplicated) and `FallbackApplied` (WARN, deduplicated per process on `slug`+`reason`) — emitted from task 5-4's per-slot outcomes so cadence and attrs are single-sited.

**Outcome**: A constant install logs one `theme: loaded slug=…`; a pair logs two, each with `slot=light|dark`; a broken slot logs `theme: fallback applied slug=<the one that failed> slot=… reason=…` **plus** a `theme: loaded` naming the fallback; and `log.Discard` silences all of it for doctor, export and `capturetool`.

**Do**:
- Extend `internal/theme/events.go`:
  - `func (e *EventLogger) Loaded(slug string, slot Slot)` — INFO, message `loaded`; attrs `slug` always, `slot` **only** when the slot is `SlotLight`/`SlotDark` (absent under a constant). **No dedup** — it is per-load, and Phase 9's commit-time load is the same event at a different cadence.
  - `func (e *EventLogger) FallbackApplied(slug string, slot Slot, r Reason)` — WARN, message `fallback applied`; attrs `slug` (the nomination that **failed**), `slot` where one applies, and `reason`. **Deduplicated per process on `slug`+`reason`**, on the same instance-held dedup state as `Rejected` / `DirectoryUnusable`.
- Emit from task 5-4's `ResolveNomination`, per slot, in a fixed order: on a fallback emit `FallbackApplied` first (the failure), then `Loaded` carrying the **fallback's** slug; on success emit `Loaded` alone. Exactly **one** `Loaded` per slot — the slug that actually loaded, never the one that failed.
- Render `slot` as the strings `light` / `dark`; a constant carries no `slot` attr at all (not an empty string, not `constant`).
- Carry **no** `count` and no enumeration attr — nothing is enumerated at construction. Keep every attr inside the closed set (`slug`, `slot`, `reason`, `path`, `token`, `count`, `rejected`); no key is invented at a call site.
- Update the in-source event catalogue comment Phase 1 left behind: mark `theme: loaded` and `theme: fallback applied` as implemented here and note that `loaded` **also fires at commit time** in Phase 9 for the newly-live opposite slot on a constant → adaptive conversion.
- Change nothing about emission control: the `*EventLogger` is injected, `cmd` passes `log.For("theme")` on the paths where a theme is *used* and `log.Discard()` on `portal doctor`, `portal theme export` and `capturetool`, and the dedup state lives on the instance so every path in a TUI process shares it.

**Acceptance Criteria**:
- [ ] A constant nomination emits exactly one `loaded` record with a `slug` attr and **no** `slot` attr.
- [ ] A pair emits exactly two `loaded` records, one `slot=light` and one `slot=dark`, each with its own `slug`.
- [ ] A failed light slot emits one WARN `fallback applied` carrying the **failed** slug, `slot=light` and the reason, followed by one INFO `loaded` carrying `tokyo-night-day`.
- [ ] Both `loaded` and `fallback applied` name different slugs in that case — asserted explicitly, since naming the same slug is the failure the rule exists to prevent.
- [ ] Five successive resolutions of the same broken slug emit **one** `fallback applied` and **five** `loaded` records (dedup applies to the WARN only).
- [ ] The same failed slug with a *different* reason emits a second `fallback applied`.
- [ ] `loaded` is INFO, `fallback applied` is WARN, and both carry component `theme`.
- [ ] Every attr key used is drawn from the closed seven; no `count` or `rejected` attr appears on either event.
- [ ] `NewEventLogger(log.Discard())` produces zero records for a full resolution including fallbacks; a nil logger does not panic.
- [ ] Two separately constructed `EventLogger`s each emit their own first `fallback applied` — dedup state is per instance, not package state.

**Tests**:
- `"it emits one loaded per nomination"` — `TestEvents_LoadedOncePerNomination` (constant → 1, pair → 2)
- `"it carries slot only under a pair"` — `TestEvents_SlotAttrOnlyUnderAPair`
- `"it emits loaded for the fallback's slug"` — `TestEvents_LoadedNamesTheFallbackSlug`
- `"it names the failed slug on the fallback warning"` — `TestEvents_FallbackAppliedNamesTheFailedSlug`
- `"it dedups fallback applied on slug and reason"` — `TestEvents_FallbackAppliedDedupsOnSlugAndReason`
- `"it emits twice for the same slug with a different reason"` — `TestEvents_FallbackDifferentReasonEmitsTwice`
- `"it does not dedup loaded"` — `TestEvents_LoadedIsNotDeduplicated`
- `"it emits loaded at INFO and fallback at WARN"` — `TestEvents_LevelsAreLoadedInfoFallbackWarn`
- `"it uses only the closed attr-key set"` — `TestEvents_AttrKeysAreInTheClosedSet`
- `"it emits nothing under the discard logger"` — `TestEvents_DiscardSilencesResolution`
- `"it keeps dedup state per instance"` — `TestEvents_FreshInstanceHasFreshDedupState`

**Edge Cases**:
- **One `theme: loaded` per nomination** — one under a constant, two under a pair — never one combined line, so `slug` and `slot` stay single-valued and a `grep "theme:"` answers per theme.
- `slot` is carried only under a pair and is **absent** under a constant.
- When a nomination is unloadable, `theme: loaded` fires **for the fallback too**, carrying the *fallback's* slug — otherwise both events name the slug that failed and a grep on a broken install cannot answer which palette is actually rendering.
- `theme: fallback applied` is **WARN** with `slug` / `slot` / `reason`, **deduplicated per process on `slug`+`reason`** — a persistently broken theme re-resolves at construction, again on every panel open and again on every `Esc`, and "per fallback" read literally would turn a passive forensic trail into running commentary.
- `theme: loaded` is **INFO and not deduplicated** — it is per-load, and Phase 9's commit-time load is the same event at a different cadence.
- No count and no enumeration attr, because nothing is enumerated at construction.
- Attrs stay inside the closed key set; no key is invented at a call site.
- Emission is controlled by the **injected logger**, so `log.Discard` on doctor, export and `capturetool` silences it entirely and leaves their dedup state owned rather than dangling.
- The dedup state lives on the instance and is shared by every path in a TUI process, which is what stops the construction-time read and the panel's enumeration double-reporting the same condition.
- The component records where a theme is *used*, never where one is *diagnosed*.

**Context**:
> §12.3, `theme: loaded`: "At TUI construction, **one line per nominated theme** — one under a constant, two under an adaptive pair — each carrying `slug` and, for the pair, `slot`. Resolved slug(s) only; **no count**… One line per nomination rather than one combined line keeps `slug`/`slot` single-valued, which is what makes the log greppable per theme. **Also fires at commit time**… **When a nomination is unloadable it fires for the fallback too**, carrying the fallback's slug — otherwise `theme: fallback applied` and `theme: loaded` both name the slug that *failed*, and a `grep "theme:"` on a broken install cannot answer which palette is actually rendering."
> §12.3, `theme: fallback applied`: "WARN. Carries `slug` (the nomination that failed), `slot` where one applies, and `reason` — without them the line is not greppable, which is the whole reason the log earns its place. **Deduplicated per process on `slug`+`reason`**."
> §12.3: "**Emission is controlled by an injected logger, not by the loader deciding.**… **The per-process dedup state lives on that injected logger**, so it is shared by every path in a TUI process… It is not package state in the leaf, and a test controls it by injecting a fresh one."
> §12.3: "**Why the log earns its place:** a TUI launch that rejects a theme should leave a **passive** record. The panel's row is only visible if the panel is opened; doctor must be invoked."
> §12.3: attr keys are the closed set `slug`, `slot`, `reason`, `path`, `token`, `count`, `rejected`; the component is a spec-governed amendment to the closed vocabulary (17 → 18), CLAUDE.md's count corrected in **Phase 10**.
> Phase boundary: `theme: enumerated` is Phase 8; `theme: appearance migrated` and `theme: commit failed` are Phase 6 and Phase 9's persister.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §12.3, §8.5, §6.3, §5.5

## theming-system-5-6

### Task 5.6: An unresolvable fallback is a fatal one-line message, never a panic

**Problem**: §7.6 deliberately removed the safety net beneath the fallback — there is **no runtime last-resort hardcoded palette**, replaced by a build-time guarantee. That makes one state genuinely fatal: if the theme a slot falls back to cannot itself be loaded from the embedded set (a built-in file renamed in a later PR, a typo'd `DefaultDarkSlug`, a corrupt embed), Portal has nothing to paint and no honest choice left. Handled badly it becomes a nil-`Theme` render (a silently colourless picker), an infinite fallback loop, or a Go panic trace where a sentence belongs. Handled at the wrong layer it lands in `internal/theme`, which §7.6 explicitly keeps out of the escalation business — the loader returns an **ordinary error** for an embedded parse failure, and the escalation happens *where the fallback is needed*.

**Solution**: Single-source §14A's pinned sentence in `internal/theme`, return it as an ordinary error from task 5-4's resolution when a **fallback** cannot resolve, let it travel up the normal `cmd` error path to one printed line, and add the minimal injectable built-in source that makes the otherwise-unreachable path testable.

**Outcome**: A binary whose embedded set cannot supply its fallback prints `built-in theme tokyo-night is missing or invalid — this binary is broken` and exits non-zero through `main`'s single exit owner — no panic trace, no colourless limp-on, and no runtime crutch palette added anywhere.

**Do**:
- Add to `internal/theme` a single-sourced message, e.g. `const brokenBuiltinFormat = "built-in theme %s is missing or invalid — this binary is broken"` with `func BrokenBuiltinError(slug string) error`. Copy the sentence **verbatim** from §14A, em dash included, and comment that it is pinned copy on a path the build-time guarantee makes unreachable — terse is right, but it is still new user-facing copy and is single-sourced rather than left implicit.
- Return it from task 5-4's `ResolveNomination` when the **fallback** resolution fails — never when a *nomination* fails (that is absorbed silently by the fallback). Distinguish the two explicitly in-source: the failure is fatal only because a *fallback* is what is missing.
- Never fall back a second time and never substitute a compiled-in palette. Add a comment recording that §7.6 rejected "a compiled-in last-resort palette equal to Tokyo Night Dark" — "a build-time guarantee beats a runtime crutch" — so this path must not grow one later.
- Add the seam that makes the path exercisable: an injectable built-in byte source on the `Loader` (a `func(slug string) ([]byte, bool)` field defaulting to Phase 2's `BuiltinBytes`, alongside the already-injectable reserved-slug set and event logger). A test injects a source that omits or corrupts a fallback slug. Document that the seam exists **because** the path is unreachable in a correctly built binary, and that it costs nothing in production.
- Let the error travel the ordinary path: `cmd/open.go`'s `openTUI` returns it, `Execute` writes the one line, `main.go` owns the single `os.Exit`. **No bare `os.Exit` outside `main`**, no `log.Fatal`, no panic. `main.go`'s panic-recovering exit and its `process: panic` lifecycle marker stay the backstop for a genuine programming fault rather than the designed route.
- Do **not** add startup-eager validation: nothing walks the embedded set at init or at launch. §7.6's build-time test already proves the set, and re-proving it on every launch buys cost on the one cold path this feature otherwise adds nothing to. Pin this as a negative assertion (a counting built-in source proves at most two built-in reads occur for a pair, at most one for a constant).
- Emit no new event: this is a fatal returned to the user, not a `theme` component line. (`theme: fallback applied` may already have fired for the nomination that failed; nothing further is logged for the fatal.)

**Acceptance Criteria**:
- [ ] With an injected built-in source that omits `DefaultDarkSlug`, resolving a constant whose nomination is broken returns an error whose message is **exactly** `built-in theme tokyo-night is missing or invalid — this binary is broken`.
- [ ] The same holds for `DefaultLightSlug` on a broken light slot, naming `tokyo-night-day`.
- [ ] With an injected source whose fallback file is *corrupt* rather than absent, the same sentence is produced — the message does not vary by reason.
- [ ] The message is asserted against the exported single source **and** against the literal §14A string, so a drift in either fails.
- [ ] The failure is an ordinary `error`: no panic, no `os.Exit` inside `internal/theme` or `cmd`, no `log.Fatal` — proven by a source guard plus a test that recovers nothing.
- [ ] `openTUI` returns the error and constructs **no** TUI on that path; `Execute` prints one line and the process exits non-zero via `main`.
- [ ] A *nomination* failure with a healthy fallback is **not** fatal — it returns a valid `Resolution` with `FellBack=true` and no error.
- [ ] Nothing walks the embedded set at startup: a counting built-in source records at most one read under a constant and two under a pair, and zero on the exec path.
- [ ] No compiled-in fallback palette exists anywhere in the tree — a guard test asserts `internal/theme` declares no `Theme` literal outside tests.
- [ ] The seam defaults to `BuiltinBytes` in production, so nothing at a call site has to pass it.

**Tests**:
- `"it returns the pinned fatal for a missing built-in fallback"` — `TestFallback_MissingBuiltinIsFatal` (table: dark constant, light slot, dark slot)
- `"it returns the same message for a corrupt built-in fallback"` — `TestFallback_CorruptBuiltinIsFatal`
- `"it matches the pinned copy verbatim"` — `TestBrokenBuiltinError_CopyIsPinned`
- `"it returns an error rather than panicking"` — `TestFallback_NeverPanics`
- `"it constructs no TUI on the fatal path"` — `TestOpenTUI_FatalBeforeModelConstruction`
- `"it is not fatal when only the nomination fails"` — `TestFallback_NominationFailureIsNotFatal`
- `"it reads no built-in beyond the nominated ones"` — `TestResolution_NoStartupEagerValidation` (counting source)
- `"it declares no runtime last-resort palette"` — `TestTheme_NoCompiledInFallbackPalette`
- `"it keeps os.Exit with main"` — existing bare-`os.Exit` source guard, extended to the new code

**Edge Cases**:
- §14A's copy pinned **verbatim** — `built-in theme <slug> is missing or invalid — this binary is broken` — terse being right for a path the build-time guarantee makes unreachable, but still new copy and single-sourced rather than left implicit.
- It is an **ordinary error returned up the normal path**, so the user sees one line rather than a Go panic trace; `main.go`'s panic-recovering exit and its `process: panic` marker stay the backstop for a genuine programming fault rather than the designed route.
- The loader is unchanged — it still returns an ordinary rejection for an embedded parse failure, because the escalation belongs **where the fallback is needed** and not in `internal/theme`'s parse layer (§7.6 explicitly left it out of Phase 2).
- **Nothing walks the embedded set at startup** — a negative assertion, since §7.6's build-time test already proves the set and re-proving it on every launch buys cost on the one cold path this feature otherwise adds nothing to.
- There is **no runtime last-resort hardcoded palette** beneath the fallback, a build-time guarantee having deliberately replaced the runtime crutch, so this path must not grow one.
- The path is unreachable in a correctly built binary, which is exactly why it needs a seam (an injected built-in byte source) to be exercised at all.
- Exit stays with `main.go`'s single owner — no bare `os.Exit` outside `main`.
- The failure is fatal only because a *fallback* is what is missing, so it must not be confused with a *nomination* failing, which task 5-4 absorbs silently.

**Context**:
> §7.6: "There is **no runtime fallback to hardcoded values** beneath the built-in fallback. Instead the situation is made impossible at build time… **Mechanism:** the loader returns an ordinary error for an embedded parse failure — it does not panic. The escalation happens where the fallback is *needed*: a fallback that cannot resolve is a fatal error returned up the normal path, so the user sees a one-line message rather than a Go panic trace. `main.go`'s panic-recovering exit and its `process: panic` lifecycle marker remain the backstop for a genuine programming fault, not the designed route. **Validation is not startup-eager** — nothing walks the embedded set at init."
> §7.6: "With no path pretending to handle it, a binary somehow shipped with a broken default fails **loudly at startup** rather than limping on values nobody chose." Rejected: "a compiled-in last-resort palette equal to Tokyo Night Dark. A build-time guarantee beats a runtime crutch."
> §14A: "**Fatal startup message (§7.6)**, on the should-never-happen path where a fallback slug does not resolve within the embedded set: `built-in theme <slug> is missing or invalid — this binary is broken`. Terse is right for a path the build-time guarantee makes unreachable, but it is still new copy and is pinned rather than left implicit."
> CLAUDE.md: "`main.go` … owns the single `os.Exit` via a panic-recovering exit shape — **bare `os.Exit` outside `main` is prohibited**."
> Phase boundary: Phase 2 task 2-8 proved the embedded set and the fallback slugs resolve **at build time**; this task is the runtime escalation that build-time guarantee is paired with, deliberately deferred out of Phase 2 because escalation belongs where the fallback is needed.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §7.6, §8.5, §14A, §12.6

## theming-system-5-7

### Task 5.7: Construction loads the nominated themes and hands `tui.Build` the nomination

**Problem**: Everything Phase 5 built is inert. `cmd/open.go` still reads `prefsStore.LoadAppearance()` and maps the legacy `auto|light|dark` enum onto a nomination in memory (Phase 3 task 3-2's deliberate bridge), so `prefs.json`'s `theme` / `theme_light` / `theme_dark` decide nothing, no themes directory is ever consulted, and the real startup win — a **constant** skipping detection and the first-paint wait — is reachable only through a legacy pin. Leaving the bridge alongside the new read would be worse than either: a dead read path is a second source of truth for which theme is active, exactly the divergence 3-2 avoided by deleting `WithAppearance` rather than keeping it.

**Solution**: Read the three keys once through the existing prefs store, derive the setting (5-2), resolve the nomination with fallback (5-3/5-4) against `themesDirPath()` and a real `theme` component logger, hand the result to `tui.Build`'s existing nomination slot, and delete `prefs.Appearance` and its API with its last caller.

**Outcome**: `"theme": "nord"` in `prefs.json` paints Nord from frame one with no detection; `"theme_dark": "nord"` paints Nord in a dark terminal and `tokyo-night-day` in a light one; a typo'd slug paints the shipped default while `prefs.json` stays byte-unchanged; and `portal open <target>` still constructs no TUI, reads no prefs and does no theme work.

**Do**:
- In `cmd/open.go`'s `openTUI`: replace the `appearance := prefs.AppearanceAuto; … LoadAppearance()` block with `keys, _ := prefsStore.LoadThemeKeys()` on the **same** `prefsStore` instance that already serves the initial mode and that Phase 6's persister will write through — prefs is read **once** per process. Keep the existing tolerant discard-the-error shape and its comment style: every degenerate case yields empty keys, which is the shipped pair.
- Derive and resolve: `setting, rawKeys := theme.ResolveSetting(keys.Theme, keys.Light, keys.Dark)`, then `res, err := loader.ResolveNomination(setting, themesDir)` where `loader` is constructed with `theme.NewEventLogger(log.For("theme"))` — the §12.3 assignment for a path where a theme is *used* — and `themesDir` comes from `themesDirPath()`.
- Handle the two failure shapes distinctly: `themesDirPath()` returning an error degrades to an **empty** directory string (task 5-3 turns that into `not found` per slug, so built-ins still resolve and a drop-in slug falls back) and must never block opening the TUI — the same tolerance the nil prefs store already has for the grouping mode. `ResolveNomination`'s error is task 5-6's fatal and is returned.
- Replace `tuiConfig.appearance` with the nomination and map it in `buildTUIModel` onto `tui.Deps.Theme` (Phase 3 task 3-2's field / `WithThemeNomination` option). Delete the `Appearance:` line from the `tui.Deps` literal.
- **Do not** thread `rawKeys` onto the model yet — Phase 8 owns the constructor slot and every consumer (badges, the `not found` / charset-rejected rows, §14A's confirm). Keep the value local (or discard it with a `_` plus a comment naming Phase 8) so nothing half-wires it.
- Delete `prefs.Appearance`, `parseAppearance`, `Appearance.String()`, the three `appearance*String` constants, `LoadAppearance` and `SaveAppearance` from `internal/prefs`, plus their tests. **Keep** task 5-1's raw `appearance` field on `prefsFile` — it still round-trips untouched through every write.
- Remove the last `prefs.Appearance` references in `internal/tui` / `internal/capture` / `cmd/capturetool` if any survive 3-2/3-4, and add a source guard asserting the identifier no longer appears in the tree.
- Prove the exec path is untouched: `portal open <target>` constructs no TUI, so it must never call `LoadThemeKeys`, `themesDirPath` or any loader entry point. Assert with a `logtest.Sink` (zero `theme` records) and a poisoned `PORTAL_THEMES_DIR`.
- Leave `cmd/capturetool` alone: it passes the constant shape from `--theme` and reads no prefs, so its frames stay byte-deterministic and `internal/capture`'s no-real-config import guard is untouched.

**Acceptance Criteria**:
- [ ] `"theme": "nord"` yields a constant nomination: the gate is resolved and unarmable at construction, `Init` issues no timeout tick, and the first frame paints Nord's canvas.
- [ ] `"theme_dark": "nord"` with no `theme` yields a pair; a dark gate answer selects Nord, a light answer selects `tokyo-night-day`, and a timeout selects Nord (the standing dark no-answer fallback).
- [ ] The gate resolves **exactly once**: a `tea.BackgroundColorMsg` arriving after the timeout still populates `originalBg` for `restore.go` and does not change the active theme.
- [ ] A hand-edited file carrying both `theme` and both slots renders the constant (the slots are never read, so a broken slot value cannot fail the launch).
- [ ] `"theme": "no-such-theme"` renders `tokyo-night`, emits `theme: fallback applied` + `theme: loaded tokyo-night`, and leaves `prefs.json` byte-identical.
- [ ] Directory-read budget: with `PORTAL_THEMES_DIR` holding 50 valid `.theme` files, a built-in constant reads the directory **zero** times, a drop-in constant once, a drop-in pair twice, and nothing calls `ReadDir`.
- [ ] `prefs.Appearance`, `parseAppearance`, `LoadAppearance` and `SaveAppearance` no longer exist anywhere; the raw `appearance` field still round-trips (task 5-1's tests unchanged and green).
- [ ] A `themesDirPath()` failure still opens the TUI on the shipped pair; a nil prefs store still opens the TUI on the shipped pair.
- [ ] Under `NO_COLOR` both members of a pair are still loaded, the gate is skipped, the dark member is selected, and `theme: loaded` fires twice as normal.
- [ ] The retained startup canvas hex (task 3-3) is still captured from the theme the gate **selected**, now for a nomination sourced from disk.
- [ ] `portal open <target>` performs no prefs read, no themes-directory access and emits zero `theme` records.
- [ ] `capturetool` still passes the constant shape and reads no prefs; every fixture renders byte-identically to its Phase 3 output.

**Tests**:
- `"it paints a persisted constant from frame one"` — `TestConstruction_PersistedConstantSkipsTheGate`
- `"it resolves a persisted pair through the gate"` — `TestConstruction_PersistedPairSelectsByGate` (dark reply, light reply, timeout)
- `"it consumes a late reply without re-theming"` — `TestConstruction_LateReplyNeverReThemes`
- `"it lets a persisted constant win over stale slots"` — `TestConstruction_ConstantWinsOverStaleSlots`
- `"it falls back and keeps the persisted name"` — `TestConstruction_UnloadableNominationFallsBackWithoutWriting` (byte-compare `prefs.json`)
- `"it reads only the nominated themes"` — `TestConstruction_ReadBudget` (50-file directory; counts opens; asserts no `ReadDir`)
- `"it degrades to the shipped pair when prefs or the themes dir cannot resolve"` — `TestConstruction_PathFailuresDegradeNotBlock`
- `"it loads both members and selects dark under NO_COLOR"` — `TestConstruction_NoColorLoadsBothSelectsDark`
- `"it captures the startup canvas hex from the selected member"` — `TestConstruction_StartupCanvasHexFromSelectedMember`
- `"it deletes the appearance enum with its last caller"` — `TestPrefs_AppearanceAPIIsGone` (source guard over the tree)
- `"it does no theme work on the exec path"` — `TestOpenExecPath_DoesNoThemeWork` (poisoned themes dir + `logtest.Sink`)
- `"it emits the construction events once per nomination"` — `TestConstruction_EmitsLoadedPerNomination`

**Edge Cases**:
- Replaces task 3-2's in-memory `appearance` mapping **outright** — `prefs.Appearance`, `parseAppearance`, `LoadAppearance` and `SaveAppearance` are deleted with their last caller rather than left alongside (a dead read path is a second source of truth), while the raw `appearance` *field* from task 5-1 stays and round-trips untouched.
- A constant loads **one** theme and a pair **two**, never three and never an enumeration — with `PORTAL_THEMES_DIR` pointing at a directory of files, construction reads none of them for a built-in nomination.
- The gate is **skipped entirely** under a constant now that a constant can come from `prefs.json` — the real startup win, previously only reachable through an `appearance` pin — and **resolves exactly once** under a pair.
- A late OSC 11 reply is still consumed for `restore.go`'s original-background capture and for §9.3's later mid-session conversion, but never re-themes, because under split a late flip swaps a whole named theme rather than a variant.
- The retained startup canvas hex is still captured from the theme the gate **selected**, so task 3-3's anchor holds against a nomination that now comes from disk.
- Under `NO_COLOR` the machinery runs unchanged — both members are still loaded so a later commit has something to persist against, the gate is skipped, and the standing dark no-answer fallback selects the member.
- A prefs path-resolution failure must not block opening the TUI — it degrades to the shipped pair, matching the existing nil-store tolerance for the grouping mode; a `themesDirPath()` failure degrades to an empty directory, which resolves built-ins and yields `not found` for anything else.
- Prefs is read **once** per process through the same store instance that already serves the grouping mode and that Phase 6's persister will write through.
- `cmd` passes a **real** `theme` component logger here (the path where a theme is *used*) while doctor, export and `capturetool` keep `log.Discard`.
- `capturetool` still passes the constant shape and reads no prefs, so its frames stay byte-deterministic and `internal/capture`'s no-real-config import guard is untouched.
- The control-stripped raw keys task 5-2 produces are deliberately **not** threaded onto the model yet — Phase 8 owns the constructor slot and every consumer of it.
- `portal open <target>` constructs no TUI, reads no prefs and does no theme work, proven with a poisoned themes directory and a `logtest.Sink`.

**Context**:
> §8.4: "**At construction Portal loads every *nominated* theme — at most two.** The light/dark gate then only **selects** between values already in hand… **Cold-path cost:** one file read for a constant, two for a pair. No file read on the critical path, no flip."
> §8.8: a constant user "needs no detection, so their first paint is immediate — a real startup win"; the adaptive user keeps the ~50ms race and the **dark** no-answer fallback. "**The gate resolves exactly once.** … The reply is still *consumed* — `restore.go` needs it for the original-background capture, and §9.3 needs it in hand for a mid-session conversion — but it never flips the active theme." The `prefs.Appearance` **enum and its API** die; the on-disk field does not.
> §9.10: "**Under `NO_COLOR` the theme machinery still runs normally, unchanged.**… **Both nominated themes are still loaded** at construction… **The gate is skipped**… **The startup canvas hex is captured as normal** from the selected member."
> §12.3: "The exec path is not a surfacing route… under lazy discovery the loader **never runs there**… **And a win worth recording explicitly: on the path Portal is most careful to keep free of cost, this feature adds nothing at all.**"
> §10.5: "**The migration therefore runs only where a TUI is constructed** — which is also the only place its result is used, since the exec path constructs no TUI and reads no prefs." Phase 6 adds that migration at this same call site (`loadPrefsStore`), which is why prefs is read through one store instance here.
> **Phase boundary flagged**: deleting `LoadAppearance` here means that, for the single phase between this task and Phase 6's marker-gated translation, an install carrying `"appearance": "dark"` and no theme keys renders the shipped **adaptive pair** rather than a pinned dark — §10.1's silent flip, briefly reintroduced. This is a direct consequence of the plan's own ordering (5-7 deletes the enum with its last caller; the translation is Phase 6) and is safe to carry because task 5-1 preserves the `appearance` value on disk untouched, so Phase 6 translates it exactly. Do not paper over it with an interim in-memory mapping — that would recreate the second source of truth this task exists to remove.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §8.4, §8.5, §8.8, §9.10, §10.5, §12.3, §13.3
