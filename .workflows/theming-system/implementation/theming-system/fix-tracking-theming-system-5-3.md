## Attempt 1

ISSUES:

- `/Users/leeovery/Code/portal/internal/theme/resolve_test.go:151-205` — `TestResolveByName_BuiltinNeverReadsDirectory` does not discriminate the ordering it is named for, and its doc comment (lines 162-166) claims two proofs it does not deliver:
  (a) "with no `directory unusable` record, that it was never even stat'd" — false. A 0000 directory stats perfectly well; `resolve.go:78-81` says so itself. No record is emitted whether or not the directory was stat'd.
  (b) the shadowing fixture "separates 'never reads it' from 'reads it, then prefers the built-in'" — false. `LoadFile`'s reserved-name rung (`load.go:141-143`) refuses a `<builtin>.theme` file *before* reading it, so both orderings answer identically. (The comment also calls the fixture "valid-but-different"; it is a broken file.)

  Empirically: replacing the body with "try the directory first; on any rejection fall through to `LoadBuiltin`" passes the entire `internal/theme` suite. §8.4's "never reads the themes directory at all" is a cost and no-config property (§5.7, capture's import guard) that can therefore regress silently.

  FIX: add a fourth subtest whose directory state is itself a rejection — the one built-in-slug case with an observable side effect, since `statThemeDir` emits there:

  ```go
  t.Run("with a regular file where the themes directory belongs", func(t *testing.T) {
      loader, sink := resolveLoader(t)
      dir := writeFile(t, t.TempDir(), "themes", "this is not a directory\n")
      for _, slug := range requireBuiltins(t) {
          result, rejection := loader.ResolveByName(slug, dir)
          requireBuiltinResolved(t, result, rejection, slug)
      }
      requireNoThemeRecords(t, sink)
  })
  ```

  The reviewer verified this exact subtest: it FAILS under the directory-first mutation ("emitted 1 records… directory unusable") and PASSES against the implementation as written. Also correct the two false claims in the doc comment so the reasoning matches what the fixtures prove.

  ALTERNATIVE: a structural assertion that `ResolveByName`'s body reaches `LoadBuiltin` before `loadFromThemesDir` (the file already has an AST walker). Tradeoff: it pins ordering directly but is blind to behaviour and duplicates machinery. The fixture is behavioural, three lines, and reuses existing helpers — the reviewer recommends the fixture.

  CONFIDENCE: high

- `/Users/leeovery/Code/portal/internal/theme/resolve.go:144-146` — `narrowReadFailure` attributes *every* non-ENOENT `Lstat` failure to the directory, not just a denied lookup. A charset-valid but over-long slug (§5.2 fixes no length bound, and `resolve_test.go:572` deliberately exercises a 200-char one) fails both syscalls with ENAMETOOLONG, so `ResolveByName(strings.Repeat("x",300), dir)` emits `theme: directory unusable path=<dir> reason=unreadable` against a perfectly healthy directory — confirmed against the real code. §5.5 defines that event as "unreadable, or a regular file where a directory belongs"; this is neither. It also consumes the `path`+`reason` dedup slot, so a genuine unusable-directory record from Phase 8's enumeration in the same process would be suppressed.

  FIX: narrow the branch to the condition deviation #2 exists for — a denied lookup:

  ```go
  if errors.Is(err, fs.ErrPermission) {
      return l.reportDirectoryUnusable(themesDir, rejection)
  }
  return rejection
  ```

  A 0000 directory's `Lstat` returns EACCES (verified), so the approved deviation keeps working unchanged; everything else stays `unreadable` with the read's OS error verbatim, which is what the Do-list asks for. Add the long-slug case to `TestResolveByName_AbsentFileIsNotFound` asserting `unreadable` plus `requireNoThemeRecords`.

  ALTERNATIVE: leave as-is and treat "the name could not be resolved under the directory" as the directory's fault. Tradeoff: simpler, but it puts a spec-defined WARN on a state the spec does not define it for and risks masking the real one via dedup. The reviewer recommends narrowing.

  CONFIDENCE: medium

NOTES:

- Full unit lane green (`go test ./...`), `go vet` clean, `gofmt -l` clean on the changed packages, `golangci-lint run ./internal/theme/... ./cmd/...` → 0 issues.
- Both approved deviations are implemented correctly; the reviewer verified their premises against the OS (a 0000 dir stats clean; lookups inside it return EACCES for present and absent names alike).
- SPEC_CONFORMANCE conformant: §8.6 charset-before-composition, §8.4 embedded-set-first, §5.5's three directory states, §5.7's one-read-no-ReadDir budget, §12.3's dir-anchored dedup identity, and §12.1's four export frames are all implemented as specified. The loader still resolves no path (`themesDir` stays injected), and `cmd/theme.go`'s `unlocatableAsUnreadable` fold is exact: `themesDirPath` returns `("", err)` (`cmd/config.go:185-196`), so `not found` + non-nil dirErr is unambiguously the unlocatable state.
- ARCHITECTURE sound: extracting `statThemeDir` so §5.5's directory-state table is stated once and shared by enumeration and the by-name read is the right call — it is precisely what makes §12.3's cross-surface dedup structurally possible rather than coincidental. The three helpers (`notFound` / `narrowReadFailure` / `reportDirectoryUnusable`) each hold exactly one decision, and routing both unusable-directory sightings through one emitter is what pins the record's path identity. Export's re-point plus the AST guard genuinely closes the drift risk.
- Mutation results (all run against the real suite, zero repo writes): drop `ValidSlug` → FAILS CharsetCheckedBeforePathComposition ✓; decide ENOENT from the read err → FAILS AbsentFileIsNotFound/dangling symlink ✓; report the record against the composed file path → FAILS UnusableDirectoryIsUnreadable + Deduped ✓; sneak an `os.ReadDir` in → FAILS NoReadDirAndSingleRead ✓; add a second `os.ReadFile` → FAILS NoReadDirAndSingleRead ✓; reintroduce `filepath.Join` in `cmd/theme.go` → FAILS UsesSharedByNameResolver ✓; directory first, fall through to the embedded set on reject → WHOLE SUITE PASSES ✗ (issue #1).
- The `themesDir == ""` early return (`resolve.go:83-85`) is behaviourally redundant today — `os.Stat("")` is ENOENT, so `statThemeDir` already answers "absent" and composes no path. Removing it fails no test. Keep it: it is cheap, documents task 5-7's contract, and guards the platform assumption. Just be aware the "an empty directory string composes no path" subtest cannot distinguish the guard from the stat.
- `TestResolveByName_ContentReasonsPassThrough` asserting against `LoadFile`'s own answer (rather than a restated string) is the right shape — it defeats a resolver that rebuilds the rejection, which a reason-only assertion would not.
- The cmd-side AST guard is well-chosen but bans the bare identifier `Join` file-wide; a future unrelated `strings.Join` in `cmd/theme.go` would trip it. Acceptable given the file's size and single purpose — noted so the failure is legible if it ever fires.
- CONVENTIONS followed: doc-comment style matches the package ("pins §…"), gofmt/vet/lint clean, no `t.Parallel()`, temp-dir-only fixtures, root skips + chmod cleanups on both permission fixtures, assertions compare whole values rather than substrings.
