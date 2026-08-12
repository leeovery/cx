TASK: theming-system-15-9 — Make The ThemeSource Seam Uniform In What It Consumes

ACCEPTANCE CRITERIA:
- All four `ThemeSource` methods take `theme.RawKeys` (plus a slot where relevant); none takes a pre-collapsed `Setting` or a pre-defaulted slug.
- `persistedSlotSlug` is gone and no `theme.Slot` switch selecting a slug remains in `internal/tui`.
- `Setting.Slug(Slot)` performs the shipped-default substitution inside `internal/theme`.
- The `theme: loaded` emission cadence is unchanged, and no seam method reads the filesystem.
- Panel open, `Esc` close, badge recompute and the constant → adaptive commit behave identically.
- `go test ./internal/tui ./internal/theme` passes.

STATUS: complete

SPEC CONTEXT:
The seam itself is an internal architecture concern, but the two rules it moves are spec-governed. §5.8 (spec:414) fixes the panel's enumeration as retained for its lifetime so arrowing previews from values already in hand — the "no fresh parse" constraint the seam's no-I/O guarantee implements. §8.4/§8.5 (spec:805, 809) make the constant → adaptive conversion's newly-live slot the one theme load that happens outside construction, resolved from the retained enumeration with the shipped default substituted for an untouched slot (so it is *not* reported as a fallback of a slug nobody set). §12.3 (spec:1485) pins the `theme: loaded` cadence: construction lines plus the one commit-time line, carrying the slug that actually rendered. This task changes none of that behaviour — it relocates the collapse and the substitution from the render layer into `internal/theme`, which owns them.

IMPLEMENTATION:
- Status: Implemented (mechanism later refined by task 17-4 and 17-3, both in-plan)
- Location:
  - `internal/tui/theme_seams.go:11-16` — all four methods now take `theme.RawKeys`; `ResolveSlot(e, slot, keys) (SlotResolution, error)` was subsequently narrowed by 17-4 to `LoadSlot(e, slot, keys) error` (the panel never used the returned record).
  - `internal/theme/setting.go:86-95` — `Setting.Slug(Slot)`, shipped-default substitution per slot, empty for an unset constant.
  - `internal/theme/setting.go:101-104` — `SlugForSlot(keys, slot)` (added by 17-4) is the single ResolveSetting+Slug pairing; `internal/theme/slug_collapse_guard_test.go` is a repo-wide source guard proving it is the only one outside `internal/theme`'s own tests.
  - `internal/theme/dir_theme_source.go:26-39` — the production adapter collapses internally (`ResolveSetting` for `Resolve`, `SlugForSlot` for `LoadSlot`); neither touches `Dir`.
  - `internal/tui/theme_panel_confirm.go:72-77` — `loadNewlyLiveSlot` hands the mirrored raw keys straight over; `persistedSlotSlug` deleted.
  - `internal/tui/theme_panel.go:199-203` — `themeSetting()` survives for exactly one consumer, the confirm gate at `internal/tui/theme_panel_commit.go:74` (`IsConstant`), with a doc comment narrowed to that.
  - Fakes updated in one pass: `internal/capture/theme_fake.go:42,51`, `internal/tui/theme_source_fake_test.go:51-61`, `internal/tui/theme_seams_test.go:27-34`.
- Notes:
  - Criterion-by-criterion: (1) satisfied — no `Setting` or bare slug crosses the seam anywhere (`grep` finds no surviving `Resolve(… theme.Setting)` / `ResolveSlot(… slug string)` implementation outside `Loader.ResolveSlot`, which is the rule body inside the owning package). (2) satisfied — `persistedSlotSlug` is gone repo-wide, and the only `theme.Slot` switches left in `internal/tui` are `inForceSlot`'s slot *match* (`theme_panel.go:207-215`) and `memberForSlot` in a test, neither selecting a slug. (3) satisfied. (4) cadence verified structurally: `Resolve` → `Loader.ResolveNominationFrom` → `enumerationPass` → `reportFallback` (no `Loaded`), `LoadSlot` → `Loader.ResolveSlot` → `commitPass` → `reportSlot` (emits `Loaded`) — `internal/theme/resolution.go:104-110,193-206`; no filesystem read in `Resolve`/`Reassemble`/`LoadSlot`. (5) behaviour preserved: the old `persistedSlotSlug` read `Setting.Light/Dark` off `ResolveSetting`, whose adaptive branch already substituted the defaults, so for the only reachable input (mirrored keys, constant cleared by `RawKeys.WithMember`) the new `SlugForSlot` answers identically.
  - The task text's "none of the four methods may read the filesystem" is only true of the resolving three — `Open` enumerates the directory by design (`DirThemeSource.Open` → `Assembler.Open` → `Loader.OpenEnumeration`). The implementation reads it that way correctly; the seam's doc comment does not (see notes below).
  - Not drift: 17-4's `ResolveSlot` → `LoadSlot` narrowing and 17-3's re-pointing of `Setting.Slug` at `defaultSlugFor` are later in-plan tasks that tightened this one's outcome rather than superseding it — both leave every acceptance criterion here satisfied.

TESTS:
- Status: Adequate (one weak test, minor redundancy)
- Coverage:
  - `Setting.Slug` unit table for a constant, both halves of a pair, both unset slots and an unset constant — `internal/theme/setting_test.go:304-356`; reinforced by `internal/theme/slot_default_test.go:44-49`.
  - Collapse-through-the-adapter: `internal/theme/dir_theme_source_test.go:11-35` (`SlugForSlot` table incl. tiebreak and control-stripping), `:39-62` (`LoadSlot` emits exactly one `theme: loaded` carrying the collapsed default), `:64-80` (`Resolve` emits none) — this is the cadence criterion pinned directly on the production adapter.
  - Production-vs-fake parity of the derived slug: `internal/tui/theme_slot_collapse_test.go:11-43` — the one thing a fake could plausibly get wrong now that the input is raw keys.
  - Seam shape: compile-time assertions for the fixture fake and the exported adapter (`internal/tui/theme_seams_test.go:36-41`); `Setting.Slug` added to the exported-API guard (`internal/theme/theme_test.go` `wantExports`).
  - Behavioural pins unchanged and re-pointed at the new input rather than relaxed: open hands the seam the uncollapsed construction snapshot (`theme_panel_open_test.go:479-495`), `Esc` re-resolution counts (`theme_panel_close_test.go:127-155`), commit-time load cadence and fallback naming (`theme_panel_commit_load_test.go:331-384`), nil-persister inertness (`theme_panel_confirm_test.go:731-766`), and the shared-rule-body equivalence between the slot path and the badge path (`theme_panel_commit_load_test.go:549-576`).
- Notes:
  - `internal/tui/theme_seams_test.go:99-123` no longer tests what its name says: after 17-4 narrowed `LoadSlot` to an error-only return, the assertions run against a *fresh* `loader.ResolveSlot(...)` call the test makes itself, so the only property it pins about the seam is that `LoadSlot` returns nil. It would stay green if `LoadSlot` loaded the wrong slot entirely. See non-blocking notes.
  - Mild redundancy: the unset-slot substitution is now pinned five times (`Setting.Slug` unit, `slot_default_test`, `SlugForSlot` table, adapter emission, fake/production parity). Each has a genuinely different subject (accessor / pairing guard / collapse helper / emission / parity), so this reads as layered rather than bloated — not flagged as over-testing.
  - Test execution not attempted (verification is by reading, per the review protocol); nothing read suggests a compile or assertion break.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays the owner of the `theme` log component (the seam comment states it and the code holds to it — `internal/tui` binds no `log.For("theme")`); the DI seam remains a small interface with production and fake implementations; no logging added to a leaf.
- SOLID principles: Good — this is a net improvement in exactly the way the finding described. The resolution and defaulting rules now live only in the package that owns them, so the render layer can no longer derive a setting that disagrees with what it lists and marks, and the source guard (`slug_collapse_guard_test.go`) makes the single-collapse property structural rather than disciplinary.
- Complexity: Low. `Setting.Slug` is a three-arm switch over `cmp.Or`; `SlugForSlot` is two lines; the panel lost a helper.
- Modern idioms: Yes — `cmp.Or` for the substitution, total switch with a `default` arm so the accessor cannot fall off the end.
- Readability: Good. Naming distinguishes the halves cleanly (`Slug` = per-slot read of a collapsed setting; `SlugForSlot` = the collapse from raw keys). `LoadSlot`'s error-only return correctly signals that the palette belongs to `Resolve`'s nomination.
- Issues: One stale comment on the seam and one stale test failure-message (both below); neither affects behaviour.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_seams.go:7-8 — the doc block claims "No method reads the filesystem", which `Open` falsifies: `DirThemeSource.Open` calls `Assembler.Open` → `Loader.OpenEnumeration(dir)` and enumerates the themes directory. This task wrote the accurate form ("None of the three reads the filesystem"); a later comment sweep over-generalised it. Replace the sentence with: "Only Open reads the filesystem; the other three resolve against the enumeration it retained, since a further parse of the same slug could disagree with the row on screen."
- [quickfix] internal/tui/theme_seams_test.go:99-123 — `TestThemeSourceLoadsASlotFromTheRawKeys` asserts on a `loader.ResolveSlot(...)` call it makes itself, not on anything `source.LoadSlot` did, so a wrong slot or slug inside `LoadSlot` would not fail it. Either build the source over a capture logger (`logtest.NewCaptureLogger` + `theme.NewEventLogger`) and assert the emitted `theme: loaded` carries `theme.DefaultDarkSlug` and `slot=dark`, as `internal/theme/dir_theme_source_test.go:39-62` does, or delete it as superseded by that test plus `internal/tui/theme_slot_collapse_test.go`.
- [do-now] internal/tui/theme_panel_confirm_test.go:760 — the failure message's reason ("the un-mirrored keys' slots are both empty, which is the shape a load run here would resolve") no longer holds: since this task a load run here collapses through `SlugForSlot`, so the empty slots resolve the shipped defaults and no fallback would be reported. Replace the trailing clause with: "— a load run here would collapse the un-mirrored constant's empty slots onto the shipped defaults and report a load for a write that never happened".
