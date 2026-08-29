## Attempt 1

ISSUES:
- internal/nanoid/leaf_guard_test.go:16-25 — the test is named TestNanoIDPackage_DependsOnTheStandardLibraryAlone and its failure message says "the id vocabulary is a stdlib-only leaf", but the assertion only rejects deps under github.com/leeovery/portal/. A third-party import would pass the guard while falsifying both the test's name and the package doc at nanoid.go:1-6 ("It depends on the standard library alone"), and while breaking acceptance criterion 2. The guard as written enforces "no module edge", not "stdlib only".
  FIX: replace the module-prefix test with a stdlib test that subsumes it — a dependency whose first path segment contains a dot is not stdlib:
      root, _, _ := strings.Cut(dep, "/")
      if strings.Contains(root, ".") { t.Errorf(...) }
  Verified against live output: the only dotted first segment today is github.com (the package itself, already skipped), and stdlib-vendored paths arrive as vendor/golang.org/... whose root is vendor — so there are no false positives.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- internal/session/naming.go:12 — the diff introduced this doc comment (the type carried none before) and it narrows the alias to session naming, but cmd/hooks.go:45 types the pane-token minter TokenMinter session.IDGenerator, where what is minted is a pane token, not a session-name suffix.
  OLD: // IDGenerator mints the nanoid suffix a generated session name ends in.
  NEW: // IDGenerator mints one opaque nanoid per call; a generated session name ends in one.

NOTES:
- The specification's §3.2 argument (lines 125-129) now describes a placement the codebase no longer has, including "internal/session is the only home that permits the derivation" and the conclusion that no guard test is needed. Expected consequence of an analysis-cycle override, not a defect in this task.
- The task text's "both stay unexported outside the leaf" reads literally as also unexporting the alphabet. Keeping Alphabet exported is the correct call: internal/spawn's isOptionSafeID and internal/transienttest's key seeder both consume the charset directly.
- Optional, no change requested: cmd/hooks.go:45,97 could type its TokenMinter as nanoid.Generator directly rather than session.IDGenerator, since the pane-token minter has nothing to do with session naming.
