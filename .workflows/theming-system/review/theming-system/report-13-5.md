TASK: theming-system-13-5 — Replace The Hand-Rolled cmd Theme-Test Helpers With The Shared Ones (tick-a63cd3, phase 13 "Analysis (Cycle 3)", severity low, source: duplication)

ACCEPTANCE CRITERIA:
1. `snapshotTree` is gone; the three no-write assertions run on `portaltest.SnapshotStateDir` and report via `FormatDelta`.
2. A file-to-symlink swap under an asserted root now fails the no-write assertion.
3. One vacuity guard exists and is called by all three silent-path tests.
4. `go test ./cmd` passes.

STATUS: complete

SPEC CONTEXT:
The task carries no new product behaviour — it is a test-quality remediation over claims the spec pins elsewhere. The claims the touched assertions defend are §12.2 ("`portal doctor` — a read-only theme health line": doctor "heals nothing on the read-only path"; §1471 extends the read-only theme scan to the `--fix` path across both passes) and §1495 ("Emission is controlled by an injected logger, not by the loader deciding" — `cmd` passes `log.Discard` on `portal doctor`, `portal theme export` and `capturetool`, so those surfaces write no `theme` record at all). The three no-write assertions are the evidence for the first claim; the three silent-path tests are the evidence for the second. The task only changes where the primitives behind that evidence live, so nothing in the spec constrains it beyond "the claims must stay enforced, and enforced non-vacuously".

IMPLEMENTATION:
- Status: Implemented (all five "Do" steps).
- Location:
  - Shared helpers, `cmd/theme_test.go:366-382` — `treeFingerprint` (wraps `portaltest.SnapshotStateDir`, `t.Fatalf` on snapshot error) and `assertTreeUnchanged` (re-fingerprints and reports one `t.Errorf` per `portaltest.DiffFingerprints` delta through `portaltest.FormatDelta`, prefixed by the caller's `subject`).
  - Vacuity guard, `cmd/theme_test.go:384-404` — `assertNoThemeRecords(t, run func()) []logtest.Record`: installs a `logtest.Sink`, runs the closure, asserts zero `theme`-component records via the existing `themeEvents` filter, then installs a second sink and emits one real `theme.NewEventLogger(log.For("theme")).Rejected(...)`, `t.Fatalf`-ing unless exactly one event is captured. Returns the run's full capture for callers whose claim is total silence.
  - No-write call sites (all three converted): `cmd/theme_test.go:469`/`478`, `cmd/doctor_theme_test.go:392`/`401`, `cmd/doctor_fix_theme_test.go:187`/`204`.
  - Vacuity-guard call sites (all three silent-path tests, four call sites): `cmd/theme_test.go:356` (`TestThemeExport_EmitsNoThemeEvents`), `cmd/doctor_theme_test.go:349` and `:366` (both subtests of `TestThemeAdvisories_EmitsNoThemeRecords`), `cmd/doctor_fix_theme_test.go:315` (`TestDoctorFix_EmitsNoThemeRecords` — the one that previously had no guard at all).
  - Deletions: `snapshotTree` is gone repo-wide (grep for `snapshotTree` across `*.go` returns nothing), as are the two byte-identical "the sink captures a theme event when one is emitted" subtests. Imports it alone required were dropped: `crypto/sha256`, `io/fs`, `maps` from `cmd/theme_test.go`; `maps`, `internal/log`, `internal/logtest`, `internal/theme` from `cmd/doctor_fix_theme_test.go` (the later-added `maps`/`slices` use at `doctor_fix_theme_test.go:189` re-imports `maps` for a different purpose — the snapshot-size vacuity check).
- Criterion 1: met. Criterion 3: met.
- Criterion 2: met structurally. `portaltest.fingerprintEntry` stats with `os.Lstat` (`internal/portaltest/fingerprint.go:75`) and records `IsSymlink`/`SymlinkTarget`, and `fieldDeltas` (`fingerprint.go:165-167`) returns a single `became-symlink` delta whenever `IsSymlink` differs — so a file-to-symlink swap of identical content is a delta, which `assertTreeUnchanged` reports as `<path>.IsSymlink: pre=false post=true` (`fingerprint.go:210-211`). The old `entry.Info()` version was blind to exactly this.
- Criterion 4: judged by reading (test execution is out of scope for this review, and the commit predates several later green full-suite passes). No compile-level defect found: every helper signature matches its call sites (`assertTreeUnchanged` 4-arg at all three sites; `assertNoThemeRecords`'s return dropped only at `doctor_fix_theme_test.go:315`, which is legal and correct because a `--fix` run legitimately emits non-`theme` records); `logtest.Record` and `(*Sink).Records()` exist with the used shapes (`internal/logtest/capture.go:31`, `:153`); the import sets of all three files are consistent with their remaining uses.
- Notes:
  - `cmd/theme_test.go` is the first **untagged** (unit-lane) `cmd` file to import `internal/portaltest`. That is safe: `portaltest` carries no build tags outside its `fingerprint_{darwin,linux}.go` platform pair, has no `init()`, and importing it neither spawns a daemon nor execs a binary — the CLAUDE.md lane rule is about what a test *does*, not what it imports. `internal/state/pgrep_sandbox_prod.go`'s mention of `portaltest` is a comment, not an import, so no production package picks it up.
  - Later commits (`a4bc7bd5`, `915e7fcb` — the comment-standard and adversarial-comment passes) stripped the doc comments this commit wrote on the helpers and the three tests. That is the intentional later-phase revision the review context warns about, not drift. The one comment that survived (`cmd/theme_test.go:365`, "Lstat-based, so a file-to-symlink swap of identical content is a change.") is accurate against `portaltest`'s implementation, and no stale reference to `snapshotTree` or to the removed subtests survives anywhere in `cmd`, `internal` or the specification.

TESTS:
- Status: Adequate.
- Coverage: This task *is* test code, so the relevant question is whether the claims it moved are still enforced, and enforced at least as strongly. They are, and strictly more strongly on both axes:
  - No-write: `portaltest.Fingerprint` compares size, mtime, ctime, content hash, symlink-ness and symlink target per entry versus the old "mode + sha256" string. Every mutation the old map caught is still caught, plus the file-to-symlink swap, plus an identical-content atomic rewrite (mtime/ctime move). The old explicit `info.Mode()` field is no longer compared directly, but a `chmod` moves `CtimeNanos`, which `statNanos` populates from `syscall.Stat_t` on both supported platforms (`fingerprint_darwin.go` / `fingerprint_linux.go`) — so mode coverage is preserved indirectly rather than lost.
  - Failure diagnostics: `FormatDelta` names the offending path and field (`<path>: created (post=…)`, `<path>.Sha256: pre=… post=…`) instead of the old two-whole-maps dump, which is the readability half of the task's outcome.
  - Non-vacuity: every caller's own vacuity precondition survived the refactor (`theme_test.go:470` empty-snapshot check, `doctor_theme_test.go:393` `len(before) < len(files)`, `doctor_fix_theme_test.go:188` `len(before) < 3`, and the advisory-count guards inside each `assertNoThemeRecords` closure), and `TestDoctorFix_EmitsNoThemeRecords` gained the harness-live guard it previously lacked.
  - No false-positive risk introduced: the fingerprint tracks mtime/ctime but not atime, so a read-only scan of the asserted tree cannot trip it.
- Notes: The task's "Tests" section lists three temporary mutation experiments (write a file, swap a file for a symlink, emit a record on a silent path) — transient verification steps by design, so there is nothing committed to check. All three would now fail as intended: a write yields a `created`/`content` delta named by `FormatDelta`, a symlink swap yields `became-symlink`, and a stray `theme` record trips the `themeEvents` check inside the shared guard. No over-testing: the three surviving caller-side `len(records) != 0` assertions each carry distinct, information-bearing wording (export vs scan vs unusable-directory), which the task explicitly asked to keep.

CODE QUALITY:
- Project conventions: Followed. Helpers are `t.Helper()`-marked, `*testing.T`-first, `t.Fatalf` for harness failures and `t.Errorf` for claim failures; shared `cmd` theme test helpers already live in sibling test files (`themeEvents` in `open_theme_construction_test.go`, fixture builders in `theme_source_test.go`), so declaring these in `theme_test.go` matches the package's existing arrangement. No `t.Parallel()`. No new production code, no new log component, no tmux or filesystem reach outside `t.TempDir()`.
- SOLID principles: Good. Each helper does one thing; `assertTreeUnchanged` composes `treeFingerprint` rather than duplicating it.
- Complexity: Low — three helpers, no branching beyond error handling.
- Modern idioms: Yes. Closure-taking assertion helper is the right shape here (it must own the sink lifetime around the run), and the diff loop is a plain `range`.
- Readability: Good. `subject` as the caller-supplied claim prefix keeps each failure line self-describing.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/theme_test.go:397-403 — after the vacuity probe, re-install the run's sink (add `log.SetTestHandler(t, sink)` after the `themeEvents(t, live)` check) so `assertNoThemeRecords` does not leave the unreachable `live` handler installed for the remainder of the caller's test; today any logging a caller performs after the helper returns lands in a sink nothing can inspect and which already holds one record.
