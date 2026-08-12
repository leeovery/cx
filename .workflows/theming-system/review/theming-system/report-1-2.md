TASK: 1.2 — Lex `.theme` Lines Into Key/Value Pairs Or Exactly One `bad syntax` Rejection (theming-system-1-2)

ACCEPTANCE CRITERIA:
- `text.primary = #ECEFF4 # tuned for the lighter canvas` lexes to one pair whose value is `#ECEFF4 # tuned for the lighter canvas` verbatim.
- `  text.primary  =  #ECEFF4  ` lexes to key `text.primary`, value `#ECEFF4`; `   # note` is a comment.
- A CRLF file lexes identically to its LF twin; a BOM at file start is invisible; a BOM anywhere else is `bad syntax`.
- An empty file and a comments-only file lex to zero pairs with no rejection.
- `= #FFFFFF`, `text.primary`, `text primary = #FFF` each yield `bad syntax` / `line N: not a key = value pair`.
- A duplicated key yields `bad syntax` naming the second line, for all four {known, unknown} x {same, different value} combinations.
- `"#FFFFFF"`, `'#FFFFFF'`, `"#FFFFFF` all yield `bad syntax` / `line N: quoted value` — never `bad colour`.
- `text.primary = #ECEFF4 = x` lexes to value `#ECEFF4 = x`.
- `text.primary =` lexes to an empty value (a well-formed pair, not a syntax error).
- The rejection is a single `*Rejection`; no partial pairs alongside an error.

STATUS: complete

SPEC CONTEXT:
§4.2 (specification.md:243–272) is the branch-by-branch specification of this task: `#` starts a comment only at line start (so a `#` right of `=` is part of the value), values are bare (any leading quote — matched or not — is `bad syntax`, deliberately not `bad colour`), duplicate keys are rejected lexically and unconditionally, each line is trimmed at both ends before classification, CRLF is tolerated and a BOM is stripped at file start only, and a well-formed key is non-empty with no whitespace and no `=` (so a typo'd key reports the typo rather than misdirecting to `missing tokens`). §4.1 (l.241) explicitly closes the empty-value branch — an empty value must reach `bad colour`, not fail here. §6.2 (l.431–456) fixes the seven-reason vocabulary and the short-circuiting ladder, `bad syntax` at rung 4. §14A (l.1862) pins the three detail phrases verbatim, with a duplicate naming the second occurrence's line.

IMPLEMENTATION:
- Status: Implemented (later phases touched comments only — no behavioural drift)
- Location:
  - `internal/theme/reason.go:1–49` — `Reason` string type with the seven terse labels in ladder order, and the `Rejection` struct (`Reason`/`Detail`/`Line`, plus `BadNameCause`/`Tokens`/`Values`/`Err` added by later planned tasks 1.5, 13.7, 17.9) implementing `error` via `*Rejection`.
  - `internal/theme/lex.go:9–112` — `Pair{Key,Value,Line}`, `lexPairs`, `lexLine`, `trimLine`, `wellFormedKey`, `startsQuoted`, `badSyntax`, and the three detail-phrase constants.
  - Consumer: `internal/theme/load.go:106–112` (`parseThemeBytes` = lex then validate), so `bad syntax` short-circuits before any value or presence check; `cmd/doctor_theme.go:218–223` prints `Rejection.Detail` verbatim, so the pinned phrases reach doctor unmodified.
- Every acceptance criterion verified against the current code:
  - `#` handling: comment detection is `strings.HasPrefix(text, "#")` on the *trimmed* line only, and the value is `strings.Cut(text, "=")`'s remainder trimmed — nothing right of the first `=` is re-interpreted (lex.go:39, 66–73).
  - Line numbering: comments/blanks are iterated, not skipped, so `index+1` is the original file's 1-based line (lex.go:35–36).
  - BOM: one leading `﻿` stripped from the whole file (lex.go:35); any surviving BOM on an interpreted line rejects (lex.go:62). `unicode.IsSpace` does not cover U+FEFF, so `TrimSpace` cannot silently swallow an interior BOM — the reject path is genuinely reachable and is pinned by test.
  - Duplicate check is purely lexical, after per-line syntax and before any known/unknown classification, and never compares values (lex.go:47–51); it names the second occurrence's line.
  - Quoted values judged on the first character alone, either quote (lex.go:102–104).
  - Empty value deliberately has no branch — `text.primary =` yields `Pair{Value: ""}` and fails later at `wellFormedHex` (validate.go:50, 120) as `bad colour`, exactly as §4.1/§4.2 require.
  - On rejection `nil` pairs are returned (lex.go:44–49), so no partial file ever escapes.
- Notes:
  - `wellFormedKey`'s `strings.Contains(key, "=")` clause is unreachable through `lexPairs` (the pre-first-`=` half of a `Cut` can never contain `=`). It is a faithful transcription of the task's key definition and harmless, but the comment that used to explain its unreachability was removed by the phase-11/17 comment audit — see the note below.
  - A BOM *inside a comment body* (`# note﻿more`) is tolerated rather than rejected, because the comment skip precedes the BOM check. A strict reading of §4.2's "a BOM anywhere but the first bytes" would reject it; the tolerated case is invisible to the user, discarded before it can corrupt a key or value, and has no better detail phrase in §14A's closed set of three. Judged correct as built, not drift.

TESTS:
- Status: Adequate
- Coverage: `internal/theme/lex_test.go` (339 lines, package-internal, as `lexPairs` is unexported) contains all 13 test functions the task names, with the four required tables:
  - Happy path + 1-based line numbers over a file with a comment and a blank (l.11–24).
  - Whole-line and around-separator trimming, incl. tabs and an indented comment (l.26–38).
  - The `#ECEFF4 # tuned for the lighter canvas` forcing case, asserting the value verbatim (l.40–48).
  - CRLF file compared against its LF twin's own lexed output (l.50–61) — combined with the literal-value test above, this pins both identity and content.
  - BOM stripped at file start (l.63–73) and interior BOM rejected in four positions incl. the doubled-BOM case (l.154–194).
  - Blank/comment-only files, five variants, zero pairs and no rejection (l.75–95).
  - Malformed lines, five cases incl. whitespace-only key and tab-in-key (l.196–242).
  - Duplicate key, exactly the four {known, unknown} x {same, different value} combinations, each asserting the second occurrence's line (l.244–284).
  - All three leading-quote shapes (l.286–303).
  - First-`=`-only split, incl. `text=primary = …` (l.97–123); trailing whitespace (l.125–132); empty value in three separator shapes (l.134–152).
  - `requireBadSyntax` (l.321–339) asserts reason, line, exact detail string *and* that no pairs came back — so the "single rejection, no partial pairs" criterion is checked on every rejection case rather than once.
  - `reason_test.go` pins the seven label strings and their order (as an external `theme_test` package), the `error` implementation via a compile-time assertion (l.10), and `Error()`'s two renderings.
- Notes:
  - Details are stated as complete literals by each case (`"line 3: duplicate key text.primary"`), never re-derived from the production format string — the right choice for user-facing copy, and it is what makes these tests fail if the phrases drift.
  - No over-testing: the only apparent overlap (`TestLex_TrimsLineAndAroundSeparator` vs `TestLex_TrailingWhitespaceIsNotAnError`) is explicitly required by the task, and the second is the §4.2 row distinguishing trailing from interior whitespace.
  - Downstream coverage of the ladder interaction exists where it belongs, not duplicated here: `internal/theme/load_test.go:314–317` pins that a file which is simultaneously duplicate-keyed, bad-coloured and missing a token reports `bad syntax`.
  - Unreachable-by-construction `strings.Contains(key, "=")` is the only production branch with no test; it cannot be exercised through the package's own API.

CODE QUALITY:
- Project conventions: Followed. Test files named after their source (`lex.go` -> `lex_test.go`, `reason.go` -> `reason_test.go`), matching the golang-testing skill's file-per-source rule; internal-package tests without the `_internal_test.go` suffix are consistent with `validate_test.go` (the suffix is used in this package only where an external counterpart exists). No `t.Parallel()`, per CLAUDE.md. `internal/theme` stays loader-shaped: `lex.go` imports only `fmt`/`strings`/`unicode` and decides nothing about logging or paths.
- SOLID principles: Good. One responsibility per function (file split, line classification, key rule, quote rule, rejection construction); the lexer knows nothing about the 19 token names, which is exactly what §6.2's ladder requires and what lets `validate.go` own the value/presence rungs.
- Complexity: Low. `lexPairs` is a single loop with three exits; `lexLine` is four guarded returns; the predicates are one expression each.
- Modern idioms: Yes — `strings.Cut` for the first-separator split (the idiom that makes the "never re-interpret anything right of the first `=`" rule structural rather than remembered), `strings.ContainsFunc(key, unicode.IsSpace)`, `slices.Equal` in the test helper. `strings.Split` with a used index is correct here; `SplitSeq` cannot carry the line number.
- Readability: Good. Detail phrases are named constants adjacent to their producer, `badSyntax` is the single place `"line %d: %s"` is composed, and the comments state why rather than what.
- Comment accuracy: Comments hold against the code, and the phase-11/17 audit removed the spec-section and phase citations. One gap noted below (the removed rationale left an unreachable branch unexplained).
- Security: N/A — pure byte lexing, no execution, no path handling, closed key set (§4.4 note).
- Performance: N/A at this scale — one `Split` allocation over a ~25-line file, read once at load.
- Issues: none blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/lex.go:90–97 — `wellFormedKey`'s `strings.Contains(key, "=")` clause is unreachable via `lexPairs` (the left half of a first-`=` `Cut` cannot contain `=`), and the comment explaining that was dropped in the comment audit; a reader now meets a branch no input reaches. Append to the existing doc comment: "The `=` clause is unreachable through the first-`=` split — it is stated so the definition of a well-formed key does not silently depend on the splitter."
- [idea] internal/theme/lex.go:85–87 — `trimLine`'s `strings.TrimSuffix(raw, "\r")` is redundant: the `strings.TrimSpace` that wraps it already removes a trailing `\r` (and any run of them), so removing the `TrimSuffix` is behaviour-identical for every input. Decide whether the explicit CRLF step is worth keeping as intent (the comment above it already carries that intent) or should be dropped as dead work.
