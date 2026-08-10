## Attempt 1

ISSUES:
- `cmd/doctor_theme_test.go:390` and `cmd/doctor_theme_test.go:817-818` — the two INLINE mutation sites lost their fixture-vacuity guard. The old code went through `themeLineIndex`, which FATALS when the built-in no longer declares the key; the shared mutators are silent no-ops by design. Both tests then pass vacuously.
  EMPIRICALLY CONFIRMED by renaming a token out of the canonical table + built-ins and running only these tests:
    - with `bg.subtle` renamed, `TestThemeAdvisories_OneReasonPerFile/a doubly-broken file…` FAILS loudly at HEAD but PASSES at HEAD+patch — the "doubly-broken" fixture silently became singly-broken and the test no longer tests its stated claim;
    - with `text.primary` renamed, `TestThemeAdvisories_BadNameNeverReportsContent` likewise FAILS at HEAD, PASSES after — its "broken at every rung below" fixture is then unbroken, so the loop asserting the advisory names no content reason is arguing about a clean file.
  This is the exact class the diff itself guards against for the three `cmd` helpers, and that this file already invests in explicitly (see the `requireDeniedRead` vacuity note at line 826), so the omission is internally inconsistent.
  FIX: hoist the guards to LINES level so both the byte-returning helpers and the inline sites share them. Add to `cmd/theme_test.go` beside the existing helpers:
    - `missingTokenLines(t *testing.T, lines []string, keys ...string) []string`
    - `badColourLines(t *testing.T, lines []string, overrides ...themeOverride) []string`
    - `duplicateKeyLines(t *testing.T, lines []string, key string, at int) []string`
  each holding the outcome check that currently sits in `sourceMissingTokens` (line 635) / `sourceBadColours` (line 652) / `sourceDuplicateKeyAt` (lines 678-683). Reduce the three `sourceX` helpers to `themetest.Render(xLines(t, themeKeyLines(t), …))`. Then rewrite:
    - line 390 → `lines := missingTokenLines(t, badColourLines(t, themeKeyLines(t), themeOverride{"canvas", "blue"}), "bg.subtle")`
    - lines 817-818 → `lines := duplicateKeyLines(t, badColourLines(t, themeKeyLines(t), themeOverride{"canvas", "blue"}), "text.primary", len(themeKeyLines(t))+1)`
  Fixture bytes stay identical (the guards only fatal). Re-run the rename simulation above and confirm both tests FAIL loudly again.
  ALTERNATIVE: minimal-diff — leave the helpers as they are and reinstate `themeLineIndex(t, lines, key)` as an explicit pre-check at the two inline sites before mutating. Cheaper and exactly restores prior behaviour, but leaves the guard expressed three different ways in one file and reads as a bare side-effecting call. The first is recommended: it also removes the duplicated guard now sitting inside `sourceMissingTokens` and `sourceBadColours`, which is squarely this task's DRY intent.
  CONFIDENCE: high (that the regression is real and in scope); medium (on which shape to take).

COMMENT_CORRECTIONS:
- `internal/themetest/theme_file.go:91-93` — the panic claim is false on the path documented one sentence earlier: with an undeclared key the function early-returns and never splices, so no `at` panics (verified: `at=9999` with an undeclared key returns cleanly; `at=len+2` and `at=0` with a declared key panic).
  OLD: `// an at outside the result, which is a fixture that could not have been staged.`
  NEW: `// outside the result panics, which is a fixture that could not have been staged.`
  (the preceding line changes from `// consumer pinning ...  has to choose N. It panics on` to `// consumer pinning ...  has to choose N. Splicing`)

NOTES:
- Fixture bytes independently verified UNCHANGED, far beyond the executor's dump: HEAD and HEAD+patch trees both dumped `validThemeSource`, `themeKeyLines`, `sourceMissingTokens` for all 19 tokens plus a 3-key combo, `sourceBadColours` for all 19 plus a 2-override combo, `sourceDuplicateKeyAt` for EVERY legal (key, line) pair (~190 forms), and both open-coded inline sites. 4858 lines each, `diff` clean.
- Fingerprint matrix run at SEVEN probes per test, not four, with a no-op control: identical-bytes rewrite, leaked sibling in the themes dir, chmod of the themes dir, create-then-delete inside it, leaked sibling AT THE CONFIG ROOT (the 14-7 narrowing class — proves the snapshot root is still `root`), chmod of a staged `.theme` file, and symlink swap of a staged file. All 14 CAUGHT; both controls pass. Coverage is not narrowed.
- The three `cmd` helpers' moved vacuity guards DO fire — driving each with `"no.such.token"` fatals in all three.
- `themetest` remains test-only; both new exports have real consumers.
- The positional `WithDuplicateKeyAt(lines, key, at)` rather than the task's suggested `WithDuplicateKey(lines, key)` is a justified deviation (the loader's detail carries the line number, so the consumer must choose it) and the task said "e.g.".
- Out of scope, flagged for a future sweep: `internal/theme/builtins_tokyo_night_day_test.go:283` still open-codes a join+WriteFile; folding it in would change the staged mode (0o644 → themetest's 0o600).
- Minor, not now: `useThemesDir` points `PORTAL_THEMES_DIR` at a `t.TempDir()` directly while `seedThemesDir` points it at `t.TempDir()/themes`, so the two sibling helpers stage at different depths and `seedThemesDir` restates the `t.Setenv` that `useThemesDir` owns.
- Latent sharp edge, do not change: `TestWithDuplicateKeyAt_ProducesTheBadSyntaxRejection` hardcodes `at = 12`, legal only while the vocabulary has ≥11 tokens. It mirrors the `cmd` consumers' pinned 12.
