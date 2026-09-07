## Attempt 1

ISSUES:
- `cmd/capturetool/import_guard_test.go:34` and `cmd/capturetool/shared_constructor_test.go:13` still call `portalbintest.ProjectRoot()` directly, each with its own `resolve project root` fatal — the acceptance criterion "`ProjectRoot` is called from exactly one place across the guard family" is unmet. The executor flagged this as a deliberate deviation, and the *reasoning* is right (neither site is a source scan; routing them through `RepoSources` would parse ~1000 files to obtain one string) — but the conclusion does not follow, because the criterion is reachable without doing that. It also makes a claim now written into `CLAUDE.md` false: "…through `sourceguardtest.RepoSources`, which is the one place the guard family calls it from."
  FIX: Extract the fatal-wrapping root resolution already inside `scanRoot` (`internal/sourceguardtest/reposources.go:51-67`) as an exported `ProjectRoot(t harnesstest.TestingT) string`, have `scanRoot` call it, and re-point both `cmd/capturetool` sites onto it. That deletes the last two copies of the preamble, makes the CLAUDE.md sentence true as written, and costs no extra parsing — `import_guard_test.go` still gets a bare root for `sourceguardtest.InDir`, `shared_constructor_test.go` still joins its two named paths.
  ALTERNATIVE: Accept the deviation and amend the CLAUDE.md sentence to say `RepoSources` is the only place the *scan* resolves it, recording the two root-anchoring sites as sanctioned. Cheaper, but leaves the criterion unmet and two copies of the preamble standing — which is the duplication the task exists to remove. Take the fix.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/restoretest/orchestrator_literal_guard_test.go:127` — names a `root` parameter the function no longer has.
  OLD: // orchestrator type in a _test.go under root, as "<file>:<line>". It reads the
  NEW: // orchestrator type in a _test.go the scan reaches, as "<file>:<line>". It reads the
- `internal/restoretest/session_restorer_literal_guard_test.go:151` — same stale `root` reference.
  OLD: // session-restorer type in an integration-tagged _test.go under root, as
  NEW: // session-restorer type in an integration-tagged _test.go the scan reaches, as

NOTES:
- Verdict preservation independently confirmed on eleven guards by planting violations in a throwaway tree; all fired with their own wording and a root-relative path. The reviewer additionally probed the one guard the executor reported as unprobeable (a file need only parse, not compile, for an AST guard) and confirmed it fires, and confirmed one lane-discriminating guard correctly stays silent on an untagged plant.
- The leaf-guard allowlist edit is sound rather than a weakening: the assertion judges the transitive set, so admitting one stdlib-only untagged package admits exactly that package, and the day it grows a dependency the guard flags it.
- Both additions beyond the Do list earn their place: `Rooted` is what lets the driver serve the two fixture-driven guards without a second export, and `ParsedSource.Position` is the direct replacement for the deleted variant, without which the deleted sites would grow back.
- One scan-set change, judged sanctioned: two `internal/log` guards previously walked with a narrower skip list, so twelve vendored skill-asset files under a dot-directory leave their scan. Grepped — none contains the symbols either guard polices, so no verdict changed, and the Do list mandates composing the enumerator whose documented rule is exactly this.
- Residue worth a later sweep: `internal/tui/theme_flash_precedence_test.go:194-201` now parses one package twice in a subtest; four `rel := source.Path` aliases stand where the conversion left them; and several guards still reach `source.Fset.Position(...)` for a bare line where `source.Position(...)` now exists, so two routes to a position coexist.
- `Selection.accepts` uses `default: return true`, so an out-of-range value behaves as `AllSources`. Fine for a test helper; noted because the enum is exported.
