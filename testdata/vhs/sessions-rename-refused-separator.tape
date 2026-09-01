# vhs tape: the rename modal refusing a name containing ":".
#
# SCAFFOLDING, NOT AN ASSET. This tape and the PNG it renders exist for the
# visual gate on task 8-6 and are CLEARED OUT at sign-off. The Go fixture
# definition in internal/capture is PERMANENT and is not covered by that
# clearance — the swap-and-diff guard drives it.
#
# WHAT TO LOOK AT: the notice band under the title separator — the "▌" left bar,
# the "⚠" glyph, and the copy. Compare its framing and voice against the picker's
# other flash copy (sessions-inline-flash).
Output "testdata/vhs/.gifcache/sessions-rename-refused-separator.gif"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 900
Set Height 700
Set Shell "bash"
Type "go run ./cmd/capturetool --fixture sessions-rename-refused-separator"
Enter
Sleep 5s
Type "r"
Sleep 800ms
Type ":"
Sleep 400ms
Enter
Sleep 1s
Screenshot "testdata/vhs/sessions-rename-refused-separator.png"
