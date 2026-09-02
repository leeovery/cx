## Attempt 1

ISSUES:
- `internal/portaltest/fingerprint.go:243` — `const deltaFmt = "portaltest backstop: developer state dir mutated at %s: %s"` is the third — and the only *runtime* — statement of exactly the claim this task exists to retire. After the fix, a developer whose test trips the backstop is told their **developer state dir** was mutated when the delta is in the per-test temp `HOME`. The corrected comment three lines below it now contradicts the message the same file emits.
  FIX: change the const to name what it walks, e.g. `"portaltest backstop: state dir under the scrubbed HOME mutated at %s: %s"`, and move the duplicated literal in `internal/portaltest/fingerprint_test.go:33` (`hasDelta`) with it.
  ALTERNATIVE: in the same edit, derive the helper from the const — `want := fmt.Sprintf(deltaFmt, path, deltaType)` in `hasDelta` — so the assertion can never drift from the message again. Recommended: it removes the hand-copied literal that made this a two-file edit in the first place; the only cost is that `hasDelta` no longer reads as a literal expectation.
  CONFIDENCE: high

- `internal/portaltest/backstop_ordering_test.go:44-56` — the second test `os.MkdirAll`s and writes `leaked.json` into `*gotDir`, a path resolved from the **ambient env**. Today that is safe only because `isolated_env_test.go`'s `TestMain` re-points `HOME`/`XDG_CONFIG_HOME` for the whole binary — a guarantee living in a different file under a different package clause, unmentioned here. Under precisely the regression this file exists to catch (resolution hoisted above the scrub) plus any future change to that `TestMain`, the write lands in the developer's live `~/.config/portal/state`. The repo's ABSOLUTE INVARIANT (no mutation outside the test's own temp dirs) and `fingerprint_test.go:1-3`'s own file header ("driven against a controlled `t.TempDir()` root — never the developer's real state directory") both argue for the test to carry its own guarantee.
  FIX: open the test with `t.Setenv("XDG_CONFIG_HOME", t.TempDir())`, mirroring `TestIsolateStateForTest_ResolvesDevStateDirUnderScrubbedHome:29-30`, so the resolution lands in a temp dir on either side of the ordering.
  ALTERNATIVE: assert the invariant before writing — `if !strings.HasPrefix(*gotDir, os.Getenv("HOME")) { t.Fatalf(...) }` — which refuses to write anywhere unexpected and documents the coupling. Tradeoff: it re-states the first test's assertion; the `t.Setenv` is one line and makes the test hermetic outright. Recommended: the `t.Setenv`.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/portaltest/isolated_env.go:106-107` — the seam's comment is a claim about tests ("so a test can observe … and so pin that…"), which `code-quality.md` lists under "Never in a comment". The repo's own seam vars either carry no comment (`cmd/open_burst.go:12`, `cmd/state_daemon.go:69`) or state the substitutability without naming a test (`cmd/config.go:160-161`).
  OLD:
  // Indirection so a test can observe which directory the backstop is installed
  // over, and so pin that it is the one resolved after the HOME scrub.
  NEW:
  // A var so the installer can be substituted and the directory it is handed —
  // the one resolved after the HOME scrub — observed.

NOTES:
- On the seam, judged explicitly: it earns its place, narrowly. It is unexported, cleanup-restored, initialised to the real function, and matches the repo's dominant seam idiom. A seamless pin exists (stage a fake host `XDG_CONFIG_HOME`, let the real backstop's cleanup fail under a reorder) but asserts nothing explicitly and pins only the negative half; a `sourceguardtest` AST guard on statement order would pin it with zero production surface but is materially more brittle. Keep the var.
- `TestIsolateStateForTest_RegistersBackstopOverResolvedDir` does not pin ordering — under a reorder it still passes. Its value is the linkage between the captured dir and the captured snapshot, which is acceptable, but not the second pin its name implies. Criterion 3 is satisfied by the first test alone.
- The two new tests replace the backstop for their own duration, so they run without it. Harmless — neither spawns a subprocess — but worth knowing if either grows.
- `CLAUDE.md:79` now carries two cross-references to the same section, one with an inexact title (`see "Test isolation" below` mid-row, `See "Test isolation for daemon-spawning tests" below` at the end). Cosmetic.
- `CLAUDE.md:119` ("the fingerprint backstop enforces it") sits in mild tension with the narrowed reach, but makes no claim about the developer's install and predates this diff, so criterion 2 is still met. Flagged for a future doc pass.
