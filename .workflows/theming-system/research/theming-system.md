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

## Separating "persistent preference" from "initial guess"

Lee raised a middle option: use detection **only** to pick a theme on first load,
and never re-apply it once something is configured. This is the most useful
distinction to come out of the thread so far, because it splits two jobs that
`appearance` currently conflates:

- **Job A — what does the user want, persistently?** A *preference*. Detection is
  bad at this: it's unreliable inside tmux, and re-inferring on every launch means
  a flaky detect can flip the UI between runs. That inconsistency is probably a
  large part of why auto *feels* broken even when it's working as designed.
- **Job B — what's a sensible starting guess?** A *seed*, consumed once.
  Detection is fine at this, because a wrong one-time guess is a one-time
  annoyance the user corrects once, permanently.

**The key structural point: the seed approach keeps nearly all the
simplification.** The large wins come from "a theme carries its own canvas, so
there is no light/dark axis" — *not* from "never look at the terminal
background". Those are separable:

- The gate's **first-paint blocking role disappears either way**, because a
  persisted theme is known at construction. Nothing to race, paint on frame one,
  50ms wait still deleted.
- `theme.Mode`, `Token.ColorFor`, the dual-canvas contrast bookkeeping,
  `prefs.Appearance`, and the paired-vs-split question are still all gone.
- What comes back is small: a **non-blocking, best-effort, one-shot** read of the
  OSC 11 reply, used to choose between two defaults and then persisted. It needs
  no race and no timeout, because nothing is gated on it.
- And the reply is **arriving anyway** — `restore.go` already needs the OSC 11
  query to capture the original background for restore-on-exit. Reading that same
  reply once to pick a default is close to free.

Residual costs: a first-run-only paint-then-flip if the light reply lands after
first paint (mild, and only ever once), plus a small amount of code. The real
hazard is scope creep on the trigger condition — if it fires on anything other
than "the user has never chosen", the between-runs flakiness comes straight back.

## Shipping a default: the options

Whatever happens, Portal must boot with *a* theme. Four shapes on the table.

### 1. Hardcoded default, manual change thereafter

Dark, and this doesn't look like a close call:

- **MV is dark-first by construction.** The package doc says so, `theme.Mode`'s
  zero value is Dark *on purpose*, and the dark hexes are the pinned §2.9
  authoritative source. The light variants were derived afterwards and needed six
  individual corrections plus three eyeball-pinned surface tints. The dark palette
  is simply the better-tuned artifact.
- **The failure mode is symmetric and safe.** Canvas ownership guarantees the
  contrast floor in both directions, so a mismatch is *jarring, never illegible*.
  That reduces the choice to base rates, and terminal users skew dark.
- It keeps an existing documented fallback rather than making a new decision.

### 2. Detection as a first-run seed

Composes with (1) rather than replacing it: default is the dark theme; on first
run only, if OSC 11 reports light, start on the light theme instead and persist.
Hardcoded default is the floor; detection is an optional improvement on top. Fails
safe by construction.

### 3. First-run modal asking the user to pick

Worth pushing back on, for two structural reasons rather than taste:

- **"First run" isn't reliably a TUI moment.** Portal's dominant entry point is
  `portal open` / the `x` function, and a bare positional resolves and attaches
  **directly via `syscall.Exec` without ever showing the picker**. So a first-run
  modal either has to block a non-TUI path (bad) or fire only when the picker
  happens to open first (inconsistent).
- **Modals blank the screen to the canvas** — the exact problem that pushed the
  theme selector toward a slide-over in the first place. A theme-picking *modal*
  has nothing to preview against, so the first-run experience would be strictly
  worse at picking a theme than the normal slide-over is.

Also: it asks the user to choose before they've seen the app or any theme. The
counter-argument (explicit, honest, one interruption ever) is real but doesn't
answer the two structural points.

### 4. No persisted default at all — resolve to dark at read time

Not raised, but worth having on the table because it interacts with (2): if
"unconfigured" stays *distinguishable* from "explicitly chose dark" (i.e. nothing
is written to `prefs.json` until the user picks), then the seed can fire on any
launch where the user has never chosen — not just a literal first run. A failed
detect gets another chance next launch, and no first-run bookkeeping is needed.
Persist on first successful detect and the window closes cleanly.

### A naming consequence

The seed flagged that *token* names become a public contract a user themes
against. **Theme names are the same class of problem** — once `tokyo-night-dark`
is a string users write in config (and that ships as the default), renaming it
breaks their config. Worth settling deliberately alongside the border-token
rename rather than falling out of implementation.

## Assessment (for discussion to ratify, not settled here)

Asked directly for a read, with the reasoning rather than just the verdict:

**Removing the light/dark appearance axis looks right**, and the strongest argument
is not the code deletion — that's a consequence, not a reason. It's that the axis
**infers something it cannot reliably infer, in order to make a choice the user is
better placed to make**, and Portal's own history documents the unreliability
(tmux passthrough). An unreliable inference driving a loud outcome — Portal
repaints the terminal background — is a poor trade.

The "third position on the same question" concern also looks weaker on inspection.
Read the arc: (1) don't own the background, best-effort adaptive plus an override
escape hatch; (2) own the canvas, guarantee the floor, detect *which* canvas;
(3) own the canvas, let the user name it. Every step **reduced the amount of
inference**. This is the same direction of travel one step further, not
vacillation.

**What the ecosystem does** (from recollection — flagged as the thing a deep dive
should verify rather than trusted): terminal *applications* overwhelmingly default
to a single named theme and let the user set it — `bat`, `delta` (defaults dark,
`--light`/`--dark` by hand), Helix (`theme = "…"`), lazygit, k9s, Zellij. Auto
light/dark detection lives mostly at the *emulator* layer (Ghostty's mode-mapped
theme setting, iTerm2 presets, Windows Terminal), and where applications do offer
it, it tends to be opt-in rather than the mechanism. Neovim's `background` is the
notable counter-example and is set by hand. That pattern supports: hardcoded
sensible default + manual selection, with detection as a convenience rather than
the mechanism.

Net: option **(1) + (2)**, and specifically (2) implemented via the (4)
"unconfigured is distinguishable" shape, gets the user close to right without
making detection load-bearing. Option (3) has structural problems in Portal
specifically.

## The slide-over: overlay, and the mechanism already ships

Lee's picture clarified: the panel **overlays** the right-hand side. Content does
not shrink or reflow — it stays exactly where it is and the panel draws on top of
it. "Much like the modal, but fixed to the right-hand side as a list" — the
difference from a modal being that a modal blanks the background and the slide-over
does not.

**This is already implemented in Portal.** `internal/tui/pagepreview.go:731-747`
(`overlayHelpOnPreview`) composites the `?` help panel over the Preview page using
z-ordered layers:

```go
background := lipgloss.NewLayer(preview).X(0).Y(0).Z(0)
foreground := lipgloss.NewLayer(panel).X(x).Y(y).Z(1)
return lipgloss.NewCompositor(background, foreground).Render()
```

Its doc comment states the exact principle Lee described — *"the preview content
stays visible behind it (§9 — the Preview `?` help overlays, it does NOT blank).
… leaving every cell outside the panel showing the preview underneath."* So the
non-blanking overlay is not a new chrome shape at the mechanism level: it is an
existing, shipped, spec-sanctioned one, currently used at one site. The slide-over
is that same call with `X` pinned to the right edge and full height instead of
centred.

Verified: `charm.land/lipgloss/v2 v2.0.4` ships genuine compositing —
`Layer` with `X`/`Y`/**`Z`** and nested `AddLayers`, plus `Compositor` with
`Render()` and `Hit(x, y)` (mouse hit-testing, which a clickable panel could use
later). No ANSI-aware string surgery required; this is a first-class API. Portal
uses it at exactly one site today.

### What overlay buys and what it costs

- **It sidesteps the pagination-invariant risk entirely.** The list's width and
  height are untouched, so nothing reflows, no height re-budget, no
  one-row-per-delegate perturbation. That was the main hazard in the shrink-to-fit
  alternative and overlay removes it rather than managing it.
- **The owned canvas makes overlay clean.** Because Portal paints an opaque
  backdrop on every cell, the panel's cells are fully opaque with no transparency
  artefacts. In a TUI that leaves the terminal background showing through, an
  overlay panel tends to look broken — canvas ownership is what makes this tidy.
- **But it hides the right-hand column, which is where Portal puts information.**
  This is the honest cost and it isn't "covering empty space": the footer's
  right-aligned `? help` (`right_anchored_row.go`), the right-side header hint, and
  session row meta all live at the right edge. Shrink-to-fit would hide nothing but
  pay reflow; overlay pays no reflow but hides the informational edge. A genuine
  tradeoff, and it argues for thinking about *what* sits behind the panel, not just
  the panel.
- **Narrow terminals need a separate answer.** `header.go` pins
  `minTerminalWidth = 40` with staged degradation below it. A usably-wide panel
  (~24–30 columns) over a 40-column terminal covers most of the screen, at which
  point the overlay degenerates into "a modal that doesn't blank" and there is
  almost nothing left visible to preview against — defeating the reason for
  choosing a slide-over. Some threshold behaviour is needed.

### Naming hazard

`lipgloss.Canvas` is a *different concept* from Portal's "owned canvas" (the
painted mode-matched backdrop). Both terms are now in play in the same feature.
Worth keeping them lexically separate in the spec so they don't get conflated.

## Live-preview semantics: apply-on-arrow

Lee's model: arrowing the list **sets** the theme for real; `Esc` just closes the
panel; the theme stays set. No separate commit step.

Methodological note he was right to make: the earlier framing let *existing code*
constrain the design question. The feature is allowed to change the code — the
question is what the right behaviour is, then whether it's implementable.

### Two established idioms, and his is one of them

- **Picker idiom** — hover previews, `Esc` **reverts** to what was active, `Enter`
  commits. VS Code's theme quick-pick, Telescope's `colorscheme` picker.
- **Settings-panel idiom** — a change applies immediately and there is no cancel.
  iTerm2's appearance prefs, and most application settings surfaces.

Lee's model is the settings-panel idiom. It is coherent, and more importantly it is
**consistent with Portal's own existing behaviour**: the `s` grouping-mode toggle
already persists on every press with no undo. Internal consistency is the stronger
argument here than matching VS Code.

Its one honest cost: **no escape hatch from a bad landing.** Arrow through eight
themes, dislike where you stopped, and you have to *remember* what you had. Trivial
with two or three built-ins; less so with a growing user library. A cheap mitigation
that preserves the model completely: mark the theme that was active when the panel
opened (Portal already has a marker vocabulary — `●`). "Get back" becomes visible
rather than remembered, with no revert machinery and no shadow state.

### Is it possible — yes, and the OSC 11 concern was overstated

**Correction to the earlier framing.** Bubble Tea v2 **diffs** the view's
background colour and only emits when it changes —
`bubbletea/v2@v2.0.7/cursed_renderer.go:411-432`:

```go
{newColor: view.BackgroundColor, oldColor: lbg, reset: ansi.ResetBackgroundColor, setter: ansi.SetBackgroundColor},
…
if c.newColor != c.oldColor { … s.scr.WriteString(c.setter(col.Hex())) }
```

So there is no per-keystroke churn. Hovering N themes emits OSC 11 exactly once per
*distinct* canvas landed on — the minimum possible, and precisely what the feature
requires. The declarative `v.BackgroundColor` assignment per frame is not a
per-frame write.

**The `restore.go` echo-guard concern also dissolves on inspection.** That guard
exists because the startup OSC 11 *query reply* can race Portal's own canvas set.
The query is issued once, from `Init`. A later theme switch issues no new query, so
it creates no new race — the guard only ever needs to compare against the canvas
active during the *startup* window, not the currently-active one. Anchoring it to
the startup canvas keeps it correct with no new machinery. (This holds whether or
not the appearance axis is removed: `restore.go` needs the query regardless, for the
original-background capture.)

### Settled in session: persist on close, marker, no revert key

Lee's refinement: mark the theme active when the panel opens (yes), **no special
keybinding to get back to it** (deliberately not added — the marker is
informational, not an affordance), whatever you're on when you close is what's
saved, and the panel carries a **visual statement of the contract** — an
"escape closes / saves on close" hint.

That resolves the persist-timing question in favour of **persist on close**, which
is also the no-write-storm option. It makes the marker self-consistent: the marker
shows the *persisted* theme, so if you close on a different one the marker has
moved next time you open. One coherent story rather than two mechanisms.

Two consequences worth recording.

**The contract hint sits along the top of the panel, not the bottom** (Lee's
clarification). That side-steps the footer question entirely: Portal's footer is
descriptor-driven (`keymap.go`, `keymapEntry` with `Core` / `RightAligned` /
`Destructive`), consumed by *both* the footer and the `?` help, with
`keymap_dispatch_guard_test.go` guarding descriptor↔dispatch drift, and every entry
there is a `key + action-label` pair. A statement about behaviour ("saves on close")
doesn't fit that vocabulary — but as panel *header* chrome it doesn't need to, and
needs no descriptor entry.

**"Applied but not persisted" becomes a reachable state.** Under persist-on-close,
if Portal dies while the panel is open — `Ctrl-C`, terminal closed, or any exit path
that isn't "close the panel" — the visually-applied theme was never written, and the
next launch comes back on the marker. That is arguably *correct* (you never finished,
so nothing saved) and it is defensible, but it is a state that simply cannot exist
under write-per-arrow. Worth being a deliberate choice rather than a side effect.

### The one thing apply-on-arrow genuinely does need decided

Visual application and *persistence* are separable, and only the second is
awkward. "Arrow sets the theme" taken literally means a `prefs.json` write per arrow
keypress. Apply-on-arrow visually while persisting on panel close (or debounced)
gives byte-identical UX with no write storm. Same idiom, different write cadence —
worth choosing deliberately rather than inheriting the `s` toggle's write-per-press
behaviour by analogy.

## VERIFIED: the mode-2031 chain works in Portal's stack today

The deep dive named this as its single most load-bearing unverified item. Checked
directly against the pinned dependency versions — **the whole chain is present and
needs no upstream change.**

**1. `github.com/charmbracelet/x/ansi v0.11.7` has full mode-2031 support:**

```go
ModeLightDark        = DECMode(2031)
SetModeLightDark     = "\x1b[?2031h"
ResetModeLightDark   = "\x1b[?2031l"
RequestModeLightDark = "\x1b[?2031$p"
RequestLightDarkReport = "\x1b[?996n"   // poll current state
func LightDarkReport(dark bool) string  // "\x1b[?997;1n" / "\x1b[?997;2n"
```

**2. `ultraviolet` decodes the report into typed events** (`decoder.go:428-440`):
DSR `997;1` → `DarkColorSchemeEvent{}`, `997;2` → `LightColorSchemeEvent{}`.

**3. Bubble Tea v2 delivers them to `Update` without wrapping.** This is the part
that could have blocked it, and it doesn't: `tea.go:50` declares
`type Msg = uv.Event` — a **type alias**, not a distinct type — and
`translateInputEvent` (`input.go`) ends with a bare `return e` for any event it has
no explicit case for. It has no case for the two colour-scheme events, so they
arrive in `Update` verbatim as `uv.DarkColorSchemeEvent` / `uv.LightColorSchemeEvent`
and can be type-switched directly. No fork, no upstream PR, no parsing by hand.

**4. Portal must opt in itself, and there is a sanctioned route.** Bubble Tea does
not enable 2031 on its own (the renderer sets bracketed paste, focus, mouse,
alt-screen, synchronised output and unicode-core — not 2031) and ships no
`Request…` Cmd for it. But `tea.Raw(any) Cmd` (`raw.go`) exists precisely for this,
documented as *"for advanced use cases where you need to query the terminal or send
escape sequences directly"*, and the program `execute`s it verbatim
(`tea.go:858`). So `tea.Raw(ansi.SetModeLightDark)` from `Init`, optionally with
`tea.Raw(ansi.RequestLightDarkReport)` to poll immediately rather than wait for a
push.

**5. Environment check: the installed tmux is 3.7b, not the 3.6b CLAUDE.md
records.** Mode 2031 landed in tmux 3.6, so it is available — and the local
environment is a release newer than the documented target. (Minor doc drift worth
noting separately.)

### What this changes

The premise that *"bg-detection passthrough is unreliable inside tmux"* — inherited
from the `spectrum-tui-design` discussion, repeated in Portal's README, and used in
this session as an argument for deleting the appearance axis — was formed against
**OSC 11**, a colour query the app must classify by luminance and race with a
timeout. Mode 2031 is a different mechanism: a *semantic* light/dark answer, pushed
on change, which tmux synthesises even when the outer terminal cannot answer.

That does not decide anything, but it does mean the appearance question should be
re-opened in discussion against the 2031 mechanism rather than against OSC 11. The
"detection is unreliable" leg of the removal argument is no longer supported by the
evidence that produced it.

**Still unverified (needs a real terminal, not code reading):** whether
Ghostty → tmux 3.7b → Portal actually produces the event in practice. The code path
exists end-to-end; the behavioural confirmation does not yet.

## Convergence flag — threads that have left research territory

Lee called this out mid-session and he is right: the slide-over thread drifted into
deciding rather than exploring. Recording the boundary so the phase doesn't keep
crossing it.

**These are decisions, not findings — they belong to discussion.** They are recorded
above because they were reasoned through here, but they should be re-opened and
ratified in the discussion phase rather than treated as settled by research:

- Overlay vs shrink-to-fit for the panel (landed on overlay).
- Apply-on-arrow vs preview-then-commit (landed on apply-on-arrow).
- Persist-on-close vs persist-per-keypress (landed on persist-on-close).
- Marker for the on-open theme; no revert keybinding.
- Whether the appearance axis is removed at all.
- Panel input routing — what `Enter` does while the panel is open, who owns arrows,
  behaviour during an active filter / multi-select / on other pages. **This is
  squarely discussion territory and was wrongly pushed as a research question.**

**The distinction worth holding onto**, because the session blurred it: *"is
light/dark detection reliable inside tmux?"* is a research question — factual,
testable, and it changes what options exist. *"Should Portal remove the appearance
axis?"* is a discussion question. The first informs the second; they are not the
same question, and the session had started treating them as one.

**What remains genuinely unexplored, and is research territory:** the theme-file
format and its dependency cost; the border-token rename and naming prior art; where
an additional built-in theme's values come from and whether an off-the-shelf palette
can clear Portal's floors; what the validation rule set actually is; the
capture-harness cost; and whether detection works in Portal's stack at all.

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
