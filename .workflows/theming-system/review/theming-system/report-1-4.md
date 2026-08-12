TASK: 1.4 — Derive A Slug From A Filename — Charset And Extension Rules, Reject Never Normalise (tick-a2aa9d / theming-system-1-4)

ACCEPTANCE CRITERIA:
- `nord.theme` → `nord`; `tokyo-night-day.theme` → `tokyo-night-day`; `a.theme` → `a`; `nord-.theme` → `nord-` (trailing hyphen legal).
- `.theme` (empty stem), `-nord.theme`, `nord_lee.theme`, `nord lee.theme`, `Nord.theme` → `bad name` cause slug.
- `Nord.THEME`, `nord.Theme`, `nord.THEME` → `bad name` cause extension, distinguishable from the slug cause.
- `nord.theme.bak` and `nord` (no extension) → `bad name` cause extension.
- `Nord.theme` never produces the slug `nord` — empty slug, rejection is the only result.
- A 300-character all-legal stem is accepted; no length bound anywhere in the file.
- `ValidSlug` false for `""`, `"../evil"`, `"-nord"`, `"Nord"`, `"nord lee"`, `"nord_lee"`, `"nord/evil"`; true for `"nord"`, `"a"`, `"a-b-c"`, `"tokyo-night-day"`, `"n0rd"`, `"nord-"`.

STATUS: complete

SPEC CONTEXT:
§5.2 fixes the slug charset at `^[a-z0-9][a-z0-9-]*$` and states the three anchor edges (empty illegal, leading hyphen illegal, trailing hyphen legal-but-pointless) plus "no length bound — §9.8's truncation is a display concern that must not silently become a validity rule", and "reject, never normalise" (lowercasing `Nord.theme` would shadow the built-in §5.4 protects, and keeps the reserved-name check exact string equality). §5.6 (spec line 389) splits enumeration from acceptance: the extension is matched case-insensitively for *enumeration* (so a mis-cased file is visible, not invisible) but only the exact lowercase `.theme` is *accepted*, which is what preserves structural slug uniqueness. §6.2 (line 440) pins ONE reason class `bad name` over three causes across two input classes, with the cause existing only so doctor/export can name which. §14A (lines 1845–1846) gives the two doctor frames, and justifies the extension frame as "a distinct message **because the slug portion is already legal**".

IMPLEMENTATION:
- Status: Implemented (one acceptance criterion intentionally superseded later in the same plan — see below).
- Location:
  - `internal/theme/name.go:13` `FileExtension` (exported by task 12-11, originally the unexported `themeExtension`).
  - `internal/theme/name.go:17-28` `BadNameCause` + `BadNameNone`/`BadNameSlug`/`BadNameExtension`.
  - `internal/theme/name.go:34-45` `ValidSlug` — byte-wise anchored scan, no regexp, no length bound.
  - `internal/theme/name.go:65-75` `SlugFromFilename` — `strings.CutSuffix` on the exact-byte extension, then `ValidSlug` on the stem.
  - `internal/theme/name.go:78-90` `misCasedExtensionCause` — the 14-13 cause refinement.
  - `internal/theme/name.go:94-96` `badName` constructor (single source of Reason↔cause pairing, no Detail).
  - `internal/theme/reason.go:27-39` `Rejection.BadNameCause` field.
- Notes:
  - **Deliberate later supersession, not drift**: the criterion "`Nord.THEME` → cause **extension**" was inverted by plan task 14-13 (`ff3e81d0` "report the bad-name extension cause only when the slug is legal"). `Nord.THEME` now reports cause **slug**, because §14A's extension frame asserts "the slug portion is already legal" — emitting it for a stem that is *also* illegal would send the user to fix the one thing that is fine, the exact misdirection §9.4/§12.1 discriminate against. Judged against the amended intent this is correct, and the remaining criteria (`nord.THEME`, `nord.Theme` → extension; `Nord.theme` → slug; `nord.theme.bak`, `nord` → extension) all hold verbatim (`name_test.go:106-111`, `146`).
  - `misCasedExtensionCause` byte-slicing is safe on every input I traced: `len(base) < len(FileExtension)` short-circuits (`nord`), a non-`.theme`-shaped tail fails `EqualFold` (`nord.txt`, `nord.theme.bak`, `nordtheme`), and a split landing mid-rune yields `RuneError` bytes that simply fail the fold — no panic, no false accept. `.THEME` (mis-cased extension *and* empty stem) reports the slug cause, consistent with `.theme`.
  - Reject-never-normalise holds package-wide: the only `ToLower`/`EqualFold` on theme identity are `enumerate.go:72` (deliberate looser *visibility* match, documented), `name.go:83` (cause discrimination only, after acceptance already failed) and `union.go:225` (display ordering). No path lowercases a stem into a slug.
  - The reuse contract survives the later comment-stripping passes as behaviour rather than prose: `ValidSlug` is the sole charset authority and is applied to non-file inputs at `resolve.go:28` (the by-name/CLI path, before any path component is composed) and `union.go:197` (the persisted `prefs.json` slug), with no second copy of the rule anywhere in the tree (grepped: no other `a-z0-9` slug pattern exists).
  - The cause is consumed exactly where the spec says it should be: `cmd/doctor_theme.go:209-214` switches on it to pick between the two §14A frames. Nothing renders the cause value itself.
  - `StripControl` (`name.go:52-59`) is a later task's addition to this file, not this task's scope.

TESTS:
- Status: Adequate (one redundancy, see notes).
- Coverage: `internal/theme/name_test.go` carries all eight planned test names plus the 14-13 and 12-11 additions. Charset accepts/rejects tables (`:10`, `:33`) cover every enumerated value including `../evil`, `nord/evil`, and a multi-byte rune the plan did not ask for; `:58` pins the no-length-bound criterion through *both* `ValidSlug` and `SlugFromFilename`; `:74` covers the four derive cases; `:100`/`:134` cover extension casing and the cause split; `:201` proves one reason class with two distinct causes *and* that neither equals `BadNameNone` while an unrelated rejection does; `:229` proves no case normalisation with an empty slug returned; `:256` covers the bare `.theme`; `:270` covers leading-hyphen/underscore/space stems. Every rejection assertion also checks the slug is empty, so a "normalise then reject" regression fails.
- Notes: These are behavioural (public API only, `package theme_test`), table-driven with named subtests, no mocks, no `t.Parallel()` — matching the project rule and the golang-testing skill. The only imbalance is duplication between `TestSlugFromFilename_RejectsNonLowercaseExtension` (`:100`) and `TestSlugFromFilename_ExtensionCauseOnlyWhenStemIsLegal` (`:134`): `nord.THEME`, `nord.Theme` and `Nord.THEME` are each asserted for the same reason/cause/empty-slug triple in both tables. `TestSlugFromFilename_CausesAreDistinct` overlaps on inputs too but earns its place — it asserts the cross-cause invariants, not the per-input mapping.

CODE QUALITY:
- Project conventions: Followed. No regexp for a byte-class check, byte-wise scan with two tiny predicates, `strings.CutSuffix` (modern stdlib), errors-as-values via the `*Rejection` package idiom, single constructor so Reason and cause cannot drift. Comments carry rationale rather than restating code, and the later comment-audit passes removed the spec-section citations the original task text asked for — correct for the repo's stated comment standard.
- SOLID principles: Good. `name.go` owns naming and nothing else; the cause is a discriminator returned to callers rather than copy composed here, so `cmd` owns rendering (doctor's two frames) and the package stays render-free.
- Complexity: Low. `ValidSlug` is one guard plus a loop; `SlugFromFilename` is two guards; the cause helper is three guards.
- Modern idioms: Yes.
- Readability: Good, with two small naming/comment frictions noted below.
- Security: The load-bearing property — `ValidSlug` runs before any path is composed (`resolve.go:28` precedes the `filepath.Join` at `:57`), so `../evil` from a hand-edited `prefs.json` is refused as `bad name` and never becomes a path component. Covered by test (`name_test.go:39`) and by the by-name resolve path.
- Performance: N/A — allocation-free scans.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/name.go:10-12 — the `FileExtension` comment claims "enumeration **alone** matches it case-insensitively", which `misCasedExtensionCause` at line 83 falsifies by case-folding in this same file. Replace with: "Acceptance compares this by exact bytes and never case-folds; enumeration matches it case-insensitively so a mis-cased file is visible before it is rejected, and the bad-name cause does the same to tell a mis-cased extension from an illegal stem."
- [do-now] internal/theme/name.go:78 — rename `misCasedExtensionCause` to `nonExactExtensionCause`: it fires for every non-exact suffix, including `nord.txt`, `nordtheme` and `nord` where nothing is mis-cased, and it can return `BadNameSlug`, so the current name states neither its trigger nor its result. One call site (line 68).
- [do-now] internal/theme/name.go:77 — the comment "The stem is judged here only — never returned, never a slug." reads as "only this function judges the stem", which line 70 falsifies. Replace with: "Reports the extension cause only when the stem would have been a legal slug — the extension frame tells the user the slug portion is fine, so it must not fire over a stem that is also illegal."
- [quickfix] internal/theme/name_test.go:100-178 — fold the three duplicated rows (`nord.THEME`, `nord.Theme`, `Nord.THEME`) out of `TestSlugFromFilename_RejectsNonLowercaseExtension` and keep the extension-casing mapping in the single `TestSlugFromFilename_ExtensionCauseOnlyWhenStemIsLegal` table, leaving the former with the non-`.theme`-shaped inputs (`nord.theme.bak`, `nord`, `nordtheme`) it uniquely covers.
- [do-now] internal/theme/name_test.go:270 — rename `TestSlugFromFilename_RejectsLeadingHyphenStem` to `TestSlugFromFilename_RejectsIllegalStems`; its table also covers the underscore and space cases, which the current name hides.
- [quickfix] internal/theme/name.go:52 — move `StripControl` below `SlugFromFilename` so the two slug functions and their helpers stay contiguous; it currently splits `ValidSlug` from its only in-file caller.
