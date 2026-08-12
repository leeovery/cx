TASK: theming-system-12-2 — Bind The Commit Path's In-Memory Key Mirror To What Prefs Actually Writes

ACCEPTANCE CRITERIA:
1. The constant-clears-slots and slot-clears-constant transformations appear once each in `internal/theme`, and both TUI commit handlers call them.
2. No `theme.RawKeys` literal in `internal/tui/theme_panel_commit.go` re-states either rule.
3. At least one test asserts the post-commit model keys equal the keys read back from the written `prefs.json`, in both commit directions.
4. A failed write still leaves the keys untouched and does not recompute.

STATUS: complete

SPEC CONTEXT:
Specification §8.2 (`specification.md:760`) states the rule the task is single-sourcing: "Mutual exclusion is enforced on write. Committing a constant clears both slots; assigning a slot clears the constant. Whichever was set last wins." §9.1 (`:992-994`) pins the keypresses (`Enter` → `theme = <selection>` + clear both slots; `d`/`l` → slot + clear constant), and §8.2 (`:766`) notes the hand-edited-file consequence — under a constant the slots are unread, so a stale slot only becomes visible on the *next* commit. That last point is exactly the observability limit the added cmd test navigates.

IMPLEMENTATION:
- Status: Implemented (mechanism partly superseded by later in-plan tasks; outcome intact)
- Location:
  - `internal/theme/setting.go:31-46` — `RawKeys.WithConstant(slug)` (returns `RawKeys{Theme: slug}` — structural clear of both slots) and `RawKeys.WithMember(m Member, slug)` (returns a fresh pair with the named half replaced, the other carried across, constant dropped).
  - `internal/tui/theme_panel_commit.go:45-50` (`commitConstant` → `keys.WithConstant(slug)`) and `:86-91` (`commitSlot` → `keys.WithMember(member, slug)`), both threaded through the shared `commit(write, mirror)` protocol at `:31-43`.
  - `internal/prefs/store.go:220-247` — the file-side rules the methods mirror (`SaveTheme` clears both slots; `SaveThemeSlot` clears the constant, leaves the other slot).
  - `internal/theme/theme_test.go:200-201` — the exported-surface guard enrols `RawKeys.WithConstant` / `RawKeys.WithMember`, so a silent rename/removal fails.
- Criterion 1: met. Each rule is stated exactly once (`setting.go:33-35`, `:41-46`); a repo-wide grep for `RawKeys{` outside `internal/theme` finds only `cmd/open.go:509`'s zero-value error return and test fixtures.
- Criterion 2: met. `theme_panel_commit.go` contains no `theme.RawKeys` composite literal at all; both handlers pass a one-line mirror closure.
- Criterion 4: met. `commit` (`:31-43`) returns on the write error *before* the mirror and `recomputeThemePanel`, so keys and badges are untouched.
- Notes on drift vs the task text: the task named `WithSlot(slot Slot, …)`; the shipped method is `WithMember(m Member, …)`, and the `mirrorThemeSlot` helper the task told the executor to rewrite no longer exists. Both are later, deliberate in-plan supersessions — 12-9 moved the panel onto the domain `theme.Member` type (converting only at the persister seam, `cmd/theme_persister.go:59-64`) and 17-2 extracted the single `commit(write, mirror)` protocol. The `Member` narrowing is a genuine improvement over the planned `Slot`: `Slot` carries a constant position that names no half of the pair, so `WithMember` is total without a default arm, and the `if MemberLight … else dark` shape matches the package's own idiom (`Member.Opposite`, `Member.Slot` in `internal/theme/member.go:12-22`). Not drift against intent.

TESTS:
- Status: Adequate
- Coverage:
  - `internal/theme/setting_test.go:449-457` — `WithConstant` over a constant-plus-stale-pair holds only the constant.
  - `internal/theme/setting_test.go:459-518` — table over `WithMember`: light/dark over a constant, either half of a pair with the other carried across, constant-beside-stale-pair, and empty-slug-empties-the-half. Six cases, no redundancy.
  - `internal/theme/setting_test.go:520-532` — both transformations leave the receiver alone and both actually move (the second clause keeps the first from passing vacuously). Not vacuous under Go semantics: a switch to a mutating pointer receiver would still compile at these addressable call sites and would fail here.
  - `cmd/open_theme_commit_test.go:143-179` — `assertBadgesMatchPersistedKeys` is the criterion-3 join: one non-migrating read-back (`loadPrefsStoreNoMigrate().LoadThemeKeys()`), resolved through the production `themeResolution` → `theme.Badges`, compared against the `●` markers parsed off the rendered frame (which are a pure projection of the model's unexported `themeState.keys` via `applyCommittedSetting`, `theme_panel_commit.go:120-127`).
  - Both required directions are bound: constant-over-a-pair at `:286-304` (`ConsecutiveCommitsStayBoundToPrefs`, seeded `theme_light`+`theme_dark`) and `:236-255`; slot-over-a-constant at `:257-281` (seeded `theme`, `d` → confirm → assert).
  - Criterion 4 in both commit directions: `internal/tui/theme_panel_commit_test.go:267-297` and `theme_panel_commit_slot_test.go:483` (keys untouched, panel stays open, theme stays applied), plus `theme_panel_commit_recompute_test.go:426` ("the failed commit ran 0 reassemblies").
  - Existing `internal/tui` commit tests still assert the mirrored keys against independent literals (`requireConstantKeys`, `theme_panel_commit_test.go:98-103`) — the expectation is expressed separately from the production rule, which is the right call and keeps the unit tests meaningful rather than tautological.
- Notes: the badge set is a lossy projection of the keys, and the test says so at `:140-142` — under a constant the slots are unread, so a constant commit that failed to clear the slots moves no badge *on that frame*. The chosen mitigation is sound: `ConsecutiveCommitsStayBoundToPrefs` commits a constant over a real pair and then a slot over that constant, at which point a surviving stale slot would badge a slug in memory that the file does not name, and the comparison fails. I traced each way `WithConstant`/`WithMember` could regress (fail to clear slots, fail to clear the constant, fail to carry the other half) and each is caught by at least one of the three badge-bound tests, except the "other half carried across" case in a pure pair→pair commit, which is only covered by `internal/tui`/`internal/theme` unit tests (see the note below). No over-testing found — the added cases are distinct and each would fail on a real regression.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays hex-free and log-free; `prefs` remains a leaf (the task's stated blocker on full single-sourcing is respected — the conversion lives at `cmd/theme_persister.go:59-64`, not in `prefs`); comments are declarative and free of task/phase references.
- SOLID principles: Good. The rule moves to the type that owns the data (`RawKeys`), and the TUI keeps only the call. `commit(write, mirror)` gives one protocol with two injected behaviours rather than two near-duplicate bodies.
- Complexity: Low. Two total functions, one branch each, no default arm to reason about.
- Modern idioms: Yes. Value receivers returning fresh values; the `With…` naming reads as the transformation it is.
- Readability: Good. `WithConstant`'s unused receiver is the point of the method ("nothing of the receiver survives") and is documented as such at `setting.go:31-32`; the comment earns its place rather than restating the return.
- Comment accuracy: verified line by line against the code. `setting.go:31-32`, `:37-40` and `theme_panel_commit.go:27-30`, `:84-85` all hold — including the `● both` reachability claim (`WithMember` carrying the other half is what makes `d` then `l` on one row land two slots) and the "totality via Member" claim (`Member` has exactly two values, `member.go:6-9`).
- Issues: none.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `cmd/open_theme_commit_test.go:307-328` — `TestThemePanelCommit_DarkKeyRoundTripsOneSlotToPrefs` is the only round-trip covering a slot commit over an existing *pair* (the direction that exercises `WithMember`'s carry-across of the untouched half) and it is the one round-trip that never calls `assertBadgesMatchPersistedKeys`. Capture the model at `:318` (`m = update(t, m, themePanelDarkSlotKey)` instead of discarding it) and add `assertBadgesMatchPersistedKeys(t, m)` after the `assertPrefsOnDisk` at `:320`; with the seeded `theme_light=aurora` / `theme_dark=nord` and a `sunset` dark commit, both sides derive `aurora → "● light"`, `sunset → "● dark"`, so it passes as written and binds the third direction to disk.
