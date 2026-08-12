TASK: theming-system-14-5 — Extract the shared commit "no other I/O" test body in internal/tui

ACCEPTANCE CRITERIA:
- The "no other I/O" setup and its five assertions exist once in `internal/tui`.
- Both tests still assert their own commit landed with the value they expect.
- Failure messages still name which commit failed.
- `go test ./internal/tui` passes.

STATUS: complete

SPEC CONTEXT:
Specification §9.4/§5.8 (specification.md:1027) pins the contract this pair of tests guards:
"So a commit re-derives the union (§9.4), re-sorts it (§9.5) … The directory is *not*
re-enumerated — §5.8 pins that to panel open, and a commit changes prefs, not the
directory." The extracted helper's enumeration assertion reproduces that reasoning
verbatim in its failure string ("a commit re-derives from the retained parse and never
re-reads the directory"), so the shared body still states the spec's claim rather than a
weakened paraphrase. The remaining four assertions cover the complementary half — the
prefs write is the single seam call the keypress may make, and no route to disk bypasses
a counted seam.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - Helper: `internal/tui/theme_testing_test.go:73-144` (`requireCommitDoesNoOtherIO`)
  - Caller 1: `internal/tui/theme_panel_commit_test.go:360-367` (`TestPanelEnter_NoOtherIO`)
  - Caller 2: `internal/tui/theme_panel_commit_slot_test.go:627-636` (`TestPanelSlotCommit_NoOtherIO`)
  - Commit: `9c4b132c` (−157/+118 across the three files)
- Notes:
  - Every item in the task's "Do" list landed. The helper holds the `PORTAL_PREFS_FILE`
    env setup, the themes-dir seed, `newCountingStores`, the recording
    `fakeThemePersister`, `countingEnumeratorOver`, the construction-time
    `ResolveNomination`, the ten-field `Build(Deps{…})` literal, the size/sessions setup,
    the `t` keypress with its open assertion, and the `stores.reset()` / `opens` capture
    (theme_testing_test.go:84-118). Both tests are reduced to exactly the four things that
    differed: `RawKeys` fixture, keypress, commit assertion, and the `subject` noun.
  - `press` is typed `func(*testing.T, Model) (Model, tea.Cmd)`, which is
    `pressCommitKey`'s signature exactly (theme_panel_commit_test.go:76), so the constant
    path passes the existing helper by name and only the slot path needs a closure to bind
    `slotDarkPress`. Good seam shape — no adapter on the common case.
  - Task step 6 (prefer an existing construction helper) is satisfiable-as-written: the
    pre-existing `newDirBackedPanelModelOver` (theme_testing_test.go:55) constructs via
    `New(...)` with no counted stores and no `ThemePersister`, so it could not cover this
    `Build(Deps{…})` sequence. No second construction path was added to the panel fixtures
    — the literal lives in the one helper.
  - Task step 5 asked the shared doc comment to keep three clauses; the surviving comment
    (theme_testing_test.go:73-74) keeps the "every route to disk runs through a counted
    seam / internal/tui resolves no config path" clause and drops the "prefs call is the
    one write, everything else asserted absent" clause. That trim came from the later
    deliberate comment passes (`e3fa1503` "strip internal/tui to the code-quality
    standard", `915e7fcb` "adversarial audit of the surviving comments"), not from this
    task, and the dropped substance survives verbatim inside the assertion string at
    theme_testing_test.go:124 ("the prefs write is the only one"). Not drift.
  - Likewise the per-test doc comments this task wrote (visible in `9c4b132c`) were
    removed by those same later comment passes. Judged against the plan's amended intent,
    that is superseding work, not regression — though the two call sites now carry no
    comment at all and lean entirely on the `subject` literal to say what makes each case
    distinct, which reads acceptably here because the argument list is four lines long.
  - Fidelity check against the pre-extraction bodies: the only differences are ones later
    tasks introduced repo-wide — `writeThemeFileForTest` → `themetest.Write` +
    `themetest.MonochromeLines` (17-5) and `theme.NewLoader(nil)` → `theme.NewSilentLoader()`
    (14-12). No claim was weakened, no assertion dropped, no threshold moved.
  - `t.Helper()` is present (theme_testing_test.go:82), so a failure reports the caller's
    line and the failing test's own name — which is what keeps "failure messages still name
    which commit failed" true even for the shared `t.Fatalf` fixture guards.

TESTS:
- Status: Adequate
- Coverage: This task is itself a test-side refactor, so the unit of verification is
  whether the two tests still hold what they held. They do:
  - Constant path: `theme.RawKeys{Theme: "sunset"}` → `Enter` → `requireCommitted(p, "sunset")`,
    which also asserts zero slot writes (theme_panel_commit_test.go:93-95).
  - Slot path: `theme.RawKeys{Light: DefaultLightSlug, Dark: "sunset"}` → `d` →
    `requireSlotCommits(p, {slug:"sunset", member:MemberDark})`.
  - All five shared assertions survive with their information-carrying wording: seam-call
    count with the seven-value breakdown (:123-127), enumeration count against the open's
    baseline (:128-130), nil `tea.Cmd` (:131-133), empty config dir (:134-136), single
    themes-dir entry (:137-143).
- Notes:
  - The two negative controls the task listed are transient by nature and leave no
    artifact, but the helper would in fact catch both: a second seam touch trips
    `stores.calls() != 0` and prints `subject`; a scheduled command trips `cmd != nil` and
    prints `subject`. Neither assertion is reachable-but-vacuous — `stores.reset()` runs
    after the open (:117) so the open's own calls are excluded, and `opens` is captured at
    the same point so the enumeration comparison measures only what the commit adds.
  - Not over-tested: the helper adds no assertion the originals lacked, and the assertion
    ordering (commit first, then the counters) is preserved, so a wrong-value commit still
    fails on the specific `requireCommitted`/`requireSlotCommits` message rather than on a
    generic counter.
  - The `PORTAL_PREFS_FILE` redirect is load-bearing rather than decorative: it gives a
    hypothetical rogue in-package path resolution somewhere observable to land, which is
    exactly what the empty-config-dir assertion then reads. Setup and assertion are
    coherent.

CODE QUALITY:
- Project conventions: Followed. Test-only helper in the package's shared panel test file
  as directed; no `t.Parallel()` (repo prohibition); `t.Setenv` over `os.Setenv`; `t.Helper()`
  on the helper and on every fixture helper it calls.
- SOLID principles: Good. The helper takes the four axes of variation as parameters and
  owns nothing else; the two behavioural expectations arrive as function values rather than
  as a mode flag or a bool, so adding a third commit path needs no edit to the helper body.
- Complexity: Low. Straight-line setup → one injected keypress → five flat assertions, no
  branching beyond the guards.
- Modern idioms: Yes. Function-valued seams over enum switches; `themetest` fixtures over
  hand-rolled file writes.
- Readability: Good. `subject` threads the noun through every message so failures read as
  prose about the commit under test.
- Issues: One ineffectual assignment (below), plus two residual duplications that the
  extraction did not reach — both recorded as non-blocking notes.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_testing_test.go:120 — change `m, cmd := press(t, m)` to
  `_, cmd := press(t, m)`. `m` is never read after this line, so the assignment is
  ineffectual; `ineffassign` is in the enabled standard set (`.golangci.yml:8`), and the
  package's own convention for a discarded model is `_, cmd :=` (e.g.
  `keymap_dispatch_guard_test.go:97`, `multi_select_keymap_test.go:308`).
- [quickfix] internal/tui/theme_testing_test.go:134-143 and :189-198 — the config-dir-empty
  plus single-themes-dir-entry pair is now written twice in this one file: once at the tail
  of `requireCommitDoesNoOtherIO` and once inside `requireNoPrefsOrThemesWrite`'s absent
  subtest, differing only in the `subject`/`path.verb` noun and one trailing clause.
  (The second copy arrived later, with 16-2's open/close proof — not this task's doing, but
  it re-opens the same drift the task closed.) Extract
  `requireOnlySeededThemeFile(t *testing.T, configDir, dir, subject string)` holding both
  reads and both messages, and call it from each.
- [quickfix] internal/tui/theme_testing_test.go:99-110 & :123-127 versus
  internal/tui/apply_theme_test.go:218-227 & :239-242 — the seven-field
  `countingStores` → `Deps` wiring and the seven-value failure breakdown are each stated at
  both sites, so adding an eighth counted seam to `countingStores` needs three coordinated
  edits (the struct, both wirings, both messages) and silently under-counts if one is
  missed. Add `func (c countingStores) deps() Deps` returning the seven counted fields for
  the caller to overlay, and `func (c countingStores) breakdown() string` returning the
  parenthesised per-seam tally, then call both from each site.
- [idea] internal/tui/theme_testing_test.go:87 — the helper hardcodes the seeded drop-in as
  `sunset.theme` while accepting `keys` as a parameter, so the two are only correlated by
  convention. A third caller passing keys that name a different slug would resolve through
  the rejection fallback to a built-in and still satisfy all five assertions, i.e. pass
  vacuously. Decide between deriving the seeded filename from `keys` (needs a rule for
  which slot to read when both `Light` and `Dark` are set) and adding a fixture guard after
  the open that the panel's rows carry the seeded slug.
