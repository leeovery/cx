TASK: theming-system-10-6 — CLAUDE.md's Five Remaining Stale Entries (tick-6e2508)

ACCEPTANCE CRITERIA:
1. The `tui/theme` and `tui` rows are byte-unchanged by this task's diff.
2. The config-path section names `themesDirPath` as a directory resolver outside `configFilePath`, with no Application Support migration, and lists `PORTAL_THEMES_DIR`.
3. The config-path section records the non-migrating prefs read and its purpose.
4. The TUI wiring clause lists `WithThemePersister` and no longer mentions `WithAppearance` anywhere in the file.
5. `theme` appears in the documented `skipTmuxCheck` set, and that set matches `cmd/root.go`'s map exactly.
6. The `prefs` row documents the four keys and states `appearance` survives as a preserved raw string read but never parsed, with the reason.
7. The logging section reads 18 component names, includes `theme` with its seven attr keys, and notes the three emitting packages under bind-once-per-package.
8. The capture-harness section describes PNGs and tapes as scaffolding, fixtures and harness as permanent, and documents `--theme <slug|path>` (default `tokyo-night`).
9. `grep -n "WithAppearance\|17 component\|--appearance" CLAUDE.md` returns nothing.
10. The MV owned-canvas bullet no longer presents `#0b0c14` / `#e1e2e7` as *the* canvas nor calls it "mode-matched"; the two-layer fill mechanism and height-budget note byte-unchanged.
11. The MV light/dark-detection bullet names the loaded nomination (constant skips the gate, pair resolves it once) and mentions no `appearance` pref; the race, single-resolution rule, dark fallback and `NO_COLOR` carve-out byte-unchanged.
12. The sweep of the MV and Cold-path sections and the Architecture opener was run, and every further falsified clause was corrected or left by explicit decision.
13. No corrigendum block added and no knowledge re-index run for this file.

STATUS: complete

SPEC CONTEXT:
Spec §12.6 ("`CLAUDE.md` is what an implementing agent reads first") tables seven stale entries; Phase 3 owned two (the `tui/theme` and `tui` rows), this task owns the remaining five plus the prose sweep the task description adds on the grounds that §12.6 names a minimum, not a ceiling. Supporting sections: §3.2 (`internal/theme` relocation, `themesDirPath`), §5.5 (`PORTAL_THEMES_DIR`, directory never created), §8.1/§8.8 (prefs key schema; `appearance` kept on disk as a raw round-tripping field, whose deletion "silently erases the user's `appearance` pin … at the moment the user is least likely to notice"), §8.9 (`WithThemePersister`; the `theme`-emitting set closed at three sites), §10.5 (the non-migrating read for doctor), §12.1 (`theme` bootstrap-exempt), §12.3 (the 18th log component, attr keys `slug`/`slot`/`reason`/`path`/`token`/`count`/`rejected`, emission via injected logger with `Discard` on doctor/export/capturetool), §13.2 (capture PNGs are scaffolding; fixtures and harness are permanent), §13.3 (`--theme` replaces `--appearance`).

IMPLEMENTATION:
- Status: Implemented (documentation-only; later phases amended the same sections further, which is expected per the remediation-cycle note).
- Location: commit `bff3f2c4` — `CLAUDE.md` only (+ `.tick/tasks.jsonl`, `.workflows/theming-system/manifest.json` task bookkeeping). Current file: `/Users/leeovery/Code/portal/CLAUDE.md:23-27` (capture harness), `:31` (Architecture opener), `:69` (`prefs` row), `:87-91` (config-path section), `:98` (logging taxonomy), `:126` (bootstrap-exempt set), `:145` (cold-path), `:170-177` (Modern Vivid TUI).
- Notes — every edited clause was cross-checked against the code it describes, all accurate:
  - Criterion 1: the diff shows the `theme` row (post-3-1 rename) and the `tui` row as context lines only — byte-unchanged.
  - Criterion 2/3: `cmd/config.go:194` `themesDirPath` matches the documented `PORTAL_THEMES_DIR` → `xdg.ConfigBase()/portal/themes` chain, creates/stats nothing, and takes no `configFileComponents` entry; `cmd/config.go:93` `loadPrefsStoreNoMigrate` exists with the matching "must stay inert" rationale.
  - Criterion 4: `WithThemePersister` exists (`internal/tui/model.go:531`, applied at `internal/tui/build.go:124`); `grep "WithAppearance\|--appearance\|17 component"` returns nothing.
  - Criterion 5: documented set (`version`, `init`, `help`, `alias`, `hook`, `doctor`, `theme`, `uninstall`, `state`, `__complete`) matches `cmd/root.go:23-34` exactly — same ten keys, no extras, no omissions.
  - Criterion 6: `internal/prefs/store.go:70-78` carries `session_list_mode`, `theme`, `theme_light`, `theme_dark`, `theme_migrated`, all `omitempty`, plus the preserved raw `Appearance string json:"appearance,omitempty"` the row's warning protects.
  - Criterion 7: 18 matches the observability spec's own "single source of truth" component table (18 rows incl. `log-rotate`, which the code binds inside `internal/log`); the seven `theme` attr keys match `internal/theme/events.go` emissions exactly; the emitting sites are `internal/theme`'s `EventLogger`, `cmd/config.go:170`, and `cmd/theme_persister.go:27`, with the single binding at `cmd/open.go:27`.
  - Criterion 8: `cmd/capturetool/main.go:37` declares `--theme` defaulting to `theme.DefaultDarkSlug` = `tokyo-night` (`internal/theme/builtins.go:13`); no `--appearance` flag remains. The permanence claim is real — `internal/capture/theme_swap_guard_test.go` + `swap_harness_test.go` drive the fixture renderer over `capture.FixtureNames()`.
  - Criteria 10/11: the retained clauses are byte-identical in the diff; the only lead-in change ("The canvas mode is chosen" → "When armed, the canvas is chosen") is required by the retirement of the "mode" framing. `internal/tui/appearance_gate.go:31-47` confirms the documented behaviour (constant/zero nomination → `pinned`, `arm()` no-ops; adaptive → single-resolution; `resolveDark` fallback).
  - Criterion 12: the sweep demonstrably ran and found more than the two named bullets — the Architecture opener ("owned mode-matched canvas" → "owned canvas painted from the active theme"), the cold-path section ("11-step" → "ten-step" ×2, `state.red` → `state.destructive`, the appearance-gate sentence), and the header caret ("violet" → "`accent.primary`"). All verified: `internal/theme/theme.go:67,72` declare `accent.primary` and `state.destructive`; `internal/tui/loading_view.go:237` renders the fatal frame with `th.StateDestructive`; `internal/tui/header.go:97` uses `th.AccentPrimary`; the orchestrator is documented (and enumerated) as ten steps.
  - Criterion 13: commit touched no `.workflows/` corrigendum and ran no re-index — only the CLAUDE.md edit plus task-state bookkeeping.
  - Deliberate, justified deviation from the task text: the task asked for "three packages"; the implementation wrote "three sites across two packages", which is what the code shows (`cmd/config.go` and `cmd/theme_persister.go` are one package) and what the amended observability spec (`portal-observability-layer/specification.md:189`) now says verbatim. This is a correction, not drift.
  - One sweep miss, since closed: at commit time the MV render-structure bullet still named `internal/tui/right_anchored_row.go`, a file that has never existed in this repo's history (the helper lives in `footer.go`). It was inside the swept section, so the sweep should have caught it; phase-14 task 14-14 corrected it to "`footer.go`'s `assembleRightAnchoredRow`". No residual action.

TESTS:
- Status: Adequate (documentation task — the plan specifies grep/diff checks, not new Go tests; adding a guard test for prose was correctly not attempted).
- Coverage: all eight stated checks re-run and passing — `WithAppearance`/`--appearance` absent; `mode-matched`/`#0b0c14`/`#e1e2e7` absent; `18 component names` present and `17 component` absent; the documented exempt set diffed key-for-key against `cmd/root.go`'s map; `themesDirPath` + `PORTAL_THEMES_DIR` present in the config-path section; "preserved raw string" present in the `prefs` row; the Phase 3 rows appear as context-only hunks; and every documented symbol resolves in code (`WithThemePersister`, `themesDirPath`, `loadPrefsStoreNoMigrate`, `capturetool --theme`, the `theme` component's attr keys, `accent.primary`, `state.destructive`).
- Notes: no test executed (per instructions); adequacy judged by reading. The one structural gap is inherent to the entry class the task itself flags — the `skipTmuxCheck` set is still quoted verbatim with no compiler or test signal tying it to `cmd/root.go`. It is correct today; nothing in the repo keeps it correct. Raised below as an idea rather than a defect, since the task was scoped to correcting the prose, not to inventing a guard.

CODE QUALITY:
- Project conventions: Followed. Edits match CLAUDE.md's established register (dense, rationale-carrying prose; the "why" stated alongside the "what"), and the two highest-risk clauses — the preserved `appearance` field and the fixtures-are-permanent carve-out — both carry the consequence, not just the rule, which is what makes them survive a future editor.
- SOLID principles: N/A (documentation).
- Complexity: N/A.
- Modern idioms: N/A.
- Readability: Good. The themes-directory paragraph is split out rather than bolted onto the already-long config-path sentence, and the capture-harness scaffolding/permanence distinction gets its own paragraph — both are the clauses most likely to be skimmed, so the separation is load-bearing.
- Comment accuracy: N/A for source comments; every prose claim introduced by this task was verified true against the code above.
- Issues: none.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] CLAUDE.md:98 — the logging paragraph says doctor/export/capturetool "construct it with `log.Discard()`", but no call site does: all three use `theme.NewSilentLoader()` (`cmd/doctor_theme.go:52`, `cmd/theme.go:54`, `cmd/capturetool/main.go:87`). Replace "construct it with `log.Discard()` and write nothing" with "construct it via `theme.NewSilentLoader()` (an `EventLogger` over `log.Discard()`) and write nothing", so the claim is greppable to the constructor that implements it.
- [do-now] CLAUDE.md:41 and after :45 — the feature's user-facing verb is never introduced: `theme` appears in the bootstrap-exempt set (:126) and `portal theme export` in the logging paragraph (:98), but no Command flow entry defines it, and the Diagnostics entry omits doctor's new theme advisory lines. Add after the Resume-hook entry: "**Theme command:** `cmd/theme.go` — `portal theme export <slug>` writes the resolved theme file's bytes to stdout verbatim (comments preserved — it is the file, not a re-serialisation of the parsed `Theme`), refusing `not found` / `unreadable` / invalid with a plain error. Bootstrap-exempt; the loader is silent (`theme.NewSilentLoader`), so an export emits no `theme` events." And in the Diagnostics entry, after "as a host-terminal check line", add ", and renders one advisory line per unusable theme file plus a line per unresolvable persisted slug (read-only — `--fix` has no theme repair)".
- [idea] CLAUDE.md:126 — the `skipTmuxCheck` set is quoted verbatim with nothing binding it to `cmd/root.go:23`; this task had to re-derive it by hand and the same silent-staleness hazard remains for the next verb. Decide whether to add a unit-lane guard that parses the documented list out of CLAUDE.md and diffs it against the map (the repo already has ~20 source guards via `sourceguardtest`, so the precedent exists), or to accept the prose as unguarded — it is a design call about how much of CLAUDE.md becomes machine-checked, not a mechanical edit.
