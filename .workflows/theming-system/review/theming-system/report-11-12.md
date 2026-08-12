TASK: theming-system-11-12 — Make The Docs Example Theme Provably Equal To The Shipped Dark Built-In

ACCEPTANCE CRITERIA:
- A guard test fails if any token value in `tokyo-night.theme` changes without the doc block following.
- The guard tolerates the differing header comment and nothing else.
- The existing "example parses as a valid theme" assertion survives.
- `docs/theming.md` is unchanged by this task.

STATUS: complete

SPEC CONTEXT:
`.workflows/theming-system/specification/theming-system/specification.md` §12.4 / §13.5 (lines 1515-1536, 1726, 1898-1900) make `docs/theming.md` "the source of truth for the public contract" and require a guard on it. The spec's baseline for the example block is weaker than this task: line 1900 — "**The doc's example theme** | Covered by the same guard as the doc — must parse and contain all 19 keys, so it is not an unguarded fourth copy." This remediation task strengthens that to full equality with the embedded dark built-in, which is consistent with the spec's stated intent (the doc must not be an unguarded copy) and with the doc's own prose claim at `docs/theming.md:93` ("These are the values of Portal's dark built-in."). Not drift — a strengthening within the spec's purpose. Spec line 620/707 confirm palette re-derivation is an ordinary, contemplated change, which is exactly the drift class this guard closes.

IMPLEMENTATION:
- Status: Implemented
- Location: `/Users/leeovery/Code/portal/internal/theme/docs_guard_test.go` — new live guard `TestThemingDocExampleThemeIsTheDarkBuiltin` (:128-137); comparison helper `auditDocExampleMatchesBuiltin` (:329-353); header-stripping helper `bodyAfterHeaderComment` (:355-368); fixture helpers `requireDarkBuiltinSource` (:370-378), `exampleFromBody` (:382-384), `rewriteFirstMatchingLine` (:386-405), `movedValue` (:407-416), `restatedComment` (:418-423). Commit `bd175e05` touched only this test file (plus workflow/tick bookkeeping) — no production change, no doc change, matching AC 4.
- Notes:
  - The task's fallback ("export a minimal read-only accessor if none is reachable") was correctly unnecessary: `theme.BuiltinBytes` already exists (`internal/theme/builtins.go:30`) and returns the embedded bytes verbatim, itself pinned against the committed file by `internal/theme/builtins_test.go:42`. So the guard's "built-in" side is anchored to real shipped bytes, not to a restatement.
  - Anchored on `DefaultDarkSlug` rather than a hardcoded `"tokyo-night"` — correct, since the doc's claim is about *the dark built-in*, so re-pointing the default relocates the guard's subject automatically.
  - Comparison semantics verified by hand against the two live artefacts. `docs/theming.md:96-98` is a 3-line header; `internal/theme/builtins/tokyo-night.theme:1-4` is a 4-line header; both are stripped by `bodyAfterHeaderComment`, and the remaining 25 lines (blank separators, the three section comments, and all 19 `key = value` lines) are byte-identical and in the same order. The live guard is currently green and the AC-1 failure mode (a value moving in the `.theme` file only) produces exactly one problem naming the diverging line.
  - AC 2 ("header comment and nothing else") is enforced precisely: `bodyAfterHeaderComment` strips only the *leading* run of `#`-prefixed lines, so section comments, blank lines and ordering are all compared verbatim. Length asymmetry is handled in both directions ("stops short of" / "runs past"), so a truncated or extended doc block cannot pass.
  - A residual, deliberate tolerance: the doc may also *omit* its header entirely, or match after a header rewrite on either side. That is the intended scope ("modulo the header comment") — the doc's header addresses a copier, the file's addresses a maintainer — and is not drift.
  - AC 3 satisfied: `TestThemingDocExampleThemeIsValid` (:97-116) is untouched and still parses the block through `parseThemeBytes` and asserts no token parsed empty.
  - A later non-plan commit (`25626754 chore(comments): strip internal/theme to the code-quality standard`) removed the doc comments this task shipped. The code is unchanged; the current file simply carries no comments. No stale-comment exposure remains (there are none), so nothing to flag under comment accuracy.

TESTS:
- Status: Adequate
- Coverage:
  - Live identity: `TestThemingDocExampleThemeIsTheDarkBuiltin` (:128) — real doc vs real embedded built-in.
  - Negative (the task's required case): `TestThemingDocGuard_ExampleValueDivergingFromBuiltinFails` (:149) mutates one in-memory copy (`text.primary = #010203`) and asserts exactly one problem names it — the AC-1 failure mode proven, not assumed.
  - Tolerance boundary: `TestThemingDocGuard_ExampleHeaderCommentMayDiffer` (:139) proves the forgiven difference, driven from the built-in's own body so it stays meaningful even if the two headers ever converged.
  - Tolerance ceiling: `TestThemingDocGuard_ExampleSectionCommentDivergingFromBuiltinFails` (:158) proves the "nothing else" half of AC 2 — a non-leading comment is a failure.
  - Vacuity: `TestThemingDocGuard_ExampleWithNoBodyFailsLoudly` (:167) refuses a comparison over two header-only sources, so the guard cannot report success without having read a palette. Traced by hand: both sides reduce to zero body lines, the short-circuit at :337 fires, and the message contains "no theme lines" as asserted.
- Notes:
  - Not over-tested: five cases, each pinning a distinct branch (identity / tolerated / value drift / comment drift / vacuous). No redundant assertions, no mocks, no setup beyond reading two in-repo files.
  - `rewriteFirstMatchingLine` (:386) is a good failure-mode choice — it *finds* the line to mutate instead of naming a palette value, so the negative cases don't break on the very palette re-derivation the guard exists to catch, and it fails loudly if a rewrite is a no-op or matches nothing.
  - `TestThemingDocExampleThemeIsValid` is now logically implied by the equality guard (the built-in's own validity is separately guarded), but AC 3 explicitly requires keeping it, so its retention is compliance rather than redundancy.
  - Lane/convention: no build tag (correct — unit lane, hermetic file reads only), no `t.Parallel()` (matches the CLAUDE.md prohibition), white-box `package theme` as needed for `parseThemeBytes`, and helpers all call `t.Helper()`.
  - Gap (minor): the `no fenced block under the "Example theme" heading` branch of `auditDocExampleMatchesBuiltin` (:331) has no negative case, unlike every other branch of the new helper. The live guard would still fail loudly if the fence or heading disappeared, so this is coverage tidiness rather than a hole.

CODE QUALITY:
- Project conventions: Followed. Test file named after its subject and co-located with the pre-existing doc guards; message style ("problem = %q, want …", "got/want" framing) matches the surrounding file and the repo's guard idiom; failures name the file under audit (`themingDocPath`) so a reader knows what to act on.
- SOLID principles: Good. `auditDocExampleMatchesBuiltin` is a pure `([]byte, []byte) -> []string` audit with no `*testing.T` dependency, which is what lets the same function serve the live guard and all four negative cases — single responsibility, and the reason the negative cases are cheap.
- Complexity: Low. The comparison is one bounded loop with a three-arm switch; `bodyAfterHeaderComment` is a single scan.
- Modern idioms: Yes — `for i := range max(len(a), len(b))` (Go 1.22+ range-over-int with builtin `max`), `strings.Cut`, `slices.Clone`, `strings.SplitSeq` in the neighbouring pre-existing helper.
- Readability: Good. Names state the claim (`requireDarkBuiltinSource`, `movedValue`, `restatedComment`), and the problem strings are full sentences a reader can act on without opening the test.
- Security: N/A — read-only reads of two in-repo artefacts.
- Performance: N/A — trivial, unit-lane cost.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/docs_guard_test.go:277 and :332 — the identical `fmt.Sprintf("no fenced block under the %q heading", exampleThemeHeading)` problem string is now built in both `auditDocExampleTheme` and `auditDocExampleMatchesBuiltin`; extract it to a shared helper (e.g. `missingExampleProblem() string`) so the two audits cannot drift in what they report for the same condition.
- [quickfix] internal/theme/docs_guard_test.go:331 — add a negative case for the missing-fence branch (call `auditDocExampleMatchesBuiltin` with a doc carrying no `Example theme` heading and assert exactly one problem naming the heading), so every branch of the new helper is pinned the way the divergence and vacuity branches already are.
