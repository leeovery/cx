TASK: 13-10 — Rename the panel's theme seam to cover resolution as well as enumeration (`ThemeEnumerator` → `ThemeSource`, `DirEnumerator` → `DirThemeSource`)

ACCEPTANCE CRITERIA:
- The identifiers `ThemeEnumerator` and `DirEnumerator` appear nowhere in the repo, including comments, test names and string literals.
- The string-literal guard at cmd/open_theme_nomination_test.go still guards the renamed constructor and still fails when its invariant is broken.
- Method set, signatures and behaviour are unchanged.
- `go build ./... && go test ./...` pass; `golangci-lint run` is clean.

STATUS: complete

SPEC CONTEXT:
Spec §13.3 (specification.md:196, :1621, :1623, :1641, :1741) mandates the seam itself — "the panel's theme enumeration is behind an injectable seam", matching the `TmuxEnumerator` / `ScrollbackReader` idiom, returning the *finished §9.4 union* (not a directory listing) so `internal/theme` owns assembly and `internal/tui` never becomes a fourth emitter of the `theme` component. The spec names the interface `ThemeEnumerator`; this task is a Phase-13 architecture-analysis remediation that deliberately renames it, on the grounds that two of the seam's four methods (`Resolve`, `ResolveSlot`) are setting *resolution*, not enumeration. The spec constrains the seam's *responsibilities*, all of which are preserved; only the identifier diverges from the frozen spec text. Nothing in the spec's behaviour is touched.

IMPLEMENTATION:
- Status: Implemented (as amended by later phases)
- Location:
  - `internal/tui/theme_seams.go:11` — `type ThemeSource interface` (was `ThemeEnumerator`).
  - `internal/theme/dir_theme_source.go:6` — `type DirThemeSource struct` (file renamed from `internal/theme/enumerator.go`).
  - `cmd/theme_source.go:9-12` — `newThemeSource` (file renamed from `cmd/theme_enumerator.go`) returning `theme.DirThemeSource`.
  - `internal/tui/build.go:33,127-128` — `Deps.ThemeSource` field + `WithThemeSource` option; `internal/tui/model.go:545` — `WithThemeSource`; `internal/tui/theme_state.go:44` — `themeState.source` (was `themeState.enumerator`).
  - `cmd/open.go:458` (`themeSource tui.ThemeSource`), `:534` (`ThemeSource: cfg.themeSource`), `:640` (`themeSource: newThemeSource(themeLoader)`).
  - `internal/capture/theme_fake.go:11,20,22` — `fakeThemeSource` / `newFakeThemeSource`, with `var _ tui.ThemeSource = fakeThemeSource{}`.
  - Commit `4a8d45b2` (36 files); repo-wide grep for `ThemeEnumerator|DirEnumerator` over `*.go`, `CLAUDE.md`, `README.md` and `docs/` returns zero hits (only historical `.workflows/` planning/review prose retains the old names, which is the correct place for it).
- Notes:
  - AC 1 met. AC 3 met: the commit's non-comment diff is identifier substitution plus gofmt re-alignment only — verified by filtering the diff of `cmd/open.go`, `cmd/capturetool/main.go`, `internal/capture/*` for lines that are not rename/comment changes; nothing else appears.
  - AC 2 met: `cmd/open_theme_nomination_test.go:143` carries `"newThemeSource": true` in the `local` map and the explanatory comment at `:29` names `newThemeSource`. The guard is live — `newThemeSource` is called at `cmd/open.go:640` inside `openTUI`, which is in the `allowed` set, so a call from any other function trips `TestOpenExecPath_DoesNoThemeWork`. A second, easily-missed string-literal guard was also correctly updated: `internal/capture/theme_panel_fixture_test.go:489` matches the AST type name `"fakeThemeSource"`, and it is self-non-vacuous (`:381-384` fatals when the struct isn't found), so a future missed rename fails loudly rather than silently.
  - Step 8 honoured exactly: `CLAUDE.md` at `4a8d45b2^` did not name this seam (its only `Enumerator` mention is the unrelated preview `TmuxEnumerator`), so the commit correctly left it alone; the current `CLAUDE.md:176` names `theme_seams.go` (the `ThemeSource` seam), added by the later CLAUDE.md-inventory task.
  - Later supersession, not drift: task 15-9 changed `Resolve`'s second parameter from `theme.Setting` to `theme.RawKeys`, 17-4 narrowed `ResolveSlot` → `LoadSlot(…) error`, and two `chore(comments)` sweeps (`25626754`, `915e7fcb`, plus `e3fa1503`) shortened the doc blocks this task authored — so the "first line names BOTH responsibilities" wording (step 2) no longer survives verbatim at `internal/theme/dir_theme_source.go:3-5`. The seam's *name* now carries that job, which is the task's stated outcome, and the tui-side comment (`internal/tui/theme_seams.go:5-10`) still describes the resolution half. Judged against the amended intent, met.

TESTS:
- Status: Adequate
- Coverage:
  - Behavioural suites renamed with no assertion change: `cmd/theme_source_test.go` (`TestThemeSource_ReadsOnlyWhenOpened`, `_ReassembleDoesNoIO`, `_SharesTheConstructionReadsDedupScope`), `internal/tui/theme_seams_test.go` (`TestThemeSource*`, incl. the compile-time `var enumerator tui.ThemeSource = theme.DirThemeSource{…}` conformance at `:46`), `internal/capture/theme_panel_fixture_test.go` (`TestFakeThemeSource_*`).
  - Compile-time conformance is asserted on both sides of the seam (`internal/capture/theme_fake.go:20`, `internal/tui/theme_seams_test.go:36-41`), so a signature drift is a build failure rather than a nil seam.
  - Both string-literal guards that reference the renamed identifiers were updated (see above); the capture-side one has an explicit non-vacuity fatal.
- Notes:
  - Not over-tested: the task added no tests, which is right for a pure rename — the existing suites are the baseline.
  - One vacuity hole survives in the file this task renamed: `assertEnumeratorTakesBoundLoader` (`cmd/theme_source_test.go:136-151`) walks `openTUI` for `newThemeSource(…)` calls and only errors *inside* a match. If the call disappears (or is renamed again without updating the literal at `:143`), the helper inspects nothing and `TestOpenTUI_BuildsOneThemeLoader` passes silently — precisely the failure mode the task's own step 4 warns about. Non-blocking (the guard is correct today); see notes.
  - Test execution not performed (verification is by reading, per role). Compile consistency is corroborated by four later phases (15/17) editing the same files after this commit.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays the owner of assembly + resolution; `internal/tui` gains no `theme` log-component binding; the seam remains the constructor-injected 1-interface DI idiom `CLAUDE.md` prescribes (the 4-method breach of the 1-3 method guidance is argued and accepted in the task itself — the methods share one retained enumeration and one WARN dedup scope, so splitting would be worse).
- SOLID principles: Good. The rename tightens interface-name/responsibility correspondence; no responsibility moved.
- Complexity: Low — identifier substitution only.
- Modern idioms: Yes; no idiom surface touched.
- Readability: Good. `ThemeSource` / `DirThemeSource` / `fakeThemeSource` / `countingThemeSource` read consistently at the wiring sites, and `themeState.source` reads better than the old `themeState.enumerator`.
- Issues: The rename stopped at the exported/typed surface — a handful of *test-side* helper and local identifiers still say "enumerator" while holding a `ThemeSource` (detailed below). It does not break anything, but it partially undercuts the task's own "so grep stays useful" rationale.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_panel_behaviour_test.go:18,23 (call site :64) — rename `behaviourEnumerator` → `behaviourThemeSource` and `newBehaviourEnumerator` → `newBehaviourThemeSource`; the type embeds `theme.DirThemeSource` and its own doc comment already says so, and its siblings are already `fakeThemeSource` / `countingThemeSource`.
- [do-now] internal/tui/theme_panel_entry_test.go:35 and internal/tui/theme_panel_open_test.go:22,41 — rename `newEntryEnumerator` → `newEntryThemeSource` (returns `*fakeThemeSource`; call sites theme_panel_entry_test.go:99,196,228,254,278,298,385,479, theme_flash_precedence_test.go:119,355, keymap_dispatch_guard_test.go:262), `newOpenEnumerator` → `newOpenThemeSource`, and `countingEnumeratorOver` → `countingThemeSourceOver` (returns `*countingThemeSource`; call sites theme_panel_open_test.go:217,245, theme_panel_arrow_test.go:380, theme_panel_commit_load_test.go:553, theme_testing_test.go:63,92).
- [do-now] cmd/theme_source_test.go:131,136 — rename `assertEnumeratorTakesBoundLoader` → `assertThemeSourceTakesBoundLoader` so the helper greps with the seam it guards.
- [do-now] internal/tui/keymap_dispatch_guard_test.go:275,286 — replace the failure text "t did not open the panel against a faked enumerator — the guard's probe would be vacuous" with "t did not open the panel against a faked theme source — the guard's probe would be vacuous".
- [quickfix] internal/tui/theme_panel_{open,close,arrow,cursor,commit_recompute,commit_slot,commit_load,behaviour}_test.go, internal/tui/theme_testing_test.go, internal/tui/theme_seams_test.go, cmd/theme_source_test.go:40,63 — sweep the ~40 local variables still named `enumerator` (each holds a `ThemeSource`) to `source` / `themeSource`; mechanical but larger than an inline edit, so route it through the pipeline rather than applying inline.
- [quickfix] cmd/theme_source_test.go:136-151 — `assertEnumeratorTakesBoundLoader` passes vacuously when no `newThemeSource(…)` call is found in `openTUI`; track a `found` bool across the `ast.Inspect` and `t.Fatal` when it is still false, so a future rename or an un-wired panel fails the guard instead of silently disabling it.
