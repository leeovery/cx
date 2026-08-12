TASK: 10-10 — Clear This Feature's Captures and Tapes at Sign-Off (tick-417ab6)

ACCEPTANCE CRITERIA (from the plan):
1. Every Phase 3 / 8 / 9 visual gate confirmed taken and signed off before any deletion.
2. The directory listing was inventoried and classified before deleting.
3. No `*.png` or `*.tape` from this feature remains in `testdata/vhs/`.
4. `internal/capture`'s fixture definitions, `cmd/capturetool` and the VHS route untouched; `capture.FixtureNames()` returns the same set before and after.
5. `testdata/vhs/README.md` survives and no longer contradicts task 10-6's CLAUDE.md correction or `capturetool`'s current flag.
6. `LOCK-IN.md`'s fate decided explicitly and the decision stated.
7. `testdata/vhs/reference/`'s fate raised at the gate rather than decided silently, and the agreed action taken.
8. `grep -rn "testdata/vhs" --include="*.go" .` run after deletion; every remaining hit is a comment deliberately kept or updated.
9. Both lanes green after the deletion.
10. `git status` shows deletions of images and tapes only — no fixture, harness or Go source deleted.

STATUS: issues_found

SPEC CONTEXT:
§13.2 draws the retention rule this feature is the first to live under: captures and the tapes that render them are scaffolding — created as work proceeds, committed while collaborated on, cleared out after sign-off — because there is no visual-regression obligation and a permanent image set would rot on the first token rename. Two bounded acts are sanctioned (one at the start of the feature, one at sign-off), explicitly not a rolling clear-as-you-go and not a repo-wide sweep. §13.2 further carves the deletion to images and tapes only: the Go fixture *definitions* and the harness are permanent, because §13.4's swap-and-diff guard enumerates whatever fixtures exist and §13.3 notes that a missing fixture is a blind spot the guard structurally cannot report (absence reads as coverage). §12.6 required CLAUDE.md's capture-harness row to stop describing `testdata/vhs/` as a durable asset (task 10-6), so an unemptied directory contradicts the document an implementing agent reads first.

VERIFICATION METHOD (scope caveat):
My tooling for this review is read-only file access — I have no directory listing, no `grep`, no `git`, and no test execution. I therefore verified by probing individual paths with the Read tool (absent file ⇒ ENOENT error; present file ⇒ content). That establishes presence/absence for the paths I probed but **cannot enumerate `testdata/vhs/` exhaustively**, so the residue list below is a floor, not a complete inventory. Criteria 1, 2, 9 and 10 are process/gate criteria whose evidence (gate transcripts, `git status`, two green lanes) is outside what I can observe; they are reported as unverifiable-by-me rather than failed.

IMPLEMENTATION:
- Status: **Partial / drifted at final sign-off** — the clearing act itself was clearly performed, but this feature's `testdata/vhs/` artifacts are NOT absent from the current tree.
- Location: `testdata/vhs/`, `testdata/vhs/README.md`, `internal/capture/fixtures.go`, `cmd/capturetool/main.go`.

What holds (verified):
- **Pre-feature and Phase 8/9 non-geometry artifacts are gone.** Confirmed absent: `sessions-flat.tape`, `projects.tape`, `contrast-validation-nord.tape`, `sessions-by-project-nord.tape`, `theme-panel-projects.tape`, `theme-panel-invalid-row.tape`, `theme-panel-dir-unreadable.tape`, `theme-panel-commit-failed.tape`. The whole `testdata/vhs/trail/` subtree is gone too (probed `trail/sessions-flat/1-1.png` — absent), so the clear was broad and not a token gesture.
- **Criterion 4 holds.** `internal/capture/fixtures.go:133-163` still declares the full registry — 27 builders including every `themePanel*` fixture whose tape/PNG was deleted (`themePanelProjectsFixture`, `themePanelInvalidRowFixture`, `themePanelDirUnreadableFixture`, `themePanelCommitFailedFixture` …) — and `FixtureNames()` (`fixtures.go:176-185`) still appends `ContrastValidationFixture`. Nothing in the fixture set was swept along with the images, which is the failure mode §13.3 warns is invisible to the guard. `cmd/capturetool/main.go` is intact and still carries `--theme <slug|path>` (`main.go:37`, `110-149`), so the VHS/live-view route survives the artifact deletion.
- **Criterion 5 holds.** `testdata/vhs/README.md:22-57` now states the retention rule verbatim in §13.2's terms ("A capture and its tape are created as work proceeds … cleared out at sign-off"; "an empty-looking directory is the normal resting state"), carries the Lifetime table, and documents `--theme <slug|path>` at `README.md:102-127` including the slug-vs-path rule and the hard-error-never-fallback contract. Read side by side with CLAUDE.md's capture-harness paragraph ("`testdata/vhs/*.png` and `*.tape` are written as work proceeds and cleared out after sign-off rather than living in the repo"; "`testdata/vhs/reference/*.png` is the carve-out and is kept"; "the Go fixture definitions … are permanent"), the two documents agree — no contradiction remains.
- **Criterion 6 holds.** `LOCK-IN.md` survives and its fate is stated, not swept: `README.md:42` records it as **Kept**, with the reasoning ("A historical record; the captures it names were cleared long ago").
- **Criterion 7 holds.** `testdata/vhs/reference/sessions-nord-port.png` is present, and the keep decision is documented in two places rather than taken silently — `README.md:41` (table) + `README.md:44-52` ("Why `reference/` is exempt" … "no Go source cites them by path, so their retention rests on this table"), reinforced by CLAUDE.md's carve-out paragraph (plan task 16-9, "Record The Reference-Frame Retention Carve-Out In CLAUDE.md"). Whether it was raised *at the gate* is not observable from the tree, but the outcome is explicitly recorded, which is the substance of the criterion.
- **Criterion 8, spot-checked clean.** `cmd/capturetool/main.go` contains no `testdata/vhs` path; the head of `internal/capture/capture_test.go` contains none. `internal/capture/fixtures.go:61-63` states the rule rather than a path ("Tapes are scaffolding and do not live in the repository, so the post-load key script is declared here"). I could not run the repo-wide grep.

What fails (criterion 3):
- **Capture artifacts from this feature are still in `testdata/vhs/` in the signed-off tree.** Confirmed present by direct read:
  - `testdata/vhs/theme-panel-confirm.tape` **and** `testdata/vhs/theme-panel-confirm.png`
  - `testdata/vhs/theme-panel-paginated.tape` **and** `testdata/vhs/theme-panel-paginated.png`
  - `testdata/vhs/theme-panel-narrow.tape`
  - `testdata/vhs/theme-panel-adaptive-pair.tape`
  - `testdata/vhs/theme-panel-min-height-message.tape`
  - `testdata/vhs/theme-panel-constant-previewing.tape`
  (I probed only two of the six PNGs; both existed, so the matching PNGs for the other four should be assumed present until the directory is listed.)
- The residue is self-indicting: `theme-panel-confirm.tape:4-10` declares of itself "SCAFFOLDING, NOT AN ASSET (spec §13.2). This tape and the PNG it renders are created for the collaboration on task 9-12 … and CLEARED OUT at sign-off in Phase 10". It was not.
- It also falsifies three documents this same phase wrote: `README.md:33` ("an empty-looking directory is the normal resting state"), CLAUDE.md's retention sentence, and `internal/capture/fixtures.go:61` ("Tapes … do not live in the repository").

- Notes on cause (mitigating, not exculpating): the six survivors are exactly the panel **geometry/state** frames — narrow (width ladder), min-height-message (height floor), paginated (dots), confirm (message slot at minimum width), constant-previewing and adaptive-pair (setting states) — while every non-geometry panel frame was deleted. The plan's later phases re-worked precisely that surface (the priority-1 "Panel Chrome Revision — Page-Matched Vertical Rhythm, Inner Gutter And A Wider Ladder", 13-11 "Reconcile The Panel Width Ladder With The Specified Column Band", 13-12 "Restore The Bold On The Panel Cursor Row", 15-2 "Stop The Panel's Blank Page-Alignment Rows From Raising The Height Refuse Threshold"), and `theme-panel-confirm.tape:89-97` describes the *post*-reconciliation 24/30-column ladder. The most probable history is that 10-10 executed correctly at Phase 10 and these tapes/PNGs were re-created afterwards as scaffolding for those later gates, with no task in Phases 11-17 re-running the clear (the plan contains none; the only other capture-cleanup task, 13-13 "Perform §13.2's Start-Of-Feature Capture Deletion", is `cancelled`). That makes the remedy a repeat of the same bounded act on six artifacts — not a re-do of this task's other halves, all of which stand.

TESTS:
- Status: **N/A by nature, with one reading-verifiable guard intact.** This task ships no code; its "tests" are procedural gates (`go test ./...`, `go test -tags integration -p 1 ./...`, the grep, `git status` reviewed entry by entry, a live `capturetool` run, and a before/after `capture.FixtureNames()` comparison). None are executable within this review's read-only mandate.
- Coverage: the one durable, code-level protection the task must not damage — the fixture registry that §13.4's swap-and-diff guard enumerates — is verifiably undamaged (`internal/capture/fixtures.go:133-185`), and `internal/capture/fixture_registry_test.go` exists to pin it. A before/after `FixtureNames()` diff is unavailable to me, but the registry still names every fixture whose artifacts were deleted, which is the substantive claim.
- Notes: nothing in the suite could have caught the residue — the failure here is "files that should be absent are present", which no assertion in the repo makes. Only the acceptance criterion and a directory listing catch it, which is exactly why criterion 3 is worth failing on.

CODE QUALITY:
- Project conventions: Followed. The README rewrite matches CLAUDE.md's vocabulary and the retention table is the single home the CLAUDE.md paragraph points at ("`testdata/vhs/README.md` holds the full retention table") — one authority, referenced, not duplicated.
- SOLID principles: N/A (documentation + artifact deletion).
- Complexity: N/A.
- Modern idioms: N/A.
- Readability: Good. `README.md`'s "What lives here, and for how long" section states the rule, the table, the `reference/` exemption rationale and the do-not-delete-a-fixture warning in the order a newcomer needs them; the "Adding (or removing) a fixture" step 3 ("Clear the tape and the PNG at sign-off. Leave the fixture.") makes the rule operational rather than aspirational.
- Comment accuracy: `internal/capture/fixtures.go:61-63` and `README.md:33` are both currently falsified by the un-cleared artifacts; both become true again the moment the residue is deleted, so they are consequences of the blocking issue rather than separate defects.
- Issues: none beyond the residue.

BLOCKING ISSUES:
- `testdata/vhs/` still contains this feature's capture scaffolding at sign-off, failing acceptance criterion 3 and contradicting the retention rule this very task wrote into `testdata/vhs/README.md` and CLAUDE.md. Confirmed present: `theme-panel-confirm.{tape,png}`, `theme-panel-paginated.{tape,png}`, `theme-panel-narrow.tape`, `theme-panel-adaptive-pair.tape`, `theme-panel-min-height-message.tape`, `theme-panel-constant-previewing.tape` (assume the four unprobed sibling PNGs too). Remedy: list `testdata/vhs/`, delete every remaining `*.png` and `*.tape` at its top level, keep `README.md`, `LOCK-IN.md` and `reference/`, and touch nothing in `internal/capture` or `cmd/capturetool` — deleting a fixture would silently shrink §13.4's guard. Re-run both lanes afterwards, as the task specifies.

NON-BLOCKING NOTES:
- [do-now] `testdata/vhs/LOCK-IN.md:20-22` — the "Captures (regenerate with `vhs <tape>`…)" bullets name `contrast-validation-light.png` / `.tape` and `contrast-validation-dark.png` / `.tape`, all four of which are gone, so the instruction cannot be followed as written. Replace the parenthetical with the live-view route that still exists: "(the tapes were cleared at sign-off — view the swatch with `go run ./cmd/capturetool --fixture contrast-validation --theme <slug|path>`)". `README.md:42` already acknowledges the captures were cleared, so this only aligns the record with its own index.
- [do-now] `testdata/vhs/README.md:39-40` — the Lifetime table names the tape and PNG rows "Scaffolding — cleared at sign-off" without saying *whose* sign-off when a feature is followed by later remediation phases, which is the gap this feature fell through. Add to the "What lives here, and for how long" prose, after the block quote at line 31: "Sign-off means the *feature's* sign-off, not a phase's — work that re-captures a screen in a later phase owns clearing what it re-captured." One sentence; it turns the rule that was missed into one that names the case that missed it.
