TASK: theming-system-12-11 — Export The Theme-File Extension And Drop The Restatements (tick-8aa850, severity low, source: duplication)

ACCEPTANCE CRITERIA:
- `theme.FileExtension` is exported and documented.
- No `".theme"` literal remains in `cmd/capturetool` or `internal/capture`.
- Every fixture filename that pairs with a slug constant is composed from that constant plus the exported extension.
- `internal/theme`'s own comparison behaviour is unchanged (exact bytes, no case folding).

STATUS: complete

SPEC CONTEXT:
Specification §5.3 ("Extension — `.theme`") makes the extension part of the published theme-file contract: theme files carry `.theme`, and §5.6/§9 (specification.md:389-391) pin the two-sided rule the constant's comment now carries — enumeration matches the extension **case-insensitively** so a mis-cased file is *visible*, while acceptance takes **only the exact lowercase** `.theme`, which is what preserves §5.1's structural-uniqueness claim (a non-exact extension never mints a slug, so `foo.THEME` beside `foo.theme` cannot produce two rows with one label). §5.4's no-shadowing safety property depends on the same exactness ("reject, never normalise", specification.md:347-349). The by-name construction path (§8.4) looks for `<slug>.theme` and nothing else, and §12.1's export workflow prints the extension to users (`… > ~/.config/portal/themes/nord-lee.theme`, specification.md:1413) — so the extension genuinely is a user-facing contract with, previously, more than one home in the code.

IMPLEMENTATION:
- Status: Implemented (commit bbf8e392, later comment-audited by 25626754/915e7fcb — the constant, its call sites and the test are intact).
- Location:
  - `internal/theme/name.go:10-13` — `const FileExtension = ".theme"` with the exact-bytes/never-case-fold note moved onto it (Do step 1). The package's own consumers read it: `name.go:66` (`strings.CutSuffix(base, FileExtension)`), `name.go:79-83` (the mis-cased-extension cause helper), `enumerate.go:72` (`strings.EqualFold(filepath.Ext(name), FileExtension)`), `builtins.go:31,50`, `resolve.go:57`. No unexported `themeExtension` alias survives, so there is exactly one spelling in the owning package.
  - `cmd/capturetool/main.go:122` — `isThemePath` now reads `theme.FileExtension`; the local `themeFileExtension` constant and its "local restatement …" comment are deleted (Do step 2, verified against the commit diff).
  - `internal/capture/fixtures.go:426, 531, 535, 539, 618` — all twelve former inline literals are composed as `<slug> + theme.FileExtension` (or `"<literal-name>" + theme.FileExtension` for the two entries that deliberately have no slug: `"aurora-glow"` and the bad-name `"My Gorgeous Midnight Palette"`). Later tasks (13-6, 17-6, 17-12) re-derived several fixture unions from their entries, which is why five composition sites now cover what was twelve — that is the amended intent, not a lost replacement.
  - `internal/theme/theme_test.go:171` — `"FileExtension"` added to the pinned `wantExports` public-surface list, so the export is now guarded rather than incidental.
- Notes:
  - No `".theme"` literal remains anywhere in `cmd/capturetool` or `internal/capture` production code (`fixtures.go`, `fakes.go`, `harness.go`, `swatch.go`, `theme_fake.go`, `main.go`); the only production literal repo-wide is `name.go:13` itself. The surviving `.theme` mentions in `cmd/capturetool/main.go:9,103,159` are prose/user-facing copy, not compositions.
  - `cmd/doctor_theme.go:17-18` keeps `.theme` inside two format strings — that is §14A's verbatim pinned copy (specification.md:1846-1847), not a constant restatement, and substituting a constant into spec-verified copy would be a regression. Correctly left alone.
  - Comparison semantics are byte-identical to before: `CutSuffix` against the same value, `EqualFold` still confined to enumeration. Pure constant move, as Do step 4 required.

TESTS:
- Status: Adequate (one small redundancy — see notes).
- Coverage:
  - `internal/theme/name_test.go:180-199` `TestFileExtension_IsWhatSlugFromFilenameAccepts` — pins the composed-with-the-constant filename round-tripping to its slug, and `strings.ToUpper(theme.FileExtension)` rejecting as `ReasonBadName`/`BadNameExtension`. This is the test the task asked for.
  - The pre-existing literal-based suites remain the independent restatement that would catch the constant's *value* changing: `name_test.go:80-83` (`"nord.theme"` → slug), `name_test.go:106-111` (`"nord.THEME"`, `"nord.Theme"`, `"nord.theme.bak"`, no-extension), `name_test.go:134-148` (cause discrimination), `name_test.go:229-268` (never-normalise, empty stem). Deliberately keeping literals on the assertion side is the right call — a test composed entirely from the constant would pass even if the constant were wrong.
  - `internal/theme/theme_test.go:171` (`wantExports`) is the structural guard that the export is intentional and cannot be silently unexported again.
  - Consumer-side coverage is unchanged and needed no regeneration: `internal/capture/theme_panel_fixture_test.go:105-110` re-derives every panel fixture's union from its declared entries, and `:118-139` asserts the invalid-row identities (including `"My Gorgeous Midnight Palette" + theme.FileExtension`), so a mis-composed fixture filename fails rather than silently renders. `cmd/capturetool`'s path-vs-slug discrimination is exercised by its own tests; no reference PNG regeneration is implied since no rendered string changed.
- Notes: `TestFileExtension_IsWhatSlugFromFilenameAccepts`'s two assertions are behaviourally covered already by `TestSlugFromFilename_DerivesStem` and `TestSlugFromFilename_RejectsNonLowercaseExtension`; its only added signal is "the call site uses the constant". The task's Tests bullet asked to extend the existing name test rather than add a parallel one, and a standalone function was added instead (in the right file, at least). Low-value duplication, not a defect — noted below.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays the owner of the file-format vocabulary; nothing new imports it that did not already; the test-only/production import boundaries are untouched; no logging or config-discovery crept into the constant's package.
- SOLID principles: Good. Single-source-of-truth move — the package that parses the format now owns the string every other surface recognises it by.
- Complexity: Low. Identifier substitution only; no branch, allocation or control-flow change.
- Modern idioms: Yes. `strings.CutSuffix` / `strings.EqualFold` / `filepath.Ext` usage is unchanged and idiomatic; `strings.ToUpper(theme.FileExtension)` in the test derives the negative case rather than hardcoding a second literal.
- Readability: Good. `themePanelDirEntry(themePanelBrokenSlug+theme.FileExtension, themePanelBrokenSlug)` now makes the filename/slug pairing self-evidently consistent — the exact class of silent disagreement the task cited.
- Issues: One local doc-comment inconsistency (below). Also note the task commit carried a one-line unrelated comment typo fix at `internal/theme/theme_test.go:66-70` ("the The canonical" → "the canonical") — trivial, harmless, no action.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/name.go:10-13 — the doc comment on the newly exported `FileExtension` does not open with the identifier, while every other exported symbol in the same file does (`ValidSlug reports…`, `StripControl removes…`, `SlugFromFilename derives…`, `BadNameCause records…`, `BadNameNone is the zero value…`). Replace the three comment lines with: `// FileExtension is the one extension a theme file carries. Acceptance compares` / `// it by exact bytes and never case-folds; enumeration alone matches it` / `// case-insensitively, so a mis-cased file is visible before it is rejected.` — same information, godoc-conventional prefix restored, no restatement added.
- [quickfix] internal/theme/name_test.go:180-199 — `TestFileExtension_IsWhatSlugFromFilenameAccepts` re-asserts behaviour already table-covered at `name_test.go:80` (`"nord.theme"` → `"nord"`) and `name_test.go:106` (`"nord.THEME"` → `BadNameExtension`). Fold it into those tables as two cases — `{name: "composed from FileExtension", base: "nord-lee" + theme.FileExtension, want: "nord-lee"}` in `TestSlugFromFilename_DerivesStem` and `{name: "uppercased FileExtension", base: "nord-lee" + strings.ToUpper(theme.FileExtension), wantCause: theme.BadNameExtension}` in `TestSlugFromFilename_RejectsNonLowercaseExtension` — and delete the standalone function, which is what the task's Tests bullet ("extend the existing name test rather than adding a parallel one") asked for.
