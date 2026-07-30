# Specification: Theming System

## Specification

## 1. Overview & Scope

### 1.1 Purpose

Portal's TUI colour layer is already tokenised: `internal/tui/theme` declares a closed set of named semantic role tokens, every renderer references a token rather than a raw hex, and `theme.MV` is the single built-in palette compiled into the binary. Layout is fixed; only colour is parameterised.

This feature changes **where token values come from** and adds **a surface to switch them**:

1. **Token values load from theme files, not Go code.** A theme is a flat `key = value` file. Built-ins are the same files, embedded via `go:embed` and parsed by the same loader as a user's.
2. **Portal ships three built-in themes** — Tokyo Night Dark, Tokyo Night Light, Nord — so it launches with genuine options rather than a single palette.
3. **An in-app theme selector** — a non-blanking full-height slide-over on the right edge, opened with `t`, where arrowing re-themes the app live behind it.
4. **The token vocabulary is renamed and consolidated** — 20 → 19 tokens, hue names and use-site names replaced by weight-and-meaning names — before those names become the public contract a theme author writes against.

### 1.2 The shape of the change

- **A theme is one palette**, not a light/dark pair. `Token` collapses from `{Name, Light, Dark}` to `{Name, Value}`; `Token.ColorFor` and `theme.Mode` are removed, and the `mode` parameter stops travelling through the render layer.
- **The model holds the active `Theme`** and threads it where `mode` is threaded today. No package-level mutable state.
- **Light/dark is expressed by the shape of the theme setting**, not by anything inside a theme. `prefs.appearance` is removed and replaced by `theme` / `theme_light` / `theme_dark`.
- **Detection ships**, and follows the **terminal background** via OSC 11 — not the OS colour scheme.
- **The exec path (`portal open <target>`) does zero theme work** — no scan, no file read, no parse. It constructs no TUI.

### 1.3 Audience and contribution routes

Portal is realistically a single-user tool. Two contribution routes exist regardless:

1. **PR route** — anyone may open a pull request adding a theme file; if accepted it ships embedded as a built-in.
2. **Drop-in route** — a theme file placed in the user's themes directory is auto-discovered with no registration step. If valid it appears in the selector alongside the built-ins.

This is the ecosystem's standard two-tier shape (library directory + selection setting) without committing to ecosystem-scale governance. It sets the two quality tiers (§7) and makes the token names a public contract worth settling now: renaming a token is a mechanical repo-wide change for built-ins, but it *breaks* files in a user's themes directory.

### 1.4 Deferred by decision

Not omissions — each was considered and deliberately excluded from this feature:

- **Transparent themes** (a distinguished value meaning "use the terminal default"). The file format leaves the door open; following the terminal background rather than the OS scheme keeps adding it purely additive later.
- **Panel search / filtering** of the theme list.
- **A second light theme** beyond Tokyo Night Light — follow-up work, and a design task rather than a file drop (§7.5).
- **Merge over a base** (partial files declaring a parent). Full-replacement files remain valid under any later merge model, so this stays available.
- **A `--theme` flag** and **`portal theme list`** — the panel lists themes and doctor validates them.
- **A one-shot detection seed** for a virgin install — the shipped adaptive default already covers the case.
- **Per-theme licence lines, "(adapted)" naming conventions, and any PR contribution ceremony.** Attribution is a source and a link in the docs, nothing further.
- **`fsnotify` directory watching** — the panel re-reads the directory on every open instead.
- **A general settings panel** that would also swallow the `s` grouping-mode cycle. Two mechanisms for two prefs is an accepted mild inconsistency.
- **A runtime last-resort hardcoded palette** beneath the fallback — replaced by a build-time guarantee (§7.6).
- **A panel key that unsets a theme back to the shipped default** (§9.9).
- **A theme "variant" (light/dark) concept** anywhere in the product — neither declared in a file nor derived at load (§4.6).

## 2. Token vocabulary — the 19 roles

### 2.1 The vocabulary is 19 tokens

The closed vocabulary goes from 20 tokens to **19**. Every renderer references a token; no raw hex survives at a call site (the existing glob-based colour-literal guard in `internal/tui` continues to enforce this, excluding the `theme` subpackage).

### 2.2 The border tokens consolidate to one

`border.separator` and `border.footer` are **one role**, not two:

- `renderJoinedPanel` already takes a single border token — the 2-tone footer leg was dropped during the Modern Vivid implementation, making the MV spec's §8.1 "2-tone border (`border.separator` + `border.footer`)" claim stale.
- `border.separator` serves the title rule, every modal panel frame (destructive-confirm, edit, help, rename), and edit-modal chips. `border.footer` has exactly **one** production consumer: the footer rule.
- The two carry an identical light hex (`#C9CDDB`), differing only in dark (`#292E42` vs `#20232E`) — a shade nothing ever renders side by side.

**`border.footer` is dropped.** The footer rule renders with the same token as the title rule.

**Accepted visual change:** in dark themes the footer rule becomes marginally more prominent (`#292E42` rather than `#20232E`). Verified through the capture harness.

### 2.3 Naming principle — meaning and weight, never hue or place

Three naming failures are in play; two are failures:

| Kind | Example | Verdict |
|---|---|---|
| A **place** | `border.footer` | Wrong — goes stale as other surfaces reuse the token. |
| A **hue** | `accent.violet` | Wrong — lies in every port. A Gruvbox author writes `accent.violet = #d79921` (Gruvbox yellow) and the key actively misdescribes its own value. |
| A **meaning** | `state.destructive` | Right — stays true regardless of palette or where it is drawn. |

This does **not** make everything weight-based. The text ramp and the border want intrinsic-**weight** names because their role genuinely is "how prominent". The accents want **meaning** names because a theme author needs to know what a colour signifies in order to choose one.

### 2.4 The rename table

All 19 tokens, with the Go field name each maps to:

| # | Current token | New token | Go field | Why |
|---|---|---|---|---|
| 1 | `text.primary` | `text.primary` | `TextPrimary` | unchanged — top of ramp, already intrinsic |
| 2 | `text.strong` | `text.secondary` | `TextSecondary` | ordinal makes ramp position explicit |
| 3 | `text.muted-bright` | `text.tertiary` | `TextTertiary` | current name is self-contradictory |
| 4 | `text.detail` | `text.muted` | `TextMuted` | `detail` describes content, not weight |
| 5 | `text.dim` | `text.subtle` | `TextSubtle` | ladder consistency |
| 6 | `text.faint` | `text.faint` | `TextFaint` | unchanged — decorative floor |
| 7 | `text.on-selection` | `text.on-selection` | `TextOnSelection` | unchanged — contrast pairing |
| 8 | `accent.violet` | `accent.primary` | `AccentPrimary` | hue → role (primary accent) |
| 9 | `accent.blue` | `accent.key` | `AccentKey` | hue → role (key-hint) |
| 10 | `accent.cyan` | `accent.mode` | `AccentMode` | hue → role (signals a distinct mode) |
| 11 | `accent.orange` | `accent.attention` | `AccentAttention` | hue → role; one warm token covers filter query, edit-mode, warning flash |
| 12 | `state.green` | `state.positive` | `StatePositive` | hue → meaning (live / attached / success) |
| 13 | `state.red` | `state.destructive` | `StateDestructive` | hue → meaning |
| 14 | `canvas` | `canvas` | `Canvas` | unchanged — already intrinsic |
| 15 | `bg.selection` | `bg.selection` | `BgSelection` | unchanged — names a state, not a place |
| 16 | `bg.warning` | `bg.attention` | `BgAttention` | pairs with `accent.attention` |
| 17 | `bg.track` | `bg.subtle` | `BgSubtle` | use-site → intrinsic weight (a low neutral fill) |
| 18 | `border.separator` | `border` | `Border` | sole border token after consolidation |
| 19 | `text.on-warning` | `text.on-attention` | `TextOnAttention` | lockstep with `bg.attention` |

### 2.5 Role meanings (the public contract)

These meanings are the substance of `docs/theming.md` (§12.4), which is the source of truth for the contract.

**Text ramp — bright to faint, in weight order:**

| Token | Role |
|---|---|
| `text.primary` | Names, wordmark, active labels, modal titles, chip text |
| `text.secondary` | Selected-row meta, help actions, banner/signpost |
| `text.tertiary` | Done-tick labels, selected-row path |
| `text.muted` | Paths, counts, footer labels, subtitles, group headings |
| `text.subtle` | Group `··· N` counts, pending loading steps |
| `text.faint` | Decorative only — inactive dots, `+ add`, mode indicator, hints |
| `text.on-selection` | Name on the selected row (pairs against `bg.selection`) |

**Accents and states:**

| Token | Role |
|---|---|
| `accent.primary` | Cursor, selector bar, active dot, `?` key, focused field label, mode bar, loading bar |
| `accent.key` | Footer / modal key-hint glyphs |
| `accent.mode` | Sessions header, Preview chrome, active tick — signals a distinct mode |
| `accent.attention` | Filter query and `/`, edit-mode, warning flash `⚠` |
| `state.positive` | `●` attached, Sessions count, Projects label, `✓` done, success flash |
| `state.destructive` | Kill / delete emphasis, `▲` |

**Surfaces:**

| Token | Role |
|---|---|
| `canvas` | The owned mode-matched canvas, painted on every cell |
| `bg.selection` | Selected-row tint |
| `bg.attention` | Warning-flash band |
| `bg.subtle` | Low neutral fill — loading-bar empty track |
| `border` | Title rule, footer rule, modal panel frames, edit-modal chips |
| `text.on-attention` | Warning-flash message (pairs against `bg.attention`) |

### 2.6 Accepted ambiguities

Three spots were flagged as genuinely arguable and resolved to the values above:

1. **The ramp's middle join.** `text.tertiary` → `text.muted` mixes an ordinal vocabulary with a qualitative one, so ordering at that join rests on convention rather than the names. Fully positional names (`text.1`…`text.6`) would remove the ambiguity but strip all meaning from ~20 files of call sites — rejected. The ramp's weight ordering is documented in `docs/theming.md` (§12.4), which is where a theme author learns the vocabulary.
2. **`accent.key`** could read as "important" rather than "keyboard key". Accepted over `accent.keyhint` / `accent.hint`.
3. **`bg.subtle`** reuses the word from `text.subtle` in a different namespace. Accepted over `bg.inactive`, which generalises less well.

### 2.7 File ordering is not a contract

Token order in a theme file carries no meaning and is not enforced. The names carry their own meaning (unlike base16, where `base00`–`base07` must run dark-to-light because position *is* the meaning), and the flat `key = value` format parses unordered — so any ordering "contract" would be both unenforceable and undetectable. The ramp's weight ordering lives in `docs/theming.md`.

## 3. Theme model — split, not paired

### 3.1 A theme is one palette

Today a theme is `{20 tokens} × {light, dark}`: `Token{Name, Light, Dark}` with `Token.ColorFor(mode)` picking the variant, and `theme.Mode` threaded as a parameter through essentially every render helper in `internal/tui` (`headerStyle(tok, mode, colourless)` and ~20 files of the same shape) whose only job is to reach `ColorFor`.

**A theme becomes one palette of 19 values, and is itself light or dark.** MV splits into two built-ins carrying the existing values.

Decisive reasons:

- **Authoring burden under the contribution routes.** 19 values against one canvas versus 38 against two is the difference between a contributor porting a palette in an evening and not bothering — and dark-only famous palettes (Dracula, Nord) have no light half to supply at all.
- **The pairing MV implies isn't real.** Six of MV's light hexes needed *individual* correction and three light surface tints were eyeball-pinned at a validation gate. MV's light and dark are two independently-tuned palettes that happen to share token names; the struct claims a derivation relationship that does not exist.
- **Detection and pairing are independent axes.** Auto-detection with single-palette themes — where detection picks between two *named themes* rather than two variants — is a shipping design (Helix's). Wanting detection does not commit Portal to paired.
- Single-palette is the overwhelmingly dominant ecosystem shape.

### 3.2 Go-side data shape

- `Token` becomes `{Name, Value string}`.
- `Token.ColorFor` is **removed**.
- `theme.Mode` (the `Light`/`Dark` enum) is **removed**, along with its threading through the render layer.
- `Theme` remains a struct of 19 named `Token` fields with a stable-order `All()` accessor, but is no longer a package-level `var` holding one built-in — it is the parse result of a theme file (§4).
- `theme.MV` as an exported package-level value **ceases to exist**. Its values move into `tokyo-night.theme` and `tokyo-night-day.theme` (§7.3).

### 3.3 Consequences that follow from split

- **The "missing variant" problem ceases to exist** rather than being handled. There is no hole for a dark-only palette to leave.
- **No `appearance` pref survives.** The light/dark override becomes the *shape* of the theme setting (§8), not a mode enum.
- **The selector list is mixed-mode.** Arrowing in a dark terminal can land on a light theme and flip the whole canvas. This is accepted behaviour, not a defect (§9.2).
- **Contrast checking loses the product-side light/dark distinction** it needs for the three eyeball-pinned light surface tints. Resolved test-side: a test table is allowed to know things the runtime does not (§13.5).

### 3.4 Plumbing — the model holds the active theme

`theme.MV` is currently a package-level global read directly at ~182 call sites. Making the active theme switchable is a straight substitution rather than a new mechanism, because **split removes the `mode` parameter** from every one of those sites — so all 182 are being edited regardless, and a parameter slot is freed at exactly the same moment.

**The model holds the active `Theme` and passes it where `mode` is passed today.** No package-level mutable state (`theme.Active` var + setter), no new parameter.

Rejected: mutable package state. Its entire advantage was avoiding churn Portal is now paying anyway, and it would put order-dependent mutable state on the render path. Secondary benefit that matters in this codebase specifically: a test can construct a model with any theme instead of mutating a global and hoping nothing else observed it — and the suite already forbids `t.Parallel()` because the `cmd` package injects mocks via package-level mutable state.

## 4. Theme file format

### 4.1 Flat `key = value` with `#` comments

A theme file is a flat map of 19 `key = value` pairs with `#` comments. No JSON, no TOML, no third-party parser.

```
# Nord — https://www.nordtheme.com/
# state.destructive and state.positive are corrected for Portal's contrast floors.

canvas = #2E3440
text.primary = #ECEFF4
…
```

Rationale:

- Portal already parses this shape (`aliases`), so it is not a new idiom, and it needs **zero new dependencies** — every config today is stdlib `encoding/json` plus one flat `key=value` file.
- **JSON cannot carry comments**, and a theme is the one config file that genuinely wants them: ported palettes need attribution, and the eyeball-pinned light tints need a note recording the judgement behind them. Attribution being repo-side rather than in-UI makes a file header its natural home.
- TOML would add a third-party parser to a codebase that has deliberately avoided one for config, and buys nesting Portal does not need.
- The dividing line already implicit in Portal's own config: *nesting needed → JSON*; *flat human-authored map → flat file*. A theme is squarely the second.

Accepted cost: a small hand-rolled parser, and a second non-JSON config format to document.

**Forward note (not a requirement):** the deferred transparent-theme idea would need a distinguished value meaning "use the terminal default". The format should leave that door open rather than close it.

### 4.2 Lexical rules

`#` is both the comment marker and the hex prefix, and **every value in a theme file starts with `#`** — so the collision must be resolved explicitly. The forcing case is `text.primary = #ECEFF4 # tuned for the lighter canvas`: a colour plus a trailing note, or one invalid value?

| Rule | Detail |
|---|---|
| **Comments** | `#` starts a comment **only at the beginning of a line**, after optional leading whitespace. There are no trailing comments, so the ambiguity never arises — a `#` after `=` is always part of the colour. |
| **Values are bare** | Never quoted. A quoted value is **rejected** with a message saying so. |
| **Duplicate keys** | **Rejected**, not resolved. Silently taking one of two conflicting values is exactly the quiet wrongness the validity rule exists to prevent, and "all 19 present" would otherwise have to define what a repeat counts as. |
| **Whitespace** | Trimmed around `=`. Blank lines ignored. |
| **Keys** | Lowercase by definition (per the vocabulary charset), matched **case-sensitively**. |
| **Encoding** | CRLF tolerated; a BOM is stripped. |
| **Malformed lines** | A line that is neither blank, a comment, nor a well-formed `key = value` pair rejects the file (`bad syntax`). |

### 4.3 Value domain — hex only, `#RRGGBB`

**Values are hex only, in `#RRGGBB` form.** No ANSI indices, no named colours, no `#RGB` shorthand (six digits cost nothing and remove a parse branch).

Portal owns its own validator regardless, because `lipgloss.Color` **never returns an error** and its accepted domain is wider and stranger than a theme format wants: `"212"` is a valid ANSI-256 index, `"-5"` is silently abs'd to `5`, `"16777215"` is reinterpreted as packed RGB (white), and every failure is the silent `noColor` sentinel. Owning the validator is what turns all of that into one honest message.

Two reasons for excluding ANSI indices, the second decisive:

- The MV spec's §2.4 is an explicit decision that Portal **imposes its own exact hues via truecolor and does not inherit the terminal's 16 ANSI colours** — a recognisable identity needs consistent hues across machines. Admitting ANSI indices lets a theme opt back into the palette Portal deliberately declined.
- **An ANSI index has no fixed RGB.** The validator must parse to RGB anyway, and that same parse is what any contrast check needs. A token valued `212` cannot be measured against anything — admitting them would permanently foreclose checking a theme numerically, including Portal's own built-ins.

Hex case (upper or lower) is not constrained.

### 4.4 What a theme file may contain

A Portal theme file contains **exactly the 19 token keys and nothing else**. Unknown keys are ignored. There is no `name` field, no behaviour, no includes, no nesting.

**Security consequence, worth stating:** Ghostty's documented caveat — *a theme can set any config option, so don't use untrusted ones* — **does not transfer**. Portal's theme file is a closed key set of colour values with no capacity to influence anything else, so ingesting an unreviewed drop-in file carries no configuration-injection surface.

### 4.5 Full replacement, no merge

**Every theme must declare all 19 tokens.** There is no merge-over-a-base, no `inherits`/`parent`/`base` key, no partial files.

- The `go:embed` decision already solves the problem merge exists to solve: because a built-in *is* a file, "copy a built-in and edit it" is a first-class workflow (§12.1 makes it reachable), and at 19 tokens the copy is trivial.
- Merge drags in a **Portal-specific hazard**: the canvas is *itself a token*, so a partial theme supplying a new canvas while inheriting `text.primary` from a base produces an inherited foreground measured against a background it was never tuned for. Merge can silently compose two individually fine themes into an illegible one.
- Merge was never a requirement — it arrived as an inherited option. It stays available as a future addition, because full-replacement files remain valid under any later merge model (a file that declares everything simply inherits nothing).

### 4.6 Vocabulary evolution — ignore unknown, reject missing

The two directions the vocabulary can move are governed by two independent levers:

- **Unknown key → ignored.** This makes *removing* a token survivable: old files keep working.
- **Missing key → the whole theme is rejected.** It is not selectable, Portal falls back per §8.5, and a message names the missing tokens.

Rejected: per-token degradation (missing token falls back to a baked-in base default, theme still loads as "degraded"). It needs a new partial-load path and a fallback source that is not trivial under split — a light theme missing `text.primary` cannot borrow the dark built-in's value, so "base defaults" would have to mean *the same-mode built-in*, which is merge-with-a-base under another name with the canvas hazard intact.

Whole-theme rejection **reuses machinery Portal needs regardless**: "persisted theme isn't loadable" already has to exist for a deleted file, a renamed file, or a typo in `prefs.json`. Adding a 20th token in future routes into that same path rather than inventing a second one.

**Scope note:** this is near-hypothetical. Portal's own token rule (MV spec §2.8) is that a new surface reuses an existing role and a new token is promoted only where the value genuinely differs — the vocabulary is designed not to grow.

### 4.7 No variant concept

**Portal has no notion of a theme being "light" or "dark".** It is neither declared in the file nor derived from canvas luminance.

The mechanic has no consumer:

- Under the adaptive two-slot form, **the slot classifies the theme** — the light slot means "use this when the terminal is light". Portal never inspects the palette to know that.
- Warning that a dark theme sits in the light slot is a *perceptual* judgement, which validation explicitly never makes (§6.1).
- Grouping or filtering the selector list by variant is the deferred panel-search feature. Ordering same-mode themes first was proposed as a mitigation for the mixed-mode flash and **rejected** (§9.2), which removes the last candidate consumer.

The asymmetry is what makes not-deciding safe rather than merely convenient: *declaring* would lock a key into the public contract now, whereas *deriving* costs nothing and needs no format change — so if a selector filter ever ships, the value can be computed that day.

**The one exception is test-side, not product-side:** the contrast test table names which built-ins are light, because the three light surface tints are not numerically checkable (§13.5). A test is allowed to know things the runtime does not.

## 5. Identity, discovery & enumeration

### 5.1 The filename is the identity

**The filename minus its extension is the slug**, and the slug is the durable identity Portal persists in `prefs.json`, writes in config, and displays in the selector. There is no in-file `name` field and no separate display label.

- Zero duplication: file and content cannot disagree.
- Identity is structurally unique by virtue of being a filename in a directory.
- Renaming a theme is a file move — an operation users already understand.
- The contract is a *filename*, so a user renaming their own theme is a deliberate file operation with an obvious consequence, and Portal renaming a built-in is the same kind of breaking change as renaming a token: visible, deliberate, and rare.

An optional display-label field was considered and **rejected**. Two files with distinct slugs could both carry `name = "Nord"`, so labels could collide even though identity could not, and alphabetical ordering would become ambiguous (by slug or by label — they differ the moment a label is set). The cost is display prettiness (`tokyo-night-day` rather than "Tokyo Night Day"), judged not worth a second identifier-shaped thing in the file. Every comparable tool lists slugs, and the constrained charset reads cleanly.

### 5.2 Slug charset — `[a-z0-9-]`

**A slug must match `[a-z0-9-]`.** A file whose name does not is **rejected** with reason `bad name` and rendered as an unselectable row (§9.5).

**Reject, never normalise.** Lowercasing `Nord.theme` to `nord` would let it shadow the built-in, breaking the rule §5.4 exists to protect.

This removes the case question outright rather than defining case-insensitive matching, so the reserved-name check stays **exact string equality** — which is what the no-shadowing safety property requires, and what makes `Nord.theme` beside a built-in `nord` safe on a case-insensitive macOS filesystem.

The same charset check applies to a **persisted slug** read from `prefs.json` (§8.6).

### 5.3 Extension — `.theme`

Theme files carry the `.theme` extension. Some extension is needed for slug derivation; `.theme` is the choice.

### 5.4 No shadowing — built-in slugs are reserved

**A user file whose slug collides with a built-in is rejected**, with reason `reserved name`, through the same channel as any other invalid theme.

This exists because of a hard constraint: an invalid theme falls back to a built-in, so **if a user file could shadow the built-in that is the fallback, the fallback itself could be broken.** Drop in `tokyo-night.theme` with a typo'd hex and the thing Portal falls back to is the same broken file. That must be impossible.

Rejected alternatives: user-dir-shadows-built-ins with reserved names (needs a reserved-name special case, a precedence chain to document, and "which `nord` am I looking at?" ambiguity), and built-ins-always-win-silently (you edit a file and nothing happens, with no signal at all).

The workaround is a two-second file rename and is self-documenting: copy `nord` to `nord-lee.theme`. With the PR route open, genuinely *correcting* a built-in has a proper channel rather than needing a local override.

**Accepted consequence:** because built-in rows are deliberately indistinguishable from drop-in rows in the panel (§9.5), the reserved-slug set is not discoverable from the UI — a user learns a slug is reserved by having their file rejected with a message naming the conflict. `portal theme export` (§12.1) and `docs/theming.md` make the set discoverable outside the panel.

### 5.5 Directory resolution

The themes directory resolves through Portal's existing per-file chain shape:

**dedicated env var → `XDG_CONFIG_HOME/portal/themes/` → `~/.config/portal/themes/`**

Note this resolves a *directory* where `configFilePath` resolves *files* — a small mechanical difference. There is no one-shot migration from the old macOS Application Support path (the directory is new; nothing exists there to move).

**Directory states:**

| State | Behaviour |
|---|---|
| **Absent** | The common case. **Silent** — zero drop-ins is not an error. No doctor line, no log entry. Portal never creates or seeds it. |
| **Unreadable**, or a regular file where a directory belongs | A genuine misconfiguration: a **doctor advisory line** and a **log entry**. |

### 5.6 Enumeration rules

- **Top-level only** — files matching `.theme` in the directory itself. No subdirectory recursion.
- **Symlinked files are followed** — the standard dotfiles shape, and dotfiles users are exactly who hand-authors a theme. The slug derives from the link name as enumerated.
- **Symlinked directories are not followed.**

### 5.7 Discovery is lazy

Auto-discovery must not turn one config read into an N-file scan-parse-validate sweep on a cold path that is explicitly latency-engineered.

- **At construction**, Portal loads **only the nominated themes by name** — one file read for a constant, two for an adaptive pair (§8.4). No enumeration.
- **Enumeration happens only when the slide-over opens**, where a few milliseconds is invisible against the keypress that opened it.

This means the drop-in route can never degrade startup no matter how many files a user accumulates, and the exec path (`portal open <target>`) does no theme work at all.

Rejected: startup scan (pays the sweep on every launch including the overwhelming majority where nobody opens the selector), and `fsnotify` watching (machinery for a problem Portal does not have — it does not need to *watch* the directory, it needs to not *cache* it).

### 5.8 Enumeration re-reads on every open

The directory is enumerated **on every panel open**, not once per process. It is a directory read of a handful of small files behind a keypress; caching buys nothing measurable while breaking the loop the drop-in route exists for — copy a built-in, edit it, see it, without relaunching Portal.

---

## Working Notes
