TASK: theming-system-16-9 — Record The Reference-Frame Retention Carve-Out In CLAUDE.md (tick-887927, severity low, sources: standards)

ACCEPTANCE CRITERIA:
- CLAUDE.md's capture-harness paragraph states the `reference/` carve-out and its reason.
- CLAUDE.md and `testdata/vhs/README.md` no longer disagree about what is cleared at sign-off.
- No file outside CLAUDE.md changes.

STATUS: complete

SPEC CONTEXT:
Spec §13.2 ("Committed reference PNGs were scaffolding, not an asset") draws the retention rule this task documents: no visual-regression obligation, so images and the tapes that render them are created as work proceeds, committed while collaborated on, and cleared at sign-off; the Go fixture definitions in `internal/capture` and the harness are permanent. Spec §14's CLAUDE.md-amendment table (line 1571) explicitly lists "The visual capture harness section" as an entry this feature must correct, because it read as describing a durable asset. §13.2 as written deletes *the pre-feature* committed reference PNGs — it does not itself describe a `testdata/vhs/reference/` carve-out; that carve-out arose in flight from the reference-first visual process (export and commit the design frame *before* implementing against it), where the artefacts are design exports rather than renders of the code. This task's job was to make CLAUDE.md say so, since CLAUDE.md is what an implementing agent reads first.

IMPLEMENTATION:
- Status: Implemented (and intentionally refined by a later in-plan sync)
- Location: /Users/leeovery/Code/portal/CLAUDE.md:27 (the capture-harness retention paragraph). Commit 2eb64f04 ("record the reference-frame retention carve-out in CLAUDE.md"); subsequently amended by daada4dd ("docs: sync the reference-frame and hysteresis notes to the stripped source").
- Notes:
  - The clause as delivered by 2eb64f04 justified retention partly by "production and test comments cite them by path (`internal/tui/keymap.go`, `internal/tui/loading_view.go`, many sites in `internal/capture/fixtures.go`)". The plan's later comment-strip work removed all 21 in-source citations (comments must carry no artifact paths), which would have left that sentence false. daada4dd corrected it to "Nothing in the Go source points at them (code comments carry no artifact paths), so a sign-off sweep must not treat them as orphans" and added the "for some screens they are the only reference that exists" reason in its place, and synced the matching sentence in `testdata/vhs/README.md:50-52`. This is the "later task superseded the mechanism" case from the verifier context, not drift: the *outcome* the task exists for (CLAUDE.md states the carve-out and its reason, and the two documents agree) holds in the current tree, with a reason that is true of the current source.
  - Verified the current wording's factual claims rather than taking them on trust:
    * `testdata/vhs/reference/` exists and holds 30 committed PNGs.
    * No Go source cites an artefact path: `grep -rniE "\.png|vhs/|reference frame" --include="*.go" .` returns nothing, so "Nothing in the Go source points at them" is accurate as scoped.
    * `testdata/vhs/README.md:37-43` holds the full retention table, with `reference/*.png` marked **Kept** — so "README holds the full retention table" is accurate and the table was not restated in CLAUDE.md (Do-item 3 respected).
    * The two globs named in the swept half (`testdata/vhs/*.png`, `testdata/vhs/*.tape`) match the live directory: 6 tapes and 6 PNGs, name-paired 1:1 (`theme-panel-{adaptive-pair,confirm,constant-previewing,min-height-message,narrow,paginated}`), matching CLAUDE.md:25's "a `.tape` plus its rendered PNG for each screen under active work".
    * `internal/capture` exists as described (fixtures.go, harness.go, swatch.go, theme_fake.go, fakes.go + the guard tests), so the "permanent" half of the paragraph still names real things.
  - Do-item 2 (addition, not rewrite) respected: the pre-existing sentences about the harness and about `internal/capture` fixture definitions being permanent are byte-identical either side of the diff; the change is a single inserted clause plus the glob-scoping of "they".
  - Do-item 4 (change no code, no capture) respected: the diff of 2eb64f04 touches CLAUDE.md, `.tick/tasks.jsonl` and `.workflows/theming-system/manifest.json` only — the latter two are workflow bookkeeping the harness writes for every task, not repository content. No source file, test, tape or PNG changed.

TESTS:
- Status: Adequate (documentation-only change; the task's own test list is a read-through plus the unchanged suite)
- Coverage: No Go code changed by this task, so the suite is unaffected and no new coverage is owed. The task's stated verification — "a read-through confirming the two documents state the same rule, and that every directory named in the CLAUDE.md clause exists as described" — was executed here and passes on both halves:
  * Same rule: CLAUDE.md:27 ("`testdata/vhs/*.png` and `testdata/vhs/*.tape` … cleared out after sign-off"; "`testdata/vhs/reference/*.png` is the carve-out and is kept") vs README:29-31 (the blockquote rule) + README:37-43 (the table: tape/png = "Scaffolding — cleared at sign-off", `reference/*.png` = "**Kept**") + README:45-52 ("Why `reference/` is exempt"). No contradiction remains on scope, on reason, or on the no-inbound-Go-reference point.
  * Directories named exist as described: `testdata/vhs/`, `testdata/vhs/reference/`, `testdata/vhs/README.md`, `internal/capture` — all present and matching their descriptions.
- Notes: Nothing here is amenable to (or worth) an automated guard; a doc-consistency test would be over-testing. Per instructions the suite was not executed — no code path is reachable from this change, so a read-level judgement is sufficient.

CODE QUALITY:
- Project conventions: Followed. The amendment lands in the existing capture-harness section rather than creating a new one, keeps CLAUDE.md's house style (bolded rule, em-dash reason, pointer to the detailed home), and honours the standing convention that comments/docs carry no stale artefact pointers — indeed the daada4dd sync exists precisely to keep this paragraph honest after the comment strip.
- SOLID principles: N/A (no code).
- Complexity: Low — one clause, one scoping edit.
- Modern idioms: N/A.
- Readability: Good. The paragraph now reads as one rule with one exception rather than a rule that the repository visibly violates; the swept set is glob-scoped (`*.png`/`*.tape`) so `LOCK-IN.md` and `reference/` are outside the sweep by construction rather than by the reader's inference.
- Comment accuracy: The delivered text's factual claims were checked against the tree and all hold (see IMPLEMENTATION). Note the one claim that did *not* survive — the in-source-citation justification — was caught and corrected inside this plan; nothing stale remains at CLAUDE.md:27.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] .workflows/theming-system/specification/theming-system/specification.md:~1596 (§13.2 "Retention rule, drawn now") — the spec's rule still reads as unconditional ("Everything that exists today as an image or tape is deleted — the committed reference PNGs and the VHS tapes that produce them"), with no `testdata/vhs/reference/` carve-out; CLAUDE.md and `testdata/vhs/README.md` now both document one. The carve-out was adopted in flight (reference-first: export and commit the design frame before implementing) and covers design exports rather than renders of the code, so the spec is not wrong about what it decided — but a future reader reconciling the three documents finds two-of-three agreement. Concrete change: add one sentence to §13.2 recording that design exports committed under `reference/` are kept, distinct from the captures the rule sweeps. Tagged idea rather than do-now because whether a signed-off spec is amended post-hoc (versus leaving the deviation recorded only in the review trail) is a decision, not a transcription.
