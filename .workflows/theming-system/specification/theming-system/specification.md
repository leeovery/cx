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

---

## Working Notes
