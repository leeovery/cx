TASK: theming-system-1-3 — Turn Lexed Pairs Into A Validated Theme — Value Domain And Presence

ACCEPTANCE CRITERIA:
- `#FFF`, `#FFFFFFFF`, `#GGGGGG`, `` (empty), `#FF FFFF`, `blue`, `212`, `-5`, `16777215` each yield `bad colour` when carried by a known key.
- `text.primary = #c0caf5` produces `Token.Value == "#C0CAF5"`; a lowercase file and its uppercase twin produce identical `Theme` values.
- An unknown key with a malformed value contributes no rejection and appears in no detail; the file validates if the 19 known keys are present and well-formed.
- `Text.Primary = #FFFFFF` is treated as unknown, so the file fails `missing tokens` with `text.primary` named in the detail.
- A file that lexed to zero pairs yields `missing tokens` listing all 19 names in §2.4 order.
- With both a bad colour and a missing token, the reason is `bad colour` and the detail carries no token-presence information.
- Details are deterministic: bad-colour pairs in file order, missing names in §2.4 order.

STATUS: complete

SPEC CONTEXT:
§4.3 fixes the value domain at `#RRGGBB` hex only — no ANSI indices, no named colours, no `#RGB` shorthand — because `lipgloss.Color` never errors and silently absorbs `212` / `-5` / `16777215`, and because an ANSI index has no fixed RGB (which would foreclose numeric contrast checking). Input case is unconstrained and the parser canonicalises to uppercase, load-bearing for §11.4's retained startup canvas hex and §11.3's background diffing. §4.4/§4.6 make unknown keys ignored entirely (key *and* value), which is the forward-compatibility lever that lets a removed token's stale line survive. §4.5 requires all 19 tokens — no merge, no partial file — because canvas is itself a token. §6.1 makes validity purely syntactic (never perceptual). §6.2 fixes the ladder: `bad colour` (rung 5) is evaluated across known keys only and short-circuits before `missing tokens` (rung 6), and doctor "enumerates within the reason, not across reasons". §14A pins the detail copy: `text.primary = #GGGGGG, canvas = blue` for bad colour, `missing text.primary, bg.subtle` for missing tokens.

IMPLEMENTATION:
- Status: Implemented (with two later plan tasks deliberately extending it — see Notes)
- Location:
  - `internal/theme/validate.go:22` `themeFromPairs` — two ordered stages, zero `Theme` on rejection.
  - `internal/theme/validate.go:41` `applyPairs` — case-sensitive known-key lookup, unknown keys `continue`d before any value inspection, offenders accumulated in file order, `strings.ToUpper` canonicalisation on store (`:55`), offending value echoed uncanonicalised.
  - `internal/theme/validate.go:73` `requireEveryToken` — walks the canonical `fields()` table in §2.4 order, empty `Value` = absent, detail `missing <names>`.
  - `internal/theme/validate.go:120` `wellFormedHex` / `:133` `isHexDigit` — exactly `#` + six hex digits, length-first so `#FFF`, `#FFFFFFFF`, `#FF FFFF`, `blue`, `212`, `-5`, `16777215` and `""` all fail without a second parse branch.
  - Wired at `internal/theme/load.go:106` `parseThemeBytes` (lex → validate), the single content path for disk and embedded bytes alike, so a built-in cannot bypass the validator.
  - Canonical table consumed via `internal/theme/theme.go:58` `fields()`, so names/order/count derive from one source.
- Notes:
  - Every acceptance criterion is satisfied against the current tree. Rung ordering is enforced structurally (`applyPairs` returns before `requireEveryToken` is called), so `bad colour` detail can carry no presence information by construction.
  - Two later plan tasks intentionally extended this task's output rather than drifting from it: 13-7 added `Rejection.Tokens` and 17-9 unified `Tokens`/`Values` across both reasons (`internal/theme/reason.go:27`). `renderedPairs` (`validate.go:95`) was factored out so `internal/theme/events.go:159` can render the log attr from the structured lists instead of re-parsing `Detail`. The pinned §14A copy is unchanged by this.
  - `themeFromPairs` stays unexported and is reached only through `parseThemeBytes`, matching §6.2's "no caller may lex or validate on its own".

TESTS:
- Status: Adequate
- Coverage (`internal/theme/validate_test.go`, unit lane, white-box so the unexported entry point is exercised directly):
  - `TestValidate_AcceptsNineteenWellFormedTokens:12` — happy path, asserting both `All()` order/values and, via reflection, the per-field stored `Name` (the second assertion is not redundant: `All()` re-derives names from the table and would mask a wrong stored `Name`).
  - `TestValidate_RejectsMalformedHexForms:26` — named subtests over exactly the nine forms in the acceptance criteria, each asserting reason *and* the echoed detail.
  - `TestValidate_CanonicalisesHexToUppercase:53` — the `#c0caf5 → #C0CAF5` claim plus whole-struct equality of a lowercase file against its uppercase twin, which is the §4.3 comparison-site guarantee.
  - `TestValidate_IgnoresUnknownKeyAndItsValue:71`, `TestValidate_WrongCaseKeyFailsAsMissingTokens:83`, `TestValidate_EmptyFileMissesAllNineteen:91` (nil and empty-slice subtests), `TestValidate_BadColourPrecedesMissingTokens:168`.
  - Determinism is tested rather than assumed: the bad-colour and missing-token enumeration tests feed a `reversed()` pair list (plus a leading unknown key) and assert bad-colour follows *file* order while missing names follow *table* order — the one construction that would catch an accidental map iteration.
  - `requireRejection:277` additionally pins the zero `Theme` and `Line == 0` on every rejection, so a partly-populated palette or a stray line number cannot escape.
  - The §2.4-order assertions read `TokenNames()`, which is not self-referential: `internal/theme/theme_test.go:87` pins that list against a literal 19-name table.
- Notes:
  - Not over-tested: no mocks, no setup beyond slice helpers, each test states one claim. The one overlap is `TestValidate_MissingTokensCarriesTheAbsentNamesAsData:137` repeating the preceding test's setup and detail assertion verbatim before adding its `Tokens`/`Values` claims (see notes).
  - One defensive branch is uncovered: `renderedPairs:98` (`i >= len(values)` → bare name) is unreachable from `applyPairs`, which always appends in lockstep, and no test drives it through `events.Rejected`.

CODE QUALITY:
- Project conventions: Followed. Leaf-package discipline holds (no `internal/log` import in `validate.go`; logging stays behind the injected `EventLogger`). No raw hex at a renderer call site. Test file is named after the source file, no `t.Parallel()`, table-driven with named subtests, `got`/`want` phrasing throughout, `t.Helper()` on both assertion helpers. Comments carry no spec-section, phase or task citations (the 11-3/12-8 sweeps hold on this file).
- SOLID principles: Good. `themeFromPairs` composes two single-purpose stages; the canonical table is the single source of names/order; the validator owns syntax only and knows nothing about files, paths, slugs or logging.
- Complexity: Low. No stage exceeds one loop and one branch; `wellFormedHex` is length-check-then-digits with no alternate parse path, which is exactly the "six digits remove a parse branch" argument in §4.3.
- Modern idioms: Yes. `len("#RRGGBB")` as a self-documenting compile-time constant, byte-wise digit test rather than a regexp on a hot-ish path, `slices`/`maps` in tests, Go 1.22+ integer `range` in the reflection helper.
- Readability: Good. Names state intent (`wellFormedHex`, `requireEveryToken`, `applyPairs`), and each comment explains a decision the code cannot state (why Portal owns the validator, why unknown keys are skipped whole, why the offending value is echoed uncanonicalised) rather than restating the code. Comments verified against the code: all hold.
- Issues: None material. Security: no injection surface in the validator itself; see the `[idea]` note on the echoed value. Performance: one map build per file load, irrelevant at 19 keys.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/validate.go:15 — `detailMissingTokensLeadIn` has no reference anywhere but the next line; collapse the pair to a single `detailMissingTokens = "missing %s"` and drop the lead-in constant.
- [quickfix] internal/theme/validate_test.go:137 — `TestValidate_MissingTokensCarriesTheAbsentNamesAsData` re-runs the identical setup and the identical `requireRejection(..., "missing text.primary, bg.subtle")` assertion as the test directly above it; drop the duplicated detail literal from this test and keep only its `Tokens` / empty-`Values` / detail-derived-from-`Tokens` claims, so each test states one claim.
- [quickfix] internal/theme/events_test.go:117 — add a bad-colour row with `Tokens` longer than `Values` (e.g. `Tokens: {"canvas","text.primary"}, Values: {"blue"}`) asserting `token=canvas = blue, text.primary`, so `renderedPairs`' unpaired-list degrade branch (`validate.go:98`) is covered rather than only asserted in a comment.
- [idea] internal/theme/validate.go:55 — the offending value is echoed verbatim into `Rejection.Detail`, which doctor prints to a terminal; a `.theme` file value containing an ANSI escape or control byte therefore reaches the terminal unstripped. §9.4/§12.1 control-strip slugs from `prefs.json` and CLI arguments for exactly this reason but say nothing about token values. Decide whether to extend the same strip to the echoed value (and, if so, whether it belongs here or in the lexer alongside `wellFormedKey`).
