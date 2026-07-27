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

## Triage

(none)
