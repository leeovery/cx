# Review Tracking: Theming System - Input Review

Cycle 8. Full fresh pass over the whole specification against the whole discussion source.

## Findings

### 1. `portal theme export` never inherits the `unreadable`-versus-`not found` discrimination the rest of the feature is built on

**Source**: `discussion/theming-system.md` — "`portal theme export` — command surface (the review's F5)" (lines 2042–2053: *"**Slug domain: built-ins *and* drop-ins.** Resolving both makes export a diagnosis tool"*, *"an **unknown** slug likewise"*), read against "Write-path robustness … The themes directory itself (F11)" (lines 1948–1951) and "Reason vocabulary extended to seven (F8)" (lines 2536–2540)
**Category**: Gap/Ambiguity
**Affects**: §12.1 (`portal theme export <slug>` — command surface table), §14A (export copy); cross-refs §5.5, §13.3

**Details**:

§5.5 fixes a discrimination the specification treats as important enough to state as a rule: *"A theme made unreachable by an unusable directory carries the reason `unreadable`, not `not found`. The distinction is the one §9.4 draws for a persisted slug: `not found` sends the user to check the filename, `unreadable` sends them to check permissions — and permissions is the actual problem."* It then enumerates where the rule applies:

> It applies uniformly to the reason attr on `theme: fallback applied` (§12.3), the terse reason on the persisted-slug rows rendered beneath the pinned directory row (§9.5), and doctor's line through §14A.

**Export is the fourth by-name resolver and is absent from that list.** §10.5 states it explicitly — *"Its argument is a slug, which resolves by name against the embedded set and then the themes directory (§8.4's ordering)"* — so export hits the identical condition set: an unreadable themes directory, a regular file where the directory belongs, a dangling symlink, an EACCES on the file itself. §12.1's command-surface table admits only two failure shapes:

| **Invalid drop-in** | Refused, with its reason on **stderr** and a **non-zero exit**. |
| **Unknown slug** | Same — reason on stderr, non-zero exit. |

and §14A pins the unknown-slug copy as `no theme named <slug>`.

So `portal theme export nord-lee` against a `themes/` directory the user cannot read prints **"no theme named nord-lee"** about a file that plainly exists — the precise misdirection §12.1 refuses one row later for a charset failure (*"telling a user their file is missing when they typed an illegal name sends them looking in the wrong place"*), and the one §5.5 and §9.4 each go out of their way to close.

Two things sharpen it:

- **The reason already exists.** `unreadable` is one of §6.2's seven, and §14A already pins its detail format for doctor (*"The OS error verbatim — it is the only thing that distinguishes a permission denial from a dangling symlink"*). Export needs no new vocabulary, only a row.
- **The sibling surface has it.** §13.3 gives `capturetool --theme <path>` an explicit reason set including `unreadable`. Export — the surface a user actually reaches — is the one without it.

Also unstated in the same table: whether export's `<reason>` frame (`theme <slug> is not valid: <reason>`) is even the right frame for a directory-level failure, since the file is not invalid — nothing was read.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §12.1's table gains an `unreadable` row so export inherits §5.5's discrimination as the fourth by-name resolver, with the misdirection it closes named (exporting against an unreadable directory would print "no theme named X" about a file that exists). §14A gains a separate stderr frame — `theme <slug> could not be read: <OS error>` — because the file is not *invalid*; nothing was read.

---

### 2. A forced close with a failed commit outstanding raises two flashes into a one-slot notice band

**Source**: `discussion/theming-system.md` — "Panel vertical axis (the review's F6)" (lines 2055–2068: *"**Resize while open: degrade in place**, closing with a flash only if the terminal falls below the render floor"*) and "A failed commit write (F13)" (lines 1953–1959: *"reports inside the panel, **keeps the theme applied in memory**"*). Decided in different review rounds and never composed.
**Category**: Gap/Ambiguity
**Affects**: §9.13 (A failed commit write), §9.8 (Geometry — forced close), §14A (Flashes)

**Details**:

The specification composes each of these two source decisions with the close path *separately*, and both land on the same single-slot notice band in the same event:

- §9.8: a resize below the floor closes the panel **with a flash** — `terminal too narrow — theme picker closed` / `terminal too short — theme picker closed` (both pinned in §14A).
- §9.13: *"closing the panel with a failed commit outstanding raises a main-screen flash: `⚠ theme not saved — see portal.log`"*, and the state is cleared only by a subsequent successful commit — *"nothing else clears it"*.

A forced close **is** a close, so on a forced close with an outstanding failure both fire at once, into a band §9.13 itself describes as having *"one slot"*. The specification notices the collision but only uses it as an argument for the discharge rule:

> Without that, reopening the panel and pressing `Esc` would re-fire the flash about a failure already reported, on every close for the life of the process, and §9.8's forced close would stack it against `terminal too narrow — theme picker closed` in a notice band with one slot.

Discharge fixes the *repeat* firing. It does not decide the simultaneous case, and two sub-questions follow that an implementer must answer silently:

1. **Which string wins.** They report different things — one an unsaved setting the user must act on, the other a transient geometry event — and §9.13's whole argument is that the commit failure *"reports a state the user must act on"* and must not be lost.
2. **Whether the state is discharged when its flash loses the slot.** If it is, the report the section exists to guarantee is dropped exactly once, in the one path where the user also cannot reopen the panel to retry (the terminal is below the floor). If it is not, the state persists and re-fires on the next ordinary close — which is the behaviour the discharge rule was added to prevent.

Worth noting the path is not contrived: §9.8 already names the forced close as *"the state §11.4 names as the one where a colour the user never chose can survive Portal's exit"*, so it is a path the specification treats as live rather than theoretical.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §9.13 now settles the simultaneous case: on a forced close both flashes are due and the failed-commit flash wins, with the state discharged. Reasoning recorded — the geometry event is self-evident to the user, the unsaved setting is not, and losing the commit flash on the one path where the panel cannot be reopened to retry is the exact failure the section closes.

---

### 3. The `appearance` translation is specified as having no user-facing surface at runtime; the specification never says so

**Source**: `discussion/theming-system.md` — "Upgrade path for an existing `appearance` value (the review's F4)" (lines 1888–1892: *"It fires **exactly once ever**, gated on an explicit `theme_migrated` marker … The `theme` log component records it, giving a forensic trail **with no user-facing interruption**."*)
**Category**: Enhancement to existing topic
**Affects**: §10.5 (Ownership and write-path robustness); cross-refs §10.2, §12.5, §6.3, §14A

**Details**:

The source states the translation's user-facing behaviour in the same sentence that assigns it a log line: the trail is forensic, and the user is **not** interrupted. The specification carries the log line and drops the second half, so nothing anywhere says the migration is silent to the user at runtime.

That is a live hole rather than an obvious default, because the specification's own reflexes point the other way:

- §10.1 frames the problem as *"a user … upgrades into the shipped adaptive pair and **silently gets a light Portal** with nothing explaining why"*, which reads as an argument against silence — when the actual resolution is that the translation preserves intent so precisely that there is nothing to explain.
- §9.13 spends a section establishing that a state the user must act on has to be *reported*, not silent, and §14A pins a flash for it. An implementer applying that reflex to a one-shot config mutation would add a `theme setting migrated` flash in good faith.
- §6.3 rejects a permanent notice-band contender for a *rarer* event on the grounds that the band is a single-slot arbiter with six contenders already — the same reasoning applies here and is not invoked.
- §14A claims to pin *"every new user-facing string"*. If the migration is silent, that claim is true; if it is not, a string is missing. The claim is only checkable once the silence is stated.

The compensating channel is already specified — §12.5 requires the CHANGELOG to carry *"that **`appearance` is replaced and translated automatically** … so a user who set it does not have to act"* — which is the honest place for the notice precisely because the translation runs at prefs load, before any surface exists to render one.

**Current**:

> §10.5: The translation emits `theme: appearance migrated` (INFO, one-shot) — see §12.3.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §10.5 now states the translation is silent to the user at runtime, with the three reasons (intent is preserved exactly so there is nothing to explain; it runs before any surface exists; the notice band is a six-contender single slot §6.3 already refused for a rarer event) and the note that the spec's own §9.13 reflex points the other way. CHANGELOG named as the compensating channel.

---

### 4. The source's precedent survey — strong for the picker, empty for the slot keys — is absent entirely

**Source**: `discussion/theming-system.md` — "Precedent — mixed, and honestly thin in one place" (lines 1580–1592: *"For **assigning a theme to a light/dark slot from inside a picker, nothing was found.** Helix, Ghostty, Zellij, kitty and bat all require editing config for the pair. The slot keys are genuinely novel — not a reason to avoid them, but the reason a Paper mockup earns its keep, since there is no established shape to borrow."*)
**Category**: Enhancement to existing topic
**Affects**: §9.14 (Reference frames); cross-refs §9.5 (marker treatment A), §13.3 (panel fixtures)

**Details**:

This is a whole source subsection with no representation anywhere in the specification. It splits the panel's design into two halves with opposite risk profiles:

- **The picker half has strong prior art** — Helix's `:theme` re-themes live behind a three-state prompt, Ghostty's `+list-themes` is close to the described layout (list one side, live preview the other, `Esc` to exit), kitty's themes kitten has live preview. The specification cites ecosystem precedent freely elsewhere when it justifies a decision (Ghostty/kitty in §7.1, Helix's split-plus-detection design in §3.1, btop in §4.1, *"Every comparable tool lists slugs"* in §5.1), so the omission reads as "not surveyed" rather than "judged irrelevant".
- **The slot half has none.** `d`/`l` assignment from inside a picker, and the `● dark` / `● light` / `● both` badge vocabulary that expresses it, exist in no surveyed tool — every one of Helix, Ghostty, Zellij, kitty and bat requires editing config for the pair.

The consequence is the part worth carrying, because two specification decisions rest on it. §9.14 presents the three artboards as *"the forward-looking reference for this panel"* without saying why they were built, and §13.3 requires panel fixtures *"so every specified panel surface is visible during implementation rather than at release"*. The source's answer — that for the slot idiom there is **no established shape to borrow**, so the frames and the fixtures are the only reference there is — is what makes both non-negotiable rather than nice-to-have, and it tells a reviewer where to concentrate the visual gate: the picker half can be checked against a familiar idiom, the slot half cannot be checked against anything.

**Current**:

> ### 9.14 Reference frames
>
> Three Paper artboards are the forward-looking reference for this panel, all built on the canonical `Sessions — Modern Vivid v2` frame so they inherit the shipped MV conventions:
>
> - `Theme slide-over — A (inline slot badges)` — the adaptive-pair state
> - `Theme slide-over — A (constant set, previewing another)` — a constant `●` on one row while the cursor sits on a different theme
> - `Theme slide-over — B (assignment header)` — the **rejected** treatment, retained as the record of what was weighed

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §9.14 gains the precedent split before the artboard list: the picker half has strong prior art (Helix, Ghostty, kitty) and can be checked against a familiar idiom; the slot half exists in no surveyed tool, so the frames and fixtures are the only reference there is — which is what makes them non-negotiable and tells a reviewer where to concentrate the visual gate.

---
