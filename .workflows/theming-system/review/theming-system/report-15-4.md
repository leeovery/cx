TASK: 15-4 — Make theme.Loader's Unsafe Zero Value Unreachable From Production Code (tick-df08ed)

ACCEPTANCE CRITERIA:
- A guard exists that fails when a `theme.Loader` composite literal is introduced into any production file.
- `NewLoader`'s own construction is exempted explicitly and by name, not by a broad pattern.
- The guard fatals rather than passing when its enumeration is empty.
- Test files continue to construct `theme.Loader{...}` freely; no existing test changes.
- `go test ./...` passes.

STATUS: complete

SPEC CONTEXT:
Specification §5.4 (lines 357–367) is the property under protection: a user file whose slug collides with a built-in is rejected with `reserved name`, because "an invalid theme falls back to a built-in, so if a user file could shadow the built-in that is the fallback, the fallback itself could be broken. That must be impossible." §5.3 (line 349) pins the check to exact string equality. §11 (line 800) states the complementary half: on the by-name construction path "the embedded set first, then the themes directory ... the safety property has to come from ordering". So the codebase enforces no-shadowing twice — by *ordering* in resolution (`resolveNamed` consults `LoadBuiltin` before any directory/enumeration source, resolve.go:32, resolution.go:119-123) and by *verdict* in `Loader.isReserved` (load.go:79-81, 117-120). This task closes the second half against a zero-value `Loader`, whose nil `ReservedSlugs` reserves nothing.

IMPLEMENTATION:
- Status: Implemented (with one deliberate later-phase supersession, see Notes)
- Location:
  - `internal/theme/loader_construction_guard_test.go:20-84` — the new source guard `TestLoader_HasNoProductionCompositeLiteral` plus its `isLoaderTypeExpr` type matcher.
  - `internal/theme/load.go:12-26` — the `Loader` field docs the task asked to extend.
  - Commit `01e95df4` touched exactly `internal/theme/load.go` (+6/-5, doc comment only) and the new guard file — no production behaviour changed, no existing test changed.
- Notes:
  - Scope is repo-wide as required (`portalbintest.ProjectRoot()` → `sourceguardtest.GoSourceFiles(root)`), so a literal written in `cmd`, `internal/tui`, `internal/capture` or `cmd/capturetool` is caught. Verified by inspection that the current tree holds exactly one production `Loader` literal — `internal/theme/load.go:36` inside `NewLoader` — and ~20 test-site literals across `load_test.go`, `builtins_test.go`, `enumerate_test.go`, `events_test.go`, `union_test.go` and `internal/themetest/theme_file_test.go`, all untouched.
  - The exemption is explicit and doubly pinned: `rel == loaderConstructionFile && fn.Name.Name == loaderConstructionFunc` (line 50) exempts only that one FuncDecl, and the trailing `exempted != 1` fatal (lines 70-72) makes the exemption self-verifying — if `NewLoader`'s construction moves or is renamed, the guard fails loudly rather than degrading into a scan that matches nothing. That second assertion is also what stops the whole negative assertion from being vacuous in the "scanner is broken" sense, not merely the "found no files" sense.
  - The file enumeration is not hand-rolled and matches the shape every sibling guard uses (`internal/prefs/appearance_api_guard_test.go:29-37`, `internal/theme/slug_collapse_guard_test.go:20-27`, `internal/log/discard_guard_test.go:19`), per Do item 1 as amended by tasks 16-5 / 17-14 (the helpers moved `portalbintest` → `internal/sourceguard` → `internal/sourceguardtest`; the guard reads the current home).
  - Do item 5 ("extend the field doc comments to name the guard") *was* done in this commit and was then deliberately reverted by task 16-4 (`735daeeb`, "rewrite production comments that name a test or count call sites"), which is a later-phase supersession, not drift. The surviving doc (load.go:13-15) still carries the load-bearing half: the zero value reserves nothing and production goes through `NewLoader`/`NewSilentLoader`.
  - Package-local detection is correct and narrow: bare `Loader{}` is only matched when the file's directory equals `internal/theme` (line 47), so an unrelated package's own `Loader` type cannot false-positive.

TESTS:
- Status: Adequate
- Coverage: The deliverable *is* a test, and it covers the stated acceptance surface: repo-wide production scan (line 25, `_test.go` skipped at line 32), named single exemption (line 50), empty-enumeration fatal (lines 67-69), exemption-still-real fatal (lines 70-72). Both selector (`theme.Loader{}`) and package-local (`Loader{}`) literal forms are matched, including the `&theme.Loader{}` address-of form (the UnaryExpr wraps a CompositeLit whose Type is the selector).
- Notes:
  - No meta-test drives the guard against a synthetic offending file, so `isLoaderTypeExpr` itself is unproven by automation. That is the repo's established position for source guards (none of the siblings self-test either), and here it is materially mitigated: the `exempted == 1` assertion proves on every run that the matcher does detect a real literal in a real file, so a matcher that silently stopped matching cannot pass. I do not consider this under-tested.
  - Not over-tested: single test function, no redundant assertions, no fixture staging.
  - `go test ./...` was not run (test execution is out of scope for this review). Judged by reading: the file is in `package theme_test`, all four imports are used, and the three declared identifiers (`loaderConstructionFile`, `loaderConstructionFunc`, `isLoaderTypeExpr`) are unique across the `theme_test` package. Both runtime assertions hold against the current tree by inspection.

CODE QUALITY:
- Project conventions: Followed. Uses the single-sourced scan helpers rather than a hand-rolled walk; matches the sibling-guard idiom for root resolution, relativisation and fatal-on-parse-error; unit-lane (untagged) and stdlib-plus-test-helper only, so it runs in `go test ./...` as the other guards do.
- SOLID principles: Good. The matcher is factored out of the traversal; the guard owns policy only, the enumeration lives in `sourceguardtest`.
- Complexity: Acceptable. One function, three nesting levels (paths → decls → `ast.Inspect`), each level doing one thing. The `exempt` flag hoisted out of the closure keeps the inner callback flat.
- Modern idioms: Yes. `parser.SkipObjectResolution`, `ast.Inspect` with a typed switch, `filepath.Rel` for reporting.
- Readability: Good. Both failure messages state the wrong thing *and* the correct route, as Do item 3 required, and the `exempted != 1` message explains what the count failure implies about the scan's coverage.
- Issues: One accuracy overstatement in the primary failure message (see the first non-blocking note) — the causal claim it makes is not the mechanism the resolution code actually exhibits.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/loader_construction_guard_test.go:61 — the failure message claims a hand-assembled Loader lets "a drop-in taking the slug of the built-in a slot falls back to ... shadow it", but resolution consults the embedded set first on *both* passes (`resolveNamed`, resolve.go:27-37; `ResolveByNameFrom`, resolution.go:119-123), so the fallback cannot be shadowed — the real damage is the verdict: the drop-in is judged valid instead of `reserved name`, so it lists in the panel union as a second selectable row for a built-in slug (`Assembler.Reassemble` appends `builtinRows` and `fileRows` without slug dedup, union.go:131-138) and doctor reports it as loadable, while resolution silently applies the built-in. Replace the clause with: "a hand-assembled Loader reserves no built-in slugs, so a drop-in taking a built-in's slug is judged valid instead of `reserved name` — it lists as a second selectable row for that slug and diagnoses as loadable, while resolution still applies the built-in; production callers take theme.NewLoader or theme.NewSilentLoader".
- [do-now] internal/theme/union.go:107-109 — the `Assembler.Loader` doc reads "The zero Loader is valid and silent: it reserves nothing and emits nothing", which now reads as a sanction for exactly the shape the guard forbids in production. Replace the last sentence with: "The zero Loader is valid and silent — reserving nothing, emitting nothing — which is a shape for driving the ladder with a synthetic set, never one that resolves a user's theme." (Wording deliberately avoids naming the guard, per task 16-4's rule for production comments.)
- [idea] internal/theme/loader_construction_guard_test.go:75-84 — the guard covers composite literals only, so the zero value stays reachable from production by other routes: `var l theme.Loader`, `new(theme.Loader)`, an elided-type element (`[]theme.Loader{{}}`, `map[string]theme.Loader{"a": {}}`), and — most realistically — a sibling struct written without its Loader field, since both `theme.Assembler` (union.go:106-111) and `theme.DirThemeSource` (dir_theme_source.go:6-12) hold an exported `Loader` field, so `theme.DirThemeSource{Dir: d}` yields a reserve-nothing loader with no `Loader` literal anywhere. Decide how far the guarantee should extend — e.g. also flag `var`/`new` of the type and require the `Loader` field to be present in `Assembler`/`DirThemeSource` literals, versus accepting composite-literal coverage as the pragmatic line.
- [quickfix] internal/theme/loader_construction_guard_test.go:79-81 — the selector arm matches the literal package identifier `theme`, so a production file importing the package under an alias (`import th ".../internal/theme"`, or a dot-import) writes `th.Loader{}` and the guard stays silent. Resolve the file's local name for `github.com/leeovery/portal/internal/theme` from `file.Imports` once per file and compare the selector's package ident against that name (falling back to `theme` when the import is unnamed).
- [idea] internal/theme/loader_construction_guard_test.go:32 — the production/test split is the `_test.go` suffix, so a non-`_test.go` helper in a test-only package (`internal/themetest`, `internal/capture`, `internal/spawntest`) is treated as production and would fail the guard even though it never links into the shipped binary. Decide whether those packages should be exempt by path, or whether the current strictness is the intended contract.
