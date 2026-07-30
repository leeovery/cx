# Review Tracking: Theming System - Gap Analysis

## Findings

### 1. The mandated RMW abort rule makes the first-ever prefs write impossible — `prefs.json` absent is not discriminated from unreadable

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Critical
**Affects**: §8.9 (concurrent instances and prefs writes), §8.1 (marker rules), §9.2 (`Enter` commits a constant), §9.5 (virgin install badges), §13.6 (prefs + migration test)

**Details**:

§8.9 states the write-path rule absolutely: *"A re-read that does not yield a usable file aborts the write — it never becomes an overwrite… a decode failure **or** an I/O failure on the re-read is treated as a failed write: nothing is written, `theme: commit failed` is emitted, and the panel reports it."*

The specification also establishes, repeatedly and deliberately, that **`prefs.json` does not exist on a fresh install**:

- §8.1: the marker is *"Not written when `prefs.json` does not exist"*, and empty values are omitted on write — *"a key the user has never set is absent"*.
- §9.5: *"on a brand-new install, where §8.1 leaves `prefs.json` absent entirely"* — and specifies exactly what the panel shows there (both shipped-default slot badges).

So the most common first commit in the product — a brand-new user opens the panel and presses `Enter` — performs an RMW whose re-read hits ENOENT. Read literally, that is an I/O failure, the write aborts, `theme: commit failed` is emitted and §9.13's outstanding-failure machinery fires, **permanently**: nothing else creates `prefs.json` either, because §8.9's *"Every writer must read-modify-write"* covers the `s`-key mode persister too (§8.9 names it explicitly as one of the three surfaces), and the migration write is barred from creating the file by §8.1. Under the literal rule `prefs.json` can never come into existence.

The two conditions are genuinely different and the spec's own reasoning distinguishes them everywhere else (§5.5's absent-vs-unreadable directory, §12.1's `not found`-vs-`unreadable` export): **absent** means "there is nothing to merge, proceed and create", while **corrupt or unreadable** means "there are bytes we cannot merge into, abort". §8.9's rationale is written entirely about the corrupt case (*"a stray comma degrades to a zero-value struct"*, *"merging into that and committing it would erase `session_list_mode`, `theme_migrated`…"*) — it never contemplates the absent case, which has nothing to erase.

An implementer must invent the discrimination, and the two readings differ by whether the shipped-default install can save a theme at all. §13.6's prefs test is specified around merge and round-trip, not around file creation, so the suite as specified would not catch the literal reading either.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Real defect, fixed. §8.9 now discriminates absent from unusable: an absent prefs.json means nothing to merge and nothing to lose, so the write proceeds and creates the file (the ordinary first write — a fresh install has no prefs file, and an abort there would be permanent since the s-key persister is under the same rule and §8.1 bars the migration from creating it); a present-but-undecodable or unreadable file aborts. Same discrimination §5.5 and §12.1 already draw. §13.6's prefs test extended to cover file creation.

---

### 2. The translation's no-op condition governs the write only — whether the computed constant is still applied *in memory* is unspecified, and the literal reading flips the user's theme for a launch

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §10.5 (separate computing from persisting), §10.3 (trigger vs no-op condition), §8.9 (no-op condition evaluated at the RMW re-read)

**Details**:

§10.5 splits the translation into two halves and pins the in-memory half unconditionally: *"At prefs load, read `appearance`, compute the translated theme, and **use it in memory immediately**; the write is best-effort and non-blocking. A failed write means Portal renders the correct theme this launch and retries next launch… so **it can never flip the user to the wrong theme**."*

§10.3 then adds the no-op condition — but scoped explicitly and only to the write: *"If any theme key is already set, **the translation writes no theme key** — it only sets the marker… it is refusing to clobber a choice the user has already made."* §8.9 reinforces the write-scoping: the no-op condition is *"evaluated at the RMW re-read, against the bytes about to be merged"* — a write-path moment that happens after the in-memory value has already been computed and used.

Nothing says the in-memory application is skipped. On §10.3's own reachable sequence — user upgrades, migration never fires, hand-edits `theme_dark = nord`, then launches the picker — the literal composition is: compute `theme = tokyo-night` from the retained `appearance: dark`, **apply it in memory** (so this launch renders Tokyo Night, not the Nord the user just authored), skip the theme-key write, set the marker. The next launch renders Nord. That is a one-launch silent flip of exactly the kind §10.5 claims cannot happen and §10.1 exists to prevent, delivered by the mechanism added to prevent it.

Two further consequences ride on the same ambiguity:

- If the in-memory result *is* skipped, the check must run against the **load-time snapshot** (the write-path check runs at the RMW re-read, far too late to affect what was painted) — so the condition is evaluated at two different points against two different reads, which the spec does not say.
- §12.3 ties `theme: appearance migrated` to successful persist. Under the skip-the-write case the marker is written but no theme key is; whether that counts as the "successful persist" that fires the event is left open by the same seam.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §10.5: §10.3's no-op condition governs both halves, and for the in-memory half it is evaluated against the load-time snapshot — if any theme key is set the translation neither writes nor applies. Scoping it to the write alone produced a one-launch silent flip on §10.3's own reachable sequence. The deliberate two-point evaluation (load for memory, RMW re-read for the write) is stated, and `theme: appearance migrated` pinned to fire only when a theme key is actually persisted.

---

### 3. The mandated panel fixtures need persisted-key state, but the only specified fixture input is a nomination that `capturetool` always passes in the constant shape

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §13.3 (harness changes, new panel fixtures), §8.4 (the constructor takes the raw persisted theme keys), §9.5 (badge derivation), §8.2 (`theme` wins ⇒ no slot badges)

**Details**:

§13.3 pins two things that do not compose:

1. *"`tui.Build` takes the loaded **nomination** where it takes a `prefs.Appearance` today"*, and *"`capturetool` always passes the **constant shape**: a single pinned theme, no gate, no wait — which is what keeps captures byte-deterministic."*
2. The required new fixtures include *"**The adaptive-pair state** (two slot badges)"* and *"**The constant-while-previewing state** (a bare `●` while the cursor sits elsewhere)"*.

But badges do not derive from the nomination. §8.4 makes that explicit — the constructor *also* takes the raw persisted keys, because *"a badge needs the **persisted** slug, not the nomination's"* — and §8.2 makes the two badge states mutually exclusive **from the raw keys**: a non-empty `theme` means a bare `●` and *"no slot badges"*. So a fixture built from a constant-shape nomination alone can only ever render the bare-`●` state, and the mandated adaptive-pair fixture is unreachable.

The specification never says how a fixture supplies the raw persisted-key state, whether `internal/capture` fixtures may set it independently of the nomination `--theme` pins, or what the constant-shape rule means when a fixture's raw keys declare an empty `theme` and two slots (the combination §8.2 classifies as adaptive while the nomination has one member). The same input is required by other mandated panel fixtures — the `not found` / charset-rejected persisted-slug rows of §9.4 exist only because prefs names them, and the fallback-badge case of §9.5 needs a persisted slug the enumeration rejects.

This matters more than a wiring detail because §13.1 makes the harness the **only** route to seeing any of this before release, and §9.14 identifies the slot half of the panel as the part with no prior art anywhere — the badges are precisely what the visual gate exists to judge.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §13.3: a fixture declares its own raw persisted theme keys independently of `--theme`, which pins only the nomination's palette. Without the separation the adaptive-pair fixture is unreachable, since capturetool always passes the constant shape and §8.2 makes a non-empty `theme` render a bare ● with no slot badges. Noted as fixture data rather than config discovery, so §7.1's import guard is untouched.

---

### 4. `theme: loaded` is defined per *nomination*, so after a fallback the log never names the theme actually in force

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §12.3 (event catalogue), §8.5 (fallback), §6.3 (the log is the only passive trail)

**Details**:

§12.3 defines `theme: loaded` as firing *"one line per **nominated** theme… Resolved slug(s) only"*, and `theme: fallback applied` as carrying *"`slug` (the nomination that failed), `slot` where one applies, and `reason`"*.

When a nomination is unloadable, both lines are about the slug that **failed**. Nothing in the catalogue emits the slug of the theme that actually rendered. Whether `theme: loaded` fires a second time for the fallback (and with the fallback's slug, or the nomination's) is undefined, and either choice is defensible: it is not a *nominated* theme, so the cadence column argues against it; but §6.3 states the log's job is to be *"the only record that exists without the user going looking"*, and §12.3 justifies the component on greppability — neither of which holds if a `grep "theme:"` on a broken install cannot answer "which palette am I looking at".

The same question applies to the commit-time load §8.4 adds (the newly-live opposite slot), which §12.3 explicitly routes through `theme: loaded` and which §8.4 explicitly allows to be unresolvable and take the fallback.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §12.3's `theme: loaded` now also fires for the fallback, carrying the fallback's slug — otherwise both it and `theme: fallback applied` name the slug that failed, and a grep on a broken install cannot answer which palette is rendering, which is the greppability the component is justified on.

---

### 5. The message slot may wrap to two rows at minimum width, but the minimum-height floor counts exactly one message row

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.1 (message slot), §9.8 (minimum height), §13.3 (the minimum-height-with-a-message fixture)

**Details**:

§9.1: *"At the minimum panel width the slot may wrap to two rows. It is not a list delegate, so wrapping costs nothing structurally."*

§9.8: refuse *"only when **header + footer + one row + one message row** cannot fit"* — a floor of exactly one message row.

Wrapping costs nothing to `bubbles/list` pagination (which is what §9.1's clause asserts), but it does cost a **row of vertical budget**, and vertical budget is what §9.8's floor is made of. At minimum width *and* minimum height with a confirm live — a reachable state, and one §13.3 mandates a fixture for (*"the panel at its minimum height with a message live"*, precisely because *"that arithmetic is only observable on a frame that renders it"*) — the two-row message leaves the panel with zero list rows or overflows its own frame. Neither outcome is specified, and the confirm cannot be suppressed (§9.8: both contenders are non-suppressible).

An implementer must pick one of: the floor counts two message rows at the minimum width, the message is single-line-truncated rather than wrapped when the panel is at its height floor, the list is allowed to reach zero rows while a message is live, or the copy is constrained so it cannot wrap at the minimum width (§14A already treats panel wording as a layout constraint, and pins `clear constant <slug>?  y / n` — whose length is slug-dependent).

*(Cycle 4's finding 8 raised the collision and was resolved by putting a message row in the floor; the wrap sub-case is the residue that fix did not reach.)*

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §9.1 now states that at the minimum *height* the message is truncated to one line rather than wrapped — wrapping costs a row of vertical budget, and §9.8's floor counts exactly one message row with both contenders non-suppressible. Truncation degrades the message rather than the row the user is being asked about.

---

### 6. The swap-and-diff guard is specified as scanning rendered output for *hex values*, which styled ANSI output does not contain

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §13.4 (the swap-and-diff completeness guard), §4.3 (canonicalisation rationale)

**Details**:

§13.4 defines the guard as *"render every fixture under theme A, switch to theme B, render again, and **scan the second output for any colour value belonging to theme A**"*, and justifies forcing truecolor with *"otherwise lipgloss would strip colour and **there would be no hexes to diff** at all"*. §4.3 lists the guard as one of three sites that *"compare hex strings"*, and cites it as a reason the parser canonicalises hex to uppercase.

Rendered lipgloss/ANSI output carries no hex — a truecolor foreground is `ESC[38;2;R;G;B m`, decimal. So the comparison the guard performs is between a token's parsed value and its *rendered escape-sequence form*, and the mapping from one to the other is unstated. This matters in one direction specifically: assertion 1 is a **negative** (*"no theme-A value survives"*), which passes vacuously if the search representation is wrong. Assertion 2 (every theme-B value present) is the backstop that would fail loudly, so the exposure is bounded — but the guard is the feature's central completeness mechanism and its stated mechanism is not literally implementable as written.

§4.3's canonicalisation rationale is correspondingly inaccurate on this one of its three cited sites (the other two — §11.4's retained canvas hex and §11.3's background diffing — are genuine hex-string comparisons and are unaffected).

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §13.4 now states the comparison is against each token's rendered SGR form, not its hex — styled output carries no hex. Flagged that assertion 1 is a negative, so a wrong search representation passes vacuously and silently, with assertion 2 as a bounded backstop the guard should not rest on. §4.3's canonicalisation rationale corrected from three sites to two.

---

### 7. Doctor's new advisory lines have pinned copy but no defined position in the report

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §12.2 (doctor's two classes of line), §14A (doctor copy), §15.1 (the doctor-contract amendment)

**Details**:

§12.2 amends doctor's contract to carry two classes of line and adds a closing summary; §14A pins every string, including the `⚠`-marked per-finding lines and all three summary forms. What is never stated is **where the advisory lines sit** in a report that is otherwise *"an ordered catalog of read-only health checks… one line per check"* (CLAUDE.md's description of the command this feature amends).

The theme class differs structurally from every existing line: it is 0..N lines rather than one line per check, its cardinality depends on user content, and it does not participate in `<N>`/`<T>`. So an implementer must decide whether the advisories interleave into the ordered catalog (and at which position), form a trailing block before the summary, or are grouped under their own sub-heading — and the choice is visible in every run of a command whose whole output is copy this specification otherwise pins to the character.

A second, smaller instance of the same omission: whether the theme scan runs on the `--fix` path at all (it has no repair, and doctor re-diagnoses after repairs), and therefore whether the advisories and the `· <M> advisories` suffix appear in `--fix` output.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §12.2 now places advisories as a trailing block after the ordered catalog and before the summary (the catalog is fixed-order one-line-per-check; the theme class is 0..N and user-content-dependent, so interleaving would make a fixed report vary in length and position), and states the theme scan runs on `--fix` too — suppressing it would make `--fix` a less informative diagnosis than the plain run.

---
