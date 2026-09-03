## Attempt 1

# Fix attempt 1 — resume-hooks-silently-lost-8-30

## ISSUES

### `cmd/seam_guard_test.go:198-213` — the explicit-type arm silently drops a seam declared with a NAMED func type

`holdsFunc` returns `false` for any explicit type that is not a literal `*ast.FuncType`. So:

- `var seam someFn = impl` (where `type someFn func(string) string`) yields `*ast.Ident` → dropped.
- `var seam nanoid.Generator = gen` yields `*ast.SelectorExpr` → dropped.

This is precisely the false negative the recogniser's own doc comment claims it prevents. The reviewer probed it on a scratch copy of the repo: adding `type tomorrowFn func(string) string` / `var namedTypeSeam tomorrowFn = defaultTomorrow` plus a bare `namedTypeSeam = …` assignment in `cmd/open_test.go` leaves the guard **green**. Named func types are a live idiom in this repo — 16 production declarations across `cmd/bootstrap`, `internal/nanoid`, `internal/spawn`, `internal/session`, `internal/xdg`, `internal/theme`, `internal/resolver` — so the shape is one ordinary declaration away, and the guard would be reporting a safety it does not provide.

FIX (the reviewer verified this exact shape on a scratch copy — it flags the staged named-type offender, leaves the real tree green, and the derived set stays at 18 because `cmd` declares no func types today):

1. In `funcSeamDecls`, collect the package's `type X func(…)` declarations into a `funcTypes` set in the same pass that builds `packageFuncs`.
2. In `holdsFunc`, replace the explicit-type branch with a switch:
   - `*ast.FuncType` → true
   - `*ast.Ident` → `funcTypes[name]`
   - default → false
3. Add a `type localFn func(int) error` / `var fromNamedType localFn = someFunc` pair to the `TestFuncSeamGuard` derivation fixture **and** to its `want` slice.

`fatalErrorStderr` keeps its exclusion under this rule: its type is a `SelectorExpr` (`io.Writer`).

The imported-named-func-type case (`var seam pkg.FuncType = …`) cannot be resolved from the AST alone. **State it as the residual in the doc comment** rather than claiming it away — the current sentence "An explicit type settles it either way" is true only for a literal `func(…)` type and must be corrected.

Do NOT take the reviewer's ALTERNATIVE (falling through to the initialiser arms for a `SelectorExpr` type). It catches a little more at the cost of two interacting branches; the flat switch plus an honest residual note is the wanted shape.

## COMMENT_CORRECTIONS

- `CLAUDE.md:107` — the DI/testing paragraph still names the guard file by its pre-rename path. `cmd/deps_seam_guard_test.go` no longer exists; it is `cmd/seam_guard_test.go`. This is the only live reference to the old name outside archived workflow artifacts.
  - OLD: A source guard (`cmd/deps_seam_guard_test.go`) fails any `*_test.go` assigning either kind of seam directly
  - NEW: A source guard (`cmd/seam_guard_test.go`) fails any `*_test.go` assigning either kind of seam directly

## NOTES

Everything else in the review came back clean and needs no action:

- The 9 → 18 family expansion was **forced, not scope creep** — the reviewer independently confirmed that leaving the other nine hand-installed would have left the derived guard red on the tree. Keep it.
- The generous qualified-name initialiser arm admits exactly `acquireDaemonLock` and `osExit` today, both real seams. No spurious member. Keep it.
- The 128-site AST rewrite was sampled hard across every touched file: no changed replacement value, no lost capture, no reordered cleanup. `installStubVersionChecker` (`cmd/version_guard_test.go:26-31`) still registers the seam restore before the `resetVersionCheckForTest` cleanup, so LIFO order is unchanged. Nothing to repair.
- The `TestMain` exemption is sound: keyed on a reserved name Go permits once per build configuration, and an assignment moved into a helper called from `TestMain` would still be flagged.
- Do not act on the reviewer's blind-to-pointer-aliasing note (`p := &openTUIFunc; *p = f`) — pre-existing, shared with the `*Deps` arm, and guards target accidents not evasion.
- Do not cache `declaredSeams`' re-parse; measured at ~0.15s for the whole guard file.
- The reviewer noted the subtest named "it derives the function-var seam set from the production sources" actually derives from a synthetic fixture. That is the better test and should stay as-is; renaming it is optional and not required.

## VERIFICATION

- `go test -count=1 ./cmd/...` and the full unit lane `go test ./...` clean.
- `go vet ./...`, `go vet -tags integration ./cmd/...`, `gofmt -l`, `golangci-lint run` clean.
- Prove the fix bites using a **scratch copy** of the repo — never `-overlay` (an overlay cannot validate a source-reading guard: the guard parses from disk while the overlay substitutes only build inputs, so the biting direction passes vacuously) and never by editing the repo itself.
