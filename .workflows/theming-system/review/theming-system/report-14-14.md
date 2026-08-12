TASK: theming-system-14-14 — Correct CLAUDE.md's Keymap Claim And Add The Theme Slide-Over To The Render Inventory (tick-800ed4)

ACCEPTANCE CRITERIA:
- CLAUDE.md names all three `pinArrowOnlyNav` call sites and the panel-specific reason.
- The render-structure inventory includes the theme slide-over.
- Every claim in the amended bullet is checkable against `internal/tui` as it stands.
- No other section of CLAUDE.md is modified.

STATUS: complete

SPEC CONTEXT:
Spec §9 ("The slide-over panel", specification.md:935-952, 1172) defines the surface the doc entry now describes: a full-height, right-edge, non-blanking overlay with a **left border only** (§9 l.939, l.947 — "deliberately *not* an inset bordered panel like the modals"), a fixed ~24–30 column budget (l.1172, l.1098), and a **vertical** keymap footer rather than Portal's horizontal footer row (l.952). Spec l.15 additionally states the selector is "opened with `t`". The added CLAUDE.md entry is faithful to all of these.

IMPLEMENTATION:
- Status: Implemented (documentation-only change, commit 6d2322c8; CLAUDE.md is the only source file touched — the other two files in the commit are workflow metadata: `.tick/tasks.jsonl`, `.workflows/theming-system/manifest.json`).
- Location: CLAUDE.md:176 (render-structure bullet), CLAUDE.md:177 (keymap-revision bullet), CLAUDE.md:86 (Config path resolution — see the deviation note below).

Criterion 1 — MET. CLAUDE.md:177 now reads "on **all three** lists — Sessions, Projects and the theme slide-over, where the pin is load-bearing for a further reason: the v2 default binds `l` and `d` to NextPage, and both collide with the panel's own commit keys." Verified against code: the three call sites are `internal/tui/model.go:781` (Sessions list), `internal/tui/model.go:808` (Projects list) and `internal/tui/theme_panel.go:90` (panel list) — `grep -rn "pinArrowOnlyNav" --include='*.go' .` returns exactly those three plus the declaration at model.go:789. The panel-specific reason is verified at both ends: `charm.land/bubbles/v2@v2.1.0/list/keys.go:50` binds NextPage to `"right", "l", "pgdown", "f", "d"`, and the panel's commit keys are `d` = set as dark and `l` = set as light (`internal/tui/keymap.go:67-68`, dispatched via `handleSlotCommitKey`, `internal/tui/theme_panel_commit.go:73). The doc's wording matches the in-source comment at theme_panel.go:82-83.

Criterion 2 — MET. CLAUDE.md:176 adds the slide-over entry in the same register as the modal entries.

Criterion 3 — MET. Every claim in the amended bullet checks out against `internal/tui` as it stands:
- "full height at its right edge": `renderThemePanel(m.themePanel, contentH, …)` + `overlayThemePanel(content, panel, contentW)` at model.go:2835-2836; `themePanelBlock` pads to `height` (theme_panel_render.go:77-79) and the foreground layer is placed at `X = contentW - width(panel), Y = 0` (theme_panel_render.go:118).
- "24 or 30 columns wide (`themePanelWidthFor`'s two-stage ladder)": `themePanelPreferredWidth = 30` / `themePanelMinWidth = 24` (theme_panel_geometry.go:11-12), two-stage ladder at theme_panel_geometry.go:97-102.
- "left border only (no top, bottom or right edge)": theme_panel_render.go:56-81 — the `prefix` is `panelFrameSide` + gutter on every bordered row and nothing else; the in-source comment says the same.
- "composited as an **opaque** layer … the page beneath is deliberately not re-laid-out": `overlayThemePanel`'s Z-layer compositor (theme_panel_render.go:116-120, comment "Composite, never re-lay-out: the base stays at the unreduced width"), with opacity supplied by `themePanelPainter`'s canvas backfill (theme_panel_render.go:95-112).
- File roles all verified: `theme_panel.go` (themePanel struct l.55-80 = state; `themePanelEntry` l.96 = entry gate; `newThemePanelList` l.84 = list construction; `updateThemePanel` l.314 = key dispatch); `theme_panel_geometry.go` (`themePanelWidthFor`, `themePanelFloor`, `themePanelChromeRows`/`themePanelListSize`); `theme_panel_render.go` (top-to-bottom block assembly + `overlayThemePanel`); `theme_panel_commit.go` / `_confirm.go` (`raiseSlotConfirm`/`confirmSlotAssignment`) / `_message.go` (single `themePanelMessage` slot) / `_footer.go` (`themePanelFooterRows` — one row per entry, i.e. vertical); `theme_row.go` (`themeRowDelegate` + `compose`'s `labelBudget` truncation); `theme_seams.go` (`ThemeSource` interface, l.11); `theme_state.go` (`themeState`, model-level and outliving a panel open). Every named file exists in `internal/tui`.
- The bullet's other in-scope corrections are also right: `right_anchored_row.go` no longer exists and the geometry now lives at `footer.go:158` (`assembleRightAnchoredRow`); the descriptor sentence's new "theme panel's vertical footer (`themePanelKeymap`)" clause matches `keymap.go:64` feeding `renderThemePanelFooter` via `themePanelFooterScope`.
- Pre-existing claims re-checked while in the bullet: `accent.primary` is still a live token name (`internal/theme/theme.go:67`) and is what `header.go:97` styles the `▌` caret with; `placeModalOnClearedCanvas` (modal.go:21) and `renderJoinedPanel` (panel.go:28) exist; every other named chrome/edge-state/loading file exists.
- Notes: I checked one thing the bullet does *not* claim, to avoid a false finding — the theme flash reuses the existing transient-flash slot (`setThemeFlash` → `setFlash`, model.go:1342-1345, adding only an origin marker), so the notice-band clause is not made stale by this feature.

Criterion 4 — NOT MET (deviation, benign). The commit also amended a third hunk in a different section: CLAUDE.md:86 under "### Config path resolution (cmd/config.go)", changing "…`WithThemePersister` options in `cmd/open.go`" to "…options, which `internal/tui/build.go` applies from the `tui.Deps` fields `cmd/open.go` populates." The task's Do item 4 and its fourth acceptance criterion both said to change no other section. Mitigating, and the reason I am not raising it as blocking or proposing a revert: (a) the edit is factually correct — `internal/tui/build.go:117-124` is where `WithInitialMode` / `WithModePersister` / `WithThemePersister` are applied, and `cmd/open.go` contains none of those calls, it only populates `tui.Deps`, so the prior text was the stale claim; (b) `git log -S` confirms no other task in the plan introduced or owned that sentence, so this is not a double-edit or a pre-empted sibling task; (c) reverting it would restore a false claim to the doc. The finding is scope discipline, not correctness — worth recording because the constraint was explicit, but there is no defect to fix.

TESTS:
- Status: Adequate (documentation-only change; the task's own micro-acceptance is the two manual greps plus an unchanged suite).
- Coverage: Both manual verifications pass — `grep -rn "pinArrowOnlyNav" --include='*.go' .` returns exactly the three call sites the doc now names, and every file named in the added slide-over entry exists in `internal/tui`.
- Notes: No Go source references CLAUDE.md (`grep -rln "CLAUDE.md" --include='*.go' .` is empty), so the change cannot affect `go test ./...`; the claim of an unchanged suite is structurally sound without execution. Per my instructions I did not run the suite. No new test is expected or warranted for a doc edit — adding one would be over-testing.

CODE QUALITY:
- Project conventions: Followed. The added entry matches the surrounding bullet's density and register (dense prose, backticked file/identifier names, the "why" carried inline), and states facts checkable against the code with no workflow vocabulary, no task/phase ids and no spec-section references — the one exception, the "(§12.2)" tag on the keymap bullet, is pre-existing text this task did not introduce.
- SOLID principles: N/A (no code changed).
- Complexity: N/A.
- Modern idioms: N/A.
- Readability: Good. The entry leads with the discriminator that matters to a reader ("the one panel that is *not* a modal"), gives the geometry and the composite contract before the file list, and the keymap clause names the collision rather than just asserting the pin is load-bearing.
- Issues: None in the amended text.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] CLAUDE.md:177 — The keymap bullet enumerates the page keys but never states which key opens the slide-over it now documents, so an agent learns the panel exists without learning how to reach it. `t` is bound on both Sessions and Projects (`internal/tui/keymap.go:40` and `:53`, `{Key: "t", Action: "theme", HelpAction: "Theme picker"}`) and the spec calls it out at specification.md:15. Insert after "`s` is Sessions-only (cycle grouping);": "`t` opens the theme slide-over (Sessions and Projects);".
- [do-now] CLAUDE.md:176 — The descriptor sentence this task edited names only `keymap_dispatch_guard_test.go`, but the panel's and the confirm's descriptor↔dispatch parity live in the peer file `internal/tui/keymap_dispatch_guard_theme_test.go` (`TestThemePanelDescriptorDispatchParity:152`, `TestThemeConfirmDescriptorDispatchParity:156`), which the sentence's newly added "theme panel's vertical footer" clause now points a reader towards. Replace "`keymap_dispatch_guard_test.go` guards descriptor↔dispatch drift (dispatch is separate)" with "`keymap_dispatch_guard_test.go` — and `keymap_dispatch_guard_theme_test.go` for the panel and its confirm — guards descriptor↔dispatch drift (dispatch is separate)".
