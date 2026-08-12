TASK: theming-system-11-15 — Make The Theme Seam Gating And Model-Level Theme State Consistent With Their Siblings

ACCEPTANCE CRITERIA:
- `internal/tui` uses no `reflect` for seam gating; all optional `Deps` seams use the same plain nil gate.
- The model-level theme state is one grouped struct whose type doc states the `canvasMode`/`gate.appearance` divergence and the `startupCanvasHex` retention rule.
- `Deps` exposes the capture seeds under a nested struct, visibly separate from production wiring fields.
- The OSC 11 set-back behaviour (canvas-echo guard, `NO_COLOR` carve-out) is unchanged.

STATUS: complete

SPEC CONTEXT:
This is an architecture-sourced consistency task, not a spec-behaviour task: it changes no rendered surface. The behaviours it must not disturb are the CLAUDE.md-documented `tui` invariants — the owned-canvas restore-on-exit contract (`RestoreTerminalBackground` set-back, the canvas-echo guard anchored to the *retained* startup canvas hex, the `NO_COLOR` write-nothing carve-out, and the deleted `canvasHexFor` helper the source guard keeps gone) and the theme slide-over's nil-seam silent no-op. Both survive intact.

Three later tasks in the same plan deliberately revised this task's output, and all three are supersession rather than drift:
- 13-10 renamed the seam `ThemeEnumerator` → `ThemeSource` (`internal/tui/theme_seams.go`), so the AC's named field reads `ThemeSource` today.
- 15-3 collapsed `bgReplyArrived`/`bgReplyDark` into the single `terminalReply` value and typed the answer as `theme.Member`.
- 16-6 moved `flashOrigin` back onto `Model` (it is a flash-precedence tier, not theme state), so its absence from `themeState` is intended.
The two comment-audit commits (e3fa1503, 915e7fcb) then trimmed this task's comments — see the one note under CODE QUALITY.

IMPLEMENTATION:
- Status: Implemented (as amended by 13-10 / 15-3 / 16-6)
- Location:
  - `internal/tui/build.go:127-129` — `if deps.ThemeSource != nil { opts = append(opts, WithThemeSource(deps.ThemeSource)) }`, the same plain gate as `Enumerator` (105), `Reader` (108), `PreviewAttacher` (111), `ModePersister` (120), `ThemePersister` (123), `ProjectEditor` (99), `AliasEditor` (102).
  - `internal/tui/model.go:547` — `WithThemeSource` is now a plain assignment; no `reflect`.
  - `internal/tui/theme_seams.go` — reduced to the `ThemeSource` interface; the `liveThemeEnumerator` reflect wrapper is gone. A tree-wide grep confirms **no production file under `internal/tui` imports `reflect`** (the ~10 remaining uses are all `_test.go` structural guards, which is legitimate).
  - `internal/tui/theme_panel.go:124` — `if m.themeState.source == nil { return m, nil }` retained as the runtime guard, exactly as the task directed.
  - `cmd/theme_source.go:9-12` — production returns a `theme.DirThemeSource` **struct value**, and `internal/capture/fixtures.go:113-118` returns an untyped `nil` for a panel-less fixture. Neither can produce the typed-nil-in-interface shape the deleted `reflect` check defended against, so the gate change adds no exposure. The task's premise is verified, not assumed.
  - `internal/tui/theme_state.go:33-97` — `themeState` groups `nomination`, `keys`, `source`, `persister`, `gate`, `canvasMode`, `active`, `startupCanvasHex`, `reply`, `commitFailed` plus the three capture seeds, with `inForceMode` / `adoptGateAnswer` / `adoptRetainedReply` as the accessors. Held on `Model` at `model.go:337` beside `themePanel:334`.
  - `internal/tui/build.go:62,65-83` — `Capture CaptureSeeds`, a nested named type at the end of `Deps`; `internal/capture/fixtures.go:92-104` authors through it; `internal/tui/build_test.go:114,133,165` and `theme_panel_cursor_test.go:432` consume it.
  - `internal/tui/restore.go:20-32` — unchanged in behaviour: `colourless` → write nothing; empty original → write nothing; `sameHexColour(original, m.themeState.startupCanvasHex)` → skip; else set back. Only the field path moved into `themeState`.
- Notes:
  - The `startupCanvasHex` contract is preserved *structurally*, not just by comment. `captureStartupCanvasHex` (`model.go:869-875`) is called only from `syncResolvedMode`, and the mid-session constant→adaptive conversion (`theme_panel_confirm.go:73`) calls `adoptRetainedReply()` directly rather than `syncResolvedMode()`, so a commit cannot re-capture the hex. `ApplyTheme` (`model.go:897-900`) and `applyCanvasMode` (`model.go:905-915`) both carry the "must not write startupCanvasHex" warning at the sites where it would be violated.
  - `restore_source_guard_test.go:26-32` was correctly updated to the new field path (`themeState.startupCanvasHex` in `restoreComparisonReads` **and** as `restoreAnchorRead`), so the guard still fails a helper that compares against nothing — the nesting did not silently defeat it. `canvasHexFor` is absent tree-wide (grep confirms; the guard spells the name in halves so the guard file itself does not trip it).
  - `originalBg` correctly stayed on `Model` — it is the terminal's captured colour, not theme state, and was not in the task's field list.

TESTS:
- Status: Adequate
- Coverage:
  - Nil-seam refusal — `internal/tui/theme_panel_open_test.go:388` `TestThemePanelOpen_NilSeamIsASilentNoOp` drives a nil `ThemeSource` *through `Build`* (so it exercises the new plain gate, not just the runtime guard) and asserts `!m.themePanel.open`. This is exactly the test the task required and it covers the changed code path rather than an adjacent one.
  - Restore-on-exit — `restore_test.go` (canvas-echo guard table, anchored-to-startup-hex both directions, non-hex reply still sets back, `NO_COLOR` no-op), `restore_divergence_test.go` (committed-theme divergence incl. an explicit "the startup hex did not move with the commit" subtest, uncommitted-preview divergence, survives-swaps, echo-guard shapes after swap, empty capture after swap), `startup_canvas_hex_test.go` (captured at gate resolution; constant captured at construction). All four scenarios the task named are present.
  - Source guards — `restore_source_guard_test.go` runs four independent assertions (no theme import on the exit path, only the three permitted reads, the anchor read is mandatory so the guard cannot pass vacuously, `canvasHexFor` gone tree-wide) plus the two-launch-site contract.
  - Divergence invariant — `theme_answer_test.go:38-55` `TestInForceMode_ConversionOnAPinnedGateTakesTheRetainedReply` pins both halves in one test: `inForceMode() == MemberLight` while `gate.appearance == MemberDark` ("want the untouched"). The invariant the AC wants documented is behaviourally guarded regardless of where the prose lives. `TestInForceMode_LateReplyIsRecordedButNeverReThemes` (57) covers the no-second-resolution side.
  - Structural — `apply_theme_test.go:246-260` recurses into same-package struct fields, so the `themeState` nesting is walked rather than skipped by the no-`theme.Loader`-on-Model guard.
- Notes:
  - Not under-tested: every acceptance criterion has at least one test that would fail if the criterion regressed.
  - Not over-tested: the only apparent overlap is the "an echo of the startup canvas is still skipped" subtest appearing in both `restore_test.go:51` and `restore_divergence_test.go:76/112` — these are distinct fixtures (pre-swap vs post-commit vs post-preview), which is the point of the divergence file. Justified.
  - No new tests were added for the grouping/nesting itself, which is correct: it is a pure refactor, and the existing suite compiles against the new field paths, which is the real proof.

CODE QUALITY:
- Project conventions: Followed. Constructor-injected seams with plain nil gates match the `Deps` pattern documented in CLAUDE.md; no `t.Parallel()` added; `internal/capture` fixture authoring updated in lockstep so the swap-and-diff completeness guard still drives every fixture.
- SOLID principles: Good. The change is a straight cohesion improvement — `themeState` gives the theme machinery one owner with three named accessors (`inForceMode`, `adoptGateAnswer`, `adoptRetainedReply`) instead of sixteen bare fields written from arbitrary call sites, and `CaptureSeeds` separates the fixture-authoring interface from the production wiring interface on the same type.
- Complexity: Low. No new branching; the reflect removal strictly reduces it.
- Modern idioms: Yes. `theme.Member`'s zero value is `MemberDark` (`internal/theme/member.go:8`), so the `canvasMode` field's "zero value is the standing no-answer fallback" doc is accurate rather than aspirational.
- Readability: Good. Field docs on `themeState` are specific and non-restating.
- Comment accuracy: Checked every comment in `theme_state.go`, `restore.go` and the changed `build.go` block against the code. All hold: `source` "nil makes `t` a silent no-op" matches `theme_panel.go:124`; `persister` "every call site must nil-guard" matches `theme_panel_confirm.go:60`; `keys` "construction-time snapshot, never refreshed" matches the single `WithThemeKeys` injection; `nomination` zero-value sentinel matches `hasNomination`. No process-artifact references, no false claims found.
- Issues: One documentation residue, recorded as a note below rather than an issue — the AC asked for both invariants on the *type* doc, and the type doc is now a single line because two later comment-audit commits trimmed it. The `startupCanvasHex` rule survived onto its field; the `canvasMode`/`gate.appearance` divergence did not survive in either place.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_state.go:51-53 — the `canvasMode` field doc no longer states the deliberate divergence from `gate.appearance`, which AC 2 asked for; the original implementation carried it and the later comment audit removed it, leaving the trap ("fix the drift by routing the conversion through syncResolvedMode") undocumented at the field a reader would consult. Replace the existing two-line comment with: `// Zero value is the standing no-answer fallback, established through` / `// adoptGateAnswer / adoptRetainedReply rather than decided at each reader. After` / `// a mid-session constant → adaptive conversion this deliberately diverges from` / `// gate.appearance, which stays pinned on the constant's fallback: routing that` / `// conversion through syncResolvedMode would re-capture startupCanvasHex` / `// mid-session and strand a colour the user never chose in the terminal on exit.`
- [idea] internal/tui/build.go:130-137 — two gating idioms still coexist in `Build`: eight seams use `if deps.X != nil`, while `Detector`, `Resolve`, `SessionExists`, `AckChannel`, `SpawnExe`, `SpawnGetenv` and `SpawnLogger` are applied unconditionally through nil-tolerant options. They are behaviourally identical (every setter is a plain assignment to a nil-defaulted field), but AC 1's "all optional `Deps` seams use the same plain nil gate" is not literally true, so the ambiguity the task set out to remove is reduced rather than eliminated. Either add the seven guards or drop the eight — the direction is a judgment call about whether seven no-op guards are worth the churn.
- [idea] internal/tui/theme_state.go:71-80 — `themeState` mixes its three capture-only seeds (`initialCursor`, `initialConfirm`, `initialCommitFailed`) with production state, separated only by a bare `// Capture-only seeds:` line, while `Deps` got a nested `CaptureSeeds` type for exactly this separation. Mirroring it (a `capture themeCaptureSeeds` field, updating the three reads in `armThemePanel`/`seedThemePanelMessage` and the three options) would make the model side as visibly split as the wiring side; whether three fields justify the extra type is the open question.
