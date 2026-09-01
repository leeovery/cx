## Attempt 1

ISSUES:
- `internal/sourceguardtest/packagedeps_test.go:20-26` — the transitivity assertion is vacuous, and the
  comment justifying it is false. `io/fs` is a **direct** import of `internal/sourceguardtest`
  (`gosourcefiles.go:5`), not something that "arrives only through `path/filepath`'s walk":
  `go list -f '{{join .Imports "\n"}}' ./internal/sourceguardtest` returns
  `fmt go/ast io/fs os os/exec path/filepath strings`. Both asserted packages are immediate imports, so the
  test asserts nothing about depth.

  The reviewer demonstrated it: degrading `PackageDeps` to
  `exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Imports \" \"}}", pkg)` — package plus
  immediate imports only — leaves **both** `TestPackageDeps_*` tests PASSING.

  Why it matters: `PackageDeps` is now the single dependency enumerator behind four guards, three of which
  police *transitive* reach — the hooks guard's "it drags in no session, tmux or state tree transitively",
  prefs' "transitively imports", and theme's xdg check. The reviewer's planted `nanoid → xdg` edge is
  exactly that class and is invisible to an immediate-import view. If the shared primitive ever lost
  transitivity, three guards would silently stop catching their violation and this — their only coverage of
  the primitive — would stay green. That is the "reports safety it isn't providing" failure the task's fifth
  criterion exists to prevent.

  FIX: assert a genuinely transitive-only dependency. Change line 22 to
  ```go
  for _, want := range []string{"go/ast", "go/token"} {
  ```
  `go/token` is reached only through `go/ast`, and that edge is definitional rather than incidental, so it
  will not drift. Replace the false comment at lines 20-21 with something true, e.g. `// go/ast is imported
  directly; go/token arrives only through it, so the pair distinguishes a transitive list from an immediate
  one.` Verified by the reviewer: passes against the real `PackageDeps`, and fails against the degraded one
  with `PackageDeps(...) omits go/token: [... fmt go/ast io/fs os os/exec path/filepath strings]`.
  ALTERNATIVE: `context` (reached only via `os/exec`) works identically and was verified present in the deps
  set but absent from the imports set. `go/token` is preferred because the `go/ast` → `go/token` edge is
  definitional, whereas `os/exec`'s use of `context` is an implementation detail that could change.
  CONFIDENCE: high

NOTES:
- All six converted guards were re-plant-verified independently, in a `.git`-free scratchpad copy of the
  tree: each failed on the violation it exists to catch, including the transitive-only `nanoid → xdg` edge
  that only the hooks transitive subtest can see. Prefs' anti-vacuity arm was inverse-tested too (swapping
  `fileutil.AtomicWrite` for an `os.WriteFile` closure fires `the leaf guard may be vacuous`).
- **Your fset claim is true and the deviation is justified.** The `store.go:113:9` diagnostic is what the
  mutations guard reported before the refactor; dropping the `*token.FileSet` would have regressed it to the
  bare filename the `CleanStale` guard uses.
- **Both signature deviations were judged justified and the reviewer would not change them.**
  `sourceguardtest.TestingT` is the only way the prescribed unresolvable-package test is writable without a
  subprocess, and CLAUDE.md already records the `logtest.TestingT` precedent. The added `dir` parameter is
  what makes the empty-scan test possible, and both call sites pass `"."`, so guard behaviour is unchanged.
- **Your `go list -deps` reasoning holds**, and it was checked rather than assumed: `go list -deps` on each
  of the four leaf packages returns 0 matches for `sourceguardtest`, with internal deps exactly
  `{fileutil, log, nanoid, storelog}` (hooks), `{fileutil}` (prefs), `{}` (nanoid), `{log}` (theme).
- The empty-scan and fatal paths were mutation-tested and are not vacuous: removing the `Fatalf` from
  `scanPackageCalls`' enumerate-error branch fails its test, removing it from `PackageDeps` fails its test,
  and mutating each of `CalleeName`'s three branches independently fails exactly one distinct test each.
- Both lanes green under the reviewer's own forced-fresh runs (`cmd` 415.9s, `cmd/bootstrap` 54.2s,
  `internal/restore` 35.4s), `golangci-lint` 0 issues, protected tmux server and daemon confirmed untouched.
- Two observations left as taste calls, no change asked: `guardT` is byte-for-byte
  `sourceguardtest.TestingT` which the same file already imports, and `recordingT` / `scanRecorderT` are the
  same stub authored twice. Declaring the interface at the consumer is idiomatic Go — but a task about
  removing duplicated guard scaffolding re-introducing a little of it is worth a second thought. Your call.
- The `scanned == 0` fatal (`cleanstale_staleness_guard_test.go:82-84`) is genuinely unreachable —
  `PackageGoFiles` errors on an empty match, so `err == nil` implies at least one path and the loop has no
  `continue`. The empty-scan test therefore exercises the *enumerate-error* fatal. Correctly kept (the task
  asked for it), but the doc comment's "parsing nothing … [is] fatal" describes a path that cannot be taken.
