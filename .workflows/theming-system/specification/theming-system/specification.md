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

## 6. Validity & rejection

### 6.1 The validity rule

**A theme is valid if and only if all 19 tokens are present AND every value is syntactically well-formed.**

Explicitly **not** checked at load: whether the colours are good, readable, mutually distinguishable, or clear any contrast floor. Validity is syntactic, never perceptual.

Validity is what makes a theme **selectable**. An invalid theme is listed but unselectable (§9.5), and anything nominating it falls back per §8.5.

### 6.2 The reason vocabulary

Seven reject classes. The terse label appears on the panel row; the detail appears in `portal doctor` and the `theme` log component.

| Reason | Cause |
|---|---|
| `missing tokens` | One or more of the 19 keys absent |
| `bad colour` | A value that is not a well-formed `#RRGGBB` hex |
| `bad syntax` | Duplicate key, quoted value, or a malformed line |
| `bad name` | Slug does not match `[a-z0-9-]` |
| `reserved name` | Slug collides with a built-in |
| `unreadable` | The file could not be read |
| `not found` | A slug named by `prefs.json` with no corresponding file |

*Which* token is missing, *which* line is malformed, and *which* key carries a bad colour stays in doctor, where there is width to enumerate.

### 6.3 Where rejection surfaces

The job splits by surface rather than forcing any one of them to do all of it:

| Surface | Carries |
|---|---|
| **The slide-over panel** | Every theme file gets a row; invalid ones render unselectable with the terse reason (§9.5). Sufficient to tell the user their file did not work and it is not their imagination. |
| **`portal doctor`** | The detail — full terminal width, per-file, enumerating exactly which tokens are missing or which key is bad. Advisory only; does not drive the exit code (§12.2). |
| **The `theme` log component** | The passive forensic trail (§12.3). The only record that exists without the user going looking. |

**Falling back must never overwrite the persisted theme name in `prefs.json`.** Portal keeps the user's choice and renders the fallback; fixing the theme file restores it on the next launch without the user re-selecting. Overwriting would make the failure destructive rather than transient.

A **permanent notice-band entry** was considered and rejected. Portal's notice band is a single-slot arbiter with six contenders already; a seventh permanent contender is a real cost for a rare event. Under whole-theme rejection the symptom is already loud — Portal is visibly the fallback theme instead of the user's — so the message is *explanation*, not alarm.

### 6.4 Two quality tiers

**Contrast floors apply to what Portal ships; syntactic validity applies to what users write.**

| Tier | Membership | Requirement |
|---|---|---|
| **Bundled** | Built-in, or an accepted PR — a PR is *intake into this tier* | Must be valid **and good**. Contrast floors, bands and thresholds are checked (§13.5). It carries Portal's name. |
| **Drop-in** | The user's themes directory | Must be **valid only** (§6.1). Whether it looks good is the user's business. |

The bundled tier is what stops the selector filling with Portal-endorsed themes nobody can read. Relaxing a floor for a named port was the one option ruled out, because it would break the guarantee that is the entire point of having tiers.

**Consequence: porting is not free.** A straight palette lift may not clear the floors unmodified — MV's own light variants needed six individual corrections, and the Nord port needs two (§7.4). Each bundled theme is real work, which argues for shipping a small number well rather than a large library.

### 6.5 Terminal colour capability — no action

A floor validated on a truecolor hex says nothing about the colour actually painted after `lipgloss`/`colorprofile` downsamples on a 256- or 16-colour terminal. Some applications (Helix) refuse truecolor themes on incapable terminals.

**Portal does not.** The MV spec's §2.4 already accepts downsampling as graceful degradation — "a hue may approximate, but the contrast floor still governs legibility" — and nothing about user themes changes that. Bundled themes are floor-checked on their truecolor values exactly as MV is today; drop-ins are syntactic-only by decision, so there is no floor to invalidate. This is real validation on an axis Portal has already chosen not to police.

## 7. Built-in theme set

### 7.1 A built-in *is* a theme file

Built-in themes are `.theme` files embedded via `go:embed` and parsed by the **same loader** as a user's drop-in. They are not Go structs.

- One code path, one format, one validity rule. A PR is "add a file". A user copies a built-in, tweaks two values, drops it in `themes/` — which is how people actually make themes.
- The format is dogfooded by every built-in, so a bad format is the maintainer's problem on day one rather than a stranger's on day ninety.
- Prior art: Ghostty and kitty avoid inventing a theme format at all — a theme *is* a config file.

Consequences:

- **Parse failures move from compile-time to load-time**, so built-ins need the build-time guarantee of §7.6.
- **`internal/capture`'s no-real-config import guard is preserved** — `go:embed` is not config discovery, so the embedded set stays reachable from the capture harness without touching the config path.
- **MV's inline erratum comments are deleted, not ported.** `contrast_test.go` already enforces the corrected values numerically, so a comment recording *why* a hex differs from its upstream sibling is duplicated history — revert a hex and the test fails, with or without the comment. The one class of judgement that is *not* numerically recoverable (the three eyeball-pinned light surface tints) moves into the theme file as a `#` comment, which the flat format supports (§13.6).

### 7.2 The shipped set — three built-ins

Portal ships **three** built-in themes:

| Slug | Palette |
|---|---|
| `tokyo-night` | Tokyo Night Dark — the existing MV dark values |
| `tokyo-night-day` | Tokyo Night Light — the existing MV light values |
| `nord` | Nord, dark-only as the palette is |

Two routes were rejected: split-only (two themes — satisfies the letter of "not a single built-in" but not the spirit of "genuine options", being one palette in two modes), and a four-theme set including a second light theme.

The deciding argument was **risk, not scope**: the 19-token vocabulary has only ever been exercised by the palette it was designed for, so porting one genuinely external palette is the first real test of whether the roles map cleanly — and that test must happen *before* the names become a public contract. Nord makes the test unusually sharp because its canvas is `#2E3440`, a mid-dark rather than a near-black, so its contrast headroom is materially tighter than MV's.

The counterweight is that everything after the first external theme is cheap by construction: `go:embed` makes adding a theme literally adding a file, and the PR route exists to receive exactly that.

**Accepted cost:** the light side ships with a single option until the follow-up (§7.5). The adaptive default still works out of the box either way, since it is Tokyo Night on both slots.

### 7.3 Tokyo Night Dark and Light — the existing MV values

The existing MV values move across unchanged, subject to the erratum re-derivation check of §7.7.

**`tokyo-night.theme`** (from MV's `Dark` variants):

| Token | Value | | Token | Value |
|---|---|---|---|---|
| `canvas` | `#0b0c14` | | `accent.primary` | `#BB9AF7` |
| `text.primary` | `#C0CAF5` | | `accent.key` | `#7AA2F7` |
| `text.secondary` | `#A9B1D6` | | `accent.mode` | `#7DCFFF` |
| `text.tertiary` | `#828BB8` | | `accent.attention` | `#FF9E64` |
| `text.muted` | `#737AA2` | | `state.positive` | `#9ECE6A` |
| `text.subtle` | `#535C86` | | `state.destructive` | `#F7768E` |
| `text.faint` | `#3B4261` | | `bg.selection` | `#28243a` |
| `text.on-selection` | `#FFFFFF` | | `bg.attention` | `#241B10` |
| `border` | `#292E42` | | `bg.subtle` | `#26283A` |
| `text.on-attention` | `#E8C9A0` | | | |

**`tokyo-night-day.theme`** (from MV's `Light` variants):

| Token | Value | | Token | Value |
|---|---|---|---|---|
| `canvas` | `#e1e2e7` | | `accent.primary` | `#8A3FD1` |
| `text.primary` | `#2E3C64` | | `accent.key` | `#2D5CCA` |
| `text.secondary` | `#3F4760` | | `accent.mode` | `#0D6C87` |
| `text.tertiary` | `#4C5478` | | `accent.attention` | `#9A5200` |
| `text.muted` | `#586093` | | `state.positive` | `#3B5E18` |
| `text.subtle` | `#767DA2` | | `state.destructive` | `#BD2545` |
| `text.faint` | `#AEB2C6` | | `bg.selection` | `#D0C6F0` |
| `text.on-selection` | `#1A1B2E` | | `bg.attention` | `#E8D6A8` |
| `border` | `#C9CDDB` | | `bg.subtle` | `#D2D4DE` |
| `text.on-attention` | `#7A4B12` | | | |

Note `border` takes the former `border.separator` value in both; `border.footer` is dropped (§2.2).

### 7.4 The Nord port

Nord is a 16-slot ANSI palette (Polar Night `nord0–3`, Snow Storm `nord4–6`, Frost `nord7–10`, Aurora `nord11–15`). Portal's 19-token vocabulary is meaningfully wider than 16 slots **at the dark end**, so the port takes 14 values directly, **corrects two**, and **invents three**.

**`nord.theme`:**

| Token | Value | Source | Ratio vs canvas |
|---|---|---|---|
| `canvas` | `#2E3440` | nord0 | — |
| `text.primary` | `#ECEFF4` | nord6 | 10.84 |
| `text.secondary` | `#E5E9F0` | nord5 | 10.26 |
| `text.tertiary` | `#D8DEE9` | nord4 | 9.25 |
| `text.muted` | `#939EB2` | **invented** | 4.62 |
| `text.subtle` | `#73819B` | **invented** | 3.18 |
| `text.faint` | `#4C566A` | nord3 | 1.69 |
| `text.on-selection` | `#FFFFFF` | functional maximum | 8.63 on `bg.selection` |
| `accent.primary` | `#B48EAD` | nord15 | 4.41 |
| `accent.key` | `#81A1C1` | nord9 | 4.64 |
| `accent.mode` | `#88C0D0` | nord8 | 6.24 |
| `accent.attention` | `#EBCB8B` | nord13 | 8.00 |
| `state.positive` | `#A7C492` | **corrected** from nord14 `#A3BE8C` | 6.51 canvas / 4.50 selection |
| `state.destructive` | `#DD8188` | **corrected** from nord11 `#BF616A` | 4.50 |
| `bg.selection` | `#434C5E` | nord2 | 1.45 fill |
| `bg.subtle` | `#3B4252` | nord1 | 1.24 fill |
| `bg.attention` | `#3D4046` | **invented** | 1.20 fill |
| `border` | `#4C566A` | nord3 | no numeric floor |
| `text.on-attention` | `#ECEFF4` | nord6 | 9.02 on `bg.attention` |

**Correction 1 — the red.** `state.destructive` carries the 4.5 normal floor; Nord's published red `#BF616A` measures **3.05** against Nord's own canvas. Shipped corrected as `#DD8188` (4.50). The floor holds with no carve-out — this being the *first* external palette, a carve-out granted here would set the precedent for every PR theme after it.

**Correction 2 — the green.** The single `state.positive` token must clear **both** the canvas and the selection tint. Nord's `#A3BE8C` clears canvas at 6.13 but only **4.23** on nord2. Corrected to `#A7C492` (Oklab ΔE 0.018 — essentially imperceptible), clearing selection at 4.50 and canvas at 6.51, with chroma marginally *above* the original. This is precisely the problem MV itself solved by darkening its light green.

**Invention 1 & 2 — the ramp's middle.** Nord's greys are barrelled at the ends: three bright (9.25 / 10.26 / 10.84) and three dark (1.24 / 1.45 / 1.69), with nothing between. Portal needs `text.muted` ≥ 4.5 and `text.subtle` in the 3.0–4.5 band, so both are interpolated on nord3's hue and saturation.

**Invention 3 — the warning band.** `bg.attention` is a *background tint* — neither a neutral from the barrelled dark end nor a foreground accent. Settled at `#3D4046` (~8% nord13-into-canvas blend, fill 1.20), matching MV's own proportion: MV's `bg.warning` measures only **1.15** against its canvas — the tint is a whisper, not a wash. A first arithmetic answer (`#54524F`, a 20% blend at fill 1.60) was rejected at a visual gate as far too heavy and pushed into a warm grey outside Nord's cool family.

**One honest divergence from MV:** MV warms its on-band text (`#E8C9A0`) to match the band. Nord's Snow Storm is entirely cool and has no warm light, so `text.on-attention` uses nord6 — cooler than MV's treatment, but faithful to the palette. A deliberate port choice.

**Structural finding worth carrying forward:** Nord's dark end holds only three values (nord1/2/3) for Portal's **five** dark-end roles (`bg.subtle`, `bg.selection`, `border`, `text.faint`, `bg.attention`). `nord3` therefore serves both `border` and `text.faint`, *and* `bg.attention` is interpolated outright. A palette choosing one value for two roles is legitimate (unlike two tokens that differ pointlessly, which the border consolidation removed) — but **every port should expect to invent at the dark end.**

**Fidelity versus floors — resolved.** The floors win, and the corrected values ship under the palette's own name. No application maps a 16-slot ANSI palette 1:1 onto its own semantic roles; every Nord port in the wild adapts. The corrections are minimal and perceptually close, judged **visually** (both reds mocked side by side in a Nord kill modal), and `docs/theming.md` records them alongside the attribution.

**Derivation method — corrections versus inventions.** Contrast **corrections** must be computed in a **perceptual space (Oklab), never by moving HSL lightness** — raising lightness at fixed HSL saturation *drops actual chroma* (the first red offered, `#CF888F`, lost ~27% of Nord's red saturation and read washed-out and pink). A correction has a published source value whose chroma must be preserved. An **invented** value has no source to preserve; its constraints are landing in the right band and looking right, which is why `bg.attention` was settled at a visual gate rather than by arithmetic.

**Outstanding visual gate:** `text.subtle` has no locus on any captured Nord frame — it renders group `··· N` counts and pending loading steps, neither of which appears on the flat Sessions frame. **It needs a visual gate at implementation, on a grouped Nord capture.** (`text.muted` has already been seen — it is the "N window(s)" text on `Sessions — Nord (port)`.)

### 7.5 A further light theme — follow-up work

A second light theme is deferred to separate work, and is **a design task, not a file drop**:

- A **dark** theme is genuinely near-free: floor checks auto-enumerate the embedded set, and no eyeball pins are involved.
- A **light** theme requires `TestLightSurfaceTintsPinned` per-light-theme, whose pins are established by human eyeball at a visual gate through `capturetool` — the only viable route, because Portal cannot be run from a temporary build (§13.1).
- **There is no CI** (tests and lint run locally), so a contributor gets no signal that their theme fails a floor until the maintainer runs the suite.

### 7.6 The build-time guarantee

There is **no runtime fallback to hardcoded values** beneath the built-in fallback. Instead the situation is made impossible at build time.

**A unit test must:**

1. **Parse and validate every embedded built-in** against the full validity rule (§6.1).
2. **Assert that every fallback slug and the shipped default pair resolve within that set.**

Both halves are load-bearing. Validating the files alone proves the *files* are good, but the fallback is hardcoded slug constants (`tokyo-night`, `tokyo-night-day`) resolving *into* that set — rename a built-in file in a later PR, or typo a constant, and every embedded theme still validates while **every fallback path becomes unresolvable.**

With no path pretending to handle it, a binary somehow shipped with a broken default fails **loudly at startup** rather than limping on values nobody chose. `main.go` already owns a panic-recovering exit with a `process: panic` lifecycle marker, so that is a *marked* termination, not an unhandled crash.

Rejected: a compiled-in last-resort palette equal to Tokyo Night Dark. A build-time guarantee beats a runtime crutch.

### 7.7 MV's erratum values — a re-derivation check

MV's six corrected light values are described in-source as *"darkened, hue-preserved"*, which may carry the same chroma flaw as the rejected Nord red — in the opposite direction.

**Owned by this feature's implementation, before MV's values are frozen into theme files:** re-derive the six corrected light values in Oklab, measure chroma loss, and give a **fresh visual gate** to any that moved materially.

**Flagged consequence:** if the check finds anything, shipped colours change, `TestLightSurfaceTintsPinned`'s eyeball-established pins move, and "Tokyo Night Dark/Light are just the existing values" (§7.3) stops holding exactly. **The built-in-set decision is conditional on this check.**

## 8. The theme setting — resolution & detection

### 8.1 On-disk shape — three flat string keys in `prefs.json`

`prefs.json` gains three keys alongside the existing `session_list_mode`:

```json
{
  "session_list_mode": "flat",
  "theme": "",
  "theme_light": "",
  "theme_dark": ""
}
```

Rejected: a polymorphic `theme` field (string *or* object — tolerant-decoding a two-typed field means probing both, and "what does a corrupt value degrade to" turns murky in the store meant to be dumbest), and an always-object form (`{"constant": …}` / `{"light": …, "dark": …}` — explicit but verbose for the common case, and invents a wrapper key).

Three flat keys match what `prefs.json` already is — a flat map of scalars — so **tolerant decode stays exactly as dumb as today**: missing, empty or unrecognised falls to the shipped default *per field*, with no type probing.

**`prefs.json` is the hand-editable home for the theme setting.** Portal has no separate user config file, and prefs already holds `appearance` today with the README instructing users to set it by hand. The theme setting inherits exactly that: machine-written by the panel, hand-editable by anyone who prefers.

### 8.2 Two states, not three

A theme setting is either:

- **Constant** — `"theme": "nord"`. Detection is never consulted.
- **Adaptive** — `"theme_light"` / `"theme_dark"`. Detection chooses.

"Nothing set" and "pair nominated" are **the same state**: the shipped default *is* an implicit pair, so the loader needs no unconfigured branch — only a default value per slot.

**Mutual exclusion is enforced on write.** Committing a constant clears both slots; assigning a slot clears the constant. Whichever was set last wins, so "both a constant and a pair are present" cannot arise from Portal's own writes.

**If a hand-edit leaves both present, `theme` wins** — a documented deterministic rule. The "only two states" model stays a *rule* rather than being encoded in a type: non-empty `theme` ⇒ constant, otherwise the pair.

### 8.3 The shipped default is the adaptive pair

Portal ships with the pair already nominated:

```
theme_light = tokyo-night-day
theme_dark  = tokyo-night
```

So a brand-new user gets whichever matches their terminal, automatically.

Reasons over shipping a constant dark default:

- **The 50ms is a timeout, not a price** — terminals that answer do so in single-digit ms — and it applies only to TUI launches, since `portal open <target>` execs without painting.
- **It degrades to the alternative**: no answer resolves to dark, so the adaptive pair is a superset of a constant dark default with a bounded downside.
- **Asymmetric escape.** Pinning is one line and is the *simpler* config (`"theme": "tokyo-night"`), so an annoyed user has an obvious remedy. The alternative's failure has no signal at all — a light-terminal user gets a dark Portal forever and never learns a light theme exists.

**Risk named:** a terminal that answers OSC 11 inconsistently makes Portal flip between launches. The one-line pin is the remedy.

**Partial pairs do not exist.** The adaptive form always has two slots and the shipped values are their *defaults*, so `"theme_dark": "nord"` yields `{light: tokyo-night-day, dark: nord}` — light is still the shipped default because it was never overridden. There is no incomplete-pair state to validate, explain, or render around, and the shipped default and a partially-overridden pair are **the same mechanism** rather than two.

### 8.4 Construction timing — load every nominated theme

**At construction Portal loads every *nominated* theme — at most two.** The light/dark gate then only **selects** between values already in hand.

This is load-bearing because three other decisions collide otherwise: the model holds the active `Theme` (§3.4), discovery is lazy and does one read by name (§5.7), and a two-slot user's light/dark resolves **after** `Init`, when the OSC 11 reply or the 50ms timeout lands. Since the shipped default *is* the adaptive pair, the common path constructs the model before the slot is known — and both alternatives were bad: defer the read onto the first-paint critical path, or paint dark and flip.

**Cold-path cost:** one file read for a constant, two for a pair. No file read on the critical path, no flip.

**Mid-session slot assignment reads at commit time.** A constant nominates one theme, so assigning a slot (converting the user to adaptive in-session) reads that slot's file at keypress time — already the panel's cost model.

### 8.5 Fallback — per-slot and mode-matched

When a nominated theme is unloadable (invalid file, missing file, bad persisted slug):

| Nominated slot | Falls back to |
|---|---|
| `theme_dark` | `tokyo-night` |
| `theme_light` | `tokyo-night-day` |
| `theme` (constant) | `tokyo-night` |

This introduces **no new mechanism** — it is the already-decided "an unset slot holds the shipped default" rule applied to a slot that is *set but unloadable* rather than unset. One rule covers both cases, and it makes the shipped adaptive default and the fallback default **the same values**.

Rejected: a single fixed fallback regardless of mode. Simpler to state, worse in practice — a light-terminal user with a typo in their light slot would be thrown to a dark theme, a bigger surprise than falling to the light default.

**One not-loadable path serves every cause** — a deleted file, a renamed file, a typo in `prefs.json`, a missing token, a bad colour. All fall back, keep the persisted name (§6.3), and surface through the panel, doctor and the log.

### 8.6 The persisted slug is validated before use

The persisted value comes from a hand-editable file and is used to **locate a file by name** on a path that deliberately does not enumerate — so `../something` would be used as a path component.

**Validate the persisted slug against the same `[a-z0-9-]` charset before use** (§5.2), and treat an invalid one as unresolvable: fallback plus report, identical to any other unresolvable theme.

### 8.7 Light/dark detection

**Detection ships. The signal is the terminal background via OSC 11.** DEC mode 2031 (the OS colour scheme) is deliberately **not** adopted.

The two answer different questions: `ModeLightDark` reports *the operating system's* colour-scheme preference; OSC 11 reports *what colour the terminal's background is*. They routinely disagree — a terminal pinned dark on a light OS is the canonical case. On terminals that don't support 2031, tmux *synthesises* the answer by guessing from the background colour anyway.

**What detection is for discriminates the signals.** Because Portal owns an opaque canvas and guarantees its contrast floors against that canvas, a mode mismatch is *jarring, never illegible*. Detection's entire payoff is therefore **aesthetic blending with the surrounding terminal** — which wants the terminal's background, OSC 11's question.

Three arguments carried it:

1. **Transition dominance.** Portal's dwell time is seconds — launch, pick, exec into a session, many times a day — so the transition in and out dominates the experience. Matching the terminal reads as "your terminal, with a picker in it". Matching the OS against a pinned terminal flashes light and drops back to dark, twice per use.
2. **A terminal/OS mismatch is usually deliberate, not stale.** A pinned terminal is an explicit choice about the environment Portal lives in. For something that lives inside a terminal, the terminal's background is arguably the *more* relevant preference.
3. **Forward compatibility with transparency** (deferred, not rejected). A transparent theme *must* follow the terminal background, so choosing terminal now makes adding transparency later purely additive.

**Accepted cost: OSC 11 is query-only; 2031 pushes on change.** Portal gets *correct-at-startup*, not *live-following* — a terminal that flips mid-session is not noticed until the next launch. Judged thin: terminal backgrounds rarely change mid-session, and when they do it is usually because the terminal is itself following the OS.

**The "detection is unreliable inside tmux" premise is retired.** It was the main argument for deleting the appearance axis entirely, it appears in the README, and it does not survive testing — OSC 11 works reliably through tmux. The README advice that rests on it comes out with the setting (§12.5).

**A one-shot detection seed is not shipped.** Under this design detection acts only when the user nominated a pair — but since the shipped default *is* a pair, a brand-new user with a light terminal already gets the light theme. There is no unconfigured case left to seed.

### 8.8 What survives and what dies in the appearance gate

Under the two-slot form, the gate is only *partly* removed:

| | |
|---|---|
| **Dies** | `prefs.Appearance` — the `auto\|light\|dark` enum, its tolerant decode, `LoadAppearance`/`SaveAppearance`, `WithAppearance`. (`SaveAppearance` has no production caller today, so this is mostly read-path removal.) |
| **Dies via split** | `Token.ColorFor`, `theme.Mode` threading, the dual-canvas contrast bookkeeping. |
| **Survives, but conditional** | The detect-or-timeout first-paint gate. A user on a **constant** theme needs no detection, so their first paint is immediate — a real startup win. A user on the **adaptive** form still needs light/dark resolved *before* first paint or Portal paints one theme and flips, so the same race, ~50ms timeout and **dark** no-answer fallback still apply. |
| **Survives unchanged** | The OSC 11 *query* itself — `restore.go` needs it to capture the original background for restore-on-exit, independent of detection. The `NO_COLOR` carve-out. The canvas-echo guard, whose comparison re-points from "the mode's canvas" to a retained startup canvas hex (§11.4). |

**The query is issued from `Init` regardless of the setting shape.** That is what makes a mid-session constant → adaptive conversion work without a new query, race or gate (§9.3).

### 8.9 Concurrent instances and prefs writes

Portal's multi-window burst routinely produces several concurrent processes, so multiple live instances are normal.

- Each instance loads its theme at construction; an instance that changes theme persists it; **other instances are unaffected until relaunch.** There is no file watch.
- This is exactly how `session_list_mode` already behaves — the `s` toggle persists per-instance with no cross-instance sync, via the existing `ModePersister` seam that a theme persister follows.

**But a stale whole-file write can silently revert a theme.** Before this feature `prefs.json` had one field with a production writer. It now holds four independently-mutated fields written from two surfaces: instance A, constructed ten minutes ago, presses `s` and writes *its* in-memory prefs, silently reverting the theme instance B just committed. `AtomicWrite` does not help — this is a lost update, not a partial write.

**Both writers must read-modify-write:** re-read `prefs.json` immediately before writing, mutate only their own field(s), and write the merged result. Not novel — the project and hooks stores already do this for their own mutations.

`prefs.json` continues to go through `fileutil.AtomicWrite`, so all three theme keys land in one atomic write and partial failure is impossible.

---

## Working Notes
