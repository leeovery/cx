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
  split (one palette per theme).
- **Uncertain:** everything downstream of the file format, the detection
  question, the token vocabulary, and the whole slide-over surface.

## Triage

(none)
