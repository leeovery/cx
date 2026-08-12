TASK: theming-system-12-12 — Make The Nomination Contract Structural At Both Ends (tick-499583)

ACCEPTANCE CRITERIA:
- No call to the adaptive-pair constructor can produce a nomination with a zero `Theme` in either member.
- `themeState.nomination` follows one stated contract on both commit paths, and the contract is written on the field.
- The appearance gate's single-resolution behaviour, the `theme: loaded` event and the `canvasMode` update are unchanged.
- No rendered output changes.

STATUS: complete

SPEC CONTEXT:
Spec §8.4 / §8.8 (specification.md:811-824, 891): the TUI constructor takes the loaded *nomination* (one palette under a constant, both under an adaptive pair), the nomination carries no provisional active member, and the light/dark gate resolves EXACTLY ONCE before the first paint — a late flip would swap to a different theme entirely, and §11.4's retained startup canvas hex depends on that single resolution. The panel additionally needs the raw persisted keys, because a slug that never loaded is not in the nomination and a badge marks the *persisted* slug rather than the nomination's fallback. §1485 pins `theme: loaded` firing once per nominated theme at construction plus once at commit time for the newly-live opposite slot on a constant → adaptive conversion.

IMPLEMENTATION:
- Status: Implemented; the theme-package half was deliberately superseded by a later plan task.
- Location:
  - `internal/theme/nomination.go:32-36` — `AdaptivePair(light, dark Theme)`.
  - `internal/theme/resolution.go:141-165` — the single production caller (`resolveNomination`), both halves resolved (error-checked) before construction.
  - `internal/tui/theme_state.go:34-37` — the `nomination` field and its stated contract.
  - `internal/tui/theme_panel_commit.go:31-43, 97-127` — `commit` → `recomputeThemePanel` → `applyCommittedSetting`, the single site that re-derives badges AND nomination from the mutated keys.
  - `internal/tui/theme_panel_confirm.go:66-77` — `loadNewlyLiveSlot`, now load-for-the-event + `adoptRetainedReply()` only.
  - `internal/tui/model.go:830-865` — gate construction and `syncResolvedMode` (the only nomination readers).
- Notes:
  - Half (1) at task time (commit c502058b) reshaped `AdaptivePair(named MemberPalette, opposite Theme)` over an internal `pairFor(light, dark Theme)`, deleting the `hold` fold that could leave a member zero. Later plan task 17-10 (`1ae04028`, "Collapse AdaptivePair's Tagged-Palette Machinery To A Named Constructor") removed `MemberPalette` entirely and exported `pairFor` as `AdaptivePair(light, dark Theme)`. **This is sanctioned supersession, not drift** (17-10 is a listed task in this same plan, plan-overview.md:189). The task's actual defect — a same-member pair filling one member and leaving the other the zero `Theme` — is now unexpressible: the member concept is gone from the constructor and both members are always assigned from the two arguments, so no call can leave a member unfilled. Reading criterion 1 hyper-literally, a caller can still hand the constructor a zero `Theme` explicitly (`internal/theme/nomination_test.go:144` does exactly that, deliberately, to prove the zero-Nomination sentinel stays distinguishable); that is garbage-in from a caller, not a constructor that silently drops a half. The single production caller cannot reach it — `resolveSlot` returns either a loaded palette or an error, and both errors are propagated before `AdaptivePair` is called.
  - The risk the collapse trades in — a transposed positional pair — is covered at both ends: `TestAdaptivePair_ArgumentOrderIsLightThenDark` (nomination_test.go:49) pins the order as observable, and `cmd/open_theme_construction_test.go:239-240` pins end-to-end that the `theme_light` key lands in the light member.
  - Half (2): the chosen contract is "always current". Every commit routes through `commit()` (theme_panel_commit.go:31), which mirrors the keys and calls `recomputeThemePanel` → `applyCommittedSetting`, so the constant path (`commitConstant`) and the slot path (`commitSlot`, incl. the constant → adaptive confirm) both re-resolve the nomination from one site. `loadNewlyLiveSlot` no longer writes the field — its palette is deliberately discarded and the seam (`ThemeSource.LoadSlot`, theme_seams.go:8-10) exists for the commit-time `theme: loaded` emission. One writer, no second source of truth.
  - The contract is stated on the field (theme_state.go:35-36, compressed by the later comment audit from 13-9's longer form: "Describes what is persisted, not what is rendered — the palette in force is active").
  - Criterion 3 holds: `syncResolvedMode` is still reached only from `New`, `Update` (gate resolve / timeout) and `armAppearanceDetection` — and that is enforced by a source-scanning guard (`theme_panel_commit_load_test.go:760-768`), so a conversion cannot re-enter the gate path. `canvasMode` is still updated on the conversion, now via `adoptRetainedReply()` (moved ahead of the load, unconditionally, by later task 14-3 — again sanctioned supersession). `theme: loaded` cadence unchanged.
  - Criterion 4 holds: the commit touched no `internal/capture` fixture, no `testdata/vhs` reference and no renderer; `applyCommittedSetting` explicitly never calls `ApplyTheme`, and `TestCommitSlotLoad_ActiveThemeUnchanged` asserts the composed frame still paints the previewed canvas and NOT the newly-loaded slot's.

TESTS:
- Status: Adequate.
- Coverage:
  - Constructor: `TestAdaptivePair_HoldsBothWithNoActiveMember` (per-member identity), `TestAdaptivePair_ArgumentOrderIsLightThenDark` (order observable + both directions), `TestAdaptivePair_FillsBothMembers` (no member is the zero `Theme`), `TestNomination_ZeroValueIsNeitherState` / `..._IsDistinguishableFromBothStates` (the sentinel survives the collapse). The task's "same-member pair cannot be expressed" test is satisfied structurally rather than by a test, which is the stronger form the task asked for first.
  - Contract: `TestCommit_NominationTracksThePersistedSetting` (theme_panel_commit_load_test.go:386) drives BOTH required paths — `Enter` over a constant asserts the nomination is the newly-committed constant, and a slot commit over a pair asserts both members track the write. `TestCommitSlotLoad_NonConvertingCommitIsSilent` extends that to `d`/`l`/`Enter` over a pair with a positive control that exactly one write happened (so the silence is not vacuous).
  - Negative paths: `TestCommitSlotLoad_FailedCommitLoadsNothing` (failed write leaves nomination, keys, answer and log untouched, with a landed-write positive control), `TestCommitSlotLoad_BrokenBuiltinDegrades` (resolve fatal moves nothing and does not quit).
  - Invariants: `TestCommitSlotLoad_ConversionDoesNotMoveStartupCanvasHex` (incl. the `syncResolvedMode` caller-set source guard), `TestCommitSlotLoad_ConversionWithNoReplyIsDark` (a late reply moves neither the answer nor the nomination — the single-resolution rule), `TestCommitSlotLoad_ActiveThemeUnchanged` (render unchanged, asserted on the composed frame), `TestCommitSlotLoad_EmitsLoadedOncePerConversion` / `TestCommitSlotLoad_LoadedNamesTheFallbackSlug` (event cadence and attrs).
  - Fixtures are self-guarding: `newConversionPanelModel` / `newAdaptivePanelModel` fatal if the seeded keys resolve to the wrong nomination shape, so a fixture drift cannot quietly make the assertions vacuous.
- Notes: Not over-tested for the risk surface — each test in the cluster pins a distinct invariant (badge/nomination agreement, event cadence, canvas anchor, degrade policy) and the shared `require*` helpers keep the assertions from duplicating. `TestAdaptivePair_FillsBothMembers` is now weakly redundant with `TestAdaptivePair_HoldsBothWithNoActiveMember` (which asserts the stronger per-member identity), but it is the explicit acceptance pin for the "no zero member" property, so retaining it is the right call. No test executed (read-only review, per instructions).

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays log-free at the constructor (the `theme` component is emitted through the injected `EventLogger`); `internal/tui` never emits the component itself (theme_seams.go:5-7); no raw hex reaches a call site; no `t.Parallel()`; the exported-API guard (`internal/theme/theme_test.go:136+`) lists `AdaptivePair` and no longer lists `MemberPalette`, so the collapse is pinned rather than incidental.
- SOLID principles: Good. Single writer for the nomination (`applyCommittedSetting`), single reader path (the gate + `syncResolvedMode`); `loadNewlyLiveSlot`'s narrowed responsibility (announce the newly-live slot) is matched by the seam returning `error` only, so it cannot regrow a second palette writer.
- Complexity: Low. `AdaptivePair` is a one-line struct literal; `applyCommittedSetting` is a five-line function with one early return.
- Modern idioms: Yes. Positional value constructor, comparable value type (`Nomination` is compared with `!=` in tests and in `hasNomination`), explicit `_ =` on the discarded seam error with the reason stated directly above it.
- Readability: Good. Comments state contracts and consequences rather than restating code; no process-artifact references (no task ids/phases) survive in the changed code.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_state.go:35-36 — the field comment claims the nomination "Describes what is persisted", which the resolve-fatal degrade momentarily falsifies: `commit` mirrors the keys and then `applyCommittedSetting` returns early on a `Resolve` error, so a landed write can leave the field holding the previous setting. Name the maintainer and the exception in the same line, e.g. replace "Describes what is persisted, not what is rendered — the palette in force is active." with "Re-derived by applyCommittedSetting on every landed commit, so it describes what is persisted, not what is rendered — the palette in force is active; a resolve fatal leaves it standing."
