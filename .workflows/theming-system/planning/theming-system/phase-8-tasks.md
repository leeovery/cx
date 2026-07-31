# Phase 8: The slide-over panel — tasks 8-1 … 8-8

## theming-system-8-1

### Task 8.1: Assemble the §9.4 union behind the `ThemeEnumerator` seam

**Problem**: The panel must list *every* theme the user could plausibly be looking for — every `.theme` file in the themes directory, every built-in, and every slug named in `prefs.json` that resolves to neither — and it must do so as **one row per slug**. Keying that union on "has a file" instead of "resolves" mints a second `⚠ not found` row for every persisted built-in slug, which is the state the panel's single most common action produces (`Enter` on `tokyo-night`). The pieces all exist and nothing joins them: Phase 1 supplies directory entries only (`Loader.Enumerate`), Phase 2 supplies the embedded set, Phase 5 supplies the control-stripped raw persisted keys. There is also no site for `theme: enumerated`, whose `count` and `rejected` attrs are only computable where the merge happens — and if the merge lived in `internal/tui` that package would become a **fourth** emitter of the `theme` component, which §8.9 closes at three (loader, translation, persister). Finally, without an assembled-union seam the panel's row rules have no test home at all: `internal/capture`'s no-real-config import guard forbids the harness reaching a real themes directory, so an invalid-theme row could never be rendered offline.

**Solution**: A union assembler in `internal/theme` — one directory read producing a retained `Enumeration`, plus a pure derivation over that enumeration and the raw persisted keys producing the finished `Union` — exposed to the TUI through a `ThemeEnumerator` seam declared in `internal/tui` (the `TmuxEnumerator` / `ScrollbackReader` idiom the preview page already uses). The seam returns the finished row set, never a directory listing.

**Outcome**: `Open(keys)` returns an `Enumeration` (the retained parse results) plus a `Union` whose rows are files ∪ built-ins ∪ unresolvable persisted slugs — deduped one-slug-one-row, each carrying its single §6.2 reason, with the unusable-directory condition carried as a flag rather than a member — and emits exactly one `theme: enumerated`. `Reassemble(enumeration, keys)` re-derives the same union from changed keys with **no** fresh directory read and no event.

**Do**:
- Add `internal/theme/union.go` declaring:
  ```go
  type RowSource int // SourceBuiltin, SourceFile, SourcePersisted

  type Row struct {
      Slug      string      // "" exactly when the row yields no slug
      Filename  string      // set iff SourceFile
      Persisted string      // the raw persisted string, set iff SourcePersisted with no slug
      Source    RowSource
      Theme     Theme       // populated iff Rejection == nil
      Rejection *Rejection  // nil ⇒ valid and selectable
  }
  func (r Row) Selectable() bool { return r.Rejection == nil }

  type Enumeration struct {
      Entries     []Entry // Phase 1 task 1-7's classified directory entries
      DirUnusable bool
      DirPath     string
  }

  type Union struct {
      Rows        []Row
      DirUnusable bool // drives the pinned ⚠ dir unreadable chrome row (task 8-6)
      Count       int  // len(Rows) — what theme: enumerated reports
      Rejected    int  // rows carrying a Rejection
  }

  func (l Loader) Open(themesDir string, keys RawKeys) (Enumeration, Union)
  func (l Loader) Reassemble(e Enumeration, keys RawKeys) Union
  ```
- **`Open` performs exactly one directory read** — `l.Enumerate(themesDir)` — folds its `(entries, *Rejection)` into `Enumeration` (an `unreadable` rejection sets `DirUnusable` + `DirPath` and leaves `Entries` empty; an absent directory yields zero entries, `DirUnusable=false`, silently), then calls `Reassemble` and emits `theme: enumerated` once. `Reassemble` does **no** I/O of any kind: it is a pure function of `(Enumeration, RawKeys)` and is the entry point §9.2's post-commit recompute (Phase 9) and task 8-10's `Esc` re-resolution both re-call with changed prefs state.
- **Assemble in a fixed order** inside `Reassemble`:
  1. One row per embedded built-in (`SourceBuiltin`, always valid — Phase 2 task 2-8 proves that at build time).
  2. One row per enumeration entry (`SourceFile`), carrying the entry's `Theme` or its single `Rejection` unchanged. A file whose rejection is `reserved name` **also stands** — it is the one legitimate two-rows-for-one-slug case, because that collision *is* the reason's entire content — so it is never deduped against the built-in it collides with. Every *other* file slug is unique by construction (§5.6 mints no duplicate slug), so no other dedup arises between rows 1 and 2.
  3. The persisted keys **in force**: derive `Setting` by calling `ResolveSetting(keys.Theme, keys.Light, keys.Dark)` (idempotent on already-stripped input) so the `theme`-wins tiebreak is applied once, in one place, exactly as Phase 7's doctor line applies it. Under a constant only `keys.Theme` contributes; under a pair only the **non-empty** raw `keys.Light` / `keys.Dark` contribute (an unset slot holds the shipped default, which is a built-in and already has a row).
- **Per in-force persisted value**, in this order: if a row already exists whose `Slug` equals it, contribute **nothing** — that row *is* the persisted slug's row (this is the "resolves, not has a file" rule, and it is what stops `Enter` on `tokyo-night` minting a `⚠ not found` twin). Else if it fails `ValidSlug` (Phase 1 task 1-4), add a `SourcePersisted` row with `Persisted` set, `Slug` empty and reason **`bad name`** — never `not found`. Else add a `SourcePersisted` row with `Slug` set and reason **`unreadable`** when `DirUnusable`, otherwise **`not found`** (§5.5: `not found` sends the user to check the filename, `unreadable` sends them to check permissions, and permissions is the actual problem).
- **Dedup the two slots against each other**: both slots naming the same missing slug produce **one** row, not two.
- **Add `EventLogger.Enumerated(count, rejected int)`** in `internal/theme/events.go` — INFO, message `enumerated`, attrs `count` and `rejected` only, **not deduplicated** (it is a per-event INFO, one per open, so five opens legitimately emit five lines). Delete the "Phase 8" placeholder note task 1-8 left for it. It fires on an **absent** directory (count reflects the built-ins) and on an **unusable** one (count likewise, alongside the deduped `theme: directory unusable` that `Enumerate` already emits) — the panel opened either way, which is what the event records. `count` is `len(Rows)`; the `⚠ dir unreadable` chrome row is **not** a union member and is never counted.
- **Declare the seam** in a new `internal/tui/theme_seams.go`, doc-commented in the same register as `preview_seams.go`:
  ```go
  type ThemeEnumerator interface {
      Open(keys theme.RawKeys) (theme.Enumeration, theme.Union)
      Reassemble(e theme.Enumeration, keys theme.RawKeys) theme.Union
  }
  ```
  State in its doc comment that the seam returns the **finished union**, that assembly lives in `internal/theme` so `internal/tui` never emits the `theme` component, and that **task 8-8 extends this seam with a third method** (`Resolve`, the open-time re-resolution against a retained enumeration) — so an implementer does not design a closed two-method shape. Wiring the seam into `Deps` / `Build` / `cmd` is task 8-7; this task declares it and the `internal/theme` functions a production adapter wraps.
- Keep `Union` an **ordinary value** with exported fields so a fixture can fake one wholesale with no real directory (§13.3), and add no methods to it that read the filesystem.

**Acceptance Criteria**:
- [ ] A persisted `"theme": "tokyo-night"` against a directory containing no such file yields **one** row — the built-in's — and **no** `not found` row.
- [ ] A persisted slug naming an existing-but-invalid file yields **one** row: that file's, carrying its reason (the badge lands on it in task 8-3).
- [ ] A `nord.theme` drop-in beside the `nord` built-in yields **two** rows — the built-in (valid) and the file (`reserved name`) — and this is the only input that produces two rows for one slug.
- [ ] A persisted slug resolving to neither built-in nor file yields one unselectable row with reason `not found`; the same slug with an unusable directory yields `unreadable`.
- [ ] A persisted string failing the charset check yields one row with reason **`bad name`**, `Slug` empty, `Persisted` set.
- [ ] Under a **constant**, the two slot keys contribute nothing even when both are set to unresolvable slugs; under a **pair**, each non-empty slot contributes and both slots naming the same missing slug collapse to one row.
- [ ] An **absent** directory yields built-ins only, zero rejections, no error and no `directory unusable` record; an **unusable** one yields built-ins plus persisted rows with `Union.DirUnusable` true and the directory condition **absent from `Rows`**.
- [ ] `Count == len(Rows)` and `Rejected` counts exactly the rows with a non-nil `Rejection`; `theme: enumerated` carries those two values and no other attr.
- [ ] `Open` emits exactly one `theme: enumerated`; five successive `Open` calls over the same broken directory emit five `enumerated` lines but exactly one `theme: rejected` per distinct slug+reason (Phase 1's per-process dedup).
- [ ] `Reassemble` performs no file or directory access (proven with the directory removed between the `Open` and the `Reassemble`) and emits nothing.
- [ ] Built-in rows carry nothing that distinguishes them from a valid drop-in other than `Source`, which is consumed only by task 8-2's tie-break — no reason, no marker, no flag reaches the row's rendered content.
- [ ] `Loader` constructed with `log.Discard()` produces zero records for any sequence of `Open` calls.

**Tests**:
- `"it makes a persisted built-in slug that built-in's row"` — `TestUnion_PersistedBuiltinIsOneRow`
- `"it makes a persisted invalid file that file's row"` — `TestUnion_PersistedInvalidFileIsOneRow`
- `"it keeps both the built-in and its reserved-name collider"` — `TestUnion_ReservedNameIsTheOnlyTwoRowCase`
- `"it mints a not-found row for a persisted slug resolving to nothing"` — `TestUnion_UnresolvablePersistedSlugIsNotFound`
- `"it mints unreadable rather than not found under an unusable directory"` — `TestUnion_UnresolvablePersistedSlugIsUnreadableWhenDirUnusable`
- `"it mints bad name for a charset-rejected persisted string"` — `TestUnion_CharsetRejectedPersistedStringIsBadName`
- `"it reads no slot under a constant"` — `TestUnion_ConstantContributesOnlyTheConstant`
- `"it collapses two slots naming the same missing slug"` — `TestUnion_BothSlotsSameMissingSlugIsOneRow`
- `"it yields built-ins only for an absent directory"` — `TestUnion_AbsentDirectoryIsBuiltinsOnly`
- `"it carries the directory condition as a flag, never a row"` — `TestUnion_DirUnusableIsAFlagNotAMember`
- `"it counts rows produced and the unselectable subset"` — `TestUnion_CountAndRejectedAttrs`
- `"it emits enumerated on every open"` — `TestUnion_EnumeratedFiresPerOpenUndeduped`
- `"it re-derives with changed keys and no directory read"` — `TestUnion_ReassembleReadsNothing` (directory removed between calls)
- `"it emits nothing on the discard logger"` — `TestUnion_DiscardSilencesEnumerated`
- `"it is fakeable wholesale"` — `TestUnion_IsAnOrdinaryValue` (construct a `Union` literal with no loader and no directory)

**Edge Cases**:
- The union keys on **"resolves"**, not "has a file" — a persisted built-in slug **is** that built-in's row, never a second `⚠ not found` row, which is the state the panel's most common action (`Enter` on `tokyo-night`) produces.
- One slug is one row **always**, so a persisted slug naming an existing-but-invalid file is that file's row carrying both the reason and (task 8-3) the badge.
- A `reserved name` file is the **one** legitimate two-rows-for-one-slug case — the built-in and the rejected file both stand, because that collision is the reason's entire content.
- A persisted slug resolving to neither built-in nor file gets its own unselectable `not found` row; a persisted string rejected by §8.6's charset check gets `bad name`, **never** `not found` — telling a user their file is missing when they typed an illegal name sends them looking in the wrong place.
- It applies **per slot** under a pair with one dead slug; under a constant only the constant contributes, because the slots are not read at all.
- An **absent** directory yields built-ins only, silently; an **unusable** one yields built-ins plus persisted rows with the directory condition returned as a **flag**, never a union member.
- `theme: enumerated` fires on **every** open including both those cases — `count` is rows produced (built-ins included, the `⚠ dir unreadable` chrome row excluded) and `rejected` is the unselectable subset carrying a §6.2 reason.
- Built-in rows carry nothing distinguishing them from drop-ins.
- Assembly lives in `internal/theme` so `internal/tui` never becomes a fourth package emitting the `theme` component, and both attrs are computable where they are emitted.
- The seam returns the **finished** union rather than a directory listing, and takes the raw persisted keys as an input.
- The entry point must be **re-callable with changed prefs state and no fresh directory read**, because §9.2's post-commit recompute (Phase 9) and task 8-10's `Esc` both depend on exactly that.
- Phase 1's per-process dedup on `theme: rejected` is what stops five opens producing five identical WARN sets.
- The union is an ordinary value so a fixture can fake it wholesale with no real directory.

**Context**:
> §9.4: "**Every `*.theme` file in the themes directory gets a row, plus every built-in, plus any slug named in `prefs.json` that resolves to neither.** '**Resolves**', not 'has a file' — the distinction is load-bearing… keying the union on file-existence would mint a second `⚠ not found` row for every persisted built-in slug — which is the state produced by the panel's most common action, pressing `Enter` on `tokyo-night`. **One slug is one row**, always."
> §13.3: "**The seam returns the finished §9.4 union, not a directory listing.** It takes the raw persisted theme keys and returns the complete row set… already deduped one-slug-one-row and already carrying each row's reason. **`internal/theme` owns that assembly**, which is what keeps three other decisions consistent: `theme: enumerated`'s `count` and `rejected` are computable where they are emitted (§12.3), `internal/tui` does not become a fourth package emitting the `theme` component (§8.9 closes that set at three), and the panel does no merging of its own."
> §12.3 (`theme: enumerated`): "INFO. At panel open, **every open** (it is a per-event INFO, not a repeated warning, so it needs no dedup). `count` is **rows produced** — the full §9.4 union, built-ins included — and `rejected` is **unselectable rows**… Fires on an **absent** directory too (`count` reflects the built-ins) and on an **unusable** one (`count` likewise, alongside `theme: directory unusable`)."
> §5.5: "A theme made unreachable by an unusable directory carries the reason `unreadable`, not `not found`."
> §9.5: "**An unreadable themes directory gets its own row** — `⚠ dir unreadable`, **chrome pinned to the viewport directly beneath the header, not a list row**… **It is not counted by `theme: enumerated`'s `count`**, which counts rows produced by the §9.4 union."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.4, §9.5, §5.5, §5.8, §12.3, §13.3

## theming-system-8-2

### Task 8.2: Row ordering — sort key, comparison and the built-in-first tie

**Problem**: The union arrives in enumeration order (built-ins, then `os.ReadDir` order, then persisted leftovers), which is neither alphabetical nor deterministic across filesystems. §9.5 fixes the order as alphabetical by slug — but "by slug" is not enough on its own, because three row shapes have no slug: a `bad name` file (§5.2 rejects rather than normalises, so it never yields one), a charset-rejected persisted string (it has neither slug nor file), and — the inverse trap — a `reserved name` file, which *is* labelled by its filename yet must sort by its **slug** so it lands adjacent to the built-in it collides with, which is the whole of §9.5's adjacency argument. Comparison is also not free: filenames are not lowercase by construction, so a byte-wise-only comparison files `Zed.theme` ahead of every valid theme. And one tie is **guaranteed by construction** and unsettleable byte-wise — a `reserved name` row and its built-in have an *identical* sort key, because that identity is the definition of the reason. Left to `sort.Slice`'s unstable ordering the panel fixtures stop being reproducible and the adjacency argument becomes incidental rather than concrete.

**Solution**: Two separate derived values on `Row` — a fully-determined sort key and a display label, neither derived from the other — plus a total comparator (case-insensitive, byte-wise tie-break, built-in first) applied to `Union.Rows` inside the assembler so every consumer receives the union already ordered.

**Outcome**: `Union.Rows` is in a total, deterministic order for any input; `nord` (built-in) sorts immediately before `nord.theme` (`reserved name`); `Zed.theme` sorts among the `z`s rather than ahead of everything; and `Row.Label()` is a separate value from `Row.SortKey()`, so a `reserved name` row is labelled `nord.theme` while sorting on `nord`.

**Do**:
- Add to `internal/theme/union.go`:
  ```go
  // SortKey is the §9.5 ordering value. Slug wherever one exists, else the
  // filename, else the persisted string itself.
  func (r Row) SortKey() string
  // Label is the §9.5 DISPLAY value and is deliberately NOT derived from SortKey.
  func (r Row) Label() string
  ```
- **`SortKey()`**: `r.Slug` when non-empty — including a `reserved name` row, whose slug is valid and identical to the built-in's, and a `not found` persisted row. Else `r.Filename` when non-empty (the `bad name` file case). Else `r.Persisted` (the charset-rejected persisted string — control-stripped at read per Phase 5 task 5-2). Document that there is exactly one thing to sort that last shape by and that using it is what keeps the order **total**.
- **`Label()`**: `r.Filename` for a **`bad name`** row *and* for a **`reserved name`** row (`nord.theme` beside `nord` tells the user which one is theirs, where two rows reading `nord` would not); `r.Persisted` for a charset-rejected persisted row; `r.Slug` otherwise. Carry an explicit source comment that the two functions are independent values and that neither may be re-derived from the other at render time — task 8-4's delegate consumes `Label()` and nothing else.
- **Comparator**: `sort.SliceStable` over `Union.Rows` with, in order: `strings.ToLower(a.SortKey()) < strings.ToLower(b.SortKey())`; on equality, byte-wise `a.SortKey() < b.SortKey()`; on equality, `a.Source == SourceBuiltin` sorts first. Comment the third leg as the guaranteed-by-construction tie and why the built-in wins — "the valid, selectable thing the user can act on, immediately followed by the row explaining why their file is not it".
- Sort **inside `Reassemble`** (task 8-1), so both the `Open` path and every recompute receive an ordered union and no consumer can forget to sort.
- Add nothing else to the ordering. **Alphabetical by slug and nothing else** — same-mode-first was proposed as a mixed-mode-flash mitigation and rejected (§9.2), so no palette is ever inspected here and no variant concept enters.
- Leave the `⚠ dir unreadable` condition **outside the ordering entirely** — it is `Union.DirUnusable`, not a row, so there is nothing to sort (task 8-6 renders it as viewport chrome).

**Acceptance Criteria**:
- [ ] `nord` (built-in) sorts immediately before `nord.theme` (`reserved name`), with no other row able to fall between them.
- [ ] A `reserved name` row's `SortKey()` is its **slug** while its `Label()` is its **filename**, asserted on the same row.
- [ ] A `bad name` file sorts by filename; a charset-rejected persisted string sorts by itself; a `not found` persisted row sorts by its slug.
- [ ] `Zed.theme` sorts between `tokyo-night-day` and any `zz`-prefixed row, not ahead of `nord`.
- [ ] Two rows whose sort keys differ only in case order deterministically (case-insensitive first, byte-wise second) and the result is identical across runs.
- [ ] The order is **total**: shuffling the pre-sort input in a table-driven test always yields the identical output sequence.
- [ ] `Label()` and `SortKey()` are never equal by construction for a `reserved name` or `bad name` row, and a test asserts neither is computed from the other (changing the filename of a `reserved name` row moves its label and **not** its position).
- [ ] `Union.Rows` is ordered on return from both `Open` and `Reassemble`; no caller sorts.
- [ ] Nothing in the comparator reads a `Theme`, a canvas or any palette value.

**Tests**:
- `"it sorts a reserved-name row by slug and labels it by filename"` — `TestRowOrder_ReservedNameSortsBySlugLabelsByFilename`
- `"it puts the built-in ahead of its colliding file"` — `TestRowOrder_BuiltinFirstOnTheGuaranteedTie`
- `"it sorts a bad-name file by filename"` — `TestRowOrder_BadNameSortsByFilename`
- `"it sorts a charset-rejected persisted string by itself"` — `TestRowOrder_CharsetRejectedSortsByItself`
- `"it compares case-insensitively with a byte-wise tie-break"` — `TestRowOrder_CaseInsensitiveThenByteWise` (`Zed.theme` among the `z`s)
- `"it is total and deterministic"` — `TestRowOrder_TotalAndDeterministic` (shuffled input table)
- `"it keeps the sort key and label independent"` — `TestRowOrder_SortKeyAndLabelAreSeparateValues`
- `"it orders inside the assembler"` — `TestRowOrder_UnionIsOrderedOnReturn` (both `Open` and `Reassemble`)
- `"it never inspects a palette"` — `TestRowOrder_NoVariantConcept`

**Edge Cases**:
- The sort key is the **slug** wherever one exists — including a `reserved name` row, which is *labelled* by filename yet sorts by slug, putting it adjacent to the built-in it collides with — and a `not found` persisted row.
- Only a **`bad name`** row has no slug and sorts by filename.
- A persisted string rejected by the charset check has neither a slug nor a file and sorts by **itself**, control-stripped and (at render) truncated as it is for display — there is exactly one thing to sort it by and using it is what keeps the order total.
- Comparison is **case-insensitive with a byte-wise tie-break**, because slugs are lowercase by construction but filenames are not and byte-wise alone files `Zed.theme` ahead of every valid theme.
- The one tie guaranteed by construction — a `reserved name` row against its built-in, sharing an identical sort key by definition — cannot be settled byte-wise, and the **built-in sorts first**.
- Ordering is alphabetical by slug and nothing else; same-mode-first was **rejected**.
- The `⚠ dir unreadable` row is **outside the ordering entirely**, being viewport chrome rather than a list member.
- The order must be total and deterministic or the panel fixtures are not reproducible and §9.5's adjacency argument is incidental rather than concrete.
- Sort key and display label are **separate values**, never derived from one another at render time.

**Context**:
> §9.5: "**Sort key and display label are separate, and the sort key is fully determined:** The **sort key is the slug** wherever one exists — including a `reserved name` row, which is why it sorts adjacent to the built-in it collides with despite being *labelled* by filename… Only a **`bad name`** row has no slug; it sorts by **filename**. A **persisted string rejected by §8.6's charset check** has neither a slug nor a file — it sorts by **the persisted string itself**… Comparison is **case-insensitive, with a byte-wise tie-break**… **One tie is guaranteed by construction and the byte-wise tie-break cannot settle it: a `reserved name` row and the built-in it collides with have the identical sort key**… **The built-in sorts first**… It also makes the panel fixtures deterministic (§13.3), which a sort left to chance would not."
> §9.5: "**A `reserved name` row is likewise labelled by its filename.** Its slug is valid, but it is *identical* to the built-in's — labelling by slug would put two rows reading `nord` in a list where §9.5 deliberately makes built-in and drop-in rows indistinguishable."
> §9.2: "**List order is alphabetical by slug**; ordering same-mode themes first was proposed as a mitigation and **rejected** as unnecessary once the flash is accepted."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.5, §9.2, §5.2, §6.2

## theming-system-8-3

### Task 8.3: The `●` badge derivation table

**Problem**: The `●` marker is the panel's entire answer to "what is actually set?", and it is the one signal the picker idiom rests on — the cursor says what is *previewed*, the badge says what is *persisted*. Deriving it naively from the loaded nomination is wrong in two directions that both bite the most common installs. Under a **fallback** the nomination holds the theme that loaded, not the one the user set, so the badge would move onto a theme they never chose and silently claim the fallback was their choice — exactly what §6.3's "falling back must never overwrite the persisted theme name" exists to prevent, reintroduced at the display layer. And on a **virgin install** §8.1 leaves `prefs.json` absent entirely, so a persisted-slug-only rule shows no marker anywhere at all, which falsifies §9.4's whole justification for assembling the union ("the `●` marker always has something to sit on"). The slot vocabulary is also novel — §9.14 records that assigning a theme to a light/dark slot from inside a picker was found in no surveyed tool — so `● dark` / `● light` / `● both` / bare `●` has no established shape to borrow and must be pinned rather than inferred.

**Solution**: A pure derivation in `internal/theme` from Phase 5 task 5-4's per-slot resolution record — whose `Requested` field is by construction the slug a slot resolves to **before** fallback — onto a badge keyed by that slug, with the `● both` collapse and the constant's bare `●` handled as shapes of the same table.

**Outcome**: `Badges(slots)` returns a `map[string]Badge` the panel looks each row up in by its badge key: a constant yields one bare `●`; a pair yields `● light` + `● dark` on two rows, or a single `● both` when the slots name the same slug; a broken slot's badge stays on the **persisted** slug while the fallback's row carries none; and an untouched slot badges the shipped default's slug, so a virgin install shows two badges.

**Do**:
- Add `internal/theme/badge.go`:
  ```go
  type Badge int // BadgeNone, BadgeConstant, BadgeLight, BadgeDark, BadgeBoth
  func (b Badge) Text() string // "", "●", "● light", "● dark", "● both"
  func Badges(slots []SlotResolution) map[string]Badge
  // BadgeKey is the value a Row is looked up in Badges' map by. "" means the row
  // can never carry a badge.
  func (r Row) BadgeKey() string
  ```
- **Key every badge on `SlotResolution.Requested`** and comment that this single field *is* §9.5's three-row table: `Requested` is the persisted slug when the slot was set (`WasSet=true`) whether or not it loaded, and the shipped default's slug when it was not (`WasSet=false`). One field, three rows, no branching on `FellBack` — and state explicitly that reading `Resolved` instead would move the badge onto a fallback, which is the bug this task exists to prevent.
- **Shapes**: a `[]SlotResolution` of length 1 with `Slot == SlotConstant` yields `{Requested: BadgeConstant}`. A length-2 slice (light then dark) yields `{light.Requested: BadgeLight, dark.Requested: BadgeDark}`, collapsed to `{slug: BadgeBoth}` when the two `Requested` values are equal.
- **Define the row's lookup key, and exclude the one collision.** `Row.BadgeKey()` returns the row's identity — `Slug` where one exists, else `Persisted`, else `Filename` — i.e. the same value `SortKey()` derives from, which is what makes task 8-1's charset-rejected persisted row (keyed on its raw string) match its badge. The **one exception is a `reserved name` row, which returns `""` and therefore never carries a badge**: its slug is identical to the built-in's by definition, so a bare identity lookup would paint `●` on *both* rows, and the rejected file is not what is persisted — the persisted slug resolved to the built-in, which is the same discrimination task 7-6 draws for doctor's persisted line. State in-source that this is the only place the union's one legitimate two-rows-for-one-slug case has an observable consequence, and that the panel must look badges up through this method rather than reading `Slug` directly.
- **Pin the badge text verbatim** as package constants — `●`, `● dark`, `● light`, `● both` — and comment that `● both` is chosen over `● dark light` because with exactly two slots "both" is fully determined, and that it is deliberately **no wider than `● light`** so it cannot move task 8-4's truncation budget. Add a test asserting that width relation directly rather than leaving it to prose.
- Document, in-source, that the badge renders in **`accent.primary`** and never `state.positive`: `●` marks *assignment*, and `state.positive` would wrongly read as the Sessions list's attached-session dot. (The token is applied in task 8-4's delegate; this task fixes the decision beside the vocabulary so the two cannot drift.)
- Document that **the badge and the cursor are independent signals** — `●` is what is *set*, `▌` + tint is what is *previewed* — so a badge legitimately sits on an **unselectable** row (the persisted-but-broken theme), and that no consumer may infer selectability from the presence of a badge.
- Note in-source that the two setting states never coexist on screen because §8.2's `theme`-wins rule means the slots are **not read at all** — a resolution rule, not a file constraint — so `Badges` can never be handed a slice mixing a `SlotConstant` with a slot, and should treat that as a programming error rather than rendering both forms.
- Keep the function **pure and total**: no I/O, no logging, deterministic, and a nil/empty slice returns an empty map rather than panicking. It reads no `Theme` and no palette.
- Add nothing that *moves* a badge — a commit's recompute (the visible collapse of two slot badges into one bare `●` on a virgin install) is Phase 9's; this task supplies the derivation Phase 9 re-runs.

**Acceptance Criteria**:
- [ ] A constant `"theme": "nord"` yields exactly `{"nord": BadgeConstant}` whose text is the bare `●` with no slot word.
- [ ] A pair `{light: tokyo-night-day, dark: nord}` yields `● light` on `tokyo-night-day` and `● dark` on `nord`.
- [ ] Both slots naming the same slug yield a **single** entry with `BadgeBoth`, never two entries.
- [ ] A slot that **fell back** keeps its badge on `Requested` (the persisted slug) and puts **no** badge on `Resolved` (the fallback), asserted on a `SlotResolution` with `FellBack=true` and differing `Requested`/`Resolved`.
- [ ] A **never-set** slot badges the shipped default's slug, so a virgin install (`prefs.json` absent → `WasSet=false` on both slots) yields `tokyo-night-day` `● light` and `tokyo-night` `● dark`.
- [ ] A charset-rejected persisted value is badged on that raw value, so it matches the union row keyed by the same string (task 8-1) and the badge is not lost.
- [ ] `Row.BadgeKey()` returns the slug for a built-in, a valid file and a `not found` persisted row; the raw persisted string for a charset-rejected row; the filename for a `bad name` row; and **`""` for a `reserved name` row**.
- [ ] With `"theme": "nord"` persisted and a `nord.theme` drop-in present, exactly **one** row's `BadgeKey()` matches the badge map — the built-in's — so only one `●` can render.
- [ ] `lipgloss.Width(BadgeBoth.Text()) <= lipgloss.Width(BadgeLight.Text())`.
- [ ] The function is pure: identical input yields identical output, it performs no I/O and it references no `Theme`, canvas or palette value.
- [ ] A nil or empty slice returns an empty map with no panic.

**Tests**:
- `"it badges a constant with a bare dot"` — `TestBadges_ConstantIsBareDot`
- `"it badges each slot of a pair"` — `TestBadges_PairBadgesLightAndDark`
- `"it collapses matching slots to both"` — `TestBadges_SameSlugInBothSlotsIsBoth`
- `"it leaves the badge on the persisted slug under a fallback"` — `TestBadges_FallbackDoesNotMoveTheBadge`
- `"it badges the shipped default for an unset slot"` — `TestBadges_UnsetSlotBadgesShippedDefault` (the virgin-install case)
- `"it badges a charset-rejected persisted value on that value"` — `TestBadges_CharsetRejectedValueKeepsItsBadge`
- `"it gives a reserved-name row no badge key"` — `TestBadgeKey_ReservedNameRowHasNone`
- `"it keys every other row on its identity"` — `TestBadgeKey_MatchesRowIdentity` (table: built-in, valid file, `not found` persisted, charset-rejected persisted, `bad name` file)
- `"it keeps both no wider than light"` — `TestBadges_BothIsNoWiderThanLight`
- `"it is pure and total"` — `TestBadges_PureAndTotal` (nil slice, empty slice, repeat call)
- `"it reads Requested, never Resolved"` — `TestBadges_KeyedOnRequestedNotResolved` (a table where the two differ on every row)

**Edge Cases**:
- The three-row table is exhaustive and each row is load-bearing: set-and-loadable badges the persisted slug; **set-but-unloadable still badges the persisted slug**, because a fallback never moves the badge and the fallback's own row carries none; never-set badges the **shipped default's** slug.
- The never-set row is the most common install and the only thing making §9.4's "the `●` always has something to sit on" true on a virgin install, where §8.1 leaves `prefs.json` absent entirely and a persisted-slug-only rule would show no marker anywhere.
- `● dark` / `● light` under a pair, **`● both`** when the two slots name the same slug (reachable in two keypresses and a likely path), and a **bare `●`** under a constant with no slot word.
- The two setting states never coexist on screen because `theme` winning means the slots are not read — a resolution rule, not a file constraint.
- `● both` is deliberately no wider than `● light` so it does not move §9.5's truncation budget, and is chosen over `● dark light` because with exactly two slots "both" is fully determined.
- `●` marks **assignment** in `accent.primary`, never liveness — `state.positive` would wrongly read as the Sessions list's attached dot.
- The badge and the cursor are independent signals (`●` is what is *set*, `▌` + tint is what is *previewed*), so a badge legitimately sits on an unselectable row.
- The row→badge lookup key is `Row.BadgeKey()`, never `Row.Slug` read directly — because a **`reserved name`** row shares its slug with the built-in it collides with by definition, so a bare slug lookup paints `●` on both rows. The rejected file is not what is persisted (the slug resolved to the built-in), so `BadgeKey()` returns `""` for it — the only observable consequence of §9.4's one legitimate two-rows-for-one-slug case.
- Derivation reads the raw persisted keys plus Phase 5 task 5-4's per-slot resolution record, never the nomination alone, which cannot express a slug that never loaded.
- A commit visibly collapses two slot badges to one bare `●` on a virgin install, which is correct — two inherited defaults became one pin (the movement itself is Phase 9's).

**Context**:
> §9.5: "**The badge marks the slug a slot resolves to *before* fallback.** One rule covering all three cases… | Set and loadable → The persisted slug | Set but unloadable (§8.5) → Still the **persisted** slug — a fallback never moves the badge; the `●` means 'what is set', and the fallback's own row carries no badge | Never set → The **shipped default's** slug (§8.3) — `tokyo-night-day` carries `● light`, `tokyo-night` carries `● dark`."
> §9.5: "**When both slots name the same slug, that one row carries `● both`.** This is reachable in two keypresses (`d` then `l` on one row) and is a likely path… `● both` is chosen over a combined `● dark light` because with exactly two slots 'both' is fully determined, and it is no wider than `● light`, so it does not move the truncation budget."
> §9.1 (token table): "`●` / `● dark` / `● light` badge → `accent.primary` — the badge marks *assignment*, which is the primary-accent role Portal already uses for active dots and the selector bar; `state.positive` would wrongly imply liveness, which is what `●` means on the Sessions list."
> Phase 5 task 5-4 declared `SlotResolution{Slot, Requested, WasSet, Resolved, FellBack, Reason, Theme}` with `Requested` documented as "the slug that was nominated (shipped default when the slot was unset)" — which is precisely the pre-fallback value this table needs, and why the derivation reads the resolution record rather than the nomination.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.5, §9.4, §8.2, §8.3, §8.5, §9.1

## theming-system-8-4

### Task 8.4: Row composition and the invalid-row treatment

**Problem**: Four elements — the `⚠` glyph, the `●` badge, the label and the terse reason — compete for a fixed ~24–30 columns on a row that must be **exactly one delegate line**. That one-line rule is not cosmetic: it is the `bubbles/list` pagination invariant, and Portal already has the scar from breaking it (the in-`SessionItem` heading injection drew uncounted extra lines, overflowed the frame and scrolled the title and cursor off the top). Task 8-9's invalid-row arrow skip and task 8-11's paging both rest on it too. Without a fixed priority the elements collide non-deterministically at narrow widths; without a truncation floor a long user slug erases every other element; and without a pinned token split the invalid row either dims its own `⚠` into illegibility or renders the label in `text.faint`, which §13.5 floors *below* the UI threshold precisely so it can never carry content — while §9.4's entire justification for listing invalid files is that the user can read **which** of their files is broken.

**Solution**: A one-line `bubbles/list` delegate for the panel's row item, composing the four elements in a fixed priority with a pinned truncation floor, taking its tokens from the previewed `Theme` on **every** frame rather than caching them.

**Outcome**: Every panel row renders as exactly one line at any panel width down to the refuse threshold; an invalid row shows a legible `accent.attention` `⚠` and reason beside a `text.subtle` label; a persisted-and-invalid row keeps its badge and drops its reason; and the cursor row carries the shipped selection treatment so the panel reads as the same kind of list as Sessions.

**Do**:
- Add `internal/tui/theme_row.go` declaring the list item and delegate:
  ```go
  type themeRowItem struct{ Row theme.Row; Badge theme.Badge }
  func (i themeRowItem) FilterValue() string // the label; filtering is DISABLED on the panel list

  type themeRowDelegate struct {
      Theme      theme.Theme
      Colourless bool
      Width      int // the panel's inner content width
  }
  func (d themeRowDelegate) Height() int  { return 1 }
  func (d themeRowDelegate) Spacing() int { return 0 }
  func (d themeRowDelegate) Render(w io.Writer, m list.Model, index int, item list.Item)
  ```
  Note in the item's doc comment that panel search/filtering is **deferred by decision** (§1.4), so the panel list is constructed with `SetFilteringEnabled(false)` and `FilterValue` is never consulted — declared for the interface only.
- **Compose in this fixed priority**, budgeting against `d.Width`:
  1. **Cursor column** — 2 cells (`▌` + space on the selected row, two canvas spaces otherwise) so every row shares a left edge.
  2. **`⚠` glyph** — 2 cells (`⚠` + space), **always** rendered on an invalid row. It is the invalidity signal.
  3. **`●` badge**, right-aligned — reserve `width(badge) + 1` when the row has one. §9.4 exists so the marker always has a home, so the badge **outranks** the reason.
  4. **Label** — `theme.Row.Label()` (task 8-2), truncated with `…` to a floor of **three visible characters plus the ellipsis**.
  5. **Terse reason**, right-aligned — rendered only when what remains after satisfying the label at its *natural* width still holds it plus one separating space. It is therefore **the first element dropped** whenever a badge competes for the same edge: `⚠` still says the row is invalid and doctor says why, which is the split §6.3 draws.
  Below the label floor the panel is already at task 8-11's refuse threshold, so add **no** further degradation rule.
- **Tokens, per §9.1's table, with no raw hex at any call site** (the colour-literal guard still scans `internal/tui`): valid label `text.primary`; invalid label **`text.subtle`, not `text.faint`**; the `⚠` and its reason `accent.attention` — the `⚠` keeps its own token rather than inheriting the dimmed row so the invalidity signal stays legible on a deliberately dimmed line; badge `accent.primary`; row background `canvas`; cursor row the shipped selection treatment — `bg.selection` tint across the full row width, `accent.primary` `▌`, `text.on-selection` label.
- **Reason text is §6.2's terse vocabulary verbatim, prefixed `⚠ `** — the `Reason` constants' own string values (`missing tokens`, `bad colour`, `bad syntax`, `bad name`, `reserved name`, `unreadable`, `not found`). Render the glyph from the rejection's reason and never re-word it; full detail stays in doctor.
- **Re-derive per frame.** The delegate is constructed from the *previewed* theme at each render (task 8-6 rebuilds it in the panel's render path, task 8-9 re-points it on every arrow), so rows, badges, reasons and the cursor treatment are never cached — this is §11.2's per-keypress surface and the worst case of the cached-style class.
- **Glyph-backed throughout** (`⚠`, `●`, `▌`) so the row survives a colourless render even though §9.10 blocks the panel under `NO_COLOR`; honour `Colourless` by dropping the canvas background and every hue, exactly as `SessionDelegate` does.
- A slug arriving from prefs is **already control-stripped** (Phase 5 task 5-2); truncation is the panel-local half of that split and is the only string surgery this delegate performs.

**Acceptance Criteria**:
- [ ] Every row renders exactly one line — `lipgloss.Height(row) == 1` — for every combination of (valid/invalid) × (badge/no badge) × (short/long label) at the preferred and minimum widths.
- [ ] An invalid row with **no** badge renders `⚠`, label and reason; the same row **with** a badge renders `⚠`, label and badge, and the reason is absent.
- [ ] A label longer than the remaining budget is truncated with `…` and never falls below three visible characters plus the ellipsis.
- [ ] An invalid label renders in `text.subtle` while its `⚠` and reason render in `accent.attention` — asserted as distinct SGR runs on the same line, so the `⚠` demonstrably does not inherit the dimmed label's colour.
- [ ] `text.faint` appears nowhere in a panel row's output.
- [ ] A `bad name` row is labelled by its **filename** and a `reserved name` row likewise, both via `Row.Label()` with no second derivation in the delegate.
- [ ] A `reserved name` row renders **no** `●` even when its slug is the persisted one, while the built-in it collides with renders the badge — asserted on the two adjacent rows of the same union.
- [ ] A valid row is `text.primary` and a built-in is byte-identical to a valid drop-in with the same label and badge state.
- [ ] The cursor row carries the `bg.selection` tint across the full row width with an `accent.primary` `▌` and a `text.on-selection` label.
- [ ] Rendering the same item under two different `Theme`s produces different colour output with no cached-style carry-over (the delegate holds no package-scope or construction-time derived style).
- [ ] Under `Colourless` the row emits no background SGR and no hue, and still carries `⚠`, `●` and `▌` as glyphs.
- [ ] `internal/tui`'s colour-literal guard passes — the delegate contains no hex literal.

**Tests**:
- `"it renders every row on exactly one line"` — `TestThemeRow_AlwaysOneDelegateLine` (matrix table)
- `"it drops the reason first when a badge competes"` — `TestThemeRow_ReasonIsDroppedBeforeBadge`
- `"it always renders the warning glyph on an invalid row"` — `TestThemeRow_InvalidAlwaysCarriesTheGlyph`
- `"it truncates the label to the three-character floor"` — `TestThemeRow_LabelTruncationFloor`
- `"it dims the invalid label without dimming its warning"` — `TestThemeRow_InvalidLabelIsSubtleWarningIsAttention`
- `"it never uses text.faint"` — `TestThemeRow_NeverUsesTextFaint`
- `"it labels bad-name and reserved-name rows by filename"` — `TestThemeRow_FilenameLabelledRows`
- `"it badges the built-in and not its reserved-name collider"` — `TestThemeRow_ReservedNameRowCarriesNoBadge`
- `"it makes a built-in indistinguishable from a drop-in"` — `TestThemeRow_BuiltinRendersLikeADropIn`
- `"it applies the shipped selection treatment to the cursor row"` — `TestThemeRow_CursorRowSelectionTreatment`
- `"it re-derives from the previewed theme every frame"` — `TestThemeRow_NoCachedStyles` (render, swap theme, render, diff)
- `"it stays glyph-backed and hue-free under colourless"` — `TestThemeRow_ColourlessIsGlyphBacked`
- `"it renders the terse reason vocabulary verbatim"` — `TestThemeRow_ReasonLabelsAreTheSixTerseStrings`

**Edge Cases**:
- One row is exactly **one delegate line**, never two — the `bubbles/list` pagination invariant that paging and the invalid-row skip both rest on, and the same invariant the Sessions group headers were reshaped to preserve.
- The four-element priority is fixed: `⚠` glyph (always on an invalid row, two columns) → right-aligned `●` badge → label truncated with `…` down to a floor of **three visible characters** → terse reason, right-aligned and **the first element dropped** when a badge competes for the same edge.
- Below the truncation floor the panel is already at §9.8's refuse threshold, so no further degradation rule is needed.
- An invalid label renders in **`text.subtle`, not `text.faint`** — the user must be able to read *which* file is broken, and §13.5 forbids `text.faint` reaching the UI floor precisely so it never carries content.
- The `⚠` and its reason keep **`accent.attention`** rather than inheriting the dimmed row, so the invalidity signal stays legible on a deliberately dimmed line.
- `bad name` **and** `reserved name` rows are labelled by **filename** — `nord.theme` beside `nord` tells the user which one is theirs, where two rows reading `nord` would not.
- A valid row is `text.primary`, selectable, and a built-in is deliberately indistinguishable from a drop-in.
- The cursor row takes the shipped selection treatment (`bg.selection` tint, `accent.primary` `▌`, `text.on-selection` name) so the panel reads as the same kind of list as Sessions.
- Reason labels are §6.2's terse vocabulary verbatim, each prefixed `⚠ `, with full detail staying in doctor.
- The delegate **re-derives per frame** from the previewed theme — rows, badges, reasons and the cursor treatment are never cached, this being §11.2's per-keypress surface.
- The row is glyph-backed so it survives colourless even though §9.10 blocks the panel under `NO_COLOR`.
- A slug from prefs arrives already control-stripped (Phase 5 task 5-2); truncation is the panel-local half of that split.

**Context**:
> §9.5: "**Row composition — one row per theme, always.** An invalid row never wraps to two lines: every list row is exactly one delegate line, which is the invariant `bubbles/list` pagination depends on… The elements compete for a fixed ~24–30 columns in this priority order: 1. **The `⚠` glyph**… 2. **The `●` badge**… right-aligned… the badge outranks the reason. 3. **The label**… truncated with `…`… down to a floor of three visible characters plus the ellipsis. 4. **The terse reason**, right-aligned — **the first element dropped** when a badge competes for the same edge."
> §9.1 (token table): "Invalid row label → `text.subtle` — **not `text.faint`**: §2.5 and §13.5 make `text.faint` decorative-only and *forbid* it reaching the UI floor, but this label is the filename or slug the user must read to know which of their files is broken… Invalid row `⚠` and its terse reason → `accent.attention`… The `⚠` keeps its own token rather than inheriting the row's dimmed treatment, so the invalidity signal stays legible on a row that is deliberately dimmed."
> §11.2: "**The panel introduces a third `bubbles/list` instance, and it is the worst case of this class.** Its styles are assigned once at panel open, and it is the one surface whose theme changes on *every* arrow keypress… **The panel's delegate re-derives per frame from the previewed theme**, like every other delegate — so rows, badges, reasons and the cursor treatment are never cached."
> §1.4 records panel search / filtering as deliberately deferred, which is why the panel list disables filtering rather than wiring `FilterValue`.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.5, §9.1, §11.2, §6.2, §6.3, §13.5, §1.4

## theming-system-8-5

### Task 8.5: The panel keymap scope and its vertical footer

**Problem**: The panel introduces four commit/close keys through a **bespoke vertical footer outside `keymap.go`** — a second place a key label can go stale, which is the exact drift class `keymap_dispatch_guard_test` exists to guard and which this codebase has already paid for once (the retired `projectHelpKeys` second binding list). A horizontal footer is not an option: `⏎ set theme · d set as dark · l set as light · esc close` does not fit ~30 columns. And the scope cannot be authored as just the four visible keys, because the guard's contract is descriptor↔dispatch **parity** — arrows and paging are dispatched inside the panel, so omitting them from the descriptor is precisely what breaks the guard rather than what satisfies it.

**Solution**: A panel scope in the keymap descriptor carrying all six keys with the four commits marked `Core`, plus a vertical footer renderer that takes its entries as a **parameter** and renders the `Core` subset one row per entry — so Phase 9's nested confirm scope can substitute its own entries without a second renderer.

**Outcome**: `themePanelKeymap()` lists `↑↓`, `^↑/↓`, `⏎`, `d`, `l`, `esc`; `renderThemePanelFooter` renders exactly four rows reading `⏎ set theme` / `d set as dark` / `l set as light` / `esc close` in the panel's key-column idiom; and neither main-screen footer nor either page's help body gains an entry.

**Do**:
- Add `themePanelKeymap()` to `internal/tui/keymap.go`, beside the existing page descriptors and doc-commented in the same register:
  ```go
  func themePanelKeymap() []keymapEntry {
      return []keymapEntry{
          {Key: "↑↓", HelpKey: "↑/↓", Action: "navigate", HelpAction: "Move selection"},
          {Key: "^↑/↓", Action: "page", HelpAction: "Next / prev page"},
          {Key: "⏎", Action: "set theme", HelpAction: "Set as the theme", Core: true},
          {Key: "d", Action: "set as dark", HelpAction: "Assign to the dark slot", Core: true},
          {Key: "l", Action: "set as light", HelpAction: "Assign to the light slot", Core: true},
          {Key: "esc", Action: "close", HelpAction: "Close the theme picker", Core: true},
      }
  }
  ```
  The `Action` strings are §14A's pinned copy and the rendered rows must read exactly `⏎ set theme`, `d set as dark`, `l set as light`, `esc close`. No entry sets `RightAligned` (a vertical footer has no right anchor) and there is **no `?` entry** — `?` does nothing inside the panel.
- **Document the Core/non-core split in the descriptor's doc comment**: the descriptor must be **complete** or the dispatch guard's parity is what breaks, while the footer renders only the `Core` entries — arrows and paging are present as **non-core**, exactly the distinction §14.1 applies to arrows on the main footer and for the same reason (arrows in a list are a given). Say plainly that this split is how the six-entry descriptor and §14A's pinned four-row footer are both satisfied without either being a special case.
- **Add `renderThemePanelFooter(entries []keymapEntry, width int, th theme.Theme, colourless bool) string`** in a new `internal/tui/theme_panel_footer.go`: filter to `Core`, render one row per entry as a key glyph (`helpKeyGlyph`, so `HelpKey` overrides resolve exactly as they do in the help body) in **`accent.key`** padded to a fixed key column, a single canvas gap, then the `Action` label in **`text.muted`** — the same split the horizontal footer uses, and the same two-column key-column idiom as `helpModalRow`. Every cell is canvas-painted; `colourless` drops the canvas and every hue.
- **Take the entries as a parameter, never call `themePanelKeymap()` inside the renderer.** State in the doc comment that Phase 9 adds a **nested confirm scope** beneath the panel scope whose footer temporarily replaces this one (`y confirm` / `n cancel`), so the shape must admit substitution rather than assuming one footer per panel.
- **Pin the footer's height** as a value derived from the `Core` count so task 8-6's layout and task 8-11's floor arithmetic both read one source (`themePanelFooterHeight(entries)`), measured off the rendered block exactly as `sessionFooterHeight` is.
- **Prove containment**: add assertions that the panel scope does not leak — `sessionsKeymap()`, `projectsKeymap()`, both rendered footers and both help bodies are byte-unchanged by this task, and no `set theme` / `set as dark` / `set as light` string appears in any of them.
- Leave `keymap_dispatch_guard_test`'s coverage of this scope to **Phase 9** (it lands with the `Enter`/`d`/`l` dispatch it would otherwise probe against a no-op), but note in the descriptor's doc comment that the guard will consume it and that the scope must therefore already be complete.

**Acceptance Criteria**:
- [ ] `themePanelKeymap()` returns exactly six entries in the declared order, four of them `Core`.
- [ ] The rendered footer is exactly four rows reading `⏎ set theme`, `d set as dark`, `l set as light`, `esc close` — asserted against the §14A strings verbatim, not a paraphrase.
- [ ] Arrows and paging are present in the descriptor and **absent** from the rendered footer.
- [ ] Key glyphs render in `accent.key` and labels in `text.muted`, asserted as distinct SGR runs.
- [ ] Labels share a left edge (the key column is padded to a fixed width) and every row is exactly one line.
- [ ] Passing a different entry slice (a two-entry `y confirm` / `n cancel` stand-in) renders that footer instead, with no change to the renderer — proving the Phase 9 substitution point.
- [ ] `themePanelFooterHeight` equals `lipgloss.Height` of the rendered block for both the four-entry and the two-entry cases.
- [ ] No `RightAligned` entry and no `?` entry exists in the scope.
- [ ] `sessionsKeymap()`, `projectsKeymap()`, both page footers and both help bodies are byte-identical before and after this task.
- [ ] Under `colourless` the footer emits no background SGR and no hue.

**Tests**:
- `"it carries all six panel keys"` — `TestThemePanelKeymap_CarriesAllSixKeys`
- `"it marks exactly the four commits core"` — `TestThemePanelKeymap_CoreIsTheFourCommits`
- `"it renders the four pinned footer rows"` — `TestThemePanelFooter_PinnedCopy`
- `"it omits arrows and paging from the footer"` — `TestThemePanelFooter_NonCoreEntriesAreNotRendered`
- `"it splits key and label tokens"` — `TestThemePanelFooter_KeyIsAccentKeyLabelIsTextMuted`
- `"it aligns the labels in a fixed key column"` — `TestThemePanelFooter_KeyColumnIsFixedWidth`
- `"it renders whatever entries it is given"` — `TestThemePanelFooter_AcceptsASubstitutedScope` (the Phase 9 confirm stand-in)
- `"it measures its own height"` — `TestThemePanelFooter_HeightMatchesRender`
- `"it leaves the page keymaps untouched"` — `TestThemePanelKeymap_DoesNotLeakIntoPageSurfaces`
- `"it drops hue under colourless"` — `TestThemePanelFooter_Colourless`

**Edge Cases**:
- The descriptor carries **all six** keys because an incomplete scope is what breaks the dispatch guard's descriptor↔dispatch parity.
- The footer renders only the **`Core`** entries with arrows and paging present as **non-core** — exactly the distinction §14.1 applies to arrows on the main footer, and how the six-entry descriptor and §14A's pinned four-row footer are both satisfied without either being a special case.
- The copy is pinned verbatim: `⏎ set theme` / `d set as dark` / `l set as light` / `esc close`.
- The footer is **vertical** because a horizontal keymap does not fit a ~30-column panel; the vertical form matches the help modal's key-column idiom.
- Key glyphs `accent.key`, labels `text.muted` — the same split the horizontal footer uses.
- **`?` does nothing inside the panel**: it is swallowed with everything else, there is no panel help modal, and the scope exists to drive the footer and the guard rather than a help body — a help modal over a non-blanking overlay would also stack the shape §9.6 rejects.
- The panel scope must not leak into either main-screen footer or either page's help body.
- `keymap_dispatch_guard_test`'s coverage of this scope is **Phase 9's**, but the descriptor must already be complete.
- Phase 9 adds a **nested confirm scope** beneath this one whose footer temporarily replaces it, so the shape must admit that rather than assuming one footer per panel.

**Context**:
> §9.12: "**The panel's keys live in the keymap descriptor as a panel scope** — **all six**: `↑`/`↓`, `Ctrl+↑`/`Ctrl+↓`, `Enter`, `d`, `l`, `Esc`. The descriptor must be complete or the dispatch guard's descriptor↔dispatch parity is what breaks. **Its vertical footer renders from the descriptor, filtered to the `Core` entries** — `Enter`, `d`, `l`, `Esc`. Arrows and paging are present in the descriptor as **non-core**… **`?` does nothing inside the panel.**"
> §9.1: "**A vertical keymap footer** (`⏎ set theme` / `d set as dark` / `l set as light` / `esc close`) rather than Portal's horizontal footer row — a horizontal keymap does not fit a ~30-column panel, and the vertical form matches the help modal's key-column idiom." Token table: "Vertical keymap footer → key glyphs `accent.key`, labels `text.muted` — the same split the horizontal footer uses."
> §9.2: "**The panel footer switches to the confirm's own keys while it is live** — `y confirm` / `n cancel` — and switches back when it resolves… **The confirm's keys live in the descriptor as a nested confirm scope** under the panel scope (§9.12)." (Phase 9; this task only leaves room for it.)

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.12, §9.1, §14.1, §14A

## theming-system-8-6

### Task 8.6: The slide-over surface — overlay, chrome and the pinned directory row

**Problem**: There is no surface. And the surface's shape is not a preference: Portal's modals **blank the page to the canvas** before drawing (`modal.go` clears to canvas, then `placeModalOnClearedCanvas`), so a modal theme picker would render the canvas plus its own frame and **preview nothing** — and live preview is the entire feature. Non-blanking is therefore the only shape that can do the job, and every downstream constraint already decided (the ~24–30 column budget, the four-element row priority, the message-slot truncation rule, the accepted covering of three footer entries) follows from that one fact. Two sub-surfaces have no other home either: the header's exact two-row cost, which task 8-11's floor arithmetic resolves against, and the `⚠ dir unreadable` row, which is what stands between a user with an unreadable directory and the "completely in the dark" state — and which cannot be a list delegate, because a list row participates in pagination and would vanish the moment the user paged down.

**Solution**: A panel model and renderer in `internal/tui` composing a full-height, right-edge, left-border-only block — header, optional pinned directory row, the panel's own `bubbles/list` instance, an empty-but-budgeted message slot, and task 8-5's vertical footer — composited over the already-composed page view through the existing lipgloss `Compositor` z-layer mechanism, with the page beneath deliberately **not** re-laid-out.

**Outcome**: Given a union, a badge map and a theme, `renderThemePanel` produces a block of exactly `height` rows and `width` columns whose left column is a `border` rule and whose body is `canvas`; overlaid at the right edge it leaves the Sessions/Projects list fully visible behind it, cutting whatever falls under it mid-label.

**Do**:
- Add `internal/tui/theme_panel.go` with the panel state and its renderer:
  ```go
  type themePanel struct {
      open        bool
      list        list.Model            // the THIRD bubbles/list instance
      enumeration theme.Enumeration     // retained for the panel's lifetime (task 8-7)
      union       theme.Union
      badges      map[string]theme.Badge
      message     string                // §9.1 message slot — ALWAYS empty in Phase 8
      width       int
  }
  func renderThemePanel(p themePanel, height int, th theme.Theme, colourless bool) string
  func overlayThemePanel(base, panel string, contentW int) string
  ```
- **Chrome, every surface resolving to an existing token** (§9.1's table): body `canvas`; **left border only** — one `panelFrameSide` (`│`) cell per row in `border`, no top, bottom or right edge, so it reads as a slide-over rather than an inset bordered dialog; header label `Themes` in **`accent.mode`** (bold) followed by a one-row `border` rule (`headerRuleGlyph`) spanning the inner width. **The header costs exactly two rows and carries no theme count** — noise at this list size — and that two-row cost is what task 8-11's floor resolves against. Explicitly **do not adopt** the reference frames' `#0C0C16` body and `#2B3050` border: they are per-frame literals, expressing that distinction would need a 20th token whose only role is "slightly different from the canvas", and every panel surface resolving to an existing token is what keeps the colour-literal guard and §13.4's swap-and-diff guard free of a carve-out.
- **The `⚠ dir unreadable` row is chrome pinned directly beneath the header, not a list delegate.** Render it from `union.DirUnusable` in `accent.attention` (glyph and text), above the list body, always visible regardless of page. Pin its copy as a constant — `⚠ dir unreadable`, deliberately 16 columns so it fits the minimum width **without truncation**; none of task 8-4's four composition priorities apply to it (no label, no badge, no reason) and the truncation-floor argument does not transfer to a fixed string. **Built-in and persisted-slug rows still render beneath it** — a user with an unreadable directory must not lose the `●`.
- **The message slot** is a single row directly above the footer and is **not reserved when empty**: `renderThemePanelMessage` returns `""` for an empty `message`, and the list's height budget subtracts its *measured* height, so a message appearing shrinks the list by one exactly the way the main screen's notice band recomputes list height. **Both contenders are Phase 9's** (the slot-from-constant confirm and the failed-commit line); Phase 8 leaves the slot's height accounted for and the field always empty. Do not add a contender, a setter or an arbiter here.
- **Lay out the panel** as: header (2) + directory row (0 or 1) + list body (the remainder, ≥ 1) + message slot (0, always, this phase) + footer (task 8-5's measured height). Size the panel's `list.Model` to `(innerWidth, bodyRows)` through the same `SetSize` discipline the other two lists use, and construct it with `SetFilteringEnabled(false)`, `SetShowTitle(false)`, `SetShowStatusBar(false)` and `SetShowHelp(false)` — the panel's own chrome supplies all of that. Declare `themePanelPreferredWidth = 30` and `themePanelMinWidth = 24` here as the two ends of the ladder; **task 8-11 owns choosing between them and the refusal below the minimum** — this task renders at whatever width it is handed.
- **Declare the delegate's single construction point.** Add `func (m Model) themeRowDelegate() themeRowDelegate` returning task 8-4's delegate built from the **previewed** theme (`m.activeTheme`), `m.colourless`, and the panel's current inner content width — and build the list's delegate only through it, here at construction and (task 8-9) on every restyle. There must be exactly one place the three inputs are assembled: two construction sites can disagree about width or colourlessness, and that disagreement is invisible until a resize during a live preview, on the surface §11.2 calls the worst case of the cached-style class.
- **Composite, do not re-lay-out.** `overlayThemePanel` mirrors `overlayHelpOnPreview` exactly: `lipgloss.NewLayer(base).X(0).Y(0).Z(0)` and `lipgloss.NewLayer(panel).X(contentW - panelWidth).Y(0).Z(1)` through `lipgloss.NewCompositor(...).Render()`. State in-source that the main screen is deliberately **not** re-laid-out while the panel is open — that is what keeps the swap the O(1) restyle of §11.1 and keeps the surface being previewed from reflowing under the user — so the overlay cuts wherever its left border falls, mid-label included (`x proje▏`), which is **not** a §14.4 violation (§14.4 governs how the footer lays *itself* out as the terminal narrows; the panel is an opaque layer over a footer that laid out at full width).
- **No animation.** Opening and closing are one frame each. Record in-source why: §11.3's OSC 11 emission would fire repeatedly through a canvas-bearing slide, intermediate panel widths would render frames no fixture covers, and `t` followed immediately by `Esc` would need a race resolved. "Slide-over" names the shape — full-height, right-edge, left-border-only — not a motion idiom.
- Note in the panel's doc comment that this is the **third `bubbles/list` instance** and §11.2's worst case of the cached-style class: its `bubbles/list`-owned styles (pagination dots, its own help/title styles) are assigned here at construction and are re-pointed by task 8-9's restyle path, not rebuilt.

**Acceptance Criteria**:
- [ ] `renderThemePanel` returns exactly `height` lines, each exactly `width` cells, for every height from the floor upward.
- [ ] Every row's first cell is the `border`-coloured `│`; no top, bottom or right border glyph is emitted anywhere.
- [ ] The header is exactly two rows — `Themes` in `accent.mode` then a `border` rule — and contains **no count**.
- [ ] With `DirUnusable` true the `⚠ dir unreadable` row renders directly beneath the header in `accent.attention`, is **not** a list item (it is present on page 2 as well as page 1), and built-in and persisted rows still render beneath it.
- [ ] The directory row's copy is exactly `⚠ dir unreadable` and fits `themePanelMinWidth` without truncation.
- [ ] With an empty `message` the slot contributes zero rows and the list body is one row taller than with a one-row message (asserted by setting the field directly, since no contender exists yet).
- [ ] The panel body is `canvas`; a `NO_COLOR`-style `colourless` render emits no background SGR anywhere in the block.
- [ ] `overlayThemePanel` leaves every base cell to the left of the panel byte-identical and replaces every cell under it, with the base view composed at the **unreduced** content width (a footer entry is cut mid-label rather than reflowed).
- [ ] The panel's list is constructed with filtering, title, status bar and help all disabled, and `SetSize` is fed the inner width and the computed body height.
- [ ] The panel's list delegate is built only through `m.themeRowDelegate()`, which takes its `Theme`, `Colourless` and `Width` from the model and the panel's current inner width — no second construction site exists.
- [ ] Rendering the same panel state under two different `Theme`s changes every chrome colour, with no surface holding a cached style.
- [ ] The colour-literal guard passes over `internal/tui` — no hex literal is introduced, and no new token is added to the closed 19.

**Tests**:
- `"it renders a full-height right-edge block"` — `TestThemePanel_BlockGeometry` (height/width table)
- `"it draws a left border only"` — `TestThemePanel_LeftBorderOnly`
- `"it costs two header rows and shows no count"` — `TestThemePanel_HeaderIsTwoRowsNoCount`
- `"it pins the directory row beneath the header"` — `TestThemePanel_DirUnreadableIsPinnedChrome` (asserted on page 2)
- `"it keeps rows rendering beneath the directory row"` — `TestThemePanel_RowsRenderBeneathDirRow`
- `"it fits the directory copy at the minimum width"` — `TestThemePanel_DirRowFitsMinimumWidthUntruncated`
- `"it reserves no message row when empty"` — `TestThemePanel_MessageSlotUnreservedWhenEmpty`
- `"it shrinks the list by one when a message is present"` — `TestThemePanel_MessageSlotRecomputesListHeight`
- `"it composites over the page without reflowing it"` — `TestThemePanel_OverlayDoesNotRelayoutTheBase`
- `"it cuts the covered footer mid-label"` — `TestThemePanel_OverlayCutsMidLabel`
- `"it uses only existing tokens"` — `TestThemePanel_EveryChromeSurfaceIsATokenLookup` (render under two themes, diff)
- `"it builds its delegate from one place"` — `TestThemePanel_DelegateHasASingleConstructionPoint`
- `"it drops the canvas under colourless"` — `TestThemePanel_Colourless`

**Edge Cases**:
- Non-blanking is not a preference but the only shape that can do the job — Portal's modals blank the page to canvas before drawing, so a modal theme picker would preview **nothing**, and every downstream constraint follows from that one fact.
- Full-height, right-edge, **left border only**, composited over the existing overlay mechanism with real z-layers, deliberately not an inset bordered panel like the modals.
- Body `canvas`, left border and header rule `border`, header label `Themes` in `accent.mode` — the reference frames' `#0C0C16` body and `#2B3050` border are **not adopted**, being per-frame literals that would need a 20th token whose only role is "slightly different from the canvas" and would reopen §2.1's closed count.
- The header costs exactly **two rows** (label + rule) and carries **no theme count**, which is what §9.8's floor arithmetic resolves against.
- The panel **does not animate** — open and close are one frame each, so no intermediate width renders a frame no fixture covers and `t`-then-`Esc` needs no race resolved.
- The `⚠ dir unreadable` row is **chrome pinned beneath the header, not a list delegate** — a list row participates in pagination and would vanish on page 2, and this row is what stands between the user and the "completely in the dark" state.
- Its copy is deliberately 16 columns so it fits the minimum width without truncation, none of the four composition priorities apply to it, and built-in and persisted-slug rows **still render beneath it** or a user with an unreadable directory loses the `●` entirely.
- The message slot is a single row directly above the footer, **not reserved when empty** — it appears and the list shrinks by one — and both its contenders are Phase 9's.
- The panel introduces a **third `bubbles/list` instance**, §11.2's worst case of the cached-style class.
- The delegate's three inputs (previewed `Theme`, `Colourless`, inner content width) are assembled in exactly one helper — `m.themeRowDelegate()` — which task 8-9's restyle path re-invokes. Two construction sites can disagree about width or colourlessness, and the disagreement is invisible until a resize during a preview.
- The main screen is deliberately **not** re-laid-out while the panel is open, so the overlay cuts wherever the left border falls, mid-label included (`x proje▏`) — not a §14.4 violation but the price of not reflowing the surface being previewed.
- The panel covers the right-hand column — the right-side header hint, session-row meta and the right end of the footer, including `t theme` itself — which is the least theme-informative part of the screen.
- Every panel surface resolves to an existing token, so the colour-literal guard and the swap-and-diff guard need no carve-out.

**Context**:
> §9.1: "A **full-height, right-edge, non-blanking overlay** with a **left border only** — deliberately *not* an inset bordered panel like the modals… **A modal was never available, and this is the constraint the whole shape follows from.**… **The panel body is painted `canvas`; the left border is `border`.** No new token… **Header.** The label is **`Themes`**, rendered in `accent.mode`… followed by a one-row `border` rule… **No theme count**… The header therefore costs **two rows**… **Message slot.** A single-row region directly above the vertical keymap footer, **not reserved when empty**."
> §9.5: "**An unreadable themes directory gets its own row** (§5.5) — `⚠ dir unreadable`, **chrome pinned to the viewport directly beneath the header, not a list row**… **Its copy is deliberately short (16 columns) so it fits the panel's minimum width without truncation**… **Built-in rows and persisted-slug rows still render beneath it** — the persisted rows especially, or a user with an unreadable directory loses the `●` entirely."
> §9.1: "**The overlay cuts wherever its left border falls, mid-label included** — `x proje▏`. That is not a violation of §14.4's 'never truncate a label'… **The main screen is deliberately not re-laid-out while the panel is open**, which is what keeps the swap the O(1) restyle of §11.1 and keeps the surface being previewed from reflowing under the user."
> Phase boundary: the phase note records that Phase 8 "leaves the message slot's height accounted for and empty" — both contenders (the slot-from-constant confirm and the failed-commit line) are commit-path states owned by Phase 9.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.1, §9.5, §9.8, §11.2, §11.3, §14.4

## theming-system-8-7

### Task 8.7: The constructor slot and `t` opening a panel that re-enumerates on every open

**Problem**: Phase 5 task 5-7 deliberately left the control-stripped raw persisted keys **unthreaded** — computed, then discarded with a comment naming this phase — because §8.4's "the constructor also takes the raw persisted theme keys" has no consumer until the panel exists. Every consumer lands here: a slug that never loaded is not in the nomination, a badge needs the *persisted* slug rather than the nomination's, and §14A's confirm renders the persisted constant. The panel also needs a directory read on a cadence nothing currently provides: enumerating once per process would break the loop the whole drop-in route exists for (copy a built-in, edit it, see it, without relaunching Portal), while enumerating at construction would pay an N-file sweep on a cold path this feature is otherwise careful to leave free. And the two config sources must be read on **deliberately opposite** cadences — the themes directory fresh on every open, `prefs.json` from the construction-time snapshot — because the directory is what the drop-in loop edits by hand while prefs is what Portal itself writes, and re-reading prefs would silently import another instance's commit, the cross-instance sync §8.4 declines.

**Solution**: Thread the deferred constructor slot (`Deps.ThemeKeys`, plus the per-slot resolution record the badges derive from) and the `ThemeEnumerator` seam through `Build` and `cmd/open.go`, bind `t` on Sessions and Projects with the filter carve-out, and have the keypress — not construction — perform the directory read, retaining the parse results for the panel's lifetime and discarding them on close.

**Outcome**: `t` on either page opens the panel over the live list; the panel's rows come from a directory read taken at that keypress; a second open after editing a file shows the edit; construction reads no directory and `portal open <target>` is untouched.

**Do**:
- **Add the constructor slots** in `internal/tui/build.go`: `Deps.ThemeKeys theme.RawKeys` (the control-stripped keys Phase 5 task 5-2 produces), `Deps.ThemeSlots []theme.SlotResolution` (Phase 5 task 5-4's per-slot record — the badge source task 8-3 derives from, injected because a nomination cannot express a slug that never loaded), and `Deps.ThemeEnumerator ThemeEnumerator`, each with the matching `WithThemeKeys` / `WithThemeSlots` / `WithThemeEnumerator` option beside `WithModePersister`. Hold all three on the model. Derive `Setting` where needed by calling `theme.ResolveSetting(keys.Theme, keys.Light, keys.Dark)` — idempotent on already-stripped input — rather than injecting a fourth value.
- **Wire production** in `cmd/open.go`: a small adapter closing over the same `theme.Loader` Phase 5 task 5-7 already constructs (built on `cmd`'s package-level `themeLogger` from task 3-2 — the **real** component logger, because the panel is a path where a theme is *used*) and over `themesDirPath()`, mirroring the `ScrollbackReader` adapter that closes over `stateDir` at TUI construction. Pass the `RawKeys` and `[]SlotResolution` the construction-time resolution already produced — replacing task 5-7's `_`-plus-comment placeholder. Doctor, `portal theme export` and `capturetool` keep `log.Discard`.
- **Bind `t`** in `updateSessionList` and `updateProjectsPage`, in both cases **after** the `SettingFilter()` guard so it is a literal filter character while `/` is focused — exactly as `s` already is. Theme is a global setting, which is why it is bound on Projects too.
- **On open**: call `m.themeEnumerator.Open(m.themeKeys)`, retain both returned values on the panel (`enumeration` and `union`), derive `badges` from the injected slots (task 8-3), build the panel's list from the union's rows (task 8-4's items and delegate) at task 8-6's preferred width, and set `open = true`. The directory read happens **here**, on the keypress — never at construction.
- **Retain, then discard**: the parse results live for the panel's lifetime so arrowing previews from values already in hand (task 8-9), and are dropped on close so the next open re-reads. Do **not** cache them on the model beyond the panel.
- **Do not re-read `prefs.json` on open.** Comment the asymmetry with the fresh directory read explicitly, naming both halves (the directory is what the drop-in loop edits by hand; prefs is what Portal itself writes, and re-reading would import another instance's commit — the cross-instance sync §8.4 declines). A user who hand-edits prefs mid-session sees it next launch, consistent with every other prefs consumer.
- **Route panel input minimally**: add an `updateThemePanel` arm ahead of the page dispatch that keeps `Ctrl-C` live (quit) and **swallows everything else**, plus a *provisional* `Esc` that clears the panel state. Mark the `Esc` body explicitly as provisional — **task 8-10 replaces it with the re-resolution close and is the real close path**; it is correct only at this point in the sequence because no arrow has yet previewed anything. Task 8-9 adds arrow handling and task 8-13 owns the entry conditions, the blocked-`t` flashes and the tested rationale for key-exclusivity.
- **Nil-guard the seam** following the `modePersister` precedent: a nil `ThemeEnumerator` (a fixture or `capturetool` model that wires none) makes `t` a silent no-op rather than a panic, and a typed-nil concrete value boxed into the interface must not be mistaken for a live seam.
- **Assert the negative**: construction enumerates nothing — no `ReadDir` on any construction path, with `PORTAL_THEMES_DIR` pointing at a populated directory — and `portal open <target>` still constructs no TUI, reads no prefs and does no theme work.

**Acceptance Criteria**:
- [ ] `t` on Sessions and on Projects opens the panel; `t` while `/` is focused inserts a literal `t` into the filter query on both pages.
- [ ] The themes directory is read **on the keypress**: a construction with a populated `PORTAL_THEMES_DIR` performs zero directory reads until `t` is pressed.
- [ ] Each open performs exactly one directory read and emits exactly one `theme: enumerated`; three opens emit three.
- [ ] A file edited between two opens is reflected in the second open's rows without relaunching.
- [ ] The retained enumeration is discarded on close — the next open re-reads (asserted by mutating the directory between close and re-open).
- [ ] `prefs.json` is **not** read on open: rewriting the file between construction and `t` changes nothing the panel shows.
- [ ] Badges render on the first open from the injected `[]SlotResolution` with no additional resolution work.
- [ ] A nil `ThemeEnumerator` makes `t` a silent no-op with no panic; a typed-nil concrete value behaves identically.
- [ ] While the panel is open every key except `Ctrl-C` and the provisional `Esc` is swallowed — `k` does not kill, `x` does not switch page, `m` does not enter multi-select.
- [ ] `cmd` passes a real `theme` component logger on this path; a `capturetool` / fixture model reaches no config and both `internal/capture`'s no-real-config import guard and `TestPortalBinaryDoesNotImportCapture` stay green.
- [ ] `portal open <target>` performs no theme work (no directory read, no prefs read, no `theme` record).

**Tests**:
- `"it opens the panel from Sessions and Projects"` — `TestThemePanelOpen_BoundOnBothPages`
- `"it treats t as a filter character while filtering"` — `TestThemePanelOpen_FilterCarveOut` (both pages)
- `"it reads the directory on the keypress, not at construction"` — `TestThemePanelOpen_NoEnumerationAtConstruction`
- `"it re-enumerates on every open"` — `TestThemePanelOpen_ReEnumeratesPerOpen` (three opens, three reads, three events)
- `"it picks up a mid-session file edit on the next open"` — `TestThemePanelOpen_SeesAMidSessionEdit`
- `"it discards the enumeration on close"` — `TestThemePanelOpen_EnumerationDiscardedOnClose`
- `"it does not re-read prefs on open"` — `TestThemePanelOpen_UsesConstructionTimePrefsSnapshot`
- `"it renders badges from the injected slot record"` — `TestThemePanelOpen_BadgesFromInjectedSlots`
- `"it tolerates a nil enumerator"` — `TestThemePanelOpen_NilSeamIsASilentNoOp` (nil interface and typed-nil)
- `"it swallows every key but Ctrl-C while open"` — `TestThemePanelOpen_SwallowsPageKeys`
- `"it does no theme work on the exec path"` — `TestThemePanelOpen_ExecPathUntouched` (poisoned themes dir + `logtest.Sink`)

**Edge Cases**:
- Phase 5 task 5-7 deliberately left the control-stripped raw keys unthreaded — this is the constructor slot it deferred, and every consumer of it lands in this phase.
- The seam is `ThemeEnumerator`-shaped, matching the `TmuxEnumerator` / `ScrollbackReader` idiom the preview page already uses — production wires the real implementation, fixtures fake it.
- Enumeration runs on **every open**, never once per process, because caching buys nothing measurable while breaking the loop the drop-in route exists for (copy a built-in, edit it, see it, without relaunching).
- The parse results are **retained for the panel's lifetime** so arrowing previews from values already in hand, and are discarded on close so the next open re-reads.
- The panel uses the **construction-time prefs snapshot** and does **not** re-read `prefs.json` on open — the deliberate asymmetry with the fresh directory read, since re-reading would import another instance's commit (the cross-instance sync §8.9 declines).
- `t` is bound on Sessions **and** Projects because theme is a global setting, with the filter carve-out making it a literal character while `/` is focused, exactly as `s` already is.
- `cmd` passes a **real** `theme` component logger here — the panel is a path where a theme is *used* — while doctor, export and `capturetool` keep `log.Discard` and their dedup state owned.
- The production enumerator takes its directory from `themesDirPath`, the loader still never resolving it.
- A fixture / `capturetool` model gets a faked seam and reaches no config, so `internal/capture`'s no-real-config import guard and `TestPortalBinaryDoesNotImportCapture` both stay green.
- A nil seam must not panic, following the `modePersister` nil-guard precedent.
- Construction still enumerates nothing — the directory read happens on the keypress — and `portal open <target>` remains untouched.
- The `Esc` landed here is **provisional** (it clears panel state and does no theme work, which is correct only because nothing has previewed yet); task 8-10 replaces its body with the re-resolution close, which is the real close path.

**Context**:
> §5.7: "**At construction**, Portal loads **only the nominated themes by name**… No enumeration. **Enumeration happens only when the slide-over opens**, where a few milliseconds is invisible against the keypress that opened it."
> §5.8: "The directory is enumerated **on every panel open**, not once per process… caching buys nothing measurable while breaking the loop the drop-in route exists for — copy a built-in, edit it, see it, without relaunching Portal. **The enumeration's parse results are retained for the panel's lifetime**… They are discarded when the panel closes; the next open re-reads."
> §8.4: "**The panel uses the construction-time prefs snapshot; it does not re-read `prefs.json` on open.** This is a deliberate asymmetry with §5.8's fresh directory read, and the two are asymmetric because the files are… Re-reading it would let another instance's commit silently change what this panel shows and marks — the cross-instance sync §8.9 explicitly declines."
> §9.6: "**Key: `t`** — free on Sessions… | **Projects** | Yes — theme is a *global* setting; refusing would make it feel page-scoped for no reason… **`t` needs the filter carve-out** — while `/` is focused it is a literal filter character, exactly as `s` already is."
> Phase 5 task 5-7 (Do): "**Do not** thread `rawKeys` onto the model yet — Phase 8 owns the constructor slot and every consumer (badges, the `not found` / charset-rejected rows, §14A's confirm)."
> **Ambiguity flagged**: §8.4 pins the raw keys as a constructor input but does not name the badge derivation's input explicitly; the plan's task 8-3 edge case does ("the raw persisted keys plus Phase 5 task 5-4's per-slot resolution record"). Injecting `Deps.ThemeSlots` is the minimal way to satisfy that at the *first* open, before task 8-8's open-time re-resolution exists to produce a fresher record. Record the choice in a source comment so task 8-8 replaces rather than duplicates it.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §5.7, §5.8, §8.4, §9.6, §12.3, §13.3

## theming-system-8-8

### Task 8.8: Opening lands the cursor on the theme actually rendering

**Problem**: Where the cursor lands on open decides whether the panel is a picker or a puzzle. Landing it at index 0 breaks the invariant everything else rests on — *the cursor's row is always what is painted behind the panel* — so the first arrow keypress would jump the canvas to a theme the user never approached. Landing it on the **persisted** row when that row is broken is worse: arrows skip unselectable rows, so the cursor would sit somewhere navigation cannot return to, while showing a row that is emphatically not what is on screen. And the open is not a passive read: §5.8 makes the panel's fresh parse supersede the construction-time one, so opening must apply a mid-session file edit — including one that **invalidates** the active theme, which must flip to the fallback on *open* rather than being deferred to `Esc`, since deferring would leave the panel listing a theme as invalid while the screen still renders it. None of that is reachable without a resolution that runs against the panel's **retained** enumeration rather than the filesystem, which does not yet exist.

**Solution**: Extend the `ThemeEnumerator` seam with a `Resolve` method backed by a new `internal/theme` entry point that re-runs Phase 5's per-slot resolution and fallback against a retained `Enumeration` with **no** directory read; on open, use its result to refresh the badges, apply the resolved active theme through `Model.ApplyTheme`, and anchor the cursor to that theme's **row identity**.

**Outcome**: Opening the panel puts the cursor on the row that is painting the screen in all four cases (constant, in-force slot, `● both`, fallback), previews nothing, applies an edited-but-still-valid active theme's new values, and flips to the fallback when the active theme's file has been broken mid-session — with the `●` still on the persisted slug.

**Do**:
- **Add `func (l Loader) ResolveNominationFrom(e Enumeration, s Setting) (Resolution, error)`** to `internal/theme`, beside Phase 5 task 5-4's `ResolveNomination`. Same rules exactly — charset check, **embedded set first**, then the source, per-slot mode-matched fallback, structured `[]SlotResolution`, never writes prefs, a failing *fallback* returns task 5-6's fatal — with one difference: the "then" source is the **enumeration's entries**, not the directory. Factor the shared body so the two entry points cannot drift. Comment that Phase 9's mid-session slot load and task 8-10's `Esc` both reuse it, and that issuing a directory read here would produce a *third* parse of the same slug that can disagree with the row the user is looking at, reintroducing the staleness split §5.8 exists to close.
- **Extend the seam** (task 8-1) with `Resolve(e theme.Enumeration, s theme.Setting) (theme.Resolution, error)` and wire the production adapter and the fixture fake to it.
- **Pin the error policy once, here.** `Resolve` returns task 5-6's fatal only when a *fallback* cannot resolve within the embedded set, which Phase 2 task 2-8's build-time guarantee makes unreachable in a correctly built binary. The panel therefore **degrades rather than escalating**: on a non-nil error leave the badges, the active theme and the cursor exactly as they were, carry on with the union already in hand, and write nothing — a settings surface must not become the route by which a broken binary quits Portal mid-session, and §7.6 puts the fatal on the *startup* path deliberately. State in-source that this one policy governs **every** panel call site of `Resolve` — this task's open, task 8-10's close and task 9-2's recompute — so the three cannot each invent their own.
- **On open, after the enumeration**: derive `Setting` from the raw keys, call `Resolve`, and use the returned `Resolution` for three things — refresh `badges` from its `Slots` (task 8-3), select the in-force member, and anchor the cursor. Replace task 8-7's injected `[]SlotResolution` as the panel's badge source from this point on (the injected value remains the pre-open truth for construction).
- **Select the in-force member**: under a constant, the constant; under a pair, the light member iff the gate's resolved light/dark answer is light, else dark — the gate's **single** resolution with the standing dark no-answer fallback (the unexported `internal/tui` concept Phase 3 task 3-1 kept). Never re-run detection.
- **Apply it**: if the selected theme differs from `m.activeTheme`, call `Model.ApplyTheme` (Phase 4 task 4-2's production entry point) — never a rebuild, never a direct field assignment. When the file has not changed this is a no-op, so **opening never changes which theme is shown**.
- **Anchor the cursor to a row identity, not an index.** Add a helper that finds the union row whose identity matches a slug (slug where one exists, else filename, else the persisted string — the same key `Row.SortKey()` derives from) and sets the list index to it. The target is the **resolved** slug (`SlotResolution.Resolved`) — the fallback's slug when the slot fell back, which is why the cursor lands on the fallback's row and not the persisted-but-broken one. Comment that anchoring to an index would silently break the invariant the moment a row is inserted above the cursor, which is exactly what Phase 9's commit recompute does.
- **Degrade, never panic**: if the identity is not found, or the union is somehow empty, clamp to the first **selectable** row and then to index 0 rather than indexing out of range. Note that built-ins are always valid (Phase 2 task 2-8), so a fully-unselectable list is unreachable — the clamp is a structural guard, not a live path.
- **Assert the invariant that survives every case**: the cursor is on a selectable row **and** that row is what is painted behind the panel. Make this a directly named test rather than an implication of the four cases.
- Write **nothing**: no prefs write, no `@portal-*` option, no file. Opening is a read plus a restyle.

**Acceptance Criteria**:
- [ ] Under a **constant**, the cursor lands on the constant's row.
- [ ] Under a **pair**, the cursor lands on the in-force slot's row (light in a light terminal, dark otherwise) while the *other* slot's row still carries its badge.
- [ ] When both slots name the same slug the cursor lands on the single `● both` row.
- [ ] When the resolved theme is a **fallback**, the cursor lands on the **fallback's** row, the persisted row is present and unselectable with its reason, and the `●` is still on the persisted slug.
- [ ] Opening with an unchanged directory leaves the rendered theme byte-identical — `ApplyTheme` either is not called or is called with the active theme.
- [ ] Editing the active theme's file to new but **valid** values and then opening re-renders the same slug with the new values, with no arrowing required.
- [ ] Editing the active theme's file to make it **invalid** and then opening flips the screen to the §8.5 fallback **on open**, with the persisted row unselectable and reasoned.
- [ ] `Resolve` performs **no** directory read (proven with the directory removed after the enumeration) and writes nothing to `prefs.json`.
- [ ] The cursor is anchored by identity: inserting a row above the target before the anchor runs still lands the cursor on the target.
- [ ] With an identity absent from the union the cursor clamps to the first selectable row with no panic and no out-of-range index.
- [ ] A `Resolve` returning task 5-6's fatal leaves the badges, the active theme and the cursor unchanged, still opens the panel on the union already in hand, writes nothing and does not quit Portal — driven through the seam with an error-returning fake.
- [ ] After every open case the cursor's row is selectable **and** equals the theme applied to the model.
- [ ] The in-force answer comes from the gate's existing single resolution — no OSC 11 query is issued on open.

**Tests**:
- `"it lands on the constant's row"` — `TestPanelOpenCursor_Constant`
- `"it lands on the in-force slot's row"` — `TestPanelOpenCursor_InForceSlot` (light terminal and dark terminal)
- `"it lands on the both row"` — `TestPanelOpenCursor_BothSlotsSameSlug`
- `"it lands on the fallback's row, not the broken one"` — `TestPanelOpenCursor_FallbackRow`
- `"it leaves the badge on the persisted slug"` — `TestPanelOpenCursor_BadgeStaysOnPersisted`
- `"it changes nothing when the directory is unchanged"` — `TestPanelOpen_DoesNotChangeTheRenderedTheme`
- `"it applies an edited-but-valid active theme on open"` — `TestPanelOpen_AppliesMidSessionEdit`
- `"it flips to the fallback on open when the active theme is broken"` — `TestPanelOpen_InvalidatedActiveThemeFlipsOnOpen`
- `"it resolves against the retained enumeration with no directory read"` — `TestResolveNominationFrom_ReadsNothing`
- `"it anchors the cursor by identity, not index"` — `TestPanelOpenCursor_AnchoredByIdentity`
- `"it degrades rather than indexing out of range"` — `TestPanelOpenCursor_DegradesOnMissingIdentity`
- `"it degrades rather than escalating an unresolvable fallback"` — `TestPanelOpen_ResolveErrorDegrades`
- `"it keeps the cursor on a selectable row that is what is painted"` — `TestPanelOpen_CursorInvariant` (table over all four cases)
- `"it issues no new detection query"` — `TestPanelOpen_NoNewOSC11Query`
- `"it writes nothing"` — `TestPanelOpen_WritesNothing` (byte-compare `prefs.json`; absent file stays absent)

**Edge Cases**:
- Four opening cases, each specified: the **constant's** row; the **in-force slot's** row (light in a light terminal, dark otherwise) with the other slot's row still carrying its badge; the single **`● both`** row; and, when the resolved theme is a **fallback**, the **fallback's** row rather than the persisted-but-broken one.
- Parking the cursor on the broken row would put it somewhere navigation cannot return to (arrows skip unselectable rows) and would show a row that is not what is on screen.
- The `●` **stays on the persisted slug** while the cursor sits on the fallback — exactly the split §9.5 draws, `●` being what is *set* and the cursor what is *previewed*.
- Because the cursor starts on what is already rendering, **opening never changes which theme is shown** and the mixed-mode flash fires only on deliberate navigation.
- Opening **can** change that theme's *values* and that is correct — §5.8's fresh enumeration supersedes the construction-time parse, so an edited-and-still-valid active theme re-renders with its new values on open; making the user arrow away and back would be a bug wearing a rule's clothing.
- An edit that **invalidates** the active theme resolves §8.5's fallback and flips on **open**, never deferred to `Esc`, because deferring would leave the panel listing a theme as invalid while the screen still renders it.
- The invariant surviving both cases: the cursor is always on a selectable row and that row is always what is painted behind the panel.
- The in-force slot comes from the gate's **single** resolution with the standing dark no-answer fallback; no new query is issued.
- The cursor is anchored to a row **identity**, not an index — the property Phase 9's commit recompute rests on.
- Built-ins are always valid so a fully-unselectable list is unreachable, but the anchor must degrade rather than index out of range.
- `Resolve` can return task 5-6's fatal, which the build-time guarantee makes unreachable in a correctly built binary — the panel **degrades** (badges, active theme and cursor untouched, nothing written) rather than escalating, because a settings surface must not be the route by which a broken binary quits mid-session. One policy governs all three panel call sites: this task's open, task 8-10's close and task 9-2's recompute.
- Resolution runs against the **retained enumeration**, never the filesystem — a commit-time or open-time directory read would produce a third parse that can disagree with the row the user is looking at.

**Context**:
> §9.2: "**Opening state: the cursor lands on the theme that is actually rendering, and opening previews nothing.** Under a **constant**, that is the constant's row. Under an **adaptive pair**, it is the row for the slot currently in force… When both slots name the same slug there is one row carrying `● both`, and the cursor is on it. When the resolved theme is a **fallback** (§8.5), the cursor lands on the **fallback's** row, not on the persisted-but-broken one… The `●` still marks the persisted slug, which is exactly the split §9.5 draws: `●` is what is *set*, the cursor is what is *previewed*."
> §9.2: "**Edited and still valid** — the panel re-renders the *same slug* with its new values on open… **Edited and now invalid** — the active theme is no longer loadable, so opening resolves the §8.5 fallback and the cursor lands on the fallback's row… The flip happens on **open**, not deferred to `Esc`… The invariant that survives both cases: **the cursor is always on a selectable row, and that row is always what is painted behind the panel.**"
> §8.4: "**A stale hand-edited slot** resolves from the panel's retained enumeration (§5.8), which already parsed and classified every file in the directory when the panel opened… **No commit-time directory read.** Issuing one would produce a *third* parse of the same slug — neither construction's nor the panel's — that can disagree with the row the user is looking at."
> §9.2: "…**re-anchors the cursor to the previewed theme's identity, never to its index**. Anchoring to an index would silently break §9.2's invariant the moment a row is inserted above the cursor."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.2, §9.4, §9.5, §8.4, §8.5, §5.8, §8.8
# Phase 8: The slide-over panel — tasks 8-9 … 8-16

## theming-system-8-9

### Task 8.9: Arrowing re-themes the app and the panel through the restyle path

**Problem**: The panel now exists (task 8-6), opens on a keypress (task 8-7) and lands its cursor on the theme that is actually rendering (task 8-8) — but arrowing does nothing. Live preview *is* the feature: a panel that lists themes without showing them is a config screen with extra steps. Three distinct traps sit on this one keypress. **Driving the swap through anything but the production restyle path**: a rebuild would fire `rebuildSessionList`'s lazy per-session tmux pane reads (the known ~0.5s By-Project cost at ~38 sessions) on *every* arrow, and a test-only setter would leave §13.4's guard exercising a path production never takes. **Reading the file per keystroke**, which turns §11.1's O(1) restyle into I/O on the one surface whose theme changes on every key. And **the panel's own `bubbles/list` instance** — §11.2 names it the worst case of the cached-style class precisely because its styles are assigned once at open while its theme changes on every arrow, so a swap that re-points only the two main-screen lists leaves the panel rendering the previous theme's pagination dots on top of the new canvas. Finally the cursor must step past unselectable rows, or §9.2's invariant ("the cursor is always on a selectable row, and that row is always what is painted behind the panel") breaks the moment one invalid theme is listed.

**Solution**: An arrow/paging arm in `updateThemePanel` that drives the panel's list, applies an invalid-row skip modelled on the existing group-header skip, then previews the newly selected row's **already-parsed** `Theme` through `Model.ApplyTheme` — with `applyCanvasMode` extended to re-point the panel's `bubbles/list`-owned styles and its delegate alongside the two main-screen lists.

**Outcome**: `↑`/`↓` and `Ctrl+↑`/`Ctrl+↓` move the panel cursor, skipping invalid rows and composing with paging; every landing re-themes the entire frame — main screen *and* panel chrome — with zero file reads, no list rebuild, no movement of `startupCanvasHex`, and nothing written anywhere.

**Do**:
- **Add the arrow arm** to task 8-7's `updateThemePanel`, ahead of the swallow-everything default: route `↑`, `↓`, `Ctrl+↑`, `Ctrl+↓` (matched against the panel list's own `KeyMap`, the same way the Sessions page matches `CursorUp`/`CursorDown`/`PrevPage`/`NextPage`) into `p.list.Update(msg)`. Every other key stays swallowed except `Ctrl-C` (quit) and `Esc` (task 8-10). Per §12.2's arrow-only revision the panel binds **no** vim aliases, no `PgUp`/`PgDn`/`Home`/`End` and no uppercase forms — the panel's list keymap is pinned the same way `pinArrowOnlyNav` pins the other two.
- **Add `skipUnselectableThemeRow(msg tea.KeyPressMsg)`** in `internal/tui/theme_panel.go`, modelled on `model.go`'s `skipHeaderRow` and documented as reusing that mechanism: after the list has processed the key, while the selected item's `Row.Selectable()` is false, step once more in the direction of travel (up for `CursorUp`/`PrevPage`, down otherwise). **Two deliberate differences from `skipHeaderRow`**, both stated in-source: unselectable rows **can be adjacent** (several broken drop-ins in one directory), so the step is a bounded *loop* rather than a single step; and on reaching a boundary with no selectable row in the direction of travel the loop **reverses** rather than falling off the list. Built-ins are always valid (Phase 2 task 2-8), so at least one selectable row always exists and the loop always terminates — bound the loop by the row count regardless, so a future all-invalid union cannot spin.
- **Preview from values already in hand**: after the skip, read the selected `themeRowItem.Row.Theme` — parsed by the enumeration at open and retained for the panel's lifetime (task 8-7) — and call `m.ApplyTheme(th)` (Phase 4 task 4-2's production entry point) when it differs from `m.activeTheme`. **No file read, no `Reassemble`, no directory access, no re-resolution** on an arrow keypress. This is what keeps the swap §11.1's O(1) restyle.
- **Extend the restyle path to the third list.** In `applyCanvasMode` (both the colourless and the coloured branch), when the panel is open also re-point the panel list's `bubbles/list`-owned styles — help styles, pagination dots, `TitleBar`/`Title` — through the same `canvasHelpStyles` / `canvasPaginationDots` / `colourless*` helpers the main lists use, and `p.list.SetDelegate(m.themeRowDelegate())` so task 8-4's delegate carries the previewed `Theme`. State in-source that this is **not** a rebuild: no item is re-derived, no content is touched — §11.1 rules the rebuild out as the expensive path and it would be worse here, on a per-keypress surface.
- **Assert the negatives, not just the positive.** A preview swap must: not call `rebuildSessionList`; perform no `DirReader`/pane read and no file read; leave `startupCanvasHex` untouched (the anchor Phase 4 task 4-5 rests on); and write nothing — no `prefs.json`, no tmux option, no file. Every one of these is a named test, not a comment.
- **Do nothing about OSC 11.** `View()` assigns `v.BackgroundColor` declaratively from the active theme's canvas and Bubble Tea **diffs** it, so hovering N themes emits exactly once per *distinct* canvas landed on. The query is issued only from `Init`, so a later switch creates no new race and the canvas-echo guard needs no new handling. Record this as a deliberate no-op so nobody adds a suppression or a debounce.
- **The mixed-mode flash is the feature.** Arrowing past a light theme in a dark terminal flips the whole canvas near-white and back; add no ordering mitigation (same-mode-first was proposed and rejected, §9.2), no suppression and no transition. A colourless model stays colourless across a preview swap — no hue leaks through the re-point.

**Acceptance Criteria**:
- [ ] `↑`/`↓` move the panel cursor one row and `Ctrl+↑`/`Ctrl+↓` page it; no other key moves it, and no vim alias, `PgUp`/`PgDn` or `Home`/`End` is bound.
- [ ] Landing on a row applies that row's `Theme`: the rendered main screen **and** the panel chrome both change colour, asserted as a diff of the composed frame across one arrow.
- [ ] The cursor never rests on an unselectable row: arrowing down through a block of three consecutive invalid rows lands on the next valid one, and arrowing up past the top-most invalid row reverses rather than falling off.
- [ ] The skip composes with paging exactly as the group-header skip does: `Ctrl+↓` onto a page whose first row is invalid lands on a selectable row on that page.
- [ ] A preview swap performs **zero** file or directory reads (asserted with the themes directory removed after the panel opened).
- [ ] A preview swap does not call `rebuildSessionList` and triggers no `DirReader` pane read (counted seam records zero).
- [ ] `startupCanvasHex` is byte-identical after one swap and after fifty.
- [ ] `prefs.json` is byte-identical after any number of arrow keypresses, and an absent `prefs.json` stays absent.
- [ ] The panel's pagination dots, help styles and delegate all render in the previewed theme after a swap — asserted on a union large enough to paginate, so the dots are actually drawn.
- [ ] A colourless model emits no hue and no background SGR after a preview swap.
- [ ] Arrowing onto the row that is already active is a no-op frame (identical output), and repeated swaps are idempotent per swap.

**Tests**:
- `"it moves the cursor on arrows and pages on ctrl-arrows"` — `TestPanelArrow_NavigationBindings`
- `"it binds no vim alias or page-jump key"` — `TestPanelArrow_ArrowOnlyNavigation`
- `"it re-themes the main screen and the panel together"` — `TestPanelArrow_PreviewsThroughApplyTheme`
- `"it skips a block of adjacent invalid rows"` — `TestPanelArrow_SkipsConsecutiveInvalidRows`
- `"it reverses rather than falling off the list"` — `TestPanelArrow_SkipReversesAtTheBoundary`
- `"it composes the skip with paging"` — `TestPanelArrow_SkipComposesWithPaging`
- `"it reads no file per keystroke"` — `TestPanelArrow_NoFileReadPerKeystroke` (directory removed after open)
- `"it does not rebuild the session list"` — `TestPanelArrow_DoesNotRebuildSessionList`
- `"it never moves the startup canvas hex"` — `TestPanelArrow_StartupCanvasHexUnmoved` (1 swap and 50)
- `"it writes nothing on an arrow"` — `TestPanelArrow_WritesNothing`
- `"it re-points the panel's own list styles"` — `TestPanelArrow_PanelListStylesRepointed` (paginating union, dots diffed)
- `"it keeps a colourless model colourless"` — `TestPanelArrow_ColourlessStaysColourless`
- `"it treats an unchanged selection as a no-op"` — `TestPanelArrow_SameRowIsANoOp`

**Edge Cases**:
- The swap goes through Phase 4 task 4-2's `Model.ApplyTheme` — the production restyle entry point the panel was always specified to drive — **never** a rebuild and never a test-only setter.
- **No file read per keystroke**: the preview renders from the retained enumeration's already-parsed values, which is what keeps the swap the O(1) restyle of §11.1.
- The panel's own chrome re-themes with the previewed theme, **no exceptions** — a fixed panel shows a theme that cannot be fully judged, and a surface that deliberately ignores the active theme is precisely the carve-out §13.4's guard exists to catch.
- The panel's `bubbles/list`-owned styles (pagination dots, its own help/title styles) are re-pointed by **the same restyle path as the main list**, extended to cover the third instance — not a rebuild, which §11.1 rules out and which would be worse on a per-keypress surface.
- The panel's delegate re-derives per frame so nothing is cached.
- Arrows **skip invalid rows**, reusing the mechanism that already skips group-header rows, and the skip composes with `Ctrl+↑`/`Ctrl+↓` paging exactly as the group-header skip already does. Unlike group headers, invalid rows can be adjacent, so the skip loops.
- `rebuildSessionList` and its lazy per-session pane reads must not fire, and `startupCanvasHex` must not move on any swap.
- The **mixed-mode flash is the feature, not a defect** — arrowing past a light theme in a dark terminal flips the whole canvas near-white and back, which is what live preview is for.
- OSC 11 is emitted once per **distinct** canvas landed on because Bubble Tea diffs the declarative background, and the query is issued only from `Init`, so a later switch creates no new race and the echo guard needs no new handling.
- A colourless model stays colourless across a preview swap.
- **Nothing is written on any arrow keypress.**

**Context**:
> §9.2: "`↑` / `↓` — Move the cursor. **The app re-themes live behind the panel. Nothing is written.** | `Ctrl+↑` / `Ctrl+↓` — Page, per MV spec §12.2."
> §11.1: "**Restyle** — `applyCanvasMode` swaps the delegate and re-points the cached style structs `bubbles/list` holds. O(1), no I/O, no list content touched… from here its callers are the panel's **arrow-preview**, its **open**… and its **close**… **`applyCanvasMode` does not call `rebuildSessionList`.** Nothing heavy is on the theme-swap path."
> §11.2: "**The panel introduces a third `bubbles/list` instance, and it is the worst case of this class.**… **The `bubbles/list`-owned styles the panel uses** (pagination dots, its own help/title styles) are re-pointed by **the same restyle path as the main list**, extended to cover the panel's instance."
> §11.3: "**No per-keystroke churn.** Bubble Tea v2 **diffs** the view's background colour and emits only on change, so hovering N themes emits OSC 11 exactly once per *distinct* canvas landed on… **The echo guard needs no new race handling.**"
> §9.5: "**Arrow keys skip invalid rows**, reusing the mechanism that already skips group-header rows on the Sessions list. The skip composes with paging exactly as the group-header skip already does."
> §5.8: "**The enumeration's parse results are retained for the panel's lifetime**, so arrowing previews from values already in hand — no file read per keystroke, which is what keeps the swap the O(1) restyle of §11.1."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.2, §9.5, §9.11, §11.1, §11.2, §11.3, §5.8

## theming-system-8-10

### Task 8.10: `Esc` discards the preview onto the resolved persisted state

**Problem**: Task 8-7 left a **provisional** `Esc` that merely clears panel state — correct only at that point in the sequence, because nothing had previewed yet. Now arrowing re-themes the app (task 8-9), so closing must undo a preview the user explicitly discarded, and the naive implementation — snapshot the theme at open, restore it at close — is wrong in *two* directions. It is wrong when the user broke the active theme's file mid-session (the snapshot restores a palette the config no longer yields, so Portal shows a stale copy it happens to still hold rather than what the config now says), and it is wrong forward: Phase 9's commits land on exactly this close path, and `Esc` after a commit must resolve to the **newly** persisted state, not to what was rendering when the panel opened. §11.1 also names the close as the caller that matters most — a missed style re-point there leaves a discarded preview painting the main screen, with no surface left open to explain it. And `Esc` is the *only* way out (`Enter` deliberately does not close), so this one path is also where §9.8's forced close and Phase 9's failed-commit flash must attach.

**Solution**: Replace the provisional body with a single `closeThemePanel` that **re-resolves persisted state** against the panel's retained enumeration, applies the resolved active theme through `Model.ApplyTheme`, then discards the panel's retained state and re-syncs the page layout — one function serving `Esc`, the forced close, and Phase 9's hooks.

**Outcome**: `Esc` closes the panel and leaves the screen painting whatever the persisted setting now resolves to — the same theme as before when nothing changed, the §8.5 fallback when the user broke the active theme's file mid-session, and (from Phase 9) the newly committed theme when a commit landed. The retained enumeration is dropped, so the next open re-reads. Nothing is written.

**Do**:
- **Add `func (m *Model) closeThemePanel()`** in `internal/tui/theme_panel.go`, and point task 8-7's provisional `Esc` at it. Steps, in this order:
  1. Derive `Setting` from the model's raw keys: `theme.ResolveSetting(keys.Theme, keys.Light, keys.Dark)` — idempotent on already-stripped input, the same single tiebreak site tasks 8-1 and 8-8 use.
  2. Call the seam's `Resolve(p.enumeration, setting)` (task 8-8's `ResolveNominationFrom`) — resolution runs **against the retained enumeration**, never the filesystem, so it agrees with the rows the user was just looking at and issues no third parse. A non-nil error takes task 8-8's degrade policy: skip steps 3 and 4, leaving the active theme exactly as it is, and fall through to step 5 so the panel still closes.
  3. Select the in-force member exactly as task 8-8 does — the constant under a constant, else the light member iff the gate's **single** resolved answer is light, else dark. Never re-run detection.
  4. `m.ApplyTheme(selected)` — the same production restyle path every other caller drives. When nothing changed this is a no-op.
  5. **Then** discard: `open=false`, zero `enumeration`, `union`, `badges` and `message`, and re-sync the active page's layout so the list reclaims the panel's frame. Order matters — resolution reads the enumeration, so the discard is last.
- **State the anti-pattern in-source**: `Esc` does **not** restore a theme snapshotted at open. Name both directions it would be wrong (a mid-session edit that invalidates the active theme; a Phase 9 commit that changes what persisted state resolves to) so a later reader does not "simplify" it back into a snapshot.
- **Pin the emission policy of the retained-enumeration resolver** (it is re-called on every open and every close, so cadence matters, and this is the task where the repetition becomes real):
  - `theme: fallback applied` **does** fire — §12.3 explicitly names "again on every panel open and again on every `Esc`" as the reason it is deduplicated per process on `slug`+`reason`, so a persistently broken active theme produces one WARN, not one per close.
  - `theme: loaded` does **not** fire on this path. Its catalogued cadence is construction (one line per nomination) plus the one commit-time load outside construction, and it is deliberately **not** deduplicated — emitting per open/close would turn a per-load INFO into the running commentary the neighbouring dedup rules exist to prevent. Wire the emission onto the construction and commit entry points, not onto the shared body, and carry a source comment saying so (this governs task 8-8's open path too, which calls the same resolver — one policy, one place).
- **Discard the enumeration on close** so the next open re-reads the directory (§5.8's whole point, and what makes fixing a previously-invalid theme take effect without relaunching). Do not retain it "as a cache" on the model.
- **One close path, no second one.** The forced close (task 8-11) calls `closeThemePanel` and then raises its flash; Phase 9's `⚠ theme not saved — see portal.log` flash and its outstanding-failure discharge attach to this same function. Leave a named hook point (a single post-close step) rather than letting Phase 9 fork the path.
- **Key-exclusivity holds through the close**: `Esc` is consumed by the panel and never reaches the page beneath — it must not clear an applied filter, must not exit multi-select, and must not quit. Closing over multi-select returns to multi-select with the marked set intact (`Esc` resolves innermost-first, the rule modals already follow).
- **Write nothing.** No prefs write, no tmux option, no file — every write is an explicit keypress (Phase 9), which is what eliminates the "applied but not persisted" state persist-on-close would reach. Closing is one frame: no animation, no transition.

**Acceptance Criteria**:
- [ ] With nothing changed, `Esc` after arrowing three rows away restores the exact frame that was painted before the panel opened (byte-compare the composed view).
- [ ] With the active theme's file edited mid-session to **new but valid** values, `Esc` renders the new values — not the palette held at open.
- [ ] With the active theme's file **invalidated** mid-session, `Esc` renders the §8.5 fallback (the panel already flipped on open per task 8-8, so this asserts the close resolves rather than restores).
- [ ] The close resolves through the **retained enumeration**: with the themes directory removed after the panel opened, `Esc` still resolves and reads nothing.
- [ ] `Esc` discards the enumeration — the next open performs a fresh directory read (asserted by mutating the directory between close and re-open).
- [ ] `Esc` writes nothing: `prefs.json` byte-identical, an absent file stays absent, no tmux option set.
- [ ] Ten open/close cycles against a persistently broken active theme emit exactly **one** `theme: fallback applied` and **zero** `theme: loaded` records.
- [ ] `Esc` with a filter applied on the page beneath closes the panel and leaves the filter applied; `Esc` inside multi-select closes the panel and leaves the marked set and the banner intact.
- [ ] `Esc` does not quit Portal on either page.
- [ ] The panel's list, delegate, badges and message are all cleared on close, and the page list is re-sized so it reclaims the full frame.
- [ ] Closing is one frame — no intermediate width, no animation state.

**Tests**:
- `"it restores the pre-open frame when nothing changed"` — `TestPanelClose_DiscardsThePreview`
- `"it renders an edited-but-valid theme's new values"` — `TestPanelClose_ResolvesEditedValues`
- `"it lands on the fallback when the active theme was invalidated"` — `TestPanelClose_ResolvesToFallback`
- `"it resolves against the retained enumeration"` — `TestPanelClose_ReadsNothing` (directory removed after open)
- `"it discards the enumeration so the next open re-reads"` — `TestPanelClose_EnumerationDiscarded`
- `"it writes nothing on close"` — `TestPanelClose_WritesNothing`
- `"it emits one fallback record across many closes and no loaded record"` — `TestPanelClose_EventCadence`
- `"it leaves an applied filter alone"` — `TestPanelClose_DoesNotClearTheFilter`
- `"it returns to multi-select with the set intact"` — `TestPanelClose_NestsOverMultiSelect`
- `"it never quits"` — `TestPanelClose_EscDoesNotQuit` (both pages)
- `"it is the single close path"` — `TestPanelClose_ForcedCloseUsesTheSameFunction` (task 8-11's forced close asserted to route here)

**Edge Cases**:
- `Esc` **re-resolves persisted state** — it does not restore a theme snapshotted at open, which is the naive implementation and is wrong in two directions.
- Resolution runs against the **panel's** enumeration rather than what construction loaded, so if the user broke the active theme's file mid-session `Esc` lands on the §8.5 fallback: Portal shows what the config now says, not a stale copy it happens to still hold.
- The mirror case is the whole point of re-reading — fixing a previously-invalid theme takes effect on the next open with no relaunch.
- Close discards the uncommitted preview **and** the retained enumeration, so the next open re-reads.
- The close path is the one §11.1 says matters most, because a missed re-point there leaves a preview the user explicitly discarded painting the main screen.
- `Esc` equals "what you had before" only when nothing was committed — commits are Phase 9 and land on this same resolution, so the mechanism must be re-resolution from the start.
- `Esc` is the **only** way out (`Enter` deliberately does not close, Phase 9), so close must not become a side effect of anything else.
- Closing is one frame, instantaneous, and re-themes through `ApplyTheme` like every other caller.
- §9.8's forced close takes this path **exactly**, so the two share one implementation rather than two that can drift.
- Phase 9's `⚠ theme not saved — see portal.log` flash and the outstanding-failure discharge hook onto this single close path and must have somewhere to attach.
- **Nothing is written on close** — every write is an explicit keypress, which is what eliminates the "applied but not persisted" state persist-on-close would reach.

**Context**:
> §9.2: "`Esc` — **Closes.** Discards an uncommitted preview and renders the resolved persisted state… Which sharpens `Esc` precisely: **`Esc` discards the preview and renders the resolved persisted state.** That equals 'what you had before' only when nothing was committed. Commit slots and `Esc` lands on the newly-resolved theme, which is correct."
> §5.8: "**`Esc` resolves persisted state against the panel's enumeration**, not against what construction loaded. If the user edited their active theme's file and broke it, `Esc` lands on the §8.5 fallback — Portal shows what the config now says, not a stale copy it happens to still hold."
> §11.1: "**The close path matters most** — a missed re-point there leaves a preview the user explicitly discarded painting the main screen, and §13.4's guard drives the arrow-preview entry point only."
> §9.2: "**Every write is an explicit keypress; nothing writes on close.** This eliminates the 'applied but not persisted' state reachable under persist-on-close."
> §9.8: "**A forced close takes the `Esc` path exactly** — it discards an uncommitted preview and renders the resolved persisted state (§9.2)."
> **Ambiguity flagged and resolved conservatively**: §12.3 gives `theme: fallback applied` an explicit per-process dedup *because* resolution re-runs "on every panel open and again on every `Esc`", but its `theme: loaded` cadence column names only construction and the commit-time load, and `loaded` is deliberately undeduplicated. The spec never says whether the panel's re-resolution emits `loaded`. This task pins **no `loaded` on the panel path** — emitting it per open/close would contradict the undeduplicated per-load contract the catalogue states — and records the choice in-source so Phase 9's commit-time load (which *is* catalogued) is added deliberately rather than inherited.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.2, §5.8, §8.4, §8.5, §9.8, §11.1, §12.3

## theming-system-8-11

### Task 8.11: Panel geometry — degrade between preferred and minimum, refuse below the floor

**Problem**: Task 8-6 renders the panel at whatever width and height it is handed and deliberately owns neither choice. Nothing yet decides what happens as the terminal narrows, and the two available doctrines point in opposite directions: MV §2.7's degrade-never-break governs a *space shortage*, while the multi-select precedent (a proactive block at entry) governs a *capability absence* — applying the wrong one either opens a broken frame on a 30-column terminal or refuses a panel that would have fitted. The height floor is not obvious either: it is **header (two rows) + footer + one list row + one message row**, and the message row is in the floor even though §9.1 does not reserve it when empty — because both of its Phase 9 contenders are non-suppressible, so a floor computed without it puts the panel exactly one row short at the moment a message appears, asking "clear constant `<slug>`?" about a row that has just been pushed off screen. And the same predicate has to serve **two** callers — §9.7's entry condition and §9.8's resize condition — or a terminal can pass one check and fail the other.

**Solution**: One width ladder and one floor predicate in `internal/tui/theme_panel.go`, consumed by the resize handler here and by task 8-13's entry gate, with a below-floor resize routed through task 8-10's `closeThemePanel` plus §14A's pinned per-dimension flash.

**Outcome**: The panel takes its preferred width on a normal terminal, shrinks toward its minimum as the terminal narrows, refuses to open (with a flash) below the floor, degrades in place on resize, and force-closes onto the resolved persisted state — with the pinned copy — when a resize crosses the floor with the panel open.

**Do**:
- **Pin the width ladder** as `themePanelWidthFor(contentW int) (w int, ok bool)` beside task 8-6's `themePanelPreferredWidth = 30` / `themePanelMinWidth = 24`: `w := contentW / 2`, clamped to `[themePanelMinWidth, themePanelPreferredWidth]`; `ok` is false when `contentW < themePanelMinWidth`. Document the derivation rather than the number: the half-width cap is what keeps the previewed page visible while the terminal is wide enough to afford it, and the clamp is what produces §2.7's *staged* shrink (≥60 cols → 30; 48–59 → a shrinking 24–29; 24–47 → the 24-column minimum; below → refuse). Note that §9.8 leaves exact thresholds to implementation, as §2.7 already does for its own steps.
- **Pin the height floor** as `themePanelMinHeight(entries []keymapEntry, dirUnusable bool) int` = 2 (task 8-6's header: label + rule) + `themePanelFooterHeight(entries)` (task 8-5's measured height, never a literal) + 1 list row + 1 message row + 1 when `dirUnusable`. Carry the two justifications in-source: the message row is counted although §9.1 does not reserve it when empty, because both Phase 9 contenders are non-suppressible; and the directory row is counted **conditionally** because otherwise the warning consumes the single list row while §9.5 simultaneously requires built-in and persisted rows to render beneath it.
- **One predicate, two callers**: `themePanelFloor(contentW, contentH int, dirUnusable bool) (dim themePanelDim, ok bool)` returning which dimension failed (`dimWidth` / `dimHeight`, width checked first) so both the entry flash (task 8-13) and the resize flash below select their copy from the same result. Compute it once; do not let task 8-13 re-derive it.
- **Resize while open — degrade in place.** In the `tea.WindowSizeMsg` path, when the panel is open: re-run the ladder and the body-height arithmetic and `SetSize` the panel's list, exactly as the two page lists are re-sized. The main screen is deliberately **not** re-laid-out to the reduced width (§9.1) — the panel is composited over a page that laid out at full width, so a panel width change never reflows the surface being previewed.
- **Resize below the floor — forced close.** Call task 8-10's `closeThemePanel()` (the *same* function, not a second teardown) and then set the pinned flash through the transient-flash slot: `terminal too narrow — theme picker closed` / `terminal too short — theme picker closed`, single-sourced as constants beside task 8-13's entry strings. Record why any other behaviour is refused: it would strand the user rendering a theme they never chose, with the surface that could change it gone and a terminal too narrow to reopen it — the state §11.4 names as the one where an unchosen colour can survive Portal's exit.
- **Message-slot degradation rule.** `renderThemePanelMessage` truncates to **one line** when the panel is at its minimum height and wraps otherwise — the two dimensions degrade differently on purpose, and truncation degrades the message rather than the row it is about. Phase 8's slot is always empty, so drive this by setting the field directly in a test, the same way task 8-6 drove the height recompute.
- **Note the Phase 9 hook, do not build it**: a live slot-from-constant confirm is **silently cancelled** by a forced close — nothing has been written at that point, so there is no partial state to leave behind. It is stated because the confirm is otherwise specified as resolvable only by a keypress; the confirm itself is Phase 9's.

**Acceptance Criteria**:
- [ ] `themePanelWidthFor` returns the preferred width on a wide terminal, a value strictly between minimum and preferred across a band of narrowing widths (monotone non-increasing), the minimum at the bottom of the band, and `ok=false` below it.
- [ ] The height floor equals header(2) + the **measured** footer height + 1 + 1, and gains exactly one more row when `DirUnusable` is true.
- [ ] `themePanelFloor` reports `dimWidth` when both dimensions fail (width is checked first), and its result is the single input to both the entry gate and the resize path (asserted by driving both through one seam).
- [ ] A resize that stays above the floor keeps the panel open and re-sizes its list; the panel's rendered height still equals the content height and every row still equals the panel width.
- [ ] A resize that stays above the floor does **not** re-lay-out the main screen: the page view beneath is composed at the unreduced width and a footer entry is still cut mid-label rather than reflowed.
- [ ] A resize below the **width** floor closes the panel and raises exactly `terminal too narrow — theme picker closed`; below the **height** floor, `terminal too short — theme picker closed`.
- [ ] The forced close routes through `closeThemePanel` — the resolved persisted state is rendered and the retained enumeration is discarded, identically to `Esc` (asserted against the `Esc` path's own output).
- [ ] The forced close writes nothing.
- [ ] With a one-row message set directly, the panel at its minimum height renders the message on exactly one line (truncated, not wrapped) and still renders one list row; at the minimum width above the floor the same message may occupy two rows.
- [ ] A panel open at exactly the floor renders header, directory row (when unusable), one list row, the message row's budget and the footer, with no overflow past the content height.

**Tests**:
- `"it stages the width between preferred and minimum"` — `TestPanelGeometry_WidthLadder` (table across widths, monotonicity asserted)
- `"it refuses below the minimum width"` — `TestPanelGeometry_WidthFloor`
- `"it computes the height floor from the measured footer"` — `TestPanelGeometry_HeightFloorArithmetic`
- `"it adds a row to the floor for an unreadable directory"` — `TestPanelGeometry_DirRowRaisesTheFloor`
- `"it reports width before height"` — `TestPanelGeometry_FloorReportsWidthFirst`
- `"it degrades in place on a resize"` — `TestPanelGeometry_ResizeDegradesInPlace`
- `"it never re-lays-out the page beneath"` — `TestPanelGeometry_ResizeDoesNotReflowTheBase`
- `"it force-closes with the pinned narrow copy"` — `TestPanelGeometry_ResizeBelowWidthFloorClosesWithFlash`
- `"it force-closes with the pinned short copy"` — `TestPanelGeometry_ResizeBelowHeightFloorClosesWithFlash`
- `"it force-closes through the Esc path exactly"` — `TestPanelGeometry_ForcedCloseIsTheEscPath`
- `"it truncates the message at the minimum height"` — `TestPanelGeometry_MessageTruncatesAtFloorHeight`
- `"it renders coherently at exactly the floor"` — `TestPanelGeometry_RendersAtTheFloor`

**Edge Cases**:
- Narrow **degrades, it does not refuse**: the panel shrinks between a preferred ~24–30 columns and a minimum as the terminal narrows, staged consistently with §2.7's existing width ladder, and refuses only when even the minimum panel cannot render — and then it flashes rather than opening a broken frame.
- The multi-select precedent (proactive block at entry) deliberately does **not** transfer, because that is a capability *absence* and this is a space *shortage*.
- The height floor is **header (two rows) + footer + one list row + one message row**, plus the `⚠ dir unreadable` chrome row when the directory is unusable — otherwise the warning would consume the single list row while §9.5 simultaneously requires rows beneath it.
- The message row is in the floor even though §9.1 does not reserve it when empty, because both contenders are non-suppressible and a floor computed without it puts the panel one row short at exactly the moment a message appears, asking about a row no longer on screen.
- At the minimum **height** the message truncates to one line while at the minimum **width** it may wrap — the two degrade differently on purpose, and truncation degrades the message rather than the row it is about.
- Overflow scrolls through `bubbles/list` so `Ctrl+↑`/`Ctrl+↓` paging applies and the invalid-row skip composes with it.
- Resize while open degrades **in place**, and below either dimension's floor takes the `Esc` path **exactly** plus the pinned flash.
- Any other forced-close behaviour would strand the user rendering a theme they never chose with the surface that could change it gone and the terminal too narrow to reopen it — the state §11.4 names as the one where an unchosen colour can survive Portal's exit.
- A live slot-from-constant confirm is **silently cancelled** by a forced close (Phase 9's confirm; nothing is written at that point so there is no partial state), stated because the confirm is otherwise resolvable only by a keypress.
- Exact thresholds are pinned at implementation as §2.7 already does for its own steps.
- The same floor predicate serves §9.7's **entry** condition, so it is computed once rather than twice.
- The main screen is not re-laid-out while the panel is open, so a panel width change never reflows what is being previewed.

**Context**:
> §9.8: "**Narrow terminals degrade, they do not refuse.**… The panel **shrinks** between a preferred and a minimum width as the terminal narrows — staged degradation, consistent with §2.7's existing width steps… It **refuses only when even the minimum panel cannot render**, which is very narrow indeed — and then it flashes rather than opening a broken frame. **Exact thresholds are pinned at implementation.**"
> §9.8: "**Minimum height: the same degrade-then-refuse rule as width** — shrink the visible row count, and refuse with a flash only when **header + footer + one row + one message row** cannot fit. The message row is part of the floor even though §9.1 does not reserve it when empty: both of its contenders are non-suppressible."
> §9.5: "**It costs a viewport row while present, so §9.8's minimum-height floor gains it conditionally** — header + footer + one row + one message row, **plus this row when the directory is unusable**."
> §9.1: "At the minimum panel width the slot may wrap to two rows… **It does cost a row of vertical budget, so at the minimum *height* the message is truncated to one line rather than wrapped.**"
> §14A (flashes): "Resize below the **width** floor with the panel open → `terminal too narrow — theme picker closed`; Resize below the **height** floor with the panel open → `terminal too short — theme picker closed`."
> **Ambiguity flagged**: §9.8 pins refusal at "even the minimum panel cannot render" and does **not** require any of the previewed page to remain visible, so the floor here is `contentWidth ≥ themePanelMinWidth` and nothing more. At exactly the floor the panel therefore covers nearly the whole content region; that is accepted rather than fixed with an invented "keep N columns of page" rule, which the spec does not license.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.8, §9.1, §9.5, §9.7, §11.4, §14A

## theming-system-8-12

### Task 8.12: Projects gains the transient-flash slot

**Problem**: Every one of §14A's six theme flashes is reachable from the Projects page — `t` is bound there (§9.6), `t theme` is in its footer (§14.2), and the panel opens over it — but Portal's notice-band arbiter is **Sessions-only**: `activeNoticeBand`, `renderSessionBandSlot` and the flash lifecycle's layout re-sync (`resyncSessionLayout`) all address the Sessions list alone, and `updateProjectsPage` has no actionable-key flash clear at all. So today a blocked `t` on Projects would set `flashText` and render nothing. The two obvious alternatives each destroy a decided guarantee: suppressing the flashes on Projects makes §9.10's proactive block a silent no-op — the walkable dead end it exists to prevent, reached by another route — and makes Phase 9's failed-commit report vanish outright, since closing the panel *discharges* the outstanding state whether or not a flash rendered; while refusing `t` on Projects contradicts §9.6, which binds it there precisely because theme is a global setting.

**Solution**: Give the Projects band slot the **transient-flash contender alone** — arbitrated against the existing command-pending banner, rendered through the shared `renderNoticeBand` primitive, and driven by the existing flash lifecycle (generation counter, tick, actionable-key clear) rather than a second mechanism.

**Outcome**: A flash raised while the Projects page is active renders as the same `▌` band the Sessions page shows, in the slot beneath the title separator, with the list height recomputed underneath it; it outranks the command-pending banner for its duration; and it clears on the same tick and the same next-actionable-key rule.

**Do**:
- **Arbitrate the Projects slot.** Add `activeProjectNoticeBand() (role noticeBandRole, message string, ok bool)` beside `activeNoticeBand`, returning the flash (`flashBandRole(m.flashKind)`, `m.flashText`) when one is live, else the command-pending banner. Re-point `renderProjectBandSlot` at it so the slot renders the arbitrated band plus its existing blank breathing row. **Arbitrate, never co-render** — the band is one slot, and the Sessions arm's deliberate multi-select co-render exception (documented in `activeNoticeBand`) has no Projects analogue.
- **Keep `projectBandHeight` measuring off the slot**, unchanged — it already derives the reserve from the rendered block, so the flash's row is budgeted by construction and the one-row-per-delegate pagination invariant holds with no second arithmetic.
- **Re-sync the active page.** `setFlash` / `setSuccessFlash` / `clearFlash` currently call `resyncSessionLayout`, which re-sizes only the session list. Widen it (or add its Projects sibling) so the active page's list is re-sized when a flash appears or clears; keep the existing no-op-before-first-`WindowSizeMsg` behaviour so the flash state primitives stay observable on a bare `Model{}`.
- **Add the actionable-key clear to `updateProjectsPage`**, mirroring `updateSessionList`'s: at the top of the `tea.KeyPressMsg` arm, `if m.flashText != "" && isActionableKey(msg) { m.clearFlash() }` with the same deliberate fall-through ("one key, one intent") and the same comment naming it. Place it in the same relative position the Sessions arm uses so the two read identically.
- **Reuse the timer, not a second one.** `flashTickMsg` is already handled in the top-level `Update` with its generation guard, so a Projects flash inherits the auto-clear for free; the caller returns `flashTickCmd(m.flashGen)` exactly as the Sessions callers do. Add no Projects-specific duration, kind or tick.
- **Only the flash contender.** Do not port the no-tags signpost, the multi-select banner, the unsupported banner or the burst-progress band to Projects — no other contender has a Projects analogue and inventing them would be scope for nothing. Say so in the arbiter's doc comment.
- **Do not touch the filter-line precedence.** §14A's change (theme flashes outranking the filter line) is **Phase 9's** and must not be pulled in here; the Projects filter header keeps today's behaviour in this phase.
- **Leave room for Phase 9.** Phase 9's `⚠ theme not saved — see portal.log` lands in this same slot, so the slot must not be scoped to entry-blocked flashes — it takes any `flashText`, whatever set it.

**Acceptance Criteria**:
- [ ] A flash set while the Projects page is active renders a `▌` band beneath the title separator with the same role/tint/glyph treatment as the Sessions band (byte-identical for the same message and width).
- [ ] The Projects list height shrinks by the slot's measured height when the flash appears and is restored when it clears — asserted through `projectBandHeight`, with no separate arithmetic.
- [ ] A flash outranks the command-pending banner while shown; when it clears, the banner returns.
- [ ] The band never co-renders with the command-pending banner (exactly one band row set is present).
- [ ] An actionable keypress on Projects clears the flash and still reaches its normal handler (`x` still switches page, `e` still opens the edit modal).
- [ ] A non-key event (window size, focus, blur) does not clear a Projects flash.
- [ ] The auto-clear tick clears a Projects flash through the existing generation guard, and a superseded tick does not clear a newer flash.
- [ ] No Sessions-only contender renders on Projects (no signpost, no multi-select banner, no unsupported banner).
- [ ] Under `colourless` the Projects band drops hue and tint and keeps `▌` and `⚠`/`✓`.
- [ ] The Sessions band's behaviour is byte-unchanged by this task, including the documented multi-select co-render exception.
- [ ] The Projects filter header's precedence is unchanged (the Phase 9 change is absent).

**Tests**:
- `"it renders a flash on the Projects page"` — `TestProjectsFlash_RendersInTheBandSlot`
- `"it recomputes the Projects list height"` — `TestProjectsFlash_RecomputesListHeight`
- `"it outranks the command-pending banner"` — `TestProjectsFlash_WinsTheSlotOverCommandPending`
- `"it never co-renders two bands"` — `TestProjectsFlash_SingleSlot`
- `"it clears on the next actionable key and falls through"` — `TestProjectsFlash_ActionableKeyClearsAndFallsThrough`
- `"it survives a non-key event"` — `TestProjectsFlash_SurvivesWindowSize`
- `"it clears on the shared tick with the generation guard"` — `TestProjectsFlash_TickClearsWithGenerationGuard`
- `"it adds no Sessions-only contender"` — `TestProjectsFlash_OnlyTheFlashContender`
- `"it drops hue under colourless"` — `TestProjectsFlash_Colourless`
- `"it leaves the Sessions band untouched"` — `TestProjectsFlash_SessionsBandUnchanged`

**Edge Cases**:
- The existing notice band is a **Sessions-only** arbiter — every one of its six contenders is a Sessions element — yet §9.6 binds `t` on Projects, §14.2 puts `t theme` in its footer, and all six of §14A's theme flashes are reachable there.
- Projects gets the **flash contender alone**, not the full arbiter: no other contender has a Projects analogue and inventing them would be scope for nothing.
- Both alternatives destroy a decided guarantee — suppressing the flashes makes §9.10's proactive block a silent no-op, and makes Phase 9's failed-commit report vanish outright since closing discharges the state whether or not a flash rendered; while refusing `t` on Projects contradicts §9.6.
- The band appears and the list height recomputes exactly as the Sessions band already does, so the one-row-per-delegate pagination invariant holds.
- It must **arbitrate** against the existing Projects command-pending banner rather than co-render with it.
- The flash reuses the existing transient timer and clear-on-next-actionable-keypress machinery, not a second mechanism.
- The **filter-line precedence** change (theme flashes outranking the filter line) is Phase 9's and must not be pulled in here.
- Phase 9's `⚠ theme not saved — see portal.log` lands in this same slot, so the slot must not be scoped to entry-blocked flashes.
- §13.3 requires a Projects-with-panel fixture (task 8-16) so the page is seen with the panel over it before release.

**Context**:
> §14A: "**Projects gains a transient-flash slot.** The existing arbiter is Sessions-only — every one of its six contenders is a Sessions element — yet §9.6 binds `t` on Projects and §14.2 puts `t theme` in its footer, and all six of these flashes are reachable there. **Projects gets the flash contender alone**, not the full arbiter: no other contender has a Projects analogue, and inventing them would be scope for nothing."
> §14A: "The two alternatives each destroy a decided guarantee. Suppressing the flashes on Projects makes §9.10's proactive block a silent no-op — the walkable dead end it exists to prevent — and makes §9.13's report vanish outright, since closing the panel discharges the outstanding state whether or not a flash rendered. Refusing `t` on Projects contradicts §9.6."
> §14A: "**This is a change to the band's precedence, scoped to these flashes**" — the filter-line half, which is Phase 9's; this task adds the slot only.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §14A, §9.6, §9.10, §9.13, §13.3

## theming-system-8-13

### Task 8.13: Entry conditions, blocked-`t` flashes and key-exclusive routing

**Problem**: `t` currently opens the panel unconditionally wherever it is bound (task 8-7), which walks the user into three dead ends the spec explicitly forbids. Under `NO_COLOR` Portal paints no canvas and imposes no hues, so the panel previews nothing, its cursor tint and slot dots have no colour, and committing would persist a choice with zero visible feedback — a **capability absence**, blocked proactively exactly as `m` already is. Below the render floor the panel would open as a broken frame — a **space shortage**, and §9.10 draws the two calls deliberately opposite, so conflating them gets one of them wrong. And during a pending burst the model is input-locked (only `Ctrl-C`/`Esc` live) because it is mid-async-operation, so `t` must be swallowed consistently with that lock rather than exempted from it. Routing is the mirror problem: while the panel is open, pass-through is genuinely bad — `k` would kill the highlighted session while you pick a theme, `x` would swap to Projects with the panel open, `m` would start a multi-select behind it — but swallowing `Ctrl-C` would take away the user's exit key inside a settings surface.

**Solution**: A single entry gate returning either "open" or a blocked outcome carrying its pinned copy (or silence), consumed by both pages' `t` dispatch; plus the panel's key-exclusive routing with `Ctrl-C` live and multi-select nesting, driven off task 8-11's one floor predicate.

**Outcome**: `t` opens the panel on Sessions and Projects; it flashes §14A's exact copy under `NO_COLOR` and below either render-floor dimension; it is silently swallowed during a pending burst and unbound on Preview, Loading and modals; and while the panel is open every key except its own and `Ctrl-C` does nothing, with multi-select surviving underneath.

**Do**:
- **Add the gate** `func (m Model) themePanelEntry() (blockedFlash string, ok bool)` in `internal/tui/theme_panel.go`, evaluated in this order: `m.colourless` (the `NO_COLOR` carve-out) → task 8-11's `themePanelFloor(contentW, contentH, false)` on the current content dimensions → otherwise open. It returns the pinned copy for the blocked cases and is the **only** place the pre-read decision is made; `t`'s dispatch on both pages consults it and does nothing else.
- **Re-evaluate the floor once the directory's state is known.** The `⚠ dir unreadable` row raises the height floor by one (task 8-11) but `Union.DirUnusable` is a product of the enumeration, which task 8-7 pins to the keypress *after* this gate — so the pre-read evaluation passes `dirUnusable = false` and task 8-7's open sequence re-applies **the same predicate** with the real flag as soon as `Open` returns. If the real flag now fails the floor, discard the enumeration, do **not** open, and raise the same pinned height flash (`terminal too short for the theme picker`). One predicate, two evaluations — task 8-11's "compute it once" is about not re-deriving the arithmetic, not about evaluating it once. Record why neither shortcut is taken: assuming `true` at entry refuses terminals that would have rendered a good panel, contradicting §9.8's degrade-don't-refuse; assuming `false` and never re-checking opens a panel whose list body is **zero rows**, which is the "completely in the dark" state §9.5's pinned row exists to prevent. Accept, and state in-source, that a blocked open in this rare case has already performed its directory read and emitted `theme: enumerated` — the enumeration genuinely happened, and splitting the read from its emission would fork task 8-1's seam for one edge.
- **Single-source the copy** as constants beside task 8-11's resize strings, following Portal's `spawn.UnsupportedNoopMessage` convention — verbatim: `theme picker needs colour — NO_COLOR is set`, `terminal too narrow for the theme picker`, `terminal too short for the theme picker`. Assert them against the §14A strings, not a paraphrase.
- **Dispatch on both pages**: in `updateSessionList` and `updateProjectsPage`, `t` (after the `SettingFilter()` guard task 8-7 established) calls the gate; on `ok` it opens, otherwise it raises the returned flash through the page's band — the Sessions band or task 8-12's Projects slot — and returns `flashTickCmd(m.flashGen)` so the block inherits the standard auto-clear, exactly as the `m` proactive block does.
- **Silence where the key is not bound at all**: **Preview** and **Loading** bind no `t` and render nothing — the feedback rule is *flash where the key is bound and the user could reasonably expect it to work, silent where it is not bound at all*, which is precisely how `s` already behaves. Record both refusals: Preview's body is deliberately out-of-theme captured scrollback (a weak preview surface) *and* it is already a full-screen overlay, so the panel would stack an overlay on an overlay; Loading is inert by design and renders no notice band to flash into, and on the cold + TUI path it holds for at least `LoadingMinDuration` with the user watching, so it is not a corner case.
- **Modals and the burst**: modals are key-exclusive already, so `t` never reaches the gate. A **pending burst swallows** `t` — the existing `burstPending` arm at the top of `updateSessionList` returns before any rune dispatch, so this is *consistency with the lock*, not a new exception; assert it rather than adding a branch.
- **Key-exclusive routing**: task 8-7 already swallows non-panel keys; here pin it as tested behaviour with the reasoning attached — `k` does not kill, `x` does not switch page, `m` does not enter multi-select, `/` does not filter, `?` does nothing (there is no panel help modal), `s`/`e`/`n`/`r`/`q` are inert — while **`Ctrl-C` stays live** and quits. `d` and `l` are **panel-owned** keys, not swallowed ones: they are inert this phase and become commit keys in task 9-3, so assert them as "the page's own binding never fires" (on Projects, `d` must not open the delete modal) rather than as "the model is unchanged", which task 9-3 would falsify. State in-source that non-blanking and key-exclusive are not in tension: seeing the list without being able to drive it *is* the live-preview premise.
- **Nest over multi-select**: `t` opens with the marked set **unaffected**, the `N selected` banner still visible in the notice band behind the panel, and `Esc` resolving innermost-first (task 8-10's close returns to multi-select with selections intact) — the rule modals already follow. Previewing mid-selection is legitimate, since the marked-row `●` is itself themed.
- **Own the dispatch only.** The **display** half of the block — dropping the `t` row from the footer and from `?` help in lockstep — is task 8-14's; do not filter any descriptor here.

**Acceptance Criteria**:
- [ ] `t` opens the panel on Sessions and on Projects when nothing blocks it.
- [ ] Under `NO_COLOR` (`colourless`), `t` on either page opens nothing and raises exactly `theme picker needs colour — NO_COLOR is set`.
- [ ] Below the width floor `t` raises exactly `terminal too narrow for the theme picker`; below the height floor, `terminal too short for the theme picker`; when both fail, the width copy (width is checked first, per task 8-11).
- [ ] The entry gate and the resize path read the **same** floor predicate — a size that blocks entry also force-closes an open panel, asserted across one table.
- [ ] With a **usable** directory, a terminal one row above the non-directory floor opens the panel — the gate does not reserve the `⚠ dir unreadable` row speculatively.
- [ ] With an **unusable** directory at that same height, the panel does **not** open: the enumeration is discarded, `terminal too short for the theme picker` is raised, and no panel state survives.
- [ ] A panel that opens with an unusable directory always renders **at least one list row** beneath the pinned `⚠ dir unreadable` row — asserted at the directory-inclusive floor exactly.
- [ ] `t` on the Preview page and on the Loading page does nothing and raises **no** flash.
- [ ] `t` with a modal open does nothing (the modal keeps the key) and `t` during a pending burst is swallowed with no flash.
- [ ] Blocked `t` inherits the auto-clear: the flash clears on the tick and on the next actionable key on both pages.
- [ ] While the panel is open: `k`, `x`, `m`, `/`, `?`, `s`, `e`, `n`, `r`, `q` each leave the model state unchanged (no kill, no page switch, no mode entry, no filter, no modal).
- [ ] `d` and `l` never reach the page beneath while the panel is open — on Projects, `d` opens **no** delete modal — asserted as an absence of the page's effect, so the criterion still holds once task 9-3 makes them commit keys.
- [ ] `Ctrl-C` while the panel is open quits.
- [ ] `t` entered from multi-select leaves the marked set and its banner intact, and closing returns to multi-select with the same set.
- [ ] No descriptor is filtered by this task (both keymaps byte-unchanged).

**Tests**:
- `"it opens from both pages when unblocked"` — `TestPanelEntry_OpensOnSessionsAndProjects`
- `"it blocks under NO_COLOR with the pinned copy"` — `TestPanelEntry_NoColorBlocked` (both pages)
- `"it blocks below each render-floor dimension"` — `TestPanelEntry_FloorBlocked` (narrow, short, both)
- `"it does not reserve the directory row before it knows"` — `TestPanelEntry_UsableDirectoryOpensAtTheNonDirFloor`
- `"it blocks after the read when the directory row raises the floor"` — `TestPanelEntry_UnusableDirectoryBlocksOnTheReEvaluation` (enumeration discarded, pinned short flash, panel closed)
- `"it shares one floor predicate with the resize path"` — `TestPanelEntry_SameFloorAsResize`
- `"it is silent where t is not bound"` — `TestPanelEntry_SilentOnPreviewAndLoading`
- `"it is swallowed during a pending burst"` — `TestPanelEntry_SwallowedWhileBurstPending`
- `"it never reaches a modal"` — `TestPanelEntry_ModalKeepsTheKey`
- `"its flash auto-clears like every other"` — `TestPanelEntry_BlockedFlashLifecycle`
- `"it swallows every page key while open"` — `TestPanelRouting_KeyExclusive` (table over k/x/m//,?,s,e,n,r,q)
- `"it keeps the panel's own keys off the page beneath"` — `TestPanelRouting_PanelOwnedKeysNeverReachThePage` (`d` on Projects opens no delete modal; `l` reaches no page binding)
- `"it keeps Ctrl-C live"` — `TestPanelRouting_CtrlCQuits`
- `"it nests over multi-select without disturbing the set"` — `TestPanelRouting_NestsOverMultiSelect`
- `"it filters no descriptor"` — `TestPanelEntry_LeavesDescriptorsUnfiltered`

**Edge Cases**:
- Nothing blocks `t` except a modal, a pending burst, `NO_COLOR`, a terminal below either render-floor dimension, and the pages where it is not bound at all.
- `NO_COLOR` is a **capability absence** — no canvas, no hues, a preview of nothing — so it is proactively blocked, while a narrow or short terminal is a **space shortage**: deliberately the opposite calls, drawn by §9.10 on exactly that distinction.
- Below-the-floor is an **entry** condition as well as §9.8's resize condition, and both read the one predicate.
- The floor is **conditional on `DirUnusable`**, which does not exist until the enumeration runs on the keypress — so the predicate is evaluated twice against the same function: once before the read with `dirUnusable = false`, once immediately after `Open` returns with the real flag. Assuming `true` up front would refuse terminals that fit; assuming `false` and never re-checking would open a panel with **zero** list rows beneath the pinned warning row, the state §9.5 requires rows beneath it precisely to prevent. A blocked open on the re-evaluation has already read the directory and emitted `theme: enumerated`, which is accepted rather than worked around.
- Feedback follows the existing precedent — **flash** where the key is bound and the user could reasonably expect it to work, **silent** where it is not bound at all, exactly how `s` already behaves.
- **Loading** is silent and is not a corner case, since on the cold + TUI path it holds for at least `LoadingMinDuration` with the user watching and renders no notice band to flash into.
- **Preview** is refused on two grounds — its body is deliberately out-of-theme captured scrollback so the preview would be a weak surface, and it is already a full-screen overlay so the panel would stack an overlay on an overlay.
- A **pending burst swallows** `t`, consistent with the input-lock rather than an exception to it.
- The panel is **key-exclusive** — it owns arrows, `Enter`, `d`, `l` and `Esc` and swallows everything else, because `k` would kill the highlighted session while you pick a theme, `x` would swap pages behind it and `m` would start a multi-select — but **`Ctrl-C` stays live**, since swallowing it would take away the exit key inside a settings surface. `d`/`l` are owned rather than swallowed, so they are asserted as never reaching the page beneath (no Projects delete modal) rather than as leaving the model unchanged — task 9-3 makes them write.
- Non-blanking and key-exclusive are not in tension: seeing the list without being able to drive it *is* the live-preview premise.
- The panel **nests** over multi-select with the marked set unaffected, the banner still visible in the notice band behind it, and `Esc` resolving innermost-first — the rule modals already follow — and previewing mid-selection is legitimate since the marked-row `●` is itself themed.
- The **display** half of the block (dropping the `t` row from the footer and `?` help in lockstep) is task 8-14's, so this task owns the dispatch and the flash only.

**Context**:
> §9.7: "**Nothing blocks `t` except a modal, a pending burst, `NO_COLOR`, a terminal below the render floor, and the pages where it is not bound at all**… **Blocked-`t` feedback follows the existing precedent:** **flash** where the key *is* bound and the user could reasonably expect it to work; **silent** where it is not bound at all. That is exactly how `s` already behaves."
> §9.7: "**The panel is key-exclusive.** It owns arrows, `Enter`, `d`, `l` and `Esc`; everything else is swallowed **except `Ctrl-C`, which stays live**… **Multi-select** — `t` opens, and the marked set is **unaffected**. The panel *nests* over the mode and `Esc` resolves innermost-first."
> §9.10: "**`t` is blocked under `NO_COLOR`, with a flash**, following the multi-select precedent exactly… This is deliberately the **opposite** call to the narrow-terminal one. Narrow is a *space shortage*, where §2.7 mandates degrade. `NO_COLOR` is a *capability absence*."
> §14A (flashes): "`t` under `NO_COLOR` → `theme picker needs colour — NO_COLOR is set`; `t` below the **width** floor → `terminal too narrow for the theme picker`; `t` below the **height** floor → `terminal too short for the theme picker`."
> §9.6: "**Preview** | **No.** The preview body is captured real ANSI scrollback that is deliberately out-of-theme… It is also already a full-screen overlay. **Loading** | **No**, and **silently**."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.6, §9.7, §9.8, §9.10, §14A

## theming-system-8-14

### Task 8.14: §14's footer revision on both pages with lockstep help filtering

**Problem**: The feature is otherwise near-invisible — `--theme` and `portal theme list` are ruled out, the themes directory is silent and never seeded, built-in rows are indistinguishable from drop-ins, the reserved-slug set is not discoverable from the UI, and there is no active-theme indicator when the panel is closed. Discoverability would rest entirely on `?` help and `docs/theming.md`. §14 answers that by putting `t theme` in both footers — which forces three further changes: `↑↓ navigate` comes out (arrows in a list are a given, and it is the entry that genuinely deserves non-core status), `m` is promoted alongside `t` and shortened to `m multi` (the measured width fits with ~5px spare and no headroom at the reference 86 columns), and the footer must be filtered **in lockstep with `?` help** — advertising a key that only produces a blocked flash is the dead end the proactive block exists to prevent. There is also a live defect this uncovers: §14.4 requires `? help` never be dropped, but today's `assembleRightAnchoredRow` drops the **right anchor first** and pads the left cluster, so at narrow widths Portal loses precisely the escape hatch that makes every dropped entry recoverable.

**Solution**: Re-author both page descriptors for the new Core sets and orders, route both footers through a model-side **filtered** descriptor shared with `?` help, and invert the right-anchored assembler so the anchor survives and the left cluster degrades right-to-left beneath it.

**Outcome**: Sessions reads `⏎ attach · / filter · ␣ preview · s switch view · x projects · t theme · m multi` with a right-aligned `? help`; Projects reads `⏎ new session · x sessions · e edit · / filter · t theme` with `? help`; a blocked `t` or `m` is absent from **both** the footer and the help body; and narrowing drops entries from the right while `? help` stays until it alone no longer fits.

**Do**:
- **Sessions descriptor** (`sessionsKeymap`): clear `Core` on the `↑↓` nav entry (it stays listed, non-core, as `^↑/↓` already is); add `{Key: "t", Action: "theme", HelpAction: "Theme picker", Core: true}`; set `Core: true` on the `m` entry and change its footer `Action` to **`multi`** (its `HelpAction` stays `Multi-select mode`). **Re-order** the descriptor so the Core relative order is `⏎ · / · ␣ · s · x · t · m · ?` — i.e. move `m` to sit after `x` and insert `t` between them — because the footer renders Core entries in descriptor order and §14.2's row is pinned. Update the descriptor's doc comment: the help body's order moves with it (the nav-first principle is preserved; only the tail order changes), and §15.1 names this as the amendment rather than a regression.
- **Projects descriptor** (`projectsKeymap`): add `{Key: "t", Action: "theme", HelpAction: "Theme picker", Core: true}` after the `/` entry so the Core order is `⏎ · x · e · / · t · ?`. Nav is already non-core there, so nothing else moves.
- **Filter both surfaces from one place.** Extend `sessionsHelpKeymap()` to also drop the `t` entry when the panel is blocked under `NO_COLOR`, and add the matching **`projectsHelpKeymap()`** — §9.10 names only the Sessions call site, but `t theme` is now in the Projects footer too, so the second call site is required. Then re-point **the footers** at the same filtered slices: `renderSessionsFooter` / `renderProjectsFooter` take `entries []keymapEntry` (as `renderCondensedFooter` already does internally) and every call site — the composed view **and** the height-budget computation — passes `m.sessionsHelpKeymap()` / `m.projectsHelpKeymap()`, so the budget and the render resolve against the identical entry set. The **static** `sessionsKeymap()` / `projectsKeymap()` stay unfiltered so `keymap_dispatch_guard_test` keeps probing the full descriptor (the `m` filter's existing call-site rationale, unchanged).
- **Invert the narrow-degrade assembler.** In `assembleRightAnchoredRow`, `? help` is **never dropped while it fits**: when the left cluster and the anchor cannot both fit, keep the anchor and shrink the left cluster (`fitLeftCluster` already reserves `rightWidth + 1`, so the fix is the assembler's fallback branch, which today drops the anchor and pads the left cluster); when only the anchor fits, render it alone right-aligned; below that, render the row empty. Keep the existing `· …` ellipsis marker on a truncated cluster — §14.4 forbids wrapping and truncating a **label**, which the marker is not — and keep `fitClusterToWidth` unchanged so the filter footers degrade identically. Pin the thresholds by test, as §2.7 already does for its own steps.
- **Keep the dispatch guard honest for `t`.** Adding `t` to both static descriptors puts it under the existing descriptor↔dispatch probe, so the guard's seed model must wire a **fake `ThemeEnumerator`** (task 8-7's nil-seam no-op would otherwise make the probe pass vacuously against a key that did nothing). Mirror the `m` probe's discipline — the guard seed keeps detection unwired and `colourless` false so neither block is armed. The panel *scope*'s guard coverage remains Phase 9's; this is the page scopes' existing guard picking up one new entry each.
- **Move the goldens deliberately.** Every existing footer and `?` help assertion changes with this task (the dropped `navigate`, the two new entries, the `m multi` label, the reordered tail). Update them as part of the change and state in the commit-facing comment that §15.1 names this the amendment, not a regression.

**Acceptance Criteria**:
- [ ] The Sessions footer renders exactly `⏎ attach · / filter · ␣ preview · s switch view · x projects · t theme · m multi` with a right-aligned `? help` at the reference width.
- [ ] The Projects footer renders exactly `⏎ new session · x sessions · e edit · / filter · t theme` with a right-aligned `? help`.
- [ ] `↑↓ navigate` is absent from both footers and present in both help bodies.
- [ ] The `m` entry's footer label is `multi` and its help label is `Multi-select mode`.
- [ ] With the panel blocked under `NO_COLOR`, the `t` entry is absent from **both** the footer and the help body on **both** pages; with `m` blocked, the `m` entry is absent from both on Sessions.
- [ ] The footer's height budget and its render resolve against the same filtered entries — a blocked-state footer's reserved height equals its rendered height.
- [ ] Narrowing drops footer entries from the right one at a time; `? help` survives every step at which it fits, and the row renders empty only below the width where `? help` alone fits.
- [ ] No footer label is ever wrapped or truncated mid-word at any width.
- [ ] `sessionsKeymap()` / `projectsKeymap()` remain unfiltered static functions and `keymap_dispatch_guard_test` passes, including a `t` probe that genuinely opens the panel against a faked enumerator.
- [ ] Every updated footer/help golden asserts the §14.2 copy verbatim rather than a paraphrase.

**Tests**:
- `"it renders the pinned Sessions footer"` — `TestFooterRevision_SessionsPinnedCopy`
- `"it renders the pinned Projects footer"` — `TestFooterRevision_ProjectsPinnedCopy`
- `"it drops navigate from the footers and keeps it in help"` — `TestFooterRevision_NavigateIsNonCore`
- `"it shortens the multi label"` — `TestFooterRevision_MultiLabelIsShort`
- `"it filters t from footer and help together"` — `TestFooterRevision_BlockedThemeKeyFilteredInLockstep` (both pages)
- `"it filters m from footer and help together"` — `TestFooterRevision_BlockedMultiKeyFilteredInLockstep`
- `"it reserves exactly what it renders in a blocked state"` — `TestFooterRevision_BudgetMatchesFilteredRender`
- `"it degrades right-to-left and never drops help"` — `TestFooterRevision_HelpAnchorSurvivesNarrowing` (width table)
- `"it renders help alone then empty at the extremes"` — `TestFooterRevision_ExtremeNarrowLadder`
- `"it never truncates a label"` — `TestFooterRevision_LabelsAreNeverTruncated`
- `"it keeps the static descriptors unfiltered"` — `TestFooterRevision_StaticDescriptorsUnfiltered`
- `"it probes the new t binding through a faked seam"` — `TestKeymapDispatchGuard_ThemeKeyProbe`

**Edge Cases**:
- `↑↓ navigate` is **dropped** from both footers — arrows in a list are a given and are the entry that genuinely deserves non-core status — while staying listed in `?` help.
- Both `t` **and** `m` are promoted to **core** so both appear in the footer as well as help.
- The label is **`m multi`, not `m multi-select`**, buying back ~47px against an arithmetic that fits with ~5px spare and no headroom at the reference 86-column width.
- Footer and `?` help are filtered **in lockstep through the same call-site filter** — advertising a key that only produces a blocked flash is the dead end the proactive block exists to prevent, and help/footer disagreeing about one key is a live inconsistency.
- **Projects needs its own call-site filter** — §9.10 names only `sessionsHelpKeymap()`, but `t theme` is now in the Projects footer too: same mechanism, second call site.
- Filtering only ever removes entries so every blocked-state footer is strictly narrower and §14.3's budget needs no separate case.
- Degradation is **right-to-left** — drop entries from the right until the row fits, never wrap and never truncate a label, since a truncated `x proje…` advertises nothing while costing the same space.
- **`? help` is never dropped**, being right-aligned and the escape hatch that makes every dropped entry recoverable — which **inverts today's assembler**, whose narrow path drops the right anchor first and pads the left cluster.
- Below the width where `? help` alone fits the footer renders empty, consistent with degrade-never-break, and Portal's documented 40-column minimum sits well above it.
- The **static** descriptors stay unfiltered so `keymap_dispatch_guard_test` stays green — and its seed model must wire a faked `ThemeEnumerator` so the new `t` probe is not vacuous against task 8-7's nil-seam no-op.
- Every existing footer and help golden assertion moves with this change, which §15.1 names as the amendment rather than a regression.

**Context**:
> §14.1: "**Drop `↑↓ navigate` from the footer.** Arrows in a list are a given… **Promote both `t` and `m` to core**, so both appear in the footer as well as `?` help."
> §14.2: "**Sessions** — `⏎ attach · / filter · ␣ preview · s switch view · x projects · t theme · m multi` + right-aligned `? help`. **Projects** — `⏎ new session · x sessions · e edit · / filter · t theme` + right-aligned `? help`."
> §14.3: "**The label is therefore `m multi`, not `m multi-select`**… **The footer is filtered in lockstep with `?` help.**… **The Projects footer needs its own call-site filter.**"
> §14.4: "**Rule: drop footer entries from the right until the row fits, and never wrap or truncate a label.**… **`? help` is never dropped.** It is right-aligned and it is the escape hatch that makes every dropped entry recoverable… Below the width where `? help` alone fits, the footer renders empty."
> §14 intro: the revision exists because the feature "would otherwise be near-invisible — `--theme` and `portal theme list` ruled out, the themes directory silent and never seeded, built-in rows indistinguishable from drop-ins, the reserved-slug set invisible, and no active-theme indicator when the panel is closed."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §14.1, §14.2, §14.3, §14.4, §9.10, §15.1

## theming-system-8-15

### Task 8.15: Panel fixture inputs and the two setting-state frames

**Problem**: §13.1 makes the capture harness the **only** route to seeing a visual change before release — Portal cannot be run from a temporary build without disturbing the live daemon and real state — and §9.14 identifies the panel's slot half as the part with **no prior art anywhere**: assigning a theme to a light/dark slot from inside a picker was found in no surveyed tool, so `d`/`l` and the `● dark` / `● light` / `● both` vocabulary have no established shape to borrow and the Paper frames plus these fixtures are the only reference that exists. But the harness cannot render the panel at all: fixtures are one-shot renders with no way to declare persisted keys, no faked union, and — the input §13.3 calls out as **previously unstated** — no cursor position. Without the fourth input the mandated *constant-while-previewing* frame is unreachable, because its whole point is a cursor on a row other than the marked one, otherwise reachable only by arrowing. And the raw keys must be declarable **independently of `--theme`**: `capturetool` always passes the constant shape, and §8.2 makes a non-empty `theme` render a bare `●` with no slot badges, so a fixture built from the nomination alone could only ever produce that one state — leaving the adaptive-pair frame, the badge vocabulary's primary reference, unreachable.

**Solution**: Four declared fixture inputs (palette, raw persisted keys, faked `ThemeEnumerator` row set, cursor position) plus a no-I/O fake for the seam, and the two setting-state fixtures registered and captured.

**Outcome**: `go run ./cmd/capturetool --fixture theme-panel-adaptive-pair --theme nord` renders the panel over the Sessions list with `● light` / `● dark` badges on two rows; `--fixture theme-panel-constant-previewing --theme tokyo-night-day` renders a bare `●` on `nord` while the cursor sits on a different row — the picker idiom made visible — and both frames reach no real themes directory and no prefs.

**Do**:
- **Add the four fixture inputs** to `internal/capture/fixtures.go`, following the existing `initial*` seed idiom: `themeKeys theme.RawKeys` → `Deps.ThemeKeys` (task 8-7's constructor slot); `themeUnion theme.Union` + `themeEnumeration theme.Enumeration` fed to a **`fakeThemeEnumerator`** implementing the seam's `Open` / `Reassemble` / `Resolve` from the declared values with **no I/O**; `themeSlots []theme.SlotResolution` → `Deps.ThemeSlots` (task 8-3's badge source); and `initialThemeCursor string` → `Deps.InitialThemeCursor`, the row identity the panel's cursor lands on.
- **Open the panel through the real path**, not a bespoke "already open" seed: declare `captureKeys` (Phase 4 task 4-2) as a single `t` press, so the frame goes through task 8-7's open and task 8-8's anchor with the faked seam supplying the union. Then `InitialThemeCursor`, when non-empty, re-anchors the panel cursor **by row identity** after the open.
- **The seeded cursor is placement only — it applies no theme.** The frame's palette is the `--theme` nomination's, which is what keeps `capturetool --theme <slug|path>` meaningful on precisely the frames a drop-in author most wants to check; applying a faked row's `Theme` instead would make the flag inert exactly there. Say this in the seed's doc comment.
- **Write the coherence rule into each fixture's doc comment** (it is an authoring obligation the harness cannot check): **`--theme` must name the theme under the cursor**, because §9.2's invariant is that the cursor's row is always what is painted behind the panel. For the adaptive-pair frame at open that resolves to the **dark** slot's theme (`capturetool` runs no gate, so the standing no-answer fallback selects dark); for constant-while-previewing it is the *previewed* theme, deliberately **not** the marked constant. Note why it cannot be automated: an incoherent frame is indistinguishable from a correct one to a reviewer, and §13.4's guard enumerates fixtures and diffs colours, so it passes either way.
- **Register the two fixtures** — `theme-panel-adaptive-pair` and `theme-panel-constant-previewing` — in **both** `FixtureByName` and `FixtureNames()`, or Phase 4 task 4-3's registry drift check fails:
  - **`theme-panel-adaptive-pair`**: raw keys `{Light: "tokyo-night-day", Dark: "nord"}`; a faked union of the three built-ins plus one valid drop-in; slots yielding `● light` on `tokyo-night-day` and `● dark` on `nord`; cursor on `nord`. Captured with `--theme nord`.
  - **`theme-panel-constant-previewing`**: raw keys `{Theme: "nord"}`; the same union; slots yielding a single bare `●` on `nord`; cursor on `tokyo-night-day`. Captured with `--theme tokyo-night-day`. The constant frame completes the panel's specification because the two setting states never coexist on screen.
- **Fixture data is not config discovery**, so §7.1's import guard is untouched — the same reasoning that admits `--theme <path>`. Assert it: the fixture reaches no themes directory, no `prefs.json`, no XDG lookup, and both `internal/capture`'s no-real-config guard and `TestPortalBinaryDoesNotImportCapture` stay green.
- **Write one `.tape` per fixture** in the existing idiom (`go run ./cmd/capturetool --fixture <name> --theme <…>`, fixed font/size, `Sleep`, `Screenshot`), and **verify a fresh write before trusting or reviewing each PNG** — confirm the file's hash changed and retry on failure. VHS reports no error when it fails to write, every capture here is a first-time write through a freshly-written tape, and a theme change is visible **only** in the image, so an unverified capture reads as either "the change didn't render" or a false pass.
- **Retention**: tapes and images are scaffolding under §13.2 — created as work proceeds, cleared at sign-off (Phase 10). The Go fixture definitions and the harness are **permanent**; §13.4's guard drives them.
- **Read the two Paper artboards as reference, never truth** — `Theme slide-over — A (inline slot badges)` and `… (constant set, previewing another)` use per-frame literal hexes, so §9.1's token table is the authority for every surface (the frames' `#0C0C16` body and `#2B3050` border are explicitly not adopted, per task 8-6).

**Acceptance Criteria**:
- [ ] A fixture can declare all four inputs and render the panel with no real themes directory and no prefs file.
- [ ] `theme-panel-adaptive-pair` renders two badge rows — `● light` on `tokyo-night-day`, `● dark` on `nord` — with the cursor on `nord`.
- [ ] `theme-panel-constant-previewing` renders a single bare `●` on `nord` with the cursor on a *different* row, and no slot badges anywhere.
- [ ] The seeded cursor changes the highlighted row only — the rendered palette is the `--theme` nomination's in both fixtures (asserted by rendering one fixture under two `--theme` values and diffing).
- [ ] Both fixtures appear in `FixtureByName` **and** `FixtureNames()`, and task 4-3's drift check passes.
- [ ] Both fixtures are picked up by §13.4's swap-and-diff guard with no test edit (the guard enumerates).
- [ ] The fake enumerator performs no file or directory access on any of its three methods.
- [ ] `internal/capture`'s no-real-config import guard and `TestPortalBinaryDoesNotImportCapture` both pass.
- [ ] A `.tape` exists per fixture and each captured PNG is verified as a **fresh** write (hash changed) before review.
- [ ] Each fixture's doc comment states its coherence pairing (`--theme` ↔ cursor row) explicitly.

**Tests**:
- `"it declares all four panel inputs"` — `TestPanelFixture_FourInputs`
- `"it renders the adaptive-pair badges"` — `TestPanelFixture_AdaptivePairBadges`
- `"it renders a bare dot while previewing another row"` — `TestPanelFixture_ConstantWhilePreviewing`
- `"it treats the cursor seed as placement only"` — `TestPanelFixture_CursorSeedDoesNotApplyATheme`
- `"it takes its palette from the theme flag"` — `TestPanelFixture_PaletteFollowsTheThemeFlag`
- `"it registers in both lists"` — `TestPanelFixture_RegisteredInBothRegistries`
- `"it reaches no config"` — `TestPanelFixture_NoConfigAccess`
- `"its fake enumerator does no I/O"` — `TestFakeThemeEnumerator_NoIO`
- `"it is enumerated by the swap-and-diff guard"` — `TestPanelFixture_UnderTheGuard`

**Edge Cases**:
- A panel fixture has **four** inputs — the `--theme` palette, the raw persisted theme keys, the faked `ThemeEnumerator` row set, and the **cursor position** — and the fourth was previously unstated yet is required: §9.2 puts the cursor on the active theme at open, so a fixture that cannot declare it cannot render the mandated constant-while-previewing frame at all, that frame's whole point being a cursor on a row other than the marked one, otherwise reachable only by arrowing while fixtures are one-shot renders.
- The raw keys are declared **independently of `--theme`** because `capturetool` always passes the constant shape and §8.2 makes a non-empty `theme` render a bare `●` with no slot badges — a fixture built from the nomination alone could only ever produce that one state, leaving the adaptive-pair frame unreachable.
- The **coherence rule** is an authoring rule the harness cannot check — `--theme` must name the theme **under the cursor**, since the cursor's row is always what is painted behind the panel — and an incoherent frame is indistinguishable from a correct one to a reviewer while §13.4's guard enumerates and diffs colours and passes either way.
- For the adaptive-pair fixture at open that resolves to the **dark** slot's theme (`capturetool` runs no gate, so the standing no-answer fallback selects dark); for constant-while-previewing it is the *previewed* theme, deliberately **not** the marked constant.
- The constant frame completes the panel's specification because the two setting states never coexist on screen — it is the picker idiom made visible.
- Fixture data is not config discovery, so §7.1's import guard is untouched by the same reasoning that admits `--theme <path>`.
- Every new fixture is registered in **both** `FixtureByName` and `FixtureNames()` or Phase 4 task 4-3's drift check fails.
- The new fixtures inherit §13.4's three assertions automatically, which is the point of enumerating rather than naming, and §9.1's token table records that the panel fixtures are what cover `accent.mode` and `accent.attention` outside their transient main-screen states.
- **VHS fails silently on write** — verify the file's hash changed and retry before trusting or reviewing a capture, since every capture here is a first-time write through a freshly-written tape and a theme change is visible only in the image.
- Tapes and images are scaffolding created as work proceeds and cleared at sign-off (Phase 10) while the Go fixture definitions are permanent.
- The two Paper artboards are the only reference that exists for the slot half, and they use per-frame literal hexes so they are reference, never truth.

**Context**:
> §13.3: "**A fixture declares its own raw persisted theme keys, independently of `--theme`.**… **A panel fixture has four inputs:** the `--theme` palette, the raw persisted theme keys, the faked `ThemeEnumerator`'s row set, and **the cursor position**. The fourth is required and was previously unstated."
> §13.3: "**The coherence rule, stated generally: `--theme` must name the theme under the cursor.**… This is an authoring rule the harness cannot check: an incoherent frame is indistinguishable from a correct one to a reviewer, and §13.4's guard enumerates fixtures and diffs colours, so it passes too."
> §9.14: "**The slot half has none.** Assigning a theme to a light/dark slot *from inside a picker* was found in no surveyed tool… That is not a reason to avoid them; it is the reason there is **no established shape to borrow**, so the Paper frames and §13.3's fixtures are the only reference that exists."
> §13.3: "**The harness is known to fail silently on write, and this feature is unusually exposed to it.**… **Mitigation, procedural and mandatory: verify a fresh write before trusting or reviewing a capture** — confirm the file's hash changed — and retry on failure."
> §9.14: "**Caution when reading any Paper frame:** the mocks use **per-frame literal hexes**, so the same token can carry different values across frames. The frames are reference, never truth."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §13.1, §13.2, §13.3, §13.4, §9.2, §9.14, §8.2

## theming-system-8-16

### Task 8.16: The five remaining panel fixtures

**Problem**: Task 8-15 makes the panel capturable and delivers the two setting-state frames, but five specified surfaces still have **no way to be seen before release** — and §13.4's guard structurally cannot report their absence, because it enumerates whatever fixtures exist, so a missing fixture reads as coverage rather than as a gap. Each of the five carries something no other frame does. The **invalid row** is the only place the four-element composition priority is observable at all — particularly the case where the row is *also* persisted, so the badge competes for the right edge and the reason becomes the first element dropped. The pinned **`⚠ dir unreadable`** row has its own placement rule, its own token and its own pinned copy, and no other way to be checked. The **narrow degraded** panel is the only observable check on the width ladder between preferred and minimum. The **paginating** panel is required by §11.2's own coverage consequence: pagination dots render only when the panel's list overflows, so without a fixture carrying enough rows §13.4's guard is blind at exactly the new `bubbles/list` instance the panel adds. And the **panel over Projects** exists because `t` is bound there, Projects now carries its own flash slot, and every other panel fixture is implicitly Sessions-based.

**Solution**: Five more fixtures on task 8-15's four inputs, each declaring the minimum union, keys and cursor that make its surface visible, registered in both registry lists with a `.tape` apiece.

**Outcome**: Every panel surface specified in Phase 8 can be rendered offline and reviewed as an image, and §13.4's guard covers the panel's list instance including its pagination dots.

**Do**:
- **`theme-panel-invalid-row`** — a union mixing valid built-ins with **three** invalid rows: one carrying only a reason (glyph + `text.subtle` label + `accent.attention` reason), one that is **also persisted** (badge present, reason dropped — the composition priority made visible), and one whose label is long enough to hit the truncation floor. Cursor on a valid row (arrows skip invalid ones, so the cursor can never rest there).
- **`theme-panel-dir-unreadable`** — `Union.DirUnusable = true` with the built-ins plus a persisted row beneath, so the frame shows the pinned `⚠ dir unreadable` chrome directly under the header **and** rows still rendering beneath it (a user with an unreadable directory must not lose the `●`). Size the capture so more than one page of rows exists, and take the shot on page 2 (via a `Ctrl+↓` in `captureKeys`) so the frame demonstrates the row is chrome, not a paginated list member.
- **`theme-panel-narrow`** — the same union as the adaptive-pair frame, captured through a tape whose terminal width lands the panel in the **degraded band** between preferred and minimum (VHS sets the terminal in pixels via `Set Width` / `Set FontSize`, so the tape's comment must record the intended column count it resolves to). It is the only observable check on task 8-11's ladder.
- **`theme-panel-paginated`** — a union of enough rows to overflow the panel body at the capture height (built-ins plus ~25 synthetic valid drop-in rows), so the panel's pagination dots actually render. Required by §11.2's coverage consequence, and the row that makes §13.4's guard non-blind at the panel's `bubbles/list` instance.
- **`theme-panel-projects`** — the panel over the **Projects** page: reuse the existing `projects` fixture's project data and declare `captureKeys` of `x` then `t` (the `projects` fixture already reaches its page through an `x` press), with the adaptive-pair keys and union so the badges are visible there too.
- **Every frame declares its own palette, raw keys, faked union and cursor coherently** under task 8-15's rule — `--theme` names the theme under the cursor — and the union is faked wholesale, so no real themes directory is needed and §7.1's import guard is untouched.
- **Register all five in both `FixtureByName` and `FixtureNames()`** (task 4-3's drift check enforces the pair), and write a `.tape` per fixture, **verifying a fresh write** (hash changed) before trusting or reviewing each capture — the same mandatory VHS mitigation as task 8-15.
- **Do not build Phase 9's frames**: the two message-slot frames (the slot-from-constant confirm and the failed-commit line) and the minimum-height-with-a-message frame are commit-path states and fall in Phase 9. Phase 8's message slot stays empty.

**Acceptance Criteria**:
- [ ] `theme-panel-invalid-row` renders a `text.subtle` label with an `accent.attention` `⚠` and terse reason on **one** line, plus a persisted-and-invalid row where the badge is present and the reason is absent, plus a truncated label ending in `…`.
- [ ] `theme-panel-dir-unreadable` renders `⚠ dir unreadable` directly beneath the header **on page 2**, with built-in and persisted rows still rendering beneath it.
- [ ] `theme-panel-narrow` renders the panel at a width strictly between `themePanelMinWidth` and `themePanelPreferredWidth`, with every row still exactly one line.
- [ ] `theme-panel-paginated` overflows the panel body so the pagination dots render, and §13.4's guard exercises those dots.
- [ ] `theme-panel-projects` renders the panel over the Projects page with its badges visible and the Projects footer beneath cut mid-label by the overlay.
- [ ] All five appear in both registries and task 4-3's drift check passes.
- [ ] All five are enumerated by §13.4's guard with no test edit.
- [ ] No fixture reaches a real themes directory, `prefs.json` or an XDG lookup; both import guards stay green.
- [ ] A `.tape` exists per fixture and each PNG is verified as a fresh write before review.
- [ ] No confirm, failed-commit or minimum-height-with-message fixture is added in this phase.

**Tests**:
- `"it renders the invalid-row composition priority"` — `TestPanelFixture_InvalidRowFrame`
- `"it drops the reason when a badge competes"` — `TestPanelFixture_InvalidPersistedRowDropsTheReason`
- `"it pins the directory row on page two"` — `TestPanelFixture_DirUnreadableIsChromeOnPageTwo`
- `"it renders rows beneath the directory row"` — `TestPanelFixture_RowsBeneathDirRow`
- `"it renders in the degraded width band"` — `TestPanelFixture_NarrowIsBetweenMinAndPreferred`
- `"it paginates and draws the dots"` — `TestPanelFixture_PaginatedDrawsDots`
- `"it renders over the Projects page"` — `TestPanelFixture_OverProjects`
- `"it registers all five in both lists"` — `TestPanelFixture_AllRegistered`
- `"it reaches no config"` — `TestPanelFixture_RemainingFramesNoConfigAccess`
- `"it adds no Phase 9 frame"` — `TestPanelFixture_NoMessageSlotFixtures`

**Edge Cases**:
- The **invalid-row** frame must show the `text.subtle` label with the `accent.attention` `⚠` and terse reason on one line, and is the only place the composition priority is observable — including a row that is *also* persisted, where the badge competes and the reason is the first element dropped.
- The **`⚠ dir unreadable`** frame has its own placement rule, token and pinned copy and **no other way to be checked**, and must show built-in and persisted-slug rows still rendering beneath the pinned row.
- The **narrow degraded** frame is the only observable check on the width ladder between preferred and minimum.
- The **paginating** frame is required by §11.2's coverage consequence — pagination dots render only when the panel's list overflows, so without a fixture carrying enough rows §13.4's guard is blind at exactly the new `bubbles/list` instance the panel adds.
- The **panel over Projects** frame is required because `t` is bound there, Projects now carries its own flash slot, and every other panel fixture is implicitly Sessions-based.
- The minimum-height-with-a-message frame and the two message-slot frames (confirm, failed commit) are **Phase 9's**, both contenders being commit-path states.
- A missing fixture is a blind spot the guard structurally cannot report, since §13.4 enumerates whatever exists and absence reads as coverage.
- Each frame declares its own palette, raw keys, faked union and cursor **coherently** under task 8-15's rule, and the union may be faked wholesale so no real themes directory is needed and the import guard stays untouched.
- Every fixture is registered in both registry lists, and the same VHS fresh-write verification applies to every capture.

**Context**:
> §13.3: "**New fixtures are added for the slide-over**, so every specified panel surface is visible during implementation rather than at release… **An invalid-theme row**, and **the pinned `⚠ dir unreadable` row** — the latter has its own placement rule, token and pinned copy, and no other way to be checked… **The narrow degraded panel**… **A panel long enough to paginate** (§11.2)… **The panel over the Projects page**… A missing fixture is a blind spot the guard structurally cannot report: §13.4 enumerates whatever fixtures exist, so absence reads as coverage."
> §11.2: "**Coverage consequence for §13.4:** pagination dots only render when the panel's list paginates, so one of §13.3's panel fixtures must carry enough theme rows to overflow. Otherwise the guard is blind at exactly the new site this paragraph adds."
> §9.5: "**An unreadable themes directory gets its own row** — `⚠ dir unreadable`, **chrome pinned to the viewport directly beneath the header, not a list row**… a list row participates in pagination, so the warning would vanish the moment the user paged down."
> §14A: "**§13.3 accordingly requires one Projects-with-panel fixture**, so the page is seen with the panel over it before release rather than after."
> Phase boundary: the phase note records that the confirm, failed-commit and minimum-height-with-message fixtures fall in Phase 9 — both message-slot contenders are commit-path states.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §13.3, §13.4, §11.2, §9.5, §9.8, §14A
