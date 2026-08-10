# Analysis Report: theming-system (Cycle 5)

## Stats

- Total findings: 13 (duplication 5, standards 3, architecture 5)
- Deduplicated findings: 13 (no cross-agent duplicates; two pairs grouped into shared tasks)
- Proposed tasks: 9

## Summary

No two agents found the same thing this cycle, so nothing merged on identity — but two pairs describe one surface each and were grouped: doctor re-deriving rules `internal/theme` owns (architecture + duplication), and the theme panel's stated geometry disagreeing with what it renders (two standards findings). The substantive work is four medium items — doctor taking two independent parses of the same themes directory, the panel's blank page-alignment rows raising §9.8's height refuse threshold ~3 rows above the specified floor, three near-identical `theme.Member`-valued fields on the model kept correct only by comment, and `theme.Loader`'s zero value compiling as a production loader that reserves no built-in slugs. The rest is contained test-scaffolding duplication in `internal/tui` plus two low structural-tightening items. Production code remains essentially duplication-free and the implementation continues to track the specification closely.

## Interaction with earlier settled decisions

- **Panel width band (13-11, user decision).** Task 2 changes the **height** floor only. The width ladder (`themePanelPreferredWidth` 30 / `themePanelMinWidth` 24), the preferred-affordance step and `dimWidth`'s refuse threshold are explicitly out of scope and must not move. The narrower band the user chose stands untouched.
- **`themeState.nomination`'s post-commit assignment (13-9, user decision).** Task 3 does **not** touch `nomination`. The three representations it collapses are `appearanceGate.appearance`, `themeState.canvasMode` and the `bgReplyArrived`/`bgReplyDark` pair — a different set of fields from the one 13-9 settled. The task carries an explicit "do not touch `nomination`" constraint so the decision cannot be reversed by drift.
- **Positional `prefs.ThemeKeys` → `theme.RawKeys` (13-4).** Not raised this cycle; nothing proposed near it.

## Discarded Findings

- **`internal/themetest` is missing from CLAUDE.md's internal-package inventory** (standards, low) — premise is false. Task 13-14 added exactly this row; it landed in commit `b95fd990` and the row is present at CLAUDE.md:84, carrying the "production code must not import" statement. Nothing to do.
- **Every package hand-rolls its own AST source-guard scaffolding — the enumerate-and-parse half** (duplication, medium) — settled residue, narrowed out. Task 14-4 single-sourced the *enumeration* into `portalbintest.PackageGoFiles`, and 13-3 did the repo-wide walk (`GoSourceFiles`); a reviewer confirmed the residual ~8-line enumerate-and-parse loop is written per package by design. Verified: `cmd`, `internal/tui` and `internal/theme` all now call `portalbintest.PackageGoFiles(".", false)` and differ only in the loop around it. The parse-mode/`FileSet` "drift" the finding names is cosmetic rather than behavioural — `parser.SkipObjectResolution` versus mode `0` changes no guard's coverage, and `internal/theme` mints a `FileSet` per file precisely because its guards report positions and carry the set beside the file. The repo-wide walk half is likewise 13-3 residue. **Only the third layer survives** as Task 6: the `Decls → *ast.FuncDecl → ast.Inspect → *ast.CallExpr` preamble written out at ~12 sites, which no prior cycle raised and which `portalbintest` has no helper for.
- **The "`theme` component record" sink predicate is stated twice in cmd's tests** (duplication, low) — low severity, two call sites in one package, no drift and no drift hazard (both helpers apply the identical filter and a change to one could not make the other assert something false). It does not cluster with anything else proposed. Pure test tidiness; dropped under the low-severity filter.
