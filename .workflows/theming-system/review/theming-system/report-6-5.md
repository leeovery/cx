TASK: theming-system-6-5 — Split The Prefs Read And Compute The Marker-Gated `appearance` Translation In Memory

ACCEPTANCE CRITERIA (from plan):
- `{"appearance":"dark"}`, no theme keys, no marker → `Keys = {Theme: tokyo-night}`, `TranslationPending=true`, `TranslatedSlug=tokyo-night`; `prefs.json` byte-identical afterwards.
- `{"appearance":"light"}` → `tokyo-night-day`; `auto` / absent / `sepia` → `TranslatedSlug=""` with `TranslationPending` still true.
- `"Dark"`, `" dark"`, `"DARK"` translate to nothing (exact-match rule).
- Marker `true` → no mapping at all: `TranslationPending=false`, `Keys` exactly as read.
- `appearance: dark` plus any theme key set → `Keys` unchanged, `TranslatedSlug` still populated.
- Loss-of-setting sequence closed: `{"appearance":"dark","theme_dark":"nord"}` with no marker renders Nord this launch.
- Absent `prefs.json` → zero keys, `TranslatedSlug=""`, creates nothing.
- Corrupt `prefs.json` → zero keys, no error, does not block TUI construction.
- Both slugs come from `theme.DefaultDarkSlug` / `theme.DefaultLightSlug`.
- `loadPrefsStoreNoMigrate()` reads nothing beyond `prefs.NewStore` and computes no translation.
- The migrating loader has exactly one production caller (`openTUI`), asserted by a source guard over `cmd`.
- `portal open <target>` still reads no prefs and does no theme work.

STATUS: complete

SPEC CONTEXT:
§10.1 names the failure this task closes: deleting `prefs.Appearance` (task 5-7) makes a pinned `appearance` unknown, so a user who pinned `dark` silently upgrades into the shipped adaptive pair. §10.2 fixes the mapping as exact (`dark` → `theme: tokyo-night`, `light` → `theme: tokyo-night-day`, `auto`/absent → nothing). §10.3 makes the trigger an explicit `theme_migrated` marker — never the absence of theme keys, which is re-armable — and separates that trigger from a no-op condition on the theme keys. §10.5 puts ownership in `cmd/config.go`'s `loadPrefsStore` (`prefs` is a leaf that must not import `internal/log`), requires a non-migrating variant for doctor's read-only path (§12.2 line 1454), separates computing from persisting, and states that §10.3's no-op condition governs the in-memory half too, "evaluated against the load-time snapshot" — checked twice against two reads deliberately (load, then the RMW re-read). §8.4 pins that "as read" means the post-translation in-memory value, because the panel's badges and the `d`/`l` confirm are driven off those keys.

Note on phase supersession: this task specified "write nothing"; plan task 6-6 deliberately folded the persist into the same loader (`persistTranslation` → `runTranslationPersist` → `store.SaveTranslation`). The byte-identical criterion is therefore judged against the amended intent — the load computes and applies in memory, the write is dispatched non-blocking off the launch path.

IMPLEMENTATION:
- Status: Implemented (as amended by task 6-6)
- Location:
  - `cmd/config.go:91-99` — `loadPrefsStoreNoMigrate()` is today's unchanged body (`prefsFilePath` + `prefs.NewStore`) with the "must stay inert" contract comment.
  - `cmd/config.go:101-110` — `prefsLoad{Store, Keys, TranslationPending, TranslatedSlug}` with the "post-translation in-memory value" comment.
  - `cmd/config.go:117-152` — `loadPrefsStore()`: path error returned unchanged; tolerant `LoadThemeKeys` / `LoadMigrationState` with discarded errors; marker gate returns early with keys as read; otherwise `TranslationPending=true`, `TranslatedSlug=translateAppearance(...)`, and the in-memory application gated on the load-time snapshot (`TranslatedSlug != "" && all three keys empty`); `TranslatedSlug` deliberately not zeroed.
  - `cmd/config.go:176-185` — `translateAppearance` switches on the exact tokens and returns `theme.DefaultDarkSlug` / `theme.DefaultLightSlug`; no string literals.
  - `cmd/open.go:599-611` — `openTUI` degrades a load error to `prefsLoad{}`, keeps the single `*prefs.Store` for the initial mode read (`:608`) and the theme persister (`:660-663`), and hands `loadedPrefs.Keys` (post-translation) to `themeResolution`, which produces both the nomination and the panel's raw keys (`cmd/open.go:500-512`) — §8.4 satisfied end to end.
  - `cmd/doctor.go:resolveDoctorDeps` is the only production caller of `loadPrefsStoreNoMigrate` (pinned by `cmd/doctor_persisted_theme_test.go:545-576`).
- Notes: every ordering step in the task's "Do" list is present and in order. `state.Migrated` short-circuits before any translation, so a migrated file with a hand-deleted key stays on the shipped pair (§10.3's escape hatch). The mapping constants resolve to `tokyo-night` / `tokyo-night-day` (`internal/theme/builtins.go:13-14`), matching §10.2's table.

TESTS:
- Status: Adequate
- Coverage: `cmd/prefs_translation_test.go` carries every test the plan named, one-to-one: `TestLoadPrefsStore_TranslatesDark` (:43), `_TranslatesLight` (:51), `_NoTranslationCases` (:77, table incl. `sepia`, asserting the marker still pending), `TestTranslateAppearance_ExactMatchOnly` (:310, incl. `Dark`, `" dark"`, `DARK`, `dark\n`), `_MarkerGatesTheTranslation` (:96, both the key-set and the keys-deleted arms), `_ExistingKeySuppressesTheInMemoryValue` (:116, table over `theme`/`theme_light`/`theme_dark`, asserting `TranslatedSlug` survives), `_HandEditedSlotWinsOnTheTranslatingLaunch` (:137, asserted at the nomination so the reachable loss-of-setting sequence is closed end to end), `_ComputesWithoutWriting` (:147, byte-compare plus absent-stays-absent), `TestTranslateAppearance_UsesSharedConstants` (:322, with a guard that the two slugs differ and each names a real built-in), `_TolerantOnDegenerateFiles` (:176, absent/corrupt/empty/array, each also asserted at the nomination), `TestLoadPrefsStoreNoMigrate_ComputesAndWritesNothing` (:196, behavioural arm plus an AST arm pinning the closed call set), `TestLoadPrefsStore_SingleProductionCaller` (:287, production-only source walk — `PackageGoFiles(".", false)` excludes `_test.go`). The exec-path criterion is covered by `cmd/prefs_translation_persist_test.go:403` (`TestOpenExecPath_NoTranslation`: zero dispatches, prefs bytes unchanged, no theme events).
- The extra `TestLoadPrefsStore_TranslatedPinRendersAsAConstantThisLaunch` (:62) is not redundant — it is the §8.4 assertion that the translated keys reach the nomination as a *constant* (detection stays off), which none of the field-level assertions cover.
- Determinism: `cmd/testmain_isolation_test.go:24` neutralises `persistTranslation` package-wide, so the byte-identical assertions are not racing task 6-6's goroutine. Worth stating in the test itself (see notes).
- Notes: no over-testing found — `assertLoad` collapses what would otherwise be three assertions per case, and the tables carry one row per distinct rule rather than per permutation.

CODE QUALITY:
- Project conventions: Followed. Ownership sits in `cmd/config.go` so `prefs` stays a leaf with no `internal/log` import; the `theme` component keeps its single `cmd`-side binding; the non-migrating variant keeps doctor's read-only claim literal, structurally pinned by a source guard rather than by discipline.
- SOLID principles: Good. `translateAppearance` is a pure single-sourced mapping; the loader composes the non-migrating read rather than duplicating it; nothing about persistence leaks into the translation.
- Complexity: Low. One early return, one guarded assignment, one three-arm switch.
- Modern idioms: Yes. Deliberate `_`-discarded tolerant reads mirror the existing initial-mode read; constants over literals throughout.
- Readability: Good. Comments explain *why* (marker-not-absence, load-time snapshot, unzeroed `TranslatedSlug`, exact match) and carry no spec-section or task references. All comment claims hold against the code.
- Issues: none blocking. Two small ones below (a local named `state`, and a three-field emptiness test that will not notice a fourth key).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [bug] cmd/config.go:123-124 — `LoadThemeKeys()` and `LoadMigrationState()` each perform their own `os.ReadFile` (`internal/prefs/store.go:191,201`), so §10.5's "load-time snapshot" is in fact two snapshots. Between the two reads another instance's `SaveTranslation` can land, yielding `keys={}` with `Migrated=true`; the marker gate then returns zero keys and that launch renders the shipped adaptive pair instead of the pin just translated — §10.1's silent flip, narrow (two simultaneous cold launches) and self-correcting on the next launch, but reachable. Fix: add one combined prefs read (e.g. `LoadThemeState() (ThemeKeys, MigrationState, error)` off a single `readFile`) and call it once in `loadPrefsStore`; it also removes one of the three prefs file reads the launch path currently performs.
- [quickfix] cmd/config.go:141 — replace `keys.Theme == "" && keys.Light == "" && keys.Dark == ""` with `keys == (prefs.ThemeKeys{})`. `ThemeKeys` is comparable and the suite already uses that form (`cmd/prefs_translation_test.go:228`); the enumerated form silently stops covering the no-op condition if a fourth key is ever added to the struct.
- [do-now] cmd/config.go:108 — add a comment recording that `TranslationPending` has no production reader (grep confirms: written at `:135`, read only from tests): the persist is dispatched inside the loader itself, so this field is the loader's observable "the marker is still owed" contract for the suite. Without the note the next reader hunts for a consumer that does not exist.
- [do-now] cmd/config.go:124 — rename the local `state` (a `prefs.MigrationState`) to `migration`. `state` is the name of `internal/state`, imported across the `cmd` package; the file does not import it today, so this is currently only a readability snag, but it becomes a shadow the moment it does.
- [do-now] cmd/prefs_translation_test.go:147 — add a comment on `TestLoadPrefsStore_ComputesWithoutWriting` stating that `TestMain` neutralises `persistTranslation` package-wide, so the byte-identical assertion covers the loader's *own* writes; the dispatched persist is covered by `TestLoadPrefsStore_PersistIsNonBlocking`. As written the subtest name ("a translating launch leaves the file byte-identical") reads as a claim about the shipped behaviour, which since task 6-6 writes asynchronously.
