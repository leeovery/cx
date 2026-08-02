# vhs tape: Contrast-validation swatch — NORD (§7.4 port gate).
#
# Drives the offline capture harness (cmd/capturetool) with the
# `contrast-validation` fixture pointed at the embedded `nord` built-in. The
# swatch is the §13.1 human-eyeball instrument: a labelled set of tint bands —
# bg.selection, bg.attention, bg.subtle and the border rule — each filled with
# its own tint and carrying its §13.5 on-tint foreground text, all rendered on
# the theme's OWN canvas (Nord's #2E3440, a mid-dark rather than a near-black).
#
# This tape exists for task 2-7's gate: the two corrections
# (state.destructive #DD8188 from nord11, state.positive #A7C492 from nord14)
# and the three inventions (text.muted #939EB2, text.subtle #73819B,
# bg.attention #3D4046) are the judgements the numbers cannot settle. Read it
# beside testdata/vhs/reference/kill-confirm-modal-nord.png and
# testdata/vhs/reference/sessions-inline-flash-nord.png — reference, never
# truth: the Paper mocks use per-frame literal hexes (§9.14), so the .theme
# file is the authority.
#
# Like the sessions tapes this NEVER opens a tmux server or touches
# ~/.config/portal.
#
# Run it (sandbox MUST be disabled — vhs needs loopback networking for ttyd;
# see testdata/vhs/README.md):
#
#   vhs testdata/vhs/contrast-validation-nord.tape
#
# NOTE: vhs tape paths containing a slash MUST be quoted (the GIF Output and the
# Screenshot path below) — an unquoted slashed path is a parser error.

# Output is required by vhs even though we only care about the PNG screenshot.
Output "testdata/vhs/.gifcache/contrast-validation-nord.gif"

Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1280
Set Height 800
# A fixed shell prompt keeps the captured frame deterministic across machines.
Set Shell "bash"

# Build then launch the capture tool with the swatch fixture on the nord theme.
# `go run` compiles + runs in one step; no tmux server, no bootstrap, no daemon.
Type "go run ./cmd/capturetool --fixture contrast-validation --theme nord"
Enter

# First paint: the swatch renders the four tint bands on Nord's canvas. Give
# the compile + first render time to settle.
Sleep 4s

Screenshot "testdata/vhs/contrast-validation-nord.png"
