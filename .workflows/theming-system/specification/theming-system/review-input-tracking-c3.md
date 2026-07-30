# Review Tracking: Theming System - Input Review

Cycle 3. Full fresh pass over the whole specification against the whole discussion source.

## Findings

### 1. §14A claims to pin every new user-facing string, but two promised messages are absent

**Source**: `discussion/theming-system.md` — "Amendment — slug rules, and the `name` field is dropped" (line 236-239: the doctor detail is given as *"theme file `Nord.theme`: slug must be lowercase letters, digits and hyphens"*), "Namespace collision — no shadowing" (line 289-291: *"a colliding user file is rejected **with a message naming the conflict**"*), and "No runtime floor under the fallback" (line 1280-1284: *"a binary somehow shipped with a broken default fails **loudly at startup**"*).
**Category**: Enhancement to existing topic
**Affects**: §14A (User-facing copy); cross-refs §9.5, §6.2, §7.6

**Details**:

§14A opens with a completeness claim — *"Every new user-facing string is pinned here"* — and closes with *"Copy that is **not** pinned here is unchanged from what already ships."* Two new strings the specification itself promises elsewhere are neither pinned nor pre-existing:

1. **The doctor line for a file with no slug.** §14A's invalid-theme form is `⚠ theme <slug>: <reason> — <detail>`, but §9.5 establishes that a **`bad name`** file *has no slug* (§5.2 rejects rather than normalises) and is labelled by its filename in the panel. The doctor line has the same problem and no stated form, so `<slug>` is either empty or an invented filename substitution at implementation. The source gives the shape directly — it names the file, not a slug (*"theme file `Nord.theme`: …"*) — and that is exactly the class of file for which "which file is it?" is the whole diagnostic value.

2. **The `reserved name` conflict sentence.** §9.5 states outright that *"doctor carries the sentence naming the conflict"* — a promised string with no pinned copy. It cannot use the generic `<slug>: <reason> — <detail>` form usefully either, since the point is to say *which* built-in the file collides with and that the fix is a rename (§5.4's workaround, which §5.4 also calls "self-documenting" — it is only self-documenting if the message says it).

Related, same class: §7.6's build-time-guarantee escape hatch says *"the user sees a one-line message rather than a Go panic trace"* when a fallback slug cannot resolve. That one-line message is new user-facing copy and is not pinned. It is a should-never-happen path, so terse is fine — but "not pinned" and "should never happen" are different statements, and §14A currently makes the first one for it silently.

**Current**:

> **`portal doctor` (§12.2)** — one advisory line per finding, `⚠`-marked, detail after a colon:
>
> | Case | Copy |
> |---|---|
> | Invalid theme file | `⚠ theme <slug>: <reason> — <detail>` where detail enumerates within the reason (e.g. `missing text.primary, bg.subtle`) |
> | Persisted theme unresolvable | `⚠ theme <slug> (<slot>) does not resolve: <reason>` |
> | Themes directory unusable | `⚠ themes directory unreadable: <path>` |
> | Closing summary | `<N> checks passed · <M> advisories` |
>
> Copy that is **not** pinned here is unchanged from what already ships.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §14A extended with three pinned strings: the no-slug doctor line (names the file, taking the source's own shape), the reserved-name conflict sentence (names the conflict *and* the rename fix, which is what makes §5.4's workaround self-documenting), and the fatal startup message for the unreachable broken-fallback path.

---

### 2. Both slots nominating the same slug puts two badges on one row, which §9.5's marker rules do not admit

**Source**: `discussion/theming-system.md` — "Partial pairs — an unset slot holds the shipped default" (line 1116-1118: *"the panel … can show both slots' current values at all times, including ones never touched"*) and "Paper spike — two marker/slot treatments" / "The constant state — third frame" (the badge treatment, lines 1601-1654)
**Category**: Gap/Ambiguity
**Affects**: §9.5 (Row rendering and markers), §9.2 (Opening state)

**Details**:

`theme_light` and `theme_dark` may name the **same slug** — reachable in two keypresses (`d` then `l` on one row), and reachable by hand-edit. It is also the *shipped-adjacent* state the moment a user pins one slot to the value the other already holds.

§9.5's marker rules enumerate three badge forms — `● dark`, `● light`, bare `●` — and state only that *"the two setting states never coexist on screen, so a row never carries both forms"*, which resolves constant-vs-slot, not slot-vs-slot. So the one row that is both slots has no defined badge. §9.2's opening state has the matching hole: it says *"the other slot's row still carries its `● light`/`● dark` badge"*, presuming two rows.

This is not free to resolve by implementation, because §9.5 pins row composition as a fixed priority order competing for 24–30 columns and ranks the badge **above** the terse reason. A combined `● dark light` is materially wider than either badge alone, so whatever form is chosen changes the truncation budget the section already fixes. The alternatives are visibly different products: one badge listing both slots, two glyphs, or a distinct "both" form.

Worth noting it is the state a user lands in when they want "this theme everywhere" without realising `Enter` (a constant) is the idiom for it — so it is a likely user path, not a contrived one, and §9.9's no-unset acceptance means they cannot back out of it inside the panel either.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §9.5: both slots on one slug renders `● both`. Chosen over a combined `● dark light` because with exactly two slots "both" is fully determined and it is no wider than `● light`, so the §9.5 truncation budget is unmoved. §9.2's opening state updated to put the cursor on that single row.

---

### 3. Case-insensitive extension matching can mint two rows with an identical slug, contradicting §5.1's structural-uniqueness claim

**Source**: `discussion/theming-system.md` — "Theme identity — filename is the slug" (line 200-204: *"The persisted value is **structurally unique** by virtue of being a filename in a directory"*) and "Discovery — lazy, not startup-scanned" (line 2644-2647: *"Enumeration is top-level only — files matching the theme extension"*)
**Category**: Gap/Ambiguity
**Affects**: §5.1 (The filename is the identity), §5.6 (Enumeration rules), §9.5 (ordering and markers)

**Details**:

§5.1 rests the whole no-display-label decision on identity being *"structurally unique by virtue of being a filename in a directory"* — the property that lets the slug be the persisted key with no collision rule for user files (only §5.4's built-in reservation exists).

§5.6 then matches the extension **case-insensitively**, deliberately, so a file is never invisible. On a case-sensitive filesystem `foo.theme` and `foo.THEME` both enumerate and both derive the slug `foo`: two selectable rows with the same label, sorting adjacently, and a persisted `"theme": "foo"` that names both. Nothing in §5.2, §5.4 or §5.6 rejects either file — the slug is legal, and neither collides with a built-in.

The uniqueness claim only holds if the slug is derived by a case-**sensitive** filename-minus-extension rule on a case-**insensitive** filesystem. The spec's rules produce neither guarantee: extension case is normalised away, and the filesystem's case behaviour is not Portal's to assume (macOS APFS can be created case-sensitive, and the loader is plain Go).

Cheap to close in any of three directions — reject a duplicate slug as a new reason, define first-wins by byte-wise filename order, or restrict the case-insensitive match to *reporting* rather than acceptance — but all three are visible behaviour, and the by-name construction path (§8.4) needs the same answer since it resolves a slug to a file without enumerating.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §5.6 by splitting enumeration from acceptance: any extension casing is *enumerated* (so the file is never invisible) but only exact lowercase `.theme` is *accepted*; other casings are rejected `bad name`. A non-exact extension therefore never contributes a slug, so a duplicate slug cannot be minted — no new reason, precedence rule or tie-break needed, and the by-name construction path inherits the guarantee since it looks for `<slug>.theme` and nothing else.

---

### 4. §4.2's empty-value rule forecloses the exact mechanism §4.1's forward note was written to preserve

**Source**: `discussion/theming-system.md` — "File format — flat `key = value`" (line 339-341: *"the deferred transparent-theme idea would need a distinguished value meaning 'use the terminal default' — **btop's precedent is an empty value**. The format should leave that door open rather than close it."*)
**Category**: Enhancement to existing topic
**Affects**: §4.1 (Forward note), §4.2 (branch table), cross-ref §1.4 (Deferred by decision)

**Details**:

The source's forward note names a concrete mechanism for the deferred transparent theme — an **empty value**, per btop — and asks the format to keep that door open. The specification carries the note but drops the named mechanism, and then §4.2's branch table assigns `text.primary =` (empty value) → **`bad colour`**, closing precisely that door.

The door is not closed for transparency as such: a future distinguished value can be a keyword (`transparent`, `none`) admitted by widening §4.3's value domain, which is a loader change and additive exactly as §1.4 claims. But as written, three statements sit unreconciled — the forward note says keep the door open, §4.2 closes the only form the source named, and §1.4 asserts adding transparency later stays "purely additive" without saying by what route.

One line recording the surviving route (a distinguished **keyword**, not an empty value, because §4.2 pins empty as `bad colour`) makes the forward note honest and stops a future reader either re-deriving it or reading §4.2's branch as an oversight to be reversed. It also matters for the branch table's own framing, which presents each row as *"a user-visible reason label and a test case in the loader test"* — the empty-value row is the one branch that a later feature would have to re-open.

**Current**:

> §4.1: **Forward note (not a requirement):** the deferred transparent-theme idea would need a distinguished value meaning "use the terminal default". The format should leave that door open rather than close it.
>
> §4.2: | `text.primary =` (empty value) | `bad colour` | The line *is* a well-formed pair; the value simply is not `#RRGGBB`. |

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §4.1: the surviving route for a deferred transparent theme is a distinguished *keyword* admitted by widening §4.3's value domain (purely additive, as §1.4 claims). btop's empty-value precedent is explicitly closed, since §4.2 pins an empty value as `bad colour` — recorded so the branch is not later read as an oversight.

---
