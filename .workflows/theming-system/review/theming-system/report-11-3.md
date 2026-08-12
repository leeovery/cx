TASK: theming-system-11-3 (tick-56c2d3) — Strip Spec-Section, Phase/Task And Design-Argument Citations From Production Comments

ACCEPTANCE CRITERIA:
1. `grep -rn '§' --include='*.go'` returns no hits in the step-1 production files.
2. No `Phase [0-9]` / `task [0-9]+-[0-9]+` (any case) remains in those files.
3. No block-quoted spec prose remains; `internal/tui/notice_band.go` states its conclusion in its own words.
4. The load-bearing warnings named in step 5 survive with their meaning intact (startupCanvasHex / OSC 11 set-back, hysteresis justification, "do not add/remove" guard notes).
5. No production code changed — the diff is comments only.

STATUS: complete

SPEC CONTEXT: This is a standards-sourced remediation task, not a spec-feature task. Its governing rule is the project's code-quality standard (mirrored in the review checklist's "Comment accuracy" criterion): a comment must hold for a reader with no knowledge of the process that produced the code, and may not reference process artifacts (task ids, phases, spec sections). The load-bearing exception set it must preserve is documented in CLAUDE.md — the `internal/tui/restore.go` canvas-echo guard ("Do NOT drop the guard"), the `internal/prefs/store.go` `appearance` field-preservation note, and the daemon hysteresis justification block.

IMPLEMENTATION:
- Status: Implemented (commit e30939b2, 66 files, +3554/-3697)
- Location: `internal/theme/*.go`, `internal/tui/*.go` (incl. `notice_band.go`, `restore.go`, `appearance_gate.go`, `builtin_themes.go`, `theme_*.go`, `model.go`), `cmd/theme.go`, `cmd/theme_persister.go`, `cmd/theme_enumerator.go`, `cmd/doctor_theme.go`, `cmd/config.go`, `cmd/open.go`, `cmd/root.go`, `cmd/doctor.go`, `cmd/capturetool/main.go`, `internal/prefs/store.go`, `internal/capture/{fixtures,harness,swatch,theme_fake}.go`.
- Criterion 1 — VERIFIED, and stronger than required: `§` appears in **zero** `.go` files repo-wide, tests included (the ~2,200 cited occurrences are gone).
- Criterion 2 — VERIFIED for every production file: a case-insensitive sweep for `phase [0-9]` / `task [0-9]+-[0-9]+` across all non-`_test.go` Go sources returns nothing. A wider sweep for `.workflows`, `tick-<id>`, `T<n>-<n>`, "plan task", "acceptance criteria" also returns nothing in production sources.
- Criterion 3 — VERIFIED. The block-quoted spec paragraph formerly at `notice_band.go:418-424` is gone; the surviving text is the conclusion in its own words ("Projects has a transient-flash slot of its own because `t` is bound on this page and every theme flash is reachable here…"). In the *current* tree the file is slimmer still (284 lines) — later phases (notably 17-15) further tightened these comments — and `internal/tui/notice_band.go:193-195` now carries the arbiter's rationale as a plain two-sentence claim with no citation and no quotation.
- Criterion 4 — VERIFIED, and one warning was strengthened rather than merely preserved:
  - `internal/tui/restore.go:26-56` keeps "CANVAS-ECHO GUARD — DO NOT DROP THIS GUARD" and the startup-hex anchoring rule, and the rewrite *adds* the enforcement sentence "a canvas-hex-of-the-active-theme helper, in any form — reintroduces both, so no such helper may come back". It states the ban without spelling the forbidden identifier, so `internal/tui/restore_source_guard_test.go:95-108` (which fails on the literal string tree-wide) stays green.
  - `internal/prefs/store.go:71-78` keeps "Do not delete the field: an undeclared key is dropped on re-encode, erasing a downgraded binary's pin", plus the matching note on `ThemeMigrated`.
  - `cmd/state_daemon.go:96-101` (hysteresis justification, with the measurement date and the 2× clamp) is untouched — correctly outside this feature's file set.
- Criterion 5 — VERIFIED. Filtering the commit's `.go` diff to lines that are neither comment-only nor blank leaves exactly two hunks, and in both the change is confined to the trailing comment on the line: `internal/theme/…` `return // the absence of the event IS the failure signal (§10.5)` → `… failure signal`, and `modalHelp // §8.5 per-page ? help …` → `// per-page ? help …`. No `/* */` comment appears in the diff, so nothing hides outside the `//` filter. Compilation and behaviour are structurally unaffected.
- Notes: Rewrite quality is high, not mechanical deletion. Sampled `internal/theme/union.go` (RowSource / Row / SortKey / Label), `internal/theme/builtins.go`, `internal/theme/resolve.go`, `internal/capture/theme_fake.go` and `internal/tui/restore.go`: in every case the citation was replaced by the fact it pointed at ("since names are rejected rather than normalised", "the row that gives the `●` marker something to sit on when the theme it marks has gone") rather than dropped, leaving the claim intact and checkable against the code. The three worked examples the task named as exemplars — `internal/theme/resolve.go` ("what task 5-7 hands over"), `internal/theme/builtins.go` ("Phase 5 consumes them"), `internal/capture/theme_fake.go` ("Task 8-8's panel OPEN") — all now read as statements about the code with no provenance clause left behind.

TESTS:
- Status: Adequate (no new tests expected or added — correct for a comments-only sweep)
- Coverage: The task's own test bar is "build and unit lane stay green; existing source guards stay green". Because the diff is provably comment-only, no compilation unit changed. I read the three guards most exposed to a comment edit and each still holds:
  - `internal/tui/restore_source_guard_test.go` — scans **every** `.go` file for the retired canvas-hex helper name as a raw string. The reworded restore.go comment deliberately paraphrases instead of naming it, so the guard is not tripped; the AST half of the guard reads code only.
  - `internal/tui/retired_token_guard_test.go` — the one guard in the repo that parses with `ParseComments` and asserts on comment text. It post-dates this commit (the token rename landed in a later phase) and the *current* comments in `internal/tui` / `internal/capture` name no retired token, so the sweep's output is compatible with it.
  - `internal/tui/colour_literal_guard_test.go` — AST call-expression based, comment-blind; the sweep introduced no hex literal (verified: no `#rrggbb` on any added line).
- Notes: No test asserts on the *presence* of any comment this task rewrote, so there is no coverage gap created by the edit. Adding a source guard for `§`/`task N-M` would be the only way to make this task self-enforcing, but that is a plan-level decision and the task did not ask for one.

CODE QUALITY:
- Project conventions: Followed. The sweep respects the CLAUDE.md carve-outs (daemon hysteresis block untouched as out-of-feature; `prefs` field-preservation note kept verbatim in meaning; the restore.go guard kept and hardened). `internal/theme/events.go:11` retains "a closed, spec-governed vocabulary" for the `theme` log component — that is the project's own live constraint as stated in CLAUDE.md ("New components/attrs require amending the spec"), not a pointer into `.workflows/theming-system/`, so it is correctly left standing.
- SOLID principles: N/A (no code changed)
- Complexity: N/A (no code changed)
- Modern idioms: N/A
- Readability: Good, and improved. Comments now state invariants directly; the "here is why the rejected alternative was rejected" passages are gone while the "do not do X because it breaks Y" passages survive. Comment-line density in the swept files (e.g. `internal/theme/union.go` 67/242, `internal/tui/theme_panel.go` 76/408, `internal/prefs/store.go` 45/306) is still roughly twice comparable untouched files (`internal/state/capture.go` 16/369, `cmd/open_burst.go` 14/97), but what remains is load-bearing invariant text rather than provenance or argument, which is what the task's outcome asked for; the density gap alone is not a defect.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/capture/theme_panel_fixture_render_test.go:256 — the failure message still cites a plan task: `"…the pinned render size is below task 8-11's floor"`. It is the only surviving `task N-M` citation in the whole Go tree, and it is in a file this feature authored (the sweep was scoped to non-`_test.go` files, so it was legitimately out of scope, but it is the same dangling pointer the task exists to remove, and a reader hits it on a real test failure). Replace the clause with the fact: `"…the pinned render size is below the panel's minimum-entry floor"`.
