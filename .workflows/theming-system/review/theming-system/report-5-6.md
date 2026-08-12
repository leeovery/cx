TASK: theming-system-5-6 — An Unresolvable Fallback Is A Fatal One-Line Message, Never A Panic

ACCEPTANCE CRITERIA:
1. Injected built-in source omitting DefaultDarkSlug + broken constant nomination → error message exactly `built-in theme tokyo-night is missing or invalid — this binary is broken`.
2. Same for DefaultLightSlug on a broken light slot, naming `tokyo-night-day`.
3. A *corrupt* (not absent) fallback file produces the same sentence — the message does not vary by reason.
4. The message is asserted against the exported single source AND against the literal §14A string.
5. The failure is an ordinary `error`: no panic, no `os.Exit` inside `internal/theme` or `cmd`, no `log.Fatal` — proven by a source guard plus a test that recovers nothing.
6. `openTUI` returns the error and constructs no TUI; `Execute` prints one line and the process exits non-zero via `main`.
7. A *nomination* failure with a healthy fallback is not fatal — valid `Resolution`, `FellBack=true`, no error.
8. Nothing walks the embedded set at startup — a counting built-in source records at most one read under a constant, two under a pair, zero on the exec path.
9. No compiled-in fallback palette anywhere — guard test asserts `internal/theme` declares no `Theme` literal outside tests.
10. The seam defaults to `BuiltinBytes` in production; no call site has to pass it.

STATUS: complete

SPEC CONTEXT:
§7.6 ("The build-time guarantee", specification.md:661-676) removes the runtime last-resort palette: a unit test parses/validates every embedded built-in and asserts the fallback slugs resolve within that set; the loader returns an ordinary rejection for an embedded parse failure and never panics; escalation happens "where the fallback is needed" as a fatal error travelling the normal path; validation is explicitly not startup-eager; and a compiled-in Tokyo-Night-Dark last resort is a recorded rejection. §14A (specification.md:1866) pins the copy `built-in theme <slug> is missing or invalid — this binary is broken`. I diffed the spec sentence against the implemented format string character-for-character (em dash included) — they match.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - `internal/theme/broken_builtin.go:5-14` — `brokenBuiltinFormat` + `BrokenBuiltinError(slug)`; the single production occurrence of the sentence in the tree (verified by tree-wide grep: only this file, plus test literals).
  - `internal/theme/resolution.go:167-190` — `resolveSlot` returns `BrokenBuiltinError(fallbackSlug)` with a zero `SlotResolution` when the *fallback* load rejects; the nomination's rejection and the fallback's are distinct named values (`rejection` vs `fallbackRejection`), so the "fatal only because a fallback is missing" distinction is explicit in-source. No second fallback, no substituted palette. `ResolveNomination` (60-65, 141-165) propagates it with a zero `Resolution`.
  - `internal/theme/load.go:17-21` + `internal/theme/builtins.go:81-86` — the `BuiltinSource func(slug string) ([]byte, bool)` seam, documented as staging the otherwise-unreachable broken-binary state, with `builtinBytes` falling through to `BuiltinBytes` on nil, so production call sites pass nothing (AC 10).
  - `cmd/open.go:496-512, 610-614` — `themeResolution` returns the error unaltered; `openTUI` returns it at line 612-614, before `buildSessionConnector`, the `tmux.InsideTmux()` current-session read (665) and `buildTUIModel` (673) (AC 6).
  - `cmd/root.go:193-203` — `Execute` returns it untouched (it only prints for `*bootstrap.FatalError`); `main.go:62-79` `classify` writes the single stderr line and `main.go:37` owns the sole `os.Exit`.
- Notes:
  - AC 6 says "`Execute` prints one line"; in the shipped code `main.classify` is what prints, and `Execute` prints only `*bootstrap.FatalError` messages. This is the correct reading of the existing architecture (a theme fatal is an ordinary error, and the AC's own second half says the exit is main's) — the user-visible outcome, one line on stderr plus exit 1, is what the tests pin. Not drift.
  - The plan's "Do" asked for an in-source comment naming §7.6's rejected compiled-in last-resort palette. `resolution.go:167-169` records the substance ("no safety net belongs beneath this point, since a last-resort palette would paint values nobody chose while a broken embedded set still looked fine") and the guard test's failure message states the rejection explicitly. Satisfied in substance, and it correctly avoids citing spec sections in code comments.
  - Sibling paths behave consistently and deliberately: `cmd/doctor_theme.go` diagnoses via `ResolveByName` (no fallback ⇒ no escalation on the read-only path), and `internal/tui/theme_panel.go:168-185` / `theme_panel_commit.go:117-127` degrade rather than escalate mid-session, both with comments saying why. `internal/tui/builtin_themes.go` seeds from the embedded set rather than a Go-side palette.
  - Nothing is emitted on the fatal path: `reportSlot`/`reportFallback` run only after a successful load, so no `theme` component line fires for the fatal (AC-adjacent plan instruction "emit no new event" holds).

TESTS:
- Status: Adequate
- Coverage:
  - `internal/theme/broken_builtin_test.go:50-96` `TestFallback_MissingBuiltinIsFatal` — table over dark constant / light slot / dark slot, asserting the literal sentence, equality with `theme.BrokenBuiltinError(...)` (AC 4's dual assertion), and a zero `Resolution` (AC 1, 2).
  - `:98-114` `TestFallback_CorruptBuiltinIsFatal` — corrupt (missing-token) embedded bytes yield the identical sentence (AC 3).
  - `:19-48` `TestBrokenBuiltinError_CopyIsPinned` — both slugs against the transcribed literal.
  - `:116-125` `TestFallback_NeverPanics`; `:127-154` `TestFallback_NominationFailureIsNotFatal` (FellBack=true, Requested preserved, fallback palette on both slot and nomination) (AC 7).
  - `:156-200` `TestResolution_NoStartupEagerValidation` — counting `BuiltinSource`: 1 read for a constant, 2 for a pair, 1 for a drop-in nomination (AC 8's first half). `internal/theme/leaf_guard_test.go:91-128` bans `init()` and call-bearing package-level initialisers. The exec-path half of AC 8 is covered by `cmd/open_theme_nomination_test.go:26-42`, which guards the direct-attach path against any `themeResolution`/`buildThemeLoader` call.
  - `:202-221` `TestTheme_NoCompiledInFallbackPalette` (AC 9); `:223-247` `TestBuiltinSource_DefaultsToTheEmbeddedSet` and `:254-297` `TestBuiltinSource_HasNoProductionCallSite` (AC 10, including a self-check that the exemption list still names live files).
  - `internal/theme/embedded_test.go:160-207` — internal/theme source guard banning `panic` (excluding `NewLoader`'s deliberate nil-seam panic), `os.Exit` and `log.Fatal*`, plus a single-source check that only `broken_builtin.go` carries the copy (AC 5, first half).
  - `cmd/theme_fatal_test.go:14-51` `TestThemeFatal_TravelsExecuteUnaltered` — the error survives `Execute` unwrapped and classifies as none of `*bootstrap.FatalError` / silent-exit / `*UsageError`, with `fatalErrorStderr` untouched so the line is printed exactly once; `:56-124` scoped source guard over cmd's theme-touching functions (AC 5, second half).
  - `cmd/open_theme_construction_test.go:348-391` `TestOpenTUI_FatalBeforeModelConstruction` — good test: the tmux commander recording zero calls and the still-pending staged bootstrap warning are two independent proofs that no model was built (AC 6).
  - `main_theme_fatal_test.go:10-28` — `run()` returns code 1, does not panic, and stderr is exactly the pinned line + `\n`, transcribed from the spec rather than composed from the format string.
- Notes:
  - Mild over-testing: `internal/theme/resolution_test.go:281-329` (`TestResolveNomination_UnresolvableFallbackErrors` table + its corrupt subtest) is now a strict subset of the two broken-builtin tests — same loader, same settings, same `requireZeroResolution`, minus the message assertion. Its "it never falls back a second time" subtest (331-347) is unique and should stay.
  - `TestFallback_NeverPanics` is subsumed by the first row of the missing-builtin table (a panic there would already fail the test), but it is plan-named and costs one loader construction; leaving it is defensible as a named-property anchor.
  - No test asserts the fatal path emits nothing on the `theme` component. The plan permitted a prior `fallback applied` WARN, so the property is weakly specified, but it is currently unpinned.

CODE QUALITY:
- Project conventions: Followed. No bare `os.Exit`/`log.Fatal` outside `main` (guarded on both sides); the closed log vocabulary is untouched (no new event); `internal/theme` stays path-free and leaf-shaped; tests are table-driven, `t.Parallel()`-free, and use `themetest`/`sourceguardtest` rather than hand-rolled scaffolding.
- SOLID principles: Good. Escalation lives where the fallback is consumed (`resolveSlot`), the parse layer keeps returning ordinary `*Rejection`s, and the seam is a single-purpose function field rather than an interface widening.
- Complexity: Low. `resolveSlot` is one branch deeper than before; `builtinBytes` is a two-line nil-default.
- Modern idioms: Yes — `fmt.Errorf` with no wrapped cause (nothing downstream can act on one), function-field seam with a nil-means-production default, `slices.Equal` in the counting assertions.
- Readability: Good. Every comment on the changed lines is accurate against the code (I checked each): `broken_builtin.go:7-11` correctly states the slug named is the fallback's; `resolution.go:167-169` correctly describes the no-second-fallback contract; `load.go:19-21` correctly states nil reads the embedded set. No comment references a task id or phase.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/resolution_test.go:281-329 — delete `TestResolveNomination_UnresolvableFallbackErrors`'s three-case table and its "a corrupt fallback fails the same way as a missing one" subtest; both are strictly weaker duplicates of `broken_builtin_test.go:50-114`. Keep the unique "it never falls back a second time" subtest (331-347), renaming the parent to match what survives.
- [quickfix] internal/theme/broken_builtin_test.go:218-221 — `isThemeTypeExpr` matches only an `*ast.Ident` named `Theme`, so a palette table written as `[]Theme{{...}}` or `map[string]Theme{...}` (elided element types) slips past the no-compiled-in-palette guard. Extend the walk to also flag composite literals whose enclosing `ArrayType.Elt` / `MapType.Value` is `Theme`.
- [quickfix] internal/theme/broken_builtin_test.go:50-96 — add a `logtest.Sink`-backed loader to the missing-fallback table and assert zero `theme` component records after the fatal returns, pinning "nothing further is logged for the fatal" (currently only true by construction).
