## Attempt 1

ISSUES:

- `internal/tui/restore_divergence_test.go:329-332` — the empty capture is dropped from the file with the claim *"The empty-capture fall-through is unchanged by a swap … the exit path returns on an empty original before it compares anything"*, delegating to `background_restore_test.go:209`, which never swaps (`New(fakeLister{})`). **The claim is false.** The exit path is not the only thing that can change whether the original is empty — the *swap* path can populate it.

  Mutant **M7** (`ApplyTheme` also setting `m.originalBg = th.Canvas.Value` — the "refresh the captured background on swap" edit) **passes the whole `internal/tui` package including this file**, and its production consequence is exactly §11.4's named harm: `RestoreTerminalBackground` then writes `\e]11;#2E3440` after Bubble Tea's OSC 111 reset, leaving the *previewed* Nord canvas — a colour the user never chose — stuck after Portal exits.

  This is the single acceptance-criterion item omitted, and it is the one whose post-swap form is non-redundant.

  FIX: add a named test observing the empty capture **through** the swap. The original must **not** be handed in after the swap (`withCapturedOriginal(m, "")` would overwrite the regression's fingerprint and re-open the hole). The reviewer verified this shape passes on clean production and fails under M7 with `wrote "\e]11;#2E3440", want nothing`:

  ```go
  func TestRestoreBackground_EmptyCaptureAfterSwapWritesNothing(t *testing.T) {
      m := startupModel(t, testDarkTheme(t))
      if got := m.OriginalBackground(); got != "" {
          t.Fatalf("probe setup: captured original = %q before any reply, want empty", got)
      }
      m.ApplyTheme(testNordTheme(t))
      assertSkipped(t, m)
  }
  ```

  Then correct the comment at `:329-332` — "unchanged by a swap" must become an assertion, not a claim. Also note `assertSkipped`'s message says "an echo of the STARTUP canvas", which wants a one-line generalisation or a sibling `assertNothingWritten` so the empty case's failure reads honestly.

  CONFIDENCE: high

- `internal/tui/restore_divergence_test.go:162-182` — `TestRestoreBackground_CommittedThemeDivergence` asserts no premise that the active theme actually diverged. Measured with **M6** (`ApplyTheme` a no-op): case 1 stays **fully green** while testing no divergence at all — it degrades into a duplicate of `restore_test.go`'s basic skip/emit pair.

  Its two siblings both guard this (`:214-216` *"or there is no divergence to test"*, `:261-266`), as do `startupModel` (`:85`), `testNordTheme` (`:63`) and `newSwapFrameModel`'s probe-setup Fatal — so this is the file's own standard, unmet in the first of the two cases the task exists to deliver. (M6 is caught elsewhere in the package by the restyle tests, but an edit to line 163 handing both variables the same theme would not be caught anywhere.)

  FIX: after `m.ApplyTheme(committed)` at `:166`, add the sibling-shaped guard:

  ```go
  if got := m.activeTheme.Canvas.Value; got != testLightThemeCanvas {
      t.Fatalf("active canvas = %q after the commit, want %q — without the divergence there is nothing to test", got, testLightThemeCanvas)
  }
  ```

  CONFIDENCE: high

NOTES:
- **Both executor corrections verified CORRECT.** (a) M1 (the naive `sameHexColour(original, m.activeTheme.Canvas.Value)`) fails **both** divergence cases — all four skip/emit assertions — so the killer case is genuinely not "the single assertion that distinguishes" the naive implementation. Its actual distinction (the naive answer being *visually indistinguishable* from the correct one) is accurate and the comment at `:241-246` states it correctly. (b) Traced independently: `p.Run()` returns after Bubble Tea's OSC 111 reset, then `cmd/open.go:1077` / `cmd/capturetool/main.go:104` write the OSC 11 set — so the naive comparison's *write* on the echo path re-sticks the startup canvas on any terminal honouring an OSC 11 set, reset-honouring ones included. On the set-back side an original equal to the *active* canvas cannot be a race echo (query issued once at `Init`, §11.3), so it is a genuine background coinciding with that canvas and the screen result matches under both implementations. The harm sits on the echo assertion; the comments at `:145-161` and `:198-201` say exactly this and are accurate.
- **Full mutation matrix run by the reviewer via `go test -overlay` (zero repo writes):** M1 naive comparison → fails 5/6; M2 `ApplyTheme` writes `startupCanvasHex` → fails **5**/6 (not six — `NonHexReplyAfterSwap` passes, correctly, the `rgb:` fall-through being comparison-independent); M3 echo guard deleted → fails all 4 skip assertions independently; M4 writes `startupCanvasHex` instead of the captured original → fails all **4** `assertSetBack` sites (not three); M5 belt-and-braces skip-if-either-canvas → caught by all three set-back assertions; M6 `ApplyTheme` a no-op → case 1 passes, cases 2 and 3 Fatal on their premise guards; M7 `ApplyTheme` refreshes `originalBg` → **passes the entire package**.
- Two imprecisions in the executor's **report** only, not in the file: M2 fails 5 of 6 tests rather than "all six"; M4 fails four `assertSetBack` sites rather than three.
- **Every other comment claim checked and holds**: `BackgroundColorMsg.String()` → `fmt.Sprintf("#%02x%02x%02x", …)` via ultraviolet's `colorToHex`, so the three non-lower-case echo shapes genuinely are unreachable through `Update` (`:96-100`); nord is the third built-in and neither half of the pair, and `:286`'s run covers all three; the dark/light constants are pinned by `TestBuiltinCanvasValuesPinned`; `View()` sets `v.BackgroundColor` from the active theme (`model.go:4096`), so `startupModel`'s "the frame that set the terminal's default background" is literal; `§9.7` for "Ctrl-C with the panel open" is the spec's own citation; `§2.6` for the gate matches the repo-wide convention.
- `assertSetBack` cannot pass vacuously on an empty original — `ansi.SetBackgroundColor("")` returns `"\x1b]11;\x07"`, not `""`.
- §9.10's "defined and unused" justifies the absent `NO_COLOR` case (`restore.go:59` returns on `m.colourless` before comparing) — that omission is sound.
- No production change (`git diff` over `internal/`/`cmd/` empty); no panel/commit/prefs surface introduced; task 3-3's `TestRestorePath_ReadsNoTheme` untouched and green. `withCapturedOriginal` returning a copy is correct (`Model` is a value and `RestoreTerminalBackground` takes it by value).
- Unrelated pre-existing failure in the unit lane: `internal/state TestTailScrollback_PerformanceBudget` (11.4ms vs a 5ms budget) — a machine-load perf assertion in a package this task does not touch.
