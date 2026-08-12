TASK: theming-system-7-4 — The Filename-Reason Frames: Two `bad name` Messages And The Reserved-Name Line

ACCEPTANCE CRITERIA:
- `Nord.theme` produces exactly `⚠ theme file Nord.theme: slug must be lowercase letters, digits and hyphens`.
- `nord_lee.theme`, `nord lee.theme`, `-nord.theme` and `.theme` each produce the same slug-cause frame, labelled by their own filename.
- `nord.THEME` and `nord.Theme` each produce exactly `⚠ theme file <filename>: extension must be lowercase .theme` — the extension cause, never the slug cause.
- A `themes/nord.theme` alongside the built-in `nord` produces exactly `⚠ theme file nord.theme: nord is a built-in — rename it (e.g. nord-mine.theme)`.
- Every member of `theme.BuiltinSlugs()` produces the reserved-name line when a colliding file is dropped in; the test names no theme.
- A `bad name` file that is also unreadable and also has a bad hex produces exactly one line, the filename one.
- A `reserved name` file whose contents are perfectly valid still produces its line.
- A `bad name` advisory carries an empty `slug`; a `reserved name` advisory carries `Entry.Slug`; both carry `fromPrefs == false`.
- No filename-reason line uses the `⚠ theme <slug>:` frame, and no content-reason line uses the `⚠ theme file <filename>:` frame.
- The three frames are single-sourced in `cmd/doctor_theme.go`; no format string for them exists elsewhere.

STATUS: complete

SPEC CONTEXT:
§14A (specification.md:1845-1847) pins all three lines verbatim; the implementation's three const format strings match those rows byte-for-byte, including the `(e.g. <slug>-mine.theme)` suffix and the em-dash in `is a built-in — rename it`. §6.2 (line 440) states the one-reason-class-for-the-panel rule and names the two differing line frames (`⚠ theme file <filename>: …` versus `⚠ theme <slug> …`) as what carries the input class; doctor is the surface with the width to name which cause. The §6.2 ladder's rung ordering (filename → reserved → read → content) is enforced upstream in `internal/theme/load.go:74-89`: `SlugFromFilename` rejects first, `isReserved` second, and `os.ReadFile` only runs after both — so a `bad name` or `reserved name` file structurally cannot also report `unreadable` or a content reason. `internal/theme/enumerate.go:11-19` documents the matching invariant "Slug is empty exactly when the rejection is `bad name`", which is what makes task 7-6's non-collision rule structural.

IMPLEMENTATION:
- Status: Implemented (with one later in-plan amendment, see Notes)
- Location:
  - `cmd/doctor_theme.go:16-18` — the three single-sourced format consts.
  - `cmd/doctor_theme.go:188-199` — the `ReasonBadName` / `ReasonReservedName` arms of `themeFileAdvisory`, with the identity fields set alongside each line (`slug: ""` / `slug: entry.Slug`, `fromPrefs: false` in both).
  - `cmd/doctor_theme.go:209-214` — `badNameAdvisoryLine`, selecting on `theme.BadNameExtension` and falling through to the slug message.
  - Upstream contracts relied on: `internal/theme/name.go:15-28,65-96` (`BadNameCause`, `SlugFromFilename`, `misCasedExtensionCause`), `internal/theme/enumerate.go:84-100` (`Entry.Filename` = `filepath.Base(path)`, never the full path; `Slug` re-derived and empty on `bad name`), `internal/theme/load.go:74-90` + `:36` (reserved set derived from `builtinSlugSet()` → `BuiltinSlugs()` → the embedded `builtins/*.theme` directory, `internal/theme/builtins.go:38-55`).
- Notes:
  - `Entry.Filename` is used for all three lines, never `Entry.Path` — matches §14A's `<filename>` placeholder.
  - The reserved arm reads `entry.Slug`, which is provably non-empty on that reason (a reserved rejection is only reachable after `SlugFromFilename` succeeded).
  - The `bad name` arm states `slug: ""` literally rather than copying `entry.Slug`; the surviving doc comment at `cmd/doctor_theme.go:172-175` records why, and `TestThemeAdvisories_BadNameSlugIsStatedNotCopied` pins it with a hand-built entry that carries a non-empty slug.
  - Reserved-set derivation needs no edit for a future built-in: `BuiltinSlugs()` walks the embed FS, so adding a `.theme` file to `builtins/` extends the reserved set and the guard test's loop simultaneously.
  - **Later in-plan amendments (not drift):** task 14-13 (`ff3e81d0`) refined the extension/slug cause boundary so a doubly-illegal name (`Nord.THEME`) reports the *slug* cause — consistent with this task's stated rationale that the extension message may only be claimed when the stem is already legal; `misCasedExtensionCause` implements it and `TestThemeAdvisories_DoublyIllegalNameRendersTheSlugLine` pins it. Task 15-1 (`381e57ca`) moved the producer onto a single retained `theme.Enumeration`, and 13-8 (`16da37e7`) moved the dedup identity onto the file-local `themeAdvisory` record — both change plumbing above this task's arms, not the frames.
  - **Doc-comment requirements superseded:** the task asked for three rationale doc comments; the original commit (`abeaedbe`) contained all three, and the plan's own comment-quality remediation (11-3 `e30939b2`, 12-7 `c69101ca`, plus the `chore(comments)` sweeps `a4bc7bd5` / `915e7fcb`) deliberately stripped spec-citation and design-argument comments repo-wide. The load-bearing residue survives (`:172-175` empty-slug-is-stated; `:207-208` `BadNameNone` unreachable). Judged against the amended intent, not a finding.

TESTS:
- Status: Adequate
- Coverage (all in `cmd/doctor_theme_test.go`, unit lane, no build tag):
  - `TestThemeAdvisories_BadNameSlugFrame` (:473) — table of five: `Nord.theme`, `nord_lee.theme`, `nord lee.theme`, `-nord.theme`, `.theme`, each in its own temp dir, exact line equality. Covers criteria 1 and 2.
  - `TestThemeAdvisories_BadNameExtensionFrame` (:497) — `nord.THEME` and `nord.Theme`, one dir each (the comment at :495 records why: the names fold on a case-insensitive filesystem — a real macOS hazard, correctly handled). Covers criterion 3.
  - `TestThemeAdvisories_FilenameReasonsLabelledByFilename` (:538) — five files in one dir, `slices.Equal` over the full ordered line list mixing content reasons (`⚠ theme <slug>:`) and filename reasons (`⚠ theme file <filename>:`). This is the criterion-9 cross-check in both directions, and it also pins ordering.
  - `TestThemeAdvisories_ReservedNameFrame` (:561) — exact line, plus a negative assertion that the terse `reserved name` reason label is absent (proving it does not follow the generic `<reason> — <detail>` frame), plus `slug == "nord"` and `!fromPrefs`. Covers criterion 4 and half of 8.
  - `TestThemeAdvisories_ReservedSetIsTheEmbeddedSet` (:581) — loops `theme.BuiltinSlugs()`, names no theme, fails loudly on an empty set so the assertions can never be vacuous, asserts one line per slug and the exact text of each. Covers criterion 5.
  - `TestThemeAdvisories_BadNameNeverReportsContent` (:606) — `Bad_Name.theme` with a bad hex *and* a duplicate key *and* mode 0000, asserting exactly one advisory, the slug-cause line, and that none of the four content/read reason strings appear. Covers criterion 6 and is a genuine rung-1 negative.
  - `TestThemeAdvisories_BadNameCarriesNoSlug` (:624) — both causes, `slug == ""` and `!fromPrefs`. Completes criterion 8.
  - `TestThemeAdvisories_ReservedNameDecidedBeforeRead` (:678) — valid contents *and* a mode-0000 file, both yielding the reserved line. Covers criterion 7 and pins rung 2's before-any-read property.
  - `TestThemeAdvisories_FilenameFramesAreSingleSourced` (:702) — AST literal scan asserting each of the three copy fragments is declared at exactly one site, `doctor_theme.go`. Covers criterion 10 within the `cmd` package (see note).
  - `requireBuiltinSlug` (:461) guards every `nord`-based fixture against the embedded set, so a future rename of the built-in fails the fixture loudly instead of silently making the test prove nothing. Good defensive design.
- Notes: Test names match the plan's list exactly. No over-testing found — the apparent overlaps are each load-bearing on a distinct property (`BadNameCarriesNoSlug` = the enumerated path, `BadNameSlugIsStatedNotCopied` = the stated-not-copied property via a hand-built entry that only the unit boundary can express). Fixture helpers (`themesDirWith`, `requireOneAdvisory`, `advisoryLines`, `themetest.DenyRead`, `validThemeSource`, `sourceBadColours`, `sourceMissingTokens`, `duplicateKeyLines`) all resolve. No `t.Parallel()`, per the project rule.

CODE QUALITY:
- Project conventions: Followed. Copy lives in named consts at the top of the owning file; the producer stays in `cmd` while `internal/theme` keeps the classification; the loader is constructed silent (`theme.NewSilentLoader`), honouring the "the `theme` log component records use, never diagnosis" rule; no raw formatting scattered at call sites.
- SOLID principles: Good. `themeFileAdvisory` maps reason → line/identity and nothing else; `badNameAdvisoryLine` is a single-purpose cause selector; the frames are data, the selection is behaviour.
- Complexity: Low. One switch with four arms plus a two-branch helper; no nesting beyond one level.
- Modern idioms: Yes. Typed `Reason` / `BadNameCause` discriminators rather than string sniffing; the switch is exhaustive over the owned reasons with the unowned one visibly defaulted.
- Readability: Good. The `default` arm's comment explains why `not found` is skipped rather than leaving a silent hole; the const block's comment explains that lines carry their own `⚠ ` because the renderer only indents.
- Comment accuracy: Verified against the code. `:172-175` ("stated here, not copied") matches `slug: ""` at :191. `:207-208` ("BadNameNone is unreachable: both causes are set by the constructor that builds this reason") is true — `internal/theme/name.go:94-96` is the only production constructor of a `ReasonBadName` rejection (the only other site, `internal/capture/fixtures.go:536`, is a fixture that sets `BadNameSlug` explicitly). `:200-202` ("`not found` belongs to the persisted producer") matches `LoadFile`'s documented contract. No process-artifact references, no restated code.
- Security: N/A — read-only diagnosis; filenames are echoed from a directory the user owns, and `theme.StripControl` handling belongs to the surfaces that echo persisted/CLI values, not to this enumerated-filename path.
- Performance: N/A — one directory enumeration, reused across both producers.
- Issues: none.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/doctor_theme_test.go:702-718 — `TestThemeAdvisories_FilenameFramesAreSingleSourced` proves single-sourcing only inside the `cmd` package: `cmdLiteralSites` → `parsePackageFilesByName` (cmd/open_theme_nomination_test.go:243) → `sourceguardtest.PackageGoFiles(".", false)`, a single-directory walk. The acceptance criterion is "no format string for them exists elsewhere". Switch the enumeration to `sourceguardtest.GoSourceFiles` (the repo-wide walk) with the site map keyed by relative path, so a copy of any of the three fragments in `internal/tui` or `internal/theme` fails the guard. A repo-wide grep is clean today, so this is a guard-scope gap rather than a live duplicate.
- [quickfix] cmd/doctor_theme.go:18,196 — `reservedNameAdvisoryFormat` spends two separate `%s` verbs on the same value, forcing the call site to pass `entry.Slug` twice (`fmt.Sprintf(reservedNameAdvisoryFormat, entry.Filename, entry.Slug, entry.Slug)`). Rewrite the const as `"⚠ theme file %[1]s: %[2]s is a built-in — rename it (e.g. %[2]s-mine.theme)"` and drop the duplicated argument, so the two occurrences cannot drift apart at the call site.
