# cmd config-path fallback tests can relocate the developer's real config files

The `cmd` package's config-path resolution tests exercise the "no env override, no
`XDG_CONFIG_HOME`" fallback arm by clearing both of those variables — but they leave `HOME`
pointing at the developer's real home directory. `configFilePath` therefore computes the real
`~/.config/portal/<file>` path, and on that path it also runs `migrateConfigFile`, the one-shot
move from the old macOS location (`~/Library/Application Support/portal/`).

That migration is not a read. If the old path holds a config file, running `go test ./cmd`
**moves it** to `~/.config/portal/`. On a machine where the old directory is already gone the
call is a stat-only no-op, which is why the suite has been green and why this has gone
unnoticed — the developer's current machine happens to be in that state.

The clearest instance is `cmd/prefs_path_test.go:39-57`, the
`TestPrefsFilePath/"falls back to ~/.config/portal/prefs.json"` subtest. The identical pattern
repeats in `cmd/config_test.go` for the hooks, aliases and projects files, so the exposure is
suite-wide across every config file Portal owns rather than a single stray test.

Notably, the *sibling* subtests in the same files — the ones that deliberately exercise the
migration itself, from `prefs_path_test.go:60` onward — do re-point `HOME` at a temp directory.
So the correct pattern is already present and applied a few lines below the affected cases; the
fallback arms simply do not use it.

This breaches CLAUDE.md's absolute test-isolation invariant, which states that a test must never
mutate or affect the real system — not the filesystem outside its temp dirs, not the default
tmux server, and not any OS process the test did not spawn. The filesystem boundary is the one
crossed here, and unlike the state-directory case there is no fingerprint-diff backstop watching
`~/.config/portal/`, so a relocation would be silent.

Surfaced by the reviewer on theming-system task 7-5 while verifying a separate, genuinely new
isolation breach that task had introduced (doctor's new `prefs.json` read reaching the real
config, since closed by a `PORTAL_PREFS_FILE` poison in the `cmd` package's `TestMain`). This
one is pre-existing and unrelated to the theming-system work — it predates that feature and was
merely found alongside it.
