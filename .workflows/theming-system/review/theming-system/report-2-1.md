TASK: theming-system-2-1 — Embed The Built-In Theme Set And Ship tokyo-night.theme

ACCEPTANCE CRITERIA:
1. `LoadBuiltin("tokyo-night")` returns no rejection, `found == true`, and a `Theme` whose 19 tokens carry §7.3's dark values uppercase-canonical (`Canvas #0B0C14`, `BgSelection #28243A`, `BgSubtle #26283A`).
2. `LoadBuiltin("no-such-theme")` returns `found == false` with a nil rejection, touching no filesystem; signature takes one string and no directory (proven at compile level and with `PORTAL_THEMES_DIR` pointing at a nonexistent path).
3. `BuiltinBytes("tokyo-night")` is byte-identical to the committed file, comments and trailing newline included.
4. `BuiltinSlugs()` derives from the embedded filenames, returns them sorted, contains `tokyo-night`.
5. `parseThemeBytes` on deliberately broken bytes returns an ordinary `*Rejection` — never a panic — for `bad syntax`, `bad colour`, `missing tokens`.
6. `tokyo-night.theme` contains exactly 19 `key = value` lines, no `border.footer`, and a header comment; parses identically through `LoadFile`.
7. Source guards pass: no hex literal in non-test `internal/theme` Go source, no `func init()`.
8. `internal/tui/theme` untouched (phase boundary), `go build ./...` / `go test ./...` green, `cmd/capturetool`'s import guard still passes.

STATUS: complete

SPEC CONTEXT:
§7.1 decides a built-in *is* a `.theme` file, `go:embed`ed and parsed by the **same loader** as a stranger's drop-in — one code path, one format, one validity rule; the named consequences are that parse failures move from compile time to load time (hence §7.6's build-time guarantee, task 2-8) and that `internal/capture`'s no-real-config import guard survives because `go:embed` is not config discovery. §7.3 gives the dark table verbatim (19 values, `canvas #0b0c14` and `bg.selection #28243a` written lowercase; `border` takes the former `border.separator` value and `border.footer` is dropped per §2.2). §7.1 also rules that MV's inline erratum comments are deleted rather than ported, so this dark file carries attribution only — the eyeball-pin notes belong to the light file (task 2-5). §7.6 requires that nothing walk the embedded set at init.

IMPLEMENTATION:
- Status: Implemented (with two later-phase amendments, both intentional — see Notes)
- Location:
  - `/Users/leeovery/Code/portal/internal/theme/builtins.go:23-24` — the single `//go:embed builtins/*.theme` directive over `var builtinFS embed.FS`.
  - `/Users/leeovery/Code/portal/internal/theme/builtins.go:30-36` — `BuiltinBytes(slug) ([]byte, bool)`, path composed from `builtinDir + "/" + slug + FileExtension`; a miss is not-found, never an error.
  - `/Users/leeovery/Code/portal/internal/theme/builtins.go:41-55` — `BuiltinSlugs()` reads the embedded dir, strips `FileExtension`, then sorts (order deliberately applied *after* stripping, since stripping can invert two names, e.g. `a-b.theme` < `a.theme` but `a` < `a-b`).
  - `/Users/leeovery/Code/portal/internal/theme/builtins.go:71-86` — `(Loader) LoadBuiltin(slug) (Result, *Rejection, bool)`; skips the ladder's filename rungs, delegates to `resultFromBytes`.
  - `/Users/leeovery/Code/portal/internal/theme/load.go:59-66` — `resultFromBytes` (the shared content tail, populating `Result.Source` with the verbatim bytes) and `/Users/leeovery/Code/portal/internal/theme/load.go:106-112` — `parseThemeBytes` = `lexPairs` → `themeFromPairs`.
  - `/Users/leeovery/Code/portal/internal/theme/load.go:47-53` — `Result.Source []byte`, documented nil-on-rejection; consumed by `portal theme export` at `/Users/leeovery/Code/portal/cmd/theme.go:62`.
  - `/Users/leeovery/Code/portal/internal/theme/builtins/tokyo-night.theme` — 19 keys, `# Tokyo Night — <upstream URL>` header, `border = #292E42`, no `border.footer`.
  - `/Users/leeovery/Code/portal/internal/theme/leaf_guard_test.go:69-128` — the two source guards (`TestThemePackage_DeclaresNoHexLiterals`, `TestThemePackage_HasNoInitFunction` + `requireNoCallingInitialiser`), enumerating via `sourceguardtest.PackageGoFiles(".", false)`, which errors rather than passing vacuously on an empty match.
- Verification against each criterion:
  1. MET. Every value in the file matches §7.3's table exactly (spec lines 537-546); `applyPairs` (`validate.go:55`) upper-cases the stored value, so `#0b0c14`/`#28243a` land canonical. Pinned by `builtins_test.go:130-174`.
  2. MET. `LoadBuiltin` reads only `builtinFS`; compile-level proof is the type assertion at `builtins_test.go:208` (`var _ func(string) (theme.Result, *theme.Rejection, bool) = theme.Loader{}.LoadBuiltin`), behaviour at `builtins_test.go:210-224` with `PORTAL_THEMES_DIR` set to a nonexistent path. The package-level "resolves no paths" guard (`leaf_guard_test.go:23-67`, incl. the `go list -deps` check that `internal/xdg` is absent) backs it structurally.
  3. MET. `builtins_test.go:42-80`, comparing against `os.ReadFile` of the committed file, plus a fresh-copy mutation sub-test and an unknown-slug sub-test.
  4. MET. `builtins_test.go:82-94` compares against a runtime `os.ReadDir` of `builtins/`, so a new file enrols with no Go edit.
  5. MET. `load_internal_test.go:8-57` corrupts the shipped bytes three ways (duplicate key / `blue` value / truncation) and asserts reason, non-empty detail and a zero `Theme`. The no-panic property is additionally enforced package-wide by `embedded_test.go:160-207`.
  6. MET. Key count is asserted against `len(theme.TokenNames())` and the key *set* against the closed vocabulary (`builtins_test.go:22-40`), with an explicit `border.footer` assertion and a header-comment assertion. `LoadFile` parity is `builtins_test.go:176-206`.
  7. MET. Confirmed independently: `grep -nE '#[0-9a-fA-F]{6}' internal/theme/*.go` (non-test) returns nothing, and the package declares no `func init()` and no calling package-level var initialiser.
  8. MET as amended. `cmd/capturetool`'s guards (`cmd/capturetool/import_guard_test.go`) are intact and `internal/capture` reaches the built-in set through the loader with no directory argument. `internal/tui/theme` no longer exists — Phase 3 deleted it as planned; the "untouched" clause was a phase boundary at authoring time, so this is amendment, not drift.
- Notes (later-phase amendments, all intentional):
  - `Loader.BuiltinSource` (`load.go:18-21`, used by `builtins.go:81-86`) is a later seam for staging the otherwise-unreachable broken-binary state (`broken_builtin.go`); it defaults to the embedded set, so 2-1's "no second parse path" property is unchanged.
  - `DefaultDarkSlug` / `DefaultLightSlug` and `builtinSlugSet()` were added by tasks 2-2/5-x and now live in this file; both are additive.
  - `LoadPath` (`load.go:94-101`) reuses the same `resultFromBytes` tail, so the "exactly one parse path" invariant still holds across all three entry points — verified by grep: `lexPairs`/`themeFromPairs` have exactly one caller each, `parseThemeBytes`.

TESTS:
- Status: Adequate
- Coverage: All eight acceptance criteria have a directly corresponding assertion (mapped above). Beyond them, the shared-parse-path test compares `LoadBuiltin` and `LoadFile` output for the *same* bytes and checks both `Source` payloads, so a divergent second path cannot pass; `BuiltinSlugs` is compared against a live directory read rather than a restated literal, so the derivation property (not just today's answer) is under test; the guards enumerate the package rather than naming files, and `PackageGoFiles` errors on an empty match so neither guard can pass having stopped looking.
- Notes:
  - Not over-tested. `wantTokyoNightTokens` (`builtins_test.go:130-150`) restates all 19 shipped values in Go, which looks like duplication of the `.theme` file but is the point: it is the pin that makes an accidental palette edit fail loudly, and it is expressed in canonical (upper-case) form so it also proves the canonicalisation. The three `BuiltinBytes` sub-tests each cover a distinct property (identity, copy semantics, miss).
  - Not under-tested for this task's edges: canonicalisation, the not-found-vs-rejection distinction, comment/trailing-newline fidelity, and the three rejection reasons are all covered. Embedded-set-wide validity and fallback resolvability are deliberately task 2-8's (`embedded_test.go`) and are present.
  - Test-name mapping (informational, not a defect): the plan's `TestParseThemeBytes_BrokenInputIsARejectionNotAPanic` ships as `TestEmbeddedParseFailureIsAnOrdinaryError` (`load_internal_test.go:8`) — same three cases, same intent; the rename is consistent with the later comment/vocabulary scrub tasks.
  - Lane placement is correct: all tests are unit-lane, hermetic, spawn no daemon and exec no portal binary (the only subprocess is `go list` in the leaf guard, matching existing repo guards).

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays a leaf that resolves no paths and hardcodes no slugs (CLAUDE.md's `theme` row); colour values live only in `.theme` files, which is exactly the precondition CLAUDE.md records for the `internal/tui` colour-literal guard having no exemption. No `panic`/`os.Exit`/`log.Fatal` on the load path — a broken embedded file is an ordinary `*Rejection`, matching §7.6's "escalation happens where a fallback is needed".
- SOLID principles: Good. One reason to change per unit: `lex.go` lexes, `validate.go` validates, `load.go` owns entry points and the shared tail, `builtins.go` owns embedding. `LoadBuiltin`'s three-value `(Result, *Rejection, bool)` return keeps "not a built-in" distinct from "a broken built-in", which is what lets by-name resolution (`resolve.go:32`) fall through without inspecting an error's contents.
- Complexity: Low. Every function in `builtins.go` is straight-line with a single branch; the deepest control flow in the task's code is `applyPairs`'s single loop.
- Modern idioms: Yes — `slices.Sort`, `strings.TrimSuffix`, `embed.FS`, `strings.Cut` in the shared lexer. No reflection, no `init()`.
- Readability: Good. Comments explain *why* (the post-strip sort ordering, why the filename rungs are skipped for a built-in, why `Source` is bytes rather than a re-serialisation) rather than restating the code, and the later scrub commits removed spec-section and task citations, so nothing points at workflow artefacts.
- Comment accuracy: Verified line by line against the code. Two claims worth pinning explicitly, both true: `builtins.go:28-29` ("embed matches exactly and rejects `..`") holds because `embed.FS.lookup` runs `fs.ValidPath`, which rejects any `..` element; `builtins.go:44` ("Unreachable: go:embed fails the build when its pattern matches no file") is correct, and the nil return would surface loudly through `TestEmbeddedSetIsNonEmpty` rather than silently. The one inaccuracy found is the `# Surfaces.` grouping header, noted below.
- Security: N/A-adjacent and clean — the only input is embedded bytes and a slug used solely as an exact FS key; no path traversal is reachable (above), no filesystem or env access.
- Performance: Fine. `BuiltinSlugs()` walks 3 embedded entries and is called once per `NewLoader`; §7.6 explicitly forbids hoisting this to init, so the per-construction cost is the deliberate trade.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/builtins/tokyo-night.theme:23 — the `# Surfaces.` group header also covers `text.on-attention = #E8C9A0` (line 29), which is a foreground token, so the heading is falsified by its own last line. Replace the header with `# Surfaces, and the text drawn on the attention surface.` The identical shape exists in `internal/theme/builtins/tokyo-night-day.theme:93` and `internal/theme/builtins/nord.theme:73`; apply the same replacement to all three so the built-ins stay uniform.
- [quickfix] internal/theme/builtins_test.go:16-20 — this file declares `builtinsDir` and `tokyoNightSlug` but calls `builtinPath`/`readBuiltinFile`, which are declared in `internal/theme/builtins_tokyo_night_day_test.go:135-148`, which in turn reads `builtinsDir` from here. The two per-theme test files are mutually dependent, so deleting or retiring the day-theme test file breaks the dark-theme tests for no related reason. Move `builtinsDir`, `builtinPath` and `readBuiltinFile` into a shared `internal/theme/builtins_shared_test.go` and leave each per-theme file holding only its own assertions.
