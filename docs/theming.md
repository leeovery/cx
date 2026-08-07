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
