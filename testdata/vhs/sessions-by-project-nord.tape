# vhs tape: Sessions — by Project (grouped) on the NORD built-in.
#
# SCAFFOLDING, NOT AN ASSET (spec §13.2). This tape and the PNG it renders are
# created for the collaboration on task 3-5, committed while that gate is being
# worked, and CLEARED OUT at sign-off in Phase 10 — on exactly the same terms as
# the image. Nothing is expected to keep them; do not treat either as a
# visual-regression baseline. The Go fixture definitions in internal/capture are
# permanent and are NOT covered by that clearance.
#
# WHY THIS FRAME EXISTS — the §7.4 outstanding visual gate.
#
# Nord's port shipped with one judgement still open. `text.subtle` #73819B has no
# locus on any Nord frame captured so far: it paints the group `··· N` counts and
# the loading page's pending steps, and neither appears on the flat Sessions
# screen the port was gated on. It is also an INVENTION rather than a palette
# lift — Nord's greys are barrelled at the ends (1.24 / 1.45 / 1.69 dark,
# 9.25 / 10.26 / 10.84 bright, nothing between), so the value is interpolated on
# nord3's hue and saturation and measures 3.18 against Nord's own canvas #2E3440.
# That is inside §13.5's 3.00-4.49 band with 0.18 of headroom, on the palette
# whose canvas is a mid-dark and whose contrast headroom is the tightest in the
# shipped set. The arithmetic cannot say whether it reads as de-emphasised but
# still readable. Only an eye on this frame can.
#
# WHAT TO LOOK AT: the `··· N` counts beside the group headings
# (agentic-workflows ··· 2, aviva ··· 1, designlab ··· 1, evvi ··· 1, fab ··· 1,
# Unknown ··· 1). They must read as a quieter tally than the heading word beside
# them (text.muted #939EB2) — clearly de-emphasised — while still being legible
# rather than sunk into the canvas. Both conditions matter: too bright and it is
# not the de-emphasised step the ramp needs; too dim and it is text.faint.
#
# The by-project fixture opens directly in By-Project mode (initialMode), so no
# key drive is needed to reach the grouped view — the first painted frame already
# carries the `— by project` section-header suffix, the group headings and the
# nested rows. The locus is proven to be ON this frame by
# internal/capture/grouped_subtle_locus_test.go
# (TestGroupedRender_CarriesTextSubtleCountLocus), which renders this same
# fixture under this same theme through the same tui.Build constructor — so the
# gate can never be taken on a frame where the token is invisible.
#
# Like every other fixture this NEVER opens a tmux server, spawns no daemon, runs
# no bootstrap and touches no real config: --theme takes an explicit input, not a
# config lookup.
#
# Run it (sandbox MUST be disabled — vhs needs loopback networking for ttyd;
# see testdata/vhs/README.md):
#
#   vhs testdata/vhs/sessions-by-project-nord.tape
#
# VHS IS KNOWN TO FAIL SILENTLY ON WRITE (§13.3): it runs the tape, reports no
# error, and does not produce the PNG. Hash the target before and after and
# confirm the hash CHANGED before trusting or reviewing the capture; retry on
# failure. A stale or absent image reads as a false pass, and on this frame the
# image is the entire signal — no functional assertion would catch it.
#
# To view the same screen live in a real terminal instead:
#
#   go run ./cmd/capturetool --fixture sessions-by-project --theme nord
#
# NOTE: vhs tape paths containing a slash MUST be quoted (the GIF Output and the
# Screenshot path below) — an unquoted slashed path is a parser error.

# Output is required by vhs even though we only care about the PNG screenshot.
Output "testdata/vhs/.gifcache/sessions-by-project-nord.gif"

Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1280
Set Height 800
# A fixed shell prompt keeps the captured frame deterministic across machines.
Set Shell "bash"

# Build then launch the capture tool with the by-project fixture pinned to nord.
# `go run` compiles + runs in one step; no tmux server, no bootstrap, no daemon.
Type "go run ./cmd/capturetool --fixture sessions-by-project --theme nord"
Enter

# First paint: the model fetches the fixture sessions + projects on Init and
# renders the grouped By-Project list. Give the compile + first render time to
# settle.
Sleep 4s

Screenshot "testdata/vhs/sessions-by-project-nord.png"
