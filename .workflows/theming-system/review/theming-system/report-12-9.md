TASK: theming-system-12-9 — Hold The Domain Light/Dark Type In The Panel And Convert Once At The Persister Seam

ACCEPTANCE CRITERIA:
- No `prefs.ThemeSlot` value appears in `internal/tui` outside the single persister-seam call (or not at all, if the seam takes the domain type).
- Exactly one domain→persistence slot conversion exists in the codebase.
- No conversion has a `default` arm that maps an unknown slot to a valid one.
- The `theme` component's commit log lines carry the same `slot` attr values as today.

STATUS: complete

SPEC CONTEXT:
Spec §12.3 (line 1491) pins the event: `theme: commit failed` is WARN, per failed write, carrying `slug`, `slot` (absent when committing a constant) and `reason`. Spec §8.9 (line 904) pins ownership: the panel's commit write is `cmd`-owned via an injected persister (`WithThemePersister`), because `prefs` is a leaf that must not import `internal/log`, the write needs prefs path resolution, and the persister is the sole emission site for `theme: commit failed`. Nothing in the spec dictates which type the panel carries for light/dark — that is exactly the architecture-level layering the task addresses, and the spec's constraint (prefs is a leaf, so the two vocabularies must stay separate types) is respected by keeping the conversion at the `cmd` seam.

IMPLEMENTATION:
- Status: Implemented (later refined by 17-2 / 17-4, which folded `commitSelectedSlot` into the shared `commitSelected` + `commit` protocol; the task's outcome survives that refactor intact)
- Location:
  - `internal/tui/model.go:76-79` — `ThemePersister.CommitThemeSlot(slug string, member theme.Member)`; the seam itself now takes the domain type, so the conversion is entirely behind it.
  - `internal/tui/theme_panel_commit.go:73` (`handleSlotCommitKey(member theme.Member)`), `:86` (`commitSlot(slug string, member theme.Member)`), `:89` (mirror is `keys.WithMember(member, slug)`).
  - `internal/tui/theme_panel_confirm.go:12-15` (`themeSlotConfirm{slug, member theme.Member}`), `:23` (`raiseSlotConfirm`), `:57` (`confirmSlotAssignment`), `:72-77` (`loadNewlyLiveSlot(assigned theme.Member)` → `assigned.Opposite()`).
  - `internal/tui/theme_panel.go:331,333` — the `d`/`l` dispatch passes `theme.MemberDark` / `theme.MemberLight`.
  - `cmd/theme_persister.go:34-39,59-64` — the single seam conversion `prefsSlotFor(member theme.Member) prefs.ThemeSlot`.
  - `internal/theme/setting.go:37-46` — `RawKeys.WithMember` (pre-existing) is now the only mirror, replacing the deleted `mirrorThemeSlot`.
- Deleted as required: `oppositeThemeMember`, `mirrorThemeSlot`, `themeSlotFor`, `commitSelectedSlot` — verified absent repo-wide by grep.
- AC1: met. `prefs.ThemeSlot` / `prefs.Slot*` appear nowhere in `internal/tui` (production or test) — only `cmd/theme_persister.go` and its test name them. `internal/tui` still imports `prefs` for `SessionListMode` (`build.go`), which is unrelated and correct.
- AC2: met. `prefsSlotFor` (`cmd/theme_persister.go:59`) is the only domain→persistence conversion; it is the only caller of `prefs.SaveThemeSlot` outside the store.
- AC3: met. `prefsSlotFor` is a two-armed `if` over the 2-valued `theme.Member` (zero = `MemberDark`), so there is no `default` arm and no path that can produce `SlotConstant`. `prefs.SaveThemeSlot` (`internal/prefs/store.go:232-235`) keeps the structural store-side rejection the task asked to retain.
- AC4: met and behaviour-preserving. `themeSlotAttr` (`cmd/theme_persister.go:52-55`) renders via `member.Slot().AttrName()`, and `Member.Slot()` (`internal/theme/member.go:19-24`) returns only `SlotLight`/`SlotDark`, both of which `AttrName` names — so the attr is always present with the same `"light"`/`"dark"` values as before. `CommitTheme` (constant) still carries no `slot`, matching spec §12.3.
- Notes: `loadNewlyLiveSlot` preserves the old `oppositeThemeMember` semantics exactly (assign one half → load the other), now as a domain-to-domain `Member.Opposite()` call. The commit diff (4051241f) is scoped to the retype plus test updates; no behavioural change is smuggled in.

TESTS:
- Status: Adequate
- Coverage:
  - Retyped panel tests (task test item 1): `internal/tui/theme_panel_commit_slot_test.go` (`TestPanelSlotCommit_DarkWritesTheDarkSlot`, `..._LightWritesTheLightSlot`, `..._FailedWriteLeavesKeysAlone`), `theme_panel_confirm_test.go`, `theme_panel_commit_failure_test.go`, `theme_panel_behaviour_test.go` — all assert against `slotCommit{slug, member}` recorded by the `fakeThemePersister` (`theme_persister_seam_test.go:18-42`), so a transposed half fails. `d` and `l` are asserted separately, which is what catches an inverted mapping.
  - Persister receives the right prefs slot (task test item 2): `cmd/theme_persister_test.go:40-61` (`TestThemePersister_CommitThemeSlot`) asserts the on-disk `theme_light` / `theme_dark` for both members, and `:63-75` (`TestThemePersister_MemberToPrefsSlot`) pins `MemberLight→prefs.SlotLight`, `MemberDark→prefs.SlotDark` directly. Both were explicitly requested by the task; the direct table is not redundant with the on-disk one (it pins the conversion independently of the file layout).
  - Commit log `slot` attr for both slots (task test item 3): `cmd/theme_persister_test.go:104-176` (`TestThemePersister_CommitFailedAttrs`) asserts exact attr-key order per commit shape (`slot` absent for a constant, present for each member), the values `"light"`/`"dark"`, and the closed key set. The final subtest reads the expected rendering back off a real `theme.NewEventLogger(...).Loaded(...)` emission rather than restating literals, so the persister's attr cannot drift from the loader's.
  - Regression guard for the AC itself: `internal/tui/theme_panel_commit_slot_test.go:416-479` (`TestPanelSlotCommit_TypedSlotOnly`) walks the package AST and fails on any `prefs.Slot*` / `prefs.ThemeSlot` selector, any `theme.Member(...)` conversion, and any member value beyond the two — plus a reflection assertion that `ThemePersister.CommitThemeSlot`'s second parameter is `theme.Member`. This makes AC1 and AC3 structural rather than one-time. `internal/theme/theme_test.go:185-189` (exported-API surface pin) means a third `Member` value cannot be added silently either.
  - Domain helpers: `internal/theme/nomination_test.go:80-104` pins `Member.Slot()` / `Member.Opposite()` for both halves and the zero value being `MemberDark`; `internal/theme/setting_test.go:477-524` pins `WithMember`'s mutual exclusion.
- Notes: No over-testing attributable to this task — the added assertions are one guard test plus retyped existing ones. Would the tests fail if the feature broke? Yes: a re-introduced `prefs.ThemeSlot` in the panel fails the AST guard; a transposed half fails both the panel slot-commit tests and the on-disk persister test; a lost `slot` attr fails the attr-key equality assertion.

CODE QUALITY:
- Project conventions: Followed. The seam is a 2-method interface injected via `Deps`/`Option` (CLAUDE.md DI pattern); logging stays single-sited in `cmd` (`TestCommitFailed_SingleEmissionSite` enforces `internal/tui` binds no `theme` component); `prefs` remains a no-logging leaf and is not imported for slot purposes by the domain.
- SOLID principles: Good. This is a dependency-inversion cleanup — the TUI now depends only on `internal/theme`'s vocabulary and the persistence vocabulary lives at the edge that owns persistence. `RawKeys.WithMember` gives the mirror one owner instead of a local copy of the rule.
- Complexity: Low. Two conditionals removed net; `commitSlot` is a single `commit(write, mirror)` composition.
- Modern idioms: Yes. `reflect.TypeFor[...]` in the guard, table-driven subtests, `slices.Equal`/`maps.Keys` in the attr assertions.
- Readability: Good. `member`/`assigned` read as halves of a pair; `prefsSlotFor` names its direction correctly (was `themeSlotFor`, which read backwards after the flip).
- Comment accuracy: Comments on the changed code hold. `theme_panel_commit.go:27-30` ("a nil persister is inert…") matches the nil check preceding `write()`; `theme_panel_confirm.go:52-54` ("the persister nil-check cannot be inferred from the nil error") matches `commit` returning nil on the writer-less path; `cmd/theme_persister.go:57-58` correctly states why the two vocabularies stay separate types. No task ids, phases or spec-section references in production comments.
- Security / performance: No surface — pure type threading, no new I/O.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] cmd/theme_persister.go:52 — `themeSlotAttr` discards `AttrName`'s presence bool with no stated reason; add the one-line why above the func: `// A Member is one half of the pair, never the constant, so the name is always present.`
- [do-now] internal/tui/theme_panel_commit_slot_test.go:452-454 — the prefs-slot detection mixes `&&` and `||` unparenthesised; parenthesise to state the intended grouping (identical semantics under Go precedence): `if (isPackageSelector(node, "prefs", "") && strings.HasPrefix(node.Sel.Name, "Slot")) ||` / `\t isPackageSelector(node, "prefs", "ThemeSlot") {`.
- [idea] cmd/theme_persister_test.go — AC2 ("exactly one domain→persistence conversion") is pinned structurally on the `internal/tui` side only; consider whether the `cmd` package warrants a matching AST guard asserting `prefs.Slot*` / `prefs.ThemeSlot` is named only in `theme_persister.go`, or whether the repo's existing guard budget makes that redundant given `SaveThemeSlot` has a single caller.
