## Attempt 1

ISSUES:
- `cmd/doctor_test.go:394-395` — `TestDoctorRejectsArgs` still hand-installs and restores `doctorDeps`
  and hand-drives `resetRootCmd()` + `rootCmd.SetArgs([]string{"doctor", "unexpected"})` (`:396-397`), so
  the install/restore pair the task set out to single-source still has two homes and the criterion is
  unmet. The task's stated outcome — one place decides how a doctor run is isolated and driven — is two
  places while this site survives.
  FIX: replace the body from `:394` to `:400` with a single delegation, deleting the manual install,
  cleanup, `resetRootCmd` and `SetArgs`:
  ```go
  if _, _, err := runDoctorWith(t, &DoctorDeps{StateDir: dir}, "unexpected"); err == nil {
      t.Error("Execute returned nil for `doctor unexpected`; want a NoArgs error")
  }
  ```
  This is behaviour-preserving: the composed argv is identical, the deps value is identical (deliberately
  *not* wrapped in `withHealthyRuntime`, matching today), the assertion and its message are unchanged, and
  the two added buffers simply replace the throwaway buffer `resetRootCmd` already installs. It also picks
  up `isolateTerminalsFile`, which this site currently lacks — harmless today only because `cobra.NoArgs`
  rejects before `RunE` and `resolveDoctorDeps` never runs.
  ALTERNATIVE: leave it as-is on the reading that the criterion's "install/restore" clause scopes to the
  driver helpers and this test drives cobra's arg validator rather than a doctor run (it never reaches
  `RunE`). Defensible, but it leaves a second site that a future change to how deps are installed would
  have to remember — the exact failure mode the task cites. The conversion is recommended.
  CONFIDENCE: high (on the mechanics; the scope judgement is the arguable part, hence the alternative).

NOTES:
- Your "all three collapsed cleanly" claim was verified and is honest — nothing was contorted. The two
  drivers were genuinely byte-identical apart from the argv element, and `runDoctor`'s old body was
  byte-identical to `runDoctorCmd`'s apart from the deps expression, so the two-line wrapper loses
  nothing.
- One ordering change was checked specifically: `runDoctor` previously called `isolateTerminalsFile(t)`
  **before** building its deps; the wrapper now evaluates `withHealthyRuntime(&DoctorDeps{StateDir: dir})`
  first. Safe — `withHealthyRuntime` only fills nil struct fields and assigns `doctorUnsupportedResolve`
  as a value rather than calling it, so it performs no env read and no config I/O. The other 45 call sites
  already evaluated their deps expression before the helper body under Go's argument-evaluation order.
- The conversion was verified mechanically: applying the transform to the HEAD versions of all 11 files
  reproduces the working tree exactly. 45 converted call sites confirmed. Baseline from `git archive HEAD`
  vs the working tree: 1611 verdict lines each, 0 FAIL, sorted verdict-name sets byte-identical.
- The retained comment about `terminals.json` being read eagerly is accurate and the isolation is
  load-bearing: `resolveDoctorDeps` (`cmd/doctor.go:74-76`) builds the production spawn seams — which load
  `terminals.json` — unconditionally, before it consults the injected `doctorDeps`.
- The three seeding calls at `cmd/doctor_test.go:390-392` are inert for `TestDoctorRejectsArgs`, since
  `cobra.NoArgs` rejects before `RunE`. Noted only so the conversion above is not mistaken for having
  dropped something load-bearing.
