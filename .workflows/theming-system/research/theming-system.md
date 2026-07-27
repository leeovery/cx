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
