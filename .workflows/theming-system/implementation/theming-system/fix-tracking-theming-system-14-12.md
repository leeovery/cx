## Attempt 1

ISSUES:
- `internal/theme/embedded_test.go:309,349-368` — the panic ban is relaxed from "no `panic` in any production file of `internal/theme`" to "no `panic` outside any `New`-prefixed function". Four production constructors are exempted (`NewLoader`, `NewSilentLoader`, `NewEventLogger`, `NewRawKeys`) plus every future `New*` and every func literal nested inside one, where exactly ONE exemption is required. `inConstructor`'s own doc argues the narrowness ("a constructor is handed WIRING and never a theme") — but the check tests a NAME PREFIX, so the property it claims is unenforced: a later `NewBuiltinSet`/`NewResolution` that reads or parses could carry the `panic("broken built-in")` the guard's rationale paragraph (lines 285-288) names as the standing temptation, and pass.
  FIX: narrow the predicate to the one constructor — replace `strings.HasPrefix(fn.Name.Name, "New")` with a comparison against a named constant (e.g. `const nilSeamConstructor = "NewLoader"`, matched with `fn.Name.Name == nilSeamConstructor`), and rewrite `inConstructor`'s doc to state what it now enforces: the single wiring precondition in `NewLoader`, with every read/parse/resolve path still reporting a `*Rejection` or a returned error. Rename to something like `inNilSeamConstructor` so the helper name carries the narrowing.
  ALTERNATIVE: keep the AST scan strict and exempt by MESSAGE instead — allow a panic only where the string contains `NewSilentLoader`. That binds the exemption to the specific fault rather than to a location, but couples a structural guard to copy that may be reworded; the name-match is the more stable pin and is recommended.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/theme/resolution_test.go:30-36` — the doc directly above the changed line still describes a nil seam and claims to stand as proof it is safe at TUI construction; the loader is now discard-backed and no production path carries a nil seam.
  OLD:
  ```
  // The seam is NIL — the silent one — because these fixtures assert the RESOLUTION
  // RECORD, and a resolution now emits `theme: loaded` and `theme: fallback applied`
  // per slot. Those events are asserted where they belong, against a capturing seam
  // in events_test.go; a loader here that emitted into the process handler would
  // only add noise to assertions about the record's shape. It doubles as the
  // standing proof that the nil seam is safe on the path every TUI construction
  // runs.
  ```
  NEW:
  ```
  // The seam is SILENT — the discard-backed one — because these fixtures assert the
  // RESOLUTION RECORD, and a resolution emits `theme: loaded` and `theme: fallback
  // applied` per slot. Those events are asserted where they belong, against a
  // capturing seam in events_test.go; a loader here that emitted into the process
  // handler would only add noise to assertions about the record's shape.
  ```

NOTES:
- WHICH surfaces emit is genuinely UNCHANGED, verified independently: `cmd/doctor_theme.go:135`, `cmd/theme.go:116` and `cmd/capturetool/main.go:123` all still take `NewSilentLoader()` and were not in the diff; `cmd/open.go:756` is still the only emitting production loader. The one behavioural delta (the TUI seed) was silent before via nil-seam early return and is silent after via discard seam — and structurally silent regardless, since `LoadBuiltin` emits nothing on any path. Silence did not move; it was re-labelled.
- Panicking IS the right call and is unreachable in production: the type-level route is genuinely unavailable (`EventLogger` carries a `sync.Mutex` + shared dedup map, so a value parameter would copy a lock), and the only production caller passes `theme.NewEventLogger(themeLogger)` with `themeLogger` bound at package init and `NewEventLogger` never returning nil.
- Guard 1 (`TestEvents_DiscardSilencesResolution`) is JUSTIFIED, no hole: at HEAD it pinned three silence shapes, one of which was the exact premise this task removes. The nil-`*EventLogger` silence coverage survives via `theme.Loader{}`; the removed non-panic half is now inverted and pinned by `TestNewLoader_NilSeamPanics`. Its vacuity guard still bites (all four entries produce `wantFallbacks = 3`, verified).
- `TestDefaultDarkTheme_SeedsTheShippedPaletteSilently` cannot fail for the reason it appears to test — `LoadBuiltin` emits nothing under any seam, and the non-vacuity control exercises `Enumerate` rather than the seed path. This is the shape the task prescribed and there is no reachable regression it misses; worth knowing it documents intent rather than fencing it.
- `TestNewSilentLoader_ReservesEveryBuiltinSlug` duplicates `TestReservedSet_CoversEveryBuiltinSlug` with a different constructor. Task-mandated and it pins something real (a future `NewSilentLoader` that stopped delegating would drop reservation), so the duplication is earned.
- Cosmetic report inaccuracy: `grep "NewLoader(nil)"` returns three lines in `silent_loader_test.go` (the call plus two in the doc and failure message), not one.
- `load.go:62-65` and `silent_loader_test.go:78-80` phrase the invariant as "one named route… one grep", mildly overstated while `theme.Loader{}` remains a silent test shape. The `ReservedSlugs` doc immediately above qualifies it — leave both as they stand.
