# vhs tape: the §9.1 theme slide-over in its CONSTANT setting state, previewing
# another row.
#
# SCAFFOLDING, NOT AN ASSET (spec §13.2). This tape and the PNG it renders are
# created for the collaboration on task 8-15, committed while that gate is being
# worked, and CLEARED OUT at sign-off in Phase 10 — on exactly the same terms as
# the image. Nothing is expected to keep them; do not treat either as a
# visual-regression baseline. The Go fixture definition in internal/capture is
# PERMANENT and is NOT covered by that clearance — §13.4's swap-and-diff guard
# drives it.
#
# WHY THIS FRAME EXISTS — it COMPLETES the panel's specification.
#
# §9.5: "the two setting states never coexist on screen". A frame showing the slot
# badges structurally cannot show the constant's bare `●`, so the constant form
# needs a capture of its own or half the panel's specified vocabulary is never
# seen before release. §13.1 makes the harness the ONLY route to seeing any of it:
# Portal cannot be run from a temporary build without disturbing the live daemon
# and real state.
#
# It is also the PICKER IDIOM MADE VISIBLE, which is the thing the whole design
# rests on: the `●` is what is PERSISTED, the cursor plus the canvas is what is
# PREVIEWED, and `Esc` would restore the marked one. On this frame those are two
# DIFFERENT rows, which is the only way to see that they are two different
# signals.
#
# WHAT TO LOOK AT: a single BARE `●` on `nord` — no slot word beside it, because
# §8.2's tiebreak means a non-empty `theme` wins and the slots are not read at all
# — while the cursor bar sits on `tokyo-night-day` and the whole frame is painted
# in TOKYO NIGHT DAY, the light theme under the cursor. The mixed-mode flash is
# the feature, not a defect (§9.2): a light theme previewing in a dark terminal is
# precisely what live preview is for.
#
# THE COHERENCE PAIRING: `--theme tokyo-night-day` names the theme UNDER THE
# CURSOR — the PREVIEWED theme, deliberately NOT the marked constant `nord`.
# §9.2's invariant is that the cursor's row is always what is painted behind the
# panel. Capturing this under `--theme nord` would paint the persisted constant
# while the cursor sat elsewhere, which is exactly the state this frame exists to
# rule out — and an incoherent frame is INDISTINGUISHABLE from a correct one to a
# reviewer, with §13.4's guard passing either way.
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
#   vhs testdata/vhs/theme-panel-constant-previewing.tape
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
#   go run ./cmd/capturetool --fixture theme-panel-constant-previewing --theme tokyo-night-day
#
# NOTE: vhs tape paths containing a slash MUST be quoted (the GIF Output and the
# Screenshot path below) — an unquoted slashed path is a parser error.

# Output is required by vhs even though we only care about the PNG screenshot.
Output "testdata/vhs/.gifcache/theme-panel-constant-previewing.gif"

Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1280
Set Height 800
# A fixed shell prompt keeps the captured frame deterministic across machines.
Set Shell "bash"

# Build then launch the capture tool with the constant fixture pinned to the theme
# its cursor lands on — the PREVIEWED one, not the marked constant. `go run`
# compiles + runs in one step; no tmux server, no bootstrap, no daemon.
Type "go run ./cmd/capturetool --fixture theme-panel-constant-previewing --theme tokyo-night-day"
Enter

# First paint: the model fetches the fixture sessions + projects on Init and
# renders the Sessions list. Give the compile + first render time to settle.
Sleep 4s

# The fixture's declared captureKeys, typed here so the tape and the offline
# driver reach the same frame: `t` opens the slide-over (§9.6).
Type "t"
Sleep 1s

Screenshot "testdata/vhs/theme-panel-constant-previewing.png"
