## Attempt 1

ISSUES:

1. `internal/hooks/store_shape_test.go:44-50` and `:119-125` still open-code the identical assertion, in
   the identical string spelling, with a byte-identical failure message
   (`"hooks.json changed:\n before %s\n after  %s"`) to the two you converted in
   `cleanstale_snapshot_test.go`. Same package, same rule, and the file already imports `hookstest`. This
   matters more than a leftover: task 7-20 (open) designates `store_shape_test.go` as the *single
   surviving home* of the shape rule and deletes `cleanstale_snapshot_test.go`'s copies — so once 7-20
   lands, one of this task's conversions is deleted and the hand-rolled form becomes the only remaining
   spelling of the assertion in `internal/hooks`. No other phase-7 task covers these lines (all 35 were
   checked).
   FIX: replace the two after-read + compare blocks with
   `hookstest.AssertHooksFileUnchanged(t, path, before, "<context>")`, taking the `before` reads
   (`:22-25`, `:99-102`) through the package's local `readFileBytes` (fatal-on-absent, matching the
   sibling files' before-reads), then drop the now-unused `os` import — those four sites are its only
   uses in the file. Suggested contexts mirroring the ones already chosen:
   `"changed by a clean that removed nothing"` (`:44`) and
   `"changed when every candidate was retained"` (`:119`). Leave the subtests themselves intact so
   7-20's assumptions hold.
   ALTERNATIVE: bank it for 7-20 to fold in, on the grounds that task 4-6 once fenced these exact reads
   as "pre-existing debt; do not fold them in". Not recommended — that fence was scoped to 4-6's one-line
   edit, 7-20's Do list explicitly says leave `store_shape_test.go` unchanged, and no task in the phase
   owns the debt, so it would simply be lost.
   CONFIDENCE: high

2. `internal/hookstest/hooks_test.go:74-80` — `captureAssert` returns nil whenever the code under test
   calls `Fatalf`, so its documented contract ("absorbing the panic a Fatalf raises so an unexpected
   fatal is a returned message rather than an abort") is false. The result is unnamed, so recovering the
   panic returns the zero value; the caller then nil-derefs on `rec.fatals` and the subtest crashes with
   `invalid memory address or nil pointer dereference` instead of reporting the recorded message.
   Confirmed empirically. Dormant today (no current subtest drives a fatal) but it is the exact path the
   helper exists for, and the next person to test `HooksFileBytes`'s non-ENOENT fatal will hit it.
   FIX: give the result a name and assign it in the deferred func, matching the working precedent already
   in the repo at `internal/logtest/capture_test.go:355-362` —
   `func captureAssert(fn func(*recordingT)) (rec *recordingT) { rec = &recordingT{}; defer func() { _ = recover() }(); fn(rec); return rec }`.
   Consider also adding the fatal-path subtest that this then makes possible (a directory staged at the
   path yields a non-ENOENT read error).
   CONFIDENCE: high

3. `cmd/doctor_test.go:861` and `:923` — swapping `os.ReadFile` + fatal for the ENOENT-tolerant
   `readFileBytes` turned two absence-of-substring checks vacuous for a deleted file. At `:861` a
   `doctor --fix` run that removed `hooks.json` entirely now passes `assertStalePrunesApplied` (nil bytes
   contain nothing, and the stdout breadcrumb check still holds); previously it fataled on the read. At
   `:923` the same for `projects.json` in `assertDownServerDeferral`. That is the wipe-regression failure
   mode this whole work unit exists to catch. (The projects half at `:866` is unaffected — the positive
   `liveDir` check at `:870` still fires; `:1566-1582` is unaffected because both halves are compared.)
   FIX: at those two sites add the precondition the file must exist, e.g.
   `if len(hooksAfter) == 0 { t.Fatalf("hooks.json is absent or empty after the prune — the file itself was removed") }`
   immediately after each read, keeping the shared `readFileBytes` (so the both-halves-through-a-helper
   criterion still holds).
   ALTERNATIVE: give `cmd` a second fatal-on-absent read helper for after-reads that must exist and use
   it at these two sites. Cleaner semantics, but it reintroduces two read helpers in one package for a
   two-site need — the precondition is preferred.
   CONFIDENCE: medium

NOTES:
- Your `t.Fatalf` → `t.Errorf` deviation at
  `cmd/doctor_fix_transient_listpanes_shared_integration_test.go:74` is **acceptable at that site**.
  Nothing after the assertion depends on hooks.json being intact — the remaining checks read portal.log
  and each carry their own `Fatalf` — so the only change is that a wipe regression now reports alongside
  whatever the log assertions say rather than aborting first. The `spec.name` and the "wipe regression
  has returned" wording are both carried through the context argument, and the `len(before) == 0`
  precondition above it is retained. Verified the subtests still execute and pass under the integration
  lane.
- Your reasoning for declining a projects.json read counterpart **holds up against the tree**:
  `hookstest` is documented as hooks.json scaffolding, `internal/hooks` never reads projects.json, and
  `cmd`'s `readFileBytes` was already path-generic. A `hookstest.ProjectsFileBytes` would have been
  mis-homed. All three pairs now read both halves through it.
- Task 7-20 will delete `cleanstale_snapshot_test.go`'s "it still retains a non-token-shaped key"
  subtest, so one of this task's nine conversions is transient. Harmless, but it is why issue 1 matters
  rather than being cosmetic.
- `cmd/bootstrap/transient_listpanes_helpers_integration_test.go:134` does compare two hooks.json reads
  with `bytes.Equal`, so your "none remain anywhere" claim is literally inexact. It is correctly **not** a
  conversion target: it asserts `HooksJSONBytes` is deterministic across two reads with no route in
  between, not that a route wrote nothing, and the shared assertion's wording would misdescribe it.
- `TestReadFileBytes` sits in `testhelpers_test.go` directly beneath the helper it pins. Slightly unusual
  placement, but it keeps the rule beside the rule's implementation — leave it.
