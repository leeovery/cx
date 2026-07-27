# Research: Theming System

Making Portal's Modern Vivid colour-token layer user-themeable — loading token
values from config rather than code, shipping additional built-in themes, adding
a live-preview in-app theme selector, and settling the border token names before
they become the public contract a user themes against.

## Starting Point

What we know so far:

**Prompted by** — two inbox ideas picked up together in discovery: the deferred
user-overridable theme system (external theme file, more built-in themes plus a
selector, contrast-floor validation for user colours, a documented token
vocabulary) and the rethink of the two border design-token names
(`border.separator` / `border.footer`), which are named after their first
use-site rather than their intrinsic weight. The rename matters most *before* the
theme system ships, because the role token names become the public contract.

**Existing ground** — the Modern Vivid token layer already exists
(`internal/tui/theme`: ~20 semantic role tokens, each with Light and Dark
variants; `theme.MV` is the single built-in) and every renderer already
references a token, never raw hex. So the work is changing *where token values
come from* and adding a surface to switch them — not building a theming system
from nothing.

**Lee's picture** — config-driven at the base, but with a strong preference for
selecting a theme *inside* the app and seeing the display change immediately. He
identified a concrete obstacle himself: Portal's existing modals blank the whole
screen to the canvas, so a modal-based theme picker would leave nothing visible
to preview against. His proposed answer is a new chrome shape — a TUI equivalent
of a web "slide over", occupying a strip of the right-hand side, where arrowing
down the theme list re-renders the main Portal view live behind it. Whether live
re-theming is technically achievable was deliberately parked for this phase to
answer rather than guessed at in discovery.

**Custom theme authoring is not a UI concern** — that's a configuration file.
Format undecided and open (YAML, JSON, TOML, or possibly just a flat key/value
list), with a preference for whatever is least verbose and least prescriptive
rather than a heavyweight schema.

**Starting direction** — technical feasibility (live re-theming behind a
slide-over panel; loading token values from config; contrast-floor validation for
user-supplied colours) plus general direction on theme-file format and token
naming.

**Constraints:**
- Ship at least one additional built-in theme so Portal launches with genuine
  options rather than a single built-in.
- The token rename must settle before the theme system ships (public contract).
- Existing contrast floors are numerically verified against the exact owned
  canvas (`contrast_test.go`); colour literals at call sites are guarded
  (`colour_literal_guard_test.go`).
- The visual *layout* is fixed — themes change colour only, not structure
  (carried over from the `spectrum-tui-design` redesign).
- Colours stay light/dark adaptive and truecolor-first with graceful
  downsampling; `NO_COLOR` handling already exists and interacts with this.
- Behaviour of the list/grouping/filtering layer is out of scope — this is the
  styling layer.

---

## Prior context (knowledge base)

The `spectrum-tui-design` discussion already pre-scoped this work in its
**"Theming system"** section: build the redesign token-based (in scope, done) and
log user-overridable themes as a separate initiative — external theme file,
merge-over-default, validation, multiple built-ins, a `theme` setting, docs.
Confidence recorded as "high on the split".

One thing in that discussion has since **changed the shape of the validation
problem**. Its `AdaptiveColor binary classification` decision originally treated
the contrast floor as *best-effort* because Portal didn't own its background, and
named "the eventual manual `--theme` override (the deferred user-theme
initiative)" as the escape hatch for mid-tone backgrounds. That stance was then
**REVERSED (2026-06-18)** when Portal took ownership of a mode-matched canvas:
the floor became *guaranteed*, and mid-tone / detection-misfire stopped being
legibility risks. So the theme system inherits a **cleaner** validation problem
than the seed anticipated — see "Contrast validation" below.

---

## Code grounding — what actually exists today

A read of the current code, done before the session, to replace assumption with
fact. Findings, not conclusions.

### The token layer

`internal/tui/theme/theme.go` — 185 lines. `Theme` is a **struct with 20 named
fields** (7 text-ramp, 6 accents, 7 surfaces), each a `Token{Name, Light, Dark}`.
`Token.ColorFor(mode)` picks the variant and hands it to `lipgloss.Color`.
`Theme.All()` returns the 20 tokens in stable order. `theme.MV` is a single
package-level `var` holding the built-in.

Notable: a token's identity is a **Go struct field**, and its `Name` string
(`"text.primary"`, `"border.separator"`, …) exists purely for `All()` consumers
— tests and, in the doc comment's own words, *"future theme tooling"*. So the
string names already exist but are not yet a lookup key. A config-driven theme
needs name → field resolution in some form.

### Live re-theming: the parked feasibility question

**The mechanism already ships.** Two independent findings:

1. **Call sites resolve at render time, not init time.** All 182 non-test
   `theme.MV` references across 20 files sit *inside* render functions
   (`headerStyle(theme.MV.TextPrimary, mode, colourless)` etc.), so they re-read
   the token on every `View()`. Bubble Tea re-renders the whole frame each
   `Update`, so a swap of the active theme is picked up on the next frame with
   **zero call-site churn**. Exactly **one** package-scope exception:
   `internal/tui/pagepreview.go:35` — `var previewBorderColorToken =
   theme.MV.AccentCyan` copies the `Token` struct at init and would not see a
   swap.

2. **Portal already performs a mid-session restyle.** `model.go`'s
   `applyResolvedMode` / `applyCanvasMode` / `applyProjectCanvasMode` exist
   because the OSC 11 appearance query is *async and non-gating* — it can answer
   *after* the first paint, at which point the model re-points its stateful leaf
   styles (session + project delegates, `bubbles/list` help styles, pagination
   dots, the outer canvas fill) at the newly-resolved mode. This is functionally
   the same operation a live theme switch needs, already in production and
   exercised on every auto-appearance run.

   The corollary is where the real work sits: `applyCanvasMode` exists precisely
   because some styles **are** cached rather than re-derived per frame — the
   `bubbles/list` internals Portal doesn't own. Those are the sites a live theme
   swap must go through, and any future cached style must be added there.

3. **The canvas is the awkward one.** `Canvas` isn't only painted per-cell — it
   is also SET as the *terminal default background* via OSC 11 (`restore.go`), so
   the canvas reaches the window gutter. A live theme change therefore has to
   re-emit OSC 11 mid-session. And `restore.go`'s canvas-echo guard compares the
   captured "original" background against `canvasHexFor(m.canvasMode)` — with
   themes, "the canvas Portal painted" becomes a *set* of possible hexes across a
   session, not one, so the guard's comparison needs revisiting. This is the one
   genuinely fiddly mechanic in the live-preview picture, and it lives in a place
   that already carries a "do **not** drop this guard" warning.

### Two axes, not one

Every token carries **both** a Light and a Dark variant, and mode is chosen
independently by the `appearance` pref (`auto|light|dark`) plus OSC 11 detection.
So "which theme" and "which mode" are **orthogonal axes**, and a *theme* is a
pair of palettes rather than one. Open: whether a user theme must supply both
variants, and what happens when it supplies only one.

### Contrast validation

Today the floor is enforced **at test time**, numerically: `theme/contrast_test.go`
(509 lines) measures each token's Light variant against `#e1e2e7` and each Dark
variant against `#0b0c14` — the *exact* owned canvas hexes. The MV values carry
inline erratum comments recording original → corrected hex and the measured ratio
(six light hexes were darkened to clear the floor).

Two consequences for user themes:

- The canvas is **itself a token**, so a user theme changes the very background
  the floor is measured against. The check becomes "each token vs *this theme's*
  canvas" — self-consistent and computable at load time, without knowing anything
  about the terminal. That is a materially easier problem than the seed's framing
  ("Portal does not own the terminal background") — canvas ownership retired that
  worry.
- Several floors are **3:1 (UI/accent)** and others **4.5:1 (text)**, and some
  tokens are measured against a *tint* rather than the canvas (`text.strong` and
  `state.green` on `bg.selection`; `text.on-warning` on `bg.warning`). So the
  validation rule isn't one uniform "token vs canvas" sweep — there is a
  per-token pairing table implied by the existing test, and a runtime validator
  would need that relationship encoded as data rather than as test cases.

### Guard tests that will need reshaping

- `theme/theme_test.go` — `TestMVDarkVariantsPinned` pins all 20 dark hexes
  exactly; `TestMVTokenCount` pins the count at exactly 20 (closed vocabulary).
  Both are MV-specific by construction; additional built-ins need their own
  treatment, and the count guard's meaning shifts from "MV has 20" to "the
  vocabulary is 20".
- `internal/tui/colour_literal_guard_test.go` — AST-parses every production file
  in `internal/tui` (via a glob, so coverage grows automatically) and fails on any
  raw literal passed to `lipgloss.Color`. The `theme/` subpackage is the only
  sanctioned home for raw colour values. A config loader that parses user hex
  strings introduces a *second* place raw colour values enter the system — the
  guard doesn't cover it (different package) but the invariant it protects is
  worth thinking about.

### Other touchpoints

- `internal/capture/swatch.go` references `theme.MV` — the offline visual-capture
  harness (`cmd/capturetool`, `testdata/vhs/`). New themes imply a decision about
  fixture/reference-PNG coverage: capture every theme, or one canonical.
- Config resolution already exists (`cmd/config.go` `configFilePath`, per-file env
  var → `XDG_CONFIG_HOME/portal/` → `~/.config/portal/`), as does the one-shot
  legacy-path migration and the log-component mapping. `prefs.json`
  (`internal/prefs`) already holds the `appearance` override and is deliberately a
  **leaf** (no `internal/log`) — the natural existing home for a persisted `theme`
  selection.

### The one real architectural tradeoff surfaced so far

`theme.MV` is a package-level global read directly at 182 call sites. Making the
active theme switchable is a choice between:

- **Mutable package state** — `theme.Active` var + setter, or an accessor
  function. Near-zero call-site churn; but package-level mutable state, and the
  render path becomes order-dependent on the setter.
- **Explicit plumbing** — pass a `Theme` (or a resolver) down through the
  renderers. Idiomatic and test-friendly; but a large mechanical change across
  182 sites plus signature churn on every render helper, and the existing
  functions already thread `mode` and `colourless` — a third parameter.

Both are viable; the tradeoff is Go-idiom cleanliness versus blast radius. Not a
decision for this phase.

---

## The slide-over becomes an appearance surface too, not just a theme list

Lee's reaction to the two-axes finding: put the **light / dark / auto** picker in
the slide-over as well. His reasoning — that setting exists today but is
*config-only* (`prefs.json` `appearance`), and this is the opportunity to make it
TUI-configurable rather than a file edit.

That is feasible on the same seam, with one wrinkle. `appearance` is read **once
at TUI construction** (`WithAppearance` in `cmd/open.go`) and its value gates
whether OSC 11 detection runs at all — `light`/`dark` *pin* the canvas and skip
both detection and the ~50ms first-paint wait. So:

- **auto → light/dark** is trivial: set `canvasMode`, re-apply, re-emit the canvas
  OSC 11, persist.
- **light/dark → auto** is the interesting direction: detection has to run *now*,
  mid-session. Mechanically fine (there's no first-paint gate left to win), but the
  appearance gate's designed invariant is **single-resolution, never
  paint-then-flip** — and switching to `auto` live *is* a paint-then-flip by
  nature. The invariant was written for startup, so this isn't a contradiction so
  much as a case the gate was never asked to handle.

Worth noting both axes converge on the same machinery: **theme change and
appearance change are the same re-apply + re-emit-OSC-11 operation**, differing
only in which input moved. The slide-over is one panel driving two settings
through one seam.

Scope observation (not a proposal): `prefs.json` currently holds exactly two
things — `session_list_mode` and `appearance`. If the slide-over becomes the home
for `appearance`, then one pref is adjusted by a keybind cycle (`s`) and the other
by a panel. There's a latent consistency question there about what a "settings
surface" in Portal is, which this work will brush against whether or not it
answers it.

## Paired vs split: is "one theme carries both variants" the right model?

Lee raised this himself and explicitly invited the discussion: today a theme is
`{20 tokens} × {light, dark}` — one theme, two variants. The alternative is
`theme = one palette` tagged light or dark, so "Tokyo Night Dark" and "Tokyo Night
Light" are two separate themes and the appearance axis selects between them.

**The central reframe: splitting does not remove the pairing, it relocates it.**
`auto` mode is the reason. Under `auto`, detection says "the terminal is light" and
something must know *which* palette to use — so a split model still needs a
light↔dark relationship, expressed either as a naming convention
(`gruvbox-light`/`gruvbox-dark` — fragile and prescriptive), as two settings
(`theme.light` / `theme.dark`), or as metadata inside the theme (`variant:` plus a
`pair:` pointer — which is pairing again, just externalised). So the real question
is **where the pairing lives**, not whether it exists.

### Arguments the current paired model is right

- **It's what the code already is.** `Token{Name, Light, Dark}` + `ColorFor(mode)`.
  Zero migration, zero call-site churn.
- **The appearance axis stays genuinely orthogonal.** `auto` can flip light↔dark
  without changing theme *identity* — the user picked "Tokyo Night" and stays on
  Tokyo Night all day. That's a clean conceptual model.
- **One selector list**, and arrowing through it never changes the canvas mode —
  so live preview is a pure colour change with no mode flip mid-scroll.

### Arguments for splitting

- **The whole terminal ecosystem is single-palette.** base16/base24, iTerm2
  `.itermcolors`, Alacritty, WezTerm, Zellij, Helix, `bat`/`delta` — one palette
  per file, with "Gruvbox Light" and "Gruvbox Dark" as two named themes. If Portal
  ever wants theme files that are recognisable to (or cribbable from) that
  ecosystem, single-palette matches convention.
- **Authors only care about the mode they use.** Paired forces every author to
  either design both halves or leave one undefined — which is precisely the
  "what renders if the mode is undefined?" open question below. Split makes that
  question disappear: an author writes exactly one palette and nothing is missing.
- **The asymmetry is real, and Portal's own code is the evidence.** A good light
  palette is not a mechanical lightening of a good dark one. In MV, six light
  hexes had to be *individually* darkened with erratum comments to clear the
  floor, and the three light surface tints were **eyeball-pinned at a validation
  gate** rather than derived. MV's light and dark are, in practice, two
  independently-tuned palettes that happen to share token names — the paired
  struct implies a derivation relationship that doesn't actually exist.
- **It would collapse `mode` out of the render layer.** `theme.Mode` is currently
  threaded as a parameter through essentially every render helper in
  `internal/tui` (`headerStyle(tok, mode, colourless)`, `renderDeleteModalContent(…,
  mode, colourless)`, and so on across ~20 files), and its only job is choosing a
  variant — it is always passed straight down to `ColorFor`. If the active theme
  were already resolved to a single palette, `Token` collapses to `{Name, Value}`,
  `ColorFor` disappears, and `mode` stops needing to reach renderers at all. That
  is a substantial simplification of the render layer — and simultaneously a
  substantial mechanical change to make.

### Consequences that cut across both

- **Live preview behaves differently.** Under paired, arrowing the theme list never
  changes light/dark. Under split, arrowing onto a light-only theme while in dark
  mode must either flip the canvas (arguably *more* honest — you see the theme as
  designed) or refuse/filter it (the list shows only themes matching the current
  mode). Both are defensible; they're different products.
- **Ghostty solves this exact problem in its own config** with a mode-mapped theme
  setting (a single theme name, *or* a light/dark pair in one setting) — a directly
  relevant precedent, and Portal already has a native Ghostty spawn adapter. Worth
  verifying the exact shape rather than trusting recollection.
- **Validation is indifferent.** Each variant is already measured against its own
  canvas independently, so the floor check works the same either way.

## What `appearance: ""` means, and what Lee's test does / doesn't prove

Lee's local `prefs.json` carries `"appearance": ""`. Traced: `parseAppearance("")`
hits the tolerant-decode default → **`AppearanceAuto`**. So he is on auto, and the
detect-or-timeout gate *is* arming. Everything is wired correctly —
`tea.RequestBackgroundColor` is issued from `Init` (`model.go:2130`), the reply is
classified by `msg.IsDark()` (`model.go:2229`), and a 50ms `tea.Tick` races it to
the dark fallback.

He switched to light mode and Portal stayed dark, across a restart. Two readings,
and they are **not** equivalent evidence:

1. **OSC 11 got no usable reply inside 50ms.** Plausible — the
   `spectrum-tui-design` discussion explicitly flagged that Portal runs *inside
   tmux*, "where bg-detection passthrough is unreliable". Under this reading auto
   is ineffective in Lee's actual environment.
2. **The terminal never actually changed.** If Ghostty is configured with a fixed
   theme rather than a light/dark-mapped one, switching *macOS* appearance changes
   nothing about the terminal's background — and Portal reporting dark would be
   **correct**, not broken.

Unresolved which. Noting it because "auto doesn't work" and "auto worked, my
terminal just didn't change" are different justifications for removing auto — even
though, as below, the case for removal doesn't actually rest on either.

## Proposal (Lee): delete the appearance axis entirely

Lee's position after the above: **remove `auto`, and remove light/dark as an
appearance/config concept altogether.** Split MV into `Tokyo Night Dark` and
`Tokyo Night Light` as two ordinary themes, so nothing is lost — the existing
values survive as two entries in the theme list. Theme selection is manual, in the
TUI. Rationale: terminal *applications* generally don't auto-switch (you pick a
theme), and Portal is a transient UI, so a mismatch is not fatal.

### This is a much larger simplification than the theme model alone

The thing worth surfacing: `appearance` is not merely a setting. It is the **sole
reason the appearance gate exists**. If the chosen theme already answers "light or
dark", then the following all lose their only consumer:

- **`internal/tui/appearance_gate.go` in its entirety** (167 lines) — the
  detect-or-timeout race, `appearanceTimeoutMsg`, `arm()`, `resolveFromDark`,
  `resolveDark`, the single-resolution / no-flip invariant, the pending
  blank-first-frame.
- **The 50ms first-paint wait.** In auto mode today the first *real* paint is held
  behind a neutral blank frame for up to 50ms waiting for OSC 11. Delete the axis
  and Portal paints the correct canvas on frame one, always, with no race to win.
  That's a startup win *and* the removal of the trickiest timing code in the TUI.
- **`prefs.Appearance`** — the enum, its tolerant decode, `LoadAppearance` /
  `SaveAppearance`, and `WithAppearance`.
- **`Token.ColorFor` and `theme.Mode` threading.** `Token` collapses to
  `{Name, Value}`; the `mode` parameter stops travelling through ~20 files of
  render helpers.
- **The dual-canvas contrast bookkeeping.** Each theme is validated against *its
  own* canvas, one palette at a time — no light-vs-`#e1e2e7` / dark-vs-`#0b0c14`
  split, and no erratum comments recording which background a ratio was measured
  against.
- **The paired-vs-split question itself**, which was only hard *because* `auto`
  needed a light↔dark pairing. Remove auto and the pairing has no consumer. The
  slide-over becomes one list on one axis.

### What does *not* disappear (precision matters here)

- **The OSC 11 *query* stays.** `restore.go` captures the original terminal
  background in order to restore it on exit — independent of detection. What dies
  is the *classification* and the race, not `tea.RequestBackgroundColor`.
- **The canvas-echo guard stays** (and keeps its "do not drop this" status); its
  comparison just re-points from "the mode's canvas" to "the active theme's
  canvas".
- **`NO_COLOR` still needs its carve-out**, but gets simpler: it currently rides
  the gate as `newColourlessGate` — a gate whose only job is to skip a race. With
  no race, NO_COLOR becomes plainly "suppress canvas and colour".

### Honest costs

- **First-run mismatch becomes sticky.** A light-terminal user's first launch is
  dark until they change it. Today auto *sometimes* gets that right; under the
  proposal it never does by itself. Traded for a reliable worst case and a large
  simplification — but it is a real regression in the best case.
- **Mismatch is more visible for Portal than for most TUIs**, precisely because
  Portal *owns* the canvas and sets the terminal default background — a dark
  Portal in a light terminal repaints the window rather than blending into it. That
  cuts both ways: it makes a mismatch louder, *and* it removes any "blend in
  quietly" option that auto might otherwise be protecting. Portal is imposing a
  background either way, so the argument that the user should simply choose it has
  force.
- **A prefs migration decision.** Existing files carry `appearance`. Tolerant
  decode means dropping the field is graceful (it becomes dead data), but there's a
  choice about whether to *translate* a persisted `light`/`dark` into a default
  theme selection on first run, or just ignore it.
- **This reverses a spec decision that was itself a reversal.** §2.6's
  detect-or-timeout gate replaced the earlier "best-effort + `--theme` override"
  stance when canvas ownership landed. This would be the third position on the same
  question — not a criticism (the ground genuinely moved each time), but the spec
  edit is real and a body of tests pinning the gate's invariants would be deleted
  (two dedicated test files, `appearance_detection_test.go` and
  `appearance_option_test.go`, plus references across ~8 more).

### On the premise

Lee's claim that terminals don't usually have auto mode is **right where it
matters and wrong where it doesn't**. Terminal *emulators* increasingly do have it
(Ghostty's mode-mapped theme setting, iTerm2's separate light/dark presets, Windows
Terminal). Terminal *applications* — `bat`, `delta`, Helix, lazygit, k9s — almost
universally do not: you name a theme and that's the theme. Portal is an
application, so the convention he's invoking is the applicable one. Vim/Neovim's
`background=light|dark` is the notable counter-example, and it is set by hand.

## Open Questions

- Must a user theme supply both Light and Dark variants, or can it supply one?
  What renders if the user is in a mode the theme doesn't define?
- Is a user theme a *full replacement* or a *merge over a base*? (The seed
  proposes merge-over-default; worth testing whether that still holds when the
  base itself is selectable.)
- Does validation **warn** or **clamp** when a user colour falls below the floor?
  (Seed leaves both open.)
- How do the theme name and the `appearance` pref compose in the selector UI —
  one list, or two axes?
- Does the theme file define *one* theme or *many* (a library the selector reads
  from)?

## Triage

(none)
