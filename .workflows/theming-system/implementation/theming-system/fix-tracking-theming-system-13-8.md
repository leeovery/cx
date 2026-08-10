## Attempt 1

ISSUES:
- `cmd/open_theme_nomination_test.go:196-213` — the new `assembledThemeAdvisories` is a second, untracked entry point into doctor's theme scan, opening a hole in `TestOpenExecPath_DoesNoThemeWork`'s source guard. `doctor_theme.go` is exempt in full, so the guard's only cover for that file is the `local` map, which tracks `collectThemeAdvisories` alone. The reviewer verified this in a scratch copy of the repo: injecting `_ = assembledThemeAdvisories(&DoctorDeps{})` into `emitResolveDecision` in `cmd/open.go` leaves the guard **green**, whereas the same injection with `collectThemeAdvisories` fails with `open.go: emitResolveDecision calls collectThemeAdvisories — theme work belongs to TUI construction`. The runtime half cannot back this up either (doctor's loader is `log.Discard()`), so the exec path is now one line of future wiring away from an unguarded themes-directory read.
  FIX: add `"assembledThemeAdvisories": true` to the `local` map in `themeCallSites` (`cmd/open_theme_nomination_test.go:196`), beside `collectThemeAdvisories`, and extend that entry's note to say the scan now has two entry points and both are tracked. Do **not** add it to `allowed`. The reviewer verified in the scratch copy that this makes the injected call fail the guard, and that `parseCmdFiles` skips `_test.go` files so the legitimate test call sites in `doctor_theme_test.go` / `doctor_fix_theme_test.go` / `doctor_theme_union_test.go` are unaffected.
  ALTERNATIVE: drop `assembledThemeAdvisories` entirely — inline the assembly back into `collectThemeAdvisories` and have the union/scan tests compose `assembleThemeAdvisories(scanThemesDirectory(loader, dir), persistedThemeAdvisories(deps, loader))` themselves. That removes the second entry point rather than tracking it, but duplicates the shared-loader construction across every test helper and lets that construction drift from production. The one-line map addition is recommended.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/doctor_theme_union_test.go:38-41` — "a producer outside this file" is inaccurate: `themeAdvisory` is package-visible, the producers live in `cmd/doctor_theme.go` (not this test file), and what the signature actually excludes is a producer of the renderer's `advisory` class.
  OLD:
  ```
  // A line-only advisory cannot reach the union: the assembly takes the
  // theme-local record, so a producer outside this file has no way to enter the
  // dedup — nor any identity field to leave unset. Widening the signature stops
  // this file compiling.
  ```
  NEW:
  ```
  // A line-only advisory cannot reach the union: the assembly takes the
  // theme-local record, so a producer of the renderer's advisory class has no way
  // into the dedup — nor any identity field to leave unset. Widening the signature
  // stops this file compiling.
  ```
- `cmd/doctor_theme_union_test.go:129-134` — the edit left a broken reflow (a four-word stub line mid-sentence). Indentation is two tabs per line.
  OLD:
  ```
  		// The union's rank is DECLARED on the themeAdvisory record and must be read
  		// from there, not inferred from which
  		// producer's slice a line arrived in — otherwise the declared identity is
  		// unread, and a test asserting `fromPrefs: true` on a persisted line would
  		// be asserting a field nothing consults. Only a hand-built value can
  		// separate the two, since the real producer always sets it.
  ```
  NEW:
  ```
  		// The union's rank is DECLARED on the themeAdvisory record and must be read
  		// from there, not inferred from which producer's slice a line arrived in —
  		// otherwise the declared identity is unread, and a test asserting
  		// `fromPrefs: true` on a persisted line would be asserting a field nothing
  		// consults. Only a hand-built value can separate the two, since the real
  		// producer always sets it.
  ```

NOTES:
- Rendered output confirmed unchanged: the conversion in `collectThemeAdvisories` is a 1:1 order-preserving copy of `line`; producers' copy constants and format strings are untouched; length (and hence the advisory-count suffix) is preserved; nil-vs-empty unchanged.
- The removed subtest lost no claim — the fields it discriminated on no longer exist, so its claim is now structurally unfalsifiable, and the new structural test pins the shape while two existing tests still pin "line reaches the render verbatim".
- The compile-time seam guard is weakly meaningful rather than decorative: largely redundant with the file's hand-built call sites, but it survives their deletion and mirrors an existing convention in the package. Acceptable.
- `assembleThemeAdvisories` vs `assembledThemeAdvisories` differ by one character and sit 34 lines apart in the same file, with both called from the same test files. Worth considering a distinct name (e.g. `themeAdvisoryUnion(deps)`) — non-blocking, use your judgement.
- `TestThemeAdvisoryUnion_OrderIsDeterministic`'s repeat-run subtest and `TestThemeAdvisoryUnion_CountMatchesRenderedLines` now assert line content on a separately collected assembled slice from the one they render. Coverage still holds via the new union test, but the coupling is weaker than before.
- `TestAdvisories_CarryOnlyTheRenderedLine` pins field count and name but not the field's type. Marginal.
