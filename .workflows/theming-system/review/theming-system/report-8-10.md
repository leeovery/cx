TASK: theming-system-8-10 — Esc Discards The Preview Onto The Resolved Persisted State

ACCEPTANCE CRITERIA (from plan):
- With nothing changed, `Esc` after arrowing three rows away restores the exact pre-open frame (byte-compare the composed view).
- With the active theme's file edited mid-session to new-but-valid values, `Esc` renders the new values.
- With the active theme's file invalidated mid-session, `Esc` renders the §8.5 fallback.
- The close resolves through the retained enumeration (themes dir removed after open → `Esc` still resolves, reads nothing).
- `Esc` discards the enumeration — the next open performs a fresh directory read.
- `Esc` writes nothing: prefs.json byte-identical, absent file stays absent, no tmux option set.
- Ten open/close cycles against a persistently broken active theme emit exactly one `theme: fallback applied` and zero `theme: loaded`.
- `Esc` with a filter applied leaves the filter applied; `Esc` inside multi-select leaves the marked set and banner intact.
- `Esc` does not quit Portal on either page.
- The panel's list, delegate, badges and message are all cleared on close.
- Neither open nor close re-lays-out the page beneath (list width/height byte-identical before/during/after).
- Closing is one frame — no intermediate width, no animation state.

STATUS: complete

SPEC CONTEXT:
Spec §9.2 (line ~995, ~1012): `Esc` "Closes. Discards an uncommitted preview and renders the resolved persisted state"; "Every write is an explicit keypress; nothing writes on close"; `Enter` deliberately does not close. §5.8 (line 418): "`Esc` resolves persisted state against the panel's enumeration, not against what construction loaded… Portal shows what the config now says, not a stale copy it happens to still hold." §9.8 (line 1187): "A forced close takes the `Esc` path exactly." §11.1 (line 1355): the restyle's new callers are the panel's arrow-preview, its open and its close; "The close path matters most — a missed re-point there leaves a preview the user explicitly discarded painting the main screen." §12.3: `theme: fallback applied` is deduped per process on slug+reason precisely because resolution re-runs on every open and every `Esc`; `theme: loaded` is per-load and undeduplicated.

IMPLEMENTATION:
- Status: Implemented (with one deliberate, later-phase-consistent refinement to the plan's step 1 — see Notes)
- Location:
  - `internal/tui/theme_panel.go:241-245` — `closeThemePanel() tea.Cmd`: re-resolves via `applyInForceTheme(m.themePanel.enumeration)`, then `m.themePanel = themePanel{}` (discard last), then returns the single post-close hook `reportOutstandingCommitFailure()`.
  - `internal/tui/theme_panel.go:171-185` — `applyInForceTheme`: `source.Resolve(e, keys)` against the retained enumeration; non-nil error ⇒ degrade (skip select + apply, no write, fall through to the discard); `inForceSlot(resolution, m.themeState.inForceMode())` selects constant-under-constant else the slot matching the gate's already-resolved `canvasMode` (no re-detection); `ApplyTheme` only when the resolved theme differs (no-op when nothing changed).
  - `internal/tui/theme_panel.go:246-255` — `reportOutstandingCommitFailure`: the named single post-close step Phase 9's `⚠ theme not saved` flash and outstanding-failure discharge attach to.
  - `internal/tui/theme_panel.go:322-324` — `Esc` in `updateThemePanel` routes to `closeThemePanel`; `internal/tui/theme_panel.go:261-281` — the forced close (`resizeThemePanel`) calls the same function, no second path.
  - `internal/tui/model.go:1648-1655` — key-exclusivity: while `themePanel.open`, key input is intercepted ahead of the page dispatch (non-key msgs still reach the page), so `Esc` never reaches the filter / multi-select / quit handlers.
  - `internal/theme/dir_theme_source.go:22-29` and `internal/theme/resolution.go:69-114` — the retained-enumeration resolver reads nothing (`ResolveNominationFrom`) and, via the `resolutionPass` pairing (`enumerationPass` → `reportFallback`), emits the fallback WARN only, never `theme: loaded`; the emission policy is pinned in-source at `resolution.go:86-88` and on the seam (`internal/tui/theme_seams.go:5-10`), with `loaded` wired onto the commit entry point (`ResolveSlot`) instead.
- Notes:
  - **Deliberate refinement of the plan's step 1**: the plan had `closeThemePanel` call `theme.ResolveSetting(...)` at the call site and pass a `Setting` to `Resolve`. The shipped seam takes raw keys and performs the tiebreak inside `internal/theme` (`DirThemeSource.Resolve`, `dir_theme_source.go:27`), so the single-tiebreak-site intent is preserved and the `tui` package stays free of theme-vocabulary policy. Same outcome, better boundary — not drift.
  - **Amendment by Phase 9 (expected)**: `closeThemePanel` now returns a `tea.Cmd`, so the "closing is one frame, cmd == nil" criterion holds only when no commit failure is outstanding; with one outstanding the close legitimately schedules the flash auto-clear tick. Both branches are asserted (`theme_panel_close_test.go:89-91`, `theme_panel_close_report_test.go:173-188`).
  - The anti-pattern is stated in-source (`theme_panel.go:238-240`) naming both wrong directions (mid-session breakage; post-commit re-resolution) plus the discard-last ordering, exactly as the plan required, and with no spec/task-id references.
  - Enumeration is genuinely dropped (whole struct zeroed) — no "cache" retained.

TESTS:
- Status: Adequate
- Coverage: `internal/tui/theme_panel_close_test.go` implements every micro-acceptance test the plan named, one-to-one:
  - `TestPanelClose_DiscardsThePreview:65` — byte-compares the composed pre-open and post-close frames, asserts `cmd == nil` (one frame), and counts seam resolutions (1 at open, still 1 after three arrows, 2 after close) so re-resolution is proven rather than a snapshot restore.
  - `TestPanelClose_ResolvesEditedValues:106` / `TestPanelClose_ResolvesToFallback:131` — real loader over a real temp themes dir; both edit the active theme's file mid-session and assert the close lands on the new values / the §8.5 fallback, each with a fixture guard that the arrow actually previewed something.
  - `TestPanelClose_ReadsNothing:157` — removes the themes directory after open; asserts the close still resolves to the retained parse, enumerations stay at 1, and the directory is not recreated.
  - `TestPanelClose_EnumerationDiscarded:187` — asserts every field of the discarded struct (enumeration, union, badges, message, width, list items, list size, and the list keymap standing in for the delegate, which `bubbles/list` does not expose), then mutates the directory and re-opens to prove the re-read.
  - `TestPanelClose_WritesNothing:244` — counting mode/theme persisters plus a positive control (`s` on the closed picker persists once), a `cmd == nil` check for a deferred write, and the shared `requireNoPrefsOrThemesWrite` (present prefs.json byte-identical; absent config dir stays empty; themes dir untouched).
  - `TestPanelClose_EventCadence:283` — ten open/close cycles against a broken active theme: exactly one `fallback applied`, zero `loaded`, with two positive controls (ten `enumerated` records prove the sink sees this loader; a by-name resolution through the *same* loader emits `loaded`, so the zero means something).
  - `TestPanelClose_DoesNotClearTheFilter:319`, `TestPanelClose_NestsOverMultiSelect:351`, `TestPanelClose_EscDoesNotQuit:391` — key-exclusivity, each with a positive control that the same `Esc` on the closed picker does clear the filter / exit multi-select / quit.
  - `TestPanelClose_PageLayoutUnchangedAcrossOpenAndClose:427` — sessions and projects, list size compared before/during/after.
  - `TestPanelClose_ForcedCloseUsesTheSameFunction:463` — an AST guard that `themePanel{}` is assigned in exactly one function, plus a behavioural comparison of a direct `closeThemePanel()` call against the `Esc` path (active theme and composed frame).
  - Phase 9 extends the same path where it should: `TestPanelEnter_EscResolvesTheCommittedTheme` (`theme_panel_commit_test.go:240`) covers the "Esc after a commit resolves the newly persisted state" edge case with a real loader, and `theme_panel_close_report_test.go` covers the flash hook, discharge, forced-close precedence and the caller-set guard (`closeThemePanel` called only from `resizeThemePanel` and `updateThemePanel`).
- Notes: Not over-tested. Assertions are behavioural, each test carries a fixture precondition or positive control that would catch a vacuous pass, and the two single-close-path guards (zero-assignment sites vs. caller set) check different failure modes rather than duplicating. Would the tests fail if the feature broke? Yes — swapping the implementation for an open-time snapshot fails `ResolvesEditedValues`/`ResolvesToFallback` and the resolution counter in `DiscardsThePreview`; retaining the enumeration fails `EnumerationDiscarded`; emitting `loaded` on the panel path fails `EventCadence`.

CODE QUALITY:
- Project conventions: Followed. Seam-based DI (`ThemeSource`), no `t.Parallel()`, unit-lane only (no daemon/binary), no raw hex at call sites, `internal/tui` emits no `theme` log component (the injected `EventLogger` owns emission), comments explain *why* in the codebase's established voice and carry no task ids, phase numbers or spec-section references.
- SOLID principles: Good. One close function with one responsibility, extended by a named post-close hook rather than by forking the path; resolution policy (tiebreak, default substitution, event emission) stays behind the `theme` package boundary; the `resolutionPass` type makes "the retained-enumeration pass must not emit `loaded`" a structural pairing rather than a convention.
- Complexity: Low. `closeThemePanel` is three statements; the branching lives in `applyInForceTheme`/`inForceSlot`, each single-purpose.
- Modern idioms: Yes (`slices.IndexFunc`, `max`, pointer-receiver mutators with value-receiver readers consistent with the package).
- Readability: Good. Ordering constraints (resolve before discard; read `commitFailed` before the close discharges it) are stated at the point where a reader would otherwise reorder them.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_panel.go:238-240 — the plan asked for the "no re-layout on close" negative to be stated at the close itself so a reader cannot "complete" the close with a reclaim step (and thence add the open-time reduction that would justify it). The invariant is currently stated only on the composite (`theme_panel_render.go:114`). Append one sentence to `closeThemePanel`'s doc comment: "Nothing is reclaimed: the page beneath was never reduced — the panel composites over a base laid out at the full content width — so a reclaim step here would only invite the open-time reduction that reflows the surface being previewed."
