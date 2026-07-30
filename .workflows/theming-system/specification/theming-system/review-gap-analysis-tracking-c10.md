# Review Tracking: Theming System - Gap Analysis

## Findings

### 1. The loader is named as an emission site for the `theme` component, but doctor and export run the same loader and must emit nothing — no mechanism reconciles them

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §12.3 (the `theme` log component), §3.2 (the token layer moves to `internal/theme`), §8.9 (emission sites), §12.1 (`portal theme export`), §12.2 (`portal doctor`), §5.5 (the construction-time directory read)

**Details**:
Three statements meet and cannot all be satisfied without a mechanism the spec never names.

§3.2 puts emission inside the loader: *"the loader does file I/O and binds the `theme` log component (§12.3)"* and *"One package holds the vocabulary, the parser, the validator, the §6.2 ladder, by-name resolution, enumeration and the embedded set. It binds the `theme` log component."* §8.9 repeats it: *"the `theme` component is emitted from more than one package — the loader (`internal/theme`), the translation (`cmd/config.go`), and this persister."*

§12.3 then forbids emission from two of the loader's four callers: *"`portal doctor` and `portal theme export` both enumerate or parse and both can hit every §6.2 reason — and **neither emits any `theme` event**."*

But doctor performs the same directory scan and per-file validation the panel does, and export performs the same by-name resolve-and-parse construction does. If `theme: rejected` / `theme: directory unusable` / `theme: loaded` are emitted *inside* the loader, doctor emits a full WARN set on every run — the exact outcome §12.3 rules out, and the outcome it says makes the dedup determinate (*"the emitting processes are TUI launches, and nothing else"*). If instead emission lives at the *call sites*, then §3.2's and §8.9's "the loader binds the component" is wrong, and the four events with no other named owner (`loaded`, `enumerated`, `rejected`, `directory unusable`, `fallback applied`) have no stated emission site at all — unlike `theme: commit failed`, whose site §8.9 pins explicitly, and `theme: appearance migrated`, whose site §10.5 pins explicitly.

An implementer must therefore invent one of: a logger/`log.OrDiscard` injected into the loader, a quiet-mode flag, a parse-only entry point beside a logging one, or a return-results-and-emit-at-the-caller split. The choice is not cosmetic — it decides whether `internal/theme` imports `internal/log` at all (which §3.2 gives as a *reason* for relocating the package), and getting it wrong produces a silent contract breach: doctor writing WARNs about a state it just printed, which §12.3 calls *"the same shape of side effect"* it went to trouble to avoid.

The same gap swallows a second question. Three events are *"deduplicated per process"*, and §5.5 depends on the dedup spanning two different call paths: *"emitted both by the panel's enumeration and by the construction-time by-name read (§8.4) when it hits the same condition, deduplicated per process (§12.3) so the two never double up."* Nothing says where that per-process dedup state lives. If emission is caller-side it must be shared across the TUI construction path and the panel path — i.e. a process-wide set someone owns; if it is loader-side it is package-level mutable state in a leaf that §3.4 elsewhere goes out of its way to avoid, and one that tests will need to reset. Either way it is a named requirement with no owner.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. §2.5's role table is the public contract, but the panel's token assignments add consumers it does not list — and one assignment contradicts the role it is given

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Important
**Affects**: §2.5 (role meanings — the public contract), §9.1 (every panel surface's token), §9.5 (invalid rows), §12.4 (`docs/theming.md`), §13.5 (the contrast bands and the doc guard)

**Details**:
§2.5 is not descriptive prose — §12.4 makes it the substance of the document that *is* the contract: *"The 19-token vocabulary with each role's meaning — the substance of §2.5. `docs/theming.md` is **the source of truth for the public contract**."* A theme author picks each value by reading these role descriptions.

§9.1 then introduces a whole new surface with eleven token assignments, and §2.5's role lists are not updated for any of them: `accent.mode` gains the panel header, `accent.primary` gains the `●` assignment badge, `accent.attention` gains the invalid-row reason text and the pinned directory row, `text.primary` gains the valid row label, `text.secondary` gains the confirm message, `border` gains the panel's left border and header rule, and `text.muted` / `accent.key` gain the vertical footer. §13.5's doc guard cannot catch the drift — it *"parses the doc's token table and compares the name set against `Theme.All()`"*, i.e. names only, never roles. This is the same drift class §13.5 justifies the guard with (*"this feature found the MV spec's '2-tone border' claim stale against the implementation purely by chance"*), reintroduced by this feature's own new surface.

One of the assignments is not merely unlisted but contradictory. §2.5 defines the token as:

> | `text.faint` | Decorative only — inactive dots, `+ add`, mode indicator, hints |

and §13.5 floors it accordingly:

> | `text.faint` | band **> 1.00 and < 3.00** — visible but decorative-only; reaching the UI floor is a failure |

§9.1 and §9.5 then assign that token to the invalid row's **label** — the filename or slug the user must read to know *which* of their theme files is broken. §9.4's entire justification for the row is that the user *"sees 'there's my theme, it's registered, but it's invalid' rather than being completely in the dark"*, which requires reading the label. So a bundled theme is *required* to hold that token below the UI floor, and the row is guaranteed to render identifying content at sub-3.00 contrast.

The spec is aware of half of this — §9.1 keeps the `⚠` on its own token *"so the invalidity signal stays legible on a row that is deliberately dimmed"* — but the signal being legible does not make the row identifiable. An implementer following §2.5 and a theme author reading `docs/theming.md` will both act on "decorative only", and neither has any way to know the token now carries the one string that makes a diagnostic row useful. Either the role text needs to cover de-emphasised-but-readable row labels (which changes what a theme author is being asked for), or the label wants a different token; the spec currently asserts both positions in different sections.

**Current**:

§2.5:

> | `text.faint` | Decorative only — inactive dots, `+ add`, mode indicator, hints |

> | `accent.primary` | Cursor, selector bar, active dot, `?` key, focused field label, mode bar, loading bar |
> | `accent.key` | Footer / modal key-hint glyphs |
> | `accent.mode` | Sessions header, Preview chrome, active tick — signals a distinct mode |
> | `accent.attention` | Filter query and `/`, edit-mode, warning flash `⚠` |

§9.1:

> | Valid row label | `text.primary` |
> | Invalid row label | `text.faint` |
> | Invalid row `⚠` and its terse reason | `accent.attention` — §2.5 assigns the warning glyph to it, and the reason is part of the same signal. The `⚠` keeps its own token rather than inheriting the row's `text.faint`, so the invalidity signal stays legible on a row that is deliberately dimmed |

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 3. The pinned `⚠ dir unreadable` row is specified as both "first in the list" and "outside the ordering", and its relationship to scrolling and to the minimum-height floor is undefined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §9.5 (row rendering — the pinned directory row), §9.8 (geometry — overflow and minimum height), §9.4 (the union), §13.3 (the mandated pinned-row and minimum-height fixtures), §12.3 (`theme: enumerated`'s `count`)

**Details**:
§9.5 describes the row two ways without resolving which structure it has:

> **An unreadable themes directory gets its own row** (§5.5) — a non-selectable `⚠ dir unreadable` pinned at the top of the list, above the themes.

> The pinned `⚠ dir unreadable` row is **outside the ordering** — it is always first.

"A row … at the top of the list" and "outside the ordering" support two different implementations with materially different behaviour, and the panel is a scrolling, paginating list (§9.8: *"Overflow: scroll, through the `bubbles/list` machinery, so `Ctrl+↑/↓` paging applies"*):

- **A real list row** (the `HeaderItem` precedent on the Sessions list, which §9.5's arrow-skip and §9.7's key handling both borrow from) — it participates in pagination, so it is on page 1 only and **disappears the moment the user pages down**. The warning that §9.5 says *"is what stands between the user and the 'completely in the dark' state"* is then absent from every page but the first.
- **Chrome pinned to the viewport**, above the scrolling region — it is always visible, but it permanently costs a row of vertical budget, and §9.8's floor does not count it:

> refuse with a flash only when **header + footer + one row + one message row** cannot fit

Under that arithmetic, at the minimum height with an unreadable directory the single list row is consumed by the warning and **no theme row renders at all** — the panel refuses nothing and shows nothing selectable, while §9.5 simultaneously requires *"Built-in rows and persisted-slug rows still render beneath it — the persisted rows especially, or a user with an unreadable directory loses the `●` entirely."*

The spec is otherwise unusually precise about this row (its 16-column copy is pinned in §14A specifically *"so it fits the panel's minimum width without truncation"*, §9.5 exempts it from the four-element composition priority, and §13.3 mandates a fixture for it plus a separate minimum-height-with-a-message fixture). The one property left open is the one the fixtures will expose: whether the frame at the floor shows a theme row, and whether the warning survives paging. A secondary consequence rides on the same decision — §12.3 defines `theme: enumerated`'s `count` as *"rows produced — the full §9.4 union, built-ins included"*, and whether the directory row is a "row produced" changes the number by one on exactly the install the event is most useful for.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 4. Doctor's unresolvable-persisted line has no form for a slug named by both slots, and the one-line rule forbids emitting two

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §14A (doctor copy), §12.2 (one slug produces one advisory line), §9.5 (`● both`), §12.3 (`theme: fallback applied` dedup)

**Details**:
§9.5 treats both slots naming one slug as a first-class, likely state — *"This is reachable in two keypresses (`d` then `l` on one row) and is a likely path"* — and pins a dedicated badge for it (`● both`). §14A's doctor line has no counterpart:

> | Persisted theme unresolvable | `⚠ theme <slug> (<slot>) does not resolve: <reason>`. `<slot>` renders `light` or `dark` under an adaptive pair; under a **constant** the parenthetical is omitted entirely |

If a user sets `theme_light` and `theme_dark` to the same slug and the file is later deleted, `<slot>` has no defined value: it is neither `light` nor `dark` alone, and the constant carve-out does not apply. §12.2 rules out the obvious escape of printing two lines — *"**One slug produces one advisory line**, mirroring §9.4's 'one slug is one row, always' — the two surfaces render the same union and must not disagree about how many problems exist"* — and the panel's own answer for that state is a single row reading `● both`, so two doctor lines would also break the surfaces-agree property the rule exists for.

The same state affects `<M>`: two lines would count two advisories for one problem, which §12.2 explicitly says the count must not do (*"`<M>` counts lines, so it counts problems rather than detections"*).

Worth noting the adjacent behaviour is already determined and asymmetric: §12.3 deduplicates `theme: fallback applied` on `slug`+`reason`, so the log emits **one** line for the two failed slots regardless. Doctor is the only surface with no stated answer.

**Current**:

§14A:

> | Persisted theme unresolvable | `⚠ theme <slug> (<slot>) does not resolve: <reason>`. `<slot>` renders `light` or `dark` under an adaptive pair; under a **constant** the parenthetical is omitted entirely — `⚠ theme <slug> does not resolve: <reason>` |

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 5. While the slot-from-constant confirm is live the panel's pinned footer advertises four keys that are all swallowed, and the confirm's keys have no descriptor home

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.2 (the confirm), §9.12 (the panel's keymap is descriptor-governed), §14A (pinned panel footer copy), §13.3 (the mandated confirm fixture)

**Details**:
§9.2 makes the confirm key-exclusive inside a surface that is already key-exclusive: *"While the confirm is live it is **key-exclusive within the panel** and resolves on exactly three inputs … **Every other key is swallowed** — arrows, `Enter`, the other slot key, all of it."*

§14A pins the panel footer unconditionally:

> **Panel — header and footer (§9.1):** header `Themes`; footer `⏎ set theme` / `d set as dark` / `l set as light` / `esc close`.

So in the confirm state the footer advertises four keys of which **none** does what it says — `⏎`, `d` and `l` are swallowed, and `esc` cancels the confirm rather than closing the panel. Nothing states whether the footer changes while the confirm is live, and §13.3 mandates a fixture for exactly this frame (*"The message slot in both states — the confirm and the failed-commit line"*), so the implementer must decide what that frame renders with no rule to follow. The spec is elsewhere firm that a surface must not advertise a key that will not act (§14.3: *"Advertising a key in the footer that only produces a blocked flash is the dead-end the proactive block exists to prevent"*), which makes the silence here conspicuous rather than safe.

The confirm's own keys are the other half. §9.12 enumerates the panel scope as closed and complete — *"**all six**: `↑`/`↓`, `Ctrl+↑`/`Ctrl+↓`, `Enter`, `d`, `l`, `Esc`. The descriptor must be complete or the dispatch guard's descriptor↔dispatch parity is what breaks"* — while §9.2 binds `y`/`Y`/`n`/`N` inside the panel. Portal's modal confirms sit outside the descriptor entirely (they are key-exclusive modals, not descriptor-governed pages), so there is a defensible precedent for excluding them; but §9.12 declares the panel scope complete at six and §13.6 extends `keymap_dispatch_guard_test` to cover it, so an implementer adding a probe for `y` would break a guard the spec says must stay green, and one omitting it leaves four bound keys in a descriptor-governed surface undeclared. One sentence settles both halves.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 6. The migration's theme-key write and its marker write are not required to be one write, and split writes break the event contract §12.3 relies on

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §10.5 (compute versus persist), §8.9 (the `prefs` save methods and the atomic-write claim), §8.1 (the `theme_migrated` contract), §12.3 (`theme: appearance migrated`)

**Details**:
The translation persists two things at once: the constant (`theme`, with §8.2's mutual exclusion clearing both slots) and the `theme_migrated` marker. §8.9 fixes the `prefs` API as three field-specific methods — *"behind field-specific save methods — `SaveTheme`, `SaveThemeSlot`, `SaveMigrationMarker`"* — none of which writes both, and each of which performs its own read-modify-write. So an implementer either issues two RMW writes or invents a fourth combined method; nothing says which.

§8.9's atomicity sentence does not cover it, because it is scoped to the theme keys of a single commit: *"`prefs.json` continues to go through `fileutil.AtomicWrite`, so all three theme keys land in one atomic write and partial failure is impossible."* Two sequential `Save*` calls are two atomic writes with a window between them, and the window is reachable — §10.5 makes the write *"best-effort and non-blocking"*, i.e. explicitly liable to be cut short.

The consequence lands on a contract the spec states as load-bearing. §12.3 pins the event to the persist precisely so its absence is diagnostic: *"Tied to the persist, it fires exactly once — and its absence after a translation is itself the signal that the write failed."* With split writes, a failure between them leaves the theme key persisted and the marker unset; the next launch finds the marker false, evaluates §10.3's no-op condition against the re-read, sees a theme key already set, *"writes no theme key — it only sets the marker"*, and therefore — per §10.5's *"`theme: appearance migrated` fires only when a theme key is actually persisted"* — never emits the event at all. The translation succeeded and the log says it failed, which is the one reading §12.3 designs the event to make impossible.

The end state is otherwise correct and §10.5's retry argument still holds, so this is a small hole — but it is a decision the implementer must take blind, and the cheapest fix (state that the theme key and the marker land in one write) is one clause.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 7. §2 now describes four naming kinds and four accepted ambiguities while both introductions still say three

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §2.3 (naming principle), §2.6 (accepted ambiguities)

**Details**:
The pairing-name material added to §2.3 leaves both of §2's counted lists stating a number that no longer matches their contents.

§2.3 opens *"Three naming failures are in play; two are failures"* — self-contradictory as written (three failures of which two are failures), and stale besides: the section goes on to name a **fourth** kind (*"A fourth kind is deliberately kept: a *pairing* name"*), so the table's three rows are three naming **kinds**, of which two are failures and one is right, plus a fourth kept kind below.

§2.6 opens *"Three spots were flagged as genuinely arguable and resolved to the values above"* and then lists **four** numbered items, the fourth being *"The `text.on-*` pairing names"*.

Both are editorial rather than behavioural, but §2 is the section a theme author's public contract is derived from (§12.4), the vocabulary's counts are used as load-bearing figures elsewhere in this spec (19 tokens, four eyeball-pinned tints, six erratum values plus a seventh), and a numbered list whose stated count disagrees with its own contents invites a reader to assume an item was dropped.

**Current**:

§2.3:

> Three naming failures are in play; two are failures:

§2.6:

> Three spots were flagged as genuinely arguable and resolved to the values above:

**Proposed Addition**:

**Resolution**: Pending
**Notes**:
