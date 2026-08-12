TASK: theming-system-1-1 — Declare The 19-Token Vocabulary In A New internal/theme Leaf Package

ACCEPTANCE CRITERIA:
- `Theme` declares exactly 19 fields, all of type `Token`, and `reflect.TypeOf(Theme{}).NumField() == len(Theme{}.All())`.
- `TokenNames()` equals, in order: text.primary, text.secondary, text.tertiary, text.muted, text.subtle, text.faint, text.on-selection, accent.primary, accent.key, accent.mode, accent.attention, state.positive, state.destructive, canvas, bg.selection, bg.attention, bg.subtle, border, text.on-attention.
- `All()` returns tokens in exactly that order — whole-slice assertion, not a set.
- No `border.footer` / `BorderFooter` field; no `Light`/`Dark` fields, no `Mode` type, no `ColorFor` method, no package-level `MV` (or any other) built-in `Theme` var.
- `Token{}.Color()` returns a `color.Color` without panicking; `Token{Value: "#C0CAF5"}.Color()` resolves through `lipgloss.Color`.
- `internal/tui/theme` byte-unchanged; `go build ./... && go test ./...` green.

STATUS: complete

SPEC CONTEXT:
§2.4 is the rename table — 19 rows, each pinning `token name → Go field name`, and §3.2 states the data shape: `Token` becomes `{Name, Value}`, `ColorFor`/`theme.Mode` are removed in favour of a no-argument `Token.Color()`, `Theme` stays a struct of 19 named `Token` fields with a stable-order `All()` but ceases to be a package-level built-in var, and it carries **no identity field** (the slug is held alongside by whoever loaded it — what makes `capturetool --theme <path>` possible). §3.2 also pins the move to a new leaf package `internal/theme` so `cmd/doctor.go` and `portal theme export` don't import a TUI subpackage. "`All()`'s stable order is the §2.4 table order, 1 through 19 — the numbering *is* the definition." §2.5 holds the role meanings, and §12.4 makes `docs/theming.md` (not the Go comments) the source of truth for that public contract.

IMPLEMENTATION:
- Status: Implemented (one acceptance criterion intentionally superseded by a later phase — see Notes).
- Location:
  - `/Users/leeovery/Code/portal/internal/theme/theme.go:5` package doc; `:15` `Token{Name, Value}`; `:22` `Token.Color()`; `:26-49` the 19-field `Theme`; `:51-80` the canonical `fieldRef` table `Theme.fields()`; `:85` `All()`; `:96` `TokenNames()`.
  - Downstream single-source proof: `/Users/leeovery/Code/portal/internal/theme/validate.go:24` is the parser's key→field assignment and derives from the same `fields()` table; `internal/theme/theme.go` is the **only** production Go file in the repo carrying token-name string literals (verified by search for `"text.primary"` across `*.go`).
  - Contract propagation: `/Users/leeovery/Code/portal/internal/theme/docs_guard_test.go:48` binds `docs/theming.md`'s role table to `TokenNames()`, so the §2.5 contract cannot drift from the vocabulary.
- Notes:
  - Names, spelling and order match spec §2.4 rows 1–19 exactly (checked row by row against specification.md:94-112), including the `border.separator`+`border.footer` → `border` consolidation and the `text.on-attention` lockstep rename.
  - `Theme` carries no identity/slug field and there is no package-level `Theme` var — verified against the whole-package export list and by search: no `ColorFor`, no `theme.MV`, no `BorderFooter` anywhere in `*.go`.
  - **Superseded, not drift:** the "`internal/tui/theme` is byte-unchanged" criterion was a phase-boundary guard for this task's commit only. That package is now deleted (Phase 3 deleted it as the task's own Context section anticipated), so the criterion is judged against the amended intent — the vocabulary lives solely in `internal/theme`, which is the outcome §3.2 demanded.
  - Also superseded: the task's "Comment each field with its §2.5 role sentence" instruction. Two later comment-standard passes (25626754, 915e7fcb) stripped the per-field role comments deliberately, and §12.4 makes `docs/theming.md` the contract's source of truth (guarded by `docs_guard_test.go`), so the Go-side duplication was correctly removed rather than lost. One artefact of that strip survives and is wrong — see NON-BLOCKING NOTES.
  - `All()` has a value receiver calling the pointer-receiver `fields()`; legal (the receiver copy is addressable) and it is what makes `All()` non-mutating. `TokenNames()` allocating a throwaway `Theme` is the price of a single table and is not on a hot path.
  - Package is import-clean for its stated role: `leaf_guard_test.go` proves it depends on no `internal/xdg`, reads no `PORTAL_THEMES_DIR`/home dir, declares no hex literals and does no init-time work.

TESTS:
- Status: Adequate.
- Coverage (`/Users/leeovery/Code/portal/internal/theme/theme_test.go`):
  - `TestTokenCount_IsNineteen:39` — pins 19 for `All()` and `TokenNames()`, and self-checks the guard's own literal name table first (so a corrupted expectation fails loudly instead of silently weakening).
  - `TestAll_ReturnsSpecTableOrder:51` — seeds each of the 19 fields with a **distinct** value and asserts the whole ordered `{Name, Value}` slice. This is the strongest assertion in the file: it catches reordering, and crucially catches a *mis-paired* table row (e.g. `{Name: "text.primary", Field: &t.TextSecondary}`), which a names-only assertion would miss.
  - `TestTokenNames_MatchExactSet:87` — ordered `slices.Equal` against the literal list (the deliberate second statement of the names, so a rename must be made twice).
  - `TestAll_CoversEveryStructField:93` — `reflect` field count vs `len(All())`, plus every field is a `Token`. A field added without a table row fails here.
  - `TestTokenColor_ZeroValueDoesNotPanic:108` and `TestTokenColor_ResolvesHexThroughLipgloss:118` — the latter goes beyond identity comparison to convert through `color.RGBAModel` and assert the exact RGBA, so it would fail if `Color()` stopped resolving hex.
  - `TestVocabulary_HasNoModeSurface:245` — AST-derived exported-symbol list plus a `Token` field-name assertion; `Mode`, `ColorFor`, `Light`/`Dark` and any built-in var are excluded by construction.
- Notes:
  - Not over-tested for the vocabulary itself: each test pins a distinct failure mode and the assertions do not overlap redundantly.
  - The one over-reach is `TestVocabulary_HasNoModeSurface`: its `wantExports` list has grown to ~108 entries covering the whole package, so it now locks every unrelated export under a test named for the token vocabulary. That is what the plan asked for, and it has real value (no accidental export), but the name misdescribes it — see NON-BLOCKING NOTES.
  - Edge cases from the task are all covered: skip/duplicate/reorder (count + ordered slice), stale name on a renamed field (names come from the table and are asserted literally), zero-value `Color()` safety for hand-built themes, and "table order, not alphabetical/grouped/struct order" (the seeded ordered assertion is order-sensitive and independent of struct layout).

CODE QUALITY:
- Project conventions: Followed. Leaf package with the closed vocabulary; no hex literals in Go (`leaf_guard_test.go` enforces); no `t.Parallel()`; guards routed through `sourceguardtest`.
- SOLID principles: Good. One canonical table is the single source for `All()`, `TokenNames()` and the parser's key→field assignment (`validate.go:24`) — no second literal list exists in production code, which is the invariant the task was built to protect.
- Complexity: Low. Three short table-driven functions, no branching.
- Modern idioms: Yes — `reflect.TypeFor`, `Type.Fields()` range-over-func, `slices.Equal` in the guard; `image/color` return type rather than a lipgloss-specific type.
- Readability: Good. The table reads as the spec table, so a reviewer can diff it against §2.4 by eye.
- Issues: One inaccurate comment (below). No security or performance concerns — this is pure data.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/theme.go:27 — the comment `// TextFaint is decorative only — never content a user must read.` sits immediately above `TextPrimary`, so godoc and any reader attribute a claim about the ramp's *floor* to the ramp's *top*, which is exactly backwards (`text.primary` is names, wordmark, modal titles). It is residue from commit 915e7fcb dropping the leading "Text ramp, brightest to faintest." sentence off what was a group comment. Move the line to sit directly above `TextFaint` (theme.go:33), or delete it — `docs/theming.md` already carries the role sentence and is the guarded source of truth.
- [quickfix] internal/theme/theme_test.go:245 — rename `TestVocabulary_HasNoModeSurface` (and its `wantExports` var at :136) to something that says what it actually pins, e.g. `TestPackageExports_MatchTheDeclaredSurface`, and leave a small vocabulary-scoped test holding the `Token`-fields assertion at :250-257. As it stands, a contributor whose unrelated new export fails the suite is pointed at a test named for a light/dark surface that has not existed since Phase 1.
- [quickfix] internal/tui/*_test.go (≈171 occurrences across ~30 files; e.g. empty_states_test.go:103,106,109,112 and multi_select_footer_test.go:115,118,121) — assertion messages still name the pre-§2.4 tokens (`accent.blue`, `text.detail`, `accent.violet`, `border.footer`) while asserting against the renamed fields (`AccentKey`, `TextMuted`, `AccentPrimary`, `Border`), so a failure message names a token the vocabulary no longer holds. Sweep the messages onto the current names. Not a `[do-now]` because the sweep needs per-site judgement: `internal/tui/retired_token_guard_test.go`, `internal/tui/active_theme_test.go` and `internal/theme/docs_guard_test.go:76` name the retired tokens deliberately and must be left alone. Belongs to the Phase 3 render-layer rename rather than to this task, recorded here because it is the same rename contract this task established.
