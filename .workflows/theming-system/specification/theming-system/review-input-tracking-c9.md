# Review Tracking: Theming System - Input Review

Cycle 9. Full fresh pass over the whole specification against the whole discussion source.

## Findings

### 1. Six new flashes are specified onto a band whose existing top contender outranks them, so the report §9.13 exists to guarantee can be silently suppressed

**Source**: `discussion/theming-system.md` — "Vocabulary evolution — reject missing, ignore unknown" (lines 2869–2874: *"Portal's notice band is a **single-slot arbiter with six contenders already** (filter line → burst progress → transient flash → multi-select banner → unsupported banner → no-tags signpost); a seventh permanent contender is a real cost for a rare event"*), read against "A failed commit write (F13)" (lines 1953–1959) and "`NO_COLOR` — block the panel" (lines 1480–1502)
**Category**: Gap/Ambiguity
**Affects**: §9.13 (A failed commit write), §9.10 (`NO_COLOR`), §9.8 (Geometry — entry and forced close), §14A (Flashes); cross-refs §6.3, §10.5

**Details**:

The source enumerates the notice band's precedence **in full and in order** — filter line first, transient flash third — at the one point where it refuses this feature a permanent band entry. The specification carries the *count* from that sentence in two places (§6.3: *"a single-slot arbiter with six contenders already"*; §10.5: the same phrase, used to refuse the migration a notice) but never carries the **order**, and then routes six new user-facing signals through the transient-flash slot:

- `theme picker needs colour — NO_COLOR is set` (§9.10)
- `terminal too narrow / too short for the theme picker` (§9.8 entry)
- `terminal too narrow / too short — theme picker closed` (§9.8 resize)
- `⚠ theme not saved — see portal.log` (§9.13)

**The filter line outranks all of them**, and a filter can be applied-but-unfocused on the Sessions list while the panel is opened, used and closed. Two specified guarantees fail in that state, and both are the exact failure their section was written to close:

- **§9.13's guarantee.** The section spends four paragraphs establishing that a failed commit must be *reported* rather than silent, invents an "outstanding" state so the report survives the close, and pins the flash as the mechanism. With a filter applied, `Esc` discharges the state and the flash never reaches the band — the silent revert, restored, with the state now discharged so nothing re-fires. Cycle 8 settled the flash-versus-flash collision on a forced close; the flash-versus-existing-contender collision is the same class and is not settled.
- **§9.10's guarantee.** `t` under `NO_COLOR` is *proactively* blocked with a flash "rather than letting the user walk into a dead end". With a filter applied the key produces nothing at all — the walkable dead end, arrived at by a different route.

Three things make this worth deciding rather than leaving to implementation. The band is an existing shared mechanism with an existing arbiter, so an implementer adding a flash inherits whatever precedence is already coded — silently. The spec quotes the six-contender figure twice as a *reason* (both times to refuse a new contender), which reads as having consulted the arbiter. And §9.13 explicitly discharges the outstanding state on raising the flash, so a suppressed flash is not merely delayed — the report is destroyed.

The multi-select case composes correctly by contrast and is worth noting as the reason the gap is narrow rather than broad: §9.7 has the panel nest over multi-select with its banner live in the band, and a transient flash outranks a banner, so the theme flashes win there. It is the two contenders *above* flash — the filter line, and burst progress (already closed, since §9.7 swallows `t` during a pending burst) — that need a stated answer.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. §12.4's contents list omits discovery entirely — where a theme file goes, what it must be called, and the env var §5.5 was named for

**Source**: `discussion/theming-system.md` — "Docs" (lines 2214–2219: the doc's contents list), "Discovery — lazy, not startup-scanned" (lines 2640–2647: directory resolution chain, top-level-only enumeration, the `.theme` extension), "On-ramp — `portal theme export`" (lines 1229–1233: the `export … > ~/.config/portal/themes/…` workflow), and "Amendment — slug rules" (lines 235–243: the `[a-z0-9-]` charset)
**Category**: Enhancement to existing topic
**Affects**: §12.4 (`docs/theming.md`); cross-refs §5.1, §5.2, §5.3, §5.5, §12.1

**Details**:

The source's docs list (lines 2214–2219) names four things: the vocabulary with meanings, the file format, the two-slot config, and attribution. §12.4 carries all four and adds three more (ramp ordering, example theme, reserved slugs). **Neither list contains discovery** — and three other specification sections each state or depend on the doc carrying it:

- **§5.5** names `PORTAL_THEMES_DIR` in the spec rather than leaving it to implementation *"because it is a user-facing documented contract — `docs/theming.md` (§12.4) **has to print it**"*. §12.4 has no row it could be printed under. The name is fixed for a doc obligation the doc's own contents list does not carry.
- **§12.1** publishes the two-line drop-in workflow (`mkdir -p ~/.config/portal/themes` then `portal theme export nord > …`) and states *"`docs/theming.md` carries the same two lines"*, with the `mkdir -p` called out as load-bearing because Portal never creates the directory. §12.4 lists the *example theme* from that same section but not the workflow.
- **§5.1/§5.2/§5.3** make the **filename** the identity, constrain it to `^[a-z0-9][a-z0-9-]*$`, and require exactly lowercase `.theme` — user-facing rules whose violation produces `bad name`, a §6.2 reason the panel and doctor both render. §12.4's "file format" bullet is scoped to *contents* ("lexical rules, value domain, the closed key set"), so the rules governing the file's **name** have no documented home at all.

This is the one omission that bites the doc's primary reader. §12.4 opens on the `docs/custom-terminals.md` precedent — *a user-authored config file with its own doc* — and a drop-in author's first three questions are *where does it go, what do I call it, how does Portal find it*, none of which the listed contents answer. §5.6's enumeration rules (top-level only, symlinked files followed, symlinked directories skipped) are the same class: they decide whether a user's file is seen at all.

It also weakens §14's discoverability premise from the other end. §14 accepts that the themes directory is *"silent and never seeded"* and that discoverability rests on `?` help **and `docs/theming.md`** — which only holds if the doc is where a user learns the directory exists.

Worth noting the doc guard (§13.5) does not catch this: it parses the token table and compares the name set against `Theme.All()`, so it polices the vocabulary half and is blind to a missing discovery section.

**Current**:

> ### 12.4 `docs/theming.md`
>
> A new user-facing doc, following the `docs/custom-terminals.md` precedent (a user-authored config file with its own doc).
>
> **Contents:**
>
> - **The 19-token vocabulary with each role's meaning** — the substance of §2.5. `docs/theming.md` is **the source of truth for the public contract.**
> - **The text ramp's weight ordering** — the sole record of it, since file ordering carries nothing (§2.7).
> - **The file format** — lexical rules, value domain, the closed key set.
> - **A complete copy-pasteable example theme** (also the no-terminal on-ramp).
> - **The two-slot config** — `theme` / `theme_light` / `theme_dark`, constant vs adaptive, mutual exclusion, the `theme`-wins hand-edit rule.
> - **The reserved built-in slugs.**
> - **Attribution for ported palettes** — source and link, plus the Nord corrections. Attribution lives in the repo and README, **explicitly not in the UI** (no credits screen, nothing in the slide-over).

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---
