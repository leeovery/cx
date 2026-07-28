# Discussion: Theming System

## Context

Portal's TUI is already tokenised: `internal/tui/theme` holds ~20 semantic role
tokens (`Token{Name, Light, Dark}`), every renderer references a token rather
than raw hex, and `theme.MV` is the single built-in "Modern Vivid" palette. The
layout is fixed; only the colour layer is parameterised. So this work is not
building a theming system from nothing — it is changing **where token values
come from** and adding **a surface to switch them**.

Four deliverables arrived together from two inbox ideas taken as one feature:

- Load token values from config rather than code (user-overridable themes).
- Ship at least one additional built-in theme so Portal launches with genuine
  options, not a single palette.
- Add an in-app theme selector with live preview — Lee's picture is a TUI
  "slide-over" overlaying the right-hand strip, arrowing the list re-theming the
  main view live behind it (a modal blanks the screen to the canvas, so a modal
  picker has nothing to preview against).
- Settle the border token names (`border.separator` / `border.footer` — named
  after their first use-site, not their intrinsic weight) **before** the token
  names become the public contract a user themes against.

Research is complete and unusually decisive on feasibility. Live re-theming
works: call sites resolve tokens at render time, and `applyCanvasMode` already
performs exactly the mid-session restyle a theme swap needs. The non-blanking
right-hand overlay already ships (`overlayHelpOnPreview`, lipgloss v2
`Compositor` with real z-layers). Detection works both ways — OSC 11 (terminal
background) and DEC mode 2031 (OS colour scheme) are both plumbed end-to-end in
Portal's dependency stack.

What research deliberately did *not* settle is the decision set. It flagged its
own convergence — several threads landed on positions that belong to this phase
to ratify or overturn, not to inherit as settled.

### References

- [Research](../research/theming-system.md) — including Appendix A (ecosystem
  evidence: ~20 applications, 5 emulators, spec families) and Appendix B
  (Portal-specific integration surface and corrections)
- [Seed: user-overridable theme system](../seeds/2026-06-17-user-overridable-theme-system.md)
- [Seed: border token role names](../seeds/2026-06-21-border-token-role-names.md)
- [Prior discussion: spectrum-tui-design](../../spectrum-tui-design/discussion/spectrum-tui-design.md) — the "Theming system" section pre-scoped this work

---

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture. Not every subtopic needs its own section — minor items resolved in passing can be folded into their parent. The Discussion Map (which subtopics exist and their states) lives in the manifest, not this file.*

---

## Theme audience — curated set, with two contribution routes

### Context

Research flagged this as the genuinely unasked question, and it sits upstream of
several others: *who writes Portal themes?* The answer sets how much the token
names matter as a public contract, how much documentation and error-message
polish a theme file needs, and — via authoring burden — it bears on the
paired-vs-split theme model.

Two poles were put on the table:

**A — Curated set, config as escape hatch.** Lee authors everything that ships;
the loader exists so a determined user can drop one in. Token names barely
matter, docs are a README table, paired light/dark costs little because Lee
tunes both halves anyway.

**B — Theme ecosystem.** Strangers write and share themes. Token names become a
versioned public contract with an evolution story, error messages must serve
someone who has never read the code, and paired doubles an outsider's burden
(40 values against two canvases vs 20 against one). Every surveyed tool with a
real theme ecosystem went single-palette.

The framing offered was *"build A, name like B"* — accept that a B-scale
ecosystem won't materialise for a tool this size, but take B's naming discipline
now because it is nearly free up front and expensive to retrofit (which is
precisely what the border-rename seed was already worried about).

### Decision

**Mostly A, with two explicit contribution routes.** Lee is realistically the
only user, but:

1. **PR route → baked into the binary.** Anyone can open a pull request adding a
   theme. If Lee accepts it, it ships as a built-in.
2. **Drop-in route → auto-discovered from config.** A user puts a theme file in
   a `themes/` directory inside the Portal config dir and Portal picks it up
   automatically — no registration step. If it is valid it appears in the
   selector alongside the built-ins.

This lands on the ecosystem's standard two-tier shape (library directory +
selection setting) without committing to ecosystem-scale governance.

### Consequences carried forward

- **Token names still matter, but the blast radius is asymmetric.** Renaming a
  token is a repo-wide mechanical change for built-ins; it only *breaks* files
  in a user's `themes/` dir. That keeps the "name it right before shipping"
  pressure real without making it existential — and it is a further argument for
  settling the vocabulary in this feature rather than after.
- **Auto-discovery makes the validity rule user-visible.** "Valid ⇒ selectable"
  means an invalid file silently doesn't appear unless something says why.
  Research already settled the rule (all 20 tokens present + syntactically
  well-formed) and that rejection surfaces inside the slide-over; this decision
  is what makes that surfacing load-bearing rather than a nicety.
- **Two namespaces now exist** (built-in names and user-directory names), so
  collision/shadowing needs an answer — deferred to
  *theme-file-format-and-discovery*.
- **The authoring-burden argument for split is weakened but not removed.** A PR
  contributor still does the work, and dark-only famous palettes (Dracula, Nord)
  have no light half to supply.

---

## Theme file format & discovery

### Built-ins are theme files, not Go code (decided)

#### Context

The drop-in route (above) means Portal must parse a theme file. That raises a
fork the current code doesn't have: today `theme.MV` is a Go `var` holding a
struct of 20 `Token{Name, Light, Dark}` fields, so a built-in and a user theme
would be two different things arriving by two different paths.

#### Options considered

**Option 1 — built-ins stay Go structs.** User themes are parsed into the same
struct. Two authoring paths for one concept: a PR contributor writes Go, a
drop-in user writes a file.
- Cons: the paths can drift (Go can express what the format can't); the format's
  rough edges never bite the maintainer because he never uses it; two things to
  keep in sync.

**Option 2 — a built-in *is* a theme file**, embedded via `go:embed`, parsed by
the same loader as a user's.
- Pros: one code path, one format, one validity rule. A PR is "add a file". A
  user copies a built-in, tweaks two values, drops it in `themes/` — which is
  how people actually make themes. The format is dogfooded by every built-in, so
  a bad format is the maintainer's problem on day one rather than a stranger's on
  day ninety. Prior art: Ghostty and kitty avoid inventing a theme format at all
  — a theme *is* a config file.
- Cons: MV's values currently carry inline erratum comments (original →
  corrected hex, measured ratio) which a data file must either support or drop;
  parse failures move from compile-time to load-time, so built-ins want a test
  that loads all of them; `internal/capture`'s no-real-config import guard needs
  the embedded set reachable without touching the config path (fine — embed is
  not config).

#### Decision

**Option 2.** Both agreed independently, on symmetry and single-path grounds.

**The erratum comments are deleted, not ported.** Lee: *"we can just delete all
that … we don't need all that history spewing through with it."* The existing
MV values move across clean. This is defensible beyond taste: `contrast_test.go`
already enforces the corrected values numerically, so a comment recording *why* a
hex differs from its upstream Tokyo Night sibling is duplicated history — revert
a hex and the test fails, with or without the comment.

**Thread this opens:** it is the *test* that makes deleting the comments safe, so
whether built-in themes stay contrast-tested once they are data files becomes
load-bearing. Recorded against *test-and-capture-harness-impact*.

### Theme identity — filename is the slug, in-file `name` is the label

#### Context

Surfaced by the background review: the discussion had made a file the unit of
both a built-in and a user theme, and deferred name collision, without ever
defining what a *name* is. Four distinct things were being conflated — the
filename, an in-file `name` field, the human display label in the selector, and
the durable slug persisted in `prefs.json`. Research raised slug-vs-display-name
(B4) as a durable-identifier question that was never answered.

#### Options Considered

**Filename is identity** — `themes/nord.json` → slug `nord`, displayed as `nord`.
- Pros: zero duplication; file and content cannot disagree; rename is a file
  move. Effectively what kitty and Ghostty do.
- Cons: display names are constrained to filename shape (`tokyo-night-day`, not
  "Tokyo Night Day").

**In-file `name` is identity** — filename irrelevant.
- Pros: free display names.
- Cons: two files can claim the same name; file and content can disagree — a
  confusion class the first option doesn't have.

**Both, with distinct jobs** — filename is the *slug* (durable key, persisted,
written in config); an optional in-file `name` is *only* the display label.
- Pros: they can never collide because only one is an identifier. The persisted
  value is structurally unique by virtue of being a filename in a directory, and
  the rendered value is free text. Renaming a theme is a file move — an
  operation users already understand.

#### Decision

**Both, with distinct jobs.** Filename (minus extension) is the slug and the
durable identity Portal persists; an optional in-file `name` is the display
label in the selector, falling back to the slug when absent.

This also answers research's "theme names are the same class of public-contract
problem as token names" concern in the cheapest possible way: the contract is a
*filename*, so a user renaming their own theme is a deliberate file operation
with an obvious consequence, and Portal renaming a built-in is the same kind of
breaking change as renaming a token — visible, deliberate, and rare.

### Namespace collision — no shadowing; built-in names are reserved

#### Context

Two namespaces exist (embedded built-ins, user `themes/` directory), and the
whole-theme-rejection decision puts a hard constraint on how they compose:
**an invalid theme falls back to the default built-in, so if a user file can
shadow the default built-in, the fallback itself can be broken** — drop in
`tokyo-night.json` with a typo'd hex and the thing Portal falls back to is the
same broken file. That must be impossible.

#### Options Considered

**User dir shadows built-ins, with reserved names** — the ecosystem norm
(two-tier search is standard; Helix reserves `default` and `base16_default`).
- Pros: lets a user correct a built-in in place, keeping the name they know.
- Cons: a reserved-name special case; "which `nord` am I looking at?" ambiguity;
  a precedence chain to document.

**Built-ins always win; a colliding user file is silently ignored.**
- Pros: no reserved-name case.
- Cons: silent — you edit a file and nothing happens, with no signal at all.

**No shadowing; built-in names are reserved, full stop.** A user file whose slug
collides with a built-in is rejected through the same message channel as any
other invalid theme.
- Pros: collapses the reserved-default special case into one general rule; the
  selector never shows two entries with the same label; no precedence chain; the
  failure is loud rather than silent.
- Cons: to tweak a built-in you must copy it under a new slug.

#### Decision

**No shadowing.** Built-in slugs are reserved; a colliding user file is rejected
with a message naming the conflict.

Deciding factor: the workaround is a two-second file rename and is
self-documenting. Lee: *"if I wanted to edit that as a user theme, I would just
call it Nord-Lee. So it's really not a big deal."* And with the PR route open,
genuinely *correcting* a built-in has a proper channel rather than needing a
local override.

### File format — flat `key = value`

#### Context

Research found **no ecosystem consensus whatsoever** — each tool uses whatever
its config already used (Helix TOML, k9s YAML, Zellij KDL, btop flat
`key="value"`, atuin TOML, gitui RON, Glamour JSON, base16/tinted8 YAML). So the
decision is Portal-internal, and Portal's constraint is sharp: **no third-party
YAML or TOML dependency exists** — every config today is stdlib `encoding/json`
(`projects.json`, `hooks.json`, `prefs.json`, `terminals.json`) plus one flat
`key=value` file (`aliases`).

#### Options Considered

**JSON** — matches most existing config, zero new deps.
- Cons: **cannot carry comments**, and a theme is the one config file that
  genuinely wants one — ported palettes need attribution, and attribution was
  settled as repo-side rather than in-UI, making a file header its natural home.
  Also the most punctuation-heavy way to express a flat map of 20 strings.

**TOML** — comments, less punctuation.
- Cons: adds a third-party parser to a codebase that has deliberately avoided
  one for config; buys nesting Portal doesn't need.

**Flat `key = value` with `#` comments**
- Pros: Portal already parses this shape (`aliases`), so it is not a new idiom.
  Zero deps, minimal punctuation, comments free. A theme is literally 20
  key→value pairs plus a `name` line. Closest structural match in the survey is
  btop (`theme[main_bg]="#…"`, 41 lines, flat).
- Cons: a hand-rolled parser (small); a second non-JSON config format to
  document.

#### Decision

**Flat `key = value` with `#` comments.** Lee: *"nice and simple."*

The dividing line already implicit in Portal's own config supports it —
*nesting needed → JSON* (`terminals.json` maps glob patterns to command recipes;
`projects.json` is machine-written), *flat human-authored map → flat file*
(`aliases`). A theme is squarely the second.

**Forward note (not a decision):** the deferred transparent-theme idea would need
a distinguished value meaning "use the terminal default" — btop's precedent is an
empty value. The format should leave that door open rather than close it.

### Value domain — hex only, `#RRGGBB`

#### Context

Research named this "the actual decision here" for validation: everything else
about the validator follows from what values a theme may contain. `Token`'s own
doc comment currently sanctions ANSI indices, so hex-plus-index was a live
option, not an invented one.

The pressure comes from `lipgloss.Color`, which **never returns an error** and
whose accepted domain is wider and stranger than a theme format wants: `"212"`
is a valid ANSI-256 index, `"-5"` is silently abs'd to `5`, `"16777215"` is
reinterpreted as packed RGB (white), and every failure is the silent `noColor`
sentinel. Portal therefore owns its own validator regardless — which is what
turns all of that into one honest message ("theme `nord`: `text.primary` =
`#GGGGGG` is not a valid colour").

#### Decision

**Hex only, `#RRGGBB`.** No ANSI indices, no named colours, no `#RGB` shorthand
(six digits cost nothing and remove a parse branch).

Two reasons, the second decisive:

- Spec §2.4 is an explicit decision that Portal **imposes its own exact hues via
  truecolor and does not inherit the terminal's 16 ANSI colours** — a
  recognisable identity needs consistent hues across machines. Admitting ANSI
  indices lets a theme opt back into the palette Portal deliberately declined
  (research A5 records ANSI-inheriting themes as the road not taken).
- **An ANSI index has no fixed RGB.** The validator must parse to RGB anyway,
  and that same parse is what any future contrast check needs. A token valued
  `212` cannot be measured against anything — admitting them would permanently
  foreclose checking a theme numerically, including Portal's own built-ins.

Confidence: **high on the reasoning, low investment from Lee** — he explicitly
deferred to the recommendation rather than holding a position. Worth re-testing
in spec if it constrains something unforeseen.

### Theme variant (light/dark identity) — no such concept

#### Context

Research listed this as *"a related mechanic split needs anyway"*: under split,
Portal must know whether a given theme is light or dark — either declared in the
file (tinted8 requires `variant: dark|light`) or derived from the theme's own
canvas luminance (Ghostty's theme browser does this with Rec.709 coefficients
and a 0.5 threshold). The orchestrator opened this as a genuine choice and
recommended deriving.

#### Journey — the premise was wrong

Lee pushed back: *"Do we care? I don't think Portal needs to know, does it?"*
And he is right. The framing assumed Portal has to classify themes itself, which
it does not:

- Under the adaptive two-slot form, **the slot classifies the theme.**
  `light = "x"` means "use this when the terminal is light" — Portal never
  inspects the palette to know that.
- Warning that a dark theme sits in the light slot is exactly the *perceptual*
  judgement already ruled out of validation (validity is syntactic, never
  perceptual).
- Grouping or filtering the selector list by variant is the panel search/filter
  feature already deferred as YAGNI.

So the mechanic has **no consumer**.

#### Decision

**No variant concept.** Not declared, not derived.

The asymmetry is what makes not-deciding safe rather than merely convenient:
*declaring* would lock a key into the public contract now, whereas *deriving*
costs nothing and needs no format change — so if a selector filter ever ships,
the value can be computed that day. Not deciding is free here; deciding is not.

---

## Light/dark detection — ships, follows the terminal, two-slot form

### Context

Portal owns a mode-matched canvas, so something must decide which canvas to
paint. Today that is `prefs.appearance = auto|light|dark` plus an OSC 11
detect-or-timeout gate. Split removes the *variant* axis from the theme, so the
question re-forms as: does Portal detect at all, against which signal, and what
does the user-facing setting look like?

Research spent the majority of its time here and arrived at "match the terminal"
— but flagged it for ratification. This section ratifies it and records the
journey, because the journey contains two reversals that are easy to
re-introduce if forgotten.

### Journey — including two false paths

**1. The starting premise was that detection is unreliable inside tmux.**
Inherited from the `spectrum-tui-design` discussion, repeated in Portal's README,
and used as the main argument for deleting the appearance axis entirely.

**2. Lee proposed deleting the appearance axis outright** — no `auto`, no
light/dark as a config concept; split MV into two ordinary themes and select
manually. The appeal was large: `appearance_gate.go` (167 lines), the 50ms
first-paint wait, `prefs.Appearance`, `Token.ColorFor`, and the dual-canvas
contrast bookkeeping all lose their only consumer.

**3. DEC mode 2031 was found to be plumbed end-to-end in Portal's stack** —
`x/ansi` has the mode constants and report parsers, `ultraviolet` decodes DSR
`997;1`/`997;2` into typed events, and Bubble Tea v2 passes them through to
`Update` verbatim (`type Msg = uv.Event` is a type *alias*, and
`translateInputEvent` returns unhandled events bare). Portal opts in with
`tea.Raw(ansi.SetModeLightDark)`. tmux 3.6+ supports it and the installed tmux is
3.7b. That looked like it refuted the unreliability premise.

**4. FALSE PATH — 2031 answers a different question.** The packages say so
explicitly: `ModeLightDark` reports *"the **operating system's** color scheme
preference"*. OSC 11 answers *"what colour is my terminal's background?"*. Those
routinely disagree, and Lee's own environment is the disagreement case — a
Ghostty pinned dark on a light macOS. A further wrinkle cuts the same way: on
terminals that don't support 2031, tmux *synthesises* the answer by guessing from
the background colour, which is the very signal whose unreliability was the
original objection. So 2031 is a better-engineered answer to a question Portal
may not be asking.

**5. The unasked question: what is detection FOR?** Because Portal owns an
opaque canvas and guarantees its contrast floors against that canvas, a
mode mismatch is *jarring, never illegible*. Detection's entire payoff is
therefore **aesthetic blending with the surrounding terminal** — and naming that
is what discriminates the signals. Blending wants the terminal's background,
which is OSC 11's question, not 2031's.

**6. The live test settled it.** Lee set macOS to light with Ghostty pinned dark
and captured screenshots. His shell prompt broke badly — it followed the OS, set
*foreground only*, and inherited a background it does not control, so contrast
collapsed into washed-out barely-legible text. Portal, in the same terminal in
the same state, rendered perfectly. Portal paints **both** (leaf
`.Background(canvas)`, the outer `fillCanvas`, and OSC 11 for the gutter), so it
cannot produce that failure whichever signal it follows — but the comparison
shows exactly what following the OS costs anything that doesn't own its
background.

**7. Retroactive correction — the tmux premise was probably never true here.**
Ghostty being pinned dark means Lee's original observation (Portal stayed dark
when macOS went light) was Portal reading the terminal *correctly*. OSC 11 was
working the whole time. The "detection is unreliable inside tmux" premise, which
drove a large part of the research session and sits in the README, is retired.

### Decision

**Detection ships. The signal is the terminal background via OSC 11. Mode 2031
is deliberately not adopted. The user-facing form is Helix's two-slot shape.**

Three arguments carried it:

1. **Transition dominance.** Portal's dwell time is seconds — launch, pick, exec
   into a session, many times a day — so the transition in and out dominates the
   experience. Matching the terminal reads as "your terminal, with a picker in
   it". Matching the OS against a pinned terminal flashes light and drops back to
   dark, twice per use. (Correcting an earlier framing: Portal does not sit
   *inside* anything — it covers the window edge to edge, so the mismatch is felt
   only at the transition.)
2. **A terminal/OS mismatch is usually deliberate, not stale.** Lee's Ghostty is
   *pinned* dark — an explicit choice about the environment Portal lives in. For
   something that lives inside a terminal, the terminal's background is arguably
   the *more* relevant preference, not the weaker one.
3. **Forward compatibility with transparency**, which is deferred-not-rejected.
   A transparent theme *must* follow the terminal background. Choosing terminal
   now makes adding transparency later **purely additive** — no second mechanism,
   no per-theme signal selection. Choosing OS now makes it expensive.

**Config shape** (no `appearance` setting exists anywhere — the override is the
*shape* of the theme setting):

- `theme = "tokyo-night"` — a constant. Detection is never consulted.
- `theme.light = "…"` / `theme.dark = "…"` — opts in; detection chooses.
- A no-answer resolves to the **dark** slot (Helix carries an explicit third
  `fallback` value for exactly this, defaulting to dark; Helix, Neovim, delta and
  Glamour v2 all use dark as the universal no-answer fallback).

### Accepted cost

**OSC 11 is query-only; 2031 pushes on change.** So Portal gets
*correct-at-startup*, not *live-following* — a terminal that flips mid-session
is not noticed until the next launch. Judged thin: terminal backgrounds rarely
change mid-session, and when they do it is usually because the terminal is itself
following the OS.

### Precision — what actually dies, and what does not

An earlier research framing counted the whole appearance gate and the 50ms
first-paint wait as deleted. Under the two-slot form that is **only partly
true**, and the difference matters:

- **Dies:** `prefs.Appearance` (the `auto|light|dark` enum, its tolerant decode,
  `LoadAppearance`/`SaveAppearance`, `WithAppearance`) — replaced by the shape of
  the theme setting. Note `SaveAppearance` has no production caller today, so
  this is mostly read-path removal.
- **Dies via split, not via this decision:** `Token.ColorFor`, `theme.Mode`
  threading, the dual-canvas contrast bookkeeping.
- **Survives, but becomes conditional:** the detect-or-timeout gate. A user on a
  constant theme needs no detection, so their first paint is immediate — a real
  startup win. A user on the two-slot form still needs light/dark resolved
  *before* first paint or Portal paints one theme and flips, so the same race,
  timeout and dark fallback still apply to them.
- **Survives unchanged:** the OSC 11 *query* itself (`restore.go` needs it to
  capture the original background for restore-on-exit, independent of
  detection); the `NO_COLOR` carve-out; and the canvas-echo guard, whose
  comparison re-points from "the mode's canvas" to "the active theme's canvas".

Confidence: **high.** Ratified against research's landed position with the live
test as evidence; the only soft spot is the accepted loss of live-following.

### The one-shot seed job — not shipped

Research separated detection into two independently-optional jobs: **Job 2**,
follow the terminal continuously (what the two-slot form is), and **Job 1**, use
detection once to seed a default for a user who has configured nothing.

Under the locked design, detection acts only when the user nominated a pair — so
a brand-new user with a light terminal gets the dark default and stays there
until they change it. Job 1 would use the OSC 11 reply, on a launch where nothing
is configured, to start on the light built-in instead.

**Options weighed.** It is cheap: the reply arrives anyway for `restore.go`'s
original-background capture, and because nothing is gated on it, it needs no race
and no timeout. Research named the real hazard as scope creep on the trigger —
if it fires on anything other than "the user has never chosen", the between-run
flipping returns, which is what made `auto` *feel* broken even when working as
designed.

**Decision: not shipped.** The two-slot form already gives anyone who cares about
matching their terminal an explicit and reliable way to say so, so the seed buys
one saved setup step at the cost of a second detection consumer with different
semantics from the first. Research A4 also found that **no terminal application
surveyed prompts or seeds on first run** — every one ships a hardcoded default
and starts rendering. "Unconfigured user sees the dark default" is the
convention, not a failure.

---

## Process note — research positions are leans, not decisions

Recorded because it changed how the rest of this session ran.

Research landed roughly eleven decision-shaped positions (terminal-vs-OS signal,
overlay vs shrink-to-fit, apply-on-arrow, persist-on-close, the on-open marker
with no revert key, the contract hint on the panel top, attribution in the repo,
validity as presence-plus-syntax, no panel search, the swap-cost split) and
flagged in its own convergence section that these *"should be re-opened and
ratified in the discussion phase rather than treated as settled by research"*.

Mid-session the question arose whether the discussion was needlessly
re-deriving settled ground. The agreed position, in Lee's words: *"What's in the
research is research. Anything that is decision-shaped still needs to be
ratified, but it does give us a lean … we should be ratifying and discussing
everything, even if it appears to be a decision at the research level."*

So every research position is treated as a **lean with reasoning attached** — it
shortens the discussion but does not replace it, and it can be overturned. Two
in particular have *changed inputs* since research reasoned about them, because
the split decision landed after: the mixed-mode selector list under
apply-on-arrow (research reasoned about arrow-preview under paired, where
arrowing could never flip the canvas), and the fallback-default choice.

---

### Still open under this subtopic

- The concrete file format (JSON / TOML / flat `key=value`) — Portal has no
  third-party parser dependency today; everything is stdlib `encoding/json` plus
  one flat `key=value` file (`aliases`).
- Directory layout and env var; how a built-in and a user theme of the same name
  resolve (two namespaces now exist).
- Whether a theme is a full replacement or a merge over a base, and which base.
- What renders when a persisted theme name no longer exists on disk.

---

## Theme model — split, not paired

### Context

Today a theme is `{20 tokens} × {light, dark}`: `Token{Name, Light, Dark}` with
`Token.ColorFor(mode)` picking the variant, and `theme.Mode` threaded as a
parameter through essentially every render helper in `internal/tui`
(`headerStyle(tok, mode, colourless)` and ~20 files of the same shape) whose only
job is to reach `ColorFor`. The alternative is that a theme is **one palette**
that *is* light or dark, so MV becomes two entries rather than one.

This is the branch that determines what a theme file even contains — 20 values
or 40 — so it had to settle before the format.

### Options Considered

**Paired** (status quo)
- Pros: zero migration, zero call-site churn; theme *identity* survives a
  light/dark flip (you pick "Tokyo Night" and stay on it whichever mode is
  active); the selector is one list where arrowing never changes the canvas
  mode, so live preview is a pure colour change.
- Cons: an author must supply 40 values and clear contrast against *two*
  canvases; dark-only famous palettes (Dracula, Nord) have no light half, so
  "what renders in the hole?" becomes a real rule; `mode` keeps travelling
  through the render layer.

**Split**
- Pros: `Token` collapses to `{Name, Value}`, `ColorFor` disappears, and `mode`
  stops being threaded through ~20 files — a substantial simplification of the
  render layer. 20 values against one canvas. The "missing variant" problem
  *ceases to exist* rather than being handled. Single-palette is the
  overwhelmingly dominant ecosystem shape (~20 tools surveyed).
- Cons: theme identity no longer survives a light/dark flip — "follow the
  terminal" becomes jumping between two *different* themes, which only works if
  the user nominated a pair. The selector list gets longer and mixed-mode, so
  arrowing in a dark terminal will land on light themes and flip the whole
  screen white. It is also a substantial mechanical change to make.

### Journey

Research's central reframe was that **splitting does not remove the pairing, it
relocates it** — under auto-detection something must still know which palette to
use, expressed as a naming convention, two settings, or in-file metadata. That
initially made split look like a wash.

What dissolved that was the observation that **detection and pairing are
independent axes**, and all four combinations ship somewhere. The cell the
earlier framing never considered — auto-detection *with* single-palette themes,
where detection picks between two **named themes** rather than two variants — is
Helix's design and the best-articulated version found anywhere. So wanting
detection does not commit Portal to paired.

Two further corrections mattered:

- **The pairing MV implies isn't real.** Six of MV's light hexes needed
  *individual* correction and three light surface tints were eyeball-pinned at a
  validation gate — by the person who designed the palette. MV's light and dark
  are two independently-tuned palettes that happen to share token names; the
  struct claims a derivation relationship that does not exist.
- **Charm's direction is easy to misread.** Lipgloss v2 moved `AdaptiveColor`
  into `compat`, but the recommended replacement
  (`lightDark := lipgloss.LightDark(hasDarkBG)`) *keeps paired values* and makes
  detection explicit. Charm de-recommended implicit detection, not paired
  colours — so it is not evidence for split.

The decisive argument was the **authoring burden under the PR route just
decided**: 20 values against one canvas versus 40 against two is the difference
between a contributor porting a palette in an evening and not bothering — and
the dark-only palettes have no light half to supply at all.

### Decision

**Split.** A theme is one palette of 20 values and is itself light or dark. MV
splits into two built-ins carrying the existing values.

Consequences:

- `Token` becomes `{Name, Value}`; `Token.ColorFor` and `theme.Mode` threading
  are removed from the render layer.
- Two questions research listed as *dissolved* stay dissolved, because they were
  conditional on exactly this outcome: *"must a user theme supply both
  variants?"* (no such thing under split) and *"how do the theme name and the
  `appearance` pref compose in the selector?"* (no `appearance` pref survives
  split — the override becomes the *shape* of the theme setting, not a mode
  enum).
- The appearance-axis question is therefore **not a standalone decision** — it
  falls out of this one. What remains genuinely open is whether Portal detects
  at all, and what the adaptive two-theme form looks like if it does.
- New mechanic split needs regardless: **how does Portal know a given theme's
  light/dark identity?** Either derive it from the theme's own canvas luminance
  (the canvas is itself a token — Ghostty's theme browser does exactly this) or
  declare it in the file (tinted8 makes `variant: dark|light` required). Owned
  by *light-dark-detection* / *theme-file-format-and-discovery*.
- The mixed-mode selector list is a live-preview experience question for
  *slide-over-panel-design*, not a blocker here.

Confidence: **high**. Both parties reached it independently, and the mechanical
cost is bounded (research verified the render layer resolves tokens per-frame,
so the change is mostly signature removal rather than behaviour change).

---

## Theme validity — full replacement, no merge

### Context

Surfaced by the background review as a live contradiction in this document: the
audience decision inherited research's validity rule ("all 20 tokens present +
syntactically well-formed") while the file-format section listed "full
replacement or merge over a base" as still open, four paragraphs apart. Those
cannot both hold — a merge model makes partial files legitimate by definition,
which negates "all 20 present".

Merge-over-a-base is also the clear modern direction in the ecosystem (gitui
migrated to it deliberately; atuin chains via `parent`, Helix via `inherits`,
yazi merges user `theme.toml` over the flavor), and research noted that
merge-over-default and selectable-base are the *same* feature rather than
competing ones — so this was a real option, not a straw man.

### Options Considered

**Full replacement** — every theme names all 20 tokens.
- Pros: one rule, one failure mode, self-contained files. Preserves the validity
  rule exactly as inherited. No base-resolution semantics to define.
- Cons: "Tokyo Night but with a red cursor" means copying 20 lines to change one.

**Merge over a base** — partial files declaring `base = "…"`.
- Pros: cheap tweaks; matches the ecosystem's direction.
- Cons: needs base-resolution rules (which base — always MV? the selected theme?
  the same-mode built-in?), and drags in a **Portal-specific hazard**: the canvas
  is *itself a token*, so a partial theme supplying a new canvas while inheriting
  `text.primary` from a base produces an inherited foreground measured against a
  background it was never tuned for. Merge can silently compose two individually
  fine themes into an illegible one.

### Decision

**Full replacement. Every theme must declare all 20 tokens.**

The deciding factor is that the go:embed decision already solved the problem
merge exists to solve: because a built-in *is* a file, "copy a built-in and edit
it" is a first-class workflow, and at 20 tokens the copy is trivial. Lee: *"it's
such a small number of tokens that it's really not difficult to create a variant
by just copy and pasting the whole theme and editing."*

Merge was also never discussed as a want — it arrived as an inherited research
option, not a requirement. YAGNI applies; it stays available as a future
addition because full-replacement files remain valid under any later merge
model (a file that declares everything simply inherits nothing).

The validity rule inherited from research therefore stands, now ratified rather
than assumed: **a theme is listed only if all 20 tokens are present AND every
value is syntactically well-formed.** Explicitly not checked: whether the
colours are good, readable, mutually distinguishable, or clear any contrast
floor.

### Vocabulary evolution — reject missing, ignore unknown

#### Context

Raised by the background review as the direct consequence of full replacement:
under "all 20 present", **adding a 21st token is a breaking change for every
user theme simultaneously** — and it arrives from ordinary future UI work, not
from a deliberate naming pass. `TestMVTokenCount` already pins the count at
exactly 20, so vocabulary size is an enforced invariant. Research found no
application-scale precedent for versioning a token vocabulary (the only
mechanisms surveyed were ecosystem-scale: base16 compatibility branches,
tinted8's required `scheme.supports.styling-spec` field).

Two independent levers, mapping to the two directions the vocabulary can move:

- **Unknown key** → ignore or reject. Tolerating unknown keys makes *removing* a
  token survivable (old files keep working).
- **Missing key** → reject or fall back. Tolerating missing keys makes *adding*
  one survivable — but is a back door into the inheritance just ruled out.

#### Options Considered

**Shape A — degrade per-token.** Missing token falls back to a baked-in base
default; the theme still loads and stays selectable; a permanent banner names
what fell back.
- Pros: degrades gracefully in exactly the case that triggers this — when the
  missing token is *brand new*, the surface it paints is brand new too, so the
  foreign colour is confined to something no existing theme ever coloured. Much
  less destructive than a general merge.
- Cons: needs a **new partial-load path** (load, detect incomplete, patch
  specific tokens, carry "degraded" as a state). And the fallback source is not
  trivial under split — a light theme missing `text.primary` cannot borrow the
  dark built-in's `#C0CAF5` (illegible on a light canvas), so "base defaults"
  must mean *the same-mode built-in*, which is merge-with-a-base under another
  name, canvas hazard intact.

**Shape B — degrade whole-theme.** Missing token means the theme is invalid: it
is not selectable, Portal falls back to the default built-in, and a message
names the missing tokens.
- Pros: **reuses machinery Portal needs regardless.** "Persisted theme isn't
  loadable" already has to exist — a user deletes a theme file, renames it, or
  typos the name in `prefs.json`. B routes the new-token case into that same
  path rather than inventing a second one.
- Cons: throws away 20 correct values because of one absent one; on an upgrade
  that adds a token, a user's theme visibly vanishes and Portal reverts.

#### Decision

**Ignore unknown keys; reject missing keys — Shape B.**

The deciding factor was that B's simplicity is real rather than apparent: the
not-loadable path is required anyway, so B adds no new state, while A adds a
partial-load path plus a same-mode-default resolution rule that recreates the
merge semantics and the canvas hazard.

Surfacing, and a deliberate departure from Lee's opening instinct for a
permanent main-UI banner:

- **Not a permanent notice-band entry.** Portal's notice band is a single-slot
  arbiter with six contenders already (filter line → burst progress → transient
  flash → multi-select banner → unsupported banner → no-tags signpost); a
  seventh permanent contender is a real cost for a rare event. Under B the
  symptom is already loud — Portal is visibly the default theme instead of the
  user's — so the message is *explanation*, not alarm.
- **In the slide-over**, consistent with the rejection-surfacing route already
  favoured, naming the exact missing tokens.
- **Plus a `portal doctor` line.** Doctor is Portal's established config-health
  surface and — unlike anything in the TUI — it works on the
  `portal open <target>` exec path, where the picker never renders and no banner
  would ever be seen. Carried to *non-tui-surfaces-and-docs*.

**Corollary (stated here, not separately debated — flag for spec):** falling
back must **not** overwrite the persisted theme name in `prefs.json`. Portal
keeps the user's choice and renders the default; fixing the theme file restores
it on the next launch without the user re-selecting. Overwriting would make the
failure destructive rather than transient.

**Scope note:** this may be near-hypothetical. Portal's own token rule (spec
§2.8) is that a new surface reuses an existing role and a new token is promoted
only where the value genuinely differs — the vocabulary is designed not to grow.

**Open, pushed downstream:** *which* built-in is the fallback default. Research's
read is dark (MV is dark-first by construction, `theme.Mode`'s zero value is Dark
on purpose, and the dark hexes are the pinned authoritative source). Owned by
*built-in-theme-set*.

---

## Summary

### Key Insights

1. **Detection and pairing are independent axes**, not one package — the session
   inherited them fused from earlier framing, and separating them is what let
   the theme model settle without first settling detection.
2. **One path beats two.** The audience decision and the built-ins-as-files
   decision both resolved by preferring a single mechanism used by everyone
   (including the maintainer) over a privileged internal path plus a public one.

### Open Threads

- Research's own convergence flag lists positions it reasoned through that must
  be **re-ratified here, not inherited** — notably the terminal-vs-OS signal
  choice, overlay vs shrink-to-fit, apply-on-arrow, persist-on-close, and the
  slide-over conclusions generally.
- Vocabulary *evolution* (adding or removing a token, not just renaming one) has
  no owner yet — under an "all 20 tokens present" validity rule, adding a 21st
  token invalidates every existing user theme at once.
- Startup cost of an N-file discovery scan is unbudgeted against a cold path
  that is explicitly latency-engineered.

### Current State

- **Decided:** theme audience (curated, two contribution routes); built-ins are
  embedded theme files parsed by the same loader as user themes; theme model is
  split (one palette per theme); themes are full replacements with no merge;
  validity is all-20-present + syntactically well-formed; unknown keys ignored,
  missing keys reject the whole theme with fallback to the default built-in.
- **Uncertain:** the concrete file format, theme identity and discovery
  mechanics, the detection question, the token vocabulary, and the whole
  slide-over surface.

## Triage

(none)
