# Themes (`.theme` files)

A Portal theme is one palette: 19 named colour roles in a flat `key = value`
file. Everything the picker draws resolves through those 19 roles — no part of
the interface carries a colour of its own — so a new palette restyles all of it.

The themes Portal ships with are ordinary `.theme` files, read by the same parser
as one you write yourself. A built-in has no privileges a drop-in lacks: same
keys, same rules, same result.

One file is one palette, and a palette is light or dark by virtue of the colours
in it. A light theme and a dark theme are two separate files.

## The 19 roles

These names are the contract between your file and Portal. All 19 must be
present: a role is what Portal asks for at the moment it draws something, so
there is no sensible default for one you leave out.

Roles are named for what they **mean** and how **prominent** they are, never for
a hue or for one place they happen to be drawn. `state.destructive` stays
truthful in any palette, whereas a name like `accent.violet` would not survive
the first port to a palette with no violet in it — and Portal deliberately reuses
one token across several surfaces, so naming it after any one of them would go
stale. Two roles are named after another token — `text.on-selection` and
`text.on-attention` — because their whole job is to stay legible on a specific
tint.

The use-sites below illustrate each role. They are not an inventory of every
place it appears.

### Text ramp

Six of the seven text roles form a single ramp, ordered here from **brightest to
faintest**:

`text.primary` → `text.secondary` → `text.tertiary` → `text.muted` →
`text.subtle` → `text.faint`

Each step should recede a little further than the one before it. That ordering is
a property of the roles themselves — how prominent each one is meant to be — and
not of the order you happen to write the keys in.

The seventh, `text.on-selection`, is not a step on the ramp: it is chosen to read
against the selection tint rather than for a weight of its own.

| Token | Role |
|---|---|
| `text.primary` | The brightest text. Session and project names, the `PORTAL` wordmark, active labels, modal titles, chip text. |
| `text.secondary` | One step down. The meta line on the selected row, help-modal actions, banner and signpost copy. |
| `text.tertiary` | Mid-ramp. Completed loading steps, the path on the selected row. |
| `text.muted` | Supporting detail, read at a glance rather than studied. Paths, counts, footer labels, subtitles, group headings. |
| `text.subtle` | De-emphasised but still readable. The `··· N` count beside a group heading, loading steps not yet reached. |
| `text.faint` | Decorative only. Inactive dots, the `+ add` slot, the mode indicator, hints. Never carries content a user must read. |
| `text.on-selection` | The session name on the selected row. Choose it to read against `bg.selection`, the tint it sits on. |

`text.faint` is the floor of the ramp and is **decorative only** — it never
carries content a user must read. Pick something visible but plainly receding:
Portal's own themes keep it deliberately *below* the legibility threshold every
other text role has to clear, because anything a user actually needs to read
belongs a step further up.

### Accents and states

Accents keep meaning names rather than weight names, because choosing a value for
one means knowing what the colour signifies.

| Token | Role |
|---|---|
| `accent.primary` | The primary accent, and the most used. The cursor, the selector bar, the active dot, the `?` key, a focused field label, the mode bar, the loading bar. |
| `accent.key` | Key-hint glyphs — the keys named in the footer and in modal hints. |
| `accent.mode` | Signals a distinct mode. The Sessions header, preview chrome, the tick of the step in progress. |
| `accent.attention` | The warm one. The `/` filter query, edit mode, the `⚠` warning glyph. |
| `state.positive` | Live, attached, done. The `●` attached dot, the Sessions count, the Projects label, `✓`, a success flash. |
| `state.destructive` | Kill and delete emphasis, and the `▲` marker. |

### Surfaces

Backgrounds, the one border role, and the text that sits on a tint.

| Token | Role |
|---|---|
| `canvas` | The backdrop Portal paints on every cell, window gutter included. This is the value that makes a theme light or dark, and every other role is chosen to read against it. |
| `bg.selection` | The tint behind the selected row. |
| `bg.attention` | The band behind a warning flash. |
| `bg.subtle` | A low neutral fill — the unfilled part of the loading bar. Nothing is drawn on top of it. |
| `border` | One role for every rule and frame: the title rule, the footer rule, modal panel frames, and edit-modal chips. |
| `text.on-attention` | The message inside a warning flash. Choose it to read against `bg.attention`, the tint it sits on. |

## Example theme

A complete theme — every key Portal requires, with a real value. Copy it and
change the colours. These are the values of Portal's dark built-in.

```ini
# Tokyo Night — https://github.com/tokyo-night/tokyo-night-vscode-theme
#
# Portal's dark built-in, reproduced here in full as a starting point.

# Text ramp, bright to faint.
text.primary = #C0CAF5
text.secondary = #A9B1D6
text.tertiary = #828BB8
text.muted = #737AA2
text.subtle = #535C86
text.faint = #3B4261
text.on-selection = #FFFFFF

# Accents and states.
accent.primary = #BB9AF7
accent.key = #7AA2F7
accent.mode = #7DCFFF
accent.attention = #FF9E64
state.positive = #9ECE6A
state.destructive = #F7768E

# Surfaces.
canvas = #0b0c14
bg.selection = #28243a
bg.attention = #241B10
bg.subtle = #26283A
border = #292E42
text.on-attention = #E8C9A0
```

## The file format

A theme file is a flat list of `key = value` lines with `#` comments — no
sections, no nesting, no includes. It carries colours and nothing else: a theme
cannot reach any other Portal setting, so a file someone hands you is only ever a
palette.

### `#` starts a comment only at the start of a line

**This is the rule worth reading twice**, because every value in a theme file
begins with `#` as well. Portal settles the collision by position: a `#` opens a
comment **only at the beginning of a line**, after any leading whitespace. There
are no trailing comments — a `#` anywhere to the right of the `=` is part of the
colour.

```ini
# A whole-line comment.
  # Indented, still a comment.

text.primary = #C0CAF5 # slightly warmer than the built-in
```

That last line is not a colour with a note attached. The value is everything
after the first `=`, so it reads `#C0CAF5 # slightly warmer than the built-in`,
which is not a colour — and the file is rejected as `bad colour`, naming that key.
Notes go on a line of their own above.

### The rest of the rules

| Rule | Detail |
|---|---|
| **Values are bare** | Never quoted. A value beginning with `"` or `'` is rejected as `bad syntax`, matched quote or not. |
| **A key appears once** | Writing a key twice rejects the file (`bad syntax`), whether or not the two values agree. Portal will not quietly pick one of them. |
| **Whitespace is trimmed** | Each line is trimmed at both ends, then around the `=`, so indenting a key is fine. Blank lines are ignored. |
| **Keys are lowercase, matched exactly** | `Text.Primary` is not `text.primary`. It is an unknown key (below), and the file then fails for the role it never declared. |
| **Every other line must parse** | Anything that is neither blank, nor a comment, nor a well-formed `key = value` pair rejects the file (`bad syntax`): a key with no `=`, a value with no key, a key containing a space. |

`portal doctor` names the offending line:

```
⚠ theme mine: bad syntax — line 30: duplicate key text.primary
```

### Colour values

Every value is a six-digit hex colour — `#RRGGBB`, upper or lower case, and
nothing else. There is no `#RGB` shorthand, no ANSI colour number (`212`), and no
colour name (`blue`). Anything else is `bad colour`, and the message names each
key alongside the value you wrote.

Portal validates the value itself rather than trusting the terminal colour
library, whose accepted range is far wider and stranger — there `212` is a valid
ANSI-256 index, `-5` quietly becomes `5`, and a failure is a silent nothing
rather than an error. Hex-only is a decision on top of that. Portal paints its
own exact hues instead of inheriting the terminal's sixteen ANSI colours, because
a palette that looks like itself on every machine is the whole point of having
one; and an ANSI index has no fixed RGB — `212` is whatever that terminal decided
it is — so it could never be measured against anything, including the contrast
floors Portal holds its own themes to.

### Unknown keys are ignored, a missing key rejects the theme

Two rules pulling in opposite directions, deliberately:

- **An unknown key is ignored** — key and value alike, so its value is not even
  checked. A line left over from an older Portal costs nothing.
- **A missing key rejects the whole theme.** It is not selectable, Portal renders
  a built-in in its place, and the message names what is missing:

  ```
  ⚠ theme short: missing tokens — missing text.primary, bg.subtle
  ```

So there is **no merge, no `inherits` / `parent` / `base` key, and no partial
files**. Every theme declares all 19 roles, which is why starting from a copy of
a built-in is the short route to a valid file — the copy already has them.

The pair also explains the one rejection that reads oddly. A mistyped or
wrong-cased key is an unknown key, so it is ignored, and the file then fails for
the role that key was meant to fill. Doctor names that role, which is what makes
the typo findable.

## Naming the file

Worth reading before you write the file: a name Portal will not accept means a
theme that never loads, whatever is inside it.

- **The filename is the identity.** The name minus `.theme` is the theme's
  **slug** — what the theme picker lists, what the config names, what
  `portal theme export` takes. There is no `name` field inside the file and no
  separate display label, so renaming a theme is renaming the file.
- **The slug must match `^[a-z0-9][a-z0-9-]*$`** — lowercase letters, digits and
  hyphens, starting with a letter or a digit. No spaces, no underscores, no
  capitals, and no leading hyphen (it would read as a flag everywhere a slug is
  typed). There is no length limit.
- **The extension must be exactly lowercase `.theme`.**

Break either and the file is rejected as `bad name` — an unselectable row in the
theme picker, and a line in `portal doctor` saying which rule the name broke:

```
⚠ theme file Nord.theme: slug must be lowercase letters, digits and hyphens
⚠ theme file nord.THEME: extension must be lowercase .theme
```

**Portal rejects a name; it never repairs one.** `Nord.theme` is not quietly read
as `nord`. Lowercasing it would let a user file take a built-in's slug — and a
built-in is exactly what Portal falls back to when a theme fails to load, so a
broken file could otherwise break the thing meant to catch it.

A wrong-cased extension gets one deliberate difference: `nord.THEME` is still
**listed**, and then rejected for its extension. A file that simply vanished
would leave you with nothing to go on — and you are most likely to type it that
way on macOS, where the filesystem does not distinguish the two names, so nothing
else would tell you either.

## Where the file goes

Portal reads themes from one directory, resolved in this order:

1. `PORTAL_THEMES_DIR`, when it is set
2. `$XDG_CONFIG_HOME/portal/themes/`
3. `~/.config/portal/themes/`

The `_DIR` suffix is the mechanical difference from `PORTAL_TERMINALS_FILE` and
its siblings: this one resolves a **directory**, where the others resolve single
files.

### What Portal sees in it

- **Top level only** — the files in the directory itself. There is no recursion
  into subdirectories.
- **Symlinked files are followed**, and the slug comes from the link's name
  rather than its target's, so keeping the real files in a dotfiles repo and
  linking them in works as you would expect.
- **The themes directory itself may be a symlink**, and is followed too —
  symlinking `~/.config/portal` wholesale is a normal thing to do.
- **An entry that resolves to a directory is skipped silently**, whether it is a
  real subdirectory named `something.theme` or a symlink pointing at one. What
  the entry resolves to is what decides, not whether a link is involved.
- **A dangling symlink is listed, then reported `unreadable`.** That reason
  covers every read failure, not only permissions.

A file whose name does not end in `.theme` — in any casing — is ignored
completely: no row, no reason, no log line. A file that was never a theme file
did not fail to be one.

### When the directory is missing, or unreadable

**An absent themes directory is silent.** Having no drop-ins is not a problem to
report: no doctor line, no log entry, nothing in the picker. Portal never creates
the directory and never seeds it, which is why the workflow below opens with
`mkdir -p`.

A directory that exists but cannot be read — wrong permissions, or a regular file
sitting where the directory belongs — is a genuine misconfiguration and says so.
`portal doctor` reports it:

```
⚠ themes directory unreadable: /Users/you/.config/portal/themes
```

and the theme picker pins a `⚠ dir unreadable` row above the list.

## Starting from a built-in

`portal theme export <slug>` writes a theme's file to stdout, comments and all.
The built-ins live inside the binary, so this is the way to get one onto disk —
and since it resolves your own themes too, it doubles as a way to ask what Portal
actually read.

The whole drop-in workflow is two lines:

```sh
mkdir -p ~/.config/portal/themes
portal theme export nord > ~/.config/portal/themes/nord-lee.theme
```

**The `mkdir -p` is part of that workflow, not an omission.** Portal never
creates the themes directory, and a shell redirect will not create it either —
without that line the first thing you meet is a redirect error.

The copy is named `nord-lee` rather than `nord` because a built-in's slug is
reserved: a drop-in needs a name of its own. Edit the copy, then open the theme
picker (`t`) to see it — the directory is re-read every time the picker opens, so
the edit-and-look loop needs no restart.

An export that cannot answer — an unknown name, an invalid file, a directory it
cannot read — prints the reason on stderr, writes nothing to stdout, and exits 1:

```
$ portal theme export nord-le
no theme named nord-le
```
