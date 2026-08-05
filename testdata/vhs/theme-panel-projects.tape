# vhs tape: the §9.1 theme slide-over over the PROJECTS page.
#
# SCAFFOLDING, NOT AN ASSET (spec §13.2). This tape and the PNG it renders are
# created for the collaboration on task 8-16, committed while that gate is being
# worked, and CLEARED OUT at sign-off in Phase 10 — on exactly the same terms as
# the image. Nothing is expected to keep them; do not treat either as a
# visual-regression baseline. The Go fixture definition in internal/capture is
# PERMANENT and is NOT covered by that clearance — §13.4's swap-and-diff guard
# drives it.
#
# WHY THIS FRAME EXISTS — §14A: "§13.3 accordingly requires one Projects-with-panel
# fixture, so the page is seen with the panel over it BEFORE release rather than
# after."
#
# §9.6 binds `t` on Projects because theme is a GLOBAL setting — refusing would
# make it feel page-scoped for no reason — and §14A gave that page its own
# transient-flash slot for exactly the panel's six signals. Every other panel
# fixture is implicitly Sessions-based, so this is the only frame on which the
# Projects chrome and the slide-over are seen together at all.
#
# WHAT TO LOOK AT: the two-line Projects rows and their green `Projects 14` section
# header running UNDER the panel, with both slot badges (`● dark` on nord,
# `● light` on tokyo-night-day) rendering exactly as they do over Sessions — theme
# is global, so the panel must not look like a different surface here. And the
# footer: §9.1's ACCEPTED COST is that the overlay cuts WHEREVER ITS LEFT BORDER
# FALLS, because the page beneath is composed at the unreduced content width and
# deliberately NOT re-laid-out. On this frame that border lands past the last core
# entry, so what it takes is the whole right-aligned `? help` rather than half a
# word — the Sessions frames show the same cut landing mid-entry, which is the
# other outcome of the one rule. Reflowing to the reduced width would produce a
# cleaner edge and was rejected for that cost: it would reflow the very surface
# being previewed.
#
# THE COHERENCE PAIRING: `--theme nord` names the theme UNDER THE CURSOR. §9.2's
# invariant is that the cursor's row is always what is painted behind the panel, so
# the palette follows the CURSOR, never the persisted setting. At open that resolves
# to the DARK slot's theme, because capturetool runs no light/dark gate and the
# standing no-answer fallback selects dark (§8.8). Changing the --theme below
# without changing the fixture's seeded cursor produces a frame that is
# INDISTINGUISHABLE from a correct one to a reviewer and that §13.4's guard passes
# either way.
#
# THE TAPE TYPES THE KEYS ITSELF. The fixture declares `x` then `t` in its
# captureKeys and capturetool replays NO keys of its own — captureKeys is the
# offline mirror ModelAt drives, declared once so the two cannot drift. The `x` is
# the real user path to this page (the model opens on Sessions, the production
# no-arg default), and a tape that omitted either key would screenshot the wrong
# screen entirely, with nothing on the image to say so.
#
# Like every other fixture this NEVER opens a tmux server, spawns no daemon, runs
# no bootstrap and touches no real config: the project store is the in-memory fake,
# the union, the persisted keys, the slot records and the cursor are all declared
# fixture data, and --theme takes an explicit input rather than a config lookup.
#
# Run it (sandbox MUST be disabled — vhs needs loopback networking for ttyd;
# see testdata/vhs/README.md):
#
#   vhs testdata/vhs/theme-panel-projects.tape
#
# VHS IS KNOWN TO FAIL SILENTLY ON WRITE (§13.3): it runs the tape, reports no
# error, and does not produce the PNG. Hash the target before and after and
# confirm the hash CHANGED before trusting or reviewing the capture; retry on
# failure. This is a FIRST-TIME write through a freshly-written tape and a theme
# change is visible ONLY in the image, so an absent capture reads as "the change
# didn't render" and a stale one reads as a false pass.
#
# To view the same screen live in a real terminal instead (press `x` then `t` by
# hand — capturetool replays no keys):
#
#   go run ./cmd/capturetool --fixture theme-panel-projects --theme nord
#
# NOTE: vhs tape paths containing a slash MUST be quoted (the GIF Output and the
# Screenshot path below) — an unquoted slashed path is a parser error.

# Output is required by vhs even though we only care about the PNG screenshot.
Output "testdata/vhs/.gifcache/theme-panel-projects.gif"

Set FontFamily "JetBrains Mono"
Set FontSize 16
# 1280x800 resolves to 113 columns x 31 rows, the same terminal the other
# preferred-width panel tapes capture at, so the frames are comparable.
Set Width 1280
Set Height 800
# A fixed shell prompt keeps the captured frame deterministic across machines.
Set Shell "bash"

# Build then launch the capture tool with the Projects panel fixture pinned to the
# theme its cursor lands on. `go run` compiles + runs in one step; no tmux server,
# no bootstrap, no daemon.
Type "go run ./cmd/capturetool --fixture theme-panel-projects --theme nord"
Enter

# First paint: the model fetches the fixture sessions + projects on Init and
# renders the Sessions list. Give the compile + first render time to settle.
Sleep 4s

# The fixture's declared captureKeys, typed here so the tape and the offline
# driver reach the same frame: `x` switches to the Projects page, then `t` opens
# the slide-over over it (§9.6).
Type "x"
Sleep 1s
Type "t"
Sleep 1s

Screenshot "testdata/vhs/theme-panel-projects.png"
