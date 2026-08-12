TASK: theming-system-14-13 — Report The Bad-Name Extension Cause Only When The Slug Portion Is Actually Legal

ACCEPTANCE CRITERIA:
- `Nord.THEME` and `My Theme.THEME` report `BadNameSlug`; `nord.THEME` and `sunset.Theme` still report `BadNameExtension`.
- A name with no `.theme`-shaped suffix still reports `BadNameExtension`.
- No slug is minted from a non-exact extension, and nothing is normalised or returned lowercased.
- The rejection ladder order and the single `bad name` reason class are unchanged.
- `go test ./internal/theme ./cmd` passes.

STATUS: complete

SPEC CONTEXT:
Spec §14A (specification.md:1845-1846) fixes the two doctor advisory frames: `⚠ theme file <filename>: slug must be lowercase letters, digits and hyphens` and `⚠ theme file <filename>: extension must be lowercase .theme`, the latter justified as "a distinct message **because the slug portion is already legal**, and sending the user to fix the one thing that is fine is exactly the misdirection §9.4 and §12.1 discriminate against elsewhere". §6.2 (line 440) keeps `bad name` as ONE reason class with causes discriminated only by doctor/export, and the ladder note (lines 449-450) requires the filename rung to precede `reserved name` and any file read. This task makes the §14A premise true rather than assumed: the extension line is emitted only where the stem has actually been checked against `ValidSlug`.

IMPLEMENTATION:
- Status: Implemented (matches all acceptance criteria)
- Location:
  - `internal/theme/name.go:65-75` — `SlugFromFilename` still cuts the EXACT lowercase suffix first (`strings.CutSuffix`), but the non-exact branch now defers the cause to `misCasedExtensionCause(base)` instead of hardcoding `BadNameExtension`.
  - `internal/theme/name.go:77-90` — `misCasedExtensionCause`: too-short base → `BadNameExtension`; last-`len(".theme")`-bytes not `EqualFold` `.theme` → `BadNameExtension`; `.theme`-shaped suffix over a stem failing `ValidSlug` → `BadNameSlug`; otherwise `BadNameExtension`.
  - `internal/theme/name.go:94-96` — `badName` remains the single rejection constructor and `ReasonBadName` the single reason class (verified: the only other producers, `resolve.go:29` and `union.go:198`, route through it too).
  - `cmd/doctor_theme.go:209-214` (`badNameAdvisoryLine`), the two advisory formats (`cmd/doctor_theme.go:16-17`) and the panel row's reason label are untouched, as item 5 required — they render whichever cause they are handed.
- Notes:
  - No-normalisation property holds: the fold-stripped stem in `misCasedExtensionCause` is a judgement only — it is never returned, and the function returns a `BadNameCause` rather than a string, so a non-exact extension structurally cannot mint a slug. `SlugFromFilename` returns `""` on every rejection path.
  - Ladder order intact: `Loader.LoadFile` (`internal/theme/load.go:74-89`) still runs name → reserved → read → parse, and `isReserved` (`load.go:117-120`) is still exact map equality over the built-in slug set. `Nord.theme` / `Nord.THEME` therefore still yield no slug and can never collide with the built-in `nord`.
  - Edge behaviour is coherent across both entry points: `.theme` (exact, empty stem) and `.THEME` (fold-shaped, empty stem) both give `BadNameSlug`; `nord.theme.bak` / `nordtheme` / `nord.txt` / `nord` give `BadNameExtension`. Byte-slicing at a fixed suffix length is safe for multi-byte names (a mid-rune cut simply fails the `EqualFold` and falls to the extension cause).
  - Downstream consistency verified — no stale expectation anywhere: `internal/tui` and `internal/capture` build `theme.Entry` values by hand (`fixtures.go:536` pairs an exact-extension illegal stem with `BadNameSlug`, correct under the new rule), `cmd/capturetool/main_test.go:277-300` asserts only the reason, and `docs/theming.md:231-234` illustrates both lines with single-fault names (`Nord.theme` → slug, `nord.THEME` → extension), which stay correct.
  - Deliberate later supersession, not drift: commits `25626754` / `915e7fcb` (the plan's comment-standard sweep) stripped the rationale paragraph this task's Do-item 4 added to `SlugFromFilename`'s doc comment, and the explanatory comments it added to four tests. The rule itself survives in compressed form on the `BadNameExtension` const (`name.go:25-27`) and the helper (`name.go:77`), and the test names (`…_ExtensionCauseOnlyWhenStemIsLegal`, `…_DoublyIllegalNameRendersTheSlugLine`) carry the intent. Judged against the amended intent this is fine — with the one accuracy note below.

TESTS:
- Status: Adequate (one small redundancy, one uncovered corner)
- Coverage:
  - `internal/theme/name_test.go:134-178` `TestSlugFromFilename_ExtensionCauseOnlyWhenStemIsLegal` is the required table and covers every listed shape: legal stem + wrong-case extension (`nord.THEME`, `sunset.Theme` → extension), illegal stem + wrong-case extension (`Nord.THEME`, `My Theme.THEME` → slug), illegal stem + exact extension (`My Theme.theme` → slug), no `.theme`-shaped suffix (`nord.txt` → extension), legal stem + exact extension (slug returned). It also asserts the empty slug and the `ReasonBadName` class on every rejection row, so the no-mint property is pinned per case.
  - `cmd/doctor_theme_test.go:515-533` `TestThemeAdvisories_DoublyIllegalNameRendersTheSlugLine` — the required doctor test: `Nord.THEME` and `My Theme.THEME` render the slug line end-to-end through a real themes dir, while `…_BadNameExtensionFrame` (486-513) still pins the extension line for single-fault names.
  - `internal/theme/reserved_test.go:63-87` plus `caseVariants` (208-226) — the required reserved-name test: every built-in slug's mis-cased stem, with BOTH extensions, is `bad name` with the slug cause and no derived slug, while `<slug>.THEME` keeps the extension cause. This is what proves exact-equality reservation is untouched.
  - `internal/theme/enumerate_test.go:172-196` was correctly re-pointed (`Nord.THEME` → slug cause, `nord.Theme` → extension cause) and still asserts the entry is visible with an empty slug.
  - These fail if the feature breaks: reverting `name.go:68` to a hardcoded `BadNameExtension` breaks the name table, the enumerate table, the reserved variants and the doctor test.
- Notes:
  - `.THEME` (a fold-shaped suffix over an EMPTY stem) is not pinned anywhere; its exact-extension twin `.theme` is (`name_test.go:256-268`). That path returns `BadNameSlug` and is worth one row.
  - `TestSlugFromFilename_RejectsNonLowercaseExtension` (`name_test.go:100-132`) is now largely subsumed by the new table — same assertion body, and 3 of its 6 rows are duplicates or near-duplicates. Mild over-testing introduced by this task; see the note below.

CODE QUALITY:
- Project conventions: Followed. Leaf-package discipline intact (`internal/theme/name.go` gains no imports beyond the `strings` it already had); no raw hex, no logging added; the change stays in the loader's pure-judgement layer with no seam or path resolution, per the `theme` package contract in CLAUDE.md.
- SOLID principles: Good. One added unexported helper with a single responsibility (say which cause a non-exact extension reports); `badName` remains the single construction point, so the reason class cannot drift from the cause.
- Complexity: Low. Three guard clauses, no loops, no allocation; the caller gained one function call.
- Modern idioms: Yes — `strings.CutSuffix` for the exact path, `strings.EqualFold` for the judgement-only fold compare. There is no stdlib fold-aware `CutSuffix`, so the length-based split is the reasonable idiom.
- Readability: Good. The exact/inexact split reads cleanly at the call site, and the helper's "never returned, never a slug" comment names the safety property that matters most here. Two small wording/naming nits below.
- Issues: None material. The helper duplicates, by a different mechanism, the "is this extension `.theme`-shaped" question `enumerate.go:71-73` asks via `filepath.Ext` + `EqualFold`. They agree for every realistic name and ask subtly different questions (fixed-length suffix vs last dot-extension), so a shared helper would likely obscure more than it saves — noted, not raised.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/name.go:25-27 — the `BadNameExtension` doc admits a reading the code falsifies: `.THEME` is a `.theme`-shaped suffix over an empty stem (which "leaves nothing to judge") yet returns `BadNameSlug`. Replace the three comment lines with: "BadNameExtension — the extension is not exactly lowercase `.theme`, over a stem that is a legal slug, or over a name with no `.theme`-shaped suffix to leave a stem at all. A stem that is illegal too is BadNameSlug."
- [do-now] internal/theme/name.go:78 (declaration) and :68 (call) — rename `misCasedExtensionCause` to `inexactExtensionCause`: it also decides names whose extension is not mis-cased at all but simply absent or different (`nord`, `nordtheme`, `nord.txt`), which the current name overclaims.
- [do-now] internal/theme/name_test.go:141-147 — add a row `{name: "empty stem, shouted extension", base: ".THEME", wantCause: theme.BadNameSlug}` to `TestSlugFromFilename_ExtensionCauseOnlyWhenStemIsLegal`, pinning the fold-stripped empty-stem corner (its exact-extension twin is pinned at name_test.go:256).
- [quickfix] internal/theme/name_test.go:100-132 — `TestSlugFromFilename_RejectsNonLowercaseExtension` now has a byte-identical assertion body to `TestSlugFromFilename_ExtensionCauseOnlyWhenStemIsLegal` and overlapping rows (`nord.THEME`, `Nord.THEME`, and `nord.Theme` ≈ `sunset.Theme`). Fold its three distinct rows (`nord.theme.bak`, `nord`, `nordtheme`) into the newer table and delete the older test, leaving one table over `SlugFromFilename`'s cause selection.
