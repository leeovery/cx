TASK: theming-system-9-6 — The Newly-Live Opposite Slot Loads At Commit With `theme: loaded`

ACCEPTANCE CRITERIA (from the plan task):
- A confirmed `l` on a constant loads the dark slot (and `d` loads the light slot) — the opposite one, never the assigned one.
- An untouched opposite slot resolves the shipped default from the embedded set with zero directory access.
- A stale hand-edited opposite slot resolves from the panel's retained enumeration; a slug absent from the enumeration falls through to the embedded set; a slug in neither takes §8.5's per-slot fallback.
- No commit-time directory read on any branch (asserted with the directory removed after the panel opened).
- Exactly one `theme: loaded` per converting commit, carrying `slug` and `slot`; two conversions emit two lines (undeduplicated).
- When the newly-live slot is unloadable, `theme: loaded` carries the fallback's slug and `theme: fallback applied` the failed nomination's.
- An already-adaptive `d`/`l` and any `Enter` emit no `theme: loaded` and resolve no new member.
- A failed write emits no `theme: loaded`.
- The nomination holds both members after a conversion; the active theme and composed frame are unchanged.
- After a conversion on a light terminal the answer is light and the next close selects the light slot; dark on a dark terminal.
- No new OSC 11 query, no new gate.
- `startupCanvasHex` byte-identical across a conversion (light, dark, no-reply).
- The classification does not route through `syncResolvedMode` (asserted structurally).
- A conversion before any reply resolves dark; a later reply still does not re-theme.
- An adaptive-launch user's answer is not re-derived.
- `ResolveSlot` and the badge path's `Resolve` agree for the same input (shared rule body).
- A `log.Discard`-backed loader emits nothing on this path.

STATUS: complete

SPEC CONTEXT:
§8.4 ("Mid-session slot assignment loads the *other* slot at commit time") specifies that a constant → adaptive conversion makes a second slot live that construction never loaded; the assigned slot needs no read (§5.8's enumeration holds its parse), an untouched opposite slot resolves the shipped default from the embedded set, a stale hand-edited one resolves from the retained enumeration, and there is **no commit-time directory read** (a third parse could disagree with the row on screen). §8.5 gives the per-slot mode-matched fallback. §9.3 states the answer half of the transition dissolves because `restore.go` queries OSC 11 regardless — no new query, no race, no gate, dark when no reply landed — while the file half does not dissolve. §12.3 pins `theme: loaded` as INFO, undeduplicated, with `slug`+`slot`, "also fires at commit time" and firing for the fallback's slug when a nomination is unloadable.

IMPLEMENTATION:
- Status: Implemented (seam shape deliberately amended by later remediation tasks — see Notes).
- Location:
  - `internal/tui/theme_panel_confirm.go:55-77` — `confirmSlotAssignment` clears the confirm, commits, returns on write error or nil persister, then `loadNewlyLiveSlot(pending.member)`; that function records the answer via `themeState.adoptRetainedReply()` and loads `assigned.Opposite().Slot()`.
  - `internal/tui/theme_state.go:11-31, 85-97` — `terminalReply` (arrival + classified member), `terminalReplyFrom`, `answer()` (dark when unarrived), `inForceMode()`, `adoptGateAnswer` / `adoptRetainedReply`. The conversion writes `canvasMode` directly, never through `syncResolvedMode` (`internal/tui/model.go:858-875`), so `startupCanvasHex` is untouched.
  - `internal/tui/theme_seams.go:11-16` — `ThemeSource.LoadSlot(e, slot, keys) error`.
  - `internal/theme/dir_theme_source.go:31-39` — production adapter: collapses the keys through `SlugForSlot` and calls `Loader.ResolveSlot`, reading no directory.
  - `internal/theme/resolve.go:78-114, 141-206` — `Loader.ResolveSlot` runs `commitPass` (enumeration load + `reportSlot`) over the *same* `resolveSlot` body `ResolveNominationFrom` uses with `enumerationPass` (`reportFallback` only). The `resolutionPass` type is what makes the "which pass may emit `loaded`" pairing structural.
  - `internal/theme/events.go:55-61` — `Loaded` is INFO and explicitly not deduplicated; `themeAttrs` carries `slug` + `slot` (omitted for `SlotConstant`).
  - `internal/theme/setting.go:97-104` — `SlugForSlot` single-sources the tiebreak + shipped-default substitution, so an untouched slot nominates the shipped default rather than "".
  - Fakes wired: `internal/tui/theme_source_fake_test.go:58`, `internal/tui/theme_seams_test.go:32`, `internal/capture/theme_fake.go:51`.
- Notes:
  - **Deliberate amendments, not drift.** The plan named the seam method `ResolveSlot(e, slot, slug) (theme.SlotResolution, error)` on `ThemeEnumerator` and had the loaded palette join the nomination directly. The shipped shape is `LoadSlot(e, slot, keys) error` on `ThemeSource` (plan tasks "Make The Theme Slot Seam State What It Does And Single-Source The Slot Collapse" and "Set The In-Force Light/Dark Answer Independently Of The Newly-Live Slot's Load"). `Loader.ResolveSlot` still exists with the planned signature underneath, and the nomination pair is now single-owned by `applyCommittedSetting` (`theme_panel_commit.go:120-127`), which re-resolves both slots from the mirrored keys against the same retained enumeration. Outcome is unchanged: the pair is complete after a conversion, the active member is not moved, `ApplyTheme` is not called.
  - Consequence of that amendment worth recording: `LoadSlot` is now effectively an emission-only call (its `SlotResolution` is discarded), so the opposite slot is resolved twice per conversion — once by `Resolve` for the nomination/badges, once by `LoadSlot` for the log line. Both run the identical rule body over the identical enumeration and keys, so they cannot disagree, and the cost is two in-memory lookups. `fallback applied` cannot double up because it is deduped per slug+reason.
  - Ordering is correct on the failure paths: a failed write returns before the load *and* before the classification (`theme_panel_confirm.go:57-59`); a nil persister returns at :60; a broken-builtin fallback returns from `resolveSlot` *before* `pass.report`, so no `loaded` line is emitted on the fatal, and the panel degrades rather than quitting.
  - The conversion path is only reachable from a constant (`handleSlotCommitKey` raises the confirm only when `themeSetting().IsConstant`), so "emit nothing where nothing converts" holds by construction rather than by a runtime check.
  - `Open` is the only filesystem-touching seam method and it runs at panel open; `Resolve`/`LoadSlot` both take the retained `Enumeration`, so the no-commit-time-read rule holds structurally.

TESTS:
- Status: Adequate.
- Coverage: `internal/tui/theme_panel_commit_load_test.go` carries all 16 tests the plan named, under the plan's names, plus four the remediation tasks added (`TestCommit_NominationTracksThePersistedSetting`, `TestCommitSlotLoad_RestoreStaysAnchoredAfterACommit`, `TestCommitSlotLoad_AnswerIsIndependentOfTheLoad`, `TestCommitSlotLoad_BrokenBuiltinDegrades`). Each acceptance criterion maps to an assertion:
  - opposite-slot direction — `LoadsTheOppositeSlot` (both keys, table-driven, asserts the log line *and* which nomination member changed).
  - untouched / stale / not-in-enumeration / unresolvable — `UntouchedSlotIsTheShippedDefault`, `StaleSlotFromEnumeration` (file rewritten *after* the open; asserts the retained parse's canvas, which is the only assertion that can distinguish a re-read), `UnresolvableTakesTheModeMatchedFallback` (both slots).
  - no directory read — `NoDirectoryRead` removes the themes dir and re-stats it to prove the assertion is not vacuous, driving the real `DirThemeSource` (the counting fake embeds the production adapter rather than re-implementing it, `theme_panel_open_test.go:31-43`).
  - emission cadence — `EmitsLoadedOncePerConversion` (two conversions, two lines, attrs checked on both), `LoadedNamesTheFallbackSlug` (asserts the two lines name *different* slugs, with an explicit failure message explaining why), `NonConvertingCommitIsSilent` (adaptive `d`, adaptive `l`, `Enter` over a pair, `Enter` over a constant — each with a "the keypress wrote exactly one commit" guard so the silence is not vacuous), `FailedCommitLoadsNothing` (with a landed-write positive control), `DiscardSilencesLoaded` (with a capture-backed control run first).
  - nomination/active split — `ActiveThemeUnchanged` asserts the composed frame still paints the previewed canvas *and* does not paint the newly-loaded slot's.
  - answer half — `ConversionUsesTheRetainedAnswer` (light/dark, each verifying the subsequent close lands on the right member, plus a reply-arrives-after-open subtest), `ConversionIssuesNoQuery` (nil cmd, gate struct untouched, plus a guard that the gate's own appearance differs from the answer so the fixture can discriminate), `ConversionWithNoReplyIsDark` (with a late reply that must not re-theme).
  - anchor — `ConversionDoesNotMoveStartupCanvasHex` (light/dark/no-reply, with two anti-vacuity guards proving the anchor differs from both the previewed and the newly-loaded canvas), plus the structural subtest pinning `syncResolvedMode`'s caller set to exactly `{New, Update, armAppearanceDetection}` over the non-test package sources, with a "the scan found a known caller" guard so absence proves something.
  - shared rule body — `SharesTheResolverBody` compares `ResolveSlot` against the badge path's `Resolve` slot record for five slugs including `nope` and `../escape`.
  - Layer-below coverage: `internal/theme/dir_theme_source_test.go` pins that `LoadSlot` emits exactly one `loaded` with the collapsed slug and that `Resolve` emits none; `internal/tui/theme_slot_collapse_test.go` pins production adapter and fake on the same collapsed slug.
- Notes: No under-testing found. Mild overlap between `theme_answer_test.go:38 TestInForceMode_ConversionOnAPinnedGateTakesTheRetainedReply` and `ConversionUsesTheRetainedAnswer` + `ConversionIssuesNoQuery`; it retains one unique assertion (the launch gate is `pinned`, i.e. never armed), so it is not redundant enough to flag. No excessive mocking — the conversion suite drives the real loader and real adapter over a temp themes dir, and the recording fake is used only for the two degrade paths a real loader cannot produce.

CODE QUALITY:
- Project conventions: Followed. No `*slog.Logger` constructed outside `internal/log` (the `theme` component is emitted through the injected `EventLogger`, bound once in `cmd`); no raw hex at call sites; no `t.Parallel()`; source guards use `sourceguardtest`; the new attrs (`slug`, `slot`) are inside the spec-governed `theme` vocabulary already recorded in CLAUDE.md.
- SOLID principles: Good. `resolutionPass` is a neat expression of the interface-segregation concern the task raised — it makes "which resolution route may emit `loaded`" a type-level pairing rather than a call-site convention, so a future caller cannot accidentally inherit or lose the emission. `SlugForSlot` gives the collapse a single owner. The seam narrowed to `error` keeps `applyCommittedSetting` the single writer of the nomination.
- DRY: Good. `ResolveSlot` and `ResolveNominationFrom` share `resolveSlot`, and the parity is enforced by a test rather than asserted in a comment.
- Complexity: Low. `loadNewlyLiveSlot` is four lines; `confirmSlotAssignment` is a straight guard chain.
- Modern idioms: Yes (`cmp.Or` for the default substitution, table-driven subtests, `slices.Sort/Compact` in the structural guard).
- Readability: Good. Comments explain the load-bearing *why* (the classification assigned first and unconditionally; the nomination's single owner; never `ApplyTheme`).
- Comment accuracy: One overclaim — see the first non-blocking note. One documentation gap the plan explicitly asked for — see the second.
- Security: N/A beyond the existing charset check, which still runs before any path is composed (`resolveNamed` → `ValidSlug`); the `../escape` case is covered in the parity test.
- Performance: The double resolution per conversion noted above is two map/slice lookups on a keypress path already doing more work; not a concern.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_seams.go:8 — the doc comment's "No method reads the filesystem" is falsified by `Open`, which re-reads the themes directory on every panel open (`theme_panel.go:121-128` states that re-read as deliberate). Replace that sentence with: "Only Open reads the filesystem, once per panel open; every other method answers from the enumeration it returned, since a further parse of the same slug could disagree with the row on screen."
- [do-now] internal/tui/theme_panel_confirm.go:72 — the plan required the code to state in-source that the *assigned* slot is deliberately not re-loaded ("the half a reader is most likely to add for symmetry"); the current comment covers the classification and the nomination ownership but not this. Add as the opening sentence of `loadNewlyLiveSlot`'s comment: "Only the opposite slot loads: the assigned slot's parse is already retained in the panel's enumeration, so re-loading it for symmetry would buy nothing and re-announce a load that already happened."
