TASK: theming-system-15-1 — Drive Doctor's Theme Block Off One Enumeration And Off theme.Slot's Own Labels

ACCEPTANCE CRITERIA:
- A doctor run over a themes directory parses each candidate file exactly once; no second `ReadDir` or per-slug file open after the enumeration.
- Doctor's file line and persisted line for the same slug derive from the same parsed entry.
- `internal/theme` exposes one documented enumeration-backed by-name resolver; `resolveFromEnumeration` has no second implementation.
- `persistedThemeSlotLabel` contains no `case theme.SlotLight` / `case theme.SlotDark` arms; `themeSlotLight` / `themeSlotDark` are gone.
- Doctor's rendered output byte-identical for: usable dir with valid+invalid files, unreadable dir, missing persisted slug, charset-illegal slug, `reserved name` slug, constant setting.
- `go test ./...` and `go test -tags integration -p 1 ./cmd` pass.

STATUS: complete

SPEC CONTEXT:
§14A pins doctor's theme copy: the file frame `⚠ theme <slug>: <reason> — <detail>`, the two `bad name` frames, the `reserved name` sentence, the persisted frame `⚠ theme <slug> (<slot>) does not resolve: <reason>` (slot renders `light`/`dark`, `both` for one slug in both slots, omitted under a constant), and `⚠ themes directory unreadable: <path>`. §6.2/§5.5 fix the reason vocabulary: an unreachable theme under an unusable directory is `unreadable`, not `not found`; an absent directory is silent. §12.2/§12.3 make doctor read-only and emission-free (the `theme` component records use, never diagnosis) — which is why doctor's loader is `NewSilentLoader`. The spec constrains the rendered lines and the vocabulary, not how many times the directory is read; this task's one-enumeration property is an architecture/duplication remediation on top of that contract.

IMPLEMENTATION:
- Status: Implemented (mechanism partly superseded by a later phase, intent preserved)
- Location:
  - `/Users/leeovery/Code/portal/cmd/doctor_theme.go:51-56` — `themeAdvisoryUnion` takes the enumeration once via `loader.OpenEnumeration(deps.ThemesDir)` and hands the same value to both producers; the single shared `theme.NewSilentLoader()` and its owned dedup scope are unchanged.
  - `/Users/leeovery/Code/portal/cmd/doctor_theme.go:136-147` — `persistedThemeAdvisory` now resolves through `loader.ResolveByNameFrom(enumeration, slug)`; the `themesDir` argument is gone from that path rather than left unread.
  - `/Users/leeovery/Code/portal/cmd/doctor_theme.go:158-170` — `scanThemesDirectory(enumeration)` reads `DirUnusable` / `DirPath` / `Entries` off the retained enumeration; it opens nothing.
  - `/Users/leeovery/Code/portal/cmd/doctor_theme.go:125-132` — `persistedThemeSlotLabel` is the `Both` special case plus `name, _ := key.Slot.AttrName(); return name`; `themeSlotBoth` is retained as doctor's own label (correct — no slot can name it).
  - `/Users/leeovery/Code/portal/internal/theme/resolution.go:116-139` — `ResolveByNameFrom` is the exported enumeration-backed resolver, documented as `ResolveByName`'s ladder with no I/O and no events; the unexported `resolveFromEnumeration` is gone and `enumerationLoad` (resolution.go:112-114) now routes through the exported one, so `entryResult` is the single body.
  - `/Users/leeovery/Code/portal/internal/theme/theme_test.go:181` — the new export is enrolled in the package's exported-surface guard, the project's convention for a deliberate export.
- Notes:
  - The task's step 2 called for a `cmd`-local `enumerateThemesDir` helper. A later phase moved that dir-classification into `internal/theme` as `Loader.OpenEnumeration` (`internal/theme/enumerate.go:54-65`), which the panel's `Assembler.Open` also uses (`internal/theme/union.go:118`). That is a deliberate later supersession, not drift — it strengthens the outcome (doctor and the panel now classify one directory identically, pinned by `cmd/doctor_theme_enumeration_test.go:11`).
  - Rendered-line parity checked path by path against the old `ResolveByName(slug, themesDir)` route: charset failure → `badName(BadNameSlug)` in the shared `resolveNamed`; built-in / `reserved name` → the embedded-set rung answers first in both; a present-but-invalid file → the same `LoadFile` rejection, since `classify` builds the entry with `LoadFile`; a missing slug → `unresolvedRejection` yields `not found`, or `unreadable` when `DirUnusable` (matching the old `statThemeDir` failure). Doctor renders `rejection.Reason` only on the persisted frame, so the dropped `Detail`/`Err` on the unusable-directory rejection changes no byte. Empty `ThemesDir` (unresolved path) still yields the empty enumeration → no directory line, and the persisted producer still answers `not found` off the embedded-set rung.
  - The one behaviour delta outside the AC's enumerated cases is on case-insensitive filesystems (macOS default): a mis-cased file such as `Zed-Lee.theme` used to satisfy a persisted `zed-lee` through path composition (`os.ReadFile(dir/zed-lee.theme)` succeeding), so no persisted line was emitted; resolving off the enumeration, that file carries `bad name` with an empty slug and the persisted slug is now `not found`. This is a convergence with the panel (`persistedRows`/`listedUnder` already behaved this way) and with `enumerate.go:69-73`'s stated stance that a mis-cased name must be visibly rejected rather than silently honoured — an improvement, but currently unpinned by a test.
  - Step 6 honoured: `assembleThemeAdvisories`, `persistedSlugs`' `fromPrefs`-keyed membership and the pinned region order are untouched; the union's opposite survivor rule in `internal/theme/union.go` was not shared into `cmd`.

TESTS:
- Status: Adequate
- Coverage:
  - `cmd/doctor_persisted_theme_test.go:660-671` (`TestThemeAdvisories_DirectoryIsReadOnce`) — AST guard: exactly one `OpenEnumeration` call in `doctor_theme.go` and zero `Enumerate` / `ResolveByName` / `LoadFile` / `ReadDir` / `ReadFile` / `Open` calls.
  - `cmd/doctor_theme_union_test.go:102-138` (`TestThemeAdvisoryUnion_OneParseBehindBothProducers`) — the behavioural half of the same claim: the enumeration is taken, then the file is rewritten valid / the directory removed, and both producers still describe the original parse. This is the requested "content differs between two reads" test and it would fail loudly if the persisted producer re-read.
  - `internal/theme/resolution_test.go:954-982` (`TestResolveByNameFrom_MatchesResolveByName`) — the requested five-case pin against `ResolveByName` (valid file, rejected file, missing slug, built-in, charset-illegal), with a guard that the non-built-in cases are genuinely not in the embedded set.
  - `internal/theme/resolution_test.go:984-1016` (`TestResolveByNameFrom_ReadsNothing`) — resolves a drop-in after its directory is deleted, plus a reachability walk asserting zero `os.*` call sites, with a vacuity check on `ResolveByName`'s own `os.ReadFile`.
  - `cmd/doctor_persisted_theme_test.go:410-446` (`TestPersistedThemeSlotLabel_ReadsTheSlotsOwnName`) — the label equals `Slot.AttrName()` for all three slots plus the `both` case, no `light`/`dark` literal in `doctor_theme.go`, and an AST guard that the file names no `theme.Slot*` selector at all. That guard is the durable form of the plan's temporary "add a fourth Slot" experiment.
  - Existing advisory tests carry unchanged: `reserved name` non-collision (`doctor_theme_union_test.go:166`), `bad name` no-slug (`:140`), directory-line never deduped (`:204`), region order + 10-run byte-identical render (`:264-297`), advisory count vs rendered lines (`:350`).
- Notes:
  - The read-once assertion is structural (a call-site count in one file) rather than a runtime open counter, so a reader introduced in a *different* `cmd` file and called from here would slip past it. The behavioural test above covers the property that matters, and `deps.ThemesDir` has no other production consumer (`cmd/doctor.go:60,102,111` only sets it), so the gap is theoretical.
  - No redundancy found: the five resolver cases, the two disturbance fixtures and the label cases each pin a distinct claim.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays free of path resolution and logging decisions (the new resolver emits nothing, matching `ResolveByName`); doctor keeps the silent loader so the `theme` component still records use and not diagnosis (CLAUDE.md / spec §12.3); the new export is enrolled in the package's exported-surface guard; no test uses `t.Parallel()`.
- SOLID principles: Good. The enumeration becomes an explicit parameter of both producers (dependency made visible rather than each producer reaching for the filesystem); `entryResult` is the single lookup body behind three passes (`enumerationPass`, `commitPass`, doctor).
- Complexity: Low. `persistedThemeSlotLabel` drops from a four-arm switch to two lines; `scanThemesDirectory` loses its dir-resolution branch.
- Modern idioms: Yes.
- Readability: Good. Comments state the reasoning (one parse behind two lines, why resolution goes by name and never through `ResolveNomination`) without restating code.
- Issues: one inaccurate clause in the new resolver's doc comment (below). Nothing else.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] /Users/leeovery/Code/portal/internal/theme/resolution.go:118 — the clause "The Result carries no Source bytes — an enumeration retains parses, not files." is false on the ladder's first rung: a built-in slug resolves through `LoadBuiltin` → `resultFromBytes` (`internal/theme/builtins.go:71-79`, `load.go:59-66`), which sets `Source` to the embedded bytes. Replace the sentence with: "A drop-in resolved from the enumeration carries no Source bytes — an enumeration retains parses, not files; a built-in still carries its embedded source."
- [quickfix] /Users/leeovery/Code/portal/cmd/doctor_theme_union_test.go:140 — add a case to `TestThemeAdvisoryUnion_BadNameFileNeverCollides` staging `Zed-Lee.theme` with prefs `{"theme":"zed-lee"}` and asserting both the `⚠ theme file Zed-Lee.theme: slug must be lowercase letters, digits and hyphens` line and the `⚠ theme zed-lee does not resolve: not found` line. On a case-insensitive filesystem the pre-task path-composition read silently satisfied the persisted slug from the mis-cased file and emitted only the first line; resolving off the enumeration now emits both, matching the panel — the delta is desirable but currently unpinned.
