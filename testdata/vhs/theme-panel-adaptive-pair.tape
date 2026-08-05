# vhs tape: the §9.1 theme slide-over in its ADAPTIVE-PAIR setting state.
#
# SCAFFOLDING, NOT AN ASSET (spec §13.2). This tape and the PNG it renders are
# created for the collaboration on task 8-15, committed while that gate is being
# worked, and CLEARED OUT at sign-off in Phase 10 — on exactly the same terms as
# the image. Nothing is expected to keep them; do not treat either as a
# visual-regression baseline. The Go fixture definition in internal/capture is
# PERMANENT and is NOT covered by that clearance — §13.4's swap-and-diff guard
# drives it.
#
# WHY THIS FRAME EXISTS — §9.14's "no prior art anywhere".
#
# The panel is two halves with opposite risk profiles. The PICKER half has strong
# prior art (Helix's `:theme`, Ghostty's `+list-themes`, kitty's themes kitten all
# re-theme live behind a list), so a reviewer can check it against a familiar
# idiom. The SLOT half has NONE: assigning a theme to a light/dark slot FROM
# INSIDE A PICKER was found in no surveyed tool — Helix, Ghostty, Zellij, kitty
# and bat all require editing config for the pair — so `d`/`l` and the `● dark` /
# `● light` / `● both` badge vocabulary are genuinely novel and there is no
# established shape to borrow. This frame and the Paper artboard beside it
# (testdata/vhs/reference/theme-panel-adaptive-pair-mv.png) are the only reference
# that exists.
#
# WHAT TO LOOK AT: the two slot badges, right-aligned on their own rows —
# `● light` on `tokyo-night-day` and `● dark` on `nord` — with the cursor bar on
# `nord`, and the whole frame painted in NORD. The `●` marks what is SET; the
# `▌` + tint marks what is being BROWSED. They are independent signals and the
# frame is where that reads (or does not).
#
# THE COHERENCE PAIRING: `--theme nord` names the theme UNDER THE CURSOR. §9.2's
# invariant is that the cursor's row is always what is painted behind the panel,
# so the palette follows the CURSOR, never the persisted setting. At open that
# resolves to the DARK slot's theme, because capturetool runs no light/dark gate
# and the standing no-answer fallback selects dark (§8.8). Changing the --theme
# below without changing the fixture's seeded cursor produces a frame that is
# INDISTINGUISHABLE from a correct one to a reviewer and that §13.4's guard passes
# either way.
#
# THE TAPE TYPES THE KEY ITSELF. The fixture declares `t` in its captureKeys and
# capturetool replays NO keys of its own — captureKeys is the offline mirror
# ModelAt drives, declared once so the two cannot drift. A panel tape that omitted
# the keypress would screenshot a panel-less Sessions frame: the whole subject of
# the capture missing, with nothing on the image to say so.
#
# Like every other fixture this NEVER opens a tmux server, spawns no daemon, runs
# no bootstrap and touches no real config: the union, the persisted keys, the slot
# records and the cursor are all declared fixture data, and --theme takes an
# explicit input rather than a config lookup.
#
# Run it (sandbox MUST be disabled — vhs needs loopback networking for ttyd;
# see testdata/vhs/README.md):
#
#   vhs testdata/vhs/theme-panel-adaptive-pair.tape
#
# VHS IS KNOWN TO FAIL SILENTLY ON WRITE (§13.3): it runs the tape, reports no
# error, and does not produce the PNG. Hash the target before and after and
# confirm the hash CHANGED before trusting or reviewing the capture; retry on
# failure. This is a FIRST-TIME write through a freshly-written tape and a theme
# change is visible ONLY in the image, so an absent capture reads as "the change
# didn't render" and a stale one reads as a false pass.
#
# To view the same screen live in a real terminal instead (press `t` by hand —
# capturetool replays no keys):
#
#   go run ./cmd/capturetool --fixture theme-panel-adaptive-pair --theme nord
#
# NOTE: vhs tape paths containing a slash MUST be quoted (the GIF Output and the
# Screenshot path below) — an unquoted slashed path is a parser error.

# Output is required by vhs even though we only care about the PNG screenshot.
Output "testdata/vhs/.gifcache/theme-panel-adaptive-pair.gif"

Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1280
Set Height 800
# A fixed shell prompt keeps the captured frame deterministic across machines.
Set Shell "bash"

# Build then launch the capture tool with the adaptive-pair fixture pinned to the
# theme its cursor lands on. `go run` compiles + runs in one step; no tmux server,
# no bootstrap, no daemon.
Type "go run ./cmd/capturetool --fixture theme-panel-adaptive-pair --theme nord"
Enter

# First paint: the model fetches the fixture sessions + projects on Init and
# renders the Sessions list. Give the compile + first render time to settle.
Sleep 4s

# The fixture's declared captureKeys, typed here so the tape and the offline
# driver reach the same frame: `t` opens the slide-over (§9.6).
Type "t"
Sleep 1s

Screenshot "testdata/vhs/theme-panel-adaptive-pair.png"
