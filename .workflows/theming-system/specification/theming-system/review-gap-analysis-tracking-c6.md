# Review Tracking: Theming System - Gap Analysis

Cycle 6. Full fresh pass over the whole specification as a standalone document. Prior cycles' findings (gap-analysis c1–c5, input review c1–c6) were read first; nothing already resolved is re-raised.

## Findings

### 1. An OSC 11 reply that lands after the gate has already resolved — flip or ignore — is undefined, and §11.1 and §8.8 describe that path differently

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §8.8 (what survives in the appearance gate), §11.1 (speed is a non-issue), §11.4 (the exit-time canvas restore); cross-ref §9.3

**Details**:

The detect-or-timeout gate is a race: the OSC 11 reply or a ~50ms timeout, first to resolve wins, dark on no answer. The spec never says what happens when the **reply arrives after the timeout has already resolved the gate and the first frame is painted**. Two sections point opposite ways:

- **§8.8** frames the gate as resolving *before* first paint precisely so Portal does not "paint one theme and flip" — which reads as: a late reply is ignored, the selection is single-resolution.
- **§11.1** asserts the restyle path is "already exercised in production: **it is what runs when the OSC 11 reply lands after first paint**" — which reads as: a late reply *does* restyle, post-paint.

Both cannot describe the same mechanism, and each has a consequence the other does not:

- If a late reply flips, the flip is materially louder under this feature than it is today. It no longer swaps a light/dark variant of one palette — it swaps to **a different theme** (the other slot's nomination, potentially Nord vs Tokyo Night Day), a full palette change a second or two after the user is already looking at the picker. Under the old paired model the flip was cosmetic; under split it is the whole canvas *and* every accent.
- If a late reply is ignored, then §11.1's claim that the swap entry point is already production-exercised has no live caller, and the entry point §13.4 requires the guard to drive ("the same entry point the panel's arrow-preview uses") is a path this feature must establish rather than reuse. That changes what the implementer is being asked to build — reuse versus new — on the mechanism the whole live-swap section rests on.

There is a second-order consequence that touches the one path the spec singles out as able to leave a colour stuck in the user's terminal. **§11.4 retains "the startup canvas hex … captured from the theme the gate *selected*"** — singular. If the gate can resolve twice (timeout → dark theme → late reply → light theme), there are two selected themes and two candidate hexes, and the spec does not say which is retained. The echo guard and `RestoreTerminalBackground` both anchor to that value, and §11.3's "the guard only ever needs to compare against the canvas active during the *startup* window" becomes ambiguous about where that window ends.

Also unresolved by the same gap: whether a late reply is allowed to flip **while the panel is open with an uncommitted preview** — where it would overwrite what the user is actively previewing.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. The model never gets specified state for the *raw persisted theme keys*, which the panel's badges and its `not found` / `bad name` rows all require

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §8.4 (construction timing — what the constructor takes), §9.4 (the list — files ∪ whatever prefs names), §9.5 (markers), §14A (the confirm's `<slug>`); cross-ref §8.9, §12.2

**Details**:

§8.4 defines exactly what the constructor is handed: "the loaded *nomination*" — one `Theme` under a constant, two under a pair — "plus which member is currently active". §3.2 adds that `Theme` carries no identity field and that "the slug is held alongside the palette by whatever loaded it — the model for the active theme".

That state is insufficient for the panel, in three ways, and none is closed anywhere:

- **A slug that did not load is not in the nomination at all.** §9.4 requires a row for "any slug named in `prefs.json` that resolves to neither" a built-in nor a file, and §9.4/§8.6 require a row for a persisted string rejected by the charset check. Neither ever produced a loaded `Theme`, so neither can be recovered from the nomination. The panel needs the raw persisted strings.
- **The non-active slot's slug is needed for its badge.** §9.5's closing argument is that "the panel shows **both slots' badges at all times** … including slots never touched, which hold shipped defaults". The nomination holds the other slot's *palette*; the badge needs its *slug*, and under a fallback (§8.5) the badge must sit on the **persisted** slug, not the fallback's — "the `●` still marks the persisted slug" (§9.2). So slug-of-nomination is not the same value as slug-persisted, and both are needed simultaneously.
- **§14A's confirm renders the persisted constant** — `clear constant <slug>?` — on a path where that constant may be the one that failed to load.

Second half of the same gap: **where the panel reads those keys from when it opens**. §5.8 mandates a fresh *directory* read on every open and argues freshness explicitly; nothing says whether the *prefs* side is re-read or is the construction-time snapshot. The spec points both ways: §8.9's "other instances are unaffected until relaunch. There is no file watch" argues snapshot; but §8.1 makes `prefs.json` "the hand-editable home for the theme setting" and §9.9's accepted no-unset rests on the user hand-editing it — so a user who edits prefs mid-session gets their *file* edits picked up on the next panel open and their *prefs* edits ignored, an asymmetry that is defensible but is currently neither stated nor derivable.

`portal doctor` needs the same rule stated once: §12.2 reports "when a persisted theme name no longer resolves", and §14A's line renders `<slug>` and an optional `<slot>` — which under §8.2's `theme`-wins hand-edit means doctor must know whether it reports the winning constant only, or the ignored slots too.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 3. What the panel recomputes after a successful commit — badges, row set, ordering, cursor — is never stated, and §8.2 requires a mid-session row change

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §9.4 (the list), §9.5 (row rendering and markers), §9.2 (commit keys), §8.2 (two states, not three)

**Details**:

Every panel commit mutates persisted state while the panel stays open (§9.2: "`Enter` does not close", `d`/`l` stay open). The badges must therefore move — §9.13 says a *failed* commit "does not move the `●`", which only means anything because a successful one does. But the spec stops at the badge, and a commit can change more than badges:

- **`Enter` clears both slots.** If a `not found` row (or a charset-rejected row) existed *only* because a slot named that slug, its reason for existing is gone. Does the row disappear mid-session, or persist until the panel is reopened?
- **`d`/`l` on a constant clears the constant and makes the *other* slot live.** §8.2 states this outcome and says "the stale slot surfacing is then **plainly visible in the panel's badges** the moment the confirm resolves". But if that stale slot names a slug with no file and no built-in, §9.4 requires it to have a **row** — and the open-time union never minted one, because §8.2 says a `theme`-wins file's slots "are not read at all". So a row has to appear mid-session for the badge to have somewhere to sit.
- **Any row appearing or disappearing re-sorts the list** (§9.5's ordering) and **shifts indices under the cursor**. The cursor is load-bearing here: §9.2's invariant is that "the cursor is always on a selectable row, and that row is always what is painted behind the panel". A recompute that leaves the cursor on an index rather than on the identity of the previewed theme breaks that invariant silently — the screen keeps previewing theme X while the cursor sits on Y.

The union in §9.4 reads as an open-time computation (§5.8 pins enumeration to open, and retains its parses for the panel's lifetime), so an implementer has no basis to decide whether commits re-derive it. All three questions are answered by one rule — state it once, covering what is recomputed (badges only, or the full union + sort) and what the cursor is anchored to across the recompute.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 4. §8.4 requires a fresh by-name read for the opposite slot at commit time, but the panel already holds a parse of that slug — the spec does not say which wins

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §8.4 (mid-session slot assignment), §5.8 (enumeration re-reads on every open); cross-ref §9.4

**Details**:

§8.4 splits the constant → adaptive conversion into two halves: "The slot the user just assigned needs no read — §5.8's enumeration already holds its parse", while "the read that is needed is the **opposite** one", specified as resolving from the embedded set for an untouched slot and as "a **by-name directory read**" for a stale hand-edited slot.

That by-name read is in tension with two rules that are stated as unconditional:

- **§9.4** puts a row in the panel for every `*.theme` file in the directory, so a stale slot naming a real file **already has a parse in hand** — the panel enumerated it on open. The by-name read re-reads a file the panel has already read and classified.
- **§5.8** states, without qualification, that "**the panel's parse supersedes the construction-time parse for the same slug**" because "after a mid-session edit the panel holds the fresher truth". A commit-time by-name read is a *third* parse that is neither the construction one nor the panel's, and it can disagree with the row the user is looking at — the panel row says `bad colour` (or renders it as valid) while the commit path resolves against different bytes.

The implementer has to choose between two mechanisms the spec names for the same value, and the choice is observable: look the slug up in the retained enumeration (consistent with §5.8, no I/O, and the panel row and the applied theme cannot disagree), or issue the read §8.4 specifies (fresher by milliseconds, but reintroduces exactly the staleness split §5.8 exists to close, and duplicates the reason ladder off the panel's own row model).

Narrow in user impact, but it is a genuine fork in the panel's commit handler, and it also decides whether the loader needs a by-name entry point reachable from the TUI at all outside construction.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 5. The panel's ordering is claimed to be total, but a `reserved name` row and the built-in it collides with have identical sort keys by construction

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Important
**Affects**: §9.5 (sort key and display label)

**Details**:

§9.5 resolves sorting by making the sort key the slug wherever one exists — explicitly including a `reserved name` row — and closes with a tie-break rule for case-insensitive equality. But `reserved name` is *defined* as "slug collides with a built-in" (§6.2), so whenever that reason occurs there are two rows whose sort key is **the same string**: the built-in's slug and the rejected file's slug. The stated tie-break ("case-insensitive, with a byte-wise tie-break") does not separate them either — the bytes are identical too.

So the one ordering tie the design guarantees will occur is the one the rule cannot break. §9.5 asserts the sort key "is fully determined" and that using the persisted string "keeps the ordering total"; both claims fail on this pair. §5.6 was careful to make duplicate *slugs* impossible from files precisely so no ordering tie-break would be needed — this pair is the residue that construction does not cover, because the two rows come from different sources (embedded set and directory).

It is not cosmetic: §9.5's own argument for labelling the rejected file by filename is that "it sorts adjacent to the built-in it collides with, which is where the explanation is most useful" — adjacency is stated, but which of the two comes first is left to whatever the sort implementation happens to do, which is also a determinism problem for the panel fixtures and their captures (§13.3).

**Current**:

> §9.5:
> - The **sort key is the slug** wherever one exists — including a `reserved name` row, which is why it sorts adjacent to the built-in it collides with despite being *labelled* by filename. A `not found` persisted-slug row sorts by its slug too.
> - Only a **`bad name`** row has no slug; it sorts by **filename**. A **persisted string rejected by §8.6's charset check** has neither a slug nor a file — it sorts by **the persisted string itself**, control-stripped and truncated as it is for display. There is exactly one thing to sort it by, and using it keeps the ordering total.
> - Comparison is **case-insensitive, with a byte-wise tie-break**. Slugs are lowercase by construction, but filenames are not, and a byte-wise-only comparison would file `Zed.theme` ahead of every valid theme.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 6. Two sections require `portal theme export` to read `prefs.json`; §12.1's command surface gives it no reason or route to

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Important
**Affects**: §12.1 (`portal theme export`), §10.5 (non-migrating read), §9.5 (control-stripping a persisted slug)

**Details**:

§12.1 fixes export's surface completely: exactly one slug argument (`ExactArgs(1)`), resolving built-ins and drop-ins, output is the file's bytes. Nothing in that surface reads the theme setting — a slug argument resolves by name without consulting `prefs.json` at all.

But two other sections state that it does:

- **§10.5** justifies the non-migrating prefs read partly on export's behalf: "`portal doctor` must read `prefs.json` … and `portal theme export` **may resolve a persisted slug**". If export never reads prefs, that clause is unfounded and the non-migrating variant is needed for doctor alone.
- **§9.5** makes control-stripping a property of a value read from prefs and then names its consumers: "it reaches **three** surfaces: the panel row, doctor's advisory line (§14A), and **`portal theme export`'s stderr**". For a prefs-sourced slug to reach export's stderr, export must render a value it read from prefs.

Either export reads prefs in some case the command surface does not describe (and §12.1 needs to say when, and what it does with it), or it does not (and both clauses need correcting).

The second half matters independently of which way it resolves: **a slug arriving as a CLI argument is not covered by any sanitisation rule.** §9.5 scopes control-stripping to "a slug that came from `prefs.json` … at the point it is read", yet §14A pins export's stderr copy as `no theme named <slug>` and `theme <slug> is not valid: <reason>` — echoing an argument that can equally carry a pasted newline or ANSI escape. If export never reads prefs, that stripping needs a second home rather than disappearing with the clause.

**Current**:

> §10.5: **`cmd/config.go` also exposes a non-migrating read variant, which every bootstrap-exempt command uses.** `portal doctor` must read `prefs.json` to report an unresolvable theme (§12.2), and `portal theme export` may resolve a persisted slug — but doctor's contract is that it **heals nothing on the read-only path** …

> §9.5: **A slug that came from `prefs.json` is control-stripped at the point it is read, not at the point it is drawn** — it is a property of the value, so every consumer inherits it. §8.6 validates it before *use* as a path component, but a charset-rejected value is still *reported* (§9.4, §12.1), and it reaches three surfaces: the panel row, doctor's advisory line (§14A), and `portal theme export`'s stderr.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 7. `theme: rejected` is deduplicated across panel opens; `theme: fallback applied` and `theme: directory unusable` fire on the same repeating trigger with no rule

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Important
**Affects**: §12.3 (event catalogue — the cadence column); cross-ref §5.8, §9.2

**Details**:

§12.3 identifies the repeat problem explicitly for one event and solves it: `theme: rejected` is "deduplicated per process … so five panel opens (enumeration re-reads on every open, §5.8) do not produce five identical WARN sets". Cycle 6's input review then narrowed the emitting set to TUI launches only — which makes a single long-lived picker process the whole population, and sharpens rather than relieves the repeat question.

Two other WARN events fire on exactly the same repeating trigger and carry no equivalent rule:

- **`theme: directory unusable` — "Per enumeration where the themes directory is unreadable."** Enumeration is per panel open (§5.8). A user with a bad directory gets one WARN per open, identical every time — the same shape §12.3 refused for `rejected`, one row down the same table.
- **`theme: fallback applied` — "Per fallback."** A fallback is now resolved in at least three places per panel visit under a persistently broken active theme: at construction (§8.4/§8.5), on **open** (§9.2 — "opening resolves the §8.5 fallback"), and on **`Esc`** (§5.8 — "`Esc` resolves persisted state against the panel's enumeration … lands on the §8.5 fallback"). Under the plain reading that is three identical WARNs for one broken file per open/close cycle, forever, from a component whose stated job is a *passive* forensic trail rather than a running commentary.

The catalogue is declared closed and spec-governed, and its cadence column is what an implementer builds against, so "per fallback" needs to say per *what* — per resolution event, per process, or per slug+reason like its neighbour. The dedup decision also interacts with §12.3's own dedup-key rule (`slug`+`reason`, or `path`+`reason` where there is no slug), which was defined for `rejected` only.

**Current**:

> §12.3:
> | `theme: directory unusable` | WARN | Per enumeration where the themes directory is unreadable, or a regular file sits where a directory belongs (§5.5). Carries `path` and `reason`. An *absent* directory emits nothing. |
> | `theme: fallback applied` | WARN | Per fallback. Carries `slug` (the nomination that failed), `slot` where one applies, and `reason`. Without them the line is not greppable, which is the whole reason the log earns its place. |

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 8. §9.7's entry conditions are stated exhaustively and omit the geometry refusal that §9.8 and §14A both specify

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §9.7 (entry conditions and input routing); cross-ref §9.8, §14A

**Details**:

§9.7 opens with a closed statement — "**Nothing blocks `t` except** a modal, a pending burst, `NO_COLOR`, and the pages where it is not bound at all" — and then enumerates those four cases as a list, which reads as the complete entry-condition contract.

§9.8 adds a fifth blocker: the panel "**refuses only when even the minimum panel cannot render** … and then it flashes rather than opening a broken frame", on both dimensions. §14A pins two distinct flash strings for it (`terminal too narrow for the theme picker`, `terminal too short for the theme picker`), so the behaviour is fully decided — it is only §9.7's list that is short.

The list is also where the flash-versus-silent rule is stated ("**flash** where the key *is* bound … **silent** where it is not bound at all"), and the geometry case is a *third* shape: bound, available in principle, refused for want of space — which §9.8 resolves as a flash, matching the `NO_COLOR` leg but for the opposite reason (§9.10 draws that distinction deliberately: capability absence versus space shortage). An implementer reading §9.7 alone builds the wrong gate; an implementer reading §9.8 alone misses that this is an entry condition and not only a resize condition.

**Current**:

> §9.7: **Nothing blocks `t` except a modal, a pending burst, `NO_COLOR`, and the pages where it is not bound at all (§9.6 — Preview and Loading).**

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 9. `theme: enumerated`'s `count` and `rejected` are declared as attrs but not defined as quantities

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §12.3 (event catalogue — `theme: enumerated`)

**Details**:

The row declares the attrs but not what they measure, and the panel's row set makes that genuinely ambiguous rather than obvious: §9.4's list is "every `*.theme` file in the themes directory … **plus every built-in**, plus any slug named in `prefs.json` that resolves to neither". So `count` has at least three defensible readings — directory entries considered, rows produced, or valid selectable themes — and they differ by three on every install (a clean directory with one drop-in yields 1, 4, or 4). `rejected` is likewise either "files that failed the §6.2 ladder" or "unselectable rows", which differ whenever a persisted slug contributes a `not found` or `bad name` row from no file at all.

This is a small definition, but the component is declared closed and spec-governed, the attr vocabulary is fixed, and the value's whole purpose is to be greppable after the fact — a count whose denominator is unknown is not.

Two adjacent one-liners the same row leaves open: whether `theme: enumerated` fires at all when the themes directory is **absent** (the deliberately silent case of §5.5 — silence is specified for doctor and the log's *directory* event, but this event is per panel open, which still happened), and whether it fires when the directory is **unusable** (where `theme: directory unusable` is specified but the enumeration produced nothing to count).

**Current**:

> §12.3: | `theme: enumerated` | INFO | At panel open. Carries `count` and `rejected`. |

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---
