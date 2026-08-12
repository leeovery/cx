TASK: theming-system-9-12 — The Confirm, Failed-Commit And Minimum-Height-With-Message Fixtures

ACCEPTANCE CRITERIA:
1. `theme-panel-confirm` renders `clear constant nord?  y / n` verbatim in `text.secondary`, no band, footer exactly `y confirm` / `n cancel`.
2. At the captured minimum width the confirm occupies two rows and the list body has shrunk by two.
3. `theme-panel-commit-failed` renders `⚠ couldn't save theme` in `accent.attention`, no `bg.attention` band, standing four-row footer, `●` in its pre-failure position.
4. `theme-panel-min-height-message` renders exactly one list row and a one-line (truncated, not wrapped) message at the floor, with the standing footer.
5. Each fixture's `--theme` names the theme under its cursor, stated in the doc comment.
6. All three appear in `FixtureByName` and `FixtureNames()`; the registry drift check passes.
7. All three are enumerated by the swap-and-diff guard with no test edit.
8. The seeds declare state only — no fixture supplies message text.
9. No fixture reaches a themes directory, `prefs.json` or an XDG lookup; nil persister.
10. A `.tape` per fixture, each PNG verified as a fresh write.
11. The tapes record their intended column/row counts in comments.

STATUS: complete

SPEC CONTEXT:
Spec §13.3 requires capturable frames for the message slot in both states, plus the panel at its minimum height with a message live ("that arithmetic is only observable on a frame that renders it"). §9.1 pins the slot's two treatments — confirm → `text.secondary`, no band; failed commit → `⚠` and text in `accent.attention`, no `bg.attention` band — and the per-dimension degrade rule: at minimum *width* the slot may wrap to two rows, at minimum *height* it truncates to one line. §9.8/§1185 define the floor as header + footer + one row + one message row. §14A pins the copy verbatim: `clear constant <slug>?  y / n` and `⚠ couldn't save theme` (spec lines 1798–1799 — the implementation matches byte-for-byte). §13.2 makes tapes/PNGs scaffolding, cleared at sign-off, while the Go fixture definitions are permanent.

IMPLEMENTATION:
- Status: Implemented (mechanism partly superseded by later in-plan remediation — see Notes; not drift)
- Location:
  - `/Users/leeovery/Code/portal/internal/capture/fixtures.go:468-495` — the three builders, each derived from its base (`constant-previewing` → confirm; `adaptive-pair` → commit-failed; commit-failed → min-height-message), plus the seed fields at `:44-49` and their wiring into `tui.CaptureSeeds` at `:92-104`.
  - `/Users/leeovery/Code/portal/internal/capture/fixtures.go:133-185` — single `fixtureBuilders()` registry; both `FixtureByName` and `FixtureNames()` derive from it, so AC 6 is structurally unfalsifiable rather than list-maintained.
  - `/Users/leeovery/Code/portal/internal/tui/build.go:69-83,145-147` — `CaptureSeeds.ThemeConfirm` / `ThemeCommitFailed` (both `bool`) applied as Options.
  - `/Users/leeovery/Code/portal/internal/tui/model.go:688-698` — `WithInitialThemeConfirm` / `WithInitialThemeCommitFailed`.
  - `/Users/leeovery/Code/portal/internal/tui/theme_panel.go:139-166` — `armThemePanel` calls `seedThemePanelMessage` last (after the cursor anchor); the confirm routes through the production `raiseSlotConfirm`, the failure through `reportCommitFailure` (`/Users/leeovery/Code/portal/internal/tui/theme_panel_commit.go:65-68`), the one site pairing the line with the outstanding state.
  - Copy single-sourced at `/Users/leeovery/Code/portal/internal/tui/theme_panel_message.go:15-22`; footer substitution at `:74-79`; confirm slug read off `keys.Theme` (the persisted constant, hence `nord`) at `:44-46`.
  - Tapes: `/Users/leeovery/Code/portal/testdata/vhs/theme-panel-confirm.tape` (690x800 → 54 cols x 31 rows, rationale recorded) and `/Users/leeovery/Code/portal/testdata/vhs/theme-panel-min-height-message.tape` (780x366 → 63 cols x 10 rows, floor arithmetic spelled out).
- Notes:
  - AC 1/3/4 verified by reading the render path and the assertions: confirm → `TextSecondary` + no `BgAttention`/`BgSelection`/`BgSubtle` + canvas present; failed commit → single `AccentAttention` foreground run for glyph and text, no band; footer scope substituted only for the confirm.
  - AC 5 verified by reading: confirm's doc comment names `--theme tokyo-night-day` (cursor = `theme.DefaultLightSlug`), commit-failed and min-height name `--theme nord` (cursor = `nord`). All three pairings are coherent with the seeded cursor.
  - AC 9: `fixtures.go` imports no config machinery; `themesDirPath` is a fake string constant; `Deps` wires no `ThemePersister`. The `cmd/capturetool` import guard is untouched.
  - AC 10/11: the commit that implemented this task (`ea4c4142`) added all three `.tape` + `.png` pairs; `71e24eef` (task 10-10) cleared them at sign-off per §13.2, and later tasks re-created the confirm and min-height artefacts. The commit-failed pair is currently absent — that is the retention policy operating, not a gap. Fresh-write verification is a process step that cannot be re-checked from the tree; both surviving tapes carry the hash-check instruction verbatim.
  - Superseded-by-later-tasks (per the plan's remediation phases, judged as intent-as-amended): the two-list registry became one builder list ("Make The Capture Fixture Registry One Authoritative List"); the fixture render size became a declared field ("Let A Capture Fixture Declare Its Render Size"), which is what lets `min-height-message` pin 63x10 rather than depend on a tape; and the original spec-section-laden doc comments were trimmed to the repo's current comment convention.

TESTS:
- Status: Adequate (one vacuous sub-assertion — see below)
- Coverage:
  - `/Users/leeovery/Code/portal/internal/capture/theme_panel_message_fixtures_test.go` — confirm copy/token/no-band/footer substitution (`TestPanelFixture_ConfirmFrame`), the two-row wrap at min width plus the one-row control at preferred width (`TestPanelFixture_ConfirmWrapsAtMinWidth`), the failed-commit line's single `accent.attention` run and band absence with the standing footer (`TestPanelFixture_CommitFailedFrame`), badge-unmoved against the pre-failure sibling frame with a non-vacuous "the sibling frame carries badges at all" control (`TestPanelFixture_CommitFailedBadgeUnmoved`), the floor's one-body-row/one-message-row arithmetic plus a one-row-shorter refusal control (`TestPanelFixture_MinHeightMessageFrame`), truncation-not-wrapping at the floor including the ellipsis check on the copy that wraps above it (`TestPanelFixture_MinHeightMessageTruncates`), seeds-are-`bool`-and-the-package-source-holds-no-copy (`TestPanelFixture_MessageSeedsAreStateOnly`), nil persister for all three (`TestPanelFixture_MessageFramesWireNoThemePersister`).
  - Enumerating coverage the three inherit for free: `TestModelAt_ReachesCapturedState` (its table is asserted to cover every build-backed fixture; rows at `swap_harness_test.go:58-62`), `TestThemeSwapGuard_EnumeratesRegistry`, `TestPanelFixture_Registered`, `TestPanelFixture_RegistryHoldsTheSpecifiedPanelSet`, `TestPanelFixture_FourInputs`, `TestPanelFixture_UnionIsProductionAssembled`, `TestFixtureRenderSize_DeclaredOnlyByTheGeometryFixtures`.
  - Behavioural backstop in the owning package: `/Users/leeovery/Code/portal/internal/tui/theme_panel_message_test.go:133-185` proves the slot costs the list body one row (and a wrapped slot two), and `:191+` proves truncation at the floor — so the capture-side claim rests on real coverage.
- Notes:
  - Not over-tested: each assertion has a distinct subject, and several carry explicit anti-vacuity controls (the sibling-badges check, the "one row shorter refuses" check, the preferred-width one-row control).
  - The plan's names `TestPanelFixture_MessageFramesRegistered` / `_MessageFramesNoConfigAccess` / `_MessageFramesUnderTheGuard` do not exist as written; later remediation tasks deliberately collapsed the per-fixture registration/no-config/guard tests into the enumerating ones listed above, which is structurally stronger (a new fixture cannot be omitted). Equivalent coverage confirmed; not a gap.
  - One sub-assertion is an algebraic identity and can never fail (detail in the notes below). The claim it purports to make is covered in `internal/tui`, so no behaviour is unverified — but the assertion itself proves nothing.
  - Tests were assessed by reading only; nothing was executed.

CODE QUALITY:
- Project conventions: Followed. Fixtures derive from their bases rather than restating data (matching `narrow`/`paginated`); seeds are wired through the existing `CaptureSeeds` Option pattern; the copy stays single-sourced in `internal/tui`; capture-only fields are set nowhere outside `internal/capture` (verified — the only `Capture:` literal in non-test code is `fixtures.go:92`); comments carry no spec-section or task-id references.
- SOLID principles: Good. `reportCommitFailure` extraction gives the line+state pairing one owner, reached by both the real write failure and the seed; the seed declares state and reuses the production writers rather than duplicating them.
- Complexity: Low. `seedThemePanelMessage` is a two-arm switch; the three builders are three lines each.
- Modern idioms: Yes.
- Readability: Good, with two small gaps noted below (an unexplained `theme.MemberLight` constant and a fixture whose doc comment asks for a capture width it does not declare).
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] /Users/leeovery/Code/portal/internal/capture/theme_panel_message_fixtures_test.go:119-125 — the "the wrapped rows are charged to the list body" subtest is an identity: with `footerStart == len(lines)-len(confirmFooterRows)`, `want = emptySlot-messageRows` reduces to `start-firstRow`, which is `got`, so it passes for any frame. Measure the empty-slot body instead — render `theme-panel-constant-previewing` (the confirm's base, message slot empty) at `messagePanelTermWidth`/`harnessHeight`, count its body rows, and assert the confirm frame's body is that count minus `messageRows`.
- [quickfix] /Users/leeovery/Code/portal/internal/capture/theme_panel_message_fixtures_test.go:27 — replace the literal `messagePanelTermWidth = 54` with `tui.ThemePanelMinWidthTerminal()`; 63 terminal columns leaves 59 content columns, still below `2*themePanelPreferredWidth`, so the ladder yields the same 24-column panel and the same wrap, while anchoring the claim to the ladder the sibling geometry fixtures already use instead of a bare number.
- [do-now] /Users/leeovery/Code/portal/testdata/vhs/README.md:93 — the live-view example pairs `--fixture theme-panel-confirm` with `--theme nord`, which breaks the coherence rule the fixture's own doc comment states (its cursor is seeded to `theme.DefaultLightSlug`). Change the line to `go run ./cmd/capturetool --fixture theme-panel-confirm --theme tokyo-night-day`.
- [do-now] /Users/leeovery/Code/portal/internal/tui/theme_panel.go:155-166 — `seedThemePanelMessage` hard-codes `theme.MemberLight` with no stated reason (the original commit explained it; the comment trim dropped it). Add to the existing doc comment: "The confirm's slot is light and immaterial to the frame: the question names the constant being cleared, not the half being written, and nothing is written until `y` — which a capture never presses."
- [idea] /Users/leeovery/Code/portal/internal/capture/fixtures.go:468-475 — `themePanelConfirmFixture` asks in its comment to be captured at the panel's minimum width but declares no width, so `capturetool --fixture theme-panel-confirm` on a wide terminal silently renders the un-wrapped one-row confirm — the wrap being the frame's whole subject. Decide whether the confirm frame IS a geometry (declare `fx.width = tui.ThemePanelMinWidthTerminal()` and add it to `geometryFixtureBases` in fixture_render_size_test.go, which would also subsume the previous note) or stays caller-sized with the requirement carried only by the tape.
