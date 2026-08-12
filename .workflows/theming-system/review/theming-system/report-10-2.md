TASK: theming-system-10-2 — The File Format and Discovery Half — Writing a Theme File and Where It Goes (docs/theming.md)

ACCEPTANCE CRITERIA:
1. Comment rule documented as line-start-only, with the "every value starts with `#`" reason.
2. Bare values, duplicate-key rejection, whitespace trimming, case-sensitive keys and malformed-line rejection all documented.
3. Value domain documented as `#RRGGBB` only, explicitly excluding `#RGB`, ANSI indices and named colours.
4. Both vocabulary levers together — unknown key ignored, missing key rejects the whole theme — with "no merge, no inherits, no partial files".
5. Filename section precedes the directory section, carrying the identity rule, the `^[a-z0-9][a-z0-9-]*$` charset and the exactly-lowercase `.theme` extension.
6. `Nord.theme` documented as rejected rather than lowercased; a wrong-cased extension as visible-then-rejected.
7. `PORTAL_THEMES_DIR` printed verbatim with the full three-step chain and the `_DIR`-versus-`_FILE` note.
8. The five enumeration rules documented (top-level only, symlinked files followed, symlinked root followed, directory-valued entries skipped silently, dangling symlink → `unreadable`).
9. Absent directory documented as silent and never created; unreadable one as a reported misconfiguration.
10. The two-line workflow appears with `mkdir -p` and an explicit note that it is part of the workflow.
11. `portal theme list` and a `--theme` flag appear nowhere in the doc.
12. Task 10-1's guard still green — the token table's 19 rows and the example theme untouched by this edit.

STATUS: complete

SPEC CONTEXT: §12.4 makes `docs/theming.md` the source of truth for the public theme contract and splits it in two: a guarded vocabulary half (10-1) and a hand-maintained discovery half (this task) covering the file format, filename rules, directory resolution, enumeration and the two-line drop-in workflow. §4.2 pins the lexical rules (comment at line start only, bare values, duplicate keys rejected, both-ends trimming, case-sensitive keys, malformed line → `bad syntax`), §4.3 the `#RRGGBB`-only value domain and its two reasons, §4.5/§4.6 full replacement plus the ignore-unknown / reject-missing pair, §5.1–§5.3 the filename-is-identity rule with the slug charset and exact lowercase extension (reject, never normalise — §5.2), §5.5 the `PORTAL_THEMES_DIR` → `XDG_CONFIG_HOME/portal/themes/` → `~/.config/portal/themes/` chain and the absent/unreadable directory-state table, §5.6 the five enumeration rules, §5.8 the re-read-on-every-open loop, and §12.1 the `mkdir -p` + `portal theme export` workflow.

IMPLEMENTATION:
- Status: Implemented (docs-only, as intended)
- Location: `/Users/leeovery/Code/portal/docs/theming.md:126-324` (added wholesale by commit `700f82b7`, a docs-only diff: `.tick/tasks.jsonl`, the workflow manifest and `docs/theming.md` +195 lines — no Go source touched).
- Criterion-by-criterion:
  1. `docs/theming.md:133-151` — "`#` starts a comment only at the start of a line", "after any leading whitespace", "**This is the rule worth reading twice**, because every value in a theme file begins with `#`", with a worked example whose verdict (`bad colour`, naming the key) matches `internal/theme/lex.go:66-81` + `internal/theme/validate.go:41-67`. ✓
  2. `docs/theming.md:155-161` — the five rules in one table; each matches the loader: bare values (`lex.go:77,102-104` — first character, matched or not), duplicate key → `bad syntax` (`lex.go:47-49`), both-ends trim then around `=` (`lex.go:38,73`), case-sensitive keys (`validate.go:107-115`, and the doc correctly routes `Text.Primary` through unknown-key → missing-token), malformed line (`lex.go:61-76`). ✓
  3. `docs/theming.md:171-184` — `#RRGGBB` only, "no `#RGB` shorthand, no ANSI colour number (`212`), and no colour name (`blue`)", plus §4.3's two reasons (own hues via truecolor; an ANSI index has no fixed RGB so it can never be contrast-checked). Matches `validate.go:120-138`. ✓
  4. `docs/theming.md:186-206` — both levers under one heading, ending in "no merge, no `inherits` / `parent` / `base` key, and no partial files", plus the odd-rejection explanation (typo'd key → unknown → fails for the role it was meant to fill). Matches `validate.go:19-34,45-56,73-89`. ✓
  5. "Naming the file" at `:208` precedes "Where the file goes" at `:247`; identity rule `:213-216`, charset `:217-220` (verbatim `^[a-z0-9][a-z0-9-]*$`, with the leading-hyphen and no-length-limit edges from §5.2), extension `:221`. Matches `internal/theme/name.go:13,34-45,65-75`. ✓
  6. `docs/theming.md:236-245` — "Portal rejects a name; it never repairs one", the shadowing reason, and `nord.THEME` "still **listed**, and then rejected for its extension" with the case-insensitive-filesystem justification. The two doctor lines quoted at `:232-233` are byte-exact against `cmd/doctor_theme.go:16-17`, and the cause routing is exact: `Nord.theme` → `BadNameSlug` (`name.go:66-72`), `nord.THEME` → `BadNameExtension` (`name.go:78-90`), enumeration visibility from `enumerate.go:71-73`. ✓
  7. `docs/theming.md:249-257` — the chain printed verbatim in three numbered steps, plus the `_DIR`-vs-`PORTAL_TERMINALS_FILE` note. Matches `cmd/config.go:191-205`. ✓
  8. `docs/theming.md:261-276` — all five rules, each traceable: top-level only (`enumerate.go:28-52`, no recursion), symlinked files followed with the slug from the link name (`dirEntry.Name()` at `:37`), symlinked root followed (`statThemeDir` uses `os.Stat`, `enumerate.go:126-138`), directory-valued entry skipped silently (`resolvesToDirectory`, `:42-44,78-81`), dangling symlink listed then `unreadable` (`:77-80` comment + `classify`). The bonus paragraph at `:274-276` (a non-`.theme` name is ignored completely) matches `isCandidate`. ✓
  9. `docs/theming.md:278-293` — "An absent themes directory is silent… no doctor line, no log entry… Portal never creates the directory and never seeds it", and the unreadable/regular-file case with the doctor line quoted byte-exact against `cmd/doctor_theme.go:14` and the panel's pinned row against `internal/tui/theme_panel.go:19`. Matches `enumerate.go:126-137` (absent → `(false, nil)`) and `cmd/config.go:191-193`. ✓
  10. `docs/theming.md:302-311` — the two lines verbatim including `mkdir -p`, followed by "**The `mkdir -p` is part of that workflow, not an omission.**" and the redirect-error reason. ✓
  11. Grepped: neither `theme list` nor `--theme` occurs anywhere in the doc. `portal theme export <slug>` is named as the route to a built-in's bytes (`:297-300`), and its failure shape (`:318-324`) is accurate — `cmd/theme.go:38-47` returns a plain error, `cmd/root.go:162-163` silences cobra's own printing, and `main.go:69-78` prints the bare message to stderr and exits 1. ✓
  12. Structurally satisfied: the commit touched no Go file and inserted only new sections below the guarded ones. Re-read of `internal/theme/docs_guard_test.go:222-248` against the current doc confirms the guard still parses exactly the three `| Token | Role |` tables (7+6+6 = 19 rows at `:47-55`, `:68-75`, `:81-88`); the new "The rest of the rules" table opens `| Rule | Detail |` and 10-3's later `| Slug | Palette |` / `| Theme | Source |` tables never set `inTokenTable`, so no stray row is captured. The example block at `:95-124` is still line-identical to `internal/theme/builtins/tokyo-night.theme:5-29` below its header comment, which is what `auditDocExampleMatchesBuiltin` compares. ✓
- Notes: I verified the doc's factual claims against the current code rather than the state at commit time, per the "later phases may supersede" instruction; nothing in phases 11–17 moved any of the mechanisms this half describes (constants, chain, ladder and enumeration rules all still read as documented). The doc stays out of the guarded table's territory — the discovery half names token keys only inside examples and never restates a role's meaning or opens a second token list.

TESTS:
- Status: Adequate (for a documentation task with a deliberately hand-maintained half)
- Coverage: The only automated obligation is that task 10-1's two guards stay green — `TestThemingDocTokenTableMatchesAllTokens` and `TestThemingDocExampleThemeIsValid` (plus `TestThemingDocExampleThemeIsTheDarkBuiltin`), all in `internal/theme/docs_guard_test.go`. Judged by reading: the parse boundaries (`parseDocTokenRows` keys on a `Token` header cell and resets on any non-table line) are unaffected by every table this task added, and the example fence is untouched, so the guards hold. The task's remaining tests are manual walkthroughs (scratch `PORTAL_THEMES_DIR` export, `nord.THEME`, `-bad.theme`, trailing comment, absent directory); I could not execute them, but each asserted outcome is pinned by code I read — `cmd/doctor_theme.go:14-18` for the three quoted advisory lines, `name.go:65-90` for the two `bad name` causes, `validate.go:120-131` for the trailing-comment `bad colour`, `enumerate.go:126-131` for the silent absent directory. Every user-visible string quoted in the doc matches its production constant exactly.
- Notes: No new automated test is expected here — §12.4 states the discovery half has no automated check and is maintained by hand, so the absence is a spec decision rather than under-testing. Not over-tested: this task added no tests at all, which is correct for a docs-only change.

CODE QUALITY:
- Project conventions: Followed. Docs-only change following the `docs/custom-terminals.md` precedent; hand-wrapped at 80 columns (the >80-byte lines are em-dash multibyte artefacts, not over-long lines) with long-form markdown tables, matching the rest of the file.
- SOLID principles: N/A (no code).
- Complexity: N/A.
- Modern idioms: N/A.
- Readability: Good. Ordering is the one the task demanded and the one a user needs — format → colour values → vocabulary levers → filename → directory → enumeration → directory states → workflow — so nothing is described before the rule that governs writing it. Each rule is followed by its reason, and the doctor/export output blocks give the reader the exact string they will see.
- Comment accuracy: N/A for source; the doc's quoted outputs and rule statements were each checked against the implementation and hold.
- Issues: None blocking. Three small gaps listed below.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] docs/theming.md:161 — add a sixth row to "The rest of the rules" for §4.2's Encoding rule, the one lexical rule the doc omits: `| **Line endings and BOM** | A CRLF file reads the same as an LF one, and a byte-order mark at the very start of the file is stripped. A BOM anywhere else rejects the file (`bad syntax`). |` (implemented at `internal/theme/lex.go:35,62-63,85-87`).
- [do-now] docs/theming.md:171-174 — add one sentence to the "Colour values" paragraph covering the empty right-hand side, which an editing user will hit: "A key with nothing after the `=` is a `bad colour` too, not a syntax error — the line is a well-formed pair whose value simply is not a colour." (spec §4.2's branch table; `internal/theme/validate.go:120-122`).
- [do-now] docs/theming.md:251 — "`PORTAL_THEMES_DIR`, when it is set" → "`PORTAL_THEMES_DIR`, when it is set to a non-empty value", matching `cmd/config.go:195`, which falls through to the XDG step on an empty value.
