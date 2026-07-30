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

This is the ecosystem's standard two-tier shape (library directory + selection setting) without committing to ecosystem-scale governance. It sets the two quality tiers (§6.4) and makes the token names a public contract worth settling now: renaming a token is a mechanical repo-wide change for built-ins, but it *breaks* files in a user's themes directory.

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
- **A theme "variant" (light/dark) concept** anywhere in the product — neither declared in a file nor derived at load (§4.7).

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
- **The pairing MV implies isn't real.** Six of MV's light hexes needed *individual* correction and four light surface tints were eyeball-pinned at a validation gate. MV's light and dark are two independently-tuned palettes that happen to share token names; the struct claims a derivation relationship that does not exist.
- **Detection and pairing are independent axes.** Auto-detection with single-palette themes — where detection picks between two *named themes* rather than two variants — is a shipping design (Helix's). Wanting detection does not commit Portal to paired.
- Single-palette is the overwhelmingly dominant ecosystem shape.

### 3.2 Go-side data shape

- `Token` becomes `{Name, Value string}`.
- `Token.ColorFor` is **removed**.
- `theme.Mode` (the `Light`/`Dark` enum) is **removed**, along with its threading through the render layer.
- `Theme` remains a struct of 19 named `Token` fields with a stable-order `All()` accessor, but is no longer a package-level `var` holding one built-in — it is the parse result of a theme file (§4). It is an ordinary struct, constructible in a test without going through the loader (which is what the swap-and-diff guard's synthetic themes need, §13.4).
- **`All()`'s stable order is the §2.4 table order, 1 through 19.** It was previously asserted without being defined; the numbering is the definition.
- **`Theme` carries no identity field.** The slug is held alongside the palette by whatever loaded it — the model for the active theme, the enumeration row for a listed one. This is what lets `capturetool --theme <path>` (§13.3) work at all: a theme loaded from an explicit path has no slug, and a struct with a mandatory-but-empty identity field would be lying. Consumers that need both (the `theme: loaded` log line, the panel's `●` placement) already have the slug in hand, because they are the ones that resolved it.
- `theme.MV` as an exported package-level value **ceases to exist**. Its values move into `tokyo-night.theme` and `tokyo-night-day.theme` (§7.3).

### 3.3 Consequences that follow from split

- **The "missing variant" problem ceases to exist** rather than being handled. There is no hole for a dark-only palette to leave.
- **No `appearance` pref survives.** The light/dark override becomes the *shape* of the theme setting (§8), not a mode enum.
- **The selector list is mixed-mode.** Arrowing in a dark terminal can land on a light theme and flip the whole canvas. This is accepted behaviour, not a defect (§9.2).
- **Contrast checking loses the product-side light/dark distinction** it needs for the four eyeball-pinned light surface tints (§13.5). Resolved test-side: a test table is allowed to know things the runtime does not.

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

**Branch-by-branch, because each one is a user-visible reason label and a test case in the embedded-set validity test (§7.6):**

| Input | Reason | Why |
|---|---|---|
| `text.primary =` (empty value) | `bad colour` | The line *is* a well-formed pair; the value simply is not `#RRGGBB`. |
| `= #FFFFFF` (no key) | `bad syntax` | Not a pair. |
| `text.primary` (no `=`) | `bad syntax` | Not a pair. |
| A duplicated key — **known or unknown**, **same value or different** | `bad syntax` | The duplicate check is lexical and runs before any key is classified or compared. Making it conditional on the key being known, or on the values differing, adds branches to buy nothing. |
| `Text.Primary = …` | ignored as unknown → file fails `missing tokens` | Keys match case-sensitively (above). Doctor names the missing token, which is what makes it findable (§6.2). |
| `#FFF` / `#FFFFFFFF` / `#GGGGGG` | `bad colour` | §4.3 admits `#RRGGBB` only. |
| Trailing or internal whitespace in a value | `bad colour` | Trimming is defined around `=` only; a value with interior whitespace is not a valid hex. Trailing whitespace after the value **is** trimmed, since it is whitespace around the pair rather than inside the value. |
| An empty file, or one containing only comments | `missing tokens` | It parsed; it declares nothing. |
| A BOM anywhere but the first bytes of the file | `bad syntax` | The BOM strip applies at file start only. |

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

**The one exception is test-side, not product-side:** the contrast test table names which built-ins are light, because the four light surface tints are not numerically checkable (§13.5). A test is allowed to know things the runtime does not.

## 5. Identity, discovery & enumeration

### 5.1 The filename is the identity

**The filename minus its extension is the slug**, and the slug is the durable identity Portal persists in `prefs.json`, writes in config, and displays in the selector. There is no in-file `name` field and no separate display label.

- Zero duplication: file and content cannot disagree.
- Identity is structurally unique by virtue of being a filename in a directory.
- Renaming a theme is a file move — an operation users already understand.
- The contract is a *filename*, so a user renaming their own theme is a deliberate file operation with an obvious consequence, and Portal renaming a built-in is the same kind of breaking change as renaming a token: visible, deliberate, and rare.

An optional display-label field was considered and **rejected**. Two files with distinct slugs could both carry `name = "Nord"`, so labels could collide even though identity could not, and alphabetical ordering would become ambiguous (by slug or by label — they differ the moment a label is set). The cost is display prettiness (`tokyo-night-day` rather than "Tokyo Night Day"), judged not worth a second identifier-shaped thing in the file. Every comparable tool lists slugs, and the constrained charset reads cleanly.

### 5.2 Slug charset — `[a-z0-9-]`

**A slug must match `^[a-z0-9][a-z0-9-]*$`** — lowercase letters, digits and hyphens, at least one character, not starting with a hyphen. A file whose name does not is **rejected** with reason `bad name` and rendered as an unselectable row (§9.5).

The anchoring closes three edges a bare character class leaves open: the **empty slug** is illegal (so a file named exactly `.theme` is rejected, and the empty string stays unambiguously the *unset* sentinel of §8.1), a **leading hyphen** is illegal (it reads as a flag in every context a slug is typed into), and a **trailing hyphen** is legal but pointless. There is **no length bound** — the slug is an identity, and §9.8's truncation is a display concern that must not silently become a validity rule.

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

**`PORTAL_THEMES_DIR` → `XDG_CONFIG_HOME/portal/themes/` → `~/.config/portal/themes/`**

The env var is named here rather than left to implementation because it is a user-facing documented contract — `docs/theming.md` (§12.4) has to print it, and every other member of Portal's config chain carries a spec-fixed name for the same reason. The `_DIR` suffix (rather than the `_FILE` of `PORTAL_TERMINALS_FILE` and siblings) marks the mechanical difference: this resolves a *directory* where `configFilePath` resolves *files*. There is no one-shot migration from the old macOS Application Support path (the directory is new; nothing exists there to move).

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

**The enumeration's parse results are retained for the panel's lifetime**, so arrowing previews from values already in hand — no file read per keystroke, which is what keeps the swap the O(1) restyle of §11.1. They are discarded when the panel closes; the next open re-reads.

**The panel's parse supersedes the construction-time parse for the same slug.** After a mid-session edit the panel holds the fresher truth, and that is the entire point of re-reading. Two consequences, both following from the same rule:

- **`Esc` resolves persisted state against the panel's enumeration**, not against what construction loaded. If the user edited their active theme's file and broke it, `Esc` lands on the §8.5 fallback — Portal shows what the config now says, not a stale copy it happens to still hold.
- **The mirror case works for the same reason**: fixing a previously-invalid theme takes effect on the next panel open, without relaunching. That symmetry is what §5.8 exists to buy.

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

**Reasons are evaluated in a fixed order and the first failure short-circuits**, so a file always has exactly one reason and the panel's single-reason row is never a choice:

1. `bad name` — the slug is checked before the file is opened, so a `bad name` file can never also report `unreadable` or anything about its contents.
2. `reserved name` — likewise decided from the slug alone, before any read.
3. `unreadable` — the read itself failed.
4. `bad syntax` — lexical failure (§4.2) aborts the parse, so no value-level or presence check runs.
5. `bad colour` — per-value validation across the whole file.
6. `missing tokens` — the presence check runs last, on a file that parsed and whose every present value is well-formed.

`not found` is not in this ladder — it applies only to a persisted slug with no file (§9.4), where there is nothing to check.

**Doctor enumerates within the reason, not across reasons** — all missing tokens, or all bad-coloured keys, for the one reason that applies. It does not report a file as both `bad colour` and `missing tokens`.

**A wrong-case key is an unknown key** (§4.2 matches case-sensitively), so `Text.Primary` is ignored and the file fails as `missing tokens`. That reason is technically accurate but can misdirect, so doctor's detail line names the missing tokens — which is what makes the mistake findable.

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
- **MV's inline erratum comments are deleted, not ported.** `contrast_test.go` already enforces the corrected values numerically, so a comment recording *why* a hex differs from its upstream sibling is duplicated history — revert a hex and the test fails, with or without the comment. The one class of judgement that is *not* numerically recoverable (the four eyeball-pinned light surface tints — §13.5) moves into the theme file as a `#` comment, which the flat format supports (§13.6).

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
| `accent.mode` | `#88C0D0` | nord8 — chosen over nord7 `#8FBCBB` (5.99) as Nord's own primary UI accent | 6.24 |
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

**The pairing legs the port was verified against.** The per-token ratios above are only half the rule set — the second correction was found by walking the *pairing* legs. This is the port's verification baseline, to be re-checked if any value moves (§7.7):

| Leg | Nord | Floor | |
|---|---|---|---|
| `bg.subtle` fill vs canvas | 1.24 | ≥ 1.10 | ✓ |
| `state.positive` on `bg.selection` | 4.23 → **4.50** corrected | ≥ 4.50 | ✗ → ✓ |
| `text.on-selection` on `bg.selection` | 8.63 | ≥ 4.50 | ✓ |
| `text.secondary` on `bg.selection` | 7.09 | ≥ 4.50 | ✓ |
| `text.tertiary` on `bg.selection` | 6.39 | ≥ 4.50 | ✓ |
| `text.on-attention` on `bg.attention` | 9.02 | ≥ 4.50 | ✓ |
| `accent.mode` vs canvas (peek chrome) | 6.24 | ≥ 4.50 | ✓ |
| `text.subtle` band | 3.18 | 3.00–4.49 | ✓ |
| `text.faint` band | 1.69 | 1.00–2.99 | ✓ |
| `state.destructive` vs canvas | 4.50 | ≥ 4.50 | ✓ |

**A failure on an unwalked leg can force re-deriving an *invented* value — which then needs a fresh visual gate.** The port was twice found incomplete (first covering 16 of 19 tokens, then roughly half the rule set), and each time the completeness claim was plausible enough to pass unexamined. The floor test auto-enumerating the embedded set (§13.5) means a missed leg surfaces at implementation rather than shipping — but if it lands on `text.muted`, `text.subtle` or `bg.attention`, the new value is an *invention*, and this port's own precedent (§7.4, `bg.attention`) is that inventions are settled at a visual gate rather than by arithmetic.

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

With no path pretending to handle it, a binary somehow shipped with a broken default fails **loudly at startup** rather than limping on values nobody chose.

**Mechanism:** the loader returns an ordinary error for an embedded parse failure — it does not panic. The escalation happens where the fallback is *needed*: a fallback that cannot resolve is a fatal error returned up the normal path, so the user sees a one-line message rather than a Go panic trace. `main.go`'s panic-recovering exit and its `process: panic` lifecycle marker remain the backstop for a genuine programming fault, not the designed route. **Validation is not startup-eager** — nothing walks the embedded set at init, because §7.6's test already proves it at build time and re-proving it on every launch buys nothing on a cold path this feature otherwise adds no cost to.

Rejected: a compiled-in last-resort palette equal to Tokyo Night Dark. A build-time guarantee beats a runtime crutch.

### 7.7 MV's erratum values — a re-derivation check

MV's six corrected light values are described in-source as *"darkened, hue-preserved"*, which may carry the same chroma flaw as the rejected Nord red — in the opposite direction.

**Owned by this feature's implementation, before MV's values are frozen into theme files:** re-derive the six corrected light values in Oklab — the minimal-Oklab-distance colour that clears the same floor — and compare each against the shipped value.

**Threshold: Oklab ΔE ≥ 0.05 is "moved materially".** The Nord port anchors the scale at the other end (ΔE 0.018, cited as essentially imperceptible), and 0.05 is comfortably above that while still well below a difference anyone would describe as a colour change. Under it, nothing happens.

**Acceptance criteria, so the check has a determinate outcome either way:**

- **Every value under threshold** → the check passes, `§7.3`'s tables stand, nothing moves, and the result is recorded (a passing check is a finding, not a non-event).
- **Any value at or over threshold** → that value is replaced by the re-derivation and gets a **fresh visual gate**. If it is one of the four eyeball-pinned tints (§13.5), `TestLightSurfaceTintsPinned` and `TestLightTintFillsArePerceptible` take the new pin from that gate.
- **If anything moves, §7.3's value tables in this specification are superseded by the theme files** rather than being re-written here. The files are the source of truth for values (§15.3); this spec's tables are the record of what was carried across, and a note pointing at the moved values is the honest form once they diverge.

**Flagged consequence:** if the check finds anything, shipped colours change, `TestLightSurfaceTintsPinned`'s eyeball-established pins move, and "Tokyo Night Dark/Light are just the existing values" (§7.3) stops holding exactly. **The built-in-set decision is conditional on this check.**

## 8. The theme setting — resolution & detection

### 8.1 On-disk shape — three flat string keys in `prefs.json`, plus a migration marker

`prefs.json` gains three theme keys and one migration marker alongside the existing `session_list_mode`:

```json
{
  "session_list_mode": "flat",
  "theme": "",
  "theme_light": "",
  "theme_dark": "",
  "theme_migrated": false
}
```

**`theme_migrated`** is not a theme setting — it is the one-shot gate for the `appearance` translation (§10.3). Its contract:

- **Type: boolean.** Not a version string or timestamp — the translation is a single event with no successor, so there is nothing to version.
- **Tolerant decode:** anything that is not literal `true` — absent, empty, corrupt, unrecognised — decodes to `false`. This keeps decode as dumb as the string keys: the failure direction is "run the translation again", and the translation is idempotent by §10.5, so a corrupt marker costs one redundant write rather than a wrong theme.
- **Written unconditionally on the first post-upgrade prefs load**, including when there is nothing to translate (`appearance` is `auto` or absent). Otherwise the condition is re-evaluated on every launch forever. §10.2's "Nothing" refers to the *theme keys* — the marker is still set.
- **Never participates in mutual exclusion** (§8.2). It is orthogonal to which theme keys are set, and clearing theme keys by hand does not clear it — that is precisely the property §10.3 exists to guarantee.

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

**Resolution order on the by-name path: the embedded set first, then the themes directory.** A nominated slug that names a built-in resolves to the built-in and **never reads the themes directory at all**. This is what makes §5.4's no-shadowing guarantee implementable on the path that matters — construction does not enumerate, so there is no collision to *detect* there; the safety property has to come from ordering. And construction is where the fallback resolves, which is the exact thing no-shadowing exists to protect.

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

**But a stale whole-file write can silently revert a theme.** Before this feature `prefs.json` had one field with a production writer. It now holds five independently-mutated fields written from three surfaces: instance A, constructed ten minutes ago, presses `s` and writes *its* in-memory prefs, silently reverting the theme instance B just committed. `AtomicWrite` does not help — this is a lost update, not a partial write.

**Every writer must read-modify-write:** re-read `prefs.json` immediately before writing, mutate only its own field(s), and write the merged result. Not novel — the project and hooks stores already do this for their own mutations.

**This includes the migration write** (§10.5). Its idempotence argument covers simultaneous cold launches computing the same value, but not the case this rule exists to close: an instance constructed against a pre-migration file flushing stale in-memory prefs and reverting a commit another instance made in between. The RMW re-read also lets the migration observe that another instance already set `theme_migrated` and skip its own write.

`prefs.json` continues to go through `fileutil.AtomicWrite`, so all three theme keys land in one atomic write and partial failure is impossible.

## 9. The slide-over panel

### 9.1 Shape and placement

A **full-height, right-edge, non-blanking overlay** with a **left border only** — deliberately *not* an inset bordered panel like the modals, so it reads as a slide-over rather than a floating dialog.

- The Sessions (or Projects) list stays fully visible behind it and re-themes live.
- Rendered over the existing overlay mechanism (`overlayHelpOnPreview` / the lipgloss v2 `Compositor` with real z-layers), which already ships.
- **Cursor row** uses the shipped selection treatment (`▌` + tint + white bold name), so the panel's list reads as the same kind of list as Sessions.
- **A vertical keymap footer** (`⏎ set theme` / `d set as dark` / `l set as light` / `esc close`) rather than Portal's horizontal footer row — a horizontal keymap does not fit a ~30-column panel, and the vertical form matches the help modal's key-column idiom.

**Header.** The label is **`Themes`**, rendered in `accent.mode` — the token whose role is signalling a distinct mode, which is what the panel is — followed by a one-row `border` rule, matching the Sessions section-header idiom minus the count. **No theme count** — noise at this list size. The header therefore costs **two rows**, which is what §9.8's minimum-height rule (header + footer + one row) resolves against.

**Message slot.** A single-row region directly above the vertical keymap footer, **not reserved when empty** — it appears and the list shrinks by one, the same way the main screen's notice band recomputes list height. It is a **single-slot arbiter** with two contenders, which can never be live at once because a confirm resolves before any write happens:

1. The **slot-from-constant confirm** (§9.2).
2. A **failed commit write** (§9.13).

At the minimum panel width the slot may wrap to two rows. It is not a list delegate, so wrapping costs nothing structurally.

**What the panel covers:** the right-hand column, where the footer's right-aligned `? help`, the right-side header hint, and session row meta live. **Accepted** — the theme is carried almost entirely by the *left* of the screen (session names, cursor bar, group headers, footer key glyphs), while the right edge is metadata. The overlay covers the least theme-informative part of the screen, which is exactly what a preview surface wants.

### 9.2 The interaction model — picker idiom, not settings panel

| Key | Effect | Panel |
|---|---|---|
| `↑` / `↓` | Move the cursor. **The app re-themes live behind the panel. Nothing is written.** | stays open |
| `Ctrl+↑` / `Ctrl+↓` | Page, per MV spec §12.2 | stays open |
| `Enter` | **Commits a constant** — writes `theme = <selection>`, clears both slots | **stays open** |
| `d` | **Commits the dark slot** — writes `theme_dark = <selection>`, clears the constant | stays open |
| `l` | **Commits the light slot** — writes `theme_light = <selection>`, clears the constant | stays open |
| `Esc` | **Closes.** Discards an uncommitted preview and renders the resolved persisted state | closes |

**Opening state: the cursor lands on the theme that is actually rendering, and opening previews nothing.**

- Under a **constant**, that is the constant's row.
- Under an **adaptive pair**, it is the row for the slot currently in force — the light slot in a light terminal, the dark slot otherwise. The other slot's row still carries its `● light`/`● dark` badge; only the cursor is singular.
- When the resolved theme is a **fallback** (§8.5), the cursor lands on the **fallback's** row, not on the persisted-but-broken one. The persisted row is unselectable (§9.5) and the arrows are specified to skip it, so parking the cursor there would put it somewhere navigation cannot return to — and it would show a row that is not what is on screen. The `●` still marks the persisted slug, which is exactly the split §9.5 draws: `●` is what is *set*, the cursor is what is *previewed*.

Because the cursor starts on what is already rendering, **opening the panel never changes the screen** and the mixed-mode flash fires only on deliberate navigation.

**Every write is an explicit keypress; nothing writes on close.** This eliminates the "applied but not persisted" state reachable under persist-on-close, where Portal dies with the panel open and the visually-applied theme was never written.

**`Enter` does not close.** If it did, a user who had just set both slots would press `Enter` to exit and thereby commit a constant, wiping the pair they just built. `Esc` is the only way out — one exit key, no dual-purpose keys, and the pair flow needs no special case.

**Cost accepted:** the common case ("pick one and go") is two keys rather than one.

**Committing to a non-active slot changes nothing on screen.** Previewing a light theme in a dark terminal and pressing `l` writes the light slot, but the resolved-active theme is still the dark slot. A commit is a **write, not a navigation** — the panel keeps previewing whatever the cursor is on; the display resolves from persisted state only on close.

Which sharpens `Esc` precisely: **`Esc` discards the preview and renders the resolved persisted state.** That equals "what you had before" only when nothing was committed. Commit slots and `Esc` lands on the newly-resolved theme, which is correct.

**Assigning a slot while a constant is set asks for confirmation first.** This is the one place a keypress described as inert can silently cost the user a setting they chose: on `"theme": "nord"`, pressing `l` clears the constant, the untouched dark slot falls back to the shipped default, and `Esc` in a dark terminal lands on `tokyo-night` rather than `nord`.

- `d`/`l` on a constant raises an **inline confirm in the panel's message slot** (§9.13) naming the constant that will be cleared.
- **`y` confirms** — the constant is cleared and the slot written, in one atomic prefs write. **Any other key cancels**, including `Esc`, which cancels the confirm without closing the panel.
- While the confirm is live it is **key-exclusive within the panel**: arrows, `Enter` and the other slot key are swallowed until it resolves. Nothing has been written yet, so there is no partial state to leave behind.
- It is **inline, not a modal** — the panel does not blank, and stacking a modal over an overlay is the shape §9.6 rejects for the Preview page.

**The reverse direction needs no confirm.** `Enter` on a theme while a pair is set clears both slots — but `Enter` visibly does what it says: you get the theme you are looking at, and it is the theme already previewing behind the panel. Nothing is surprising, so the confirm would be friction for its own sake. The asymmetry is the point: the confirm guards the case where the *resolved* theme changes as a side effect of a write the user was told is inert.

**The mixed-mode flash is the feature, not a defect.** Under split plus apply-on-arrow, arrowing past a light theme in a dark terminal flips the entire canvas near-white and back. Seeing a light theme as designed is precisely what live preview is for, and under the picker idiom it is transient and reversible. **List order is alphabetical by slug**; ordering same-mode themes first was proposed as a mitigation and **rejected** as unnecessary once the flash is accepted.

### 9.3 Mid-session constant → adaptive

Assigning a slot converts a constant-theme user to adaptive in-session, which needs a light/dark answer their launch deliberately never waited for.

**This dissolves.** `restore.go` issues the OSC 11 query from `Init` **regardless** — it needs the original background to restore on exit, independent of detection. The terminal's background is therefore already in hand; the detection decision only ever governed whether to **classify and use** it. Converting to adaptive mid-session starts using an answer that already arrived: no new query, no race, no gate.

The startup win survives intact — skipping the gate for constant users is about not **blocking first paint**, not about not asking. If the reply has not landed (requiring the panel to be opened within milliseconds of launch) it falls to **dark**, the same rule as everywhere else.

### 9.4 The list — files ∪ whatever prefs names

**Every `*.theme` file in the themes directory gets a row, plus every built-in, plus any slug named in `prefs.json` that has no file.**

- Enumerating every file means an invalid theme is *present and named*, so the user sees "there's my theme, it's registered, but it's invalid" rather than being completely in the dark about why it did not appear.
- A persisted slug with **no file** gets a row too — marked, unselectable, reason `not found`. Same shape as an invalid file: the user sees what is set and why it is not applying. This covers a deleted file, a renamed file, and a typo in `prefs.json`.
- A persisted slug **rejected by the charset check** (§8.6) before any file is sought gets a row with reason **`bad name`**, not `not found` — the reason maps to the actual failure, and each §6.2 reason has exactly one condition. Telling a user their file is missing when they typed an illegal name sends them looking in the wrong place.
- Applies **per-slot** under an adaptive pair with one dead or broken slug.

This is what makes the `●` marker always have something to sit on, so it keeps meaning "this is what's persisted" and nothing ever implies the fallback was chosen.

A **skipped-count line** (`⚠ 2 theme files skipped`) was the earlier design and is **superseded** by per-file rows: the row is present and named instead of a count that sends the user to another command to discover which file and why.

### 9.5 Row rendering and markers

**Valid rows** are selectable and render as an ordinary list row.

**Built-in rows are deliberately indistinguishable from drop-in rows** — a valid drop-in is simply selectable, sitting alphabetically among the built-ins with no visual distinction.

**Invalid rows** render in `text.faint` with `⚠` and a terse reason from §6.2 (`missing tokens`, `bad colour`, `bad syntax`, `bad name`, `reserved name`, `unreadable`, `not found`) — **glyph-backed** per MV spec §2.2 so it survives colourless. Full detail stays in doctor, where there is width to enumerate.

**A `bad name` row is labelled by its filename**, not a slug — it has none, because §5.2 rejects rather than normalises. The same applies to its position in the list: **ordering is alphabetical by slug, falling back to the filename for a row that has no slug.**

**A `reserved name` row is likewise labelled by its filename.** Its slug is valid, but it is *identical* to the built-in's — labelling by slug would put two rows reading `nord` in a list where §9.5 deliberately makes built-in and drop-in rows indistinguishable. `nord.theme` beside `nord` tells the user exactly which one is theirs, and it sorts adjacent to the built-in it collides with, which is where the explanation is most useful. The terse reason stays `reserved name`; doctor carries the sentence naming the conflict.

**A displayed slug that came from `prefs.json` is truncated and control-stripped before rendering.** It is hand-editable text drawn into a fixed-width frame, and §8.6 validates it before *use* as a path component but the unresolvable row still shows it — so a pasted newline, tab or ANSI escape would otherwise reach the panel.

**Row composition — one row per theme, always.** An invalid row never wraps to two lines: every list row is exactly one delegate line, which is the invariant `bubbles/list` pagination depends on and which §9.8's paging and the invalid-row skip both rest on. The elements compete for a fixed ~24–30 columns in this priority order:

1. **The `⚠` glyph** — always rendered on an invalid row. It is the invalidity signal and costs two columns.
2. **The `●` badge** (`● dark` / `● light` / bare `●`), right-aligned, when the row is a persisted slot. §9.4 exists so the marker always has a home, so the badge outranks the reason.
3. **The label** — slug, or filename for a `bad name`/`reserved name` row (above). Truncated with `…` to fill the space left, down to a floor of three visible characters plus the ellipsis.
4. **The terse reason**, right-aligned — **the first element dropped** when a badge competes for the same edge. `⚠` still says the row is invalid and doctor says why, which is exactly the split §6.3 draws.

Below the label's truncation floor the panel is already at §9.8's refuse threshold, so no further degradation rule is needed.

**An unreadable themes directory gets its own row** (§5.5) — a non-selectable `⚠ themes dir unreadable` pinned at the top of the list, above the themes. Without it every drop-in silently vanishes and the user sees only built-ins: the exact "completely in the dark" state §9.4 exists to prevent, in the surface it was chosen to prevent it, at the moment the user is standing there to pick a theme. **Built-in rows and persisted-slug rows still render beneath it** — the persisted rows especially, or a user with an unreadable directory loses the `●` entirely. Full detail stays in doctor (§12.2).

**Arrow keys skip invalid rows**, reusing the mechanism that already skips group-header rows on the Sessions list. The skip composes with paging exactly as the group-header skip already does.

**Markers — treatment A (inline slot badges):**

- The assigned rows carry a right-aligned **`● dark`** / **`● light`** badge.
- A **constant** theme carries a bare **`●`** with no slot word — with no slots there is nothing to qualify, and a label would be redundant with the marker.
- The two setting states never coexist on screen, so a row never carries both forms.

The `●` glyph is correct here: Portal **already repurposes** `●` for multi-select marking, where it indicates a marked row rather than a live session. `●` is Portal's general "marked / active" glyph, not an attached-only one. The two signals stay independent: **`●` marks assignment**, the **`▌` + tint cursor treatment marks browse position**.

Treatment **B** (a `dark → … / light → …` key-value block pinned under the panel header, with a plain list below) was rejected: more legible at a glance, but it puts theme names in a second place, pushes the list down, and with only two slots the badges say the same thing without the extra region. A also scales better as the library grows, since a badge stays attached to the row it describes.

**Accepted caveat:** with a very long list the assignments could scroll out of view. Judged fine — a user knows what they picked and can scroll to find it.

**Because the panel shows both slots' badges at all times**, a user can see what light is set to without having to remember whether they set it — including slots never touched, which hold shipped defaults (§8.3).

### 9.6 Opening the panel — `t`, on Sessions and Projects

**Key: `t`** — free on Sessions (taken there: `/ s x m k d e r ? Space Enter Esc` plus arrows) and the obvious mnemonic.

| Page | `t` |
|---|---|
| **Sessions** | Yes — the default page and the richest surface to preview against |
| **Projects** | Yes — theme is a *global* setting; refusing would make it feel page-scoped for no reason, and `t` is free there |
| **Preview** | **No.** The preview body is captured real ANSI scrollback that is deliberately out-of-theme, so live preview would only re-theme the frame chrome — a weak surface. It is also already a full-screen overlay, so the panel would stack an overlay on an overlay. |
| **Modals** | No — modals are key-exclusive by design |

**`t` needs the filter carve-out** — while `/` is focused it is a literal filter character, exactly as `s` already is.

### 9.7 Entry conditions and input routing

**Nothing blocks `t` except a modal, a pending burst, and `NO_COLOR`.**

- **Multi-select** — `t` opens, and the marked set is **unaffected**. The panel *nests* over the mode and `Esc` resolves innermost-first (closing the panel and returning to multi-select with selections intact), which is what MV spec §8.1 already specifies for modals. The multi-select banner sits in the notice band on the left, so it stays visible behind the panel. Previewing mid-selection is legitimate — the marked-row `●` is itself themed.
- **A pending burst** — `t` is swallowed. The burst input-locks the model (only `Ctrl-C`/`Esc` live) because it is mid-async-operation; swallowing is consistent with that lock rather than an exception to it.
- **Modals** — capture keystrokes, so no `t`, per existing key-exclusivity.
- **Sessions and Projects normal view** — always available.

**The panel is key-exclusive.** It owns arrows, `Enter`, `d`, `l` and `Esc`; everything else is swallowed. Pass-through is genuinely bad — `k` would kill the highlighted session while you pick a theme, `x` would swap to Projects with the panel open, `m` would start a multi-select behind it. Non-blanking and key-exclusive are not in tension: seeing the list without being able to drive it *is* the live-preview premise.

**Blocked-`t` feedback follows the existing precedent:** **flash** where the key *is* bound and the user could reasonably expect it to work (`NO_COLOR` on Sessions/Projects); **silent** where it is not bound at all (Preview, modals, burst-locked). That is exactly how `s` already behaves.

### 9.8 Geometry — degrade, don't refuse

**Width.** A fixed preferred width of ~24–30 columns (name, markers, slot indicators, border, padding), with long user slugs truncated `…` as Portal already does for session names. A fixed width is predictable to lay out against; content-driven width would make the panel jump around as the library changes.

**Narrow terminals degrade, they do not refuse.** MV spec §2.7's doctrine for space shortage is explicit: degrade, never break.

- The panel **shrinks** between a preferred and a minimum width as the terminal narrows — staged degradation, consistent with §2.7's existing width steps (drop right-side header hint → compact wordmark → truncate names).
- It **refuses only when even the minimum panel cannot render**, which is very narrow indeed — and then it flashes rather than opening a broken frame.
- **Exact thresholds are pinned at implementation**, as §2.7 already does for its own degradation steps.

The multi-select precedent (proactive block at entry) deliberately does **not** transfer: multi-select is blocked because of a capability *absence* — the terminal genuinely cannot spawn windows. A narrow terminal is a space *shortage*.

**Height.**

- **Overflow: scroll**, through the `bubbles/list` machinery, so `Ctrl+↑/↓` paging applies per MV spec §12.2. The invalid-row skip composes with paging exactly as the group-header skip already does.
- **Minimum height: the same degrade-then-refuse rule as width** — shrink the visible row count, and refuse with a flash only when header + footer + one row cannot fit.
- **Resize while open: degrade in place**, closing with a flash only if the terminal falls below the render floor. The entry condition is not the only check; §2.7's degradation is already per-dimension.

### 9.9 No unset — accepted

Every panel action *sets*: `Enter` sets a constant, `d`/`l` set a slot, nothing clears. So returning to the shipped pair after setting `theme_dark = nord` means explicitly setting `tokyo-night` — which resolves identically today but converts an **inherited default into a pin**, so a future change to the shipped default would no longer reach that user.

**Accepted and documented rather than fixed with a clear key.** It only bites if the shipped default changes, and `prefs.json` is hand-editable.

### 9.10 `NO_COLOR` — the panel is blocked

Under `NO_COLOR` Portal paints no canvas, imposes no hues, and renders glyph-backed on the terminal's native fg/bg. A theme panel previews nothing, its cursor tint and slot dots have no colour, and committing persists a choice with zero visible feedback.

**`t` is blocked under `NO_COLOR`, with a flash**, following the multi-select precedent exactly — proactively blocked at entry rather than letting the user walk into a dead end. **The `t` help row is filtered out while blocked**, via the same `sessionsHelpKeymap()` call-site filter that already drops the `m` row (the static descriptor is unchanged, so the keymap dispatch guard stays green).

This is deliberately the **opposite** call to the narrow-terminal one. Narrow is a *space shortage*, where §2.7 mandates degrade. `NO_COLOR` is a *capability absence* — there is no colour to theme, so the panel's purpose is inert rather than cramped.

**Counter recorded rather than buried:** someone may run `NO_COLOR` in one context and not another, so blocking prevents setting a theme that *would* apply elsewhere. Accepted, because the escape hatch is first-class — `prefs.json` is the documented hand-editable home for the theme setting, so three keys can be set by hand.

### 9.11 Everything re-themes, panel included

**The slide-over's own chrome re-themes with the previewed theme. No exceptions.**

1. It is the honest preview — the panel is part of what the theme paints, so a fixed panel shows a theme that cannot be fully judged.
2. It avoids a **permanent exception in the render layer** — a surface that deliberately ignores the active theme is precisely the shape the swap-and-diff guard exists to catch, so the alternative would mean carving out the one test protecting against accidental carve-outs.
3. The unreadable-panel risk is smaller than it looks, because **`Esc` is a keypress, not a visible affordance** — no need to read the hint to close the panel. The picker idiom does the rest.

**Residue recorded rather than hidden:** since a drop-in need only be *valid*, not good, a legal-but-awful theme can render the panel's own list unreadable while the user is standing on it. A user can only get *stuck* there by explicitly committing one, and recovery is then editing `prefs.json` rather than anything in the UI. Since a drop-in is by decision the user's own creation and only they can reach this state, that is judged proportionate — but it is a real edge.

### 9.12 The panel's keymap is descriptor-governed

The panel introduces `Enter`, `d`, `l` and `Esc` through a bespoke vertical footer outside `keymap.go` — a second place a key label can go stale, the very drift class guarded elsewhere.

- **The panel's keys live in the keymap descriptor as a panel scope.**
- **Its vertical footer renders from the descriptor.**
- **`keymap_dispatch_guard_test` covers them.**

### 9.13 A failed commit write

A failed write on `Enter`/`d`/`l`:

- **Reports in the panel's message slot** (§9.1) — `⚠` plus a terse statement that the theme could not be saved, glyph-backed per Portal's convention. It **persists until the next keypress** rather than timing out like a transient flash: it reports a state the user must act on, and a message that vanishes on its own can be missed in the surface where the only other feedback is the `●` deliberately *not* moving.
- **Keeps the theme applied in memory.**
- **Does not move the `●`** — the marker means "what is persisted" and would be lying if it moved.

This recreates "applied but not persisted", but as a *reported* state rather than a silent one, which is the distinction the picker idiom was buying.

### 9.14 Reference frames

Three Paper artboards are the forward-looking reference for this panel, all built on the canonical `Sessions — Modern Vivid v2` frame so they inherit the shipped MV conventions:

- `Theme slide-over — A (inline slot badges)` — the adaptive-pair state
- `Theme slide-over — A (constant set, previewing another)` — a constant `●` on one row while the cursor sits on a different theme
- `Theme slide-over — B (assignment header)` — the **rejected** treatment, retained as the record of what was weighed

The constant frame completes the panel's specification because the two setting states never coexist on screen. It is the picker idiom made visible: the `●` is what is *persisted*, the cursor + canvas is what is *previewed*, and `Esc` would restore the marked one.

**Caution when reading any Paper frame:** the mocks use **per-frame literal hexes**, so the same token can carry different values across frames. The frames are reference, never truth.

## 10. Upgrade path from `appearance`

### 10.1 The problem

Real installs hold `"appearance": "dark"` or `"light"` today — the README currently *recommends* pinning it. Deleting `prefs.Appearance` makes that field unknown, so tolerant decode silently ignores it, and a user who deliberately pinned `dark` on a light terminal upgrades into the shipped adaptive pair and **silently gets a light Portal** with nothing explaining why. That is the worst outcome for precisely the group who expressed a preference.

### 10.2 The translation

The mapping is exact, which makes the fix cheap: `appearance: dark` meant "always dark regardless of terminal", and the new equivalent is a **constant** theme.

| Existing `appearance` | Action |
|---|---|
| `dark` | Write `"theme": "tokyo-night"` |
| `light` | Write `"theme": "tokyo-night-day"` |
| `auto` | Nothing — ignoring it lands exactly on the adaptive default, which is what `auto` meant |
| absent | Nothing |

Intent is preserved precisely rather than approximately: a pinned mode becomes a pinned theme, and detection stays off for them just as it was.

Portal has the precedent — `migrateConfigFile` performs a one-shot move from the old macOS config path.

Rejected: accepting the silent flip as cosmetic and one keypress to fix. Wrong when the affected users are exactly those who set a preference, and when the translation is this small and this exact.

### 10.3 The trigger is an explicit marker

**The translation is gated on an explicit `theme_migrated` marker in `prefs.json`, not on the absence of theme keys.**

Gating on absence would be re-armable, and it composes badly with the "no unset" acceptance (§9.9), whose documented escape hatch is to hand-edit `prefs.json`: an upgraded user who deletes their theme keys to return to the shipped adaptive pair would get **silently re-translated and re-pinned** on the next launch — Portal reinstating exactly what they just undid.

With an explicit marker: `appearance` is retained, deleting theme keys does nothing, and the trigger fires **exactly once ever**.

### 10.4 `appearance` is retained, not dropped

**The translation adds the theme keys and leaves `appearance` in place.**

Portal ships via Homebrew where reverting a version is routine, and the protected population is exactly those who pinned `appearance`. Dropping the field would mean that post-translation their pin is gone, an older binary reads nothing, falls to `auto`, and resumes detecting — precisely what the translation prevented, displaced in time.

Retaining it is inert to the new binary and still meaningful to an old one, and it removes a schema mutation entirely (which also removes the question of who owns performing the deletion).

**Accepted:** the retained `appearance` is a **frozen legacy value** and is **not** kept in sync with later panel commits. A downgraded binary honours the user's old pin rather than their current choice — which is the most a binary with no concept of themes could do.

### 10.5 Ownership and write-path robustness

**`cmd/config.go`'s `loadPrefsStore` owns the translation.** Three decided constraints meet here: `prefs` is a deliberate leaf that must not import `internal/log`; the translation happens at prefs load; the `theme` log component records it. `loadPrefsStore` already owns prefs path resolution and the migrate breadcrumb for every other config file, and is not a leaf, so it can log. **`prefs` stays dumb.**

**`cmd/config.go` also exposes a non-migrating read variant, which every bootstrap-exempt command uses.** `portal doctor` must read `prefs.json` to report an unresolvable theme (§12.2), and `portal theme export` may resolve a persisted slug — but doctor's contract is that it **heals nothing on the read-only path**, and a one-shot config mutation as a side effect of running a diagnosis breaks that. Splitting the read from the migration keeps doctor's "read-only" claim literally true and keeps `export` side-effect-free, without relocating ownership of the translation away from `loadPrefsStore` (which is where the logging constraint puts it).

**The migration therefore runs only where a TUI is constructed** — which is also the only place its result is used, since the exec path constructs no TUI and reads no prefs.

**Separate *computing* from *persisting*.** At prefs load, read `appearance`, compute the translated theme, and **use it in memory immediately**; the write is **best-effort and non-blocking**. A failed write means Portal renders the correct theme this launch and retries next launch (the condition is still true), so it can never flip the user to the wrong theme — which was the translation's entire purpose.

**Concurrency is a non-issue, for a stateable reason:** several burst-launched instances hitting the condition simultaneously all compute **the same value from the same input**, so the write is idempotent and last-write-wins is harmless. That is what makes it safe where a general read-modify-write would not be. It also never runs on the exec path, which constructs no TUI and reads no prefs.

The translation emits `theme: appearance migrated` (INFO, one-shot) — see §12.3.

## 11. Live-swap mechanics

### 11.1 Speed is a non-issue

The cheap path already exists and already excludes the expensive one:

- **Restyle** — `applyCanvasMode` swaps the delegate and re-points the cached style structs `bubbles/list` holds. O(1), no I/O, no list content touched. It is already exercised in production: it is what runs when the OSC 11 reply lands after first paint, and it performs exactly the mid-session restyle a theme swap needs.
- **Rebuild** — `rebuildSessionList` re-derives the item list and, in grouped modes, runs the lazy dir-resolution pass with its per-session tmux pane reads (the known ~0.5s By-Project cost at ~38 sessions).

**`applyCanvasMode` does not call `rebuildSessionList`.** Nothing heavy is on the theme-swap path, so no deferral mechanism is needed.

The premise that the re-render is the cost is also wrong: Bubble Tea rebuilds the whole view string on *every* keypress regardless, diffs it, and writes only changed cells — holding the down arrow in the sessions list already does this dozens of times a second. **A theme swap costs one ordinary keypress plus the style re-point.**

**"Bake in on exit" is rejected**: nothing is left un-baked, and deferring work to panel close would create a visible discontinuity at the one moment that should be seamless.

### 11.2 The real risk is completeness

Threading the theme (§3.4) fixes most of this: anything taking the theme as a parameter re-derives per frame. What remains is the **cached styles Portal does not own** — `bubbles/list`'s help styles, pagination dots, TitleBar, and both filter inputs — which are assigned once. That list is hand-maintained with no guard test, unlike the colour-literal rule which has an AST glob guard. Miss a site and the element silently keeps the previous theme's colours until something else re-renders it.

**Two known offenders are fixed outright**, not guarded around:

1. `pagepreview.go` copies a `Token` at **package init**, so it would never see a swap. The package-scope copy goes.
2. `canvasHexFor` references `theme.MV` directly — a hardcoded MV reference outside the token render path. It becomes theme-agnostic.

Fixing them does not make the guard redundant; **the guard is what stops them returning** (§13.4).

**These two are what was *found*, not the boundary of the class.** Init-time copies of *derived styles* (a style struct built from a token at package scope, rather than the token itself) were never swept for at all. Implementation must run that sweep rather than treating the two named fixes as closing the category. The swap-and-diff guard is the safety net that catches whatever the sweep misses — but a sweep that is never run leaves the guard doing work a five-minute grep would have done, and leaves the residue undocumented.

### 11.3 OSC 11 re-emission

- **No per-keystroke churn.** Bubble Tea v2 **diffs** the view's background colour and emits only on change, so hovering N themes emits OSC 11 exactly once per *distinct* canvas landed on — the minimum the feature requires. The declarative per-frame `BackgroundColor` assignment is not a per-frame write.
- **The echo guard needs no new race handling.** It exists because the startup OSC 11 *query reply* can race Portal's own canvas set. The query is issued once from `Init`; a later theme switch issues no new query, so it creates no new race. The guard only ever needs to compare against the canvas active during the *startup* window.

### 11.4 The exit-time canvas restore

`RestoreTerminalBackground` currently derives its comparison value *at exit* from `m.canvasMode` via `canvasHexFor`, which reads `theme.MV.Canvas` directly. Under a switchable theme that is wrong: it would compare against the *active* theme's canvas rather than the one in force during the startup window.

**Required change:**

- **Capture and retain the startup canvas hex as model state**, and anchor `RestoreTerminalBackground`'s comparison to it.
- **Make `canvasHexFor` theme-agnostic** — no `theme.MV` reference.

This is the mechanic carrying an explicit *"do **not** drop this guard"* warning, and the swap-and-diff guard structurally cannot cover it (it scans rendered fixture output, and this is an exit-time OSC 11 write). **It therefore needs its own named verification** — a direct unit test on `RestoreTerminalBackground`, driven without fixtures, asserting it compares against the retained startup canvas and not the active theme's, **including the case where a theme was committed mid-session** (§13.4).

The stakes are why: this is the one path where a mistake re-sticks a colour in the user's terminal **after Portal exits**.

## 12. Non-TUI surfaces, logging & docs

### 12.1 `portal theme export <slug>`

Writes the named theme to **stdout** in canonical form, so the full drop-in workflow is:

```
portal theme export nord > ~/.config/portal/themes/nord-lee.theme
```

This closes a structural gap. *"Copy a built-in and edit it"* carries **two** decisions — it is the pro that justified `go:embed` (§7.1), and the deciding factor that rejected merge-over-a-base (§4.5, full replacement is only cheap if copying is cheap). But built-ins live inside the binary, `portal theme list` and `--theme` are ruled out, and an absent `themes/` directory is deliberately silent and never seeded — so without `export` the only route was finding the file on GitHub, which was never named as the workflow and is unavailable offline.

**Command surface:**

| | |
|---|---|
| **Bootstrap-exempt** | Added to `skipTmuxCheck`. Printing a file must not start a tmux server, ensure the saver, or run restore. |
| **Slug domain** | **Built-ins *and* drop-ins.** Resolving both makes export a diagnosis tool — "show me what Portal parsed" — not just an on-ramp. |
| **Invalid drop-in** | Refused, with its reason on **stderr** and a **non-zero exit**. Doctor's advisory-vs-health distinction (§12.2) is doctor's own contract and does not extend here. |
| **Unknown slug** | Same — reason on stderr, non-zero exit. |
| **Verb group** | The `theme` group has only `export`. A one-member group, noted deliberately. |

**Output is the file's bytes, comments included** — not a re-serialisation of the parsed `Theme`.

The theme is still parsed and validated first (that is what refuses an invalid drop-in and an unknown slug), but what is written is the source file. Re-serialising would **drop every `#` comment**, and comments are not decoration here: they carry the attribution header the file format was chosen for (§4.1) and the eyeball-pin derivation notes that are the only surviving record of a non-numeric judgement (§7.1). A user running `portal theme export tokyo-night-day > …` to start a light theme would otherwise get a file stripped of exactly the notes explaining its pinned tints.

Byte-faithful output also makes the diagnosis framing honest — "show me the file Portal read" — and needs no separate decision on key ordering or trailing newline, since the shipped file already parses.

**This partially reverses the YAGNI ruling on theme CLI verbs, deliberately.** That ruling was about *listing* and *selecting* — both genuinely redundant with the panel. Export is redundant with nothing.

Considered and rejected: a panel key duplicating the highlighted theme into `themes/` as `<slug>-copy.theme`. Better placed (on-ramp at the point of intent) but it adds a key and makes the TUI write files; the verb is simpler, scriptable, and works when the panel is unavailable.

`docs/theming.md` additionally carries a complete copy-pasteable example theme for the no-terminal case.

### 12.2 `portal doctor` — a read-only theme health line

Doctor is Portal's established config-health surface, with full terminal width to enumerate per-file reasons on demand. It:

- **Scans the themes directory** and reports any file failing validity, with the reason and the specific token/line/key.
- **Reports when a persisted theme name no longer resolves.**
- **Reports an unreadable themes directory** (or a regular file where a directory belongs). An *absent* directory is silent (§5.5).

**Read-only, with no `--fix` action.** Doctor can prune a stale hook entry; it cannot repair someone's colours. Reading `prefs.json` to report an unresolvable theme goes through the **non-migrating** prefs read (§10.5), so running doctor never triggers the one-shot `appearance` translation — the read-only claim holds literally.

**Theme lines are advisory and do NOT drive the exit code — this amends doctor's contract.** Doctor's contract is a scriptable exit code, 0 iff all checks pass. Because there is deliberately no repair path, a failing theme line would go **permanently** non-zero until someone hand-edits a file — unlike every other check, which is either `--fix`-repairable or indicates genuine runtime breakage. The exit code exists as a signal about the **resurrection machinery** — daemon alive, hooks registered, state sane. A stray junk file in `themes/` is not that: Portal is working, it simply did not list one theme. Letting it hold the diagnostic red means an automated health check fires about the daemon because someone left a half-written palette lying around.

So doctor gains **two classes of line**:

| Class | Marker | Drives exit code |
|---|---|---|
| **Portal-health checks** | existing pass/fail markers | Yes, as today |
| **User-content diagnostics** | **`⚠`** — Portal's established warning glyph (MV §2.2, glyph-backed so it survives colourless) | **No** |

Theme validity is the first member of the second class. **Doctor's closing summary distinguishes the two counts** — e.g. *"N checks passed · 2 advisories"* — so the exit code's meaning is legible without reading the contract.

Rejected: failing the exit code on the grounds that a user who dropped a broken file into a Portal-read directory should get a loud persistent signal. They do — via the panel row and the doctor line — without conscripting a signal that means something else.

### 12.3 A new `theme` log component

Portal's log component taxonomy is **closed and spec-governed** — components are never invented at a call site. **This feature adds a `theme` component via spec amendment**, with direct precedent: `spawn` and `resolve` were both added by the features that needed them.

What distinguishes it from `prefs` and `terminals` (both deliberately outside the vocabulary) is that those are **dumb stores with no runtime behaviour**, whereas the theme loader has parse/validate/fallback *outcomes*.

**Event catalogue:**

| Event | Level | Cadence |
|---|---|---|
| `theme: loaded` | INFO | At TUI construction. Resolved slug(s) only — **no count** (nothing is enumerated at construction). |
| `theme: enumerated` | INFO | At panel open. Carries `count` and `rejected`. |
| `theme: rejected` | WARN | One per rejected file, **deduplicated per process** — a given slug+reason logs once, so five panel opens (enumeration re-reads on every open, §5.8) do not produce five identical WARN sets. |
| `theme: directory unusable` | WARN | Per enumeration where the themes directory is unreadable, or a regular file sits where a directory belongs (§5.5). Carries `path` and `reason`. An *absent* directory emits nothing. |
| `theme: fallback applied` | WARN | Per fallback |
| `theme: appearance migrated` | INFO | One-shot |
| `theme: commit failed` | WARN | Per failed write |

**Attr keys:** `slug`, `slot`, `reason`, `path`, `token`, `count`, `rejected`.

Both additions close holes in the closed declaration rather than extending it by preference: `rejected` was already used by `theme: enumerated` without being declared, and §5.5's required log entry for an unusable directory had no event that fits (`theme: rejected` is per-*file*, and §6.2's `unreadable` reason is defined as "the file could not be read").

Rejections are **WARN**, not INFO: doctor treats them as advisory for *exit-code* purposes, but "your config did not work" is a warning in a log.

**Why the log earns its place:** a TUI launch that rejects a theme should leave a **passive** record. The panel's row is only visible if the panel is opened; doctor must be invoked. The log is the only trail that exists without the user going looking.

**Correction to a premise, recorded so it is not re-derived:** the exec path (`portal open <target>`) constructs no TUI, so under lazy discovery the loader **never runs there** — nothing themed is rendered and there is no failure to surface or record on that path at all. Both the doctor line and the log component earn their places on other grounds (above). **And a win worth recording explicitly: on the path Portal is most careful to keep free of cost, this feature adds nothing at all.**

### 12.4 `docs/theming.md`

A new user-facing doc, following the `docs/custom-terminals.md` precedent (a user-authored config file with its own doc).

**Contents:**

- **The 19-token vocabulary with each role's meaning** — the substance of §2.5. `docs/theming.md` is **the source of truth for the public contract.**
- **The text ramp's weight ordering** — the sole record of it, since file ordering carries nothing (§2.7).
- **The file format** — lexical rules, value domain, the closed key set.
- **A complete copy-pasteable example theme** (also the no-terminal on-ramp).
- **The two-slot config** — `theme` / `theme_light` / `theme_dark`, constant vs adaptive, mutual exclusion, the `theme`-wins hand-edit rule.
- **The reserved built-in slugs.**
- **Attribution for ported palettes** — source and link, plus the Nord corrections. Attribution lives in the repo and README, **explicitly not in the UI** (no credits screen, nothing in the slide-over).

**Attribution and licensing are deliberately not pursued further.** No per-theme licence line, no "(adapted)" naming convention, no PR contribution requirement. Ported palettes keep their own names. Recorded so a future reader does not mistake the omission for an oversight.

**`docs/theming.md` gets a guard** (§13.5) — it is now the sole record of the ramp ordering and role meanings, with nothing otherwise keeping it honest.

### 12.5 README and CHANGELOG

`appearance` is described in `README.md` at four places, including a paragraph recommending users pin it *"when auto-detection misfires (for example under tmux passthrough)"*.

**That paragraph comes out with the setting** — and the advice is obsolete twice over, since the premise was probably never true in the first place (§8.7).

README gains the theme setting in its place, pointing at `docs/theming.md`.

**`CLAUDE.md` needs correcting too:** it currently describes `testdata/vhs/` as committed reference PNGs forming a visual-verification harness, which reads as a durable asset. It is not (§13.2).

## 13. Capture harness & test strategy

### 13.1 Why `capturetool` is load-bearing

**Portal cannot be run from a temporary build to check a visual change.** A scratch build interferes with the live system — it disturbs the running daemon, its bootstrap sequence touches real state, and sandboxing does not fully contain it.

So `capturetool` is not a convenience; it is the **only viable route** to seeing a visual change before release. This also endorses the fixtures' deliberate shallowness: they do just enough to visualise what is meant to be visualised. **Fixtures are about look, not behaviour** — they need not be functionally complete.

**Two mechanisms, two audiences — both stay:**

| Mechanism | Audience | Why |
|---|---|---|
| **A producible PNG per fixture** | The **agent** | During the agentic implementation loop the implementer captures a screen, looks at it, and assesses its own work; the reviewer does the same. **Without a producible PNG the agent cannot see what it built**, and every task ends up hand-corrected — the exact failure mode this tooling exists to prevent. |
| **`capturetool --fixture`** | **The human** | Loaded in a real terminal at the human-in-the-loop gate and judged as the real thing — Portal's look and feel, without running Portal. |

**The workflow this serves:** implement → capture → agent self-assesses → reviewer → converge → *then* the human gate.

### 13.2 Committed reference PNGs were scaffolding, not an asset

The committed reference PNGs were never meant to persist — they existed so the redesign could be watched coming to life during implementation. **There is no visual-regression obligation**, so there is no themes × fixtures matrix problem: three built-ins do not multiply 43 committed images into 129.

**Retention rule, drawn now:**

- **Everything that exists today as an image or tape is deleted** — the committed reference PNGs and the VHS tapes that produce them. They could not survive the token rename and the theme split without a full recapture in any case.
- **From this feature forward, captures are created as work proceeds, committed while they are being collaborated on, and cleared out after sign-off** so they do not live in the repository forever.
- **Cleaning up is not this feature's job** and is not done as we go.

**The deletion covers images and tapes, NOT fixtures.** The Go fixture *definitions* in `internal/capture` and the harness itself are **permanent** — the swap-and-diff guard drives the fixture renderer and its coverage assertion needs the fixture set to exist. "Cleared out after sign-off" likewise means the images, not the fixtures.

### 13.3 Harness changes required

- **`capturetool` and `internal/capture` survive and are open for edit.** Whatever the tool needs to work with the new system is in scope for this feature — no separate redevelopment work unit.
- **`tui.Build` takes a *theme* where it takes a `prefs.Appearance` today** — the exact injection mechanism this work removes. Without this the harness can only ever render the compiled-in default.
- **`capturetool` gains a `--theme` flag, replacing `--appearance`.** `--theme` accepts a built-in slug **and an explicit path to a real theme file**. An explicit path from a flag is an **input, not config discovery**, so the `internal/capture` no-real-config import guard's invariant is preserved (no XDG lookup, no prefs read). This matters disproportionately: it is the only visual-verification route for someone authoring a drop-in.
- **`--appearance` is removed, not kept alongside.** It exists today (`dark|light`, resolving to a pinned `prefs.Appearance`), and its entire backing mechanism — `prefs.Appearance` and `WithAppearance` — is deleted by §8.8. There is no mode left to pin; a theme *is* the mode.
- **The contrast-validation swatch fixture is re-pointed to `--theme` too.** `capturetool` carries a standalone labelled-tint swatch branch (the MV §16.5 lock-in/bail surface) which deliberately does not route through `tui.Build` and is driven by `--appearance` today. It is the surface that satisfies the human eyeball gate §7.5 and §13.5 require for a new light theme's pinned tints, so it must take a theme like everything else.
- **Direct PNG output from `capturetool` is required, not an optimisation.** The retention decision deletes the tapes that made PNG production work, while the harness requirement is that every fixture can produce a PNG. VHS is retained only if a gif is ever wanted for motion.
- **New fixtures are added for the slide-over** — the adaptive-pair state, the constant-while-previewing state, an invalid-theme row, and the narrow degraded panel — so the panel is visible during implementation rather than at release.

### 13.4 The swap-and-diff completeness guard

**What it is:** render a screen under theme A, switch to theme B, render again, and scan the second output for any colour value belonging to theme A. A survivor means some element never got the new theme — the "assert no stale data survived the invalidation" trick applied to rendered output rather than a cache. It exists because the cached styles `bubbles/list` holds cannot reliably be found by reading code (§11.2).

This is a **behavioural** guard, not a structural one, deliberately. It catches *any* missed site — including ones added years later — without anyone having to remember a rule. A structural guard would have to recognise "this is a cached style" in the AST, which is not mechanically well-defined.

**It uses two synthetic themes constructed inside the test, all 38 values deliberately unique** — none repeated within a theme or across the pair. Using two shipped themes has two failure modes, and both are a matter of time:

- A hex both palettes happen to set identically survives the swap *legitimately*, so the test fails permanently for a non-bug.
- Worse and silent: a token with the *same* value in both themes renders identically before and after, so the test cannot tell whether that site updated — it passes either way and the site is uncovered with no signal.

Synthetic themes make coincidence impossible, cover every token site genuinely, and mean nothing done to the shipped palettes can break or blind the guard.

**Three assertions:**

1. **No theme-A value survives** in the post-swap output.
2. **Every expected theme-B value is present** — catching a site that renders *nothing* rather than merely stale. This is a **union across fixtures**, not per fixture: no single screen renders all 19 roles.
3. **Every token is exercised by at least one fixture.** The union in (2) is only complete if every token appears on *some* fixture, and the at-risk ones are the transient states (`bg.attention` / `text.on-attention`, `accent.mode`, `state.destructive`, `text.on-selection`). Making this an assertion of the guard means a token with no fixture **fails the test** and someone adds a fixture, rather than the guard being silently blind at precisely the sites it exists to protect.

**Lane: unit.** It renders only through the offline harness — no tmux server, no daemon, no built binary.

**Colourless fixtures are excluded.** A colourless render contains no theme hexes, so there is nothing to diff — inclusion would be meaningless rather than merely redundant.

**The two known offenders (§11.2) stay fixed *and* guarded.** Fixing `pagepreview.go`'s package-init `Token` copy does not make the guard redundant; the guard is what stops it returning.

**Not covered by this guard, needing its own test:** the exit-time canvas restore (§11.4). The guard scans *rendered fixture output*, so it structurally cannot cover an OSC 11 write that happens after the last render.

### 13.5 Contrast checking

**Floor-check enrolment is automatic.** The floor test **auto-enumerates the embedded set**, so a new built-in is checked by default.

**Plus a light/dark table**, needed because the light surface tints are not numerically checkable (light-tint-on-light-canvas is numeric-insufficient — hence `TestLightSurfaceTintsPinned`), so the carve-out must apply to light themes only.

**The eyeball-pinned set is four tokens, not three.** `TestLightSurfaceTintsPinned` today pins five entries — `bg.selection`, `bg.warning`, `bg.track`, `border.separator`, `border.footer` — which is **four distinct tokens after the §2.2 border consolidation**: `bg.selection`, `bg.attention`, `bg.subtle`, `border`. Each carries a matching `pinned — derivation … eyeball-confirmed` comment in `theme.go`, and `TestLightTintFillsArePerceptible` covers the same set. The count is load-bearing: it determines which pin notes move into the theme files as `#` comments (§7.1), and how wide the light-only carve-out has to be. **All four.**

- **It is the *test* that needs to know, not the product.** A test table is allowed to know things the runtime does not — the vocabulary stays variant-free (§4.7) and the table names which built-ins are light.
- **The table carries an assertion that every embedded theme appears in it.** A forgotten entry fails the suite rather than silently shipping a Portal-endorsed theme nobody checked — or measuring a light theme against a dark reference.

**`contrast_test.go` resolves its reference background from the theme.** It currently measures against two hardcoded canvases; under split each theme carries its own `canvas` token, so the reference comes from the theme rather than from a constant.

**`docs/theming.md` gets a guard.** It is now the sole record of the ramp's weight ordering and the 19 roles' meanings, with nothing keeping it honest — and this feature found the MV spec's "2-tone border" claim stale against the implementation purely by chance. Same drift class, same subsystem. **A test parses the doc's token table and compares the name set against `Theme.All()`** — cheap, and matching the codebase's existing guard idiom. The doc's copy-pasteable example theme is covered by the same guard: it must parse and contain all 19 keys, so it is not a fourth unguarded copy of the vocabulary.

### 13.6 Guard-test reshape

| Test | Change |
|---|---|
| **`TestMVTokenCount`** | Moves 20 → 19, and its meaning shifts from "MV has 20 tokens" to "**the vocabulary is 19**". |
| **`TestMVDarkVariantsPinned`** | **Deleted.** Once themes are data files whose values are their own source of truth, an exact-hex pin in a Go test is a change-detector duplicating the file. The contrast floor test is the real guard for bundled themes. |
| **`TestLightSurfaceTintsPinned`** | **Survives, and becomes per-light-theme.** The four light surface tints (§13.5) are not numerically checkable, so for those the exact-value pin is the only guard. They keep their pin, and the *why* moves into the theme file as a `#` comment — which the flat format supports. **The format decision is what makes deleting the Go-side erratum comments safe rather than lossy.** Pins for any new light theme are established by human eyeball at a visual gate. |
| **`TestEachTokenCarriesLightVariant`** (`theme_test.go`) | **Deleted.** It asserts the `ColorFor(Light) ≠ ColorFor(Dark)` resolver seam, which cannot compile once `Token` is `{Name, Value}` and `ColorFor` is gone. |
| **`TestEveryTokenHasLightVariant`** (`contrast_test.go`) | **Deleted.** Same fate — it asserts every token carries a populated, parseable `Light` hex. Its *parseability* half is subsumed by the embedded-set validity test (§7.6), which checks every value in every shipped theme. |
| **`TestLightTintFillsArePerceptible`** | **Survives, and becomes per-light-theme**, alongside `TestLightSurfaceTintsPinned`. It covers the same four tints and takes the same light/dark table membership (§13.5); its ≥1.1 fill floor resolves its reference background from the theme rather than the hardcoded light canvas. |
| **Embedded-set validity + fallback-slug resolution** | **New** (§7.6). |
| **Swap-and-diff completeness guard** | **New** (§13.4). |
| **`RestoreTerminalBackground` anchor test** | **New** (§11.4). |
| **`docs/theming.md` token-table guard** | **New** (§13.5). |
| **`keymap_dispatch_guard_test`** | Extended to cover the panel scope (§9.12). |
| **Colour-literal guard** | Unchanged in mechanism; continues to exclude the `theme` subpackage. |

## 14. Footer keymap revision

This is a change to the existing MV spec §12.2 keymap revision, driven by discoverability: the feature would otherwise be near-invisible — `--theme` and `portal theme list` ruled out, the themes directory silent and never seeded, built-in rows indistinguishable from drop-ins, the reserved-slug set invisible, and no active-theme indicator when the panel is closed. Discoverability would rest entirely on `?` help and `docs/theming.md`.

### 14.1 The change

- **Drop `↑↓ navigate` from the footer.** Arrows in a list are a given, and arrows are the entry that genuinely deserves non-core status — still listed in `?` help. This is the distinction (core vs non-core) applied to the right thing.
- **Promote both `t` and `m` to core**, so both appear in the footer as well as `?` help.

### 14.2 Decided footers

- **Sessions** — `⏎ attach · / filter · ␣ preview · s switch view · x projects · t theme · m multi` + right-aligned `? help`
- **Projects** — `⏎ new session · x sessions · e edit · / filter · t theme` + right-aligned `? help`

### 14.3 Width, measured rather than assumed

Dropping `↑↓ navigate` frees ~93px; `t theme` costs ~61px and `m multi-select` ~116px, netting **+84px** against an 89px spacer at the reference mock's 86-column width — it fits with ~5px spare and no headroom.

**The label is therefore `m multi`, not `m multi-select`**, buying back ~47px.

The Projects footer was verified against the `Projects (MV)` frame: it carries no `navigate` today and has ~322px of slack before `? help`.

**The footer is filtered in lockstep with `?` help.** A blocked `t` (under `NO_COLOR`, §9.10) or a blocked `m` (unsupported terminal) is absent from **both** surfaces, through the same call-site filter. Advertising a key in the footer that only produces a blocked flash is the dead-end the proactive block exists to prevent, and help/footer disagreeing about the same key is a live inconsistency.

**Consequence for the width budget:** §14.3's arithmetic is measured with both entries present, which is the tight case. Filtering only ever removes entries, so every blocked-state footer is strictly narrower and no separate budget is needed.

---

## 15. Spec amendments this feature carries

The Modern Vivid specification is amended by this feature's work. Named explicitly so none is missed:

### 15.1 The three named amendments

1. **MV spec §12.2 — the keymap revision.** The footer changes of §14 above.
2. **The `portal doctor` contract.** Two classes of line — Portal-health checks driving the exit code, user-content diagnostics carrying `⚠` and not driving it — plus a closing summary distinguishing the counts (§12.2).
3. **The log-component vocabulary.** A new `theme` component with its own attr keys and event catalogue (§12.3).

### 15.2 The MV vocabulary sections

Also amended, as spec-phase work rather than left unowned:

| Section | Amendment |
|---|---|
| **§2.1 / §2.9** | The token renames (§2.4), 20 → 19, the dropped `border.footer`. |
| **§2.9** | The removal of `Token.ColorFor` and `theme.Mode`; the two-hardcoded-canvas framing goes — each theme carries its own `canvas` token and contrast is measured against it. |
| **§8.1** | The stale "2-tone border (`border.separator` + `border.footer`)" claim, which the implementation already dropped. |

### 15.3 Where the vocabulary lives after this feature

The 19 roles are described in four places. Their standing is not equal:

| Location | Standing |
|---|---|
| **`docs/theming.md`** | **The source of truth for the public contract** — the 19 roles, their meanings, the ramp's weight ordering. Guarded (§13.5). |
| **The MV spec** | Amended per §15.2. Design rationale and contrast rules. |
| **The doc's example theme** | Covered by the same guard as the doc — must parse and contain all 19 keys, so it is not an unguarded fourth copy. |
| **The embedded `.theme` files** | The values themselves. Guarded by the embedded-set validity test (§7.6). |

### 15.4 The MV Paper frames are historical, not specification

**Modern Vivid is already implemented, so the code is the source of truth.** The MV Paper frames are historical reference from that feature's design phase; a footer in them that no longer matches (e.g. still showing `↑↓ navigate`) is **not a defect** and is not worth updating.

Only the **new** frames are forward-looking reference material, because they describe surfaces that do not exist yet:

- `Theme slide-over — A (inline slot badges)`
- `Theme slide-over — A (constant set, previewing another)`
- `Theme slide-over — B (assignment header)` (the rejected treatment)
- `Sessions — Nord (port)`
- `Kill Modal — Nord (state.destructive #DD8188)`
- `Sessions — Nord inline flash (bg.attention #3D4046)`

**And even those are reference, never truth:** the Paper mocks use per-frame literal hexes, so the same token can carry different values across frames. That is exactly the drift the token layer prevents in code.

---

## Working Notes
