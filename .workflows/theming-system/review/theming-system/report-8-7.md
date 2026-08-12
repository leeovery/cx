TASK: theming-system-8-7 — The Constructor Slot And `t` Opening A Panel That Re-Enumerates On Every Open

ACCEPTANCE CRITERIA:
1. `t` on Sessions and on Projects opens the panel; `t` while `/` is focused inserts a literal `t` on both pages.
2. The themes directory is read on the keypress — construction with a populated `PORTAL_THEMES_DIR` performs zero directory reads until `t`.
3. Each open performs exactly one directory read and emits exactly one `theme: enumerated`; three opens emit three.
4. A file edited between two opens is reflected in the second open's rows without relaunching.
5. The retained enumeration is discarded on close — the next open re-reads.
6. `prefs.json` is not read on open.
7. Badges render on the first open from the injected `[]SlotResolution` with no additional resolution work.
8. A nil `ThemeEnumerator` makes `t` a silent no-op with no panic; a typed-nil concrete value behaves identically.
9. While the panel is open every key except `Ctrl-C` and the provisional `Esc` is swallowed (`k`, `x`, `m`).
10. `cmd` passes a real `theme` component logger; `capturetool`/fixture models reach no config and both import guards stay green.
11. `portal open <target>` performs no theme work.

STATUS: complete

SPEC CONTEXT:
§5.7 (line 404) — no enumeration at construction; enumeration happens only when the slide-over opens. §5.8 (lines 412–414) — the directory is enumerated on every panel open, never once per process (the drop-in loop: copy a built-in, edit it, see it, without relaunching); the parse results are retained for the panel's lifetime and discarded on close. §8.4 (line 826) — the panel uses the construction-time prefs snapshot and does not re-read `prefs.json`, the deliberate asymmetry with the fresh directory read (the themes dir is hand-edited between opens; `prefs.json` is what Portal writes, and re-reading would import another instance's commit — the cross-instance sync §8.9 declines). §9.6 (line 1153) — `t` needs the filter carve-out, exactly as `s` has. §13.3 (line 1488) — `theme: directory unusable` is deduped per process precisely because enumeration runs per open.

Amendment note: the plan's `Deps.ThemeSlots` injection and the seam name `ThemeEnumerator` were both deliberately superseded by later tasks in this plan. Task 8-8's open-time re-resolution became the badge source (`ThemeSource.Resolve` against the retained enumeration — no I/O), which is exactly the "task 8-8 replaces rather than duplicates it" outcome the task's own ambiguity note anticipated; the seam grew to `ThemeSource` (`Open`/`Reassemble`/`Resolve`/`LoadSlot`, narrowed again by 17-4). Criteria are judged against that amended intent.

IMPLEMENTATION:
- Status: Implemented (mechanism amended by later plan tasks as described above)
- Location:
  - `internal/tui/build.go:44-50` (`Deps.ThemeKeys`), `:33` (`Deps.ThemeSource`), `:126-129` (options applied; `WithThemeKeys` unconditionally because its zero value is meaningful, `WithThemeSource` nil-guarded).
  - `internal/tui/model.go:539-549` (`WithThemeKeys` / `WithThemeSource`), `:1651-1655` (panel arm ahead of the page dispatch, key messages only — resizes/refreshes still reach the model beneath), `:2424-2425` (Sessions `t`, below the `SettingFilter` break at `:2359`), `:1754-1757` (Projects `t`, below the `SettingFilter` break at `:1717`).
  - `internal/tui/theme_panel.go:123-134` (`openThemePanel`: nil-seam guard, the per-keypress `source.Open(keys)`, floor re-check with the real `DirUnusable`), `:139-153` (`armThemePanel` retains `enumeration` + `union`, sizes at the preferred width, sets `open`), `:189-197` (badges from the seam's resolution), `:241-245` (`closeThemePanel` zeroes the panel struct — the discard), `:314-339` (`updateThemePanel`: `Ctrl-C` live, everything else consumed).
  - `internal/tui/theme_state.go:37-44` (construction-time `keys` snapshot; nil `source` = silent no-op).
  - `internal/tui/theme_seams.go:11-16` (`ThemeSource`).
  - `cmd/theme_source.go:9-12` (`newThemeSource` closes over the construction-time `theme.Loader` and `themesDirPath()`), `cmd/open.go:485-494, 610-611, 638-640, 533-534` (real `theme` component logger via package-level `themeLogger`; `RawKeys` from the construction-time resolution threaded to `Deps.ThemeKeys`; one loader per launch = one WARN dedup scope).
  - `internal/theme/dir_theme_source.go:14-39` (`Open` is the only method that touches `Dir`; `Resolve`/`LoadSlot` run against the retained enumeration).
- Notes:
  - Criterion 2/3/4/5 hold structurally: `openThemePanel` is the sole `Open` call site, `closeThemePanel` assigns the zero `themePanel`, and `Resolve`/`Reassemble` are documented and implemented as I/O-free.
  - Criterion 6 holds: nothing on the open path reads prefs; `internal/tui` has no config-path knowledge at all, and the model carries the construction snapshot.
  - Criterion 10 holds: `cmd/open.go` builds the loader on `log.For("theme")`, while `cmd/doctor_theme.go:52`, `cmd/theme.go:54` and `cmd/capturetool/main.go:87` use `theme.NewSilentLoader()`; `internal/capture/fixtures.go:111-115` fakes the seam with a value-typed fake and a synthetic `/home/user/.config/portal/themes` constant, so no fixture reaches config.
  - Criterion 8 is met for the nil interface but not for the typed-nil half: `Build` gates on `deps.ThemeSource != nil` and `openThemePanel` on `m.themeState.source == nil`, both of which a typed-nil concrete value passes. No production or fixture site can produce one today (`theme.DirThemeSource` and `capture.fakeThemeSource` are value types), so this is latent, not live — see notes.
  - Entry gating added later (8-13) runs ahead of the nil-seam guard, so under `NO_COLOR` or a sub-floor terminal an unwired seam raises the entry flash rather than being silent. Correct behaviour for the amended design (the refusal is about the terminal, not the seam), but it makes the nil-seam test weaker than it reads — see notes.

TESTS:
- Status: Adequate
- Coverage:
  - `internal/tui/theme_panel_open_test.go:125` `TestThemePanelOpen_BoundOnBothPages` (both pages, one enumeration, both returned values retained, preferred width, page unchanged); `:166` `_FilterCarveOut` (both pages, zero enumerations, the rune lands in the filter query); `:213` `_NoEnumerationAtConstruction` (real dir + real loader wrapped by a counting embed, 0 calls and 0 `theme: enumerated` at construction, 1 of each on the keypress); `:241` `_ReEnumeratesPerOpen` (3 opens → 3 reads → 3 records); `:265` `_SeesAMidSessionEdit` (bad colour → repaired file becomes selectable on the second open); `:286` `_EnumerationDiscardedOnClose` (zero enumeration/union/items after close, then a new file appears on re-open); `:323` `_UsesConstructionTimePrefsSnapshot` (a rewritten prefs file on disk changes neither the keys handed to `Open` nor to `Resolve`); `:363` `_BadgesFromTheSeamsResolution` (the `●` marks the persisted slug, never the fallback row); `:388` `_NilSeamIsASilentNoOp`; `:467` `_SwallowsPageKeys` (each case establishes the panel-closed control first, so a swallow is never vacuous; plus `Ctrl-C` quits and `Esc` closes).
  - `cmd/theme_source_test.go:35` `TestThemeSource_ReadsOnlyWhenOpened`; `:59` `_ReassembleDoesNoIO`; `:80` `_SharesTheConstructionReadsDedupScope` (with an explicit control proving a second loader would double-WARN); `:122` `TestOpenTUI_BuildsOneThemeLoader` + `:136` the AST assertion that the seam is handed a bound loader; `:170` `_WiredThroughBuildTUIModel` (end-to-end `t` through `buildTUIModel`, asserting the `●` that only a reached `Resolve` can produce); `:190` `_ExecPathUntouched` (mode-0000 themes dir made loud first, so the zero-record assertion is provably non-vacuous).
  - `cmd/open_theme_nomination_test.go:22` `TestOpenExecPath_DoesNoThemeWork` (source guard + behavioural exec-path run).
- Notes:
  - Test design is strong on the anti-vacuity front (panel-closed controls, the "a second loader would not" control, the poisoned-dir loudness pre-check) and the counting seam embeds the production adapter rather than restating it, so it cannot drift.
  - `_NilSeamIsASilentNoOp` asserts only `!m.themePanel.open`, which is also true on the geometry/`NO_COLOR` refusal path. It passes today for the right reason (the 80×24 fallback dims clear the floor), but nothing pins that — see notes.
  - No typed-nil case exists for criterion 8's second half.
  - Not over-tested: each test carries one distinct fact, and the cadence tests (`ReEnumeratesPerOpen`, `DiscardedOnClose`) assert different invariants despite touching the same code path.

CODE QUALITY:
- Project conventions: Followed. Seam-shaped DI matching the `TmuxEnumerator`/`ScrollbackReader` idiom; the adapter lives in `cmd` and closes over `themesDirPath()` so `internal/theme` still resolves no paths; the `theme` log component is bound once per package (`cmd/open.go:27`); `internal/tui` emits no theme records; the panel arm sits ahead of page dispatch without swallowing non-key messages.
- SOLID principles: Good. `DirThemeSource` adds no policy over `Assembler`; the panel owns cadence, the seam owns I/O, `internal/theme` owns the tiebreak.
- Complexity: Low. `openThemePanel`/`armThemePanel` are short and linear; ordering constraints are stated where they bite.
- Modern idioms: Yes (`slices.IndexFunc`, `max`, `range N`).
- Readability: Good — comment density is high and mostly load-bearing (why, not what). Two spots read against the code; see notes.
- Issues: none blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_seams.go:7-8 — the doc claims "No method reads the filesystem", which `Open` falsifies (it is the per-keypress directory read this whole task exists to place). Replace that sentence with: "Only Open reads the filesystem; the others work from the enumeration it returned — a further parse of the same slug could disagree with the row on screen."
- [do-now] internal/tui/theme_panel_open_test.go:388-396 — `TestThemePanelOpen_NilSeamIsASilentNoOp` cannot distinguish the nil-seam no-op from an entry refusal (both leave `open` false). Add `if m.flashText != "" { t.Errorf("t raised %q with no seam wired; the nil-seam path must be silent", m.flashText) }` after the existing check.
- [do-now] internal/tui/theme_panel.go:150-151 — the two consecutive `anchorThemePanelCursor` calls read as a duplicate. Add above them: "The capture seed anchors last so a fixture can park the cursor on any row; it is empty in production, where the resolved slug wins."
- [quickfix] internal/tui/build.go:127-129 + internal/tui/theme_panel.go:124 — the plan's typed-nil half of the nil-seam criterion is unimplemented: a typed-nil concrete value boxed into `Deps.ThemeSource` passes both nil checks and would panic on `Open`. Nothing in production or `internal/capture` can produce one (both implementations are value types), so either add the guard used for `prefsStore` at `cmd/open.go:657-663` (wire only a live seam, with the same comment) or add a test pinning that every `ThemeSource` implementation is a value type, so the caller-discipline invariant is enforced rather than assumed.
