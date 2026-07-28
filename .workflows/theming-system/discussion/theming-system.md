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

## Summary

### Key Insights

*(to be filled as the discussion progresses)*

### Open Threads

*(to be filled as the discussion progresses)*

### Current State

- Nothing decided yet — session starting.

## Triage

(none)
