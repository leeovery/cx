# vhs tape: the rename modal refusing a name beginning with "$".
#
# SCAFFOLDING, NOT AN ASSET. Cleared at sign-off; the Go fixture is permanent.
#
# The modal opens with the cursor at the END of the existing name and only a
# LEADING "$" is refused, so ctrl+a (the text input's line-start binding) is
# load-bearing here.
#
# WHAT TO LOOK AT: the notice band's copy names "$" and the start of the name,
# not ":" — the wrong-character bug this pair of strings exists to prevent.
Output "testdata/vhs/.gifcache/sessions-rename-refused-id-prefix.gif"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 900
Set Height 700
Set Shell "bash"
Type "go run ./cmd/capturetool --fixture sessions-rename-refused-id-prefix"
Enter
Sleep 5s
Type "r"
Sleep 800ms
Ctrl+a
Sleep 300ms
Type "$"
Sleep 400ms
Enter
Sleep 1s
Screenshot "testdata/vhs/sessions-rename-refused-id-prefix.png"
