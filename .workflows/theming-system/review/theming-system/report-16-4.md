TASK: theming-system-16-4 — Rewrite The Topic's Production Comments That Name A Test Or Count Call Sites

ACCEPTANCE CRITERIA:
- No production comment in the named files names a test, a guard test, or a fixture set, or describes what a test proves.
- No production comment in the named files counts implementations, call sites, emission sites, readers or reachable packages.
- Each rewritten comment still states the invariant the original was protecting.
- `LoadPath`'s comment claims sharing only where the code is actually shared.
- The listed trap warnings survive verbatim.
- `git diff` shows comment-line changes only; `go build ./... && go test ./...` pass and `golangci-lint run` is clean.

STATUS: complete

SPEC CONTEXT: This is a standards/remediation task, not a spec-behaviour task. The governing standard is `.claude/skills/…/code-quality.md` as quoted in the task: comments may not claim what a test pins/proves (a rename turns the claim into a lie) and may not make cardinality claims ("the only caller", "the single site"), which ordinary additive change falsifies. The named surfaces are the theming feature's production files: `cmd/open.go`, `cmd/theme_persister.go`, `internal/theme/{load,resolve,builtins,theme,union}.go`, `internal/tui/theme_panel.go`. CLAUDE.md's counterpart constraint — the `startupCanvasHex` / canvas-echo "do not drop the guard" note — is the explicitly protected trap warning.

IMPLEMENTATION:
- Status: Implemented (delivered in 735daeeb; several of the rewritten paragraphs were later superseded by the `chore/comment-strip` sweep — ab0d1583 / e3fa15038 / a4bc7bd5 — which trimmed the same comments further in the same direction. Verified against both the commit and the current tree.)
- Location: 735daeeb touched only comment lines in cmd/open.go, cmd/theme_persister.go, internal/theme/{builtins,load,resolve,theme,union}.go, internal/tui/theme_panel.go.
- Notes — each named site checked, at the commit and in the current tree:
  - `internal/theme/load.go:26-30` (Loader.ReservedSlugs): the "a source guard keeps it out of production code" clause is deleted, per Do item 3; the invariant survives as "anything resolving a user's theme must be built by NewLoader or NewSilentLoader or it has no shadowing protection" (current internal/theme/load.go:13-16).
  - `internal/theme/resolve.go:31-33`, `internal/theme/builtins.go:145-147`, `internal/theme/union.go:267`: every "build-time validation of the embedded set …" claim is gone. A repo-wide grep of non-test Go for "build-time validation" / "always valid" now returns only an unrelated UTF-8 comment (internal/tui/pagepreview.go:39).
  - `internal/theme/builtins.go:56`/`:161`, `internal/theme/union.go:183`, `internal/theme/resolve.go:33`: the "reachable from internal/capture under its no-real-config import guard" formulations are restated as the property ("stays reachable where config discovery is forbidden" — current internal/theme/builtins.go:20-21), which is the rule the code depends on rather than the name of the package that exercises it.
  - `internal/tui/theme_panel.go:26`, `:193`, `:544-549`, `:726-727`: the colour-literal/swap-and-diff guard references, the "a paginating fixture exercises the dots" sentence (deleted outright per Do item 3), the fixture-inventory description of the cursor filter, and the "which the swap-and-diff guard structurally cannot see" clause are all gone. The invariants survive: pagination-dot strings are read out of the styles once so the restyle must re-feed the paginator (current internal/tui/theme_panel.go:301-303); the Selectable filter keeps a seed off a row the arrows cannot return to (current :227-229).
  - Cardinality: `load.go:207` ("the only implementation of it") → the rule "no caller may lex or validate on its own" (current internal/theme/load.go:103-105); `theme.go:86-87` → the derivation rule, now "Names, order and count all derive from this table rather than from the struct"; `cmd/theme_persister.go:22-23` ("the emission site … which otherwise has none") → the placement rationale (prefs is a no-logging leaf, the write needs prefs path resolution); `cmd/open.go:789` ("the ONE construction-time theme read") → "the construction-time snapshot the panel keeps for its whole life, never re-read". A grep of the named files for `the ONE` / `the ONLY` returns nothing.
  - `LoadPath` (load.go:190): the commit corrected the over-claim to "Rungs 4 to 6 are parseThemeBytes, the same code LoadFile and LoadBuiltin run … The read and its `unreadable` verdict are not shared code". The later strip reduced it to "the content rungs only, no filename rungs and no slug" (current internal/theme/load.go:91-93), which asserts no sharing at all — the criterion holds in the current tree either way. One nuance at commit time: the read call is duplicated but the `unreadable(err)` classifier is a shared helper, so "its `unreadable` verdict [is] not shared code" was slightly over-tight; superseded, no action.
  - Trap warnings intact: the canvas-echo guard and its "must stay anchored to the retained startup canvas hex, never re-derived from the active theme" rationale is present at internal/tui/restore.go:15-19 (its file was not touched by this commit).
  - Comment-only constraint verified mechanically: `git show 735daeeb -- cmd internal` filtered to changed lines that are not `//` comments yields zero lines. The `//go:embed builtins/*.theme` directive's adjacency to `var builtinFS embed.FS` is preserved through the edited block above it — the one real hazard in a comment-only edit near a directive, and it was handled.

TESTS:
- Status: Adequate (comment-only task; verification is the unit lane + lint + a diff/grep review, not new coverage).
- Coverage: The change cannot affect behaviour — every changed line in the Go files is a comment line, confirmed by filtering the commit diff. `.golangci.yml` runs the standard set plus `modernize`; none of those linters enforces a doc-comment identifier prefix, and in any case no rewritten comment's first line changed, so lint status is unaffected. The task's grep criterion holds against the named files today: `Test[A-Z]`, `_test`, "the only", "nowhere else", "which otherwise has none", "the ONE" return nothing; the surviving "guard" hits (internal/tui/theme_panel.go:272, :303, cmd/open.go:237) all name code guards — an `if !m.themePanel.open` early return, the `--ack` flag guard — not guard tests.
- Notes: No new test was expected or added, correctly. The behaviour these comments describe is already covered by the theming feature's own suites.

CODE QUALITY:
- Project conventions: Followed. The rewrites match the code-quality standard the task cites — invariants stated as properties of the code, no enforcer named, no counts. No workflow vocabulary (task ids, phase numbers, spec citations) was introduced into any comment.
- SOLID principles: N/A (no logic changed).
- Complexity: Unchanged.
- Modern idioms: N/A.
- Readability: Improved. The replacements are shorter and self-contained; a reader with no knowledge of the test suite can now act on every one of them. Two commit-time wordings were slightly awkward — cmd/open.go's openTUI paragraph was left with a ragged short line after the edit, and theme_persister.go's "recorded here rather than beneath or above" is oblique — but the later comment-strip sweep replaced both, so neither survives in the tree.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_panel.go:205 — the comment reads "The false return is a resolution naming no slot — a shape a fixture can hand back; callers degrade on it rather than selecting a zero Theme." This is the only remaining reference to a fixture across the eight named files, and it was reintroduced *after* this task by the comment-strip sweep (e3fa15038), not left behind by it. It names an external harness rather than the property the branch protects. Replace with: "// The false return is a resolution naming no slot — a shape only a hand-built\n// resolution produces; callers degrade on it rather than selecting a zero Theme."
