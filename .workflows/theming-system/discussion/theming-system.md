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

## Token vocabulary & rename

### The two border tokens are one role — consolidate

#### Evidence

Research left one item as genuinely-still-research: *does any surface render both
border tokens together in a way that requires them to differ?* Checked in code
during this session, and the answer is no — the code says so explicitly.

- `panel.go`'s `renderJoinedPanel` doc comment: *"single-tone: the 2-tone footer
  leg of §8.1 was dropped in task 3-4"*. **Spec §8.1's "2-tone border
  (`border.separator` + `border.footer`)" is stale** — the implementation dropped
  it and the helper takes a single border token.
- `border.separator` serves the title rule (`header.go:130`), every modal panel
  frame (destructive-confirm, edit, help, rename), and edit-modal chips.
- **`border.footer` has exactly one production consumer** — `footer.go:267`, the
  footer rule.
- The two carry an **identical light hex** (`#C9CDDB`), differing only in dark
  (`#292E42` vs `#20232E`).

Under split this becomes visible in the artefact itself: the light theme file
would carry two keys with the same value, and the dark file two keys differing by
a shade nothing ever renders side by side.

#### Decision

**Consolidate to one border token.** `border.footer` is dropped; the footer rule
renders with the same token as the title rule. The vocabulary goes **20 → 19**
(`TestMVTokenCount`'s exact-20 pin moves with it).

Accepted visual change: the footer rule becomes marginally more prominent in dark
(`#292E42` rather than `#20232E`) — verifiable directly through the capture
harness. Lee: *"we don't need tokens that are marginally different when they
don't appear together. That's just drift."*

This dissolves the border seed's original premise. There is no longer a
*two*-token rename to settle — there is one border token to name, inside a wider
vocabulary question.

### Scope — full audit of all 19, including the hue-named accents

#### The finding research missed

Research's audit flagged three issues (`bg.track` is use-site-derived; the
`text.on-*` tokens are pairing names coupling to other tokens; the text ramp
encodes no ordering). It did not flag the largest one:

**Six tokens are named after MV's hues, not their roles** — `accent.violet`,
`accent.blue`, `accent.cyan`, `state.green`, `state.red`, `accent.orange`.

For a single built-in this is harmless: MV's primary accent *is* violet. As a
public theming contract it is guaranteed to lie. A Gruvbox port has no violet, so
its author writes `accent.violet = "#d79921"` — Gruvbox yellow — and the key
actively misdescribes its own value. Every port after the first has this problem,
and it cannot be fixed later without breaking their files.

It is the same anti-pattern the border seed logged, but larger in scale and
worse in kind: `border.footer` merely *goes* stale as surfaces reuse it, whereas
a hue name is wrong the moment anyone themes against it.

#### The distinction that defines the target

Three naming failures are in play and only two are failures:

- **A place** (`border.footer`) — wrong; goes stale as surfaces reuse the token.
- **A hue** (`accent.violet`) — wrong; lies in every port.
- **A meaning** (`state.destructive`) — right; stays true regardless of palette
  or where it is drawn.

The third is the target. Crucially this does **not** mean everything becomes
weight-based: research (A7) found use-site naming is the ecosystem *norm* (Helix
names essentially its whole UI half that way). The ramp and the border want
intrinsic-**weight** names because their role genuinely is "how prominent"; the
accents want **meaning** names because a theme author needs to know what a colour
signifies in order to choose one.

§2.1 already defines these roles in exactly those terms — "primary accent",
"key-hint", "state — attached", "state — destructive", "filter/search &
warning", "preview mode-chrome". The role names already exist in the spec; the
tokens simply do not use them.

#### Decision

**Full audit of all 19 tokens**, including renaming the hue-named accents to
their roles. Lee: *"let's absolutely audit that! This is the perfect time to do
so!"*

### The scheme (decided)

Accepted as proposed, including all three flagged spots resolved to the
recommendation: `accent.key` (not `accent.keyhint`), `bg.subtle` (not
`bg.inactive`), and the ramp's ordinal/qualitative join accepted with **file
ordering as part of the contract** — the ramp ships in ramp order with a header
comment saying so (base16's rule that ordering is spec, not incident). Fully
positional names (`text.1`…`text.6`) were rejected: they would remove the
ambiguity but strip all meaning from ~20 files of call sites.

| # | Current | Proposed | Why |
|---|---|---|---|
| 1 | `text.primary` | `text.primary` | unchanged — top of ramp, already intrinsic |
| 2 | `text.strong` | `text.secondary` | ordinal makes ramp position explicit |
| 3 | `text.muted-bright` | `text.tertiary` | current name is self-contradictory |
| 4 | `text.detail` | `text.muted` | `detail` describes content, not weight |
| 5 | `text.dim` | `text.subtle` | ladder consistency |
| 6 | `text.faint` | `text.faint` | unchanged — decorative floor |
| 7 | `text.on-selection` | `text.on-selection` | unchanged — contrast pairing (Crush `onPrimary` convention) |
| 8 | `accent.violet` | `accent.primary` | hue → role (§2.1 "primary accent") |
| 9 | `accent.blue` | `accent.key` | hue → role (§2.1 "key-hint") |
| 10 | `accent.cyan` | `accent.mode` | hue → role (§2.1 "preview mode-chrome" = signals a distinct mode) |
| 11 | `accent.orange` | `accent.attention` | hue → role; one warm token covers filter query, edit-mode, warning flash |
| 12 | `state.green` | `state.positive` | hue → meaning (live / attached / success) |
| 13 | `state.red` | `state.destructive` | hue → meaning |
| 14 | `canvas` | `canvas` | unchanged — already intrinsic |
| 15 | `bg.selection` | `bg.selection` | unchanged — names a state, not a place |
| 16 | `bg.warning` | `bg.attention` | pairs with `accent.attention` |
| 17 | `bg.track` | `bg.subtle` | use-site → intrinsic weight (a low neutral fill) |
| 18 | `border.separator` | `border` | sole border token after consolidation |
| 19 | `text.on-warning` | `text.on-attention` | lockstep with `bg.attention` |

**Three spots flagged as genuinely arguable:**

1. **The ramp's middle join.** `text.tertiary` → `text.muted` mixes an ordinal
   vocabulary with a qualitative one, so the ordering at that join rests on
   convention rather than the names. Mitigation follows base16's insight that
   ordering is part of the *contract*: the ramp ships in ramp order in the file
   with a header comment saying so. Fully positional names (`text.1`…`text.6`)
   would remove the ambiguity but strip all meaning from ~20 files of call sites.
2. **`accent.key`** could read as "important" rather than "keyboard key";
   `accent.keyhint` / `accent.hint` are the alternatives.
3. **`bg.subtle`** reuses the word from `text.subtle` in a different namespace;
   `bg.inactive` is the alternative but generalises less well.

---

## Built-in theme set & quality tiers

### The shipped set

**Four built-ins:**

- **Tokyo Night Dark** and **Tokyo Night Light** — the existing MV values, split
  into two themes by the split decision. Nothing is lost; the palette survives
  as two entries.
- **Nord** (dark-only, as the palette is).
- **One further light theme** (family TBD at implementation).

Two dark, two light. The split alone does not satisfy discovery's "ship at least
one additional theme so Portal launches with genuine options" requirement —
"Tokyo Night" and "Tokyo Night Day" are one identity in two modes — so two
materially different palettes join them.

All four appear in the slide-over alongside anything the user has dropped in
their themes directory.

### Two quality tiers (answers the review's F14)

**Contrast floors apply to what Portal ships; syntactic validity applies to what
users write.**

- **Bundled** (built-in, or an accepted PR — a PR is *intake into this tier*):
  must be valid **and good**. Contrast floors, bands and thresholds are checked.
  It carries Portal's name, and this is what stops the selector filling with
  Portal-endorsed themes nobody can read.
- **Drop-in** (user themes directory): must be **valid only** — all 19 tokens
  present, every value well-formed. Whether it looks good is the user's business.
  Lee: *"we can't control what users do … it has to be valid, but it doesn't have
  to look good."*

This is consistent with the position recorded in research (the strict floor was
for MV, the theme Portal designs) and with floors already being MV-specific in
code.

**Consequence:** porting is not free. A straight palette lift may not clear the
floors unmodified — MV's own light variants needed six individual corrections.
Each bundled theme is real work, which argues for shipping a small number well
rather than a large library.

---

## Shipped default & the theme setting's two states

### Decision — ship the adaptive pair (option b)

Portal ships with the light/dark pair already nominated
(`light = tokyo-night-day`, `dark = tokyo-night`), so a brand-new user gets
whichever matches their terminal, automatically.

The alternative considered was shipping a constant dark default, with the
light-terminal user changing it once.

**Two corrections landed on the way.** First, this was a *gap*, not a reversal:
the earlier decision was to skip the **one-shot seed** (detect once on a virgin
install and persist), which is a different mechanism; the shipped default config
had never been decided. Second, the orchestrator's earlier claim that "every
surveyed application ships a hardcoded default and starts rendering" came from a
research paragraph research itself marked **SUPERSEDED and explicitly refuted**.
The surviving claim is narrower — nobody *prompts* on first run (A4). On
detection, A1 is the opposite: `bat` (`--theme` defaults `auto`), `delta`
(`--detect-dark-light` defaults `auto`), Neovim (`background` auto-set by the TUI
on startup and re-detected when a UI attaches) and `yazi` all detect **by
default**.

Reasons:

- The 50ms is a **timeout, not a price** — terminals that answer do so in
  single-digit ms — and it applies only to TUI launches, since
  `portal open <target>` execs without painting.
- It **degrades to the alternative**: no answer resolves to dark, so (b) is a
  superset of (a) with a bounded downside.
- **Asymmetric escape.** Pinning is one line and is the *simpler* config
  (`theme = "tokyo-night"`), so an annoyed user has an obvious remedy. The
  alternative's failure has no signal at all — a light-terminal user gets a dark
  Portal forever and never learns a light theme exists.

**Risk named:** a terminal that answers OSC 11 inconsistently makes Portal flip
between launches — the exact thing that made `auto` feel broken historically.
Lee's environment is evidence against it (the live test showed OSC 11 working
reliably through tmux); the one-line pin is the remedy.

### There are only two states, not three

"Nothing set" and "pair nominated" are the same thing — the shipped default *is*
an implicit pair. So the loader needs no unconfigured branch, only a default
value for the pair. A theme setting is either:

- **constant** (`theme = "nord"`) — detection off; or
- **adaptive** (`light = …` / `dark = …`) — detection on.

---

## Slide-over panel — interaction model

### Journey

Lee's opening model was: arrow previews live, `Enter` sets a constant **and
closes**, slot keybindings assign dark/light without closing, `Esc` closes.

**First correction — `Enter` and `Esc` cannot both commit.** Research had landed
on *persist-on-close* (apply-on-arrow, save whatever you're on when the panel
closes, with a "saves on close" hint). An explicit `Enter` breaks that: if `Esc`
also saves, `Enter` does nothing `Esc` doesn't. So one must not commit — which
means `Esc` discards. That flips the panel from the settings-panel idiom to the
**picker idiom** (Helix's completion prompt is exactly three-state: `Update`
previews, `Abort` reverts, `Validate` commits).

**Second correction — Lee's own, and it resolves a hole in the first.** If
`Enter` closes, how do you exit after setting *both* slots? Pressing `Enter`
would commit a constant and wipe the pair you just built. His answer: **`Enter`
does not close. `Esc` is the only way out.**

### Decision

- **Arrowing previews only.** The app re-themes live behind the panel; nothing is
  written.
- **`Enter` commits a constant** (`theme = <selection>`). Panel stays open.
- **Slot keys commit to the dark or light slot.** Panel stays open — building a
  pair is inherently two selections.
- **`Esc` closes.** Anything committed persists; an uncommitted preview is
  discarded and the previously persisted theme is restored.

Every write is an explicit keypress; nothing writes on close. This also
eliminates the *"applied but not persisted"* state research flagged as reachable
under persist-on-close — where Portal dies with the panel open and the
visually-applied theme was never written.

Cost accepted: the common case ("pick one and go") is two keys rather than one.
Bought uniformity — one exit key, no dual-purpose keys, and the pair flow needs
no special case.

**Mutual exclusion.** Committing a constant clears the slots; assigning a slot
clears the constant. Whichever was set last wins. This also answers "what if both
a constant and a pair are present" — that state cannot exist.

### Partial pairs — an unset slot holds the shipped default

Lee's question: if you set dark and no light, do you have to set both? What
renders in a light terminal?

**Answer: you never have to set both, because a slot is never empty.** The
adaptive form always has two slots; the shipped values are their *defaults*. So
`dark = nord` yields `{light: tokyo-night-day, dark: nord}` — light is still the
shipped default because it was never overridden. There is no incomplete-pair
state to validate, explain, or render around.

This makes the shipped default and a partially-overridden pair **the same
mechanism** rather than two, which is why it beats the alternatives. Zellij's
rule (set only one slot and the static theme stays authoritative) doesn't
transfer — mutual exclusion means no static theme survives once a slot is set.
"Must set both" would need an invalid intermediate state plus something to render
during it.

Consequence for the panel: it can show both slots' current values at all times,
including ones never touched — you can see what light is set to without having to
remember whether you set it.

**Parked for the mockup:** research settled a marker for the theme active when
the panel opened, but under a pair there are potentially *two* markers (one per
slot) plus a different shape for a constant. Visual question, not a logic one.

### The mixed-mode flash, and list order

Under split + apply-on-arrow, arrowing past a light theme in a dark terminal
flips the entire canvas near-white and back. With four built-ins, two of four
rows do it, every time they are passed. Research could not have reasoned about
this — under paired, arrowing could never change the canvas mode.

**Accepted as correct behaviour, not a defect.** Lee: *"the flash isn't a bug,
it's the feature."* Seeing a light theme as designed is precisely what
live-preview is for, and under the picker idiom it is transient and reversible
(`Esc` restores), which is far milder than a persisted flip.

**List order is alphabetical.** Ordering same-mode themes first was proposed as a
free mitigation — it would turn the surprise into a deliberate act — and
**rejected**: unnecessary once the flash is accepted as the feature.

That keeps the earlier "no variant concept" decision fully intact. Ordering would
have required Portal to derive each theme's mode from canvas luminance; with the
mitigation rejected, the variant concept has **no consumer anywhere** — nothing
declared in the file, nothing derived at load.

### Opening the panel — `t`, on Sessions and Projects

**Key: `t`.** Free on Sessions (taken there: `/ s x m k d e r ? Space Enter Esc`
plus arrows) and the obvious mnemonic.

**Pages:**

- **Sessions** — yes; the default page and the richest surface to preview
  against.
- **Projects** — yes. Theme is a *global* setting; refusing would make it feel
  page-scoped for no reason, and `t` is free there.
- **Preview** — **no.** The preview body is captured real ANSI scrollback that is
  deliberately out-of-theme, so live preview would only re-theme the frame
  chrome — a weak surface. It is also already a full-screen overlay, so the panel
  would stack an overlay on an overlay. `Esc` out and change it.
- **Modals** — no; modals are key-exclusive by design.

**Considered and ruled out:** research noticed a latent question about what a
"settings surface" in Portal is — after this work `prefs.json` holds exactly two
things, grouping mode (changed by the `s` key cycle) and theme (changed by a
panel). Making the slide-over a general settings panel that also swallows
grouping mode would resolve that inconsistency, but `s` is fast and good, and it
is scope creep on an already-large feature. Two mechanisms for two prefs is a
mild inconsistency worth living with.

### Panel geometry — degrade, don't refuse

**Width.** Fixed preferred width (~24–30 columns: name, markers, slot
indicators, border, padding), with long user slugs truncated `…` as Portal
already does for session names. A fixed width is predictable to lay out against;
content-driven width would make the panel jump around as the library changes.

**Narrow terminals — the orchestrator's first proposal was wrong.** It suggested
*refusing to open* below a threshold of "panel width + the app's documented
40-column minimum", citing the multi-select precedent (`m` is proactively blocked
at entry on an unsupported terminal with a flash, rather than letting the user
walk into a dead end).

Lee pushed back: *"it would have to be very narrow for that to be the case. Like
on a mobile, I would still expect it to open."* He is right, and there is a
principled reason the precedent doesn't transfer: **multi-select is blocked
because of a capability *absence* — the terminal genuinely cannot spawn windows.
A narrow terminal is a space *shortage*, and §2.7's doctrine for space shortage
is explicit: degrade, never break.** Refusing on width would contradict Portal's
own established stance.

So:

- The panel **shrinks** between a preferred and a minimum width as the terminal
  narrows — staged degradation, consistent with §2.7's existing width steps
  (drop right-side header hint → compact wordmark → truncate names).
- It **refuses only when even the minimum panel cannot render**, which is very
  narrow indeed — and then it flashes rather than opening a broken frame.
- Exact thresholds are pinned at implementation, as §2.7 already does for its
  own degradation steps.

**What the panel covers.** Overlay hides the right-hand column, where the
footer's right-aligned `? help`, the right-side header hint, and session row meta
live. **Accepted** — and for a better reason than transience: the theme is
carried almost entirely by the *left* of the screen (session names, cursor bar,
group headers, footer key glyphs), while the right edge is metadata. The overlay
covers the least theme-informative part of the screen, which is exactly what a
preview surface wants.

### Precedent — mixed, and honestly thin in one place

For the picker itself it is strong: Helix's `:theme` re-themes live without
restart behind a three-state prompt; Ghostty's `+list-themes` is close to the
described layout (list one side, live preview the other, `Esc` to exit, `f`
cycling a light/dark filter); kitty's themes kitten has live preview and a
recently-used list.

For **assigning a theme to a light/dark slot from inside a picker, nothing was
found.** Helix, Ghostty, Zellij, kitty and bat all require editing config for the
pair. The slot keys are genuinely novel — not a reason to avoid them, but the
reason a Paper mockup earns its keep, since there is no established shape to
borrow.

### Paper spike — two marker/slot treatments

Two artboards added to the **Portal** Paper file, both built on the canonical
`Sessions — Modern Vivid v2` frame so they inherit the shipped MV conventions
(JetBrains Mono, 32px rows, `#0B0C14` canvas, `#28243A` selection tint,
`#BB9AF7` cursor, panel chrome `#0C0C16` on `#2B3050` border):

- **`Theme slide-over — A (inline slot badges)`** — slot assignment shown as a
  right-aligned `dark` / `light` badge on the assigned rows.
- **`Theme slide-over — B (assignment header)`** — a `dark → … / light → …`
  key-value block pinned under the panel header, with a plain list below.

Shared decisions expressed in both frames:

- **Full-height, flush to the right edge, left border only** — deliberately
  *not* an inset bordered panel like the modals, so it reads as a slide-over
  rather than a floating dialog.
- **Non-blanking overlay.** The Sessions list stays fully visible; the panel
  covers the right column, which visibly costs the footer's `x projects` and
  `? help` — exactly the accepted trade recorded above.
- **Vertical keymap footer** (`⏎ set theme` / `d set as dark` / `l set as light`
  / `esc close`) rather than Portal's horizontal footer row — a horizontal keymap
  does not fit a ~30-column panel, and the vertical form matches the help
  modal's key-column idiom.
- **Cursor row** uses the shipped selection treatment (`▌` + tint + white bold
  name), so the panel's list reads as the same kind of list as Sessions.
- The list shows a **user drop-in theme** (`nord-lee`) sitting alphabetically
  among the built-ins, with no visual distinction — deliberate, matching the
  decision that a valid drop-in is simply selectable.

**Treatment chosen: A.** B's assignment block is more legible at a glance, but it
puts theme names in a second place, pushes the list down, and with only two slots
the badges say the same thing without the extra region. A also scales better as
the library grows, since a badge stays attached to the row it describes. Lee's
accepted caveat: with a very long list the assignments could scroll out of view —
judged fine, since a user knows what they picked and can scroll to find it.

**The `●` was restored, and the reasoning for dropping it was wrong.** The spike
removed the badge's leading `●` on the grounds that the glyph already means
*attached* in the sessions list. Lee corrected this: Portal **already
repurposes** `●` for multi-select marking, where it indicates a marked row rather
than a live session. So `●` is Portal's general "marked / active" glyph, not an
attached-only one, and using it for slot assignment is consistent with existing
practice rather than a collision. The two signals stay independent: `●` marks
*assignment*, the `▌` + tint cursor treatment marks *browse position*.

One refinement that stood: the panel header dropped a theme count (noise at this
list size).

### The constant state — third frame

`Theme slide-over — A (constant set, previewing another)` completes the panel's
specification, since the two setting states never coexist on screen: it shows a
**constant** theme (`nord`, carrying a bare `●` with no slot word, exactly as
multi-select marks a row) while the **cursor sits on a different theme**
(`tokyo-night`, which the app behind is therefore rendering).

That combination is the picker idiom made visible: the `●` is what is
*persisted*, the cursor + canvas is what is *previewed*, and `Esc` would restore
the marked one. It is also why the bare `●` is sufficient — with no slots there
is nothing to qualify, and the label would be redundant with the marker.

### Sequencing — model first, then mock

Agreed to settle the interaction in words before mocking, so the Paper frame
expresses a decided model rather than exploring one. The panel still has
undecided structure that changes the frame (width, what sits behind it,
narrow-terminal behaviour, whether it opens from pages other than Sessions). The
frame then becomes the reference artifact carried into spec and planning.

---

## Theme plumbing — swap `mode` for the theme

### Context

Research called this the feature's one real architectural tradeoff: `theme.MV` is
a package-level global read directly at **182 call sites**, so making the active
theme switchable meant choosing between

- **mutable package state** (`theme.Active` var + setter) — near-zero call-site
  churn, but package-level mutable state on the render path, and render becomes
  order-dependent on the setter; or
- **explicit plumbing** — pass a `Theme` down through the renderers; idiomatic
  and test-friendly, but a large mechanical change plus a third parameter on
  every render helper that already threads `mode` and `colourless`.

### The split decision dissolved the tradeoff

Every call site reads `headerStyle(theme.MV.TextPrimary, mode, colourless)` — and
**split removes the `mode` parameter**, because a theme no longer has variants to
resolve between. So all 182 sites are being edited regardless, and a parameter
slot is freed at exactly the same moment.

That guts the case for mutable global state: its entire advantage was avoiding
churn Portal is now paying anyway.

### Decision

**The model holds the active `Theme` and passes it where `mode` is passed
today** — a straight substitution of one threaded value for another, in code
already being touched. No package-level mutable state, no new parameter.

Secondary benefit that matters in this codebase specifically: a test can
construct a model with any theme instead of mutating a global and hoping nothing
else observed it. The suite already forbids `t.Parallel()` because the cmd
package injects mocks via package-level mutable state — adding another global
would push in the wrong direction.

---

## Live-swap mechanics

### Speed is a non-issue, and "bake in on exit" is rejected

Research verified the cheap path already exists and already excludes the
expensive one:

- **Restyle** — `applyCanvasMode` swaps the delegate and re-points the cached
  style structs `bubbles/list` holds. O(1), no I/O, no list content touched. It
  is already exercised in production: it is what runs when the OSC 11 reply lands
  after first paint.
- **Rebuild** — `rebuildSessionList` re-derives the item list and, in grouped
  modes, runs the lazy dir-resolution pass with its per-session tmux pane reads
  (the known ~0.5s By-Project cost at ~38 sessions).

**`applyCanvasMode` does not call `rebuildSessionList`.** So the original ask —
"take the shortest path to updating colours, defer heavier work to panel exit" —
needs no deferral mechanism: nothing heavy is on the path.

The premise that the re-render is the cost is also wrong. Bubble Tea rebuilds
the whole view string on *every* keypress regardless, diffs it, and writes only
changed cells — holding the down arrow in the sessions list already does this
dozens of times a second. A theme swap costs one ordinary keypress plus the
style re-point.

**"Bake in on exit" is therefore rejected**: nothing is left un-baked, and
deferring work to panel close would create a visible discontinuity at the one
moment that should be seamless.

### The real risk is completeness — guarded behaviourally

Threading the theme (above) fixes most of this: anything taking the theme as a
parameter re-derives per frame. What remains is the **cached styles Portal does
not own** — `bubbles/list`'s help styles, pagination dots, TitleBar, and both
filter inputs — which are assigned once. That list is **hand-maintained with no
guard test**, unlike the colour-literal rule which has an AST glob guard. Miss a
site and the element silently keeps the previous theme's colours until something
else re-renders it.

Two specific offenders research already found: `pagepreview.go:35` copies a
`Token` at **package init**, so it would never see a swap; and init-time copies
of *derived styles* were never swept for at all.

**Decision — a behavioural guard, not a structural one.** Render every capture
fixture under theme A, swap to theme B, render again, and assert **no theme-A hex
survives** in the output. The offline capture harness already renders every
canonical screen deterministically through the shared `tui.Build` constructor
with every tmux seam faked, so the fixture list *is* the coverage list, and it
grows automatically as screens are added.

This beats a structural guard because it catches *any* missed site — including
ones added years later — without anyone having to remember a rule. A structural
guard would have to recognise "this is a cached style" in the AST, which is not
mechanically well-defined.

The two known offenders are fixed outright rather than guarded around: the
package-scope `Token` copy goes, and `canvasHexFor` stops referencing `theme.MV`.

Confidence: **high on the approach; Lee deferred to the recommendation** rather
than holding a position ("whatever you think there is the right thing to do").

### OSC 11 re-emission and the canvas-echo guard

- **No per-keystroke churn.** Bubble Tea v2 **diffs** the view's background
  colour and emits only on change (`cursed_renderer.go:411-432`), so hovering N
  themes emits OSC 11 exactly once per *distinct* canvas landed on — the minimum
  the feature requires. The declarative per-frame `BackgroundColor` assignment is
  not a per-frame write.
- **The echo guard needs no new race handling.** It exists because the startup
  OSC 11 *query reply* can race Portal's own canvas set. The query is issued once
  from `Init`; a later theme switch issues no new query, so it creates no new
  race. The guard only ever needs to compare against the canvas active during the
  *startup* window.
- **But it does need work**, and it lands on the one mechanic carrying an
  explicit "do **not** drop this guard" warning: `RestoreTerminalBackground`
  currently derives its comparison value *at exit* from `m.canvasMode` via
  `canvasHexFor`, which reads `theme.MV.Canvas` **directly** — a hardcoded MV
  reference outside the token render path. Anchoring to the startup canvas means
  capturing and retaining that hex as model state, and making `canvasHexFor`
  theme-agnostic.

### Concurrent Portal instances (the review's F12)

Portal's multi-window burst routinely produces several concurrent processes, so
multiple live instances are normal rather than an edge case. Behaviour under the
decided design: each instance loads its theme at construction; an instance that
changes theme persists it; other instances are unaffected until relaunch (there
is no file watch — discovery is lazy, and `fsnotify` was rejected). `prefs.json`
is last-write-wins.

**This is exactly how `session_list_mode` already behaves** — the `s` toggle
persists per-instance with no cross-instance sync, via the existing
`ModePersister` seam that a theme persister follows. No new hazard, no new
mechanism.

---

## Non-TUI surfaces, logging & docs

### `portal doctor` — a read-only theme health line

Doctor is Portal's established config-health surface and the **only** surfacing
route that works on the `portal open <target>` exec path, where the picker never
renders and no in-TUI message would ever be seen. It:

- scans the themes directory and reports any file failing validity, with the
  reason;
- reports when the persisted theme name no longer resolves.

**Read-only, with no `--fix` action.** Doctor can prune a stale hook entry;
it cannot repair someone's colours.

### Ruled out as YAGNI

- **`--theme` flag** — a third way to set a theme, for a single launch nobody
  asked for. (Note this was the escape hatch named in the *reversed* pre-canvas
  stance, so it arrives with no live constituency.)
- **`portal theme list` verb** — the slide-over lists them; doctor validates
  them.

### A new `theme` log component

Portal's log component taxonomy is **closed and spec-governed** — components are
never invented at a call site. `terminals.json` maps to the *empty* component
because "terminals" is not in the vocabulary, making it silent in `portal.log`.

**Decision: add a `theme` component** via spec amendment. Precedent is direct —
`spawn` and `resolve` were both added by the features that needed them, as
spec-governed amendments. Events worth recording: theme loaded, theme rejected
with reason, fallback applied.

What distinguishes this from `prefs` and `terminals` (both deliberately outside
the vocabulary) is that those are **dumb stores with no runtime behaviour**,
whereas the theme loader has parse/validate/fallback *outcomes*. It also closes
the gap the review raised as F10/F9 together: without a log component, a broken
theme on the exec path leaves **no trace anywhere**, since doctor is manual and
the slide-over never opens. Lee: *"logging is good because it helps diagnose
problems."*

### Docs

`docs/theming.md`, following the `docs/custom-terminals.md` precedent (a
user-authored config file with its own doc). Contents: the **19-token vocabulary
with each role's meaning** — the seed's own deliverable, and far more load-bearing
now the names are a public contract — the file format, the two-slot config, and
attribution for ported palettes (attribution lives in the repo and README,
explicitly **not** in the UI).

**README/CHANGELOG consequences.** `appearance` is described in `README.md` at
four places, including a paragraph recommending users pin it *"when
auto-detection misfires (for example under tmux passthrough)"*. That comes out
with the setting — and the advice is obsolete twice over, since the premise was
probably never true in the first place.

---

## Loose ends closed

### Persisted theme missing from disk

Ratified explicitly rather than left implied: a persisted theme name that no
longer resolves takes **the same path as a rejected theme** — fall back to the
default built-in, keep the persisted name (never overwrite), and surface via the
slide-over, `portal doctor`, and the `theme` log component. One not-loadable
path serves deletion, renaming, a typo in the name, and a missing token alike.

### What a theme file may contain (the review's F8)

Answered by the accumulated decisions, but worth stating: a Portal theme file
contains **exactly the 19 token keys plus an optional `name`**. Unknown keys are
ignored; there is no behaviour, no includes, no nesting.

This means Ghostty's documented caveat — *a theme can set any config option, so
don't use untrusted ones* — **does not transfer**. Portal's theme file is a
closed key set of colour values with no capacity to influence anything else, so
ingesting an unreviewed drop-in file carries no configuration-injection surface.

### Terminal colour capability (the review's F13)

Raised as an open question: a floor validated on a truecolor hex says nothing
about the colour actually painted after `lipgloss`/`colorprofile` downsamples on
a 256- or 16-colour terminal, and Helix's `is_16_color()` check refuses truecolor
themes on incapable terminals.

**No action, deliberately.** Spec §2.4 already accepts downsampling as graceful
degradation for MV — "a hue may approximate, but the contrast floor still governs
legibility" — and nothing about user themes changes that. Bundled themes are
floor-checked on their truecolor values exactly as MV is today; drop-ins are
syntactic-only by decision, so there is no floor to invalidate. Helix's check is
real validation on an axis Portal has already chosen not to police.

---

## Capture harness & tests

### The premise correction — committed PNGs were scaffolding, not an asset

The orchestrator analysed this as a **themes × fixtures matrix problem**: 43
committed reference PNGs × 4 built-ins = 172 images on a harness already recorded
as flaky-on-write, with every accepted PR theme adding 43 more. That looked like
a wall.

**Lee: the committed references were never meant to persist.** They existed so he
could watch the redesign come to life during implementation — *"it was never
really designed to be kept … I should have dictated that those weren't persisted
once the feature was implemented."* With no regression obligation, the matrix
obligation disappears entirely and the cost analysis collapses.

(Note this contradicts how `CLAUDE.md` currently describes `testdata/vhs/` — as
committed reference PNGs forming a visual-verification harness — which reads as a
durable asset. Worth correcting when the docs are updated.)

### The constraint that makes `capturetool` load-bearing

Recorded prominently because it explains why this tool exists and must keep
working:

**Lee cannot run Portal from a temporary build to check a visual change.** A
scratch build interferes with the live system — it disturbs the running daemon,
its bootstrap sequence touches real state, and *"even when you try to sandbox it,
there are issues"*. So `capturetool` is not a convenience; it is the **only
viable route** to seeing a visual change before release.

This also explains and endorses the fixtures' deliberate shallowness: *"they do
just enough to visualise what you're meant to be visualising, and then that's
it."* Fixtures are about **look, not behaviour** — they need not be functionally
complete.

### Decisions

- **`capturetool` and `internal/capture` survive and are open for edit.** Whatever
  the tool needs to work with the new system is in scope for this feature — no
  separate redevelopment work unit. Lee: *"whatever you have to do to the
  capturetool to make it work with the new system, that's what we have to do."*
- **Everything the tool previously produced is deleted.** The committed reference
  PNGs go; they could not survive the token rename and the theme split without a
  full recapture in any case, and they are explicitly not wanted as a permanent
  asset.
- **The harness must be repaired for the theme change**: `tui.Build` takes a
  *theme* where it took a `prefs.Appearance` (the exact injection mechanism this
  work removes), and `capturetool` gains a `--theme` flag. Without this the
  harness can only ever render the compiled-in default.
- **New fixtures are added for the slide-over** — the adaptive-pair state, the
  constant-while-previewing state, and the narrow degraded panel — so the panel is
  visible during implementation rather than at release.
- **`internal/capture` must stay alive regardless** of what happens to the
  images: the swap-and-diff completeness guard drives the fixture *renderer*.

### Guard tests reshape

- **`TestMVTokenCount`** moves 20 → 19, and its meaning shifts from "MV has 20
  tokens" to "the vocabulary is 19".
- **`TestMVDarkVariantsPinned` is deleted.** Once themes are data files whose
  values are their own source of truth, an exact-hex pin in a Go test is a
  change-detector duplicating the file. The contrast floor test is the real guard
  for bundled themes.
- **`TestLightSurfaceTintsPinned` survives** — and this closes the thread opened
  when the erratum comments were dropped (the review's F5). Three light surface
  tints are **not numerically checkable** (light-tint-on-light-canvas is
  numeric-insufficient; they were confirmed by human eyeball at a validation
  gate), so for those the exact-value pin is the only guard and the comment was
  the only record of the judgement behind it. They keep their pin, and the *why*
  moves into the theme file as a comment — which the flat `key = value` format
  supports. **The format decision is what makes deleting the Go-side erratum
  comments safe rather than lossy.**

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

### Discovery — lazy, not startup-scanned

#### Context

Surfaced by the background review: auto-discovery turns one config read into an
**N-file scan-parse-validate sweep**, on a cold path that is explicitly
latency-engineered — the concurrent-bootstrap flip exists for that path, and this
very feature counts a 50ms first-paint wait as a cost worth removing. An
unbudgeted per-launch sweep would take back exactly what the detection decision
just won.

#### Options Considered

- **Scan at startup** — simple, whole list always ready. Pays the sweep on every
  launch including the overwhelming majority where nobody opens the selector, and
  `portal open <target>` execs straight through with no TUI at all.
- **Lazy** — load the *selected* theme by name at startup (one file read, no
  enumeration); enumerate the directory only when the slide-over opens, where a
  few milliseconds is invisible against a keypress.
- **Watch with `fsnotify`** — k9s's approach on its skins directory, with a
  listener notifying UI components. Machinery for a problem Portal doesn't have:
  you cannot edit a theme file while looking at Portal.

#### Decision

**Lazy.** The cold path costs exactly one extra file read regardless of how many
themes exist — which also means the drop-in route can never degrade startup no
matter how many files a user accumulates.

Details settled with it:

- **Directory resolution** follows Portal's existing per-file chain shape:
  dedicated env var → `XDG_CONFIG_HOME/portal/themes/` → `~/.config/portal/themes/`.
  Note this is a *directory*, where `configFilePath` resolves *files* — a small
  mechanical difference for the spec to pin.
- **Enumeration is top-level only** — files matching the theme extension in the
  directory itself; no subdirectory recursion, no symlink chasing.
- **Extension:** `.theme` (recommended, low stakes) — the slug is the filename
  minus the extension, so *some* extension is needed for slug derivation.

### Still open under this subtopic

- What renders when a persisted theme name no longer exists on disk — expected to
  fall into the same not-loadable path as a rejected theme (fall back to the
  default built-in, keep the persisted name), but not yet explicitly ratified.

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
